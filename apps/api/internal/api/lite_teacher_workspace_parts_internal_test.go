package api

// lite_teacher_workspace_parts_internal_test.go — what the shared §6 checks
// read out of a patch. A surface whose patch nests its strings (the parent
// report's {"body": {"<section>": "<text>"}}) must not get past either check.

import (
	"slices"
	"strings"
	"testing"

	"mindimprint/api/internal/liteworkspace"
)

func checkedPartsForTest(t *testing.T, patch map[string]any) []string {
	t.Helper()
	parts, err := liteWorkspaceCheckedParts("好的。", nil, patch)
	if err != nil {
		t.Fatalf("checked parts: %v", err)
	}
	return parts
}

// countCheckFails is the handler's per-part head-count loop, for a class of
// twelve with nothing counted this turn.
func countCheckFails(parts []string) bool {
	for _, p := range parts {
		if bad := liteworkspace.UngroundedCounts(p, []int{12}); len(bad) > 0 {
			return true
		}
	}
	return false
}

func TestLiteWorkspaceCheckedPartsReadsNestedPatchLeaves(t *testing.T) {
	leaf := "这周班里有 3 人没有交作业"
	patch := map[string]any{
		"body": map[string]any{
			"reading": leaf,
			"writing": "写作进度正常",
		},
		"sections": []any{"第一段", map[string]any{"note": "第二段"}},
	}
	parts := checkedPartsForTest(t, patch)

	for _, want := range []string{leaf, "写作进度正常", "第一段", "第二段"} {
		if !slices.Contains(parts, want) {
			t.Fatalf("parts = %q, want the leaf %q as its own part", parts, want)
		}
	}
	// Per leaf: the nested sentence alone carries a head count nobody
	// grounded, and the handler's per-part loop fails the turn on it.
	if !countCheckFails(parts) {
		t.Fatalf("no part fails the head-count check; parts = %q", parts)
	}
}

func TestLiteWorkspaceCheckedPartsSkipsOnlyTheTopLevelText(t *testing.T) {
	pasted := "原文：全校 1200 人参加了活动"
	nested := "修改后：3 人缺席"
	patch := map[string]any{
		"text":  pasted,
		"title": "阅读作业",
		"body":  map[string]any{"text": nested},
	}
	parts := checkedPartsForTest(t, patch)

	if slices.Contains(parts, pasted) {
		t.Fatalf("parts = %q, the top-level text (proven verbatim by setMaterial) must be skipped", parts)
	}
	if !slices.Contains(parts, "阅读作业") {
		t.Fatalf("parts = %q, want the title", parts)
	}
	if !slices.Contains(parts, nested) {
		t.Fatalf("parts = %q, a nested key named text is not the pasted article and must be checked", parts)
	}
}

// TestLiteWorkspaceCheckedPartsReadsTypedGoValues — a surface may build its
// patch from typed Go values. Each shape below once fell through the walk
// unread; each carries an ungrounded head count that must fail the check.
func TestLiteWorkspaceCheckedPartsReadsTypedGoValues(t *testing.T) {
	type section struct {
		Name string `json:"name"`
		Body string `json:"body"`
	}
	leaf := "有 3 人本周没有登录"
	cases := map[string]any{
		"slice of maps":   []map[string]any{{"body": leaf}},
		"map of slices":   map[string][]string{"reading": {leaf}},
		"struct":          section{Name: "阅读", Body: leaf},
		"pointer struct":  &section{Name: "阅读", Body: leaf},
		"slice of struct": []section{{Name: "阅读", Body: leaf}},
	}
	for name, value := range cases {
		t.Run(name, func(t *testing.T) {
			parts := checkedPartsForTest(t, map[string]any{"body": value})
			if !slices.Contains(parts, leaf) {
				t.Fatalf("parts = %q, want %q", parts, leaf)
			}
			if !countCheckFails(parts) {
				t.Fatalf("no part fails the head-count check; parts = %q", parts)
			}
		})
	}
}

// TestLiteWorkspaceCheckedPartsSkipsNonStringLeaves — tier 3 is not a claim
// about 3 people, and must not become the part "3".
func TestLiteWorkspaceCheckedPartsSkipsNonStringLeaves(t *testing.T) {
	parts := checkedPartsForTest(t, map[string]any{
		"tier": 3, "hidden": true, "slug": nil, "nested": map[string]any{"n": 3},
	})
	if len(parts) != 1 {
		t.Fatalf("parts = %q, want only the reply", parts)
	}
}

func TestLiteWorkspaceCheckedPartsFailsOnAnUnmarshallablePatch(t *testing.T) {
	_, err := liteWorkspaceCheckedParts("好的。", nil, map[string]any{"body": make(chan int)})
	if err == nil {
		t.Fatal("an unmarshallable patch was walked without an error")
	}
}

func TestLiteWorkspaceCheckedPartsOrderIsStable(t *testing.T) {
	patch := map[string]any{}
	for _, k := range strings.Split("a b c d e f g h i j k l", " ") {
		patch[k] = map[string]any{"x": k + "1", "y": k + "2", "z": []any{k + "3"}}
	}
	first := checkedPartsForTest(t, patch)
	for i := 0; i < 20; i++ {
		if got := checkedPartsForTest(t, patch); !slices.Equal(got, first) {
			t.Fatalf("run %d: parts = %q, want %q", i, got, first)
		}
	}
	if first[1] != "a1" || first[len(first)-1] != "l3" {
		t.Fatalf("parts = %q, want keys walked in sorted order", first)
	}
}
