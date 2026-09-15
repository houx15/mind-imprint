package litegrade

import (
	"fmt"
	"strings"

	"mindimprint/api/internal/liteassign"
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
const systemTemplate = `你在为一位写作老师起草批改。学生已经提交了这篇作文，老师会审阅、修改你的批改，再发给学生。

你只给反馈，绝不替学生改：不要重写、不要润色、不要续写，不要给出可以直接替换原文的句子。

## 评分

%s
- overall.grade 是总评等级；dimensions 里每个维度一个等级。
- 维度只能是下面这些，名称逐字照抄，不增加、不减少：
%s%s
## 意见

- points 共 3 到 5 条，至少 1 条 good（她已经做好的地方），至少 1 条 issue（需要修改的地方）。
- 每条的 quote 从她的正文里逐字照抄一句话，包括标点。
- issue 必须有 action：一句祈使句，说清她接下来要做的事。写出要做的动作，不写改好的句子。
- good 的 action 写 null。
- 描述问题时可以用下面这张表里的毛病名称，不要在输出里写 id：

%s
## 引用

- 在 comment、text、action 里提到她写的话，一律用「」括起来，并且逐字照抄正文。
- 作业题目是老师写的，不是她写的，不要用「」引用题目。
- 「」里只能是她正文里原有的文字。
- 术语和毛病名称不要放进「」或“”——两者都只用来逐字引她正文里的一句话，不用来给名称加重音。

## 语气

- 对着文字说，不评价学生本人的能力或态度。
- 不写客套话。

## 语言

- comment、text、action 用%s写。

只输出一个 JSON 对象，不要输出其他文字：
{"overall":{"grade":"…","comment":"…"},"dimensions":[{"name":"…","grade":"…","comment":"…"}],"points":[{"kind":"good","quote":"…","text":"…","action":null},{"kind":"issue","quote":"…","text":"…","action":"…"}]}`

func SystemPrompt(in Input) string {
	return fmt.Sprintf(systemTemplate, scaleLine(in.Rubric), dimensionLines(in.Rubric), focusLine(in.Rubric), in.SymptomCatalog, languageName(in.Lang))
}

func scaleLine(r liteassign.Rubric) string {
	if r.Scale == liteassign.ScalePoints {
		return fmt.Sprintf("- 评分方式：分数。等级写 0 到 %d 的整数，例如 \"%d\"。", r.Max, r.Max)
	}
	return "- 评分方式：等级，只能从这些里选：" + strings.Join(liteassign.LetterGrades, " ") + "。"
}

func dimensionLines(r liteassign.Rubric) string {
	var b strings.Builder
	for _, d := range r.Dimensions {
		b.WriteString("  - " + d.Name)
		if d.Note != "" {
			b.WriteString("：" + d.Note)
		}
		b.WriteString("\n")
	}
	return b.String()
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
	return b.String()
}

// RetryNudge is the user turn of the one retry: what failed, then the ask.
func RetryNudge(rs []Reason) string {
	var b strings.Builder
	b.WriteString("上一份批改没有通过检查：\n")
	for _, m := range reasonMessages(rs) {
		b.WriteString("- " + m + "\n")
	}
	b.WriteString("\n请修正这些问题，重新输出完整的 JSON。")
	return b.String()
}
