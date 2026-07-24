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
}

func TestDepthDims(t *testing.T) {
	depth := DepthDims()
	if len(depth) != 6 {
		t.Fatalf("depth dims = %d, want 6", len(depth))
	}
	for _, d := range depth {
		if d.ID == "" || d.Name == "" || d.Means == "" {
			t.Fatalf("depth dim %+v missing ID/Name/Means", d)
		}
		for _, k := range []string{"L1", "L2", "L3", "L4"} {
			if d.Anchors[k] == "" {
				t.Fatalf("depth dim %s missing anchor %s", d.ID, k)
			}
		}
	}
}

func TestAutonomySignals(t *testing.T) {
	signals := AutonomySignals()
	if len(signals) != 6 {
		t.Fatalf("autonomy signals = %d, want 6", len(signals))
	}
	for _, s := range signals {
		if s.ID == "" || s.Name == "" || s.Means == "" || s.Event == "" {
			t.Fatalf("autonomy signal %+v missing field", s)
		}
	}
}

func TestLenses(t *testing.T) {
	lenses := Lenses()
	if len(lenses) != 6 {
		t.Fatalf("lenses = %d, want 6", len(lenses))
	}
	for _, l := range lenses {
		if l.ID == "" || l.Name == "" || l.Guide == "" {
			t.Fatalf("lens %+v missing field", l)
		}
	}
}

func TestAutonomyBand(t *testing.T) {
	if AutonomyBand() == "" {
		t.Fatalf("AutonomyBand() is empty")
	}
}

func TestStandard(t *testing.T) {
	std, ok := Standard("ap-research")
	if !ok {
		t.Fatalf("Standard(ap-research) not found")
	}
	if len(std.Components) != 4 {
		t.Fatalf("components = %d, want 4", len(std.Components))
	}
	if len(std.AlignmentItems) != 5 {
		t.Fatalf("alignmentItems = %d, want 5", len(std.AlignmentItems))
	}

	if _, ok := Standard("nope"); ok {
		t.Fatalf("Standard(nope) unexpectedly found")
	}
}
