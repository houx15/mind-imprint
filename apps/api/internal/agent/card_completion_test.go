package agent

import (
	"reflect"
	"testing"

	"mindimprint/api/internal/cards"
)

// craapSpecFixture is a minimal fixture mirroring the CRAAP C2 config
// authored in packages/contracts/cards/craap.json (Task 1), kept inline so
// these tests stay pure and independent of the embedded catalog.
func craapSpecFixture() cards.Spec {
	return cards.Spec{
		ID:        "craap",
		Primitive: "annotate",
		Params: cards.Params{
			Tags: []string{"currency", "relevance", "authority", "accuracy", "purpose"},
		},
		Completion: []cards.CompletionPredicate{
			{Kind: "every_tag_present", Tags: []string{"currency", "relevance", "authority", "accuracy", "purpose"}},
			{Kind: "field_written_by", Field: "risk_note", Author: "student"},
		},
		Observe: []cards.ObserveRule{
			{When: "tag=authority AND note_len<15", Verb: "post_intervention", Level: "I2"},
		},
	}
}

func completeAnchors() []Anchor {
	return []Anchor{
		{ID: "a0", Dimension: "currency", Author: "ai", Answer: "2024年发布，数据较新"},
		{ID: "a1", Dimension: "relevance", Author: "ai", Answer: "直接支持中国可持续论点"},
		{ID: "a2", Dimension: "authority", Author: "ai", Answer: "NASA地球观测团队发布，具备权威性"},
		{ID: "a3", Dimension: "accuracy", Author: "ai", Answer: "数据可在Nature Sustainability交叉核对"},
		{ID: "a4", Dimension: "purpose", Author: "ai", Answer: "科普告知性质，非商业推广"},
		{ID: "a5", Dimension: "risk_note", Author: "student", Answer: "仍需留意样本口径是否一致"},
	}
}

func TestEvaluateCompletion_AllPresent(t *testing.T) {
	spec := craapSpecFixture()
	complete, missing := EvaluateCompletion(spec, completeAnchors())
	if !complete {
		t.Fatalf("complete = false, missing = %v, want true", missing)
	}
	if missing != nil {
		t.Fatalf("missing = %v, want nil", missing)
	}
}

func TestEvaluateCompletion_MissingTag(t *testing.T) {
	spec := craapSpecFixture()
	anchors := completeAnchors()
	// Drop the authority anchor.
	var without []Anchor
	for _, a := range anchors {
		if a.Dimension == "authority" {
			continue
		}
		without = append(without, a)
	}
	complete, missing := EvaluateCompletion(spec, without)
	if complete {
		t.Fatal("complete = true, want false (authority missing)")
	}
	if !reflect.DeepEqual(missing, []string{"authority"}) {
		t.Fatalf("missing = %v, want [authority]", missing)
	}
}

func TestEvaluateCompletion_RiskNoteWrongAuthor(t *testing.T) {
	spec := craapSpecFixture()
	anchors := completeAnchors()
	for i := range anchors {
		if anchors[i].Dimension == "risk_note" {
			anchors[i].Author = "ai" // student never wrote it
		}
	}
	complete, missing := EvaluateCompletion(spec, anchors)
	if complete {
		t.Fatal("complete = true, want false (risk_note not student-authored)")
	}
	if len(missing) == 0 {
		t.Fatal("missing should report risk_note")
	}
}

func TestEvaluateCompletion_RiskNoteEmptyAnswer(t *testing.T) {
	spec := craapSpecFixture()
	anchors := completeAnchors()
	for i := range anchors {
		if anchors[i].Dimension == "risk_note" {
			anchors[i].Answer = ""
		}
	}
	complete, _ := EvaluateCompletion(spec, anchors)
	if complete {
		t.Fatal("complete = true, want false (risk_note answer empty)")
	}
}

func TestObserveCandidates_WeakAnswerFires(t *testing.T) {
	spec := craapSpecFixture()
	anchors := []Anchor{
		{ID: "a2", Dimension: "authority", Author: "ai", Answer: "还可以"}, // 9 bytes, < 15
	}
	got := ObserveCandidates(spec, "ci-1", anchors)
	if len(got) != 1 {
		t.Fatalf("candidates = %d, want 1", len(got))
	}
	c := got[0]
	if c.Verb != "post_intervention" {
		t.Fatalf("verb = %q, want post_intervention", c.Verb)
	}
	if c.Level != "I2" {
		t.Fatalf("level = %q, want I2", c.Level)
	}
	if c.AnchorID == "" {
		t.Fatal("AnchorID should not be empty")
	}
}

func TestObserveCandidates_StrongAnswerSilent(t *testing.T) {
	spec := craapSpecFixture()
	anchors := []Anchor{
		{ID: "a2", Dimension: "authority", Author: "ai", Answer: "NASA地球观测团队发布，具备权威性和同行评审背书"},
	}
	got := ObserveCandidates(spec, "ci-1", anchors)
	if len(got) != 0 {
		t.Fatalf("candidates = %v, want none", got)
	}
}

func TestObserveCandidates_UnrelatedTagSilent(t *testing.T) {
	spec := craapSpecFixture()
	anchors := []Anchor{
		{ID: "a0", Dimension: "currency", Author: "ai", Answer: "短"},
	}
	got := ObserveCandidates(spec, "ci-1", anchors)
	if len(got) != 0 {
		t.Fatalf("candidates = %v, want none (rule only watches authority)", got)
	}
}

// siftSpec mirrors SIFT's lateral-reading completion config: the "find"
// dimension is where the student names what an INDEPENDENT source says.
func siftSpec() cards.Spec {
	return cards.Spec{
		ID:         "sift",
		Params:     cards.Params{LateralDimension: "find"},
		Completion: []cards.CompletionPredicate{{Kind: "lateral_source_present"}},
	}
}

func TestLateralSourcePresent_RequiresADifferentMaterial(t *testing.T) {
	// She wrote a "find" answer, but it is anchored to the SAME source she is
	// checking. That is not lateral reading — it is reading the page again.
	anchors := []Anchor{
		{ID: "a1", MaterialID: "mat-blog", Dimension: "stop", Answer: "夸张", Author: "student"},
		{ID: "a2", MaterialID: "mat-blog", Dimension: "find", Answer: "文章自己说的", Author: "student"},
	}
	complete, missing := EvaluateCompletion(siftSpec(), anchors)
	if complete {
		t.Fatal("must not complete: the 'lateral' source is the same material")
	}
	if len(missing) != 1 || missing[0] != "find" {
		t.Fatalf("missing = %v, want [find]", missing)
	}
}

func TestLateralSourcePresent_RequiresANonEmptyAnswer(t *testing.T) {
	anchors := []Anchor{
		{ID: "a1", MaterialID: "mat-blog", Dimension: "stop", Answer: "夸张", Author: "student"},
		{ID: "a2", MaterialID: "mat-nasa", Dimension: "find", Answer: "   ", Author: "student"},
	}
	if complete, _ := EvaluateCompletion(siftSpec(), anchors); complete {
		t.Fatal("must not complete: a source was added but she said nothing about it")
	}
}

func TestLateralSourcePresent_CompletesWithARealOtherSource(t *testing.T) {
	anchors := []Anchor{
		{ID: "a1", MaterialID: "mat-blog", Dimension: "stop", Answer: "夸张", Author: "student"},
		{ID: "a2", MaterialID: "mat-nasa", Dimension: "find", Answer: "NASA 只说绿化面积，没说可持续", Author: "student"},
	}
	complete, missing := EvaluateCompletion(siftSpec(), anchors)
	if !complete {
		t.Fatalf("must complete: a real other source is in the project; missing = %v", missing)
	}
}

// toulminSpec loads the real embedded "toulmin" card spec (Slice 7 Task 1)
// so this test exercises the graph_slots_complete predicate against the
// actual params.slots config, not a hand-rolled fixture.
func toulminSpec(t *testing.T) cards.Spec {
	t.Helper()
	s, ok := cards.ByID("toulmin")
	if !ok {
		t.Fatalf("toulmin card not found")
	}
	return s
}

func TestGraphSlotsComplete(t *testing.T) {
	spec := toulminSpec(t)

	// One text anchor per slot; needSrc slots (warrant/evidence/concession)
	// also get a source anchor. All five text anchors ≥12 chars.
	full := []Anchor{
		{Dimension: "claim", Answer: "中国的政策在净效果上让全球更可持续。", Author: "student"},
		{Dimension: "warrant", Answer: "卫星植被数据到可持续判断之间的推理如下所述。", Author: "student"},
		{Dimension: "warrant", MaterialID: "m_nasa", Author: "student"},
		{Dimension: "evidence", Answer: "NASA 观测显示中国主导了全球变绿的增量。", Author: "student"},
		{Dimension: "evidence", MaterialID: "m_nasa", Author: "student"},
		{Dimension: "counter", Answer: "反方最强点：中国碳排放总量全球第一。", Author: "student"},
		{Dimension: "concession", Answer: "承认排放第一，但人均与历史累积远低于发达国家。", Author: "student"},
		{Dimension: "concession", MaterialID: "m_bp", Author: "student"},
	}
	if ok, missing := EvaluateCompletion(spec, full); !ok {
		t.Fatalf("full graph should complete, missing=%v", missing)
	}

	// Drop the evidence source anchor -> evidence slot is unsourced.
	noSrc := append([]Anchor{}, full[:4]...)
	noSrc = append(noSrc, full[5:]...) // skip the evidence source anchor
	ok, missing := EvaluateCompletion(spec, noSrc)
	if ok {
		t.Fatalf("evidence with no source must not complete")
	}
	if len(missing) != 1 || missing[0] != "evidence" {
		t.Fatalf("missing = %v, want [evidence]", missing)
	}

	// Short claim text (<12 chars) -> claim slot incomplete.
	shortClaim := append([]Anchor{}, full...)
	shortClaim[0].Answer = "太短"
	if ok, _ := EvaluateCompletion(spec, shortClaim); ok {
		t.Fatalf("claim under 12 chars must not complete")
	}
}

func TestItemsBucketedCountsOnlyInVocabularyAnsweredAnchors(t *testing.T) {
	spec := cards.Spec{Completion: []cards.CompletionPredicate{
		{Kind: "items_bucketed", Tags: []string{"事实", "观点"}, Min: 2},
	}}
	anchors := []Anchor{
		{Dimension: "事实", Answer: "可以去核查"},
		{Dimension: "观点", Answer: " "},        // blank answer — not counted
		{Dimension: "rewrite", Answer: "改写句"}, // out of vocabulary — ignored
	}
	complete, missing := EvaluateCompletion(spec, anchors)
	if complete {
		t.Fatal("expected incomplete: only 1 of 2 items bucketed")
	}
	if len(missing) != 1 || missing[0] != "items" {
		t.Fatalf("missing = %v, want [items]", missing)
	}

	anchors = append(anchors, Anchor{Dimension: "观点", Answer: "需要给理由"})
	if complete, _ := EvaluateCompletion(spec, anchors); !complete {
		t.Fatal("expected complete at min")
	}
}

func TestItemsBucketedCoexistsWithFieldWrittenBy(t *testing.T) {
	spec := cards.Spec{Completion: []cards.CompletionPredicate{
		{Kind: "items_bucketed", Tags: []string{"强证据"}, Min: 1},
		{Kind: "field_written_by", Field: "rewrite", Author: "student"},
	}}
	placed := []Anchor{{Dimension: "强证据", Answer: "多个独立来源"}}
	if complete, missing := EvaluateCompletion(spec, placed); complete || len(missing) != 1 || missing[0] != "rewrite" {
		t.Fatalf("want incomplete missing [rewrite], got complete=%v missing=%v", complete, missing)
	}
	withRewrite := append(placed, Anchor{Dimension: "rewrite", Author: "student", Answer: "中国很可能……"})
	if complete, _ := EvaluateCompletion(spec, withRewrite); !complete {
		t.Fatal("expected complete once rewrite is written")
	}
}

func TestMatrixCompleteRequiresEveryCellOfEnoughRows(t *testing.T) {
	spec := cards.Spec{
		Params: cards.Params{
			Cols:     []cards.Axis{{ID: "position"}, {ID: "grounds"}, {ID: "blind_spot"}},
			MinItems: 2,
		},
		Completion: []cards.CompletionPredicate{{Kind: "matrix_complete"}},
	}
	// One complete row, one row missing blind_spot.
	anchors := []Anchor{
		{Quote: "政府", Dimension: "position", Answer: "治理有决心"},
		{Quote: "政府", Dimension: "grounds", Answer: "植树与限排政策"},
		{Quote: "政府", Dimension: "blind_spot", Answer: "回避了排放总量"},
		{Quote: "环保组织", Dimension: "position", Answer: "进展不足"},
		{Quote: "环保组织", Dimension: "grounds", Answer: "碳排放全球第一"},
	}
	complete, missing := EvaluateCompletion(spec, anchors)
	if complete {
		t.Fatal("expected incomplete: 环保组织 has no blind_spot cell")
	}
	if len(missing) != 1 || missing[0] != "环保组织" {
		t.Fatalf("missing = %v, want [环保组织]", missing)
	}

	anchors = append(anchors, Anchor{Quote: "环保组织", Dimension: "blind_spot", Answer: "低估了转型速度"})
	if complete, _ := EvaluateCompletion(spec, anchors); !complete {
		t.Fatal("expected complete once every cell is filled")
	}
}

func TestMatrixCompleteReportsRowsWhenTooFewRows(t *testing.T) {
	spec := cards.Spec{
		Params:     cards.Params{Cols: []cards.Axis{{ID: "position"}}, MinItems: 2},
		Completion: []cards.CompletionPredicate{{Kind: "matrix_complete"}},
	}
	anchors := []Anchor{{Quote: "政府", Dimension: "position", Answer: "治理有决心"}}
	complete, missing := EvaluateCompletion(spec, anchors)
	if complete || len(missing) != 1 || missing[0] != "rows" {
		t.Fatalf("want incomplete missing [rows], got complete=%v missing=%v", complete, missing)
	}
}
