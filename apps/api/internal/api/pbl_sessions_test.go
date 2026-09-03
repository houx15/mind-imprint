package api_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	. "mindimprint/api/internal/api"
)

// openSession posts a session and returns the decoded body.
func openSession(t *testing.T, h http.Handler, c *http.Cookie, projectID, body string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest(
		"POST", "/api/v1/pbl/projects/"+projectID+"/sessions", strings.NewReader(body)), c))
	return rec
}

func sessionID(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()
	var out struct {
		ID    string `json:"id"`
		Depth int    `json:"depth"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil || out.ID == "" {
		t.Fatalf("decode session: %v — body=%s", err, rec.Body)
	}
	return out.ID
}

// newProject creates a project over HTTP and returns its id.
//
// 🚨 先走一遍 §4 那道门。自 2026-09-03 起，学生的第一个项目就是做她自己的主页，
// 主页发布之前 POST /pbl/projects 一律 409（见 pbl_site.go 的 siteGateOpen）。
// 所以「一个有第二个项目的学生」必然是一个主页已经在线上的学生——这些测试要的
// 就是那个学生，openSiteGate 把她放到那个位置上。
func newProjectViaAPI(t *testing.T, h http.Handler, c *http.Cookie) string {
	t.Helper()
	openSiteGate(t, h, c)
	rec := postPblProject(t, h, c, "我们学校每天剩好多饭")
	if rec.Code != http.StatusCreated {
		t.Fatalf("create project = %d; body=%s", rec.Code, rec.Body)
	}
	id, _ := decodePblProject(t, rec)["id"].(string)
	return id
}

// Nesting works to three levels and then says so, rather than flattening.
func TestPblSession_NestingStopsAtThree(t *testing.T) {
	h, cookie, _, _ := liteHandlerWithProvider(t, nil)
	pid := newProjectViaAPI(t, h, cookie)

	rec := openSession(t, h, cookie, pid, `{"kind":"free","anchorKind":"hook","question":"为什么剩的都是米饭？"}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("level 0 = %d; body=%s", rec.Code, rec.Body)
	}
	l0 := sessionID(t, rec)

	rec = openSession(t, h, cookie, pid, `{"kind":"free","parentId":"`+l0+`"}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("level 1 = %d; body=%s", rec.Code, rec.Body)
	}
	l1 := sessionID(t, rec)

	rec = openSession(t, h, cookie, pid, `{"kind":"free","parentId":"`+l1+`"}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("level 2 = %d; body=%s", rec.Code, rec.Body)
	}
	l2 := sessionID(t, rec)

	rec = openSession(t, h, cookie, pid, `{"kind":"free","parentId":"`+l2+`"}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("level 3 = %d, want 400 — the cap must be said out loud, not silently flattened", rec.Code)
	}
}

// A parent from another project is not a parent.
func TestPblSession_RejectsForeignParent(t *testing.T) {
	// A stub provider, because this is the one test here that needs a SECOND
	// project: her first is the website by rule and skips the classifier, and
	// every project after it consults the model.
	h, cookie, _, _ := liteHandlerWithProvider(t, classifyStub(`{"kind":"investigation"}`))
	a := newProjectViaAPI(t, h, cookie)
	b := newProjectViaAPI(t, h, cookie)

	rec := openSession(t, h, cookie, a, `{"kind":"free"}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("seed session = %d; body=%s", rec.Code, rec.Body)
	}
	foreign := sessionID(t, rec)

	rec = openSession(t, h, cookie, b, `{"kind":"free","parentId":"`+foreign+`"}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("cross-project parent = %d, want 400", rec.Code)
	}
}

// 🚨 The write-back gate: a session cannot close having recorded nothing.
func TestPblSession_CloseRequiresWriteBack(t *testing.T) {
	h, cookie, _, _ := liteHandlerWithProvider(t, nil)
	pid := newProjectViaAPI(t, h, cookie)
	sid := sessionID(t, openSession(t, h, cookie, pid, `{"kind":"free"}`))

	closeURL := "/api/v1/pbl/projects/" + pid + "/sessions/" + sid + "/close"

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("POST", closeURL,
		strings.NewReader(`{"takeaway":"   "}`)), cookie))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("empty takeaway = %d, want 400 — 方法论表演 is back", rec.Code)
	}

	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("POST", closeURL,
		strings.NewReader(`{"takeaway":"剩的主要是米饭，不是菜"}`)), cookie))
	if rec.Code != http.StatusOK {
		t.Fatalf("close with a takeaway = %d; body=%s", rec.Code, rec.Body)
	}

	// Closing twice is refused — the thread would gain a second write-back for
	// one conclusion.
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("POST", closeURL,
		strings.NewReader(`{"takeaway":"再来一次"}`)), cookie))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("second close = %d, want 400", rec.Code)
	}
}

// A typed session needs its own contract, not merely some prose.
func TestPblSession_TypedContractEnforced(t *testing.T) {
	h, cookie, _, _ := liteHandlerWithProvider(t, nil)
	pid := newProjectViaAPI(t, h, cookie)
	sid := sessionID(t, openSession(t, h, cookie, pid, `{"kind":"brainstorm"}`))
	closeURL := "/api/v1/pbl/projects/" + pid + "/sessions/" + sid + "/close"

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("POST", closeURL,
		strings.NewReader(`{"takeaway":"聊得挺好"}`)), cookie))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("brainstorm closed without a next_bet = %d, want 400", rec.Code)
	}

	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("POST", closeURL,
		strings.NewReader(`{"writeBack":{"next_bet":"先做一版只有入口提示的"}}`)), cookie))
	if rec.Code != http.StatusOK {
		t.Fatalf("brainstorm with a next_bet = %d; body=%s", rec.Code, rec.Body)
	}
}

// 🚨 A nested session's conclusion returns to its PARENT, not to the main
// thread. Skipping a level delivers a conclusion without its context.
func TestPblSession_WriteBackGoesToTheParentThread(t *testing.T) {
	h, cookie, _, _ := liteHandlerWithProvider(t, nil)
	pid := newProjectViaAPI(t, h, cookie)

	parent := sessionID(t, openSession(t, h, cookie, pid, `{"kind":"free"}`))
	child := sessionID(t, openSession(t, h, cookie, pid, `{"kind":"free","parentId":"`+parent+`"}`))

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("POST",
		"/api/v1/pbl/projects/"+pid+"/sessions/"+child+"/close",
		strings.NewReader(`{"takeaway":"米饭是分量问题，不是口味问题"}`)), cookie))
	if rec.Code != http.StatusOK {
		t.Fatalf("close child = %d; body=%s", rec.Code, rec.Body)
	}

	// The parent's thread has it…
	inParent := threadContents(t, h, cookie, pid, parent)
	if len(inParent) != 1 || !strings.Contains(inParent[0], "米饭是分量问题") {
		t.Fatalf("parent thread = %v, want the child's takeaway", inParent)
	}
	// …and the main thread does not.
	inMain := threadContents(t, h, cookie, pid, "")
	for _, m := range inMain {
		if strings.Contains(m, "米饭是分量问题") {
			t.Fatalf("a nested takeaway jumped to the main thread: %v", inMain)
		}
	}
}

func threadContents(t *testing.T, h http.Handler, c *http.Cookie, projectID, sessionID string) []string {
	t.Helper()
	url := "/api/v1/pbl/projects/" + projectID + "/thread"
	if sessionID != "" {
		url += "?session=" + sessionID
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("GET", url, nil), c))
	if rec.Code != http.StatusOK {
		t.Fatalf("GET thread = %d; body=%s", rec.Code, rec.Body)
	}
	var rows []struct {
		Content string `json:"content"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &rows); err != nil {
		t.Fatalf("decode thread: %v — body=%s", err, rec.Body)
	}
	out := make([]string, 0, len(rows))
	for _, r := range rows {
		out = append(out, r.Content)
	}
	return out
}

// Someone else's project is indistinguishable from one that does not exist.
func TestPblSession_OtherStudentGets404(t *testing.T) {
	h, cookie, _, pool := liteHandlerWithProvider(t, nil)
	pid := newProjectViaAPI(t, h, cookie)

	otherID := createStudent(t, pool, SeedSchoolID, "other-session@demo.local")
	other := signInAs(t, pool, otherID)

	rec := openSession(t, h, other, pid, `{"kind":"free"}`)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("foreign session open = %d, want 404", rec.Code)
	}
}
