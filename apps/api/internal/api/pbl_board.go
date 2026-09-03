package api

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"mindimprint/api/internal/httpx"
	"mindimprint/api/internal/store/sqlc"
)

// pbl_board.go —— 便签板。
//
// 产品负责人 2026-09-01：「we have sticky board where we can manage sticky
// notes—observations, quotes, assumptions, and questions—which students can
// correct, add to, and organize」。
//
// 五种便签共用一张板。前四种是她带回来的材料，第五种（办法）是「想办法」那件
// 工具用的——点子只是第五种便签，不值得再来一套表。
//
// 🚨 author 和 edited 两个字段是这张板最重要的部分，虽然界面上几乎看不见：
// 印记写的便签她留下了、她改过了、她自己写的，是三件不同的事。少了这个区分，
// 过程记录里会全是 AI 的话，而评估会把 AI 的思考算成她的。

var pblNoteKinds = map[string]bool{
	"observation": true, "quote": true, "assumption": true,
	"question": true, "idea": true,
}

type pblNoteDTO struct {
	ID        string  `json:"id"`
	Kind      string  `json:"kind"`
	Body      string  `json:"body"`
	Author    string  `json:"author"`
	Edited    bool    `json:"edited"`
	Cluster   string  `json:"cluster"`
	X         float32 `json:"x"`
	Y         float32 `json:"y"`
	// ImageKey 是这条便签带回来的那张照片在 OSS 里的 key，空 = 没有照片。
	//
	// 🚨 给的是 key，不是 URL。URL 是签出来的、会过期，塞进 DTO 就成了一条
	// 第二天必然失效的链接。前端拿 key 去换一个签好的 GET（/oss/resolve-url）。
	ImageKey  string  `json:"imageKey"`
	// 这条便签放进了结构里的哪一块。null = 还在板上，没放进去。
	// 🚨 放不进去的那几条，就是这个结构没盖到的地方——「盖全了吗」的答案。
	TreeNodeID *string `json:"treeNodeId"`
	// 她把这张纸摆进了问题陈述的哪一格：谁 / 需要什么 / 为什么。空 = 还在
	// 「我们看到的证据」那一堆里，没被判过。
	//
	// 🚨 和 Cluster 是两回事，别合并。cluster 是板上的归堆（"这几张是一回事"），
	// 这一列是问题陈述里的角色（"这条是在说谁"）。见 migration 0132。
	ReframeSlot string `json:"reframeSlot"`
	// 她挑出来先试的那条办法（只对 kind='idea' 有意义），和为什么先试它。
	//
	// 🚨 一定要往外给。这一列上一次就是「存进去了、DTO 没往外给」——界面和回灌
	// 都读不到，印记只好照着列表第一条瞎说。见 migration 0128。
	Picked   bool   `json:"picked"`
	PickWhy  string `json:"pickWhy"`
	// 她自己拖过这张纸吗。没拖过的，位置是代码排的，不是她的判断。
	Dragged  bool   `json:"dragged"`
	CreatedAt string  `json:"createdAt"`
}

func toPblNoteDTO(n sqlc.PblNote) pblNoteDTO {
	out := pblNoteDTO{
		ID: n.ID.String(), Kind: n.Kind, Body: n.Body, Author: n.Author,
		Edited: n.Edited, Cluster: n.Cluster, X: n.X, Y: n.Y,
		ImageKey:  n.ImageKey,
		// 🚨 往外给。这一处漏掉的话，界面每次打开都是一块空板——她摆过的三格
		// 全在库里躺着，谁也读不到。flip / option.author / substep 三次都是
		// 这么坏的：存进去了，DTO 没给。
		ReframeSlot: n.ReframeSlot,
		Picked:    n.PickedAt.Valid,
		PickWhy:   n.PickWhy,
		Dragged:   n.Dragged,
		CreatedAt: n.CreatedAt.Format(time.RFC3339),
	}
	if n.TreeNodeID.Valid {
		id := uuid.UUID(n.TreeNodeID.Bytes).String()
		out.TreeNodeID = &id
	}
	return out
}

// placePblNote —— 把一条便签放进结构里的某一块，或者拿回来。
//
// 🚨 「这个分法盖全了吗」一直是个没法回答的问题：她只能盯着提纲想「大概全了吧」。
// 把她自己攒的材料一条一条拖进节点里，答案就看得见了——**放不进去的那几条就是
// 没盖到的地方**。那几条是她亲手收集的，比任何自评都硬。
// pickPblIdea —— 她挑出来先试的那一条办法，以及为什么先试它。
//
// 🚨 这个端点存在，是因为「印记从来看不到 onFinish 的 payload」。界面原来把
// picked 塞进 payload 就当交差了，回灌回头读表时表里没有这一列，印记只好照着
// 列表第一条说「你那条点子说……」——说的是她没挑的那一条。
//
// 有终点信号、但做出来的东西没回到印记那儿，就不算闭环。
func (a *API) pickPblIdea(w http.ResponseWriter, r *http.Request) {
	atomID, ok := a.loadOwnedPblProject(w, r)
	if !ok {
		return
	}
	nid, err := uuid.Parse(r.PathValue("nid"))
	if err != nil {
		httpx.WriteError(w, r, httpx.ErrNotFound("这条便签不存在"))
		return
	}
	note, err := a.d.Queries.GetPblNote(r.Context(), nid)
	if err != nil || note.AtomID != atomID {
		httpx.WriteError(w, r, httpx.ErrNotFound("这条便签不存在"))
		return
	}
	if note.Kind != "idea" {
		httpx.WriteError(w, r, httpx.ErrBadRequest("not_idea", "只有办法才谈得上先试哪一条", nil))
		return
	}
	var req struct {
		Why string `json:"why"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteError(w, r, errBadJSON(err))
		return
	}
	// 挑一个先试是单选：先把这个项目里旧的松开，再挑这一条。
	if err := a.d.Queries.ClearPblIdeaPicks(r.Context(), atomID); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	row, err := a.d.Queries.PickPblIdea(r.Context(), sqlc.PickPblIdeaParams{
		ID: nid, PickWhy: strings.TrimSpace(req.Why),
	})
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, toPblNoteDTO(row))
}

func (a *API) placePblNote(w http.ResponseWriter, r *http.Request) {
	atomID, ok := a.loadOwnedPblProject(w, r)
	if !ok {
		return
	}
	nid, err := uuid.Parse(r.PathValue("nid"))
	if err != nil {
		httpx.WriteError(w, r, httpx.ErrNotFound("这条便签不存在"))
		return
	}
	note, err := a.d.Queries.GetPblNote(r.Context(), nid)
	if err != nil || note.AtomID != atomID {
		httpx.WriteError(w, r, httpx.ErrNotFound("这条便签不存在"))
		return
	}
	var req struct {
		// null / 空 = 从结构里拿回来，放回板上。
		NodeID *string `json:"nodeId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteError(w, r, errBadJSON(err))
		return
	}
	node := pgtype.UUID{}
	if req.NodeID != nil && strings.TrimSpace(*req.NodeID) != "" {
		id, perr := uuid.Parse(strings.TrimSpace(*req.NodeID))
		if perr != nil {
			httpx.WriteError(w, r, httpx.ErrNotFound("这一块不存在"))
			return
		}
		// 只能放进**这个项目自己的**结构：别的项目的节点不该在这里被引用。
		n, gerr := a.d.Queries.GetPblTreeNode(r.Context(), id)
		if gerr != nil || n.AtomID != atomID {
			httpx.WriteError(w, r, httpx.ErrNotFound("这一块不存在"))
			return
		}
		node = pgtype.UUID{Bytes: id, Valid: true}
	}
	out, err := a.d.Queries.PlacePblNote(r.Context(),
		sqlc.PlacePblNoteParams{ID: nid, TreeNodeID: node})
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, toPblNoteDTO(out))
}

// pblReframeSlots —— 问题陈述板上的三格。空串是第四个合法值：拿回证据堆。
//
// 🚨 名字用中文原词，和界面上写的一模一样。这一列会被念给印记听（回灌里
// 「她把这条判成了『需要什么』」），一个 who/needs/why 的英文枚举到那儿还得
// 翻一次，而每一次翻译都是一次可以漂移的机会。
var pblReframeSlots = map[string]bool{"谁": true, "需要什么": true, "为什么": true, "": true}

// setPblNoteReframeSlot —— 她把一张纸摆进了问题陈述的某一格，或者拿回证据堆。
//
// 🚨 单独一个端点，不塞进 PATCH /notes/{nid}。那一条通用 PATCH 会连着改
// body/kind/cluster，而摆格子**不该碰 cluster**——她在便签板上归的堆是另一句
// 判断，不能被这一下悄悄擦掉。见 migration 0132。
func (a *API) setPblNoteReframeSlot(w http.ResponseWriter, r *http.Request) {
	atomID, ok := a.loadOwnedPblProject(w, r)
	if !ok {
		return
	}
	nid, err := uuid.Parse(r.PathValue("nid"))
	if err != nil {
		httpx.WriteError(w, r, httpx.ErrNotFound("这条便签不存在"))
		return
	}
	note, err := a.d.Queries.GetPblNote(r.Context(), nid)
	if err != nil || note.AtomID != atomID {
		httpx.WriteError(w, r, httpx.ErrNotFound("这条便签不存在"))
		return
	}
	var req struct {
		Slot string `json:"slot"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteError(w, r, errBadJSON(err))
		return
	}
	slot := strings.TrimSpace(req.Slot)
	if !pblReframeSlots[slot] {
		httpx.WriteError(w, r, httpx.ErrBadRequest("invalid_slot", "这一格不存在", nil))
		return
	}
	out, err := a.d.Queries.SetPblNoteReframeSlot(r.Context(),
		sqlc.SetPblNoteReframeSlotParams{ID: nid, ReframeSlot: slot})
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, toPblNoteDTO(out))
}

func (a *API) listPblNotes(w http.ResponseWriter, r *http.Request) {
	atomID, ok := a.loadOwnedPblProject(w, r)
	if !ok {
		return
	}
	rows, err := a.d.Queries.ListPblNotes(r.Context(), atomID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	out := make([]pblNoteDTO, 0, len(rows))
	for _, n := range rows {
		out = append(out, toPblNoteDTO(n))
	}
	httpx.WriteJSON(w, http.StatusOK, out)
}

// createPblNote —— 贴一张便签。
//
// 一次可以贴一批：她出门回来往往一口气写下好几条，印记拆解一段话也是一次几条。
func (a *API) createPblNote(w http.ResponseWriter, r *http.Request) {
	atomID, ok := a.loadOwnedPblProject(w, r)
	if !ok {
		return
	}
	var req struct {
		Notes []struct {
			Kind     string `json:"kind"`
			Body     string `json:"body"`
			Author   string `json:"author"`
			Cluster  string `json:"cluster"`
			ImageKey string `json:"imageKey"`
		} `json:"notes"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteError(w, r, errBadJSON(err))
		return
	}
	if len(req.Notes) == 0 {
		httpx.WriteError(w, r, httpx.ErrBadRequest("empty", "没有要贴的便签", nil))
		return
	}
	out := make([]pblNoteDTO, 0, len(req.Notes))
	for _, n := range req.Notes {
		kind := strings.TrimSpace(n.Kind)
		body := strings.TrimSpace(n.Body)
		if !pblNoteKinds[kind] {
			httpx.WriteError(w, r, httpx.ErrBadRequest("bad_kind", "不认识这种便签", nil))
			return
		}
		if body == "" {
			httpx.WriteError(w, r, httpx.ErrBadRequest("empty_note", "空便签贴不上去", nil))
			return
		}
		author := "student"
		if strings.TrimSpace(n.Author) == "yinji" {
			author = "yinji"
		}
		row, err := a.d.Queries.CreatePblNote(r.Context(), sqlc.CreatePblNoteParams{
			AtomID: atomID, Kind: kind, Body: body, Author: author,
			Cluster:  strings.TrimSpace(n.Cluster),
			ImageKey: strings.TrimSpace(n.ImageKey),
		})
		if err != nil {
			httpx.WriteError(w, r, err)
			return
		}
		out = append(out, toPblNoteDTO(row))
	}
	httpx.WriteJSON(w, http.StatusCreated, out)
}

// loadOwnedPblNote —— 别人的便签和不存在的便签，对外长得一样。
func (a *API) loadOwnedPblNote(w http.ResponseWriter, r *http.Request) (sqlc.GetPblNoteRow, bool) {
	atomID, ok := a.loadOwnedPblProject(w, r)
	if !ok {
		return sqlc.GetPblNoteRow{}, false
	}
	nid, err := uuid.Parse(r.PathValue("nid"))
	if err != nil {
		httpx.WriteError(w, r, httpx.ErrNotFound("这张便签不存在"))
		return sqlc.GetPblNoteRow{}, false
	}
	row, err := a.d.Queries.GetPblNote(r.Context(), nid)
	if err != nil || row.AtomID != atomID {
		httpx.WriteError(w, r, httpx.ErrNotFound("这张便签不存在"))
		return sqlc.GetPblNoteRow{}, false
	}
	return row, true
}

// updatePblNote —— 改内容、改类型、归堆。
//
// 归堆（cluster）就是"把哪些放一起"这个动作本身，而这个动作正是从一堆零散
// 观察里看出线索的那一步。
func (a *API) updatePblNote(w http.ResponseWriter, r *http.Request) {
	note, ok := a.loadOwnedPblNote(w, r)
	if !ok {
		return
	}
	var req struct {
		Body    *string  `json:"body"`
		Kind    *string  `json:"kind"`
		Cluster *string  `json:"cluster"`
		X       *float32 `json:"x"`
		Y       *float32 `json:"y"`
		// 这一次挪动是她用手拖的吗。代码排座位、切换坐标视图时换算单位，都不是。
		Dragged bool     `json:"dragged"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteError(w, r, errBadJSON(err))
		return
	}
	body, kind, cluster := note.Body, note.Kind, note.Cluster
	if req.Body != nil {
		body = strings.TrimSpace(*req.Body)
	}
	if req.Kind != nil {
		kind = strings.TrimSpace(*req.Kind)
	}
	if req.Cluster != nil {
		cluster = strings.TrimSpace(*req.Cluster)
	}
	if body == "" {
		httpx.WriteError(w, r, httpx.ErrBadRequest("empty_note", "便签不能是空的", nil))
		return
	}
	if !pblNoteKinds[kind] {
		httpx.WriteError(w, r, httpx.ErrBadRequest("bad_kind", "不认识这种便签", nil))
		return
	}
	row, err := a.d.Queries.UpdatePblNote(r.Context(), sqlc.UpdatePblNoteParams{
		ID: note.ID, Body: body, Kind: kind, Cluster: cluster,
	})
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}

	// 位置单独走一条 UPDATE，不并进上面那条。
	//
	// 🚨 UpdatePblNote 带着 edited = (edited OR author='yinji')：她把印记写的
	// 便签**挪了个地方**，不该被记成"她改了它"。挪动是整理，改字才是纠正，两
	// 件事在过程记录里的分量完全不同。
	if req.X != nil && req.Y != nil {
		moved, merr := a.d.Queries.MovePblNote(r.Context(), sqlc.MovePblNoteParams{
			ID: note.ID, X: *req.X, Y: *req.Y, Dragged: req.Dragged,
		})
		if merr != nil {
			httpx.WriteError(w, r, merr)
			return
		}
		row = moved
	}
	httpx.WriteJSON(w, http.StatusOK, toPblNoteDTO(row))
}

// clusterPblNotes —— 把选中的几张归成一堆。
//
// 这是这块板上唯一真正要动脑的动作，所以它是一个端点，不是前端循环调 PATCH：
// 归堆是一次决定（"这几张是一回事"），不是三次独立的修改。
func (a *API) clusterPblNotes(w http.ResponseWriter, r *http.Request) {
	atomID, ok := a.loadOwnedPblProject(w, r)
	if !ok {
		return
	}
	var req struct {
		IDs     []string `json:"ids"`
		Cluster string   `json:"cluster"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteError(w, r, errBadJSON(err))
		return
	}
	if len(req.IDs) == 0 {
		httpx.WriteError(w, r, httpx.ErrBadRequest("empty", "没选中便签", nil))
		return
	}
	cluster := strings.TrimSpace(req.Cluster)
	out := make([]pblNoteDTO, 0, len(req.IDs))
	for _, raw := range req.IDs {
		nid, perr := uuid.Parse(strings.TrimSpace(raw))
		if perr != nil {
			httpx.WriteError(w, r, httpx.ErrNotFound("这张便签不存在"))
			return
		}
		note, gerr := a.d.Queries.GetPblNote(r.Context(), nid)
		if gerr != nil || note.AtomID != atomID {
			httpx.WriteError(w, r, httpx.ErrNotFound("这张便签不存在"))
			return
		}
		row, uerr := a.d.Queries.SetPblNoteCluster(r.Context(), sqlc.SetPblNoteClusterParams{
			ID: nid, Cluster: cluster,
		})
		if uerr != nil {
			httpx.WriteError(w, r, uerr)
			return
		}
		out = append(out, toPblNoteDTO(row))
	}
	httpx.WriteJSON(w, http.StatusOK, out)
}

// archivePblNote —— 拿下来。不是删除：她拿下过什么，也是过程的一部分。
func (a *API) archivePblNote(w http.ResponseWriter, r *http.Request) {
	note, ok := a.loadOwnedPblNote(w, r)
	if !ok {
		return
	}
	row, err := a.d.Queries.ArchivePblNote(r.Context(), note.ID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, toPblNoteDTO(row))
}
