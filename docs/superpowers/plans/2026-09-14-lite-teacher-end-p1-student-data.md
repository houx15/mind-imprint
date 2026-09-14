# Lite Teacher End · Plan 1 — Shell + Student Data Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** A teacher at a lite school logs into lite and sees their classes, a roster with live per-student totals, each student's readings / writings / projects, each item's outputs and moments (never the chat), and the student's interest tree read-only.

**Architecture:** Role branch in `apps/lite-web/src/LiteApp.tsx` mounts a new `LiteTeacherShell` (same pattern as pro `AppShell` → `ConsoleShell`). New Go handlers under `/api/v1/lite/teacher/…` gated `liteOnly` + `RequireRole("teacher","admin")` + `authTeacherStudent`. A per-day time bucket table (`atom_active_day`) is written by every heartbeat, and projects get a heartbeat.

**Tech Stack:** Go (`net/http`, `pgx`, `sqlc` v1.27.0 via `go tool sqlc`, `goose`), PostgreSQL, React + Vite + TypeScript + Tailwind, vitest, Playwright for a one-off screenshot walk.

**Spec:** `docs/superpowers/specs/2026-09-14-lite-teacher-end-design.md` (§3, §4, §8 plan 1, §10). Read it before starting.

## Global Constraints

- Lite must never break pro: never delete or rename files; before creating a file in `apps/api/internal/store/{queries,migrations}` or `apps/api/internal/api`, `ls` it first. Shared-component edits are additive optional props whose default preserves pro behaviour. After the last task run pro's own tests.
- Teachers see outputs and moments only. No teacher endpoint may put `atom_message.content` in a response.
- Every teacher route: `liteOnly` + `RequireRole("teacher","admin")`; every route that names a student uses `authTeacherStudent` (path values `{id}` = class id, `{userId}` = student id); every route that names an atom also checks `atom.user_id = userId`; all failures 404 `资源不存在`.
- Teacher reads never call `loadOwnedAtom` (it bumps `last_activity_at`).
- Beijing time is a fixed `+08:00` offset (`time.FixedZone("CST", 8*3600)`), never `time.LoadLocation` (prod image has no tzdata).
- Heartbeat clamp: 120 seconds per call (`heartbeatCeiling`).
- UI copy follows AGENTS.md § 界面文案怎么写: labels are nouns, buttons are actions, states 已/待/中, errors `动词+失败：{后台原话}`, no literary style.
- Tests are logic tests only (Go handlers, pure functions, normalizers). UI is verified with Playwright screenshots, not jsdom assertions.
- Go tests: `cd apps/api && CGO_ENABLED=0 go test ./internal/... -run <Name> -timeout 1800s` (testcontainers Postgres). Regenerate sqlc: `cd apps/api && make sqlc`.
- Frontend checks: `pnpm --filter lite-web typecheck`, `pnpm --filter lite-web test`, and for pro `pnpm --filter web typecheck`, `pnpm --filter web test`. (If the filter name differs, read `apps/lite-web/package.json` `name`.)
- Commit after each task; stage specific files, never `git add -A`. Commit messages end with `Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>`.

---

## File Structure

**Backend (create)**
- `apps/api/internal/store/migrations/0148_atom_active_day.sql` — per-day time buckets. (Use the next free number if 0148 is taken.)
- `apps/api/internal/store/queries/atom_active_day.sql` — bucket upsert.
- `apps/api/internal/store/queries/lite_teacher.sql` — roster, student totals, item lists for teachers.
- `apps/api/internal/liteweek/liteweek.go` (+ `_test.go`) — Beijing day/week pure functions (plan 3 reuses it).
- `apps/api/internal/api/pbl_heartbeat.go` — project heartbeat handler.
- `apps/api/internal/api/lite_teacher_routes.go` — route registration helper.
- `apps/api/internal/api/lite_teacher_roster.go` — roster + student page handlers.
- `apps/api/internal/api/lite_teacher_item.go` — item detail handler + DTO builders.
- `apps/api/internal/api/lite_teacher_tree.go` — student tree handler.
- Tests: `lite_teacher_roster_test.go`, `lite_teacher_item_test.go`, `lite_teacher_tree_test.go`, `pbl_heartbeat_test.go`, `atom_heartbeat_day_internal_test.go`.

**Backend (modify)**
- `apps/api/internal/api/atom_heartbeat.go` — also write the day bucket.
- `apps/api/internal/api/interest.go` — extract `buildInterestTree(ctx, userID)`; `getInterestTree` calls it (no behaviour change).
- `apps/api/internal/api/api.go` — register PBL heartbeat + call `a.registerLiteTeacherRoutes(mux)`.

**Frontend (create)**
- `apps/lite-web/src/api/teacher.ts` (+ `teacher.test.ts`) — client + normalizers.
- `apps/lite-web/src/teacher/teacherRouting.ts` (+ `.test.ts`) — URL model for the teacher shell.
- `apps/lite-web/src/teacher/LiteTeacherShell.tsx` — rail + route switch.
- `apps/lite-web/src/teacher/ClassPage.tsx` — class header (join code) + roster table.
- `apps/lite-web/src/teacher/StudentPage.tsx` — header stats, item lists, tree.
- `apps/lite-web/src/teacher/ItemPage.tsx` — reading / writing / project detail.
- `apps/lite-web/src/teacher/format.ts` (+ `.test.ts`) — minutes/turns/status formatting.

**Frontend (modify)**
- `apps/lite-web/src/LiteApp.tsx` — role branch.
- `apps/lite-web/src/api/reports.ts` — `sendHeartbeat` accepts `"project"`.
- `apps/lite-web/src/shared/useHeartbeat.ts` — kind type widened.
- `apps/lite-web/src/projects/ProjectRoom.tsx` — `useHeartbeat("project", projectId, …)`.
- `apps/lite-web/src/tree/useInterestTree.ts` — optional fetcher.
- `apps/lite-web/src/tree/TreeView.tsx` — optional `readOnly` prop.

---

### Task 1: Per-day time buckets on every heartbeat

**Files:**
- Create: `apps/api/internal/store/migrations/0148_atom_active_day.sql`
- Create: `apps/api/internal/store/queries/atom_active_day.sql`
- Create: `apps/api/internal/liteweek/liteweek.go`, `apps/api/internal/liteweek/liteweek_test.go`
- Create: `apps/api/internal/api/atom_heartbeat_day_internal_test.go`
- Modify: `apps/api/internal/api/atom_heartbeat.go`

**Interfaces:**
- Produces: `liteweek.Beijing *time.Location`; `liteweek.Day(t time.Time) time.Time` (Beijing midnight of t, in Beijing zone); `liteweek.WeekStart(t time.Time) time.Time` (Monday 00:00 Beijing of t's week); `sqlc.Queries.AddAtomActiveDay(ctx, AddAtomActiveDayParams{AtomID uuid.UUID, Day pgtype.Date, Seconds int32}) error`; `(*API).recordHeartbeat(ctx, atomID uuid.UUID, seconds int32, now time.Time) error`.

- [ ] **Step 1: Write the failing liteweek tests**

```go
package liteweek

import (
	"testing"
	"time"
)

func TestDayUsesBeijingNotUTC(t *testing.T) {
	// 2026-09-13 16:30 UTC = 2026-09-14 00:30 Beijing.
	got := Day(time.Date(2026, 9, 13, 16, 30, 0, 0, time.UTC))
	if got.Year() != 2026 || got.Month() != 9 || got.Day() != 14 {
		t.Fatalf("Day = %v, want 2026-09-14", got)
	}
	if got.Hour() != 0 || got.Minute() != 0 {
		t.Fatalf("Day not at midnight: %v", got)
	}
}

func TestDayBoundaryOneMinuteApart(t *testing.T) {
	before := Day(time.Date(2026, 9, 14, 23, 59, 30, 0, Beijing))
	after := Day(time.Date(2026, 9, 15, 0, 0, 30, 0, Beijing))
	if before.Equal(after) {
		t.Fatalf("23:59:30 and 00:00:30 Beijing landed on the same day %v", before)
	}
}

func TestWeekStartIsMondayBeijing(t *testing.T) {
	// Sunday 2026-09-20 23:59 Beijing belongs to the week starting Monday 2026-09-14.
	got := WeekStart(time.Date(2026, 9, 20, 23, 59, 0, 0, Beijing))
	want := time.Date(2026, 9, 14, 0, 0, 0, 0, Beijing)
	if !got.Equal(want) {
		t.Fatalf("WeekStart = %v, want %v", got, want)
	}
	// Monday 00:00 is its own week's start.
	if m := WeekStart(want); !m.Equal(want) {
		t.Fatalf("WeekStart(Monday 00:00) = %v, want %v", m, want)
	}
}
```

- [ ] **Step 2: Run to verify failure**

Run: `cd apps/api && CGO_ENABLED=0 go test ./internal/liteweek/ -timeout 120s`
Expected: FAIL — `undefined: Day` / package has no Go files.

- [ ] **Step 3: Implement liteweek**

```go
// Package liteweek holds the lite edition's calendar arithmetic: a student's
// day and week are Beijing days and weeks. The offset is fixed (+08:00)
// because the production image has no tzdata, and time.LoadLocation there
// silently falls back to UTC.
package liteweek

import "time"

// Beijing is UTC+8 with no daylight saving.
var Beijing = time.FixedZone("CST", 8*3600)

// Day returns midnight (Beijing) of the Beijing date t falls on.
func Day(t time.Time) time.Time {
	b := t.In(Beijing)
	return time.Date(b.Year(), b.Month(), b.Day(), 0, 0, 0, 0, Beijing)
}

// WeekStart returns Monday 00:00 (Beijing) of the week t falls in.
func WeekStart(t time.Time) time.Time {
	d := Day(t)
	offset := (int(d.Weekday()) + 6) % 7 // Monday → 0 … Sunday → 6
	return d.AddDate(0, 0, -offset)
}
```

- [ ] **Step 4: Run liteweek tests**

Run: `cd apps/api && CGO_ENABLED=0 go test ./internal/liteweek/ -timeout 120s`
Expected: PASS

- [ ] **Step 5: Migration + query**

`ls apps/api/internal/store/migrations | tail -3` — confirm 0148 is free. Create `0148_atom_active_day.sql`:

```sql
-- +goose Up
-- 每一次心跳按北京日期落一格。atom.active_seconds 是累计值，拆不出「上周」；
-- 教师端的本周时长与上周表现总结都从这张表按日期求和。
-- 不回填：迁移之前的日子没有格子，界面显示「—」，不估算。
CREATE TABLE atom_active_day (
  atom_id uuid NOT NULL REFERENCES atom(id) ON DELETE CASCADE,
  day     date NOT NULL,
  seconds integer NOT NULL DEFAULT 0 CHECK (seconds >= 0),
  PRIMARY KEY (atom_id, day)
);
CREATE INDEX atom_active_day_day_idx ON atom_active_day (day);

-- +goose Down
DROP TABLE atom_active_day;
```

`ls apps/api/internal/store/queries/atom_active_day.sql` must fail (file absent). Create it:

```sql
-- name: AddAtomActiveDay :exec
INSERT INTO atom_active_day (atom_id, day, seconds)
VALUES ($1, $2, $3)
ON CONFLICT (atom_id, day) DO UPDATE SET seconds = atom_active_day.seconds + EXCLUDED.seconds;
```

Run: `cd apps/api && make sqlc && go build ./...` — Expected: builds.

- [ ] **Step 6: Write the failing heartbeat-bucket test**

`atom_heartbeat_day_internal_test.go` (package `api`, internal test so it can call the unexported method). Look at `atom_heartbeat_internal_test.go` for how an internal test gets a pool and a seeded atom, and reuse the same helpers.

```go
package api

import (
	"context"
	"testing"
	"time"

	"mindimprint/api/internal/liteweek"
)

func TestRecordHeartbeatSplitsAcrossBeijingMidnight(t *testing.T) {
	a, atomID := newHeartbeatTestAtom(t) // helper defined below in this file
	ctx := context.Background()

	if err := a.recordHeartbeat(ctx, atomID, 60, time.Date(2026, 9, 14, 23, 59, 30, 0, liteweek.Beijing)); err != nil {
		t.Fatal(err)
	}
	if err := a.recordHeartbeat(ctx, atomID, 500, time.Date(2026, 9, 15, 0, 0, 30, 0, liteweek.Beijing)); err != nil {
		t.Fatal(err)
	}

	rows, err := a.d.Pool.Query(ctx, `SELECT day::text, seconds FROM atom_active_day WHERE atom_id=$1 ORDER BY day`, atomID)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	got := map[string]int32{}
	for rows.Next() {
		var d string
		var s int32
		if err := rows.Scan(&d, &s); err != nil {
			t.Fatal(err)
		}
		got[d] = s
	}
	if got["2026-09-14"] != 60 || got["2026-09-15"] != 120 {
		t.Fatalf("buckets = %v, want 09-14:60 09-15:120 (second call clamped)", got)
	}
	var total int32
	if err := a.d.Pool.QueryRow(ctx, `SELECT active_seconds FROM atom WHERE id=$1`, atomID).Scan(&total); err != nil {
		t.Fatal(err)
	}
	if total != 180 {
		t.Fatalf("atom.active_seconds = %d, want 180", total)
	}
}
```

Write `newHeartbeatTestAtom(t) (*API, uuid.UUID)` in the same file using whatever `atom_heartbeat_internal_test.go` already uses to build an `*API` against a test DB (`NewTestDB(t)`) and create a reading atom for the seed user (`sqlc.Queries.CreateAtom` + `CreateReading`; read `queries/atom.sql` and `queries/reading.sql` for exact params).

- [ ] **Step 7: Run to verify failure**

Run: `cd apps/api && CGO_ENABLED=0 go test ./internal/api -run TestRecordHeartbeatSplitsAcrossBeijingMidnight -timeout 1800s`
Expected: FAIL — `a.recordHeartbeat undefined`.

- [ ] **Step 8: Implement recordHeartbeat and use it**

In `atom_heartbeat.go` add (imports: `context`, `time`, `github.com/google/uuid`, `github.com/jackc/pgx/v5/pgtype`, `mindimprint/api/internal/liteweek`):

```go
// recordHeartbeat adds one clamped heartbeat to the atom's running total and
// to today's Beijing-date bucket, in one transaction so the two never
// disagree. `now` is a parameter so the midnight split is testable.
func (a *API) recordHeartbeat(ctx context.Context, atomID uuid.UUID, seconds int32, now time.Time) error {
	secs := clampHeartbeatSeconds(seconds)
	tx, err := a.d.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(context.WithoutCancel(ctx)) }()
	qtx := a.d.Queries.WithTx(tx)
	if err := qtx.AddAtomActiveSeconds(ctx, sqlc.AddAtomActiveSecondsParams{ID: atomID, ActiveSeconds: secs}); err != nil {
		return err
	}
	if err := qtx.AddAtomActiveDay(ctx, sqlc.AddAtomActiveDayParams{
		AtomID:  atomID,
		Day:     pgtype.Date{Time: liteweek.Day(now), Valid: true},
		Seconds: secs,
	}); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
```

Replace the `AddAtomActiveSeconds` call inside `heartbeatAtom` with:

```go
	if err := a.recordHeartbeat(r.Context(), at.ID, req.Seconds, time.Now()); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
```

If sqlc generated `Day` as `time.Time` rather than `pgtype.Date` (check `internal/store/sqlc/atom_active_day.sql.go`), pass `liteweek.Day(now)` directly.

- [ ] **Step 9: Run tests**

Run: `cd apps/api && CGO_ENABLED=0 go test ./internal/api -run 'TestRecordHeartbeat|Heartbeat' -timeout 1800s`
Expected: PASS (new test and existing heartbeat tests).

- [ ] **Step 10: Commit**

```bash
git add apps/api/internal/store/migrations/0148_atom_active_day.sql apps/api/internal/store/queries/atom_active_day.sql apps/api/internal/store/sqlc/ apps/api/internal/liteweek/ apps/api/internal/api/atom_heartbeat.go apps/api/internal/api/atom_heartbeat_day_internal_test.go
git commit -m "feat(lite): 心跳按北京日期落格（atom_active_day）"
```

---

### Task 2: Project heartbeat

**Files:**
- Create: `apps/api/internal/api/pbl_heartbeat.go`, `apps/api/internal/api/pbl_heartbeat_test.go`
- Modify: `apps/api/internal/api/api.go` (one route line near the other `/api/v1/pbl/projects/{id}/…` routes)
- Modify: `apps/lite-web/src/api/reports.ts`, `apps/lite-web/src/shared/useHeartbeat.ts`, `apps/lite-web/src/projects/ProjectRoom.tsx`

**Interfaces:**
- Consumes: `(*API).recordHeartbeat` (Task 1), `(*API).loadOwnedPblProject(w, r) (atomID uuid.UUID, ok bool)`.
- Produces: `POST /api/v1/pbl/projects/{id}/heartbeat` `{seconds}` → 204. Frontend `sendHeartbeat(kind: "reading"|"writing"|"project", id, seconds)`.

- [ ] **Step 1: Write the failing handler tests**

```go
package api_test

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestPblHeartbeatAddsSecondsAndBucket(t *testing.T) {
	h, cookie, _, pool := liteHandler(t)
	projectID, atomID := createPblProjectForTest(t, pool) // see note below

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("POST", "/api/v1/pbl/projects/"+projectID+"/heartbeat",
		bytes.NewReader([]byte(`{"seconds":60}`))), cookie))
	if rec.Code != http.StatusNoContent {
		t.Fatalf("heartbeat = %d body=%s", rec.Code, rec.Body)
	}
	var total, bucket int32
	if err := pool.QueryRow(context.Background(), `SELECT active_seconds FROM atom WHERE id=$1`, atomID).Scan(&total); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(context.Background(), `SELECT COALESCE(sum(seconds),0) FROM atom_active_day WHERE atom_id=$1`, atomID).Scan(&bucket); err != nil {
		t.Fatal(err)
	}
	if total != 60 || bucket != 60 {
		t.Fatalf("total=%d bucket=%d, want 60/60", total, bucket)
	}
}

func TestPblHeartbeatFinishedProjectIsNoop(t *testing.T) {
	h, cookie, _, pool := liteHandler(t)
	projectID, atomID := createPblProjectForTest(t, pool)
	if _, err := pool.Exec(context.Background(), `UPDATE pbl_project SET status='review' WHERE id=$1`, projectID); err != nil {
		t.Fatal(err)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("POST", "/api/v1/pbl/projects/"+projectID+"/heartbeat",
		bytes.NewReader([]byte(`{"seconds":60}`))), cookie))
	if rec.Code != http.StatusNoContent {
		t.Fatalf("heartbeat = %d", rec.Code)
	}
	var total int32
	_ = pool.QueryRow(context.Background(), `SELECT active_seconds FROM atom WHERE id=$1`, atomID).Scan(&total)
	if total != 0 {
		t.Fatalf("finished project accrued %d seconds", total)
	}
}

func TestPblHeartbeatOtherStudentsProject404(t *testing.T) {
	h, _, _, pool := liteHandler(t)
	projectID, _ := createPblProjectForTest(t, pool)
	other := signInAs(t, pool, createStudent(t, pool, SeedSchoolID, "hb-other@demo.local"))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("POST", "/api/v1/pbl/projects/"+projectID+"/heartbeat",
		bytes.NewReader([]byte(`{"seconds":60}`))), other))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("other student's heartbeat = %d, want 404", rec.Code)
	}
}
```

`createPblProjectForTest(t, pool) (projectID string, atomID uuid.UUID)`: search `pbl_*_test.go` for an existing helper that inserts a project for the seed user (e.g. `grep -n "func .*[Pp]roject.*(t \*testing.T" apps/api/internal/api/pbl_*_test.go`). Reuse it if present; otherwise write one in this file using `sqlc.CreateAtom` (kind `project`) + `sqlc.CreatePblProject` (read `queries/pbl_project.sql` for params). Note `pbl_project.id` vs `atom_id`: check whether the route `{id}` is the project id or the atom id by reading `GetPblProject`'s WHERE clause, and return whichever the route expects as `projectID`.

- [ ] **Step 2: Run to verify failure**

Run: `cd apps/api && CGO_ENABLED=0 go test ./internal/api -run TestPblHeartbeat -timeout 1800s`
Expected: FAIL — 404/405 on the unregistered route.

- [ ] **Step 3: Implement handler + route**

`pbl_heartbeat.go`:

```go
package api

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/google/uuid"

	"mindimprint/api/internal/httpx"
)

// pblFinishedStatuses are the project states that count as done (same set
// that queues interest harvesting). A heartbeat on a done project is a 204
// no-op: reopening a finished project is not time spent on the work.
var pblFinishedStatuses = map[string]bool{"review": true, "keeping": true, "archived": true}

// postPblHeartbeat handles POST /api/v1/pbl/projects/{id}/heartbeat — the
// project room's half of the minute count readings and writings already have.
func (a *API) postPblHeartbeat(w http.ResponseWriter, r *http.Request) {
	atomID, ok := a.loadOwnedPblProject(w, r)
	if !ok {
		return
	}
	// loadOwnedPblProject already parsed {id} successfully, so this cannot fail.
	projectID, _ := uuid.Parse(r.PathValue("id"))
	proj, err := a.d.Queries.GetPblProject(r.Context(), projectID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if pblFinishedStatuses[proj.Status] {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	var req struct {
		Seconds int32 `json:"seconds"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteError(w, r, httpx.ErrBadJSON(err))
		return
	}
	if err := a.recordHeartbeat(r.Context(), atomID, req.Seconds, time.Now()); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if _, terr := a.d.Queries.TouchAtom(r.Context(), atomID); terr != nil {
		slog.Warn("pbl heartbeat: touch last_activity_at failed",
			"err", terr, "atom_id", atomID, "request_id", httpx.RequestIDFromContext(r.Context()))
	}
	w.WriteHeader(http.StatusNoContent)
}
```

In `api.go`, next to the other `pbl/projects/{id}` routes:

```go
	mux.Handle("POST /api/v1/pbl/projects/{id}/heartbeat", liteOnly(a.postPblHeartbeat))
```

- [ ] **Step 4: Run tests**

Run: `cd apps/api && CGO_ENABLED=0 go test ./internal/api -run TestPblHeartbeat -timeout 1800s`
Expected: PASS

- [ ] **Step 5: Frontend wiring**

`apps/lite-web/src/api/reports.ts`: find `sendHeartbeat`. Keep `AtomKind` as is (reports only exist for reading/writing) and add a separate type:

```ts
export type HeartbeatKind = AtomKind | "project";

const heartbeatBase: Record<HeartbeatKind, string> = {
  reading: "/api/v1/readings",
  writing: "/api/v1/writings",
  project: "/api/v1/pbl/projects",
};
```

Change `sendHeartbeat`'s signature to `(kind: HeartbeatKind, id: string, seconds: number)` and build its URL as `` `${heartbeatBase[kind]}/${encodeURIComponent(id)}/heartbeat` `` (keep its existing body and method).

`useHeartbeat.ts`: import `HeartbeatKind` instead of `AtomKind`, and type the `kind` parameter as `HeartbeatKind`. Update the doc comment's route list to include `/api/v1/pbl/projects/{id}/heartbeat`.

`ProjectRoom.tsx`: import `useHeartbeat` from `"../shared/useHeartbeat"` and, after the `project` state is declared, add:

```ts
  // 项目室的时长。只在项目载入且还没进入回顾/保留/归档时计时，与阅读、写作同一规则。
  useHeartbeat(
    "project",
    projectId,
    project !== null && !["review", "keeping", "archived"].includes(project.status),
  );
```

Run: `pnpm --filter lite-web typecheck && pnpm --filter lite-web test`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add apps/api/internal/api/pbl_heartbeat.go apps/api/internal/api/pbl_heartbeat_test.go apps/api/internal/api/api.go apps/lite-web/src/api/reports.ts apps/lite-web/src/shared/useHeartbeat.ts apps/lite-web/src/projects/ProjectRoom.tsx
git commit -m "feat(lite): 项目室计时心跳"
```

---

### Task 3: Teacher route group + class roster endpoint

**Files:**
- Create: `apps/api/internal/store/queries/lite_teacher.sql`
- Create: `apps/api/internal/api/lite_teacher_routes.go`, `apps/api/internal/api/lite_teacher_roster.go`, `apps/api/internal/api/lite_teacher_roster_test.go`
- Modify: `apps/api/internal/api/api.go` (one line: `a.registerLiteTeacherRoutes(mux)` just before `return SessionAuth(...)`)

**Interfaces:**
- Consumes: `assertTeacherOwnsClass`, `RequireRole`, `requireEdition`, `liteweek.WeekStart`.
- Produces:
  - `(*API).registerLiteTeacherRoutes(mux *http.ServeMux)` and inside it `liteTeacher := func(h http.HandlerFunc) http.Handler` = edition lite + role teacher/admin. Later tasks add routes inside this function.
  - `GET /api/v1/lite/teacher/classes/{id}/roster` → `{"roster": []LiteRosterRowDTO}`.
  - `type LiteRosterRowDTO struct { ID, DisplayName, AvatarColor string; LastActiveAt *string; ActiveDaysThisWeek, MinutesTotal, MinutesThisWeek, Turns, ReadingsDone, ReadingsTotal, WritingsDone, WritingsTotal, ProjectsDone, ProjectsTotal int32 }` with JSON camelCase tags (`id`, `displayName`, `avatarColor`, `lastActiveAt`, `activeDaysThisWeek`, `minutesTotal`, `minutesThisWeek`, `turns`, `readingsDone`, `readingsTotal`, `writingsDone`, `writingsTotal`, `projectsDone`, `projectsTotal`). `minutesThisWeek` is `-1` when the student has no bucket at all yet (UI shows "—").
  - sqlc `ListLiteClassRoster(ctx, ListLiteClassRosterParams{ClassID uuid.UUID, WeekStart time.Time, WeekEnd time.Time, WeekStartDay pgtype.Date, WeekEndDay pgtype.Date})`.

- [ ] **Step 1: Write failing tests**

```go
package api_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	. "mindimprint/api/internal/api"
	"mindimprint/api/internal/cards"
	"mindimprint/api/internal/store/sqlc"
)

type liteRosterRow struct {
	ID              string `json:"id"`
	MinutesTotal    int32  `json:"minutesTotal"`
	MinutesThisWeek int32  `json:"minutesThisWeek"`
	Turns           int32  `json:"turns"`
	ReadingsDone    int32  `json:"readingsDone"`
	ReadingsTotal   int32  `json:"readingsTotal"`
	WritingsTotal   int32  `json:"writingsTotal"`
}

// liteTeacherFixture: a lite school, a teacher who owns a class, one enrolled student.
func liteTeacherFixture(t *testing.T) (h http.Handler, pool *pgxpool.Pool, teacher *http.Cookie, classID string, studentID uuid.UUID) {
	t.Helper()
	pool = newAPITestPool(t)
	h = New(Deps{Queries: sqlc.New(pool), Pool: pool, ChatResolver: fakeResolver(), EvalResolver: fakeEvalResolver(), SpecByID: cards.ByID}).Handler()
	if _, err := pool.Exec(context.Background(), `UPDATE schools SET edition = 'lite'`); err != nil {
		t.Fatal(err)
	}
	teacher = signInAs(t, pool, createTeacher(t, pool, SeedSchoolID, "lt-teacher@demo.local"))
	classID = createClassViaAPI(t, h, teacher, "Lite Class")
	studentID = createStudent(t, pool, SeedSchoolID, "lt-student@demo.local")
	enrollStudent(t, pool, studentID, classID)
	return
}

func getJSON(t *testing.T, h http.Handler, c *http.Cookie, path string, out any) int {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("GET", path, nil), c))
	if out != nil && rec.Code == http.StatusOK {
		if err := json.Unmarshal(rec.Body.Bytes(), out); err != nil {
			t.Fatalf("decode %s: %v body=%s", path, err, rec.Body)
		}
	}
	return rec.Code
}

func TestLiteRosterCountsLiteActivity(t *testing.T) {
	h, pool, teacher, classID, studentID := liteTeacherFixture(t)
	ctx := context.Background()
	// One finished reading with 600s total, 120s bucket today, two student turns + one ai turn.
	readingAtom := seedLiteReadingForUser(t, pool, studentID, "finished", 600)
	if _, err := pool.Exec(ctx, `INSERT INTO atom_active_day (atom_id, day, seconds) VALUES ($1, (now() AT TIME ZONE 'Asia/Shanghai')::date, 120)`, readingAtom); err != nil {
		t.Fatal(err)
	}
	for _, role := range []string{"student", "ai", "student"} {
		seedAtomMessage(t, pool, readingAtom, role, "hello")
	}
	seedLiteWritingForUser(t, pool, studentID, "active")

	var resp struct{ Roster []liteRosterRow `json:"roster"` }
	if code := getJSON(t, h, teacher, "/api/v1/lite/teacher/classes/"+classID+"/roster", &resp); code != http.StatusOK {
		t.Fatalf("roster = %d", code)
	}
	if len(resp.Roster) != 1 {
		t.Fatalf("rows = %d", len(resp.Roster))
	}
	r := resp.Roster[0]
	if r.MinutesTotal != 10 || r.MinutesThisWeek != 2 || r.Turns != 2 || r.ReadingsDone != 1 || r.ReadingsTotal != 1 || r.WritingsTotal != 1 {
		t.Fatalf("row = %+v", r)
	}
}

func TestLiteRosterNoBucketsIsMinusOne(t *testing.T) {
	h, _, teacher, classID, _ := liteTeacherFixture(t)
	var resp struct{ Roster []liteRosterRow `json:"roster"` }
	getJSON(t, h, teacher, "/api/v1/lite/teacher/classes/"+classID+"/roster", &resp)
	if resp.Roster[0].MinutesThisWeek != -1 {
		t.Fatalf("minutesThisWeek = %d, want -1", resp.Roster[0].MinutesThisWeek)
	}
}

func TestLiteRosterAuthz(t *testing.T) {
	h, pool, _, classID, studentID := liteTeacherFixture(t)
	other := signInAs(t, pool, createTeacher(t, pool, SeedSchoolID, "lt-other@demo.local"))
	if code := getJSON(t, h, other, "/api/v1/lite/teacher/classes/"+classID+"/roster", nil); code != http.StatusNotFound {
		t.Fatalf("other teacher = %d, want 404", code)
	}
	student := signInAs(t, pool, studentID)
	if code := getJSON(t, h, student, "/api/v1/lite/teacher/classes/"+classID+"/roster", nil); code != http.StatusForbidden {
		t.Fatalf("student = %d, want 403", code)
	}
}

func TestLiteRosterProSchool404(t *testing.T) {
	h, pool, teacher, classID, _ := liteTeacherFixture(t)
	if _, err := pool.Exec(context.Background(), `UPDATE schools SET edition = 'pro'`); err != nil {
		t.Fatal(err)
	}
	if code := getJSON(t, h, teacher, "/api/v1/lite/teacher/classes/"+classID+"/roster", nil); code != http.StatusNotFound {
		t.Fatalf("pro school = %d, want 404", code)
	}
}
```

Also in this test file, write seed helpers used by Tasks 3–5 (read `queries/atom.sql`, `reading.sql`, `writing_atom.sql` for exact sqlc params):
- `seedLiteReadingForUser(t, pool, userID uuid.UUID, status string, activeSeconds int32) uuid.UUID` — `CreateAtom` kind `reading`, `CreateReading` title "Test reading", set `status`/`finished_at` via SQL when `finished`, set `active_seconds` via SQL.
- `seedLiteWritingForUser(t, pool, userID uuid.UUID, status string) uuid.UUID` — same for writing.
- `seedAtomMessage(t, pool, atomID uuid.UUID, role, content string)` — `INSERT INTO atom_message (atom_id, seq, role, content) VALUES ($1, (SELECT COALESCE(max(seq),0)+1 FROM atom_message WHERE atom_id=$1), $2, $3)`; check the table's NOT NULL columns in its migration first.

Note: the SQL in the test uses `Asia/Shanghai` only inside Postgres (which has tzdata); Go code never does.

- [ ] **Step 2: Run to verify failure**

Run: `cd apps/api && CGO_ENABLED=0 go test ./internal/api -run TestLiteRoster -timeout 1800s`
Expected: FAIL — roster route 404 for the owning teacher.

- [ ] **Step 3: Queries**

`ls apps/api/internal/store/queries/lite_teacher.sql` must fail. Create:

```sql
-- 教师端（lite）读学生数据。只读；只数、只取产出，不取对话正文。

-- name: ListLiteClassRoster :many
SELECT u.id, u.display_name, u.avatar_color,
       (SELECT max(a.last_activity_at) FROM atom a WHERE a.user_id = u.id)::timestamptz AS last_active_at,
       COALESCE((SELECT sum(a.active_seconds) FROM atom a WHERE a.user_id = u.id), 0)::int AS seconds_total,
       (SELECT sum(d.seconds) FROM atom_active_day d JOIN atom a ON a.id = d.atom_id
         WHERE a.user_id = u.id AND d.day >= sqlc.arg(week_start_day)::date AND d.day < sqlc.arg(week_end_day)::date)::int AS seconds_this_week,
       (SELECT count(*) FROM atom_active_day d JOIN atom a ON a.id = d.atom_id WHERE a.user_id = u.id)::int AS bucket_count,
       (SELECT count(DISTINCT x.day) FROM (
          SELECT d.day FROM atom_active_day d JOIN atom a ON a.id = d.atom_id
           WHERE a.user_id = u.id AND d.seconds > 0 AND d.day >= sqlc.arg(week_start_day)::date AND d.day < sqlc.arg(week_end_day)::date
          UNION
          SELECT (m.created_at AT TIME ZONE 'Asia/Shanghai')::date FROM atom_message m JOIN atom a ON a.id = m.atom_id
           WHERE a.user_id = u.id AND m.role = 'student' AND m.created_at >= sqlc.arg(week_start)::timestamptz AND m.created_at < sqlc.arg(week_end)::timestamptz
        ) x)::int AS active_days_this_week,
       (SELECT count(*) FROM atom_message m JOIN atom a ON a.id = m.atom_id WHERE a.user_id = u.id AND m.role = 'student')::int AS turns,
       (SELECT count(*) FROM reading r JOIN atom a ON a.id = r.atom_id WHERE a.user_id = u.id AND r.status = 'finished')::int AS readings_done,
       (SELECT count(*) FROM atom a WHERE a.user_id = u.id AND a.kind = 'reading')::int AS readings_total,
       (SELECT count(*) FROM writing w JOIN atom a ON a.id = w.atom_id WHERE a.user_id = u.id AND w.status = 'finished')::int AS writings_done,
       (SELECT count(*) FROM atom a WHERE a.user_id = u.id AND a.kind = 'writing')::int AS writings_total,
       (SELECT count(*) FROM pbl_project p JOIN atom a ON a.id = p.atom_id WHERE a.user_id = u.id AND p.status IN ('review','keeping','archived'))::int AS projects_done,
       (SELECT count(*) FROM atom a WHERE a.user_id = u.id AND a.kind = 'project')::int AS projects_total
FROM enrollments e
JOIN users u ON u.id = e.user_id
WHERE e.class_id = sqlc.arg(class_id) AND e.role_in_class = 'student'
ORDER BY u.display_name;
```

`AT TIME ZONE 'Asia/Shanghai'` is correct inside Postgres (it ships tzdata; the no-tzdata constraint is only about the Go binary). Never write `AT TIME ZONE '+08'`: Postgres reads POSIX offsets with the sign inverted, so that means UTC−8.

Run: `cd apps/api && make sqlc && go build ./...`

- [ ] **Step 4: Handler + route group**

`lite_teacher_routes.go`:

```go
package api

import "net/http"

// registerLiteTeacherRoutes mounts the lite teacher end. Every route is lite
// edition AND teacher/admin role; student-scoped routes additionally go
// through authTeacherStudent inside the handler.
func (a *API) registerLiteTeacherRoutes(mux *http.ServeMux) {
	liteTeacher := func(h http.HandlerFunc) http.Handler {
		return a.requireEdition("lite", RequireRole("teacher", "admin")(h))
	}
	mux.Handle("GET /api/v1/lite/teacher/classes/{id}/roster", liteTeacher(a.getLiteClassRoster))
}
```

`requireEdition` wraps in `RequireUser`, so an unauthenticated call is 401 and a student is 403 (role check runs after the edition check; a pro-school teacher gets 404 from the edition check first).

`lite_teacher_roster.go`:

```go
package api

import (
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"mindimprint/api/internal/httpx"
	"mindimprint/api/internal/liteweek"
	"mindimprint/api/internal/store/sqlc"
)

type LiteRosterRowDTO struct {
	ID                 string  `json:"id"`
	DisplayName        string  `json:"displayName"`
	AvatarColor        string  `json:"avatarColor"`
	LastActiveAt       *string `json:"lastActiveAt"`
	ActiveDaysThisWeek int32   `json:"activeDaysThisWeek"`
	MinutesTotal       int32   `json:"minutesTotal"`
	MinutesThisWeek    int32   `json:"minutesThisWeek"` // -1 = no time buckets recorded yet
	Turns              int32   `json:"turns"`
	ReadingsDone       int32   `json:"readingsDone"`
	ReadingsTotal      int32   `json:"readingsTotal"`
	WritingsDone       int32   `json:"writingsDone"`
	WritingsTotal      int32   `json:"writingsTotal"`
	ProjectsDone       int32   `json:"projectsDone"`
	ProjectsTotal      int32   `json:"projectsTotal"`
}

// currentLiteWeek is [Monday 00:00 Beijing, next Monday) around now.
func currentLiteWeek(now time.Time) (start, end time.Time) {
	start = liteweek.WeekStart(now)
	return start, start.AddDate(0, 0, 7)
}

func pgDate(t time.Time) pgtype.Date { return pgtype.Date{Time: t, Valid: true} }

func secondsToMinutes(s int32) int32 { return (s + 30) / 60 }

// getLiteClassRoster handles GET /api/v1/lite/teacher/classes/{id}/roster.
func (a *API) getLiteClassRoster(w http.ResponseWriter, r *http.Request) {
	classID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		httpx.WriteError(w, r, httpx.ErrNotFound("资源不存在"))
		return
	}
	if _, err := a.assertTeacherOwnsClass(r.Context(), classID); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	start, end := currentLiteWeek(time.Now())
	rows, err := a.d.Queries.ListLiteClassRoster(r.Context(), sqlc.ListLiteClassRosterParams{
		ClassID: classID, WeekStart: start, WeekEnd: end,
		WeekStartDay: pgDate(start), WeekEndDay: pgDate(end),
	})
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	out := make([]LiteRosterRowDTO, 0, len(rows))
	for _, row := range rows {
		out = append(out, liteRosterRow(row))
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"roster": out})
}

func liteRosterRow(row sqlc.ListLiteClassRosterRow) LiteRosterRowDTO {
	dto := LiteRosterRowDTO{
		ID: row.ID.String(), DisplayName: row.DisplayName, AvatarColor: row.AvatarColor,
		ActiveDaysThisWeek: row.ActiveDaysThisWeek,
		MinutesTotal:       secondsToMinutes(row.SecondsTotal),
		MinutesThisWeek:    -1,
		Turns:              row.Turns,
		ReadingsDone: row.ReadingsDone, ReadingsTotal: row.ReadingsTotal,
		WritingsDone: row.WritingsDone, WritingsTotal: row.WritingsTotal,
		ProjectsDone: row.ProjectsDone, ProjectsTotal: row.ProjectsTotal,
	}
	if row.BucketCount > 0 {
		dto.MinutesThisWeek = 0
		if row.SecondsThisWeek != nil {
			dto.MinutesThisWeek = secondsToMinutes(*row.SecondsThisWeek)
		}
	}
	if row.LastActiveAt != nil {
		s := row.LastActiveAt.Format(time.RFC3339)
		dto.LastActiveAt = &s
	}
	return dto
}
```

Field types of the generated row (`*int32` vs `int32`, `*time.Time`) depend on sqlc's nullability inference — open `internal/store/sqlc/lite_teacher.sql.go` and adjust the nil checks to match. Keep the `MinutesThisWeek = -1` rule.

In `api.go`, before `return SessionAuth(a.d.Queries)(mux)`: `a.registerLiteTeacherRoutes(mux)`.

- [ ] **Step 5: Run tests**

Run: `cd apps/api && CGO_ENABLED=0 go test ./internal/api -run TestLiteRoster -timeout 1800s`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add apps/api/internal/store/queries/lite_teacher.sql apps/api/internal/store/sqlc/ apps/api/internal/api/lite_teacher_routes.go apps/api/internal/api/lite_teacher_roster.go apps/api/internal/api/lite_teacher_roster_test.go apps/api/internal/api/api.go
git commit -m "feat(lite-teacher): 班级学生名单与实时统计"
```

---

### Task 4: Student page endpoint

**Files:**
- Modify: `apps/api/internal/store/queries/lite_teacher.sql`
- Modify: `apps/api/internal/api/lite_teacher_roster.go`, `lite_teacher_routes.go`, `lite_teacher_roster_test.go`

**Interfaces:**
- Consumes: `authTeacherStudent(w, r) (classID, userID uuid.UUID, ok bool)`, `LiteRosterRowDTO`, seed helpers from Task 3.
- Produces:
  - `GET /api/v1/lite/teacher/classes/{id}/students/{userId}` → `{"student": LiteRosterRowDTO, "items": []LiteItemRowDTO}`.
  - `type LiteItemRowDTO struct { AtomID, Kind, Title, Status string; Level *int32; Minutes int32; Turns int32; CreatedAt, LastActiveAt string; FinishedAt *string }` — JSON `atomId, kind, title, status, level, minutes, turns, createdAt, lastActiveAt, finishedAt`. `kind ∈ reading|writing|project`. `status` for reading/writing is the table's `status` (`active`/`finished`); for projects it is `pbl_project.status`. `minutes` is `-1` when `active_seconds = 0`.
  - sqlc `ListLiteStudentItems(ctx, userID uuid.UUID)`; sqlc `GetLiteStudentRosterRow(ctx, GetLiteStudentRosterRowParams{UserID, WeekStart, WeekEnd, WeekStartDay, WeekEndDay})` — same select list as `ListLiteClassRoster` for one user.

- [ ] **Step 1: Failing tests** (append to `lite_teacher_roster_test.go`)

```go
type liteItemRow struct {
	AtomID  string `json:"atomId"`
	Kind    string `json:"kind"`
	Status  string `json:"status"`
	Minutes int32  `json:"minutes"`
	Turns   int32  `json:"turns"`
}

func TestLiteStudentPageListsItems(t *testing.T) {
	h, pool, teacher, classID, studentID := liteTeacherFixture(t)
	r := seedLiteReadingForUser(t, pool, studentID, "finished", 300)
	seedAtomMessage(t, pool, r, "student", "x")
	seedLiteWritingForUser(t, pool, studentID, "active")

	var resp struct {
		Student liteRosterRow `json:"student"`
		Items   []liteItemRow `json:"items"`
	}
	if code := getJSON(t, h, teacher, "/api/v1/lite/teacher/classes/"+classID+"/students/"+studentID.String(), &resp); code != http.StatusOK {
		t.Fatalf("student page = %d", code)
	}
	if resp.Student.ID != studentID.String() || len(resp.Items) != 2 {
		t.Fatalf("resp = %+v", resp)
	}
	var reading *liteItemRow
	for i := range resp.Items {
		if resp.Items[i].AtomID == r.String() {
			reading = &resp.Items[i]
		}
	}
	if reading == nil || reading.Kind != "reading" || reading.Status != "finished" || reading.Minutes != 5 || reading.Turns != 1 {
		t.Fatalf("reading row = %+v", reading)
	}
}

func TestLiteStudentPageStudentOfOtherClass404(t *testing.T) {
	h, pool, teacher, classID, _ := liteTeacherFixture(t)
	stranger := createStudent(t, pool, SeedSchoolID, "lt-stranger@demo.local")
	if code := getJSON(t, h, teacher, "/api/v1/lite/teacher/classes/"+classID+"/students/"+stranger.String(), nil); code != http.StatusNotFound {
		t.Fatalf("stranger = %d, want 404", code)
	}
}
```

- [ ] **Step 2: Run to verify failure**

Run: `cd apps/api && CGO_ENABLED=0 go test ./internal/api -run TestLiteStudentPage -timeout 1800s`
Expected: FAIL (404 route).

- [ ] **Step 3: Queries** (append to `lite_teacher.sql`)

Copy `ListLiteClassRoster`'s select list into `GetLiteStudentRosterRow :one` with `FROM users u WHERE u.id = sqlc.arg(user_id)` (no enrollments join).

```sql
-- name: ListLiteStudentItems :many
SELECT a.id AS atom_id, a.kind, a.created_at, a.last_activity_at, a.active_seconds,
       COALESCE(r.title, w.title, NULLIF(p.name, ''), p.idea, '')::text AS title,
       COALESCE(r.status, w.status, p.status, '')::text AS status,
       r.library_tier AS level,
       COALESCE(r.finished_at, w.finished_at) AS finished_at,
       (SELECT count(*) FROM atom_message m WHERE m.atom_id = a.id AND m.role = 'student')::int AS turns
FROM atom a
LEFT JOIN reading r ON r.atom_id = a.id
LEFT JOIN writing w ON w.atom_id = a.id
LEFT JOIN pbl_project p ON p.atom_id = a.id
WHERE a.user_id = $1 AND a.kind IN ('reading','writing','project')
ORDER BY a.last_activity_at DESC;
```

Check column names against the migrations (`reading.library_tier`, `pbl_project.name`/`idea`) before running sqlc. Run: `cd apps/api && make sqlc && go build ./...`

- [ ] **Step 4: Handler** (append to `lite_teacher_roster.go`; register `mux.Handle("GET /api/v1/lite/teacher/classes/{id}/students/{userId}", liteTeacher(a.getLiteStudentPage))`)

```go
type LiteItemRowDTO struct {
	AtomID       string  `json:"atomId"`
	Kind         string  `json:"kind"`
	Title        string  `json:"title"`
	Status       string  `json:"status"`
	Level        *int32  `json:"level"`
	Minutes      int32   `json:"minutes"` // -1 = no time recorded
	Turns        int32   `json:"turns"`
	CreatedAt    string  `json:"createdAt"`
	LastActiveAt string  `json:"lastActiveAt"`
	FinishedAt   *string `json:"finishedAt"`
}

// getLiteStudentPage handles GET /api/v1/lite/teacher/classes/{id}/students/{userId}.
func (a *API) getLiteStudentPage(w http.ResponseWriter, r *http.Request) {
	_, userID, ok := a.authTeacherStudent(w, r)
	if !ok {
		return
	}
	ctx := r.Context()
	start, end := currentLiteWeek(time.Now())
	head, err := a.d.Queries.GetLiteStudentRosterRow(ctx, sqlc.GetLiteStudentRosterRowParams{
		UserID: userID, WeekStart: start, WeekEnd: end, WeekStartDay: pgDate(start), WeekEndDay: pgDate(end),
	})
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	rows, err := a.d.Queries.ListLiteStudentItems(ctx, userID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	items := make([]LiteItemRowDTO, 0, len(rows))
	for _, it := range rows {
		dto := LiteItemRowDTO{
			AtomID: it.AtomID.String(), Kind: it.Kind, Title: it.Title, Status: it.Status,
			Level: it.Level, Minutes: -1, Turns: it.Turns,
			CreatedAt: it.CreatedAt.Format(time.RFC3339), LastActiveAt: it.LastActivityAt.Format(time.RFC3339),
		}
		if it.ActiveSeconds > 0 {
			dto.Minutes = secondsToMinutes(it.ActiveSeconds)
		}
		if it.FinishedAt != nil {
			s := it.FinishedAt.Format(time.RFC3339)
			dto.FinishedAt = &s
		}
		items = append(items, dto)
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{
		"student": liteRosterRow(sqlc.ListLiteClassRosterRow(head)),
		"items":   items,
	})
}
```

`sqlc.ListLiteClassRosterRow(head)` compiles only if both generated row structs have identical fields in identical order; if they differ, write a small `liteRosterRowFromHead(head sqlc.GetLiteStudentRosterRowRow) LiteRosterRowDTO` instead. Match nullable field types to the generated code.

- [ ] **Step 5: Run tests**

Run: `cd apps/api && CGO_ENABLED=0 go test ./internal/api -run 'TestLiteStudentPage|TestLiteRoster' -timeout 1800s`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add apps/api/internal/store/queries/lite_teacher.sql apps/api/internal/store/sqlc/ apps/api/internal/api/lite_teacher_roster.go apps/api/internal/api/lite_teacher_routes.go apps/api/internal/api/lite_teacher_roster_test.go
git commit -m "feat(lite-teacher): 学生页 —— 阅读、写作、项目列表"
```

---

### Task 5: Item detail endpoint (outputs and moments, never the chat)

**Files:**
- Create: `apps/api/internal/api/lite_teacher_item.go`, `apps/api/internal/api/lite_teacher_item_test.go`
- Modify: `apps/api/internal/api/lite_teacher_routes.go`

**Interfaces:**
- Consumes: `authTeacherStudent`, `ensureAtomReport(ctx, userID, atomID, kind) (sqlc.AtomReport, bool, error)` (reading/writing only), `detachedModelCtx` (see `reading_lens.go`), existing sqlc queries: `GetAtom`, `GetReading`, `GetReadingSource`, `GetReadingTakeaway`, `ListAtomAnnotations`, `ListAtomCards`, `GetWriting`, `ListWritingOutline`, `ListWritingSnippets`, `GetWritingDraft`, `ListWritingComments`, `GetPblProject` (by project id — for items we have the atom id, so add `GetPblProjectByAtom` if no such query exists), `GetPblLivePlan`, `ListPblPlanSteps`, `ListPblTools`, `ListPblArtifacts`, `ListPblKeepEntries`, `ListPblCourseAssignments`, `GetPblSite`.
- Produces: `GET /api/v1/lite/teacher/classes/{id}/students/{userId}/items/{atomId}` →

```json
{
  "item": { /* LiteItemRowDTO fields */ },
  "reading": { "source": {"librarySlug": "…", "level": 3, "url": "…"}, "highlights": [{"quote":"…","note":"…"}], "takeaway": "…", "lenses": [{"title":"…","fields":{…}}] } | null,
  "writing": { "targetWords": 800, "lang": "zh", "structureKey": "…", "outline": [{"role":"…","text":"…"}], "snippets": [{"position":0,"text":"…"}], "draft": "…", "comments": [{"scope":"draft","summary":"…","points":[…]}] } | null,
  "project": { "idea": "…", "status": "…", "stepsDone": 2, "stepsTotal": 5, "steps": [{"title":"…","status":"done"}], "tools": [{"key":"…","result":{…}}], "artifacts": [{"title":"…","payload":{…}}], "keeps": [{"text":"…"}], "courses": [{"slug":"…","why":"…","takeaway":"…"}], "siteToken": "…" | null } | null,
  "report": { "stats": [{"key":"…","value":0,"unit":"…"}], "moments": [{"quote":"…","where":"…"}], "keep": {"text":"…","source":"student"} | null } | null,
  "reportError": "…" | null
}
```

- [ ] **Step 1: Failing tests**

```go
package api_test

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"net/http/httptest"
)

func TestLiteItemDetailNeverReturnsChat(t *testing.T) {
	h, pool, teacher, classID, studentID := liteTeacherFixture(t)
	atom := seedLiteReadingForUser(t, pool, studentID, "active", 0)
	seedAtomMessage(t, pool, atom, "student", "SECRET-CHAT-LINE-7731")
	seedAtomMessage(t, pool, atom, "ai", "SECRET-AI-LINE-7731")
	if _, err := pool.Exec(context.Background(),
		`INSERT INTO atom_annotation (atom_id, block_id, span, quote, note) VALUES ($1, 'b1', '{}', 'article words', 'her note')`, atom); err != nil {
		t.Fatal(err) // adjust columns to the real atom_annotation schema
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("GET",
		"/api/v1/lite/teacher/classes/"+classID+"/students/"+studentID.String()+"/items/"+atom.String(), nil), teacher))
	if rec.Code != http.StatusOK {
		t.Fatalf("item = %d body=%s", rec.Code, rec.Body)
	}
	body := rec.Body.String()
	if strings.Contains(body, "SECRET-CHAT-LINE-7731") || strings.Contains(body, "SECRET-AI-LINE-7731") {
		t.Fatalf("chat content leaked: %s", body)
	}
	if !strings.Contains(body, "her note") {
		t.Fatalf("highlight note missing: %s", body)
	}
}

func TestLiteItemDetailAtomOfAnotherStudent404(t *testing.T) {
	h, pool, teacher, classID, studentID := liteTeacherFixture(t)
	stranger := createStudent(t, pool, SeedSchoolID, "lt-item-stranger@demo.local")
	atom := seedLiteReadingForUser(t, pool, stranger, "active", 0)
	if code := getJSON(t, h, teacher, "/api/v1/lite/teacher/classes/"+classID+"/students/"+studentID.String()+"/items/"+atom.String(), nil); code != http.StatusNotFound {
		t.Fatalf("foreign atom = %d, want 404", code)
	}
}

func TestLiteItemDetailWritingDraft(t *testing.T) {
	h, pool, teacher, classID, studentID := liteTeacherFixture(t)
	atom := seedLiteWritingForUser(t, pool, studentID, "active")
	if _, err := pool.Exec(context.Background(), `INSERT INTO writing_draft (atom_id, body) VALUES ($1, 'HER-DRAFT-BODY')`, atom); err != nil {
		t.Fatal(err) // adjust to the real writing_draft columns
	}
	var resp struct {
		Writing struct {
			Draft string `json:"draft"`
		} `json:"writing"`
	}
	if code := getJSON(t, h, teacher, "/api/v1/lite/teacher/classes/"+classID+"/students/"+studentID.String()+"/items/"+atom.String(), &resp); code != http.StatusOK {
		t.Fatalf("item = %d", code)
	}
	if resp.Writing.Draft != "HER-DRAFT-BODY" {
		t.Fatalf("draft = %q", resp.Writing.Draft)
	}
}

func TestLiteItemDetailDoesNotTouchActivity(t *testing.T) {
	h, pool, teacher, classID, studentID := liteTeacherFixture(t)
	atom := seedLiteReadingForUser(t, pool, studentID, "active", 0)
	var before, after string
	_ = pool.QueryRow(context.Background(), `SELECT last_activity_at::text FROM atom WHERE id=$1`, atom).Scan(&before)
	getJSON(t, h, teacher, "/api/v1/lite/teacher/classes/"+classID+"/students/"+studentID.String()+"/items/"+atom.String(), nil)
	_ = pool.QueryRow(context.Background(), `SELECT last_activity_at::text FROM atom WHERE id=$1`, atom).Scan(&after)
	if before != after {
		t.Fatalf("teacher read bumped last_activity_at %s → %s", before, after)
	}
}
```

- [ ] **Step 2: Run to verify failure**

Run: `cd apps/api && CGO_ENABLED=0 go test ./internal/api -run TestLiteItemDetail -timeout 1800s`
Expected: FAIL (route 404).

- [ ] **Step 3: Implement**

`lite_teacher_item.go`. Structure (fill each builder from the listed sqlc queries; read the generated structs for field names):

```go
package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"mindimprint/api/internal/httpx"
	"mindimprint/api/internal/store/sqlc"
)

// getLiteTeacherItem handles GET /api/v1/lite/teacher/classes/{id}/students/{userId}/items/{atomId}.
//
// Outputs and moments only (spec DEC-1). This file must never read
// atom_message.content: the chat with 印记 is not shown to teachers. Turn
// counts come from the item row query, which counts rows without content.
func (a *API) getLiteTeacherItem(w http.ResponseWriter, r *http.Request) {
	_, userID, ok := a.authTeacherStudent(w, r)
	if !ok {
		return
	}
	atomID, err := uuid.Parse(r.PathValue("atomId"))
	if err != nil {
		httpx.WriteError(w, r, httpx.ErrNotFound("资源不存在"))
		return
	}
	ctx := r.Context()
	at, err := a.d.Queries.GetAtom(ctx, atomID)
	if err != nil || at.UserID != userID {
		httpx.WriteError(w, r, httpx.ErrNotFound("资源不存在"))
		return
	}
	resp := map[string]any{"reading": nil, "writing": nil, "project": nil, "report": nil, "reportError": nil}
	row, err := a.liteTeacherItemRow(ctx, userID, atomID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	resp["item"] = row

	switch at.Kind {
	case "reading":
		v, err := a.liteTeacherReading(ctx, atomID)
		if err != nil {
			httpx.WriteError(w, r, err)
			return
		}
		resp["reading"] = v
	case "writing":
		v, err := a.liteTeacherWriting(ctx, atomID)
		if err != nil {
			httpx.WriteError(w, r, err)
			return
		}
		resp["writing"] = v
	case "project":
		v, err := a.liteTeacherProject(ctx, atomID)
		if err != nil {
			httpx.WriteError(w, r, err)
			return
		}
		resp["project"] = v
	default:
		httpx.WriteError(w, r, httpx.ErrNotFound("资源不存在"))
		return
	}

	if at.Kind == "reading" || at.Kind == "writing" {
		rep, rerr := a.liteTeacherReport(ctx, userID, atomID, at.Kind)
		if rerr != nil {
			msg := "报告生成失败：" + rerr.Error()
			resp["reportError"] = msg
		} else if rep != nil {
			resp["report"] = rep
		}
	}
	httpx.WriteJSON(w, http.StatusOK, resp)
}

// liteTeacherReport returns the stored report's teacher-facing slice
// (stats, moments, keep), generating it first when the item is finished and
// no report exists yet (spec DEC-14). Unfinished → (nil, nil).
func (a *API) liteTeacherReport(ctx context.Context, userID, atomID uuid.UUID, kind string) (map[string]any, error) {
	mctx, cancel := detachedModelCtx(ctx)
	defer cancel()
	row, ok, err := a.ensureAtomReport(mctx, userID, atomID, kind)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, nil
	}
	var rep struct {
		Stats   json.RawMessage `json:"stats"`
		Moments json.RawMessage `json:"moments"`
		Keep    json.RawMessage `json:"keep"`
	}
	if err := json.Unmarshal(row.Report, &rep); err != nil {
		return nil, err
	}
	return map[string]any{"stats": rep.Stats, "moments": rep.Moments, "keep": rep.Keep}, nil
}

func notFoundIsNil[T any](v T, err error) (*T, error) {
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &v, nil
}
```

Check `detachedModelCtx`'s real signature in `reading_lens.go` and adapt (it may return only a context). Also write, in the same file:
- `liteTeacherItemRow(ctx, userID, atomID) (LiteItemRowDTO, error)` — add `GetLiteStudentItem :one` to `lite_teacher.sql` (the `ListLiteStudentItems` select with `AND a.id = $2`), map with the same code as Task 4 (extract that mapping into `liteItemRowDTO(row)` shared by both; since the two generated row types differ, give the mapper the fields it needs as arguments).
- `liteTeacherReading(ctx, atomID) (map[string]any, error)` — `GetReading` (library_slug, library_tier), `GetReadingSource` via `notFoundIsNil` (only `url`, never `body`), `ListAtomAnnotations` → `[{quote, note}]`, `GetReadingTakeaway` via `notFoundIsNil` → text, `ListAtomCards` filtered to `status='submitted'` → `[{cardId, fields: field_values}]`.
- `liteTeacherWriting(ctx, atomID) (map[string]any, error)` — `GetWriting` (target_words, lang, structure_key), `ListWritingOutline` → `[{role, text}]`, `ListWritingSnippets` → `[{position, text}]`, `GetWritingDraft` via `notFoundIsNil` → body, `ListWritingComments` → `[{scope, summary, points}]`.
- `liteTeacherProject(ctx, atomID) (map[string]any, error)` — project row by atom id, live plan steps (`GetPblLivePlan` via `notFoundIsNil`, then `ListPblPlanSteps(version.ID)`), `ListPblTools` → `[{key, status, result}]`, `ListPblArtifacts`, `ListPblKeepEntries`, `ListPblCourseAssignments` → `[{slug, why, takeaway, finishedAt}]`, and if the project is the student's website project, `GetPblSite` share token when published. Read each query's params (most take `atom_id`).

Register: `mux.Handle("GET /api/v1/lite/teacher/classes/{id}/students/{userId}/items/{atomId}", liteTeacher(a.getLiteTeacherItem))`.

- [ ] **Step 4: Run tests**

Run: `cd apps/api && CGO_ENABLED=0 go test ./internal/api -run TestLiteItemDetail -timeout 1800s`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add apps/api/internal/api/lite_teacher_item.go apps/api/internal/api/lite_teacher_item_test.go apps/api/internal/api/lite_teacher_routes.go apps/api/internal/store/queries/lite_teacher.sql apps/api/internal/store/sqlc/
git commit -m "feat(lite-teacher): 单项详情 —— 产出与金句，不含对话"
```

---

### Task 6: Student interest tree for teachers

**Files:**
- Modify: `apps/api/internal/api/interest.go`
- Create: `apps/api/internal/api/lite_teacher_tree.go`, `apps/api/internal/api/lite_teacher_tree_test.go`
- Modify: `apps/api/internal/api/lite_teacher_routes.go`

**Interfaces:**
- Produces: `(*API).buildInterestTree(ctx context.Context, userID uuid.UUID) (map[string]any, error)` returning exactly the map `getInterestTree` writes today (`fields`, `keywords`). `GET /api/v1/lite/teacher/classes/{id}/students/{userId}/tree` → same JSON as `GET /api/v1/interest/tree` for that student.

- [ ] **Step 1: Failing test**

```go
package api_test

import (
	"context"
	"net/http"
	"testing"
)

func TestLiteTeacherTreeShowsStudentsKeywords(t *testing.T) {
	h, pool, teacher, classID, studentID := liteTeacherFixture(t)
	if _, err := pool.Exec(context.Background(),
		`INSERT INTO interest_keyword (user_id, text_zh, text_en, field, strength, note) VALUES ($1, '金融', 'finance', 'society', 3, '')`, studentID); err != nil {
		t.Fatal(err) // adjust columns/field id to the real interest_keyword schema and disciplines.Fields
	}
	var resp struct {
		Keywords []struct {
			TextZh string `json:"textZh"`
		} `json:"keywords"`
		Fields []struct{ ID string `json:"id"` } `json:"fields"`
	}
	if code := getJSON(t, h, teacher, "/api/v1/lite/teacher/classes/"+classID+"/students/"+studentID.String()+"/tree", &resp); code != http.StatusOK {
		t.Fatalf("tree = %d", code)
	}
	if len(resp.Keywords) != 1 || resp.Keywords[0].TextZh != "金融" || len(resp.Fields) == 0 {
		t.Fatalf("tree = %+v", resp)
	}
}
```

- [ ] **Step 2: Run to verify failure**

Run: `cd apps/api && CGO_ENABLED=0 go test ./internal/api -run TestLiteTeacherTree -timeout 1800s`
Expected: FAIL (404).

- [ ] **Step 3: Refactor + handler**

In `interest.go`, move the body of `getInterestTree` after the `UserFromContext` check into:

```go
// buildInterestTree is the tree payload for one user. Shared by the student's
// own GET /interest/tree and the teacher's read-only view of a student.
func (a *API) buildInterestTree(ctx context.Context, userID uuid.UUID) (map[string]any, error) {
	// …the existing body, with u.ID → userID, and every
	// `httpx.WriteError(w, r, err); return` → `return nil, err`…
	return map[string]any{"fields": fields, "keywords": out}, nil
}
```

and make `getInterestTree`:

```go
func (a *API) getInterestTree(w http.ResponseWriter, r *http.Request) {
	u, ok := UserFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, r, httpx.ErrUnauthorized("未登录"))
		return
	}
	// **这是一次纯读。** 采集在后台队列里跑（interest_jobs.go）。
	tree, err := a.buildInterestTree(r.Context(), u.ID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, tree)
}
```

Keep the existing Chinese comments in the moved body.

`lite_teacher_tree.go`:

```go
package api

import "net/http"

// getLiteTeacherTree handles GET /api/v1/lite/teacher/classes/{id}/students/{userId}/tree.
func (a *API) getLiteTeacherTree(w http.ResponseWriter, r *http.Request) {
	_, userID, ok := a.authTeacherStudent(w, r)
	if !ok {
		return
	}
	tree, err := a.buildInterestTree(r.Context(), userID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, tree)
}
```

(import `mindimprint/api/internal/httpx`). Register the route in `lite_teacher_routes.go`.

- [ ] **Step 4: Run tests (new + existing tree tests)**

Run: `cd apps/api && CGO_ENABLED=0 go test ./internal/api -run 'TestLiteTeacherTree|Interest' -timeout 1800s`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add apps/api/internal/api/interest.go apps/api/internal/api/lite_teacher_tree.go apps/api/internal/api/lite_teacher_tree_test.go apps/api/internal/api/lite_teacher_routes.go
git commit -m "feat(lite-teacher): 学生的兴趣树（只读）"
```

---

### Task 7: Frontend client, routing, formatting

**Files:**
- Create: `apps/lite-web/src/api/teacher.ts`, `apps/lite-web/src/api/teacher.test.ts`
- Create: `apps/lite-web/src/teacher/teacherRouting.ts`, `apps/lite-web/src/teacher/teacherRouting.test.ts`
- Create: `apps/lite-web/src/teacher/format.ts`, `apps/lite-web/src/teacher/format.test.ts`

**Interfaces:**
- Produces:
  - `teacher.ts`: types `RosterRow`, `ItemRow`, `StudentPage`, `ItemDetail`; functions `getRoster(classId): Promise<RosterRow[]>`, `getStudentPage(classId, userId): Promise<StudentPage>`, `getItem(classId, userId, atomId): Promise<ItemDetail>`, `getStudentTree(classId, userId): Promise<InterestTree>` (reuses `normalize` from `api/interest.ts` — export it as `normalizeInterestTree` there, additive).
  - `teacherRouting.ts`: `type TeacherRoute = {view:"classes"} | {view:"class"; classId} | {view:"student"; classId; userId} | {view:"item"; classId; userId; atomId} | {view:"settings"}`; `parseTeacherRoute(pathname): TeacherRoute`; `teacherRoutePath(route): string`.
  - `format.ts`: `formatMinutes(n: number): string` (`-1` → `"—"`, `<60` → `"{n} 分钟"`, else `"{h} 小时 {m} 分钟"` with `m` omitted when 0); `itemStatusLabel(kind, status): string` (reading/writing `active`→`进行中`, `finished`→`已完成`; project `talking`→`立项中`, `running`→`进行中`, `review`→`回顾中`, `keeping`→`已完成`, `archived`→`已归档`; unknown → the raw status); `kindLabel(kind)` (`reading`→`阅读`, `writing`→`写作`, `project`→`项目`).

- [ ] **Step 1: Failing tests**

`format.test.ts`:

```ts
import { describe, expect, it } from "vitest";
import { formatMinutes, itemStatusLabel, kindLabel } from "./format";

describe("formatMinutes", () => {
  it("shows a dash when no time was recorded", () => expect(formatMinutes(-1)).toBe("—"));
  it("minutes under an hour", () => expect(formatMinutes(45)).toBe("45 分钟"));
  it("whole hours drop the minutes", () => expect(formatMinutes(120)).toBe("2 小时"));
  it("hours and minutes", () => expect(formatMinutes(95)).toBe("1 小时 35 分钟"));
  it("zero is zero, not a dash", () => expect(formatMinutes(0)).toBe("0 分钟"));
});

describe("itemStatusLabel", () => {
  it("reading finished", () => expect(itemStatusLabel("reading", "finished")).toBe("已完成"));
  it("project keeping counts as done", () => expect(itemStatusLabel("project", "keeping")).toBe("已完成"));
  it("unknown status passes through", () => expect(itemStatusLabel("project", "weird")).toBe("weird"));
});

describe("kindLabel", () => {
  it("names each kind", () => {
    expect([kindLabel("reading"), kindLabel("writing"), kindLabel("project")]).toEqual(["阅读", "写作", "项目"]);
  });
});
```

`teacherRouting.test.ts`:

```ts
import { describe, expect, it } from "vitest";
import { parseTeacherRoute, teacherRoutePath, type TeacherRoute } from "./teacherRouting";

const cases: [string, TeacherRoute][] = [
  ["/classes", { view: "classes" }],
  ["/classes/c1", { view: "class", classId: "c1" }],
  ["/classes/c1/students/u1", { view: "student", classId: "c1", userId: "u1" }],
  ["/classes/c1/students/u1/items/a1", { view: "item", classId: "c1", userId: "u1", atomId: "a1" }],
  ["/settings", { view: "settings" }],
];

describe("teacher routing", () => {
  it.each(cases)("parses %s", (path, route) => expect(parseTeacherRoute(path)).toEqual(route));
  it.each(cases)("round-trips %s", (path, route) => expect(teacherRoutePath(route)).toBe(path));
  it("unknown paths land on the class list", () => {
    expect(parseTeacherRoute("/")).toEqual({ view: "classes" });
    expect(parseTeacherRoute("/readings/abc")).toEqual({ view: "classes" });
    expect(parseTeacherRoute("/classes/c1/students")).toEqual({ view: "class", classId: "c1" });
  });
});
```

`teacher.test.ts` — normalizer only (mock `apiFetch` with `vi.mock("./client", …)`):

```ts
import { describe, expect, it, vi } from "vitest";

vi.mock("./client", () => ({
  apiFetch: vi.fn(async () => ({
    roster: [{ id: "u1", displayName: "林同学", minutesThisWeek: -1 }],
  })),
}));

import { getRoster } from "./teacher";

describe("getRoster", () => {
  it("fills missing numeric fields with 0 and keeps -1", async () => {
    const [row] = await getRoster("c1");
    expect(row.minutesThisWeek).toBe(-1);
    expect(row.turns).toBe(0);
    expect(row.lastActiveAt).toBeNull();
  });
});
```

- [ ] **Step 2: Run to verify failure**

Run: `pnpm --filter lite-web test -- src/teacher src/api/teacher.test.ts`
Expected: FAIL (modules missing).

- [ ] **Step 3: Implement**

`format.ts`:

```ts
export function formatMinutes(n: number): string {
  if (n < 0) return "—";
  if (n < 60) return `${n} 分钟`;
  const h = Math.floor(n / 60);
  const m = n % 60;
  return m === 0 ? `${h} 小时` : `${h} 小时 ${m} 分钟`;
}

const STATUS: Record<string, Record<string, string>> = {
  reading: { active: "进行中", finished: "已完成" },
  writing: { active: "进行中", finished: "已完成" },
  project: { talking: "立项中", running: "进行中", review: "回顾中", keeping: "已完成", archived: "已归档" },
};

export function itemStatusLabel(kind: string, status: string): string {
  return STATUS[kind]?.[status] ?? status;
}

export function kindLabel(kind: string): string {
  return ({ reading: "阅读", writing: "写作", project: "项目" } as Record<string, string>)[kind] ?? kind;
}
```

`teacherRouting.ts`:

```ts
// The lite teacher shell's URL model. Same approach as ../routing.ts: no
// router library, root-relative paths, History API via `navigate`.
export type TeacherRoute =
  | { view: "classes" }
  | { view: "class"; classId: string }
  | { view: "student"; classId: string; userId: string }
  | { view: "item"; classId: string; userId: string; atomId: string }
  | { view: "settings" };

const dec = (s: string) => {
  try {
    return decodeURIComponent(s);
  } catch {
    return s;
  }
};
const enc = encodeURIComponent;

export function parseTeacherRoute(pathname: string): TeacherRoute {
  const seg = pathname.replace(/^\/+|\/+$/g, "").split("/").filter(Boolean).map(dec);
  if (seg[0] === "settings") return { view: "settings" };
  if (seg[0] !== "classes" || !seg[1]) return { view: "classes" };
  const classId = seg[1];
  if (seg[2] !== "students" || !seg[3]) return { view: "class", classId };
  const userId = seg[3];
  if (seg[4] !== "items" || !seg[5]) return { view: "student", classId, userId };
  return { view: "item", classId, userId, atomId: seg[5] };
}

export function teacherRoutePath(r: TeacherRoute): string {
  switch (r.view) {
    case "classes":
      return "/classes";
    case "settings":
      return "/settings";
    case "class":
      return `/classes/${enc(r.classId)}`;
    case "student":
      return `/classes/${enc(r.classId)}/students/${enc(r.userId)}`;
    case "item":
      return `/classes/${enc(r.classId)}/students/${enc(r.userId)}/items/${enc(r.atomId)}`;
  }
}
```

`teacher.ts`:

```ts
import { apiFetch } from "./client";
import { normalizeInterestTree, type InterestTree } from "./interest";

export interface RosterRow {
  id: string;
  displayName: string;
  avatarColor: string;
  lastActiveAt: string | null;
  activeDaysThisWeek: number;
  minutesTotal: number;
  minutesThisWeek: number; // -1 = no time recorded yet
  turns: number;
  readingsDone: number;
  readingsTotal: number;
  writingsDone: number;
  writingsTotal: number;
  projectsDone: number;
  projectsTotal: number;
}

export interface ItemRow {
  atomId: string;
  kind: "reading" | "writing" | "project";
  title: string;
  status: string;
  level: number | null;
  minutes: number;
  turns: number;
  createdAt: string;
  lastActiveAt: string;
  finishedAt: string | null;
}

export interface StudentPage {
  student: RosterRow;
  items: ItemRow[];
}

export interface ReportSlice {
  stats: { key: string; value: number; unit: string }[];
  moments: { quote: string; where: string }[];
  keep: { text: string; source: "student" | "coach"; label?: string } | null;
}

export interface ItemDetail {
  item: ItemRow;
  reading: {
    source: { librarySlug: string | null; level: number | null; url: string | null } | null;
    highlights: { quote: string; note: string }[];
    takeaway: string | null;
    lenses: { cardId: string; fields: Record<string, unknown> }[];
  } | null;
  writing: {
    targetWords: number | null;
    lang: string;
    structureKey: string;
    outline: { role: string; text: string }[];
    snippets: { position: number; text: string }[];
    draft: string | null;
    comments: { scope: string; summary: string; points: unknown[] }[];
  } | null;
  project: {
    idea: string;
    status: string;
    stepsDone: number;
    stepsTotal: number;
    steps: { title: string; status: string }[];
    tools: { key: string; status: string; result: unknown }[];
    artifacts: { title: string; payload: unknown }[];
    keeps: { text: string }[];
    courses: { slug: string; why: string; takeaway: string; finishedAt: string | null }[];
    siteToken: string | null;
  } | null;
  report: ReportSlice | null;
  reportError: string | null;
}

const n = (v: unknown) => (typeof v === "number" ? v : 0);
const s = (v: unknown) => (typeof v === "string" ? v : "");

export function normalizeRosterRow(raw: Record<string, unknown>): RosterRow {
  return {
    id: s(raw.id),
    displayName: s(raw.displayName),
    avatarColor: s(raw.avatarColor),
    lastActiveAt: typeof raw.lastActiveAt === "string" ? raw.lastActiveAt : null,
    activeDaysThisWeek: n(raw.activeDaysThisWeek),
    minutesTotal: n(raw.minutesTotal),
    minutesThisWeek: typeof raw.minutesThisWeek === "number" ? raw.minutesThisWeek : -1,
    turns: n(raw.turns),
    readingsDone: n(raw.readingsDone),
    readingsTotal: n(raw.readingsTotal),
    writingsDone: n(raw.writingsDone),
    writingsTotal: n(raw.writingsTotal),
    projectsDone: n(raw.projectsDone),
    projectsTotal: n(raw.projectsTotal),
  };
}

const base = (classId: string) => `/api/v1/lite/teacher/classes/${encodeURIComponent(classId)}`;

export async function getRoster(classId: string): Promise<RosterRow[]> {
  const r = await apiFetch<{ roster?: Record<string, unknown>[] }>(`${base(classId)}/roster`);
  return (r.roster ?? []).map(normalizeRosterRow);
}

export async function getStudentPage(classId: string, userId: string): Promise<StudentPage> {
  const r = await apiFetch<{ student: Record<string, unknown>; items?: ItemRow[] }>(
    `${base(classId)}/students/${encodeURIComponent(userId)}`,
  );
  return { student: normalizeRosterRow(r.student ?? {}), items: r.items ?? [] };
}

export async function getItem(classId: string, userId: string, atomId: string): Promise<ItemDetail> {
  return apiFetch<ItemDetail>(
    `${base(classId)}/students/${encodeURIComponent(userId)}/items/${encodeURIComponent(atomId)}`,
  );
}

export async function getStudentTree(classId: string, userId: string): Promise<InterestTree> {
  return normalizeInterestTree(
    await apiFetch(`${base(classId)}/students/${encodeURIComponent(userId)}/tree`),
  );
}
```

In `api/interest.ts`, rename nothing; add `export const normalizeInterestTree = normalize;` right after `normalize` is defined (and check `normalize`'s parameter type — cast the `apiFetch` result to it in `getStudentTree`).

- [ ] **Step 4: Run tests + typecheck**

Run: `pnpm --filter lite-web test && pnpm --filter lite-web typecheck`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add apps/lite-web/src/api/teacher.ts apps/lite-web/src/api/teacher.test.ts apps/lite-web/src/api/interest.ts apps/lite-web/src/teacher/
git commit -m "feat(lite-teacher): 前端接口、路由与格式化"
```

---

### Task 8: Teacher shell, class list, class page

**Files:**
- Create: `apps/lite-web/src/teacher/LiteTeacherShell.tsx`, `apps/lite-web/src/teacher/ClassPage.tsx`
- Modify: `apps/lite-web/src/LiteApp.tsx`

**Interfaces:**
- Consumes: `parseTeacherRoute`, `teacherRoutePath` (Task 7), `getRoster`, `formatMinutes` (Task 7); pro `ClassesView` (`@/console/ClassesView`) with pro `api` client (`import { api } from "@/api"`); pro client functions `api.getClass`, `api.renameClass`, `api.regenerateJoinCode`, `api.removeEnrollment`; lite `navigate` from `../routing`; pro `SettingsView`.
- Produces: `LiteTeacherShell({ user, onLogout }: { user: MeUser; onLogout: () => void })`; `ClassPage({ classId, role, onOpenStudent })`.

UI is verified by screenshot in Task 10, not by unit tests.

- [ ] **Step 1: Shell**

`LiteTeacherShell.tsx` — copy the rail markup from `LiteShell` in `LiteApp.tsx` (the `mk-lite-navslot`/`mk-lite-nav` block, `labelCls`, the settings button pinned with `mt-auto`) so the two shells look identical. Rail items: 班级 (`Users` icon from lucide-react) → `/classes`; 设置 at the foot. Route switch:

```tsx
import { useEffect, useState } from "react";
import { Users } from "lucide-react";
import { Icon, Pebble, Settings } from "@/ui";
import { api } from "@/api";
import { ClassesView } from "@/console/ClassesView";
import { SettingsView } from "@/shell/settings/SettingsView";
import { navigate } from "../routing";
import type { MeUser } from "../api/auth";
import { parseTeacherRoute, teacherRoutePath, type TeacherRoute } from "./teacherRouting";
import { ClassPage } from "./ClassPage";
import { StudentPage } from "./StudentPage";
import { ItemPage } from "./ItemPage";

export function LiteTeacherShell({ user, onLogout }: { user: MeUser; onLogout: () => void }) {
  const [route, setRoute] = useState<TeacherRoute>(() => parseTeacherRoute(window.location.pathname));
  useEffect(() => {
    const onPop = () => setRoute(parseTeacherRoute(window.location.pathname));
    window.addEventListener("popstate", onPop);
    return () => window.removeEventListener("popstate", onPop);
  }, []);
  const go = (r: TeacherRoute) => navigate(teacherRoutePath(r));

  return (
    <div className="flex h-full w-full overflow-hidden bg-mk-paper text-mk-ink">
      {/* rail: copied from LiteShell; items 班级 + 设置 */}
      <main className="min-w-0 flex-1 overflow-y-auto">
        {route.view === "classes" && (
          <ClassesView client={api} role={user.role} onOpenClass={(classId) => go({ view: "class", classId })} />
        )}
        {route.view === "class" && (
          <ClassPage
            classId={route.classId}
            role={user.role}
            onBack={() => go({ view: "classes" })}
            onOpenStudent={(userId) => go({ view: "student", classId: route.classId, userId })}
          />
        )}
        {route.view === "student" && (
          <StudentPage
            classId={route.classId}
            userId={route.userId}
            onBack={() => go({ view: "class", classId: route.classId })}
            onOpenItem={(atomId) => go({ view: "item", classId: route.classId, userId: route.userId, atomId })}
          />
        )}
        {route.view === "item" && (
          <ItemPage
            classId={route.classId}
            userId={route.userId}
            atomId={route.atomId}
            onBack={() => go({ view: "student", classId: route.classId, userId: route.userId })}
          />
        )}
        {route.view === "settings" && (
          <SettingsView session={unusedSettingsSession} user={user} onLogout={onLogout} />
        )}
      </main>
    </div>
  );
}
```

At module scope in `LiteTeacherShell.tsx`, recreate the same unused session `LiteApp.tsx` passes (`import { createSession, makeMemoryStorage } from "@/shell/session";`):

```tsx
// SettingsView requires a pro SessionStore; lite's auth state lives in LiteApp,
// so this one is never read. Same construction as LiteApp.tsx's unusedSettingsSession.
const unusedSettingsSession = createSession({ storage: makeMemoryStorage() });
``` `ClassesView` is typed against a `Pick<ApiClient, …>`; pass `api` (it satisfies the pick). If `ClassesView` renders pro-only copy that is wrong for lite, do not edit it — note it in the task report.

Tasks 9's `StudentPage` and `ItemPage` do not exist yet: create both files in this task as minimal components that render `<div className="p-10">加载中…</div>` so the shell compiles; Task 9 fills them.

- [ ] **Step 2: Class page**

`ClassPage.tsx`: header (class name with 改名 → inline input + 保存 / 取消 calling `api.renameClass(classId, name)`; 邀请码 + 复制 + 轮换 with the same confirm text as pro: 轮换后旧邀请码立即失效，确定？ / 确认轮换 / 取消, calling `api.regenerateJoinCode(classId)`), a 返回 link, then the roster table from `getRoster(classId)`. Class name and join code come from `api.getClass(classId)`.

**Admin rail items (spec §3.1).** When `user.role === "admin"`, the rail also shows 概览 / 教师 / 导入 above 设置, mounting pro's `OverviewView`, `TeachersView`, `ImportView` (`@/console/…`) with `client={api}`. Add `{view:"overview"} | {view:"teachers"} | {view:"import"}` to `TeacherRoute` with paths `/overview`, `/teachers`, `/import`, and add those three cases to `teacherRouting.test.ts`'s table (Task 7 already exists by now — extend its test and parser in this task). A teacher navigating to those paths lands on `/classes`. Admins start on 概览, teachers on 班级 (same as pro `ConsoleShell`).

Columns (labels are nouns): 学生 · 最近活跃 · 本周活跃天数 · 本周时长 · 累计时长 · 对话轮次 · 阅读 · 写作 · 项目 · 操作. 阅读/写作/项目 cells show `{done}/{total}`. 最近活跃 shows `M月D日` or `—`. Clicking a row calls `onOpenStudent(id)`. 操作 is 移出班级 with an inline confirm (移出后该学生将无法看到本班内容，确定？ / 确认移出 / 取消) calling `api.removeEnrollment(classId, userId)` then reloading. Column headers sort client-side (click toggles asc/desc); keep the sort as component state.

States: loading → 加载中…; error → `加载失败：{message}` + 重试; empty roster → `暂无学生。请将邀请码 {code} 发给学生。`

Styling: Tailwind with lite tokens (`bg-mk-surface`, `text-mk-ink`, `text-mk-muted`, `rounded-mk-lg`, `shadow-mk-xs`), like `ReadingsLanding`. Do not use Tailwind alpha modifiers on `mk-*` colours (they emit no CSS); use `color-mix` in `style` when a tint is needed. No left colour bars.

- [ ] **Step 3: Role branch in LiteApp**

In `LiteApp.tsx`, import `LiteTeacherShell` and replace `<LiteShell user={user} onLogout={onLogout} />` with:

```tsx
        {user.role === "teacher" || user.role === "admin" ? (
          <LiteTeacherShell user={user} onLogout={onLogout} />
        ) : (
          <LiteShell user={user} onLogout={onLogout} />
        )}
```

Check `LiteApp`'s `MeUser` (from `./api/auth`) has `role`; if the lite type lacks it, add `role: string` there (the API already sends it).

- [ ] **Step 4: Typecheck + tests**

Run: `pnpm --filter lite-web typecheck && pnpm --filter lite-web test`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add apps/lite-web/src/teacher/LiteTeacherShell.tsx apps/lite-web/src/teacher/ClassPage.tsx apps/lite-web/src/teacher/StudentPage.tsx apps/lite-web/src/teacher/ItemPage.tsx apps/lite-web/src/LiteApp.tsx apps/lite-web/src/api/auth.ts
git commit -m "feat(lite-teacher): 教师端外壳、班级列表与班级页"
```

---

### Task 9: Student page, item page, read-only tree

**Files:**
- Modify: `apps/lite-web/src/teacher/StudentPage.tsx`, `apps/lite-web/src/teacher/ItemPage.tsx`
- Modify: `apps/lite-web/src/tree/useInterestTree.ts`, `apps/lite-web/src/tree/TreeView.tsx`

**Interfaces:**
- Consumes: `getStudentPage`, `getItem`, `getStudentTree`, `formatMinutes`, `itemStatusLabel`, `kindLabel` (Task 7); `statLabel` resolution from `apps/lite-web/src/reports/statLabels.ts`.
- Produces: `useInterestTree(fetcher?: () => Promise<InterestTree>)` (default `fetchInterestTree`); `TreeView({ user, live, readOnly }: { user: MeUser; live: LiveTree; readOnly?: boolean })`.

- [ ] **Step 1: Tree hooks**

`useInterestTree.ts`: signature `export function useInterestTree(fetcher: () => Promise<InterestTree> = fetchInterestTree): LiveTree`, call `fetcher()` instead of `fetchInterestTree()`, and add `fetcher` to the effect deps. Callers passing an inline arrow must memoise it (`useCallback`) to avoid refetch loops — note this in the doc comment.

`TreeView.tsx`: add `readOnly?: boolean` (default `false`). When `readOnly`:
- do not call `useQuizTaken()`'s side effects that navigate; hide every quiz entry point (`openQuiz` buttons, the empty-branch invite `inviteField` UI);
- in the keyword drawer, hide `DigSection` and any proposal / accept / reject controls (find them with `grep -n "DigSection\|Proposal\|openQuiz\|inviteField" apps/lite-web/src/tree/*.tsx`); pass `readOnly` down to `KeywordDrawer` as an optional prop defaulting to `false`.
- everything else (branches, leaves, roots, growth timeline, source list) renders unchanged.

Default `false` keeps the student tree byte-for-byte the same.

- [ ] **Step 2: Student page**

`StudentPage.tsx`:
- Header: name, 返回, stat tiles — 累计时长 (`formatMinutes(minutesTotal)`), 本周时长, 本周活跃天数 (`{n} 天`), 对话轮次 (`{n} 轮`), 阅读 `{done}/{total}`, 写作, 项目.
- A reserved slot titled 上周表现总结 with the text `下一版本提供` is **not** added — plan 3 adds the card; leave no placeholder UI.
- Three sections 阅读 / 写作 / 项目, each a list of `ItemRow`s of that kind: title, 状态 (`itemStatusLabel`), 时长 (`formatMinutes(minutes)`), 对话 `{turns} 轮`, 最近活跃 date, level for readings (`第 {level} 档`). Clicking opens the item. Empty section → `暂无{kindLabel}记录`.
- 兴趣树 section: `const fetcher = useCallback(() => getStudentTree(classId, userId), [classId, userId]); const live = useInterestTree(fetcher);` then `<TreeView user={studentAsMeUser} live={live} readOnly />`. `TreeView` needs a `MeUser`; build a minimal object from the student row (`id`, `display_name`, `role: "student"`, and the other required fields with empty values) — read which fields `TreeView` actually uses and fill those truthfully. States: error → `兴趣树加载失败：{error}`; empty → `暂无兴趣关键词`.
- Loading/error states as in `ClassPage`.

- [ ] **Step 3: Item page**

`ItemPage.tsx` renders `ItemDetail`:
- Header: `kindLabel` · title · status · 时长 · 对话轮次 · finished date; 返回.
- A line under the header: `对话内容不向教师展示；以下为学生的产出。`
- `report`: stat tiles (label from `statLabels.ts` by `key`, never the stored label); 金句 list labelled 学生原话; 收获 card with an attribution line — `source === "student"` → 学生自己写的收获, `"coach"` → 印记根据学习过程整理 (never drop this line).
- `reportError` → `报告生成失败：{reportError}` (the server already prefixes 报告生成失败：, so render it as-is).
- reading: 来源 (library slug + `第 {level} 档`, or URL link), 划线与笔记 (quote in muted text, note below), 收获 (`takeaway`), 阅读透镜 (each lens as a card listing field name → value; render string values only, skip non-strings).
- writing: 要求 (目标字数 / 语言 / 结构), 提纲 (role → text), 片段, 成稿 (`draft` with preserved line breaks, `whitespace-pre-wrap`), AI 批注 (summary + points as a list of strings when they are strings).
- project: 驱动问题 (`idea`), 进度 `{stepsDone}/{stepsTotal}` + step list with status, 工具产出 (key + result shown as formatted JSON in a `<pre>` only if it is not a plain string), 成果 (artifacts), 保留条目, 课程 (slug + why + takeaway), 主页链接 `/p/{siteToken}` when present.
- Empty sub-sections are omitted, not shown as 暂无.

- [ ] **Step 4: Typecheck + tests (lite and pro, since tree files are lite-only but `@/` components are shared)**

Run: `pnpm --filter lite-web typecheck && pnpm --filter lite-web test`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add apps/lite-web/src/teacher/StudentPage.tsx apps/lite-web/src/teacher/ItemPage.tsx apps/lite-web/src/tree/useInterestTree.ts apps/lite-web/src/tree/TreeView.tsx apps/lite-web/src/tree/KeywordDrawer.tsx
git commit -m "feat(lite-teacher): 学生页、单项详情页与只读兴趣树"
```

---

### Task 10: Browser walk, pro safety, push

**Files:**
- Create (throwaway, not committed): `$CLAUDE_JOB_DIR/tmp/teacher-walk.spec.ts` or a script under `apps/lite-web/e2e/` if the repo's e2e config requires it there (check `apps/lite-web/playwright.config.*`).

- [ ] **Step 1: Seed data locally**

Start Postgres + API + lite-web the way the repo's docs describe (`apps/api/README` / `docs/deploy` / `apps/lite-web/e2e/freshAccount.ts`). Create: a lite school (or set the seed school `edition='lite'`), a teacher, a class, two students enrolled; for one student create a finished reading with an annotation and a takeaway, a writing with a draft, a project, a few student messages, heartbeat buckets, and one interest keyword.

- [ ] **Step 2: Walk and screenshot**

Log in as the teacher at 1440×900 and at 400×800. Capture: class list, class page roster, student page (with tree), a reading item, a writing item, a project item. Read each screenshot. Check: nothing blank, no overflow at 400px, labels follow the copy rules, no chat text anywhere, `—` shown where time is unknown, the tree renders without quiz / dig / proposal controls.

Also log in as a student and confirm the student app is unchanged (rail, explore landing).

Fix anything found, then re-run Steps 4 of the affected tasks.

- [ ] **Step 3: Pro safety**

```bash
git diff --diff-filter=D --name-only origin/main..HEAD   # must print nothing
git diff --diff-filter=R --name-only origin/main..HEAD   # must print nothing
git diff --numstat origin/main..HEAD -- apps/web apps/api/internal/api/interest.go
pnpm --filter web typecheck && pnpm --filter web test
cd apps/api && CGO_ENABLED=0 go vet ./... && CGO_ENABLED=0 go test ./... -timeout 1800s
```

Expected: no deletions/renames; `apps/web` untouched; all suites green. Report any pre-existing failures separately with evidence that they fail on `origin/main` too.

- [ ] **Step 4: Push**

```bash
git fetch origin && git rebase origin/main && git push origin HEAD:main
```

(If a migration number collides after rebase, renumber this plan's migration to the next free number, re-run `make sqlc`, re-run the Go tests, and amend in a new commit.)
