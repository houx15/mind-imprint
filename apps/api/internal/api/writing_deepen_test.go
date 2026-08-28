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

	brief := buildDeepenBrief(wr, outline, block, "更麻烦的是维护。", []string{"这笔钱谁出？"})

	for _, want := range []string{"城市该不该大规模种行道树", "该种，但要先定谁长期养", "维护年年花钱", "更麻烦的是维护。", "这笔钱谁出？"} {
		if !strings.Contains(brief, want) {
			t.Errorf("brief is missing %q", want)
		}
	}
}
