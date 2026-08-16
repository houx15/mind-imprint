package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"mindimprint/api/internal/config"
	"mindimprint/api/internal/oss"
)

// TestCourseAssetUploadKey covers the pure key-construction helper directly —
// it must agree with courseAssetKey (course_asset_urls.go) on the same
// slug+relativePath so the write side (upload) and read side (asset-urls)
// land on the same object.
func TestCourseAssetUploadKey(t *testing.T) {
	if got := courseAssetUploadKey("compare-claims", "assets/videos/case.mp4"); got != "courses/compare-claims/assets/videos/case.mp4" {
		t.Fatalf("courseAssetUploadKey = %q", got)
	}
}

// testUploadOSS builds a real *oss.Service with fake AK/SK. SignUpload is a
// local signature computation (github.com/aliyun/aliyun-oss-go-sdk), not a
// network call, so this runs offline with no live OSS creds required — unlike
// oss.NewSigner, whose origin bucket is nil and would panic on SignUpload.
func testUploadOSS(t *testing.T) *oss.Service {
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

const testUploadAdminKey = "admin-secret-123"

// newAssetUploadRequest builds a POST request against the handler directly
// (bypassing the mux, matching course_asset_urls_test.go's convention),
// carrying slug as a path value and an optional admin bearer key.
func newAssetUploadRequest(t *testing.T, slug, body, key string) *http.Request {
	t.Helper()
	r := httptest.NewRequest(http.MethodPost, "/api/v1/admin/courses/"+slug+"/asset-upload-url", strings.NewReader(body))
	r.SetPathValue("slug", slug)
	if key != "" {
		r.Header.Set("Authorization", "Bearer "+key)
	}
	return r
}

func TestPostCourseAssetUploadURL_Signs(t *testing.T) {
	a := &API{d: Deps{OSS: testUploadOSS(t), OSSAdminKey: testUploadAdminKey}}
	w := httptest.NewRecorder()
	body := `{"relativePath":"assets/videos/case.mp4","contentType":"video/mp4","size":1000}`
	a.postCourseAssetUploadURL(w, newAssetUploadRequest(t, "compare-claims", body, testUploadAdminKey))
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body %s", w.Code, w.Body.String())
	}
	var resp struct {
		PutURL, ObjectKey, RequiredContentType, ExpiresAt string
		MaxBytes                                          int64
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.ObjectKey != "courses/compare-claims/assets/videos/case.mp4" {
		t.Fatalf("objectKey = %q", resp.ObjectKey)
	}
	if resp.PutURL == "" {
		t.Fatalf("putUrl empty")
	}
	if resp.RequiredContentType != "video/mp4" {
		t.Fatalf("requiredContentType = %q", resp.RequiredContentType)
	}
	if resp.MaxBytes != 500<<20 {
		t.Fatalf("maxBytes = %d, want 500 MB", resp.MaxBytes)
	}
	if resp.ExpiresAt == "" {
		t.Fatalf("expiresAt empty")
	}
}

func TestPostCourseAssetUploadURL_NeedsAdminKey(t *testing.T) {
	a := &API{d: Deps{OSS: testUploadOSS(t), OSSAdminKey: testUploadAdminKey}}
	body := `{"relativePath":"assets/a.png","contentType":"image/png","size":1000}`

	w := httptest.NewRecorder()
	a.postCourseAssetUploadURL(w, newAssetUploadRequest(t, "demo", body, ""))
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("no key: want 401 got %d %s", w.Code, w.Body.String())
	}

	w2 := httptest.NewRecorder()
	a.postCourseAssetUploadURL(w2, newAssetUploadRequest(t, "demo", body, "wrong"))
	if w2.Code != http.StatusUnauthorized {
		t.Fatalf("wrong key: want 401 got %d %s", w2.Code, w2.Body.String())
	}
}

func TestPostCourseAssetUploadURL_BadRelativePath(t *testing.T) {
	a := &API{d: Deps{OSS: testUploadOSS(t), OSSAdminKey: testUploadAdminKey}}
	for _, rel := range []string{"../escape", "/leading", "http://x/y.png", "a/../b"} {
		body := `{"relativePath":"` + rel + `","contentType":"image/png","size":1000}`
		w := httptest.NewRecorder()
		a.postCourseAssetUploadURL(w, newAssetUploadRequest(t, "demo", body, testUploadAdminKey))
		if w.Code != http.StatusBadRequest || !strings.Contains(w.Body.String(), "invalid_asset_path") {
			t.Fatalf("relativePath %q: want 400 invalid_asset_path got %d %s", rel, w.Code, w.Body.String())
		}
	}
}

func TestPostCourseAssetUploadURL_BadSlug(t *testing.T) {
	a := &API{d: Deps{OSS: testUploadOSS(t), OSSAdminKey: testUploadAdminKey}}
	body := `{"relativePath":"assets/a.png","contentType":"image/png","size":1000}`
	for _, slug := range []string{"a/b", "a..b"} {
		w := httptest.NewRecorder()
		a.postCourseAssetUploadURL(w, newAssetUploadRequest(t, slug, body, testUploadAdminKey))
		if w.Code != http.StatusBadRequest || !strings.Contains(w.Body.String(), "invalid_slug") {
			t.Fatalf("slug %q: want 400 invalid_slug got %d %s", slug, w.Code, w.Body.String())
		}
	}
}

func TestPostCourseAssetUploadURL_UnsupportedType(t *testing.T) {
	a := &API{d: Deps{OSS: testUploadOSS(t), OSSAdminKey: testUploadAdminKey}}
	body := `{"relativePath":"assets/a.txt","contentType":"text/plain","size":1000}`
	w := httptest.NewRecorder()
	a.postCourseAssetUploadURL(w, newAssetUploadRequest(t, "demo", body, testUploadAdminKey))
	if w.Code != http.StatusBadRequest || !strings.Contains(w.Body.String(), "unsupported_type") {
		t.Fatalf("want 400 unsupported_type got %d %s", w.Code, w.Body.String())
	}
}

func TestPostCourseAssetUploadURL_SizeLimits(t *testing.T) {
	a := &API{d: Deps{OSS: testUploadOSS(t), OSSAdminKey: testUploadAdminKey}}

	tooBig := `{"relativePath":"assets/big.mp4","contentType":"video/mp4","size":629145600}` // 600 MB
	w := httptest.NewRecorder()
	a.postCourseAssetUploadURL(w, newAssetUploadRequest(t, "demo", tooBig, testUploadAdminKey))
	if w.Code != http.StatusBadRequest || !strings.Contains(w.Body.String(), "file_too_large") {
		t.Fatalf("600 MB: want 400 file_too_large got %d %s", w.Code, w.Body.String())
	}

	zero := `{"relativePath":"assets/a.png","contentType":"image/png","size":0}`
	w2 := httptest.NewRecorder()
	a.postCourseAssetUploadURL(w2, newAssetUploadRequest(t, "demo", zero, testUploadAdminKey))
	if w2.Code != http.StatusBadRequest || !strings.Contains(w2.Body.String(), "file_too_large") {
		t.Fatalf("zero size: want 400 file_too_large got %d %s", w2.Code, w2.Body.String())
	}
}

func TestPostCourseAssetUploadURL_OSSDisabled(t *testing.T) {
	a := &API{d: Deps{OSSAdminKey: testUploadAdminKey}}
	body := `{"relativePath":"assets/a.png","contentType":"image/png","size":1000}`
	w := httptest.NewRecorder()
	a.postCourseAssetUploadURL(w, newAssetUploadRequest(t, "demo", body, testUploadAdminKey))
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", w.Code)
	}
}
