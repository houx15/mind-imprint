package api

// course_covers.go — resolving and validating a COURSE cover. This extends the
// project cover scheme (project_covers.go: "img:<n>" stock photo / "grad:<name>"
// gradient) with a third form unique to generated courses:
//
//	asset:<relativePath>   e.g.  asset:cover/course-cover.webp
//
// the teacher toolkit's generated 16:9 WebP, uploaded to the course's own OSS
// namespace at courses/<slug>/cover/course-cover.webp. The STORED value never
// carries a raw OSS object key or URL — only the course-relative path. The OSS
// key is re-derived from the course slug at read time (courseAssetKey), so a
// stored cover can only ever address an object inside this course's own
// namespace, never an arbitrary bucket object (authoring-API contract rule 5).

import (
	"context"
	"net/http"
	"strings"

	"mindimprint/api/internal/httpx"
)

const courseCoverAssetPrefix = "asset:"

// validCourseCoverAssetPath reports whether p is an acceptable generated-cover
// relative path: a safe course-relative path (validRelativeAssetPath) that
// lives under cover/ and is a .webp. This is the first-version restriction from
// the authoring-API contract (rule 2) — the toolkit encodes exactly one file at
// cover/course-cover.webp.
func validCourseCoverAssetPath(p string) bool {
	if !validRelativeAssetPath(p) {
		return false
	}
	lower := strings.ToLower(p)
	return strings.HasPrefix(lower, "cover/") && strings.HasSuffix(lower, ".webp")
}

// looksLikeWebP reports whether b begins with the RIFF....WEBP container magic
// (bytes 0-3 "RIFF", 8-11 "WEBP"). A cheap content check so a mislabeled or
// non-image object can't be published as a cover — the .webp extension alone is
// not trusted (rule 4: verify the object IS WebP).
func looksLikeWebP(b []byte) bool {
	return len(b) >= 12 && string(b[0:4]) == "RIFF" && string(b[8:12]) == "WEBP"
}

// courseCoverObjectKey returns the OSS object key a course cover resolves to and
// whether it is an asset-scheme cover at all. Pure + slug-scoped: only an
// "asset:<relativePath>" cover whose path is still valid yields a key, and that
// key is always inside courses/<slug>/. "img:"/"grad:"/"" return ok=false so the
// caller falls back to the shared project-cover resolver.
func courseCoverObjectKey(slug, cover string) (string, bool) {
	if !strings.HasPrefix(cover, courseCoverAssetPrefix) {
		return "", false
	}
	rel := strings.TrimPrefix(cover, courseCoverAssetPrefix)
	if !validCourseCoverAssetPath(rel) {
		return "", false
	}
	return courseAssetKey(slug, rel), true
}

// resolveCourseCoverURL signs a course's cover into a short-lived GET URL.
// "asset:" covers are re-derived to a slug-scoped OSS key and signed; every
// other form ("img:"/"grad:"/"") falls through to the shared project-cover
// resolver, so existing behavior is preserved exactly (rule 6).
func (a *API) resolveCourseCoverURL(slug, cover string) string {
	if key, ok := courseCoverObjectKey(slug, cover); ok {
		if a.d.OSS == nil {
			return ""
		}
		url, err := a.d.OSS.SignDownload(key)
		if err != nil {
			return ""
		}
		return url
	}
	return a.resolveCoverURL(cover)
}

// coverAssetGetter is the narrow OSS slice the ship-time cover check needs:
// object existence + a byte read for the WebP magic check. *oss.Service
// satisfies it; tests use a stub (a.d.OSS is a concrete type, not fakeable —
// see course_ship_test.go's header).
type coverAssetGetter interface {
	Exists(ctx context.Context, key string) (bool, error)
	GetObject(ctx context.Context, key string) ([]byte, error)
}

// validateCoverAsset verifies a generated-cover upload before it is bound at
// publish, and returns the value to persist ("asset:<relativePath>"). It
// enforces the authoring-API contract: a safe cover/<...>.webp path, an object
// that actually exists inside THIS course's namespace (the key is server-
// derived from the slug, never client-supplied), and WebP content. Any failure
// returns a *httpx.APIError and an empty store value — the caller must not
// change publication status.
func validateCoverAsset(ctx context.Context, getter coverAssetGetter, slug, coverAssetPath string) (string, *httpx.APIError) {
	if !validCourseCoverAssetPath(coverAssetPath) {
		return "", httpx.ErrBadRequest("invalid_cover_path", "封面路径必须是 cover/ 下的 .webp 文件。", nil)
	}
	key := courseAssetKey(slug, coverAssetPath)
	exists, err := getter.Exists(ctx, key)
	if err != nil {
		return "", httpx.ErrInternal()
	}
	if !exists {
		return "", &httpx.APIError{Status: http.StatusUnprocessableEntity, Code: "cover_not_found", Message: "封面文件不存在，请先上传后再发布。"}
	}
	data, err := getter.GetObject(ctx, key)
	if err != nil {
		return "", httpx.ErrInternal()
	}
	if !looksLikeWebP(data) {
		return "", &httpx.APIError{Status: http.StatusUnprocessableEntity, Code: "cover_not_webp", Message: "封面文件必须是 WebP 格式。"}
	}
	return courseCoverAssetPrefix + coverAssetPath, nil
}
