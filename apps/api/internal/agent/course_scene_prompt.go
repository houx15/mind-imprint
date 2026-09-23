package agent

// Prompt assembly for course_scene.go. Pure: no database or model calls.
// Static teaching text lives in internal/prompts; context selection is separate
// where a feature has a dedicated *_context.go file.

import (
	"fmt"
	"strings"
)

// sceneSystemPrompt returns the posture + rules for the given slot. The rules
// enforce the design doc §6.1/§6.2 restraints; the facts themselves ride in
// the user message (mirroring reportgen.go's system=rules / user=facts split).
func sceneSystemPrompt(which string) string {
	var b strings.Builder
	b.WriteString("你是「印记」，一门课程的旁白。请用简体中文、克制而温和地写一段简短的口播旁白，直接对学生说话。只用下面用户消息里给出的事实，不要编造任何没有给出的信息。只输出旁白正文本身，不要标题、不要小标题、不要列表符号、不要引号包裹。\n")
	if which == "closing" {
		b.WriteString("这是课程的结束语，要求：① 与给定的『课程小结』保持一致；② 只提到真实存在的证据（用户消息里给出的才算存在），没有的证据绝不杜撰；③ 区分『完成了这门课』与『掌握了这项能力』，不要把完成说成精通；④ 不做任何没有依据的进步断言（例如不要说『你比之前进步了』，除非用户消息里明确给了这样的证据）；⑤ 点出这门课的收获（takeaways）与可迁移的应用（transfer）。\n")
	} else {
		b.WriteString("这是课程的开场白，要求：① 先简短问候；② 说明这节课学生将要做什么；③ 输入提供预计用时时再说明用时，不自行估算；④ 只有在用户消息给出了『可用的学习历史信号』时，才可温和地把这节课和学生此前的学习联系起来（信号为空则完全不要提及过去，也不要暗示）；⑤ 以一句邀请学生开始的话收尾。\n")
	}
	return b.String()
}

// sceneUserPrompt lays out the approved facts for the slot. Only signals in
// `used` (non-empty evidence) are surfaced, so an unpermitted or empty signal
// can never leak into the narration.
func sceneUserPrompt(in SceneGenInput, used []string) string {
	var b strings.Builder
	if in.Which == "closing" {
		fmt.Fprintf(&b, "课程：%s\n", in.CourseTitle)
		if strings.TrimSpace(in.PreparedSummary) != "" {
			fmt.Fprintf(&b, "课程小结（结束语必须与之一致）：%s\n", in.PreparedSummary)
		}
		if len(in.Takeaways) > 0 {
			fmt.Fprintf(&b, "收获（takeaways）：%s\n", strings.Join(in.Takeaways, "；"))
		}
		if len(in.TransferApplications) > 0 {
			fmt.Fprintf(&b, "可迁移的应用（transfer）：%s\n", strings.Join(in.TransferApplications, "；"))
		}
	} else {
		fmt.Fprintf(&b, "课程：%s\n", in.CourseTitle)
		if in.EstimatedMinutes > 0 {
			fmt.Fprintf(&b, "预计用时：约 %d 分钟\n", in.EstimatedMinutes)
		}
		if len(in.Objectives) > 0 {
			fmt.Fprintf(&b, "本课目标：%s\n", strings.Join(in.Objectives, "；"))
		}
		if len(in.LearningPreview) > 0 {
			fmt.Fprintf(&b, "本课将要做的事：%s\n", strings.Join(in.LearningPreview, "；"))
		}
	}
	if len(used) > 0 {
		b.WriteString("可用的学习历史信号（只有下面列出的才可使用）：\n")
		for _, s := range used {
			fmt.Fprintf(&b, "- %s：%s\n", s, in.SignalEvidence[s])
		}
	}
	return b.String()
}
