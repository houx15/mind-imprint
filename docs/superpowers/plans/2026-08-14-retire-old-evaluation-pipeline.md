# Retire the Old Evaluation Pipeline — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make the new `EvaluationReport` the pipeline for every report *page*; delete the old generation + student-facing rendering + parent report; add a generation lifecycle (status/claim) so the long generator can't double-fire. Leave the teacher roster/weekly/student-detail cluster + `evaluations` table untouched (separate future design).

**Architecture:** Same as before — finish → generate → store → read → render. Only the report object changes (DualAxis/`evaluations` → `EvaluationReport`/`evaluation_report`) and gains a status lifecycle. Backend deletion is compiler-gated: `go build ./...` green after every batch. Frontend deletion is tsc-gated.

**Tech Stack:** Go (net/http + pgx + sqlc + goose + testcontainers), React + Vite + TS + Tailwind, Zod contracts.

**Authoritative spec:** `docs/superpowers/specs/2026-08-14-retire-old-evaluation-pipeline-design.md`. Its **B.0** (keep-cluster + MUST-KEEP), **B.1** (backend delete), **B.2/B.3/B.4/B.5** (frontend delete/edit/stub/keep), and **Part A** (lifecycle) are the verified inventory — this plan sequences them and supplies the constructive code. When a task says "per spec B.x", that list is the exact set; do not re-derive it.

## Global Constraints

- **`go build ./...` (from `apps/api`) MUST pass at the end of every backend task.** The compiler is the arbiter of what the kept roster/course/teacher cluster still needs. If a "delete" breaks the build via a kept consumer, that consumer was mis-classified — keep the symbol (relocate if needed), don't hack the consumer.
- **NEVER drop `evaluations`, `student_evaluation`, or `llm_usage`.** Only `project_mirror_prose` and `parent_report_prose` are dropped.
- **KEEP `internal/rubric/`** (imported by live `agent/moment.go`) and **`internal/studio/report_dto.go`** (used by `course.go`/`teacher_read.go`/`teacher_weekly.go`).
- **`assess_report.go` is SPLIT, not deleted** — the `Report`/`DepthDim`/`AutonomySignal` type graph that `teacher.DBadge/ABadge` + `studio.ReportDTO` unmarshal MUST survive.
- **Relocate `uuidText`** out of the deleted `growth_history.go` (roster/detail call it) before deleting that file.
- Go tests run with `-timeout 1800s`. sqlc regen: pin `sqlc` `@v1.27.0`, `CGO_ENABLED=0`; after any `.sql` change run the repo's sqlc-generate step and commit the generated `*.sql.go`.
- Migrations are append-only forward files under `apps/api/internal/store/migrations/`; next numbers are `0065`, `0066`.
- `git add` explicit paths, never `-A`. Commit after each task.
- Envelope JSON shape is fixed: `null` | `{status:"generating"}` | `{status:"failed"}` | `{status:"ready", report:{…}}`. Zod `.nullable()`→Go `*T` (marshals null); `.optional()`→`omitempty`.
- The four 铁律 still bind (observation-not-grading; D/A by color; no ghostwriting). This is deletion + rewire, not a content-meaning change.

---

## Task 1: Backend — remove old API handlers + route registrations

**Files:**
- Modify: `apps/api/internal/api/api.go` (route registrations), `apps/api/internal/api/teacher_read.go` (relocate `uuidText`; delete `getStudentReport`), `apps/api/internal/api/workspace_review.go` (delete mirror half, keep reflection half), `apps/api/internal/api/project_finish_test.go` (drop refs to orphaned `generateProjectReport`/`composeAndStoreProjectMirror`)
- Delete: `apps/api/internal/api/{assessment.go,assessment_test.go,growth_history.go,growth_history_test.go,chat_assessment.go,chat_assessment_test.go,ability.go,ability_test.go,parent_report.go,parent_report_test.go,parent_stage_report.go,parent_stage_report_test.go}`
- Modify: `apps/api/internal/api/workspace_review_test.go` (drop mirror tests, keep reflection tests)

**Interfaces:**
- Consumes: nothing new.
- Produces: after this task, the agent generation funcs (`AssessReport`, `ComposeMirror`, `ComposeParent*`) and old sqlc queries are uncalled-but-present (Go allows unused exported symbols) — Task 2/3 remove them.

- [ ] **Step 1: Relocate `uuidText`.** Move `func uuidText(id pgtype.UUID) string` from `growth_history.go` into `teacher_read.go` (it is called at `teacher_read.go:220,225`). Keep the signature identical. Ensure `teacher_read.go` imports `pgtype`.

- [ ] **Step 2: Delete the mirror half of `workspace_review.go`.** Remove `mirrorDTO`, `mirrorSectionDTO`, `mirrorDTOFromRow`, `getMirror`, `postMirror`, `composeAndStoreProjectMirror`, `toMirrorSectionDTOs`, `composeMirror`, `buildMirrorInput`, `outlineDigest`, `processDigest`. **KEEP** `reflectionDTO`, `decodeReflectionAnswers`, `getReflection`, `putReflection` (still gate `finishProject` + serve the Review reflection doc). In `workspace_review_test.go` delete mirror tests, keep reflection tests.

- [ ] **Step 3: Delete `getStudentReport`** from `teacher_read.go` (the OLD per-student report handler, ~lines 310–383) — the teacher per-student report is served by the NEW `getStudentEvaluationReport`. Keep `getClassRosterReport`, `getStudentDetail`, and everything else in the file.

- [ ] **Step 4: Delete the OLD handler files** listed above (assessment / growth_history / chat_assessment / ability / parent_report / parent_stage_report + their tests).

- [ ] **Step 5: Remove the route registrations** in `api.go` (per spec B.1 EDIT table): `GET /growth/history`, `GET /growth/ability`, `GET|POST /projects/{id}/mirror`, `GET /projects/{id}/assessment`, `GET|POST /chat/threads/{id}/assessment`, `GET .../students/{userId}/reports/{surface}/{scopeId}`, `GET|POST .../parent-report/...`, `GET|POST .../parent-stage-report/...`. **KEEP** `GET /growth/cards`, the `evaluation-report` routes, `summary` routes, roster/detail/weekly routes.

- [ ] **Step 6: Fix `project_finish_test.go`** — remove references to the now-deleted orphans `generateProjectReport` / `composeAndStoreProjectMirror` (per map, ~lines 12, 283). Do NOT change the finish behavior assertions.

- [ ] **Step 7: Build + test.**

Run: `cd apps/api && go build ./... && go test ./internal/api/... -timeout 1800s`
Expected: PASS (unused generation funcs + old queries remain, that's fine).

- [ ] **Step 8: Commit.**

```bash
git add apps/api/internal/api
git commit -m "refactor(api): remove old evaluation/mirror/parent/growth handlers + routes"
```

---

## Task 2: Backend — delete old agent generation + split `assess_report.go` + drop OLD-only packages

**Files:**
- Delete: `apps/api/internal/agent/{assess_report_prompt.go,assess_input.go,assess_input_test.go,ai_use_assess_test.go,compose_mirror.go,compose_parent.go,compose_parent_test.go,compose_parent_stage.go,compose_parent_stage_test.go}`
- Modify/split: `apps/api/internal/agent/assess_report.go` → keep types (extract to `apps/api/internal/agent/report_types.go`), delete `AssessReport()` generator; trim `assess_report_test.go` to type-level tests (or delete if all cases are generation)
- Delete packages: `apps/api/internal/ability/` (whole dir), `apps/api/internal/parent/` (whole dir)

**Interfaces:**
- Consumes: the kept `teacher.DBadge/ABadge` read `agent.Report.DepthAxis[].Level` and `agent.Report.AutonomyAxis[].Level/.Opportunity`; `studio.ReportDTO` unmarshals `agent.Report`.
- Produces: `agent.Report` (+ nested types) survives in `report_types.go`; no generation code remains.

- [ ] **Step 1: Split `assess_report.go`.** Create `apps/api/internal/agent/report_types.go` containing the surviving type declarations: `Report`, `DepthDim`, `AutonomySignal`, and every nested type still referenced by kept packages (`PromptLens`, `InteractionRow`, `OfficialProjection`, `WorkAndProcess`, `Guidance`, etc. — include whatever `studio.ReportDTO`/`teacher` need; the compiler in Step 4 tells you the exact set). Delete `AssessReport()` and any generation-only helper/prompt wiring from `assess_report.go` (delete the file once emptied of everything but the moved types). Keep package `agent`.

- [ ] **Step 2: Delete the generation files** listed above (compose_*, assess_input*, assess_report_prompt, ai_use_assess_test).

- [ ] **Step 3: Delete the OLD-only packages** `internal/ability/` and `internal/parent/` (their only importers — `api/ability.go`, `api/parent_report.go`, `api/parent_stage_report.go` — were deleted in Task 1).

- [ ] **Step 4: Build.** `cd apps/api && go build ./...`
Expected: PASS. If it fails because a kept package (`studio`, `teacher`, `course`, `api`) references a type you didn't move, MOVE that type into `report_types.go` — do not delete the consumer.

- [ ] **Step 5: Test.** `go test ./internal/agent/... ./internal/studio/... ./internal/teacher/... -timeout 1800s`
Expected: PASS (badge tests + report_dto tests still green using the moved types).

- [ ] **Step 6: Commit.**

```bash
git add apps/api/internal/agent apps/api/internal/ability apps/api/internal/parent
git commit -m "refactor(agent): delete old report generation; keep scoring types for roster/weekly"
```

---

## Task 3: Backend — delete old SQL queries, drop the two prose tables, regenerate sqlc

**Files:**
- Delete: `apps/api/internal/store/queries/parent.sql`
- Modify: `apps/api/internal/store/queries/evaluation.sql` (delete old gen/read queries; keep a minimal seed path if tests need it — see Step 2), `apps/api/internal/store/queries/workspace.sql` (delete `GetProjectMirror`, `InsertProjectMirror`), `apps/api/internal/store/queries/teacher.sql` (delete only dead per-student-report blocks; keep roster/detail/weekly)
- Create: `apps/api/internal/store/migrations/0065_drop_mirror_parent_prose.sql`
- Regenerate: `apps/api/internal/store/sqlc/*` (via sqlc)
- Modify test seeders that used deleted queries: `apps/api/internal/api/{project_finish_test.go,teacher_read_test.go}`, `apps/api/internal/store/{migrate_0021_test.go,growth_history_query_test.go,evaluations_by_user_query_test.go,eval_async_migrate_test.go,teacher_query_test.go}` (delete/trim per what they exercise)

**Interfaces:**
- Consumes: nothing new.
- Produces: `evaluation.sql.go` and `parent.sql.go` gone/trimmed; `workspace.sql.go` loses the mirror funcs; `ProjectMirrorProse`/`ParentReportProse` model structs gone; `Evaluation` struct KEPT (table kept).

- [ ] **Step 1: Delete `parent.sql`** and the mirror query blocks in `workspace.sql` (`GetProjectMirror`, `InsertProjectMirror`).

- [ ] **Step 2: Trim `evaluation.sql`.** Delete `GetLatestProjectEvaluation`, `InsertSessionEvaluation`, `GetLatestSessionEvaluation`, `InsertThreadEvaluation`, `GetLatestThreadEvaluation`, `ListGrowthHistory`, `ListEvaluationsByUser`. **Decision on `InsertProjectEvaluation`:** the teacher roster/detail tests need to seed `evaluations`. Keep `InsertProjectEvaluation` (rename its `-- name:` comment to signal test-seed use is fine) IF those tests call it; otherwise delete it and have the tests seed via a raw `INSERT` in the testcontainer. Pick whichever keeps `teacher_read_test.go` green with the least churn.

- [ ] **Step 3: Trim `teacher.sql`.** Delete ONLY blocks that exclusively served the deleted `getStudentReport`: `GetStudentProjectEvaluationForTeacher`, `GetStudentThreadEvaluationForTeacher`, and `ListStudentEvaluationsForTeacher` **iff** grep confirms no kept handler calls it. KEEP `ListClassRosterReport`, `ListStudentReportsForTeacher`, `GetLatestReportScoresForStudent`, `ListStudentProjectsForTeacher`, `GetStudentWeekStats`.

- [ ] **Step 4: Write migration `0065_drop_mirror_parent_prose.sql`.**

```sql
-- +goose Up
DROP TABLE IF EXISTS project_mirror_prose;
DROP TABLE IF EXISTS parent_report_prose;

-- +goose Down
-- (no-op: these tables are retired; recreate from history if ever needed)
```

- [ ] **Step 5: Regenerate sqlc** and delete the now-empty generated files. Run the repo's sqlc generate (sqlc `@v1.27.0`, `CGO_ENABLED=0`). Confirm `parent.sql.go` is gone, `evaluation.sql.go` is gone-or-trimmed, `workspace.sql.go` has no mirror funcs, `models.go` no longer has `ProjectMirrorProse`/`ParentReportProse`, and `Evaluation` remains.

- [ ] **Step 6: Fix test seeders.** Update the test files above so they compile against the trimmed query set (seed `evaluations` via the retained insert or raw SQL; delete tests that only exercised deleted queries like `growth_history_query_test.go`, `evaluations_by_user_query_test.go`).

- [ ] **Step 7: Build + full store/api tests.**

Run: `cd apps/api && go build ./... && go test ./internal/store/... ./internal/api/... -timeout 1800s`
Expected: PASS (roster/weekly/finish still green; testcontainers apply 0065).

- [ ] **Step 8: Commit.**

```bash
git add apps/api/internal/store apps/api/internal/api
git commit -m "refactor(store): drop mirror/parent prose tables + old eval queries; regen sqlc"
```

---

## Task 4: Backend — generation lifecycle (status + atomic claim + envelope)

**Files:**
- Create: `apps/api/internal/store/migrations/0066_evaluation_report_status.sql`
- Modify: `apps/api/internal/store/queries/evaluation_report.sql` (replace Get/List, add Claim/Complete/Fail)
- Regenerate: `apps/api/internal/store/sqlc/evaluation_report.sql.go`
- Modify: `apps/api/internal/api/evaluation_report.go` (envelope + claim/complete/fail), `apps/api/internal/evalreport/` (no change to Placeholder shape)
- Test: `apps/api/internal/api/evaluation_report_lifecycle_test.go` (new)

**Interfaces:**
- Consumes: `evalreport.Placeholder(...)` unchanged.
- Produces: `ClaimEvaluationReportGeneration(ctx, projectID) (uuid.UUID, error)` (pgx.ErrNoRows = not claimed), `CompleteEvaluationReport`, `FailEvaluationReport`, `GetEvaluationReport`, `ListEvaluationReports`; `generateAndStoreEvaluationReport` is now single-flight. GET/teacher-GET return the envelope.

- [ ] **Step 1: Migration `0066_evaluation_report_status.sql`.**

```sql
-- +goose Up
ALTER TABLE evaluation_report ALTER COLUMN report DROP NOT NULL;
ALTER TABLE evaluation_report ADD COLUMN status text NOT NULL DEFAULT 'ready'
  CHECK (status IN ('generating','ready','failed'));
DROP INDEX IF EXISTS evaluation_report_project_created_idx;
ALTER TABLE evaluation_report ADD CONSTRAINT evaluation_report_project_key UNIQUE (project_id);

-- +goose Down
ALTER TABLE evaluation_report DROP CONSTRAINT evaluation_report_project_key;
CREATE INDEX evaluation_report_project_created_idx ON evaluation_report (project_id, created_at DESC);
ALTER TABLE evaluation_report DROP COLUMN status;
ALTER TABLE evaluation_report ALTER COLUMN report SET NOT NULL;
```

- [ ] **Step 2: Rewrite `evaluation_report.sql`** — replace `GetLatestEvaluationReport` + `ListEvaluationReports` and add the lifecycle queries (verbatim from spec Part A):

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

-- name: CompleteEvaluationReport :exec
UPDATE evaluation_report SET report = @report, status = 'ready' WHERE project_id = @project_id;

-- name: FailEvaluationReport :exec
UPDATE evaluation_report SET status = 'failed' WHERE project_id = @project_id AND status = 'generating';

-- name: GetEvaluationReport :one
SELECT * FROM evaluation_report WHERE project_id = @project_id;

-- name: ListEvaluationReports :many
SELECT er.project_id, er.created_at, p.title, p.qualification
FROM evaluation_report er
JOIN project p ON p.id = er.project_id
WHERE p.user_id = @user_id AND er.status = 'ready'
ORDER BY er.created_at DESC;
```

- [ ] **Step 3: Regenerate sqlc.** Confirm `InsertEvaluationReport`/`GetLatestEvaluationReport` old funcs are replaced by the new set; `Report` column is now `[]byte`/nullable in the model.

- [ ] **Step 4: Rewrite `generateAndStoreEvaluationReport`** to claim → generate → complete/fail (spec Part A Go block). On `pgx.ErrNoRows` from the claim, return `nil` (already owned/generated — no double-generate). On any post-claim error, call `FailEvaluationReport` then return the error.

- [ ] **Step 5: Envelope for the read endpoints.** Define a small response encoder in `evaluation_report.go`:

```go
// evalReportEnvelope renders the three-state read contract.
// nil row  → JSON null
// generating/failed → {"status":"..."}
// ready → {"status":"ready","report":{…}}
func (a *API) writeEvalReportEnvelope(w http.ResponseWriter, r *http.Request, row sqlc.EvaluationReport, hadRow bool) {
    if !hadRow {
        httpx.WriteJSON(w, http.StatusOK, nil)
        return
    }
    switch row.Status {
    case "ready":
        rep, verr := evalreport.Validate(row.Report)
        if verr != nil { httpx.WriteError(w, r, verr); return }
        httpx.WriteJSON(w, http.StatusOK, map[string]any{"status": "ready", "report": rep})
    default: // generating | failed
        httpx.WriteJSON(w, http.StatusOK, map[string]any{"status": row.Status})
    }
}
```

  Update `getEvaluationReport`, `getStudentEvaluationReport`, and `postGenerateEvaluationReport` to use `GetEvaluationReport` + `writeEvalReportEnvelope` (POST: claim-and-generate if absent via `generateAndStoreEvaluationReport`, then re-read + envelope). Update `listEvaluationReports` to the new `ListEvaluationReports` row shape (fields unchanged: projectId/title/type/createdAt).

- [ ] **Step 6: Update the finish worker** `runProjectReport` — no behavior change needed (it already calls `generateAndStoreEvaluationReport`), but confirm it still marks `SetProjectFinished` on success and rolls back on error. The claim inside guarantees single-flight vs. a concurrent page-open POST.

- [ ] **Step 7: Lifecycle test** `evaluation_report_lifecycle_test.go` (testcontainers):
  - concurrent `generateAndStoreEvaluationReport` (2 goroutines, same project) → exactly one `ready` row (query `COUNT(*)`).
  - a `failed` row is re-claimable (set status=failed, claim returns a row).
  - a fresh `generating` row is NOT re-claimable (claim returns ErrNoRows).
  - GET envelope: none→null, generating→`{"status":"generating"}`, ready→`{"status":"ready",...}`.

- [ ] **Step 8: Build + test.** `cd apps/api && go build ./... && go test ./internal/api/... ./internal/store/... -timeout 1800s`
Expected: PASS.

- [ ] **Step 9: Commit.**

```bash
git add apps/api/internal
git commit -m "feat(api): evaluation report generation lifecycle — status + atomic claim + envelope"
```

---

## Task 5: Frontend — envelope client + report page polling state machine

**Files:**
- Modify: `apps/web/src/api/evaluationReport.ts` (envelope return types), `apps/web/src/api/teacher.ts` (`getStudentEvaluationReport` → envelope)
- Modify: `apps/web/src/shell/report/EvaluationReportPage.tsx` (poll state machine)
- Test: `apps/web/test/shell/report/EvaluationReportPage.test.tsx` (new or updated), `apps/web/test/api/evaluationReport.test.ts` (if present)

**Interfaces:**
- Consumes: backend envelope (`null` | `{status}` | `{status:"ready",report}`).
- Produces: `EvalReportEnvelope` type + `getEvaluationReport`/`getStudentEvaluationReport` returning it; page renders ready / polls generating / errors failed.

- [ ] **Step 1: Envelope type + client.** In `api/evaluationReport.ts`:

```ts
export type EvalReportEnvelope =
  | { status: "generating" }
  | { status: "failed" }
  | { status: "ready"; report: EvaluationReport };

export async function getEvaluationReport(projectId: string): Promise<EvalReportEnvelope | null> {
  const raw = await apiFetch<unknown>(`/api/v1/projects/${projectId}/evaluation-report`);
  return parseEnvelope(raw);
}
export async function generateEvaluationReport(projectId: string): Promise<EvalReportEnvelope | null> {
  const raw = await apiFetch<unknown>(`/api/v1/projects/${projectId}/evaluation-report/generate`, { method: "POST" });
  return parseEnvelope(raw);
}
function parseEnvelope(raw: unknown): EvalReportEnvelope | null {
  if (raw == null) return null;
  const o = raw as { status?: string; report?: unknown };
  if (o.status === "ready") return { status: "ready", report: EvaluationReport.parse(o.report) };
  if (o.status === "generating") return { status: "generating" };
  if (o.status === "failed") return { status: "failed" };
  return null;
}
```

  Mirror the same envelope parsing for `getStudentEvaluationReport` in `api/teacher.ts` (it returns `EvalReportEnvelope | null`).

- [ ] **Step 2: Report page state machine.** Rewrite `EvaluationReportPage`'s effect to: fetch GET → if `null` POST generate → then branch on envelope. On `generating`, set a `generating` state and poll GET every 3s (clear on unmount) until `ready`/`failed`. States: `loading | generating | error | ready`. `ready`→`EvaluationReportView`; `generating`→spinner + rotating caption; `error`/`failed`→EmptyState + 重试 (re-runs the effect). Keep the existing top bar (返回 + disabled 导出 PDF).

- [ ] **Step 3: Test** the page: ready renders body; generating polls then flips to ready (fake timers + mocked client); failed shows retry. Mock the client module.

- [ ] **Step 4: tsc + vitest.** `cd apps/web && npx tsc --noEmit && npx vitest run test/shell/report test/api/evaluationReport.test.ts`
Expected: PASS.

- [ ] **Step 5: Commit.**

```bash
git add apps/web/src/api/evaluationReport.ts apps/web/src/api/teacher.ts apps/web/src/shell/report/EvaluationReportPage.tsx apps/web/test/shell/report apps/web/test/api
git commit -m "feat(web): evaluation report envelope + polling state machine"
```

---

## Task 6: Frontend — rewire the teacher per-student report to new-only

**Files:**
- Modify: `apps/web/src/console/TeacherReportView.tsx` (envelope + drop old), `apps/web/src/console/StudentDetailView.tsx` (drop parent), `apps/web/src/console/ConsoleShell.tsx` (drop `getStudentReport` from Pick), `apps/web/src/shell/AppShell.tsx` (drop `getStudentReport` from Pick)
- Modify tests: `apps/web/test/console/{TeacherReportView.test.tsx,ConsoleShell.test.tsx,StudentDetailView.test.tsx}`

**Interfaces:**
- Consumes: `getStudentEvaluationReport` → `EvalReportEnvelope | null` (Task 5).
- Produces: teacher report view renders new report via envelope; no imports of `EvidenceMap`/`ParentReport`/`getStudentReport`.

- [ ] **Step 1: `TeacherReportView` → new-only** per spec B.3: remove the `getStudentReport` fetch + `data`/`error`/`parentReportOpen` state, `EvidenceMap`/`ParentReport` imports + sections, the 导出家长版 PDF button, the old title/chips fallbacks. Drive the body off `getStudentEvaluationReport`'s envelope (ready→`EvaluationReportView`; generating→"生成中"; failed→retry; null→"未生成"). Keep the header shell + the non-project placeholder (course/chat stub).

- [ ] **Step 2: `StudentDetailView`** — remove `ParentReport`/`ParentStageReport` imports, their state, the two 导出家长版 buttons + overlays. Keep records list + the 查看完整能力报告 → `onOpenReport` project-report nav.

- [ ] **Step 3: `ConsoleShell` + `AppShell`** — drop `getStudentReport` from the console-client `Pick<>` unions (keep `getStudentEvaluationReport`).

- [ ] **Step 4: Rewrite the three console tests** — strip old `getStudentReport`/`DualAxisReport`/EvidenceMap/ParentReport fixtures + assertions; keep new-report + project-nav cases. `getStudentEvaluationReport` mock now returns an envelope.

- [ ] **Step 5: tsc + vitest.** `cd apps/web && npx tsc --noEmit && npx vitest run test/console`
Expected: PASS.

- [ ] **Step 6: Commit.**

```bash
git add apps/web/src/console apps/web/src/shell/AppShell.tsx apps/web/test/console
git commit -m "refactor(console): teacher per-student report renders new EvaluationReport only"
```

---

## Task 7: Frontend — delete old components/clients/contracts + strip wiring

**Files:**
- Delete (per spec B.2): `apps/web/src/shell/report/DualAxisReport.tsx`, `apps/web/src/shell/growth/GrowthReport.tsx`, `apps/web/src/shell/chat/` (whole dir), `apps/web/src/console/{EvidenceMap.tsx,ParentReport.tsx,ParentReportChrome.tsx,ParentStageReport.tsx,parentReportContent.ts}`, `apps/web/src/api/{assessment.ts,chatAssessment.ts,growth.ts}`, `packages/contracts/src/{dualAxisReport.ts,growthHistory.ts,parentReport.ts}`, and all OLD-only tests listed in B.2.
- Modify: `apps/web/src/api/index.ts` (drop old members per B.3), `apps/web/src/api/teacher.ts` (drop `TeacherReport`/`getStudentReport`/parent members), `packages/contracts/src/index.ts` (remove the 3 `export *` lines)

**Interfaces:**
- Consumes: nothing (deletions).
- Produces: no references to old report modules anywhere; `ApiClient` shed of old members.

- [ ] **Step 1: Strip `api/index.ts`** — remove imports, `ApiClient` interface members, and `api` object entries for `getAssessment`, `getChatAssessment`, `generateChatAssessment`, `getGrowthHistory`, `getStudentReport`, `getParentReport`, `generateParentReportProse`, `getParentStageReport`, `generateParentStageProse`, plus the `DualAxisReport`/`GrowthHistoryEntry`/`ParentReport`/`ParentStageReport`/`TeacherReport` type imports+re-exports.

- [ ] **Step 2: Strip `api/teacher.ts`** — delete `TeacherReport`, `getStudentReport`, `getParentReport`, `generateParentReportProse`, `getParentStageReport`, `generateParentStageProse` + the `DualAxisReport`/`ParentReport`/`ParentStageReport` type imports. Keep roster/weekly/`getStudentDetail`/`getStudentEvaluationReport`.

- [ ] **Step 3: Remove `packages/contracts/src/index.ts`** lines exporting `dualAxisReport`/`growthHistory`/`parentReport`.

- [ ] **Step 4: Delete** all the B.2 component/client/contract files + their OLD-only tests + the `shell/chat/` dir.

- [ ] **Step 5: Grep for dangling refs.** `cd apps/web && grep -rn "DualAxisReport\|GrowthReport\|ChatReport\|EvidenceMap\|ParentReport\|ParentStageReport\|getGrowthHistory\|getStudentReport\|getChatAssessment\|getAssessment" src test` — expect only the NEW-pipeline `EvaluationReport*` names and the contracts `EvidenceMap` essay type (keep). Fix any stragglers.

- [ ] **Step 6: contracts + tsc + vitest.** `cd packages/contracts && npx vitest run` then `cd apps/web && npx tsc --noEmit && npx vitest run`
Expected: PASS.

- [ ] **Step 7: Commit.**

```bash
git add apps/web/src apps/web/test packages/contracts/src packages/contracts/test
git commit -m "refactor(web): delete old evaluation components, clients, and contracts"
```

---

## Task 8: Frontend — course completion placeholder stub

**Files:**
- Modify: `apps/web/src/shell/courses/CoursesContainer.tsx` (report branch → placeholder), `apps/web/src/shell/courses/CourseReport.tsx` (reduce to stub or delete), `apps/web/src/api/courses.ts` + `apps/web/src/api/index.ts` (drop `getCourseReport` if orphaned), `packages/contracts/src/course.ts` (remove `CourseReport` type, keep the rest)
- Modify/delete test: `apps/web/test/shell/courses/CourseReport.test.tsx`

**Interfaces:**
- Consumes: nothing.
- Produces: course-completion surface shows an inert 敬请期待 placeholder; `onFinish → report` transition still resolves.

- [ ] **Step 1: Placeholder.** In `CoursesContainer.tsx`, replace the `view.name === "report"` `<CourseReport>` body with an inert placeholder (`EmptyState`/simple card: "课程完成报告即将上线"). Keep the `onFinish → report` transition so completing a course still lands somewhere coherent (a 完成 confirmation + back-to-courses).

- [ ] **Step 2: Remove `CourseReport` component** (or reduce to the placeholder), and drop `getCourseReport` from `api/courses.ts` + `api/index.ts` and the `CourseReport` zod type from `contracts/src/course.ts` (keep `CourseSummary`/`CoursePlayerPayload`/`CourseProgress`). Delete/rewrite `CourseReport.test.tsx`.

- [ ] **Step 3: Grep + tsc + vitest.** `cd apps/web && grep -rn "CourseReport\|getCourseReport" src test` (expect none), then `npx tsc --noEmit && npx vitest run test/shell/courses` and `cd packages/contracts && npx vitest run`.
Expected: PASS.

- [ ] **Step 4: Commit.**

```bash
git add apps/web/src/shell/courses apps/web/src/api packages/contracts/src/course.ts apps/web/test/shell/courses
git commit -m "refactor(web): course completion report → placeholder stub"
```

---

## Task 9: Full-suite verification

**Files:** none (verification only).

- [ ] **Step 1: Backend full suite.** `cd apps/api && go build ./... && go vet ./... && go test ./... -timeout 1800s` → PASS.
- [ ] **Step 2: Contracts.** `cd packages/contracts && npx vitest run` → PASS.
- [ ] **Step 3: Web.** `cd apps/web && npx tsc --noEmit && npx vitest run` → PASS.
- [ ] **Step 4: Dangling-ref sweep** (backend): `cd apps/api && grep -rn "AssessReport\|ComposeMirror\|ComposeParent\|getStudentReport\|project_mirror_prose\|parent_report_prose\|GetProjectMirror" internal` — expect none except comments/migration-history. Fix stragglers.
- [ ] **Step 5: Commit** any final cleanup.

---

## Self-review notes (author)

- **Spec coverage:** Part A → Task 4/5; B.1 → Tasks 1–3; B.2/B.3 → Tasks 6–7; B.4 → Task 8; B.5 (keep) enforced by Global Constraints + compiler gates. ✅
- **Sequencing:** handlers/routes (T1) before generation funcs (T2) before queries/tables (T3) — so `go build` never sees a caller of a deleted symbol before the caller is gone. Lifecycle (T4) is additive on the kept `evaluation_report`. Frontend envelope (T5) before teacher rewire (T6) before deletions (T7) — so tsc never sees a live importer of a deleted module. ✅
- **Type consistency:** `EvalReportEnvelope` shape identical across `evaluationReport.ts`, `teacher.ts`, and the Go `writeEvalReportEnvelope`. `ClaimEvaluationReportGeneration` returns id / ErrNoRows used identically in `generateAndStoreEvaluationReport`. ✅
- **Risk:** the one judgment-heavy task is T2 (which types survive the `assess_report.go` split) — mitigated by the compiler gate in Step 4. T3's `InsertProjectEvaluation` keep/drop is decided by whichever keeps teacher tests green.
