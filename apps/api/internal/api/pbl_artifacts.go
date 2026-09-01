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
	ID        string          `json:"id"`
	Kind      string          `json:"kind"`
	Title     string          `json:"title"`
	Payload   json.RawMessage `json:"payload"`
	Guessed   []string        `json:"guessed"`
	Admits    []string        `json:"admits"`
	Verdict   *string         `json:"verdict"`
	Why       string          `json:"why"`
	SettledAt *string         `json:"settledAt"`
	CreatedAt string          `json:"createdAt"`
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
	out := make([]pblArtifactDTO, 0, len(rows))
	for _, x := range rows {
		out = append(out, toPblArtifactDTO(x))
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
		httpx.WriteError(w, r, httpx.ErrBadRequest("bad_json", "请求格式不对", nil))
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
	var req struct {
		Verdict string `json:"verdict"`
		Why     string `json:"why"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteError(w, r, httpx.ErrBadRequest("bad_json", "请求格式不对", nil))
		return
	}
	verdict := strings.TrimSpace(req.Verdict)
	switch verdict {
	case "kept", "revise", "dropped":
	default:
		httpx.WriteError(w, r, httpx.ErrBadRequest("bad_verdict", "不认识这个判断", nil))
		return
	}
	// The whole design in one condition.
	if strings.TrimSpace(req.Why) == "" {
		httpx.WriteError(w, r, httpx.ErrBadRequest("no_reason",
			"写一句为什么——收下也好，退回也好，都要说得出理由", nil))
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

/* ── tools: endpoint retained, interaction deferred ─────────────────────── */

type pblToolDTO struct {
	ID        string          `json:"id"`
	Tool      string          `json:"tool"`
	Reason    string          `json:"reason"`
	Status    string          `json:"status"`
	Result    json.RawMessage `json:"result,omitempty"`
	CreatedAt string          `json:"createdAt"`
}

func toPblToolDTO(t sqlc.PblToolInstance) pblToolDTO {
	return pblToolDTO{
		ID: t.ID.String(), Tool: t.Tool, Reason: t.Reason, Status: t.Status,
		Result: json.RawMessage(t.Result), CreatedAt: t.CreatedAt.Format(time.RFC3339),
	}
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
		Reason    string `json:"reason"`
		SessionID string `json:"sessionId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteError(w, r, httpx.ErrBadRequest("bad_json", "请求格式不对", nil))
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
		AtomID: atomID, SessionID: sess, Tool: tool, Reason: strings.TrimSpace(req.Reason),
	})
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, toPblToolDTO(row))
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
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteError(w, r, httpx.ErrBadRequest("bad_json", "请求格式不对", nil))
		return
	}
	status := strings.TrimSpace(req.Status)
	if status != "declined" && status != "done" {
		httpx.WriteError(w, r, httpx.ErrBadRequest("bad_status", "不认识这个结果", nil))
		return
	}
	out, err := a.d.Queries.ResolvePblTool(r.Context(), sqlc.ResolvePblToolParams{
		ID: tid, Status: status, Result: req.Result,
	})
	if err != nil {
		httpx.WriteError(w, r, httpx.ErrBadRequest("already_resolved", "这件已经有结果了", nil))
		return
	}
	httpx.WriteJSON(w, http.StatusOK, toPblToolDTO(out))
}
