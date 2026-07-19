package agent

import (
	"fmt"
	"strings"

	"mindimprint/api/internal/rubric"
)

const reportPosture = `你是「思维印记」的过程评估者。依据可观察的行为证据，判断学生在与 AI 协作中「怎么思考」——不给分数以外的结论、不排名、不下判决式结论。
本模型是双轴模型：第一轴「认知深度」按 0–3 打分（四维小计满分 12）；第二轴「智识自主」只用观察语言描述、绝不打分；跨轴「元认知」同时描述深度面与自主面、不单独打分。
测量公理（必须原样体现，不得改写）：两轴永不合成总分；单次会话为事件级证据，不构成人级档位判定。
另需产出：SOLO 判层（对每一轮学生回应判 L1–L4，标注判据与发起方 自发/引导后；准确性是门槛不是刻度；序数不作均值；指令轮与核查行为不判层）；提示词透镜（对提示词而非学生分档 P0–P4，统计主动指令轮/边界设定/对手邀请）。
严禁替学生改写或撰写作文内容；只描述与诊断其思考路径。只输出 JSON。`

func assessReportSystemPrompt(m rubric.DualAxis) string {
	var b strings.Builder
	b.WriteString(reportPosture)

	b.WriteString("\n\n第一轴 · 认知深度（0–3 打分，覆盖以下每维）：\n")
	for _, d := range rubric.DepthDims() {
		b.WriteString(fmt.Sprintf("%s %s：0 %s ｜ 1 %s ｜ 2 %s ｜ 3 %s\n",
			d.ID, d.Name, d.Anchors["0"], d.Anchors["1"], d.Anchors["2"], d.Anchors["3"]))
	}

	auto := rubric.AutonomyDim()
	b.WriteString(fmt.Sprintf("\n第二轴 · 智识自主（不打分，仅观察）：\n%s %s：%s\n", auto.ID, auto.Name, auto.ObservationGuide))

	cross := rubric.CrossDim()
	b.WriteString(fmt.Sprintf("\n跨轴 · 元认知（不单独打分）：\n%s %s：%s\n", cross.ID, cross.Name, cross.Guide))

	b.WriteString("\nSOLO 层级：")
	for _, s := range m.SoloLevels {
		b.WriteString(fmt.Sprintf("%s %s；", s.Level, s.Name))
	}
	b.WriteString("\n提示词档位：")
	for _, t := range m.PromptTiers {
		b.WriteString(fmt.Sprintf("%s %s；", t.Tier, t.Label))
	}

	b.WriteString(`

输出格式（严格 JSON；depthAxis.dims 覆盖 D1/D3/D4/D5，autonomyAxis 绝不含 score 字段）：
{"depthAxis":{"dims":[{"code":"D1","score":0,"evidence":"…","promptEvidence":"…"}]},
"autonomyAxis":{"observation":"…","anchoredSignals":["…"],"promptedSignals":["…"],"adversaryInvites":0,"promptEvidence":"…"},
"crossAxis":{"depthLevel":"L1|L2|L3|L4|NA","initiative":"自发|引导后|混合","prose":"…","promptEvidence":"…"},
"solo":[{"round":1,"excerpt":"…","level":"L3","rationale":"…","initiative":"自发|引导后"}],
"promptLens":{"directiveRounds":0,"totalRounds":0,"boundarySettings":0,"adversaryInvites":0,
"questions":[{"title":"…","body":"…"}],"bestPrompt":{"round":0,"quote":"…","annotation":"…"},
"takeaway":{"round":0,"quote":"…","annotation":"…"},"perRound":[{"round":1,"tier":"P0|P1|P2|P3","label":"…"}]},
"timeline":[{"round":1,"task":"…","prompt":"…","pTag":"P0|P1|P2|P3","dimTags":["D1=2"]}],
"keyEvidence":[{"label":"…","quote":"…"}],
"guidance":{"anchored":"…","prompted":"…","risk":"…","nextSteps":[{"title":"…","body":"…"}]},
"narrative":"…"}`)
	return b.String()
}

func assessReportUserInput(in AssessmentInput) string {
	var b strings.Builder
	b.WriteString("过程记录：\n")
	if len(in.Rounds) > 0 {
		b.WriteString("逐轮学生提示词与语境：\n")
		for _, r := range in.Rounds {
			b.WriteString(fmt.Sprintf("R%d 语境：%s ｜ 学生：%s\n", r.N, r.AiContext, r.StudentPrompt))
		}
	}
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
