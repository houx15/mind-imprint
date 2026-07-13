package api

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"mindimprint/api/internal/agent"
	"mindimprint/api/internal/agent/enforcement"
	"mindimprint/api/internal/cards"
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

func (e *studioEmitter) Card(cardInstanceID, cardID, nudgeText string, anchors []byte) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.sse.Card(cardInstanceID, cardID, nudgeText, anchors)
}

// studioSimilarity is the Studio turn endpoint's enforcement.Similarity seam:
// a keyless, offline lexical-overlap heuristic (no embedding call, no key, no
// network). A real embedding-backed Similarity is a separate, later task once
// an embedding provider is chosen.
func studioSimilarity() enforcement.Similarity { return enforcement.LexicalSimilarity{} }

// postProjectTurn drives the agent runtime one step (RunAgentStep) for a
// Studio project and streams the result over SSE: at most one card,
// intervention, or gate event, then done.
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
	if err := store.AppendEvent(r.Context(), agent.EventRow{
		ProjectID: projectID, Surface: "studio", Type: "prompt_sent", Payload: []byte(`{}`),
	}); err != nil {
		slog.Warn("studio turn: append prompt_sent event failed",
			"err", err, "request_id", httpx.RequestIDFromContext(r.Context()))
	}

	sk, _ := skills.ByID("writing-project")
	deps := agent.AgentDeps{
		Store:            store,
		Provider:         a.d.Provider,
		Resolved:         resolved,
		Sim:              studioSimilarity(),
		Skill:            &sk,
		SkipSurfaceCards: false,
	}
	action, err := agent.RunAgentStep(r.Context(), deps, projectID, agent.Trigger{Kind: "student_turn"})
	if err != nil {
		slog.Error("studio turn: RunAgentStep",
			"err", err, "request_id", httpx.RequestIDFromContext(r.Context()))
		_ = em.ErrorEnvelope("internal_error", "对话处理失败，请重试")
		_ = em.Done()
		return
	}
	// A turn is activity — the roster's 最近活跃 depends on it. A failure to
	// touch must not fail the student's turn, which already succeeded.
	if err := a.d.Queries.TouchProject(r.Context(), projectID); err != nil {
		slog.Warn("studio turn: touch project last_active_at",
			"err", err, "project_id", projectID, "request_id", httpx.RequestIDFromContext(r.Context()))
	}

	a.streamAction(r.Context(), em, action, projectID, store)
	_ = em.Done()
}

// streamAction emits one RunAgentStep result over the Studio SSE emitter —
// shared by postProjectTurn and submitProjectCard (Task 5), the two
// endpoints that both drive RunAgentStep and stream whatever it returns.
// Does not call em.Done(); callers do that themselves once, after any
// endpoint-specific streaming this function doesn't cover.
func (a *API) streamAction(ctx context.Context, em *studioEmitter, action *agent.Action, projectID uuid.UUID, store agent.AgentStore) {
	switch {
	case action == nil:
		// silence: a legitimate first-class outcome (design §2) — nothing to
		// stream but done.
	case action.Kind == "surface_card":
		spec, _ := cards.ByID(action.CardID)
		anchors := []byte("[]")
		// Guidance level L1: the AI authors these anchors at surface time via
		// AnchorGenerator. Higher guidance levels populate card_instance.anchors
		// upstream (from student action) without touching this seam.
		if spec.Primitive == "annotate" {
			if raw, ok := a.surfaceAnchors(ctx, store, projectID, spec, action.CardInstanceID); ok {
				anchors = raw
			}
		}
		_ = em.Card(action.CardInstanceID, action.CardID, spec.Name, anchors)
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
}

// surfaceAnchors generates the L1 AI anchors for a just-surfaced annotate
// card (e.g. craap), persists them on the card_instance, and returns the
// JSON to carry on the same SSE `card` frame. Degrades to (nil,false) on
// any error — parse failure, persistence failure, empty generation — so the
// card still surfaces with no anchors rather than failing the whole turn;
// generating pre-anchored questions is a nice-to-have on top of the card
// existing, not a precondition for it.
func (a *API) surfaceAnchors(ctx context.Context, store agent.AgentStore, projectID uuid.UUID, spec cards.Spec, cardInstanceID string) ([]byte, bool) {
	cid, err := uuid.Parse(cardInstanceID)
	if err != nil {
		return nil, false
	}
	materials, err := a.projectMaterials(ctx, projectID)
	if err != nil {
		return nil, false
	}
	gen := agent.NewAnchorGenerator(a.d.Provider, a.d.ChatResolver)
	anchors, err := gen.Generate(ctx, spec, materials)
	if err != nil || len(anchors) == 0 {
		return nil, false
	}
	raw, err := json.Marshal(anchors)
	if err != nil {
		return nil, false
	}
	if err := store.SetCardInstanceAnchors(ctx, projectID, cid, raw); err != nil {
		slog.Warn("surface anchors: persist failed", "err", err, "request_id", httpx.RequestIDFromContext(ctx))
		return nil, false
	}
	return raw, true
}

// projectMaterials reads the project's materials into the agent runtime's
// view — ID (needed by the deterministic fallback) and Blocks (needed by
// the real LLM path for quote->offset resolution). Mirrors
// sqlcTurnStore.ListMaterials (turn.go), the legacy task-scoped equivalent.
func (a *API) projectMaterials(ctx context.Context, projectID uuid.UUID) ([]agent.Material, error) {
	rows, err := a.d.Queries.ListMaterialsByProject(ctx, pgtype.UUID{Bytes: projectID, Valid: true})
	if err != nil {
		return nil, err
	}
	out := make([]agent.Material, 0, len(rows))
	for _, r := range rows {
		m := agent.Material{ID: r.ID.String(), Title: r.Title}
		if len(r.Blocks) > 0 {
			_ = json.Unmarshal(r.Blocks, &m.Blocks)
		}
		out = append(out, m)
	}
	return out, nil
}
