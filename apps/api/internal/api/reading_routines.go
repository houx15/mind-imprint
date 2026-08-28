package api

// reading_routines.go — 阅读流程库：一组**写死的**读法。
//
// 产品的原话（2026-08-27）：
//
//   > an intelligent reading coach would generate a task list after students
//   > giving a paragraph. this task list, in the later, would be generated
//   > according to students' ability and the paper's difficulty.
//   >
//   > general scan → understanding / look at one/two focal paragraphs /
//   > read through a lens / think about information / finish the questions
//
// 清单来自这里，模型只负责**挑一套并调参**——和写作的结构库同一条理由，
// 而且这三条理由在阅读这边只会更硬：
//
//  1. **必须通用。** 让模型自由编任务，它编出来的会带上这篇文章的内容
//     （「找出作者关于碳排放的三个论据」）——那一刻「注意到什么」这件事已经
//     被 AI 做完了，而那正是任务本身。写死的步骤只会说「找出这一段里最难的
//     那句话」，它不可能替她注意。
//  2. **必须稳定。** 她读第四篇文章时，应该认得出这就是第一篇的那套方法。
//     那个「认得出」就是学习本身。每次给一份崭新的 AI 灵感，教会的只是新鲜感。
//  3. **必须便宜且诚实。** 铺一套流程是确定性的，不该烧一次模型调用。
//
// 「按学生能力和文章难度生成」因此**不需要任何结构改动**：能力和难度只是同一次
// 挑选调用的额外输入，routine 库一行都不用改。这就是现在把它写死的全部理由。

// readingTaskKind is what a step renders as. Adding a KIND is a code change;
// adding a ROUTINE is a data change — the same "schema-driven or not" test
// AGENTS.md applies to the tool cards.
type readingTaskKind string

const (
	// 通读：先把整篇过一遍，不求细节。
	taskRead readingTaskKind = "read"
	// 精读某一两段：印记 挑出的重点段，配段落工具。
	taskFocusBlock readingTaskKind = "focus_block"
	// 用一个透镜再看一遍。
	taskLens readingTaskKind = "lens"
	// 想想这篇给了你什么信息。
	taskReflect readingTaskKind = "reflect"
	// 联系你自己：把这篇跟她自己的经历、见过的事、原本的想法接上。这一步没有
	// 对错，也不检查——它存在的意义是让这篇文章跟她本人有关系。
	taskConnect readingTaskKind = "connect"
	// 找一找：不是打字回答，是回到文章里把某样东西点出来。收尾用它，因为
	// 打字的答案可以凭印象给，点出来的句子不能。
	taskHunt readingTaskKind = "hunt"
)

type readingRoutineStep struct {
	Kind   readingTaskKind `json:"kind"`
	Label  string          `json:"label"`
	Detail string          `json:"detail"`
}

type readingRoutine struct {
	Key   string `json:"key"`
	Name  string `json:"name"`
	Blurb string `json:"blurb"`
	// Lang scopes a routine to the language it is idiomatic in. An English
	// close-read and a Chinese 通读 are not translations of each other — they
	// spend their attention on different things.
	Lang  string               `json:"lang"`
	Steps []readingRoutineStep `json:"steps"`
}

// readingRoutines is the whole library. Growing it = appending here; nothing
// else in the pipeline changes (the recommend prompt is rendered from this
// slice, and the frontend fetches it rather than holding a second copy).
var readingRoutines = []readingRoutine{
	{
		Key:   "zh-scan-focus-lens",
		Lang:  "zh",
		Name:  "通读 → 精读 → 透镜",
		Blurb: "默认读法。适合说明文、议论文、新闻这类讲道理的文章。",
		Steps: []readingRoutineStep{
			{Kind: taskRead, Label: "先通读一遍", Detail: "不查词、不停下来，先知道这篇大概在说什么。"},
			{Kind: taskFocusBlock, Label: "精读重点段", Detail: "挑出来的这一段值得慢慢看——点开段落工具，把它拆开。"},
			{Kind: taskLens, Label: "换一个透镜再看", Detail: "用一个角度重新过一遍，看看能不能看出刚才没看见的东西。"},
			{Kind: taskReflect, Label: "这篇给了你什么", Detail: "用你自己的话说：读完之后，你知道了什么以前不知道的？"},
			{Kind: taskConnect, Label: "你见过这件事吗", Detail: "这篇讲的事，你自己身边、新闻里、或者别的书里，有没有碰到过？想到什么说什么，这一步没有标准答案。"},
			{Kind: taskHunt, Label: "回去找一句", Detail: "在文章里点出最能撑住作者观点的那一句。点出来，我们一起看看它撑不撑得住。"},
		},
	},
	{
		Key:   "zh-narrative",
		Lang:  "zh",
		Name:  "跟着故事读",
		Blurb: "适合记叙文、人物报道、散文——有人、有事、有转折的文章。",
		Steps: []readingRoutineStep{
			{Kind: taskRead, Label: "先读完这件事", Detail: "先把故事看完，别急着分析。"},
			{Kind: taskFocusBlock, Label: "看转折那一段", Detail: "事情在这里变了方向——点开段落工具，看看作者是怎么写的。"},
			{Kind: taskReflect, Label: "作者想让你有什么感觉", Detail: "他是靠什么让你有这种感觉的？"},
			{Kind: taskConnect, Label: "换成你呢", Detail: "如果是你在那个位置上，你会怎么做？跟他一样吗？说说你的理由。"},
			{Kind: taskHunt, Label: "回去找一句", Detail: "在文章里点出你觉得写得最好的那一句——不是最重要的，是最好的。"},
		},
	},
	{
		Key:   "en-close-read",
		Lang:  "en",
		Name:  "Close Read",
		Blurb: "英文文章的默认读法：先看懂，再看它是怎么写的。",
		Steps: []readingRoutineStep{
			{Kind: taskRead, Label: "先整体过一遍", Detail: "遇到不认识的词先跳过，先抓大意。"},
			{Kind: taskFocusBlock, Label: "拆开这一段", Detail: "点开段落工具：翻译、关键单词、语法、写作解析，一样一样看。"},
			{Kind: taskFocusBlock, Label: "再拆一段", Detail: "第二段。这一次先自己读，读不懂再点工具。"},
			{Kind: taskLens, Label: "换一个透镜再看", Detail: "用一个角度重新过一遍。"},
			{Kind: taskReflect, Label: "用你自己的话复述", Detail: "不看原文，用中文把这篇讲一遍。"},
			{Kind: taskConnect, Label: "你原来是怎么想的", Detail: "读之前你对这件事是什么印象？读完之后变了没有？"},
			{Kind: taskHunt, Label: "回去找一句", Detail: "在文章里点出你觉得最难、但现在读懂了的那一句。"},
		},
	},
	{
		Key:   "en-argument",
		Lang:  "en",
		Name:  "Follow the Argument",
		Blurb: "适合英文议论文、社论、TOEFL 阅读——作者在说服你的时候用。",
		Steps: []readingRoutineStep{
			{Kind: taskRead, Label: "先整体过一遍", Detail: "先找出作者站哪一边。"},
			{Kind: taskFocusBlock, Label: "拆开他最用力的那一段", Detail: "点开段落工具，看他是怎么把话说重的。"},
			{Kind: taskLens, Label: "换一个透镜再看", Detail: "用一个角度检查他的论证。"},
			{Kind: taskReflect, Label: "你信吗", Detail: "哪一步你觉得站得住，哪一步你觉得他跳过去了？"},
			{Kind: taskConnect, Label: "你站哪边", Detail: "读之前你自己是什么立场？作者动摇你了吗，还是让你更确定了？"},
			{Kind: taskHunt, Label: "回去找一句", Detail: "在文章里点出作者最没说服你的那一句。"},
		},
	},
}

func readingRoutinesFor(lang string) []readingRoutine {
	want := lang
	if want != "en" {
		want = "zh"
	}
	out := make([]readingRoutine, 0, len(readingRoutines))
	for _, r := range readingRoutines {
		if r.Lang == want {
			out = append(out, r)
		}
	}
	return out
}

// findReadingRoutine resolves a key. Returns false for an unknown one so
// every caller fails closed — a routine key that no longer exists must never
// silently lay out an empty task list.
func findReadingRoutine(key string) (readingRoutine, bool) {
	for _, r := range readingRoutines {
		if r.Key == key {
			return r, true
		}
	}
	return readingRoutine{}, false
}

// ---------------------------------------------------------------------------
// 段落工具
// ---------------------------------------------------------------------------

// readingBlockTool is one of the per-paragraph explainers. They differ by
// language because the SKILLS differ by language: an English paragraph is hard
// because of vocabulary and syntax, a Chinese one because of 修辞 and 用典.
type readingBlockTool struct {
	ID    string `json:"id"`
	Label string `json:"label"`
	// Lang scopes a tool to the language whose difficulty it addresses. "" =
	// both: 想一想 and 仿写 are about what the paragraph DOES, which is not a
	// language-specific problem.
	Lang string `json:"lang"`
	// Instruction is appended to the shared system prompt. Kept beside the
	// label so a new tool is one entry here and nothing else.
	Instruction string `json:"-"`
	// Shape decides how the reply is parsed and what it is ALLOWED to contain:
	//
	//   "prose"     — free explanation of the article. Safe: explaining someone
	//                 else's published paragraph is what a teacher does.
	//   "questions" — questions only. Anything not ending in a question mark is
	//                 dropped server-side.
	//   "imitate"   — a named move plus situations to try it on. There is
	//                 deliberately NO FIELD for a sample paragraph, so the
	//                 model has nowhere to put one even if it wants to.
	//
	// The last two are 铁律① enforced by output TYPE rather than by asking the
	// model to behave — the same trick the writing room's guiding box uses.
	Shape string `json:"shape"`
}

// 铁律 CHECK. These are EXPLANATORY, and that is why they are safe: 铁律①
// forbids the AI writing the STUDENT's own prose. Explaining someone else's
// published paragraph is what a teacher does — withholding it would make the
// room less useful without making it more honest.
//
// The line that IS held: every tool is keyed on a `blockId` of the ARTICLE and
// has no write path into any student-authored field (摘要 / 批注 / takeaway).
// Enforced structurally — reading_block.go never touches those tables.
var readingBlockTools = []readingBlockTool{
	{
		Shape: "prose", ID: "translate", Label: "翻译", Lang: "en",
		Instruction: "把这一段忠实地翻译成中文。不要意译到走样，也不要逐字硬译到读不通。只给译文。",
	},
	{
		Shape: "prose", ID: "vocabulary", Label: "关键单词", Lang: "en",
		Instruction: "挑出这一段里**真正值得学**的 3–5 个词（不是最长的，是最有用的、在这里意思特别的）。每个词给：词 — 在这句里的意思 — 一个短例子。不要把整段的词都列出来，那是词典干的事。",
	},
	{
		Shape: "prose", ID: "grammar", Label: "语法", Lang: "en",
		Instruction: "指出这一段里让人读不懂的那 1–2 个句子结构（长从句、倒装、插入语、非谓语……）。把那个句子摘出来，说清楚它的主干是什么、修饰的部分挂在哪。只讲让人卡住的，不要通篇语法课。",
	},
	{
		Shape: "prose", ID: "craft", Label: "写作解析", Lang: "en",
		Instruction: "这一段在整篇里**在干什么**（提出主张、举例、让步、转折、收束……），以及作者用什么手法让它起作用。两三句话，说的是写法，不是内容摘要。",
	},
	{
		Shape: "prose", ID: "rhetoric", Label: "成语修辞", Lang: "zh",
		Instruction: "指出这一段用到的成语、俗语和修辞手法（比喻、排比、反问、对比……），每个都说清楚它在这里起了什么效果。没有就直说没有，不要硬找。",
	},
	{
		Shape: "prose", ID: "examples", Label: "案例", Lang: "zh",
		Instruction: "这一段举了哪些具体的事例、数据或引用？每个说清楚它是用来支持什么的。没有具体事例就直说这一段是在讲道理，不是在举例。",
	},
	{
		Shape: "prose", ID: "structure", Label: "结构解析", Lang: "zh",
		Instruction: "这一段在整篇里**在干什么**（起头、承接、转折、举证、收束……），以及它和上一段是什么关系。两三句话，说的是位置和作用，不是内容摘要。",
	},
}

// 想一想 and 仿写 close the loop from reading into writing: understand the
// paragraph, then make the same move yourself. They are language-independent
// because "what does this paragraph DO" is not a language-specific question.
var readingWritingTools = []readingBlockTool{
	{
		Shape: "questions", ID: "questions", Label: "想一想", Lang: "",
		Instruction: "针对这一段，给她 2 到 4 个能帮她想下去的问题。必须是问题，每条以问号结尾，一条一个问题。要具体到这一段的内容，不要问「这段讲了什么」这种空问题。**不要在问题里把答案说出来。**",
	},
	{
		Shape: "imitate", ID: "imitate", Label: "仿写", Lang: "",
		Instruction: "先用一句话说清楚这一段**在写法上做了什么**（比如「先给一个日常场景，再解释背后的原理」），然后给 2 到 3 个她可以用同一个写法去写的、和原文无关的话题。",
	},
}

func readingBlockToolsFor(lang string) []readingBlockTool {
	want := lang
	if want != "en" {
		want = "zh"
	}
	out := make([]readingBlockTool, 0, len(readingBlockTools)+len(readingWritingTools))
	for _, t := range readingBlockTools {
		if t.Lang == want {
			out = append(out, t)
		}
	}
	// The language-independent pair always comes last: understand first, then
	// make the move yourself.
	out = append(out, readingWritingTools...)
	return out
}

func findReadingBlockTool(id string) (readingBlockTool, bool) {
	for _, t := range readingBlockTools {
		if t.ID == id {
			return t, true
		}
	}
	for _, t := range readingWritingTools {
		if t.ID == id {
			return t, true
		}
	}
	return readingBlockTool{}, false
}
