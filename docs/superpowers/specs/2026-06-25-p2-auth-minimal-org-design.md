# P2 · Auth + Minimal Org — Design

> **Phase spec.** Replace the P1 dev `ActAsSeed` shim with real, DB-backed
> cookie-session authentication and wire the existing `AuthScreen`. Authored
> 2026-06-25. Sub-project of the
> [backend platform architecture north-star](2026-06-24-backend-platform-architecture-design.md);
> consumes the contracts in [api-design.md](../../architecture/api-design.md) and
> [database-schema.md](../../architecture/database-schema.md). Builds on P1.1–P1.4
> (Go service, agent brain + gateway, HTTP API + inline eval + entitlement, frontend
> cutover), all merged to `main`.

## Goal

A student can **sign up with a class join code, sign in, and stay signed in via a
session cookie**, and every token-consuming endpoint runs as that real user instead
of the seeded dev user. After P2 the Phoebe acceptance aorta runs through a real
login (no `ActAsSeed`), and the org invariant (every account belongs to a school; a
student also to a class) is enforced atomically at signup.

## Scope frame — what P1 already gave us

P1 shipped the org schema (`schools` / `classes` / `users` / `enrollments`, with the
NOT NULL `users.school_id` and `unique(user_id, class_id)`), the `WithUser` /
`UserFromContext` request-context seam, the `HasEntitlement(ctx, user)` stub gated
before `/turn` and `/evaluate`, the API error envelope, and the middleware chain
(`RequestID → Recover → Logger → CORS`). The frontend has `apiFetch` with
`credentials: 'include'` and a fully-built but pass-through `AuthScreen`.

P2 therefore adds: **two tables** (sessions, verification tokens), an `internal/auth`
package, the auth store queries + handlers, the session middleware (replacing
`ActAsSeed`), and the frontend auth wiring. No new org tables; no teacher/admin
self-service.

## Decisions locked in brainstorming

1. **Email verification — build the flow, auto-verify in P2.** The
   `email_verification_tokens` table, the token-hashing helper, and
   `POST /auth/verify-email` are built and Go-tested, but signup marks the account
   verified immediately (`email_verified_at = now()`) and sends **no** email. The
   verify endpoint is exercised by tests that insert a token row directly. The
   `Mailer` package and real email send are deferred (carry-forward) — when a
   provider lands, signup flips to "create token + send raw token, leave
   `email_verified_at` null" with no endpoint changes.
2. **Security hardening — Lax cookie only.** Session cookies are httpOnly + Secure
   (config-gated) + SameSite=Lax, which already blocks the cross-site-form CSRF
   vector. The **double-submit CSRF token** and the **rate limiter** are deferred
   (carry-forward).
3. **Seed + aorta — seed Phoebe with a real password; aorta logs in.** The seeded
   Phoebe (`…0003`) gets a real argon2id password hash for a known dev password and
   keeps `email_verified_at` set; the acceptance aorta signs in as her and her
   existing seeded task history is right there. Signup-via-join-code is exercised
   separately by tests. `ActAsSeed` is **deleted**.

## Data model — migration `0003_auth.sql`

Two new tables. Both store **only the SHA-256 hash** of their token; the raw token
exists only in the cookie / (future) email and is never persisted or logged.

```sql
CREATE TABLE sessions (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash  text NOT NULL UNIQUE,
    expires_at  timestamptz NOT NULL,
    user_agent  text,
    ip          inet,
    created_at  timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX idx_sessions_user_id ON sessions(user_id);

CREATE TABLE email_verification_tokens (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash  text NOT NULL UNIQUE,
    expires_at  timestamptz NOT NULL,
    consumed_at timestamptz,
    created_at  timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX idx_evt_user_id ON email_verification_tokens(user_id);
```

**Seed update — migration `0004_seed_password.sql`.** The P1 seed (`0002_seed.sql`)
gave Phoebe `password_hash = 'SEED_NO_LOGIN'`. A separate migration (kept distinct
from the `0003` table DDL) `UPDATE`s it to a real argon2id PHC hash of a known dev
password. Because the hash must match the live
`auth.HashPassword` encoding (and argon2id salts are random), the seed migration is
written with a **precomputed PHC string** generated once from the real helper and
pasted in (documented in the plan with the exact command to regenerate). The
`DEMO-0001` join code and Phoebe's `email_verified_at = now()` stay. Dev credentials:
`phoebe@demo.mindimprint.local` / a dev password recorded in the plan and the seed
file comment.

## Backend

### `internal/auth/` package (pure, no DB)
- `HashPassword(plain string) (string, error)` — argon2id, params `m=65536` (64 MiB),
  `t=1`, `p=4`, 16-byte random salt, 32-byte key; returns the standard PHC string
  `$argon2id$v=19$m=65536,t=1,p=4$<b64salt>$<b64hash>`.
- `VerifyPassword(plain, phc string) (bool, error)` — parse PHC, recompute, constant-
  time compare. Unknown/malformed PHC → error (not panic).
- `NewToken() (raw string, hash string, error)` — 32 random bytes → `raw` is
  base64url (no padding); `hash` is the SHA-256 hex of the raw token. Used for both
  session tokens and verification tokens.
- `HashToken(raw string) string` — SHA-256 hex, for lookups.

Table-driven unit tests: hash round-trip, wrong-password reject, salt uniqueness
(two hashes of the same password differ), malformed-PHC error, token raw≠hash and
hash determinism.

### Store queries (`store/queries/auth.sql`, sqlc-generated)
- `GetUserByEmail(email)` → user row (for signin / email-uniqueness).
- `GetClassByJoinCode(join_code)` → class row (for signup; not found → invalid code).
- `CreateUser(...)` and `CreateEnrollment(...)` — used **inside a single pgx
  transaction** in the signup handler (sqlc emits per-query funcs; the handler opens
  the tx, runs both with `q.WithTx(tx)`, commits — atomic org invariant). User is
  created with `school_id` taken from the class row, `role='student'`,
  `email_verified_at = now()`.
- `MarkEmailVerified(user_id)` and verify-token queries:
  `CreateEmailVerificationToken(...)`, `GetEmailVerificationTokenByHash(token_hash)`,
  `ConsumeEmailVerificationToken(id)`.
- `CreateSession(...)`, `GetSessionWithUserByHash(token_hash)` (join sessions→users,
  returns user fields + expires_at), `DeleteSessionByHash(token_hash)`.
- `ListClassesForUser(user_id)` and `GetSchool(school_id)` (for `/me`).

Email uniqueness relies on the existing `users.email` UNIQUE constraint: the signup
tx attempts the insert and maps a unique-violation (pg error code `23505`) to
`email_taken`, rather than a check-then-insert race.

### Handlers (`internal/api/`)
All requests/responses use the existing JSON + error-envelope helpers.

- **`signup.go` — `POST /auth/signup`** `{email, password, display_name, join_code}`
  → validate non-empty + password length floor (≥ 8); `GetClassByJoinCode` (not found
  → 400 `invalid_join_code`); `HashPassword`; open tx → `CreateUser` (school from
  class, verified) + `CreateEnrollment(role_in_class='student')`; unique-violation →
  409 `email_taken`; commit → **201 `{}`** (no auto-signin; the client then calls
  signin, matching api-design.md).
- **`signin.go` — `POST /auth/signin`** `{email, password}` → `GetUserByEmail`
  (not found → 401 `invalid_credentials`); `VerifyPassword` (false → 401
  `invalid_credentials`); `email_verified_at IS NULL` → 403 `email_unverified`;
  `auth.NewToken()` → `CreateSession(hash, expires_at=now+30d, user_agent, ip)`;
  `Set-Cookie mk_session=<raw>`; **200 `{user}`**.
- **`verify.go` — `POST /auth/verify-email`** `{token}` → `HashToken` →
  `GetEmailVerificationTokenByHash`; nil / consumed / expired → 400
  `token_invalid_or_expired`; else `ConsumeEmailVerificationToken` +
  `MarkEmailVerified`; **200 `{user}`**. Dormant in the P2 UI; covered by a
  direct-insert Go test.
- **`signout.go` — `POST /auth/signout`** → read cookie; `DeleteSessionByHash`
  (idempotent — missing cookie/session still 204); clear cookie; **204**.
- **`me.go` — `GET /auth/me`** → `UserFromContext` (RequireUser guarantees present) →
  assemble `{id, email, display_name, role, avatar_color, school:{id,name},
  classes:[{id,name,role_in_class}]}`; **200 `{user}`**.

### Middleware & routing
- **`SessionAuth`** (new, in `internal/api` or `httpx`) — reads the `mk_session`
  cookie; if present, `GetSessionWithUserByHash`; if found and not expired, builds the
  `User` and `WithUser`s it into the context. **Never rejects** on its own — absence
  just means no user in context. Replaces `ActAsSeed` at the same mount point.
- **`RequireUser`** (new) — wraps the protected group; `UserFromContext` absent →
  401 `unauthorized`.
- **Routing split.** The `mux` registers public auth routes
  (`/auth/signup`, `/auth/signin`, `/auth/verify-email`, `/auth/signout`) and a
  protected group (`/auth/me`, `/tasks…`, `/tasks/:id/turn`, `/cards…`,
  `/evaluate…`). `SessionAuth` runs for all (so signout can find the cookie);
  `RequireUser` wraps only the protected group. Implementation: register protected
  handlers through a small `protected(h)` helper = `RequireUser(h)`; the outer
  `Handler()` wraps the whole mux in `SessionAuth` (replacing the
  `ActAsSeed(...)(mux)` line).
- **`ActAsSeed` deleted**, along with `SeedUserID` if unused after the test-harness
  migration. The `User` struct, `WithUser`, `UserFromContext`, and `ctxKeyUser` stay.

### Cookie & config
- Cookie `mk_session`: `HttpOnly`, `SameSite=Lax`, `Path=/`, `MaxAge` = 30 days,
  `Secure` gated by a new config field. Value = the raw base64url session token.
- New config field **`CookieSecure bool`** (env `COOKIE_SECURE`, default `true`;
  set `false` for local http dev so the browser sends the cookie). No session signing
  secret is needed — tokens are opaque and validated against the DB.
- Session TTL constant (30 days) lives beside the handlers; mirrored into
  `sessions.expires_at`.

### Errors (`httpx/errors.go`)
Add constructors/codes: `email_taken` (409), `invalid_join_code` (400),
`email_unverified` (403), `token_invalid_or_expired` (400), `invalid_credentials`
(401). `unauthorized` (401) and `not_entitled` (403) already exist.

## Frontend

- **`src/api/auth.ts`** — `signup(body)`, `verifyEmail(token)`, `signin(body)`,
  `signout()`, `getMe()` over `apiFetch` (already `credentials:'include'`). Added to
  the `ApiClient` interface + default `api` so controllers/tests can inject.
- **`AppShell` boot gate** — on mount call `getMe()`: `unauthorized`/null → render
  `AuthScreen`; success → render the workspace with the real `user`. A signed-in
  user is held in shell state and passed where the seed user used to be implied.
- **`AuthScreen`** — wire the existing login / register / bind tabs:
  - Login → `signin` → on success store user + enter app; map `invalid_credentials`
    / `email_unverified` to inline messages.
  - Register (+ bind / join code) → `signup` → on success immediately `signin`
    (account is auto-verified) → enter app; map `email_taken` / `invalid_join_code`.
  - Loading + disabled states on submit. The hardcoded demo values may stay as
    placeholders but real input drives the calls.
- **Settings** — add a Signout action calling `api.signout()` → clear shell user →
  back to `AuthScreen`.

## Data flow — the new boot + auth lifecycle

1. **Boot** — `AppShell` calls `GET /auth/me`. 401 → `AuthScreen`.
2. **Signup** — `POST /auth/signup` (201) → client calls `POST /auth/signin`.
3. **Signin** — `POST /auth/signin` (200 + `Set-Cookie mk_session`) → client has the
   user; subsequent `apiFetch` calls carry the cookie automatically.
4. **Authed requests** — `SessionAuth` resolves the cookie → `RequireUser` passes →
   handlers read the real user (tasks scoped to `user.ID`, entitlement checked).
5. **Signout** — `POST /auth/signout` (204) → cookie cleared → boot gate shows
   `AuthScreen` next load.

## Error handling

Backend never leaks provider/internal detail (unchanged guarantee). Auth failures map
to the safe codes above; the frontend shows code-specific inline messages and a
generic fallback for `internal_error` / network failure. Token hashes, password
hashes, and raw tokens never appear in logs or error bodies.

## Testing

- **Go `auth` package** — unit tests per the package section above.
- **Go handlers (testcontainers)** — happy path **signup → signin → /me**; failure
  cases `email_taken`, `invalid_join_code`, `invalid_credentials`,
  `email_unverified` (insert an unverified user directly); a **direct-insert verify**
  test (insert token row → `verify-email` → 200, then reuse → 400); signout revokes
  (signin → authed call ok → signout → same cookie now 401). `RequireUser` returns
  401 with no cookie.
- **Test harness migration** — replace the `ActAsSeed`-based harness with a
  `signInAsSeed(t)` helper that creates a session for Phoebe and returns the cookie;
  update `TestE2EPhoebeVertical` to sign in first, then run the existing
  create→turn→card→refeed→evaluate→get aorta with the cookie.
- **Web** — `api/auth` client tests (mock fetch, assert method/path/body, error
  mapping); `AuthScreen` flow test (signin success → onEnter; error code → message;
  register → signup+signin); `AppShell` boot-gate test (getMe 401 → AuthScreen, 200 →
  workspace). Keep all existing card/view-model tests.

## Carry-forward / out of scope

Documented deferrals (not bugs):
- **Mailer package + real email send** — the verify flow is built and tested but
  dormant; accounts auto-verify at signup. Wire a `Mailer` (LogMailer dev →
  Aliyun/Tencent/Resend prod) when email matters; signup then creates a token and
  leaves `email_verified_at` null.
- **CSRF double-submit token** and **rate limiter** (per-IP/per-email on
  signup/signin/verify) — Lax cookies ship now; add in a hardening pass.
- **Teacher/admin self-service**, class roster management, school-dimension
  aggregation — **P3**.
- **Async evaluation via river** — **P4**.
- Password reset, "remember me" / session-list management, OAuth — later.
```

