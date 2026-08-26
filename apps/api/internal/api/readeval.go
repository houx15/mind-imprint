package api

// readeval.go — Task 5: POST /projects/{id}/cards/{cid}/evaluate, the
// selection-evaluate endpoint for a reading-room card: the student picked a
// sentence out of the material as her evidence for the card's active
// dimension, and this judges it against the card's lens (agent.EvaluateSelection,
// Task 4's pure core). This does NOT flip the card_instance's status — the
// existing submitProjectCard (projectcards.go) is still the confirm/save
// path; this only records the eval for that later step to consolidate.

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"mindimprint/api/internal/agent"
	"mindimprint/api/internal/cards"
	"mindimprint/api/internal/httpx"
)

// evaluateSelectionReq is evaluateProjectCard's request body: the span the
// student picked in the material as her evidence, plus which dimension of the
// active card she's answering.
type evaluateSelectionReq struct {
	BlockID   string `json:"block_id"`
	Start     int    `json:"start"`
	End       int    `json:"end"`
	Quote     string `json:"quote"`
	Dimension string `json:"dimension"`
}

// selectionCheckDTO is one SelectionCheck on the wire — camelCase, since the
// TS side Zod-parses this response directly (Task 8).
type selectionCheckDTO struct {
	Key         string `json:"key"`
	Label       string `json:"label"`
	Status      string `json:"status"`
	Evidence    string `json:"evidence"`
	Explanation string `json:"explanation"`
}

// selectionEvalDTO is agent.SelectionEval's wire/persistence shape. Defined
// here rather than adding json tags to the agent type: SelectionEval is pure
// agent-package output with no wire concept of its own, and camelCase is a
// property of THIS endpoint's contract, not of the type. Reused for both the
// HTTP response and the framework_fill persistence write, so the two can
// never drift.
type selectionEvalDTO struct {
	Verdict       string              `json:"verdict"`
	VerdictLabel  string              `json:"verdictLabel"`
	VerdictReason string              `json:"verdictReason"`
	Checks        []selectionCheckDTO `json:"checks"`
	Finding       string              `json:"finding"`
	Judgment      string              `json:"judgment"`
	Support       string              `json:"support"`
	Caveat        string              `json:"caveat"`
	NextStep      string              `json:"nextStep"`
	SpanIDs       []string            `json:"spanIds"`
	// degraded — true when NO model produced this review (agent.fallbackEval's
	// canned text). It is persisted into framework_fill along with everything
	// else, which is the point: a later report reads framework_fill's `finding`
	// as the student's own reading of her sentence, and must be able to exclude
	// the ones no model ever read. The client's Zod SelectionEval schema is
	// non-strict, so the extra key is simply ignored there.
	Degraded bool `json:"degraded"`
}

func toSelectionEvalDTO(e agent.SelectionEval) selectionEvalDTO {
	checks := make([]selectionCheckDTO, 0, len(e.Checks))
	for _, c := range e.Checks {
		checks = append(checks, selectionCheckDTO{
			Key: c.Key, Label: c.Label, Status: c.Status,
			Evidence: c.Evidence, Explanation: c.Explanation,
		})
	}
	spanIDs := e.SpanIDs
	if spanIDs == nil {
		spanIDs = []string{}
	}
	return selectionEvalDTO{
		Verdict: e.Verdict, VerdictLabel: e.VerdictLabel, VerdictReason: e.VerdictReason,
		Checks: checks, Finding: e.Finding, Judgment: e.Judgment, Support: e.Support,
		Caveat: e.Caveat, NextStep: e.NextStep, SpanIDs: spanIDs,
		Degraded: e.Degraded,
	}
}

// evaluateProjectCard judges the student's picked sentence against the active
// reading-room card's lens. loadOwnedProjectCard both hides a non-owned
// project as 404 AND scopes {cid} to it (the same IDOR guard
// activateProjectCard/skipProjectCard use) — no separate ownership check
// needed here.
func (a *API) evaluateProjectCard(w http.ResponseWriter, r *http.Request) {
	projectID, cid, ok := a.loadOwnedProjectCard(w, r)
	if !ok {
		return
	}
	u, _ := UserFromContext(r.Context())

	// Entitlement gate — this endpoint spends a flagship-tier call.
	entitled, err := HasEntitlement(r.Context(), u)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if !entitled {
		httpx.WriteError(w, r, httpx.ErrNotEntitled())
		return
	}

	var body evaluateSelectionReq
	if err := decodeJSON(r, &body); err != nil {
		httpx.WriteError(w, r, err)
		return
	}

	store := agent.NewSqlcAgentStore(a.d.Queries, a.d.Pool)
	ci, err := store.GetCardInstance(r.Context(), cid)
	if err != nil {
		httpx.WriteError(w, r, httpx.ErrNotFound("资源不存在"))
		return
	}

	// The card's material id isn't a CardInstanceRow field — read it off the
	// example anchor postReadingTurn persisted when it summoned this card
	// (SetCardInstanceAnchors), the only place that id was ever recorded for
	// a reading-room card_instance. A missing/malformed anchor set degrades
	// to "" (the student's span is still evaluated; it just carries no
	// material attribution).
	materialID := ""
	var seedAnchors []agent.Anchor
	if json.Unmarshal(ci.Anchors, &seedAnchors) == nil && len(seedAnchors) > 0 {
		materialID = seedAnchors[0].MaterialID
	}

	studentSpan := agent.Anchor{
		ID: "sel0", MaterialID: materialID, BlockID: body.BlockID,
		Start: body.Start, End: body.End, Quote: body.Quote,
		Dimension: body.Dimension, Author: "student",
	}

	spec, _ := cards.ByID(ci.CardID)

	eval, resolved, usage, _ := agent.EvaluateSelection(r.Context(), a.d.Provider, a.d.EvalResolver, spec, body.Dimension, studentSpan)
	if resolved.Provider != "" {
		if rerr := store.RecordLLMCall(r.Context(), agent.LLMCallRow{
			ProjectID: projectID, Surface: "studio", Purpose: "read_eval",
			Resolved: resolved, PromptTokens: int32(usage.InputTokens), CompletionTokens: int32(usage.OutputTokens),
		}); rerr != nil {
			slog.Warn("evaluate selection: record llm usage failed",
				"err", rerr, "request_id", httpx.RequestIDFromContext(r.Context()))
		}
	}

	dto := toSelectionEvalDTO(eval)
	// Persist for the later confirm/mint (submitProjectCard consolidates
	// framework_fill; this endpoint never flips card_instance.status itself).
	if frameworkJSON, merr := json.Marshal(dto); merr == nil {
		if serr := store.SetCardInstanceFramework(r.Context(), projectID, cid, frameworkJSON); serr != nil {
			slog.Warn("evaluate selection: persist framework failed",
				"err", serr, "request_id", httpx.RequestIDFromContext(r.Context()))
		}
	}

	httpx.WriteJSON(w, http.StatusOK, dto)
}
