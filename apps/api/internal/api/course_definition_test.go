package api_test

// course_definition_test.go — Course Runtime Slice 8: GET /api/v1/courses/{slug}
// /definition. A course WITH a stored 2.0 definition returns it (border-checked
// schemaVersion=="2.0"); a legacy course with none is 404; unauth is 401.

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"mindimprint/api/internal/agent"
	. "mindimprint/api/internal/api"
	"mindimprint/api/internal/store/sqlc"
)

// A minimal, border-valid CourseDefinition 2.0 document. The endpoint only
// checks schemaVersion; full structural validation is the frontend's job.
const testCourseDefinitionJSON = `{"schemaVersion":"2.0","course":{"id":"test-runtime-course","title":"Test Course","language":"en","estimatedMinutes":5,"objectives":[],"parts":[]}}`

// seedCourseWithDefinition upserts a bare course row under slug and attaches a
// CourseDefinition 2.0 document to it. Structure/render_cache are empty ({}) —
// a 2.0 course plays through the runtime, not the pre-rendered player.
func seedCourseWithDefinition(t *testing.T, pool *pgxpool.Pool, slug, definitionJSON string) {
	t.Helper()
	st := agent.NewSqlcAgentStore(sqlc.New(pool), pool)
	if err := st.UpsertCourse(context.Background(), agent.UpsertCourseInput{
		Slug: slug, Branch: "Runtime", Title: "Test Course", Blurb: "b", TimeLabel: "约 5 分钟",
		CardIDs: []string{}, Structure: []byte("{}"), RenderCache: []byte("{}"), StepCount: 0,
	}); err != nil {
		t.Fatalf("upsert course %s: %v", slug, err)
	}
	if err := st.SetCourseDefinition(context.Background(), slug, []byte(definitionJSON)); err != nil {
		t.Fatalf("set definition %s: %v", slug, err)
	}
}

func TestCourseDefinitionServed(t *testing.T) {
	pool := newAPITestPool(t)
	seedCourseWithDefinition(t, pool, "def-course", testCourseDefinitionJSON)
	h := New(Deps{Queries: sqlc.New(pool), Pool: pool}).Handler()
	cookie := signInSeed(t, pool)

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("GET", "/api/v1/courses/def-course/definition", nil), cookie))
	if rec.Code != http.StatusOK {
		t.Fatalf("definition: want 200, got %d %s", rec.Code, rec.Body)
	}
	var resp struct {
		Definition struct {
			SchemaVersion string `json:"schemaVersion"`
			Course        struct {
				ID string `json:"id"`
			} `json:"course"`
		} `json:"definition"`
		Hash string `json:"hash"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v — body %s", err, rec.Body)
	}
	if resp.Definition.SchemaVersion != "2.0" {
		t.Fatalf("schemaVersion = %q, want 2.0", resp.Definition.SchemaVersion)
	}
	if resp.Definition.Course.ID != "test-runtime-course" {
		t.Fatalf("course.id = %q, want test-runtime-course", resp.Definition.Course.ID)
	}
	// P2-08/D5: hash is a real sha256 hex digest of the STORED bytes — fetch
	// them the same way the handler does (Postgres's jsonb round-trip
	// reformats whitespace, so this must compare against the actual stored
	// bytes, not the literal Go string used to seed them).
	store := agent.NewSqlcAgentStore(sqlc.New(pool), pool)
	storedDef, _, err := store.GetCourseDefinition(context.Background(), "def-course")
	if err != nil {
		t.Fatalf("GetCourseDefinition: %v", err)
	}
	wantSum := sha256.Sum256(storedDef)
	wantHash := hex.EncodeToString(wantSum[:])
	if resp.Hash != wantHash {
		t.Fatalf("hash = %q, want sha256(stored bytes) = %q", resp.Hash, wantHash)
	}
	if len(resp.Hash) != 64 {
		t.Fatalf("hash %q is not a sha256 hex digest (want 64 hex chars, got %d)", resp.Hash, len(resp.Hash))
	}
}

// TestCourseDefinitionHashStable — the same stored bytes always hash the same
// (repeat GETs, and two DIFFERENT courses seeded with byte-identical
// definitions), and a definition with different content hashes differently —
// the whole point of the P2-08/D5 revision signal.
func TestCourseDefinitionHashStable(t *testing.T) {
	pool := newAPITestPool(t)
	seedCourseWithDefinition(t, pool, "def-course-a", testCourseDefinitionJSON)
	seedCourseWithDefinition(t, pool, "def-course-b", testCourseDefinitionJSON)
	otherJSON := `{"schemaVersion":"2.0","course":{"id":"test-runtime-course","title":"Test Course v2","language":"en","estimatedMinutes":5,"objectives":[],"parts":[]}}`
	seedCourseWithDefinition(t, pool, "def-course-c", otherJSON)
	h := New(Deps{Queries: sqlc.New(pool), Pool: pool}).Handler()
	cookie := signInSeed(t, pool)

	fetchHash := func(slug string) string {
		t.Helper()
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, withCookie(httptest.NewRequest("GET", "/api/v1/courses/"+slug+"/definition", nil), cookie))
		if rec.Code != http.StatusOK {
			t.Fatalf("definition %s: want 200, got %d %s", slug, rec.Code, rec.Body)
		}
		var resp struct {
			Hash string `json:"hash"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("decode %s: %v — body %s", slug, err, rec.Body)
		}
		return resp.Hash
	}

	hashA1 := fetchHash("def-course-a")
	hashA2 := fetchHash("def-course-a") // repeat GET: same bytes, same hash.
	hashB := fetchHash("def-course-b")  // different course, byte-identical definition: same hash.
	hashC := fetchHash("def-course-c")  // different content: different hash.

	if hashA1 != hashA2 {
		t.Fatalf("hash not stable across repeat GETs: %q vs %q", hashA1, hashA2)
	}
	if hashA1 != hashB {
		t.Fatalf("byte-identical definitions hashed differently: %q vs %q", hashA1, hashB)
	}
	if hashA1 == hashC {
		t.Fatalf("differing definitions hashed the same: %q", hashA1)
	}
}

// TestCourseDefinitionMissing404 — a legacy course (seeded with no 2.0
// definition) returns 404 from the definition endpoint, so the frontend routes
// it to the legacy player.
func TestCourseDefinitionMissing404(t *testing.T) {
	pool := newAPITestPool(t)
	seedAMidCourse(t, pool) // legacy course, no course_definition
	h := New(Deps{Queries: sqlc.New(pool), Pool: pool}).Handler()
	cookie := signInSeed(t, pool)

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("GET", "/api/v1/courses/a-mid/definition", nil), cookie))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("legacy course definition: want 404, got %d %s", rec.Code, rec.Body)
	}
}

func TestCourseDefinitionRequiresAuth(t *testing.T) {
	pool := newAPITestPool(t)
	seedCourseWithDefinition(t, pool, "def-course", testCourseDefinitionJSON)
	h := New(Deps{Queries: sqlc.New(pool), Pool: pool}).Handler()

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", "/api/v1/courses/def-course/definition", nil))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("unauth: want 401, got %d", rec.Code)
	}
}
