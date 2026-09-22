package agent

// Prompt assembly for course_coach.go. Pure: no database or model calls.
// Static teaching text lives in internal/prompts; context selection is separate
// where a feature has a dedicated *_context.go file.

import (
	"fmt"
	"strings"
)

// BuildCourseAskPrompt returns the free-Q&A coach's system prompt: it names
// the course, the current step, and the course's stated goal, states the
// posture (student asks freely, coach helps him think it through — not
// answers for him), and then the four 铁律 guardrails verbatim: ① never hand
// over this step's quiz answer, only help him judge it himself; ② never
// conclude or write for him; ③ one question at a time; ④ keep it short.
// stepText (the step's authored content, when available) rides along as
// grounding context ONLY — the prompt explicitly tells the model not to
// recite it back, just to use it to answer in-context.
func BuildCourseAskPrompt(courseTitle, stepTitle, courseGoal, stepText string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "你是「印记」，正在陪一名学生上《%s》这门课的「%s」这一步。本课目标：%s。学生会自由提问。概念、词义和操作问题请直接解释，必要时使用与测验不同的例子。\n",
		courseTitle, stepTitle, courseGoal)
	b.WriteString("硬规则：① 不要直接给出本步测验题的正确答案，只引导他自己判断；② 不替他下结论、不替他写作；③ 最多问一个需要学生回答的问题，无需补充信息时可以不问；④ 回答简短，使用术语时附简明解释，不评价学生的能力或态度。\n")
	if strings.TrimSpace(stepText) != "" {
		b.WriteString("\n这一步的内容（仅供你理解语境，不要逐字复述给学生）：\n" + stepText + "\n")
	}
	return b.String()
}
