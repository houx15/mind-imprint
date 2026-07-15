package agent

import (
	"testing"

	"mindimprint/api/internal/skills"
)

func TestBudgetVerdict(t *testing.T) {
	band := &skills.WordBudget{Min: 1500, Max: 2000}
	cases := []struct {
		wc        int
		wantState string
		wantDelta int
	}{
		{1780, "in", 0},
		{1500, "in", 0},
		{2000, "in", 0},
		{2340, "over", 340},
		{2001, "over", 1},
		{1290, "under", 210},
		{1499, "under", 1},
	}
	for _, c := range cases {
		state, delta := BudgetVerdict(c.wc, band)
		if state != c.wantState || delta != c.wantDelta {
			t.Errorf("BudgetVerdict(%d) = (%q,%d), want (%q,%d)", c.wc, state, delta, c.wantState, c.wantDelta)
		}
	}
	if state, delta := BudgetVerdict(999, nil); state != "in" || delta != 0 {
		t.Errorf("BudgetVerdict(_, nil) = (%q,%d), want (in,0)", state, delta)
	}
}
