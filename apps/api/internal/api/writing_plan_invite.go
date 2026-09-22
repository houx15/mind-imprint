package api

import (
	"strings"

	"mindimprint/api/internal/store/sqlc"
)

// 请她去写的那一轮，话里不能还挂着一个问题。
//
// 🚨 **同一块屏幕不能既说「回答我这个问题」又说「你可以开始写了」。**
//
// 产品负责人 2026-09-12 报的第一条，带着截图：
//
//	「写作思路尚未梳理完整，AI 就判断已足以支撑一篇文章，判断依据不清晰。」
//
// 截图上那一刻，印记最后一句话是：
//
//	「中间差一步——是各地的学生基数撑起了学校，还是政策往各地分资源，或是别的？
//	  你自己有没有见过非省会城市办出好高中的例子？」
//
// 紧贴在它下面的，是绿框「计划已可开始写作 / 这份思路已经够撑起一篇」。
//
// 两句话都不假：结构上确实过了线（中心论点 2、分论点 2、她自己的材料 1，
// planLooksReady 数出来的）。但她读到的是**自相矛盾**：一边催她答题，一边说
// 想好了。系统提示词里本来就写着这一轮「不要再问问题——一个问号都不要有」，
// 问题出在那条判据是**服务端算的、模型不知道**：
// `ready` = 模型自己的 Ready **或** planLooksReady(live)。模型以为这是普通
// 的一轮，照常抛了个问题；服务端这边线刚好过了，于是邀请就贴在问号底下。
//
// 这是那条老规矩的又一例（[[prompt-twice-then-make-it-checkable]]）：
// 写在提示词里的「必须」，模型有权推翻；能在代码里验的，才是真的必须。
// 判据很便宜 —— 这一轮要请她走，话里就不许有问号。
//
// 判错的代价有界：重试一次，重试完照样用拿得到的那一份。

// writingPlanReplyAsks 判断这句话里还挂着问题。
//
// 中英文问号都认。故意只看问号，不去判断语气：一个不带问号的「你可以再想想
// 有没有例子」不会被拦下来，那是可以接受的漏网 —— 这条判据要挡的是她**以为
// 自己必须先回答**的那一类句子，而问号就是那个信号。
func writingPlanReplyAsks(reply string) bool {
	return strings.ContainsAny(reply, "？?")
}

// writingPlanShapeWith 把这一轮**还没落库**的节点算进形状里。
//
// 为什么不能等落库之后再判：回复是先写进 atom_message 再加节点的，等 live
// 有了那几个节点，那句带问号的话已经存进对话里了，改不动了。
//
// 丢掉的那几类（空文字、重复）和落库那边丢的是同一批，否则这里数出来的形状
// 会比真的多。kind 不合法的那一条在解析时就已经没了。
func writingPlanShapeWith(
	rows []sqlc.WritingOutline,
	add []writingPlanAdd,
) writingPlanShape {
	s := writingPlanShapeOf(rows)
	seen := rows
	for _, n := range add {
		if strings.TrimSpace(n.Text) == "" || outlineHasText(seen, n.Text) {
			continue
		}
		s.count(n.Kind, n.Source)
		// 同一轮里两个一模一样的节点，落库那边也只会留下第一个。
		seen = append(seen, sqlc.WritingOutline{Text: n.Text, Kind: n.Kind, Depth: writingKindDepth(n.Kind)})
	}
	return s
}
