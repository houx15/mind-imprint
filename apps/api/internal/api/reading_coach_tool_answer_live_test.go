package api

// reading_coach_tool_answer_live_test.go —— 她在「仿写」底下写了一段，印记 的
// 反馈长什么样。
//
//	LIVE_LLM=1 go test ./internal/api -run TestLiveToolAnswerFeedback -v -count=1
//
// 🚨 这里要看的是铁律①：**不替她写**。「把她那段改写一遍给她看」是仿写反馈里
// 最自然、也最伤的一种失败 —— 改写后的那段就是替她写了。这件事压不成一个子串
// 判据（[[optimizing-a-detector-made-coaching-worse-2026-09-14]]），所以整段打出来
// 由人读；能验的那一半（推进被拦掉）在 reading_coach_tool_answer_test.go。

import (
	"context"
	"testing"
	"time"

	"mindimprint/api/internal/gateway"
	"mindimprint/api/internal/store/sqlc"
)

func TestLiveToolAnswerFeedback(t *testing.T) {
	prov, r := liveClass(t, gateway.ClassDialogue)
	blocks := liveLensDoneBlocks()
	tasks := []sqlc.ReadingTask{
		{Label: "通读第1–5段·配送费的去向", Kind: "read", Status: "done"},
		{Label: "拆开作者的论证", Kind: "label", Status: "done"},
		{Label: "你怎么看", Kind: "critique", Status: "pending"},
	}
	cases := []struct{ prompt, hers string }{
		{"仿写 · 第2段：先说大家以为的，再用「实际上」翻出真实的流程",
			"很多人以为食堂的饭钱都给了厨师。实际上，饭钱先交给学校，学校再拿去付菜钱、水电和阿姨的工资。"},
		{"想一想 · 第4段：这个数字由平台自己提供，这对它的可信度有什么影响？",
			"我觉得不太可信，因为平台自己说自己好，就像考试自己给自己打分。"},
	}
	system := buildReadingCoachSystem("zh", "")
	for i := 0; i < 6; i++ {
		c := cases[i%2]
		content := composeCardAnswerMessage(c.prompt, "", c.hers, blocks...)
		prompt := buildReadingCoachPrompt("一份外卖的配送费，到底付给了谁？", blocks, readingOutline{}, tasks, nil, nil, content, nil, toolAnswerLine(true))
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
		res, err := gateway.Collect(ctx, prov, r, gateway.ChatRequest{
			Messages: []gateway.ChatMessage{
				{Role: gateway.RoleSystem, Content: system},
				{Role: gateway.RoleUser, Content: prompt},
			},
		})
		cancel()
		if err != nil {
			t.Fatalf("sample %d: %v", i, err)
		}
		got, ok := parseReadingCoachReply(res.Text, blocks, "zh", func(string) bool { return false })
		if !ok {
			t.Errorf("sample %d: 解析失败\n%s", i, res.Text)
			continue
		}
		leak := firstProtocolLeak(got.Reply)
		she := replyCallsHerShe(got.Reply, blocks)
		t.Logf("sample %d advance=%q card=%v leak=%q 她=%v\n--- reply ---\n%s\n---", i, got.Advance, got.Card != nil, leak, she, got.Reply)
	}
}
