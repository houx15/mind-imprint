package liteassign

import (
	"testing"
	"time"
)

func TestStatus(t *testing.T) {
	due := time.Date(2026, 9, 20, 22, 0, 0, 0, time.UTC)
	before := due.Add(-time.Hour)
	after := due.Add(time.Hour)
	at := func(t time.Time) *time.Time { return &t }

	cases := []struct {
		name       string
		started    bool
		finishedAt *time.Time
		now        time.Time
		want       string
	}{
		{"not started before due", false, nil, before, "not_started"},
		{"not started exactly at due", false, nil, due, "not_started"},
		{"not started after due", false, nil, after, "overdue"},
		{"started before due", true, nil, before, "in_progress"},
		{"started after due, unfinished", true, nil, after, "overdue"},
		{"finished exactly at due", true, at(due), after, "done"},
		{"finished one second late", true, at(due.Add(time.Second)), after, "done_late"},
		{"finished early, viewed later", true, at(before), after, "done"},
	}
	for _, c := range cases {
		if got := Status(c.started, c.finishedAt, due, c.now); got != c.want {
			t.Errorf("%s: got %s want %s", c.name, got, c.want)
		}
	}
}

func TestStatusLabel(t *testing.T) {
	want := map[string]string{"not_started": "未开始", "in_progress": "进行中", "done": "已完成", "done_late": "逾期完成", "overdue": "已逾期"}
	for k, v := range want {
		if StatusLabel(k) != v {
			t.Errorf("%s → %s, want %s", k, StatusLabel(k), v)
		}
	}
}
