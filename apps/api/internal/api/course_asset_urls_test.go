package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"mindimprint/api/internal/oss"
)

func TestValidRelativeAssetPath(t *testing.T) {
	ok := []string{"assets/videos/case.mp4", "interactions/html/sim.html", "a/b/c.png"}
	bad := []string{"", "/leading", "../escape", "a/../b", "http://x/y.png", "https://x", "data:text/plain,hi"}
	for _, p := range ok {
		if !validRelativeAssetPath(p) {
			t.Errorf("validRelativeAssetPath(%q) = false, want true", p)
		}
	}
	for _, p := range bad {
		if validRelativeAssetPath(p) {
			t.Errorf("validRelativeAssetPath(%q) = true, want false", p)
		}
	}
}

func TestCourseAssetKey(t *testing.T) {
	if got := courseAssetKey("compare-claims", "assets/videos/case.mp4"); got != "courses/compare-claims/assets/videos/case.mp4" {
		t.Fatalf("courseAssetKey = %q", got)
	}
}

func newAssetURLsRequest(t *testing.T, slug, body string) *http.Request {
	t.Helper()
	r := httptest.NewRequest(http.MethodPost, "/api/v1/courses/"+slug+"/asset-urls", strings.NewReader(body))
	r.SetPathValue("slug", slug)
	return r
}

func TestPostCourseAssetURLs_Signs(t *testing.T) {
	a := &API{d: Deps{OSS: oss.NewSigner("mind-oss.uni-robot.cn", "k", 2*time.Hour)}}
	w := httptest.NewRecorder()
	a.postCourseAssetURLs(w, newAssetURLsRequest(t, "demo", `{"paths":["assets/a.png","assets/a.png","assets/v.mp4"]}`))
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body %s", w.Code, w.Body.String())
	}
	var resp struct {
		AssetURLs map[string]string `json:"assetUrls"`
		ExpiresAt string            `json:"expiresAt"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(resp.AssetURLs) != 2 {
		t.Fatalf("assetUrls len = %d, want 2 (deduped)", len(resp.AssetURLs))
	}
	got := resp.AssetURLs["assets/a.png"]
	if !strings.Contains(got, "/courses/demo/assets/a.png?auth_key=") {
		t.Fatalf("signed url = %q", got)
	}
	if resp.ExpiresAt == "" {
		t.Fatalf("expiresAt empty")
	}
}

func TestPostCourseAssetURLs_BadPath(t *testing.T) {
	a := &API{d: Deps{OSS: oss.NewSigner("d", "k", time.Hour)}}
	w := httptest.NewRecorder()
	a.postCourseAssetURLs(w, newAssetURLsRequest(t, "demo", `{"paths":["../secret"]}`))
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", w.Code)
	}
}

func TestPostCourseAssetURLs_OSSDisabled(t *testing.T) {
	a := &API{d: Deps{}}
	w := httptest.NewRecorder()
	a.postCourseAssetURLs(w, newAssetURLsRequest(t, "demo", `{"paths":["assets/a.png"]}`))
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", w.Code)
	}
}
