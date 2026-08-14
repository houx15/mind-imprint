# 教师端迁移到活动指标 + 两视图 + 设计令牌 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Rebuild the teacher console to run entirely on cheap no-LLM activity metrics (removing all D/A axis dependence on the now-unwritten `evaluations`), split the class page into a narrated last-completed-week report (View A) and a live DB-only roster (View B), and re-skin every console file onto the app's `mk-*` tokens + `@/ui` primitives.

**Architecture:** Backend first (queries → rule layer → composer → handlers → retirement), each compiler-gated; then frontend contracts, then the two-view UI + full token re-skin. The weekly narrative keeps its existing generate-once-per-(class,week) lifecycle (lazy on first open, cached forever) — only the *facts* change from axis to activity, and the *window* shifts from the in-progress week to the last completed week.

**Tech Stack:** Go (`net/http` + `pgx` + `sqlc` + `goose`), React + Vite + TS + Tailwind (`mk-*` tokens, `@/ui`). Spec: `docs/superpowers/specs/2026-08-14-teacher-end-activity-migration-design.md`.

## Global Constraints

- **Activity metrics only in the teacher path.** No `agent.Report`, no `teacher.DBadge/ABadge`, no `student_evaluation`/`evaluations` reads anywhere the teacher console touches. Every number comes from `event` / `project` / `evaluation_report` / `course_progress`.
- **KEEP (shared — do not touch):** `internal/agent/report_types.go` (`agent.Report` — still used by `studio.ReportDTO`), `internal/studio/report_dto.go`, `internal/rubric/`, `packages/contracts/src/dualaxis.json`. The `evaluations` / `student_evaluation` / `llm_usage` tables stay **physically intact** as a frozen cost ledger — the teacher path just stops reading them. No migration drops them.
- **Definitions (schema-verified):** active project = `project.status = 'active'`; course finished = `course_progress.completed_at IS NOT NULL`; report count = `evaluation_report.status = 'ready'` joined to the student's `project`.
- **sqlc regen:** `cd apps/api && make sqlc` (= `CGO_ENABLED=0 go tool sqlc generate`, pinned v1.27.0). After any `.sql` change, regen before building.
- **Tests:** run FULL Go packages (never `-run` alone for the final check), `go test ./... -timeout 1800s`; testcontainers must run in the **FOREGROUND** (backgrounding stalls them). Web: `npm run build` (tsc) + `npm test`.
- **Design:** use `mk-*` CSS variables + `@/ui`. For tint backgrounds use the **solid** accent steps (`--mk-accent-50/-100`), never `bg-mk-*/<opacity>` (alpha suffix emits transparent). Follow the four 铁律; no new visual language.
- **Never `git add -A`** — stage explicit paths.

---

## File Structure

**Backend (`apps/api`):**
- `internal/teacher/week.go` — MODIFY: add completed-week window + validation helpers.
- `internal/store/queries/teacher.sql` — MODIFY: replace `ListClassRosterReport` with count-based `ListClassRosterCounts`; add `GetClassLiveHeader`; repoint `ListStudentProjectsForTeacher`; delete `GetLatestReportScoresForStudent`, `ListStudentReportsForTeacher`.
- `internal/store/queries/teacher_weekly.sql` — MODIFY: repoint `GetClassWeekStats` reports arm; replace `ListClassStudentWindowUsage` with `ListClassStudentWeekActivity`; delete `ListClassRecentReports`.
- `internal/teacher/weekly.go` — REWRITE: activity-metric rule layer; delete axis code.
- `internal/teacher/badges.go` — DELETE.
- `internal/agent/compose_weekly.go` — MODIFY: facts/prompt/prose/validation drop axis.
- `internal/api/teacher_weekly.go` — MODIFY: completed-week window + `weekStart` param; DTO drops depth/autonomy.
- `internal/api/teacher_read.go` — MODIFY: roster handler → counts + live header; student-detail head drops badges; records repointed; remove dead `ReportContext`/`TeacherReportDTO` if unreferenced.
- `internal/store/migrations/0067_truncate_weekly_prose_cache.sql` — CREATE.
- Test files in `internal/teacher/`, `internal/agent/`, `internal/api/` — MODIFY to new shapes.

**Frontend (`apps/web`):**
- `src/api/teacher.ts` — MODIFY: new `RosterEntry`, `ClassLiveHeader`, `WeeklyReport` (no axis), `weekStart` param, `StudentDetail` (no badges).
- `src/console/ClassWeeklyView.tsx` — MODIFY: remove 班级思维维度; week nav; reskin.
- `src/console/ClassRosterTable.tsx` — MODIFY: new columns; reskin.
- `src/console/ClassDetailView.tsx` — MODIFY: live header + tab labels; reskin.
- `src/console/StudentDetailView.tsx` — MODIFY: drop D/A head badges; reskin.
- `src/console/TeacherReportView.tsx` — MODIFY: `generating` polling; reskin chrome.
- `src/console/ConsoleShell.tsx`, `ConsoleRail.tsx`, `ClassesView.tsx` — MODIFY: reskin.
- `src/console/badgeColor.ts` — DELETE.

---

## Task 1: Completed-week window helpers

**Files:**
- Modify: `apps/api/internal/teacher/week.go`
- Test: `apps/api/internal/teacher/week_test.go` (create if absent)

**Interfaces:**
- Produces: `LastCompletedWeekStart(now time.Time) time.Time`; `CompletedWeekWindows(weekStart time.Time) (start, end, prevStart, prevEnd time.Time)`; `ValidateCompletedWeekStart(weekStart, now time.Time) error`; `IsLatestCompletedWeek(weekStart, now time.Time) bool`. All UTC-pinned, Monday-aligned. Existing `WeekWindow`/`WeekLabel` unchanged.

- [ ] **Step 1: Write failing tests** in `week_test.go`:

```go
package teacher

import (
	"testing"
	"time"
)

func TestLastCompletedWeekStart(t *testing.T) {
	// Wed 2026-08-12 UTC → current week Mon 2026-08-10 → last completed Mon 2026-08-03.
	now := time.Date(2026, 8, 12, 9, 0, 0, 0, time.UTC)
	got := LastCompletedWeekStart(now)
	want := time.Date(2026, 8, 3, 0, 0, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Fatalf("LastCompletedWeekStart = %v, want %v", got, want)
	}
}

func TestCompletedWeekWindows(t *testing.T) {
	ws := time.Date(2026, 8, 3, 0, 0, 0, 0, time.UTC)
	start, end, ps, pe := CompletedWeekWindows(ws)
	if !start.Equal(ws) || !end.Equal(ws.AddDate(0, 0, 7)) {
		t.Fatalf("window = [%v,%v)", start, end)
	}
	if !ps.Equal(ws.AddDate(0, 0, -7)) || !pe.Equal(ws) {
		t.Fatalf("prev = [%v,%v)", ps, pe)
	}
}

func TestValidateCompletedWeekStart(t *testing.T) {
	now := time.Date(2026, 8, 12, 9, 0, 0, 0, time.UTC)
	if err := ValidateCompletedWeekStart(time.Date(2026, 8, 3, 0, 0, 0, 0, time.UTC), now); err != nil {
		t.Fatalf("valid last-completed week rejected: %v", err)
	}
	// Current (in-progress) week is not viewable.
	if err := ValidateCompletedWeekStart(time.Date(2026, 8, 10, 0, 0, 0, 0, time.UTC), now); err == nil {
		t.Fatal("current week accepted")
	}
	// Not a Monday.
	if err := ValidateCompletedWeekStart(time.Date(2026, 8, 4, 0, 0, 0, 0, time.UTC), now); err == nil {
		t.Fatal("non-Monday accepted")
	}
}

func TestIsLatestCompletedWeek(t *testing.T) {
	now := time.Date(2026, 8, 12, 9, 0, 0, 0, time.UTC)
	if !IsLatestCompletedWeek(time.Date(2026, 8, 3, 0, 0, 0, 0, time.UTC), now) {
		t.Fatal("last completed week should be latest")
	}
	if IsLatestCompletedWeek(time.Date(2026, 7, 27, 0, 0, 0, 0, time.UTC), now) {
		t.Fatal("older week should not be latest")
	}
}
```

- [ ] **Step 2: Run to verify failure**: `cd apps/api && go test ./internal/teacher/ -run 'CompletedWeek|LastCompleted|ValidateCompleted|IsLatest' -v` → FAIL (undefined).

- [ ] **Step 3: Implement** — append to `week.go`:

```go
// LastCompletedWeekStart is the Monday of the most recent FULLY-elapsed week:
// the current week's Monday minus seven days. This is View A's default window.
func LastCompletedWeekStart(now time.Time) time.Time {
	start, _ := WeekWindow(now)
	return start.AddDate(0, 0, -7)
}

// CompletedWeekWindows returns the full [Mon,next-Mon) window for a completed
// week and the full prior week used for deltas. Unlike PrevWindow (week-to-date
// vs same-elapsed-offset, for the live in-progress week), a completed week is
// compared full-week vs full-week — the week is over, so there is no "elapsed
// offset" to match.
func CompletedWeekWindows(weekStart time.Time) (start, end, prevStart, prevEnd time.Time) {
	weekStart = weekStart.UTC()
	start = weekStart
	end = weekStart.AddDate(0, 0, 7)
	prevStart = weekStart.AddDate(0, 0, -7)
	prevEnd = weekStart
	return
}

// ValidateCompletedWeekStart rejects a requested week that is not a UTC Monday
// midnight, or that is the current/future week (only completed weeks are
// viewable). now bounds the future edge.
func ValidateCompletedWeekStart(weekStart, now time.Time) error {
	weekStart = weekStart.UTC()
	y, m, d := weekStart.Date()
	if weekStart.Hour() != 0 || weekStart.Minute() != 0 || weekStart.Second() != 0 || weekStart.Nanosecond() != 0 ||
		!weekStart.Equal(time.Date(y, m, d, 0, 0, 0, 0, time.UTC)) || weekStart.Weekday() != time.Monday {
		return fmt.Errorf("teacher: weekStart must be a UTC Monday midnight")
	}
	curStart, _ := WeekWindow(now)
	if !weekStart.Before(curStart) {
		return fmt.Errorf("teacher: weekStart must be a completed week")
	}
	return nil
}

// IsLatestCompletedWeek reports whether weekStart is the most recent completed
// week (so the UI disables "next").
func IsLatestCompletedWeek(weekStart, now time.Time) bool {
	return weekStart.UTC().Equal(LastCompletedWeekStart(now))
}
```

- [ ] **Step 4: Run** `go test ./internal/teacher/ -timeout 300s` → PASS (existing `week` tests + new). Existing `weekly.go` still compiles here (badges/rules not yet touched).

- [ ] **Step 5: Commit** `git add apps/api/internal/teacher/week.go apps/api/internal/teacher/week_test.go && git commit -m "feat(teacher): completed-week window helpers for View A"`

---

## Task 2: Roster + live header on activity metrics

**Files:**
- Modify: `apps/api/internal/store/queries/teacher.sql`, `apps/api/internal/api/teacher_read.go`
- Regenerate: `apps/api/internal/store/sqlc/*`
- Test: `apps/api/internal/api/teacher_read_test.go` (update)

**Interfaces:**
- Produces: `GET /api/v1/classes/{id}/roster-report` now returns `{ "roster": RosterEntry[], "header": ClassLiveHeader }`. `RosterEntry = { id, displayName, avatarColor, activeProjects, reportCount, coursesFinished }`. `ClassLiveHeader = { classSize, activeStudents, activeProjects, turns, reports }`.

- [ ] **Step 1: Replace the roster query** in `teacher.sql` — delete `ListClassRosterReport` and add:

```sql
-- name: ListClassRosterCounts :many
-- One row per student: current-state activity counts, all no-LLM. active
-- projects = status='active'; report count = ready evaluation_report on the
-- student's projects; courses finished = course_progress.completed_at set.
-- Class-scoped like every teacher query (enrollments + role_in_class='student').
SELECT
  u.id, u.display_name, u.avatar_color,
  COALESCE(ap.n, 0)::int AS active_projects,
  COALESCE(rc.n, 0)::int AS report_count,
  COALESCE(cf.n, 0)::int AS courses_finished
FROM enrollments e
JOIN users u ON u.id = e.user_id
LEFT JOIN LATERAL (
  SELECT count(*) AS n FROM project p WHERE p.user_id = u.id AND p.status = 'active'
) ap ON true
LEFT JOIN LATERAL (
  SELECT count(*) AS n FROM evaluation_report er JOIN project p ON p.id = er.project_id
  WHERE p.user_id = u.id AND er.status = 'ready'
) rc ON true
LEFT JOIN LATERAL (
  SELECT count(*) AS n FROM course_progress cp WHERE cp.user_id = u.id AND cp.completed_at IS NOT NULL
) cf ON true
WHERE e.class_id = @class_id AND e.role_in_class = 'student'
ORDER BY u.display_name;

-- name: GetClassLiveHeader :one
-- Live class-level snapshot for View B's header. active_students/turns/reports
-- are windowed (the current in-progress week); active_projects is current state.
WITH members AS (
  SELECT u.id FROM enrollments e JOIN users u ON u.id = e.user_id
  WHERE e.class_id = @class_id AND e.role_in_class = 'student'
)
SELECT
  (SELECT count(*) FROM members)::int AS class_size,
  (SELECT count(DISTINCT ev.user_id) FROM event ev JOIN members m ON m.id = ev.user_id
     WHERE ev.created_at >= @week_start AND ev.created_at < @week_end)::int AS active_students,
  (SELECT count(*) FROM event ev JOIN members m ON m.id = ev.user_id
     WHERE ev.created_at >= @week_start AND ev.created_at < @week_end
       AND ev.type IN ('prompt_sent','course_message'))::int AS turns,
  (SELECT count(*) FROM project p JOIN members m ON m.id = p.user_id
     WHERE p.status = 'active')::int AS active_projects,
  (SELECT count(*) FROM evaluation_report er JOIN project p ON p.id = er.project_id
     JOIN members m ON m.id = p.user_id
     WHERE er.status = 'ready' AND er.created_at >= @week_start AND er.created_at < @week_end)::int AS reports;
```

- [ ] **Step 2: Regenerate sqlc** — `cd apps/api && make sqlc`. Verify `ListClassRosterCounts` / `GetClassLiveHeader` appear in `internal/store/sqlc/`.

- [ ] **Step 3: Rewrite the roster handler** in `teacher_read.go` — replace `RosterReportEntry` + `getClassRosterReport`:

```go
// RosterEntry is one student row in View B (实时 roster): current-state activity
// counts, all no-LLM. No axis, no badges.
type RosterEntry struct {
	ID              string `json:"id"`
	DisplayName     string `json:"displayName"`
	AvatarColor     string `json:"avatarColor"`
	ActiveProjects  int32  `json:"activeProjects"`
	ReportCount     int32  `json:"reportCount"`
	CoursesFinished int32  `json:"coursesFinished"`
}

// ClassLiveHeader is View B's light live class-level snapshot.
type ClassLiveHeader struct {
	ClassSize      int32 `json:"classSize"`
	ActiveStudents int32 `json:"activeStudents"`
	ActiveProjects int32 `json:"activeProjects"`
	Turns          int32 `json:"turns"`
	Reports        int32 `json:"reports"`
}

// getClassRosterReport handles GET /api/v1/classes/{id}/roster-report: View B's
// live roster + header. assertTeacherOwnsClass is the only guard needed — every
// query JOINs enrollments with role_in_class='student'.
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
	ctx := r.Context()
	rows, err := a.d.Queries.ListClassRosterCounts(ctx, id)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	start, end := teacher.WeekWindow(time.Now())
	hdr, err := a.d.Queries.GetClassLiveHeader(ctx, sqlc.GetClassLiveHeaderParams{
		ClassID: id, WeekStart: start, WeekEnd: end,
	})
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	out := make([]RosterEntry, 0, len(rows))
	for _, row := range rows {
		out = append(out, RosterEntry{
			ID: row.ID.String(), DisplayName: row.DisplayName, AvatarColor: row.AvatarColor,
			ActiveProjects: row.ActiveProjects, ReportCount: row.ReportCount, CoursesFinished: row.CoursesFinished,
		})
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{
		"roster": out,
		"header": ClassLiveHeader{
			ClassSize: hdr.ClassSize, ActiveStudents: hdr.ActiveStudents,
			ActiveProjects: hdr.ActiveProjects, Turns: hdr.Turns, Reports: hdr.Reports,
		},
	})
}
```

Note: this removes the last use of `agent` + `teacher.DBadge/ABadge` in the roster handler. `encoding/json` may still be used elsewhere in the file (student-detail) — leave imports for Task 5 to finalize. If the build complains about an unused `agent` import *after this task alone*, keep it until Task 5 (student-detail still imports `agent`); do not delete `agent` usage that Task 5 owns.

- [ ] **Step 4: Update tests** — in `teacher_read_test.go`, find roster-report assertions (grep `dBadge`, `roster-report`, `RosterReportEntry`) and rewrite to assert the new `roster[].activeProjects/reportCount/coursesFinished` + `header`. Seed a student with an active project, a ready `evaluation_report`, and a completed `course_progress`; assert counts = 1/1/1.

- [ ] **Step 5: Run** `cd apps/api && go build ./... && go test ./internal/api/ -run Roster -timeout 900s` (FOREGROUND) → PASS.

- [ ] **Step 6: Commit** explicit paths (`teacher.sql`, regenerated `sqlc/`, `teacher_read.go`, test) — `git commit -m "feat(teacher): roster + live header on activity counts (no axis)"`

---

## Task 3: Weekly rule layer + facts + composer (activity metrics)

**Files:**
- Rewrite: `apps/api/internal/teacher/weekly.go`
- Delete: `apps/api/internal/teacher/badges.go`
- Modify: `apps/api/internal/agent/compose_weekly.go`
- Test: `apps/api/internal/teacher/weekly_test.go`, `apps/api/internal/agent/compose_weekly_test.go` (update)

**Interfaces:**
- Produces (teacher): `StudentWeek{ UserID, DisplayName, AvatarColor string; ActiveDays, Turns, PrevActiveDays, ReportsThisWeek, PriorReports int; LatestReportProjectID string }`; `Card{ UserID, DisplayName, AvatarColor, TagCode, TagLabel, Kind, Evidence string; HasReport bool; ReportScopeID string }`; `Weekly{ Praise, Watch []Card }`; `Detect([]StudentWeek) Weekly`; `Stats(cur, prev ClassWeekCounts, classSize int) []Stat` (unchanged signature); `BuildWeeklyFacts(className string, classSize int, weekLabel string, w Weekly) agent.WeeklyFacts`.
- Produces (agent): `WeeklyFacts{ ClassName string; ClassSize int; WeekLabel string; Cards []WeeklyFactCard }`; `WeeklyProse{ Comment string; Cards []WeeklyCardProse }`; `ComposeWeekly` signature unchanged.
- Consumed by Task 4 (`loadWeekly` maps query rows → `StudentWeek`; `weeklyDTO` reads `Weekly`).

- [ ] **Step 1: Delete** `badges.go`: `git rm apps/api/internal/teacher/badges.go`.

- [ ] **Step 2: Rewrite** `internal/teacher/weekly.go` entirely:

```go
package teacher

import (
	"fmt"
	"sort"

	"mindimprint/api/internal/agent"
)

// StudentWeek is one student's completed week as the activity rules see it:
// usage this window and the same class's prior window, report counts, and the
// project id of their latest ready report (for the "看能力报告" link). No axis,
// no agent.Report — the teacher path no longer reads the retired evaluations.
type StudentWeek struct {
	UserID, DisplayName, AvatarColor string
	ActiveDays, Turns, PrevActiveDays int
	ReportsThisWeek, PriorReports     int
	LatestReportProjectID             string // "" when the student has no ready report
}

// Card is one 值得表扬 / 需要建议 card. Evidence is ALWAYS deterministic — a bare
// statement of the week's numbers. Lead/action (wording) are added later by the
// composer and are not fields here.
type Card struct {
	UserID, DisplayName, AvatarColor string
	TagCode, TagLabel, Kind          string
	Evidence                         string
	HasReport                        bool
	ReportScopeID                    string // project id; surface is always "project"
}

type Weekly struct {
	Praise, Watch []Card
}

// strongEngagementFloor is the minimum turns a report-less student needs before
// "使用最活跃" can fire, so a near-empty class does not crown a 3-turn student.
const strongEngagementFloor = 10

// Detect is the whole judgment layer: activity rules decide who appears, with
// which tag and which numeric evidence. A student carries at most one card;
// watch precedes praise. strong_engagement additionally needs to be class-top,
// so the class turn stats are computed first.
func Detect(students []StudentWeek) Weekly {
	var w Weekly
	// classMeanTurns and topTurns gate strong_engagement (class-relative).
	sum, n, topTurns := 0, 0, 0
	for _, s := range students {
		sum += s.Turns
		n++
		if s.Turns > topTurns {
			topTurns = s.Turns
		}
	}
	mean := 0.0
	if n > 0 {
		mean = float64(sum) / float64(n)
	}
	for _, s := range students {
		if c, ok := watchCard(s); ok {
			w.Watch = append(w.Watch, c)
		} else if c, ok := praiseCard(s, mean, topTurns); ok {
			w.Praise = append(w.Praise, c)
		}
	}
	return w
}

func mkCard(s StudentWeek, kind, code, label, evidence string) Card {
	return Card{
		UserID: s.UserID, DisplayName: s.DisplayName, AvatarColor: s.AvatarColor,
		TagCode: code, TagLabel: label, Kind: kind, Evidence: evidence,
		HasReport: s.LatestReportProjectID != "", ReportScopeID: s.LatestReportProjectID,
	}
}

// watchCard: severity order, first match wins.
func watchCard(s StudentWeek) (Card, bool) {
	if s.ActiveDays == 0 {
		return mkCard(s, "watch", "never_used", "本周未使用",
			fmt.Sprintf("本周 0 天活动记录；上周 %d 天。", s.PrevActiveDays)), true
	}
	if s.PrevActiveDays >= 2 && s.ActiveDays <= s.PrevActiveDays-2 {
		return mkCard(s, "watch", "dropped_off", "本周掉线",
			fmt.Sprintf("活跃天数 上周 %d 天 → 本周 %d 天。", s.PrevActiveDays, s.ActiveDays)), true
	}
	if s.Turns > 0 && s.ReportsThisWeek == 0 {
		return mkCard(s, "watch", "stuck_no_output", "有对话没产出",
			fmt.Sprintf("本周 %d 轮对话，但没有完成能力报告。", s.Turns)), true
	}
	return Card{}, false
}

// praiseCard: milestone first, then output, then class-top engagement.
func praiseCard(s StudentWeek, classMeanTurns float64, topTurns int) (Card, bool) {
	if s.ReportsThisWeek > 0 {
		if s.PriorReports == 0 {
			return mkCard(s, "praise", "first_report", "第一份报告",
				"本周完成了第一份能力报告。"), true
		}
		return mkCard(s, "praise", "produced_report", "有产出",
			fmt.Sprintf("本周完成 %d 份能力报告。", s.ReportsThisWeek)), true
	}
	if s.Turns >= strongEngagementFloor && s.Turns == topTurns && float64(s.Turns) >= classMeanTurns*1.5 {
		return mkCard(s, "praise", "strong_engagement", "使用最活跃",
			fmt.Sprintf("本周 %d 轮对话，是班里使用最活跃的。", s.Turns)), true
	}
	return Card{}, false
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

// Stats renders the four cards. An unchanged metric renders ±0 as flat.
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
		mk("reports", "生成能力报告", cur.Reports, prev.Reports, "份", "完成并生成过程评估的项目"),
		mk("turns", "AI 对话轮次", cur.Turns, prev.Turns, "轮", "反映本周使用强度"),
		mk("course_steps", "完成课程节", cur.CourseSteps, prev.CourseSteps, "节", "平台内自学课程"),
	}
}

// BuildWeeklyFacts is the ONLY thing the composer sees: the class identity and
// the deterministically-selected cards with their numeric evidence. It omits
// class-level usage counters (they keep ticking; prose is written once).
func BuildWeeklyFacts(className string, classSize int, weekLabel string, w Weekly) agent.WeeklyFacts {
	facts := agent.WeeklyFacts{ClassName: className, ClassSize: classSize, WeekLabel: weekLabel}
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

- [ ] **Step 3: Update the composer** `internal/agent/compose_weekly.go`:
  - `WeeklyFacts`: drop `DepthBuckets`, `RatedCount`, `AutonomyMean`, `AutonomyDelta`, `BucketChanges`. Keep `ClassName`, `ClassSize`, `WeekLabel`, `Cards`. Delete the `WeeklyBucketChange` type.
  - `WeeklyProse`: drop `DepthNote`, `AutonomyNote`. Keep `Comment`, `Cards`.
  - `weeklySystemPrompt`: keep rules 1–5 but change the JSON schema in rule 5 to `{"comment":"","cards":[{"userId":"","lead":"","action":""}]}` and rule 3's card description (unchanged — lead/action). Keep rule 2 (说人话, no D/A codes) — still valuable.
  - `WeeklyFactsPrompt`: emit only the class header + `需要写措辞的卡片:` list. Remove the depth-distribution / bucket-change / autonomy-mean lines:

```go
func WeeklyFactsPrompt(f WeeklyFacts) string {
	var b strings.Builder
	fmt.Fprintf(&b, "班级：%s（%d 名学生）\n周次：%s\n", f.ClassName, f.ClassSize, f.WeekLabel)
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
```
  - `validateWeeklyProse`: drop the `DepthNote`/`AutonomyNote` length checks; the `texts` slice for the bare-code scan becomes `[]string{p.Comment}` plus each card's `Lead`/`Action`. Keep the per-card presence/uniqueness/length/bare-code checks and the `weeklyCommentMax` check. Delete `weeklyNoteMax`.

- [ ] **Step 4: Rewrite the rule/composer tests** — `weekly_test.go`: delete axis/bucket/autonomy tests; add table tests for each rule (never_used, dropped_off, stuck_no_output, first_report, produced_report, strong_engagement) and for the watch-before-praise precedence + at-most-one-card invariant. `compose_weekly_test.go`: update the `WeeklyFacts` fixtures to the new shape; assert `WeeklyFactsPrompt` carries the cards and does NOT carry depth/autonomy strings; keep the bare-code-rejection and missing-card-rejection tests.

- [ ] **Step 5: Run** `cd apps/api && go build ./internal/teacher/ ./internal/agent/ && go test ./internal/teacher/ ./internal/agent/ -timeout 300s`. (Package `internal/api` will NOT build yet — Task 4 owns those callers. That is expected; do not "fix" api here.)

- [ ] **Step 6: Commit** `git add apps/api/internal/teacher/weekly.go apps/api/internal/agent/compose_weekly.go apps/api/internal/teacher/weekly_test.go apps/api/internal/agent/compose_weekly_test.go && git rm apps/api/internal/teacher/badges.go && git commit -m "feat(teacher): activity-metric weekly rules + composer; drop axis badges"`

---

## Task 4: Weekly queries + handler + DTO + prose-cache migration

**Files:**
- Modify: `apps/api/internal/store/queries/teacher_weekly.sql`, `apps/api/internal/api/teacher_weekly.go`
- Create: `apps/api/internal/store/migrations/0067_truncate_weekly_prose_cache.sql`
- Regenerate: sqlc
- Test: `apps/api/internal/api/teacher_weekly_test.go` (update)

**Interfaces:**
- Consumes Task 3's `teacher.StudentWeek`/`Weekly`/`BuildWeeklyFacts`/`agent.WeeklyFacts`/`WeeklyProse`.
- Produces: `GET /classes/{id}/weekly-report?weekStart=<RFC3339>` (default = last completed week). DTO `WeeklyReportDTO` drops `Depth`/`Autonomy`, adds `IsLatestWeek bool`. Focus cards drop `ReportSurface` from the model side — set `ReportSurface:"project"` in the DTO so the frontend link stays project-scoped.

- [ ] **Step 1: Migration** `0067_truncate_weekly_prose_cache.sql`:

```sql
-- +goose Up
-- class_weekly_prose is a regenerable cache (every number is live; only wording
-- is stored). The fact shape changed from axis to activity metrics AND the window
-- moved to the last completed week, so any pre-existing row narrates the old
-- world. Truncate so no stale axis-worded prose is ever served; it regenerates
-- lazily on next open. depth_note / autonomy_note columns are LEFT in place
-- (harmless, written as '') to avoid a query-shape change on the prose table.
TRUNCATE class_weekly_prose;

-- +goose Down
-- No-op: a cache truncation cannot be un-done, and does not need to be.
SELECT 1;
```

- [ ] **Step 2: Repoint the weekly queries** in `teacher_weekly.sql`:
  - `GetClassWeekStats`: change the `reports` subquery from `student_evaluation` to ready `evaluation_report` in the window:

```sql
  (SELECT count(*) FROM evaluation_report er JOIN project p ON p.id = er.project_id
     JOIN members m ON m.id = p.user_id
     WHERE er.status = 'ready' AND er.created_at >= @week_start AND er.created_at < @week_end)::int AS reports;
```
  - Delete `ListClassStudentWindowUsage` and `ListClassRecentReports`; add:

```sql
-- name: ListClassStudentWeekActivity :many
-- Per-student activity for a COMPLETED week + the full prior week, plus report
-- counts (this week / before this week) and the latest ready report's project id
-- for the card link. All no-LLM; no student_evaluation.
SELECT
  u.id AS user_id, u.display_name, u.avatar_color,
  COALESCE(cur.active_days, 0)::int AS active_days,
  COALESCE(cur.turns, 0)::int       AS turns,
  COALESCE(prv.active_days, 0)::int AS prev_active_days,
  COALESCE(rep.n, 0)::int           AS reports_this_week,
  COALESCE(prior.n, 0)::int         AS prior_reports,
  latest.project_id                 AS latest_report_project_id
FROM enrollments e
JOIN users u ON u.id = e.user_id
LEFT JOIN LATERAL (
  SELECT COUNT(DISTINCT (ev.created_at AT TIME ZONE 'UTC')::date) AS active_days,
         COUNT(*) FILTER (WHERE ev.type IN ('prompt_sent','course_message')) AS turns
  FROM event ev
  WHERE ev.user_id = u.id AND ev.created_at >= @week_start AND ev.created_at < @week_end
) cur ON true
LEFT JOIN LATERAL (
  SELECT COUNT(DISTINCT (ev.created_at AT TIME ZONE 'UTC')::date) AS active_days
  FROM event ev
  WHERE ev.user_id = u.id AND ev.created_at >= @prev_start AND ev.created_at < @prev_end
) prv ON true
LEFT JOIN LATERAL (
  SELECT count(*) AS n FROM evaluation_report er JOIN project p ON p.id = er.project_id
  WHERE p.user_id = u.id AND er.status = 'ready' AND er.created_at >= @week_start AND er.created_at < @week_end
) rep ON true
LEFT JOIN LATERAL (
  SELECT count(*) AS n FROM evaluation_report er JOIN project p ON p.id = er.project_id
  WHERE p.user_id = u.id AND er.status = 'ready' AND er.created_at < @week_start
) prior ON true
LEFT JOIN LATERAL (
  SELECT er.project_id FROM evaluation_report er JOIN project p ON p.id = er.project_id
  WHERE p.user_id = u.id AND er.status = 'ready'
  ORDER BY er.created_at DESC LIMIT 1
) latest ON true
WHERE e.class_id = @class_id AND e.role_in_class = 'student'
ORDER BY u.display_name;
```
  Note: `latest_report_project_id` is nullable (LEFT JOIN) → sqlc maps to `pgtype.UUID`.

- [ ] **Step 3: Regenerate sqlc** — `cd apps/api && make sqlc`.

- [ ] **Step 4: Rewrite `loadWeekly` + `weeklyDTO` + handler** in `teacher_weekly.go`:
  - `WeeklyReportDTO`: delete `Depth WeeklyDepthDTO` and `Autonomy WeeklyAutonomyDTO` fields; delete the `WeeklyBucketDTO`/`WeeklyDepthDTO`/`WeeklyAutonomyDTO` types. Add `IsLatestWeek bool json:"isLatestWeek"`. `WeeklyCardDTO`: keep as-is (its `ReportSurface`/`ReportScopeID` stay; set surface `"project"` in `conv`).
  - `loadWeekly(ctx, cls, weekStart, now)` — take the completed `weekStart`:

```go
func (a *API) loadWeekly(ctx context.Context, cls sqlc.Class, weekStart, now time.Time) (weeklyData, error) {
	start, end, prevStart, prevEnd := teacher.CompletedWeekWindows(weekStart)

	cur, err := a.d.Queries.GetClassWeekStats(ctx, sqlc.GetClassWeekStatsParams{ClassID: cls.ID, WeekStart: start, WeekEnd: end})
	if err != nil {
		return weeklyData{}, err
	}
	prev, err := a.d.Queries.GetClassWeekStats(ctx, sqlc.GetClassWeekStatsParams{ClassID: cls.ID, WeekStart: prevStart, WeekEnd: prevEnd})
	if err != nil {
		return weeklyData{}, err
	}
	act, err := a.d.Queries.ListClassStudentWeekActivity(ctx, sqlc.ListClassStudentWeekActivityParams{
		ClassID: cls.ID, WeekStart: start, WeekEnd: end, PrevStart: prevStart, PrevEnd: prevEnd,
	})
	if err != nil {
		return weeklyData{}, err
	}
	students := make([]teacher.StudentWeek, 0, len(act))
	for _, u := range act {
		s := teacher.StudentWeek{
			UserID: u.UserID.String(), DisplayName: u.DisplayName, AvatarColor: u.AvatarColor,
			ActiveDays: int(u.ActiveDays), Turns: int(u.Turns), PrevActiveDays: int(u.PrevActiveDays),
			ReportsThisWeek: int(u.ReportsThisWeek), PriorReports: int(u.PriorReports),
		}
		if u.LatestReportProjectID.Valid {
			s.LatestReportProjectID = uuid.UUID(u.LatestReportProjectID.Bytes).String()
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
```
  - `weeklyDTO(d, prose)`: build header from `teacher.CompletedWeekWindows(d.WeekStart)`; `WeekLabel(d.WeekStart)`; `AsOf` = `end`; set `IsLatestWeek = teacher.IsLatestCompletedWeek(d.WeekStart, d.Now)`. Delete all `Depth`/`Autonomy` assembly. In `conv`, set `ReportSurface: "project"`, `ReportScopeID: c.ReportScopeID`. Read `prose.Comment` only (ignore depth/autonomy columns).
  - Both handlers: parse `weekStart` and validate. Add a helper:

```go
func (a *API) resolveWeekStart(r *http.Request, now time.Time) (time.Time, error) {
	q := r.URL.Query().Get("weekStart")
	if q == "" {
		return teacher.LastCompletedWeekStart(now), nil
	}
	ws, err := time.Parse(time.RFC3339, q)
	if err != nil {
		return time.Time{}, httpx.ErrBadRequest("weekStart 格式不正确")
	}
	if err := teacher.ValidateCompletedWeekStart(ws, now); err != nil {
		return time.Time{}, httpx.ErrBadRequest("只能查看已结束的周")
	}
	return ws.UTC(), nil
}
```
  (Confirm the exact `httpx.ErrBadRequest` constructor name by grepping `httpx`; use whatever the package exposes for 400.)
  - `getClassWeeklyReport` / `postClassWeeklyProse`: after `assertTeacherOwnsClass`, `ws, err := a.resolveWeekStart(r, now)`; pass `ws` into `loadWeekly`. The prose `weekParam` already derives from `data.WeekStart` (now the completed week's Monday) — unchanged. `InsertClassWeeklyProse`: pass `DepthNote: "", AutonomyNote: ""` (columns kept). The `!hasRow && len(facts.Cards) == 0` empty-guard: drop the `data.Weekly.Depth.RatedCount == 0` clause (no depth now) → guard on `len(facts.Cards) == 0` alone.

- [ ] **Step 5: Update tests** `teacher_weekly_test.go`: seed reports/events in a *completed* week; assert stats.reports counts `evaluation_report`; assert focus cards fire on the activity rules; assert DTO has no `depth`/`autonomy` and carries `isLatestWeek`; assert `?weekStart=<current-week-monday>` → 400.

- [ ] **Step 6: Run** `cd apps/api && make sqlc && go build ./... && go test ./internal/api/ -run Weekly -timeout 900s` (FOREGROUND) → PASS. `internal/api` now builds fully again.

- [ ] **Step 7: Commit** explicit paths.

---

## Task 5: Student-detail repoint + head-badge removal + dead-code retirement

**Files:**
- Modify: `apps/api/internal/store/queries/teacher.sql`, `apps/api/internal/api/teacher_read.go`
- Regenerate: sqlc
- Test: `apps/api/internal/api/teacher_read_test.go`

**Interfaces:**
- Produces: `StudentHeadDTO{ id, displayName, avatarColor }` (no badges/unrated). `records` come from projects only (project surface), `hasReport` from `evaluation_report`. `usage.reportCount` = lifetime ready reports; `usage.courseCount` = finished courses.

- [ ] **Step 1: Repoint / delete queries** in `teacher.sql`:
  - `ListStudentProjectsForTeacher`: change `has_report` to `evaluation_report`:

```sql
-- name: ListStudentProjectsForTeacher :many
SELECT p.id, p.title, p.last_active_at,
       EXISTS (SELECT 1 FROM evaluation_report er WHERE er.project_id = p.id AND er.status = 'ready') AS has_report
FROM project p
WHERE p.user_id = @user_id
ORDER BY p.last_active_at DESC NULLS LAST;
```
  - Add a lifetime finished-course count (mirror `FinishedCourseIDsByUser` but as a scalar):

```sql
-- name: CountFinishedCoursesForStudent :one
SELECT count(*)::int FROM course_progress WHERE user_id = @user_id AND completed_at IS NOT NULL;
```
  - Delete `ListStudentReportsForTeacher` and `GetLatestReportScoresForStudent` (both read the retired `evaluations`/`student_evaluation`). If `GetStudentWeekStats` (reads `student_evaluation` for a `reports` count) is still referenced, repoint its reports sub-select to `evaluation_report` the same way; if it is unreferenced after this task, delete it. Grep to decide: `grep -rn "GetStudentWeekStats\|ListStudentReportsForTeacher\|GetLatestReportScoresForStudent\|ListClassRecentReports\|ListClassStudentWindowUsage\|ListClassRosterReport" apps/api` must return **zero** non-generated hits when done.

- [ ] **Step 2: Regenerate sqlc**.

- [ ] **Step 3: Rewrite `getStudentDetail`** in `teacher_read.go`:
  - `StudentHeadDTO`: drop `DBadge`, `ABadge`, `Unrated`.
  - Remove the `GetLatestReportScoresForStudent` block and the `ListStudentReportsForTeacher` block.
  - Records: iterate `ListStudentProjectsForTeacher` only — every project is a row; `Status` = `"能力报告已生成"` if `has_report` else `"进行中"`; `HasReport` from the column; `Date` = `last_active_at`. `reportCount` = count of `has_report` projects. `courseCount` = `CountFinishedCoursesForStudent`.
  - Keep `GetStudentUsageForTeacher` (this-week active days/turns) — it does not read evaluations.

```go
	head := StudentHeadDTO{ID: userID.String(), DisplayName: user.DisplayName, AvatarColor: user.AvatarColor}

	projects, err := a.d.Queries.ListStudentProjectsForTeacher(ctx, userID)
	if err != nil { httpx.WriteError(w, r, err); return }
	courseCount, err := a.d.Queries.CountFinishedCoursesForStudent(ctx, userID)
	if err != nil { httpx.WriteError(w, r, err); return }

	records := make([]RecordDTO, 0, len(projects))
	reportCount := 0
	for _, p := range projects {
		status := "进行中"
		if p.HasReport { status = "能力报告已生成"; reportCount++ }
		records = append(records, RecordDTO{
			Surface: "project", ScopeID: p.ID.String(), Title: p.Title,
			Date: p.LastActiveAt.Format(time.RFC3339), Status: status, HasReport: p.HasReport,
		})
	}
	// projects already ordered by last_active_at DESC — no re-sort needed.

	httpx.WriteJSON(w, http.StatusOK, map[string]any{
		"student": head,
		"usage": UsageDTO{ActiveDays: usageRow.ActiveDays, Turns: usageRow.Turns, ReportCount: reportCount, CourseCount: int(courseCount)},
		"records": records,
	})
```
  Confirm `p.LastActiveAt` type — if it is `pgtype.Timestamptz`, format `p.LastActiveAt.Time`; if `time.Time`, format directly (match the pre-existing code's handling).

- [ ] **Step 4: Remove now-dead deep-report seam.** Grep the frontend for the teacher deep-report endpoint that returns `TeacherReportDTO`/`ReportContext` (`grep -rn "reportContext\|projectTitle\|TeacherReportDTO" apps/web/src`). The current `TeacherReportView` uses `getStudentEvaluationReport` (new report), so the old per-scope `TeacherReportDTO` endpoint + `ReportContext` (which call `teacher.DBadge/ABadge`) are dead. Delete the handler, its route in `api.go`, `TeacherReportDTO`, and `ReportContext` — BUT only after confirming zero frontend callers. `studio.ReportDTO` stays (course still uses it). If a frontend caller exists, STOP and surface it (the design assumed none).

- [ ] **Step 5: Finalize imports** — remove the now-unused `agent`, `sort`, and `encoding/json` imports from `teacher_read.go` if the compiler flags them (json/sort were only used by the deleted report-merge). Run `cd apps/api && go build ./...` → green. `grep -rn "teacher.DBadge\|teacher.ABadge\|agent.Report" apps/api/internal/api apps/api/internal/teacher` → zero hits.

- [ ] **Step 6: Update tests** `teacher_read_test.go`: student-detail assertions drop `dBadge`/`aBadge`; assert `records` come from projects with `hasReport` driven by `evaluation_report`; assert `usage.reportCount`/`courseCount`.

- [ ] **Step 7: Full suite** `cd apps/api && go build ./... && go vet ./... && go test ./... -timeout 1800s` (FOREGROUND) → all green.

- [ ] **Step 8: Commit** explicit paths — `git commit -m "refactor(teacher): student-detail on evaluation_report; retire axis dead code"`

---

## Task 6: Frontend contracts (`api/teacher.ts`)

**Files:**
- Modify: `apps/web/src/api/teacher.ts` (+ `src/api/index.ts` if it re-exports the removed names)
- Test: none (types); `npm run build` gates it.

**Interfaces:**
- Produces the TS types the UI tasks consume.

- [ ] **Step 1: Replace the types + client calls**:

```ts
export interface RosterEntry {
  id: string;
  displayName: string;
  avatarColor: string;
  activeProjects: number;
  reportCount: number;
  coursesFinished: number;
}

export interface ClassLiveHeader {
  classSize: number;
  activeStudents: number;
  activeProjects: number;
  turns: number;
  reports: number;
}

export interface ClassRoster {
  roster: RosterEntry[];
  header: ClassLiveHeader;
}

export interface StudentRecord {
  surface: "project" | "course" | "chat";
  scopeId: string;
  title: string;
  date: string;
  status: string;
  hasReport: boolean;
}

export interface StudentDetail {
  student: { id: string; displayName: string; avatarColor: string };
  usage: { activeDays: number; turns: number; reportCount: number; courseCount: number };
  records: StudentRecord[];
}

export async function getClassRosterReport(classId: string): Promise<ClassRoster> {
  return apiFetch<ClassRoster>(`/api/v1/classes/${classId}/roster-report`);
}

export interface WeeklyCard {
  userId: string; displayName: string; avatarColor: string;
  tagCode: string; tagLabel: string; kind: "praise" | "watch";
  evidence: string; lead: string; action: string;
  hasReport: boolean; reportSurface?: string; reportScopeId?: string;
}

export interface WeeklyReport {
  weekLabel: string; weekStart: string; weekEnd: string; asOf: string;
  className: string; classSize: number;
  stats: { key: string; label: string; value: number; unit: string; foot: string; delta: string; deltaDir: "up" | "down" | "flat" }[];
  praise: WeeklyCard[]; watch: WeeklyCard[];
  comment: string | null; proseReady: boolean; isLatestWeek: boolean;
}

export async function getClassWeeklyReport(classId: string, weekStart?: string): Promise<WeeklyReport> {
  const qs = weekStart ? `?weekStart=${encodeURIComponent(weekStart)}` : "";
  return apiFetch<WeeklyReport>(`/api/v1/classes/${classId}/weekly-report${qs}`);
}

export async function generateClassWeeklyProse(classId: string, weekStart?: string): Promise<WeeklyReport> {
  const qs = weekStart ? `?weekStart=${encodeURIComponent(weekStart)}` : "";
  return apiFetch<WeeklyReport>(`/api/v1/classes/${classId}/weekly-report/prose${qs}`, { method: "POST" });
}
```
  Keep `getStudentDetail` and `getStudentEvaluationReport` as-is (return-type of `getStudentDetail` is the trimmed `StudentDetail`). Remove `RosterReportEntry` and the old `getClassRosterReport` return-array shape. Update `ApiClient` type/`src/api/index.ts` re-exports if they reference removed names.

- [ ] **Step 2: Typecheck** `cd apps/web && npm run build` — EXPECT failures in the console components (they still read `.dBadge`, `.depth`, array roster). That is expected; Tasks 7–10 fix them. Confirm the failures are ONLY in `src/console/*` and are the renamed-field errors, then commit the contract.

- [ ] **Step 3: Commit** `git add apps/web/src/api/teacher.ts apps/web/src/api/index.ts && git commit -m "feat(teacher-web): activity-metric contracts (roster counts, live header, no axis)"`

---

## Frontend re-skin vocabulary (Tasks 7–10)

Every console file replaces its hardcoded cool-blue-grey hex with `mk-*` CSS variables and its per-file consts with `@/ui` primitives. Use `var(--mk-*)` in inline styles (the console has no Tailwind classes to convert). **Mapping (apply everywhere):**

| Console hardcoded | Token |
|---|---|
| navy `#2A3B7A` (brand/primary/link) | `var(--mk-accent)` (strong: `--mk-accent-600`) |
| ink `#1C2333` / `#2A3040` | `var(--mk-ink)` / body `var(--mk-secondary)` |
| muted `#8A92A3` `#6C7488` `#9198A8` `#A2A9B8` `#B7BECC` | `var(--mk-muted)` / faint `var(--mk-faint)` |
| borders `#EAECF2` `#F2F3F7` `#EEF0F4` | `var(--mk-border)` |
| `#fff` card surface | `var(--mk-surface)`; page bg `var(--mk-paper)` |
| tint bg `#EDEFF9` `#F1F2F8` (navy tint) | `var(--mk-accent-50)` (SOLID step — never alpha) |
| danger `#C76B6B` `#C4574D` | `var(--mk-danger)` / bg `var(--mk-danger-bg)` |
| success green `#3E8A6E` / `#E9F2EC` | `var(--mk-success)` / `var(--mk-success-bg)` (or `--mk-matcha*`) |
| warning amber `#B0863A` `#C68A3A` | `var(--mk-warning)` / `var(--mk-warning-bg)` |
| shadows `0 4px 20px rgba(20,30,60,.04)` etc. | `var(--mk-shadow-sm)` / `--mk-shadow-md` |
| radius 12/16/18 | `var(--mk-radius-lg)` (or `--mk-radius-md`) |

**Kind colors** (praise/watch cards, delta pills): praise → `--mk-matcha-*` / `--mk-success*`; watch → `--mk-peach-*` / `--mk-warning*`; delta up→success, down→danger, flat→muted.

**`@/ui` substitutions:** wrapper `<div style={{background:#fff,border,radius,shadow}}>` → `<Card>` (from `@/ui`); local `dangerBtn`/`backBtn`/chip-tab consts → `<Button variant=…>` (grep `apps/web/src/ui/Button.tsx` for the variant names — likely `primary`/`ghost`/`danger`/`secondary`); "加载中…"/"重试"/empty strings → `<EmptyState>` + `Loader2`/`Skeleton` from `@/ui`; hand-inlined stat SVGs → `<Icon>` with a lucide icon where a barrel export exists, otherwise leave the inline SVG but drive its `stroke` from a token/`currentColor`. Import from `@/ui` (the barrel).

---

## Task 7: View A — ClassWeeklyView reskin + axis removal + week nav

**Files:** Modify `apps/web/src/console/ClassWeeklyView.tsx`.

- [ ] **Step 1: Remove the 班级思维维度 section** entirely (the whole trailing block rendering `data.depth` / `data.autonomy`, and the `dTotal` const and `badgeColor` import). The `DeltaPill` stays (used by the stat cards).
- [ ] **Step 2: Week navigation.** Lift `weekStart` into state (default `undefined` → server picks last completed). Thread it through `getClassWeeklyReport(classId, weekStart)` and `generateClassWeeklyProse(classId, weekStart)`. Render a header row with ◄ / ► (`ChevronLeft`/`ChevronRight` from `@/ui`): ◄ sets `weekStart = data.weekStart − 7d` (compute from `data.weekStart` ISO string); ► sets `+7d`, **disabled when `data.isLatestWeek`**. Reset the `asked` ref keyed by `${classId}:${weekStart}` so each week can generate its own prose once. Update the title to use `data.weekLabel` (already the completed week).
- [ ] **Step 3: Reskin** every hardcoded hex per the mapping table; wrap the 点评 block, stat cards, and focus cards in `<Card>`; keep the deterministic evidence + lead/action layout. The card's "看能力报告" still calls `onOpenReport(card.reportSurface!, card.reportScopeId!, …)` (surface is `"project"`).
- [ ] **Step 4:** `cd apps/web && npm run build` → this file clean. Visually verify at the teacher weekly view (seed class): numbers render, focus cards fire, no axis section, ◄/► navigate and ► disables on the latest week.
- [ ] **Step 5: Commit.**

---

## Task 8: View B — roster columns + live header + reskin

**Files:** Modify `apps/web/src/console/ClassRosterTable.tsx`, `apps/web/src/console/ClassDetailView.tsx`; delete `apps/web/src/console/badgeColor.ts`.

- [ ] **Step 1: `ClassRosterTable`** — new columns: 学生 · 进行中项目 · 能力报告 · 完成课程 · (remove). Row fields `s.activeProjects` / `s.reportCount` / `s.coursesFinished`. Avatar tint → `s.avatarColor` (drop `badgeColor`). Delete the `Badge` component and the D/A columns. Reskin via the mapping; the remove-confirm inline uses `<Button variant="danger">` / `variant="ghost"`. Prop type `roster: RosterEntry[]`.
- [ ] **Step 2: `ClassDetailView`** — the roster tab now consumes `ClassRoster` (`{roster, header}`): fetch once, render a light live-header strip above the table (活跃学生 / 进行中项目 / 对话轮次 / 能力报告 from `header`), then `<ClassRosterTable roster={header ? roster : []} …/>`. Update the tab label to `实时` (or keep `全部学生`); reskin the sub-tab chips + back button to `@/ui` `Button`. Confirm the roster fetch call site uses the new `getClassRosterReport` returning `{roster,header}`.
- [ ] **Step 3: Delete** `badgeColor.ts` (`git rm`); confirm `grep -rn badgeColor apps/web/src` is empty.
- [ ] **Step 4:** `npm run build` clean; visually verify roster shows the three counts + live header, remove-student still works.
- [ ] **Step 5: Commit.**

---

## Task 9: Student detail + teacher report view

**Files:** Modify `apps/web/src/console/StudentDetailView.tsx`, `apps/web/src/console/TeacherReportView.tsx`.

- [ ] **Step 1: `StudentDetailView`** — remove the D/A head-badge boxes (`dColors`/`aColors`, the two badge boxes, the `badgeColor` import); avatar tint → `student.avatarColor`. Keep the 4-stat grid (活跃天数 / 对话轮次 / 能力报告 / 完成课程) reading `usage`. Reskin per the mapping (Card wrappers, Button, tokens). The records list + 全部/课程/对话/项目 filter stays; every record is now `surface:"project"`.
- [ ] **Step 2: `TeacherReportView`** — add polling on the `generating` state to match `EvaluationReportPage` (self-rescheduling `setTimeout`, one GET in flight, stop on `ready`/`failed`); reskin the bespoke breadcrumb/header chrome to tokens + `@/ui`. Body stays the shared `EvaluationReportView`.
- [ ] **Step 3:** `npm run build` clean; visually verify student detail (no badges) and open a student's report (renders; generating→ready polling works with a freshly-finished project).
- [ ] **Step 4: Commit.**

---

## Task 10: Console chrome reskin

**Files:** Modify `apps/web/src/console/ConsoleShell.tsx`, `ConsoleRail.tsx`, `ClassesView.tsx`, and any remaining chrome in `ClassDetailView.tsx`.

- [ ] **Step 1:** Reskin `ConsoleRail` (nav rail: hand-inlined SVGs → `@/ui` `Icon`; colors → tokens; active state → `--mk-accent`), `ConsoleShell` (page background `--mk-paper`, layout), and `ClassesView` (class card grid → `<Card>`; create-class form → `@/ui` `forms` + `Button`; 我的班级/全校班级 toggle → `Button`). Apply the mapping table throughout; remove any remaining local button/chip consts.
- [ ] **Step 2:** `cd apps/web && npm run build && npm test` → green. Full visual pass across the whole console (class list → weekly → roster → student detail → report): one warm macaron design system, no cool-blue-grey remnants (`grep -rnE "#2A3B7A|#1C2333|#8A92A3|#EAECF2|#C76B6B" apps/web/src/console` → empty).
- [ ] **Step 3: Commit.**

---

## Self-review notes (checked against spec)

- **Spec coverage:** View A (Tasks 4,7), View B (Tasks 2,8), activity-metric focus rules (Task 3), definitions locked to schema (Tasks 2,4,5), retirement compiler-gated (Tasks 3,5), reskin (Tasks 7–10), no cron/daily (nothing scheduled). ✓
- **KEEP invariants:** no task touches `agent/report_types.go`, `studio/report_dto.go`, `rubric/`, or drops `evaluations`/`llm_usage`. Task 5 explicitly stops STOP-and-surfaces if a frontend caller of the dead deep-report seam exists. ✓
- **Type consistency:** `StudentWeek`/`Card`/`Weekly` (Task 3) ↔ `loadWeekly` mapping (Task 4); `RosterEntry`/`ClassLiveHeader` (Task 2 Go ↔ Task 6 TS); `WeeklyReport` drops `depth`/`autonomy` and adds `isLatestWeek` in both Go DTO (Task 4) and TS (Task 6). ✓
- **Deviation from spec (noted):** `strong_engagement` drops the "≥2 active projects" clause (avoids anachronistic current-state count inside a historical week); it is now class-top turns above a floor — a plan-level tuning the spec permitted ("thresholds tuned in the plan").
