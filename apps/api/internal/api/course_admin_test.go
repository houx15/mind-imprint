package api_test

// course_admin_test.go — Task 7's httptest suite for postAdminUploadCourse:
// the OSS_ADMIN_KEY-gated developer publish path (border-validate structure +
// render_cache + card_ids, then agent.UpsertCourse keyed by slug). Mirrors
// oss_test.go's admin-key bearer pattern and course_test.go's
// New(Deps{...}).Handler() + newAPITestPool(t) fixture wiring.

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

// validCourseJSON/validRenderCacheJSON are the smallest course+render_cache
// pair that satisfies the handler's border validation: course.id/title
// non-empty with ≥1 step, render_cache.courseId matching, ≥1 rendered step.
const validCourseJSON = `{"id":"admin-upload-test","title":"Admin Upload Test Course","steps":[{"id":"step_01","title":"Step One"}]}`
const validRenderCacheJSON = `{"version":1,"courseId":"admin-upload-test","steps":[{"stepId":"step_01","content":{"title":"Step One"}}]}`

// renderCacheWithTeachingJSON is validRenderCacheJSON's course-audio variant:
// its one step has a non-empty "teaching" segment, so
// agent.GenerateCourseAudio (course voice narration, Task 3) would produce a
// manifest entry for it IF both a.d.Voice and a.d.OSS were configured — used
// by TestAdminUploadCourseAudioManifestDegradesWithoutOSS to prove the OSS
// half of the nil-guard on its own (Voice present, OSS absent).
const renderCacheWithTeachingJSON = `{"version":1,"courseId":"admin-upload-audio-test","steps":[{"stepId":"step_01","content":{"title":"Step One","segments":[{"kind":"teaching","text":"这是一段讲解文本。"}]}}]}`

func adminUploadBody(course, renderCache string, cardIDs []string) string {
	cardIDsJSON, _ := json.Marshal(cardIDs)
	return `{"course":` + course + `,"renderCache":` + renderCache + `,"cardIds":` + string(cardIDsJSON) + `,"branch":"A","blurb":"test blurb","time_label":"5 分钟"}`
}

// TestAdminUploadCourseValidUploadIsListable asserts a valid admin upload
// returns 200 {slug, step_count} and the course is then gettable by slug —
// the whole point of the endpoint is to make GET /courses/{slug} work.
func TestAdminUploadCourseValidUploadIsListable(t *testing.T) {
	pool := newAPITestPool(t)
	h := New(Deps{Queries: sqlc.New(pool), Pool: pool, OSSAdminKey: testAdminKey}).Handler()

	body := adminUploadBody(validCourseJSON, validRenderCacheJSON, []string{"craap"})
	req := httptest.NewRequest("POST", "/api/v1/admin/courses", strings.NewReader(body))
	bearer(req, testAdminKey)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("valid upload: want 200 got %d %s", rec.Code, rec.Body)
	}
	var resp struct {
		Slug      string `json:"slug"`
		StepCount int    `json:"step_count"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Slug != "admin-upload-test" || resp.StepCount != 1 {
		t.Fatalf("resp = %+v, want slug=admin-upload-test step_count=1", resp)
	}

	// Now listable/gettable via the ordinary student-facing endpoints.
	cookie := signInSeed(t, pool)
	getRec := httptest.NewRecorder()
	h.ServeHTTP(getRec, withCookie(httptest.NewRequest("GET", "/api/v1/courses/admin-upload-test", nil), cookie))
	if getRec.Code != http.StatusOK {
		t.Fatalf("get uploaded course: want 200 got %d %s", getRec.Code, getRec.Body)
	}
	var getResp struct {
		Course struct {
			Slug    string   `json:"slug"`
			Title   string   `json:"title"`
			CardIDs []string `json:"cardIds"`
		} `json:"course"`
	}
	if err := json.Unmarshal(getRec.Body.Bytes(), &getResp); err != nil {
		t.Fatalf("get course decode: %v", err)
	}
	if getResp.Course.Slug != "admin-upload-test" || getResp.Course.Title != "Admin Upload Test Course" {
		t.Fatalf("uploaded course payload = %+v", getResp.Course)
	}
	if len(getResp.Course.CardIDs) != 1 || getResp.Course.CardIDs[0] != "craap" {
		t.Fatalf("uploaded course card_ids = %v, want [craap]", getResp.Course.CardIDs)
	}

	// Course voice narration (Task 4): this Deps has neither Voice nor OSS
	// configured, so agent.GenerateCourseAudio must have degraded to an
	// empty manifest — and, per the plan's Global Constraints, that must
	// never have failed the upload above (it didn't: rec.Code was 200).
	payload, _, err := agent.NewSqlcAgentStore(sqlc.New(pool), pool).GetCoursePayload(context.Background(), "admin-upload-test")
	if err != nil {
		t.Fatalf("GetCoursePayload: %v", err)
	}
	if len(payload.AudioManifest) != 0 {
		t.Fatalf("AudioManifest = %v, want empty (no Voice/OSS configured)", payload.AudioManifest)
	}
}

// TestAdminUploadCourseAudioManifestDegradesWithoutOSS asserts the other half
// of Task 4's nil-guard: a.d.Voice configured but a.d.OSS absent (the common
// case for an environment that has TTS but not object storage, or vice
// versa) must still degrade to an empty manifest and a successful upload —
// agent.GenerateCourseAudio requires BOTH dependencies non-nil before it
// synthesizes anything, so the stub Voice's Synthesize must never even be
// called. Deps.OSS is a concrete *oss.Service (not fakeable without a real
// bucket — see internal/oss/oss_test.go's own network-avoidance note), so
// the non-empty-manifest path is covered instead at the unit level in
// course_audio_test.go (Task 3) and the wiring level in
// seed_courses_test.go's TestSeedCoursesGeneratesAudioManifest (Task 4),
// both of which stub agent.CourseAudioSynth/CourseAudioStore directly.
func TestAdminUploadCourseAudioManifestDegradesWithoutOSS(t *testing.T) {
	pool := newAPITestPool(t)
	stub := &stubVoice{audio: []byte("MP3")}
	h := New(Deps{Queries: sqlc.New(pool), Pool: pool, OSSAdminKey: testAdminKey, Voice: stub}).Handler()

	body := adminUploadBody(validCourseJSON, renderCacheWithTeachingJSON, []string{"craap"})
	// Reuse validCourseJSON's id (course.ID) but the audio-flavored render
	// cache, so the courseId fields must agree — swap validCourseJSON's id
	// to match renderCacheWithTeachingJSON's courseId.
	body = strings.Replace(body, `"id":"admin-upload-test"`, `"id":"admin-upload-audio-test"`, 1)

	req := httptest.NewRequest("POST", "/api/v1/admin/courses", strings.NewReader(body))
	bearer(req, testAdminKey)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("upload with Voice-but-no-OSS: want 200 got %d %s", rec.Code, rec.Body)
	}

	if stub.calls != 0 {
		t.Fatalf("stub Voice.Synthesize was called %d times, want 0 (OSS absent must short-circuit before synthesis)", stub.calls)
	}

	payload, _, err := agent.NewSqlcAgentStore(sqlc.New(pool), pool).GetCoursePayload(context.Background(), "admin-upload-audio-test")
	if err != nil {
		t.Fatalf("GetCoursePayload: %v", err)
	}
	if len(payload.AudioManifest) != 0 {
		t.Fatalf("AudioManifest = %v, want empty (OSS not configured)", payload.AudioManifest)
	}
}

// TestAdminUploadCourseUnauthorized asserts a missing or wrong bearer 401s
// BEFORE any parsing/DB work, and never echoes the configured key.
func TestAdminUploadCourseUnauthorized(t *testing.T) {
	pool := newAPITestPool(t)
	h := New(Deps{Queries: sqlc.New(pool), Pool: pool, OSSAdminKey: testAdminKey}).Handler()

	body := adminUploadBody(validCourseJSON, validRenderCacheJSON, []string{"craap"})

	t.Run("missing bearer", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/api/v1/admin/courses", strings.NewReader(body))
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("missing bearer: want 401 got %d %s", rec.Code, rec.Body)
		}
		if strings.Contains(rec.Body.String(), testAdminKey) {
			t.Fatalf("response echoed the admin key: %s", rec.Body)
		}
	})

	t.Run("wrong bearer", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/api/v1/admin/courses", strings.NewReader(body))
		bearer(req, "wrong-key")
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("wrong bearer: want 401 got %d %s", rec.Code, rec.Body)
		}
	})
}

// TestAdminUploadCourseValidation covers the three border-validation
// rejections: an unknown card id, a course with no steps, and a render_cache
// whose courseId doesn't match the course's own id. Each must 400
// validation_failed and must NOT write a course row.
func TestAdminUploadCourseValidation(t *testing.T) {
	pool := newAPITestPool(t)
	h := New(Deps{Queries: sqlc.New(pool), Pool: pool, OSSAdminKey: testAdminKey}).Handler()

	post := func(body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest("POST", "/api/v1/admin/courses", strings.NewReader(body))
		bearer(req, testAdminKey)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		return rec
	}

	t.Run("unknown card id", func(t *testing.T) {
		rec := post(adminUploadBody(validCourseJSON, validRenderCacheJSON, []string{"not-a-card"}))
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("unknown card id: want 400 got %d %s", rec.Code, rec.Body)
		}
		if !strings.Contains(rec.Body.String(), "validation_failed") {
			t.Fatalf("unknown card id: want validation_failed code, got %s", rec.Body)
		}
	})

	t.Run("missing course.steps", func(t *testing.T) {
		noSteps := `{"id":"admin-upload-nosteps","title":"No Steps"}`
		rec := post(adminUploadBody(noSteps, validRenderCacheJSON, []string{"craap"}))
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("missing steps: want 400 got %d %s", rec.Code, rec.Body)
		}
		if !strings.Contains(rec.Body.String(), "validation_failed") {
			t.Fatalf("missing steps: want validation_failed code, got %s", rec.Body)
		}
	})

	t.Run("renderCache.courseId mismatch", func(t *testing.T) {
		mismatchedRC := `{"version":1,"courseId":"some-other-course","steps":[{"stepId":"step_01","content":{"title":"Step One"}}]}`
		rec := post(adminUploadBody(validCourseJSON, mismatchedRC, []string{"craap"}))
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("courseId mismatch: want 400 got %d %s", rec.Code, rec.Body)
		}
		if !strings.Contains(rec.Body.String(), "validation_failed") {
			t.Fatalf("courseId mismatch: want validation_failed code, got %s", rec.Body)
		}
	})

	// step-count mismatch: course.steps has 2 entries but renderCache.steps
	// has only 1 — completion (len(structure.steps)) and paging
	// (len(renderCache.steps)) would silently disagree if this were allowed
	// through.
	t.Run("step count mismatch", func(t *testing.T) {
		twoStepCourse := `{"id":"admin-upload-stepmismatch","title":"Step Mismatch","steps":[{"id":"step_01","title":"Step One"},{"id":"step_02","title":"Step Two"}]}`
		oneStepRC := `{"version":1,"courseId":"admin-upload-stepmismatch","steps":[{"stepId":"step_01","content":{"title":"Step One"}}]}`
		rec := post(adminUploadBody(twoStepCourse, oneStepRC, []string{"craap"}))
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("step count mismatch: want 400 got %d %s", rec.Code, rec.Body)
		}
		if !strings.Contains(rec.Body.String(), "validation_failed") {
			t.Fatalf("step count mismatch: want validation_failed code, got %s", rec.Body)
		}
	})

	// None of the rejected uploads should have written a course row.
	var count int
	if err := pool.QueryRow(context.Background(), `SELECT count(*) FROM course WHERE slug LIKE 'admin-upload-%'`).Scan(&count); err != nil {
		t.Fatalf("count courses: %v", err)
	}
	if count != 0 {
		t.Fatalf("rejected uploads wrote %d course rows, want 0", count)
	}
}
