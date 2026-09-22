package awakening

import (
	"strings"
	"testing"
)

// 她的原话那条退路不花调用、不会失败，所以它必须自己站得住 —— 模型那一次
// 没回上来时，线索库里显示的就是它。
func TestFallbackTitleCutsAtHerFirstClause(t *testing.T) {
	cases := map[string]string{
		"最近老是刷到潮汐发电的视频，一个海湾里的闸门一开一合就能发电": "最近老是刷到潮汐发电的视频",
		"我家在海边。小时候赶海要看潮汐表":              "我家在海边",
		"桌游":                            "桌游",
		"   ":                           "",
	}
	for in, want := range cases {
		if got := FallbackTitle(in); got != want {
			t.Errorf("FallbackTitle(%q) = %q, want %q", in, got, want)
		}
	}
}

// 长度上限是给界面的，不是给她的：一行放不下的名字会被界面截断，那等于又切了
// 她的字。所以在这里就裁到位，而且**不加省略号** —— 它是一个标签。
func TestFallbackTitleStaysWithinTheLabelWidth(t *testing.T) {
	long := "我一直在想为什么潮汐这么规律却几乎没有地方用它来发电这件事"
	got := FallbackTitle(long)
	if runeLen(got) > titleMaxRunes {
		t.Errorf("FallbackTitle 太长：%q（%d 字）", got, runeLen(got))
	}
	if strings.Contains(got, "…") || strings.Contains(got, "...") {
		t.Errorf("标签不该带省略号：%q", got)
	}
}

func TestParseTitlesReadsTheModelReply(t *testing.T) {
	got := ParseTitles("```json\n{\"titles\":[\"潮汐发电为什么少\",\"「闸门的节奏」\",\"潮汐发电为什么少\"]}\n```")
	want := []string{"潮汐发电为什么少", "闸门的节奏"}
	if len(got) != len(want) {
		t.Fatalf("ParseTitles = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("ParseTitles = %v, want %v", got, want)
		}
	}
}

// 读不懂就交空表，让调用方退回她的原话 —— 绝不编一个名字
// （memory: ai-errors-must-surface-never-fake）。
func TestParseTitlesReturnsNothingRatherThanInventing(t *testing.T) {
	for _, bad := range []string{"", "我给你起了三个名字：潮汐发电", "{\"titles\":", "{}"} {
		if got := ParseTitles(bad); len(got) != 0 {
			t.Errorf("ParseTitles(%q) = %v, 应当什么都不给", bad, got)
		}
	}
}

// 🚨 她的原话始终是候选之一，而且模型全军覆没时它是唯一那个。
func TestTitleChoicesAlwaysOffersHerOwnWords(t *testing.T) {
	first := "最近老是刷到潮汐发电的视频，看了四十分钟"
	own := FallbackTitle(first)

	withModel := TitleChoices([]string{"潮汐发电为什么少", "闸门的节奏"}, first)
	if len(withModel) != 3 || withModel[2] != own {
		t.Errorf("她的原话应当排在最后：%v", withModel)
	}

	alone := TitleChoices(nil, first)
	if len(alone) != 1 || alone[0] != own {
		t.Errorf("模型没回上来时只剩她的原话，得到 %v", alone)
	}

	// 一轮都还没答：这一步没有东西可挑，界面据此不摆它。
	if got := TitleChoices(nil, ""); len(got) != 0 {
		t.Errorf("没有语料时不该造出候选：%v", got)
	}
}

// 模型重复写出和她原话一样的那个时不该出现两遍。
func TestTitleChoicesDropsDuplicates(t *testing.T) {
	first := "桌游规则总是被改"
	got := TitleChoices([]string{FallbackTitle(first), "规则怎么被改写"}, first)
	if len(got) != 2 {
		t.Errorf("重复的候选没有去掉：%v", got)
	}
}

func TestTitlePromptOnlyCarriesWhatSheWrote(t *testing.T) {
	system, user := BuildTitlePrompt([]string{"潮汐发电的视频", "", "闸门的节奏"})
	if !strings.Contains(user, "潮汐发电的视频") || !strings.Contains(user, "闸门的节奏") {
		t.Errorf("她的原话没进 user 段：\n%s", user)
	}
	// 空格子不进去 —— 一个空编号会让模型替她补一句
	// （同 TestPromptsSkipTheQuestionsSheNeverReached）。
	if strings.Contains(user, "2. \n") {
		t.Errorf("她没答的那一问被带进来了：\n%s", user)
	}
	if !strings.Contains(system, "14") {
		t.Error("长度上限没写进 prompt，模型会写出界面放不下的名字")
	}
}
