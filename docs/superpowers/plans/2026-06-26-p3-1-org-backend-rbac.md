# P3.1 · Org Backend + RBAC Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a Go backend organization layer on P2 auth — role-based authorization, admin-seeded bootstrap, teacher invite-code signup, teacher/admin-created classes, admin bulk-import, and aggregate-only roster/school views.

**Architecture:** New `0005_org.sql` (teacher_invites table + `classes.created_by` + `llm_usage` view) and `0006_seed_admin.sql` (seeded admin). A new `internal/org` code generator. Authorization split into a coarse `RequireRole` middleware (403) plus per-resource tenancy guards (404). New `internal/api` handlers for classes/roster and admin invites/import/overview, each wiring its own route into the existing `Handler()` mux. Every new route is additive — no breaking cutover. sqlc queries are added per feature group and regenerated with `make sqlc`.

**Tech Stack:** Go 1.26 (`net/http` 1.22 routing), pgx/v5 + sqlc, goose migrations, `golang.org/x/crypto/argon2` (existing `internal/auth`), testcontainers-go (postgres:16-alpine), `log/slog`.

## Global Constraints

- **Tests run with `go test -p 1 ./...`** — parallel testcontainers exhaust Docker. testcontainers needs `DOCKER_HOST=unix:///var/run/docker.sock` set at run time only, **never committed**.
- **sqlc regeneration is `make sqlc`** (runs `CGO_ENABLED=0 go tool sqlc generate`) from `apps/api/`. Run it in any task that edits `internal/store/queries/*.sql`.
- **Never `git add` the repo-root `package.json`** (it carries a pre-existing unrelated modification) — always stage explicit paths.
- **Commit trailer** on every commit: `Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>`.
- **Authorization mapping is binding:** coarse role failure → `403 forbidden` (`httpx.ErrForbidden`); cross-tenant / not-owned resource → `404 not_found` (`httpx.ErrNotFound`, existence hiding). Unauthenticated → `401` (existing `RequireUser`).
- **Codes:** `classes.join_code` and `teacher_invites.code` are plaintext, unique, distributable. Teacher invite codes are `T-` prefixed. Invites are single-use (`consumed_at IS NULL AND expires_at > now()` filtered in SQL). Emails are trimmed + lowercased at the app edge.
- **Secrets/internals never** enter git, logs, client error messages, or responses. Real error detail goes to `slog` only (existing `httpx.WriteError` does this).
- **Aggregate-only visibility:** roster/overview expose counts + timestamps + cost, **never** evaluation contents, transcripts, or process trees.
- **Migrations** are goose SQL files in `apps/api/internal/store/migrations/` (embedded via `//go:embed migrations/*.sql`); sqlc also reads them for type generation. Keep `-- +goose Up` / `-- +goose Down` and make Down reversible.
- **Demo fixtures (from `0002_seed.sql`):** school `…0001` "Demo School", class `…0002` "Demo Class" (`join_code` `DEMO-0001`), student Phoebe `…0003`.

---

### Task 1: Migration `0005_org.sql` — teacher_invites, classes.created_by, llm_usage view

**Files:**
- Create: `apps/api/internal/store/migrations/0005_org.sql`
- Test: `apps/api/internal/store/org_migrate_test.go`

**Interfaces:**
- Produces: table `teacher_invites(id, school_id, code, email, created_by, expires_at, consumed_at, consumed_by, created_at)`; column `classes.created_by uuid null`; view `llm_usage`. No Go symbols yet (sqlc models come in later tasks that add queries).

- [ ] **Step 1: Write the failing test**

`apps/api/internal/store/org_migrate_test.go`:
```go
package store_test

import (
	"context"
	"testing"

	"mindimprint/api/internal/store"
)

// TestOrgMigrationArtifacts verifies 0005 created the table, column, and view.
func TestOrgMigrationArtifacts(t *testing.T) {
	pool := newStoreTestPool(t) // existing helper in sqlc_test.go
	ctx := context.Background()

	var n int
	if err := pool.QueryRow(ctx,
		`SELECT count(*) FROM information_schema.tables WHERE table_name='teacher_invites'`,
	).Scan(&n); err != nil || n != 1 {
		t.Fatalf("teacher_invites table missing: n=%d err=%v", n, err)
	}
	if err := pool.QueryRow(ctx,
		`SELECT count(*) FROM information_schema.columns WHERE table_name='classes' AND column_name='created_by'`,
	).Scan(&n); err != nil || n != 1 {
		t.Fatalf("classes.created_by missing: n=%d err=%v", n, err)
	}
	if err := pool.QueryRow(ctx,
		`SELECT count(*) FROM information_schema.views WHERE table_name='llm_usage'`,
	).Scan(&n); err != nil || n != 1 {
		t.Fatalf("llm_usage view missing: n=%d err=%v", n, err)
	}
	// Unique constraint on code: a duplicate insert must fail.
	_, err := pool.Exec(ctx, `INSERT INTO teacher_invites (school_id, code, created_by, expires_at)
		VALUES ('00000000-0000-0000-0000-000000000001','T-DUPE','00000000-0000-0000-0000-000000000003', now()+interval '1 day')`)
	if err != nil {
		t.Fatalf("first invite insert: %v", err)
	}
	_, err = pool.Exec(ctx, `INSERT INTO teacher_invites (school_id, code, created_by, expires_at)
		VALUES ('00000000-0000-0000-0000-000000000001','T-DUPE','00000000-0000-0000-0000-000000000003', now()+interval '1 day')`)
	if err == nil {
		t.Fatal("expected unique violation on duplicate code, got nil")
	}
}
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `cd apps/api && DOCKER_HOST=unix:///var/run/docker.sock go test -p 1 -run TestOrgMigrationArtifacts ./internal/store/`
Expected: FAIL (relation `teacher_invites` does not exist).

- [ ] **Step 3: Write the migration**

`apps/api/internal/store/migrations/0005_org.sql`:
```sql
-- +goose Up
-- Teacher invite codes: an admin mints a single-use, school-scoped code; a teacher
-- self-signs-up with it. Plaintext + distributable (like classes.join_code).
CREATE TABLE teacher_invites (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    school_id   uuid NOT NULL REFERENCES schools(id) ON DELETE RESTRICT,
    code        text NOT NULL UNIQUE,
    email       text,
    created_by  uuid NOT NULL REFERENCES users(id),
    expires_at  timestamptz NOT NULL,
    consumed_at timestamptz,
    consumed_by uuid REFERENCES users(id),
    created_at  timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX idx_teacher_invites_school ON teacher_invites (school_id);

-- Who created the class (teacher or admin). Nullable: the seeded Demo Class has none.
ALTER TABLE classes ADD COLUMN created_by uuid REFERENCES users(id);

-- Read-only cost rollup unioning assistant messages and evaluations, keyed to school.
CREATE VIEW llm_usage AS
  SELECT m.id, t.user_id, u.school_id, 'chat'::text AS kind,
         m.provider, m.model, m.tier,
         m.prompt_tokens, m.completion_tokens, m.cost_estimate, m.created_at
    FROM messages m
    JOIN tasks t ON t.id = m.task_id
    JOIN users u ON u.id = t.user_id
   WHERE m.role = 'assistant' AND m.model IS NOT NULL
  UNION ALL
  SELECT e.id, t.user_id, u.school_id, 'eval'::text AS kind,
         NULL, e.model, e.tier,
         e.prompt_tokens, e.completion_tokens, e.cost_estimate, e.created_at
    FROM evaluations e
    JOIN tasks t ON t.id = e.task_id
    JOIN users u ON u.id = t.user_id;

-- +goose Down
DROP VIEW IF EXISTS llm_usage;
ALTER TABLE classes DROP COLUMN IF EXISTS created_by;
DROP TABLE IF EXISTS teacher_invites;
```

- [ ] **Step 4: Run the test to verify it passes**

Run: `cd apps/api && DOCKER_HOST=unix:///var/run/docker.sock go test -p 1 -run TestOrgMigrationArtifacts ./internal/store/`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add apps/api/internal/store/migrations/0005_org.sql apps/api/internal/store/org_migrate_test.go
git commit -m "$(printf 'feat(p3.1): migration 0005 — teacher_invites, classes.created_by, llm_usage view\n\nCo-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>')"
```

---

### Task 2: `internal/org` code generator

**Files:**
- Create: `apps/api/internal/org/codes.go`
- Test: `apps/api/internal/org/codes_test.go`

**Interfaces:**
- Produces: `func NewClassJoinCode() (string, error)` → e.g. `"K7Q2-9F3M"`; `func NewTeacherInviteCode() (string, error)` → e.g. `"T-K7Q29F3M"`. Both use `crypto/rand` over an unambiguous alphabet (no `0/O/1/I`).

- [ ] **Step 1: Write the failing test**

`apps/api/internal/org/codes_test.go`:
```go
package org_test

import (
	"strings"
	"testing"

	"mindimprint/api/internal/org"
)

const ambiguous = "01OI"

func TestNewClassJoinCode(t *testing.T) {
	seen := map[string]bool{}
	for i := 0; i < 200; i++ {
		c, err := org.NewClassJoinCode()
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		if len(c) != 9 || c[4] != '-' { // XXXX-XXXX
			t.Fatalf("bad format: %q", c)
		}
		if strings.ContainsAny(c, ambiguous) {
			t.Fatalf("ambiguous char in %q", c)
		}
		if seen[c] {
			t.Fatalf("collision: %q", c)
		}
		seen[c] = true
	}
}

func TestNewTeacherInviteCode(t *testing.T) {
	c, err := org.NewTeacherInviteCode()
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if !strings.HasPrefix(c, "T-") {
		t.Fatalf("missing T- prefix: %q", c)
	}
	if strings.ContainsAny(strings.TrimPrefix(c, "T-"), ambiguous) {
		t.Fatalf("ambiguous char in %q", c)
	}
}
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `cd apps/api && go test ./internal/org/`
Expected: FAIL (package `org` does not exist).

- [ ] **Step 3: Write the implementation**

`apps/api/internal/org/codes.go`:
```go
// Package org holds organization-layer helpers shared by the API handlers:
// distributable code generation for class join codes and teacher invites.
package org

import (
	"crypto/rand"
	"strings"
)

// alphabet excludes visually ambiguous characters (0/O, 1/I/L) so codes are safe
// to print, read aloud, and re-type.
const alphabet = "ABCDEFGHJKMNPQRSTUVWXYZ23456789"

// randString returns n characters drawn uniformly from alphabet using crypto/rand.
func randString(n int) (string, error) {
	buf := make([]byte, n)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	var b strings.Builder
	b.Grow(n)
	for _, v := range buf {
		b.WriteByte(alphabet[int(v)%len(alphabet)])
	}
	return b.String(), nil
}

// NewClassJoinCode returns a student-facing class code formatted XXXX-XXXX.
func NewClassJoinCode() (string, error) {
	s, err := randString(8)
	if err != nil {
		return "", err
	}
	return s[:4] + "-" + s[4:], nil
}

// NewTeacherInviteCode returns a teacher invite code prefixed "T-" to keep it
// visually distinct from class join codes.
func NewTeacherInviteCode() (string, error) {
	s, err := randString(8)
	if err != nil {
		return "", err
	}
	return "T-" + s, nil
}
```

- [ ] **Step 4: Run the test to verify it passes**

Run: `cd apps/api && go test ./internal/org/`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add apps/api/internal/org/
git commit -m "$(printf 'feat(p3.1): org code generator (class join + teacher invite codes)\n\nCo-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>')"
```

---

### Task 3: Migration `0006_seed_admin.sql` — seeded admin account

**Files:**
- Create: `apps/api/internal/store/migrations/0006_seed_admin.sql`
- Test: `apps/api/internal/store/seed_admin_test.go`

**Interfaces:**
- Produces: an admin user, fixed id `00000000-0000-0000-0000-000000000005`, email `admin@demo.mindimprint.local`, `role='admin'`, `school_id='…0001'`, **no enrollment**. Dev password `admin-dev-pass`.

**Notes for implementer:** generate the hash with `cd apps/api && go run ./tools/genhash admin-dev-pass` and paste its output verbatim into the migration. `seed_admin_test.go` re-verifies it (catches a bad paste). This mirrors `0004_seed_password.sql`.

- [ ] **Step 1: Write the failing test**

`apps/api/internal/store/seed_admin_test.go`:
```go
package store_test

import (
	"context"
	"testing"

	"mindimprint/api/internal/auth"
	"mindimprint/api/internal/store/sqlc"
)

func TestSeedAdminCanLogIn(t *testing.T) {
	pool := newStoreTestPool(t)
	q := sqlc.New(pool)
	u, err := q.GetUserByEmail(context.Background(), "admin@demo.mindimprint.local")
	if err != nil {
		t.Fatalf("get admin: %v", err)
	}
	if u.Role != "admin" {
		t.Fatalf("role = %q, want admin", u.Role)
	}
	if err := auth.VerifyPassword(u.PasswordHash, "admin-dev-pass"); err != nil {
		t.Fatalf("seeded admin password must verify: %v", err)
	}
	// Admin maps to no class.
	var enrollments int
	if err := pool.QueryRow(context.Background(),
		`SELECT count(*) FROM enrollments WHERE user_id=$1`, u.ID).Scan(&enrollments); err != nil {
		t.Fatalf("count enrollments: %v", err)
	}
	if enrollments != 0 {
		t.Fatalf("admin enrollments = %d, want 0", enrollments)
	}
}
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `cd apps/api && DOCKER_HOST=unix:///var/run/docker.sock go test -p 1 -run TestSeedAdminCanLogIn ./internal/store/`
Expected: FAIL (no such user).

- [ ] **Step 3: Generate the hash, then write the migration**

Run `cd apps/api && go run ./tools/genhash admin-dev-pass` and copy the printed PHC string into `password_hash` below.

`apps/api/internal/store/migrations/0006_seed_admin.sql`:
```sql
-- +goose Up
-- Seeded admin for the demo school. Dev login: admin@demo.mindimprint.local,
-- password "admin-dev-pass". Hash generated by `go run ./tools/genhash admin-dev-pass`;
-- regenerate with that command if argon2id params change. seed_admin_test.go asserts it.
-- Admins belong to a school but no class (no enrollments row).
INSERT INTO users (id, email, email_verified_at, password_hash, role, school_id, display_name, avatar_color)
VALUES ('00000000-0000-0000-0000-000000000005',
        'admin@demo.mindimprint.local',
        now(),
        '<PASTE_GENHASH_OUTPUT_HERE>',
        'admin',
        '00000000-0000-0000-0000-000000000001',
        'Demo Admin',
        '#E0A458');

-- +goose Down
DELETE FROM users WHERE id = '00000000-0000-0000-0000-000000000005';
```

- [ ] **Step 4: Run the test to verify it passes**

Run: `cd apps/api && DOCKER_HOST=unix:///var/run/docker.sock go test -p 1 -run TestSeedAdminCanLogIn ./internal/store/`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add apps/api/internal/store/migrations/0006_seed_admin.sql apps/api/internal/store/seed_admin_test.go
git commit -m "$(printf 'feat(p3.1): seed admin account for the demo school\n\nCo-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>')"
```

---

### Task 4: Authorization layer — `RequireRole` + tenancy guards + test helpers

**Files:**
- Create: `apps/api/internal/api/authz.go`
- Create: `apps/api/internal/store/queries/org.sql` (authz queries; more added in later tasks)
- Modify: `apps/api/internal/store/sqlc/*` (regenerated via `make sqlc`)
- Modify: `apps/api/internal/api/maintest_test.go` (add helpers)
- Test: `apps/api/internal/api/authz_test.go`

**Interfaces:**
- Produces:
  - `func RequireRole(roles ...string) func(http.Handler) http.Handler` — 403 if the context user's role is not in `roles` (401 path already handled upstream by `RequireUser`; `RequireRole` is always composed *inside* `RequireUser`).
  - `func (a *API) assertTeacherOwnsClass(ctx context.Context, classID uuid.UUID) (sqlc.Class, error)` — returns the class if the caller teaches it OR is an admin of its school; otherwise `httpx.ErrNotFound`.
  - `func (a *API) assertAdminOfSchool(ctx context.Context, schoolID uuid.UUID) error` — nil if caller is an admin whose `school_id == schoolID`; otherwise `httpx.ErrNotFound`.
  - sqlc: `GetClassByID`, `GetEnrollment` (by user+class), reused by guards.
  - maintest helpers: `signInAs(t, pool, userID) *http.Cookie`; `SeedAdminID`; `signInAdmin(t, pool)`; `createTeacher(t, pool, schoolID, email) uuid.UUID`.
- Consumes: `UserFromContext` (api.go), `httpx.ErrForbidden`/`ErrNotFound`.

- [ ] **Step 1: Add the authz queries to `org.sql`**

`apps/api/internal/store/queries/org.sql`:
```sql
-- name: GetClassByID :one
SELECT * FROM classes WHERE id = $1;

-- name: GetEnrollment :one
SELECT * FROM enrollments WHERE user_id = $1 AND class_id = $2;
```

- [ ] **Step 2: Regenerate sqlc**

Run: `cd apps/api && make sqlc`
Expected: new methods `GetClassByID`, `GetEnrollment` on `*sqlc.Queries`; build clean (`go build ./...`).

- [ ] **Step 3: Write the failing test**

`apps/api/internal/api/authz_test.go`:
```go
package api_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	. "mindimprint/api/internal/api"
)

// roleProbe is a trivial protected handler that 200s if reached.
func roleProbe(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) }

func TestRequireRoleForbidsWrongRole(t *testing.T) {
	pool := newAPITestPool(t)
	h := SessionAuthForTest(pool, RequireUser(RequireRole("admin")(http.HandlerFunc(roleProbe))))

	// Seeded student (Phoebe) must be forbidden from an admin-only route.
	rec := httptest.NewRecorder()
	req := withCookie(httptest.NewRequest("GET", "/probe", nil), signInSeed(t, pool))
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("student got %d, want 403", rec.Code)
	}

	// Seeded admin passes.
	rec = httptest.NewRecorder()
	req = withCookie(httptest.NewRequest("GET", "/probe", nil), signInAdmin(t, pool))
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("admin got %d, want 200", rec.Code)
	}
}

func TestAssertAdminOfSchoolRejectsOtherSchool(t *testing.T) {
	pool := newAPITestPool(t)
	a := newTestAPI(pool) // existing helper that builds *API from the pool
	ctx := WithUser(context.Background(), User{
		ID: SeedAdminID, SchoolID: SeedSchoolID, Role: "admin",
	})
	if err := a.AssertAdminOfSchoolForTest(ctx, SeedSchoolID); err != nil {
		t.Fatalf("same school must pass: %v", err)
	}
	other := mustUUID("00000000-0000-0000-0000-0000000000ff")
	if err := a.AssertAdminOfSchoolForTest(ctx, other); err == nil {
		t.Fatal("other school must be rejected")
	}
}
```

Implementer note: `SessionAuthForTest`, `newTestAPI`, `AssertAdminOfSchoolForTest`, `SeedSchoolID`, `mustUUID` are small test-only exports/helpers — add them to a new `apps/api/internal/api/export_test.go` (Go's convention for exposing unexported internals to the `_test` package). Define `SeedSchoolID = uuid.MustParse("00000000-0000-0000-0000-000000000001")` and `SeedAdminID = uuid.MustParse("00000000-0000-0000-0000-000000000005")` there; `SessionAuthForTest(pool, h)` wraps `h` with `SessionAuth(sqlc.New(pool))`; `newTestAPI(pool)` builds `New(Deps{Queries: sqlc.New(pool), Pool: pool, CookieSecure: false})`; `AssertAdminOfSchoolForTest` forwards to the unexported guard. `mustUUID` wraps `uuid.MustParse`.

- [ ] **Step 4: Run the test to verify it fails**

Run: `cd apps/api && DOCKER_HOST=unix:///var/run/docker.sock go test -p 1 -run 'TestRequireRole|TestAssertAdmin' ./internal/api/`
Expected: FAIL (undefined `RequireRole`, etc.).

- [ ] **Step 5: Write `authz.go`**

`apps/api/internal/api/authz.go`:
```go
package api

import (
	"context"
	"net/http"

	"github.com/google/uuid"

	"mindimprint/api/internal/httpx"
	"mindimprint/api/internal/store/sqlc"
)

// RequireRole rejects a request whose context user's role is not in roles (403).
// It is always composed inside RequireUser, so a missing user is already a 401 by
// the time this runs; defensively, an absent user here is also forbidden.
func RequireRole(roles ...string) func(http.Handler) http.Handler {
	allowed := make(map[string]bool, len(roles))
	for _, r := range roles {
		allowed[r] = true
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			u, ok := UserFromContext(r.Context())
			if !ok || !allowed[u.Role] {
				httpx.WriteError(w, r, httpx.ErrForbidden("权限不足"))
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// assertAdminOfSchool returns nil only if the caller is an admin of schoolID.
// Any mismatch is reported as not-found so cross-tenant probing can't enumerate.
func (a *API) assertAdminOfSchool(ctx context.Context, schoolID uuid.UUID) error {
	u, ok := UserFromContext(ctx)
	if !ok || u.Role != "admin" || u.SchoolID != schoolID {
		return httpx.ErrNotFound("资源不存在")
	}
	return nil
}

// assertTeacherOwnsClass returns the class if the caller teaches it, or is an
// admin of its school. Otherwise (and for a non-existent class) → not-found.
func (a *API) assertTeacherOwnsClass(ctx context.Context, classID uuid.UUID) (sqlc.Class, error) {
	u, ok := UserFromContext(ctx)
	if !ok {
		return sqlc.Class{}, httpx.ErrNotFound("资源不存在")
	}
	cls, err := a.d.Queries.GetClassByID(ctx, classID)
	if err != nil {
		return sqlc.Class{}, httpx.ErrNotFound("资源不存在") // ErrNoRows or bad id
	}
	if u.Role == "admin" {
		if u.SchoolID != cls.SchoolID {
			return sqlc.Class{}, httpx.ErrNotFound("资源不存在")
		}
		return cls, nil
	}
	enr, err := a.d.Queries.GetEnrollment(ctx, sqlc.GetEnrollmentParams{UserID: u.ID, ClassID: classID})
	if err != nil || enr.RoleInClass != "teacher" {
		return sqlc.Class{}, httpx.ErrNotFound("资源不存在")
	}
	return cls, nil
}
```

- [ ] **Step 6: Add `export_test.go` and the maintest helpers**

`apps/api/internal/api/export_test.go`:
```go
package api

import (
	"context"
	"net/http"

	"github.com/google/uuid"

	"github.com/jackc/pgx/v5/pgxpool"

	"mindimprint/api/internal/store/sqlc"
)

var (
	SeedSchoolID = uuid.MustParse("00000000-0000-0000-0000-000000000001")
	SeedAdminID  = uuid.MustParse("00000000-0000-0000-0000-000000000005")
)

func SessionAuthForTest(pool *pgxpool.Pool, h http.Handler) http.Handler {
	return SessionAuth(sqlc.New(pool))(h)
}

func newTestAPI(pool *pgxpool.Pool) *API {
	return New(Deps{Queries: sqlc.New(pool), Pool: pool, CookieSecure: false})
}

func (a *API) AssertAdminOfSchoolForTest(ctx context.Context, schoolID uuid.UUID) error {
	return a.assertAdminOfSchool(ctx, schoolID)
}
```
Implementer note: `export_test.go` is compiled only under test but lives in package `api` (not `api_test`), so it can see unexported members. The `_test` package (`maintest_test.go`, handler tests) then references `api.SessionAuthForTest`, etc. via the dot-import.

Append to `apps/api/internal/api/maintest_test.go`:
```go
// signInAs creates a live session for an arbitrary user and returns its cookie.
func signInAs(t *testing.T, pool *pgxpool.Pool, userID uuid.UUID) *http.Cookie {
	t.Helper()
	q := sqlc.New(pool)
	raw, hash, err := auth.NewToken()
	if err != nil {
		t.Fatalf("token: %v", err)
	}
	if _, err := q.CreateSession(context.Background(), sqlc.CreateSessionParams{
		UserID: userID, TokenHash: hash, ExpiresAt: time.Now().Add(time.Hour),
	}); err != nil {
		t.Fatalf("create session: %v", err)
	}
	return &http.Cookie{Name: "mk_session", Value: raw}
}

func signInAdmin(t *testing.T, pool *pgxpool.Pool) *http.Cookie {
	return signInAs(t, pool, SeedAdminID)
}

// createTeacher inserts a verified teacher in schoolID and returns its id.
func createTeacher(t *testing.T, pool *pgxpool.Pool, schoolID uuid.UUID, email string) uuid.UUID {
	t.Helper()
	q := sqlc.New(pool)
	u, err := q.CreateUser(context.Background(), sqlc.CreateUserParams{
		Email: email, PasswordHash: "x", Role: "teacher", SchoolID: schoolID,
		DisplayName: "T " + email, AvatarColor: "#888888",
		EmailVerifiedAt: pgtype.Timestamptz{Time: time.Now(), Valid: true},
	})
	if err != nil {
		t.Fatalf("create teacher: %v", err)
	}
	return u.ID
}
```
Also refactor the existing `signInSeed` to `return signInAs(t, pool, SeedUserID)` (DRY), and add the imports `"github.com/google/uuid"` and `"github.com/jackc/pgx/v5/pgtype"` to `maintest_test.go`.

- [ ] **Step 7: Run the tests to verify they pass**

Run: `cd apps/api && DOCKER_HOST=unix:///var/run/docker.sock go test -p 1 -run 'TestRequireRole|TestAssertAdmin' ./internal/api/`
Expected: PASS.

- [ ] **Step 8: Commit**

```bash
git add apps/api/internal/api/authz.go apps/api/internal/api/authz_test.go apps/api/internal/api/export_test.go apps/api/internal/api/maintest_test.go apps/api/internal/store/queries/org.sql apps/api/internal/store/sqlc/
git commit -m "$(printf 'feat(p3.1): authz layer — RequireRole gate + tenancy guards + test helpers\n\nCo-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>')"
```

---

### Task 5: Teacher invites — admin mints & lists

**Files:**
- Modify: `apps/api/internal/store/queries/org.sql` (+ regenerate sqlc)
- Create: `apps/api/internal/api/invites.go`
- Modify: `apps/api/internal/api/api.go` (wire two admin routes)
- Test: `apps/api/internal/api/invites_test.go`

**Interfaces:**
- Consumes: `org.NewTeacherInviteCode`, `RequireRole`, `httpx`.
- Produces: `POST /api/v1/admin/teacher-invites` → `201 {code, expires_at}`; `GET /api/v1/admin/teacher-invites` → `200 {invites:[{id,email,code,expires_at,created_at}]}`. sqlc: `CreateTeacherInvite`, `ListActiveTeacherInvitesBySchool`, `GetActiveTeacherInviteByCode`, `ConsumeTeacherInvite` (last two consumed by Task 6).

- [ ] **Step 1: Add the queries to `org.sql`**

Append to `apps/api/internal/store/queries/org.sql`:
```sql
-- name: CreateTeacherInvite :one
INSERT INTO teacher_invites (school_id, code, email, created_by, expires_at)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: ListActiveTeacherInvitesBySchool :many
SELECT * FROM teacher_invites
WHERE school_id = $1 AND consumed_at IS NULL AND expires_at > now()
ORDER BY created_at DESC;

-- name: GetActiveTeacherInviteByCode :one
SELECT * FROM teacher_invites
WHERE code = $1 AND consumed_at IS NULL AND expires_at > now();

-- name: ConsumeTeacherInvite :exec
UPDATE teacher_invites SET consumed_at = now(), consumed_by = $2 WHERE id = $1;
```

- [ ] **Step 2: Regenerate sqlc**

Run: `cd apps/api && make sqlc` then `go build ./...`
Expected: four new methods; clean build.

- [ ] **Step 3: Write the failing test**

`apps/api/internal/api/invites_test.go`:
```go
package api_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	. "mindimprint/api/internal/api"
)

func TestAdminCreatesAndListsTeacherInvite(t *testing.T) {
	pool := newAPITestPool(t)
	h := New(DepsForTest(pool)).Handler()
	admin := signInAdmin(t, pool)

	// Create.
	body, _ := json.Marshal(map[string]any{"email": "newteacher@demo.local"})
	rec := httptest.NewRecorder()
	req := withCookie(httptest.NewRequest("POST", "/api/v1/admin/teacher-invites", bytes.NewReader(body)), admin)
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create got %d body=%s", rec.Code, rec.Body)
	}
	var created struct {
		Code      string `json:"code"`
		ExpiresAt string `json:"expires_at"`
	}
	json.Unmarshal(rec.Body.Bytes(), &created)
	if !strings.HasPrefix(created.Code, "T-") || created.ExpiresAt == "" {
		t.Fatalf("bad create payload: %+v", created)
	}

	// List shows it.
	rec = httptest.NewRecorder()
	req = withCookie(httptest.NewRequest("GET", "/api/v1/admin/teacher-invites", nil), admin)
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), created.Code) {
		t.Fatalf("list got %d body=%s", rec.Code, rec.Body)
	}
}

func TestTeacherInviteRequiresAdmin(t *testing.T) {
	pool := newAPITestPool(t)
	h := New(DepsForTest(pool)).Handler()
	rec := httptest.NewRecorder()
	req := withCookie(httptest.NewRequest("POST", "/api/v1/admin/teacher-invites", strings.NewReader("{}")), signInSeed(t, pool))
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("student got %d, want 403", rec.Code)
	}
}
```
Implementer note: add `func DepsForTest(pool *pgxpool.Pool) Deps` to `export_test.go` returning `Deps{Queries: sqlc.New(pool), Pool: pool, CookieSecure: false}` (handler tests need a fully-wired `Deps`; the agent/gateway fields stay nil because no org route calls them).

- [ ] **Step 4: Run the test to verify it fails**

Run: `cd apps/api && DOCKER_HOST=unix:///var/run/docker.sock go test -p 1 -run 'TeacherInvite' ./internal/api/`
Expected: FAIL (route 404 / handler undefined).

- [ ] **Step 5: Write the handler**

`apps/api/internal/api/invites.go`:
```go
package api

import (
	"net/http"
	"strings"
	"time"

	"mindimprint/api/internal/httpx"
	"mindimprint/api/internal/org"
	"mindimprint/api/internal/store/sqlc"
)

const defaultInviteTTLDays = 14

func (a *API) createTeacherInvite(w http.ResponseWriter, r *http.Request) {
	u, _ := UserFromContext(r.Context())
	var body struct {
		Email       string `json:"email"`
		ExpiresDays int    `json:"expires_days"`
	}
	if err := decodeJSON(r, &body); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	days := body.ExpiresDays
	if days <= 0 {
		days = defaultInviteTTLDays
	}
	code, err := org.NewTeacherInviteCode()
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	var email *string
	if e := strings.TrimSpace(strings.ToLower(body.Email)); e != "" {
		email = &e
	}
	inv, err := a.d.Queries.CreateTeacherInvite(r.Context(), sqlc.CreateTeacherInviteParams{
		SchoolID:  u.SchoolID,
		Code:      code,
		Email:     email,
		CreatedBy: u.ID,
		ExpiresAt: time.Now().Add(time.Duration(days) * 24 * time.Hour),
	})
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, map[string]any{
		"code":       inv.Code,
		"expires_at": inv.ExpiresAt.Format(tsLayout),
	})
}

func (a *API) listTeacherInvites(w http.ResponseWriter, r *http.Request) {
	u, _ := UserFromContext(r.Context())
	rows, err := a.d.Queries.ListActiveTeacherInvitesBySchool(r.Context(), u.SchoolID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	out := make([]map[string]any, 0, len(rows))
	for _, inv := range rows {
		var email string
		if inv.Email != nil {
			email = *inv.Email
		}
		out = append(out, map[string]any{
			"id":         inv.ID.String(),
			"email":      email,
			"code":       inv.Code,
			"expires_at": inv.ExpiresAt.Format(tsLayout),
			"created_at": inv.CreatedAt.Format(tsLayout),
		})
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"invites": out})
}
```
Implementer note: `teacher_invites.expires_at`/`created_at` are NOT NULL, so this codebase's sqlc maps them to `time.Time` (same as `taskDTO`'s `CreatedAt`) — hence `ExpiresAt: time.Now().Add(...)` and `inv.ExpiresAt.Format(tsLayout)`, no `pgtype` wrapper. If `make sqlc` surprises you with `pgtype.Timestamptz` here, switch to `pgtype.Timestamptz{Time: ..., Valid: true}` and `.Time.Format(...)`. The `Email` param is `*string` (nullable text).

- [ ] **Step 6: Wire the routes in `api.go`**

In `Handler()`, after the `protected` closure, add an admin group and register the two routes:
```go
	adminOnly := func(h http.HandlerFunc) http.Handler { return RequireUser(RequireRole("admin")(http.HandlerFunc(h))) }
	mux.Handle("POST /api/v1/admin/teacher-invites", adminOnly(a.createTeacherInvite))
	mux.Handle("GET /api/v1/admin/teacher-invites", adminOnly(a.listTeacherInvites))
```
(`RequireUser` stays the outermost so an unauthenticated caller still gets 401, not 403.)

- [ ] **Step 7: Run the tests to verify they pass**

Run: `cd apps/api && DOCKER_HOST=unix:///var/run/docker.sock go test -p 1 -run 'TeacherInvite' ./internal/api/`
Expected: PASS.

- [ ] **Step 8: Commit**

```bash
git add apps/api/internal/api/invites.go apps/api/internal/api/invites_test.go apps/api/internal/api/api.go apps/api/internal/api/export_test.go apps/api/internal/store/queries/org.sql apps/api/internal/store/sqlc/
git commit -m "$(printf 'feat(p3.1): admin teacher-invite create + list endpoints\n\nCo-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>')"
```

---

### Task 6: Signup extension — teacher invite code path

**Files:**
- Modify: `apps/api/internal/api/signup.go`
- Test: `apps/api/internal/api/signup_test.go` (add cases)

**Interfaces:**
- Consumes: `GetActiveTeacherInviteByCode`, `ConsumeTeacherInvite` (Task 5), existing `GetClassByJoinCode`, `CreateUser`, `CreateEnrollment`.
- Produces: signup resolves the submitted `join_code` as a teacher invite first, else a class join code. Teacher path: create `role='teacher'` user (school from invite), consume invite, **no enrollment**, all in one tx.

- [ ] **Step 1: Write the failing tests**

Add to `apps/api/internal/api/signup_test.go`:
```go
func TestSignupWithTeacherInviteCreatesTeacher(t *testing.T) {
	pool := newAPITestPool(t)
	h := New(DepsForTest(pool)).Handler()

	// Admin mints an invite (no bound email).
	admin := signInAdmin(t, pool)
	rec := httptest.NewRecorder()
	req := withCookie(httptest.NewRequest("POST", "/api/v1/admin/teacher-invites", strings.NewReader("{}")), admin)
	h.ServeHTTP(rec, req)
	var created struct{ Code string `json:"code"` }
	json.Unmarshal(rec.Body.Bytes(), &created)

	// Teacher signs up with it.
	body, _ := json.Marshal(map[string]any{
		"email": "tt@demo.local", "password": "password123",
		"display_name": "Teacher T", "join_code": created.Code,
	})
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("POST", "/api/v1/auth/signup", bytes.NewReader(body)))
	if rec.Code != http.StatusCreated {
		t.Fatalf("teacher signup got %d body=%s", rec.Code, rec.Body)
	}

	q := sqlc.New(pool)
	u, err := q.GetUserByEmail(context.Background(), "tt@demo.local")
	if err != nil || u.Role != "teacher" {
		t.Fatalf("expected teacher user, got role=%q err=%v", u.Role, err)
	}
	if u.SchoolID != SeedSchoolID {
		t.Fatalf("teacher school = %v, want seed", u.SchoolID)
	}
	// Invite is consumed → reusing it fails.
	rec = httptest.NewRecorder()
	body2, _ := json.Marshal(map[string]any{
		"email": "tt2@demo.local", "password": "password123",
		"display_name": "Teacher Two", "join_code": created.Code,
	})
	h.ServeHTTP(rec, httptest.NewRequest("POST", "/api/v1/auth/signup", bytes.NewReader(body2)))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("reused invite got %d, want 400", rec.Code)
	}
}

func TestSignupTeacherInviteEmailMismatch(t *testing.T) {
	pool := newAPITestPool(t)
	h := New(DepsForTest(pool)).Handler()
	admin := signInAdmin(t, pool)
	body, _ := json.Marshal(map[string]any{"email": "bound@demo.local"})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("POST", "/api/v1/admin/teacher-invites", bytes.NewReader(body)), admin))
	var created struct{ Code string `json:"code"` }
	json.Unmarshal(rec.Body.Bytes(), &created)

	// Sign up with a different email than the invite was bound to.
	su, _ := json.Marshal(map[string]any{
		"email": "someoneelse@demo.local", "password": "password123",
		"display_name": "X", "join_code": created.Code,
	})
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("POST", "/api/v1/auth/signup", bytes.NewReader(su)))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("email mismatch got %d, want 400", rec.Code)
	}
}
```
(The existing student-signup test still passes — the class path is unchanged.)

- [ ] **Step 2: Run the tests to verify they fail**

Run: `cd apps/api && DOCKER_HOST=unix:///var/run/docker.sock go test -p 1 -run 'TestSignup' ./internal/api/`
Expected: the two new tests FAIL (teacher created as student / invite not consumed).

- [ ] **Step 3: Rewrite `signup` to resolve the code**

Replace the body of `signup` between the validation block and the `WriteJSON(201)` with a resolver. Replace lines 42–92 of `signup.go` with:
```go
	hash, err := auth.HashPassword(body.Password)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}

	// Resolve the code: teacher invite first, then class join code.
	if inv, ierr := a.d.Queries.GetActiveTeacherInviteByCode(r.Context(), body.JoinCode); ierr == nil {
		if inv.Email != nil && *inv.Email != body.Email {
			httpx.WriteError(w, r, httpx.ErrInvalidJoinCode())
			return
		}
		a.signupTeacher(w, r, body.Email, body.DisplayName, hash, inv)
		return
	}

	cls, err := a.d.Queries.GetClassByJoinCode(r.Context(), body.JoinCode)
	if err != nil {
		httpx.WriteError(w, r, httpx.ErrInvalidJoinCode())
		return
	}
	a.signupStudent(w, r, body.Email, body.DisplayName, hash, cls)
}

// signupTeacher creates a teacher (school from the invite, no enrollment) and
// consumes the invite, in one transaction.
func (a *API) signupTeacher(w http.ResponseWriter, r *http.Request, email, displayName, hash string, inv sqlc.TeacherInvite) {
	tx, err := a.d.Pool.Begin(r.Context())
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	defer func() { _ = tx.Rollback(r.Context()) }()
	qtx := a.d.Queries.WithTx(tx)

	u, err := qtx.CreateUser(r.Context(), sqlc.CreateUserParams{
		Email: email, PasswordHash: hash, Role: "teacher", SchoolID: inv.SchoolID,
		DisplayName: displayName, AvatarColor: defaultAvatarColor,
		EmailVerifiedAt: pgtype.Timestamptz{Time: time.Now(), Valid: true},
	})
	if err != nil {
		if isUniqueViolation(err) {
			httpx.WriteError(w, r, httpx.ErrEmailTaken())
			return
		}
		httpx.WriteError(w, r, err)
		return
	}
	if err := qtx.ConsumeTeacherInvite(r.Context(), sqlc.ConsumeTeacherInviteParams{
		ID: inv.ID, ConsumedBy: pgtype.UUID{Bytes: u.ID, Valid: true},
	}); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if err := tx.Commit(r.Context()); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, map[string]any{})
}

// signupStudent is the unchanged P2 student path, extracted verbatim.
func (a *API) signupStudent(w http.ResponseWriter, r *http.Request, email, displayName, hash string, cls sqlc.Class) {
	tx, err := a.d.Pool.Begin(r.Context())
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	defer func() { _ = tx.Rollback(r.Context()) }()
	qtx := a.d.Queries.WithTx(tx)

	u, err := qtx.CreateUser(r.Context(), sqlc.CreateUserParams{
		Email: email, PasswordHash: hash, Role: "student", SchoolID: cls.SchoolID,
		DisplayName: displayName, AvatarColor: defaultAvatarColor,
		EmailVerifiedAt: pgtype.Timestamptz{Time: time.Now(), Valid: true},
	})
	if err != nil {
		if isUniqueViolation(err) {
			httpx.WriteError(w, r, httpx.ErrEmailTaken())
			return
		}
		httpx.WriteError(w, r, err)
		return
	}
	if _, err := qtx.CreateEnrollment(r.Context(), sqlc.CreateEnrollmentParams{
		UserID: u.ID, ClassID: cls.ID, RoleInClass: "student",
	}); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if err := tx.Commit(r.Context()); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, map[string]any{})
}

// isUniqueViolation reports whether err is a Postgres 23505 (unique_violation).
func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}
```
Implementer note: confirm the generated `ConsumeTeacherInviteParams` field name/type for `consumed_by` (likely `ConsumedBy pgtype.UUID`); match it. Keep the existing imports (`errors`, `pgconn`, `pgtype`, `time`). The `pgtype.UUID{Bytes: u.ID, Valid: true}` conversion assumes `u.ID` is a `uuid.UUID` ([16]byte) — `pgtype.UUID.Bytes` is `[16]byte`, so use `pgtype.UUID{Bytes: u.ID, Valid: true}` (uuid.UUID is assignable to [16]byte).

- [ ] **Step 4: Run the tests to verify they pass**

Run: `cd apps/api && DOCKER_HOST=unix:///var/run/docker.sock go test -p 1 -run 'TestSignup' ./internal/api/`
Expected: PASS (new teacher cases + existing student case).

- [ ] **Step 5: Commit**

```bash
git add apps/api/internal/api/signup.go apps/api/internal/api/signup_test.go
git commit -m "$(printf 'feat(p3.1): signup resolves teacher invite codes (teacher path, no enrollment)\n\nCo-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>')"
```

---

### Task 7: Class create + list

**Files:**
- Modify: `apps/api/internal/store/queries/org.sql` (+ regenerate sqlc)
- Create: `apps/api/internal/api/classes.go`
- Create: `apps/api/internal/api/classes_dto.go`
- Modify: `apps/api/internal/api/api.go` (wire two routes)
- Test: `apps/api/internal/api/classes_test.go`

**Interfaces:**
- Produces: `POST /api/v1/classes` (`{name, teacher_user_id?}`) → `201 {class}`; `GET /api/v1/classes` → `200 {classes:[…]}`. classDTO `{id, name, join_code, school_id, created_at}`. sqlc: `CreateClass`, `ListClassesForTeacher`, `ListClassesBySchool`, `GetUserByIDInSchool`.
- Consumes: `org.NewClassJoinCode`, `RequireRole`, tenancy context (`UserFromContext`).

- [ ] **Step 1: Add the queries**

Append to `org.sql`:
```sql
-- name: CreateClass :one
INSERT INTO classes (school_id, name, join_code, created_by)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: ListClassesForTeacher :many
SELECT c.* FROM classes c
JOIN enrollments e ON e.class_id = c.id
WHERE e.user_id = $1 AND e.role_in_class = 'teacher'
ORDER BY c.name;

-- name: ListClassesBySchool :many
SELECT * FROM classes WHERE school_id = $1 ORDER BY name;

-- name: GetUserByIDInSchool :one
SELECT * FROM users WHERE id = $1 AND school_id = $2;
```

- [ ] **Step 2: Regenerate sqlc**

Run: `cd apps/api && make sqlc && go build ./...`
Expected: clean build with the four methods.

- [ ] **Step 3: Write the failing test**

`apps/api/internal/api/classes_test.go`:
```go
package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	. "mindimprint/api/internal/api"
)

func TestTeacherCreatesAndListsOwnClass(t *testing.T) {
	pool := newAPITestPool(t)
	h := New(DepsForTest(pool)).Handler()
	teacherID := createTeacher(t, pool, SeedSchoolID, "tc@demo.local")
	teacher := signInAs(t, pool, teacherID)

	body, _ := json.Marshal(map[string]any{"name": "Block 3 History"})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("POST", "/api/v1/classes", bytes.NewReader(body)), teacher))
	if rec.Code != http.StatusCreated {
		t.Fatalf("create got %d body=%s", rec.Code, rec.Body)
	}
	var created struct {
		Class struct {
			ID       string `json:"id"`
			JoinCode string `json:"join_code"`
		} `json:"class"`
	}
	json.Unmarshal(rec.Body.Bytes(), &created)
	if created.Class.JoinCode == "" {
		t.Fatal("class missing join_code")
	}
	// Creator is enrolled as teacher.
	q := mustNewQueries(pool)
	enr, err := q.GetEnrollment(context.Background(), GetEnrollmentParamsForTest(teacherID, created.Class.ID))
	if err != nil || enr.RoleInClass != "teacher" {
		t.Fatalf("creator not enrolled as teacher: %v", err)
	}

	// List returns it.
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("GET", "/api/v1/classes", nil), teacher))
	if rec.Code != http.StatusOK || !bytes.Contains(rec.Body.Bytes(), []byte("Block 3 History")) {
		t.Fatalf("list got %d body=%s", rec.Code, rec.Body)
	}
}

func TestStudentCannotCreateClass(t *testing.T) {
	pool := newAPITestPool(t)
	h := New(DepsForTest(pool)).Handler()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("POST", "/api/v1/classes", bytes.NewReader([]byte(`{"name":"x"}`))), signInSeed(t, pool)))
	if rec.Code != http.StatusForbidden {
		t.Fatalf("student got %d, want 403", rec.Code)
	}
}
```
Implementer note: add `mustNewQueries(pool) *sqlc.Queries` and `GetEnrollmentParamsForTest(userID uuid.UUID, classID string) sqlc.GetEnrollmentParams` to `export_test.go` (the latter parses `classID` via `uuid.MustParse`). These keep the `_test` package free of direct sqlc-internal coupling.

- [ ] **Step 4: Run the test to verify it fails**

Run: `cd apps/api && DOCKER_HOST=unix:///var/run/docker.sock go test -p 1 -run 'TestTeacherCreates|TestStudentCannotCreate' ./internal/api/`
Expected: FAIL.

- [ ] **Step 5: Write the DTO and handler**

`apps/api/internal/api/classes_dto.go`:
```go
package api

import "mindimprint/api/internal/store/sqlc"

type classDTO struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	JoinCode  string `json:"join_code"`
	SchoolID  string `json:"school_id"`
	CreatedAt string `json:"created_at"`
}

func toClassDTO(c sqlc.Class) classDTO {
	return classDTO{
		ID:        c.ID.String(),
		Name:      c.Name,
		JoinCode:  c.JoinCode,
		SchoolID:  c.SchoolID.String(),
		CreatedAt: c.CreatedAt.Format(tsLayout),
	}
}
```
Implementer note: `classes.created_at` is NOT NULL → `time.Time` (`c.CreatedAt.Format(tsLayout)`), matching `taskDTO`. The new nullable `classes.created_by` maps to `pgtype.UUID` (like `card_instances.parent_node_id` in the existing `toCardDTO`) — not surfaced in this DTO, but that's its type in `CreateClassParams`.

`apps/api/internal/api/classes.go`:
```go
package api

import (
	"net/http"
	"strings"

	"github.com/google/uuid"

	"mindimprint/api/internal/httpx"
	"mindimprint/api/internal/org"
	"mindimprint/api/internal/store/sqlc"
)

// createClass: teacher creates a class they own; admin may create one for any
// teacher in their school via teacher_user_id. The creator/assignee is enrolled
// as role_in_class='teacher'. One transaction (class + enrollment).
func (a *API) createClass(w http.ResponseWriter, r *http.Request) {
	u, _ := UserFromContext(r.Context())
	var body struct {
		Name          string `json:"name"`
		TeacherUserID string `json:"teacher_user_id"`
	}
	if err := decodeJSON(r, &body); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if strings.TrimSpace(body.Name) == "" {
		httpx.WriteError(w, r, httpx.ErrBadRequest("validation_failed", "班级名称不能为空", nil))
		return
	}

	// Determine the owning teacher.
	teacherID := u.ID
	if u.Role == "admin" {
		if body.TeacherUserID == "" {
			httpx.WriteError(w, r, httpx.ErrBadRequest("validation_failed", "需要指定班级教师", nil))
			return
		}
		tid, err := uuid.Parse(body.TeacherUserID)
		if err != nil {
			httpx.WriteError(w, r, httpx.ErrBadRequest("validation_failed", "teacher_user_id 无效", nil))
			return
		}
		// The assignee must be a teacher in the admin's school.
		tu, err := a.d.Queries.GetUserByIDInSchool(r.Context(), sqlc.GetUserByIDInSchoolParams{ID: tid, SchoolID: u.SchoolID})
		if err != nil || tu.Role != "teacher" {
			httpx.WriteError(w, r, httpx.ErrBadRequest("validation_failed", "指定的教师无效", nil))
			return
		}
		teacherID = tid
	}

	code, err := org.NewClassJoinCode()
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}

	tx, err := a.d.Pool.Begin(r.Context())
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	defer func() { _ = tx.Rollback(r.Context()) }()
	qtx := a.d.Queries.WithTx(tx)

	cls, err := qtx.CreateClass(r.Context(), sqlc.CreateClassParams{
		SchoolID: u.SchoolID, Name: strings.TrimSpace(body.Name), JoinCode: code,
		CreatedBy: pgtype.UUID{Bytes: u.ID, Valid: true},
	})
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if _, err := qtx.CreateEnrollment(r.Context(), sqlc.CreateEnrollmentParams{
		UserID: teacherID, ClassID: cls.ID, RoleInClass: "teacher",
	}); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if err := tx.Commit(r.Context()); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, map[string]any{"class": toClassDTO(cls)})
}

func (a *API) listClasses(w http.ResponseWriter, r *http.Request) {
	u, _ := UserFromContext(r.Context())
	var rows []sqlc.Class
	var err error
	if u.Role == "admin" {
		rows, err = a.d.Queries.ListClassesBySchool(r.Context(), u.SchoolID)
	} else {
		rows, err = a.d.Queries.ListClassesForTeacher(r.Context(), u.ID)
	}
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	out := make([]classDTO, 0, len(rows))
	for _, c := range rows {
		out = append(out, toClassDTO(c))
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"classes": out})
}
```
Implementer note: import `"github.com/jackc/pgx/v5/pgtype"`. If `CreateClassParams.CreatedBy` is typed `pgtype.UUID`, the conversion above is correct; if sqlc generated `*uuid.UUID`, pass `&u.ID`. Match the generated type.

- [ ] **Step 6: Wire the routes**

In `api.go` add a teacher-or-admin group and register:
```go
	teacherOrAdmin := func(h http.HandlerFunc) http.Handler {
		return RequireUser(RequireRole("teacher", "admin")(http.HandlerFunc(h)))
	}
	mux.Handle("POST /api/v1/classes", teacherOrAdmin(a.createClass))
	mux.Handle("GET /api/v1/classes", teacherOrAdmin(a.listClasses))
```

- [ ] **Step 7: Run the tests to verify they pass**

Run: `cd apps/api && DOCKER_HOST=unix:///var/run/docker.sock go test -p 1 -run 'TestTeacherCreates|TestStudentCannotCreate' ./internal/api/`
Expected: PASS.

- [ ] **Step 8: Commit**

```bash
git add apps/api/internal/api/classes.go apps/api/internal/api/classes_dto.go apps/api/internal/api/classes_test.go apps/api/internal/api/api.go apps/api/internal/api/export_test.go apps/api/internal/store/queries/org.sql apps/api/internal/store/sqlc/
git commit -m "$(printf 'feat(p3.1): class create + list (teacher owns; admin assigns)\n\nCo-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>')"
```

---

### Task 8: Class detail + roster (aggregate signals)

**Files:**
- Modify: `apps/api/internal/store/queries/org.sql` (+ regenerate sqlc)
- Modify: `apps/api/internal/api/classes.go` (add `getClass`)
- Modify: `apps/api/internal/api/classes_dto.go` (roster DTO)
- Modify: `apps/api/internal/api/api.go` (wire `GET /classes/{id}`)
- Test: `apps/api/internal/api/classes_test.go` (add cases)

**Interfaces:**
- Produces: `GET /api/v1/classes/:id` → `200 {class, roster:[{id, display_name, email, last_active_at, task_count, evaluation_count, card_count}]}`. Tenancy-guarded via `assertTeacherOwnsClass`. sqlc: `GetClassRoster`.

- [ ] **Step 1: Add the roster query**

Append to `org.sql`:
```sql
-- name: GetClassRoster :many
SELECT u.id, u.display_name, u.email,
       MAX(t.last_active_at) AS last_active_at,
       COUNT(DISTINCT t.id)  AS task_count,
       COUNT(DISTINCT ev.id) AS evaluation_count,
       COUNT(DISTINCT ci.id) AS card_count
FROM enrollments e
JOIN users u             ON u.id = e.user_id
LEFT JOIN tasks t        ON t.user_id = u.id
LEFT JOIN evaluations ev ON ev.task_id = t.id
LEFT JOIN card_instances ci ON ci.task_id = t.id
WHERE e.class_id = $1 AND e.role_in_class = 'student'
GROUP BY u.id, u.display_name, u.email
ORDER BY u.display_name;
```

- [ ] **Step 2: Regenerate sqlc**

Run: `cd apps/api && make sqlc && go build ./...`
Expected: `GetClassRoster` method; row struct `GetClassRosterRow{ID, DisplayName, Email, LastActiveAt, TaskCount, EvaluationCount, CardCount}` (counts are `int64`, `LastActiveAt` nullable `pgtype.Timestamptz` or `interface{}` — inspect and adapt the DTO).

- [ ] **Step 3: Write the failing test**

Add to `classes_test.go`:
```go
func TestClassRosterShowsAggregateSignals(t *testing.T) {
	pool := newAPITestPool(t)
	h := New(DepsForTest(pool)).Handler()
	teacherID := createTeacher(t, pool, SeedSchoolID, "rt@demo.local")
	teacher := signInAs(t, pool, teacherID)

	// Create a class, then enroll the seeded student (Phoebe) into it directly.
	classID := createClassViaAPI(t, h, teacher, "Roster Class")
	enrollStudent(t, pool, SeedUserID, classID)
	// Give Phoebe one task so a count is non-zero.
	seedTaskFor(t, pool, SeedUserID)

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("GET", "/api/v1/classes/"+classID, nil), teacher))
	if rec.Code != http.StatusOK {
		t.Fatalf("roster got %d body=%s", rec.Code, rec.Body)
	}
	var resp struct {
		Roster []struct {
			Email     string `json:"email"`
			TaskCount int    `json:"task_count"`
		} `json:"roster"`
	}
	json.Unmarshal(rec.Body.Bytes(), &resp)
	if len(resp.Roster) != 1 || resp.Roster[0].TaskCount < 1 {
		t.Fatalf("unexpected roster: %+v", resp.Roster)
	}
	// Roster must NOT leak any contents field.
	if bytes.Contains(rec.Body.Bytes(), []byte("narrative")) {
		t.Fatal("roster leaked evaluation contents")
	}
}

func TestRosterDeniedToOtherSchoolTeacher(t *testing.T) {
	pool := newAPITestPool(t)
	h := New(DepsForTest(pool)).Handler()
	owner := signInAs(t, pool, createTeacher(t, pool, SeedSchoolID, "owner@demo.local"))
	classID := createClassViaAPI(t, h, owner, "Owned")

	// A teacher in a different school must get 404 (existence hidden).
	otherSchool := seedSecondSchool(t, pool)
	stranger := signInAs(t, pool, createTeacher(t, pool, otherSchool, "stranger@other.local"))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("GET", "/api/v1/classes/"+classID, nil), stranger))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("cross-school teacher got %d, want 404", rec.Code)
	}
}
```
Implementer note: add these test helpers to `export_test.go` / `maintest_test.go`:
- `createClassViaAPI(t, h, cookie, name) string` — POSTs `/api/v1/classes`, returns the new class id.
- `enrollStudent(t, pool, userID, classID)` — `CreateEnrollment(role_in_class='student')`.
- `seedTaskFor(t, pool, userID)` — `CreateTask` with a dummy title.
- `seedSecondSchool(t, pool) uuid.UUID` — inserts a second school, returns its id.

- [ ] **Step 4: Run the test to verify it fails**

Run: `cd apps/api && DOCKER_HOST=unix:///var/run/docker.sock go test -p 1 -run 'TestClassRoster|TestRosterDenied' ./internal/api/`
Expected: FAIL.

- [ ] **Step 5: Add the roster DTO and handler**

Append to `classes_dto.go`:
```go
type rosterEntryDTO struct {
	ID              string  `json:"id"`
	DisplayName     string  `json:"display_name"`
	Email           string  `json:"email"`
	LastActiveAt    *string `json:"last_active_at"`
	TaskCount       int64   `json:"task_count"`
	EvaluationCount int64   `json:"evaluation_count"`
	CardCount       int64   `json:"card_count"`
}
```

Add to `classes.go`:
```go
func (a *API) getClass(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		httpx.WriteError(w, r, httpx.ErrNotFound("资源不存在"))
		return
	}
	cls, err := a.assertTeacherOwnsClass(r.Context(), id)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	rows, err := a.d.Queries.GetClassRoster(r.Context(), id)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	roster := make([]rosterEntryDTO, 0, len(rows))
	for _, row := range rows {
		e := rosterEntryDTO{
			ID: row.ID.String(), DisplayName: row.DisplayName, Email: row.Email,
			TaskCount: row.TaskCount, EvaluationCount: row.EvaluationCount, CardCount: row.CardCount,
		}
		if row.LastActiveAt.Valid {
			s := row.LastActiveAt.Time.Format(tsLayout)
			e.LastActiveAt = &s
		}
		roster = append(roster, e)
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"class": toClassDTO(cls), "roster": roster})
}
```
Implementer note: `row.LastActiveAt` is nullable from `MAX(...)` over a LEFT JOIN — sqlc usually types it `pgtype.Timestamptz` (use `.Valid`/`.Time`). If it instead generated `interface{}`, add a typed cast in the query (`MAX(t.last_active_at)::timestamptz`) and regenerate. Counts may be `int64`; match the DTO field types to the generated row.

- [ ] **Step 6: Wire the route**

In `api.go`: `mux.Handle("GET /api/v1/classes/{id}", teacherOrAdmin(a.getClass))`

- [ ] **Step 7: Run the tests to verify they pass**

Run: `cd apps/api && DOCKER_HOST=unix:///var/run/docker.sock go test -p 1 -run 'TestClassRoster|TestRosterDenied' ./internal/api/`
Expected: PASS.

- [ ] **Step 8: Commit**

```bash
git add apps/api/internal/api/classes.go apps/api/internal/api/classes_dto.go apps/api/internal/api/classes_test.go apps/api/internal/api/api.go apps/api/internal/api/export_test.go apps/api/internal/api/maintest_test.go apps/api/internal/store/queries/org.sql apps/api/internal/store/sqlc/
git commit -m "$(printf 'feat(p3.1): class detail + roster with aggregate signals\n\nCo-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>')"
```

---

### Task 9: Class patch (rename / regenerate code) + remove enrollment

**Files:**
- Modify: `apps/api/internal/store/queries/org.sql` (+ regenerate sqlc)
- Modify: `apps/api/internal/api/classes.go` (add `patchClass`, `removeEnrollment`)
- Modify: `apps/api/internal/api/api.go` (wire two routes)
- Test: `apps/api/internal/api/classes_test.go` (add cases)

**Interfaces:**
- Produces: `PATCH /api/v1/classes/:id` (`{name?, regenerate_join_code?}`) → `200 {class}`; `DELETE /api/v1/classes/:id/enrollments/:userId` → `204`. sqlc: `UpdateClassName`, `SetClassJoinCode`, `DeleteEnrollment`.

- [ ] **Step 1: Add the queries**

Append to `org.sql`:
```sql
-- name: UpdateClassName :one
UPDATE classes SET name = $2 WHERE id = $1 RETURNING *;

-- name: SetClassJoinCode :one
UPDATE classes SET join_code = $2 WHERE id = $1 RETURNING *;

-- name: DeleteEnrollment :execrows
DELETE FROM enrollments WHERE class_id = $1 AND user_id = $2 AND role_in_class = 'student';
```

- [ ] **Step 2: Regenerate sqlc**

Run: `cd apps/api && make sqlc && go build ./...`
Expected: three methods; `DeleteEnrollment` returns `(int64, error)`.

- [ ] **Step 3: Write the failing test**

Add to `classes_test.go`:
```go
func TestPatchClassRenameAndRegenerate(t *testing.T) {
	pool := newAPITestPool(t)
	h := New(DepsForTest(pool)).Handler()
	teacher := signInAs(t, pool, createTeacher(t, pool, SeedSchoolID, "pt@demo.local"))
	classID := createClassViaAPI(t, h, teacher, "Old Name")

	body, _ := json.Marshal(map[string]any{"name": "New Name", "regenerate_join_code": true})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("PATCH", "/api/v1/classes/"+classID, bytes.NewReader(body)), teacher))
	if rec.Code != http.StatusOK {
		t.Fatalf("patch got %d body=%s", rec.Code, rec.Body)
	}
	var resp struct{ Class struct{ Name, JoinCode string } `json:"class"` }
	json.Unmarshal(rec.Body.Bytes(), &resp)
	if resp.Class.Name != "New Name" || resp.Class.JoinCode == "" {
		t.Fatalf("rename/regenerate failed: %+v", resp.Class)
	}
}

func TestRemoveStudentFromClass(t *testing.T) {
	pool := newAPITestPool(t)
	h := New(DepsForTest(pool)).Handler()
	teacher := signInAs(t, pool, createTeacher(t, pool, SeedSchoolID, "rm@demo.local"))
	classID := createClassViaAPI(t, h, teacher, "RM Class")
	enrollStudent(t, pool, SeedUserID, classID)

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("DELETE", "/api/v1/classes/"+classID+"/enrollments/"+SeedUserID.String(), nil), teacher))
	if rec.Code != http.StatusNoContent {
		t.Fatalf("remove got %d", rec.Code)
	}
	// The student account still exists (only the enrollment was removed).
	if _, err := mustNewQueries(pool).GetUserByID(context.Background(), SeedUserID); err != nil {
		t.Fatalf("student account must survive: %v", err)
	}
}
```

- [ ] **Step 4: Run the test to verify it fails**

Run: `cd apps/api && DOCKER_HOST=unix:///var/run/docker.sock go test -p 1 -run 'TestPatchClass|TestRemoveStudent' ./internal/api/`
Expected: FAIL.

- [ ] **Step 5: Add the handlers**

Add to `classes.go`:
```go
func (a *API) patchClass(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		httpx.WriteError(w, r, httpx.ErrNotFound("资源不存在"))
		return
	}
	if _, err := a.assertTeacherOwnsClass(r.Context(), id); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	var body struct {
		Name               *string `json:"name"`
		RegenerateJoinCode bool    `json:"regenerate_join_code"`
	}
	if err := decodeJSON(r, &body); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	var cls sqlc.Class
	if body.Name != nil {
		if strings.TrimSpace(*body.Name) == "" {
			httpx.WriteError(w, r, httpx.ErrBadRequest("validation_failed", "班级名称不能为空", nil))
			return
		}
		cls, err = a.d.Queries.UpdateClassName(r.Context(), sqlc.UpdateClassNameParams{ID: id, Name: strings.TrimSpace(*body.Name)})
		if err != nil {
			httpx.WriteError(w, r, err)
			return
		}
	}
	if body.RegenerateJoinCode {
		code, cerr := org.NewClassJoinCode()
		if cerr != nil {
			httpx.WriteError(w, r, cerr)
			return
		}
		cls, err = a.d.Queries.SetClassJoinCode(r.Context(), sqlc.SetClassJoinCodeParams{ID: id, JoinCode: code})
		if err != nil {
			httpx.WriteError(w, r, err)
			return
		}
	}
	if cls.ID == (uuid.UUID{}) { // neither field changed → reload current state
		cls, err = a.d.Queries.GetClassByID(r.Context(), id)
		if err != nil {
			httpx.WriteError(w, r, err)
			return
		}
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"class": toClassDTO(cls)})
}

func (a *API) removeEnrollment(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		httpx.WriteError(w, r, httpx.ErrNotFound("资源不存在"))
		return
	}
	if _, err := a.assertTeacherOwnsClass(r.Context(), id); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	userID, err := uuid.Parse(r.PathValue("userId"))
	if err != nil {
		httpx.WriteError(w, r, httpx.ErrNotFound("资源不存在"))
		return
	}
	if _, err := a.d.Queries.DeleteEnrollment(r.Context(), sqlc.DeleteEnrollmentParams{ClassID: id, UserID: userID}); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
```
Implementer note: `sqlc.Class` zero-value check — if `Class.ID` is `pgtype.UUID` rather than `uuid.UUID`, compare against its zero differently (e.g. track a `changed bool`). Simpler and type-agnostic: introduce a local `changed := false`, set it true in each branch, and `if !changed { cls, err = GetClassByID(...) }`. Prefer that to the zero-value comparison.

- [ ] **Step 6: Wire the routes**

```go
	mux.Handle("PATCH /api/v1/classes/{id}", teacherOrAdmin(a.patchClass))
	mux.Handle("DELETE /api/v1/classes/{id}/enrollments/{userId}", teacherOrAdmin(a.removeEnrollment))
```

- [ ] **Step 7: Run the tests to verify they pass**

Run: `cd apps/api && DOCKER_HOST=unix:///var/run/docker.sock go test -p 1 -run 'TestPatchClass|TestRemoveStudent' ./internal/api/`
Expected: PASS.

- [ ] **Step 8: Commit**

```bash
git add apps/api/internal/api/classes.go apps/api/internal/api/classes_test.go apps/api/internal/api/api.go apps/api/internal/store/queries/org.sql apps/api/internal/store/sqlc/
git commit -m "$(printf 'feat(p3.1): class rename/regenerate-code + remove-student endpoints\n\nCo-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>')"
```

---

### Task 10: Admin bulk import

**Files:**
- Modify: `apps/api/internal/store/queries/org.sql` (+ regenerate sqlc)
- Create: `apps/api/internal/api/import.go`
- Modify: `apps/api/internal/api/api.go` (wire route)
- Test: `apps/api/internal/api/import_test.go`

**Interfaces:**
- Produces: `POST /api/v1/admin/import` body `{rows:[{class, teacher_email?, student_email?}]}` → `201 {classes:[{name, join_code}], teacher_invites:[{email, code}]}`. Idempotent per (school, class name). One tx; row-indexed validation error. sqlc: `GetClassBySchoolAndName`.
- Consumes: `CreateClass`, `CreateTeacherInvite`, `org` code gens, `RequireRole("admin")`.

- [ ] **Step 1: Add the lookup query**

Append to `org.sql`:
```sql
-- name: GetClassBySchoolAndName :one
SELECT * FROM classes WHERE school_id = $1 AND name = $2;
```

- [ ] **Step 2: Regenerate sqlc**

Run: `cd apps/api && make sqlc && go build ./...`

- [ ] **Step 3: Write the failing test**

`apps/api/internal/api/import_test.go`:
```go
package api_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	. "mindimprint/api/internal/api"
)

func importBody(rows []map[string]string) *bytes.Reader {
	b, _ := json.Marshal(map[string]any{"rows": rows})
	return bytes.NewReader(b)
}

func TestAdminImportCreatesStructureIdempotently(t *testing.T) {
	pool := newAPITestPool(t)
	h := New(DepsForTest(pool)).Handler()
	admin := signInAdmin(t, pool)

	rows := []map[string]string{
		{"class": "Imported A", "teacher_email": "ta@demo.local", "student_email": "s1@demo.local"},
		{"class": "Imported A", "student_email": "s2@demo.local"},
		{"class": "Imported B", "teacher_email": "tb@demo.local"},
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("POST", "/api/v1/admin/import", importBody(rows)), admin))
	if rec.Code != http.StatusCreated {
		t.Fatalf("import got %d body=%s", rec.Code, rec.Body)
	}
	var resp struct {
		Classes        []struct{ Name, JoinCode string } `json:"classes"`
		TeacherInvites []struct{ Email, Code string }    `json:"teacher_invites"`
	}
	json.Unmarshal(rec.Body.Bytes(), &resp)
	if len(resp.Classes) != 2 || len(resp.TeacherInvites) != 2 {
		t.Fatalf("want 2 classes + 2 invites, got %d/%d", len(resp.Classes), len(resp.TeacherInvites))
	}

	// Re-import the same rows → no duplicate classes (idempotent).
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("POST", "/api/v1/admin/import", importBody(rows)), admin))
	if rec.Code != http.StatusCreated {
		t.Fatalf("re-import got %d", rec.Code)
	}
	var count int
	pool.QueryRow(t.Context(), `SELECT count(*) FROM classes WHERE name LIKE 'Imported %'`).Scan(&count)
	if count != 2 {
		t.Fatalf("idempotency broken: %d classes", count)
	}
}

func TestAdminImportRejectsEmptyClass(t *testing.T) {
	pool := newAPITestPool(t)
	h := New(DepsForTest(pool)).Handler()
	rows := []map[string]string{{"class": "", "student_email": "x@demo.local"}}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("POST", "/api/v1/admin/import", importBody(rows)), signInAdmin(t, pool)))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("empty class got %d, want 400", rec.Code)
	}
	// Nothing was created (rolled back).
	var count int
	pool.QueryRow(t.Context(), `SELECT count(*) FROM classes WHERE name='' OR name IS NULL`).Scan(&count)
	if count != 0 {
		t.Fatal("rollback failed")
	}
}
```
Implementer note: `t.Context()` exists in Go 1.24+; if the toolchain predates it use `context.Background()`.

- [ ] **Step 4: Run the test to verify it fails**

Run: `cd apps/api && DOCKER_HOST=unix:///var/run/docker.sock go test -p 1 -run 'TestAdminImport' ./internal/api/`
Expected: FAIL.

- [ ] **Step 5: Write the handler**

`apps/api/internal/api/import.go`:
```go
package api

import (
	"errors"
	"net/http"
	"strings"

	"github.com/jackc/pgx/v5"

	"mindimprint/api/internal/httpx"
	"mindimprint/api/internal/org"
	"mindimprint/api/internal/store/sqlc"
)

type importRow struct {
	Class        string `json:"class"`
	TeacherEmail string `json:"teacher_email"`
	StudentEmail string `json:"student_email"`
}

// adminImport provisions org structure from parsed rows in one transaction.
// Idempotent per (school, class name): an existing class is reused, not recreated.
// A distinct teacher_email yields one invite per class it appears with.
func (a *API) adminImport(w http.ResponseWriter, r *http.Request) {
	u, _ := UserFromContext(r.Context())
	var body struct {
		Rows []importRow `json:"rows"`
	}
	if err := decodeJSON(r, &body); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	for i, row := range body.Rows {
		if strings.TrimSpace(row.Class) == "" {
			httpx.WriteError(w, r, httpx.ErrBadRequest("validation_failed", "班级名称不能为空", map[string]any{"row": i}))
			return
		}
	}

	tx, err := a.d.Pool.Begin(r.Context())
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	defer func() { _ = tx.Rollback(r.Context()) }()
	qtx := a.d.Queries.WithTx(tx)

	classByName := map[string]sqlc.Class{}
	invitedTeachers := map[string]string{} // email → code, deduped across rows
	type classOut struct{ Name, JoinCode string }
	var classes []classOut

	for i, row := range body.Rows {
		name := strings.TrimSpace(row.Class)
		cls, ok := classByName[name]
		if !ok {
			// Reuse an existing class with this (school, name), else create one.
			existing, gerr := qtx.GetClassBySchoolAndName(r.Context(), sqlc.GetClassBySchoolAndNameParams{SchoolID: u.SchoolID, Name: name})
			switch {
			case gerr == nil:
				cls = existing
			case errors.Is(gerr, pgx.ErrNoRows):
				code, cerr := org.NewClassJoinCode()
				if cerr != nil {
					httpx.WriteError(w, r, cerr)
					return
				}
				cls, cerr = qtx.CreateClass(r.Context(), sqlc.CreateClassParams{
					SchoolID: u.SchoolID, Name: name, JoinCode: code,
					CreatedBy: pgtype.UUID{Bytes: u.ID, Valid: true},
				})
				if cerr != nil {
					httpx.WriteError(w, r, cerr)
					return
				}
			default:
				httpx.WriteError(w, r, gerr)
				return
			}
			classByName[name] = cls
			classes = append(classes, classOut{Name: cls.Name, JoinCode: cls.JoinCode})
		}

		if te := strings.TrimSpace(strings.ToLower(row.TeacherEmail)); te != "" {
			if _, done := invitedTeachers[te]; !done {
				code, cerr := org.NewTeacherInviteCode()
				if cerr != nil {
					httpx.WriteError(w, r, cerr)
					return
				}
				email := te
				if _, ierr := qtx.CreateTeacherInvite(r.Context(), sqlc.CreateTeacherInviteParams{
					SchoolID: u.SchoolID, Code: code, Email: &email, CreatedBy: u.ID,
					ExpiresAt: nowPlusDays(defaultInviteTTLDays),
				}); ierr != nil {
					httpx.WriteError(w, r, ierr)
					return
				}
				invitedTeachers[te] = code
			}
		}
		_ = i // row index reserved for richer per-row errors later
	}
	if err := tx.Commit(r.Context()); err != nil {
		httpx.WriteError(w, r, err)
		return
	}

	invites := make([]map[string]string, 0, len(invitedTeachers))
	for email, code := range invitedTeachers {
		invites = append(invites, map[string]string{"email": email, "code": code})
	}
	classesOut := make([]map[string]string, 0, len(classes))
	for _, c := range classes {
		classesOut = append(classesOut, map[string]string{"name": c.Name, "join_code": c.JoinCode})
	}
	httpx.WriteJSON(w, http.StatusCreated, map[string]any{"classes": classesOut, "teacher_invites": invites})
}
```
Implementer note: add `func nowPlusDays(d int) time.Time { return time.Now().Add(time.Duration(d) * 24 * time.Hour) }` to `dto.go` (import `time` there if needed). `CreateTeacherInviteParams.ExpiresAt` is `time.Time` (NOT NULL). Import `"github.com/jackc/pgx/v5/pgtype"` for the `CreatedBy: pgtype.UUID{...}` nullable conversion. Student rows intentionally create no artifact (locked decision) — they only drive which classes need codes; the class join code is the binding mechanism.

- [ ] **Step 6: Wire the route**

```go
	mux.Handle("POST /api/v1/admin/import", adminOnly(a.adminImport))
```

- [ ] **Step 7: Run the tests to verify they pass**

Run: `cd apps/api && DOCKER_HOST=unix:///var/run/docker.sock go test -p 1 -run 'TestAdminImport' ./internal/api/`
Expected: PASS.

- [ ] **Step 8: Commit**

```bash
git add apps/api/internal/api/import.go apps/api/internal/api/import_test.go apps/api/internal/api/api.go apps/api/internal/api/dto.go apps/api/internal/store/queries/org.sql apps/api/internal/store/sqlc/
git commit -m "$(printf 'feat(p3.1): admin bulk import (classes + teacher invites, idempotent)\n\nCo-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>')"
```

---

### Task 11: Admin school overview (aggregation)

**Files:**
- Modify: `apps/api/internal/store/queries/org.sql` (+ regenerate sqlc)
- Create: `apps/api/internal/api/overview.go`
- Modify: `apps/api/internal/api/api.go` (wire route)
- Test: `apps/api/internal/api/overview_test.go`

**Interfaces:**
- Produces: `GET /api/v1/admin/overview` → `200 {counts:{student,teacher,class,task,evaluation,active_student}, usage_by_tier:[{tier, prompt_tokens, completion_tokens, cost}]}`. School-scoped to the admin. sqlc: `GetSchoolCounts`, `GetSchoolUsageByTier`.

- [ ] **Step 1: Add the aggregation queries**

Append to `org.sql`:
```sql
-- name: GetSchoolCounts :one
SELECT
  (SELECT count(*) FROM users WHERE school_id = $1 AND role = 'student')   AS student_count,
  (SELECT count(*) FROM users WHERE school_id = $1 AND role = 'teacher')   AS teacher_count,
  (SELECT count(*) FROM classes WHERE school_id = $1)                      AS class_count,
  (SELECT count(*) FROM tasks t JOIN users u ON u.id = t.user_id WHERE u.school_id = $1) AS task_count,
  (SELECT count(*) FROM evaluations e JOIN tasks t ON t.id = e.task_id JOIN users u ON u.id = t.user_id WHERE u.school_id = $1) AS evaluation_count,
  (SELECT count(DISTINCT t.user_id) FROM tasks t JOIN users u ON u.id = t.user_id WHERE u.school_id = $1) AS active_student_count;

-- name: GetSchoolUsageByTier :many
SELECT COALESCE(tier, 'unknown') AS tier,
       COALESCE(SUM(prompt_tokens), 0)::bigint     AS prompt_tokens,
       COALESCE(SUM(completion_tokens), 0)::bigint AS completion_tokens,
       COALESCE(SUM(cost_estimate), 0)::numeric    AS cost
FROM llm_usage
WHERE school_id = $1
GROUP BY tier
ORDER BY tier;
```

- [ ] **Step 2: Regenerate sqlc**

Run: `cd apps/api && make sqlc && go build ./...`
Expected: `GetSchoolCounts` (one row of int64s), `GetSchoolUsageByTier` (rows with `Cost` typed as `pgtype.Numeric` or string — inspect).

- [ ] **Step 3: Write the failing test**

`apps/api/internal/api/overview_test.go`:
```go
package api_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	. "mindimprint/api/internal/api"
)

func TestAdminOverviewCountsScopedToSchool(t *testing.T) {
	pool := newAPITestPool(t)
	h := New(DepsForTest(pool)).Handler()
	// Seed school already has 1 student (Phoebe), 1 admin, 1 class.
	seedTaskFor(t, pool, SeedUserID) // 1 task, 1 active student

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("GET", "/api/v1/admin/overview", nil), signInAdmin(t, pool)))
	if rec.Code != http.StatusOK {
		t.Fatalf("overview got %d body=%s", rec.Code, rec.Body)
	}
	var resp struct {
		Counts struct {
			Student       int `json:"student"`
			Class         int `json:"class"`
			Task          int `json:"task"`
			ActiveStudent int `json:"active_student"`
		} `json:"counts"`
	}
	json.Unmarshal(rec.Body.Bytes(), &resp)
	if resp.Counts.Student < 1 || resp.Counts.Class < 1 || resp.Counts.Task < 1 || resp.Counts.ActiveStudent < 1 {
		t.Fatalf("counts wrong: %+v", resp.Counts)
	}
}

func TestOverviewForbiddenToTeacher(t *testing.T) {
	pool := newAPITestPool(t)
	h := New(DepsForTest(pool)).Handler()
	teacher := signInAs(t, pool, createTeacher(t, pool, SeedSchoolID, "ov@demo.local"))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("GET", "/api/v1/admin/overview", nil), teacher))
	if rec.Code != http.StatusForbidden {
		t.Fatalf("teacher got %d, want 403", rec.Code)
	}
}
```

- [ ] **Step 4: Run the test to verify it fails**

Run: `cd apps/api && DOCKER_HOST=unix:///var/run/docker.sock go test -p 1 -run 'TestAdminOverview|TestOverviewForbidden' ./internal/api/`
Expected: FAIL.

- [ ] **Step 5: Write the handler**

`apps/api/internal/api/overview.go`:
```go
package api

import (
	"net/http"

	"mindimprint/api/internal/httpx"
)

func (a *API) adminOverview(w http.ResponseWriter, r *http.Request) {
	u, _ := UserFromContext(r.Context())
	c, err := a.d.Queries.GetSchoolCounts(r.Context(), u.SchoolID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	rows, err := a.d.Queries.GetSchoolUsageByTier(r.Context(), u.SchoolID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	usage := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		usage = append(usage, map[string]any{
			"tier":              row.Tier,
			"prompt_tokens":     row.PromptTokens,
			"completion_tokens": row.CompletionTokens,
			"cost":              numericString(row.Cost),
		})
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{
		"counts": map[string]any{
			"student":        c.StudentCount,
			"teacher":        c.TeacherCount,
			"class":          c.ClassCount,
			"task":           c.TaskCount,
			"evaluation":     c.EvaluationCount,
			"active_student": c.ActiveStudentCount,
		},
		"usage_by_tier": usage,
	})
}
```
Implementer note: add `func numericString(n pgtype.Numeric) string` to `dto.go` that renders the cost — if `n.Valid`, use `v, _ := n.Value(); return fmt.Sprint(v)`, else `"0"`. If sqlc typed `Cost` as `string` already, drop the helper and use `row.Cost` directly. Match the generated field types for the count fields (likely `int64`).

- [ ] **Step 6: Wire the route**

```go
	mux.Handle("GET /api/v1/admin/overview", adminOnly(a.adminOverview))
```

- [ ] **Step 7: Run the tests to verify they pass**

Run: `cd apps/api && DOCKER_HOST=unix:///var/run/docker.sock go test -p 1 -run 'TestAdminOverview|TestOverviewForbidden' ./internal/api/`
Expected: PASS.

- [ ] **Step 8: Commit**

```bash
git add apps/api/internal/api/overview.go apps/api/internal/api/overview_test.go apps/api/internal/api/api.go apps/api/internal/api/dto.go apps/api/internal/store/queries/org.sql apps/api/internal/store/sqlc/
git commit -m "$(printf 'feat(p3.1): admin school overview (counts + usage-by-tier, school-scoped)\n\nCo-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>')"
```

---

### Task 12: Org end-to-end test + carry-forward doc

**Files:**
- Create: `apps/api/internal/api/org_e2e_test.go`
- Modify: `docs/遗留项追踪_Carryforward.md`

**Interfaces:**
- Consumes: every endpoint built above, exercised as a vertical: admin mints invite → teacher signs up → teacher creates class → student signs up with the class join code → teacher sees the student on the roster → admin overview reflects it.

- [ ] **Step 1: Write the end-to-end test**

`apps/api/internal/api/org_e2e_test.go`:
```go
package api_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	. "mindimprint/api/internal/api"
)

// TestE2EOrgProvisioning walks the full P3.1 provisioning vertical.
func TestE2EOrgProvisioning(t *testing.T) {
	pool := newAPITestPool(t)
	h := New(DepsForTest(pool)).Handler()
	admin := signInAdmin(t, pool)

	post := func(path string, cookie *http.Cookie, payload any) *httptest.ResponseRecorder {
		var rdr *bytes.Reader
		if s, ok := payload.(string); ok {
			rdr = bytes.NewReader([]byte(s))
		} else {
			b, _ := json.Marshal(payload)
			rdr = bytes.NewReader(b)
		}
		rec := httptest.NewRecorder()
		req := httptest.NewRequest("POST", path, rdr)
		if cookie != nil {
			req.AddCookie(cookie)
		}
		h.ServeHTTP(rec, req)
		return rec
	}

	// 1. Admin mints a teacher invite.
	rec := post("/api/v1/admin/teacher-invites", admin, map[string]any{"email": "e2eteacher@demo.local"})
	if rec.Code != http.StatusCreated {
		t.Fatalf("mint invite: %d %s", rec.Code, rec.Body)
	}
	var inv struct{ Code string `json:"code"` }
	json.Unmarshal(rec.Body.Bytes(), &inv)

	// 2. Teacher signs up with it.
	rec = post("/api/v1/auth/signup", nil, map[string]any{
		"email": "e2eteacher@demo.local", "password": "password123",
		"display_name": "E2E Teacher", "join_code": inv.Code,
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("teacher signup: %d %s", rec.Code, rec.Body)
	}
	teacherCookie := signInViaAPI(t, h, "e2eteacher@demo.local", "password123")

	// 3. Teacher creates a class.
	rec = post("/api/v1/classes", teacherCookie, map[string]any{"name": "E2E Class"})
	if rec.Code != http.StatusCreated {
		t.Fatalf("create class: %d %s", rec.Code, rec.Body)
	}
	var cc struct {
		Class struct{ ID, JoinCode string } `json:"class"`
	}
	json.Unmarshal(rec.Body.Bytes(), &cc)

	// 4. A student signs up with the class join code.
	rec = post("/api/v1/auth/signup", nil, map[string]any{
		"email": "e2estudent@demo.local", "password": "password123",
		"display_name": "E2E Student", "join_code": cc.Class.JoinCode,
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("student signup: %d %s", rec.Code, rec.Body)
	}

	// 5. Teacher sees the student on the roster.
	rec = httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/v1/classes/"+cc.Class.ID, nil)
	req.AddCookie(teacherCookie)
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "e2estudent@demo.local") {
		t.Fatalf("roster missing student: %d %s", rec.Code, rec.Body)
	}

	// 6. Admin overview reflects the new class.
	rec = httptest.NewRecorder()
	req = httptest.NewRequest("GET", "/api/v1/admin/overview", nil)
	req.AddCookie(admin)
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("overview: %d %s", rec.Code, rec.Body)
	}
}
```
Implementer note: add `signInViaAPI(t, h, email, password) *http.Cookie` to `maintest_test.go` — POST `/api/v1/auth/signin`, then extract the `mk_session` cookie from the response's `Set-Cookie` (`rec.Result().Cookies()`). This exercises the real signin path rather than minting a session directly.

- [ ] **Step 2: Run the test to verify it passes**

Run: `cd apps/api && DOCKER_HOST=unix:///var/run/docker.sock go test -p 1 -run TestE2EOrg ./internal/api/`
Expected: PASS.

- [ ] **Step 3: Run the FULL gate**

Run:
```bash
cd apps/api && DOCKER_HOST=unix:///var/run/docker.sock go vet ./... && DOCKER_HOST=unix:///var/run/docker.sock go test -p 1 ./...
```
Expected: all packages PASS.

- [ ] **Step 4: Append the carry-forward section**

Append a `## P3.1 · Org backend + RBAC` section to `docs/遗留项追踪_Carryforward.md` documenting the deferred items: class soft-archive; per-student work-detail visibility (process tree / transcript / eval narrative — waits on the eval-model brainstorm); CSV *parsing* lives in P3.2; teacher-invite email enforcement is advisory; rate-limiting + CSRF still deferred from P2; teacher/admin frontend consoles = P3.2 / P3.3. Mirror the format of the existing P2 section.

- [ ] **Step 5: Commit**

```bash
git add apps/api/internal/api/org_e2e_test.go apps/api/internal/api/maintest_test.go docs/遗留项追踪_Carryforward.md
git commit -m "$(printf 'test(p3.1): org provisioning e2e + carry-forward doc\n\nCo-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>')"
```

---

## Notes for the executor

- **sqlc field types are the #1 integration risk.** After every `make sqlc`, read the regenerated params/row structs and match the handler/DTO code to the actual generated types (`pgtype.UUID` vs `uuid.UUID`, `pgtype.Timestamptz` vs `time.Time`, `int64` counts, `pgtype.Numeric` cost). The plan flags each spot; trust the generated code over the snippet when they disagree on a field type.
- **Existing tests must stay green.** Tasks 4–12 only add routes/queries; the only existing file with behavior changes is `signup.go` (Task 6), whose existing student test must still pass.
- **`export_test.go` grows across tasks.** Add helpers as each task needs them; keep it in package `api` (not `api_test`).
- The repo-root `package.json` carries a pre-existing unrelated change — never stage it.
