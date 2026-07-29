package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"mindimprint/api/internal/agent"
	"mindimprint/api/internal/gateway"
	"mindimprint/api/internal/httpx"
	"mindimprint/api/internal/store/sqlc"
	"mindimprint/api/internal/studio"
)

// workspace_review.go — Slice 5 (Review room): the student's own five-dimension
// reflection doc (plain owned-project REST, no model call) and the 你的思维印记
// mirror (POST composes once via the flagship EvalResolver, first-open-wins;
// GET never calls a model). Nothing here is graded to the student.

// -- Reflection doc ---------------------------------------------------------

// reflectionDTO is the wire shape: an up-to-five-entry string array + done.
type reflectionDTO struct {
	Answers []string `json:"answers"`
	Done    bool     `json:"done"`
}

// decodeReflectionAnswers reads the stored jsonb string array; a malformed or
// absent blob reads as an empty slice (never fails the read).
func decodeReflectionAnswers(raw []byte) []string {
	answers := []string{}
	if len(raw) == 0 {
		return answers
	}
	if err := json.Unmarshal(raw, &answers); err != nil {
		return []string{}
	}
	return answers
}

// getReflection returns the project's reflection answers + done, or the empty
// state (no answers, not done) when no row exists yet.
func (a *API) getReflection(w http.ResponseWriter, r *http.Request) {
	projectID, ok := a.loadOwnedProject(w, r)
	if !ok {
		return
	}
	row, err := a.d.Queries.GetProjectReflection(r.Context(), projectID)
	if errors.Is(err, pgx.ErrNoRows) {
		httpx.WriteJSON(w, http.StatusOK, reflectionDTO{Answers: []string{}, Done: false})
		return
	}
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, reflectionDTO{Answers: decodeReflectionAnswers(row.Answers), Done: row.Done})
}

// putReflection upserts the reflection answers + done. done is optional in the
// body: absent → the current done value is preserved (an autosave of the
// answers must not silently un-finish a completed review). On done flipping
// from false to true, one auto-log line ("完成回顾") is dropped.
func (a *API) putReflection(w http.ResponseWriter, r *http.Request) {
	projectID, ok := a.loadOwnedProject(w, r)
	if !ok {
		return
	}
	var body struct {
		Answers []string `json:"answers"`
		Done    *bool    `json:"done"`
	}
	if err := decodeJSON(r, &body); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if body.Answers == nil {
		body.Answers = []string{}
	}

	// Read the prior done so the auto-log fires exactly once, on the false→true
	// transition, and so an answers-only PUT preserves the existing done.
	prevDone := false
	if prev, gerr := a.d.Queries.GetProjectReflection(r.Context(), projectID); gerr == nil {
		prevDone = prev.Done
	} else if !errors.Is(gerr, pgx.ErrNoRows) {
		httpx.WriteError(w, r, gerr)
		return
	}
	nextDone := prevDone
	if body.Done != nil {
		nextDone = *body.Done
	}

	answersJSON, merr := json.Marshal(body.Answers)
	if merr != nil {
		httpx.WriteError(w, r, httpx.ErrInternal())
		return
	}
	row, err := a.d.Queries.UpsertProjectReflection(r.Context(), sqlc.UpsertProjectReflectionParams{
		ProjectID: projectID, Answers: answersJSON, Done: nextDone,
	})
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if !prevDone && nextDone {
		if err := a.appendAutoLog(r.Context(), a.d.Queries, projectID, "完成回顾"); err != nil {
			slog.Warn("reflection: append auto-log failed", "err", err, "request_id", httpx.RequestIDFromContext(r.Context()))
		}
	}
	httpx.WriteJSON(w, http.StatusOK, reflectionDTO{Answers: decodeReflectionAnswers(row.Answers), Done: row.Done})
}

// -- Mirror -----------------------------------------------------------------

// mirrorDTO is the wire shape: the narrative sections + two carry-forwards.
type mirrorDTO struct {
	Sections      []mirrorSectionDTO `json:"sections"`
	CarryForwards []string           `json:"carryForwards"`
}

type mirrorSectionDTO struct {
	Title string `json:"title"`
	Body  string `json:"body"`
}

// mirrorDTOFromRow decodes the stored jsonb columns into the wire shape.
func mirrorDTOFromRow(row sqlc.ProjectMirrorProse) mirrorDTO {
	dto := mirrorDTO{Sections: []mirrorSectionDTO{}, CarryForwards: []string{}}
	var secs []mirrorSectionDTO
	if len(row.Sections) > 0 {
		if err := json.Unmarshal(row.Sections, &secs); err == nil {
			dto.Sections = secs
		}
	}
	var carries []string
	if len(row.CarryForwards) > 0 {
		if err := json.Unmarshal(row.CarryForwards, &carries); err == nil {
			dto.CarryForwards = carries
		}
	}
	return dto
}

// getMirror returns the stored mirror, or JSON null when none has been composed
// yet (the client then POSTs to compose it). This handler NEVER calls a model.
func (a *API) getMirror(w http.ResponseWriter, r *http.Request) {
	projectID, ok := a.loadOwnedProject(w, r)
	if !ok {
		return
	}
	row, err := a.d.Queries.GetProjectMirror(r.Context(), projectID)
	if errors.Is(err, pgx.ErrNoRows) {
		httpx.WriteJSON(w, http.StatusOK, nil)
		return
	}
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, mirrorDTOFromRow(row))
}

// postMirror is the ONLY Review endpoint that spends. First-open-wins: a stored
// row is returned without a model call; otherwise the flagship composer runs
// once. A SUCCESSFUL composition is persisted (so the second POST is a no-spend
// read of that row). A FAILED composition is NOT persisted (BE4): it returns a
// graceful minimal mirror this once, so a later open — when real data or the
// model becomes available — retries instead of being stuck on canned text
// forever. Cost is still recorded on failure (composeMirror does it).
func (a *API) postMirror(w http.ResponseWriter, r *http.Request) {
	projectID, ok := a.loadOwnedProject(w, r)
	if !ok {
		return
	}
	ctx := r.Context()

	// Already composed → return it, no spend (first-open-wins).
	if row, gerr := a.d.Queries.GetProjectMirror(ctx, projectID); gerr == nil {
		httpx.WriteJSON(w, http.StatusOK, mirrorDTOFromRow(row))
		return
	} else if !errors.Is(gerr, pgx.ErrNoRows) {
		httpx.WriteError(w, r, gerr)
		return
	}

	mirror, model, tier, composed := a.composeMirror(ctx, projectID)

	// Compose failed → return the graceful minimal mirror WITHOUT persisting, so
	// a later open retries (never a stuck canned mirror; BE4).
	if !composed {
		httpx.WriteJSON(w, http.StatusOK, mirrorDTO{
			Sections:      toMirrorSectionDTOs(mirror.Sections),
			CarryForwards: mirror.CarryForwards,
		})
		return
	}

	sectionsJSON, merr := json.Marshal(mirror.Sections)
	if merr != nil {
		httpx.WriteError(w, r, httpx.ErrInternal())
		return
	}
	carriesJSON, cerr := json.Marshal(mirror.CarryForwards)
	if cerr != nil {
		httpx.WriteError(w, r, httpx.ErrInternal())
		return
	}
	if err := a.d.Queries.InsertProjectMirror(ctx, sqlc.InsertProjectMirrorParams{
		ProjectID: projectID, Sections: sectionsJSON, CarryForwards: carriesJSON, Model: model, Tier: tier,
	}); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	// Re-read: a concurrent composer may have won the insert (ON CONFLICT DO
	// NOTHING); the winner's row is what both must see.
	row, rerr := a.d.Queries.GetProjectMirror(ctx, projectID)
	if rerr != nil {
		httpx.WriteJSON(w, http.StatusOK, mirrorDTO{
			Sections:      toMirrorSectionDTOs(mirror.Sections),
			CarryForwards: mirror.CarryForwards,
		})
		return
	}
	httpx.WriteJSON(w, http.StatusOK, mirrorDTOFromRow(row))
}

// composeAndStoreProjectMirror is the finish goroutine's best-effort mirror
// pass: compose once over the now-complete process record and persist ONLY on a
// successful compose (same no-persist-on-failure rule as postMirror). Never
// returns an error — a failed compose (or a mirror that already exists) simply
// leaves the row for a later Review open to compose. ctx should carry the user
// (WithUser) so the cost row records.
func (a *API) composeAndStoreProjectMirror(ctx context.Context, projectID uuid.UUID) {
	if _, gerr := a.d.Queries.GetProjectMirror(ctx, projectID); gerr == nil {
		return // already composed — first-open-wins
	} else if !errors.Is(gerr, pgx.ErrNoRows) {
		return
	}
	mirror, model, tier, composed := a.composeMirror(ctx, projectID)
	if !composed {
		return // don't persist a failed compose (BE4)
	}
	sectionsJSON, merr := json.Marshal(mirror.Sections)
	if merr != nil {
		return
	}
	carriesJSON, cerr := json.Marshal(mirror.CarryForwards)
	if cerr != nil {
		return
	}
	if err := a.d.Queries.InsertProjectMirror(ctx, sqlc.InsertProjectMirrorParams{
		ProjectID: projectID, Sections: sectionsJSON, CarryForwards: carriesJSON, Model: model, Tier: tier,
	}); err != nil {
		slog.Warn("finish mirror: insert failed", "err", err)
	}
}

func toMirrorSectionDTOs(secs []agent.MirrorSection) []mirrorSectionDTO {
	out := make([]mirrorSectionDTO, 0, len(secs))
	for _, s := range secs {
		out = append(out, mirrorSectionDTO{Title: s.Title, Body: s.Body})
	}
	return out
}

// composeMirror runs the flagship composer over the project's process record
// and records the call's cost (surface="studio", purpose="mirror") BEFORE any
// bail. It NEVER returns an error: on a missing provider or a rejected/failed
// composition it returns MinimalMirror with composed=false (still recording cost
// when a call was actually made). composed=true only on a real, successful
// composition — the caller persists ONLY then (BE4). Returns the mirror plus the
// resolved model/tier and the composed flag.
func (a *API) composeMirror(ctx context.Context, projectID uuid.UUID) (agent.Mirror, string, string, bool) {
	in := a.buildMirrorInput(ctx, projectID)

	resolved, rerr := a.d.EvalResolver(ctx)
	if rerr != nil {
		slog.Warn("mirror: no provider", "err", rerr)
		return agent.MinimalMirror(), "", "", false
	}
	mirror, usage, cerr := agent.ComposeMirror(ctx, a.d.Provider, resolved, in)
	if u, ok := UserFromContext(ctx); ok && resolved.Provider != "" {
		// Unpriced model → explicit $0.00 (CostNumeric(cost, true)), never the
		// ok-derived NULL Numeric — llm_call.cost_estimate is NOT NULL.
		cost, priced := gateway.EstimateCost(resolved.Provider, resolved.Model, usage.InputTokens, usage.OutputTokens)
		if !priced {
			slog.Warn("mirror llm_call: unpriced model — cost recorded as 0", "provider", resolved.Provider, "model", resolved.Model)
		}
		if _, err := a.d.Queries.RecordLLMCall(ctx, sqlc.RecordLLMCallParams{
			UserID: u.ID, ProjectID: pgtype.UUID{Bytes: projectID, Valid: true},
			Surface: "studio", Purpose: "mirror",
			Provider: resolved.Provider, Model: resolved.Model, Tier: resolved.Tier,
			PromptTokens: int32(usage.InputTokens), CompletionTokens: int32(usage.OutputTokens),
			CostEstimate: gateway.CostNumeric(cost, true),
		}); err != nil {
			slog.Warn("mirror: record llm call", "err", err)
		}
	}
	if cerr != nil {
		slog.Warn("mirror: compose failed — not persisting (retry on a later open)", "err", cerr)
		return agent.MinimalMirror(), resolved.Model, resolved.Tier, false
	}
	return mirror, resolved.Model, resolved.Tier, true
}

// buildMirrorInput assembles the honest process record from the project's
// proposal (four kick-off dims), reflection answers, outline, current draft and
// an events/cards digest. Every read is best-effort — a missing part is simply
// left out (never fabricated).
func (a *API) buildMirrorInput(ctx context.Context, projectID uuid.UUID) agent.MirrorInput {
	in := agent.MirrorInput{}

	if u, ok := UserFromContext(ctx); ok {
		in.StudentName = u.DisplayName
	}
	if p, err := a.d.Queries.GetProjectProposal(ctx, projectID); err == nil {
		in.Objective, in.Reason, in.Activities, in.Resources = p.Objective, p.Reason, p.Activities, p.Resources
	}
	if refl, err := a.d.Queries.GetProjectReflection(ctx, projectID); err == nil {
		in.Reflection = decodeReflectionAnswers(refl.Answers)
	}
	in.Outline = outlineDigest(ctx, a.d.Queries, projectID)
	if draft, err := a.d.Queries.GetEditBuffer(ctx, projectID); err == nil {
		in.Draft = draft
	}
	in.ProcessDigest = processDigest(ctx, a.d.Queries, projectID)
	return in
}

// outlineDigest renders the outline as indented bullets ("" when empty).
func outlineDigest(ctx context.Context, q *sqlc.Queries, projectID uuid.UUID) string {
	nodes, err := q.ListOutlineNodes(ctx, projectID)
	if err != nil || len(nodes) == 0 {
		return ""
	}
	var b strings.Builder
	for _, n := range nodes {
		if strings.TrimSpace(n.Text) == "" {
			continue
		}
		indent := strings.Repeat("  ", int(n.Depth))
		fmt.Fprintf(&b, "%s- %s\n", indent, strings.TrimSpace(n.Text))
	}
	return strings.TrimRight(b.String(), "\n")
}

// processDigest summarizes the project's process: how many events accrued and
// which tool cards the student summoned (deduped, in first-seen order). "" when
// nothing to report — the composer must not invent a card that was not used.
func processDigest(ctx context.Context, q *sqlc.Queries, projectID uuid.UUID) string {
	d, err := studio.Load(ctx, q, projectID)
	if err != nil {
		return ""
	}
	var parts []string
	if len(d.Events) > 0 {
		parts = append(parts, fmt.Sprintf("过程事件 %d 条", len(d.Events)))
	}
	seen := map[string]bool{}
	var cardIDs []string
	for _, c := range d.Cards {
		if c.CardID == "" || seen[c.CardID] {
			continue
		}
		seen[c.CardID] = true
		cardIDs = append(cardIDs, c.CardID)
	}
	if len(cardIDs) > 0 {
		parts = append(parts, "召唤过的工具卡："+strings.Join(cardIDs, "、"))
	}
	return strings.Join(parts, "；")
}
