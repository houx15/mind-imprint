# P1.1 · Go Service Foundation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Stand up the `apps/api/` Go service foundation: a service that compiles, loads typed config from env, connects to Postgres via a pgx pool, applies goose migrations + a seed, exposes a type-safe sqlc store proven against a real Postgres (testcontainers), embeds the 33 card JSON specs via a synced mirror + drift check, serves health endpoints behind middleware with a JSON error envelope, and shuts down gracefully. No agent loop, no LLM gateway, no real auth, no HTTP business endpoints beyond health.

**Architecture:** A single Go binary (`mindimprint/api`) laid out as `cmd/api` (wiring) + `internal/*` (private packages: `config`, `httpx`, `store`, `cards`) + `migrations/`. The HTTP layer is the standard-library `net/http` ServeMux wrapped by a hand-written middleware chain; the data layer is `pgx/v5` + `pgxpool` with sqlc-generated, type-safe queries; the card catalog is a `go:embed` of a generated mirror of the canonical `packages/contracts/cards/` directory, guarded by a byte-equality drift test.

**Tech Stack:** Go 1.26 · `net/http` ServeMux · `pgx/v5` + `pgxpool` · `sqlc` (v2, pgx/v5) · `goose` (programmatic) · `caarlos0/env` v11 + `joho/godotenv` · `rs/cors` · `log/slog` · `google/uuid` · `testcontainers-go` (postgres module) · distroless runtime image.

## Global Constraints

- Module path `mindimprint/api`; Go 1.26.
- Secrets (DB DSN, provider keys) come only from env; `.env*` is gitignored; never commit secrets, never log them, never put them in error responses. Commit `.env.example` with blank values only.
- The repo-root `package.json` has an unrelated uncommitted change — do NOT stage, modify, or commit it. Only touch files under `apps/api/` and the plan/docs.
- The standard envelope is stored as JSONB validated at the edge: `card_instances.field_values jsonb default '{}'`, `event_trace jsonb default '[]'`; the deep shape stays the TS Zod contract. Do not add per-field columns.
- Canonical card source is `packages/contracts/cards/`; Go uses a generated mirror at `internal/cards/specs/` + a drift-check test. Never hand-edit the mirror.
- DB conventions: uuid PK `gen_random_uuid()`, `timestamptz` UTC, text columns + CHECK for enums — exactly as in `database-schema.md`.
- TDD: every task writes a failing test first; frequent commits; each commit message ends with the trailer `Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>`.
- Tests: table-driven + `httptest` for handlers; `testcontainers-go` (postgres module) for the store/migration layer. No mocking of SQL.

---

## File Structure

All paths are under `apps/api/` unless noted.

| Path | Responsibility |
|---|---|
| `go.mod` / `go.sum` | Module `mindimprint/api`, Go 1.26, dependency pins. |
| `.gitignore` | Ignore `.env*` (keep `.env.example`) and build artifacts. |
| `.env.example` | Documents env vars (PORT, DATABASE_URL, CORS_ORIGINS, ANTHROPIC_API_KEY, DEEPSEEK_API_KEY) with blank/example values. |
| `Makefile` | `sync-cards`, `sqlc`, `migrate-up`, `test`, `run` targets. |
| `sqlc.yaml` | sqlc v2 config (postgresql engine, pgx/v5 driver). |
| `Dockerfile` | Multi-stage build → `gcr.io/distroless/static:nonroot`. |
| `cmd/api/main.go` | Single entrypoint: load config, open pool, build router, run server with graceful shutdown. |
| `internal/config/config.go` | `Config` struct + `Load()` (godotenv + caarlos0/env, fail-fast). |
| `internal/config/config_test.go` | Table-driven config tests. |
| `internal/httpx/errors.go` | JSON error envelope: `APIError`, constructors, `WriteError`, `WriteJSON`. |
| `internal/httpx/errors_test.go` | Mapping branch tests (status + body shape + no-leak on 500). |
| `internal/httpx/middleware.go` | `chain`, `RequestID`, `Recover`, `Logger`, `CORS` + request-id context helpers. |
| `internal/httpx/middleware_test.go` | Request-id header, panic→500, CORS preflight tests. |
| `internal/httpx/server.go` | `NewServer` (ServeMux + timeouts) and `RunServer` (graceful shutdown). |
| `internal/httpx/health.go` | `Pinger` interface + `Healthz` / `Readyz` handlers. |
| `internal/httpx/health_test.go` | `/healthz` 200, `/readyz` healthy→200 / failing→503. |
| `internal/store/db.go` | `NewPool(ctx, dsn)` pgxpool constructor. |
| `internal/store/migrate.go` | `RunMigrations(ctx, pool)` applying embedded goose migrations. |
| `internal/store/migrations/0001_init.sql` | P1.1 tables + `llm_usage` view (goose up/down). |
| `internal/store/migrations/0002_seed.sql` | Seed school/class/student/enrollment (goose up/down). |
| `internal/store/migrate_test.go` | testcontainers: migrate up, assert tables + seed + school invariant. |
| `internal/store/queries/users.sql` | `GetUserByID`. |
| `internal/store/queries/tasks.sql` | `CreateTask`, `GetTask`, `ListTasksByUser`. |
| `internal/store/queries/messages.sql` | `AppendMessage`, `ListMessagesByTask`. |
| `internal/store/sqlc/` | sqlc-generated `db.go`, `models.go`, `*.sql.go` (never hand-edited). |
| `internal/store/sqlc_test.go` | testcontainers: round-trip task + messages, GetTask. |
| `internal/cards/loader.go` | `//go:embed specs/*.json`, `Spec` type, `Catalog()`, `ByID()`. |
| `internal/cards/specs/*.json` | Synced mirror of the 33 canonical card specs (committed). |
| `internal/cards/loader_test.go` | Catalog loads 33, unique ids, known id resolves, drift check. |
| `tools/synccards/main.go` | `go run` tool copying canonical cards → `internal/cards/specs/`. |

---

### Task 1: Module + plumbing

**Files:**
- Create: `apps/api/go.mod`
- Create: `apps/api/.gitignore`
- Create: `apps/api/.env.example`
- Create: `apps/api/Makefile`
- Create: `apps/api/cmd/api/main.go` (trivial; expanded in Task 5)

**Interfaces:**
- Consumes: nothing.
- Produces: a compilable module `mindimprint/api`; `func main()` in `package main` at `cmd/api`.

- [ ] **Step 1: Create `apps/api/go.mod`**

`apps/api/go.mod`:
```
module mindimprint/api

go 1.26
```

- [ ] **Step 2: Create `apps/api/.gitignore`**

`apps/api/.gitignore`:
```
# secrets — never commit
.env
.env.*
!.env.example

# build artifacts
/api
/bin/
*.out
```

- [ ] **Step 3: Create `apps/api/.env.example`** (blank/example values only)

`apps/api/.env.example`:
```
# HTTP listen port
PORT=8080

# Postgres DSN, e.g. postgres://user:pass@localhost:5432/mindimprint?sslmode=disable
DATABASE_URL=

# Comma-separated list of allowed SPA origins, e.g. http://localhost:5173
CORS_ORIGINS=

# LLM provider keys (server-side only; never sent to the browser)
ANTHROPIC_API_KEY=
DEEPSEEK_API_KEY=
```

- [ ] **Step 4: Create `apps/api/Makefile`** (real commands; no placeholders)

`apps/api/Makefile`:
```makefile
.PHONY: sync-cards sqlc migrate-up test run

# Mirror the canonical card JSON specs into the embeddable directory.
sync-cards:
	go run ./tools/synccards

# Regenerate the type-safe sqlc store from queries/ + migrations/.
sqlc:
	go tool sqlc generate

# Apply goose migrations against DATABASE_URL (env or .env.local).
migrate-up:
	go run ./cmd/api -migrate-up

# Run the full Go test suite (unit + testcontainers integration).
test:
	go test ./...

# Run the API server locally.
run:
	go run ./cmd/api
```

> Note: `go tool sqlc` and the `-migrate-up` flag are wired in Tasks 7 and 6 respectively. The Makefile is written now so later tasks only add their backing code; the targets are real commands, not stubs.

- [ ] **Step 5: Create a trivial `cmd/api/main.go`**

`apps/api/cmd/api/main.go`:
```go
package main

import (
	"fmt"
	"os"
)

func main() {
	fmt.Fprintln(os.Stdout, "mindimprint/api: foundation booting")
}
```

- [ ] **Step 6: Run the deliverable gate**

```
cd apps/api && go vet ./... && go build ./...
```
Expected output: no errors, no output from `go vet`; `go build` produces no diagnostics and exits 0.

- [ ] **Step 7: Commit**

```
git add apps/api/go.mod apps/api/.gitignore apps/api/.env.example apps/api/Makefile apps/api/cmd/api/main.go
git commit -m "feat(api): bootstrap Go module + plumbing (P1.1 task 1)

Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>"
```

---

### Task 2: Config (`internal/config`)

**Files:**
- Create: `apps/api/internal/config/config.go`
- Create: `apps/api/internal/config/config_test.go`

**Interfaces:**
- Consumes: environment variables; optional `.env.local`.
- Produces:
  - `type Config struct { Port string; DatabaseURL string; CORSOrigins []string; AnthropicKey string; DeepSeekKey string }`
  - `func Load() (Config, error)`

- [ ] **Step 1: Add dependencies**

```
cd apps/api && go get github.com/caarlos0/env/v11@v11.3.1 && go get github.com/joho/godotenv@v1.5.1
```
Expected output: `go.mod`/`go.sum` updated with both modules; no errors.

- [ ] **Step 2: Write the failing test**

`apps/api/internal/config/config_test.go`:
```go
package config

import (
	"os"
	"reflect"
	"testing"
)

// allEnvKeys are every variable Load reads. Each subtest starts from a clean
// slate by unsetting all of them, then setting only what the case needs.
var allEnvKeys = []string{
	"PORT", "DATABASE_URL", "CORS_ORIGINS", "ANTHROPIC_API_KEY", "DEEPSEEK_API_KEY",
}

func clearEnv(t *testing.T) {
	t.Helper()
	for _, k := range allEnvKeys {
		if err := os.Unsetenv(k); err != nil {
			t.Fatalf("unsetenv %s: %v", k, err)
		}
	}
}

func TestLoad(t *testing.T) {
	tests := []struct {
		name    string
		env     map[string]string
		wantErr bool
		check   func(t *testing.T, c Config)
	}{
		{
			name:    "missing required DATABASE_URL is an error",
			env:     map[string]string{},
			wantErr: true,
		},
		{
			name: "all set parses values and splits CORS origins",
			env: map[string]string{
				"PORT":              "9090",
				"DATABASE_URL":      "postgres://localhost/db",
				"CORS_ORIGINS":      "http://localhost:5173,https://app.example.com",
				"ANTHROPIC_API_KEY": "ak-test",
				"DEEPSEEK_API_KEY":  "dk-test",
			},
			wantErr: false,
			check: func(t *testing.T, c Config) {
				if c.Port != "9090" {
					t.Fatalf("Port = %q, want 9090", c.Port)
				}
				if c.DatabaseURL != "postgres://localhost/db" {
					t.Fatalf("DatabaseURL = %q", c.DatabaseURL)
				}
				want := []string{"http://localhost:5173", "https://app.example.com"}
				if !reflect.DeepEqual(c.CORSOrigins, want) {
					t.Fatalf("CORSOrigins = %#v, want %#v", c.CORSOrigins, want)
				}
				if c.AnthropicKey != "ak-test" || c.DeepSeekKey != "dk-test" {
					t.Fatalf("keys = %q/%q", c.AnthropicKey, c.DeepSeekKey)
				}
			},
		},
		{
			name: "PORT defaults to 8080 when unset",
			env: map[string]string{
				"DATABASE_URL": "postgres://localhost/db",
			},
			wantErr: false,
			check: func(t *testing.T, c Config) {
				if c.Port != "8080" {
					t.Fatalf("Port = %q, want default 8080", c.Port)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Start clean, then set only this case's vars. t.Setenv restores the
			// process environment automatically after the subtest.
			clearEnv(t)
			for k, v := range tt.env {
				t.Setenv(k, v)
			}
			got, err := Load()
			if tt.wantErr {
				if err == nil {
					t.Fatalf("Load() error = nil, want error")
				}
				return
			}
			if err != nil {
				t.Fatalf("Load() unexpected error: %v", err)
			}
			if tt.check != nil {
				tt.check(t, got)
			}
		})
	}
}
```

> Why `os.Unsetenv` + `t.Setenv`: `caarlos0/env` v11 treats a `required` field as satisfied if the variable is *present at all*, even when empty. So the "missing required" case must truly remove `DATABASE_URL` from the environment (`os.Unsetenv`), not set it to `""`. `clearEnv` removes all five first; `t.Setenv` then sets only the case's vars and auto-restores the original environment when the subtest ends. If the developer's shell happens to export any of these (e.g. via a sourced `.env.local`), `clearEnv` still wins because it runs inside the subtest. Note: `Load` calls `godotenv.Load(".env.local")`, which is a no-op here because the test runs in the package dir where no `.env.local` exists.

- [ ] **Step 3: Run the test — it fails (no `Config`/`Load` yet)**

```
cd apps/api && go test ./internal/config/...
```
Expected output: compile failure — `undefined: Config`, `undefined: Load`.

- [ ] **Step 4: Implement `config.go`**

`apps/api/internal/config/config.go`:
```go
// Package config loads typed service configuration from the environment.
//
// 12-factor: configuration lives in env vars. A local .env.local is loaded as a
// developer convenience only and never committed. Missing required values are a
// fatal, fail-fast error at boot.
package config

import (
	"github.com/caarlos0/env/v11"
	"github.com/joho/godotenv"
)

// Config is the fully-resolved service configuration.
type Config struct {
	// Port is the HTTP listen port; defaults to 8080.
	Port string `env:"PORT" envDefault:"8080"`
	// DatabaseURL is the Postgres DSN. Required.
	DatabaseURL string `env:"DATABASE_URL,required"`
	// CORSOrigins is the comma-separated allowlist of SPA origins.
	CORSOrigins []string `env:"CORS_ORIGINS" envSeparator:","`
	// AnthropicKey is the Anthropic provider key (server-side only).
	AnthropicKey string `env:"ANTHROPIC_API_KEY"`
	// DeepSeekKey is the DeepSeek provider key (server-side only).
	DeepSeekKey string `env:"DEEPSEEK_API_KEY"`
}

// Load reads .env.local if present (ignored if absent), then parses the
// environment into a Config. Returns an error if a required value is missing.
func Load() (Config, error) {
	// Best-effort local file; absence is not an error.
	_ = godotenv.Load(".env.local")
	return env.ParseAs[Config]()
}
```

- [ ] **Step 5: Run the test — it passes**

```
cd apps/api && go test ./internal/config/...
```
Expected output: `ok  	mindimprint/api/internal/config	<time>`.

- [ ] **Step 6: Tidy + commit**

```
cd apps/api && go mod tidy
git add apps/api/go.mod apps/api/go.sum apps/api/internal/config/
git commit -m "feat(api): typed env config with fail-fast loader (P1.1 task 2)

Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>"
```

---

### Task 3: Error envelope (`internal/httpx/errors.go`)

**Files:**
- Create: `apps/api/internal/httpx/errors.go`
- Create: `apps/api/internal/httpx/errors_test.go`

**Interfaces:**
- Consumes: `slog`, `pgx.ErrNoRows`, the request-id context value from Task 4 (forward-declared here as a package function `requestIDFromContext`).
- Produces:
  - `type APIError struct { Status int; Code string; Message string; Details any }` implementing `error` via `func (e *APIError) Error() string`.
  - Constructors: `func ErrBadRequest(code, msg string, details any) *APIError`, `func ErrUnauthorized(msg string) *APIError`, `func ErrForbidden(msg string) *APIError`, `func ErrNotFound(msg string) *APIError`, `func ErrConflict(msg string) *APIError`, `func ErrInternal() *APIError`.
  - `func WriteJSON(w http.ResponseWriter, status int, v any)`
  - `func WriteError(w http.ResponseWriter, r *http.Request, err error)`

> **Ownership of the request-id context plumbing:** `errors.go` (this task) is the canonical home for both `ctxKeyRequestID` (the unexported context key) and `requestIDFromContext(ctx) string` (the accessor). The `RequestID` middleware in Task 4 *stores* a value under that same key and *reads* nothing — it only writes. So both symbols are defined exactly once, here, and Task 4 references them without redefining. This keeps Task 3 independently compilable and avoids any duplicate-symbol clash.

- [ ] **Step 1: Add pgx dependency**

```
cd apps/api && go get github.com/jackc/pgx/v5@v5.7.2
```
Expected output: `go.mod`/`go.sum` updated; no errors.

- [ ] **Step 2: Write the failing test**

`apps/api/internal/httpx/errors_test.go`:
```go
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
```

- [ ] **Step 3: Run the test — it fails**

```
cd apps/api && go test ./internal/httpx/...
```
Expected output: compile failure — `undefined: ErrBadRequest`, `undefined: WriteError`, etc.

- [ ] **Step 4: Implement `errors.go`**

`apps/api/internal/httpx/errors.go`:
```go
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

func ErrNotFound(msg string) *APIError {
	return &APIError{Status: http.StatusNotFound, Code: "not_found", Message: msg}
}

func ErrConflict(msg string) *APIError {
	return &APIError{Status: http.StatusConflict, Code: "conflict", Message: msg}
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
```

- [ ] **Step 5: Run the test — it passes**

```
cd apps/api && go test ./internal/httpx/...
```
Expected output: `ok  	mindimprint/api/internal/httpx	<time>`.

- [ ] **Step 6: Commit**

```
git add apps/api/go.mod apps/api/go.sum apps/api/internal/httpx/errors.go apps/api/internal/httpx/errors_test.go
git commit -m "feat(api): JSON error envelope with safe 500 mapping (P1.1 task 3)

Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>"
```

---

### Task 4: Middleware (`internal/httpx/middleware.go`)

**Files:**
- Create: `apps/api/internal/httpx/middleware.go`
- Create: `apps/api/internal/httpx/middleware_test.go`

**Interfaces:**
- Consumes: `ctxKeyRequestID` + `requestIDFromContext` from `errors.go` (Task 3); `WriteError`; `config.Config.CORSOrigins`.
- Produces:
  - `func chain(h http.Handler, mw ...func(http.Handler) http.Handler) http.Handler`
  - `func RequestID(next http.Handler) http.Handler`
  - `func Recover(next http.Handler) http.Handler`
  - `func Logger(next http.Handler) http.Handler`
  - `func CORS(origins []string) func(http.Handler) http.Handler`

- [ ] **Step 1: Add dependencies**

```
cd apps/api && go get github.com/google/uuid@v1.6.0 && go get github.com/rs/cors@v1.11.1
```
Expected output: `go.mod`/`go.sum` updated; no errors.

- [ ] **Step 2: Write the failing test**

`apps/api/internal/httpx/middleware_test.go`:
```go
package httpx

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRequestID(t *testing.T) {
	var seen string
	h := chain(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = requestIDFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	}), RequestID)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	h.ServeHTTP(rec, req)

	if seen == "" {
		t.Fatal("request id not present in context")
	}
	if got := rec.Header().Get("X-Request-ID"); got != seen {
		t.Fatalf("X-Request-ID header = %q, context = %q", got, seen)
	}
}

func TestRecover(t *testing.T) {
	h := chain(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("boom")
	}), RequestID, Recover)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	// Must not propagate the panic.
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
	env := decodeEnvelope(t, rec.Body.String())
	if env["code"] != "internal_error" {
		t.Fatalf("code = %v, want internal_error", env["code"])
	}
}

func TestCORSPreflight(t *testing.T) {
	allowed := "http://localhost:5173"
	h := chain(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}), CORS([]string{allowed}))

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodOptions, "/x", nil)
	req.Header.Set("Origin", allowed)
	req.Header.Set("Access-Control-Request-Method", "POST")
	h.ServeHTTP(rec, req)

	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != allowed {
		t.Fatalf("Allow-Origin = %q, want %q", got, allowed)
	}
	if got := rec.Header().Get("Access-Control-Allow-Credentials"); got != "true" {
		t.Fatalf("Allow-Credentials = %q, want true", got)
	}
}
```

- [ ] **Step 3: Run the test — it fails**

```
cd apps/api && go test ./internal/httpx/...
```
Expected output: compile failure — `undefined: chain`, `undefined: RequestID`, `undefined: Recover`, `undefined: CORS`.

- [ ] **Step 4: Implement `middleware.go`**

`apps/api/internal/httpx/middleware.go`:
```go
package httpx

import (
	"context"
	"log/slog"
	"net/http"
	"runtime/debug"
	"time"

	"github.com/google/uuid"
	"github.com/rs/cors"
)

// chain composes middleware around h. The first middleware in mw is the
// outermost (runs first on the way in, last on the way out).
func chain(h http.Handler, mw ...func(http.Handler) http.Handler) http.Handler {
	for i := len(mw) - 1; i >= 0; i-- {
		h = mw[i](h)
	}
	return h
}

// statusRecorder captures the response status for logging.
type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (s *statusRecorder) WriteHeader(code int) {
	s.status = code
	s.ResponseWriter.WriteHeader(code)
}

// Flush implements http.Flusher when the underlying writer supports it, so SSE
// handlers (later phases) keep working through the chain.
func (s *statusRecorder) Flush() {
	if f, ok := s.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

// RequestID assigns each request a UUID, stores it in the context, and echoes it
// in the X-Request-ID response header.
func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := uuid.NewString()
		w.Header().Set("X-Request-ID", id)
		ctx := context.WithValue(r.Context(), ctxKeyRequestID, id)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// Recover converts a panic in a downstream handler into a 500 JSON envelope,
// logging the stack and the request id. The server stays up.
func Recover(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				slog.Error("panic recovered",
					"request_id", requestIDFromContext(r.Context()),
					"method", r.Method,
					"path", r.URL.Path,
					"panic", rec,
					"stack", string(debug.Stack()),
				)
				WriteError(w, r, ErrInternal())
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// Logger emits one structured JSON line per request: method, path, status, dur,
// and request id.
func Logger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		sr := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(sr, r)
		slog.Info("request",
			"request_id", requestIDFromContext(r.Context()),
			"method", r.Method,
			"path", r.URL.Path,
			"status", sr.status,
			"dur", time.Since(start).String(),
		)
	})
}

// CORS configures rs/cors from the allowed origins. Credentials are allowed (the
// SPA sends the session cookie), so the origin is never "*"; it is echoed exactly
// when it matches the allowlist.
func CORS(origins []string) func(http.Handler) http.Handler {
	c := cors.New(cors.Options{
		AllowedOrigins:   origins,
		AllowCredentials: true,
		AllowedMethods:   []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodOptions},
		AllowedHeaders:   []string{"Content-Type", "X-CSRF-Token"},
	})
	return c.Handler
}
```

- [ ] **Step 5: Run the test — it passes**

```
cd apps/api && go test ./internal/httpx/...
```
Expected output: `ok  	mindimprint/api/internal/httpx	<time>`.

- [ ] **Step 6: Commit**

```
git add apps/api/go.mod apps/api/go.sum apps/api/internal/httpx/middleware.go apps/api/internal/httpx/middleware_test.go
git commit -m "feat(api): request-id, recover, logger, CORS middleware (P1.1 task 4)

Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>"
```

---

### Task 5: HTTP server + health + graceful shutdown

**Files:**
- Create: `apps/api/internal/httpx/health.go`
- Create: `apps/api/internal/httpx/health_test.go`
- Create: `apps/api/internal/httpx/server.go`
- Modify: `apps/api/cmd/api/main.go`

**Interfaces:**
- Consumes: `config.Config`, the middleware from Task 4, the error envelope from Task 3.
- Produces:
  - `type Pinger interface { Ping(ctx context.Context) error }`
  - `func Healthz(w http.ResponseWriter, r *http.Request)`
  - `func Readyz(p Pinger) http.HandlerFunc`
  - `func NewServer(cfg config.Config, p Pinger) *http.Server`
  - `func RunServer(srv *http.Server, onShutdown func(context.Context)) error`

- [ ] **Step 1: Write the failing health test**

`apps/api/internal/httpx/health_test.go`:
```go
package httpx

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

type stubPinger struct{ err error }

func (s stubPinger) Ping(ctx context.Context) error { return s.err }

func TestHealthz(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	Healthz(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if rec.Body.String() != "ok" {
		t.Fatalf("body = %q, want ok", rec.Body.String())
	}
}

func TestReadyz(t *testing.T) {
	tests := []struct {
		name       string
		pinger     Pinger
		wantStatus int
	}{
		{"healthy → 200", stubPinger{nil}, http.StatusOK},
		{"db down → 503", stubPinger{errors.New("connection refused")}, http.StatusServiceUnavailable},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
			Readyz(tt.pinger).ServeHTTP(rec, req)
			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d", rec.Code, tt.wantStatus)
			}
			if tt.wantStatus == http.StatusServiceUnavailable {
				env := decodeEnvelope(t, rec.Body.String())
				if env["code"] != "unavailable" {
					t.Fatalf("code = %v, want unavailable", env["code"])
				}
			}
		})
	}
}
```

- [ ] **Step 2: Run the test — it fails**

```
cd apps/api && go test ./internal/httpx/...
```
Expected output: compile failure — `undefined: Healthz`, `undefined: Readyz`, `undefined: Pinger`.

- [ ] **Step 3: Implement `health.go`**

`apps/api/internal/httpx/health.go`:
```go
package httpx

import (
	"context"
	"net/http"
	"time"
)

// Pinger is the minimal dependency Readyz needs — satisfied by *pgxpool.Pool.
type Pinger interface {
	Ping(ctx context.Context) error
}

// Healthz is liveness: the process is up. Always 200 "ok".
func Healthz(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
}

// Readyz is readiness: dependencies are reachable. Pings the pool with a short
// timeout; 200 when ok, 503 envelope when not.
func Readyz(p Pinger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		if err := p.Ping(ctx); err != nil {
			WriteError(w, r, &APIError{
				Status:  http.StatusServiceUnavailable,
				Code:    "unavailable",
				Message: "依赖未就绪",
			})
			return
		}
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ready"))
	}
}
```

- [ ] **Step 4: Run the health test — it passes**

```
cd apps/api && go test ./internal/httpx/...
```
Expected output: `ok  	mindimprint/api/internal/httpx	<time>`.

- [ ] **Step 5: Implement `server.go`**

`apps/api/internal/httpx/server.go`:
```go
package httpx

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"mindimprint/api/internal/config"
)

// NewServer builds the fully-wired HTTP server: ServeMux with health routes,
// the middleware chain, and a ReadHeaderTimeout to blunt slow-loris attacks.
func NewServer(cfg config.Config, p Pinger) *http.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", Healthz)
	mux.Handle("GET /readyz", Readyz(p))

	handler := chain(mux, RequestID, Recover, Logger, CORS(cfg.CORSOrigins))

	return &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
	}
}

// RunServer starts srv and blocks until SIGINT/SIGTERM, then shuts down
// gracefully (stop accepting, drain in-flight) and runs onShutdown (e.g. closing
// the pool) within a bounded window.
func RunServer(srv *http.Server, onShutdown func(context.Context)) error {
	errc := make(chan error, 1)
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errc <- err
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	select {
	case err := <-errc:
		return err
	case <-stop:
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		shutdownErr := srv.Shutdown(ctx)
		if onShutdown != nil {
			onShutdown(ctx)
		}
		return shutdownErr
	}
}
```

- [ ] **Step 6 (forward reference — DO NOT apply in Task 5): the target `cmd/api/main.go`**

This is the final wiring, shown here for completeness because `server.go` above is its consumer. **It depends on `store.NewPool` / `store.RunMigrations`, which do not exist until Task 6, so it does NOT compile in Task 5.** Leave `cmd/api/main.go` at its trivial Task-1 form during Task 5; this exact file is written in Task 6 Step 10.

`apps/api/cmd/api/main.go` (target — applied in Task 6):
```go
package main

import (
	"context"
	"flag"
	"log/slog"
	"os"

	"mindimprint/api/internal/config"
	"mindimprint/api/internal/httpx"
	"mindimprint/api/internal/store"
)

func main() {
	migrateUp := flag.Bool("migrate-up", false, "apply migrations then exit")
	flag.Parse()

	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))

	cfg, err := config.Load()
	if err != nil {
		slog.Error("config load failed", "err", err.Error())
		os.Exit(1)
	}

	ctx := context.Background()
	pool, err := store.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		slog.Error("db pool failed", "err", err.Error())
		os.Exit(1)
	}

	if *migrateUp {
		if err := store.RunMigrations(ctx, pool); err != nil {
			slog.Error("migrations failed", "err", err.Error())
			pool.Close()
			os.Exit(1)
		}
		slog.Info("migrations applied")
		pool.Close()
		return
	}

	srv := httpx.NewServer(cfg, pool)
	slog.Info("listening", "addr", srv.Addr)
	if err := httpx.RunServer(srv, func(context.Context) { pool.Close() }); err != nil {
		slog.Error("server error", "err", err.Error())
		os.Exit(1)
	}
}
```

> `main.go` now imports `store.NewPool` and `store.RunMigrations`, which are created in Task 6. **Execution order requirement:** Task 6 must land before `cmd/api/main.go` compiles. To keep each task independently green, do Step 6 **as part of Task 6's final step** (re-wiring main), not here. In Task 5, leave `cmd/api/main.go` at its Task 1 trivial form and only add the health/server packages. The version above is the *target* main written at the end of Task 6.

Corrected Task 5 `cmd/api/main.go` stays the trivial Task-1 file (unchanged). Verify the package still builds:

```
cd apps/api && go build ./...
```
Expected output: builds clean (server.go references only `config`, which exists).

- [ ] **Step 7: Run the full suite + commit**

```
cd apps/api && go test ./...
```
Expected output: `ok` for `internal/config` and `internal/httpx`; `cmd/api` reports `no test files`.

```
git add apps/api/internal/httpx/health.go apps/api/internal/httpx/health_test.go apps/api/internal/httpx/server.go
git commit -m "feat(api): health endpoints + server lifecycle with graceful shutdown (P1.1 task 5)

Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>"
```

---

### Task 6: DB pool + migrations

**Files:**
- Create: `apps/api/internal/store/db.go`
- Create: `apps/api/internal/store/migrate.go`
- Create: `apps/api/internal/store/migrations/0001_init.sql`
- Create: `apps/api/internal/store/migrations/0002_seed.sql`
- Create: `apps/api/internal/store/migrate_test.go`
- Modify: `apps/api/cmd/api/main.go` (to the target version from Task 5 Step 6)

**Interfaces:**
- Consumes: `cfg.DatabaseURL`.
- Produces:
  - `func NewPool(ctx context.Context, dsn string) (*pgxpool.Pool, error)`
  - `func RunMigrations(ctx context.Context, pool *pgxpool.Pool) error`
  - Embedded migration files satisfying `httpx.Pinger` via `*pgxpool.Pool.Ping`.

- [ ] **Step 1: Add dependencies**

```
cd apps/api && go get github.com/jackc/pgx/v5/pgxpool@v5.7.2 && go get github.com/pressly/goose/v3@v3.24.1 && go get github.com/jackc/pgx/v5/stdlib@v5.7.2
```
Expected output: `go.mod`/`go.sum` updated; no errors. (`pgx/v5/stdlib` gives goose a `database/sql` handle backed by pgx.)

- [ ] **Step 2: Add testcontainers dependency**

```
cd apps/api && go get github.com/testcontainers/testcontainers-go@v0.34.0 && go get github.com/testcontainers/testcontainers-go/modules/postgres@v0.34.0
```
Expected output: `go.mod`/`go.sum` updated; no errors. (Docker must be running for these tests.)

- [ ] **Step 3: Write `0001_init.sql`** (exact columns from `database-schema.md`)

`apps/api/internal/store/migrations/0001_init.sql`:
```sql
-- +goose Up
CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE schools (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    name       text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE classes (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    school_id  uuid NOT NULL REFERENCES schools(id) ON DELETE RESTRICT,
    name       text NOT NULL,
    join_code  text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX classes_join_code_key ON classes (join_code);
CREATE INDEX classes_school_id_idx ON classes (school_id);

CREATE TABLE users (
    id                uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    email             text NOT NULL,
    email_verified_at timestamptz,
    password_hash     text NOT NULL,
    role              text NOT NULL DEFAULT 'student' CHECK (role IN ('student','teacher','admin')),
    school_id         uuid NOT NULL REFERENCES schools(id) ON DELETE RESTRICT,
    display_name      text NOT NULL,
    avatar_color      text NOT NULL,
    created_at        timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX users_email_key ON users (email);
CREATE INDEX users_school_id_idx ON users (school_id);

CREATE TABLE enrollments (
    id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id       uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    class_id      uuid NOT NULL REFERENCES classes(id) ON DELETE RESTRICT,
    role_in_class text NOT NULL DEFAULT 'student' CHECK (role_in_class IN ('student','teacher')),
    created_at    timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX enrollments_user_id_idx ON enrollments (user_id);
CREATE INDEX enrollments_class_id_idx ON enrollments (class_id);
CREATE UNIQUE INDEX enrollments_user_class_key ON enrollments (user_id, class_id);

CREATE TABLE tasks (
    id             uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id        uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    title          text NOT NULL,
    seed           text,
    status         text NOT NULL DEFAULT 'active' CHECK (status IN ('active','evaluated')),
    created_at     timestamptz NOT NULL DEFAULT now(),
    last_active_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX tasks_user_last_active_idx ON tasks (user_id, last_active_at DESC);

CREATE TABLE messages (
    id                uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    task_id           uuid NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
    role              text NOT NULL CHECK (role IN ('user','assistant','system','summary')),
    content           text NOT NULL,
    tool_call         jsonb,
    provider          text,
    model             text,
    tier              text,
    prompt_tokens     integer,
    completion_tokens integer,
    cost_estimate     numeric(12,6),
    created_at        timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX messages_task_created_id_idx ON messages (task_id, created_at, id);

CREATE TABLE card_instances (
    id             uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    card_id        text NOT NULL,
    task_id        uuid NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
    parent_node_id uuid,
    status         text NOT NULL CHECK (status IN ('proposed','active','completed','skipped')),
    field_values   jsonb NOT NULL DEFAULT '{}',
    event_trace    jsonb NOT NULL DEFAULT '[]',
    rubric_tags    text[] NOT NULL DEFAULT '{}',
    created_at     timestamptz NOT NULL DEFAULT now(),
    completed_at   timestamptz
);
CREATE INDEX card_instances_task_created_idx ON card_instances (task_id, created_at);
CREATE INDEX card_instances_parent_idx ON card_instances (parent_node_id);

CREATE TABLE evaluations (
    id                uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    task_id           uuid NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
    scores            jsonb NOT NULL,
    narrative         text NOT NULL,
    model             text NOT NULL,
    tier              text NOT NULL,
    prompt_tokens     integer,
    completion_tokens integer,
    cost_estimate     numeric(12,6),
    status            text NOT NULL CHECK (status IN ('queued','running','done','failed')),
    error             text,
    created_at        timestamptz NOT NULL DEFAULT now(),
    completed_at      timestamptz
);
CREATE INDEX evaluations_task_created_idx ON evaluations (task_id, created_at DESC);

CREATE VIEW llm_usage AS
  SELECT m.id, t.user_id, u.school_id, 'chat'::text AS kind,
         m.provider, m.model, m.tier,
         m.prompt_tokens, m.completion_tokens, m.cost_estimate, m.created_at
    FROM messages m JOIN tasks t ON t.id = m.task_id JOIN users u ON u.id = t.user_id
   WHERE m.role = 'assistant' AND m.model IS NOT NULL
  UNION ALL
  SELECT e.id, t.user_id, u.school_id, 'eval'::text AS kind,
         NULL, e.model, e.tier,
         e.prompt_tokens, e.completion_tokens, e.cost_estimate, e.created_at
    FROM evaluations e JOIN tasks t ON t.id = e.task_id JOIN users u ON u.id = t.user_id;

-- +goose Down
DROP VIEW IF EXISTS llm_usage;
DROP TABLE IF EXISTS evaluations;
DROP TABLE IF EXISTS card_instances;
DROP TABLE IF EXISTS messages;
DROP TABLE IF EXISTS tasks;
DROP TABLE IF EXISTS enrollments;
DROP TABLE IF EXISTS users;
DROP TABLE IF EXISTS classes;
DROP TABLE IF EXISTS schools;
```

- [ ] **Step 4: Write `0002_seed.sql`** (org-bound seeded student)

`apps/api/internal/store/migrations/0002_seed.sql`:
```sql
-- +goose Up
-- Deterministic seed: one school, one class, one org-bound student.
-- Fixed UUIDs keep the seed idempotent-friendly and referenceable by tests/dev.
INSERT INTO schools (id, name)
VALUES ('00000000-0000-0000-0000-000000000001', 'Demo School');

INSERT INTO classes (id, school_id, name, join_code)
VALUES ('00000000-0000-0000-0000-000000000002',
        '00000000-0000-0000-0000-000000000001',
        'Demo Class', 'DEMO-0001');

INSERT INTO users (id, email, email_verified_at, password_hash, role, school_id, display_name, avatar_color)
VALUES ('00000000-0000-0000-0000-000000000003',
        'phoebe@demo.mindimprint.local',
        now(),
        'SEED_NO_LOGIN',
        'student',
        '00000000-0000-0000-0000-000000000001',
        'Phoebe',
        '#7C9CF0');

INSERT INTO enrollments (id, user_id, class_id, role_in_class)
VALUES ('00000000-0000-0000-0000-000000000004',
        '00000000-0000-0000-0000-000000000003',
        '00000000-0000-0000-0000-000000000002',
        'student');

-- +goose Down
DELETE FROM enrollments WHERE id = '00000000-0000-0000-0000-000000000004';
DELETE FROM users       WHERE id = '00000000-0000-0000-0000-000000000003';
DELETE FROM classes     WHERE id = '00000000-0000-0000-0000-000000000002';
DELETE FROM schools     WHERE id = '00000000-0000-0000-0000-000000000001';
```

- [ ] **Step 5: Write the failing migration test**

`apps/api/internal/store/migrate_test.go`:
```go
package store

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

// newTestPool spins up a throwaway Postgres, runs all migrations, and returns a
// connected pool. The container is terminated via t.Cleanup.
func newTestPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	ctx := context.Background()

	pg, err := tcpostgres.Run(ctx,
		"postgres:16-alpine",
		tcpostgres.WithDatabase("mindimprint"),
		tcpostgres.WithUsername("test"),
		tcpostgres.WithPassword("test"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(60*time.Second),
		),
	)
	if err != nil {
		t.Fatalf("start postgres container: %v", err)
	}
	t.Cleanup(func() { _ = pg.Terminate(context.Background()) })

	dsn, err := pg.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatalf("connection string: %v", err)
	}

	pool, err := NewPool(ctx, dsn)
	if err != nil {
		t.Fatalf("new pool: %v", err)
	}
	t.Cleanup(pool.Close)

	if err := RunMigrations(ctx, pool); err != nil {
		t.Fatalf("run migrations: %v", err)
	}
	return pool
}

func TestMigrationsCreateTablesAndSeed(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping testcontainers integration in -short mode")
	}
	ctx := context.Background()
	pool := newTestPool(t)

	wantTables := []string{
		"schools", "classes", "users", "enrollments",
		"tasks", "messages", "card_instances", "evaluations",
	}
	for _, name := range wantTables {
		var exists bool
		err := pool.QueryRow(ctx,
			`SELECT EXISTS (SELECT 1 FROM information_schema.tables
			 WHERE table_schema='public' AND table_name=$1)`, name).Scan(&exists)
		if err != nil {
			t.Fatalf("check table %s: %v", name, err)
		}
		if !exists {
			t.Fatalf("table %s missing after migration", name)
		}
	}

	// llm_usage view exists.
	var viewExists bool
	if err := pool.QueryRow(ctx,
		`SELECT EXISTS (SELECT 1 FROM information_schema.views
		 WHERE table_schema='public' AND table_name='llm_usage')`).Scan(&viewExists); err != nil {
		t.Fatalf("check view: %v", err)
	}
	if !viewExists {
		t.Fatal("llm_usage view missing")
	}

	// Seeded student present and org-bound; school matches the class's school.
	var (
		studentSchool string
		classSchool   string
		role          string
	)
	err := pool.QueryRow(ctx, `
		SELECT u.school_id::text, c.school_id::text, e.role_in_class
		  FROM users u
		  JOIN enrollments e ON e.user_id = u.id
		  JOIN classes c ON c.id = e.class_id
		 WHERE u.email = 'phoebe@demo.mindimprint.local'`).
		Scan(&studentSchool, &classSchool, &role)
	if err != nil {
		t.Fatalf("seeded student/enrollment not found: %v", err)
	}
	if studentSchool != classSchool {
		t.Fatalf("invariant broken: user.school_id %s != class.school_id %s", studentSchool, classSchool)
	}
	if role != "student" {
		t.Fatalf("role_in_class = %q, want student", role)
	}
}
```

- [ ] **Step 6: Run the test — it fails**

```
cd apps/api && go test ./internal/store/...
```
Expected output: compile failure — `undefined: NewPool`, `undefined: RunMigrations`.

- [ ] **Step 7: Implement `db.go`**

`apps/api/internal/store/db.go`:
```go
// Package store is the data-access layer: the pgx connection pool, goose
// migrations, and sqlc-generated type-safe queries.
package store

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

// NewPool opens a pgx connection pool against dsn and verifies it with a Ping.
func NewPool(ctx context.Context, dsn string) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, err
	}
	// Small service: a modest pool ceiling is plenty.
	cfg.MaxConns = 20

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, err
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, err
	}
	return pool, nil
}
```

- [ ] **Step 8: Implement `migrate.go`** (embeds the SQL, runs goose over a pgx-backed `*sql.DB`)

`apps/api/internal/store/migrate.go`:
```go
package store

import (
	"context"
	"embed"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
)

//go:embed migrations/*.sql
var migrationFS embed.FS

// RunMigrations applies all pending goose migrations to the pool's database.
// It opens a short-lived database/sql handle over the same pgx config (goose
// needs database/sql), runs "up", then closes it. The pgxpool is untouched.
func RunMigrations(ctx context.Context, pool *pgxpool.Pool) error {
	db := stdlib.OpenDBFromPool(pool)
	defer db.Close()

	goose.SetBaseFS(migrationFS)
	if err := goose.SetDialect("postgres"); err != nil {
		return err
	}
	return goose.UpContext(ctx, db, "migrations")
}
```

- [ ] **Step 9: Run the migration test — it passes** (Docker must be running)

```
cd apps/api && go test ./internal/store/...
```
Expected output: `ok  	mindimprint/api/internal/store	<time>` (first run is slower while it pulls `postgres:16-alpine`).

- [ ] **Step 10: Re-wire `cmd/api/main.go` to the target version**

Replace `apps/api/cmd/api/main.go` with the target version shown in Task 5 Step 6 (imports `config`, `httpx`, `store`; supports `-migrate-up`; opens the pool; passes the pool as the `httpx.Pinger` and closes it on shutdown).

- [ ] **Step 11: Build + full suite + commit**

```
cd apps/api && go vet ./... && go build ./... && go test ./...
```
Expected output: vet/build clean; tests `ok` for `config`, `httpx`, `store`.

```
git add apps/api/go.mod apps/api/go.sum apps/api/internal/store/ apps/api/cmd/api/main.go
git commit -m "feat(api): pgx pool + goose migrations + org-bound seed (P1.1 task 6)

Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>"
```

---

### Task 7: sqlc store

**Files:**
- Create: `apps/api/sqlc.yaml`
- Create: `apps/api/internal/store/queries/users.sql`
- Create: `apps/api/internal/store/queries/tasks.sql`
- Create: `apps/api/internal/store/queries/messages.sql`
- Generate: `apps/api/internal/store/sqlc/db.go`, `models.go`, `users.sql.go`, `tasks.sql.go`, `messages.sql.go`
- Create: `apps/api/internal/store/sqlc_test.go`

**Interfaces:**
- Consumes: the schema from migration `0001_init.sql`; the seeded user id `00000000-0000-0000-0000-000000000003`.
- Produces (sqlc-generated, package `sqlc`):
  - `func New(db DBTX) *Queries`
  - `func (q *Queries) GetUserByID(ctx context.Context, id uuid.UUID) (User, error)`
  - `func (q *Queries) CreateTask(ctx context.Context, arg CreateTaskParams) (Task, error)`
  - `func (q *Queries) GetTask(ctx context.Context, id uuid.UUID) (Task, error)`
  - `func (q *Queries) ListTasksByUser(ctx context.Context, userID uuid.UUID) ([]Task, error)`
  - `func (q *Queries) AppendMessage(ctx context.Context, arg AppendMessageParams) (Message, error)`
  - `func (q *Queries) ListMessagesByTask(ctx context.Context, taskID uuid.UUID) ([]Message, error)`

- [ ] **Step 1: Pin sqlc as a Go tool**

```
cd apps/api && go get -tool github.com/sqlc-dev/sqlc/cmd/sqlc@v1.27.0
```
Expected output: `go.mod` gains a `tool` directive for sqlc; `go tool sqlc version` prints `v1.27.0`.

- [ ] **Step 2: Write `sqlc.yaml`**

`apps/api/sqlc.yaml`:
```yaml
version: "2"
sql:
  - engine: "postgresql"
    schema: "internal/store/migrations"
    queries: "internal/store/queries"
    gen:
      go:
        package: "sqlc"
        out: "internal/store/sqlc"
        sql_package: "pgx/v5"
        emit_json_tags: true
        emit_pointers_for_null_types: true
        overrides:
          - db_type: "uuid"
            go_type: "github.com/google/uuid.UUID"
          - db_type: "timestamptz"
            go_type: "time.Time"
```

> sqlc parses the goose migration files directly as the schema source (it understands the `-- +goose Up`/`Down` markers well enough to read the DDL in the Up block). The seed file `0002_seed.sql` contains only `INSERT`/`DELETE`, which sqlc ignores for type generation.

- [ ] **Step 3: Write `queries/users.sql`**

`apps/api/internal/store/queries/users.sql`:
```sql
-- name: GetUserByID :one
SELECT * FROM users WHERE id = $1;
```

- [ ] **Step 4: Write `queries/tasks.sql`**

`apps/api/internal/store/queries/tasks.sql`:
```sql
-- name: CreateTask :one
INSERT INTO tasks (user_id, title, seed)
VALUES ($1, $2, $3)
RETURNING *;

-- name: GetTask :one
SELECT * FROM tasks WHERE id = $1;

-- name: ListTasksByUser :many
SELECT * FROM tasks
WHERE user_id = $1
ORDER BY last_active_at DESC;
```

- [ ] **Step 5: Write `queries/messages.sql`**

`apps/api/internal/store/queries/messages.sql`:
```sql
-- name: AppendMessage :one
INSERT INTO messages (
    task_id, role, content, tool_call,
    provider, model, tier, prompt_tokens, completion_tokens, cost_estimate
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10
)
RETURNING *;

-- name: ListMessagesByTask :many
SELECT * FROM messages
WHERE task_id = $1
ORDER BY created_at, id;
```

- [ ] **Step 6: Generate the store**

```
cd apps/api && go tool sqlc generate
```
Expected output: no errors; new files appear under `internal/store/sqlc/` (`db.go`, `models.go`, `users.sql.go`, `tasks.sql.go`, `messages.sql.go`).

- [ ] **Step 7: Write the failing integration test**

`apps/api/internal/store/sqlc_test.go`:
```go
package store_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"mindimprint/api/internal/store"
	"mindimprint/api/internal/store/sqlc"
)

// seededStudentID is the fixed UUID from migration 0002_seed.sql.
var seededStudentID = uuid.MustParse("00000000-0000-0000-0000-000000000003")

func TestStoreRoundTrip(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping testcontainers integration in -short mode")
	}
	ctx := context.Background()
	pool := newStoreTestPool(t)
	q := sqlc.New(pool)

	// The seeded user resolves.
	u, err := q.GetUserByID(ctx, seededStudentID)
	if err != nil {
		t.Fatalf("GetUserByID: %v", err)
	}
	if u.DisplayName != "Phoebe" {
		t.Fatalf("display_name = %q, want Phoebe", u.DisplayName)
	}

	// Create a task for the seeded user.
	task, err := q.CreateTask(ctx, sqlc.CreateTaskParams{
		UserID: seededStudentID,
		Title:  "中国是否让地球变得更可持续？",
		Seed:   ptr("https://example.com/article"),
	})
	if err != nil {
		t.Fatalf("CreateTask: %v", err)
	}

	// GetTask round-trips.
	got, err := q.GetTask(ctx, task.ID)
	if err != nil {
		t.Fatalf("GetTask: %v", err)
	}
	if got.Title != task.Title {
		t.Fatalf("GetTask title = %q, want %q", got.Title, task.Title)
	}

	// ListTasksByUser includes it.
	tasks, err := q.ListTasksByUser(ctx, seededStudentID)
	if err != nil {
		t.Fatalf("ListTasksByUser: %v", err)
	}
	if len(tasks) != 1 || tasks[0].ID != task.ID {
		t.Fatalf("ListTasksByUser = %d rows, want 1 matching", len(tasks))
	}

	// Append a user message then an assistant message carrying usage.
	if _, err := q.AppendMessage(ctx, sqlc.AppendMessageParams{
		TaskID:  task.ID,
		Role:    "user",
		Content: "这篇文章可信吗？",
	}); err != nil {
		t.Fatalf("AppendMessage(user): %v", err)
	}

	cost := pgtype.Numeric{}
	if err := cost.Scan("0.001200"); err != nil {
		t.Fatalf("scan numeric: %v", err)
	}
	if _, err := q.AppendMessage(ctx, sqlc.AppendMessageParams{
		TaskID:           task.ID,
		Role:             "assistant",
		Content:          "我们一起核查一下来源。",
		Provider:         ptr("deepseek"),
		Model:            ptr("deepseek-chat"),
		Tier:             ptr("chaperone"),
		PromptTokens:     ptrInt32(120),
		CompletionTokens: ptrInt32(45),
		CostEstimate:     cost,
	}); err != nil {
		t.Fatalf("AppendMessage(assistant): %v", err)
	}

	msgs, err := q.ListMessagesByTask(ctx, task.ID)
	if err != nil {
		t.Fatalf("ListMessagesByTask: %v", err)
	}
	if len(msgs) != 2 {
		t.Fatalf("got %d messages, want 2", len(msgs))
	}
	if msgs[0].Role != "user" || msgs[1].Role != "assistant" {
		t.Fatalf("message order wrong: %q, %q", msgs[0].Role, msgs[1].Role)
	}
	if msgs[1].Model == nil || *msgs[1].Model != "deepseek-chat" {
		t.Fatalf("assistant row missing model")
	}
}

func ptr(s string) *string { return &s }
func ptrInt32(n int32) *int32 { return &n }

// newStoreTestPool mirrors newTestPool but lives in the external test package; it
// reuses the exported NewPool/RunMigrations.
func newStoreTestPool(t *testing.T) *store.PoolForTest { // see note below
	t.Helper()
	return store.SpinUpTestPostgres(t)
}
```

> **Type-consistency note for the worker:** The migration test in Task 6 (`migrate_test.go`, package `store`) already has an internal `newTestPool` helper. The sqlc test here is in the *external* test package `store_test`, so it cannot call the unexported `newTestPool`. Resolve this by adding ONE exported test helper to package `store` (in a `testhelp_test.go` file shared by both, OR an exported function guarded for tests). The cleanest approach the worker MUST take: replace the last function above with a direct testcontainers spin-up duplicated in `store_test` (copy the body of `newTestPool` from Task 6, changing only the return type to `*pgxpool.Pool` and the imports). Concretely, delete the `newStoreTestPool` wrapper and the `store.PoolForTest`/`store.SpinUpTestPostgres` references, and inline a `newStoreTestPool(t *testing.T) *pgxpool.Pool` whose body is identical to Task 6 Step 5's `newTestPool` (testcontainers Run → ConnectionString → `store.NewPool` → `store.RunMigrations`). Import `github.com/jackc/pgx/v5/pgxpool`, the testcontainers modules, and `mindimprint/api/internal/store`. `sqlc.New` accepts a `*pgxpool.Pool` because it satisfies the generated `DBTX` interface.

The final `newStoreTestPool` the worker writes:
```go
func newStoreTestPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	ctx := context.Background()
	pg, err := tcpostgres.Run(ctx, "postgres:16-alpine",
		tcpostgres.WithDatabase("mindimprint"),
		tcpostgres.WithUsername("test"),
		tcpostgres.WithPassword("test"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).WithStartupTimeout(60*time.Second)),
	)
	if err != nil {
		t.Fatalf("start postgres: %v", err)
	}
	t.Cleanup(func() { _ = pg.Terminate(context.Background()) })
	dsn, err := pg.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatalf("dsn: %v", err)
	}
	pool, err := store.NewPool(ctx, dsn)
	if err != nil {
		t.Fatalf("pool: %v", err)
	}
	t.Cleanup(pool.Close)
	if err := store.RunMigrations(ctx, pool); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return pool
}
```
with imports added: `"time"`, `"github.com/jackc/pgx/v5/pgxpool"`, `"github.com/testcontainers/testcontainers-go"`, `tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"`, `"github.com/testcontainers/testcontainers-go/wait"`.

> **Generated-field-name note:** sqlc names the assistant-usage param fields `Provider`, `Model`, `Tier`, `PromptTokens`, `CompletionTokens`, `CostEstimate` and (with `emit_pointers_for_null_types: true`) types the nullable text/int columns as `*string`/`*int32`. `cost_estimate numeric(12,6)` is generated as `pgtype.Numeric` (not a pointer), which is why the test builds it via `pgtype.Numeric{}.Scan`. If the generated type differs (e.g. `pgtype.Text` instead of `*string`), the worker adjusts the test call sites to the actual generated types — the assertions stay the same.

- [ ] **Step 8: Run the test — it fails first because sqlc types must exist, then passes after generation**

```
cd apps/api && go test ./internal/store/...
```
Expected output (after Step 6 generation): `ok  	mindimprint/api/internal/store	<time>` for both `store` and `store_test` test binaries. If field-type mismatches surface, fix the test call sites per the note above and re-run until green.

- [ ] **Step 9: Commit** (including the generated `sqlc/` mirror)

```
cd apps/api && go mod tidy
git add apps/api/go.mod apps/api/go.sum apps/api/sqlc.yaml apps/api/internal/store/queries/ apps/api/internal/store/sqlc/ apps/api/internal/store/sqlc_test.go
git commit -m "feat(api): sqlc type-safe store for tasks/messages/users (P1.1 task 7)

Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>"
```

---

### Task 8: Card specs (`internal/cards`)

**Files:**
- Create: `apps/api/tools/synccards/main.go`
- Create: `apps/api/internal/cards/specs/*.json` (synced mirror of the 33 canonical specs)
- Create: `apps/api/internal/cards/loader.go`
- Create: `apps/api/internal/cards/loader_test.go`

**Interfaces:**
- Consumes: canonical `packages/contracts/cards/*.json` (repo root, four levels up from the package).
- Produces:
  - `type Spec struct { ID string; Category string; Name string; NameEN string; Purpose string; TriggerCondition string }`
  - `func Catalog() ([]Spec, error)`
  - `func ByID(id string) (Spec, bool)`

- [ ] **Step 1: Write the sync tool**

`apps/api/tools/synccards/main.go`:
```go
// Command synccards mirrors the canonical card JSON specs from
// packages/contracts/cards into internal/cards/specs so they can be embedded.
// It is the ONLY writer of the mirror; never hand-edit internal/cards/specs.
//
// Run from the apps/api module root: `go run ./tools/synccards` (or `make sync-cards`).
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// canonicalRel is the canonical card dir relative to apps/api/.
const canonicalRel = "../../packages/contracts/cards"

// mirrorRel is the embeddable mirror relative to apps/api/.
const mirrorRel = "internal/cards/specs"

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "synccards:", err)
		os.Exit(1)
	}
}

func run() error {
	entries, err := os.ReadDir(canonicalRel)
	if err != nil {
		return fmt.Errorf("read canonical dir %s: %w", canonicalRel, err)
	}

	if err := os.MkdirAll(mirrorRel, 0o755); err != nil {
		return fmt.Errorf("mkdir mirror: %w", err)
	}

	// Remove stale mirror files so deletions in canonical propagate.
	mirrorEntries, err := os.ReadDir(mirrorRel)
	if err != nil {
		return fmt.Errorf("read mirror dir: %w", err)
	}
	for _, e := range mirrorEntries {
		if strings.HasSuffix(e.Name(), ".json") {
			if err := os.Remove(filepath.Join(mirrorRel, e.Name())); err != nil {
				return fmt.Errorf("remove stale %s: %w", e.Name(), err)
			}
		}
	}

	count := 0
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		src := filepath.Join(canonicalRel, e.Name())
		dst := filepath.Join(mirrorRel, e.Name())
		data, err := os.ReadFile(src)
		if err != nil {
			return fmt.Errorf("read %s: %w", src, err)
		}
		if err := os.WriteFile(dst, data, 0o644); err != nil {
			return fmt.Errorf("write %s: %w", dst, err)
		}
		count++
	}
	fmt.Printf("synccards: mirrored %d card specs\n", count)
	return nil
}
```

- [ ] **Step 2: Run the sync to populate the mirror**

```
cd apps/api && go run ./tools/synccards
```
Expected output: `synccards: mirrored 33 card specs`. Verify: `ls internal/cards/specs | wc -l` prints `33`.

- [ ] **Step 3: Write the failing loader test**

`apps/api/internal/cards/loader_test.go`:
```go
package cards

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCatalog(t *testing.T) {
	specs, err := Catalog()
	if err != nil {
		t.Fatalf("Catalog: %v", err)
	}
	if len(specs) != 33 {
		t.Fatalf("catalog has %d specs, want 33", len(specs))
	}

	seen := map[string]bool{}
	for _, s := range specs {
		if s.ID == "" {
			t.Fatal("spec with empty id")
		}
		if seen[s.ID] {
			t.Fatalf("duplicate id %q", s.ID)
		}
		seen[s.ID] = true
	}
}

func TestByID(t *testing.T) {
	got, ok := ByID("ai-boundary")
	if !ok {
		t.Fatal("ai-boundary not found")
	}
	if got.NameEN != "AI Boundary & Hallucination Check" {
		t.Fatalf("name_en = %q", got.NameEN)
	}
	if got.Category != "AI伦理" {
		t.Fatalf("category = %q", got.Category)
	}

	if _, ok := ByID("does-not-exist"); ok {
		t.Fatal("unknown id should not resolve")
	}
}

// TestMirrorMatchesCanonical fails if someone edited a canonical card without
// re-running `make sync-cards`. It compares the embedded mirror byte-for-byte
// against packages/contracts/cards.
func TestMirrorMatchesCanonical(t *testing.T) {
	const canonicalRel = "../../../../packages/contracts/cards"
	canonicalEntries, err := os.ReadDir(canonicalRel)
	if err != nil {
		t.Fatalf("read canonical dir: %v", err)
	}

	canonicalJSON := map[string][]byte{}
	for _, e := range canonicalEntries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		b, err := os.ReadFile(filepath.Join(canonicalRel, e.Name()))
		if err != nil {
			t.Fatalf("read %s: %v", e.Name(), err)
		}
		canonicalJSON[e.Name()] = b
	}

	mirrorEntries, err := specFS.ReadDir("specs")
	if err != nil {
		t.Fatalf("read embedded specs: %v", err)
	}
	mirrorJSON := map[string][]byte{}
	for _, e := range mirrorEntries {
		b, err := specFS.ReadFile("specs/" + e.Name())
		if err != nil {
			t.Fatalf("read embedded %s: %v", e.Name(), err)
		}
		mirrorJSON[e.Name()] = b
	}

	if len(canonicalJSON) != len(mirrorJSON) {
		t.Fatalf("file count drift: canonical=%d mirror=%d (run `make sync-cards`)",
			len(canonicalJSON), len(mirrorJSON))
	}
	for name, want := range canonicalJSON {
		got, ok := mirrorJSON[name]
		if !ok {
			t.Fatalf("mirror missing %s (run `make sync-cards`)", name)
		}
		if !bytes.Equal(got, want) {
			t.Fatalf("mirror drift in %s (run `make sync-cards`)", name)
		}
	}
}
```

- [ ] **Step 4: Run the test — it fails**

```
cd apps/api && go test ./internal/cards/...
```
Expected output: compile failure — `undefined: Catalog`, `undefined: ByID`, `undefined: specFS`.

- [ ] **Step 5: Implement `loader.go`**

`apps/api/internal/cards/loader.go`:
```go
// Package cards loads the embedded card-spec catalog. The specs are a generated
// mirror of the canonical packages/contracts/cards; never hand-edit specs/.
package cards

import (
	"embed"
	"encoding/json"
	"fmt"
	"sort"
)

//go:embed specs/*.json
var specFS embed.FS

// Spec is the minimal view of a card the backend needs (catalog/prompt/refeed).
// The deep card shape stays owned by the TS Zod contract.
type Spec struct {
	ID               string `json:"id"`
	Category         string `json:"category"`
	Name             string `json:"name"`
	NameEN           string `json:"name_en"`
	Purpose          string `json:"purpose"`
	TriggerCondition string `json:"trigger_condition"`
}

// Catalog reads and parses every embedded spec, sorted by id for determinism.
func Catalog() ([]Spec, error) {
	entries, err := specFS.ReadDir("specs")
	if err != nil {
		return nil, fmt.Errorf("read embedded specs: %w", err)
	}
	specs := make([]Spec, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		data, err := specFS.ReadFile("specs/" + e.Name())
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", e.Name(), err)
		}
		var s Spec
		if err := json.Unmarshal(data, &s); err != nil {
			return nil, fmt.Errorf("parse %s: %w", e.Name(), err)
		}
		specs = append(specs, s)
	}
	sort.Slice(specs, func(i, j int) bool { return specs[i].ID < specs[j].ID })
	return specs, nil
}

// ByID returns the spec with the given id, if present.
func ByID(id string) (Spec, bool) {
	specs, err := Catalog()
	if err != nil {
		return Spec{}, false
	}
	for _, s := range specs {
		if s.ID == id {
			return s, true
		}
	}
	return Spec{}, false
}
```

- [ ] **Step 6: Run the test — it passes**

```
cd apps/api && go test ./internal/cards/...
```
Expected output: `ok  	mindimprint/api/internal/cards	<time>`.

- [ ] **Step 7: Commit** (mirror + tool + loader)

```
git add apps/api/tools/synccards/ apps/api/internal/cards/
git commit -m "feat(api): embedded card catalog with sync tool + drift check (P1.1 task 8)

Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>"
```

---

### Task 9: Dockerfile + gate wiring

**Files:**
- Create: `apps/api/Dockerfile`
- Modify: `apps/api/Makefile` (no change needed if Task 1 targets are intact — verify only)
- Modify (docs): note the full gate command in this plan's closing checklist.

**Interfaces:**
- Consumes: the whole `apps/api` module.
- Produces: a runnable container image; a documented one-line gate.

- [ ] **Step 1: Write the Dockerfile**

`apps/api/Dockerfile`:
```dockerfile
# syntax=docker/dockerfile:1

FROM golang:1.26 AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /api ./cmd/api

FROM gcr.io/distroless/static:nonroot
COPY --from=build /api /api
EXPOSE 8080
USER nonroot:nonroot
ENTRYPOINT ["/api"]
```

- [ ] **Step 2: Verify the image builds** (Docker required)

```
cd apps/api && docker build -t mindimprint/api:p1.1 .
```
Expected output: a successful multi-stage build ending in `naming to docker.io/mindimprint/api:p1.1` (or BuildKit's equivalent success line).

- [ ] **Step 3: Run the full Go gate**

```
cd apps/api && go vet ./... && go build ./... && go test ./...
```
Expected output: vet clean; build clean; `ok` for `internal/config`, `internal/httpx`, `internal/store`, `internal/cards`; `cmd/api` and `tools/synccards` report `no test files`.

- [ ] **Step 4: Run the repo-wide gate**

```
pnpm -r typecheck && pnpm -r test && (cd apps/api && go vet ./... && go test ./...)
```
Expected output: the existing JS/TS workspaces pass typecheck + Vitest, then the Go suite is green. (This is the binding gate from spec §10.)

> **Do not** stage the repo-root `package.json` (it carries an unrelated uncommitted change). `git add` only the files listed for this task.

- [ ] **Step 5: Commit**

```
git add apps/api/Dockerfile
git commit -m "feat(api): multi-stage distroless Dockerfile + gate wiring (P1.1 task 9)

Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>"
```

---

## Done-when (P1.1 acceptance)

- `cd apps/api && go vet ./... && go build ./... && go test ./...` is green (testcontainers integration included; Docker running).
- `make migrate-up` applies `0001_init.sql` + `0002_seed.sql` against `DATABASE_URL`.
- The seeded org-bound student (`phoebe@demo.mindimprint.local`) exists with `school_id == class.school_id`.
- The card catalog loads exactly 33 specs; `ByID("ai-boundary")` resolves; the drift test guards the mirror.
- `make sync-cards`, `make sqlc`, `make test`, `make run` are real, working targets.
- The full repo gate `pnpm -r typecheck && pnpm -r test && (cd apps/api && go vet ./... && go test ./...)` passes.
- No secrets committed; the root `package.json` change is untouched.
