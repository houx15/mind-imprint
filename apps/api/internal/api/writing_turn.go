package api

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

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

// buildWritingCoachProjection assembles the free-text state projection
// agent.ProposeProjectCoachReply's BuildProjectCoachContext renders verbatim
// under "项目当前状态（供你参考，别照搬复述）". Kept pure and separate from
// the handler so the "what does writing have instead of an article" assembly
// can be reasoned about (and tested) on its own, mirroring
// buildReadingRouteInput's split in reading_turn.go.
//
// target_words is reported as either a number or "还没定" (never omitted,
// never invented) — 2026-08-27 product ruling (W-R7): the coach must be able
// to SEE that length is still unsettled while she is in 构思 so she can raise
// it, but nothing here treats it as a precondition.
// draftBody 是成稿那一页上她此刻的正文。空串 = 她还没进成稿（或者那一页还没
// 落过字），这时以段落为准。
func buildWritingCoachProjection(wr sqlc.Writing, outline []sqlc.WritingOutline, snippets []sqlc.WritingSnippet, draftBody string) string {
	var b strings.Builder
	b.WriteString(writingTopicLine(wr, "题目/想法："))
	// 🚨 The language rule reaches the MAIN coach chat here. It was missing
	// entirely, which is why 印记 kept discussing an English piece as though
	// every artifact it produced should be Chinese. See writing_lang.go.
	b.WriteString(writingLangLine(wr))
	// 🚨 「约 %d 字」 was hard-coded here too. On an English piece this told the
	// coach a 500-word essay was 500 Chinese characters — see writing_lang.go
	// for the advice that came out the other end.
	if wr.TargetWords != nil {
		b.WriteString(writingLengthLine(wr, "目标篇幅"))
		b.WriteString(writingLengthGapBlock(wr, draftBody))
	} else {
		b.WriteString("目标篇幅：还没定\n")
	}
	if len(outline) > 0 {
		b.WriteString("已确定的提纲：\n")
		for _, o := range outline {
			indent := strings.Repeat("  ", int(o.Depth))
			fmt.Fprintf(&b, "%s- %s\n", indent, truncateRunes(o.Text, 120))
		}
	}
	// 🚨 **成稿那一页开了之后，她的正文就是那一页，不再是段落那几块。**
	//
	// 这是 2026-09-12 第二十七轮走查里她自己诊断出来的 —— 而且她是对的：
	//
	//	「印记反复让我改一句我正文里根本没有的话（**它在读下面那段只读的旧文本**），
	//	  导致对话死循环」
	//	「印记给的修改意见滞后了，我正文框里的第二段已经是改过时态的版本了，
	//	  它还让我改 go、see、finish eat」
	//
	// 她在成稿里改的是 writing_draft.body；段落那几块 writing_snippet 停在她
	// 进成稿之前的样子（那一栏在屏幕上本来就标着「只读、不会跟着上面变」）。
	// 而这个上文一直只喂 snippets —— 陪练读的**确实**是那份旧的。
	//
	// 前面几轮我一直在治这个的症状（提示词说「以这里为准」、幻引判据、
	// 把截断上限一路调高），都没治到这儿。根因就是这个函数从来没拿过 draft。
	//
	// 一旦有成稿，就**只**给成稿：两份她的文字同时摆在上文里，正是让它挑错
	// 一份的原因。段落那几块此刻是历史，不是她的正文。
	if body := strings.TrimSpace(draftBody); body != "" {
		b.WriteString("正文（**这是她此刻的正文，以这里为准**；" +
			"上面对话里你早先引过的句子她可能已经改掉了，不要照着那些再提一遍）：\n")
		b.WriteString(writingProjectionSnippet(body) + "\n")
		// 🚨 这三条在成稿这一支上同样要有。差点漏掉：这一支是后加的、而且
		// 提前 return，而那条测试当时用的是空成稿，绿着也没发现。
		b.WriteString(writingCoachGroundingRules)
		return b.String()
	}

	nonEmpty := 0
	for _, s := range snippets {
		if strings.TrimSpace(s.Text) != "" {
			nonEmpty++
		}
	}
	if nonEmpty > 0 {
		// 🚨 **说清这几段是「现在这一版」。**
		//
		// 上文里同时摆着两样东西：她此刻的正文，和最近十二轮对话 —— 而那十二轮
		// 里有印记自己说过的话，引着她**当时**写的句子。模型会顺着自己上一轮
		// 接着说，于是一轮一轮重复一个她早就改掉的毛病。
		//
		// 2026-09-12 第二十三轮，她连着三步在说这件事：
		//
		//	「印记让我删的那句话在正文框里已经不存在了，它还在拿旧版本的问题指导我」
		//	「它一直说我正文里有 huge 和 50 kilogram 要我删，但我正文框里早就没有
		//	  这些词了。我不知道该听它的还是按我现在的正文来」
		//
		// 最后那半句是真正的代价：**她开始怀疑该信屏幕上的哪一个**。
		b.WriteString("已经写好的片段（**这是她此刻的正文，以这里为准**；" +
			"上面对话里你早先引过的句子她可能已经改掉了，不要照着那些再提一遍）：\n")
		// 🚨 **带上这一块的标题，别只给一个号。**
		//
		// 第三十六轮中文那一路：「印记说的『第3段最后那两句』跟我现在看到的
		// 第一段最后一句有点像，但不确定它到底在说哪一段，有点乱。」
		// 屏幕上每一块的抬头写的是**结构那一步的标题**，一个数字都没有；
		// 而这里只给了号。于是「第3段」在她那边没有任何落点，只能自己数 ——
		// 而空的块也占位置，数出来常常对不上。
		//
		// 两头一起改：屏幕上把号摆出来（SnippetsStage），这里把标题给它。
		// 有标题就能说「『各地都能开设很好的学校』那一块」，比数字准得多。
		headingOf := map[string]string{}
		for _, o := range outline {
			headingOf[o.ID.String()] = strings.TrimSpace(o.Text)
		}
		for _, s := range snippets {
			text := strings.TrimSpace(s.Text)
			if text == "" {
				continue
			}
			label := ""
			if s.OutlineID.Valid {
				if h := headingOf[uuid.UUID(s.OutlineID.Bytes).String()]; h != "" {
					label = "「" + h + "」"
				}
			}
			fmt.Fprintf(&b, "  [第%d块]%s %s\n", s.Position+1, label, writingProjectionSnippet(text))
		}
		b.WriteString(writingCoachGroundingRules)
	}
	return b.String()
}

// writingCoachGroundingRules 是**她已经写出东西之后**，才加进上文的两条。
//
// 只在有片段时加：一张白纸上没有句子可引，也没有「她已经写过了」可言。
//
// # 一 · 说她哪里不行，就得指着那一句说
//
// 结构化的那条路（`请印记看看这一段`）早就守着这条：每条意见的 quote 必须
// 逐字出现在她写的东西里，对不上的整条丢掉（validateCommentPoints）。
// **对话这条路一直没有任何约束** —— 于是 2026-09-12 第二十轮走查里，
// 十来条卡壳说的都是同一件事：
//
//	「它不告诉我具体哪一句要改、怎么改才叫立起来」
//	「它说缺少权衡和限定、像绝对断言，但没告诉我具体哪句要改」
//	「印记说『判断没有立起来』，但我开头和结尾都写了啊，不太懂它要我改哪里」
//
// 一句「你的判断没立起来」，她无从下手，也无从反驳 —— 她甚至没法确认
// 印记读的是不是她这一版。
//
// 🚨 这一条**没法在代码里验**（自由对话没有可校验的输出类型），所以它只是
// 一条希望，不是保证 —— 见 [[prompt-output-must-be-verifiable-2026-09-03]]。
// 真正的保证在那条结构化的路上；这里能做的是把她的原文摆在上文里
// （上面那几段就是），让「引一句」成为最省力的选择。
//
// # 二 · 她此刻在写，不在现场
//
// 走查里印记连着几轮让她「去查一下成本」「去问问打饭阿姨」。她的原话：
//
//	「让我去查成本或者问阿姨，但我现在坐在电脑前根本没法去问，只能自己编一个」
//
// **让她去编，是这个产品最不该做的事。** 她手上有的是她见过的、记得的东西；
// 要她去取一件此刻取不到的材料，只会把她推向编造。
//
// # 三 · 指出毛病之后，得给一个动作
//
// 第二十二轮走查里她连着两步在烦这个：
//
//	「它一直让我自己读、自己想，不直接告诉我怎么改，有点烦」
//	「它一直问我觉得是重复还是呼应，又不直接告诉我怎么改，烦死了」
//	「它说结尾只在重复开头，但没告诉我结尾该怎么写才算不重复」
//
// 🚨 这一条要小心读：**她想要的不是答案，而结论也不是「那就把答案给她」。**
// 铁律①在这儿不让步。真正缺的是 story-coach 给那种回复起的名字
// —— "Diagnostic Without Return"：一轮以「你这里不对，你觉得呢」结束，
// 把她的下一步收走了。
//
// 结构化那条路早就守着这条：每条 issue 必须带一句祈使的 `Action`，空的整条
// 丢掉（CommentPoint.Action，writing_comment.go）。对话这条路没有。
// 所以这里补的是同一件事：**给动作，不给那句话**。
// 「把第三句挪到第一句前面」是动作；替她写出那一句，就是替她写作文。
const writingCoachGroundingRules = `
【说她哪里不行的时候】
先把她原文里那一句**逐字引出来**（或者说清是第几段的哪一句），再说它怎么了。
「你的判断没立起来」这种话她没法下手，也没法确认你读的是不是她这一版 ——
她刚刚才改过。

【别让她去取她此刻取不到的东西】
她正坐在电脑前写这一篇，去不了食堂，问不到打饭阿姨，也查不了采购成本。
要材料就问她**见过什么、记得什么**。要她去取一件现在取不到的东西，
她只会编一个 —— 那是这里最不该发生的事。

【指出毛病之后，给一个她现在就能做的动作】
说完「这里怎么了」，用一句祈使收尾，落在她已经写下的字上：
删掉哪个词、把哪句挪到哪句前面、把哪个形容词换成一个具体的数。
不要只留下一个问题让她自己琢磨 —— 一轮里可以有一个问题，但不能**只有**问题。

🚨 给的是**动作**，不是那句话本身。「把第三句挪到第一句前面」是动作；
替她写出那一句，就是替她写作文。
`

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

// writingProjectionSnippet 渲染一段正文，**并且在真的截断时说出来**。
//
// 🚨 截断要说出来，不能只留一个「…」。默不作声地切一刀，下游那个读的人
// （这里是模型）会把「被切掉」当成「她没写」，然后去要一件她已经写过的东西。
// 我自己在走查那只眼睛上犯过一模一样的错：`f.value.slice(0, 120)` 不声不响
// 切一刀，学生于是连着五六步在「补全被截掉的那一段」，而她一个字都没丢。
// 同一个毛病，这次在产品里。
func writingProjectionSnippet(text string) string {
	r := []rune(text)
	if len(r) <= writingProjectionSnippetRunes {
		return text
	}
	return fmt.Sprintf("%s……（这一段一共 %d 字，上面只给了前 %d 字，后面的她已经写了，只是没放进来——别据此说她少写了什么）",
		string(r[:writingProjectionSnippetRunes]), len(r), writingProjectionSnippetRunes)
}

// buildWritingTurnHistory windows the raw transcript to writingTurnsWindow
// (never the whole thread — see the constant's comment) and converts it to
// agent.ChatTurn, then appends the CURRENT student turn — mirroring how
// coach.go's postCoachSubagentTurn builds ProposeProjectCoachReply's history
// (load prior, then append the turn under way, so the producer's context
// always ends with what she just said regardless of whether the persist
// below succeeds).
//
// role='system' rows (writing_stage.go's stage-transition trace, e.g. "stage:
// ideate → outline") are deliberately excluded from the conversation itself —
// they are a structural record, not something either side "said", and
// BuildProjectCoachContext has no third bucket for them; folding one in under
// "学生：" would misattribute it to her. They still count against the window
// (it slices the raw tail first, then filters), so a writing thread with many
// stage skips cannot inflate the prompt past the same bound reading enjoys.
func buildWritingTurnHistory(msgs []sqlc.AtomMessage, studentText string) []agent.ChatTurn {
	tail := msgs
	if len(tail) > writingTurnsWindow {
		tail = tail[len(tail)-writingTurnsWindow:]
	}
	history := make([]agent.ChatTurn, 0, len(tail)+1)
	for _, m := range tail {
		switch m.Role {
		case "student":
			history = append(history, agent.ChatTurn{Role: "user", Content: m.Content})
		case "ai":
			history = append(history, agent.ChatTurn{Role: "assistant", Content: m.Content})
		}
	}
	return append(history, agent.ChatTurn{Role: "user", Content: studentText})
}

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
	// 这是 writing_ghostquote.go 那条纪律的落点：结构化那条路（请印记看看这一段）
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
				Content: "【你引的那句不在她正文里】「" + talkOnly + "」是她在**对话里**跟你说的，" +
					"她的作品里没有这句。她会照着你的话去正文里找，找不到就会以为自己弄丢了什么。" +
					"请重答一遍：要么改成引【已经写好的片段】里逐字有的句子，" +
					"要么把出处说出来（「你刚才跟我说的……」），别让它读起来像她写过的。",
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
				Content: "【刚才那一版你引错了】你引的那句「" + ghost + "」" +
					"在她现在的正文里一个字都找不到 —— 多半是你在照着这段对话里更早的" +
					"版本说话，而她已经改过了。请重答一遍：只引【已经写好的片段】里" +
					"逐字有的句子，或者干脆不引、直接说第几段的第几句。",
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
