package api

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"mindimprint/api/internal/httpx"
	"mindimprint/api/internal/pbl"
	"mindimprint/api/internal/store/sqlc"
)

// pbl_turn.go — 一轮对话.
//
// The turn is where every piece built so far meets: the thread scoping, the
// context rules (§10.3), the router, and the metering. It writes the student's
// message and 印记's reply in ONE transaction — a turn that persisted half of
// itself is a thread that no longer makes sense.

type pblTurnDTO struct {
	Reply    string `json:"reply"`
	Hook     string `json:"hook,omitempty"`
	HookKind string `json:"hookKind,omitempty"`
}

// pblHookPayload rides on the AI message so the hook survives a refresh.
// Without it a hook would live exactly as long as the tab.
type pblHookPayload struct {
	Kind     string `json:"kind"`
	Hook     string `json:"hook"`
	HookKind string `json:"hookKind"`
}

func (a *API) postPblTurn(w http.ResponseWriter, r *http.Request) {
	u, _ := UserFromContext(r.Context())
	atomID, ok := a.loadOwnedPblProject(w, r)
	if !ok {
		return
	}
	entitled, err := HasEntitlement(r.Context(), u)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if !entitled {
		httpx.WriteError(w, r, httpx.ErrNotEntitled())
		return
	}

	var req struct {
		Text      string `json:"text"`
		SessionID string `json:"sessionId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteError(w, r, httpx.ErrBadRequest("bad_json", "请求格式不对", nil))
		return
	}
	studentText := strings.TrimSpace(req.Text)
	if studentText == "" {
		httpx.WriteError(w, r, httpx.ErrBadRequest("empty_turn", "说点什么再发", nil))
		return
	}

	// Which thread is this? NULL = the project's main thread.
	var scope pgtype.UUID
	var sessionKind, sessionQuestion string
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
		if s.ClosedAt.Valid {
			httpx.WriteError(w, r, httpx.ErrBadRequest("session_closed", "这一层已经收起来了", nil))
			return
		}
		scope = pgtype.UUID{Bytes: sid, Valid: true}
		sessionKind, sessionQuestion = s.Kind, s.Question
	}

	in, err := a.buildPblCoachInput(r, atomID, scope, sessionKind, sessionQuestion)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	in.Recent = append(in.Recent, pbl.Turn{Role: "student", Content: studentText})

	resolved, rok := a.resolveEval(r.Context())
	if !rok {
		httpx.WriteError(w, r, httpx.ErrAIDialogueFailed("model_unavailable"))
		return
	}
	out, usage, cerr := pbl.Coach(r.Context(), a.d.Provider, resolved, in)
	// Meter before any bail — a call that yielded nothing still cost money.
	a.recordLiteLLMCall(r.Context(), u.ID, atomID, "pbl_turn", resolved, usage)
	if cerr != nil {
		// Surface it. A canned sentence here would leave her talking to a dead
		// turn while the real failure stays invisible.
		slog.Warn("pbl turn: model turn failed; surfacing to student",
			"err", cerr, "atom_id", atomID, "request_id", httpx.RequestIDFromContext(r.Context()))
		httpx.WriteError(w, r, httpx.ErrAIDialogueFailed("model_unavailable"))
		return
	}

	var payload []byte
	if out.Hook != "" {
		payload, err = json.Marshal(pblHookPayload{
			Kind: "hook", Hook: out.Hook, HookKind: out.HookKind,
		})
		if err != nil {
			httpx.WriteError(w, r, err)
			return
		}
	}

	// Both messages in one transaction, with consecutive seq under the atom
	// lock — see queries/atom.sql · LockAtom.
	tx, err := a.d.Pool.Begin(r.Context())
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	defer func() { _ = tx.Rollback(r.Context()) }()
	qtx := a.d.Queries.WithTx(tx)

	if _, err := qtx.LockAtom(r.Context(), atomID); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	next, err := qtx.NextAtomMessageSeq(r.Context(), atomID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if _, err := qtx.AppendPblSessionMessage(r.Context(), sqlc.AppendPblSessionMessageParams{
		AtomID: atomID, Seq: next, Role: "student", Content: studentText, SessionID: scope,
	}); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if _, err := qtx.AppendPblSessionMessage(r.Context(), sqlc.AppendPblSessionMessageParams{
		AtomID: atomID, Seq: next + 1, Role: "ai", Content: out.Reply,
		Payload: payload, SessionID: scope,
	}); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if err := tx.Commit(r.Context()); err != nil {
		httpx.WriteError(w, r, err)
		return
	}

	httpx.WriteJSON(w, http.StatusOK, pblTurnDTO{
		Reply: out.Reply, Hook: out.Hook, HookKind: out.HookKind,
	})
}

// buildPblCoachInput assembles the turn's context per spec §10.3.
//
// Inside a session: that session's turns and its question. On the main thread:
// the main turns plus the CONCLUSIONS of closed sessions — never their working,
// which is what stops a deep dig from flooding the project.
func (a *API) buildPblCoachInput(r *http.Request, atomID uuid.UUID, scope pgtype.UUID, sessionKind, sessionQuestion string) (pbl.CoachInput, error) {
	p, err := a.d.Queries.GetPblProject(r.Context(), atomID)
	if err != nil {
		return pbl.CoachInput{}, err
	}
	in := pbl.CoachInput{
		Idea: p.Idea, Kind: p.Kind,
		SessionKind: sessionKind, SessionQuestion: sessionQuestion,
	}

	// The live plan, if she has approved one.
	if v, err := a.d.Queries.GetPblLivePlan(r.Context(), atomID); err == nil {
		if steps, serr := a.d.Queries.ListPblPlanSteps(r.Context(), v.ID); serr == nil {
			for _, s := range steps {
				in.Steps = append(in.Steps, s.Title+"（"+s.Status+"）")
			}
		}
	}

	var rows []sqlc.AtomMessage
	if scope.Valid {
		rows, err = a.d.Queries.ListPblSessionMessages(r.Context(), sqlc.ListPblSessionMessagesParams{
			AtomID: atomID, SessionID: scope,
		})
	} else {
		rows, err = a.d.Queries.ListPblMainThread(r.Context(), atomID)
		if err == nil {
			// Conclusions of the sessions that hang off THIS thread.
			wbs, werr := a.d.Queries.ListPblSessionWriteBacks(r.Context(), sqlc.ListPblSessionWriteBacksParams{
				AtomID: atomID, ParentID: pgtype.UUID{},
			})
			if werr == nil {
				for _, s := range wbs {
					if t := strings.TrimSpace(s.Takeaway); t != "" {
						in.WriteBacks = append(in.WriteBacks, t)
					}
				}
			}
		}
	}
	if err != nil {
		return pbl.CoachInput{}, err
	}
	for _, m := range rows {
		// System rows (step dividers, returned write-backs) are context the
		// model already has through Steps/WriteBacks; replaying them as turns
		// would double them in the prompt.
		if m.Role == "system" {
			continue
		}
		in.Recent = append(in.Recent, pbl.Turn{Role: m.Role, Content: m.Content})
	}
	return in, nil
}
