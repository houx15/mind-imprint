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
		ID: "craap", Name: "CRAAP", Mode: "annotation",
		Steps: []cards.Step{{Key: "authority", Title: "权威性 · Authority"}, {Key: "accuracy", Title: "准确性 · Accuracy"}},
	}
}

func sampleMaterials() []Material {
	return []Material{{ID: "m1", Title: "文章", Blocks: []MaterialBlock{
		{ID: "b0", Text: "某科技博主综合整理的这篇文章称，地球绿了 5%。"},
	}}}
}

func TestParseAnchorGenComputesOffsetsFromQuote(t *testing.T) {
	raw := "```json\n[{\"block_id\":\"m0:b0\",\"quote\":\"某科技博主综合整理\",\"dimension\":\"权威性 · Authority\",\"question\":\"这位作者是权威吗？\"}]\n```"
	got, err := parseAnchorGen(raw, annotationSpec(), sampleMaterials(), GuidanceL1)
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

// The tolerant path: a model quote that is verbatim MODULO CJK punctuation
// (half-width ,/. and straight quotes where the article has full-width) must
// still ground to the ORIGINAL text's rune span — this is what stops the
// summon hard-failing on an otherwise-correct quote.
func TestComputeOffsets_TolerantOfPunctuationNormalization(t *testing.T) {
	text := "研究者说：“地球变绿了”，但样本只覆盖北半球。"
	// straight quotes + half-width comma + surrounding whitespace (trimmed);
	// verbatim modulo punctuation, so NOT a byte substring of the original.
	quote := ` "地球变绿了",但样本只覆盖北半球 `

	start, end := computeOffsets(text, quote)
	if end <= start {
		t.Fatalf("tolerant match failed: got (%d, %d) — the summon would hard-fail", start, end)
	}
	// The returned span indexes the ORIGINAL text: slicing it back yields the
	// original (full-width-punctuated) sentence, not the model's normalized form.
	got := string([]rune(text)[start:end])
	if !strings.Contains(got, "地球变绿了") || !strings.Contains(got, "北半球") {
		t.Fatalf("tolerant span maps to wrong text: %q", got)
	}
}

// buildCardExamplePrompt must fold in a lens's task_prompt/selection_hint/
// example_focus when reading_lens is present (the whole reason lenses replaced
// tool cards on this path); a card WITHOUT reading_lens falls back to
// purpose/trigger and never references those fields.
func TestBuildCardExamplePrompt_UsesLensFieldsWhenPresent(t *testing.T) {
	lens, ok := cards.ByID("lens-logic")
	if !ok {
		t.Fatal("lens-logic not in registry")
	}
	p := buildCardExamplePrompt(lens)
	if lens.ReadingLens == nil {
		t.Fatal("lens-logic has no reading_lens (test precondition)")
	}
	for _, want := range []string{lens.ReadingLens.TaskPrompt, lens.ReadingLens.SelectionHint, lens.ReadingLens.ExampleFocus} {
		if want != "" && !strings.Contains(p, want) {
			t.Fatalf("lens prompt missing %q\n---\n%s", want, p)
		}
	}

	// A plain tool card (no reading_lens) still builds a prompt and does not
	// crash trying to read the nil block.
	tool, ok := cards.ByID("argument-map")
	if !ok {
		t.Fatal("argument-map not in registry")
	}
	if tool.ReadingLens != nil {
		t.Fatal("argument-map unexpectedly has reading_lens")
	}
	if got := buildCardExamplePrompt(tool); got == "" {
		t.Fatal("tool-card prompt is empty")
	}
}

func TestParseAnchorGenL1ResolvesTheRightMaterial(t *testing.T) {
	materials := []Material{
		{ID: "mat-a", Title: "NASA 观测", Blocks: []MaterialBlock{{ID: "b0", Text: "叶面积指数上升。"}}},
		{ID: "mat-b", Title: "BP 统计", Blocks: []MaterialBlock{{ID: "b0", Text: "煤炭消费仍在上升。"}}},
	}
	text := `[{"block_id":"m0:b0","quote":"叶面积指数上升","dimension":"authority","question":"这条数据出自谁？"}]`
	out, err := parseAnchorGen(text, cards.Spec{}, materials, GuidanceL1)
	if err != nil {
		t.Fatalf("parseAnchorGen: %v", err)
	}
	if out[0].MaterialID != "mat-a" {
		t.Errorf("MaterialID = %q, want mat-a — a flat lookup resolves this to the LAST material", out[0].MaterialID)
	}
	if out[0].BlockID != "b0" {
		t.Errorf("BlockID = %q — the stored block id stays material-local", out[0].BlockID)
	}
	if out[0].Start == 0 && out[0].End == 0 {
		t.Error("offsets must resolve against the correct material's block text")
	}
}

func TestParseAnchorGenRejectsUnknownBlock(t *testing.T) {
	raw := `[{"block_id":"zzz","quote":"x","dimension":"d","question":"q"}]`
	if _, err := parseAnchorGen(raw, annotationSpec(), sampleMaterials(), GuidanceL1); err == nil {
		t.Fatal("expected error for unknown block_id")
	}
}

// TestParseAnchorGenRejectsUnqualifiedBlockIDWithMultipleMaterials pins the
// fail-closed contract: blockLookup is qualified-only (no bare-id fallback),
// so a reply that emits the bare form the L1 prompt no longer asks for must
// error rather than silently resolve to either material.
// The two materials deliberately have DIFFERENT block ids, so the bare "b0" is
// unambiguous — the one shape that discriminates. An earlier revision indexed
// bare ids when only one material carried them, and a fixture where both
// materials own "b0" errors under that revision too, so it would have pinned
// nothing.
func TestParseAnchorGenRejectsUnqualifiedBlockIDWithMultipleMaterials(t *testing.T) {
	materials := []Material{
		{ID: "mat-a", Title: "NASA 观测", Blocks: []MaterialBlock{{ID: "b0", Text: "叶面积指数上升。"}}},
		{ID: "mat-b", Title: "BP 统计", Blocks: []MaterialBlock{{ID: "c0", Text: "煤炭消费仍在上升。"}}},
	}
	raw := `[{"block_id":"b0","quote":"叶面积指数上升","dimension":"authority","question":"这条数据出自谁？"}]`
	_, err := parseAnchorGen(raw, cards.Spec{}, materials, GuidanceL1)
	if err == nil {
		t.Fatal("expected error: unqualified block_id must not silently resolve to either material")
	}
	if !strings.Contains(err.Error(), "unknown block_id") {
		t.Fatalf("want unknown block_id error, got: %v", err)
	}
}

func TestFallbackAnchorsOnePerDimension(t *testing.T) {
	got := fallbackAnchors(annotationSpec(), nil, GuidanceL1)
	if len(got) != 2 || got[0].Dimension != "权威性 · Authority" || got[0].BlockID != "" || got[0].Author != "ai" {
		t.Fatalf("bad fallback: %+v", got)
	}
}

func TestGenerateUsesStubThenPersistsAnchors(t *testing.T) {
	script := []gateway.StreamEvent{
		{Kind: gateway.EventTextDelta, TextDelta: `[{"block_id":"m0:b0","quote":"某科技博主综合整理","dimension":"权威性 · Authority","question":"作者是谁？"}]`},
		{Kind: gateway.EventDone},
	}
	gen := NewAnchorGenerator(gateway.NewStubProvider(script), func(_ context.Context) (gateway.Resolved, error) { return gateway.Resolved{Provider: "stub"}, nil })
	res, err := gen.Generate(context.Background(), annotationSpec(), sampleMaterials(), GuidanceL1)
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
	got := fallbackAnchors(spec, mats, GuidanceL1)

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
		{"block_id":"m0:b0","quote":"some source","dimension":"Currency","question":"数据是哪一年的？"},
		{"block_id":"m0:b0","quote":"some source","dimension":"C · Currency 时效性","question":"数据是哪一年的？"},
		{"block_id":"m0:b0","quote":"some source","dimension":"时效性","question":"数据是哪一年的？"}
	]`
	script := []gateway.StreamEvent{{Kind: gateway.EventTextDelta, TextDelta: raw}, {Kind: gateway.EventDone}}
	gen := NewAnchorGenerator(gateway.NewStubProvider(script), func(_ context.Context) (gateway.Resolved, error) { return gateway.Resolved{Provider: "stub"}, nil })
	res, err := gen.Generate(context.Background(), spec, mats, GuidanceL1)
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
	raw := `[{"block_id":"m0:b0","quote":"some source","dimension":"Currency","question":"q"}]`
	if _, err := parseAnchorGen(raw, spec, mats, GuidanceL1); err == nil {
		t.Fatal("expected error: off-vocabulary dimension for a tag-keyed card must not parse cleanly")
	}
}

func TestGenerateFallsBackWhenModelReturnsGarbage(t *testing.T) {
	script := []gateway.StreamEvent{{Kind: gateway.EventTextDelta, TextDelta: "not json at all"}, {Kind: gateway.EventDone}}
	gen := NewAnchorGenerator(gateway.NewStubProvider(script), func(_ context.Context) (gateway.Resolved, error) { return gateway.Resolved{Provider: "stub"}, nil })
	res, err := gen.Generate(context.Background(), annotationSpec(), sampleMaterials(), GuidanceL1)
	if err != nil {
		t.Fatalf("fallback must not error: %v", err)
	}
	if len(res.Anchors) != 2 { // one per dimension
		t.Fatalf("want 2 fallback anchors, got %d", len(res.Anchors))
	}
}

// countingProvider (loop_test.go, N3b) already wraps a real gateway.Provider
// and counts Stream calls — reused here rather than redeclared.

func TestGenerate_L1_Unchanged(t *testing.T) {
	script := []gateway.StreamEvent{
		{Kind: gateway.EventTextDelta, TextDelta: `[{"block_id":"m0:b0","quote":"某科技博主综合整理","dimension":"权威性 · Authority","question":"这位作者是权威吗？"}]`},
		{Kind: gateway.EventDone},
	}
	cp := &countingProvider{inner: gateway.NewStubProvider(script)}
	gen := NewAnchorGenerator(cp, func(_ context.Context) (gateway.Resolved, error) { return gateway.Resolved{Provider: "stub"}, nil })

	res, err := gen.Generate(context.Background(), annotationSpec(), sampleMaterials(), GuidanceL1)
	if err != nil {
		t.Fatal(err)
	}
	if cp.calls != 1 {
		t.Fatalf("want provider called once at L1, got %d calls", cp.calls)
	}
	if len(res.Anchors) != 1 {
		t.Fatalf("want 1 anchor, got %d: %+v", len(res.Anchors), res.Anchors)
	}
	a := res.Anchors[0]
	if a.Author != "ai" {
		t.Fatalf("L1 anchor must be author=ai, got %q", a.Author)
	}
	if a.Quote == "" {
		t.Fatalf("L1 anchor must have a non-empty quote, got %+v", a)
	}
	if a.BlockID != "b0" {
		t.Fatalf("L1 anchor must have a resolved block_id, got %+v", a)
	}
	if a.Start == 0 && a.End == 0 {
		t.Fatalf("L1 anchor must have real rune offsets, got %+v", a)
	}
}

func TestGenerate_L2_QuestionOnlyNoSpan(t *testing.T) {
	script := []gateway.StreamEvent{
		{Kind: gateway.EventTextDelta, TextDelta: `[{"dimension":"权威性 · Authority","question":"这段材料的作者有没有说明自己的身份或资历？"},{"dimension":"准确性 · Accuracy","question":"这段材料里的数字有没有可以核对的来源？"}]`},
		{Kind: gateway.EventDone},
	}
	cp := &countingProvider{inner: gateway.NewStubProvider(script)}
	gen := NewAnchorGenerator(cp, func(_ context.Context) (gateway.Resolved, error) { return gateway.Resolved{Provider: "stub"}, nil })

	res, err := gen.Generate(context.Background(), annotationSpec(), sampleMaterials(), GuidanceL2)
	if err != nil {
		t.Fatal(err)
	}
	if cp.calls != 1 {
		t.Fatalf("want provider called once at L2 (still metered), got %d calls", cp.calls)
	}
	if res.Resolved.Provider == "" {
		t.Fatalf("want Resolved populated at L2 (still metered), got %+v", res.Resolved)
	}
	if len(res.Anchors) == 0 {
		t.Fatal("want at least one anchor")
	}
	for _, a := range res.Anchors {
		if a.Author != "student" {
			t.Fatalf("L2 anchor must be author=student, got %q: %+v", a.Author, a)
		}
		if a.Question == "" {
			t.Fatalf("L2 anchor must have a non-empty question: %+v", a)
		}
		if a.BlockID != "" || a.Start != 0 || a.End != 0 || a.Quote != "" {
			t.Fatalf("L2 anchor span must be blank, got %+v", a)
		}
	}
}

func TestGenerate_L3_NoModelCall(t *testing.T) {
	spec, ok := cards.ByID("craap")
	if !ok {
		t.Fatal("craap spec not found")
	}
	mats := []Material{{ID: "m1", Blocks: []MaterialBlock{{ID: "b0", Text: "some source text"}}}}
	// A provider that panics if invoked: L3 must never reach the network.
	panicky := &panicProvider{}
	gen := NewAnchorGenerator(panicky, func(_ context.Context) (gateway.Resolved, error) {
		t.Fatal("resolver must not be called at L3 — no call happens, so there is nothing to resolve a key for")
		return gateway.Resolved{}, nil
	})

	res, err := gen.Generate(context.Background(), spec, mats, GuidanceL3)
	if err != nil {
		t.Fatal(err)
	}
	if panicky.calls != 0 {
		t.Fatalf("want provider NEVER called at L3, got %d calls", panicky.calls)
	}
	if res.Resolved.Provider != "" {
		t.Fatalf("want Resolved.Provider empty at L3 (nothing to meter), got %+v", res.Resolved)
	}
	wantTags := map[string]bool{"currency": false, "relevance": false, "authority": false, "accuracy": false, "purpose": false}
	if len(res.Anchors) != len(wantTags) {
		t.Fatalf("want one anchor per spec.Params.Tags entry (%d), got %d: %+v", len(wantTags), len(res.Anchors), res.Anchors)
	}
	for _, a := range res.Anchors {
		if _, isTag := wantTags[a.Dimension]; !isTag {
			t.Fatalf("anchor dimension %q not in tag vocabulary", a.Dimension)
		}
		wantTags[a.Dimension] = true
		if a.Author != "student" {
			t.Fatalf("L3 anchor must be author=student, got %+v", a)
		}
		if a.Question != "" {
			t.Fatalf("L3 anchor must have blank question, got %+v", a)
		}
		if a.BlockID != "" || a.Start != 0 || a.End != 0 || a.Quote != "" {
			t.Fatalf("L3 anchor span must be blank, got %+v", a)
		}
	}
	for tag, seen := range wantTags {
		if !seen {
			t.Fatalf("no L3 anchor for tag %q", tag)
		}
	}
}

// panicProvider fails the test the moment Stream is invoked. It exists
// alongside countingProvider (which merely counts) because the L3 test must
// prove zero-call, not just count calls after the fact — a real call to a
// provider that isn't there would otherwise hang or nil-panic in a less
// diagnosable way.
type panicProvider struct {
	calls int
}

func (p *panicProvider) Stream(ctx context.Context, r gateway.Resolved, req gateway.ChatRequest) (<-chan gateway.StreamEvent, error) {
	p.calls++
	panic("provider must not be called at GuidanceL3")
}

func TestGenerate_L2_ParseFailureFallsBackAtL2(t *testing.T) {
	script := []gateway.StreamEvent{{Kind: gateway.EventTextDelta, TextDelta: "not json at all"}, {Kind: gateway.EventDone}}
	gen := NewAnchorGenerator(gateway.NewStubProvider(script), func(_ context.Context) (gateway.Resolved, error) { return gateway.Resolved{Provider: "stub"}, nil })

	res, err := gen.Generate(context.Background(), annotationSpec(), sampleMaterials(), GuidanceL2)
	if err != nil {
		t.Fatalf("fallback must not error: %v", err)
	}
	if len(res.Anchors) != 2 { // one per dimension
		t.Fatalf("want 2 fallback anchors, got %d", len(res.Anchors))
	}
	for _, a := range res.Anchors {
		if a.Author != "student" {
			t.Fatalf("degraded L2 fallback must stay L2-shaped (author=student), got author=%q: %+v", a.Author, a)
		}
		if a.Question == "" {
			t.Fatalf("degraded L2 fallback must still have a question (only the span is missing), got %+v", a)
		}
		if a.BlockID != "" || a.Start != 0 || a.End != 0 || a.Quote != "" {
			t.Fatalf("degraded L2 fallback span must be blank, got %+v", a)
		}
	}
}
