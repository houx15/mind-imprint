# A3 — Project Terminal + 成长报告 History Hub · Design

> Third sub-project of the cross-surface assessment track (A1 course → A2 chat →
> **A3 project + history** → B DualAxis → C ability model). A1 (`8509940`) and A2
> (`44167a9`) are merged. This spec does **not** modify A1 or A2 behaviour.

## Goal

Give the **project** the terminal it never had — a student-initiated finish that
**automatically generates** the project's growth report — and turn **成长报告**
(the progress center) into the one place where every past report across all three
surfaces is read back.

## The unifying principle (user requirement)

**A growth report is visible only after a terminal act on its surface, and that
act auto-generates it — once.** A report is a one-time artifact: no regeneration,
on any surface. Chat is the sole exception to *auto*-generation: it never ends, so
its report is a manual opt-in click (still one-time).

| Surface | Terminal | Report generation | State before A3 |
|---|---|---|---|
| **Project** | new 完成任务·归档 button (gated on 整稿体检) | **automatic on finish** | ❌ on-demand via an always-on button — A3 fixes |
| **Course** | 完成课程 (advance past 回看 → `status='finished'`) | **automatic** at the finish-reached report view (DEC-A1.4) | ✅ already terminal-gated — unchanged |
| **Chat** | manual "生成本次对话的思维印记" (opt-in) | on the click | ✅ already opt-in — unchanged |

Course and chat already satisfy the principle (verified: `CourseReport` is routed
**only** via `onFinish`, which fires on `status:"finished"`; chat's button is
opt-in and never auto-POSTs). **The project is the only surface needing a terminal.**

## Architecture

Two additive server surfaces plus one migration; the project's studio projection
gains two booleans; 成长报告 is rebuilt as a history hub. No change to the agent,
the enforcement stack, A1, or A2.

- **Project terminal:** `POST /api/v1/projects/{id}/finish` — one atomic act that
  finishes the project *and* mints its terminal report. `finished ⟺ has a
  terminal report` holds for every project created after A3.
- **History hub:** `GET /api/v1/growth/history` — one owner-filtered union query
  returning every report (project + course session + chat thread) with its full
  report DTO embedded. 成长报告 renders it as an expandable list.

---

## Decisions

### DEC-A3.1 — Project terminal is a minimal finish button, not the S6 station

A **完成任务·归档** button in the studio, visible only when the S5 gate item
`draft_polish.whole_draft_review == "solid"` **and** the project is not already
finished. We do **not** build the S6 反思归档 station (self-score bands / retro
prompts / AI-usage declaration, dc.html 2168–2181) — that stays its own deferred
slice. The dead `onFinishTask` stub (dc.html 2739) is replaced by a live handler.

### DEC-A3.2 — `POST /api/v1/projects/{id}/finish` (auto-generates the report)

In order:
1. `loadOwnedProject` (404-hides non-ownership) + `HasEntitlement` (a flagship
   call is about to be spent — same seam A1/A2 use).
2. **Gate guard (server-enforced):** read `ListGateStates(projectID)`; require
   `draft_polish.whole_draft_review == "solid"`. Not solid → `422`
   `gate_not_met` ("还没通过整稿体检，先完成整稿体检再归档"). Never trusts the client.
3. **One-time guard:** already `finished` → `409` `already_finished`
   ("任务已归档"). The report is minted once; there is no regeneration path
   anywhere (DEC-A3.5).
4. **Generate (automatic):** the shared helper `generateProjectReport` (DEC-A3.5,
   extracted from today's generation body) runs the flagship assessor —
   `studio.Load` → `buildAssessmentInputFromProject` → `agent.Assess` on
   `a.d.EvalResolver` (never downgraded) → **record cost via `RecordLLMCall` even
   if the output is then rejected**.
5. **On rejection** (`agent.Assess` error) → `422` `assessment_rejected`; the
   project is **not** finished (retryable). Cost already recorded.
6. **On success** (single implicit transaction ordering): `InsertProjectEvaluation`
   → `SetProjectFinished` (new query, `status='finished'`) → append a
   `project_finished` event (`surface:"studio"`, project-scoped, process data) →
   return the `AssessmentDTO`. If any of these fail, surface the error; do not
   claim a finish that did not persist.

### DEC-A3.3 — Migration 0026: close the `project.status` enum

`project.status` is `text NOT NULL DEFAULT 'active'` with **no CHECK** (0016) and
is written by nothing today. Add `CHECK (status IN ('active','finished'))`. Every
existing row is `'active'`, so a validated CHECK adds cleanly (no `NOT VALID`
needed). Down drops the CHECK. This is the same closed-enum shape `course_session`
already carries (0023).

### DEC-A3.4 — Studio projection gains `Finished` + `CanFinish`

`studio.Project` (or `projects.go`'s DTO) exposes:
- `finished bool` — from `project.status == "finished"`.
- `canFinish bool` — `whole_draft_review` solid **and** not finished.

Both derive from facts the projection already computes (the gate states it reads
for the S5 station; the project row it loads). The studio renders 完成任务·归档
when `canFinish`; when `finished`, it renders an inert "已归档" state instead.

### DEC-A3.5 — One-time generation: the finish endpoint is the sole path; no regenerate

The report is a one-time artifact. The finish endpoint (DEC-A3.2) is the **only**
project-report generation path; there is no 重新生成 on any surface.

- The generation body currently in `generateAssessment` is extracted into a
  shared helper `generateProjectReport(ctx, projectID, userID) → (AssessmentDTO,
  error)` and called **only** by finish.
- The standalone **`POST /api/v1/projects/{id}/assessment` regenerate route is
  removed**, along with its web client method and its UI affordance. Its
  generation-core assertions move into the finish endpoint's tests.
- **`GET /api/v1/projects/{id}/assessment` is retained** (a benign read; may be
  used by e2e). The new 成长报告 does not depend on it — it reads the history
  endpoint (DEC-A3.7) — so removing it is out of scope here, but nothing new
  relies on it either.

### DEC-A3.6 — A1 and A2 are not modified

Course auto-generates its report at the finish-reached report view (DEC-A1.4);
chat is opt-in. Both already satisfy the unifying principle. A3 touches neither
`course_assessment.go`/`CourseReport.tsx` nor `chat_assessment.go`/`ChatReport.tsx`.

### DEC-A3.7 — 成长报告 becomes a pure cross-surface history hub

Report generation is now a terminal act on each surface, so 成长报告 no longer
generates anything on its own. It is rebuilt as:
- The **history list** from `GET /growth/history` — every report the student has
  (project + course + chat), newest first. Each row: a surface chip
  (项目 / 课程 / 聊天) + label + 日期. **No row-level score, level, or rank**
  (RL-5). Click expands to the embedded report, rendered by the same
  `DimensionRow` + narrative view already used in-surface.
- **Every row is read-only** — reports are one-time terminal artifacts (DEC-A3.5),
  so there is no 重新生成 on any surface.
- Empty state (no reports at all): "完成一个任务、一节课，或在聊天里生成一次思维印记，报告会在这里汇集。"
- The old always-on "生成成长报告" button and the `list[0]`-project pattern are
  **removed**.

### DEC-A3.8 — `ListGrowthHistory`: one owner-filtered union query

`UNION ALL` of three subqueries, each `DISTINCT ON (scope)` newest-per-scope,
owner-filtered through its scope's join, then outer `ORDER BY created_at DESC`:
- project evals → `JOIN project` on `project_id` (`WHERE project.user_id = @user`,
  label `project.title`, no sublabel).
- session evals → `JOIN course_session` on `session_id`
  (`WHERE course_session.user_id = @user`) `JOIN course` (label `course.title`,
  sublabel = the session `phase`).
- thread evals → `JOIN chat_thread` on `thread_id`
  (`WHERE chat_thread.user_id = @user`, label `chat_thread.title`, no sublabel).

Each returned row carries `surface`, `scope_id`, `label`, `sublabel`,
`created_at`, and the full `scores` + `narrative` (the report itself). Because the
list is already owner-filtered, embedding the full report needs no second fetch
and no separate per-report ownership check.

### DEC-A3.9 — Contracts: `GrowthHistory` DTO

A Zod DTO in `packages/contracts`:
`{ entries: Array<{ surface: "project"|"course"|"chat", scopeId: string, label:
string, sublabel: string|null, createdAt: string, report: Assessment }> }`,
reusing the existing `Assessment` schema for the embedded report. Web parses the
response through it.

---

## Data flow

**Finish (project):**
```
studio 完成任务·归档 (shown only when canFinish)
  → POST /projects/{id}/finish
    → gate guard (422 if not solid) · idempotency (409 if finished)
    → generateProjectReport  [flagship, cost recorded even on reject]
        reject → 422, project stays active (retryable)
        ok → InsertProjectEvaluation + SetProjectFinished + project_finished event
    → returns AssessmentDTO
  → web routes to 成长报告 (the fresh report is now the newest history row)
```

**Read back (any surface):**
```
成长报告 mounts → GET /growth/history → [{surface,label,date,report}, …] newest-first
  → render collapsed rows (surface chip + label + date; no score)
  → click row → expand embedded report (DimensionRow + narrative) — read-only
```

---

## Testing

**Go (full packages, never `-run` subsets — card/gate/projection/migration changes):**
- Finish endpoint: gate-not-solid → 422; already-finished → 409 (one-time — no
  second generation); happy path sets `status='finished'`, persists a project
  evaluation, appends `project_finished`, returns the DTO; assessor rejection →
  422 **and** project stays `active` **and** cost is recorded; the persisted
  evaluation's tier is `flagship`.
- `ListGrowthHistory`: returns project + course + chat rows for the owner;
  excludes another user's reports; newest-first; one row per scope (reports are
  one-time, so this is defensive — if two rows ever share a scope, the latest
  wins); labels/sublabels correct per surface.
- Migration 0026 up/down: CHECK present after up (rejects `status='bogus'`),
  absent after down; existing `active` rows survive up.
- Projection: `canFinish` true iff `whole_draft_review` solid and not finished;
  `finished` tracks `status`.

**Web / contracts (from their own dir):**
- `GrowthHistory` Zod parse of a representative payload.
- 成长报告 renders the history list, expands a row to its (read-only) report,
  shows the empty state with no entries, and no longer shows any generate or
  重新生成 button.
- Studio shows 完成任务·归档 only when `canFinish`, "已归档" when `finished`, and
  POSTs finish then routes to 成长报告.

---

## Out of scope (deferred)

- The full **S6 反思归档** station (self-score / retro / AI-usage declaration).
- Any change to A1 (course) or A2 (chat) behaviour.
- The **C** ability model — the 能力素养 9-dimension radar that *aggregates*
  levels across reports. A3 delivers the history *list*; C builds the aggregation
  on top of the rows A1/A2/A3 now produce.
- Deep-linking a history row back into its origin surface — reports render inline
  in 成长报告 from the embedded DTO (identical content).

## Invariants carried through

- **RL-5:** no total/aggregate/rank anywhere; history rows are date-ordered and
  score-free; embedded reports stay per-dimension diagnostic (label + evidence).
- **铁律 2 (不操纵):** finishing is a student click; auto-generation happens *at*
  that student-initiated terminal, never as a nudge, streak, or background job.
  History is read-only.
- **旗舰绝不降级:** the terminal report uses `EvalResolver`; cost recorded even on
  rejection ("记录档位 + token + 成本" holds on the reject path).
- **Secrets** stay server-side; nothing new is logged, rendered, or persisted that
  carries a key.
