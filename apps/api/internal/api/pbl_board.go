package api

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

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
	CreatedAt string  `json:"createdAt"`
}

func toPblNoteDTO(n sqlc.PblNote) pblNoteDTO {
	return pblNoteDTO{
		ID: n.ID.String(), Kind: n.Kind, Body: n.Body, Author: n.Author,
		Edited: n.Edited, Cluster: n.Cluster, X: n.X, Y: n.Y,
		CreatedAt: n.CreatedAt.Format(time.RFC3339),
	}
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
			Kind    string `json:"kind"`
			Body    string `json:"body"`
			Author  string `json:"author"`
			Cluster string `json:"cluster"`
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
			Cluster: strings.TrimSpace(n.Cluster),
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
			ID: note.ID, X: *req.X, Y: *req.Y,
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
