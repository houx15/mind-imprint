package agent

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"mindimprint/api/internal/cards"
)

func mustSift(t *testing.T) cards.Spec {
	t.Helper()
	s, ok := cards.ByID("sift_craap")
	if !ok {
		t.Fatal("sift_craap not in catalog")
	}
	return s
}

func TestRefeedSkippedIsMinimal(t *testing.T) {
	t.Skip("fixture generated in Task 9")
	spec := mustSift(t)
	p := SerializeCardForRefeed(spec, CardInstance{CardID: "sift_craap", Status: "skipped"})
	b, err := json.Marshal(p)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	wantBytes, err := os.ReadFile(filepath.Join("testdata", "refeed_sift_skipped.json"))
	if err != nil {
		t.Fatalf("read golden: %v", err)
	}
	if !jsonEqual(t, b, wantBytes) {
		t.Fatalf("skipped refeed mismatch.\n got: %s\nwant: %s", b, wantBytes)
	}
}

func TestRefeedCompletedMatchesGolden(t *testing.T) {
	t.Skip("fixture generated in Task 9")
	spec := mustSift(t)
	inst := CardInstance{
		CardID: "sift_craap",
		Status: "completed",
		FieldValues: map[string]map[string]any{
			"sift": {
				"stop": "证明中国让地球更可持续",
				"sources": []any{
					map[string]any{"name": "NASA", "type": "官方", "verdict": "可信"},
					map[string]any{"name": "Nature Sustainability", "type": "学者/机构", "verdict": "可信"},
				},
				"better": "原始研究来自 NASA / Nature Sustainability",
				"trace":  "https://www.nature.com/...",
			},
		},
	}
	p := SerializeCardForRefeed(spec, inst)
	b, err := json.Marshal(p)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	wantBytes, err := os.ReadFile(filepath.Join("testdata", "refeed_sift_completed.json"))
	if err != nil {
		t.Fatalf("read golden: %v", err)
	}
	if !jsonEqual(t, b, wantBytes) {
		t.Fatalf("completed refeed mismatch.\n got: %s\nwant: %s", b, wantBytes)
	}
}

func TestRefeedOmitsEmptyFields(t *testing.T) {
	spec := mustSift(t)
	inst := CardInstance{
		CardID: "sift_craap",
		Status: "completed",
		FieldValues: map[string]map[string]any{
			"sift": {"stop": "x"},
		},
	}
	p := SerializeCardForRefeed(spec, inst)
	if len(p.Steps) != 1 {
		t.Fatalf("steps = %d, want 1", len(p.Steps))
	}
	if len(p.Steps[0].Answers) != 1 {
		t.Fatalf("answers = %d, want 1 (only filled field)", len(p.Steps[0].Answers))
	}
	if p.Steps[0].Answers[0].Value != "x" {
		t.Fatalf("answer value = %v", p.Steps[0].Answers[0].Value)
	}
}

func TestRefeedRepeatableGroupRemap(t *testing.T) {
	spec := mustSift(t)
	inst := CardInstance{
		CardID: "sift_craap",
		Status: "completed",
		FieldValues: map[string]map[string]any{
			"sift": {
				"sources": []any{
					map[string]any{"name": "NASA", "type": "官方", "verdict": "可信"},
				},
			},
		},
	}
	p := SerializeCardForRefeed(spec, inst)
	rows, ok := p.Steps[0].Answers[0].Value.([]map[string]any)
	if !ok {
		t.Fatalf("repeatable value not []map: %T", p.Steps[0].Answers[0].Value)
	}
	if rows[0]["来源"] != "NASA" || rows[0]["类型"] != "官方" || rows[0]["可信？"] != "可信" {
		t.Fatalf("remap wrong: %v", rows[0])
	}
}

// jsonEqual compares two JSON byte slices semantically (key order independent).
func jsonEqual(t *testing.T, a, b []byte) bool {
	t.Helper()
	var x, y any
	if err := json.Unmarshal(a, &x); err != nil {
		t.Fatalf("unmarshal a: %v", err)
	}
	if err := json.Unmarshal(b, &y); err != nil {
		t.Fatalf("unmarshal b: %v", err)
	}
	ab, _ := json.Marshal(x)
	bb, _ := json.Marshal(y)
	return string(ab) == string(bb)
}
