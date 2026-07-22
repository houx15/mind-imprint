package agent

import "testing"

// TestGuidanceFor exercises the ladder (spec §3): 0 -> L1, 1 -> L2, 2+ -> L3.
// A negative count can never occur in practice (it comes from a count(*)),
// but the case is here anyway: a clamp that silently mapped a negative to 0
// uses would be indistinguishable from a real 0, hiding a real bug behind a
// defensive default. GuidanceFor must map it to L1, same as 0, not panic or
// return something else.
func TestGuidanceFor(t *testing.T) {
	cases := []struct {
		uses int
		want GuidanceLevel
	}{
		{0, GuidanceL1},
		{1, GuidanceL2},
		{2, GuidanceL3},
		{3, GuidanceL3},
		{7, GuidanceL3},
		{-1, GuidanceL1},
	}
	for _, c := range cases {
		if got := GuidanceFor(c.uses); got != c.want {
			t.Errorf("GuidanceFor(%d) = %v, want %v", c.uses, got, c.want)
		}
	}
}
