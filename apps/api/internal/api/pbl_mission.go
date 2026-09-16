package api

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"mindimprint/api/internal/httpx"
	"mindimprint/api/internal/store/sqlc"
)

// pbl_mission.go —— 出门前的观察清单。
//
// 🚨 「观察日记」一直没有 before-state：她带着一段话出门，回来面对几个空白框。
// 产品负责人 2026-09-02 说这件工具 boring，根子在这儿。
//
// docs/2026-09-01-pbl-detail.md 要的是「a small real-world mission」加「a simple
// observation method」。方法就落在这几条上：把「去看看」拆成她在现场十分钟内做
// 得到的几件事，一条一条点掉。回来时那几条已经是填好类别的便签底稿。
//
// 没点掉的那几条同样要紧。「第 2 条（听一句原话）没做到」是铁律④要的信号，
// 而在这张表之前，做没做到在系统里毫无区别。

// pblWantKinds 对齐便签的类别——她点掉一条回来，便签的类别就是从这儿来的。
var pblWantKinds = map[string]bool{
	"observation": true, "quote": true, "assumption": true, "question": true,
}

// pblWantKind 把模型给的值收进允许的那几个里。
//
// 认不出来就留空，让她自己判类别：一个乱写的类别会让回来那一屏预填错的东西，
// 比不预填更糟。
func pblWantKind(s string) string {
	if pblWantKinds[s] {
		return s
	}
	return ""
}

type pblMissionDTO struct {
	Version         string `json:"version"`
	EditedByStudent bool   `json:"editedByStudent"`
	ID              string `json:"id"`
	Prompt          string `json:"prompt"`
	// 这一条要带回哪一类。空 = 她自己判断。
	WantKind string `json:"wantKind"`
	Ordinal  int32  `json:"ordinal"`
	// 她在现场点掉的时刻。null = 还没做到。
	DoneAt       *string `json:"doneAt"`
	SupersededAt *string `json:"supersededAt"`
}

func toPblMissionDTO(m sqlc.PblMissionItem) pblMissionDTO {
	raw, _ := json.Marshal(m)
	out := pblMissionDTO{
		Version: fmt.Sprintf("%x", sha256.Sum256(raw)), EditedByStudent: m.EditedByStudent,
		ID: m.ID.String(), Prompt: m.Prompt, WantKind: m.WantKind, Ordinal: m.Ordinal,
	}
	if m.DoneAt.Valid {
		t := m.DoneAt.Time.Format(time.RFC3339)
		out.DoneAt = &t
	}
	if m.SupersededAt.Valid {
		t := m.SupersededAt.Time.Format(time.RFC3339)
		out.SupersededAt = &t
	}
	return out
}

// A student's edit creates one new pending task and preserves the old record.
// The tool lock is shared with ticking and AI revisions.
func (a *API) editPblMissionItem(w http.ResponseWriter, r *http.Request) {
	atomID, ok := a.loadOwnedPblProject(w, r)
	if !ok {
		return
	}
	mid, err := uuid.Parse(r.PathValue("mid"))
	if err != nil {
		httpx.WriteError(w, r, httpx.ErrNotFound("这一条不存在"))
		return
	}
	var req struct {
		Prompt  string `json:"prompt"`
		Version string `json:"version"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteError(w, r, errBadJSON(err))
		return
	}
	req.Prompt = strings.TrimSpace(req.Prompt)
	if req.Prompt == "" || len([]rune(req.Prompt)) > 2000 || req.Version == "" {
		httpx.WriteError(w, r, httpx.ErrBadRequest("invalid_mission_edit", "请输入1至2000字的任务内容，并提供当前版本", nil))
		return
	}
	row, err := a.d.Queries.GetPblMissionItem(r.Context(), mid)
	if err != nil {
		httpx.WriteError(w, r, httpx.ErrNotFound("这一条不存在"))
		return
	}
	tx, err := a.d.Pool.Begin(r.Context())
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	defer func() { _ = tx.Rollback(r.Context()) }()
	q := a.d.Queries.WithTx(tx)
	tool, err := q.LockPblMissionTool(r.Context(), row.ToolID)
	if err != nil || tool.AtomID != atomID || tool.Tool != "observe" {
		httpx.WriteError(w, r, httpx.ErrNotFound("这一条不存在"))
		return
	}
	if tool.Status != "summoned" && tool.Status != "accepted" {
		httpx.WriteError(w, r, httpx.ErrConflict("观察任务已结束，不能修改"))
		return
	}
	rows, err := q.ListPblMissionItems(r.Context(), tool.ID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	for _, current := range rows {
		if current.ID != mid {
			continue
		}
		if current.SupersededAt.Valid || toPblMissionDTO(current).Version != req.Version {
			httpx.WriteError(w, r, httpx.ErrConflict("此任务已发生变化，请查看最新清单后重新修改"))
			return
		}
		if current.Prompt == req.Prompt {
			httpx.WriteJSON(w, http.StatusOK, missionDTOs(rows))
			return
		}
		if err := q.SupersedePblMissionItem(r.Context(), mid); err != nil {
			httpx.WriteError(w, r, err)
			return
		}
		_, err = q.CreateStudentPblMissionItem(r.Context(), sqlc.CreateStudentPblMissionItemParams{ToolID: tool.ID, Prompt: req.Prompt, WantKind: current.WantKind, Ordinal: current.Ordinal})
		if err != nil {
			httpx.WriteError(w, r, err)
			return
		}
		updated, err := q.ListPblMissionItems(r.Context(), tool.ID)
		if err != nil {
			httpx.WriteError(w, r, err)
			return
		}
		if err := tx.Commit(r.Context()); err != nil {
			httpx.WriteError(w, r, err)
			return
		}
		httpx.WriteJSON(w, http.StatusOK, missionDTOs(updated))
		return
	}
	httpx.WriteError(w, r, httpx.ErrNotFound("这一条不存在"))
}

// listPblMission —— 这一趟要看的几件事。
func (a *API) listPblMission(w http.ResponseWriter, r *http.Request) {
	atomID, ok := a.loadOwnedPblProject(w, r)
	if !ok {
		return
	}
	tid, err := uuid.Parse(r.PathValue("tid"))
	if err != nil {
		httpx.WriteError(w, r, httpx.ErrNotFound("这件工具不存在"))
		return
	}
	tool, err := a.d.Queries.GetPblTool(r.Context(), tid)
	if err != nil || tool.AtomID != atomID {
		httpx.WriteError(w, r, httpx.ErrNotFound("这件工具不存在"))
		return
	}
	rows, err := a.d.Queries.ListPblMissionItems(r.Context(), tid)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	out := make([]pblMissionDTO, 0, len(rows))
	for _, m := range rows {
		out = append(out, toPblMissionDTO(m))
	}
	httpx.WriteJSON(w, http.StatusOK, out)
}

// tickPblMissionItem —— 她在现场点掉一条。
//
// 🚨 再点一下是取消。现场点错了不该没法反悔，所以这是个开关，不是一条单向的路。
func (a *API) tickPblMissionItem(w http.ResponseWriter, r *http.Request) {
	u, _ := UserFromContext(r.Context())
	mid, err := uuid.Parse(r.PathValue("mid"))
	if err != nil {
		httpx.WriteError(w, r, httpx.ErrNotFound("这一条不存在"))
		return
	}
	row, err := a.d.Queries.GetPblMissionItem(r.Context(), mid)
	// 别人的清单和不存在的清单，对外长得一样。
	if err != nil || row.UserID != u.ID {
		httpx.WriteError(w, r, httpx.ErrNotFound("这一条不存在"))
		return
	}
	var req struct {
		Done bool `json:"done"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteError(w, r, errBadJSON(err))
		return
	}
	at := pgtype.Timestamptz{}
	if req.Done {
		at = pgtype.Timestamptz{Time: time.Now(), Valid: true}
	}
	tx, err := a.d.Pool.Begin(r.Context())
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	defer func() { _ = tx.Rollback(r.Context()) }()
	q := a.d.Queries.WithTx(tx)
	if _, err = q.LockPblMissionTool(r.Context(), row.ToolID); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	current, err := q.GetPblMissionItem(r.Context(), mid)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if current.SupersededAt.Valid {
		httpx.WriteError(w, r, httpx.ErrConflict("此任务已修订，请刷新清单"))
		return
	}
	out, err := q.TickPblMissionItem(r.Context(),
		sqlc.TickPblMissionItemParams{ID: mid, DoneAt: at})
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if err = tx.Commit(r.Context()); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, toPblMissionDTO(out))
}

func missionDTOs(rows []sqlc.PblMissionItem) []pblMissionDTO {
	out := make([]pblMissionDTO, 0, len(rows))
	for _, row := range rows {
		out = append(out, toPblMissionDTO(row))
	}
	return out
}
