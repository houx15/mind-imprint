package api

// projectcards.go — Task 4: POST /projects/{id}/cards/{cid}/activate and
// .../skip, the project-scoped card lifecycle endpoints that sit beside
// postProjectTurn's card surfacing (Task 5c-2, agent-spec §3). Both reuse
// loadOwnedProjectCard, which mirrors disposition.go's ownership+membership
// pattern: loadOwnedProject hides non-owned/missing projects as 404, then a
// ListCardInstancesByProject scan scopes {cid} to that project (a card_id
// has no project filter of its own in the schema, so without this scan a
// caller who owns projectID could act on another user's card by
// guessing/observing a foreign card id — the same IDOR loadOwnedProjectCard's
// sibling in disposition.go documents for interventions). 404, not 403, so
// existence never leaks.

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"mindimprint/api/internal/agent"
	"mindimprint/api/internal/cards"
	"mindimprint/api/internal/gateway"
	"mindimprint/api/internal/httpx"
	"mindimprint/api/internal/skills"
)

// loadOwnedProjectCard scopes {cid} to the owned {id} project (404-no-leak).
func (a *API) loadOwnedProjectCard(w http.ResponseWriter, r *http.Request) (uuid.UUID, uuid.UUID, bool) {
	projectID, ok := a.loadOwnedProject(w, r)
	if !ok {
		return uuid.UUID{}, uuid.UUID{}, false
	}
	cid, err := uuid.Parse(r.PathValue("cid"))
	if err != nil {
		httpx.WriteError(w, r, httpx.ErrNotFound("资源不存在"))
		return uuid.UUID{}, uuid.UUID{}, false
	}
	cis, err := a.d.Queries.ListCardInstancesByProject(r.Context(), pgtype.UUID{Bytes: projectID, Valid: true})
	if err != nil {
		httpx.WriteError(w, r, err)
		return uuid.UUID{}, uuid.UUID{}, false
	}
	for _, ci := range cis {
		if ci.ID == cid {
			return projectID, cid, true
		}
	}
	httpx.WriteError(w, r, httpx.ErrNotFound("资源不存在"))
	return uuid.UUID{}, uuid.UUID{}, false
}

// activateProjectCard marks a surfaced card "active" once the student opens
// it (design's "触发是自动的，但「打开」由学生确认" — opening is a distinct,
// recorded step from being surfaced). Uses the agent.AgentStore seam (Task 2)
// so the write goes through the same path RunAgentStep itself would use.
func (a *API) activateProjectCard(w http.ResponseWriter, r *http.Request) {
	projectID, cid, ok := a.loadOwnedProjectCard(w, r)
	if !ok {
		return
	}
	store := agent.NewSqlcAgentStore(a.d.Queries)
	if err := store.SetCardInstanceStatus(r.Context(), projectID, cid, "active"); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if err := store.AppendEvent(r.Context(), agent.EventRow{
		ProjectID: projectID, Surface: "studio", Type: "card_activated", Payload: []byte(`{}`),
	}); err != nil {
		slog.Warn("card activate: append card_activated event failed",
			"err", err, "request_id", httpx.RequestIDFromContext(r.Context()))
	}
	w.WriteHeader(http.StatusNoContent)
}

// skipProjectCard records a skipped card: process-is-data (design's 「过程即
// 数据」) means a skip is submitted with an empty field_values payload rather
// than silently discarded, so the event trace still lands in the process
// tree.
func (a *API) skipProjectCard(w http.ResponseWriter, r *http.Request) {
	projectID, cid, ok := a.loadOwnedProjectCard(w, r)
	if !ok {
		return
	}
	var body struct {
		EventTrace json.RawMessage `json:"event_trace"`
	}
	if err := decodeJSON(r, &body); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if err := validateEventTrace(body.EventTrace); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	store := agent.NewSqlcAgentStore(a.d.Queries)
	if err := store.SubmitProjectCardInstance(r.Context(), projectID, cid, []byte("{}"), body.EventTrace); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if err := store.SetCardInstanceStatus(r.Context(), projectID, cid, "skipped"); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if err := store.AppendEvent(r.Context(), agent.EventRow{
		ProjectID: projectID, Surface: "studio", Type: "card_skipped", Payload: []byte(`{}`),
	}); err != nil {
		slog.Warn("card skip: append card_skipped event failed",
			"err", err, "request_id", httpx.RequestIDFromContext(r.Context()))
	}
	w.WriteHeader(http.StatusNoContent)
}

// submitProjectCard (SSE, Task 5) persists a filled card envelope, runs
// CompleteCard (mints the evidence node once the spec's completion
// predicates hold over the submitted anchors — design §6/agent-spec §3),
// then refeeds one RunAgentStep so the coach can react. Mirrors
// postProjectTurn's SSE shape (heartbeat, studioEmitter, streamAction)
// rather than duplicating it, since both endpoints drive the same
// RunAgentStep result vocabulary.
func (a *API) submitProjectCard(w http.ResponseWriter, r *http.Request) {
	projectID, cid, ok := a.loadOwnedProjectCard(w, r)
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

	var body struct {
		FieldValues json.RawMessage `json:"field_values"`
		EventTrace  json.RawMessage `json:"event_trace"`
		Anchors     json.RawMessage `json:"anchors"`
	}
	if err := decodeJSON(r, &body); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if err := validateFieldValues(body.FieldValues); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if err := validateEventTrace(body.EventTrace); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if err := validateAnchors(body.Anchors); err != nil {
		httpx.WriteError(w, r, err)
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
	// postProjectTurn's shape exactly.
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
	if err := store.SetCardInstanceAnchors(r.Context(), projectID, cid, body.Anchors); err != nil {
		_ = em.ErrorEnvelope("internal_error", "提交失败，请重试")
		_ = em.Done()
		return
	}
	if err := store.SubmitProjectCardInstance(r.Context(), projectID, cid, body.FieldValues, body.EventTrace); err != nil {
		_ = em.ErrorEnvelope("internal_error", "提交失败，请重试")
		_ = em.Done()
		return
	}

	sk, _ := skills.ByID("writing-project")
	deps := agent.AgentDeps{
		Store: store, Provider: a.d.Provider, Resolved: resolved,
		Sim: studioSimilarity(), Skill: &sk, SkipSurfaceCards: false,
	}

	// Mint the evidence node (if the submitted anchors satisfy the spec's
	// completion predicates) BEFORE the refeed, so the coach's reaction can
	// see the freshly promoted evidence in the same graph read. Only a
	// satisfying submit (CompleteCard's complete == true) flips
	// card_instance.status to "completed" — student confirmation of a
	// satisfying submit, not AI adjudication (not a DEC-3 issue). An
	// unsatisfying submit (every form-only CRAAP fill today, since the
	// schema field renderer produces field_values, never anchors — the
	// live mint is deferred to Slice 6's material-annotation surface)
	// leaves the card "active" so the student can refill/resubmit instead
	// of being falsely marked done with no evidence node.
	row, err := store.GetCardInstance(r.Context(), cid)
	if err == nil {
		if spec, ok := cards.ByID(row.CardID); ok {
			complete, err := agent.CompleteCard(r.Context(), deps, spec, cid)
			if err != nil {
				slog.Error("card submit: CompleteCard", "err", err, "request_id", httpx.RequestIDFromContext(r.Context()))
			} else if complete {
				if err := store.SetCardInstanceStatus(r.Context(), projectID, cid, "completed"); err != nil {
					_ = em.ErrorEnvelope("internal_error", "提交失败，请重试")
					_ = em.Done()
					return
				}
			}
		}
	} else {
		slog.Error("card submit: GetCardInstance", "err", err, "request_id", httpx.RequestIDFromContext(r.Context()))
	}

	action, err := agent.RunAgentStep(r.Context(), deps, projectID, agent.Trigger{Kind: "card_refeed"})
	if err != nil {
		slog.Error("card submit: refeed", "err", err, "request_id", httpx.RequestIDFromContext(r.Context()))
		_ = em.ErrorEnvelope("internal_error", "提交失败，请重试")
		_ = em.Done()
		return
	}
	streamAction(em, action)
	_ = em.Done()
}
