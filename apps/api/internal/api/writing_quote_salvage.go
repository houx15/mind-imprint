package api

import (
	"strings"

	"mindimprint/api/internal/quotematch"
)

// 引文被写进了 text 里，quote 那一格空着 —— 别为这个把整条意见扔掉。
//
// # 实测出来的，不是想出来的
//
// 2026-09-21，`TestLiveDroppedIssue` 连跑三趟，**三趟同一个死法**：
// 模型给出一条很好的 issue —— 认出这一段缺分析句、给了因果分析法的具体动作、
// 第三趟还顺带看出它和上一段的分论点撞车了 —— 然后把她那句话写进了
// `text` 里用「」括着，`quote` 那一格是空的：
//
//	text : 「以前我还能看完一本小说，现在看两页就想去刷手机」这个例子摆完就直接跳到…
//	quote: ""
//
// `validateCommentPoints` 看见 quote 是空的，整条丢掉（`no_quote`）。
// 她拿回来的于是只剩一句总评和一条夸她的话 —— 被告知这儿不对，
// 却没有一个字可以照着改。这就是同事说的「一直在挑衅我」里最伤人的那一种。
//
// # 为什么捞是对的，而放宽比对是错的
//
// 那条**载重**的保证是「锚点逐字是她写的」（validateCommentPoints 的文件头），
// 这里一个字都不动它：捞出来的候选仍然要**逐字**出现在她这一段里，
// 对不上就照旧丢掉。变的只是「去哪儿找那句话」——
// 模型把它放错了格子，不等于它编了一句话。
//
// 同 `salvageWritingPlanReply` 的判断：这一轮的钱已经花掉了，
// 而坏掉的地方不是内容本身，是它被放在哪一格。
//
// 🚨 **绝不放宽比对**（[[detector-must-target-the-real-failure]] 里那条
// 「幻引第二次仍不逐字就去掉引号，而不是丢掉整轮 —— 但绝不放宽比对」）。
// 捞不到逐字的候选，这条意见照旧丢掉。

// 中英两套引号。把她那句话括起来的可能是任何一种。
var quoteBrackets = [][2]string{
	{"「", "」"},
	{"『", "』"},
	{"“", "”"},
	{"\"", "\""},
}

// salvageQuoteFromText 从一条意见的正文里，把她那句话捞出来。
//
// 取**最长的那个逐字候选**：一段话里常常括着好几处，越长的锚点越具体，
// 划在她屏幕上也越说明问题。捞不到就返回空字符串，调用方照旧丢掉这条。
func salvageQuoteFromText(text, source string) string {
	best := ""
	for _, br := range quoteBrackets {
		for _, cand := range bracketedSpans(text, br[0], br[1]) {
			span, ok := verbatimSpan(source, cand)
			if !ok {
				continue
			}
			if len([]rune(span)) > len([]rune(best)) {
				best = span
			}
		}
	}
	return best
}

// verbatimSpan 判这个候选是不是**逐字**出现在她写的东西里。
//
// 和 validateCommentPoints 里那一段同一条规矩：先直接比，只差标点空格大小写的
// 交给 quotematch.Locate 从原文里取回那一段真的字。两条都不过就是不过。
func verbatimSpan(source, cand string) (string, bool) {
	cand = strings.TrimSpace(cand)
	// 太短的锚点划出来没有意义，而且容易撞上一个到处都有的词。
	if len([]rune(cand)) < 4 {
		return "", false
	}
	if strings.Contains(source, cand) {
		return cand, true
	}
	return quotematch.Locate(source, cand)
}

// bracketedSpans 把 text 里所有 open…close 之间的内容取出来。
//
// 不嵌套、不跨越：取最近的一个 close。同一种括号成对出现多次是常态
// （模型一条意见里常引两处）。
func bracketedSpans(text, open, close string) []string {
	var out []string
	rest := text
	for {
		i := strings.Index(rest, open)
		if i < 0 {
			return out
		}
		rest = rest[i+len(open):]
		j := strings.Index(rest, close)
		if j < 0 {
			return out
		}
		out = append(out, rest[:j])
		rest = rest[j+len(close):]
	}
}
