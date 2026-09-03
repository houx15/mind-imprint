package api

import (
	"encoding/json"
	"net/http"
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
	ID     string `json:"id"`
	Prompt string `json:"prompt"`
	// 这一条要带回哪一类。空 = 她自己判断。
	WantKind string `json:"wantKind"`
	Ordinal  int32  `json:"ordinal"`
	// 她在现场点掉的时刻。null = 还没做到。
	DoneAt *string `json:"doneAt"`
}

func toPblMissionDTO(m sqlc.PblMissionItem) pblMissionDTO {
	out := pblMissionDTO{
		ID: m.ID.String(), Prompt: m.Prompt, WantKind: m.WantKind, Ordinal: m.Ordinal,
	}
	if m.DoneAt.Valid {
		t := m.DoneAt.Time.Format(time.RFC3339)
		out.DoneAt = &t
	}
	return out
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
	out, err := a.d.Queries.TickPblMissionItem(r.Context(),
		sqlc.TickPblMissionItemParams{ID: mid, DoneAt: at})
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, toPblMissionDTO(out))
}
