package api

import (
	"encoding/json"
	"strings"
	"testing"

	"mindimprint/api/internal/liteassign"
)

// TestLiteWorkspaceClampsToWhatPublishAccepts — the clamps a tool applies must
// be the caps the publish path enforces, field by field.
//
// The two differ by an order of magnitude (200 for the title, 2000 for the
// instructions), and getting it wrong is invisible until the last step: the
// card shows a title we wrote ourselves and 发布作业 then refuses it with
// 「请填写作业标题，不超过 200 字」. This test feeds the clamped value straight
// into the real validators, so it fails if either cap drifts.
func TestLiteWorkspaceClampsToWhatPublishAccepts(t *testing.T) {
	long := strings.Repeat("气", 5000)

	title := liteWorkspaceClampRunes(long, maxAssignmentTitleRunes)
	if _, err := parseAssignmentTitle(title); err != nil {
		t.Fatalf("a clamped title is rejected at publish: %v", err)
	}

	ins := liteWorkspaceClampRunes(long, liteWorkspaceMaxInstructionsRunes)
	if _, err := liteassign.ValidateInstructions(ins); err != nil {
		t.Fatalf("clamped instructions are rejected at publish: %v", err)
	}

	// The title cap is the smaller of the two. If someone ever raises it to
	// the instructions cap, the assertion above stops meaning anything, so
	// pin the relationship too.
	if maxAssignmentTitleRunes >= liteWorkspaceMaxInstructionsRunes {
		t.Fatalf("title cap %d is no longer smaller than the instructions cap %d",
			maxAssignmentTitleRunes, liteWorkspaceMaxInstructionsRunes)
	}
}

// TestLiteWorkspaceCardStateIsAllChinese pins spec §12.1: the text that goes
// into the model's system prompt must carry only the words the teacher
// already knows from the assignment form, never a wire value. Production once
// put 「种类：reading」「材料来源：library」「文章 slug：biden-creates-climate-corps」
// in front of the model, and it read them back to her.
func TestLiteWorkspaceCardStateIsAllChinese(t *testing.T) {
	tier := 3
	raw, err := json.Marshal(liteWorkspaceArtifact{
		Kind:          "reading",
		Title:         "美国气候队",
		ReadingSource: "library",
		Slug:          "biden-creates-climate-corps",
		Tier:          &tier,
	})
	if err != nil {
		t.Fatalf("marshal artifact: %v", err)
	}
	got := liteWorkspaceCardState(raw, nil)

	for _, wire := range []string{"reading", "library", "biden-creates-climate-corps"} {
		if strings.Contains(got, wire) {
			t.Fatalf("card state still carries the wire value %q: %q", wire, got)
		}
	}
	for _, chinese := range []string{"类型：阅读", "材料来源：分级阅读库", "文章：《美国气候队》", "难度：进阶"} {
		if !strings.Contains(got, chinese) {
			t.Fatalf("card state = %q, want it to contain %q", got, chinese)
		}
	}
}

// TestLiteWorkspaceCardStateUnknownSlug — an article the catalogue does not
// carry must not fall back to showing the slug: the teacher would see the
// same raw value the lookup was added to hide.
func TestLiteWorkspaceCardStateUnknownSlug(t *testing.T) {
	raw, err := json.Marshal(liteWorkspaceArtifact{Kind: "reading", ReadingSource: "library", Slug: "not-a-real-article"})
	if err != nil {
		t.Fatalf("marshal artifact: %v", err)
	}
	got := liteWorkspaceCardState(raw, nil)

	if strings.Contains(got, "not-a-real-article") {
		t.Fatalf("card state leaked an unknown slug: %q", got)
	}
	if !strings.Contains(got, "文章：未找到") {
		t.Fatalf("card state = %q, want 文章：未找到", got)
	}
}
