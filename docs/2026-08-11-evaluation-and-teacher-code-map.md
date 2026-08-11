# Evaluation & Teacher-End Code Map — mind-imprint

> **Audience:** engineers taking over **(A) process evaluation (过程评估)** and **(B) the teacher/admin end (教师端 / 组织端)**. This is a where-does-the-code-live reference: every endpoint, agent, sqlc query, migration, contract, and frontend view, with exact file paths. For where the *data* lands, see the companion **`2026-08-11-evaluation-data-storage-guide.md`**.

**Stack:** frontend `apps/web` (React + Vite + TS), backend `apps/api` (Go, `net/http` + `pgx`/`sqlc`), shared Zod contracts `packages/contracts`. The frontend never talks to a model — all LLM calls go through the Go gateway, which is the only holder of keys and the only writer of `llm_call` metering rows.

**Two cross-cutting invariants to keep in mind before you touch either area:**
1. **Read-no-call / write-only-spend.** `GET` endpoints for any report read stored rows and never call a model. Composition happens only on `POST` (first-open-wins) or inside the detached finish goroutine. Every model call writes an `llm_call` row with a `purpose` string.
2. **Flagship, never downgraded, for evaluation.** All assessment/mirror/summary/parent/review paths resolve their model through the `EvalResolver` seam (flagship tier). Coaching uses a separate fast resolver.

---

# Part A — Process Evaluation

> **Scoring model:** the canonical model is the **dual-axis (双轴)** model, *not* a single 0–5 rubric. Depth axis **D1–D6** with L1–L4 anchors; autonomy axis **A1–A6**, event-counted, band 0–5; plus **6 process lenses**. The two axes are never combined into a total. "0–5" only appears on the autonomy band.

## A1. Rubric / dual-axis model definition
| Path | What it is |
|---|---|
| `apps/api/internal/rubric/dualaxis.go` | Go types + accessors: `DepthDim` (D1–D6, L1–L4 anchors, D6 has `ReflectionRule`), `AutonomySignal` (A1–A6, band 0–5), `Lens` (6 process lenses), `OfficialStandard`/`OfficialComponentSpec` (external-standard alignment, e.g. AP Research). Exposes `Model()`, `DepthDims()`, `AutonomySignals()`, `Lenses()`, `AutonomyBand()`, `Standard(id)`. |
| `apps/api/internal/rubric/dualaxis.json` | Embedded config — the source of truth for the model (axiom: "两轴永不合成总分"; D1 任务理解与问题表述 … D6; A1–A6; lenses; standards). |
| `apps/api/internal/rubric/dualaxis_test.go` | Tests. |
| `packages/contracts/src/rubric.ts` + `dualaxis.json` | Frontend mirror: `DepthDim`/`AutonomySignal`/`Lens`/`DualAxis` schemas + `DUALAXIS_MODEL`. Keep in sync with the Go copy. |

## A2. HTTP handlers / routes (`apps/api/internal/api/`)
Routes are registered in `apps/api/internal/api/api.go`.

**Project-scoped evaluation:**
| Method + path | Handler (file) | Notes |
|---|---|---|
| `GET /api/v1/projects/{id}/assessment` | `getAssessment` (`assessment.go`) | Returns the latest stored dual-axis report; **never calls a model**. |
| — (internal) | `generateProjectReport` (`assessment.go`) | Flagship generator; records `llm_call` purpose `"assessment"`. |
| `POST /api/v1/projects/{id}/self-score` | `submitSelfScore` (`self_score.go`) | Student self-score bands; no model call. |
| `POST /api/v1/projects/{id}/reflection` | `submitReflection` (`reflection.go`) | Flips the reflection-archive gate; no model call. |
| `GET/PUT /api/v1/projects/{id}/reflection-doc` | `getReflection`/`putReflection` (`workspace_review.go`) | 5-dimension reflection doc, plain REST. |
| `GET/POST /api/v1/projects/{id}/mirror` | `getMirror`/`postMirror` (`workspace_review.go`) | 你的思维印记 mirror. POST composes once (first-open-wins) via flagship `EvalResolver`, purpose `"mirror"`. See `composeAndStoreProjectMirror`, `composeMirror`, `buildMirrorInput`. |
| `GET/POST /api/v1/projects/{id}/summary` | `getProjectSummary`/`postProjectSummary` (`workspace_summary.go`) | Summary-on-return; `composeReturnSummary`, purpose `"summary"`. |
| `POST /api/v1/projects/{id}/finish` | `finishProject` (`project_finish.go`) | **Terminal trigger** (see A6). |
| `POST /api/v1/projects/{id}/finish-writing` | `finishWriting` (`project_writing_finish.go`) | Marks a doc finished; the assessment gate. |
| `POST /api/v1/projects/{id}/cards/{cid}/evaluate` | `evaluateProjectCard` (`readeval.go`) | Per-card reading-selection eval via `agent.EvaluateSelection`. |
| `POST /api/v1/projects/{id}/cards/reflect` | `postReflectProjectCard` (`card_reflect.go`) | |

**Review seams** (framework/proposal/essay/exploration reviewers — all flagship `EvalResolver`):
| Method + path | Handler (file) |
|---|---|
| `POST /api/v1/projects/{id}/proposal-track/review` | `reviewProposalPart` (`proposal_track.go`) |
| `POST /api/v1/projects/{id}/proposal-annotations/review` | `reviewProposalAnnotations` (`proposal_annotations.go`) |
| `POST /api/v1/projects/{id}/evidence-map/subquestions/{sqId}/review` | `reviewSubQuestionSaturation` (`evidence_map.go`) |
| `POST /api/v1/projects/{id}/essay-statement/review` | `reviewEssayStatement` (`essay_statement.go`) |
| `POST /api/v1/projects/{id}/exploration/review` | `postExplorationReview` (`exploration_review.go`) |
| `POST /api/v1/projects/{id}/snapshots/{sid}/review` | `orderReview` (`workspace_review.go`) |

**Chat-thread-scoped assessment:**
| Method + path | Handler (file) |
|---|---|
| `GET/POST /api/v1/chat/threads/{id}/assessment` | `getChatAssessment`/`generateChatAssessment` (`chat_assessment.go`) — purpose `"assessment"` via `RecordChatLLMCall`. |

**Growth history / overview:** `overview.go` (purpose `"evaluation"`), `growth_history.go`.
**Studio DTO:** `apps/api/internal/studio/report_dto.go` — `ReportDTO` returned by `generateProjectReport`.

## A3. Agent / LLM layer (`apps/api/internal/agent/`)
| Path | What it does |
|---|---|
| `assess_report.go` | `AssessReport(ctx, prov, resolved, rubric.DualAxis, AssessmentInput)` — the flagship dual-axis report generator; parses/normalizes/clamps depth L1–L4, autonomy 0–5, lenses (`enforceReport`, `AnchoredNilGuards`). |
| `assess_report_prompt.go` | `assessReportSystemPrompt`, `outputFormatTemplate`, `assessReportUserInput` — builds the strict-JSON prompt from the live rubric. |
| `assess_input.go` | `AssessmentInput` assembly (per-round prompts, event timeline). |
| `compose_mirror.go` | `ComposeMirror`, `MinimalMirror`, `mirrorSystemPrompt`, `sanitizeMirror` — 思维印记 mirror. |
| `project_summary.go` | `ComposeReturnSummary` — summary-on-return prose. |
| `reading_eval.go` | `EvaluateSelection(...)` — per-card reading-selection eval. |
| `reading_router.go`/`reading_gate.go`/`reading_takeaway.go` | Reading-loop grading/routing. |
| `review.go`, `framework_review.go`, `exploration_review.go`, `edge_proposer.go` | Reviewers (`ReviewFramework` is the framework-verdict seam). |
| `assess_report_test.go`, `reading_eval_test.go`, … | Tests. |

## A4. Resolver seams (`apps/api/internal/gateway/`)
- `keyresolver.go` — `type KeyResolver func(ctx) (Resolved, error)`; `NewKeyResolver` (chat/DeepSeek), `NewFastChaperoneResolver`, **`NewEvalKeyResolver` (flagship for evaluation, never flash)**.
- Wired in `api.go` as struct fields `ChatResolver`, `FastChatResolver`, **`EvalResolver`** (used by all assessment/mirror/summary/parent/review paths).
- Provider mux + pricing: `gateway/deepseek.go`, `anthropic.go`, `mux.go`, `pricing.go`, `types.go`.

## A5. sqlc queries + metering
Query sources (`apps/api/internal/store/queries/`) → generated Go (`apps/api/internal/store/sqlc/`):
- `evaluation.sql` — `InsertProjectEvaluation`, `GetLatestProjectEvaluation`, `Insert/GetLatestSessionEvaluation`, `Insert/GetLatestThreadEvaluation`, `ListGrowthHistory`, `ListEvaluationsByUser`.
- `workspace.sql` — `Get/UpsertProjectReflection`, `Get/InsertProjectMirror`, `Get/InsertProjectSummaryProse`, `Get/UpsertProjectAIUse`.
- `parent.sql` — `Get/InsertParentReportProse`.
- `teacher_weekly.sql` — class weekly prose (see Part B).
- Metering write helper: `RecordLLMCall` on `sqlcAgentStore` (`apps/api/internal/agent/agentstore.go`); handlers use `RecordLLMCall`/`RecordChatLLMCall`. Purpose strings: `"assessment"`, `"mirror"`, `"summary"`, `"parent_report"`, `"evaluation"`.

## A6. Trigger / async model
`finishProject` (`project_finish.go`) is the terminal trigger:
- Guards in order: ownership → entitlement → already-finished/evaluating (409) → reflection-done → essay-finished gate.
- Flips `project.status` `active` → `evaluating`, returns **202**, then a **detached `context.Background()` goroutine** composes the mirror and flips to `finished`; rolls back to `active` on failure.
- **This is an in-process detached goroutine, not a river/queue worker.** The `evaluations` table's `status`/`trigger`/`trigger_milestone` columns (migrations `0007`/`0008`) model queued/running/failed state and enforce the one-in-flight-per-task guard. (P4 in the refactor roadmap is where this becomes a real async worker.)
- `advance.go` (`advanceGates`) derives gate state, no model call.

## A7. Migrations (`apps/api/internal/store/migrations/`)
`0001_init.sql` (creates `evaluations`) · `0007_eval_async.sql` (status/queued/running) · `0008_eval_trigger.sql` (`trigger`, `trigger_milestone`, one-inflight unique) · `0019_llm_call_usage.sql` · `0021_assessment_project.sql`, `0024_session_assessment_scope.sql`, `0025_thread_assessment_scope.sql`, `0028_canonical_assessment_clean_slate.sql` (assessment scoping) · `0027_dualaxis_clean_slate.sql` (dual-axis reset) · `0032_student_evaluation_view.sql` · `0036_workspace_redesign.sql` (creates `project_reflection`, `project_mirror_prose`) · `0037_project_status_evaluating.sql` · `0038_continuous_session.sql` (creates `project_summary_prose`).

## A8. Contracts (`packages/contracts/src`)
`rubric.ts` (+`dualaxis.json`) · `dualAxisReport.ts` (`DualAxisReport` — the assessment payload) · `mirror.ts` (`MirrorSection`, `Mirror`) · `reflectionDoc.ts` (`ReflectionDoc = {answers, done}`) · `parentReport.ts` · `cardReflect.ts`.

## A9. Frontend (`apps/web/src`)
**API clients:** `api/assessment.ts` (`getAssessment`) · `api/chatAssessment.ts` (`getChatAssessment`, `generateChatAssessment`) · `workspace/api/workspace.ts` (`get/putReflection`, `get/postMirror`, `get/postProjectSummary`, `reflectProjectCard`).
**Views:** `shell/report/DualAxisReport.tsx` (the assessment report) · `shell/growth/GrowthReport.tsx` · `shell/chat/ChatReport.tsx` · `workspace/blocks/ReviewBlock.tsx` + `ReviewArtifacts.tsx` (reflection doc, self-score, mirror) · `workspace/blocks/ProsePane.tsx` (mirror/summary prose) · `studio/reading/FinalizeReadingPanel.tsx`, `HangingCard.tsx`, `ReadingRoom.tsx`, `readingLoop.ts` (reading-eval loop).

---

# Part B — Teacher / Admin End (教师端 / 组织端)

**Architecture facts:**
- Roles are `student`/`teacher`/`admin` on `users`, plus a separate `role_in_class` (`student`/`teacher`) on `enrollments`. Multi-tenancy is by `users.school_id`.
- The SPA is **role-routed**: `AppShell` picks `ConsoleShell` (teacher/admin) vs `StudentApp` from `session.getUser().role`.
- **Teacher/admin DTOs are NOT in `packages/contracts`** — class/enrollment/roster/teacher types are defined locally in the frontend `apps/web/src/api/*.ts`. Only the *report* payloads (`DualAxisReport`, `ParentReport`, `ParentStageReport`) are shared contracts. **Flag for the handoff:** this means backend↔frontend type drift is possible for class/roster shapes.

## B1. Role model & RBAC (`apps/api/internal/api/`)
- `authz.go` — `RequireRole(...)`; `assertAdminOfSchool`; `assertTeacherOwnsClass` (checks `enrollments.role_in_class == "teacher"`). Cross-tenant probes return not-found.
- `auth.go` — `User` context struct (ID/SchoolID/Role/DisplayName); `RequireUser` (401 gate) + the `protected` wrapper.
- `api.go` (≈ lines 228–257) — route table; `adminOnly = RequireUser(RequireRole("admin"))`, `teacherOrAdmin = RequireUser(RequireRole("teacher","admin"))`.

## B2. HTTP routes → handlers (`apps/api/internal/api/`)
**Admin-only:**
| Method + path | Handler (file) |
|---|---|
| `POST /api/v1/admin/teacher-invites` | `createTeacherInvite` (`invites.go`) |
| `GET /api/v1/admin/teacher-invites` | `listTeacherInvites` (`invites.go`) |
| `POST /api/v1/admin/import` | `adminImport` (`import.go`) — bulk roster CSV import |
| `GET /api/v1/admin/overview` | `adminOverview` (`overview.go`) — school aggregation dashboard |
| `GET /api/v1/admin/teachers` | `adminListTeachers` (`admin_teachers.go`) |
| `POST /api/v1/classes/{id}/teachers` | `assignClassTeacher` (`class_teachers.go`) |
| `DELETE /api/v1/classes/{id}/teachers/{userId}` | `removeClassTeacher` (`class_teachers.go`) |

**Teacher-or-admin:**
| Method + path | Handler (file) |
|---|---|
| `POST /api/v1/classes` | `createClass` (`classes.go`) |
| `GET /api/v1/classes` | `listClasses` (`classes.go`) |
| `GET /api/v1/classes/{id}` | `getClass` (`classes.go`) |
| `PATCH /api/v1/classes/{id}` | `patchClass` — rename + regenerate join code (`classes.go`) |
| `DELETE /api/v1/classes/{id}/enrollments/{userId}` | `removeEnrollment` (`classes.go`) |
| `GET /api/v1/classes/{id}/roster-report` | `getClassRosterReport` (`teacher_read.go`) |
| `GET /api/v1/classes/{id}/students/{userId}` | `getStudentDetail` (`teacher_read.go`) |
| `GET /api/v1/classes/{id}/students/{userId}/reports/{surface}/{scopeId}` | `getStudentReport` (`teacher_read.go`) |
| `GET /api/v1/classes/{id}/weekly-report` | `getClassWeeklyReport` (`teacher_weekly.go`) |
| `POST /api/v1/classes/{id}/weekly-report/prose` | `postClassWeeklyProse` (`teacher_weekly.go`) |
| `GET/POST .../parent-report/{surface}/{scopeId}[/prose]` | `getParentReport`/`postParentReportProse` (`parent_report.go`) |
| `GET/POST .../parent-stage-report/{weekStart}[/prose]` | `getParentStageReport`/`postParentStageProse` (`parent_stage_report.go`) |

**Signup / join-code entry path:** `signup.go` — `signup` dispatches on the code type: `signupTeacher` (consumes a teacher invite) vs `signupStudent` (class join code → creates the `enrollment`). **There is no separate "join class" route** — joining happens through signup.
**Code generation:** `apps/api/internal/org/codes.go` — `NewClassJoinCode()` (`XXXX-XXXX`), `NewTeacherInviteCode()`, `randString`.
**DTO helpers:** `classes_dto.go` — `toClassDTO`, `toRosterEntryDTO`.

## B3. Agent / LLM prose (teacher & parent-facing)
| Path | What it does |
|---|---|
| `apps/api/internal/agent/compose_weekly.go` | `ComposeWeekly`, `WeeklyProse`/`WeeklyFacts`/`WeeklyCardProse`, `weeklySystemPrompt`, `WeeklyFactsPrompt` — class weekly prose. |
| `apps/api/internal/agent/compose_parent.go` | `ComposeParent`/`ParentProse` — parent report aggregation. |
| `apps/api/internal/agent/compose_parent_stage.go` | `ComposeParentStage`/`ParentStageProse` + `validateParentStageProse`. |
| `apps/api/internal/teacher/weekly.go`, `week.go`, `badges.go` | **Deterministic** weekly fact/badge computation feeding the agent. |

> **Design constraint (not a stub):** the LLM only contributes *wording* — `ComposeWeekly`/`ComposeParent*` validate that the model fills prose fields over deterministically-computed facts. The numbers are computed in Go; the model only phrases them.

## B4. sqlc queries (`apps/api/internal/store/queries/` → `.../sqlc/`)
- `org.sql` — `GetClassByID`, `GetEnrollment`, `CreateTeacherInvite`, `ListActiveTeacherInvitesBySchool`, `GetActiveTeacherInviteByCode`, `ConsumeTeacherInvite`, `CreateClass`, `ListClassesForTeacher`, `ListClassesBySchool`, `GetUserByIDInSchool`, `GetClassRoster`, `UpdateClassName`, `SetClassJoinCode`, `DeleteEnrollment`, `GetClassBySchoolAndName`, `GetSchoolCounts`, `GetSchoolUsageByTier`, `ListTeachersBySchool`, `GetClassTeachers`, `AssignClassTeacher`, `DeleteClassTeacher`. (`CreateEnrollment` used by signup lives in `auth.sql`/`users.sql`.)
- `teacher.sql` — `ListClassRosterReport`, `GetStudentUsageForTeacher`, `ListStudentReportsForTeacher`, `ListStudentProjectsForTeacher`, `GetLatestReportScoresForStudent`, `GetStudentProjectEvaluationForTeacher`, `GetStudentThreadEvaluationForTeacher`, `GetStudentWeekStats`, `ListStudentEvaluationsForTeacher`.
- `teacher_weekly.sql` — `GetClassWeeklyProse`, `InsertClassWeeklyProse`, `AppendClassWeeklyProseCards`, `GetClassWeekStats`, `ListClassStudentWindowUsage`, `ListClassRecentReports`.
- `parent.sql` — `GetParentReportProse`, `InsertParentReportProse`.

## B5. Migrations (`apps/api/internal/store/migrations/`)
`0001_init.sql` (creates `schools`, `classes` w/ `join_code`, `users` w/ `role` CHECK + `school_id`, `enrollments` w/ `role_in_class`) · `0005_org.sql` (`teacher_invites` + `classes.created_by`) · `0032_student_evaluation_view.sql` (teacher-facing student eval view) · `0033_class_weekly_prose.sql` · `0035_parent_report_prose.sql`.

## B6. Seeding (Demo School / Demo Class / teacher & admin accounts)
`0002_seed.sql` (Demo School + Demo Class + join code + seeded users) · `0004_seed_password.sql` (seeded student dev password) · `0006_seed_admin.sql` (`admin@demo.mindimprint.local` / `admin-dev-pass`) · `0029_seed_teacher_class.sql` (吴老师's "IBDP 一年级·研究组", 9 students, enrollments, demo projects/evaluations for the teacher read-path) · `0030_seed_teacher_password.sql` (`wu.teacher@demo.mindimprint.local` / `phoebe-dev-pass`) · `0034_seed_teacher_week.sql` (weekly-report data).

## B7. Frontend (`apps/web/src`)
**Role routing:** `Root.tsx` (top-level; `?demo/?proto/?ds` dev surfaces else `AppShell`) · `shell/AppShell.tsx` (**role fork:** teacher/admin → `ConsoleShell`, else `StudentApp`) · `shell/session.ts` (session store w/ user + role).

**Teacher/Admin console (`apps/web/src/console/`):**
| File | What it is |
|---|---|
| `ConsoleShell.tsx` | Console router; tab state (admin default `overview`, teacher `classes`). |
| `ConsoleRail.tsx` | Left nav; `ConsoleTab = overview \| classes \| teachers \| import \| settings` (overview/teachers/import admin-only). |
| `OverviewView.tsx` | Admin school-level aggregation dashboard. |
| `ClassesView.tsx` | Class list + create class. |
| `ClassDetailView.tsx` | Single class: roster, join code, teachers. |
| `ClassRosterTable.tsx` | Roster / student-list table. |
| `ClassWeeklyView.tsx` | Class weekly report + prose generation. |
| `StudentDetailView.tsx` | Per-student drill-down. |
| `TeacherReportView.tsx` | Student report (dual-axis) rendered for the teacher. |
| `TeachersView.tsx` | Admin: list teachers, invites, assign to classes. |
| `ImportView.tsx` + `csv.ts` | Admin bulk roster import. |
| `ParentReport.tsx`, `ParentReportChrome.tsx`, `ParentStageReport.tsx`, `parentReportContent.ts` | Parent report rendering. |
| `EvidenceMap.tsx`, `badgeColor.ts`, `time.ts` | Supporting components/helpers. |

**Frontend API clients (`apps/web/src/api/`) — local type defs:**
- `classes.ts` — `listClasses`, `createClass`, `getClass`, `renameClass`, `regenerateJoinCode`, `removeEnrollment`; types `ClassSummary` (incl. `join_code`), `RosterStudent`, `Teacher`, `ClassDetail`.
- `teacher.ts` — `getClassRosterReport`, `getStudentDetail`, `getStudentReport`, `getClassWeeklyReport`, `generateClassWeeklyProse`, `getParentReport`, `generateParentReportProse`, `getParentStageReport`, `generateParentStageProse`; types `RosterReportEntry`, `StudentRecord`, `StudentDetail`, `TeacherReport`, `WeeklyCard`, `WeeklyReport` (imports the shared report contracts).
- `admin.ts` — `getOverview`, `listTeacherInvites`, `createTeacherInvite`, `adminImport`, `listTeachers`, `assignTeacher`, `removeTeacher`.
- `index.ts` — assembles the `ApiClient` surface used by `ConsoleShell`.

## B8. Build-completeness (as of 2026-08-11)
- **Fully built, end-to-end wired:** role RBAC + guards; class CRUD + join-code regen; roster/enrollment management; teacher-invite issuance & teacher self-signup; student join via class code; admin overview / teacher assignment / CSV import; class weekly report + LLM prose; parent report + parent stage report + prose; full console UI for all of it — all backed by sqlc queries, migrations, and seeded demo data (admin + 吴老师 teacher + 9-student class).
- **Not a stub, a deliberate constraint:** the LLM only phrases deterministically-computed facts (validators in `compose_weekly.go`, `compose_parent_stage.go`).
- **Gap to flag:** teacher/class/roster/enrollment DTOs are duplicated as local interfaces in `apps/web/src/api/classes.ts` + `teacher.ts` rather than shared via `packages/contracts` — only report payloads are contract-shared, so watch for type drift on class/roster shapes.
- No dedicated `school*.go`/`org*.go`/`roster*.go`/`enrollment*.go` handler files exist — that functionality is folded into `classes.go`, `overview.go`, `import.go`, `teacher_read.go`, and `internal/org/codes.go`.

---

## Quick start for the two takeovers

**Evaluation:** start at `rubric/dualaxis.json` (the model), then `agent/assess_report.go` + `assess_report_prompt.go` (how a report is generated), then `api/assessment.go` + `project_finish.go` (how it's triggered and served), then `shell/report/DualAxisReport.tsx` (how it's shown). The read-no-call/write-only-spend and flagship-EvalResolver invariants are the two things not to break.

**Teacher end:** start at `api/api.go` (the route table + `adminOnly`/`teacherOrAdmin` wrappers) and `api/authz.go` (the guards), then `console/ConsoleShell.tsx` (the UI router) and `api/classes.ts`/`teacher.ts`/`admin.ts` (the client surface). The LLM-phrases-facts-only constraint and the contracts-drift gap are the two things to keep in mind.
