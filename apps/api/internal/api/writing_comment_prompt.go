package api

// Writing prompt assembly. These functions render selected context without database or model calls.
// Preserve context selection and the order of stable and changing prompt sections.

import (
	"fmt"
	"strings"

	"mindimprint/api/internal/prompts"
	"mindimprint/api/internal/store/sqlc"
	"mindimprint/api/internal/teachingvoice"
	"mindimprint/api/internal/vocab"
)

const writingCommentRules = prompts.WritingCommentRules

const writingCommentSystem = prompts.WritingCommentSystem

// buildWritingCommentSystem 把症状表和这一次允许的 issue 条数填进去。
//
// 表按 writing.lang 选（中文一张、英文一张，见 writing_symptoms.go），
// 所以这个 prompt 不是常量——一篇英文稿子拿到的是 IELTS 那 13 个能力 id，
// 一篇中文稿子拿到的是 qifeng 那 18 条诊断。
// kind 是学生停在的那一块是什么（writing_kind.go 的闭表）。空串 = 通篇审阅那一路，
// 或者一个没有结构图节点的自由段落 —— 那时候不附分块的检查表。
// help 是这一轮该用哪种帮法（writing_stall.go）。helpAsk 什么都不加。
//
// 🚨 helpShow 那一档摆的句式按**位置**挑，位置从 kind 算（writingKindAppliesTo）。
// kind 是空串的两条路都归到 "body"，这是对的：通篇审阅那一路永远传 helpAsk，
// 一条句式都不会摆出来；另一路是一段没有结构图节点的自由段落，按正文散文处理。
func buildWritingCommentSystem(lang string, maxIssues int, kind string, help writingHelpMode, genre string) string {
	s := fmt.Sprintf(writingCommentSystem, writingSymptomCatalog(lang, genre), maxIssues) +
		writingCommentBlockJob(kind) +
		writingHelpModeBlock(help, writingKindAppliesTo(kind), lang, genre)
	// 中式英语那一节只在英文写作上加 —— 一篇中文作文里没有「回译」这回事。
	// 中文那一支因此逐字节不变。
	if lang == langEnglish {
		s += prompts.WritingCommentChinglishEN
	}
	return s + teachingvoice.Rules
}
func writingCommentBlockJob(kind string) string {
	switch kind {
	case writingKindOpening:
		return `

## 审阅开头
开头帮助读者了解文章的话题、观点和讨论缘由。结合题目与全文，判断这些信息是否清楚。
完整事例与详细论证可以安排在后续段落，开头不必列出所有分论点。请依据已提供的上下文判断，未展示的后文不视为没有写。`

	// 🚨 2026-09-23：「写出它的名称」那一句是补回来的，不是新加的。
	//
	// 这一节在 2026-09-22 的提示词重写里从「按语文课的五句型看」改成了现在这版。
	// 改得对——旧版会让模型把例子、解释、让步一次全说了。但旧版里还有一条
	// 「需要补充解释时，说明相应句子的作用和正式名称」，新版没有对应的落点，
	// 一起没了。线上 e2e（teaching-r4.spec.ts）当场抓到：模型把缺的那一步讲得
	// 很清楚、还点了「因果分析法」，就是从头到尾没说「分析句」三个字，而那条
	// 走查钉的正是这个词。
	//
	// 两件事不冲突，所以两条都留着：一轮仍然只说一处（新版的收敛），
	// 说的那一处要带上名称（旧版的教学）。学生来这儿就是要学这套词
	// （AGENTS.md「界面文案怎么写」第 6 条、memory yinji-must-talk-like-a-teacher）。
	case writingKindPoint, writingKindCounter:
		return `

## 审阅主体段
主体段的任务是说明一个观点，并帮助读者理解理由与材料的关系。可以参考以下五种表达作用，它们不必分别写成五句话：
1. 观点句说明本段重点，通常在段首，与中心论点在意义上相关。
2. 阐释句解释观点中的概念。
3. 材料句提供具体事件、数据或其他材料。
4. 分析句解释材料如何说明本段观点。
5. 结论句概括本段得到的认识。

先检查原文已完成哪些作用，需要帮助时再介绍适用的方法：因果分析法说明结果的形成原因，假设分析法讨论关键条件改变后的结果，归纳分析法比较多个例子的共同特点。依据材料选择方法，不凭例子是否包含结果就机械指定写法。
结合已提供的其他段落检查重复、联系与结论适用范围。每轮优先反馈最影响理解的一处，不按五种作用分别生成意见。
这一处对应上面某一种作用时，在意见里写出它的名称：解释材料与观点关系的是分析句，解释观点中抽象概念的是阐释句。写出名称不改变「一轮只说一处」。
缺少一两种表达作用，但仍能理解观点时，判为 polish。仅需补充分析句的具体材料也判为 polish；本段偏离分论点，或材料无法说明观点时，才判为 revise。
若计划卡片已明确本段观点，不判定学生没有观点。需要在正文中表达这一观点时，可以作为 polish 提出建议。`

	case writingKindRebuttal:
		return `

## 审阅对不同观点的回应
判断回应是否准确针对原文中的不同观点。若回应曲解或过度简化了该观点，指出具体差异，并说明可以怎样调整回应。`

	case writingKindClosing:
		return `

## 审阅结尾
结尾用于综合全文并回应文章讨论的问题。判断它是否体现了前文的论证结果，或仅重复开头的表述。
需要建议时，从适用方法中选择一种并解释用途：首尾呼应联系开头的问题或场景，名言警句须解释引文与全文的关系，总结归纳综合前文结论，修辞收束用适当修辞概括已有内容。
结尾通常不承担展开新证据的任务。出现新材料时，结合其作用判断是否应放入主体段。
结尾篇幅以写作要求和全文比例为依据。两百字可作为一般参考，不能仅因超过该数值就判为需要修改。`

	case writingKindScene, writingKindTurn:
		return `

## 审阅叙事情节
这类段落通过具体事件帮助读者理解当时的情形。关注动作、对话、人物反应和相关环境是否足以说明事件，而不是要求每段都有所有类别的细节。
动词需要准确呈现实际动作，较长或较复杂的词不一定更好。确有表述模糊的地方，说明需要明确哪种动作，由学生选择用词。
需要展开时，帮助学生回忆当前事件中的相关细节，不要求编造困难，也不要求叙事段承担议论文的证明任务。`

	case writingKindDetail:
		return `

## 审阅细节描写
细节用于呈现人物、物件或情境的具体特点。检查描写是否明确、是否与文章主题有关、是否与学生提供的其他信息一致。
人物细节不必是只有此人才有的特征。仅凭文字不能核实的经历，不判为虚构。
需要建议时，指出对应文字并说明怎样描写得更准确，由学生自行选择词句。`

	case writingKindFeeling:
		return `

## 审阅感悟
检查学生表达的认识是否与前文事件有明确联系。开头已表达某种认识或感受时，可结合全文判断结尾是否形成回应。
不预设经历必然带来态度转变，也不要求从不满变为赞许。若感悟较为概括，帮助学生说明它与具体经历的联系。`
	}
	return ""
}
func buildWritingCommentPrompt(wr sqlc.Writing, label, text, piece, genre string) string {
	var b strings.Builder
	b.WriteString(writingTopicLine(wr, "题目："))
	b.WriteString(writingLangLine(wr))
	b.WriteString(writingLengthLine(wr, "目标字数"))
	b.WriteString("\n【可用的方法】（method 须使用此表的 id）\n")
	for _, m := range vocab.ForLang(wr.Lang, genre) {
		b.WriteString("- " + m.ID + "（" + m.Label() + "）：" + m.Definition + "\n")
	}
	b.WriteString(piece)

	b.WriteString("\n" + label + "：\n" + text + "\n")
	b.WriteString(writingRepeatBlock(text, wr.Lang))
	b.WriteString("\n输出前逐条核对：每条 point 的 quote 必须从本次正文逐字摘取；symptom 必须是本次系统问题表里的 id。summary 只描述正文已写出的内容，所有具体问题放在有原文引文的 points 中。summary 避免「缺」「不足」「尚未」「找不到」等缺失表述；即使学生讨论的是现实中的短缺问题，也概括为讨论对象与表达方式，例如「通过走廊自习的经历讨论图书馆开放时间」，不要把缺失词放入总评。反馈解释具体内容的关系；不得要求学生添加原文未表明的事实。\n")
	return b.String()
}
