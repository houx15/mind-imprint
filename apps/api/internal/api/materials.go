package api

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"mindimprint/api/internal/agent"
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
		t, body, ferr := a.d.Fetcher.FetchReadable(r.Context(), req.URL)
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

	dtoBlocks := make([]studio.MaterialBlockDTO, len(blocks))
	for i, b := range blocks {
		dtoBlocks[i] = studio.MaterialBlockDTO{ID: b.ID, Text: b.Text}
	}
	dto := studio.MaterialDTO{
		ID:         mat.ID.String(),
		Title:      title,
		Kind:       mat.Kind,
		Origin:     origin,
		Blocks:     dtoBlocks,
		Locked:     false,
		Role:       "",
		Tier:       req.Tier,
		Takeaway:   req.Takeaway,
		Anchors:    []json.RawMessage{},
		TimeSpentS: 0, // freshly ingested — never opened yet
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
		store := agent.NewSqlcAgentStore(a.d.Queries)
		if err := store.AppendEvent(r.Context(), agent.EventRow{
			ProjectID: projectID, Surface: "studio", Type: "source_opened", Payload: payload,
		}); err != nil {
			slog.Warn("log source open: append source_opened event failed",
				"err", err, "request_id", httpx.RequestIDFromContext(r.Context()))
		}
	}

	w.WriteHeader(http.StatusNoContent)
}
