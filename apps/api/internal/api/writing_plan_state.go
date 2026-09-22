package api

import (
	"strconv"
	"strings"

	"mindimprint/api/internal/store/sqlc"
)

// writing_plan_state.go —— 规划那一步的两件事：**计划现在有什么**（由服务端算，
// 当事实喂进 prompt），和**什么时候别再问了**（由代码数，不由模型感觉）。
//
// # 为什么这两件事不能交给模型
//
// `planLooksReady`（writing_plan.go）已经做对了一半：模型永远还能再教一点，
// 所以「够了没有」不能只听它的，由地图的形状来兜底。缺的是另一半——
// **一个话很少的学生**。她的地图永远到不了「一个中心论点 + 两条分论点 +
// 一条她自己的材料」，于是 `planLooksReady` 永远是 false，模型就一直问下去。
//
// 阅读室 2026-09-10 栽的是同一个跟头（「通读那一步会变成审问」，连七轮），
// 修法也是同一个：**数轮数**，不是让模型更小心。营地走查里的林知遥就是这一种
// 学生，她的那句 probe 写着「问题：产品会不会因为她不主动说，就一直停在原地」。
//
// # 停下来的判据，抄的是 my-literary-moment
//
// 那个 skill 的「停止提问与转场」写了三条，满足任一即停：
//
//	1. 完整性：三层信息均已具象（即使用户回答简短）。
//	2. 意愿：用户说「就这些」「帮我写吧」。
//	3. 效率：在经历层和感受层都已完成后，**若连续两次提问用户都只给出极简
//	   无实质内容回答**（如「嗯」「没啥」），停止提问。
//
// 第 1 条我们已经有了（`planLooksReady`）。第 2 条已经在 prompt 里（她说想写
// 就立刻 ready）。**第 3 条是这个文件**，而它是三条里唯一数得出来的那条。
//
// 配套还有一条同样重要的：回答「不知道」「没感觉」→ **接受原话，不反复追问**。
// 所以下面那张表里就有这几个词——它们不是「无效输入」，是她真实的回答，
// 而正确的反应是往下走，不是换个说法再问一遍。

// writingPlanStallTurns 连续几轮低实质回答就停。
//
// 二，和 my-literary-moment 的「连续两次」一致。一轮太急（谁都会有一句
// 「嗯」），三轮太慢——她第三次答「不知道」的时候，多半已经在想退出了。
const writingPlanStallTurns = 2

// writingPlanSubstantiveRunes 一句回答至少要有几个字才算「说了点什么」。
//
// 🚨 按 rune 数，不是 byte 数。中文一个字三个 byte，按 byte 算等于把门槛降到
// 三分之一。
//
// 🚨 **四，而且这个数字是被测试打下来的。** 第一版写的是八，于是
// 「我想写食堂浪费」——七个字，一个完整、具体、可以直接开工的回答——
// 被判成了「等于没答」。中文里一句短句往往就是一整句话，长度在这里几乎不带
// 信息。真正干活的是下面那张闭表；长度只负责兜住表里漏掉的那几个
// （「就那样」「随你」），所以门槛压到刚好比它们高一点。
const writingPlanSubstantiveRunes = 4

// writingPlanEmptyReplies 是「等于没答」的那些整句。
//
// 🚨 **整句比对，不是子串比对**，而且这一点是承重的：
// 「不知道该从哪儿说起，可能是食堂那个事吧」里含有「不知道」，
// 但它给了一个真的题目。子串比对会把这句判成空的，然后停止提问——
// 恰好在她终于说出一件事的那一轮。
var writingPlanEmptyReplies = map[string]bool{
	"嗯": true, "嗯嗯": true, "哦": true, "噢": true, "啊": true,
	"好": true, "好的": true, "行": true, "行吧": true, "可以": true,
	"还行": true, "一般": true, "差不多": true, "都行": true, "随便": true,
	"不知道": true, "我不知道": true, "不清楚": true, "不太清楚": true,
	"我也不清楚": true, "我也不知道": true, "说不好": true, "说不上来": true,
	"没有": true, "没了": true, "没什么": true, "没想法": true, "没感觉": true,
	"没什么想法": true, "没想好": true, "还没想好": true,
	"不会": true, "想不出来": true, "不太会": true, "写不出来": true,
	"ok": true, "okay": true, "yes": true, "no": true, "idk": true,
	"i don't know": true, "nothing": true, "not sure": true,
}

// lowSubstanceReply 判断这一句学生回答是不是「等于没答」。
//
// 两条路都算：整句落在闭表里，或者短于 writingPlanSubstantiveRunes。
// 后者兜住表里没有的那些（「就那样」「随你」），前者兜住比阈值长的那些
// （「我不太清楚」六个字，短；「i don't know」十二个 byte 但只有三个词）。
func lowSubstanceReply(s string) bool {
	t := strings.TrimSpace(s)
	if t == "" {
		return true
	}
	// 句末的标点不影响它是不是一句「嗯」。
	t = strings.TrimRight(t, "。．.！!？?～~、，,　 ")
	if writingPlanEmptyReplies[strings.ToLower(t)] {
		return true
	}
	return len([]rune(t)) < writingPlanSubstantiveRunes
}

// writingPlanStalled 数**结尾处**连续几轮她都等于没答。
//
// 只看她说的话（role=student），并且只数最后连续的那一段：中间答得好、
// 最近两轮敷衍，该停；最近两轮答得好、开头敷衍过，不该停。
//
// 🚨 从后往前数，一遇到有实质的回答立刻收手。这是「连续」两个字的意思，
// 而不是「总共有两轮低实质回答」——后者会在一段长对话里几乎必然成立。
//
// 🚨 `pending` 是**她这一轮刚说的那句话**，必须单独传进来：调用点拿到的
// `msgs` 是在把这一轮落库**之前**读出来的（writing_plan.go 里
// ListAtomMessages 在 AppendAtomMessage 之前），所以只数 msgs 永远差最新的
// 那一轮。第一版就是这么写的，于是「连续两轮」实际判的是「上两轮」，
// 她这一次答的「不知道」要等到下一轮才被看见。
func writingPlanStalled(msgs []sqlc.AtomMessage, pending string) bool {
	run := 0
	if !lowSubstanceReply(pending) {
		return false
	}
	run++
	if run >= writingPlanStallTurns {
		return true
	}
	for i := len(msgs) - 1; i >= 0; i-- {
		if msgs[i].Role != "student" {
			continue
		}
		if !lowSubstanceReply(msgs[i].Content) {
			return false
		}
		run++
		if run >= writingPlanStallTurns {
			return true
		}
	}
	return false
}

// writingPlanShape 是这份计划此刻的形状。
//
// 这几个数字 `planLooksReady` 本来就在算，只是算完就丢掉了。把它们**当事实
// 写进 prompt**，模型就不必从十六轮对话里重新推一遍「她定下中心论点了吗」——
// 而那个重新推的动作，是它每一轮都在做、而且每一轮都可能推错的事。
type writingPlanShape struct {
	Top    int // depth 0：中心论点、开头、结尾这些最上层的块
	Points int // depth 1：分论点
	// depth 2 及以下：撑着某条分论点的材料。
	//
	// 🚨 **两种都算**：她自己见过、经历过的事，以及她找来的研究、报道、数据、
	// 访谈。这个计数从来就没有区分过两者（它只看深度），但 prompt 那边一直只
	// 问「你自己经历过吗」—— 2026-09-16 改成两种并列，理由见 writing_plan.go
	// 的「材料有两种」。一个十五岁的学生，自己的经历通常只够撑一条理由。
	Material int
	// Wider records external materials for internal diagnostics, not readiness.
	Wider int
}

// count 把一个节点记进形状里 —— writingPlanShapeOf 和 writingPlanShapeWith
// （还没落库的那一轮）共用，两边不会数得不一样。
//
// 🚨 2026-09-20：这个函数原来要靠四张关键词表去猜一个节点是什么
// （writingRoleIsExample / writingRoleIsReasoning / writingRoleIsPoint /
// writingRoleIsPersonal），而每一张都是一次线上事故的补丁：
// 例子落在最上层被当成分论点、挂得更深的分论点被当成例子、一条道理被当成例子。
// 现在节点自己带着 kind，这里就只剩一个 switch。
//
// 材料不管挂在哪一层都是材料，分论点不管挂在哪一层都是分论点 —— 深度已经由
// kind 强制过了，这里连深度都不必看。
func (s *writingPlanShape) count(kind, source string) {
	_ = source
	switch kind {
	case writingKindOpening, writingKindThesis, writingKindClosing:
		s.Top++
	case writingKindPoint, writingKindCounter:
		s.Points++
	case writingKindEvidence, writingKindReference:
		s.Material++
		if writingKindIsWider(kind) {
			s.Wider++
		}
	case writingKindReasoning, writingKindRebuttal:
		// 🚨 道理和对反方的回应都是**推理**，不是材料。
		// 2026-09-18 实测：一条道理被当成「不是个人经历的例子」，于是只有她
		// 自己那一件事也过了线。道理站得住是好事，但它撑不起「你有什么证据」。
	case writingKindGap:
		// 🚨 一个洞不是一条材料。见 writing_kind.go 的 writingKindIsMaterial。
	}
}

func writingPlanShapeOf(rows []sqlc.WritingOutline) writingPlanShape {
	var s writingPlanShape
	for _, r := range rows {
		if strings.TrimSpace(r.Text) == "" {
			continue
		}
		s.count(writingKindOf(r), r.Source)
	}
	return s
}

// ready 是 planLooksReady 的判据，写在形状上。
//
// 两处必须一致，所以 planLooksReady 改成调用这里——一份判据两个实现，
// 是它们悄悄分岔的唯一原因。
//
// 判据跟着这一篇的篇幅走，见 writingPlanNeedOf。
func (s writingPlanShape) ready(need writingPlanNeed) bool {
	return s.Top >= 1 && s.Points >= need.Points && s.Material >= need.Material
}

// writingPlanNeed 是这一篇**按它的篇幅**该有的骨架。
//
// 🚨 **一条固定的线，量不了两种长度的文章。**
//
// 产品负责人 2026-09-12：
//
//	「希望能支持不同类型的文本的写作，目前思路梳理的部分太短了，
//	  800字和3000字的论文的思路长短不一，需要扩展。」
//
// 她说得准。篇幅本来只以一句软话进了提示词（「篇幅只用来判断要几条分论点」），
// 而**真正放她走的那条线是写死的**：两条分论点、一条材料。于是一篇 3000 字的
// 论文和一篇 800 字的短文，在第三条分论点还没出现的时候就同时被判为「想好了」。
// 模型那边再怎么被提醒篇幅，也跨不过一条服务端写死的线 —— 这和
// [[prompt-twice-then-make-it-checkable]] 是同一件事的另一面：
// 真正算数的是代码里那条判据，那就得让它知道篇幅。
//
// 每条分论点撑多少：中文按 400 字，英文按 250 词（英文一个词大致抵一个半到
// 两个汉字，这个比例和 writingLengthLine 里换算单位的那一处是同一个来源）。
//
//	800 字  → 2 条分论点、2 个例子
//	1600 字 → 4 条、4 个
//	3000 字 → 封顶 4 条、4 个
//
// 封在 4：再往上就不是「还没想清楚」，而是这篇文章该拆章节了，而拆章节不是
// 拦着她不让动笔的理由。下限仍是 2 —— 一条理由撑不起一篇议论文。
//
// 没设目标字数就还是原来那条线：不知道她要写多长，就不该替她加码。
//
// 🚨 2026-09-18 产品负责人：「对于一个800字的议论文，要求起码2-3个例子，
// 目前思维导图的长度完全不够。」原来 800 字只要 1 条材料 —— 两条分论点里有一条
// 是空推理也放她去写。现在例子至少和分论点一样多、下限 2 个。材料来源不作为数量门槛；
// 具体证据是否合适，由教练结合观点与题目要求讨论。
type writingPlanNeed struct {
	Points   int
	Material int
}

const (
	writingPlanRunesPerPoint = 400 // 中文，字
	writingPlanWordsPerPoint = 250 // 英文，词
	writingPlanMinPoints     = 2
	writingPlanMaxPoints     = 4
	writingPlanMinExamples   = 2
)

func writingPlanNeedOf(wr sqlc.Writing) writingPlanNeed {
	need := writingPlanNeed{Points: writingPlanMinPoints, Material: writingPlanMinExamples}
	if wr.TargetWords == nil {
		return need
	}
	per := writingPlanRunesPerPoint
	if wr.Lang == langEnglish {
		per = writingPlanWordsPerPoint
	}
	n := int(*wr.TargetWords) / per
	if n < writingPlanMinPoints {
		n = writingPlanMinPoints
	}
	if n > writingPlanMaxPoints {
		n = writingPlanMaxPoints
	}
	need.Points = n
	// 每条分论点底下至少一个例子，下限 2（800 字要 2–3 个例子）。
	need.Material = n
	if need.Material < writingPlanMinExamples {
		need.Material = writingPlanMinExamples
	}

	return need
}

// promptBlock 把形状渲染成 prompt 里那一段。
//
// 写的是**还缺什么**，而不只是有什么：模型要挑的是下一个问题，
// 而「下一个问题」直接由缺口决定。这也是 my-literary-moment 的「单点聚焦」——
// 她一次给了多层就跳过已有层，只问缺的。
//
// 🚨 这里的门槛必须和 ready(need) 用同一份 need。写死一个 2 的话，一篇 3000 字
// 的论文里模型会以为两条分论点就齐了、催她去写，而服务端那道门还关着 ——
// 两边说的话对不上，她就卡在中间。
func (s writingPlanShape) promptBlock(need writingPlanNeed) string {
	var b strings.Builder
	b.WriteString("\n【这份计划现在有什么】（服务端数出来的，不用你再数一遍）\n")
	b.WriteString("- 最上层的块：" + strconv.Itoa(s.Top) + " 个\n")
	b.WriteString("- 分论点：" + strconv.Itoa(s.Points) + " 条（这篇篇幅下要 " + strconv.Itoa(need.Points) + " 条）\n")
	b.WriteString("- 例子（挂在某条分论点下面的材料）：" +
		strconv.Itoa(s.Material) + " 个（要 " + strconv.Itoa(need.Material) + " 个）\n")

	missing := s.missing(need)
	if missing == "" {
		b.WriteString("- **判据都满足了。这一轮就请她去写。**\n")
		return b.String()
	}
	b.WriteString("- 还缺：" + missing + "。\n")
	b.WriteString("- **按上面这个顺序补，一轮补一件。**\n")
	return b.String()
}

// missing 是「还缺什么」那一句，空串 = 判据都满足了。promptBlock 和
// 「请她去写得太早」那次重试（writingPlanReadyTooSoonNudge）说的是同一句。
func (s writingPlanShape) missing(need writingPlanNeed) string {
	var missing []string
	if s.Top == 0 {
		missing = append(missing, "这篇要说的那一句话还没定下来")
	}
	if s.Points < need.Points {
		missing = append(missing, "说明中心论点的分论点还不到 "+strconv.Itoa(need.Points)+" 条")
	}
	if s.Material < need.Material {
		missing = append(missing, "与分论点相关的例子还不到 "+strconv.Itoa(need.Material)+
			" 个（每条分论点底下至少一个）")
	}
	return strings.Join(missing, "；")
}

// outlineHasText 说这张图上有没有已经写着这句话的节点。
//
// 比的是**规范化之后**的文字：去掉首尾空白、统一大小写（英文那一侧
// "Serving staff give too much" 和 "serving staff give too much" 是同一句），
// 并且把空白和常见句读抹平 —— 模型复述同一句话时，最常变的就是句末那个标点。
//
// 🚨 不做模糊匹配。「差不多的两句」是两句，判重只认「基本上就是同一句」：
// 判错的代价不对称 —— 多留一个重复节点她自己看得见、能改；把她真的新说的
// 一件事当成重复丢掉，她永远不知道发生过什么。
func outlineHasText(rows []sqlc.WritingOutline, text string) bool {
	want := normalizeOutlineText(text)
	if want == "" {
		return false
	}
	for _, r := range rows {
		if normalizeOutlineText(r.Text) == want {
			return true
		}
	}
	return false
}

func normalizeOutlineText(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	var b strings.Builder
	for _, r := range s {
		switch r {
		case ' ', '\t', '\n', '　',
			'。', '，', '、', '；', '：', '！', '？',
			'.', ',', ';', ':', '!', '?':
			// 空白和句读不参与比对。
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}
