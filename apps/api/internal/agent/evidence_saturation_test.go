package agent

import (
	"context"
	"testing"

	"mindimprint/api/internal/gateway"
)

func TestReviewEvidenceSaturation_Parses(t *testing.T) {
	prov := stubProviderText(`{"saturated":false,"why":"只有支持材料，缺一个反例","gaps":["缺一个反例或替代解释","再多几篇强证据"]}`)
	v, usage, err := ReviewEvidenceSaturation(context.Background(), prov, gateway.Resolved{Provider: "stub"}, EvidenceReviewInput{
		SubQuestion: "新能源投资的净效应",
		Papers:      []EvidencePaper{{Title: "A", Nature: "support", Argument: "投资降低碳排", Finding: "装机量↑"}},
	})
	if err != nil {
		t.Fatalf("ReviewEvidenceSaturation err = %v", err)
	}
	if v.Saturated || v.Why == "" || len(v.Gaps) != 2 {
		t.Fatalf("verdict = %+v", v)
	}
	if usage.OutputTokens == 0 {
		t.Error("usage should be reported for metering")
	}
}

func TestReviewEvidenceSaturation_Fenced(t *testing.T) {
	prov := stubProviderText("```json\n{\"saturated\":true,\"why\":\"支持与反例都齐了\",\"gaps\":[]}\n```")
	v, _, err := ReviewEvidenceSaturation(context.Background(), prov, gateway.Resolved{Provider: "stub"}, EvidenceReviewInput{SubQuestion: "x"})
	if err != nil {
		t.Fatalf("fenced err = %v", err)
	}
	if !v.Saturated || len(v.Gaps) != 0 {
		t.Fatalf("verdict = %+v", v)
	}
}

func TestReviewEvidenceSaturation_Garbage(t *testing.T) {
	prov := stubProviderText("我觉得还行。")
	if _, _, err := ReviewEvidenceSaturation(context.Background(), prov, gateway.Resolved{Provider: "stub"}, EvidenceReviewInput{SubQuestion: "x"}); err == nil {
		t.Fatal("expected error on unparseable reply (caller degrades)")
	}
}
