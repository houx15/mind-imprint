package api

import (
	"regexp"
	"strings"
)

// writing_continuation.go —— 读后续写的那道硬门槛。
//
// # 为什么这不是一段提示词就够了
//
// 源材料 §4.1 是一条**判前输入检查**：
//
//	必须拿到前文全文 + 两个段首句 + 学生续写三者齐备才可出终评。
//	缺前文/段首句时：只能评语言与句子层面，明确提示「内容（融洽度/逻辑）
//	无法判档，请补原文」。
//
// 光在提示词里写「没有前文就别判内容」，是把一条可以在服务端算出来的事实
// 交给模型每轮自己判断。服务端数得出来的事实就别让模型数
// （memory: hardcoded-thresholds-vs-user-set-scale-2026-09-12 同一条道理）。
// 这里把它算好，再把结论写进提示词。
//
// # 前文住在哪儿
//
// 住在 assigned_prompt 里 —— 一道读后续写题印出来就是「一段前文 + 两个段首句」，
// 老师把题面贴进来，那一整块就是它。没有为此加一列：题面本来就是这个形状，
// 而多一列意味着一次迁移加一处前端输入，换来的只是把同一段文字挪个地方。

// continuationInputs 是判内容之前要齐备的那三样里，题目该给的两样。
type continuationInputs struct {
	// Source 是前文。空串 = 没给。
	Source string
	// Openers 是两个段首句，按出现顺序。不足两句就是没给齐。
	Openers []string
}

// Ready 说这道题的依据够不够判内容（情节、衔接、伏笔）。
//
// 🚨 「够」的判据是**两个段首句都在，而且前文有实际长度**。只有段首句没有
// 前文，照样判不了伏笔回收和人物性格是否一致。
func (c continuationInputs) Ready() bool {
	return len(c.Openers) >= 2 && len([]rune(c.Source)) >= continuationSourceMinRunes
}

// 前文短于这个长度就当它没给。
//
// 一道真题的前文是三四百词的一段故事；几十个字的东西是题目说明，不是前文。
// 取 120 个 rune：英文按 rune 算约二十来个词，比任何一句题目说明都长，
// 又远低于最短的真实前文。
const continuationSourceMinRunes = 120

// 段首句的惯例写法。语料里 43 道读后续写全是这个形状
// （Paragraph 1: … / Paragraph 2: …），中文卷面上偶尔写成「第一段：」。
var continuationOpenerRe = regexp.MustCompile(
	`(?i)(?:paragraph\s*([12])\s*[::]|第\s*([一二12])\s*段\s*[：:])\s*(.*)`)

// parseContinuationInputs 从题面里挑出前文和两个段首句。
//
// 🚨 挑不出来就返回空的那一份，**绝不猜**。猜错的代价是对着一段不存在的前文
// 去判「伏笔没回收」，而学生根本没拿到过那份前文。
func parseContinuationInputs(assigned string) continuationInputs {
	assigned = strings.ReplaceAll(assigned, "\r\n", "\n")
	if strings.TrimSpace(assigned) == "" {
		return continuationInputs{}
	}

	var out continuationInputs
	var body []string
	seen := map[string]bool{}
	for _, line := range strings.Split(assigned, "\n") {
		trimmed := strings.TrimSpace(line)
		if m := continuationOpenerRe.FindStringSubmatch(trimmed); m != nil {
			which := m[1] + m[2] // 只有一组会非空
			sentence := strings.TrimSpace(m[3])
			// 同一个段号出现两次只认第一次 —— 题面里常把段首句再抄一遍到
			// 答题位上，两份是同一句。
			if sentence != "" && !seen[which] {
				seen[which] = true
				out.Openers = append(out.Openers, sentence)
			}
			continue
		}
		body = append(body, line)
	}
	// 段首句之外剩下的就是前文（外加题目说明，它们一起构成「她手上有什么」）。
	out.Source = strings.TrimSpace(strings.Join(body, "\n"))
	return out
}

// continuationGateNote 是要写进提示词的那一句 —— 依据不齐时说清判不了什么。
//
// 齐备时返回空串：这一档没有多余的话要说，前文本身已经在上下文里了。
func continuationGateNote(c continuationInputs) string {
	if c.Ready() {
		return ""
	}
	var missing []string
	if len([]rune(c.Source)) < continuationSourceMinRunes {
		missing = append(missing, "前文")
	}
	if len(c.Openers) < 2 {
		missing = append(missing, "两个段首句")
	}
	return "【这道题的依据还不齐】现在手上没有" + strings.Join(missing, "和") +
		"。不要对情节、衔接、伏笔回收、人物是否走样下判断 —— 这几样离开前文没有依据。" +
		"这一轮只谈语言和句子，并且明确告诉学生：内容和衔接现在判不了，" +
		"把原文和两个段首句补上再看。"
}
