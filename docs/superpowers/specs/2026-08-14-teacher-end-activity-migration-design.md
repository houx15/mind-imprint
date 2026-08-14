# 教师端迁移到活动指标 + 两视图 + 设计令牌 — Design

**Date:** 2026-08-14
**Status:** Approved (design), pending implementation plan
**Related:** [[retire-old-evaluation-pipeline-2026-08-14]] (this session finishes the teacher-side of that retirement)

## Goal

Rebuild the entire teacher console so it no longer depends on the now-unwritten
`evaluations`/`agent.Report` (D/A axis) data. Every teacher number becomes a
cheap, no-LLM **activity metric** read straight from live tables. The class page
splits into two views with opposite cost profiles: a **narrated weekly report**
of the last *completed* week (LLM prose, generated once and cached forever) and a
**live roster** of the current state (DB-only, zero LLM). Finally, re-skin the
console — today a design-system island — onto the app's `mk-*` tokens and `@/ui`
primitives.

## Why (current state / what breaks)

After the 2026-08-14 evaluation-pipeline retirement, `evaluations`/
`student_evaluation` are no longer written; only the new `evaluation_report`
(per-project, fact-rich, status `generating|ready|failed`) is. But the teacher
console was left reading the dead table:

- **Weekly report** (`internal/api/teacher_weekly.go` + `internal/teacher/weekly.go`):
  the 4 top cards come from `event` (fine), but the **class summary prose,
  praise/watch focus-students, and the 班级思维维度 (D distribution + A mean)
  section all unmarshal `agent.Report` from `evaluations`** → they render empty.
- **Roster** (`ListClassRosterReport` + `getClassRosterReport`): D/A badge columns
  derive from `agent.Report` via `teacher.DBadge/ABadge` → empty/`—`.
- **Student detail** (`getStudentDetail`): D/A head badges + records list +
  has-report all read `evaluations`.
- **Design**: `apps/web/src/console/**` uses **zero** `mk-*` tokens and **zero**
  `@/ui` components — its own cool blue-grey hardcoded palette, reinvented
  button/chip/badge consts per file. It is the furthest-from-DS surface in the repo.

## The two-view model

The class-detail page (`ClassDetailView`) keeps two chip sub-tabs, repurposed:

| View | Window | LLM | Caching | Answers |
|---|---|---|---|---|
| **A · 上周周报** (default) | last *completed* week | class comment + per-card wording only | generate-once per (class, week), cached forever | "how did the class do last week?" |
| **B · 实时** (roster) | current state | none | none — live DB reads | "who's active right now, what are they working on?" |

The expensive-but-cached narrative is quarantined to a **frozen past week** (a
completed week's facts never change, so the prose is never regenerated).
Everything a teacher wants *right now* comes from **cheap live DB reads**. Total
LLM spend is bounded to **one call per viewed class per week**.

## Data vocabulary (all no-LLM facts)

Every teacher number derives from four live sources — never `evaluations`/
`agent.Report`:

| Metric | Source | Definition |
|---|---|---|
| active days (window) | `event` | `COUNT(DISTINCT date)` in window, `AT TIME ZONE 'UTC'` |
| AI turns (window) | `event` | `type IN ('prompt_sent','course_message')` in window |
| course steps (window) | `event` | `COUNT(DISTINCT (user,course,ordinal))` where `type='step_viewed'` |
| active students (window) | `event` | students with ≥1 event in window |
| **active projects** | `project` | `status = 'active'` (current, not window-scoped) |
| **report count** | `evaluation_report` ⋈ `project` | `status='ready'` on the student's projects; window-scoped by `created_at` where a window applies |
| **courses finished** | `course_progress` | `completed_at IS NOT NULL` (current, not window-scoped) |

## View A · 上周周报 (narrated, weekly)

### Window
Default = the **last completed week** (`weekStart` = start of previous ISO week,
`weekEnd` = start of current ISO week; both UTC-pinned, matching the existing
`weekWindow` convention). A minimal **prev/next navigation among completed weeks**
is included (cannot navigate into the current/in-progress week). Numbers are
recomputed live per selected window; prose is cached per (class, week).

### Content
- **4 top cards** (unchanged keys): 活跃学生 / 能力报告 / 对话轮次 / 完成课程节, for
  the selected completed week (frozen). `能力报告` now counts **ready
  `evaluation_report`** rows created in the window (was `evaluations`).
- **Focus students** (值得表扬 / 需要建议): deterministically selected by activity
  rules (below); each card carries avatar, tag chip, a **numeric evidence line**
  ("本周 42 轮对话，完成 1 份能力报告"), a "看能力报告" link when the student has a ready
  report, and LLM-written 怎么鼓励/怎么开口 advice.
- **REMOVED**: the entire 班级思维维度 section (D distribution bar + A mean block)
  and the D/A badges on focus cards.

### Focus-student rules (activity metrics only)
Pure Go, no model. Reshape `teacher.Detect` / `weekly.go`. `never_used`,
`dropped_off`, `stuck_no_output`, and the delta rules compare this-window vs
prior-window usage (the handler already loads both windows).

**需要建议 (watch)**, severity order:
1. `never_used` — enrolled but 0 events in the window.
2. `dropped_off` — had activity in the prior window, 0 events this window.
3. `stuck_no_output` — active this window (has turns) but produced **0 ready
   reports** this window (engaged in dialogue, nothing completed).

**值得表扬 (praise)**:
1. `first_report` — produced their first-ever ready report this window.
2. `produced_report` — completed ≥1 ready report this window.
3. `strong_engagement` — AI turns at the top of the class this window **and** ≥2
   active projects.

Exact thresholds (what counts as "top of the class" / minimum turns) are tuned in
the plan against the seeded class so cards fire on real data, not lorem. Selection
caps and dedup mirror the current implementation (a student appears on at most one
card; watch takes precedence over praise).

### Prose (LLM, wording-only)
Rework `agent.ComposeWeekly` + its facts/prompt so the model writes **only**:
`comment` (class-level summary) and per-card `lead`/`action`. It cannot add,
drop, reclassify, or renumber cards. Drop `depthNote`/`autonomyNote`. Validation
(length caps, no invented numbers, no bare internal codes) is retained/adapted.

### Generation lifecycle (unchanged architecture)
- `GET /classes/{id}/weekly-report?weekStart=…` — **never calls a model**; returns
  all numbers + focus cards deterministically, plus stored prose if present and a
  `proseReady` flag.
- `POST /classes/{id}/weekly-report/prose` — generates + stores prose once per
  (class, week), first-writer-wins (`class_weekly_prose`, `ON CONFLICT DO NOTHING`),
  with the existing card top-up for cards that appear after first generation.
- Frontend: on opening View A, if `proseReady` is false, fire the POST once
  (guarded), then render the returned prose. **No cron, no daily** — a completed
  week is generated at most once, lazily, and only for classes a teacher opens.

## View B · 实时 (live roster, DB-only)

Zero LLM, nothing cached — every field is a live DB read.

- **Live header** (light, current state): 活跃学生 (this week) · 进行中项目 (class
  total `status='active'`) · 对话轮次 (this week) · 能力报告 (ready reports this week).
- **Roster table**, one row per student (enrollment-scoped, `role_in_class='student'`):
  - **学生** — avatar (tinted by the student's own `avatarColor`) + display name.
  - **进行中项目** — count of `project.status='active'`.
  - **能力报告** — count of ready `evaluation_report` on the student's projects.
  - **完成课程** — count of `course_progress.completed_at IS NOT NULL`.
  - **actions** — remove-from-class (unchanged).
  - Sortable by any count column.
- **REMOVED**: D/A badge columns, `badgeColor` avatar tint, `本周活跃`/`对话轮次`
  columns (those live on View A + student detail). If a teacher wants activity,
  it's on View A / the live header.

New roster payload shape (replaces `RosterReportEntry`):
```
RosterEntry { id, displayName, avatarColor, activeProjects, reportCount, coursesFinished }
```

## Student detail

- **REMOVE** the D/A head-badge boxes; keep the existing 4-stat grid (活跃天数 /
  对话轮次 / 能力报告 / 完成课程节) and add nothing axis-derived. Avatar tint → own
  `avatarColor`.
- **Repoint** `ListStudentReportsForTeacher` (records list) and
  `ListStudentProjectsForTeacher` (`has_report`) from `evaluations` →
  `evaluation_report` (status `ready`).
- "查看完整能力报告" already opens `TeacherReportView` (the new `EvaluationReport`) —
  unchanged. `TeacherReportView` gains polling on `generating` for parity with the
  student page (the future real generator will need it; harmless with the instant
  placeholder).

## Retirement inventory (compiler-gated)

**DELETE (this migration makes them unreferenced — verify with `go build`/tsc):**
- `teacher.DBadge` / `teacher.ABadge` and their helpers `DLevels` / `AMean`
  (`internal/teacher/badges.go`) — once roster, student-detail head, and
  `ReportContext` stop calling them.
- Axis-derived weekly DTO fields (`depth` distribution, `autonomy` aggregate) and
  their `weekly.go` producers (`autonomyAgg`, depth-bucket logic).
- Dead `teacher.sql` queries: `GetLatestReportScoresForStudent`,
  `ListClassRosterReport`'s `student_evaluation` LATERAL, `ListClassRecentReports`
  (axis facts) — replaced by activity queries.
- `apps/web/src/console/badgeColor.ts` and every axis render site.

**REWORK (reshaped, not deleted):**
- `teacher.Detect` / `internal/teacher/weekly.go` → activity-metric selection.
- `agent.ComposeWeekly` + facts/prompt → class comment + card wording only.
- Roster query + handler → new count fields.
- Student-detail record/has-report queries → `evaluation_report`.

**KEEP (shared — do NOT chase this session):**
- `agent.Report` / `internal/agent/report_types.go` — still referenced by
  `studio.ReportDTO`; migrating studio/course off it is out of scope.
- `evaluations` / `student_evaluation` / `llm_usage` tables — retained **physically
  intact as a frozen cost ledger**; the teacher path simply stops reading them.
  Rebuilding `llm_usage` without the eval arm is a separate future concern.

## Design re-skin (tokens + `@/ui`)

Every `apps/web/src/console/**.tsx` migrates off the hardcoded cool blue-grey
palette onto the warm `mk-*` CSS-variable tokens + `src/ui/tokens.ts`, replacing
per-file button/chip/badge/empty-state consts with the shared `@/ui` primitives
(`Button`, `Card`, `EmptyState`, `Badge`, `Icon`, feedback/loaders). The already-
aligned `TeacherReportView` (reuses the student `EvaluationReportView`) is the
model. Follow the four design 铁律 and the existing design system; no new visual
language. Files: `ConsoleShell`, `ConsoleRail`, `ClassesView`, `ClassDetailView`,
`ClassWeeklyView`, `ClassRosterTable`, `StudentDetailView`, `TeacherReportView`;
delete `badgeColor.ts`.

## Non-goals

- ❌ Migrating `studio.ReportDTO` / course-completion off `agent.Report`.
- ❌ Dropping `evaluations`/rebuilding `llm_usage` (frozen ledger stays).
- ❌ Any cron/scheduler (lazy-once-cached only).
- ❌ Daily regeneration.
- ❌ Per-student LLM summaries on the roster (roster is pure DB).
- ❌ New card primitives or a new visual language.

## Phasing (for the plan)

1. **Backend — data & contracts.** New roster query + live-header query (activity
   counts); weekly window shift to last-completed-week; `Detect`/facts/composer on
   activity metrics; remove axis DTO fields; repoint student-detail reads; delete
   proven-dead axis code. Contracts (`packages/contracts` + `api/teacher.ts` types).
   Tests green (`go test ./...`).
2. **Frontend — two-view split + reskin.** Rewire View A (weekly, last-completed
   week + prev/next) and View B (live roster + header) to the new fields; remove
   all axis UI; full `mk-*` + `@/ui` re-skin of every console file; delete
   `badgeColor.ts`. Web + contracts test suites green.
