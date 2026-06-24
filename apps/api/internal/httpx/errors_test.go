package httpx

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
)

func decodeEnvelope(t *testing.T, body string) map[string]any {
	t.Helper()
	var got map[string]any
	if err := json.Unmarshal([]byte(body), &got); err != nil {
		t.Fatalf("body is not JSON: %v (%q)", err, body)
	}
	env, ok := got["error"].(map[string]any)
	if !ok {
		t.Fatalf("response missing error envelope: %q", body)
	}
	return env
}

func TestWriteError(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		wantStatus int
		wantCode   string
		// mustNotContain asserts internal detail does not leak into the body.
		mustNotContain string
	}{
		{
			name:       "APIError maps to its own status and code",
			err:        ErrBadRequest("validation_failed", "标题不能为空", []map[string]string{{"field": "title", "issue": "required"}}),
			wantStatus: http.StatusBadRequest,
			wantCode:   "validation_failed",
		},
		{
			name:       "not found constructor",
			err:        ErrNotFound("任务不存在"),
			wantStatus: http.StatusNotFound,
			wantCode:   "not_found",
		},
		{
			name:       "pgx.ErrNoRows maps to 404",
			err:        pgx.ErrNoRows,
			wantStatus: http.StatusNotFound,
			wantCode:   "not_found",
		},
		{
			name:           "unknown error maps to 500 and does not leak detail",
			err:            errInternalDetail("db password is hunter2 at /secret/path"),
			wantStatus:     http.StatusInternalServerError,
			wantCode:       "internal_error",
			mustNotContain: "hunter2",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/x", nil)
			WriteError(rec, req, tt.err)

			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d", rec.Code, tt.wantStatus)
			}
			if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
				t.Fatalf("Content-Type = %q", ct)
			}
			body := rec.Body.String()
			env := decodeEnvelope(t, body)
			if env["code"] != tt.wantCode {
				t.Fatalf("code = %v, want %v", env["code"], tt.wantCode)
			}
			if _, ok := env["message"].(string); !ok {
				t.Fatalf("message missing/not a string: %q", body)
			}
			if tt.mustNotContain != "" && strings.Contains(body, tt.mustNotContain) {
				t.Fatalf("internal detail leaked into body: %q", body)
			}
		})
	}
}

// errInternalDetail is a plain error carrying sensitive text, used to prove the
// 500 path never echoes it.
func errInternalDetail(msg string) error { return &plainErr{msg} }

type plainErr struct{ s string }

func (e *plainErr) Error() string { return e.s }
