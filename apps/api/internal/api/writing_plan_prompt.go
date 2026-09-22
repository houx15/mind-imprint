package api

// Writing prompt assembly. These functions render selected context without database or model calls.
// Preserve context selection and the order of stable and changing prompt sections.

import (
	"strconv"
	"strings"

	"mindimprint/api/internal/promptassembly"
	"mindimprint/api/internal/prompts"
	"mindimprint/api/internal/store/sqlc"
)

const writingPlanSystem = prompts.WritingPlanSystem

const writingPlanArgumentKinds = prompts.WritingPlanArgumentKinds
const writingPlanNarrativeKinds = prompts.WritingPlanNarrativeKinds

func writingPlanSystemFor(genre string, lang string) string {
	english := lang == langEnglish
	narrative := genre == genreNarrative

	kinds := writingPlanArgumentKinds
	switch {
	case english && narrative:
		kinds = writingPlanEnglishNarrativeKinds
	case english:
		kinds = writingPlanEnglishArgumentKinds
	case narrative:
		kinds = writingPlanNarrativeKinds
	}

	material, skeleton := writingPlanMaterialZH, writingPlanSkeletonZH
	if english {
		material, skeleton = writingPlanMaterialEN, writingPlanSkeletonEN
	}

	s := strings.Replace(writingPlanSystem, "@@KINDS@@", kinds, 1)
	s = strings.Replace(s, "@@MATERIAL@@", material, 1)
	s = strings.Replace(s, "@@SKELETON@@", skeleton, 1)
	s = strings.Replace(s, "%d", strconv.Itoa(writingPlanMaxNewNodes), 1)
	if english {
		s += "\n这篇是英文写作。reply 讨论写作内容时使用对应的英文术语：中心论点称为 thesis statement，分论点称为 topic sentence，说明材料与观点联系的分析称为 commentary。结合正在讨论的内容使用其术语并附中文解释。节点 text 必须使用英文，对话 reply 用中文。计划检查中的中文标签只是计数名称，不覆盖这些教学术语。\n"
	}
	return s
}
func buildWritingPlanPrompt(wr sqlc.Writing, rows []sqlc.WritingOutline, msgs []sqlc.AtomMessage, studentText string) string {
	return renderWritingPlanPrompt(selectWritingPlanContext(wr, rows, msgs, studentText)).Text
}

func renderWritingPlanPrompt(c writingPlanContext) promptassembly.Document {
	wr, rows, studentText := c.Writing, c.Rows, c.StudentText
	var b promptassembly.Builder
	b.Mark("assignment", "mixed", "internal/api/writing_plan_prompt.go")
	b.WriteString(writingTopicLine(wr, "学生一开始说想写的是："))
	b.WriteString(writingLangLine(wr))
	b.WriteString(writingLengthLine(wr, "目标篇幅"))
	if wr.TargetWords != nil {
		b.WriteString("（篇幅只用来判断要几条分论点，用于规划内容数量，不要求学生填充无关内容。）\n")
	}
	b.Mark("readiness", "mixed", "internal/api/writing_plan_prompt.go")
	b.WriteString(c.Shape.promptBlock(c.Need))
	b.Mark("point-relevance", "mixed", "internal/api/writing_plan_prompt.go")
	b.WriteString(writingPointsCheckBlock(wr, rows))
	b.Mark("point-angles", "instruction", "internal/api/writing_plan_prompt.go")
	b.WriteString(writingPointAnglesBlock(wr, rows, c.Need.Points))
	b.Mark("stalled", "instruction", "internal/api/writing_plan_prompt.go")
	if c.Stalled {
		b.WriteString(writingPlanStalledBlock)
	}
	b.Mark("delegation-request", "instruction", "internal/api/writing_plan_prompt.go")
	if c.AskedToDoIt {
		b.WriteString(writingRefusalBlock)
	}

	b.Mark("outline", "mixed", "internal/api/writing_plan_prompt.go")
	b.WriteString("\n【当前的图】\n")
	if len(rows) == 0 {
		b.WriteString("（图是空的。先检查学生本轮是否已经表达观点或理由，已表达就直接整理；缺失才询问。）\n")
	} else {
		for _, r := range rows {
			indent := strings.Repeat("  ", int(r.Depth))
			line := indent + "- " + r.Text
			if lbl := writingKindLabel(writingKindOf(r), r.Source); lbl != "" {
				line += "（" + lbl + "）"
			}
			if src := strings.TrimSpace(r.Source); src != "" {
				line += "【出处：" + src + "】"
			}
			b.WriteString(line + "\n")
		}
	}
	b.Mark("methods", "mixed", "internal/api/writing_plan_prompt.go")
	b.WriteString("\n【可用的方法】（名称须取自此表）\n")
	for _, m := range c.Methods {
		b.WriteString("- " + m.Label() + "（" + m.AppliesTo + "）：" + m.Definition + "\n")
	}

	b.Mark("history", "context", "internal/api/writing_plan_prompt.go")
	b.WriteString("\n【你们刚才聊的】\n")
	tail := c.History
	any := false
	for _, m := range tail {
		var who string
		switch m.Role {
		case "student":
			who = "学生"
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

	b.Mark("latest-input", "context", "internal/api/writing_plan_prompt.go")
	b.WriteString("\n【学生刚刚说的】\n" + studentText + "\n")
	b.Mark("extraction-boundary", "instruction", "internal/api/writing_plan_prompt.go")
	b.WriteString("\n只能从「学生刚刚说的」这段话里提取节点。学生这段话里没有新的点子，add 就给空数组。向学生说明时用「观点」或直接说具体想法。已经说清的理由直接整理，只问仍缺少的内容。\n")
	return b.Document(c.Selection)
}
