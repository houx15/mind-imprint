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
	Top      int // depth 0：中心论点、开头、结尾这些最上层的块
	Points   int // depth 1：分论点
	Material int // depth 2 及以下：她自己的材料
}

func writingPlanShapeOf(rows []sqlc.WritingOutline) writingPlanShape {
	var s writingPlanShape
	for _, r := range rows {
		if strings.TrimSpace(r.Text) == "" {
			continue
		}
		switch r.Depth {
		case 0:
			s.Top++
		case 1:
			s.Points++
		default:
			s.Material++
		}
	}
	return s
}

// ready 是 planLooksReady 的判据，写在形状上。
//
// 两处必须一致，所以 planLooksReady 改成调用这里——一份判据两个实现，
// 是它们悄悄分岔的唯一原因。
func (s writingPlanShape) ready() bool {
	return s.Top >= 1 && s.Points >= 2 && s.Material >= 1
}

// promptBlock 把形状渲染成 prompt 里那一段。
//
// 写的是**还缺什么**，而不只是有什么：模型要挑的是下一个问题，
// 而「下一个问题」直接由缺口决定。这也是 my-literary-moment 的「单点聚焦」——
// 她一次给了多层就跳过已有层，只问缺的。
func (s writingPlanShape) promptBlock() string {
	var b strings.Builder
	b.WriteString("\n【这份计划现在有什么】（服务端数出来的，不用你再数一遍）\n")
	b.WriteString("- 最上层的块：" + strconv.Itoa(s.Top) + " 个\n")
	b.WriteString("- 分论点：" + strconv.Itoa(s.Points) + " 条\n")
	b.WriteString("- 她自己的材料（挂在某条分论点下面的）：" + strconv.Itoa(s.Material) + " 条\n")

	var missing []string
	if s.Top == 0 {
		missing = append(missing, "这篇要说的那一句话还没定下来")
	}
	if s.Points < 2 {
		missing = append(missing, "支撑它的分论点还不到两条")
	}
	if s.Material == 0 {
		missing = append(missing, "还没有一条她自己见过、经历过的材料")
	}
	if len(missing) == 0 {
		b.WriteString("- **三条判据都满足了。这一轮就请她去写。**\n")
		return b.String()
	}
	b.WriteString("- 还缺：" + strings.Join(missing, "；") + "。\n")
	b.WriteString("- **按上面这个顺序补，一轮补一件。**\n")
	return b.String()
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

// writingPlanStalledBlock 是她连着两轮等于没答时，加进 prompt 的那一段。
//
// 🚨 这一段**只在真的停滞时出现**，不做常驻。2026-09-05 的教训：
// 一轮只能做一件事，而把这类提示做成常驻，模型会一直去处理那条提示、
// 把该做的事挤掉（那次是六轮里一直在补一张卡，她的主页三处一直是空的）。
const writingPlanStalledBlock = `
【她连着两轮几乎没说什么】
不要再换一个说法问同一件事，也不要再问一个新问题——她已经答了两次「不知道」
那一类的话，第三次只会让她想退出。

这一轮做这三件事，然后收住：
1. 如果图上已经有东西，就照着图上**她自己写过的那一句**说一句具体的话。
2. 说清她现在就可以去写——写出来之后再回来补计划，比在这儿继续想更省力。
3. **这一轮一个问号都不要有。**
`

