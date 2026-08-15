# Course Runtime — Slice 8: Student-Platform Integration + Retire Old Player

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Wire the course-runtime packages into `apps/web`: serve/store a CourseDefinition 2.0 package, persist a `CourseSession`, provide a CDN `AssetResolver` + API `SessionAdapter` + the Slice 7 `ApiSceneGenerator`, mount the `@mind-imprint/course-renderer` `CoursePlayer` in the course tab, and seed the golden coverage course as a real 2.0 course so the player has content.

**Architecture:** The student app supplies the three production adapters and hands them to `CoursePlayer` (Slice 3). CourseDefinition 2.0 documents are stored in a new `course_definition` jsonb column on the `course` table and served by `GET /api/v1/courses/{slug}/definition`. `CourseSession` is a new `course_session` table storing the session JSON blob per (user, course); the `ApiSessionAdapter` snapshots the whole session (the runtime holds authoritative state client-side, so snapshot-save is sufficient and simplest). Assets resolve to signed OSS URLs via the existing `resolveUrl` seam.

> **DECISION — old-player cutover (default chosen; confirm with the user if reachable).** The user asked to "replace now." Existing authored courses are the OLD `render_cache` shape and have no 2.0 definition, so a blind hard-swap would break them. **Default:** the course container routes PER COURSE — a course WITH a `course_definition` plays through the NEW runtime; a course without one falls back to the legacy `CoursePlayer` until the (out-of-scope) generator emits 2.0. This honors "replace now" for every 2.0 course while not breaking legacy content. If the user prefers a hard cutover (delete the legacy player, accept that legacy courses stop playing until re-authored), collapse Task 6 to remove the legacy branch. **Surface this choice at dispatch.**

**Tech Stack:** Go (goose migration, sqlc, handlers), `apps/web` React + the three runtime packages.

**Authoritative spec:** design doc §16 (CourseSession), §17.16 (adapters); integration map `docs/2026-08-16-course-runtime-integration-surface.md`.

## Global Constraints

- Reuse existing patterns: goose migrations in `apps/api/internal/store/migrations` (next ordinal after the current highest — CHECK the directory, do not hardcode); sqlc regen with `CGO_ENABLED=0 go tool sqlc generate` (pin the sqlc version already used); handlers mirror `course.go`; signed URLs via the existing OSS resolve used by `oss.go`.
- The stored 2.0 document must pass `validateCourseDefinition` (Slice 1) before serving — validate on read (or on seed) and 422 a malformed one rather than shipping it to the player.
- `ApiSessionAdapter` implements the Slice 2 `SessionAdapter` interface EXACTLY (read `packages/course-runtime/src/adapters.ts` for the method set). Prefer a minimal server surface: `create` (get-or-create) + `save` (full snapshot); map the finer-grained adapter methods onto snapshot-save client-side if the interface has them.
- Go tests: `cd apps/api && go test ./... -timeout 1800s`, FOREGROUND. Frontend: `pnpm --filter web test` + `pnpm --filter web typecheck`.
- No `/dev/null` redirects; never `git add -A`. Deploy is a separate, later step (do NOT deploy in this plan).

---

### Task 1: `course_definition` storage + migration

**Files:** Create a new goose migration (next ordinal) adding `course.course_definition jsonb` (nullable). Modify `apps/api/internal/store/queries/course.sql` (add `GetCourseDefinition`, extend upsert). Regenerate sqlc. Test in `apps/api/internal/store` or via the endpoint test in Task 2.

- [ ] **Step 1: Write the migration** (`ALTER TABLE course ADD COLUMN course_definition jsonb;`) + a `-- name: GetCourseDefinition :one` selecting `course_definition` by slug.
- [ ] **Step 2: Regenerate sqlc**, run `go build ./...`.
- [ ] **Step 3: Commit** — `feat(api): course_definition column + query`.

---

### Task 2: `GET /courses/{slug}/definition` endpoint

**Files:** Modify `apps/api/internal/api/course.go` (or new `course_definition.go`) + `api.go` route. Test `apps/api/internal/api/course_definition_test.go`.

**Interfaces:** handler returns `{ definition: <CourseDefinitionDocument> }` (the raw stored jsonb) for a course that has one; 404 if the course has no 2.0 definition. Behind `protected`.

- [ ] **Step 1: Failing test** — seed a course row with a valid 2.0 `course_definition` (use the golden coverage JSON from Task 5, or an inline minimal one); GET returns it; a course without a definition → 404; unauth → 401.
- [ ] **Step 2: Run → FAIL** (`cd apps/api && go test ./internal/api/ -run Definition -timeout 1800s`).
- [ ] **Step 3: Implement** + route `GET /api/v1/courses/{slug}/definition`.
- [ ] **Step 4: Run → PASS.** — [ ] **Step 5: Commit** — `feat(api): serve course definition 2.0`.

---

### Task 3: `course_session` table + endpoints

**Files:** New goose migration for `course_session` (`id uuid pk, user_id, course_id, session jsonb, status text, created_at, updated_at`, unique `(user_id, course_id)`); queries in a new `course_session.sql` (`GetOrCreateCourseSession`, `SaveCourseSession`); regen sqlc. Handlers in `apps/api/internal/api/course_session.go` + routes. Test `course_session_test.go`.

**Interfaces:**
- `POST /api/v1/courses/{slug}/session` → get-or-create → `{ session: CourseSession }`.
- `PUT /api/v1/courses/{slug}/session` body `{ session: CourseSession }` → snapshot-save → `{ ok: true }`. Owner-scoped (session belongs to the authed user).

- [ ] **Step 1: Failing test** — create → returns a `created` session; PUT a mutated session → GET/create again returns the saved blob; a second user cannot read the first's session.
- [ ] **Step 2: Run → FAIL.** — [ ] **Step 3: Implement.** — [ ] **Step 4: Run → PASS.** — [ ] **Step 5: Commit** — `feat(api): course session persistence`.

---

### Task 4: Frontend adapters — AssetResolver, ApiSessionAdapter

**Files:** Create `apps/web/src/course/assetResolver.ts`, `apps/web/src/course/apiSessionAdapter.ts`, `apps/web/src/api/courseDefinition.ts` (client fns). Test `apps/web/test/course/apiSessionAdapter.test.ts`.

**Interfaces:**
- `makeCdnAssetResolver(baseUrl or resolveFn): AssetResolver` — maps a course's relative asset path to a signed/absolute URL (reuse the existing `resolveUrl` seam; the course package's assets live under an OSS prefix per course slug — resolve `assets/...` against it).
- `makeApiSessionAdapter(slug): SessionAdapter` (implements the Slice 2 interface): `create` → `POST session`; the mutating methods (`appendEvent`/`saveSliceState`/`saveScene`/`setStatus`) update a locally-held `CourseSession` and debounce a `PUT session` snapshot. Read `packages/course-runtime/src/adapters.ts` for the exact method names/signatures and implement all of them.
- `getCourseDefinition(slug)` client → `GET /courses/{slug}/definition`.

- [ ] **Step 1: Failing test** — mock the client; `create` returns the server session; calling a mutating method then flushing issues a `PUT` with the updated blob; determinism (ids/timestamps injected).
- [ ] **Step 2: Run → FAIL.** — [ ] **Step 3: Implement.** — [ ] **Step 4: Run → PASS** + typecheck. — [ ] **Step 5: Commit** — `feat(web): course cdn asset resolver + api session adapter`.

---

### Task 5: Seed the golden coverage course as a 2.0 course

**Files:** Modify `apps/api/internal/store/seed_courses.go` (or add a seed) to insert one `course` row whose `course_definition` is the golden coverage CourseDefinition 2.0 (reuse the Slice 1 coverage fixture JSON — copy it into a Go embed under the api, since the contract fixture is TS test-only). Test: the seed produces a course whose definition passes structural read.

- [ ] **Step 1: Failing test** — after seeding, `GetCourseDefinition(slug)` returns a non-null document that round-trips as valid JSON with `schemaVersion:"2.0"`.
- [ ] **Step 2–4:** implement seed + `go:embed` the JSON; run → PASS.
- [ ] **Step 5: Commit** — `feat(api): seed golden 2.0 course`.

---

### Task 6: Mount the new CoursePlayer in `apps/web` (per-course routing)

**Files:** Modify `apps/web/src/shell/courses/CoursesContainer.tsx` (route to new vs legacy player), create `apps/web/src/shell/courses/RuntimeCoursePlayer.tsx` (wraps `@mind-imprint/course-renderer` `CoursePlayer` with the three adapters + `ApiSceneGenerator` from Slice 7 + injected `idFactory`/`clock`). Keep the legacy `CoursePlayer.tsx` for courses lacking a 2.0 definition (see DECISION above). Test `apps/web/test/shell/courses/runtimeCoursePlayer.test.tsx`.

**Interfaces:** `RuntimeCoursePlayer({ slug, onExit, onFinish })` — fetches the definition, builds adapters (`makeCdnAssetResolver`, `makeApiSessionAdapter(slug)`, `makeApiSceneGenerator(slug)`), supplies `idFactory` (e.g. crypto.randomUUID) + `clock` (Date-based — this is the HOST boundary where real time enters; the packages stay pure), and mounts the runtime `CoursePlayer`. On completion → `onFinish`.

- [ ] **Step 1: Failing test** — with a mocked `getCourseDefinition` returning the golden course + mocked adapters, `RuntimeCoursePlayer` renders the Opening and, after driving to the end, calls `onFinish`; `CoursesContainer` mounts `RuntimeCoursePlayer` for a course with a definition and the legacy player for one without (mock the definition fetch to 404).
- [ ] **Step 2: Run → FAIL.** — [ ] **Step 3: Implement.** — [ ] **Step 4: Run → PASS** + typecheck; run the FULL web suite to ensure no regressions in the course tab. — [ ] **Step 5: Commit** — `feat(web): mount runtime course player with per-course routing`.

---

## Self-Review Notes

- **"Load and show" scope honored:** this slice makes the student platform load a 2.0 package and play it, with real session persistence and adapters. Authoring/generation of 2.0 courses stays out of scope — hence seeding one golden course so there is content.
- **Old-course safety:** per-course routing prevents the "replace now" instruction from breaking every existing authored course. Flagged as the one decision to confirm with the user; the alternative (hard cutover) is a one-task simplification.
- **Purity boundary:** `apps/web` is where real `Date`/`crypto` enter, injected as `clock`/`idFactory` into the pure packages — the determinism rule holds everywhere below the app.
- **Deferred:** deploy (separate step); migrating existing authored courses to 2.0 (needs the generator, out of scope); richer per-event server persistence (snapshot-save is sufficient for load+show).
