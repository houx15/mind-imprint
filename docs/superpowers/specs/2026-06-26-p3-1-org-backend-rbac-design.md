# P3.1 · Org Backend + RBAC — Design Spec

> **Phase:** P3.1 (first sub-phase of P3 「完整组织端」). Backend-only.
> **Authored:** 2026-06-26. **Status:** approved, ready for plan.
> **North-star:** `docs/superpowers/specs/2026-06-24-backend-platform-architecture-design.md`;
> schema `docs/architecture/database-schema.md`; API `docs/architecture/api-design.md`.
>
> P3 is decomposed into **P3.1 org backend + RBAC** (this spec), **P3.2 teacher
> console** (frontend), **P3.3 admin console** (frontend). Each is its own
> spec→plan→build cycle. This spec covers P3.1 only.

## Goal

A Go backend organization layer on top of P2 auth: role-based authorization,
admin-seeded bootstrap, teacher invite-code signup, teacher/admin-created classes,
admin bulk-import of org structure, and **aggregate-only** roster/school views. No
frontend in this phase — fully demoable via the HTTP API and exercised by tests.

## Scope decisions (locked during brainstorming)

1. **Admin accounts** are bootstrapped by a seed migration (no role above admin, so
   no self-service admin creation). One admin seeded into the demo school.
2. **Teacher accounts**: an admin mints a single-use, school-scoped **invite code**;
   the teacher self-signs-up with it, reusing the entire P2 signup/verify machinery.
   Teacher sets their own password.
3. **Student accounts**: unchanged P2 class-join-code signup.
4. **Classes**: teachers create & own their own classes; admins may also create
   classes school-wide and assign a teacher.
5. **Bulk import** (admin): uploading a teacher-student table creates classes +
   teacher invites + class join codes, returning a distributable code sheet.
   Everyone still self-signs-up with their code. Imported **student rows create no
   per-student account or DB artifact** — the class join code is the binding
   mechanism. (The option-C "expected enrollment matched by email" mechanism was
   explicitly **not** chosen.)
6. **Data visibility**: **aggregate / metadata only**. Teacher sees roster +
   per-student activity *signals* (counts, timestamps), never contents. Admin sees
   school-level rollups (usage/cost via the `llm_usage` view + counts). Reading the
   actual process tree / evaluation narrative is **out of scope** for P3.1 — it
   defers to a later phase aligned with the still-provisional evaluation data model.
   This keeps eval-model coupling out of P3.1.

## Non-goals (P3.1)

- No frontend (teacher/admin consoles are P3.2/P3.3).
- No per-student work-detail reads (process tree, transcript, eval narrative).
- No CSV *parsing* in the backend — the import endpoint takes already-parsed JSON
  rows; file parsing lives in the P3.2 frontend.
- No class soft-archive (deferred; remove-student covers roster correction).
- No real email (Mailer still stubbed from P2); no rate-limiting (deferred from P2).
- No new evaluation/billing data model (those remain their own deferred brainstorms).

---

## Data model

### New table — `teacher_invites` (migration `0005_org.sql`)

| column | type | notes |
|---|---|---|
| `id` | uuid PK | |
| `school_id` | uuid FK → `schools(id)` on delete restrict, NOT NULL | invite is school-scoped |
| `code` | text | **unique**, plaintext, `T-` prefixed, distributable like `classes.join_code` |
| `email` | text null | optional pre-bind: if present, signup email must match |
| `created_by` | uuid FK → `users(id)` | the admin who minted it |
| `expires_at` | timestamptz | TTL (default 14 days) |
| `consumed_at` | timestamptz null | set on successful teacher signup; single-use |
| `consumed_by` | uuid FK → `users(id)` null | the teacher who consumed it |
| `created_at` | timestamptz | |

Index: `(code)` unique. Index: `(school_id)`. "Active" = `consumed_at IS NULL AND
expires_at > now()` (filtered in SQL, mirrors `email_verification_tokens`).

### Schema deltas (same migration)

- `classes` += `created_by uuid null FK → users(id)` (who created the class).
- No new columns on `users` (`role` exists) or `enrollments` (`role_in_class` exists).

### `llm_usage` view (same migration)

Add the read-only view defined in `database-schema.md` (unions assistant `messages`
rows and `evaluations` rows, joined to `users.school_id`). Powers admin aggregation
without a new ledger table.

---

## Account provisioning & signup

### Admin seed (migration `0006_seed_admin.sql`)

Seed one admin into the existing demo school: real argon2id `password_hash`
(generated via the `tools/genhash` dev tool, as seed-Phoebe was in P2), documented
dev credentials, `role='admin'`, `school_id` = demo school, **no enrollment**
(admins map to no class). Reversible `Down` blanks the hash → no-login.

### Signup extension (one endpoint, one `join_code` field)

`POST /api/v1/auth/signup` keeps its single `join_code` field. The server resolves
the submitted code by trying `teacher_invites` **first**, then `classes.join_code`:

- **Active teacher invite** → create `user{role:'teacher', school_id from invite}`,
  **no enrollment**, consume the invite (`consumed_at`/`consumed_by`) — one tx. If
  the invite carries an `email`, it must equal the (lowercased) signup email, else
  `400 invalid_join_code` (no enumeration of which check failed).
- **Class join code** → existing student path (`user{role:'student'}` + enrollment),
  unchanged.
- **Neither** → `400 invalid_join_code`.

The `T-` prefix keeps the two code namespaces visually distinct and collision-free.
Auto-verify (P2 behavior) still applies: `email_verified_at = now()` at signup.

---

## Authorization model (`internal/api/authz.go`)

The session context already carries `role`. Two layers:

- **Coarse role gate** — `RequireRole(roles ...string) middleware` → `403 forbidden`
  (distinct from `RequireUser`'s `401 unauthorized`). Wraps role-restricted groups.
- **Per-resource tenancy guards** — helper funcs called inside handlers:
  - `assertTeacherOwnsClass(ctx, classID)` — passes if the caller has an
    `enrollments` row for the class with `role_in_class='teacher'`, OR the caller is
    an admin of the class's school.
  - `assertAdminOfSchool(ctx, schoolID)` — caller `role='admin'` AND
    `school_id == schoolID`.
  - Cross-tenant / not-owned access → `404 not_found` (never leak existence).
    Admin is scoped to **their own school**, never global.

---

## Endpoints

All protected (require a session). Base path `/api/v1`.

### Classes & roster (teacher or admin)

- **`POST /classes`** — create. Teacher: `school_id` = own, auto-enroll creator as
  `role_in_class='teacher'`, generate `join_code`, set `created_by`. Admin: same,
  may pass `teacher_user_id` to assign + enroll that teacher (the teacher must belong
  to the admin's school). `201 {class}`.
- **`GET /classes`** — teacher: own classes; admin: school-wide. `200 {classes:[…]}`.
- **`GET /classes/:id`** — class + roster. Roster = enrolled students with
  `display_name, email, last_active_at, task_count, evaluation_count, card_count`
  (aggregate signals only, no contents). Tenancy-guarded. `200 {class, roster:[…]}`.
- **`PATCH /classes/:id`** — `{name?, regenerate_join_code?}`. Rename and/or rotate
  the join code. Tenancy-guarded. `200 {class}`.
- **`DELETE /classes/:id/enrollments/:userId`** — remove a student from the class
  (deletes the `enrollment` row only; never the user account or their tasks).
  Tenancy-guarded. `204`.

### Admin-only (`RequireRole("admin")`)

- **`POST /admin/teacher-invites`** — `{email?, expires_days?}` → `201 {code,
  expires_at}`. School = admin's own.
- **`GET /admin/teacher-invites`** — list active invites. `200 {invites:[…]}`.
- **`POST /admin/import`** — body = parsed rows
  `{rows:[{class, teacher_email?, student_email?}]}` (frontend parses the CSV later;
  the backend takes JSON to stay free of CSV-format coupling). Idempotent per
  (school, class name): creates missing classes + join codes, mints teacher invites
  for distinct `teacher_email`s. Returns a **code sheet**
  `{classes:[{name, join_code}], teacher_invites:[{email, code}]}`. One tx;
  partial failure rolls back with a row-indexed error `400 validation_failed`
  (`details.row`).
- **`GET /admin/overview`** — school aggregation from `llm_usage` + counts: totals
  (tokens/cost by tier), per-class and per-teacher breakdown, active-student counts,
  task/evaluation counts. **Counts + cost only**, no eval contents. `200 {overview}`.

### Unchanged

`GET /auth/me` already returns `role` + `classes[]` (teachers get several, admins
none) — no change.

---

## Error handling

Reuse the `httpx` error envelope. Add a `forbidden` (403) constructor. Tenancy
failures map to `404 not_found`. Import validation → `400 validation_failed` with
`details.row`. No internals in client errors; real detail to `slog` server-side.

## Testing

Established pattern: testcontainers Postgres, `go test -p 1`. Coverage per endpoint:

- **Happy path** for each route.
- **Authz matrix**: student → `403`; wrong-school teacher → `404`; admin
  cross-school → `404`; unauthenticated → `401`.
- **Teacher invite**: consume, expire, email-mismatch, already-consumed.
- **Signup resolution**: teacher-code path vs class-code path vs invalid.
- **Import**: idempotency (re-run is a no-op on existing classes), partial-failure
  rollback, code-sheet shape.
- **Aggregation**: counts/cost correct, scoped to the admin's school only.

New `maintest` helpers: `signInTeacher` / `signInAdmin` returning session cookies
from seeded accounts (an admin from the seed migration; a teacher created via the
invite+signup path in the test).

## Carry-forward (deferred, recorded — to `docs/遗留项追踪_Carryforward.md`)

- Class soft-archive (`archived_at` column reserved if added; remove-student covers
  roster correction for now).
- Per-student work-detail visibility (process tree / transcript / eval narrative) —
  waits on the evaluation-data-model brainstorm.
- CSV *parsing* (P3.2 frontend); teacher-invite `email` enforcement is advisory
  unless the invite was minted with an email.
- Rate-limiting + CSRF double-submit token (still deferred from P2).
- Teacher/admin **frontend consoles** = P3.2 / P3.3.
