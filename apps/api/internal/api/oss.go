package api

import (
	"crypto/subtle"
	"net/http"
	"path"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"

	"mindimprint/api/internal/httpx"
)

// ossUploadTTL bounds the signed upload URL (the client may pick a file then
// upload). Downloads no longer use a fixed TTL — see oss.Service.SignDownload
// / DownloadWindow.
const (
	ossUploadTTL = 10 * time.Minute
	// ossPresignBodyLimit caps the presign JSON request body. The body is a
	// handful of short fields (scope/contentType/size/filename); this guards
	// the endpoint against oversized bodies — it does NOT bound the eventual
	// OSS PUT, which stays a soft, client-declared size check.
	ossPresignBodyLimit = 4 << 10 // 4 KB
)

// ossWriteGate names the two write-authorization mechanisms.
type ossWriteGate int

const (
	gateAdminKey ossWriteGate = iota // OSS_ADMIN_KEY bearer (scripts)
	gateSelf                         // logged-in session, key scoped to uid
)

// ossScope is one upload category. Adding a category = one registry entry.
type ossScope struct {
	gate         ossWriteGate
	allowedTypes map[string]bool
	maxBytes     int64
	// prefix builds the object-key prefix; uid is "" for admin-key scopes.
	prefix func(uid string) string
}

func typeSet(types ...string) map[string]bool {
	m := make(map[string]bool, len(types))
	for _, t := range types {
		m[t] = true
	}
	return m
}

// ossScopes is the single source of truth for upload categories.
var ossScopes = map[string]ossScope{
	"web_resource": {
		gate:         gateAdminKey,
		allowedTypes: typeSet("image/png", "image/jpeg", "image/webp", "image/svg+xml"),
		maxBytes:     10 << 20, // 10 MB
		prefix:       func(string) string { return "web/" },
	},
	"course_material": {
		gate:         gateAdminKey,
		allowedTypes: typeSet("image/png", "image/jpeg", "image/webp", "application/pdf", "video/mp4", "video/webm", "video/quicktime"),
		maxBytes:     500 << 20, // 500 MB (carries course video)
		prefix:       func(string) string { return "courses/" },
	},
	"user_image": {
		gate:         gateSelf,
		allowedTypes: typeSet("image/png", "image/jpeg", "image/webp"),
		maxBytes:     10 << 20, // 10 MB
		prefix:       func(uid string) string { return "users/" + uid + "/images/" },
	},
	// user_doc carries an uploaded reading document (PDF / DOCX) that the server
	// then downloads and extracts to material blocks (ingest_file.go).
	"user_doc": {
		gate:         gateSelf,
		allowedTypes: typeSet("application/pdf", docxContentType),
		maxBytes:     30 << 20, // 30 MB
		prefix:       func(uid string) string { return "users/" + uid + "/docs/" },
	},
}

// docxContentType is the Office Open XML wordprocessing MIME type.
const docxContentType = "application/vnd.openxmlformats-officedocument.wordprocessingml.document"

// ossKnownPrefixes gates resolve-url to keys under our own scopes.
var ossKnownPrefixes = []string{"web/", "courses/", "users/"}

// extByContentType maps an allowed content type to a canonical file extension.
var extByContentType = map[string]string{
	"image/png":       ".png",
	"image/jpeg":      ".jpg",
	"image/webp":      ".webp",
	"image/svg+xml":   ".svg",
	"application/pdf": ".pdf",
	docxContentType:   ".docx",
	"video/mp4":       ".mp4",
	"video/webm":      ".webm",
	"video/quicktime": ".mov",
}

var safeExtRe = regexp.MustCompile(`^\.[a-z0-9]{1,8}$`)

// objectExt prefers the sanitized extension from the client filename, falling
// back to the content-type's canonical extension. The filename itself is never
// used as a path segment.
func objectExt(filename, contentType string) string {
	if filename != "" {
		if ext := strings.ToLower(path.Ext(filename)); safeExtRe.MatchString(ext) {
			return ext
		}
	}
	return extByContentType[contentType]
}

type ossUploadReq struct {
	Scope       string `json:"scope"`
	ContentType string `json:"contentType"`
	Size        int64  `json:"size"`
	Filename    string `json:"filename"`
}

type ossUploadResp struct {
	PutURL              string `json:"putUrl"`
	ObjectKey           string `json:"objectKey"`
	RequiredContentType string `json:"requiredContentType"`
	MaxBytes            int64  `json:"maxBytes"`
	ExpiresAt           string `json:"expiresAt"`
}

// ossBearer returns the token from an "Authorization: Bearer <token>" header.
func ossBearer(r *http.Request) string {
	const p = "Bearer "
	h := r.Header.Get("Authorization")
	if strings.HasPrefix(h, p) {
		return strings.TrimPrefix(h, p)
	}
	return ""
}

// ossAdminAuthed reports whether the request carries the correct admin key.
// A blank configured key never authenticates.
func (a *API) ossAdminAuthed(r *http.Request) bool {
	key := a.d.OSSAdminKey
	if key == "" {
		return false
	}
	got := ossBearer(r)
	if got == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(got), []byte(key)) == 1
}

// ossAdminUploadURL signs a presigned PUT URL for an admin-key scope
// (web_resource / course_material). Used by backend scripts; no session.
func (a *API) ossAdminUploadURL(w http.ResponseWriter, r *http.Request) {
	if a.d.OSS == nil || a.d.OSSAdminKey == "" {
		httpx.WriteError(w, r, httpx.ErrOSSUnavailable())
		return
	}
	if !a.ossAdminAuthed(r) {
		httpx.WriteError(w, r, httpx.ErrUnauthorized("需要有效的管理密钥"))
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, ossPresignBodyLimit)
	var req ossUploadReq
	if err := decodeJSON(r, &req); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	sc, ok := ossScopes[req.Scope]
	if !ok || sc.gate != gateAdminKey {
		httpx.WriteError(w, r, httpx.ErrBadRequest("unknown_scope", "未知的上传类型。", nil))
		return
	}
	a.writeUploadURL(w, r, sc, "", req)
}

// ossUserUploadURL signs a presigned PUT URL for a logged-in user's own object.
// The scope defaults to "user_image" (back-compat: the image uploader sends no
// scope); "user_doc" is also accepted. Only gateSelf scopes are permitted here.
func (a *API) ossUserUploadURL(w http.ResponseWriter, r *http.Request) {
	if a.d.OSS == nil {
		httpx.WriteError(w, r, httpx.ErrOSSUnavailable())
		return
	}
	u, ok := UserFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, r, httpx.ErrUnauthorized("需要登录"))
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, ossPresignBodyLimit)
	var req ossUploadReq
	if err := decodeJSON(r, &req); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	scopeName := req.Scope
	if scopeName == "" {
		scopeName = "user_image"
	}
	sc, ok := ossScopes[scopeName]
	if !ok || sc.gate != gateSelf {
		httpx.WriteError(w, r, httpx.ErrBadRequest("unknown_scope", "未知的上传类型。", nil))
		return
	}
	a.writeUploadURL(w, r, sc, u.ID.String(), req)
}

// writeUploadURL validates the request against the scope and writes a signed
// PUT URL. uid is interpolated into the key for uid-scoped scopes.
func (a *API) writeUploadURL(w http.ResponseWriter, r *http.Request, sc ossScope, uid string, req ossUploadReq) {
	if !sc.allowedTypes[req.ContentType] {
		httpx.WriteError(w, r, httpx.ErrBadRequest("unsupported_type", "不支持的文件类型。", nil))
		return
	}
	if req.Size <= 0 || req.Size > sc.maxBytes {
		httpx.WriteError(w, r, httpx.ErrBadRequest("file_too_large", "文件超出大小限制。", nil))
		return
	}
	objectKey := sc.prefix(uid) + uuid.NewString() + objectExt(req.Filename, req.ContentType)
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

type ossResolveReq struct {
	ObjectKey string `json:"objectKey"`
}

type ossResolveResp struct {
	URL       string `json:"url"`
	ExpiresAt string `json:"expiresAt"`
}

// ossResolveURL signs a short-lived GET URL (CDN domain) for one object.
// Authorized by a valid session OR the admin key.
func (a *API) ossResolveURL(w http.ResponseWriter, r *http.Request) {
	if a.d.OSS == nil {
		httpx.WriteError(w, r, httpx.ErrOSSUnavailable())
		return
	}
	_, sessionOK := UserFromContext(r.Context())
	if !sessionOK && !a.ossAdminAuthed(r) {
		httpx.WriteError(w, r, httpx.ErrUnauthorized("需要登录"))
		return
	}
	var req ossResolveReq
	if err := decodeJSON(r, &req); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if !validObjectKey(req.ObjectKey) {
		httpx.WriteError(w, r, httpx.ErrBadRequest("invalid_key", "无效的对象路径。", nil))
		return
	}
	url, err := a.d.OSS.SignDownload(req.ObjectKey)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, ossResolveResp{
		URL:       url,
		ExpiresAt: time.Now().Add(a.d.OSS.DownloadWindow()).UTC().Format(time.RFC3339),
	})
}

// validObjectKey rejects empty keys, path traversal, and keys outside our known
// scope prefixes.
func validObjectKey(key string) bool {
	if key == "" || strings.Contains(key, "..") {
		return false
	}
	for _, p := range ossKnownPrefixes {
		if strings.HasPrefix(key, p) {
			return true
		}
	}
	return false
}
