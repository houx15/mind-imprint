package api

// writing_kind_live_test.go —— 真模型会不会照着闭表回 kind。
//
// 跑法：
//
//	LIVE_LLM=1 go test ./internal/api -run TestLiveWritingPlanKind -v -count=1
//
// 🚨 为什么这条必须用真模型：stub 测试只证明解析器读得懂**我写的** JSON
// （[[prompt-output-must-be-verifiable]]）。闭表这件事整个建立在「模型愿意
// 从九个词里挑一个」上面 —— 它要是继续回 `parentId` 或者自己造一个 role，
// 那么每一条节点都会在 parseWritingPlanReply 里被丢掉，而屏幕上看起来只是
// 「印记这轮什么都没记下来」，日志里也只有一行 warn。
//
// 同事 2026-09-20 的那个真实题目当用例：「学校应不应该允许学生带手机」。

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"mindimprint/api/internal/gateway"
	"mindimprint/api/internal/store/sqlc"
)

// livePlanTurn 跑一次真的规划轮，返回模型原样回的那份文本。
func livePlanTurn(t *testing.T, wr sqlc.Writing, rows []sqlc.WritingOutline, said string) string {
	t.Helper()
	prov, resolved := liveClass(t, gateway.ClassCompose)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	res, err := gateway.Collect(ctx, prov, resolved, gateway.ChatRequest{
		Messages: []gateway.ChatMessage{
			// 🚨 用 writingPlanSystemFor，不要用那个模板本身。
			// 2026-09-22 发现：这里原来传的是 `writingPlanSystem`，里面
			// `@@KINDS@@` 和 `%d` 两个占位符都还没替换 —— 也就是说这条
			// 「模型会不会照着闭表回 kind」的测试，**从来没有把那张闭表发给
			// 模型**。它测的是一份生产环境不会发出去的提示词。
			{Role: gateway.RoleSystem, Content: writingPlanSystemFor(genreArgument, wr.Lang, "")},
			{Role: gateway.RoleUser, Content: buildWritingPlanPrompt(wr, rows, nil, said)},
		},
	})
	if err != nil {
		t.Fatalf("provider: %v", err)
	}
	t.Logf("模型原样回的：\n%s", res.Text)
	return res.Text
}

func TestLiveWritingPlanKindIsAlwaysLegal(t *testing.T) {
	wr := sqlc.Writing{Lang: "zh"}
	words := int32(500)
	wr.TargetWords = &words

	// 一轮一轮往下走，每一轮都检查它回的 kind 在不在闭表里。
	// 三轮里覆盖到主张、理由、材料三种最常见的产出。
	turns := []struct {
		name string
		said string
		rows []sqlc.WritingOutline
	}{
		{
			name: "她说出主张",
			said: "我觉得学校应该允许学生带手机。",
			rows: nil,
		},
		{
			name: "她给了一条理由",
			said: "放学的时候可以联系家长，我家离学校远，有时候要换车。",
			rows: []sqlc.WritingOutline{
				{Text: "学校应该允许学生带手机", Kind: writingKindThesis, Role: "中心论点", Depth: 0, Position: 0},
			},
		},
		{
			name: "她带回来一份研究",
			said: "我找到一个研究，Lu 2008，30 个高中生用手机短信学英语单词，测出来比纸质组记得多。出处是 onlinelibrary.wiley.com。",
			rows: []sqlc.WritingOutline{
				{Text: "学校应该允许学生带手机", Kind: writingKindThesis, Role: "中心论点", Depth: 0, Position: 0},
				{Text: "放学能联系家长", Kind: writingKindPoint, Role: "分论点", Depth: 1, Position: 1},
				{Text: "学习上可以用手机查不会的题", Kind: writingKindPoint, Role: "分论点", Depth: 1, Position: 2},
			},
		},
	}

	for _, tc := range turns {
		t.Run(tc.name, func(t *testing.T) {
			out := livePlanTurn(t, wr, tc.rows, tc.said)

			parsed, ok := parseWritingPlanReply(out)
			if !ok {
				t.Fatalf("真模型的回复读不出来：\n%s", out)
			}

			// 🚨 解析器已经把非法的 kind 丢掉了，所以「解析成功」证明不了
			// 模型守规矩 —— 要看它原样回的那一份。
			var raw struct {
				Add []map[string]any `json:"add"`
			}
			if err := json.Unmarshal([]byte(stripWritingPlanFence(out)), &raw); err != nil {
				t.Fatalf("原始 JSON 读不出来：%v\n%s", err, out)
			}
			for i, node := range raw.Add {
				kind, _ := node["kind"].(string)
				if !writingKindValid(strings.ToLower(strings.TrimSpace(kind))) {
					t.Errorf("第 %d 个节点的 kind 不在闭表里：%q（整份：%s）", i+1, kind, out)
				}
				// 它不该再给位置 —— 给了说明提示词那一节没说清，
				// 而一个被采信的 parentId 正是这次要根除的那一类 bug。
				if _, has := node["parentId"]; has {
					t.Errorf("第 %d 个节点还带着 parentId：%v", i+1, node)
				}
			}

			t.Logf("reply=%q add=%d", parsed.Reply, len(parsed.Add))
		})
	}
}

// 她请我们替她搜 → 这一轮必须先说明再往下走（同事 2026-09-20 的意见 8）。
//
// 🚨 这条也只能用真模型验：`writingReplyOwnsTheRefusal` 判的是模型自己写的
// 那句话，stub 里我写什么它就说什么，证明不了提示词能不能让它说出来。
func TestLiveWritingPlanOwnsTheRefusal(t *testing.T) {
	wr := sqlc.Writing{Lang: "zh"}
	rows := []sqlc.WritingOutline{
		{Text: "学校应该允许学生带手机", Kind: writingKindThesis, Role: "中心论点", Depth: 0, Position: 0},
		{Text: "放学能联系家长", Kind: writingKindPoint, Role: "分论点", Depth: 1, Position: 1},
	}
	said := "我懒得搜，你帮我找吧" // 截图里的原话

	out := livePlanTurn(t, wr, rows, said)

	parsed, ok := parseWritingPlanReply(out)
	if !ok {
		t.Fatalf("真模型的回复读不出来：\n%s", out)
	}
	if !writingReplyOwnsTheRefusal(parsed.Reply) {
		t.Errorf("她请我们代劳，而这一轮一个字都没说自己不做这件事：\n%s", parsed.Reply)
	}
	// 说明之后要往下走，不要停在拒绝上 —— 回复里得有「找什么」的具体交代。
	if len([]rune(parsed.Reply)) < 20 {
		t.Errorf("只说了拒绝，没给她下一步：%q", parsed.Reply)
	}
	t.Logf("reply=%q", parsed.Reply)
}
