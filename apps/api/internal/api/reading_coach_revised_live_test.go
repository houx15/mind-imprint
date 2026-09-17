package api

// reading_coach_revised_live_test.go —— 她改了答案之后，印记 回应的是不是新的
// 那一份。
//
//	LIVE_LLM=1 go test ./internal/api -run TestLiveRevisedAnswer -v -count=1
//
// 能验的（推进、卡片、协议词）在别的测试里；这里把回复整段打出来由人读：
// 它有没有说清她从哪一句换到了哪一句、有没有把上一份当成她现在的想法。

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"mindimprint/api/internal/gateway"
	"mindimprint/api/internal/store/sqlc"
)

func TestLiveRevisedAnswer(t *testing.T) {
	prov, r := liveClass(t, gateway.ClassDialogue)
	blocks := liveLensDoneBlocks()
	prompt := "哪一句最能说明这笔账难以核对？"
	first := "平台会说，抽成用于调度系统、保险和补贴。"
	second := "这个数字来自平台自己提供的数据，独立机构难以验证。"
	card, _ := json.Marshal(map[string]any{"card": map[string]any{"type": "choose_span", "prompt": prompt}})
	ans1, _ := json.Marshal(map[string]any{"answer": map[string]any{"type": "choose_span", "prompt": prompt, "choice": first}})
	msgs := []sqlc.AtomMessage{
		{Role: "ai", Content: "我们看看这笔账谁来记。", Payload: card},
		{Role: "student", Content: composeCardAnswerMessage(prompt, first, "", blocks...), Payload: ans1},
		{Role: "ai", Content: "你选了第 3 段讲抽成用途的那一句。它说的是钱花在哪儿，还没说这笔账能不能被核对。"},
	}
	tasks := []sqlc.ReadingTask{
		{Label: "通读第1–5段·配送费的去向", Kind: "read", Status: "done"},
		{Label: "你怎么看", Kind: "critique", Status: "pending"},
	}
	current := "> 【她改了答案】\n" + composeCardAnswerMessage(prompt, second, "", blocks...)
	system := buildReadingCoachSystem("zh")
	for i := 0; i < 4; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
		res, err := gateway.Collect(ctx, prov, r, gateway.ChatRequest{Messages: []gateway.ChatMessage{
			{Role: gateway.RoleSystem, Content: system},
			{Role: gateway.RoleUser, Content: buildReadingCoachPrompt("一份外卖的配送费，到底付给了谁？", blocks, readingOutline{}, tasks, msgs, nil, current, nil, "")},
		}})
		cancel()
		if err != nil {
			t.Fatalf("sample %d: %v", i, err)
		}
		got, ok := parseReadingCoachReply(res.Text, blocks, "zh", func(string) bool { return false })
		if !ok {
			t.Errorf("sample %d: 解析失败", i)
			continue
		}
		t.Logf("sample %d advance=%q leak=%q\n--- reply ---\n%s\n---", i, got.Advance, firstProtocolLeak(got.Reply), got.Reply)
	}
}
