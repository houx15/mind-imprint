package agent

import (
	"strings"
	"testing"
)

func sampleParentReport() Report {
	return Report{
		DepthAxis: []DepthDim{
			{Code: "D1", Name: "任务理解与问题表述", Level: "L4", Evidence: "把宽泛影响收窄为可研究关系"},
			{Code: "D2", Name: "证据与信源", Level: "L3", Evidence: "比较多篇文献推出 gap"},
			{Code: "D3", Name: "论证结构", Level: "L3", Evidence: "gap-method-evidence 链条稳定"},
			{Code: "D4", Name: "视角与偏见", Level: "L3", Evidence: "识别自陈与便利样本限制"},
			{Code: "D5", Name: "反馈处理与修订", Level: "L3", Evidence: "按反馈重排图表收窄结论"},
			{Code: "D6", Name: "反思与元认知", Level: "NA", Evidence: ""},
		},
		AutonomyAxis: []AutonomySignal{
			{Code: "A1", Name: "方向自主", Level: 4, Opportunity: "given_taken", Evidence: "主动改题"},
			{Code: "A2", Name: "发起自主", Level: 4, Opportunity: "given_taken", Evidence: "自发补证据"},
			{Code: "A3", Name: "边界主权", Level: 4, Opportunity: "given_taken", Evidence: "拒绝代写"},
			{Code: "A4", Name: "对抗与检验", Level: 3, Opportunity: "given_not_taken", Evidence: "部分引导后强化"},
			{Code: "A5", Name: "判断署名", Level: 5, Opportunity: "given_taken", Evidence: "自评 Paper 4"},
			{Code: "A6", Name: "求真优先", Level: 4, Opportunity: "given_taken", Evidence: "因证据收窄结论"},
		},
	}
}

func TestParentFactsPromptCarriesEvidenceNotLensOrOfficial(t *testing.T) {
	rep := sampleParentReport()
	rep.PromptLens.Note = "透镜秘密"
	p := ParentFactsPrompt(rep, "林知远", "嵌入式广告与正常化")
	if !strings.Contains(p, "主动改题") || !strings.Contains(p, "任务理解与问题表述") {
		t.Fatal("facts prompt must carry the canonical evidence + dim names")
	}
	if strings.Contains(p, "透镜秘密") {
		t.Fatal("facts prompt must NOT carry prompt-lens / teacher-only material")
	}
	// D6 is NA → not offered to the composer for a reading.
	if strings.Contains(p, "D6") && strings.Contains(p, "反思与元认知：") {
		t.Fatal("NA dim must not be sent for a reading")
	}
}

func validParentProse() ParentProse {
	return ParentProse{
		Glance: "方法对齐、证据充分，自主性强", DOverview: "多数维度稳定在熟练", AOverview: "六个方面都观察到主动信号",
		Opportunity: "这次机会多由孩子自己创造，边界守得好，真实性高。", WarmLine: "把 AI 当审稿人而不是代笔。",
		DReadings: map[string]string{"D1": "能把宽泛话题收窄成可研究的问题。", "D2": "会比较资料、说清每份能支持什么。", "D3": "用证据一步步推出结论。", "D4": "会承认研究的局限与偏差。", "D5": "收到质疑真的改进论证。"},
		AReadings: map[string]string{"A1": "自己决定方向。", "A2": "主动查证补证据。", "A3": "用 AI 时设了边界。", "A4": "主动请人挑刺。", "A5": "为自己的结论负责。", "A6": "证据不足时愿意把结论改小。"},
		Advice:    []ParentAdvice{{Title: "请他讲给你听", Text: "用一句话说清这份研究不能说明什么。"}},
	}
}

func TestValidateParentProse(t *testing.T) {
	rep := sampleParentReport()

	if err := validateParentProse(validParentProse(), rep); err != nil {
		t.Fatalf("valid prose rejected: %v", err)
	}

	// Missing a wanted (real-level) D reading.
	p := validParentProse()
	delete(p.DReadings, "D3")
	if err := validateParentProse(p, rep); err == nil {
		t.Error("expected rejection for missing D3 reading")
	}

	// A reading for NA dim D6 is not wanted → unknown key rejected.
	p = validParentProse()
	p.DReadings["D6"] = "不该有的反思读数"
	if err := validateParentProse(p, rep); err == nil {
		t.Error("expected rejection for NA-dim reading D6")
	}

	// Missing an A reading (all six wanted).
	p = validParentProse()
	delete(p.AReadings, "A4")
	if err := validateParentProse(p, rep); err == nil {
		t.Error("expected rejection for missing A4 reading")
	}

	// Empty wording.
	p = validParentProse()
	p.Glance = "  "
	if err := validateParentProse(p, rep); err == nil {
		t.Error("expected rejection for empty glance")
	}

	// Bare internal code leaks (说人话).
	for _, bad := range []string{"D3 很稳", "d 4 不足", "given_not_taken 出现", "SOLO 分布", "P2 水平"} {
		p = validParentProse()
		p.Opportunity = bad
		if err := validateParentProse(p, rep); err == nil {
			t.Errorf("expected rejection for leaked code %q", bad)
		}
	}

	// Advice title over its rune cap is rejected (M1: title now capped too).
	p = validParentProse()
	p.Advice = []ParentAdvice{{Title: strings.Repeat("字", parentAdviceMax+1), Text: p.Advice[0].Text}}
	if err := validateParentProse(p, rep); err == nil {
		t.Error("expected rejection for over-cap advice title")
	}

	// Glance over its rune cap is rejected.
	p = validParentProse()
	p.Glance = strings.Repeat("字", parentGlanceMax+1)
	if err := validateParentProse(p, rep); err == nil {
		t.Error("expected rejection for over-cap glance")
	}
}
