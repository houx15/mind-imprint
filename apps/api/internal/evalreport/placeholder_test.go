package evalreport

import (
	"encoding/json"
	"testing"
)

func TestPlaceholder_IsAbundantAndValid(t *testing.T) {
	r := Placeholder("p-1", "rep-1", "u-1", "Phoebe", "中国是否让地球变得更可持续？", "Extended Essay", "2026-08-06T00:00:00Z")

	if len(r.Depth) != 6 || len(r.Autonomy) != 6 {
		t.Fatalf("want 6 D + 6 A dims, got %d/%d", len(r.Depth), len(r.Autonomy))
	}
	if len(r.Materials) < 6 {
		t.Fatalf("want >=6 materials, got %d", len(r.Materials))
	}
	if len(r.Events) < 5 {
		t.Fatalf("want >=5 events, got %d", len(r.Events))
	}
	for _, d := range r.Depth {
		if len(d.Evidence) == 0 || d.Evidence[0].Quote == "" || d.Evidence[0].Boundary == "" {
			t.Fatalf("dim %s lacks rich evidence", d.ID)
		}
	}
	for _, m := range r.Materials {
		if m.CannotSupport == "" || m.FinalStatus == "" {
			t.Fatalf("material %s lacks cannotSupport/finalStatus", m.MaterialID)
		}
	}
	// round-trips through Validate
	raw, _ := json.Marshal(r)
	if _, err := Validate(raw); err != nil {
		t.Fatalf("placeholder output failed Validate: %v", err)
	}
}

func TestPlaceholder_Deterministic(t *testing.T) {
	a := Placeholder("p-1", "rep-1", "u-1", "Phoebe", "T", "EE", "2026-08-06T00:00:00Z")
	b := Placeholder("p-1", "rep-1", "u-1", "Phoebe", "T", "EE", "2026-08-06T00:00:00Z")
	ra, _ := json.Marshal(a)
	rb, _ := json.Marshal(b)
	if string(ra) != string(rb) {
		t.Fatal("placeholder is not deterministic")
	}
}
