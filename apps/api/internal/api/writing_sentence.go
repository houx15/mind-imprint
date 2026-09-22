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
		Hint:  "请在段首写出本段的分论点，并说明它与中心论点的关系。",
	},
	{
		Label: "阐释句",
		Hint:  "请解释观点句中的关键概念，让读者理解后面的例子为什么与它有关。",
		// 讲义：引用名言 / 运用比喻 / 用逻辑关系 / 运用对比 来阐述分论点。
		Methods: []string{"point_quote", "point_metaphor", "point_reasoning", "point_contrast"},
	},
	{
		Label: "材料句",
		Hint:  "请简要交代人物、事件和结果。材料篇幅建议不超过本段的一半，为分析留出空间。",
		Methods: []string{"point_pee"},
	},
	{
		Label: "分析句",
		Hint:  "请说明材料如何支持分论点。可以分析原因、比较假设条件下的结果，或归纳多个例子的共同点。",
		// 讲义（五）第三节的三法，各带一句可套的句式。
		Methods: []string{"analysis_cause", "analysis_suppose", "analysis_induce"},
	},
	{
		Label: "结论句",
		Hint:  "请总结本段的论证结果，说明它如何支持中心论点。",
	},
}

// 议论文的开篇与结尾，以及反方那两块。这几块讲义里没有逐句的表，
// 用的是 R3 定下的那几步，结尾一格补上讲义（六）的四技法。
var (
	writingOpeningSentences = []writingSentenceRole{
		{Label: "引出话题", Hint: "一句话让读者知道为什么值得谈这件事。"},
		{Label: "提出中心论点", Hint: "这篇要证明的那一句，直接说出来。"},
		{Label: "交代论证顺序", Hint: "请简要说明后文的论证顺序，具体例子留在主体段展开。"},
	}
	writingClosingSentences = []writingSentenceRole{
		{Label: "回到中心论点", Hint: "回到开头那一句，但不要原样再说一遍。"},
		{
			Label:   "总结论证结果",
			Hint:    "请根据全文论证，更准确地表述结论。可选择呼应开头、引用、归纳或修辞收束。",
			Methods: []string{"closing_echo", "closing_quote", "closing_sum", "closing_figure"},
		},
		{Label: "提出具体建议", Hint: "请提出与论证结果相关、可以执行的建议。结尾宜简短，可参考两百字以内的篇幅。"},
	}
	writingCounterSentences = []writingSentenceRole{
		{Label: "反方的主要理由", Hint: "请准确介绍反方的主要理由，避免歪曲或简化对方的观点。"},
		{Label: "承认它成立的地方", Hint: "请说明反方理由在哪些条件下成立。"},
		{Label: "指出它的适用范围", Hint: "请说明反方理由的适用条件，以及它为什么不足以推翻你的结论。"},
	}
	writingRebuttalSentences = []writingSentenceRole{
		{Label: "复述对方观点", Hint: "请先准确复述你要回应的观点。"},
		{Label: "指出它的适用范围", Hint: "请说明对方观点的适用范围，以及当前情形与这些条件的区别。"},
		{Label: "回到自己的结论", Hint: "请说明回应反方意见后，你的结论仍有哪些依据。"},
	}
)

// 记叙文那四块。来源是两份记叙文讲义：细节描写（人物／环境／物件、
// 精准动词、三禁忌）和抑扬转情法（抑→渡→转→扬）。
var (
	writingSceneSentences = []writingSentenceRole{
		{Label: "交代时间和地点", Hint: "请交代事件的时间、地点和当时的活动，帮助读者理解情境。"},
		{
			Label:   "把这件事写具体",
			Hint:    "动作、神态、说过的话，挑和主题相关的写。动词要准：「弓着身子蹒跚着走来」和「走过来」是两件事。",
			Methods: []string{"detail_person", "detail_scene", "detail_object"},
		},
		{Label: "写你当时的反应", Hint: "请描述当时的心理反应及其变化，并用具体细节解释产生这些感受的原因。"},
	}
	writingDetailSentences = []writingSentenceRole{
		{
			Label:   "选一个角度",
			Hint:    "人物的动作神态、周围的环境、还是一件东西。一次写一个，写透。",
			Methods: []string{"detail_person", "detail_scene", "detail_object"},
		},
		{Label: "选择准确的动词和修饰词", Hint: "请检查「走」「拿」「看」等动作是否足够具体，选择符合当时情境的词。"},
		{Label: "检查这三条", Hint: "请检查细节是否符合事件、能否表现人物特点、是否与主题有关。"},
	}
	writingTurnSentences = []writingSentenceRole{
		{Label: "交代事件过渡", Hint: "请交代时间或情境的变化，让读者理解接下来的转折。"},
		{
			Label:   "描写相关环境",
			Hint:    "请选择与事件有关的天气、时间或声音，说明环境怎样影响人物的行动。",
			Methods: []string{"detail_scene"},
		},
		{Label: "写出转折原因", Hint: "请写出引起看法或情绪变化的具体动作、话语或事件。"},
	}
	writingFeelingSentences = []writingSentenceRole{
		{Label: "说出新的认识", Hint: "这件事之后，你对这个人、这件事的看法变成了什么。"},
		{Label: "回到开头", Hint: "请联系开头的感受，说明经历这件事后，你的看法有哪些变化。"},
		{Label: "说明后续行动", Hint: "请说明这次认识会怎样影响你以后的行动。"},
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
