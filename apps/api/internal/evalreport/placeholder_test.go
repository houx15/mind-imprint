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

// TestPlaceholder_EnumValuesValid guards against placeholder literals drifting
// out of the contract's enums (packages/contracts owns the Zod truth; Go's
// Validate only checks the envelope, so a bad enum value here would pass Go
// but throw on the frontend's EvaluationReport.parse()).
func TestPlaceholder_EnumValuesValid(t *testing.T) {
	r := Placeholder("p-1", "rep-1", "u-1", "Phoebe", "T", "EE", "2026-08-06T00:00:00Z")

	validRiskTypes := map[string]bool{
		"ai-ghostwrite":        true,
		"missing-source":       true,
		"argument-logic":       true,
		"data-scope":           true,
		"rabbit-hole-offtopic": true,
	}
	for _, risk := range r.Risks {
		if !validRiskTypes[risk.Type] {
			t.Errorf("risk type %q is not a valid RiskType enum value", risk.Type)
		}
	}

	validEventKinds := map[string]bool{
		"chat": true, "reading": true, "graph": true,
		"writing": true, "review": true, "milestone": true,
	}
	for _, ev := range r.Events {
		if !validEventKinds[ev.Kind] {
			t.Errorf("event kind %q is not a valid EventKind enum value", ev.Kind)
		}
	}

	validDepthIDs := map[string]bool{"D1": true, "D2": true, "D3": true, "D4": true, "D5": true, "D6": true}
	for _, d := range r.Depth {
		if !validDepthIDs[d.ID] {
			t.Errorf("depth id %q is not one of D1..D6", d.ID)
		}
		if d.Level < 1 || d.Level > 4 {
			t.Errorf("depth %s level %d out of range 1..4", d.ID, d.Level)
		}
	}

	validAutonomyIDs := map[string]bool{"A1": true, "A2": true, "A3": true, "A4": true, "A5": true, "A6": true}
	for _, a := range r.Autonomy {
		if !validAutonomyIDs[a.ID] {
			t.Errorf("autonomy id %q is not one of A1..A6", a.ID)
		}
		if a.Band < 0 || a.Band > 5 {
			t.Errorf("autonomy %s band %d out of range 0..5", a.ID, a.Band)
		}
	}
}
