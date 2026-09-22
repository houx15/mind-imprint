package api

// reading_genre.go — 按文章体裁换读法、换板、换带读的说法。
//
// # 来源
//
// 同事 2026-09-17 的阅读模块 PRD（「后续优化PRD」）：
//
//	阅读模块先覆盖议论文、说明文、记叙文、新闻报道。AI 根据文章类型、段落作用
//	和学生的阅读目的，选择对应的读法、交互卡和帮助方式。
//
// 产品负责人同一天的要求：「don't bother the current experience of argument
// papers」「follow the current big stages and process」。
//
// # 所以边界是这样划的
//
//   - 体裁在排读法那一次就判了（reading_plan.go 的 genre，闭表见
//     reading_outline.go）。这里不另起一次分类调用。
//   - 读法跟着体裁走（pickRoutineForGenre）。
//   - **议论文一个字都不动**：板还是那两套，system prompt 还是那一份，
//     下面这一节带读说明只在另外三种体裁上才加进 prompt。
//     体裁认不出来（老数据、模型漏填）也按议论文办 —— 那就是今天的样子。
//   - 另外三种体裁各有一块自己的标注板（格子名是闭表，由服务端填，
//     和议论文那两套同一条纪律），报道和记叙多一块排序板（order_events）。

import (
	"sort"
	"strings"
)

// coachGenreBoard 是一种体裁的标注板：格子和标准题目。
type coachGenreBoard struct {
	Bins []string
	// Prompt 是这块板的标准题目。模型写的题目点了别的格子名、或者兜底摆板时，
	// 用它。书面、一句话、不写怎么拖 —— 和 coachLabelBoardPrompt 同一条规矩。
	Prompt string
	// Named 是 prompt 里给模型看的那一句：这块板在这篇上是什么格子。
	Named string
}

// coachGenreBoards —— 议论文之外的三块板。议论文不在表里：它走 coachBinSetFor
// 的那两套，一行都没变。
//
// 格子名取自 PRD 的原话，而且都是中学语文课上真有的词（事实/引述、说明对象、
// 动作描写……）—— 她来这儿就是要学这套词（AGENTS.md 界面文案第 6 条）。
var coachGenreBoards = map[string]coachGenreBoard{
	// PRD：区分事实、引述和解释；辨认「谁说的、依据是什么」。
	genreReport: {
		Bins:   []string{"事实", "引述", "解释"},
		Prompt: "分析下列句子，判断它们各自是报道中的哪一类信息。",
		// 🚨 引述包括转述：「Some residents blamed the council」没有引号，
		// 仍然是某一方的说法，不是记者核实的事实（2026-09-18 线上走查）。
		Named: "「事实 / 引述 / 解释」——记者核实的事、某一方的说法（直接引用或转述都算）、对事件的解释或推断",
	},
	// PRD：文章内容 = 说明对象、主要的说明部分；搭出结构或过程。
	genreExplain: {
		Bins:   []string{"说明对象", "原理与过程", "例子与数据"},
		Prompt: "分析下列句子，判断它们各自在说明中承担什么作用。",
		Named:  "「说明对象 / 原理与过程 / 例子与数据」——要讲清楚的那样东西、它怎么运作、拿来说明它的例子或数字",
	},
	// PRD：人物变化卡记录行动、情绪及对应原文；回看叙述与细节。
	genreNarrative: {
		Bins:   []string{"动作描写", "语言描写", "心理描写", "环境描写"},
		Prompt: "分析下列句子，判断它们各自属于哪一种描写。",
		Named:  "「动作描写 / 语言描写 / 心理描写 / 环境描写」——人物做了什么、说了什么、想了什么，以及周围的环境",
	},
}

// genreBoardFor —— 这篇文章该用哪一块标注板。议论文和认不出来的体裁返回 false：
// 那两种走原来的 coachBinSetFor。
func genreBoardFor(genre string) (coachGenreBoard, bool) {
	b, ok := coachGenreBoards[validateGenre(genre)]
	return b, ok
}

// fitBoardToGenre 把一张标注板的格子换成这篇文章体裁的那一套。
//
// 🚨 在校验**之后**调：validateCoachCardWhy 按议论文填了格子（它不知道体裁），
// 这里再按体裁覆盖。议论文和认不出来的体裁原样返回 —— 那条路一个字节都不变。
//
// 题目也要跟着换：题目里点了一个不在这套格子里的名字（「判断哪句是证据」），
// 或者是议论文那句标准题目，都换成这块板自己的标准题目 —— 格子被换掉了，
// 题目还在说另一套，她只能猜哪一个算数（同一个毛病在议论文那一侧出过一次，
// 见 labelPromptInventsBins）。
func fitBoardToGenre(c *coachCard, genre string) *coachCard {
	if c == nil || c.Type != coachCardLabelRoles {
		return c
	}
	board, ok := genreBoardFor(genre)
	if !ok {
		return c
	}
	out := *c
	out.Labels = board.Bins
	out.BinSet = ""
	if out.Prompt == coachLabelBoardPrompt || promptNamesOtherBins(out.Prompt, board.Bins) {
		out.Prompt = board.Prompt
	}
	return &out
}

// promptNamesOtherBins —— 题目里是不是点了一个不在这套格子里的格子名。
func promptNamesOtherBins(prompt string, bins []string) bool {
	for _, l := range allBoardLabels() {
		if containsString(bins, l) || strings.Contains(strings.Join(bins, "|"), l) {
			continue
		}
		if strings.Contains(prompt, l) {
			return true
		}
	}
	return false
}

func containsString(xs []string, s string) bool {
	for _, x := range xs {
		if x == s {
			return true
		}
	}
	return false
}

// genreBoardBins —— 所有体裁板上的格子名，给 isRoleLabel / allRoleLabels 用：
// 读回转写里她摆过的板时要认得出来。
func genreBoardBins() []string {
	keys := make([]string, 0, len(coachGenreBoards))
	for k := range coachGenreBoards {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	out := make([]string, 0, 10)
	for _, k := range keys {
		out = append(out, coachGenreBoards[k].Bins...)
	}
	return out
}

// ---------------------------------------------------------------------------
// 排序板（order_events）
// ---------------------------------------------------------------------------

const (
	// 三到五件事。两件谈不上排序；六件以上在手机上要来回翻。
	coachOrderMinOptions = 3
	coachOrderMaxOptions = 5
	// coachOrderPrompt —— 兜底和改题目时用的标准题目。
	coachOrderPrompt = "请按事情发生的先后，排列下列事件。"
)

// genreHasOrderBoard —— 这种体裁上摆不摆排序板。
//
// 只有报道和记叙：它们的结构就是事件顺序（PRD：「搭建事件时间线」「事件卡
// 排序」）。议论文上不给 —— 那条路一个字节都不变；说明文的结构是信息层次，
// 不是先后。
func genreHasOrderBoard(genre string) bool {
	switch validateGenre(genre) {
	case genreReport, genreNarrative:
		return true
	}
	return false
}

// orderByArticle 把排序板上的几句按**原文顺序**摆。
//
// 🚨 模型给的顺序多半就是它心里的发生顺序 —— 原样摆出来等于把答案交给她。
// 原文顺序是讲述顺序，而「讲述顺序和发生顺序哪里不一样」正是这块板要她看出来
// 的东西（PRD：「切换发生顺序／讲述顺序」）。每张卡片上还带着「第几段」，
// 她一眼看得到讲述顺序。
func orderByArticle(opts []coachCardOption, blocks []Block) []coachCardOption {
	type pos struct{ block, at int }
	where := func(o coachCardOption) pos {
		for i, b := range blocks {
			if b.ID == o.BlockID {
				return pos{i, strings.Index(b.Text, o.Quote)}
			}
		}
		return pos{len(blocks), 0}
	}
	out := append([]coachCardOption(nil), opts...)
	sort.SliceStable(out, func(i, j int) bool {
		a, b := where(out[i]), where(out[j])
		if a.block != b.block {
			return a.block < b.block
		}
		return a.at < b.at
	})
	return out
}

// ---------------------------------------------------------------------------
// 带读说明
// ---------------------------------------------------------------------------

// genreCoachGuide —— 每种体裁在 prompt 里多出来的那一节。议论文没有这一节。
//
// 内容来自 PRD 的「四类文章的工作流与交互」表：阅读流程、具体交互、AI 负责的
// 部分，三列各取要点。
var genreCoachGuide = map[string]string{
	genreReport: `这是一篇**新闻报道**：记者在讲发生了什么、各方怎么说，自己不表态。
读法：了解事件 → 梳理时间与参与方 → 区分事实、引述和解释 → 比较来源与说法 → 总结已知与待了解的信息。
你负责的部分：
- 补充理解这件事所需的背景（地名、机构、事情的来龙去脉），一两句就够。
- 帮她辨认**谁说的、依据是什么**：一句话是记者核实的事实，还是某一方的说法，还是对事件的解释。
- 引导她观察报道**选了哪些信息、呈现了哪些视角**，有没有哪一方没被问到。
- 标题和正文要对得上：标题说的那件事，正文中哪些信息与标题对应。
- 系统说明里「作者的观点」「论证」那些说法在这篇上不适用：这里没有作者要她接受的看法。`,
	genreExplain: `这是一篇**说明文**：作者在讲清楚一样东西是什么、怎么运作。
读法：明确说明对象 → 理清概念 → 搭出结构或过程 → 解释关键关系 → 换情境应用。
你负责的部分：
- 她卡在术语上，**直接解释**这个术语在这里的意思，不用梯子。
- 理不清关系时，先示范**一个**关系（「A 让 B 变热，所以……」），再请她说下一个。
- 帮她补齐理解里缺的那一环：她说出了原因和结果，中间那一步没说，就指出那一步在第几段。
- 结构按信息层次说：总—分、并列、先后步骤、因果链。
- 系统说明里「作者的观点」「论证」那些说法在这篇上换成「说明对象」「原理」。`,
	genreNarrative: `这是一篇**记叙文**：一件事按时间讲下来，或者一个人的故事。
读法：读懂事件 → 找到转折 → 理解人物 → 回看叙述与细节 → 形成自己的解释。
你负责的部分：
- 帮她梳理时间、人物与事件：谁、在哪儿、先发生什么后发生什么。
- 分清**发生顺序**和**讲述顺序**：作者先讲了哪件、后讲了哪件，为什么这样安排（倒叙、插叙）。
- 结合人物前后的行为**追问动机**：他为什么在这里这样做，原文哪几个字能看出来。
- 连起伏笔与照应：前面一处细节，后面在哪儿有了回应。
- 她给出一种解释时，对比这种解释和原文细节的关系；有多种解释符合原文时，说明这些解释各自的依据。
- 系统说明里「作者的观点」「证据」那些说法在这篇上不适用。`,
}

// buildGenreCoachSection 是 prompt 里「这篇的体裁」那一节。议论文、认不出来的
// 体裁返回空串 —— 那两种的 prompt 和今天一字不差。
func buildGenreCoachSection(genre string) string {
	genre = validateGenre(genre)
	guide, ok := genreCoachGuide[genre]
	if !ok {
		return ""
	}
	var b strings.Builder
	b.WriteString("\n【这篇的体裁，以及在这篇上怎么带】\n")
	b.WriteString(guide + "\n")
	if board, ok := genreBoardFor(genre); ok {
		b.WriteString("\n🚨 **这篇的标注板换了一套格子**：" + board.Named + "。" +
			"系统说明里「论点 / 论据 / 论证」「论点 / 驳斥观点 / 论据 / 论证」那两套在这篇上不用，" +
			"binSet 也不用给 —— 格子由系统按这篇的体裁填。说板的时候就用这几个格子名，" +
			"题目照这个样子写：「" + board.Prompt + "」\n")
	}
	if genreHasOrderBoard(genre) {
		b.WriteString("\n**这篇还能用第六种卡片：排序板。**\n" +
			`{"type":"order_events","prompt":"一句话的问题","options":[{"blockId":"b3","quote":"文章里的原话"}]}` + "\n" +
			"几件事摆在板上，她按**发生的先后**排好。options 给 3 到 5 条，每一条都是文章里" +
			"写着一件事的那一句，**逐字抄**（系统会核对）；可以来自同一段。" +
			"板上按原文顺序摆（那就是讲述顺序），你给的顺序不会被用上，所以不用费心打乱。\n" +
			"什么时候用：清单走到「排出事件顺序 / 排出事件时间线」那一步的时候，用它把这一步交给她。" +
			"她排完之后，先接住她的排法，再说一句**讲述顺序和发生顺序哪里不一样、作者为什么这样安排**，然后推进。" +
			"🚨 给板的那一轮不要把正确的先后说出来。\n")
	}
	return b.String()
}

// buildOrderBoard —— 「排出事件顺序」那一步模型没给板时，服务端摆一块。
//
// 和 buildLabelBoard 同一个理由：这一步的全部内容就是那块板，而「有没有板」
// 不需要模型的判断力。句子取自**分散在全篇**的几段 —— 一篇报道的时间线散在
// 各个部分里，只从一段里取，排出来的只是一段话的顺序。
//
// 有切法就每个部分取一句（部分的第一段里第一句够长的），没有就在全篇里等距取。
// 挑不出三句就返回 nil：宁可没有板，也不要一块两句的板。
func buildOrderBoard(blocks []Block, parts []readingPart) *coachCard {
	if len(blocks) == 0 {
		return nil
	}
	idx := make(map[string]int, len(blocks))
	for i, b := range blocks {
		idx[b.ID] = i
	}
	var picks []int
	for _, p := range parts {
		if i, ok := idx[p.From]; ok {
			picks = append(picks, i)
		}
	}
	if len(picks) < coachOrderMinOptions {
		picks = picks[:0]
		n := coachOrderMaxOptions - 1
		if len(blocks) < n {
			n = len(blocks)
		}
		for k := 0; k < n; k++ {
			picks = append(picks, k*len(blocks)/n)
		}
	}
	out := make([]coachCardOption, 0, coachOrderMaxOptions)
	seen := map[string]bool{}
	for _, i := range picks {
		for _, sent := range splitSentences(blocks[i].Text) {
			n := len([]rune(sent))
			if n < boardSentenceMinRunes || n > boardSentenceMaxRunes || seen[sent] {
				continue
			}
			seen[sent] = true
			out = append(out, coachCardOption{BlockID: blocks[i].ID, Quote: sent})
			break
		}
		if len(out) == coachOrderMaxOptions-1 {
			break
		}
	}
	if len(out) < coachOrderMinOptions {
		return nil
	}
	return &coachCard{Type: coachCardOrderEvents, Prompt: coachOrderPrompt, Options: out}
}
