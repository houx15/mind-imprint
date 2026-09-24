package api

import (
	"mindimprint/api/internal/gateway"
	"mindimprint/api/internal/guidance"
)

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

	// 看作者怎么安排这篇：叙述顺序、详略、线索。
	//
	// 🚨 2026-09-23 产品负责人第 5 条：「when reading a 记叙文, it always
	// focuses on very detailed things and ignores the general structure,
	// the writing format etc. to give guidance.」
	//
	// 她是对的，而且原因在清单里**量得出来**：zh-narrative 那一套七步里，
	// 除了 sequence 之外每一步都是句子或段落那一层的（精读一段、给几句话
	// 贴描写类型、说一个人物的动机、点一句写得好的），而它是九套读法里唯一
	// 既没有 predict 也没有 reflect 的一套 —— **整篇那一层根本没有位置**。
	// 陪练不是不想谈结构，是清单上没有一步请它谈。
	//
	// 和 sequence 分开，因为它们问的是两件事：sequence 排的是**事情**的
	// 先后（那块板），shape 问的是**作者**为什么这样排。合在一步里她只会
	// 做板那一半 —— 和 label/critique 当初必须拆开是同一个理由。
	taskShape readingTaskKind = "shape"
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
// 2026-09-22：挑哪一套由 internal/guidance 那条共用规则决定。行为不变 ——
// 挑对了不动，挑错了换成同语言里服务这个体裁的第一套。
func pickRoutineForGenre(chosen readingRoutine, genre string) readingRoutine {
	genre = validateGenre(genre)
	if genre == "" || chosen.serves(genre) {
		return chosen
	}
	rows := make([]guidance.Row[readingRoutine], 0, len(readingRoutines))
	for _, r := range readingRoutines {
		// serves() 把空 Genres 当成「谁都不服务」，而 Scope.Matches 把它当成
		// 「不限」—— 同一个字段两种读法。今天 9 套读法都写了 Genres，两边
		// 读出来一样；但只查 Lang 的话，未来一套 Genres: nil 的读法会被
		// Scope.Matches 判成「服务所有体裁」，悄悄顶替模型的选择。这里按
		// 窄的那一种办，两条规则不许分家。
		if r.Lang != chosen.Lang || !r.serves(genre) {
			continue
		}
		rows = append(rows, guidance.Row[readingRoutine]{
			Scope: guidance.Scope{
				Surface: guidance.SurfaceRead,
				Lang:    r.Lang,
				Genres:  r.Genres,
			},
			Value: r,
		})
	}
	k := guidance.Key{Surface: guidance.SurfaceRead, Lang: chosen.Lang, Genre: genre}
	if fitted, ok := guidance.Pick(k, rows); ok {
		return fitted
	}
	// 同语言里没有服务这个体裁的 —— 保留模型挑的那一套，
	// 给她一份读不对的清单，也好过给她一份读不懂的。
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
			// 🚨 R3「AI 通读全文、划分内容单元 → 学生分段通读并概括 →
			// **拼出全文结构** → 按文章类型精读 → 整理阅读成果」里的第三段。
			//
			// 2026-09-25 产品负责人点头之后加的。在这之前它撞在
			// 「don't bother the current experience of argument papers」
			// （2026-09-17）那条禁令上，2026-09-24 做出来又撤了一次 ——
			// 那一次的撤回是对的：禁令还在的时候，测试就该拦住它。
			//
			// 用 reflect 而不是新开一个 kind：加 kind 是代码改动（见上面那段
			// 注释），而「把几部分合起来想一想」正是 reflect 的意思。
			// 它指向正文上方那张论证图 —— 整篇那一层 2026-09-24 才有，
			// 这一步以前就算写出来也没有东西可对照。
			{Kind: taskReflect, Label: "拼出全文结构", Detail: "把刚才读过的几部分合起来：这篇文章分成哪几块，每一块在做什么？可以打开正文上方的「论证图」对照。"},
			{Kind: taskFocusBlock, Label: focusBlockLabelBase, Detail: "这一段值得细读。请打开段落工具，分析其中的内容与写法。"},
			{Kind: taskLabel, Label: "拆开作者的论证", Detail: "把几句话各自归到论证三要素里：论点、论据、论证。"},
			{Kind: taskCritique, Label: "你怎么看", Detail: "作者说的你同意吗？作者给的证据够不够？有没有另一种解释？挑一个角度说。"},
			{Kind: taskReflect, Label: "总结论点", Detail: "用你自己的话总结本文的论点：作者的中心论点是什么？作者分几层来论证？"},
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
		// 🚨 2026-09-24 按产品负责人给的《记叙文和散文》how-to 重排
		// （docs/reference/writing-teaching/how-to/）。那一份是老师写的线上
		// 带读步骤，八步：检查字词 → 初读勾画概括主要事件 → 理清结构找切入点 →
		// 用主问题研读重点段 → 分析人物和细节描写 → 梳理情感变化 →
		// 朗读关键段落 → 读写结合。
		//
		// 它同时回答了她 2026-09-23 报的那一条（「when reading a 记叙文, it
		// always focuses on very detailed things and ignores the general
		// structure」）。那一份 how-to 的收尾里逐字写着办法：
		//
		//	用一个主问题带动重点段，下面拆成小问题逐个反馈，避免逐句讲解。
		//	先读后问，每个问题都要求学生从原文找依据。
		//
		// 所以精读那一步的措辞从「请看作者是怎么写的」改成一个**主问题**，
		// 并明说不逐句讲。
		Steps: []readingRoutineStep{
			{Kind: taskPredict, Label: "先预测", Detail: "只看标题：这篇大概会讲一件什么事？"},
			// 概括给公式，是 how-to 的原话「概括给出公式，降低难度」，
			// 公式本身也是它给的：人物＋起因＋经过＋结果。
			{Kind: taskRead, Label: "初读并概括", Detail: "先把故事看完，边读边把人物、事件、地点勾出来。再用一句话概括：谁＋起因＋经过＋结果。"},
			{Kind: taskSequence, Label: "排出事件顺序", Detail: "把几件事按发生的先后排好，再和文章讲述的顺序对照。"},
			// 「从标题或关键句切入，把全文串起来」+「切入问题要具体、能从原文
			// 找到答案」。how-to 举的例子是从「背影」二字切入：先问共写了几次，
			// 再问买橘子那次为什么详写。
			{Kind: taskShape, Label: "理清结构，找切入点", Detail: "先划出层次。再从标题或一个反复出现的词切入：它在文中出现了几次？哪一次写得最细，哪几段几句话就带过去了？"},
			// 🚨 这一步是那条抱怨的正面回答：一个主问题，不是逐段逐句。
			{Kind: taskFocusBlock, Label: focusBlockLabelBase, Detail: "这一段是全文最详写的地方。先回答一个问题：作者为什么在这里写得这么细？答完再往下拆。"},
			// 「要求学生先在原文中找到句子，再说人物特点，避免空答」。
			{Kind: taskLabel, Label: "分析人物与细节", Detail: "先在原文里找出写这个人的句子，再把它们各自归到一种描写：动作、语言、心理、环境。说人物特点之前先把句子指出来。"},
			{Kind: taskCritique, Label: "人物为什么这样做", Detail: "结合前后的行为，说出你对人物动机的解释，并指出原文依据。"},
			// how-to 第 6 步「梳理情感变化」：找出叙述者态度或情感变化的句子，
			// 排出变化过程。它举的例子是《背影》里「不理解 → 顿悟 → 感激 → 感念」。
			{Kind: taskReflect, Label: "梳理情感变化", Detail: "找出叙述者态度变化的那几句，按先后说出变化过程：从什么变成了什么，转在哪一句。"},
			// how-to 第 8 步「读写结合」，写作提示也是它的原话：
			// 「写人记事要选最动情的一件事，写这件事要突出最动情的瞬间」。
			{Kind: taskConnect, Label: "读写结合", Detail: "学这篇的写法，写一个自己的生活片段：选最动情的一件事，把最动情的那一个瞬间写出来。"},
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
			{Kind: taskFocusBlock, Label: focusBlockLabelBase, Detail: "这一段值得细读。请打开段落工具，分析其中的内容与写法。"},
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
			{Kind: taskCritique, Label: "你怎么看", Detail: "请选择一处，说明作者的解释或例子是否帮助你理解了原理。"},
			{Kind: taskReflect, Label: "解释关键关系", Detail: "用你自己的话说清楚：文中的一个原因是怎么导致那个结果的？"},
			{Kind: taskConnect, Label: "换个情境用一用", Detail: "把文中的原理放到另一个情境里，它还成立吗？会有什么不同？"},
			{Kind: taskHunt, Label: "找出关键句", Detail: "文中哪一句最能概括这个原理？把它点出来。"},
		},
	},
	{
		// 🚨 2026-09-24 新增。散文成为一种**阅读**体裁。
		//
		// reading-suggestion.md 里「散文、诗歌……我觉得这个用ai来有点太难了，
		// 先列一下放着」那一句是旧的 —— 产品负责人 2026-09-24 逐字：
		// 「this is very old... and we are about to do them now.」
		//
		// 步骤照她给的那份《记叙文和散文》how-to 的散文八步：
		// 听读自读定感情基调 → 了解作者和背景 → 梳理线索和思路 →
		// 品读重点段抓景物或物象特点 → 品味语言 → 体会情感理清情感脉络 →
		// 理解主旨 → 背诵积累。
		//
		// 🚨 它和记叙文**不是同一套读法换个说法**，轴不一样：
		// how-to 的原话是「记叙文侧重事件和人物，散文侧重线索、景物或物象和情感」。
		// 所以这一套里没有 sequence（散文没有一件要排先后的事），
		// 而多了「线索」这一步 —— 那是散文的脊梁。
		Key:    "zh-prose",
		Lang:   "zh",
		Genres: []string{genreProse},
		Name:   "读散文",
		Blurb:  "散文的读法：一条线索串起几个片段，落在情感上（写景抒情、写物言志）。",
		Steps: []readingRoutineStep{
			// 「只介绍理解这篇文章必需的背景，其余放到理解情感时再补」——how-to 第 2 步。
			{Kind: taskPredict, Label: "读前：看题目和作者", Detail: "只看题目和作者：这一篇大概写什么，你猜它的调子是明亮的还是低沉的？正文先别读。"},
			// how-to 第 1 步：先自读，说出初步感受，确定感情基调。
			{Kind: taskRead, Label: "自读，定感情基调", Detail: "先把全文读一遍，不急着分析。读完说一个词：这篇文章整体是什么调子（明亮、宁静、怅惘、深情……）？"},
			// 🚨 how-to 第 3 步带一条明确的提醒：「线索和思路要分开问，学生常把
			// 两者混在一起」。所以这一步把两个问题**分开写出来**。
			{Kind: taskShape, Label: "梳理线索和思路", Detail: "分两个问题答。思路：先写什么、后写什么？线索：全文围绕什么展开 —— 一样东西、一处景、一种情感、一条路线、还是一段时间？"},
			// how-to 第 4 步：概括所写景物或物象及其特点，并给概括的格式要求。
			{Kind: taskFocusBlock, Label: focusBlockLabelBase, Detail: "这一段是全文写景（或写物）最用力的地方。用一个偏正短语概括它写的是什么（例如「月下的荷塘」），再说这个景物有什么特点。"},
			{Kind: taskLabel, Label: "分清写景与抒情", Detail: "把几句各自归到一种：写景、叙事、抒情、议论。"},
			// how-to 第 5 步：赏析修辞、动词、叠词；「可以用换词比较的方式出题，
			// 让学生判断原词好在哪里」。
			{Kind: taskCritique, Label: "品味语言", Detail: "挑一处你觉得写得好的：一个动词、一个叠词、一处比喻。把它换成一个平常的说法，再说换完之后丢了什么。"},
			// how-to 第 6 步 + 第 7 步。「形散神聚」：形指人物、事物、景物、器物，
			// 神指要表现的理念、情感、哲理或志趣。学生答得笼统时追问具体对象和原因。
			{Kind: taskReflect, Label: "从景到情，说出主旨", Detail: "先找出直接抒情的那几句，说出情感从什么变成了什么。再说这篇文章借这些景物要表达的是什么 —— 说具体：对什么的情感，因为什么。"},
			{Kind: taskConnect, Label: "你有没有见过这样的景", Detail: "文中那一处景，你自己在什么时候见过类似的？当时是什么心情？"},
			// how-to 第 8 步是背诵积累。我们没有背诵这一步，但「积累优美语句」
			// 正好是 hunt 在做的事，所以收尾换成积累的说法。
			{Kind: taskHunt, Label: "摘下值得积累的一句", Detail: "全文哪一句最值得抄下来记住？把它点出来。"},
		},
	},
	{
		// 🚨 2026-09-23 新增。产品负责人：「since we will face reading
		// poems/文言文 in chinese reading…… these are very important scene in
		// junior study.」在这之前一首绝句会落到上面四种里的某一个 ——
		// 被当成「记叙」去排事件顺序，或者被当成「说明」去找说明对象。
		//
		// 步骤照她给的那份诗歌鉴赏讲义（shige/SKILL.md 的「多维分析」）：
		// 白话翻译 → 情感脉络 → 意象拆解 → 手法识别 → 典故背景。
		// 讲义里那条输出原则也搬进了判据：**「为什么好」比「用了什么」重要** ——
		// 所以「你怎么看」那一步问的不是它用了什么手法，是那个手法在这里做成了
		// 什么。
		//
		// 🚨 没有 sequence 那一步：一首诗里没有「几件事的先后」可排。
		Key:    "zh-poem",
		Lang:   "zh",
		Genres: []string{genrePoem},
		Name:   "读一首诗",
		Blurb:  "古诗词的读法：先读懂字面，再看它怎么把情感写出来（绝句、律诗、词、曲）。",
		// 🚨 2026-09-24 按产品负责人给的 how-to-read-a-poem.md 重排。
		// 那一份是「由外到内、由粗到细」的七步：读题目定方向 → 知人论世 →
		// 疏通字面 → 圈意象拼画面体会意境 → 找诗眼抓情感 → 分析手法 →
		// 回到整体评价与联系。它的收尾点名了两步：
		//
		//	尤其是第四步和第六步，最容易跳过也最能拉开差距。
		//
		// 所以圈意象（第四步）从原来埋在精读里，提成自己一步。
		Steps: []readingRoutineStep{
			// 🚨 题材判断是一张**查表**，不是印象：how-to 给的就是这几条。
			// 题材一定，情感方向就有了预判，后面的阅读是去验证和细化它。
			{Kind: taskPredict, Label: "读题目，定方向", Detail: "只看题目：带「送、别、赠」多是送别诗；带「登、望、怀古」多是登临怀古；带「夜泊、旅、宿」多是羁旅诗；「咏」或「赋得」加一样东西多是咏物诗。先判题材，再猜它大概写什么情感。正文先别读。"},
			{Kind: taskRead, Label: "疏通字面", Detail: "先把每一句写了什么说清楚。遇到倒装、省略、互文先把语序还原。这一步不分析，只把画面和事情理顺。"},
			// 🚨 第四步，how-to 点名的两个差距点之一。
			//
			// 这里**不给意象的固定含义**，而是让她按五组二选一自己判：
			// 语料里根本没有意象字段（307 条记录里「意象」零次），三份来源
			// 加起来只有七条配对，其中四条还只是某一首诗的局部读法。
			// 让模型运行时自己生成意象含义，正是那份赏析讲义的负例
			// 「鉴赏虚构意象」。而冷暖／远近／动静是她自己看得出来的。
			{Kind: taskLabel, Label: "圈意象，拼画面", Detail: "把诗里的景物、人物、事物一个个圈出来。再给每一个判三件事：远还是近、动还是静、冷色还是暖色。最后说这些东西凑在一起是什么氛围。"},
			{Kind: taskShape, Label: "看这一首怎么安排", Detail: "绝句看起承转合、末句有没有翻转；律诗看对仗和中间两联；词看上下阕怎么接。这一首的转在哪一句 —— 看哪一句换了主体、换了时间、换了视角，或者由景入情。"},
			{Kind: taskFocusBlock, Label: focusBlockLabelBase, Detail: "这几句里有这一首的诗眼。先找出最能透露情感的那个字或那一句：可能是直接写情的字（愁、恨、喜、怜），也可能是最传神的那个动词。"},
			// 🚨 第六步，how-to 点名的另一个差距点。它的原话：
			// 「关键是说明手法和情感之间的关系，而不是只贴标签。」
			// 切入顺序也是它给的：情景关系 → 描写方法 → 修辞与结构。
			{Kind: taskCritique, Label: "分析手法", Detail: "按这个顺序看：先看情景关系（借景抒情、情景交融、以乐景写哀情），再看描写方法（动静、虚实、远近），最后看修辞与结构（对比、用典、以景结情、首尾呼应）。说出手法的名字之后必须接着说它和情感是什么关系 —— 只贴一个标签不算。"},
			{Kind: taskReflect, Label: "把握情感", Detail: "用你自己的话说清楚：这一首是什么情，因何而起，是单一的还是几种交织在一起？"},
			// how-to 第七步「回到整体，评价与联系」：在同题材作品里有什么独特
			// 之处，能联想到哪些相似的诗。这是从「读懂」到「读透」。
			{Kind: taskConnect, Label: "回到整体，联系开去", Detail: "这一首好在哪里？同样写这个题材的诗你还读过哪一首，两首放在一起有什么不一样？"},
			{Kind: taskHunt, Label: "找出关键句", Detail: "这一首里最经得起回味的是哪一句？把它点出来。"},
		},
	},
	{
		// 🚨 2026-09-23 新增，理由同上。
		//
		// 步骤照她给的那份古文讲义（guwen/SKILL.md 的「四层译讲法」）：
		// 逐句白话 → 关键字词（通假 / 古今异义 / 专名 / 典故）→
		// 句法（判断句 / 宾语前置 / 被动 / 省略）→ 背景与寓意。
		//
		// 讲义里那条「分层译讲，不堆砌」也是这套读法的形状：字面先通，
		// 再谈道理。反过来（先讲寓意再回来抠字）她会把译文当成结论背下来。
		Key:    "zh-classical",
		Lang:   "zh",
		Genres: []string{genreClassical},
		Name:   "读文言文",
		Blurb:  "文言文的读法：先把字面读通，再看它在讲什么道理（古文、史传、寓言）。",
		// 🚨 2026-09-24 按产品负责人给的《文言文阅读》how-to 重排。
		// 那一份是**专门为线上带读写的**，而且自己点出了和课堂版的差别：
		//
		//	顺序：课堂是整体感知、疏通、研读；线上是逐段疏通、回到整体、研读。
		//	主动性：每一步先让学生作答，再给答案。
		//	颗粒度：一次一段、一个重点、一个问题。
		//
		// 🚨 其中第四步是一条**对陪练的硬约束**：
		//
		//	学生先试译，再给反馈……不直接给整段译文。先检查一两个关键词，
		//	答对再给全句；答错给提示。
		//
		// 所以疏通那一步的措辞是「你先说」，而不是陪练先讲。
		// （划选之后那颗「白话翻译」按钮不受这条约束 —— 那是她主动来取第一层，
		// 正是那份古文讲义说的「用户要哪层给哪层」。这里管的是陪练带读的顺序。）
		Steps: []readingRoutineStep{
			// 「简要介绍作者、文体和出处，给出一个带着读的问题」+
			// 「背景不宜一次讲多，其余的放到理解主旨时再补」。
			{Kind: taskPredict, Label: "读前：交代背景", Detail: "先看作者、文体和出处。带着一个问题去读：这一篇是以什么为线索写下来的？正文先别细看。"},
			{Kind: taskRead, Label: "逐段疏通", Detail: "一段一段来，先读准字音和句读。每一段你先自己试着讲一遍意思，讲不通的地方标出来 —— 先说你的，再对答案。"},
			// 「每段挑一两个重点实词、虚词或特殊句式」+「点到即止」。
			{Kind: taskFocusBlock, Label: focusBlockLabelBase, Detail: "这一段挑一两处重点：一个实词或虚词，一处特殊句式。划选那几个字，用「白话翻译」「查字」「句法」。一次只看一处。"},
			{Kind: taskLabel, Label: "分清叙事与议论", Detail: "把几句各自归到一种：叙事、描写、议论、对话。"},
			// 「回到整体，理解内容」：串联各段概括，理清线索；就内容提问，
			// 要求从原文找依据。how-to 举的例子是用最简洁的词概括桃花源的
			// 整体印象（美、乐、奇）。
			{Kind: taskShape, Label: "回到整体，看章法", Detail: "把各段的小结串起来。这一篇是先叙后议，还是借一件事说一个道理？「议」是从哪一句开始的 —— 找那一句不再讲发生了什么、而开始给评价的话。"},
			{Kind: taskCritique, Label: "你怎么看", Detail: "作者的说法你同意吗？他举的那件事撑得住他的结论吗？挑一个角度说，并指出原文依据。"},
			{Kind: taskReflect, Label: "分析写法与主旨", Detail: "用你自己的话说清楚：作者想让人明白的是什么？他是靠哪一句、哪一件事让人明白的？"},
			{Kind: taskConnect, Label: "换到今天", Detail: "这一篇讲的道理，放到今天你身边的事情上还成立吗？"},
			{Kind: taskHunt, Label: "找出关键句", Detail: "全篇哪一句最能说明作者的意思？把它点出来。"},
		},
	},
	{
		Key:    "en-close-read",
		Lang:   "en",
		Genres: []string{genreArgument},
		Name:   "Close Read",
		Blurb:  "英文文章的默认读法：先看懂，再看它是怎么写的。",
		Steps: []readingRoutineStep{
			{Kind: taskPredict, Label: "先预测", Detail: "请根据标题和第一句预测文章主题。"},
			{Kind: taskRead, Label: "通读全文", Detail: "遇到不认识的词先跳过，先理解大意。"},
			{Kind: taskFocusBlock, Label: focusBlockLabelBase, Detail: "点开段落工具：翻译、关键单词、写作解析，一样一样看。哪一句读不通，就划选那一句点「句子解析」。"},
			{Kind: taskLabel, Label: "拆开作者的论证", Detail: "把几句话各自归到论证三要素里：论点、论据、论证。"},
			{Kind: taskCritique, Label: "你怎么看", Detail: "作者说的你同意吗？作者给的证据够不够？有没有另一种解释？挑一个角度说。"},
			{Kind: taskConnect, Label: "你原来是怎么想的", Detail: "读之前你对这件事是什么印象？读完之后变了没有？"},
			// 🚨 复述在前，找句在后，而且是这个顺序才对：她先凭记忆说一遍，
			// 再回文章里核对自己说得准不准。反过来（点完句子再合上文章复述）
			// 是让她把刚看过的那一句背一遍，什么都测不出来。
			// hunt 必须是最后一步 —— 见 reading_routines_internal_test.go：
			// 打字的答案可以凭印象给，点出来的句子不能。
			{Kind: taskRecall, Label: "合上文章复述", Detail: "先别看原文：请用自己的话概括作者的观点，再回忆两三个文中的表达。"},
			{Kind: taskHunt, Label: "找出中心论点", Detail: "现在回到文章里：作者直接表达主要观点的是哪一句？把它点出来，对照你刚才的复述。"},
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
			{Kind: taskFocusBlock, Label: focusBlockLabelBase, Detail: "点开段落工具：翻译、关键单词，一样一样看。哪一句读不通，就划选那一句点「句子解析」。"},
			{Kind: taskLabel, Label: "分清事实与说法", Detail: "把几句话各自归类：记者核实的事实、某一方说的话、对事件的解释。"},
			{Kind: taskCritique, Label: "比较来源", Detail: "这篇报道有没有哪一方没被问到？哪一句还需要其他来源核实？"},
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
			{Kind: taskFocusBlock, Label: focusBlockLabelBase, Detail: "这一段讲的是关键的概念或原理。点开段落工具：翻译、关键单词；哪一句读不通，就划选那一句点「句子解析」。"},
			{Kind: taskLabel, Label: "理清说明结构", Detail: "把几句话各自归类：说明对象、原理与过程、例子与数据。"},
			{Kind: taskCritique, Label: "你怎么看", Detail: "请选择一处，说明作者的解释或例子是否帮助你理解了原理。"},
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
			// 2026-09-23：和 zh-narrative 同一条理由补上 predict 和 shape。
			// 英文那一套原来同样没有整篇那一层（它有 recall，但复述问的是
			// 「你记住了什么」，不是「作者怎么安排的」）。
			{Kind: taskPredict, Label: "先预测", Detail: "只看标题：这篇大概会讲一件什么事？"},
			{Kind: taskRead, Label: "通读全文", Detail: "遇到不认识的词先跳过，先弄清楚谁、在哪儿、发生了什么。"},
			{Kind: taskSequence, Label: "排出事件顺序", Detail: "把几件事按发生的先后排好，再和文章讲述的顺序对照。"},
			{Kind: taskShape, Label: "看作者怎么安排这篇", Detail: "这篇是按事情发生的先后讲下来的，还是先写了后面的事再回头补？哪一段写得最细，哪几段几句话就带过去了？"},
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
		Name:   "Follow the Argument",
		Blurb:  "适合英文议论文、社论、TOEFL 阅读——作者在说服你的时候用。",
		Steps: []readingRoutineStep{
			{Kind: taskPredict, Label: "先预测", Detail: "只看标题：请预测作者可能持有什么观点，暂不阅读正文。"},
			{Kind: taskRead, Label: "通读全文", Detail: "请先找出作者对这个问题的观点。"},
			// 同 zh 那一套：R3 的「拼出全文结构」，见上面那段注释。
			{Kind: taskReflect, Label: "拼出全文结构", Detail: "把刚才读过的几部分合起来：这篇文章分成哪几块，每一块在做什么？可以打开正文上方的「论证图」对照。"},
			{Kind: taskFocusBlock, Label: focusBlockLabelBase, Detail: "请打开段落工具，分析作者怎样表达观点、使用理由。"},
			{Kind: taskLabel, Label: "拆开作者的论证", Detail: "把几句话各自归类：作者的论点、作者反驳的观点、论据、论证。"},
			{Kind: taskCritique, Label: "你怎么看", Detail: "作者说的你同意吗？作者给的证据够不够？有没有另一种解释？挑一个角度说。"},
			{Kind: taskConnect, Label: "观点变化", Detail: "读之前你自己是什么立场？文章是否改变了你的看法？"},
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
	// Genres 把一件工具收窄到某几种体裁（reading_outline.go 的闭表）。
	// 空 = 这一语言下的每一篇都给。
	//
	// 🚨 2026-09-23 加的。在这之前工具只按语言挑，于是「字词释义」「句法」
	// 这两件只有文言文用得上的工具会出现在每一篇中文文章上 ——
	// 一篇现代散文的段落工具条上摆着「通假字、古今异义」是没有意义的按钮，
	// 而没有意义的按钮会让她不信任整条工具条。
	Genres []string `json:"-"`
	// NotGenres 把一件**通用**工具从某几种体裁上撤掉。空 = 不撤。
	//
	// 🚨 它和 Genres 在两件事上相反，两件都是故意的：
	//
	//  1. 方向相反 —— Genres 是白名单，这是黑名单。
	//  2. **体裁认不出来时的默认相反。** 白名单在体裁为空时不给
	//     （宁可少一件，也不摆一个按下去讲不出东西的按钮）；黑名单在体裁为空时
	//     照给。因为它修饰的是一件本来每篇都有的通用工具，认不出体裁就撤掉它，
	//     等于把老数据上本来有的东西拿走了。
	//
	// 2026-09-24：「结构解析」和「案例」因此从诗词上撤掉。产品负责人走查
	// 《江雪》时这两件都在工具条上，而它们问的是：
	//
	//	把这一段按句子拆成 2 到 5 个层次 …… 这一段举了哪些具体的事例、数据或引用
	//
	// 「千山鸟飞绝，万径人踪灭。」十个字里没有层次可拆，也没有数据可举。
	// 诗词的结构是整首的起承转合，而那一层根本不在「段落」这个尺度上 ——
	// 《江雪》被切成了两段，任何一件段落工具都看不见整首诗。
	NotGenres []string `json:"-"`
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
	// Scope 说这件工具读的是多大一块：空 = 一个段落，"article" = 整篇。
	//
	// 🚨 2026-09-24 加的，产品负责人：「for poems or 记叙文, we also need a
	// whole-level view of analysis (actually, all need that, the article
	// structure, etc.」
	//
	// 这不是「把段落工具的说明改长一点」能办到的事，有三件东西**结构上**不在
	// 段落这个尺度上：
	//
	//   - 详略安排：要比较各段的长短，一个段落自己比不出来。
	//   - 首尾呼应：要同时拿着第一段和最后一段。
	//   - 叙事的「转」：从一种心态到另一种，是一条跨段的链子。
	//
	// 《江雪》被切成两段，所以连「这首诗的起承转合」都没有任何一件段落工具
	// 看得见。这一位就是为此存在的。
	Scope string `json:"scope,omitempty"`
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
		Instruction: "从这一段选择 3–5 个值得讲解的词。" +
			"优先选择影响本段理解、含义特殊或用法值得学习的词，避免仅因单词较长而入选。",
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
		Instruction: "学生点了这一段里的**一个词**（见下面【要讲解的这一个词】），只讲这一个词，给**一张**词卡。" +
			"讲的是它在**这一句里**的意思；它是一个词组的一部分时，term 写整个词组（仍然要逐字出现在段落里）。",
	},
	{
		// 2026-09-16：按句子讲，不再讲整段。见 readingBlockTool.Subject。
		// 2026-09-17：从一段散文改成一张卡（shape "grammar"）。产品负责人：
		// 「like words, become a card. sentence composition split? grammar
		// points? with highlighting, knowledge point, cases」。结构见
		// reading_block_grammar.go。
		Shape: "grammar", ID: "grammar", Label: "句子解析", Lang: "en", Subject: "sentence",
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
			"**这一段在干什么**：它在整篇里承担什么（提出观点、举例、让步、转折、收束……），" +
			"以及作者用什么手法让它起作用。两三句话，说的是写法，不是内容摘要。\n" +
			"**作者表达的确定程度**：从本段选 1–2 处动词或短语，结合语境说明作者是在报告观察、提出解释，还是表达可能性。" +
			"例如 may / could / might 表示可能，suggests / indicates 常提示尚需判断的关系；shows / demonstrates 的力度仍取决于实际证据。" +
			"若出现 is associated with，说明它表达相关关系，单凭该词组不能确定因果；相关与因果不作为同一条确定程度的等级。" +
			"可以比较换成更确定的表达后，结论会增加哪些原文尚未证明的内容。只分析实际出现的表达；纯事件叙述无需附加观点强弱判断。",
	},
	{
		// 🚨 2026-09-23，文言文专用。产品负责人：「we will face reading
		// poems/文言文 in chinese reading」。
		//
		// 分类照她给的那份古文讲义（guwen/SKILL.md 的第二层）：
		// 通假字 / 古今异义 / 专有名词 / 典故。这四类是中学文言文真正要背的
		// 那几样，也是她读不通一句话时的四种原因。
		//
		// 只给文言文：一篇现代散文上摆「通假字」是个没有意义的按钮。
		Shape: "words", ID: "classical_words", Label: "字词释义", Lang: "zh",
		Genres: []string{genreClassical}, Subject: "sentence",
		Class: gateway.ClassDigest,
		Instruction: "挑这一句里最值得讲的两三个字词，每个给一张词卡。" +
			"每张写清楚它属于哪一类：通假字（在这里当作另一个字）、古今异义（今天的意思和这里不一样）、" +
			"专有名词（人名、地名、官职、年号）、典故。" +
			"讲的是它**在这一句里**的意思；有两种解释时把两种都写出来并说明各自的依据，不把一家之言说成定论。",
	},
	{
		// 🚨 2026-09-23，文言文专用。讲义的第三层：判断句 / 宾语前置 /
		// 被动 / 省略。「点到为止」也是讲义的原话 —— 一句话里把四种句式全讲
		// 一遍，一个中学生一样都记不住。
		Shape: "prose", ID: "classical_syntax", Label: "句法", Lang: "zh",
		Genres: []string{genreClassical}, Subject: "sentence", MaxRunes: 260,
		Instruction: "只讲这一句的句式，挑最要紧的一两处：判断句（……者……也）、宾语前置、被动、省略。" +
			"先指出是哪一种、在哪几个字上，再说清楚它省了什么或者哪两个词调了位置，" +
			"最后把这一句按今天的语序顺过来说一遍。没有特殊句式就直说这一句是正常语序，并把它的意思说一遍。",
	},
	{
		// 🚨 2026-09-24：**第一层**。产品负责人：「students may select some texts
		// and need the explanation/translation, like in the skills I provided you.」
		//
		// 那份古文讲义（guwen/SKILL.md）的第一条核心约束是
		//
		//	分层译讲，不堆砌：按「白话翻译 → 关键字词 → 句法 → 背景寓意」
		//	四层递进；用户要哪层给哪层，不强行全给。
		//
		// 而**第一层没有入口**。在这之前中文这边只有第二层（字词释义）和
		// 第三层（句法）两颗按钮，于是她想知道「这几个字什么意思」时，
		// 只能从「句法」进去，先读一段句式分析。讲义里第一层是**默认层**，
		// 第三层写的是「点到即止」，第四层写的是「如需」—— 默认反了。
		//
		// 诗词一起给：一首诗最常被问的就是这一句话是什么意思。
		Shape: "prose", ID: "classical_translate", Label: "白话翻译", Lang: "zh",
		Genres: []string{genreClassical, genrePoem}, Subject: "sentence",
		Class: gateway.ClassDigest, MaxRunes: 200,
		// 「按句直译，保留原意；标点、语气尽量对应原文」+「忠于原意，不增删」
		// 都是讲义的原话。六字诀（留删补换调变）不在讲义里，所以这里只写
		// 它实际要求的那几个动作，不端出一套它没说过的口诀。
		Instruction: "把学生划出的这几个字译成白话。按句直译，语序调成今天的说法，" +
			"原文省掉的成分补出来并放在括号里，标点和语气尽量对应原文。\n" +
			"人名、地名、官名、年号、书名照抄不译。\n" +
			"不增不删：不要发挥、不要替作者多说一层意思、不要把直译换成大意概述。\n" +
			"遇到比喻、借代、用典、互文，译出它指的那件事，不要停在字面。\n" +
			"这几个字有两种通行的读法时，两种都写出来并各自说依据，不把一种说成定论；" +
			"拿不准的地方直说拿不准，不要猜。\n" +
			"只给译文和必须的括号补充，不讲句式、不讲字词来历 —— 那是另外两件工具。",
	},
	{
		// 🚨 2026-09-24：**第二层里她自己挑的那一个字**。产品负责人：
		// 「I think we need to make chinese lookup different from english.」
		//
		// 和「字词释义」的分工，同英文那边「关键单词」与「查词」的分工：
		// 前者是模型替她挑的两三个字，后者是**她自己**卡住的那一个 ——
		// 后者才是她真正不认识的。
		//
		// 🚨 它和英文的「查词」不共用输出约定。英文那份 words 契约里写着
		// 「example：一个**新造的**英文例句」——给一个文言字造一句英文例句是
		// 没有意义的。所以这件工具用 Shape "hanwords"，另一份契约：
		// 类别（通假/古今异义/活用…）、本义、在这里的意思、凭什么这么判。
		// 产物仍然是词卡（同一个 words 字段），所以正文里的荧光笔、卡片的样子、
		// 报告里的生词表都不用另写一份。
		Shape: "hanwords", ID: "classical_word", Label: "查字", Lang: "zh",
		Genres: []string{genreClassical, genrePoem}, Subject: "word",
		Class: gateway.ClassDigest,
		Instruction: "学生点了一个字或一个词（见下面【要讲解的这一个词】），只讲这一个，给**一张**卡。" +
			"讲的是它**在这一句里**的意思；它是一个词的一部分时，term 写整个词（仍然要逐字出现在段落里）。",
	},
	{
		// 🚨 2026-09-23，诗词专用。照她给的那份诗歌鉴赏讲义
		// （shige/SKILL.md 的「核心意象」表：意象 | 象征 | 情感）。
		//
		// 讲义里那条输出原则写进了说明：**「为什么好」比「用了什么」重要**。
		Shape: "prose", ID: "poem_images", Label: "意象", Lang: "zh",
		Genres: []string{genrePoem}, MaxRunes: 300,
		Instruction: "挑这几句里两三处具体的景物或器物，一处一行：先写它是什么，" +
			"再写它在这一首里带着什么情绪。常见的意象有固定的用法（月与思乡、柳与离别、梧桐与孤寂），" +
			"可以说出来，但要回到这一首的上下文核对，对不上就按这一首的说。" +
			"说出手法的名字之后要接着说它在这里做成了什么 —— 只说「这里用了借景抒情」等于没说。",
	},
	{
		Shape: "prose", ID: "rhetoric", Label: "成语修辞", Lang: "zh",
		Instruction: "指出这一段用到的成语、俗语和修辞手法（比喻、排比、反问、对比……），每个都说清楚它在这里起了什么效果。未发现相关表达时如实说明。",
	},
	{
		// 诗词上撤掉：一首绝句里没有事例、数据、引用可以点。见 NotGenres。
		Shape: "prose", ID: "examples", Label: "案例", Lang: "zh",
		NotGenres:   []string{genrePoem},
		Instruction: "这一段举了哪些具体的事例、数据或引用？每个说清楚它是用来支持什么的。没有具体事例就直说这一段是在讲道理，不是在举例。",
	},
	{
		// 🚨 2026-09-18 同事（《敬业与乐业》第 8 段）：「这个结构解析工具基本没解析，
		// 按理来说现在应该在段落内，对句子划分结构，但是解析工具只是整体概述了一下」。
		// 原来的说明只问「这一段在整篇里在干什么」，模型照做了 —— 给的就是一句概述。
		// 改成先拆段内的层次（哪几句是一层、这一层在干什么、层与层怎么接），
		// 最后才用一句话说这一段在全文的位置。
		// 诗词上撤掉：它拆的是段内层次，而诗的结构是整首的起承转合，
		// 那一层落在「整篇」尺度上（见 readingArticleTools 的 poem_shape）。
		Shape: "prose", ID: "structure", Label: "结构解析", Lang: "zh", MaxRunes: 360,
		NotGenres: []string{genrePoem},
		// 2026-09-18 线上第一次跑：一句一条列了十一条、远超字数上限。层次是几句合成一层，
		// 所以明说 2 到 5 层、不要一句一层。
		Instruction: "把这一段**按句子拆成 2 到 5 个层次**（一层通常包括几句，不要一句一层），写成编号列表，一层一行：" +
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
		Instruction: "针对这一段，给学生 2 到 4 个能帮学生想下去的问题。必须是问题，每条以问号结尾，一条一个问题。要具体到这一段的内容，不要问「这段讲了什么」这种空问题。**不要在问题里把答案说出来。**",
	},
	{
		Shape: "imitate", ID: "imitate", Label: "仿写", Lang: "",
		Instruction: "先用一句话说清楚这一段**在写法上做了什么**（比如「先给一个日常场景，再解释背后的原理」），然后给 2 到 3 个学生可以用同一个写法去写的、和原文无关的话题。",
	},
}

// readingBlockToolsFor 挑**这一篇**用得上的段落工具。
//
// 🚨 体裁认不出来（空串、老数据）时，带体裁的那几件工具**不给** ——
// 这个方向是故意的，而且和别处的「空 = 不限」相反：
//
//	一篇认不出体裁的中文文章上摆着「通假字、古今异义」，是一个按下去
//	讲不出东西的按钮，而没有意义的按钮会让她不信任整条工具条。
//	少一件工具的代价，比多一件假的小得多。
//
// 要「库里一共有哪几件」（报告那一侧按 id 查标签）用 readingBlockToolsAll。
func readingBlockToolsFor(lang string, genre string) []readingBlockTool {
	want := lang
	if want != "en" {
		want = "zh"
	}
	genre = validateGenre(genre)
	out := make([]readingBlockTool, 0, len(readingBlockTools)+len(readingWritingTools))
	for _, t := range readingBlockTools {
		if t.Lang != want {
			continue
		}
		if len(t.Genres) > 0 && !containsString(t.Genres, genre) {
			continue
		}
		// 黑名单只在体裁真的判出来了的时候才撤 —— 见 NotGenres 的第 2 条。
		if genre != "" && containsString(t.NotGenres, genre) {
			continue
		}
		out = append(out, t)
	}
	// The language-independent pair always comes last: understand first, then
	// make the move yourself.
	out = append(out, readingWritingTools...)
	return out
}

// readingBlockToolsAll —— 库里的每一件工具，不按语言也不按体裁挑。
//
// 报告那一侧用它：它数的是**她真的用过**哪几件，按 id 查回标签。
// 按体裁挑会让一篇文言文的报告漏掉「字词释义」那一行 —— 她明明用过。
func readingBlockToolsAll() []readingBlockTool {
	out := make([]readingBlockTool, 0,
		len(readingBlockTools)+len(readingWritingTools)+len(readingArticleTools))
	out = append(out, readingBlockTools...)
	out = append(out, readingWritingTools...)
	// 整篇那几件也算进来：报告那一侧按 id 查回标签，漏掉它们会让一份用过
	// 「论证图」的报告少掉那一行 —— 她明明用过。
	out = append(out, readingArticleTools...)
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
	for _, t := range readingArticleTools {
		if t.ID == id {
			return t, true
		}
	}
	return readingBlockTool{}, false
}
