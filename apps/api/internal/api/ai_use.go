package api

// ai_use.go — S5 · the AI-interaction retrospective endpoints (回顾 · 复盘我与
// AI 的互动). Split-hybrid, cloning S2's takeaway shape:
//   - GET /ai-use-draft: assemble the objective record (no spend) + a seed draft
//     (one metered mid-tier call, only when there IS a record and nothing saved).
//   - GET /ai-use: the stored student statement (no spend).
//   - POST /ai-use: persist the student-authored statement (no spend) + event.
// The AI never writes the reflection — it assembles the objective facts and
// seeds a draft; the student authors used_for/not_used_for (AI 克制).

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"mindimprint/api/internal/agent"
	"mindimprint/api/internal/httpx"
	"mindimprint/api/internal/store/sqlc"
)

type aiUseStatementDTO struct {
	UsedFor    string `json:"usedFor"`
	NotUsedFor string `json:"notUsedFor"`
}

type aiUseRecordDTO struct {
	CoachTurns        int            `json:"coachTurns"`
	CardsProposed     int            `json:"cardsProposed"`
	CardsAccepted     int            `json:"cardsAccepted"`
	CardsDismissed    int            `json:"cardsDismissed"`
	SourcesOpened     int            `json:"sourcesOpened"`
	LLMCallsByPurpose map[string]int `json:"llmCallsByPurpose"`
	GhostwroteEssay   bool           `json:"ghostwroteEssay"`
	PredictedScore    bool           `json:"predictedScore"`
}

type aiUseDraftDTO struct {
	Record aiUseRecordDTO    `json:"record"`
	Draft  aiUseStatementDTO `json:"draft"`
}

func toAIUseRecordDTO(r aiUseRecord) aiUseRecordDTO {
	m := r.LLMCallsByPurpose
	if m == nil {
		m = map[string]int{}
	}
	return aiUseRecordDTO{
		CoachTurns: r.CoachTurns, CardsProposed: r.CardsProposed, CardsAccepted: r.CardsAccepted,
		CardsDismissed: r.CardsDismissed, SourcesOpened: r.SourcesOpened,
		LLMCallsByPurpose: m, GhostwroteEssay: r.GhostwroteEssay, PredictedScore: r.PredictedScore,
	}
}

func toAIUseRecordView(r aiUseRecord) agent.AIUseRecordView {
	return agent.AIUseRecordView{
		CoachTurns: r.CoachTurns, CardsProposed: r.CardsProposed, CardsAccepted: r.CardsAccepted,
		CardsDismissed: r.CardsDismissed, SourcesOpened: r.SourcesOpened, LLMCallsByPurpose: r.LLMCallsByPurpose,
	}
}

// buildProjectAIUseRecord assembles the objective interaction record for a
// project from its events + llm_call rows. Best-effort loads (a failed load
// degrades a slice to empty, never fails the caller). ctx-based so both the
// request handler and the finish-goroutine assessor can call it.
func (a *API) buildProjectAIUseRecord(ctx context.Context, projectID uuid.UUID) aiUseRecord {
	pid := pgtype.UUID{Bytes: projectID, Valid: true}
	events, _ := a.d.Queries.ListEventsByProject(ctx, pid)
	llmCalls, _ := a.d.Queries.ListLLMCallsByProject(ctx, pid)
	return buildAIUseRecord(events, llmCalls)
}

// aiUseRecordLine is a one-line objective-record digest for the assessor prompt.
func aiUseRecordLine(r aiUseRecord) string {
	// CardsAccepted spans all summon paths (not only the proposed ones), so phrase
	// it as a separate count, not a subset of CardsProposed.
	return fmt.Sprintf("%d 轮对话 · AI 提议 %d 张卡（跳过 %d）· 你一共打开 %d 张卡 · 打开 %d 个来源 · 无代写正文、无预测分数",
		r.CoachTurns, r.CardsProposed, r.CardsDismissed, r.CardsAccepted, r.SourcesOpened)
}

// getAIUseDraft returns the objective record + a draft statement. The draft is
// the student's SAVED statement if one exists (no spend); otherwise, when there
// is an interaction record to reflect on, one metered mid-tier seed. An empty
// record spends nothing; a failed seed returns the record with an empty draft.
func (a *API) getAIUseDraft(w http.ResponseWriter, r *http.Request) {
	row, ok := a.loadOwnedProjectRow(w, r)
	if !ok {
		return
	}
	projectID := row.ID
	rec := a.buildProjectAIUseRecord(r.Context(), projectID)

	usedFor, notUsedFor := "", ""
	if saved, err := a.d.Queries.GetProjectAIUse(r.Context(), projectID); err == nil {
		usedFor, notUsedFor = saved.UsedFor, saved.NotUsedFor
	}

	if row.IsDemo {
		// Demo project (guided-tour P2, world-readable): never call the LLM —
		// return the saved project_ai_use statement if one exists, else an
		// empty draft, the same shape this handler returns below when there's
		// nothing to seed.
		httpx.WriteJSON(w, http.StatusOK, aiUseDraftDTO{
			Record: toAIUseRecordDTO(rec),
			Draft:  aiUseStatementDTO{UsedFor: usedFor, NotUsedFor: notUsedFor},
		})
		return
	}

	// Seed ONLY when nothing is saved yet AND there is a record to seed from.
	if usedFor == "" && notUsedFor == "" && hasInteractionRecord(rec) {
		if resolved, rerr := a.d.ChatResolver(r.Context()); rerr == nil {
			u, n, usage, cerr := agent.ComposeAIUseSeed(r.Context(), a.d.Provider, resolved, toAIUseRecordView(rec))
			if resolved.Provider != "" && cerr == nil {
				store := agent.NewSqlcAgentStore(a.d.Queries, a.d.Pool)
				if e := store.RecordLLMCall(r.Context(), agent.LLMCallRow{
					ProjectID: projectID, Surface: "studio", Purpose: "ai_use_retrospective",
					Resolved: resolved, PromptTokens: int32(usage.InputTokens), CompletionTokens: int32(usage.OutputTokens),
				}); e != nil {
					slog.Warn("ai-use draft: record llm", "err", e, "request_id", httpx.RequestIDFromContext(r.Context()))
				}
			}
			if cerr == nil {
				usedFor, notUsedFor = u, n
			}
		}
	}

	httpx.WriteJSON(w, http.StatusOK, aiUseDraftDTO{
		Record: toAIUseRecordDTO(rec),
		Draft:  aiUseStatementDTO{UsedFor: usedFor, NotUsedFor: notUsedFor},
	})
}

// getAIUse returns the stored student statement (zero doc if none). No spend.
func (a *API) getAIUse(w http.ResponseWriter, r *http.Request) {
	projectID, ok := a.loadOwnedProject(w, r)
	if !ok {
		return
	}
	st := aiUseStatementDTO{}
	if saved, err := a.d.Queries.GetProjectAIUse(r.Context(), projectID); err == nil {
		st = aiUseStatementDTO{UsedFor: saved.UsedFor, NotUsedFor: saved.NotUsedFor}
	}
	httpx.WriteJSON(w, http.StatusOK, st)
}

// postAIUse persists the student-authored AI-use statement. No spend; upsert
// (re-review supersedes); emits an ai_use_written event (过程即数据).
func (a *API) postAIUse(w http.ResponseWriter, r *http.Request) {
	projectID, ok := a.loadOwnedProject(w, r)
	if !ok {
		return
	}
	var body aiUseStatementDTO
	if err := decodeJSON(r, &body); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if err := a.d.Queries.UpsertProjectAIUse(r.Context(), sqlc.UpsertProjectAIUseParams{
		ProjectID: projectID, UsedFor: body.UsedFor, NotUsedFor: body.NotUsedFor,
	}); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	store := agent.NewSqlcAgentStore(a.d.Queries, a.d.Pool)
	if err := store.AppendEvent(r.Context(), agent.EventRow{
		ProjectID: projectID, Surface: "studio", Type: "ai_use_written", Payload: []byte(`{}`),
	}); err != nil {
		slog.Warn("ai-use: append ai_use_written event failed", "err", err, "request_id", httpx.RequestIDFromContext(r.Context()))
	}
	httpx.WriteJSON(w, http.StatusOK, body)
}
