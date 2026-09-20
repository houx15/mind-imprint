package api

import (
	"strings"

	"mindimprint/api/internal/store/sqlc"
)

// 这一篇是议论文还是记叙文。
//
// # 🚨 不问她
//
// writing_setup.go 开头那段写着，这是产品的明确要求：
//
//	「**没有文体单选** —— 这是产品的明确要求：学生未必知道「文体」……
//	  由模型去判断这是议论还是记叙。」
//
// 所以这里做的是**推断**，不是加一个下拉。
//
// # 默认议论文，而且这个方向是故意的
//
// 错的代价不对称：这间屋子是按议论文搭的（中心论点 / 分论点 / 论据 是它的
// 骨架）。把一篇议论文当成记叙文，她会拿到一整套用不上的引导 —— 细节描写、
// 先抑后扬 —— 而她真正需要的分论点四角度一条都不给。反过来只是少拿到几条
// 记叙文的词，她照样写得下去。所以拿不准就是 argument。
//
// # 判据的顺序：先看板上已经有什么，再看题目
//
// 板上的东西是**她自己摆的**，比题目里的几个字硬。一篇题为「记一次……」的
// 作业，如果她已经摆出了中心论点和三条分论点，那她在写的就是议论文，
// 不管题目长什么样。
// # 用的是阅读室那张体裁闭表里的两个词
//
// `genreArgument` / `genreNarrative` 在 reading_outline.go 里已经定了（同一个
// 包）。写作面只用得上这两个 —— 阅读室那张表还有 report 和 explain，那是
// 「读到的文章是什么」，不是「她在写什么」。不另起一套名字：同一个值两个
// 名字，迟早有人在一处改了另一处没改。
//
// vocab 里也有一份同样的两个词，因为 methods.json 的 `genre` 字段是它在读
//（理由同 course.audience：两处词表各自成立，不写 DB CHECK）。两处一字不差
// 由 TestWritingGenreWordsMatchVocab 钉住。

// 题目里出现这些词，才考虑记叙文。
//
// 🚨 这张表只在**板上什么都还没有**的时候起作用，所以它宁可漏不可多：
// 判错一篇的代价见上面那段。「故事」「经历」这类词在议论文的题目里也常出现
// （「用你的经历说明……」），所以收的是记叙文题干里那几个成套的说法。
var narrativeIdeaMarkers = []string{
	"记一次", "记一个", "记我", "写一个人", "写一件事",
	"那一天", "那个人", "难忘的", "最难忘",
	"我和", "有你的日子", "成长的",
	"记叙文", "写人记事", "叙事",
}

// writingGenreOf 推断这一篇的文体。见文件头。
//
// outline 传 nil 也成立（还没摆图的时候），那时只看题目。
func writingGenreOf(wr sqlc.Writing, outline []sqlc.WritingOutline) string {
	// 1. 板上已经有议论文的骨架 —— 她在按议论文摆，不必再猜。
	//    这一条放在最前面：她的动作胜过题目里的字。
	var sawNarrativeKind bool
	for _, row := range outline {
		switch writingKindOf(row) {
		case writingKindThesis, writingKindPoint, writingKindCounter, writingKindRebuttal:
			return genreArgument
		case writingKindScene, writingKindDetail, writingKindTurn, writingKindFeeling:
			sawNarrativeKind = true
		}
	}
	if sawNarrativeKind {
		return genreNarrative
	}

	// 2. 板上还是空的 —— 只好看题目。
	//    老师布置的那句（AssignedPrompt）和她自己的标题都算。
	idea := wr.Title
	if wr.AssignedPrompt != nil {
		idea += " " + *wr.AssignedPrompt
	}
	for _, marker := range narrativeIdeaMarkers {
		if strings.Contains(idea, marker) {
			return genreNarrative
		}
	}

	// 3. 拿不准就是议论文。
	return genreArgument
}

// writingGenreLabel 是她在界面上看得见的那个词。
func writingGenreLabel(genre string) string {
	if genre == genreNarrative {
		return "记叙文"
	}
	return "议论文"
}
