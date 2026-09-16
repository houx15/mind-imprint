package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"mindimprint/api/internal/httpx"
	"mindimprint/api/internal/pbl"
	"mindimprint/api/internal/store/sqlc"
)

func (a *API) publishPblCodeSite(w http.ResponseWriter, r *http.Request, value string) {
	user, _ := UserFromContext(r.Context())
	id, err := uuid.Parse(value)
	if err != nil {
		httpx.WriteError(w, r, httpx.ErrBadRequest("invalid_version", "主页版本无效", nil))
		return
	}
	site, err := a.d.Queries.GetPblSite(r.Context(), user.ID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if !site.AtomID.Valid {
		httpx.WriteError(w, r, httpx.ErrNotFound("主页项目不存在"))
		return
	}
	atom := uuid.UUID(site.AtomID.Bytes)
	version, err := a.d.Queries.GetPblCodeVersion(r.Context(), sqlc.GetPblCodeVersionParams{AtomID: atom, ID: id})
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	page, missing := publicationPage(version.Brief)
	if missing != "" {
		httpx.WriteError(w, r, httpx.ErrConflict(missing))
		return
	}
	if err = pbl.ValidatePageCopy(version.Html, *page); err != nil {
		httpx.WriteError(w, r, httpx.ErrConflict("版本内容不完整，请重新生成并检查"))
		return
	}
	tx, err := a.d.Pool.Begin(r.Context())
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	defer tx.Rollback(r.Context())
	// Same lock order as generation: creative direction before site.
	var document []byte
	if err = tx.QueryRow(r.Context(), "SELECT document FROM pbl_creative_direction WHERE atom_id=$1 FOR UPDATE", atom).Scan(&document); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	var direction pbl.CreativeDirection
	if err = json.Unmarshal(document, &direction); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if direction.Trial == nil || direction.Trial.VersionID != id.String() || strings.TrimSpace(direction.Trial.Observation) == "" {
		httpx.WriteError(w, r, httpx.ErrConflict("请先试用并保留要发布的完整主页版本"))
		return
	}
	var token *string
	if err = tx.QueryRow(r.Context(), "SELECT share_token FROM pbl_site WHERE user_id=$1 AND atom_id=$2 FOR UPDATE", user.ID, atom).Scan(&token); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if token == nil || *token == "" {
		fresh, e := newShareToken()
		if e != nil {
			httpx.WriteError(w, r, e)
			return
		}
		token = &fresh
	}
	q := a.d.Queries.WithTx(tx)
	if err = q.SetPblSitePublication(r.Context(), sqlc.SetPblSitePublicationParams{UserID: user.ID, AtomID: atom, VersionID: id}); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if _, err = q.SetPblSiteShare(r.Context(), sqlc.SetPblSiteShareParams{UserID: user.ID, ShareToken: token, PublishedAt: pgtype.Timestamptz{Time: time.Now(), Valid: true}}); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if err = tx.Commit(r.Context()); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, 200, map[string]any{"published": true, "url": publicSiteURL(r, a.d.CORSOrigins, *token)})
}

func (a *API) renderPublicCodeSite(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Robots-Tag", "noindex, nofollow, noarchive")
	token := r.PathValue("token")
	site, err := a.d.Queries.GetPblSiteByShareToken(r.Context(), &token)
	if err != nil {
		httpx.WriteError(w, r, httpx.ErrNotFound("资源不存在"))
		return
	}
	publication, err := a.d.Queries.GetPblSitePublication(r.Context(), site.UserID)
	if errors.Is(err, pgx.ErrNoRows) {
		httpx.WriteError(w, r, httpx.ErrNotFound("资源不存在"))
		return
	}
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	version, err := a.d.Queries.GetPblCodeVersion(r.Context(), sqlc.GetPblCodeVersionParams{AtomID: publication.AtomID, ID: publication.VersionID})
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if expected := r.URL.Query().Get("publication"); expected != "" && expected != publicationRenderKey(publication.VersionID) {
		httpx.WriteError(w, r, httpx.ErrNotFound("资源不存在"))
		return
	}
	version, err = a.comparisonVersion(r, version)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	a.writePblCodeVersion(w, r, version, site.UserID)
}

func publicationPage(brief []byte) (*pbl.SiteContent, string) {
	var snapshot struct {
		Page *pbl.SiteContent `json:"pageContent"`
	}
	if json.Unmarshal(brief, &snapshot) != nil {
		return nil, "版本内容读取失败"
	}
	if snapshot.Page != nil {
		for _, text := range snapshot.Page.About {
			if strings.TrimSpace(text) != "" {
				return snapshot.Page, ""
			}
		}
		for _, section := range snapshot.Page.Sections {
			// A student's first project can be this site's own documented process.
			// It still needs to be included in the generated HTML and reviewed.
			if strings.TrimSpace(section.Body) != "" {
				return snapshot.Page, ""
			}
		}
	}
	return nil, "请先将自我介绍或作品内容加入这一版，再试用完整主页"
}
