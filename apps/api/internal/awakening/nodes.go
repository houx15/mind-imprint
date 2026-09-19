package awakening

// nodes.go —— 终端那 8 个节点。
//
// 这条链来自参考设计（SPARK → FOCUS → CONNECT → TENSION → QUESTION → READ →
// THINK → CREATE），措辞基本照用，按 AGENTS.md §界面文案怎么写 收了几处口语。
// 参考设计里还有第 9 个节点 PROJECT，它做的事是「把前面八轮整理成一张卡」——
// 在这一版里那件事由**报告**做，所以终端只有 8 个要她回答的节点。
//
// # 为什么问题写死在服务端
//
// 模型不挑下一个问题，也不宣布她做完了。它拿到的是「当前这个节点要问什么」，
// 任务是接住她上一句、再把这个问题用自己的语气问出来。
//
// 让模型自己决定推进，是参考设计里那个矛盾的来源：它的 prompt 写着「ready
// 不必等到第 9 个节点」，而终端又按固定 9 步走。服务端数得出来的事实就别让
// 模型每轮自己数（memory: hardcoded-thresholds-vs-user-set-scale-2026-09-12）。

// Node 是终端里的一个节点。
type Node struct {
	// Code 是终端左上角那一行状态字，也是日志里认这一轮的标识。
	Code string
	// Objective 只进 prompt，她看不见。它说这一问**要问出什么**，
	// 所以模型改写措辞时知道哪些东西不能丢。
	Objective string
	// Ask 是这一问的默认问法。她第一次进来看见的就是它。
	Ask string
	// MinRunes 是这一节点的回答短到什么程度就该再问一次。
	//
	// 不是及格线，是**「这句话里没有可摘的东西」**的判据：三个字的回答里
	// 没有原话可以当 evidence，那这个词就长不出来，不如当场换个问法再问一次。
	MinRunes int
	// Retry 是她答得太薄时换的那个问法。参考设计管它叫 alternative entry：
	// 不重复同一句话，而是换一个更具体、更容易落地的入口。
	Retry string
}

// Nodes 是终端的全部节点，顺序就是推进顺序。
var Nodes = []Node{
	{
		Code:      "NODE 01 / SPARK",
		Objective: "从她最近的真实经历里，找到一个她愿意继续靠近的兴趣对象。",
		Ask:       "最近有什么话题、作品或现象，让你主动点开、反复谈起，或者看完还想继续了解？请说一个具体的例子。",
		MinRunes:  8,
		Retry:     "那先不找「最喜欢的事」。最近你主动点开过哪条视频、文章、故事或讨论？哪一个看完之后还留在脑子里？",
	},
	{
		Code:      "NODE 02 / FOCUS",
		Objective: "把宽泛的兴趣缩小成一个具体的细节。",
		Ask:       "在这件事里，最抓住你的是哪一个细节、画面、人物、规则或矛盾？如果只能保留一部分，你会留下什么？",
		MinRunes:  8,
		Retry:     "不用概括整个主题。回到一个最具体的瞬间：哪个画面、人物、规则、句子或冲突让你停了下来？",
	},
	{
		Code:      "NODE 03 / CONNECT",
		Objective: "把这个细节连回她自己的经历、感受或她身边正在发生的事。",
		Ask:       "这个细节为什么会让你在意？它让你想到自己的哪段经历、某种感受，或者身边正在发生的什么事？",
		MinRunes:  8,
		Retry:     "如果一时说不清原因，就想想它让你感到什么：兴奋、困惑、不公平、熟悉，还是被理解？选最接近的一种，说说是在什么时候。",
	},
	{
		Code:      "NODE 04 / TENSION",
		Objective: "找到一处她想不通、感到意外，或者知道有争议的地方。这是后面那个研究问题的来源。",
		Ask:       "关于它，哪一点最让你想不通、感到意外，或者你觉得大家可能有不同看法？",
		MinRunes:  8,
		Retry:     "换个入口：关于这件事，别人最常说的一句话是什么？你完全同意吗，还是觉得哪里不够？",
	},
	{
		Code:      "NODE 05 / QUESTION",
		Objective: "把兴趣和那处反差变成一个开放、具体、能靠阅读验证的问题。",
		Ask:       "如果把这份好奇变成一个搜一次答不了的问题，你现在最想追问什么？可以从「为什么」「如何」或者「在什么条件下」开始。",
		MinRunes:  8,
		Retry:     "先把你的困惑写成一句「为什么……」或者「在什么条件下……」。不用写得完整，只要它值得继续查材料。",
	},
	{
		Code:      "NODE 06 / READ",
		Objective: "规划为回答这个问题需要读的三类材料：基础概念、真实案例、不同观点。",
		Ask:       "为了回答这个问题，你还需要先读懂什么？基础概念、真实案例，还是和你不同的观点？请说明原因。",
		MinRunes:  6,
		Retry:     "想象你要向同学解释这个问题：你最怕自己在哪一点上说不清？那一点就是第一类要读的内容。",
	},
	{
		Code:      "NODE 07 / THINK",
		Objective: "在阅读之前先形成一个能被材料修正的暂定回答。",
		Ask:       "在继续阅读之前，你目前的暂定回答是什么？你依据什么这样想，又看到什么证据时愿意修改它？",
		MinRunes:  8,
		Retry:     "先用「我目前猜测……因为……」说一句，再补一句「如果看到……我会修改想法」。这里不要求正确。",
	},
	{
		Code:      "NODE 08 / CREATE",
		Objective: "把探究变成一件面向真实读者的作品设想。",
		Ask:       "如果把探索结果做成一件作品，你最想让谁看到？你想让他理解什么，或者开始讨论什么？文章、视频、播客、图解、展览都可以。",
		MinRunes:  8,
		Retry:     "先不选形式。你最想让同学看完之后理解一个什么观点，产生一种什么感受，或者开始讨论什么问题？",
	},
}

// NodeCount 是要她回答的节点数。界面上的「第几步 / 共几步」读它。
var NodeCount = len(Nodes)

// NodeAt 取第 i 个节点。越界给最后一个 —— 一次越界的请求不该让对话调用崩掉，
// 而调用方（advance）另外会把 i 夹回范围内。
func NodeAt(i int) Node {
	if i < 0 {
		return Nodes[0]
	}
	if i >= len(Nodes) {
		return Nodes[len(Nodes)-1]
	}
	return Nodes[i]
}

// TooThin 报告这个回答薄到该在同一个节点再问一次。
//
// 判据是**字符数**加上一小张「等于什么都没说」的表。两者都必要：
// 「不知道」只有三个字，会被长度挡下；「我就是喜欢啊反正说不清楚」有十二个字，
// 长度挡不住，但它里面同样没有一句可以当 evidence 的原话。
//
// 这条判据的后果是再问一次，不是判她答错 —— 换一个更具体的入口（Node.Retry），
// 然后继续。
func TooThin(nodeIndex int, answer string) bool {
	a := foldSpace(answer)
	if a == "" {
		return true
	}
	if runeLen(a) < NodeAt(nodeIndex).MinRunes {
		return true
	}
	return isEmptyAnswer(a)
}

// emptyAnswers 是「等于什么都没说」的那些话。
//
// 来自参考设计的 isRecallDifficulty / needsInterestClarification 两条正则，
// 摊平成一张表：一张表读得出、测得到，而两条长正则读不出自己漏了什么。
var emptyAnswers = []string{
	"想不起来", "不记得", "记不清", "不知道", "没想过", "没有印象",
	"好像没有", "没什么", "无所谓", "不知道怎么说", "说不清",
	"什么都喜欢", "都喜欢", "都可以", "随便", "就是喜欢",
	"因为好玩", "因为有趣",
}

// isEmptyAnswer 报告这句话是不是只由「说不清」这类词构成。
//
// 🚨 判据是**去掉这些词之后还剩不剩东西**，不是「包含其中一个」。
// 「我不知道为什么，但每次看到猫从高处跳下来我都会停下来看完」包含「不知道」，
// 而它是这一轮里最有价值的一句话。按包含判，它会被当成没说话。
func isEmptyAnswer(s string) bool {
	rest := s
	for _, w := range emptyAnswers {
		rest = replaceAll(rest, w, "")
	}
	// 去掉那些词和标点之后还剩下三个字以上，就说明她说了别的东西。
	return runeLen(stripPunct(rest)) < 3
}
