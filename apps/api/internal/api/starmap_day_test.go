package api

import (
	"testing"
	"time"
)

// starmap_day_test.go —— 星图的「今天」在哪一刻翻页。
//
// 🚨 2026-09-11 在生产上量到的：api 跑在 distroless 镜像里，镜像没有 tzdata，
// compose 也没设 TZ，于是 `time.Local` 是 UTC。星图的「今天」因此是一个 UTC 日，
// **早上八点才翻页** —— 学生凌晨到八点之间看到的是昨天那五颗星，而屏幕上写着
// 「今日探索地图」。
//
// 这条判断读代码看不出对错（`time.Now().In(time.Local)` 长得完全正常），而它判
// 错的样子不是报错，是一屏过期的新闻。所以它值一个纯函数加一组边界。

func TestStarmapDayTurnsOverAtMidnightInChinaNotUTC(t *testing.T) {
	cst := time.FixedZone("CST", 8*60*60)

	cases := []struct {
		name string
		at   time.Time
		want string
	}{
		{
			// 这一刻正是那个 bug：北京 00:30 已经是 11 号，而 UTC 还是 10 号。
			name: "北京 9/11 00:30（UTC 仍是 9/10 16:30）",
			at:   time.Date(2026, 9, 11, 0, 30, 0, 0, cst),
			want: "2026-09-11",
		},
		{
			name: "北京 9/11 07:59（旧实现要等到八点才翻页）",
			at:   time.Date(2026, 9, 11, 7, 59, 0, 0, cst),
			want: "2026-09-11",
		},
		{
			name: "北京 9/10 23:59，还是 10 号",
			at:   time.Date(2026, 9, 10, 23, 59, 0, 0, cst),
			want: "2026-09-10",
		},
		{
			// 传进来的时刻带什么时区都不该影响结果 —— 折算按学生的时区来。
			name: "同一刻用 UTC 表示，答案不变",
			at:   time.Date(2026, 9, 10, 16, 30, 0, 0, time.UTC),
			want: "2026-09-11",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := starmapDay(c.at)
			if !got.Valid {
				t.Fatal("date 无效")
			}
			if s := got.Time.Format("2006-01-02"); s != c.want {
				t.Errorf("starmapDay(%s) = %s，want %s", c.at.Format(time.RFC3339), s, c.want)
			}
		})
	}
}

// 存进库的那个值必须是「零点、UTC」—— pgtype.Date 只取日期部分，带上偏移会让
// 同一天在库里出现两种表示。
func TestStarmapDayIsAMidnightUTCDate(t *testing.T) {
	got := starmapDay(time.Date(2026, 9, 11, 15, 4, 5, 0, time.FixedZone("CST", 8*60*60)))
	if h, m, s := got.Time.Clock(); h != 0 || m != 0 || s != 0 {
		t.Errorf("时间部分不是零点：%v", got.Time)
	}
	if got.Time.Location() != time.UTC {
		t.Errorf("不是 UTC：%v", got.Time.Location())
	}
}
