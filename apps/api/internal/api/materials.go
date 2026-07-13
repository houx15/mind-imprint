package api

import (
	"encoding/json"
	"net/http"

	"github.com/jackc/pgx/v5/pgtype"

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
		ID:       mat.ID.String(),
		Title:    title,
		Kind:     mat.Kind,
		Origin:   origin,
		Blocks:   dtoBlocks,
		Locked:   false,
		Role:     "",
		Tier:     req.Tier,
		Takeaway: req.Takeaway,
		Anchors:  []json.RawMessage{},
	}
	if mat.SourceUrl != nil {
		dto.SourceURL = *mat.SourceUrl
	}
	httpx.WriteJSON(w, http.StatusCreated, dto)
}
