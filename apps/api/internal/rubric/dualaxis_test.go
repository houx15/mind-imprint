package rubric

import "testing"

func TestModelParsesDualAxis(t *testing.T) {
	m := Model()
	if m.ID != "dualaxis" {
		t.Fatalf("id = %q, want dualaxis", m.ID)
	}
	if m.Axiom != "两轴永不合成总分；单次会话为事件级证据，不构成人级档位判定" {
		t.Fatalf("axiom mismatch: %q", m.Axiom)
	}
	if len(m.Dimensions) != 6 {
		t.Fatalf("dims = %d, want 6", len(m.Dimensions))
	}
	depth := DepthDims()
	if len(depth) != 4 {
		t.Fatalf("depth dims = %d, want 4", len(depth))
	}
	for _, d := range depth {
		if d.Axis != "depth" {
			t.Fatalf("dim %s axis = %q, want depth", d.ID, d.Axis)
		}
		for _, k := range []string{"0", "1", "2", "3"} {
			if d.Anchors[k] == "" {
				t.Fatalf("depth dim %s missing anchor %s", d.ID, k)
			}
		}
	}
	if AutonomyDim().ObservationGuide == "" {
		t.Fatalf("autonomy dim missing observationGuide")
	}
	if CrossDim().Guide == "" {
		t.Fatalf("cross dim missing guide")
	}
	if len(m.PromptTiers) != 4 || len(m.SoloLevels) != 4 {
		t.Fatalf("tiers=%d solo=%d, want 4/4", len(m.PromptTiers), len(m.SoloLevels))
	}
}
