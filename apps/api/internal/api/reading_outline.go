package api

// reading_outline.go — 导读：一句话、结构、每一段的承重。
//
// # 为什么有这个文件
//
// 印记 开场那一轮原来的活儿是「介绍一下你排的读法，然后领她进第一步」，同时
// 又被要求「不要说出这篇文章的结论、主旨、答案」。这是一道自相矛盾的题，真模型
// 给出的答案是把文章讲了一遍：
//
//	我们先一起把整篇文章通读一遍。这篇文章讲的是冲突里援助组织在忙什么——
//	以色列和哈马斯在打仗，援助组织得想办法把物资送进去、把人救出来。……
//	读完告诉我一声。
//
// 一句话里同时违反了三条已经写在 prompt 里的规矩（讲了内容、替她减压、
// 以「读完告诉我一声」收尾）。修法不是把规矩再写重一遍 —— 是把这一轮的活儿
// 拿走：她一进来该看见的三样东西由**确定性的渲染**摆出来，模型那一轮只剩
// 递第一张卡片。见 docs/2026-09-10-reading-guidance-redesign.md。
//
// # 三样东西
//
//	oneLine  这篇在**问**什么。不是它的结论 —— 结论说出来，后面每一步都是走过场。
//	shape    它怎么组织的：问题 → 证据 → 让步 → 结论。
//	load     每一段的承重：核心 / 支撑 / 过渡。
//
// 承重这一层来自 ljg-read 的段落分类（它叫骨/肌/筋）。它是整份调研里唯一真的
// 回答了「该在哪儿停下来」的启发式 —— 按论证承重，不按生词多少。核心段停下来
// 问一轮，支撑段连着读过去，过渡段一句带过。
//
// # 能验的都在代码里
//
// [[prompt-output-must-be-verifiable-2026-09-03]]：prompt 里的「必须」如果代码里
// 验不了，它就只是一句期望。这里能验的有三条，`validateOutline` 全做了：
//
//	承重只能取三个值            越界的那一段退回「支撑」
//	核心段不超过全文的一半      超了整份 outline 作废（全是核心 = 没有核心）
//	一段核心都没有              整份作废（那份分类没在分类）
//	一句话必须是中文的          一个汉字都没有 → 整份作废
//	两个字段都有字数上限        超了截断
//
// 第二条是真正有意义的那条。模型很容易把每一段都标成核心 —— 那份 outline 在
// 屏幕上仍然长得像一份导读，但它一个字的信息都没有，而带读的节奏会退回「每段
// 都停」，也就是没有节奏。**它 2026-09-17 从三分之一放宽到一半，有实测撑着**：
// 见 coreShareCap。
//
// 🚨 整份作废的代价 2026-09-17 起翻了一倍：切法（parts）和这份导读同生共死，
// 而切法现在是通读那一步的台阶（reading_plan.go 的 readingPartSteps）。
// 再往这里加「整份作废」的理由之前，先想清楚这件事。
//
// 第四条是线上第一次跑就撞上的：文章是英文的，模型顺着文章的语言把导读也写成了
// 英文（「outbreak → blockade → aid scramble → war」），摆在中文界面上对她等于
// 不存在。

import (
	"encoding/json"
	"strings"
)

// 承重的三个取值。闭表，和学科表同一个理由：模型能编出第四种，而第四种在
// 界面上没有颜色、在带读规矩里没有对应的节奏。
const (
	loadCore    = "core"    // 核心段：承载主张
	loadSupport = "support" // 支撑段：证据、例子、数据
	loadBridge  = "bridge"  // 过渡段：连接
)

// loadLabels 是给学生看的名字。
//
// 🚨 不叫骨/肌/筋。那是比喻，而 ui-copy-style 第 10 条不让在界面上用比喻替代
// 本来就有的说法。核心/支撑/过渡是说明文，而且更准。
var loadLabels = map[string]string{
	loadCore:    "核心",
	loadSupport: "支撑",
	loadBridge:  "过渡",
}

// readingPart 是这篇文章的一个部分：从第几段到第几段，它在干什么。
//
// # 为什么要把文章切成部分（2026-09-16）
//
// 产品负责人走查之后的原话：
//
//	> currently, the 通读部分 is too general. and one student, if they haven't
//	> read the article before, they would feel that ai's guidance is not easy
//	> to understand. maybe we should let ai give more scaffolding for student
//	> to read the whole article, like: general structure guidance, 导读 …
//	> then ask students to read part by part.
//
// 「通读全文」对一个没读过这篇的学生来说不是一个动作，是一整件事。切成三到五
// 个部分之后它才有台阶：一次读一部分，一部分问一句。带读那一步据此一段一段
// 地走（见 reading_coach.go 的「通读」那一节）。
type readingPart struct {
	// Title 是这一部分的名字，中文，短。
	Title string `json:"title"`
	// From / To 是这一部分的头尾段 id，闭区间。
	From string `json:"from"`
	To   string `json:"to"`
	// Does 是这一部分**在干什么**（提出问题 / 给证据 / 让步 / 收束），
	// 不是它讲了什么内容 —— 讲了什么要她自己去读。
	Does string `json:"does"`
}

// readingOutline 就是落进 reading_source.outline 的那份 jsonb。
type readingOutline struct {
	OneLine string `json:"oneLine"`
	// Gist 是这篇的**中心思想**：作者到底主张什么。
	//
	// 🚨 这一项 2026-09-16 才加，而且它**推翻了 OneLine 旁边那条注释的一半**。
	// 原来整份导读刻意不说结论（「结论说出来，后面每一步都是走过场」）。
	// 产品负责人走查之后判的是另一头：一个没读过这篇的学生，面对一个只有
	// 问题、没有答案的导读，连印记在说什么都跟不上。
	//
	// 两者并存的办法是把它们分工：OneLine 说这篇在**问**什么（她带着这个问题
	// 去读），Gist 说作者**答**了什么（她拿它当地图对照）。她要做的事没有被
	// 拿走 —— 一篇文章的价值不在于那个结论，而在于它凭什么这么说，而「凭什么」
	// 每一步都还在等她。雅思学术阅读的第一项训练目标「抓住主旨」也仍然在练：
	// 每一部分的主旨还是要她自己说（见 readingPart）。
	//
	// 允许为空：老数据没有这一项，界面据此整行不显示。
	Gist string `json:"gist,omitempty"`
	// Genre 是这篇文章的体裁，闭表：argument / report / narrative / explain。
	//
	// 🚨 它不摆在屏幕上 —— 它管的是**给她什么工具**。同事 2026-09-17 逐字：
	// 「我总觉得不是所有的文章都应该按照主张、证据、限制这样的内容来拆分，
	// 而且主张、证据、限制很多时候并不知道哪些该在哪里。」他看的那一篇是
	// 战地新闻报道，四句话里一句作者的主张都没有，而 印记 仍然摆出了那块板。
	//
	// 允许为空：认不出体裁就不挡任何东西（老数据也一样）。判错的方向和别处
	// 一致——宁可放过，不可误伤：少发一块板是少一次练习，发错一块板是让她
	// 对着一套根本不适用的词干瞪眼。
	Genre string            `json:"genre,omitempty"`
	Shape string            `json:"shape"`
	Load  map[string]string `json:"load"`
	// Parts 是这篇分成的几个部分，按正文顺序。允许为空 —— 老数据没有，
	// 校验没过的也会被整个丢掉（见 validateOutline）。
	Parts []readingPart `json:"parts,omitempty"`
}

const (
	outlineOneLineMaxRunes = 60
	outlineGistMaxRunes    = 80
	outlineShapeMaxRunes   = 40
	outlinePartTitleMax    = 20
	outlinePartDoesMax     = 40
	// 一篇文章最多切几部分。六：再多就不是「部分」了，那是把段落重新编了一次号，
	// 而带读要一部分停一次 —— 七个停顿比不停更难走完。
	outlinePartsMax = 6
	// 少于两部分就不是分部分。一整篇算一部分，等于没切。
	outlinePartsMin = 2
)

// 体裁闭表。四个词，和 prompt 里那一段一一对应。
const (
	genreArgument  = "argument"  // 作者在说服你接受一个看法
	genreReport    = "report"    // 新闻报道：发生了什么、各方怎么说
	genreNarrative = "narrative" // 记叙：一件事按时间讲下来
	genreExplain   = "explain"   // 说明：讲清楚一样东西是怎么回事
)

// validateGenre 把模型给的体裁收进闭表。认不出来就是空 —— 空不挡任何东西。
func validateGenre(s string) string {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case genreArgument:
		return genreArgument
	case genreReport:
		return genreReport
	case genreNarrative:
		return genreNarrative
	case genreExplain:
		return genreExplain
	}
	return ""
}

// coreShareCap 是核心段占全文的上限，写成分母：core * coreShareCap > 总段数
// 就作废。
//
// 🚨 **2026-09-17 从 3（三分之一）改成 2（一半），有实测。**
//
// 这条注释原来写的是：「三分之一：一篇十二段的报道，核心段最多四段。这个数字
// 不是精算出来的，它是『还能构成一个判断』的边界 —— **一半以上都是核心，
// 这份分类就没有在分类**。」最后那句说的是一半，代码写的是三分之一 —— 从一开始
// 这两者就不是同一个数。
//
// 实测（`LIVE_LLM=1 ... TestLiveEnglishReadingPlanParses`，一篇 12 段的英文
// 社论，两轮各 6 次）：模型反复给出 **5/12** 段核心，而 12/3=4，于是整份导读
// 被丢掉 —— 第一轮 3/6，第二轮 1/6。被丢掉的那几份**别的什么毛病都没有**：
// 中文写得好、中心思想准、12 段每段都有承重、切法完整。5/12 是 42%，
// 它离「全是核心」很远。
//
// 代价 2026-09-17 起翻了一倍：切法（parts）和这份导读同生共死，而切法现在是
// 通读那一步的台阶（reading_plan.go 的 readingPartSteps）。一段承重标得宽一点，
// 换来的是她的通读退回「读完告诉我一声」那个死锁。
//
// 「一段核心都没有」那条没动：那一种确实是没在分类。
const coreShareCap = 2

// blank 说的是这份导读什么都没说。空的 outline 不落库，前端因此不必区分
// 「没有导读」和「有一份空导读」。
func (o readingOutline) blank() bool {
	return strings.TrimSpace(o.OneLine) == "" && strings.TrimSpace(o.Shape) == "" && len(o.Load) == 0
}

// validateParts 把模型切出来的那几个部分收进我们能保证的范围里。
//
// 返回 nil 表示这份切法不能用 —— **整份丢掉，不修补**。一份被修补过的切法在
// 屏幕上和一份真的切法长得一模一样，而她没有办法分辨；而且带读会照着它一部分
// 一部分地走，走到一个不存在的段落上就断在那儿。
//
// 五条判据，每一条都对应一种真的会发生的坏切法：
//
//	段 id 不存在        模型自己数段号，数错了（它不止一次把 b13 写成 b31）
//	from 在 to 后面      头尾写反
//	和上一部分重叠       同一段属于两个部分，带读会把它读两遍
//	不是从第一段开始      前面几段没人管，她读到那里没有任何提示
//	少于两部分           一整篇算一部分，等于没切
//
// 中间**允许有缝**（上一部分到 b5、下一部分从 b7 开始）：那是模型漏了一段，
// 而漏一段的代价远小于整份丢掉。缝里的段落照常显示，只是不属于任何一部分。
//
// 尾巴上**不允许有缝**，但也不丢 —— 最后一个部分直接补到末段，见函数末尾。
func validateParts(got []readingPart, blocks []Block) []readingPart {
	if len(got) < outlinePartsMin {
		return nil
	}
	order := make(map[string]int, len(blocks))
	for i, b := range blocks {
		order[b.ID] = i
	}
	out := make([]readingPart, 0, len(got))
	prevEnd := -1
	for _, p := range got {
		from, okFrom := order[strings.TrimSpace(p.From)]
		to, okTo := order[strings.TrimSpace(p.To)]
		if !okFrom || !okTo || from > to || from <= prevEnd {
			return nil
		}
		title := trimRunes(strings.TrimSpace(p.Title), outlinePartTitleMax)
		if title == "" {
			return nil
		}
		prevEnd = to
		out = append(out, readingPart{
			Title: title,
			From:  strings.TrimSpace(p.From),
			To:    strings.TrimSpace(p.To),
			Does:  trimRunes(strings.TrimSpace(p.Does), outlinePartDoesMax),
		})
		if len(out) == outlinePartsMax {
			break
		}
	}
	// 必须从第一段开始。从第三段开始的切法，意味着前两段她读到的时候屏幕上
	// 什么提示都没有 —— 而那两段往往正是导语。
	if len(out) < outlinePartsMin || out[0].From != blocks[0].ID {
		return nil
	}
	// 🚨 **也必须切到最后一段。** 2026-09-17 走查第 1 条：一篇 18 段的文章，
	// 通读走到第 14 段就结束了。
	//
	// 头一端从第一天起就有人守（上面那行），尾巴那一端没有 —— 而 2026-09-17
	// 之后这两端的代价不再对称：切法是通读那一步的台阶
	// （reading_plan.go 的 readingPartSteps），最后一个部分停在哪儿，
	// **她的通读就停在哪儿**。模型漏在中间的缝还有别的步骤兜（那几段照常显示），
	// 漏在尾巴上的那几段是彻底没人读了。
	//
	// 两条路都会走到这里：模型自己就没切到底，或者它切了七八个部分而
	// outlinePartsMax 在第 6 个上 break 掉了后面的。
	//
	// 补而不是丢：这份切法别的地方都是对的，丢掉它换来的是通读退回
	// 「读完告诉我一声」那个死锁（[[reading-room-rulings-2026-09-17]] 里
	// 「整份丢掉的代价会被后来的改动悄悄放大」说的就是这件事）。补出来的那个
	// 部分名字仍然是模型写的，只是管的段落多几段。
	if last := len(out) - 1; out[last].To != blocks[len(blocks)-1].ID {
		out[last].To = blocks[len(blocks)-1].ID
	}
	return out
}

// outlineReject 说的是这份导读为什么没留下。
//
// 🚨 这四种以前共用一个 bool 和一行 `slog.Info("reading plan: outline rejected")`
// —— 于是线上只知道「又没了」，不知道是哪一条，而四条的修法完全不同
// （[[model-json-half-arrived-2026-09-08]] 里排读法那三种 reject 分开数是同一
// 个理由）。2026-09-17 那次实测之所以能定位到「核心段多了一段」，靠的正是把
// 它们分开数。
type outlineReject string

const (
	outlineOK                outlineReject = ""
	outlineRejectNotCJK      outlineReject = "oneLine is not written in Chinese"
	outlineRejectTooMuchCore outlineReject = "more than half the article is marked core"
	outlineRejectNoCore      outlineReject = "no paragraph is marked core"
	outlineRejectBlank       outlineReject = "nothing was filled in"
)

// validateOutline 把模型给的那份导读收进我们能保证的范围里。
//
// 返回的 bool 是「这份还值不值得存」。不值得的时候整份丢掉而不是修补：一份
// 修补过的导读在屏幕上和一份真的导读长得一模一样，而她没有办法分辨。
func validateOutline(got readingOutline, blocks []Block) (readingOutline, bool) {
	out, why := validateOutlineWhy(got, blocks)
	return out, why == outlineOK
}

// validateOutlineWhy 多返回一个「为什么没留下」，给日志用。
func validateOutlineWhy(got readingOutline, blocks []Block) (readingOutline, outlineReject) {
	out := readingOutline{
		OneLine: trimRunes(strings.TrimSpace(got.OneLine), outlineOneLineMaxRunes),
		Gist:    trimRunes(strings.TrimSpace(got.Gist), outlineGistMaxRunes),
		Genre:   validateGenre(got.Genre),
		Shape:   trimRunes(strings.TrimSpace(got.Shape), outlineShapeMaxRunes),
		Load:    map[string]string{},
		// 切法单独校验，单独丢弃：它没过不该让整份导读作废（导读的其余三样
		// 仍然有用），但它坏了也绝不能修补着用 —— 带读会照着它一部分一部分
		// 地走，走到一个不存在的段落上就断在那儿。
		Parts: validateParts(got.Parts, blocks),
	}

	// 每一段都要有一个标签，而且只能是我们认识的那三个。模型漏掉的、写错的，
	// 一律退回「支撑」—— 那是最不打扰的一档：它不会让带读在一段普通的证据上
	// 停下来，也不会让一段真正的核心被跳过去（跳过去的代价更大）。
	core := 0
	for _, b := range blocks {
		if strings.TrimSpace(b.Text) == "" {
			continue
		}
		kind := strings.TrimSpace(got.Load[b.ID])
		switch kind {
		case loadCore, loadSupport, loadBridge:
		default:
			kind = loadSupport
		}
		if kind == loadCore {
			core++
		}
		out.Load[b.ID] = kind
	}

	// 🚨 导读必须是中文的。
	//
	// 线上第一次跑就写成了英文（「War is escalating — can aid groups still
	// reach…」「outbreak → blockade → aid scramble → war」）—— 文章是英文的，
	// 模型顺着文章的语言写了下去。那份导读摆在中文界面上，对她等于不存在。
	//
	// prompt 里当然也写了这一条，但一句 prompt 里的「必须」如果代码里验不了，
	// 它就只是一句期望（[[prompt-output-must-be-verifiable-2026-09-03]]）。
	// 判据取最宽的那一个：**一个汉字都没有**才算没写中文。这样英文的专有名词
	// （人名、地名、机构名）照抄原文不会被误伤 —— 那本来就是对的做法。
	//
	// 🚨 2026-09-18 起**按字段丢，不再整份丢**。线上同一篇英文故事连着两次只有
	// oneLine 写成了英文（「What made her change her mind」），其余几样都是中文 ——
	// 而整份作废把切法也一起带走了，于是一篇十段的故事只剩「通读全文」一步，
	// 她的台阶没了。那是 2026-09-17 记下的同一个坑：往「整份作废」的校验上挂
	// 东西，代价是挂上去的每一样（[[reading-room-rulings-2026-09-17]]）。
	//
	// 丢掉一个字段不是修补：屏幕上少一行，而不是多一行编出来的话。切法的标题
	// 会印在她的清单上（「通读第1–3段·<标题>」），所以哪个部分的标题是英文，
	// 整份切法照旧丢 —— 切法本来就是整份丢、不修补的（validateParts）。
	// 三样一个中文字都没剩、也没有切法，才算这份导读没有写中文。
	if !hasCJK(out.OneLine) {
		out.OneLine = ""
	}
	if !hasCJK(out.Gist) {
		out.Gist = ""
	}
	if !hasCJK(out.Shape) {
		out.Shape = ""
	}
	for _, part := range out.Parts {
		if !hasCJK(part.Title) {
			out.Parts = nil
			break
		}
	}
	if out.OneLine == "" && out.Gist == "" && out.Shape == "" && len(out.Parts) == 0 {
		return readingOutline{}, outlineRejectNotCJK
	}

	// 一半以上都是核心 = 没有核心。见 coreShareCap。
	if core*coreShareCap > len(out.Load) {
		return readingOutline{}, outlineRejectTooMuchCore
	}
	// 一段核心都没有，这份分类也没在分类 —— 带读会退回「哪儿都不停」。
	if core == 0 {
		return readingOutline{}, outlineRejectNoCore
	}
	if out.blank() {
		return readingOutline{}, outlineRejectBlank
	}
	return out, outlineOK
}

// coreBlockIDs 是核心段的 id，按正文顺序。
func (o readingOutline) coreBlockIDs(blocks []Block) []string {
	out := make([]string, 0, 4)
	for _, b := range blocks {
		if o.Load[b.ID] == loadCore {
			out = append(out, b.ID)
		}
	}
	return out
}

// decodeOutline 读一行 reading_source 上的那一列。读不动就当没有 —— 一份坏掉的
// 导读不该让整篇文章打不开。
func decodeOutline(raw []byte) readingOutline {
	// Load 永远返回一张真的 map，不是 nil。读一张 nil map 在 Go 里是安全的，
	// 但**写**它会 panic —— 这里不留那个坑给以后。
	empty := readingOutline{Load: map[string]string{}}
	if len(raw) == 0 {
		return empty
	}
	var o readingOutline
	if err := json.Unmarshal(raw, &o); err != nil {
		return empty
	}
	if o.Load == nil {
		o.Load = map[string]string{}
	}
	return o
}

// hasCJK —— 这段字里有没有汉字。判的是 CJK 统一汉字那一段，不含假名和谚文：
// 我们要分辨的是「中文」和「英文」，不是「东亚文字」和别的。
func hasCJK(s string) bool {
	for _, r := range s {
		if r >= 0x4E00 && r <= 0x9FFF {
			return true
		}
	}
	return false
}

func trimRunes(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n])
}
