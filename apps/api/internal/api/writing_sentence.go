package api

// writing_sentence.go —— 一段里那几句各自的功能。
//
// # 来源
//
// `docs/reference/writing-teaching/议论文写作汇总（五）讲义.md` 第一节，
// 主体段的五句型。讲义把一个范例逐句标了功能：
//
//	①生命需要一盏灯，它便是乐观。（观点句）
//	②乐观犹如夜幕中的一盏灯，鼓励我们勇敢面对困难。（阐释句）
//	③史铁生正是因为有了「乐观」这盏灯……（材料句）
//	④如果史铁生当初选择的是放弃人生，那么他还会有今天的成就吗？（分析句）
//	⑤由此看来，生命需要乐观，因为它能给予我们活下去的勇气。（结论句）
//
// # 为什么值得单独立一个文件
//
// R4 之前，段落那一步给的是四步：分论点句 / 论据 / 分析 / 回扣。少的那一句
// 是**阐释句**，而它恰好是学生最常跳过的一句 —— 观点句写完直接跳到例子，
// 于是抽象的主张和具体的事之间没有桥，读者得自己搭。
//
// 更要紧的是「分析」那一格：R2 上线那天的实测里，模型自己说的是
//
//	「有具体的例子，站得住，但例子讲完就结束了，还差一句把它和主张
//	  连起来的话」
//
// —— 它认得出缺了什么，却说不出那一句叫什么、怎么写。讲义给了它三个名字和
// 三句可套的话（vocab 的 analysis_* 三条）。这张表是把那三条接到位置上的地方。
//
// 🚨 **确定性的，不花模型调用。** 一段的骨架是固定的知识，每次让模型重说
// 一遍既费钱又会漂（同事的意见 7：「每一次刷新就会变成新的东西」）。
//
// 🚨 apps/lite-web/src/writings/paragraphShape.ts 是它的 TS 孪生。
// 两边的步数、次序、用词必须一致，由 TestParagraphShapeMatchesFrontend 钉住。

// writingSentenceRole —— 一段里一句话的功能。
type writingSentenceRole struct {
	// Label 是她在引导框里看见的那个名字，用语文课上的正式词
	//（AGENTS.md 文案规则 6：学生来这儿就是要学这套词）。
	Label string
	// Hint 一句话说清这一句要干什么。
	//
	// 🚨 这些字**直接渲染成纯文本**，不过 markdown。写 `**强调**` 会原样印出
	// 两个星号 —— 2026-09-20 的看图台截图里就是这么发现的。
	Hint string
	// Methods 是这一格用得上的 vocab id。空表示这一格不点名方法。
	Methods []string
}

// 议论文主体段：讲义的五句型。
var writingBodyFiveSentences = []writingSentenceRole{
	{
		Label: "观点句",
		Hint:  "这一段要证的那一句，放在段首。里面要有中心论点的关键词，读者才知道它扣着题。",
	},
	{
		Label: "阐释句",
		Hint:  "把观点句里那个抽象的词说开一层，再去找例子。这一句最常被跳过，跳过了主张和例子之间就没有桥。",
		// 讲义：引用名言 / 运用比喻 / 用逻辑关系 / 运用对比 来阐述分论点。
		Methods: []string{"point_quote", "point_metaphor", "point_reasoning", "point_contrast"},
	},
	{
		Label: "材料句",
		Hint:  "谁、做了什么、结果怎么样。叙述要简明——这一段是论证不是记叙，材料占的篇幅别超过一半。",
		Methods: []string{"point_pee"},
	},
	{
		Label: "分析句",
		Hint:  "这件事凭什么证明观点句。三种写法：只摆了现象就追原因，已经有结论就问假如，列了好几个例子就找共性。",
		// 讲义（五）第三节的三法，各带一句可套的句式。
		Methods: []string{"analysis_cause", "analysis_suppose", "analysis_induce"},
	},
	{
		Label: "结论句",
		Hint:  "回到段首那一句，扣住关键词，说清这一条把中心论点推进了哪一步。",
	},
}

// 议论文的开篇与结尾，以及反方那两块。这几块讲义里没有逐句的表，
// 用的是 R3 定下的那几步，结尾一格补上讲义（六）的四技法。
var (
	writingOpeningSentences = []writingSentenceRole{
		{Label: "引出话题", Hint: "一句话让读者知道为什么值得谈这件事。"},
		{Label: "亮出中心论点", Hint: "这篇要证明的那一句，直接说出来。"},
		{Label: "交代分几条讲", Hint: "一句话说清后面分几条。不必举例——例子在后面的段里。"},
	}
	writingClosingSentences = []writingSentenceRole{
		{Label: "回到中心论点", Hint: "回到开头那一句，但不要原样再说一遍。"},
		{
			Label:   "说得比开头更准",
			Hint:    "用全文证到的东西，把那句话重新说一次。四种收法：回应开头、引一句名言、把几条收成一句、用一组句子收住。",
			Methods: []string{"closing_echo", "closing_quote", "closing_sum", "closing_figure"},
		},
		{Label: "落到一件具体的事上", Hint: "一个可执行的建议，比一句口号有力。讲义的硬话：结尾不超过两百字。"},
	}
	writingCounterSentences = []writingSentenceRole{
		{Label: "反方最强的那一点", Hint: "写它真正有力的版本，不是一个好打的版本。"},
		{Label: "承认它成立的地方", Hint: "承认得越具体，后面的转折越有力。"},
		{Label: "指出它的适用范围", Hint: "它在什么情况下才成立——范围之外就轮到你的结论。"},
	}
	writingRebuttalSentences = []writingSentenceRole{
		{Label: "接住对方那一点", Hint: "先复述你要回应的到底是哪一句。"},
		{Label: "指出它的适用范围", Hint: "不是说它全错，是说它管不到这里。"},
		{Label: "回到自己的结论", Hint: "一句话说清为什么你的说法仍然站得住。"},
	}
)

// 记叙文那四块。来源是两份记叙文讲义：细节描写（人物／环境／物件、
// 精准动词、三禁忌）和抑扬转情法（抑→渡→转→扬）。
var (
	writingSceneSentences = []writingSentenceRole{
		{Label: "交代时间和地点", Hint: "哪一天、在哪儿、当时在做什么。读者要先站得住，才看得见后面的事。"},
		{
			Label:   "把这件事写具体",
			Hint:    "动作、神态、说过的话，挑和主题相关的写。动词要准：「弓着身子蹒跚着走来」和「走过来」是两件事。",
			Methods: []string{"detail_person", "detail_scene", "detail_object"},
		},
		{Label: "写你当时的反应", Hint: "从惊讶到明白，一点点转过来。写心理变化，不要直接写一句「我很感动」。"},
	}
	writingDetailSentences = []writingSentenceRole{
		{
			Label:   "选一个角度",
			Hint:    "人物的动作神态、周围的环境、还是一件东西。一次写一个，写透。",
			Methods: []string{"detail_person", "detail_scene", "detail_object"},
		},
		{Label: "换准的动词和修饰词", Hint: "把「走」「拿」「看」换成只有这个人在这一刻才会做的那个动作。"},
		{Label: "检查这三条", Hint: "是真的发生过吗，是不是只有他才会这样，换个人还成立吗。三条都过了才留下。"},
	}
	writingTurnSentences = []writingSentenceRole{
		{Label: "过渡一句", Hint: "「日子就这样一天天过着，直到有一天」。先铺平，转折才不突兀。"},
		{
			Label:   "让环境替你说话",
			Hint:    "天气、时辰、周围的声音。环境越难，他做的那件事越见分量。",
			Methods: []string{"detail_scene"},
		},
		{Label: "写触发的那一下", Hint: "一个具体的动作或一句话，让你突然发现之前想错了。这一下是全篇最要紧的地方。"},
	}
	writingFeelingSentences = []writingSentenceRole{
		{Label: "说出新的认识", Hint: "这件事之后，你对这个人、这件事的看法变成了什么。"},
		{Label: "回到开头", Hint: "呼应开头写的那份不满，让读者看见你确实变了。"},
		{Label: "落到以后", Hint: "往后你打算怎么做。一句就够，不要喊口号。"},
	}
)

// writingSentenceShape 交出这一种块的段内结构。
//
// 没有对应的（论据、道理、待补）返回 nil —— 那时候引导框里这一节整个不渲染，
// 而不是摆一份不适用的步骤。
func writingSentenceShape(kind string) []writingSentenceRole {
	switch kind {
	case writingKindOpening, writingKindThesis:
		return writingOpeningSentences
	case writingKindPoint:
		return writingBodyFiveSentences
	case writingKindCounter:
		return writingCounterSentences
	case writingKindRebuttal:
		return writingRebuttalSentences
	case writingKindClosing:
		return writingClosingSentences
	case writingKindScene:
		return writingSceneSentences
	case writingKindDetail:
		return writingDetailSentences
	case writingKindTurn:
		return writingTurnSentences
	case writingKindFeeling:
		return writingFeelingSentences
	}
	return nil
}

// writingSentenceLabels 是这一种块的几句各叫什么，供 prompt 引用。
func writingSentenceLabels(kind string) []string {
	shape := writingSentenceShape(kind)
	if len(shape) == 0 {
		return nil
	}
	out := make([]string, 0, len(shape))
	for _, s := range shape {
		out = append(out, s.Label)
	}
	return out
}
