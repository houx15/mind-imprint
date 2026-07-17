# A1 · Course Session Report Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Give `event` and `evaluations` a course-session scope so course evidence stops being unreachable, and make the course's 学习报告 carry a real, session-scoped growth assessment.

**Architecture:** Migration 0024 adds `session_id` to `event` + `evaluations` (the `event` CHECK goes on `NOT VALID` — pre-existing chat/course rows are permanently unattributable). `CourseStore.InsertUserEvent` — whose scopeless signature *is* the bug — is replaced by `InsertSessionEvent`. A new `buildAssessmentInputFromSession` feeds the **unchanged** `agent.Assess` over the **unchanged** CT rubric, behind two new endpoints that mirror the project's assessment pair. `CourseReport.tsx` gains the 能力评估 and 收集到的工具 blocks and stops claiming challenges were passed.

**Tech Stack:** Go (`net/http`, sqlc, goose, pgx/pgtype, testcontainers), TypeScript + React + vitest, Zod contracts.

**Spec:** `docs/superpowers/specs/2026-07-17-course-session-report-design.md`

## Global Constraints

- **Client never calls a model directly.** All LLM calls go through the backend gateway; API keys live only in `apps/api` server-side env. Secrets never enter git, logs, errors, migrations, or payloads.
- **RL-5 — the assessment is diagnostic evidence, never a grade or verdict.** No total, no rank, no aggregate score anywhere (engine, DTO, or UI). Every level carries its behavioral evidence.
- **铁律 2 (不操纵)** — no streaks, no leaderboards, no addictive mechanics.
- **DEC-A1.3 — do NOT touch the rubric or the assessor.** `agent.Assess`, `agent.AssessmentInput`, `agent.BuildAssessmentInput`, `rubric.CT()`, `ct-rubric.json`, the assess system prompt, `studio.AssessmentDTO`, and the `Assessment` Zod contract are all unchanged. Sub-project B replaces them.
- **No fabrication.** A designed block with no real data is omitted, not filled. `我的学习笔记` and `导出笔记` stay out (no note or export feature exists anywhere in the repo).
- **Binding design copy is verbatim.** `docs/design/思维印记_工作区.dc.html` wins over spec/plan for all UI: exact colours, spacing, and copy. Icons are inline SVG — **never** lucide-react.
- **`make sqlc` / `make sync-skills` run from `apps/api`.** Never hand-edit `apps/api/internal/store/sqlc/*` or `apps/api/internal/skills/specs/*`.
- **Go tests:** `CGO_ENABLED=0 go test -p 1 ./...` from `apps/api`, on a quiet Docker daemon. For card/gate/projection changes run **FULL packages, never `-run` subsets.**
- **Run web/contracts test commands from their OWN directory** — never a compound `cd`.
- **NEVER `git add` a whole directory.** Add exact paths only. The pre-existing `M package.json` and the untracked user files (`docs/03_课程库_单课设计/`, `docs/2026-07-06-spec.md`, `docs/astranova/`, `docs/design/Toddle风格网页设计-handoff.zip`, `pm-e2e-01-directory.png`, `walk-01-directory.png`) are NOT ours — leave them untouched.
- **Mock fidelity (the Slice 12 lesson).** Five of Slice 12's six Criticals came from mocks encoding shapes the backend cannot produce. Check every mock against the real contract: SSE frame shapes, DTO camelCase, and `completed_ordinals`' server-side origin. Distrust a green suite whose mocks you have not checked.

---

### Task 1: Migration 0024 — session scope on `event` and `evaluations`

**Files:**
- Create: `apps/api/internal/store/migrations/0024_session_assessment_scope.sql`
- Create: `apps/api/internal/store/migrate_0024_test.go`

**Interfaces:**
- Consumes: nothing (first task).
- Produces: columns `event.session_id` and `evaluations.session_id` (both `uuid NULL REFERENCES course_session(id) ON DELETE CASCADE`); constraints `event_scope_ck` (NOT VALID) and the widened `evaluations_scope_ck`; indexes `event_session_created_idx`, `evaluations_session_idx`.

- [ ] **Step 1: Write the failing test**

Create `apps/api/internal/store/migrate_0024_test.go`. Note this file is `package store` (internal, like `migrate_0021_test.go`) so it can reach `migrationFS` for the Down test. `newTestPool` and `refactor2SeededStudentID` come from the existing test files in that package. Each test gets its own container, so `goose.Down` is safe here.

```go
package store

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
)

// TestMigration0024SessionScope exercises A1's migration: event and
// evaluations gain a nullable session_id, evaluations_scope_ck widens to
// include it, and event_scope_ck is added NOT VALID — new rows are enforced
// while the pre-existing unattributable chat/course rows (written by
// InsertUserEvent with project_id NULL and no scope column to fill) are
// grandfathered rather than deleted or given a fabricated scope.
func TestMigration0024SessionScope(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping testcontainers integration in -short mode")
	}
	ctx := context.Background()
	pool := newTestPool(t)
	courseID := "00000000-0000-0000-0000-0000000000c1"

	var sessionID string
	if err := pool.QueryRow(ctx, `
		INSERT INTO course_session (user_id, course_id, skill_id, phase)
		VALUES ($1, $2, 'info-literacy-course', 'demonstrate')
		RETURNING id::text`, refactor2SeededStudentID, courseID).Scan(&sessionID); err != nil {
		t.Fatalf("seed course_session: %v", err)
	}

	// 1. A session-scoped event satisfies event_scope_ck with only session_id.
	if _, err := pool.Exec(ctx, `
		INSERT INTO event (user_id, session_id, surface, type, payload)
		VALUES ($1, $2, 'course', 'course_message', '{"unprompted":true}'::jsonb)`,
		refactor2SeededStudentID, sessionID); err != nil {
		t.Fatalf("session-scoped event must satisfy event_scope_ck: %v", err)
	}

	// 2. A brand-new scopeless COURSE event is REJECTED — the hole A1 closes.
	if _, err := pool.Exec(ctx, `
		INSERT INTO event (user_id, surface, type, payload)
		VALUES ($1, 'course', 'course_message', '{}'::jsonb)`,
		refactor2SeededStudentID); err == nil {
		t.Fatal("a new course event with project_id AND session_id both NULL should violate event_scope_ck, got no error")
	}

	// 2b. But an unscoped CHAT event is still ACCEPTED — the explicit
	// `surface = 'chat'` exemption. Chat has no scope column until A2, and its
	// three event writes swallow errors into slog.Warn: without the exemption
	// this constraint would silently stop chat recording evidence and no test
	// would fail. A2 deletes the arm when it scopes chat's writes. This
	// assertion is the regression guard for that silent data loss.
	if _, err := pool.Exec(ctx, `
		INSERT INTO event (user_id, surface, type, payload)
		VALUES ($1, 'chat', 'prompt_sent', '{}'::jsonb)`,
		refactor2SeededStudentID); err != nil {
		t.Fatalf("an unscoped chat event must still be accepted until A2 scopes chat's writes — "+
			"enforcing before the writer has a scope silently kills chat evidence: %v", err)
	}

	// 3. NOT VALID's actual contract: a pre-existing unattributable row still
	// reads fine. Simulate one by dropping the constraint, inserting the kind
	// of row that only pre-A1 code could have written, then re-adding NOT VALID
	// over it. Use surface='course' — a 'chat' row would pass the exemption arm
	// and prove nothing about NOT VALID.
	if _, err := pool.Exec(ctx, `ALTER TABLE event DROP CONSTRAINT event_scope_ck`); err != nil {
		t.Fatalf("drop constraint: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO event (user_id, surface, type, payload)
		VALUES ($1, 'course', 'course_message', '{}'::jsonb)`, refactor2SeededStudentID); err != nil {
		t.Fatalf("seed legacy unattributable row: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		ALTER TABLE event ADD CONSTRAINT event_scope_ck
		CHECK (surface = 'chat' OR num_nonnulls(project_id, session_id) >= 1) NOT VALID`); err != nil {
		t.Fatalf("re-add NOT VALID constraint over a legacy row — this is the whole point of NOT VALID: %v", err)
	}
	var legacy int
	if err := pool.QueryRow(ctx, `
		SELECT count(*) FROM event
		WHERE surface = 'course' AND project_id IS NULL AND session_id IS NULL`).Scan(&legacy); err != nil {
		t.Fatalf("read legacy row: %v", err)
	}
	if legacy != 1 {
		t.Fatalf("legacy unattributable course rows = %d, want 1 (grandfathered, never deleted)", legacy)
	}

	// 4. evaluations accepts a session-scoped row and still rejects a scopeless one.
	if _, err := pool.Exec(ctx, `
		INSERT INTO evaluations (session_id, scores, narrative, model, tier, status)
		VALUES ($1, '[]'::jsonb, 'course narrative', 'deepseek-reasoner', 'flagship', 'done')`,
		sessionID); err != nil {
		t.Fatalf("session-scoped evaluation must satisfy evaluations_scope_ck: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO evaluations (scores, narrative, model, tier, status)
		VALUES ('[]'::jsonb, 'unscoped', 'deepseek-reasoner', 'flagship', 'done')`); err == nil {
		t.Fatal("evaluation with task_id, project_id AND session_id all NULL should violate evaluations_scope_ck, got no error")
	}
}

// TestMigration0024Down verifies 0024 reverses cleanly. No test in this repo
// runs a migration Down anywhere (pre-existing, repo-wide gap) — A1 does it
// for its own migration. Safe because newTestPool gives this test its own
// container.
func TestMigration0024Down(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping testcontainers integration in -short mode")
	}
	ctx := context.Background()
	pool := newTestPool(t)

	db := stdlib.OpenDBFromPool(pool)
	defer db.Close()
	goose.SetBaseFS(migrationFS)
	if err := goose.SetDialect("postgres"); err != nil {
		t.Fatalf("dialect: %v", err)
	}

	if err := goose.DownContext(ctx, db, "migrations"); err != nil {
		t.Fatalf("goose down 0024: %v", err)
	}

	// The columns and constraints are gone.
	for _, q := range []struct{ name, sql string }{
		{"event.session_id", `SELECT 1 FROM information_schema.columns WHERE table_name='event' AND column_name='session_id'`},
		{"evaluations.session_id", `SELECT 1 FROM information_schema.columns WHERE table_name='evaluations' AND column_name='session_id'`},
		{"event_scope_ck", `SELECT 1 FROM pg_constraint WHERE conname='event_scope_ck'`},
	} {
		var one int
		err := pool.QueryRow(ctx, q.sql).Scan(&one)
		if err == nil {
			t.Fatalf("%s still present after Down", q.name)
		}
	}

	// evaluations_scope_ck is restored to its 0021 form: a project-scoped
	// insert still works, a scopeless one is still rejected.
	var projectID string
	if err := pool.QueryRow(ctx, `
		INSERT INTO project (user_id, title) VALUES ($1, 'down-test') RETURNING id::text`,
		refactor2SeededStudentID).Scan(&projectID); err != nil {
		t.Fatalf("seed project: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO evaluations (project_id, scores, narrative, model, tier, status)
		VALUES ($1, '[]'::jsonb, 'n', 'm', 'flagship', 'done')`, projectID); err != nil {
		t.Fatalf("project-scoped evaluation must still work after Down: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO evaluations (scores, narrative, model, tier, status)
		VALUES ('[]'::jsonb, 'n', 'm', 'flagship', 'done')`); err == nil {
		t.Fatal("restored evaluations_scope_ck should reject a scopeless row, got no error")
	}

	// And Up restores it.
	if err := goose.UpContext(ctx, db, "migrations"); err != nil {
		t.Fatalf("goose up after down: %v", err)
	}
	var one int
	if err := pool.QueryRow(ctx, `
		SELECT 1 FROM information_schema.columns WHERE table_name='event' AND column_name='session_id'`).Scan(&one); err != nil {
		t.Fatalf("event.session_id missing after re-Up: %v", err)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run from `apps/api`: `CGO_ENABLED=0 go test ./internal/store/ -run 'TestMigration0024'`
Expected: FAIL — `session-scoped event must satisfy event_scope_ck: ERROR: column "session_id" of relation "event" does not exist`.

- [ ] **Step 3: Write the migration**

Create `apps/api/internal/store/migrations/0024_session_assessment_scope.sql`:

```sql
-- +goose Up
-- A1: give the event stream and evaluations a course-session scope, so course
-- evidence stops being unreachable. Additive, mirroring 0023's session scope
-- over material/card_instances.
ALTER TABLE event       ADD COLUMN session_id uuid REFERENCES course_session(id) ON DELETE CASCADE;
ALTER TABLE evaluations ADD COLUMN session_id uuid REFERENCES course_session(id) ON DELETE CASCADE;

CREATE INDEX event_session_created_idx ON event (session_id, created_at);
CREATE INDEX evaluations_session_idx   ON evaluations (session_id, created_at DESC);

-- evaluations: ordinary widening — every existing row has task_id or
-- project_id and satisfies this unchanged.
ALTER TABLE evaluations DROP CONSTRAINT evaluations_scope_ck;
ALTER TABLE evaluations ADD CONSTRAINT evaluations_scope_ck
  CHECK (num_nonnulls(task_id, project_id, session_id) >= 1);

-- event: NOT VALID, deliberately. event.user_id is NOT NULL, so ownership is
-- never at risk here — these columns carry attribution, not ownership. Every
-- chat/course event already stored was written by InsertUserEvent, which
-- recorded no scope at all (project_id NULL, and no thread/session column
-- existed to fill): those rows are permanently unattributable and there is
-- nothing to backfill FROM. NOT VALID enforces every new row while
-- grandfathering the old ones, rather than deleting real records or inventing
-- a scope they never had.
--
-- The `surface = 'chat'` arm is an EXPLICIT, TEMPORARY exemption. Chat has no
-- scope column until A2, and all three of its event writes are best-effort
-- (error swallowed to a slog.Warn — chat.go:195, chat.go:268,
-- chat_step.go:181). Without this arm the constraint would reject every chat
-- event and chat would silently stop recording evidence, with no test failing.
-- A constraint cannot be enforced one slice before its writers have a scope to
-- satisfy it. A2 adds thread_id, scopes chat's writes, and MUST delete this
-- arm — the exemption is written into the schema so it stays louder than the
-- hole it stands in for.
ALTER TABLE event ADD CONSTRAINT event_scope_ck
  CHECK (surface = 'chat' OR num_nonnulls(project_id, session_id) >= 1) NOT VALID;

-- +goose Down
ALTER TABLE event       DROP CONSTRAINT IF EXISTS event_scope_ck;
ALTER TABLE evaluations DROP CONSTRAINT IF EXISTS evaluations_scope_ck;
ALTER TABLE evaluations ADD CONSTRAINT evaluations_scope_ck
  CHECK (task_id IS NOT NULL OR project_id IS NOT NULL);
DROP INDEX IF EXISTS event_session_created_idx;
DROP INDEX IF EXISTS evaluations_session_idx;
ALTER TABLE event       DROP COLUMN IF EXISTS session_id;
ALTER TABLE evaluations DROP COLUMN IF EXISTS session_id;
```

- [ ] **Step 4: Run tests to verify they pass**

Run from `apps/api`: `CGO_ENABLED=0 go test ./internal/store/ -run 'TestMigration0024'`
Expected: PASS (both tests).

- [ ] **Step 5: Run the full store package**

Run from `apps/api`: `CGO_ENABLED=0 go test -p 1 ./internal/store/...`
Expected: PASS. This catches any existing test that inserts an unscoped `event` row and would now be rejected — if one fails, that test was relying on the hole; fix the test to scope its row, do not weaken the constraint.

- [ ] **Step 6: Commit**

```bash
git add apps/api/internal/store/migrations/0024_session_assessment_scope.sql apps/api/internal/store/migrate_0024_test.go
git commit -m "feat(a1): migration 0024 — session scope on event + evaluations"
```

---

### Task 2: Session-scoped queries

**Files:**
- Modify: `apps/api/internal/store/queries/event.sql`
- Modify: `apps/api/internal/store/queries/evaluation.sql`
- Create: `apps/api/internal/store/session_assessment_sqlc_test.go`
- Regenerate: `apps/api/internal/store/sqlc/*` (via `make sqlc` — **never hand-edit**)

**Interfaces:**
- Consumes: Task 1's `event.session_id`, `evaluations.session_id`.
- Produces: `q.ListEventsBySession(ctx, pgtype.UUID) ([]sqlc.Event, error)`; `q.InsertSessionEvaluation(ctx, sqlc.InsertSessionEvaluationParams) (sqlc.Evaluation, error)` with fields `SessionID pgtype.UUID, Scores []byte, Narrative string, Model string, Tier string, PromptTokens *int32, CompletionTokens *int32, CostEstimate pgtype.Numeric`; `q.GetLatestSessionEvaluation(ctx, pgtype.UUID) (sqlc.Evaluation, error)`.

- [ ] **Step 1: Write the failing test**

Create `apps/api/internal/store/session_assessment_sqlc_test.go` (`package store_test`, using `newStoreTestPool` / `seededStudentID` from `sqlc_test.go`):

```go
package store_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"mindimprint/api/internal/store/sqlc"
)

// TestSessionAssessmentQueries exercises A1's three new queries: the course
// session's event stream becomes readable (it was unreachable — the only
// SELECT against event filtered WHERE project_id = $1, which can never match
// a NULL), and evaluations round-trip at session scope.
func TestSessionAssessmentQueries(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping testcontainers integration in -short mode")
	}
	ctx := context.Background()
	pool := newStoreTestPool(t)
	q := sqlc.New(pool)
	courseID := uuid.MustParse("00000000-0000-0000-0000-0000000000c1")

	s, err := q.CreateCourseSession(ctx, sqlc.CreateCourseSessionParams{
		UserID: seededStudentID, CourseID: courseID, SkillID: "info-literacy-course", Phase: "demonstrate",
	})
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	sid := pgtype.UUID{Bytes: s.ID, Valid: true}

	// Two session events, in order.
	for _, ev := range []struct{ typ, payload string }{
		{"course_message", `{"unprompted":true}`},
		{"phase_advanced", `{"to":"guided"}`},
	} {
		if _, err := q.AppendEvent(ctx, sqlc.AppendEventParams{
			UserID: seededStudentID, SessionID: sid, Surface: "course",
			Type: ev.typ, Payload: []byte(ev.payload),
		}); err != nil {
			t.Fatalf("append %s: %v", ev.typ, err)
		}
		time.Sleep(2 * time.Millisecond)
	}

	// A DIFFERENT session's event must not leak into this session's stream.
	other, err := q.CreateCourseSession(ctx, sqlc.CreateCourseSessionParams{
		UserID: seededStudentID, CourseID: uuid.MustParse("00000000-0000-0000-0000-0000000000c2"),
		SkillID: "info-literacy-course", Phase: "demonstrate",
	})
	if err != nil {
		t.Fatalf("create other session: %v", err)
	}
	if _, err := q.AppendEvent(ctx, sqlc.AppendEventParams{
		UserID: seededStudentID, SessionID: pgtype.UUID{Bytes: other.ID, Valid: true},
		Surface: "course", Type: "course_finished", Payload: []byte(`{}`),
	}); err != nil {
		t.Fatalf("append other: %v", err)
	}

	got, err := q.ListEventsBySession(ctx, sid)
	if err != nil {
		t.Fatalf("ListEventsBySession: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("ListEventsBySession returned %d events, want exactly this session's 2", len(got))
	}
	if got[0].Type != "course_message" || got[1].Type != "phase_advanced" {
		t.Fatalf("events = %q,%q — want temporal order course_message,phase_advanced", got[0].Type, got[1].Type)
	}

	// Evaluations round-trip at session scope; latest wins.
	first, err := q.InsertSessionEvaluation(ctx, sqlc.InsertSessionEvaluationParams{
		SessionID: sid, Scores: []byte(`[{"code":"D1"}]`), Narrative: "first",
		Model: "deepseek-reasoner", Tier: "flagship",
	})
	if err != nil {
		t.Fatalf("InsertSessionEvaluation: %v", err)
	}
	if first.ProjectID.Valid || first.TaskID.Valid {
		t.Fatalf("session evaluation has project/task scope set: %+v", first)
	}
	if first.Status != "done" {
		t.Fatalf("first.Status = %q, want done", first.Status)
	}
	time.Sleep(10 * time.Millisecond)
	second, err := q.InsertSessionEvaluation(ctx, sqlc.InsertSessionEvaluationParams{
		SessionID: sid, Scores: []byte(`[{"code":"D1"}]`), Narrative: "second",
		Model: "deepseek-reasoner", Tier: "flagship",
	})
	if err != nil {
		t.Fatalf("InsertSessionEvaluation (second): %v", err)
	}
	latest, err := q.GetLatestSessionEvaluation(ctx, sid)
	if err != nil {
		t.Fatalf("GetLatestSessionEvaluation: %v", err)
	}
	if latest.ID != second.ID {
		t.Fatalf("latest = %s, want the newest %s (first was %s)", latest.ID, second.ID, first.ID)
	}
}
```

**Note on the second course:** `00000000-...-0000c2` must be a real seeded course. Check `apps/api/internal/store/migrations/0012_*.sql` for the seeded course ids. If only `…c1` exists, insert a second course row directly in the test before creating `other`:

```go
if _, err := pool.Exec(ctx, `
	INSERT INTO course (id, branch, title) VALUES ($1, 'test', 'leak-guard course')
	ON CONFLICT (id) DO NOTHING`, "00000000-0000-0000-0000-0000000000c2"); err != nil {
	t.Fatalf("seed second course: %v", err)
}
```

- [ ] **Step 2: Run test to verify it fails**

Run from `apps/api`: `CGO_ENABLED=0 go test ./internal/store/ -run TestSessionAssessmentQueries`
Expected: FAIL — `q.ListEventsBySession undefined` (and `SessionID` unknown in `AppendEventParams`).

- [ ] **Step 3: Add the queries**

Append to `apps/api/internal/store/queries/event.sql`:

```sql
-- name: ListEventsBySession :many
SELECT * FROM event
WHERE session_id = $1
ORDER BY created_at, id;
```

Modify `AppendEvent` in the same file to carry the session scope:

```sql
-- name: AppendEvent :one
INSERT INTO event (project_id, user_id, session_id, surface, type, payload)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;
```

Append to `apps/api/internal/store/queries/evaluation.sql`:

```sql
-- Course session scope (A1): mirrors the project-scoped pair above. A course
-- session's report is one evaluations row scoped by session_id alone.

-- name: InsertSessionEvaluation :one
INSERT INTO evaluations (session_id, scores, narrative, model, tier,
  prompt_tokens, completion_tokens, cost_estimate, status)
VALUES (@session_id, @scores, @narrative, @model, @tier,
  @prompt_tokens, @completion_tokens, @cost_estimate, 'done')
RETURNING *;

-- name: GetLatestSessionEvaluation :one
SELECT * FROM evaluations
WHERE session_id = @session_id
ORDER BY created_at DESC
LIMIT 1;
```

- [ ] **Step 4: Regenerate sqlc**

Run from `apps/api`: `make sqlc`
Expected: `apps/api/internal/store/sqlc/*` regenerates with no errors. **Never hand-edit the generated files.** If the generator needs `CGO_ENABLED=0` on macOS, use it.

- [ ] **Step 5: Fix `AppendEvent` call sites**

`AppendEventParams` now has a `SessionID` field, so every existing caller must compile. Update these three to pass an explicit empty scope — they are project-scoped and must stay so:

- `apps/api/internal/agent/agentstore.go:120-131` (`AppendEvent`): add `SessionID: pgtype.UUID{Valid: false},`
- `apps/api/internal/agent/chatstore.go:101-106` (`InsertUserEvent`): add `SessionID: pgtype.UUID{Valid: false},` — **chat keeps its unscoped signature until A2** (DEC-A1.2); do not change it here.
- `apps/api/internal/api/chat.go:195-197` (the inline `a.d.Queries.AppendEvent`): add `SessionID: pgtype.UUID{Valid: false},`

Leave `apps/api/internal/agent/coursestore.go`'s `InsertUserEvent` compiling for now — Task 3 replaces it.

- [ ] **Step 6: Run test to verify it passes**

Run from `apps/api`: `CGO_ENABLED=0 go test ./internal/store/ -run TestSessionAssessmentQueries`
Expected: PASS.

- [ ] **Step 7: Build the whole module**

Run from `apps/api`: `CGO_ENABLED=0 go build ./... && CGO_ENABLED=0 go vet ./...`
Expected: exit 0 — proves every `AppendEvent` caller was updated.

- [ ] **Step 8: Commit**

```bash
git add apps/api/internal/store/queries/event.sql apps/api/internal/store/queries/evaluation.sql apps/api/internal/store/sqlc apps/api/internal/store/session_assessment_sqlc_test.go apps/api/internal/agent/agentstore.go apps/api/internal/agent/chatstore.go apps/api/internal/api/chat.go
git commit -m "feat(a1): session-scoped event + evaluation queries"
```

---

### Task 3: `CourseStore.InsertSessionEvent` replaces `InsertUserEvent`

**Files:**
- Modify: `apps/api/internal/agent/course_step.go` (the `CourseStore` interface ~:118-139; call sites at :284, :294, :393, :419)
- Modify: `apps/api/internal/agent/coursestore.go` (:198-218)
- Modify: `apps/api/internal/api/course_session.go` (:331)
- Modify: `apps/api/internal/agent/course_step_test.go` (the in-memory fake)

**Interfaces:**
- Consumes: Task 2's `AppendEventParams.SessionID`.
- Produces: `CourseStore.InsertSessionEvent(ctx context.Context, sessionID uuid.UUID, typ string, payload []byte) error` — resolves `user_id` from the session row, always sets `session_id`, always writes `surface="course"`. And `CourseStore.RecordCourseLLMCall(ctx context.Context, userID uuid.UUID, purpose string, resolved gateway.Resolved, prompt, completion int32) error` — note the **new `purpose` parameter** in third position.

- [ ] **Step 1: Write the failing test**

Add to `apps/api/internal/agent/coursestore_test.go` (create the file if absent; it needs a real pool — follow `agentstore_chat_sqlc_test.go`'s harness, using its pool helper and seeded ids):

```go
// TestInsertSessionEventScopesTheRow is the regression guard for A1's whole
// reason to exist: the old InsertUserEvent took no scope and hard-coded
// project_id NULL, so every course event was written unattributable and
// unreadable. InsertSessionEvent must set session_id and resolve user_id from
// the session — never take either on faith from a caller.
func TestInsertSessionEventScopesTheRow(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping testcontainers integration in -short mode")
	}
	ctx := context.Background()
	pool := newAgentTestPool(t)
	q := sqlc.New(pool)
	store := agent.NewSqlcCourseStore(q)

	s, err := q.CreateCourseSession(ctx, sqlc.CreateCourseSessionParams{
		UserID: agentSeededStudentID, CourseID: uuid.MustParse("00000000-0000-0000-0000-0000000000c1"),
		SkillID: "info-literacy-course", Phase: "demonstrate",
	})
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	if err := store.InsertSessionEvent(ctx, s.ID, "course_message", []byte(`{"unprompted":true}`)); err != nil {
		t.Fatalf("InsertSessionEvent: %v", err)
	}

	rows, err := q.ListEventsBySession(ctx, pgtype.UUID{Bytes: s.ID, Valid: true})
	if err != nil || len(rows) != 1 {
		t.Fatalf("ListEventsBySession = %v (err %v), want 1 row", rows, err)
	}
	got := rows[0]
	if got.Surface != "course" {
		t.Fatalf("surface = %q, want course (the store writes its own surface)", got.Surface)
	}
	if got.UserID != agentSeededStudentID {
		t.Fatalf("user_id = %s, want it resolved from the session (%s)", got.UserID, agentSeededStudentID)
	}
	if !got.SessionID.Valid || got.SessionID.Bytes != s.ID {
		t.Fatalf("session_id = %+v, want %s — an unscoped course event is the A1 bug", got.SessionID, s.ID)
	}
	if got.ProjectID.Valid {
		t.Fatalf("project_id = %+v, want NULL (a course session has no project)", got.ProjectID)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run from `apps/api`: `CGO_ENABLED=0 go test ./internal/agent/ -run TestInsertSessionEventScopesTheRow`
Expected: FAIL — `store.InsertSessionEvent undefined`.

- [ ] **Step 3: Replace the interface method**

In `apps/api/internal/agent/course_step.go`, in the `CourseStore` interface, **delete** the line:

```go
	InsertUserEvent(ctx context.Context, userID uuid.UUID, surface, typ string, payload []byte) error
```

and replace it with:

```go
	// InsertSessionEvent appends one course event scoped to the session.
	// A1 (DEC-A1.2): this REPLACES InsertUserEvent, whose scopeless signature
	// was the bug — it hard-coded project_id NULL with no scope column to fill,
	// so every course event was written unreadable. Neither user nor surface is
	// a parameter: user_id is resolved from the session row, and a CourseStore
	// writes "course" events and nothing else.
	InsertSessionEvent(ctx context.Context, sessionID uuid.UUID, typ string, payload []byte) error
```

Also change `RecordCourseLLMCall`'s signature in the same interface to take a purpose:

```go
	RecordCourseLLMCall(ctx context.Context, userID uuid.UUID, purpose string, resolved gateway.Resolved, prompt, completion int32) error
```

- [ ] **Step 4: Implement in the sqlc adapter**

In `apps/api/internal/agent/coursestore.go`, replace `InsertUserEvent` (:198-203) with:

```go
func (s *sqlcCourseStore) InsertSessionEvent(ctx context.Context, sessionID uuid.UUID, typ string, payload []byte) error {
	sess, err := s.q.GetCourseSession(ctx, sessionID)
	if err != nil {
		return err
	}
	_, err = s.q.AppendEvent(ctx, sqlc.AppendEventParams{
		ProjectID: pgtype.UUID{Valid: false},
		UserID:    sess.UserID,
		SessionID: pgtype.UUID{Bytes: sessionID, Valid: true},
		Surface:   "course",
		Type:      typ,
		Payload:   payload,
	})
	return err
}
```

And give `RecordCourseLLMCall` its `purpose` parameter — change its signature to
`func (s *sqlcCourseStore) RecordCourseLLMCall(ctx context.Context, userID uuid.UUID, purpose string, resolved gateway.Resolved, prompt, completion int32) error`
and replace the hard-coded `Purpose: "coach"` in its `RecordLLMCall` params with `Purpose: purpose`. Leave every other line of that method (the `EstimateCost` call, the unpriced-model `slog.Warn`, `ProjectID: pgtype.UUID{Valid: false}`, `Surface: "course"`) exactly as it is.

- [ ] **Step 5: Update the four course call sites**

In `apps/api/internal/agent/course_step.go`, each `deps.Store.InsertUserEvent(ctx, deps.UserID, "course", <typ>, <payload>)` becomes `deps.Store.InsertSessionEvent(ctx, deps.SessionID, <typ>, <payload>)`:

- `:284` — `card_surfaced`
- `:294` — `course_message` (payload `{"unprompted":true}`)
- `:393` — `course_finished`
- `:419` — `phase_advanced`

And the existing `RecordCourseLLMCall` call in `ProposeCourseReply`'s caller gains `"coach"` as its third argument.

In `apps/api/internal/api/course_session.go:331`, the `card_completed` event's insert becomes `InsertSessionEvent(r.Context(), sess.ID, "card_completed", []byte("{}"))`.

- [ ] **Step 6: Update the in-memory fake**

In `apps/api/internal/agent/course_step_test.go`, rename the fake's `InsertUserEvent` to `InsertSessionEvent` with the new signature, and record `(sessionID, typ)` instead of `(userID, surface, typ)`. **Check every assertion that reads the fake's recorded events** — if one asserted a surface string the fake invented rather than one the backend produces, fix the assertion (mock fidelity, per Global Constraints). Add `purpose` to the fake's `RecordCourseLLMCall`.

- [ ] **Step 7: Run the FULL agent package**

Run from `apps/api`: `CGO_ENABLED=0 go test -p 1 ./internal/agent/...`
Expected: PASS. Full package, not a `-run` subset — this is a projection/event change.

- [ ] **Step 8: Commit**

```bash
git add apps/api/internal/agent/course_step.go apps/api/internal/agent/coursestore.go apps/api/internal/agent/course_step_test.go apps/api/internal/agent/coursestore_test.go apps/api/internal/api/course_session.go
git commit -m "feat(a1): InsertSessionEvent replaces the scopeless InsertUserEvent"
```

---

### Task 4: `buildAssessmentInputFromSession`

**Files:**
- Create: `apps/api/internal/api/course_assessment_input.go`
- Create: `apps/api/internal/api/course_assessment_input_test.go`

**Interfaces:**
- Consumes: `agent.BuildAssessmentInput` (unchanged), `agent.EventDigest`, `agent.CardUse`, `eventText`/`studio.Event` from `assessment.go`.
- Produces: `buildAssessmentInputFromSession(events []studio.Event, cards []sqlc.CardInstance) agent.AssessmentInput` — **pure, no I/O**.

- [ ] **Step 1: Write the failing test**

Create `apps/api/internal/api/course_assessment_input_test.go`:

```go
package api

import (
	"testing"

	"github.com/jackc/pgx/v5/pgtype"

	"mindimprint/api/internal/store/sqlc"
	"mindimprint/api/internal/studio"
)

// TestBuildAssessmentInputFromSession: a course session's evidence is its
// event stream and its cards. It has no gates, snapshots, or graph — those
// stay empty, and Assess's own NA defaults report the unevidenced dimensions
// honestly (spec §4). Pure function, no I/O.
func TestBuildAssessmentInputFromSession(t *testing.T) {
	events := []studio.Event{
		{Type: "course_message", Surface: "course", Payload: []byte(`{"unprompted":true}`)},
		{Type: "phase_advanced", Surface: "course", Payload: []byte(`{"to":"guided"}`)},
	}
	cards := []sqlc.CardInstance{
		{CardID: "craap", Status: "completed"},
		{CardID: "concession", Status: "skipped"},
	}

	in := buildAssessmentInputFromSession(events, cards)

	if len(in.Timeline) != 2 {
		t.Fatalf("Timeline = %v, want one line per event", in.Timeline)
	}
	if !strings.Contains(in.Timeline[0], "course_message") || !strings.Contains(in.Timeline[0], "unprompted") {
		t.Fatalf("Timeline[0] = %q, want the event type and its payload rendered", in.Timeline[0])
	}
	if len(in.CardUses) != 2 || in.CardUses[0].CardID != "craap" {
		t.Fatalf("CardUses = %+v, want one per session card", in.CardUses)
	}
	if len(in.Dispositions) != 2 {
		t.Fatalf("Dispositions = %+v, want one per session card (completed and skipped both count)", in.Dispositions)
	}

	// A course session genuinely has none of these. Empty is the honest answer
	// — Assess defaults the unevidenced dimensions to NA.
	if len(in.GateProgress) != 0 || len(in.WordCounts) != 0 || len(in.ReviewBands) != 0 || in.GraphSummary != "" {
		t.Fatalf("course input claims studio-only evidence: gates=%v words=%v bands=%v graph=%q",
			in.GateProgress, in.WordCounts, in.ReviewBands, in.GraphSummary)
	}
	if in.SnapshotCount != 0 {
		t.Fatalf("SnapshotCount = %d, want 0 (a course session has no drafts)", in.SnapshotCount)
	}
}

// An empty session must not panic and must not invent evidence.
func TestBuildAssessmentInputFromSessionEmpty(t *testing.T) {
	in := buildAssessmentInputFromSession(nil, nil)
	if len(in.Timeline) != 0 || len(in.CardUses) != 0 || len(in.Dispositions) != 0 {
		t.Fatalf("empty session produced evidence: %+v", in)
	}
}
```

Imports for this file are exactly:

```go
import (
	"strings"
	"testing"

	"mindimprint/api/internal/store/sqlc"
	"mindimprint/api/internal/studio"
)
```

- [ ] **Step 2: Run test to verify it fails**

Run from `apps/api`: `CGO_ENABLED=0 go test ./internal/api/ -run TestBuildAssessmentInputFromSession`
Expected: FAIL — `undefined: buildAssessmentInputFromSession`.

- [ ] **Step 3: Write the implementation**

Create `apps/api/internal/api/course_assessment_input.go`:

```go
package api

import (
	"mindimprint/api/internal/agent"
	"mindimprint/api/internal/store/sqlc"
	"mindimprint/api/internal/studio"
)

// buildAssessmentInputFromSession maps one course session's evidence onto
// agent.BuildAssessmentInput's primitive slices — the session-scoped sibling of
// buildAssessmentInputFromProject (assessment.go).
//
// A course session's evidence is its event stream and its cards. It has no
// gates, no draft snapshots, and no argument graph, so those arguments are
// empty: Assess's own NA defaults then report those dimensions as unevidenced,
// which is the honest answer for a course (spec §4) — not a bug to paper over.
//
// Pure — no I/O. The handler does the loading.
func buildAssessmentInputFromSession(events []studio.Event, cards []sqlc.CardInstance) agent.AssessmentInput {
	return agent.BuildAssessmentInput(
		eventDigestsFromProject(events),
		cardUsesFromSession(cards),
		dispositionUsesFromSession(cards),
		nil, // GateProgress — a course session has no gates
		nil, // WordCounts   — no draft snapshots
		nil, // ReviewBands  — no whole-draft review
		"",  // GraphSummary — no argument graph
	)
}

// cardUsesFromSession carries each session card. Unlike the project side there
// is no Equipment projection to zip against, so Spont is left empty rather than
// guessed — the 自发/提示后 signal is a studio derivation and inventing one here
// would be a fabricated fact.
func cardUsesFromSession(cards []sqlc.CardInstance) []agent.CardUse {
	out := make([]agent.CardUse, 0, len(cards))
	for _, ci := range cards {
		out = append(out, agent.CardUse{CardID: ci.CardID})
	}
	return out
}

// dispositionUsesFromSession reports what the student did with each offered
// card. A skip is evidence, not an absence — Slice 12 made card offers
// skippable by design, and the skip is exactly the signal 过程即数据 wants.
func dispositionUsesFromSession(cards []sqlc.CardInstance) []agent.DispositionUse {
	out := make([]agent.DispositionUse, 0, len(cards))
	for _, ci := range cards {
		out = append(out, agent.DispositionUse{Kind: ci.Status})
	}
	return out
}
```

- [ ] **Step 4: Run test to verify it passes**

Run from `apps/api`: `CGO_ENABLED=0 go test ./internal/api/ -run TestBuildAssessmentInputFromSession`
Expected: PASS (both tests).

- [ ] **Step 5: Commit**

```bash
git add apps/api/internal/api/course_assessment_input.go apps/api/internal/api/course_assessment_input_test.go
git commit -m "feat(a1): buildAssessmentInputFromSession"
```

---

### Task 5: The two endpoints

**Files:**
- Create: `apps/api/internal/api/course_assessment.go`
- Modify: `apps/api/internal/api/api.go` (add two routes after line 81)
- Create: `apps/api/internal/api/course_assessment_test.go`

**Interfaces:**
- Consumes: Task 2's `q.ListEventsBySession` / `q.InsertSessionEvaluation` / `q.GetLatestSessionEvaluation`; Task 3's `RecordCourseLLMCall(ctx, userID, purpose, resolved, prompt, completion)`; Task 4's `buildAssessmentInputFromSession`; the existing `a.loadOwnedSession(w, r) (sqlc.CourseSession, bool)` (`course_session.go:41`) and `dtoFromEvaluationRow(row sqlc.Evaluation) (studio.AssessmentDTO, error)` (`assessment.go`).
- Produces: routes `GET /api/v1/courses/{id}/session/assessment` and `POST /api/v1/courses/{id}/session/assessment`, returning `studio.AssessmentDTO` (or JSON `null` from GET when unassessed).

- [ ] **Step 1: Write the failing test**

Create `apps/api/internal/api/course_assessment_test.go`. It uses the package's existing helpers — `newAPITestPool(t)`, `signInSeed(t, pool)`, `withCookie(req, cookie)`, `fakeResolver()`, `cards.ByID`, and `assessStubProvider`/`assessReply` from `assessment_test.go`. **Do not invent a parallel harness.**

Note the existing `countLLMCalls(t, pool, projectID)` takes a **project id** and cannot be used here: a course call has `project_id NULL`. This file adds its own counter.

```go
package api_test

// course_assessment_test.go — A1: GET/POST /api/v1/courses/{id}/session/assessment.
// The session-scoped sibling of assessment_test.go. GET never calls a model;
// POST runs the isolated flagship assessor once over the course session's own
// evidence and persists it at session scope.

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	. "mindimprint/api/internal/api"
	"mindimprint/api/internal/cards"
	"mindimprint/api/internal/store/sqlc"
)

// seededCourseID is the course seeded by migration 0012.
const seededCourseID = "00000000-0000-0000-0000-0000000000c1"

// countCourseLLMCalls counts course-surface llm_call rows. The existing
// countLLMCalls filters by project_id, which is NULL for every course call.
func countCourseLLMCalls(t *testing.T, pool *pgxpool.Pool) int {
	t.Helper()
	var n int
	if err := pool.QueryRow(context.Background(),
		`SELECT count(*) FROM llm_call WHERE surface = 'course' AND purpose = 'assessment'`).Scan(&n); err != nil {
		t.Fatalf("count course llm_call: %v", err)
	}
	return n
}

// startSession creates the caller's course session via the real endpoint, so
// the test exercises the same session the handlers resolve.
func startSession(t *testing.T, h http.Handler, cookie *http.Cookie) {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("POST", "/api/v1/courses/"+seededCourseID+"/session", strings.NewReader("")), cookie))
	if rec.Code != http.StatusOK {
		t.Fatalf("start session = %d, body=%s", rec.Code, rec.Body)
	}
}

// TestGetCourseAssessment_EmptyBeforeGenerate — "not yet assessed" is a normal
// state, never a 404: 200 + literal JSON null, and ZERO llm_call rows.
func TestGetCourseAssessment_EmptyBeforeGenerate(t *testing.T) {
	pool := newAPITestPool(t)
	h := New(Deps{Queries: sqlc.New(pool), Pool: pool}).Handler()
	cookie := signInSeed(t, pool)
	startSession(t, h, cookie)

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("GET", "/api/v1/courses/"+seededCourseID+"/session/assessment", nil), cookie))
	if rec.Code != http.StatusOK {
		t.Fatalf("GET = %d, want 200; body=%s", rec.Code, rec.Body)
	}
	if strings.TrimSpace(rec.Body.String()) != "null" {
		t.Fatalf("GET body = %s, want literal null", rec.Body)
	}
	if n := countCourseLLMCalls(t, pool); n != 0 {
		t.Fatalf("llm_call rows after GET = %d, want 0 — the read path never calls a model", n)
	}
}

// TestGenerateCourseAssessment_PersistsAtSessionScope — one flagship call,
// metered surface=course/purpose=assessment, persisted so GET replays it.
func TestGenerateCourseAssessment_PersistsAtSessionScope(t *testing.T) {
	pool := newAPITestPool(t)
	h := New(Deps{
		Queries: sqlc.New(pool), Pool: pool,
		Provider: assessStubProvider(assessReply), ChatResolver: fakeResolver(), SpecByID: cards.ByID,
	}).Handler()
	cookie := signInSeed(t, pool)
	startSession(t, h, cookie)

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("POST", "/api/v1/courses/"+seededCourseID+"/session/assessment", strings.NewReader("")), cookie))
	if rec.Code != http.StatusOK {
		t.Fatalf("POST = %d, want 200; body=%s", rec.Code, rec.Body)
	}

	var dto struct {
		Dimensions []struct {
			Code, Name, Level, Evidence string
		} `json:"dimensions"`
		Narrative   string `json:"narrative"`
		GeneratedAt string `json:"generatedAt"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &dto); err != nil {
		t.Fatalf("decode DTO: %v — body=%s", err, rec.Body)
	}
	if len(dto.Dimensions) != 10 {
		t.Fatalf("dimensions len = %d, want 10 (every rubric dimension, unevidenced ones NA)", len(dto.Dimensions))
	}
	if dto.Narrative == "" || dto.GeneratedAt == "" {
		t.Fatalf("dto = %+v, want narrative + generatedAt", dto)
	}

	var surface, purpose string
	if err := pool.QueryRow(context.Background(),
		`SELECT surface, purpose FROM llm_call ORDER BY created_at DESC LIMIT 1`).Scan(&surface, &purpose); err != nil {
		t.Fatalf("read llm_call: %v", err)
	}
	if surface != "course" || purpose != "assessment" {
		t.Fatalf("llm_call = (%s,%s), want (course,assessment)", surface, purpose)
	}

	// Persisted at session scope: GET now replays it with no second model call.
	rec2 := httptest.NewRecorder()
	h.ServeHTTP(rec2, withCookie(httptest.NewRequest("GET", "/api/v1/courses/"+seededCourseID+"/session/assessment", nil), cookie))
	if strings.TrimSpace(rec2.Body.String()) == "null" {
		t.Fatal("GET after POST returned null — the report was not persisted at session scope")
	}
	if n := countCourseLLMCalls(t, pool); n != 1 {
		t.Fatalf("llm_call rows after POST+GET = %d, want exactly 1", n)
	}
}

// TestGenerateCourseAssessment_RecordsCostOnRejection — a rejected call still
// cost money, so the llm_call row must exist even though nothing is persisted.
func TestGenerateCourseAssessment_RecordsCostOnRejection(t *testing.T) {
	pool := newAPITestPool(t)
	h := New(Deps{
		Queries: sqlc.New(pool), Pool: pool,
		Provider:     assessStubProvider(`{"dimensions":[],"narrative":"作为一个 AI 语言模型，我认为你做得很好。"}`),
		ChatResolver: fakeResolver(), SpecByID: cards.ByID,
	}).Handler()
	cookie := signInSeed(t, pool)
	startSession(t, h, cookie)

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("POST", "/api/v1/courses/"+seededCourseID+"/session/assessment", strings.NewReader("")), cookie))
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("POST (banned phrasing) = %d, want 422; body=%s", rec.Code, rec.Body)
	}
	if n := countCourseLLMCalls(t, pool); n != 1 {
		t.Fatalf("llm_call rows after rejection = %d, want 1 — a rejected call still cost money", n)
	}
}

// TestCourseAssessment_NoSession404s — a caller with no session on this course
// gets 404 and leaks nothing (loadOwnedSession resolves by (caller, course_id),
// so another user's session is unreachable by construction).
func TestCourseAssessment_NoSession404s(t *testing.T) {
	pool := newAPITestPool(t)
	h := New(Deps{Queries: sqlc.New(pool), Pool: pool}).Handler()
	cookie := signInSeed(t, pool)
	// No startSession call.

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("GET", "/api/v1/courses/"+seededCourseID+"/session/assessment", nil), cookie))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("GET without a session = %d, want 404", rec.Code)
	}
}
```

**Verify the rejection fixture before relying on it.** The banned-phrasing string above must actually trip `agent.Assess`'s enforcement — read `apps/api/internal/agent/assess.go`'s banned-phrasing list and use a phrase it really rejects. If the stub's output is accepted, this test passes for the wrong reason (mock infidelity — the Slice 12 lesson).

**On the entitlement case:** `HasEntitlement` is a stub returning `true` unconditionally today, with no injection seam. Do **not** build one for this test — that is scope creep. Instead confirm by reading `course_assessment.go` that the entitlement gate precedes every model call, and note in the task report that the unentitled path has no test until `HasEntitlement` gains a seam. Record it as a carry-forward.

- [ ] **Step 2: Run test to verify it fails**

Run from `apps/api`: `CGO_ENABLED=0 go test ./internal/api/ -run TestCourseAssessment`
Expected: FAIL — 404 on an unregistered route.

- [ ] **Step 3: Write the handlers**

Create `apps/api/internal/api/course_assessment.go`:

```go
package api

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"mindimprint/api/internal/agent"
	"mindimprint/api/internal/gateway"
	"mindimprint/api/internal/httpx"
	"mindimprint/api/internal/rubric"
	"mindimprint/api/internal/store/sqlc"
	"mindimprint/api/internal/studio"
)

// getCourseAssessment returns the session's most recently generated report, or
// an explicit JSON null when none exists — the report view's own empty state,
// never a 404 ("not yet assessed" is a normal state for a session). No model
// call, ever. Mirrors getAssessment (assessment.go) at session scope.
func (a *API) getCourseAssessment(w http.ResponseWriter, r *http.Request) {
	sess, ok := a.loadOwnedSession(w, r)
	if !ok {
		return
	}
	row, err := a.d.Queries.GetLatestSessionEvaluation(r.Context(), pgtype.UUID{Bytes: sess.ID, Valid: true})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			httpx.WriteJSON(w, http.StatusOK, nil)
			return
		}
		httpx.WriteError(w, r, err)
		return
	}
	dto, derr := dtoFromEvaluationRow(row)
	if derr != nil {
		httpx.WriteError(w, r, httpx.ErrInternal())
		return
	}
	httpx.WriteJSON(w, http.StatusOK, dto)
}

// generateCourseAssessment runs the isolated flagship assessor over one course
// session's evidence: load the session's events + cards, digest, ONE flagship
// call (never downgraded), record the cost regardless of outcome, and — only on
// success — persist at session scope and return. Never in the coach loop, and
// never inside the course's SSE turn (DEC-A1.4).
func (a *API) generateCourseAssessment(w http.ResponseWriter, r *http.Request) {
	sess, ok := a.loadOwnedSession(w, r)
	if !ok {
		return
	}
	u, _ := UserFromContext(r.Context())
	entitled, eerr := HasEntitlement(r.Context(), u)
	if eerr != nil {
		httpx.WriteError(w, r, eerr)
		return
	}
	if !entitled {
		httpx.WriteError(w, r, httpx.ErrNotEntitled())
		return
	}

	sid := pgtype.UUID{Bytes: sess.ID, Valid: true}
	eventRows, err := a.d.Queries.ListEventsBySession(r.Context(), sid)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	events := make([]studio.Event, len(eventRows))
	for i, ev := range eventRows {
		events[i] = studio.Event{
			Type: ev.Type, Surface: ev.Surface,
			Payload: json.RawMessage(ev.Payload), CreatedAt: ev.CreatedAt,
		}
	}
	cards, err := a.d.Queries.ListCardInstancesBySession(r.Context(), sid)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}

	in := buildAssessmentInputFromSession(events, cards)

	resolved, rerr := a.d.ChatResolver(r.Context())
	if rerr != nil {
		httpx.WriteError(w, r, httpx.ErrInternal())
		return
	}
	assessment, usage, aerr := agent.Assess(r.Context(), a.d.Provider, resolved, rubric.CT(), in, agent.EmbeddedAnchors())

	// Record the call's cost even if enforcement then rejects the output — a
	// rejected call still cost money (mirrors generateAssessment/orderReview).
	// The project-side RecordLLMCall cannot be reused: it resolves the owning
	// user via GetProject, and a course session has no project.
	store := agent.NewSqlcCourseStore(a.d.Queries)
	if resolved.Provider != "" {
		if err := store.RecordCourseLLMCall(r.Context(), sess.UserID, "assessment", resolved,
			int32(usage.InputTokens), int32(usage.OutputTokens)); err != nil {
			slog.Warn("generate_course_assessment: record llm call", "err", err)
		}
	}
	if aerr != nil {
		slog.Warn("generate_course_assessment: rejected", "err", aerr)
		httpx.WriteError(w, r, &httpx.APIError{
			Status: http.StatusUnprocessableEntity, Code: "assessment_rejected",
			Message: "这次评估没通过内部校验，请再试一次",
		})
		return
	}

	scoresJSON, merr := json.Marshal(assessment.Dimensions)
	if merr != nil {
		httpx.WriteError(w, r, httpx.ErrInternal())
		return
	}
	cost, priced := gateway.EstimateCost(resolved.Provider, resolved.Model, usage.InputTokens, usage.OutputTokens)
	promptTokens := int32(usage.InputTokens)
	completionTokens := int32(usage.OutputTokens)
	row, err := a.d.Queries.InsertSessionEvaluation(r.Context(), sqlc.InsertSessionEvaluationParams{
		SessionID:        sid,
		Scores:           scoresJSON,
		Narrative:        assessment.Narrative,
		Model:            resolved.Model,
		Tier:             resolved.Tier,
		PromptTokens:     &promptTokens,
		CompletionTokens: &completionTokens,
		CostEstimate:     gateway.CostNumeric(cost, priced),
	})
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	dto, derr := dtoFromEvaluationRow(row)
	if derr != nil {
		httpx.WriteError(w, r, httpx.ErrInternal())
		return
	}
	httpx.WriteJSON(w, http.StatusOK, dto)
}
```

- [ ] **Step 4: Register the routes**

In `apps/api/internal/api/api.go`, immediately after line 81 (`.../session/cards/{cid}/skip`):

```go
	mux.Handle("GET /api/v1/courses/{id}/session/assessment", protected(a.getCourseAssessment))
	mux.Handle("POST /api/v1/courses/{id}/session/assessment", protected(a.generateCourseAssessment))
```

- [ ] **Step 5: Run the FULL api package**

Run from `apps/api`: `CGO_ENABLED=0 go test -p 1 ./internal/api/...`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add apps/api/internal/api/course_assessment.go apps/api/internal/api/course_assessment_test.go apps/api/internal/api/api.go
git commit -m "feat(a1): course session assessment endpoints"
```

---

### Task 6: `collectedCards` on the course session DTO

**Why:** 收集到的工具 (`dc.html:482-494`) needs the session's **completed** cards. `CourseSessionDTO` today carries only `OpenCards` — offers still *proposed/active* (Slice 12's reload-rehydration fix). A completed card is deliberately excluded from that list, so the report has no way to learn what the student collected. The data exists server-side (`ListCardInstancesBySession`); only the wire is missing. No new query is needed — filter the existing one.

**Files:**
- Modify: `apps/api/internal/api/course_session_dto.go`
- Modify: `apps/api/internal/api/course_session.go` (the `getCourseSession` handler)
- Modify: `packages/contracts/src/course.ts` (the `CourseSession` Zod schema)
- Modify: `apps/api/internal/api/course_session_test.go`
- Modify: `packages/contracts/src/course.test.ts` (or wherever `CourseSession` is asserted)

**Interfaces:**
- Consumes: the existing `q.ListCardInstancesBySession(ctx, pgtype.UUID) ([]sqlc.CardInstance, error)`.
- Produces: `CourseSessionDTO.CollectedCards []CourseCollectedCardDTO` with `CourseCollectedCardDTO{ CardID string \`json:"cardId"\` }`, and the matching Zod `collectedCards: z.array(z.object({ cardId: z.string() }))`.

- [ ] **Step 1: Write the failing test**

Add to `apps/api/internal/api/course_session_test.go`:

```go
// A1: the report's 收集到的工具 block needs the session's COMPLETED cards.
// openCards deliberately carries only undispositioned offers, so a completed
// card would otherwise never reach the client. A skipped card is NOT collected
// — the student declined it, and saying otherwise would be a fabrication.
func TestGetCourseSession_CarriesCollectedCards(t *testing.T) {
	pool := newAPITestPool(t)
	h := New(Deps{Queries: sqlc.New(pool), Pool: pool, SpecByID: cards.ByID}).Handler()
	cookie := signInSeed(t, pool)
	startSession(t, h, cookie)

	q := sqlc.New(pool)
	ctx := context.Background()
	sess, err := q.GetCourseSessionByUserCourse(ctx, sqlc.GetCourseSessionByUserCourseParams{
		UserID: seededStudentUserID(t, pool), CourseID: uuid.MustParse(seededCourseID),
	})
	if err != nil {
		t.Fatalf("get session: %v", err)
	}
	sid := pgtype.UUID{Bytes: sess.ID, Valid: true}
	for _, c := range []struct{ cardID, status string }{
		{"craap", "completed"},
		{"concession", "skipped"},
		{"steelman", "proposed"},
	} {
		if _, err := q.CreateSessionCardInstance(ctx, sqlc.CreateSessionCardInstanceParams{
			SessionID: sid, CardID: c.cardID, Status: c.status,
		}); err != nil {
			t.Fatalf("create %s: %v", c.cardID, err)
		}
	}

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("GET", "/api/v1/courses/"+seededCourseID+"/session", nil), cookie))
	if rec.Code != http.StatusOK {
		t.Fatalf("GET session = %d; body=%s", rec.Code, rec.Body)
	}
	var dto struct {
		CollectedCards []struct {
			CardID string `json:"cardId"`
		} `json:"collectedCards"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &dto); err != nil {
		t.Fatalf("decode: %v — body=%s", err, rec.Body)
	}
	if len(dto.CollectedCards) != 1 || dto.CollectedCards[0].CardID != "craap" {
		t.Fatalf("collectedCards = %+v, want only the completed craap — a skipped or still-proposed card is not collected", dto.CollectedCards)
	}
}
```

If a `seededStudentUserID` helper does not exist in the package, resolve the id the way the neighbouring course-session tests already do and follow that; do not add a redundant helper.

- [ ] **Step 2: Run test to verify it fails**

Run from `apps/api`: `CGO_ENABLED=0 go test ./internal/api/ -run TestGetCourseSession_CarriesCollectedCards`
Expected: FAIL — `collectedCards = []`.

- [ ] **Step 3: Add the DTO field**

In `apps/api/internal/api/course_session_dto.go`, add the type and the field:

```go
// CourseCollectedCardDTO is one tool the student actually completed in this
// session — the 收集到的工具 block's row. Distinct from CourseCardOfferDTO:
// that carries UNdispositioned offers (proposed/active) for reload
// rehydration, so a completed card never appears in it. A skipped card is not
// collected — Slice 12 made offers skippable by design, and a skip is a
// decline, not a collection.
type CourseCollectedCardDTO struct {
	CardID string `json:"cardId"`
}
```

Add `CollectedCards []CourseCollectedCardDTO \`json:"collectedCards"\`` to `CourseSessionDTO`, and a `collected []CourseCollectedCardDTO` parameter to `toCourseSessionDTO`, wired through.

- [ ] **Step 4: Fill it in the handler**

In `getCourseSession` (`apps/api/internal/api/course_session.go`), load the session's cards with the existing `ListCardInstancesBySession` and filter to `status == "completed"`, deduplicating by `card_id` (the same tool completed twice is one collected tool). Pass the result to `toCourseSessionDTO`. Update every other `toCourseSessionDTO` call site to compile.

- [ ] **Step 5: Mirror the Zod contract**

In `packages/contracts/src/course.ts`, add beside `CourseCardOffer`:

```ts
// CourseCollectedCard is one tool the student completed in this session — the
// 收集到的工具 block's row. Distinct from CourseCardOffer, which carries only
// undispositioned offers (proposed/active) for reload rehydration.
export const CourseCollectedCard = z.object({ cardId: z.string() });
export type CourseCollectedCard = z.infer<typeof CourseCollectedCard>;
```

and `collectedCards: z.array(CourseCollectedCard)` to the `CourseSession` schema. Export the type alongside the others.

- [ ] **Step 6: Run the tests**

Run from `apps/api`: `CGO_ENABLED=0 go test -p 1 ./internal/api/...`
Run from `packages/contracts`: `npx vitest run`
Expected: PASS both. If a web test breaks because `CourseSession` now requires `collectedCards`, fix the **fixture** to carry the field the backend really sends — do not make the field optional to keep a mock happy (mock fidelity).

- [ ] **Step 7: Commit**

```bash
git add apps/api/internal/api/course_session_dto.go apps/api/internal/api/course_session.go apps/api/internal/api/course_session_test.go packages/contracts/src/course.ts
git commit -m "feat(a1): collectedCards on the course session DTO"
```

---

### Task 7: Web API client

**Files:**
- Create: `apps/web/src/api/courseAssessment.ts`
- Modify: `apps/web/src/api/index.ts` (register on the `api` object — follow how `courseSession.ts` is registered)
- Create: `apps/web/src/api/courseAssessment.test.ts`

**Interfaces:**
- Consumes: Task 5's two routes; the existing `Assessment` Zod schema from `@mind-imprint/contracts` (**unchanged** — DEC-A1.3).
- Produces: `getCourseAssessment(courseId: string): Promise<Assessment | null>` and `generateCourseAssessment(courseId: string): Promise<Assessment>`.

- [ ] **Step 1: Write the failing test**

Create `apps/web/src/api/courseAssessment.test.ts`, mirroring the existing `assessment.ts` client tests. Assert the exact URLs (`/api/v1/courses/co1/session/assessment`), that GET's `null` maps to `null` (not a throw, not `{}`), and that both parse through the `Assessment` Zod schema.

- [ ] **Step 2: Run test to verify it fails**

Run **from `apps/web`**: `npx vitest run src/api/courseAssessment.test.ts`
Expected: FAIL — module not found.

- [ ] **Step 3: Write the client**

Create `apps/web/src/api/courseAssessment.ts`, mirroring `apps/web/src/api/assessment.ts` exactly, with the session routes:

```ts
import { Assessment } from "@mind-imprint/contracts";
import { apiFetch } from "./client";

// A1: the course session's report. Scoped by course id — the server resolves
// the session from (caller, course_id), so no session id appears in the URL
// (IDOR closed by construction, mirroring every other course route).
export async function getCourseAssessment(courseId: string): Promise<Assessment | null> {
  const raw = await apiFetch<unknown>(`/api/v1/courses/${courseId}/session/assessment`);
  if (raw == null) return null;
  return Assessment.parse(raw);
}

export async function generateCourseAssessment(courseId: string): Promise<Assessment> {
  const raw = await apiFetch<unknown>(`/api/v1/courses/${courseId}/session/assessment`, { method: "POST" });
  return Assessment.parse(raw);
}
```

Check `apps/web/src/api/assessment.ts` for the real `apiFetch` import path and match it.

- [ ] **Step 4: Run test to verify it passes**

Run from `apps/web`: `npx vitest run src/api/courseAssessment.test.ts`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add apps/web/src/api/courseAssessment.ts apps/web/src/api/courseAssessment.test.ts apps/web/src/api/index.ts
git commit -m "feat(a1): course assessment api client"
```

---

### Task 8: `CourseReport` — 能力评估, 收集到的工具, and the 挑战通过 fix

**Files:**
- Modify: `apps/web/src/shell/courses/CourseReport.tsx`
- Modify: `apps/web/src/shell/courses/CourseReport.test.tsx`

**Interfaces:**
- Consumes: Task 7's `api.getCourseAssessment` / `api.generateCourseAssessment`; Task 6's `api.getCourseSession().collectedCards` (`{cardId: string}[]`); `Assessment` / `DimensionScore` from `@mind-imprint/contracts`; the existing `CARD_REGISTRY`; `api.getCourseProgress().completed_ordinals`.
- Produces: nothing downstream (last task).

**Binding design:** `docs/design/思维印记_工作区.dc.html` lines 466-494. Read them before writing — copy colours, spacing, and copy **verbatim**. Icons inline SVG.

- [ ] **Step 1: Write the failing test**

Add to `apps/web/src/shell/courses/CourseReport.test.tsx`:

```tsx
// A1: 挑战通过 counted challenges that EXIST, not ones the student reached —
// every student was told they passed every challenge, including ones never
// seen. 通过 is defined as engagement (DEC-A1.5): reached, i.e. the ordinal is
// in the server-recorded completed_ordinals.
it("counts only challenges the student actually reached", async () => {
  (api.getCourse as any).mockResolvedValue({
    ...courseFixture,
    steps: [
      { id: "s0", ordinal: 0, kind: "teaching", purpose: "看一遍" },
      { id: "s1", ordinal: 1, kind: "challenge", purpose: "核查一处断言" },
      { id: "s2", ordinal: 2, kind: "challenge", purpose: "找一个反例" },
    ],
  });
  (api.getCourseProgress as any).mockResolvedValue({ completed_ordinals: [0, 1] });
  (api.getCourseAssessment as any).mockResolvedValue(null);
  (api.generateCourseAssessment as any).mockResolvedValue(assessmentFixture);

  render(<CourseReport courseId="co1" onBackToCourses={vi.fn()} onGoPortal={vi.fn()} />);

  const tile = await screen.findByText("挑战通过");
  // Reached ordinal 1 only — ordinal 2 was never opened.
  expect(tile.parentElement).toHaveTextContent("1");
});

it("generates the report once when none exists, and renders its dimensions with evidence", async () => {
  (api.getCourseAssessment as any).mockResolvedValue(null);
  (api.generateCourseAssessment as any).mockResolvedValue(assessmentFixture);

  render(<CourseReport courseId="co1" onBackToCourses={vi.fn()} onGoPortal={vi.fn()} />);

  expect(await screen.findByText("能力评估")).toBeInTheDocument();
  expect(screen.getByText("按 SOLO 四级 · 来自这门课里你的表现")).toBeInTheDocument();
  expect(screen.getByText("信源辨识")).toBeInTheDocument();
  expect(screen.getByText("学生在第 3 轮追问了来源的作者与机构。")).toBeInTheDocument();
  await waitFor(() => expect(api.generateCourseAssessment).toHaveBeenCalledTimes(1));
});

it("does not regenerate when a report already exists", async () => {
  (api.getCourseAssessment as any).mockResolvedValue(assessmentFixture);
  render(<CourseReport courseId="co1" onBackToCourses={vi.fn()} onGoPortal={vi.fn()} />);
  expect(await screen.findByText("能力评估")).toBeInTheDocument();
  expect(api.generateCourseAssessment).not.toHaveBeenCalled();
});

// RL-5: no total, no rank, no aggregate anywhere in the DOM.
it("renders no total, rank, or aggregate score", async () => {
  (api.getCourseAssessment as any).mockResolvedValue(assessmentFixture);
  const { container } = render(<CourseReport courseId="co1" onBackToCourses={vi.fn()} onGoPortal={vi.fn()} />);
  await screen.findByText("能力评估");
  expect(container.textContent).not.toMatch(/总分|排名|平均分|\d+\s*\/\s*40/);
});

// An NA dimension is rendered, not hidden — "this course produced no evidence
// here" is true and useful; dropping the row would overstate the coverage.
it("renders an NA dimension as 未涉及 with no lit segments", async () => {
  (api.getCourseAssessment as any).mockResolvedValue({
    ...assessmentFixture,
    dimensions: [{ code: "D7", name: "论证质量", level: "NA", evidence: "这门课没有产出书面论证。" }],
  });
  render(<CourseReport courseId="co1" onBackToCourses={vi.fn()} onGoPortal={vi.fn()} />);
  expect(await screen.findByText("未涉及")).toBeInTheDocument();
  expect(screen.getByText("这门课没有产出书面论证。")).toBeInTheDocument();
  expect(screen.queryByText("L0")).toBeNull();
});

// Generation failure is honest — never a fabricated report.
it("shows an honest error when generation fails", async () => {
  (api.getCourseAssessment as any).mockResolvedValue(null);
  (api.generateCourseAssessment as any).mockRejectedValue(new Error("boom"));
  render(<CourseReport courseId="co1" onBackToCourses={vi.fn()} onGoPortal={vi.fn()} />);
  expect(await screen.findByText("能力评估暂时没能生成，稍后再看看。")).toBeInTheDocument();
});
```

Define `assessmentFixture` at the top of the file — **check it against the real DTO**: camelCase `generatedAt`, `dimensions[].{code,name,level,evidence}`, `level` ∈ `L1|L2|L3|L4|NA`, no total field:

```tsx
const assessmentFixture = {
  dimensions: [
    { code: "D2", name: "信源辨识", level: "L3", evidence: "学生在第 3 轮追问了来源的作者与机构。" },
  ],
  narrative: "这门课里，你从接受说法转向了追问说法的来源。",
  generatedAt: "2026-07-17T09:00:00Z",
};
```

Add `getCourseAssessment` / `generateCourseAssessment` to the file's existing `vi.mock("../../api")` block.

- [ ] **Step 2: Run test to verify it fails**

Run from `apps/web`: `npx vitest run src/shell/courses/CourseReport.test.tsx`
Expected: FAIL — `能力评估` not found; the 挑战通过 tile reads `2` instead of `1`.

- [ ] **Step 3: Implement**

In `apps/web/src/shell/courses/CourseReport.tsx`:

1. Add state: `const [assessment, setAssessment] = useState<Assessment | null>(null);` and `const [assessErr, setAssessErr] = useState(false);`
2. In the existing `useEffect`, after the course/progress load, run the DEC-A1.4 auto-generate:

```tsx
try {
  let a = await api.getCourseAssessment(courseId);
  if (a == null) a = await api.generateCourseAssessment(courseId);
  if (!cancelled) setAssessment(a);
} catch {
  if (!cancelled) setAssessErr(true);
}
```

3. Fix the stat (DEC-A1.5) — `completed` is `course_progress.completed_ordinals`, recorded server-side by the render handler, so it is not client-assertable:

```tsx
const reachedChallenges = challenges.filter((c) => completed.includes(c.ordinal));
```
and `<Stat value={`${reachedChallenges.length}`} label="挑战通过" color="#D98263" />`.

4. In 挑战回顾, render the green ✓ only when `completed.includes(c.ordinal)`.

5. Add the 能力评估 block after 挑战回顾, from the binding markup (`dc.html:466-480`). One row per `assessment.dimensions`: the dimension `name`; a chip — for `L1..L4` the label is the level string on `#D98263`/`#FBEEE7`, for `NA` it is `未涉及` on `#9AA1B0`/`#F3F4F7`; four segments where `NA` lights zero and `L{n}` lights `n`; and the `evidence` as the note. When `assessErr`, render `能力评估暂时没能生成，稍后再看看。` in place of the rows. **No total, no rank, no aggregate** (RL-5).

6. Add the 收集到的工具 block after it (`dc.html:482-494`): fetch the session via `api.getCourseSession(courseId)` in the same effect, and render one pill per `collectedCards[]` entry (Task 6), named `CARD_REGISTRY[c.cardId]?.name`. A card id absent from the registry is skipped rather than rendered as a raw id. When `collectedCards` is empty, **omit the whole block** — an empty 收集到的工具 card would imply the student collected nothing when they may simply not have reached a card, and the design has no empty state for it.

7. Do **not** add `我的学习笔记` or `导出笔记` — no note or export feature exists (Global Constraints: no fabrication).

- [ ] **Step 4: Run test to verify it passes**

Run from `apps/web`: `npx vitest run src/shell/courses/CourseReport.test.tsx`
Expected: PASS.

- [ ] **Step 5: Run the full web suite + typecheck**

Run from `apps/web`: `npx vitest run` then `npx tsc --noEmit`
Expected: PASS, exit 0.

- [ ] **Step 6: Commit**

```bash
git add apps/web/src/shell/courses/CourseReport.tsx apps/web/src/shell/courses/CourseReport.test.tsx
git commit -m "feat(a1): course report 能力评估 + 收集到的工具; 挑战通过 counts engagement"
```

---

## Final gate

Before the whole-branch review:

- From `apps/api`: `CGO_ENABLED=0 go test -p 1 ./...` — all packages, quiet Docker daemon.
- From `packages/contracts`: `npx vitest run`
- From `apps/web`: `npx vitest run` and `npx tsc --noEmit`
- From `apps/api`: `make sqlc` and `make sync-skills` produce **zero drift** (`git status` clean afterwards).

Then dispatch the whole-branch review on the most capable model with `scripts/review-package $(git merge-base main HEAD) HEAD`. Attention lens = the Global Constraints above, especially: the `NOT VALID` contract (new rows enforced, legacy rows readable); that **no** course event can still be written unscoped (`InsertUserEvent` is gone from `CourseStore`, not merely unused); RL-5 (no grade/rank/aggregate in engine, DTO, or UI); the rubric/assessor untouched (DEC-A1.3); GET makes no model call (llm_call-count assertion); cost recorded on rejection; 挑战通过 counts engagement only; and **mock fidelity** — every fixture checked against the real contract, since that single failure produced five of Slice 12's six Criticals.
