package liteweekly

import (
	"fmt"
	"strings"
)

// ClassWeekStats holds the class-level numbers of one completed week. The
// class weekly prompt and the digit check of the class prose both read them
// through ClassFactsText, so the model can cite exactly these numbers and no
// others.
type ClassWeekStats struct {
	ClassSize, ActiveStudents, Minutes, Turns, Finished int
	AssignmentRate                                      int // integer percent; -1 when no assignments were due
}

// kindLabel maps a card kind to the teacher-facing category name.
func kindLabel(kind string) string {
	if kind == "praise" {
		return "值得表扬"
	}
	return "需要建议"
}

// ClassFactsText renders the class name, the week label, every number in
// stats, and one line per card of each flagged student (name, category,
// label, evidence). It is used verbatim in the class prompt and as
// ProseCheck.FactsText for the class prose.
//
// Student user IDs are deliberately left out: they can contain digits, and
// anything in this text widens the set of digits the prose may use.
func ClassFactsText(className, weekLabel string, stats ClassWeekStats, flagged []StudentWeek, cards map[string][]Card) string {
	var b strings.Builder
	fmt.Fprintf(&b, "班级：%s\n", className)
	fmt.Fprintf(&b, "周次：%s\n", weekLabel)
	fmt.Fprintf(&b, "班级人数 %d 人；活跃学生 %d 人；学习 %d 分钟；对话 %d 轮；完成 %d 项；",
		stats.ClassSize, stats.ActiveStudents, stats.Minutes, stats.Turns, stats.Finished)
	if stats.AssignmentRate < 0 {
		b.WriteString("作业完成率 无到期作业\n")
	} else {
		fmt.Fprintf(&b, "作业完成率 %d%%\n", stats.AssignmentRate)
	}

	b.WriteString("卡片：")
	n := 0
	for _, s := range flagged {
		for _, c := range cards[s.UserID] {
			fmt.Fprintf(&b, "\n- %s · %s · %s · %s", s.Name, kindLabel(c.Kind), c.Label, c.Evidence)
			n++
		}
	}
	if n == 0 {
		b.WriteString("无")
	}
	return b.String()
}
