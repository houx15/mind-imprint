package api

// lite_teacher_workspace_parts_internal_test.go — what the shared §6 checks
// read out of a patch. A surface whose patch nests its strings (the parent
// report's {"body": {"<section>": "<text>"}}) must not get past either check.

import (
	"slices"
	"testing"

	"mindimprint/api/internal/liteworkspace"
)

func TestLiteWorkspaceCheckedPartsReadsNestedPatchLeaves(t *testing.T) {
	leaf := "这周班里有 3 人没有交作业"
	patch := map[string]any{
		"body": map[string]any{
			"reading": leaf,
			"writing": "写作进度正常",
		},
		"sections": []any{"第一段", map[string]any{"note": "第二段"}},
	}
	parts := liteWorkspaceCheckedParts("好的。", nil, patch)

	for _, want := range []string{leaf, "写作进度正常", "第一段", "第二段"} {
		if !slices.Contains(parts, want) {
			t.Fatalf("parts = %q, want the leaf %q as its own part", parts, want)
		}
	}

	// Per leaf: the nested sentence alone carries a head count nobody
	// grounded, and the handler's per-part loop would fail the turn on it.
	failed := false
	for _, p := range parts {
		if bad := liteworkspace.UngroundedCounts(p, []int{12}); len(bad) > 0 {
			failed = true
		}
	}
	if !failed {
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
	parts := liteWorkspaceCheckedParts("好的。", nil, patch)

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
