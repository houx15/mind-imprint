package api

import (
	"strings"
	"testing"
)

// 四格各拿各的毛病表，而英文记叙那一格必须拿到它自己那五条。
//
// 🚨 这条的意义在 2026-09-23 之后才成立。四期 Task 2 给 {write, en, narrative}
// 登记了 5 条英文记叙专属的毛病（show-don't-tell、时态跳变、filtering、
// 对话标点、and-then 连接单一），但那一格当时**根本到不了** ——
// writingGenreOf 的记叙文词表全是中文子串，每一篇英文都被判成议论文，
// 所以这 5 条从登记那天起就是死代码，而所有测试都绿着。
// 词表补上英文之后（见 TestWritingGenreNarrativeMarkersReachEnglish），
// 这条才真的在守一件线上会发生的事。
func TestSymptomCatalogDiffersPerCellAndReachesEnglishNarrative(t *testing.T) {
	cat := map[string]string{}
	for _, lang := range []string{"zh", langEnglish} {
		for _, genre := range []string{genreArgument, genreNarrative} {
			c := strings.TrimSpace(writingSymptomCatalog(lang, genre))
			if c == "" {
				t.Fatalf("%s/%s 的毛病表是空的", lang, genre)
			}
			cat[lang+"/"+genre] = c
		}
	}

	// 四格两两不同 —— 同一张表发给四种稿子，等于文体和语言这两条轴都没接。
	keys := []string{"zh/argument", "zh/narrative", "en/argument", "en/narrative"}
	for i := 0; i < len(keys); i++ {
		for j := i + 1; j < len(keys); j++ {
			if cat[keys[i]] == cat[keys[j]] {
				t.Errorf("%s 和 %s 拿到的是同一张毛病表", keys[i], keys[j])
			}
		}
	}

	// 英文记叙那五条要逐字在场，并且**只**在那一格。
	for _, id := range []string{
		"showing_vs_telling", "tense_drift", "filtering_distance",
		"dialogue_mechanics", "connector_monotony",
	} {
		if !strings.Contains(cat["en/narrative"], id) {
			t.Errorf("英文记叙那一格少了「%s」", id)
		}
		if strings.Contains(cat["en/argument"], id) {
			t.Errorf("英文议论那一格拿到了记叙专属的「%s」", id)
		}
	}
}
