package agent

import (
	"context"
	"testing"

	"mindimprint/api/internal/gateway"
)

func TestReviewProposalPart_ParsesVerdict(t *testing.T) {
	prov := stubProviderText(`{"ready":true,"why":"这部分把关键词定义清楚了。","suggestions":["可以再点明一个争议点"]}`)
	v, usage, err := ReviewProposalPart(context.Background(), prov, gateway.Resolved{Provider: "stub"}, ProposalPartReviewInput{
		Title: "中国是否让地球更可持续", StepTitle: "对题目的理解",
		StepPrompt: "解释你对题目的理解", StudentText: "我认为「可持续」在这里指……",
	})
	if err != nil {
		t.Fatalf("ReviewProposalPart err = %v", err)
	}
	if !v.Ready || v.Why == "" || len(v.Suggestions) != 1 {
		t.Fatalf("verdict = %+v", v)
	}
	if usage.OutputTokens == 0 {
		t.Error("usage should be reported for metering")
	}
}

func TestReviewProposalPart_GarbageErrors(t *testing.T) {
	prov := stubProviderText("我觉得写得不错。")
	if _, _, err := ReviewProposalPart(context.Background(), prov, gateway.Resolved{Provider: "stub"}, ProposalPartReviewInput{StudentText: "x"}); err == nil {
		t.Fatal("expected error on unparseable reply (caller degrades)")
	}
}
