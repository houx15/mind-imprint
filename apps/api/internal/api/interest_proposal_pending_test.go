package api

// interest_proposal_pending_test.go —— 报告那一节该不该说「还在跑」。
//
// 盖章和词落库之间隔着一次模型调用。2026-09-08 的全链路走查里，她那一页问的
// 唯一一次正好落在这个窗口中间 —— 拿到「不 pending 且零条」，于是整节不显示、
// 也不再问第二次，而那个词就躺在库里。逐毫秒的证据见 harvestStillRunning 上面
// 的注释。
//
// 🚨 两头都要守住：窗口里要说「还在跑」，窗口过了要老实承认没有词。只守一头
// 的话，一篇真的采不出词的薄阅读会在那一节上永远转圈 —— 那正是 pending 这个
// 字段当初被单独拎出来的理由。

import (
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
)

func TestHarvestStillRunning(t *testing.T) {
	now := time.Now()
	cases := []struct {
		name    string
		stamped pgtype.Timestamptz
		found   int
		want    bool
	}{
		{"还没有人开始采", pgtype.Timestamptz{}, 0, true},
		{"刚盖章，词还没落库", pgtype.Timestamptz{Time: now.Add(-time.Second), Valid: true}, 0, true},
		{"刚盖章，词已经落库", pgtype.Timestamptz{Time: now.Add(-time.Second), Valid: true}, 2, false},
		{"很久以前采过，确实一个词都没有", pgtype.Timestamptz{Time: now.Add(-time.Hour), Valid: true}, 0, false},
		{"很久以前采过，有词", pgtype.Timestamptz{Time: now.Add(-time.Hour), Valid: true}, 3, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := harvestStillRunning(c.stamped, c.found); got != c.want {
				t.Errorf("harvestStillRunning = %v, want %v", got, c.want)
			}
		})
	}
}
