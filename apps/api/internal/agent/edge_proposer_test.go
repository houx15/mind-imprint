package agent

import (
	"context"
	"testing"

	"mindimprint/api/internal/gateway"
)

// edge_proposer_test.go — Task B3: 印记 proposes labeled edges between the
// student's own top-level question nodes. Mirrors exploration_guide_test.go/
// dig_query_test.go's stubbed-provider pattern. The critical robustness
// property under test: the model speaks in 1-based INDICES (never UUIDs),
// and any proposal that doesn't survive validation (bad label, out-of-range
// index, self-edge) must be silently dropped rather than returned.

func TestProposeQuestionEdges_FiltersBadLabelAndOutOfRange(t *testing.T) {
	script := []gateway.StreamEvent{
		{Kind: gateway.EventTextDelta, TextDelta: `{"proposals":[` +
			`{"from":1,"to":2,"label":"反驳/张力","why":"中国碳排放总量全球第一，是反例"},` +
			`{"from":1,"to":2,"label":"无关","why":"不在闭集里"},` +
			`{"from":1,"to":5,"label":"支持","why":"越界索引"}` +
			`]}`},
		{Kind: gateway.EventDone},
	}
	prov := gateway.NewStubProvider(script)
	in := EdgeProposerInput{
		Questions: []EdgeProposerQuestion{
			{Text: "中国是否让地球更可持续？"},
			{Text: "中国碳排放总量全球第一，是否推翻论点？"},
		},
	}
	got, _, err := ProposeQuestionEdges(context.Background(), prov, gateway.Resolved{Provider: "fake", Model: "m"}, in)
	if err != nil {
		t.Fatalf("propose: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("proposals = %+v, want exactly 1 (bad label + out-of-range index dropped)", got)
	}
	if got[0].FromIndex != 1 || got[0].ToIndex != 2 || got[0].Label != "反驳/张力" {
		t.Fatalf("unexpected surviving proposal: %+v", got[0])
	}
}

func TestProposeQuestionEdges_DropsSelfEdge(t *testing.T) {
	script := []gateway.StreamEvent{
		{Kind: gateway.EventTextDelta, TextDelta: `{"proposals":[{"from":1,"to":1,"label":"细化","why":"自环"}]}`},
		{Kind: gateway.EventDone},
	}
	prov := gateway.NewStubProvider(script)
	in := EdgeProposerInput{
		Questions: []EdgeProposerQuestion{
			{Text: "问题一"},
			{Text: "问题二"},
		},
	}
	got, _, err := ProposeQuestionEdges(context.Background(), prov, gateway.Resolved{Provider: "fake", Model: "m"}, in)
	if err != nil {
		t.Fatalf("propose: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("proposals = %+v, want empty (from==to self-edge must be dropped)", got)
	}
}

func TestProposeQuestionEdges_StripsFences(t *testing.T) {
	script := []gateway.StreamEvent{
		{Kind: gateway.EventTextDelta, TextDelta: "```json\n" +
			`{"proposals":[{"from":2,"to":1,"label":"子问题","why":"B 是 A 的子问题"}]}` +
			"\n```"},
		{Kind: gateway.EventDone},
	}
	prov := gateway.NewStubProvider(script)
	in := EdgeProposerInput{
		Questions: []EdgeProposerQuestion{
			{Text: "A"},
			{Text: "B"},
		},
	}
	got, _, err := ProposeQuestionEdges(context.Background(), prov, gateway.Resolved{Provider: "fake", Model: "m"}, in)
	if err != nil {
		t.Fatalf("propose: %v", err)
	}
	if len(got) != 1 || got[0].FromIndex != 2 || got[0].ToIndex != 1 || got[0].Label != "子问题" {
		t.Fatalf("unexpected proposals after fence stripping: %+v", got)
	}
}
