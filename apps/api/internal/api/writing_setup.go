package api

// writing_setup.go — 进门那一步：设定 + 开场。
//
// 2026-08-27 的走查发现两件事，这个文件是它们的答案：
//
//  1. **房间是哑的。** 学生打开一次写作，她那句开场白孤零零挂在对话栏里，
//     AI 一个字都不说。「没有被引导的感觉」就是这么来的——不是引导得不好，
//     是根本没有引导。postWritingOpening 让印记先开口。
//  2. **目标字数形同虚设。** PUT /target-words 一直是 200，存进去了，但界面
//     不给任何回应、之后也没有一个地方显示过它，于是从学生那一侧看就是
//     「点了没反应」。它现在改由 setup 一次定下，并在房间顶部常驻显示。
//
// 设定弹窗只有三样东西：语言、目标篇幅、以及一个「还想说点什么都行」的
// 自由输入框。**没有文体单选**——这是产品的明确要求：学生未必知道「文体」
// 是什么意思，与其让她在一个她读不懂的词上做选择，不如让她用自己的话再说
// 两句，由模型去判断这是议论还是记叙（writing_structure.go 的推荐环节）。
//
// 那段自由文字不进新列，而是原样作为一条 student 消息落进 atom_message：
// 它本来就是她自己的话，进了转录就自动成为开场、结构推荐、每一块引导问题
// 三条链路共同的材料，不需要任何一处专门去读一个新字段。

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"mindimprint/api/internal/gateway"
	"mindimprint/api/internal/httpx"
	"mindimprint/api/internal/store/sqlc"
)

// writingSetupNoteMaxRunes bounds the free-text box. Generous — she is being
// invited to talk — but bounded, because this text lands in every downstream
// prompt and lite has no compaction layer (writing_turn.go's own reasoning).
const writingSetupNoteMaxRunes = 2000

// setWritingSetup is PUT /api/v1/writings/{id}/setup. No model call, so no
// entitlement gate: this is settings, not spend. The opening coach line is a
// SEPARATE endpoint (postWritingOpening) so submitting the dialog closes it
// instantly instead of blocking on a model round-trip.
func (a *API) setWritingSetup(w http.ResponseWriter, r *http.Request) {
	at, ok := a.loadOwnedWritingAtom(w, r)
	if !ok {
		return
	}
	var req struct {
		Lang string `json:"lang"`
		// Pointer, not int: absent/null means "she chose not to set a
		// length", which is a real and permanently valid state (铁律② —
		// length is never a precondition for anything). A plain int would
		// make 0 indistinguishable from unset.
		TargetWords *int   `json:"targetWords"`
		Note        string `json:"note"`
	}
	if err := decodeJSON(r, &req); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	// An assigned writing's language and target are the teacher's (owner,
	// 2026-09-15): the body's lang and targetWords are ignored and the stored
	// values kept. The stamp and the note are saved as for her own writing.
	cur, err := a.d.Queries.GetWriting(r.Context(), at.ID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	lang, tw := cur.Lang, cur.TargetWords
	if !writingIsAssigned(cur) {
		if req.Lang != "zh" && req.Lang != "en" {
			httpx.WriteError(w, r, httpx.ErrBadRequest("invalid_lang", "语言只能是中文或英文。", nil))
			return
		}
		lang, tw = req.Lang, nil
		if req.TargetWords != nil {
			if *req.TargetWords < minTargetWords || *req.TargetWords > maxTargetWords {
				httpx.WriteError(w, r, httpx.ErrBadRequest("invalid_target_words", "目标字数需在 1 到 100000 之间。", nil))
				return
			}
			v := int32(*req.TargetWords)
			tw = &v
		}
	}
	note := strings.TrimSpace(req.Note)
	if len([]rune(note)) > writingSetupNoteMaxRunes {
		note = string([]rune(note)[:writingSetupNoteMaxRunes])
	}

	// The settings write and the note-as-message write go in ONE transaction.
	// Split, a failure between them leaves her having typed something that
	// vanished while the dialog reports success — and that text is often the
	// most considered thing she has said about the piece.
	tx, err := a.d.Pool.Begin(r.Context())
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	defer func() { _ = tx.Rollback(r.Context()) }()
	qtx := a.d.Queries.WithTx(tx)

	wr, err := qtx.SetWritingSetup(r.Context(), sqlc.SetWritingSetupParams{
		AtomID: at.ID, Lang: lang, TargetWords: tw,
	})
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if note != "" {
		// 串行化这颗原子的 seq 分配。NextAtomMessageSeq 是先读后插，
		// 并发下两笔事务会读到同一个 MAX——唯一索引保住的是数据，代价是
		// 其中一轮直接失败，而那一轮的模型钱已经花掉了。见 queries/atom.sql。
		if _, err := qtx.LockAtom(r.Context(), at.ID); err != nil {
			httpx.WriteError(w, r, err)
			return
		}
		seq, err := qtx.NextAtomMessageSeq(r.Context(), at.ID)
		if err != nil {
			httpx.WriteError(w, r, err)
			return
		}
		if _, err := qtx.AppendAtomMessage(r.Context(), sqlc.AppendAtomMessageParams{
			AtomID: at.ID, Seq: seq, Role: "student", Content: note,
		}); err != nil {
			httpx.WriteError(w, r, err)
			return
		}
	}
	if err := tx.Commit(r.Context()); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, writingDTOOf(wr, at.CreatedAt, at.LastActivityAt))
}

// writingOpeningSystem is the coach's opening line. It is the single most
// load-bearing prompt in the room: it sets whether the student feels
// accompanied or abandoned in her first three seconds.
//
// 铁律① is stated as a hard prohibition rather than left implicit, because
// "help me start this essay" is precisely the moment a model is most tempted
// to hand over a thesis. 铁律③ (one question at a time) is stated as a count,
// because "be concise" is not something a model reliably converts into "ask
// exactly one thing".
const writingOpeningSystem = `你是「印记」，一个陪中学生写作的伙伴。学生刚刚打开一次新的写作，下面是她自己写下的题目和她说过的话。

接下来你们要做的是**规划**：一起把这篇要说什么、按什么顺序说，逐步想清楚。她说的每一点都会长到右边那张图上。现在由你先开口。

你的开场要做到三件事，合起来不超过 120 个字：

1. 用一句话把她想写的东西说回给她，让她确认你听懂了。用她自己的说法，不要换成更"高级"的表述。
2. 用一句话告诉她这一步要干什么——先一起把要说的想清楚、理出顺序，然后再动笔。
3. 问她**一个**问题，而且是能让她马上答得上来的那种。开场最该问的是**这篇到底要说的那一句话**：她最想让读者相信/明白什么。

绝对不要做的事：
- 不要替她写出任何一句可以直接放进文章的话（论点、开头句、段落）。你是陪她想的，不是替她写的。
- 不要一次问好几个问题。只问一个。
- 不要现在就列提纲、给结构方案，也不要把「并排说几条」「先承认，再反驳」「比一比」这类方法名当成选项摆给她挑——结构要在后面从她自己说的话里得出。
- 不要说"作为AI"、不要夸她"这个题目很棒"这类空话。
- 不用「慢慢」「一点一点」「一步一步」「理顺」这类修饰和比喻，直接说要做的事。

直接说话，不要任何前缀或标题。`

// The two sentences of writingOpeningSystem that call the topic hers, and what
// replaces them when her teacher assigned the writing. The topic is then the
// teacher's prompt, and restating it "用她自己的说法" as what she wants to write
// would put the teacher's words in her mouth in the room's first turn.
const (
	openingTopicOwn        = "下面是她自己写下的题目和她说过的话。"
	openingTopicAssigned   = "这篇写作是老师布置的：下面是老师布置的题目和她自己说过的话。"
	openingRestateOwn      = "1. 用一句话把她想写的东西说回给她，让她确认你听懂了。用她自己的说法，不要换成更\"高级\"的表述。"
	openingRestateAssigned = "1. 用一句话说明老师布置的题目要求写什么，并点明这是老师的要求，不要说成是她自己想写的。"
)

// writingOpeningSystemFor is writingOpeningSystem for this writing: unchanged
// for a writing she opened herself, with the two sentences above swapped for
// an assigned one.
func writingOpeningSystemFor(wr sqlc.Writing) string {
	if wr.Origin == "brought" {
		return writingBroughtOpeningSystem
	}
	if !writingIsAssigned(wr) {
		return writingOpeningSystem
	}
	return strings.NewReplacer(
		openingTopicOwn, openingTopicAssigned,
		openingRestateOwn, openingRestateAssigned,
	).Replace(writingOpeningSystem)
}

// writingIsAssigned: the writing was started from a teacher's assignment. Its
// topic, language and target are the teacher's. A blank prompt counts as none,
// the same rule as the client's isAssignedWriting.
func writingIsAssigned(wr sqlc.Writing) bool {
	return wr.AssignedPrompt != nil && strings.TrimSpace(*wr.AssignedPrompt) != ""
}

// writingBroughtOpeningSystem is the opening for a piece she wrote elsewhere
// and brought in for feedback.
//
// 🚨 2026-09-18 写作入口走查：带进来的一篇，印记的第一句是规划开场——
// 「你最想让读者最后相信的一件事是什么？」。她的文章已经写完、就摆在左边，
// 这句话等于没看见它。所以带进来的那一篇有自己的开场：先看见这篇，
// 再告诉她这一页怎么用。
const writingBroughtOpeningSystem = `你是「印记」，一个陪中学生写作的伙伴。学生带来了一篇**已经写好**的文章，想听意见。下面是题目和她的全文。现在由你先开口。

你的开场做到三件事，合起来不超过 120 个字：

1. 说出这篇里一处**真的写得好**的地方，要具体到她写的某个例子、某个说法或某个安排，并说明它好在哪里。不要泛泛地夸。
2. 用一句话说明接下来怎么做：点上方的「AI审阅」，印记会通篇读一遍，先指出最要紧的一两处；她照着改，改完可以再审阅一次。
3. 问她**一个**问题，帮你给出更有用的意见：比如这篇是为什么场合写的（考试、作业、比赛），或者她自己最没把握的是哪一部分。

绝对不要做的事：
- 不要现在就逐条挑毛病，也不要替她改写任何一句。
- 不要问她「想写什么」「最想让读者相信什么」——文章已经写完了。
- 不要一次问好几个问题。
- 不要说"作为AI"。

直接说话，不要任何前缀或标题。`

// broughtOpeningDraftRunes bounds how much of her piece the opening reads. The
// opening only needs enough to name one real strength.
const broughtOpeningDraftRunes = 4000

// buildBroughtOpeningPrompt is the opening's input for a brought piece: the
// title, the language line, and her text.
func buildBroughtOpeningPrompt(wr sqlc.Writing, body string) string {
	var b strings.Builder
	b.WriteString("题目：" + wr.Title + "\n")
	b.WriteString(writingLangLine(wr))
	if wr.TargetWords != nil {
		b.WriteString(writingLengthLine(wr, "她定的目标篇幅"))
	}
	b.WriteString("\n【她带来的全文】\n")
	body = strings.TrimSpace(body)
	if body == "" {
		b.WriteString("（正文是空的。）\n")
	} else {
		b.WriteString(cutRunes(body, broughtOpeningDraftRunes) + "\n")
	}
	return b.String()
}

// buildWritingOpeningPrompt assembles what the coach sees: her title, the
// settings she just chose, and everything she has said. AI turns are excluded
// — on the opening path there are none by construction (the handler refuses
// to run once one exists), and including the role would only invite the model
// to continue a conversation rather than start one.
func buildWritingOpeningPrompt(wr sqlc.Writing, msgs []sqlc.AtomMessage) string {
	var b strings.Builder
	b.WriteString(writingTopicLine(wr, "题目/想法："))
	b.WriteString(writingLangLine(wr))
	if wr.TargetWords != nil {
		b.WriteString(writingLengthLine(wr, "她定的目标篇幅"))
	} else {
		b.WriteString("她还没定篇幅（这完全没问题，别追问）。\n")
	}
	b.WriteString("\n【她自己说过的话】\n")
	any := false
	for _, m := range msgs {
		if m.Role != "student" {
			continue
		}
		if s := strings.TrimSpace(m.Content); s != "" {
			b.WriteString("- " + s + "\n")
			any = true
		}
	}
	if !any {
		b.WriteString("（她还没说什么，只有上面那个题目。）\n")
	}
	return b.String()
}

// postWritingOpening is POST /api/v1/writings/{id}/opening — the coach's
// first line, called by the frontend immediately after the setup dialog
// closes.
//
// IDEMPOTENT, AND POSTGRES IS WHAT MAKES IT SO. The first version of this
// handler read the transcript for an 'ai' message and, finding none,
// generated one — a check-then-write with nothing holding the gap. Production
// on 2026-08-28 caught two requests 386 ms apart doing exactly that: two
// model charges, two stored greetings, a room that says hello twice.
//
// Why not a partial unique index (the usual answer here, and the one 0096
// used for atom_card): there is no predicate that picks out the opening. The
// planning conversation legitimately accumulates MANY 'ai' rows in the same
// thread (writing_plan.go, writing_turn.go), so "one ai row per atom" is
// false; and the opening is not at a fixed seq either — seq 1 is always her
// idea (writings.go) and seq 2 is her setup note when she wrote one, so the
// greeting lands at 2 or 3. The only property that distinguishes it is "it is
// the first ai turn", which no index predicate can express without a new
// marker column.
//
// So the guarantee is a TRANSACTION-SCOPED ADVISORY LOCK keyed on the atom —
// the same instrument ensureRootQuestion (exploration.go) already uses in
// this codebase for the same shape, and the same thing 0096's index bought
// atom_card: arbitration by the database, not by application sequencing. It
// also buys something an index alone cannot — the loser blocks BEFORE the
// model call, so a race costs one call, not two. That is the whole point;
// a unique index would have caught the second row after paying for it.
//
// The cost, stated plainly: the winner holds a pool connection for the length
// of the model call (chaperone tier, a couple of seconds). Openings are once
// per writing, so this is a handful of connections out of 20 at worst.
func (a *API) postWritingOpening(w http.ResponseWriter, r *http.Request) {
	at, ok := a.loadOwnedWritingAtom(w, r)
	if !ok {
		return
	}
	// Cheap pre-check outside the lock: after the first call this is the only
	// cost, and it short-circuits every reload without touching the lock.
	msgs, err := a.d.Queries.ListAtomMessages(r.Context(), at.ID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if reply, found := firstAIReply(msgs); found {
		httpx.WriteJSON(w, http.StatusOK, map[string]any{"reply": reply, "generated": false})
		return
	}

	u, _ := UserFromContext(r.Context())
	entitled, err := HasEntitlement(r.Context(), u)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if !entitled {
		httpx.WriteError(w, r, httpx.ErrNotEntitled())
		return
	}

	turnCtx, cancel := context.WithTimeout(context.WithoutCancel(r.Context()), 150*time.Second)
	defer cancel()

	// Everything from here to the commit is the critical section. A second
	// request blocks on the advisory lock until this one commits, then re-reads
	// and finds the greeting — so it never reaches the provider at all.
	tx, err := a.d.Pool.Begin(turnCtx)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	defer func() { _ = tx.Rollback(context.WithoutCancel(turnCtx)) }()
	// hashtext of the atom uuid — a per-writing mutex held to COMMIT. Namespaced
	// with a suffix so it can never collide with another advisory lock that
	// happens to key on the same uuid.
	if _, err := tx.Exec(turnCtx, "SELECT pg_advisory_xact_lock(hashtext($1))", at.ID.String()+":opening"); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	qtx := a.d.Queries.WithTx(tx)

	// Re-read UNDER the lock. This is the check that actually decides; the one
	// above the entitlement gate is only an optimisation.
	msgs, err = qtx.ListAtomMessages(turnCtx, at.ID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if reply, found := firstAIReply(msgs); found {
		httpx.WriteJSON(w, http.StatusOK, map[string]any{"reply": reply, "generated": false})
		return
	}

	wr, err := qtx.GetWriting(turnCtx, at.ID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}

	// §model-routing · dialogue. An opening greeting is conversational, not
	// evaluative — the same class as the ordinary coach turn.
	resolved, rerr := a.routeE(turnCtx, gateway.ClassDialogue)
	if rerr != nil {
		slog.Warn("writing opening: resolve model failed", "err", rerr,
			"atom_id", at.ID, "request_id", httpx.RequestIDFromContext(r.Context()))
		httpx.WriteError(w, r, httpx.ErrAIDialogueFailed("model_unavailable"))
		return
	}
	userPrompt := buildWritingOpeningPrompt(wr, msgs)
	if wr.Origin == "brought" {
		draft, derr := qtx.GetWritingDraft(turnCtx, at.ID)
		if derr != nil && !errors.Is(derr, pgx.ErrNoRows) {
			httpx.WriteError(w, r, derr)
			return
		}
		userPrompt = buildBroughtOpeningPrompt(wr, draft.Body)
	}
	res, cerr := gateway.Collect(turnCtx, a.d.Provider, resolved, gateway.ChatRequest{
		Messages: []gateway.ChatMessage{
			{Role: gateway.RoleSystem, Content: writingOpeningSystemFor(wr)},
			{Role: gateway.RoleUser, Content: userPrompt},
		},
	})
	a.recordLiteLLMCall(turnCtx, u.ID, at.ID, "writing_opening", resolved, res.Usage)
	reply := strings.TrimSpace(res.Text)
	if cerr != nil || reply == "" {
		// USER RULE (2026-08-22): an AI-dialogue failure is surfaced as a real
		// 502, never masked by a canned stand-in sentence — even here, where a
		// hardcoded greeting would be trivially easy and would look fine. It
		// would also be a lie about whether the coach is actually working.
		slog.Warn("writing opening: model call failed", "err", cerr,
			"atom_id", at.ID, "request_id", httpx.RequestIDFromContext(r.Context()))
		httpx.WriteError(w, r, httpx.ErrAIDialogueFailed("model_unavailable"))
		return
	}

	// 串行化这颗原子的 seq 分配。NextAtomMessageSeq 是先读后插，
	// 并发下两笔事务会读到同一个 MAX——唯一索引保住的是数据，代价是
	// 其中一轮直接失败，而那一轮的模型钱已经花掉了。见 queries/atom.sql。
	if _, err := qtx.LockAtom(turnCtx, at.ID); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	seq, err := qtx.NextAtomMessageSeq(turnCtx, at.ID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if _, err := qtx.AppendAtomMessage(turnCtx, sqlc.AppendAtomMessageParams{
		AtomID: at.ID, Seq: seq, Role: "ai", Content: reply,
	}); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if err := tx.Commit(turnCtx); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"reply": reply, "generated": true})
}

// firstAIReply is the "has the coach already spoken here?" test, in one place
// because postWritingOpening asks it twice (once cheaply, once under the
// lock) and the two must never disagree. The room's own thread only —
// ListAtomMessages already filters block_id IS NULL, so a 深入一层 sub-agent
// turn on some block can never be mistaken for the opening.
func firstAIReply(msgs []sqlc.AtomMessage) (string, bool) {
	for _, m := range msgs {
		if m.Role == "ai" {
			return m.Content, true
		}
	}
	return "", false
}
