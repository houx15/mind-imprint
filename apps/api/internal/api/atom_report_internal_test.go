package api

// atom_report_internal_test.go — validateMoments is unexported, so its test
// lives here (package api), not in atom_report_test.go (package api_test).

import (
	"strings"
	"testing"

	"github.com/google/uuid"

	"mindimprint/api/internal/store/sqlc"
)

func TestValidateMoments(t *testing.T) {
	corpus := "我觉得人均排放更能说明责任。\n作者只讲了总量，没有讲人口。"
	got := validateMoments([]reportMoment{
		{Quote: "人均排放更能说明责任", Where: "我的收获"},
		{Quote: "她展现了批判性思维", Where: "我的收获"}, // AI's own prose — dropped
		{Quote: "中国碳排放世界第一", Where: "批注"},   // the article's — dropped
		{Quote: "  ", Where: "批注"},          // empty — dropped
	}, corpus)
	if len(got) != 1 {
		t.Fatalf("kept %d, want 1: %+v", len(got), got)
	}
	if got[0].Quote != "人均排放更能说明责任" {
		t.Errorf("kept the wrong moment: %+v", got[0])
	}
}

// TestReportDedupesMomentsAgainstKeep is F4: her 收获 is already rendered verbatim
// as `keep`, so a moment that is the SAME sentence (or a substring of it)
// must be dropped — otherwise the same line prints twice on one report. A
// moment unrelated to keep, and a nil/empty keep, must both pass every
// moment through untouched.
func TestReportDedupesMomentsAgainstKeep(t *testing.T) {
	keep := &reportKeep{Label: "我的收获", Text: "我觉得应该多看数据来源，而不是只看结论。"}
	moments := []reportMoment{
		{Quote: "我觉得应该多看数据来源，而不是只看结论。", Where: "我的收获"}, // exact match with keep — dropped
		{Quote: "多看数据来源", Where: "我的收获"},               // substring of keep — dropped
		{Quote: "碳排放全球第一", Where: "写论证的时候"},            // unrelated — kept
	}

	got := dedupeMomentsAgainstKeep(moments, keep)

	if len(got) != 1 {
		t.Fatalf("kept %d, want 1: %+v", len(got), got)
	}
	if got[0].Quote != "碳排放全球第一" {
		t.Errorf("kept the wrong moment: %+v", got[0])
	}

	// A nil keep (writing reports never have one) must be a no-op.
	if got := dedupeMomentsAgainstKeep(moments, nil); len(got) != len(moments) {
		t.Errorf("nil keep should pass every moment through, got %d of %d", len(got), len(moments))
	}

	// An empty-text keep must also be a no-op — nothing to dedupe against.
	if got := dedupeMomentsAgainstKeep(moments, &reportKeep{Label: "我的收获", Text: "   "}); len(got) != len(moments) {
		t.Errorf("blank-text keep should pass every moment through, got %d of %d", len(got), len(moments))
	}
}

// TestLiteReportSystemAddressesHerDirectly — V2 fix: the report's `gains`
// used to come back in third person ("她抓住了…"), which reads as the
// student being described to someone else rather than a teacher speaking to
// her. The prompt must instruct second person (你) for gains, and must not
// itself model or invite third-person reference (她/这位学生/该生) — a
// third-person exemplar in the instruction would just teach the model the
// habit it's supposed to forbid. Precedent for pinning a prompt clause this
// way: TestReadingCoachSystemCarriesTheRulings in
// reading_coach_internal_test.go.
// TestBuildReadingLensNotes covers the four behaviours the lens-notes
// builder must have (see this feature's task doc): it picks the STUDENT
// anchor's quote — never the AI's own grounding example — pairs it with
// framework_fill.finding, skips any card that isn't 'submitted', skips a
// card whose id the registry doesn't know (rather than printing the raw
// id), and preserves the rows' own order.
func TestBuildReadingLensNotes(t *testing.T) {
	atomID := uuid.New()

	craapNote := sqlc.AtomCard{
		ID: uuid.New(), AtomID: atomID, CardID: "craap", Status: "submitted",
		Anchors: []byte(`[
			{"author":"ai","quote":"AI 挑的例句，不该出现"},
			{"author":"student","quote":"她自己选的句子一"}
		]`),
		FrameworkFill: []byte(`{"finding":"发现一"}`),
	}
	unsubmitted := sqlc.AtomCard{
		ID: uuid.New(), AtomID: atomID, CardID: "sift", Status: "proposed",
		Anchors:       []byte(`[{"author":"student","quote":"还没提交，不该出现"}]`),
		FrameworkFill: []byte(`{"finding":"还没提交，不该出现"}`),
	}
	unknownID := sqlc.AtomCard{
		ID: uuid.New(), AtomID: atomID, CardID: "not_a_real_card", Status: "submitted",
		Anchors:       []byte(`[{"author":"student","quote":"未知卡片，不该出现"}]`),
		FrameworkFill: []byte(`{"finding":"未知卡片，不该出现"}`),
	}
	concessionNote := sqlc.AtomCard{
		ID: uuid.New(), AtomID: atomID, CardID: "concession", Status: "submitted",
		Anchors:       []byte(`[{"author":"student","quote":"她自己选的句子二"}]`),
		FrameworkFill: []byte(`{"finding":"发现二"}`),
	}
	empty := sqlc.AtomCard{
		// submitted, known id, but neither a student anchor nor a finding —
		// nothing to show, must be skipped.
		ID: uuid.New(), AtomID: atomID, CardID: "sift", Status: "submitted",
		Anchors:       []byte(`[]`),
		FrameworkFill: []byte(`{}`),
	}

	got := buildReadingLensNotes([]sqlc.AtomCard{craapNote, unsubmitted, unknownID, concessionNote, empty})

	if len(got) != 2 {
		t.Fatalf("got %d lens notes, want 2: %+v", len(got), got)
	}

	if got[0].Quote != "她自己选的句子一" {
		t.Errorf("note 0 quote = %q, want the STUDENT anchor, not the AI example", got[0].Quote)
	}
	if got[0].Finding != "发现一" {
		t.Errorf("note 0 finding = %q, want %q", got[0].Finding, "发现一")
	}
	if got[0].Lens != "信源辨识卡 CRAAP / CRRAAB" {
		t.Errorf("note 0 lens = %q, want the craap card's registry display name", got[0].Lens)
	}

	// Order: craapNote comes before concessionNote in the input, and the
	// unsubmitted/unknown/empty rows between them must not shift that.
	if got[1].Quote != "她自己选的句子二" {
		t.Errorf("note 1 quote = %q, want the second submitted card's student pick, in order", got[1].Quote)
	}
	if got[1].Finding != "发现二" {
		t.Errorf("note 1 finding = %q, want %q", got[1].Finding, "发现二")
	}
	if got[1].Lens != "让步段 · 以退为进" {
		t.Errorf("note 1 lens = %q, want the concession card's registry display name", got[1].Lens)
	}
}

// TestBuildReadingLensNotesDropsDegradedFinding is the fix for the finding
// raised on 47d1c7c4's review: ev.Degraded (selectionEvalDTO, readeval.go)
// means agent.fallbackEval minted the finding, not a model that actually
// read her sentence — "你选了这句作为证据。" is the REAL canned string
// fallbackEval sets (reading_eval.go), used verbatim here so this test
// fails if that string ever drifts silently. A degraded card must keep her
// quote (her pick is her work regardless of whether the model said anything
// useful) but drop the canned finding — never present it as the room's
// genuine 发现.
func TestBuildReadingLensNotesDropsDegradedFinding(t *testing.T) {
	atomID := uuid.New()
	const cannedFinding = "你选了这句作为证据。" // agent.fallbackEval's exact canned text

	degraded := sqlc.AtomCard{
		ID: uuid.New(), AtomID: atomID, CardID: "craap", Status: "submitted",
		Anchors:       []byte(`[{"author":"student","quote":"她自己选的句子"}]`),
		FrameworkFill: []byte(`{"finding":"` + cannedFinding + `","degraded":true}`),
	}

	got := buildReadingLensNotes([]sqlc.AtomCard{degraded})

	if len(got) != 1 {
		t.Fatalf("got %d lens notes, want 1 (the quote must survive a degraded finding): %+v", len(got), got)
	}
	if got[0].Quote != "她自己选的句子" {
		t.Errorf("quote = %q, want her pick kept even though the finding degraded", got[0].Quote)
	}
	if got[0].Finding != "" {
		t.Errorf("finding = %q, want empty — the canned fallback text must never be shown as a genuine 发现", got[0].Finding)
	}
}

func TestLiteReportSystemAddressesHerDirectly(t *testing.T) {
	if !strings.Contains(liteReportSystem, "用\"你\"称呼她本人") {
		t.Error("liteReportSystem must explicitly instruct gains to address her as 你, not describe her in third person")
	}
	for _, banned := range []string{"她抓住了", "她能说出", "她调整了", "这位学生", "该生"} {
		if strings.Contains(liteReportSystem, banned) {
			t.Errorf("liteReportSystem must not model third-person reference as an example, found %q", banned)
		}
	}
}
