package api

import (
	"context"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"mindimprint/api/internal/agent"
	"mindimprint/api/internal/gateway"
	"mindimprint/api/internal/httpx"
	"mindimprint/api/internal/store/sqlc"
)

// writing_turn.go — 陪练一轮 for the WRITING room (Task 4).
//
// Step 1 finding, recorded here as the brief asked: agent.RouteReading (the
// producer postLiteReadingTurn reuses, reading_turn.go) is shaped around an
// Article plus a reading-card catalog — neither of which writing has. What
// writing has instead — the idea, the outline she has confirmed, the
// fragments she has already written, and which of the four stages
// (构思/大纲/段落/成稿) she is in — has no dedicated router of its own, and
// none is needed: §6.2.2 of the spec asks for "talk first" — an always-reply
// conversational coach, not a card-routing decision. That producer already
// exists in internal/agent, already proven in production: pro's own
// project-wide coach (coach.go's postCoach / postCoachSubagentTurn,
// card_reflect.go) is built on exactly agent.ProposeProjectCoachReply — a
// tool-less, room-aware, always-reply producer that takes (history, a
// free-text state projection, an active-surface label) and returns one
// restrained reply. That is precisely the three things writing can supply:
// the windowed transcript, a projection of title/target-words/outline/
// snippets, and the current stage as the surface label. internal/agent
// required ZERO changes to serve this room — the same reuse thesis as
// reading, on a different existing producer because the shape of what
// writing has is different from what reading has.
//
// Card summoning is deliberately OUT of this turn: Task 1.5 left
// liteSummonCard reading-only ("a writing room summons over an outline or
// snippet — a different shape entirely... Writing gets its own summon
// endpoint"), and no such endpoint exists yet anywhere in this phase's plan.
// So decision/card/nudge/hintCardId below hold liteTurnDTO's shape — the
// exact shape the frontend already speaks for reading — at their zero value
// (decision "respond", card nil) until that door is built.
//
// The three jobs this handler does, mirroring postLiteReadingTurn exactly:
//  1. assemble — agent.ChatTurn history (windowed) + a projection built from
//     the lite writing tables
//  2. persist  — student turn + AI turn in ONE transaction, consecutive seq
//  3. meter    — one llm_call row (surface='lite', purpose='writing_turn')

// writingTurnsWindow bounds how many atom_message rows feed the coach's
// context. A SEPARATE constant from reading_turn.go's recentTurnsWindow,
// deliberately not reused, even though both currently read 12: they guard two
// different producers whose prompts are built out of different material (an
// article that must keep dominating the prompt, versus a stage/outline/
// snippet projection that already carries its own separate size discipline —
// see writingProjectionSnippetRunes). Collapsing them into
// one shared constant would couple two rooms' prompt budgets to a single
// number for no reason other than that the numbers happen to match today.
// Load-bearing for the same reason as reading's: lite has NO compaction
// layer, so this window is the only thing bounding prompt growth on a long
// writing thread.
const writingTurnsWindow = 12

// writingStageLabels names each stage for the coach's "you are here" framing
// (mirrors coachSurfaceLabel's pro-side room names, projectcoach.go, but
// keyed on writing.stage rather than a room scope).
//
// Two entries here are READ-ONLY mappings for values this API no longer
// accepts (validWritingStages, writing_stage.go): 'finished' cannot reach
// this handler at all (loadOwnedWritingAtom's finished-write gate refuses the
// POST first), and 'ideate' was retired when the map collapsed to three steps
// — 0100 migrated the rows, but an un-migrated replica or an old fixture
// could still hand one over, and a label lookup must degrade to the right
// step rather than to the generic "写作".
var writingStageLabels = map[string]string{
	"ideate":   "结构",
	"outline":  "结构",
	"snippets": "段落",
	"draft":    "成稿",
	"finished": "成稿",
}

func writingStageLabel(stage string) string {
	if s, ok := writingStageLabels[stage]; ok {
		return s
	}
	return "写作"
}

// liteWritingTurnReq is postLiteWritingTurn's request body. Writing has no
// analogue of reading's focusedSpans (there is no article to select spans
// in), so this is deliberately smaller than reading_turn.go's liteTurnReq.
type liteWritingTurnReq struct {
	Text string `json:"text"`
	// Board 说的是「这一条消息是摆完一块板产生的」。闭表，认不出来的值当没给
	// 处理——她摆的东西是真的，少一句上下文不该把这一轮弄丢。
	// 见 writing_board.go。
	Board string `json:"board"`
}

// writingProjectionSnippetRunes 是一段正文喂进陪练上下文时的上限。
//
// 🚨 **原来是 300，而这个数字是线上量出来错的。** 2026-09-11 的走查里，
// 同一个学生在三步上报了同一件事：
//
//	[35]「印记说缺例子，但我框里明明已经写了张伟那句，感觉它没看到。」
//	[38]「正文框0里明明已经有张伟和王浩的例子了，但印记说缺。」
//	[43]「框0里明明已经有张伟的例子了，印记还让我加。」
//
// 她是对的，而且三次都对：那个例子在第 300 个字之后，陪练**根本没拿到**。
// 于是它诚实地按它看见的那半段作了判断，说缺例子；她照做又补一遍，
// 补出来的还是在 300 之后。这是个会自我延续的坑。
//
// 🚨 **这个数被线上打回来两次了。** 第一版 300，第二版 1200，都是按
// 「一篇八百字、一段总不会超过这个数」估的 —— 而学生真的会写。
// 2026-09-12 第二十五轮，英文那个写到 2269 字，于是同一件事第二次发生：
//
//	「印记非说正文里没有我加的句子，但我看框[0]里面明明就在最后一句写着呢」
//	「印记说正文里没有我的解释，但框0的文本里明明已经写了 By "more" I mean…」
//	「不知道是它读错了还是页面没同步，或者字数超了被截断」  ← 她自己猜对了
//
// 新加的句子几乎总在段尾，而截断正好从段尾切 —— 所以这个坑专挑
// 「她刚照着意见改完」的那一刻发作，最伤的一刻。
//
// 四千。这一次不按「她现在写多少」估，按**一段正文根本到不了的高度**定。
// 走查那只眼睛上早就学过这一课（那边放到 8000），产品这边却还在跟着她的
// 字数往上挪 —— 每次她写得更多，这个数就又小了一次。
const writingProjectionSnippetRunes = 4000

// postLiteWritingTurn drives one coach turn for the writing room.
func (a *API) postLiteWritingTurn(w http.ResponseWriter, r *http.Request) {
	at, ok := a.loadOwnedWritingAtom(w, r)
	if !ok {
		return
	}
	u, _ := UserFromContext(r.Context())
	// Entitlement is checked BEFORE the model call — this is a token-spending
	// endpoint, exactly where the HasEntitlement seam belongs (mirrors
	// postLiteReadingTurn).
	entitled, err := HasEntitlement(r.Context(), u)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if !entitled {
		httpx.WriteError(w, r, httpx.ErrNotEntitled())
		return
	}

	var req liteWritingTurnReq
	if err := decodeJSON(r, &req); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	studentText := strings.TrimSpace(req.Text)
	if studentText == "" {
		httpx.WriteError(w, r, httpx.ErrBadRequest("missing_text", "先说点什么，我在听。", nil))
		return
	}

	// Run to completion even if the student navigates away mid-reply — same
	// reasoning and same 150s cap as postLiteReadingTurn / coach.go's turnCtx:
	// a synchronous POST is cancelled by net/http the instant the browser
	// disconnects, and a refresh mid-call would otherwise abort the model
	// call, the metering row AND the transaction below — money spent, nothing
	// recorded, and she loses the answer she already paid for.
	turnCtx, cancelTurn := context.WithTimeout(context.WithoutCancel(r.Context()), 150*time.Second)
	defer cancelTurn()

	// 1. 装配 — everything the coach needs to see, entirely from the lite
	// writing tables: her own writing row, the outline, the fragments already
	// written, and the windowed transcript.
	wr, err := a.d.Queries.GetWriting(turnCtx, at.ID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	outline, err := a.d.Queries.ListWritingOutline(turnCtx, at.ID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	snippets, err := a.d.Queries.ListWritingSnippets(turnCtx, at.ID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	msgs, err := a.d.Queries.ListAtomMessages(turnCtx, at.ID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	// 成稿那一页的正文。没有这一行，陪练在成稿里读的是段落那份旧的 ——
	// 见 buildWritingCoachProjection 里那段。没有这一行就当空串（她还没进成稿）。
	var draftBody string
	if dr, derr := a.d.Queries.GetWritingDraft(turnCtx, at.ID); derr == nil {
		draftBody = dr.Body
	}
	history := buildWritingTurnHistory(msgs, studentText)
	projection := buildWritingCoachProjection(wr, outline, snippets, draftBody)
	// 她刚摆完一块板 → 在上文里加一句说明，好让 印记 知道她交了作业，
	// 不是在闲聊。加在 projection 上而不是改那个 producer，因为它是 pro 和
	// lite 共用的（lite-must-not-break-pro）。见 writing_board.go。
	projection += writingBoardNote(req.Board)
	// 她请我们替她搜索或替她写 → 这一轮先说明再往下走。一次性，不做常驻。
	// 见 writing_refusal.go（同事 2026-09-20 的意见 8）。
	if writingAsksUsToDoIt(studentText) {
		projection += writingRefusalBlock
	}
	surfaceLabel := writingStageLabel(wr.Stage)

	resolved, rerr := a.routeE(turnCtx, gateway.ClassDialogue)
	if rerr != nil {
		// No call was ever attempted — nothing to meter, same as coach.go's
		// ChatResolver failure branch — but still surfaced as the honest
		// ai_dialogue_failed 502 rather than ErrInternal: from the student's
		// side this is indistinguishable from "the model didn't answer".
		slog.Warn("lite writing turn: resolve model failed", "err", rerr,
			"atom_id", at.ID, "request_id", httpx.RequestIDFromContext(r.Context()))
		httpx.WriteError(w, r, httpx.ErrAIDialogueFailed("model_unavailable"))
		return
	}

	// 2. 复用 AI 大脑，一行没改 — agent.ProposeProjectCoachReply, the same
	// always-reply room-aware producer pro's own coach.go runs.
	out, usage, cerr := agent.ProposeProjectCoachReply(turnCtx, a.d.Provider, resolved, history, projection, surfaceLabel)

	// Meter BEFORE any bail: a call that reached a provider cost money
	// whatever happens to its reply, including an enforcement rejection.
	a.recordLiteLLMCall(turnCtx, u.ID, at.ID, "writing_turn", resolved, usage)

	// 🚨 引了一句她根本没写过的话 —— 查一下，查到就再要一次。
	//
	// 这是 writing_ghostquote.go 那条纪律的落点：结构化那条路（AI审阅这一段）
	// 从第一天起就验引文，对话这条路一直没验。提示词打过两次没压下去
	//（第二十三轮三条、第二十四轮八条），而她已经说出了代价：
	//「我不知道该听它的还是按我现在的正文来。」
	//
	// 一次重试，而且**查不过也照样把回复给她**：一句引错的话仍然带着有用的
	// 教学，扣下整轮反而让她白等一次。这跟解析失败不一样 —— 那种情况屏幕上
	// 什么都没有。
	if cerr == nil {
		// 判幻引要对着**陪练该读的那一份**来，否则它会跟着一起认旧文本。
		written := writingWrittenCorpus(snippets, draftBody)
		said := writingSaidCorpus(msgs)
		corpus := written + said

		// 🚨 第二种错法：这句话她**说过**，但没写进作品里。
		//
		// 判据分三种（见 writing_ghostquote.go 开头）：在正文里、只在对话里、
		// 哪儿都没有。只在对话里的那一种原来是放行的，于是印记可以指着一句她
		// 只在聊天里提过的话让她改，而她会去框里找 —— 找不到。
		// 第三十六、三十七两轮各撞一次：
		//	「印记引用的那句『看到什么就拿什么』在我现在的框[0]里根本找不到」
		//
		// 不禁止它引对话（「你刚才说那个男生一口没动红烧肉」是好教学），
		// 只要求它**说清这是她说的、不是她写的**。
		talkOnly := ""
		if firstGhostQuote(out.Body, corpus) == "" {
			talkOnly = firstGhostQuote(out.Body, written)
		}
		if talkOnly != "" {
			slog.Info("lite writing turn: reply quoted something she only said, retrying once",
				"atom_id", at.ID, "request_id", httpx.RequestIDFromContext(r.Context()),
				"talk_quote", talkOnly)
			retryHistory := append(append([]agent.ChatTurn{}, history...), agent.ChatTurn{
				Role: "user",
				Content: "【引文来源需要纠正】「" + talkOnly + "」出自学生的对话，不在当前正文中。" +
					"请重新回复：若评价正文，逐字引用【已经写好的片段】中的句子；" +
					"若讨论这句对话，明确说明「你刚才提到……」，不将其描述为正文内容。",
			})
			out2, usage2, cerr2 := agent.ProposeProjectCoachReply(turnCtx, a.d.Provider, resolved, retryHistory, projection, surfaceLabel)
			// 打到 provider 就已经花钱了，无论这一版用不用 —— 先记账。
			a.recordLiteLLMCall(turnCtx, u.ID, at.ID, "writing_turn", resolved, usage2)
			if cerr2 == nil && strings.TrimSpace(out2.Body) != "" {
				out = out2
			}
		}

		// 🚨 **一轮最多纠正一次。**
		// 幻引那一条和上面这一条是两种错法，但它们不该叠加成两次额外调用：
		// 一轮本来就要等一个模型，再叠两次就顶着 150 秒的请求上限了，
		// 而超上限的样子就是她屏幕上那句 model_unavailable
		//（第三十八轮出现过两次）。已经纠正过一次就到此为止。
		if ghost := firstGhostQuote(out.Body, corpus); ghost != "" && talkOnly == "" {
			slog.Warn("lite writing turn: reply quoted text she never wrote; retrying once",
				"atom_id", at.ID, "request_id", httpx.RequestIDFromContext(r.Context()),
				"ghost_quote", ghost)
			retryHistory := append(append([]agent.ChatTurn{}, history...), agent.ChatTurn{
				Role: "user",
				Content: "【引文需要纠正】上一轮引用的「" + ghost + "」不在当前提供的原文中。" +
					"请依据当前稿件重新回复：引文须与【已经写好的片段】逐字一致；" +
					"也可以明确指出段落和句子位置，不生成或引用旧版本的句子。",
			})
			out2, usage2, cerr2 := agent.ProposeProjectCoachReply(turnCtx, a.d.Provider, resolved, retryHistory, projection, surfaceLabel)
			a.recordLiteLLMCall(turnCtx, u.ID, at.ID, "writing_turn", resolved, usage2)
			if cerr2 == nil && strings.TrimSpace(out2.Body) != "" {
				if again := firstGhostQuote(out2.Body, corpus); again == "" {
					out = out2
				} else {
					// 两次都引错。用第二版（它至少刚被提醒过），并记下来 ——
					// 这条日志是「这个毛病还在不在」的唯一证据。
					slog.Warn("lite writing turn: retry quoted a ghost too",
						"atom_id", at.ID, "ghost_quote", again)
					out = out2
				}
			}
		}
	}

	// USER RULE: an AI-dialogue failure is surfaced as a real 502, never
	// masked by a canned stand-in sentence. Unlike agent.RouteReading (see
	// postLiteReadingTurn's long comment on this exact point),
	// ProposeProjectCoachReply does NOT swallow its own failures: a provider
	// error, an empty completion, or an enforcement rejection
	// (enforcement.ValidateOutput / BannedPhrasing) all come back as a
	// genuine non-nil `cerr` — there is no raw-decision-vs-gated-decision
	// distinction to worry about here, because this producer proposes no
	// gate of its own. So `cerr != nil` alone is already the honest signal;
	// the empty-body check below is kept as a second line of defence, not
	// because this producer is known to need it.
	reply := strings.TrimSpace(out.Body)
	if cerr != nil || reply == "" {
		slog.Warn("lite writing turn: model turn failed; surfacing to student",
			"err", cerr, "atom_id", at.ID, "request_id", httpx.RequestIDFromContext(r.Context()))
		httpx.WriteError(w, r, httpx.ErrAIDialogueFailed("model_unavailable"))
		return
	}

	// 3. 落库 — the student turn and the AI turn in ONE transaction, with
	// consecutive seq allocated inside it via NextAtomMessageSeq (never
	// hardcoded), exactly mirroring postLiteReadingTurn and
	// setWritingStage's atomic stage+trace write.
	tx, err := a.d.Pool.Begin(turnCtx)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	defer func() { _ = tx.Rollback(turnCtx) }()
	qtx := a.d.Queries.WithTx(tx)

	// 串行化这颗原子的 seq 分配。NextAtomMessageSeq 是先读后插，
	// 并发下两笔事务会读到同一个 MAX——唯一索引保住的是数据，代价是
	// 其中一轮直接失败，而那一轮的模型钱已经花掉了。见 queries/atom.sql。
	if _, err := qtx.LockAtom(turnCtx, at.ID); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	next, err := qtx.NextAtomMessageSeq(turnCtx, at.ID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if _, err := qtx.AppendAtomMessage(turnCtx, sqlc.AppendAtomMessageParams{
		AtomID: at.ID, Seq: next, Role: "student", Content: studentText,
	}); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if _, err := qtx.AppendAtomMessage(turnCtx, sqlc.AppendAtomMessageParams{
		AtomID: at.ID, Seq: next + 1, Role: "ai", Content: reply,
	}); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if err := tx.Commit(turnCtx); err != nil {
		httpx.WriteError(w, r, err)
		return
	}

	// Writing mounts no card-summon door in this task (see the file comment):
	// decision/card/nudge/hintCardId hold liteTurnDTO's shape at their zero
	// value until that endpoint exists.
	httpx.WriteJSON(w, http.StatusOK, liteTurnDTO{
		Reply: reply, Decision: "respond", Card: nil, Nudge: "", HintCardID: nil,
	})
}
