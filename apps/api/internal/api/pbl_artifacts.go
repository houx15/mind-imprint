package api

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"mindimprint/api/internal/httpx"
	"mindimprint/api/internal/pbl"
	"mindimprint/api/internal/store/sqlc"
)

// pbl_artifacts.go — 成果，以及工具的端点。
//
// Two rules live here, and they are the reason this file is not just CRUD:
//
//   1. 印记 hands over what it GUESSED and what it ADMITS is still wrong. A
//      handover that hides its assumptions can only be accepted, never
//      reviewed — and reviewing is the point.
//   2. Nothing settles without a reason. The moment 「就用这个」 works on its
//      own, this is a machine that generates and a student who approves.
//
// 🚨 The tool endpoints are deliberately shallow: they record that a tool was
// summoned, why, and what came of it. There is no spec, no field vocabulary and
// no renderer, because the product owner rejected the form interaction on
// 2026-09-01 and the replacement is not designed. Deferring the INTERACTION is
// not the same as deferring the record.

var pblArtifactKinds = map[string]bool{
	"options": true, "draft": true, "spec": true,
	"image": true, "site": true, "html": true,
}

type pblArtifactDTO struct {
	Superseded bool            `json:"superseded,omitempty"`
	Stale      bool            `json:"stale,omitempty"`
	ID         string          `json:"id"`
	Kind       string          `json:"kind"`
	Title      string          `json:"title"`
	Payload    json.RawMessage `json:"payload"`
	Guessed    []string        `json:"guessed"`
	Admits     []string        `json:"admits"`
	Verdict    *string         `json:"verdict"`
	Why        string          `json:"why"`
	SettledAt  *string         `json:"settledAt"`
	CreatedAt  string          `json:"createdAt"`
}

func toPblArtifactDTO(a sqlc.PblArtifact) pblArtifactDTO {
	out := pblArtifactDTO{
		ID: a.ID.String(), Kind: a.Kind, Title: a.Title,
		Payload: json.RawMessage(a.Payload), Why: a.Why,
		CreatedAt: a.CreatedAt.Format(time.RFC3339),
	}
	_ = json.Unmarshal(a.Guessed, &out.Guessed)
	_ = json.Unmarshal(a.Admits, &out.Admits)
	if out.Guessed == nil {
		out.Guessed = []string{}
	}
	if out.Admits == nil {
		out.Admits = []string{}
	}
	if a.Verdict != nil {
		out.Verdict = a.Verdict
	}
	if a.SettledAt.Valid {
		s := a.SettledAt.Time.Format(time.RFC3339)
		out.SettledAt = &s
	}
	return out
}

func (a *API) listPblArtifacts(w http.ResponseWriter, r *http.Request) {
	atomID, ok := a.loadOwnedPblProject(w, r)
	if !ok {
		return
	}
	rows, err := a.d.Queries.ListPblArtifacts(r.Context(), atomID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	superseded := supersededArtifactIDs(rows)
	out := make([]pblArtifactDTO, 0, len(rows))
	for _, x := range rows {
		dto := toPblArtifactDTO(x)
		dto.Superseded = superseded[x.ID]
		if x.Kind == "site" && !x.SettledAt.Valid {
			dto.Stale, err = a.siteReviewStale(r, x.Kind, x.AtomID, x.Payload)
			if err != nil {
				httpx.WriteError(w, r, err)
				return
			}
		}
		out = append(out, dto)
	}
	httpx.WriteJSON(w, http.StatusOK, out)
}

// handOverPblArtifact — 印记 puts something it made on the table.
func (a *API) handOverPblArtifact(w http.ResponseWriter, r *http.Request) {
	atomID, ok := a.loadOwnedPblProject(w, r)
	if !ok {
		return
	}
	var req struct {
		Kind      string          `json:"kind"`
		Title     string          `json:"title"`
		Payload   json.RawMessage `json:"payload"`
		Guessed   []string        `json:"guessed"`
		Admits    []string        `json:"admits"`
		SessionID string          `json:"sessionId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteError(w, r, errBadJSON(err))
		return
	}
	kind := strings.TrimSpace(req.Kind)
	if !pblArtifactKinds[kind] {
		httpx.WriteError(w, r, httpx.ErrBadRequest("bad_kind", "不认识这种成果", nil))
		return
	}
	// 🚨 Both required. An artifact that names neither what it guessed nor what
	// is still wrong with it is one she can only accept.
	if len(req.Guessed) == 0 || len(req.Admits) == 0 {
		httpx.WriteError(w, r, httpx.ErrBadRequest("no_disclosure",
			"交出来的东西要说清楚：猜了什么，哪里还不对", nil))
		return
	}

	sess := pgtype.UUID{}
	if raw := strings.TrimSpace(req.SessionID); raw != "" {
		sid, perr := uuid.Parse(raw)
		if perr != nil {
			httpx.WriteError(w, r, httpx.ErrNotFound("这一层不存在"))
			return
		}
		s, gerr := a.d.Queries.GetPblSession(r.Context(), sid)
		if gerr != nil || s.AtomID != atomID {
			httpx.WriteError(w, r, httpx.ErrNotFound("这一层不存在"))
			return
		}
		sess = pgtype.UUID{Bytes: sid, Valid: true}
	}

	guessed, err := json.Marshal(req.Guessed)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	admits, err := json.Marshal(req.Admits)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	payload := req.Payload
	if len(payload) == 0 {
		payload = json.RawMessage(`{}`)
	}

	row, err := a.d.Queries.CreatePblArtifact(r.Context(), sqlc.CreatePblArtifactParams{
		AtomID: atomID, SessionID: sess, Kind: kind,
		Title: strings.TrimSpace(req.Title), Payload: payload,
		Guessed: guessed, Admits: admits,
	})
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, toPblArtifactDTO(row))
}

// settlePblArtifact — 🚨 the reason gate.
func (a *API) settlePblArtifact(w http.ResponseWriter, r *http.Request) {
	atomID, ok := a.loadOwnedPblProject(w, r)
	if !ok {
		return
	}
	aid, err := uuid.Parse(r.PathValue("aid"))
	if err != nil {
		httpx.WriteError(w, r, httpx.ErrNotFound("这件成果不存在"))
		return
	}
	row, err := a.d.Queries.GetPblArtifact(r.Context(), aid)
	if err != nil || row.AtomID != atomID {
		httpx.WriteError(w, r, httpx.ErrNotFound("这件成果不存在"))
		return
	}
	versions, err := a.d.Queries.ListPblArtifacts(r.Context(), atomID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if supersededArtifactIDs(versions)[aid] {
		httpx.WriteError(w, r, httpx.ErrBadRequest("superseded_artifact", "这份成果已有新版，请审核最新版本；旧版记录仍可查看", nil))
		return
	}
	var req struct {
		Verdict string `json:"verdict"`
		Why     string `json:"why"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteError(w, r, errBadJSON(err))
		return
	}
	verdict := strings.TrimSpace(req.Verdict)
	if verdict == "kept" {
		stale, err := a.siteReviewStale(r, row.Kind, row.AtomID, row.Payload)
		if err != nil {
			httpx.WriteError(w, r, err)
			return
		}
		if stale {
			httpx.WriteError(w, r, httpx.ErrBadRequest("stale_site_review", "主页已更新，请刷新审核后检查当前页面", nil))
			return
		}
	}
	switch verdict {
	case "kept", "revise", "dropped":
	default:
		httpx.WriteError(w, r, httpx.ErrBadRequest("bad_verdict", "不认识这个判断", nil))
		return
	}
	// 🚨 理由只在「重新执行任务」这一档是必填的（产品负责人 2026-09-02）。
	//
	// 三档的差别是有道理的：
	//   审核通过  —— 她没有意见，硬要她写一句就是逼她编。
	//   执行修改  —— 她的意见已经逐条写在划线和审核要点里了，那些就是指令。
	//   重新执行  —— 推倒重来，如果不说清往哪个方向重做，印记只能再猜一遍，
	//                而她会拿到第二份同样不对的东西。
	if verdict == "dropped" && strings.TrimSpace(req.Why) == "" {
		httpx.WriteError(w, r, httpx.ErrBadRequest("no_reason",
			"重新执行需要一个明确的方向，否则只会再做出一份一样的东西", nil))
		return
	}

	out, err := a.d.Queries.SettlePblArtifact(r.Context(), sqlc.SettlePblArtifactParams{
		ID: aid, Verdict: &verdict, Why: strings.TrimSpace(req.Why),
	})
	if err != nil {
		httpx.WriteError(w, r, httpx.ErrBadRequest("already_settled", "这件已经定过了", nil))
		return
	}
	httpx.WriteJSON(w, http.StatusOK, toPblArtifactDTO(out))
}

/* ── 工具 ────────────────────────────────────────────────────────────────
 *
 * 一件工具的一生：印记递出来（summoned）→ 她打开（accepted）→ 有了结果
 * （done），或者她不用（declined）。
 *
 * 🚨 accepted 这一档是给 world 工具留的。她答应"出去看看"之后可能几天不回
 * 来；没有这一档，「正在做」和「已完成」就只能二选一，印记只好去猜——猜错
 * 的那一半就是催她。
 */

type pblToolDTO struct {
	ID         string          `json:"id"`
	SessionID  *string         `json:"sessionId"`
	Tool       string          `json:"tool"`
	Kind       string          `json:"kind"`
	Label      string          `json:"label"`
	Reason     string          `json:"reason"`
	Status     string          `json:"status"`
	Result     json.RawMessage `json:"result,omitempty"`
	Note       string          `json:"note"`
	AcceptedAt *string         `json:"acceptedAt"`
	ResolvedAt *string         `json:"resolvedAt"`
	CreatedAt  string          `json:"createdAt"`
}

func toPblToolDTO(t sqlc.PblToolInstance) pblToolDTO {
	out := pblToolDTO{
		ID: t.ID.String(), Tool: t.Tool, Kind: t.Kind, Label: t.Tool,
		Reason: t.Reason, Status: t.Status, Note: t.StudentNote,
		Result: json.RawMessage(t.Result), CreatedAt: t.CreatedAt.Format(time.RFC3339),
	}
	if t.SessionID.Valid {
		s := uuid.UUID(t.SessionID.Bytes).String()
		out.SessionID = &s
	}
	// 表里有就用表里的名字；表外的工具就用它自己的名字，界面照样能显示。
	if def, ok := pbl.LookupTool(t.Tool); ok {
		out.Label = def.Label
	}
	if t.AcceptedAt.Valid {
		s := t.AcceptedAt.Time.Format(time.RFC3339)
		out.AcceptedAt = &s
	}
	if t.ResolvedAt.Valid {
		s := t.ResolvedAt.Time.Format(time.RFC3339)
		out.ResolvedAt = &s
	}
	return out
}

func (a *API) listPblTools(w http.ResponseWriter, r *http.Request) {
	atomID, ok := a.loadOwnedPblProject(w, r)
	if !ok {
		return
	}
	rows, err := a.d.Queries.ListPblTools(r.Context(), atomID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	out := make([]pblToolDTO, 0, len(rows))
	for _, t := range rows {
		out = append(out, toPblToolDTO(t))
	}
	httpx.WriteJSON(w, http.StatusOK, out)
}

// summonPblTool records that 印记 offered a tool, and why.
//
// `tool` is a free string on purpose: the toolbox is open, and a new tool must
// not need a migration. What a tool LOOKS like is the open design question.
func (a *API) summonPblTool(w http.ResponseWriter, r *http.Request) {
	atomID, ok := a.loadOwnedPblProject(w, r)
	if !ok {
		return
	}
	var req struct {
		Tool      string `json:"tool"`
		Kind      string `json:"kind"`
		Reason    string `json:"reason"`
		SessionID string `json:"sessionId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteError(w, r, errBadJSON(err))
		return
	}
	tool := strings.TrimSpace(req.Tool)
	if tool == "" {
		httpx.WriteError(w, r, httpx.ErrBadRequest("no_tool", "没说是哪件工具", nil))
		return
	}
	// An unexplained tool is an ambush — this product's whole claim is that she
	// can see what the AI is doing.
	if strings.TrimSpace(req.Reason) == "" {
		httpx.WriteError(w, r, httpx.ErrBadRequest("no_reason",
			"说清楚为什么这时候递这件工具", nil))
		return
	}

	sess := pgtype.UUID{}
	if raw := strings.TrimSpace(req.SessionID); raw != "" {
		sid, perr := uuid.Parse(raw)
		if perr != nil {
			httpx.WriteError(w, r, httpx.ErrNotFound("这一层不存在"))
			return
		}
		s, gerr := a.d.Queries.GetPblSession(r.Context(), sid)
		if gerr != nil || s.AtomID != atomID {
			httpx.WriteError(w, r, httpx.ErrNotFound("这一层不存在"))
			return
		}
		sess = pgtype.UUID{Bytes: sid, Valid: true}
	}

	row, err := a.d.Queries.SummonPblTool(r.Context(), sqlc.SummonPblToolParams{
		AtomID: atomID, SessionID: sess, Tool: tool,
		Reason: strings.TrimSpace(req.Reason),
		// 已知工具以工具箱为准，表外的才听请求的（internal/pbl/tools.go）。
		Kind: pbl.ResolveToolKind(tool, strings.TrimSpace(req.Kind)),
	})
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, toPblToolDTO(row))
}

// acceptPblTool —— 她打开了这件工具。
//
// 单独一个端点而不是 resolve 的一种状态，因为它回答的是另一个问题：不是
// 「结果是什么」，而是「她现在在做吗」。world 工具最需要这个区分。
func (a *API) acceptPblTool(w http.ResponseWriter, r *http.Request) {
	atomID, ok := a.loadOwnedPblProject(w, r)
	if !ok {
		return
	}
	tid, err := uuid.Parse(r.PathValue("tid"))
	if err != nil {
		httpx.WriteError(w, r, httpx.ErrNotFound("这件工具不存在"))
		return
	}
	row, err := a.d.Queries.GetPblTool(r.Context(), tid)
	if err != nil || row.AtomID != atomID {
		httpx.WriteError(w, r, httpx.ErrNotFound("这件工具不存在"))
		return
	}
	out, err := a.d.Queries.AcceptPblTool(r.Context(), tid)
	if err != nil {
		httpx.WriteError(w, r, httpx.ErrBadRequest("not_open", "这件工具不是刚递出来的状态", nil))
		return
	}
	httpx.WriteJSON(w, http.StatusOK, toPblToolDTO(out))
}

// appendPblToolRecord 在对话里留一条"她用完了这件工具"的痕迹。
//
// role=system：界面上它是一条灰色的记录，不是一个气泡——因为没有人说过这句话，
// 是这件事发生了。她自己写的那句（如果有）原样跟在后面，一个字不改。
//
// 失败不影响这次请求：工具结果已经落库，少一条痕迹远好过整件事回滚。
func (a *API) appendPblToolRecord(r *http.Request, atomID uuid.UUID, sess pgtype.UUID, tool, note string) {
	label := tool
	if def, ok := pbl.LookupTool(tool); ok {
		label = def.Label
	}
	line := "用完了「" + label + "」"
	if note != "" {
		line += "：" + note
	}
	a.appendPblStatus(r, atomID, sess, line)
}

func (a *API) appendPblStatus(r *http.Request, atomID uuid.UUID, sess pgtype.UUID, line string) {
	warn := func(err error) {
		slog.Warn("pbl: could not record status",
			"err", err, "atom_id", atomID)
	}
	tx, err := a.d.Pool.Begin(r.Context())
	if err != nil {
		warn(err)
		return
	}
	defer func() { _ = tx.Rollback(r.Context()) }()
	qtx := a.d.Queries.WithTx(tx)
	if _, err := qtx.LockAtom(r.Context(), atomID); err != nil {
		warn(err)
		return
	}
	next, err := qtx.NextAtomMessageSeq(r.Context(), atomID)
	if err != nil {
		warn(err)
		return
	}
	if _, err := qtx.AppendPblSessionMessage(r.Context(), sqlc.AppendPblSessionMessageParams{
		AtomID: atomID, Seq: next, Role: "system", Content: line, SessionID: sess,
	}); err != nil {
		warn(err)
		return
	}
	if err := tx.Commit(r.Context()); err != nil {
		warn(err)
	}
}

// resolvePblTool records what came of a summoned tool.
//
// `declined` is a first-class status: 铁律② says the tool triggers
// automatically but SHE confirms opening it, so declining is a normal outcome —
// and 过程即数据 says a decline is a record, not an absence.
func (a *API) resolvePblTool(w http.ResponseWriter, r *http.Request) {
	atomID, ok := a.loadOwnedPblProject(w, r)
	if !ok {
		return
	}
	tid, err := uuid.Parse(r.PathValue("tid"))
	if err != nil {
		httpx.WriteError(w, r, httpx.ErrNotFound("这件工具不存在"))
		return
	}
	row, err := a.d.Queries.GetPblTool(r.Context(), tid)
	if err != nil || row.AtomID != atomID {
		httpx.WriteError(w, r, httpx.ErrNotFound("这件工具不存在"))
		return
	}
	var req struct {
		Status string          `json:"status"`
		Result json.RawMessage `json:"result"`
		Note   string          `json:"note"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteError(w, r, errBadJSON(err))
		return
	}
	status := strings.TrimSpace(req.Status)
	if status != "declined" && status != "done" {
		httpx.WriteError(w, r, httpx.ErrBadRequest("bad_status", "不认识这个结果", nil))
		return
	}
	// 🚨 拒绝不要理由。理由字段是留给她想说的时候用的，不是门槛——
	// 如果拒绝比接受更费事，那就不是一个真的选择（铁律②）。
	note := strings.TrimSpace(req.Note)
	if row.Tool == "creative" && len(req.Result) > 0 {
		result := creativeContext(req.Result)
		if result == "" {
			httpx.WriteError(w, r, httpx.ErrBadRequest("bad_result", "创作结果格式无效", nil))
			return
		}
		req.Result = json.RawMessage(result)
	}
	out, err := a.d.Queries.ResolvePblTool(r.Context(), sqlc.ResolvePblToolParams{
		ID: tid, Status: status, Result: req.Result, StudentNote: note,
	})
	if err != nil {
		httpx.WriteError(w, r, httpx.ErrBadRequest("already_resolved", "这件已经有结果了", nil))
		return
	}

	// 用完一件工具，对话里留一条记录。
	//
	// 原来是前端把结果拼成一句「我想了 4 个办法，先试这个：X。因为Y」，当成她
	// 说的话发回对话。换成一条记录，理由很实在：那种拼出来的句子读着像表单输出，
	// 不像一个初中生说的话。她自己写的那句原样带上，没写就只留"用完了"。
	//
	// （这不是铁律问题。铁律管的是不替她写正文——产品负责人 2026-09-02 纠正过
	// 我一次，别再把那条往外扩。）
	if status == "done" {
		a.appendPblToolRecord(r, atomID, row.SessionID, out.Tool, note)
	}
	httpx.WriteJSON(w, http.StatusOK, toPblToolDTO(out))
}
