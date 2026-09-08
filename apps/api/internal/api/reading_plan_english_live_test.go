package api

// reading_plan_english_live_test.go —— 排读法这一步的实测。
//
//	set -a; . .deploy-local/env.prod; set +a
//	LIVE_LLM=1 go test ./internal/api -run TestLiveEnglishReadingPlan -v -count=1
//
// 2026-09-08 线上验证时发现：同一个「开始」按钮上有**两个**互不相干的毛病。
// 教练那一轮修好之后，四次里仍有三次 502，日志里换成了另一句：
//
//	WARN reading plan: unparseable or out-of-library reply
//
// 那一句把三种失败裹成一个 bool —— JSON 读不了 / routineKey 不在库里 /
// 挑的读法语言不对。三种的修法完全不同，所以这个测试先把它们分开数。

import (
	"context"
	"testing"
	"time"

	"mindimprint/api/internal/gateway"
)

func TestLiveEnglishReadingPlanParses(t *testing.T) {
	prov, r := liveClass(t, gateway.ClassCompose)
	blocks := liveEnglishBlocks()
	prompt := buildReadingPlanPrompt("en", "Is skipping breakfast a moral failure?", blocks)

	bad := 0
	for i := 0; i < 6; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
		res, err := gateway.Collect(ctx, prov, r, gateway.ChatRequest{
			Messages: []gateway.ChatMessage{
				{Role: gateway.RoleSystem, Content: readingPlanSystem},
				{Role: gateway.RoleUser, Content: prompt},
			},
		})
		cancel()
		if err != nil {
			t.Fatalf("sample %d: live call failed (%s): %v", i, r.ModelID, err)
		}
		plan, routine, reject := parseReadingPlan(res.Text, "en")
		t.Logf("sample %d — out=%d stop=%q len=%d", i, res.Usage.OutputTokens, res.StopReason, len(res.Text))
		if reject != planOK {
			bad++
			t.Logf("sample %d: REJECTED (%s)\n--- raw ---\n%s\n--- end ---", i, reject, res.Text)
			continue
		}
		positions, _, _, _, _ := buildReadingTasks(routine, plan, blocks)
		t.Logf("sample %d ok — routine=%s focus=%v steps=%d", i, routine.Key, plan.FocusBlocks, len(positions))
		if len(positions) == 0 {
			t.Errorf("sample %d: routine produced no usable steps", i)
		}
	}
	t.Logf("RESULT: %d/6 rejected", bad)
	if bad > 0 {
		t.Errorf("%d/6 English plans unparseable — 「开始」按下去是 502", bad)
	}
}
