package agent

import (
	"context"
	"testing"

	"mindimprint/api/internal/cards"
	"mindimprint/api/internal/gateway"
)

func annotationSpec() cards.Spec {
	return cards.Spec{
		ID: "sift_craap", Name: "CRAAP", Mode: "annotation",
		Steps: []cards.Step{{Key: "authority", Title: "权威性 · Authority"}, {Key: "accuracy", Title: "准确性 · Accuracy"}},
	}
}

func sampleMaterials() []Material {
	return []Material{{ID: "m1", Title: "文章", Blocks: []MaterialBlock{
		{ID: "b0", Text: "某科技博主综合整理的这篇文章称，地球绿了 5%。"},
	}}}
}

func TestParseAnchorGenComputesOffsetsFromQuote(t *testing.T) {
	raw := "```json\n[{\"block_id\":\"b0\",\"quote\":\"某科技博主综合整理\",\"dimension\":\"权威性 · Authority\",\"question\":\"这位作者是权威吗？\"}]\n```"
	got, err := parseAnchorGen(raw, annotationSpec(), sampleMaterials())
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].BlockID != "b0" || got[0].Author != "ai" || got[0].Question == "" {
		t.Fatalf("bad anchor: %+v", got)
	}
	if got[0].Quote != "某科技博主综合整理" || got[0].Start != 0 {
		t.Fatalf("offset/quote wrong: %+v", got[0])
	}
	if got[0].MaterialID != "m1" {
		t.Fatalf("material id not resolved: %+v", got[0])
	}
}

func TestParseAnchorGenRejectsUnknownBlock(t *testing.T) {
	raw := `[{"block_id":"zzz","quote":"x","dimension":"d","question":"q"}]`
	if _, err := parseAnchorGen(raw, annotationSpec(), sampleMaterials()); err == nil {
		t.Fatal("expected error for unknown block_id")
	}
}

func TestFallbackAnchorsOnePerDimension(t *testing.T) {
	got := fallbackAnchors(annotationSpec(), nil)
	if len(got) != 2 || got[0].Dimension != "权威性 · Authority" || got[0].BlockID != "" || got[0].Author != "ai" {
		t.Fatalf("bad fallback: %+v", got)
	}
}

func TestGenerateUsesStubThenPersistsAnchors(t *testing.T) {
	script := []gateway.StreamEvent{
		{Kind: gateway.EventTextDelta, TextDelta: `[{"block_id":"b0","quote":"某科技博主综合整理","dimension":"权威性 · Authority","question":"作者是谁？"}]`},
		{Kind: gateway.EventDone},
	}
	gen := NewAnchorGenerator(gateway.NewStubProvider(script), func(_ context.Context) (gateway.Resolved, error) { return gateway.Resolved{Provider: "stub"}, nil })
	got, err := gen.Generate(context.Background(), annotationSpec(), sampleMaterials())
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Question != "作者是谁？" {
		t.Fatalf("generate wrong: %+v", got)
	}
}

func TestGenerateFallsBackWhenModelReturnsGarbage(t *testing.T) {
	script := []gateway.StreamEvent{{Kind: gateway.EventTextDelta, TextDelta: "not json at all"}, {Kind: gateway.EventDone}}
	gen := NewAnchorGenerator(gateway.NewStubProvider(script), func(_ context.Context) (gateway.Resolved, error) { return gateway.Resolved{Provider: "stub"}, nil })
	got, err := gen.Generate(context.Background(), annotationSpec(), sampleMaterials())
	if err != nil {
		t.Fatalf("fallback must not error: %v", err)
	}
	if len(got) != 2 { // one per dimension
		t.Fatalf("want 2 fallback anchors, got %d", len(got))
	}
}
