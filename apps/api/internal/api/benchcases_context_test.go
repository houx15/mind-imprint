package api

import (
	"strings"
	"testing"
)

// Bad fixture statuses, duplicate zero IDs and chat-provider role names all
// silently erase or mislabel the very state the routing benchmark measures.
func TestReadingBenchmarkContainsOneCurrentStepAndHistory(t *testing.T) {
	c := readingCoachCase()
	user := c.Request.Messages[1].Content
	if strings.Count(user, "← **她现在在这一步**") != 1 {
		t.Fatal("benchmark must expose exactly one current task")
	}
	if strings.Contains(user, "所有步骤都走完了") {
		t.Fatal("benchmark lost its pending task")
	}
	if !strings.Contains(user, "(focus_block)") || !strings.Contains(user, "(reflect)") {
		t.Fatal("benchmark must use registered task kinds")
	}
	for _, turn := range []string{"你：先通读一遍", "她：他想说中国在可再生能源上投了很多钱"} {
		if !strings.Contains(user, turn) {
			t.Fatal("benchmark history disappeared during production context assembly")
		}
	}
}
