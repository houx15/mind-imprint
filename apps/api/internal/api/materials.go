package api

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"mindimprint/api/internal/agent"
	"mindimprint/api/internal/cards"
	"mindimprint/api/internal/httpx"
	"mindimprint/api/internal/materialize"
	"mindimprint/api/internal/store/sqlc"
	"mindimprint/api/internal/studio"
)

// ingestMaterialReq is one of two shapes: a URL to fetch, or pasted text.
// takeaway/tier are the student's own words — the AI writes neither, and has
// no path to this handler at all (spec §4, RL-2).
type ingestMaterialReq struct {
	URL      string `json:"url"`
	Title    string `json:"title"`
	Text     string `json:"text"`
	Takeaway string `json:"takeaway"`
	Tier     string `json:"tier"`
}

// ingestMaterial is the ONLY way a material row can be created. It is reached
// solely by the owning student's explicit POST — never by the agent, never by
// a summonable tool. The material row and its source_log_entry land in one
// transaction: a material with no log entry would be a source that was never
// "opened", exactly the state RL-2 forbids.
func (a *API) ingestMaterial(w http.ResponseWriter, r *http.Request) {
	projectID, ok := a.loadOwnedProject(w, r)
	if !ok {
		return
	}
	u, _ := UserFromContext(r.Context())

	// Entitlement gate — this endpoint spends network + (indirectly) tokens.
	entitled, err := HasEntitlement(r.Context(), u)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if !entitled {
		httpx.WriteError(w, r, httpx.ErrNotEntitled())
		return
	}

	var req ingestMaterialReq
	if err := decodeJSON(r, &req); err != nil {
		httpx.WriteError(w, r, err)
		return
	}

	title, text, origin := req.Title, req.Text, "pasted"
	if req.URL != "" {
		t, body, _, ferr := a.d.Fetcher.FetchReadable(r.Context(), req.URL)
		if ferr != nil {
			httpx.WriteError(w, r, httpx.ErrBadRequest("fetch_failed",
				"取不到这个链接的正文，可以直接把正文粘进来。", nil))
			return
		}
		title, text, origin = t, body, "fetched"
	}

	blocks := materialize.Segment(text)
	if len(blocks) == 0 {
		httpx.WriteError(w, r, httpx.ErrBadRequest("empty_body", "正文是空的。", nil))
		return
	}
	if title == "" {
		title = req.URL // never a blank card in the dossier
	}
	if title == "" {
		// The URL fallback above is a no-op on the paste path (req.URL is ""
		// there) — only the form's client-side 标题-required rule guarded
		// against a nameless dossier card; enforce it server-side too.
		httpx.WriteError(w, r, httpx.ErrBadRequest("missing_title", "给这条素材起个名字。", nil))
		return
	}
	rawBlocks, err := json.Marshal(blocks)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}

	var sourceURL *string
	if req.URL != "" {
		v := req.URL
		sourceURL = &v
	}
	var tier *string
	if req.Tier != "" {
		v := req.Tier
		tier = &v
	}

	// One transaction: a material with no log entry is a source that was
	// never "opened" — precisely the state RL-2 forbids.
	tx, err := a.d.Pool.Begin(r.Context())
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	defer func() { _ = tx.Rollback(r.Context()) }()
	qtx := a.d.Queries.WithTx(tx)

	mat, err := qtx.CreateProjectMaterial(r.Context(), sqlc.CreateProjectMaterialParams{
		TaskID:    pgtype.UUID{}, // NULL — the task surface is gone (migration 0020)
		ProjectID: pgtype.UUID{Bytes: projectID, Valid: true},
		Kind:      "article",
		Source:    origin,
		Title:     title,
		SourceUrl: sourceURL,
		Blocks:    rawBlocks,
	})
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if _, err := qtx.CreateSourceLogEntry(r.Context(), sqlc.CreateSourceLogEntryParams{
		ProjectID:  projectID,
		MaterialID: pgtype.UUID{Bytes: mat.ID, Valid: true},
		Url:        req.URL,
		Title:      title,
		Takeaway:   req.Takeaway,
		Tier:       tier,
	}); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if err := tx.Commit(r.Context()); err != nil {
		httpx.WriteError(w, r, err)
		return
	}

	// M1: a fresh (unevaluated) article material can flip
	// allArticlesHaveRiskNote (attest.go) from true to false — e.g. she had
	// 2/2 articles evaluated, then added a 3rd with no risk_note yet.
	// attestS3S4 is the ONLY place that recomputes source_risk_notes, and
	// this endpoint is the only mutation of the materials list that did not
	// already call it (submitProjectCard does, on every card submit) — so
	// without this call the gate item kept reading "solid" from the LAST
	// write that happened to make it true, until some unrelated card submit
	// incidentally recomputed it. Same attestation-before-advanceGates order
	// as projectcards.go: gate state must be current before advanceGates
	// decides which contracts just finished.
	a.attestS3S4(r.Context(), projectID)
	a.advanceGates(r.Context(), projectID)

	dtoBlocks := make([]studio.MaterialBlockDTO, len(blocks))
	for i, b := range blocks {
		dtoBlocks[i] = studio.MaterialBlockDTO{ID: b.ID, Text: b.Text}
	}
	dto := studio.MaterialDTO{
		ID:          mat.ID.String(),
		Title:       title,
		Kind:        mat.Kind,
		Origin:      origin,
		Blocks:      dtoBlocks,
		Locked:      false,
		Role:        "",
		Tier:        req.Tier,
		Takeaway:    req.Takeaway,
		Anchors:     []json.RawMessage{},
		TimeSpentS:  0,     // freshly ingested — never opened yet
		LateralRead: false, // no cross_check mint has touched this source yet
	}
	if mat.SourceUrl != nil {
		dto.SourceURL = *mat.SourceUrl
	}
	httpx.WriteJSON(w, http.StatusCreated, dto)
}

// logOpenReq is the body of POST .../materials/{mid}/open — the amount of
// reading time (seconds) to add for this open.
type logOpenReq struct {
	TimeSpentS int32 `json:"time_spent_s"`
}

// logSourceOpen accumulates reading time on the source-log entry and appends
// a source_opened event — the ledger Slice 10's assessor reads, and (via
// Task 7's dossier timer) the first HTTP path that reaches the event table.
//
// Best-effort by policy: losing a timing sample must never break a student's
// reading, so once the request itself is valid, a downstream write failure
// only warns (slog.Warn) and still returns 204. A malformed body or a
// negative time_spent_s is a genuine client error and stays a 400 — that is
// not a "timing sample lost", it's a request that was never valid.
//
// mid is scoped to the owned project via the log entry's own project_id
// (fetched anyway for its url) rather than a separate lookup: an unknown mid,
// or one belonging to another project, is hidden as 404 — the same
// ownership-hidden-as-not-found convention loadOwnedProject and
// loadOwnedProjectCard already use, so a caller can't bump another project's
// reading-time ledger by guessing a foreign material id.
func (a *API) logSourceOpen(w http.ResponseWriter, r *http.Request) {
	projectID, ok := a.loadOwnedProject(w, r)
	if !ok {
		return
	}
	mid, err := uuid.Parse(r.PathValue("mid"))
	if err != nil {
		httpx.WriteError(w, r, httpx.ErrNotFound("资源不存在"))
		return
	}
	var req logOpenReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.TimeSpentS < 0 {
		httpx.WriteError(w, r, httpx.ErrBadRequest("bad_json", "请求格式不对", nil))
		return
	}

	midPg := pgtype.UUID{Bytes: mid, Valid: true}
	logEntry, err := a.d.Queries.GetSourceLogByMaterial(r.Context(), midPg)
	if err != nil || logEntry.ProjectID != projectID {
		httpx.WriteError(w, r, httpx.ErrNotFound("资源不存在"))
		return
	}

	if err := a.d.Queries.AddSourceTimeSpent(r.Context(), sqlc.AddSourceTimeSpentParams{
		MaterialID: midPg,
		TimeSpentS: req.TimeSpentS,
	}); err != nil {
		slog.Warn("log source open: accumulate time_spent_s failed",
			"err", err, "request_id", httpx.RequestIDFromContext(r.Context()))
	}

	payload, err := json.Marshal(map[string]any{
		"url":          logEntry.Url,
		"time_spent_s": req.TimeSpentS,
	})
	if err != nil {
		slog.Warn("log source open: marshal source_opened payload failed",
			"err", err, "request_id", httpx.RequestIDFromContext(r.Context()))
	} else {
		store := agent.NewSqlcAgentStore(a.d.Queries, a.d.Pool)
		if err := store.AppendEvent(r.Context(), agent.EventRow{
			ProjectID: projectID, Surface: "studio", Type: "source_opened", Payload: payload,
		}); err != nil {
			slog.Warn("log source open: append source_opened event failed",
				"err", err, "request_id", httpx.RequestIDFromContext(r.Context()))
		}
	}

	// Opening and logging a source IS the recon evaluate_perspectives'
	// recon_logged item names (spec §6.2) — attest it, then re-derive gate
	// state from the graph. Both are best-effort (advance.go): neither may
	// fail this request.
	a.attestReconLogged(r.Context(), projectID)
	a.advanceGates(r.Context(), projectID)

	w.WriteHeader(http.StatusNoContent)
}

// prepareSourceAnnotation makes "印记 reads the article WITH the student" real:
// when a source is OPENED (not closed — logSourceOpen fires on close), it
// surfaces the source's evaluation card (CRAAP annotate, or SIFT compare once
// evaluated) and generates the per-CRAAP-dimension article anchors, so the
// article lights up its flagged sentences and the interactive tool-card appears
// in the rail on the very next project fetch. Delivery is via projection
// reload, not a live frame — the frontend re-fetches after calling this, and
// projectActiveCard/projectMaterials already surface the proposed card and its
// persisted anchors (studio/projection.go).
//
// The summon DECISION reuses SurfaceCardCandidates verbatim (the same rule the
// turn loop uses) so open-path and turn-path summoning never drift: it is
// silent while any card is in flight, never re-summons an already-evaluated
// source, and only ever fires for this exact material. Fully best-effort — a
// failure here must never break opening a source, so every error only warns and
// still returns 204. Idempotent: a second open finds the card already surfaced
// (in-flight guard) and no-ops.
func (a *API) prepareSourceAnnotation(w http.ResponseWriter, r *http.Request) {
	projectID, ok := a.loadOwnedProject(w, r)
	if !ok {
		return
	}
	mid, err := uuid.Parse(r.PathValue("mid"))
	if err != nil {
		httpx.WriteError(w, r, httpx.ErrNotFound("资源不存在"))
		return
	}

	store := agent.NewSqlcAgentStore(a.d.Queries, a.d.Pool)
	g, err := store.LoadGraph(r.Context(), projectID)
	if err != nil {
		slog.Warn("prepare annotation: load graph failed",
			"err", err, "request_id", httpx.RequestIDFromContext(r.Context()))
		w.WriteHeader(http.StatusNoContent)
		return
	}
	// Ownership: the graph carries only THIS project's materials — a mid that
	// isn't among them is foreign or unknown, hidden as 404 (same convention as
	// loadOwnedProject). Works for seeded materials too, unlike the source-log
	// lookup logSourceOpen uses (seed rows have no source_log entry).
	inProject := false
	for _, m := range g.Materials {
		// Parse-based compare: MaterialView.ID is a pgtype.UUID rendering that
		// may differ in dashing/case from google/uuid's canonical String().
		if pid, perr := uuid.Parse(m.ID); perr == nil && pid == mid {
			inProject = true
			break
		}
	}
	if !inProject {
		httpx.WriteError(w, r, httpx.ErrNotFound("资源不存在"))
		return
	}
	surfaced := false
	for _, c := range agent.SurfaceCardCandidates(g) {
		// AnchorID is a MaterialView.ID (pgtype rendering) — parse-compare it to
		// this material, same reason as the ownership loop above.
		cid, cerr := uuid.Parse(c.AnchorID)
		if c.Verb != "surface_card" || c.AnchorKind != "material" || cerr != nil || cid != mid {
			continue
		}
		spec, specOK := cards.ByID(c.CardID)
		if !specOK {
			break
		}
		action, serr := agent.SurfaceCard(r.Context(), agent.AgentDeps{Store: store}, projectID, spec, mid)
		if serr != nil || action == nil {
			slog.Warn("prepare annotation: surface card failed",
				"err", serr, "card_id", c.CardID, "request_id", httpx.RequestIDFromContext(r.Context()))
			break
		}
		if spec.Primitive == "annotate" || spec.Primitive == "compare" {
			// Reuses the turn path's own anchor generator + guidance fade;
			// degrades to no anchors (card still surfaces) on any failure.
			// scopeToMaterial=true: the student opened THIS source to read it,
			// so anchors must quote it (never another material).
			a.surfaceAnchors(r.Context(), store, projectID, spec, action.CardInstanceID, action.MaterialID, true)
		}
		surfaced = true
		break
	}

	// `surfaced` tells the client whether a NEW card/anchors were minted — it
	// refetches the projection only then, so an open that found nothing to do
	// (in-flight card, already-evaluated source) costs no needless reload.
	httpx.WriteJSON(w, http.StatusOK, map[string]bool{"surfaced": surfaced})
}
