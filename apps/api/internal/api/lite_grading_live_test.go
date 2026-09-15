package api

// lite_grading_live_test.go — AI 批改 against the real review model.
//
//	set -a; . .deploy-local/env.prod; set +a
//	LIVE_LLM=1 go test ./internal/api -run TestLiveLiteGrading -v -count=1
//
// The stub tests only prove Check reads JSON written by hand. This test
// proves the real model, given the real prompt, returns a grading that
// passes Check within the production retry. Run it three times before Part B
// ships and judge the worst run (AGENTS.md: score the worst case).

import (
	"context"
	"testing"
	"time"

	"mindimprint/api/internal/gateway"
	"mindimprint/api/internal/liteassign"
	"mindimprint/api/internal/litegrade"
	"mindimprint/api/internal/store/sqlc"
)

const liveGradingZH = `学校食堂每天倒掉的饭特别多。上周五中午，我在回收桶旁边站了二十分钟，数到六个桶都装满了。

光是那一天，倒掉的米饭大概能装满两个洗菜盆。食堂阿姨说，周五的剩饭总是最多，因为很多同学下午放学早，中午随便吃几口就走了。

我觉得学校可以把周五的饭量减少一些。也有同学说，饭少了会有人吃不饱。到底是什么让我们倒掉一整盘饭的时候，连眼皮都不会抬一下？`

const liveGradingEN = `Because of the food waste problem is very serious in our school, I think we should do something.
Every day the canteen throw away many rice. Last Friday I counted six bins are full.

Some students say the portions are too big. Others say the food is not tasty. In my opinion, the school should let students choose their portion size, because this is the simplest way.`

func TestLiveLiteGrading(t *testing.T) {
	cases := []struct {
		name, lang, prompt, body string
	}{
		{"zh", "zh", "写一篇议论文，讨论学校食堂的浪费问题，并提出你的建议。", liveGradingZH},
		{"en", "en", "Write an essay about food waste in your school and suggest one solution.", liveGradingEN},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			prov, resolved := liveClass(t, gateway.ClassReview)
			ctx, cancel := context.WithTimeout(context.Background(), 6*time.Minute)
			defer cancel()
			prompt := tc.prompt
			in := liteGradingInput(sqlc.GetLiteGradingSourceRow{
				Number: 1, Title: "食堂浪费", Body: tc.body, Lang: tc.lang, AssignedPrompt: &prompt,
			}, liteassign.DefaultRubric(tc.lang))

			content, reasons, attempts := gradeWithRetry(ctx, prov, resolved, in, func(u gateway.ChatUsage) {
				t.Logf("call: in=%d out=%d", u.InputTokens, u.OutputTokens)
			})
			t.Logf("attempts = %d", attempts)
			if len(reasons) > 0 {
				t.Fatalf("grading failed after %d attempts: %s", attempts, litegrade.JoinReasons(reasons))
			}
			if rs := litegrade.Check(content, in); len(rs) > 0 {
				t.Fatalf("returned content does not pass Check: %s", litegrade.JoinReasons(rs))
			}
			t.Logf("overall %s: %s", content.Overall.Grade, content.Overall.Comment)
			for _, d := range content.Dimensions {
				t.Logf("  %s %s: %s", d.Name, d.Grade, d.Comment)
			}
			for _, p := range content.Points {
				action := ""
				if p.Action != nil {
					action = *p.Action
				}
				t.Logf("  [%s] 「%s」\n    %s\n    %s", p.Kind, *p.Quote, p.Text, action)
			}
		})
	}
}
