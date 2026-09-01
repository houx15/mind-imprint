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
	rec := postPblProject(t, h, cookie, "   ")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body=%s", rec.Code, rec.Body)
	}
}

// spec §4 — 第一个项目就是她自己的主页，不管她在框里写了什么。
//
// Provider is nil here on purpose: if the handler consulted the model for a
// first project, this test would fail with a nil-provider panic or a 502. Its
// passing is the proof that we do not spend a token to be overruled.
func TestCreatePblProject_FirstIsWebsiteWithoutAModelCall(t *testing.T) {
	h, cookie, _, _ := liteHandlerWithProvider(t, nil)
	rec := postPblProject(t, h, cookie, "我想弄明白我们学校的剩饭到底去哪了")
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201; body=%s", rec.Code, rec.Body)
	}
	got := decodePblProject(t, rec)
	if got["kind"] != "website" {
		t.Fatalf("kind = %v, want \"website\" for a first project", got["kind"])
	}
	if got["status"] != "talking" {
		t.Fatalf("status = %v, want \"talking\"", got["status"])
	}
	if got["name"] != "" {
		t.Fatalf("name = %v, want empty — she names it in the modal", got["name"])
	}
	if got["idea"] != "我想弄明白我们学校的剩饭到底去哪了" {
		t.Fatalf("idea = %v, want it echoed back verbatim", got["idea"])
	}
}

// 第二个项目起，才轮到分类器说话。
func TestCreatePblProject_SecondUsesClassifier(t *testing.T) {
	h, cookie, _, _ := liteHandlerWithProvider(t, classifyStub(`{"kind":"investigation"}`))

	if rec := postPblProject(t, h, cookie, "先做个主页"); rec.Code != http.StatusCreated {
		t.Fatalf("first create = %d; body=%s", rec.Code, rec.Body)
	}
	rec := postPblProject(t, h, cookie, "我想去问问食堂阿姨每天剩多少")
	if rec.Code != http.StatusCreated {
		t.Fatalf("second create = %d, want 201; body=%s", rec.Code, rec.Body)
	}
	if got := decodePblProject(t, rec); got["kind"] != "investigation" {
		t.Fatalf("kind = %v, want \"investigation\" from the classifier", got["kind"])
	}
}

// 判不出来就报错。绝不静默兜底成一个看起来合理的类型——那会给她的项目挂上
// 一个错的、看不见的标签，并且把坏掉的分类器藏起来。
func TestCreatePblProject_ClassifyFailureSurfaces(t *testing.T) {
	h, cookie, _, _ := liteHandlerWithProvider(t, classifyStub(`{"kind":"podcast"}`))

	if rec := postPblProject(t, h, cookie, "先做个主页"); rec.Code != http.StatusCreated {
		t.Fatalf("first create = %d; body=%s", rec.Code, rec.Body)
	}
	rec := postPblProject(t, h, cookie, "我想做个播客")
	if rec.Code != http.StatusBadGateway {
		t.Fatalf("status = %d, want 502; body=%s", rec.Code, rec.Body)
	}
	if strings.Contains(rec.Body.String(), "research") {
		t.Fatalf("a fallback kind leaked into the error body: %s", rec.Body)
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
