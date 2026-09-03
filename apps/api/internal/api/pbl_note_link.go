package api

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/google/uuid"

	"mindimprint/api/internal/httpx"
	"mindimprint/api/internal/store/sqlc"
)

// pbl_note_link.go —— 两张便签之间的关系。
//
// 🚨 板上的意义不在单张纸上，在两张纸之间。「走廊上站着 14 个人」和「教室里坐
// 不住」各自都只是一条记录；把它们连起来说「这个导致那个」，才是一次判断。
//
// 四种关系里**矛盾**最要紧：两条都是她亲眼看到的，却互相打架——真正的问题几乎
// 都是从那儿长出来的。这个文件存在的首要理由，就是让「我这儿有两条对不上的
// 观察」有地方待着，而不是烂在她脑子里。

// pblNoteRelations —— 她能连出来的四种关系。
//
// 只有四种，而且都是**她判断出来的**，不是印记算出来的：因果、矛盾、同一件事、
// 支持。多了就成了一张分类表，她会开始猜「这算哪一种」而不是想两张纸的关系。
var pblNoteRelations = map[string]bool{
	"causes":      true, // 这个导致那个
	"contradicts": true, // 这两条对不上
	"same":        true, // 说的是同一件事
	"supports":    true, // 这条撑着那条
}

type pblNoteLinkDTO struct {
	ID       string `json:"id"`
	FromID   string `json:"fromId"`
	ToID     string `json:"toId"`
	Relation string `json:"relation"`
}

func toPblNoteLinkDTO(l sqlc.PblNoteLink) pblNoteLinkDTO {
	return pblNoteLinkDTO{
		ID: l.ID.String(), FromID: l.FromID.String(),
		ToID: l.ToID.String(), Relation: l.Relation,
	}
}

func (a *API) listPblNoteLinks(w http.ResponseWriter, r *http.Request) {
	atomID, ok := a.loadOwnedPblProject(w, r)
	if !ok {
		return
	}
	rows, err := a.d.Queries.ListPblNoteLinks(r.Context(), atomID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	out := make([]pblNoteLinkDTO, 0, len(rows))
	for _, l := range rows {
		out = append(out, toPblNoteLinkDTO(l))
	}
	httpx.WriteJSON(w, http.StatusOK, out)
}

// createPblNoteLink —— 她把两张纸连起来，并说清楚是哪一种关系。
func (a *API) createPblNoteLink(w http.ResponseWriter, r *http.Request) {
	atomID, ok := a.loadOwnedPblProject(w, r)
	if !ok {
		return
	}
	var req struct {
		FromID   string `json:"fromId"`
		ToID     string `json:"toId"`
		Relation string `json:"relation"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteError(w, r, errBadJSON(err))
		return
	}
	rel := strings.TrimSpace(req.Relation)
	if !pblNoteRelations[rel] {
		httpx.WriteError(w, r, httpx.ErrBadRequest("bad_relation", "不认识这种关系", nil))
		return
	}
	from, ferr := uuid.Parse(strings.TrimSpace(req.FromID))
	to, terr := uuid.Parse(strings.TrimSpace(req.ToID))
	if ferr != nil || terr != nil {
		httpx.WriteError(w, r, httpx.ErrNotFound("这两条便签不存在"))
		return
	}
	// 一张纸连到自己不是关系，是手滑。
	if from == to {
		httpx.WriteError(w, r, httpx.ErrBadRequest("same_note", "同一张便签连不到自己", nil))
		return
	}
	// 两头都必须是**这个项目**的便签：连线不该成为读别人数据的一条路。
	for _, id := range []uuid.UUID{from, to} {
		n, err := a.d.Queries.GetPblNote(r.Context(), id)
		if err != nil || n.AtomID != atomID {
			httpx.WriteError(w, r, httpx.ErrNotFound("这两条便签不存在"))
			return
		}
	}
	out, err := a.d.Queries.CreatePblNoteLink(r.Context(), sqlc.CreatePblNoteLinkParams{
		AtomID: atomID, FromID: from, ToID: to, Relation: rel,
	})
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, toPblNoteLinkDTO(out))
}

// deletePblNoteLink —— 连错了就拆掉。
func (a *API) deletePblNoteLink(w http.ResponseWriter, r *http.Request) {
	atomID, ok := a.loadOwnedPblProject(w, r)
	if !ok {
		return
	}
	lid, err := uuid.Parse(r.PathValue("lid"))
	if err != nil {
		httpx.WriteError(w, r, httpx.ErrNotFound("这条线不存在"))
		return
	}
	// 带上 atom_id 一起删：删不到就是不属于她，静默即可——别人的线和不存在的线
	// 对外长得一样。
	if err := a.d.Queries.DeletePblNoteLink(r.Context(),
		sqlc.DeletePblNoteLinkParams{ID: lid, AtomID: atomID}); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
