package liteweekly

import (
	"regexp"
	"strconv"
	"strings"
	"testing"
)

var runRe = regexp.MustCompile(`[0-9]+`)

// TestClassFactsTextCarriesEveryNumberAndEvidence: the class prose may only
// use digits that appear in this text, so a stats number missing from it
// would reject correct prose on every attempt. Numbers are compared as whole
// digit runs so 37 is not satisfied by 137.
func TestClassFactsTextCarriesEveryNumberAndEvidence(t *testing.T) {
	stats := ClassWeekStats{ClassSize: 23, ActiveStudents: 17, Minutes: 412, Turns: 138, Finished: 9, AssignmentRate: 64}
	flagged := []StudentWeek{
		{UserID: "u1", Name: "林知遥"},
		{UserID: "u2", Name: "周子墨"},
	}
	cards := map[string][]Card{
		"u1": {
			{Kind: "watch", Code: "overdue", Label: "作业逾期", Evidence: "本周到期的作业中有 2 份未完成。"},
			{Kind: "praise", Code: "new_interest", Label: "新的兴趣", Evidence: "兴趣树新增关键词：海绵城市。"},
		},
		"u2": {
			{Kind: "watch", Code: "stalled", Label: "进度停滞", Evidence: "《雨水花园调查报告》超过 7 天没有进展。"},
		},
		"u3": {}, // not flagged: must not appear
	}

	out := ClassFactsText("IBDP 一年级", "9 月 1 日–9 月 7 日", stats, flagged, cards)

	runs := map[string]bool{}
	for _, r := range runRe.FindAllString(out, -1) {
		runs[r] = true
	}
	for _, n := range []int{stats.ClassSize, stats.ActiveStudents, stats.Minutes, stats.Turns, stats.Finished, stats.AssignmentRate} {
		if !runs[strconv.Itoa(n)] {
			t.Errorf("facts text is missing number %d:\n%s", n, out)
		}
	}
	for _, want := range []string{
		"IBDP 一年级", "9 月 1 日–9 月 7 日", "林知遥", "周子墨",
		"作业逾期", "新的兴趣", "进度停滞", "值得表扬", "需要建议",
		"本周到期的作业中有 2 份未完成。", "兴趣树新增关键词：海绵城市。", "《雨水花园调查报告》超过 7 天没有进展。",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("facts text is missing %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "u1") || strings.Contains(out, "u2") {
		t.Errorf("facts text must not carry user IDs (their digits would widen the digit check):\n%s", out)
	}
}

// TestClassFactsTextNoAssignmentsDue: -1 is a sentinel. Rendering it raw
// would put "1" in the allowed digit set.
func TestClassFactsTextNoAssignmentsDue(t *testing.T) {
	out := ClassFactsText("班", "周", ClassWeekStats{ClassSize: 5, ActiveStudents: 4, Minutes: 30, Turns: 6, Finished: 2, AssignmentRate: -1}, nil, nil)
	allowed := map[string]bool{"5": true, "4": true, "30": true, "6": true, "2": true}
	for _, r := range runRe.FindAllString(out, -1) {
		if !allowed[r] {
			t.Fatalf("digit run %q is not a stats number (sentinel -1 leaked?):\n%s", r, out)
		}
	}
	if !strings.Contains(out, "作业完成率 无到期作业") {
		t.Fatalf("want 作业完成率 无到期作业:\n%s", out)
	}
	if !strings.Contains(out, "卡片：无") {
		t.Fatalf("want 卡片：无 when nobody is flagged:\n%s", out)
	}
}
