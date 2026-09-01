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

// pbl_reframe.go —— 把问题说清楚。
//
// 产品负责人 2026-09-01：「reframe it into a clear "Who needs what, and why?"
// statement and then a "How might we?" question」，而且「new observations can
// change the problem, solution, and plan at any time, with every major update
// reviewed and confirmed by the student」。
//
// 两条规则从那段话直接掉出来，都在这个文件里：
//
//  1. 🚨 改写不覆盖旧的，而是新开一版指回去（supersedes）。问题被重新框定的
//     那一刻正是这门课要教的事——覆盖掉就等于把它删了。
//  2. 🚨 确认由她做。四格没填齐不给确认：一个填了一半的问题陈述读起来像想
//     清楚了，其实没有，而后面每一步都会架在它上面。

type pblReframeDTO struct {
	ID          string  `json:"id"`
	Who         string  `json:"who"`
	Needs       string  `json:"needs"`
	Why         string  `json:"why"`
	HMW         string  `json:"hmw"`
	Supersedes  *string `json:"supersedes"`
	ConfirmedAt *string `json:"confirmedAt"`
	CreatedAt   string  `json:"createdAt"`
}

func toPblReframeDTO(r sqlc.PblReframe) pblReframeDTO {
	out := pblReframeDTO{
		ID: r.ID.String(), Who: r.Who, Needs: r.Needs, Why: r.Why, HMW: r.Hmw,
		CreatedAt: r.CreatedAt.Format(time.RFC3339),
	}
	if r.Supersedes.Valid {
		s := uuid.UUID(r.Supersedes.Bytes).String()
		out.Supersedes = &s
	}
	if r.ConfirmedAt.Valid {
		s := r.ConfirmedAt.Time.Format(time.RFC3339)
		out.ConfirmedAt = &s
	}
	return out
}

func (a *API) listPblReframes(w http.ResponseWriter, r *http.Request) {
	atomID, ok := a.loadOwnedPblProject(w, r)
	if !ok {
		return
	}
	rows, err := a.d.Queries.ListPblReframes(r.Context(), atomID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	out := make([]pblReframeDTO, 0, len(rows))
	for _, x := range rows {
		out = append(out, toPblReframeDTO(x))
	}
	httpx.WriteJSON(w, http.StatusOK, out)
}

// createPblReframe —— 开一版新的问题陈述。
//
// supersedes 指向被它替掉的那一版。留着旧的，是为了她之后回头能看见"我当时
// 以为问题是这个"。
func (a *API) createPblReframe(w http.ResponseWriter, r *http.Request) {
	atomID, ok := a.loadOwnedPblProject(w, r)
	if !ok {
		return
	}
	var req struct {
		Who        string `json:"who"`
		Needs      string `json:"needs"`
		Why        string `json:"why"`
		HMW        string `json:"hmw"`
		Supersedes string `json:"supersedes"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteError(w, r, httpx.ErrBadRequest("bad_json", "请求格式不对", nil))
		return
	}

	sup := pgtype.UUID{}
	if raw := strings.TrimSpace(req.Supersedes); raw != "" {
		old, perr := uuid.Parse(raw)
		if perr != nil {
			httpx.WriteError(w, r, httpx.ErrNotFound("上一版不存在"))
			return
		}
		prev, gerr := a.d.Queries.GetPblReframe(r.Context(), old)
		if gerr != nil || prev.AtomID != atomID {
			httpx.WriteError(w, r, httpx.ErrNotFound("上一版不存在"))
			return
		}
		sup = pgtype.UUID{Bytes: old, Valid: true}
	}

	row, err := a.d.Queries.CreatePblReframe(r.Context(), sqlc.CreatePblReframeParams{
		AtomID: atomID,
		Who:    strings.TrimSpace(req.Who),
		Needs:  strings.TrimSpace(req.Needs),
		Why:    strings.TrimSpace(req.Why),
		Hmw:    strings.TrimSpace(req.HMW),
		// pgx 用 $6，参数名由 sqlc 生成为 Supersedes。
		Supersedes: sup,
	})
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, toPblReframeDTO(row))
}

func (a *API) loadOwnedPblReframe(w http.ResponseWriter, r *http.Request) (sqlc.GetPblReframeRow, bool) {
	atomID, ok := a.loadOwnedPblProject(w, r)
	if !ok {
		return sqlc.GetPblReframeRow{}, false
	}
	rid, err := uuid.Parse(r.PathValue("rid"))
	if err != nil {
		httpx.WriteError(w, r, httpx.ErrNotFound("这一版不存在"))
		return sqlc.GetPblReframeRow{}, false
	}
	row, err := a.d.Queries.GetPblReframe(r.Context(), rid)
	if err != nil || row.AtomID != atomID {
		httpx.WriteError(w, r, httpx.ErrNotFound("这一版不存在"))
		return sqlc.GetPblReframeRow{}, false
	}
	return row, true
}

// updatePblReframe —— 确认之前随便改。确认之后要改，就是新开一版。
func (a *API) updatePblReframe(w http.ResponseWriter, r *http.Request) {
	cur, ok := a.loadOwnedPblReframe(w, r)
	if !ok {
		return
	}
	if cur.ConfirmedAt.Valid {
		httpx.WriteError(w, r, httpx.ErrBadRequest("already_confirmed",
			"这一版已经定了。要改就开新的一版，旧的留着。", nil))
		return
	}
	var req struct {
		Who   *string `json:"who"`
		Needs *string `json:"needs"`
		Why   *string `json:"why"`
		HMW   *string `json:"hmw"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteError(w, r, httpx.ErrBadRequest("bad_json", "请求格式不对", nil))
		return
	}
	who, needs, why, hmw := cur.Who, cur.Needs, cur.Why, cur.Hmw
	if req.Who != nil {
		who = strings.TrimSpace(*req.Who)
	}
	if req.Needs != nil {
		needs = strings.TrimSpace(*req.Needs)
	}
	if req.Why != nil {
		why = strings.TrimSpace(*req.Why)
	}
	if req.HMW != nil {
		hmw = strings.TrimSpace(*req.HMW)
	}
	row, err := a.d.Queries.UpdatePblReframe(r.Context(), sqlc.UpdatePblReframeParams{
		ID: cur.ID, Who: who, Needs: needs, Why: why, Hmw: hmw,
	})
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, toPblReframeDTO(row))
}

// confirmPblReframe —— 她说这就是问题。
//
// 🚨 四格必须齐。填了一半的问题陈述读起来像"想清楚了"，其实没有，而后面每
// 一步都会架在它上面。这个门槛在服务端，因为只活在按钮里的门槛是装饰。
func (a *API) confirmPblReframe(w http.ResponseWriter, r *http.Request) {
	cur, ok := a.loadOwnedPblReframe(w, r)
	if !ok {
		return
	}
	missing := []string{}
	if strings.TrimSpace(cur.Who) == "" {
		missing = append(missing, "是谁")
	}
	if strings.TrimSpace(cur.Needs) == "" {
		missing = append(missing, "需要什么")
	}
	if strings.TrimSpace(cur.Why) == "" {
		missing = append(missing, "为什么")
	}
	if strings.TrimSpace(cur.Hmw) == "" {
		missing = append(missing, "我们可以怎样")
	}
	if len(missing) > 0 {
		httpx.WriteError(w, r, httpx.ErrBadRequest("incomplete",
			"还差："+strings.Join(missing, "、"), nil))
		return
	}
	row, err := a.d.Queries.ConfirmPblReframe(r.Context(), cur.ID)
	if err != nil {
		httpx.WriteError(w, r, httpx.ErrBadRequest("already_confirmed", "这一版已经定过了", nil))
		return
	}
	httpx.WriteJSON(w, http.StatusOK, toPblReframeDTO(row))
}
