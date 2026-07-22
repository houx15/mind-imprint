package api

// declaration.go — Task 8: the S6 AI 使用申报单. reflect_archive's `human`
// gate item `declaration_signed` had no producer anywhere, so the last
// station could never complete. The declaration is four counters projected
// from data that already exists, which the student then signs. No model
// call, ever.

import (
	"encoding/json"
	"net/http"

	"mindimprint/api/internal/agent"
	"mindimprint/api/internal/httpx"
	"mindimprint/api/internal/store/sqlc"
	"mindimprint/api/internal/studio"
)

// DeclarationCounts is the AI 使用申报单's counters — persisted verbatim into
// the `declaration` graph_node signDeclaration mints, so the signature
// records exactly what was signed rather than a live view that keeps moving
// (过程即数据).
type DeclarationCounts struct {
	Asks             int `json:"asks"`
	Dispositions     int `json:"dispositions"`
	CardsSpontaneous int `json:"cardsSpontaneous"`
	CardsPrompted    int `json:"cardsPrompted"`
	// AiWrittenProse is always 0 — a claim by construction, not a
	// measurement. It holds because RL-1 makes the whole-draft review's
	// `fix` field advice that is never written back into the draft (the
	// review path only ever appends `review_item` interventions; nothing
	// writes to draft_snapshot/edit_buffer on the AI's behalf — see
	// writing.go/orderReview), and because no other code path places
	// model-generated text into the student's draft. If a future slice adds
	// ANY path that lets AI text reach the draft, this field must start
	// MEASURING that path's output rather than silently keep printing 0 —
	// delete this field rather than let the declaration go on lying.
	AiWrittenProse int `json:"aiWrittenProse"`
}

// countDeclaration derives the declaration's counters from data that already
// exists elsewhere in the graph — no model call, no new storage. Pure and
// DB-free so it is unit-testable directly over a hand-built
// studio.ProjectData.
func countDeclaration(d studio.ProjectData) DeclarationCounts {
	asks := 0
	for _, e := range d.Events {
		if e.Type == "prompt_sent" {
			asks++
		}
	}

	// Shared with projectEquipment (studio/projection.go) via
	// studio.NudgedCardInstanceIDs — one rule, one definition, so the two can
	// never disagree about a given card.
	nudged := studio.NudgedCardInstanceIDs(d)
	spont, prompted := 0, 0
	for _, ci := range d.Cards {
		if nudged[ci.ID.String()] {
			prompted++
		} else {
			spont++
		}
	}

	return DeclarationCounts{
		Asks:             asks,
		Dispositions:     len(d.Dispositions),
		CardsSpontaneous: spont,
		CardsPrompted:    prompted,
		AiWrittenProse:   0,
	}
}

// signDeclaration persists the S6 AI 使用申报单: the student's signature over
// counters computed from data that already exists (no model call).
//
// Its own endpoint rather than attestGate (writing.go): attestGate is
// deliberately restricted to a contract's student_written names so a caller
// cannot forge a machine or human item, and declaration_signed is a `human`
// item. signDeclaration recomputes the counters server-side, persists them
// into a `declaration` graph_node (author "student"), and only then sets
// reflect_archive's declaration_signed item solid — in ONE transaction
// (createProject's pattern, project_create.go): a signature recorded with no
// counters, or counters with no signature, are both worse than a clean
// failure. advanceGates runs AFTER the commit, exactly like every other
// gate-affecting write.
//
// Idempotent: a second sign reads back the already-recorded
// declaration_signed item inside the same transaction and returns without
// minting a second node.
func (a *API) signDeclaration(w http.ResponseWriter, r *http.Request) {
	projectID, ok := a.loadOwnedProject(w, r) // 404 hides other users'
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

	tx, err := a.d.Pool.Begin(r.Context())
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	defer func() { _ = tx.Rollback(r.Context()) }()
	qtx := a.d.Queries.WithTx(tx)
	store := agent.NewSqlcAgentStore(qtx, a.d.Pool)

	recorded, err := store.ListGateStates(r.Context(), projectID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	rec := recorded["reflect_archive"]
	alreadySigned := rec.Items != nil && rec.Items["declaration_signed"] == "solid"

	if !alreadySigned {
		// Not yet signed — mint the declaration node and flip the gate item,
		// both inside this same transaction.
		d, err := studio.Load(r.Context(), qtx, projectID)
		if err != nil {
			httpx.WriteError(w, r, err)
			return
		}
		nodeBody, err := json.Marshal(countDeclaration(d))
		if err != nil {
			httpx.WriteError(w, r, err)
			return
		}

		if _, err := qtx.InsertGraphNode(r.Context(), sqlc.InsertGraphNodeParams{
			ProjectID: projectID, Type: "declaration", Body: nodeBody, Author: "student", SpanRef: nil,
		}); err != nil {
			httpx.WriteError(w, r, err)
			return
		}

		if rec.Items == nil {
			rec.Items = map[string]string{}
		}
		rec.Items["declaration_signed"] = "solid"
		if err := store.UpsertGateState(r.Context(), projectID, "reflect_archive", rec); err != nil {
			httpx.WriteError(w, r, err)
			return
		}
	}
	// Already signed — idempotent no-op, never mint a second node, but still
	// commit + advanceGates below exactly like every other gate-affecting
	// write. advanceGates is cheap and pure (it recomputes gate state from
	// scratch); calling it unconditionally is what makes a crash between a
	// first sign's commit and its advanceGates call self-healing on the next
	// duplicate request instead of leaving reflect_archive's station badge
	// stuck forever.

	if err := tx.Commit(r.Context()); err != nil {
		httpx.WriteError(w, r, err)
		return
	}

	a.advanceGates(r.Context(), projectID)
	httpx.WriteJSON(w, http.StatusOK, map[string]any{})
}
