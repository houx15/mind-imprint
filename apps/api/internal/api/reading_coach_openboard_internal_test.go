package api

import (
	"testing"

	"mindimprint/api/internal/store/sqlc"
)

// openBoard 说的是「她屏幕上现在摆着一块没交的板」。屏幕上摆着什么，由**最新的
// 那张卡**决定：一块板后面又来了一张卡，板就折起来了（CoachCard 的 stale）。
func TestOpenBoardIsOnlyTheNewestCard(t *testing.T) {
	aiCard := func(typ string) sqlc.AtomMessage {
		return sqlc.AtomMessage{Role: "ai", Payload: []byte(`{"card":{"type":"` + typ + `","prompt":"p"}}`)}
	}
	said := sqlc.AtomMessage{Role: "student", Content: "我不知道先动哪一张"}
	aiWords := sqlc.AtomMessage{Role: "ai", Content: "把下面这几张卡片拖到位置上。"}
	submitted := sqlc.AtomMessage{Role: "student", Payload: coachCardAnswerPayload(
		&coachCardAnswer{Type: coachCardOrderEvents, Prompt: "p", Choice: "第1：\nx"})}

	cases := []struct {
		name string
		msgs []sqlc.AtomMessage
		open bool
	}{
		{"板刚发出去", []sqlc.AtomMessage{aiCard(coachCardOrderEvents)}, true},
		// 她说了几句、印记 只回了话 —— 板仍然是最新的那张卡，仍然摆着。
		{"之后只有说话", []sqlc.AtomMessage{aiCard(coachCardLabelRoles), said, aiWords, said}, true},
		{"交了", []sqlc.AtomMessage{aiCard(coachCardOrderEvents), submitted}, false},
		// 🚨 2026-09-18 议论文复走那一幕的反面：板后面来过一张卡，板折起来了。
		{"后面又来一张卡", []sqlc.AtomMessage{aiCard(coachCardLabelRoles), said, aiCard(coachCardShortText)}, false},
		{"从来没有板", []sqlc.AtomMessage{aiCard(coachCardShortText), said}, false},
	}
	for _, c := range cases {
		if got := openBoard(c.msgs) != nil; got != c.open {
			t.Errorf("%s: openBoard 开着 = %v，想要 %v", c.name, got, c.open)
		}
	}
}
