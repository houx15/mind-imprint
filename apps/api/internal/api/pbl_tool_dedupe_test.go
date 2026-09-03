package api_test

import (
	"encoding/json"
	"net/http"
	"testing"
)

// 🚨 同一件工具在她屏幕上只能有一张卡。
//
// 2026-09-03 线上实测：印记递了「头脑风暴」，那张卡因为前端没刷新没显示出来；
// 她只好自己打字，下一轮印记又递了一遍——于是屏幕上并排两张一模一样的邀请卡，
// 理由还各写各的（「说不定能看出几条线」/「才看得出哪些是一回事」）。她做完
// 其中一张，另一张就永远挂在那儿，指着一件早就做完的事。
//
// 前端那个刷新 bug 已经修了，但印记这边也要有一道闸：prompt 里写了「已经递过
// 的不要再递」，而 prompt 是请求，这是保证。
func TestPblTurn_ToolAlreadyOnHerScreenIsNotOfferedTwice(t *testing.T) {
	h, cookie, _, _ := liteHandlerWithProvider(t, pblCoachSaying(
		`{"reply":"先摊开看看。","hook":"","hook_kind":"",
		  "tool":"board","tool_reason":"你刚带回来一堆观察，先摊开看看",
		  "produce":null}`))
	pid := newProjectViaAPI(t, h, cookie)

	// 第一轮：递出来，正常。
	rec := pblPost(t, h, cookie, "/api/v1/pbl/projects/"+pid+"/turn",
		`{"text":"我想弄清楚食堂为什么剩这么多饭"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("turn 1 = %d; body=%s", rec.Code, rec.Body)
	}
	var first struct {
		Tool string `json:"tool"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &first); err != nil {
		t.Fatalf("decode turn 1: %v", err)
	}
	if first.Tool != "board" {
		t.Fatalf("第一次就没递出来：tool=%q", first.Tool)
	}

	// 第二轮：她没点那张卡，印记又递了一次同一件。这一次必须被挡掉。
	rec = pblPost(t, h, cookie, "/api/v1/pbl/projects/"+pid+"/turn",
		`{"text":"那我该先看哪儿"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("turn 2 = %d; body=%s", rec.Code, rec.Body)
	}
	var second struct {
		Reply string `json:"reply"`
		Tool  string `json:"tool"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &second); err != nil {
		t.Fatalf("decode turn 2: %v", err)
	}
	if second.Reply == "" {
		t.Fatal("回话被一起吞了；闸拦的只该是那张重复的卡")
	}
	if second.Tool != "" {
		t.Fatalf("同一件工具递了第二遍：%q", second.Tool)
	}

	// 屏幕上只有一张。
	rec = pblReq(t, h, cookie, "GET", "/api/v1/pbl/projects/"+pid+"/tools", "")
	var tools []struct {
		Tool   string `json:"tool"`
		Status string `json:"status"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &tools); err != nil {
		t.Fatalf("decode tools: %v — body=%s", err, rec.Body)
	}
	var boards int
	for _, x := range tools {
		if x.Tool == "board" {
			boards++
		}
	}
	if boards != 1 {
		t.Fatalf("她屏幕上有 %d 张「头脑风暴」，应该只有 1 张：%+v", boards, tools)
	}
}
