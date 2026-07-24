package agent

import (
	"fmt"
	"strings"

	"mindimprint/api/internal/rubric"
)

const reportPosture = `你是「思维印记」的过程评估者。依据可观察的行为证据，判断学生在与 AI 协作中「怎么思考」——不给分数以外的结论、不排名、不下判决式结论。
本模型是双轴模型：第一轴「认知深度」（D1–D6）按 L1–L4 判层，判据是可观察的思考行为，不是分数；第二轴「智识自主」（A1–A6）按行为计数带判 0–5，是行为计数带，不是质量打分；提示词透镜对提示词本身（而非学生）判 0–5，是过程证据，不是第三根评分轴，绝不并入任何总分。
测量公理（必须原样体现，不得改写）：两轴永不合成总分；单次会话为事件级证据，不构成人级档位判定。
严禁替学生改写或撰写作文内容；只描述与诊断其思考路径。只输出 JSON。
铁律·禁止杜撰学生提示词：交互证据 interactionEvidence 一律只能依据「过程记录」里逐条给出的「逐轮学生提示词与语境」原文作证；某一轮的学生提示词为空，或整份记录压根没有提供逐轮学生提示词时，对应轮次必须省略——绝不允许编造或转述一个听起来合理的学生发言当作真实证据。`

// assessReportSystemPrompt builds the flagship system prompt from the live
// rubric config: per-dimension L1-L4 anchors (depth), the autonomy count-band
// guide plus per-signal countable events, the six prompt lenses, and — only
// when projectProjection is true — the official-standard alignment section
// (currently the sole standard, "ap-research"). The output-format JSON
// template mirrors reportWire and includes the officialProjection/
// workAndProcess keys only in the project-projection case.
func assessReportSystemPrompt(m rubric.DualAxis, projectProjection bool) string {
	var b strings.Builder
	b.WriteString(reportPosture)

	b.WriteString("\n\n第一轴 · 认知深度（每维判 L1–L4，暂无可计入证据时判 NA，不是低分）：\n")
	for _, d := range rubric.DepthDims() {
		b.WriteString(fmt.Sprintf("%s %s：L1 %s ｜ L2 %s ｜ L3 %s ｜ L4 %s\n",
			d.ID, d.Name, d.Anchors["L1"], d.Anchors["L2"], d.Anchors["L3"], d.Anchors["L4"]))
	}

	b.WriteString("\n第二轴 · 智识自主（行为计数带，0–5，不是质量打分）：\n")
	b.WriteString(rubric.AutonomyBand() + "\n")
	for _, a := range rubric.AutonomySignals() {
		b.WriteString(fmt.Sprintf("%s %s：可计入事件=%s\n", a.ID, a.Name, a.Event))
	}
	b.WriteString("机会供给规则：" + m.OpportunityRule + "\n")

	b.WriteString("\n提示词透镜（对提示词本身判 0–5，是过程证据，不并入任何总分）：\n")
	for _, l := range rubric.Lenses() {
		b.WriteString(fmt.Sprintf("%s %s：%s\n", l.ID, l.Name, l.Guide))
	}

	if projectProjection {
		if std, ok := rubric.Standard("ap-research"); ok {
			b.WriteString(fmt.Sprintf("\n官方投影（%s，%s；训练折算仅作作品就绪度参考，不与 D/A 双轴合成）：\n", std.ID, std.Name))
			for _, c := range std.Components {
				b.WriteString(fmt.Sprintf("%s（%s）：%s\n", c.Name, c.Scale, c.Kou))
			}
			b.WriteString("对齐要点：\n")
			for _, item := range std.AlignmentItems {
				b.WriteString("- " + item + "\n")
			}
			b.WriteString("同时给出作品与过程要点：workSamples 引作品本身片段，processMaterials 诊断过程材料（如 SIFT 记录）的完成情况。\n")
		}
	}

	b.WriteString(outputFormatTemplate(projectProjection))
	return b.String()
}

// outputFormatTemplate is the strict-JSON output-format spec appended to the
// system prompt. It mirrors reportWire: depthAxis/autonomyAxis/lenses are
// arrays keyed by code (the engine re-indexes and fills names); the
// officialProjection/workAndProcess keys appear ONLY in the project-
// projection case.
func outputFormatTemplate(projectProjection bool) string {
	base := `
输出格式（严格 JSON；depthAxis 覆盖 D1–D6，autonomyAxis 覆盖 A1–A6，promptLens.lenses 覆盖六个透镜 id，均按 code 索引；某维度确无证据可省略该项，引擎按 rubric 补全为 NA/0）：
{"depthAxis":[{"code":"D1","level":"L1|L2|L3|L4|NA","levelRange":"","evidence":"…","promptEvidence":"…"}],
"autonomyAxis":[{"code":"A1","level":0,"opportunity":"given_taken|given_not_taken|not_supplied","evidence":"…","promptEvidence":"…"}],
"promptLens":{"stats":[{"label":"…","value":"…"}],"lenses":[{"code":"L_decisions","level":0,"evidence":"…"}]},
"interactionEvidence":[{"round":1,"student":"…","aiSummary":"…","signal":"D1→L3"}],
"narrative":"…",
"guidance":{"nextSteps":[{"title":"…","task":"…"}]}`
	if !projectProjection {
		return base + "}"
	}
	return base + `,
"officialProjection":{"standard":{"id":"ap-research","name":"AP Research"},
"components":[{"name":"Academic Paper","judgement":"…","reason":"…"}],
"alignment":[{"item":"…","standard":"…","performance":"…","impact":"…"}],
"readiness":{"score":0,"note":"…（须注明不与 D/A 双轴合成）"}},
"workAndProcess":{"workSamples":[{"title":"…","text":"…"}],"processMaterials":[{"name":"…","status":"…","diagnosis":"…"}]}}`
}

// assessReportUserInput renders the compact process digest as the user turn:
// per-round student prompts + AI context, the append-only event timeline,
// card uses, dispositions, gate progress, snapshot/word-count facts, review
// bands, the argument-graph summary, and — only when supplied — a work-sample
// excerpt section (project surface only; chat/course pass none).
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
	if len(in.WorkSamples) > 0 {
		b.WriteString("作品片段：\n")
		for i, s := range in.WorkSamples {
			b.WriteString(fmt.Sprintf("%d. %s\n", i+1, s))
		}
	}
	return b.String()
}
