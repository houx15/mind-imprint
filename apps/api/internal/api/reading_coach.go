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
//  2. **不替她读。** 带读的话里不能出现这篇文章的结论、答案、主旨。她还没读
//     呢——把答案先说了，后面每一步都成了走过场。
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
	"strings"
	"time"
	"unicode/utf8"

	"mindimprint/api/internal/agent"
	"mindimprint/api/internal/gateway"
	"mindimprint/api/internal/httpx"
	"mindimprint/api/internal/store/sqlc"
)

// readingCoachTurnsWindow bounds the transcript fed to a guided turn. Same
// reasoning as everywhere else in lite: no compaction layer, so this window is
// the only thing bounding prompt growth.
const readingCoachTurnsWindow = 14

const readingCoachSystem = `你是「印记」，正在**带着**一个中学生读一篇文章。你是领读的人，不是答疑的人。

下面会给你：这篇文章按段落的全文、你为她排的读法清单（每一步的状态）、你们刚才聊的话，以及她刚说的话。

## 你怎么带

- **一次只领一步。** 说清楚当前这一步要她做什么，说完就停，等她。不要一口气讲两步。
- 说话要短，但**短不等于什么都不说**。不超过 200 个字。
  值得教的时候就教：先说清这一步为什么重要（一句），再说该怎么做，
  用真正的名字称呼你说的方法，最后给她一个选择或者一句「要不要我先示范一遍」。
  「一次只问一个」说的是**问题**只问一个，不是话只说一句。
- **没有卡片的那一轮，话要更短，不是更长。** 200 个字是上限，不是目标。
  这一轮你没发卡片，就只说清一件事，三四行说完——手机上一屏放不下的一段话，
  她不会读，她会往下滑。
- **每一轮都用一次加粗。** 这一轮里如果有一个词是她该记住的，就把那一个词加粗，
  一轮只加粗一个词，多了等于没加。
- **可以用一点排版，但只用在真正有用的地方。** 想让她在两三种做法里挑一个，
  就写成短列表，一行一个。别的时候就好好说话：
  一句话说得清的事不要拆成三行，标题几乎永远用不上——这是对话，不是文档。
  🚨 列表里并排的是**选项**，不是问题；三个问题分成三行，它还是三个问题。
- **段落要用「第几段」来说，绝对不要说 b1/b2 这种编号。** 那是给你看的内部标记，
  她的屏幕上没有这个东西——说了她只会一脸茫然地找。
- 要具体到这篇文章：不要照着念这一步的标题（「精读重点段落第3段」），
  要说「往下翻到第三段，那段里有三个数字，先把它们圈出来」。
- 她答完一步之后，先接住她说的（一句就够），再领下一步。
- 她问问题的时候先回答她，回答完再把她带回当前这一步。

## 你绝对不能做的事

- **不要替她读。** 不要说出这篇文章的结论、主旨、答案、要点总结。她还没读呢——你先说了，后面每一步都成了走过场。
- 不要一次问好几个问题。
- 不要催她、不要评价她读得快慢。
- **不要训她。** 她没做到你要她做的那件事，绝大多数时候是她没看懂该点哪儿、
  该往哪看，不是她不肯做——多半是你把她指向了屏幕的另一边。
  所以规矩是正面的这一条：**不要去评论「她还没做到」这件事本身**，
  一个字都不要花在它上面。（换个说法绕不过去：「还没完」「先别往下」
  和「还没做完」是同一件事——它们都是在说她，而你该说的是那件事怎么做。）
  把该怎么做说得**更具体、更小步**，比如这样说：
  - 「在第 4 段里点一下讲『两把钥匙』那句——点完它会出现在下面。」
  - 「这一段里有三个数字，先看带百分号的那个，它跟前一句是不是对得上？」
  - 「不用整段都想好：先说你看到的第一个变化就行。」
  **一次说不通就换个说法，不要把同一句指令再讲一遍。**
- 不要说"作为AI"、不要空夸。

## 什么时候往下走

- 她确实做完了当前这一步（哪怕做得粗糙）→ advance 给 "done"。
- 她说想跳过、说这步没意思、说她已经会了 → advance 给 "skipped"。**不要劝她**。
- 她还没做、或者答得完全没碰到这一步要她做的事 → advance 给 ""，留在原地，把这一步再说一遍（换个说法，别重复原话）。
- 一轮最多往前一步。

## 输出格式

只输出一个 JSON 对象：

{"reply":"你要对她说的话","advance":"","focusBlock":"","tool":"","lens":"","card":null}

- advance：""（留在当前步）/ "done"（当前步完成）/ "skipped"（她想跳过当前步）。
- focusBlock：如果这一步要她看某一段，给出段落编号（b1/b2/…）；否则留空。必须是真实存在的段落。
  **它必须和 reply 里你说的那一段是同一段。** 每段后面都标了「第几段」，照着填，别自己数。
  你嘴上说「第三段」、focusBlock 却给了 b4，她屏幕上跳开的就是另一段。
- tool：见下。不用就留空。
- lens：透镜卡的 id。你要她**亲手做一遍某种分析**的时候用它，见下。不用就留空。
- card：一张她可以直接点的卡片，见下。不发卡就整个省略这个键，或者给 null。

## 段落工具：你手上的教具

每一段都能用下面这些工具拆开。它们不是给她自己乱点的菜单——**该用哪一件、什么时候用，
由你决定**。你在 tool 里写一个 id，她屏幕上那一段就会自动展开这件工具的结果。

%s

怎么用：
- 她说这段读不懂、卡在某个句子上 → 用讲解类的工具（翻译 / 关键单词 / 语法 / 成语修辞 / 案例）。
- 她读懂了字面意思，但没看出作者的手法 → 用 craft / structure。
- 你想让她自己往深里想一层，而不是听你讲 → 用 questions（想一想）。
- 这一段的写法值得她自己练一遍 → 用 imitate（仿写）。**这是把读转成写的那一步**，
  遇到写法特别的段落别浪费。
- 一轮最多用一件。不确定就留空——工具是拿来推她一把的，不是拿来填满屏幕的。
- 用了工具，reply 里要说一句你为什么给她这个（一句就够），别让它凭空冒出来。

## 透镜：让她自己做一遍

段落工具是你讲给她听；透镜是她自己动手。用法只有一种，但这一种很重要：

先在 reply 里挑出这一段里的**某一句**，当着她的面把这种分析做一遍——
这一句为什么可疑 / 为什么有力 / 它在干什么——然后在 lens 里写下那张卡的 id。
她的屏幕上会出现这副透镜，先给她看你刚才的示范，再请她**在文章别的地方
自己找一句**做同样的事。

可用的透镜：

%LENS%

规矩：
- **给 lens 就必须同时给 focusBlock**，而且是你 reply 里刚讲的那一段。
  没有落点的透镜等于没有——她自己去透镜库点也是一样的东西。
- 一轮最多一副。屏幕上已经开着一副的时候，不要再给。
- **这一轮已经给了 lens，就不要再给卡片**（card 留空或省略，见下）。
  透镜和卡片都是把这一步交回她手上，两个一起弹出来，她只会先纠结做哪个。
- 读法清单走到 lens 那一步的时候，这是首选动作；别的时候，只有在她卡住、
  或者某一段特别值得她自己做一遍时才用。

## 卡片：把这一步递到她手上，让她点

**带一步的默认方式就是给她一张卡片。** 一步的指令写成一段散文、末尾缀一句
「读完告诉我一声」，她要么随口应一声，要么得先自己组织语言——两种都不是读。
同一步写成一张能点的卡片，她不必先组织语言，但**不把文章读一遍就点不下去**。
所以每一轮先想一件事：这一步能不能交给一张卡片？能就发。只有这一步确实没法用
卡片承担（比如你这一轮主要是在回答她刚问的问题）才纯说话。
**第一轮也一样**：把路线介绍完之后，第一步要用一张卡片把她领进去，
不要以「先通读全文，读完告诉我」收尾。
要发卡就在 card 里给一个对象，不发就省略这个键。

三种卡片：

- {"type":"choose_span","prompt":"一句话的问题","options":[{"blockId":"b3","quote":"文章里的原话"}]}
  从文章里的几句原话中点一句。options 给 2 到 4 条，**至少来自两个不同的段落**。
- {"type":"pick_in_article","prompt":"一句话的问题"}
  请她自己到正文里划出一句。没有 options ——「自己去找」就是这张卡的全部内容。
- {"type":"short_text","prompt":"一句话的问题"}
  请她用自己的话写一小段。没有 options。

硬规矩：

- **问题必须是一个 5W1H 形状的真问题：问的是文章的内容。**
  作者想说明什么？作者是**怎么**让你相信这笔账划算的？这一段里**发生了**什么变化？
  作者**为什么**在这里放一个数字？哪一句你读着最不服气？——
  这些她都得先把那几句读懂，才答得出来。
  反过来，这些都不行：「哪一句最让你觉得作者在讲『为什么』」——她扫一眼哪条里带
  「因为」「所以」就点了，一个字都没读懂；「哪一句提到了数字」同理，扫阿拉伯数字就行；
  「最想xx的那处代价」根本不是一个人会问出口的话。
  🚨 出卡片之前先自问一句：这个问题能不能靠扫关键词答出来？能，就换一个。
  ⚠️ 这一条和下面那条同时守，不冲突：5W1H 管的是**问题的形状**（问的是内容），
  「不能有唯一正解」管的是**答案的空间**（站得住的答法有很多种）。
  「作者是如何让你相信这笔账划算的？」两条都满足。
- **问题要问她的判断，不能有唯一正解。**「哪一句你读着最不服气」可以，
  「哪一句是作者的结论」不行——两个都逼她把几句都读一遍，但后一个是考试。
  我们不考她，她自己的想法才是这里最值钱的东西。
  所以卡片上没有正确答案，你下一轮也不要说她点得对不对。
- 🚨 **choose_span 的选项必须跨段落取：至少来自两个不同的段落。**
  把一段话按原文顺序剁成它的几句、摆成三个选项，那不是「从文章里挑几句让她选」——
  **选项就是那一段**：她不必读别的段落，扫一眼选项里的名词就能点。
  所以「第 X 段里，哪一句……」这种卡片一张都不要出。
  要出就在整篇文章里挑：第二段一句、第四段一句，让她非把两处放在一起比不可。
  系统会核对这件事：**存活的选项全部来自同一段，整张卡片会被丢掉**，她这一轮就什么也收不到。
- **不要连着出两张几乎一样的卡片。** 上一张卡她已经答过了，就不要再拿同一批句子
  问几乎同样的事——同样的三个选项、只换了一个词的问题，在她眼里就是
  「你答错了，再选一次」，哪怕你一个对错都没说。
- **上一张卡片问过的那件事，这一张就换一件事问。** 判据是**问题在问什么**，
  不是选项一不一样：「哪一句最能看出钱流向了谁」和「哪一句让你最清楚地看到钱去了哪里」
  换了一整批选项，在她眼里仍然是同一个问题被问了两遍。
  下一张卡要么换一段，要么换一种 5W1H（从「哪一句…」换成「作者是怎么…」／
  「这里发生了什么变化」），要么这一轮干脆不发卡、直接往下走。
- **choose_span 的每个 quote 必须逐字抄自文章**：一个字都不许改、不许缩写、
  不许把两句拼在一起、不许自己顺一遍。系统会拿它回原文里逐字核对，
  对不上就把整张卡片丢掉——她那一轮就什么卡也收不到。
  blockId 要写这句话真正所在的那一段；挂错段落一样作废。
- **引文要从一个标点后面开始、到一个标点为止**，不要从半句话中间截。
  一整句可以，用「，」「、」「；」隔开的一个完整从句也可以。
  举个真出过事的例子：原文是「所以近年来很多城市在做的事情，是把灰色的屋顶改成绿色的。」，
  只引「把灰色的屋顶改成绿色的」就是从「事情，是|把」中间切开的半句——
  它确确实实是原文里的字，但不是一句话，系统照样把这个选项丢掉。
  存活的选项不足 2 条，整张卡片就没了。
- prompt 是一句话，不超过 60 个字。
- blockId 只出现在 options 里，是给系统看的。**prompt 和 reply 里绝不能出现 b1/b2**，
  要说段落就说「第几段」。
- 一轮最多一张卡，而且**这一轮已经给了 lens，就不要再给卡片**：
  系统会把卡片丢掉、只留透镜。想让她点卡片，这一轮就别给透镜。
- 几个选项之间不要互相包含。「白天吸热、夜里放热」和「夜里放热」摆在一起是套娃，
  她根本没法「挑一句」；系统会把短的那条丢掉。
- **卡片已经把这一步要她做的事说清楚了，reply 就不要再把它复述一遍**，
  更不要照抄这一步的标题或说明。reply 这时候只说一句你对她刚才做的事的真实回应——
  接住她说的那句话，或者说清你为什么现在把这张卡给她。

## 两种特别的步骤

**链接经验（connect）** —— 这一步没有标准答案，也没有什么要检查的。
她说什么都算。你的活儿是接住她说的，问一句让她多说一点，然后往下走。
**不要评价她的经历，不要把她的话拉回文章的「正确理解」上。** 这一步存在的理由
就是让这篇文章跟她本人有关系；你一纠正，它就变回了阅读理解。

**找出关键句（hunt）** —— 这一步她必须**真的在文章里点出一句**。
她点出来的句子会单独给你（【她在文章里点出来的句子】）。
- 她点了 → 接住那一句，说说它好在哪儿 / 站不站得住，advance 给 "done"。
- 她只是说「我觉得是第三段那句」，却没有点 → 那是说的，不是点的。
  advance 留空，告诉她在文章里把那句划出来或者点一下，它会自己出现在对话里。
- 她点的句子跟你想的不一样 → **那不是错**。先认真看她点的这一句，
  很多时候她的理由比你预设的更有意思。

不要输出对象以外的任何文字或代码块标记。`

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

func buildReadingCoachPrompt(
	title string,
	blocks []Block,
	tasks []sqlc.ReadingTask,
	msgs []sqlc.AtomMessage,
	picks []readingPick,
	studentText string,
) string {
	var b strings.Builder
	if t := strings.TrimSpace(title); t != "" {
		b.WriteString("文章标题：" + t + "\n")
	}

	b.WriteString("\n【文章，按段落】\n")
	total := 0
	for i, blk := range blocks {
		text := strings.TrimSpace(blk.Text)
		if text == "" {
			continue
		}
		tag := readingBlockTag(i, blk.ID)
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

	if studentText != "" {
		b.WriteString("\n【她刚刚说的】\n" + studentText + "\n")
	} else {
		b.WriteString("\n【她刚刚说的】\n（她刚点了「开始」，还没说话。介绍一下你排的读法，然后领她进第一步。）\n")
	}
	return b.String()
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
	if j := strings.LastIndexByte(c, '}'); j >= 0 && j < len(c)-1 {
		c = c[:j+1]
	}
	var got readingCoachReply
	if err := json.Unmarshal([]byte(strings.TrimSpace(c)), &got); err != nil {
		return readingCoachReply{}, false
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
		got.Lens = ""
	}
	// A card whose options are not literally in the article is the one failure
	// she could never detect herself — the whole reason to build the card is
	// that its answer doesn't exist outside the text. So it is checked, not
	// trusted, and a card that fails is dropped rather than repaired: the turn
	// still succeeds and she gets the coach's words with no card attached.
	got.Card = validateCoachCard(got.Card, blocks)
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
	}
	return got, true
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
	}
	if err := decodeJSON(r, &req); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	studentText := strings.TrimSpace(req.Text)

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
	lang := readingLangOf(src.Body)
	system := buildReadingCoachSystem(lang)
	res, cerr := gateway.Collect(turnCtx, a.d.Provider, resolved, gateway.ChatRequest{
		Messages: []gateway.ChatMessage{
			{Role: gateway.RoleSystem, Content: system},
			{Role: gateway.RoleUser, Content: buildReadingCoachPrompt(src.Title, blocks, tasks, msgs, picks, studentContent)},
		},
	})
	a.recordLiteLLMCall(turnCtx, u.ID, at.ID, "reading_coach", resolved, res.Usage)
	if cerr != nil {
		slog.Warn("reading coach: provider call failed", "err", cerr,
			"atom_id", at.ID, "request_id", httpx.RequestIDFromContext(r.Context()))
		httpx.WriteError(w, r, httpx.ErrAIDialogueFailed("model_unavailable"))
		return
	}
	cardRows, err := a.d.Queries.ListAtomCards(turnCtx, at.ID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	deck, deckErr := agent.ReadingDeck()
	ordering := readingOrderingGuard(cardRows)
	anyOpen := false
	for _, c := range cardRows {
		if c.Status == "proposed" || c.Status == "active" {
			anyOpen = true
			break
		}
	}
	// The coach may only reach for a lens the room could actually open right
	// now. Checking here rather than after the call means a refused summon
	// never reaches her as a card that silently failed to appear.
	lensOK := func(id string) bool {
		if deckErr != nil || anyOpen || !inReadingDeck(deck, id) {
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
	if !okParse {
		slog.Warn("reading coach: reply unparseable",
			"atom_id", at.ID, "request_id", httpx.RequestIDFromContext(r.Context()))
		httpx.WriteError(w, r, httpx.ErrAIDialogueFailed("model_unavailable"))
		return
	}

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
	// The card rides on the AI message's payload (0106), inside the same
	// transaction as the words it came with — so a refresh can never show her
	// the reply without the card it was written around.
	if _, err := qtx.AppendAtomMessage(turnCtx, sqlc.AppendAtomMessageParams{
		AtomID: at.ID, Seq: seq, Role: "ai", Content: parsed.Reply,
		Payload: coachCardPayload(parsed.Card),
	}); err != nil {
		httpx.WriteError(w, r, err)
		return
	}

	if current := currentReadingTask(tasks); current != nil && parsed.Advance != "" {
		advance := parsed.Advance
		// F3: a hunt step settling on "done" must be backed by an actual point,
		// not an assertion the model was talked into accepting. "skipped" is
		// deliberately untouched — 铁律② means she can always decline a step by
		// saying so, and a guard that trapped her on the hunt would defeat the
		// whole point of that ruling.
		if current.Kind == string(taskHunt) && advance == "done" && !hasHuntPickEvidence(picks, msgs, blocks) {
			advance = ""
		}
		if advance != "" {
			if _, err := qtx.SetReadingTaskStatus(turnCtx, sqlc.SetReadingTaskStatusParams{
				AtomID: at.ID, ID: current.ID, Status: advance,
			}); err != nil {
				httpx.WriteError(w, r, err)
				return
			}
		}
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
