package teacher_test

import (
	"strings"
	"testing"

	"mindimprint/api/internal/agent"
	"mindimprint/api/internal/teacher"
)

func rep(depth []agent.DepthDim, auto []agent.AutonomySignal) *agent.Report {
	return &agent.Report{DepthAxis: depth, AutonomyAxis: auto}
}

func autos(levels ...int) []agent.AutonomySignal {
	codes := []string{"A1", "A2", "A3", "A4", "A5", "A6"}
	out := make([]agent.AutonomySignal, 0, len(levels))
	for i, l := range levels {
		out = append(out, agent.AutonomySignal{
			Code: codes[i], Level: l, Opportunity: "given_taken",
			Evidence: "自主证据 " + codes[i],
		})
	}
	return out
}

func depths(levels ...string) []agent.DepthDim {
	codes := []string{"D1", "D2", "D3", "D4", "D5", "D6"}
	out := make([]agent.DepthDim, 0, len(levels))
	for i, l := range levels {
		out = append(out, agent.DepthDim{Code: codes[i], Level: l, Evidence: "深度证据 " + codes[i]})
	}
	return out
}

func TestNeverUsedFiresOnZeroActiveDays(t *testing.T) {
	w := teacher.Detect([]teacher.StudentWeek{{
		UserID: "u1", DisplayName: "罗一", ActiveDays: 0, PrevActiveDays: 0,
	}})
	if len(w.Watch) != 1 || w.Watch[0].TagCode != "never_used" {
		t.Fatalf("watch = %+v; want one never_used card", w.Watch)
	}
	if w.Watch[0].TagLabel != "本周未使用" {
		t.Fatalf("label = %q; want 本周未使用", w.Watch[0].TagLabel)
	}
}

func TestDroppedOffNeedsATwoDayFallAndNoNewReport(t *testing.T) {
	fires := teacher.Detect([]teacher.StudentWeek{{
		UserID: "u1", DisplayName: "陈屿", ActiveDays: 2, PrevActiveDays: 5, ReportsThisWeek: 0,
	}})
	if len(fires.Watch) != 1 || fires.Watch[0].TagCode != "dropped_off" {
		t.Fatalf("watch = %+v; want dropped_off", fires.Watch)
	}
	if !strings.Contains(fires.Watch[0].Evidence, "5 天") || !strings.Contains(fires.Watch[0].Evidence, "2 天") {
		t.Fatalf("evidence = %q; want both week counts stated", fires.Watch[0].Evidence)
	}
	quiet := teacher.Detect([]teacher.StudentWeek{{
		UserID: "u1", DisplayName: "陈屿", ActiveDays: 4, PrevActiveDays: 5, ReportsThisWeek: 0,
	}})
	if len(quiet.Watch) != 0 {
		t.Fatalf("watch = %+v; a one-day dip must not fire", quiet.Watch)
	}
	reported := teacher.Detect([]teacher.StudentWeek{{
		UserID: "u1", DisplayName: "陈屿", ActiveDays: 2, PrevActiveDays: 5, ReportsThisWeek: 1,
	}})
	if len(reported.Watch) != 0 {
		t.Fatalf("watch = %+v; a student who produced a report this week is not offline", reported.Watch)
	}
}

func TestDroppedOffFiresExactlyAtTheTwoDayThreshold(t *testing.T) {
	w := teacher.Detect([]teacher.StudentWeek{{
		UserID: "u1", DisplayName: "陈屿", ActiveDays: 3, PrevActiveDays: 5, ReportsThisWeek: 0,
	}})
	if len(w.Watch) != 1 || w.Watch[0].TagCode != "dropped_off" {
		t.Fatalf("watch = %+v; a fall of exactly 2 (5→3) must fire dropped_off", w.Watch)
	}
}

func TestOutsourcedJudgmentQuotesTheLowestSuppliedSignal(t *testing.T) {
	r := rep(depths("L1", "L2"), autos(0, 1, 1, 1, 1, 1)) // mean 0.833
	w := teacher.Detect([]teacher.StudentWeek{{
		UserID: "u1", DisplayName: "周子墨", ActiveDays: 3, PrevActiveDays: 3, Latest: r,
	}})
	if len(w.Watch) != 1 || w.Watch[0].TagCode != "outsourced_judgment" {
		t.Fatalf("watch = %+v; want outsourced_judgment", w.Watch)
	}
	if !strings.Contains(w.Watch[0].Evidence, "自主证据 A1") {
		t.Fatalf("evidence = %q; want the lowest supplied signal's own evidence, verbatim", w.Watch[0].Evidence)
	}
}

func TestOutsourcedJudgmentSkipsNotSuppliedWhenQuoting(t *testing.T) {
	auto := autos(0, 1, 1, 1, 1, 1) // A1 would be the numeric lowest — but it's not_supplied
	auto[0] = agent.AutonomySignal{
		Code: "A1", Level: 0, Opportunity: "not_supplied",
		Evidence: "未获机会证据 A1（不应出现）",
	}
	r := rep(depths("L3"), auto) // supplied signals (A2-A6) mean 1.0
	w := teacher.Detect([]teacher.StudentWeek{{
		UserID: "u1", DisplayName: "周子墨", ActiveDays: 3, PrevActiveDays: 3, Latest: r,
	}})
	if len(w.Watch) != 1 || w.Watch[0].TagCode != "outsourced_judgment" {
		t.Fatalf("watch = %+v; want outsourced_judgment", w.Watch)
	}
	if !strings.Contains(w.Watch[0].Evidence, "自主证据 A2") {
		t.Fatalf("evidence = %q; want the lowest SUPPLIED signal's evidence (A2), verbatim", w.Watch[0].Evidence)
	}
	if strings.Contains(w.Watch[0].Evidence, "未获机会证据 A1") {
		t.Fatalf("evidence = %q; must never quote a not_supplied signal's evidence", w.Watch[0].Evidence)
	}
}

func TestNoBoundariesIgnoresNotSuppliedA3(t *testing.T) {
	supplied := autos(4, 4, 4, 4, 4, 4)
	supplied[2] = agent.AutonomySignal{Code: "A3", Level: 0, Opportunity: "given_not_taken", Evidence: "没有一条边界句"}
	w := teacher.Detect([]teacher.StudentWeek{{
		UserID: "u1", DisplayName: "许清", ActiveDays: 4, PrevActiveDays: 4, Latest: rep(depths("L2"), supplied),
	}})
	if len(w.Watch) != 1 || w.Watch[0].TagCode != "no_boundaries" {
		t.Fatalf("watch = %+v; want no_boundaries", w.Watch)
	}

	debt := autos(4, 4, 4, 4, 4, 4)
	debt[2] = agent.AutonomySignal{Code: "A3", Level: 0, Opportunity: "not_supplied", Evidence: ""}
	quiet := teacher.Detect([]teacher.StudentWeek{{
		UserID: "u1", DisplayName: "许清", ActiveDays: 4, PrevActiveDays: 4, Latest: rep(depths("L2"), debt),
	}})
	if len(quiet.Watch) != 0 {
		t.Fatalf("watch = %+v; a not_supplied A3 is platform debt, never a student failing", quiet.Watch)
	}
}

func TestWatchBeatsPraise(t *testing.T) {
	prev := rep(depths("L1", "L1"), autos(0, 0, 0, 0, 0, 0))
	latest := rep(depths("L1", "L1"), autos(1, 1, 1, 1, 1, 1)) // A mean 0→1: +1.0, but still ≤1.0
	w := teacher.Detect([]teacher.StudentWeek{{
		UserID: "u1", DisplayName: "周子墨", ActiveDays: 3, PrevActiveDays: 3, Latest: latest, Previous: prev,
	}})
	if len(w.Praise) != 0 {
		t.Fatalf("praise = %+v; a student still outsourcing judgment must not be filed as praise", w.Praise)
	}
	if len(w.Watch) != 1 || w.Watch[0].TagCode != "outsourced_judgment" {
		t.Fatalf("watch = %+v; want outsourced_judgment", w.Watch)
	}
}

// stuckAtStart neutral baseline: avoids every earlier-priority watch rule so
// the report actually reaches the stuck_at_start check. ActiveDays ==
// PrevActiveDays (never_used / dropped_off), autonomy mean 2.0 > 1.0
// (outsourced_judgment), and a non-zero A3 (no_boundaries).
func stuckAtStartWeek(depth []agent.DepthDim) teacher.StudentWeek {
	return teacher.StudentWeek{
		UserID: "u1", DisplayName: "白露", ActiveDays: 5, PrevActiveDays: 5,
		Latest: rep(depth, autos(2, 2, 2, 2, 2, 2)),
	}
}

func TestStuckAtStartFiresWhenHalfRatedDimsAreL1(t *testing.T) {
	w := teacher.Detect([]teacher.StudentWeek{stuckAtStartWeek(depths("L1", "L2"))})
	if len(w.Watch) != 1 || w.Watch[0].TagCode != "stuck_at_start" {
		t.Fatalf("watch = %+v; want stuck_at_start (1 of 2 rated dims at L1, none above L2)", w.Watch)
	}
	if w.Watch[0].TagLabel != "停在起步档" {
		t.Fatalf("label = %q; want 停在起步档", w.Watch[0].TagLabel)
	}
	if w.Watch[0].Evidence != "深度证据 D1" {
		t.Fatalf("evidence = %q; want the lowest-ranked depth dim's own evidence, verbatim", w.Watch[0].Evidence)
	}
}

func TestStuckAtStartDoesNotFireBelowHalfL1(t *testing.T) {
	w := teacher.Detect([]teacher.StudentWeek{stuckAtStartWeek(depths("L1", "L2", "L2"))})
	if len(w.Watch) != 0 {
		t.Fatalf("watch = %+v; only 1 of 3 rated dims at L1 must not fire stuck_at_start", w.Watch)
	}
}

func TestStuckAtStartDoesNotFireWithADimAboveL2(t *testing.T) {
	w := teacher.Detect([]teacher.StudentWeek{stuckAtStartWeek(depths("L1", "L3"))})
	if len(w.Watch) != 0 {
		t.Fatalf("watch = %+v; a dim above L2 must not fire stuck_at_start even though half are L1", w.Watch)
	}
}

func TestStuckAtStartFiresOnOddRatedCount(t *testing.T) {
	w := teacher.Detect([]teacher.StudentWeek{stuckAtStartWeek(depths("L1", "L1", "L2"))})
	if len(w.Watch) != 1 || w.Watch[0].TagCode != "stuck_at_start" {
		t.Fatalf("watch = %+v; want stuck_at_start (2 of 3 rated dims at L1, none above L2)", w.Watch)
	}
}

func TestStuckAtStartDoesNotFireWithZeroRatedDims(t *testing.T) {
	w := teacher.Detect([]teacher.StudentWeek{stuckAtStartWeek(depths("NA", "NA"))})
	if len(w.Watch) != 0 || len(w.Praise) != 0 {
		t.Fatalf("cards = %+v/%+v; a report with no rated depth dims must produce no card", w.Watch, w.Praise)
	}
}

func TestDepthUpFiresOnAHigherMaxLevel(t *testing.T) {
	w := teacher.Detect([]teacher.StudentWeek{{
		UserID: "u1", DisplayName: "吴桐", ActiveDays: 5, PrevActiveDays: 5,
		Previous: rep(depths("L2", "L2"), autos(3, 3, 3, 3, 3, 3)),
		Latest:   rep(depths("L2", "L3"), autos(3, 3, 3, 3, 3, 3)),
	}})
	if len(w.Praise) != 1 || w.Praise[0].TagCode != "depth_up" {
		t.Fatalf("praise = %+v; want depth_up", w.Praise)
	}
	if w.Praise[0].Kind != "praise" {
		t.Fatalf("kind = %q; want praise", w.Praise[0].Kind)
	}
}

func TestUnratedStudentEarnsNoJudgmentTag(t *testing.T) {
	w := teacher.Detect([]teacher.StudentWeek{{
		UserID: "u1", DisplayName: "新同学", ActiveDays: 4, PrevActiveDays: 4, Latest: nil,
	}})
	if len(w.Watch) != 0 || len(w.Praise) != 0 {
		t.Fatalf("cards = %+v/%+v; no report means no judgment", w.Watch, w.Praise)
	}
}

func TestDepthDistributionBucketsOnTheBadgeUpperBound(t *testing.T) {
	w := teacher.Detect([]teacher.StudentWeek{
		{UserID: "a", DisplayName: "林知远", ActiveDays: 6, PrevActiveDays: 6, Latest: rep(depths("L3", "L4"), autos(4, 4, 4, 4, 4, 4))},
		{UserID: "b", DisplayName: "苏晚", ActiveDays: 5, PrevActiveDays: 5, Latest: rep(depths("L3", "L3"), autos(3, 3, 3, 3, 3, 3))},
		{UserID: "c", DisplayName: "新同学", ActiveDays: 1, PrevActiveDays: 1, Latest: nil},
	})
	if w.Depth.RatedCount != 2 {
		t.Fatalf("ratedCount = %d; want 2 — an unrated student is counted nowhere", w.Depth.RatedCount)
	}
	byCode := map[string]int{}
	for _, b := range w.Depth.Buckets {
		byCode[b.Code] = b.Count
	}
	if byCode["L4"] != 1 || byCode["L3"] != 1 {
		t.Fatalf("buckets = %+v; L3–L4 buckets on its upper bound (L4)", w.Depth.Buckets)
	}
	if len(w.Depth.Buckets) != 4 {
		t.Fatalf("buckets = %d; want all four rendered, including empty ones", len(w.Depth.Buckets))
	}
}

func TestAutonomyMeanAndDelta(t *testing.T) {
	w := teacher.Detect([]teacher.StudentWeek{
		{UserID: "a", DisplayName: "甲", ActiveDays: 5, PrevActiveDays: 5,
			Previous: rep(depths("L3"), autos(2, 2, 2, 2, 2, 2)),
			Latest:   rep(depths("L3"), autos(3, 3, 3, 3, 3, 3))},
		{UserID: "b", DisplayName: "乙", ActiveDays: 5, PrevActiveDays: 5,
			Latest: rep(depths("L3"), autos(2, 2, 2, 2, 2, 2))},
	})
	if w.Autonomy.Mean != "2.5" {
		t.Fatalf("mean = %q; want 2.5 (3.0 and 2.0)", w.Autonomy.Mean)
	}
	if w.Autonomy.Delta != "+1.0" {
		t.Fatalf("delta = %q; want +1.0 (only 甲 has a baseline)", w.Autonomy.Delta)
	}
	if w.Autonomy.DeltaDir != "up" {
		t.Fatalf("dir = %q; want up when the class's mean autonomy rose", w.Autonomy.DeltaDir)
	}
}

func TestAutonomyDeltaIsEmDashWithoutABaseline(t *testing.T) {
	w := teacher.Detect([]teacher.StudentWeek{
		{UserID: "a", DisplayName: "甲", ActiveDays: 5, PrevActiveDays: 5, Latest: rep(depths("L3"), autos(3, 3, 3, 3, 3, 3))},
	})
	if w.Autonomy.Delta != "—" {
		t.Fatalf("delta = %q; want the em-dash U+2014 when no student has a second report", w.Autonomy.Delta)
	}
	if w.Autonomy.DeltaDir != "flat" {
		t.Fatalf("dir = %q; want flat when nobody has a baseline — there is no direction to claim", w.Autonomy.DeltaDir)
	}
}

func TestAutonomyDeltaDirDownWhenTheClassMeanFalls(t *testing.T) {
	w := teacher.Detect([]teacher.StudentWeek{
		{UserID: "a", DisplayName: "甲", ActiveDays: 5, PrevActiveDays: 5,
			Previous: rep(depths("L3"), autos(3, 3, 3, 3, 3, 3)),
			Latest:   rep(depths("L3"), autos(2, 2, 2, 2, 2, 2))},
	})
	if w.Autonomy.Delta != "-1.0" {
		t.Fatalf("delta = %q; want -1.0", w.Autonomy.Delta)
	}
	if w.Autonomy.DeltaDir != "down" {
		t.Fatalf("dir = %q; want down when the class's mean autonomy fell — the pill must never lie green", w.Autonomy.DeltaDir)
	}
}

// TestAutonomyDeltaDirAgreesWithTheRoundedRenderedNumber pins the exact bug
// this fix closes: a raw per-student mean change of ~0.028 is a genuine,
// positive rise, but %+.1f renders it as "+0.0" — DeltaDir must read that
// same rendered string and say flat, never up, so the pill and the number
// can never disagree.
func TestAutonomyDeltaDirAgreesWithTheRoundedRenderedNumber(t *testing.T) {
	same := autos(2, 2, 2, 2, 2, 2)
	noChange := func(id, name string) teacher.StudentWeek {
		return teacher.StudentWeek{
			UserID: id, DisplayName: name, ActiveDays: 5, PrevActiveDays: 5,
			Previous: rep(depths("L3"), same), Latest: rep(depths("L3"), same),
		}
	}
	students := []teacher.StudentWeek{
		noChange("s1", "学生一"), noChange("s2", "学生二"), noChange("s3", "学生三"),
		noChange("s4", "学生四"), noChange("s5", "学生五"),
		{
			UserID: "bump", DisplayName: "微升", ActiveDays: 5, PrevActiveDays: 5,
			Previous: rep(depths("L3"), autos(2, 2, 2, 2, 2, 2)),
			Latest:   rep(depths("L3"), autos(3, 2, 2, 2, 2, 2)), // +1/6 for this student alone
		},
	}
	w := teacher.Detect(students)
	// deltaSum = 1/6 over deltaN = 6 students → raw avg ≈ 0.0278, a genuine
	// rise — but %+.1f renders "+0.0".
	if w.Autonomy.Delta != "+0.0" {
		t.Fatalf("delta = %q; want +0.0 (raw ~0.028 rounds down to the tenth)", w.Autonomy.Delta)
	}
	if w.Autonomy.DeltaDir != "flat" {
		t.Fatalf("dir = %q; want flat — a +0.0-rendering change must never carry an up-arrow", w.Autonomy.DeltaDir)
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
