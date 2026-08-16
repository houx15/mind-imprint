package api

// course_asset_urls.go — OSS CDN URL鉴权 slice. POST /api/v1/courses/{slug}/asset-urls
// signs a batch of a course's relative asset paths into cacheable CDN read URLs.
// The client (which holds the full CourseDefinition) collects the paths; the
// server maps each to the key courses/<slug>/<path> and signs it — so a caller
// can only obtain URLs within its own course's asset namespace, and the server
// never parses course block internals. The same endpoint is the refresh endpoint
// (re-POST the same paths when a session outlasts the URL鉴权 window).

import (
	"net/http"
	"strings"
	"time"

	"mindimprint/api/internal/httpx"
)

const (
	assetURLsBodyLimit = 16 << 10 // 16 KB: a few hundred short relative paths
	assetURLsMaxPaths  = 256
)

type courseAssetURLsReq struct {
	Paths []string `json:"paths"`
}

type courseAssetURLsResp struct {
	AssetURLs map[string]string `json:"assetUrls"`
	ExpiresAt string            `json:"expiresAt"`
}

// validRelativeAssetPath mirrors the shape of packages/course-contract's
// relativeAssetPathSchema: non-empty, no leading slash, no ".." segment, no
// scheme. It is a border check, not a schema port.
func validRelativeAssetPath(p string) bool {
	if p == "" || strings.HasPrefix(p, "/") || strings.Contains(p, "..") {
		return false
	}
	lower := strings.ToLower(p)
	return !strings.HasPrefix(lower, "http://") &&
		!strings.HasPrefix(lower, "https://") &&
		!strings.HasPrefix(lower, "data:")
}

// courseAssetKey maps a course slug + a validated relative asset path to its OSS
// object key.
func courseAssetKey(slug, path string) string {
	return "courses/" + slug + "/" + path
}

func (a *API) postCourseAssetURLs(w http.ResponseWriter, r *http.Request) {
	if a.d.OSS == nil {
		httpx.WriteError(w, r, httpx.ErrOSSUnavailable())
		return
	}
	slug := r.PathValue("slug")
	if slug == "" || strings.Contains(slug, "..") || strings.Contains(slug, "/") {
		httpx.WriteError(w, r, httpx.ErrBadRequest("invalid_slug", "无效的课程标识。", nil))
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, assetURLsBodyLimit)
	var req courseAssetURLsReq
	if err := decodeJSON(r, &req); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if len(req.Paths) > assetURLsMaxPaths {
		httpx.WriteError(w, r, httpx.ErrBadRequest("too_many_paths", "资源路径过多。", nil))
		return
	}
	urls := make(map[string]string, len(req.Paths))
	for _, p := range req.Paths {
		if !validRelativeAssetPath(p) {
			httpx.WriteError(w, r, httpx.ErrBadRequest("invalid_asset_path", "无效的资源路径。", nil))
			return
		}
		if _, done := urls[p]; done {
			continue
		}
		url, err := a.d.OSS.SignDownload(courseAssetKey(slug, p))
		if err != nil {
			httpx.WriteError(w, r, err)
			return
		}
		urls[p] = url
	}
	httpx.WriteJSON(w, http.StatusOK, courseAssetURLsResp{
		AssetURLs: urls,
		ExpiresAt: time.Now().Add(a.d.OSS.DownloadWindow()).UTC().Format(time.RFC3339),
	})
}
