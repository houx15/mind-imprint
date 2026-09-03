package api_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	. "mindimprint/api/internal/api"
	"mindimprint/api/internal/gateway"
)

// classifyStub returns a canned model reply for pbl.DetectKind's gateway.Collect
// call — the same scripted-provider shape reviewStubProvider uses.
func classifyStub(reply string) gateway.Provider {
	return gateway.NewStubProvider([]gateway.StreamEvent{
		{Kind: gateway.EventTextDelta, TextDelta: reply},
		{Kind: gateway.EventUsage, Usage: &gateway.ChatUsage{InputTokens: 30, OutputTokens: 8}},
		{Kind: gateway.EventDone, StopReason: gateway.StopStop},
	})
}

func postPblProject(t *testing.T, h http.Handler, cookie *http.Cookie, idea string) *httptest.ResponseRecorder {
	t.Helper()
	body, err := json.Marshal(map[string]string{"idea": idea})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(
		httptest.NewRequest("POST", "/api/v1/pbl/projects", strings.NewReader(string(body))), cookie))
	return rec
}

func decodePblProject(t *testing.T, rec *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var out map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode: %v — body=%s", err, rec.Body)
	}
	return out
}

// 空输入框是误点，不是项目。
func TestCreatePblProject_RejectsEmptyIdea(t *testing.T) {
	h, cookie, _, _ := liteHandler(t)
	openSiteGate(t, h, cookie) // §4：主页发布之前，自由项目一律 409
	rec := postPblProject(t, h, cookie, "   ")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body=%s", rec.Code, rec.Body)
	}
}

// 🚨 建项目不调模型，也不判类别。
//
// 产品负责人 2026-09-02：「neither should we decide the category of a project
// then.」她刚写下一句话，自己都还没想清楚要做什么。
//
// Provider 传 nil 是这条测试的全部力量：只要 handler 还去问模型，这里就会
// panic 或者 502。它通过，就证明这条路径一次模型调用都没有——顺带也保证她
// 建项目时不可能再撞上"接口错误"。
func TestCreatePblProject_NoModelCallAndNoCategory(t *testing.T) {
	h, cookie, _, _ := liteHandlerWithProvider(t, nil)
	openSiteGate(t, h, cookie) // §4：主页发布之前，自由项目一律 409
	rec := postPblProject(t, h, cookie, "我想弄明白我们学校的剩饭到底去哪了")
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201; body=%s", rec.Code, rec.Body)
	}
	got := decodePblProject(t, rec)
	if got["kind"] != "" {
		t.Fatalf("kind = %v, want \"\" —— 类别由她自己选", got["kind"])
	}
	if got["status"] != "talking" {
		t.Fatalf("status = %v, want \"talking\"", got["status"])
	}
	// 名字先给一个短的，页头才放得下；她随时能改。
	name, _ := got["name"].(string)
	if name == "" {
		t.Fatal("新项目应该先有一个名字")
	}
	if len([]rune(name)) > 15 {
		t.Fatalf("name = %q，太长，页头会被挤没", name)
	}
	// 🚨 她原来那句话一个字不改地留着——那是过程记录里唯一的"起点"。
	if got["idea"] != "我想弄明白我们学校的剩饭到底去哪了" {
		t.Fatalf("idea = %v, want it echoed back verbatim", got["idea"])
	}
}

// 第二个、第三个项目也一样，不问模型。
func TestCreatePblProject_StillNoModelOnLaterProjects(t *testing.T) {
	h, cookie, _, _ := liteHandlerWithProvider(t, nil)
	openSiteGate(t, h, cookie) // §4：主页发布之前，自由项目一律 409
	for i, idea := range []string{"先做个主页", "我想去问问食堂阿姨每天剩多少"} {
		rec := postPblProject(t, h, cookie, idea)
		if rec.Code != http.StatusCreated {
			t.Fatalf("create #%d = %d; body=%s", i+1, rec.Code, rec.Body)
		}
		if got := decodePblProject(t, rec); got["kind"] != "" {
			t.Fatalf("create #%d kind = %v, want empty", i+1, got["kind"])
		}
	}
}

// 空看板要是 []，不是 null——null 会让前端直接崩。
func TestListPblProjects_EmptyIsArrayNotNull(t *testing.T) {
	h, cookie, _, _ := liteHandler(t)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("GET", "/api/v1/pbl/projects", nil), cookie))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body)
	}
	if strings.TrimSpace(rec.Body.String()) != "[]" {
		t.Fatalf("body = %s, want []", rec.Body)
	}
}

// 别人的项目，对她来说应该和不存在没有区别。
func TestPatchPblProject_OtherStudentGets404(t *testing.T) {
	h, cookie, _, pool := liteHandlerWithProvider(t, nil)
	openSiteGate(t, h, cookie) // §4：主页发布之前，自由项目一律 409
	rec := postPblProject(t, h, cookie, "做一个记录校园植物的网站")
	if rec.Code != http.StatusCreated {
		t.Fatalf("create = %d; body=%s", rec.Code, rec.Body)
	}
	id, _ := decodePblProject(t, rec)["id"].(string)

	otherID := createStudent(t, pool, SeedSchoolID, "other-pbl@demo.local")
	otherCookie := signInAs(t, pool, otherID)

	rec2 := httptest.NewRecorder()
	h.ServeHTTP(rec2, withCookie(httptest.NewRequest("PATCH", "/api/v1/pbl/projects/"+id,
		strings.NewReader(`{"name":"偷来的"}`)), otherCookie))
	if rec2.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404; body=%s", rec2.Code, rec2.Body)
	}
}

// 不认识的状态在 handler 就被拦下，不该变成一条 CHECK 约束的 500。
func TestPatchPblProject_RejectsUnknownStatus(t *testing.T) {
	h, cookie, _, _ := liteHandlerWithProvider(t, nil)
	openSiteGate(t, h, cookie) // §4：主页发布之前，自由项目一律 409
	rec := postPblProject(t, h, cookie, "做一个记录校园植物的网站")
	if rec.Code != http.StatusCreated {
		t.Fatalf("create = %d; body=%s", rec.Code, rec.Body)
	}
	id, _ := decodePblProject(t, rec)["id"].(string)

	rec2 := httptest.NewRecorder()
	h.ServeHTTP(rec2, withCookie(httptest.NewRequest("PATCH", "/api/v1/pbl/projects/"+id,
		strings.NewReader(`{"status":"done"}`)), cookie))
	if rec2.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body=%s", rec2.Code, rec2.Body)
	}
}

// 起名 + 选封面，一次 PATCH 落地。
func TestPatchPblProject_NameAndCover(t *testing.T) {
	h, cookie, _, _ := liteHandlerWithProvider(t, nil)
	openSiteGate(t, h, cookie) // §4：主页发布之前，自由项目一律 409
	rec := postPblProject(t, h, cookie, "做一个记录校园植物的网站")
	if rec.Code != http.StatusCreated {
		t.Fatalf("create = %d; body=%s", rec.Code, rec.Body)
	}
	id, _ := decodePblProject(t, rec)["id"].(string)

	rec2 := httptest.NewRecorder()
	h.ServeHTTP(rec2, withCookie(httptest.NewRequest("PATCH", "/api/v1/pbl/projects/"+id,
		strings.NewReader(`{"name":"校园植物图鉴","coverGround":"matcha","coverGlyph":"leaf"}`)), cookie))
	if rec2.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec2.Code, rec2.Body)
	}
	got := decodePblProject(t, rec2)
	if got["name"] != "校园植物图鉴" || got["coverGround"] != "matcha" || got["coverGlyph"] != "leaf" {
		t.Fatalf("patched = %v, want the name and cover she chose", got)
	}
}
