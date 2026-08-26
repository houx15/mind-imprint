package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"mindimprint/api/internal/httpx"
	"mindimprint/api/internal/store/sqlc"
)

// reading_notes.go — three small lite-edition endpoint groups that share a
// shape (upsert one row / append one row): brief (why-read-THIS), takeaway
// (the one-line "so what"), and annotations (span-anchored margin notes).
// Named with a `lite` prefix throughout — `putReadingBrief` and friends
// already exist for the pro side in reading_brief.go, in the same package.

// --- brief -----------------------------------------------------------------

type liteBriefDTO struct {
	PhaseTag      *string `json:"phaseTag"`
	ReadingReason string  `json:"readingReason"`
	ReadingFocus  string  `json:"readingFocus"`
}

func liteBriefDTOOf(row sqlc.ReadingBrief) liteBriefDTO {
	return liteBriefDTO{PhaseTag: row.PhaseTag, ReadingReason: row.ReadingReason, ReadingFocus: row.ReadingFocus}
}

// liteGetBrief returns the persisted brief, or a zero-value DTO when none has
// been saved yet. A brand-new reading has no brief — that is a normal state,
// not an error, so this must never 404 (unlike GET /source, where a reading
// with no article pasted is genuinely unusable).
func (a *API) liteGetBrief(w http.ResponseWriter, r *http.Request) {
	at, ok := a.loadOwnedReadingAtom(w, r)
	if !ok {
		return
	}
	row, err := a.d.Queries.GetReadingBrief(r.Context(), at.ID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			httpx.WriteJSON(w, http.StatusOK, liteBriefDTO{})
			return
		}
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, liteBriefDTOOf(row))
}

// litePutBrief is a full replacement, not a partial patch: the client always
// re-sends all three fields, so upserting only what's present would silently
// clear whatever this call omitted. Same semantics as the pro side's
// putReadingBrief (reading_brief.go).
func (a *API) litePutBrief(w http.ResponseWriter, r *http.Request) {
	at, ok := a.loadOwnedReadingAtom(w, r)
	if !ok {
		return
	}
	var req struct {
		PhaseTag      *string `json:"phaseTag"`
		ReadingReason string  `json:"readingReason"`
		ReadingFocus  string  `json:"readingFocus"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteError(w, r, httpx.ErrBadRequest("bad_json", "请求格式不对", nil))
		return
	}
	row, err := a.d.Queries.UpsertReadingBrief(r.Context(), sqlc.UpsertReadingBriefParams{
		AtomID:        at.ID,
		PhaseTag:      nullableText(derefOr(req.PhaseTag, "")),
		ReadingReason: strings.TrimSpace(req.ReadingReason),
		ReadingFocus:  strings.TrimSpace(req.ReadingFocus),
	})
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, liteBriefDTOOf(row))
}

// --- takeaway ----------------------------------------------------------

type liteTakeawayDTO struct {
	Text      string `json:"text"`
	UpdatedAt string `json:"updatedAt"`
}

func liteTakeawayDTOOf(row sqlc.ReadingTakeaway) liteTakeawayDTO {
	return liteTakeawayDTO{Text: row.Text, UpdatedAt: row.UpdatedAt.Format(time.RFC3339)}
}

// liteGetTakeaway mirrors liteGetBrief: no row yet is a normal, brand-new
// state, so it returns 200 + zero value rather than 404.
func (a *API) liteGetTakeaway(w http.ResponseWriter, r *http.Request) {
	at, ok := a.loadOwnedReadingAtom(w, r)
	if !ok {
		return
	}
	row, err := a.d.Queries.GetReadingTakeaway(r.Context(), at.ID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			httpx.WriteJSON(w, http.StatusOK, liteTakeawayDTO{})
			return
		}
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, liteTakeawayDTOOf(row))
}

// litePutTakeaway upserts the single takeaway row wholesale.
func (a *API) litePutTakeaway(w http.ResponseWriter, r *http.Request) {
	at, ok := a.loadOwnedReadingAtom(w, r)
	if !ok {
		return
	}
	var req struct {
		Text string `json:"text"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteError(w, r, httpx.ErrBadRequest("bad_json", "请求格式不对", nil))
		return
	}
	row, err := a.d.Queries.UpsertReadingTakeaway(r.Context(), sqlc.UpsertReadingTakeawayParams{
		AtomID: at.ID, Text: strings.TrimSpace(req.Text),
	})
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, liteTakeawayDTOOf(row))
}

// --- annotations -------------------------------------------------------

type liteAnnotationDTO struct {
	ID        string          `json:"id"`
	BlockID   string          `json:"blockId"`
	Span      json.RawMessage `json:"span"`
	Quote     string          `json:"quote"`
	Note      string          `json:"note"`
	CreatedAt string          `json:"createdAt"`
}

func liteAnnotationDTOOf(row sqlc.AtomAnnotation) liteAnnotationDTO {
	return liteAnnotationDTO{
		ID: row.ID.String(), BlockID: row.BlockID, Span: json.RawMessage(row.Span),
		Quote: row.Quote, Note: row.Note, CreatedAt: row.CreatedAt.Format(time.RFC3339),
	}
}

// liteListAnnotations lists every margin note on this reading, oldest first
// (ListAtomAnnotations orders by created_at).
func (a *API) liteListAnnotations(w http.ResponseWriter, r *http.Request) {
	at, ok := a.loadOwnedReadingAtom(w, r)
	if !ok {
		return
	}
	rows, err := a.d.Queries.ListAtomAnnotations(r.Context(), at.ID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	out := make([]liteAnnotationDTO, 0, len(rows))
	for _, row := range rows {
		out = append(out, liteAnnotationDTOOf(row))
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"annotations": out})
}

// liteCreateAnnotation appends one annotation. blockId is the only required
// field; quote and note may be empty (a student may anchor a note before
// writing it). span is stored verbatim as jsonb — Go validates only that it
// decodes as a JSON object, nothing deeper; the shape's truth lives in the
// frontend contract.
func (a *API) liteCreateAnnotation(w http.ResponseWriter, r *http.Request) {
	at, ok := a.loadOwnedReadingAtom(w, r)
	if !ok {
		return
	}
	var req struct {
		BlockID string          `json:"blockId"`
		Span    json.RawMessage `json:"span"`
		Quote   string          `json:"quote"`
		Note    string          `json:"note"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteError(w, r, httpx.ErrBadRequest("bad_json", "请求格式不对", nil))
		return
	}
	blockID := strings.TrimSpace(req.BlockID)
	if blockID == "" {
		httpx.WriteError(w, r, httpx.ErrBadRequest("missing_block_id", "block id 不能为空。", nil))
		return
	}
	var obj map[string]any
	if err := json.Unmarshal(req.Span, &obj); err != nil {
		httpx.WriteError(w, r, httpx.ErrBadRequest("invalid_span", "span 必须是一个 JSON 对象。", nil))
		return
	}
	row, err := a.d.Queries.CreateAtomAnnotation(r.Context(), sqlc.CreateAtomAnnotationParams{
		AtomID: at.ID, BlockID: blockID, Span: []byte(req.Span), Quote: req.Quote, Note: req.Note,
	})
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, liteAnnotationDTOOf(row))
}
