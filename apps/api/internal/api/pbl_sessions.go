package api

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"mindimprint/api/internal/httpx"
	"mindimprint/api/internal/pbl"
	"mindimprint/api/internal/store/sqlc"
)

// pbl_sessions.go — 深挖与四种思考模式共用的那套端点。
//
// A session is one object with a kind: an untyped dig she opened off a hook
// question, or one of the four Thinking Sessions. Same table, same thread
// plumbing, same write-back gate — see spec §10.

// loadOwnedPblProject resolves {id} to a project this student owns.
//
// 🚨 Someone else's project answers 404, never 403. A 403 confirms the project
// exists, which is a different sentence than "there is nothing here" and not
// one we want to say about another student's work.
func (a *API) loadOwnedPblProject(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	u, _ := UserFromContext(r.Context())
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		httpx.WriteError(w, r, httpx.ErrNotFound("项目不存在"))
		return uuid.Nil, false
	}
	row, err := a.d.Queries.GetPblProject(r.Context(), id)
	if err != nil || row.UserID != u.ID {
		httpx.WriteError(w, r, httpx.ErrNotFound("项目不存在"))
		return uuid.Nil, false
	}
	return row.AtomID, true
}

type pblSessionDTO struct {
	ID         string            `json:"id"`
	Kind       string            `json:"kind"`
	ParentID   *string           `json:"parentId"`
	Depth      int               `json:"depth"`
	AnchorKind string            `json:"anchorKind"`
	AnchorRef  string            `json:"anchorRef"`
	Question   string            `json:"question"`
	Takeaway   string            `json:"takeaway"`
	WriteBack  map[string]string `json:"writeBack,omitempty"`
	ClosedAt   *string           `json:"closedAt"`
	CreatedAt  string            `json:"createdAt"`
}

func toPblSessionDTO(s sqlc.PblSession) pblSessionDTO {
	out := pblSessionDTO{
		ID: s.ID.String(), Kind: s.Kind, Depth: int(s.Depth),
		AnchorKind: s.AnchorKind, AnchorRef: s.AnchorRef, Question: s.Question,
		Takeaway: s.Takeaway, CreatedAt: s.CreatedAt.Format(time.RFC3339),
	}
	if s.ParentID.Valid {
		p := uuid.UUID(s.ParentID.Bytes).String()
		out.ParentID = &p
	}
	if s.ClosedAt.Valid {
		c := s.ClosedAt.Time.Format(time.RFC3339)
		out.ClosedAt = &c
	}
	if len(s.Writeback) > 0 {
		_ = json.Unmarshal(s.Writeback, &out.WriteBack)
	}
	return out
}

// openPblSession — 印记 proposed a session, or she opened one herself.
func (a *API) openPblSession(w http.ResponseWriter, r *http.Request) {
	atomID, ok := a.loadOwnedPblProject(w, r)
	if !ok {
		return
	}
	var req struct {
		Kind       string  `json:"kind"`
		ParentID   *string `json:"parentId"`
		AnchorKind string  `json:"anchorKind"`
		AnchorRef  string  `json:"anchorRef"`
		Question   string  `json:"question"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteError(w, r, errBadJSON(err))
		return
	}
	kind := strings.TrimSpace(req.Kind)
	if kind == "" {
		kind = "free"
	}
	if !pbl.IsSessionKind(kind) {
		httpx.WriteError(w, r, httpx.ErrBadRequest("bad_kind", "不认识这种深挖", nil))
		return
	}

	// Resolve the parent before computing depth. A parent that does not exist,
	// or belongs to another project, is not a parent — and refusing it is also
	// what makes a cycle impossible in a tree built one node at a time.
	var parent pgtype.UUID
	parentExists := false
	parentDepth := 0
	if req.ParentID != nil && strings.TrimSpace(*req.ParentID) != "" {
		pid, err := uuid.Parse(strings.TrimSpace(*req.ParentID))
		if err != nil {
			httpx.WriteError(w, r, httpx.ErrBadRequest("bad_parent", "找不到要接着挖的那一层", nil))
			return
		}
		prow, err := a.d.Queries.GetPblSession(r.Context(), pid)
		if err != nil || prow.AtomID != atomID {
			httpx.WriteError(w, r, httpx.ErrBadRequest("bad_parent", "找不到要接着挖的那一层", nil))
			return
		}
		parent = pgtype.UUID{Bytes: pid, Valid: true}
		parentExists = true
		parentDepth = int(prow.Depth)
	}

	depth, err := pbl.ChildDepth(parentExists, parentDepth)
	if err != nil {
		// Not a server error: she has hit the bottom of the nesting, and the
		// honest thing is to say so rather than to silently flatten it.
		httpx.WriteError(w, r, httpx.ErrBadRequest("too_deep", "已达到讨论层级上限，请继续当前讨论", nil))
		return
	}

	anchorKind := strings.TrimSpace(req.AnchorKind)
	if anchorKind == "" {
		anchorKind = "free"
	}
	s, err := a.d.Queries.CreatePblSession(r.Context(), sqlc.CreatePblSessionParams{
		AtomID: atomID, Kind: kind, ParentID: parent, Depth: int16(depth),
		AnchorKind: anchorKind, AnchorRef: strings.TrimSpace(req.AnchorRef),
		Question: strings.TrimSpace(req.Question),
	})
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, toPblSessionDTO(s))
}

// closePblSession — the write-back gate, and the only way a session ends.
func (a *API) closePblSession(w http.ResponseWriter, r *http.Request) {
	atomID, ok := a.loadOwnedPblProject(w, r)
	if !ok {
		return
	}
	sid, err := uuid.Parse(r.PathValue("sid"))
	if err != nil {
		httpx.WriteError(w, r, httpx.ErrNotFound("这一层不存在"))
		return
	}
	s, err := a.d.Queries.GetPblSession(r.Context(), sid)
	if err != nil || s.AtomID != atomID {
		httpx.WriteError(w, r, httpx.ErrNotFound("这一层不存在"))
		return
	}
	if s.ClosedAt.Valid {
		httpx.WriteError(w, r, httpx.ErrBadRequest("already_closed", "这一层已经收起来了", nil))
		return
	}

	var req struct {
		Takeaway  string            `json:"takeaway"`
		WriteBack map[string]string `json:"writeBack"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteError(w, r, errBadJSON(err))
		return
	}

	// 🚨 The gate. A session that records nothing is 方法论表演 — the ritual
	// completed while the project's question, evidence and plan stayed put.
	// Refused in the handler, not by a disabled button.
	wb := pbl.WriteBack{Takeaway: req.Takeaway, Fields: req.WriteBack}
	if err := pbl.ValidateClose(s.Kind, wb); err != nil {
		httpx.WriteError(w, r, httpx.ErrBadRequest("no_writeback",
			"请记录本次讨论的结论后再结束", nil))
		return
	}

	payload, err := json.Marshal(req.WriteBack)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}

	// Closing and returning the conclusion are one transaction: a session that
	// closed without its write-back reaching the parent thread would be a
	// conclusion nobody can see.
	tx, err := a.d.Pool.Begin(r.Context())
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	defer func() { _ = tx.Rollback(r.Context()) }()
	qtx := a.d.Queries.WithTx(tx)

	closed, err := qtx.ClosePblSession(r.Context(), sqlc.ClosePblSessionParams{
		ID: sid, Takeaway: strings.TrimSpace(req.Takeaway), Writeback: payload,
	})
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}

	// The write-back goes to the PARENT thread — the main thread for a
	// top-level session, the parent session for a nested one. Skipping a level
	// would deliver a conclusion without the context that produced it.
	if _, err := qtx.LockAtom(r.Context(), atomID); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	next, err := qtx.NextAtomMessageSeq(r.Context(), atomID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	origin, err := json.Marshal(map[string]any{
		"kind": "session_writeback", "sessionId": sid.String(), "sessionKind": s.Kind,
	})
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	body := strings.TrimSpace(req.Takeaway)
	if body == "" {
		body = summariseWriteBack(req.WriteBack)
	}
	if _, err := qtx.AppendPblSessionMessage(r.Context(), sqlc.AppendPblSessionMessageParams{
		AtomID: atomID, Seq: next, Role: "system", Content: body,
		Payload: origin, SessionID: closed.ParentID,
	}); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if err := tx.Commit(r.Context()); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, toPblSessionDTO(closed))
}

// summariseWriteBack renders a typed session's fields as one line, for the
// message that returns to the parent thread. Used only when there is no
// takeaway — a typed session's contract is its fields, and forcing her to
// ALSO write a sentence would be asking the same thing twice.
func summariseWriteBack(fields map[string]string) string {
	parts := make([]string, 0, len(fields))
	for _, k := range []string{"frame", "next_bet", "observation", "resolution", "reason", "reading"} {
		if v := strings.TrimSpace(fields[k]); v != "" {
			parts = append(parts, v)
		}
	}
	return strings.Join(parts, "；")
}

func (a *API) listPblSessions(w http.ResponseWriter, r *http.Request) {
	atomID, ok := a.loadOwnedPblProject(w, r)
	if !ok {
		return
	}
	rows, err := a.d.Queries.ListPblSessions(r.Context(), atomID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	out := make([]pblSessionDTO, 0, len(rows))
	for _, s := range rows {
		out = append(out, toPblSessionDTO(s))
	}
	httpx.WriteJSON(w, http.StatusOK, out)
}

type pblThreadMessageDTO struct {
	Seq       int32           `json:"seq"`
	Role      string          `json:"role"`
	Content   string          `json:"content"`
	Payload   json.RawMessage `json:"payload,omitempty"`
	CreatedAt string          `json:"createdAt"`
}

// getPblThread returns one thread: the project's main thread by default, or a
// session's own when ?session= is given.
func (a *API) getPblThread(w http.ResponseWriter, r *http.Request) {
	atomID, ok := a.loadOwnedPblProject(w, r)
	if !ok {
		return
	}
	var rows []sqlc.AtomMessage
	var err error
	if raw := strings.TrimSpace(r.URL.Query().Get("session")); raw != "" {
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
		rows, err = a.d.Queries.ListPblSessionMessages(r.Context(), sqlc.ListPblSessionMessagesParams{
			AtomID: atomID, SessionID: pgtype.UUID{Bytes: sid, Valid: true},
		})
	} else {
		rows, err = a.d.Queries.ListPblMainThread(r.Context(), atomID)
	}
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	out := make([]pblThreadMessageDTO, 0, len(rows))
	for _, m := range rows {
		out = append(out, pblThreadMessageDTO{
			Seq: m.Seq, Role: m.Role, Content: m.Content,
			// json.RawMessage, not []byte — encoding/json base64s a []byte with
			// no error, and the failure surfaces in a client that cannot parse
			// its own payload. Same trap as liteMessageDTO.
			Payload:   json.RawMessage(m.Payload),
			CreatedAt: m.CreatedAt.Format(time.RFC3339),
		})
	}
	httpx.WriteJSON(w, http.StatusOK, out)
}
