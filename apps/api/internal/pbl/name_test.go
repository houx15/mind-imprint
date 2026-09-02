package pbl_test

import (
	"strings"
	"testing"

	"mindimprint/api/internal/pbl"
)

// 名字先由我们给一个，她随时能改。这里只有一条要求：**短**——整句原文顶在
// 页头上会把整个房间挤没，而看板上会变成一大段话。
func TestDefaultProjectName_ShortEnoughToSitInAHeader(t *testing.T) {
	cases := []struct{ idea, want string }{
		{"我们学校每天剩好多饭，我想弄明白这些饭最后去哪了，能不能少一点。", "我们学校每天剩好多饭"},
		{"大家总是在小区迷路", "大家总是在小区迷路"},
		{"想做个网站。", "想做个网站"},
		// 第一小节本身就超长时，截断并留一个省略号。
		{"I want to know why people get lost, honestly", "I want to know…"},
	}
	for _, c := range cases {
		got := pbl.DefaultProjectName(c.idea)
		if got != c.want {
			t.Fatalf("DefaultProjectName(%q) = %q, want %q", c.idea, got, c.want)
		}
		if len([]rune(got)) > 15 {
			t.Fatalf("%q 太长了，页头放不下", got)
		}
	}
}

// 一句没有标点的长句也要能收住，不能整句顶上去。
func TestDefaultProjectName_TruncatesAnUnpunctuatedRun(t *testing.T) {
	got := pbl.DefaultProjectName(strings.Repeat("食", 40))
	if len([]rune(got)) > 15 {
		t.Fatalf("%q 有 %d 个字，太长", got, len([]rune(got)))
	}
	if !strings.HasSuffix(got, "…") {
		t.Fatalf("截断了却没有省略号：%q", got)
	}
}

// 空的、或者只有标点的，也要给得出一个名字——看板上不能出现一张没名字的卡。
func TestDefaultProjectName_NeverEmpty(t *testing.T) {
	for _, idea := range []string{"", "   ", "。", "，，，"} {
		if got := pbl.DefaultProjectName(idea); strings.TrimSpace(got) == "" {
			t.Fatalf("DefaultProjectName(%q) 给了空名字", idea)
		}
	}
}
