package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

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
// nullable links. phaseTag/takeaway (S2, Task 8) fold the reading sub-agent's
// state onto the same row the library already renders: phaseTag is the reading
// brief's phase (null until the student sets one via putReadingBrief), takeaway
// is the full structured 5-field object (null until postFinalizeReading — this
// is the same field the interim finalize response used to echo as a top-level
// sibling key; folding it here removes that duplication). readingReason/
// readingFocus (Task 9 fix) surface the SAME persisted brief fields
// putReadingBrief writes, so a client reopening a source can seed its editor
// from the true saved values instead of re-deriving a stale default — without
// this, a full-replace PUT of the brief silently overwrites whichever field
// the client didn't have available to resend.
type referenceDTO struct {
	ID             string                 `json:"id"`
	Title          string                 `json:"title"`
	Classification string                 `json:"classification"`
	Author         string                 `json:"author"`
	Credentials    string                 `json:"credentials"`
	Year           string                 `json:"year"`
	URL            string                 `json:"url"`
	Tags           []string               `json:"tags"`
	CollectionID   *string                `json:"collectionId"`
	Credibility    *string                `json:"credibility"`
	Evaluation     string                 `json:"evaluation"`
	ReadingNote    string                 `json:"readingNote"`
	Decision       *string                `json:"decision"`
	Pending        bool                   `json:"pending"`
	SearchHints    []string               `json:"searchHints"`
	MaterialID     *string                `json:"materialId"`
	Notes          []readingNoteDTO       `json:"notes"`
	PhaseTag       *string                `json:"phaseTag"`
	ReadingReason  *string                `json:"readingReason"`
	ReadingFocus   *string                `json:"readingFocus"`
	Takeaway       *agent.ReadingTakeaway `json:"takeaway"`
	// #4: the Crossref abstract + journal recovered from a DOI (persisted on the
	// reference row; "" until a DOI resolved). The abstract is context for the
	// reading room / library preview, not the article body; journal fills the
	// annotated bib.
	Abstract string `json:"abstract"`
	Journal  string `json:"journal"`
	// A1: one shelf, three states (待读/在读/读完). NOT NULL with a DB default —
	// every reference always has a value, never null.
	ReadingStatus string `json:"readingStatus"`
	// Slice 4a · 证据地图 facets (all "" / false by default).
	Triage            string `json:"triage"`
	EvidenceNature    string `json:"evidenceNature"`
	EvidenceArgument  string `json:"evidenceArgument"`
	EvidenceFinding   string `json:"evidenceFinding"`
	EvidencePlacement string `json:"evidencePlacement"`
	Archived          bool   `json:"archived"`
}

func toReferenceDTO(row sqlc.Reference, notes []readingNoteDTO) referenceDTO {
	if notes == nil {
		notes = []readingNoteDTO{}
	}
	var takeaway *agent.ReadingTakeaway
	if row.TakeawayFinalizedAt.Valid && len(row.Takeaway) > 0 {
		var tk agent.ReadingTakeaway
		if json.Unmarshal(row.Takeaway, &tk) == nil {
			// Heal rows finalized before the non-nil fix: a stored null slice
			// unmarshals to nil and re-marshals to null, which the client
			// z.array contract rejects — one bad row blanks the whole library.
			if tk.Findings == nil {
				tk.Findings = []string{}
			}
			if tk.KeyQuotes == nil {
				tk.KeyQuotes = []agent.KeyQuote{}
			}
			if tk.NewLeads == nil {
				tk.NewLeads = []string{}
			}
			takeaway = &tk
		}
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
		ReadingNote:    row.ReadingNote,
		Decision:       row.Decision,
		Pending:        row.Pending,
		SearchHints:    jsonbToStrings(row.SearchHints),
		MaterialID:     pgUUIDToStringPtr(row.MaterialID),
		Notes:          notes,
		PhaseTag:       row.PhaseTag,
		ReadingReason:  row.ReadingReason,
		ReadingFocus:   row.ReadingFocus,
		Takeaway:       takeaway,
		Abstract:          row.Abstract,
		Journal:           row.Journal,
		ReadingStatus:     row.ReadingStatus,
		Triage:            row.Triage,
		EvidenceNature:    row.EvidenceNature,
		EvidenceArgument:  row.EvidenceArgument,
		EvidenceFinding:   row.EvidenceFinding,
		EvidencePlacement: row.EvidencePlacement,
		Archived:          row.Archived,
	}
}

var validCredibility = map[string]bool{"strong": true, "mixed": true, "weak": true}
var validDecision = map[string]bool{"use": true, "maybe": true, "drop": true}
var validReadingStatus = map[string]bool{"to_read": true, "reading": true, "done": true}

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

	// Finding D · when the student pastes a DOI (bare, or a doi.org URL) into the
	// 链接/DOI field, resolve it to real bibliographic metadata via Crossref so the
	// library shows the paper's title/author/year — not the raw DOI string. The
	// client can't do this (no external calls / CORS); the platform resolves it
	// server-side. Best-effort with a short budget: any failure just stores what
	// the student typed, unchanged. Never overwrites a title the student typed
	// (only fills a blank one, or one that is just the DOI/url echoed back).
	title, classification := body.Title, body.Classification
	var doiMeta *materialize.DOIMeta
	if a.d.Fetcher != nil {
		if doi, ok := materialize.DetectDOI(body.URL); ok {
			dctx, cancel := context.WithTimeout(r.Context(), 6*time.Second)
			doiMeta = a.d.Fetcher.ResolveDOI(dctx, doi)
			cancel()
			if doiMeta != nil && strings.TrimSpace(doiMeta.Title) != "" {
				echoed := strings.TrimSpace(title) == "" ||
					strings.TrimSpace(title) == strings.TrimSpace(body.URL) ||
					strings.TrimSpace(title) == doi
				if echoed {
					title = doiMeta.Title
				}
				if strings.TrimSpace(classification) == "" || classification == "网页" {
					classification = "期刊论文"
				}
			}
		}
	}

	row, err := a.d.Queries.CreateReference(r.Context(), sqlc.CreateReferenceParams{
		ProjectID:      projectID,
		Title:          title,
		Classification: classification,
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
	// Fill author/year/journal/abstract from the resolved DOI metadata (only
	// where the student left them blank — patchReferenceMeta never overwrites).
	if doiMeta != nil {
		a.patchReferenceMeta(r.Context(), projectID, row, doiMeta)
		if refreshed, rerr := a.d.Queries.GetReference(r.Context(), sqlc.GetReferenceParams{ID: row.ID, ProjectID: projectID}); rerr == nil {
			row = refreshed
		}
	}
	// Mechanism-2 mutation event: source_added — the "when + phase + motivation
	// each material was added" signal. `provenance` defaults to "manual": this
	// create body carries no provenance/source hint today (a frontend hint like
	// "chat_link"/"dig" can be wired into the request body later without
	// changing this call site).
	a.emitMutation(r.Context(), projectID, "source_added", map[string]any{
		"referenceId":    row.ID.String(),
		"title":          row.Title,
		"classification": row.Classification,
		"provenance":     "manual",
	})
	// 过程即数据: adding a source is a real research milestone — log it so the
	// 活动日志 reflects the whole journey, not just framework/plan events. Use the
	// title if present, else the raw url/DOI (best-effort; never fails the write).
	label := strings.TrimSpace(row.Title)
	if label == "" {
		label = strings.TrimSpace(row.Url)
	}
	if label != "" {
		if err := a.appendAutoLog(r.Context(), a.d.Queries, projectID, "添加来源《"+truncateRunes(label, 30)+"》"); err != nil {
			slog.Warn("create-reference: append auto-log failed", "err", err, "request_id", httpx.RequestIDFromContext(r.Context()))
		}
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
		ReadingNote    *string         `json:"readingNote"`
		ReadingStatus  *string         `json:"readingStatus"`
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
		ReadingNote:    cur.ReadingNote,
		Abstract:       cur.Abstract,
		Journal:        cur.Journal,
		ReadingStatus:  cur.ReadingStatus,
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
	if body.ReadingNote != nil {
		next.ReadingNote = *body.ReadingNote
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
	if body.ReadingStatus != nil {
		if !validReadingStatus[*body.ReadingStatus] {
			httpx.WriteError(w, r, httpx.ErrBadRequest("validation_failed", "readingStatus 只能是 to_read/reading/done", nil))
			return
		}
		next.ReadingStatus = *body.ReadingStatus
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
	// Mechanism-2 mutation event: source_reclassified — only when decision or
	// classification actually changed (an untouched-field patch, e.g. author,
	// must not emit). Compares against `cur`, read before the mutation.
	if body.Classification != nil && cur.Classification != row.Classification {
		a.emitMutation(r.Context(), projectID, "source_reclassified", map[string]any{
			"referenceId": rid.String(), "field": "classification",
			"before": cur.Classification, "after": row.Classification,
		})
	}
	if body.Decision != nil {
		before, after := "", ""
		if cur.Decision != nil {
			before = *cur.Decision
		}
		if row.Decision != nil {
			after = *row.Decision
		}
		if before != after {
			a.emitMutation(r.Context(), projectID, "source_reclassified", map[string]any{
				"referenceId": rid.String(), "field": "decision",
				"before": cur.Decision, "after": row.Decision,
			})
		}
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
	// Read the title BEFORE deleting so the source_dropped payload can carry
	// it (best-effort: an unfound reference just skips the emit below, since
	// DeleteReference itself is a no-op DELETE that never errors on 0 rows).
	title, found := "", false
	if cur, cerr := a.d.Queries.GetReference(r.Context(), sqlc.GetReferenceParams{ID: rid, ProjectID: projectID}); cerr == nil {
		title, found = cur.Title, true
	}
	if err := a.d.Queries.DeleteReference(r.Context(), sqlc.DeleteReferenceParams{ID: rid, ProjectID: projectID}); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if found {
		a.emitMutation(r.Context(), projectID, "source_dropped", map[string]any{
			"referenceId": rid.String(), "title": title,
		})
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

	// suggestedReason: a deterministic first-draft intention (student edits).
	// No spend — templated from the proposal objective + the source title. A
	// fresh project has no proposal row yet (pgx.ErrNoRows), tolerated as an
	// empty objective.
	prop, _ := a.d.Queries.GetProjectProposal(r.Context(), projectID)
	suggested := suggestReadingReason(prop.Objective, ref.Title)

	// Merge onto the existing flat MaterialSource response rather than nesting
	// it, so today's clients/tests decoding top-level fields keep working.
	raw, err := json.Marshal(dto)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	out["suggestedReason"] = suggested
	httpx.WriteJSON(w, http.StatusOK, out)
}

// fetchFailedError builds the 422 paste-fallback for a failed fetch, enriched
// with any DOI metadata Crossref returned (#4): the title, authors, year,
// journal, and abstract we recovered even though the full text couldn't be
// fetched — so the reading room shows them and asks the student to paste the
// body, rather than a bare "取不到正文".
func fetchFailedError(err error) *httpx.APIError {
	msg := "取不到这个链接的正文，可以直接把正文粘进来。"
	var details any
	var fe *materialize.FetchError
	if errors.As(err, &fe) && fe.Meta != nil {
		m := fe.Meta
		details = map[string]any{
			"title": m.Title, "author": m.Author, "year": m.Year,
			"journal": m.Journal, "abstract": m.Abstract,
		}
		if strings.TrimSpace(m.Abstract) != "" {
			msg = "只自动取到了这篇的元信息和摘要，正文取不到——看看摘要，把正文粘进来就能逐句共读。"
		} else if strings.TrimSpace(m.Title) != "" {
			msg = "取到了这篇的元信息，但正文取不到——把正文粘进来就能逐句共读。"
		}
	}
	return &httpx.APIError{Status: http.StatusUnprocessableEntity, Code: "fetch_failed", Message: msg, Details: details}
}

// patchReferenceMeta fills a reference's bibliographic fields from the DOI
// metadata we recovered (#4), so the annotated bib + reading-room header are
// populated whether or not the full text could be fetched. Abstract/journal are
// context we always persist when Crossref returned them (they have no
// student-authored counterpart to protect); author/year are only filled when
// the student left them blank (never overwrites her own). Best-effort: logs
// rather than surfaces errors.
func (a *API) patchReferenceMeta(ctx context.Context, projectID uuid.UUID, ref sqlc.Reference, m *materialize.DOIMeta) {
	if m == nil {
		return
	}
	author, year := ref.Author, ref.Year
	abstract, journal := ref.Abstract, ref.Journal
	changed := false
	if strings.TrimSpace(author) == "" && strings.TrimSpace(m.Author) != "" {
		author = m.Author
		changed = true
	}
	if strings.TrimSpace(year) == "" && strings.TrimSpace(m.Year) != "" {
		year = m.Year
		changed = true
	}
	// Abstract/journal: fill when we recovered them and don't already have them
	// (a re-read shouldn't clobber a longer stored abstract with a blank one).
	if strings.TrimSpace(abstract) == "" && strings.TrimSpace(m.Abstract) != "" {
		abstract = m.Abstract
		changed = true
	}
	if strings.TrimSpace(journal) == "" && strings.TrimSpace(m.Journal) != "" {
		journal = m.Journal
		changed = true
	}
	if !changed {
		return
	}
	if _, err := a.d.Queries.UpdateReference(ctx, sqlc.UpdateReferenceParams{
		ID: ref.ID, ProjectID: projectID,
		Title: ref.Title, Classification: ref.Classification, Author: author, Credentials: ref.Credentials,
		Year: year, Url: ref.Url, Tags: ref.Tags, CollectionID: ref.CollectionID,
		Credibility: ref.Credibility, Evaluation: ref.Evaluation, Decision: ref.Decision,
		Pending: ref.Pending, SearchHints: ref.SearchHints, ReadingNote: ref.ReadingNote,
		Abstract: abstract, Journal: journal, ReadingStatus: ref.ReadingStatus,
	}); err != nil {
		slog.Warn("patch reference meta failed", "err", err, "ref", ref.ID)
	}
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

	t, body, meta, ferr := a.d.Fetcher.FetchReadable(r.Context(), ref.Url)
	if ferr != nil {
		// #4 · fill the bib (author/year) from any recovered DOI metadata, then
		// 422 + a standard {error:{code,message,details}} envelope so the
		// reading-room client can parse code="fetch_failed" and offer its inline
		// paste-body box — enriched with the same metadata.
		var fe *materialize.FetchError
		if errors.As(ferr, &fe) && fe.Meta != nil {
			a.patchReferenceMeta(r.Context(), projectID, ref, fe.Meta)
		}
		httpx.WriteError(w, r, fetchFailedError(ferr))
		return uuid.UUID{}, true
	}
	// #4 · SUCCESS path: the full text WAS fetched, but a DOI still resolved to
	// Crossref metadata — persist the abstract/journal (+ author/year if the
	// student left them blank) so the annotated bib + reading-room header show
	// them, not just the failure fallback. Best-effort, before the material tx.
	if meta != nil {
		a.patchReferenceMeta(r.Context(), projectID, ref, meta)
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
