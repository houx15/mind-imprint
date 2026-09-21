package api

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/google/uuid"
	"mindimprint/api/internal/gateway"
)

// 总评里不许说「缺少什么」。
//
// 🚨 **一条意见的分量，来自它指着的那句话；总评没有那句话。**
//
// 这一块面板上，每条 point 都得带一句她原文里逐字存在的 quote，对不上的整条
// 丢掉（validateCommentPoints）。只有 summary 是一句自由的话，谁也没验过它 ——
// 而它以满字重挂在最上面，是她第一眼读到的东西。
//
// 2026-09-12 第二十八轮线上走查，中文那一路的两次通篇审阅，总评分别是：
//
//	「观察很具体，但文章停在了『告诉大家有浪费』这一步，缺少对素材的分析和解释。」
//	「你收集了很多具体事例，但全文缺少一个核心问题来统领这些材料。」
//
// 第二句是**假的**。她那一版正文（818 字，服务端 source_text 存着的就是它）
// 第三段结尾写着：
//
//	「到底是什么让我们倒掉一整盘饭的时候，连眼皮都不会抬一下？」
//
// 稿子整篇没被截断、走的又是旗舰 review 档 —— 它读到了，然后照样说没有。
// 她的原话：
//
//	「印记说『缺少一个核心问题来统领这些材料』，但我第一段开头就加了那个问句啊，
//	  不知道它还要我怎么统领。」
//	「我已经加了问句了但它还是标红说没有，有点烦。」
//
// 为什么偏偏是总评错：写总评的时候，模型是在概括自己刚写的那几条，不是回头
// 重读她的正文。指着某句话说话要求它去找那句话；说「缺少 X」不要求它去找 X。
//
// 所以这条不靠再写一段提示词（那是 [[prompt-twice-then-make-it-checkable]]
// 记下的老路），而是定成一条**输出类型上的**规矩：总评说这篇稿子现在在哪儿，
// 「缺什么」交给下面那几条 —— 那几条带着原话和一个她现在就能做的动作，
// 而且验得了。判据因此只用认「缺失」这一类词，不用去判断它说得对不对。
//
// 判错的代价是有界的：一次重试，重试完照样把结果给她（同 firstGhostQuote 那条
// 路）。所以宁可多抓几个 —— 漏掉一句假的「你缺 X」比多跑一次模型贵得多。
var writingSummaryAbsenceMarkers = []string{
	// 中文。「缺」单独留着是故意的：缺少/缺乏/缺失/缺点 都该被这条盖住。
	"缺", "不足", "尚未", "还没", "没能", "未能",
	// 🚨 2026-09-21 线上走查补的。真模型绕过了上面那几个词：
	//   「这两段各写了一边的事，但通篇**找不到**一句是你自己的判断」
	// 说的是同一件事，一个标记都没踩上。
	// 「没有一句」「没有一个」这样带量词的写法收进来，光秃秃的「没有」不收 ——
	// 「这一段没有问题」是句好话，收了它每一轮都要重试。
	"找不到", "看不到", "没有一句", "没有一个", "一句都没有", "一个都没有",
	// 英文。全部小写比对。
	"lack", "missing", "absent", "fails to", "fail to", "without a", "no clear",
}

// summaryClaimsAbsence 判断这句总评是不是在说她「缺」了什么。
//
// 只做子串匹配，不做语义判断 —— 这条判据要的是便宜和稳定，不是准。
func summaryClaimsAbsence(summary string) string {
	lowered := strings.ToLower(summary)
	for _, m := range writingSummaryAbsenceMarkers {
		if strings.Contains(lowered, m) {
			return m
		}
	}
	return ""
}

// writingSummaryAbsenceNudge 是重试那一轮加进去的纠正话。
//
// 照着 writingGuideBracketNudge 的做法：把犯的那一处指出来，而不是把整条规矩
// 再念一遍 —— 念规矩它上一轮已经读过了。
const writingSummaryAbsenceNudge = `刚才那份 summary 里说了她「缺」什么（出现了「%s」）。

summary 只说这篇稿子现在站在哪儿，不说少了什么。少了什么由下面那几条 point 去说——
那几条必须指着她原文里的一句话，并给出她接下来要做的那个动作。

重新输出一次完整的 JSON，只改 summary 这一个字段，points 原样保留。`

// collectWritingComment 跑一轮通篇/单段审阅，并在总评说了「缺什么」的时候
// **重试一次**。
//
// 两个调用点（单段的 writing_comment.go、通篇的 writing_compose.go）形状一样，
// 所以这一段只写一份 —— 上一轮那个「成稿这一支提前 return，把接地规矩漏掉了」
// 的教训就是复制两份代码换来的。
//
// 返回的是拿得到的那一份：重试完总评还在说「缺」，照样把结果给她。一句不好的
// 总评下面仍然挂着几条验过的、指着她原话的意见；为这个扣下整轮，她什么都拿不到。
// 这一点和「JSON 解析不了」不同 —— 那个是真的没有东西可给。
// deliver 把模型原样回的 points 变成**她最后真的会看到的**那几条。
//
// 🚨 这个参数是 2026-09-21 真学生走查逼出来的。原来这里查的是
// `parsed.Points` —— 模型原样回的那一份，而她看到的是它经过
// `validateCommentPoints` 加两道减法之后剩下的那一份。闸门站在筛子的**上游**，
// 于是「模型给了一条好意见、被服务端丢掉了」这一整类，它一次都看不见：
// 实测三趟三趟如此（模型把引文写进 text、quote 那格空着 ⇒ no_quote ⇒ 整条丢掉）。
//
// 判据要落在**交付物**上，不是落在中间产物上。
type writingCommentDeliver func([]CommentPoint) []CommentPoint

func (a *API) collectWritingComment(
	ctx context.Context,
	userID, atomID uuid.UUID,
	purpose string,
	resolved gateway.Resolved,
	system, user string,
	deliver writingCommentDeliver,
	logArgs ...any,
) (writingCommentResult, bool) {
	msgs := []gateway.ChatMessage{
		{Role: gateway.RoleSystem, Content: system},
		{Role: gateway.RoleUser, Content: user},
	}

	res, cerr := gateway.Collect(ctx, a.d.Provider, resolved, gateway.ChatRequest{Messages: msgs})
	// 打到 provider 就已经花钱了，无论回复拿它怎么办 —— 先记账，同这一族
	// 每一个 lite generate 端点。
	a.recordLiteLLMCall(ctx, userID, atomID, purpose, resolved, res.Usage)
	if cerr != nil {
		slog.Warn("writing comment: provider call failed", append(logArgs, "err", cerr)...)
		return writingCommentResult{}, false
	}
	parsed, ok := parseWritingComment(res.Text)
	if !ok {
		slog.Warn("writing comment: reply unparseable or empty summary", logArgs...)
		return writingCommentResult{}, false
	}

	// 🚨 查她真的会看到的那几条，不是模型原样回的那份。见 writingCommentDeliver。
	delivered := parsed.Points
	if deliver != nil {
		delivered = deliver(parsed.Points)
	}

	marker, nudge := writingCommentProblem(parsed, delivered)
	if marker == "" {
		return parsed, true
	}

	// 重试那一轮把它自己上一份回复也带上，否则「原样保留」无从谈起。
	slog.Info("writing comment: retrying once",
		append(logArgs, "marker", marker, "summary", parsed.Summary)...)
	retry := append(msgs,
		gateway.ChatMessage{Role: gateway.RoleAssistant, Content: res.Text},
		gateway.ChatMessage{Role: gateway.RoleUser, Content: nudge},
	)
	res2, cerr2 := gateway.Collect(ctx, a.d.Provider, resolved, gateway.ChatRequest{Messages: retry})
	a.recordLiteLLMCall(ctx, userID, atomID, purpose, resolved, res2.Usage)
	if cerr2 != nil {
		slog.Warn("writing comment: absence retry call failed", append(logArgs, "err", cerr2)...)
		return parsed, true
	}
	parsed2, ok2 := parseWritingComment(res2.Text)
	if !ok2 {
		slog.Warn("writing comment: absence retry unparseable", logArgs...)
		return parsed, true
	}
	delivered2 := parsed2.Points
	if deliver != nil {
		delivered2 = deliver(parsed2.Points)
	}
	// 两次都没过同一道闸。用第二份 —— 它至少是照着这条规矩又想了一遍的。
	// 「说了有问题却没有一条她能照着改的」那一档，调用方会把 verdict 收回
	// 到 pass：与其让她对着一段没有任何可做之事的卡片读到「待打磨」，
	// 不如不说这句话。
	if m2, _ := writingCommentProblem(parsed2, delivered2); m2 != "" {
		slog.Warn("writing comment: still not clean after the retry",
			append(logArgs, "marker", m2, "verdict", parsed2.Verdict, "summary", parsed2.Summary,
				"raw_points", len(parsed2.Points), "delivered", len(delivered2))...)
	}
	return parsed2, true
}

// writingVerdictNoPointMarker 是第二种触发的记号。不是她正文里的词，
// 只是给日志和分支用的一个标签。
const writingVerdictNoPointMarker = "<verdict-without-point>"

// writingNoPointNudge —— 说了这篇有问题，却一条都没指出来的那一轮。
//
// 照着 writingSummaryAbsenceNudge 的做法：指出犯的是哪一处，不把整条规矩再念一遍。
// 🚨 这段话原来写的是「points 却是空的」—— 而 points **往往不是空的**：
// 里面常常有一条 good。模型照着这句话回头看自己那一份，发现 points 有东西，
// 于是认为这条提醒不适用，原样又回一遍。
//
// **提醒里说的那件事必须是真发生的那件事**，否则它只是一句模型对不上号的话。
// 现在说的是「没有一条 issue」，并且把 issue 活下来要满足的条件列清楚 ——
// 实测里被丢掉的那些，十有八九是漏了其中一条。
const writingNoPointNudge = `刚才那一份里，verdict 不是 pass，但 points 里**没有一条 kind 是 issue**
（有 good 也不算——夸奖不是她能照着改的东西）。

你说了这篇还有要改的地方，却没有给出一条她能动手的意见。她看到的会是一句
挂在最上面、点不动也追不到原文的话。

重新输出一次完整的 JSON：
- 真的有要改的地方 → 至少给一条 kind 为 issue 的，并且四样都要齐，缺一样这条就会被丢掉：
  · quote：**单独写在 quote 这个字段里**，逐字照抄她原文里的一句（写在 text 里不算）；
  · symptom：只能用给定清单里的 id；
  · text：说清楚是什么问题；
  · action：她现在就能做的那一个动作。
- 其实没有 → 把 verdict 改成 pass，summary 改成说这一篇现在站在哪儿。`

// writingCommentProblem —— 这一份意见有没有哪一道闸没过，以及该怎么跟它说。
//
// 三道，按「对她的伤害」从大到小排；命中第一道就不往下看了，因为重问那一轮
// 只带一条提醒 —— 一次说三件事，模型哪件都做不好
// （[[pbl-refeed-one-produce-slot-2026-09-05]] 是同一个道理）。
//
//  1. **说了这篇有问题，却没有一条她能照着改的。** 最伤人的一种：
//     她被告知这儿不对，却没有一个字可以动手。
//  2. **总评说她「缺」什么。** 总评是唯一没人验过的字段，
//     一句没有东西撑着的断言。
//  3. **话写成了对她的判决。** 同事的原话是「一直在挑衅我」。
//
// 判据全部落在**她真的会看到的那一份**上（delivered），不是模型原样回的那份。
func writingCommentProblem(parsed writingCommentResult, delivered []CommentPoint) (marker, nudge string) {
	if parsed.Verdict != writingVerdictPass && !writingHasIssue(delivered) {
		return writingVerdictNoPointMarker, writingNoPointNudge
	}
	if m := summaryClaimsAbsence(parsed.Summary); m != "" {
		return m, fmt.Sprintf(writingSummaryAbsenceNudge, m)
	}
	if p := writingHostileTone(parsed); p != "" {
		return p, fmt.Sprintf(writingHostileToneNudge, p)
	}
	return "", ""
}
