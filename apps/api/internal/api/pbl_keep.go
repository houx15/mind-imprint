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

// pbl_keep.go —— 上线之后。
//
// 产品负责人 2026-09-01：「sometimes students have shipped their website or put
// their results in some real situations. how to track and use data to iterate?
// ... when students give some statistics, feedbacks, new thoughts, we can add a
// new session for this project. (so one project may have several sessions)」
//
// 这一步是这个产品和"交作业"最不一样的地方：东西交出去之后还有事情发生，而
// 那些事情才是真的。所以一条数据进来可以就地开一轮新的思考——不是记一笔流水
// 账，是让这个项目重新活一次。
//
// 我们不替她保管成品（网站活在她自己的世界里）。我们保管的是她从成品那里
// 学到的东西。

var pblKeepKinds = map[string]bool{"stat": true, "feedback": true, "thought": true}
var pblKeepStages = map[string]bool{
	"ship": true, "observe": true, "interpret": true, "change": true,
}

type pblKeepDTO struct {
	ID        string  `json:"id"`
	Kind      string  `json:"kind"`
	Body      string  `json:"body"`
	Stage     string  `json:"stage"`
	SessionID *string `json:"sessionId"`
	CreatedAt string  `json:"createdAt"`
}

func toPblKeepDTO(k sqlc.PblKeepEntry) pblKeepDTO {
	out := pblKeepDTO{
		ID: k.ID.String(), Kind: k.Kind, Body: k.Body, Stage: k.Stage,
		CreatedAt: k.CreatedAt.Format(time.RFC3339),
	}
	if k.SessionID.Valid {
		s := uuid.UUID(k.SessionID.Bytes).String()
		out.SessionID = &s
	}
	return out
}

func (a *API) listPblKeepEntries(w http.ResponseWriter, r *http.Request) {
	atomID, ok := a.loadOwnedPblProject(w, r)
	if !ok {
		return
	}
	rows, err := a.d.Queries.ListPblKeepEntries(r.Context(), atomID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	out := make([]pblKeepDTO, 0, len(rows))
	for _, k := range rows {
		out = append(out, toPblKeepDTO(k))
	}
	httpx.WriteJSON(w, http.StatusOK, out)
}

func (a *API) createPblKeepEntry(w http.ResponseWriter, r *http.Request) {
	atomID, ok := a.loadOwnedPblProject(w, r)
	if !ok {
		return
	}
	var req struct {
		Kind  string `json:"kind"`
		Body  string `json:"body"`
		Stage string `json:"stage"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteError(w, r, httpx.ErrBadRequest("bad_json", "请求格式不对", nil))
		return
	}
	kind := strings.TrimSpace(req.Kind)
	if !pblKeepKinds[kind] {
		httpx.WriteError(w, r, httpx.ErrBadRequest("bad_kind", "这是数据、反馈，还是你的想法？", nil))
		return
	}
	body := strings.TrimSpace(req.Body)
	if body == "" {
		httpx.WriteError(w, r, httpx.ErrBadRequest("empty", "写点什么", nil))
		return
	}
	stage := strings.TrimSpace(req.Stage)
	if !pblKeepStages[stage] {
		stage = "observe"
	}
	row, err := a.d.Queries.CreatePblKeepEntry(r.Context(), sqlc.CreatePblKeepEntryParams{
		AtomID: atomID, Kind: kind, Body: body, Stage: stage,
	})
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, toPblKeepDTO(row))
}

// openPblKeepSession —— 一条数据长出一轮新的思考。
//
// 「so one project may have several sessions」——这一步就是维持和归档的分界：
// 数字看过就算了，那是归档；数字让她重新想一遍，这个项目还活着。
//
// 一个条目只开一轮：再点一次应该回到原来那一轮，而不是又开一条平行的线。
func (a *API) openPblKeepSession(w http.ResponseWriter, r *http.Request) {
	atomID, ok := a.loadOwnedPblProject(w, r)
	if !ok {
		return
	}
	kid, err := uuid.Parse(r.PathValue("kid"))
	if err != nil {
		httpx.WriteError(w, r, httpx.ErrNotFound("这一条不存在"))
		return
	}
	entry, err := a.d.Queries.GetPblKeepEntry(r.Context(), kid)
	if err != nil || entry.AtomID != atomID {
		httpx.WriteError(w, r, httpx.ErrNotFound("这一条不存在"))
		return
	}
	if entry.SessionID.Valid {
		httpx.WriteJSON(w, http.StatusOK, map[string]any{
			"sessionId": uuid.UUID(entry.SessionID.Bytes).String(),
		})
		return
	}

	question := "这条说明了什么？下一步该改哪一件事？"
	sess, err := a.d.Queries.CreatePblSession(r.Context(), sqlc.CreatePblSessionParams{
		AtomID: atomID, Kind: "keeping", ParentID: pgtype.UUID{}, Depth: 0,
		AnchorKind: "free", AnchorRef: entry.ID.String(), Question: question,
	})
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if _, err := a.d.Queries.LinkPblKeepEntrySession(r.Context(), sqlc.LinkPblKeepEntrySessionParams{
		ID: entry.ID, SessionID: pgtype.UUID{Bytes: sess.ID, Valid: true},
	}); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, map[string]any{"sessionId": sess.ID.String()})
}
