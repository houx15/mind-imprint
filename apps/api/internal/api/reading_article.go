package api

import (
	"encoding/json"
	"strings"

	"mindimprint/api/internal/gateway"
)

// 整篇那一层的工具 —— 「拼出全文结构」。
//
// # 为什么段落工具办不到这件事
//
// 产品负责人 2026-09-24：「for poems or 记叙文, we also need a whole-level view
// of analysis (actually, all need that, the article structure, etc.」
//
// 这不是把段落工具的说明改长一点能办到的。有四样东西**结构上**不在段落这个
// 尺度上：
//
//   - 详略安排：要比较各段的长短，一个段落自己比不出来。
//   - 首尾呼应：要同时拿着第一段和最后一段，还要判断那个词的意思变没变。
//   - 叙事的「转」：从一种心态到另一种，是一条跨段的链子。
//   - 诗的起承转合：《江雪》被切成了两段，没有任何一件段落工具看得见整首诗。
//
// # 每一种体裁交出什么，是她自己定过的
//
// `docs/reference/writing-teaching/reading-suggestion.md` 第 4 节逐字写着
// 阅读结束要保留的那份成果：
//
//	议论文为论证图，说明文为知识结构／流程图，记叙文为事件／人物变化图，
//	新闻报道为事件与来源对照
//
// 所以这里一种体裁一件工具，标签和里面的分类词都照那一份走，不另起一套。
// 同一份文件第 1 节还写着整条流程：
//
//	AI 通读全文、划分内容单元 → 学生分段通读并概括 → 拼出全文结构 →
//	按文章类型精读 → 整理阅读成果
//
// 这一件工具是其中的「拼出全文结构」那一步。
//
// # 产物的形状
//
// 一份结构，不是一段散文（同 words / grammar 的理由）：每一块都带一句
// **逐字来自原文**的引文，服务端拿它回全文里核对，核不上的那一块丢掉。
// 那次核对是这件工具唯一可验的判据 —— 否则「这篇文章分成四块」这种话
// 一个字都验不了，而模型最爱在整篇这一层上凭印象说话。
var readingArticleTools = []readingBlockTool{
	{
		Shape: "article", ID: "argument_map", Label: "论证图", Lang: "zh",
		Scope: "article", Genres: []string{genreArgument},
		Class: gateway.ClassCompose, MaxRunes: 520,
		// reading-suggestion.md 的议论文那一行要的是**两轴**：
		// 「文章内容：论点、论据（包含正反）、分析段；文章组织架构：递进、对照等」。
		// 原来的读法只有内容那一轴（论点、论据、论证），组织那一轴整个不在。
		Instruction: "把全文拆成 3 到 6 块，做成一张论证图。\n" +
			"每一块的 label 只能取：中心论点 / 分论点 / 正面论据 / 反面论据 / 分析段 / 让步 / 结论。\n" +
			"spine 写这篇文章的组织架构，只能取：递进 / 对照 / 总分 / 分总 / 总分总 / 并列，" +
			"并在后面用一句话说凭什么这么判（哪两块之间是这种关系）。\n" +
			"takeaway 写这篇文章要证明的那一句话。",
	},
	{
		Shape: "article", ID: "explain_map", Label: "知识结构", Lang: "zh",
		Scope: "article", Genres: []string{genreExplain},
		Class: gateway.ClassCompose, MaxRunes: 520,
		Instruction: "把全文拆成 3 到 6 块，做成一张知识结构图。\n" +
			"每一块的 label 只能取：说明对象 / 定义 / 分类 / 特征 / 过程 / 原因 / 举例 / 比较 / 应用。\n" +
			"spine 写这篇文章用的事物组织方法，只能取：总分 / 递进 / 并列 / 时间顺序 / 空间顺序 / 逻辑顺序，" +
			"并用一句话说凭什么这么判。\n" +
			"takeaway 写这篇文章讲清楚的那一件事。",
	},
	{
		Shape: "article", ID: "report_map", Label: "事件与来源", Lang: "zh",
		Scope: "article", Genres: []string{genreReport},
		Class: gateway.ClassCompose, MaxRunes: 520,
		// 「帮助辨认『谁说的、依据是什么』」——reading-suggestion.md 新闻报道那一行。
		Instruction: "把全文拆成 3 到 6 块，做成一张事件与来源对照。\n" +
			"每一块的 label 只能取：事件 / 时间 / 参与方 / 事实 / 引述 / 解释 / 背景 / 待了解。\n" +
			"是引述的那一块，note 里要写清楚**是谁说的**；是解释的那一块，写清楚**依据是什么**。\n" +
			"spine 写这篇报道是按什么组织的，只能取：倒金字塔 / 时间顺序 / 并列来源 / 因果。\n" +
			"takeaway 写读完之后已知的那件事，一句话。",
	},
	{
		// 记叙文与散文共用。散文那一侧的依据薄：来源库里 15 个题材文件没有
		// 「散文」，评分标准只有记叙文/说明文/议论文，而拿记叙文的「一波三折」
		// 去读《春》会判成「没有结构」。所以 spine 的闭表里给了散文用得上的
		// 那几项（以物为线索、情感变化），不逼它套事件结构。
		Shape: "article", ID: "narrative_map", Label: "事件与人物", Lang: "zh",
		Scope: "article", Genres: []string{genreNarrative, genreProse},
		Class: gateway.ClassCompose, MaxRunes: 560,
		// 顺序照 reading-suggestion.md 记叙文那一行：
		// 「读懂事件 → 找到转折 → 理解人物 → 回看叙述与细节 → 形成自己的解释」。
		// 「找到转折」排在第二位 —— 而这正是段落读法结构上到不了的那一样。
		Instruction: "把全文拆成 3 到 6 块，做成一张事件与人物变化图。\n" +
			"每一块的 label 只能取：起因 / 经过 / 转折 / 结果 / 感悟 / 铺垫 / 插叙 / 环境 / 首尾呼应。\n" +
			"**必须有一块是「转折」**：作者对同一个人或同一件事的态度从什么变成什么，" +
			"转发生在哪一句。判据是那一句要给出一个看得见的瞬间（一个具体画面），" +
			"只写「我突然明白了」的，note 里如实说这次转是空的。\n" +
			"spine 写全文是按什么串起来的，只能取：时间顺序 / 倒叙 / 插叙 / 一波三折 / " +
			"双线并进 / 以物为线索 / 空间顺序 / 情感变化，并用一句话说凭什么这么判。\n" +
			"takeaway 写这篇文章最后让人明白的那一件事，一句话。",
	},
	{
		// 诗词。这一件是「结构解析」在诗上的替身 —— 后者拆的是段内层次，
		// 而一首绝句的结构是整首的起承转合（见 readingBlockTools 里 structure
		// 的 NotGenres）。
		Shape: "article", ID: "poem_shape", Label: "起承转合", Lang: "zh",
		Scope: "article", Genres: []string{genrePoem},
		Class: gateway.ClassCompose, MaxRunes: 520,
		Instruction: "把整首诗按位置拆块，做成一张起承转合图。\n" +
			// 🚨 「一句一块，每一句都要出现」是线上走查逼出来的：《江雪》
			// 第一次回来只有三块，「万径人踪灭」整句没了。诗只有四句，
			// 丢一句就是丢掉四分之一首。
			"**一句诗一块，每一句都必须出现，不许把两句并成一块，也不许跳过任何一句。**\n" +
			"绝句四句就是四块，label 依次取：起 / 承 / 转 / 合。" +
			"律诗按联拆，label 取：首联 / 颔联 / 颈联 / 尾联。" +
			"词按上下阕拆，label 取：上阕 / 下阕。古体诗按情感段落拆，label 取：起 / 承 / 转 / 合。\n" +
			"每一块的 note 写这一句在这个位置上做成了什么（破题 / 铺叙 / 转折 / 收束），" +
			"不要只说它写了什么景。\n" +
			"**「转」在哪一句要说出判据**：看哪一句换了主体、换了时间、换了视角，或者由景入情。\n" +
			"spine 写这一首的情景关系，只能取：触景生情 / 缘情造景 / 以景结情 / 借景抒情 / " +
			"融情于景 / 情景交融 / 以乐景写哀情 / 以哀景写乐情，并用一句话说凭什么这么判。\n" +
			"takeaway 写这一首的题材和它定下的情感方向，一句话。",
	},
	{
		Shape: "article", ID: "classical_shape", Label: "全篇章法", Lang: "zh",
		Scope: "article", Genres: []string{genreClassical},
		Class: gateway.ClassCompose, MaxRunes: 520,
		Instruction: "把全篇拆成 3 到 6 块，做成一张章法图。\n" +
			"每一块的 label 只能取：叙事 / 描写 / 议论 / 抒情 / 对话 / 背景 / 评语。\n" +
			"文言文最常见的形状是先叙后议，所以**要指出「议」从哪一句开始**，" +
			"判据是那一句不再讲发生了什么，而开始给评价或讲道理。全篇都是叙事就直说没有议论那一块。\n" +
			"spine 写全篇的章法，只能取：先叙后议 / 总分 / 分总 / 时间顺序 / 对照 / 层递。\n" +
			"takeaway 写这一篇要说明的那个道理或那个人的那一面，一句话。",
	},
	{
		// 英文那一侧先只给一件通用的。英文议论文的体验是产品负责人明确
		// 划出来不要动的（「don't bother the current experience of argument
		// papers」），所以这里不改它的读法，只多一件整篇的图。
		Shape: "article", ID: "article_shape_en", Label: "Article structure", Lang: "en",
		Scope: "article", Class: gateway.ClassCompose, MaxRunes: 520,
		Instruction: "把全文拆成 3 到 6 块，做成一张整篇结构图。\n" +
			"每一块的 label 只能取：thesis / topic sentence / evidence / commentary / " +
			"counterargument / rebuttal / concession / conclusion。\n" +
			"spine 写这篇文章的组织方式，只能取：thesis-body-conclusion / " +
			"claim-counterargument-refutation / point-by-point comparison / block comparison / " +
			"chronological / problem-solution，并用一句话说凭什么这么判" +
			"（哪两块之间是这种关系 —— 只给名字不算）。\n" +
			"takeaway 写这篇文章要证明的那一句话。",
	},
}

// readingArticleToolsFor 挑这一篇用得上的整篇工具。
//
// 体裁认不出来时一件都不给：整篇这一层的每一件工具都带一套**这一体裁专属的
// 分类词**（论点/分论点 对 起因/经过/转折 对 起/承/转/合），拿错一套比不给更糟。
// 这和段落工具那边 Genres 白名单的默认是同一条（见 readingBlockToolsFor）。
//
// 英文那一件没有 Genres，所以它对每一篇英文文章都在 —— 英文这一侧目前只有
// 一套通用分类词，不存在拿错的问题。
func readingArticleToolsFor(lang string, genre string) []readingBlockTool {
	want := lang
	if want != "en" {
		want = "zh"
	}
	genre = validateGenre(genre)
	out := make([]readingBlockTool, 0, 2)
	for _, t := range readingArticleTools {
		if t.Lang != want {
			continue
		}
		if len(t.Genres) > 0 && !containsString(t.Genres, genre) {
			continue
		}
		out = append(out, t)
	}
	return out
}

// readingArticleBlockID 是整篇那份讲解在 reading_block_note 里占的 block_id。
//
// 用一个哨兵值而不是新开一张表：缓存、重放、失败处理、用量记账、报告那一侧
// 都已经按 (atom_id, block_id, tool, subject) 这把键写好了，整篇只是「block_id
// 不是任何一段」的那一行。
//
// 🚨 它必须是一个 SplitBlocks 永远不会产出的 id。段落 id 形如 b1、b2……
// （见 SplitBlocks），所以带 @ 的这个值不可能和任何一段撞上。正文里的荧光笔
// 按 blockId 找段落，找不到就不标 —— 整篇那一份本来也不该标在某一段上。
const readingArticleBlockID = "@article"

// readingArticleOutline 是整篇那一层的产物。
type readingArticleOutline struct {
	// Parts 是全文被拆成的那几块。
	Parts []readingArticlePart `json:"parts"`
	// Spine 是全文的组织方式（论证链 / 时间顺序 / 起承转合 …）加一句判据。
	Spine string `json:"spine"`
	// Takeaway 是一句话的主旨。
	Takeaway string `json:"takeaway"`
}

// readingArticlePart 是其中一块。
type readingArticlePart struct {
	// Label 是这一块在全篇里干什么，取自这一体裁的闭表。
	Label string `json:"label"`
	// Quote 逐字来自原文，是这一块的锚。
	//
	// 🚨 它是这件工具唯一可验的东西。整篇这一层最容易出的毛病是模型凭印象
	// 说「这篇分成四块」，而四块都指不到原文的哪里 —— 一句能回原文里查到的
	// 引文把那种话挡在外面。
	Quote string `json:"quote"`
	// Note 是这一块在全篇里起什么作用，一两句。
	Note string `json:"note"`
}

// readingArticleShapeSuffix 是 Shape "article" 的输出约定。
const readingArticleShapeSuffix = "\n\n只输出一个 JSON 对象：" +
	`{"parts":[{"label":"","quote":"","note":""}],"spine":"","takeaway":""}` + "\n\n" +
	"- label：这一块在全篇里干什么，只能从上面那张闭表里取，不要自己造词。\n" +
	// 🚨 长度不写死一个区间。第一版写的是「8 到 20 个字」，而一首五言绝句
	// 每一句正好**五个字** —— 线上拿《江雪》一走，模型为了凑够 8 个字把四句
	// 并成了三块，「万径人踪灭」整句消失，起承转合的标签跟着全部错位。
	// 一个对散文合理的下限，在诗上就是一条做不到的要求。
	"- quote：这一块**开头的那几个字**，逐字照抄原文，一个标点都不许改。" +
	"抄到能在原文里唯一认出这个位置就够了：一句诗就抄**整句**（五言五个字、七言七个字），" +
	"散文抄开头十来个字。**系统会拿它回全文里逐字核对，对不上的那一块整块丢掉。**" +
	"不要写段号、不要写「第三段」、不要自己概括 —— 要的是原文里真有的那一串字。\n" +
	"- note：这一块在全篇里起什么作用，一两句话，不超过 40 字。说的是作用，不是内容摘要。\n" +
	"- parts 按在文章里出现的先后排。\n" +
	"不要输出对象以外的任何文字或代码块标记。"

// parseArticleOutline 读整篇那份回话，并丢掉一切核对不上的块。
//
// 丢弃规则（和词卡那一套同形）：
//
//  1. label 或 quote 为空 → 丢。
//  2. **quote 在全文里找不到 → 丢。** 找不到就说明这一块指不到原文的任何位置。
//  3. 同一句引文重复 → 只留第一块。
//  4. 一块都不剩 → 整件工具算失败，绝不返回一张空图。
//
// 回话里的 spine / takeaway 原样留着：它们是判断，本来就不是原文里的字。
func parseArticleOutline(body, article string) (*readingArticleOutline, bool) {
	var got readingArticleOutline
	if err := json.Unmarshal([]byte(body), &got); err != nil {
		return nil, false
	}
	out := readingArticleOutline{
		Spine:    strings.TrimSpace(got.Spine),
		Takeaway: strings.TrimSpace(got.Takeaway),
		Parts:    make([]readingArticlePart, 0, len(got.Parts)),
	}
	seen := map[string]bool{}
	for _, p := range got.Parts {
		label := strings.TrimSpace(p.Label)
		quote := strings.TrimSpace(p.Quote)
		note := strings.TrimSpace(p.Note)
		if label == "" || quote == "" {
			continue
		}
		i := strings.Index(article, quote)
		if i < 0 {
			continue
		}
		if seen[quote] {
			continue
		}
		seen[quote] = true
		out.Parts = append(out.Parts, readingArticlePart{Label: label, Quote: quote, Note: note})
		if len(out.Parts) == readingArticlePartsMax {
			break
		}
	}
	if len(out.Parts) == 0 {
		return nil, false
	}
	return &out, true
}

// readingArticlePartsMax 是一张图最多几块。
//
// 八块：提示词要的是 3–6 块，多出来的那些十有八九是模型在按段落数凑数，
// 而一张按段落一块的图就是段落列表本身，读它不如读文章。
const readingArticlePartsMax = 8

// articleOutlineAsProse 是这份图的纯文字形态，存进 body。
//
// 理由同词卡：一份只有 JSON 的记录，在任何一个不带解析器的地方
// （报告、教师端）就是一段乱码。
func articleOutlineAsProse(o *readingArticleOutline) string {
	var b strings.Builder
	if o.Spine != "" {
		b.WriteString("组织方式：" + o.Spine + "\n\n")
	}
	for _, p := range o.Parts {
		b.WriteString("【" + p.Label + "】「" + p.Quote + "」")
		if p.Note != "" {
			b.WriteString(" —— " + p.Note)
		}
		b.WriteString("\n")
	}
	if o.Takeaway != "" {
		b.WriteString("\n主旨：" + o.Takeaway)
	}
	return strings.TrimSpace(b.String())
}

// buildReadingArticlePrompt 给整篇那一层搭上下文。
//
// 每一段都带上段号和**字数**：字数不是装饰，「详略安排」这一维就是靠比各段
// 长短得出来的（来源库把它列为初二的重点考察项），而模型自己数不准。
// 服务端数得出来的事实就别让模型数 —— 同 writing 那边「还差多少字」的教训。
func buildReadingArticlePrompt(title string, blocks []Block) string {
	var b strings.Builder
	if t := strings.TrimSpace(title); t != "" {
		b.WriteString("文章标题：" + t + "\n")
	}
	b.WriteString("\n【全文】\n")
	for i, blk := range blocks {
		text := strings.TrimSpace(blk.Text)
		if text == "" {
			continue
		}
		b.WriteString("第" + itoa(i+1) + "段（" + itoa(len([]rune(text))) + "字）：" + text + "\n")
	}
	return b.String()
}
