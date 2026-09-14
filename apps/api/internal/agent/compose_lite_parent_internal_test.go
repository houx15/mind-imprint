package agent

import (
	"strings"
	"testing"

	"mindimprint/api/internal/liteweekly"
)

func TestStripListMarkers(t *testing.T) {
	cases := []struct{ name, in, want string }{
		{"dot and space", "1. 请她讲一讲文章", "请她讲一讲文章"},
		{"dunhao", "2、请和她安排时间", "请和她安排时间"},
		{"full-width parens", "（3）请陪她读", "请陪她读"},
		{"ascii parens", "(1) 请陪她读", "请陪她读"},
		{"full-width digit and dot", "１．请陪她读", "请陪她读"},
		{"right paren", "2) 请陪她读", "请陪她读"},
		{"leading spaces", "  10. 请陪她读", "请陪她读"},
		{"several lines", "1. 甲\n2. 乙\n3. 丙", "甲\n乙\n丙"},
		{"mid-line count kept", "这段时间完成 3 篇", "这段时间完成 3 篇"},
		{"year at line start kept", "2026年8月17日开始", "2026年8月17日开始"},
		{"month at line start kept", "12月3日完成", "12月3日完成"},
		{"decimal kept", "1.5 小时", "1.5 小时"},
		{"three digits kept", "100. 条", "100. 条"},
		{"digit then text kept", "3 篇文章", "3 篇文章"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := stripListMarkers(tc.in); got != tc.want {
				t.Fatalf("stripListMarkers(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

// Stripping the marker must not hide a count the facts do not have.
func TestStripListMarkersKeepsInventedCount(t *testing.T) {
	check := liteweekly.ProseCheck{FactsText: "活跃 5 天；学习 64 分钟"}
	err := liteweekly.CheckProse(stripListMarkers("1. 完成 7 篇\n2. 请陪她读"), nil, check)
	if err == nil || !strings.Contains(err.Error(), "digit not in facts: 7") {
		t.Fatalf("err = %v, want digit not in facts: 7", err)
	}
	if err := liteweekly.CheckProse(stripListMarkers("1. 活跃 5 天\n2. 学习 64 分钟"), nil, check); err != nil {
		t.Fatalf("markers with real counts: %v", err)
	}
}

// Ruling 18 C2: a Chinese-numeral count fails outside quotes and titles; 一
// and text inside 「」 / 《》 pass.
func TestCheckChineseCounts(t *testing.T) {
	pass := []string{
		"建议下一阶段读一篇观点相反的文章。",
		"请和她一起读。",
		"她写下「我读了三遍」。",
		"她写下「我读了三篇」。",
		"她读了《十万个为什么》。",
		"她读完《三个火枪手》。",
		"这段时间活跃 5 天。",
		"第一天",
	}
	for _, s := range pass {
		if err := checkChineseCounts(s); err != nil {
			t.Errorf("checkChineseCounts(%q) = %v, want pass", s, err)
		}
	}
	fail := []struct{ in, frag string }{
		{"她完成了三篇阅读。", "三篇"},
		{"到期作业两份。", "两份"},
		{"每天学习二十分钟。", "二十分钟"},
		{"连续十二天。", "十二天"},
		{"三十五轮对话", "三十五轮"},
		{"十一篇", "十一篇"},
		{"三个小时", "三个小时"},
		{"她读完三个火枪手。", "三个"},
		{"「我读了三遍」之后又读了四本书", "四本"},
		{"未闭合「引号里写了两次", "两次"},
	}
	for _, c := range fail {
		err := checkChineseCounts(c.in)
		if err == nil || err.Error() != "chinese numeral count: "+c.frag {
			t.Errorf("checkChineseCounts(%q) = %v, want chinese numeral count: %s", c.in, err, c.frag)
		}
	}
}

func TestReplaceOutsideSpans(t *testing.T) {
	got := replaceOutsideSpans("高一（3）班的她写下「高一（3）班真好」。", "高一（3）班", "\n")
	if want := "\n的她写下「高一（3）班真好」。"; got != want {
		t.Fatalf("replaceOutsideSpans = %q, want %q", got, want)
	}
}

func TestNormalizeSectionKeys(t *testing.T) {
	p := map[string]string{"Overview ": "a", " NEXT": "b", "reading": "c"}
	if err := normalizeSectionKeys(p); err != nil {
		t.Fatal(err)
	}
	if len(p) != 3 || p["overview"] != "a" || p["next"] != "b" || p["reading"] != "c" {
		t.Fatalf("keys = %v", p)
	}
	dup := map[string]string{"overview": "a", "Overview": "b"}
	if err := normalizeSectionKeys(dup); err == nil || err.Error() != "duplicate section: overview" {
		t.Fatalf("duplicate = %v", err)
	}
}
