package api_test

import (
	"encoding/json"
	"net/http"
	"testing"
)

// 🚨 递一件点开是空的工具，是这轮 walk 里最挫败的一幕。
//
// 2026-09-02 线上实测：印记说「我们一起把方案定下来」，递出「理性决策」，她
// 点进去看到的是
//
//	「暂时没有需要决策的内容。印记提出几个方案时，会在这里让你选。」
//
// 结构审查和分工建议一模一样。她是照着印记说的点进来的，却被打发回去求印记再
// 做一遍它刚说要做的事——这教她的是"这里的按钮不作数"。
//
// 修法有两层：prompt 里写清楚工具和产出要同一轮一起给（coach.go），以及这道闸
// ——产出没落上，工具就不递。prompt 是请求，闸是保证。
func TestPblTurn_ToolThatWouldOpenBlankIsNotOffered(t *testing.T) {
	h, cookie, _, _ := liteHandlerWithProvider(t, pblCoachSaying(
		`{"reply":"我们把方案定下来。","hook":"","hook_kind":"",
		  "tool":"decide","tool_reason":"你现在有两条路，先摊开看看",
		  "produce":null}`))
	pid := newProjectViaAPI(t, h, cookie)

	rec := pblPost(t, h, cookie, "/api/v1/pbl/projects/"+pid+"/turn",
		`{"text":"我想弄清楚食堂为什么剩这么多饭"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("turn = %d; body=%s", rec.Code, rec.Body)
	}
	var turn struct {
		Reply string `json:"reply"`
		Tool  string `json:"tool"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &turn); err != nil {
		t.Fatalf("decode turn: %v — body=%s", err, rec.Body)
	}
	// 她该看见的回话照常给她——闸拦的只是那件空工具。
	if turn.Reply == "" {
		t.Fatal("回话被闸一起吞了；她该看见印记说的话")
	}
	if turn.Tool != "" {
		t.Fatalf("递出了一件点开是空的工具：%q", turn.Tool)
	}

	rec = pblReq(t, h, cookie, "GET", "/api/v1/pbl/projects/"+pid+"/tools", "")
	var tools []struct {
		Tool string `json:"tool"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &tools); err != nil {
		t.Fatalf("decode tools: %v — body=%s", err, rec.Body)
	}
	if len(tools) != 0 {
		t.Fatalf("空工具还是落库了：%+v", tools)
	}
}

// 同一件工具，配上这一轮做出来的那个选择，就该照常递到她手边。
//
// 闸判的是"她点进去有没有东西"，不是"模型说它做了没有"——所以顺序要紧：
// applyPblProduce 先落库，闸再问库。
func TestPblTurn_ToolArrivesWhenItsContentArrivesWithIt(t *testing.T) {
	h, cookie, _, _ := liteHandlerWithProvider(t, checkedPlanCoach(
		`{"reply":"两条路，你来定。","hook":"","hook_kind":"",
		  "tool":"decide","tool_reason":"你现在有两条路，先摊开看看",
		  "produce":{"kind":"decision","payload":{
		    "subject":"先改哪一头",
		    "options":[{"label":"改打饭的量","description":"见效快，但只压住了症状"},
		               {"label":"改菜单","description":"慢，但动的是根子"}]}}}`))
	pid := newProjectViaAPI(t, h, cookie)

	rec := pblPost(t, h, cookie, "/api/v1/pbl/projects/"+pid+"/turn",
		`{"text":"我想弄清楚食堂为什么剩这么多饭"}`)
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
	if turn.Tool != "decide" || turn.ToolID == nil {
		t.Fatalf("产出和工具一起来的，工具却没递出去：tool=%q id=%v", turn.Tool, turn.ToolID)
	}

	// 她点进去要有东西：那个选择在库里。
	rec = pblReq(t, h, cookie, "GET", "/api/v1/pbl/projects/"+pid+"/decisions", "")
	var ds []struct {
		Subject string `json:"subject"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &ds); err != nil {
		t.Fatalf("decode decisions: %v — body=%s", err, rec.Body)
	}
	if len(ds) != 1 || ds[0].Subject != "先改哪一头" {
		t.Fatalf("界面上没有她要判的那个选择：%+v", ds)
	}
}

// 自带内容的工具不受这道闸影响——观察日记、便签板这些，界面里本来就有事情做。
func TestPblTurn_ToolWithOwnContentIsUnaffectedByTheGate(t *testing.T) {
	h, cookie, _, _ := liteHandlerWithProvider(t, pblCoachSaying(
		`{"reply":"先去看三天。","hook":"","hook_kind":"",
		  "tool":"observe","tool_reason":"你刚说没仔细看过，先去看三天中午",
		  "produce":null}`))
	pid := newProjectViaAPI(t, h, cookie)

	rec := pblPost(t, h, cookie, "/api/v1/pbl/projects/"+pid+"/turn",
		`{"text":"我想弄清楚食堂为什么剩这么多饭"}`)
	var turn struct {
		Tool string `json:"tool"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &turn); err != nil {
		t.Fatalf("decode turn: %v — body=%s", err, rec.Body)
	}
	if turn.Tool != "observe" {
		t.Fatalf("闸拦错了人：一件自带内容的工具没递出去，tool=%q", turn.Tool)
	}
}
