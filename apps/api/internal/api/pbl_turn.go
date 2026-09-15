package api

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"mindimprint/api/internal/gateway"
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
	// 这一轮印记递了一件工具。前端拿到就去刷新工具列表。
	Tool   string  `json:"tool,omitempty"`
	ToolID *string `json:"toolId,omitempty"`
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
		httpx.WriteError(w, r, errBadJSON(err))
		return
	}
	// 空文本 = 她没说话，是刚发生了一件事（用完一件工具）要印记接一句。
	// 允许，但只在对话里已经有东西的时候——对着一个空房间凭空说一句，是印记
	// 在自言自语。
	studentText := strings.TrimSpace(req.Text)

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
	// 这一轮是不是"开场那一轮"：进来的时候这条线上还一句话都没有。
	// 下面拿到锁之后要用它再确认一次（见 openingRaced）。
	opening := len(in.Recent) == 0
	if studentText != "" {
		in.Recent = append(in.Recent, pbl.Turn{Role: "student", Content: studentText})
	} else if len(in.Recent) > 0 {
		// 🚨 她没打字，是刚做完一件工具回来。不说清楚"刚发生了什么"，这一轮的
		// 上文就以印记自己的话结尾，模型会把那句话原样再说一遍，并且把她刚做完
		// 的工具再递一次（2026-09-02 线上实测）。
		in.JustHappened = a.lastPblToolEvent(r, atomID)
	}
	// 🚨 她连着答不上来的次数，服务端数，每一轮都算。
	//
	// 必须放在把她这一句 append 进去之后：她刚打的那句「我不知道」正是要数进去
	// 的那一句。见 pbl/stuck.go —— 这是【怎么问】那条禁令唯一的出口，而出口要
	// 能在代码里验，不能是 prompt 里的一句请求。
	in.Stuck = pbl.StuckRun(in.Recent)
	in.AskedForHelp = pbl.AskedForHelp(in.Recent)

	if studentText == "" && len(in.Recent) == 0 && !scope.Valid {
		// 支线里允许空文本：印记要为这条支线开个头，而它的上文来自主线。
		httpx.WriteError(w, r, httpx.ErrBadRequest("empty_turn", "请输入内容", nil))
		return
	}

	resolved, rok := a.route(r.Context(), gateway.ClassDialogue)
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
		// 🚨 把网关真正说了什么带给她。gateway 的 error 是按"给客户端看也安全"
		// 设计的（密钥在请求头里，上游响应体早就丢掉了），所以可以原样显示。
		// 原来传的是 "model_unavailable" 这种机器码，等于什么都没说——
		// 2026-09-02 那五个 503 就是这样被藏了一下午。
		httpx.WriteError(w, r, httpx.ErrAIDialogueFailed(cerr.Error()))
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
	// 🚨 开场那一轮只准落库一次。
	//
	// 前端在"线程是空的"时会把她写的那句话当第一轮发出去。可这一轮要等模型，
	// 好几十秒；这期间她刷新一下页面、或者 StrictMode 把挂载跑两遍，新的那次
	// 看到的线程**仍然是空的**（第一轮还没提交），于是又发一遍。她就会在屏幕
	// 上看见自己那句话出现两遍、三遍，每遍下面跟着一段不一样的回话。
	//
	// 客户端的闸拦不住这个——跨页面刷新的两次请求互相看不见。只有在锁里再看
	// 一眼才作数：拿到锁之后线程已经不空了，说明别人先落库了，这一轮就丢掉。
	// 模型的钱已经花了（也照常记账），但不能让她看见重复的自己。
	if opening {
		raced, rerr := qtx.CountPblThreadMessages(r.Context(), sqlc.CountPblThreadMessagesParams{
			AtomID: atomID, SessionID: scope,
		})
		if rerr != nil {
			httpx.WriteError(w, r, rerr)
			return
		}
		if raced > 0 {
			_ = tx.Rollback(r.Context())
			httpx.WriteJSON(w, http.StatusOK, pblTurnDTO{Reply: ""})
			return
		}
	}
	next, err := qtx.NextAtomMessageSeq(r.Context(), atomID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	// studentText 为空 = 她没说话，是刚发生了一件事（用完一件工具、收起一层）
	// 要印记接一句。这时候只写印记那条，不要凭空造一条"她说的话"。
	aiSeq := next
	if studentText != "" {
		if _, err := qtx.AppendPblSessionMessage(r.Context(), sqlc.AppendPblSessionMessageParams{
			AtomID: atomID, Seq: next, Role: "student", Content: studentText, SessionID: scope,
		}); err != nil {
			httpx.WriteError(w, r, err)
			return
		}
		aiSeq = next + 1
	}
	if _, err := qtx.AppendPblSessionMessage(r.Context(), sqlc.AppendPblSessionMessageParams{
		AtomID: atomID, Seq: aiSeq, Role: "ai", Content: out.Reply,
		Payload: payload, SessionID: scope,
	}); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if err := tx.Commit(r.Context()); err != nil {
		httpx.WriteError(w, r, err)
		return
	}

	// 印记 offering a tool.
	//
	// 落在对话之外（pbl_tool_instance），不在消息里，因为一件工具有自己的一生：
	// 递出 → 她打开或者不用 → 有结果。塞进消息的 payload 里，"她后来到底用了
	// 没有"就无处可存。
	//
	// 🚨 递失败不让这一轮失败。她该看见的是印记刚说的话；一件没递成的工具，
	// 下一轮还可以再递。
	dto := pblTurnDTO{Reply: out.Reply, Hook: out.Hook, HookKind: out.HookKind}

	// 印记这一轮做出来的东西：一份计划、一个要她拿主意的选择、一份交给她审的
	// 成果、某一步的分工、一份结构（见 pbl_produce.go）。
	//
	// 🚨 和递工具一样，落在这一轮提交之后，失败也不让这一轮失败：她该看见的
	// 回话已经写进去了，产出没落上是我们的问题，不该把她那一轮也拖没。下一轮
	// 印记还可以再做一次。
	if out.Produce != nil {
		if perr := a.applyPblProduce(r.Context(), atomID, scope, out.Produce); perr != nil {
			slog.Warn("pbl turn: 印记 made something we could not record",
				"err", perr, "atom_id", atomID, "kind", out.Produce.Kind,
				"request_id", httpx.RequestIDFromContext(r.Context()))
		}
	}

	// 🚨 一件点开是空的工具，比不递这件工具糟得多——见 pbl_tool_gate.go。
	// 这一句必须排在 applyPblProduce 之后：印记这一轮做出来的东西已经落库了，
	// 所以这里问的是"她现在点进去有没有东西"，而不是"模型说它做了没有"。
	// 🚨 同一件工具在她屏幕上只能有一张卡。2026-09-03 线上实测：并排两张
	// 「头脑风暴」，理由各写各的，她根本不知道该点哪张。
	if out.Tool != "" && a.pblToolAlreadyOnHerScreen(r, atomID, out.Tool) {
		slog.Info("pbl turn: 印记 re-offered a tool already on her screen; dropping it",
			"atom_id", atomID, "tool", out.Tool,
			"request_id", httpx.RequestIDFromContext(r.Context()))
		out.Tool, out.ToolReason = "", ""
	}

	if out.Tool != "" && !a.pblToolHasContent(r.Context(), atomID, out.Tool) {
		slog.Warn("pbl turn: 印记 offered a tool whose surface would be blank; dropping it",
			"atom_id", atomID, "tool", out.Tool, "needs", pbl.ToolNeeds(out.Tool),
			"request_id", httpx.RequestIDFromContext(r.Context()))
		// 🚨 撤掉之后必须有人知道。
		//
		// 撤掉本身是对的，但上一版到此为止：印记刚在回话里说「审核助手我给你
		// 了」，那句话已经落库了，而卡不会出现。她读到的和她看到的对不上，于是
		// 她去找；印记下一轮的上文里什么都没变，于是它再说一遍。2026-09-04 的
		// 走查里，Marcus 在这个循环里耗掉约 35 步。
		//
		// 所以两边都告诉：线程里补一行说明（她马上看得见），并记一行给下一轮的
		// 上下文（印记下一轮知道该补什么）。
		a.recordPblToolDrop(r, atomID, scope, out.Tool)
		out.Tool, out.ToolReason = "", ""
	}

	if out.Tool != "" {
		tool, terr := a.d.Queries.SummonPblTool(r.Context(), sqlc.SummonPblToolParams{
			AtomID: atomID, SessionID: scope, Tool: out.Tool, Reason: out.ToolReason,
			Kind: pbl.ResolveToolKind(out.Tool, ""),
		})
		if terr != nil {
			slog.Warn("pbl turn: could not record the tool 印记 offered",
				"err", terr, "atom_id", atomID, "tool", out.Tool,
				"request_id", httpx.RequestIDFromContext(r.Context()))
		} else {
			id := tool.ID.String()
			dto.ToolID = &id
			dto.Tool = tool.Tool
			// 🚨 出门清单跟着这次「观察日记」落库。落不上不让这一轮失败——她该
			// 看见的回话已经写进去了，清单没挂上，下一轮印记还能再递一次。
			for i, m := range out.Mission {
				if _, merr := a.d.Queries.CreatePblMissionItem(r.Context(),
					sqlc.CreatePblMissionItemParams{
						ToolID: tool.ID, Prompt: m.Prompt,
						WantKind: pblWantKind(m.WantKind), Ordinal: int32(i),
					}); merr != nil {
					slog.Warn("pbl turn: could not record a mission item",
						"err", merr, "atom_id", atomID, "tool_id", tool.ID,
						"request_id", httpx.RequestIDFromContext(r.Context()))
					break
				}
			}
		}
	}
	httpx.WriteJSON(w, http.StatusOK, dto)
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
		Assigned: p.Assigned, AssignedBrief: derefOr(p.AssignedBrief, ""),
		SessionKind: sessionKind, SessionQuestion: sessionQuestion,
	}
	// 🚨 她在工具里做出来的东西，每一轮都要重新交给印记（见 pbl_refeed.go）。
	a.attachPblToolWork(r, atomID, &in)

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
		// 🚨 一条刚开的支线里一句话都没有。这时候只给印记一个孤零零的问题，
		// 它只能干巴巴地把那个问题再问一遍——而这条支线本来就是从她刚说的某
		// 句话上长出来的。
		//
		// 产品负责人 2026-09-02：「system should gives AI a context about this
		// branch first so that we can guide student」。所以把主线最后几轮一起
		// 带上，印记才说得出"你刚才说 X，我们单独看看这一点"。
		if err == nil && len(rows) == 0 {
			if parent, perr := a.d.Queries.ListPblMainThread(r.Context(), atomID); perr == nil {
				rows = parent
			}
		}
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
