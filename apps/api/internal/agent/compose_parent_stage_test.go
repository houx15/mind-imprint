package agent

import (
	"strings"
	"testing"
)

func validStageProse() ParentStageProse {
	return ParentStageProse{
		WarmLine:       "这一阶段，孩子在自己拿主意上有明显的进步。",
		StageGrowth:    "这段时间他更愿意先自己想清楚，再去请 AI 帮忙检查，而不是一上来就要答案。",
		StageHighlight: "本周他主动请 AI 扮演反方，来挑自己论证里的问题，这是很成熟的学习方式。",
		StageForward:   "可以给他更高一点的目标，鼓励他把研究的意义讲得更具体。",
		Advice: []ParentAdvice{
			{Title: "请他讲给你听", Text: "让他用一句话说清这份研究不能说明什么。"},
			{Title: "保护他的自主", Text: "鼓励他先自己判断，再去问 AI。"},
			{Title: "给一点挑战", Text: "问他如果要再进一步，还差哪一步。"},
		},
	}
}

func TestValidateParentStageProse_OK(t *testing.T) {
	if err := validateParentStageProse(validStageProse()); err != nil {
		t.Fatalf("valid prose rejected: %v", err)
	}
}

func TestValidateParentStageProse_EmptyHighlightAllowed(t *testing.T) {
	p := validStageProse()
	p.StageHighlight = "" // 敢于空白: a thin window has no highlight
	if err := validateParentStageProse(p); err != nil {
		t.Fatalf("empty highlight must be allowed: %v", err)
	}
}

func TestValidateParentStageProse_RejectsEmptyGrowth(t *testing.T) {
	p := validStageProse()
	p.StageGrowth = "   "
	if err := validateParentStageProse(p); err == nil {
		t.Fatal("empty stageGrowth must be rejected")
	}
}

func TestValidateParentStageProse_RejectsLevelCodeLeak(t *testing.T) {
	p := validStageProse()
	p.StageGrowth = "他从 L2 迈向 L3，进步明显。" // bare level codes must never reach parents
	if err := validateParentStageProse(p); err == nil {
		t.Fatal("L-code leak must be rejected")
	}
}

func TestValidateParentStageProse_RejectsAxisCodeLeak(t *testing.T) {
	p := validStageProse()
	p.Advice[0].Text = "在 A4 上多鼓励他。"
	if err := validateParentStageProse(p); err == nil {
		t.Fatal("A-code leak must be rejected")
	}
}

func TestValidateParentStageProse_RejectsWrongAdviceCount(t *testing.T) {
	p := validStageProse()
	p.Advice = p.Advice[:2] // must be exactly 3
	if err := validateParentStageProse(p); err == nil {
		t.Fatal("advice count != 3 must be rejected")
	}
}

func TestValidateParentStageProse_RejectsOverCap(t *testing.T) {
	p := validStageProse()
	p.WarmLine = strings.Repeat("字", parentWarmMax+1)
	if err := validateParentStageProse(p); err == nil {
		t.Fatal("over-cap warmLine must be rejected")
	}
}
