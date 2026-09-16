package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/jackc/pgx/v5"
	"mindimprint/api/internal/gateway"
	"mindimprint/api/internal/httpx"
	"mindimprint/api/internal/pbl"
	"mindimprint/api/internal/store/sqlc"
)

type audienceDocumentDTO struct {
	Document pbl.AudienceDocument `json:"document"`
	Revision int32                `json:"revision"`
}

func (a *API) summarizePblAudienceDocument(w http.ResponseWriter, r *http.Request) {
	atom, ok := a.loadOwnedPblProject(w, r)
	if !ok {
		return
	}
	u, _ := UserFromContext(r.Context())
	var in struct {
		Revision *int32 `json:"revision"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1024)).Decode(&in); err != nil {
		httpx.WriteError(w, r, errBadJSON(err))
		return
	}
	if in.Revision == nil {
		httpx.WriteError(w, r, httpx.ErrBadRequest("invalid_revision", "生成失败：缺少人物板版本", nil))
		return
	}
	row, err := a.d.Queries.GetPblAudienceDocument(r.Context(), atom)
	if errors.Is(err, pgx.ErrNoRows) {
		httpx.WriteError(w, r, httpx.ErrConflict("请先完成人物板"))
		return
	}
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if row.Revision != *in.Revision {
		httpx.WriteError(w, r, httpx.ErrConflict("人物板已修改，请核对最新内容"))
		return
	}
	var doc pbl.AudienceDocument
	if err = json.Unmarshal(row.Document, &doc); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	doc, err = pbl.NormalizeAudienceDocument(doc, true)
	if err != nil {
		httpx.WriteError(w, r, httpx.ErrBadRequest("incomplete_audience", err.Error(), nil))
		return
	}
	resolved, err := a.routeE(r.Context(), gateway.ClassCompose)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	summary, usage, err := pbl.GenerateAudienceSummary(r.Context(), a.d.Provider, resolved, doc)
	a.recordLiteLLMCall(r.Context(), u.ID, atom, "pbl_audience_summary", resolved, usage)
	if err != nil {
		httpx.WriteError(w, r, httpx.ErrBadRequest("summary_failed", "生成失败："+err.Error(), nil))
		return
	}
	latest, err := a.d.Queries.GetPblAudienceDocument(r.Context(), atom)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if latest.Revision != row.Revision {
		httpx.WriteError(w, r, httpx.ErrConflict("生成人物板总结期间内容已修改，请重新生成"))
		return
	}
	httpx.WriteJSON(w, http.StatusOK, struct {
		Revision int32               `json:"revision"`
		Summary  pbl.AudienceSummary `json:"summary"`
	}{row.Revision, summary})
}

// Confirmation publishes every board atomically. Draft edits remain private
// until a later confirmation; retries with the old revision cannot duplicate rows.
func (a *API) confirmPblAudienceDocument(w http.ResponseWriter, r *http.Request) {
	atom, ok := a.loadOwnedPblProject(w, r)
	if !ok {
		return
	}
	var in struct {
		Revision *int32              `json:"revision"`
		Keywords map[string][]string `json:"keywords"`
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 100000))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&in); err != nil {
		httpx.WriteError(w, r, errBadJSON(err))
		return
	}
	if in.Revision == nil || *in.Revision < 0 {
		httpx.WriteError(w, r, httpx.ErrBadRequest("invalid_revision", "确认失败：缺少有效版本", nil))
		return
	}
	tx, err := a.d.Pool.Begin(r.Context())
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	defer func() { _ = tx.Rollback(r.Context()) }()
	q := a.d.Queries.WithTx(tx)
	if _, err = q.LockAtom(r.Context(), atom); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	row, err := q.GetPblAudienceDocument(r.Context(), atom)
	if errors.Is(err, pgx.ErrNoRows) {
		httpx.WriteError(w, r, httpx.ErrConflict("请先保存人物板"))
		return
	}
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if row.Revision != *in.Revision {
		httpx.WriteError(w, r, httpx.ErrConflict("人物板已修改，请核对后重新确认"))
		return
	}
	var doc pbl.AudienceDocument
	if err = json.Unmarshal(row.Document, &doc); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	doc, err = pbl.NormalizeAudienceDocument(doc, true)
	if err != nil {
		httpx.WriteError(w, r, httpx.ErrBadRequest("incomplete_audience", err.Error(), nil))
		return
	}
	if len(in.Keywords) != len(doc.Boards) {
		httpx.WriteError(w, r, httpx.ErrBadRequest("invalid_keywords", "请确认每个人物板的内容关键词", nil))
		return
	}
	for _, board := range doc.Boards {
		words := in.Keywords[board.ID]
		if len(words) < 1 || len(words) > 6 {
			httpx.WriteError(w, r, httpx.ErrBadRequest("invalid_keywords", "每个人物板需要1至6个内容关键词", nil))
			return
		}
		seen := map[string]bool{}
		for i, word := range words {
			word = strings.TrimSpace(word)
			if word == "" || len([]rune(word)) > 40 || seen[word] {
				httpx.WriteError(w, r, httpx.ErrBadRequest("invalid_keywords", "关键词为空、重复或超过40字", nil))
				return
			}
			seen[word] = true
			words[i] = word
		}
	}
	// CAS also rejects a save racing with this confirmation.
	row, err = q.SavePblAudienceDocument(r.Context(), sqlc.SavePblAudienceDocumentParams{AtomID: atom, Document: row.Document, Revision: *in.Revision})
	if errors.Is(err, pgx.ErrNoRows) {
		httpx.WriteError(w, r, httpx.ErrConflict("人物板已修改，请核对后重新确认"))
		return
	}
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if err = q.ClearPblPersonaChosen(r.Context(), atom); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	out := make([]pblPersonaDTO, 0, len(doc.Boards))
	for _, board := range doc.Boards {
		words, _ := json.Marshal(in.Keywords[board.ID])
		p, err := q.CreatePblPersona(r.Context(), sqlc.CreatePblPersonaParams{AtomID: atom, Label: board.Role + " · " + board.Person, WhyKnows: "年龄：" + board.AgeRange + "；日常兴趣爱好（学生填写）：" + strings.Join(board.Hobbies, "、"), Wants: "学生判断对方关注的主页内容或呈现方式：" + strings.Join(board.Interests, "、") + "；学生准备展示：" + strings.Join(board.Offerings, "、"), Keywords: words})
		if err != nil {
			httpx.WriteError(w, r, err)
			return
		}
		p, err = q.ChoosePblPersona(r.Context(), sqlc.ChoosePblPersonaParams{AtomID: atom, ID: p.ID})
		if err != nil {
			httpx.WriteError(w, r, err)
			return
		}
		out = append(out, a.toPblPersonaDTO(p))
	}
	if err = tx.Commit(r.Context()); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, struct {
		Revision int32           `json:"revision"`
		Personas []pblPersonaDTO `json:"personas"`
	}{row.Revision, out})
}

func (a *API) getPblAudienceDocument(w http.ResponseWriter, r *http.Request) {
	atom, ok := a.loadOwnedPblProject(w, r)
	if !ok {
		return
	}
	row, err := a.d.Queries.GetPblAudienceDocument(r.Context(), atom)
	if errors.Is(err, pgx.ErrNoRows) {
		httpx.WriteJSON(w, http.StatusOK, audienceDocumentDTO{Document: pbl.AudienceDocument{Boards: []pbl.AudienceBoard{}, Step: "roles"}})
		return
	}
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	var doc pbl.AudienceDocument
	if err = json.Unmarshal(row.Document, &doc); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, audienceDocumentDTO{Document: doc, Revision: row.Revision})
}

func (a *API) savePblAudienceDocument(w http.ResponseWriter, r *http.Request) {
	atom, ok := a.loadOwnedPblProject(w, r)
	if !ok {
		return
	}
	var in struct {
		Document pbl.AudienceDocument `json:"document"`
		Revision *int32               `json:"revision"`
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 100000))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&in); err != nil {
		httpx.WriteError(w, r, errBadJSON(err))
		return
	}
	doc, err := pbl.NormalizeAudienceDocument(in.Document, false)
	if err != nil || in.Revision == nil || *in.Revision < 0 {
		message := "人物板版本无效"
		if err != nil {
			message = err.Error()
		}
		httpx.WriteError(w, r, httpx.ErrBadRequest("invalid_audience_document", "保存失败："+message, nil))
		return
	}
	tx, err := a.d.Pool.Begin(r.Context())
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	defer func() { _ = tx.Rollback(r.Context()) }()
	q := a.d.Queries.WithTx(tx)
	if err = q.EnsurePblAudienceDocument(r.Context(), atom); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	data, _ := json.Marshal(doc)
	row, err := q.SavePblAudienceDocument(r.Context(), sqlc.SavePblAudienceDocumentParams{AtomID: atom, Document: data, Revision: *in.Revision})
	if errors.Is(err, pgx.ErrNoRows) {
		httpx.WriteError(w, r, httpx.ErrConflict("人物板已在其他窗口修改，请核对最新内容；当前输入尚未保存"))
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
	httpx.WriteJSON(w, http.StatusOK, audienceDocumentDTO{Document: doc, Revision: row.Revision})
}
