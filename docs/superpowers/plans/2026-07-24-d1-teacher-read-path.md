# D1 教师端读路径 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** A teacher can walk 全部学生 → 学生个人页 → 深度能力报告 → 证据地图 for any student in their class — every judgment evidence-backed, single-direction read-only, all a teacher projection of the canonical `agent.Report` Spec B already produces.

**Architecture:** New teacher-scoped read queries (tenancy-guarded, mirroring `ListGrowthHistory`) + three console views + a client-derived evidence map + a seed migration. No new LLM call, no analytics pipeline (that is D2). Depends only on Spec B (`agent.Report` / `studio.ReportDTO`, already on main).

**Tech Stack:** Go (`net/http` + sqlc + goose) backend; React + TypeScript + Vite web; testcontainers for Go store/handler tests; vitest for web.

**Design source of truth:** `docs/design/teacher end/project/思维印记 教师端.dc.html` (binding UI). Spec: `docs/superpowers/specs/2026-07-24-d1-teacher-read-path-design.md`.

## Global Constraints

- **Client NEVER calls a model.** D1 makes ZERO LLM calls (pure read). No key handling anywhere.
- **Single-direction read-only.** Every new endpoint is `GET`. No teacher action reaches the student end. No writes to student data.
- **Tenancy.** Every teacher endpoint first calls `assertTeacherOwnsClass` (cross-class / non-teacher → 404, existence-hiding). Student-scoped queries additionally JOIN `enrollments` with `class_id=@class_id AND role_in_class='student'` so a teacher cannot read a non-member. A student hitting a teacher endpoint → 403 (via `RequireRole("teacher","admin")`).
- **RL-5.** The two axes never combine into a total. The only numeric aggregate anywhere is `OfficialProjection.Readiness.Score` (project-surface only). Badge derivation produces display strings, never a composite score.
- **每个判断带证据.** Every rendered level is backed by its `evidence` text; nothing renders without evidence.
- **敢于空白.** Missing evidence renders "暂无可计入的证据" / unrated `—`, never a fabricated judgment.
- **sqlc discipline.** Run `make sqlc` from `apps/api` after editing any `store/queries/*.sql`. NEVER hand-edit `apps/api/internal/store/sqlc/*` (generated).
- **Icons inline SVG.** Never `lucide-react`.
- **git add by explicit path only.** Never `git add .` / a whole directory. Pre-existing `M package.json` + untracked `docs/` + PNGs are NOT ours.
- **Go test command (FULL packages, never `-run` subsets for query/handler/migration/DTO changes):**
  `cd /Users/houyuxin/08Coding/mind-imprint/apps/api && DOCKER_HOST=unix:///var/run/docker.sock CGO_ENABLED=0 go test -p 1 ./...`
- **Web test:** `cd /Users/houyuxin/08Coding/mind-imprint/apps/web && npm test` + `npx tsc --noEmit`.
- **Turn-count口径 (pinned):** 对话轮次 = `COUNT(event WHERE type IN ('prompt_sent','course_message'))` for the user in the week window. `prompt_sent` fires for both `studio` and `chat` surfaces; `course_message` for course. 活跃天数 = `COUNT(DISTINCT date(event.created_at))` over ALL event types.

---

## File Structure

**New backend files:**
- `apps/api/internal/teacher/badges.go` + `badges_test.go` — pure D/A badge derivation from an `agent.Report`.
- `apps/api/internal/store/queries/teacher.sql` — teacher-scoped read queries (regenerates into `store/sqlc/`).
- `apps/api/internal/api/teacher_read.go` — the three handlers + DTOs.
- `apps/api/internal/api/teacher_read_test.go` — tenancy matrix + shape tests.
- `apps/api/internal/store/migrations/0029_seed_teacher_class.sql` — seed teacher + 9 students + 3 CASE evaluations.

**Modified backend files:**
- `apps/api/internal/api/api.go` — register 3 routes under `teacherOrAdmin`.

**New web files:**
- `apps/web/src/api/teacher.ts` — client fns + types.
- `apps/web/src/console/StudentDetailView.tsx` + `test/console/StudentDetailView.test.tsx`.
- `apps/web/src/console/TeacherReportView.tsx` + `test/console/TeacherReportView.test.tsx`.
- `apps/web/src/console/EvidenceMap.tsx` + `test/console/EvidenceMap.test.tsx`.

**Modified web files:**
- `apps/web/src/api/index.ts` — aggregate the new client fns into `ApiClient`.
- `apps/web/src/console/ConsoleShell.tsx` — add `openStudentId` / `openReport` screen state.
- `apps/web/src/console/ClassDetailView.tsx` — enrich roster table (D/A badges + usage), rows clickable.
- `apps/web/test/console/ClassDetailView.test.tsx` — update for enriched roster (if exists).

---

## Task 1: D/A badge derivation (pure Go)

**Files:**
- Create: `apps/api/internal/teacher/badges.go`
- Test: `apps/api/internal/teacher/badges_test.go`

**Interfaces:**
- Consumes: `agent.Report` (`mindimprint/api/internal/agent`; `DepthAxis []DepthDim{Level string L1|L2|L3|L4|NA}`, `AutonomyAxis []AutonomySignal{Level int 0..5, Opportunity string}`).
- Produces: `func DBadge(r agent.Report) string`, `func ABadge(r agent.Report) string`. Both pure. `DBadge` → `"L2–L4"` | `"L3"` | `"—"`. `ABadge` → `"4.2"` | `"—"`.

**Rules (spec §5.2):**
- `DBadge`: over `DepthAxis` levels in {L1,L2,L3,L4} (rank L1<L2<L3<L4), ignore `NA`. None present → `"—"`. min==max → single (`"L3"`). Else min–max with en-dash `–` (U+2013): `"L2–L4"`.
- `ABadge`: mean of `AutonomySignal.Level` **only for signals where `Opportunity != "not_supplied"`** (机会供给先于判定 — platform-unsupplied signals don't count against the student), one decimal. Zero supplied signals → `"—"`.

- [ ] **Step 1: Write the failing test**

```go
package teacher

import (
	"testing"

	"mindimprint/api/internal/agent"
)

func d(level string) agent.DepthDim { return agent.DepthDim{Level: level} }
func a(level int, opp string) agent.AutonomySignal { return agent.AutonomySignal{Level: level, Opportunity: opp} }

func TestDBadge(t *testing.T) {
	cases := []struct {
		name string
		in   []agent.DepthDim
		want string
	}{
		{"range", []agent.DepthDim{d("L3"), d("L4"), d("L3"), d("L3"), d("L4"), d("L3")}, "L3–L4"},
		{"single", []agent.DepthDim{d("L2"), d("L2")}, "L2"},
		{"na ignored", []agent.DepthDim{d("NA"), d("L1"), d("NA")}, "L1"},
		{"all na", []agent.DepthDim{d("NA"), d("NA")}, "—"},
		{"empty", nil, "—"},
		{"full spread", []agent.DepthDim{d("L1"), d("L4")}, "L1–L4"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := DBadge(agent.Report{DepthAxis: c.in}); got != c.want {
				t.Fatalf("DBadge = %q, want %q", got, c.want)
			}
		})
	}
}

func TestABadge(t *testing.T) {
	cases := []struct {
		name string
		in   []agent.AutonomySignal
		want string
	}{
		{"plain mean", []agent.AutonomySignal{a(4, "given_taken"), a(4, "given_taken"), a(4, "given_taken"), a(4, "given_taken"), a(5, "given_taken"), a(4, "given_not_taken")}, "4.2"},
		{"not_supplied excluded", []agent.AutonomySignal{a(4, "given_taken"), a(0, "not_supplied"), a(0, "not_supplied")}, "4.0"},
		{"all not_supplied", []agent.AutonomySignal{a(0, "not_supplied"), a(0, "not_supplied")}, "—"},
		{"empty", nil, "—"},
		{"half band", []agent.AutonomySignal{a(0, "given_not_taken"), a(1, "given_taken")}, "0.5"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := ABadge(agent.Report{AutonomyAxis: c.in}); got != c.want {
				t.Fatalf("ABadge = %q, want %q", got, c.want)
			}
		})
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd /Users/houyuxin/08Coding/mind-imprint/apps/api && CGO_ENABLED=0 go test ./internal/teacher/`
Expected: FAIL (package/functions not defined).

- [ ] **Step 3: Write minimal implementation**

```go
// Package teacher holds pure teacher-facing projections of the canonical
// assessment object (agent.Report). Badges are display summaries only — never
// a composite score (RL-5). Recomputed on every read; never persisted.
package teacher

import (
	"fmt"

	"mindimprint/api/internal/agent"
)

var depthRank = map[string]int{"L1": 1, "L2": 2, "L3": 3, "L4": 4}

// DBadge summarises the six depth levels as a min–max range (en-dash), a single
// level when they coincide, or "—" when no dimension carries a real level.
func DBadge(r agent.Report) string {
	min, max := 0, 0
	for _, d := range r.DepthAxis {
		rank, ok := depthRank[d.Level]
		if !ok {
			continue // NA / unknown → no evidence, skip
		}
		if min == 0 || rank < min {
			min = rank
		}
		if rank > max {
			max = rank
		}
	}
	if min == 0 {
		return "—"
	}
	if min == max {
		return fmt.Sprintf("L%d", min)
	}
	return fmt.Sprintf("L%d–L%d", min, max) // U+2013 en-dash
}

// ABadge is the mean of the autonomy levels for signals whose opportunity was
// actually supplied (机会供给先于判定: not_supplied is platform debt, excluded),
// to one decimal, or "—" when no signal was supplied.
func ABadge(r agent.Report) string {
	sum, n := 0, 0
	for _, a := range r.AutonomyAxis {
		if a.Opportunity == "not_supplied" {
			continue
		}
		sum += a.Level
		n++
	}
	if n == 0 {
		return "—"
	}
	return fmt.Sprintf("%.1f", float64(sum)/float64(n))
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd /Users/houyuxin/08Coding/mind-imprint/apps/api && CGO_ENABLED=0 go test ./internal/teacher/`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add apps/api/internal/teacher/badges.go apps/api/internal/teacher/badges_test.go
git commit -m "feat(d1): D/A badge derivation from canonical report"
```

---

## Task 2: Teacher-scoped read queries (sqlc)

**Files:**
- Create: `apps/api/internal/store/queries/teacher.sql`
- Modify (generated, via `make sqlc`): `apps/api/internal/store/sqlc/*`
- Test: `apps/api/internal/store/teacher_query_test.go`

**Interfaces:**
- Produces (sqlc-generated on `Queries`): `ListClassRosterReport`, `GetStudentUsageForTeacher`, `ListStudentReportsForTeacher`, `ListStudentProjectsForTeacher`, `GetStudentProjectEvaluationForTeacher`, `GetStudentSessionEvaluationForTeacher`, `GetStudentThreadEvaluationForTeacher`.
- Consumes: existing tables `enrollments`, `users`, `project`, `evaluations`, `event`, `course_session`, `course`, `chat_thread`, `graph_node`.

**Context:** these mirror `ListGrowthHistory` (`evaluation.sql:57`) and `GetClassRoster` (`org.sql:41`), swapping the owner filter for a class-membership JOIN. All week-windowed usage takes `@week_start` / `@week_end` (half-open) params — the handler computes the window; the query never calls `now()`.

- [ ] **Step 1: Write the queries**

Create `apps/api/internal/store/queries/teacher.sql`:

```sql
-- Teacher read-path (Spec D1). Every query is class-scoped: it JOINs enrollments
-- with role_in_class='student' so a teacher can only read members of the class
-- the handler already authorised via assertTeacherOwnsClass. No writes.

-- name: ListClassRosterReport :many
-- One row per student: latest project-scope report scores (for D/A badge
-- derivation, NULL when unrated) + this-week activity. Mirrors GetClassRoster's
-- enrollment scoping (org.sql).
SELECT
  u.id, u.display_name, u.avatar_color,
  ev.scores AS latest_project_scores,
  COALESCE(act.active_days, 0)::int AS active_days,
  COALESCE(act.turns, 0)::int       AS turns,
  (ev.scores IS NOT NULL)           AS has_report
FROM enrollments e
JOIN users u ON u.id = e.user_id
LEFT JOIN LATERAL (
  SELECT ev2.scores
  FROM evaluations ev2
  JOIN project p2 ON p2.id = ev2.project_id
  WHERE p2.user_id = u.id AND ev2.project_id IS NOT NULL
  ORDER BY ev2.created_at DESC
  LIMIT 1
) ev ON true
LEFT JOIN LATERAL (
  SELECT
    COUNT(DISTINCT (ev4.created_at)::date) AS active_days,
    COUNT(*) FILTER (WHERE ev4.type IN ('prompt_sent','course_message')) AS turns
  FROM event ev4
  WHERE ev4.user_id = u.id
    AND ev4.created_at >= @week_start AND ev4.created_at < @week_end
) act ON true
WHERE e.class_id = @class_id AND e.role_in_class = 'student'
ORDER BY u.display_name;

-- name: GetStudentUsageForTeacher :one
-- This-week active days + turns for one student (used by the student detail head).
SELECT
  COUNT(DISTINCT (ev.created_at)::date) AS active_days,
  COUNT(*) FILTER (WHERE ev.type IN ('prompt_sent','course_message')) AS turns
FROM event ev
WHERE ev.user_id = @user_id
  AND ev.created_at >= @week_start AND ev.created_at < @week_end;

-- name: ListStudentReportsForTeacher :many
-- Every report one class member owns, across all three scopes, newest-first,
-- one row per scope. Teacher variant of ListGrowthHistory: same shape, but the
-- owner filter is replaced by "this user AND a student member of this class".
SELECT surface, scope_id, label, sublabel, created_at
FROM (
  (SELECT DISTINCT ON (e.project_id)
     'project'::text AS surface, e.project_id AS scope_id,
     p.title AS label, NULL::text AS sublabel, e.created_at AS created_at
   FROM evaluations e JOIN project p ON p.id = e.project_id
   WHERE e.project_id IS NOT NULL AND p.user_id = @user_id
   ORDER BY e.project_id, e.created_at DESC)
  UNION ALL
  (SELECT DISTINCT ON (e.session_id)
     'course'::text, e.session_id, c.title, cs.phase, e.created_at
   FROM evaluations e
     JOIN course_session cs ON cs.id = e.session_id
     JOIN course c ON c.id = cs.course_id
   WHERE e.session_id IS NOT NULL AND cs.user_id = @user_id
   ORDER BY e.session_id, e.created_at DESC)
  UNION ALL
  (SELECT DISTINCT ON (e.thread_id)
     'chat'::text, e.thread_id, t.title, NULL::text, e.created_at
   FROM evaluations e JOIN chat_thread t ON t.id = e.thread_id
   WHERE e.thread_id IS NOT NULL AND t.user_id = @user_id
   ORDER BY e.thread_id, e.created_at DESC)
) rows
ORDER BY created_at DESC;

-- name: ListStudentProjectsForTeacher :many
-- All of a student's projects (title/date + whether a report exists), so
-- in-progress projects without a report still appear in the records list.
SELECT p.id, p.title, p.last_active_at,
       EXISTS (SELECT 1 FROM evaluations ev WHERE ev.project_id = p.id) AS has_report
FROM project p
WHERE p.user_id = @user_id
ORDER BY p.last_active_at DESC NULLS LAST;

-- name: GetStudentProjectEvaluationForTeacher :one
-- Read one project report + its RQ context, guarded by student ownership.
-- researchQuestion: prefer the plan node's research_question, else project.title.
SELECT e.scores, e.created_at, p.title AS project_title,
       (SELECT gn.body FROM graph_node gn
          WHERE gn.project_id = p.id AND gn.type = 'research_question'
          ORDER BY gn.created_at, gn.id LIMIT 1) AS rq_body
FROM evaluations e
JOIN project p ON p.id = e.project_id
WHERE e.project_id = @scope_id AND p.user_id = @user_id
ORDER BY e.created_at DESC
LIMIT 1;

-- name: GetStudentSessionEvaluationForTeacher :one
SELECT e.scores, e.created_at, c.title AS course_title
FROM evaluations e
JOIN course_session cs ON cs.id = e.session_id
JOIN course c ON c.id = cs.course_id
WHERE e.session_id = @scope_id AND cs.user_id = @user_id
ORDER BY e.created_at DESC
LIMIT 1;

-- name: GetStudentThreadEvaluationForTeacher :one
SELECT e.scores, e.created_at, t.title AS thread_title
FROM evaluations e
JOIN chat_thread t ON t.id = e.thread_id
WHERE e.thread_id = @scope_id AND t.user_id = @user_id
ORDER BY e.created_at DESC
LIMIT 1;
```

- [ ] **Step 2: Regenerate sqlc**

Run: `cd /Users/houyuxin/08Coding/mind-imprint/apps/api && make sqlc`
Expected: no errors; new methods appear in `internal/store/sqlc/`. If `research_question` is not a valid node type or a column name differs, fix the SQL (do NOT hand-edit generated files). Verify the generated param struct names (`ListClassRosterReportParams` with `ClassID`, `WeekStart`, `WeekEnd`, etc.).

- [ ] **Step 3: Write the store test (failing until queries exist — they now do)**

Create `apps/api/internal/store/teacher_query_test.go`. Follow the existing testcontainers pattern in this package (grep a sibling `*_query_test.go` for the harness that spins up Postgres, runs migrations, and returns `*sqlc.Queries`). The test must:
1. Seed a school, class, one teacher enrollment, two students (A enrolled in the class, B enrolled in a DIFFERENT class).
2. Give student A: one project + one `evaluations` row (`project_id` set, `scores` = a minimal valid `agent.Report` JSON with 6 depth + 6 autonomy) + a few `event` rows in-window (`prompt_sent`, `course_message`, and one out-of-window).
3. Assert `ListClassRosterReport(classID, weekStart, weekEnd)` returns exactly student A (not B), with `has_report=true`, `active_days`/`turns` matching the in-window events only, and `latest_project_scores` non-nil.
4. Assert `GetStudentUsageForTeacher` matches. Assert `ListStudentReportsForTeacher` returns the project row. Assert `GetStudentProjectEvaluationForTeacher(scopeID=projectID, userID=A)` returns scores + title; and returns `pgx.ErrNoRows` when `userID=B` (ownership guard).

- [ ] **Step 4: Run the store test**

Run: `cd /Users/houyuxin/08Coding/mind-imprint/apps/api && DOCKER_HOST=unix:///var/run/docker.sock CGO_ENABLED=0 go test ./internal/store/`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add apps/api/internal/store/queries/teacher.sql apps/api/internal/store/sqlc apps/api/internal/store/teacher_query_test.go
git commit -m "feat(d1): teacher-scoped read queries (roster-report, usage, student reports)"
```

---

## Task 3: Roster-report endpoint

**Files:**
- Create: `apps/api/internal/api/teacher_read.go` (roster handler + DTOs + week-window helper)
- Modify: `apps/api/internal/api/api.go` (register route)
- Test: `apps/api/internal/api/teacher_read_test.go`

**Interfaces:**
- Consumes: `assertTeacherOwnsClass` (`authz.go:47`), `Queries.ListClassRosterReport`, `teacher.DBadge`/`teacher.ABadge`, `agent.Report`.
- Produces: `GET /api/v1/classes/{id}/roster-report` → `{ roster: RosterReportEntry[] }`; `func weekWindow(now time.Time) (start, end time.Time)` (Mon 00:00 local … +7d, half-open).

- [ ] **Step 1: Write the failing handler test**

Create `apps/api/internal/api/teacher_read_test.go`. Use the existing api-package handler test harness (grep a sibling `*_test.go` in `internal/api` for how it builds an `*API` with a testcontainer DB + an authed request context via `UserFromContext`/session). Assert:
1. **Happy path:** teacher of the class → 200, JSON `{roster:[...]}` with each entry carrying `id, displayName, avatarColor, dBadge, aBadge, activeDays, turns, hasReport, unrated`. A student with a seeded project eval has `dBadge` like `L2–L4`, `unrated=false`; a student with none has `dBadge="—"`, `unrated=true`.
2. **Cross-class 404:** a teacher of a different class → 404.
3. **Student 403:** the route is under `RequireRole("teacher","admin")`; assert a student user is rejected (403) — either via the middleware test or by asserting the wiring in api.go (see Step 4 note).

- [ ] **Step 2: Run test to verify it fails**

Run: `cd /Users/houyuxin/08Coding/mind-imprint/apps/api && DOCKER_HOST=unix:///var/run/docker.sock CGO_ENABLED=0 go test ./internal/api/ -run TestRosterReport`
Expected: FAIL (handler not defined). (Full-package run happens at Step 5.)

- [ ] **Step 3: Write the handler**

Create `apps/api/internal/api/teacher_read.go`:

```go
package api

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/google/uuid"

	"mindimprint/api/internal/agent"
	"mindimprint/api/internal/httpx"
	"mindimprint/api/internal/store/sqlc"
	"mindimprint/api/internal/teacher"
)

// RosterReportEntry is one student row in the teacher's 全部学生 view: usage
// facts + derived D/A badges. Badges are display summaries (RL-5), recomputed
// from the latest project report on every read.
type RosterReportEntry struct {
	ID          string `json:"id"`
	DisplayName string `json:"displayName"`
	AvatarColor string `json:"avatarColor"`
	DBadge      string `json:"dBadge"`
	ABadge      string `json:"aBadge"`
	ActiveDays  int32  `json:"activeDays"`
	Turns       int32  `json:"turns"`
	HasReport   bool   `json:"hasReport"`
	Unrated     bool   `json:"unrated"`
}

// weekWindow returns the half-open [Mon 00:00, next Mon 00:00) window enclosing
// now, in now's location. D1 uses the current week only (no deltas — that is D2).
func weekWindow(now time.Time) (time.Time, time.Time) {
	y, m, d := now.Date()
	midnight := time.Date(y, m, d, 0, 0, 0, 0, now.Location())
	offset := (int(now.Weekday()) + 6) % 7 // Monday=0
	start := midnight.AddDate(0, 0, -offset)
	return start, start.AddDate(0, 0, 7)
}

func (a *API) getClassRosterReport(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		httpx.WriteError(w, r, httpx.ErrNotFound("资源不存在"))
		return
	}
	if _, err := a.assertTeacherOwnsClass(r.Context(), id); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	start, end := weekWindow(time.Now())
	rows, err := a.d.Queries.ListClassRosterReport(r.Context(), sqlc.ListClassRosterReportParams{
		ClassID: id, WeekStart: pgTimestamp(start), WeekEnd: pgTimestamp(end),
	})
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	out := make([]RosterReportEntry, 0, len(rows))
	for _, row := range rows {
		e := RosterReportEntry{
			ID: row.ID.String(), DisplayName: row.DisplayName, AvatarColor: row.AvatarColor,
			ActiveDays: row.ActiveDays, Turns: row.Turns, HasReport: row.HasReport,
			DBadge: "—", ABadge: "—", Unrated: true,
		}
		if len(row.LatestProjectScores) > 0 {
			var rep agent.Report
			if json.Unmarshal(row.LatestProjectScores, &rep) == nil {
				e.DBadge, e.ABadge, e.Unrated = teacher.DBadge(rep), teacher.ABadge(rep), false
			}
		}
		out = append(out, e)
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"roster": out})
}
```

Notes for the implementer:
- Confirm the sqlc types: `row.AvatarColor` (string), `row.ActiveDays`/`row.Turns` (int32), `row.LatestProjectScores` ([]byte, nil when NULL), `row.HasReport` (bool). Adjust field types to match generated code.
- `pgTimestamp` / the `@week_start` param type: match how other queries pass a `time.Time` to a `timestamptz` param in this codebase (grep for `pgtype.Timestamptz` usage; if params are plain `time.Time`, pass `start`/`end` directly and drop the helper).
- `avatar_color` column: confirm it is NOT NULL in `users` (seed sets it). If nullable in the generated row, coalesce.

- [ ] **Step 4: Register the route**

In `apps/api/internal/api/api.go`, in the `teacherOrAdmin` block (after line 128):

```go
	mux.Handle("GET /api/v1/classes/{id}/roster-report", teacherOrAdmin(a.getClassRosterReport))
```

(The `RequireRole("teacher","admin")` wrapper gives the 403-for-students guarantee; no separate check needed.)

- [ ] **Step 5: Run FULL api package + verify**

Run: `cd /Users/houyuxin/08Coding/mind-imprint/apps/api && DOCKER_HOST=unix:///var/run/docker.sock CGO_ENABLED=0 go test -p 1 ./internal/api/...`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add apps/api/internal/api/teacher_read.go apps/api/internal/api/teacher_read_test.go apps/api/internal/api/api.go
git commit -m "feat(d1): GET class roster-report endpoint with D/A badges"
```

---

## Task 4: Student-detail endpoint

**Files:**
- Modify: `apps/api/internal/api/teacher_read.go` (add handler + DTOs)
- Modify: `apps/api/internal/api/api.go` (route)
- Modify: `apps/api/internal/api/teacher_read_test.go` (add cases)

**Interfaces:**
- Consumes: `assertTeacherOwnsClass`, `Queries.GetStudentUsageForTeacher`, `ListStudentReportsForTeacher`, `ListStudentProjectsForTeacher`, and a class-membership check for the target student.
- Produces: `GET /api/v1/classes/{id}/students/{userId}` → `{ student: StudentHeadDTO, usage: UsageDTO, records: RecordDTO[] }`.

**Membership guard:** after `assertTeacherOwnsClass`, verify `userId` is a student member of the class before reading their data. Use `GetEnrollment(userID, classID)` (`org.sql:4`) and require `RoleInClass=="student"`; else 404.

- [ ] **Step 1: Add failing test cases**

Extend `teacher_read_test.go`: teacher opens a member student → 200 with `usage.activeDays`/`turns`/`reportCount`/`courseCount` and a `records` array (project rows incl. one without a report → `hasReport:false`; a course/chat report row → `hasReport:true`). Cross-class student or non-member → 404.

- [ ] **Step 2: Run to verify fail**

Run: `cd /Users/houyuxin/08Coding/mind-imprint/apps/api && DOCKER_HOST=unix:///var/run/docker.sock CGO_ENABLED=0 go test ./internal/api/ -run TestStudentDetail`
Expected: FAIL.

- [ ] **Step 3: Implement**

Add to `teacher_read.go`:

```go
type StudentHeadDTO struct {
	ID          string `json:"id"`
	DisplayName string `json:"displayName"`
	AvatarColor string `json:"avatarColor"`
	DBadge      string `json:"dBadge"`
	ABadge      string `json:"aBadge"`
	Unrated     bool   `json:"unrated"`
}

type UsageDTO struct {
	ActiveDays  int32 `json:"activeDays"`
	Turns       int32 `json:"turns"`
	ReportCount int   `json:"reportCount"`
	CourseCount int   `json:"courseCount"`
}

// RecordDTO is one row in the student's 平台使用记录 list.
type RecordDTO struct {
	Surface   string `json:"surface"` // project|course|chat
	ScopeID   string `json:"scopeId"`
	Title     string `json:"title"`
	Date      string `json:"date"`
	Status    string `json:"status"`
	HasReport bool   `json:"hasReport"`
}

func (a *API) getStudentDetail(w http.ResponseWriter, r *http.Request) {
	classID, userID, ok := a.authTeacherStudent(w, r) // helper below: asserts class ownership + student membership, writes 404 on failure
	if !ok {
		return
	}
	start, end := weekWindow(time.Now())
	usage, err := a.d.Queries.GetStudentUsageForTeacher(r.Context(), sqlc.GetStudentUsageForTeacherParams{
		UserID: userID, WeekStart: pgTimestamp(start), WeekEnd: pgTimestamp(end),
	})
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	reports, err := a.d.Queries.ListStudentReportsForTeacher(r.Context(), userID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	projects, err := a.d.Queries.ListStudentProjectsForTeacher(r.Context(), userID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	// Merge: project rows (report-or-not) + course/chat report rows. Projects that
	// already have a report come from `reports`; dedupe by scopeId so a project is
	// listed once. Build records, count reports + course reports for the head cards.
	// (Implementer: assemble RecordDTO list — projects without a report get
	// Status "进行中"/hasReport:false; report rows get "能力报告已生成"/true.)
	// DBadge/ABadge for the head: derive from the latest project report (reuse the
	// roster query's latest-scores, or fetch the latest project eval here).
	...
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"student": head, "usage": usage2, "records": records})
}
```

Add the shared guard helper `authTeacherStudent(w, r) (classID, userID uuid.UUID, ok bool)` in `teacher_read.go`: parses both path values, `assertTeacherOwnsClass`, then `GetEnrollment` requiring `role_in_class='student'`, writing 404 on any failure. Refactor Task 3's handler to reuse the class-parse part if clean.

For the head D/A badge: reuse `GetStudentProjectEvaluationForTeacher` (Task 5 adds it) OR the latest-project-scores subquery. To avoid ordering coupling, in this task fetch the latest project eval via a small inline call and derive badges (or leave head badges from the roster call on the web side — **decision: web StudentDetailView receives the D/A badge via this endpoint's `student` object**, so derive here).

- [ ] **Step 4: Register route**

```go
	mux.Handle("GET /api/v1/classes/{id}/students/{userId}", teacherOrAdmin(a.getStudentDetail))
```

- [ ] **Step 5: Run FULL api package**

Run: `cd /Users/houyuxin/08Coding/mind-imprint/apps/api && DOCKER_HOST=unix:///var/run/docker.sock CGO_ENABLED=0 go test -p 1 ./internal/api/...`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add apps/api/internal/api/teacher_read.go apps/api/internal/api/teacher_read_test.go apps/api/internal/api/api.go
git commit -m "feat(d1): GET student-detail endpoint (usage + records)"
```

---

## Task 5: Report-read endpoint (teacher report DTO + RQ context)

**Files:**
- Modify: `apps/api/internal/api/teacher_read.go`
- Modify: `apps/api/internal/api/api.go`
- Modify: `apps/api/internal/api/teacher_read_test.go`

**Interfaces:**
- Consumes: `authTeacherStudent`, the three `GetStudent*EvaluationForTeacher` queries, `studio.ToReportDTO`, `agent.Report`.
- Produces: `GET /api/v1/classes/{id}/students/{userId}/reports/{surface}/{scopeId}` → `TeacherReportDTO{ report: studio.ReportDTO, context: ReportContext }`.

- [ ] **Step 1: Add failing test cases**

Project report → 200 with `report.depthAxis` (6), `report.officialProjection` non-null, and `context.researchQuestion` populated (from the `research_question` graph node, else project title). A course/chat report → 200 with core-only report (`officialProjection` null) and empty `context`. Wrong owner/scope → 404. Unknown surface → 404.

- [ ] **Step 2: Run to verify fail**

Run: `cd /Users/houyuxin/08Coding/mind-imprint/apps/api && DOCKER_HOST=unix:///var/run/docker.sock CGO_ENABLED=0 go test ./internal/api/ -run TestStudentReport`
Expected: FAIL.

- [ ] **Step 3: Implement**

```go
type ReportContext struct {
	ProjectTitle     string `json:"projectTitle,omitempty"`
	ResearchQuestion string `json:"researchQuestion,omitempty"`
}

type TeacherReportDTO struct {
	Report  studio.ReportDTO `json:"report"`
	Context ReportContext    `json:"context"`
}

func (a *API) getStudentReport(w http.ResponseWriter, r *http.Request) {
	_, userID, ok := a.authTeacherStudent(w, r)
	if !ok {
		return
	}
	surface := r.PathValue("surface")
	scopeID, err := uuid.Parse(r.PathValue("scopeId"))
	if err != nil {
		httpx.WriteError(w, r, httpx.ErrNotFound("资源不存在"))
		return
	}
	var scores []byte
	var createdAt time.Time
	var ctx ReportContext
	switch surface {
	case "project":
		row, e := a.d.Queries.GetStudentProjectEvaluationForTeacher(r.Context(), sqlc.GetStudentProjectEvaluationForTeacherParams{ScopeID: scopeID, UserID: userID})
		if e != nil { notFoundOr(w, r, e); return }
		scores, createdAt = row.Scores, row.CreatedAt
		ctx.ProjectTitle = row.ProjectTitle
		ctx.ResearchQuestion = rqFromNode(row.RqBody, row.ProjectTitle) // parse body->>'text' or 'question', fallback title
	case "course":
		row, e := a.d.Queries.GetStudentSessionEvaluationForTeacher(r.Context(), sqlc.GetStudentSessionEvaluationForTeacherParams{ScopeID: scopeID, UserID: userID})
		if e != nil { notFoundOr(w, r, e); return }
		scores, createdAt = row.Scores, row.CreatedAt
	case "chat":
		row, e := a.d.Queries.GetStudentThreadEvaluationForTeacher(r.Context(), sqlc.GetStudentThreadEvaluationForTeacherParams{ScopeID: scopeID, UserID: userID})
		if e != nil { notFoundOr(w, r, e); return }
		scores, createdAt = row.Scores, row.CreatedAt
	default:
		httpx.WriteError(w, r, httpx.ErrNotFound("资源不存在"))
		return
	}
	var rep agent.Report
	if err := json.Unmarshal(scores, &rep); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, TeacherReportDTO{Report: studio.ToReportDTO(rep, createdAt), Context: ctx})
}
```

Implementer adds: `notFoundOr(w,r,err)` (maps `pgx.ErrNoRows`→404 else 500, mirroring `assertTeacherOwnsClass`), and `rqFromNode(body []byte, fallback string) string` (unmarshal the `research_question` node body; try `text` then `question` key; fallback to project title; empty stays empty for 敢于空白). Confirm the actual `research_question` node body shape by grepping where such nodes are written (`grep -rn "research_question" internal/`).

- [ ] **Step 4: Register route + run FULL package**

```go
	mux.Handle("GET /api/v1/classes/{id}/students/{userId}/reports/{surface}/{scopeId}", teacherOrAdmin(a.getStudentReport))
```

Run: `cd /Users/houyuxin/08Coding/mind-imprint/apps/api && DOCKER_HOST=unix:///var/run/docker.sock CGO_ENABLED=0 go test -p 1 ./internal/api/...`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add apps/api/internal/api/teacher_read.go apps/api/internal/api/teacher_read_test.go apps/api/internal/api/api.go
git commit -m "feat(d1): GET student-report endpoint (teacher DTO + RQ context)"
```

---

## Task 6: Seed migration — teacher + class of 9 with 3 CASE evaluations

**Files:**
- Create: `apps/api/internal/store/migrations/0029_seed_teacher_class.sql`

**Interfaces:** consumes existing tables. Produces fixed-UUID rows a dev/test can reference.

**Context:** today there is 1 student (Phoebe), 0 teachers. This seeds the dc.html class `IBDP 一年级 · 研究组` (吴老师 + 9 students), with 林知远=high / 沈亦然=mid / 周子墨=low carrying full project evaluations. Follow `0002_seed.sql` exactly for style (fixed UUIDs, `+goose Up`/`Down`, idempotent-friendly, reversible Down deleting children-first).

- [ ] **Step 1: Author the seed**

Create `0029_seed_teacher_class.sql`. Under school `...001`:
- **New class** (fixed UUID, e.g. `...0902`): `IBDP 一年级 · 研究组`, join_code `RESEARCH-01`.
- **1 teacher** user (`...0910`, role `'teacher'`, `email_verified_at=now()`, `password_hash='SEED_NO_LOGIN'`, display_name `吴老师`, avatar_color set) + enrollment `role_in_class='teacher'`.
- **9 student** users (`...0911`..`...0919`, role `'student'`, verified, avatar_color per dc.html ROSTER colors) named 林知远/沈亦然/周子墨/陈屿/吴桐/许清/何知/苏晚/罗一, each enrolled `role_in_class='student'`.
- **3 project rows** (林/沈/周) with `last_active_at=now()`.
- **3 evaluations rows** (`project_id` set, `status='done'`, `model`/`tier` any non-null, token/cost 0), `scores` = the CASE→`agent.Report` JSON (Step 2).
- **A `research_question` graph_node** per of the 3 projects (body `{"text": "<CASE.rq>"}`) so the report 总览 RQ populates.
- Optionally a few `event` rows (type `prompt_sent`, recent `created_at`) for 林/沈/周/吴/许/何/苏 so roster usage is non-zero; 罗一 gets none (unrated + 0 usage empty-state).

`Down`: delete events → graph_node → evaluations → project → enrollments → users → class, children-first, by the fixed UUIDs.

- [ ] **Step 2: Transform the 3 CASES into `agent.Report` JSON**

Source: the `CASES` array in `docs/design/teacher end/project/思维印记 教师端.dc.html` (ids `low`/`mid`/`high`). Target: `agent.Report` (spec §8 mapping). Per case, produce full-coverage JSON:
- `depthAxis[]` ← `dAxis[]`: `{code:"D1".."D6", name:<dimension>, level:<L-level; take the base level, e.g. "L3" from "L3-L4">, levelRange:<"L3–L4" when the case gives a range, else omit>, evidence:<evidence>, promptEvidence:""}`.
- `autonomyAxis[]` ← `aAxis[]`: `{code:"A1".."A6", name:<dimension>, level:<int from "N级">, opportunity:<"given_taken" when the evidence shows the student took a supplied chance; "given_not_taken" when supplied-but-not; "not_supplied" for 0级 with no opportunity>, evidence:<evidence>, promptEvidence:""}`.
- `promptLens` ← `prompt[]`: `{stats:[{label:"提示词平均",value:<avg>},{label:"A 轴平均",value:<aAvg>},{label:"D 轴状态",value:<dOverall>}], lenses:[{code:"L1".."L6"? use lens index, name:<lens>, level:<int>, evidence:<evidence>}], note:""}`. (Use codes `PL1..PL6` or the lens names as `name`; `code` just needs to be a stable unique key.)
- `interactionEvidence[]` ← `interaction[]`: `{round:<int>, student:<student>, aiSummary:<ai>, signal:<signal>}`.
- `narrative` ← the `最终训练判定` reason (or `profile`).
- `guidance.nextSteps[]` ← `nextSteps[]`.
- `axiom`: `"两轴永不合成总分；单次会话为事件级证据，不构成人级档位判定。"`.
- `officialProjection`: `{standard:{id:"ap-research",name:"AP Research"}, components:<official[]→{name:component,judgement,reason}>, alignment:<officialAlignment[]→{item,standard,performance,impact}>, readiness:{score:<round(scoreValue)>, note:"仅表示作品就绪度，不与 D/A 双轴合成总分。"}}`.
- `workAndProcess`: `{workSamples:<workSample[]→{title:"作品片段 N",text}>, processMaterials:<process[]→{name,status,diagnosis}>}`.

Sanity: each report must have exactly 6 depth + 6 autonomy + 6 lenses (seed bypasses Go normalizers). Paste the JSON inline in the SQL (`scores` jsonb literal).

- [ ] **Step 3: Run migrations + a read verification test**

Run the FULL api + store suite (migrations run in testcontainers):
`cd /Users/houyuxin/08Coding/mind-imprint/apps/api && DOCKER_HOST=unix:///var/run/docker.sock CGO_ENABLED=0 go test -p 1 ./...`
Expected: PASS (no migration or JSON-unmarshal failure). If a `store` or `api` test asserts a fixed roster/student count for school `...001`, update it for the added rows.

Add one focused assertion (in `teacher_read_test.go` or `teacher_query_test.go`): after migrations, `ListClassRosterReport(<seeded class ...0902>)` returns 9 rows, exactly 3 with `has_report=true`, and `GetStudentProjectEvaluationForTeacher` for 林知远's project unmarshals to a Report with 6 depth dims.

- [ ] **Step 4: Commit**

```bash
git add apps/api/internal/store/migrations/0029_seed_teacher_class.sql apps/api/internal/api/teacher_read_test.go
git commit -m "feat(d1): seed teacher + 9-student class with 3 CASE evaluations"
```

---

## Task 7: Web API client (teacher.ts)

**Files:**
- Create: `apps/web/src/api/teacher.ts`
- Modify: `apps/web/src/api/index.ts` (aggregate into `ApiClient`)
- Test: `apps/web/test/api/teacher.test.ts` (if the api dir has a test pattern; else covered via view tests)

**Interfaces:**
- Produces: `getClassRosterReport(classId)`, `getStudentDetail(classId, userId)`, `getStudentReport(classId, userId, surface, scopeId)` + TS types mirroring the Go DTOs (`RosterReportEntry`, `StudentDetail`, `TeacherReport`). The report body type reuses `DualAxisReport` from `@mind-imprint/contracts`.

- [ ] **Step 1: Write the client + types**

```ts
import { apiFetch } from "./client";
import type { DualAxisReport } from "@mind-imprint/contracts";

export interface RosterReportEntry {
  id: string; displayName: string; avatarColor: string;
  dBadge: string; aBadge: string; activeDays: number; turns: number;
  hasReport: boolean; unrated: boolean;
}
export interface StudentRecord {
  surface: "project" | "course" | "chat";
  scopeId: string; title: string; date: string; status: string; hasReport: boolean;
}
export interface StudentDetail {
  student: { id: string; displayName: string; avatarColor: string; dBadge: string; aBadge: string; unrated: boolean };
  usage: { activeDays: number; turns: number; reportCount: number; courseCount: number };
  records: StudentRecord[];
}
export interface TeacherReport {
  report: DualAxisReport;
  context: { projectTitle?: string; researchQuestion?: string };
}

export async function getClassRosterReport(classId: string): Promise<RosterReportEntry[]> {
  const r = await apiFetch<{ roster: RosterReportEntry[] }>(`/api/v1/classes/${classId}/roster-report`);
  return r.roster;
}
export async function getStudentDetail(classId: string, userId: string): Promise<StudentDetail> {
  return apiFetch<StudentDetail>(`/api/v1/classes/${classId}/students/${userId}`);
}
export async function getStudentReport(classId: string, userId: string, surface: string, scopeId: string): Promise<TeacherReport> {
  return apiFetch<TeacherReport>(`/api/v1/classes/${classId}/students/${userId}/reports/${surface}/${scopeId}`);
}
```

- [ ] **Step 2: Aggregate into ApiClient**

In `apps/web/src/api/index.ts`, export the three fns and add them to the `ApiClient` type/object exactly as the existing `classes.ts` fns are wired (grep `getClass` in `index.ts` to match the pattern). Add them to `ConsoleClient`'s `Pick<...>` in `ConsoleShell.tsx`.

- [ ] **Step 3: Typecheck**

Run: `cd /Users/houyuxin/08Coding/mind-imprint/apps/web && npx tsc --noEmit`
Expected: no errors.

- [ ] **Step 4: Commit**

```bash
git add apps/web/src/api/teacher.ts apps/web/src/api/index.ts apps/web/src/console/ConsoleShell.tsx
git commit -m "feat(d1): web api client for teacher read-path"
```

---

## Task 8: Roster-report table + clickable rows

**Files:**
- Modify: `apps/web/src/console/ClassDetailView.tsx`
- Modify: `apps/web/src/console/ConsoleShell.tsx` (add `openStudentId` state)
- Test: `apps/web/test/console/ClassDetailView.test.tsx`

**Interfaces:**
- Consumes: `getClassRosterReport`, `onOpenStudent(userId)`.
- Produces: the 全部学生 table (D/A badges + 本周活跃天 + 对话轮次 + 报告状态), rows clickable.

**Design:** the enriched roster table replaces the current bare table (`ClassDetailView.tsx:226-267`). Keep the existing class-management controls (rename/join-code/teacher-assign) above. Fetch `getClassRosterReport(classId)` in addition to `getClass`. Badge colors follow dc.html `levelColor`/`tint` (`dc.html:1513`): L1/0–1→`#C4574D`, L2/2→`#C68A3A`, L3/3→`#3E7CA8`, L4/4–5→`#3E8A6E`, `—`→`#8A92A3`; tint background per `dc.html:1514`.

- [ ] **Step 1: Write the failing test**

`ClassDetailView.test.tsx`: mock a client whose `getClassRosterReport` returns two entries (one rated `dBadge:"L3–L4"`, `aBadge:"4.2"`, `hasReport:true`; one `unrated:true`, `dBadge:"—"`). Assert both names render, the badges render, and clicking a row calls `onOpenStudent` with the id. (Add `onOpenStudent` to the component props.)

- [ ] **Step 2: Run to verify fail**

Run: `cd /Users/houyuxin/08Coding/mind-imprint/apps/web && npm test -- ClassDetailView`
Expected: FAIL.

- [ ] **Step 3: Implement**

- Add `onOpenStudent: (userId: string) => void` to props.
- Add a `rosterReport` state; in `load()`, also `client.getClassRosterReport(classId).then(setRosterReport)`.
- Replace the `<table>` body: columns 学生（avatar+name）· 本周活跃（`{activeDays} 天`）· 对话轮次（`turns`）· D 轴（badge chip）· A 轴（badge chip）· 能力报告（`hasReport ? "✓ 已生成" : "—"`）. Row `onClick={() => onOpenStudent(s.id)}`, `cursor:pointer`. Add a small `badgeColor(text)` helper mirroring `dc.html:1513`.
- Keep management controls + teacher-assign block unchanged. Remove the "行不可点入" caption (`:268`).

- [ ] **Step 4: Wire ConsoleShell**

Add `const [openStudentId, setOpenStudentId] = useState<string | null>(null)`. Pass `onOpenStudent={setOpenStudentId}` to `ClassDetailView`. When `openStudentId != null` render `StudentDetailView` (Task 9) instead of `ClassDetailView`. Reset `openStudentId` when leaving the class or tab.

- [ ] **Step 5: Run web tests + typecheck**

Run: `cd /Users/houyuxin/08Coding/mind-imprint/apps/web && npm test && npx tsc --noEmit`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add apps/web/src/console/ClassDetailView.tsx apps/web/src/console/ConsoleShell.tsx apps/web/test/console/ClassDetailView.test.tsx
git commit -m "feat(d1): enriched roster table with D/A badges + clickable rows"
```

---

## Task 9: StudentDetailView

**Files:**
- Create: `apps/web/src/console/StudentDetailView.tsx`
- Test: `apps/web/test/console/StudentDetailView.test.tsx`
- Modify: `apps/web/src/console/ConsoleShell.tsx` (render it + `openReport` state)

**Interfaces:**
- Consumes: `getStudentDetail(classId, userId)`, `onBack()`, `onOpenReport(surface, scopeId)`.
- Produces: the 学生个人页 (header D/A + 4 usage cards + records tabs + inert export buttons).

**Design (dc.html 265-344):** header = avatar + name + D/A badge chips; 4 usage cards (本周活跃天数/AI 对话轮次/生成能力报告/完成课程节); records tabs 全部/课程/对话/项目; record rows with type chip + title + `date · status` + `查看报告` (when `hasReport`) → `onOpenReport`. Export buttons (导出家长版·项目/阶段) are inert placeholders (title tooltip "家长版报告即将上线", → Spec E).

- [ ] **Step 1: Write the failing test**

Mock `getStudentDetail` returning a student with `dBadge/aBadge`, usage numbers, and 3 records (one project `hasReport:true`, one chat `hasReport:false`, one course). Assert: name + badges render; 4 usage values render; tab filtering works (click 项目 → only project rows); clicking a `hasReport` record's 查看报告 calls `onOpenReport("project", scopeId)`; a `hasReport:false` row shows no report link.

- [ ] **Step 2: Run to verify fail**

Run: `cd /Users/houyuxin/08Coding/mind-imprint/apps/web && npm test -- StudentDetailView`
Expected: FAIL.

- [ ] **Step 3: Implement** the view (props `{ client, classId, userId, onBack, onOpenReport }`), loading on mount, rendering header/usage/tabs/records per the design; inline SVG icons only; loading + error states mirroring `ClassDetailView`.

- [ ] **Step 4: Wire ConsoleShell** — add `const [openReport, setOpenReport] = useState<{surface:string;scopeId:string}|null>(null)`; render `StudentDetailView` when `openStudentId && !openReport`, passing `onBack={() => setOpenStudentId(null)}` and `onOpenReport={(surface,scopeId)=>setOpenReport({surface,scopeId})}`; render `TeacherReportView` (Task 10) when `openReport` set.

- [ ] **Step 5: Run web tests + typecheck** — `npm test && npx tsc --noEmit`. Expected PASS.

- [ ] **Step 6: Commit**

```bash
git add apps/web/src/console/StudentDetailView.tsx apps/web/test/console/StudentDetailView.test.tsx apps/web/src/console/ConsoleShell.tsx
git commit -m "feat(d1): student detail page (usage + records)"
```

---

## Task 10: TeacherReportView (deep report, 7 non-map sections)

**Files:**
- Create: `apps/web/src/console/TeacherReportView.tsx`
- Test: `apps/web/test/console/TeacherReportView.test.tsx`

**Interfaces:**
- Consumes: `getStudentReport(classId, userId, surface, scopeId)` → `TeacherReport`, `onBack()`.
- Produces: the teacher deep report per dc.html (总览/官方投影/双轴读数/[证据地图 slot for Task 11]/交互证据/提示词透镜/作品与过程/下一步) + catalog nav + breadcrumb.

**Note — deliberately NOT reusing `DualAxisReport`:** the student report (`apps/web/src/shell/report/DualAxisReport.tsx`) is a different projection — different section order, narrow single-column layout, no catalog/RQ-overview/evidence-map. The teacher view is its own composition against the SAME `DualAxisReport` contract type. Sharing would fight both layouts; this is distinct projection, not duplication. Leave `DualAxisReport.tsx` untouched (student render unchanged).

**Design (dc.html 346-591):** fixed catalog nav (right), header (breadcrumb 全部学生/name + name·title + chips + inert 导出家长版 PDF button), then sections:
- **总览**: RQ (`context.researchQuestion`) + 研究概况 (`report.narrative`, or omit if empty — 敢于空白) + 训练折算 (`report.officialProjection?.readiness.score`/100 + note) + 最终训练判定 (derive from narrative or omit). Only render officialProjection-derived blocks when it is non-null.
- **官方投影** (project only): `officialProjection.components` cards + `alignment` rows (要求/表现/影响). Omit whole section if null.
- **双轴读数**: D-axis 6 cards (name + level badge + evidence) + A-axis 6 cards (name + Lv badge + evidence). Badge colors per `dc.html:1513`.
- **交互证据**: `interactionEvidence` rounds.
- **提示词透镜**: `promptLens.stats` (3) + `promptLens.lenses` (6) cards + note.
- **作品与过程** (project only): `workAndProcess.workSamples` + `processMaterials`. Omit if null.
- **下一步**: `guidance.nextSteps`.
- Evidence-map section: render a placeholder `<div ref>` slot with heading 证据地图; Task 11 fills it.

- [ ] **Step 1: Write the failing test**

Mock `getStudentReport` returning a project `TeacherReport` (officialProjection + workAndProcess present, 6 depth, 6 autonomy, `context.researchQuestion` set). Assert: breadcrumb + RQ render; all 6 D + 6 A dimension names render; readiness `NN / 100` renders; next-steps render; 官方投影 + 作品与过程 sections render. Second case: a core-only report (officialProjection null) → 官方投影 + 作品与过程 sections are ABSENT, core sections present.

- [ ] **Step 2: Run to verify fail** — `npm test -- TeacherReportView`. Expected FAIL.

- [ ] **Step 3: Implement** the view (props `{ client, classId, userId, surface, scopeId, onBack }`), loading on mount. Reuse small idioms from `DualAxisReport.tsx` (badge chips, evidence rows) re-authored inline for the teacher layout. Catalog nav may be static anchors (smooth-scroll optional; a simple in-page nav is enough for D1). Inline SVG icons only.

- [ ] **Step 4: Run web tests + typecheck** — `npm test && npx tsc --noEmit`. Expected PASS.

- [ ] **Step 5: Commit**

```bash
git add apps/web/src/console/TeacherReportView.tsx apps/web/test/console/TeacherReportView.test.tsx
git commit -m "feat(d1): teacher deep report view (7 sections + catalog)"
```

---

## Task 11: EvidenceMap (client-derived 7-node map)

**Files:**
- Create: `apps/web/src/console/EvidenceMap.tsx`
- Test: `apps/web/test/console/EvidenceMap.test.tsx`
- Modify: `apps/web/src/console/TeacherReportView.tsx` (mount it in the 证据地图 slot)

**Interfaces:**
- Consumes: `TeacherReport` (report + context). Pure — NO network.
- Produces: `<EvidenceMap report={...} context={...} />` — fixed 7-node graph + click-to-detail.

**Design (dc.html 472-496 + reportVals 1613-1628):** fixed nodes derived entirely from the report object:
- `center` — 研究概况 (context.researchQuestion or narrative)
- `rq` — Research Question (context.researchQuestion)
- `official` — 官方作品投影 (readiness summary) — **only when officialProjection present**
- `dAxis` — D 轴 (DBadge-style summary of depth levels)
- `aAxis` — A 轴 (ABadge-style summary)
- `prompt` — 提示词透镜 (lens summary)
- `ai` — AI 互动证据 (`{interactionEvidence.length} 轮交互`)

Fixed edge topology (dc.html:1626). Clicking a node shows its `detail` in a panel below. For core-only reports (no officialProjection), omit the `official` node (and its edges) OR render the map without it — **decision: render the map, omit the `official` node**, since the other 6 nodes are meaningful for chat/course too. (Spec §5.5's "core reports don't render the map" default is overridden here per this task — the map is cheap and useful; confirm with the reviewer if the spec default is preferred.)

- [ ] **Step 1: Write the failing test**

Given a project report, assert 7 node labels render and clicking `ai` shows a detail containing the interaction count. Given a core-only report (officialProjection null), assert the `official` node is absent and the other 6 render.

- [ ] **Step 2: Run to verify fail** — `npm test -- EvidenceMap`. Expected FAIL.

- [ ] **Step 3: Implement** the pure component (fixed node/edge arrays, click state, detail panel; positions/styles per dc.html). No network, no props beyond the report + context.

- [ ] **Step 4: Mount in TeacherReportView** — render `<EvidenceMap>` in the 证据地图 slot.

- [ ] **Step 5: Run FULL web suite + typecheck** — `cd /Users/houyuxin/08Coding/mind-imprint/apps/web && npm test && npx tsc --noEmit`. Expected PASS.

- [ ] **Step 6: Commit**

```bash
git add apps/web/src/console/EvidenceMap.tsx apps/web/test/console/EvidenceMap.test.tsx apps/web/src/console/TeacherReportView.tsx
git commit -m "feat(d1): client-derived evidence map, wired into teacher report"
```

---

## Final Verification (before whole-branch review)

- [ ] Go FULL suite: `cd /Users/houyuxin/08Coding/mind-imprint/apps/api && DOCKER_HOST=unix:///var/run/docker.sock CGO_ENABLED=0 go test -p 1 ./...`
- [ ] Web: `cd /Users/houyuxin/08Coding/mind-imprint/apps/web && npm test && npx tsc --noEmit`
- [ ] Contracts (only if a Zod file was touched — D1 should NOT touch it): `cd /Users/houyuxin/08Coding/mind-imprint/packages/contracts && npm test && npx tsc --noEmit`
- [ ] Manual seam check: the three GET endpoints under `teacherOrAdmin`; a student token → 403; a cross-class teacher → 404.

## Self-Review Notes (plan author)

- **Spec coverage:** §5.1 queries → T2; §5.2 badges → T1; §5.3 report+RQ join → T5; §5.4 usage → T2/T3; §5.6 endpoints → T3/T4/T5; §6 web views → T8/T9/T10; §6.3 evidence map → T11; §8 seed → T6. Testing §11 folded into each task + Final Verification. ✓
- **Type consistency:** `RosterReportEntry`/`StudentDetail`/`TeacherReportDTO` Go DTOs ↔ `teacher.ts` TS types are camelCase-aligned; report body is `studio.ReportDTO` ↔ `DualAxisReport` contract type (unchanged). Badge fns `DBadge`/`ABadge` referenced identically in T1/T3/T4.
- **One open confirmation for the reviewer/human:** T11 overrides spec §5.5's "core reports don't render the evidence map" default (renders it minus the `official` node). Flag at review; trivially revertible to the spec default.
