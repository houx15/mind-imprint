package api

// writing_outline_internal_test.go — pure-function unit test for
// validateWritingOutlineArrayLengths (no harness/DB needed), following the
// internal-test convention used by reading_brief_internal_test.go /
// teacher_read_internal_test.go for unexported symbols.

import (
	"net/http"
	"testing"

	"mindimprint/api/internal/httpx"
)

// TestValidateWritingOutlineArrayLengths_MismatchReturns400 — the Task 1
// review's hard guard, exercised directly: buildWritingOutlineArrays can
// never actually produce mismatched slices in production (all three come
// from one loop over one source slice), so this white-box test proves the
// 400 path itself works, independent of whether production code can
// currently trigger it — the guard against ReplaceWritingOutline's raw NOT
// NULL constraint violation surfacing as an opaque 500.
func TestValidateWritingOutlineArrayLengths_MismatchReturns400(t *testing.T) {
	texts := []string{"a", "b"}
	depths := []int32{0}
	positions := []int32{0, 1}

	err := validateWritingOutlineArrayLengths(texts, depths, positions)
	if err == nil {
		t.Fatalf("mismatched array lengths must be rejected, got nil error")
	}
	apiErr, ok := err.(*httpx.APIError)
	if !ok {
		t.Fatalf("error = %T, want *httpx.APIError", err)
	}
	if apiErr.Status != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", apiErr.Status)
	}
}

// TestValidateWritingOutlineArrayLengths_MatchedIsNil — the ordinary case:
// equal-length arrays (as buildWritingOutlineArrays always produces) pass.
func TestValidateWritingOutlineArrayLengths_MatchedIsNil(t *testing.T) {
	texts := []string{"a", "b"}
	depths := []int32{0, 1}
	positions := []int32{0, 1}
	if err := validateWritingOutlineArrayLengths(texts, depths, positions); err != nil {
		t.Fatalf("matched lengths must pass, got %v", err)
	}
}

// TestBuildWritingOutlineArrays_AlwaysProducesEqualLengths — proves the
// "equal by construction" claim in buildWritingOutlineArrays' doc comment
// directly, across a few sizes including zero.
func TestBuildWritingOutlineArrays_AlwaysProducesEqualLengths(t *testing.T) {
	for _, n := range []int{0, 1, 5} {
		items := make([]writingOutlineItemReq, n)
		for i := range items {
			items[i] = writingOutlineItemReq{Text: "x", Depth: int32(i)}
		}
		texts, depths, positions := buildWritingOutlineArrays(items)
		if len(texts) != n || len(depths) != n || len(positions) != n {
			t.Fatalf("n=%d: got lengths %d/%d/%d", n, len(texts), len(depths), len(positions))
		}
		if err := validateWritingOutlineArrayLengths(texts, depths, positions); err != nil {
			t.Fatalf("n=%d: buildWritingOutlineArrays' own output failed its own guard: %v", n, err)
		}
	}
}
