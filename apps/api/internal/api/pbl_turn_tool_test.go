package api_test

import (
	"encoding/json"
	"net/http"
	"testing"

	"mindimprint/api/internal/gateway"
)

// pblCoachSaying scripts one coach turn.
func pblCoachSaying(jsonOut string) gateway.Provider {
	return gateway.NewStubProvider([]gateway.StreamEvent{
		{Kind: gateway.EventTextDelta, TextDelta: jsonOut},
		{Kind: gateway.EventUsage, Usage: &gateway.ChatUsage{InputTokens: 30, OutputTokens: 20}},
		{Kind: gateway.EventDone, StopReason: gateway.StopStop},
	})
}

// 🚨 印记 递出来的工具必须真的到她手边。
//
// 这是整条链子最容易断的一环：工具箱、界面、端点全做好了，但模型那一头不知道
// 它们存在，于是一件也递不出来——七件工具就成了摆设，而且从测试上完全看不出来。
func TestPblTurn_ToolTheCoachOffersReachesHer(t *testing.T) {
	h, cookie, _, _ := liteHandlerWithProvider(t, pblCoachSaying(
		`{"reply":"你刚说了三件不太一样的事。","hook":"","hook_kind":"",
		  "tool":"board","tool_reason":"你刚说了三件不一样的事，先摊开看看"}`))
	pid := newProjectViaAPI(t, h, cookie)

	rec := pblPost(t, h, cookie, "/api/v1/pbl/projects/"+pid+"/turn",
		`{"text":"我想弄清楚食堂为什么剩这么多饭"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("turn = %d; body=%s", rec.Code, rec.Body)
	}
	var turn struct {
		Reply  string  `json:"reply"`
		Tool   string  `json:"tool"`
		ToolID *string `json:"toolId"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &turn); err != nil {
		t.Fatalf("decode turn: %v — body=%s", err, rec.Body)
	}
	if turn.Tool != "board" || turn.ToolID == nil {
		t.Fatalf("印记 递的工具没送出来：tool=%q id=%v", turn.Tool, turn.ToolID)
	}

	// 它要在工具列表里，带着理由和类别——那张邀请卡就是从这里画出来的。
	rec = pblReq(t, h, cookie, "GET", "/api/v1/pbl/projects/"+pid+"/tools", "")
	var tools []struct {
		ID     string `json:"id"`
		Tool   string `json:"tool"`
		Kind   string `json:"kind"`
		Label  string `json:"label"`
		Reason string `json:"reason"`
		Status string `json:"status"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &tools); err != nil {
		t.Fatalf("decode tools: %v — body=%s", err, rec.Body)
	}
	if len(tools) != 1 {
		t.Fatalf("工具列表里有 %d 件，want 1", len(tools))
	}
	got := tools[0]
	if got.Status != "summoned" {
		t.Fatalf("status = %q, want summoned", got.Status)
	}
	if got.Reason == "" {
		t.Fatal("递出来却没有理由——没有理由的工具是伏击")
	}
	if got.Kind != "thinking" {
		t.Fatalf("kind = %q, want thinking（头脑风暴是当场做完的）", got.Kind)
	}
	if got.Label != "头脑风暴" {
		t.Fatalf("label = %q —— 界面上要显示给她看的名字", got.Label)
	}
}

// 说不出为什么就当没递。服务端本来就会拒绝落库，与其半路失败，不如当它没发生。
func TestPblTurn_ToolWithoutAReasonIsNotOffered(t *testing.T) {
	h, cookie, _, _ := liteHandlerWithProvider(t, pblCoachSaying(
		`{"reply":"我们接着说。","tool":"board","tool_reason":"  "}`))
	pid := newProjectViaAPI(t, h, cookie)

	rec := pblPost(t, h, cookie, "/api/v1/pbl/projects/"+pid+"/turn", `{"text":"继续"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("turn = %d; body=%s", rec.Code, rec.Body)
	}
	var turn struct {
		ToolID *string `json:"toolId"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &turn)
	if turn.ToolID != nil {
		t.Fatal("没有理由的工具不该递出去")
	}

	rec = pblReq(t, h, cookie, "GET", "/api/v1/pbl/projects/"+pid+"/tools", "")
	var tools []map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &tools)
	if len(tools) != 0 {
		t.Fatalf("工具列表里有 %d 件，want 0", len(tools))
	}
}

// 出门做的那件工具，类别由工具箱说了算——模型说什么都不改这一点。
func TestPblTurn_WorldToolKeepsItsKind(t *testing.T) {
	h, cookie, _, _ := liteHandlerWithProvider(t, pblCoachSaying(
		`{"reply":"你还没真的去看过。","tool":"observe",
		  "tool_reason":"你说的都是猜的，先去看三天中午"}`))
	pid := newProjectViaAPI(t, h, cookie)

	if rec := pblPost(t, h, cookie, "/api/v1/pbl/projects/"+pid+"/turn",
		`{"text":"我觉得是米饭剩得最多"}`); rec.Code != http.StatusOK {
		t.Fatalf("turn = %d; body=%s", rec.Code, rec.Body)
	}
	rec := pblReq(t, h, cookie, "GET", "/api/v1/pbl/projects/"+pid+"/tools", "")
	var tools []struct {
		Kind  string `json:"kind"`
		Label string `json:"label"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &tools)
	if len(tools) != 1 || tools[0].Kind != "world" {
		t.Fatalf("观察日记应该是 world：%+v", tools)
	}
	if tools[0].Label != "观察日记" {
		t.Fatalf("label = %q", tools[0].Label)
	}
}
