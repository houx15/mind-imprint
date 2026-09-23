package api

// reading_coach.go — 带读：由 印记 领着走的阅读。
//
// 2026-08-27 的第二轮裁定，推翻了同一天早些时候那个「任务清单 + 复选框」：
//
//   > merge the tasks with the AI bar. at start the AI begins with 让我来带你
//   > 详细阅读这篇文章吧, clicks 开始. then AI generates the plan, introduces
//   > the plan, then we will enter a stage directly. student doesn't handle the
//   > stages themselves, but the AI directs these.
//
// 所以这一版里，**学生不再管理阶段**。她不点「做完了」，也不点「跳过」——她
// 只是读、只是回答。是否往下走、走到哪一步，由 印记 判断。清单还在库里（它是
// 过程证据），但它不再是一张要她操作的表，而是 印记 手里的教案。
//
// ## 三条硬规则
//
//  1. **一次只领一步。** 每一轮只说当前这一步要做什么，说完就停。铁律③。
//  2. **不抢答。** 她没明确索答时，留给她自己判断；明确索答时如实回答，
//     但不把索答误记成跳过。
//  3. **推进由模型判断，但只能往前一步。** 一轮最多推进一步：一次跳三步等于
//     替她把整篇读完了。
//
// 跳过没有消失，只是不再是一颗按钮：她说「这步跳过吧」，模型把它标成 skipped
// 再往下走。跳过依然被记录（铁律④），只是现在记录的是一句她真说过的话。

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"mindimprint/api/internal/agent"
	"mindimprint/api/internal/cards"
	"mindimprint/api/internal/gateway"
	"mindimprint/api/internal/httpx"
	"mindimprint/api/internal/store/sqlc"
)

// readingCoachTurnsWindow bounds the transcript fed to a guided turn. Same
// reasoning as everywhere else in lite: no compaction layer, so this window is
// the only thing bounding prompt growth.
const readingCoachTurnsWindow = 14

// readingPick is one sentence she pointed at in the article, rather than
// typed. Same shape, and the same reason, as the writing room's comment-quote
// validator: a guarantee you can check (literal substring of a real
// paragraph) beats one you asked the model to honor.
type readingPick struct {
	BlockID string `json:"blockId"`
	Quote   string `json:"quote"`
}

// validateReadingPicks keeps only picks that point at a real paragraph AND
// quote it literally. Anything else — an unknown block id, an empty quote, a
// paraphrase, or words that are real but belong to a different paragraph —
// is dropped silently rather than passed on for the model to sort out.
func validateReadingPicks(picks []readingPick, blocks []Block) []readingPick {
	byID := make(map[string]string, len(blocks))
	for _, b := range blocks {
		byID[b.ID] = b.Text
	}
	out := make([]readingPick, 0, len(picks))
	for _, p := range picks {
		q := strings.TrimSpace(p.Quote)
		if q == "" {
			continue
		}
		body, ok := byID[p.BlockID]
		if !ok || !strings.Contains(body, q) {
			continue
		}
		out = append(out, readingPick{BlockID: p.BlockID, Quote: q})
	}
	return out
}

// readingPickOrdinal finds the paragraph ordinal (第几段) for a pick's block
// id, counting position in blocks the same way readingBlockTag does — so the
// number shown here always matches the number the paragraph listing above it
// uses for the same block.
func readingPickOrdinal(blocks []Block, blockID string) (int, bool) {
	for i, blk := range blocks {
		if blk.ID == blockID {
			return i + 1, true
		}
	}
	return 0, false
}

// hasHuntPickEvidence is the F3 guard: a hunt step may only settle when a
// valid pick (a literal quote of a real paragraph) was seen THIS turn, or in
// the immediately preceding student turn. The one-turn lookback exists
// because she may point on turn N and the coach may legitimately settle on
// turn N+1 — without it, a coach that says "good, noted" one turn late would
// read as her having asserted rather than pointed.
//
// This needs no schema change: validateReadingPicks already guarantees
// `picks` covers this turn, and ReadingCoachPanel.tsx's send() always inlines
// every surviving quote into the student message content as `> ` blockquote
// lines BEFORE it is persisted — so re-validating the previous student
// message's quoted lines against the article's real paragraphs is an honest
// reconstruction of "did she point last turn", not a guess.
func hasHuntPickEvidence(picks []readingPick, msgs []sqlc.AtomMessage, blocks []Block) bool {
	if len(picks) > 0 {
		return true
	}
	for i := len(msgs) - 1; i >= 0; i-- {
		if msgs[i].Role != "student" {
			continue
		}
		return quotedLinesCiteArticle(msgs[i].Content, blocks)
	}
	return false
}

// quotedLinesCiteArticle reports whether content contains at least one `> `
// blockquote line that is a literal substring of some real paragraph — the
// same "real paragraph, quoted literally" bar validateReadingPicks holds
// structured picks to, applied to the transcript's own `> ` convention.
func quotedLinesCiteArticle(content string, blocks []Block) bool {
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, ">") {
			continue
		}
		quote := strings.TrimSpace(strings.TrimPrefix(line, ">"))
		if quote == "" {
			continue
		}
		for _, blk := range blocks {
			if strings.Contains(blk.Text, quote) {
				return true
			}
		}
	}
	return false
}

// quoteIsArticleText reports whether s appears literally inside SOME
// paragraph — the broad form of the check validateReadingPicks makes against
// one declared paragraph. It exists for one decision only: does this string
// belong to the article, and therefore have to reach the transcript behind a
// `> ` prefix? That question must be answered by looking at the article, not
// by trusting a `type` field the client sent along with the text.
//
// It normalizes CRLF for the same reason SplitBlocks does: the article it is
// compared against is already LF-only, so a quote that kept its "\r\n" would
// fail this test on line endings alone and be filed as her own words.
func quoteIsArticleText(s string, blocks []Block) bool {
	s = normalizeCardAnswerText(s)
	if s == "" {
		return false
	}
	for _, blk := range blocks {
		if strings.Contains(blk.Text, s) {
			return true
		}
	}
	return false
}

// normalizeCardAnswerText brings a client-sent string onto the SAME line
// endings SplitBlocks (reading_blocks.go) already normalized the article to.
// Without it a quote carrying CRLF — a Windows browser, a PDF paste — can
// never be a literal substring of any block, so it fails every "are these the
// article's words?" test and lands in the transcript bare.
func normalizeCardAnswerText(s string) string {
	return strings.TrimSpace(strings.ReplaceAll(s, "\r\n", "\n"))
}

// cardAnswerChoiceParts classifies her tapped choice against the ARTICLE and
// splits it into the two halves composeCardAnswerMessage keeps apart: `quote`
// (the article's words, every line of which reaches the transcript behind a
// `> `) and `own` (hers, left bare). It also returns the picks the tap earns.
//
// 🚨 Classified by CHECKING, never by the declared type. A client that
// mislabels an article sentence as short_text would otherwise drop the
// article's words, unprefixed, into the corpus of "her own words".
func cardAnswerChoiceParts(choice, blockID string, blocks []Block) (quote, own string, picks []readingPick) {
	choice = normalizeCardAnswerText(choice)
	if choice == "" {
		return "", "", nil
	}
	if !quoteIsArticleText(choice, blocks) {
		// Not the article's words WHOLE — which is not the same as "none of it
		// is the article's". composeCardAnswerMessage classifies this half line
		// by line; a cross-paragraph selection lands here (a block never
		// contains a blank line) and still reaches the transcript quoted.
		return "", choice, nil
	}
	// Tapping a sentence IS pointing at it. Fed through the same validator as
	// a drag-selection so it earns the same standing: the coach sees it under
	// 【她在文章里点出来的句子】, and a hunt step may settle on it
	// (hasHuntPickEvidence).
	//
	// 🚨 …but only above the same floor card OPTIONS have to clear
	// (coachCardMinQuoteRunes). Two characters are a substring of the article
	// too, and promoting a two-character short_text answer into `picks` would
	// let a hunt step settle on words she never pointed at — exactly what F3
	// of the system prompt refuses ("她只是说…却没有点 → 那不是点的").
	if utf8.RuneCountInString(choice) < coachCardMinQuoteRunes {
		return choice, "", nil
	}
	return choice, "", validateReadingPicks([]readingPick{{BlockID: blockID, Quote: choice}}, blocks)
}

// dedupeReadingPicks drops a pick that repeats one already in the list — same
// paragraph, same sentence. Two paths produce picks in one turn (the panel
// inlines every drag-selection, and the tapped choice is promoted here), and
// 【她在文章里点出来的句子】 would otherwise show her one sentence twice.
func dedupeReadingPicks(picks []readingPick) []readingPick {
	seen := make(map[readingPick]bool, len(picks))
	out := make([]readingPick, 0, len(picks))
	for _, p := range picks {
		if seen[p] {
			continue
		}
		seen[p] = true
		out = append(out, p)
	}
	return out
}

// composeCardAnswerMessage turns her tap into the one thing the transcript
// stores: a student message whose every line of ARTICLE text carries a `> `
// prefix, and whose own-words half carries none.
//
// 🚨 This is R4's third enforcement point, and the reason it is a named
// function with its own test rather than three lines inside the handler.
// On 2026-08-29 the same invariant was broken in ReadingCoachPanel.send():
// it prefixed only the FIRST line of a drag-selected quote. SplitBlocks
// splits the article on blank lines only, so a hard-wrapped paragraph (a PDF
// paste, a poem, a stretch of dialogue) is ONE block containing newlines —
// the article's later lines landed in a role='student' row with no prefix,
// sailed through report_facts.go's stripQuotedLines, and became eligible to
// be printed under her name on a shareable picture. Card options ARE article
// sentences, so they are that same landmine's second chance: every line gets
// the prefix, including the card's own question (印记's words, not hers —
// they must not count as her prose either).
//
// `own` is whatever is CLAIMED to be hers this turn — a short_text answer, or
// anything she typed alongside the tap. Claimed, not trusted: it is classified
// LINE BY LINE against the article, and only the lines that are not the
// article's reach the transcript bare.
//
// 🚨 Per line, because the classification upstream is whole-string, and a
// whole-string test is all-or-nothing: a selection that spans a blank line
// (blocks never contain one) or that carried CRLF matches NO block, so under
// the old rule EVERY line of it landed bare. Line by line is exactly the test
// stripArticleLines (atom_report.go) makes when the report is built — moved to
// write time, so it no longer depends on the source row still being there.
// That second net degrades to a no-op when the source is gone; this one
// cannot. A line of her own prose that happens to be a literal substring of
// the article is prefixed too, the same trade stripArticleLines already makes
// and for the same reason: a coincidental drop costs her nothing a report
// needs, a false keep is the leak this code exists to close.
func composeCardAnswerMessage(prompt, quote, own string, blocks ...Block) string {
	lines := make([]string, 0, 8)
	// 印记's own question, on ONE line. The prompt arrives from the client and
	// a card's question may quote a sentence; folded onto a second line, that
	// sentence would be a `> ` line that reads back as HER pointing at the
	// article (quotedLinesCiteArticle → hasHuntPickEvidence), letting a hunt
	// step settle on 印记's words. Behind 【印记问】 on a single line it can
	// never be a literal substring of any paragraph.
	if p := collapseCardPrompt(prompt); p != "" {
		lines = append(lines, "> 【印记问】"+p)
	}
	if q := normalizeCardAnswerText(quote); q != "" {
		// Every line, unconditionally — an internal "\n" inside one quote is
		// the whole reason this loop exists.
		for _, line := range strings.Split(q, "\n") {
			lines = append(lines, "> "+line)
		}
	}
	if o := normalizeCardAnswerText(own); o != "" {
		if len(lines) > 0 {
			// The same blank-line separation ReadingCoachPanel.send() uses
			// between a quote block and what she typed under it.
			lines = append(lines, "")
		}
		for _, line := range strings.Split(o, "\n") {
			if quoteIsArticleText(line, blocks) {
				lines = append(lines, "> "+line)
				continue
			}
			lines = append(lines, line)
		}
	}
	return strings.Join(lines, "\n")
}

// collapseCardPrompt readies the client-sent card question for the one `> `
// line it is allowed: newlines folded into spaces, and anything longer than
// the cap validateCoachCard holds 印记's OWN cards to dropped outright. A
// prompt over 60 runes did not come from a card this coach wrote, and the
// transcript is not the place to find out what it did come from.
func collapseCardPrompt(prompt string) string {
	parts := make([]string, 0, 2)
	for _, line := range strings.Split(normalizeCardAnswerText(prompt), "\n") {
		if line = strings.TrimSpace(line); line != "" {
			parts = append(parts, line)
		}
	}
	p := strings.Join(parts, " ")
	if p == "" || utf8.RuneCountInString(p) > coachCardPromptMaxRunes {
		return ""
	}
	return p
}

// readingLensDone is what she just finished doing with a lens, when this turn
// is the room reporting a completed one rather than her saying something.
//
// ## Why this exists (2026-09-03, colleague trial)
//
// Reported as 「透镜应用完毕之后，没有响应，没有推进到下一步。没有和透镜选句
// 打通」. It was all three, and none of them were the model's fault:
//
// The lens loop is shared with pro (apps/web/src/studio/reading/readingLoop.ts).
// Its confirm() saved the outcome, appended a canned confirmation line to
// loop.messages, and called clearCard(). But lite does not RENDER
// loop.messages — it renders ReadingCoachPanel, a different thread over the
// same atom_message table. So the one acknowledgement the loop produced went
// into an array nothing on screen reads, no turn was ever posted, and 印记
// therefore never reacted and never advanced the step. From the student's side
// the lens just vanished.
//
// 🚨 The fix could NOT be an empty-text turn. That is the exact shape that bit
// the PBL room (an empty-text turn after finishing a tool made 印记 repeat
// itself verbatim AND re-summon the tool she had just done), and this builder
// has the same landmine at the bottom: studentText == "" prints 「她刚点了
// 「开始」，还没说话」, which would make 印记 re-introduce the whole reading
// plan the moment she finished a lens. So a completed lens arrives as its own
// labelled section, and it SUPPRESSES that fallback line.
//
// Nothing here is re-asked of a model: Quote is the sentence she picked and
// Finding is the evaluation agent.EvaluateSelection already returned when she
// submitted the card — the same two values reportLensNote reads at report
// time.
type readingLensDone struct {
	// CardName is the lens's display name (「溯源体检」), not its id: this
	// string is for the model to say back to her.
	CardName string `json:"cardName"`
	// Quote is the sentence SHE found in the article. The selecting is the
	// thinking, so this is the part 印记 must respond to specifically.
	Quote string `json:"quote"`
	// Finding is what the room already concluded from her pick — 印记's OWN
	// prior words, never presented to it as something she said.
	Finding string `json:"finding"`
	// Verdict / VerdictReason are the room's own read of her pick: "strong" |
	// "partial" | "rethink", plus the one line it showed her under it.
	//
	// 🚨 没有它们的时候，这一轮是**两个模型各说各的**：复核那一次
	// （agent.EvaluateSelection）当着她的面判了「这一句撑不住」，而 印记 这一轮
	// 只拿到了句子和 finding，于是照着「她交作业了，先说她哪里选得准」那一节
	// 夸了一句。产品负责人 2026-09-17 逐字报的：「句子匹配不通过，但是点击记录
	// 发现后，主 AI 又给出了不一样的回答。」
	//
	// 值不是模型给的 —— 它是同一间房间几秒钟前已经算出来并**显示给她看过**的
	// 那一个，所以这里只是把它转述过去，不是再问一次。
	// 允许为空：老客户端不发，那时候退回原来的行为。
	Verdict       string `json:"verdict"`
	VerdictReason string `json:"verdictReason"`
}

// verdictWord 是三档复核结论的中文说法，给 印记 在提示词里读。闭表：
// 客户端送来一个我们不认识的值，就当它没说（宁可少一句，不要一句编的）。
var verdictWord = map[string]string{
	"strong":  "选句能支持这项分析",
	"partial": "选句部分相关，分析仍需补充",
	"rethink": "选句与这项分析不匹配，请根据理由重新选择",
}

// clean reports whether this outcome carries enough to be worth a turn. A
// lens with no quote is not a completed lens, and refeeding one would spend a
// flagship call to say "nice work" about nothing.
func (l *readingLensDone) clean() bool {
	return l != nil && strings.TrimSpace(l.Quote) != ""
}

// maxLabelBoards 是一篇文章里最多摆几块标注板。
//
// 二：一块给她摆，一块留给「她摆完之后确实需要整体重摆」的那种情况。
// 产品负责人的那一篇摆了四块，其中三块是同一件事 —— 那不是练习，是重复劳动。
// 改正一两张的正确做法是**在话里说清楚**，见 taskLabel 的推进判据。
const maxLabelBoards = 2

// countLabelBoards —— 这篇文章里已经发过几块标注板。
//
// 数的是 印记 发出去的那些（ai 消息 payload 里的 card），不是她答过几块：
// 一块发出去她没答的板，对她来说照样是一次「又来一块」。
func countLabelBoards(msgs []sqlc.AtomMessage) int {
	return countCards(msgs, coachCardLabelRoles)
}

// countCards —— 这篇文章里 印记 已经发过几张这种类型的卡。
func countCards(msgs []sqlc.AtomMessage, typ string) int {
	n := 0
	for _, m := range msgs {
		if m.Role != "ai" || len(m.Payload) == 0 {
			continue
		}
		var p coachMessagePayload
		if err := json.Unmarshal(m.Payload, &p); err != nil {
			continue
		}
		if p.Card != nil && p.Card.Type == typ {
			n++
		}
	}
	return n
}

// answeredBoard —— 这一轮她交上来的是不是一块摆完了的板。
//
// 只认两块板的类型，且作答非空。她在输入框里打一句「我摆好了」不算：
// 这条判据的全部价值就在于它认的是**动作**，不是一句声明。
func answeredBoard(a *coachCardAnswer) bool {
	if a == nil || strings.TrimSpace(a.Choice) == "" {
		return false
	}
	switch strings.TrimSpace(a.Type) {
	case coachCardLabelRoles, coachCardWordBank, coachCardOrderEvents:
		return true
	}
	return false
}

// answeredOrderBoard —— 这一轮她交上来的是一块排好了的排序板。
func answeredOrderBoard(a *coachCardAnswer) bool {
	return answeredBoard(a) && strings.TrimSpace(a.Type) == coachCardOrderEvents
}

// dropReason —— 这一轮有什么东西没送到她屏幕上。卡片优先（它更具体）；
// 卡片没问题的时候，透镜那条也要说。
func (r readingCoachReply) dropReason() cardReject {
	if r.cardWhy != cardOK && r.cardWhy != cardRejectNoCard {
		return r.cardWhy
	}
	if r.lensWhy != "" {
		return cardReject(r.lensWhy)
	}
	return cardOK
}

// spokenParagraph —— 这句回复里提到的**最后一个**段号，换成段 id。
//
// 「第 5 段」是 印记 对她唯一的坐标说法（prompt 里明令不许说 b1/b2）。一句话里
// 提到好几段时取最后一个：「第 2 段说了封锁，现在我们看第 5 段」—— 她要去的是
// 第 5 段。段号不存在（它数错了）就当没说。
func spokenParagraph(reply string, blocks []Block) string {
	re := regexp.MustCompile(`第\s*([0-9]{1,2}|[一二三四五六七八九十]{1,3})\s*段`)
	all := re.FindAllStringSubmatch(reply, -1)
	for i := len(all) - 1; i >= 0; i-- {
		n := parseChineseOrdinal(all[i][1])
		if n >= 1 && n <= len(blocks) {
			return blocks[n-1].ID
		}
	}
	return ""
}

// parseChineseOrdinal —— 「5」或者「五」变成 5。超出两位就不认了（段号不会那么大）。
func parseChineseOrdinal(s string) int {
	if n, err := strconv.Atoi(s); err == nil {
		return n
	}
	digits := map[rune]int{'一': 1, '二': 2, '三': 3, '四': 4, '五': 5,
		'六': 6, '七': 7, '八': 8, '九': 9}
	r := []rune(s)
	switch {
	case len(r) == 1 && r[0] == '十':
		return 10
	case len(r) == 1:
		return digits[r[0]]
	case len(r) == 2 && r[0] == '十': // 十一 … 十九
		return 10 + digits[r[1]]
	case len(r) == 2 && r[1] == '十': // 二十 … 九十
		return digits[r[0]] * 10
	case len(r) == 3 && r[1] == '十': // 二十一 …
		return digits[r[0]]*10 + digits[r[2]]
	}
	return 0
}

// lastOpenCard —— 她屏幕上现在摆着的那张卡片（最后一条 印记 的话带的那张），
// 她还没答的时候。答过了就不必再给模型看：那一轮的作答本来就在转写里。
func lastOpenCard(msgs []sqlc.AtomMessage) *coachCard {
	for i := len(msgs) - 1; i >= 0; i-- {
		m := msgs[i]
		if m.Role == "student" {
			// 她在这张卡之后说过话 —— 那就是答过了（或者这一轮不是卡片轮）。
			if coachAnswerFromPayload(m.Payload) != nil {
				return nil
			}
			continue
		}
		if m.Role != "ai" || len(m.Payload) == 0 {
			continue
		}
		var p coachMessagePayload
		if err := json.Unmarshal(m.Payload, &p); err != nil {
			return nil
		}
		// 🚨 这条回复带的是别的东西（一条被丢掉的卡的理由、「这条没说完」、
		// 「回到原题」），不是一张卡 —— 那么她屏幕上摆着的仍然是**再往前**
		// 那张。不接着往前找的话，一张卡被丢掉之后模型就再也看不见她手上那张，
		// 而屏幕上它一直在。和 payload 整个为空的那一条走同一条路。
		if p.Card == nil {
			continue
		}
		return p.Card
	}
	return nil
}

// openBoard returns the latest board still awaiting a real board submission.
// A later ordinary card replaces an older board on screen, so it stops here
// rather than searching further back through the transcript.
func openBoard(msgs []sqlc.AtomMessage) *coachCard {
	for i := len(msgs) - 1; i >= 0; i-- {
		m := msgs[i]
		if m.Role == "student" {
			if a := coachAnswerFromPayload(m.Payload); a != nil && answeredBoard(a) {
				return nil
			}
			continue
		}
		if m.Role != "ai" || len(m.Payload) == 0 {
			continue
		}
		var p coachMessagePayload
		if err := json.Unmarshal(m.Payload, &p); err != nil || p.Card == nil {
			continue
		}
		switch p.Card.Type {
		case coachCardLabelRoles, coachCardWordBank, coachCardOrderEvents:
			return p.Card
		}
		return nil
	}
	return nil
}

// coachAnswerFromPayload —— 这条学生消息里有没有一次卡片作答。
func coachAnswerFromPayload(raw []byte) *coachCardAnswer {
	if len(raw) == 0 {
		return nil
	}
	var p coachMessagePayload
	if err := json.Unmarshal(raw, &p); err != nil {
		return nil
	}
	return p.Answer
}

// lastDroppedCard —— 最后一条 印记 说的话里，那张卡片是不是被丢掉了；是的话
// 给出理由。只看最后一条：再往前的那些它已经收到过反馈了。
func lastDroppedCard(msgs []sqlc.AtomMessage) string {
	for i := len(msgs) - 1; i >= 0; i-- {
		if msgs[i].Role != "ai" {
			continue
		}
		if len(msgs[i].Payload) == 0 {
			return ""
		}
		var p coachMessagePayload
		if err := json.Unmarshal(msgs[i].Payload, &p); err != nil {
			return ""
		}
		return p.Dropped
	}
	return ""
}

// readingTaskAfterAdvance —— 这一轮走完之后，她站在哪一步。
//
// 和提交之后 `currentReadingTask(after)` 算出来的是同一个答案，只是从内存里的
// 那份清单算，因此**在写消息之前**就拿得到。要它是因为那颗「跳到第 N 段」按钮
// 存在消息的 payload 上（刷新之后还要在），而 payload 是在事务里写的。
//
// advance 为空 = 这一步没走完，她还在这一步；否则往后找第一个还 pending 的。
func readingTaskAfterAdvance(tasks []sqlc.ReadingTask, advance string) *sqlc.ReadingTask {
	cur := currentReadingTask(tasks)
	if cur == nil || advance == "" {
		return cur
	}
	for i := range tasks {
		if tasks[i].Status == "pending" && tasks[i].ID != cur.ID {
			return &tasks[i]
		}
	}
	return nil
}

// currentReadingTask is the first step not yet settled. Nil when everything is
// done or skipped — the state where the coach stops leading rather than
// inventing a step to fill the silence.
func currentReadingTask(tasks []sqlc.ReadingTask) *sqlc.ReadingTask {
	for i := range tasks {
		if tasks[i].Status == "pending" {
			return &tasks[i]
		}
	}
	return nil
}

type readingCoachReply struct {
	Reply      string `json:"reply"`
	Advance    string `json:"advance"`
	FocusBlock string `json:"focusBlock"`
	// cardWhy 是这一轮那张卡片为什么没发出去（发出去了就是空）。不是模型给的，
	// 是校验器填的 —— 所以没有 json tag，它不参与解析。跟着 reply 一起走出去，
	// 是为了让它能被存进这条消息的 payload，下一轮当面告诉模型。
	cardWhy cardReject
	// lensWhy 同理：这一轮那副透镜为什么没落到文章上。
	lensWhy string
	// askedPrompt：这一轮它本来想问的那道题（哪怕那张卡后来被丢掉了）。
	// 兜底发卡时用它，这样她看到的仍然是 印记 问的那句话，不是我们编的。
	askedPrompt string
	// lensRetry：透镜递出去了，但这一轮的话配不上它 —— 没有当着她的面做一遍，
	// 或者话里说的是另一件她此刻做不了的事（板）。
	// 和上面两个不一样：它不导致任何东西被丢掉，只让这一轮重来一次。
	lensRetry    bool
	lensRetryWhy string
	// twoAsks：这一轮发了一张卡片，而话又以**另一个**问题收尾。屏幕上于是有
	// 两道题，措辞还不一样，她只能挑一道信。和 lensRetry 一样：不丢任何东西，
	// 只让这一轮重来一次。
	twoAsks bool
	// ghostQuote：这一轮的话里引了一句**文章上、卡片上、她嘴里都没有**的话
	// （reading_ghostquote.go）。存的是那句引文本身，因为日志里要有它 ——
	// 「又引错了」和「它把原文两句压成一句」是两件不同的事。
	// 和 twoAsks 一样：不丢任何东西，只让这一轮重来一次。
	ghostQuote string
	// leak：这一轮说漏嘴了 —— 把协议词写进了她读到的话里（「advance给done」），
	// 或者当着她的面把她叫成「她」。见 reading_coach_leak.go。
	// 和 twoAsks 一样：不丢任何东西，只让这一轮重来一次。
	leak string
	// The paragraph tool the coach chose to reach for this turn, if any. The
	// tools are its teaching instruments, not a menu she is left to browse.
	Tool string `json:"tool"`
	// Lens is the reading-deck card id the coach reaches for this turn, if
	// any. A paragraph tool explains a paragraph; a lens makes her perform an
	// analysis on a sentence she chooses herself. Empty on most turns.
	Lens string `json:"lens"`
	// Card is the tappable card the coach wrote into this reply, if any. It
	// rides on the message, not on atom_card: this is not a lens summon, it
	// is the step's own instruction shaped so she answers by pointing.
	// Nil on most turns, and nil whenever validateCoachCard refused it —
	// a refused card is not an error, she simply gets words instead.
	Card *coachCard `json:"card"`
}

// tailRunes returns the last n runes of s, for logging a reply we could not
// read. Rune-safe: cutting UTF-8 by bytes puts a broken character in the log.
func tailRunes(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return "…" + string(r[len(r)-n:])
}

// headRunes returns the first n runes of s, for the same reason tailRunes
// returns the last ones.
//
// 🚨 两头都要。一份读不出来的回复，只看尾巴分不出「前几块是好的、坏在最后一
// 块」和「第一块就坏了」——而这两种的处置完全相反：前者该查救援那条路，
// 后者该查提示词。2026-09-11 线上那一条只有尾巴，两种猜都成立。
func headRunes(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}

// salvageCoachReply reads a coach reply key by key and keeps every field that
// arrived whole, stopping at the first one that did not.
//
// # 🚨 为什么需要它
//
// 模型会**在正常收尾的同时**把 JSON 断在半路：provider 报 finish_reason
// "stop"，completion 只有一两百个 token（上限是 16000），而对象停在
//
//	…"advance":"","focusBlock":"","tool":"","lens":"","card":{"type":"choose_span",
//	"prompt":"作者站哪一边？","options":[{"blockId":"b10","quote":"It shows that the
//
// 实测（TestLiveEnglishCoachFirstTurnParses，dashscope/deepseek-v4-pro）：
// **英文文章的第一轮 6 次里断 3 次**，加上生产那一次重试仍有约四分之一的轮次
// 直接变成 502。中文文章上同一个毛病只有 1/6（lensdone 那个文件量到的），
// 差别在于第一轮总会带一张卡片，而卡片的 options 要**逐字引用原文** ——
// 英文原句是同义中文句的三到五倍 token，尾巴就长得多。
// 给 provider 加 response_format=json_object 没有用（实测 4/6，没有变好）。
//
// 关键的事实是：断点**永远在 reply 后面**。学生要的那句话每一次都是齐的，
// 被切掉的是那件可有可无的教具。整份丢掉，等于为了一张卡片扔掉一轮好回答。
//
// 这不是编一个回答（[[ai-errors-must-surface-never-fake]] 禁的是那件事）——
// 留下来的每个字都是模型真的发出来的；没到齐的字段当作它没给。reply 自己
// 断了就仍然算失败，调用点会再问一次，两次都断才报错给她看。
//
// 同 `salvagePlanets`（news/select.go）：一天的星图也曾因为同一件事整个丢掉。
func salvageCoachReply(s string) (readingCoachReply, bool) {
	dec := json.NewDecoder(strings.NewReader(s))
	tok, err := dec.Token()
	if err != nil {
		return readingCoachReply{}, false
	}
	if d, ok := tok.(json.Delim); !ok || d != '{' {
		return readingCoachReply{}, false
	}
	var got readingCoachReply
	for {
		key, kerr := dec.Token()
		if kerr != nil {
			break
		}
		if d, isDelim := key.(json.Delim); isDelim && d == '}' {
			break
		}
		name, isStr := key.(string)
		if !isStr {
			break
		}
		var raw json.RawMessage
		if verr := dec.Decode(&raw); verr != nil {
			// 这个字段没写完 —— 后面不会再有完整的东西了。
			break
		}
		// 单个字段类型不对，只丢这一个字段，不牵连整轮。
		switch name {
		case "reply":
			_ = json.Unmarshal(raw, &got.Reply)
		case "advance":
			_ = json.Unmarshal(raw, &got.Advance)
		case "focusBlock":
			_ = json.Unmarshal(raw, &got.FocusBlock)
		case "tool":
			_ = json.Unmarshal(raw, &got.Tool)
		case "lens":
			_ = json.Unmarshal(raw, &got.Lens)
		case "card":
			var card coachCard
			if json.Unmarshal(raw, &card) == nil {
				got.Card = &card
			}
		}
	}
	if strings.TrimSpace(got.Reply) == "" {
		return readingCoachReply{}, false
	}
	return got, true
}

func parseReadingCoachReply(text string, blocks []Block, lang string, lensOK func(cardID string) bool) (readingCoachReply, bool) {
	valid := make(map[string]bool, len(blocks))
	for _, blk := range blocks {
		valid[blk.ID] = true
	}
	c := strings.TrimSpace(text)
	if strings.HasPrefix(c, "```json") {
		c = strings.TrimLeft(strings.TrimPrefix(c, "```json"), " \t\r\n")
	} else if strings.HasPrefix(c, "```") {
		c = strings.TrimLeft(c[3:], " \t\r\n")
	}
	if strings.HasSuffix(c, "```") {
		c = strings.TrimRight(c[:len(c)-3], " \t\r\n")
	}
	if i := strings.IndexByte(c, '{'); i > 0 {
		c = c[i:]
	}
	whole := strings.TrimSpace(c)
	if j := strings.LastIndexByte(c, '}'); j >= 0 && j < len(c)-1 {
		c = c[:j+1]
	}
	var got readingCoachReply
	if err := json.Unmarshal([]byte(escapeRawControlInStrings(strings.TrimSpace(c))), &got); err != nil {
		// 🚨 断在半路的回复，把已经到齐的那部分留下来。见 salvageCoachReply。
		var ok bool
		if got, ok = salvageCoachReply(escapeRawControlInStrings(whole)); !ok {
			// 🚨 最后一种：它压根没在写 JSON，直接说了人话。
			//
			// 2026-09-10 的模拟学生走查抓到的，日志里逐字记着：256 个字符、
			// stop_reason "stop"、一句**完全能用**的话 ——
			// 「你的眼睛很准——第4段是整个报道里信息最密的一段。现在我要给你
			// 一张卡片……」。我们把它整份丢掉，然后给她看
			// 「后台错误：AI 响应错误（model_unavailable）」，她当场卡死。
			//
			// 把它当 reply 用，别的字段全空：她拿到 印记 真正说的那句话，
			// 没有卡片、不推进，下一轮照常。这不是编造 —— 这就是模型说的话，
			// 只是没穿那件 JSON 外套。
			//
			// 🚨 只在**一个左大括号都没有**的时候这么做。它要是在写 JSON 只是
			// 写坏了，原样端给她的会是一堆 {"reply":... —— 那比报错更糟。
			if prose := strings.TrimSpace(whole); prose != "" && !strings.Contains(prose, "{") {
				return readingCoachReply{Reply: prose}, true
			}
			return readingCoachReply{}, false
		}
	}
	got.Reply = strings.TrimSpace(got.Reply)
	if got.Reply == "" {
		return readingCoachReply{}, false
	}
	// Only the two advances the contract names. Anything else — including a
	// model trying to jump several steps by inventing a value — leaves her
	// exactly where she is, which is the safe direction to fail in.
	if got.Advance != "done" && got.Advance != "skipped" {
		got.Advance = ""
	}
	if got.FocusBlock != "" && !valid[got.FocusBlock] {
		got.FocusBlock = ""
	}
	// A tool the coach named must exist AND fit this article's language — a
	// 语法 breakdown of a Chinese paragraph is confidently useless. An
	// unusable id is dropped rather than passed on: the reply still stands,
	// she just doesn't get an instrument that would have opened onto nothing.
	if got.Tool != "" {
		tool, found := findReadingBlockTool(got.Tool)
		if !found || (tool.Lang != "" && tool.Lang != lang) {
			got.Tool = ""
		}
	}
	// A tool with no paragraph to open on is meaningless.
	if got.Tool != "" && got.FocusBlock == "" {
		got.Tool = ""
	}
	// A lens must be aimed. An un-aimed coach summon is exactly the 透镜库
	// she already has — what makes this the thing the product asked for is
	// that it lands on the paragraph the coach just talked about.
	if got.Lens != "" && (got.FocusBlock == "" || lensOK == nil || !lensOK(got.Lens)) {
		// 🚨 透镜和卡片一样，被丢掉是**静默**的。线上实测（OSIRIS-REx 那篇）：
		// 印记 连着八轮**一字不差**地说「用一副透镜重新看一遍第8段」——透镜每轮
		// 都因为没有落点被丢掉，她屏幕上什么都没变，于是下一轮的状态和上一轮
		// 完全一样，同样的输入自然产出同样的输出。
		//
		// 记下来，理由和卡片走同一条路：存进这条回复的 payload，下一轮当面说。
		got.Lens = ""
		if got.lensWhy == "" {
			if got.FocusBlock == "" {
				got.lensWhy = "the lens had no focusBlock to land on"
			} else {
				got.lensWhy = "the lens id is not in the deck"
			}
		}
	}
	cardType, cardPrompt := "", ""
	if got.Card != nil {
		cardType = got.Card.Type
		cardPrompt = tailRunes(got.Card.Prompt, 60)
		// 🚨 校验会把这张卡整个丢掉，而**她的那道题是好的** —— 坏的是选项
		// （引文对不上原文、或者全来自同一段）。留住那句问题，兜底的时候还给她：
		// 见 fallbackCardFor。
		got.askedPrompt = strings.TrimSpace(got.Card.Prompt)
	}
	// A card whose options are not literally in the article is the one failure
	// she could never detect herself — the whole reason to build the card is
	// that its answer doesn't exist outside the text. So it is checked, not
	// trusted, and a card that fails is dropped rather than repaired: the turn
	// still succeeds and she gets the coach's words with no card attached.
	got.Card, got.cardWhy = validateCoachCardWhy(got.Card, blocks)
	// 🚨 说出为什么。丢掉是对的，静默不是：走查里 印记 连着两轮在说「把这几句
	// 拖到格子里」而板从来没出现过，日志里一个字都没有，只能靠猜。
	// 「no card in the reply」不记 —— 大多数轮本来就没有卡片，那不是失败。
	// 🚨 说了「点这张卡」却没给卡：对她来说和「卡片被丢掉」长得一模一样 ——
	// 屏幕上一句指着空气的话。区别只在日志里干净得可怕（没有东西被丢掉，
	// 是根本没有东西），所以这一条必须自己抓。
	//
	// 🚨 只在**它压根没给卡**的时候判这一条。给了卡但卡被上面那道校验刷掉，
	// 真正的原因是那一条（句子不在原文里、选项只来自一段、问法被禁……），
	// 在这里改写成「你提了卡却没给」就把真原因盖掉了 —— 日志和喂回去的
	// 修正话术会一起说错，而喂错了它下一轮只会照着错的方向改。
	// 实测那一幕：日志写着「提了卡片却没附」，同一行里却印着那张卡的 type
	// 和 prompt；她那一步什么都没等到，屏幕上只有一句指着空气的话。
	if got.Card == nil && got.cardWhy == cardRejectNoCard && replyPromisesACard(got.Reply) {
		got.cardWhy = cardRejectPromised
	}
	// 🚨 断在半句上的回复也算这一轮坏了。她读到的是半截话，不知道该干嘛。
	// 只在没卡片也没透镜的时候判 —— 带着卡片时用冒号收尾是正常写法。
	//
	// 🚨 这里原来写的是 `got.cardWhy == cardOK`，而**没有卡片的那一轮 cardWhy 是
	// cardRejectNoCard**（见 validateCoachCardWhy）—— 于是这道闸只对「带着卡片的
	// 回复」生效，而它的条件里又要求 Card == nil。两个条件永远不会同时成立，
	// 这道闸从写下来那天起一次都没响过。
	//
	// 线上逐字证据（atom 609f3910，2026-09-11）：seq 3 是
	// 「对，调查数据是一个方向。**但」，payload 里 dropped 是空的 —— 没有任何
	// 东西被判失败，她只能自己打一个「?」去问。产品负责人报的第 1 条就是它。
	if (got.cardWhy == cardOK || got.cardWhy == cardRejectNoCard) &&
		got.Card == nil && got.Lens == "" && replyLooksCutOff(got.Reply) {
		got.cardWhy = cardRejectCutOff
	}
	// 🚨 递透镜的那一轮，话里必须当着她的面把这套看法做一遍 —— 拿原文的一句。
	//
	// prompt 里早就写着「先在 reply 里挑出这一段里的某一句，当着她的面把这种
	// 分析做一遍」，但没有任何东西验它。实测她逐字报的：
	//   「它一直让我用一副『透镜』去拆句子，但从来没给我看过这副透镜是什么、
	//     怎么用。前面说要先演示一遍给我看，结果什么都没有。」
	//   「它让我用传播学的角度分析……我真的不懂这些词是什么意思，我只是个高中生。」
	// 方法名照说（[[yinji-must-talk-like-a-teacher]]：要用真的方法名），
	// 但光有名字没有示范，那个名字对她就是一个生词。
	//
	// 判据是能验的那一个：**这一轮的话里有没有一段逐字来自落点段的原文**。
	// 示范一定引原句，空谈一定不引。
	//
	// 🚨 这一条**不丢透镜**。丢了她就只剩那几个生词而没有工具，比教得薄更糟。
	// 走的是重来一次那条路：第二次带上示范就用第二次，仍然没有就照常把透镜给她。
	if got.Lens != "" && got.lensWhy == "" && !replyQuotesBlock(got.Reply, blocks, got.FocusBlock) {
		got.lensRetry = true
		got.lensRetryWhy = "the lens turn never demonstrates the method on a real sentence"
	}
	// 🚨 一轮里递了透镜，话里却在说板 —— 她照着话去做，做不成。
	//
	// 铁律③ 一次只交给她一件事：透镜在的时候卡片会被丢掉（cardRejectLensWon），
	// 于是「把这句挪到证据那个格子里」这句话指向的东西根本不存在。实测她逐字
	// 报的：「它让我把句子挪到『证据』那个格子里，但我现在看不到任何可以拖拽的
	// 板子或卡片，只有文本框。」
	// 递哪件，话就只说哪件。这一轮重来一次。
	if got.Lens != "" && got.lensWhy == "" && replyPromisesACard(got.Reply) {
		got.lensRetry = true
		got.lensRetryWhy = "the turn gives a lens but the words describe a board"
	}
	// 🚨 递透镜的那一轮，话不要以一个问句收尾。
	//
	// 透镜自己就是那句「请她做什么」：她要去文章里点一句。话里再抛一个问题，
	// 屏幕上就有了两件事，而它们要的动作不一样 —— 一个要她点句子，一个要她
	// 打字。实测她逐字报的：
	//   「它让我用『经济学透镜』在第12段里找一句，看哪个成本被漏掉了。但它又说
	//     『这一步要在文章里做』，我不知道到底是要我从第12段 pick 一句英文，
	//     还是在下面那个框里用中文写答案。」
	// 示范照做（上面那条），收尾用陈述句把手交给她。
	if got.Lens != "" && got.lensWhy == "" && !got.lensRetry && replyEndsOnAQuestion(got.Reply) {
		got.lensRetry = true
		got.lensRetryWhy = "the lens turn ends on a question, which asks her to type instead of pick"
	}
	// 🚨 发卡片的那一轮，话也不要以一个问句收尾 —— 同一条理由，另一件器械。
	//
	// 产品负责人 2026-09-17 逐字报的：「卡片内容上的要求和对话窗口的文本要求
	// 不一致。」截图里 印记 在话里问「你觉得古训和俗话，跟第 3 句在让人相信
	// 这件事上有什么不一样？」，而它同一轮递出去的卡片问的是「哪一句读起来最
	// 不像在讲道理，更像在讲一个不变的事实？」—— 两道题，两种措辞，她不知道
	// 该答哪一个。
	//
	// 这一轮的问题由**卡片**承担（卡片就在这句话底下，她点得到）；话负责接住
	// 她上一句、说清这一步为什么值得做，然后把手交出去。
	if got.Card != nil && got.cardWhy == cardOK && !got.lensRetry && replyEndsOnAQuestion(got.Reply) {
		got.twoAsks = true
	}
	// 🚨 讲完就停、什么也没请她做的那一轮，也算这一轮坏了。
	// 她屏幕上只剩一句讲完的话和一个灰着的发送键，而她不知道该等还是该点。
	//
	// 🚨 推进了一步**不算**给了她事做。第一版在这里加了 Advance == ""，于是
	// 「你选得准，我们进到下一段」这种一句话的收尾照样溜过去 —— 而下一步要她
	// 先开口，她手上却没有任何东西可说。实测她逐字报的：「它说我选得准、推进到
	// 下一段了，但是下面没有任何新题目或者按钮让我继续，发送也按不动。」
	// 推进和交给她一件事，是这一轮要同时做的两件事。
	if (got.cardWhy == cardOK || got.cardWhy == cardRejectNoCard) &&
		got.Card == nil && got.Lens == "" &&
		(!replyAsksForSomething(got.Reply) || replyOnlyAsksHerToRead(got.Reply)) {
		got.cardWhy = cardRejectDeadTurn
	}
	if got.cardWhy != cardOK && got.cardWhy != cardRejectNoCard {
		slog.Info("reading coach: card dropped", "why", string(got.cardWhy),
			"type", cardType, "prompt", cardPrompt)
	}
	if got.lensWhy != "" {
		slog.Info("reading coach: lens dropped", "why", got.lensWhy)
	}
	// 铁律③「一次只问一个」：透镜和卡片都是把这一步交回她手上。两个一起弹到
	// 屏幕上，她第一件要做的事就变成了「先做哪个」—— 那是我们替她制造的分心。
	//
	// 丢卡片、留透镜：透镜是更重的教学器械（召唤 → 选句 → 评估 → 发现一整套
	// 流程），已经落进 atom_card 并受 atom_card_one_open_idx 约束；卡片是轻的，
	// 只在这条消息上，下一轮再给一张没有任何损失。
	//
	// 这一步**必须排在两个校验之后**：透镜自己没活下来（比如没给 focusBlock）
	// 的那一轮，她只剩卡片一件教具，「一次只问一个」本来就满足了，
	// 没有理由让卡片跟着陪葬。
	if got.Lens != "" && got.Card != nil {
		got.Card = nil
		// 🚨 第三个静默丢弃点，而且它在上面那两条日志**之后**，所以之前一次都
		// 没被记下来过。模拟学生走查（2026-09-10，第三轮）就死在这儿：
		// 印记 说「现在我给你一张卡片，把这三层的关系看清楚」，卡片被这一行拿掉，
		// 她在屏幕上找了三轮那张卡，最后 stuck。日志里干干净净。
		//
		// 丢卡片仍然是对的（铁律③：一次只问一个），但她那边少了一样它刚说过的
		// 东西，所以照样要说 —— 走的是和另外两种同一条路。
		got.cardWhy = cardRejectLensWon
		slog.Info("reading coach: card dropped", "why", string(got.cardWhy))
	}
	return got, true
}

// enforceLensDoneTurn is the code half of the system prompt's 「她刚做完一副
// 透镜的那一轮」 rule: she just handed something in, so this turn must not
// hand her something new.
//
// 🚨 Why this is code and not only prose. The prompt already says 「lens、card
// 都留空」. Running the real model three times through the real prompt
// (TestLiveLensDoneReplyParses, 2026-09-04) gave: 3/3 parsed, 3/3 advanced
// with "done", the words themselves good — and **2 of 3 attached a card
// anyway**. Which is not mysterious: the same prompt carries a STRONGER
// standing default ("give a card for an active step"), and when two rules
// collide a model follows the louder one.
//
// That is exactly what [[prompt-output-must-be-verifiable-2026-09-03]] is
// about — a 「必须」 in a prompt has to be checkable in code. So this rule
// lands here, structurally identical to the parser's own 「已给 lens 就丢卡片」
// (铁律③): drop the lighter instrument so the thing she just finished is what
// gets seen.
//
// Only `Card` and `Lens` are dropped. `Reply`, `Advance` and `FocusBlock` are
// precisely what this turn SHOULD carry: catch the sentence she picked, settle
// the step, walk her to the next one.
//
// A pure function rather than four lines inline, so the live test can run the
// same rule production runs instead of asserting on an approximation of it.
func enforceLensDoneTurn(got readingCoachReply, lensDone *readingLensDone) readingCoachReply {
	if lensDone == nil {
		return got
	}
	got.Card = nil
	got.Lens = ""
	return got
}

// postReadingCoachTurn is POST /api/v1/readings/{id}/coach.
//
// One guided turn. `text` empty means she pressed 开始 — the coach introduces
// the plan and leads her into step one. A plan is generated on demand if there
// isn't one yet, so 开始 is genuinely the only button she needs.
func (a *API) postReadingCoachTurn(w http.ResponseWriter, r *http.Request) {
	at, ok := a.loadOwnedReadingAtom(w, r)
	if !ok {
		return
	}
	u, _ := UserFromContext(r.Context())
	entitled, eerr := HasEntitlement(r.Context(), u)
	if eerr != nil {
		httpx.WriteError(w, r, eerr)
		return
	}
	if !entitled {
		httpx.WriteError(w, r, httpx.ErrNotEntitled())
		return
	}

	var req struct {
		Text  string        `json:"text"`
		Picks []readingPick `json:"picks"`
		// CardAnswer is set when this turn IS a tap on the card 印记 wrote into
		// its last reply. It is not a substitute for `text` — she may tap and
		// type in the same turn — and it never arrives as free-floating words:
		// the choice is checked against the article before it is allowed near
		// the transcript.
		CardAnswer *coachCardAnswer `json:"cardAnswer"`
		// LensDone is set when the ROOM is reporting that she finished a lens,
		// rather than her saying something. Carries no authority of its own:
		// it only ever lands in the prompt (see readingLensDone), never in the
		// transcript, and never marks a step done by itself — 印记 still
		// decides `advance`.
		LensDone *readingLensDone `json:"lensDone"`
	}
	if err := decodeJSON(r, &req); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	studentText := strings.TrimSpace(req.Text)
	// Trimmed here so `clean()` and every prompt read below agree, and so a
	// client sending whitespace cannot buy a turn.
	lensDone := req.LensDone
	if !lensDone.clean() {
		lensDone = nil
	}

	src, err := a.d.Queries.GetReadingSource(r.Context(), at.ID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	blocks := SplitBlocks(src.Body)
	if len(blocks) == 0 {
		httpx.WriteError(w, r, httpx.ErrBadRequest("empty_article", "这篇还没有正文，先把文章贴进来。", nil))
		return
	}
	picks := validateReadingPicks(req.Picks, blocks)

	// A tapped answer becomes a REAL turn: one student message, written into
	// the same transcript everything else reads. Anything less and her answer
	// is invisible — buildReadingCoachPrompt would not see it next turn, and a
	// refresh would show 印记 asking a question she had already answered.
	//
	// studentContent is what actually gets stored and what the model is shown,
	// deliberately the same string: what the coach reads this turn is exactly
	// what the next turn will read back out of 【你们刚才聊的】.
	studentContent := studentText
	var studentPayload []byte
	// 这一轮是她在段落工具底下写的那一段（想一想 / 仿写），不是清单上这一步的作业。
	toolAnswerTurn := req.CardAnswer != nil && strings.TrimSpace(req.CardAnswer.Type) == blockToolAnswerType
	// An answer with nothing in it is not a turn: composing on the prompt alone
	// would store a student message that is only 印记's own question.
	if ca := req.CardAnswer; ca != nil && (normalizeCardAnswerText(ca.Choice) != "" || studentText != "") {
		answer := &coachCardAnswer{
			Type: strings.TrimSpace(ca.Type),
			// The client's own copy of 印记's question, held to the same cap
			// validateCoachCard holds the card to — so what is stored is what
			// the room will re-render, and neither can be an essay.
			Prompt:  collapseCardPrompt(ca.Prompt),
			Choice:  normalizeCardAnswerText(ca.Choice),
			BlockID: strings.TrimSpace(ca.BlockID),
		}
		quote, ownFromChoice, pointed := cardAnswerChoiceParts(answer.Choice, answer.BlockID, blocks)
		picks = dedupeReadingPicks(append(picks, pointed...))
		own := studentText
		if ownFromChoice != "" {
			if own != "" {
				own = ownFromChoice + "\n\n" + own
			} else {
				own = ownFromChoice
			}
		}
		if content := composeCardAnswerMessage(answer.Prompt, quote, own, blocks...); content != "" {
			studentContent = content
			studentPayload = coachCardAnswerPayload(answer)
		}
	}

	turnCtx, cancel := context.WithTimeout(context.WithoutCancel(r.Context()), 150*time.Second)
	defer cancel()

	// A plan on demand: 开始 is the only button, so pressing it with no plan
	// yet must produce one rather than refusing. This is the ONLY caller that
	// plans implicitly — the explicit endpoint stays for 重排.
	tasks, err := a.d.Queries.ListReadingTasks(turnCtx, at.ID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if len(tasks) == 0 {
		if tasks, err = a.planReadingTasks(turnCtx, u.ID, at.ID, src, blocks); err != nil {
			httpx.WriteError(w, r, err)
			return
		}
	}

	msgs, err := a.d.Queries.ListAtomMessages(turnCtx, at.ID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}

	// 🚨 她按了「给点提示」的那一轮，屏幕上那张卡片原地不动。
	//
	// 产品负责人 2026-09-20 报的第 1 条：「对于一个卡片上的交互也没有进行管理
	// （比如可以就一张卡片一直点提示一下，使得论文阅读流程卡住，无法进行下一步）」。
	// 系统说明里第 3 条本来就写着「沿用那张卡，card、lens 留空，不推进」，而模型
	// 该发还是发 —— 每按一次提示换一张新卡，她写了一半的草稿跟着那张卡一起没了，
	// 清单却一步都没动。提示词里的软话跨不过代码里的硬判据
	// （[[prompt-twice-then-make-it-checkable-2026-09-12]]），所以判据在这里。
	//
	// 唯一的例外是**辅助题**：三次提示之后，一道要她自己写／自己去指的开放题可以
	// 换成一道选择题（见 helpRequestSection）。那是同一件事换个问法，不是新的一题。
	helpTurn := isCoachHelpAsk(studentText) && req.CardAnswer == nil
	openCardNow := lastOpenCard(msgs)
	hintRound := coachHintRound(msgs)
	assistAllowed := helpTurn && openCardNow != nil && hintRound >= coachHintCap && openCardIsOpenForm(openCardNow)
	helpHoldsCard := helpTurn && openCardNow != nil
	// 她交的是一道辅助题的答案 —— 答完（而且这一步没走完）就回到原题。
	answeredAssist := req.CardAnswer != nil && !toolAnswerTurn && openCardNow != nil && openCardNow.Assist

	// §model-routing · dialogue.
	//
	// 🚨 This is the single biggest deliberate downgrade of the 2026-09-02 class
	// migration, and it is a HYPOTHESIS, not a settled decision. The previous
	// note here argued flagship: "leading someone through a text — deciding
	// whether what she just said actually counts as having done this step — is
	// judgement, not conversation." That reasoning is real. What it was weighed
	// against, when there were only three lanes, was a chaperone lane that also
	// served the ordinary chat turn — so "some judgement" could only be bought
	// by buying the never-downgrade reviewer, full reasoning and all.
	//
	// The cost of that: every turn a student waits through in the lite reading
	// room ran at flagship price with reasoning at max. dialogue is the class
	// that says "she is watching this land, so it has 4 seconds" — and the
	// judgement it must still make is what routebench's dialogue judge case
	// scores. If a chaperone-class model cannot tell "she did the step" from
	// "she did not", this call goes back up to review and the win is given back.
	resolved, okResolve := a.route(turnCtx, gateway.ClassDialogue)
	if !okResolve {
		slog.Warn("reading coach: no provider resolved",
			"atom_id", at.ID, "request_id", httpx.RequestIDFromContext(r.Context()))
		httpx.WriteError(w, r, httpx.ErrAIDialogueFailed("model_unavailable"))
		return
	}
	// 🚨 这几样要在**调用模型之前**算出来：透镜开着这件事得进 prompt。
	//
	// 2026-09-17 之前它们排在模型调用之后，只用来事后驳回一副多余的透镜 ——
	// 于是「她屏幕上正开着一副透镜」这个事实模型完全不知道，照常往下领新的一步。
	// 这一栏本来是锁住的，所以看不出来；输入框放开之后（她卡在找不到句子的时候
	// 得能开口求助），它立刻就会露出来。
	cardRows, err := a.d.Queries.ListAtomCards(turnCtx, at.ID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	deck, deckErr := agent.ReadingDeck()
	ordering := readingOrderingGuard(cardRows)
	anyOpen := false
	openLensName := ""
	for _, c := range cardRows {
		if c.Status == "proposed" || c.Status == "active" {
			anyOpen = true
			if spec, found := cards.ByID(c.CardID); found {
				openLensName = spec.Name
			}
			break
		}
	}

	lang := readingLangOf(src.Body)
	system := buildReadingCoachSystem(lang)
	chatReq := gateway.ChatRequest{
		Messages: []gateway.ChatMessage{
			{Role: gateway.RoleSystem, Content: system},
			{Role: gateway.RoleUser, Content: buildReadingCoachPrompt(src.Title, blocks, decodeOutline(src.Outline), tasks, msgs, picks, studentContent, lensDone, openLensLine(anyOpen, openLensName)+toolAnswerLine(toolAnswerTurn))},
		},
	}
	res, cerr := gateway.Collect(turnCtx, a.d.Provider, resolved, chatReq)
	a.recordLiteLLMCall(turnCtx, u.ID, at.ID, "reading_coach", resolved, res.Usage)
	if cerr != nil {
		slog.Warn("reading coach: provider call failed", "err", cerr,
			"atom_id", at.ID, "request_id", httpx.RequestIDFromContext(r.Context()))
		httpx.WriteError(w, r, httpx.ErrAIDialogueFailed("model_unavailable"))
		return
	}
	// The coach may only reach for a lens the room could actually open right
	// now. Checking here rather than after the call means a refused summon
	// never reaches her as a card that silently failed to appear.
	// 🚨 这份清单里有没有「深入思考」那一步。
	//
	// 产品负责人 2026-09-17：「at this stage, I think we can skip the 透镜 part.
	// it is really not applicable in many papers. and difficult for students to
	// understand. the above mentioned critical thinking can be a better
	// replacement of lens.」
	//
	// 读法库里已经没有 lens 那一步了（critique 顶掉了它），但**机器还在**：
	// prompt 里仍然有透镜那一节，模型仍然可能顺手召一副。所以判据放在这里 ——
	// 清单上没有这一步，就一副都不给。
	//
	// 这样写而不是把整套透镜删掉，是因为「at this stage」：哪天读法库里再排上
	// lens，它原样就能用；而在那之前，她一副也碰不到。
	planHasLens := false
	for _, t := range tasks {
		if t.Kind == string(taskLens) {
			planHasLens = true
			break
		}
	}
	lensOK := func(id string) bool {
		if deckErr != nil || anyOpen || !planHasLens || !inReadingDeck(deck, id) {
			return false
		}
		if id == "sift" && !ordering.AllowSift {
			return false
		}
		if id == "craap" && !ordering.AllowCraap {
			return false
		}
		return true
	}
	parsed, okParse := parseReadingCoachReply(res.Text, blocks, lang, lensOK)
	genre := decodeOutline(src.Outline).Genre
	// An open board is already the component mentioned in the reply; do not
	// retry or add another card merely because the model refers to it.
	boardOpen := openBoard(msgs) != nil && !answeredBoard(req.CardAnswer)
	if okParse && boardOpen && parsed.cardWhy == cardRejectPromised {
		parsed.cardWhy = cardRejectNoCard
	}
	// 🚨 让她把一张卡挪到它**已经在**的那一格，是一条她做不到的指令。
	//
	// 她摆完的结果原样在转写里（「限制：」加上那一句），所以这是它没读，不是
	// 我们没给。实测她逐字报的：「它让我把发电机那句拖到限制格，但那句已经在
	// 限制格里了……屏幕上显示的摆放和它文字描述的矛盾了，我没法确定该怎么挪。」
	//
	// 判据要同时满足三件事，才不会误伤一句正常的肯定
	// （「你把发电机那句放进限制，这个判断很准」是对的，不能拦）：
	// 话里有一个「挪」的动词 + 提到了那一句 + 点了那一格的名字。
	if okParse && replyAsksForANoOpMove(parsed.Reply, lastBoardPlacement(msgs)) {
		parsed.lensRetry = true
		parsed.lensRetryWhy = "the reply asks her to move a card into the bin it is already in"
	}
	if okParse && !parsed.lensRetry && answeredBoard(req.CardAnswer) && replyAsksToMoveOnABoard(parsed.Reply) {
		parsed.lensRetry = true
		parsed.lensRetryWhy = "the board was just submitted and is gone; the reply still asks her to move a card"
	}
	// 🚨 一篇文章里这块板只摆一次 —— 加上一次改正的机会。
	//
	// 产品负责人 2026-09-17 逐字：「only one such practice in one paper is
	// enough. (in my just finished paper, I repeated at least four times.
	// although three of them are the same one)」
	//
	// 四次里有三次是同一块板：她摆完，印记 觉得有一两张放错，就再发一块让她
	// **从头摆一遍**。改正是好的，从头摆不是 —— 那三次她做的是同一件事。
	//
	// prompt 里已经写了这条（taskLabel 的判据），但散文跨不过判据
	// （[[reading-room-rulings-2026-09-17]] 第一条），所以这里数出来：
	// 这篇文章里已经发过 maxLabelBoards 块板，第 maxLabelBoards+1 块丢掉。
	if okParse && parsed.Card != nil && parsed.cardWhy == cardOK &&
		parsed.Card.Type == coachCardLabelRoles && countLabelBoards(msgs) >= maxLabelBoards {
		parsed.Card = nil
		parsed.cardWhy = cardRejectBoardRepeat
		slog.Info("reading coach: card dropped", "why", string(cardRejectBoardRepeat),
			"atom_id", at.ID, "boards", countLabelBoards(msgs))
	}
	// 🚨 排序板只属于报道和记叙（genreHasOrderBoard），而且一篇只摆两块 ——
	// 和标注板同一条上限，同一个理由。
	//
	// 2026-09-17 之前这里挡的是另一件事：报道和记叙上不许摆「主张 / 证据」板。
	// 那三种体裁现在各有自己的格子（fitBoardToGenre），板本身不再需要挡。
	if okParse && parsed.Card != nil && parsed.cardWhy == cardOK &&
		parsed.Card.Type == coachCardOrderEvents &&
		(!genreHasOrderBoard(genre) || countCards(msgs, coachCardOrderEvents) >= maxLabelBoards) {
		parsed.Card = nil
		parsed.cardWhy = cardRejectOrderNotHere
		slog.Info("reading coach: card dropped", "why", string(cardRejectOrderNotHere),
			"atom_id", at.ID, "genre", genre)
	}
	// 🚨 走到「排出事件顺序」那一步，第一块排序板必须真的到她屏幕上。
	//
	// 这一步的全部内容就是那块板，和标注步一样（reading_coach_board_build.go 的
	// 文件头）。模型没给，先重来一次（下面那个 if），还没有就由服务端摆
	// （buildOrderBoard）。
	seqCur := currentReadingTask(tasks)
	needOrderBoard := seqCur != nil && seqCur.Kind == string(taskSequence) &&
		genreHasOrderBoard(genre) && !anyOpen && !toolAnswerTurn &&
		!answeredOrderBoard(req.CardAnswer) && countCards(msgs, coachCardOrderEvents) == 0
	if okParse && needOrderBoard && parsed.Lens == "" &&
		(parsed.Card == nil || parsed.Card.Type != coachCardOrderEvents) {
		parsed.cardWhy = cardRejectNoOrderBoard
	}
	// 🚨 引了一句文章上、卡片上、她嘴里都没有的话。产品负责人 2026-09-17 报的
	// 第 2 条，见 reading_ghostquote.go。
	//
	// 判在这里而不在 parseReadingCoachReply 里：语料要用到转写和导读，
	// 而那两样解析器拿不到（它只看得见这一段 JSON 和文章）。
	if okParse {
		parsed.ghostQuote = firstGhostQuote(parsed.Reply, readingQuoteCorpus(
			src.Title, blocks, decodeOutline(src.Outline), tasks, msgs, parsed.Card, lensDone))
	}
	// 🚨 说漏嘴的两种（产品负责人 2026-09-17 第三轮走查，见
	// reading_coach_leak.go）：协议词写进了正文，或者当面把她叫成「她」。
	if okParse {
		if w := firstProtocolLeak(parsed.Reply); w != "" {
			parsed.leak = "the reply prints the protocol word " + w + " where she can read it"
		} else if replyCallsHerShe(parsed.Reply, blocks) {
			parsed.leak = "the reply calls her 「她」 to her face"
		} else if shipsOrderBoard(parsed, needOrderBoard) {
			// 🚨 发排序板那一轮，话里先把顺序背了一遍 —— 板子就是答案。
			// 见 reading_order_recital.go。
			if s := firstOrderAnswerRecital(parsed.Reply); s != "" {
				parsed.leak = "the reply recites the order on the turn that hands out the order board: " + s
			}
		}
	}
	// 🚨 「说了给卡片，却没给」也算这一轮坏了，和解析失败一样，也用同一条退路：
	// 再问一次。
	//
	// 这是模拟学生走查里唯一一个反复挡住整条链子的东西：印记 在话里说
	// 「现在给你一张卡片」「用一张卡片收」，JSON 里却没有 card，她屏幕上什么都
	// 没有 —— 于是她在那儿找卡片，找不到就 stuck。四条走查死在这上面。
	//
	// 把理由喂给下一轮是对的，但**救不了这一轮**：她这一轮看到的仍然是一句指着
	// 空气的话，而她往往就在这一轮放弃了。所以在把它交给她之前先重来一次。
	//
	// 只重来一次，而且失败了就照常往下走（她拿到那句话，没有卡片）——
	// 不编、不改写模型的话（[[ai-errors-must-surface-never-fake]]）。
	// 🚨 被校验刷掉的卡片也要立刻重来一次，不只是「说了卡却没给」那一种。
	//
	// 把理由留给下一轮，救不了这一轮：她这一轮看到的是 印记 说「我给你一张卡」
	// 而屏幕上什么都没有。线上逐字证据（atom 609f3910，2026-09-11）：seq 36
	// 的卡因为选项全来自同一段被丢掉，seq 42 的卡因为引文对不上被丢掉 ——
	// 中间 印记 连着两轮道歉「卡没送到你手里」，她连着两轮回「没有卡啊」。
	// 产品负责人报的第 5 条就是这两轮。
	// 反馈那一轮「说完就停」是对的，不是一轮死掉的话 —— 不为它花一次重试。
	if okParse && toolAnswerTurn && parsed.cardWhy == cardRejectDeadTurn {
		parsed.cardWhy = cardRejectNoCard
	}
	// 🚨 提示那一轮，**任何和卡片有关的驳回都不买重试**。
	//
	// 理由是结构性的：这一轮她手上那张卡还开着，而下面那道闸无论如何都会把新卡
	// 丢掉 —— 为一张注定不会到她屏幕上的卡再打一次模型，是纯粹的浪费，而且重来
	// 那一次收到的指令（「结尾要么给一张卡片，要么明确请她做一件事」「要给就真的
	// 给」）正好和「不要再发新卡片」顶上。理由还会存进 payload，下一轮当面告诉它
	// 「你递出去的东西没到她屏幕上、不要再提这张卡」—— 而那张卡她正看着。
	//
	// 2026-09-20 线上走查两次量到，两次是**不同的**驳回理由：先是
	// cardRejectDeadTurn（「这一轮什么都没给她做」—— 她手上有卡，这是假的），
	// 修掉之后换成 cardRejectPromised（印记 说「这张卡」指的就是她开着的那张，
	// 被读成「许诺了一张新卡却没给」）。一条一条补下去补不完，所以判据放在
	// 「这一轮的卡片注定落不了地」这件事上，而不是放在某一个理由上。
	// （同一条推理上面已经有过一次：openBoard 那块板开着时也豁免 cardRejectPromised。）
	//
	// 🚨 只豁免卡片那一列。引了文章里没有的话、把协议词写给她看、一轮里问两件事
	// —— 这些是**这句话本身**坏了，提示轮同样要重来一次。
	//
	// assistAllowed 例外：那一轮我们**真的想要**一张好的选择题，所以照常重来。
	if okParse && helpHoldsCard && !assistAllowed &&
		parsed.cardWhy != cardOK && parsed.cardWhy != cardRejectNoCard {
		slog.Info("reading coach: help turn, a card-shaped rejection does not buy a retry",
			"atom_id", at.ID, "why", string(parsed.cardWhy))
		parsed.cardWhy = cardRejectNoCard
	}
	if okParse && readingCoachReplyNeedsRetry(parsed) {
		why := string(parsed.cardWhy)
		if parsed.lensRetry {
			why = parsed.lensRetryWhy
		} else if readingCoachSettlesWithTool(parsed) {
			why = "the turn settles the current step but hands out the next tool"
		}
		if parsed.twoAsks {
			why = "the turn hands her a card AND ends its words on a different question"
		}
		if parsed.ghostQuote != "" {
			why = "the reply quotes a sentence that is not in the article, on the card, or in her own words: " + parsed.ghostQuote
		}
		if parsed.leak != "" {
			why = parsed.leak
		}
		slog.Warn("reading coach: reply looks broken, retrying once",
			"why", why,
			"atom_id", at.ID, "request_id", httpx.RequestIDFromContext(r.Context()))
		if retryRes, retryErr := gateway.Collect(turnCtx, a.d.Provider, resolved, chatReq); retryErr == nil {
			a.recordLiteLLMCall(turnCtx, u.ID, at.ID, "reading_coach", resolved, retryRes.Usage)
			// 只在第二次**确实更好**的时候采用它：解析得动，而且不再是一句空话。
			// 否则留着第一次那份 —— 它至少是完整的一句话。
			// 第二次只在**它确实更好**的时候采用：解析得动，而且没有被判失败。
			if again, ok2 := acceptableReadingCoachRecovery(retryRes.Text, blocks, lang, lensOK); ok2 &&
				!again.lensRetry && !again.twoAsks &&
				firstProtocolLeak(again.Reply) == "" && !replyCallsHerShe(again.Reply, blocks) &&
				// 第二张排序板也一样要过体裁那一关，否则「重来一次」只是把同一张
				// 不适用的板又发了一遍。
				!(again.Card != nil && again.Card.Type == coachCardOrderEvents && !genreHasOrderBoard(genre)) &&
				// 第二次还是把顺序背了一遍，就不算「确实更好」—— 留着第一次那份，
				// 下面那一刀照样会把那句话拿掉。
				!(shipsOrderBoard(again, needOrderBoard) && firstOrderAnswerRecital(again.Reply) != "") &&
				firstGhostQuote(again.Reply, readingQuoteCorpus(
					src.Title, blocks, decodeOutline(src.Outline), tasks, msgs, again.Card, lensDone)) == "" {
				res, parsed = retryRes, again
			}
		}
	}
	// 重来之后还是没有排序板：服务端摆一块（buildOrderBoard）。
	// cardRejectNoOrderBoard 只是「该重来」的信号，不是一张被丢掉的卡 ——
	// 不清掉的话，下一轮会被告知「你上一轮的东西没到她屏幕上」，而那张
	// choose_span 明明到了。
	if okParse && parsed.cardWhy == cardRejectNoOrderBoard {
		if parsed.Card != nil && parsed.Card.Type == coachCardOrderEvents {
			parsed.cardWhy = cardOK
		} else if built := validateCoachCard(buildOrderBoard(blocks, decodeOutline(src.Outline).Parts), blocks); built != nil && parsed.Lens == "" {
			slog.Info("reading coach: sequence step had no order board, built one",
				"atom_id", at.ID, "options", len(built.Options))
			parsed.Card = built
			parsed.cardWhy = cardOK
		} else if parsed.Card != nil {
			parsed.cardWhy = cardOK
		} else {
			parsed.cardWhy = cardRejectNoCard
		}
	}
	if !okParse {
		// 🚨 ASK ONCE MORE. Measured 2026-09-04 against the live model
		// (TestLiveLensDoneReplyParses, 6 samples): **1 in 6 replies arrives
		// truncated** — the model writes a perfectly good reply, gets as far
		// as the trailing optional key and simply stops:
		//
		//	…"advance":"done","focusBlock":"","tool":"","lens":"","card":
		//
		// and the provider reports `stop_reason: "stop"`, i.e. a NORMAL
		// finish. So there is nothing to detect it by other than the parse
		// failing, and no client cap to raise: max_tokens is 16000 and the
		// reply was ~100.
		//
		// One retry turns a ~17% chance of 「AI 响应错误」 into ~3%. It is not
		// a workaround for a prompt problem — the reply that came back was
		// GOOD, it was cut off mid-serialisation — and it is not a fake
		// answer either ([[ai-errors-must-surface-never-fake]] forbids
		// inventing a plausible sentence; asking the model again is the
		// opposite of that). A second failure still surfaces honestly.
		//
		// This is a pre-existing defect of every reading-coach turn, not of
		// the lens-completion turn — it was simply never measured until a
		// live walk of the lens refeed hit it.
		slog.Warn("reading coach: reply unparseable, retrying once",
			"atom_id", at.ID, "stop_reason", res.StopReason,
			"request_id", httpx.RequestIDFromContext(r.Context()))
		res, cerr = gateway.Collect(turnCtx, a.d.Provider, resolved, chatReq)
		a.recordLiteLLMCall(turnCtx, u.ID, at.ID, "reading_coach", resolved, res.Usage)
		if cerr != nil {
			slog.Warn("reading coach: retry call failed", "err", cerr,
				"atom_id", at.ID, "request_id", httpx.RequestIDFromContext(r.Context()))
			httpx.WriteError(w, r, httpx.ErrAIDialogueFailed("model_unavailable"))
			return
		}
		parsed, okParse = parseReadingCoachReply(res.Text, blocks, lang, lensOK)
		if !okParse {
			// 🚨 把回复的尾巴带上。2026-09-08 排查这条 502 时，日志里有
			// atom_id、有 stop_reason，唯独没有**模型到底回了什么** —— 而那
			// 正是唯一能分辨「断在半路」和「答得不对」的东西，只能靠重跑一遍
			// 实测去猜。尾巴 200 字，够看清断点在哪个字段上。
			slog.Warn("reading coach: reply unparseable after retry",
				"atom_id", at.ID, "stop_reason", res.StopReason,
				"reply_len", len(res.Text), "reply_tail", tailRunes(res.Text, 200),
				"request_id", httpx.RequestIDFromContext(r.Context()))
			httpx.WriteError(w, r, httpx.ErrAIDialogueFailed("model_unavailable"))
			return
		}
	}

	parsed = enforceLensDoneTurn(parsed, lensDone)

	// The student turn, the coach turn, and the step advance land together.
	// Split, a crash between them leaves the transcript saying one thing and
	// the plan another — and the plan is what the next turn reads to decide
	// where she is.
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
	seq, err := qtx.NextAtomMessageSeq(turnCtx, at.ID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	// 🚨 studentContent, not studentText: a PURE tap types nothing, and the old
	// `studentText != ""` gate wrote no student row at all. Her answer would
	// have been missing from the next turn's context and gone after a refresh.
	// The tap's payload rides along so the room can re-render the card she
	// already answered instead of one still waiting for her.
	if studentContent != "" {
		if _, err := qtx.AppendAtomMessage(turnCtx, sqlc.AppendAtomMessageParams{
			AtomID: at.ID, Seq: seq, Role: "student", Content: studentContent,
			Payload: studentPayload,
		}); err != nil {
			httpx.WriteError(w, r, err)
			return
		}
		seq++
	}
	// 🚨 走到「标注论证」这一步而模型没给板 —— 服务端自己摆一块。
	//
	// 这一步的**全部内容**就是那块板。而实测下来模型一遍遍在话里说「把这三句
	// 拖到格子里」却不附 card：十条走查里这是唯一反复挡住整条链子的东西。
	// 检测、重试、把理由喂回去，都只是降低概率，她还是会撞上「屏幕上根本没有板」。
	//
	// 这一步不需要模型来决定「有没有板」：读法库已经规定了它是标注论证
	// （reading_routines.go：步骤由 routine 拥有）。挑哪几句仍然优先用它的判断，
	// 它没给才用确定性的规则兜底 —— 和排读法那条链子是同一个分工。
	//
	// 🚨 兜底出来的那块板照样送进 validateCoachCard：它不是一条绕过校验的后门，
	// 句子逐字来自正文，本来就过得了。
	// 🚨 两种情况都要摆板：走到标注论证那一步，**或者**它嘴上说了板却没附。
	//
	// 后一种是实测反复出现的那一幕：她刚把板交上去（板随即从屏幕上收走），
	// 印记 接着说「把这句挪到主张旁边」「再拖一张过去」，她照着做时屏幕上什么
	// 都没有。prompt 里写了「想让她再摆一次就重新发一块新的板」，它不照做。
	// 写了两版规矩都不管用之后，改成：它说了，我们就真的给她一块。
	// 🚨 她屏幕上正开着一副透镜的时候，这一轮什么器械都不再递，也不推进。
	//
	// 这条路 2026-09-17 才成立：在那之前透镜开着时输入框是锁死的，她根本没法
	// 开口，所以这一轮不会发生。放开输入框（找不到句子的时候她得能求助）之后，
	// 一句「我找不到这样的句子」就会走到这里 —— 而模型看不见屏幕，它默认的反应
	// 是接着往下领一步，把那副还等着她的透镜甩在后面。
	// 🚨 她在段落工具（想一想 / 仿写）底下写的那一段，**不推进这一步**。
	//
	// 那是一次旁支练习 —— 她在第 3 段上顺手仿写了一段，而清单此刻可能停在
	// 「你怎么看」。模型会把「她交了一段话」读成「这一步的作业交了」，给 done，
	// 于是她一个字没写那一步就过去了。判据在这里，不在提示词里
	// （[[reading-room-rulings-2026-09-17]]：散文跨不过判据）。
	if toolAnswerTurn {
		parsed.Advance = ""
		// 这一轮是反馈，不是再出一道题。
		parsed.Card = nil
		parsed.Lens = ""
	}
	// 🚨 提示那一轮：她手上那张卡留着，这一步不推进。见上面 helpHoldsCard。
	if helpHoldsCard {
		if parsed.Card != nil && assistAllowed && parsed.Card.Type == coachCardChooseSpan {
			// 辅助题：同一件事换成一道选择题。标记由服务端盖，模型给不了。
			parsed.Card.Assist = true
			slog.Info("reading coach: three hints in, an open-form card was swapped for a choice card",
				"atom_id", at.ID, "was", openCardNow.Type)
		} else if parsed.Card != nil {
			// 🚨 丢得**不留理由**（cardRejectNoCard 那条路，payload 里什么都不写）。
			// 别的丢卡理由下一轮会当面告诉模型「你递出去的东西没到她屏幕上，
			// 不要再提这张卡」—— 而这一轮那句话是假的：她屏幕上的卡好好地在。
			slog.Info("reading coach: help turn, her open card stays and the new one was dropped",
				"atom_id", at.ID, "type", parsed.Card.Type, "round", hintRound)
			parsed.Card = nil
			parsed.cardWhy = cardRejectNoCard
		}
		parsed.Lens = ""
		parsed.Advance = ""
	}
	if anyOpen {
		if parsed.Card != nil {
			parsed.Card = nil
			parsed.cardWhy = cardRejectLensWon
			slog.Info("reading coach: card dropped, a lens is already open on the article",
				"atom_id", at.ID)
		}
		// 「跳过」照常放行 —— 她说不做了就是不做了（铁律④），而且卡片上那个
		// 跳过本来就在。只拦「做完了」：那副透镜还开着，这一步就没做完。
		if parsed.Advance == "done" {
			parsed.Advance = ""
		}
	}

	// 🚨 `!anyOpen`：她屏幕上正开着一副透镜的时候，一张卡都不要再递。铁律③
	// 一次只问一个 —— 两件器械同时摆着，她第一件要做的事就变成了「先做哪个」。
	// 这条以前不需要，因为透镜开着时输入框是锁死的，这一轮压根不会发生；
	// 放开输入框（她得能在找不到句子的时候求助）之后它就是一条真的路了。
	// 🚨 `hasAuthorsArgument`：这块板摆的是「主张 / 证据 / 限制」，作者不表态的
	// 文章上它没有指称对象。en-report 那套读法里本来就没有标注步，但
	// `replyPromisesACard` 这条路在任何一步上都通 —— 兜底不该把一件刚被挡掉的
	// 事从后门放进来。
	// 🚨 什么时候兜一块板（2026-09-17 入口走查，两次）：
	//   - 「拆开作者的论证」那一步；
	//   - 别的步骤上，话里说的是**板**（拖到、角色、格子），而且没请她写。
	// 原来的条件是「标注步，**或者**话里提到了任何卡片」，于是在「先预测」那一步
	// 印记 说「写在下面这张卡上」，屏幕上出来一块「关键主张 / 证据」板 —— 话要她写，
	// 卡要她摆（第一批反馈第 2 条）。只收成「标注步」又矫枉过正：精读那一步印记说
	// 「把每一句拖到它该在的角色里」，屏幕上一块板都没有。
	// 板数上限对兜底一样生效 —— 模型那块被上限挡掉的板不能从这里再建出来。
	if cur := currentReadingTask(tasks); cur != nil && !anyOpen && !toolAnswerTurn && !helpHoldsCard && parsed.Card == nil && parsed.Lens == "" &&
		cur.Kind != string(taskSequence) && !boardOpen &&
		countLabelBoards(msgs) < maxLabelBoards &&
		(!answeredBoard(req.CardAnswer) || replyAsksToMoveOnABoard(parsed.Reply)) &&
		(cur.Kind == string(taskLabel) ||
			(replyPromisesABoard(parsed.Reply) && !replyAsksToWrite(parsed.Reply))) {
		focus := parsed.FocusBlock
		if focus == "" {
			focus = cur.BlockID
		}
		// 🚨 它嘴上说的那一段才是她正在看的那一段。
		//
		// focusBlock 和这一步自带的 BlockID 经常都是空的，兜底就从第 1 段取句子
		// —— 而 印记 那句话说的是「我们来摆第 5 段的这几句」。实测她逐字报的：
		// 「它说让我摆第5段的句子，但板上给的卡片全是第1段的。」
		//
		// 它对她只会说「第几段」（prompt 里明令不许说 b1/b2），所以从回复里把那个
		// 段号读回来，比任何字段都准。
		if spoken := spokenParagraph(parsed.Reply, blocks); spoken != "" {
			focus = spoken
		}
		if built := validateCoachCard(buildLabelBoardFromReply(blocks, focus, parsed.Reply), blocks); built != nil {
			slog.Info("reading coach: label step had no board, built one",
				"atom_id", at.ID, "options", len(built.Options))
			parsed.Card = built
			parsed.cardWhy = cardOK
		}
	}

	// 🚨 最后一道兜底：**它说了有卡，她屏幕上就必须有卡。**
	//
	// 产品负责人 2026-09-12 定的那条线：「不应该让用户有 bug 的感觉。要么不满足
	// 自己不调用，要么就是有兜底策略。」
	//
	// 到这里为止，一张卡可能已经被驳回两次（一次原始、一次重试），上面那块板也
	// 可能没摆成（不是标注那一步、或者这篇文章挑不出句子）。再往下走，她看到的
	// 就是 印记 说「我给你一张卡」而屏幕上什么都没有 —— 那正是她逐字说过的
	// 「没有卡啊」。
	//
	// 兜底只沿用印记写过的题，或当前步骤说明里的真问题；没有题就不制造一张
	// 不知道要做什么的卡。
	if !anyOpen && !toolAnswerTurn && !helpHoldsCard && !boardOpen && parsed.Card == nil && parsed.Lens == "" && replyPromisesACard(parsed.Reply) &&
		(parsed.askedPrompt != "" || req.CardAnswer == nil) {
		if fb := fallbackCardFor(parsed.askedPrompt, parsed.Reply, stepQuestion(currentReadingTask(tasks))); fb != nil {
			parsed.Card = fb
			parsed.cardWhy = cardOK
			slog.Info("reading coach: promised a card and had none, fell back",
				"atom_id", at.ID, "type", parsed.Card.Type, "kept_prompt", parsed.askedPrompt != "")
		}
	}

	// 🚨 两次都把协议词写进了正文，就把漏出来的那一句拿掉。
	//
	// 这不是改写它的话：被拿掉的那一句是**我们自己的脚手架**
	// （「这一步做完。advance给done。」的后半句），不承载任何教学内容，而她
	// 读到它只会以为屏幕坏了。见 stripProtocolLeak。
	if w := firstProtocolLeak(parsed.Reply); w != "" {
		parsed.Reply = stripProtocolLeak(parsed.Reply)
		slog.Info("reading coach: stripped a protocol word from the reply",
			"atom_id", at.ID, "word", w)
	}

	// 🚨 两次都在发板那一轮把顺序背了一遍，就把那一句拿掉。
	//
	// 同上一条：被拿掉的那一句不承载教学内容 —— 事情都在板子上，话里再数一遍
	// 只是把答案先说了。见 stripOrderAnswerRecital。
	if shipsOrderBoard(parsed, false) {
		if s := firstOrderAnswerRecital(parsed.Reply); s != "" {
			parsed.Reply = stripOrderAnswerRecital(parsed.Reply)
			slog.Info("reading coach: stripped the order recital from an order-board turn",
				"atom_id", at.ID, "sentence", s)
		}
	}

	parsed.Card = dropAlreadyPlaced(parsed.Card, lastBoardPlacement(msgs))

	// 标注板的格子换成这篇体裁的那一套。议论文原样不动。见 fitBoardToGenre。
	parsed.Card = fitBoardToGenre(parsed.Card, genre)

	// 🚨 排序板的选项在发出去之前打乱 —— 摆出来的顺序不能就是答案。
	// 放在这里是因为这是所有来路（模型给的、buildOrderBoard 兜底的、
	// replyPromisesACard 补的）汇合之后、落库之前的最后一处。见
	// reading_order_shuffle.go。
	parsed.Card = shuffleOrderOptions(parsed.Card)
	if current := currentReadingTask(tasks); current != nil {
		advance := protectedReadingCoachAdvance(parsed.Advance, current, req.CardAnswer, picks, msgs, blocks, studentText)
		parsed.Advance = advance
		parsed = enforceSettledReadingTurn(parsed)
		// F3: a hunt step settling on "done" must be backed by an actual point,
		// not an assertion the model was talked into accepting. "skipped" is
		// deliberately untouched — 铁律② means she can always decline a step by
		// saying so, and a guard that trapped her on the hunt would defeat the
		// whole point of that ruling.
		// 🚨 她把标注板摆完了，这一步就是做完了 —— 不由模型决定。
		//
		// 和上面那条 hunt 是同一件事的另一半：hunt 那条防的是「模型被说服了就
		// 推进」，这条防的是「她真的做完了，模型却不推进」。
		//
		// 实测（2026-09-11 模拟学生走查）：她把三张卡片全摆进格子、提交，印记
		// 回了一段很好的点评（「这三张贴得很干净……」），然后 advance 给了空 ——
		// 这一步永远停在那儿。130 步只走完 8 步里的 3 步，卡的就是这里。
		// prompt 里那一节明写着「这一步已经用这块板做完了，advance 给 done」，
		// 而它不照做，所以改由代码兜底
		// （[[prompt-output-must-be-verifiable-2026-09-03]]）。
		//
		// 这不是替她判对错：板上本来就没有对错，摆完这个动作本身就是这一步的
		// 产出，和 hunt 要求「真的点一句」是同一种判据。
		if current.Kind == string(taskLabel) && advance == "" && answeredBoard(req.CardAnswer) {
			advance = "done"
		}
		// 排序板同一条：排好交上来，这一步就做完了。
		if current.Kind == string(taskSequence) && advance == "" && answeredOrderBoard(req.CardAnswer) {
			advance = "done"
		}
		// 🚨 话里领她去读哪几段，清单就停在哪一步（reading_coach_align.go）。
		// 透镜开着、或者这一轮是段落工具的反馈时，上面已经把 done 拦下了，不动。
		if !anyOpen && !toolAnswerTurn {
			if aligned := alignAdvanceWithReply(tasks, advance, parsed.Reply); aligned != advance {
				slog.Info("reading coach: advance realigned with the reply",
					"atom_id", at.ID, "kind", current.Kind, "from", advance, "to", aligned)
				advance = aligned
			}
		}
		// 🚨 通读那一步的 done 要有真凭据 —— 见 reading_coach_readstep.go。
		//
		// 排在对齐之后：对齐用「印记 已经把她领去下一部分」这个信号把空的
		// advance 提成 done，而那正是这道闸认的三样凭据之一，所以两者不打架。
		// 排在耗满六轮之前：走不动了替她往下走是另一件事，那一条照常生效。
		if guarded := guardReadStepAdvance(advance, current, req.CardAnswer, studentText, parsed.Reply, tasks); guarded != advance {
			slog.Info("reading coach: a read step tried to settle with no evidence she read it",
				"atom_id", at.ID, "label", current.Label, "from", advance)
			advance = guarded
		}
		// 🚨 一步耗满六轮，我们替她往下走。
		//
		// 实测下来模型**极少主动推进**：一条 150 步的走查里，八步只走完两步，
		// 每一步都在「再找一句」「再说说看」之间来回。卡住提示（第 3 轮）给过
		// 之后再等三轮，还不动就是不会动了 —— 这正是通读那一步变成审问的那个
		// 形状，只是换了一步。
		//
		// 往下走**不等于**判她做完了：她在这一步里说过的每一句话都在转写里，
		// 过程评估读的是那个（铁律④）。把她钉在原地才是真的丢东西 ——
		// 她会直接关掉页面。
		//
		// hunt 那一步不在此列：它要的是「真的在文章里点一句」，而那个证据
		// 上面已经单独判过了，替她推进会把这一步唯一的保证也抹掉。
		// 🚨 提示那一轮不算在内：她按提示是在做这一步，不是在耗着它，而「按了三次
		// 提示这一步就自己过去了」正是产品负责人要拦的那件事（不推进阅读进度）。
		if advance == "" && current.Kind != string(taskHunt) && !helpTurn &&
			!readingCoachAnswerOnly(studentText) && coachStepStalled(tasks, msgs) {
			// 🚨 透镜那一步耗满了，先把敞开的透镜撤掉再往下走。
			//
			// 不撤的话她根本走不掉：透镜开着时这一栏是锁住的，而「往下走」只改了
			// 任务状态，屏幕上那副透镜还在，她还是只能对着它。
			//
			// 实测那一幕：印记 在一篇打仗救援的新闻上召了「科学方法论」的透镜，
			// 一遍遍要她找「样本不够、测量有偏差」的句子。她连着说了三遍
			// 「这篇根本没有做实验」，越说越烦 —— 而那样的句子确实不存在。
			// 一副套不上这篇文章的透镜，硬耗下去只会把她耗走。
			if current.Kind == string(taskLens) {
				for _, c := range cardRows {
					if c.Status == "proposed" || c.Status == "active" {
						if _, err := qtx.UpdateAtomCardStatus(turnCtx, sqlc.UpdateAtomCardStatusParams{ID: c.ID, Status: "skipped"}); err != nil {
							slog.Warn("reading coach: could not retire the stalled lens",
								"err", err, "atom_id", at.ID)
						}
					}
				}
			}
			slog.Info("reading coach: step stalled, advancing for her",
				"atom_id", at.ID, "kind", current.Kind)
			advance = "done"
		}
		parsed.Advance = advance
		parsed = enforceSettledReadingTurn(parsed)
		if advance != "" {
			if _, err := qtx.SetReadingTaskStatus(turnCtx, sqlc.SetReadingTaskStatusParams{
				AtomID: at.ID, ID: current.ID, Status: advance,
			}); err != nil {
				httpx.WriteError(w, r, err)
				return
			}
		}
	}
	// 🚨 回到原题：上一张是辅助题（coachCard.Assist），她答完了，而这一步还没走完
	// —— 于是她原来那张开放题从「已替换」回到可作答，连着她在上面写了一半的草稿
	// （产品负责人 2026-09-20：「恢复原题时保留已有草稿和辅助结果」）。
	//
	// 这一步真的走完了、或者这一轮又递了一张新卡，就不回去：那时回去的是一道
	// 已经不在问的题。
	restored := answeredAssist && parsed.Advance == "" && parsed.Card == nil && parsed.Lens == ""
	if restored {
		slog.Info("reading coach: assist card answered, the original card comes back", "atom_id", at.ID)
	}

	// 这一轮领她去看哪一段。
	//
	// 🚨 **在写消息之前算**，而且响应里用的是同一个值。她屏幕上那颗「跳到第 N 段」
	// 是从这条消息的 payload 读的（刷新之后还在），乐观更新读的是响应 ——
	// 两边必须是同一个事实，否则刷新一下按钮会指向另一段。
	//
	// 规则和它原来在提交之后那一版一样：这一轮真的召出了一副透镜，就停在透镜
	// 落的那一段（`parseReadingCoachReply` 要求两者配对）；否则清单自己那一步
	// 说的段落胜过模型的随手一猜 —— 排读法的时候就已经决定这一步讲哪一段了。
	focus := parsed.FocusBlock
	if parsed.Lens == "" {
		if nextNow := readingTaskAfterAdvance(tasks, parsed.Advance); nextNow != nil && nextNow.BlockID != "" {
			focus = nextNow.BlockID
		}
	}

	// The final card rides on the AI message's payload inside the same
	// transaction as the words and task status. Run the completion guard first:
	// otherwise the saved payload could show a card that the response withheld.
	if _, err := qtx.AppendAtomMessage(turnCtx, sqlc.AppendAtomMessageParams{
		AtomID: at.ID, Seq: seq, Role: "ai", Content: parsed.Reply,
		// 卡片没发出去的时候，理由也一起存 —— 下一轮当面告诉它。见
		// coachMessagePayload.Dropped。
		// 🚨 两次都断的时候，这条半句话仍然会交给她（不编、不改写它的话）——
		// 但要让界面说出「这条没说完」。产品负责人 2026-09-12：
		// 「sometimes the AI response interrupts mid-stream without any notice」。
		Payload: coachCardPayloadFull(parsed.Card, parsed.dropReason(),
			replyLooksCutOff(parsed.Reply), restored, focus),
	}); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if err := tx.Commit(turnCtx); err != nil {
		httpx.WriteError(w, r, err)
		return
	}

	after, err := a.d.Queries.ListReadingTasks(r.Context(), at.ID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	next := currentReadingTask(after)
	currentID := ""
	if next != nil {
		currentID = next.ID.String()
	}

	var cardOut *cardDTO
	nudge := ""
	if parsed.Lens != "" {
		// 铁律④ — origin is 'router': SHE did not pick this lens, and the
		// autonomy signal on the row must say so. Failure is silent: the
		// coach's words still stand, she simply doesn't get the instrument.
		lres, serr := a.summonReadingLens(turnCtx, u.ID, at.ID, parsed.Lens, parsed.FocusBlock, cardOriginRouter)
		if serr != nil {
			slog.Info("lite coach: lens summon failed; turn stands without it",
				"atom_id", at.ID, "card_id", parsed.Lens,
				"request_id", httpx.RequestIDFromContext(r.Context()), "err", serr)
		} else if lres.Card != nil {
			cardOut, nudge = lres.Card, lres.Nudge
		}
	}

	resp := map[string]any{
		"reply":         parsed.Reply,
		"tasks":         readingTaskDTOs(after),
		"currentTaskId": currentID,
		"focusBlock":    focus,
		"tool":          parsed.Tool,
		"finished":      next == nil,
	}
	// The model's thinking for this turn, shown FOLDED next to the reply so she
	// can open it if she wants to see how the thing asking her questions got to
	// this one. Absent whenever the class this call routes to has thinking off,
	// and the client then renders no fold at all — an empty fold reads as "it
	// did not think" when the truth is "there was nothing to read".
	//
	// Never persisted: it is not written into the transcript this turn is saved
	// into, so a refresh loses it. The model's scratch work is not her record;
	// the process tree is.
	if t := strings.TrimSpace(res.Reasoning); t != "" {
		resp["thinking"] = t
	}
	if cardOut != nil {
		resp["card"] = cardOut
		resp["nudge"] = nudge
	}
	// 🚨 NOT "card". That key above is the lens card — a different thing with
	// a different lifetime (a row in atom_card, opened and closed) — and the
	// two would silently overwrite each other on any turn that produced both.
	if parsed.Card != nil {
		resp["coachCard"] = parsed.Card
	}
	// 🚨 「回到原题」同时进响应和那条消息的 payload：乐观更新看的是这里，
	// 刷新之后读回来的是那边，两边必须是同一个事实。
	if restored {
		resp["restored"] = true
	}
	httpx.WriteJSON(w, http.StatusOK, resp)
}

// readingCoachReplyNeedsRetry lists replies the request path gives one chance
// to repair before a student sees them.
func readingCoachReplyNeedsRetry(got readingCoachReply) bool {
	return readingCoachSettlesWithTool(got) ||
		got.cardWhy == cardRejectPromised || got.cardWhy == cardRejectCutOff ||
		got.cardWhy == cardRejectDeadTurn || got.lensRetry || got.twoAsks ||
		got.ghostQuote != "" || got.leak != "" ||
		got.cardWhy == cardRejectOneBlock || got.cardWhy == cardRejectFewOptions ||
		got.cardWhy == cardRejectFewWords || got.cardWhy == cardRejectBannedForm ||
		got.cardWhy == cardRejectAsksMultiple ||
		got.cardWhy == cardRejectNoArgument || got.cardWhy == cardRejectOrderNotHere ||
		got.cardWhy == cardRejectNoOrderBoard || got.cardWhy == cardRejectBoardRepeat
}

// acceptableReadingCoachRecovery is the request handler's selection rule
// after a readable card/lens reply asked for one recovery attempt.
func acceptableReadingCoachRecovery(raw string, blocks []Block, lang string, lensOK func(string) bool) (readingCoachReply, bool) {
	got, ok := parseReadingCoachReply(raw, blocks, lang, lensOK)
	cutOffPlainReply := got.Card == nil && got.Lens == "" && replyLooksCutOff(got.Reply)
	if !ok || cutOffPlainReply || readingCoachSettlesWithTool(got) || got.lensRetry || got.twoAsks ||
		firstProtocolLeak(got.Reply) != "" || replyCallsHerShe(got.Reply, blocks) ||
		(got.cardWhy != cardOK && got.cardWhy != cardRejectNoCard) {
		return readingCoachReply{}, false
	}
	return got, true
}

func readingCoachSettlesWithTool(got readingCoachReply) bool {
	// A completed step can hand the next task to the student with one card.
	// A skipped step ends without opening another teaching interaction.
	return got.Advance == "skipped" && (got.Card != nil || got.Lens != "")
}

// A skipped turn must not open a new teaching interaction. A completed turn
// may introduce the next task with its card, so it is deliberately preserved.
func enforceSettledReadingTurn(got readingCoachReply) readingCoachReply {
	if got.Advance != "skipped" {
		return got
	}
	got.Card = nil
	got.Lens = ""
	return got
}

// protectedReadingCoachAdvance applies objective advancement guards in the
// request path; benchmark fixtures reuse only these small settled-step rules.
func protectedReadingCoachAdvance(proposed string, current *sqlc.ReadingTask, answer *coachCardAnswer, picks []readingPick, msgs []sqlc.AtomMessage, blocks []Block, studentText string) string {
	if current == nil {
		return proposed
	}
	// A bare request for the answer is neither the student's completed work
	// nor an instruction to skip. Keep this intentionally narrow: mixed turns
	// may also contain a real answer or an explicit skip request.
	if answer == nil && readingCoachAnswerOnly(studentText) {
		return ""
	}
	if current.Kind == string(taskHunt) && proposed == "done" && !hasHuntPickEvidence(picks, msgs, blocks) {
		return ""
	}
	if current.Kind == string(taskLabel) && proposed == "" && answeredBoard(answer) {
		return "done"
	}
	return proposed
}

func readingCoachAnswerOnly(text string) bool {
	s := strings.Trim(strings.TrimSpace(text), " \t\r\n。！？，,.!?\"")
	switch s {
	case "我放弃", "直接告诉我答案", "请直接告诉我答案", "告诉我答案", "给我答案", "给我看范例":
		return true
	}
	return false
}

// replyQuotesBlock —— 这一轮的话里，有没有一段逐字来自那一段的原文。
//
// 用来判「它是真的做了一遍示范，还是只说了这套看法的名字」。示范一定引原句
// （prompt 要求挑出那一段里的某一句当着她的面分析），空谈一定不引。
//
// 窗口沿用 replyMentionsSentence 那一套：中文八个字、英文二十个字母。
func replyQuotesBlock(reply string, blocks []Block, blockID string) bool {
	if reply == "" || blockID == "" {
		return false
	}
	for _, b := range blocks {
		if b.ID != blockID {
			continue
		}
		for _, sent := range splitSentences(b.Text) {
			if replyMentionsSentence(reply, sent) {
				return true
			}
		}
		return false
	}
	return false
}

// replyEndsOnAQuestion —— 这句话是不是以一个问句收尾。
//
// 只看**最后一句**：中间出现问号是正常的（「这句在问什么？它在说成本」这种
// 自问自答是讲解的一部分），收尾那句才决定她接下来伸手去做什么。
func replyEndsOnAQuestion(reply string) bool {
	r := []rune(strings.TrimSpace(reply))
	if len(r) == 0 {
		return false
	}
	// 收尾的引号、括号不算数，往回找到真正的最后一个字。
	for len(r) > 0 && strings.ContainsRune("」』）)\"'“”", r[len(r)-1]) {
		r = r[:len(r)-1]
	}
	if len(r) == 0 {
		return false
	}
	return r[len(r)-1] == '？' || r[len(r)-1] == '?'
}

// lastBoardPlacement —— 她最近一次摆完的板：每一句现在在哪一格。
//
// 读的是她那条消息里那份原样的作答（composeBoardAnswer 写的格式）：
// 一行「格子名：」，下一行是那句原文。
func lastBoardPlacement(msgs []sqlc.AtomMessage) map[string]string {
	for i := len(msgs) - 1; i >= 0; i-- {
		m := msgs[i]
		if m.Role != "student" {
			continue
		}
		ans := coachAnswerFromPayload(m.Payload)
		if ans == nil || ans.Type != coachCardLabelRoles {
			continue
		}
		out := map[string]string{}
		lines := strings.Split(ans.Choice, "\n")
		for j := 0; j+1 < len(lines); j++ {
			bin := strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(lines[j]), "："))
			if !isAnyBoardLabel(bin) {
				continue
			}
			if sent := strings.TrimSpace(lines[j+1]); sent != "" {
				out[sent] = bin
			}
		}
		return out
	}
	return nil
}

// dropAlreadyPlaced prevents a fresh board from carrying a mixture of new
// options and sentences already placed on the previous board. An all-old or
// too-small board is retained so a deliberate retry remains usable.
func dropAlreadyPlaced(c *coachCard, placed map[string]string) *coachCard {
	if c == nil || c.Type != coachCardLabelRoles || len(placed) == 0 {
		return c
	}
	fresh := make([]coachCardOption, 0, len(c.Options))
	for _, o := range c.Options {
		if _, seen := placed[strings.TrimSpace(o.Quote)]; !seen {
			fresh = append(fresh, o)
		}
	}
	if len(fresh) == len(c.Options) || len(fresh) < coachCardMinOptions {
		return c
	}
	out := *c
	out.Options = fresh
	return &out
}

// replyAsksToMoveOnABoard distinguishes a new operational instruction from
// feedback such as “you put this in the evidence bin accurately”.
func replyAsksToMoveOnABoard(reply string) bool {
	verbs := []string{"挪到", "挪进", "挪回", "移到", "移进", "拖到", "拖进", "改放", "换到"}
	for _, sent := range strings.FieldsFunc(reply, func(r rune) bool {
		return r == '。' || r == '！' || r == '？' || r == '\n'
	}) {
		moves := false
		for _, v := range verbs {
			if strings.Contains(sent, v) {
				moves = true
				break
			}
		}
		if !moves {
			continue
		}
		for _, label := range allBoardLabels() {
			if strings.Contains(sent, label) {
				return true
			}
		}
	}
	return false
}

// coachMoveVerbs —— 「把它挪过去」的说法。
var coachMoveVerbs = []string{"拖到", "拖进", "挪到", "挪进", "移到", "移进", "放进", "放到", "改放", "换到"}

// replyAsksForANoOpMove —— 这句话是不是在让她把一张卡挪到它已经在的那一格。
func replyAsksForANoOpMove(reply string, placed map[string]string) bool {
	if reply == "" || len(placed) == 0 {
		return false
	}
	moves := false
	for _, v := range coachMoveVerbs {
		if strings.Contains(reply, v) {
			moves = true
			break
		}
	}
	if !moves {
		return false
	}
	for sent, bin := range placed {
		if strings.Contains(reply, bin) && replyMentionsSentence(reply, sent) {
			return true
		}
	}
	return false
}

// replyOnlyAsksHerToRead —— 这一轮的全部内容是「你先把全文读一遍」。
//
// 🚨 「请通读一遍全文」是一句**祈使句**，所以 replyAsksForSomething 认它是
// 「请她做事了」—— 但读文章这件事**在屏幕上交不出来**：没有卡片可点、没有句子
// 可划，她读完之后手里什么都没有，只能干等。实测她连着四轮说的是同一句：
//
//	「它说『先通读一遍全文』，但我读完了不知道接下来要干嘛，没有下一步的按钮。」
//
// 第五轮她放弃了。
//
// prompt 里早就写着「不要以『先通读全文，读完告诉我』收尾」，它照样这么收尾 ——
// 按 [[prompt-twice-then-make-it-checkable-2026-09-12]]，写第三遍不如做成判据。
//
// 只在**没有卡片也没有透镜**的时候判：带着卡片说「先通读一遍再点」是正常的，
// 那一轮她手上有东西。
func replyOnlyAsksHerToRead(reply string) bool {
	read := false
	for _, w := range []string{"通读", "读一遍", "全文读", "读完全文", "先读一下全文"} {
		if strings.Contains(reply, w) {
			read = true
			break
		}
	}
	if !read {
		return false
	}
	// 它在同一轮里还请她做了别的（划一句、写一句、挑一个）—— 那就有落点。
	for _, w := range []string{"划", "挑一", "选一", "找出", "标出", "圈出", "写下", "写一"} {
		if strings.Contains(reply, w) {
			return false
		}
	}
	return true
}

// fallbackCardFor —— 它说了有卡、而那张卡没能发出去时，兜底的那一张。
//
// 🚨 只可能是 pick_in_article：五种卡片里只有它**没有 options**，因此不存在
// 「引文和原文对不上」这种驳回理由 —— 它一定发得出去。这正是兜底需要的性质。
//
// 问题优先用 印记 自己刚才写的那一道（哪怕那张卡因为选项坏了被丢掉，那道题
// 本身是好的）。它连题都没写才用中性的那句。
// replyAsksToWrite —— 这句回复是在请她打字。
func replyAsksToWrite(reply string) bool {
	for _, w := range []string{"写几个字", "写下", "写一", "写出", "打字", "用你自己的话", "写在"} {
		if strings.Contains(reply, w) {
			return true
		}
	}
	return false
}

func fallbackCardFor(asked, reply, stepQ string) *coachCard {
	// 🚨 兜底那张卡要她做的事，必须和话里说的是同一件事。
	//
	// 线上实测（2026-09-17，刚部署完那一轮）：印记 说「下面那张卡上写几个字
	// 就行：你觉得这篇报道接下来会讲哪几类消息？」而兜出来的卡片写着
	// 「请在文章里点出你想说的那一句。」—— 一个要她打字，一个要她点句子。
	// 这和产品负责人报的第 2 条是同一种伤：「对话框指令和动手部分的指令不一致」，
	// 只是这一次那句不一致是**我们自己写的**。
	//
	// 两种形状都不可能被驳回（都没有 options，也就没有「引文对不上原文」
	// 可言），所以按话里的动词挑：请她写就给 short_text，其余给 pick_in_article。
	kind := coachCardPickInArticle
	if replyAsksToWrite(reply) {
		kind = coachCardShortText
	}
	for _, prompt := range []string{asked, questionIn(reply, true), stepQ} {
		prompt = strings.TrimSpace(prompt)
		// 🚨 兜底那张卡也逃不掉「题目装不下」那一条：pick_in_article 她只指得了
		// 一处，一道要两处的题在它上面同样无解（见 promptAsksForSeveral）。
		// 要她自己写的那种不受限：一句里写两处是她的自由。
		if kind == coachCardPickInArticle && promptAsksForSeveral(prompt) {
			continue
		}
		if n := utf8.RuneCountInString(prompt); n > 0 && n <= coachCardPromptMaxRunes && !promptPresumesOptions(prompt) {
			return &coachCard{Type: kind, Prompt: prompt}
		}
	}
	return nil
}

func stepQuestion(t *sqlc.ReadingTask) string {
	if t == nil {
		return ""
	}
	return questionIn(t.Detail, false)
}

func questionIn(text string, last bool) string {
	r := []rune(strings.ReplaceAll(strings.TrimSpace(text), "**", ""))
	end := -1
	for i, c := range r {
		if c == '？' || c == '?' {
			end = i
			if !last {
				break
			}
		}
	}
	if end < 0 {
		return ""
	}
	start := 0
	for i := end - 1; i >= 0; i-- {
		if strings.ContainsRune("。！？!?：:\n", r[i]) {
			start = i + 1
			break
		}
	}
	return strings.TrimSpace(string(r[start : end+1]))
}

// promptPresumesOptions —— 这道题要的是一组摆在卡上的句子（「分析下列句子，判断
// 它们各自属于……」）。兜底的两种卡都没有选项，照抄这道题她就对着一张空卡被要求
// 分类（2026-09-17 入口走查，星图那篇果蝇脑图）。
func promptPresumesOptions(prompt string) bool {
	for _, w := range []string{"下列", "下面这几句", "以下几句", "各自属于", "分别属于", "每一句"} {
		if strings.Contains(prompt, w) {
			return true
		}
	}
	return false
}

// escapeRawControlInStrings 把 JSON 字符串值里没转义的控制字符（换行、回车、
// 制表符等）改写成合法的转义序列；字符串外面的内容一律不动。
//
// 🚨 2026-09-14 的 coachwalk 走查录到：模型在 reply 里分了两段，换行直接写进了
// 字符串值 ——
//
//	{"reply":"第 6 段最后一句是：「……」⏎⏎这句话是整篇里最要紧的一处。…
//
// 严格的 JSON 不允许这样，json.Unmarshal 和 salvageCoachReply 用的 Decoder 都拒收，
// 调用点只能再问一次，两次都这样就回 502。而这份回复本身是完整、可用的。
// system prompt 要她分段、写短列表，原来却没说字符串里的换行要写成 \n ——
// 所以这是 prompt 在要求一种 JSON 装不下的输出。prompt 已补上这一句；
// 这里是那句话没被遵守时的保证。
//
// 字符串外面的换行是合法空白（缩进过的 JSON 里到处都是），必须原样保留。
// 只处理控制字符：字符串里没转义的直双引号无法从外部判断，不在这里修。
func escapeRawControlInStrings(s string) string {
	const hex = "0123456789abcdef"
	var b strings.Builder
	b.Grow(len(s) + 16)
	inStr, esc := false, false
	for _, r := range s {
		if !inStr {
			if r == '"' {
				inStr = true
			}
			b.WriteRune(r)
			continue
		}
		if esc {
			esc = false
			b.WriteRune(r)
			continue
		}
		switch {
		case r == '\\':
			esc = true
			b.WriteRune(r)
		case r == '"':
			inStr = false
			b.WriteRune(r)
		case r == '\n':
			b.WriteString(`\n`)
		case r == '\r':
			b.WriteString(`\r`)
		case r == '\t':
			b.WriteString(`\t`)
		case r < 0x20:
			b.WriteString(`\u00`)
			b.WriteByte(hex[r>>4])
			b.WriteByte(hex[r&0xf])
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}
