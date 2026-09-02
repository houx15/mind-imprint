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
	"context"
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
	store := agent.NewSqlcAgentStore(a.d.Queries, a.d.Pool)
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
	store := agent.NewSqlcAgentStore(a.d.Queries, a.d.Pool)
	if err := store.SubmitAndSkipCardInstance(r.Context(), projectID, cid, []byte("{}"), body.EventTrace); err != nil {
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

// validateAnchorMaterialsBelongToProject verifies every anchor whose
// material_id is set names a material that actually belongs to projectID.
// validateAnchors (cards.go) only checks shape — it has no DB access and
// cannot know whose material an id refers to. Without this check, a client
// could submit a SIFT lateral anchor naming ANOTHER project's material, and
// CompleteCard would mint a "cites" edge straight to it (card_effects.go's
// GraphEffects trusts every anchor's MaterialID verbatim), satisfying
// lateral_source_present — and S3's cross_check gate — with a source that
// isn't hers (whole-branch review IMPORTANT 3). A malformed material_id is a
// genuine client error (400); one that parses but doesn't resolve to a
// material in THIS project is hidden as 404 — the same
// ownership-hidden-as-not-found convention loadOwnedProject/logSourceOpen
// already use, so a caller can't distinguish "no such material" from "that
// material belongs to someone else".
func (a *API) validateAnchorMaterialsBelongToProject(ctx context.Context, projectID uuid.UUID, raw json.RawMessage) error {
	if len(raw) == 0 {
		return nil
	}
	var anchors []agent.Anchor
	if err := json.Unmarshal(raw, &anchors); err != nil {
		return httpx.ErrBadRequest("validation_failed", "anchors 必须是数组", nil)
	}
	seen := make(map[string]bool, len(anchors))
	for _, an := range anchors {
		if an.MaterialID == "" || seen[an.MaterialID] {
			continue
		}
		seen[an.MaterialID] = true
		mid, err := uuid.Parse(an.MaterialID)
		if err != nil {
			return httpx.ErrBadRequest("validation_failed", "anchors.material_id 不是合法 id", nil)
		}
		mat, err := a.d.Queries.GetMaterial(ctx, mid)
		if err != nil || !mat.ProjectID.Valid || mat.ProjectID.Bytes != projectID {
			return httpx.ErrNotFound("资源不存在")
		}
	}
	return nil
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
	if err := a.validateAnchorMaterialsBelongToProject(r.Context(), projectID, body.Anchors); err != nil {
		httpx.WriteError(w, r, err)
		return
	}

	resolved, err := a.routeE(r.Context(), gateway.ClassDialogue)
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

	// cardStatus is the RESULTING state this submit reports on the "done"
	// frame: "active" (the default, and the honest answer for every
	// early-return below — nothing has changed the row's status yet) unless
	// the completion predicate is actually satisfied further down, in which
	// case it flips to "completed". The client (conversation.ts submitCard)
	// only retires its local card on an explicit "completed" — never on a
	// bare "done" — so this variable must be honest at every return point,
	// not just the happy path.
	cardStatus := "active"

	store := agent.NewSqlcAgentStore(a.d.Queries, a.d.Pool)
	if err := store.SetCardInstanceAnchors(r.Context(), projectID, cid, body.Anchors); err != nil {
		_ = em.ErrorEnvelope("internal_error", "提交失败，请重试")
		_ = em.DoneCard(cardStatus)
		return
	}
	if err := store.SubmitProjectCardInstance(r.Context(), projectID, cid, body.FieldValues, body.EventTrace); err != nil {
		_ = em.ErrorEnvelope("internal_error", "提交失败，请重试")
		_ = em.DoneCard(cardStatus)
		return
	}

	sk, _ := skills.ByID("writing-project")
	deps := agent.AgentDeps{
		Store: store, Provider: a.d.Provider, Resolved: resolved,
		Sim: studioSimilarity(), Skill: &sk, SkipSurfaceCards: false,
		SuppressSurfaceCardIDs: studioSuppressedSurfaceCardIDs,
	}

	// Mint the evidence node (if the submitted anchors satisfy the spec's
	// completion predicates) BEFORE the refeed, so the coach's reaction can
	// see the freshly promoted evidence in the same graph read. Only a
	// satisfying submit (CompleteCard's complete == true) flips
	// card_instance.status to "completed" — student confirmation of a
	// satisfying submit, not AI adjudication (not a DEC-3 issue). An
	// unsatisfying submit (partial anchors — e.g. the anchor generator
	// degraded and shipped fewer than the spec's full tag set, studioturn.go
	// surfaceAnchors) leaves the card "active" so the student can
	// refill/resubmit instead of being falsely marked done with no evidence
	// node — and now (FIX 1) reports that honestly on "done" instead of
	// having the client discard the card regardless.
	row, err := store.GetCardInstance(r.Context(), cid)
	if err == nil {
		if spec, ok := cards.ByID(row.CardID); ok {
			complete, err := agent.CompleteCard(r.Context(), deps, spec, cid)
			if err != nil {
				// FIX 5: a CompleteCard error must not be swallowed and
				// reported as success — the SSE stream and the client both
				// need to see the submit failed, not silently proceed to
				// the refeed as if the card had either completed or been
				// left cleanly active.
				slog.Error("card submit: CompleteCard", "err", err, "request_id", httpx.RequestIDFromContext(r.Context()))
				_ = em.ErrorEnvelope("internal_error", "提交失败，请重试")
				_ = em.DoneCard(cardStatus)
				return
			}
			if complete {
				if err := store.SetCardInstanceStatus(r.Context(), projectID, cid, "completed"); err != nil {
					_ = em.ErrorEnvelope("internal_error", "提交失败，请重试")
					_ = em.DoneCard(cardStatus)
					return
				}
				cardStatus = "completed"
			}
		}
	} else {
		slog.Error("card submit: GetCardInstance", "err", err, "request_id", httpx.RequestIDFromContext(r.Context()))
	}

	// Filling a card is student activity too — the roster's 最近活跃 depends
	// on it, same as postProjectTurn's touch. Touched here, right after the
	// card is successfully persisted/completed and BEFORE the refeed below:
	// the refeed can itself error and return early, and a card submit whose
	// refeed errors must still advance last_active_at — the exact roster lie
	// this touch was added to fix. A failure to touch must not fail the
	// submit, which already succeeded, and must not corrupt the SSE stream
	// (no error envelope, just a log).
	if err := a.d.Queries.TouchProject(r.Context(), projectID); err != nil {
		slog.Warn("card submit: touch project last_active_at",
			"err", err, "project_id", projectID, "request_id", httpx.RequestIDFromContext(r.Context()))
	}

	action, err := agent.RunAgentStep(r.Context(), deps, projectID, agent.Trigger{Kind: "card_refeed", CardInstanceID: cid.String()})
	if err != nil {
		slog.Error("card submit: refeed", "err", err, "request_id", httpx.RequestIDFromContext(r.Context()))
		_ = em.ErrorEnvelope("internal_error", "提交失败，请重试")
		_ = em.DoneCard(cardStatus)
		return
	}
	a.streamAction(r.Context(), em, action, projectID, store)
	// N3f: the three S3/S4 student_written items are attested from graph
	// state, so they must be recomputed BEFORE advanceGates decides which
	// contracts are now finished — otherwise the submit that completes the
	// last risk_note attests it but does not advance until the NEXT write.
	a.attestS3S4(r.Context(), projectID)
	a.advanceGates(r.Context(), projectID)
	_ = em.DoneCard(cardStatus)
}
