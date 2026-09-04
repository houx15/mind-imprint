package pbl

import "strings"

// stuck.go —— 她连着几轮答不上来。
//
// 🚨 这个信号存在的理由，来自 2026-09-04 的模拟学生走查。
//
// 【怎么问】里那条「不要给他两个选项让他挑」**本身是对的**：把印记自己猜的两种
// 可能塞给学生，她只能在这个框里选一个，而她真看到的东西很可能两个都不是；更要紧
// 的是，猜出来的答案会被当成她的话往下用，那既违背铁律①，也过不了 GroundSiteDraft
// 那道逐字校验。
//
// 但那条规矩**没有出口**。走查里有个学生用自己的话求了三次「给我几个选项选一下」，
// 一次都没得到——印记只能换个侧面再问一遍。第三次之后她不敢再点「接着做我的主页」，
// 她自己的话是「每次点都会被问答不上来的大问题」。另一个学生被追问食堂细节追了十轮，
// 是同一条指令的另一副面孔。
//
// 所以出口不是取消那条规矩，是给它一个有边界的例外，而例外要**能在代码里验**：
// prompt 里写「她说不上来的时候要给例子」是一句请求，模型判不判得出「她说不上来」
// 全看它自己。这里把这件事变成一个数——服务端数出来的，每一轮都算，模型只负责用。
// 见 [[prompt-output-must-be-verifiable-2026-09-03]]。

// noAnswer 是「我答不上来」的说法。
//
// 只收**明说**的那些。把「短回答」也算进来的话，「食堂」「12 点半」这种真答案会
// 被当成卡住，印记就会对着一个正在好好回答的学生开始举例子。宁可漏，不可误：
// 漏了她下一轮还会再说一次，误了是印记不听她说话。
var noAnswer = []string{
	"不知道", "不清楚", "不太清楚", "没想过", "没想好", "想不出", "想不到",
	"说不上来", "答不上来", "不懂", "没概念", "没有想法", "没什么想法",
	"你说吧", "你决定", "你来定", "你帮我", "随便", "都行", "无所谓",
	"太难了", "答不出", "回答不了",
}

// askedForHelp 是她**直接开口要**支持的说法。
//
// 🚨 这一类和上面那一类要分开，因为它一次就该算数。走查里那个学生用自己的话求
// 了三次「给我几个选项选一下」，一次都没得到——她已经明说了她要什么，还要她先
// 卡够两轮才给，是把一条本来就该听见的话当成噪音。
var askedForHelp = []string{
	"举个例子", "举几个例子", "有什么例子", "有没有例子",
	"给我几个", "给几个", "有哪些选项", "有什么选项", "给我选项",
	"选一下", "帮我列", "能不能给我",
}

// 她只是在应声，没有答内容。这几个词单独成句才算——「对，中午十二点半最多」
// 是一句真答案，不能因为开头有个「对」就判成卡住。
var justNoise = map[string]bool{
	"": true, "嗯": true, "哦": true, "啊": true, "?": true, "？": true,
	"额": true, "呃": true, "。": true, "…": true, "......": true,
}

// StuckRun 数的是**结尾这一段**里，她连着有几轮答不上来。
//
// 只看结尾：她中间卡过一次、后来聊开了，那件事已经过去了，不该让印记在第十轮
// 还惦记着第二轮的那句「不知道」。碰到一轮真的答了内容，就归零。
func StuckRun(recent []Turn) int {
	n := 0
	for i := len(recent) - 1; i >= 0; i-- {
		t := recent[i]
		if t.Role != "student" {
			continue // 印记说的话不打断这个连续段，也不计数。
		}
		if !readsAsNoAnswer(t.Content) {
			break
		}
		n++
	}
	return n
}

// AskedForHelp 说的是：她最后那一句直接开口要例子或选项了。
//
// 只看最后一句，而且一次就算数。她已经明说了她要什么，让她先卡够两轮才给，是把
// 一条本来就该听见的话当成噪音。
func AskedForHelp(recent []Turn) bool {
	for i := len(recent) - 1; i >= 0; i-- {
		if recent[i].Role != "student" {
			continue
		}
		low := strings.ToLower(strings.TrimSpace(recent[i].Content))
		for _, w := range askedForHelp {
			if strings.Contains(low, w) {
				return true
			}
		}
		return false
	}
	return false
}

// readsAsNoAnswer 判断这一句是不是「我答不上来」。
func readsAsNoAnswer(s string) bool {
	t := strings.TrimSpace(s)
	if justNoise[t] {
		return true
	}
	// 一句话里既有「不知道」又有别的内容（「不知道，可能是中午人多」），那是
	// 一个带保留的真答案，不算卡住。所以对长句不判——20 个字是个宽松的线，
	// 「我真的不知道该给谁看这个网站」是 15 个字。
	if len([]rune(t)) > 20 {
		return false
	}
	low := strings.ToLower(t)
	for _, w := range noAnswer {
		if strings.Contains(low, w) {
			return true
		}
	}
	// 开口要例子也是「这一轮我答不上来」，所以它同样进这个连续段。
	for _, w := range askedForHelp {
		if strings.Contains(low, w) {
			return true
		}
	}
	return false
}

// stuckThreshold 是从第几次开始换打法。
//
// 2：第一次说不上来，换个问法再问一次是对的——问题可能只是问得太大。第二次还
// 答不上来，那就不是问法的事了，接着换侧面问下去只是在同一个坑里换角度挖。
const stuckThreshold = 2
