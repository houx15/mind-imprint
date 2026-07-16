package agent

import (
	"fmt"
	"strings"

	"mindimprint/api/internal/rubric"
)

// assessPosture is the growth-assessor's iron rule (RL-5): diagnostic evidence
// for growth, never a grade, never a rank, never a ghostwriting instruction.
const assessPosture = `你是「思维印记」的过程评估者。你的职责是依据可观察的行为证据，判断学生在与 AI 协作中「怎么思考」，而不是给分数、不排名、不下结论式判决。
每个维度给出 L1–L4（或证据不足时 NA）以及支撑该等级的具体行为证据（引用过程记录里的真实动作）。最后给一段面向成长的叙事，指出这次最大的跃迁与下一步。
严禁替学生改写或撰写作文内容；只描述与诊断其思考路径。只输出 JSON。`

// assessSystemPrompt builds the system turn: posture + the rubric ladders + the
// anchor few-shot samples. Pure — no I/O — so tests can assert on the string
// directly (mirrors reviewSystemPrompt).
func assessSystemPrompt(rb rubric.Rubric, anchors []AnchorSample) string {
	var b strings.Builder
	b.WriteString(assessPosture)
	b.WriteString("\n\n评分维度与等级阶梯：\n")
	for _, d := range rb.Dimensions {
		b.WriteString(fmt.Sprintf("%s %s（%s）：L1 %s ｜ L2 %s ｜ L3 %s ｜ L4 %s\n",
			d.ID, d.Name, d.Framework,
			d.Anchors["L1"], d.Anchors["L2"], d.Anchors["L3"], d.Anchors["L4"]))
	}
	if len(anchors) > 0 {
		b.WriteString("\n参考样例（few-shot）：\n")
		for _, a := range anchors {
			b.WriteString(fmt.Sprintf("【%s】过程：%s\n", a.Name, a.Digest))
			for _, d := range a.Dimensions {
				b.WriteString(fmt.Sprintf("  %s=%s（%s）\n", d.Code, d.Level, d.Evidence))
			}
			b.WriteString("  叙事：" + a.Narrative + "\n")
		}
	}
	b.WriteString(`
输出格式（严格 JSON，dimensions 覆盖上述每个维度）：
{"dimensions":[{"code":"D1","level":"L1|L2|L3|L4|NA","evidence":"…"}],"narrative":"…"}`)
	return b.String()
}

// assessUserInput serialises the process-record digest as the user turn.
func assessUserInput(in AssessmentInput) string {
	var b strings.Builder
	b.WriteString("过程记录：\n")
	if len(in.Timeline) > 0 {
		b.WriteString("时间线：\n" + strings.Join(in.Timeline, "\n") + "\n")
	}
	for _, c := range in.CardUses {
		b.WriteString(fmt.Sprintf("工具卡：%s（维度 %s，%s）\n", c.CardID, c.Dimension, c.Spont))
	}
	for _, d := range in.Dispositions {
		b.WriteString(fmt.Sprintf("对反馈的处置：%s —— %s\n", d.Kind, d.Reason))
	}
	if len(in.GateProgress) > 0 {
		b.WriteString("关卡进度：" + strings.Join(in.GateProgress, "；") + "\n")
	}
	if in.SnapshotCount > 0 {
		b.WriteString(fmt.Sprintf("草稿快照：%d 次，字数 %v\n", in.SnapshotCount, in.WordCounts))
	}
	if len(in.ReviewBands) > 0 {
		b.WriteString("整稿体检：" + strings.Join(in.ReviewBands, "、") + "\n")
	}
	if in.GraphSummary != "" {
		b.WriteString("论证结构：" + in.GraphSummary + "\n")
	}
	return b.String()
}
