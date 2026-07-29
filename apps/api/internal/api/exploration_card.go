package api

// exploration_card.go — S4 · the 兔子洞 (rabbit-hole) card's persistence path,
// deferred from S3 (ExplorationView's TODO(S4)). The rabbit-hole card is a
// legacy spec: no completion predicate, no graph_effects, no material anchor —
// so it mints no graph node (CompleteCard would have nothing to promote).
// Persisting it honestly therefore means: store the student's completed
// reflection as a durable card_instance (status=completed) AND drop a process
// event into the event stream, which the process-tree projection reads. That
// keeps the submit copy honest — the reflection is recorded (过程即数据), not
// silently discarded as it was in S3. Shares persistProjectCardEnvelope with the
// generic card-persist path (card_persist.go) used by cross-phase proposals.

import (
	"encoding/json"
	"net/http"

	"mindimprint/api/internal/httpx"
)

// postExplorationRabbitHole persists one completed rabbit-hole card envelope.
// Ownership-gated (404-no-leak via loadOwnedProject); no LLM spend, so no
// entitlement gate.
func (a *API) postExplorationRabbitHole(w http.ResponseWriter, r *http.Request) {
	projectID, ok := a.loadOwnedProject(w, r)
	if !ok {
		return
	}
	var body struct {
		FieldValues json.RawMessage `json:"field_values"`
		EventTrace  json.RawMessage `json:"event_trace"`
	}
	if err := decodeJSON(r, &body); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	id, err := a.persistProjectCardEnvelope(r.Context(), projectID, "rabbit-hole", body.FieldValues, body.EventTrace, "rabbit_hole_logged")
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"cardInstanceId": id.String()})
}
