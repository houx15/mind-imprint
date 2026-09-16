package api

import (
	"encoding/json"
	"errors"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"mindimprint/api/internal/gateway"
	"mindimprint/api/internal/httpx"
	"mindimprint/api/internal/pbl"
	"mindimprint/api/internal/store/sqlc"
	"net/http"
	"strings"
)

type creativeDirectionDTO struct {
	Document pbl.CreativeDirection `json:"document"`
	Revision int32                 `json:"revision"`
}

func (a *API) getPblCreativeDirection(w http.ResponseWriter, r *http.Request) {
	atom, ok := a.loadOwnedPblProject(w, r)
	if !ok {
		return
	}
	row, err := a.d.Queries.GetPblCreativeDirection(r.Context(), atom)
	if errors.Is(err, pgx.ErrNoRows) {
		httpx.WriteJSON(w, http.StatusOK, creativeDirectionDTO{Document: pbl.CreativeDirection{Stage: "feeling", Motifs: []string{}, Suggestions: []string{}}})
		return
	}
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	var doc pbl.CreativeDirection
	if err = json.Unmarshal(row.Document, &doc); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, creativeDirectionDTO{Document: doc, Revision: row.Revision})
}

func (a *API) savePblCreativeDirection(w http.ResponseWriter, r *http.Request) {
	atom, ok := a.loadOwnedPblProject(w, r)
	if !ok {
		return
	}
	var in struct {
		Document pbl.CreativeDirection `json:"document"`
		Revision *int32                `json:"revision"`
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 100000))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&in); err != nil {
		httpx.WriteError(w, r, errBadJSON(err))
		return
	}
	doc, err := pbl.NormalizeCreativeDirection(in.Document, false)
	if err != nil || in.Revision == nil || *in.Revision < 0 {
		message := "创作构思版本无效"
		if err != nil {
			message = err.Error()
		}
		httpx.WriteError(w, r, httpx.ErrBadRequest("invalid_creative_direction", "保存失败："+message, nil))
		return
	}
	if doc.Trial != nil {
		versionID, parseErr := uuid.Parse(doc.Trial.VersionID)
		if parseErr != nil {
			httpx.WriteError(w, r, httpx.ErrBadRequest("invalid_trial_version", "试用版本无效", nil))
			return
		}
		if _, loadErr := a.d.Queries.GetPblCodeVersion(r.Context(), sqlc.GetPblCodeVersionParams{AtomID: atom, ID: versionID}); loadErr != nil {
			if errors.Is(loadErr, pgx.ErrNoRows) {
				httpx.WriteError(w, r, httpx.ErrNotFound("试用版本不存在"))
				return
			}
			httpx.WriteError(w, r, loadErr)
			return
		}
	}
	tx, err := a.d.Pool.Begin(r.Context())
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	defer func() { _ = tx.Rollback(r.Context()) }()
	q := a.d.Queries.WithTx(tx)
	if err = q.EnsurePblCreativeDirection(r.Context(), atom); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	data, _ := json.Marshal(doc)
	row, err := q.SavePblCreativeDirection(r.Context(), sqlc.SavePblCreativeDirectionParams{AtomID: atom, Document: data, Revision: *in.Revision})
	if errors.Is(err, pgx.ErrNoRows) {
		httpx.WriteError(w, r, httpx.ErrConflict("创作构思已在其他窗口修改，请核对最新内容；当前输入尚未保存"))
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
	httpx.WriteJSON(w, http.StatusOK, creativeDirectionDTO{Document: doc, Revision: row.Revision})
}

func (a *API) suggestPblCreativeMotifs(w http.ResponseWriter, r *http.Request) {
	atom, ok := a.loadOwnedPblProject(w, r)
	if !ok {
		return
	}
	account, _ := UserFromContext(r.Context())
	entitled, entitlementErr := HasEntitlement(r.Context(), account)
	if entitlementErr != nil {
		httpx.WriteError(w, r, entitlementErr)
		return
	}
	if !entitled {
		httpx.WriteError(w, r, httpx.ErrNotEntitled())
		return
	}

	var in struct {
		Revision *int32 `json:"revision"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1024)).Decode(&in); err != nil {
		httpx.WriteError(w, r, errBadJSON(err))
		return
	}
	if in.Revision == nil {
		httpx.WriteError(w, r, httpx.ErrBadRequest("invalid_revision", "缺少创作构思版本", nil))
		return
	}
	row, err := a.d.Queries.GetPblCreativeDirection(r.Context(), atom)
	if errors.Is(err, pgx.ErrNoRows) {
		httpx.WriteError(w, r, httpx.ErrConflict("请先保存风格描述"))
		return
	}
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if row.Revision != *in.Revision {
		httpx.WriteError(w, r, httpx.ErrConflict("创作构思已修改，请读取最新版本"))
		return
	}
	var doc pbl.CreativeDirection
	if err = json.Unmarshal(row.Document, &doc); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if strings.TrimSpace(doc.Feeling) == "" {
		httpx.WriteError(w, r, httpx.ErrBadRequest("missing_feeling", "请先描述喜欢的感觉", nil))
		return
	}
	resolved, err := a.routeE(r.Context(), gateway.ClassCompose)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	suggestions, usage, err := pbl.GenerateCreativeMotifs(r.Context(), a.d.Provider, resolved, doc.Feeling)
	user, _ := UserFromContext(r.Context())
	a.recordLiteLLMCall(r.Context(), user.ID, atom, "pbl_creative_motifs", resolved, usage)
	if err != nil {
		httpx.WriteError(w, r, httpx.ErrBadRequest("motifs_failed", "生成失败："+err.Error(), nil))
		return
	}
	doc.Suggestions = suggestions
	data, _ := json.Marshal(doc)
	updated, err := a.d.Queries.SavePblCreativeDirection(r.Context(), sqlc.SavePblCreativeDirectionParams{AtomID: atom, Document: data, Revision: row.Revision})
	if errors.Is(err, pgx.ErrNoRows) {
		httpx.WriteError(w, r, httpx.ErrConflict("生成期间创作构思已修改，请保留当前输入并读取最新版本"))
		return
	}
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, creativeDirectionDTO{Document: doc, Revision: updated.Revision})
}

func (a *API) refinePblHeroPrompt(w http.ResponseWriter, r *http.Request) {
	atom, ok := a.loadOwnedPblProject(w, r)
	if !ok {
		return
	}
	account, _ := UserFromContext(r.Context())
	entitled, entitlementErr := HasEntitlement(r.Context(), account)
	if entitlementErr != nil {
		httpx.WriteError(w, r, entitlementErr)
		return
	}
	if !entitled {
		httpx.WriteError(w, r, httpx.ErrNotEntitled())
		return
	}

	var in struct {
		Revision *int32 `json:"revision"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1024)).Decode(&in); err != nil {
		httpx.WriteError(w, r, errBadJSON(err))
		return
	}
	if in.Revision == nil {
		httpx.WriteError(w, r, httpx.ErrBadRequest("invalid_revision", "缺少创作构思版本", nil))
		return
	}
	row, err := a.d.Queries.GetPblCreativeDirection(r.Context(), atom)
	if errors.Is(err, pgx.ErrNoRows) {
		httpx.WriteError(w, r, httpx.ErrConflict("请先保存风格描述"))
		return
	}
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if row.Revision != *in.Revision {
		httpx.WriteError(w, r, httpx.ErrConflict("创作构思已修改，请读取最新版本"))
		return
	}
	var doc pbl.CreativeDirection
	if err = json.Unmarshal(row.Document, &doc); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if strings.TrimSpace(doc.Feeling) == "" {
		httpx.WriteError(w, r, httpx.ErrBadRequest("missing_feeling", "请先描述喜欢的感觉", nil))
		return
	}
	if doc.Hero == nil || doc.Hero.Mode == "" || strings.TrimSpace(doc.Hero.Scene) == "" {
		httpx.WriteError(w, r, httpx.ErrBadRequest("missing_hero", "请先描述第一幕画面并选择呈现方式", nil))
		return
	}
	resolved, err := a.routeE(r.Context(), gateway.ClassCompose)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	prompt, usage, err := pbl.GenerateHeroPrompt(r.Context(), a.d.Provider, resolved, doc)
	user, _ := UserFromContext(r.Context())
	a.recordLiteLLMCall(r.Context(), user.ID, atom, "pbl_hero_prompt", resolved, usage)
	if err != nil {
		httpx.WriteError(w, r, httpx.ErrBadRequest("motifs_failed", "生成失败："+err.Error(), nil))
		return
	}
	doc.Hero.Prompt = prompt
	data, _ := json.Marshal(doc)
	updated, err := a.d.Queries.SavePblCreativeDirection(r.Context(), sqlc.SavePblCreativeDirectionParams{AtomID: atom, Document: data, Revision: row.Revision})
	if errors.Is(err, pgx.ErrNoRows) {
		httpx.WriteError(w, r, httpx.ErrConflict("生成期间创作构思已修改，请保留当前输入并读取最新版本"))
		return
	}
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, creativeDirectionDTO{Document: doc, Revision: updated.Revision})
}
