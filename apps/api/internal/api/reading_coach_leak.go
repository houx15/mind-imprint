package api

import (
	"regexp"
	"strings"
)

// reading_coach_leak.go —— 两种「说漏嘴」：把协议词写进她读到的话里，
// 以及当着她的面把她叫成「她」。
//
// 两条都是产品负责人 2026-09-17 第三轮走查里逐字指出来的，而两条的成因是同一个：
// **prompt 里写给模型看的那些字，模型有时会顺手抄进回复。**

// coachProtocolLeaks —— 协议词漏进正文的那几种写法。
//
// 🚨 截图里她读到的最后一句是：
//
//	这一步做完。advance给done。
//
// 「advance」是 JSON 里那个键，「done」是它的取值 —— 两个都是写给服务端看的，
// 她屏幕上不该出现任何一个。她读到的是一句不知道在说什么的话，而这一句恰好
// 出现在一步结束、她最需要知道接下来干什么的时候。
//
// 判据收得很紧：**advance 和它的取值要一起出现**。单独一个 "advance" 放过 ——
// 英文文章里真的会有这个词（"advance the argument"），而回复引原文是正常的。
var coachProtocolLeaks = []string{
	"advance给done", "advance给skipped", "advance 给 done", "advance 给 skipped",
	"advance:done", "advance: done", "advance=done", "advance：done",
	"advance:skipped", "advance: skipped", "advance：skipped",
	"advance设为", "advance 设为", "advance填done", "advance 填 done",
	`"advance"`, "focusBlock", "summon_card",
}

// bareStatusAfterHan / bareStatusAfterStop —— 取值单独漏出来、没带 advance 的那种。
//
// 2026-09-17 入口走查（星图那篇果蝇脑图）录到：「…第7段接着讲这次怎么做，done。」
// 上面那张表要求 advance 和取值一起出现，这一句一个都没对上。
//
// 判据：done / skipped 紧跟在**汉字或中文收尾标点**后面，后面是中文句末标点或
// 结尾。引英文原文时（"the work is done."）它前面是英文字母，不算。
var (
	bareStatusAfterHan  = regexp.MustCompile(`(\p{Han}|[）」』”])\s*[，,、]?\s*(?i:done|skipped)\s*([。！？；]|$)`)
	bareStatusAfterStop = regexp.MustCompile(`([。！？])\s*(?i:done|skipped)\s*[。！？]?`)
)

// firstProtocolLeak 返回回复里第一处协议词，没有就返回 ""。
func firstProtocolLeak(reply string) string {
	if m := bareStatusAfterHan.FindString(reply); m != "" {
		return m
	}
	if m := bareStatusAfterStop.FindString(reply); m != "" {
		return m
	}
	return tableProtocolLeak(reply)
}

// tableProtocolLeak —— 只查 coachProtocolLeaks 那张表。
func tableProtocolLeak(reply string) string {
	lower := strings.ToLower(reply)
	for _, w := range coachProtocolLeaks {
		if strings.Contains(lower, strings.ToLower(w)) {
			return w
		}
	}
	return ""
}

// stripProtocolLeak 把带协议词的那**一句**从回复里拿掉。
//
// 🚨 这不是在改写它的话（[[ai-errors-must-surface-never-fake]] 禁的是编一句
// 听着像样的答案顶上去）。被拿掉的那一句是**我们自己的脚手架**漏出来的字，
// 它不承载任何教学内容 —— 「这一步做完。advance给done。」拿掉后半句，前半句
// 一个字没变，而她读到的话第一次是完整的。
//
// 先重来一次（调用点），两次都漏才走这里。
func stripProtocolLeak(reply string) string {
	if firstProtocolLeak(reply) == "" {
		return reply
	}
	// 表里那几种（「advance给done」）整句拿掉，先做；剩下单独漏出来的取值只拿掉
	// 那个词 —— 它前面那半句是真话，整句删掉会丢内容。顺序反过来，「advance给
	// done。」会先变成「advance给。」，表里就再也认不出它了。
	if tableProtocolLeak(reply) == "" {
		return stripBareStatus(reply)
	}
	// 按行拆，再按中文句号拆 —— 漏出来的那一句总是自成一句。
	lines := strings.Split(reply, "\n")
	outLines := make([]string, 0, len(lines))
	for _, line := range lines {
		kept := make([]string, 0, 4)
		for _, sent := range splitCJKSentences(line) {
			if tableProtocolLeak(sent) == "" {
				kept = append(kept, sent)
			}
		}
		joined := strings.TrimSpace(strings.Join(kept, ""))
		// 整行只有那一句协议词的话，这一行就没了 —— 但不要留下一个空行。
		if joined == "" && strings.TrimSpace(line) != "" {
			continue
		}
		outLines = append(outLines, joined)
	}
	out := strings.TrimSpace(strings.Join(outLines, "\n"))
	if out == "" {
		// 整条回复只有协议词。这时候还给原话 —— 空回复比一句怪话更糟，
		// 而且调用点那一侧有「这一轮什么都没请她做」在守。
		return reply
	}
	return stripBareStatus(out)
}

// stripBareStatus 只拿掉单独漏出来的 done / skipped 那个词。
func stripBareStatus(s string) string {
	s = bareStatusAfterHan.ReplaceAllString(s, "${1}${2}")
	return bareStatusAfterStop.ReplaceAllString(s, "${1}")
}

// splitCJKSentences 按中文句末标点切句，标点留在句子里。
func splitCJKSentences(s string) []string {
	var out []string
	start := 0
	r := []rune(s)
	for i, c := range r {
		switch c {
		case '。', '！', '？', '；':
			out = append(out, string(r[start:i+1]))
			start = i + 1
		}
	}
	if start < len(r) {
		out = append(out, string(r[start:]))
	}
	return out
}

// replyCallsHerShe —— 这一轮当着她的面把她叫成了「她」。
//
// 🚨 产品负责人 2026-09-17 逐字：「and the AI often says 她, but we are talking.」
//
// 成因在我们这一侧：整条 system prompt 都在用「她」描述这个学生（「她读完一部分
// 答一次」「先说她哪里选得准」），而模型顺着上文往下写，就把第三人称带进了
// 回复。这不是模型在犯错，是我们给的那份说明书就是那么写的 ——
// prompt 已经补上一句「一律用你」，这里是那句话没被遵守时的保证。
//
// # 判据为什么这样收
//
// 「她」在回复里**不一定**是指学生：一篇讲某个女性的文章，讲解里当然会出现
// 「她」。所以只在**文章里一个「她」都没有**的时候才判 —— 那时候回复里的
// 「她」除了指学生没有别的可能。英文文章一律满足这个条件，而截图里那一篇
// 正是英文的。
//
// 判错的方向和别处一致：宁可放过，不可误伤。
func replyCallsHerShe(reply string, blocks []Block) bool {
	if !strings.Contains(reply, "她") {
		return false
	}
	for _, b := range blocks {
		if strings.Contains(b.Text, "她") {
			return false
		}
	}
	return true
}
