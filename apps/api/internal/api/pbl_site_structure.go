package api

import (
	"encoding/json"
	"net/http"

	"github.com/google/uuid"
	"mindimprint/api/internal/httpx"
	"mindimprint/api/internal/pbl"
	"mindimprint/api/internal/store/sqlc"
)

// Apply a student-confirmed outline. Never copy planning instructions or private
// notes into public text. Keep existing text only for surviving node identities.
func (a *API) applyPblSiteStructure(w http.ResponseWriter, r *http.Request) {
	atomID, ok := a.loadOwnedPblProject(w, r)
	if !ok {
		return
	}
	u, _ := UserFromContext(r.Context())
	tx, err := a.d.Pool.Begin(r.Context())
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	defer func() { _ = tx.Rollback(r.Context()) }()
	q := a.d.Queries.WithTx(tx)
	if _, err = q.LockAtom(r.Context(), atomID); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	site, err := q.GetPblSite(r.Context(), u.ID)
	if err != nil || !site.AtomID.Valid || uuid.UUID(site.AtomID.Bytes) != atomID {
		httpx.WriteError(w, r, httpx.ErrBadRequest("not_homepage", "应用失败：这不是当前主页项目", nil))
		return
	}
	nodes, err := q.ListPblTreeNodes(r.Context(), sqlc.ListPblTreeNodesParams{AtomID: atomID, Tree: "main"})
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if len(nodes) == 0 || len(nodes) > 60 {
		httpx.WriteError(w, r, httpx.ErrBadRequest("invalid_outline", "应用失败：主页结构需要1至60个节点", nil))
		return
	}
	var draft pbl.SiteDraft
	if err = json.Unmarshal(site.Content, &draft); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	old := map[string]pbl.SiteSection{}
	for _, section := range draft.Sections {
		old[section.Key] = section
	}
	children := map[uuid.UUID][]sqlc.PblTreeNode{}
	for _, node := range nodes {
		parent := uuid.Nil
		if node.ParentID.Valid {
			parent = uuid.UUID(node.ParentID.Bytes)
		}
		children[parent] = append(children[parent], node)
	}
	sections := []pbl.SiteSection{}
	seen := map[uuid.UUID]bool{}
	var walk func(uuid.UUID, int)
	walk = func(parent uuid.UUID, depth int) {
		for _, node := range children[parent] {
			if seen[node.ID] {
				continue
			}
			seen[node.ID] = true
			key := pbl.SiteItemID(u.ID.String(), node.ID.String())
			sections = append(sections, pbl.SiteSection{Key: key, Title: node.Title, Depth: depth, Body: old[key].Body, ImageKey: old[key].ImageKey})
			walk(node.ID, depth+1)
		}
	}
	walk(uuid.Nil, 0)
	if len(sections) != len(nodes) {
		httpx.WriteError(w, r, httpx.ErrBadRequest("invalid_outline", "应用失败：结构存在未连接的节点", nil))
		return
	}
	draft.Sections = sections
	blob, err := json.Marshal(clampDraft(draft))
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
