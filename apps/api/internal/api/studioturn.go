package api

import (
	"log/slog"
	"net/http"
	"sync"
	"time"

	"mindimprint/api/internal/agent"
	"mindimprint/api/internal/agent/enforcement"
	"mindimprint/api/internal/gateway"
	"mindimprint/api/internal/httpx"
	"mindimprint/api/internal/skills"
)

// studioEmitter serializes all SSE writes (intervention/gate/done deltas +
// heartbeats) under one mutex, since the heartbeat goroutine and
// postProjectTurn both write the same stream. Mirrors turn.go's syncEmitter,
// but over the Studio event vocabulary (Intervention/Gate) rather than the
// legacy Text/Card.
type studioEmitter struct {
	mu  sync.Mutex
	sse *gateway.SSEWriter
}

func (e *studioEmitter) Intervention(id, body, anchor, criterion, level string) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.sse.Intervention(id, body, anchor, criterion, level)
}

func (e *studioEmitter) Gate(contract, status string, passed, total int, missing []string) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.sse.Gate(contract, status, passed, total, missing)
}

func (e *studioEmitter) Done() error {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.sse.Done("")
}

func (e *studioEmitter) ErrorEnvelope(code, msg string) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.sse.ErrorEnvelope(code, msg)
}

func (e *studioEmitter) Heartbeat() error {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.sse.Heartbeat()
}

// studioSimilarity is the Studio turn endpoint's enforcement.Similarity seam:
// a keyless, offline lexical-overlap heuristic (no embedding call, no key, no
// network). A real embedding-backed Similarity is a separate, later task once
// an embedding provider is chosen.
func studioSimilarity() enforcement.Similarity { return enforcement.LexicalSimilarity{} }

// postProjectTurn drives the agent runtime one step (RunAgentStep) for a
// Studio project and streams the result over SSE: at most one intervention or
// gate event, then done. Card production is off (SkipSurfaceCards) — 5c's
// conversational loop defers card-surfacing to its own dedicated pass.
func (a *API) postProjectTurn(w http.ResponseWriter, r *http.Request) {
	projectID, ok := a.loadOwnedProject(w, r)
	if !ok {
		return
	}
	u, _ := UserFromContext(r.Context())

	// Entitlement gate — JSON error BEFORE committing to the stream.
	entitled, err := HasEntitlement(r.Context(), u)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if !entitled {
		httpx.WriteError(w, r, httpx.ErrNotEntitled())
		return
	}

	var body struct {
		UserInput string `json:"user_input"`
	}
	if err := decodeJSON(r, &body); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if len(body.UserInput) == 0 {
		// Same constructor decodeJSON itself uses for a malformed body — 400
		// validation_failed, just with a field-specific message.
		httpx.WriteError(w, r, httpx.ErrBadRequest("validation_failed", "user_input 不能为空", nil))
		return
	}

	resolved, err := a.d.ChatResolver(r.Context())
	if err != nil {
		httpx.WriteError(w, r, httpx.ErrInternal())
		return
	}

	// Commit to streaming. After this, errors are SSE error events, not JSON.
	sse, err := gateway.NewSSEWriter(w)
	if err != nil {
		httpx.WriteError(w, r, httpx.ErrInternal())
		return
	}
	em := &studioEmitter{sse: sse}

	// Heartbeat until the turn returns or the client disconnects — clones
	// turn.go's shape (postTurn) exactly: same 15s cadence, same
	// stop/hbDone handshake so a heartbeat write can never outlive the
	// handler (the ResponseWriter is invalid once ServeHTTP returns).
	stop := make(chan struct{})
	hbDone := make(chan struct{})
	go func() {
		defer close(hbDone)
		ticker := time.NewTicker(15 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-stop:
				return
			case <-r.Context().Done():
				return
			case <-ticker.C:
				_ = em.Heartbeat()
			}
		}
	}()
	defer func() {
		close(stop)
		<-hbDone
	}()

	store := agent.NewSqlcAgentStore(a.d.Queries)
	if err := store.CreateChatMessage(r.Context(), projectID, "user", body.UserInput); err != nil {
		slog.Error("studio turn: persist student message",
			"err", err, "request_id", httpx.RequestIDFromContext(r.Context()))
		_ = em.ErrorEnvelope("internal_error", "对话处理失败，请重试")
		_ = em.Done()
		return
	}
	_ = store.AppendEvent(r.Context(), agent.EventRow{
		ProjectID: projectID, Surface: "studio", Type: "prompt_sent", Payload: []byte(`{}`),
	})

	sk, _ := skills.ByID("writing-project")
	deps := agent.AgentDeps{
		Store:            store,
		Provider:         a.d.Provider,
		Resolved:         resolved,
		Sim:              studioSimilarity(),
		Skill:            &sk,
		SkipSurfaceCards: true,
	}
	action, err := agent.RunAgentStep(r.Context(), deps, projectID, agent.Trigger{Kind: "student_turn"})
	if err != nil {
		slog.Error("studio turn: RunAgentStep",
			"err", err, "request_id", httpx.RequestIDFromContext(r.Context()))
		_ = em.ErrorEnvelope("internal_error", "对话处理失败，请重试")
		_ = em.Done()
		return
	}
	switch {
	case action == nil:
		// silence: a legitimate first-class outcome (design §2) — nothing to
		// stream but done.
	case action.Kind == "intervention":
		// Anchor is sent EMPTY, deliberately: a live intervention's anchor is
		// a {kind,id} node reference, not the seed data's {label} shape — there
		// is no clean display label to derive from a node id (a UUID) without
		// resolving the node's text or storing a label at mint time, which is
		// deferred polish. The client shows the criterion chip only, which
		// matches what the reload projection (projectCoach) already produces
		// for these interventions (anchorLabel("{kind,id}") == "").
		_ = em.Intervention(action.InterventionID, action.Output.Body, "", action.Output.Criterion, "")
	case action.Kind == "check_gate" && action.GateReport != nil:
		gr := action.GateReport
		_ = em.Gate(gr.Contract, gr.Status, 0, 0, gr.Missing) // passed/total: deferred polish, see GateReport.Items
	}
	_ = em.Done()
}
