package litegrade

import (
	"encoding/json"
	"fmt"
	"strings"

	"mindimprint/api/internal/liteassign"
	"mindimprint/api/internal/prompts"
	"mindimprint/api/internal/teachingvoice"
	"mindimprint/api/internal/textstat"
)

// systemTemplate is the instructions sent with every 批改 call. Check (see
// check.go) verifies only some of what it asks for: the grade scale, the
// rubric's dimension names, the point count and the good/issue mix, a
// required action on every issue, every quote and every long-enough 「」
// quotation being hers once normalized (or, failing that, the teacher's
// prompt, which gets its own reason — a short quotation, a term or a
// symptom-catalog name, isn't checked at all), sentences that judge her
// instead of her writing (only the phrases PersonJudging recognises), and
// the feedback being mostly written in the writing's language. "不要重写、
// 不要润色、不要续写" and "不写客套话" below are prompt-only — there is no
// code check for either.
//
// points[].dimension / .symptom aren't checked here either —
// SanitizeProvenance (check.go) clears either instead of failing the
// grading over them; see prompts.GradingSystemTemplate's doc comment.
//
// The countable measures (internal/textstat) are NOT here: factsBlock goes
// into the USER message — see prompts.GradingFactsBlock for why. Check does
// not gate on them either; the four numbers only steer wording, they carry
// no contract of their own.
//
// 🚨 **This template is byte-identical for every student in one assignment.**
// Keep it that way: anything per-student belongs in UserPrompt. A volatile
// block in here breaks the implicit prefix cache for the whole class batch
// (AGENTS.md 「prompt 块的顺序是一条成本契约」).
const systemTemplate = prompts.GradingSystemTemplate

func SystemPrompt(in Input) string {
	return fmt.Sprintf(systemTemplate, scaleLine(in.Rubric), dimensionLines(in.Rubric), focusLine(in.Rubric), in.SymptomCatalog, languageName(in.Lang), dimensionSkeleton(in.Rubric)) + teachingvoice.Rules
}

// factsBlock renders the countable measures internal/textstat computes from
// her submitted body (Input.Body) — the model is told not to recompute
// them; see prompts.GradingFactsBlock for the instruction itself.
//
// 🚨 These numbers inform the model's judgment and, via points[].dimension /
// .comment prose, the teacher's 依据 view. They are never rendered to the
// student as a score — the product owner ruled out exact scores for
// students (task-4-brief.md); the prompt tells the model the same thing
// ("不要把这里的具体数字写进给学生看的内容里").
//
// 🚨 中文那一路的「词」其实是**字**：textstat 的分词规则（wordTokens，照
// agent.CountWords）把每个汉字算一个 token，所以中文的多样度是字种比、
// 句长是字数。标签按语言分开写，不让两种语言的数字看起来可比。
func factsBlock(in Input) string {
	s := textstat.Compute(in.Body, in.Lang)
	unit, variety := "字", "用字多样度（不重复字数 / 总字数，中文按字计）"
	if in.Lang == "en" {
		unit, variety = "词", "词汇多样度（不重复词数 / 总词数）"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "- %s：%.0f%%\n", variety, s.TypeTokenRatio*100)
	fmt.Fprintf(&b, "- 平均句长：%.1f %s\n", s.MeanSentenceLength, unit)
	fmt.Fprintf(&b, "- 含从句的句子占比（按连词词表匹配）：%.0f%%\n", s.ComplexSentenceRatio*100)
	fmt.Fprintf(&b, "- 连接词密度（按连接词词表匹配）：每句 %.1f 个\n", s.ConnectiveDensity)
	return b.String()
}

func scaleLine(r liteassign.Rubric) string {
	if r.Scale == liteassign.ScalePoints {
		return fmt.Sprintf("- 评分方式：分数。等级写 0 到 %d 的整数，例如 \"%d\"。", r.Max, r.Max)
	}
	return "- 评分方式：等级，只能从这些里选：" + strings.Join(liteassign.LetterGrades, " ") + "。"
}

// dimensionLines names each dimension as the exact name value, with its note
// apart. Written as 「- 内容：立意是否明确…」 the model copied the whole line
// into name on both attempts of a grading (2026-09-17).
func dimensionLines(r liteassign.Rubric) string {
	var b strings.Builder
	for _, d := range r.Dimensions {
		name, _ := json.Marshal(d.Name)
		b.WriteString("  - name 写 " + string(name))
		if d.Note != "" {
			b.WriteString("；这一维看：" + d.Note)
		}
		b.WriteString("\n")
	}
	return b.String()
}

// dimensionSkeleton is the dimensions array of the output example with the
// rubric's names already in place.
func dimensionSkeleton(r liteassign.Rubric) string {
	parts := make([]string, 0, len(r.Dimensions))
	for _, d := range r.Dimensions {
		name, _ := json.Marshal(d.Name)
		parts = append(parts, `{"name":`+string(name)+`,"grade":"…","comment":"…"}`)
	}
	return strings.Join(parts, ",")
}

func focusLine(r liteassign.Rubric) string {
	if r.Focus == "" {
		return ""
	}
	return "- 老师这次的批改重点：" + r.Focus + "\n"
}

func languageName(lang string) string {
	if lang == "en" {
		return "英文"
	}
	return "中文"
}

// UserPrompt: the teacher's prompt, labelled as the teacher's, then her version.
func UserPrompt(in Input) string {
	var b strings.Builder
	if in.AssignedPrompt != "" {
		b.WriteString("作业题目（老师布置，不是学生的原文）：\n" + in.AssignedPrompt + "\n\n")
	}
	if in.Title != "" {
		b.WriteString("标题：" + in.Title + "\n")
	}
	if in.TargetWords > 0 {
		fmt.Fprintf(&b, "目标字数：%d\n", in.TargetWords)
	}
	fmt.Fprintf(&b, "\n学生正文（第 %d 版）：\n%s\n", in.VersionNumber, in.Body)
	// 🚨 统计块排在学生的正文**后面**，而且只在这条用户消息里 —— 它是每个学生
	// 都不一样的东西，放进系统提示词会把整个班共用的前缀打碎（见
	// prompts.GradingFactsBlock）。
	fmt.Fprintf(&b, prompts.GradingFactsBlock, factsBlock(in))
	b.WriteString("\n请输出完整对象：points 最多 5 条，每条都应有原文依据。不要求同时包含 good 和 issue；没有需要单独指出的内容时可返回空数组。总评与各维度仍需完整填写。\n")
	return b.String()
}

// RetryNudge is the user turn of the one retry: what failed, then the ask.
func RetryNudge(rs []Reason) string {
	var b strings.Builder
	b.WriteString("上一份批改没有通过检查：\n")
	for _, m := range reasonMessages(rs) {
		b.WriteString("- " + m + "\n")
	}
	for _, r := range rs {
		if r.Code == ReasonUnparseable {
			// Measured 2026-09-17: the review model put a stray
			// "dimensions_placeholder": null inside the dimensions array, and
			// told only 「不是有效的 JSON」 it broke the retry the same way.
			if r.Detail != "" {
				b.WriteString("JSON 解析错误：" + r.Detail + "\n")
			}
			b.WriteString("dimensions 和 points 都是数组，数组里只能放 {…} 对象，不能放「\"键\": 值」；不要添加格式之外的键。\n")
			break
		}
	}
	b.WriteString("\n请修正这些问题，重新输出完整的 JSON。")
	return b.String()
}
