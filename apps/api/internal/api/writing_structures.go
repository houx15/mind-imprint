package api

// writing_structures.go — 结构库：一组**固定的、通用的**文章骨架。
//
// 这是 2026-08-27 产品裁定的地基：「AI 永远不直接生成提纲。AI 帮学生思考，
// 给出通用结构，引导学生用自己的想法和经历去填每一块。」
//
// 所以骨架写死在这里，而不是每次让模型现编：
//
//   1. **它必须是通用的。** 「反方最强的说法」对任何议论题都成立——它是文体
//      知识，不是她这篇文章的内容。模型现编的「块名」会不可避免地带上她的
//      题目（「手机对睡眠的影响」），那一刻 AI 就已经替她想好了论点，正是
//      裁定要禁止的事。写死，就不可能漂移。
//   2. **它必须稳定。** 同一个学生第二次写议论文，应该认得出上次那副骨架；
//      每次都换一套「AI 灵感」是在教她依赖新鲜感，而不是教她文章的结构。
//   3. **它必须便宜。** 铺骨架是零成本的确定性步骤，不该烧一次模型调用。
//
// 模型在这条链路上只剩一件事：**从这张表里挑一副**并说明理由
// （writing_structure_recommend.go）。挑选是判断，不是创作——它读不到、也
// 写不出块里的任何一个字。挑错了她可以自己换一副，代价是一次点击。
//
// 每一块只有两样东西：role（块名，进 writing_outline.role，0100）和 hint
// （一句「这一块该放什么」的通用说明，只用于渲染，不入库）。两者都不含她的
// 内容——她自己那句话写进同一行的 text 列，与 role 严格分开。

// writingStructureBlock is one block of a skeleton: a generic role label and
// a generic one-line description of what belongs there. Neither ever carries
// student-specific content — that is the whole point of the file.
type writingStructureBlock struct {
	Role string `json:"role"`
	Hint string `json:"hint"`
}

// writingStructure is one skeleton in the library.
type writingStructure struct {
	Key string `json:"key"`
	// Lang scopes a skeleton to the language it is idiomatic in. A 记叙文
	// skeleton and a personal-essay skeleton are NOT translations of each
	// other — they teach different conventions — so the library keeps two
	// genuinely separate sets rather than one set with translated labels.
	Lang   string                  `json:"lang"`
	Name   string                  `json:"name"`
	Blurb  string                  `json:"blurb"`
	Blocks []writingStructureBlock `json:"blocks"`
}

// writingStructures is the whole library. Growing it = appending an entry
// here; nothing else in the pipeline needs to change (the recommend prompt
// is built from this slice, and the frontend fetches it rather than
// hardcoding a second copy — see GET /writings/structures).
var writingStructures = []writingStructure{
	{
		Key:   "zh-argument-stance",
		Lang:  "zh",
		Name:  "立场式议论",
		Blurb: "你心里已经有立场，想把它讲得让人信服。最常用的一副骨架。",
		Blocks: []writingStructureBlock{
			{Role: "你的立场", Hint: "一句话说清楚你站哪一边。先别解释为什么，就把话说死。"},
			{Role: "最强的那个理由", Hint: "你所有理由里最能站住的一条。不是最多人说的那条，是你自己最信的那条。"},
			{Role: "支持它的证据或经历", Hint: "一件真事、一个数字、一次你自己的经历——能让人看见，而不只是听见。"},
			{Role: "第二个理由", Hint: "换一个角度再推一把。和上一条不该是同一件事的两种说法。"},
			{Role: "结论", Hint: "回到你的立场，但说出比开头多一点的东西。"},
		},
	},
	{
		Key:   "zh-argument-concession",
		Lang:  "zh",
		Name:  "让步式议论",
		Blurb: "对方也有道理，硬顶反而站不住。先承认，再转折——写出来最有说服力的一副骨架。",
		Blocks: []writingStructureBlock{
			{Role: "你的立场", Hint: "一句话说清楚你站哪一边。"},
			{Role: "反方最强的说法", Hint: "认真找对方最难反驳的那条，不是最好打的那条。找软柿子捏，读者一眼就看出来。"},
			{Role: "你承认它哪一部分是对的", Hint: "老实说出它成立的地方。这不是认输，这是让接下来的转折有分量。"},
			{Role: "你的回应", Hint: "承认之后，为什么你依然站原来那边？转折点就在这一块。"},
			{Role: "结论", Hint: "把让步和回应收拢成一句话。"},
		},
	},
	{
		Key:   "zh-explain",
		Lang:  "zh",
		Name:  "说明文",
		Blurb: "你想把一件事讲清楚，而不是争论对错。",
		Blocks: []writingStructureBlock{
			{Role: "要说清楚的是什么", Hint: "一句话点题。读者读完这一句就知道接下来要懂什么。"},
			{Role: "它是怎么运作的", Hint: "拆成几步，按顺序讲。别一次塞太多。"},
			{Role: "一个具体例子", Hint: "抽象讲完，落到一个能想象出画面的例子上。"},
			{Role: "常见的误解", Hint: "大多数人以为是怎样的？为什么那样想不对？"},
			{Role: "小结", Hint: "读者应该记住的那一件事。"},
		},
	},
	{
		Key:   "zh-narrative",
		Lang:  "zh",
		Name:  "记叙文",
		Blurb: "你想讲一件真实发生过的事，以及它改变了你什么。",
		Blocks: []writingStructureBlock{
			{Role: "事情发生前", Hint: "当时的你是什么状态？在意什么、以为什么？"},
			{Role: "关键的那一刻", Hint: "把镜头推近。具体到某一天、某一句话、某个动作。"},
			{Role: "我当时的反应", Hint: "身体的、情绪的、说出口的和没说出口的。"},
			{Role: "后来发生了什么", Hint: "事情怎么收场的。不必圆满。"},
			{Role: "它改变了我什么", Hint: "现在回头看，你和当时的你差在哪里？"},
		},
	},
	{
		Key:   "en-argument-classic",
		Lang:  "en",
		Name:  "Classic Argument",
		Blurb: "You have a position and want to argue it clearly. The default shape.",
		Blocks: []writingStructureBlock{
			{Role: "Thesis", Hint: "One sentence stating your position. Commit to it — don't hedge yet."},
			{Role: "First reason", Hint: "Your strongest reason, not the most popular one."},
			{Role: "Evidence for it", Hint: "A fact, a number, or something you saw happen. Something a reader can picture."},
			{Role: "Second reason", Hint: "A genuinely different angle — not the first reason reworded."},
			{Role: "Conclusion", Hint: "Return to the thesis, but say something more than you did at the start."},
		},
	},
	{
		Key:   "en-argument-concession",
		Lang:  "en",
		Name:  "Argument with Concession",
		Blurb: "The other side has a real point. Concede it first, then turn — the most persuasive shape.",
		Blocks: []writingStructureBlock{
			{Role: "Thesis", Hint: "One sentence stating your position."},
			{Role: "The strongest opposing view", Hint: "Find the argument that is hardest to answer, not the easiest."},
			{Role: "What you concede", Hint: "Say plainly where they are right. This is what gives your turn its weight."},
			{Role: "Your rebuttal", Hint: "Having granted that — why do you still hold your position? The turn happens here."},
			{Role: "Conclusion", Hint: "Pull the concession and the rebuttal into one line."},
		},
	},
	{
		Key:   "en-personal-essay",
		Lang:  "en",
		Name:  "Personal Essay",
		Blurb: "One true story, told so that it argues something without saying so outright.",
		Blocks: []writingStructureBlock{
			{Role: "The moment", Hint: "Open inside the scene. A specific day, a specific sentence someone said."},
			{Role: "What I believed before", Hint: "Who were you walking in? What did you assume?"},
			{Role: "What happened", Hint: "Tell it straight, in order. Resist explaining it yet."},
			{Role: "What shifted", Hint: "The hinge. Name it precisely — vague is the enemy here."},
			{Role: "What it means now", Hint: "Looking back, what do you understand that you didn't?"},
		},
	},
	{
		Key:   "en-expository",
		Lang:  "en",
		Name:  "Expository",
		Blurb: "You want to explain something clearly, not argue about it.",
		Blocks: []writingStructureBlock{
			{Role: "What needs explaining", Hint: "One sentence. After reading it, the reader knows what they're about to understand."},
			{Role: "How it works", Hint: "Break it into steps, in order. Don't crowd them."},
			{Role: "A concrete example", Hint: "Land the abstraction on something the reader can picture."},
			{Role: "A common misconception", Hint: "What do most people assume, and why is it wrong?"},
			{Role: "Wrap-up", Hint: "The one thing the reader should walk away with."},
		},
	},
}

// writingStructuresFor returns the skeletons idiomatic to one language.
// An unrecognised lang yields the zh set rather than an empty list — the
// student must always have something to pick from, and 'zh' is lite's
// default audience (writings.go creates with lang 'zh' before setup runs).
func writingStructuresFor(lang string) []writingStructure {
	out := make([]writingStructure, 0, len(writingStructures))
	want := lang
	if want != "en" {
		want = "zh"
	}
	for _, s := range writingStructures {
		if s.Lang == want {
			out = append(out, s)
		}
	}
	return out
}

// findWritingStructure resolves a key to its skeleton. Returns false for an
// unknown key so every caller fails closed — a structure_key that no longer
// exists must never silently lay out an empty outline.
func findWritingStructure(key string) (writingStructure, bool) {
	for _, s := range writingStructures {
		if s.Key == key {
			return s, true
		}
	}
	return writingStructure{}, false
}
