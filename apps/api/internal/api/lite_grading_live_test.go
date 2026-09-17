package api

// lite_grading_live_test.go — AI 批改 against the real review model.
//
// Run from apps/api (this command's working directory matters — the env
// file is sourced by absolute path so it does not depend on it):
//
//	set -a; . /Users/houyuxin/08Coding/mind-imprint/.deploy-local/env.prod; set +a
//	LIVE_LLM=1 CGO_ENABLED=0 go test ./internal/api -run TestLiveLiteGrading -v -count=1
//
// apps/api/.env.local (also non-empty DASHSCOPE_API_KEY) works the same way.
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

// liveGradingWalk is the essay of the 2026-09-17 real-user walk. The review
// model returned invalid JSON for it on both attempts, twice (a stray
// "dimensions_placeholder" key inside the dimensions array).
const liveGradingWalk = `现在很多同学都用 AI 写作业，有人直接让它生成整篇作文，也有人只用它查资料。学校到底该不该允许？我认为应该允许，但要规定清楚怎么用。

第一，AI 查资料快，能给我们提供自己想不到的角度。上次写环保作文，我用 AI 找到了塑料回收率的数据，这是我自己查不到的。

第二，全面禁止反而管不住。大家会偷偷用，老师也分辨不出来。不如公开规定：可以用来查资料、检查语法，但不能直接生成段落。

有人会说，用了 AI 学生就不会自己写了。这个担心有道理，所以规定的重点是「不能让 AI 写正文」，写的部分必须是自己的。`

func TestLiveLiteGrading(t *testing.T) {
	cases := []struct {
		name, lang, title, prompt, body string
	}{
		{"zh", "zh", "食堂浪费", "写一篇议论文，讨论学校食堂的浪费问题，并提出你的建议。", liveGradingZH},
		{"en", "en", "食堂浪费", "Write an essay about food waste in your school and suggest one solution.", liveGradingEN},
		{"zh-walk", "zh", "学校该不该允许学生用AI写作业", "学校该不该允许学生用AI写作业？请就此问题写一篇议论文，表明你的立场并给出理由。", liveGradingWalk},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			prov, resolved := liveClass(t, gateway.ClassReview)
			ctx, cancel := context.WithTimeout(context.Background(), 6*time.Minute)
			defer cancel()
			prompt := tc.prompt
			in := liteGradingInput(sqlc.GetLiteGradingSourceRow{
				Number: 1, Title: tc.title, Body: tc.body, Lang: tc.lang, AssignedPrompt: &prompt,
			}, liteassign.DefaultRubric(tc.lang))

			out := gradeWithRetry(ctx, prov, resolved, in, func(u gateway.ChatUsage) {
				t.Logf("call: in=%d out=%d", u.InputTokens, u.OutputTokens)
			})
			content, reasons, attempts := out.Content, out.Reasons, out.Attempts
			t.Logf("attempts = %d", attempts)
			for i, rs := range out.Tried {
				t.Logf("attempt %d failed: %s", i+1, litegrade.JoinReasons(rs))
			}
			if len(reasons) > 0 {
				t.Fatalf("grading failed after %d attempts: %s (parse error: %v; last reply: %s)", attempts, litegrade.JoinReasons(reasons), out.ParseErr, out.LastReply)
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
