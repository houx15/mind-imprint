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

// TestRefeedFoldsAnnotationAnchors covers the flagship keystone/annotation card:
// its answers live on `anchors` (field_values stays empty), and they must reach
// the refeed payload — otherwise "摘要回灌" drops the student's actual thinking.
func TestRefeedFoldsAnnotationAnchors(t *testing.T) {
	spec := mustSift(t)
	inst := CardInstance{
		CardID: "sift_craap",
		Status: "completed",
		// annotation cards leave field_values empty; the student's answers ride anchors.
		Anchors: []Anchor{
			{Dimension: "溯源 (SIFT)", Question: "这条信息最初来自哪里？", Answer: "原始研究来自 NASA / Nature Sustainability", Author: "ai"},
			{Dimension: "可信度 (CRAAP)", Question: "这个来源可信吗？", Answer: "NASA 是官方机构，可信", Author: "ai"},
			{Dimension: "空", Question: "没回答", Answer: "", Author: "ai"}, // unanswered → must not leak
		},
	}
	p := SerializeCardForRefeed(spec, inst)
	if p.Status != "completed" {
		t.Fatalf("status = %q, want completed", p.Status)
	}
	if len(p.Steps) != 2 {
		t.Fatalf("steps = %d, want 2 (one per answered anchor dimension)", len(p.Steps))
	}
	if p.Steps[0].Title != "溯源 (SIFT)" {
		t.Fatalf("step[0] title = %q", p.Steps[0].Title)
	}
	if p.Steps[0].Answers[0].Label != "这条信息最初来自哪里？" ||
		p.Steps[0].Answers[0].Value != "原始研究来自 NASA / Nature Sustainability" {
		t.Fatalf("step[0] answer wrong: %+v", p.Steps[0].Answers[0])
	}
}

func TestAnchorStepsLabelsFallBackToQuoteBeforeDimension(t *testing.T) {
	inst := CardInstance{Anchors: []Anchor{
		// no Question — a sort/scale/matrix anchor. The sentence (quote) is
		// the only useful label; the dimension is already the step title.
		{Quote: "中国碳排放全球第一", Dimension: "事实", Answer: "可以去核查"},
		// no Question and no Quote — falls all the way back to the dimension.
		{Dimension: "观点", Answer: "需要给理由"},
	}}
	steps := anchorSteps(inst)
	if len(steps) != 2 {
		t.Fatalf("want 2 steps, got %d", len(steps))
	}
	if steps[0].Title != "事实" || steps[0].Answers[0].Label != "中国碳排放全球第一" {
		t.Fatalf("step0 = %+v", steps[0])
	}
	if steps[1].Answers[0].Label != "观点" {
		t.Fatalf("step1 label = %q, want 观点", steps[1].Answers[0].Label)
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
