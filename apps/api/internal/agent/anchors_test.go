package agent

import (
	"context"
	"strings"
	"testing"
	"unicode/utf8"

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

func TestComputeOffsets_RuneIndicesOnCJK(t *testing.T) {
	text := "过去二十年里发生了一件事：地球比 2000 年绿了一圈。"
	quote := "地球比 2000 年绿了一圈"

	start, end := computeOffsets(text, quote)

	// Derive the expectation from utf8.RuneCountInString (never a hardcoded
	// number): find the quote's byte offset, then count runes before it.
	byteIdx := strings.Index(text, quote)
	if byteIdx < 0 {
		t.Fatalf("quote not found in text (test setup bug)")
	}
	wantStart := utf8.RuneCountInString(text[:byteIdx])
	wantEnd := wantStart + utf8.RuneCountInString(quote)

	if start != wantStart || end != wantEnd {
		t.Fatalf("computeOffsets(%q, %q) = (%d, %d), want (%d, %d) — byte offsets leaking instead of rune offsets", text, quote, start, end, wantStart, wantEnd)
	}

	// The real contract: slicing runes with these indices round-trips to the
	// original quote. A test that only checked the magic number above would
	// pass against a wrong convention (e.g. UTF-16 code units) too.
	if got := string([]rune(text)[start:end]); got != quote {
		t.Fatalf("round trip failed: string([]rune(text)[%d:%d]) = %q, want %q", start, end, got, quote)
	}

	// Not-found case still returns (0, 0).
	s2, e2 := computeOffsets(text, "这句话不存在")
	if s2 != 0 || e2 != 0 {
		t.Fatalf("not-found case: got (%d, %d), want (0, 0)", s2, e2)
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
	res, err := gen.Generate(context.Background(), annotationSpec(), sampleMaterials())
	if err != nil {
		t.Fatal(err)
	}
	got := res.Anchors
	if len(got) != 1 || got[0].Question != "作者是谁？" {
		t.Fatalf("generate wrong: %+v", got)
	}
	if res.Resolved.Provider != "stub" {
		t.Fatalf("want Resolved carried through on a real call, got %+v", res.Resolved)
	}
}

func TestFallbackAnchorsKeysOffParamsTags(t *testing.T) {
	spec, ok := cards.ByID("craap")
	if !ok {
		t.Fatal("craap spec not found")
	}
	mats := []Material{{ID: "m1", Blocks: []MaterialBlock{{ID: "b0", Text: "some source text"}}}}
	got := fallbackAnchors(spec, mats)

	// One anchor per completion tag, dimension == the tag (NOT the step title).
	wantDims := map[string]bool{"currency": false, "relevance": false, "authority": false, "accuracy": false, "purpose": false}
	for _, a := range got {
		if _, isTag := wantDims[a.Dimension]; !isTag {
			t.Fatalf("anchor dimension %q is not a completion tag (regression: step-title keying)", a.Dimension)
		}
		if a.Author != "ai" || a.Answer != "" {
			t.Fatalf("generated anchor must be ai-authored with empty answer, got author=%q answer=%q", a.Author, a.Answer)
		}
		if a.Question == "" {
			t.Fatalf("anchor for %q has empty question", a.Dimension)
		}
		wantDims[a.Dimension] = true
	}
	for tag, seen := range wantDims {
		if !seen {
			t.Fatalf("no generated anchor for completion tag %q", tag)
		}
	}
}

func TestGenerateFallsBackWhenModelUsesOffVocabularyDimensions(t *testing.T) {
	spec, ok := cards.ByID("craap")
	if !ok {
		t.Fatal("craap spec not found")
	}
	mats := []Material{{ID: "m1", Blocks: []MaterialBlock{{ID: "b0", Text: "some source text"}}}}
	// Well-formed JSON, but dimensions are off-vocabulary (e.g. "Currency" /
	// "时效性" instead of the tag "currency") — this must NOT be minted as-is,
	// because EvaluateCompletion's every_tag_present would never match.
	raw := `[
		{"block_id":"b0","quote":"some source","dimension":"Currency","question":"数据是哪一年的？"},
		{"block_id":"b0","quote":"some source","dimension":"C · Currency 时效性","question":"数据是哪一年的？"},
		{"block_id":"b0","quote":"some source","dimension":"时效性","question":"数据是哪一年的？"}
	]`
	script := []gateway.StreamEvent{{Kind: gateway.EventTextDelta, TextDelta: raw}, {Kind: gateway.EventDone}}
	gen := NewAnchorGenerator(gateway.NewStubProvider(script), func(_ context.Context) (gateway.Resolved, error) { return gateway.Resolved{Provider: "stub"}, nil })
	res, err := gen.Generate(context.Background(), spec, mats)
	if err != nil {
		t.Fatalf("fallback must not error: %v", err)
	}
	got := res.Anchors
	if res.Resolved.Provider != "stub" {
		t.Fatalf("want Resolved carried through even though parsing fell back (the call still cost money), got %+v", res.Resolved)
	}
	wantTags := map[string]bool{"currency": false, "relevance": false, "authority": false, "accuracy": false, "purpose": false}
	if len(got) != len(wantTags) {
		t.Fatalf("want %d tag-keyed fallback anchors, got %d: %+v", len(wantTags), len(got), got)
	}
	for _, a := range got {
		if _, isTag := wantTags[a.Dimension]; !isTag {
			t.Fatalf("anchor dimension %q leaked from off-vocabulary model output instead of falling back", a.Dimension)
		}
		wantTags[a.Dimension] = true
	}
	for tag, seen := range wantTags {
		if !seen {
			t.Fatalf("no fallback anchor for completion tag %q", tag)
		}
	}
}

func TestParseAnchorGenRejectsOffVocabularyDimensionForTaggedCard(t *testing.T) {
	spec, ok := cards.ByID("craap")
	if !ok {
		t.Fatal("craap spec not found")
	}
	mats := []Material{{ID: "m1", Blocks: []MaterialBlock{{ID: "b0", Text: "some source text"}}}}
	raw := `[{"block_id":"b0","quote":"some source","dimension":"Currency","question":"q"}]`
	if _, err := parseAnchorGen(raw, spec, mats); err == nil {
		t.Fatal("expected error: off-vocabulary dimension for a tag-keyed card must not parse cleanly")
	}
}

func TestGenerateFallsBackWhenModelReturnsGarbage(t *testing.T) {
	script := []gateway.StreamEvent{{Kind: gateway.EventTextDelta, TextDelta: "not json at all"}, {Kind: gateway.EventDone}}
	gen := NewAnchorGenerator(gateway.NewStubProvider(script), func(_ context.Context) (gateway.Resolved, error) { return gateway.Resolved{Provider: "stub"}, nil })
	res, err := gen.Generate(context.Background(), annotationSpec(), sampleMaterials())
	if err != nil {
		t.Fatalf("fallback must not error: %v", err)
	}
	if len(res.Anchors) != 2 { // one per dimension
		t.Fatalf("want 2 fallback anchors, got %d", len(res.Anchors))
	}
}
