package api

// reading_coach_internal_test.go — white-box tests for the coach's reply
// parsing and prompt assembly (unexported symbols), following the
// internal-test convention used by reading_brief_internal_test.go /
// writing_guide_internal_test.go / writing_plan_internal_test.go. Lives
// separately from reading_coach_test.go (package api_test, black-box) which
// cannot see unexported functions like parseReadingCoachReply.

import (
	"strings"
	"testing"
)

func TestParseReadingCoachReplyLens(t *testing.T) {
	valid := map[string]bool{"b1": true, "b2": true}
	allow := func(id string) bool { return id == "craap" }

	cases := []struct {
		name string
		json string
		want string
	}{
		{"aimed and allowed", `{"reply":"看这段","advance":"","focusBlock":"b2","lens":"craap"}`, "craap"},
		// An un-aimed lens is indistinguishable from her opening 透镜库
		// herself — the whole point of the coach summoning one is that it
		// lands on a paragraph it just talked about.
		{"un-aimed is dropped", `{"reply":"来看看来源","advance":"","focusBlock":"","lens":"craap"}`, ""},
		{"not allowed right now", `{"reply":"x","advance":"","focusBlock":"b1","lens":"sift"}`, ""},
		{"unknown id", `{"reply":"x","advance":"","focusBlock":"b1","lens":"nope"}`, ""},
		{"absent", `{"reply":"x","advance":"","focusBlock":"b1"}`, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := parseReadingCoachReply(tc.json, valid, "zh", allow)
			if !ok {
				t.Fatalf("parse failed for %s", tc.json)
			}
			if got.Lens != tc.want {
				t.Errorf("lens = %q, want %q", got.Lens, tc.want)
			}
			if got.Reply == "" {
				t.Error("a dropped lens must not take the reply down with it")
			}
		})
	}
}

// TestValidateReadingPicks — a pick is only kept when it points at a real
// paragraph AND quotes it literally. A paraphrase, a right-words-wrong-block
// pick, an unknown block id, and an empty quote are all dropped silently.
func TestValidateReadingPicks(t *testing.T) {
	blocks := []Block{
		{ID: "b1", Text: "中国的碳排放总量位居世界第一。"},
		{ID: "b2", Text: "但人均排放仍低于多数发达国家。"},
	}
	got := validateReadingPicks([]readingPick{
		{BlockID: "b1", Quote: "碳排放总量位居世界第一"}, // literal substring — kept
		{BlockID: "b2", Quote: "人均排放低于发达国家"},   // paraphrase — dropped
		{BlockID: "b9", Quote: "中国的碳排放总量"},       // no such block — dropped
		{BlockID: "b1", Quote: "  "},                  // empty — dropped
		{BlockID: "b1", Quote: "但人均排放仍低于多数发达国家。"}, // right words, wrong block — dropped
	}, blocks)
	if len(got) != 1 {
		t.Fatalf("kept %d picks, want 1: %+v", len(got), got)
	}
	if got[0].BlockID != "b1" || got[0].Quote != "碳排放总量位居世界第一" {
		t.Errorf("kept the wrong pick: %+v", got[0])
	}
}

// TestReadingCoachPrompt_RendersPicksAsOrdinalsNeverBlockIDs — a survived
// pick must show up as its own section, labelled 第几段 the same way the
// paragraph listing above it is, and must NEVER speak the block id (b2):
// the system prompt is explicit that block ids are an internal marker the
// model must not repeat back to her, and picks are no exception. An empty
// picks slice must omit the section entirely rather than print a bare
// heading.
func TestReadingCoachPrompt_RendersPicksAsOrdinalsNeverBlockIDs(t *testing.T) {
	blocks := []Block{
		{ID: "b1", Text: "中国的碳排放总量位居世界第一。"},
		{ID: "b2", Text: "但人均排放仍低于多数发达国家。"},
	}

	withPicks := buildReadingCoachPrompt("标题", blocks, nil, nil,
		[]readingPick{{BlockID: "b2", Quote: "但人均排放仍低于多数发达国家。"}}, "")
	if !strings.Contains(withPicks, "【她在文章里点出来的句子】") {
		t.Fatalf("missing picks section:\n%s", withPicks)
	}
	if !strings.Contains(withPicks, "第2段：「但人均排放仍低于多数发达国家。」") {
		t.Fatalf("pick not rendered as 第几段:\n%s", withPicks)
	}
	// The paragraph listing above legitimately says b2（第2段）— but nothing
	// in the picks section itself may hand the model a bare "b2" as
	// something it could echo back to her.
	if i := strings.Index(withPicks, "【她在文章里点出来的句子】"); i >= 0 && strings.Contains(withPicks[i:], "b2") {
		t.Errorf("picks section leaks the block id:\n%s", withPicks[i:])
	}

	noPicks := buildReadingCoachPrompt("标题", blocks, nil, nil, nil, "")
	if strings.Contains(noPicks, "【她在文章里点出来的句子】") {
		t.Errorf("picks section must be omitted when no picks survive:\n%s", noPicks)
	}
}

// TestParseReadingCoachReplyLens_NilLensOK — the guard's `lensOK == nil`
// short-circuit has no caller yet exercising it: every real call site passes
// a real predicate. A nil lensOK must still drop the lens rather than panic
// on the nil call.
func TestParseReadingCoachReplyLens_NilLensOK(t *testing.T) {
	valid := map[string]bool{"b1": true, "b2": true}

	got, ok := parseReadingCoachReply(
		`{"reply":"看这段","advance":"","focusBlock":"b2","lens":"craap"}`, valid, "zh", nil)
	if !ok {
		t.Fatalf("parse failed")
	}
	if got.Lens != "" {
		t.Errorf("lens = %q, want dropped when lensOK is nil", got.Lens)
	}
	if got.Reply == "" {
		t.Error("a dropped lens must not take the reply down with it")
	}
}
