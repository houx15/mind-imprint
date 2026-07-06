package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"mindimprint/api/internal/httpx"
	"mindimprint/api/internal/materialize"
	"mindimprint/api/internal/store/sqlc"
)

const (
	maxPasteBytes   = 500 * 1024
	maxScratchBytes = 20 * 1024
)

func writeMaterialOrNotFound(w http.ResponseWriter, r *http.Request, m sqlc.Material, err error) {
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			httpx.WriteError(w, r, httpx.ErrNotFound("资源不存在"))
			return
		}
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"material": toMaterialDTO(m)})
}

func writeFetchFailed(w http.ResponseWriter, reason string) {
	httpx.WriteJSON(w, http.StatusUnprocessableEntity, map[string]any{
		"error": map[string]any{
			"code":    "material_fetch_failed",
			"message": "无法读取该链接",
			"details": map[string]any{"reason": reason},
		},
	})
}

func (a *API) listMaterials(w http.ResponseWriter, r *http.Request) {
	t, ok := a.loadOwnedTask(w, r)
	if !ok {
		return
	}
	rows, err := a.d.Queries.ListMaterialsByTask(r.Context(), t.ID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"materials": toMaterialDTOs(rows)})
}

func (a *API) createMaterial(w http.ResponseWriter, r *http.Request) {
	t, ok := a.loadOwnedTask(w, r)
	if !ok {
		return
	}
	var body struct {
		Kind  string `json:"kind"`
		Title string `json:"title"`
		Text  string `json:"text"`
	}
	if err := decodeJSON(r, &body); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if body.Kind != "article" && body.Kind != "draft" {
		httpx.WriteError(w, r, httpx.ErrBadRequest("validation_failed", "kind 非法", nil))
		return
	}
	if len(body.Text) == 0 || len(body.Text) > maxPasteBytes {
		httpx.WriteError(w, r, httpx.ErrBadRequest("validation_failed", "text 长度非法", nil))
		return
	}
	blocks := materialize.Segment(body.Text)
	if len(blocks) == 0 {
		httpx.WriteError(w, r, httpx.ErrBadRequest("validation_failed", "text 无有效内容", nil))
		return
	}
	title := strings.TrimSpace(body.Title)
	if title == "" {
		title = "未命名材料"
	}
	m, err := a.createMaterialRow(w, r, t.ID, body.Kind, "pasted", title, nil, blocks)
	if err != nil {
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, map[string]any{"material": toMaterialDTO(m)})
}

func (a *API) materialFromSeed(w http.ResponseWriter, r *http.Request) {
	t, ok := a.loadOwnedTask(w, r)
	if !ok {
		return
	}
	if t.Seed == nil || strings.TrimSpace(*t.Seed) == "" {
		writeFetchFailed(w, "no_seed")
		return
	}
	title, text, err := a.d.Fetcher.FetchReadable(r.Context(), *t.Seed)
	if err != nil {
		reason := "unreachable"
		var fe *materialize.FetchError
		if errors.As(err, &fe) {
			reason = fe.Reason
		}
		writeFetchFailed(w, reason)
		return
	}
	blocks := materialize.Segment(text)
	if len(blocks) == 0 {
		writeFetchFailed(w, "empty")
		return
	}
	if strings.TrimSpace(title) == "" {
		title = "来自链接的材料"
	}
	seed := *t.Seed
	m, err := a.createMaterialRow(w, r, t.ID, "article", "fetched", title, &seed, blocks)
	if err != nil {
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, map[string]any{"material": toMaterialDTO(m)})
}

// createMaterialRow marshals blocks and inserts; on error it writes the envelope
// and returns err (non-nil) so callers can bail.
func (a *API) createMaterialRow(w http.ResponseWriter, r *http.Request, taskID uuid.UUID, kind, source, title string, sourceURL *string, blocks []materialize.Block) (sqlc.Material, error) {
	raw, err := json.Marshal(blocks)
	if err != nil {
		httpx.WriteError(w, r, err)
		return sqlc.Material{}, err
	}
	m, err := a.d.Queries.CreateMaterial(r.Context(), sqlc.CreateMaterialParams{
		TaskID: taskID, Kind: kind, Source: source, Title: title, SourceUrl: sourceURL, Blocks: raw,
	})
	if err != nil {
		httpx.WriteError(w, r, err)
		return sqlc.Material{}, err
	}
	return m, nil
}

func (a *API) updateMaterialScratch(w http.ResponseWriter, r *http.Request) {
	t, ok := a.loadOwnedTask(w, r)
	if !ok {
		return
	}
	mid, err := uuid.Parse(r.PathValue("mid"))
	if err != nil {
		httpx.WriteError(w, r, httpx.ErrNotFound("资源不存在"))
		return
	}
	var body struct {
		Scratch string `json:"scratch"`
	}
	if err := decodeJSON(r, &body); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if len(body.Scratch) > maxScratchBytes {
		httpx.WriteError(w, r, httpx.ErrBadRequest("validation_failed", "scratch 过长", nil))
		return
	}
	m, err := a.d.Queries.UpdateMaterialScratch(r.Context(), sqlc.UpdateMaterialScratchParams{
		ID: mid, TaskID: t.ID, Scratch: body.Scratch,
	})
	writeMaterialOrNotFound(w, r, m, err)
}
