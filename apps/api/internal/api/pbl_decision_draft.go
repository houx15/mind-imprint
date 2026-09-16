package api

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"mindimprint/api/internal/httpx"
	"mindimprint/api/internal/store/sqlc"
)

// Draft choices are intentionally separate from the final decision fields.
type decisionDraft struct {
	ContentVersion int32             `json:"contentVersion"`
	ChoiceID       string            `json:"choiceId"`
	Why            string            `json:"why"`
	Dropped        map[string]string `json:"dropped"`
	Flip           string            `json:"flip"`
	Adding         bool              `json:"adding"`
	MineLabel      string            `json:"mineLabel"`
	MineWhy        string            `json:"mineWhy"`
}

func (a *API) getPblDecisionDraft(w http.ResponseWriter, r *http.Request) {
	id, ok := a.loadOwnedPblDecision(w, r)
	if !ok {
		return
	}
	d, err := a.d.Queries.GetPblDecision(r.Context(), id)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"draft": json.RawMessage(d.Draft), "revision": d.DraftRevision, "settled": d.SettledAt.Valid})
}

func (a *API) savePblDecisionDraft(w http.ResponseWriter, r *http.Request) {
	id, ok := a.loadOwnedPblDecision(w, r)
	if !ok {
		return
	}
	var req struct {
		Draft    *decisionDraft `json:"draft"`
		Revision *int32         `json:"revision"`
	}
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 65536))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		httpx.WriteError(w, r, errBadJSON(err))
		return
	}
	if err := dec.Decode(new(any)); err != io.EOF {
		httpx.WriteError(w, r, httpx.ErrBadRequest("invalid_draft", "草稿格式无效", nil))
		return
	}
	if req.Draft == nil || req.Revision == nil || *req.Revision < 0 {
		httpx.WriteError(w, r, httpx.ErrBadRequest("invalid_draft", "缺少草稿或版本", nil))
		return
	}
	opts, err := a.d.Queries.ListPblDecisionOptions(r.Context(), id)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	ids := make(map[string]bool, len(opts))
	for _, o := range opts {
		ids[o.ID.String()] = true
	}
	d := req.Draft
	invalid := d.ChoiceID != "" && !ids[d.ChoiceID]
	for id, reason := range d.Dropped {
		invalid = invalid || !ids[id] || utf8.RuneCountInString(reason) > 8000
	}
	for _, s := range []string{d.Why, d.Flip, d.MineWhy} {
		invalid = invalid || utf8.RuneCountInString(s) > 8000
	}
	invalid = invalid || utf8.RuneCountInString(d.MineLabel) > 300
	if invalid {
		httpx.WriteError(w, r, httpx.ErrBadRequest("invalid_draft", "草稿包含无效选项或超长文本", nil))
		return
	}
	body, err := json.Marshal(d)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	saved, err := a.d.Queries.SavePblDecisionDraft(r.Context(), sqlc.SavePblDecisionDraftParams{ID: id, Draft: body, DraftRevision: *req.Revision})
	if errors.Is(err, pgx.ErrNoRows) {
		httpx.WriteError(w, r, httpx.ErrConflict("草稿版本已变化或决定已确认，请重新打开核对；当前输入尚未保存"))
		return
	}
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"draft": json.RawMessage(saved.Draft), "revision": saved.DraftRevision, "settled": false})
}
