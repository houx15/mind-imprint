package api

// writing_deepen_test.go — white-box coverage for buildDeepenBrief, the
// function that IS this sub-agent's isolation guarantee. It is tested
// directly (not just through the HTTP handler) for the same reason
// parseWritingGuide gets writing_guide_internal_test.go: the safety claim
// lives in one small function, so that function is pinned on its own.

import (
	"strings"
	"testing"

	"mindimprint/api/internal/store/sqlc"
)

// TestWritingTopicLine_AssignedPromptIsTheTeachers: an assigned writing's topic
// is the teacher's prompt. Every builder that opens with the topic carries it
// labelled as the teacher's, none presents the assignment's title as her idea,
// and the planning prompt does not claim she was the one who wanted to write
// it. A writing she opened herself keeps the old lines.
func TestWritingTopicLine_AssignedPromptIsTheTeachers(t *testing.T) {
	prompt := "写一篇关于雨的记叙文，写出雨停之前的那一刻"
	wr := sqlc.Writing{Title: "雨", Lang: "zh", AssignedPrompt: &prompt}
	outline := []sqlc.WritingOutline{{Text: "雨停之前", Role: "中心论点"}}
	built := map[string]string{
		"opening": buildWritingOpeningPrompt(wr, nil),
		"guide":   buildWritingGuidePrompt(wr, outline[0], outline, "", nil),
		"batch":   buildWritingGuideBatchPrompt(wr, outline, nil, nil),
		"coach":   buildWritingCoachProjection(wr, outline, nil, ""),
		"deepen":  buildDeepenBrief(wr, outline, outline[0], "", nil, ""),
		"comment": buildWritingCommentPrompt(wr, "她的整篇稿子", "雨下了一整天。"),
		"plan":    buildWritingPlanPrompt(wr, outline, nil, ""),
	}
	for name, out := range built {
		if !strings.Contains(out, "老师布置的题目："+prompt) {
			t.Errorf("%s prompt is missing the teacher's prompt line:\n%s", name, out)
		}
		if strings.Contains(out, "题目/想法：雨") {
			t.Errorf("%s prompt presents the assignment's title as her idea:\n%s", name, out)
		}
	}
	if strings.Contains(built["plan"], "她一开始说想写的是") {
		t.Errorf("plan prompt says she chose the teacher's prompt:\n%s", built["plan"])
	}

	own := sqlc.Writing{Title: "雨", Lang: "zh"}
	if out := buildWritingPlanPrompt(own, outline, nil, ""); !strings.Contains(out, "她一开始说想写的是：雨") {
		t.Errorf("her own writing lost its opening line:\n%s", out)
	}
	if out := buildWritingOpeningPrompt(own, nil); !strings.Contains(out, "题目/想法：雨") || strings.Contains(out, "老师") {
		t.Errorf("her own writing's opening prompt changed:\n%s", out)
	}

	// The opening's system prompt frames the user message. For an assigned
	// writing it must not tell 印记 the topic is hers and to restate it as what
	// she wants to write; both replaced sentences must actually be found, or
	// the replacer silently does nothing.
	sys := writingOpeningSystemFor(wr)
	for _, phrase := range []string{openingTopicOwn, openingRestateOwn} {
		if !strings.Contains(writingOpeningSystem, phrase) {
			t.Fatalf("writingOpeningSystem no longer contains %q; the assigned variant is not being built", phrase)
		}
		if strings.Contains(sys, phrase) {
			t.Errorf("assigned opening system prompt still says %q", phrase)
		}
	}
	if !strings.Contains(sys, "老师布置的题目") || strings.Contains(sys, "她自己写下的题目") {
		t.Errorf("assigned opening system prompt does not call the topic the teacher's:\n%s", sys)
	}
	if writingOpeningSystemFor(own) != writingOpeningSystem {
		t.Errorf("her own writing's opening system prompt changed")
	}
}

// buildDeepenBrief carries the map and this block — asserted here. Its OTHER
// half of the guarantee (that it does NOT carry the planning transcript) is
// not testable at this function: buildDeepenBrief has no transcript
// parameter to leak through, so a string-absence assertion here would be
// vacuous — nothing could ever make it fail. That guarantee is instead
// proven where a leak could actually occur — the handler, which DOES have
// the atom and could carelessly call ListAtomMessages — by
// TestDeepenTurn_PromptCarriesBriefButNotRoomThread
// (writing_deepen_isolation_test.go).
func TestBuildDeepenBrief_CarriesTheMapAndBlock(t *testing.T) {
	wr := sqlc.Writing{Title: "城市该不该大规模种行道树", Lang: "zh"}
	outline := []sqlc.WritingOutline{
		{Text: "该种，但要先定谁长期养", Role: "中心论点", Depth: 0, Position: 0},
		{Text: "维护年年花钱", Role: "一条理由", Depth: 1, Position: 1},
	}
	block := outline[1]

	brief := buildDeepenBrief(wr, outline, block, "更麻烦的是维护。", []string{"这笔钱谁出？"}, "")

	for _, want := range []string{"城市该不该大规模种行道树", "该种，但要先定谁长期养", "维护年年花钱", "更麻烦的是维护。", "这笔钱谁出？"} {
		if !strings.Contains(brief, want) {
			t.Errorf("brief is missing %q", want)
		}
	}
}
