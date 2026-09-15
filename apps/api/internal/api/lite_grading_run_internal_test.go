package api

import (
	"context"
	"strings"
	"testing"

	"mindimprint/api/internal/gateway"
	"mindimprint/api/internal/liteassign"
	"mindimprint/api/internal/litegrade"
	"mindimprint/api/internal/store/sqlc"
)

const runTestBody = "学校后门那片空地一下雨就积水。去年秋天，我在那里摔过一跤。\n\n我读到城市里的雨水花园：用下凹的绿地先把雨水接住。"

// runTestValid passes Check against runTestBody and the zh default rubric.
const runTestValid = `{"overall":{"grade":"B+","comment":"用「去年秋天，我在那里摔过一跤。」引出问题。"},
"dimensions":[{"name":"内容","grade":"B+","comment":"问题来自亲身经历。"},{"name":"结构","grade":"B","comment":"两段之间没有过渡句。"},{"name":"语言","grade":"A-","comment":"表达清楚。"},{"name":"书写规范","grade":"A","comment":"标点使用正确。"}],
"points":[{"kind":"good","quote":"去年秋天，我在那里摔过一跤。","text":"用具体经历引出问题。","action":null},
{"kind":"issue","quote":"我读到城市里的雨水花园：用下凹的绿地先把雨水接住。","text":"材料与后门空地之间没有说明联系。","action":"在这句后面写一句说明雨水花园和后门空地的关系。"},
{"kind":"issue","quote":"学校后门那片空地一下雨就积水。","text":"积水的程度没有数据。","action":"补充一次积水的深度或持续时间。"}]}`

func runTestScript(text string) []gateway.StreamEvent {
	return []gateway.StreamEvent{
		{Kind: gateway.EventTextDelta, TextDelta: text},
		{Kind: gateway.EventUsage, Usage: &gateway.ChatUsage{InputTokens: 100, OutputTokens: 50}},
		{Kind: gateway.EventDone, StopReason: gateway.StopStop},
	}
}

func runTestInput() litegrade.Input {
	return liteGradingInput(sqlc.GetLiteGradingSourceRow{Number: 1, Title: "雨水去哪儿了", Body: runTestBody, Lang: "zh"}, liteassign.DefaultRubric("zh"))
}

func TestGradeWithRetryFirstReplyPasses(t *testing.T) {
	prov := gateway.NewSequenceStubProvider(runTestScript(runTestValid))
	calls := 0
	c, reasons, attempts := gradeWithRetry(context.Background(), prov, gateway.Resolved{Provider: "stub"}, runTestInput(), func(gateway.ChatUsage) { calls++ })
	if len(reasons) != 0 || attempts != 1 || prov.Calls != 1 || calls != 1 {
		t.Fatalf("reasons=%v attempts=%d calls=%d metered=%d", reasons, attempts, prov.Calls, calls)
	}
	if c.Overall.Grade != "B+" || c.Points[0].Source != litegrade.SourceAI {
		t.Fatalf("content = %+v", c)
	}
}

func TestGradeWithRetryCarriesReasonsIntoTheRetry(t *testing.T) {
	bad := strings.Replace(runTestValid, `"quote":"去年秋天，我在那里摔过一跤。"`, `"quote":"去年冬天，我在那里摔过一跤。"`, 1)
	prov := gateway.NewSequenceStubProvider(runTestScript(bad), runTestScript(runTestValid))
	calls := 0
	_, reasons, attempts := gradeWithRetry(context.Background(), prov, gateway.Resolved{Provider: "stub"}, runTestInput(), func(gateway.ChatUsage) { calls++ })
	if len(reasons) != 0 || attempts != 2 || calls != 2 {
		t.Fatalf("reasons=%v attempts=%d metered=%d", reasons, attempts, calls)
	}
	msgs := prov.LastRequest.Messages
	last := msgs[len(msgs)-1]
	if last.Role != gateway.RoleUser || !strings.Contains(last.Content, "第 1 条意见的引文不在正文中：「去年冬天，我在那里摔过一跤。」") {
		t.Fatalf("retry turn = %+v", last)
	}
	if msgs[len(msgs)-2].Role != gateway.RoleAssistant || msgs[len(msgs)-2].Content != bad {
		t.Fatalf("the retry must carry the rejected reply, got %+v", msgs[len(msgs)-2])
	}
}

func TestGradeWithRetryGivesUpAfterTwo(t *testing.T) {
	prov := gateway.NewSequenceStubProvider(runTestScript("抱歉，我无法批改。"))
	calls := 0
	_, reasons, attempts := gradeWithRetry(context.Background(), prov, gateway.Resolved{Provider: "stub"}, runTestInput(), func(gateway.ChatUsage) { calls++ })
	if attempts != 2 || prov.Calls != 2 || calls != 2 || len(reasons) != 1 || reasons[0].Code != litegrade.ReasonUnparseable {
		t.Fatalf("reasons=%v attempts=%d calls=%d metered=%d", reasons, attempts, prov.Calls, calls)
	}
}

// liteGradingInput must wire the writing room's person check and symptom
// table; litegrade cannot import them itself.
func TestLiteGradingInputWiring(t *testing.T) {
	prompt := "写一篇关于雨的记叙文"
	target := int32(800)
	in := liteGradingInput(sqlc.GetLiteGradingSourceRow{Number: 2, Title: "雨", Body: "x", Lang: "en", AssignedPrompt: &prompt, TargetWords: &target}, liteassign.DefaultRubric("en"))
	if in.PersonJudging == nil || !in.PersonJudging("你很懒") {
		t.Fatal("PersonJudging must be personDirectedVerdict")
	}
	if in.SymptomCatalog != writingSymptomCatalog("en") || in.AssignedPrompt != prompt || in.TargetWords != 800 || in.VersionNumber != 2 {
		t.Fatalf("input = %+v", in)
	}
}

func TestLiteGradingArgsRunOnce(t *testing.T) {
	opts := LiteGradingArgs{}.InsertOpts()
	if opts.MaxAttempts != 1 || opts.Queue != liteGradingQueue || (LiteGradingArgs{}).Kind() != "lite_grading" {
		t.Fatalf("insert opts = %+v", opts)
	}
}
