package api

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"mindimprint/api/internal/httpx"
	"mindimprint/api/internal/store/sqlc"
)

// Only authored page state changes the review version; publication does not.
func siteReviewRevision(site sqlc.PblSite) string {
	var content, palette any
	_ = json.Unmarshal(site.Content, &content)
	_ = json.Unmarshal(site.Palette, &palette)
	raw, _ := json.Marshal([]any{content, palette, site.Layout, site.HeroKey})
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

func (a *API) siteReviewStale(r *http.Request, kind string, atomID uuid.UUID, rawPayload []byte) (bool, error) {
	if kind != "site" {
		return false, nil
	}
	u, _ := UserFromContext(r.Context())
	site, err := a.d.Queries.GetPblSite(r.Context(), u.ID)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return true, err
	}
	if !site.AtomID.Valid || uuid.UUID(site.AtomID.Bytes) != atomID {
		return false, nil
	}
	var payload struct {
		SiteRevision string `json:"siteRevision"`
	}
	if err := json.Unmarshal(rawPayload, &payload); err != nil {
		return true, err
	}
	return payload.SiteRevision == "" || payload.SiteRevision != siteReviewRevision(site), nil
}

// Refresh creates a new snapshot. Earlier review answers remain with their
// original artifact and never count as answers to the changed page.
func (a *API) refreshPblSiteReview(w http.ResponseWriter, r *http.Request) {
	atomID, _, ok := a.loadOwnedPblArtifact(w, r)
	if !ok {
		return
	}
	aid, _ := uuid.Parse(r.PathValue("aid"))
	old, err := a.d.Queries.GetPblArtifact(r.Context(), aid)
	if err != nil || old.Kind != "site" {
		httpx.WriteError(w, r, httpx.ErrBadRequest("not_site_review", "仅主页审核可以刷新", nil))
		return
	}
	raw := json.RawMessage(`{"kind":"site","title":"当前主页审核","dimensions":[{"prompt":"页面内容是否准确，是否保留了事实、推测与虚构示例的区别？","why":"请检查当前页面实际展示的文字。"},{"prompt":"目标读者能否找到需要的信息，知道下一步可以做什么？","why":"请结合已确定的受众和目标检查内容与顺序。"},{"prompt":"宽屏与手机预览中，文字、图片和层级是否清晰？","why":"请打开当前主页预览，检查实际阅读效果。"}]}`)
	if err := a.produceArtifact(r.Context(), atomID, pgtype.UUID{}, raw); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	a.listPblArtifacts(w, r)
}
