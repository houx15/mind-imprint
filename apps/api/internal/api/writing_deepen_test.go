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

// The brief is what makes this a sub-agent rather than a second general chat:
// it carries the map and this block, and it deliberately does NOT carry the
// planning transcript. That exclusion is the whole cost saving and the whole
// reason it stays on topic, so it is pinned here.
func TestBuildDeepenBrief_CarriesTheMapAndBlockButNotTheTranscript(t *testing.T) {
	wr := sqlc.Writing{Title: "城市该不该大规模种行道树", Lang: "zh"}
	outline := []sqlc.WritingOutline{
		{Text: "该种，但要先定谁长期养", Role: "中心论点", Depth: 0, Position: 0},
		{Text: "维护年年花钱", Role: "一条理由", Depth: 1, Position: 1},
	}
	block := outline[1]

	brief := buildDeepenBrief(wr, outline, block, "更麻烦的是维护。", []string{"这笔钱谁出？"})

	for _, want := range []string{"城市该不该大规模种行道树", "该种，但要先定谁长期养", "维护年年花钱", "更麻烦的是维护。", "这笔钱谁出？"} {
		if !strings.Contains(brief, want) {
			t.Errorf("brief is missing %q", want)
		}
	}
	if strings.Contains(brief, "PLANNING_TRANSCRIPT_MARKER") {
		t.Error("brief carries the planning transcript; it must not")
	}
}
