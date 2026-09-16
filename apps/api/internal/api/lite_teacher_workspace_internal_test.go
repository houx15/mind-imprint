package api

import (
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
