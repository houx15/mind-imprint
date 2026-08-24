package studio

import (
	"encoding/json"
	"testing"
)

// TestMaterialDTOLateralNoteAlwaysPresent asserts lateralRelation/
// lateralJudgment are present (as "") even with no cross_check yet — this
// DTO's convention (matching role/tier/takeaway): a field with no producer
// yet is present-and-empty, never hidden behind an omitted/optional key.
func TestMaterialDTOLateralNoteAlwaysPresent(t *testing.T) {
	m := MaterialDTO{ID: "m1", Blocks: []MaterialBlockDTO{}, Anchors: []json.RawMessage{}}
	raw, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	var probe struct {
		LateralRelation *string `json:"lateralRelation"`
		LateralJudgment *string `json:"lateralJudgment"`
	}
	if err := json.Unmarshal(raw, &probe); err != nil {
		t.Fatal(err)
	}
	if probe.LateralRelation == nil || *probe.LateralRelation != "" {
		t.Fatalf("lateralRelation = %v, want present and \"\"", probe.LateralRelation)
	}
	if probe.LateralJudgment == nil || *probe.LateralJudgment != "" {
		t.Fatalf("lateralJudgment = %v, want present and \"\"", probe.LateralJudgment)
	}
}
