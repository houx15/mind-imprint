package api

// course_asset_upload.go — the course generator's media upload endpoint.
// POST /api/v1/admin/courses/{slug}/asset-upload-url (OSS_ADMIN_KEY bearer)
// signs a presigned PUT to the DETERMINISTIC key courses/<slug>/<relativePath>
// — the exact path the CourseDefinition references and /asset-urls signs for
// playback. Used for video/pdf/images/interactiveHtml (narration audio is
// generated at ship, not uploaded).

import (
	"net/http"
	"strings"
	"time"

	"mindimprint/api/internal/httpx"
)

type courseAssetUploadReq struct {
	RelativePath string `json:"relativePath"`
	ContentType  string `json:"contentType"`
	Size         int64  `json:"size"`
}

func (a *API) postCourseAssetUploadURL(w http.ResponseWriter, r *http.Request) {
	if a.d.OSS == nil {
		httpx.WriteError(w, r, httpx.ErrOSSUnavailable())
		return
	}
	if !a.ossAdminAuthed(r) {
		httpx.WriteError(w, r, httpx.ErrUnauthorized("需要有效的管理密钥"))
		return
	}
	slug := r.PathValue("slug")
	if slug == "" || strings.Contains(slug, "..") || strings.Contains(slug, "/") {
		httpx.WriteError(w, r, httpx.ErrBadRequest("invalid_slug", "无效的课程标识。", nil))
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, ossPresignBodyLimit)
	var req courseAssetUploadReq
	if err := decodeJSON(r, &req); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if !validRelativeAssetPath(req.RelativePath) {
		httpx.WriteError(w, r, httpx.ErrBadRequest("invalid_asset_path", "无效的资源路径。", nil))
		return
	}
	sc := ossScopes["course_material"]
	if !sc.allowedTypes[req.ContentType] {
		httpx.WriteError(w, r, httpx.ErrBadRequest("unsupported_type", "不支持的文件类型。", nil))
		return
	}
	if req.Size <= 0 || req.Size > sc.maxBytes {
		httpx.WriteError(w, r, httpx.ErrBadRequest("file_too_large", "文件超出大小限制。", nil))
		return
	}
	objectKey := courseAssetUploadKey(slug, req.RelativePath)
	putURL, err := a.d.OSS.SignUpload(objectKey, req.ContentType, ossUploadTTL)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, ossUploadResp{
		PutURL:              putURL,
		ObjectKey:           objectKey,
		RequiredContentType: req.ContentType,
		MaxBytes:            sc.maxBytes,
		ExpiresAt:           time.Now().Add(ossUploadTTL).UTC().Format(time.RFC3339),
	})
}

// courseAssetUploadKey mirrors courseAssetKey (course_asset_urls.go) — the
// same slug+relativePath maps to the same object key on both the write
// (upload) and read (asset-urls) sides.
func courseAssetUploadKey(slug, relativePath string) string {
	return "courses/" + slug + "/" + relativePath
}
