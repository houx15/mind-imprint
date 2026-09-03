package api_test

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"mindimprint/api/internal/gateway"
)

// pblStub scripts one coach turn and keeps the handle, so a test can read back
// the context the engine actually built.
func pblStub(jsonOut string) *gateway.StubProvider {
	return gateway.NewStubProvider([]gateway.StreamEvent{
		{Kind: gateway.EventTextDelta, TextDelta: jsonOut},
		{Kind: gateway.EventUsage, Usage: &gateway.ChatUsage{InputTokens: 30, OutputTokens: 20}},
		{Kind: gateway.EventDone, StopReason: gateway.StopStop},
	})
}

// lastUserText digs out the context message the turn sent to the model.
func lastUserText(p *gateway.StubProvider) string {
	for i := len(p.LastRequest.Messages) - 1; i >= 0; i-- {
		if p.LastRequest.Messages[i].Role == gateway.RoleUser {
			return p.LastRequest.Messages[i].Content
		}
	}
	return ""
}

// 🚨 闭环：一件工具做完，做出来的东西必须回到印记那里。
//
// 产品负责人 2026-09-03：「we invoke one interactive tool, it must have a finish
// signal and the finished content have to be sent back to AI to push forward
// the flow」。
//
// gatherPblToolWork 原来漏了四件工具：结构审查那整棵树、分工建议、项目复盘她写
// 的答案、以及审核时她对每一处的判断。她可以在结构上花十分钟重排提纲，而印记
// 下一轮完全不知道这个项目现在长什么样——工具做了，环没闭上。
//
// 断言落在「印记收到的上文」上，而不是某个内部函数的返回值：断掉的正是从库到
// prompt 这一段路，只测中间那个函数照样看不出来。
func TestPblRefeed_StructureReachesTheCoach(t *testing.T) {
	prov := pblStub(`{"reply":"我给了一份提纲。","hook":"","hook_kind":"",
		  "produce":{"kind":"structure","payload":{"nodes":[
		    {"title":"开场怎么说","body":"第一句决定别人读不读下去",
		     "children":[{"title":"先说钱去哪了"}]}]}}}`)
	h, cookie, _, _ := liteHandlerWithProvider(t, prov)
	pid := newProjectViaAPI(t, h, cookie)

	rec := pblPost(t, h, cookie, "/api/v1/pbl/projects/"+pid+"/turn",
		`{"text":"这件事我不知道从哪儿开始讲"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("turn 1 = %d; body=%s", rec.Code, rec.Body)
	}

	// 先确认它真的落库了，否则下面那条断言会因为别的原因红，查起来费劲。
	rec = pblReq(t, h, cookie, "GET", "/api/v1/pbl/projects/"+pid+"/tree", "")
	var tree struct {
		Nodes []struct {
			Title string `json:"title"`
		} `json:"nodes"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &tree); err != nil {
		t.Fatalf("decode tree: %v — body=%s", err, rec.Body)
	}
	if len(tree.Nodes) != 2 {
		t.Fatalf("结构没落库，后面的断言无从谈起：%+v", tree.Nodes)
	}

	// 下一轮：印记收到的上文里必须有那份结构。
	rec = pblPost(t, h, cookie, "/api/v1/pbl/projects/"+pid+"/turn", `{"text":"那我接着写"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("turn 2 = %d; body=%s", rec.Code, rec.Body)
	}
	ctx := lastUserText(prov)
	if !strings.Contains(ctx, "她定下来的结构") {
		t.Fatalf("结构没回到印记那里——环没闭上：\n%s", ctx)
	}
	if !strings.Contains(ctx, "开场怎么说") || !strings.Contains(ctx, "先说钱去哪了") {
		t.Fatalf("结构回去了但内容是空的：\n%s", ctx)
	}
}

// 她审成果时对每一处的判断，也要回到印记那里。
//
// 原来只回灌了「通过 / 打回」和一句理由——她认认真真答完四处，印记只知道
// "她点了通过"。
func TestPblRefeed_HerReviewAnswersReachTheCoach(t *testing.T) {
	prov := pblStub(`{"reply":"我写了一版。","hook":"","hook_kind":"",
		  "produce":{"kind":"artifact","payload":{
		    "kind":"draft","title":"接龙文案","body":"大家好，我们班要办义卖。",
		    "marks":[{"part":"开头","quote":"大家好，我们班要办义卖。",
		              "question":"这一句说清楚钱去哪了吗？"}]}}}`)
	h, cookie, _, _ := liteHandlerWithProvider(t, prov)
	pid := newProjectViaAPI(t, h, cookie)

	rec := pblPost(t, h, cookie, "/api/v1/pbl/projects/"+pid+"/turn", `{"text":"写一版给我看看"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("turn 1 = %d; body=%s", rec.Code, rec.Body)
	}

	rec = pblReq(t, h, cookie, "GET", "/api/v1/pbl/projects/"+pid+"/artifacts", "")
	var arts []struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &arts); err != nil || len(arts) != 1 {
		t.Fatalf("成果没落库：%v — body=%s", err, rec.Body)
	}
	rec = pblReq(t, h, cookie, "GET",
		"/api/v1/pbl/projects/"+pid+"/artifacts/"+arts[0].ID+"/review", "")
	var plan struct {
		Marks []struct {
			ID string `json:"id"`
		} `json:"marks"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &plan); err != nil || len(plan.Marks) != 1 {
		t.Fatalf("划出来的句子没落库：%v — body=%s", err, rec.Body)
	}

	// 她答了那一处。
	rec = pblReq(t, h, cookie, "PATCH",
		"/api/v1/pbl/projects/"+pid+"/marks/"+plan.Marks[0].ID,
		`{"answer":"没说清楚，得写明钱捐给流浪动物救助站"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("答不上去：%d — body=%s", rec.Code, rec.Body)
	}

	rec = pblPost(t, h, cookie, "/api/v1/pbl/projects/"+pid+"/turn", `{"text":"改好了叫我"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("turn 2 = %d; body=%s", rec.Code, rec.Body)
	}
	ctx := lastUserText(prov)
	if !strings.Contains(ctx, "流浪动物救助站") {
		t.Fatalf("她审出来的判断没回到印记那里——环没闭上：\n%s", ctx)
	}
}
