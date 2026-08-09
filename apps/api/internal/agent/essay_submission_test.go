package agent

import (
	"strings"
	"testing"
)

func TestDeriveSubmissionSteps(t *testing.T) {
	steps := DeriveSubmissionSteps()
	want := []string{"sub:intro", "sub:conclusion", "sub:compose", "sub:polish"}
	if len(steps) != len(want) {
		t.Fatalf("got %d steps, want %d", len(steps), len(want))
	}
	for i, k := range want {
		if steps[i].Key != k {
			t.Fatalf("step %d key = %q, want %q", i, steps[i].Key, k)
		}
	}
}

func TestEssayGuideBody_Submission(t *testing.T) {
	intro := guideGenUserContent(GuideGenInput{Doc: "essay", Step: Step{Key: "sub:intro", Title: "引言", Kind: KindFixed}})
	if !strings.Contains(intro, "引言") || !strings.Contains(intro, "论证结构") {
		t.Fatalf("intro body wrong: %s", intro)
	}
	concl := guideGenUserContent(GuideGenInput{Doc: "essay", Step: Step{Key: "sub:conclusion", Title: "结论", Kind: KindFixed}})
	if !strings.Contains(concl, "结论") {
		t.Fatalf("conclusion body wrong: %s", concl)
	}
}
