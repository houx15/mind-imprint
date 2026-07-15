package api

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"github.com/google/uuid"

	"mindimprint/api/internal/agent"
	"mindimprint/api/internal/gateway"
	"mindimprint/api/internal/httpx"
	"mindimprint/api/internal/skills"
	"mindimprint/api/internal/store/sqlc"
)

// putEditBuffer upserts the student's silent edit buffer. Student text ONLY —
// there is no path for AI output to reach this handler (RL-1). No entitlement
// gate: no model call, no network.
func (a *API) putEditBuffer(w http.ResponseWriter, r *http.Request) {
	projectID, ok := a.loadOwnedProject(w, r)
	if !ok {
		return
	}
	var body struct {
		Content string `json:"content"`
	}
	if err := decodeJSON(r, &body); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if err := a.d.Queries.UpsertEditBuffer(r.Context(), sqlc.UpsertEditBufferParams{
		ProjectID: projectID, Content: body.Content,
	}); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// snapshotResp is the committed-snapshot wire shape.
type snapshotResp struct {
	ID        string `json:"id"`
	Seq       int32  `json:"seq"`
	WordCount int    `json:"word_count"`
	InBand    bool   `json:"in_band"`
}

// commitSnapshot mints an immutable draft snapshot from the posted content
// (from the buffer or a paste — identical object, spec §10). In one
// transaction: insert the snapshot, mint OR remove the word_budget_ok node
// per the word count vs the skill band, and append a version_saved event.
func (a *API) commitSnapshot(w http.ResponseWriter, r *http.Request) {
	projectID, ok := a.loadOwnedProject(w, r)
	if !ok {
		return
	}
	var body struct {
		Content string `json:"content"`
	}
	if err := decodeJSON(r, &body); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if len(body.Content) == 0 {
		httpx.WriteError(w, r, httpx.ErrBadRequest("validation_failed", "草稿是空的，先写点东西再提交。", nil))
		return
	}

	sk, _ := skills.ByID("writing-project")
	wc := agent.CountWords(body.Content)
	inBand := sk.WordBudget != nil && wc >= sk.WordBudget.Min && wc <= sk.WordBudget.Max

	tx, err := a.d.Pool.Begin(r.Context())
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	defer func() { _ = tx.Rollback(r.Context()) }()
	qtx := a.d.Queries.WithTx(tx)

	// NextSnapshotSeq (MAX(seq)+1) then InsertDraftSnapshot is TOCTOU-racy
	// under concurrent commits to the same project — the unique
	// (project_id,seq) index makes the loser of a race collide/error. That's
	// acceptable here: a single student edits their own draft sequentially,
	// there is no concurrent-writer scenario to protect against.
	seq, err := qtx.NextSnapshotSeq(r.Context(), projectID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	spanIndex := paragraphSpanIndex(body.Content) // []byte JSON
	snap, err := qtx.InsertDraftSnapshot(r.Context(), sqlc.InsertDraftSnapshotParams{
		ProjectID: projectID, Seq: seq, Content: body.Content, SpanIndex: spanIndex,
	})
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}

	// Mint or remove the word_budget_ok node so the S5 machine gate reflects
	// the LATEST snapshot honestly.
	if err := reconcileWordBudgetNode(r.Context(), qtx, projectID, inBand, wc, sk.WordBudget); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if err := tx.Commit(r.Context()); err != nil {
		httpx.WriteError(w, r, err)
		return
	}

	// Best-effort event (never fails the commit).
	store := agent.NewSqlcAgentStore(a.d.Queries, a.d.Pool)
	if err := store.AppendEvent(r.Context(), agent.EventRow{
		ProjectID: projectID, Surface: "studio", Type: "version_saved",
		Payload: mustJSON(map[string]any{"snapshot_id": snap.ID.String(), "seq": seq, "word_count": wc}),
	}); err != nil {
		slog.Warn("commit snapshot: append version_saved event failed", "err", err)
	}

	httpx.WriteJSON(w, http.StatusCreated, snapshotResp{
		ID: snap.ID.String(), Seq: seq, WordCount: wc, InBand: inBand,
	})
}

// paragraphSpanIndex records each paragraph's [start,end) rune offsets so the
// snapshot is a span-indexed material (product spec §... "Material | Any text
// with span indices: a source, or a draft snapshot"). Paragraphs split on
// blank lines; single newlines stay within a paragraph.
func paragraphSpanIndex(s string) []byte {
	type span struct {
		Start int `json:"start"`
		End   int `json:"end"`
	}
	spans := []span{}
	runes := []rune(s)
	start := 0
	for i := 0; i <= len(runes); i++ {
		atBreak := i == len(runes) || (i+1 < len(runes) && runes[i] == '\n' && runes[i+1] == '\n')
		if atBreak {
			if i > start {
				spans = append(spans, span{Start: start, End: i})
			}
			start = i + 2
			i++
		}
	}
	b, _ := json.Marshal(spans)
	return b
}

// reconcileWordBudgetNode makes the word_budget_ok node present iff inBand.
// author:"ai" matches the gate_state/plan system-node convention (agentstore.go);
// it is a typed marker, not prose.
func reconcileWordBudgetNode(ctx context.Context, q *sqlc.Queries, projectID uuid.UUID, inBand bool, wc int, band *skills.WordBudget) error {
	existing, err := q.ListGraphNodesByProject(ctx, projectID)
	if err != nil {
		return err
	}
	var okID *uuid.UUID
	for i := range existing {
		if existing[i].Type == "word_budget_ok" {
			id := existing[i].ID
			okID = &id
			break
		}
	}
	if inBand {
		if okID != nil {
			return nil // already present
		}
		bodyMap := map[string]any{"word_count": wc}
		if band != nil {
			bodyMap["min"], bodyMap["max"] = band.Min, band.Max
		}
		_, err := q.InsertGraphNode(ctx, sqlc.InsertGraphNodeParams{
			ProjectID: projectID, Type: "word_budget_ok",
			Body: mustJSON(bodyMap), Author: "ai",
		})
		return err
	}
	if okID != nil {
		return q.DeleteGraphNode(ctx, sqlc.DeleteGraphNodeParams{ID: *okID, ProjectID: projectID})
	}
	return nil
}

func mustJSON(v any) []byte { b, _ := json.Marshal(v); return b }

// attestGate records a student_written gate item as solid (or clears it). The
// gate_state STORAGE already exists (UpsertGateState); nothing else records a
// student_written item as solid — the planner's Advance deliberately never
// marks non-machine items. Restricted to the contract's own student_written
// item names so a caller cannot forge a machine/human item. No model call.
func (a *API) attestGate(w http.ResponseWriter, r *http.Request) {
	projectID, ok := a.loadOwnedProject(w, r)
	if !ok {
		return
	}
	contractID := r.PathValue("contractId")
	var body struct {
		Item      string `json:"item"`
		Confirmed bool   `json:"confirmed"`
	}
	if err := decodeJSON(r, &body); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	sk, ok2 := skills.ByID("writing-project")
	if !ok2 {
		httpx.WriteError(w, r, httpx.ErrInternal())
		return
	}
	c, ok3 := sk.Contracts[contractID]
	if !ok3 {
		httpx.WriteError(w, r, httpx.ErrNotFound("资源不存在"))
		return
	}
	allowed := false
	for _, name := range c.Gate.StudentWritten {
		if name == body.Item {
			allowed = true
			break
		}
	}
	if !allowed {
		httpx.WriteError(w, r, httpx.ErrBadRequest("validation_failed", "该条目不是学生自评项", nil))
		return
	}

	store := agent.NewSqlcAgentStore(a.d.Queries, a.d.Pool)
	recorded, err := store.ListGateStates(r.Context(), projectID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	rec := recorded[contractID]
	if rec.Items == nil {
		rec.Items = map[string]string{}
	}
	if body.Confirmed {
		rec.Items[body.Item] = "solid"
	} else {
		delete(rec.Items, body.Item)
	}
	if err := store.UpsertGateState(r.Context(), projectID, contractID, rec); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// orderReview runs the student-triggered whole-draft review over a committed
// snapshot. One snapshot, one review (spec §12): if review_item interventions
// already anchor this snapshot, stream them back — NO second model call. The
// review writes ONLY intervention rows (typed advice), never prose (RL-1).
func (a *API) orderReview(w http.ResponseWriter, r *http.Request) {
	projectID, ok := a.loadOwnedProject(w, r)
	if !ok {
		return
	}
	u, _ := UserFromContext(r.Context())
	sid, err := uuid.Parse(r.PathValue("sid"))
	if err != nil {
		httpx.WriteError(w, r, httpx.ErrNotFound("资源不存在"))
		return
	}
	snap, err := a.d.Queries.GetSnapshot(r.Context(), sqlc.GetSnapshotParams{ID: sid, ProjectID: projectID})
	if err != nil {
		httpx.WriteError(w, r, httpx.ErrNotFound("资源不存在"))
		return
	}
	voice := agent.ParseVoice(r.URL.Query().Get("voice"))

	store := agent.NewSqlcAgentStore(a.d.Queries, a.d.Pool)
	existing := reviewItemsForSnapshot(r.Context(), a.d.Queries, projectID, sid, voice)

	// Entitlement gate BEFORE the stream — only when a model call will happen.
	if len(existing) == 0 {
		entitled, eerr := HasEntitlement(r.Context(), u)
		if eerr != nil {
			httpx.WriteError(w, r, eerr)
			return
		}
		if !entitled {
			httpx.WriteError(w, r, httpx.ErrNotEntitled())
			return
		}
	}

	sse, err := gateway.NewSSEWriter(w)
	if err != nil {
		httpx.WriteError(w, r, httpx.ErrInternal())
		return
	}
	em := &studioEmitter{sse: sse}
	stop, hbDone := startHeartbeat(r.Context(), em)
	defer func() { close(stop); <-hbDone }()

	if len(existing) > 0 { // idempotent replay — no second model call.
		_ = em.Review(mustJSON(existing))
		_ = em.Done()
		return
	}

	sk, _ := skills.ByID("writing-project")
	resolved, rerr := a.d.ChatResolver(r.Context())
	if rerr != nil {
		_ = em.ErrorEnvelope("internal_error", "体检失败，请重试")
		_ = em.Done()
		return
	}
	paras := snapshotParagraphs(snap.Content)
	sbState, _ := agent.BudgetVerdict(agent.CountWords(snap.Content), sk.WordBudget)
	items, usage, perr := agent.ProposeReview(r.Context(), a.d.Provider, resolved, sk.ReviewCriteria, paras, graphSummary(r.Context(), a.d.Queries, projectID), voice, sbState == "over")
	// Record the call cost even if enforcement then rejected the output — a
	// rejected call still cost money.
	if resolved.Provider != "" {
		if err := store.RecordLLMCall(r.Context(), agent.LLMCallRow{
			ProjectID: projectID, Surface: "studio", Purpose: "order_review",
			Resolved: resolved, PromptTokens: int32(usage.InputTokens), CompletionTokens: int32(usage.OutputTokens),
		}); err != nil {
			slog.Warn("order_review: record llm call", "err", err)
		}
	}
	if perr != nil {
		// Enforcement rejection or parse failure: persist NOTHING, stream an
		// error envelope + done.
		slog.Warn("order_review: proposal rejected", "err", perr)
		_ = em.ErrorEnvelope("review_rejected", "这次体检没通过内部校验，请再试一次")
		_ = em.Done()
		return
	}

	// Persist each item as a review_item intervention anchored to the
	// snapshot. The FULL ReviewItem is marshalled into intervention.body
	// (lossless reconstruction); criterion/level stay duplicated in their
	// flat columns for any SQL that filters on them.
	anchor := mustJSON(map[string]string{"kind": "draft_snapshot", "id": sid.String(), "voice": string(voice)})
	persisted := make([]agent.ReviewItem, 0, len(items))
	for _, it := range items {
		if err := store.InsertReviewIntervention(r.Context(), agent.ReviewInterventionRow{
			ProjectID: projectID, Anchor: anchor,
			Criterion: it.CriterionCode + " " + it.CriterionName,
			Body:      string(mustJSON(it)), Level: it.Band,
		}); err != nil {
			slog.Warn("order_review: persist item", "err", err)
			continue
		}
		persisted = append(persisted, it)
	}

	// Ordering a review satisfies the S5 HUMAN gate item whole_draft_review —
	// but only if at least one item actually persisted. If every insert above
	// failed, `persisted` is empty: the gate must NOT be marked solid (the
	// gate and the UI would otherwise disagree — solid gate, zero visible
	// review items) and the review_ordered event must NOT fire (so the next
	// order attempt still sees existing==0 above and re-runs the review
	// instead of silently no-op'ing forever on a phantom "already ordered"
	// event with nothing to show for it). Like citations_matched (attestGate),
	// nothing else records a human item as solid, so do it here (best-effort
	// — never fails the stream). Guard on the item actually being in
	// draft_polish's Gate.Human.
	if len(persisted) > 0 {
		if c, ok := sk.Contracts["draft_polish"]; ok {
			for _, hi := range c.Gate.Human {
				if hi == "whole_draft_review" {
					recorded, gerr := store.ListGateStates(r.Context(), projectID)
					if gerr != nil {
						slog.Warn("order_review: list gate states", "err", gerr)
						break
					}
					rec := recorded["draft_polish"]
					if rec.Items == nil {
						rec.Items = map[string]string{}
					}
					rec.Items["whole_draft_review"] = "solid"
					if err := store.UpsertGateState(r.Context(), projectID, "draft_polish", rec); err != nil {
						slog.Warn("order_review: record whole_draft_review", "err", err)
					}
					break
				}
			}
		}

		if err := store.AppendEvent(r.Context(), agent.EventRow{
			ProjectID: projectID, Surface: "studio", Type: "review_ordered",
			Payload: mustJSON(map[string]any{"snapshot_id": sid.String(), "items": len(persisted)}),
		}); err != nil {
			slog.Warn("order_review: append event", "err", err)
		}
	}
	if err := a.d.Queries.TouchProject(r.Context(), projectID); err != nil {
		slog.Warn("order_review: touch project", "err", err)
	}
	_ = em.Review(mustJSON(persisted))
	_ = em.Done()
}

// snapshotParagraphs splits a committed snapshot's content into non-empty
// paragraphs on blank lines — the same paragraph unit ProposeReview reasons
// over (mirrors paragraphSpanIndex's break rule, but returns text, not spans).
func snapshotParagraphs(content string) []string {
	out := []string{}
	for _, p := range strings.Split(content, "\n\n") {
		if s := strings.TrimSpace(p); s != "" {
			out = append(out, s)
		}
	}
	return out
}

// reviewItemsForSnapshot returns the persisted review items anchored to sid,
// reconstructed from their intervention rows (empty if none — the
// not-yet-reviewed state, which is what makes a review idempotent: seeing
// none here is exactly the signal to call the model, seeing any is the
// signal to replay them instead).
func reviewItemsForSnapshot(ctx context.Context, q *sqlc.Queries, projectID, sid uuid.UUID, voice agent.Voice) []agent.ReviewItem {
	ivs, err := q.ListInterventionsByProject(ctx, projectID)
	if err != nil {
		return nil
	}
	out := []agent.ReviewItem{}
	for _, iv := range ivs {
		if iv.Type != "review_item" {
			continue
		}
		var anchor struct{ Kind, ID, Voice string }
		if err := json.Unmarshal(iv.Anchor, &anchor); err != nil {
			continue
		}
		// A keystone row has no anchor voice — read it as board.
		av := anchor.Voice
		if av == "" {
			av = string(agent.VoiceBoard)
		}
		if anchor.Kind != "draft_snapshot" || anchor.ID != sid.String() || av != string(voice) {
			continue
		}
		if it, ok := reviewItemFromIntervention(iv); ok {
			out = append(out, it)
		}
	}
	return out
}

// reviewItemFromIntervention reconstructs the full agent.ReviewItem from an
// intervention row's body — the body IS the marshalled ReviewItem (Task 6),
// so reconstruction is one json.Unmarshal, never string-splitting.
func reviewItemFromIntervention(iv sqlc.Intervention) (agent.ReviewItem, bool) {
	var it agent.ReviewItem
	if err := json.Unmarshal([]byte(iv.Body), &it); err != nil {
		return agent.ReviewItem{}, false
	}
	return it, true
}

// graphSummary counts the project's claim/evidence graph nodes into a short
// string ProposeReview's prompt carries as argument context (e.g. "主张 1 ·
// 证据 2") — never an error: a read failure just yields an empty summary,
// the same "never fail the turn over enrichment" posture surfaceAnchors uses.
func graphSummary(ctx context.Context, q *sqlc.Queries, projectID uuid.UUID) string {
	nodes, err := q.ListGraphNodesByProject(ctx, projectID)
	if err != nil {
		return ""
	}
	claims, evidence := 0, 0
	for _, n := range nodes {
		switch n.Type {
		case "claim":
			claims++
		case "evidence":
			evidence++
		}
	}
	return fmt.Sprintf("主张 %d · 证据 %d", claims, evidence)
}
