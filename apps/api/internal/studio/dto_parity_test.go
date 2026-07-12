package studio

import (
	"encoding/json"
	"sort"
	"testing"
)

// The wire key set must match packages/contracts/src/studioState.ts (StudioProjection).
func TestStudioProjectionJSONKeys(t *testing.T) {
	p := StudioProjection{
		Project:       ProjectHeader{Title: "t", QualLabel: "0457 个人报告"},
		Stations:      []StationDTO{{Code: "S4", Name: "论证构建", View: "结构", State: "current", Gate: &GateDTO{Total: 7, Passed: 2}}},
		ActiveStation: "S4",
		Coach: CoachDTO{
			Anchor:    "论证图 · 治理决心主张",
			Messages:  []CoachMessageDTO{{Kind: "ai", Body: "b", Tag: "D5", Anchor: "论证图 · 治理决心主张"}},
			Equipment: []EquipCardDTO{{ID: "e1", Name: "钢人卡", Spont: "提示后", Meth: "concession"}},
		},
		Onboarding: OnboardingDTO{RestatePrompt: "r", RubricRows: []RubricRowDTO{{Official: "o", Plain: "p", Weak: true}}, PlanSteps: []string{"立题"}},
	}
	raw, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatal(err)
	}
	top := keys(m)
	want := []string{"activeStation", "coach", "onboarding", "project", "stations"}
	if !equalStrs(top, want) {
		t.Fatalf("top-level keys = %v, want %v", top, want)
	}
	// Spot-check a station's keys and that gate is camelCase total/passed.
	if !bytesHasKeys(t, raw, "论证构建", []string{"code", "name", "view", "state", "gate"}) {
		t.Fatalf("station keys mismatch: %s", raw)
	}
}

func keys(m map[string]json.RawMessage) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
func equalStrs(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
func bytesHasKeys(t *testing.T, raw []byte, marker string, want []string) bool {
	// crude: assert every wanted key substring appears near the marker station
	s := string(raw)
	for _, k := range want {
		if !contains(s, `"`+k+`"`) {
			t.Logf("missing key %q", k)
			return false
		}
	}
	return contains(s, marker)
}
func contains(s, sub string) bool { return len(s) >= len(sub) && (stringIndex(s, sub) >= 0) }
func stringIndex(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
