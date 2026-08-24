package api_test

// course_ship_test.go — Task 5 of the course authoring & publish lifecycle:
// POST /api/v1/admin/courses/{slug}/ship, the ONE preview -> published
// transition. Mirrors course_definition_admin_test.go's admin-key + DB
// harness. Deps.OSS is a concrete *oss.Service (not fakeable without a real
// bucket — see course_admin_test.go's own note), so the HTTP-level tests here
// run with no Voice/OSS configured (narration degrades to 0, exactly like
// course_admin_test.go's pattern) and assert the status/cover transition;
// the audio-generation plumbing itself — that agent.GenerateDefinitionAudio
// PutObjects a narration clip for the exact definition this handler fetches
// from the DB — is asserted directly against the stored definition with
// stubbed synth/store, the same "wiring level" split course_admin_test.go
// documents for GenerateCourseAudio.

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"mindimprint/api/internal/agent"
	. "mindimprint/api/internal/api"
	"mindimprint/api/internal/store/sqlc"
)

// stubShipSynth/stubShipStore are minimal local doubles for
// agent.CourseAudioSynth/CourseAudioStore — course_definition_audio_test.go's
// stubCourseAudioSynth/stubCourseAudioStore live in package agent (internal,
// unexported), so this package_test file defines its own.
type stubShipSynth struct{ calls int }

func (s *stubShipSynth) Synthesize(_ context.Context, _ string, _ float64) ([]byte, error) {
	s.calls++
	return []byte("fake-mp3"), nil
}
func (s *stubShipSynth) Voice() string { return "test-voice" }

type stubShipStore struct {
	puts []string // keys PutObject was called with
}

func (s *stubShipStore) Exists(context.Context, string) (bool, error) { return false, nil }
func (s *stubShipStore) PutObject(_ context.Context, key, _ string, _ []byte) error {
	s.puts = append(s.puts, key)
	return nil
}

// shipCourseDefinitionDoc is a border-valid CourseDefinition 2.0 document with
// one narration (text+audio) so GenerateDefinitionAudio has something to
// synthesize.
func shipCourseDefinitionDoc(id string) string {
	return `{"schemaVersion":"2.0","course":{"id":"` + id + `","title":"Ship Test Course","language":"en","estimatedMinutes":5,` +
		`"objectives":[],"parts":[{"slices":[{"narrations":[{"text":"Welcome.","audio":"n1.mp3"}]}]}]}}`
}

func postShip(h http.Handler, slug, body, key string) *httptest.ResponseRecorder {
	req := httptest.NewRequest("POST", "/api/v1/admin/courses/"+slug+"/ship", strings.NewReader(body))
	bearer(req, key)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

// TestCourseShipPublishesAndSetsCover asserts the ship endpoint flips a
// preview course to published and sets its cover in one call, verified by a
// fresh DB read (not just the response echo).
func TestCourseShipPublishesAndSetsCover(t *testing.T) {
	pool := newAPITestPool(t)
	q := sqlc.New(pool)
	h := New(Deps{Queries: q, Pool: pool, OSSAdminKey: testAdminKey}).Handler()

	slug := "ship-course"
	putRec := putDefinition(h, slug, putCourseDefinitionBody(shipCourseDefinitionDoc(slug), []string{"craap"}, "a shippable course"))
	if putRec.Code != http.StatusOK {
		t.Fatalf("precondition put: want 200 got %d %s", putRec.Code, putRec.Body)
	}

	status, err := agent.NewSqlcAgentStore(q, pool).CourseStatus(context.Background(), slug)
	if err != nil || status != "preview" {
		t.Fatalf("precondition: course status = %q, err %v, want preview", status, err)
	}

	rec := postShip(h, slug, `{"cover":"img:3"}`, testAdminKey)
	if rec.Code != http.StatusOK {
		t.Fatalf("ship: want 200 got %d %s", rec.Code, rec.Body)
	}
	var resp struct {
		Slug                string `json:"slug"`
		Status              string `json:"status"`
		NarrationsGenerated int    `json:"narrationsGenerated"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Slug != slug || resp.Status != "published" {
		t.Fatalf("resp = %+v, want slug=%s status=published", resp, slug)
	}
	// No Voice/OSS configured on this Deps -> audio generation degrades to 0,
	// per Global Constraints ("ship still flips status, never fails the
	// publish just because voice/OSS is unconfigured").
	if resp.NarrationsGenerated != 0 {
		t.Fatalf("narrationsGenerated = %d, want 0 (no Voice/OSS configured)", resp.NarrationsGenerated)
	}

	// Fresh DB read: status AND cover, not just the response echo.
	var dbStatus, dbCover string
	if err := pool.QueryRow(context.Background(), `SELECT status, cover FROM course WHERE slug = $1`, slug).Scan(&dbStatus, &dbCover); err != nil {
		t.Fatalf("read back course row: %v", err)
	}
	if dbStatus != "published" {
		t.Fatalf("db status = %q, want published", dbStatus)
	}
	if dbCover != "img:3" {
		t.Fatalf("db cover = %q, want img:3", dbCover)
	}
}

// TestCourseShipRejectsAmbiguousCover asserts a stock cover id and a generated-
// cover asset path cannot both be supplied — the request is rejected 400 and
// the course stays preview (never published on a bad request).
func TestCourseShipRejectsAmbiguousCover(t *testing.T) {
	pool := newAPITestPool(t)
	q := sqlc.New(pool)
	h := New(Deps{Queries: q, Pool: pool, OSSAdminKey: testAdminKey}).Handler()

	slug := "ship-ambiguous"
	if rec := putDefinition(h, slug, putCourseDefinitionBody(shipCourseDefinitionDoc(slug), []string{"craap"}, "c")); rec.Code != http.StatusOK {
		t.Fatalf("precondition put: %d %s", rec.Code, rec.Body)
	}

	rec := postShip(h, slug, `{"cover":"img:3","coverAssetPath":"cover/course-cover.webp"}`, testAdminKey)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("ship ambiguous: want 400 got %d %s", rec.Code, rec.Body)
	}
	if status, _ := agent.NewSqlcAgentStore(q, pool).CourseStatus(context.Background(), slug); status != "preview" {
		t.Fatalf("status = %q, want preview (unchanged by a rejected ship)", status)
	}
}

// TestCourseShipRejectsAssetPrefixInStockCover asserts the reserved "asset:"
// scheme cannot be smuggled through the stock `cover` field to bypass the
// existence/WebP gate — it is rejected 400 and the course stays preview.
func TestCourseShipRejectsAssetPrefixInStockCover(t *testing.T) {
	pool := newAPITestPool(t)
	q := sqlc.New(pool)
	h := New(Deps{Queries: q, Pool: pool, OSSAdminKey: testAdminKey}).Handler()

	slug := "ship-asset-smuggle"
	if rec := putDefinition(h, slug, putCourseDefinitionBody(shipCourseDefinitionDoc(slug), []string{"craap"}, "c")); rec.Code != http.StatusOK {
		t.Fatalf("precondition put: %d %s", rec.Code, rec.Body)
	}

	rec := postShip(h, slug, `{"cover":"asset:cover/course-cover.webp"}`, testAdminKey)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("stock cover with asset: prefix: want 400 got %d %s", rec.Code, rec.Body)
	}
	if status, _ := agent.NewSqlcAgentStore(q, pool).CourseStatus(context.Background(), slug); status != "preview" {
		t.Fatalf("status = %q, want preview (asset: must not publish via stock cover)", status)
	}
}

// TestCourseShipCoverAssetPathRequiresOSS asserts an asset-cover ship cannot
// publish when OSS is unconfigured — it cannot verify or later serve the object
// — and the course stays preview.
func TestCourseShipCoverAssetPathRequiresOSS(t *testing.T) {
	pool := newAPITestPool(t)
	q := sqlc.New(pool)
	h := New(Deps{Queries: q, Pool: pool, OSSAdminKey: testAdminKey}).Handler() // no OSS

	slug := "ship-asset-no-oss"
	if rec := putDefinition(h, slug, putCourseDefinitionBody(shipCourseDefinitionDoc(slug), []string{"craap"}, "c")); rec.Code != http.StatusOK {
		t.Fatalf("precondition put: %d %s", rec.Code, rec.Body)
	}

	rec := postShip(h, slug, `{"coverAssetPath":"cover/course-cover.webp"}`, testAdminKey)
	if rec.Code == http.StatusOK {
		t.Fatalf("asset-cover ship with no OSS must not succeed, got 200 %s", rec.Body)
	}
	if status, _ := agent.NewSqlcAgentStore(q, pool).CourseStatus(context.Background(), slug); status != "preview" {
		t.Fatalf("status = %q, want preview (not published without OSS)", status)
	}
}

// TestCourseShipGeneratesNarrationAudio asserts the definition this handler
// fetches from the DB (store.GetCourseDefinition, the exact bytes
// postCourseShip passes to agent.GenerateDefinitionAudio) produces a
// PutObject call for its narration when synth/store are configured — the
// piece Deps.OSS being a concrete *oss.Service prevents exercising through
// the live HTTP path (see file header).
func TestCourseShipGeneratesNarrationAudio(t *testing.T) {
	pool := newAPITestPool(t)
	q := sqlc.New(pool)
	h := New(Deps{Queries: q, Pool: pool, OSSAdminKey: testAdminKey}).Handler()

	slug := "ship-course-audio"
	putRec := putDefinition(h, slug, putCourseDefinitionBody(shipCourseDefinitionDoc(slug), []string{"craap"}, "a shippable course"))
	if putRec.Code != http.StatusOK {
		t.Fatalf("precondition put: want 200 got %d %s", putRec.Code, putRec.Body)
	}

	store := agent.NewSqlcAgentStore(q, pool)
	def, status, err := store.GetCourseDefinition(context.Background(), slug)
	if err != nil {
		t.Fatalf("GetCourseDefinition: %v", err)
	}
	if status != "preview" || len(def) == 0 {
		t.Fatalf("precondition: status=%q len(def)=%d, want preview / non-empty", status, len(def))
	}

	synth := &stubShipSynth{}
	audioStore := &stubShipStore{}
	n, err := agent.GenerateDefinitionAudio(context.Background(), synth, audioStore, slug, def)
	if err != nil {
		t.Fatalf("GenerateDefinitionAudio: %v", err)
	}
	if n != 1 {
		t.Fatalf("generated = %d, want 1", n)
	}
	wantKey := "courses/" + slug + "/n1.mp3"
	if len(audioStore.puts) != 1 || audioStore.puts[0] != wantKey {
		t.Fatalf("puts = %v, want [%s]", audioStore.puts, wantKey)
	}

	// Ship for real over HTTP too — status/cover flip is unaffected by
	// whether audio was generated out-of-band above.
	rec := postShip(h, slug, `{"cover":""}`, testAdminKey)
	if rec.Code != http.StatusOK {
		t.Fatalf("ship: want 200 got %d %s", rec.Code, rec.Body)
	}
}

// TestCourseShipUnauthorized asserts a missing or wrong admin key 401s before
// any DB work, and never publishes the course.
func TestCourseShipUnauthorized(t *testing.T) {
	pool := newAPITestPool(t)
	q := sqlc.New(pool)
	h := New(Deps{Queries: q, Pool: pool, OSSAdminKey: testAdminKey}).Handler()

	slug := "ship-course-unauth"
	putRec := putDefinition(h, slug, putCourseDefinitionBody(shipCourseDefinitionDoc(slug), []string{"craap"}, "b"))
	if putRec.Code != http.StatusOK {
		t.Fatalf("precondition put: want 200 got %d %s", putRec.Code, putRec.Body)
	}

	t.Run("missing bearer", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/api/v1/admin/courses/"+slug+"/ship", strings.NewReader(`{"cover":"img:1"}`))
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("missing bearer: want 401 got %d %s", rec.Code, rec.Body)
		}
	})

	t.Run("wrong bearer", func(t *testing.T) {
		rec := postShip(h, slug, `{"cover":"img:1"}`, "wrong-key")
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("wrong bearer: want 401 got %d %s", rec.Code, rec.Body)
		}
	})

	status, err := agent.NewSqlcAgentStore(q, pool).CourseStatus(context.Background(), slug)
	if err != nil || status != "preview" {
		t.Fatalf("course status after unauthorized attempts = %q, err %v, want still preview", status, err)
	}
}

// TestCourseShipUnknownSlug asserts shipping a slug with no stored 2.0
// definition (never PUT) 404s.
func TestCourseShipUnknownSlug(t *testing.T) {
	pool := newAPITestPool(t)
	h := New(Deps{Queries: sqlc.New(pool), Pool: pool, OSSAdminKey: testAdminKey}).Handler()

	rec := postShip(h, "no-such-course", `{"cover":"img:1"}`, testAdminKey)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("unknown slug: want 404 got %d %s", rec.Code, rec.Body)
	}
}
