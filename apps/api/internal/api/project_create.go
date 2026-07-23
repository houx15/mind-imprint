package api

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"mindimprint/api/internal/agent"
	"mindimprint/api/internal/httpx"
	"mindimprint/api/internal/onboarding"
	"mindimprint/api/internal/skills"
	"mindimprint/api/internal/store/sqlc"
)

// createProject is the funnel entry: it atomically creates a project and seeds
// its S0 任务解码 onboarding nodes from the board-static fixture. No model call.
func (a *API) createProject(w http.ResponseWriter, r *http.Request) {
	u, _ := UserFromContext(r.Context())
	entitled, err := HasEntitlement(r.Context(), u)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if !entitled {
		httpx.WriteError(w, r, httpx.ErrNotEntitled())
		return
	}
	var req struct {
		Title  string `json:"title"`
		Prompt string `json:"prompt"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteError(w, r, httpx.ErrBadRequest("bad_json", "请求格式不对", nil))
		return
	}
	prompt := strings.TrimSpace(req.Prompt)
	if prompt == "" {
		httpx.WriteError(w, r, httpx.ErrBadRequest("missing_prompt", "请先贴上任务要求。", nil))
		return
	}
	title := strings.TrimSpace(req.Title)
	if title == "" {
		title = "未命名论文"
	}

	const qualification = "0457"
	fx, ok := onboarding.Load(qualification)
	if !ok {
		httpx.WriteError(w, r, errors.New("onboarding fixture missing for qualification "+qualification))
		return
	}
	rubricBody, _ := json.Marshal(map[string]any{"restate_prompt": fx.RestatePrompt, "rows": fx.Rows})
	planBody, _ := json.Marshal(map[string]any{"steps": fx.Steps})
	briefBody, _ := json.Marshal(map[string]any{"text": prompt})
	// The title is the research question she typed in the creation funnel —
	// author "student" because she wrote it. S1's banner renders it read-only
	// (dc.html:878–884) and frame_question's node_present item reads it.
	rqBody, _ := json.Marshal(map[string]any{"text": title})

	tx, err := a.d.Pool.Begin(r.Context())
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	defer func() { _ = tx.Rollback(r.Context()) }()
	qtx := a.d.Queries.WithTx(tx)

	proj, err := qtx.CreateProject(r.Context(), sqlc.CreateProjectParams{
		UserID: u.ID, Qualification: qualification, Title: title,
		Deadline: pgtype.Timestamptz{}, BoardCfgVer: 1,
	})
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	for _, n := range []struct {
		typ, author string
		body        []byte
	}{
		{"assignment_brief", "imported", briefBody},
		{"rubric_translation", "ai", rubricBody},
		{"milestone_plan", "ai", planBody},
		{"research_question", "student", rqBody},
	} {
		if _, err := qtx.InsertGraphNode(r.Context(), sqlc.InsertGraphNodeParams{
			ProjectID: proj.ID, Type: n.typ, Body: n.body, Author: n.author, SpanRef: nil,
		}); err != nil {
			httpx.WriteError(w, r, err)
			return
		}
	}
	if err := tx.Commit(r.Context()); err != nil {
		httpx.WriteError(w, r, err)
		return
	}

	// N6-E: compose a per-student journey from the pasted prompt. Best-effort —
	// the project already exists; any failure leaves the full journey (today's
	// behavior). The call is metered even when it waives nothing or is rejected.
	a.composeJourney(r.Context(), proj.ID, prompt)

	httpx.WriteJSON(w, http.StatusCreated, map[string]any{"id": proj.ID.String()})
}

// composeJourney runs the mid-tier journey composer and, if it yields a waived
// set, persists it + records a journey_composed event. Every branch that made
// a real model call is metered (purpose "compose_journey"). Fail-safe by
// construction: it only ever calls SetWaived with a non-empty set, so no path
// here can strand the student.
func (a *API) composeJourney(ctx context.Context, projectID uuid.UUID, prompt string) {
	if a.d.Provider == nil || a.d.ChatResolver == nil {
		return // no model wired (e.g. a test double) — full journey, no crash
	}
	sk, ok := skills.ByID("writing-project")
	if !ok {
		return
	}
	store := agent.NewSqlcAgentStore(a.d.Queries, a.d.Pool)
	res := agent.ComposeJourney(ctx, a.d.Provider, a.d.ChatResolver, sk, prompt)
	if res.Resolved.Provider != "" {
		if err := store.RecordLLMCall(ctx, agent.LLMCallRow{
			ProjectID: projectID, Surface: "studio", Purpose: "compose_journey",
			Resolved: res.Resolved, PromptTokens: int32(res.Usage.InputTokens), CompletionTokens: int32(res.Usage.OutputTokens),
		}); err != nil {
			slog.Warn("compose journey: record llm usage failed", "err", err, "project_id", projectID)
		}
	}
	if len(res.Waived) == 0 {
		return // full journey (fail-safe or all-keep) — nothing to persist
	}
	if err := store.SetWaived(ctx, projectID, res.Waived); err != nil {
		slog.Warn("compose journey: set waived failed", "err", err, "project_id", projectID)
		return
	}
	payload, _ := json.Marshal(map[string]any{"waived": res.Waived, "decisions": res.Decisions})
	if err := store.AppendEvent(ctx, agent.EventRow{
		ProjectID: projectID, Surface: "studio", Type: "journey_composed", Payload: payload,
	}); err != nil {
		slog.Warn("compose journey: append event failed", "err", err, "project_id", projectID)
	}
}
