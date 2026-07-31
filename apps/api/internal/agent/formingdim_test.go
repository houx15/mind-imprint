package agent

import (
	"context"
	"testing"

	"mindimprint/api/internal/gateway"
)

func formingDimProvider(reply string) *gateway.StubProvider {
	return gateway.NewStubProvider([]gateway.StreamEvent{
		{Kind: gateway.EventTextDelta, TextDelta: reply},
		{Kind: gateway.EventUsage, Usage: &gateway.ChatUsage{InputTokens: 20, OutputTokens: 10}},
		{Kind: gateway.EventDone, StopReason: gateway.StopStop},
	})
}

func TestProposeFormingDim_ReturnsSuggestionForUncoveredDim(t *testing.T) {
	prov := formingDimProvider(`{"dim":"objective","value":"想弄清中国的绿化是否真的等于更可持续"}`)
	sug, _, err := ProposeFormingDim(context.Background(), prov, gateway.Resolved{}, "我最想弄清楚的是中国变绿到底算不算真的可持续", []string{"objective", "reason"})
	if err != nil {
		t.Fatal(err)
	}
	if sug == nil || sug.Dim != "objective" || sug.Value == "" {
		t.Fatalf("want an objective suggestion, got %+v", sug)
	}
}

func TestProposeFormingDim_NilWhenModelSaysNone(t *testing.T) {
	prov := formingDimProvider(`{"dim":""}`)
	sug, _, err := ProposeFormingDim(context.Background(), prov, gateway.Resolved{}, "嗯……我还没想好要怎么说这个题目", []string{"objective", "reason"})
	if err != nil {
		t.Fatal(err)
	}
	if sug != nil {
		t.Fatalf("want nil (model articulated nothing), got %+v", sug)
	}
}

func TestProposeFormingDim_RejectsDimNotInUncovered(t *testing.T) {
	// The model suggests a dimension that is ALREADY filled (not in uncovered) —
	// must be rejected so a confirm chip never overwrites a covered dim.
	prov := formingDimProvider(`{"dim":"resources","value":"用 Zotero 管理文献"}`)
	sug, _, err := ProposeFormingDim(context.Background(), prov, gateway.Resolved{}, "我打算用 Zotero 来管理我找到的所有文献资料", []string{"objective", "reason"})
	if err != nil {
		t.Fatal(err)
	}
	if sug != nil {
		t.Fatalf("want nil (resources is already covered), got %+v", sug)
	}
}

func TestProposeFormingDim_NoSpendOnShortText(t *testing.T) {
	prov := formingDimProvider(`{"dim":"objective","value":"x"}`)
	sug, usage, err := ProposeFormingDim(context.Background(), prov, gateway.Resolved{}, "好的", []string{"objective"})
	if err != nil {
		t.Fatal(err)
	}
	if sug != nil {
		t.Fatalf("want nil for too-short text, got %+v", sug)
	}
	if usage.InputTokens != 0 || usage.OutputTokens != 0 {
		t.Fatalf("must not spend on too-short text, got usage %+v", usage)
	}
}

func TestProposeFormingDim_NoSpendWhenNothingUncovered(t *testing.T) {
	prov := formingDimProvider(`{"dim":"objective","value":"x"}`)
	sug, usage, err := ProposeFormingDim(context.Background(), prov, gateway.Resolved{}, "我想弄清楚这个题目到底在问什么问题", nil)
	if err != nil {
		t.Fatal(err)
	}
	if sug != nil || usage.InputTokens != 0 || usage.OutputTokens != 0 {
		t.Fatalf("no uncovered dims → no suggestion, no spend; got sug=%+v usage=%+v", sug, usage)
	}
}
