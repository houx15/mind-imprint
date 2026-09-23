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
//
// 🚨 **2026-09-17 跟着生产的形状改了。** 通读现在一步一个部分
// （reading_plan.go 的 readingPartSteps），精读一步一段。旧的这一份只有一个
// 「通读全文」，喂给模型的于是是一份生产里不会出现的清单 ——
// [[fixture-told-coach-session-over-2026-09-14]]：用例的形状从生产代码里抄，
// 不然实测测的是另一个产品。
func liveEnglishTasks() []sqlc.ReadingTask {
	return []sqlc.ReadingTask{
		{Position: 0, Label: "先预测", Kind: "predict", Status: "pending", Detail: "只看标题：作者大概站哪一边？正文先别读。"},
		{Position: 1, Label: "通读第1–3段·口号的来历", Kind: "read", Status: "pending",
			BlockID: "b1", Detail: "请通读第1–3段。这几段摆出问题，追问那句口号从哪儿来。"},
		{Position: 2, Label: "通读第4–6段·证据的对决", Kind: "read", Status: "pending",
			BlockID: "b4", Detail: "请通读第4–6段。这几段用两种研究互相检验。"},
		{Position: 3, Label: "通读第7–9段·为何被道德化", Kind: "read", Status: "pending",
			BlockID: "b7", Detail: "请通读第7–9段。这几段解释那种语气为什么留得住。"},
		{Position: 4, Label: "精读重点段落第6段", Kind: "focus_block", Status: "pending", BlockID: "b6"},
		{Position: 5, Label: "精读重点段落第9段", Kind: "focus_block", Status: "pending", BlockID: "b9"},
		{Position: 6, Label: "深入思考", Kind: "lens", Status: "pending"},
		{Position: 7, Label: "你信吗", Kind: "reflect", Status: "pending"},
		{Position: 8, Label: "你站哪边", Kind: "connect", Status: "pending"},
		{Position: 9, Label: "找出关键句", Kind: "hunt", Status: "pending"},
	}
}

// liveEnglishOutline 是那一篇排出来的导读，含切法 —— 生产里带读每一轮都拿得到
// 它（buildReadingCoachPrompt 的【这篇分成几个部分】那一节）。
func liveEnglishOutline() readingOutline {
	return readingOutline{
		OneLine: "不吃早餐真的是道德问题吗？",
		Gist:    "作者认为早餐有益的证据被夸大了，那种自信在替人做道德评判。",
		Shape:   "设问 → 起源 → 证据检验 → 语言批判 → 有限结论",
		Load: map[string]string{
			"b1": loadBridge, "b2": loadCore, "b3": loadSupport,
			"b4": loadSupport, "b5": loadSupport, "b6": loadCore,
			"b7": loadBridge, "b8": loadSupport, "b9": loadCore,
			"b10": loadSupport, "b11": loadSupport, "b12": loadCore,
		},
		Parts: []readingPart{
			{Title: "口号的来历", From: "b1", To: "b3", Does: "摆出问题，追问口号的出处"},
			{Title: "证据的对决", From: "b4", To: "b6", Does: "用两种研究互相检验"},
			{Title: "为何被道德化", From: "b7", To: "b9", Does: "解释那种语气为什么留得住"},
			{Title: "诚实的结论", From: "b10", To: "b12", Does: "划定证据边界，收束论点"},
		},
	}
}

func TestLiveEnglishCoachFirstTurnParses(t *testing.T) {
	prov, r := liveClass(t, gateway.ClassDialogue)
	blocks := liveEnglishBlocks()

	// 中文标题 + 英文正文，正是从地图「现在读」落下来的那个形状。
	system := buildReadingCoachSystem("en", "")
	prompt := buildReadingCoachPrompt("不吃早餐算不算不道德？", blocks, liveEnglishOutline(),
		liveEnglishTasks(), nil, nil, "", nil, "")

	bad, twoAsks, noCard, ghost := 0, 0, 0, 0
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
		cardType := "（没有卡片）"
		if got.Card != nil {
			cardType = got.Card.Type
		} else {
			noCard++
		}
		if got.twoAsks {
			twoAsks++
			t.Logf("sample %d: 卡片 + 话又以另一个问句收尾 —— 屏幕上两道题\n  reply: %s\n  card:  %s",
				i, got.Reply, got.Card.Prompt)
		}
		// 🚨 引了一句文章上、卡片上、她嘴里都没有的话。产品负责人 2026-09-17
		// 报的第 2 条（「文本内容和卡片上的内容对应不上」），判据在
		// reading_ghostquote.go。这里量的是它多久犯一次 —— 走的是重试那条路，
		// 所以偶尔一次不致命，半数以上就说明那条规矩没进去。
		if q := firstGhostQuote(got.Reply, readingQuoteCorpus("不吃早餐算不算不道德？",
			blocks, liveEnglishOutline(), liveEnglishTasks(), nil, got.Card, nil)); q != "" {
			ghost++
			t.Logf("sample %d: 引了一句哪儿都没有的话 —— %q\n  reply: %s", i, q, got.Reply)
		}
		t.Logf("sample %d ok — advance=%q focus=%q card=%s reply=%.80s…",
			i, got.Advance, got.FocusBlock, cardType, got.Reply)
	}
	t.Logf("RESULT: %d/6 rejected · 两道题 %d/6 · 没发卡片 %d/6 · 引了不存在的句子 %d/6",
		bad, twoAsks, noCard, ghost)
	if bad > 0 {
		t.Errorf("%d/6 English first turns unparseable — she gets 「AI 响应错误」", bad)
	}
	// 🚨 twoAsks 走的是重试那条路，所以偶尔一次不致命（调用点会再问一遍）。
	// 但**半数以上**就说明 prompt 那条规矩没写进去，而重试是要花钱的。
	// 产品负责人 2026-09-17 报的正是这一幕：「卡片内容上的要求和对话窗口的
	// 文本要求不一致。」
	if twoAsks*2 > 6 {
		t.Errorf("%d/6 的第一轮同时给了卡片和另一个问题 —— 她不知道该答哪一个", twoAsks)
	}
	if ghost*2 > 6 {
		t.Errorf("%d/6 引了一句文章上和卡片上都没有的话 —— 她会去找那一句，找不到", ghost)
	}
	// 第一轮必须用一张卡片把她领进去（system prompt 的「卡片」那一节明写）。
	if noCard*2 > 6 {
		t.Errorf("%d/6 的第一轮一张卡片都没有 —— 她只拿到一段话和一个输入框", noCard)
	}
}
