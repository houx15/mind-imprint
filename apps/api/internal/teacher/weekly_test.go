package teacher_test

import (
	"strings"
	"testing"

	"mindimprint/api/internal/teacher"
)

func TestNeverUsedFiresOnZeroActiveDays(t *testing.T) {
	w := teacher.Detect([]teacher.StudentWeek{{
		UserID: "u1", DisplayName: "罗一", ActiveDays: 0, PrevActiveDays: 3,
	}})
	if len(w.Watch) != 1 || w.Watch[0].TagCode != "never_used" {
		t.Fatalf("watch = %+v; want one never_used card", w.Watch)
	}
	if w.Watch[0].TagLabel != "本周未使用" {
		t.Fatalf("label = %q; want 本周未使用", w.Watch[0].TagLabel)
	}
	if !strings.Contains(w.Watch[0].Evidence, "3 天") {
		t.Fatalf("evidence = %q; want last week's active-day count stated", w.Watch[0].Evidence)
	}
}

func TestDroppedOffNeedsATwoDayFall(t *testing.T) {
	w := teacher.Detect([]teacher.StudentWeek{{
		UserID: "u1", DisplayName: "陈屿", ActiveDays: 1, PrevActiveDays: 5,
	}})
	if len(w.Watch) != 1 || w.Watch[0].TagCode != "dropped_off" {
		t.Fatalf("watch = %+v; want dropped_off", w.Watch)
	}
	if !strings.Contains(w.Watch[0].Evidence, "5 天") || !strings.Contains(w.Watch[0].Evidence, "1 天") {
		t.Fatalf("evidence = %q; want both week counts stated", w.Watch[0].Evidence)
	}

	quiet := teacher.Detect([]teacher.StudentWeek{{
		UserID: "u1", DisplayName: "陈屿", ActiveDays: 4, PrevActiveDays: 5,
	}})
	if len(quiet.Watch) != 0 {
		t.Fatalf("watch = %+v; a one-day dip must not fire", quiet.Watch)
	}
}

func TestDroppedOffFiresExactlyAtTheTwoDayThreshold(t *testing.T) {
	w := teacher.Detect([]teacher.StudentWeek{{
		UserID: "u1", DisplayName: "陈屿", ActiveDays: 3, PrevActiveDays: 5,
	}})
	if len(w.Watch) != 1 || w.Watch[0].TagCode != "dropped_off" {
		t.Fatalf("watch = %+v; a fall of exactly 2 (5→3) must fire dropped_off", w.Watch)
	}
}

func TestStuckNoOutputFiresOnTurnsWithoutAReport(t *testing.T) {
	w := teacher.Detect([]teacher.StudentWeek{{
		UserID: "u1", DisplayName: "周子墨", ActiveDays: 4, PrevActiveDays: 4, Turns: 12, ReportsThisWeek: 0,
	}})
	if len(w.Watch) != 1 || w.Watch[0].TagCode != "stuck_no_output" {
		t.Fatalf("watch = %+v; want stuck_no_output", w.Watch)
	}
	if !strings.Contains(w.Watch[0].Evidence, "12") {
		t.Fatalf("evidence = %q; want the turn count stated", w.Watch[0].Evidence)
	}

	reported := teacher.Detect([]teacher.StudentWeek{{
		UserID: "u1", DisplayName: "周子墨", ActiveDays: 4, PrevActiveDays: 4, Turns: 12, ReportsThisWeek: 1,
	}})
	if len(reported.Watch) != 0 {
		t.Fatalf("watch = %+v; a student who produced a report this week is not stuck", reported.Watch)
	}

	silent := teacher.Detect([]teacher.StudentWeek{{
		UserID: "u1", DisplayName: "周子墨", ActiveDays: 4, PrevActiveDays: 4, Turns: 0, ReportsThisWeek: 0,
	}})
	if len(silent.Watch) != 0 {
		t.Fatalf("watch = %+v; zero turns is not 有对话没产出", silent.Watch)
	}
}

func TestWatchPrecedesPraiseWhenBothCouldApply(t *testing.T) {
	// dropped_off (watch) fires; this student also produced a report this
	// week, which would otherwise earn a praise card — watch must win.
	w := teacher.Detect([]teacher.StudentWeek{{
		UserID: "u1", DisplayName: "周子墨", ActiveDays: 1, PrevActiveDays: 5,
		ReportsThisWeek: 1, PriorReports: 1,
	}})
	if len(w.Praise) != 0 {
		t.Fatalf("praise = %+v; a student who also dropped off must not be filed as praise", w.Praise)
	}
	if len(w.Watch) != 1 || w.Watch[0].TagCode != "dropped_off" {
		t.Fatalf("watch = %+v; want dropped_off", w.Watch)
	}
}

func TestFirstReportFiresWhenPriorReportsIsZero(t *testing.T) {
	w := teacher.Detect([]teacher.StudentWeek{{
		UserID: "u1", DisplayName: "吴桐", ActiveDays: 5, PrevActiveDays: 5,
		ReportsThisWeek: 1, PriorReports: 0,
	}})
	if len(w.Praise) != 1 || w.Praise[0].TagCode != "first_report" {
		t.Fatalf("praise = %+v; want first_report", w.Praise)
	}
	if w.Praise[0].TagLabel != "第一份报告" {
		t.Fatalf("label = %q; want 第一份报告", w.Praise[0].TagLabel)
	}
	if w.Praise[0].Kind != "praise" {
		t.Fatalf("kind = %q; want praise", w.Praise[0].Kind)
	}
}

func TestProducedReportFiresWhenPriorReportsIsNonZero(t *testing.T) {
	w := teacher.Detect([]teacher.StudentWeek{{
		UserID: "u1", DisplayName: "吴桐", ActiveDays: 5, PrevActiveDays: 5,
		ReportsThisWeek: 2, PriorReports: 1,
	}})
	if len(w.Praise) != 1 || w.Praise[0].TagCode != "produced_report" {
		t.Fatalf("praise = %+v; want produced_report", w.Praise)
	}
	if !strings.Contains(w.Praise[0].Evidence, "2") {
		t.Fatalf("evidence = %q; want the report count stated", w.Praise[0].Evidence)
	}
}

// TestTurnCountAloneNeverEarnsPraise guards 铁律②（不操纵）: raw AI-usage /
// turn-count intensity must never be a praise signal on its own, even for a
// class-leading, high-volume student with no report. Praise is
// finished-thinking only (first_report / produced_report). This replaces the
// removed strong_engagement rule, which rewarded turn count and was also
// unreachable dead code (stuck_no_output's `Turns > 0 && ReportsThisWeek ==
// 0` is a strict superset of what strong_engagement needed).
func TestTurnCountAloneNeverEarnsPraise(t *testing.T) {
	w := teacher.Detect([]teacher.StudentWeek{
		{UserID: "a", DisplayName: "顶流", ActiveDays: 5, PrevActiveDays: 5, Turns: 50},
		{UserID: "b", DisplayName: "乙", ActiveDays: 5, PrevActiveDays: 5, Turns: 1},
	})
	if len(w.Praise) != 0 {
		t.Fatalf("praise = %+v; turn count alone must never earn praise", w.Praise)
	}
	if len(w.Watch) != 2 || w.Watch[0].TagCode != "stuck_no_output" {
		t.Fatalf("watch = %+v; want both students' cards to be stuck_no_output", w.Watch)
	}
}

func TestAtMostOneCardPerStudent(t *testing.T) {
	// A student satisfying multiple rules (never_used AND — hypothetically —
	// any praise condition) must appear in exactly one of Watch/Praise, never
	// both, and never twice within one.
	w := teacher.Detect([]teacher.StudentWeek{{
		UserID: "u1", DisplayName: "罗一", ActiveDays: 0, PrevActiveDays: 5,
		ReportsThisWeek: 1, PriorReports: 0, Turns: 50,
	}})
	total := len(w.Watch) + len(w.Praise)
	if total != 1 {
		t.Fatalf("total cards = %d; a student must carry at most one card", total)
	}
	if len(w.Watch) != 1 || w.Watch[0].TagCode != "never_used" {
		t.Fatalf("watch = %+v; want never_used to win over any praise condition", w.Watch)
	}
}

func TestUnflaggedStudentEarnsNoCard(t *testing.T) {
	// Turns must be 0: any nonzero Turns with ReportsThisWeek==0 fires
	// stuck_no_output (see TestTurnCountAloneNeverEarnsPraise).
	w := teacher.Detect([]teacher.StudentWeek{{
		UserID: "u1", DisplayName: "普通同学", ActiveDays: 4, PrevActiveDays: 4, Turns: 0, ReportsThisWeek: 0,
	}})
	if len(w.Watch) != 0 || len(w.Praise) != 0 {
		t.Fatalf("cards = %+v/%+v; an unremarkable week earns no card", w.Watch, w.Praise)
	}
}

func TestCardCarriesReportLinkFields(t *testing.T) {
	w := teacher.Detect([]teacher.StudentWeek{{
		UserID: "u1", DisplayName: "吴桐", ActiveDays: 5, PrevActiveDays: 5,
		ReportsThisWeek: 1, PriorReports: 1, LatestReportProjectID: "proj-123",
	}})
	if len(w.Praise) != 1 {
		t.Fatalf("praise = %+v; want one card", w.Praise)
	}
	if !w.Praise[0].HasReport || w.Praise[0].ReportScopeID != "proj-123" {
		t.Fatalf("card = %+v; want HasReport=true and ReportScopeID=proj-123", w.Praise[0])
	}

	noReport := teacher.Detect([]teacher.StudentWeek{{
		UserID: "u2", DisplayName: "新同学", ActiveDays: 0, PrevActiveDays: 0,
	}})
	if noReport.Watch[0].HasReport || noReport.Watch[0].ReportScopeID != "" {
		t.Fatalf("card = %+v; want HasReport=false and no scope id", noReport.Watch[0])
	}
}

// TestPraiseCardCarriesReportOverview: a praise card leads with the student's
// own 综述 (deterministic, lifted from the report), truncated on a sentence
// boundary. Watch cards never carry it — the substance line is about a produced
// report, not an inactivity note.
func TestPraiseCardCarriesReportOverview(t *testing.T) {
	overview := "这个项目从一个具体冲突出发，收束为可研究的问题。但目前论证更接近框架展示：证据基础较薄，尚未回到材料回答核心问题。"
	w := teacher.Detect([]teacher.StudentWeek{{
		UserID: "u1", DisplayName: "Phoebe", ActiveDays: 5, PrevActiveDays: 5,
		ReportsThisWeek: 2, PriorReports: 1,
		LatestReportProjectID: "proj-1", LatestReportOverview: overview,
	}})
	if len(w.Praise) != 1 {
		t.Fatalf("praise = %+v; want one card", w.Praise)
	}
	ov := w.Praise[0].ReportOverview
	if ov == "" {
		t.Fatal("praise card must carry the report 综述")
	}
	if []rune(ov)[len([]rune(ov))-1] != '。' {
		t.Fatalf("overview = %q; a truncated teaser should end on a sentence boundary", ov)
	}
	if len([]rune(ov)) > 110 {
		t.Fatalf("overview = %d runes; want <= 110", len([]rune(ov)))
	}
}

func TestWatchCardNeverCarriesReportOverview(t *testing.T) {
	// An inactive student may still have an OLD ready report on file, but the
	// watch card is about this week's silence — no substance teaser.
	w := teacher.Detect([]teacher.StudentWeek{{
		UserID: "u1", DisplayName: "沈亦然", ActiveDays: 0, PrevActiveDays: 2,
		LatestReportProjectID: "proj-old", LatestReportOverview: "一段旧的综述。",
	}})
	if len(w.Watch) != 1 {
		t.Fatalf("watch = %+v; want one card", w.Watch)
	}
	if w.Watch[0].ReportOverview != "" {
		t.Fatalf("watch card carried an overview %q; it must not", w.Watch[0].ReportOverview)
	}
}

func TestStatsRenderDeltaAndDirection(t *testing.T) {
	got := teacher.Stats(
		teacher.ClassWeekCounts{ActiveStudents: 8, Reports: 14, Turns: 386, CourseSteps: 23},
		teacher.ClassWeekCounts{ActiveStudents: 6, Reports: 14, Turns: 458, CourseSteps: 19},
		9,
	)
	if len(got) != 4 {
		t.Fatalf("stats = %d; want 4 cards", len(got))
	}
	if got[0].Value != 8 || got[0].Unit != "/ 9 人" || got[0].Delta != "+2" || got[0].DeltaDir != "up" {
		t.Fatalf("card 0 = %+v", got[0])
	}
	if got[1].Delta != "±0" || got[1].DeltaDir != "flat" {
		t.Fatalf("card 1 = %+v; an unchanged metric must be flat, never a green up-arrow", got[1])
	}
	if got[2].Delta != "-72" || got[2].DeltaDir != "down" {
		t.Fatalf("card 2 = %+v", got[2])
	}
	if got[3].Label != "完成课程节" || got[3].Unit != "节" {
		t.Fatalf("card 3 = %+v", got[3])
	}
}
