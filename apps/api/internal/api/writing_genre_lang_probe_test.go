package api

import (
	"testing"

	"mindimprint/api/internal/store/sqlc"
)

// 文体这条轴在**英文**那边也要活着。
//
// 🚨 2026-09-23 之前它是死的：narrativeIdeaMarkers 全是中文子串，
// strings.Contains 在一句英文题目上永远不命中 ⇒ 每一篇英文都落到
// `return genreArgument`。一个写英文记叙文的学生因此拿到 thesis
// statement / topic sentence 和 TOPIC/TASK 拆解。
//
// 这条测试钉的是**两个方向**，不是一个。只钉「英文记叙文判得出来」，
// 下一个人往表里加一个 "a day" 就能让它继续绿，同时把
// 「Should schools start the day later?」判成记叙文 —— 而按这个文件
// 开头写的，那才是代价大的那个方向。
func TestWritingGenreNarrativeMarkersReachEnglish(t *testing.T) {
	narrative := []string{
		"Write about a day you will never forget",
		"A narrative essay about my grandmother",
		"Tell the story of a time you failed",
		"Narrative writing: the day everything changed",
		"Describe an experience that changed your mind",
		"Recount a time when you were wrong",
		"My most memorable summer",
		"A memorable teacher",
		"Personal essay: leaving home",
	}
	for _, title := range narrative {
		wr := sqlc.Writing{Title: title, Lang: langEnglish}
		if got := writingGenreOf(wr, nil); got != genreNarrative {
			t.Errorf("英文记叙文题目判成了 %s：%q", got, title)
		}
	}

	// 🚨 反方向。这些是英文**议论文**的题目，一条都不许落进记叙文。
	// 每一条都挑了和上面那张表擦边的词（day / story / experience /
	// time），它们正是过度收词时会先翻车的那几句。
	argument := []string{
		"Should schools start the day later?",
		"Is social media making us lonelier?",
		"Do we need a four-day school week?",
		"Discuss both views: is the story of progress overstated?",
		"To what extent does work experience improve学生's outcomes?",
		"Should students be required to do community service?",
		"Is it time we banned single-use plastics?",
		"Are exams a fair measure of ability?",
	}
	for _, title := range argument {
		wr := sqlc.Writing{Title: title, Lang: langEnglish}
		if got := writingGenreOf(wr, nil); got != genreArgument {
			t.Errorf("英文议论文题目被判成了 %s：%q", got, title)
		}
	}

	// 中文那一侧没有变。
	for _, title := range []string{"记一次难忘的经历", "写一件事", "那一天"} {
		wr := sqlc.Writing{Title: title, Lang: "zh"}
		if got := writingGenreOf(wr, nil); got != genreNarrative {
			t.Errorf("中文记叙文题目判成了 %s：%q", got, title)
		}
	}
	for _, title := range []string{"谈谈坚持的意义", "中国是否让地球变得更可持续"} {
		wr := sqlc.Writing{Title: title, Lang: "zh"}
		if got := writingGenreOf(wr, nil); got != genreArgument {
			t.Errorf("中文议论文题目判成了 %s：%q", got, title)
		}
	}
}
