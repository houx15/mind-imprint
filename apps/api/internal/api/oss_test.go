package api_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	. "mindimprint/api/internal/api"
	"mindimprint/api/internal/config"
	"mindimprint/api/internal/oss"
	"mindimprint/api/internal/store/sqlc"
)

const testAdminKey = "admin-secret-123"

func testOSS(t *testing.T) *oss.Service {
	t.Helper()
	svc, err := oss.New(config.Config{
		OSSEndpoint:     "mind-imprint.oss-cn-beijing.aliyuncs.com",
		OSSBucket:       "mind-imprint",
		OSSCDNDomain:    "mind-oss.uni-robot.cn",
		OSSAccessKeyID:  "AK-test",
		OSSAccessSecret: "SK-test",
	})
	if err != nil {
		t.Fatalf("build oss service: %v", err)
	}
	return svc
}

func bearer(req *http.Request, key string) *http.Request {
	req.Header.Set("Authorization", "Bearer "+key)
	return req
}

// TestOSSAdminAndDisabled covers the admin-key routes, resolve, and the disabled
// states — none of which need a database session, so no testcontainers.
func TestOSSAdminAndDisabled(t *testing.T) {
	enabled := New(Deps{OSS: testOSS(t), OSSAdminKey: testAdminKey}).Handler()

	post := func(h http.Handler, path, body string, key string) *httptest.ResponseRecorder {
		req := httptest.NewRequest("POST", path, strings.NewReader(body))
		if key != "" {
			bearer(req, key)
		}
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		return rec
	}

	t.Run("admin upload needs a valid bearer key", func(t *testing.T) {
		body := `{"scope":"course_material","contentType":"application/pdf","size":1000,"filename":"u.pdf"}`
		if rec := post(enabled, "/api/v1/oss/admin/upload-url", body, ""); rec.Code != http.StatusUnauthorized {
			t.Fatalf("no key: want 401 got %d %s", rec.Code, rec.Body)
		}
		if rec := post(enabled, "/api/v1/oss/admin/upload-url", body, "wrong"); rec.Code != http.StatusUnauthorized {
			t.Fatalf("wrong key: want 401 got %d %s", rec.Code, rec.Body)
		}
		rec := post(enabled, "/api/v1/oss/admin/upload-url", body, testAdminKey)
		if rec.Code != http.StatusOK {
			t.Fatalf("valid key: want 200 got %d %s", rec.Code, rec.Body)
		}
		var resp struct {
			ObjectKey, PutURL, RequiredContentType string
			MaxBytes                               int64
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if !strings.HasPrefix(resp.ObjectKey, "courses/") {
			t.Fatalf("objectKey prefix: want courses/, got %q", resp.ObjectKey)
		}
		if !strings.HasSuffix(resp.ObjectKey, ".pdf") {
			t.Fatalf("objectKey ext: want .pdf, got %q", resp.ObjectKey)
		}
		if !strings.Contains(resp.PutURL, "mind-imprint.oss-cn-beijing.aliyuncs.com") {
			t.Fatalf("putUrl not origin host: %q", resp.PutURL)
		}
		if resp.RequiredContentType != "application/pdf" {
			t.Fatalf("requiredContentType: %q", resp.RequiredContentType)
		}
	})

	t.Run("admin upload rejects a session scope", func(t *testing.T) {
		body := `{"scope":"user_image","contentType":"image/png","size":100,"filename":"c.png"}`
		rec := post(enabled, "/api/v1/oss/admin/upload-url", body, testAdminKey)
		if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "unknown_scope") {
			t.Fatalf("user_image via admin: want 400 unknown_scope got %d %s", rec.Code, rec.Body)
		}
	})

	t.Run("admin upload validates type and size", func(t *testing.T) {
		bad := `{"scope":"course_material","contentType":"text/plain","size":100}`
		if rec := post(enabled, "/api/v1/oss/admin/upload-url", bad, testAdminKey); rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "unsupported_type") {
			t.Fatalf("bad type: want 400 unsupported_type got %d %s", rec.Code, rec.Body)
		}
		big := `{"scope":"course_material","contentType":"application/pdf","size":999999999}`
		if rec := post(enabled, "/api/v1/oss/admin/upload-url", big, testAdminKey); rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "file_too_large") {
			t.Fatalf("too big: want 400 file_too_large got %d %s", rec.Code, rec.Body)
		}
	})

	t.Run("resolve accepts admin key, rejects no auth, guards traversal", func(t *testing.T) {
		body := `{"objectKey":"courses/abc.pdf"}`
		if rec := post(enabled, "/api/v1/oss/resolve-url", body, ""); rec.Code != http.StatusUnauthorized {
			t.Fatalf("resolve no auth: want 401 got %d %s", rec.Code, rec.Body)
		}
		rec := post(enabled, "/api/v1/oss/resolve-url", body, testAdminKey)
		if rec.Code != http.StatusOK {
			t.Fatalf("resolve admin: want 200 got %d %s", rec.Code, rec.Body)
		}
		var resp struct{ URL string }
		_ = json.Unmarshal(rec.Body.Bytes(), &resp)
		if !strings.Contains(resp.URL, "mind-oss.uni-robot.cn") {
			t.Fatalf("resolve url not CDN host: %q", resp.URL)
		}
		trav := `{"objectKey":"courses/../secret"}`
		if rec := post(enabled, "/api/v1/oss/resolve-url", trav, testAdminKey); rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "invalid_key") {
			t.Fatalf("traversal: want 400 invalid_key got %d %s", rec.Code, rec.Body)
		}
	})

	t.Run("disabled when OSS unconfigured", func(t *testing.T) {
		disabled := New(Deps{}).Handler() // OSS nil, no admin key
		for _, p := range []string{"/api/v1/oss/admin/upload-url", "/api/v1/oss/resolve-url"} {
			if rec := post(disabled, p, `{}`, testAdminKey); rec.Code != http.StatusServiceUnavailable {
				t.Fatalf("%s disabled: want 503 got %d %s", p, rec.Code, rec.Body)
			}
		}
	})

	t.Run("admin route disabled when admin key empty", func(t *testing.T) {
		noKey := New(Deps{OSS: testOSS(t)}).Handler() // OSS set, admin key empty
		body := `{"scope":"course_material","contentType":"application/pdf","size":100}`
		if rec := post(noKey, "/api/v1/oss/admin/upload-url", body, "anything"); rec.Code != http.StatusServiceUnavailable {
			t.Fatalf("empty admin key: want 503 got %d %s", rec.Code, rec.Body)
		}
	})
}

// TestOSSVideoAndBodyLimit covers video resources in the course_material scope
// and the presign request-body cap — neither needs a database session.
func TestOSSVideoAndBodyLimit(t *testing.T) {
	enabled := New(Deps{OSS: testOSS(t), OSSAdminKey: testAdminKey}).Handler()

	post := func(path, body, key string) *httptest.ResponseRecorder {
		req := httptest.NewRequest("POST", path, strings.NewReader(body))
		if key != "" {
			bearer(req, key)
		}
		rec := httptest.NewRecorder()
		enabled.ServeHTTP(rec, req)
		return rec
	}

	t.Run("course_material accepts video with the right extension", func(t *testing.T) {
		cases := []struct{ ct, ext string }{
			{"video/mp4", ".mp4"},
			{"video/webm", ".webm"},
			{"video/quicktime", ".mov"},
		}
		for _, c := range cases {
			body := `{"scope":"course_material","contentType":"` + c.ct + `","size":209715200}` // 200 MB
			rec := post("/api/v1/oss/admin/upload-url", body, testAdminKey)
			if rec.Code != http.StatusOK {
				t.Fatalf("%s: want 200 got %d %s", c.ct, rec.Code, rec.Body)
			}
			var resp struct{ ObjectKey string }
			_ = json.Unmarshal(rec.Body.Bytes(), &resp)
			if !strings.HasPrefix(resp.ObjectKey, "courses/") || !strings.HasSuffix(resp.ObjectKey, c.ext) {
				t.Fatalf("%s: objectKey %q want courses/*%s", c.ct, resp.ObjectKey, c.ext)
			}
		}
	})

	t.Run("video is rejected outside course_material", func(t *testing.T) {
		body := `{"scope":"web_resource","contentType":"video/mp4","size":1000}`
		rec := post("/api/v1/oss/admin/upload-url", body, testAdminKey)
		if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "unsupported_type") {
			t.Fatalf("video in web_resource: want 400 unsupported_type got %d %s", rec.Code, rec.Body)
		}
	})

	t.Run("course video over the 500 MB cap is rejected", func(t *testing.T) {
		body := `{"scope":"course_material","contentType":"video/mp4","size":629145600}` // 600 MB
		rec := post("/api/v1/oss/admin/upload-url", body, testAdminKey)
		if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "file_too_large") {
			t.Fatalf("600 MB video: want 400 file_too_large got %d %s", rec.Code, rec.Body)
		}
	})

	t.Run("an oversized presign body is rejected", func(t *testing.T) {
		body := `{"scope":"course_material","contentType":"video/mp4","size":1000,"filename":"` + strings.Repeat("a", 8<<10) + `"}`
		rec := post("/api/v1/oss/admin/upload-url", body, testAdminKey)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("8 KB body: want 400 got %d %s", rec.Code, rec.Body)
		}
	})
}

// TestOSSUserUpload covers the session-gated user route and session resolve,
// which need a real signed-in user (testcontainers DB).
func TestOSSUserUpload(t *testing.T) {
	pool := newAPITestPool(t)
	h := New(Deps{Queries: sqlc.New(pool), Pool: pool, OSS: testOSS(t), OSSAdminKey: testAdminKey}).Handler()
	cookie := signInSeed(t, pool)

	// user upload → key scoped to the caller's uid.
	body := `{"contentType":"image/png","size":2048,"filename":"me.png"}`
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("POST", "/api/v1/oss/upload-url", strings.NewReader(body)), cookie))
	if rec.Code != http.StatusOK {
		t.Fatalf("user upload: want 200 got %d %s", rec.Code, rec.Body)
	}
	var resp struct{ ObjectKey string }
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	want := "users/" + SeedUserID.String() + "/images/"
	if !strings.HasPrefix(resp.ObjectKey, want) {
		t.Fatalf("user_image key: want prefix %q, got %q", want, resp.ObjectKey)
	}

	// user upload rejects a type outside the user_image allowlist.
	badBody := `{"contentType":"application/pdf","size":2048}`
	badRec := httptest.NewRecorder()
	h.ServeHTTP(badRec, withCookie(httptest.NewRequest("POST", "/api/v1/oss/upload-url", strings.NewReader(badBody)), cookie))
	if badRec.Code != http.StatusBadRequest || !strings.Contains(badRec.Body.String(), "unsupported_type") {
		t.Fatalf("user upload pdf: want 400 unsupported_type got %d %s", badRec.Code, badRec.Body)
	}

	// resolve with a session (no admin key) → 200.
	resolveRec := httptest.NewRecorder()
	h.ServeHTTP(resolveRec, withCookie(httptest.NewRequest("POST", "/api/v1/oss/resolve-url", strings.NewReader(`{"objectKey":"`+resp.ObjectKey+`"}`)), cookie))
	if resolveRec.Code != http.StatusOK {
		t.Fatalf("resolve with session: want 200 got %d %s", resolveRec.Code, resolveRec.Body)
	}
}
