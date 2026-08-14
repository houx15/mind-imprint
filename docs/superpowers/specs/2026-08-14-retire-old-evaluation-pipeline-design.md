# Retire the Old Evaluation Pipeline — Design

> **Status:** design shaped in chat 2026-08-14; awaiting user review of this written spec. Supersedes the "keep old code as dead" deferral in the EvaluationReport v1 build. Behavior truth for the writing flow remains `docs/2026-08-09-all-statuses.md`; this spec only concerns the evaluation/report surface. **Key boundary (evolved through discussion): this is a report-content migration with the SAME architecture — every report *page* moves to the new `EvaluationReport`, old generation/rendering is deleted, but the teacher roster/weekly/student-detail cluster + `evaluations` table stay untouched for a separate future design (B.0).**

## Goal

Make the new `EvaluationReport` (contract `packages/contracts/src/evaluationReport.ts`, Go `apps/api/internal/evalreport/`, table `evaluation_report`) the evaluation pipeline for every **report-rendering** surface. Delete the old DualAxis / mirror / growth-history / parent-report / chat-assessment **generation + rendering** — code, contracts, endpoints, the two clean prose tables, and tests. Rewire the teacher **per-student report** onto the new report. Add a **report generation lifecycle** so a long-running (real) generator can never double-generate. The teacher **roster / weekly / student-detail** dashboards and the `evaluations` table they read are **deliberately left untouched** (see B.0) — their migration onto the new report is a separate future design.

## Non-negotiable scope decisions (from the user, 2026-08-14)

1. **Old report generation + student-facing rendering → deleted.** No keep-and-rebuild. Every report *page* now renders the new `EvaluationReport`.
2. **Parent report → becomes the future PDF export.** Remove the *old* `compose_parent` / `ParentReport` implementation. The "导出 PDF" intent survives only as the already-present disabled button; it is the seam for the future parent/PDF work. No parent generation code remains.
3. **Chat assessment → gone entirely.** Deleted, not stubbed.
4. **Course completion → empty placeholder.** Wherever a course-completion / course-report surface existed, leave an inert "敬请期待" stub. No logic — the course model is not settled.
5. **Teacher roster / weekly / student-detail + the `evaluations` table → untouched this pass (B.0).** Needs its own time/cost/performance design; not migrated, not dropped here.
6. **Only the two clean prose tables are dropped:** `project_mirror_prose`, `parent_report_prose`. `evaluations` / `student_evaluation` / `llm_usage` are KEPT. None of this is on prod (origin is behind; the new pipeline is unpushed/undeployed), and we have decided no backward-compat, so dropping the two prose tables loses nothing that matters.

## The four 铁律 still bind

Nothing here weakens 铁律: the new report is observation-not-grading, AI never ghostwrites, D/A shown by color not calibrated number. This is a deletion + rewire, not a change to what the report says.

---

## Part A — Generation lifecycle (fixes the double-generation race)

### The problem

`generateAndStoreEvaluationReport` does an unconditional `INSERT`. Two callers reach it:

- the **finish goroutine** (`runProjectReport`), and
- **`postGenerateEvaluationReport`** — fired by the report page's `GET → if null, POST generate` fallback.

`postGenerateEvaluationReport` uses a non-atomic check-then-insert with no unique constraint. Harmless with the instant placeholder (the goroutine's row lands within milliseconds, so the GET almost always finds it). But with the real ~10-minute generator, a viewer arriving mid-generation gets `null` → POSTs → starts a **second full generation** → double token spend, duplicate rows.

### The design — the report row IS the generation lock

`evaluation_report` becomes **one row per project** carrying a **status**, and generation is **claimed atomically**.

**Schema (new migration, reshaping the just-added `evaluation_report`):**

- `report jsonb` becomes **NULLABLE** (a `generating` row has no report yet).
- add `status text NOT NULL DEFAULT 'ready'` with `CHECK (status IN ('generating','ready','failed'))`.
- add `UNIQUE (project_id)` (one report per project — matches the "one report = one project" decision, DEC-A3.5). The old `evaluation_report_project_created_idx` is dropped (the unique index serves project lookups).

**Claim query — atomic, re-claims only stale/failed:**

```sql
-- name: ClaimEvaluationReportGeneration :one
INSERT INTO evaluation_report (project_id, version, status, report)
VALUES (@project_id, 1, 'generating', NULL)
ON CONFLICT (project_id) DO UPDATE
  SET status = 'generating', report = NULL, created_at = now()
  WHERE evaluation_report.status = 'failed'
     OR (evaluation_report.status = 'generating'
         AND evaluation_report.created_at < now() - interval '30 minutes')
RETURNING id;
```

- Fresh project → INSERT wins → row returned → caller **owns** generation.
- Existing `ready` row → `ON CONFLICT` `WHERE` is false → **no row returned** (`pgx.ErrNoRows`) → caller does nothing (report already exists).
- Existing `generating` row (fresh, someone else owns it) → no row returned → caller does nothing. **No second generation ever starts.**
- Existing `failed` row, or a `generating` row older than 30 min (crashed generation) → re-claimed. Prevents permanent lockout.

**Completion / failure:**

```sql
-- name: CompleteEvaluationReport :exec
UPDATE evaluation_report SET report = @report, status = 'ready' WHERE project_id = @project_id;

-- name: FailEvaluationReport :exec
UPDATE evaluation_report SET status = 'failed' WHERE project_id = @project_id AND status = 'generating';
```

**Read / list:**

```sql
-- name: GetEvaluationReport :one
SELECT * FROM evaluation_report WHERE project_id = @project_id;

-- name: ListEvaluationReports :many   -- timeline: only ready reports
SELECT er.project_id, er.created_at, p.title, p.qualification
FROM evaluation_report er
JOIN project p ON p.id = er.project_id
WHERE p.user_id = @user_id AND er.status = 'ready'
ORDER BY er.created_at DESC;
```

(`GetLatestEvaluationReport` + its `DISTINCT ON` list are replaced — unique `project_id` makes both trivial.)

### `generateAndStoreEvaluationReport` becomes claim → generate → complete/fail

```go
func (a *API) generateAndStoreEvaluationReport(ctx context.Context, projectID uuid.UUID) error {
    if _, err := a.d.Queries.ClaimEvaluationReportGeneration(ctx, projectID); err != nil {
        if errors.Is(err, pgx.ErrNoRows) {
            return nil // already owned/generated — never double-generate
        }
        return err
    }
    p, err := a.d.Queries.GetProject(ctx, projectID)
    if err != nil { a.d.Queries.FailEvaluationReport(ctx, projectID); return err }

    rep := evalreport.Placeholder(/* … as today … */)
    raw, err := json.Marshal(rep)
    if err != nil { a.d.Queries.FailEvaluationReport(ctx, projectID); return err }

    if err := a.d.Queries.CompleteEvaluationReport(ctx, sqlc.CompleteEvaluationReportParams{
        ProjectID: projectID, Report: raw,
    }); err != nil {
        a.d.Queries.FailEvaluationReport(ctx, projectID)
        return err
    }
    return nil
}
```

The seam the colleague swaps later is unchanged in shape: claim, run the real generator between claim and complete, `CompleteEvaluationReport` on success / `FailEvaluationReport` on error. The claim guarantees single-flight regardless of how long generation runs.

### API read contract — three states, so the page can poll

`GET /api/v1/projects/{id}/evaluation-report` returns an **envelope** (was: the report object or null):

- no row → `null`
- `generating` → `{ "status": "generating" }`
- `failed` → `{ "status": "failed" }`
- `ready` → `{ "status": "ready", "report": { …EvaluationReport… } }`

The teacher read (`GET /classes/{id}/students/{uid}/evaluation-report/{pid}`) returns the same envelope.

`POST /api/v1/projects/{id}/evaluation-report/generate` claims-and-generates if absent (safe under `ON CONFLICT`; can never double-fire); returns the same envelope (`generating` immediately for a real generator, `ready` for the instant placeholder). It stays as the on-demand backfill trigger for finished projects that predate the row.

### Frontend read path — no more lazy-generate-on-null-blindly

`EvaluationReportPage` (and the teacher view) become a small state machine:

- `ready` → render `EvaluationReportView`.
- `generating` → show the "印记正在生成…" spinner and **poll** `GET` (e.g. every ~3 s) until `ready`/`failed`.
- `failed` → error + 重试 (re-POST).
- `null` → the project is finished but has no row (old project): POST generate once (claim), then poll. For a non-finished project, treat as "not generated yet".

The finish flow itself is unchanged (still returns to 全部项目; the goroutine claims+generates). The page just stops racing it.

---

## Part B — Retirement inventory

> Filled from the two exhaustive reference maps (backend + frontend). Grouped DELETE / EDIT / STUB / KEEP. Every EDIT is a reference that would break at compile/test time if the DELETE happened without it.

### B.0 Scope boundary — the roster/weekly cluster STAYS UNTOUCHED (user decision 2026-08-14)

> "the roster and weekly (time-range based) report is still important. although the numbers shown inside needs to be reconsidered."
> "for teacher roster/weekly badges, it needs more design. we need a balance between time, cost, and performance. we have basic statistics in students' report and use those data fields to get some basic data is ok. but the students' abstract information/summary would be generated by llm. we may think about it carefully."

The `evaluations` table is **not** purely old — a shared score-projection layer built on it feeds live teacher **roster / student-detail / weekly-report** dashboards and course completion. **These stay exactly as they are in this pass — not migrated, not deleted.** Migrating them onto the new `evaluation_report` is its **own future design task**, deliberately out of scope, because it needs a time/cost/performance decision (see the deferred-design note below).

**Deferred design — teacher roster/weekly on the new report (future spec, NOT this pass):**
- **Basic stats → cheap, no LLM.** Badges/counts can be derived directly from the **FACT fields** already stored in each `evaluation_report` (depth `level`, autonomy `band`, `counters`, dates) — read-no-call, zero marginal cost. The `DBadge` (min–max of depth levels) / `ABadge` (mean autonomy) logic ports mechanically onto the new report's `depth[]`/`autonomy[]`.
- **Per-student abstract/summary → LLM → careful.** Surfacing a generated narrative per roster row is N-students × a model call — a real time/cost/performance question. Design it deliberately later; do not build it now.

Because roster/weekly stay untouched, this pass **keeps** the `evaluations` table, the `agent.Report` scoring **types** (split out from the deleted generator), the `teacher` badges, `student_evaluation`, and `llm_usage`. It only deletes the old **generation** and the **student-facing rendering surfaces the new report replaces** (+ parent → future PDF).

**MUST-KEEP (verified — shared/live; deleting any of these breaks kept features):**
- `internal/rubric/` — imported by `agent/moment.go` (LIVE coach: `loop.go`, `chat_step.go`, `projectcoach.go`, `card_persist.go`). NOT old-only.
- `internal/studio/report_dto.go` (`ReportDTO`/`ToReportDTO`) — used by `course.go`, `course_dto.go`, `teacher_read.go`, `teacher_weekly.go`.
- `internal/teacher/` package (incl. `DBadge`/`ABadge`, `WeekWindow`/`WeekLabel`) — roster/detail/weekly.
- Tables/views: **`evaluations`, `student_evaluation` (view), `llm_usage` (view)** — do NOT drop.
- Handlers/queries: `getClassRosterReport`+`ListClassRosterReport`, `getStudentDetail`+`ListStudentReportsForTeacher`/`GetLatestReportScoresForStudent`/`ListStudentProjectsForTeacher`, `getClassWeeklyReport`+`teacher_weekly.sql`, and the `evaluations`-reading blocks in `teacher.sql`.
- `uuidText` — currently in `growth_history.go` (deleted); **relocate** into `teacher_read.go` (or a shared helper), since roster/detail call it (`teacher_read.go:220,225`).
- Test seeding: teacher roster/detail tests need to populate `evaluations`; keep a minimal insert path (either retain `InsertProjectEvaluation` for tests, or have those tests seed via raw SQL) so the kept dashboards stay testable.

### B.1 DELETE — backend (old generation + surfaces the new report replaces)

**Go files (whole-file delete, OLD-only — verified none imported by the keep-cluster):**
- Agent generation: `assess_report_prompt.go`, `assess_input.go`, `assess_input_test.go`, `ai_use_assess_test.go`, `compose_mirror.go`, `compose_parent.go`, `compose_parent_test.go`, `compose_parent_stage.go`, `compose_parent_stage_test.go`.
- **`agent/assess_report.go` — SPLIT, do NOT delete whole.** The `AssessReport()` generation function + its prompt/input wiring go, but the **types** (`Report`, `DepthDim`, `AutonomySignal`, and the nested type graph that `teacher.DBadge/ABadge` and `studio.ReportDTO` unmarshal) MUST stay for the deferred roster/weekly cluster. Extract the surviving types into `agent/report_types.go`; delete the generator func + `assess_report_test.go` cases that exercise generation (keep any type-level tests). The compiler + the kept roster/course/teacher packages define exactly which types survive.
- API handlers: `api/assessment.go` (`getAssessment`, the orphaned `generateProjectReport`, `reportDTOFromEvaluationRow`, all `*FromProject` helpers — verified unused by teacher/course) + `assessment_test.go`; `api/growth_history.go` (**relocate `uuidText` first**) + `growth_history_test.go`; `api/chat_assessment.go` + `chat_assessment_test.go`; `api/ability.go` + `ability_test.go`; `api/parent_report.go` + `parent_report_test.go`; `api/parent_stage_report.go` + `parent_stage_report_test.go`.
- Packages (OLD-only — verified importers all in the delete set): `internal/ability/` (importers: `api/ability.go`, `api/parent_stage_report.go`); `internal/parent/` (importer: `api/parent_report.go`).
- **KEEP `internal/rubric/` and `internal/studio/report_dto.go`** (see B.0) even though deleted files import them.

**SQL query files (whole-file delete):** `store/queries/parent.sql` (`GetParentReportProse`/`InsertParentReportProse`). For `store/queries/evaluation.sql`: delete the old generation/read queries (`GetLatestProjectEvaluation`, `Insert/GetLatest Session/ThreadEvaluation`, `ListGrowthHistory`, `ListEvaluationsByUser`); **retain a minimal `InsertProjectEvaluation`** only if teacher tests seed through it (else delete and seed via SQL).

**SQL query blocks inside shared files (delete named blocks only):**
- `store/queries/workspace.sql` — `GetProjectMirror`, `InsertProjectMirror`. KEEP everything else (reflection, summary prose).
- `store/queries/teacher.sql` — delete only blocks that exclusively served the deleted per-student OLD report (`GetStudentProjectEvaluationForTeacher`, `GetStudentThreadEvaluationForTeacher`, `ListStudentEvaluationsForTeacher` if unused). KEEP roster/detail/weekly blocks (`ListClassRosterReport`, `ListStudentReportsForTeacher`, `GetLatestReportScoresForStudent`, `ListStudentProjectsForTeacher`, `GetStudentWeekStats`).

**Tables to DROP (clean, no live consumers):** `project_mirror_prose`, `parent_report_prose`. **Do NOT drop** `evaluations`, `student_evaluation`, `llm_usage`.

**sqlc regen:** removing the above query files/blocks regenerates away `evaluation.sql.go` (or trims it), `parent.sql.go`, and the mirror funcs in `workspace.sql.go`; the `ProjectMirrorProse`/`ParentReportProse` model structs disappear with the table drops. `Evaluation` model struct stays (table kept).

### B.2 DELETE — frontend + contracts (verified, OLD-only)

**Components** — `shell/report/DualAxisReport.tsx`, `shell/growth/GrowthReport.tsx`, `shell/chat/ChatReport.tsx`, `console/EvidenceMap.tsx`, `console/ParentReport.tsx`, `console/ParentReportChrome.tsx`, `console/ParentStageReport.tsx`, `console/parentReportContent.ts`.

**API clients** — `api/assessment.ts` (`getAssessment`, already zero callers), `api/chatAssessment.ts`, `api/growth.ts`.

**Contracts** — `packages/contracts/src/dualAxisReport.ts`, `growthHistory.ts`, `parentReport.ts` (remove their `export *` lines in `index.ts:10,20,24`). **KEEP `dualaxis.json`** (rubric anchors, used by `rubric.ts:2`).

**Tests (delete outright)** — web: `test/api/assessment.test.ts`, `test/api/growth.test.ts`, `test/console/EvidenceMap.test.tsx`, `test/console/ParentReport.test.tsx`, `test/console/ParentStageReport.test.tsx`, `test/shell/chat/ChatReport.test.tsx`, `test/shell/growth/GrowthReport.test.tsx`, `test/shell/report/DualAxisReport.test.tsx`; contracts: `test/dualAxisReport.test.ts`, `test/growthHistory.test.ts`, `test/parentReport.test.ts`.

**Chat surface** — `shell/chat/` is already nav-unreachable (`ChatContainer`/`ChatSurface`/`ChatReport`). Delete the whole dir (chat assessment gone entirely, per scope decision 3).

### B.3 EDIT — frontend references to strip

- **`console/TeacherReportView.tsx` → rewire to new-only.** Remove: `TeacherReport` import, `EvidenceMap`/`ParentReport` imports, the `getStudentReport` fetch + `data`/`error`/`parentReportOpen` state, `dataSettled` gating, `data?.context…` title/chips fallbacks, `canExportParent`, the 导出家长版 PDF button block, the entire 证据地图 section, the ParentReport overlay. Keep: `EvaluationReport`/`EvaluationReportView`, `getStudentEvaluationReport` fetch + envelope handling (updated for Part A's polling), header shell, the non-project placeholder (serves as the course/chat stub).
- **`console/StudentDetailView.tsx`** — remove `ParentReport`/`ParentStageReport` imports, their state, the two 导出家长版 buttons, both overlays. Keep `getStudentDetail`, records list, the 查看完整能力报告 → `onOpenReport` (project report nav).
- **`console/ConsoleShell.tsx`** — drop `getStudentReport` from the `ConsoleClient` Pick (keep `getStudentEvaluationReport`).
- **`shell/AppShell.tsx`** — drop `getStudentReport` from the console-client Pick (L16).
- **`api/index.ts`** — remove imports + `ApiClient` interface members + `api` object entries for `getAssessment`, `getChatAssessment`, `generateChatAssessment`, `getGrowthHistory`, `getStudentReport`, `getParentReport`, `generateParentReportProse`, `getParentStageReport`, `generateParentStageProse`; drop the `DualAxisReport`/`GrowthHistoryEntry`/`ParentReport`/`ParentStageReport`/`TeacherReport` type imports+re-exports. Keep `getStudentEvaluationReport`, `CourseReport`→(stub, see B.4), `EvaluationReport`.
- **`api/teacher.ts`** — delete `TeacherReport` interface, `getStudentReport`, `getParentReport`, `generateParentReportProse`, `getParentStageReport`, `generateParentStageProse`, and the `DualAxisReport`/`ParentReport`/`ParentStageReport` type imports. Keep roster/weekly/`getStudentDetail`/`getStudentEvaluationReport`.
- **`packages/contracts/src/index.ts`** — remove L10/L20/L24 exports.
- **Mixed tests to rewrite (component survives):** `test/console/TeacherReportView.test.tsx`, `test/console/ConsoleShell.test.tsx`, `test/console/StudentDetailView.test.tsx` — strip old fetch/fixtures/assertions; keep new-report + project-nav cases.

### B.4 STUB — course completion placeholder

Course completion is **not** deleted, just inert (course model unsettled):
- `shell/courses/CoursesContainer.tsx:25-33` — the `view.name === "report"` branch renders `<CourseReport>`; replace body with an inert 敬请期待 placeholder (keep the `onFinish → report` transition so the flow still resolves).
- `shell/courses/CourseReport.tsx` — reduce to the placeholder (or delete and inline the stub). Does NOT depend on DualAxisReport.
- Orphaned once stubbed: `api/courses.ts:31-32` `getCourseReport` + its `index.ts` wiring (L12,60,134); `packages/contracts/src/course.ts:97-116` `CourseReport` zod type — remove these, keep `CourseSummary`/`CoursePlayerPayload`/`CourseProgress`.
- `test/shell/courses/CourseReport.test.tsx` — delete/rewrite to the stub.

### B.5 KEEP — MUST NOT delete (new pipeline + shared)

- New: `evaluationReport.ts` (contract) + `index.ts:49` export; `shell/report/EvaluationReport/*`, `EvaluationReportPage.tsx`, `shell/assessment/AssessmentView.tsx`, `shell/growth/ToolkitCards.tsx`, `api/evaluationReport.ts`, `getStudentEvaluationReport`.
- **`dualaxis.json`** (rubric anchors, `rubric.ts:2`) — NOT the dual-axis report contract.
- **Contracts `EvidenceMap` type** + `api/evidenceMap.ts` + `workspace/blocks/exploration/ResearchPanel.tsx` — the ESSAY research pipeline, unrelated to the deleted console `EvidenceMap.tsx`.
- Shared infra old code also imports: `api/client.ts` (`apiFetch`/`ApiError`), `ui/*`, `console/badgeColor.ts`/`csv.ts`/`time.ts`; weekly-report + roster chain in `teacher.ts`.

---

## Migrations & sequencing (critical — deletes break sqlc/compile if mis-ordered)

Old queries reference old tables; sqlc regeneration fails if a table is dropped while a query still selects it, and Go won't compile while handlers call deleted sqlc funcs. Order per the plan:

1. Backend: split `assess_report.go` (keep types), relocate `uuidText`, delete old handlers + route registrations + old `.sql` query files/blocks (mirror in `workspace.sql`, parent.sql, old generation queries in `evaluation.sql`, dead per-student-report blocks in `teacher.sql`); **regenerate sqlc**. Keep `go build ./...` green at each deletion batch — the compiler is the arbiter of what the kept roster/course/teacher cluster still needs.
2. Add migration to **drop only** `project_mirror_prose` + `parent_report_prose`. Do NOT touch `evaluations`/`student_evaluation`/`llm_usage`.
3. Add migration to **reshape `evaluation_report`** for the lifecycle (Part A: nullable `report`, `status`, `UNIQUE(project_id)`), add lifecycle queries, regenerate sqlc; rewire `generateAndStoreEvaluationReport` + the read/list endpoints to the envelope.
4. Frontend: delete old components/clients/contracts, strip references, rewire the teacher per-student report, add course stub, implement the polling read path.
5. Full suites green (backend `go test ./...` with `-timeout 1800s`; web vitest + tsc; contracts).

## Testing

- **Lifecycle unit/integration:** concurrent `generateAndStoreEvaluationReport` on one project produces exactly one `ready` row (claim single-flight); a `failed` row is re-claimable; a fresh `generating` row is NOT re-claimable; GET returns the correct envelope per status.
- **Finish flow:** finish → one `ready` report; a page-open during a (simulated slow) generation does not create a second row.
- **Teacher end:** renders the new report via the envelope; no references to deleted old fetches remain.
- **Regression:** every deleted symbol's test is removed; no dangling imports; `go vet`/tsc clean.

## Out of scope (explicit)

- The real generation algorithm (colleague's) — the placeholder stays; only the seam/lifecycle around it changes.
- Building the PDF export (parent or student) — only the disabled button/seam remains.
- Course completion logic — placeholder only.
- Backfilling old finished projects with real reports — deferred to when the real generator lands.
