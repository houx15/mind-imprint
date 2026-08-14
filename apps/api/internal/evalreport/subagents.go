package evalreport

// G4 — which subagents the report's promptLens / toolUsage cover.
//
// Not every internal LLM call has student-authored input worth surfacing. The
// report covers ONLY the three student-facing subagents; the deterministic
// generator filters llm_call rows through this map, and reads each one's
// student-facing input from where it is already durably stored:
//
//   - review  → the whole-draft 批注 reviewer / 整稿体检. Input = the draft the
//     student sent (edit_buffer / draft_snapshot).
//   - card    → the tool-card turns. Input = card_instances.field_values /
//     event_trace (what the student filled). Note the card-REFLECT coach turn
//     meters as purpose "coach", so llm_call purpose alone does not isolate it;
//     the card input is read from card_instances directly, and the purposes
//     below cover the card-summoning turns that DO carry a dedicated purpose.
//   - reading → the read-together room. Input = the student's focused
//     selection while reading, captured as a "reading_focus" event (see
//     api/readturn.go) plus her card fills and takeaways.
//
// Everything else (search/dig, exploration guide/review, edge propose,
// placement, plan-gen, opening, classify, compaction, etc.) is an internal
// subagent and is excluded.
var StudentFacingSubagents = map[string][]string{
	"review":  {"proposal_annotation", "essay_annotation", "order_review"},
	"card":    {"question_card", "read_card_example"},
	"reading": {"read_router", "read_eval", "reading_takeaway_draft"},
}

// IsStudentFacingPurpose reports whether an llm_call purpose belongs to one of
// the three student-facing subagents, and if so which group ("review" / "card"
// / "reading"). Excluded internal purposes return ("", false).
func IsStudentFacingPurpose(purpose string) (group string, ok bool) {
	for g, purposes := range StudentFacingSubagents {
		for _, p := range purposes {
			if p == purpose {
				return g, true
			}
		}
	}
	return "", false
}
