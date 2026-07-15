package agent

import "mindimprint/api/internal/skills"

// BudgetVerdict classifies a word count against a skill's word band. state is
// "in" when Min ≤ wc ≤ Max (inclusive both ends), "over" above Max, "under"
// below Min. delta is a NON-NEGATIVE magnitude — words past Max when over,
// words short of Min when under, 0 when in — so the UI reads direction from
// state and renders delta directly (超出 {delta} / 还差 {delta}). A nil band
// means no budget is configured: nothing to violate, so ("in", 0).
func BudgetVerdict(wc int, band *skills.WordBudget) (state string, delta int) {
	if band == nil {
		return "in", 0
	}
	switch {
	case wc > band.Max:
		return "over", wc - band.Max
	case wc < band.Min:
		return "under", band.Min - wc
	default:
		return "in", 0
	}
}
