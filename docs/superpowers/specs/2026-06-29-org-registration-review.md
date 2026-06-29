# Org & Registration — Gap Review

> **Authored:** 2026-06-29. **Status:** review complete — no new design decisions taken (by decision).
> Covers discussion-queue topic **#4**. This is a *review* of the already-built P2 + P3 org/auth subsystem,
> not a redesign. **North-star:** `docs/superpowers/specs/2026-06-24-backend-platform-architecture-design.md`.

## 1. Verdict

P2 + P3 (auth + organization, teacher/admin consoles) is **production-grade and structurally sound**. The org
invariant — *every account belongs to a school; students/teachers also belong to ≥1 class; no org-less accounts* — is
enforced at **both** the DB and app layers. No new design decisions are warranted this round; the three seams below are
logged as carry-forward.

## 2. What was verified (built & sound)

- **Org model + invariant (DB-enforced).** `users.school_id NOT NULL REFERENCES schools` and
  `classes.school_id NOT NULL REFERENCES schools` (`migrations/0001_init.sql`); `enrollments` unique on
  `(user_id, class_id)` with role-in-class CHECK. Invariant holds at the schema level, not just app code.
- **Atomic registration.** `POST /auth/signup` resolves `school_id` from the teacher invite or class join code and
  creates user + enrollment (or consumes the invite) in **one transaction**, rolling back on any failure
  (`signup.go`). Join code is validated; duplicate email → 409.
- **Auth.** argon2id PHC hashing with constant-time verify (`auth/password.go`); SHA-256-hashed session tokens in a
  `sessions` table; `mk_session` HttpOnly+SameSite=Lax cookie, 30-day TTL, Secure config-gated. Login blocks on
  unverified email (`signin.go`).
- **RBAC.** `role` CHECK constraint (`student|teacher|admin`); `RequireRole` route guards + resource-level
  `assertAdminOfSchool` / `assertTeacherOwnsClass`; cross-tenant probes return **404, not 403** (existence-hiding).
- **P3 consoles.** Teacher invites, class CRUD + roster, teacher assignment, CSV import, school overview/usage-by-tier —
  all implemented and routed (`invites.go`, `classes.go`, `class_teachers.go`, `import.go`, `overview.go`).
- **`HasEntitlement` seam.** Stubbed open (returns true) **and already wired** into the token-spending endpoints
  (turn + evaluate → 403 `not_entitled`). Future billing slots in without touching call sites.
- **Secrets.** LLM keys / DB DSN read from server env only (`config.go`), never sent to the client; errors sanitized to
  a generic 500 with server-side logging (`httpx/errors.go`). No leakage.

## 3. Carry-forward seams (logged, not decided)

1. **Email verification is un-built behind a live handler.** The `verify-email` handler is live and tested, but P2
   auto-verifies at signup — nothing mints a token at signup, there is no SMTP config, and no mailer exists. *Open
   question for when it's picked up:* in a school-vouched (join-code/invite) model, does email verification earn its
   keep, or is the school's vouching sufficient? Decide before building a mailer.
2. **Student enrollment lifecycle is thin.** A student gets exactly one enrollment, at signup. There is no surfaced
   *join-another / leave / transfer* path, and **no confirmed guard** preventing enrollment into a class outside the
   user's `school_id` (a cross-school enrollment would break org coherence). If multi-class or post-signup joining is
   ever wanted, add the guard at that time.
3. **Session cleanup is unautomated.** No background job purges expired sessions. Auth stays *correct* via read-time
   `expires_at` checks; this is storage tidiness only.

## 4. Non-goals

- ❌ No new org/auth feature work this round — review only.
- ❌ No mailer / SMTP build, no enrollment-lifecycle endpoints, no session-GC job — all deferred (§3).
