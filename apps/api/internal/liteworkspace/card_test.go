package liteworkspace

import "testing"

func TestCleanTitle(t *testing.T) {
	for _, tc := range []struct{ in, want string }{
		{"《AI 当数学家教的正确用法》", "AI 当数学家教的正确用法"}, // live, 2026-09-17
		{" 「手机学习时间调查」 ", "手机学习时间调查"},
		{"读《海洋》后写一篇", "读《海洋》后写一篇"},
		{"《甲》和《乙》", "《甲》和《乙》"},
		{"《》", "《》"},
		{"议论文：学校该不该允许学生用 AI 写作业", "议论文：学校该不该允许学生用 AI 写作业"},
	} {
		if got := CleanTitle(tc.in); got != tc.want {
			t.Errorf("CleanTitle(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestDueLabelAndHumanize(t *testing.T) {
	if got := DueLabel("2026-09-18T21:00"); got != "9月18日（周五）21:00" {
		t.Fatalf("DueLabel = %q", got)
	}
	if got := DueLabel("下周三"); got != "下周三" {
		t.Fatalf("DueLabel(unreadable) = %q", got)
	}
	got := HumanizeDatetimes("截止：2026-09-23T23:59（下周三）")
	if got != "截止：9月23日 23:59（下周三）" {
		t.Fatalf("HumanizeDatetimes = %q", got)
	}
	if got := HumanizeDatetimes("第 2026 年"); got != "第 2026 年" {
		t.Fatalf("HumanizeDatetimes changed a plain number: %q", got)
	}
}

func TestUnfilledClaim(t *testing.T) {
	empty := []CardField{{Label: "驱动问题", Value: ""}, {Label: "标题", Value: "手机学习时间调查"}}
	filled := []CardField{{Label: "驱动问题", Value: "手机是工具还是黑洞？"}}
	for _, tc := range []struct {
		text   string
		fields []CardField
		want   string
	}{
		// Live, 2026-09-17, card field empty each time.
		{"已加上驱动问题：\"我们每天用手机学习的时间到底有多少？\"", empty, "驱动问题"},
		{"好了，驱动问题已写入。还需要改什么吗？", empty, "驱动问题"},
		{"我刚才已经把驱动问题填进去了：「……」。", empty, "驱动问题"},
		{"已经写好了呀——驱动问题是「……」。", empty, "驱动问题"},
		// The same words are true once the field is filled.
		{"驱动问题已写入。", filled, ""},
		// Offers and refusals are not claims.
		{"驱动问题还没有填，要我写一个吗？", empty, ""},
		{"要不要我把驱动问题填上？", empty, ""},
		{"标题已写好。", empty, ""},
		{"作业卡已经填好：\n- 驱动问题：（空）", empty, ""},
		{"标题已经填好，驱动问题还需要您定一个。", empty, ""},
		{"标题和驱动问题都已经填好了。", empty, "驱动问题"},
	} {
		if got := UnfilledClaim(tc.text, tc.fields); got != tc.want {
			t.Errorf("UnfilledClaim(%q) = %q, want %q", tc.text, got, tc.want)
		}
	}
}
