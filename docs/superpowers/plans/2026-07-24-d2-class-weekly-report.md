# D2 · Class Weekly Report Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the teacher's 班级周报 — live weekly usage numbers, rule-detected praise/watch cards with verbatim evidence, one flagship LLM call per class per week that writes the wording, and the D/A class distribution.

**Architecture:** Three layers. SQL aggregates the event stream and each student's latest two canonical reports. A pure Go rule layer (`internal/teacher`) turns those rows into every number and every judgment on the screen, and emits a fact sheet. One flagship call (`internal/agent`) turns the fact sheet into prose, stored once per (class, week). The GET endpoint never calls a model; a separate POST is the only spend.

**Tech Stack:** Go (`net/http`, `pgx`, `sqlc`, `goose`), PostgreSQL, React + TypeScript + Vite (`apps/web`), vitest + Testing Library.

Spec: `docs/superpowers/specs/2026-07-24-d2-class-weekly-report-design.md`. Read it once before Task 1; every task's brief references it.

## Global Constraints

- Client NEVER calls a model directly. All LLM calls go through the backend gateway; keys live only in `apps/api` server env and must never appear in git, logs, thrown or rendered errors, stored data, or eval payloads.
- The composition call resolves through `a.d.EvalResolver` (**flagship, never downgraded**) and is metered as an `llm_call` row with `Surface: "teacher"`, `Purpose: "class_weekly"`. Cost is recorded even when the output is rejected.
- `GET /api/v1/classes/{id}/weekly-report` makes **no model call under any circumstance**.
- 单向只读: nothing this spec adds may write to, notify, or otherwise reach a student.
- 每个判断带证据: every card carries evidence excerpted verbatim from the canonical report or a bare statement of fact. No evidence → no card.
- 敢于空白: absent evidence renders 「暂无可计入的证据」; an unrated value renders `—` (em-dash U+2014). Never fabricate a substitute.
- RL-5: the two axes never combine into a total. No composite class score exists anywhere in this work.
- One derivation per quantity. `teacher.AMean` / `teacher.DLevels` are the only implementations; `DBadge`/`ABadge` call them; the web derives **no** score, mean, or badge — it renders the strings the server sent.
- 对话轮次 口径 is fixed and shared with D1: `type IN ('prompt_sent','course_message')`.
- All time windows are UTC-pinned. `teacher.WeekWindow` is the only week-window implementation.
- Regenerate sqlc with `make sqlc` from `apps/api`. NEVER hand-edit `apps/api/internal/store/sqlc/*` — it is generated.
- Icons are inline SVG. Never import `lucide-react`.
- Go tests: `cd apps/api && DOCKER_HOST=unix:///var/run/docker.sock CGO_ENABLED=0 go test -p 1 ./...`. Run **full packages** — never `-run` subsets — for any task touching cards, gates, projections, creation, config, migrations, or queries. That is every backend task in this plan.
- Web tests: `cd apps/web && npm test` and `npx tsc --noEmit`. Web tests live in `apps/web/test/**` and import source via the `@/` alias (e.g. `@/console/ClassWeeklyView`).
- **Run every test command in the FOREGROUND.** Never use `run_in_background`, `&`, or `nohup` — a backgrounded test run strands the task.
- NEVER `git add` a whole directory. The repo has pre-existing unstaged `package.json` changes and untracked `docs/` and PNG files that are NOT part of this work. Name every file explicitly.
- New migrations: `0031`, `0032`, `0033`, `0034` in that order (latest on main is `0030`).
- Existing UI copy is binding. Chinese strings quoted in this plan are exact — including 「」 brackets, the en-dash U+2013 in ranges, and the em-dash U+2014 as the unrated marker.

---

## File Structure

**Create (backend)**
- `apps/api/internal/teacher/week.go` — `WeekWindow` / `PrevWindow` / `WeekLabel`. Pure time math, UTC-pinned.
- `apps/api/internal/teacher/week_test.go`
- `apps/api/internal/teacher/weekly.go` — the rule layer: `StudentWeek`, `Detect`, `Stats`, `BuildWeeklyFacts`. Pure.
- `apps/api/internal/teacher/weekly_test.go`
- `apps/api/internal/agent/compose_weekly.go` — `WeeklyFacts`, `WeeklyProse`, `ComposeWeekly`, validation.
- `apps/api/internal/agent/compose_weekly_test.go`
- `apps/api/internal/api/teacher_weekly.go` — the two handlers + DTOs.
- `apps/api/internal/api/teacher_weekly_test.go`
- `apps/api/internal/store/queries/teacher_weekly.sql` — weekly aggregation queries.
- `apps/api/internal/store/migrations/0031_event_course_scope.sql`
- `apps/api/internal/store/migrations/0032_student_evaluation_view.sql`
- `apps/api/internal/store/migrations/0033_class_weekly_prose.sql`
- `apps/api/internal/store/migrations/0034_seed_teacher_week.sql`

**Create (web)**
- `apps/web/src/console/ClassWeeklyView.tsx` — the 周报 screen.
- `apps/web/src/console/ClassRosterTable.tsx` — D1's roster table, extracted verbatim.
- `apps/web/test/console/ClassWeeklyView.test.tsx`

**Modify**
- `apps/api/internal/teacher/badges.go` — `DBadge`/`ABadge` refactored onto the new numeric helpers.
- `apps/api/internal/api/teacher_read.go` — drop the local `weekWindow`, call `teacher.WeekWindow`.
- `apps/api/internal/store/queries/teacher.sql` — roster + head latest-report queries go cross-surface.
- `apps/api/internal/api/course_render.go` — emit `step_viewed`.
- `apps/api/internal/api/api.go` — register two routes.
- `apps/web/src/api/teacher.ts` — two client calls + types.
- `apps/web/src/api/index.ts` — aggregate the new calls into `ApiClient`.
- `apps/web/src/console/ClassDetailView.tsx` — sub-tabs; roster table moves out.

---

### Task 1: Numeric helpers + the shared week window

**Files:**
- Create: `apps/api/internal/teacher/week.go`, `apps/api/internal/teacher/week_test.go`
- Modify: `apps/api/internal/teacher/badges.go`, `apps/api/internal/teacher/badges_test.go`, `apps/api/internal/api/teacher_read.go:36-51,68,181`

**Interfaces:**
- Produces: `teacher.AMean(agent.Report) (float64, bool)`, `teacher.DLevels(agent.Report) (min, max int, ok bool)`, `teacher.WeekWindow(time.Time) (time.Time, time.Time)`, `teacher.PrevWindow(time.Time) (time.Time, time.Time)`, `teacher.WeekLabel(time.Time) string`.
- Consumes: nothing.

- [ ] **Step 1: Write the failing tests for the numeric helpers**

Append to `apps/api/internal/teacher/badges_test.go`:

```go
func TestAMeanExcludesNotSupplied(t *testing.T) {
	r := agent.Report{AutonomyAxis: []agent.AutonomySignal{
		{Code: "A1", Level: 5, Opportunity: "given_taken"},
		{Code: "A2", Level: 4, Opportunity: "given_taken"},
		{Code: "A3", Level: 4, Opportunity: "given_not_taken"},
		{Code: "A4", Level: 4, Opportunity: "given_taken"},
		{Code: "A5", Level: 0, Opportunity: "not_supplied"},
		{Code: "A6", Level: 0, Opportunity: "not_supplied"},
	}}
	got, ok := teacher.AMean(r)
	if !ok || got != 4.25 {
		t.Fatalf("AMean = %v, %v; want 4.25, true", got, ok)
	}
}

func TestAMeanNoSuppliedSignal(t *testing.T) {
	r := agent.Report{AutonomyAxis: []agent.AutonomySignal{
		{Code: "A1", Level: 0, Opportunity: "not_supplied"},
	}}
	if _, ok := teacher.AMean(r); ok {
		t.Fatal("AMean ok = true; want false when nothing was supplied")
	}
}

func TestDLevelsMinMax(t *testing.T) {
	r := agent.Report{DepthAxis: []agent.DepthDim{
		{Code: "D1", Level: "L3"}, {Code: "D2", Level: "L4"},
		{Code: "D3", Level: "NA"}, {Code: "D4", Level: "L3"},
	}}
	min, max, ok := teacher.DLevels(r)
	if !ok || min != 3 || max != 4 {
		t.Fatalf("DLevels = %d,%d,%v; want 3,4,true", min, max, ok)
	}
}

func TestDLevelsUnrated(t *testing.T) {
	if _, _, ok := teacher.DLevels(agent.Report{DepthAxis: []agent.DepthDim{{Code: "D1", Level: "NA"}}}); ok {
		t.Fatal("DLevels ok = true; want false when no dim carries a level")
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `cd apps/api && CGO_ENABLED=0 go test ./internal/teacher/...`
Expected: FAIL — `undefined: teacher.AMean`, `undefined: teacher.DLevels`.

- [ ] **Step 3: Add the helpers and refactor the badges onto them**

Replace the bodies in `apps/api/internal/teacher/badges.go`, keeping the package doc comment and `depthRank`:

```go
// DLevels reports the min and max rated depth levels (1..4). ok is false when
// no dimension carries a real level (NA/unknown are skipped — no evidence, no
// reading). The single source for every depth-level derivation: DBadge renders
// it, and D2's class distribution buckets on max.
func DLevels(r agent.Report) (min, max int, ok bool) {
	for _, d := range r.DepthAxis {
		rank, found := depthRank[d.Level]
		if !found {
			continue
		}
		if min == 0 || rank < min {
			min = rank
		}
		if rank > max {
			max = rank
		}
	}
	return min, max, min != 0
}

// AMean is the mean autonomy level over signals whose opportunity was actually
// supplied (机会供给先于判定: not_supplied is platform debt, excluded). ok is
// false when no signal was supplied. The single source for every autonomy mean:
// ABadge formats it, and D2's class mean averages it.
func AMean(r agent.Report) (float64, bool) {
	sum, n := 0, 0
	for _, a := range r.AutonomyAxis {
		if a.Opportunity == "not_supplied" {
			continue
		}
		sum += a.Level
		n++
	}
	if n == 0 {
		return 0, false
	}
	return float64(sum) / float64(n), true
}

// DBadge summarises the six depth levels as a min–max range (en-dash), a single
// level when they coincide, or "—" when no dimension carries a real level.
func DBadge(r agent.Report) string {
	min, max, ok := DLevels(r)
	if !ok {
		return "—"
	}
	if min == max {
		return fmt.Sprintf("L%d", min)
	}
	return fmt.Sprintf("L%d–L%d", min, max) // U+2013 en-dash
}

// ABadge renders AMean to one decimal, or "—" when no signal was supplied.
func ABadge(r agent.Report) string {
	m, ok := AMean(r)
	if !ok {
		return "—"
	}
	return fmt.Sprintf("%.1f", m)
}
```

- [ ] **Step 4: Run the teacher package to verify the refactor is byte-identical**

Run: `cd apps/api && CGO_ENABLED=0 go test ./internal/teacher/...`
Expected: PASS, including every pre-existing `DBadge`/`ABadge` test — they are the guard that the refactor changed no output.

- [ ] **Step 5: Write the failing week-window tests**

Create `apps/api/internal/teacher/week_test.go`:

```go
package teacher_test

import (
	"testing"
	"time"

	"mindimprint/api/internal/teacher"
)

func TestWeekWindowStartsMondayUTC(t *testing.T) {
	// 2026-07-24 is a Friday, 15:30 in +08:00 → 07:30 UTC the same day.
	now := time.Date(2026, 7, 24, 15, 30, 0, 0, time.FixedZone("CST", 8*3600))
	start, end := teacher.WeekWindow(now)
	wantStart := time.Date(2026, 7, 20, 0, 0, 0, 0, time.UTC)
	if !start.Equal(wantStart) {
		t.Fatalf("start = %v; want %v", start, wantStart)
	}
	if !end.Equal(wantStart.AddDate(0, 0, 7)) {
		t.Fatalf("end = %v; want start+7d", end)
	}
	if start.Location() != time.UTC || end.Location() != time.UTC {
		t.Fatal("window must be pinned to UTC")
	}
}

func TestWeekWindowMondayIsItsOwnStart(t *testing.T) {
	now := time.Date(2026, 7, 20, 0, 0, 0, 0, time.UTC)
	start, _ := teacher.WeekWindow(now)
	if !start.Equal(now) {
		t.Fatalf("start = %v; want the Monday itself", start)
	}
}

func TestWeekWindowSundayStaysInTheSameWeek(t *testing.T) {
	now := time.Date(2026, 7, 26, 23, 59, 0, 0, time.UTC)
	start, end := teacher.WeekWindow(now)
	if !start.Equal(time.Date(2026, 7, 20, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("start = %v; want 2026-07-20", start)
	}
	if !now.Before(end) {
		t.Fatal("Sunday 23:59 must fall inside its own week window")
	}
}

func TestPrevWindowIsTheSameElapsedOffset(t *testing.T) {
	now := time.Date(2026, 7, 22, 9, 15, 0, 0, time.UTC) // Wednesday 09:15
	ps, pe := teacher.PrevWindow(now)
	if !ps.Equal(time.Date(2026, 7, 13, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("prev start = %v; want 2026-07-13", ps)
	}
	if !pe.Equal(time.Date(2026, 7, 15, 9, 15, 0, 0, time.UTC)) {
		t.Fatalf("prev end = %v; want the same elapsed offset (Wed 09:15)", pe)
	}
}

func TestWeekLabel(t *testing.T) {
	got := teacher.WeekLabel(time.Date(2026, 7, 20, 0, 0, 0, 0, time.UTC))
	want := "第 30 周（7.20–7.26）"
	if got != want {
		t.Fatalf("WeekLabel = %q; want %q", got, want)
	}
}
```

- [ ] **Step 6: Run to verify they fail**

Run: `cd apps/api && CGO_ENABLED=0 go test ./internal/teacher/...`
Expected: FAIL — `undefined: teacher.WeekWindow`.

- [ ] **Step 7: Implement the week functions**

Create `apps/api/internal/teacher/week.go`:

```go
package teacher

import (
	"fmt"
	"time"
)

// WeekWindow returns the half-open [Mon 00:00, next Mon 00:00) window enclosing
// now, pinned to UTC.
//
// Pinned to UTC (not now's/the server's local location) because the DB side
// buckets `event.created_at` via `AT TIME ZONE 'UTC')::date` — if this window
// were built in server-local time (e.g. Asia/Shanghai) while Postgres computes
// dates in UTC, a 7×24h window can straddle 8 distinct UTC calendar dates and
// activeDays could read 8.
//
// This is the ONLY week-window implementation. D1's roster (本周活跃) and D2's
// weekly report both call it, so the two adjacent screens cannot disagree.
func WeekWindow(now time.Time) (time.Time, time.Time) {
	now = now.UTC()
	y, m, d := now.Date()
	midnight := time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
	offset := (int(now.Weekday()) + 6) % 7 // Monday=0
	start := midnight.AddDate(0, 0, -offset)
	return start, start.AddDate(0, 0, 7)
}

// PrevWindow returns last week's comparison window: last Monday 00:00 UTC
// through the SAME elapsed offset into that week that now sits at in this one.
// Week-to-date is compared against week-to-the-same-moment, so opening on a
// Wednesday does not show every metric down purely because three days have
// passed.
func PrevWindow(now time.Time) (time.Time, time.Time) {
	start, _ := WeekWindow(now)
	elapsed := now.UTC().Sub(start)
	prevStart := start.AddDate(0, 0, -7)
	return prevStart, prevStart.Add(elapsed)
}

// WeekLabel renders the binding design's 周报 header, e.g. 第 30 周（7.20–7.26）.
// The range names the full Monday–Sunday week even though the data is
// week-to-date; the DTO's asOf states the cut.
func WeekLabel(weekStart time.Time) string {
	weekStart = weekStart.UTC()
	_, week := weekStart.ISOWeek()
	end := weekStart.AddDate(0, 0, 6)
	return fmt.Sprintf("第 %d 周（%d.%d–%d.%d）", week,
		int(weekStart.Month()), weekStart.Day(), int(end.Month()), end.Day()) // U+2013 en-dash
}
```

- [ ] **Step 8: Run to verify they pass**

Run: `cd apps/api && CGO_ENABLED=0 go test ./internal/teacher/...`
Expected: PASS.

- [ ] **Step 9: Repoint the API's week window at the shared one**

In `apps/api/internal/api/teacher_read.go`, delete the whole local `weekWindow` function (lines 36–51 including its doc comment) and replace both call sites — `start, end := weekWindow(time.Now())` at lines 68 and 181 — with `start, end := teacher.WeekWindow(time.Now())`. The `teacher` import already exists; remove the `time` import only if nothing else in the file uses it (it does — `time.RFC3339` and `time.Time` — so keep it).

- [ ] **Step 10: Run the full api package**

Run: `cd apps/api && DOCKER_HOST=unix:///var/run/docker.sock CGO_ENABLED=0 go test -p 1 ./internal/api/... ./internal/teacher/...`
Expected: PASS. This package takes ~200s; run it in the FOREGROUND and wait.

- [ ] **Step 11: Commit**

```bash
cd /Users/houyuxin/08Coding/mind-imprint
git add apps/api/internal/teacher/week.go apps/api/internal/teacher/week_test.go apps/api/internal/teacher/badges.go apps/api/internal/teacher/badges_test.go apps/api/internal/api/teacher_read.go
git commit -m "refactor(d2): single-source D/A numeric derivation and the week window"
```

---

### Task 2: Migration 0031 — course scope on events + `step_viewed`

**Files:**
- Create: `apps/api/internal/store/migrations/0031_event_course_scope.sql`
- Modify: `apps/api/internal/api/course_render.go:40-60`
- Test: `apps/api/internal/store/migrate_test.go` (add one test), `apps/api/internal/api/course_render_test.go` (add one test)

**Interfaces:**
- Produces: an `event` row with `surface='course'`, `type='step_viewed'`, `course_id` set, `payload={"ordinal":N}` on every course-step render. `event.course_id` becomes a valid scope for `event_scope_ck`.
- Consumes: nothing.

Read `apps/api/internal/store/migrations/0025_thread_assessment_scope.sql` first — this migration copies its shape exactly, including the Down's delete-before-restore.

- [ ] **Step 1: Write the migration**

Create `apps/api/internal/store/migrations/0031_event_course_scope.sql`:

```sql
-- +goose Up
-- D2: give the event stream a course scope, so a course page-turn can be
-- recorded as a timestamped event. Until now course progress lived ONLY in
-- course_progress.completed_ordinals — a mutable int array with no timestamp —
-- so "course steps completed this week" was unqueryable. Additive, mirroring
-- 0024's session scope and 0025's thread scope.
ALTER TABLE event ADD COLUMN course_id uuid REFERENCES course(id) ON DELETE CASCADE;
CREATE INDEX event_course_created_idx ON event (course_id, created_at);

-- Widen the scope check. Still NOT VALID for the same reason 0024/0025 gave:
-- pre-existing unscoped rows have nothing to backfill from and stay
-- grandfathered, while every new insert is enforced.
ALTER TABLE event DROP CONSTRAINT event_scope_ck;
ALTER TABLE event ADD CONSTRAINT event_scope_ck
  CHECK (num_nonnulls(project_id, session_id, thread_id, course_id) >= 1) NOT VALID;

-- +goose Down
-- Course-only-scoped rows have no other scope; the restored narrower CHECK
-- would reject them and abort the whole Down exactly where a step_viewed
-- exists. Delete them explicitly (mirrors 0025's Down for thread-scoped rows).
ALTER TABLE event DROP CONSTRAINT IF EXISTS event_scope_ck;
DELETE FROM event
 WHERE course_id IS NOT NULL AND project_id IS NULL AND session_id IS NULL AND thread_id IS NULL;
ALTER TABLE event ADD CONSTRAINT event_scope_ck
  CHECK (num_nonnulls(project_id, session_id, thread_id) >= 1) NOT VALID;
DROP INDEX IF EXISTS event_course_created_idx;
ALTER TABLE event DROP COLUMN IF EXISTS course_id;
```

- [ ] **Step 2: Regenerate sqlc and verify the migration applies both ways**

Run: `cd apps/api && make sqlc`
Then add to `apps/api/internal/store/migrate_test.go`, following the existing `UpToContext` idiom already used there for migration 0027:

```go
func TestMigrate0031EventCourseScopeRoundTrips(t *testing.T) {
	pool := newStoreTestPool(t)
	ctx := context.Background()
	// Down to 0030, then back up — the Down must not abort on course-scoped rows.
	if err := goose.DownToContext(ctx, stdlibDB(t, pool), migrationsDir, 30); err != nil {
		t.Fatalf("down to 0030: %v", err)
	}
	if err := goose.UpToContext(ctx, stdlibDB(t, pool), migrationsDir, 31); err != nil {
		t.Fatalf("up to 0031: %v", err)
	}
	var n int
	if err := pool.QueryRow(ctx,
		`SELECT count(*) FROM information_schema.columns WHERE table_name='event' AND column_name='course_id'`,
	).Scan(&n); err != nil {
		t.Fatalf("query column: %v", err)
	}
	if n != 1 {
		t.Fatalf("event.course_id column count = %d; want 1", n)
	}
}
```

Match the helper names actually present in `migrate_test.go` (`newStoreTestPool`, `stdlibDB`, `migrationsDir` — read the file and use its real names; do not invent helpers).

Run: `cd apps/api && DOCKER_HOST=unix:///var/run/docker.sock CGO_ENABLED=0 go test -p 1 ./internal/store/...`
Expected: PASS.

- [ ] **Step 3: Write the failing render test**

Read `apps/api/internal/api/course_render.go` around line 51 first. Add to `apps/api/internal/api/course_render_test.go` (create it if absent, following the harness in `course_test.go`):

```go
func TestCourseRenderEmitsStepViewedEvent(t *testing.T) {
	pool := newAPITestPool(t)
	a := New(DepsForTest(t, pool))
	student := signInSeed(t, pool)

	// Render step 1 of the seeded course.
	req := withCookie(httptest.NewRequest(http.MethodPost,
		"/api/v1/courses/"+courseSeededCourseID+"/steps/1/render", nil), student)
	rec := httptest.NewRecorder()
	a.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("render status = %d, body %s", rec.Code, rec.Body.String())
	}

	var typ, ordinal string
	var courseID pgtype.UUID
	err := pool.QueryRow(context.Background(),
		`SELECT type, course_id, payload->>'ordinal' FROM event
		  WHERE user_id = $1 AND type = 'step_viewed' ORDER BY created_at DESC LIMIT 1`,
		SeedUserID).Scan(&typ, &courseID, &ordinal)
	if err != nil {
		t.Fatalf("no step_viewed event recorded: %v", err)
	}
	if !courseID.Valid {
		t.Fatal("step_viewed event has no course_id — it would violate event_scope_ck")
	}
	if ordinal != "1" {
		t.Fatalf("payload ordinal = %q; want \"1\"", ordinal)
	}
}
```

Adapt the request construction and sign-in helper to whatever `course_test.go` actually uses in this repo; the assertions above are the point.

- [ ] **Step 4: Run to verify it fails**

Run: `cd apps/api && DOCKER_HOST=unix:///var/run/docker.sock CGO_ENABLED=0 go test -p 1 ./internal/api/...`
Expected: FAIL — `no step_viewed event recorded`.

- [ ] **Step 5: Emit the event**

In `apps/api/internal/api/course_render.go`, immediately after the existing `RecordCourseStepViewed` call, add:

```go
	// D2: the timestamped twin of the completed_ordinals write above, so
	// "课程节 completed this week" is queryable. Best-effort like the metering
	// write — a failed event must never fail a student's page render.
	if _, eerr := a.d.Queries.AppendEvent(r.Context(), sqlc.AppendEventParams{
		UserID:    u.ID,
		ProjectID: pgtype.UUID{Valid: false},
		SessionID: pgtype.UUID{Valid: false},
		ThreadID:  pgtype.UUID{Valid: false},
		CourseID:  pgtype.UUID{Bytes: courseID, Valid: true},
		Surface:   "course",
		Type:      "step_viewed",
		Payload:   []byte(fmt.Sprintf(`{"ordinal":%d}`, ordinal)),
	}); eerr != nil {
		slog.Warn("course_render: append step_viewed", "err", eerr)
	}
```

Use the local variable names already in scope in that handler for the user, course id, and ordinal — read the surrounding code. `AppendEventParams` gains `CourseID` after `make sqlc`; if the generated struct lacks it, re-run `make sqlc`.

- [ ] **Step 6: Run to verify it passes**

Run: `cd apps/api && DOCKER_HOST=unix:///var/run/docker.sock CGO_ENABLED=0 go test -p 1 ./internal/api/... ./internal/store/...`
Expected: PASS.

- [ ] **Step 7: Commit**

```bash
cd /Users/houyuxin/08Coding/mind-imprint
git add apps/api/internal/store/migrations/0031_event_course_scope.sql apps/api/internal/store/sqlc apps/api/internal/api/course_render.go apps/api/internal/api/course_render_test.go apps/api/internal/store/migrate_test.go
git commit -m "feat(d2): course scope on events + step_viewed on render"
```

---

### Task 3: Migration 0032 — `student_evaluation` view + cross-surface latest report

**Files:**
- Create: `apps/api/internal/store/migrations/0032_student_evaluation_view.sql`
- Modify: `apps/api/internal/store/queries/teacher.sql` (`ListClassRosterReport`, `GetLatestProjectScoresForStudent`)
- Test: `apps/api/internal/store/teacher_queries_test.go` (the existing D1 store suite)

**Interfaces:**
- Produces: SQL view `student_evaluation (id, user_id, scores, created_at, surface, scope_id)` — one row per evaluation, attributed to its owning student across all three scopes. Query `GetLatestReportsForStudent` is renamed from `GetLatestProjectScoresForStudent` and returns cross-surface rows.
- Consumes: nothing.

**Why:** D1's roster badge reads the latest *project* evaluation only. A class distribution built on that would omit every course/chat-only student from 「已评估 N 人」 — the same student would read 已评估 on the roster and unrated in the distribution. One ownership definition, in a view, so the two cannot drift.

- [ ] **Step 1: Write the migration**

Create `apps/api/internal/store/migrations/0032_student_evaluation_view.sql`:

```sql
-- +goose Up
-- D2: one definition of "this evaluation belongs to this student", across all
-- three scopes. Before this, every caller re-JOINed project/course_session/
-- chat_thread by hand and D1's roster silently read project scope only.
-- Casts are explicit so sqlc infers concrete types (a bare CASE/COALESCE comes
-- back as interface{} — see D1's ListClassRosterReport.HasReport).
CREATE VIEW student_evaluation AS
SELECT
  e.id,
  COALESCE(p.user_id, cs.user_id, t.user_id)::uuid            AS user_id,
  e.scores,
  e.created_at,
  (CASE WHEN e.project_id IS NOT NULL THEN 'project'
        WHEN e.session_id IS NOT NULL THEN 'course'
        ELSE 'chat' END)::text                                AS surface,
  COALESCE(e.project_id, e.session_id, e.thread_id)::uuid     AS scope_id
FROM evaluations e
LEFT JOIN project        p  ON p.id  = e.project_id
LEFT JOIN course_session cs ON cs.id = e.session_id
LEFT JOIN chat_thread    t  ON t.id  = e.thread_id
WHERE COALESCE(p.user_id, cs.user_id, t.user_id) IS NOT NULL;

-- +goose Down
DROP VIEW IF EXISTS student_evaluation;
```

- [ ] **Step 2: Repoint the two D1 queries**

In `apps/api/internal/store/queries/teacher.sql`, replace the `ListClassRosterReport` scores lateral:

```sql
LEFT JOIN LATERAL (
  SELECT se.scores
  FROM student_evaluation se
  WHERE se.user_id = u.id
  ORDER BY se.created_at DESC
  LIMIT 1
) ev ON true
```

and replace `GetLatestProjectScoresForStudent` entirely with:

```sql
-- name: GetLatestReportScoresForStudent :one
-- Latest report scores for one student across ALL scopes (project/course/chat),
-- for D/A head-badge derivation on the student-detail page. Mirrors the lateral
-- inside ListClassRosterReport so the roster badge and the head badge can never
-- disagree.
SELECT se.scores
FROM student_evaluation se
WHERE se.user_id = @user_id
ORDER BY se.created_at DESC
LIMIT 1;
```

Update the `ListClassRosterReport` doc comment: it no longer says "latest project-scope report scores" — say "latest report scores across all scopes".

- [ ] **Step 3: Regenerate and fix the one Go call site**

Run: `cd apps/api && make sqlc`

In `apps/api/internal/api/teacher_read.go:206`, rename the call `a.d.Queries.GetLatestProjectScoresForStudent(ctx, userID)` → `a.d.Queries.GetLatestReportScoresForStudent(ctx, userID)`. Nothing else changes — the surrounding `switch` on `pgx.ErrNoRows` is still correct.

- [ ] **Step 4: Write the failing cross-surface store test**

Add to the existing D1 store suite (the file holding `TestGetLatestProjectScoresForStudent` — rename that test too):

```go
func TestGetLatestReportScoresPrefersNewestAcrossSurfaces(t *testing.T) {
	pool := newStoreTestPool(t)
	q := sqlc.New(pool)
	ctx := context.Background()

	student := createStudentRow(t, pool)               // helper already in this file
	insertProjectEvaluation(t, pool, student, `{"narrative":"older project"}`, time.Now().Add(-48*time.Hour))
	insertThreadEvaluation(t, pool, student, `{"narrative":"newest chat"}`, time.Now().Add(-1*time.Hour))

	got, err := q.GetLatestReportScoresForStudent(ctx, student)
	if err != nil {
		t.Fatalf("GetLatestReportScoresForStudent: %v", err)
	}
	if !strings.Contains(string(got), "newest chat") {
		t.Fatalf("scores = %s; want the chat evaluation — a chat-only report must not be invisible", got)
	}
}
```

Use the helpers this file already defines for creating students/projects/threads and inserting evaluations; if a thread-evaluation helper does not exist, add one alongside the existing project one, following its shape.

- [ ] **Step 5: Run to verify it fails, then passes**

Run: `cd apps/api && DOCKER_HOST=unix:///var/run/docker.sock CGO_ENABLED=0 go test -p 1 ./internal/store/...`
Expected before the migration is applied by the harness: FAIL (`relation "student_evaluation" does not exist` or unknown method). After Steps 1–3: PASS.

- [ ] **Step 6: Run the full api package**

Run: `cd apps/api && DOCKER_HOST=unix:///var/run/docker.sock CGO_ENABLED=0 go test -p 1 ./internal/api/... ./internal/store/...`
Expected: PASS. Roster/head badges may now be non-`—` for students who previously showed unrated; if a D1 test asserted `—` for a student that has a chat/course report, that assertion is genuinely obsolete — update it and say so in the report.

- [ ] **Step 7: Commit**

```bash
cd /Users/houyuxin/08Coding/mind-imprint
git add apps/api/internal/store/migrations/0032_student_evaluation_view.sql apps/api/internal/store/queries/teacher.sql apps/api/internal/store/sqlc apps/api/internal/api/teacher_read.go apps/api/internal/store/teacher_queries_test.go
git commit -m "feat(d2): student_evaluation view; latest report goes cross-surface"
```

---

### Task 4: Migration 0033 — `class_weekly_prose` + its queries

**Files:**
- Create: `apps/api/internal/store/migrations/0033_class_weekly_prose.sql`, `apps/api/internal/store/queries/teacher_weekly.sql`
- Test: `apps/api/internal/store/teacher_weekly_test.go`

**Interfaces:**
- Produces: `GetClassWeeklyProse(class_id, week_start) :one` → `sqlc.GetClassWeeklyProseRow{Comment, DepthNote, AutonomyNote string; Cards []byte; CreatedAt time.Time}`; `InsertClassWeeklyProse(...) :exec` (first-open-wins); `AppendClassWeeklyProseCards(class_id, week_start, cards) :exec` (top-up). Both writers return only `error`.
- Consumes: nothing.

- [ ] **Step 1: Write the migration**

Create `apps/api/internal/store/migrations/0033_class_weekly_prose.sql`:

```sql
-- +goose Up
-- D2: the ONLY stored part of the weekly report. Numbers are computed live on
-- every read; only the LLM-written words are persisted, once per (class, week).
-- The primary key IS the first-open-wins lock (DEC-2): generation inserts with
-- ON CONFLICT DO NOTHING, so a concurrent second teacher's call is discarded
-- rather than overwriting.
CREATE TABLE class_weekly_prose (
    class_id      uuid NOT NULL REFERENCES classes(id) ON DELETE CASCADE,
    week_start    date NOT NULL,
    comment       text NOT NULL,
    depth_note    text NOT NULL,
    autonomy_note text NOT NULL,
    cards         jsonb NOT NULL DEFAULT '[]',
    created_at    timestamptz NOT NULL DEFAULT now(),
    updated_at    timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (class_id, week_start)
);

-- +goose Down
DROP TABLE IF EXISTS class_weekly_prose;
```

- [ ] **Step 2: Write the queries**

Create `apps/api/internal/store/queries/teacher_weekly.sql`:

```sql
-- D2 weekly-report reads. Class-scoped like teacher.sql: the handler has
-- already authorised the class via assertTeacherOwnsClass, and every student
-- query JOINs enrollments with role_in_class='student'. No writes except the
-- prose row, which holds no student data beyond names already on screen.

-- name: GetClassWeeklyProse :one
SELECT comment, depth_note, autonomy_note, cards, created_at
FROM class_weekly_prose
WHERE class_id = @class_id AND week_start = @week_start;

-- name: InsertClassWeeklyProse :exec
-- First-open-wins (DEC-2): a concurrent second generation is discarded, and
-- the caller re-reads to return the winner's row.
INSERT INTO class_weekly_prose (class_id, week_start, comment, depth_note, autonomy_note, cards)
VALUES (@class_id, @week_start, @comment, @depth_note, @autonomy_note, @cards)
ON CONFLICT (class_id, week_start) DO NOTHING;

-- name: AppendClassWeeklyProseCards :exec
-- The top-up (DEC-6): appends newly-composed card prose to an existing row.
-- comment/depth_note/autonomy_note are never touched — nothing already written
-- is ever rewritten.
UPDATE class_weekly_prose
SET cards = cards || @cards::jsonb, updated_at = now()
WHERE class_id = @class_id AND week_start = @week_start;
```

- [ ] **Step 3: Regenerate sqlc**

Run: `cd apps/api && make sqlc`

- [ ] **Step 4: Write the store test**

Create `apps/api/internal/store/teacher_weekly_test.go`:

```go
package store_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"mindimprint/api/internal/store/sqlc"
)

func TestClassWeeklyProseFirstWriteWins(t *testing.T) {
	pool := newStoreTestPool(t)
	q := sqlc.New(pool)
	ctx := context.Background()
	classID := createClassRow(t, pool) // helper already used by the D1 store tests
	week := time.Date(2026, 7, 20, 0, 0, 0, 0, time.UTC)

	first := sqlc.InsertClassWeeklyProseParams{
		ClassID: classID, WeekStart: week,
		Comment: "第一次写的点评", DepthNote: "d1", AutonomyNote: "a1",
		Cards: []byte(`[{"userId":"u1","lead":"l1","action":"act1"}]`),
	}
	if err := q.InsertClassWeeklyProse(ctx, first); err != nil {
		t.Fatalf("first insert: %v", err)
	}
	second := first
	second.Comment = "第二次写的点评"
	if err := q.InsertClassWeeklyProse(ctx, second); err != nil {
		t.Fatalf("second insert must not error: %v", err)
	}

	got, err := q.GetClassWeeklyProse(ctx, sqlc.GetClassWeeklyProseParams{ClassID: classID, WeekStart: week})
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Comment != "第一次写的点评" {
		t.Fatalf("comment = %q; want the FIRST write — first-open-wins", got.Comment)
	}
}

func TestAppendClassWeeklyProseCardsOnlyAppends(t *testing.T) {
	pool := newStoreTestPool(t)
	q := sqlc.New(pool)
	ctx := context.Background()
	classID := createClassRow(t, pool)
	week := time.Date(2026, 7, 20, 0, 0, 0, 0, time.UTC)

	if err := q.InsertClassWeeklyProse(ctx, sqlc.InsertClassWeeklyProseParams{
		ClassID: classID, WeekStart: week,
		Comment: "点评", DepthNote: "d", AutonomyNote: "a",
		Cards: []byte(`[{"userId":"u1","lead":"l1","action":"act1"}]`),
	}); err != nil {
		t.Fatalf("insert: %v", err)
	}
	if err := q.AppendClassWeeklyProseCards(ctx, sqlc.AppendClassWeeklyProseCardsParams{
		ClassID: classID, WeekStart: week,
		Cards: []byte(`[{"userId":"u2","lead":"l2","action":"act2"}]`),
	}); err != nil {
		t.Fatalf("append: %v", err)
	}

	got, err := q.GetClassWeeklyProse(ctx, sqlc.GetClassWeeklyProseParams{ClassID: classID, WeekStart: week})
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Comment != "点评" {
		t.Fatalf("comment = %q; the top-up must never rewrite it", got.Comment)
	}
	s := string(got.Cards)
	if !strings.Contains(s, `"u1"`) || !strings.Contains(s, `"u2"`) {
		t.Fatalf("cards = %s; want both the original and the appended entry", s)
	}
}
```

Use whatever class-creation helper the store test package already provides; do not invent one if an equivalent exists.

- [ ] **Step 5: Run to verify**

Run: `cd apps/api && DOCKER_HOST=unix:///var/run/docker.sock CGO_ENABLED=0 go test -p 1 ./internal/store/...`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
cd /Users/houyuxin/08Coding/mind-imprint
git add apps/api/internal/store/migrations/0033_class_weekly_prose.sql apps/api/internal/store/queries/teacher_weekly.sql apps/api/internal/store/sqlc apps/api/internal/store/teacher_weekly_test.go
git commit -m "feat(d2): class_weekly_prose storage with first-open-wins + top-up"
```

---

### Task 5: Weekly aggregation queries

**Files:**
- Modify: `apps/api/internal/store/queries/teacher_weekly.sql`
- Test: `apps/api/internal/store/teacher_weekly_test.go`

**Interfaces:**
- Produces: `GetClassWeekStats(class_id, week_start, week_end) :one` → `{ClassSize, ActiveStudents, Turns, CourseSteps, Reports int32}`; `ListClassStudentWindowUsage(class_id, week_start, week_end, prev_start, prev_end) :many` → per-student `{UserID, DisplayName, AvatarColor, ActiveDays, Turns, PrevActiveDays, PrevTurns, ReportsThisWeek}`; `ListClassRecentReports(class_id) :many` → up to 2 newest rows per student `{UserID, Scores, CreatedAt, Surface, ScopeID, Rn}`.
- Consumes: the `student_evaluation` view (Task 3) and `event.course_id` (Task 2).

- [ ] **Step 1: Append the three queries**

Append to `apps/api/internal/store/queries/teacher_weekly.sql`:

```sql
-- name: GetClassWeekStats :one
-- The four stat cards for one half-open window. 对话轮次 口径 is D1's, verbatim:
-- prompt_sent (studio AND chat) + course_message. Course steps count DISTINCT
-- (student, course, ordinal) so a re-render cannot inflate the number.
WITH members AS (
  SELECT u.id
  FROM enrollments e JOIN users u ON u.id = e.user_id
  WHERE e.class_id = @class_id AND e.role_in_class = 'student'
)
SELECT
  (SELECT count(*) FROM members)::int AS class_size,
  (SELECT count(DISTINCT ev.user_id) FROM event ev JOIN members m ON m.id = ev.user_id
     WHERE ev.created_at >= @week_start AND ev.created_at < @week_end)::int AS active_students,
  (SELECT count(*) FROM event ev JOIN members m ON m.id = ev.user_id
     WHERE ev.created_at >= @week_start AND ev.created_at < @week_end
       AND ev.type IN ('prompt_sent','course_message'))::int AS turns,
  (SELECT count(DISTINCT (ev.user_id, ev.course_id, ev.payload->>'ordinal'))
     FROM event ev JOIN members m ON m.id = ev.user_id
     WHERE ev.created_at >= @week_start AND ev.created_at < @week_end
       AND ev.type = 'step_viewed')::int AS course_steps,
  (SELECT count(*) FROM student_evaluation se JOIN members m ON m.id = se.user_id
     WHERE se.created_at >= @week_start AND se.created_at < @week_end)::int AS reports;

-- name: ListClassStudentWindowUsage :many
-- Per-student usage for THIS window and the same elapsed offset LAST week, plus
-- how many reports landed this week. Bucketed via `AT TIME ZONE 'UTC'` for the
-- same reason teacher.sql is: a 7×24h window must never span 8 UTC dates.
SELECT
  u.id AS user_id, u.display_name, u.avatar_color,
  COALESCE(cur.active_days, 0)::int AS active_days,
  COALESCE(cur.turns, 0)::int       AS turns,
  COALESCE(prv.active_days, 0)::int AS prev_active_days,
  COALESCE(prv.turns, 0)::int       AS prev_turns,
  COALESCE(rep.n, 0)::int           AS reports_this_week
FROM enrollments e
JOIN users u ON u.id = e.user_id
LEFT JOIN LATERAL (
  SELECT COUNT(DISTINCT (ev.created_at AT TIME ZONE 'UTC')::date) AS active_days,
         COUNT(*) FILTER (WHERE ev.type IN ('prompt_sent','course_message')) AS turns
  FROM event ev
  WHERE ev.user_id = u.id AND ev.created_at >= @week_start AND ev.created_at < @week_end
) cur ON true
LEFT JOIN LATERAL (
  SELECT COUNT(DISTINCT (ev.created_at AT TIME ZONE 'UTC')::date) AS active_days,
         COUNT(*) FILTER (WHERE ev.type IN ('prompt_sent','course_message')) AS turns
  FROM event ev
  WHERE ev.user_id = u.id AND ev.created_at >= @prev_start AND ev.created_at < @prev_end
) prv ON true
LEFT JOIN LATERAL (
  SELECT count(*) AS n FROM student_evaluation se
  WHERE se.user_id = u.id AND se.created_at >= @week_start AND se.created_at < @week_end
) rep ON true
WHERE e.class_id = @class_id AND e.role_in_class = 'student'
ORDER BY u.display_name;

-- name: ListClassRecentReports :many
-- Each student's two newest reports across all scopes: rn=1 is 最新, rn=2 is
-- 上一次 (the baseline for 深度升档 / 更愿意自己想 / the A-axis delta). Students
-- with no report contribute no rows — 敢于空白, not a zero.
SELECT se.user_id, se.scores, se.created_at, se.surface, se.scope_id, se.rn::int AS rn
FROM enrollments e
JOIN LATERAL (
  SELECT s.user_id, s.scores, s.created_at, s.surface, s.scope_id,
         row_number() OVER (ORDER BY s.created_at DESC) AS rn
  FROM student_evaluation s
  WHERE s.user_id = e.user_id
  ORDER BY s.created_at DESC
  LIMIT 2
) se ON true
WHERE e.class_id = @class_id AND e.role_in_class = 'student'
ORDER BY se.user_id, se.rn;
```

- [ ] **Step 2: Regenerate sqlc**

Run: `cd apps/api && make sqlc`
Then read the generated row structs in `apps/api/internal/store/sqlc/teacher_weekly.sql.go` and record the exact Go types in your report — Task 6 and Task 8 depend on them, and sqlc's inference on views and laterals is not always what you'd guess.

- [ ] **Step 3: Write the failing aggregation test**

Append to `apps/api/internal/store/teacher_weekly_test.go`:

```go
func TestGetClassWeekStatsCountsOnlyTheWindow(t *testing.T) {
	pool := newStoreTestPool(t)
	q := sqlc.New(pool)
	ctx := context.Background()

	classID := createClassRow(t, pool)
	student := createStudentRow(t, pool)
	enrollStudentRow(t, pool, student, classID)

	week := time.Date(2026, 7, 20, 0, 0, 0, 0, time.UTC)
	end := week.AddDate(0, 0, 7)
	insertEventAt(t, pool, student, "prompt_sent", week.Add(2*time.Hour))          // in
	insertEventAt(t, pool, student, "course_message", week.Add(30*time.Hour))      // in
	insertEventAt(t, pool, student, "card_surfaced", week.Add(3*time.Hour))        // in, not a turn
	insertEventAt(t, pool, student, "prompt_sent", week.Add(-1*time.Hour))         // before the window

	got, err := q.GetClassWeekStats(ctx, sqlc.GetClassWeekStatsParams{
		ClassID: classID, WeekStart: week, WeekEnd: end,
	})
	if err != nil {
		t.Fatalf("GetClassWeekStats: %v", err)
	}
	if got.Turns != 2 {
		t.Fatalf("turns = %d; want 2 (prompt_sent + course_message inside the window only)", got.Turns)
	}
	if got.ActiveStudents != 1 {
		t.Fatalf("activeStudents = %d; want 1", got.ActiveStudents)
	}
	if got.ClassSize != 1 {
		t.Fatalf("classSize = %d; want 1", got.ClassSize)
	}
}

func TestListClassRecentReportsReturnsTwoNewestPerStudent(t *testing.T) {
	pool := newStoreTestPool(t)
	q := sqlc.New(pool)
	ctx := context.Background()

	classID := createClassRow(t, pool)
	student := createStudentRow(t, pool)
	enrollStudentRow(t, pool, student, classID)
	insertProjectEvaluation(t, pool, student, `{"narrative":"oldest"}`, time.Now().Add(-72*time.Hour))
	insertProjectEvaluation(t, pool, student, `{"narrative":"middle"}`, time.Now().Add(-48*time.Hour))
	insertThreadEvaluation(t, pool, student, `{"narrative":"newest"}`, time.Now().Add(-1*time.Hour))

	rows, err := q.ListClassRecentReports(ctx, classID)
	if err != nil {
		t.Fatalf("ListClassRecentReports: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("rows = %d; want exactly 2 (newest + previous)", len(rows))
	}
	if !strings.Contains(string(rows[0].Scores), "newest") || rows[0].Rn != 1 {
		t.Fatalf("row 0 = %s rn=%d; want the newest at rn=1", rows[0].Scores, rows[0].Rn)
	}
	if !strings.Contains(string(rows[1].Scores), "middle") {
		t.Fatalf("row 1 = %s; want the previous report", rows[1].Scores)
	}
}
```

Add `enrollStudentRow` / `insertEventAt` helpers to this file if the store test package lacks equivalents; keep them minimal and local.

- [ ] **Step 4: Run to verify**

Run: `cd apps/api && DOCKER_HOST=unix:///var/run/docker.sock CGO_ENABLED=0 go test -p 1 ./internal/store/...`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
cd /Users/houyuxin/08Coding/mind-imprint
git add apps/api/internal/store/queries/teacher_weekly.sql apps/api/internal/store/sqlc apps/api/internal/store/teacher_weekly_test.go
git commit -m "feat(d2): weekly aggregation queries (class stats, per-student window, recent reports)"
```

---

### Task 6: The rule layer

**Files:**
- Create: `apps/api/internal/teacher/weekly.go`, `apps/api/internal/teacher/weekly_test.go`

**Interfaces:**
- Consumes: `teacher.AMean`, `teacher.DLevels`, `teacher.DBadge`, `teacher.WeekLabel` (Task 1); `agent.Report`.
- Produces: the types and functions below. Task 7 consumes `WeeklyFacts` (defined in `internal/agent` to avoid an import cycle — `teacher` imports `agent`, never the reverse); Task 8 consumes `Detect` and `Stats`.

```go
type StudentWeek struct {
	UserID, DisplayName, AvatarColor string
	ActiveDays, Turns                int
	PrevActiveDays, PrevTurns        int
	ReportsThisWeek                  int
	Latest, Previous                 *agent.Report
	LatestSurface, LatestScopeID     string
}

type Card struct {
	UserID, DisplayName, AvatarColor string
	TagCode, TagLabel, Kind          string // Kind: "praise" | "watch"
	Evidence                         string
	HasReport                        bool
	ReportSurface, ReportScopeID     string
}

type Bucket struct{ Code, Label string; Count int }
type BucketChange struct{ Name, From, To string }
type DepthDist struct{ Buckets []Bucket; RatedCount int }
type AutonomyAgg struct{ Mean, Delta string; RatedCount int }

type Weekly struct {
	Praise, Watch []Card
	Depth         DepthDist
	Autonomy      AutonomyAgg
	BucketChanges []BucketChange
}

type ClassWeekCounts struct{ ActiveStudents, Reports, Turns, CourseSteps int }
type Stat struct {
	Key, Label string
	Value      int
	Unit, Foot string
	Delta      string // "+2" | "-3" | "±0"
	DeltaDir   string // "up" | "down" | "flat"
}

func Detect(students []StudentWeek) Weekly
func Stats(cur, prev ClassWeekCounts, classSize int) []Stat
func BuildWeeklyFacts(className string, classSize int, weekLabel string, w Weekly) agent.WeeklyFacts
```

- [ ] **Step 1: Write the failing rule tests**

Create `apps/api/internal/teacher/weekly_test.go`:

```go
package teacher_test

import (
	"strings"
	"testing"

	"mindimprint/api/internal/agent"
	"mindimprint/api/internal/teacher"
)

func rep(depth []agent.DepthDim, auto []agent.AutonomySignal) *agent.Report {
	return &agent.Report{DepthAxis: depth, AutonomyAxis: auto}
}

func autos(levels ...int) []agent.AutonomySignal {
	codes := []string{"A1", "A2", "A3", "A4", "A5", "A6"}
	out := make([]agent.AutonomySignal, 0, len(levels))
	for i, l := range levels {
		out = append(out, agent.AutonomySignal{
			Code: codes[i], Level: l, Opportunity: "given_taken",
			Evidence: "自主证据 " + codes[i],
		})
	}
	return out
}

func depths(levels ...string) []agent.DepthDim {
	codes := []string{"D1", "D2", "D3", "D4", "D5", "D6"}
	out := make([]agent.DepthDim, 0, len(levels))
	for i, l := range levels {
		out = append(out, agent.DepthDim{Code: codes[i], Level: l, Evidence: "深度证据 " + codes[i]})
	}
	return out
}

func TestNeverUsedFiresOnZeroActiveDays(t *testing.T) {
	w := teacher.Detect([]teacher.StudentWeek{{
		UserID: "u1", DisplayName: "罗一", ActiveDays: 0, PrevActiveDays: 0,
	}})
	if len(w.Watch) != 1 || w.Watch[0].TagCode != "never_used" {
		t.Fatalf("watch = %+v; want one never_used card", w.Watch)
	}
	if w.Watch[0].TagLabel != "本周未使用" {
		t.Fatalf("label = %q; want 本周未使用", w.Watch[0].TagLabel)
	}
}

func TestDroppedOffNeedsATwoDayFallAndNoNewReport(t *testing.T) {
	fires := teacher.Detect([]teacher.StudentWeek{{
		UserID: "u1", DisplayName: "陈屿", ActiveDays: 2, PrevActiveDays: 5, ReportsThisWeek: 0,
	}})
	if len(fires.Watch) != 1 || fires.Watch[0].TagCode != "dropped_off" {
		t.Fatalf("watch = %+v; want dropped_off", fires.Watch)
	}
	if !strings.Contains(fires.Watch[0].Evidence, "5 天") || !strings.Contains(fires.Watch[0].Evidence, "2 天") {
		t.Fatalf("evidence = %q; want both week counts stated", fires.Watch[0].Evidence)
	}
	quiet := teacher.Detect([]teacher.StudentWeek{{
		UserID: "u1", DisplayName: "陈屿", ActiveDays: 4, PrevActiveDays: 5, ReportsThisWeek: 0,
	}})
	if len(quiet.Watch) != 0 {
		t.Fatalf("watch = %+v; a one-day dip must not fire", quiet.Watch)
	}
	reported := teacher.Detect([]teacher.StudentWeek{{
		UserID: "u1", DisplayName: "陈屿", ActiveDays: 2, PrevActiveDays: 5, ReportsThisWeek: 1,
	}})
	if len(reported.Watch) != 0 {
		t.Fatalf("watch = %+v; a student who produced a report this week is not offline", reported.Watch)
	}
}

func TestOutsourcedJudgmentQuotesTheLowestSuppliedSignal(t *testing.T) {
	r := rep(depths("L1", "L2"), autos(0, 1, 1, 1, 1, 1)) // mean 0.833
	w := teacher.Detect([]teacher.StudentWeek{{
		UserID: "u1", DisplayName: "周子墨", ActiveDays: 3, PrevActiveDays: 3, Latest: r,
	}})
	if len(w.Watch) != 1 || w.Watch[0].TagCode != "outsourced_judgment" {
		t.Fatalf("watch = %+v; want outsourced_judgment", w.Watch)
	}
	if !strings.Contains(w.Watch[0].Evidence, "自主证据 A1") {
		t.Fatalf("evidence = %q; want the lowest supplied signal's own evidence, verbatim", w.Watch[0].Evidence)
	}
}

func TestNoBoundariesIgnoresNotSuppliedA3(t *testing.T) {
	supplied := autos(4, 4, 4, 4, 4, 4)
	supplied[2] = agent.AutonomySignal{Code: "A3", Level: 0, Opportunity: "given_not_taken", Evidence: "没有一条边界句"}
	w := teacher.Detect([]teacher.StudentWeek{{
		UserID: "u1", DisplayName: "许清", ActiveDays: 4, PrevActiveDays: 4, Latest: rep(depths("L2"), supplied),
	}})
	if len(w.Watch) != 1 || w.Watch[0].TagCode != "no_boundaries" {
		t.Fatalf("watch = %+v; want no_boundaries", w.Watch)
	}

	debt := autos(4, 4, 4, 4, 4, 4)
	debt[2] = agent.AutonomySignal{Code: "A3", Level: 0, Opportunity: "not_supplied", Evidence: ""}
	quiet := teacher.Detect([]teacher.StudentWeek{{
		UserID: "u1", DisplayName: "许清", ActiveDays: 4, PrevActiveDays: 4, Latest: rep(depths("L2"), debt),
	}})
	if len(quiet.Watch) != 0 {
		t.Fatalf("watch = %+v; a not_supplied A3 is platform debt, never a student failing", quiet.Watch)
	}
}

func TestWatchBeatsPraise(t *testing.T) {
	prev := rep(depths("L1", "L1"), autos(0, 0, 0, 0, 0, 0))
	latest := rep(depths("L1", "L1"), autos(1, 1, 1, 1, 1, 1)) // A mean 0→1: +1.0, but still ≤1.0
	w := teacher.Detect([]teacher.StudentWeek{{
		UserID: "u1", DisplayName: "周子墨", ActiveDays: 3, PrevActiveDays: 3, Latest: latest, Previous: prev,
	}})
	if len(w.Praise) != 0 {
		t.Fatalf("praise = %+v; a student still outsourcing judgment must not be filed as praise", w.Praise)
	}
	if len(w.Watch) != 1 || w.Watch[0].TagCode != "outsourced_judgment" {
		t.Fatalf("watch = %+v; want outsourced_judgment", w.Watch)
	}
}

func TestDepthUpFiresOnAHigherMaxLevel(t *testing.T) {
	w := teacher.Detect([]teacher.StudentWeek{{
		UserID: "u1", DisplayName: "吴桐", ActiveDays: 5, PrevActiveDays: 5,
		Previous: rep(depths("L2", "L2"), autos(3, 3, 3, 3, 3, 3)),
		Latest:   rep(depths("L2", "L3"), autos(3, 3, 3, 3, 3, 3)),
	}})
	if len(w.Praise) != 1 || w.Praise[0].TagCode != "depth_up" {
		t.Fatalf("praise = %+v; want depth_up", w.Praise)
	}
	if w.Praise[0].Kind != "praise" {
		t.Fatalf("kind = %q; want praise", w.Praise[0].Kind)
	}
}

func TestUnratedStudentEarnsNoJudgmentTag(t *testing.T) {
	w := teacher.Detect([]teacher.StudentWeek{{
		UserID: "u1", DisplayName: "新同学", ActiveDays: 4, PrevActiveDays: 4, Latest: nil,
	}})
	if len(w.Watch) != 0 || len(w.Praise) != 0 {
		t.Fatalf("cards = %+v/%+v; no report means no judgment", w.Watch, w.Praise)
	}
}

func TestDepthDistributionBucketsOnTheBadgeUpperBound(t *testing.T) {
	w := teacher.Detect([]teacher.StudentWeek{
		{UserID: "a", DisplayName: "林知远", ActiveDays: 6, PrevActiveDays: 6, Latest: rep(depths("L3", "L4"), autos(4, 4, 4, 4, 4, 4))},
		{UserID: "b", DisplayName: "苏晚", ActiveDays: 5, PrevActiveDays: 5, Latest: rep(depths("L3", "L3"), autos(3, 3, 3, 3, 3, 3))},
		{UserID: "c", DisplayName: "新同学", ActiveDays: 1, PrevActiveDays: 1, Latest: nil},
	})
	if w.Depth.RatedCount != 2 {
		t.Fatalf("ratedCount = %d; want 2 — an unrated student is counted nowhere", w.Depth.RatedCount)
	}
	byCode := map[string]int{}
	for _, b := range w.Depth.Buckets {
		byCode[b.Code] = b.Count
	}
	if byCode["L4"] != 1 || byCode["L3"] != 1 {
		t.Fatalf("buckets = %+v; L3–L4 buckets on its upper bound (L4)", w.Depth.Buckets)
	}
	if len(w.Depth.Buckets) != 4 {
		t.Fatalf("buckets = %d; want all four rendered, including empty ones", len(w.Depth.Buckets))
	}
}

func TestAutonomyMeanAndDelta(t *testing.T) {
	w := teacher.Detect([]teacher.StudentWeek{
		{UserID: "a", DisplayName: "甲", ActiveDays: 5, PrevActiveDays: 5,
			Previous: rep(depths("L3"), autos(2, 2, 2, 2, 2, 2)),
			Latest:   rep(depths("L3"), autos(3, 3, 3, 3, 3, 3))},
		{UserID: "b", DisplayName: "乙", ActiveDays: 5, PrevActiveDays: 5,
			Latest: rep(depths("L3"), autos(2, 2, 2, 2, 2, 2))},
	})
	if w.Autonomy.Mean != "2.5" {
		t.Fatalf("mean = %q; want 2.5 (3.0 and 2.0)", w.Autonomy.Mean)
	}
	if w.Autonomy.Delta != "+1.0" {
		t.Fatalf("delta = %q; want +1.0 (only 甲 has a baseline)", w.Autonomy.Delta)
	}
}

func TestAutonomyDeltaIsEmDashWithoutABaseline(t *testing.T) {
	w := teacher.Detect([]teacher.StudentWeek{
		{UserID: "a", DisplayName: "甲", ActiveDays: 5, PrevActiveDays: 5, Latest: rep(depths("L3"), autos(3, 3, 3, 3, 3, 3))},
	})
	if w.Autonomy.Delta != "—" {
		t.Fatalf("delta = %q; want the em-dash U+2014 when no student has a second report", w.Autonomy.Delta)
	}
}

func TestStatsRenderDeltaAndDirection(t *testing.T) {
	got := teacher.Stats(
		teacher.ClassWeekCounts{ActiveStudents: 8, Reports: 14, Turns: 386, CourseSteps: 23},
		teacher.ClassWeekCounts{ActiveStudents: 6, Reports: 14, Turns: 458, CourseSteps: 19},
		9,
	)
	if len(got) != 4 {
		t.Fatalf("stats = %d; want 4 cards", len(got))
	}
	if got[0].Value != 8 || got[0].Unit != "/ 9 人" || got[0].Delta != "+2" || got[0].DeltaDir != "up" {
		t.Fatalf("card 0 = %+v", got[0])
	}
	if got[1].Delta != "±0" || got[1].DeltaDir != "flat" {
		t.Fatalf("card 1 = %+v; an unchanged metric must be flat, never a green up-arrow", got[1])
	}
	if got[2].Delta != "-72" || got[2].DeltaDir != "down" {
		t.Fatalf("card 2 = %+v", got[2])
	}
	if got[3].Label != "完成课程节" || got[3].Unit != "节" {
		t.Fatalf("card 3 = %+v", got[3])
	}
}
```

- [ ] **Step 2: Run to verify they fail**

Run: `cd apps/api && CGO_ENABLED=0 go test ./internal/teacher/...`
Expected: FAIL — `undefined: teacher.Detect`.

- [ ] **Step 3: Implement the rule layer**

Create `apps/api/internal/teacher/weekly.go`:

```go
package teacher

import (
	"fmt"
	"sort"

	"mindimprint/api/internal/agent"
)

// StudentWeek is one student's week as the rules see it: usage facts for this
// window and the same elapsed offset last week, plus their two most recent
// canonical reports. Latest/Previous are nil when the student has no report —
// no report, no judgment (敢于空白).
type StudentWeek struct {
	UserID, DisplayName, AvatarColor string
	ActiveDays, Turns                int
	PrevActiveDays, PrevTurns        int
	ReportsThisWeek                  int
	Latest, Previous                 *agent.Report
	LatestSurface, LatestScopeID     string
}

// Card is one 值得表扬 / 需要建议 card. Evidence is ALWAYS deterministic —
// excerpted verbatim from the report or a bare statement of fact. Lead and
// action (the wording) are added later by the composer and are not fields here:
// the rule layer never produces prose.
type Card struct {
	UserID, DisplayName, AvatarColor string
	TagCode, TagLabel, Kind          string
	Evidence                         string
	HasReport                        bool
	ReportSurface, ReportScopeID     string
}

type Bucket struct {
	Code, Label string
	Count       int
}

type BucketChange struct{ Name, From, To string }

type DepthDist struct {
	Buckets    []Bucket
	RatedCount int
}

type AutonomyAgg struct {
	Mean, Delta string
	RatedCount  int
}

type Weekly struct {
	Praise, Watch []Card
	Depth         DepthDist
	Autonomy      AutonomyAgg
	BucketChanges []BucketChange
}

var bucketLabels = []Bucket{
	{Code: "L1", Label: "起步 L1"},
	{Code: "L2", Label: "发展 L2"},
	{Code: "L3", Label: "熟练 L3"},
	{Code: "L4", Label: "优秀 L4"},
}

// Detect is the whole judgment layer. Rules decide who appears, with which tag,
// and which evidence is quoted; the model that writes the wording never sees
// anything this function did not produce.
func Detect(students []StudentWeek) Weekly {
	var w Weekly
	counts := map[string]int{}

	for _, s := range students {
		if c, ok := watchCard(s); ok {
			w.Watch = append(w.Watch, c)
		} else if c, ok := praiseCard(s); ok {
			w.Praise = append(w.Praise, c)
		}
		if s.Latest == nil {
			continue
		}
		if _, max, ok := DLevels(*s.Latest); ok {
			w.Depth.RatedCount++
			counts[fmt.Sprintf("L%d", max)]++
		}
		if s.Previous != nil {
			if from, to, ok := bucketChange(*s.Previous, *s.Latest); ok {
				w.BucketChanges = append(w.BucketChanges, BucketChange{Name: s.DisplayName, From: from, To: to})
			}
		}
	}

	w.Depth.Buckets = make([]Bucket, 0, len(bucketLabels))
	for _, b := range bucketLabels {
		b.Count = counts[b.Code]
		w.Depth.Buckets = append(w.Depth.Buckets, b)
	}
	w.Autonomy = autonomyAgg(students)
	return w
}

// watchCard applies the watch rules in severity order; the first match wins and
// a student never carries more than one card.
func watchCard(s StudentWeek) (Card, bool) {
	mk := func(code, label, evidence string) (Card, bool) {
		return Card{
			UserID: s.UserID, DisplayName: s.DisplayName, AvatarColor: s.AvatarColor,
			TagCode: code, TagLabel: label, Kind: "watch", Evidence: evidence,
			HasReport: s.Latest != nil, ReportSurface: s.LatestSurface, ReportScopeID: s.LatestScopeID,
		}, true
	}
	if s.ActiveDays == 0 {
		return mk("never_used", "本周未使用",
			fmt.Sprintf("本周 0 天活动记录；上周 %d 天。", s.PrevActiveDays))
	}
	if s.ActiveDays <= s.PrevActiveDays-2 && s.ReportsThisWeek == 0 {
		return mk("dropped_off", "本周掉线",
			fmt.Sprintf("活跃天数 上周 %d 天 → 本周 %d 天，本周无新生成报告。", s.PrevActiveDays, s.ActiveDays))
	}
	if s.Latest == nil {
		return Card{}, false
	}
	if mean, ok := AMean(*s.Latest); ok && mean <= 1.0 {
		return mk("outsourced_judgment", "判断在外包",
			join(fmt.Sprintf("A 轴 %.1f/5。", mean), lowestSuppliedEvidence(*s.Latest)))
	}
	if sig, ok := signal(*s.Latest, "A3"); ok && sig.Level == 0 && sig.Opportunity != "not_supplied" {
		return mk("no_boundaries", "从不设界", sig.Evidence)
	}
	if stuckAtStart(*s.Latest) {
		return mk("stuck_at_start", "停在起步档", lowestDepthEvidence(*s.Latest))
	}
	return Card{}, false
}

func praiseCard(s StudentWeek) (Card, bool) {
	if s.Latest == nil || s.Previous == nil {
		return Card{}, false
	}
	mk := func(code, label, evidence string) (Card, bool) {
		return Card{
			UserID: s.UserID, DisplayName: s.DisplayName, AvatarColor: s.AvatarColor,
			TagCode: code, TagLabel: label, Kind: "praise", Evidence: evidence,
			HasReport: true, ReportSurface: s.LatestSurface, ReportScopeID: s.LatestScopeID,
		}, true
	}
	_, prevMax, prevOK := DLevels(*s.Previous)
	_, curMax, curOK := DLevels(*s.Latest)
	if prevOK && curOK && curMax > prevMax {
		return mk("depth_up", "深度升档",
			join(fmt.Sprintf("%s → %s。", DBadge(*s.Previous), DBadge(*s.Latest)), risenDepthEvidence(*s.Previous, *s.Latest)))
	}
	prevMean, pOK := AMean(*s.Previous)
	curMean, cOK := AMean(*s.Latest)
	if pOK && cOK && curMean-prevMean >= 0.5 {
		return mk("more_autonomous", "更愿意自己想",
			join(fmt.Sprintf("A 轴 %.1f → %.1f。", prevMean, curMean), risenSignalEvidence(*s.Previous, *s.Latest)))
	}
	return Card{}, false
}

// join appends the quoted evidence when there is one. An empty source evidence
// leaves the factual half standing alone — never a fabricated substitute.
func join(fact, quoted string) string {
	if quoted == "" {
		return fact
	}
	return fact + quoted
}

func signal(r agent.Report, code string) (agent.AutonomySignal, bool) {
	for _, a := range r.AutonomyAxis {
		if a.Code == code {
			return a, true
		}
	}
	return agent.AutonomySignal{}, false
}

func lowestSuppliedEvidence(r agent.Report) string {
	best, found := agent.AutonomySignal{}, false
	for _, a := range r.AutonomyAxis {
		if a.Opportunity == "not_supplied" {
			continue
		}
		if !found || a.Level < best.Level {
			best, found = a, true
		}
	}
	if !found {
		return ""
	}
	return best.Evidence
}

func lowestDepthEvidence(r agent.Report) string {
	best, bestRank, found := agent.DepthDim{}, 0, false
	for _, d := range r.DepthAxis {
		rank, ok := depthRank[d.Level]
		if !ok {
			continue
		}
		if !found || rank < bestRank {
			best, bestRank, found = d, rank, true
		}
	}
	if !found {
		return ""
	}
	return best.Evidence
}

// stuckAtStart: every rated dim at L2 or below, and at least half of them at L1.
func stuckAtStart(r agent.Report) bool {
	rated, ones := 0, 0
	for _, d := range r.DepthAxis {
		rank, ok := depthRank[d.Level]
		if !ok {
			continue
		}
		rated++
		if rank > 2 {
			return false
		}
		if rank == 1 {
			ones++
		}
	}
	return rated > 0 && ones*2 >= rated
}

func risenDepthEvidence(prev, cur agent.Report) string {
	prevByCode := map[string]int{}
	for _, d := range prev.DepthAxis {
		if rank, ok := depthRank[d.Level]; ok {
			prevByCode[d.Code] = rank
		}
	}
	for _, d := range cur.DepthAxis {
		rank, ok := depthRank[d.Level]
		if ok && rank > prevByCode[d.Code] {
			return d.Evidence
		}
	}
	return ""
}

func risenSignalEvidence(prev, cur agent.Report) string {
	prevByCode := map[string]int{}
	for _, a := range prev.AutonomyAxis {
		prevByCode[a.Code] = a.Level
	}
	best, bestGain, found := agent.AutonomySignal{}, 0, false
	for _, a := range cur.AutonomyAxis {
		if a.Opportunity == "not_supplied" {
			continue
		}
		if gain := a.Level - prevByCode[a.Code]; gain > bestGain {
			best, bestGain, found = a, gain, true
		}
	}
	if !found {
		return ""
	}
	return best.Evidence
}

func bucketOf(r agent.Report) (string, bool) {
	_, max, ok := DLevels(r)
	if !ok {
		return "", false
	}
	return fmt.Sprintf("L%d", max), true
}

func bucketChange(prev, cur agent.Report) (from, to string, ok bool) {
	f, fOK := bucketOf(prev)
	t, tOK := bucketOf(cur)
	if !fOK || !tOK || f == t {
		return "", "", false
	}
	return f, t, true
}

// autonomyAgg averages each rated student's own A mean, and averages the
// per-student change for students holding a second report. Derived in Go and
// ONLY in Go — the web renders these strings, it never recomputes them.
func autonomyAgg(students []StudentWeek) AutonomyAgg {
	sum, n := 0.0, 0
	deltaSum, deltaN := 0.0, 0
	for _, s := range students {
		if s.Latest == nil {
			continue
		}
		cur, ok := AMean(*s.Latest)
		if !ok {
			continue
		}
		sum += cur
		n++
		if s.Previous == nil {
			continue
		}
		if prev, pOK := AMean(*s.Previous); pOK {
			deltaSum += cur - prev
			deltaN++
		}
	}
	agg := AutonomyAgg{Mean: "—", Delta: "—", RatedCount: n} // U+2014
	if n > 0 {
		agg.Mean = fmt.Sprintf("%.1f", sum/float64(n))
	}
	if deltaN > 0 {
		agg.Delta = fmt.Sprintf("%+.1f", deltaSum/float64(deltaN))
	}
	return agg
}

// ClassWeekCounts is one window's class-wide counters.
type ClassWeekCounts struct{ ActiveStudents, Reports, Turns, CourseSteps int }

// Stat is one of the four cards at the top of the weekly report.
type Stat struct {
	Key, Label string
	Value      int
	Unit, Foot string
	Delta      string
	DeltaDir   string
}

// Stats renders the four cards in the binding design's order. An unchanged
// metric renders ±0 as flat — deliberately NOT the design prototype's green
// up-arrow, which would be a false signal on a screen whose whole value is that
// a teacher can trust the arrows.
func Stats(cur, prev ClassWeekCounts, classSize int) []Stat {
	mk := func(key, label string, value, prior int, unit, foot string) Stat {
		d := value - prior
		s := Stat{Key: key, Label: label, Value: value, Unit: unit, Foot: foot}
		switch {
		case d > 0:
			s.Delta, s.DeltaDir = fmt.Sprintf("+%d", d), "up"
		case d < 0:
			s.Delta, s.DeltaDir = fmt.Sprintf("%d", d), "down"
		default:
			s.Delta, s.DeltaDir = "±0", "flat"
		}
		return s
	}
	return []Stat{
		mk("active_students", "本周活跃学生", cur.ActiveStudents, prev.ActiveStudents, fmt.Sprintf("/ %d 人", classSize), "登录并有活动的学生"),
		mk("reports", "生成能力报告", cur.Reports, prev.Reports, "份", "来自项目、对话与课程"),
		mk("turns", "AI 对话轮次", cur.Turns, prev.Turns, "轮", "反映本周使用强度"),
		mk("course_steps", "完成课程节", cur.CourseSteps, prev.CourseSteps, "节", "平台内自学课程"),
	}
}

// BuildWeeklyFacts is the ONLY thing the composer sees. It deliberately omits
// the class-level usage counters: those tick continuously, the prose is written
// once and never refreshed (DEC-2), so prose citing them would be wrong by
// Wednesday. A card's own evidence line may contain its own numbers.
func BuildWeeklyFacts(className string, classSize int, weekLabel string, w Weekly) agent.WeeklyFacts {
	facts := agent.WeeklyFacts{
		ClassName: className, ClassSize: classSize, WeekLabel: weekLabel,
		DepthBuckets: map[string]int{}, RatedCount: w.Depth.RatedCount,
		AutonomyMean: w.Autonomy.Mean, AutonomyDelta: w.Autonomy.Delta,
	}
	for _, b := range w.Depth.Buckets {
		facts.DepthBuckets[b.Label] = b.Count
	}
	for _, c := range w.BucketChanges {
		facts.BucketChanges = append(facts.BucketChanges, agent.WeeklyBucketChange{Name: c.Name, From: c.From, To: c.To})
	}
	for _, c := range append(append([]Card{}, w.Praise...), w.Watch...) {
		facts.Cards = append(facts.Cards, agent.WeeklyFactCard{
			UserID: c.UserID, Name: c.DisplayName, Kind: c.Kind,
			TagCode: c.TagCode, TagLabel: c.TagLabel, Evidence: c.Evidence,
		})
	}
	sort.SliceStable(facts.Cards, func(i, j int) bool { return facts.Cards[i].Kind < facts.Cards[j].Kind })
	return facts
}
```

- [ ] **Step 4: Run to verify they pass**

Run: `cd apps/api && CGO_ENABLED=0 go test ./internal/teacher/...`
Expected: PASS. `agent.WeeklyFacts` does not exist yet — it is defined in Task 7. Define the four fact types (`WeeklyFacts`, `WeeklyFactCard`, `WeeklyBucketChange`, and their fields as used above) in `apps/api/internal/agent/compose_weekly.go` as part of THIS task so the package compiles; Task 7 adds the call and validation around them.

- [ ] **Step 5: Commit**

```bash
cd /Users/houyuxin/08Coding/mind-imprint
git add apps/api/internal/teacher/weekly.go apps/api/internal/teacher/weekly_test.go apps/api/internal/agent/compose_weekly.go
git commit -m "feat(d2): rule layer — tags, evidence, distribution, stats"
```

---

### Task 7: The composition call

**Files:**
- Modify: `apps/api/internal/agent/compose_weekly.go` (add the call + validation to the types Task 6 created)
- Create: `apps/api/internal/agent/compose_weekly_test.go`

**Interfaces:**
- Consumes: `agent.WeeklyFacts` (Task 6), `gateway.Provider`, `gateway.Resolved`, `gateway.Collect`.
- Produces: `agent.ComposeWeekly(ctx context.Context, prov gateway.Provider, r gateway.Resolved, f WeeklyFacts) (WeeklyProse, gateway.ChatUsage, error)`, `agent.WeeklyProse{Comment, DepthNote, AutonomyNote string; Cards []WeeklyCardProse}`, `agent.WeeklyCardProse{UserID, Lead, Action string}`.

Read `apps/api/internal/agent/assess_report.go:369-412` first — `ComposeWeekly` follows the same shape (one `gateway.Collect`, unmarshal, validate, return usage even on rejection).

- [ ] **Step 1: Write the failing tests**

Create `apps/api/internal/agent/compose_weekly_test.go`:

```go
package agent_test

import (
	"context"
	"strings"
	"testing"

	"mindimprint/api/internal/agent"
	"mindimprint/api/internal/gateway"
)

func facts() agent.WeeklyFacts {
	return agent.WeeklyFacts{
		ClassName: "IBDP 一年级 · 研究组", ClassSize: 9, WeekLabel: "第 30 周（7.20–7.26）",
		DepthBuckets: map[string]int{"起步 L1": 2, "发展 L2": 3, "熟练 L3": 2, "优秀 L4": 1},
		RatedCount:   8, AutonomyMean: "2.6", AutonomyDelta: "+0.4",
		Cards: []agent.WeeklyFactCard{
			{UserID: "u1", Name: "周子墨", Kind: "watch", TagCode: "outsourced_judgment", TagLabel: "判断在外包", Evidence: "A 轴 0.5/5。提示词多为「帮我写一段」。"},
		},
	}
}

func composeStub(reply string) gateway.Provider {
	return gateway.NewStubProvider([]gateway.StreamEvent{{Delta: reply}, {Done: true}})
}

func TestComposeWeeklyReturnsProse(t *testing.T) {
	reply := `{"comment":"这周整体在往会自己想挪。","depthNote":"熟练档多了一人。","autonomyNote":"自主均分小幅上行。","cards":[{"userId":"u1","lead":"连续让 AI 直接给结论","action":"线下问一句这些数据凭什么说明影响。"}]}`
	got, usage, err := agent.ComposeWeekly(context.Background(), composeStub(reply), gateway.Resolved{Provider: "stub", Model: "m", Tier: "flagship"}, facts())
	if err != nil {
		t.Fatalf("ComposeWeekly: %v", err)
	}
	if got.Comment == "" || len(got.Cards) != 1 || got.Cards[0].UserID != "u1" {
		t.Fatalf("prose = %+v", got)
	}
	_ = usage
}

func TestComposeWeeklyRejectsUnknownUser(t *testing.T) {
	reply := `{"comment":"c","depthNote":"d","autonomyNote":"a","cards":[{"userId":"ghost","lead":"l","action":"x"}]}`
	_, _, err := agent.ComposeWeekly(context.Background(), composeStub(reply), gateway.Resolved{Provider: "stub"}, facts())
	if err == nil {
		t.Fatal("want rejection — the model must not invent a student")
	}
}

func TestComposeWeeklyRejectsMissingCard(t *testing.T) {
	reply := `{"comment":"c","depthNote":"d","autonomyNote":"a","cards":[]}`
	_, _, err := agent.ComposeWeekly(context.Background(), composeStub(reply), gateway.Resolved{Provider: "stub"}, facts())
	if err == nil {
		t.Fatal("want rejection — every fact-sheet card needs wording")
	}
}

func TestComposeWeeklyRejectsBareInternalCode(t *testing.T) {
	reply := `{"comment":"这个班的 D3 普遍偏弱。","depthNote":"d","autonomyNote":"a","cards":[{"userId":"u1","lead":"l","action":"x"}]}`
	_, _, err := agent.ComposeWeekly(context.Background(), composeStub(reply), gateway.Resolved{Provider: "stub"}, facts())
	if err == nil {
		t.Fatal("want rejection — 说人话: prose names the behaviour, not the code")
	}
}

func TestComposeWeeklyReturnsUsageOnRejection(t *testing.T) {
	reply := `{"comment":"c","depthNote":"d","autonomyNote":"a","cards":[]}`
	_, usage, err := agent.ComposeWeekly(context.Background(), composeStub(reply), gateway.Resolved{Provider: "stub"}, facts())
	if err == nil {
		t.Fatal("want rejection")
	}
	_ = usage // usage must be returned so the caller can still meter the spend
}

func TestWeeklyFactsPromptCarriesEvidenceVerbatim(t *testing.T) {
	p := agent.WeeklyFactsPrompt(facts())
	if !strings.Contains(p, "A 轴 0.5/5。提示词多为「帮我写一段」。") {
		t.Fatalf("prompt omits the card's verbatim evidence:\n%s", p)
	}
	if strings.Contains(p, "对话轮次") {
		t.Fatalf("prompt must not carry class-level usage counters:\n%s", p)
	}
}
```

Check `gateway.NewStubProvider`'s real event shape in `apps/api/internal/gateway/stub.go` and match it — `assessStubProvider` in `internal/api/assessment_test.go` shows the working call.

- [ ] **Step 2: Run to verify they fail**

Run: `cd apps/api && CGO_ENABLED=0 go test ./internal/agent/...`
Expected: FAIL — `undefined: agent.ComposeWeekly`.

- [ ] **Step 3: Implement the composer**

In `apps/api/internal/agent/compose_weekly.go`, keep the fact types from Task 6 and add:

```go
// WeeklyProse is the ONLY thing the model contributes to the weekly report:
// wording. It cannot add, drop, reorder, or reclassify a card — the rule layer
// already decided all of that.
type WeeklyProse struct {
	Comment      string            `json:"comment"`
	DepthNote    string            `json:"depthNote"`
	AutonomyNote string            `json:"autonomyNote"`
	Cards        []WeeklyCardProse `json:"cards"`
}

type WeeklyCardProse struct {
	UserID string `json:"userId"`
	Lead   string `json:"lead"`
	Action string `json:"action"`
}

const (
	weeklyCommentMax = 300
	weeklyNoteMax    = 200
	weeklyLeadMax    = 120
	weeklyActionMax  = 200
)

// bareCode matches an internal axis code standing alone (D3, A5). Teachers DO
// see the codes — labelled — on the deep report screen; this screen's prose
// names the behaviour instead (教师端铁律② 说人话).
var bareCode = regexp.MustCompile(`\b[DA][1-6]\b`)

func weeklySystemPrompt() string {
	return strings.Join([]string{
		"你在为一位中学教师写一份「班级周报」的措辞。",
		"你只负责措辞。谁上榜、贴哪个标签、引哪句证据，已经由系统判定完毕，你不得增删、调换或重新归类。",
		"规则：",
		"1. 只使用给你的事实。不得引入任何未给出的学生、数字或行为。",
		"2. 说人话。不要出现 D1–D6 / A1–A6 这类内部代码。",
		"3. 每张卡写两句：lead 用一句话说清发生了什么；action 写教师线下可以怎么开口（需要建议）或怎么鼓励（值得表扬）。",
		"4. 具体沟通在线下进行，不要建议教师在平台上给学生发消息或打分。",
		"5. 只输出 JSON：{\"comment\":\"\",\"depthNote\":\"\",\"autonomyNote\":\"\",\"cards\":[{\"userId\":\"\",\"lead\":\"\",\"action\":\"\"}]}",
	}, "\n")
}

// WeeklyFactsPrompt renders the fact sheet the model sees. Exported so a test
// can assert what it does and does not carry.
func WeeklyFactsPrompt(f WeeklyFacts) string {
	var b strings.Builder
	fmt.Fprintf(&b, "班级：%s（%d 名学生）\n周次：%s\n", f.ClassName, f.ClassSize, f.WeekLabel)
	fmt.Fprintf(&b, "已评估学生：%d 名\n", f.RatedCount)
	b.WriteString("认知深度分布：")
	for _, label := range []string{"起步 L1", "发展 L2", "熟练 L3", "优秀 L4"} {
		fmt.Fprintf(&b, "%s %d 人；", label, f.DepthBuckets[label])
	}
	b.WriteString("\n")
	for _, c := range f.BucketChanges {
		fmt.Fprintf(&b, "档位变化：%s 由 %s 升到 %s\n", c.Name, c.From, c.To)
	}
	fmt.Fprintf(&b, "智识自主均分：%s / 5，较上一次报告 %s\n", f.AutonomyMean, f.AutonomyDelta)
	b.WriteString("需要写措辞的卡片：\n")
	for _, c := range f.Cards {
		kind := "需要建议"
		if c.Kind == "praise" {
			kind = "值得表扬"
		}
		fmt.Fprintf(&b, "- userId=%s 姓名=%s 类别=%s 标签=%s 证据=%s\n", c.UserID, c.Name, kind, c.TagLabel, c.Evidence)
	}
	return b.String()
}

// ComposeWeekly makes ONE flagship call turning the fact sheet into wording.
// Usage is returned even when the output is rejected, so the caller records the
// spend either way.
func ComposeWeekly(ctx context.Context, prov gateway.Provider, r gateway.Resolved, f WeeklyFacts) (WeeklyProse, gateway.ChatUsage, error) {
	res, err := gateway.Collect(ctx, prov, r, gateway.ChatRequest{
		Messages: []gateway.ChatMessage{
			{Role: gateway.RoleSystem, Content: weeklySystemPrompt()},
			{Role: gateway.RoleUser, Content: WeeklyFactsPrompt(f)},
		},
	})
	if err != nil {
		return WeeklyProse{}, gateway.ChatUsage{}, err
	}
	usage := res.Usage

	var out WeeklyProse
	if err := json.Unmarshal([]byte(strings.TrimSpace(res.Text)), &out); err != nil {
		return WeeklyProse{}, usage, fmt.Errorf("agent: weekly prose not JSON: %w", err)
	}
	if err := validateWeeklyProse(out, f); err != nil {
		return WeeklyProse{}, usage, err
	}
	return out, usage, nil
}

func validateWeeklyProse(p WeeklyProse, f WeeklyFacts) error {
	want := map[string]bool{}
	for _, c := range f.Cards {
		want[c.UserID] = false
	}
	for _, c := range p.Cards {
		seen, known := want[c.UserID]
		if !known {
			return fmt.Errorf("agent: weekly prose names an unknown student %q", c.UserID)
		}
		if seen {
			return fmt.Errorf("agent: weekly prose repeats student %q", c.UserID)
		}
		want[c.UserID] = true
		if len(c.Lead) > weeklyLeadMax*3 || len(c.Action) > weeklyActionMax*3 {
			return fmt.Errorf("agent: weekly card prose too long for %q", c.UserID)
		}
	}
	for id, seen := range want {
		if !seen {
			return fmt.Errorf("agent: weekly prose is missing card %q", id)
		}
	}
	if len(p.Comment) > weeklyCommentMax*3 || len(p.DepthNote) > weeklyNoteMax*3 || len(p.AutonomyNote) > weeklyNoteMax*3 {
		return fmt.Errorf("agent: weekly prose exceeds its length cap")
	}
	texts := []string{p.Comment, p.DepthNote, p.AutonomyNote}
	for _, c := range p.Cards {
		texts = append(texts, c.Lead, c.Action)
	}
	for _, t := range texts {
		if bareCode.MatchString(t) {
			return fmt.Errorf("agent: weekly prose contains a bare internal code")
		}
	}
	return nil
}
```

Length caps are byte-based (`*3` approximates CJK runes); if the package already has a rune-count helper, use it instead. Add the needed imports (`context`, `encoding/json`, `fmt`, `regexp`, `strings`, `mindimprint/api/internal/gateway`).

- [ ] **Step 4: Run to verify they pass**

Run: `cd apps/api && CGO_ENABLED=0 go test ./internal/agent/... ./internal/teacher/...`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
cd /Users/houyuxin/08Coding/mind-imprint
git add apps/api/internal/agent/compose_weekly.go apps/api/internal/agent/compose_weekly_test.go
git commit -m "feat(d2): flagship weekly composer — wording only, validated"
```

---

### Task 8: `GET /weekly-report`

**Files:**
- Create: `apps/api/internal/api/teacher_weekly.go`, `apps/api/internal/api/teacher_weekly_test.go`
- Modify: `apps/api/internal/api/api.go:129-131` (add one route)

**Interfaces:**
- Consumes: `teacher.WeekWindow`/`PrevWindow`/`WeekLabel`/`Detect`/`Stats` (Tasks 1, 6); the Task 5 queries; `GetClassWeeklyProse` (Task 4).
- Produces: `a.getClassWeeklyReport`, the DTOs below, and `a.loadWeekly(ctx, classID) (weeklyData, error)` — the shared loader Task 9 reuses.

- [ ] **Step 1: Write the failing tenancy + shape tests**

Create `apps/api/internal/api/teacher_weekly_test.go`:

```go
package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestWeeklyReportRejectsForeignTeacher(t *testing.T) {
	pool := newAPITestPool(t)
	a := New(DepsForTest(t, pool))
	owner := signInAs(t, pool, createTeacher(t, pool, SeedSchoolID, "wk-owner@demo.local"))
	classID := createClassViaAPI(t, a, owner, "周报班")

	intruder := signInAs(t, pool, createTeacher(t, pool, SeedSchoolID, "wk-intruder@demo.local"))
	req := withCookie(httptest.NewRequest(http.MethodGet, "/api/v1/classes/"+classID+"/weekly-report", nil), intruder)
	rec := httptest.NewRecorder()
	a.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d; want 404 — a foreign class must be existence-hidden", rec.Code)
	}
}

func TestWeeklyReportRejectsStudent(t *testing.T) {
	pool := newAPITestPool(t)
	a := New(DepsForTest(t, pool))
	owner := signInAs(t, pool, createTeacher(t, pool, SeedSchoolID, "wk-owner2@demo.local"))
	classID := createClassViaAPI(t, a, owner, "周报班")

	student := signInSeed(t, pool)
	req := withCookie(httptest.NewRequest(http.MethodGet, "/api/v1/classes/"+classID+"/weekly-report", nil), student)
	rec := httptest.NewRecorder()
	a.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d; want 403", rec.Code)
	}
}

func TestWeeklyReportShapeWithoutProse(t *testing.T) {
	pool := newAPITestPool(t)
	a := New(DepsForTest(t, pool))
	owner := signInAs(t, pool, createTeacher(t, pool, SeedSchoolID, "wk-shape@demo.local"))
	classID := createClassViaAPI(t, a, owner, "周报班")
	studentID := createStudent(t, pool, SeedSchoolID, "wk-student@demo.local")
	enrollStudent(t, pool, studentID, classID)

	req := withCookie(httptest.NewRequest(http.MethodGet, "/api/v1/classes/"+classID+"/weekly-report", nil), owner)
	rec := httptest.NewRecorder()
	a.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body %s", rec.Code, rec.Body.String())
	}
	var got WeeklyReportDTO
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(got.Stats) != 4 {
		t.Fatalf("stats = %d; want 4", len(got.Stats))
	}
	if got.ProseReady {
		t.Fatal("proseReady = true; nothing has been generated yet")
	}
	if got.Comment != nil {
		t.Fatalf("comment = %v; want null before generation", *got.Comment)
	}
	if got.ClassSize != 1 {
		t.Fatalf("classSize = %d; want 1", got.ClassSize)
	}
	if got.WeekLabel == "" {
		t.Fatal("weekLabel must always be present")
	}
}
```

Use the harness helpers exactly as the D1 suite in `teacher_read_test.go` does — read that file for `createClassViaAPI`, `enrollStudent`, `signInSeed`, `withCookie` signatures rather than guessing.

- [ ] **Step 2: Run to verify they fail**

Run: `cd apps/api && DOCKER_HOST=unix:///var/run/docker.sock CGO_ENABLED=0 go test -p 1 ./internal/api/...`
Expected: FAIL — 404 route not found / `undefined: WeeklyReportDTO`.

- [ ] **Step 3: Implement the DTOs, the loader, and the GET handler**

Create `apps/api/internal/api/teacher_weekly.go`:

```go
package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"mindimprint/api/internal/agent"
	"mindimprint/api/internal/httpx"
	"mindimprint/api/internal/store/sqlc"
	"mindimprint/api/internal/teacher"
)

// WeeklyCardDTO is one 值得表扬 / 需要建议 card. Evidence is always present —
// it is deterministic. Lead and action are "" until the prose is composed; a
// card with no wording still renders its tag and its evidence (敢于空白).
type WeeklyCardDTO struct {
	UserID        string `json:"userId"`
	DisplayName   string `json:"displayName"`
	AvatarColor   string `json:"avatarColor"`
	TagCode       string `json:"tagCode"`
	TagLabel      string `json:"tagLabel"`
	Kind          string `json:"kind"`
	Evidence      string `json:"evidence"`
	Lead          string `json:"lead"`
	Action        string `json:"action"`
	HasReport     bool   `json:"hasReport"`
	ReportSurface string `json:"reportSurface,omitempty"`
	ReportScopeID string `json:"reportScopeId,omitempty"`
}

type WeeklyStatDTO struct {
	Key      string `json:"key"`
	Label    string `json:"label"`
	Value    int    `json:"value"`
	Unit     string `json:"unit"`
	Foot     string `json:"foot"`
	Delta    string `json:"delta"`
	DeltaDir string `json:"deltaDir"`
}

type WeeklyBucketDTO struct {
	Code  string `json:"code"`
	Label string `json:"label"`
	Count int    `json:"count"`
}

type WeeklyDepthDTO struct {
	Buckets    []WeeklyBucketDTO `json:"buckets"`
	RatedCount int               `json:"ratedCount"`
	Note       string            `json:"note"`
}

type WeeklyAutonomyDTO struct {
	Mean       string `json:"mean"`
	Delta      string `json:"delta"`
	RatedCount int    `json:"ratedCount"`
	Note       string `json:"note"`
}

// WeeklyReportDTO is the whole screen. Every number in it was computed on this
// read; only comment/notes/lead/action come from storage.
type WeeklyReportDTO struct {
	WeekLabel  string            `json:"weekLabel"`
	WeekStart  string            `json:"weekStart"`
	WeekEnd    string            `json:"weekEnd"`
	AsOf       string            `json:"asOf"`
	ClassName  string            `json:"className"`
	ClassSize  int               `json:"classSize"`
	Stats      []WeeklyStatDTO   `json:"stats"`
	Praise     []WeeklyCardDTO   `json:"praise"`
	Watch      []WeeklyCardDTO   `json:"watch"`
	Depth      WeeklyDepthDTO    `json:"depth"`
	Autonomy   WeeklyAutonomyDTO `json:"autonomy"`
	Comment    *string           `json:"comment"`
	ProseReady bool              `json:"proseReady"`
}

// weeklyData is everything both handlers need: the live computation plus the
// class identity. The GET renders it; the POST composes prose over it.
type weeklyData struct {
	Class     sqlc.Class
	WeekStart time.Time
	Now       time.Time
	Weekly    teacher.Weekly
	Stats     []teacher.Stat
	ClassSize int
}

// loadWeekly runs the whole deterministic layer for one class. No model call.
func (a *API) loadWeekly(ctx context.Context, cls sqlc.Class, now time.Time) (weeklyData, error) {
	start, end := teacher.WeekWindow(now)
	prevStart, prevEnd := teacher.PrevWindow(now)

	cur, err := a.d.Queries.GetClassWeekStats(ctx, sqlc.GetClassWeekStatsParams{
		ClassID: cls.ID, WeekStart: start, WeekEnd: end,
	})
	if err != nil {
		return weeklyData{}, err
	}
	prev, err := a.d.Queries.GetClassWeekStats(ctx, sqlc.GetClassWeekStatsParams{
		ClassID: cls.ID, WeekStart: prevStart, WeekEnd: prevEnd,
	})
	if err != nil {
		return weeklyData{}, err
	}
	usage, err := a.d.Queries.ListClassStudentWindowUsage(ctx, sqlc.ListClassStudentWindowUsageParams{
		ClassID: cls.ID, WeekStart: start, WeekEnd: end, PrevStart: prevStart, PrevEnd: prevEnd,
	})
	if err != nil {
		return weeklyData{}, err
	}
	recent, err := a.d.Queries.ListClassRecentReports(ctx, cls.ID)
	if err != nil {
		return weeklyData{}, err
	}

	type pair struct {
		latest, previous *agent.Report
		surface, scopeID string
	}
	byUser := map[string]*pair{}
	for _, row := range recent {
		var rep agent.Report
		if json.Unmarshal(row.Scores, &rep) != nil {
			continue // a corrupt payload is no evidence — skip, never guess
		}
		key := uuidText(row.UserID)
		p := byUser[key]
		if p == nil {
			p = &pair{}
			byUser[key] = p
		}
		if row.Rn == 1 {
			r := rep
			p.latest, p.surface, p.scopeID = &r, row.Surface, uuidText(row.ScopeID)
		} else {
			r := rep
			p.previous = &r
		}
	}

	students := make([]teacher.StudentWeek, 0, len(usage))
	for _, u := range usage {
		s := teacher.StudentWeek{
			UserID: u.UserID.String(), DisplayName: u.DisplayName, AvatarColor: u.AvatarColor,
			ActiveDays: int(u.ActiveDays), Turns: int(u.Turns),
			PrevActiveDays: int(u.PrevActiveDays), PrevTurns: int(u.PrevTurns),
			ReportsThisWeek: int(u.ReportsThisWeek),
		}
		if p := byUser[u.UserID.String()]; p != nil {
			s.Latest, s.Previous = p.latest, p.previous
			s.LatestSurface, s.LatestScopeID = p.surface, p.scopeID
		}
		students = append(students, s)
	}

	return weeklyData{
		Class: cls, WeekStart: start, Now: now,
		Weekly: teacher.Detect(students),
		Stats: teacher.Stats(
			teacher.ClassWeekCounts{ActiveStudents: int(cur.ActiveStudents), Reports: int(cur.Reports), Turns: int(cur.Turns), CourseSteps: int(cur.CourseSteps)},
			teacher.ClassWeekCounts{ActiveStudents: int(prev.ActiveStudents), Reports: int(prev.Reports), Turns: int(prev.Turns), CourseSteps: int(prev.CourseSteps)},
			int(cur.ClassSize),
		),
		ClassSize: int(cur.ClassSize),
	}, nil
}

// weeklyDTO renders the loaded data, merging in whatever prose exists.
func weeklyDTO(d weeklyData, prose *sqlc.GetClassWeeklyProseRow) WeeklyReportDTO {
	start, end := teacher.WeekWindow(d.Now)
	dto := WeeklyReportDTO{
		WeekLabel: teacher.WeekLabel(start),
		WeekStart: start.Format(time.RFC3339), WeekEnd: end.Format(time.RFC3339),
		AsOf:      d.Now.UTC().Format(time.RFC3339),
		ClassName: d.Class.Name, ClassSize: d.ClassSize,
	}
	for _, s := range d.Stats {
		dto.Stats = append(dto.Stats, WeeklyStatDTO{
			Key: s.Key, Label: s.Label, Value: s.Value, Unit: s.Unit, Foot: s.Foot,
			Delta: s.Delta, DeltaDir: s.DeltaDir,
		})
	}
	for _, b := range d.Weekly.Depth.Buckets {
		dto.Depth.Buckets = append(dto.Depth.Buckets, WeeklyBucketDTO{Code: b.Code, Label: b.Label, Count: b.Count})
	}
	dto.Depth.RatedCount = d.Weekly.Depth.RatedCount
	dto.Autonomy = WeeklyAutonomyDTO{
		Mean: d.Weekly.Autonomy.Mean, Delta: d.Weekly.Autonomy.Delta, RatedCount: d.Weekly.Autonomy.RatedCount,
	}

	wording := map[string]agent.WeeklyCardProse{}
	if prose != nil {
		var cards []agent.WeeklyCardProse
		if json.Unmarshal(prose.Cards, &cards) == nil {
			for _, c := range cards {
				wording[c.UserID] = c
			}
		}
		c := prose.Comment
		dto.Comment = &c
		dto.Depth.Note = prose.DepthNote
		dto.Autonomy.Note = prose.AutonomyNote
		dto.ProseReady = true
	}
	conv := func(cards []teacher.Card) []WeeklyCardDTO {
		out := make([]WeeklyCardDTO, 0, len(cards))
		for _, c := range cards {
			w := wording[c.UserID]
			out = append(out, WeeklyCardDTO{
				UserID: c.UserID, DisplayName: c.DisplayName, AvatarColor: c.AvatarColor,
				TagCode: c.TagCode, TagLabel: c.TagLabel, Kind: c.Kind, Evidence: c.Evidence,
				Lead: w.Lead, Action: w.Action,
				HasReport: c.HasReport, ReportSurface: c.ReportSurface, ReportScopeID: c.ReportScopeID,
			})
		}
		return out
	}
	dto.Praise, dto.Watch = conv(d.Weekly.Praise), conv(d.Weekly.Watch)
	return dto
}

// getClassWeeklyReport handles GET /api/v1/classes/{id}/weekly-report. Every
// number is computed on this read; the stored prose is merged in when it
// exists. This handler NEVER calls a model.
func (a *API) getClassWeeklyReport(w http.ResponseWriter, r *http.Request) {
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
	now := time.Now()
	data, err := a.loadWeekly(r.Context(), cls, now)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	prose, perr := a.d.Queries.GetClassWeeklyProse(r.Context(), sqlc.GetClassWeeklyProseParams{
		ClassID: id, WeekStart: data.WeekStart,
	})
	switch {
	case perr == nil:
		httpx.WriteJSON(w, http.StatusOK, weeklyDTO(data, &prose))
	case errors.Is(perr, pgx.ErrNoRows):
		httpx.WriteJSON(w, http.StatusOK, weeklyDTO(data, nil))
	default:
		httpx.WriteError(w, r, perr)
	}
}
```

If `uuidText` is not already available in the `api` package for the row's UUID types, use whatever the D1 code uses at `teacher_read.go:237`. Adjust field types to whatever `make sqlc` actually generated (Task 5 Step 2 recorded them).

- [ ] **Step 4: Register the route**

In `apps/api/internal/api/api.go`, after line 131, add:

```go
	mux.Handle("GET /api/v1/classes/{id}/weekly-report", teacherOrAdmin(a.getClassWeeklyReport))
```

- [ ] **Step 5: Run the full api package**

Run: `cd apps/api && DOCKER_HOST=unix:///var/run/docker.sock CGO_ENABLED=0 go test -p 1 ./internal/api/...`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
cd /Users/houyuxin/08Coding/mind-imprint
git add apps/api/internal/api/teacher_weekly.go apps/api/internal/api/teacher_weekly_test.go apps/api/internal/api/api.go
git commit -m "feat(d2): GET weekly-report — live numbers, no model call"
```

---

### Task 9: `POST /weekly-report/prose`

**Files:**
- Modify: `apps/api/internal/api/teacher_weekly.go`, `apps/api/internal/api/teacher_weekly_test.go`, `apps/api/internal/api/api.go`

**Interfaces:**
- Consumes: `a.loadWeekly`, `weeklyDTO` (Task 8); `agent.ComposeWeekly`, `teacher.BuildWeeklyFacts`; `InsertClassWeeklyProse`, `AppendClassWeeklyProseCards` (Task 4).
- Produces: `a.postClassWeeklyProse`.

- [ ] **Step 1: Write the failing tests**

Append to `apps/api/internal/api/teacher_weekly_test.go`:

```go
const weeklyProseReply = `{"comment":"这周整体在往会自己想挪。","depthNote":"熟练档多了一人。","autonomyNote":"自主均分小幅上行。","cards":[]}`

func TestWeeklyProseGeneratesOnceAndIsReadOnlyAfterwards(t *testing.T) {
	pool := newAPITestPool(t)
	deps := DepsForTest(t, pool)
	deps.Provider = assessStubProvider(weeklyProseReply)
	a := New(deps)
	owner := signInAs(t, pool, createTeacher(t, pool, SeedSchoolID, "wk-prose@demo.local"))
	classID := createClassViaAPI(t, a, owner, "周报班")
	// A student with zero activity fires never_used, so the fact sheet is
	// non-empty and the composer actually runs. Without this the handler's
	// "nothing to say" branch short-circuits and the test asserts nothing.
	enrollStudent(t, pool, createStudent(t, pool, SeedSchoolID, "wk-prose-s@demo.local"), classID)

	post := func() WeeklyReportDTO {
		req := withCookie(httptest.NewRequest(http.MethodPost, "/api/v1/classes/"+classID+"/weekly-report/prose", nil), owner)
		rec := httptest.NewRecorder()
		a.Handler().ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, body %s", rec.Code, rec.Body.String())
		}
		var dto WeeklyReportDTO
		if err := json.Unmarshal(rec.Body.Bytes(), &dto); err != nil {
			t.Fatalf("decode: %v", err)
		}
		return dto
	}

	first := post()
	if !first.ProseReady || first.Comment == nil {
		t.Fatalf("first POST did not produce prose: %+v", first)
	}
	second := post()
	if second.Comment == nil || *second.Comment != *first.Comment {
		t.Fatal("second POST must return the first comment — first-open-wins")
	}

	var calls int
	if err := pool.QueryRow(context.Background(),
		`SELECT count(*) FROM llm_call WHERE purpose = 'class_weekly'`).Scan(&calls); err != nil {
		t.Fatalf("count llm_call: %v", err)
	}
	if calls != 1 {
		t.Fatalf("llm_call rows = %d; want exactly 1 — the second POST must not spend", calls)
	}
}

func TestWeeklyProseFailureStillReturnsTheScreen(t *testing.T) {
	pool := newAPITestPool(t)
	deps := DepsForTest(t, pool)
	deps.Provider = assessStubProvider(`not json at all`)
	a := New(deps)
	owner := signInAs(t, pool, createTeacher(t, pool, SeedSchoolID, "wk-fail@demo.local"))
	classID := createClassViaAPI(t, a, owner, "周报班")
	enrollStudent(t, pool, createStudent(t, pool, SeedSchoolID, "wk-fail-s@demo.local"), classID)

	req := withCookie(httptest.NewRequest(http.MethodPost, "/api/v1/classes/"+classID+"/weekly-report/prose", nil), owner)
	rec := httptest.NewRecorder()
	a.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d; a failed composition must never wall the screen", rec.Code)
	}
	var dto WeeklyReportDTO
	if err := json.Unmarshal(rec.Body.Bytes(), &dto); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if dto.ProseReady || dto.Comment != nil {
		t.Fatalf("prose must be absent after a rejected composition: %+v", dto)
	}
	if len(dto.Stats) != 4 {
		t.Fatalf("stats = %d; the numbers must still render", len(dto.Stats))
	}
	var calls int
	if err := pool.QueryRow(context.Background(),
		`SELECT count(*) FROM llm_call WHERE purpose = 'class_weekly'`).Scan(&calls); err != nil {
		t.Fatalf("count llm_call: %v", err)
	}
	if calls != 1 {
		t.Fatalf("llm_call rows = %d; want 1 — cost is recorded even on rejection", calls)
	}
}

func TestWeeklyProseMakesNoCallForAnEmptyClass(t *testing.T) {
	pool := newAPITestPool(t)
	deps := DepsForTest(t, pool)
	deps.Provider = assessStubProvider(weeklyProseReply)
	a := New(deps)
	owner := signInAs(t, pool, createTeacher(t, pool, SeedSchoolID, "wk-empty@demo.local"))
	classID := createClassViaAPI(t, a, owner, "周报班") // no students enrolled

	req := withCookie(httptest.NewRequest(http.MethodPost, "/api/v1/classes/"+classID+"/weekly-report/prose", nil), owner)
	rec := httptest.NewRecorder()
	a.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	var calls int
	if err := pool.QueryRow(context.Background(),
		`SELECT count(*) FROM llm_call WHERE purpose = 'class_weekly'`).Scan(&calls); err != nil {
		t.Fatalf("count llm_call: %v", err)
	}
	if calls != 0 {
		t.Fatalf("llm_call rows = %d; an empty class has nothing to say about", calls)
	}
}

func TestWeeklyProseRejectsStudent(t *testing.T) {
	pool := newAPITestPool(t)
	a := New(DepsForTest(t, pool))
	owner := signInAs(t, pool, createTeacher(t, pool, SeedSchoolID, "wk-guard@demo.local"))
	classID := createClassViaAPI(t, a, owner, "周报班")
	student := signInSeed(t, pool)
	req := withCookie(httptest.NewRequest(http.MethodPost, "/api/v1/classes/"+classID+"/weekly-report/prose", nil), student)
	rec := httptest.NewRecorder()
	a.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d; want 403", rec.Code)
	}
}
```

`assessStubProvider` lives in `internal/api/assessment_test.go` in this same package — reuse it, do not duplicate it.

- [ ] **Step 2: Run to verify they fail**

Run: `cd apps/api && DOCKER_HOST=unix:///var/run/docker.sock CGO_ENABLED=0 go test -p 1 ./internal/api/...`
Expected: FAIL — route not found (404 instead of 200/403).

- [ ] **Step 3: Implement the POST handler**

Append to `apps/api/internal/api/teacher_weekly.go`:

```go
// postClassWeeklyProse handles POST /api/v1/classes/{id}/weekly-report/prose —
// the ONLY endpoint in D2 that spends. It generates once per (class, week);
// a second call returns the stored row without calling a model (DEC-2), and a
// card that first appeared after generation is topped up (DEC-6) without
// rewriting anything already written.
func (a *API) postClassWeeklyProse(w http.ResponseWriter, r *http.Request) {
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
	ctx := r.Context()
	now := time.Now()
	data, err := a.loadWeekly(ctx, cls, now)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}

	existing, gerr := a.d.Queries.GetClassWeeklyProse(ctx, sqlc.GetClassWeeklyProseParams{
		ClassID: id, WeekStart: data.WeekStart,
	})
	hasRow := gerr == nil
	if gerr != nil && !errors.Is(gerr, pgx.ErrNoRows) {
		httpx.WriteError(w, r, gerr)
		return
	}

	facts := teacher.BuildWeeklyFacts(cls.Name, data.ClassSize, teacher.WeekLabel(data.WeekStart), data.Weekly)

	// Nothing to say about: no cards and nobody rated. Spending a flagship call
	// to be told so is waste.
	if !hasRow && len(facts.Cards) == 0 && data.Weekly.Depth.RatedCount == 0 {
		httpx.WriteJSON(w, http.StatusOK, weeklyDTO(data, nil))
		return
	}

	if hasRow {
		missing := missingCardFacts(existing.Cards, facts)
		if len(missing) == 0 {
			httpx.WriteJSON(w, http.StatusOK, weeklyDTO(data, &existing))
			return
		}
		topUp := facts
		topUp.Cards = missing
		prose, ok := a.composeWeeklyProse(ctx, r, topUp)
		if ok && len(prose.Cards) > 0 {
			if cards, merr := json.Marshal(prose.Cards); merr == nil {
				if aerr := a.d.Queries.AppendClassWeeklyProseCards(ctx, sqlc.AppendClassWeeklyProseCardsParams{
					ClassID: id, WeekStart: data.WeekStart, Cards: cards,
				}); aerr != nil {
					slog.Warn("weekly prose: append cards", "err", aerr)
				}
			}
		}
		refreshed, rerr := a.d.Queries.GetClassWeeklyProse(ctx, sqlc.GetClassWeeklyProseParams{
			ClassID: id, WeekStart: data.WeekStart,
		})
		if rerr != nil {
			httpx.WriteJSON(w, http.StatusOK, weeklyDTO(data, &existing))
			return
		}
		httpx.WriteJSON(w, http.StatusOK, weeklyDTO(data, &refreshed))
		return
	}

	prose, ok := a.composeWeeklyProse(ctx, r, facts)
	if !ok {
		// 敢于空白: the numbers, the tags and the evidence still render.
		httpx.WriteJSON(w, http.StatusOK, weeklyDTO(data, nil))
		return
	}
	cards, merr := json.Marshal(prose.Cards)
	if merr != nil {
		httpx.WriteJSON(w, http.StatusOK, weeklyDTO(data, nil))
		return
	}
	if ierr := a.d.Queries.InsertClassWeeklyProse(ctx, sqlc.InsertClassWeeklyProseParams{
		ClassID: id, WeekStart: data.WeekStart,
		Comment: prose.Comment, DepthNote: prose.DepthNote, AutonomyNote: prose.AutonomyNote,
		Cards: cards,
	}); ierr != nil {
		httpx.WriteError(w, r, ierr)
		return
	}
	// Re-read: a concurrent teacher may have won the insert, and the winner's
	// row is what both of them must see.
	stored, serr := a.d.Queries.GetClassWeeklyProse(ctx, sqlc.GetClassWeeklyProseParams{
		ClassID: id, WeekStart: data.WeekStart,
	})
	if serr != nil {
		httpx.WriteJSON(w, http.StatusOK, weeklyDTO(data, nil))
		return
	}
	httpx.WriteJSON(w, http.StatusOK, weeklyDTO(data, &stored))
}

// composeWeeklyProse makes the flagship call and records its cost — including
// when the output is rejected. ok=false means "no prose this time", never an
// error to the client.
func (a *API) composeWeeklyProse(ctx context.Context, r *http.Request, facts agent.WeeklyFacts) (agent.WeeklyProse, bool) {
	resolved, rerr := a.d.EvalResolver(ctx)
	if rerr != nil {
		slog.Warn("weekly prose: no provider", "err", rerr)
		return agent.WeeklyProse{}, false
	}
	prose, usage, cerr := agent.ComposeWeekly(ctx, a.d.Provider, resolved, facts)
	if u, ok := UserFromContext(ctx); ok && resolved.Provider != "" {
		cost, priced := gateway.EstimateCost(resolved.Provider, resolved.Model, usage.InputTokens, usage.OutputTokens)
		if !priced {
			slog.Warn("weekly llm_call: unpriced model — cost recorded as 0", "provider", resolved.Provider, "model", resolved.Model)
		}
		if _, err := a.d.Queries.RecordLLMCall(ctx, sqlc.RecordLLMCallParams{
			UserID: u.ID, ProjectID: pgtype.UUID{Valid: false},
			Surface: "teacher", Purpose: "class_weekly",
			Provider: resolved.Provider, Model: resolved.Model, Tier: resolved.Tier,
			PromptTokens: int32(usage.InputTokens), CompletionTokens: int32(usage.OutputTokens),
			CostEstimate: gateway.CostNumeric(cost, true),
		}); err != nil {
			slog.Warn("weekly prose: record llm call", "err", err)
		}
	}
	if cerr != nil {
		slog.Warn("weekly prose: rejected", "err", cerr)
		return agent.WeeklyProse{}, false
	}
	return prose, true
}

// missingCardFacts returns the fact-sheet cards that the stored prose has no
// wording for — the top-up's input.
func missingCardFacts(stored []byte, facts agent.WeeklyFacts) []agent.WeeklyFactCard {
	have := map[string]bool{}
	var cards []agent.WeeklyCardProse
	if json.Unmarshal(stored, &cards) == nil {
		for _, c := range cards {
			have[c.UserID] = true
		}
	}
	var missing []agent.WeeklyFactCard
	for _, c := range facts.Cards {
		if !have[c.UserID] {
			missing = append(missing, c)
		}
	}
	return missing
}
```

Add the imports this needs: `log/slog`, `github.com/jackc/pgx/v5/pgtype`, `mindimprint/api/internal/gateway`.

- [ ] **Step 4: Register the route**

In `apps/api/internal/api/api.go`, next to the GET added in Task 8:

```go
	mux.Handle("POST /api/v1/classes/{id}/weekly-report/prose", teacherOrAdmin(a.postClassWeeklyProse))
```

- [ ] **Step 5: Run the full api package**

Run: `cd apps/api && DOCKER_HOST=unix:///var/run/docker.sock CGO_ENABLED=0 go test -p 1 ./internal/api/...`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
cd /Users/houyuxin/08Coding/mind-imprint
git add apps/api/internal/api/teacher_weekly.go apps/api/internal/api/teacher_weekly_test.go apps/api/internal/api/api.go
git commit -m "feat(d2): POST weekly-report prose — generate once, top up, never wall"
```

---

### Task 10: Migration 0034 — seed the demo week

**Files:**
- Create: `apps/api/internal/store/migrations/0034_seed_teacher_week.sql`
- Test: `apps/api/internal/api/teacher_weekly_test.go` (add one seeded-class test)

**Interfaces:**
- Consumes: the user/class/project/evaluation IDs seeded by `0029_seed_teacher_class.sql` — read it first.
- Produces: a seeded class whose weekly report is non-trivial on a fresh dev DB.

**Why:** 0029's events are all `now() - interval '<hours>'`, so every student shows 1 active day, every delta is `+N` off a zero baseline, and no watch or praise rule except `never_used` can fire.

- [ ] **Step 1: Write the migration**

Create `apps/api/internal/store/migrations/0034_seed_teacher_week.sql`:

```sql
-- +goose Up
-- D2: 0029's events all sit within hours of now(), so every student reads 1
-- active day, every delta is +N off a zero baseline, and no rule except
-- never_used can fire. Spread them across real days, add a previous-week arm,
-- and give one student a second (older) report so 深度升档 has a baseline.
--
-- Anchored on date_trunc('week', now()) — Postgres's ISO week starts Monday,
-- the same Monday teacher.WeekWindow computes.

-- 1. Spread the existing studio events across distinct days inside this week.
--    ctid ordering is arbitrary but stable within one statement; all we need is
--    that a student's two events land on two different days.
UPDATE event SET created_at = date_trunc('week', now()) + interval '1 day' + interval '9 hours'
 WHERE type = 'prompt_sent' AND created_at > date_trunc('week', now())
   AND user_id IN ('00000000-0000-0000-0000-000000000911','00000000-0000-0000-0000-000000000912',
                   '00000000-0000-0000-0000-000000000915','00000000-0000-0000-0000-000000000918');
UPDATE event SET created_at = date_trunc('week', now()) + interval '3 days' + interval '14 hours'
 WHERE type = 'prompt_sent' AND created_at < date_trunc('week', now()) + interval '2 days'
   AND user_id IN ('00000000-0000-0000-0000-000000000913','00000000-0000-0000-0000-000000000916',
                   '00000000-0000-0000-0000-000000000917');

-- 2. Extra distinct-day turns so 林知远/苏晚 read as genuinely active.
INSERT INTO event (user_id, project_id, surface, type, created_at) VALUES
  ('00000000-0000-0000-0000-000000000911','00000000-0000-0000-0000-000000000951','studio','prompt_sent', date_trunc('week', now()) + interval '2 days 10 hours'),
  ('00000000-0000-0000-0000-000000000911','00000000-0000-0000-0000-000000000951','studio','prompt_sent', date_trunc('week', now()) + interval '4 days 11 hours'),
  ('00000000-0000-0000-0000-000000000918','00000000-0000-0000-0000-000000000957','studio','prompt_sent', date_trunc('week', now()) + interval '2 days 16 hours'),
  ('00000000-0000-0000-0000-000000000918','00000000-0000-0000-0000-000000000957','studio','prompt_sent', date_trunc('week', now()) + interval '3 days 9 hours');

-- 3. 陈屿 (914) has no project in 0029, so scope these on the seeded course —
--    the course_id scope 0031 just added. Five distinct days LAST week, one
--    this week: 本周掉线 fires with a real 5 天 → 1 天 evidence line.
INSERT INTO event (user_id, course_id, surface, type, created_at)
SELECT '00000000-0000-0000-0000-000000000914', c.id, 'course', 'course_message',
       date_trunc('week', now()) - interval '7 days' + (n || ' days 10 hours')::interval
FROM (SELECT id FROM course ORDER BY created_at LIMIT 1) c, generate_series(0,4) AS n;
INSERT INTO event (user_id, course_id, surface, type, created_at)
SELECT '00000000-0000-0000-0000-000000000914', c.id, 'course', 'course_message',
       date_trunc('week', now()) + interval '1 day 10 hours'
FROM (SELECT id FROM course ORDER BY created_at LIMIT 1) c;

-- 4. step_viewed on both weeks so 完成课程节 has a real delta (more this week).
INSERT INTO event (user_id, course_id, surface, type, payload, created_at)
SELECT u.uid, c.id, 'course', 'step_viewed', jsonb_build_object('ordinal', n),
       date_trunc('week', now()) + interval '2 days 13 hours'
FROM (SELECT id FROM course ORDER BY created_at LIMIT 1) c,
     generate_series(1,3) AS n,
     (VALUES ('00000000-0000-0000-0000-000000000911'::uuid),
             ('00000000-0000-0000-0000-000000000915'::uuid),
             ('00000000-0000-0000-0000-000000000918'::uuid)) AS u(uid);
INSERT INTO event (user_id, course_id, surface, type, payload, created_at)
SELECT u.uid, c.id, 'course', 'step_viewed', jsonb_build_object('ordinal', n),
       date_trunc('week', now()) - interval '5 days'
FROM (SELECT id FROM course ORDER BY created_at LIMIT 1) c,
     generate_series(1,2) AS n,
     (VALUES ('00000000-0000-0000-0000-000000000911'::uuid),
             ('00000000-0000-0000-0000-000000000915'::uuid)) AS u(uid);

-- 5. 吴桐 (915) gets two project reports 10 days and 1 day old: max depth L2 →
--    L3, so 深度升档 fires and one bucket change appears. Minimal but complete
--    axis coverage, same convention as 0029 (seeds bypass the Go normalizers).
INSERT INTO evaluations (id, project_id, scores, narrative, model, tier, prompt_tokens, completion_tokens, cost_estimate, status, completed_at, created_at) VALUES
  ('00000000-0000-0000-0000-000000000981','00000000-0000-0000-0000-000000000954',
   $j${"depthAxis":[{"code":"D1","level":"L2","evidence":"问题还停在「食物浪费严不严重」这一层。"},{"code":"D2","level":"L2","evidence":"引用了两条校园公众号推文，没有查证原始数据。"},{"code":"D3","level":"L2","evidence":"结论与证据之间缺一步推理。"},{"code":"D4","level":"L1","evidence":"只写了自己这一方的说法。"},{"code":"D5","level":"L2","evidence":"改了措辞，没有改判断。"},{"code":"D6","level":"L2","evidence":"复盘只写了「下次要更认真」。"}],"autonomyAxis":[{"code":"A1","level":2,"opportunity":"given_taken","evidence":"方向由老师给定，自己接了下来。"},{"code":"A2","level":2,"opportunity":"given_taken","evidence":"多数提问在被提示之后。"},{"code":"A3","level":1,"opportunity":"given_not_taken","evidence":"没有对 AI 设过边界。"},{"code":"A4","level":1,"opportunity":"given_not_taken","evidence":"没有请 AI 挑过毛病。"},{"code":"A5","level":2,"opportunity":"given_taken","evidence":"结论署了自己的名，但理由是 AI 给的。"},{"code":"A6","level":2,"opportunity":"given_taken","evidence":"更在意写完，不太追问真假。"}],"narrative":"起步阶段：任务在推进，判断还多在外部。"}$j$,
   '起步阶段：任务在推进，判断还多在外部。','deepseek-v4-pro','flagship',0,0,0,'done', now() - interval '10 days', now() - interval '10 days'),
  ('00000000-0000-0000-0000-000000000982','00000000-0000-0000-0000-000000000954',
   $j${"depthAxis":[{"code":"D1","level":"L3","evidence":"问题收窄到「本校午餐时段的浪费成因」。"},{"code":"D2","level":"L3","evidence":"改用了食堂三周的称重记录作为主证据。"},{"code":"D3","level":"L2","evidence":"推理链补上了一步，但仍有跳跃。"},{"code":"D4","level":"L2","evidence":"补了食堂工作人员这一方的说法。"},{"code":"D5","level":"L3","evidence":"根据 AI 指出的漏洞改了样本说明。"},{"code":"D6","level":"L2","evidence":"复盘写清了自己改了什么，还没写为什么。"}],"autonomyAxis":[{"code":"A1","level":3,"opportunity":"given_taken","evidence":"自己把范围从全校收到了午餐时段。"},{"code":"A2","level":3,"opportunity":"given_taken","evidence":"三次主动开口提问。"},{"code":"A3","level":2,"opportunity":"given_taken","evidence":"说过一次「不要帮我写」。"},{"code":"A4","level":3,"opportunity":"given_taken","evidence":"三次要求 AI「挑我方法里最致命的问题」，并据此改了样本说明。"},{"code":"A5","level":3,"opportunity":"given_taken","evidence":"结论是自己下的，理由也自己写。"},{"code":"A6","level":3,"opportunity":"given_taken","evidence":"发现数据对不上时自己回去核了一遍。"}],"narrative":"这一轮开始主动请 AI 当反方，判断收回了自己手里。"}$j$,
   '这一轮开始主动请 AI 当反方，判断收回了自己手里。','deepseek-v4-pro','flagship',0,0,0,'done', now() - interval '1 day', now() - interval '1 day');

-- +goose Down
DELETE FROM evaluations WHERE id IN ('00000000-0000-0000-0000-000000000981','00000000-0000-0000-0000-000000000982');
DELETE FROM event WHERE type = 'step_viewed'
   AND user_id IN ('00000000-0000-0000-0000-000000000911','00000000-0000-0000-0000-000000000915','00000000-0000-0000-0000-000000000918');
DELETE FROM event WHERE user_id = '00000000-0000-0000-0000-000000000914' AND type = 'course_message';
DELETE FROM event WHERE type = 'prompt_sent'
   AND user_id IN ('00000000-0000-0000-0000-000000000911','00000000-0000-0000-0000-000000000918')
   AND created_at > date_trunc('week', now()) + interval '2 days';
```

罗一 (`...000919`) is deliberately left untouched with zero events — `never_used` needs a real subject.

Verify after writing: `goose` Up then Down then Up again must all succeed (the migrate test in Step 3 of Task 2 shows the idiom). If the seeded course table is empty on a fresh DB, the `SELECT id FROM course` sub-selects yield no rows and those INSERTs insert nothing — check whether an earlier migration seeds a course, and if not, scope 陈屿's events on a project instead by adding one for that student.

- [ ] **Step 2: Write the seeded-class assertion**

Append to `apps/api/internal/api/teacher_weekly_test.go`:

```go
func TestWeeklyReportForSeededClass(t *testing.T) {
	pool := newAPITestPool(t)
	a := New(DepsForTest(t, pool))
	wu := signInAs(t, pool, uuid.MustParse("00000000-0000-0000-0000-000000000910"))

	req := withCookie(httptest.NewRequest(http.MethodGet,
		"/api/v1/classes/00000000-0000-0000-0000-000000000001/weekly-report", nil), wu)
	rec := httptest.NewRecorder()
	a.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body %s", rec.Code, rec.Body.String())
	}
	var dto WeeklyReportDTO
	if err := json.Unmarshal(rec.Body.Bytes(), &dto); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if dto.ClassSize != 9 {
		t.Fatalf("classSize = %d; want 9", dto.ClassSize)
	}
	byTag := map[string]bool{}
	for _, c := range append(append([]WeeklyCardDTO{}, dto.Watch...), dto.Praise...) {
		byTag[c.TagCode] = true
		if c.Evidence == "" {
			t.Fatalf("card %s has no evidence — 每个判断带证据", c.TagCode)
		}
	}
	for _, want := range []string{"never_used", "dropped_off"} {
		if !byTag[want] {
			t.Fatalf("seeded class produced no %s card; tags = %v", want, byTag)
		}
	}
	if dto.Depth.RatedCount == 0 {
		t.Fatal("ratedCount = 0; the seeded class has evaluations")
	}
}
```

The seeded class id is the one 0029 uses — read it from that file rather than trusting the constant above if they differ.

- [ ] **Step 3: Run the full backend**

Run: `cd apps/api && DOCKER_HOST=unix:///var/run/docker.sock CGO_ENABLED=0 go test -p 1 ./...`
Expected: PASS. A seed change can shift counts asserted by pre-existing tests (D1's `admin_teachers` count test did exactly this). Any such adjustment must be a genuine adaptation, not a weakening — say which, and why, in your report.

- [ ] **Step 4: Commit**

```bash
cd /Users/houyuxin/08Coding/mind-imprint
git add apps/api/internal/store/migrations/0034_seed_teacher_week.sql apps/api/internal/api/teacher_weekly_test.go
git commit -m "feat(d2): seed a demonstrable teacher week"
```

---

### Task 11: Web API client + the 周报 screen

**Files:**
- Modify: `apps/web/src/api/teacher.ts`, `apps/web/src/api/index.ts`
- Create: `apps/web/src/console/ClassWeeklyView.tsx`, `apps/web/test/console/ClassWeeklyView.test.tsx`

**Interfaces:**
- Consumes: `GET /api/v1/classes/{id}/weekly-report`, `POST /api/v1/classes/{id}/weekly-report/prose`.
- Produces: `getClassWeeklyReport(classId)`, `generateClassWeeklyProse(classId)`, the `WeeklyReport` type, and `<ClassWeeklyView client classId onOpenStudent onOpenReport />`.

- [ ] **Step 1: Add the client calls**

Append to `apps/web/src/api/teacher.ts`:

```ts
export interface WeeklyCard {
  userId: string;
  displayName: string;
  avatarColor: string;
  tagCode: string;
  tagLabel: string;
  kind: "praise" | "watch";
  evidence: string;
  lead: string;
  action: string;
  hasReport: boolean;
  reportSurface?: string;
  reportScopeId?: string;
}

export interface WeeklyReport {
  weekLabel: string;
  weekStart: string;
  weekEnd: string;
  asOf: string;
  className: string;
  classSize: number;
  stats: { key: string; label: string; value: number; unit: string; foot: string; delta: string; deltaDir: "up" | "down" | "flat" }[];
  praise: WeeklyCard[];
  watch: WeeklyCard[];
  depth: { buckets: { code: string; label: string; count: number }[]; ratedCount: number; note: string };
  autonomy: { mean: string; delta: string; ratedCount: number; note: string };
  comment: string | null;
  proseReady: boolean;
}

export async function getClassWeeklyReport(classId: string): Promise<WeeklyReport> {
  return apiFetch<WeeklyReport>(`/api/v1/classes/${classId}/weekly-report`);
}

export async function generateClassWeeklyProse(classId: string): Promise<WeeklyReport> {
  return apiFetch<WeeklyReport>(`/api/v1/classes/${classId}/weekly-report/prose`, { method: "POST" });
}
```

Check `apiFetch`'s real options signature in `apps/web/src/api/client.ts` and match it. Then export both from `apps/web/src/api/index.ts` beside the existing D1 teacher exports, and add them to the `ApiClient` aggregate the same way `getClassRosterReport` is added.

- [ ] **Step 2: Write the failing view tests**

Create `apps/web/test/console/ClassWeeklyView.test.tsx`:

```tsx
import { render, screen, waitFor } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { ClassWeeklyView } from "@/console/ClassWeeklyView";
import type { WeeklyReport } from "@/api/teacher";

function report(over: Partial<WeeklyReport> = {}): WeeklyReport {
  return {
    weekLabel: "第 30 周（7.20–7.26）",
    weekStart: "2026-07-20T00:00:00Z",
    weekEnd: "2026-07-27T00:00:00Z",
    asOf: "2026-07-24T07:30:00Z",
    className: "IBDP 一年级 · 研究组",
    classSize: 9,
    stats: [
      { key: "active_students", label: "本周活跃学生", value: 8, unit: "/ 9 人", foot: "登录并有活动的学生", delta: "+2", deltaDir: "up" },
      { key: "reports", label: "生成能力报告", value: 14, unit: "份", foot: "来自项目、对话与课程", delta: "±0", deltaDir: "flat" },
      { key: "turns", label: "AI 对话轮次", value: 386, unit: "轮", foot: "反映本周使用强度", delta: "-72", deltaDir: "down" },
      { key: "course_steps", label: "完成课程节", value: 23, unit: "节", foot: "平台内自学课程", delta: "+4", deltaDir: "up" },
    ],
    praise: [],
    watch: [{
      userId: "u1", displayName: "周子墨", avatarColor: "#C4574D",
      tagCode: "outsourced_judgment", tagLabel: "判断在外包", kind: "watch",
      evidence: "A 轴 0.5/5。提示词多为「帮我写一段」。",
      lead: "", action: "", hasReport: true, reportSurface: "project", reportScopeId: "p1",
    }],
    depth: { buckets: [
      { code: "L1", label: "起步 L1", count: 2 }, { code: "L2", label: "发展 L2", count: 3 },
      { code: "L3", label: "熟练 L3", count: 2 }, { code: "L4", label: "优秀 L4", count: 1 },
    ], ratedCount: 8, note: "" },
    autonomy: { mean: "2.6", delta: "+0.4", ratedCount: 8, note: "" },
    comment: null,
    proseReady: false,
    ...over,
  };
}

describe("ClassWeeklyView", () => {
  it("renders the four stat cards with their deltas", async () => {
    const client = {
      getClassWeeklyReport: vi.fn().mockResolvedValue(report({ proseReady: true, comment: "本周点评" })),
      generateClassWeeklyProse: vi.fn(),
    };
    render(<ClassWeeklyView client={client} classId="c1" onOpenStudent={() => {}} onOpenReport={() => {}} />);
    expect(await screen.findByText("本周活跃学生")).toBeInTheDocument();
    expect(screen.getByText("386")).toBeInTheDocument();
    expect(screen.getByText("±0")).toBeInTheDocument();
    expect(screen.getByText("-72")).toBeInTheDocument();
    expect(client.generateClassWeeklyProse).not.toHaveBeenCalled();
  });

  it("requests prose exactly once when it is missing", async () => {
    const client = {
      getClassWeeklyReport: vi.fn().mockResolvedValue(report()),
      generateClassWeeklyProse: vi.fn().mockResolvedValue(report({ proseReady: true, comment: "生成后的点评" })),
    };
    render(<ClassWeeklyView client={client} classId="c1" onOpenStudent={() => {}} onOpenReport={() => {}} />);
    expect(await screen.findByText("生成后的点评")).toBeInTheDocument();
    await waitFor(() => expect(client.generateClassWeeklyProse).toHaveBeenCalledTimes(1));
  });

  it("renders a card's evidence even with no wording", async () => {
    const client = {
      getClassWeeklyReport: vi.fn().mockResolvedValue(report()),
      generateClassWeeklyProse: vi.fn().mockResolvedValue(report()),
    };
    render(<ClassWeeklyView client={client} classId="c1" onOpenStudent={() => {}} onOpenReport={() => {}} />);
    expect(await screen.findByText("周子墨")).toBeInTheDocument();
    expect(screen.getByText("判断在外包")).toBeInTheDocument();
    expect(screen.getByText(/A 轴 0.5\/5/)).toBeInTheDocument();
    expect(screen.getByText("本周点评暂未生成")).toBeInTheDocument();
  });

  it("shows the empty state when nobody needs attention", async () => {
    const client = {
      getClassWeeklyReport: vi.fn().mockResolvedValue(report({ watch: [], praise: [], proseReady: true, comment: "c" })),
      generateClassWeeklyProse: vi.fn(),
    };
    render(<ClassWeeklyView client={client} classId="c1" onOpenStudent={() => {}} onOpenReport={() => {}} />);
    expect(await screen.findByText("本周没有需要特别关注的学生")).toBeInTheDocument();
  });

  it("renders the unrated distribution empty state", async () => {
    // The server ALWAYS sends all four buckets — an unrated class sends them
    // with count 0, never an empty array. Mocking [] here would test a shape
    // the backend cannot produce.
    const client = {
      getClassWeeklyReport: vi.fn().mockResolvedValue(report({
        depth: { buckets: [
          { code: "L1", label: "起步 L1", count: 0 }, { code: "L2", label: "发展 L2", count: 0 },
          { code: "L3", label: "熟练 L3", count: 0 }, { code: "L4", label: "优秀 L4", count: 0 },
        ], ratedCount: 0, note: "" },
        autonomy: { mean: "—", delta: "—", ratedCount: 0, note: "" },
        proseReady: true, comment: "c",
      })),
      generateClassWeeklyProse: vi.fn(),
    };
    render(<ClassWeeklyView client={client} classId="c1" onOpenStudent={() => {}} onOpenReport={() => {}} />);
    expect(await screen.findByText("暂无可计入的证据")).toBeInTheDocument();
  });

  it("renders a distinct error with a retry when the fetch fails", async () => {
    const client = {
      getClassWeeklyReport: vi.fn().mockRejectedValue(new Error("boom")),
      generateClassWeeklyProse: vi.fn(),
    };
    render(<ClassWeeklyView client={client} classId="c1" onOpenStudent={() => {}} onOpenReport={() => {}} />);
    expect(await screen.findByText("重试")).toBeInTheDocument();
  });
});
```

- [ ] **Step 3: Run to verify they fail**

Run: `cd apps/web && npm test -- ClassWeeklyView`
Expected: FAIL — cannot resolve `@/console/ClassWeeklyView`.

- [ ] **Step 4: Implement the view**

Create `apps/web/src/console/ClassWeeklyView.tsx`.

The load-and-generate-once logic is the subtle part; write it exactly like this:

```tsx
export function ClassWeeklyView({ client, classId, onOpenStudent, onOpenReport }: {
  client: Pick<ApiClient, "getClassWeeklyReport" | "generateClassWeeklyProse">;
  classId: string;
  onOpenStudent: (userId: string) => void;
  onOpenReport: (surface: string, scopeId: string, displayName: string) => void;
}) {
  const [data, setData] = useState<WeeklyReport | null>(null);
  const [error, setError] = useState<string | null>(null);
  // Generation is a SPEND. This ref is what stops a re-render from firing a
  // second flagship call; the server is idempotent per (class, week), but the
  // client must not lean on that.
  const asked = useRef<string | null>(null);

  function load() {
    setError(null);
    client.getClassWeeklyReport(classId).then((r) => {
      setData(r);
      if (!r.proseReady && asked.current !== classId) {
        asked.current = classId;
        client.generateClassWeeklyProse(classId).then(setData).catch(() => {
          /* 敢于空白: keep the numbers and the evidence; prose stays absent. */
        });
      }
    }).catch((e) => {
      setData(null);
      setError(e instanceof ApiError ? e.message : "加载失败");
    });
  }
  useEffect(load, [client, classId]);

  if (error) {
    return (
      <div style={{ marginTop: 30, color: "#C76B6B", fontSize: 14, fontWeight: 600 }}>
        周报加载失败：{error} · <span onClick={load} style={{ cursor: "pointer", textDecoration: "underline" }}>重试</span>
      </div>
    );
  }
  if (!data) return <div style={{ marginTop: 30, color: "#8A92A3", fontSize: 14.5 }}>加载中…</div>;
  // …render below
}
```

The delta pill's three states, also exact:

```tsx
const DELTA_STYLE: Record<string, { fg: string; bg: string }> = {
  up: { fg: "#3E8A6E", bg: "#E9F2EC" },
  down: { fg: "#C4574D", bg: "#F7E6E4" },
  flat: { fg: "#8A92A3", bg: "#EEF0F4" },
};
```

`up` renders an inline-SVG up arrow, `down` a down arrow, `flat` no arrow at all.

The rest of the layout follows the binding design (`思维印记 教师端.dc.html:75–232`). Requirements, all binding:

- Header: `班级周报 · {weekLabel}` and the design's subtitle 「作业布置与提交在学校自己的平台完成。这里只看学生在思维印记上的使用强度，和他们思考维度的变化。」
- Four stat cards render in the server's order — never reordered or relabelled client-side.
- Bean comment block: renders `comment` when present, else 「本周点评暂未生成」.
- Four stat cards in the server's order, rendering `value`, `unit`, `foot`, and the `delta` pill colored by `deltaDir` — `up` `#3E8A6E` on `#E9F2EC`, `down` `#C4574D` on `#F7E6E4`, `flat` `#8A92A3` on `#EEF0F4`. Arrows are inline SVG; `flat` renders no arrow.
- 建议关注 section with the design's caption 「先看值得表扬的，再看需要给建议的 · 每张卡都带证据，具体沟通在线下进行」; 值得表扬 renders first, then 需要建议. When both lists are empty, render 「本周没有需要特别关注的学生」 and neither heading.
- Each card: avatar initial, clickable name → `onOpenStudent(userId)`, the tag chip, `lead` (omit the element entirely when `lead === ""`), `evidence`, the action row prefixed 「怎么鼓励：」 (praise) or 「怎么开口：」 (watch) — omit the whole row when `action === ""` — and, when `hasReport`, a 「看能力报告」 link calling `onOpenReport(reportSurface!, reportScopeId!, displayName)`.
- 班级思维维度 block: the D distribution as a stacked bar plus a legend of `label` + `count 人`, colored via `badgeColor(bucket.code).fg`; `已评估 {ratedCount} 人`; the `depth.note` under it when non-empty. Then the A block: `{mean} / 5`, the `delta` pill, and `autonomy.note` when non-empty. When `ratedCount === 0`, render 「暂无可计入的证据」 in place of the whole distribution.
- Error state: 「周报加载失败：{message} · 重试」 with the retry re-running the load.
- **Derive nothing.** Every number and every string comes from the server. No `Math.round`, no percentage of a mean, no badge computation. The stacked-bar widths are the only arithmetic allowed, and only as `count / ratedCount`.

- [ ] **Step 5: Run to verify they pass**

Run: `cd apps/web && npm test && npx tsc --noEmit`
Expected: PASS, tsc clean. Run the FULL web suite before committing, not the
`-- ClassWeeklyView` subset from Step 3 — adding a client method and an
`ApiClient` field ripples into every mock that satisfies that type.

- [ ] **Step 6: Commit**

```bash
cd /Users/houyuxin/08Coding/mind-imprint
git add apps/web/src/api/teacher.ts apps/web/src/api/index.ts apps/web/src/console/ClassWeeklyView.tsx apps/web/test/console/ClassWeeklyView.test.tsx
git commit -m "feat(d2): 班级周报 screen + client"
```

---

### Task 12: Sub-tabs in the class view

**Files:**
- Create: `apps/web/src/console/ClassRosterTable.tsx`
- Modify: `apps/web/src/console/ClassDetailView.tsx`, `apps/web/test/console/ClassDetailView.test.tsx`

**Interfaces:**
- Consumes: `<ClassWeeklyView>` (Task 11).
- Produces: `<ClassRosterTable roster onOpenStudent onRemove confirmRemove setConfirmRemove busy />` — D1's table, extracted verbatim.

- [ ] **Step 1: Extract the roster table**

Move the `<table>` JSX from `ClassDetailView.tsx:255-319` into `apps/web/src/console/ClassRosterTable.tsx`, along with the `TH`, `TD`, and `Badge` helpers it uses (lines 8–18) and the `badgeColor` import. Change nothing inside it — this is a move, not a rewrite. The removal controls stay in the table, so the props carry `confirmRemove`, `setConfirmRemove`, `onRemove`, and `busy` through.

- [ ] **Step 2: Add the sub-tabs**

In `ClassDetailView.tsx`:

```tsx
const [subTab, setSubTab] = useState<"weekly" | "roster">("weekly");
```

Render a two-button switch directly above the roster/weekly area, styled like the existing `chipBtn` with the active one filled:

```tsx
<div style={{ display: "flex", gap: 8, marginTop: 26 }}>
  <button onClick={() => setSubTab("weekly")} style={subTab === "weekly" ? activeTabBtn : chipBtn}>周报</button>
  <button onClick={() => setSubTab("roster")} style={subTab === "roster" ? activeTabBtn : chipBtn}>全部学生</button>
</div>
```

with

```tsx
const activeTabBtn: React.CSSProperties = { ...chipBtn, background: "#2A3B7A", borderColor: "#2A3B7A", color: "#fff" };
```

`subTab === "weekly"` renders `<ClassWeeklyView client={client} classId={classId} onOpenStudent={onOpenStudent} onOpenReport={onOpenReport} />`; `subTab === "roster"` renders the existing empty/error/loading branches and `<ClassRosterTable …/>`. Widen the `Client` type to include `"getClassWeeklyReport" | "generateClassWeeklyProse"`, and add an `onOpenReport` prop threaded from `ConsoleShell` exactly as `StudentDetailView` already receives it.

- [ ] **Step 3: Update the existing tests**

`ClassDetailView.test.tsx` currently asserts the roster table renders on open. That premise is now inverted: 周报 is the default tab. Update those tests to click 「全部学生」 first, and add:

```tsx
it("lands on the weekly tab and shows the roster only after switching", async () => {
  render(<ClassDetailView {...props} />);
  expect(await screen.findByText(/班级周报/)).toBeInTheDocument();
  expect(screen.queryByText("对话轮次")).not.toBeInTheDocument();
  await userEvent.click(screen.getByText("全部学生"));
  expect(await screen.findByText("对话轮次")).toBeInTheDocument();
});
```

Add `getClassWeeklyReport` / `generateClassWeeklyProse` stubs to the mock client in this file and in `ConsoleShell.test.tsx`. Do not weaken any existing management-control assertion (rename, join code, remove student, assign teacher) — those tests must keep passing unchanged once they click into the roster tab.

- [ ] **Step 4: Run the full web suite**

Run: `cd apps/web && npm test && npx tsc --noEmit`
Expected: PASS, tsc clean.

- [ ] **Step 5: Run everything, one last time**

Run, each in the FOREGROUND:
```
cd apps/api && DOCKER_HOST=unix:///var/run/docker.sock CGO_ENABLED=0 go test -p 1 ./...
cd apps/web && npm test && npx tsc --noEmit
cd packages/contracts && npm test && npx tsc --noEmit
```
Expected: all green.

- [ ] **Step 6: Commit**

```bash
cd /Users/houyuxin/08Coding/mind-imprint
git add apps/web/src/console/ClassRosterTable.tsx apps/web/src/console/ClassDetailView.tsx apps/web/test/console/ClassDetailView.test.tsx apps/web/test/console/ConsoleShell.test.tsx apps/web/src/console/ConsoleShell.tsx
git commit -m "feat(d2): class view sub-tabs — 周报 default, 全部学生 second"
```
