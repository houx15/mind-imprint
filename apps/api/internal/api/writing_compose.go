package api

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"mindimprint/api/internal/gateway"
	"mindimprint/api/internal/httpx"
	"mindimprint/api/internal/store/sqlc"
)

// writing_compose.go — Task 7 of the lite writing phase, the last of the
// four steps: 成稿 (compose). Four routes:
//   - POST /writings/{id}/compose  — assemble writing_draft.body from her
//     already-written fragments. THE 铁律① PRESSURE POINT of this task: this
//     step is DETERMINISTIC — string concatenation, nothing more. It must
//     never call a.d.Provider. See composeWritingDraft's comment.
//   - GET|PUT /writings/{id}/draft — read/edit the assembled draft directly
//     (she keeps polishing it herself after compose runs, same "silent
//     student scratch" shape as pro's putEditBuffer, writing.go).
//   - POST /writings/{id}/review   — one model call that comments on the
//     WHOLE piece. Feedback only: it returns commentary in the HTTP response
//     and never writes to writing_draft (or anywhere else) — the AI may
//     critique what she wrote, never hand her replacement text (AGENTS.md's
//     "Review feedback is different and legitimate" carve-out from 铁律①).
//   - POST /writings/{id}/finish   — the terminal state. Gates on a
//     non-empty draft (400 missing_draft), sets status='finished' via
//     SetWritingFinished (idempotent by construction — see that query's
//     comment), and deliberately never touches stage (per this task's
//     product-owner note: status and stage are independent facts, and a
//     store-level test already pins that independence).
//
// Named writing_compose.go / composeWritingDraft / getWritingDraft /
// putWritingDraft / reviewWritingDraft / finishWritingAtom throughout — pro
// already owns getDraft/putBuffer/finishWriting (workspace_write.go's
// getDraft, writing.go's putEditBuffer, project_writing_finish.go's
// finishWriting) for the project-scoped 写作 feature; same
// two-unrelated-domains-share-a-word situation as writing_outline.go vs
// pro's outline (see that file's comment).

// writingDraftDTO is writing_draft's wire shape. UpdatedAt is nullable
// because a writing that has never been composed nor had its draft PUT has
// no row at all yet — GetWritingDraft's ErrNoRows is a legitimate "nothing
// written yet" state (mirrors hasSource, readings.go), not an error.
type writingDraftDTO struct {
	Body      string  `json:"body"`
	UpdatedAt *string `json:"updatedAt"`
}

func toWritingDraftDTO(row sqlc.WritingDraft) writingDraftDTO {
	s := row.UpdatedAt.Format(time.RFC3339)
	return writingDraftDTO{Body: row.Body, UpdatedAt: &s}
}

// emptyWritingDraftDTO is what GET returns before any compose/PUT has ever
// happened for this atom.
func emptyWritingDraftDTO() writingDraftDTO {
	return writingDraftDTO{Body: "", UpdatedAt: nil}
}

// getWritingDraft is GET /api/v1/writings/{id}/draft.
func (a *API) getWritingDraft(w http.ResponseWriter, r *http.Request) {
	at, ok := a.loadOwnedWritingAtom(w, r)
	if !ok {
		return
	}
	row, err := a.d.Queries.GetWritingDraft(r.Context(), at.ID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			httpx.WriteJSON(w, http.StatusOK, emptyWritingDraftDTO())
			return
		}
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, toWritingDraftDTO(row))
}

// putWritingDraft is PUT /api/v1/writings/{id}/draft — she keeps editing the
// assembled draft directly, same as before compose ever ran. Student text
// ONLY, stored verbatim (no trim) — mirrors putEditBuffer's "silent student
// scratch" shape (writing.go): a full prose blob is not a single-line field
// like a title, so no whitespace normalization is imposed on it. No
// entitlement gate — no model call, no spend, same reasoning as
// putEditBuffer/putWritingSnippets/putWritingOutline.
func (a *API) putWritingDraft(w http.ResponseWriter, r *http.Request) {
	at, ok := a.loadOwnedWritingAtom(w, r)
	if !ok {
		return
	}
	var body struct {
		Body string `json:"body"`
	}
	if err := decodeJSON(r, &body); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	row, err := a.d.Queries.UpsertWritingDraft(r.Context(), sqlc.UpsertWritingDraftParams{
		AtomID: at.ID, Body: body.Body,
	})
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, toWritingDraftDTO(row))
}

// composeSnippetsIntoDraft is the mechanical assembly itself, kept pure and
// separate from the handler so "only concatenate, never invent" is
// verifiable by reading one small function: take every fragment's text IN
// POSITION ORDER (ListWritingSnippets already orders by position), drop the
// still-empty slots (an unwritten slot contributes nothing, not a blank
// paragraph), and join what remains with a single blank line between
// paragraphs. Every byte of the result traces back to a byte she typed into
// a snippet — nothing here can add a character that wasn't already in one of
// the inputs.
func composeSnippetsIntoDraft(snippets []sqlc.WritingSnippet) string {
	paragraphs := make([]string, 0, len(snippets))
	for _, s := range snippets {
		text := strings.TrimSpace(s.Text)
		if text == "" {
			continue
		}
		paragraphs = append(paragraphs, text)
	}
	return strings.Join(paragraphs, "\n\n")
}

// composeWritingDraft is POST /api/v1/writings/{id}/compose — the 铁律①
// pressure point this task exists to prove. It reads writing_snippet, calls
// composeSnippetsIntoDraft (pure string work, see its comment), and upserts
// the result into writing_draft. Nowhere in this function — nowhere in this
// FILE — does a.d.Provider, gateway.Collect, resolveEval or ChatResolver
// ever appear: there is no model call to make, because assembling her own
// fragments is a deterministic system step (AGENTS.md's 适用边界 paragraph),
// not prose generation. No entitlement gate either, for the same reason
// putEditBuffer has none — nothing here spends anything.
//
// TestWritingCompose_NeverCallsModel is the mechanical proof: it runs this
// handler with a nil (panicking-on-call) provider and asserts success.
func (a *API) composeWritingDraft(w http.ResponseWriter, r *http.Request) {
	at, ok := a.loadOwnedWritingAtom(w, r)
	if !ok {
		return
	}
	snippets, err := a.d.Queries.ListWritingSnippets(r.Context(), at.ID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	body := composeSnippetsIntoDraft(snippets)
	row, err := a.d.Queries.UpsertWritingDraft(r.Context(), sqlc.UpsertWritingDraftParams{
		AtomID: at.ID, Body: body,
	})
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, toWritingDraftDTO(row))
}

// reviewWritingDraft is POST /api/v1/writings/{id}/review — a spend endpoint
// (one model call): gates on HasEntitlement and on a non-empty draft (400
// missing_draft — reviewing nothing would just burn a call for no reason),
// meters purpose="review" BEFORE any bail.
//
// Task 5 (B4+B7) changed this handler's output shape: it used to return
// {"feedback":"<prose>"} and write nowhere — commentary that rendered once
// and evaporated on the next navigation. It now produces the SAME
// {summary, points} object commentOnSnippet does (writing_comment.go),
// validates every point's quote against the draft body with
// validateCommentPoints, PERSISTS it via CreateWritingComment with
// scope='draft' and snippet_id=NULL, and responds {"comment": Comment}. It
// still never writes to writing_draft.body or anywhere else — persisting the
// comment is not the same as persisting a rewrite of her text, and AGENTS.md's
// "Review feedback is different and legitimate... It must not hand her
// replacement text" carve-out from 铁律① is exactly as true of a structured
// comment as it was of a paragraph of prose. A model failure or an
// unparseable reply both surface as the honest 502 ai_dialogue_failed, never
// a canned fallback comment.
func (a *API) reviewWritingDraft(w http.ResponseWriter, r *http.Request) {
	at, ok := a.loadOwnedWritingAtom(w, r)
	if !ok {
		return
	}
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

	draft, derr := a.d.Queries.GetWritingDraft(r.Context(), at.ID)
	if derr != nil && !errors.Is(derr, pgx.ErrNoRows) {
		httpx.WriteError(w, r, derr)
		return
	}
	body := strings.TrimSpace(draft.Body)
	if body == "" {
		httpx.WriteError(w, r, httpx.ErrBadRequest("missing_draft", "先完成初稿，再请我看看。", nil))
		return
	}

	// Run to completion even if she navigates away mid-review — same
	// reasoning and same 150s cap as postLiteWritingTurn / generateWritingOutline.
	turnCtx, cancel := context.WithTimeout(context.WithoutCancel(r.Context()), 150*time.Second)
	defer cancel()

	wr, err := a.d.Queries.GetWriting(turnCtx, at.ID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}

	// §model-routing · review. Reviewing the whole piece is reviewer-tier work,
	// same "faithful, never downgrade" reasoning generateWritingOutline
	// applies — so this resolves EvalResolver (flagship) rather than
	// writing_turn.go's chaperone ChatResolver.
	resolved, ok := a.route(turnCtx, gateway.ClassReview)
	if !ok {
		slog.Warn("writing review: no provider resolved",
			"atom_id", at.ID, "request_id", httpx.RequestIDFromContext(r.Context()))
		httpx.WriteError(w, r, httpx.ErrAIDialogueFailed("model_unavailable"))
		return
	}

	res, cerr := gateway.Collect(turnCtx, a.d.Provider, resolved, gateway.ChatRequest{
		Messages: []gateway.ChatMessage{
			{Role: gateway.RoleSystem, Content: buildWritingCommentSystem(wr.Lang, writingDraftReviewMaxIssues)},
			{Role: gateway.RoleUser, Content: buildWritingCommentPrompt(wr, "她的整篇稿子", body)},
		},
	})

	// Meter BEFORE any bail — a call that reached the provider cost money
	// whatever happens to its reply, mirroring every other lite generate
	// endpoint.
	a.recordLiteLLMCall(turnCtx, u.ID, at.ID, "review", resolved, res.Usage)

	if cerr != nil {
		slog.Warn("writing review: provider call failed",
			"err", cerr, "atom_id", at.ID, "request_id", httpx.RequestIDFromContext(r.Context()))
		httpx.WriteError(w, r, httpx.ErrAIDialogueFailed("model_unavailable"))
		return
	}
	parsed, okParse := parseWritingComment(res.Text)
	if !okParse {
		slog.Warn("writing review: reply unparseable or empty summary",
			"atom_id", at.ID, "request_id", httpx.RequestIDFromContext(r.Context()))
		httpx.WriteError(w, r, httpx.ErrAIDialogueFailed("model_unavailable"))
		return
	}

	points := validateCommentPoints(parsed.Points, body, wr.Lang, writingDraftReviewMaxIssues)
	payload, merr := json.Marshal(points)
	if merr != nil {
		slog.Warn("writing review: marshal points failed", "err", merr, "atom_id", at.ID)
		httpx.WriteError(w, r, httpx.ErrAIDialogueFailed("model_unavailable"))
		return
	}
	row, serr := a.d.Queries.CreateWritingComment(turnCtx, sqlc.CreateWritingCommentParams{
		AtomID:    at.ID,
		SnippetID: pgtype.UUID{Valid: false},
		Scope:     "draft",
		Summary:   strings.TrimSpace(parsed.Summary),
		Points:    payload,
	})
	if serr != nil {
		httpx.WriteError(w, r, serr)
		return
	}

	// The comment is persisted (writing_comment, scope='draft') but
	// writing_draft.body is never touched — see the handler's comment.
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"comment": toCommentDTO(row)})
}

// finishWritingAtom is POST /api/v1/writings/{id}/finish — the terminal
// state for a writing.
//
// loadOwnedWritingAtomRow, NOT loadOwnedWritingAtom: this endpoint must stay
// callable (idempotently) after the writing is already finished — the exact
// exemption the finished-write gate's own doc comment carves out (readings.go),
// mirroring finishReading.
//
// Gate: the draft must have real content before it can be finished — an
// empty writing_draft.body (or no row at all) is 400 missing_draft, checked
// BEFORE any state changes, mirroring finishReading's missing_takeaway gate.
//
// SetWritingFinished is itself idempotent (`AND status <> 'finished'` —
// writing_atom.sql's comment): a second POST answers 200 with the unchanged
// row rather than re-stamping finished_at. 铁律④ makes finished_at evidence
// — a replayed request must not be able to move it hours later.
//
// Deliberately does NOT call SetWritingStage or otherwise touch stage — per
// this task's product-owner note, status ("is this piece done") and stage
// ("how far through the four steps did she actually get") are independent
// facts, and TestWritingStore_FinishDoesNotForceStage (writing_store_test.go)
// already pins that independence at the store layer. Coupling them here
// would defeat that guarantee at the API layer even though the store layer
// holds it.
func (a *API) finishWritingAtom(w http.ResponseWriter, r *http.Request) {
	at, ok := a.loadOwnedWritingAtomRow(w, r)
	if !ok {
		return
	}
	draft, err := a.d.Queries.GetWritingDraft(r.Context(), at.ID)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		httpx.WriteError(w, r, err)
		return
	}
	if strings.TrimSpace(draft.Body) == "" {
		httpx.WriteError(w, r, httpx.ErrBadRequest("missing_draft", "先完成初稿，再点完成。", nil))
		return
	}
	if err := a.d.Queries.SetWritingFinished(r.Context(), at.ID); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	a.EnqueueHarvest(r.Context(), at.ID) // 见 interest_jobs.go
	wr, err := a.d.Queries.GetWriting(r.Context(), at.ID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, writingDTOOf(wr, at.CreatedAt, at.LastActivityAt))
}
