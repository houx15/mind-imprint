package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"mindimprint/api/internal/httpx"
	"mindimprint/api/internal/store/sqlc"
)

// readings.go — the lite edition's 阅读 atom: one student's single reading
// exercise. It owns its storage outright (atom + reading + the shared atom_*
// machinery); it does NOT borrow a project row and never touches any
// project-scoped table.

type readingDTO struct {
	ID         string  `json:"id"` // the ATOM id — every reading endpoint is keyed by it
	Title      string  `json:"title"`
	Lang       string  `json:"lang"`
	Status     string  `json:"status"`
	HasSource  bool    `json:"hasSource"`
	CreatedAt  string  `json:"createdAt"`
	UpdatedAt  string  `json:"updatedAt"`
	FinishedAt *string `json:"finishedAt"`
}

func (a *API) readingDTOOf(rd sqlc.Reading, hasSource bool, createdAt time.Time) readingDTO {
	out := readingDTO{
		ID: rd.AtomID.String(), Title: rd.Title, Lang: rd.Lang, Status: rd.Status,
		HasSource: hasSource,
		CreatedAt: createdAt.Format(time.RFC3339),
		UpdatedAt: rd.UpdatedAt.Format(time.RFC3339),
	}
	if rd.FinishedAt.Valid {
		s := rd.FinishedAt.Time.Format(time.RFC3339)
		out.FinishedAt = &s
	}
	return out
}

// loadOwnedReadingAtom parses {id} as an atom of kind 'reading' owned by the
// caller. Every failure — malformed id, missing row, wrong kind, wrong owner —
// is a flat 404, so atom existence is never leaked. Shared by Tasks 3-8.
func (a *API) loadOwnedReadingAtom(w http.ResponseWriter, r *http.Request) (sqlc.Atom, bool) {
	u, _ := UserFromContext(r.Context())
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		httpx.WriteError(w, r, httpx.ErrNotFound("资源不存在"))
		return sqlc.Atom{}, false
	}
	at, err := a.d.Queries.GetAtom(r.Context(), id)
	if err != nil {
		httpx.WriteError(w, r, err) // pgx.ErrNoRows → 404
		return sqlc.Atom{}, false
	}
	if at.Kind != "reading" || at.UserID != u.ID {
		httpx.WriteError(w, r, httpx.ErrNotFound("资源不存在"))
		return sqlc.Atom{}, false
	}
	return at, true
}

// hasSource reports whether the article body has been pasted yet. A missing
// row is a legitimate state (a brand-new reading), not an error.
func (a *API) hasSource(r *http.Request, atomID uuid.UUID) (bool, error) {
	if _, err := a.d.Queries.GetReadingSource(r.Context(), atomID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func (a *API) createReading(w http.ResponseWriter, r *http.Request) {
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
	var req struct {
		Title string `json:"title"`
		Lang  string `json:"lang"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteError(w, r, httpx.ErrBadRequest("bad_json", "请求格式不对", nil))
		return
	}
	title := strings.TrimSpace(req.Title)
	if title == "" {
		title = "未命名阅读"
	}
	if len([]rune(title)) > 200 {
		title = string([]rune(title)[:200])
	}
	lang := strings.TrimSpace(req.Lang)
	if lang != "zh" && lang != "en" {
		lang = "zh"
	}

	// atom + reading in ONE transaction: an atom with no reading row would be
	// an identity nothing can render.
	tx, err := a.d.Pool.Begin(r.Context())
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	defer func() { _ = tx.Rollback(r.Context()) }()
	qtx := a.d.Queries.WithTx(tx)

	at, err := qtx.CreateAtom(r.Context(), sqlc.CreateAtomParams{Kind: "reading", UserID: u.ID})
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if _, err := qtx.CreateReading(r.Context(), sqlc.CreateReadingParams{
		AtomID: at.ID, Title: title, Lang: lang,
	}); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if err := tx.Commit(r.Context()); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, map[string]any{"id": at.ID.String()})
}

func (a *API) listReadings(w http.ResponseWriter, r *http.Request) {
	u, _ := UserFromContext(r.Context())
	rows, err := a.d.Queries.ListReadingsByUser(r.Context(), u.ID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	out := make([]readingDTO, 0, len(rows))
	for _, row := range rows {
		hasSrc, err := a.hasSource(r, row.AtomID)
		if err != nil {
			httpx.WriteError(w, r, err)
			return
		}
		out = append(out, a.readingDTOOf(sqlc.Reading{
			AtomID: row.AtomID, Title: row.Title, Lang: row.Lang,
			Status: row.Status, UpdatedAt: row.UpdatedAt, FinishedAt: row.FinishedAt,
		}, hasSrc, row.AtomCreatedAt))
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"readings": out})
}

func (a *API) getReading(w http.ResponseWriter, r *http.Request) {
	at, ok := a.loadOwnedReadingAtom(w, r)
	if !ok {
		return
	}
	rd, err := a.d.Queries.GetReading(r.Context(), at.ID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	hasSrc, err := a.hasSource(r, at.ID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, a.readingDTOOf(rd, hasSrc, at.CreatedAt))
}

func (a *API) renameReading(w http.ResponseWriter, r *http.Request) {
	at, ok := a.loadOwnedReadingAtom(w, r)
	if !ok {
		return
	}
	var req struct {
		Title string `json:"title"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteError(w, r, httpx.ErrBadRequest("bad_json", "请求格式不对", nil))
		return
	}
	title := strings.TrimSpace(req.Title)
	if title == "" {
		httpx.WriteError(w, r, httpx.ErrBadRequest("missing_title", "标题不能为空。", nil))
		return
	}
	if len([]rune(title)) > 200 {
		title = string([]rune(title)[:200])
	}
	if err := a.d.Queries.RenameReading(r.Context(), sqlc.RenameReadingParams{AtomID: at.ID, Title: title}); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	rd, err := a.d.Queries.GetReading(r.Context(), at.ID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	hasSrc, err := a.hasSource(r, at.ID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, a.readingDTOOf(rd, hasSrc, at.CreatedAt))
}
