package evalreport

import "testing"

func TestIsStudentFacingPurpose(t *testing.T) {
	facing := map[string]string{
		"proposal_annotation":    "review",
		"essay_annotation":       "review",
		"order_review":           "review",
		"question_card":          "card",
		"read_card_example":      "card",
		"read_router":            "reading",
		"read_eval":              "reading",
		"reading_takeaway_draft": "reading",
	}
	for p, want := range facing {
		g, ok := IsStudentFacingPurpose(p)
		if !ok || g != want {
			t.Errorf("IsStudentFacingPurpose(%q) = (%q,%v), want (%q,true)", p, g, ok, want)
		}
	}

	// Internal subagents are excluded.
	for _, p := range []string{
		"dig_query", "suggest_placement", "search_guidance", "exploration_guide",
		"edge_propose", "exploration_review", "plan_gen", "opening", "classify",
		"coach", "coach_compact", "anchors", "summary",
	} {
		if g, ok := IsStudentFacingPurpose(p); ok {
			t.Errorf("IsStudentFacingPurpose(%q) = (%q,true), want excluded", p, g)
		}
	}
}
