package api

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/google/uuid"

	"mindimprint/api/internal/agent"
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
