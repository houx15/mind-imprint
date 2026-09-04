package api_test

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"mindimprint/api/internal/gateway"
)

// pbl_tool_drop_test.go —— 被撤掉的那件工具，两边都要知道。
//
// 🚨 这是 2026-09-04 模拟学生走查里最烧时间的一条，而且它长得不像 bug：
// pbl_tool_gate.go 那道闸做的是对的事——印记说要递「审核助手」却没做出要审的
// 成果，那个界面打开是一块白板，撤掉比让她点进一间空屋子好。日志里也确实写了
// 「tool whose surface would be blank; dropping it」。
//
// 坏的是撤掉之后**没有人知道**。学生刚读到印记说「卡我给你了」，屏幕上没有；
// 印记下一轮的上文一个字都没变，于是它照样再说一遍。Marcus 在这个循环里耗掉约
// 35 步、8 次要老师直接替他点。
//
// 所以这里测的是那条回路的两端：她那边看得见一行说明，印记那边下一轮收得到。

// 撤掉的那件工具不会出现在她的工具列表里——这一半原来就是对的，守住它。
func TestPblToolGate_BlankToolNeverReachesHer(t *testing.T) {
	h, cookie, _, _ := liteHandlerWithProvider(t, pblCoachSaying(
		`{"reply":"我把审核助手给你了，你点开看看。","hook":"","hook_kind":"",
		  "tool":"review","tool_reason":"这一版你自己判断哪里不对"}`))
	pid := newProjectViaAPI(t, h, cookie)

	rec := pblPost(t, h, cookie, "/api/v1/pbl/projects/"+pid+"/turn", `{"text":"那你先写一版"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("turn = %d; body=%s", rec.Code, rec.Body)
	}
	var turn struct {
		Tool   string  `json:"tool"`
		ToolID *string `json:"toolId"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &turn); err != nil {
		t.Fatalf("decode turn: %v — body=%s", err, rec.Body)
	}
	if turn.Tool != "" || turn.ToolID != nil {
		t.Fatalf("没有成果却把审核助手递出去了：tool=%q id=%v", turn.Tool, turn.ToolID)
	}

	rec = pblReq(t, h, cookie, "GET", "/api/v1/pbl/projects/"+pid+"/tools", "")
	var tools []struct {
		Tool string `json:"tool"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &tools); err != nil {
		t.Fatalf("decode tools: %v — body=%s", err, rec.Body)
	}
	for _, x := range tools {
		if x.Tool == "review" {
			t.Fatalf("撤掉的工具还是进了她的列表：%s", rec.Body)
		}
	}
}

// 🚨 撤掉之后她要看得见这件事。
//
// 印记那句「我把审核助手给你了」已经落库、改不动了；能做的是紧接着说清楚它没
// 出现、以及缺的是什么。不说，她就会去找一张永远不会出现的卡。
func TestPblToolGate_TellsHerTheToolWasNotHandedOver(t *testing.T) {
	h, cookie, _, _ := liteHandlerWithProvider(t, pblCoachSaying(
		`{"reply":"我把审核助手给你了，你点开看看。","hook":"","hook_kind":"",
		  "tool":"review","tool_reason":"这一版你自己判断哪里不对"}`))
	pid := newProjectViaAPI(t, h, cookie)
	if rec := pblPost(t, h, cookie, "/api/v1/pbl/projects/"+pid+"/turn",
		`{"text":"那你先写一版"}`); rec.Code != http.StatusOK {
		t.Fatalf("turn = %d; body=%s", rec.Code, rec.Body)
	}

	rec := pblReq(t, h, cookie, "GET", "/api/v1/pbl/projects/"+pid+"/thread", "")
	var msgs []struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &msgs); err != nil {
		t.Fatalf("decode thread: %v — body=%s", err, rec.Body)
	}
	var notice string
	for _, m := range msgs {
		if m.Role == "system" && strings.Contains(m.Content, "未递出") {
			notice = m.Content
		}
	}
	if notice == "" {
		t.Fatalf("工具被撤掉了，线程里却没有一行说明——她只能自己去找那张不存在的卡：%s", rec.Body)
	}
	// 说得出缺的是什么，否则那一行等于「出错了」，她还是不知道下一步做什么。
	if !strings.Contains(notice, "可审的成果") {
		t.Errorf("说明没说清缺的是什么：%q", notice)
	}
}

// 🚨 印记下一轮要收得到，否则它会把同一句话说到底。
//
// 这里从**第二轮的 prompt**里验：第一轮撤掉之后，第二轮送进模型的上下文里必须
// 出现那件工具的名字和「没有出现在她屏幕上」。这是唯一能证明回路接上了的地方——
// 只看数据库那一行，证明的是我们记下来了，不是印记读到了。
func TestPblToolGate_NextTurnPromptCarriesTheDrop(t *testing.T) {
	prov := gateway.NewStubProvider([]gateway.StreamEvent{
		{Kind: gateway.EventTextDelta, TextDelta: `{"reply":"我把审核助手给你了。",
		  "hook":"","hook_kind":"","tool":"review","tool_reason":"这一版你自己判断哪里不对"}`},
		{Kind: gateway.EventUsage, Usage: &gateway.ChatUsage{InputTokens: 30, OutputTokens: 20}},
		{Kind: gateway.EventDone, StopReason: gateway.StopStop},
	})
	h, cookie, _, _ := liteHandlerWithProvider(t, prov)
	pid := newProjectViaAPI(t, h, cookie)

	// 第一轮：印记递 review，闸把它撤掉。
	if rec := pblPost(t, h, cookie, "/api/v1/pbl/projects/"+pid+"/turn",
		`{"text":"那你先写一版"}`); rec.Code != http.StatusOK {
		t.Fatalf("turn 1 = %d; body=%s", rec.Code, rec.Body)
	}
	// 第二轮：这一轮送进去的上下文里必须有那件被撤掉的工具。
	if rec := pblPost(t, h, cookie, "/api/v1/pbl/projects/"+pid+"/turn",
		`{"text":"审核助手在哪，我没看到"}`); rec.Code != http.StatusOK {
		t.Fatalf("turn 2 = %d; body=%s", rec.Code, rec.Body)
	}

	var last string
	for _, m := range prov.LastRequest.Messages {
		if m.Role == gateway.RoleUser {
			last = m.Content
		}
	}
	if !strings.Contains(last, "上一轮有一件工具没递出去") {
		t.Fatalf("第二轮的上下文里没提被撤掉的工具，印记只会把同一句话再说一遍：\n%s", last)
	}
	if !strings.Contains(last, "审核助手") {
		t.Errorf("说了有工具被撤，却没说是哪一件：\n%s", last)
	}
}
