package agent

import (
	"context"
	"strings"
	"testing"

	"mindimprint/api/internal/gateway"
)

func TestGenerateProposalGuideStep_Parses(t *testing.T) {
	prov := stubProviderText(`{"prompt":"用你自己的话说说你对题目的理解？","example":"For a prompt about globalization, a student wrote ...","refHint":"framework 目标"}`)
	out, usage, err := GenerateProposalGuideStep(context.Background(), prov, gateway.Resolved{Provider: "stub"}, GuideGenInput{
		Title: "中国是否让地球更可持续", Step: Step{Key: "understanding", Title: "对题目的理解", Kind: KindFixed},
	})
	if err != nil {
		t.Fatalf("GenerateProposalGuideStep err = %v", err)
	}
	if out.Prompt == "" || out.Example == "" || out.RefHint == "" {
		t.Fatalf("card = %+v, want prompt+example+refHint", out)
	}
	if usage.OutputTokens == 0 {
		t.Error("usage should be reported for metering")
	}
}

func TestGenerateProposalGuideStep_ParsesFenced(t *testing.T) {
	prov := stubProviderText("```json\n{\"prompt\":\"q\",\"example\":\"e\"}\n```")
	out, _, err := GenerateProposalGuideStep(context.Background(), prov, gateway.Resolved{Provider: "stub"}, GuideGenInput{Step: Step{Key: "thesis", Kind: KindFixed}})
	if err != nil {
		t.Fatalf("fenced err = %v", err)
	}
	if out.Prompt != "q" {
		t.Fatalf("card = %+v", out)
	}
}

func TestGenerateProposalGuideStep_EmptyPromptErrors(t *testing.T) {
	prov := stubProviderText(`{"prompt":"","example":"e"}`)
	if _, _, err := GenerateProposalGuideStep(context.Background(), prov, gateway.Resolved{Provider: "stub"}, GuideGenInput{Step: Step{Key: "thesis", Kind: KindFixed}}); err == nil {
		t.Fatal("expected error on empty prompt (caller degrades to null card)")
	}
}

func TestGuideGenUserContent_VariesByStepKind(t *testing.T) {
	base := GuideGenInput{
		Title:     "中国是否让地球更可持续",
		Objective: "研究中国可再生能源投资对全球碳排放的影响",
	}

	// fixed step → framework dims + the step's own skeleton.
	fixed := base
	fixed.Step = Step{Key: "understanding", Title: "对题目的理解", Kind: KindFixed}
	fc := guideGenUserContent(fixed)
	if !strings.Contains(fc, "对题目的理解") {
		t.Fatalf("fixed content missing step title: %s", fc)
	}

	// subq-define → decomposition guidance (mentions 子问题).
	def := base
	def.Step = Step{Key: "research-plan", Title: "研究计划 · 定子问题", Kind: KindSubqDefine}
	dc := guideGenUserContent(def)
	if !strings.Contains(dc, "子问题") {
		t.Fatalf("subq-define content should teach decomposition: %s", dc)
	}

	// subq → THIS sub-question + siblings.
	sq := base
	sq.Step = Step{Key: "subq:b", Title: "子问题 2", Kind: KindSubq, SubQuestionID: "b"}
	sq.ThisSubQuestion = "太阳能装机量的增长是否真的降低了单位 GDP 碳排放"
	sq.SiblingSubQuestions = []SubQuestion{{ID: "a", Text: "煤电占比的变化"}, {ID: "b", Text: sq.ThisSubQuestion}}
	sc := guideGenUserContent(sq)
	if !strings.Contains(sc, sq.ThisSubQuestion) {
		t.Fatalf("subq content missing THIS sub-question: %s", sc)
	}
	if !strings.Contains(sc, "煤电占比的变化") {
		t.Fatalf("subq content missing sibling sub-question: %s", sc)
	}
}

func TestGuideGen_EssayClaim(t *testing.T) {
	sq := GuideGenInput{
		Title: "T", Objective: "主问题", Doc: "essay",
		Step: Step{Key: "claim:b", Title: "论点 2", Kind: KindSubq, SubQuestionID: "b"},
		ThisSubQuestion: "太阳能装机增长的净效应",
		SiblingSubQuestions: []SubQuestion{{ID: "a", Text: "煤电占比"}, {ID: "b", Text: "太阳能装机增长的净效应"}},
	}
	sc := guideGenUserContent(sq)
	if !strings.Contains(sc, "论点") || !strings.Contains(sc, "太阳能装机增长的净效应") || !strings.Contains(sc, "煤电占比") {
		t.Fatalf("essay claim body wrong: %s", sc)
	}
	// challenges is a non-skippable essay step.
	ch := guideGenUserContent(GuideGenInput{Doc: "essay", Step: Step{Key: "challenges", Title: "面对反方观点", Kind: KindFixed}})
	if !strings.Contains(ch, "反方观点") {
		t.Fatalf("challenges body wrong: %s", ch)
	}
}
