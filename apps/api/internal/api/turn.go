package api

import (
	"log/slog"
	"net/http"
	"sync"
	"time"

	"mindimprint/api/internal/agent"
	"mindimprint/api/internal/gateway"
	"mindimprint/api/internal/httpx"
	"mindimprint/api/internal/store/sqlc"
)

// syncEmitter serializes all SSE writes (turn deltas + heartbeats) under one
// mutex, since the heartbeat goroutine and RunTurn both write the same stream.
// Implements agent.SSEEmitter plus Heartbeat/ErrorEnvelope.
type syncEmitter struct {
	mu  sync.Mutex
	sse *gateway.SSEWriter
}

func (e *syncEmitter) Text(d string) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.sse.Text(d)
}

func (e *syncEmitter) Card(ci, c, n string, anchors []byte) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.sse.Card(ci, c, n, anchors)
}

func (e *syncEmitter) Done(id string) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.sse.Done(id)
}

func (e *syncEmitter) ErrorEnvelope(code, msg string) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.sse.ErrorEnvelope(code, msg)
}

func (e *syncEmitter) Heartbeat() error {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.sse.Heartbeat()
}

func (a *API) postTurn(w http.ResponseWriter, r *http.Request) {
	t, ok := a.loadOwnedTask(w, r)
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
		Source    string `json:"source"`
	}
	if err := decodeJSON(r, &body); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	// Normalize source to the only sanctioned value: "voice". Anything else
	// (including a client bug) is dropped to "" so a malformed marker can never
	// persist a value the shared Message contract rejects on rehydration.
	if body.Source != "voice" {
		body.Source = ""
	}
	// Empty user_input is a valid continuation turn: no user message is appended,
	// and the model replies from existing history (e.g. after a card submit/skip).

	// Commit to streaming. After this, errors are SSE error events, not JSON.
	sse, err := gateway.NewSSEWriter(w)
	if err != nil {
		// ResponseWriter doesn't support flushing — still a JSON error (headers not yet sent).
		httpx.WriteError(w, r, httpx.ErrInternal())
		return
	}
	em := &syncEmitter{sse: sse}

	// Heartbeat until the turn returns or the client disconnects.
	// Both RunTurn and the heartbeat goroutine write via em, so all writes
	// are serialized through em.mu — no concurrent writes to the underlying
	// http.ResponseWriter.
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
	// Stop the heartbeat AND wait for it to fully exit before this handler
	// returns — the ResponseWriter is invalid once ServeHTTP returns, so a
	// heartbeat write must never outlive the handler.
	defer func() {
		close(stop)
		<-hbDone
	}()

	// Record activity (best-effort; failure must not abort the turn).
	_, _ = a.d.Queries.TouchTask(r.Context(), sqlc.TouchTaskParams{ID: t.ID, UserID: u.ID})

	deps := agent.TurnDeps{
		Store:       agent.NewSqlcTurnStore(a.d.Queries),
		Provider:    a.d.Provider,
		KeyResolver: a.d.ChatResolver,
		Catalog:     a.d.Catalog,
		SpecByID:    a.d.SpecByID,
		SSE:         em,
		AnchorGen:   agent.NewAnchorGenerator(a.d.Provider, a.d.ChatResolver),
	}
	if err := agent.RunTurn(r.Context(), deps, t.ID, body.UserInput, body.Source); err != nil {
		// Stream already open: report via SSE error event. Log the real cause
		// server-side only — never leak provider/internal detail to the client.
		slog.Error("turn failed",
			"request_id", httpx.RequestIDFromContext(r.Context()),
			"task_id", t.ID.String(),
			"err", err.Error(),
		)
		_ = em.ErrorEnvelope("internal_error", "对话处理失败，请重试")
		return
	}
	// Turn succeeded (assistant message persisted) — a new substantive turn may
	// cross a milestone. Best-effort; must not write to the SSE stream.
	a.maybeTriggerMilestoneEval(r.Context(), t.ID)
}
