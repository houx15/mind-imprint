package api

import (
	"strings"
	"testing"
)

// 同一个说法说了两遍，要数出来。
func TestWritingRepeatedPhrases_FindsARepeatedChinesePhrase(t *testing.T) {
	text := "我们学校的浪费很严重。每天中午我都看见有人把饭倒掉，" +
		"这说明浪费根本不是一两个人的事。后来我又去看了一次，" +
		"还是有人把饭倒掉，这说明浪费根本不是一两个人的事。"

	got := writingRepeatedPhrases(text, "zh")
	if len(got) == 0 {
		t.Fatal("同一句话说了两遍，一条都没数出来")
	}
	found := false
	for _, r := range got {
		if strings.Contains(r.Phrase, "浪费根本不是一两个人的事") && r.Count >= 2 {
			found = true
		}
	}
	if !found {
		t.Fatalf("没找到那句重复的话：%+v", got)
	}
}

// 🚨 被更长那条包住的短片段，不要再报一遍 —— 否则同一件事会说三四遍，
// 把上文撑大、把真正的问题挤掉。
func TestWritingRepeatedPhrases_DropsFragmentsCoveredByALongerOne(t *testing.T) {
	text := "这说明浪费根本不是一两个人的事。中间随便写点别的东西凑一凑长度。" +
		"这说明浪费根本不是一两个人的事。"

	got := writingRepeatedPhrases(text, "zh")
	for i, a := range got {
		for j, b := range got {
			if i == j {
				continue
			}
			if a.Count == b.Count && strings.Contains(a.Phrase, b.Phrase) {
				t.Fatalf("「%s」被「%s」包住了，还报了两条：%+v", b.Phrase, a.Phrase, got)
			}
		}
	}
}

// 一段没有重复的文字，一条都不该报（否则每一轮上文里都挂着噪音）。
func TestWritingRepeatedPhrases_QuietWhenNothingRepeats(t *testing.T) {
	text := "上周五中午我在收餐台数了二十分钟，四十多个人把饭倒进桶里。" +
		"前面那个男生红烧肉一口没动，米饭扒拉两口就全倒了。"
	if got := writingRepeatedPhrases(text, "zh"); len(got) != 0 {
		t.Fatalf("没有重复却报了出来：%+v", got)
	}
	if got := writingRepeatBlock(text, "zh"); got != "" {
		t.Fatalf("没有重复的时候这一段该是空的：%q", got)
	}
}

// 英文按词数，不按字符。
func TestWritingRepeatedPhrases_English(t *testing.T) {
	text := "The bucket was full again today. I think this is a serious problem. " +
		"Some students threw away the whole plate. I think this is a serious problem."

	got := writingRepeatedPhrases(text, langEnglish)
	found := false
	for _, r := range got {
		if strings.Contains(r.Phrase, "i think this is a serious problem") && r.Count >= 2 {
			found = true
		}
	}
	if !found {
		t.Fatalf("英文那句重复没数出来：%+v", got)
	}
}

// 标点变了仍然算同一句 —— 同一句话第二次出现时最常变的就是句末那个标点。
func TestWritingRepeatedPhrases_IgnoresPunctuation(t *testing.T) {
	text := "浪费根本不是一两个人的事，这一点我看得很清楚。" +
		"中间写一点别的，换个话题说两句。" +
		"浪费根本不是一两个人的事！"
	got := writingRepeatedPhrases(text, "zh")
	if len(got) == 0 {
		t.Fatalf("只差一个标点就没认出来：%+v", got)
	}
}

// 那一段给模型的话必须说清楚「重复不一定是毛病」——
// 排比是重复，议论文里关键词本来就该反复出现。我们只报事实，不在代码里判文章。
func TestWritingRepeatBlock_SaysRepetitionIsNotAutomaticallyAFault(t *testing.T) {
	text := "这说明浪费根本不是一两个人的事。随便写点别的凑长度。" +
		"这说明浪费根本不是一两个人的事。"
	got := writingRepeatBlock(text, "zh")
	if got == "" {
		t.Fatal("有重复却没渲染出来")
	}
	if !strings.Contains(got, "不一定") {
		t.Errorf("没说清重复不一定是毛病：\n%s", got)
	}
}
