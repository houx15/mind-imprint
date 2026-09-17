package liteworkspace

import (
	"strings"
	"testing"
	"time"
)

func TestCalendar(t *testing.T) {
	thu := time.Date(2026, 9, 17, 9, 0, 0, 0, BeijingOffset)
	c := Calendar(thu)
	lines := strings.Split(c, "\n")
	if len(lines) != CalendarDays {
		t.Fatalf("lines = %d", len(lines))
	}
	if lines[0] != "- 9月17日 本周周四（2026-09-17，今天）" || lines[1] != "- 9月18日 本周周五（2026-09-18，明天）" {
		t.Fatalf("calendar head = %q / %q", lines[0], lines[1])
	}
	if lines[3] != "- 9月20日 本周周日（2026-09-20）" {
		t.Fatalf("calendar line 3 = %q", lines[3])
	}
	// 下周三 is 9月23日, not 9月30日.
	if lines[6] != "- 9月23日 下周周三（2026-09-23）" || lines[13] != "- 9月30日 下下周周三（2026-09-30）" {
		t.Fatalf("calendar weeks = %q / %q", lines[6], lines[13])
	}
}

func TestDueWeekdayMismatch(t *testing.T) {
	ask := []string{"这周请全班读一篇关于用 AI 学数学的文章，周五晚上九点前完成。", "标题、截止时间你帮我定吧。"}
	// Live, 2026-09-17 (Thursday): 周五 → 9月20日, a Sunday.
	if got := DueWeekdayMismatch(ask, "2026-09-20T21:00"); !strings.Contains(got, "老师说的是周五") || !strings.Contains(got, "是周日") {
		t.Fatalf("Sunday for 周五 = %q", got)
	}
	if got := DueWeekdayMismatch(ask, "2026-09-18T21:00"); got != "" {
		t.Fatalf("Friday for 周五 = %q", got)
	}
	// 下周三 is still a Wednesday.
	if got := DueWeekdayMismatch([]string{"下周三交"}, "2026-09-23T18:00"); got != "" {
		t.Fatalf("下周三 = %q", got)
	}
	// The last weekday she named counts.
	if got := DueWeekdayMismatch([]string{"周五交", "改成星期一吧"}, "2026-09-21T21:00"); got != "" {
		t.Fatalf("changed to 星期一 = %q", got)
	}
	// An explicit date after the weekday is her choice.
	if got := DueWeekdayMismatch([]string{"周五交", "算了，就 20 号吧"}, "2026-09-20T21:00"); got != "" {
		t.Fatalf("explicit date = %q", got)
	}
	// No weekday, nothing to check; an unreadable due, nothing to check.
	if DueWeekdayMismatch([]string{"下周交"}, "2026-09-20T21:00") != "" || DueWeekdayMismatch(ask, "周五") != "" {
		t.Fatal("expected no mismatch")
	}
}
