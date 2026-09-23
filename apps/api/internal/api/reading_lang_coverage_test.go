package api

import (
	"strings"
	"testing"
)

// 阅读面：每种语言都要有服务四种体裁的读法，而且语言判得出来。
//
// 阅读这边的语言是**量出来的**（readingLangOf 看正文），不像写作那边只能从
// 题目推 —— 写作那条轴 2026-09-23 之前在英文上整条是死的（见
// TestWritingGenreNarrativeMarkersReachEnglish）。这条钉的是阅读这边不要
// 长出同一个洞：某种语言少一套读法时，pickRoutineForGenre 会「保留模型挑的
// 那一套」，于是一篇英文说明文可能拿到一份按议论文排的清单，而这件事
// 不会报错、也不会有别的测试红。
func TestReadingRoutinesCoverEveryGenreInBothLanguages(t *testing.T) {
	en := strings.Join([]string{
		"The last bus of the night pulled away from the stop just as I reached it.",
		"I had worked late again, and the timetable on the pole said 10:45. My watch said 10:47.",
		"So I started walking. About twenty minutes later I heard an engine behind me.",
	}, "\n\n")
	zh := "那年冬天，我在城郊的工厂上夜班，每天要赶最后一班公交回家。\n\n那天我出来得晚了，跑到站台的时候末班车已经开走。"

	for _, tc := range []struct{ name, body, want string }{
		{"英文正文", en, "en"},
		{"中文正文", zh, "zh"},
	} {
		lang := readingLangOf(tc.body)
		if lang != tc.want {
			t.Errorf("%s 判成了 %q，应该是 %q", tc.name, lang, tc.want)
			continue
		}
		covered := map[string]string{}
		for _, r := range readingRoutinesFor(lang) {
			for _, g := range r.Genres {
				covered[g] = r.Key
			}
		}
		for _, g := range []string{genreArgument, genreReport, genreExplain, genreNarrative} {
			if covered[g] == "" {
				t.Errorf("%s（lang=%s）没有服务「%s」的读法 —— 这一体裁的文章会拿到别的体裁的清单",
					tc.name, lang, g)
			}
		}
	}
}
