package liteweekly

import (
	"strings"
	"testing"
)

func TestNeverUsedSuppressesPraise(t *testing.T) {
	w, p := Cards(StudentWeek{ActiveDays: 0, PrevActiveDays: 4, NewKeywords: []string{"金融"}})
	if w == nil || w.Code != "never_used" || p != nil {
		t.Fatalf("watch=%+v praise=%+v", w, p)
	}
	if w.Evidence != "该周 0 天有学习记录；上一周 4 天。" {
		t.Fatalf("evidence = %q", w.Evidence)
	}
}

func TestOverdueBeatsDroppedOff(t *testing.T) {
	w, _ := Cards(StudentWeek{ActiveDays: 1, PrevActiveDays: 5, AssignmentsOverdue: 2})
	if w.Code != "overdue" {
		t.Fatalf("code = %s", w.Code)
	}
}

func TestOnTimeNeedsNoOverdue(t *testing.T) {
	_, p := Cards(StudentWeek{ActiveDays: 3, AssignmentsDone: 1, AssignmentsOverdue: 1})
	if p != nil && p.Code == "finished_on_time" {
		t.Fatal("finished_on_time fired with an overdue assignment")
	}
}

func TestMoreActiveThreshold(t *testing.T) {
	if _, p := Cards(StudentWeek{ActiveDays: 3, PrevActiveDays: 1}); p == nil || p.Code != "more_active" {
		t.Fatalf("3 vs 1: %+v", p)
	}
	if _, p := Cards(StudentWeek{ActiveDays: 2, PrevActiveDays: 0}); p != nil {
		t.Fatalf("2 vs 0 should not fire: %+v", p)
	}
}

func TestStalledEvidenceCountsExtra(t *testing.T) {
	w, _ := Cards(StudentWeek{ActiveDays: 2, PrevActiveDays: 2, Stalled: []Item{{"writing", "一场雨"}, {"reading", "咖啡"}}})
	if w.Code != "stalled" || w.Evidence != "《一场雨》等 2 项超过 7 天没有进展。" {
		t.Fatalf("%+v", w)
	}
}

func TestStalledEvidenceSingleItem(t *testing.T) {
	w, _ := Cards(StudentWeek{ActiveDays: 2, PrevActiveDays: 2, Stalled: []Item{{"writing", "一场雨"}}})
	if w.Code != "stalled" || w.Evidence != "《一场雨》超过 7 天没有进展。" {
		t.Fatalf("%+v", w)
	}
}

func TestDroppedOff(t *testing.T) {
	w, _ := Cards(StudentWeek{ActiveDays: 1, PrevActiveDays: 3})
	if w == nil || w.Code != "dropped_off" {
		t.Fatalf("watch = %+v", w)
	}
	if w.Evidence != "活跃天数 上一周 3 天 → 该周 1 天。" {
		t.Fatalf("evidence = %q", w.Evidence)
	}
}

func TestFinishedOnTime(t *testing.T) {
	_, p := Cards(StudentWeek{ActiveDays: 3, AssignmentsDone: 2, AssignmentsOverdue: 0})
	if p == nil || p.Code != "finished_on_time" {
		t.Fatalf("praise = %+v", p)
	}
	if p.Evidence != "该周到期的作业按时完成 2 份。" {
		t.Fatalf("evidence = %q", p.Evidence)
	}
}

func TestNewInterest(t *testing.T) {
	_, p := Cards(StudentWeek{ActiveDays: 1, NewKeywords: []string{"金融", "编程"}})
	if p == nil || p.Code != "new_interest" {
		t.Fatalf("praise = %+v", p)
	}
	if p.Evidence != "兴趣树新增关键词：金融、编程。" {
		t.Fatalf("evidence = %q", p.Evidence)
	}
}

func TestNewInterestCapsAtThree(t *testing.T) {
	_, p := Cards(StudentWeek{ActiveDays: 1, NewKeywords: []string{"金融", "编程", "生物", "历史"}})
	if p == nil || p.Code != "new_interest" {
		t.Fatalf("praise = %+v", p)
	}
	if p.Evidence != "兴趣树新增关键词：金融、编程、生物。" {
		t.Fatalf("evidence = %q", p.Evidence)
	}
}

func TestFactsTextMinutesNoRecord(t *testing.T) {
	ft := FactsText(StudentWeek{Minutes: -1}, "第 1 周")
	if want := "学习时长 无记录"; !strings.Contains(ft, want) {
		t.Fatalf("FactsText missing %q: %q", want, ft)
	}
	if strings.Contains(ft, "-1") {
		t.Fatalf("FactsText leaked -1: %q", ft)
	}
}

func TestFactsTextRendersEveryNumericField(t *testing.T) {
	s := StudentWeek{
		ActiveDays:         3,
		PrevActiveDays:     1,
		Minutes:            95,
		Turns:              12,
		Finished:           []Item{{"reading", "一场雨"}},
		AssignmentsDone:    1,
		AssignmentsLate:    0,
		AssignmentsOverdue: 1,
		Stalled:            []Item{{"writing", "咖啡"}},
	}
	ft := FactsText(s, "第 37 周")
	for _, want := range []string{"3", "1", "95", "12", "0"} {
		if !strings.Contains(ft, want) {
			t.Fatalf("FactsText missing digit %q: %q", want, ft)
		}
	}
	for _, want := range []string{"《一场雨》", "《咖啡》"} {
		if !strings.Contains(ft, want) {
			t.Fatalf("FactsText missing %q: %q", want, ft)
		}
	}
}

func TestCorpusIsQuotesAndKeywordsOnly(t *testing.T) {
	s := StudentWeek{
		Finished:    []Item{{"reading", "一场雨"}},
		Stalled:     []Item{{"writing", "咖啡"}},
		NewKeywords: []string{"金融"},
		Moments:     []Moment{{Quote: "雨落在屋檐上像敲鼓", ItemTitle: "一场雨"}},
	}
	corpus := Corpus(s)
	if !strings.Contains(corpus, "雨落在屋檐上像敲鼓") || !strings.Contains(corpus, "金融") {
		t.Fatalf("corpus missing her words: %q", corpus)
	}
	if strings.Contains(corpus, "一场雨") || strings.Contains(corpus, "咖啡") {
		t.Fatalf("corpus leaked a title: %q", corpus)
	}
}

func TestTitleCorpusIsTitlesOnly(t *testing.T) {
	s := StudentWeek{
		Finished:    []Item{{"reading", "一场雨"}},
		Stalled:     []Item{{"writing", "咖啡"}},
		NewKeywords: []string{"金融"},
		Moments:     []Moment{{Quote: "雨落在屋檐上像敲鼓", ItemTitle: "一场雨"}},
	}
	titles := TitleCorpus(s)
	if !strings.Contains(titles, "一场雨") || !strings.Contains(titles, "咖啡") {
		t.Fatalf("titles missing an item: %q", titles)
	}
	if strings.Contains(titles, "雨落在屋檐上像敲鼓") || strings.Contains(titles, "金融") {
		t.Fatalf("titles leaked her words: %q", titles)
	}
}
