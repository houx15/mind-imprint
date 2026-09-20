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

// readingCoachSystem is sent with every reading-coach turn. Keep it to
// executable rules; incident histories and lengthy rationale do not belong in
// a repeated model context.
const readingCoachSystem = `你是「印记」，带一名中学生读文章。你会收到全文（按段落）、读法清单、对话、当前输入和可能的组件回灌。

## 本轮优先级

1. 她明确说「跳过这一步／不做这一步」：确认跳过当前步，advance="skipped"，card、lens 留空；不要继续领读、提问或出题。这条优先于首次领读、当前步骤说明和默认发卡。「我放弃」单独出现是索答，不是跳过。
2. 明确索答（「直接告诉我答案」「给我答案」「我放弃」「给我看范例」）：**直接给当前问题的完整答案**，不改成提示或反问；索答不是完成或跳过，advance 留空。
3. 「不会／给点提示」或「是不是 X」：按提示梯子只给一级，不公布答案。先看她已说了什么，只补下一层方向、位置或局部词语线索；不要引用含答案的整句原文。若 prompt 说屏幕上已有卡片，沿用那张卡，card、lens 留空，不推进。独立概念或词义可直接解释，但不算完成。
4. 先看当前任务的要求：她已覆盖全部要求，即使粗糙也必须 advance="done"；不再追问、要求点击或换一种说法。解释一个术语时可以引用其他段落，不能因证据不在当前段而要求重做。当前任务有多个信息点时，只答其中一个（如只说排名、时间或「投入很大」）仍是部分回答：advance 留空，只提示缺少的信息，不替她补上答案。每轮最多推进一步。
5. 已完成、跳过，或收到标注板／透镜完成回灌时：先具体回应她的成果，再推进。跳过不附加工具；完成时按当前步骤说明直接交接下一步，必要时可给下一步的一张 card。回灌是已提交的成果，不要求她操作已经收起的组件。

## 对话方式

- 对学生一律称「你」，reply 不出现 advance、focusBlock、card、lens 或 b1/b2 等内部名；段落一律说「第几段」。文章原文中的「她」可照引。
- 一次只领当前一步、只问一个问题；具体说第几段和要做什么，不复述任务标题。先接住她的回答，再继续。
- 不超过 200 个字。**没有卡片的那一轮，话要更短，不是更长**。**每一轮都用一次加粗**，只强调一个词；可以用一点排版，但只用在真正有用的地方：列表里并排的是**选项**，不是问题。
- 不要催促、评价快慢、空夸或说「作为 AI」。**不要训她**：不要去评论「她还没做到」这件事本身；把下一步说得更具体，例如「在第 4 段点一下那句，点完它会出现在下面。」答错时必须指出句子或词语哪里不成立，这不是训斥。
- 未明确索答时，不抢答文章结论、当前练习答案或整句答案原文。学生的判断站得住，说明文章依据；只对一半或错误，先指出关键问题和原文依据，再请她重试。拿不准时先信她可能有合理读法。
- 一轮只修最重要的一项：主张误读 → 漏证据/限制 → 相关与因果、可能与证明 → 句子主干 → 转折让步 → 词汇 → 语法。纠正后要她再产出一次；最多重试两次。不要替她改写作答或给整段示范。

## 提示梯子

按顺序每轮一级：方向 → 位置 → 结构 → 局部线索 → 完整答案。明确索答可直接给完整答案；「不会／太难／给提示」不是索答。让她写时依次给约束、句子开头、最后才给填空或词库。

## 输出

只输出一个 JSON 对象：
{"reply":"给学生的话","advance":"","focusBlock":"","tool":"","lens":"","card":null}

- advance 只能是 ""、"done"、"skipped"。focusBlock 是真实段落 id，且必须对应 reply 中所说的段落；不用则空。
- tool 是下列段落工具 id；lens 是下列透镜 id；card 为一张卡，不发则省略或 null。
- reply 的换行写成 \n，不要在 JSON 字符串内直接回车。不要输出 JSON 以外文字或代码块。

## 段落工具与透镜

段落工具由你选择：读不懂句子用讲解类；读懂字面未看出手法用 craft/structure；要她思考用 questions；值得练写法用 imitate。每轮最多一件；使用时在 reply 用一句说明原因。

%s

透镜是让她亲手分析：reply 先用当前段落的一句示范「它为什么有力／可疑／在做什么」，再给 lens，请她在别处找一句同样分析。方法名后要用一句白话解释；只选文章确实撑得住的透镜。给 lens 必须给对应 focusBlock；已有打开的透镜不再给。**这一轮已经给了 lens，就不要再给卡片**。

%LENS%

收到【她刚做完一副透镜】时：具体评价她选句和分析，接回本文问题，advance="done"；不重复上一轮、不问感受、不再给 lens 或 card。若 prompt 给出「你当时给出的结论」，不得与其矛盾。

## 卡片

仅在当前任务尚未完成、需要她动手时，默认给她一张卡片；跳过、求提示、索答优先，不发卡。完成或组件完成回灌时只可按下一步说明给一张下一步 card。prompt 若说屏幕上已有卡片，表示它已送达：只围绕那张卡引导，不再发 card。第一轮也一样，用卡把她领入第一步。每轮最多一张，且有 lens 时不发 card。

- choose_span：{"type":"choose_span","prompt":"一句真问题","options":[{"blockId":"b3","quote":"原文"}]}。只能有一个问题；2–4 条完整原文句子或从标点到标点的完整分句，blockId 必须对应原句。**choose_span 的选项必须跨段落取：至少来自两个不同的段落**；通读某个多段部分时可只在该部分内跨段。做不到上述条件时改用 pick_in_article 或 short_text，不能交出不完整卡片。**存活的选项全部来自同一段，整张卡片会被丢掉**。
- pick_in_article：{"type":"pick_in_article","prompt":"一句真问题"}，请她自己在原文划一句。
- short_text：{"type":"short_text","prompt":"一句真问题"}，请她用自己的话回答。
- label_roles：{"type":"label_roles","prompt":"一句真问题","binSet":"basic","options":[...]}。basic 为「论点／论据／论证」；仅作者明确驳斥别人的观点时用 counter（「论点／驳斥观点／论据／论证」）。只在拆论证步骤使用，一篇至多一块；options 2–4 条，**每一句都必须逐字抄自文章**，可来自同段。挑句前先确认每句能放进其中一格。
- word_bank：{"type":"word_bank","prompt":"一句真问题","words":[{"blockId":"b3","term":"原词"}]}。3–6 个逐字出现在对应段落的词；选学术高频、熟词僻义、搭配或主题词。该轮不解释，回灌后只解释「不确定／不认识」的词。
- order_events：{"type":"order_events","prompt":"一句真问题","options":[...]}。仅报道或记叙的排序步骤使用；2–4 条逐字原文句子，请她按发生先后排列，出卡时不说答案。

卡片规则：

- prompt 不超过 60 字，是问文章内容的一个 5W1H 真问题，问判断而非操作；**出卡片之前先自问一句：这个问题能不能靠扫关键词答出来？能，就换一个。** **不能有唯一正解**。例如「哪一句你读着最不服气」。不问定义、步骤数、泛泛优缺点或「这段讲了什么」。
- 不在出卡轮提前说答案，尤其不替标注板分类；**不要连着出两张几乎一样的卡片**。**上一张卡片问过的那件事，这一张就换一件事问**。题目不写操作或选项数。
- choose_span／label_roles 的 quote 必须原文逐字一致、blockId 正确；引文要**从一个标点后面开始、到一个标点为止**，不截半句、不拼句。选项不能互相包含。
- card 已写明任务时，**reply 就不要再把它复述一遍**，并且 reply 不以问句收尾；只用一句交接或说明发卡理由。
- reply 中的带引号文字只能逐字引用文章、当前卡片或学生原话。

## 组件回灌

【她刚刚说的】以段落工具「想一想／仿写」开头时，只反馈该段：先具体说对了什么，再说一处最值得改的；仿写不代写，想一想看是否以段落内容支撑。advance、card、lens 都留空。

标注板（label_roles／word_bank／order_events）提交后，先具体回应某一句为何合适或最关键的误解；word_bank 只讲不确定／不认识的词；随后 advance="done"。对她已提交的合理分类，不要为了延长这一步而要求重新分类；不要要求拖动已经消失的板，也不再发板。可按下一步说明给一张下一步 card。

## 特别步骤

- read：当前 read 步只管清单标明的段落或部分，不问「读完了吗」。用卡检验该部分大意、作者立场或定位，不讲出内容替她读；她说读完即可 done。通读练主旨和定位，别用扫细节即可回答的问题。
- connect（链接经验）：没有标准答案；接住经验、问一个能让她多说一点的问题，再 done；不评价经历或拉回正确理解。
- hunt（找出关键句）：只有【她在文章里点出来的句子】才算完成。真点了就评价该句并 done；只说「第几段那句」但未点，留在当前步并请她在文章中划出。不同于你的预设不等于错。
- critique（你怎么看）：围绕文章中一个具体判断、证据或来源，给两三个思考角度并用 short_text 请她选一个说；她给出有依据的判断就 done，不要求同意你。
- sequence（排出事件顺序）：用一张 order_events 板让她排发生先后；提交后说明一处原文依据并 done，不再发第二张排序板。
`

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
	"strong":  "这一句撑得住",
	"partial": "这一句只对了一半",
	"rethink": "这一句撑不住，得重选",
}

// clean reports whether this outcome carries enough to be worth a turn. A
// lens with no quote is not a completed lens, and refeeding one would spend a
// flagship call to say "nice work" about nothing.
func (l *readingLensDone) clean() bool {
	return l != nil && strings.TrimSpace(l.Quote) != ""
}

// openLensLine —— 她屏幕上此刻正开着一副透镜的时候，prompt 里加的那一节。
//
// 🚨 产品负责人 2026-09-17：「找不到句子的时候，没法在聊天框打字求助 ai。」
// 输入框在透镜开着的时候是锁死的，所以她唯一的出路是跳过。现在输入框放开了，
// 于是模型必须知道这件事 —— 否则她一开口，它就照常往下领新的一步，而屏幕上
// 那副透镜还在等她。
func openLensLine(anyOpen bool, name string) string {
	if !anyOpen {
		return ""
	}
	which := "一副透镜"
	if n := strings.TrimSpace(name); n != "" {
		which = "透镜「" + n + "」"
	}
	return "\n【她屏幕上正开着" + which + "，还没做完】\n" +
		"这一轮**不要领新的一步，也不要给卡片或者另一副透镜**（advance 留空，card、lens 都留空）。\n" +
		"她这一轮开口，多半是在这副透镜上卡住了。按这三种回应：\n" +
		"- 她说找不到这样的句子 → **先信她**。回去看一眼这篇文章：真的没有，就说清楚这篇撑不住这副透镜，" +
		"请她跳过（卡片上那个跳过就在那儿）。真的有，就指到第几段、哪个词附近，让她自己去读那一句。\n" +
		"- 她问这副透镜到底要她干什么 → 用一句白话说清这种分析在看什么，再当场拿这篇里的某一句做一遍示范。\n" +
		"- 她问别的 → 回答她，然后一句话把她送回那副透镜。\n"
}

// toolAnswerLine —— 这一轮是她在段落工具（想一想 / 仿写）底下写的那一段。
//
// 🚨 和 openLensLine 同一个位置、同一个理由：它排在 prompt **最后**，压过
// 前面那条按步骤写的推进判据。
//
// 实测（2026-09-17，6 次）：只有 system prompt 里那一节说明时，6 次里有 5 次
// 只夸一句就转进清单上的「你怎么看」，有一次对她写的那段一个字没提 —— 末尾那条
// 「本步要她给出自己的判断」的判据赢了（[[reading-room-rulings-2026-09-17]]
// 第一条：散文跨不过判据，第四次）。
func toolAnswerLine(toolAnswerTurn bool) string {
	if !toolAnswerTurn {
		return ""
	}
	return "\n【这一轮不按上面那条推进判据走】\n" +
		"她刚才是在**段落工具**（想一想 / 仿写）底下写了一段，要你给反馈。这一轮只做这一件事：\n" +
		"1. 先说她写对了什么，引她的原话，具体到那一句。\n" +
		"2. 再只说**一处**最值得改的地方和为什么。仿写就看她有没有用上那个写法；想一想就看她的想法有没有落在那一段上、有没有拿文章里的东西撑住。\n" +
		"3. 🚨 不替她写：不给示范段、不把她那段改写一遍。\n" +
		"4. 说完就停。advance 留空，card、lens 都留空。**不要**在这一轮里开始清单上的下一件事，" +
		"最多用一句话提一下清单现在停在哪一步，**不要说有卡片在等她** —— 这一轮没有卡片。\n"
}

func buildReadingCoachPrompt(
	title string,
	blocks []Block,
	outline readingOutline,
	tasks []sqlc.ReadingTask,
	msgs []sqlc.AtomMessage,
	picks []readingPick,
	studentText string,
	lensDone *readingLensDone,
	// openLens 非空时，她屏幕上正开着一副透镜。见 openLensLine。
	openLens string,
) string {
	var b strings.Builder
	if t := strings.TrimSpace(title); t != "" {
		b.WriteString("文章标题：" + t + "\n")
	}

	// 导读。它是排读法那一次就算出来的（reading_outline.go），学生屏幕上也摆着
	// 同一份 —— 所以这里给它，是为了让你和她看的是同一张地图，不是为了让你把它
	// 念一遍。
	if strings.TrimSpace(outline.OneLine) != "" || strings.TrimSpace(outline.Shape) != "" {
		b.WriteString("\n【导读（她屏幕上也有这一份，不要复述）】\n")
		if v := strings.TrimSpace(outline.OneLine); v != "" {
			b.WriteString("这篇在问：" + v + "\n")
		}
		if v := strings.TrimSpace(outline.Gist); v != "" {
			b.WriteString("中心思想：" + v + "\n")
		}
		if v := strings.TrimSpace(outline.Shape); v != "" {
			b.WriteString("结构：" + v + "\n")
		}
		b.WriteString("每段后面标着承重。**核心段停下来问一轮；支撑段连着过，" +
			"过完说一句它们在撑哪一段；过渡段一句带过。** 这是你安排节奏的依据。\n")
	}

	// 体裁。议论文和认不出来的体裁这一节是空的 —— 那两种的 prompt 和
	// 2026-09-17 之前一字不差。见 reading_genre.go。
	b.WriteString(buildGenreCoachSection(outline.Genre))

	// 这篇分成的几个部分。通读那一步照着它一部分一部分地走 —— 见 system
	// prompt 的「一部分一部分地走」。
	//
	// 🚨 段号由**服务端**数（和 readingBlockTag 同一份）。让模型自己从 b3 数出
	// 「第三段」，它会数错，而她屏幕上那个号码是服务端给的 —— 两边对不上，
	// 她照着去找就找不到。
	if len(outline.Parts) > 0 {
		ord := make(map[string]int, len(blocks))
		for i, blk := range blocks {
			ord[blk.ID] = i + 1
		}
		var parts strings.Builder
		ok := true
		for i, p := range outline.Parts {
			from, okFrom := ord[p.From]
			to, okTo := ord[p.To]
			if !okFrom || !okTo {
				// 正文换过了（她重新粘了一份），段 id 对不上。整份切法不给 ——
				// 指着不存在的段落的台阶比没有台阶更糟，而通读那一步本来就有
				// 「没有分部分的时候」那条后路。
				ok = false
				break
			}
			parts.WriteString(itoaSmall(i+1) + ". " + p.Title +
				"：第" + itoaSmall(from) + "–" + itoaSmall(to) + "段")
			if p.Does != "" {
				parts.WriteString("，" + p.Does)
			}
			parts.WriteString("\n")
		}
		if ok {
			b.WriteString("\n【这篇分成几个部分（清单上的通读步骤一步一个）】\n")
			b.WriteString(parts.String())
			b.WriteString("**当前这一步只管它标明的那几段。** 别的部分有它们自己的步骤，" +
				"这一轮一个字都不用提。\n")
		}
	}

	// 这一轮只展开这一步真正要读的那几段，其余点名但不铺开。见
	// reading_disclosure.go：整篇文章占了这份 prompt 的九成，而服务端本来就
	// 知道这一步管哪几段。nil = 不收窄（没有分部分，或这一步要通观全文）。
	scope := readingDisclosureScope(blocks, outline, tasks, picks, msgs)
	b.WriteString("\n【文章，按段落】\n")
	if scope != nil {
		b.WriteString("（这一步只展开它要读的那几段；其余段落在下面点名，" +
			"它们存在，只是这一轮不看。需要回头看别的部分时就说出来。）\n")
	}
	total := 0
	for i, blk := range blocks {
		text := strings.TrimSpace(blk.Text)
		if text == "" {
			continue
		}
		tag := readingBlockTag(i, blk.ID)
		if label := loadLabels[outline.Load[blk.ID]]; label != "" {
			tag += "·" + label
		}
		if scope != nil && !scope[blk.ID] {
			b.WriteString(tag + "：（这一段这一轮没展开，但它存在）\n")
			continue
		}
		runes := []rune(text)
		if total+len(runes) > readingPlanArticleRuneBudget {
			b.WriteString(tag + "：（这一段没放进来，但它存在）\n")
			continue
		}
		total += len(runes)
		b.WriteString(tag + "：" + text + "\n")
	}

	b.WriteString("\n【你排的读法】\n")
	current := currentReadingTask(tasks)
	for _, t := range tasks {
		mark := "待办"
		switch t.Status {
		case "done":
			mark = "已完成"
		case "skipped":
			mark = "已跳过"
		}
		// The kind rides on every line, in the same English identifiers ("hunt",
		// "connect", …) the system prompt's own "## 两种特别的步骤" section names
		// them by — so a task line and the instructions that govern it are
		// actually joined up, instead of the model reverse-inferring a kind from
		// a Chinese label. Harmless to show her-facing paragraph tags too: like
		// the block-id tags above, this is an internal marker for the model, not
		// prose it is told to repeat to her.
		line := "- [" + mark + "] (" + t.Kind + ") " + t.Label
		if t.Detail != "" {
			line += "：" + t.Detail
		}
		if t.BlockID != "" {
			line += "（这一步看 " + t.BlockID + "）"
		}
		if current != nil && t.ID == current.ID {
			line += "   ← **她现在在这一步**"
		}
		b.WriteString(line + "\n")
	}
	if current == nil {
		b.WriteString("\n所有步骤都走完了。跟她说一句收尾的话，别再领新的一步。\n")
	}

	b.WriteString("\n【你们刚才聊的】\n")
	tail := msgs
	if len(tail) > readingCoachTurnsWindow {
		tail = tail[len(tail)-readingCoachTurnsWindow:]
	}
	any := false
	for _, m := range tail {
		var who string
		switch m.Role {
		case "student":
			who = "她"
		case "ai":
			who = "你"
		default:
			continue
		}
		if s := strings.TrimSpace(m.Content); s != "" {
			b.WriteString(who + "：" + s + "\n")
			any = true
		}
	}
	if !any {
		b.WriteString("（还没聊过。）\n")
	}

	// 🚨 她屏幕上现在摆着的那张卡片/板，原样给它看。
	//
	// 模型只看得见自己说过的**话**，看不见随那句话发出去的 card —— 而那张卡有时
	// 根本不是它写的（标注论证那一步由服务端兜底摆板，见
	// reading_coach_board_build.go）。于是它会对着一块自己没见过的板提要求：
	// 实测「把主张那张换成文章里某个人亲口说的话」，而板上四句全是叙述句，
	// 一句引语都没有 —— 她照着做不到，当场卡死。
	//
	// 递出去的东西要让它知道，这和「没送到要告诉它」是同一条闭环的两半。
	if card := lastOpenCard(tail); card != nil {
		b.WriteString("\n【她屏幕上现在摆着这张卡片，你看不到，所以照着它说话】\n")
		b.WriteString("类型：" + card.Type + "　问题：" + card.Prompt + "\n")
		for i, o := range card.Options {
			ord, ok := readingPickOrdinal(blocks, o.BlockID)
			where := ""
			if ok {
				where = "（第" + itoaSmall(ord) + "段）"
			}
			b.WriteString("  " + itoaSmall(i+1) + ". " + where + "「" + o.Quote + "」\n")
		}
		for i, w := range card.Words {
			b.WriteString("  " + itoaSmall(i+1) + ". " + w.Term + "\n")
		}
		if len(card.Labels) > 0 {
			b.WriteString("格子：" + strings.Join(card.Labels, " / ") + "\n")
		}
		b.WriteString("🚨 **只能要求她用板上真有的东西。** 板上没有的句子、没有的词，" +
			"不要让她去找 —— 她手上只有上面这几样。\n")
	}

	// 🚨 上一轮那张卡片没发出去的话，当面告诉它为什么。
	//
	// 它自己发现不了：写完就交出去了，下一轮的上文里只有它说过的话。线上实测
	// 连着六轮在说「点这张卡」而卡片每轮都被丢掉 —— 她屏幕上是一句句指着空气的
	// 话。这是「闭环」的失败那一侧：AI 递出去的东西没送到，也得让它知道。
	if why := lastDroppedCard(tail); why != "" {
		fix := cardFixIt[cardReject(why)]
		if fix == "" {
			fix = "这一轮换一件事做，或者直接把这一步说清楚。"
		}
		b.WriteString("\n【你上一轮递出去的东西没有到她屏幕上】\n" +
			"原因：" + why + "\n怎么改：" + fix + "\n" +
			"她那边只有你说的话，没有卡片、也没有透镜 —— 所以**不要再提「这张卡」" +
			"「这副透镜」「上面那块板」**，她看不到。\n" +
			"这一轮要么按上面的规矩重新给一次，要么就什么都不给，用一句具体的指令" +
			"把这一步说清楚。\n" +
			"🚨 **这件事不要说给她听。** 她不需要知道我们这边有校验、有规则、" +
			"有什么「系统不收」——那是我们的事，说出来只会让她觉得这个房间在出故障。\n")
	}

	// 🚨 这一步是不是卡住了（同一步带了三轮以上还没动）。卡住了才加这一节，
	// 没卡住一个字都不加 —— 常驻的提示会抢掉这一轮真正该做的事。
	// 见 reading_coach_repeat.go。
	if coachStepStuck(tasks, tail) {
		b.WriteString(coachStuckNudge)
	}

	// Structurally distinct from what she typed: a pick is a pointer at a real
	// paragraph, never spoken to the model as a block id (only 第几段, same as
	// the paragraph listing above) — the id is an internal marker, not
	// something the model should ever try to repeat back to her.
	if len(picks) > 0 {
		b.WriteString("\n【她在文章里点出来的句子】\n")
		for _, p := range picks {
			ord, ok := readingPickOrdinal(blocks, p.BlockID)
			if !ok {
				continue
			}
			b.WriteString("第" + itoaSmall(ord) + "段：「" + p.Quote + "」\n")
		}
	}

	// A completed lens is HER WORK, so it gets its own section rather than
	// being folded into 【她刚刚说的】 — the finding is 印记's own earlier
	// evaluation and must never read as a sentence she uttered.
	if lensDone.clean() {
		b.WriteString("\n【她刚做完一副透镜】\n")
		if n := strings.TrimSpace(lensDone.CardName); n != "" {
			b.WriteString("透镜：" + n + "\n")
		}
		b.WriteString("她自己在文章里找的那一句：「" + strings.TrimSpace(lensDone.Quote) + "」\n")
		if f := strings.TrimSpace(lensDone.Finding); f != "" {
			b.WriteString("你当时对这一句的复核（这是你自己的话，不是她说的）：" + f + "\n")
		}
		// 🚨 复核的结论也要给，而且要说明她已经看过了 —— 否则这一轮会跟几秒钟
		// 前屏幕上那个结论对着干。见 readingLensDone 的 Verdict。
		if w := verdictWord[strings.TrimSpace(lensDone.Verdict)]; w != "" {
			b.WriteString("你当时给出的结论（**她屏幕上已经看到了这一句**）：" + w + "\n")
			if vr := strings.TrimSpace(lensDone.VerdictReason); vr != "" {
				b.WriteString("你当时给的理由：" + vr + "\n")
			}
			b.WriteString("🚨 **这一轮不要跟这个结论相反。** 你判了撑不住，就不要改口夸她选得准；" +
				"判了撑得住，就不要又说它不行。她已经读过上面那一句了，" +
				"两句话打架的时候她只能猜哪一句算数。\n" +
				"判的是**那一句话**，不是她这个人 —— 承认她动手做了，说清楚问题在哪儿，然后往下走。\n")
		}
	}

	if studentText != "" {
		b.WriteString("\n【她刚刚说的】\n" + studentText + "\n")
	} else if lensDone.clean() {
		// 🚨 NOT the 「她刚点了「开始」」 line below. She did not press 开始 —
		// she just finished a lens, and telling the model otherwise makes it
		// re-introduce the reading plan from the top (the PBL repeat-itself
		// bug, in this room). See readingLensDone's own comment.
		b.WriteString("\n【她刚刚说的】\n（这一轮她没打字——她是把透镜做完了。按「她刚做完一副透镜」那一节的三件事回应她。）\n")
	} else {
		// 🚨 这里原来写的是「介绍一下你排的读法」。那是一道自相矛盾的题：
		// 要它介绍，又不许它说这篇文章的内容，它只能去说「这篇大概在讲什么」
		// ——也就是内容。真模型给出的那一份把文章讲了一遍、替她减压、还以
		// 「读完告诉我一声」收尾，三条规矩一句话里全违反了。
		//
		// 导读卡（一句话 + 结构 + 段落承重）现在由前端确定性地渲染，摆在正文
		// 顶上。所以这一轮它**没有内容可讲**，只剩下递第一张卡片这一件事。
		// 见 reading_outline.go 和 docs/2026-09-10-reading-guidance-redesign.md。
		b.WriteString("\n【她刚刚说的】\n（她刚点了「开始」，还没说话。" +
			"导读已经摆在她面前了——这篇在问什么、它怎么组织、哪几段承重，她都看得见，" +
			"**不要再复述一遍**，也不要讲这篇文章的内容。" +
			"直接领她进第一步，并且用一张卡片把她领进去。）\n")
	}
	// 她按了卡片底下的求助按钮（给点提示 / 示范一下）。见 helpRequestSection。
	b.WriteString(readingCurrentStepInstruction(tasks, studentText))
	b.WriteString(helpRequestSection(studentText))
	// 透镜开着这件事排在最后：它**取消**上面那条推进判据（这一轮不推进），
	// 而最后一节才是这一轮真正的指令。
	b.WriteString(openLens)

	return b.String()
}

// Bind the generic teaching rules to the one active task. This is a prompt
// projection only; it never settles state or invents a completion signal.
func readingCurrentStepInstruction(tasks []sqlc.ReadingTask, studentTexts ...string) string {
	studentText := ""
	if len(studentTexts) > 0 {
		studentText = studentTexts[0]
	}
	current := currentReadingTask(tasks)
	if current == nil {
		return "\n【本轮状态】读法清单已结束，简短收尾，不再布置阅读任务。\n"
	}
	rule := "按当前任务文字判断。学生已完成要求的各项内容，就给 done；只完成部分就留空并只提示缺少的那一项。不要添加第二个例子、更多证据或额外点击作为完成门槛。"
	switch current.Kind {
	case string(taskRead):
		// 🚨 这一条判的是**这一步标明的那几段**，不是整篇。通读在清单上已经
		// 一步一个部分（reading_plan.go 的 readingPartSteps），所以「答了就
		// done」在这里是对的 —— 它推进的是一个部分，下一步就是下一个部分。
		// 在拆步之前，这条判据和 system prompt 里「走完最后一个部分才 done」
		// 直接打架，而末尾的判据赢：一张卡答完，整个通读当场结束。
		rule = "本步只管它标明的那几段，不是整篇。学生明确说这几段读完了，或已回答本步的通读卡片，就给 done。" +
			"认可后直接介绍清单里的下一步（多半就是下一个部分），不再加一道通读测验。" +
			"reply 里说段落范围要照本步说明里那几段说，不要说「通读全文」。"
	case string(taskFocusBlock):
		// 🚨 产品负责人 2026-09-17：「切入精读部分，并没有交代为什么 ai 选中的
		// 段落是需要精读的段落。」这一段是排读法时挑出来的，理由只在服务端 —— 她
		// 屏幕上出现的是一句「往下翻到第 4 段」，凭什么是第 4 段没有人告诉她。
		if strings.TrimSpace(studentText) == "" {
			rule = "她尚未对当前步骤作答：reply 先用一句话说清**这一段凭什么值得精读**（它在全文里承担什么：" +
				"唯一给数据的地方、论证的转折处、作者把话说得最重的一段……），再领她做。" +
				"这句话说的是这一段在文章里的位置和作用，不是它的内容摘要。"
		} else {
			rule = "她已经对当前步骤作答或提出操作请求：不要重新介绍这一段或默认发卡。按当前任务文字判断：她做完要求的各项就给 done，只完成部分就留空并只提示缺少的那一项；任务有多个信息点时，只答其中一个仍是部分回答，不能替她补上缺少答案。解释这一步的术语可引用其他段落，不能因证据不在当前段而要求重做。"
		}
	case string(taskConnect):
		// 🚨 产品负责人 2026-09-17：「阅读的链接自身那个部分有点抽象了，还有点
		// 鸡肋。」抽象是因为问法本身是空的（「这篇讲的事你碰到过吗」）——一个
		// 没有落点的问题，她只能泛泛答一句。落点要从**这篇文章里**取。
		rule = "本步的问题必须挂在这篇文章的一个具体处上：先说出文章里的某一件事、某个数字、或者作者的某一个判断（一句），" +
			"再问她在自己这儿碰到过什么与它对得上或对不上的事。不要问「你有什么感想」「你碰到过吗」这类没有落点的问题。" +
			"学生已经表达自己的经历或联想，就给 done。尊重这段经历，不要求它符合文章的标准答案。"
	case string(taskHunt):
		rule = "只看【她在文章里点出来的句子】是否有真实选句。有选句就给 done，不要求它与你偏好的句子相同，也不要求再选一句。没有真实选句时留空并说明点击操作。"
	case string(taskLabel):
		// 🚨 「依据她的分类简短反馈并给 done」后面那半句是 2026-09-17 加的。
		// 产品负责人在一篇文章里被同一块板问了四次（「three of them are the
		// same one」）—— 摆完之后觉得有一两张放错，就再发一块板让她从头摆，
		// 而她每次摆的其实是同一件事。判断该由**话**来给，不由第二块板来收。
		rule = "已收到标注板的真实作答时，依据她的分类简短反馈并给 done；尚未提交时引导她使用标注板。" +
			"普通文字说摆好了不能替代真实作答。" +
			"🚨 觉得她有一两张放错了，就在 reply 里说清那一句为什么该换个位置，然后照常给 done —— " +
			"**绝对不要再发第二块标注板**。这一步只摆一次板。"
	case string(taskCritique):
		// 🚨 2026-09-17 新增，顶掉了 透镜 那一步。产品负责人逐字：
		//
		//	we can invite students to give some comments on this, like do they
		//	agree with author's view, do they think the evidence is enough, or
		//	do they think if there is another possiblity.
		//	give perspectives suggestion, like another possibility,
		//	credibility, another explanation, etc.
		//
		// 「你怎么看」直接问出去，收到的是「我觉得挺好的」。她需要的不是一个
		// 更大的问题，是**几个角度**——而角度要挂在这篇文章的具体处上，
		// 否则又是一个 taskConnect 那样的空问题。
		rule = "本步要她给出自己的判断，不是复述作者。reply 先在**这篇文章里**点出一处她可以下手的地方" +
			"（作者的某一个判断、某一条证据、某个只有一个来源的说法），再给她两三个角度让她挑一个，" +
			"写成一行一个的短列表。角度从这几种里挑：**同不同意**这个判断、" +
			"这条**证据够不够**撑住它、有没有**另一种解释**、这个说法的**来源可不可信**、" +
			"作者**漏掉了谁**。然后用一张 short_text 卡请她写。" +
			"她写出了自己的判断就给 done —— **不要求她的判断和你一致**，也不要求她写长。" +
			"她说同意作者，就请她说一句凭什么同意；那也是一个判断。"
	case string(taskSequence):
		// 2026-09-17，同事的阅读模块 PRD：报道「搭建事件时间线」，记叙「事件卡
		// 排序；切换发生顺序／讲述顺序」。板摆完就算做完，和标注步同一条判据
		// （代码里也兜着，见 answeredOrderBoard 的调用点）。
		rule = "本步用一块 order_events 排序板把几件事交给她，请她按发生的先后排好；给板的那一轮不要说出正确的先后。" +
			"已收到排序板的真实作答时，先接住她的排法，再用一两句说清**讲述顺序和发生顺序哪里不一样、作者为什么这样安排**" +
			"（她排错了一处，就指出原文哪几个字能看出先后），然后给 done。**不要再发第二块排序板。**" +
			"普通文字说排好了不能替代真实作答。"
	case string(taskLens):
		rule = "已收到【她刚做完一副透镜】时，反馈她的实际分析并给 done，不再要求她操作已完成的透镜；没有完成回传时按透镜步骤继续。"
	}
	return "\n【本轮推进判据】\n当前步骤：" + current.Kind + "；任务：" + current.Label + "。\n" +
		"先检查学生是否明确要求跳过当前步骤：如是，advance 必须为 skipped。单独说「我放弃」是索答，不是跳过；明确索答要直接回答，advance 留空。否则：" + rule + "\n" +
		"概念或词义提问可以直接解释；解释不算学生已经完成分析任务。学生已经完成时，advance 必须为 done；不能因介绍下一步而把 advance 留空。明确跳过时不要附加 card 或 lens。一次只推进当前一步。\n" +
		readingNextStepHandoff(tasks, current)
}

// readingNextStepHandoff gives a completed step a concrete next action in
// the same reply. Skipped steps are explicitly excluded by the preceding
// current-step instruction.
func readingNextStepHandoff(tasks []sqlc.ReadingTask, current *sqlc.ReadingTask) string {
	var next *sqlc.ReadingTask
	for i := range tasks {
		if tasks[i].Status == "pending" && tasks[i].Position > current.Position {
			if next == nil || tasks[i].Position < next.Position {
				next = &tasks[i]
			}
		}
	}
	if next == nil {
		return "仅当 advance 为 done 且这是最后一步时，用一两句收尾：说清她这一篇完成了什么，不再布置任务。\n"
	}
	return "仅当 advance 为 done 时，reply 的最后一段直接交接下一步：「" + next.Label + "」（" + next.Detail + "）。" +
		"说明这一步为什么值得做、她现在要做什么；能用一张 card 承担就给下一步的 card。不要等她回一句「好」才开始。\n"
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

// readingCoachToolMenu renders the paragraph tools for the article's language
// into the coach's own prompt, from the same table the explain endpoint
// validates against — so an id the coach names is always an id the endpoint
// will accept.
func readingCoachToolMenu(lang string) string {
	var b strings.Builder
	for _, t := range readingBlockToolsFor(lang) {
		b.WriteString("- tool=" + t.ID + " · " + t.Label + "\n")
	}
	return b.String()
}

// readingCoachLensMenu renders the reading deck — id + name + when to reach
// for it — from the registry-backed agent.ReadingDeck(), the same
// single-source-of-truth shape readingCoachToolMenu uses for the paragraph
// tools. If the deck can't be resolved, an empty string is returned: the
// coach then simply never names a lens, the safe direction to fail in.
func readingCoachLensMenu() string {
	deck, err := agent.ReadingDeck()
	if err != nil {
		return ""
	}
	var b strings.Builder
	for _, c := range deck {
		b.WriteString("- lens=" + c.CardID + " · " + c.Name + " · " + c.Trigger + "\n")
	}
	return b.String()
}

// buildReadingCoachSystem assembles the coach's system prompt for the
// article's language. Both placeholders are filled with strings.Replace at
// count 1 each — not fmt.Sprintf, and each call targets its own distinct
// token (%s for the paragraph-tool menu, %LENS% for the lens menu) so the
// second substitution never collides with or re-consumes the first.
func buildReadingCoachSystem(lang string) string {
	system := strings.Replace(readingCoachSystem, "%s", readingCoachToolMenu(lang), 1)
	system = strings.Replace(system, "%LENS%", readingCoachLensMenu(), 1)
	return system
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
	if cur := currentReadingTask(tasks); cur != nil && !anyOpen && !toolAnswerTurn && parsed.Card == nil && parsed.Lens == "" &&
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
	if !anyOpen && !toolAnswerTurn && !boardOpen && parsed.Card == nil && parsed.Lens == "" && replyPromisesACard(parsed.Reply) &&
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

	parsed.Card = dropAlreadyPlaced(parsed.Card, lastBoardPlacement(msgs))

	// 标注板的格子换成这篇体裁的那一套。议论文原样不动。见 fitBoardToGenre。
	parsed.Card = fitBoardToGenre(parsed.Card, genre)
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
		if advance == "" && current.Kind != string(taskHunt) &&
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
			replyLooksCutOff(parsed.Reply)),
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

	// F4: the card and the scroll must agree. When a lens actually minted this
	// turn, it is aimed at parsed.FocusBlock (parseReadingCoachReply requires
	// that pairing), and the response's focusBlock must stay there — not jump
	// to the NEXT step's paragraph, which is a different one on any turn that
	// both advances into a focus_block step and summons a lens. Only absent a
	// minted lens does the plan's own paragraph win over the model's guess:
	// the plan already decided which paragraph the next step is about, and
	// letting a per-turn guess override it would scroll her somewhere the step
	// never meant.
	focus := parsed.FocusBlock
	if cardOut == nil && next != nil && next.BlockID != "" {
		focus = next.BlockID
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
