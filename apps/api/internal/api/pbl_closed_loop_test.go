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
			Depth int16  `json:"depth"`
		} `json:"nodes"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &tree); err != nil {
		t.Fatalf("decode tree: %v — body=%s", err, rec.Body)
	}
	if len(tree.Nodes) != 2 {
		t.Fatalf("结构没落库，后面的断言无从谈起：%+v", tree.Nodes)
	}

	// 🚨 depth 必须真的写进去。produceStructure 一直**算**着 depth（用来在第三层
	// 打住），却从来没存过——印记建的每个节点都是 0。界面拿 depth 决定列和配色，
	// 于是十五个节点全挤在同一列、全是同一个颜色，「分层配色」从没生效过。
	//
	// 这条断言原来只看 title，所以这个 bug 一路上了线。
	byTitle := map[string]int16{}
	for _, n := range tree.Nodes {
		byTitle[n.Title] = n.Depth
	}
	if d := byTitle["开场怎么说"]; d != 0 {
		t.Fatalf("顶层那块的 depth 应该是 0，实际 %d", d)
	}
	if d := byTitle["先说钱去哪了"]; d != 1 {
		t.Fatalf("子节点的 depth 应该是 1，实际 %d——算了但没存", d)
	}

	// 下一轮：印记收到的上文里必须有那份结构。
	rec = pblPost(t, h, cookie, "/api/v1/pbl/projects/"+pid+"/turn", `{"text":"那我接着写"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("turn 2 = %d; body=%s", rec.Code, rec.Body)
	}
	ctx := lastUserText(prov)
	if !strings.Contains(ctx, "学生定下来的结构") {
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
	prov := gateway.NewSequenceStubProvider(evidenceScript(`{"reply":"我写了一版。","hook":"","hook_kind":"",
		  "produce":{"kind":"artifact","payload":{
		    "kind":"draft","title":"接龙文案","body":"大家好，我们班要办义卖。",
		    "marks":[{"part":"开头","quote":"大家好，我们班要办义卖。",
		              "question":"这一句说清楚钱去哪了吗？"}]}}}`),
		evidenceScript(`{"reply":"我会按你的审阅意见修改。"}`),
		evidenceScript(`{"supported":true,"issues":[]}`))
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
	if len(prov.Requests) != 3 {
		t.Fatalf("expected initial draft, revision, evidence check; got %d", len(prov.Requests))
	}
	rawContext, _ := json.Marshal(prov.Requests[1])
	ctx := string(rawContext)
	if !strings.Contains(ctx, "流浪动物救助站") {
		t.Fatalf("她审出来的判断没回到印记那里——环没闭上：\n%s", ctx)
	}
}

// 🚨 她把印记派给自己的一件事要回来了——这一句必须回到印记那里。
//
// 分工建议的 reassign 端点一直在，服务端也一直强制要理由，但**界面从来没调用
// 过**（2026-09-03 四个设计 agent 里有一个翻出来的）。于是 refeed 里那个
// 「她改的」分支永远不会触发：我写了一段读 StudentOwner 的代码，而没有任何东西
// 能设置它。
//
// 界面补上了，这条测试守住后半截：改完之后，印记的上文里要认得出这件事。
// 铁律④——她把 AI 的活要回来，是这个产品最该记住的一种信号。
func TestPblRefeed_HerReassignmentReachesTheCoach(t *testing.T) {
	prov := gateway.NewSequenceStubProvider(evidenceScript(`{"reply":"这一步分一下。","hook":"","hook_kind":"",
		  "produce":{"kind":"plan","payload":{"summary":"三天试一次",
		    "reason":"先小范围试","steps":[{"title":"起草文案","blurb":"",
		    "goal":"","youBring":"","iBring":"","decide":"文案里哪一句最要紧",
		    "thenBring":""}]}}}`),
		evidenceScript(`{"supported":true,"issues":[]}`),
		evidenceScript(`{"reply":"你来写，我帮你检查。"}`))
	h, cookie, _, _ := liteHandlerWithProvider(t, prov)
	pid := newProjectViaAPI(t, h, cookie)

	rec := pblPost(t, h, cookie, "/api/v1/pbl/projects/"+pid+"/turn", `{"text":"帮我理一下"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("turn 1 = %d; body=%s", rec.Code, rec.Body)
	}

	// 拿到计划里的第一步。
	rec = pblReq(t, h, cookie, "GET", "/api/v1/pbl/projects/"+pid+"/plan", "")
	var plan struct {
		Plan struct {
			VersionID string `json:"versionId"`
			Steps     []struct {
				ID string `json:"id"`
			} `json:"steps"`
		} `json:"plan"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &plan); err != nil || len(plan.Plan.Steps) == 0 {
		t.Fatalf("计划没出来：%v — body=%s", err, rec.Body)
	}
	stepID := plan.Plan.Steps[0].ID

	// 🚨 她得先认下这份计划。分工是挂在**生效中**那一版计划上的（refeed 走
	// GetPblLivePlan），没批准的那一版对印记来说还不存在——这不是测试的绕路，
	// 真实流程里 Split 那一屏读的也是生效计划。
	rec = pblReq(t, h, cookie, "POST", "/api/v1/pbl/projects/"+pid+"/plan/approve",
		`{"versionId":"`+plan.Plan.VersionID+`"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("计划没批准：%d — body=%s", rec.Code, rec.Body)
	}

	// 印记给了一份分工，第一件归它自己。
	rec = pblReq(t, h, cookie, "POST",
		"/api/v1/pbl/projects/"+pid+"/steps/"+stepID+"/substeps",
		`{"substeps":[{"title":"写第一版文案","owner":"yinji","reason":"我先起个头"}]}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("分工没落库：%d — body=%s", rec.Code, rec.Body)
	}
	var subs []struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &subs); err != nil || len(subs) != 1 {
		t.Fatalf("decode substeps: %v — body=%s", err, rec.Body)
	}

	// 她把它要了回来。
	rec = pblReq(t, h, cookie, "POST",
		"/api/v1/pbl/projects/"+pid+"/substeps/"+subs[0].ID+"/reassign",
		`{"owner":"student","reason":"文案得用我们班自己的说法"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("改判失败：%d — body=%s", rec.Code, rec.Body)
	}

	// 下一轮，印记要看得见这件事。
	rec = pblPost(t, h, cookie, "/api/v1/pbl/projects/"+pid+"/turn", `{"text":"我来写"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("turn 2 = %d; body=%s", rec.Code, rec.Body)
	}
	rawContext, _ := json.Marshal(prov.LastRequest)
	ctx := string(rawContext)
	if !strings.Contains(ctx, "这一步的分工") {
		t.Fatalf("分工没回到印记那里——环没闭上：\n%s", ctx)
	}
	if !strings.Contains(ctx, "学生改的") {
		t.Fatalf("「她改的」这条信号丢了，而它正是这件工具最要紧的产出：\n%s", ctx)
	}
	if !strings.Contains(ctx, "文案得用我们班自己的说法") {
		t.Fatalf("她给的理由没带上：\n%s", ctx)
	}
}
