package litegrade

import (
	"encoding/json"
	"fmt"
	"mindimprint/api/internal/teachingvoice"
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
- 维度只能是下面这些，一个不多、一个不少。name 只写引号里的名称，不要把后面的说明写进 name：
%s%s
## 意见

- 依据作业要求和实际文体评价。下面的问题表包含不同文体的可能问题，不是每篇都必须满足的清单；议论文不要因缺少人物动作、情节波折或首尾呼应就判为不足。先核对学生已经写出的分析与限定，不要求她重复已有内容。议论文的具体性体现在事实、范围、来源和推理，不因缺少人物对话、动作描写或描述性词语而扣分，也不把语句朴素本身当作语言问题。
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
{"overall":{"grade":"…","comment":"…"},"dimensions":[%s],"points":[{"kind":"good","quote":"…","text":"…","action":null},{"kind":"issue","quote":"…","text":"…","action":"…"}]}`

func SystemPrompt(in Input) string {
	return fmt.Sprintf(systemTemplate, scaleLine(in.Rubric), dimensionLines(in.Rubric), focusLine(in.Rubric), in.SymptomCatalog, languageName(in.Lang), dimensionSkeleton(in.Rubric)) + teachingvoice.Rules
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
	b.WriteString("\n请输出完整对象：points 必须有 3–5 条，至少 1 条 good 和 1 条 issue；可用两条 good 加一条 issue，不为凑数量虚构问题。各条简洁写明原文依据与用途，输出前核对条数和数组闭合。\n")
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
