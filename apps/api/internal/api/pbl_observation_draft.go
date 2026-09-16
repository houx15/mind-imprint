package api

import (
	"encoding/json"
	"errors"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"mindimprint/api/internal/httpx"
	"mindimprint/api/internal/store/sqlc"
	"net/http"
	"strings"
	"unicode/utf8"
)

type observationRow struct {
	Kind     string `json:"kind"`
	Body     string `json:"body"`
	ImageKey string `json:"imageKey"`
	From     string `json:"from,omitempty"`
}
type observationDraftDTO struct {
	Document []observationRow `json:"document"`
	Revision int32            `json:"revision"`
}

func observationDTO(row sqlc.PblObservationDraft) (observationDraftDTO, error) {
	out := observationDraftDTO{Revision: row.Revision}
	err := json.Unmarshal(row.Document, &out.Document)
	if out.Document == nil {
		out.Document = []observationRow{}
	}
	return out, err
}
func (a *API) ownedObservationTool(w http.ResponseWriter, r *http.Request) (sqlc.GetPblToolRow, bool) {
	atom, ok := a.loadOwnedPblProject(w, r)
	if !ok {
		return sqlc.GetPblToolRow{}, false
	}
	id, err := uuid.Parse(r.PathValue("tid"))
	if err != nil {
		httpx.WriteError(w, r, httpx.ErrNotFound("观察任务不存在"))
		return sqlc.GetPblToolRow{}, false
	}
	row, err := a.d.Queries.GetPblTool(r.Context(), id)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && (row.AtomID != atom || row.Tool != "observe")) {
		httpx.WriteError(w, r, httpx.ErrNotFound("观察任务不存在"))
		return row, false
	}
	if err != nil {
		httpx.WriteError(w, r, err)
		return row, false
	}
	return row, true
}
func validateObservation(rows []observationRow, owner uuid.UUID, submit bool) error {
	if len(rows) > 100 {
		return errors.New("记录不能超过100条")
	}
	filled := 0
	for _, row := range rows {
		if !pblNoteKinds[row.Kind] || row.Kind == "idea" {
			return errors.New("记录类型无效")
		}
		if utf8.RuneCountInString(row.Body) > 10000 || utf8.RuneCountInString(row.From) > 2000 {
			return errors.New("记录超过长度限制")
		}
		if row.ImageKey != "" && !ownSiteImage(owner, row.ImageKey) {
			return errors.New("照片不属于当前账号")
		}
		if strings.TrimSpace(row.Body) != "" {
			filled++
		}
	}
	if submit && filled == 0 {
		return errors.New("请至少填写一条记录")
	}
	return nil
}
func (a *API) getPblObservationDraft(w http.ResponseWriter, r *http.Request) {
	tool, ok := a.ownedObservationTool(w, r)
	if !ok {
		return
	}
	row, err := a.d.Queries.GetPblObservationDraft(r.Context(), tool.ID)
	if errors.Is(err, pgx.ErrNoRows) {
		httpx.WriteJSON(w, 200, observationDraftDTO{Document: []observationRow{}})
		return
	}
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	out, err := observationDTO(row)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, 200, out)
}
func (a *API) savePblObservationDraft(w http.ResponseWriter, r *http.Request) {
	tool, ok := a.ownedObservationTool(w, r)
	if !ok {
		return
	}
	var req struct {
		Document []observationRow `json:"document"`
		Revision *int32           `json:"revision"`
	}
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 500000))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		httpx.WriteError(w, r, errBadJSON(err))
		return
	}
	if req.Revision == nil || *req.Revision < 0 {
		httpx.WriteError(w, r, httpx.ErrBadRequest("invalid_revision", "缺少有效草稿版本", nil))
		return
	}
	u, _ := UserFromContext(r.Context())
	if err := validateObservation(req.Document, u.ID, false); err != nil {
		httpx.WriteError(w, r, httpx.ErrBadRequest("invalid_draft", err.Error(), nil))
		return
	}
	if err := a.d.Queries.EnsurePblObservationDraft(r.Context(), tool.ID); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if req.Document == nil {
		req.Document = []observationRow{}
	}
	raw, _ := json.Marshal(req.Document)
	row, err := a.d.Queries.SavePblObservationDraft(r.Context(), sqlc.SavePblObservationDraftParams{ToolID: tool.ID, Document: raw, Revision: *req.Revision})
	if errors.Is(err, pgx.ErrNoRows) {
		httpx.WriteError(w, r, httpx.ErrConflict("观察草稿已在其他页面修改，请比较后保存"))
		return
	}
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	out, err := observationDTO(row)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, 200, out)
}
func (a *API) submitPblObservationDraft(w http.ResponseWriter, r *http.Request) {
	tool, ok := a.ownedObservationTool(w, r)
	if !ok {
		return
	}
	var req struct {
		Revision *int32 `json:"revision"`
	}
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1024))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		httpx.WriteError(w, r, errBadJSON(err))
		return
	}
	if req.Revision == nil || *req.Revision < 0 {
		httpx.WriteError(w, r, httpx.ErrBadRequest("invalid_revision", "缺少有效草稿版本", nil))
		return
	}
	tx, err := a.d.Pool.Begin(r.Context())
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	defer func() { _ = tx.Rollback(r.Context()) }()
	q := a.d.Queries.WithTx(tx)
	if _, err = q.LockAtom(r.Context(), tool.AtomID); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	row, err := q.GetPblObservationDraft(r.Context(), tool.ID)
	if errors.Is(err, pgx.ErrNoRows) {
		httpx.WriteError(w, r, httpx.ErrConflict("请先保存观察草稿"))
		return
	}
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if row.SubmittedRevision != nil && *row.SubmittedRevision == *req.Revision {
		draft, _ := observationDTO(row)
		httpx.WriteJSON(w, 200, map[string]any{"draft": draft, "notes": json.RawMessage(row.SubmittedNotes)})
		return
	}
	if row.Revision != *req.Revision {
		httpx.WriteError(w, r, httpx.ErrConflict("观察草稿已修改，请核对后提交"))
		return
	}
	draft, err := observationDTO(row)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	u, _ := UserFromContext(r.Context())
	if err = validateObservation(draft.Document, u.ID, true); err != nil {
		httpx.WriteError(w, r, httpx.ErrBadRequest("incomplete_draft", err.Error(), nil))
		return
	}
	notes := []pblNoteDTO{}
	for _, item := range draft.Document {
		if strings.TrimSpace(item.Body) == "" {
			continue
		}
		n, err := q.CreatePblNote(r.Context(), sqlc.CreatePblNoteParams{AtomID: tool.AtomID, Kind: item.Kind, Body: strings.TrimSpace(item.Body), Author: "student", ImageKey: item.ImageKey})
		if err != nil {
			httpx.WriteError(w, r, err)
			return
		}
		notes = append(notes, toPblNoteDTO(n))
	}
	raw, _ := json.Marshal(notes)
	updated, err := q.SubmitPblObservationDraft(r.Context(), sqlc.SubmitPblObservationDraftParams{ToolID: tool.ID, SubmittedNotes: raw, Revision: row.Revision})
	if errors.Is(err, pgx.ErrNoRows) {
		httpx.WriteError(w, r, httpx.ErrConflict("观察草稿已修改，请核对后提交"))
		return
	}
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if err = tx.Commit(r.Context()); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	out, _ := observationDTO(updated)
	httpx.WriteJSON(w, 201, map[string]any{"draft": out, "notes": notes})
}
