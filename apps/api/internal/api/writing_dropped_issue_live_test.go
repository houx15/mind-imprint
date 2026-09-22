package api

// writing_dropped_issue_live_test.go —— 「总评说了有问题，底下一条都点不动」
// 那一类，到底是谁丢的。
//
//	./.tooling/live-api.sh TestLiveDroppedIssue
//
// # 这条是从哪来的
//
// 2026-09-21 真学生走查第三趟，她那一段拿回来的是：
//
//	verdict = polish
//	总评   = 「……要紧的问题只有一处——读者读到这里会把它和上一段当成同一件事。」
//	points = 只有一条 good（夸她「十几秒一个、马上有下一个」那句在追为什么）
//
// 也就是说：印记**看出来了**分论点 2 和分论点 1 是同一件事（那正是讲义
// 「分得开」那一条），然后把它写进了唯一一个**没有人验过**的字段里，
// 底下什么都没留给她。她读到一句「你这儿不太行」，点不动、追不到原文、
// 也不知道下一步做什么。
//
// 走查看得见结果，看不见成因。成因只有两种，而它们的修法**相反**：
//
//	A. 模型根本没给 issue      → 闸门该拦住它、再问一次（闸门在，但见 B）
//	B. 模型给了，服务端丢掉了  → 闸门站在筛子上游，一次都看不见
//
// 所以这条测试不断言好坏，它**把两边都打出来**：模型原样回了几条、
// 校验之后还剩几条、掉下去的每一条是被哪条规矩丢的。
//
// 🚨 拿真段落跑。用一段造出来的「缺分析句」范文，问的就又是
// teaching-r4 那个已经写在题面上的问题了。

import (
	"context"
	"testing"
	"time"

	"mindimprint/api/internal/gateway"
	"mindimprint/api/internal/store/sqlc"
)

// 走查里真正拿到那份意见的那一段，一字不改。
//
// 它的毛病是**跨段的**：单看这一段挑不出大错（观点句、例子、机制都在），
// 错在它和上一段说的是同一件事。跨段的毛病要怎么锚在这一段的字上，
// 正是这条测试要看清楚的地方。
const liveDroppedIssueParagraph = `其次，短视频让我们越来越不喜欢看长的东西了。以前我还能看完一本小说，现在看两页就想去刷手机。很多人都说自己现在没有耐心了。短视频一个只有十几秒，看完一个马上就有下一个，我们已经习惯了这种快节奏。`

// 她这一篇的其余部分 —— 上一段就在里面，印记正是靠它看出「撞车」的。
const liveDroppedIssuePiece = `在当今这个科技飞速发展的时代，短视频已经成为了我们生活中不可缺少的一部分。打开手机，各种各样的短视频扑面而来，让人眼花缭乱。那么，短视频到底有没有让我们变笨呢？我认为，短视频正在悄悄地让我们变笨。

首先，短视频把我们的注意力切得很碎。我记得有一次我想看完一部两个小时的电影，结果中间我摸了五次手机，每次都是刷了十几分钟短视频才放下。我们班上的同学也是这样，现在看课文都要老师划了重点才看得下去，一篇长一点的文章根本读不完。所以短视频真的把我们的注意力切得很碎。

其次，短视频让我们越来越不喜欢看长的东西了。以前我还能看完一本小说，现在看两页就想去刷手机。很多人都说自己现在没有耐心了。短视频一个只有十几秒，看完一个马上就有下一个，我们已经习惯了这种快节奏。`

func TestLiveDroppedIssue(t *testing.T) {
	prov, resolved := liveReview(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	wr := sqlc.Writing{Title: "短视频有没有让我们变笨？", Lang: "zh"}

	// 跑三趟。这一类是**时好时坏**的（走查三趟里两趟撞上，落在不同的段），
	// 一趟绿说明不了任何事。
	for i := 1; i <= 3; i++ {
		res, err := gateway.Collect(ctx, prov, resolved, gateway.ChatRequest{
			Messages: []gateway.ChatMessage{
				{Role: gateway.RoleSystem, Content: buildWritingCommentSystem(
					"zh", writingBlockCommentMaxIssues, writingKindPoint, helpAsk, genreArgument)},
				{Role: gateway.RoleUser, Content: buildWritingCommentPrompt(
					wr, "她写的这一段", liveDroppedIssueParagraph, liveDroppedIssuePiece, genreArgument)},
			},
		})
		if err != nil {
			t.Fatalf("第 %d 趟 provider: %v", i, err)
		}
		parsed, ok := parseWritingComment(res.Text)
		if !ok {
			t.Fatalf("第 %d 趟解析不了：\n%s", i, res.Text)
		}

		kept, drops := validateCommentPointsVerbose(
			parsed.Points, liveDroppedIssueParagraph, "zh", writingBlockCommentMaxIssues)

		t.Logf("──────── 第 %d 趟 ────────", i)
		t.Logf("verdict = %s", parsed.Verdict)
		t.Logf("总评   = %s", parsed.Summary)
		t.Logf("模型原样给了 %d 条；校验之后剩 %d 条", len(parsed.Points), len(kept))
		for _, p := range parsed.Points {
			t.Logf("  模型给的 [%s] symptom=%q action=%q\n      说：%s\n      引：%s",
				p.Kind, p.Symptom, p.Action, p.Text, p.Quote)
		}
		for _, d := range drops {
			t.Logf("  🚨 丢掉一条 [%s]，因为 %s；它引的是：%s", d.Kind, d.Reason, d.Quote)
		}

		rawIssue := writingHasIssue(parsed.Points)
		keptIssue := writingHasIssue(kept)
		switch {
		case !rawIssue && parsed.Verdict != writingVerdictPass:
			t.Logf("  ⇒ 成因 A：模型自己就没给 issue。闸门该拦下这一份。")
		case rawIssue && !keptIssue:
			t.Logf("  ⇒ 成因 B：模型给了 issue，**是服务端丢的**。闸门查的是模型原样回的那份，站在筛子上游，看不见这一类。")
		case !keptIssue && parsed.Verdict != writingVerdictPass:
			t.Logf("  ⇒ verdict=%s 却没有一条她能照着改的意见。", parsed.Verdict)
		default:
			t.Logf("  ⇒ 这一趟是好的。")
		}
	}
}
