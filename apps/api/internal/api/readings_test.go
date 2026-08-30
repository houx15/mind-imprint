package api_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	. "mindimprint/api/internal/api"
	"mindimprint/api/internal/store/sqlc"
)

// createReadingAtom returns a new reading's atom id. Shared by Tasks 3-8.
func createReadingAtom(t *testing.T, h http.Handler, cookie *http.Cookie) string {
	t.Helper()
	rec := httptest.NewRecorder()
	body := strings.NewReader(`{"title":"一篇文章","lang":"zh"}`)
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("POST", "/api/v1/readings", body), cookie))
	if rec.Code != http.StatusCreated {
		t.Fatalf("create reading = %d; body=%s", rec.Code, rec.Body)
	}
	var out struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil || out.ID == "" {
		t.Fatalf("decode id: %v — body=%s", err, rec.Body)
	}
	return out.ID
}

func TestCreateReading_CreatesAtomAndReading(t *testing.T) {
	h, cookie, q, _ := liteHandler(t)
	id := createReadingAtom(t, h, cookie)

	a, err := q.GetAtom(t.Context(), mustUUID(id))
	if err != nil {
		t.Fatalf("GetAtom: %v", err)
	}
	if a.Kind != "reading" {
		t.Fatalf("atom kind = %q, want \"reading\"", a.Kind)
	}
	if _, err := q.GetReading(t.Context(), a.ID); err != nil {
		t.Fatalf("GetReading: %v", err)
	}
}

func TestListReadings_OnlyMineNewestFirst(t *testing.T) {
	h, cookie, _, _ := liteHandler(t)
	createReadingAtom(t, h, cookie)
	createReadingAtom(t, h, cookie)

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("GET", "/api/v1/readings", nil), cookie))
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /readings = %d, want 200; body=%s", rec.Code, rec.Body)
	}
	var out struct {
		Readings []struct {
			ID        string `json:"id"`
			HasSource bool   `json:"hasSource"`
		} `json:"readings"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode: %v — body=%s", err, rec.Body)
	}
	if len(out.Readings) != 2 {
		t.Fatalf("got %d readings, want 2", len(out.Readings))
	}
	if out.Readings[0].HasSource {
		t.Fatal("a brand-new reading reports hasSource=true; nothing has been pasted yet")
	}
}

func TestGetReading_UnknownIs404(t *testing.T) {
	h, cookie, _, _ := liteHandler(t)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(
		httptest.NewRequest("GET", "/api/v1/readings/00000000-0000-0000-0000-0000000009ff", nil), cookie))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("unknown reading = %d, want 404", rec.Code)
	}
}

func TestRenameReading(t *testing.T) {
	h, cookie, _, _ := liteHandler(t)
	id := createReadingAtom(t, h, cookie)

	rec := httptest.NewRecorder()
	body := strings.NewReader(`{"title":"新标题"}`)
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("PATCH", "/api/v1/readings/"+id, body), cookie))
	if rec.Code != http.StatusOK {
		t.Fatalf("PATCH = %d, want 200; body=%s", rec.Code, rec.Body)
	}
	var out struct {
		Title string `json:"title"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil || out.Title != "新标题" {
		t.Fatalf("title = %q, want 新标题 (err=%v)", out.Title, err)
	}
}

// TestEditionGate_LiteKeepsSharedRoutes — the 127-route proOnly sweep (Task 2)
// must not have swallowed routes BOTH editions need. /auth/me and
// /evaluation-reports are deliberately NOT under /api/v1/projects and must
// stay reachable from a lite account.
func TestEditionGate_LiteKeepsSharedRoutes(t *testing.T) {
	h, cookie, _, _ := liteHandler(t)

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("GET", "/api/v1/auth/me", nil), cookie))
	if rec.Code != http.StatusOK {
		t.Fatalf("lite account GET /auth/me = %d, want 200; body=%s", rec.Code, rec.Body)
	}

	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("GET", "/api/v1/evaluation-reports", nil), cookie))
	if rec.Code != http.StatusOK {
		t.Fatalf("lite account GET /evaluation-reports = %d, want 200; body=%s", rec.Code, rec.Body)
	}
}

// TestLoadOwnedReadingAtom_WrongKindIs404 — loadOwnedReadingAtom is the
// authorization chokepoint every reading handler (and every later task)
// funnels through. atom.kind's CHECK constraint also permits 'writing'; an
// atom of that kind must be unreachable through a reading route, 404 exactly
// like a missing id — never leak that the id exists but is the wrong kind.
func TestLoadOwnedReadingAtom_WrongKindIs404(t *testing.T) {
	h, cookie, q, _ := liteHandler(t)

	at, err := q.CreateAtom(context.Background(), sqlc.CreateAtomParams{Kind: "writing", UserID: SeedUserID})
	if err != nil {
		t.Fatalf("create writing atom: %v", err)
	}

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("GET", "/api/v1/readings/"+at.ID.String(), nil), cookie))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("GET reading route on a writing atom = %d, want 404; body=%s", rec.Code, rec.Body)
	}
	if !strings.Contains(rec.Body.String(), `"code":"not_found"`) {
		t.Fatalf("body missing not_found code — got %s", rec.Body)
	}
}

// TestLoadOwnedReadingAtom_WrongOwnerIs404 — a reading atom owned by a
// different student in the same school must be indistinguishable from one
// that does not exist: 404, never 403.
func TestLoadOwnedReadingAtom_WrongOwnerIs404(t *testing.T) {
	h, cookie, q, pool := liteHandler(t)

	otherID := createStudent(t, pool, SeedSchoolID, "other-reader@demo.local")
	at, err := q.CreateAtom(context.Background(), sqlc.CreateAtomParams{Kind: "reading", UserID: otherID})
	if err != nil {
		t.Fatalf("create other's atom: %v", err)
	}
	if _, err := q.CreateReading(context.Background(), sqlc.CreateReadingParams{
		AtomID: at.ID, Title: "别人的文章", Lang: "zh",
	}); err != nil {
		t.Fatalf("create other's reading: %v", err)
	}

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("GET", "/api/v1/readings/"+at.ID.String(), nil), cookie))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("GET another student's reading = %d, want 404; body=%s", rec.Code, rec.Body)
	}
}

// TestEditionGate_LiteBlocksDeepProjectRoutes — the proOnly gate must reach
// past GET /api/v1/projects itself and cover deep sub-routes too. Also
// asserts the JSON error code is the gate's own "not_found", not merely a
// coincidental 404 that a route-registration mistake could also produce.
func TestEditionGate_LiteBlocksDeepProjectRoutes(t *testing.T) {
	h, cookie, _, _ := liteHandler(t)

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(
		httptest.NewRequest("GET", "/api/v1/projects/"+seedProjectID+"/plan", nil), cookie))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("lite account GET .../plan = %d, want 404; body=%s", rec.Code, rec.Body)
	}
	if !strings.Contains(rec.Body.String(), `"code":"not_found"`) {
		t.Fatalf("body missing not_found code — got %s", rec.Body)
	}
}

// 完成这篇不再问她要一段总结。
//
// 这条测试原来叫 TestFinishReading_RequiresTakeaway，钉的是相反的行为：空着
// 完成 → 400 missing_takeaway。那道门槛在当时是对的——一次阅读留下的全部记录
// 就是那一个输入框，空着完成等于没读。现在不是了：带读把她走过的每一步、她
// 指出的每一句、每张透镜的结论都落成了行，报告就是从这些生成的。
//
//	> we have give abundant steps for the reading. so we don't need to ask
//	> student to enter the form again.
func TestFinishReading_NeedsNoTakeaway(t *testing.T) {
	h, cookie, _, _ := liteHandler(t)
	id := createReadingAtom(t, h, cookie)

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("POST",
		"/api/v1/readings/"+id+"/finish", strings.NewReader("{}")), cookie))
	if rec.Code != http.StatusOK {
		t.Fatalf("finish without takeaway = %d, want 200; body=%s", rec.Code, rec.Body)
	}
	// …and it really is finished, not merely un-refused: the whole point is
	// that she lands on the report next.
	if !strings.Contains(rec.Body.String(), `"status":"finished"`) {
		t.Fatalf("reading did not come back finished — got %s", rec.Body)
	}
}

func TestFinishReading_IsIdempotent(t *testing.T) {
	h, cookie, _, _ := liteHandler(t)
	id := createReadingAtom(t, h, cookie)

	putTakeaway := httptest.NewRequest("PUT", "/api/v1/readings/"+id+"/takeaway",
		strings.NewReader(`{"text":"我的收获。"}`))
	h.ServeHTTP(httptest.NewRecorder(), withCookie(putTakeaway, cookie))

	for i := 0; i < 2; i++ {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, withCookie(httptest.NewRequest("POST",
			"/api/v1/readings/"+id+"/finish", strings.NewReader("{}")), cookie))
		if rec.Code != http.StatusOK {
			t.Fatalf("finish #%d = %d, want 200; body=%s", i+1, rec.Code, rec.Body)
		}
	}
}
