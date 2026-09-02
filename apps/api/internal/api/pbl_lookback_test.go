package api_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	. "mindimprint/api/internal/api"
)

type lookbackOut struct {
	ID      string `json:"id"`
	Section string `json:"section"`
	Prompt  string `json:"prompt"`
	Answer  string `json:"answer"`
}

func decodeLookback(t *testing.T, rec *httptest.ResponseRecorder) []lookbackOut {
	t.Helper()
	var out []lookbackOut
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode lookback: %v — body=%s", err, rec.Body)
	}
	return out
}

// 🚨 复盘按六段走，段里的问题由印记按这个项目真发生过的事现写。
//
// 产品负责人 2026-09-02 给了骨架，也说清了分工：段是我们定的（做了什么 / 感受
// 如何 / 印象最深 / 值得肯定 / 还能更好 / 和 AI 的协作），具体问题让 AI 按这个
// 结构写。两边都不能省——只有段就是空表单，只有问题就没有骨架。
func TestPblLookback_AsksInSectionsWrittenByYinji(t *testing.T) {
	h, cookie, _, _ := liteHandlerWithProvider(t, pblCoachSaying(`{"questions":[
	  {"section":"with_ai","prompt":"印记帮你整理数字那一段，你自己还会做吗？"},
	  {"section":"what","prompt":"你在食堂一共待了几天，看到的和你原来想的一样吗？"},
	  {"section":"moment","prompt":"哪一刻你觉得这件事真的有意思？"},
	  {"section":"随便","prompt":"这一条段名不对，应该被丢掉"}
	]}`))
	pid := newProjectViaAPI(t, h, cookie)

	got := decodeLookback(t, pblReq(t, h, cookie, "GET",
		"/api/v1/pbl/projects/"+pid+"/lookback", ""))
	if len(got) != 3 {
		t.Fatalf("生成了 %d 问，want 3（段名不对的那条要丢掉）：%+v", len(got), got)
	}
	// 🚨 顺序按六段来，不按模型吐出来的顺序——她走的顺序不该由模型决定。
	if got[0].Section != "what" || got[1].Section != "moment" || got[2].Section != "with_ai" {
		t.Fatalf("段的顺序不对：%v / %v / %v", got[0].Section, got[1].Section, got[2].Section)
	}
	if got[0].Prompt == "" {
		t.Fatal("问题是空的")
	}
}

// 🚨 写不出问题就报错，不兜底成一份通用问卷。她会照着答完，然后以为自己复盘
// 过了——那比没有复盘更糟。
func TestPblLookback_FailureSurfaces(t *testing.T) {
	h, cookie, _, _ := liteHandlerWithProvider(t, pblCoachSaying(`{"questions":[]}`))
	pid := newProjectViaAPI(t, h, cookie)
	rec := pblReq(t, h, cookie, "GET", "/api/v1/pbl/projects/"+pid+"/lookback", "")
	if rec.Code != http.StatusBadGateway {
		t.Fatalf("status = %d, want 502; body=%s", rec.Code, rec.Body)
	}
}

// 🚨 只生成一次。再生成一遍会把她答过的冲掉，而复盘本来就是隔几天回来慢慢写的。
func TestPblLookback_GeneratedOnceAndKeepsHerAnswers(t *testing.T) {
	h, cookie, _, _ := liteHandlerWithProvider(t, pblCoachSaying(`{"questions":[
	  {"section":"what","prompt":"这个项目你实际做了哪几件事？"}]}`))
	pid := newProjectViaAPI(t, h, cookie)
	url := "/api/v1/pbl/projects/" + pid + "/lookback"

	first := decodeLookback(t, pblReq(t, h, cookie, "GET", url, ""))
	if len(first) == 0 {
		t.Fatal("一问都没生成")
	}
	if rec := pblReq(t, h, cookie, "PATCH", url+"/"+first[0].ID,
		`{"answer":"最难的是承认第一个问题问错了"}`); rec.Code != http.StatusOK {
		t.Fatalf("answer = %d; body=%s", rec.Code, rec.Body)
	}

	second := decodeLookback(t, pblReq(t, h, cookie, "GET", url, ""))
	if len(second) != len(first) {
		t.Fatalf("第二次打开变成了 %d 问（原来 %d 问）——重新生成了", len(second), len(first))
	}
	if second[0].ID != first[0].ID || second[0].Answer == "" {
		t.Fatalf("她答过的被冲掉了：%+v", second[0])
	}
}

func TestPblLookback_OtherStudentGets404(t *testing.T) {
	h, cookie, _, pool := liteHandlerWithProvider(t, pblCoachSaying(`{"questions":[
	  {"section":"what","prompt":"做了什么？"}]}`))
	pid := newProjectViaAPI(t, h, cookie)

	otherID := createStudent(t, pool, SeedSchoolID, "other-lookback@demo.local")
	other := signInAs(t, pool, otherID)

	if rec := pblReq(t, h, other, "GET", "/api/v1/pbl/projects/"+pid+"/lookback", ""); rec.Code != http.StatusNotFound {
		t.Fatalf("foreign lookback = %d, want 404", rec.Code)
	}
}

/* ── 上线之后 ───────────────────────────────────────────────────────────── */

// 🚨 一条数据长出一轮新的思考——这一下就是维持和归档的分界。
// 「so one project may have several sessions」
func TestPblKeep_AnEntryGrowsANewSession(t *testing.T) {
	h, cookie, _, _ := liteHandlerWithProvider(t, nil)
	pid := newProjectViaAPI(t, h, cookie)
	url := "/api/v1/pbl/projects/" + pid + "/keep"

	rec := pblPost(t, h, cookie, url, `{"kind":"stat","body":"这周有 12 个人打开过","stage":"observe"}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("add entry = %d; body=%s", rec.Code, rec.Body)
	}
	var entry struct {
		ID        string  `json:"id"`
		SessionID *string `json:"sessionId"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &entry)
	if entry.SessionID != nil {
		t.Fatal("刚记下的一条不该已经有一轮思考")
	}

	rec = pblPost(t, h, cookie, url+"/"+entry.ID+"/session", "")
	if rec.Code != http.StatusCreated {
		t.Fatalf("open session = %d; body=%s", rec.Code, rec.Body)
	}
	var opened struct {
		SessionID string `json:"sessionId"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &opened)
	if opened.SessionID == "" {
		t.Fatal("没开出会话来")
	}

	// 再点一次应该回到同一轮，而不是又开一条平行的线。
	rec = pblPost(t, h, cookie, url+"/"+entry.ID+"/session", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("second open = %d, want 200", rec.Code)
	}
	var again struct {
		SessionID string `json:"sessionId"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &again)
	if again.SessionID != opened.SessionID {
		t.Fatalf("又开了一轮：%s vs %s", again.SessionID, opened.SessionID)
	}

	// 这一轮要真的在项目的会话列表里。
	rec = pblReq(t, h, cookie, "GET", "/api/v1/pbl/projects/"+pid+"/sessions", "")
	var sessions []struct {
		ID   string `json:"id"`
		Kind string `json:"kind"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &sessions)
	var found bool
	for _, s := range sessions {
		if s.ID == opened.SessionID && s.Kind == "keeping" {
			found = true
		}
	}
	if !found {
		t.Fatalf("开出来的那一轮不在会话列表里：%+v", sessions)
	}
}

func TestPblKeep_RejectsJunk(t *testing.T) {
	h, cookie, _, _ := liteHandlerWithProvider(t, nil)
	pid := newProjectViaAPI(t, h, cookie)
	url := "/api/v1/pbl/projects/" + pid + "/keep"

	for _, body := range []string{
		`{"kind":"随便","body":"x","stage":"observe"}`,
		`{"kind":"stat","body":"   ","stage":"observe"}`,
	} {
		if rec := pblPost(t, h, cookie, url, body); rec.Code != http.StatusBadRequest {
			t.Fatalf("add %s = %d, want 400", body, rec.Code)
		}
	}
	// 认不出的阶段退回"收集数据"，而不是报错——阶段是提示，不是门槛。
	if rec := pblPost(t, h, cookie, url,
		`{"kind":"thought","body":"也许该换个标题","stage":"什么阶段"}`); rec.Code != http.StatusCreated {
		t.Fatalf("unknown stage = %d, want 201", rec.Code)
	}
}
