package api

// explore_retry_internal_test.go —— 「这一天还该不该再试一次生成」。
//
// 这是星图那条路上唯一读代码看不出对错的判断，而它判错的两种方式都很贵：
// 判紧了，一次 503 就锁死一整天的探索面（2026-09-04 走查里六次启动撞上两次）；
// 判松了，一个坏掉的源会被每个打开星图的学生反复抓十二遍。

import (
	"testing"
	"time"

	"mindimprint/api/internal/store/sqlc"
)

func TestStarmapRetryable(t *testing.T) {
	now := time.Date(2026, 9, 5, 10, 0, 0, 0, time.UTC)
	long := now.Add(-10 * time.Minute)
	just := now.Add(-5 * time.Second)

	cases := []struct {
		name string
		day  sqlc.NewsDay
		want bool
	}{
		{
			name: "出过星图的一天不再试",
			day:  sqlc.NewsDay{PlanetCount: 5, Attempts: 1, AttemptedAt: long},
			want: false,
		},
		{
			name: "失败过、冷却已过，再试",
			day:  sqlc.NewsDay{PlanetCount: 0, Attempts: 1, AttemptedAt: long},
			want: true,
		},
		{
			name: "刚刚才试过，等冷却",
			day:  sqlc.NewsDay{PlanetCount: 0, Attempts: 1, AttemptedAt: just},
			want: false,
		},
		{
			name: "试满了就不再试",
			day:  sqlc.NewsDay{PlanetCount: 0, Attempts: maxStarmapAttempts, AttemptedAt: long},
			want: false,
		},
		{
			name: "只差一次也还能试",
			day:  sqlc.NewsDay{PlanetCount: 0, Attempts: maxStarmapAttempts - 1, AttemptedAt: long},
			want: true,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := starmapRetryable(c.day, now); got != c.want {
				t.Errorf("starmapRetryable = %v, want %v", got, c.want)
			}
		})
	}
}

// 界面照这个数决定那个按钮的样子。返回错了的症状就是营地走查里那一条：
// 一个按下去什么都不发生的「重试」。
func TestStarmapRetryAfter(t *testing.T) {
	now := time.Date(2026, 9, 5, 10, 0, 0, 0, time.UTC)

	if got := starmapRetryAfter(sqlc.NewsDay{PlanetCount: 5}, now); got != -1 {
		t.Errorf("出过星图的一天 = %d, want -1", got)
	}
	if got := starmapRetryAfter(
		sqlc.NewsDay{Attempts: maxStarmapAttempts, AttemptedAt: now.Add(-time.Hour)}, now); got != -1 {
		t.Errorf("试满的一天 = %d, want -1", got)
	}
	if got := starmapRetryAfter(
		sqlc.NewsDay{Attempts: 1, AttemptedAt: now.Add(-time.Hour)}, now); got != 0 {
		t.Errorf("冷却已过 = %d, want 0", got)
	}
	// 刚试过 30 秒，还剩 60 秒；进一取整，所以是 61。
	got := starmapRetryAfter(sqlc.NewsDay{Attempts: 1, AttemptedAt: now.Add(-30 * time.Second)}, now)
	if got < 60 || got > 61 {
		t.Errorf("刚试过 30 秒 = %d, want 60 或 61", got)
	}
}
