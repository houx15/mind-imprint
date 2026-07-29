package api

// exploration_card.go — S4 · the 兔子洞 (rabbit-hole) card's persistence path,
// deferred from S3 (ExplorationView's TODO(S4)). The rabbit-hole card is a
// legacy spec: no completion predicate, no graph_effects, no material anchor —
// so it mints no graph node (CompleteCard would have nothing to promote).
// Persisting it honestly therefore means: store the student's completed
// reflection as a durable card_instance (status=completed) AND drop a
// rabbit_hole_logged event into the process-event stream, which the process-tree
// projection reads. That keeps the submit copy honest — the reflection is
// recorded (过程即数据), not silently discarded as it was in S3.

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/google/uuid"

	"mindimprint/api/internal/agent"
	"mindimprint/api/internal/httpx"
)

// postExplorationRabbitHole persists one completed rabbit-hole card envelope.
// Ownership-gated (404-no-leak via loadOwnedProject); no LLM spend, so no
// entitlement gate. Validates the standard envelope outer shape (field_values
// object; event_trace, when present, a typed array) exactly like
// submitProjectCard, then creates → submits → completes the instance and logs
// the process event. Returns the created card_instance id.
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
	if err := validateFieldValues(body.FieldValues); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	trace := body.EventTrace
	if len(trace) == 0 {
		trace = []byte("[]")
	} else if err := validateEventTrace(trace); err != nil {
		httpx.WriteError(w, r, err)
		return
	}

	store := agent.NewSqlcAgentStore(a.d.Queries, a.d.Pool)
	// Project-scoped (no material → uuid.Nil), so parent_node_id stays NULL: this
	// is a top-level reflection, not anchored to a source.
	row, err := store.CreateCardInstance(r.Context(), projectID, uuid.Nil, "rabbit-hole", "")
	if err != nil {
		httpx.WriteError(w, r, httpx.ErrInternal())
		return
	}
	if err := store.SubmitProjectCardInstance(r.Context(), projectID, row.ID, body.FieldValues, trace); err != nil {
		httpx.WriteError(w, r, httpx.ErrInternal())
		return
	}
	if err := store.SetCardInstanceStatus(r.Context(), projectID, row.ID, "completed"); err != nil {
		httpx.WriteError(w, r, httpx.ErrInternal())
		return
	}
	if err := store.AppendEvent(r.Context(), agent.EventRow{
		ProjectID: projectID, Surface: "studio", Type: "rabbit_hole_logged",
		Payload: mustJSON(map[string]any{"cardInstanceId": row.ID.String()}),
	}); err != nil {
		slog.Warn("rabbit-hole: append rabbit_hole_logged event failed",
			"err", err, "request_id", httpx.RequestIDFromContext(r.Context()))
	}

	httpx.WriteJSON(w, http.StatusOK, map[string]any{"cardInstanceId": row.ID.String()})
}
