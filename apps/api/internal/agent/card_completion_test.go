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
