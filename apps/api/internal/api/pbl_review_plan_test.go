package api_test

import (
	"encoding/json"
	"net/http"
	"testing"
)

// 🚨 交一份东西给她审，就要说清楚这份东西该怎么看。
//
// 2026-09-03 线上实测：「审核助手」第一次真的有东西可审了（印记的接龙文案），
// 而那一屏是 marks: 0 / dimensions: 0——她能做的只有从头读到尾，然后点通过。
// 那不是审核，是走过场。
//
// createPblReviewPlan 那个端点的注释写着「印记交东西时，连着说清楚每一部分该
// 看什么」，表、端点、前端的 splitByMarks/MarkRow/AnswerBox 全都在，可 produce
// 的 payload 里一直没有这两格，模型无从填——和 2026-09-02 那个 produce 缺口
// 一模一样的断法。
//
// docs/2026-09-01-pbl-detail.md：「highlighted sentences with questions to
// answer」+「ask students to answer several dimensions」。
func TestPblTurn_ArtifactArrivesWithSomethingToActuallyReview(t *testing.T) {
	h, cookie, _, _ := liteHandlerWithProvider(t, pblCoachSaying(
		`{"reply":"我写了一版，你审一下。","hook":"","hook_kind":"",
		  "tool":"review","tool_reason":"你刚让我写一版文案，我写好了，你审一下哪里要改",
		  "produce":{"kind":"artifact","payload":{
		    "kind":"draft","title":"接龙文案草稿",
		    "body":"大家好！我们班打算办一次旧物义卖。别人看到想要的，就回复「我要」。",
		    "guessed":["我猜了你们班群平时比较活跃"],
		    "admits":["「我要」这个说法可能太直接"],
		    "marks":[{"part":"开头","partNote":"第一句决定别人读不读下去",
		              "quote":"大家好！我们班打算办一次旧物义卖。",
		              "question":"这一句说清楚钱最后去哪了吗？"}],
		    "dimensions":[{"prompt":"最怕丢脸的那个同学读完会不会放心",
		                   "why":"他不放心，东西就拿不出来"}]}}}`))
	pid := newProjectViaAPI(t, h, cookie)

	rec := pblPost(t, h, cookie, "/api/v1/pbl/projects/"+pid+"/turn",
		`{"text":"你能先写一版接龙文案给我看看吗"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("turn = %d; body=%s", rec.Code, rec.Body)
	}

	// 成果在。
	rec = pblReq(t, h, cookie, "GET", "/api/v1/pbl/projects/"+pid+"/artifacts", "")
	var arts []struct {
		ID    string `json:"id"`
		Title string `json:"title"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &arts); err != nil {
		t.Fatalf("decode artifacts: %v — body=%s", err, rec.Body)
	}
	if len(arts) != 1 {
		t.Fatalf("成果没落库：%+v", arts)
	}

	// 🚨 关键：这一屏上有她真能动手的东西。
	rec = pblReq(t, h, cookie, "GET",
		"/api/v1/pbl/projects/"+pid+"/artifacts/"+arts[0].ID+"/review", "")
	var plan struct {
		Marks []struct {
			Quote    string `json:"quote"`
			Question string `json:"question"`
		} `json:"marks"`
		Dimensions []struct {
			Prompt string `json:"prompt"`
		} `json:"dimensions"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &plan); err != nil {
		t.Fatalf("decode review plan: %v — body=%s", err, rec.Body)
	}
	if len(plan.Marks) != 1 {
		t.Fatalf("没有划出来的句子，她只能从头读到尾然后点通过：%+v", plan)
	}
	if plan.Marks[0].Question == "" {
		t.Fatal("划了一句却没问题——那只是在把字标黄")
	}
	if len(plan.Dimensions) != 1 {
		t.Fatalf("没有必须留意的方面：%+v", plan.Dimensions)
	}
}

// 划出来却不带问题的那一条要被丢掉，但不能带着整份成果一起失败——她手里有东西
// 可审，比"审得很讲究"要紧。
func TestPblTurn_MarkWithNoQuestionIsDroppedNotFatal(t *testing.T) {
	h, cookie, _, _ := liteHandlerWithProvider(t, pblCoachSaying(
		`{"reply":"我写了一版。","hook":"","hook_kind":"",
		  "produce":{"kind":"artifact","payload":{
		    "kind":"draft","title":"草稿","body":"正文在这里。",
		    "marks":[{"quote":"正文在这里。","question":"   "},
		             {"quote":"正文在这里。","question":"这一句站得住吗？"}]}}}`))
	pid := newProjectViaAPI(t, h, cookie)

	rec := pblPost(t, h, cookie, "/api/v1/pbl/projects/"+pid+"/turn", `{"text":"写一版给我"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("turn = %d; body=%s", rec.Code, rec.Body)
	}
	rec = pblReq(t, h, cookie, "GET", "/api/v1/pbl/projects/"+pid+"/artifacts", "")
	var arts []struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &arts); err != nil {
		t.Fatalf("decode artifacts: %v", err)
	}
	if len(arts) != 1 {
		t.Fatalf("一条没写好的划线把整份成果拖没了：%+v", arts)
	}
	rec = pblReq(t, h, cookie, "GET",
		"/api/v1/pbl/projects/"+pid+"/artifacts/"+arts[0].ID+"/review", "")
	var plan struct {
		Marks []struct {
			Question string `json:"question"`
		} `json:"marks"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &plan); err != nil {
		t.Fatalf("decode review plan: %v", err)
	}
	if len(plan.Marks) != 1 || plan.Marks[0].Question != "这一句站得住吗？" {
		t.Fatalf("空问题那条没被丢掉：%+v", plan.Marks)
	}
}
