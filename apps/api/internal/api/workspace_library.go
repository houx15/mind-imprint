package api

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"mindimprint/api/internal/agent"
	"mindimprint/api/internal/httpx"
	"mindimprint/api/internal/materialize"
	"mindimprint/api/internal/store/sqlc"
	"mindimprint/api/internal/studio"
)

// workspace_library.go — Slice 3 (Read room / Library) cheap-CRUD handlers: the
// Zotero-style collection tree + reference table, plus enter-reading (the bridge
// from a reference row into the existing Reading Room). None of these make a
// model call, so none gate on HasEntitlement — except enter-reading's fetch
// branch, which spends network exactly like ingestMaterial (materials.go) and
// gates on the same seam. The reading "notes" (quote→finding) a reference shows
// are PROJECTED from submitted reading cards anchored to its material, never
// stored on the reference row (migration 0036).

// -- wire DTOs --------------------------------------------------------------

// collectionDTO mirrors the contracts Collection: DB parent_id -> parentId.
type collectionDTO struct {
	ID       string  `json:"id"`
	Name     string  `json:"name"`
	ParentID *string `json:"parentId"`
	Position int32   `json:"position"`
}

func toCollectionDTO(row sqlc.Collection) collectionDTO {
	return collectionDTO{
		ID:       row.ID.String(),
		Name:     row.Name,
		ParentID: pgUUIDToStringPtr(row.ParentID),
		Position: row.Position,
	}
}

// readingNoteDTO is one projected reading outcome: the sentence the student
// referenced (anchor.quote) and what she found about it (anchor.answer). Never
// stored — derived from submitted reading cards (see notesByMaterial).
type readingNoteDTO struct {
	Quote   string `json:"quote"`
	Finding string `json:"finding"`
}

// referenceDTO mirrors the contracts Reference: DB classification/collection_id/
// search_hints/material_id -> classification/collectionId/searchHints/materialId,
// plus the projected notes[]. Credibility/decision are nullable enums (present
// only once the student has judged the source); collectionId/materialId are
// nullable links.
type referenceDTO struct {
	ID             string           `json:"id"`
	Title          string           `json:"title"`
	Classification string           `json:"classification"`
	Author         string           `json:"author"`
	Credentials    string           `json:"credentials"`
	Year           string           `json:"year"`
	URL            string           `json:"url"`
	Tags           []string         `json:"tags"`
	CollectionID   *string          `json:"collectionId"`
	Credibility    *string          `json:"credibility"`
	Evaluation     string           `json:"evaluation"`
	Decision       *string          `json:"decision"`
	Pending        bool             `json:"pending"`
	SearchHints    []string         `json:"searchHints"`
	MaterialID     *string          `json:"materialId"`
	Notes          []readingNoteDTO `json:"notes"`
}

func toReferenceDTO(row sqlc.Reference, notes []readingNoteDTO) referenceDTO {
	if notes == nil {
		notes = []readingNoteDTO{}
	}
	return referenceDTO{
		ID:             row.ID.String(),
		Title:          row.Title,
		Classification: row.Classification,
		Author:         row.Author,
		Credentials:    row.Credentials,
		Year:           row.Year,
		URL:            row.Url,
		Tags:           jsonbToStrings(row.Tags),
		CollectionID:   pgUUIDToStringPtr(row.CollectionID),
		Credibility:    row.Credibility,
		Evaluation:     row.Evaluation,
		Decision:       row.Decision,
		Pending:        row.Pending,
		SearchHints:    jsonbToStrings(row.SearchHints),
		MaterialID:     pgUUIDToStringPtr(row.MaterialID),
		Notes:          notes,
	}
}

var validCredibility = map[string]bool{"strong": true, "mixed": true, "weak": true}
var validDecision = map[string]bool{"use": true, "maybe": true, "drop": true}

// -- GET /library -----------------------------------------------------------

// getLibrary returns every collection + reference for the project. Each
// reference carries its projected notes[] (from submitted reading cards anchored
// to its material) and its materialId.
func (a *API) getLibrary(w http.ResponseWriter, r *http.Request) {
	projectID, ok := a.loadOwnedProject(w, r)
	if !ok {
		return
	}
	cols, err := a.d.Queries.ListCollections(r.Context(), projectID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	refs, err := a.d.Queries.ListReferences(r.Context(), projectID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	notesMap := a.notesByMaterial(r, projectID)

	collections := make([]collectionDTO, 0, len(cols))
	for _, c := range cols {
		collections = append(collections, toCollectionDTO(c))
	}
	references := make([]referenceDTO, 0, len(refs))
	for _, ref := range refs {
		var notes []readingNoteDTO
		if ref.MaterialID.Valid {
			notes = notesMap[uuid.UUID(ref.MaterialID.Bytes).String()]
		}
		references = append(references, toReferenceDTO(ref, notes))
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{
		"collections": collections,
		"references":  references,
	})
}

// notesByMaterial projects the reading outcomes for every material in the
// project, keyed by material id string. A "note" is one anchor of a SUBMITTED
// (status="completed") reading card that targets a material: quote←anchor.quote,
// finding←anchor.answer (the student's own finding). Best-effort — a card whose
// anchors don't parse, or an anchor with neither a quote nor a finding, is
// simply skipped, never fabricated. Reuses ListCardInstancesByProject rather
// than a new by-material query (card_instances has no material_id column — the
// link lives in each anchor's own material_id, exactly as projectMaterials reads
// it in studio/projection.go).
func (a *API) notesByMaterial(r *http.Request, projectID uuid.UUID) map[string][]readingNoteDTO {
	out := map[string][]readingNoteDTO{}
	cis, err := a.d.Queries.ListCardInstancesByProject(r.Context(), pgtype.UUID{Bytes: projectID, Valid: true})
	if err != nil {
		slog.Warn("library: list card instances failed",
			"err", err, "request_id", httpx.RequestIDFromContext(r.Context()))
		return out
	}
	for _, ci := range cis {
		if ci.Status != "completed" {
			continue
		}
		var anchors []agent.Anchor
		if err := json.Unmarshal(ci.Anchors, &anchors); err != nil {
			continue
		}
		for _, an := range anchors {
			if an.MaterialID == "" {
				continue
			}
			if strings.TrimSpace(an.Quote) == "" && strings.TrimSpace(an.Answer) == "" {
				continue
			}
			out[an.MaterialID] = append(out[an.MaterialID], readingNoteDTO{Quote: an.Quote, Finding: an.Answer})
		}
	}
	return out
}

// -- Collections ------------------------------------------------------------

func (a *API) createCollection(w http.ResponseWriter, r *http.Request) {
	projectID, ok := a.loadOwnedProject(w, r)
	if !ok {
		return
	}
	var body struct {
		Name     string  `json:"name"`
		ParentID *string `json:"parentId"`
	}
	if err := decodeJSON(r, &body); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	parent, err := stringPtrToPgUUID(body.ParentID)
	if err != nil {
		httpx.WriteError(w, r, httpx.ErrBadRequest("validation_failed", "parentId 不是有效的 id", nil))
		return
	}
	row, err := a.d.Queries.CreateCollection(r.Context(), sqlc.CreateCollectionParams{
		ProjectID: projectID,
		Name:      body.Name,
		ParentID:  parent,
		Position:  0,
	})
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, map[string]any{"collection": toCollectionDTO(row)})
}

func (a *API) patchCollection(w http.ResponseWriter, r *http.Request) {
	projectID, ok := a.loadOwnedProject(w, r)
	if !ok {
		return
	}
	cid, err := uuid.Parse(r.PathValue("cid"))
	if err != nil {
		httpx.WriteError(w, r, httpx.ErrNotFound("资源不存在"))
		return
	}
	cur, err := a.d.Queries.GetCollection(r.Context(), sqlc.GetCollectionParams{ID: cid, ProjectID: projectID})
	if err != nil {
		httpx.WriteError(w, r, httpx.ErrNotFound("资源不存在"))
		return
	}
	// parentId is doubly optional: absent (nil RawMessage) keeps the current
	// parent, present-null detaches it, present-value re-parents.
	var body struct {
		Name     *string         `json:"name"`
		ParentID json.RawMessage `json:"parentId"`
		Position *int32          `json:"position"`
	}
	if err := decodeJSON(r, &body); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	next := sqlc.UpdateCollectionParams{
		ID:        cid,
		ProjectID: projectID,
		Name:      cur.Name,
		ParentID:  cur.ParentID,
		Position:  cur.Position,
	}
	if body.Name != nil {
		next.Name = *body.Name
	}
	if body.Position != nil {
		next.Position = *body.Position
	}
	if body.ParentID != nil {
		pg, err := parseNullableUUID(body.ParentID)
		if err != nil {
			httpx.WriteError(w, r, httpx.ErrBadRequest("validation_failed", "parentId 不是有效的 id", nil))
			return
		}
		// A collection can never be its own parent — that would orphan the
		// subtree behind a cycle the tree projection can't render.
		if pg.Valid && pg.Bytes == cid {
			httpx.WriteError(w, r, httpx.ErrBadRequest("validation_failed", "分组不能作为自己的父级", nil))
			return
		}
		next.ParentID = pg
	}
	row, err := a.d.Queries.UpdateCollection(r.Context(), next)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"collection": toCollectionDTO(row)})
}

func (a *API) deleteCollection(w http.ResponseWriter, r *http.Request) {
	projectID, ok := a.loadOwnedProject(w, r)
	if !ok {
		return
	}
	cid, err := uuid.Parse(r.PathValue("cid"))
	if err != nil {
		httpx.WriteError(w, r, httpx.ErrNotFound("资源不存在"))
		return
	}
	if err := a.d.Queries.DeleteCollection(r.Context(), sqlc.DeleteCollectionParams{ID: cid, ProjectID: projectID}); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// -- References -------------------------------------------------------------

func (a *API) createReference(w http.ResponseWriter, r *http.Request) {
	projectID, ok := a.loadOwnedProject(w, r)
	if !ok {
		return
	}
	var body struct {
		Title          string   `json:"title"`
		URL            string   `json:"url"`
		Classification string   `json:"classification"`
		Author         string   `json:"author"`
		Credentials    string   `json:"credentials"`
		Year           string   `json:"year"`
		CollectionID   *string  `json:"collectionId"`
		Pending        bool     `json:"pending"`
		SearchHints    []string `json:"searchHints"`
		Tags           []string `json:"tags"`
	}
	if err := decodeJSON(r, &body); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	col, err := stringPtrToPgUUID(body.CollectionID)
	if err != nil {
		httpx.WriteError(w, r, httpx.ErrBadRequest("validation_failed", "collectionId 不是有效的 id", nil))
		return
	}
	row, err := a.d.Queries.CreateReference(r.Context(), sqlc.CreateReferenceParams{
		ProjectID:      projectID,
		Title:          body.Title,
		Classification: body.Classification,
		Author:         body.Author,
		Credentials:    body.Credentials,
		Year:           body.Year,
		Url:            body.URL,
		Tags:           stringsToJSONB(body.Tags),
		CollectionID:   col,
		Credibility:    nil,
		Evaluation:     "",
		Decision:       nil,
		Pending:        body.Pending,
		SearchHints:    stringsToJSONB(body.SearchHints),
	})
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, map[string]any{"reference": toReferenceDTO(row, nil)})
}

func (a *API) patchReference(w http.ResponseWriter, r *http.Request) {
	projectID, ok := a.loadOwnedProject(w, r)
	if !ok {
		return
	}
	rid, err := uuid.Parse(r.PathValue("rid"))
	if err != nil {
		httpx.WriteError(w, r, httpx.ErrNotFound("资源不存在"))
		return
	}
	cur, err := a.d.Queries.GetReference(r.Context(), sqlc.GetReferenceParams{ID: rid, ProjectID: projectID})
	if err != nil {
		httpx.WriteError(w, r, httpx.ErrNotFound("资源不存在"))
		return
	}
	// Nullable enum/link fields use json.RawMessage so a present-null can clear
	// them (toggle a chip off, drag out of a collection); a plain pointer can't
	// tell absent from null. Plain fields keep their absent-means-keep pointer.
	var body struct {
		Title          *string         `json:"title"`
		Classification *string         `json:"classification"`
		Author         *string         `json:"author"`
		Credentials    *string         `json:"credentials"`
		Year           *string         `json:"year"`
		URL            *string         `json:"url"`
		Tags           *[]string       `json:"tags"`
		CollectionID   json.RawMessage `json:"collectionId"`
		Credibility    json.RawMessage `json:"credibility"`
		Evaluation     *string         `json:"evaluation"`
		Decision       json.RawMessage `json:"decision"`
		Pending        *bool           `json:"pending"`
		SearchHints    *[]string       `json:"searchHints"`
	}
	if err := decodeJSON(r, &body); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	next := sqlc.UpdateReferenceParams{
		ID:             rid,
		ProjectID:      projectID,
		Title:          cur.Title,
		Classification: cur.Classification,
		Author:         cur.Author,
		Credentials:    cur.Credentials,
		Year:           cur.Year,
		Url:            cur.Url,
		Tags:           cur.Tags,
		CollectionID:   cur.CollectionID,
		Credibility:    cur.Credibility,
		Evaluation:     cur.Evaluation,
		Decision:       cur.Decision,
		Pending:        cur.Pending,
		SearchHints:    cur.SearchHints,
	}
	if body.Title != nil {
		next.Title = *body.Title
	}
	if body.Classification != nil {
		next.Classification = *body.Classification
	}
	if body.Author != nil {
		next.Author = *body.Author
	}
	if body.Credentials != nil {
		next.Credentials = *body.Credentials
	}
	if body.Year != nil {
		next.Year = *body.Year
	}
	if body.URL != nil {
		next.Url = *body.URL
	}
	if body.Evaluation != nil {
		next.Evaluation = *body.Evaluation
	}
	if body.Pending != nil {
		next.Pending = *body.Pending
	}
	if body.Tags != nil {
		next.Tags = stringsToJSONB(*body.Tags)
	}
	if body.SearchHints != nil {
		next.SearchHints = stringsToJSONB(*body.SearchHints)
	}
	if body.Credibility != nil {
		v, err := parseNullableEnum(body.Credibility, validCredibility)
		if err != nil {
			httpx.WriteError(w, r, httpx.ErrBadRequest("validation_failed", "credibility 只能是 strong/mixed/weak", nil))
			return
		}
		next.Credibility = v
	}
	if body.Decision != nil {
		v, err := parseNullableEnum(body.Decision, validDecision)
		if err != nil {
			httpx.WriteError(w, r, httpx.ErrBadRequest("validation_failed", "decision 只能是 use/maybe/drop", nil))
			return
		}
		next.Decision = v
	}
	if body.CollectionID != nil {
		pg, err := parseNullableUUID(body.CollectionID)
		if err != nil {
			httpx.WriteError(w, r, httpx.ErrBadRequest("validation_failed", "collectionId 不是有效的 id", nil))
			return
		}
		next.CollectionID = pg
	}

	row, err := a.d.Queries.UpdateReference(r.Context(), next)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	// Notes re-project from the reference's (possibly unchanged) material.
	var notes []readingNoteDTO
	if row.MaterialID.Valid {
		notes = a.notesByMaterial(r, projectID)[uuid.UUID(row.MaterialID.Bytes).String()]
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"reference": toReferenceDTO(row, notes)})
}

func (a *API) deleteReference(w http.ResponseWriter, r *http.Request) {
	projectID, ok := a.loadOwnedProject(w, r)
	if !ok {
		return
	}
	rid, err := uuid.Parse(r.PathValue("rid"))
	if err != nil {
		httpx.WriteError(w, r, httpx.ErrNotFound("资源不存在"))
		return
	}
	if err := a.d.Queries.DeleteReference(r.Context(), sqlc.DeleteReferenceParams{ID: rid, ProjectID: projectID}); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// -- enter-reading ----------------------------------------------------------

// enterReading is the bridge from a reference row into the existing Reading
// Room. It ensures the reference has a readable material — reusing an already
// linked one, else fetching+creating from its URL (materials.go's fetch path),
// else 422 when there is nothing to read — then returns the full MaterialSource
// DTO (studio.ProjectMaterials, so anchors/timeSpentS/etc. come back real when
// present, zero/false-defaulted otherwise). A timeline breadcrumb is dropped for
// every open.
func (a *API) enterReading(w http.ResponseWriter, r *http.Request) {
	projectID, ok := a.loadOwnedProject(w, r)
	if !ok {
		return
	}
	rid, err := uuid.Parse(r.PathValue("rid"))
	if err != nil {
		httpx.WriteError(w, r, httpx.ErrNotFound("资源不存在"))
		return
	}
	ref, err := a.d.Queries.GetReference(r.Context(), sqlc.GetReferenceParams{ID: rid, ProjectID: projectID})
	if err != nil {
		httpx.WriteError(w, r, httpx.ErrNotFound("资源不存在"))
		return
	}

	var materialID uuid.UUID
	switch {
	case ref.MaterialID.Valid:
		materialID = uuid.UUID(ref.MaterialID.Bytes)
	case strings.TrimSpace(ref.Url) != "":
		mid, ferr := a.fetchMaterialForReference(w, r, projectID, ref)
		if ferr {
			return // response already written
		}
		materialID = mid
	default:
		// No linked material and no URL to fetch: there is nothing to read.
		httpx.WriteJSON(w, http.StatusUnprocessableEntity, map[string]string{
			"error": "这条来源还没有可读内容——先补一个链接或粘贴正文",
		})
		return
	}

	// Project the material's dossier state — real anchors/timeSpentS/lateralRead
	// when it already has them, zero/false defaults for a freshly fetched one.
	d, err := studio.Load(r.Context(), a.d.Queries, projectID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	var dto *studio.MaterialDTO
	mats := studio.ProjectMaterials(d)
	for i := range mats {
		if mats[i].ID == materialID.String() {
			dto = &mats[i]
			break
		}
	}
	if dto == nil {
		httpx.WriteError(w, r, httpx.ErrNotFound("资源不存在"))
		return
	}

	title := strings.TrimSpace(ref.Title)
	if title == "" {
		title = dto.Title
	}
	if err := a.appendAutoLog(r.Context(), a.d.Queries, projectID, "打开来源《"+title+"》进入阅读室"); err != nil {
		slog.Warn("enter-reading: append auto-log failed",
			"err", err, "request_id", httpx.RequestIDFromContext(r.Context()))
	}
	httpx.WriteJSON(w, http.StatusOK, dto)
}

// fetchMaterialForReference fetches ref.Url, creates the material + its
// source_log_entry, and binds ref.material_id — all in one transaction (a
// material with no log entry is a source that was never "opened", RL-2's
// forbidden state, exactly as ingestMaterial guards). Returns the new material
// id; on any error it writes the response itself and returns failed=true. Gates
// on HasEntitlement — this spends network, same seam as ingestMaterial.
func (a *API) fetchMaterialForReference(w http.ResponseWriter, r *http.Request, projectID uuid.UUID, ref sqlc.Reference) (uuid.UUID, bool) {
	u, _ := UserFromContext(r.Context())
	entitled, err := HasEntitlement(r.Context(), u)
	if err != nil {
		httpx.WriteError(w, r, err)
		return uuid.UUID{}, true
	}
	if !entitled {
		httpx.WriteError(w, r, httpx.ErrNotEntitled())
		return uuid.UUID{}, true
	}

	t, body, ferr := a.d.Fetcher.FetchReadable(r.Context(), ref.Url)
	if ferr != nil {
		// 422 (not 400) + a standard {error:{code,message}} envelope so the
		// reading-room client can parse code="fetch_failed" and offer its inline
		// paste-body box instead of a generic error.
		httpx.WriteError(w, r, &httpx.APIError{
			Status:  http.StatusUnprocessableEntity,
			Code:    "fetch_failed",
			Message: "取不到这个链接的正文，可以直接把正文粘进来。",
		})
		return uuid.UUID{}, true
	}
	blocks := materialize.Segment(body)
	if len(blocks) == 0 {
		httpx.WriteError(w, r, httpx.ErrBadRequest("empty_body", "正文是空的。", nil))
		return uuid.UUID{}, true
	}
	title := t
	if title == "" {
		title = strings.TrimSpace(ref.Title)
	}
	if title == "" {
		title = ref.Url
	}
	rawBlocks, err := json.Marshal(blocks)
	if err != nil {
		httpx.WriteError(w, r, err)
		return uuid.UUID{}, true
	}
	url := ref.Url

	tx, err := a.d.Pool.Begin(r.Context())
	if err != nil {
		httpx.WriteError(w, r, err)
		return uuid.UUID{}, true
	}
	defer func() { _ = tx.Rollback(r.Context()) }()
	qtx := a.d.Queries.WithTx(tx)

	mat, err := qtx.CreateProjectMaterial(r.Context(), sqlc.CreateProjectMaterialParams{
		TaskID:    pgtype.UUID{},
		ProjectID: pgtype.UUID{Bytes: projectID, Valid: true},
		Kind:      "article",
		Source:    "fetched",
		Title:     title,
		SourceUrl: &url,
		Blocks:    rawBlocks,
	})
	if err != nil {
		httpx.WriteError(w, r, err)
		return uuid.UUID{}, true
	}
	if _, err := qtx.CreateSourceLogEntry(r.Context(), sqlc.CreateSourceLogEntryParams{
		ProjectID:  projectID,
		MaterialID: pgtype.UUID{Bytes: mat.ID, Valid: true},
		Url:        ref.Url,
		Title:      title,
	}); err != nil {
		httpx.WriteError(w, r, err)
		return uuid.UUID{}, true
	}
	if _, err := qtx.SetReferenceMaterial(r.Context(), sqlc.SetReferenceMaterialParams{
		ID:         ref.ID,
		ProjectID:  projectID,
		MaterialID: pgtype.UUID{Bytes: mat.ID, Valid: true},
	}); err != nil {
		httpx.WriteError(w, r, err)
		return uuid.UUID{}, true
	}
	if err := tx.Commit(r.Context()); err != nil {
		httpx.WriteError(w, r, err)
		return uuid.UUID{}, true
	}
	return mat.ID, false
}

// -- paste-content ----------------------------------------------------------

// pasteContent is the reading-room fallback for a URL that can't be fetched
// (enter-reading's 422 fetch_failed): the student pastes the article body and
// we create a material (source="pasted") from it via the exact paste path
// ingestMaterial uses (materialize.Segment → blocks → material + its
// source_log_entry in one tx, RL-2: never a material without a log entry), link
// it to the reference, and return the full MaterialSource DTO so the room can
// open it immediately. No model/network spend — plain owned-project write, so no
// entitlement gate (unlike enter-reading's fetch branch).
func (a *API) pasteContent(w http.ResponseWriter, r *http.Request) {
	projectID, ok := a.loadOwnedProject(w, r)
	if !ok {
		return
	}
	rid, err := uuid.Parse(r.PathValue("rid"))
	if err != nil {
		httpx.WriteError(w, r, httpx.ErrNotFound("资源不存在"))
		return
	}
	ref, err := a.d.Queries.GetReference(r.Context(), sqlc.GetReferenceParams{ID: rid, ProjectID: projectID})
	if err != nil {
		httpx.WriteError(w, r, httpx.ErrNotFound("资源不存在"))
		return
	}

	var body struct {
		Text  string `json:"text"`
		Title string `json:"title"`
	}
	if err := decodeJSON(r, &body); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if strings.TrimSpace(body.Text) == "" {
		httpx.WriteError(w, r, httpx.ErrBadRequest("validation_failed", "把正文粘进来再打开。", nil))
		return
	}

	blocks := materialize.Segment(body.Text)
	if len(blocks) == 0 {
		httpx.WriteError(w, r, httpx.ErrBadRequest("empty_body", "正文是空的。", nil))
		return
	}
	title := strings.TrimSpace(body.Title)
	if title == "" {
		title = strings.TrimSpace(ref.Title)
	}
	if title == "" {
		title = "粘贴的正文"
	}
	rawBlocks, err := json.Marshal(blocks)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}

	// One transaction: material + its source_log_entry + the reference link land
	// together (a material with no log entry is the RL-2-forbidden "never opened"
	// state, exactly as ingestMaterial/fetchMaterialForReference guard).
	tx, err := a.d.Pool.Begin(r.Context())
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	defer func() { _ = tx.Rollback(r.Context()) }()
	qtx := a.d.Queries.WithTx(tx)

	mat, err := qtx.CreateProjectMaterial(r.Context(), sqlc.CreateProjectMaterialParams{
		TaskID:    pgtype.UUID{},
		ProjectID: pgtype.UUID{Bytes: projectID, Valid: true},
		Kind:      "article",
		Source:    "pasted",
		Title:     title,
		SourceUrl: nil,
		Blocks:    rawBlocks,
	})
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if _, err := qtx.CreateSourceLogEntry(r.Context(), sqlc.CreateSourceLogEntryParams{
		ProjectID:  projectID,
		MaterialID: pgtype.UUID{Bytes: mat.ID, Valid: true},
		Url:        ref.Url,
		Title:      title,
	}); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if _, err := qtx.SetReferenceMaterial(r.Context(), sqlc.SetReferenceMaterialParams{
		ID:         ref.ID,
		ProjectID:  projectID,
		MaterialID: pgtype.UUID{Bytes: mat.ID, Valid: true},
	}); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if err := tx.Commit(r.Context()); err != nil {
		httpx.WriteError(w, r, err)
		return
	}

	// Project the material's dossier state (fresh → zero/false defaults), exactly
	// as enter-reading returns it.
	d, err := studio.Load(r.Context(), a.d.Queries, projectID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	var dto *studio.MaterialDTO
	mats := studio.ProjectMaterials(d)
	for i := range mats {
		if mats[i].ID == mat.ID.String() {
			dto = &mats[i]
			break
		}
	}
	if dto == nil {
		httpx.WriteError(w, r, httpx.ErrInternal())
		return
	}
	if err := a.appendAutoLog(r.Context(), a.d.Queries, projectID, "粘贴正文《"+title+"》"); err != nil {
		slog.Warn("paste-content: append auto-log failed",
			"err", err, "request_id", httpx.RequestIDFromContext(r.Context()))
	}
	httpx.WriteJSON(w, http.StatusOK, dto)
}

// -- shared jsonb helpers ---------------------------------------------------

// jsonbToStrings decodes a jsonb string array into []string, never nil.
func jsonbToStrings(b []byte) []string {
	out := []string{}
	if len(b) == 0 {
		return out
	}
	_ = json.Unmarshal(b, &out)
	if out == nil {
		out = []string{}
	}
	return out
}

// stringsToJSONB marshals a []string for a jsonb column (NOT NULL DEFAULT '[]');
// nil/error both fall back to an empty array so the column is never null.
func stringsToJSONB(s []string) []byte {
	if s == nil {
		s = []string{}
	}
	b, err := json.Marshal(s)
	if err != nil {
		return []byte("[]")
	}
	return b
}

// parseNullableEnum reads a nullable enum field off a PATCH body: "null" (or an
// empty string) clears it (nil); any other value must be in valid, else error.
func parseNullableEnum(raw json.RawMessage, valid map[string]bool) (*string, error) {
	if string(raw) == "null" {
		return nil, nil
	}
	var s string
	if err := json.Unmarshal(raw, &s); err != nil {
		return nil, err
	}
	if strings.TrimSpace(s) == "" {
		return nil, nil
	}
	if !valid[s] {
		return nil, fmt.Errorf("invalid enum value %q", s)
	}
	return &s, nil
}

// parseNullableUUID reads a nullable id field off a PATCH body: "null" (or an
// empty string) clears it; any other value must parse as a UUID, else error.
func parseNullableUUID(raw json.RawMessage) (pgtype.UUID, error) {
	if string(raw) == "null" {
		return pgtype.UUID{}, nil
	}
	var s string
	if err := json.Unmarshal(raw, &s); err != nil {
		return pgtype.UUID{}, err
	}
	if strings.TrimSpace(s) == "" {
		return pgtype.UUID{}, nil
	}
	id, err := uuid.Parse(s)
	if err != nil {
		return pgtype.UUID{}, err
	}
	return pgtype.UUID{Bytes: id, Valid: true}, nil
}
