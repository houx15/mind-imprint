package api

// atom_report_internal_test.go — validateMoments is unexported, so its test
// lives here (package api), not in atom_report_test.go (package api_test).

import (
	"strings"
	"testing"
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
