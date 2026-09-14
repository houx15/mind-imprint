package liteweekly

import "testing"

func TestCheckProse(t *testing.T) {
	c := ProseCheck{
		AllowedCodes: []string{"overdue", "new_interest"},
		Corpus:       "雨落在屋檐上像敲鼓\n金融",
		Titles:       "一场雨",
		FactsText:    "第 37 周（9.7–9.13） 活跃 3 天 学习 95 分钟 逾期 1 份",
		OtherNames:   []string{"王小明"},
	}
	ok := "本周有 1 份作业逾期。她写下「雨落在屋檐上像敲鼓」，可以请她继续完成《一场雨》。"
	if err := CheckProse(ok, []string{"overdue"}, c); err != nil {
		t.Fatalf("valid prose rejected: %v", err)
	}
	bad := map[string]struct {
		text  string
		codes []string
	}{
		"unknown code":     {"本周有 1 份作业逾期。", []string{"stalled"}},
		"fabricated quote": {"她写下「雨是天空的眼泪」。", []string{"overdue"}},
		"curly quote":      {"她说“我不想写”。", []string{"overdue"}},
		"stray digit":      {"本周学习 120 分钟。", []string{"overdue"}},
		"other student":    {"可以和王小明一起讨论。", []string{"overdue"}},
		"title as quote":   {"她写下「一场雨」。", []string{"overdue"}},
	}
	for name, b := range bad {
		if err := CheckProse(b.text, b.codes, c); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
}

func TestCheckProseQuoteWithDigitsPasses(t *testing.T) {
	c := ProseCheck{
		Corpus:    "这次我考了 128 分",
		FactsText: "第 37 周 活跃 3 天",
	}
	text := "她写下「这次我考了 128 分」。"
	if err := CheckProse(text, nil, c); err != nil {
		t.Fatalf("quote with digits rejected: %v", err)
	}
}

func TestCheckProseFullWidthDigitsFail(t *testing.T) {
	c := ProseCheck{
		FactsText: "第 37 周 活跃 3 天",
	}
	// full-width "120" — not a run present in FactsText once normalised.
	text := "本周学习了１２０分钟。"
	if err := CheckProse(text, nil, c); err == nil {
		t.Fatal("full-width digit run accepted")
	}
}

func TestCheckProseMinutesNoLeak(t *testing.T) {
	// Every other numeric field is 0 or 9/3; "1" appears nowhere in the
	// rendered facts once Minutes == -1 renders as 学习时长 无记录 instead
	// of leaking a bare "1" (from "-1").
	c := ProseCheck{
		FactsText: FactsText(StudentWeek{ActiveDays: 3, Minutes: -1}, "第 9 周"),
	}
	text := "本周学习了 1 分钟。"
	if err := CheckProse(text, nil, c); err == nil {
		t.Fatal("stray minutes digit accepted despite Minutes == -1")
	}
}

func TestCheckProseTitleNotInTitles(t *testing.T) {
	c := ProseCheck{
		Corpus:    "金融",
		Titles:    "一场雨",
		FactsText: "第 1 周 活跃 3 天",
	}
	text := "可以请她继续完成《咖啡》。"
	if err := CheckProse(text, nil, c); err == nil {
		t.Fatal("title not present in Titles accepted")
	}
}

func TestCheckProseNilCodesSkipsCodeCheck(t *testing.T) {
	c := ProseCheck{
		Corpus:    "雨落在屋檐上像敲鼓\n金融",
		Titles:    "一场雨",
		FactsText: "第 37 周（9.7–9.13） 活跃 3 天 学习 95 分钟 逾期 1 份",
	}
	ok := "她写下「雨落在屋檐上像敲鼓」，可以请她继续完成《一场雨》。"
	if err := CheckProse(ok, nil, c); err != nil {
		t.Fatalf("nil codes + nil AllowedCodes rejected valid text: %v", err)
	}

	bad := "她写下「雨是天空的眼泪」。"
	if err := CheckProse(bad, nil, c); err == nil {
		t.Fatal("nil codes accepted a fabricated quote")
	}
}
