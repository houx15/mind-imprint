package api

import "mindimprint/api/internal/gateway"

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
// 清单来自这里，模型只负责**挑一套并调参**——和写作的结构库同一条理由：
//
//  1. **步骤本身必须通用。** 让模型自由编**有哪些步骤**，它编出来的会带上这篇
//     文章的内容（「找出作者关于碳排放的三个论据」），于是「这篇文章该按什么
//     顺序读」这件事每篇都从头长一遍，她学不到任何可迁移的东西。写死的步骤只会
//     说「找出这一段里最难的那句话」——放在任何一篇文章上都成立。
//  2. **必须稳定。** 她读第四篇文章时，应该认得出这就是第一篇的那套方法。
//     那个「认得出」就是学习本身。每次给一份崭新的 AI 灵感，教会的只是新鲜感。
//  3. **必须便宜且诚实。** 铺一套流程是确定性的，不该烧一次模型调用。
//
// 「按学生能力和文章难度生成」因此**不需要任何结构改动**：能力和难度只是同一次
// 挑选调用的额外输入，routine 库一行都不用改。这就是把 routine 库写死的全部理由。
//
// 🚨 **2026-08-29 的推翻，只推翻一处，别扩大化。**
//
// 这段注释原来的第 1 条还多说了一句：模型不许写任务内容，因为「AI 一旦说出该
// 注意什么，注意这件事就已经被它做完了」。产品负责人当场否掉了：
//
//   > this is ridiculous. why? AI provides scaffolding. you are banning scaffolding.
//
// 他是对的：**指出什么值得看，本来就是老师做的事的大半**。脚手架不是替她做，
// 是让她够得着。所以边界重新划成这样：
//
//   - 第 1 条**对「有哪些步骤」仍然成立**——routine 库继续写死、继续通用（见上）。
//     它**对「步骤里 印记 递给她什么」不再成立**：印记 在某一步里现写一张卡片、
//     用这篇文章的原话当选项，那是脚手架，不是越界。见 reading_coach_card.go。
//   - 第 2 条**完全活着**，只是改由**固定的卡片类型**承担：她认得出的是卡片的
//     形状（choose_span / pick_in_article / short_text），不是每次崭新的措辞。
//   - 第 3 条**完全活着**：routine 的挑选仍然是确定性的，一次模型调用都不烧；
//     卡片是在一轮本来就要发生的对话里顺手产出的，没有额外的一次调用。

import "fmt"

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

	// ── 2026-09-10 新增的三步 ────────────────────────────────────────────
	//
	// 三步都来自 docs/2026-09-10-reading-guidance-redesign.md 那份调研，
	// 它们补的是我们**整套读法里根本没有的位置**，不是把已有的步骤换个说法。

	// 预测：只看标题和第一句，猜这篇要解决什么问题、作者站哪边。
	//
	// 关键在它的**禁止**：这一步明说「正文先别读」。调研里那句
	// 「Do not give the expected answer after asking prediction questions」
	// 同样重要 —— 预测是一个待检验的假设，不是一道当场对答案的题。
	// 它把「读」从「接收」变成「验证」，而这个转换只有在读之前做才有意义。
	taskPredict readingTaskKind = "predict"
	// 标注论证：给几句话各自贴一个角色（主张 / 证据 / 限制 / 背景 / 对比）。
	//
	// 这是一次**不问「你懂了吗」的理解检查**：贴不出来就是没读懂，而她一个字
	// 都不用写。对应 label_roles 那块板（reading_coach_card.go）。
	taskLabel readingTaskKind = "label"
	// 复述：合上文章，凭记忆说出三个表达、一句话主张、和它靠什么撑着。
	//
	// 调研里叫 exit retrieval。它便宜（一分半钟）、假不了（合上了文章，
	// 凭印象说不出来就是真的没留下），而且产出的是一条完整的过程记录。
	taskRecall readingTaskKind = "recall"
	// 你怎么看：她自己对这篇文章的判断 —— 同意不同意，证据够不够，
	// 有没有别的可能。
	//
	// 🚨 2026-09-17 **顶掉了 透镜 那一步**。产品负责人逐字：
	//
	//	at this stage, I think we can skip the 透镜 part. it is really not
	//	applicable in many papers. and difficult for students to understand.
	//	the above mentioned critical thinking can be a better replacement of
	//	lens.
	//
	// 它和 taskLabel 是**同一件事的两半**，而且必须分开走：label 是拆**作者**
	// 写了什么（那块板），critique 是她自己怎么看。挤在一步里她只会挑更容易的
	// 那件做，产品负责人的原话是「split them. first is analyze what author
	// written. then is students' self critical thinking.」
	taskCritique readingTaskKind = "critique"

	// 排序：把几件事按**发生的先后**排好（order_events 那块板）。
	//
	// 2026-09-17 同事的阅读模块 PRD：新闻报道要「搭建事件时间线」，记叙文要
	// 「事件卡排序；切换发生顺序／讲述顺序」。卡片上的几句按**原文顺序**摆出来
	// （那就是讲述顺序），她要排成的是发生顺序 —— 两者不一样的地方，就是这篇
	// 文章的叙述手法。议论文和说明文的读法里没有这一步。
	taskSequence readingTaskKind = "sequence"
)

// focusBlockLabelBase is the 精读 step's label WITHOUT its paragraph number.
// The number is not known when the library is written — it is whichever
// paragraph 印记 picked for this article — so the routine holds the base and
// buildReadingTasks appends 第N段 as the rows are built.
//
// 2026-08-29: the product owner read the old labels off a real screen and
// could not tell what they were asking for（「给了你什么」「回去找一句」）。
// The names are now plain: 通读全文 / 精读重点段落第N段 / 深入思考 / 总结收获 /
// 链接经验 / 找出关键句. Where a routine's own step already had a plain, human
// name (「你信吗」「换成你呢」), it keeps it — the rule is "say it in Chinese a
// 13-year-old would use", not "make every routine identical".
const focusBlockLabelBase = "精读重点段落"

// focusBlockLabel stamps the real paragraph number onto the 精读 step.
//
// 🚨 ord <= 0 means the paragraph is unknown, and the ONLY correct answer then
// is a label with no number at all. 「第 0 段」 and a literal X are both worse
// than saying less: she would go looking for a paragraph that does not exist.
func focusBlockLabel(ord int) string {
	if ord <= 0 {
		return focusBlockLabelBase
	}
	return fmt.Sprintf("%s第%d段", focusBlockLabelBase, ord)
}

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
	Lang string `json:"lang"`
	// Genres 是这套读法服务的体裁（reading_outline.go 的闭表）。
	//
	// 🚨 2026-09-17 起**读法跟着体裁走**：排读法那一次调用同时判了体裁，
	// 模型挑的读法不服务这个体裁，服务端就换成同语言里服务它的第一套
	// （pickRoutineForGenre）。原来两件事各判各的，一篇新闻报道可以被排上
	// 「通读 → 精读 → 论证」，于是整条清单都在找一个不存在的主张。
	Genres []string             `json:"genres"`
	Steps  []readingRoutineStep `json:"steps"`
}

// serves —— 这套读法是不是给这个体裁的。
func (r readingRoutine) serves(genre string) bool {
	for _, g := range r.Genres {
		if g == genre {
			return true
		}
	}
	return false
}

// pickRoutineForGenre 把模型挑的读法和它判的体裁对齐。
//
// 体裁认不出来（空）就不动 —— 和 hasAuthorsArgument 同一个方向：宁可放过。
// 模型挑的那一套本来就服务这个体裁（议论文在英文里有两套可挑），也不动。
// 只有两件事打架的时候才换，换成同语言里服务这个体裁的第一套。
func pickRoutineForGenre(chosen readingRoutine, genre string) readingRoutine {
	genre = validateGenre(genre)
	if genre == "" || chosen.serves(genre) {
		return chosen
	}
	for _, r := range readingRoutines {
		if r.Lang == chosen.Lang && r.serves(genre) {
			return r
		}
	}
	return chosen
}

// readingRoutines is the whole library. Growing it = appending here; nothing
// else in the pipeline changes (the recommend prompt is rendered from this
// slice, and the frontend fetches it rather than holding a second copy).
var readingRoutines = []readingRoutine{
	{
		Key:    "zh-scan-focus-lens",
		Lang:   "zh",
		Genres: []string{genreArgument},
		Name:   "通读 → 精读 → 论证",
		Blurb:  "议论文的读法：作者在说服你接受一个看法（议论文、社论、评论）。",
		Steps: []readingRoutineStep{
			{Kind: taskPredict, Label: "先预测", Detail: "请先根据标题预测文章主题。"},
			{Kind: taskRead, Label: "通读全文", Detail: "请先通读并把握大意；不影响理解的生词可暂时跳过。"},
			{Kind: taskFocusBlock, Label: focusBlockLabelBase, Detail: "这一段值得细读。请打开段落工具，把它拆开。"},
			{Kind: taskLabel, Label: "拆开作者的论证", Detail: "把几句话各自归到论证三要素里：论点、论据、论证。"},
			{Kind: taskCritique, Label: "你怎么看", Detail: "作者说的你同意吗？他给的证据够不够？有没有另一种解释？挑一个角度说。"},
			{Kind: taskReflect, Label: "总结论点", Detail: "用你自己的话总结本文的论点：作者的中心论点是什么？他分几层来论证？"},
			// 🚨 2026-09-17 改写。原来这句是「这篇讲的事，你自己身边、新闻里、
			// 或者别的书里，有没有碰到过？」——产品负责人报的「有点抽象，还有
			// 点鸡肋」说的就是它：一个没有落点的问题只能换来一句泛泛的话。
			// 落点由 印记 从这篇文章里取（见 readingCurrentStepInstruction 的
			// connect 分支），这里说清楚她要拿出来的是什么。
			{Kind: taskConnect, Label: "链接经验", Detail: "请就文章里的某一个说法，说出你自己见过的一件对得上、或者对不上的事。"},
			{Kind: taskHunt, Label: "找出中心论点", Detail: "回到文章里：作者直接表明中心论点的是哪一句？把它点出来，对照你刚才的总结。"},
		},
	},
	{
		// 2026-09-17 按同事的阅读模块 PRD 重排：读懂事件 → 找到转折 → 理解人物 →
		// 回看叙述与细节 → 形成自己的解释。
		Key:    "zh-narrative",
		Lang:   "zh",
		Genres: []string{genreNarrative},
		Name:   "跟着故事读",
		Blurb:  "记叙文的读法：有人、有事、有转折（记叙文、小说片段、人物故事）。",
		Steps: []readingRoutineStep{
			{Kind: taskRead, Label: "通读全文", Detail: "先把故事看完：谁、在哪儿、发生了什么。"},
			{Kind: taskSequence, Label: "排出事件顺序", Detail: "把几件事按发生的先后排好，再和文章讲述的顺序对照。"},
			{Kind: taskFocusBlock, Label: focusBlockLabelBase, Detail: "事情在这里发生了转折，请看作者是怎么写的。"},
			{Kind: taskLabel, Label: "看人物怎么写", Detail: "把几句话各自归到一种描写：动作、语言、心理、环境。"},
			{Kind: taskCritique, Label: "人物为什么这样做", Detail: "结合前后的行为，说出你对人物动机的解释，并指出原文依据。"},
			{Kind: taskConnect, Label: "角色选择", Detail: "如果遇到相同情境，你会如何处理？"},
			{Kind: taskHunt, Label: "找出关键句", Detail: "文中你觉得写得最好的是哪一句？不是最重要的，是写得最好的。把它点出来。"},
		},
	},
	{
		// 2026-09-17 新增。PRD：了解事件 → 梳理时间与参与方 → 区分事实、引述和
		// 解释 → 比较来源与说法 → 总结已知与待了解的信息。
		Key:    "zh-report",
		Lang:   "zh",
		Genres: []string{genreReport},
		Name:   "读新闻报道",
		Blurb:  "新闻报道的读法：发生了什么、各方怎么说，记者自己不表态。",
		Steps: []readingRoutineStep{
			{Kind: taskPredict, Label: "先预测", Detail: "只看标题：这篇报的是一件什么事？"},
			{Kind: taskRead, Label: "通读全文", Detail: "先弄清楚两件事：发生了什么，牵涉到哪几方。"},
			{Kind: taskSequence, Label: "排出事件时间线", Detail: "把几件事按发生的先后排好。报道常常先讲结果，再回头交代经过。"},
			{Kind: taskFocusBlock, Label: focusBlockLabelBase, Detail: "这一段值得细读。请打开段落工具，把它拆开。"},
			{Kind: taskLabel, Label: "分清事实与说法", Detail: "把几句话各自归类：记者核实的事实、某一方说的话、对事件的解释。"},
			{Kind: taskCritique, Label: "比较来源", Detail: "各方的说法依据是什么？有没有哪一方没被问到？哪一句还需要别的来源？"},
			{Kind: taskReflect, Label: "已知与待了解", Detail: "这件事目前能确定的是什么，还有哪些没有弄清楚？"},
			{Kind: taskConnect, Label: "你原来是怎么想的", Detail: "读之前你对这件事是什么印象？读完之后变了没有？"},
			{Kind: taskHunt, Label: "找出关键句", Detail: "文中哪一句最能说明这件事？把它点出来。"},
		},
	},
	{
		// 2026-09-17 新增。PRD：明确说明对象 → 理清概念 → 搭出结构或过程 →
		// 解释关键关系 → 换情境应用。
		Key:    "zh-explain",
		Lang:   "zh",
		Genres: []string{genreExplain},
		Name:   "读说明文",
		Blurb:  "说明文的读法：讲清楚一样东西是什么、怎么运作（科普、原理、流程）。",
		Steps: []readingRoutineStep{
			{Kind: taskPredict, Label: "先预测", Detail: "只看标题：这篇要说明的对象是什么？"},
			{Kind: taskRead, Label: "通读全文", Detail: "先找到说明对象，再看作者分几块来讲它。"},
			{Kind: taskFocusBlock, Label: focusBlockLabelBase, Detail: "这一段讲的是关键的概念或原理，值得细读。"},
			{Kind: taskLabel, Label: "理清说明结构", Detail: "把几句话各自归类：说明对象、原理与过程、例子与数据。"},
			{Kind: taskCritique, Label: "你怎么看", Detail: "这篇解释清楚了吗？哪一环还没讲透，例子撑不撑得住？挑一处说。"},
			{Kind: taskReflect, Label: "解释关键关系", Detail: "用你自己的话说清楚：文中的一个原因是怎么导致那个结果的？"},
			{Kind: taskConnect, Label: "换个情境用一用", Detail: "把文中的原理放到另一个情境里，它还成立吗？会有什么不同？"},
			{Kind: taskHunt, Label: "找出关键句", Detail: "文中哪一句最能概括这个原理？把它点出来。"},
		},
	},
	{
		Key:    "en-close-read",
		Lang:   "en",
		Genres: []string{genreArgument},
		Name:  "Close Read",
		Blurb: "英文文章的默认读法：先看懂，再看它是怎么写的。",
		Steps: []readingRoutineStep{
			{Kind: taskPredict, Label: "先预测", Detail: "请根据标题和第一句预测文章主题。"},
			{Kind: taskRead, Label: "通读全文", Detail: "遇到不认识的词先跳过，先抓大意。"},
			{Kind: taskFocusBlock, Label: focusBlockLabelBase, Detail: "点开段落工具：翻译、关键单词、语法、写作解析，一样一样看。"},
			{Kind: taskLabel, Label: "拆开作者的论证", Detail: "把几句话各自归到论证三要素里：论点、论据、论证。"},
			{Kind: taskCritique, Label: "你怎么看", Detail: "作者说的你同意吗？他给的证据够不够？有没有另一种解释？挑一个角度说。"},
			{Kind: taskConnect, Label: "你原来是怎么想的", Detail: "读之前你对这件事是什么印象？读完之后变了没有？"},
			// 🚨 复述在前，找句在后，而且是这个顺序才对：她先凭记忆说一遍，
			// 再回文章里核对自己说得准不准。反过来（点完句子再合上文章复述）
			// 是让她把刚看过的那一句背一遍，什么都测不出来。
			// hunt 必须是最后一步 —— 见 reading_routines_internal_test.go：
			// 打字的答案可以凭印象给，点出来的句子不能。
			{Kind: taskRecall, Label: "合上文章复述", Detail: "先别看原文：作者的主张一句话，加上你记住的两三个表达。"},
			{Kind: taskHunt, Label: "找出中心论点", Detail: "现在回到文章里：作者直接说出主张的是哪一句？把它点出来，对照你刚才的复述。"},
		},
	},
	{
		// 2026-09-17 新加，同一天改了第二次。第一次：两套英文读法都带着「拆开
		// 作者的论证」，于是一篇战地新闻报道也被要求按「主张 / 证据」拆，而
		// 报道里一句作者的主张都没有（同事逐字：「我总觉得不是所有的文章都应该
		// 按照主张、证据、限制这样的内容来拆分」）。那时的做法是这一套里不放板。
		//
		// 第二次（同事的阅读模块 PRD）：报道有它**自己的**板 —— 事实 / 引述 /
		// 解释（coachGenreBoards），外加一条时间线。板没有错，错的是格子名。
		Key:    "en-report",
		Lang:   "en",
		Genres: []string{genreReport},
		Name:   "Read the Report",
		Blurb:  "英文新闻报道、人物特写的读法——作者不表态，只讲发生了什么、各方怎么说。",
		Steps: []readingRoutineStep{
			{Kind: taskPredict, Label: "先预测", Detail: "只看标题：这篇报的是一件什么事？"},
			{Kind: taskRead, Label: "通读全文", Detail: "先弄清楚两件事：发生了什么，牵涉到哪几方。"},
			// 2026-09-17：「谁在说这句话」那一步由一块板承担（事实 / 引述 / 解释），
			// 并补上时间线。见同事的阅读模块 PRD。
			{Kind: taskSequence, Label: "排出事件时间线", Detail: "把几件事按发生的先后排好。报道常常先讲结果，再回头交代经过。"},
			{Kind: taskFocusBlock, Label: focusBlockLabelBase, Detail: "点开段落工具：翻译、关键单词、语法，一样一样看。"},
			{Kind: taskLabel, Label: "分清事实与说法", Detail: "把几句话各自归类：记者核实的事实、某一方说的话、对事件的解释。"},
			{Kind: taskCritique, Label: "比较来源", Detail: "这篇报道有没有哪一方没被问到？哪一句你觉得还需要别的来源才敢信？"},
			{Kind: taskConnect, Label: "你原来是怎么想的", Detail: "读之前你对这件事是什么印象？读完之后变了没有？"},
			{Kind: taskRecall, Label: "合上文章复述", Detail: "先别看原文：这件事一句话讲完，加上你记住的两三个细节。"},
			{Kind: taskHunt, Label: "找出关键句", Detail: "现在回到文章里：哪一句最能对上你刚才那句复述？把它点出来。"},
		},
	},
	{
		Key:    "en-explain",
		Lang:   "en",
		Genres: []string{genreExplain},
		Name:   "Understand the Explanation",
		Blurb:  "英文说明文、科普文章的读法——讲清楚一样东西是什么、怎么运作。",
		Steps: []readingRoutineStep{
			{Kind: taskPredict, Label: "先预测", Detail: "只看标题：这篇要说明的对象是什么？"},
			{Kind: taskRead, Label: "通读全文", Detail: "遇到不认识的词先跳过，先找到说明对象。"},
			{Kind: taskFocusBlock, Label: focusBlockLabelBase, Detail: "这一段讲的是关键的概念或原理。点开段落工具：翻译、关键单词、语法。"},
			{Kind: taskLabel, Label: "理清说明结构", Detail: "把几句话各自归类：说明对象、原理与过程、例子与数据。"},
			{Kind: taskCritique, Label: "你怎么看", Detail: "这篇解释清楚了吗？哪一环还没讲透，例子撑不撑得住？挑一处说。"},
			{Kind: taskReflect, Label: "解释关键关系", Detail: "用你自己的话说清楚：文中的一个原因是怎么导致那个结果的？"},
			{Kind: taskConnect, Label: "换个情境用一用", Detail: "把文中的原理放到另一个情境里，它还成立吗？"},
			{Kind: taskHunt, Label: "找出关键句", Detail: "回到文章里：哪一句最能概括这个原理？把它点出来。"},
		},
	},
	{
		Key:    "en-narrative",
		Lang:   "en",
		Genres: []string{genreNarrative},
		Name:   "Follow the Story",
		Blurb:  "英文记叙文、小说片段、人物故事的读法——有人、有事、有转折。",
		Steps: []readingRoutineStep{
			{Kind: taskRead, Label: "通读全文", Detail: "遇到不认识的词先跳过，先弄清楚谁、在哪儿、发生了什么。"},
			{Kind: taskSequence, Label: "排出事件顺序", Detail: "把几件事按发生的先后排好，再和文章讲述的顺序对照。"},
			{Kind: taskFocusBlock, Label: focusBlockLabelBase, Detail: "事情在这里发生了转折。点开段落工具：翻译、关键单词、写作解析。"},
			{Kind: taskLabel, Label: "看人物怎么写", Detail: "把几句话各自归到一种描写：动作、语言、心理、环境。"},
			{Kind: taskCritique, Label: "人物为什么这样做", Detail: "结合前后的行为，说出你对人物动机的解释，并指出原文依据。"},
			{Kind: taskConnect, Label: "角色选择", Detail: "如果遇到相同情境，你会如何处理？"},
			{Kind: taskRecall, Label: "合上文章复述", Detail: "先别看原文：这个故事一句话讲完，加上你记住的两三个表达。"},
			{Kind: taskHunt, Label: "找出关键句", Detail: "现在回到文章里：你觉得写得最好的是哪一句？把它点出来。"},
		},
	},
	{
		Key:    "en-argument",
		Lang:   "en",
		Genres: []string{genreArgument},
		Name:  "Follow the Argument",
		Blurb: "适合英文议论文、社论、TOEFL 阅读——作者在说服你的时候用。",
		Steps: []readingRoutineStep{
			{Kind: taskPredict, Label: "先预测", Detail: "只看标题：作者大概站哪一边？正文先别读。"},
			{Kind: taskRead, Label: "通读全文", Detail: "先找出作者站哪一边。"},
			{Kind: taskFocusBlock, Label: focusBlockLabelBase, Detail: "点开段落工具，看他是怎么把话说重的。"},
			{Kind: taskLabel, Label: "拆开作者的论证", Detail: "把几句话各自归类：作者的论点、他驳的那个观点、论据、论证。"},
			{Kind: taskCritique, Label: "你怎么看", Detail: "作者说的你同意吗？他给的证据够不够？有没有另一种解释？挑一个角度说。"},
			{Kind: taskConnect, Label: "观点变化", Detail: "读之前你自己是什么立场？作者动摇你了吗，还是让你更确定了？"},
			{Kind: taskHunt, Label: "找出关键句", Detail: "文中哪一句最没说服你？把它点出来。"},
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
	//   "words"     — 一组词卡（词 / 词性 / 在这句里的意思 / 讲解 / 例句）。
	//                 每个 term 都要回段落里逐字核对，核不上的丢掉 —— 那次核对
	//                 正是荧光笔能落在正文上的全部依据。
	//
	// questions / imitate 是 铁律① 由**输出类型**守着，而不是靠劝模型收敛 ——
	// 和写作室那个引导框同一个招。
	Shape string `json:"shape"`
	// MaxRunes 是这件工具最多写多少字，0 = 用默认的 200
	// （readingBlockDefaultMaxRunes）。讲解越长她越不读，所以多给字数要有理由：
	// 现在只有「写作解析」多给，因为 2026-09-17 把「把握度」并进了它。
	MaxRunes int `json:"-"`
	// Class 是这件工具走哪一个能力档。空 = dialogue（学生当场看得见的一轮）。
	//
	// 「查词」走 digest：读一段、给一张短卡、不评判她 —— 正是那一档的合同
	// （长输入短输出，压缩而不判断），而它绑着更便宜的模型。产品负责人
	// 2026-09-17：「for word analysis, we can use cheaper models.」
	// 🚨 不是 reflex：reflex 的合同是「一个标签，没有自由文本」，而且
	// catalog.go 写明它跑的那个最便宜的模型中文不够顺 —— 词卡是她要读的中文。
	Class string `json:"-"`
	// Subject 说这件工具讲的是哪一级：空 = 整段，"sentence" = 她点的那一句，
	// "word" = 她点的那一个词（2026-09-17）。
	//
	// 语法 2026-09-16 改成按句子讲。产品负责人的原话：
	//
	//	> 语法 - we should teach grammar in a sentence level. namely, we ask
	//	> student to select a sentence, and we teach that. currently we only
	//	> have paragraph level which is strange.
	//
	// 这一位同时决定三件事：界面先请她点一句、prompt 里给的是那一句、缓存的键
	// 把那一句算进去。
	Subject string `json:"subject,omitempty"`
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
		// 2026-09-16：从一段散文改成一组词卡。散文没法变成卡片，更没法回到正文
		// 里把那个词标出来 —— 要标，就得知道**哪几个字**是那个词。
		Shape: "words", ID: "vocabulary", Label: "关键单词", Lang: "en",
		Instruction: "挑出这一段里**真正值得学**的 3–5 个词（不是最长的，是最有用的、在这里意思特别的）。" +
			"不要把整段的词都列出来，那是词典干的事；也不要挑初中就学过的。",
	},
	{
		// 🚨 2026-09-17 新增：点一个词，讲这一个词。产品负责人逐字：
		// 「we should be able to select a sentence to ask grammar, select a word
		// to ask the meaning. tool bar word level - word meaning.」
		//
		// 「关键单词」是模型替她挑的词；这一件是**她自己**卡住的那个词 ——
		// 后者才是她真正不认识的。产物和关键单词是同一种卡（shape "words"），
		// 所以正文里的荧光笔、卡片的样子、报告里的生词表都不用另写一份。
		Shape: "words", ID: "lookup", Label: "查词", Lang: "en", Subject: "word",
		Class: gateway.ClassDigest,
		Instruction: "她点了这一段里的**一个词**（见下面【要讲解的这一个词】），只讲这一个词，给**一张**词卡。" +
			"讲的是它在**这一句里**的意思；它是一个词组的一部分时，term 写整个词组（仍然要逐字出现在段落里）。",
	},
	{
		// 2026-09-16：按句子讲，不再讲整段。见 readingBlockTool.Subject。
		// 2026-09-17：从一段散文改成一张卡（shape "grammar"）。产品负责人：
		// 「like words, become a card. sentence composition split? grammar
		// points? with highlighting, knowledge point, cases」。结构见
		// reading_block_grammar.go。
		Shape: "grammar", ID: "grammar", Label: "语法", Lang: "en", Subject: "sentence",
		// 2026-09-17 晚些改成三层：句法（主句/从句、句子成分）、词法（关键词）、
		// 时态。见 reading_block_grammar.go 的文件头。
		Instruction: "讲这一句语法上的重点：从句法（从句、句子成分）、词法、时态里挑这一句最值得学的一两层，" +
			"只标关键的几处。最后说这一句是什么意思。",
	},
	{
		// 🚨 2026-09-17：「把握度」那件工具**并进来了**，不再单独占一个按钮。
		// 产品负责人逐字：「把握度 is difficult to understand. let's make it a
		// part of 写作解析.」—— 它是一个我们自己造的名字，她在别处没见过，
		// 而它讲的事（作者说得有多满）本来就是写法的一部分。
		//
		// 并进来的那一半来自 english-close-reading-skill 的那一维（见
		// docs/2026-09-10-reading-guidance-redesign.md）：**分清作者声称什么和
		// 他证明了什么**。英文里那个梯子写在动词上，看动词就能判，所以它是
		// 可核对的，不是一个由模型自由发挥的印象。这一半不能丢，丢了这件工具
		// 就退回「这一段在干什么」一句话。
		//
		// 一件工具讲两件事，200 字装不下 —— 所以这一件自己带一个上限。
		Shape: "prose", ID: "craft", Label: "写作解析", Lang: "en", MaxRunes: 320,
		Instruction: "分两小段讲。\n" +
			"**这一段在干什么**：它在整篇里承担什么（提出主张、举例、让步、转折、收束……），" +
			"以及作者用什么手法让它起作用。两三句话，说的是写法，不是内容摘要。\n" +
			"**作者说得有多满**：英文里把握度写在动词上，从强到弱是 " +
			"shows / demonstrates（证明）> found / reported（报告了观察到的事）> " +
			"suggests / indicates（提示）> may / could / might（可能）> " +
			"is associated with（只是同时出现，不是因果）。" +
			"从这一段里挑 1–2 处，摘出那个动词或短语，说清它属于哪一级，" +
			"以及**如果换成更强的那一级，这句话会多说出什么**。" +
			"这一段全是叙述、没有主张，就直说这一段没有在下判断，不要硬找。",
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
		// 🚨 2026-09-18 同事（《敬业与乐业》第 8 段）：「这个结构解析工具基本没解析，
		// 按理来说现在应该在段落内，对句子划分结构，但是解析工具只是整体概述了一下」。
		// 原来的说明只问「这一段在整篇里在干什么」，模型照做了 —— 给的就是一句概述。
		// 改成先拆段内的层次（哪几句是一层、这一层在干什么、层与层怎么接），
		// 最后才用一句话说这一段在全文的位置。
		Shape: "prose", ID: "structure", Label: "结构解析", Lang: "zh", MaxRunes: 360,
		Instruction: "把这一段**按句子拆成几个层次**，写成编号列表，一层一行：" +
			"先引这一层开头的几个字（加「」，逐字照抄原文），注明是第几句到第几句，" +
			"再说这一层在干什么（提出观点、举例、引用、设问、反驳、让步、推论、总结……）" +
			"以及它和上一层是什么关系（递进、转折、因果、并列、承接……）。" +
			"列表之后用一句话说这一段在全文里的位置和作用。说的是结构，不是内容摘要。",
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
