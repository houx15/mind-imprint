package agent

// Prompt assembly for project_coach.go. Pure: no database or model calls.
// Static teaching text lives in internal/prompts; context selection is separate
// where a feature has a dedicated *_context.go file.

import (
	"mindimprint/api/internal/prompts"

	"fmt"
	"strings"
)

// projectCoachPosturePrompt is the one agent's posture across every room. The
// active surface + spine state ride in the user message (BuildProjectCoachContext),
// so ONE posture adapts to each room instead of four scope-specific prompts.
// Restrained per the 四条铁律: guide don't answer, one question at a time, never
// write the assessed deliverable, never conclude for the student.
const projectCoachPosturePrompt = prompts.ProjectCoachPosturePrompt

// BuildProjectCoachContext assembles the one agent's user message: which room
// the student is in, the recent (non-folded) conversation across the WHOLE
// thread, and a compact spine projection. Kept pure for testing.
func BuildProjectCoachContext(history []ChatTurn, spineProjection, activeSurface string) string {
	// 🚨 块的顺序是一条成本契约，不只是可读性。通道按**公共前缀**给缓存打折
	// （命中部分按输入价的 20%），所以最稳的东西要排在最前面。
	//
	// 这里原来是「房间 → 对话 → 状态投影」。对话是一个**滑动窗口**：第 13 轮
	// 把第 1 轮挤掉，整个前缀就错位了，排在它后面的状态投影——**包括她那份
	// 几千字的正文**——每一轮都是全价。实测（qwen3.8-flash，4,800 token 的
	// 真实尺寸）：这个顺序命中 **0%**，换成下面这个顺序命中 **85%**。
	//
	// 现在是「状态投影（含正文）→ 对话 → 她在哪个房间 → 这一轮要做什么」。
	// 顺带和另一条既有教训对上了：要它照做的话放在**最后**最管用
	// （[[reading-room-rulings-2026-09-17]] 的「覆盖段放 prompt 最后」）。
	var b strings.Builder
	if p := strings.TrimSpace(spineProjection); p != "" {
		b.WriteString("项目当前状态（供你参考，别照搬复述）：\n" + p + "\n\n")
	}
	b.WriteString("对话（从旧到新）：\n")
	for _, t := range history {
		who := "学生"
		if t.Role == "assistant" {
			who = "你"
		}
		b.WriteString(fmt.Sprintf("- %s：%s\n", who, t.Content))
	}
	if s := strings.TrimSpace(activeSurface); s != "" {
		b.WriteString("\n学生现在在「" + s + "」。\n")
	}
	b.WriteString("\n现在，简明回应学生最新的发言；需要解释时可用两三句，最多一个问题，允许不提问。")
	return b.String()
}
