package api_test

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"testing"

	. "mindimprint/api/internal/api"
	"mindimprint/api/internal/store/sqlc"
)

// TestDemoProjectReadOnly verifies the loadOwnedProject write-guard: a project
// flagged is_demo is readable (GET) but rejects any mutation (403
// demo_readonly), while an ordinary project owned by the same user still
// accepts the same write. SeedUserID (…003) is NOT the seeded demo owner
// (…0101 belongs to Phoebe/…003 too in some fixtures, but we don't rely on
// that) — instead we mint both projects directly so ownership is unambiguous.
func TestDemoProjectReadOnly(t *testing.T) {
	pool := newAPITestPool(t)
	h := newTestAPI(pool).Handler()
	cookie := signInSeed(t, pool)
	q := sqlc.New(pool)
	ctx := t.Context()

	normal, err := q.CreateProject(ctx, sqlc.CreateProjectParams{
		UserID: SeedUserID, Qualification: "IB", Title: "普通项目", BoardCfgVer: 1,
	})
	if err != nil {
		t.Fatalf("create normal project: %v", err)
	}

	demo, err := q.CreateProject(ctx, sqlc.CreateProjectParams{
		UserID: SeedUserID, Qualification: "IB", Title: "演示项目", BoardCfgVer: 1,
	})
	if err != nil {
		t.Fatalf("create demo project: %v", err)
	}
	if _, err := pool.Exec(ctx, `UPDATE project SET is_demo = true WHERE id = $1`, demo.ID); err != nil {
		t.Fatalf("flag is_demo: %v", err)
	}

	// GET the demo workspace → 200, isDemo: true.
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, withCookie(httptest.NewRequest("GET", "/api/v1/projects/"+demo.ID.String(), nil), cookie))
	if rr.Code != 200 {
		t.Fatalf("GET demo: want 200, got %d — %s", rr.Code, rr.Body.String())
	}
	var proj struct {
		IsDemo bool `json:"isDemo"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &proj); err != nil {
		t.Fatalf("decode GET demo: %v", err)
	}
	if !proj.IsDemo {
		t.Fatalf("GET demo workspace: want isDemo=true, got body=%s", rr.Body.String())
	}

	// PATCH (rename) the demo project → 403 demo_readonly.
	renameBody, _ := json.Marshal(map[string]string{"title": "改名"})
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, withCookie(httptest.NewRequest("PATCH", "/api/v1/projects/"+demo.ID.String(), bytes.NewReader(renameBody)), cookie))
	if rr.Code != 403 {
		t.Fatalf("PATCH demo: want 403, got %d — %s", rr.Code, rr.Body.String())
	}
	var errBody struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &errBody); err != nil {
		t.Fatalf("decode PATCH demo error: %v", err)
	}
	if errBody.Error.Code != "demo_readonly" {
		t.Fatalf("PATCH demo: want code=demo_readonly, got %q — %s", errBody.Error.Code, rr.Body.String())
	}

	// Same write on the normal project → 200 (guard is scoped to is_demo).
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, withCookie(httptest.NewRequest("PATCH", "/api/v1/projects/"+normal.ID.String(), bytes.NewReader(renameBody)), cookie))
	if rr.Code != 200 {
		t.Fatalf("PATCH normal: want 200, got %d — %s", rr.Code, rr.Body.String())
	}
}
