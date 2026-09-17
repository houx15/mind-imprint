package api

// reading_coach_english_live_test.go —— 把「英文文章的第一轮」送去见一次真模型。
//
//	set -a; . .deploy-local/env.prod; set +a
//	LIVE_LLM=1 go test ./internal/api -run TestLiveEnglishCoach -v -count=1
//
// # 为什么单独有这个文件
//
// 2026-09-08 线上：产品负责人从探索地图点「现在读」，落进阅读室的是一篇**英文**
// 文章（标题是中文的新闻标题），按 en-argument 排好了读法，她按「开始」——
//
//	POST /api/v1/readings/fee62b85…/coach  502
//	WARN reading coach: reply unparseable, retrying once   stop_reason="stop"
//	WARN reading coach: reply unparseable after retry      stop_reason="stop"
//
// 两次都是**正常收尾**（stop），所以不是 lensdone 那个文件测出来的截断。
// 而这个包里所有既有的实测（TestLiveLensDoneReplyParses 等）用的都是**中文**
// 文章。
//
// # 实测结论（dashscope/deepseek-v4-pro，各 6 次）
//
//	改之前                       3/6 不能用
//	加 response_format=json_object  4/6 不能用（没有变好，已撤回）
//	加 salvageCoachReply         0/6 不能用
//
// 断点每一次都在同一个地方：reply 已经完整，`card` 写到一半没了，而 provider
// 报的是 finish_reason "stop"、completion 只有一两百 token（上限 16000）。
// 第一轮尤其容易撞上，因为第一轮总会带一张卡片，而卡片的 options 要逐字引用
// 原文 —— 英文原句是同义中文句的三到五倍 token。中文文章上同一个毛病是 1/6。
//
// 断言的是学生感觉得到的那条线：生产的 prompt + 生产的 parser，回复必须能用。

import (
	"context"
	"strings"
	"testing"
	"time"

	"mindimprint/api/internal/gateway"
	"mindimprint/api/internal/store/sqlc"
)

// liveEnglishBlocks 是一篇英文议论文，和线上那一篇同一类：有立场、有数字、
// 有引语。**引语是关键** —— 它是模型最想原样抄进 reply 里的东西。
func liveEnglishBlocks() []Block {
	return SplitBlocks(strings.Join([]string{
		"Is skipping breakfast a moral failure? The question sounds absurd until you notice how often it is asked in that register.",
		`Wellness writers rarely say "you will feel tired." They say breakfast is "the most important meal of the day," a phrase coined in a 1917 magazine column and repeated ever since without a citation.`,
		"The 1917 column was not a study. It was advice, and the advice was written by a man who sold cereal. That origin is not a refutation, but it is worth knowing before the phrase is used as evidence.",
		"The strongest modern claim is that breakfast eaters weigh less. Observational cohorts do show that association, sometimes at eight to ten percent lower BMI.",
		"But people who eat breakfast also sleep more, smoke less, and earn more. The association survives adjustment for some of those; it does not survive all of them.",
		`When researchers ran the randomised version, the effect largely disappeared. A 2019 meta-analysis of thirteen trials concluded that "the addition of breakfast might not be a good strategy for weight loss."`,
		"So why does the moral framing persist? Partly because a habit that correlates with wealth is easy to mistake for a cause of virtue.",
		"There is a second reason, and it is the one this essay is about. Nutrition advice is one of the few remaining places where a stranger may tell you how to run your morning and be thanked for it.",
		"The language gives it away. We speak of skipping breakfast, as if a duty had been dodged, rather than of not eating breakfast, which is merely a description.",
		"None of this shows that breakfast is bad for you. It shows that the confidence of the claim outruns the evidence for it, and that the excess confidence is doing moral work.",
		"The honest version is duller. For most adults, when you eat matters less than what and how much, and the studies that would settle the rest have not been run.",
		"That is an unsatisfying place to stop. It is also where the evidence stops, and a writer who goes further is no longer reporting.",
	}, "\n\n"))
}

// liveEnglishTasks 就是线上那一篇的读法（en-argument，全部 pending，一条消息
// 都还没有）—— 她按下「开始」的那一刻。
func liveEnglishTasks() []sqlc.ReadingTask {
	return []sqlc.ReadingTask{
		{Position: 0, Label: "通读全文", Kind: "read", Status: "pending", Detail: "先找出作者站哪一边。"},
		{Position: 1, Label: "精读重点段落第6段", Kind: "focus_block", Status: "pending", BlockID: "b6"},
		{Position: 2, Label: "深入思考", Kind: "lens", Status: "pending"},
		{Position: 3, Label: "你信吗", Kind: "reflect", Status: "pending"},
		{Position: 4, Label: "你站哪边", Kind: "connect", Status: "pending"},
		{Position: 5, Label: "找出关键句", Kind: "hunt", Status: "pending"},
	}
}

func TestLiveEnglishCoachFirstTurnParses(t *testing.T) {
	prov, r := liveClass(t, gateway.ClassDialogue)
	blocks := liveEnglishBlocks()

	// 中文标题 + 英文正文，正是从地图「现在读」落下来的那个形状。
	system := buildReadingCoachSystem("en")
	prompt := buildReadingCoachPrompt("不吃早餐算不算不道德？", blocks, readingOutline{}, liveEnglishTasks(), nil, nil, "", nil, "")

	bad := 0
	for i := 0; i < 6; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
		start := time.Now()
		res, err := gateway.Collect(ctx, prov, r, gateway.ChatRequest{
			Messages: []gateway.ChatMessage{
				{Role: gateway.RoleSystem, Content: system},
				{Role: gateway.RoleUser, Content: prompt},
			},
		})
		cancel()
		if err != nil {
			t.Fatalf("sample %d: live call failed (%s): %v", i, r.ModelID, err)
		}
		t.Logf("sample %d — %s | %v | out=%d | stop=%q", i, r.ModelID,
			time.Since(start).Round(time.Millisecond), res.Usage.OutputTokens, res.StopReason)

		got, ok := parseReadingCoachReply(res.Text, blocks, "en", func(string) bool { return true })
		if !ok {
			bad++
			t.Logf("sample %d: REJECTED. stop=%q len=%d\n--- raw ---\n%s\n--- end ---",
				i, res.StopReason, len(res.Text), res.Text)
			continue
		}
		if strings.TrimSpace(got.Reply) == "" {
			t.Errorf("sample %d: parsed but reply empty", i)
		}
		t.Logf("sample %d ok — advance=%q focus=%q reply=%.80s…", i, got.Advance, got.FocusBlock, got.Reply)
	}
	t.Logf("RESULT: %d/6 rejected", bad)
	if bad > 0 {
		t.Errorf("%d/6 English first turns unparseable — she gets 「AI 响应错误」", bad)
	}
}
