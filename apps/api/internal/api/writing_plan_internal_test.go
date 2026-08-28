package api

// writing_plan_internal_test.go — white-box tests for writing_plan.go that
// need package-internal access: the raw system-prompt string (unexported
// constant) and rootInsertPosition (unexported helper).
//
// TestWritingPlanSystem_TeachesWholePieceJudgment is the RED-PHASE fix for
// B0: TestPlanTurn_AcceptsATopLevelOpeningBesideTheThesis (writing_plan_test.go,
// package api_test) exercises insertPlanNode/insertPlanNode's storage layer
// through a scripted stub — it passed even before the prompt changed, because
// nothing in storage ever forbade a second depth-0 node. This test instead
// pins the actual prompt text, so it genuinely fails without the change B0
// makes.

import (
	"strings"
	"testing"

	"mindimprint/api/internal/store/sqlc"
)

func TestWritingPlanSystem_TeachesWholePieceJudgment(t *testing.T) {
	for _, want := range []string{
		"最上层不止中心论点",
		"不超过 200 字",
	} {
		if !strings.Contains(writingPlanSystem, want) {
			t.Fatalf("writingPlanSystem is missing %q", want)
		}
	}
	if strings.Contains(writingPlanSystem, "不超过 120 字") {
		t.Fatalf("writingPlanSystem still carries the old 120-字 cap")
	}
}

func TestRootInsertPosition(t *testing.T) {
	rows := []sqlc.WritingOutline{
		{Text: "不该一刀切禁手机", Role: "中心论点", Depth: 0, Position: 0},
		{Text: "理由一", Role: "一条理由", Depth: 1, Position: 1},
	}

	cases := []struct {
		name string
		role string
		want int32
	}{
		{"opening role goes first", "开头", 0},
		{"opening synonym goes first", "钩子式开头", 0},
		{"closing role appends", "结尾", int32(len(rows))},
		{"thesis role appends", "中心论点", int32(len(rows))},
		{"unrecognised role appends", "反方会说的话", int32(len(rows))},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := rootInsertPosition(c.role, rows); got != c.want {
				t.Fatalf("rootInsertPosition(%q, rows) = %d, want %d", c.role, got, c.want)
			}
		})
	}
}
