package api

// pbl_sites.go — optional inspiration links, student observations and separate text analysis.

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/google/uuid"

	"mindimprint/api/internal/gateway"
	"mindimprint/api/internal/httpx"
	"mindimprint/api/internal/pbl"
	"mindimprint/api/internal/store/sqlc"
)

type pblSiteRefDTO struct {
	ID        string `json:"id"`
	URL       string `json:"url"`
	Title     string `json:"title"`
	What      string `json:"what"`
	Structure string `json:"structure"`
	Best      string `json:"best"`
	SheSaid   string `json:"sheSaid"`
}

func toPblSiteRefDTO(r sqlc.PblSiteRef) pblSiteRefDTO {
	return pblSiteRefDTO{
		ID: r.ID.String(), URL: r.Url, Title: r.Title,
		What: r.What, Structure: r.Structure, Best: r.Best, SheSaid: r.SheSaid,
	}
}

func (a *API) listPblSiteRefs(w http.ResponseWriter, r *http.Request) {
	atomID, ok := a.loadOwnedPblProject(w, r)
	if !ok {
		return
	}
	rows, err := a.d.Queries.ListPblSiteRefs(r.Context(), atomID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	out := make([]pblSiteRefDTO, 0, len(rows))
	for _, row := range rows {
		out = append(out, toPblSiteRefDTO(row))
	}
	httpx.WriteJSON(w, http.StatusOK, out)
}

// addPblSiteRef —— 她粘一个网址进来。
//
// 🚨 读不到就报读不到，不写一张编出来的卡。一张没人读过的「结构」会让她照着一个
// 不存在的做法搭自己的页面，而她没有任何办法发现这件事。
// 见 memory · ai-errors-must-surface-never-fake。
func (a *API) addPblSiteRef(w http.ResponseWriter, r *http.Request) {
	u, _ := UserFromContext(r.Context())
	atomID, ok := a.loadOwnedPblProject(w, r)
	if !ok {
		return
	}
	var req struct {
		URL string `json:"url"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteError(w, r, errBadJSON(err))
		return
	}
	url, err := pbl.NormalizeSiteURL(req.URL)
	if err != nil {
		httpx.WriteError(w, r, httpx.ErrBadRequest("bad_url", "这不是一个网址", nil))
		return
	}

	if a.d.Fetcher == nil {
		httpx.WriteError(w, r, errors.New("pbl: no fetcher configured"))
		return
	}
	title, text, _, ferr := a.d.Fetcher.FetchReadable(r.Context(), url)
	if ferr != nil || strings.TrimSpace(text) == "" {
		// 后台原话给出去，学生和我们看到的是同一句（AGENTS.md § 界面文案 · 9）。
		detail := "这一页读不出正文"
		if ferr != nil {
			detail = ferr.Error()
		}
		httpx.WriteError(w, r, httpx.ErrBadRequest("fetch_failed", "读取失败："+detail, nil))
		return
	}

	resolved, rerr := a.routeE(r.Context(), gateway.ClassDigest) // 长输入短输出
	if rerr != nil {
		httpx.WriteError(w, r, rerr)
		return
	}
	card, usage, cerr := pbl.ReadSiteRef(r.Context(), a.d.Provider, resolved, title, text)
	a.recordLiteLLMCall(r.Context(), u.ID, atomID, "pbl_site_ref", resolved, usage)
	if errors.Is(cerr, pbl.ErrSiteRefEvidence) && r.Context().Err() == nil {
		card, usage, cerr = pbl.ReadSiteRef(r.Context(), a.d.Provider, resolved, title, text, true)
		a.recordLiteLLMCall(r.Context(), u.ID, atomID, "pbl_site_ref_retry", resolved, usage)
	}
	if cerr != nil {
		slog.Warn("pbl sites: could not read that page", "err", cerr, "url", url,
			"request_id", httpx.RequestIDFromContext(r.Context()))
		httpx.WriteError(w, r, httpx.ErrBadRequest("read_failed", "读取失败："+cerr.Error(), nil))
		return
	}

	row, err := a.d.Queries.CreatePblSiteRef(r.Context(), sqlc.CreatePblSiteRefParams{
		AtomID: atomID, Url: url, Title: card.Title,
		What: card.What, Structure: card.Structure, Best: card.Best,
	})
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, toPblSiteRefDTO(row))
}

// setPblSiteRefSaid —— 她在某一站上补的那一句。
func (a *API) setPblSiteRefSaid(w http.ResponseWriter, r *http.Request) {
	atomID, ok := a.loadOwnedPblProject(w, r)
	if !ok {
		return
	}
	id, err := uuid.Parse(strings.TrimSpace(r.PathValue("sid")))
	if err != nil {
		httpx.WriteError(w, r, httpx.ErrNotFound("这一站不存在"))
		return
	}
	var req struct {
		SheSaid string `json:"sheSaid"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteError(w, r, errBadJSON(err))
		return
	}
	row, err := a.d.Queries.SetPblSiteRefSaid(r.Context(), sqlc.SetPblSiteRefSaidParams{
		AtomID: atomID, ID: id, SheSaid: strings.TrimSpace(req.SheSaid),
	})
	if err != nil {
		httpx.WriteError(w, r, httpx.ErrNotFound("这一站不存在"))
		return
	}
	httpx.WriteJSON(w, http.StatusOK, toPblSiteRefDTO(row))
}

// deletePblSiteRef —— 她把一站拿掉。删得掉是这一关不像表单的一半原因：
// 贴错了、看完觉得不喜欢，都该能直接拿走。
func (a *API) deletePblSiteRef(w http.ResponseWriter, r *http.Request) {
	atomID, ok := a.loadOwnedPblProject(w, r)
	if !ok {
		return
	}
	id, err := uuid.Parse(strings.TrimSpace(r.PathValue("sid")))
	if err != nil {
		httpx.WriteError(w, r, httpx.ErrNotFound("这一站不存在"))
		return
	}
	if err := a.d.Queries.DeletePblSiteRef(r.Context(),
		sqlc.DeletePblSiteRefParams{AtomID: atomID, ID: id}); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// Bookmarking does not fetch the URL, call a model or assert that it was viewed.
func (a *API) bookmarkPblSiteRef(w http.ResponseWriter, r *http.Request) {
	atom, ok := a.loadOwnedPblProject(w, r)
	if !ok {
		return
	}
	var req struct {
		URL string `json:"url"`
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 10000))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&req); err != nil {
		httpx.WriteError(w, r, errBadJSON(err))
		return
	}
	url, err := pbl.NormalizeSiteURL(req.URL)
	if err != nil {
		httpx.WriteError(w, r, httpx.ErrBadRequest("bad_url", "保存失败：网址无效", nil))
		return
	}
	row, err := a.d.Queries.BookmarkPblSiteRef(r.Context(), sqlc.BookmarkPblSiteRefParams{AtomID: atom, Url: url})
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, toPblSiteRefDTO(row))
}
