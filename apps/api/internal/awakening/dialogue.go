package awakening

import (
	"fmt"
	"strings"
)

// dialogue.go —— 终端里的每一轮。
//
// # 这一次调用要的是纯文本，不是 JSON
//
// 参考设计让同一次调用既回一句话、又回一份分析，全部包在一个 JSON 对象里。
// 这里不这么做，理由有两条实测过的：
//
//  1. DashScope 聚合口的**流式**会少送最后一个内容分片，而 finish_reason 仍然
//     是 stop（memory: streaming-drops-last-chunk-2026-09-10）。丢的那几个字符
//     里如果有收尾的 `"}`，整份 JSON 作废 —— 她看到的是一轮空白。
//  2. 一句纯文本丢了结尾，她看到的是一句话少了几个字，仍然能继续
//     （memory: model-json-half-arrived-2026-09-08）。
//
// 分析那一半挪到结束时的那一次 compose 调用（selection.go）。那一次可以走
// 非流式，也不急着当场显示。
//
// # 模型不决定推进
//
// 它拿到的是「当前节点要问什么」。它的任务只有两件：接住她刚说的那一点，
// 然后把这个问题用自己的语气问出来。下一个节点是谁、她做完没有，服务端算。

// dialogueMaxRunes 是一轮回复的上限。
//
// 参考设计写的是 70–150 字。给到 200 是因为第二次起要多一句承接她树上已有的
// 词；超过这个长度的一轮在终端那个窄面板里要滚两屏，她会跳过不读。
const dialogueMaxRunes = 200

// historyRuneBudget 是喂回去的上文预算。
//
// 八轮对话全文可以到两三千字。终端要的是「她刚说了什么」，不是通读全程，
// 所以给一个宽裕但有界的预算，从最近的轮次往前取。
const historyRuneBudget = 2400

// Turn 是已经发生过的一轮。
type Turn struct {
	NodeIndex   int
	StudentText string
	Reply       string
}

// DialogueInput 是拼一次对话 prompt 要的全部东西。
type DialogueInput struct {
	Guide GuideProfile
	Brief TreeBrief
	// NodeIndex 是这一轮要问的节点。
	NodeIndex int
	// Retry 为真表示她上一句太薄，这一轮要在**同一个节点**换个问法再问一次。
	Retry bool
	// Last 为真表示她刚回答的是最后一个节点，后面没有问题了。
	//
	// 不带这个标志时模型会照着「这一步要问的问题」把第八问**再问一遍** ——
	// 她刚答完，印记又问了一次，然后屏幕上写着「八个问题已经问完」。
	// 2026-09-19 用真浏览器走一遍才看见（接口走查看不到这种别扭）。
	Last bool
	// EnergyFocus 是能量卡牌那一屏的结论，一句话。可以为空。
	EnergyFocus string
	// History 是这一趟已经发生过的轮次，按时间正序。
	History []Turn
	// Latest 是她刚敲进去、还没有回复的那一句。
	Latest string
}

// BuildDialoguePrompt 拼出这一轮的 system 与 user 两段。
func BuildDialoguePrompt(in DialogueInput) (system, user string) {
	node := NodeAt(in.NodeIndex)
	ask := node.Ask
	if in.Retry {
		ask = node.Retry
	}
	// 第一轮且她走的是第二趟：开场问题由简报决定，不是节点的默认问法。
	if len(in.History) == 0 && in.NodeIndex == 0 && !in.Retry {
		ask = in.Brief.OpeningAsk()
	}

	var sb strings.Builder
	sb.WriteString(dialogueSystemHead)
	fmt.Fprintf(&sb, "\n你这一轮的身份是「%s」。%s\n", in.Guide.Zh, in.Guide.Style)
	fmt.Fprintf(&sb, "\n当前进度：第 %d 步，共 %d 步。\n", in.NodeIndex+1, NodeCount)
	fmt.Fprintf(&sb, "这一步要问出来的东西：%s\n", node.Objective)
	if in.Last {
		// 最后一问她已经答完了。再问一次只会让她以为自己答错了。
		sb.WriteString("她刚回答的是最后一个问题，后面没有问题了。这一轮只做两件事：" +
			"接住她这句里的一个具体的东西，再做一句暂定的推断。**不要再提问。**\n")
	} else {
		fmt.Fprintf(&sb, "这一步要问的问题：%s\n", ask)
	}
	if in.Retry {
		sb.WriteString("她上一句太短，没有可以引用的内容。这一轮**不要**重复上一个问法，换成上面这个更具体的入口，并且不要说她答得不好。\n")
	}
	if in.EnergyFocus != "" {
		fmt.Fprintf(&sb, "\n她在前一屏的能量卡牌里选出的方向：%s\n", truncRunes(in.EnergyFocus, 160))
	}
	if t := in.Brief.Text(); t != "" {
		sb.WriteString("\n")
		sb.WriteString(t)
	}
	sb.WriteString(dialogueSystemRulesHead)
	// 第三条随「后面还有没有问题」而变。两条都写进去，模型会照前面那条
	// 提问、又照后面那条收尾，自相矛盾的 prompt 得到的是自相矛盾的回复。
	if in.Last {
		sb.WriteString("3. 收尾。**这一轮不要提任何问题。**\n")
	} else {
		sb.WriteString("3. 最后把这一步要问的问题问出来。可以换成你自己的说法，但**要问的那件事不能换**。\n")
	}
	sb.WriteString(dialogueSystemRulesTail)
	fmt.Fprintf(&sb, "回复长度不超过 %d 字。\n", dialogueMaxRunes)

	return sb.String(), buildDialogueUser(in)
}

const dialogueSystemHead = `你是「觉醒协议」里的印记助手，正在和一个中学生一对一谈话。
你的任务是从她自己的经历里，帮她把一个模糊的兴趣变成一个可以继续追问的研究问题。
你不给她贴性格标签，也不根据一个爱好直接推荐职业。`

const dialogueSystemRulesHead = `
怎么回这一轮：
1. 先引用她刚才说的**一个具体的东西**（一个动作、一个画面、一个条件），用她自己的词。
2. 再做一句暂定的推断，用「可能」「看起来」「这样理解对吗」这样的说法。
`

const dialogueSystemRulesTail = `
绝对不要做的事：
- 不要把她没说过的话说成是她说的。写「你说……」「你提到……」的时候，引号里必须是她的原话，一个字都不能改。
- 不要替她回答，不要给她一个结论，不要列出好几个问题让她挑。
- 不要说教，不要把学校学习说成无趣，也不要把游戏说成答案。
- 不要提「第几步」「节点」「协议」这些流程词。她不需要知道系统怎么运作。
- 不要用比喻，不要用「悄悄」「慢慢」「一点一点」这类词。把话说清楚就够了。

只输出给她看的那段话本身，不要任何前缀、标题、编号或解释。
`

// buildDialogueUser 把上文和她刚说的那句拼成 user 段。
//
// 上文按预算从**最近**往前取：对话越往后，最有用的是刚才那两三轮。
func buildDialogueUser(in DialogueInput) string {
	var parts []string
	budget := historyRuneBudget
	for i := len(in.History) - 1; i >= 0; i-- {
		t := in.History[i]
		seg := fmt.Sprintf("学生：%s\n你：%s", t.StudentText, t.Reply)
		n := runeLen(seg)
		if n > budget {
			break
		}
		budget -= n
		parts = append([]string{seg}, parts...)
	}

	var sb strings.Builder
	if len(parts) > 0 {
		sb.WriteString("前面的对话：\n")
		sb.WriteString(strings.Join(parts, "\n\n"))
		sb.WriteString("\n\n")
	}
	sb.WriteString("她刚刚说：\n")
	sb.WriteString(in.Latest)
	sb.WriteString("\n\n请按上面的规则回这一轮。")
	return sb.String()
}

/* ── 回复的两条可验判据 ─────────────────────────────────────────────────── */

// CleanReply 把模型回的那段话收拾成能显示的样子。
//
// 它去掉模型爱加的前缀（「印记助手：」「回复：」）和外层引号，再按上限截断。
// 截断这里可以做，因为这是**模型写的**字，不是她写的字。
func CleanReply(raw string) string {
	s := strings.TrimSpace(raw)
	for _, p := range []string{"印记助手：", "印记：", "回复：", "助手：", "AI：", "assistant:"} {
		s = strings.TrimPrefix(s, p)
	}
	s = strings.TrimSpace(s)
	// 整段被一对引号裹住时去掉它。只在**首尾成对**时去，否则会把一句正常的
	// 「她说『……』」削掉半边。
	if len(s) >= 2 {
		for _, pair := range [][2]string{{`"`, `"`}, {"「", "」"}, {"“", "”"}} {
			if strings.HasPrefix(s, pair[0]) && strings.HasSuffix(s, pair[1]) {
				s = strings.TrimSuffix(strings.TrimPrefix(s, pair[0]), pair[1])
				break
			}
		}
	}
	return truncRunes(strings.TrimSpace(s), dialogueMaxRunes)
}

// quotePairs 是参与幻引判定的引号。
//
// 🚨 **英文直双引号不在这张表里。** 她自己写的正文里本来就有直引号，把它算进
// 成对判定，会把她真写过的话判成幻引
// （memory: prompt-twice-then-make-it-checkable-2026-09-12）。
var quotePairs = [][2]rune{
	{'「', '」'},
	{'『', '』'},
	{'“', '”'},
}

// attributions 是「下面这句话是她说的」的那些说法。
//
// 只有紧跟在它们后面的引号才参与幻引判定。见 HallucinatedQuotes 的说明。
var attributions = []string{
	"你说", "你写", "你提到", "你讲", "你用了", "你用的", "你的原话", "你的话",
	"你刚才", "你刚", "你说过", "你自己说", "按你", "照你", "你形容", "你称",
}

// attributionWindow 是开引号往前看几个字。
//
// 十四个字：够装下「你刚才说的那个」，又不至于跨过一个句号把上一句的「你说」
// 算到这一句头上。
const attributionWindow = 14

// HallucinatedQuotes 找出回复里**被说成是她说的、而她没写过**的话。
//
// # 这条判据的来源
//
// 一次真实事故：印记在对话里引用她没写过的句子，她的原话是「我不知道该听它的
// 还是按我现在的正文来」。当时的修法是写更长的提示词，两次都没收住 ——
// 提示词里的软话跨不过代码里的硬判据。
//
// # 为什么只判「被归给她的」那些引号
//
// 第一版判的是**所有**成对引号。上线前跑 LIVE_LLM 时它当场判失败了一条其实
// 很好的回复：模型写了「那个『等待然后释放』的模式」—— 它在给自己提出的一个
// 模式起名字，中文里引号本来就有这个用法，而它一个字都没说这是她说的。
//
// 那就是在优化一个**影子**：真正的失败是「把她没说过的话说成她说的」，
// 不是「用了引号」。对着影子改判据，会把一条好回复判死，而下一步通常是去改
// prompt 迎合判据 —— 那条路 2026-09-14 走过一次，数字清零、教得更差
// （memory: optimizing-a-detector-made-coaching-worse-2026-09-14）。
//
// 所以这里只判**紧跟在归属说法后面**的引号：「你说『……』」要查，
// 「那个『……』的模式」不查。
//
// corpus **只含她自己写下的文字**，不含印记说过的话：幻引的来源就是它的上文，
// 把上文放进语料等于允许它引用自己编的句子。
//
// 返回的是那些查不到的引文。空切片表示这一轮干净。
func HallucinatedQuotes(reply, corpus string) []string {
	flat := foldSpace(corpus)
	var bad []string
	for _, q := range extractAttributedQuotes(reply) {
		// 一两个字的引文（「对」「是」）不值得判：它几乎一定出现在任何语料里，
		// 判它只会制造噪声。
		if runeLen(q) < 3 {
			continue
		}
		if !strings.Contains(flat, foldSpace(q)) {
			bad = append(bad, q)
		}
	}
	return bad
}

// extractAttributedQuotes 摘出那些**被说成是她说的**引文。
func extractAttributedQuotes(s string) []string {
	var out []string
	r := []rune(s)
	for _, p := range quotePairs {
		start := -1
		for i, c := range r {
			switch c {
			case p[0]:
				start = i
			case p[1]:
				if start >= 0 && i > start+1 && attributedAt(r, start) {
					out = append(out, string(r[start+1:i]))
				}
				start = -1
			}
		}
	}
	return out
}

// StripBadQuotes 把**被归给她、而她没逐字写过**的那几对引号去掉，句子留下。
//
// # 为什么去引号而不是丢掉整轮
//
// 2026-09-19 线上走查里，八轮有一轮被判失败。模型写的是
//
//	你说「让我家那片海滩建不了」
//
// 而她写的是「……让他们一眼看出为什么我家那片海滩建不了」。意思是她的，
// 措辞压缩了一点。按逐字判，这是幻引；按后果判，它离那次真事故很远 ——
// 那次的伤害是「印记引用了她**没写过**的句子，于是她不知道该听谁的」。
//
// 丢掉整轮的代价是她看见一个死掉的终端。去掉引号的代价是那句话变成转述 ——
// 而转述本来就是允许的。**不变量仍然成立**：凡是打上引号说成她原话的，
// 一定逐字出自她。这比放宽比对好：放宽之后「差不多就算」会一路滑到没有判据。
func StripBadQuotes(reply, corpus string) string {
	bad := HallucinatedQuotes(reply, corpus)
	if len(bad) == 0 {
		return reply
	}
	out := reply
	for _, q := range bad {
		for _, p := range quotePairs {
			out = strings.ReplaceAll(out, string(p[0])+q+string(p[1]), q)
		}
	}
	return out
}

// attributedAt 报告开引号（下标 open）前面那一小段里有没有归属说法。
func attributedAt(r []rune, open int) bool {
	from := open - attributionWindow
	if from < 0 {
		from = 0
	}
	before := string(r[from:open])
	for _, a := range attributions {
		if strings.Contains(before, a) {
			return true
		}
	}
	return false
}

