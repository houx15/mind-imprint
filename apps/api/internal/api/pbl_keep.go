package api

import (
	"encoding/json"
	"net/http"
	"strconv"
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
	// 一个数字、它的单位，和上一次是多少。数字的意思在变化里。
	Metric string   `json:"metric"`
	Value  *float64 `json:"value"`
	Prev   *float64 `json:"prev"`
	Unit   string   `json:"unit"`
	// 改这一件事时她的预期，以及后来兑现了没有（'' / met / missed）。
	Expect    string  `json:"expect"`
	Verdict   string  `json:"verdict"`
	SessionID *string `json:"sessionId"`
	CreatedAt string  `json:"createdAt"`
}

func toPblKeepDTO(k sqlc.PblKeepEntry) pblKeepDTO {
	out := pblKeepDTO{
		ID: k.ID.String(), Kind: k.Kind, Body: k.Body, Stage: k.Stage,
		Metric: k.Metric, Unit: k.Unit, Expect: k.Expect, Verdict: k.Verdict,
		Value:     numericToFloat(k.Value),
		Prev:      numericToFloat(k.Prev),
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
		// 一个数字，和它的单位。指标名给了才存数——一个没名字的数字过两周
		// 她自己也认不出是什么。
		Metric string   `json:"metric"`
		Value  *float64 `json:"value"`
		Unit   string   `json:"unit"`
		// 改这一件事时她的预期。空 = 一次普通的改动，不强求（铁律④）。
		Expect string `json:"expect"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteError(w, r, errBadJSON(err))
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
	// 🚨 上一次是多少，由服务端查，不由她填。
	//
	// 一个数字本身不说明任何事：「这周 23 个人用了」是多还是少？只有和上一次比
	// 才有意思。让她手填上一次，她要么记不得、要么填个印象——那就把「变化」这
	// 件事变回了感觉。
	metric := strings.TrimSpace(req.Metric)
	var value, prev pgtype.Numeric
	if metric != "" && req.Value != nil {
		if err := value.Scan(strconv.FormatFloat(*req.Value, 'f', -1, 64)); err != nil {
			httpx.WriteError(w, r, httpx.ErrBadRequest("bad_value", "这个数字读不出来", nil))
			return
		}
		if last, lerr := a.d.Queries.LastPblKeepMetric(r.Context(),
			sqlc.LastPblKeepMetricParams{AtomID: atomID, Metric: metric}); lerr == nil {
			prev = last.Value
		}
	}
	row, err := a.d.Queries.CreatePblKeepEntry(r.Context(), sqlc.CreatePblKeepEntryParams{
		AtomID: atomID, Kind: kind, Body: body, Stage: stage,
		Metric: metric, Value: value, Prev: prev,
		Unit: strings.TrimSpace(req.Unit), Expect: strings.TrimSpace(req.Expect),
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

	// 🚨 一次只问一个。原来那句把两个问题塞进一句（说明了什么 + 该改哪件事），
	// 而它是这一层的题目、会当标题显示——产品负责人 2026-09-02 在文案表上标了
	// 「didn't understand this」。
	question := "这条反馈说明了什么"
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

// numericToFloat 把库里的 numeric 变成 JSON 里的数字；空就是 null。
//
// 前端要拿它算差值和画走势，字符串在那边只会被到处 parseFloat 一遍。
func numericToFloat(n pgtype.Numeric) *float64 {
	if !n.Valid {
		return nil
	}
	f, err := n.Float64Value()
	if err != nil || !f.Valid {
		return nil
	}
	v := f.Float64
	return &v
}

// settlePblKeepPrediction —— 一次改动的预期后来兑现了没有。
//
// 🚨 **没兑现才是最值钱的那一次**：它说明她原来想错了，而那正是迭代要教的东西。
// 所以这里不庆祝兑现、也不惩罚没兑现，只是记下来，让印记接得上。
func (a *API) settlePblKeepPrediction(w http.ResponseWriter, r *http.Request) {
	atomID, ok := a.loadOwnedPblProject(w, r)
	if !ok {
		return
	}
	kid, err := uuid.Parse(r.PathValue("kid"))
	if err != nil {
		httpx.WriteError(w, r, httpx.ErrNotFound("这一条不存在"))
		return
	}
	var req struct {
		Verdict string `json:"verdict"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteError(w, r, errBadJSON(err))
		return
	}
	v := strings.TrimSpace(req.Verdict)
	if v != "" && v != "met" && v != "missed" {
		httpx.WriteError(w, r, httpx.ErrBadRequest("bad_verdict", "不认识这种结果", nil))
		return
	}
	out, err := a.d.Queries.SettlePblKeepPrediction(r.Context(),
		sqlc.SettlePblKeepPredictionParams{ID: kid, Verdict: v, AtomID: atomID})
	if err != nil {
		httpx.WriteError(w, r, httpx.ErrNotFound("这一条不存在"))
		return
	}
	httpx.WriteJSON(w, http.StatusOK, toPblKeepDTO(out))
}
