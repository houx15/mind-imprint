package api_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	. "mindimprint/api/internal/api"
)

func importBody(rows []map[string]string) *bytes.Reader {
	b, _ := json.Marshal(map[string]any{"rows": rows})
	return bytes.NewReader(b)
}

func TestAdminImportCreatesStructureIdempotently(t *testing.T) {
	pool := newAPITestPool(t)
	h := New(DepsForTest(pool)).Handler()
	admin := signInAdmin(t, pool)

	rows := []map[string]string{
		{"class": "Imported A", "teacher_email": "ta@demo.local", "student_email": "s1@demo.local"},
		{"class": "Imported A", "student_email": "s2@demo.local"},
		{"class": "Imported B", "teacher_email": "tb@demo.local"},
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("POST", "/api/v1/admin/import", importBody(rows)), admin))
	if rec.Code != http.StatusCreated {
		t.Fatalf("import got %d body=%s", rec.Code, rec.Body)
	}
	var resp struct {
		Classes        []struct{ Name, JoinCode string } `json:"classes"`
		TeacherInvites []struct{ Email, Code string }    `json:"teacher_invites"`
	}
	json.Unmarshal(rec.Body.Bytes(), &resp)
	if len(resp.Classes) != 2 || len(resp.TeacherInvites) != 2 {
		t.Fatalf("want 2 classes + 2 invites, got %d/%d", len(resp.Classes), len(resp.TeacherInvites))
	}

	// Re-import the same rows → no duplicate classes (idempotent).
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("POST", "/api/v1/admin/import", importBody(rows)), admin))
	if rec.Code != http.StatusCreated {
		t.Fatalf("re-import got %d", rec.Code)
	}
	var count int
	pool.QueryRow(t.Context(), `SELECT count(*) FROM classes WHERE name LIKE 'Imported %'`).Scan(&count)
	if count != 2 {
		t.Fatalf("idempotency broken: %d classes", count)
	}
}

func TestAdminImportRejectsEmptyClass(t *testing.T) {
	pool := newAPITestPool(t)
	h := New(DepsForTest(pool)).Handler()
	rows := []map[string]string{{"class": "", "student_email": "x@demo.local"}}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("POST", "/api/v1/admin/import", importBody(rows)), signInAdmin(t, pool)))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("empty class got %d, want 400", rec.Code)
	}
	// Nothing was created (rolled back).
	var count int
	pool.QueryRow(t.Context(), `SELECT count(*) FROM classes WHERE name='' OR name IS NULL`).Scan(&count)
	if count != 0 {
		t.Fatal("rollback failed")
	}
}
