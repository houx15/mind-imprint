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
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

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
	if req.Lang != "zh" && req.Lang != "en" {
		httpx.WriteError(w, r, httpx.ErrBadRequest("invalid_lang", "语言只能是中文或英文。", nil))
		return
	}
	var tw *int32
	if req.TargetWords != nil {
		if *req.TargetWords < minTargetWords || *req.TargetWords > maxTargetWords {
			httpx.WriteError(w, r, httpx.ErrBadRequest("invalid_target_words", "目标字数需在 1 到 100000 之间。", nil))
			return
		}
		v := int32(*req.TargetWords)
		tw = &v
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
		AtomID: at.ID, Lang: req.Lang, TargetWords: tw,
	})
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if note != "" {
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
	httpx.WriteJSON(w, http.StatusOK, writingDTOOf(wr, at.CreatedAt))
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

接下来你们要做的是**规划**：一起把这篇要说什么、按什么顺序说，一点一点想清楚。她说的每一点都会长到右边那张图上。现在由你先开口。

你的开场要做到三件事，合起来不超过 120 个字：

1. 用一句话把她想写的东西说回给她，让她确认你听懂了。用她自己的说法，不要换成更"高级"的表述。
2. 用一句话告诉她这一步要干什么——先一起把要说的想清楚、理出顺序，然后再动笔。
3. 问她**一个**问题，而且是能让她马上答得上来的那种。开场最该问的是**这篇到底要说的那一句话**：她最想让读者相信/明白什么。

绝对不要做的事：
- 不要替她写出任何一句可以直接放进文章的话（论点、开头句、段落）。你是陪她想的，不是替她写的。
- 不要一次问好几个问题。只问一个。
- 不要现在就列提纲、给结构方案，也不要把「并排说几条」「先承认，再反驳」「比一比」这类方法名当成选项摆给她挑——结构是后面从她自己说的话里长出来的。
- 不要说"作为AI"、不要夸她"这个题目很棒"这类空话。

直接说话，不要任何前缀或标题。`

// buildWritingOpeningPrompt assembles what the coach sees: her title, the
// settings she just chose, and everything she has said. AI turns are excluded
// — on the opening path there are none by construction (the handler refuses
// to run once one exists), and including the role would only invite the model
// to continue a conversation rather than start one.
func buildWritingOpeningPrompt(wr sqlc.Writing, msgs []sqlc.AtomMessage) string {
	var b strings.Builder
	if t := strings.TrimSpace(wr.Title); t != "" {
		b.WriteString("题目/想法：" + t + "\n")
	}
	if wr.Lang == "en" {
		b.WriteString("这篇用英文写。\n")
	} else {
		b.WriteString("这篇用中文写。\n")
	}
	if wr.TargetWords != nil {
		b.WriteString("她定的目标篇幅：约 " + strconv.Itoa(int(*wr.TargetWords)) + " 字。\n")
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
// IDEMPOTENT BY CONSTRUCTION: if the transcript already holds any 'ai'
// message, this returns that existing opening rather than generating a
// second one. Without that guard a refresh (or React's double-invoked
// effects in dev) would mint a fresh greeting every time — burning money and,
// worse, showing her a room that keeps re-introducing itself.
func (a *API) postWritingOpening(w http.ResponseWriter, r *http.Request) {
	at, ok := a.loadOwnedWritingAtom(w, r)
	if !ok {
		return
	}
	msgs, err := a.d.Queries.ListAtomMessages(r.Context(), at.ID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	for _, m := range msgs {
		if m.Role == "ai" {
			httpx.WriteJSON(w, http.StatusOK, map[string]any{"reply": m.Content, "generated": false})
			return
		}
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

	wr, err := a.d.Queries.GetWriting(turnCtx, at.ID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}

	// §model-routing: an opening greeting is conversational, not evaluative —
	// the chaperone tier, same as the ordinary coach turn.
	resolved, rerr := a.d.ChatResolver(turnCtx)
	if rerr != nil {
		slog.Warn("writing opening: resolve model failed", "err", rerr,
			"atom_id", at.ID, "request_id", httpx.RequestIDFromContext(r.Context()))
		httpx.WriteError(w, r, httpx.ErrAIDialogueFailed("model_unavailable"))
		return
	}
	res, cerr := gateway.Collect(turnCtx, a.d.Provider, resolved, gateway.ChatRequest{
		Messages: []gateway.ChatMessage{
			{Role: gateway.RoleSystem, Content: writingOpeningSystem},
			{Role: gateway.RoleUser, Content: buildWritingOpeningPrompt(wr, msgs)},
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

	seq, err := a.d.Queries.NextAtomMessageSeq(turnCtx, at.ID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if _, err := a.d.Queries.AppendAtomMessage(turnCtx, sqlc.AppendAtomMessageParams{
		AtomID: at.ID, Seq: seq, Role: "ai", Content: reply,
	}); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"reply": reply, "generated": true})
}
