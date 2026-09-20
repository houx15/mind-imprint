package api

import (
	"strings"
	"testing"

	"mindimprint/api/internal/store/sqlc"
)

// 这两处原来没有上限：一次长写作的全部消息都进 prompt。值得测的不是「渲染出了
// 几行」，而是两条读代码看不出来的不变量：**有界**，且**一个字都不切**。

func studentMsgs(n int, runesEach int) []sqlc.AtomMessage {
	out := make([]sqlc.AtomMessage, 0, n)
	for i := 0; i < n; i++ {
		out = append(out, sqlc.AtomMessage{
			Seq:     int32(i + 1),
			Role:    "student",
			Content: strings.Repeat("字", runesEach),
		})
	}
	return out
}

func TestRecentStudentSaidIsBounded(t *testing.T) {
	// 200 messages × 500 runes = 100,000 runes if nothing bounds it.
	got := recentStudentSaid(studentMsgs(200, 500))
	total := 0
	for _, s := range got {
		total += len([]rune(s))
	}
	if total > writingGuideSaidRuneBudget {
		t.Fatalf("unbounded: kept %d runes, budget is %d", total, writingGuideSaidRuneBudget)
	}
	if len(got) == 0 {
		t.Fatal("bounded to nothing — the budget must keep her most recent words")
	}
}

// 🚨 丢掉整条旧消息可以，切断一句不行。
func TestRecentStudentSaidNeverCutsASentence(t *testing.T) {
	msgs := studentMsgs(50, 500)
	for _, s := range recentStudentSaid(msgs) {
		if len([]rune(s)) != 500 {
			t.Fatalf("a kept line was cut: %d runes, expected the whole 500", len([]rune(s)))
		}
		if strings.Contains(s, "…") {
			t.Fatalf("a kept line carries an ellipsis — her words were truncated: %q", s)
		}
	}
}

// 一条超预算的消息也要留下来：她刚说的那句话永远不能整条消失。
func TestRecentStudentSaidKeepsOneOversizeMessage(t *testing.T) {
	got := recentStudentSaid(studentMsgs(1, writingGuideSaidRuneBudget*3))
	if len(got) != 1 {
		t.Fatalf("her only message was dropped: got %d lines", len(got))
	}
}

func TestRecentStudentSaidKeepsHerOrder(t *testing.T) {
	msgs := []sqlc.AtomMessage{
		{Seq: 1, Role: "student", Content: "第一句"},
		{Seq: 2, Role: "ai", Content: "印记说的话，不该出现"},
		{Seq: 3, Role: "student", Content: "第二句"},
	}
	got := recentStudentSaid(msgs)
	if len(got) != 2 || got[0] != "第一句" || got[1] != "第二句" {
		t.Fatalf("order or filtering wrong: %#v", got)
	}
}

func TestBlockThreadIsWindowed(t *testing.T) {
	msgs := make([]sqlc.AtomMessage, 0, 100)
	for i := 0; i < 100; i++ {
		role := "student"
		if i%2 == 1 {
			role = "ai"
		}
		msgs = append(msgs, sqlc.AtomMessage{Seq: int32(i + 1), Role: role, Content: "x"})
	}
	if got := len(blockThreadToChatMessages(msgs)); got > deepenTurnsWindow {
		t.Fatalf("block thread unwindowed: %d turns, window is %d", got, deepenTurnsWindow)
	}
}
