package liteweekly

import (
	"strings"
	"testing"
)

func TestCheckProse(t *testing.T) {
	c := ProseCheck{
		AllowedCodes: []string{"overdue", "new_interest"},
		Corpus:       "雨落在屋檐上像敲鼓\n金融",
		Titles:       "一场雨",
		FactsText:    "第 37 周（9.7–9.13） 活跃 3 天 学习 95 分钟 逾期 1 份",
		OtherNames:   []string{"王小明"},
	}

	good := map[string]struct {
		text  string
		codes []string
	}{
		"valid prose": {
			"本周有 1 份作业逾期。她写下「雨落在屋檐上像敲鼓」，可以请她继续完成《一场雨》。",
			[]string{"overdue"},
		},
		"nil codes skip the code check": {
			"本周有 1 份作业逾期。她写下「雨落在屋檐上像敲鼓」，可以请她继续完成《一场雨》。",
			nil,
		},
	}
	for name, g := range good {
		if err := CheckProse(g.text, g.codes, c); err != nil {
			t.Errorf("%s: rejected: %v", name, err)
		}
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

// TestCheckProseNames: the other-names check runs on the text with verified
// quote and title spans removed and with her own name removed.
func TestCheckProseNames(t *testing.T) {
	cases := []struct {
		name   string
		check  ProseCheck
		text   string
		wantOK bool
	}{
		{
			name:   "classmate name inside her name",
			check:  ProseCheck{FactsText: "活跃 3 天", OtherNames: []string{"王丽"}, SelfName: "王丽华"},
			text:   "王丽华该周活跃 3 天，请王丽华继续保持。",
			wantOK: true,
		},
		{
			name:   "classmate name inside a verified title",
			check:  ProseCheck{Titles: "一场小雨", FactsText: "活跃 3 天", OtherNames: []string{"小雨"}},
			text:   "她读完了《一场小雨》。",
			wantOK: true,
		},
		{
			name:   "classmate name inside her verified quote",
			check:  ProseCheck{Corpus: "我和小雨一起看雨", FactsText: "活跃 3 天", OtherNames: []string{"小雨"}},
			text:   "她写下「我和小雨一起看雨」。",
			wantOK: true,
		},
		{
			name:   "classmate named outside any span",
			check:  ProseCheck{Titles: "一场小雨", FactsText: "活跃 3 天", OtherNames: []string{"小雨"}},
			text:   "她读完了《一场小雨》，可以和小雨一起讨论。",
			wantOK: false,
		},
		{
			name:   "classmate named next to her name",
			check:  ProseCheck{FactsText: "活跃 3 天", OtherNames: []string{"王丽"}, SelfName: "王丽华"},
			text:   "王丽华可以和王丽一起讨论。",
			wantOK: false,
		},
		{
			name:   "empty SelfName removes nothing",
			check:  ProseCheck{FactsText: "活跃 3 天", OtherNames: []string{"王丽"}},
			text:   "王丽华该周活跃 3 天。",
			wantOK: false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := CheckProse(tc.text, nil, tc.check)
			if tc.wantOK && err != nil {
				t.Fatalf("rejected: %v", err)
			}
			if !tc.wantOK {
				if err == nil {
					t.Fatal("accepted")
				}
				if !strings.Contains(err.Error(), "mentions other student") {
					t.Fatalf("err = %v, want mentions other student", err)
				}
			}
		})
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

func TestCheckProseUnclosedQuoteFails(t *testing.T) {
	c := ProseCheck{
		Corpus:    "雨落在屋檐上像敲鼓",
		FactsText: "第 1 周 活跃 3 天",
	}
	err := CheckProse("她写下「从未出现的内容", nil, c)
	if err == nil {
		t.Fatal("unclosed quote accepted")
	}
	if !strings.Contains(err.Error(), "unclosed quote") {
		t.Fatalf("error = %v, want to contain %q", err, "unclosed quote")
	}
}

func TestCheckProseUnclosedTitleFails(t *testing.T) {
	c := ProseCheck{
		Titles:    "一场雨",
		FactsText: "第 1 周 活跃 3 天",
	}
	err := CheckProse("可以请她继续完成《一场雨", nil, c)
	if err == nil {
		t.Fatal("unclosed title accepted")
	}
	if !strings.Contains(err.Error(), "unclosed quote") {
		t.Fatalf("error = %v, want to contain %q", err, "unclosed quote")
	}
}

func TestCheckProseStrayClosingMarkFails(t *testing.T) {
	c := ProseCheck{
		FactsText: "第 1 周 活跃 3 天",
	}
	err := CheckProse("她写下雨落在屋檐上像敲鼓」。", nil, c)
	if err == nil {
		t.Fatal("stray closing mark accepted")
	}
	if !strings.Contains(err.Error(), "unmatched closing mark") {
		t.Fatalf("error = %v, want to contain %q", err, "unmatched closing mark")
	}
}

func TestCheckProseEmptyQuotePasses(t *testing.T) {
	c := ProseCheck{
		FactsText: "第 1 周 活跃 3 天",
	}
	if err := CheckProse("她写下「」，什么都没说。", nil, c); err != nil {
		t.Fatalf("empty quote rejected: %v", err)
	}
}
