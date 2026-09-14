package agent

import (
	"fmt"
	"strings"
	"testing"
)

// 走查给陪练看的上文必须和生产一样多。多一条，陪练就显得比生产里更记得住；
// 少一条，就把一个生产里记得住的陪练记成原地打转。两种错读起来都像真结论。

func numbered(n int) []ChatTurn {
	h := make([]ChatTurn, n)
	for i := range h {
		role := "user"
		if i%2 == 0 {
			role = "assistant"
		}
		h[i] = ChatTurn{Role: role, Content: fmt.Sprintf("消息%02d", i)}
	}
	return h
}

func TestWindowWritingHistoryKeepsTwelvePriorPlusHerNewest(t *testing.T) {
	h := numbered(20) // 19 条旧消息 + 她最新的第 20 条
	got := windowWritingHistory(h)
	if len(got) != writingWalkWindow+1 {
		t.Fatalf("len = %d, want %d（12 条旧消息 + 她最新那一句）", len(got), writingWalkWindow+1)
	}
	if got[len(got)-1].Content != "消息19" {
		t.Errorf("最后一条必须是她最新说的那一句，实际 %q", got[len(got)-1].Content)
	}
	// 旧消息里留下的是最近的 12 条：消息07 … 消息18。
	if got[0].Content != "消息07" {
		t.Errorf("窗口起点 = %q, want 消息07", got[0].Content)
	}
}

func TestWindowWritingHistoryLeavesShortHistoryAlone(t *testing.T) {
	h := numbered(5)
	got := windowWritingHistory(h)
	if len(got) != 5 || got[0].Content != "消息00" || got[4].Content != "消息04" {
		t.Fatalf("短上文不该被截：%+v", got)
	}
	if out := windowWritingHistory(nil); len(out) != 0 {
		t.Fatalf("空上文应当原样返回：%+v", out)
	}
}

func TestProWalkDriverShowsTheCoachOnlyTheLastTwelveTurns(t *testing.T) {
	d := NewProWalkDriver()
	d.history = numbered(20)
	var user string
	for _, m := range d.Request().Messages {
		if m.Role == "user" {
			user = m.Content
		}
	}
	// 生产是 LoadChatHistory(…, 12)：最后 12 条，即 消息08 … 消息19。
	for _, want := range []string{"消息08", "消息19"} {
		if !strings.Contains(user, want) {
			t.Errorf("窗口里应当有 %s", want)
		}
	}
	if strings.Contains(user, "消息07") {
		t.Error("第 13 条之前的上文不该出现：生产里陪练看不到它")
	}
}
