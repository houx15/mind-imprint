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
	if s != "2026-08-17" || e != "2026-09-13" {
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
		"完成阅读 4 篇", "完成写作 5 篇", "完成项目 6 个", "新增关键词 7 个",
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
	for _, want := range []string{"《海洋塑料》（9月1日）", "「塑料不会消失，只会变小」（《海洋塑料》）", "新关键词 海洋生态（科学与自然）"} {
		if !strings.Contains(txt, want) {
			t.Errorf("facts text missing %q: %s", want, txt)
		}
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
