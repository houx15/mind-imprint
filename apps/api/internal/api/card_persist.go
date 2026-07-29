package api

// card_persist.go — S4 · the honest completion path for a card the coach
// PROPOSED cross-phase (克制 summon rung) and the student then chose to open and
// fill. Like the rabbit-hole card, these are recorded as durable card_instances
// (status=completed) + a process event, so 打开→填写→提交 is real, not the
// silently-discarded submit S3 rightly refused to ship. Only cards the coach is
// actually allowed to propose (coachProposableCards) may be persisted this way,
// so this endpoint can never be used to forge arbitrary card state.

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/google/uuid"

	"mindimprint/api/internal/agent"
	"mindimprint/api/internal/httpx"
)

// coachProposableCards is the allowlist of card ids the cross-phase proposer may
// offer (the moment classifier's vocabulary, moment.go) — the ONLY ids this
// generic persist path accepts. Keeps the endpoint from persisting a card the
// coach never proposes.
var coachProposableCards = map[string]bool{
	"fact-opinion-value": true,
	"certainty-spectrum": true,
	"steelman":           true,
}

// writingDeckCards is the allowlist of writing thinking-cards the STUDENT may
// summon in the Write room (WC · part-by-part). Persisted the same way (a
// completed card_instance + event, no node mint for the legacy ones) — the
// student fills them to think through an argument; 印记 never writes the body.
var writingDeckCards = map[string]bool{
	"toulmin":      true,
	"pee":          true,
	"concession":   true,
	"steelman":     true,
	"argument-map": true,
}

// persistableCard reports whether card_id may be persisted through the generic
// path — either an AI-proposable card or a student-summonable writing-deck card.
func persistableCard(id string) bool {
	return coachProposableCards[id] || writingDeckCards[id]
}

// persistProjectCardEnvelope creates → submits → completes a project-scoped
// (no-material) card_instance for cardID and logs eventType. Validates the
// envelope outer shape (field_values object; event_trace, when present, a typed
// array) exactly like submitProjectCard. Returns the created card_instance id.
// Shared by the rabbit-hole handler and the generic proposal-persist handler.
func (a *API) persistProjectCardEnvelope(ctx context.Context, projectID uuid.UUID, cardID string, fieldValues, eventTrace json.RawMessage, eventType string) (uuid.UUID, error) {
	if err := validateFieldValues(fieldValues); err != nil {
		return uuid.Nil, err
	}
	trace := eventTrace
	if len(trace) == 0 {
		trace = []byte("[]")
	} else if err := validateEventTrace(trace); err != nil {
		return uuid.Nil, err
	}

	store := agent.NewSqlcAgentStore(a.d.Queries, a.d.Pool)
	// Project-scoped (uuid.Nil material) → parent_node_id stays NULL: a top-level
	// record, not anchored to a source.
	row, err := store.CreateCardInstance(ctx, projectID, uuid.Nil, cardID, "")
	if err != nil {
		return uuid.Nil, httpx.ErrInternal()
	}
	if err := store.SubmitProjectCardInstance(ctx, projectID, row.ID, fieldValues, trace); err != nil {
		return uuid.Nil, httpx.ErrInternal()
	}
	if err := store.SetCardInstanceStatus(ctx, projectID, row.ID, "completed"); err != nil {
		return uuid.Nil, httpx.ErrInternal()
	}
	if err := store.AppendEvent(ctx, agent.EventRow{
		ProjectID: projectID, Surface: "studio", Type: eventType,
		Payload: mustJSON(map[string]any{"cardInstanceId": row.ID.String(), "cardId": cardID}),
	}); err != nil {
		slog.Warn("card persist: append event failed", "err", err, "type", eventType)
	}
	return row.ID, nil
}

// postDismissProposal records the student declining a coach card offer as a
// skipped card_instance. EligibleMoments treats a card with an instance in ANY
// status (including skipped) as ineligible, so this makes the coach STOP
// offering that card — 铁律 2 · 不操纵: once she says no, we do not ask again.
// Without this the dismiss was client-only and the coach re-offered the same
// card every qualifying turn (whole-branch review IMPORTANT 2). Ownership-gated;
// no spend; allowlist-enforced.
func (a *API) postDismissProposal(w http.ResponseWriter, r *http.Request) {
	projectID, ok := a.loadOwnedProject(w, r)
	if !ok {
		return
	}
	var body struct {
		CardID string `json:"card_id"`
	}
	if err := decodeJSON(r, &body); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if !coachProposableCards[body.CardID] {
		httpx.WriteError(w, r, httpx.ErrBadRequest("validation_failed", "card_id 不在可提议卡片范围内", nil))
		return
	}
	store := agent.NewSqlcAgentStore(a.d.Queries, a.d.Pool)
	row, err := store.CreateCardInstance(r.Context(), projectID, uuid.Nil, body.CardID, "")
	if err != nil {
		httpx.WriteError(w, r, httpx.ErrInternal())
		return
	}
	if err := store.SetCardInstanceStatus(r.Context(), projectID, row.ID, "skipped"); err != nil {
		httpx.WriteError(w, r, httpx.ErrInternal())
		return
	}
	if err := store.AppendEvent(r.Context(), agent.EventRow{
		ProjectID: projectID, Surface: "studio", Type: "coach_proposal_skipped",
		Payload: mustJSON(map[string]any{"cardId": body.CardID}),
	}); err != nil {
		slog.Warn("dismiss proposal: append event failed", "err", err)
	}
	w.WriteHeader(http.StatusNoContent)
}

// postPersistProjectCard persists a completed envelope for a card the coach
// proposed cross-phase. Ownership-gated; no LLM spend. card_id must be in the
// proposable allowlist (else 400).
func (a *API) postPersistProjectCard(w http.ResponseWriter, r *http.Request) {
	projectID, ok := a.loadOwnedProject(w, r)
	if !ok {
		return
	}
	var body struct {
		CardID      string          `json:"card_id"`
		FieldValues json.RawMessage `json:"field_values"`
		EventTrace  json.RawMessage `json:"event_trace"`
	}
	if err := decodeJSON(r, &body); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if !persistableCard(body.CardID) {
		httpx.WriteError(w, r, httpx.ErrBadRequest("validation_failed", "card_id 不在可持久化的卡片范围内", nil))
		return
	}
	id, err := a.persistProjectCardEnvelope(r.Context(), projectID, body.CardID, body.FieldValues, body.EventTrace, "card_logged")
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"cardInstanceId": id.String()})
}
