package api

// reading_brief_internal_test.go — pure-function unit test for
// suggestReadingReason (no harness/DB needed), following the internal-test
// convention used by cards_test.go/teacher_read_internal_test.go for
// unexported symbols.

import (
	"strings"
	"testing"
)

func TestSuggestReadingReason_Templated(t *testing.T) {
	got := suggestReadingReason("研究中国是否让地球更可持续", "NASA 全球碳排放报告")
	if got == "" || !strings.Contains(got, "NASA") {
		t.Fatalf("suggested reason should reference the source title: %q", got)
	}
}
