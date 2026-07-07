package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	. "mindimprint/api/internal/api"
	"mindimprint/api/internal/store/sqlc"
)

func TestCourseListGetProgress(t *testing.T) {
	pool := newAPITestPool(t)
	q := sqlc.New(pool)
	h := New(Deps{Queries: q, Pool: pool}).Handler()
	cookie := signInSeed(t, pool)

	// list
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("GET", "/api/v1/courses", nil), cookie))
	if rec.Code != http.StatusOK || !bytes.Contains(rec.Body.Bytes(), []byte("一条网络信息，该不该信")) {
		t.Fatalf("list: %d %s", rec.Code, rec.Body)
	}
	var listResp struct {
		Courses []struct {
			ID        string `json:"id"`
			StepCount int    `json:"step_count"`
		} `json:"courses"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &listResp)
	id := listResp.Courses[0].ID
	if listResp.Courses[0].StepCount == 0 {
		t.Fatalf("step_count 0")
	}

	// get (with steps)
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("GET", "/api/v1/courses/"+id, nil), cookie))
	if rec.Code != http.StatusOK || !bytes.Contains(rec.Body.Bytes(), []byte(`"steps"`)) || !bytes.Contains(rec.Body.Bytes(), []byte("challenge")) {
		t.Fatalf("get: %d %s", rec.Code, rec.Body)
	}

	// progress default (no row yet) → current_ordinal 0
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("GET", "/api/v1/courses/"+id+"/progress", nil), cookie))
	if rec.Code != http.StatusOK || !bytes.Contains(rec.Body.Bytes(), []byte(`"current_ordinal":0`)) {
		t.Fatalf("progress default: %d %s", rec.Code, rec.Body)
	}

	// put progress
	body, _ := json.Marshal(map[string]any{"current_ordinal": 2, "completed_ordinals": []int{0, 1}})
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("PUT", "/api/v1/courses/"+id+"/progress", bytes.NewReader(body)), cookie))
	if rec.Code != http.StatusOK || !bytes.Contains(rec.Body.Bytes(), []byte(`"current_ordinal":2`)) {
		t.Fatalf("put progress: %d %s", rec.Code, rec.Body)
	}

	// unknown course id → 404 on get
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("GET", "/api/v1/courses/00000000-0000-0000-0000-0000000000ff", nil), cookie))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("unknown course: want 404 got %d", rec.Code)
	}
	_ = context.Background()
}
