# A3 — Project Terminal + 成长报告 History Hub Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Give the project a one-time student-initiated finish that auto-generates its growth report, and rebuild 成长报告 as a read-only cross-surface history hub of every report.

**Architecture:** Two additive server endpoints (`POST /projects/{id}/finish`, `GET /growth/history`) plus migration 0026 (project.status enum CHECK). The studio projection gains `finished`/`canFinish`; the studio ReviewView renders 完成任务·归档 when `canFinish`. The report-generation body is extracted into one shared helper; the standalone regenerate route is removed. A1 (course) and A2 (chat) code is untouched.

**Tech Stack:** Go (net/http, sqlc, goose, pgx/pgtype, testcontainers), TypeScript + React + Zod + vitest.

**Spec:** `docs/superpowers/specs/2026-07-18-a3-project-terminal-and-growth-history-design.md`

## Global Constraints

Every task's requirements implicitly include these:

- **旗舰绝不降级:** the terminal report resolves via `a.d.EvalResolver` (flagship), never a chaperone resolver. Assert the persisted evaluation's `tier == "flagship"`.
- **记录档位 + token + 成本 even on rejection:** cost is recorded via `RecordLLMCall` before the reject check returns.
- **One-time (铁律 2 / 不操纵):** the finish endpoint is the ONLY project-report generation path. Already-finished → `409`. No 重新生成 / regenerate on any surface. No auto-generation except at the student's finish click. History is read-only.
- **RL-5:** no total/aggregate/rank anywhere. History rows carry NO score/level; only the expanded report shows per-dimension diagnostics (label + evidence).
- **A1 & A2 untouched:** do not modify `course_assessment*.go`, `CourseReport.tsx`, `chat_assessment*.go`, or `ChatReport.tsx`.
- **Client never calls a model directly; secrets server-side only.** Nothing new logged/rendered/persisted carries a key.
- **`make sqlc` from `apps/api`** after editing `queries/*.sql`; NEVER hand-edit `apps/api/internal/store/sqlc/*`.
- **Go tests:** `cd apps/api && DOCKER_HOST=unix:///var/run/docker.sock CGO_ENABLED=0 go test -p 1 ./...` — run the FULL affected packages, never a `-run` subset, for migration/query/projection/endpoint changes.
- **Web/contracts tests run from their OWN dir** (`cd apps/web && npm test` / `cd packages/contracts && npm test`), never a compound `cd`.
- **Icons are inline SVG**, never `lucide-react`.
- **NEVER `git add` a bare directory** — name each file. The pre-existing `M package.json` and untracked user files under `docs/` and the repo root are NOT ours; never stage them.
- **Binding design** `docs/design/思维印记_工作区.dc.html` wins for any UI copy it draws.
- Do not build the S6 反思归档 station (deferred).

---

### Task 1: Migration 0026 — close the `project.status` enum

**Files:**
- Create: `apps/api/internal/store/migrations/0026_project_status_check.sql`
- Test: `apps/api/internal/store/migrate_0026_test.go`

**Interfaces:**
- Consumes: existing `project` table (`status text NOT NULL DEFAULT 'active'`, no CHECK, migration 0016).
- Produces: `project` now enforces `CHECK (status IN ('active','finished'))`. Referenced by Task 4 (`SetProjectFinished` writes `'finished'`).

- [ ] **Step 1: Write the migration**

Create `apps/api/internal/store/migrations/0026_project_status_check.sql`:

```sql
-- +goose Up
-- A3: close project.status to a known enum. project.status has existed since
-- 0016 (text NOT NULL DEFAULT 'active') with NO CHECK and no writer; A3's
-- finish endpoint (SetProjectFinished) becomes its first writer, setting
-- 'finished'. Every existing row is 'active', so a validated CHECK adds cleanly
-- (no NOT VALID needed). Mirrors the enum shape course_session already carries
-- (0023: CHECK (status IN ('active','finished'))).
ALTER TABLE project ADD CONSTRAINT project_status_ck
  CHECK (status IN ('active', 'finished'));

-- +goose Down
ALTER TABLE project DROP CONSTRAINT IF EXISTS project_status_ck;
```

- [ ] **Step 2: Write the up/down test**

Create `apps/api/internal/store/migrate_0026_test.go`. Mirror the structure of `migrate_0025_test.go` (same package `store`, same `newTestPool(t)` helper, same `testing.Short()` skip, same `refactor2SeededStudentID` seed constant, same goose Down usage).

```go
package store

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
)

// TestMigration0026ProjectStatusCheck: after Up, project.status is a closed
// enum — 'active'/'finished' accepted, anything else rejected. Existing rows
// (all 'active') survive the validated CHECK.
func TestMigration0026ProjectStatusCheck(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping testcontainers integration in -short mode")
	}
	ctx := context.Background()
	pool := newTestPool(t) // migrates to head (0026)

	// A seeded project already exists as 'active' and survived the Up.
	var active int
	if err := pool.QueryRow(ctx, `
		INSERT INTO project (user_id, title, status) VALUES ($1, 'a3-finish', 'finished')
		RETURNING 1`, refactor2SeededStudentID).Scan(&active); err != nil {
		t.Fatalf("'finished' must satisfy project_status_ck: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO project (user_id, title, status) VALUES ($1, 'a3-bogus', 'archived')`,
		refactor2SeededStudentID); err == nil {
		t.Fatal("status='archived' should violate project_status_ck, got no error")
	}
}

// TestMigration0026Down removes the CHECK, restoring the pre-A3 free-text
// column. newTestPool migrates to head (0026); DownTo(25) reverses only 0026.
func TestMigration0026Down(t *testing.T) {
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
	if err := goose.DownToContext(ctx, db, "migrations", 25); err != nil {
		t.Fatalf("goose down to v25 (reversing 0026): %v", err)
	}

	// The CHECK is gone: a previously-illegal status now inserts fine.
	if _, err := pool.Exec(ctx, `
		INSERT INTO project (user_id, title, status) VALUES ($1, 'a3-down', 'archived')`,
		refactor2SeededStudentID); err != nil {
		t.Fatalf("after Down, free-text status must insert: %v", err)
	}

	// And Up restores the CHECK.
	if err := goose.UpContext(ctx, db, "migrations"); err == nil {
		t.Fatal("re-Up over an 'archived' row must fail the validated CHECK, got no error")
	}
}
```

Note: `TestMigration0026Down`'s re-Up deliberately fails because it seeded an `'archived'` row the validated CHECK rejects — that asserts the CHECK really validates existing rows. If `newTestPool` shares no state between the two tests (each gets its own container), this is safe; confirm by reading `newTestPool` before running.

- [ ] **Step 3: Run the tests**

Run: `cd apps/api && DOCKER_HOST=unix:///var/run/docker.sock CGO_ENABLED=0 go test -p 1 ./internal/store/`
Expected: PASS (both new tests + existing 0024/0025 migration tests still green).

- [ ] **Step 4: Commit**

```bash
git add apps/api/internal/store/migrations/0026_project_status_check.sql apps/api/internal/store/migrate_0026_test.go
git commit -m "feat(a3): migration 0026 — close project.status to ('active','finished')"
```

---

### Task 2: Queries — `SetProjectFinished` + `ListGrowthHistory`

**Files:**
- Modify: `apps/api/internal/store/queries/project.sql` (add `SetProjectFinished`)
- Modify: `apps/api/internal/store/queries/evaluation.sql` (add `ListGrowthHistory`)
- Regenerate: `apps/api/internal/store/sqlc/*` via `make sqlc`
- Test: `apps/api/internal/store/growth_history_query_test.go`

**Interfaces:**
- Produces:
  - `Queries.SetProjectFinished(ctx, id uuid.UUID) error` — sets `status='finished'`, bumps `last_active_at`.
  - `Queries.ListGrowthHistory(ctx, userID uuid.UUID) ([]sqlc.ListGrowthHistoryRow, error)` — one row per project/session/thread that has an evaluation owned by `userID`, newest-first. Row fields (sqlc-generated names): `Surface string`, `ScopeID pgtype.UUID`, `Label string`, `Sublabel pgtype.Text` (nullable), `CreatedAt pgtype.Timestamptz`, `Scores []byte`, `Narrative string`. Consumed by Task 5.

- [ ] **Step 1: Add `SetProjectFinished` to `project.sql`**

Append to `apps/api/internal/store/queries/project.sql` (next to `TouchProject`):

```sql
-- name: SetProjectFinished :exec
-- A3 terminal: the first and only writer of project.status='finished'.
UPDATE project SET status = 'finished', last_active_at = now() WHERE id = $1;
```

- [ ] **Step 2: Add `ListGrowthHistory` to `evaluation.sql`**

Append to `apps/api/internal/store/queries/evaluation.sql`:

```sql
-- A3 growth history: every report the caller owns, across all three scopes,
-- newest-first, one row per scope (reports are one-time; DISTINCT ON is
-- defensive — if two ever share a scope, the latest wins). Owner-filtered
-- through each scope's own join, so the returned ids are guaranteed owned and
-- the embedded report needs no second per-row auth. Labels: project.title /
-- course.title (+ session phase as sublabel) / chat_thread.title.

-- name: ListGrowthHistory :many
SELECT surface, scope_id, label, sublabel, created_at, scores, narrative
FROM (
  (SELECT DISTINCT ON (e.project_id)
     'project'::text AS surface, e.project_id AS scope_id,
     p.title AS label, NULL::text AS sublabel,
     e.created_at AS created_at, e.scores AS scores, e.narrative AS narrative
   FROM evaluations e JOIN project p ON p.id = e.project_id
   WHERE e.project_id IS NOT NULL AND p.user_id = @user_id
   ORDER BY e.project_id, e.created_at DESC)
  UNION ALL
  (SELECT DISTINCT ON (e.session_id)
     'course'::text, e.session_id,
     c.title, cs.phase,
     e.created_at, e.scores, e.narrative
   FROM evaluations e
     JOIN course_session cs ON cs.id = e.session_id
     JOIN course c ON c.id = cs.course_id
   WHERE e.session_id IS NOT NULL AND cs.user_id = @user_id
   ORDER BY e.session_id, e.created_at DESC)
  UNION ALL
  (SELECT DISTINCT ON (e.thread_id)
     'chat'::text, e.thread_id,
     t.title, NULL::text,
     e.created_at, e.scores, e.narrative
   FROM evaluations e JOIN chat_thread t ON t.id = e.thread_id
   WHERE e.thread_id IS NOT NULL AND t.user_id = @user_id
   ORDER BY e.thread_id, e.created_at DESC)
) rows
ORDER BY created_at DESC;
```

- [ ] **Step 3: Regenerate sqlc**

Run: `cd apps/api && make sqlc`
Expected: clean regen; `git status` shows only `apps/api/internal/store/sqlc/*` changed (new `SetProjectFinished`, `ListGrowthHistory`, `ListGrowthHistoryRow`). If `make sqlc` errors on the UNION/DISTINCT ON, read the error — the parenthesised sub-selects with per-branch `ORDER BY` are required for `DISTINCT ON` inside `UNION ALL`; do not remove them.

- [ ] **Step 4: Write the store test**

Create `apps/api/internal/store/growth_history_query_test.go`. Seed one project eval, one course-session eval, one chat-thread eval for the seeded student, plus one project eval for a DIFFERENT user; assert `ListGrowthHistory` returns exactly the three owned rows, newest-first, with correct surfaces and labels, and excludes the other user's row.

```go
package store

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
)

func TestListGrowthHistory(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping testcontainers integration in -short mode")
	}
	ctx := context.Background()
	pool := newTestPool(t)
	q := New(pool)

	// Owned project + its report.
	var projectID pgtype.UUID
	if err := pool.QueryRow(ctx, `
		INSERT INTO project (user_id, title) VALUES ($1, '中国可持续') RETURNING id`,
		refactor2SeededStudentID).Scan(&projectID); err != nil {
		t.Fatalf("seed project: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO evaluations (project_id, scores, narrative, model, tier, status)
		VALUES ($1, '[]'::jsonb, 'proj narrative', 'deepseek-v4-pro', 'flagship', 'done')`,
		projectID); err != nil {
		t.Fatalf("seed project eval: %v", err)
	}

	// Owned course session + its report (course c1 is seeded by 0012).
	var sessionID pgtype.UUID
	if err := pool.QueryRow(ctx, `
		INSERT INTO course_session (user_id, course_id, skill_id, phase)
		VALUES ($1, '00000000-0000-0000-0000-0000000000c1', 'info-literacy-course', 'reflect')
		RETURNING id`, refactor2SeededStudentID).Scan(&sessionID); err != nil {
		t.Fatalf("seed course_session: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO evaluations (session_id, scores, narrative, model, tier, status)
		VALUES ($1, '[]'::jsonb, 'course narrative', 'deepseek-v4-pro', 'flagship', 'done')`,
		sessionID); err != nil {
		t.Fatalf("seed session eval: %v", err)
	}

	// Owned chat thread + its report.
	var threadID pgtype.UUID
	if err := pool.QueryRow(ctx, `
		INSERT INTO chat_thread (user_id, title) VALUES ($1, 'CRAAP 溯源') RETURNING id`,
		refactor2SeededStudentID).Scan(&threadID); err != nil {
		t.Fatalf("seed chat_thread: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO evaluations (thread_id, scores, narrative, model, tier, status)
		VALUES ($1, '[]'::jsonb, 'chat narrative', 'deepseek-v4-pro', 'flagship', 'done')`,
		threadID); err != nil {
		t.Fatalf("seed thread eval: %v", err)
	}

	// A DIFFERENT user's project report — must be excluded.
	var otherUser pgtype.UUID
	if err := pool.QueryRow(ctx, `
		INSERT INTO users (email, name, role, school_id)
		SELECT 'a3-other@example.com', 'Other', 'student', school_id FROM users WHERE id = $1
		RETURNING id`, refactor2SeededStudentID).Scan(&otherUser); err != nil {
		t.Fatalf("seed other user: %v", err)
	}
	var otherProject pgtype.UUID
	if err := pool.QueryRow(ctx, `
		INSERT INTO project (user_id, title) VALUES ($1, 'not mine') RETURNING id`,
		otherUser).Scan(&otherProject); err != nil {
		t.Fatalf("seed other project: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO evaluations (project_id, scores, narrative, model, tier, status)
		VALUES ($1, '[]'::jsonb, 'not mine', 'deepseek-v4-pro', 'flagship', 'done')`,
		otherProject); err != nil {
		t.Fatalf("seed other eval: %v", err)
	}

	rows, err := q.ListGrowthHistory(ctx, refactor2SeededStudentID)
	if err != nil {
		t.Fatalf("ListGrowthHistory: %v", err)
	}
	if len(rows) != 3 {
		t.Fatalf("got %d rows, want 3 (own project+course+chat, other user excluded)", len(rows))
	}
	surfaces := map[string]string{}
	for _, r := range rows {
		surfaces[r.Surface] = r.Label
	}
	if surfaces["project"] != "中国可持续" {
		t.Errorf("project label = %q, want 中国可持续", surfaces["project"])
	}
	if surfaces["course"] != "信息素养" && surfaces["course"] == "" {
		t.Errorf("course row missing; got labels %v", surfaces)
	}
	if surfaces["chat"] != "CRAAP 溯源" {
		t.Errorf("chat label = %q, want CRAAP 溯源", surfaces["chat"])
	}
}
```

Note: the seeded course c1's title comes from migration 0012 — read it and assert the exact value if you prefer a strict check; the test above only asserts a course row exists. Confirm `users` insertable columns against the schema before running (adjust the other-user insert if `users` has more NOT NULL columns).

- [ ] **Step 5: Run the tests**

Run: `cd apps/api && DOCKER_HOST=unix:///var/run/docker.sock CGO_ENABLED=0 go test -p 1 ./internal/store/`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add apps/api/internal/store/queries/project.sql apps/api/internal/store/queries/evaluation.sql apps/api/internal/store/sqlc apps/api/internal/store/growth_history_query_test.go
git commit -m "feat(a3): SetProjectFinished + ListGrowthHistory queries"
```

---

### Task 3: Studio projection — `Finished` + `CanFinish`

**Files:**
- Modify: `apps/api/internal/studio/dto.go` (add two fields to `StudioProjection`)
- Modify: `apps/api/internal/studio/projection.go` (compute them in `Project`)
- Modify: `packages/contracts/src/studioState.ts` (mirror the two fields)
- Test: `apps/api/internal/studio/projection_test.go` (add one case)

**Interfaces:**
- Consumes: `d.Project.Status` (`sqlc.Project.Status string`), `agent.RecordedGatesFromNodes(d.GateStates) map[string]agent.RecordedGate` with `.Items["whole_draft_review"]`.
- Produces: `StudioProjection.Finished bool` (`json:"finished"`), `StudioProjection.CanFinish bool` (`json:"canFinish"`). Consumed by Task 9 (web ReviewView/StudioContainer).

- [ ] **Step 1: Add the fields to the Go DTO**

In `apps/api/internal/studio/dto.go`, add to the `StudioProjection` struct (after `Readiness`):

```go
	// Finished is true once the project's terminal has run (project.status ==
	// "finished"). CanFinish is true when the S5 整稿体检 gate item
	// whole_draft_review is "solid" AND the project is not already finished —
	// the studio shows 完成任务·归档 exactly then (A3). Derived, never stored.
	Finished  bool `json:"finished"`
	CanFinish bool `json:"canFinish"`
```

- [ ] **Step 2: Add a failing projection test**

In `apps/api/internal/studio/projection_test.go`, add (adapt seed helpers to the file's existing style — find an existing test that builds `ProjectData` with recorded gate states and copy its setup):

```go
func TestProjectCanFinishAndFinished(t *testing.T) {
	// whole_draft_review solid + status active → canFinish true, finished false.
	d := projectDataWithGateItem(t, "draft_polish", "whole_draft_review", "solid") // helper: build ProjectData with one recorded gate item
	d.Project.Status = "active"
	proj, err := Project(testSkill(t), testSpecByID, d)
	if err != nil {
		t.Fatalf("Project: %v", err)
	}
	if !proj.CanFinish || proj.Finished {
		t.Fatalf("solid+active: canFinish=%v finished=%v, want true/false", proj.CanFinish, proj.Finished)
	}

	// Already finished → canFinish false even though the gate is solid.
	d.Project.Status = "finished"
	proj, _ = Project(testSkill(t), testSpecByID, d)
	if proj.CanFinish || !proj.Finished {
		t.Fatalf("finished: canFinish=%v finished=%v, want false/true", proj.CanFinish, proj.Finished)
	}
}
```

If no `projectDataWithGateItem`/`testSkill`/`testSpecByID` helper exists, reuse whatever the existing projection tests use to assemble a `ProjectData` with recorded gate states (grep `RecordedGatesFromNodes` and `gate_state` in `projection_test.go`) — the point is one `ProjectData` whose gate states record `draft_polish.whole_draft_review = "solid"`.

- [ ] **Step 3: Run it to see it fail**

Run: `cd apps/api && DOCKER_HOST=unix:///var/run/docker.sock CGO_ENABLED=0 go test -p 1 ./internal/studio/ -run TestProjectCanFinishAndFinished`
Expected: FAIL (fields always zero — `Project` doesn't set them yet).

- [ ] **Step 4: Compute the fields in `Project`**

In `apps/api/internal/studio/projection.go`, inside `func Project(...)`, just before the `StudioProjection{...}` is assembled/returned, add:

```go
	finished := d.Project.Status == "finished"
	recordedGates := agent.RecordedGatesFromNodes(d.GateStates)
	// nil-map read is safe; absent gate/item → "" → not solid.
	canFinish := !finished && recordedGates["draft_polish"].Items["whole_draft_review"] == "solid"
```

Then set `Finished: finished, CanFinish: canFinish` on the returned `StudioProjection` literal. (If `Project` already computes `agent.RecordedGatesFromNodes(d.GateStates)` earlier for the stations, reuse that variable instead of re-deriving — grep the function first; do NOT introduce a second call if one exists.)

- [ ] **Step 5: Mirror the fields in the contract**

In `packages/contracts/src/studioState.ts`, add to the `StudioProjection` `z.object({...})` (after `readiness`):

```ts
  finished: z.boolean(),
  canFinish: z.boolean(),
```

- [ ] **Step 6: Run Go + contracts tests**

Run: `cd apps/api && DOCKER_HOST=unix:///var/run/docker.sock CGO_ENABLED=0 go test -p 1 ./internal/studio/`
Expected: PASS.
Run: `cd packages/contracts && npm test`
Expected: PASS (existing tests still green; the added fields don't break any).

Note: `apps/api/internal/studio/dto_parity_test.go` and web `StudioContainer.test.tsx` mocks may build a `StudioProjection` literal — if the Go parity test or a web mock now fails for a missing `finished`/`canFinish`, add the two fields to those fixtures (`finished: false, canFinish: false`). This is expected fixture maintenance, not a defect.

- [ ] **Step 7: Commit**

```bash
git add apps/api/internal/studio/dto.go apps/api/internal/studio/projection.go apps/api/internal/studio/projection_test.go packages/contracts/src/studioState.ts
git commit -m "feat(a3): studio projection exposes finished + canFinish"
```

---

### Task 4: Finish endpoint + shared generate helper; remove the regenerate route

**Files:**
- Modify: `apps/api/internal/api/assessment.go` (extract `generateProjectReport`; delete `generateAssessment` HTTP handler; keep `getAssessment`)
- Create: `apps/api/internal/api/project_finish.go` (`finishProject` handler)
- Modify: `apps/api/internal/api/api.go` (add `POST /projects/{id}/finish`; remove `POST /projects/{id}/assessment`)
- Modify: `apps/api/internal/api/assessment_test.go` (drop the POST-generate test; keep GET-null / persisted-read assertions, moved as needed)
- Test: `apps/api/internal/api/project_finish_test.go`

**Interfaces:**
- Consumes: `Queries.SetProjectFinished` (Task 2); `a.d.EvalResolver`; `agent.NewSqlcAgentStore(a.d.Queries, a.d.Pool)` with `.RecordLLMCall`, `.AppendEvent`, `.ListGateStates`; `studio.Load` / `buildAssessmentInputFromProject` / `agent.Assess` / `Queries.InsertProjectEvaluation` / `dtoFromEvaluationRow` (all already in `assessment.go`).
- Produces: `finishProject(w, r)` bound at `POST /api/v1/projects/{id}/finish`. `generateProjectReport(ctx context.Context, projectID uuid.UUID) (studio.AssessmentDTO, error)` shared helper (no userID param — `RecordLLMCall` resolves the owner from `projectID`) + sentinel `errAssessmentRejected`.

- [ ] **Step 1: Extract `generateProjectReport` in `assessment.go`**

Replace the body of `generateAssessment` (lines ~56–148) so that its generation core lives in a reusable helper and the HTTP handler is deleted. Add near the top of `assessment.go`:

```go
// errAssessmentRejected marks a generated report that failed internal
// enforcement — the caller maps it to 422 assessment_rejected. Cost is already
// recorded when this is returned.
var errAssessmentRejected = errors.New("assessment rejected")

// generateProjectReport runs the isolated flagship growth-assessor over the
// project's whole process record and persists ONE project-scoped evaluation.
// It records the call's cost even when the output is then rejected. Returns the
// wire DTO on success; errAssessmentRejected on a rejected output; any other
// error on I/O failure. This is A3's single project-report generation core —
// the finish endpoint is its only caller (the standalone regenerate route is
// gone; one-time generation, DEC-A3.5).
func (a *API) generateProjectReport(ctx context.Context, projectID uuid.UUID) (studio.AssessmentDTO, error) {
	d, err := studio.Load(ctx, a.d.Queries, projectID)
	if err != nil {
		return studio.AssessmentDTO{}, err
	}
	sk, skOK := skills.ByID("writing-project")
	if !skOK {
		return studio.AssessmentDTO{}, httpx.ErrInternal()
	}
	proj, err := studio.Project(sk, a.d.SpecByID, d)
	if err != nil {
		return studio.AssessmentDTO{}, err
	}
	in := buildAssessmentInputFromProject(d, proj, graphSummary(ctx, a.d.Queries, projectID))

	resolved, rerr := a.d.EvalResolver(ctx)
	if rerr != nil {
		return studio.AssessmentDTO{}, httpx.ErrInternal()
	}
	assessment, usage, aerr := agent.Assess(ctx, a.d.Provider, resolved, rubric.CT(), in, agent.EmbeddedAnchors())

	store := agent.NewSqlcAgentStore(a.d.Queries, a.d.Pool)
	if resolved.Provider != "" {
		if err := store.RecordLLMCall(ctx, agent.LLMCallRow{
			ProjectID: projectID, Surface: "studio", Purpose: "assessment",
			Resolved: resolved, PromptTokens: int32(usage.InputTokens), CompletionTokens: int32(usage.OutputTokens),
		}); err != nil {
			slog.Warn("generate_project_report: record llm call", "err", err)
		}
	}
	if aerr != nil {
		slog.Warn("generate_project_report: rejected", "err", aerr)
		return studio.AssessmentDTO{}, errAssessmentRejected
	}

	scoresJSON, merr := json.Marshal(assessment.Dimensions)
	if merr != nil {
		return studio.AssessmentDTO{}, httpx.ErrInternal()
	}
	cost, priced := gateway.EstimateCost(resolved.Provider, resolved.Model, usage.InputTokens, usage.OutputTokens)
	promptTokens := int32(usage.InputTokens)
	completionTokens := int32(usage.OutputTokens)
	row, err := a.d.Queries.InsertProjectEvaluation(ctx, sqlc.InsertProjectEvaluationParams{
		ProjectID:        pgtype.UUID{Bytes: projectID, Valid: true},
		Scores:           scoresJSON,
		Narrative:        assessment.Narrative,
		Model:            resolved.Model,
		Tier:             resolved.Tier,
		PromptTokens:     &promptTokens,
		CompletionTokens: &completionTokens,
		CostEstimate:     gateway.CostNumeric(cost, priced),
	})
	if err != nil {
		return studio.AssessmentDTO{}, err
	}
	return dtoFromEvaluationRow(row)
}
```

Then **delete** the entire `func (a *API) generateAssessment(...)` handler. Keep `getAssessment`, `dtoFromEvaluationRow`, `buildAssessmentInputFromProject`, and all the `*FromProject` helpers untouched. Remove now-unused imports only if the compiler flags them (do not pre-emptively strip).

- [ ] **Step 2: Write the finish handler**

Create `apps/api/internal/api/project_finish.go`:

```go
package api

import (
	"errors"
	"log/slog"
	"net/http"

	"mindimprint/api/internal/agent"
	"mindimprint/api/internal/httpx"
)

// finishProject is the project's terminal (A3): the one-time, student-initiated
// act that mints the project's growth report. Guards the S5 gate server-side,
// generates the flagship report (cost recorded even on reject), and — only on
// success — marks the project finished and appends a project_finished event.
// finished ⟺ has a terminal report: a rejected report leaves the project active
// (retryable). Already finished → 409 (no regeneration; DEC-A3.5).
func (a *API) finishProject(w http.ResponseWriter, r *http.Request) {
	projectID, ok := a.loadOwnedProject(w, r)
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

	store := agent.NewSqlcAgentStore(a.d.Queries, a.d.Pool)

	// One-time guard: already finished → 409. (Load the row fresh — loadOwnedProject
	// already fetched it, but re-read via studio.Load's project or GetProject to
	// read status.)
	proj, err := a.d.Queries.GetProject(r.Context(), projectID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if proj.Status == "finished" {
		httpx.WriteError(w, r, &httpx.APIError{
			Status: http.StatusConflict, Code: "already_finished", Message: "任务已归档",
		})
		return
	}

	// Gate guard: 整稿体检 must have passed. Never trust the client.
	recorded, gerr := store.ListGateStates(r.Context(), projectID)
	if gerr != nil {
		httpx.WriteError(w, r, gerr)
		return
	}
	if recorded["draft_polish"].Items["whole_draft_review"] != "solid" {
		httpx.WriteError(w, r, &httpx.APIError{
			Status: http.StatusUnprocessableEntity, Code: "gate_not_met",
			Message: "还没通过整稿体检，先完成整稿体检再归档",
		})
		return
	}

	// Generate the terminal report (flagship; cost recorded even on reject).
	dto, gerr2 := a.generateProjectReport(r.Context(), projectID)
	if errors.Is(gerr2, errAssessmentRejected) {
		httpx.WriteError(w, r, &httpx.APIError{
			Status: http.StatusUnprocessableEntity, Code: "assessment_rejected",
			Message: "这次评估没通过内部校验，请再试一次",
		})
		return
	}
	if gerr2 != nil {
		httpx.WriteError(w, r, gerr2)
		return
	}

	// Success: mark finished + record the terminal event. finished ⟺ report.
	if err := a.d.Queries.SetProjectFinished(r.Context(), projectID); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if err := store.AppendEvent(r.Context(), agent.EventRow{
		ProjectID: projectID, Surface: "studio", Type: "project_finished",
		Payload: mustJSON(map[string]any{"generatedAt": dto.GeneratedAt}),
	}); err != nil {
		slog.Warn("finish_project: append event", "err", err)
	}
	httpx.WriteJSON(w, http.StatusOK, dto)
}
```

Confirm the exact `agent.EventRow` field names and `store.AppendEvent`/`store.ListGateStates` signatures against `writing.go:389` and `agentstore.go:575` before compiling (adjust `EventRow` field names if they differ, e.g. `Type`/`Payload`). `mustJSON` is already defined in the `api` package (used in `writing.go`).

- [ ] **Step 3: Rewire routes in `api.go`**

In `apps/api/internal/api/api.go`:
- Remove: `mux.Handle("POST /api/v1/projects/{id}/assessment", protected(a.generateAssessment))` (line 65).
- Keep: `GET /api/v1/projects/{id}/assessment` (line 64).
- Add after the GET assessment line:

```go
	mux.Handle("POST /api/v1/projects/{id}/finish", protected(a.finishProject))
```

- [ ] **Step 4: Prune the stale `generateAssessment` test**

In `apps/api/internal/api/assessment_test.go`, delete any test that POSTs `/projects/{id}/assessment` or calls `a.generateAssessment`. Preserve tests for `getAssessment` (GET-null, and reading a persisted row). If a persisted-eval read test depended on the POST to create the row, re-point it to insert the eval directly via `sqlc.New(pool).InsertProjectEvaluation` (the test already imports `sqlc`).

- [ ] **Step 5: Write the finish endpoint test**

Create `apps/api/internal/api/project_finish_test.go`. Model it on `assessment_test.go` / `chat_assessment_test.go` (same in-package HTTP-handler test harness — a real container pool, a seeded student, a fake/stub provider that returns a valid assessment). Cover:

```go
// Cases (names indicative — match the file's existing harness style):
// 1. gate NOT solid → 422 gate_not_met, project stays 'active', NO evaluation row.
// 2. gate solid, provider returns a valid report → 200 with AssessmentDTO;
//    project.status == 'finished'; exactly one project evaluation persisted with
//    tier == 'flagship'; a project_finished event exists.
// 3. already finished → 409 already_finished (no second evaluation row).
// 4. gate solid but provider output rejected → 422 assessment_rejected AND
//    project stays 'active' AND an llm_call cost row was still recorded.
```

Reuse the existing test's mechanism for making the gate item solid (grep `whole_draft_review` / `UpsertGateState` in the api test files — the S8 tests already drive it) and for injecting a provider whose output passes vs. fails enforcement (grep how `chat_assessment_test.go` forces the reject path). Assert `tier == "flagship"` by reading the persisted row via `sqlc.New(pool).GetLatestProjectEvaluation`.

- [ ] **Step 6: Run the FULL affected packages**

Run: `cd apps/api && DOCKER_HOST=unix:///var/run/docker.sock CGO_ENABLED=0 go test -p 1 ./internal/api/ ./internal/studio/ ./internal/agent/`
Expected: PASS. If any other file referenced `generateAssessment` (grep first: `grep -rn "generateAssessment" apps/api`), fix the reference — there should be none after this task.

- [ ] **Step 7: Commit**

```bash
git add apps/api/internal/api/assessment.go apps/api/internal/api/project_finish.go apps/api/internal/api/api.go apps/api/internal/api/assessment_test.go apps/api/internal/api/project_finish_test.go
git commit -m "feat(a3): POST /projects/{id}/finish terminal; extract generateProjectReport; drop regenerate route"
```

---

### Task 5: Growth history endpoint

**Files:**
- Create: `apps/api/internal/api/growth_history.go` (`getGrowthHistory` handler + DTO structs)
- Modify: `apps/api/internal/api/api.go` (add `GET /api/v1/growth/history`)
- Test: `apps/api/internal/api/growth_history_test.go`

**Interfaces:**
- Consumes: `Queries.ListGrowthHistory` (Task 2), `studio.AssessmentDTO` / `studio.AssessmentDimensionDTO`.
- Produces: `getGrowthHistory(w, r)` at `GET /api/v1/growth/history` returning `{"entries": [...]}`. Wire shape consumed by Task 6/7.

- [ ] **Step 1: Write the handler + DTO**

Create `apps/api/internal/api/growth_history.go`:

```go
package api

import (
	"encoding/json"
	"net/http"
	"time"

	"mindimprint/api/internal/httpx"
	"mindimprint/api/internal/studio"
)

// growthHistoryEntry is one row of the 成长报告 history hub: a surface + label +
// date, with the full report embedded (the list is already owner-filtered, so
// no second fetch and no per-report auth). RL-5: no score/level at this level —
// the diagnostic lives inside report.dimensions only.
type growthHistoryEntry struct {
	Surface   string               `json:"surface"` // "project" | "course" | "chat"
	ScopeID   string               `json:"scopeId"`
	Label     string               `json:"label"`
	Sublabel  *string              `json:"sublabel"`
	CreatedAt string               `json:"createdAt"`
	Report    studio.AssessmentDTO `json:"report"`
}

// getGrowthHistory returns every report the caller owns across project, course,
// and chat scopes, newest-first. No model call, ever.
func (a *API) getGrowthHistory(w http.ResponseWriter, r *http.Request) {
	u, _ := UserFromContext(r.Context())
	rows, err := a.d.Queries.ListGrowthHistory(r.Context(), u.ID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	entries := make([]growthHistoryEntry, 0, len(rows))
	for _, row := range rows {
		var dims []studio.AssessmentDimensionDTO
		if err := json.Unmarshal(row.Scores, &dims); err != nil {
			// A malformed row must not sink the whole list; skip it.
			continue
		}
		var sub *string
		if row.Sublabel.Valid {
			s := row.Sublabel.String
			sub = &s
		}
		entries = append(entries, growthHistoryEntry{
			Surface:   row.Surface,
			ScopeID:   uuidText(row.ScopeID),
			Label:     row.Label,
			Sublabel:  sub,
			CreatedAt: row.CreatedAt.Time.Format(time.RFC3339),
			Report: studio.AssessmentDTO{
				Dimensions:  dims,
				Narrative:   row.Narrative,
				GeneratedAt: row.CreatedAt.Time.Format(time.RFC3339),
			},
		})
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"entries": entries})
}
```

Confirm the sqlc row field types before compiling: `row.Scores` (`[]byte`), `row.Sublabel` (`pgtype.Text` → `.Valid`/`.String`), `row.CreatedAt` (`pgtype.Timestamptz` → `.Time`), `row.ScopeID` (`pgtype.UUID`). For `uuidText`, reuse whatever helper the `api` package already uses to render a `pgtype.UUID` as a string (grep `pgtype.UUID` + `.String()` / `uuid.UUID(` in `apps/api/internal/api`); if none exists, inline `uuid.UUID(row.ScopeID.Bytes).String()` and import `github.com/google/uuid`).

- [ ] **Step 2: Add the route**

In `apps/api/internal/api/api.go`, near the other GETs (e.g. after the projects block):

```go
	mux.Handle("GET /api/v1/growth/history", protected(a.getGrowthHistory))
```

- [ ] **Step 3: Write the endpoint test**

Create `apps/api/internal/api/growth_history_test.go`. Seed (via the container pool) a project eval + a chat-thread eval for the seeded student; GET `/api/v1/growth/history` through the handler; assert the JSON has 2 entries newest-first, each with a `report.dimensions`/`report.narrative`, correct `surface`, and NO top-level score field. Also assert another user's eval is absent (mirror Task 2's exclusion seed).

- [ ] **Step 4: Run the FULL api package**

Run: `cd apps/api && DOCKER_HOST=unix:///var/run/docker.sock CGO_ENABLED=0 go test -p 1 ./internal/api/`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add apps/api/internal/api/growth_history.go apps/api/internal/api/api.go apps/api/internal/api/growth_history_test.go
git commit -m "feat(a3): GET /growth/history — cross-surface report list"
```

---

### Task 6: Contracts — `GrowthHistory` DTO

**Files:**
- Create: `packages/contracts/src/growthHistory.ts`
- Modify: `packages/contracts/src/index.ts` (export)
- Test: `packages/contracts/src/growthHistory.test.ts`

**Interfaces:**
- Consumes: existing `Assessment` schema (`./assessment`).
- Produces: `GrowthHistory` Zod schema + `GrowthHistoryEntry` type. Consumed by Task 7.

- [ ] **Step 1: Write the schema**

Create `packages/contracts/src/growthHistory.ts`:

```ts
import { z } from "zod";
import { Assessment } from "./assessment";

// A3: one row of the 成长报告 history hub. surface + label + date, with the full
// report embedded. RL-5: no score/level at row level — only inside report.
export const GrowthHistoryEntry = z.object({
  surface: z.enum(["project", "course", "chat"]),
  scopeId: z.string(),
  label: z.string(),
  sublabel: z.string().nullable(),
  createdAt: z.string(),
  report: Assessment,
});
export type GrowthHistoryEntry = z.infer<typeof GrowthHistoryEntry>;

export const GrowthHistory = z.object({
  entries: z.array(GrowthHistoryEntry),
});
export type GrowthHistory = z.infer<typeof GrowthHistory>;
```

- [ ] **Step 2: Export from the barrel**

In `packages/contracts/src/index.ts`, add (match the file's existing `export * from "./..."` style):

```ts
export * from "./growthHistory";
```

- [ ] **Step 3: Write the parse test**

Create `packages/contracts/src/growthHistory.test.ts` (match the test style of a sibling — grep for an existing `*.test.ts` under `packages/contracts/src`; if the package uses a different runner, follow it):

```ts
import { describe, expect, it } from "vitest";
import { GrowthHistory } from "./growthHistory";

describe("GrowthHistory", () => {
  it("parses a representative payload with a nullable sublabel", () => {
    const parsed = GrowthHistory.parse({
      entries: [
        {
          surface: "course",
          scopeId: "00000000-0000-0000-0000-0000000000c1",
          label: "信息素养",
          sublabel: "回看",
          createdAt: "2026-07-18T00:00:00Z",
          report: { dimensions: [], narrative: "n", generatedAt: "2026-07-18T00:00:00Z" },
        },
        {
          surface: "project",
          scopeId: "11111111-1111-1111-1111-111111111111",
          label: "中国可持续",
          sublabel: null,
          createdAt: "2026-07-17T00:00:00Z",
          report: { dimensions: [], narrative: "n", generatedAt: "2026-07-17T00:00:00Z" },
        },
      ],
    });
    expect(parsed.entries).toHaveLength(2);
    expect(parsed.entries[1]!.sublabel).toBeNull();
  });

  it("rejects an unknown surface", () => {
    expect(() =>
      GrowthHistory.parse({ entries: [{ surface: "email", scopeId: "x", label: "x", sublabel: null, createdAt: "x", report: { dimensions: [], narrative: "", generatedAt: "" } }] }),
    ).toThrow();
  });
});
```

- [ ] **Step 4: Run contracts tests**

Run: `cd packages/contracts && npm test`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add packages/contracts/src/growthHistory.ts packages/contracts/src/index.ts packages/contracts/src/growthHistory.test.ts
git commit -m "feat(a3): GrowthHistory contract DTO"
```

---

### Task 7: Web API client — `finishProject` + `getGrowthHistory`

**Files:**
- Modify: `apps/web/src/api/projects.ts` (add `finishProject`)
- Create: `apps/web/src/api/growth.ts` (`getGrowthHistory`)
- Modify: `apps/web/src/api/index.ts` (barrel + `Api` interface + remove `generateAssessment`)
- Modify: `apps/web/src/api/assessment.ts` (remove `generateAssessment`; keep `getAssessment`)
- Test: `apps/web/src/api/growth.test.ts`

**Interfaces:**
- Consumes: `Assessment` (embedded), `GrowthHistory` (Task 6).
- Produces: `api.finishProject(id: string): Promise<Assessment>`, `api.getGrowthHistory(): Promise<GrowthHistoryEntry[]>`. Consumed by Tasks 8 & 9.

- [ ] **Step 1: Add `finishProject` to the projects client**

In `apps/web/src/api/projects.ts`, add (match the file's `apiFetch` import + style — see `chatAssessment.ts` for the POST shape):

```ts
import { Assessment } from "@mind-imprint/contracts";

export async function finishProject(id: string): Promise<Assessment> {
  const raw = await apiFetch<unknown>(`/api/v1/projects/${id}/finish`, { method: "POST" });
  return Assessment.parse(raw);
}
```

(If `apiFetch` / `Assessment` are already imported in `projects.ts`, don't duplicate the imports.)

- [ ] **Step 2: Write the growth client**

Create `apps/web/src/api/growth.ts`:

```ts
import { GrowthHistory, type GrowthHistoryEntry } from "@mind-imprint/contracts";
import { apiFetch } from "./client";

// A3: the 成长报告 history hub. Read-only — no generation lives here (reports are
// one-time terminal artifacts). newest-first, owner-filtered server-side.
export async function getGrowthHistory(): Promise<GrowthHistoryEntry[]> {
  const raw = await apiFetch<unknown>(`/api/v1/growth/history`);
  return GrowthHistory.parse(raw).entries;
}
```

- [ ] **Step 3: Wire the barrel + remove `generateAssessment`**

In `apps/web/src/api/assessment.ts`, delete the `generateAssessment` function; keep `getAssessment`.

In `apps/web/src/api/index.ts`:
- Remove the `generateAssessment` import and its entry in both the `Api` interface and the assembled `api` object.
- Add `finishProject` (from `./projects`) and `getGrowthHistory` (from `./growth`) to the imports, the `Api` interface, and the `api` object. Match the existing declaration style:

```ts
  finishProject(id: string): Promise<Assessment>;
  getGrowthHistory(): Promise<GrowthHistoryEntry[]>;
```

- [ ] **Step 4: Write the client test**

Create `apps/web/src/api/growth.test.ts` (mirror `apps/web/src/api/projects.test.ts` — same `apiFetch`/`fetch` mock style):

```ts
import { describe, expect, it, vi, beforeEach } from "vitest";
import { getGrowthHistory } from "./growth";

// Follow the exact fetch-mock pattern projects.test.ts uses.
describe("getGrowthHistory", () => {
  beforeEach(() => vi.restoreAllMocks());
  it("parses entries", async () => {
    vi.spyOn(globalThis, "fetch").mockResolvedValue(
      new Response(JSON.stringify({ entries: [
        { surface: "chat", scopeId: "t1", label: "CRAAP", sublabel: null, createdAt: "2026-07-18T00:00:00Z",
          report: { dimensions: [], narrative: "n", generatedAt: "2026-07-18T00:00:00Z" } },
      ] }), { status: 200, headers: { "Content-Type": "application/json" } }),
    );
    const list = await getGrowthHistory();
    expect(list).toHaveLength(1);
    expect(list[0]!.surface).toBe("chat");
  });
});
```

If `projects.test.ts` mocks `apiFetch` (not `fetch`) directly, follow that instead — do not invent a mock style.

- [ ] **Step 5: Run web tests + typecheck**

Run: `cd apps/web && npm test`
Expected: PASS. Some existing tests that referenced `api.generateAssessment` (e.g. a GrowthReport test) may fail here — those are rewritten in Task 8; if a test fails ONLY because `generateAssessment` is gone and it belongs to GrowthReport, leave it for Task 8 and note it in the report. All other suites must be green.
Run: `cd apps/web && npx tsc --noEmit`
Expected: exit 0 (or only errors in `GrowthReport.tsx`, addressed in Task 8).

- [ ] **Step 6: Commit**

```bash
git add apps/web/src/api/projects.ts apps/web/src/api/growth.ts apps/web/src/api/index.ts apps/web/src/api/assessment.ts apps/web/src/api/growth.test.ts
git commit -m "feat(a3): web api client — finishProject + getGrowthHistory; drop generateAssessment"
```

---

### Task 8: Web — 成长报告 history hub

**Files:**
- Rewrite: `apps/web/src/shell/growth/GrowthReport.tsx`
- Rewrite: `apps/web/src/shell/growth/GrowthReport.test.tsx`

**Interfaces:**
- Consumes: `api.getGrowthHistory()` (Task 7), `GrowthHistoryEntry` (Task 6), `SOLO_LABELS` (existing).
- Produces: the history-hub UI. No generation, no 重新生成.

- [ ] **Step 1: Rewrite `GrowthReport.tsx` as a read-only history hub**

Replace the whole file. It loads the history on mount, renders newest-first accordion rows (surface chip + label + sublabel + date), expands a row to its embedded report (reuse the existing `LevelChip` + `DimensionRow` markup for per-dimension rows and a narrative block). No generate button anywhere. Empty state when `entries.length === 0`.

```tsx
import { useEffect, useState } from "react";
import type { DimensionScore, GrowthHistoryEntry, ScoredLevel } from "@mind-imprint/contracts";
import { SOLO_LABELS } from "@mind-imprint/contracts";
import { api } from "../../api";

const SURFACE_LABEL: Record<GrowthHistoryEntry["surface"], string> = {
  project: "项目", course: "课程", chat: "聊天",
};

function LevelChip({ level }: { level: DimensionScore["level"] }) {
  if (level === "NA") {
    return (
      <span style={{ fontSize: 12, fontWeight: 700, color: "#9AA1B0", background: "#F1F2F6", borderRadius: 999, padding: "3px 10px" }}>
        证据不足 · NA
      </span>
    );
  }
  const colors: Record<ScoredLevel, { fg: string; bg: string }> = {
    L1: { fg: "#B0691F", bg: "#FBF0E3" }, L2: { fg: "#8A6D1F", bg: "#FBF6E3" },
    L3: { fg: "#2E7D5B", bg: "#E7F5EF" }, L4: { fg: "#2A5FA8", bg: "#E8F0FB" },
  };
  const c = colors[level];
  return (
    <span style={{ fontSize: 12, fontWeight: 700, color: c.fg, background: c.bg, borderRadius: 999, padding: "3px 10px" }}>
      {SOLO_LABELS[level]}
    </span>
  );
}

function DimensionRow({ dim }: { dim: DimensionScore }) {
  return (
    <div style={{ padding: "14px 0", borderBottom: "1px solid #F3F4F7" }}>
      <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between", gap: 10 }}>
        <div style={{ fontSize: 14.5, fontWeight: 700, color: "#1C2333" }}>{dim.name}</div>
        <LevelChip level={dim.level} />
      </div>
      {dim.evidence && (
        <div style={{ fontSize: 13.5, color: "#6B7384", lineHeight: 1.6, marginTop: 6 }}>{dim.evidence}</div>
      )}
    </div>
  );
}

function HistoryRow({ entry, open, onToggle }: { entry: GrowthHistoryEntry; open: boolean; onToggle: () => void }) {
  const date = entry.createdAt.slice(0, 10);
  return (
    <div style={{ background: "#fff", border: "1px solid #EAECF2", borderRadius: 16, marginTop: 12, overflow: "hidden" }}>
      <button
        type="button"
        onClick={onToggle}
        aria-expanded={open}
        style={{ width: "100%", display: "flex", alignItems: "center", gap: 12, padding: "16px 20px", background: "none", border: "none", cursor: "pointer", fontFamily: "inherit", textAlign: "left" }}
      >
        <span style={{ fontSize: 11.5, fontWeight: 800, color: "#5B6474", background: "#F1F2F6", borderRadius: 8, padding: "3px 9px", flex: "none" }}>
          {SURFACE_LABEL[entry.surface]}
        </span>
        <span style={{ flex: 1, minWidth: 0, fontSize: 15, fontWeight: 700, color: "#1C2333" }}>
          {entry.label}{entry.sublabel ? <span style={{ color: "#8A92A3", fontWeight: 600 }}> · {entry.sublabel}</span> : null}
        </span>
        <span style={{ fontSize: 12.5, color: "#9AA1B0", flex: "none" }}>{date}</span>
        <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="#9AA1B0" strokeWidth="2.4" strokeLinecap="round" strokeLinejoin="round" style={{ flex: "none", transform: open ? "rotate(180deg)" : "none", transition: "transform .15s" }}>
          <path d="M6 9l6 6 6-6" />
        </svg>
      </button>
      {open && (
        <div style={{ padding: "0 20px 20px" }}>
          <div style={{ borderTop: "1px solid #F0F1F5", paddingTop: 8 }}>
            {entry.report.dimensions.map((d) => <DimensionRow key={d.code} dim={d} />)}
          </div>
          <div style={{ marginTop: 16, fontSize: 14, color: "#2B3346", lineHeight: 1.8 }}>{entry.report.narrative}</div>
        </div>
      )}
    </div>
  );
}

export function GrowthReport() {
  const [entries, setEntries] = useState<GrowthHistoryEntry[] | undefined>(undefined);
  const [openId, setOpenId] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;
    void (async () => {
      try {
        const list = await api.getGrowthHistory();
        if (cancelled) return;
        setEntries(list);
        if (list.length > 0) setOpenId(`${list[0]!.surface}:${list[0]!.scopeId}`); // newest expanded
      } catch {
        if (!cancelled) setError("加载失败，请重试");
      }
    })();
    return () => { cancelled = true; };
  }, []);

  if (entries === undefined) {
    return <div style={{ padding: 40, color: "#9AA1B0" }}>正在整理你的成长报告…</div>;
  }

  return (
    <div style={{ flex: 1, minHeight: 0, height: "100%", overflowY: "auto", background: "#F3F4F8" }}>
      <div style={{ maxWidth: 760, margin: "0 auto", padding: "34px 40px 56px" }}>
        <div style={{ background: "linear-gradient(135deg,#2A3B7A 0%,#34468C 100%)", borderRadius: 20, padding: "28px 30px", display: "flex", alignItems: "center", gap: 20, boxShadow: "0 10px 30px rgba(42,59,122,.20)" }}>
          <div style={{ flex: "none", width: 60, height: 60, borderRadius: 18, background: "rgba(255,255,255,.14)", display: "flex", alignItems: "center", justifyContent: "center" }}>
            <svg width="30" height="30" viewBox="0 0 24 24" fill="none" stroke="#fff" strokeWidth="2.4" strokeLinecap="round" strokeLinejoin="round"><path d="M12 2a5 5 0 0 0-5 5c0 2 1 3 1 5v2a2 2 0 0 0 2 2h4a2 2 0 0 0 2-2v-2c0-2 1-3 1-5a5 5 0 0 0-5-5z" /><path d="M9 21h6" /></svg>
          </div>
          <div style={{ flex: 1, minWidth: 0 }}>
            <div style={{ fontSize: 11, fontWeight: 700, letterSpacing: ".1em", color: "#AEB8E4" }}>成长报告</div>
            <div style={{ fontSize: 22, fontWeight: 800, color: "#fff", marginTop: 6, lineHeight: 1.3 }}>你的思维印记</div>
            <div style={{ fontSize: 13, color: "#C3CBEC", marginTop: 6 }}>每完成一个任务、一节课，或在聊天里生成一次思维印记，都会汇集到这里——按真实过程给出的诊断，不是分数。</div>
          </div>
        </div>

        {error && (
          <div style={{ marginTop: 14, fontSize: 13, color: "#B0432E", background: "#FBEDEA", borderRadius: 10, padding: "10px 14px" }}>{error}</div>
        )}

        {entries.length === 0 ? (
          <div style={{ background: "#fff", border: "1px solid #EAECF2", borderRadius: 16, padding: "34px 24px", marginTop: 16, textAlign: "center" }}>
            <div style={{ fontSize: 14.5, color: "#3A4256", fontWeight: 700 }}>还没有报告</div>
            <div style={{ fontSize: 13.5, color: "#6B7384", lineHeight: 1.7, maxWidth: 420, margin: "8px auto 0" }}>完成一个任务、一节课，或在聊天里生成一次思维印记，报告会在这里汇集。</div>
          </div>
        ) : (
          entries.map((e) => {
            const id = `${e.surface}:${e.scopeId}`;
            return <HistoryRow key={id} entry={e} open={openId === id} onToggle={() => setOpenId(openId === id ? null : id)} />;
          })
        )}
      </div>
    </div>
  );
}
```

- [ ] **Step 2: Rewrite `GrowthReport.test.tsx`**

Replace it to mock `api.getGrowthHistory` (not `listProjects`/`getAssessment`/`generateAssessment`). Cover: empty state (no entries), a rendered row that expands to show a dimension + narrative, and that NO generate/重新生成 button exists.

```tsx
import { describe, expect, it, vi, beforeEach } from "vitest";
import { fireEvent, render, screen } from "@testing-library/react";
import { GrowthReport } from "./GrowthReport";
import { api } from "../../api";

describe("GrowthReport", () => {
  beforeEach(() => vi.restoreAllMocks());

  it("shows the empty state with no reports", async () => {
    vi.spyOn(api, "getGrowthHistory").mockResolvedValue([]);
    render(<GrowthReport />);
    expect(await screen.findByText("还没有报告")).toBeTruthy();
    expect(screen.queryByText(/生成/)).toBeNull(); // no generate / 重新生成
  });

  it("renders a history row and expands it to the report", async () => {
    vi.spyOn(api, "getGrowthHistory").mockResolvedValue([
      { surface: "project", scopeId: "p1", label: "中国可持续", sublabel: null, createdAt: "2026-07-18T00:00:00Z",
        report: { dimensions: [{ code: "D1", name: "追问", level: "L3", evidence: "追问了三次" }], narrative: "你的印记…", generatedAt: "2026-07-18T00:00:00Z" } },
    ]);
    render(<GrowthReport />);
    expect(await screen.findByText("中国可持续")).toBeTruthy();
    // newest is auto-expanded → dimension + narrative visible
    expect(screen.getByText("追问")).toBeTruthy();
    expect(screen.getByText("你的印记…")).toBeTruthy();
    expect(screen.queryByText(/生成/)).toBeNull();
  });
});
```

- [ ] **Step 3: Run web tests + typecheck**

Run: `cd apps/web && npm test`
Expected: PASS (GrowthReport suite green; any Task 7-noted failures now resolved).
Run: `cd apps/web && npx tsc --noEmit`
Expected: exit 0.

- [ ] **Step 4: Commit**

```bash
git add apps/web/src/shell/growth/GrowthReport.tsx apps/web/src/shell/growth/GrowthReport.test.tsx
git commit -m "feat(a3): 成长报告 becomes a read-only cross-surface history hub"
```

---

### Task 9: Web — studio finish button + tab routing

**Files:**
- Modify: `apps/web/src/studio/views/ReviewView.tsx` (finish button)
- Modify: `apps/web/src/studio/StudioContainer.tsx` (thread `finished`/`canFinish` + `onFinish` + `onFinished` prop)
- Modify: `apps/web/src/shell/StudentApp.tsx` (pass `onFinished={() => setTab("growth")}`)
- Modify: `apps/web/src/studio/state.ts` (carry `finished`/`canFinish` into `StudioState` if that's how views receive projection data)
- Test: `apps/web/src/studio/views/ReviewView.test.tsx`, `apps/web/src/studio/StudioContainer.test.tsx`

**Interfaces:**
- Consumes: `proj.finished` / `proj.canFinish` (Task 3), `api.finishProject` (Task 7).
- Produces: 完成任务·归档 in the 就绪度 view when `canFinish`; 已归档 when `finished`; on success routes to the 成长报告 tab.

- [ ] **Step 1: Carry the flags into the view model**

`StudioContainer.toStudioState(p)` maps the projection into `StudioState`. Add `finished: p.finished` and `canFinish: p.canFinish` to `StudioState` (in `apps/web/src/studio/state.ts`) and set them in `toStudioState`. (Grep `toStudioState` + `type StudioState` first to match exact shapes.)

- [ ] **Step 2: Add the finish button to `ReviewView`**

Extend `ReviewViewProps` and render the terminal below the gauges:

```tsx
export type ReviewViewProps = {
  gauges: GaugeFx[];
  canFinish: boolean;
  finished: boolean;
  finishing: boolean;
  finishError: string | null;
  onFinish: () => void;
};
```

At the end of the `ReviewView` return (after the gauges grid `</div>`), add:

```tsx
        {finished ? (
          <div style={{ marginTop: 22, textAlign: "center", fontSize: 13.5, fontWeight: 700, color: "#4C9A82" }}>已归档 · 成长报告已生成</div>
        ) : canFinish ? (
          <div style={{ marginTop: 22, display: "flex", flexDirection: "column", alignItems: "center", gap: 10 }}>
            <button
              type="button"
              onClick={onFinish}
              disabled={finishing}
              style={{ display: "inline-flex", alignItems: "center", gap: 8, background: finishing ? "#9CB6A9" : "#4C9A82", color: "#fff", border: "none", fontSize: 14.5, fontWeight: 700, padding: "13px 26px", borderRadius: 12, cursor: finishing ? "default" : "pointer", fontFamily: "inherit" }}
            >
              <svg width="17" height="17" viewBox="0 0 24 24" fill="none" stroke="#fff" strokeWidth="2.6" strokeLinecap="round" strokeLinejoin="round"><path d="M20 6L9 17l-5-5" /></svg>
              {finishing ? "正在归档…" : "完成任务 · 归档"}
            </button>
            {finishError && <div style={{ fontSize: 12.5, color: "#B0432E" }}>{finishError}</div>}
          </div>
        ) : null}
```

- [ ] **Step 3: Wire the handler + tab routing in `StudioContainer`**

- Add prop: `onFinished?: () => void` to `StudioContainer`'s props.
- Add state: `const [finishing, setFinishing] = useState(false); const [finishError, setFinishError] = useState<string | null>(null);`
- Add `finishProject` to the `Pick<...>` api type at the top of the file (line ~14).
- Add the handler:

```tsx
  async function handleFinish() {
    if (!projectId || finishing) return;
    setFinishing(true); setFinishError(null);
    try {
      await api.finishProject(projectId);
      onFinished?.(); // route to 成长报告
    } catch {
      setFinishError("归档失败，请重试");
    } finally {
      setFinishing(false);
    }
  }
```

- Where `ReviewView` is rendered, pass the new props: `canFinish={state.canFinish} finished={state.finished} finishing={finishing} finishError={finishError} onFinish={handleFinish}`.

- [ ] **Step 4: Pass `onFinished` from `StudentApp`**

In `apps/web/src/shell/StudentApp.tsx`, change line 27:

```tsx
        {tab === "studio" && <StudioContainer onFinished={() => setTab("growth")} />}
```

- [ ] **Step 5: Update tests**

- `ReviewView.test.tsx`: add cases — with `canFinish` true, the 完成任务 · 归档 button renders and `onFinish` fires on click; with `finished` true, 已归档 renders and no button; with both false, neither.
- `StudioContainer.test.tsx`: its projection mock/fixture must include `finished`/`canFinish` (add `finished: false, canFinish: false` where it builds a `StudioProjection`). Add a case: projection with `canFinish: true` → clicking the finish button calls `api.finishProject` and then `onFinished`.

- [ ] **Step 6: Run web tests + typecheck**

Run: `cd apps/web && npm test`
Expected: PASS (all suites).
Run: `cd apps/web && npx tsc --noEmit`
Expected: exit 0.

- [ ] **Step 7: Commit**

```bash
git add apps/web/src/studio/views/ReviewView.tsx apps/web/src/studio/StudioContainer.tsx apps/web/src/studio/state.ts apps/web/src/shell/StudentApp.tsx apps/web/src/studio/views/ReviewView.test.tsx apps/web/src/studio/StudioContainer.test.tsx
git commit -m "feat(a3): studio 完成任务·归档 button + route to 成长报告"
```

---

## Final verification (before whole-branch review)

- [ ] `cd apps/api && DOCKER_HOST=unix:///var/run/docker.sock CGO_ENABLED=0 go test -p 1 ./...` — FULL suite green.
- [ ] `cd apps/api && git status --porcelain apps/api/internal/store/sqlc` — clean (no uncommitted sqlc drift).
- [ ] `cd apps/web && npm test` — green; `cd apps/web && npx tsc --noEmit` — exit 0.
- [ ] `cd packages/contracts && npm test` — green.
- [ ] `grep -rn "generateAssessment" apps/api apps/web` — no matches (route + handler + web method fully removed). If `apps/web/e2e/RUNBOOK.md` or an e2e test references the removed POST, update it.
- [ ] `grep -rn "重新生成" apps/web/src/shell/growth apps/web/src/studio` — no matches.
- [ ] Confirm A1/A2 files untouched: `git diff --name-only main -- apps/api/internal/api/course_assessment.go apps/api/internal/api/chat_assessment.go apps/web/src/shell/courses/CourseReport.tsx apps/web/src/shell/chat/ChatReport.tsx` — empty.
```
