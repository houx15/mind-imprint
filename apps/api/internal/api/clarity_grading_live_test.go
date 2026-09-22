package api

import (
	"context"
	"encoding/json"
	"fmt"
	"mindimprint/api/internal/claritytest"
	"mindimprint/api/internal/gateway"
	"mindimprint/api/internal/liteassign"
	"mindimprint/api/internal/litegrade"
	"testing"
)

func TestClarityGrading(t *testing.T) {
	for _, c := range []struct{ lang, body string }{
		{"zh", "学校图书馆可以延长开放时间。上周放学后，我和三位同学准备复习，教室已经关闭，我们只好坐在走廊。走廊里经常有人经过，我们很难集中注意力。如果图书馆能多开放一小时，我们就能找到安静的学习场所。不过，延长开放需要安排老师值班。学校可以每周试行一天，了解学生的需求，再决定是否增加天数。"},
		{"en", "Our school library should stay open for an extra hour. Last Tuesday, the classrooms and the library closed at seven, so I revised in the noisy corridor. A quiet room would help students like me concentrate. However, staying open requires staff. The school could try this once a week and count how many students use it before extending the schedule."},
	} {
		t.Run(c.lang, func(t *testing.T) {
			in := litegrade.Input{Lang: c.lang, Title: "School library", Body: c.body, VersionNumber: 1, Rubric: liteassign.DefaultRubric(c.lang), SymptomCatalog: writingSymptomCatalog(c.lang, genreNarrative), PersonJudging: personDirectedVerdict}
			req := gateway.ChatRequest{ResponseFormat: gateway.ResponseFormatJSONObject, Messages: []gateway.ChatMessage{{Role: gateway.RoleSystem, Content: litegrade.SystemPrompt(in)}, {Role: gateway.RoleUser, Content: litegrade.UserPrompt(in)}}}
			claritytest.Run(t, gateway.ClassReview, req, func(raw string) error {
				out, err := litegrade.Parse(raw)
				if err != nil {
					return err
				}
				if reasons := litegrade.Check(out, in); len(reasons) > 0 {
					return fmt.Errorf("grading contract: %v", reasons)
				}
				return checkClarityTeachingLanguage(raw)
			}, func(ctx context.Context, prov gateway.Provider, resolved gateway.Resolved, _ gateway.ChatRequest) (gateway.ChatResult, error) {
				var usage gateway.ChatUsage
				out := gradeWithRetry(ctx, prov, resolved, in, func(u gateway.ChatUsage) {
					usage.InputTokens += u.InputTokens
					usage.OutputTokens += u.OutputTokens
					usage.CachedInputTokens += u.CachedInputTokens
				})
				t.Logf("production grading attempts=%d priorFailures=%v", out.Attempts, out.Tried)
				b, _ := json.Marshal(out.Content)
				if len(out.Reasons) > 0 {
					b = []byte(out.LastReply)
				}
				res := gateway.ChatResult{Text: string(b), Usage: usage}
				if len(out.Reasons) > 0 {
					return res, fmt.Errorf("production grading failed: %v", out.Reasons)
				}
				return res, nil
			})
		})
	}
}
