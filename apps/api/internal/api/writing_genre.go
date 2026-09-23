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

// narrativeIdeaMarkersEN —— 英文题目里的同一件事，按**小写**比。
//
// 🚨 2026-09-23 实测：在这张表出现之前，文体这条轴在英文那边是**死的**。
// 上面那张表全是中文子串，strings.Contains 在一句英文题目上永远不命中，
// 于是每一篇英文都落到最后那句 `return genreArgument`。
// 「A narrative essay about my grandmother」「Narrative writing: the day
// everything changed」「Tell the story of a time you failed」——
// 五句英文记叙文题目，五句都被判成 argument。后果不是少给几个词：
// 她拿到的是 thesis statement / topic sentence 和 TOPIC/TASK 拆解，
// 一整套议论文的骨架压在一篇记叙文上。
//
// 这张表照上面那条「宁可漏不可多」写：收的是成套的说法，不收单个常用词。
// 「a day」「the day」「memorable」「childhood」这些都**故意没有**——
// 「Should schools start the day later?」里就有「the day」，
// 收了它就会把一篇议论文判成记叙文，而那正是代价大的那个方向。
var narrativeIdeaMarkersEN = []string{
	"narrative",
	// 🚨 没有裸的 "story of"：它在 "Discuss both views: is the story of
	// progress overstated?" 里就命中，把一篇议论文判成记叙文。
	// 这条是写这张表时被下面那个反方向用例当场抓到的。
	"tell the story", "a story about", "your story",
	"personal essay", "memoir", "recount",
	"a time when", "a time you", "a time i",
	"never forget", "unforgettable",
	"most memorable", "a memorable",
	"write about a day", "a day you", "the day you",
	"describe an experience", "an experience you",
}

// —— 书信 ——
//
// 🚨 2026-09-23 产品负责人：「书信 is a very important format in junior
// english. but currently we would guide students to write a letter under the
// structure of 议论文.」她说得对：在这之前这里只有两个取值，一封信必然落到
// 最后那句 `return genreArgument`，于是一个初中生被要求给一封信写中心论点。
//
// 书信和另外两种不一样的地方在于：**它的题目几乎总是自报家门。** 一篇记叙文
// 的题目可以长得像议论文，而一封信的题目里基本都有「信」「Dear」「write to」
// 这样的词 —— 所以这张表可以收得比记叙文那张更实，漏判的风险小得多。
//
// 仍然按「宁可漏不可多」写：收的是成套的说法。「邀请」「建议」这类单个词
// **故意没有** —— 「请给出你的建议」是一道议论文题。
var letterIdeaMarkers = []string{
	"一封信", "写信", "给你的信", "的一封信", "书信",
	"感谢信", "建议信", "邀请函", "倡议书", "申请信", "道歉信", "慰问信", "表扬信",
	"致全体", "致同学", "致老师", "回信", "写一封",
}

// letterIdeaMarkersEN —— 英文那一套，按**小写**比。
//
// 「dear」单独收是安全的：一道议论文题里不会出现它，而一封信的题干和范文
// 开头几乎一定有。「letter」同理。
var letterIdeaMarkersEN = []string{
	"a letter", "the letter", "letter to", "write to", "write a letter",
	"write an email", "an email to", "email to",
	"dear ", "dear sir", "dear madam",
	"thank-you note", "note to",
	"invitation", "apology letter", "application letter", "letter of application",
	"reply to his", "reply to her", "reply to the letter",
}

// writingGenreOf 推断这一篇的文体。见文件头。
//
// outline 传 nil 也成立（还没摆图的时候），那时只看题目。
func writingGenreOf(wr sqlc.Writing, outline []sqlc.WritingOutline) string {
	// 1. 板上已经有议论文的骨架 —— 她在按议论文摆，不必再猜。
	//    这一条放在最前面：她的动作胜过题目里的字。
	var sawNarrativeKind, sawLetterKind bool
	for _, row := range outline {
		switch writingKindOf(row) {
		case writingKindThesis, writingKindPoint, writingKindCounter, writingKindRebuttal:
			return genreArgument
		case writingKindScene, writingKindDetail, writingKindTurn, writingKindFeeling:
			sawNarrativeKind = true
		case writingKindPurpose, writingKindMatter, writingKindCourtesy:
			sawLetterKind = true
		}
	}
	// 书信排在记叙文前面：一封信里可以有一段小小的叙事（「上周我去了……」），
	// 反过来一篇记叙文里不会出现写信目的和结尾的礼貌话。
	if sawLetterKind {
		return genreLetter
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
	lowerIdea := strings.ToLower(idea)
	// 书信先查：「给外婆写一封信，记一件让你难忘的事」两张表都命中，
	// 而它要按信来写（称呼、目的、落款一样都少不了）。
	for _, marker := range letterIdeaMarkers {
		if strings.Contains(idea, marker) {
			return genreLetter
		}
	}
	for _, marker := range letterIdeaMarkersEN {
		if strings.Contains(lowerIdea, marker) {
			return genreLetter
		}
	}
	for _, marker := range narrativeIdeaMarkers {
		if strings.Contains(idea, marker) {
			return genreNarrative
		}
	}
	// 英文那几条按小写比：题目的大小写是她自己敲的，"Narrative" 和
	// "narrative" 不该是两种结果。两张表都查，不看 wr.Lang —— 一篇中文
	// 作业的题目里出现 "narrative essay" 同样说明它是记叙文，而按 Lang
	// 分叉只会多出一条「语言判错了所以文体也跟着判错」的路。
	lower := strings.ToLower(idea)
	for _, marker := range narrativeIdeaMarkersEN {
		if strings.Contains(lower, marker) {
			return genreNarrative
		}
	}

	// 3. 拿不准就是议论文。
	return genreArgument
}

// writingGenreLabel 是她在界面上看得见的那个词。
func writingGenreLabel(genre string) string {
	switch genre {
	case genreNarrative:
		return "记叙文"
	case genreLetter:
		return "书信"
	}
	return "议论文"
}
