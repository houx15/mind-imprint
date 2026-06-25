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

func ErrNotFound(msg string) *APIError {
	return &APIError{Status: http.StatusNotFound, Code: "not_found", Message: msg}
}

func ErrConflict(msg string) *APIError {
	return &APIError{Status: http.StatusConflict, Code: "conflict", Message: msg}
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

// ErrInternal is the generic, client-safe 500. Real detail is logged, never sent.
func ErrInternal() *APIError {
	return &APIError{Status: http.StatusInternalServerError, Code: "internal_error", Message: "服务器内部错误"}
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
