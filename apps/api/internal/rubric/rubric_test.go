package rubric

import "testing"

func TestCTHasTenDimensionsInOrder(t *testing.T) {
	r := CT()
	if r.ID != "ct" {
		t.Fatalf("id = %q, want ct", r.ID)
	}
	want := []string{"D1", "D2", "D3", "D4", "D5", "D6", "D7", "D8", "D9", "D10"}
	if len(r.Dimensions) != len(want) {
		t.Fatalf("got %d dimensions, want %d", len(r.Dimensions), len(want))
	}
	for i, d := range r.Dimensions {
		if d.ID != want[i] {
			t.Errorf("dim %d = %q, want %q", i, d.ID, want[i])
		}
	}
}

func TestCTEveryDimensionHasFourAnchors(t *testing.T) {
	for _, d := range CT().Dimensions {
		if d.Name == "" || d.Framework == "" {
			t.Errorf("%s: empty name/framework", d.ID)
		}
		for _, lvl := range []string{"L1", "L2", "L3", "L4"} {
			if d.Anchors[lvl] == "" {
				t.Errorf("%s: empty %s anchor", d.ID, lvl)
			}
		}
	}
}
