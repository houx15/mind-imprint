package api_test

// course_admin_read_test.go — the OSS_ADMIN_KEY-gated draft readback (G7): the
// course generator reads back a PREVIEW (draft) course with nothing but the
// bearer key — no admin session — where the student-facing definition route
// would 404 a preview course.

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"mindimprint/api/internal/agent"
	. "mindimprint/api/internal/api"
	"mindimprint/api/internal/store/sqlc"
)

const testAdminReadKey = "admin-read-key-abc"

func TestAdminDefinitionReadback_PreviewDraft(t *testing.T) {
	pool := newAPITestPool(t)
	// Seed a course + definition, then mark it preview (a draft students can't see).
	seedCourseWithDefinition(t, pool, "sandbox-draft", testCourseDefinitionJSON)
	st := agent.NewSqlcAgentStore(sqlc.New(pool), pool)
	if err := st.SetCourseStatusAndCover(context.Background(), "sandbox-draft", "preview", ""); err != nil {
		t.Fatalf("set preview: %v", err)
	}
	h := New(Deps{Queries: sqlc.New(pool), Pool: pool, OSSAdminKey: testAdminReadKey}).Handler()

	// With the bearer key: 200 + definition + status=preview (no session cookie).
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/v1/admin/courses/sandbox-draft/definition", nil)
	req.Header.Set("Authorization", "Bearer "+testAdminReadKey)
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("bearer readback: want 200, got %d %s", rec.Code, rec.Body)
	}
	var resp struct {
		Definition struct {
			SchemaVersion string `json:"schemaVersion"`
		} `json:"definition"`
		Hash   string `json:"hash"`
		Status string `json:"status"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v — body %s", err, rec.Body)
	}
	if resp.Definition.SchemaVersion != "2.0" {
		t.Fatalf("schemaVersion = %q", resp.Definition.SchemaVersion)
	}
	if resp.Status != "preview" {
		t.Fatalf("status = %q, want preview", resp.Status)
	}
	if len(resp.Hash) != 64 {
		t.Fatalf("hash %q not a sha256 hex digest", resp.Hash)
	}

	// No key → 401 (before any DB access).
	recNo := httptest.NewRecorder()
	h.ServeHTTP(recNo, httptest.NewRequest("GET", "/api/v1/admin/courses/sandbox-draft/definition", nil))
	if recNo.Code != http.StatusUnauthorized {
		t.Fatalf("no key: want 401, got %d %s", recNo.Code, recNo.Body)
	}

	// Wrong key → 401.
	recBad := httptest.NewRecorder()
	reqBad := httptest.NewRequest("GET", "/api/v1/admin/courses/sandbox-draft/definition", nil)
	reqBad.Header.Set("Authorization", "Bearer nope")
	h.ServeHTTP(recBad, reqBad)
	if recBad.Code != http.StatusUnauthorized {
		t.Fatalf("wrong key: want 401, got %d %s", recBad.Code, recBad.Body)
	}
}

// TestAdminCourseList_IncludesPreview — the bearer-gated list surfaces preview
// drafts (with status), unlike the student list which filters to published.
func TestAdminCourseList_IncludesPreview(t *testing.T) {
	pool := newAPITestPool(t)
	seedCourseWithDefinition(t, pool, "sandbox-draft-2", testCourseDefinitionJSON)
	st := agent.NewSqlcAgentStore(sqlc.New(pool), pool)
	if err := st.SetCourseStatusAndCover(context.Background(), "sandbox-draft-2", "preview", ""); err != nil {
		t.Fatalf("set preview: %v", err)
	}
	h := New(Deps{Queries: sqlc.New(pool), Pool: pool, OSSAdminKey: testAdminReadKey}).Handler()

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/v1/admin/courses", nil)
	req.Header.Set("Authorization", "Bearer "+testAdminReadKey)
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("admin list: want 200, got %d %s", rec.Code, rec.Body)
	}
	var resp struct {
		Courses []struct {
			Slug   string `json:"slug"`
			Status string `json:"status"`
		} `json:"courses"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v — body %s", err, rec.Body)
	}
	var found bool
	for _, c := range resp.Courses {
		if c.Slug == "sandbox-draft-2" && c.Status == "preview" {
			found = true
		}
	}
	if !found {
		t.Fatalf("admin list did not include the preview draft with status; got %s", rec.Body)
	}
}
