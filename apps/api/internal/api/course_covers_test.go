package api

import (
	"context"
	"errors"
	"net/http"
	"testing"
)

func TestValidCourseCoverAssetPath(t *testing.T) {
	for _, p := range []string{"cover/course-cover.webp", "cover/hero.webp"} {
		if !validCourseCoverAssetPath(p) {
			t.Errorf("want valid: %q", p)
		}
	}
	for _, p := range []string{
		"", "cover/x.png", "notcover/x.webp", "x.webp",
		"/cover/x.webp", "../cover/x.webp", "cover/../secret.webp",
		"https://evil/x.webp", "cover/course-cover.webp.txt",
	} {
		if validCourseCoverAssetPath(p) {
			t.Errorf("want invalid: %q", p)
		}
	}
}

func TestLooksLikeWebP(t *testing.T) {
	webp := []byte("RIFF\x24\x00\x00\x00WEBPVP8 ")
	if !looksLikeWebP(webp) {
		t.Fatal("RIFF....WEBP should be detected as webp")
	}
	if looksLikeWebP([]byte("\x89PNG\r\n\x1a\n\x00\x00\x00\x0d")) {
		t.Fatal("PNG magic must not be webp")
	}
	if looksLikeWebP([]byte("RIFF")) {
		t.Fatal("too-short buffer must not be webp")
	}
}

func TestCourseCoverObjectKey(t *testing.T) {
	key, ok := courseCoverObjectKey("slug1", "asset:cover/course-cover.webp")
	if !ok || key != "courses/slug1/cover/course-cover.webp" {
		t.Fatalf("asset cover: key=%q ok=%v, want courses/slug1/cover/course-cover.webp true", key, ok)
	}
	for _, cover := range []string{"img:3", "grad:peach", "", "asset:../x.webp", "asset:cover/x.png", "asset:notcover/x.webp"} {
		if _, ok := courseCoverObjectKey("slug1", cover); ok {
			t.Errorf("cover %q must not yield an asset key", cover)
		}
	}
}

type stubCoverGetter struct {
	exists    bool
	existsErr error
	data      []byte
	dataErr   error
}

func (s stubCoverGetter) Exists(context.Context, string) (bool, error) {
	return s.exists, s.existsErr
}
func (s stubCoverGetter) GetObject(context.Context, string) ([]byte, error) {
	return s.data, s.dataErr
}

func TestValidateCoverAsset(t *testing.T) {
	ctx := context.Background()
	webp := []byte("RIFF\x24\x00\x00\x00WEBPVP8 ")

	// happy path — exists + webp → persisted as asset:<path>
	stored, apiErr := validateCoverAsset(ctx, stubCoverGetter{exists: true, data: webp}, "s", "cover/course-cover.webp")
	if apiErr != nil {
		t.Fatalf("valid cover: unexpected apiErr %+v", apiErr)
	}
	if stored != "asset:cover/course-cover.webp" {
		t.Fatalf("stored = %q, want asset:cover/course-cover.webp", stored)
	}

	// bad path → 400 invalid_cover_path, before any getter call
	if _, e := validateCoverAsset(ctx, stubCoverGetter{}, "s", "cover/x.png"); e == nil || e.Status != http.StatusBadRequest || e.Code != "invalid_cover_path" {
		t.Fatalf("bad path: got %+v, want 400 invalid_cover_path", e)
	}

	// missing object → 422 cover_not_found
	if _, e := validateCoverAsset(ctx, stubCoverGetter{exists: false}, "s", "cover/course-cover.webp"); e == nil || e.Status != http.StatusUnprocessableEntity || e.Code != "cover_not_found" {
		t.Fatalf("missing: got %+v, want 422 cover_not_found", e)
	}

	// exists but not webp → 422 cover_not_webp
	png := []byte("\x89PNG\r\n\x1a\n\x00\x00\x00\x0d")
	if _, e := validateCoverAsset(ctx, stubCoverGetter{exists: true, data: png}, "s", "cover/course-cover.webp"); e == nil || e.Status != http.StatusUnprocessableEntity || e.Code != "cover_not_webp" {
		t.Fatalf("non-webp: got %+v, want 422 cover_not_webp", e)
	}

	// existence-check error → 500, no publish
	if _, e := validateCoverAsset(ctx, stubCoverGetter{existsErr: errors.New("oss down")}, "s", "cover/course-cover.webp"); e == nil || e.Status != http.StatusInternalServerError {
		t.Fatalf("exists error: got %+v, want 500", e)
	}
}

func TestResolveCourseCoverURLNilOSS(t *testing.T) {
	a := &API{} // Deps zero value → OSS nil
	for _, cover := range []string{"asset:cover/course-cover.webp", "img:1", "grad:peach", ""} {
		if got := a.resolveCourseCoverURL("s", cover); got != "" {
			t.Errorf("cover %q with nil OSS: got %q, want \"\"", cover, got)
		}
	}
}
