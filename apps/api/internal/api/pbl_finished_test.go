package api_test

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func patchProjectStatus(t *testing.T, h http.Handler, c *http.Cookie, projectID, status string) {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("PATCH", "/api/v1/pbl/projects/"+projectID,
		bytes.NewReader([]byte(`{"status":"`+status+`"}`))), c))
	if rec.Code != http.StatusOK {
		t.Fatalf("patch %s = %d body=%s", status, rec.Code, rec.Body)
	}
}

func TestPblFinishedAtStampedOnce(t *testing.T) {
	h, cookie, _, pool := liteHandler(t)
	projectID, _ := createPblProjectForTest(t, pool)
	read := func() *time.Time {
		var ts *time.Time
		if err := pool.QueryRow(context.Background(), `SELECT finished_at FROM pbl_project WHERE atom_id=$1`, projectID).Scan(&ts); err != nil {
			t.Fatal(err)
		}
		return ts
	}
	patchProjectStatus(t, h, cookie, projectID, "running")
	if read() != nil {
		t.Fatal("running stamped finished_at")
	}
	patchProjectStatus(t, h, cookie, projectID, "review")
	first := read()
	if first == nil {
		t.Fatal("review did not stamp finished_at")
	}
	patchProjectStatus(t, h, cookie, projectID, "running")
	patchProjectStatus(t, h, cookie, projectID, "keeping")
	if got := read(); got == nil || !got.Equal(*first) {
		t.Fatalf("finished_at changed: %v → %v", first, got)
	}
}
