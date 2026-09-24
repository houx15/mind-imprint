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
	// 🚨 2026-09-24 补：这一档是**应用文**，不只是信。
	//
	// 补之前，「写一则通知」「写一篇演讲稿」两张表一条都不命中，落到最后那句
	// 「拿不准就是议论文」—— 于是一则通知被要求写中心论点、分论点和论据，
	// 正是产品负责人当初报的那个毛病（「a letter under the structure of
	// 议论文」），只不过换了一种应用文。倡议书本来就在上面这一行里，所以
	// 这一档一直就不只是「信」，只是名单漏了几种。
	"通知", "公告", "启事", "演讲稿", "发言稿", "演讲比赛",
	"邮件", "电子邮件", "投稿", "征文", "调查报告",
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
	// 应用文的其余几种（同上）。这里收的都是**带冠词或动词的整串**，
	// 不收光秃秃的 report / notice / speech —— 那几个词在一道议论文题的
	// 题面里也会出现（"students notice that…"），单收会把议论文判成应用文。
	"a speech", "your speech", "speech contest", "give a speech",
	"a notice", "write a notice", "an announcement",
	"a news report", "news report", "write a report",
	"a proposal to", "an entry for",
}

// continuationIdeaMarkers —— 读后续写的题目词表。
var continuationIdeaMarkers = []string{
	"读后续写", "续写", "故事续写", "接着写下去",
}

// continuationIdeaMarkersEN —— 英文那一套，按小写比。
//
// 🚨 不收光秃秃的 "continue"：「continue to improve…」在一道议论文题里很常见。
var continuationIdeaMarkersEN = []string{
	"continue the story", "continue writing the story", "story continuation",
	"continuation writing", "complete the story",
}

// looksLikeContinuation 判这一道题是不是读后续写。
//
// 🚨 除了词表，还有一条**形式判据**，而且它比词表可靠：读后续写的题面会把
// 两个段首句印出来，惯例是 `Paragraph 1:` / `Paragraph 2:`。两个都在，
// 基本不可能是别的题型 —— 语料里 43 道读后续写全是这个形状
// （distilled/english-letters-genres.md §3.3：「题目同时给出①一段前文
// ②两个段首句③词数要求 150词左右」）。
func looksLikeContinuation(idea, lowerIdea string) bool {
	for _, marker := range continuationIdeaMarkers {
		if strings.Contains(idea, marker) {
			return true
		}
	}
	for _, marker := range continuationIdeaMarkersEN {
		if strings.Contains(lowerIdea, marker) {
			return true
		}
	}
	// 两个段首句同时印着 —— 形式判据。
	return strings.Contains(lowerIdea, "paragraph 1") && strings.Contains(lowerIdea, "paragraph 2")
}

// writingGenreOf 推断这一篇的文体。见文件头。
//
// outline 传 nil 也成立（还没摆图的时候），那时只看题目。
func writingGenreOf(wr sqlc.Writing, outline []sqlc.WritingOutline) string {
	// 0. 🚨 **她自己说过的，压过一切**（迁移 0191）。
	//
	// 产品负责人 2026-09-23：「for writing, maybe we need to let the students
	// select/talk with ai about what genre they are going to write.」
	//
	// 放在最前面，而不是最后当兜底：下面那两条都是**猜**，而这一条是她说的。
	// 一篇散文的题目长得和记叙文一模一样，题目那张表永远猜不出来 ——
	// 所以「让她自己说」不是锦上添花，它是某几种文体唯一到得了的路。
	//
	// 这一条同时也是纠错的出口：判错了她能改，而且改完立刻全屋生效
	// （writing_plan / writing_flow / writing_guide / writing_comment 等
	// 十二处都从这个函数取文体，没有第二个判定点）。
	if g := validateWritingGenre(wr.Genre); g != "" {
		return g
	}

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
	// 🚨 读后续写**最先查**，它和别的几种都会重叠。
	//
	// 一道读后续写题的题面里几乎一定有一段记叙性的前文（记叙文词表会命中），
	// 有时前文本身是一封信或一封邮件（书信词表也会命中）。但它的写法和那两种
	// 都不一样 —— 接住两个印好的段首句、两段均衡、伏笔回收、不说教。
	// 先查它，后面两张表就不会把它抢走。
	if looksLikeContinuation(idea, lowerIdea) {
		return genreContinuation
	}
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
		// 🚨 这一档装的是**应用文**，不只是信：倡议书从一开始就在它的词表里，
		// 2026-09-24 又补进了通知、演讲稿、邮件、投稿。只写「书信」，
		// 一个写通知的学生会看到「印记按书信在教这一篇」，那是句假话。
		return "书信与应用文"
	case genreProse:
		return "散文"
	case genreContinuation:
		return "读后续写"
	}
	return "议论文"
}

// validateWritingGenre 把一个外来的取值收进**写作面**的闭表。
//
// 认不出来就是空串，调用方按「她没说过」处理（回到推断）。
//
// 🚨 和阅读面的 validateGenre 是两张表，故意的：report / explain 是
// 「读到的文章是什么」，她写不出一篇「新闻报道体」的作业交上去；
// 而 letter 反过来不该落到任何一套读法上。同一个包里两个函数，
// 名字里各带自己的面，免得下一个人随手用错那一个。
func validateWritingGenre(s string) string {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case genreArgument:
		return genreArgument
	case genreNarrative:
		return genreNarrative
	case genreLetter:
		return genreLetter
	case genreProse:
		return genreProse
	case genreContinuation:
		return genreContinuation
	}
	return ""
}

// writingGenreChoices 是摆给她挑的那几种，以及每一种的一句说明。
//
// 说明写成「什么时候选它」而不是定义（同 vocab 的 when_to_use）：
// 「记叙文」三个字对一个初中生未必读得懂，但「写一件真实发生过的事」读得懂。
// 这正是 writing_setup.go 当初不做文体单选的那条理由 —— 它没有过时，
// 过时的是「因此干脆不让她选」。
func writingGenreChoices() []writingGenreChoiceDTO {
	return []writingGenreChoiceDTO{
		{ID: genreArgument, Label: "议论文", Blurb: "要说清一个看法，并且给出理由和材料。"},
		{ID: genreNarrative, Label: "记叙文", Blurb: "写一件真实发生过的事，写出当时的场景和你的变化。"},
		{ID: genreLetter, Label: "书信与应用文", Blurb: "写给具体的人或者一群人，要办成一件事：信、邮件、通知、演讲稿、倡议书。"},
		{ID: genreContinuation, Label: "读后续写", Blurb: "给了一段故事的前半截和两个开头句，接着往下写两段。"},
		// 🚨 散文排在最后，而且只在这里出现 —— 它没有题目词表。
		// 一篇散文的题目和一篇记叙文的题目长得一模一样，从字面上分不出来，
		// 所以它**只能由她自己说**。
		{ID: genreProse, Label: "散文", Blurb: "几件不连着的小事，靠一样东西串起来，写出一点体会。"},
	}
}

type writingGenreChoiceDTO struct {
	ID    string `json:"id"`
	Label string `json:"label"`
	Blurb string `json:"blurb"`
}
