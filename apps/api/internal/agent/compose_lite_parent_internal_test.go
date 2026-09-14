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
