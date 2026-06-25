# P2 · Auth + Minimal Org Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the dev `ActAsSeed` shim with real DB-backed cookie-session auth — signup via class join code (atomic org invariant), signin/signout, `GET /auth/me`, and a built-but-dormant email-verify flow — and wire the existing `AuthScreen`.

**Architecture:** Two new tables (`sessions`, `email_verification_tokens`, hash-only). A pure `internal/auth` package (argon2id + token gen/hash). New `internal/api` handlers `signup`/`signin`/`signout`/`verify-email`/`me`. A `SessionAuth` middleware (resolves the cookie into the existing `WithUser` context) + a `RequireUser` guard on a protected route group; `ActAsSeed` deleted. The SPA gets `src/api/auth.ts` and a `getMe()` boot gate; `AuthScreen` wires to the real endpoints.

**Tech Stack:** Go 1.26 (`net/http` 1.22 routing, `pgx/v5` + `sqlc`, `goose`, `golang.org/x/crypto/argon2`, testcontainers), React + Vite + TS + vitest.

## Global Constraints

- **Client never holds LLM keys / never calls models** — unchanged; this phase adds no model calls. Secrets only in `apps/api` server env; never in git/logs/errors/data.
- **Org invariant:** every account belongs to a school; a student also to ≥1 class. Signup creates the `users` row (`school_id` from the class) **and** the `enrollments` row in **one transaction**, or the whole thing fails.
- **Tokens stored hashed only.** Session tokens and verification tokens: persist `SHA-256` hash, never the raw token. Raw token lives only in the cookie / (future) email.
- **Password hashing:** argon2id, params `m=65536` (64 MiB), `t=1`, `p=4`, 16-byte salt, 32-byte key; encoded as the standard PHC string `$argon2id$v=19$m=65536,t=1,p=4$<b64salt>$<b64hash>` (base64 `RawStdEncoding`).
- **Cookie:** name `mk_session`, `HttpOnly`, `SameSite=Lax`, `Path=/`, `MaxAge` 30 days; `Secure` gated by config (`false` for local http dev). No session signing secret (opaque DB token).
- **Auto-verify in P2:** signup sets `email_verified_at = now()` and sends no email. The verify endpoint + token table are built and Go-tested but dormant.
- **Deferred (carry-forward, NOT this phase):** Mailer/real email, CSRF double-submit token, rate limiter, teacher/admin self-service, async eval.
- **Never stage/commit the repo-root `package.json`** (pre-existing unrelated change). Use explicit `git add` paths.
- **Go test runs:** `go test -p 1 ./...` (parallel testcontainers exhaust Docker). Testcontainers env at run time only (never committed): `DOCKER_HOST=unix:///var/run/docker.sock` (+ `TESTCONTAINERS_RYUK_DISABLED=true` if Ryuk fails).
- **sqlc regen on macOS:** `make sqlc` (it sets `CGO_ENABLED=0`). Commit the generated files under `internal/store/sqlc/`.
- Commit trailer: `Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>`.
- The project-boundary hook blocks the bare 4-letter shell builtin e-v-a-l as a standalone word and redirects outside the repo; paths/words containing "eval"/"evaluation" are fine.

**Branch:** `feat/p2-auth-minimal-org` off `main`.

---

## File structure

**Backend (`apps/api`):**
- Create `internal/store/migrations/0003_auth.sql` — sessions + email_verification_tokens.
- Create `internal/store/migrations/0004_seed_password.sql` — real argon2id hash for seed Phoebe.
- Create `internal/store/queries/auth.sql` — all auth queries (sqlc input).
- Regen `internal/store/sqlc/{models.go, auth.sql.go}` via `make sqlc`.
- Create `internal/auth/{password.go, token.go, password_test.go, token_test.go}`.
- Create `tools/genhash/main.go` — dev tool: print a PHC hash for a password.
- Create `internal/api/{signup.go, signin.go, signout.go, verify.go, me.go, cookie.go}` + per-handler tests.
- Modify `internal/api/api.go` (routes + middleware), `internal/api/auth.go` (delete `ActAsSeed`, add `SessionAuth`/`RequireUser`/`TxBeginner`), `internal/api/dto.go` (add `meUserDTO`), `internal/httpx/errors.go` (5 codes), `internal/config/config.go` (+`CookieSecure`, +its test), `cmd/api/main.go` (pass pool + CookieSecure).
- Modify test harness `internal/api/maintest_test.go` (+pool helper, +`signInSeed`) and every existing handler test (attach the session cookie).

**Frontend (`apps/web`):**
- Create `src/api/auth.ts` + `src/api/auth.test.ts`.
- Modify `src/api/index.ts` (extend `ApiClient` + `api`).
- Modify `src/shell/auth/AuthScreen.tsx` + its test (wire real calls).
- Modify `src/shell/session.ts` (+`user`) + `src/shell/AppShell.tsx` (boot gate) + `src/shell/settings/SettingsView.tsx` (real profile + logout) + their tests.

---

### Task 1: Migration 0003 — sessions + email_verification_tokens

**Files:**
- Create: `apps/api/internal/store/migrations/0003_auth.sql`
- Modify (regen): `apps/api/internal/store/sqlc/models.go`
- Test: `apps/api/internal/store/migrate_test.go` (extend `wantTables`)

**Interfaces:**
- Produces (after `make sqlc`): `sqlc.Session{ID uuid.UUID; UserID uuid.UUID; TokenHash string; ExpiresAt time.Time; UserAgent *string; Ip *string; CreatedAt time.Time}` and `sqlc.EmailVerificationToken{ID uuid.UUID; UserID uuid.UUID; TokenHash string; ExpiresAt time.Time; ConsumedAt pgtype.Timestamptz; CreatedAt time.Time}`. (`ip`/`user_agent` are nullable `text`.)

- [ ] **Step 1: Write the migration**

Create `apps/api/internal/store/migrations/0003_auth.sql`:

```sql
-- +goose Up
CREATE TABLE sessions (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash  text NOT NULL,
    expires_at  timestamptz NOT NULL,
    user_agent  text,
    ip          text,
    created_at  timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX sessions_token_hash_key ON sessions (token_hash);
CREATE INDEX sessions_user_id_idx ON sessions (user_id);

CREATE TABLE email_verification_tokens (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash  text NOT NULL,
    expires_at  timestamptz NOT NULL,
    consumed_at timestamptz,
    created_at  timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX evt_token_hash_key ON email_verification_tokens (token_hash);
CREATE INDEX evt_user_id_idx ON email_verification_tokens (user_id);

-- +goose Down
DROP TABLE IF EXISTS email_verification_tokens;
DROP TABLE IF EXISTS sessions;
```

- [ ] **Step 2: Extend the migration test**

In `apps/api/internal/store/migrate_test.go`, add the two tables to `wantTables`:

```go
	wantTables := []string{
		"schools", "classes", "users", "enrollments",
		"tasks", "messages", "card_instances", "evaluations",
		"sessions", "email_verification_tokens",
	}
```

- [ ] **Step 3: Regenerate sqlc models**

Run: `cd apps/api && make sqlc`
Expected: `internal/store/sqlc/models.go` now contains `Session` and `EmailVerificationToken` structs (sqlc emits a model per schema table even without queries). No `auth.sql.go` yet (no queries).

- [ ] **Step 4: Run the migration test**

Run: `cd apps/api && DOCKER_HOST=unix:///var/run/docker.sock go test -p 1 ./internal/store/ -run TestMigrationsCreateTablesAndSeed -v`
Expected: PASS (both new tables exist).

- [ ] **Step 5: Commit**

```bash
git add apps/api/internal/store/migrations/0003_auth.sql apps/api/internal/store/migrate_test.go apps/api/internal/store/sqlc/models.go
git commit -m "feat(p2): sessions + email_verification_tokens migration

Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>"
```

---

### Task 2: `internal/auth` package — argon2id + tokens

**Files:**
- Create: `apps/api/internal/auth/password.go`, `apps/api/internal/auth/token.go`
- Create: `apps/api/internal/auth/password_test.go`, `apps/api/internal/auth/token_test.go`
- Create: `apps/api/tools/genhash/main.go`
- Modify: `apps/api/go.mod` / `go.sum` (via `go get golang.org/x/crypto/argon2`)

**Interfaces:**
- Produces:
  - `auth.HashPassword(plain string) (string, error)` — PHC string.
  - `auth.VerifyPassword(plain, phc string) (bool, error)` — false on mismatch, error on malformed PHC.
  - `auth.NewToken() (raw string, hash string, err error)` — `raw` base64url (no pad) of 32 random bytes; `hash` = SHA-256 hex of `raw`.
  - `auth.HashToken(raw string) string` — SHA-256 hex.

- [ ] **Step 1: Add the dependency**

Run: `cd apps/api && go get golang.org/x/crypto/argon2`
Expected: `golang.org/x/crypto` appears in `go.mod` require block.

- [ ] **Step 2: Write the password test**

Create `apps/api/internal/auth/password_test.go`:

```go
package auth

import (
	"strings"
	"testing"
)

func TestHashAndVerifyPassword(t *testing.T) {
	phc, err := HashPassword("phoebe-dev-pass")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	if !strings.HasPrefix(phc, "$argon2id$v=19$m=65536,t=1,p=4$") {
		t.Fatalf("unexpected PHC prefix: %q", phc)
	}
	ok, err := VerifyPassword("phoebe-dev-pass", phc)
	if err != nil || !ok {
		t.Fatalf("verify correct: ok=%v err=%v", ok, err)
	}
	bad, err := VerifyPassword("wrong-password", phc)
	if err != nil {
		t.Fatalf("verify wrong errored: %v", err)
	}
	if bad {
		t.Fatal("verify wrong returned true")
	}
}

func TestHashPasswordSaltIsRandom(t *testing.T) {
	a, _ := HashPassword("same")
	b, _ := HashPassword("same")
	if a == b {
		t.Fatal("two hashes of the same password are identical — salt not random")
	}
}

func TestVerifyPasswordMalformed(t *testing.T) {
	if _, err := VerifyPassword("x", "not-a-phc-string"); err == nil {
		t.Fatal("malformed PHC should error, not panic")
	}
}
```

- [ ] **Step 3: Run it (fails — no impl)**

Run: `cd apps/api && go test ./internal/auth/`
Expected: build/compile failure (`HashPassword` undefined).

- [ ] **Step 4: Implement password.go**

Create `apps/api/internal/auth/password.go`:

```go
// Package auth holds password hashing (argon2id) and opaque token generation.
// It is pure (no DB, no HTTP) so it is fast to unit-test and reusable by the
// dev genhash tool and the seed migration.
package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"golang.org/x/crypto/argon2"
)

const (
	argonMemory  = 64 * 1024 // 64 MiB
	argonTime    = 1
	argonThreads = 4
	argonKeyLen  = 32
	argonSaltLen = 16
	argonVersion = argon2.Version // 19
)

// HashPassword returns the standard argon2id PHC string for plain.
func HashPassword(plain string) (string, error) {
	salt := make([]byte, argonSaltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	key := argon2.IDKey([]byte(plain), salt, argonTime, argonMemory, argonThreads, argonKeyLen)
	b64 := base64.RawStdEncoding
	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argonVersion, argonMemory, argonTime, argonThreads,
		b64.EncodeToString(salt), b64.EncodeToString(key)), nil
}

// VerifyPassword reports whether plain matches the PHC-encoded hash. A malformed
// PHC string is an error (not a silent false).
func VerifyPassword(plain, phc string) (bool, error) {
	parts := strings.Split(phc, "$")
	// ["", "argon2id", "v=19", "m=65536,t=1,p=4", "<salt>", "<hash>"]
	if len(parts) != 6 || parts[1] != "argon2id" {
		return false, errors.New("auth: malformed PHC hash")
	}
	var version int
	if _, err := fmt.Sscanf(parts[2], "v=%d", &version); err != nil {
		return false, errors.New("auth: malformed PHC version")
	}
	var mem uint32
	var t, p uint32
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &mem, &t, &p); err != nil {
		return false, errors.New("auth: malformed PHC params")
	}
	b64 := base64.RawStdEncoding
	salt, err := b64.DecodeString(parts[4])
	if err != nil {
		return false, errors.New("auth: malformed PHC salt")
	}
	want, err := b64.DecodeString(parts[5])
	if err != nil {
		return false, errors.New("auth: malformed PHC hash body")
	}
	got := argon2.IDKey([]byte(plain), salt, t, mem, uint8(p), uint32(len(want)))
	return subtle.ConstantTimeCompare(got, want) == 1, nil
}
```

- [ ] **Step 5: Write the token test**

Create `apps/api/internal/auth/token_test.go`:

```go
package auth

import "testing"

func TestNewTokenRawDiffersFromHashAndIsDeterministic(t *testing.T) {
	raw, hash, err := NewToken()
	if err != nil {
		t.Fatalf("NewToken: %v", err)
	}
	if raw == "" || hash == "" || raw == hash {
		t.Fatalf("raw=%q hash=%q", raw, hash)
	}
	if HashToken(raw) != hash {
		t.Fatal("HashToken(raw) must equal NewToken's hash")
	}
	raw2, _, _ := NewToken()
	if raw2 == raw {
		t.Fatal("two tokens collided — not random")
	}
}
```

- [ ] **Step 6: Implement token.go**

Create `apps/api/internal/auth/token.go`:

```go
package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
)

// NewToken mints a 32-byte opaque token. raw (base64url, no padding) is what
// goes in the cookie / email; hash (SHA-256 hex) is what gets persisted.
func NewToken() (raw string, hash string, err error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", "", err
	}
	raw = base64.RawURLEncoding.EncodeToString(b)
	return raw, HashToken(raw), nil
}

// HashToken returns the SHA-256 hex digest used for DB lookups.
func HashToken(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}
```

- [ ] **Step 7: Run the auth tests**

Run: `cd apps/api && go test ./internal/auth/ -v`
Expected: all PASS.

- [ ] **Step 8: Write the genhash dev tool**

Create `apps/api/tools/genhash/main.go`:

```go
// Command genhash prints an argon2id PHC hash for a password, for pasting into
// the seed migration. Usage: go run ./tools/genhash <password>
package main

import (
	"fmt"
	"os"

	"mindimprint/api/internal/auth"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: genhash <password>")
		os.Exit(2)
	}
	phc, err := auth.HashPassword(os.Args[1])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Println(phc)
}
```

- [ ] **Step 9: Verify the tool builds and runs**

Run: `cd apps/api && go run ./tools/genhash phoebe-dev-pass`
Expected: one line starting `$argon2id$v=19$m=65536,t=1,p=4$`.

- [ ] **Step 10: Commit**

```bash
git add apps/api/internal/auth/ apps/api/tools/genhash/ apps/api/go.mod apps/api/go.sum
git commit -m "feat(p2): auth package (argon2id + opaque tokens) + genhash tool

Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>"
```

---

### Task 3: Auth store queries

**Files:**
- Create: `apps/api/internal/store/queries/auth.sql`
- Regen: `apps/api/internal/store/sqlc/auth.sql.go` (+ row structs)
- Test: `apps/api/internal/store/auth_query_test.go`

**Interfaces:**
- Produces these `*sqlc.Queries` methods (names fixed by the `-- name:` annotations):
  - `GetUserByEmail(ctx, email string) (User, error)`
  - `GetClassByJoinCode(ctx, joinCode string) (Class, error)`
  - `CreateUser(ctx, CreateUserParams) (User, error)` where params = `{Email, PasswordHash, Role, SchoolID uuid.UUID, DisplayName, AvatarColor string, EmailVerifiedAt pgtype.Timestamptz}`
  - `CreateEnrollment(ctx, CreateEnrollmentParams) (Enrollment, error)` params `{UserID, ClassID uuid.UUID, RoleInClass string}`
  - `MarkEmailVerified(ctx, id uuid.UUID) (User, error)`
  - `CreateEmailVerificationToken(ctx, CreateEmailVerificationTokenParams) (EmailVerificationToken, error)` params `{UserID uuid.UUID, TokenHash string, ExpiresAt time.Time}`
  - `GetActiveEmailVerificationToken(ctx, tokenHash string) (EmailVerificationToken, error)` (filters unconsumed + unexpired)
  - `ConsumeEmailVerificationToken(ctx, id uuid.UUID) error`
  - `CreateSession(ctx, CreateSessionParams) (Session, error)` params `{UserID uuid.UUID, TokenHash string, ExpiresAt time.Time, UserAgent *string, Ip *string}`
  - `GetSessionWithUserByHash(ctx, tokenHash string) (GetSessionWithUserByHashRow, error)` row `{ID, SchoolID uuid.UUID, Role, DisplayName string}` (filters unexpired)
  - `DeleteSessionByHash(ctx, tokenHash string) error`
  - `GetSchool(ctx, id uuid.UUID) (School, error)`
  - `ListClassesForUser(ctx, userID uuid.UUID) ([]ListClassesForUserRow, error)` row `{ID uuid.UUID; Name string; RoleInClass string}`

- [ ] **Step 1: Write the queries file**

Create `apps/api/internal/store/queries/auth.sql`:

```sql
-- name: GetUserByEmail :one
SELECT * FROM users WHERE email = $1;

-- name: GetClassByJoinCode :one
SELECT * FROM classes WHERE join_code = $1;

-- name: CreateUser :one
INSERT INTO users (email, password_hash, role, school_id, display_name, avatar_color, email_verified_at)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING *;

-- name: CreateEnrollment :one
INSERT INTO enrollments (user_id, class_id, role_in_class)
VALUES ($1, $2, $3)
RETURNING *;

-- name: MarkEmailVerified :one
UPDATE users SET email_verified_at = now() WHERE id = $1 RETURNING *;

-- name: CreateEmailVerificationToken :one
INSERT INTO email_verification_tokens (user_id, token_hash, expires_at)
VALUES ($1, $2, $3)
RETURNING *;

-- name: GetActiveEmailVerificationToken :one
SELECT * FROM email_verification_tokens
WHERE token_hash = $1 AND consumed_at IS NULL AND expires_at > now();

-- name: ConsumeEmailVerificationToken :exec
UPDATE email_verification_tokens SET consumed_at = now() WHERE id = $1;

-- name: CreateSession :one
INSERT INTO sessions (user_id, token_hash, expires_at, user_agent, ip)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: GetSessionWithUserByHash :one
SELECT u.id, u.school_id, u.role, u.display_name
FROM sessions s
JOIN users u ON u.id = s.user_id
WHERE s.token_hash = $1 AND s.expires_at > now();

-- name: DeleteSessionByHash :exec
DELETE FROM sessions WHERE token_hash = $1;

-- name: GetSchool :one
SELECT * FROM schools WHERE id = $1;

-- name: ListClassesForUser :many
SELECT c.id, c.name, e.role_in_class
FROM enrollments e
JOIN classes c ON c.id = e.class_id
WHERE e.user_id = $1
ORDER BY c.name;
```

- [ ] **Step 2: Regenerate sqlc**

Run: `cd apps/api && make sqlc`
Expected: `internal/store/sqlc/auth.sql.go` created with the methods above and the `GetSessionWithUserByHashRow` / `ListClassesForUserRow` structs. `go build ./...` clean.

- [ ] **Step 3: Write the query integration test**

Create `apps/api/internal/store/auth_query_test.go` (external `store_test` package — reuses `newStoreTestPool` + `seededStudentID` from `sqlc_test.go`):

```go
package store_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"mindimprint/api/internal/store/sqlc"
)

var seededClassID = uuid.MustParse("00000000-0000-0000-0000-000000000002")

func TestAuthQueries(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping testcontainers integration in -short mode")
	}
	ctx := context.Background()
	pool := newStoreTestPool(t)
	q := sqlc.New(pool)

	// Join code resolves to the seeded class.
	cls, err := q.GetClassByJoinCode(ctx, "DEMO-0001")
	if err != nil || cls.ID != seededClassID {
		t.Fatalf("GetClassByJoinCode: %v id=%v", err, cls.ID)
	}

	// Atomic signup: create user + enrollment in a tx.
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	qtx := q.WithTx(tx)
	u, err := qtx.CreateUser(ctx, sqlc.CreateUserParams{
		Email:           "newkid@demo.local",
		PasswordHash:    "$argon2id$placeholder",
		Role:            "student",
		SchoolID:        cls.SchoolID,
		DisplayName:     "New Kid",
		AvatarColor:     "#7C9CF0",
		EmailVerifiedAt: pgtype.Timestamptz{Time: time.Now(), Valid: true},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := qtx.CreateEnrollment(ctx, sqlc.CreateEnrollmentParams{
		UserID: u.ID, ClassID: cls.ID, RoleInClass: "student",
	}); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}

	// GetUserByEmail finds it; classes list it.
	got, err := q.GetUserByEmail(ctx, "newkid@demo.local")
	if err != nil || got.ID != u.ID {
		t.Fatalf("GetUserByEmail: %v", err)
	}
	classes, err := q.ListClassesForUser(ctx, u.ID)
	if err != nil || len(classes) != 1 || classes[0].ID != cls.ID {
		t.Fatalf("ListClassesForUser: %v %+v", err, classes)
	}

	// Sessions: create, resolve, delete.
	sess, err := q.CreateSession(ctx, sqlc.CreateSessionParams{
		UserID: u.ID, TokenHash: "hash-abc", ExpiresAt: time.Now().Add(time.Hour),
	})
	if err != nil {
		t.Fatal(err)
	}
	row, err := q.GetSessionWithUserByHash(ctx, "hash-abc")
	if err != nil || row.ID != u.ID || row.SchoolID != cls.SchoolID {
		t.Fatalf("GetSessionWithUserByHash: %v %+v", err, row)
	}
	if err := q.DeleteSessionByHash(ctx, "hash-abc"); err != nil {
		t.Fatal(err)
	}
	if _, err := q.GetSessionWithUserByHash(ctx, "hash-abc"); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("after delete want ErrNoRows, got %v", err)
	}
	_ = sess

	// Expired session is not resolved.
	if _, err := q.CreateSession(ctx, sqlc.CreateSessionParams{
		UserID: u.ID, TokenHash: "hash-expired", ExpiresAt: time.Now().Add(-time.Hour),
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := q.GetSessionWithUserByHash(ctx, "hash-expired"); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("expired want ErrNoRows, got %v", err)
	}

	// Verification token: active lookup, consume, then inactive.
	evt, err := q.CreateEmailVerificationToken(ctx, sqlc.CreateEmailVerificationTokenParams{
		UserID: u.ID, TokenHash: "vt-hash", ExpiresAt: time.Now().Add(time.Hour),
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := q.GetActiveEmailVerificationToken(ctx, "vt-hash"); err != nil {
		t.Fatalf("active token: %v", err)
	}
	if err := q.ConsumeEmailVerificationToken(ctx, evt.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := q.GetActiveEmailVerificationToken(ctx, "vt-hash"); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("consumed token want ErrNoRows, got %v", err)
	}
	if _, err := q.MarkEmailVerified(ctx, u.ID); err != nil {
		t.Fatal(err)
	}

	// GetSchool resolves.
	if _, err := q.GetSchool(ctx, cls.SchoolID); err != nil {
		t.Fatalf("GetSchool: %v", err)
	}
}
```

- [ ] **Step 4: Run it**

Run: `cd apps/api && DOCKER_HOST=unix:///var/run/docker.sock go test -p 1 ./internal/store/ -run TestAuthQueries -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add apps/api/internal/store/queries/auth.sql apps/api/internal/store/sqlc/ apps/api/internal/store/auth_query_test.go
git commit -m "feat(p2): auth store queries (users/sessions/verify-tokens/classes)

Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>"
```

---

### Task 4: Migration 0004 — seed Phoebe's real password

**Files:**
- Create: `apps/api/internal/store/migrations/0004_seed_password.sql`
- Test: `apps/api/internal/store/seed_login_test.go`

**Interfaces:**
- Consumes: `auth.HashPassword` / `auth.VerifyPassword` (Task 2), `genhash` tool (Task 2), seeded user from `0002`.
- Produces: seeded Phoebe can log in with password `phoebe-dev-pass`.

- [ ] **Step 1: Generate the hash**

Run: `cd apps/api && go run ./tools/genhash phoebe-dev-pass`
Copy the printed PHC line (it contains a random salt — yours will differ from any example).

- [ ] **Step 2: Write the migration with the generated hash**

Create `apps/api/internal/store/migrations/0004_seed_password.sql`, pasting the hash from Step 1 in place of `<PASTE_PHC_HASH_HERE>`:

```sql
-- +goose Up
-- Dev login for the seeded student: email phoebe@demo.mindimprint.local,
-- password "phoebe-dev-pass". The hash below was generated by
-- `go run ./tools/genhash phoebe-dev-pass`; regenerate with that command if the
-- argon2id params change. seed_login_test.go asserts this hash verifies.
UPDATE users
SET password_hash = '<PASTE_PHC_HASH_HERE>'
WHERE id = '00000000-0000-0000-0000-000000000003';

-- +goose Down
UPDATE users
SET password_hash = 'SEED_NO_LOGIN'
WHERE id = '00000000-0000-0000-0000-000000000003';
```

- [ ] **Step 3: Write the seed-login test**

Create `apps/api/internal/store/seed_login_test.go`:

```go
package store_test

import (
	"context"
	"testing"

	"mindimprint/api/internal/auth"
	"mindimprint/api/internal/store/sqlc"
)

// Proves the pasted PHC hash in 0004_seed_password.sql actually verifies against
// the documented dev password — catches a bad paste.
func TestSeedPhoebeCanLogIn(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping testcontainers integration in -short mode")
	}
	ctx := context.Background()
	pool := newStoreTestPool(t)
	q := sqlc.New(pool)

	u, err := q.GetUserByEmail(ctx, "phoebe@demo.mindimprint.local")
	if err != nil {
		t.Fatalf("GetUserByEmail: %v", err)
	}
	ok, err := auth.VerifyPassword("phoebe-dev-pass", u.PasswordHash)
	if err != nil {
		t.Fatalf("VerifyPassword errored — hash malformed in migration: %v", err)
	}
	if !ok {
		t.Fatal("seed password does not verify — regenerate the hash in 0004")
	}
}
```

- [ ] **Step 4: Run it**

Run: `cd apps/api && DOCKER_HOST=unix:///var/run/docker.sock go test -p 1 ./internal/store/ -run TestSeedPhoebeCanLogIn -v`
Expected: PASS. If FAIL, regenerate the hash (Step 1) and re-paste — do not edit the test.

- [ ] **Step 5: Commit**

```bash
git add apps/api/internal/store/migrations/0004_seed_password.sql apps/api/internal/store/seed_login_test.go
git commit -m "feat(p2): seed real argon2id password for demo student Phoebe

Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>"
```

---

### Task 5: Signup handler + Deps/config/cookie plumbing + auth errors

**Files:**
- Create: `apps/api/internal/api/signup.go`, `apps/api/internal/api/cookie.go`
- Create: `apps/api/internal/api/signup_test.go`
- Modify: `apps/api/internal/httpx/errors.go` (5 codes), `apps/api/internal/api/api.go` (`Deps` fields + signup route), `apps/api/internal/api/auth.go` (`TxBeginner`), `apps/api/internal/config/config.go` (+`CookieSecure` + its test `config_test.go`), `apps/api/cmd/api/main.go` (pass pool + CookieSecure), `apps/api/internal/api/maintest_test.go` (+`newAPITestPool`)

**Interfaces:**
- Consumes: `auth.HashPassword` (T2); `GetClassByJoinCode`/`CreateUser`/`CreateEnrollment` (T3).
- Produces:
  - `httpx.ErrEmailTaken()` (409 `email_taken`), `httpx.ErrInvalidJoinCode()` (400 `invalid_join_code`), `httpx.ErrEmailUnverified()` (403 `email_unverified`), `httpx.ErrTokenInvalid()` (400 `token_invalid_or_expired`), `httpx.ErrInvalidCredentials()` (401 `invalid_credentials`).
  - `api.TxBeginner` interface; `Deps.Pool TxBeginner`, `Deps.CookieSecure bool`.
  - cookie helpers `setSessionCookie(w, raw, secure)`, `clearSessionCookie(w, secure)`, consts `sessionCookieName = "mk_session"`, `sessionTTL = 30 * 24 * time.Hour`.
  - test helper `newAPITestPool(t) *pgxpool.Pool`.

- [ ] **Step 1: Add the error constructors**

In `apps/api/internal/httpx/errors.go`, after `ErrConflict`, add:

```go
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
```

- [ ] **Step 2: Add CookieSecure to config**

In `apps/api/internal/config/config.go`, add the field to `Config`:

```go
	// CookieSecure sets the Secure flag on the session cookie. Default true;
	// set COOKIE_SECURE=false for local http dev so the browser sends it.
	CookieSecure bool `env:"COOKIE_SECURE" envDefault:"true"`
```

And in `apps/api/internal/config/config_test.go`, add `"COOKIE_SECURE"` to `allEnvKeys`, and in the "all set" case add `"COOKIE_SECURE": "false"` to the env map and assert it in `check`:

```go
				if c.CookieSecure {
					t.Fatalf("CookieSecure = true, want false")
				}
```

Also add to the "PORT defaults" case a check that the default is true:

```go
				// (in the defaults subtest's check)
				if !c.CookieSecure {
					t.Fatalf("CookieSecure default = false, want true")
				}
```

- [ ] **Step 3: Run config test**

Run: `cd apps/api && go test ./internal/config/ -v`
Expected: PASS.

- [ ] **Step 4: Add TxBeginner + Deps fields**

In `apps/api/internal/api/auth.go`, add the import `"github.com/jackc/pgx/v5"` and after the `User` type add:

```go
// TxBeginner is the subset of *pgxpool.Pool the signup transaction needs.
type TxBeginner interface {
	Begin(ctx context.Context) (pgx.Tx, error)
}
```

In `apps/api/internal/api/api.go`, extend `Deps`:

```go
	Pool         TxBeginner // for multi-statement transactions (signup)
	CookieSecure bool       // Secure flag on the session cookie
```

- [ ] **Step 5: Add the cookie helpers**

Create `apps/api/internal/api/cookie.go`:

```go
package api

import (
	"net/http"
	"time"
)

const (
	sessionCookieName = "mk_session"
	sessionTTL        = 30 * 24 * time.Hour
)

// setSessionCookie writes the opaque session token cookie. HttpOnly + Lax; the
// Secure flag is config-gated (off for local http dev).
func setSessionCookie(w http.ResponseWriter, raw string, secure bool) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    raw,
		Path:     "/",
		MaxAge:   int(sessionTTL.Seconds()),
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
	})
}

// clearSessionCookie expires the session cookie.
func clearSessionCookie(w http.ResponseWriter, secure bool) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
	})
}
```

- [ ] **Step 6: Write the signup test**

Create `apps/api/internal/api/signup_test.go`:

```go
package api_test

import (
	"net/http/httptest"
	"strings"
	"testing"

	. "mindimprint/api/internal/api"
	"mindimprint/api/internal/store/sqlc"
)

func TestSignup(t *testing.T) {
	pool := newAPITestPool(t)
	h := New(Deps{Queries: sqlc.New(pool), Pool: pool}).Handler()

	// Happy path: join the seeded class.
	body := `{"email":"alice@demo.local","password":"alice-pass-1","display_name":"Alice","join_code":"DEMO-0001"}`
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest("POST", "/api/v1/auth/signup", strings.NewReader(body)))
	if rr.Code != 201 {
		t.Fatalf("signup: want 201, got %d — %s", rr.Code, rr.Body.String())
	}

	// Duplicate email → 409 email_taken.
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest("POST", "/api/v1/auth/signup", strings.NewReader(body)))
	if rr.Code != 409 || !strings.Contains(rr.Body.String(), "email_taken") {
		t.Fatalf("dup email: want 409 email_taken, got %d — %s", rr.Code, rr.Body.String())
	}

	// Bad join code → 400 invalid_join_code.
	bad := `{"email":"bob@demo.local","password":"bob-pass-1","display_name":"Bob","join_code":"NOPE"}`
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest("POST", "/api/v1/auth/signup", strings.NewReader(bad)))
	if rr.Code != 400 || !strings.Contains(rr.Body.String(), "invalid_join_code") {
		t.Fatalf("bad code: want 400 invalid_join_code, got %d — %s", rr.Code, rr.Body.String())
	}

	// Short password → 400 validation_failed.
	short := `{"email":"c@demo.local","password":"x","display_name":"C","join_code":"DEMO-0001"}`
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest("POST", "/api/v1/auth/signup", strings.NewReader(short)))
	if rr.Code != 400 {
		t.Fatalf("short pw: want 400, got %d — %s", rr.Code, rr.Body.String())
	}
}
```

Delete the `newSignupHandler` stub before finalizing (it exists only to remind you the file compiles standalone; remove it):

```go
// remove newSignupHandler entirely
```

- [ ] **Step 7: Add `newAPITestPool` to the harness**

In `apps/api/internal/api/maintest_test.go`, refactor so the pool is reusable. Add the import `"github.com/jackc/pgx/v5/pgxpool"` and replace the body of `newAPITestQueries` and add `newAPITestPool`:

```go
// newAPITestPool spins up a throwaway Postgres, runs all migrations (incl. seed),
// and returns the pool. Container/pool torn down via t.Cleanup.
func newAPITestPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	if testing.Short() {
		t.Skip("skipping testcontainers integration in -short mode")
	}
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

// newAPITestQueries returns a *sqlc.Queries over a fresh test pool.
func newAPITestQueries(t *testing.T) *sqlc.Queries {
	return sqlc.New(newAPITestPool(t))
}
```

- [ ] **Step 8: Implement signup.go**

Create `apps/api/internal/api/signup.go`:

```go
package api

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"

	"mindimprint/api/internal/auth"
	"mindimprint/api/internal/httpx"
	"mindimprint/api/internal/store/sqlc"
)

const defaultAvatarColor = "#7C9CF0"

func (a *API) signup(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Email       string `json:"email"`
		Password    string `json:"password"`
		DisplayName string `json:"display_name"`
		JoinCode    string `json:"join_code"`
	}
	if err := decodeJSON(r, &body); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	body.Email = strings.TrimSpace(strings.ToLower(body.Email))
	body.DisplayName = strings.TrimSpace(body.DisplayName)
	body.JoinCode = strings.TrimSpace(body.JoinCode)
	if body.Email == "" || body.DisplayName == "" {
		httpx.WriteError(w, r, httpx.ErrBadRequest("validation_failed", "邮箱和姓名不能为空", nil))
		return
	}
	if len(body.Password) < 8 {
		httpx.WriteError(w, r, httpx.ErrBadRequest("validation_failed", "密码至少 8 位", nil))
		return
	}

	cls, err := a.d.Queries.GetClassByJoinCode(r.Context(), body.JoinCode)
	if err != nil {
		// ErrNoRows or anything else → invalid code (do not leak existence detail).
		httpx.WriteError(w, r, httpx.ErrInvalidJoinCode())
		return
	}

	hash, err := auth.HashPassword(body.Password)
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

	u, err := qtx.CreateUser(r.Context(), sqlc.CreateUserParams{
		Email:           body.Email,
		PasswordHash:    hash,
		Role:            "student",
		SchoolID:        cls.SchoolID,
		DisplayName:     body.DisplayName,
		AvatarColor:     defaultAvatarColor,
		EmailVerifiedAt: pgtype.Timestamptz{Time: time.Now(), Valid: true}, // auto-verify in P2
	})
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
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
```

- [ ] **Step 9: Register the public signup route**

In `apps/api/internal/api/api.go` `Handler()`, add **before** the `return` line:

```go
	mux.HandleFunc("POST /api/v1/auth/signup", a.signup)
```

(The route stays public — `ActAsSeed` still wraps everything for now; signup ignores any context user.)

- [ ] **Step 10: Wire main.go**

In `apps/api/cmd/api/main.go`, in the `api.New(api.Deps{...})` literal add:

```go
		Pool:         pool,
		CookieSecure: cfg.CookieSecure,
```

- [ ] **Step 11: Run the gate**

Run: `cd apps/api && go vet ./... && DOCKER_HOST=unix:///var/run/docker.sock go test -p 1 ./internal/api/ -run TestSignup -v && go test ./internal/config/`
Expected: PASS. Also `go build ./...` clean.

- [ ] **Step 12: Commit**

```bash
git add apps/api/internal/api/signup.go apps/api/internal/api/cookie.go apps/api/internal/api/signup_test.go apps/api/internal/api/api.go apps/api/internal/api/auth.go apps/api/internal/api/maintest_test.go apps/api/internal/httpx/errors.go apps/api/internal/config/ apps/api/cmd/api/main.go
git commit -m "feat(p2): signup handler (atomic join-code) + auth errors + cookie/config plumbing

Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>"
```

---

### Task 6: Signin + signout handlers

**Files:**
- Create: `apps/api/internal/api/signin.go`, `apps/api/internal/api/signout.go`
- Create: `apps/api/internal/api/signin_test.go`
- Modify: `apps/api/internal/api/api.go` (2 routes), `apps/api/internal/api/dto.go` (`meUserDTO` + `toMeUserDTO`)

**Interfaces:**
- Consumes: `auth.VerifyPassword`/`auth.NewToken` (T2); `GetUserByEmail`/`CreateSession`/`DeleteSessionByHash`/`GetSchool`/`ListClassesForUser` (T3); cookie helpers (T5).
- Produces: `meUserDTO` (id, email, display_name, role, avatar_color, school{id,name}, classes[{id,name,role_in_class}]) + `func (a *API) buildMeUser(ctx, u User) (meUserDTO, error)`.

- [ ] **Step 1: Add the me-user DTO**

In `apps/api/internal/api/dto.go`, add (and the imports it needs are already present — `context` is NOT; add `"context"`):

```go
type meSchoolDTO struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type meClassDTO struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	RoleInClass string `json:"role_in_class"`
}

type meUserDTO struct {
	ID          string       `json:"id"`
	Email       string       `json:"email"`
	DisplayName string       `json:"display_name"`
	Role        string       `json:"role"`
	AvatarColor string       `json:"avatar_color"`
	School      meSchoolDTO  `json:"school"`
	Classes     []meClassDTO `json:"classes"`
}

// buildMeUser assembles the full /me + signin user payload from the principal.
func (a *API) buildMeUser(ctx context.Context, u User) (meUserDTO, error) {
	full, err := a.d.Queries.GetUserByID(ctx, u.ID)
	if err != nil {
		return meUserDTO{}, err
	}
	school, err := a.d.Queries.GetSchool(ctx, u.SchoolID)
	if err != nil {
		return meUserDTO{}, err
	}
	rows, err := a.d.Queries.ListClassesForUser(ctx, u.ID)
	if err != nil {
		return meUserDTO{}, err
	}
	classes := make([]meClassDTO, 0, len(rows))
	for _, c := range rows {
		classes = append(classes, meClassDTO{ID: c.ID.String(), Name: c.Name, RoleInClass: c.RoleInClass})
	}
	return meUserDTO{
		ID:          full.ID.String(),
		Email:       full.Email,
		DisplayName: full.DisplayName,
		Role:        full.Role,
		AvatarColor: full.AvatarColor,
		School:      meSchoolDTO{ID: school.ID.String(), Name: school.Name},
		Classes:     classes,
	}, nil
}
```

- [ ] **Step 2: Write the signin/signout test**

Create `apps/api/internal/api/signin_test.go`:

```go
package api_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"

	. "mindimprint/api/internal/api"
	"mindimprint/api/internal/auth"
	"mindimprint/api/internal/store/sqlc"
)

func TestSigninSignout(t *testing.T) {
	pool := newAPITestPool(t)
	h := New(Deps{Queries: sqlc.New(pool), Pool: pool}).Handler()

	// Seeded Phoebe (migration 0004) can sign in.
	creds := `{"email":"phoebe@demo.mindimprint.local","password":"phoebe-dev-pass"}`
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest("POST", "/api/v1/auth/signin", strings.NewReader(creds)))
	if rr.Code != 200 {
		t.Fatalf("signin: want 200, got %d — %s", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), `"display_name":"Phoebe"`) {
		t.Fatalf("signin body missing user: %s", rr.Body.String())
	}
	var cookie *http.Cookie
	for _, c := range rr.Result().Cookies() {
		if c.Name == "mk_session" {
			cookie = c
		}
	}
	if cookie == nil || cookie.Value == "" {
		t.Fatal("signin did not set mk_session cookie")
	}

	// Wrong password → 401.
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest("POST", "/api/v1/auth/signin",
		strings.NewReader(`{"email":"phoebe@demo.mindimprint.local","password":"nope"}`)))
	if rr.Code != 401 || !strings.Contains(rr.Body.String(), "invalid_credentials") {
		t.Fatalf("bad pw: want 401 invalid_credentials, got %d — %s", rr.Code, rr.Body.String())
	}

	// Unknown email → 401 (same code, no enumeration).
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest("POST", "/api/v1/auth/signin",
		strings.NewReader(`{"email":"ghost@demo.local","password":"whatever1"}`)))
	if rr.Code != 401 {
		t.Fatalf("unknown email: want 401, got %d", rr.Code)
	}

	// Signout with the cookie → 204 and the session is revoked.
	req := httptest.NewRequest("POST", "/api/v1/auth/signout", nil)
	req.AddCookie(cookie)
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != 204 {
		t.Fatalf("signout: want 204, got %d — %s", rr.Code, rr.Body.String())
	}
	q := sqlc.New(pool)
	if _, err := q.GetSessionWithUserByHash(context.Background(), auth.HashToken(cookie.Value)); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("session not revoked after signout: %v", err)
	}
}
```

- [ ] **Step 3: Implement signin.go**

Create `apps/api/internal/api/signin.go`:

```go
package api

import (
	"net/http"
	"strings"
	"time"

	"mindimprint/api/internal/auth"
	"mindimprint/api/internal/httpx"
	"mindimprint/api/internal/store/sqlc"
)

func (a *API) signin(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := decodeJSON(r, &body); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	email := strings.TrimSpace(strings.ToLower(body.Email))

	full, err := a.d.Queries.GetUserByEmail(r.Context(), email)
	if err != nil {
		httpx.WriteError(w, r, httpx.ErrInvalidCredentials()) // ErrNoRows hidden as invalid creds
		return
	}
	ok, err := auth.VerifyPassword(body.Password, full.PasswordHash)
	if err != nil || !ok {
		httpx.WriteError(w, r, httpx.ErrInvalidCredentials())
		return
	}
	if !full.EmailVerifiedAt.Valid {
		httpx.WriteError(w, r, httpx.ErrEmailUnverified())
		return
	}

	raw, hash, err := auth.NewToken()
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	ua := r.UserAgent()
	ip := r.RemoteAddr
	if _, err := a.d.Queries.CreateSession(r.Context(), sqlc.CreateSessionParams{
		UserID:    full.ID,
		TokenHash: hash,
		ExpiresAt: time.Now().Add(sessionTTL),
		UserAgent: &ua,
		Ip:        &ip,
	}); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	setSessionCookie(w, raw, a.d.CookieSecure)

	me, err := a.buildMeUser(r.Context(), User{ID: full.ID, SchoolID: full.SchoolID, Role: full.Role, DisplayName: full.DisplayName})
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"user": me})
}
```

- [ ] **Step 4: Implement signout.go**

Create `apps/api/internal/api/signout.go`:

```go
package api

import (
	"net/http"

	"mindimprint/api/internal/auth"
	"mindimprint/api/internal/httpx"
)

func (a *API) signout(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie(sessionCookieName); err == nil && c.Value != "" {
		// Best-effort revoke; a missing/already-gone session is still a clean signout.
		if err := a.d.Queries.DeleteSessionByHash(r.Context(), auth.HashToken(c.Value)); err != nil {
			httpx.WriteError(w, r, err)
			return
		}
	}
	clearSessionCookie(w, a.d.CookieSecure)
	w.WriteHeader(http.StatusNoContent)
}
```

- [ ] **Step 5: Register the routes**

In `apps/api/internal/api/api.go` `Handler()`, add before `return`:

```go
	mux.HandleFunc("POST /api/v1/auth/signin", a.signin)
	mux.HandleFunc("POST /api/v1/auth/signout", a.signout)
```

- [ ] **Step 6: Run the gate**

Run: `cd apps/api && go vet ./... && DOCKER_HOST=unix:///var/run/docker.sock go test -p 1 ./internal/api/ -run 'TestSigninSignout|TestSignup' -v`
Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add apps/api/internal/api/signin.go apps/api/internal/api/signout.go apps/api/internal/api/signin_test.go apps/api/internal/api/api.go apps/api/internal/api/dto.go
git commit -m "feat(p2): signin (session cookie) + signout handlers

Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>"
```

---

### Task 7: Verify-email handler (dormant, tested)

**Files:**
- Create: `apps/api/internal/api/verify.go`, `apps/api/internal/api/verify_test.go`
- Modify: `apps/api/internal/api/api.go` (1 route)

**Interfaces:**
- Consumes: `auth.HashToken` (T2); `GetActiveEmailVerificationToken`/`ConsumeEmailVerificationToken`/`MarkEmailVerified` (T3); `buildMeUser` (T6).

- [ ] **Step 1: Write the test (direct-insert token)**

Create `apps/api/internal/api/verify_test.go`:

```go
package api_test

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	. "mindimprint/api/internal/api"
	"mindimprint/api/internal/auth"
	"mindimprint/api/internal/store/sqlc"
)

func TestVerifyEmail(t *testing.T) {
	pool := newAPITestPool(t)
	q := sqlc.New(pool)
	h := New(Deps{Queries: q, Pool: pool}).Handler()
	ctx := context.Background()

	// Create an UNVERIFIED user directly (signup auto-verifies, so build one by hand).
	cls, err := q.GetClassByJoinCode(ctx, "DEMO-0001")
	if err != nil {
		t.Fatal(err)
	}
	u, err := q.CreateUser(ctx, sqlc.CreateUserParams{
		Email: "unverified@demo.local", PasswordHash: "x", Role: "student",
		SchoolID: cls.SchoolID, DisplayName: "Un", AvatarColor: "#7C9CF0",
		EmailVerifiedAt: pgtype.Timestamptz{Valid: false},
	})
	if err != nil {
		t.Fatal(err)
	}

	raw, hash, _ := auth.NewToken()
	if _, err := q.CreateEmailVerificationToken(ctx, sqlc.CreateEmailVerificationTokenParams{
		UserID: u.ID, TokenHash: hash, ExpiresAt: time.Now().Add(time.Hour),
	}); err != nil {
		t.Fatal(err)
	}

	// Verify → 200.
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest("POST", "/api/v1/auth/verify-email",
		strings.NewReader(`{"token":"`+raw+`"}`)))
	if rr.Code != 200 {
		t.Fatalf("verify: want 200, got %d — %s", rr.Code, rr.Body.String())
	}

	// Reuse → 400 token_invalid_or_expired (consumed).
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest("POST", "/api/v1/auth/verify-email",
		strings.NewReader(`{"token":"`+raw+`"}`)))
	if rr.Code != 400 || !strings.Contains(rr.Body.String(), "token_invalid_or_expired") {
		t.Fatalf("reuse: want 400 token_invalid_or_expired, got %d — %s", rr.Code, rr.Body.String())
	}

	// Garbage token → 400.
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest("POST", "/api/v1/auth/verify-email",
		strings.NewReader(`{"token":"`+uuid.NewString()+`"}`)))
	if rr.Code != 400 {
		t.Fatalf("garbage: want 400, got %d", rr.Code)
	}
}
```

- [ ] **Step 2: Implement verify.go**

Create `apps/api/internal/api/verify.go`:

```go
package api

import (
	"net/http"
	"strings"

	"mindimprint/api/internal/auth"
	"mindimprint/api/internal/httpx"
)

// verifyEmail consumes a verification token and marks the account verified.
// Dormant in P2 (signup auto-verifies) but live and tested for the future
// email flow.
func (a *API) verifyEmail(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Token string `json:"token"`
	}
	if err := decodeJSON(r, &body); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	token := strings.TrimSpace(body.Token)
	if token == "" {
		httpx.WriteError(w, r, httpx.ErrTokenInvalid())
		return
	}
	evt, err := a.d.Queries.GetActiveEmailVerificationToken(r.Context(), auth.HashToken(token))
	if err != nil {
		httpx.WriteError(w, r, httpx.ErrTokenInvalid()) // ErrNoRows → invalid/expired
		return
	}
	if err := a.d.Queries.ConsumeEmailVerificationToken(r.Context(), evt.ID); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	u, err := a.d.Queries.MarkEmailVerified(r.Context(), evt.UserID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	me, err := a.buildMeUser(r.Context(), User{ID: u.ID, SchoolID: u.SchoolID, Role: u.Role, DisplayName: u.DisplayName})
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"user": me})
}
```

- [ ] **Step 3: Register the route**

In `apps/api/internal/api/api.go` `Handler()`, add before `return`:

```go
	mux.HandleFunc("POST /api/v1/auth/verify-email", a.verifyEmail)
```

- [ ] **Step 4: Run the gate**

Run: `cd apps/api && go vet ./... && DOCKER_HOST=unix:///var/run/docker.sock go test -p 1 ./internal/api/ -run TestVerifyEmail -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add apps/api/internal/api/verify.go apps/api/internal/api/verify_test.go apps/api/internal/api/api.go
git commit -m "feat(p2): verify-email handler (dormant, single-use token)

Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>"
```

---

### Task 8: `GET /auth/me` handler

**Files:**
- Create: `apps/api/internal/api/me.go`, `apps/api/internal/api/me_test.go`
- Modify: `apps/api/internal/api/api.go` (1 route)

**Interfaces:**
- Consumes: `UserFromContext` (existing); `buildMeUser` (T6).
- Note: while `ActAsSeed` is still mounted, `/me` returns the seeded student without a cookie. Task 9 makes it cookie-driven; this test asserts the seed shape now and is updated in Task 9.

- [ ] **Step 1: Write the test**

Create `apps/api/internal/api/me_test.go`:

```go
package api_test

import (
	"net/http/httptest"
	"strings"
	"testing"

	. "mindimprint/api/internal/api"
	"mindimprint/api/internal/store/sqlc"
)

func TestMeReturnsSeedUserUnderActAsSeed(t *testing.T) {
	pool := newAPITestPool(t)
	h := New(Deps{Queries: sqlc.New(pool), Pool: pool}).Handler()

	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest("GET", "/api/v1/auth/me", nil))
	if rr.Code != 200 {
		t.Fatalf("me: want 200, got %d — %s", rr.Code, rr.Body.String())
	}
	body := rr.Body.String()
	for _, want := range []string{`"display_name":"Phoebe"`, `"school"`, `"classes"`, `"role":"student"`} {
		if !strings.Contains(body, want) {
			t.Fatalf("me body missing %s — %s", want, body)
		}
	}
}
```

- [ ] **Step 2: Implement me.go**

Create `apps/api/internal/api/me.go`:

```go
package api

import (
	"net/http"

	"mindimprint/api/internal/httpx"
)

func (a *API) me(w http.ResponseWriter, r *http.Request) {
	u, ok := UserFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, r, httpx.ErrUnauthorized("未登录"))
		return
	}
	me, err := a.buildMeUser(r.Context(), u)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"user": me})
}
```

- [ ] **Step 3: Register the route**

In `apps/api/internal/api/api.go` `Handler()`, add before `return`:

```go
	mux.HandleFunc("GET /api/v1/auth/me", a.me)
```

- [ ] **Step 4: Run the gate**

Run: `cd apps/api && go vet ./... && DOCKER_HOST=unix:///var/run/docker.sock go test -p 1 ./internal/api/ -run TestMeReturnsSeedUserUnderActAsSeed -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add apps/api/internal/api/me.go apps/api/internal/api/me_test.go apps/api/internal/api/api.go
git commit -m "feat(p2): GET /auth/me handler

Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>"
```

---

### Task 9: Cutover — SessionAuth + RequireUser, delete ActAsSeed, migrate tests

**Files:**
- Modify: `apps/api/internal/api/auth.go` (delete `ActAsSeed`; add `SessionAuth`, `RequireUser`)
- Modify: `apps/api/internal/api/api.go` (`Handler()` — public vs protected split, `SessionAuth` wrap)
- Modify: `apps/api/internal/api/maintest_test.go` (add `signInSeed` + `withCookie` helpers)
- Modify: `apps/api/internal/api/me_test.go`, `tasks_test.go`, `cards_test.go`, `turn_test.go`, `evaluate_test.go`, `e2e_test.go` (attach the session cookie)

**Interfaces:**
- Consumes: `auth.HashToken` (T2); `GetSessionWithUserByHash` (T3); `SeedUserID` (existing).
- Produces: `SessionAuth(q *sqlc.Queries) func(http.Handler) http.Handler`; `RequireUser(next http.Handler) http.Handler`; test helpers `signInSeed(t, pool) *http.Cookie`, `withCookie(req, c) *http.Request`.

- [ ] **Step 1: Replace ActAsSeed with SessionAuth + RequireUser**

In `apps/api/internal/api/auth.go`, **delete** the `ActAsSeed` function and replace it with (keep `SeedUserID`, the `User` type, `WithUser`, `UserFromContext`, `TxBeginner`, and the `ctxKey`/`ctxKeyUser` declarations):

```go
// SessionAuth resolves the mk_session cookie into the request user context if a
// valid (unexpired) session exists. It never rejects on its own — absence just
// means no user in context; RequireUser does the rejecting. Replaces the P1
// ActAsSeed dev shim.
func SessionAuth(q *sqlc.Queries) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			c, err := r.Cookie(sessionCookieName)
			if err != nil || c.Value == "" {
				next.ServeHTTP(w, r)
				return
			}
			row, err := q.GetSessionWithUserByHash(r.Context(), auth.HashToken(c.Value))
			if err != nil {
				next.ServeHTTP(w, r) // invalid/expired → unauthenticated
				return
			}
			u := User{ID: row.ID, SchoolID: row.SchoolID, Role: row.Role, DisplayName: row.DisplayName}
			next.ServeHTTP(w, r.WithContext(WithUser(r.Context(), u)))
		})
	}
}

// RequireUser rejects requests with no authenticated user (401).
func RequireUser(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, ok := UserFromContext(r.Context()); !ok {
			httpx.WriteError(w, r, httpx.ErrUnauthorized("未登录"))
			return
		}
		next.ServeHTTP(w, r)
	})
}
```

Update the `auth.go` imports: drop nothing structural, add `"mindimprint/api/internal/auth"` and keep `"mindimprint/api/internal/httpx"`, `"net/http"`, `"context"`, `"github.com/google/uuid"`, `"github.com/jackc/pgx/v5"`, `"mindimprint/api/internal/store/sqlc"`. (Remove the `GetUserByID`-based seed load — it's gone with `ActAsSeed`.)

- [ ] **Step 2: Rewrite Handler() with the public/protected split**

In `apps/api/internal/api/api.go`, replace the whole `Handler()` body:

```go
// Handler returns the /api/v1 mux. Auth routes (except /me) are public; every
// other route requires a resolved session. SessionAuth runs for all requests
// (so signout can read the cookie); RequireUser guards the protected group.
func (a *API) Handler() http.Handler {
	mux := http.NewServeMux()

	// Public auth routes.
	mux.HandleFunc("POST /api/v1/auth/signup", a.signup)
	mux.HandleFunc("POST /api/v1/auth/signin", a.signin)
	mux.HandleFunc("POST /api/v1/auth/signout", a.signout)
	mux.HandleFunc("POST /api/v1/auth/verify-email", a.verifyEmail)

	// Protected routes (require a session).
	protected := func(h http.HandlerFunc) http.Handler { return RequireUser(h) }
	mux.Handle("GET /api/v1/auth/me", protected(a.me))
	mux.Handle("GET /api/v1/tasks", protected(a.listTasks))
	mux.Handle("POST /api/v1/tasks", protected(a.createTask))
	mux.Handle("GET /api/v1/tasks/{id}", protected(a.getTask))
	mux.Handle("PATCH /api/v1/tasks/{id}/cards/{cid}", protected(a.patchCard))
	mux.Handle("PUT /api/v1/tasks/{id}/cards/{cid}", protected(a.putCard))
	mux.Handle("POST /api/v1/tasks/{id}/cards/{cid}/skip", protected(a.skipCard))
	mux.Handle("POST /api/v1/tasks/{id}/turn", protected(a.postTurn))
	mux.Handle("POST /api/v1/tasks/{id}/evaluate", protected(a.postEvaluate))
	mux.Handle("GET /api/v1/tasks/{id}/evaluation", protected(a.getEvaluation))

	return SessionAuth(a.d.Queries)(mux)
}
```

- [ ] **Step 3: Add test helpers to the harness**

In `apps/api/internal/api/maintest_test.go`, add imports `"context"` (already present), `"net/http"`, `"testing"` (present), `"time"` (present), and `"mindimprint/api/internal/auth"`, then add:

```go
// signInSeed creates a live session for the seeded student and returns the
// cookie to attach to authed requests (replaces the implicit ActAsSeed inject).
func signInSeed(t *testing.T, pool *pgxpool.Pool) *http.Cookie {
	t.Helper()
	q := sqlc.New(pool)
	raw, hash, err := auth.NewToken()
	if err != nil {
		t.Fatalf("token: %v", err)
	}
	if _, err := q.CreateSession(context.Background(), sqlc.CreateSessionParams{
		UserID:    SeedUserID,
		TokenHash: hash,
		ExpiresAt: time.Now().Add(time.Hour),
	}); err != nil {
		t.Fatalf("create session: %v", err)
	}
	return &http.Cookie{Name: "mk_session", Value: raw}
}

// withCookie attaches c to req and returns it (for inline request building).
func withCookie(req *http.Request, c *http.Cookie) *http.Request {
	req.AddCookie(c)
	return req
}
```

Note: Go imports are file-scoped, so `maintest_test.go` needs its own import to see `SeedUserID`. Add `. "mindimprint/api/internal/api"` to its import block (matching the sibling `_test.go` files), so `signInSeed` can reference `SeedUserID` directly.

- [ ] **Step 4: Update me_test.go for the cookie**

Replace `TestMeReturnsSeedUserUnderActAsSeed` with a cookie-driven version and add a no-cookie 401 case:

```go
func TestMeRequiresSessionAndReturnsUser(t *testing.T) {
	pool := newAPITestPool(t)
	h := New(Deps{Queries: sqlc.New(pool), Pool: pool}).Handler()

	// No cookie → 401.
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest("GET", "/api/v1/auth/me", nil))
	if rr.Code != 401 {
		t.Fatalf("no cookie: want 401, got %d — %s", rr.Code, rr.Body.String())
	}

	// With a seed session → 200 + the seeded user.
	cookie := signInSeed(t, pool)
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, withCookie(httptest.NewRequest("GET", "/api/v1/auth/me", nil), cookie))
	if rr.Code != 200 {
		t.Fatalf("me: want 200, got %d — %s", rr.Code, rr.Body.String())
	}
	body := rr.Body.String()
	for _, want := range []string{`"display_name":"Phoebe"`, `"school"`, `"classes"`, `"role":"student"`} {
		if !strings.Contains(body, want) {
			t.Fatalf("me body missing %s — %s", want, body)
		}
	}
}
```

- [ ] **Step 5: Migrate the existing protected-route tests**

In each of `tasks_test.go`, `cards_test.go`, `turn_test.go`, `evaluate_test.go`, `e2e_test.go`, apply this mechanical transform so every request to a protected route carries a seed session cookie:

1. Where the handler is built — change `q := newAPITestQueries(t); h := New(Deps{Queries: q ...}).Handler()` to obtain a pool and a cookie:

```go
	pool := newAPITestPool(t)
	h := New(Deps{Queries: sqlc.New(pool), Pool: pool /* keep any existing Provider/Resolver/Catalog/SpecByID fields */}).Handler()
	cookie := signInSeed(t, pool)
```

   (Tests that pass `Provider`/`ChatResolver`/`EvalResolver`/`Catalog`/`SpecByID` — e.g. `turn_test.go`, `evaluate_test.go`, `e2e_test.go` — keep those fields; only add `Pool: pool` and switch `Queries` to `sqlc.New(pool)`.)

2. Wrap every `httptest.NewRequest(...)` that targets a protected route in `withCookie(..., cookie)`. Example:

```go
	// before
	h.ServeHTTP(rr, httptest.NewRequest("GET", "/api/v1/tasks", nil))
	// after
	h.ServeHTTP(rr, withCookie(httptest.NewRequest("GET", "/api/v1/tasks", nil), cookie))
```

   Apply to ALL requests in these files (they all hit protected routes). Add the `sqlc` import to any test file that now calls `sqlc.New` if it isn't already imported.

- [ ] **Step 6: Run the full backend gate**

Run: `cd apps/api && go vet ./... && DOCKER_HOST=unix:///var/run/docker.sock go test -p 1 ./...`
Expected: ALL packages PASS (auth, config, store, api incl. the migrated tasks/cards/turn/evaluate/e2e/me/signup/signin/verify). If a protected-route test 401s, a request is missing `withCookie` — fix that request, not the middleware.

- [ ] **Step 7: Commit**

```bash
git add apps/api/internal/api/auth.go apps/api/internal/api/api.go apps/api/internal/api/maintest_test.go apps/api/internal/api/me_test.go apps/api/internal/api/tasks_test.go apps/api/internal/api/cards_test.go apps/api/internal/api/turn_test.go apps/api/internal/api/evaluate_test.go apps/api/internal/api/e2e_test.go
git commit -m "feat(p2): cutover to real session auth — SessionAuth + RequireUser, delete ActAsSeed

Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>"
```

---

### Task 10: Frontend — `src/api/auth.ts`

**Files:**
- Create: `apps/web/src/api/auth.ts`, `apps/web/src/api/auth.test.ts`
- Modify: `apps/web/src/api/index.ts` (extend `ApiClient` + `api`)

**Interfaces:**
- Produces:
  - `MeUser` type `{ id; email; display_name; role; avatar_color; school: { id; name }; classes: { id; name; role_in_class }[] }`.
  - `signup(input: { email; password; display_name; join_code }): Promise<void>`
  - `verifyEmail(token: string): Promise<MeUser>`
  - `signin(input: { email; password }): Promise<MeUser>`
  - `signout(): Promise<void>`
  - `getMe(): Promise<MeUser>`
  - extends `ApiClient` interface + the default `api` object with these five.

- [ ] **Step 1: Write the auth client test**

Create `apps/web/src/api/auth.test.ts`:

```ts
import { describe, it, expect, vi, beforeEach } from "vitest";
import { signup, signin, signout, getMe, verifyEmail } from "./auth";

const ME = {
  id: "u1", email: "p@d.local", display_name: "Phoebe", role: "student",
  avatar_color: "#7C9CF0", school: { id: "s1", name: "Demo" }, classes: [],
};

function mockFetch(status: number, body: unknown) {
  return vi.fn(async () => ({
    ok: status >= 200 && status < 300,
    status,
    json: async () => body,
  })) as unknown as typeof fetch;
}

describe("api/auth", () => {
  beforeEach(() => vi.restoreAllMocks());

  it("signin posts creds and returns the user", async () => {
    const f = mockFetch(200, { user: ME });
    vi.stubGlobal("fetch", f);
    const u = await signin({ email: "p@d.local", password: "pw" });
    expect(u.display_name).toBe("Phoebe");
    const [, init] = (f as unknown as { mock: { calls: [string, RequestInit][] } }).mock.calls[0];
    expect(init.method).toBe("POST");
    expect(JSON.parse(init.body as string)).toEqual({ email: "p@d.local", password: "pw" });
  });

  it("signup posts and resolves void on 201", async () => {
    vi.stubGlobal("fetch", mockFetch(201, {}));
    await expect(signup({ email: "a@b.c", password: "pw123456", display_name: "A", join_code: "DEMO-0001" })).resolves.toBeUndefined();
  });

  it("getMe returns the user", async () => {
    vi.stubGlobal("fetch", mockFetch(200, { user: ME }));
    expect((await getMe()).email).toBe("p@d.local");
  });

  it("verifyEmail returns the user", async () => {
    vi.stubGlobal("fetch", mockFetch(200, { user: ME }));
    expect((await verifyEmail("tok")).id).toBe("u1");
  });

  it("signout resolves on 204", async () => {
    vi.stubGlobal("fetch", mockFetch(204, undefined));
    await expect(signout()).resolves.toBeUndefined();
  });

  it("signin surfaces ApiError on 401", async () => {
    vi.stubGlobal("fetch", mockFetch(401, { error: { code: "invalid_credentials", message: "邮箱或密码错误" } }));
    await expect(signin({ email: "x", password: "y" })).rejects.toMatchObject({ code: "invalid_credentials" });
  });
});
```

- [ ] **Step 2: Run it (fails — no module)**

Run: `pnpm --filter web test -- src/api/auth.test.ts`
Expected: FAIL (cannot resolve `./auth`).

- [ ] **Step 3: Implement auth.ts**

Create `apps/web/src/api/auth.ts`:

```ts
import { apiFetch } from "./client";

export interface MeUser {
  id: string;
  email: string;
  display_name: string;
  role: string;
  avatar_color: string;
  school: { id: string; name: string };
  classes: { id: string; name: string; role_in_class: string }[];
}

export async function signup(input: {
  email: string;
  password: string;
  display_name: string;
  join_code: string;
}): Promise<void> {
  await apiFetch<Record<string, never>>("/api/v1/auth/signup", {
    method: "POST",
    body: JSON.stringify(input),
  });
}

export async function verifyEmail(token: string): Promise<MeUser> {
  const r = await apiFetch<{ user: MeUser }>("/api/v1/auth/verify-email", {
    method: "POST",
    body: JSON.stringify({ token }),
  });
  return r.user;
}

export async function signin(input: { email: string; password: string }): Promise<MeUser> {
  const r = await apiFetch<{ user: MeUser }>("/api/v1/auth/signin", {
    method: "POST",
    body: JSON.stringify(input),
  });
  return r.user;
}

export async function signout(): Promise<void> {
  await apiFetch<void>("/api/v1/auth/signout", { method: "POST" });
}

export async function getMe(): Promise<MeUser> {
  const r = await apiFetch<{ user: MeUser }>("/api/v1/auth/me");
  return r.user;
}
```

- [ ] **Step 4: Extend the ApiClient surface**

In `apps/web/src/api/index.ts`, add to the imports, the `ApiClient` interface, and the `api` object:

```ts
// add near the other imports
import { signup, verifyEmail, signin, signout, getMe, type MeUser } from "./auth";

// add to the export type line
export type { TaskDetail, TurnEvent, MeUser };

// add to the ApiClient interface
  signup(input: { email: string; password: string; display_name: string; join_code: string }): Promise<void>;
  verifyEmail(token: string): Promise<MeUser>;
  signin(input: { email: string; password: string }): Promise<MeUser>;
  signout(): Promise<void>;
  getMe(): Promise<MeUser>;

// add to the api object literal
  signup, verifyEmail, signin, signout, getMe,
```

- [ ] **Step 5: Run the web gate**

Run: `pnpm --filter web test -- src/api/auth.test.ts && pnpm -r typecheck`
Expected: PASS, typecheck clean.

- [ ] **Step 6: Commit**

```bash
git add apps/web/src/api/auth.ts apps/web/src/api/auth.test.ts apps/web/src/api/index.ts
git commit -m "feat(p2): web auth API client (signup/signin/signout/me/verify)

Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>"
```

---

### Task 11: Frontend — wire AuthScreen

**Files:**
- Modify: `apps/web/src/shell/auth/AuthScreen.tsx`, `apps/web/src/shell/auth/AuthScreen.test.tsx`

**Interfaces:**
- Consumes: `api.signin`, `api.signup` (T10); `MeUser`, `ApiError`.
- Produces: `AuthScreen` prop shape changes from `{ onEnterApp: () => void }` to `{ onAuthed: (user: MeUser) => void; client?: Pick<ApiClient, "signin" | "signup"> }` (the optional `client` lets tests inject; defaults to `api`).

- [ ] **Step 1: Rewrite AuthScreen as a wired controller**

Rewrite `apps/web/src/shell/auth/AuthScreen.tsx` to: use controlled inputs; on login submit call `signin`; on register collect name/email/password then move to bind; on bind submit call `signup({email,password,display_name,join_code})` then `signin` then `onAuthed`; show inline errors (mapped from `ApiError.code`) and a disabled/loading button state. Remove the "暂时跳过" no-code escape (signup requires a join code). Preserve the exact visual styling (the `.dc.html` design is binding) — only convert `defaultValue`→`value`+`onChange`, wire `onClick` handlers, and add a small error line above each primary button.

Use this structure (full file):

```tsx
import { useState } from "react";
import { api, ApiError, type ApiClient, type MeUser } from "../../api";

type AuthClient = Pick<ApiClient, "signin" | "signup">;

const CARD = { background: "#fff", border: "1px solid #EAECF2", borderRadius: "18px", padding: "26px 26px 24px", boxShadow: "0 8px 30px rgba(20,30,60,.06)" } as const;
const LABEL = { fontSize: "12.5px", fontWeight: 600, color: "#3A4256", marginBottom: "6px" } as const;
const INPUT = { width: "100%", border: "1px solid #E1E4ED", borderRadius: "11px", padding: "12px 14px", fontSize: "14px", color: "#1C2333", background: "#FCFCFD", outline: "none", marginBottom: "14px", boxSizing: "border-box" } as const;
const BTN = { width: "100%", background: "#2A3B7A", color: "#fff", border: "none", padding: "13px", borderRadius: "12px", fontSize: "15px", fontWeight: 700, cursor: "pointer", fontFamily: "inherit" } as const;
const ERR = { color: "#C0392B", fontSize: "13px", marginBottom: "12px", textAlign: "center" } as const;

function messageFor(e: unknown): string {
  if (e instanceof ApiError) return e.message;
  return "网络错误，请稍后再试";
}

export function AuthScreen({
  onAuthed,
  client = api,
}: {
  onAuthed: (user: MeUser) => void;
  client?: AuthClient;
}) {
  const [step, setStep] = useState<"login" | "register" | "bind">("login");
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [displayName, setDisplayName] = useState("");
  const [joinCode, setJoinCode] = useState("");
  const [err, setErr] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  async function doLogin() {
    setErr(null); setBusy(true);
    try {
      onAuthed(await client.signin({ email, password }));
    } catch (e) {
      setErr(messageFor(e));
    } finally {
      setBusy(false);
    }
  }

  async function doRegister() {
    setErr(null); setBusy(true);
    try {
      await client.signup({ email, password, display_name: displayName, join_code: joinCode });
      onAuthed(await client.signin({ email, password }));
    } catch (e) {
      setErr(messageFor(e));
    } finally {
      setBusy(false);
    }
  }

  return (
    <div style={{ width: "100%", height: "100%", display: "flex", alignItems: "center", justifyContent: "center", background: "radial-gradient(1200px 600px at 50% -10%, #EAEDF8 0%, #F3F4F8 60%)", padding: "24px" }}>
      <div style={{ width: "100%", maxWidth: "420px" }}>
        <div style={{ display: "flex", flexDirection: "column", alignItems: "center", marginBottom: "26px" }}>
          <svg viewBox="0 0 48 48" width="60" height="60" style={{ display: "block", filter: "drop-shadow(0 8px 18px rgba(42,59,122,.22))" }}>
            <rect x="5" y="6" width="38" height="36" rx="13" fill="#2A3B7A"></rect>
            <rect x="5" y="6" width="38" height="17" rx="13" fill="#ffffff" opacity="0.10"></rect>
            <ellipse cx="18.5" cy="24" rx="3.4" ry="3.9" fill="#fff"></ellipse>
            <ellipse cx="29.5" cy="24" rx="3.4" ry="3.9" fill="#fff"></ellipse>
            <circle cx="19.3" cy="25" r="1.5" fill="#1C2333"></circle>
            <circle cx="30.3" cy="25" r="1.5" fill="#1C2333"></circle>
            <path d="M19 31.5 Q24 35 29 31.5" stroke="#fff" strokeWidth="2.2" fill="none" strokeLinecap="round"></path>
            <circle cx="39" cy="9" r="4.5" fill="#E8A33D"></circle>
          </svg>
          <div style={{ fontSize: "22px", fontWeight: 800, color: "#1C2333", marginTop: "14px", letterSpacing: ".01em" }}>思维印记</div>
          <div style={{ fontSize: "13.5px", color: "#8A92A3", marginTop: "6px" }}>带着真实的问题来，和 AI 一起把思考走深</div>
        </div>

        <div style={CARD}>
          {step === "login" && (
            <>
              <div style={{ fontSize: "17px", fontWeight: 700, color: "#1C2333", marginBottom: "18px" }}>登录</div>
              <div style={LABEL}>邮箱 / 手机号</div>
              <input value={email} onChange={(e) => setEmail(e.target.value)} style={INPUT} />
              <div style={LABEL}>密码</div>
              <input type="password" value={password} onChange={(e) => setPassword(e.target.value)} style={{ ...INPUT, marginBottom: "20px" }} />
              {err && <div style={ERR}>{err}</div>}
              <button onClick={doLogin} disabled={busy} style={{ ...BTN, opacity: busy ? 0.6 : 1 }}>登录</button>
              <div style={{ textAlign: "center", marginTop: "16px", fontSize: "13px", color: "#8A92A3" }}>
                还没有账号？
                <span onClick={() => { setStep("register"); setErr(null); }} style={{ color: "#2A3B7A", fontWeight: 700, cursor: "pointer" }}>注册</span>
              </div>
            </>
          )}

          {step === "register" && (
            <>
              <div style={{ fontSize: "17px", fontWeight: 700, color: "#1C2333", marginBottom: "18px" }}>创建账号</div>
              <div style={LABEL}>姓名</div>
              <input value={displayName} onChange={(e) => setDisplayName(e.target.value)} style={INPUT} />
              <div style={LABEL}>邮箱</div>
              <input value={email} onChange={(e) => setEmail(e.target.value)} style={INPUT} />
              <div style={LABEL}>设置密码</div>
              <input type="password" value={password} onChange={(e) => setPassword(e.target.value)} style={{ ...INPUT, marginBottom: "20px" }} />
              <button onClick={() => { setErr(null); setStep("bind"); }} style={BTN}>下一步 · 绑定班级</button>
              <div style={{ textAlign: "center", marginTop: "16px", fontSize: "13px", color: "#8A92A3" }}>
                已有账号？
                <span onClick={() => { setStep("login"); setErr(null); }} style={{ color: "#2A3B7A", fontWeight: 700, cursor: "pointer" }}>登录</span>
              </div>
            </>
          )}

          {step === "bind" && (
            <>
              <div style={{ fontSize: "17px", fontWeight: 700, color: "#1C2333", marginBottom: "6px" }}>绑定你的班级</div>
              <div style={{ fontSize: "13px", color: "#8A92A3", lineHeight: "1.6", marginBottom: "18px" }}>
                绑定后，老师布置的探究任务会出现在你的任务列表里。你的思考过程只属于你，老师看不到对话本身。
              </div>
              <div style={LABEL}>班级邀请码</div>
              <input value={joinCode} onChange={(e) => setJoinCode(e.target.value)} style={INPUT} />
              {err && <div style={ERR}>{err}</div>}
              <button onClick={doRegister} disabled={busy} style={{ ...BTN, opacity: busy ? 0.6 : 1 }}>完成，进入思维印记</button>
            </>
          )}
        </div>
      </div>
    </div>
  );
}
```

- [ ] **Step 2: Update the AuthScreen test**

Replace `apps/web/src/shell/auth/AuthScreen.test.tsx`:

```tsx
import { describe, it, expect, vi } from "vitest";
import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import { AuthScreen } from "./AuthScreen";
import { ApiError } from "../../api";

const ME = {
  id: "u1", email: "p@d.local", display_name: "Phoebe", role: "student",
  avatar_color: "#7C9CF0", school: { id: "s1", name: "Demo" }, classes: [],
};

describe("AuthScreen", () => {
  it("signs in and calls onAuthed with the user", async () => {
    const onAuthed = vi.fn();
    const client = { signin: vi.fn(async () => ME), signup: vi.fn() };
    render(<AuthScreen onAuthed={onAuthed} client={client} />);
    fireEvent.change(screen.getByDisplayValue(""), { target: { value: "p@d.local" } });
    fireEvent.click(screen.getByRole("button", { name: "登录" }));
    await waitFor(() => expect(onAuthed).toHaveBeenCalledWith(ME));
  });

  it("shows an inline error on bad credentials", async () => {
    const onAuthed = vi.fn();
    const client = {
      signin: vi.fn(async () => { throw new ApiError("invalid_credentials", "邮箱或密码错误", 401); }),
      signup: vi.fn(),
    };
    render(<AuthScreen onAuthed={onAuthed} client={client} />);
    fireEvent.click(screen.getByRole("button", { name: "登录" }));
    await waitFor(() => expect(screen.getByText("邮箱或密码错误")).toBeInTheDocument());
    expect(onAuthed).not.toHaveBeenCalled();
  });

  it("register → bind → signup+signin → onAuthed", async () => {
    const onAuthed = vi.fn();
    const client = { signin: vi.fn(async () => ME), signup: vi.fn(async () => undefined) };
    render(<AuthScreen onAuthed={onAuthed} client={client} />);
    fireEvent.click(screen.getByText("注册"));
    fireEvent.click(screen.getByRole("button", { name: /下一步/ }));
    fireEvent.click(screen.getByRole("button", { name: /完成/ }));
    await waitFor(() => expect(client.signup).toHaveBeenCalledOnce());
    await waitFor(() => expect(onAuthed).toHaveBeenCalledWith(ME));
  });
});
```

(The first test's `getByDisplayValue("")` targets the first empty input — the email field. If the matcher is ambiguous in your RTL version, switch to querying inputs by order via `container.querySelectorAll("input")`.)

- [ ] **Step 3: Run it**

Run: `pnpm --filter web test -- src/shell/auth/AuthScreen.test.tsx`
Expected: PASS. (`AppShell` will not compile until Task 12 updates the `onEnterApp`→`onAuthed` call site — that is expected and fixed in Task 12; run only this test file here.)

- [ ] **Step 4: Commit**

```bash
git add apps/web/src/shell/auth/AuthScreen.tsx apps/web/src/shell/auth/AuthScreen.test.tsx
git commit -m "feat(p2): wire AuthScreen to real signin/signup

Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>"
```

---

### Task 12: Frontend — AppShell boot gate, session.user, logout, settings profile

**Files:**
- Modify: `apps/web/src/shell/session.ts`, `apps/web/src/shell/AppShell.tsx`, `apps/web/src/shell/settings/SettingsView.tsx`
- Modify: `apps/web/src/shell/AppShell.test.tsx`, `apps/web/src/shell/session.test.ts`, `apps/web/src/shell/settings/SettingsView.test.tsx`

**Interfaces:**
- Consumes: `api.getMe`, `api.signout` (T10); `AuthScreen` with `onAuthed` (T11); `MeUser`.
- Produces: `session.user: MeUser | null` + `session.setUser(u)`; `AppShell` boot gate (getMe → authed); `SettingsView` shows the real profile + logout via `api.signout`.

- [ ] **Step 1: Add user to the session store**

In `apps/web/src/shell/session.ts`: import `MeUser`, add `user` to the in-memory state (not persisted — boot re-validates via getMe), and add `setUser`. Update the `Session` interface, `createSession`, and `useSession` consumers:

```ts
import type { MeUser } from "../api";
// ... keep existing zod Session for the persisted {authed, aiAvatar} ...

export interface SessionStore {
  getSnapshot(): Session;
  subscribe(listener: () => void): () => void;
  setAuthed(v: boolean): void;
  setAvatar(color: string): void;
  getUser(): MeUser | null;
  setUser(u: MeUser | null): void;
}

export function createSession(opts: { storage: RawStorage }): SessionStore {
  let state: Session = load(opts.storage);
  let user: MeUser | null = null;
  const listeners = new Set<() => void>();

  function commit(next: Session): void {
    const changed = next.authed !== state.authed || next.aiAvatar !== state.aiAvatar;
    if (!changed) return;
    state = next;
    opts.storage.setItem(SESSION_KEY, JSON.stringify(state));
    listeners.forEach((l) => l());
  }

  return {
    getSnapshot: () => state,
    subscribe(l) { listeners.add(l); return () => { listeners.delete(l); }; },
    setAuthed(v) { commit({ ...state, authed: v }); },
    setAvatar(color) { commit({ ...state, aiAvatar: color }); },
    getUser: () => user,
    setUser(u) { user = u; listeners.forEach((l) => l()); },
  };
}
```

- [ ] **Step 2: AppShell boot gate**

In `apps/web/src/shell/AppShell.tsx`: on mount, call `api.getMe()`; on success `session.setUser(u)` + `setAuthed(true)`; on failure `setAuthed(false)`. Show a neutral loading state while pending. Change the `AuthScreen` render to pass `onAuthed`. Pass the resolved user to `SettingsView`. Use an injectable `client` default `api` so tests control boot.

```tsx
import { useEffect, useState } from "react";
import { api as defaultApi, type ApiClient, type MeUser } from "../api";
// ... existing imports ...

export function AppShell({
  store = defaultStore,
  session = defaultSession,
  client = defaultApi,
}: {
  store?: Store;
  session?: SessionStore;
  client?: Pick<ApiClient, "getMe" | "signout">;
}) {
  const sess = useSession(session);
  const [booted, setBooted] = useState(false);

  useEffect(() => {
    let cancelled = false;
    client.getMe()
      .then((u: MeUser) => { if (!cancelled) { session.setUser(u); session.setAuthed(true); } })
      .catch(() => { if (!cancelled) session.setAuthed(false); })
      .finally(() => { if (!cancelled) setBooted(true); });
    return () => { cancelled = true; };
  }, [client, session]);

  const screen = sess.authed ? "app" : "auth";

  // ... existing tab/taskView state ...

  if (!booted) {
    return <div style={{ width: "100%", height: "100%", background: "#F3F4F8" }} />;
  }
  if (screen === "auth") {
    return <AuthScreen onAuthed={(u) => { session.setUser(u); session.setAuthed(true); }} />;
  }

  // ... existing app layout; change the settings render to: ...
        {tab === "settings" && (
          <SettingsView
            session={session}
            user={session.getUser()}
            onLogout={() => { void client.signout().finally(() => { session.setUser(null); session.setAuthed(false); }); }}
          />
        )}
```

- [ ] **Step 3: SettingsView real profile**

In `apps/web/src/shell/settings/SettingsView.tsx`, accept `user?: MeUser | null` and render its `display_name`/`email` (falling back to the existing placeholder text when null), and keep the existing `onLogout` button wired to the prop. Change the profile block's hardcoded `Phoebe Chen` / `phoebe@ibschool.edu` to `user?.display_name` / `user?.email` with the current strings as fallback:

```tsx
import type { MeUser } from "../../api";

export function SettingsView({
  session,
  onLogout,
  user = null,
}: {
  session: SessionStore;
  onLogout: () => void;
  user?: MeUser | null;
}) {
  // ...
  // profile name/email:
  //   <div ...>{user?.display_name ?? "Phoebe Chen"}</div>
  //   name input:  value={user?.display_name ?? "Phoebe Chen"} readOnly
  //   email input: value={user?.email ?? "phoebe@ibschool.edu"} readOnly
```

(Convert the two profile `defaultValue` inputs to `value=... readOnly` so they reflect the real user without an unmanaged-input warning. Leave the toggles/avatar UI unchanged.)

- [ ] **Step 4: Update the shell tests**

- `session.test.ts`: add a case that `setUser`/`getUser` round-trips a `MeUser` and notifies subscribers.
- `AppShell.test.tsx`: the existing test mocks `../api`; extend the mock to include `getMe`/`signout` and drive the gate. Replace it with:

```tsx
import { describe, it, expect, vi } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import { AppShell } from "./AppShell";
import { createStore } from "../store/createStore";
import { createSession } from "./session";

const ME = { id: "u1", email: "p@d.local", display_name: "Phoebe", role: "student", avatar_color: "#7C9CF0", school: { id: "s1", name: "Demo" }, classes: [] };
const mem = () => { let s = "{}"; return { getItem: () => s, setItem: (_: string, v: string) => { s = v; } }; };

describe("AppShell boot gate", () => {
  it("shows the directory when getMe succeeds", async () => {
    const store = createStore({});
    const session = createSession({ storage: mem() });
    const client = { getMe: vi.fn(async () => ME), signout: vi.fn(), listTasks: vi.fn(async () => []), createTask: vi.fn() };
    render(<AppShell store={store} session={session} client={client as never} />);
    await waitFor(() => expect(screen.getByText("今天你在尝试什么？")).toBeInTheDocument());
  });

  it("shows AuthScreen when getMe rejects (401)", async () => {
    const store = createStore({});
    const session = createSession({ storage: mem() });
    const client = { getMe: vi.fn(async () => { throw new Error("401"); }), signout: vi.fn() };
    render(<AppShell store={store} session={session} client={client as never} />);
    await waitFor(() => expect(screen.getByText("登录")).toBeInTheDocument());
  });
});
```

  Keep the `vi.mock("../api", ...)` block but ensure it also exports `api` with `listTasks`/`createTask` for the `DirectoryView` mount, and re-exports `ApiError`/types used by children. (Simplest: `vi.mock("../api", async (orig) => ({ ...(await orig()), api: { ...realApiMock } }))` — or mock only what the children import.)

- `SettingsView.test.tsx`: update the render to pass `user={ME}` and assert the email/name show; assert clicking logout calls `onLogout`.

- [ ] **Step 5: Run the full web gate**

Run: `pnpm -r typecheck && pnpm -r test`
Expected: PASS (all suites). Then `pnpm -C apps/web build` — clean (chunk-size warning is pre-existing/acceptable).

- [ ] **Step 6: Commit**

```bash
git add apps/web/src/shell/session.ts apps/web/src/shell/AppShell.tsx apps/web/src/shell/settings/SettingsView.tsx apps/web/src/shell/AppShell.test.tsx apps/web/src/shell/session.test.ts apps/web/src/shell/settings/SettingsView.test.tsx
git commit -m "feat(p2): AppShell getMe boot gate + real profile + logout

Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>"
```

---

## Final verification (after Task 12)

- [ ] **Full backend gate:** `cd apps/api && go vet ./... && DOCKER_HOST=unix:///var/run/docker.sock go test -p 1 ./...` → all PASS.
- [ ] **Full web gate:** `pnpm -r typecheck && pnpm -r test && pnpm -C apps/web build` → all PASS.
- [ ] **Manual smoke (optional):** with a real DeepSeek key in `apps/web/.env.local` and `COOKIE_SECURE=false`, run `make migrate-up` + `make run` (api) and `pnpm -C apps/web dev`; sign in as `phoebe@demo.mindimprint.local` / `phoebe-dev-pass`; confirm the directory loads and the Phoebe aorta runs.
- [ ] Update `docs/遗留项追踪_Carryforward.md` and the slice/refactor memory: P2 done; carry-forward = Mailer/email, CSRF token, rate limiter, teacher/admin (P3), async eval (P4).

## Carry-forward (deferred, NOT bugs)

- Mailer package + real email send (verify flow dormant; accounts auto-verify at signup).
- CSRF double-submit token; per-IP/per-email rate limiter on signup/signin/verify.
- Teacher/admin self-service, roster management, school aggregation (P3).
- Async evaluation via river (P4).
- `CORS` AllowedMethods omits `PATCH` (pre-existing; the card-activate PATCH is best-effort fire-and-forget — note if it surfaces).
- `go test ./...` MUST use `-p 1` (parallel testcontainers exhaust Docker).
```

