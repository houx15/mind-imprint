package api

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"mindimprint/api/internal/httpx"
	"mindimprint/api/internal/store/sqlc"
)

// pbl_tree.go —— 先看结构。
//
// 产品负责人 2026-09-01：「the most important thing is to let people know that
// there is a structure, or many people would follow the structure without
// thinking about it. so for any big output... present the structure first ->
// a mind map, an editable mindmap, let students see, modify.」
//
// 所以这棵树必须是**能改的**。一棵只能看的结构图，和没有结构图是一回事——她
// 照着写就是了，而"照着写"正是这件工具要打断的。
//
// 🚨 深度由服务端从父节点算出来，不听请求的。客户端说自己是第 0 层却挂在第
// 3 层下面，CHECK 拦得住越界，拦不住"深度和实际位置对不上"——而那之后整棵树
// 的缩进就全错了，还查不出来。

const pblTreeMaxDepth = 3

type pblNodeDTO struct {
	ID       string  `json:"id"`
	Tree     string  `json:"tree"`
	ParentID *string `json:"parentId"`
	Depth    int16   `json:"depth"`
	Ordinal  int32   `json:"ordinal"`
	Title    string  `json:"title"`
	Body     string  `json:"body"`
	Author   string  `json:"author"`
	Edited   bool    `json:"edited"`
	X        float32 `json:"x"`
	Y        float32 `json:"y"`
}

func toPblNodeDTO(n sqlc.PblTreeNode) pblNodeDTO {
	out := pblNodeDTO{
		ID: n.ID.String(), Tree: n.Tree, Depth: n.Depth, Ordinal: n.Ordinal,
		Title: n.Title, Body: n.Body, Author: n.Author, Edited: n.Edited,
		X: n.X, Y: n.Y,
	}
	if n.ParentID.Valid {
		s := uuid.UUID(n.ParentID.Bytes).String()
		out.ParentID = &s
	}
	return out
}

type pblCheckDTO struct {
	Question string `json:"question"`
	Answer   string `json:"answer"`
}

func treeName(r *http.Request) string {
	t := strings.TrimSpace(r.URL.Query().Get("tree"))
	if t == "" {
		return "main"
	}
	return t
}

func (a *API) getPblTree(w http.ResponseWriter, r *http.Request) {
	atomID, ok := a.loadOwnedPblProject(w, r)
	if !ok {
		return
	}
	tree := treeName(r)
	nodes, err := a.d.Queries.ListPblTreeNodes(r.Context(), sqlc.ListPblTreeNodesParams{
		AtomID: atomID, Tree: tree,
	})
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	checks, err := a.d.Queries.ListPblTreeChecks(r.Context(), sqlc.ListPblTreeChecksParams{
		AtomID: atomID, Tree: tree,
	})
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	outNodes := make([]pblNodeDTO, 0, len(nodes))
	for _, n := range nodes {
		outNodes = append(outNodes, toPblNodeDTO(n))
	}
	outChecks := make([]pblCheckDTO, 0, len(checks))
	for _, c := range checks {
		outChecks = append(outChecks, pblCheckDTO{Question: c.Question, Answer: c.Answer})
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{
		"tree": tree, "nodes": outNodes, "checks": outChecks,
	})
}

// resolvePblParent —— 从父节点算出深度。父节点不存在、不同项目、不同树，
// 一律当没有父节点会把结构悄悄改错，所以是 400。
func (a *API) resolvePblParent(
	r *http.Request, atomID uuid.UUID, tree, raw string,
) (pgtype.UUID, int16, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return pgtype.UUID{}, 0, nil
	}
	pid, err := uuid.Parse(raw)
	if err != nil {
		return pgtype.UUID{}, 0, httpx.ErrBadRequest("bad_parent", "找不到要挂在哪一条下面", nil)
	}
	p, err := a.d.Queries.GetPblTreeNode(r.Context(), pid)
	if err != nil || p.AtomID != atomID || p.Tree != tree {
		return pgtype.UUID{}, 0, httpx.ErrBadRequest("bad_parent", "找不到要挂在哪一条下面", nil)
	}
	depth := p.Depth + 1
	if int(depth) > pblTreeMaxDepth {
		// 不是服务器错误：结构再往下分就没人看得下去了，直说比悄悄压平好。
		return pgtype.UUID{}, 0, httpx.ErrBadRequest("too_deep",
			"分得够细了。再往下分，读的人就跟不住了。", nil)
	}
	return pgtype.UUID{Bytes: pid, Valid: true}, depth, nil
}

func (a *API) createPblTreeNode(w http.ResponseWriter, r *http.Request) {
	atomID, ok := a.loadOwnedPblProject(w, r)
	if !ok {
		return
	}
	var req struct {
		Tree     string `json:"tree"`
		ParentID string `json:"parentId"`
		Title    string `json:"title"`
		Body     string `json:"body"`
		Author   string `json:"author"`
		Ordinal  int32  `json:"ordinal"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteError(w, r, errBadJSON(err))
		return
	}
	title := strings.TrimSpace(req.Title)
	if title == "" {
		httpx.WriteError(w, r, httpx.ErrBadRequest("no_title", "这一块叫什么？", nil))
		return
	}
	tree := strings.TrimSpace(req.Tree)
	if tree == "" {
		tree = "main"
	}
	parent, depth, err := a.resolvePblParent(r, atomID, tree, req.ParentID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	row, err := a.d.Queries.CreatePblTreeNode(r.Context(), sqlc.CreatePblTreeNodeParams{
		AtomID: atomID, Tree: tree, ParentID: parent, Depth: depth,
		Ordinal: req.Ordinal, Title: title, Body: strings.TrimSpace(req.Body),
		Author: authorOf(req.Author),
	})
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, toPblNodeDTO(row))
}

func (a *API) loadOwnedPblTreeNode(w http.ResponseWriter, r *http.Request) (sqlc.GetPblTreeNodeRow, bool) {
	atomID, ok := a.loadOwnedPblProject(w, r)
	if !ok {
		return sqlc.GetPblTreeNodeRow{}, false
	}
	nid, err := uuid.Parse(r.PathValue("nid"))
	if err != nil {
		httpx.WriteError(w, r, httpx.ErrNotFound("这一条不存在"))
		return sqlc.GetPblTreeNodeRow{}, false
	}
	row, err := a.d.Queries.GetPblTreeNode(r.Context(), nid)
	if err != nil || row.AtomID != atomID {
		httpx.WriteError(w, r, httpx.ErrNotFound("这一条不存在"))
		return sqlc.GetPblTreeNodeRow{}, false
	}
	return row, true
}

func (a *API) updatePblTreeNode(w http.ResponseWriter, r *http.Request) {
	node, ok := a.loadOwnedPblTreeNode(w, r)
	if !ok {
		return
	}
	var req struct {
		Title *string  `json:"title"`
		Body  *string  `json:"body"`
		X     *float32 `json:"x"`
		Y     *float32 `json:"y"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteError(w, r, errBadJSON(err))
		return
	}
	title, body := node.Title, node.Body
	if req.Title != nil {
		title = strings.TrimSpace(*req.Title)
	}
	if req.Body != nil {
		body = strings.TrimSpace(*req.Body)
	}
	if title == "" {
		httpx.WriteError(w, r, httpx.ErrBadRequest("no_title", "这一块叫什么？", nil))
		return
	}
	row, err := a.d.Queries.UpdatePblTreeNode(r.Context(), sqlc.UpdatePblTreeNodeParams{
		ID: node.ID, Title: title, Body: body,
	})
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	// 位置单独一条 UPDATE：挪动是整理，不该被记成"她改了印记写的那一块"。
	if req.X != nil && req.Y != nil {
		moved, merr := a.d.Queries.PositionPblTreeNode(r.Context(), sqlc.PositionPblTreeNodeParams{
			ID: node.ID, X: *req.X, Y: *req.Y,
		})
		if merr != nil {
			httpx.WriteError(w, r, merr)
			return
		}
		row = moved
	}
	httpx.WriteJSON(w, http.StatusOK, toPblNodeDTO(row))
}

// movePblTreeNode —— 换个位置，或者换一层。
//
// 🚨 挂到自己的子孙下面会把这一支从树上切下来（它再也走不到根），所以要拦。
// 一棵每次只动一个节点的树，只要拦住这一种情况就不会成环。
func (a *API) movePblTreeNode(w http.ResponseWriter, r *http.Request) {
	node, ok := a.loadOwnedPblTreeNode(w, r)
	if !ok {
		return
	}
	var req struct {
		ParentID string `json:"parentId"`
		Ordinal  int32  `json:"ordinal"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteError(w, r, errBadJSON(err))
		return
	}
	if strings.TrimSpace(req.ParentID) == node.ID.String() {
		httpx.WriteError(w, r, httpx.ErrBadRequest("bad_parent", "一条不能挂在自己下面", nil))
		return
	}
	parent, depth, err := a.resolvePblParent(r, node.AtomID, node.Tree, req.ParentID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if parent.Valid {
		descendant, derr := a.pblNodeIsDescendant(r, node.Tree, node.AtomID,
			uuid.UUID(parent.Bytes), node.ID)
		if derr != nil {
			httpx.WriteError(w, r, derr)
			return
		}
		if descendant {
			httpx.WriteError(w, r, httpx.ErrBadRequest("bad_parent",
				"不能挂到自己下面的那一条里", nil))
			return
		}
	}
	row, err := a.d.Queries.MovePblTreeNode(r.Context(), sqlc.MovePblTreeNodeParams{
		ID: node.ID, ParentID: parent, Depth: depth, Ordinal: req.Ordinal,
	})
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, toPblNodeDTO(row))
}

// pblNodeIsDescendant —— candidate 是不是 root 的子孙。
//
// 顺着 parent 往上走。深度上限是 3，所以这最多走三步。
func (a *API) pblNodeIsDescendant(
	r *http.Request, tree string, atomID, candidate, root uuid.UUID,
) (bool, error) {
	cur := candidate
	for range pblTreeMaxDepth + 1 {
		if cur == root {
			return true, nil
		}
		n, err := a.d.Queries.GetPblTreeNode(r.Context(), cur)
		if err != nil || n.AtomID != atomID || n.Tree != tree || !n.ParentID.Valid {
			return false, nil
		}
		cur = uuid.UUID(n.ParentID.Bytes)
	}
	return false, nil
}

func (a *API) deletePblTreeNode(w http.ResponseWriter, r *http.Request) {
	node, ok := a.loadOwnedPblTreeNode(w, r)
	if !ok {
		return
	}
	if err := a.d.Queries.DeletePblTreeNode(r.Context(), node.ID); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// answerPblTreeCheck —— 三个问题（产品负责人原文）：盖全了吗 / 顺得下来吗 /
// 有没有更好的结构。
func (a *API) answerPblTreeCheck(w http.ResponseWriter, r *http.Request) {
	atomID, ok := a.loadOwnedPblProject(w, r)
	if !ok {
		return
	}
	var req struct {
		Tree     string `json:"tree"`
		Question string `json:"question"`
		Answer   string `json:"answer"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteError(w, r, errBadJSON(err))
		return
	}
	q := strings.TrimSpace(req.Question)
	switch q {
	case "covers", "coherent", "better":
	default:
		httpx.WriteError(w, r, httpx.ErrBadRequest("bad_question", "不认识这个问题", nil))
		return
	}
	tree := strings.TrimSpace(req.Tree)
	if tree == "" {
		tree = "main"
	}
	row, err := a.d.Queries.AnswerPblTreeCheck(r.Context(), sqlc.AnswerPblTreeCheckParams{
		AtomID: atomID, Tree: tree, Question: q, Answer: strings.TrimSpace(req.Answer),
	})
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, pblCheckDTO{Question: row.Question, Answer: row.Answer})
}
