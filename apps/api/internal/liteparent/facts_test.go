package liteparent

import (
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"mindimprint/api/internal/liteweek"
)

func TestParseRange(t *testing.T) {
	now := time.Date(2026, 9, 14, 10, 0, 0, 0, liteweek.Beijing)
	if _, _, err := ParseRange("2026-08-15", "2026-09-13", now); err != nil {
		t.Fatalf("valid: %v", err)
	}
	if _, _, err := ParseRange("2026-09-14", "2026-09-14", now); err != nil {
		t.Fatalf("today as end is allowed: %v", err)
	}
	for _, c := range [][2]string{
		{"2026-09-13", "2026-09-12"}, // reversed
		{"2026-09-10", "2026-09-15"}, // end in the future
		{"2025-09-01", "2026-09-13"}, // > 366 days
		{"2026/09/01", "2026-09-13"}, // format
	} {
		if _, _, err := ParseRange(c[0], c[1], now); !errors.Is(err, ErrBadRange) {
			t.Errorf("%v: want ErrBadRange, got %v", c, err)
		}
	}
}

// The 366-day limit counts both ends: exactly 366 days passes, 367 fails.
// Today is judged in Beijing: 2026-09-14 01:00 Beijing is still 2026-09-13 in UTC.
func TestParseRangeBoundaries(t *testing.T) {
	now := time.Date(2026, 9, 14, 10, 0, 0, 0, liteweek.Beijing)
	s, e, err := ParseRange("2025-09-13", "2026-09-13", now)
	if err != nil {
		t.Fatalf("366 days: %v", err)
	}
	if RangeDays(s, e) != 366 {
		t.Fatalf("RangeDays = %d, want 366", RangeDays(s, e))
	}
	if !s.Equal(time.Date(2025, 9, 13, 0, 0, 0, 0, liteweek.Beijing)) {
		t.Fatalf("start = %v, want Beijing midnight", s)
	}
	if _, _, err := ParseRange("2025-09-12", "2026-09-13", now); !errors.Is(err, ErrBadRange) {
		t.Fatalf("367 days: want ErrBadRange, got %v", err)
	}
	earlyUTC := time.Date(2026, 9, 13, 17, 0, 0, 0, time.UTC) // 2026-09-14 01:00 Beijing
	if _, _, err := ParseRange("2026-09-14", "2026-09-14", earlyUTC); err != nil {
		t.Fatalf("today in Beijing while still yesterday in UTC: %v", err)
	}
}

func TestDefaultRange(t *testing.T) {
	s, e := DefaultRange(time.Date(2026, 9, 14, 10, 0, 0, 0, liteweek.Beijing))
	if s != "2026-08-18" || e != "2026-09-14" {
		t.Fatalf("default = %s..%s", s, e)
	}
}

func TestSectionsWithFacts(t *testing.T) {
	got := SectionsWithFacts(Facts{Writings: []Item{{Kind: "writing", Title: "一场雨"}}})
	want := []string{"overview", "writing", "next"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
	all := SectionsWithFacts(Facts{
		Readings: []Item{{Title: "a"}}, Writings: []Item{{Title: "b"}},
		Projects: []Item{{Title: "c"}}, Keywords: []Keyword{{Text: "d"}},
	})
	if !reflect.DeepEqual(all, SectionKeys) {
		t.Fatalf("all sections = %v, want %v", all, SectionKeys)
	}
}

func TestFactsTextCarriesEveryNumber(t *testing.T) {
	f := Facts{RangeStart: "2026-08-17", RangeEnd: "2026-09-13", Days: 28, ActiveDays: 12, Minutes: 340, Turns: 86, AssignmentsTotal: 3, AssignmentsOnTime: 2, AssignmentsLate: 1}
	txt := FactsText(f)
	for _, n := range []string{"2026", "08", "17", "09", "13", "28", "12", "340", "86", "3", "2", "1"} {
		if !strings.Contains(txt, n) {
			t.Errorf("facts text missing %s: %s", n, txt)
		}
	}
}

// Each numeric field gets a distinct value so a dropped field cannot hide
// behind another field's digits.
func TestFactsTextContainsEveryNumericField(t *testing.T) {
	f := Facts{
		RangeStart: "2026-08-17", RangeEnd: "2026-09-13",
		Days: 28, ActiveDays: 11, Minutes: 347, Turns: 86,
		AssignmentsTotal: 19, AssignmentsOnTime: 14, AssignmentsLate: 3, AssignmentsMissed: 2,
		Readings: make([]Item, 4), Writings: make([]Item, 5), Projects: make([]Item, 6),
		Keywords: make([]Keyword, 7),
	}
	txt := FactsText(f)
	for _, want := range []string{
		"共 28 天", "活跃 11 天", "学习 347 分钟", "对话 86 轮",
		"到期作业 19 份", "按时完成作业 14 份", "逾期完成作业 3 份", "未完成作业 2 份",
		"完成阅读 4 篇", "完成写作 5 篇", "完成项目 6 个",
		"8月17日", "9月13日",
	} {
		if !strings.Contains(txt, want) {
			t.Errorf("facts text missing %q: %s", want, txt)
		}
	}
}

// The dates and day count carry no digit 1, so any 1 in the text could only
// come from the -1 sentinel. That also keeps "1" out of the allowed digit
// runs the prose check derives from this text.
func TestFactsTextNoRecordMinutes(t *testing.T) {
	txt := FactsText(Facts{RangeStart: "2026-08-20", RangeEnd: "2026-09-23", Days: 35, Minutes: -1})
	if strings.Contains(txt, "1") {
		t.Fatalf("facts text renders the -1 sentinel: %s", txt)
	}
	if !strings.Contains(txt, "学习时长 无记录") {
		t.Fatalf("facts text missing 学习时长 无记录: %s", txt)
	}
}

func TestFactsTextTitlesAndMoments(t *testing.T) {
	f := Facts{
		RangeStart: "2026-08-17", RangeEnd: "2026-09-13", Days: 28,
		Readings: []Item{{Kind: "reading", Title: "海洋塑料", FinishedAt: "2026-09-01"}},
		Moments:  []Moment{{Quote: "塑料不会消失，只会变小", ItemTitle: "海洋塑料"}},
		Keywords: []Keyword{{Text: "海洋生态", Field: "science", FieldLabel: "科学与自然"}},
	}
	txt := FactsText(f)
	for _, want := range []string{"完成阅读《海洋塑料》", "「塑料不会消失，只会变小」（《海洋塑料》）", "新关键词 海洋生态（科学与自然）"} {
		if !strings.Contains(txt, want) {
			t.Errorf("facts text missing %q: %s", want, txt)
		}
	}
}

// The keyword list is capped at 12 by the query, so its length is not her
// real count and must not appear as a number the prose check would accept.
// Twenty keywords with no digits of their own: the text must have no 个
// count line and no run "20" or "12".
func TestFactsTextHasNoKeywordCount(t *testing.T) {
	kws := make([]Keyword, 20)
	for i := range kws {
		kws[i] = Keyword{Text: "气候", Field: "science", FieldLabel: "科学与自然"}
	}
	txt := FactsText(Facts{RangeStart: "2026-08-03", RangeEnd: "2026-09-06", Days: 35, Minutes: 40, Keywords: kws})
	if strings.Contains(txt, "关键词 20") || strings.Contains(txt, "新增关键词") {
		t.Fatalf("facts text counts keywords: %s", txt)
	}
	for _, run := range []string{"20 ", "12"} {
		if strings.Contains(txt, run) {
			t.Fatalf("facts text carries %q from len(Keywords): %s", run, txt)
		}
	}
	// 完成项目 N 个 is the only 个 line allowed.
	if strings.Count(txt, "个") != 1 {
		t.Fatalf("facts text has a 个 count line besides 完成项目: %s", txt)
	}
}

// A finished item's date must not widen the allowed digits. Every other
// value is chosen without the digit 5, so a "5" can only come from the
// item's 2026-09-05 finish date.
func TestFactsTextHasNoItemDates(t *testing.T) {
	f := Facts{
		RangeStart: "2026-08-03", RangeEnd: "2026-09-03", Days: 32,
		ActiveDays: 9, Minutes: 60, Turns: 8,
		Readings: []Item{{Kind: "reading", Title: "海洋塑料", FinishedAt: "2026-09-05"}},
	}
	txt := FactsText(f)
	if strings.Contains(txt, "5") {
		t.Fatalf("facts text carries the item finish date: %s", txt)
	}
	if !strings.Contains(txt, "完成阅读《海洋塑料》") {
		t.Fatalf("facts text missing the item title: %s", txt)
	}
}

func TestCorporaAreSeparate(t *testing.T) {
	f := Facts{
		Readings: []Item{{Title: "海洋塑料"}},
		Writings: []Item{{Title: "一场雨"}},
		Projects: []Item{{Title: "校园堆肥"}},
		Moments:  []Moment{{Quote: "雨停了才听见自己", ItemTitle: "一场雨"}},
		Keywords: []Keyword{{Text: "气候"}},
	}
	corpus := Corpus(f)
	for _, title := range []string{"海洋塑料", "一场雨", "校园堆肥"} {
		if strings.Contains(corpus, title) {
			t.Errorf("Corpus contains title %q: %q", title, corpus)
		}
	}
	for _, want := range []string{"雨停了才听见自己", "气候"} {
		if !strings.Contains(corpus, want) {
			t.Errorf("Corpus missing %q: %q", want, corpus)
		}
	}
	titles := TitleCorpus(f)
	for _, own := range []string{"雨停了才听见自己", "气候"} {
		if strings.Contains(titles, own) {
			t.Errorf("TitleCorpus contains her words %q: %q", own, titles)
		}
	}
	if titles != "海洋塑料\n一场雨\n校园堆肥" {
		t.Errorf("TitleCorpus = %q", titles)
	}
}

// Ruling 18 C1: the digit text leaves out quotes, titles and the class name;
// FactsText, which the model reads, keeps them.
func TestDigitFactsText(t *testing.T) {
	f := Facts{
		StudentName: "林知遥", ClassName: "高一（3）班", RangeStart: "2026-08-17", RangeEnd: "2026-09-13", Days: 28,
		ActiveDays: 5, Minutes: 64, Turns: 31,
		Readings: []Item{{Kind: "reading", Title: "第7课", FinishedAt: "2026-08-20"}},
		Moments:  []Moment{{Quote: "我读了46遍", ItemTitle: "第7课"}},
	}
	full := FactsText(f)
	for _, s := range []string{"高一（3）班", "《第7课》", "「我读了46遍」"} {
		if !strings.Contains(full, s) {
			t.Fatalf("FactsText lacks %q: %s", s, full)
		}
	}
	digits := DigitFactsText(f)
	for _, s := range []string{"高一", "第7课", "46", "（3）"} {
		if strings.Contains(digits, s) {
			t.Fatalf("DigitFactsText carries %q: %s", s, digits)
		}
	}
	for _, s := range []string{"2026-08-17", "活跃 5 天", "学习 64 分钟", "对话 31 轮", "完成阅读 1 篇"} {
		if !strings.Contains(digits, s) {
			t.Fatalf("DigitFactsText lacks %q: %s", s, digits)
		}
	}
}

// VisibleFacts drops hidden moments by exact quote and hidden keywords by exact
// text, keeps every other field, and ignores an entry that matches nothing.
// Whether an entry is allowed at all is UnknownHidden's job (PATCH refuses
// it); the pure filter only filters.
func TestVisibleFacts(t *testing.T) {
	base := Facts{
		StudentName: "林知遥", Days: 28, ActiveDays: 5,
		Readings: []Item{{Kind: "reading", Title: "城市里的雨水花园"}},
		Moments: []Moment{
			{Quote: "雨水不是废水", ItemTitle: "城市里的雨水花园"},
			{Quote: "雨把街道洗亮了", ItemTitle: "一场雨"},
		},
		Keywords: []Keyword{
			{Text: "海绵城市", Field: "society", FieldLabel: "社会"},
			{Text: "气候", Field: "science", FieldLabel: "科学与自然"},
		},
	}
	quotes := func(f Facts) []string {
		out := []string{}
		for _, m := range f.Moments {
			out = append(out, m.Quote)
		}
		return out
	}
	words := func(f Facts) []string {
		out := []string{}
		for _, k := range f.Keywords {
			out = append(out, k.Text)
		}
		return out
	}
	for _, tc := range []struct {
		name         string
		hidden       Hidden
		wantQuotes   []string
		wantWords    []string
		wantSections []string
	}{
		{"nothing hidden", Hidden{}, []string{"雨水不是废水", "雨把街道洗亮了"}, []string{"海绵城市", "气候"},
			[]string{"overview", "reading", "interests", "next"}},
		{"one moment hidden", Hidden{Moments: []string{"雨水不是废水"}}, []string{"雨把街道洗亮了"}, []string{"海绵城市", "气候"},
			[]string{"overview", "reading", "interests", "next"}},
		{"a keyword hidden", Hidden{Keywords: []string{"气候"}}, []string{"雨水不是废水", "雨把街道洗亮了"}, []string{"海绵城市"},
			[]string{"overview", "reading", "interests", "next"}},
		{"all keywords hidden", Hidden{Keywords: []string{"海绵城市", "气候"}}, []string{"雨水不是废水", "雨把街道洗亮了"}, []string{},
			[]string{"overview", "reading", "next"}},
		// A quote is matched exactly: a substring, a keyword text in the
		// moments list, and an unknown text all match nothing.
		{"unknown text ignored", Hidden{Moments: []string{"雨水", "海绵城市", "编造的话"}, Keywords: []string{"雨水不是废水"}},
			[]string{"雨水不是废水", "雨把街道洗亮了"}, []string{"海绵城市", "气候"},
			[]string{"overview", "reading", "interests", "next"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := VisibleFacts(base, tc.hidden)
			if !reflect.DeepEqual(quotes(got), tc.wantQuotes) || !reflect.DeepEqual(words(got), tc.wantWords) {
				t.Fatalf("visible = %v %v, want %v %v", quotes(got), words(got), tc.wantQuotes, tc.wantWords)
			}
			if got.Moments == nil || got.Keywords == nil {
				t.Fatalf("lists must be non-nil: %#v", got)
			}
			if got.StudentName != base.StudentName || got.Days != base.Days || !reflect.DeepEqual(got.Readings, base.Readings) {
				t.Fatalf("other fields changed: %+v", got)
			}
			if s := SectionsWithFacts(got); !reflect.DeepEqual(s, tc.wantSections) {
				t.Fatalf("sections = %v, want %v", s, tc.wantSections)
			}
		})
	}
	// The input is not modified.
	if len(base.Moments) != 2 || len(base.Keywords) != 2 {
		t.Fatalf("VisibleFacts modified its input: %+v", base)
	}
	// FactsText and Corpus of the visible facts leave a hidden quote and
	// keyword out.
	v := VisibleFacts(base, Hidden{Moments: []string{"雨水不是废水"}, Keywords: []string{"气候"}})
	for _, s := range []string{FactsText(v), Corpus(v), DigitFactsText(v)} {
		if strings.Contains(s, "雨水不是废水") || strings.Contains(s, "气候") {
			t.Fatalf("visible text carries a hidden item: %s", s)
		}
	}
}

func TestUnknownHidden(t *testing.T) {
	f := Facts{
		Moments:  []Moment{{Quote: "雨水不是废水"}},
		Keywords: []Keyword{{Text: "海绵城市"}},
	}
	for _, tc := range []struct {
		name   string
		hidden Hidden
		want   string
		ok     bool
	}{
		{"empty", Hidden{}, "", false},
		{"known", Hidden{Moments: []string{"雨水不是废水"}, Keywords: []string{"海绵城市"}}, "", false},
		{"unknown moment", Hidden{Moments: []string{"编造的话"}}, "编造的话", true},
		{"substring of a quote", Hidden{Moments: []string{"雨水"}}, "雨水", true},
		{"keyword text as a moment", Hidden{Moments: []string{"海绵城市"}}, "海绵城市", true},
		{"quote as a keyword", Hidden{Keywords: []string{"雨水不是废水"}}, "雨水不是废水", true},
	} {
		got, ok := UnknownHidden(f, tc.hidden)
		if got != tc.want || ok != tc.ok {
			t.Errorf("%s: UnknownHidden = %q %v, want %q %v", tc.name, got, ok, tc.want, tc.ok)
		}
	}
}

func TestHiddenMentions(t *testing.T) {
	f := Facts{
		Readings: []Item{{Title: "城市里的雨水花园"}},
		Moments:  []Moment{{Quote: "雨水不是废水"}, {Quote: "雨把街道洗亮了"}},
		Keywords: []Keyword{{Text: "海绵城市"}, {Text: "气候"}, {Text: "雨"}},
	}
	body := map[string]string{
		"overview":  "她写下「雨水不是废水」，又写下「雨把街道洗亮了」，还写下「雨水不是废水」。",
		"reading":   "读完《城市里的雨水花园》。",
		"interests": "她关注海绵城市和气候，也写下「雨水不是废水」。",
		"next":      "请和她聊一聊下雨天。",
	}
	sectionsFor := func(h Hidden) []string { return SectionsWithFacts(VisibleFacts(f, h)) }
	for _, tc := range []struct {
		name   string
		hidden Hidden
		want   map[string][]string
	}{
		{"nothing hidden", Hidden{}, map[string][]string{}},
		{"hidden quote in overview", Hidden{Moments: []string{"雨水不是废水"}},
			map[string][]string{"overview": {"雨水不是废水"}, "interests": {"雨水不是废水"}}},
		{"hidden-list order, no duplicates", Hidden{Moments: []string{"雨把街道洗亮了", "雨水不是废水"}},
			map[string][]string{"overview": {"雨把街道洗亮了", "雨水不是废水"}, "interests": {"雨水不是废水"}}},
		// Every keyword hidden: interests is not a visible section, so the quote
		// in it is not flagged; overview still is.
		{"section not visible", Hidden{Moments: []string{"雨水不是废水"}, Keywords: []string{"海绵城市", "气候", "雨"}},
			map[string][]string{"overview": {"雨水不是废水"}}},
		{"one keyword of several", Hidden{Keywords: []string{"气候"}},
			map[string][]string{"interests": {"气候"}}},
		{"1-rune keyword ignored", Hidden{Keywords: []string{"雨"}}, map[string][]string{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := HiddenMentions(body, sectionsFor(tc.hidden), f, tc.hidden)
			if got == nil || !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("HiddenMentions = %#v, want %#v", got, tc.want)
			}
		})
	}
	if got := HiddenMentions(nil, SectionKeys, f, Hidden{Moments: []string{"雨水不是废水"}}); got == nil || len(got) != 0 {
		t.Fatalf("nil body = %#v, want an empty non-nil map", got)
	}
}

// TestHiddenMentionsQuotedFragment (final fix A): the prose check accepts a
// 「」 or “” quote that is only part of a 金句, so a quoted fragment of a
// hidden moment is flagged too, listed as the full quote.
func TestHiddenMentionsQuotedFragment(t *testing.T) {
	f := Facts{
		Readings: []Item{{Title: "中国与碳排放"}},
		Moments:  []Moment{{Quote: "中国碳排放总量第一，但人均不高"}, {Quote: "光伏板装机量世界第一"}},
		Keywords: []Keyword{{Text: "新能源"}},
	}
	sectionsFor := func(h Hidden) []string { return SectionsWithFacts(VisibleFacts(f, h)) }
	for _, tc := range []struct {
		name   string
		body   map[string]string
		hidden Hidden
		want   map[string][]string
	}{
		{"「」 fragment of a hidden moment",
			map[string]string{"overview": "她写下「碳排放总量第一」。"},
			Hidden{Moments: []string{"中国碳排放总量第一，但人均不高"}},
			map[string][]string{"overview": {"中国碳排放总量第一，但人均不高"}}},
		{"“” fragment of a hidden moment, listed once with a second fragment",
			map[string]string{"reading": "她认为“人均不高”，又说“碳排放总量第一”。"},
			Hidden{Moments: []string{"中国碳排放总量第一，但人均不高"}},
			map[string][]string{"reading": {"中国碳排放总量第一，但人均不高"}}},
		{"fragment nested in the other quote family",
			map[string]string{"next": "请回顾「她说“人均不高”那一段」。"},
			Hidden{Moments: []string{"中国碳排放总量第一，但人均不高"}},
			map[string][]string{"next": {"中国碳排放总量第一，但人均不高"}}},
		{"fragment of a visible moment is not flagged",
			map[string]string{"overview": "她写下「装机量世界第一」。"},
			Hidden{Moments: []string{"中国碳排放总量第一，但人均不高"}},
			map[string][]string{}},
		{"1-rune span ignored",
			map[string]string{"overview": "她只圈了一个「中」字。"},
			Hidden{Moments: []string{"中国碳排放总量第一，但人均不高"}},
			map[string][]string{}},
		{"unquoted fragment is not flagged",
			map[string]string{"overview": "她注意到碳排放总量第一这件事。"},
			Hidden{Moments: []string{"中国碳排放总量第一，但人均不高"}},
			map[string][]string{}},
		{"unclosed mark does not hide a later span",
			map[string]string{"overview": "「未闭合，后面是「人均不高」。"},
			Hidden{Moments: []string{"中国碳排放总量第一，但人均不高"}},
			map[string][]string{"overview": {"中国碳排放总量第一，但人均不高"}}},
		// Every keyword hidden: interests is not a visible section.
		{"span in a non-visible section ignored",
			map[string]string{"interests": "她关注「人均不高」。"},
			Hidden{Moments: []string{"中国碳排放总量第一，但人均不高"}, Keywords: []string{"新能源"}},
			map[string][]string{}},
		{"hidden-list order kept across whole and fragment matches",
			map[string]string{"overview": "「装机量」之后，她写下「总量第一，但人均」。"},
			Hidden{Moments: []string{"光伏板装机量世界第一", "中国碳排放总量第一，但人均不高"}},
			map[string][]string{"overview": {"光伏板装机量世界第一", "中国碳排放总量第一，但人均不高"}}},
		// Keywords keep the plain substring rule; a quoted part of a hidden
		// keyword is not a fragment match.
		{"keyword: quoted part not flagged",
			map[string]string{"overview": "她提到「能源」。"},
			Hidden{Keywords: []string{"新能源"}},
			map[string][]string{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := HiddenMentions(tc.body, sectionsFor(tc.hidden), f, tc.hidden)
			if got == nil || !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("HiddenMentions = %#v, want %#v", got, tc.want)
			}
		})
	}
}

func TestHiddenNormalize(t *testing.T) {
	got := Hidden{Moments: []string{"b", "a", "b"}}.Normalize()
	if !reflect.DeepEqual(got.Moments, []string{"b", "a"}) || got.Keywords == nil || len(got.Keywords) != 0 {
		t.Fatalf("Normalize = %#v", got)
	}
}

// Ruling 18 B: a moment naming a classmate is dropped; names under 2 runes
// are ignored.
func TestDropMomentsNaming(t *testing.T) {
	ms := []Moment{
		{Quote: "我和王小明一起做实验", ItemTitle: "a"},
		{Quote: "雨水不是废水", ItemTitle: "b"},
		{Quote: "明天再读", ItemTitle: "c"},
	}
	got := DropMomentsNaming(ms, []string{"王小明", "明", ""})
	if want := []Moment{ms[1], ms[2]}; !reflect.DeepEqual(got, want) {
		t.Fatalf("DropMomentsNaming = %+v, want %+v", got, want)
	}
	if got := DropMomentsNaming([]Moment{}, nil); got == nil || len(got) != 0 {
		t.Fatalf("empty input must give an empty, non-nil list: %#v", got)
	}
}
