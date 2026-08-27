// Package httpx holds the HTTP layer: router wiring, middleware, the JSON error
// envelope, health checks, and the server lifecycle.
package httpx

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/jackc/pgx/v5"
)

// ctxKey is an unexported type for context keys defined in this package.
type ctxKey string

const ctxKeyRequestID ctxKey = "request_id"

// requestIDFromContext returns the request id stored by the RequestID
// middleware, or "" if absent. It is the canonical accessor; middleware.go
// stores the value under the same key.
func requestIDFromContext(ctx context.Context) string {
	if v, ok := ctx.Value(ctxKeyRequestID).(string); ok {
		return v
	}
	return ""
}

// RequestIDFromContext is the exported accessor for packages outside httpx
// (e.g. the turn handler logging its request id). Delegates to the unexported
// canonical accessor so the context key stays private to this package.
func RequestIDFromContext(ctx context.Context) string { return requestIDFromContext(ctx) }

// APIError is a client-safe error with an HTTP status, a stable machine code, a
// human-readable message, and optional field-level details. It implements error.
type APIError struct {
	Status  int    `json:"-"`
	Code    string `json:"code"`
	Message string `json:"message"`
	Details any    `json:"details,omitempty"`
}

func (e *APIError) Error() string { return e.Code + ": " + e.Message }

// errorBody is the wire shape: {"error": {...}}.
type errorBody struct {
	Error *APIError `json:"error"`
}

// Constructors for the common status codes.

func ErrBadRequest(code, msg string, details any) *APIError {
	return &APIError{Status: http.StatusBadRequest, Code: code, Message: msg, Details: details}
}

func ErrUnauthorized(msg string) *APIError {
	return &APIError{Status: http.StatusUnauthorized, Code: "unauthorized", Message: msg}
}

func ErrForbidden(msg string) *APIError {
	return &APIError{Status: http.StatusForbidden, Code: "forbidden", Message: msg}
}

// ErrNotEntitled is the 403 returned by token-spending endpoints when the
// entitlement seam denies access. Distinct stable code for the client.
func ErrNotEntitled() *APIError {
	return &APIError{Status: http.StatusForbidden, Code: "not_entitled", Message: "当前没有可用额度"}
}

// ErrDemoReadonly — 403 for a write attempt against a read-only demo project.
func ErrDemoReadonly() *APIError {
	return &APIError{Status: http.StatusForbidden, Code: "demo_readonly", Message: "演示项目为只读，无法修改。"}
}

// ErrReadingFinished — 403 for a write attempt against a finished lite
// reading. Mirrors ErrDemoReadonly's shape (403, not 404: the reading
// genuinely exists and is hers — pretending otherwise would be confusing,
// not protective) but with its own stable code, since the two reasons a
// write is refused (this is someone else's read-only demo vs. this is mine
// but I already finished it) are not the same fact for a client to react to.
func ErrReadingFinished() *APIError {
	return &APIError{Status: http.StatusForbidden, Code: "reading_finished", Message: "这次阅读已完成，内容不能再修改。"}
}

// ErrWritingFinished — 403 for a write attempt against a finished lite
// writing. Mirrors ErrReadingFinished exactly (same shape, same reasoning:
// 403 not 404 because the writing genuinely exists and is hers), with its
// own stable code since a client needs to tell the two kinds apart.
func ErrWritingFinished() *APIError {
	return &APIError{Status: http.StatusForbidden, Code: "writing_finished", Message: "这篇写作已完成，内容不能再修改。"}
}

func ErrNotFound(msg string) *APIError {
	return &APIError{Status: http.StatusNotFound, Code: "not_found", Message: msg}
}

func ErrConflict(msg string) *APIError {
	return &APIError{Status: http.StatusConflict, Code: "conflict", Message: msg}
}

// ErrSourceLocked refuses to replace the article under a reading that already
// has process evidence hanging off it. Block ids are POSITIONAL and anchors
// carry rune offsets into the old text, so swapping the body would silently
// re-point every card, highlight and margin note at unrelated prose. 铁律④
// makes those rows evidence; corrupting them quietly is worse than refusing.
func ErrSourceLocked() *APIError {
	return &APIError{
		Status: http.StatusConflict, Code: "source_locked",
		Message: "这篇文章已经有了阅读痕迹（卡片 / 批注 / 对话），不能再换正文了——换一篇的话，新建一次阅读。",
	}
}

// P2 auth error codes — stable machine codes the SPA maps to inline messages.

func ErrEmailTaken() *APIError {
	return &APIError{Status: http.StatusConflict, Code: "email_taken", Message: "该邮箱已被注册"}
}

func ErrInvalidJoinCode() *APIError {
	return &APIError{Status: http.StatusBadRequest, Code: "invalid_join_code", Message: "班级邀请码无效"}
}

func ErrEmailUnverified() *APIError {
	return &APIError{Status: http.StatusForbidden, Code: "email_unverified", Message: "邮箱尚未验证"}
}

func ErrTokenInvalid() *APIError {
	return &APIError{Status: http.StatusBadRequest, Code: "token_invalid_or_expired", Message: "验证链接无效或已过期"}
}

func ErrInvalidCredentials() *APIError {
	return &APIError{Status: http.StatusUnauthorized, Code: "invalid_credentials", Message: "邮箱或密码错误"}
}

// ErrVoiceUnavailable is the 503 returned when the voice feature is not
// configured (Deps.Voice is nil) — the platform must still boot without it.
func ErrVoiceUnavailable() *APIError {
	return &APIError{Status: http.StatusServiceUnavailable, Code: "voice_unavailable", Message: "语音服务未启用"}
}

// ErrOSSUnavailable is the 503 returned when object storage is not configured
// (Deps.OSS is nil, or the admin routes have no OSS_ADMIN_KEY) — the platform
// must still boot without it.
func ErrOSSUnavailable() *APIError {
	return &APIError{Status: http.StatusServiceUnavailable, Code: "oss_disabled", Message: "文件存储暂未开启。"}
}

// ErrInternal is the generic, client-safe 500. Real detail is logged, never sent.
func ErrInternal() *APIError {
	return &APIError{Status: http.StatusInternalServerError, Code: "internal_error", Message: "服务器内部错误"}
}

// ErrAIDialogueFailed is the 502 returned by AI-dialogue endpoints when the model
// call cannot be made or its reply cannot be understood. We deliberately do NOT
// fabricate a plausible-looking assistant sentence in this case: a fake reply
// disguises the failure as normal conversation, so the student keeps talking to a
// dead turn (the "canned opener repeats forever" bug). Surface it as a real error
// instead. `reason` is a short, secret-free machine detail (also logged); model
// API keys live only in headers, never in these error strings.
func ErrAIDialogueFailed(reason string) *APIError {
	return &APIError{Status: http.StatusBadGateway, Code: "ai_dialogue_failed", Message: "AI 暂时没接上，请重试。", Details: reason}
}

// WriteJSON marshals v and writes it with the given status. On marshal failure
// it falls back to a bare 500 without leaking the marshal error to the client.
func WriteJSON(w http.ResponseWriter, status int, v any) {
	buf, err := json.Marshal(v)
	if err != nil {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":{"code":"internal_error","message":"服务器内部错误"}}`))
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_, _ = w.Write(buf)
}

// WriteError maps any error to the JSON envelope:
//   - *APIError       → its own Status/Code/Message/Details
//   - pgx.ErrNoRows   → 404 not_found
//   - anything else   → 500 internal_error, with the real error logged via slog
//     (carrying the request id) and NEVER echoed to the client.
func WriteError(w http.ResponseWriter, r *http.Request, err error) {
	var apiErr *APIError
	switch {
	case errors.As(err, &apiErr):
		// client-safe by construction
	case errors.Is(err, pgx.ErrNoRows):
		apiErr = ErrNotFound("资源不存在")
	default:
		// Log the real cause server-side only; respond generically.
		slog.Error("unhandled error",
			"request_id", requestIDFromContext(r.Context()),
			"method", r.Method,
			"path", r.URL.Path,
			"err", err.Error(),
		)
		apiErr = ErrInternal()
	}
	WriteJSON(w, apiErr.Status, errorBody{Error: apiErr})
}
