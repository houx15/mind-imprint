package api

import (
	"testing"

	"mindimprint/api/internal/vocab"
)

// 🚨 vocab 是过滤器不是选择器，所以它不走 guidance.Pick（见 plan Task 6）。
// 但两边的轴必须说同一套词 —— 一边改了另一边没改，症状是某个组合悄悄
// 少给一整块内容，而线上看起来只是印记话变少了。
func TestVocabAxesAgreeWithGuidanceClosedSets(t *testing.T) {
	// 写作面今天用得上的两个体裁，vocab 里都要真的有方法。
	for _, genre := range []string{genreArgument, genreNarrative} {
		for _, lang := range []string{"zh", "en"} {
			if got := vocab.ForLang(lang, genre); len(got) == 0 {
				t.Errorf("vocab.ForLang(%q,%q) 一条方法都没有", lang, genre)
			}
			if got := vocab.Structures(genre, lang); len(got) == 0 {
				t.Errorf("vocab.Structures(%q,%q) 一条结构都没有", genre, lang)
			}
		}
	}
}

// methods.json 里的 genre 字段只许出现闭表里的词（或空＝不限）。
// 现造一个词，就是在同一口气里教模型造词（AGENTS.md 提示词第 5 条）。
func TestVocabGenreFieldStaysInTheClosedSet(t *testing.T) {
	allowed := map[string]bool{
		"": true, genreArgument: true, genreNarrative: true,
		genreReport: true, genreExplain: true,
	}
	for _, m := range vocab.All() {
		if !allowed[m.Genre] {
			t.Errorf("方法 %s 的 genre=%q 不在闭表里", m.ID, m.Genre)
		}
	}
}
