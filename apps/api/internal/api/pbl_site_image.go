package api

import (
	"encoding/json"
	"github.com/google/uuid"
	"mindimprint/api/internal/httpx"
	"mindimprint/api/internal/pbl"
	"mindimprint/api/internal/store/sqlc"
	"net/http"
	"path"
	"strings"
)

func ownSiteImage(owner uuid.UUID, key string) bool {
	return key != "" && path.Clean(key) == key && strings.HasPrefix(key, "users/"+owner.String()+"/images/") && !strings.ContainsAny(key, "?#\\")
}

func (a *API) putPblSiteSectionImage(w http.ResponseWriter, r *http.Request) {
	u, _ := UserFromContext(r.Context())
	var in struct {
		ObjectKey string `json:"objectKey"`
	}
	r.Body = http.MaxBytesReader(w, r.Body, 4096)
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		httpx.WriteError(w, r, errBadJSON(err))
		return
	}
	if in.ObjectKey != "" && !ownSiteImage(u.ID, in.ObjectKey) {
		httpx.WriteError(w, r, httpx.ErrBadRequest("invalid_image", "图片不属于当前账号", nil))
		return
	}
	tx, err := a.d.Pool.Begin(r.Context())
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	defer func() { _ = tx.Rollback(r.Context()) }()
	q := a.d.Queries.WithTx(tx)
	site, err := q.GetPblSite(r.Context(), u.ID)
	if err != nil || !site.AtomID.Valid {
		httpx.WriteError(w, r, httpx.ErrNotFound("主页不存在"))
		return
	}
	if _, err = q.LockAtom(r.Context(), uuid.UUID(site.AtomID.Bytes)); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	site, err = q.GetPblSite(r.Context(), u.ID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	var draft pbl.SiteDraft
	if err = json.Unmarshal(site.Content, &draft); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	found := false
	for i := range draft.Sections {
		if draft.Sections[i].Key == r.PathValue("key") {
			draft.Sections[i].ImageKey = in.ObjectKey
			draft.Sections[i].ImageURL = ""
			found = true
		}
	}
	if !found {
		httpx.WriteError(w, r, httpx.ErrNotFound("主页模块不存在"))
		return
	}
	blob, err := json.Marshal(draft)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	site, err = q.SetPblSiteContent(r.Context(), sqlc.SetPblSiteContentParams{UserID: u.ID, Content: blob})
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if err = tx.Commit(r.Context()); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	dto, err := a.siteDTO(r, u, site)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, dto)
}
