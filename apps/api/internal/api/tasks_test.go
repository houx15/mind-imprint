package api_test

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"

	. "mindimprint/api/internal/api"
	"mindimprint/api/internal/store/sqlc"
)

func TestTasksCRUD(t *testing.T) {
	pool := newAPITestPool(t)
	h := New(Deps{Queries: sqlc.New(pool), Pool: pool}).Handler()
	cookie := signInSeed(t, pool)

	// --- Empty list returns {"tasks":[]} ---
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, withCookie(httptest.NewRequest("GET", "/api/v1/tasks", nil), cookie))
	if rr.Code != 200 {
		t.Fatalf("empty list: want 200, got %d — body: %s", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), `"tasks":[]`) {
		t.Fatalf("empty list: want tasks:[], got %s", rr.Body.String())
	}

	// --- Create task ---
	rr = httptest.NewRecorder()
	body := `{"title":"中国是否让地球更可持续？","seed":"https://example.com/article"}`
	h.ServeHTTP(rr, withCookie(httptest.NewRequest("POST", "/api/v1/tasks", strings.NewReader(body)), cookie))
	if rr.Code != 201 {
		t.Fatalf("create: want 201, got %d — body: %s", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), `"title":"中国是否让地球更可持续？"`) {
		t.Fatalf("create: response missing title — %s", rr.Body.String())
	}

	// --- Create with missing title returns 400 ---
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, withCookie(httptest.NewRequest("POST", "/api/v1/tasks", strings.NewReader(`{"title":""}`)), cookie))
	if rr.Code != 400 {
		t.Fatalf("create empty title: want 400, got %d — body: %s", rr.Code, rr.Body.String())
	}

	// --- List now has 1 task ---
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, withCookie(httptest.NewRequest("GET", "/api/v1/tasks", nil), cookie))
	if rr.Code != 200 {
		t.Fatalf("list after create: want 200, got %d", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), `"title":"中国是否让地球更可持续？"`) {
		t.Fatalf("list after create: missing task — %s", rr.Body.String())
	}

	// --- GET /tasks/{id} with a random UUID returns 404 ---
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, withCookie(httptest.NewRequest("GET", "/api/v1/tasks/"+uuid.NewString(), nil), cookie))
	if rr.Code != 404 {
		t.Fatalf("unknown id: want 404, got %d — body: %s", rr.Code, rr.Body.String())
	}

	// --- GET /tasks/{id} with invalid UUID returns 404 ---
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, withCookie(httptest.NewRequest("GET", "/api/v1/tasks/not-a-uuid", nil), cookie))
	if rr.Code != 404 {
		t.Fatalf("invalid uuid: want 404, got %d — body: %s", rr.Code, rr.Body.String())
	}
}
