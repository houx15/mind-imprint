package teacher

import (
	"fmt"
	"sort"

	"mindimprint/api/internal/agent"
)

// StudentWeek is one student's completed week as the activity rules see it:
// usage this window and the same class's prior window, report counts, and the
// project id of their latest ready report (for the "看能力报告" link). No axis,
// no agent.Report — the teacher path no longer reads the retired evaluations.
type StudentWeek struct {
	UserID, DisplayName, AvatarColor  string
	ActiveDays, Turns, PrevActiveDays int
	ReportsThisWeek, PriorReports     int
	LatestReportProjectID             string // "" when the student has no ready report
}

// Card is one 值得表扬 / 需要建议 card. Evidence is ALWAYS deterministic — a bare
// statement of the week's numbers. Lead/action (wording) are added later by the
// composer and are not fields here.
type Card struct {
	UserID, DisplayName, AvatarColor string
	TagCode, TagLabel, Kind          string
	Evidence                         string
	HasReport                        bool
	ReportScopeID                    string // project id; surface is always "project"
}

type Weekly struct {
	Praise, Watch []Card
}

// Detect is the whole judgment layer: activity rules decide who appears, with
// which tag and which numeric evidence. A student carries at most one card;
// watch precedes praise.
func Detect(students []StudentWeek) Weekly {
	var w Weekly
	for _, s := range students {
		if c, ok := watchCard(s); ok {
			w.Watch = append(w.Watch, c)
		} else if c, ok := praiseCard(s); ok {
			w.Praise = append(w.Praise, c)
		}
	}
	return w
}

func mkCard(s StudentWeek, kind, code, label, evidence string) Card {
	return Card{
		UserID: s.UserID, DisplayName: s.DisplayName, AvatarColor: s.AvatarColor,
		TagCode: code, TagLabel: label, Kind: kind, Evidence: evidence,
		HasReport: s.LatestReportProjectID != "", ReportScopeID: s.LatestReportProjectID,
	}
}

// watchCard: severity order, first match wins.
func watchCard(s StudentWeek) (Card, bool) {
	if s.ActiveDays == 0 {
		return mkCard(s, "watch", "never_used", "本周未使用",
			fmt.Sprintf("本周 0 天活动记录；上周 %d 天。", s.PrevActiveDays)), true
	}
	if s.PrevActiveDays >= 2 && s.ActiveDays <= s.PrevActiveDays-2 {
		return mkCard(s, "watch", "dropped_off", "本周掉线",
			fmt.Sprintf("活跃天数 上周 %d 天 → 本周 %d 天。", s.PrevActiveDays, s.ActiveDays)), true
	}
	if s.Turns > 0 && s.ReportsThisWeek == 0 {
		return mkCard(s, "watch", "stuck_no_output", "有对话没产出",
			fmt.Sprintf("本周 %d 轮对话，但没有完成能力报告。", s.Turns)), true
	}
	return Card{}, false
}

// praiseCard: milestone first, then output. Praise is finished-thinking only
// (完成了能力报告) — never raw AI-usage or turn-count intensity, which would
// reward engagement for its own sake and violate 铁律②（不操纵）.
func praiseCard(s StudentWeek) (Card, bool) {
	if s.ReportsThisWeek > 0 {
		if s.PriorReports == 0 {
			return mkCard(s, "praise", "first_report", "第一份报告",
				"本周完成了第一份能力报告。"), true
		}
		return mkCard(s, "praise", "produced_report", "有产出",
			fmt.Sprintf("本周完成 %d 份能力报告。", s.ReportsThisWeek)), true
	}
	return Card{}, false
}

// ClassWeekCounts is one window's class-wide counters.
type ClassWeekCounts struct{ ActiveStudents, Reports, Turns, CourseSteps int }

// Stat is one of the four cards at the top of the weekly report.
type Stat struct {
	Key, Label string
	Value      int
	Unit, Foot string
	Delta      string
	DeltaDir   string
}

// Stats renders the four cards. An unchanged metric renders ±0 as flat.
func Stats(cur, prev ClassWeekCounts, classSize int) []Stat {
	mk := func(key, label string, value, prior int, unit, foot string) Stat {
		d := value - prior
		s := Stat{Key: key, Label: label, Value: value, Unit: unit, Foot: foot}
		switch {
		case d > 0:
			s.Delta, s.DeltaDir = fmt.Sprintf("+%d", d), "up"
		case d < 0:
			s.Delta, s.DeltaDir = fmt.Sprintf("%d", d), "down"
		default:
			s.Delta, s.DeltaDir = "±0", "flat"
		}
		return s
	}
	return []Stat{
		mk("active_students", "本周活跃学生", cur.ActiveStudents, prev.ActiveStudents, fmt.Sprintf("/ %d 人", classSize), "登录并有活动的学生"),
		mk("reports", "生成能力报告", cur.Reports, prev.Reports, "份", "完成并生成过程评估的项目"),
		mk("turns", "AI 对话轮次", cur.Turns, prev.Turns, "轮", "反映本周使用强度"),
		mk("course_steps", "完成课程节", cur.CourseSteps, prev.CourseSteps, "节", "平台内自学课程"),
	}
}

// BuildWeeklyFacts is the ONLY thing the composer sees: the class identity and
// the deterministically-selected cards with their numeric evidence. It omits
// class-level usage counters (they keep ticking; prose is written once).
func BuildWeeklyFacts(className string, classSize int, weekLabel string, w Weekly) agent.WeeklyFacts {
	facts := agent.WeeklyFacts{ClassName: className, ClassSize: classSize, WeekLabel: weekLabel}
	for _, c := range append(append([]Card{}, w.Praise...), w.Watch...) {
		facts.Cards = append(facts.Cards, agent.WeeklyFactCard{
			UserID: c.UserID, Name: c.DisplayName, Kind: c.Kind,
			TagCode: c.TagCode, TagLabel: c.TagLabel, Evidence: c.Evidence,
		})
	}
	sort.SliceStable(facts.Cards, func(i, j int) bool { return facts.Cards[i].Kind < facts.Cards[j].Kind })
	return facts
}
