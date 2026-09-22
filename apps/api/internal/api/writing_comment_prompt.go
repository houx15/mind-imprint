package api

// Prompt assembly for writing_comment.go. Pure: no database or model calls.
// Static teaching text lives in internal/prompts; context selection is separate
// where a feature has a dedicated *_context.go file.

import (
	"fmt"
	"strings"

	"mindimprint/api/internal/prompts"
	"mindimprint/api/internal/store/sqlc"
	"mindimprint/api/internal/teachingvoice"
	"mindimprint/api/internal/vocab"
)

// writingCommentRules 是这一刀的四条规矩。
//
// 🚨 **不整条照抄 writingGuideTeachingRules。** 那四条是给「教一个方法、
// 让她去写」那个场合写的，其中「给她一个真的选择」在这里不成立——
// 她已经写完了，这一轮要做的是判断，不是给选项。
// 2026-09-11 刚栽过一次同形的跟头（兴趣测试整条照搬采集的 prompt，
// 把「不是这篇材料的话题」也带了过去，真模型 0/3）：
// **共用要按段落挑，不是整条照搬。**
const writingCommentRules = prompts.WritingCommentRules

const writingCommentSystem = prompts.WritingCommentSystem

// buildWritingCommentSystem 把症状表和这一次允许的 issue 条数填进去。
//
// 表按 writing.lang 选（中文一张、英文一张，见 writing_symptoms.go），
// 所以这个 prompt 不是常量——一篇英文稿子拿到的是 IELTS 那 13 个能力 id，
// 一篇中文稿子拿到的是 qifeng 那 18 条诊断。
// kind 是她停在的那一块是什么（writing_kind.go 的闭表）。空串 = 通篇审阅那一路，
// 或者一个没有结构图节点的自由段落 —— 那时候不附分块的检查表。
// help 是这一轮该用哪种帮法（writing_stall.go）。helpAsk 什么都不加。
func buildWritingCommentSystem(lang string, maxIssues int, kind string, help writingHelpMode, genre string) string {
	return fmt.Sprintf(writingCommentSystem, writingSymptomCatalog(lang, genre), maxIssues) +
		writingCommentBlockJob(kind) +
		writingHelpModeBlock(help, lang, genre) + teachingvoice.Rules
}

// writingCommentBlockJob 是**这一块的活**：每一种块该查什么，不该查什么。
//
// 🚨 同事 2026-09-20 的意见 9，原话：
//
//	「标一下这一段功能里，对每一段的分析需要明确各个段落各自的功能，
//	  而且也要知道其他的段落写了什么。」
//
// 前半句是这个函数，后半句是 writing_piece_context.go。两件事都要有：
// 光知道后面的段写了什么，而不知道「开头的活是什么」，它还是会拿正文的标准
// 去量开头。
//
// 「限制」和「与其他段的关系」两项来自 docs/2026-08-09-all-statuses.md §6
// （statement complete：for each claim, we need evidence, analysis,
// **limitation**, how it **correlates with others**）。
func writingCommentBlockJob(kind string) string {
	switch kind {
	case writingKindOpening:
		return `

## 这一块是**开头**，检查两项作用

1. 让读者愿意读下去；
2. 表明文章要讨论的观点，并与题目相关。

**不要求开头自带完整的事例。** 开头是引子，具体的事写在后面的段里 ——
你上面已经看得到那几段写了什么。后面确实有那件事，就**不要**在这里说
「没有一件具体的事」；后面也没有，那是后面那几段的问题，不是这一段的。

也不要要求开头把全文的理由先列一遍。开头可以介绍主题，不必列出全部分论点。`

	case writingKindPoint, writingKindCounter:
		return `

## 这一块是**正文的一段**，按语文课的五句型看

一个主体段由五种句子组成，各有不同作用：

1. **观点句**：这一段要证的那一句，在段首。应与中心论点的关键词相关。
2. **阐释句**：解释观点句中抽象概念的具体含义，为后面的材料作说明。
3. **材料句**：一件具体的事、一份材料。谁、做了什么、结果怎么样。
4. **分析句**：说明这个例子与本段观点之间的关系。
5. **结论句**：呼应观点句，保持关键词的含义一致。

需要补充解释时，说明相应句子的作用和正式名称。解释例子与观点关系的是**分析句**；
解释观点中抽象词语的是**阐释句**。先检查原文是否已经表达了这些内容。

缺的是分析句的时候，**点名一种写法**，别只说「要分析」：

- 材料只摆了发生过什么 → **因果分析法**：是什么……？是……。因为……，所以……
- 材料里已经有结果了 → **假设分析法**：假如……，那么……？
- 列出了多个例子 → **归纳分析法**：这些……，真正表现了……

外加两项：
- **限制**：结论超出了材料所能说明的范围（「所有」「一定」「每个人」）的时候指出来。
- **和别的段的关系**：它和上一段是不是在说同一件事（你上面看得到别的段
  写了什么，重复了就说出来）。

**五句不是五条意见。** 一次只说最要紧的那一处，不要把例子、解释、让步
机械拆成三条。

**五句也不是一张验收清单。** 它说的是一个写完的主体段长什么样，不是
「少一句就不合格」。少一两句 ⇒ **polish**。只有这一段偏离分论点、或材料无法说明观点时，才判为 revise。
已有时间、地点和人物的具体材料，仅需补充分析句时，判为 polish。

**观点句可能写在这一块的卡片上，不在正文里。** 她在图上给这一块起的
名字就是这一段的分论点。卡片上已经有了，就不要判她「没有观点句」；
可以提一句「正文里也说一次，读文章的人看不见你的卡片」，但那也是 polish。`

	case writingKindRebuttal:
		return `

## 这一块是**对反方的回应**

只看一件事：她回应的，是不是反方**真正说的那一点**。
回应了一个反方没说过的、更弱的说法，这一处要指出来。`

	case writingKindClosing:
		return `

## 这一块是**结尾**

只看一件事：它有没有**收束全文** —— 回到中心论点，而且说得比开头更准一点。
把前面说过的话原样再说一遍，不算收束。

需要调整结尾时，介绍一种适用方法并解释用途：首尾呼应（回应开头摆过的那几样
东西）、名言警句（引一句，再接一句自己的话）、总结归纳（把前面各段证到的
收成一句）、修辞收束（用排比或比喻再说一次）。

不要要求结尾引入新的证据。新证据出现在结尾，是结构问题，不是优点。
结尾不超过两百字。她写长了、或者结论未与全文内容相关，这一处要指出来。`

	// —— 记叙文那四块（R4）——

	case writingKindScene, writingKindTurn:
		return `

## 这一块是**记叙文里的一件事**

按三项看：

1. **具体**：有没有动作、神态、说过的话，还是只有「很感动」「特别好」
   这一类直接说出来的感受。具体细节可以帮助读者理解人物的动作和感受。
2. **动词准不准**：「弓着身子蹒跚着走来」和「走过来」是两件事。
   笼统的动词要指出来，并且给出一个更准的。
3. **环境是否帮助理解事件**：天气、时辰、周围的声音是否体现人物的处境。
   请她回忆真实细节，不要求编造困难来突出人物。

**不要拿议论文的标准量它。** 这一段不需要论点，不需要「这件事证明了什么」。
这一段需要清楚描述事件，使读者理解当时的情形。意义留给后面的感悟那一段。

不要建议她加一件新的事。需要展开时，请她回忆当前事件中相关的真实细节。`

	case writingKindDetail:
		return `

## 这一块是**一处细节**

按讲义的三条禁忌看：**真实**（是真的发生过的吗）、**典型**（是不是只有他
才会这样）、**独特**（是否体现了这个人物在当时的具体表现）。

三条里哪一条没过，就指哪一条，并且给出一个更准的动词或修饰词。`

	case writingKindFeeling:
		return `

## 这一块是**感悟**

只看两件事：
1. 她说出来的新认识，是否与前面所写事件有明确联系。
2. 有没有**呼应开头**。开头写的那份不满，结尾要让读者看见她确实变了。

「我们要珍惜身边的人」这一类感悟，如果前面的事件不足以解释它，
这一处要指出来。`
	}
	return ""
}

// buildWritingCommentPrompt assembles the user turn shared by both zoom
// levels: title, target words (only if set, never invented — W-R7), then the
// text itself under a caller-supplied label ("她写的这一段" vs "她的整篇稿子")
// so the model knows which zoom level it is looking at.
// 🚨 2026-09-20 加了 `piece` —— 这个 builder 原来的全部上下文是题目、语言、
// 字数、方法表、**这一段的正文**，别的段写了什么它一个字都看不见。
// 同事的意见 9 就是这一条的直接后果：开头段被判「没有一件具体的事」，
// 而那件事写在第二段里。空串 = 不给整篇（通篇审阅那一路本来就拿得到全文）。
func buildWritingCommentPrompt(wr sqlc.Writing, label, text, piece, genre string) string {
	var b strings.Builder
	b.WriteString(writingTopicLine(wr, "题目："))
	b.WriteString(writingLangLine(wr))
	b.WriteString(writingLengthLine(wr, "目标字数"))

	// 🚨 【可用的方法】必须真的出现在这里。
	//
	// 系统提示词从一开始就写着「method 取自【可用的方法】的 id」，但这个
	// builder 从来没把那张表放进去过——2026-09-11 的 LIVE_LLM 实测一眼看穿：
	// 真模型回了 `"method":"concrete_data"` 和 `"specific_detail"`，
	// 两个都不存在，于是 validateCommentPoints 把这个字段清空，
	// 「说出她刚才用对的是哪一个方法」这件事**一次都没发生过**。
	//
	// 单元测试对这个是绿的（我喂的 JSON 里写的是真 id），屏幕上也看不出来
	// （少一个方法名而已）。这正是那条「prompt 里的必须要能在代码里验」
	// 反过来的一面：能验，但得先把可选项给它。
	//
	// 按语言过滤，理由同 writing_plan.go：一句英文句式出现在中文作文的意见里
	// 是个 bug。
	b.WriteString("\n【可用的方法】（method 只能从这里挑 id，别自己造词）\n")
	for _, m := range vocab.ForLang(wr.Lang, genre) {
		b.WriteString("- " + m.ID + "（" + m.Label() + "）：" + m.Definition + "\n")
	}

	// 整篇上下文排在方法表（稳定）之后、她的正文（每轮都变）之前 ——
	// 见 writing_piece_context.go 顶上那条成本契约。
	b.WriteString(piece)

	b.WriteString("\n" + label + "：\n" + text + "\n")

	// 字句层面的重复，服务端数出来当事实给它 —— 见 writing_repeats.go。
	// 论点层面的重复（middle_collapse / ending_only_summary）和连贯
	// （paragraph_jump / reference_linking）本来就在症状表里，缺的是这一层。
	b.WriteString(writingRepeatBlock(text, wr.Lang))
	b.WriteString("\n输出前逐条核对：每条 point 的 quote 必须从本次正文逐字摘取；symptom 必须是本次系统问题表里的 id。summary 只描述正文已写出的内容，所有具体问题放在有原文引文的 points 中。summary 避免「缺」「不足」「尚未」「找不到」等缺失表述；即使学生讨论的是现实中的短缺问题，也概括为讨论对象与表达方式，例如「通过走廊自习的经历讨论图书馆开放时间」，不要把缺失词放入总评。反馈解释具体内容的关系；不得要求她添加原文未表明的事实。\n")
	return b.String()
}
