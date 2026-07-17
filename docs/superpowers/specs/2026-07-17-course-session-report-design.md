# A1 · Course Session Report — Design

**Goal:** Make the course's 学习报告 real — a session-scoped growth assessment
generated from the course's own evidence — by giving the `event` stream and
`evaluations` a **session scope**, so that chat/course evidence stops being
unreachable by construction.

**Status:** design approved 2026-07-17. Sub-project A1 of the cross-surface
assessment work (A1 course → A2 chat → A3 project terminal; then B DualAxis
model, C student-level ability model).

---

## 1. Why · what is actually broken

Slices 11 and 12 write chat and course evidence into the `event` stream. Nothing
can read it. This is not a missing query — it is **structural**:

```sql
-- 0016_refactor2_foundations.sql:139-149
CREATE TABLE event (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id uuid REFERENCES project(id) ON DELETE CASCADE,   -- nullable
    user_id    uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    surface    text NOT NULL CHECK (surface IN ('studio','course','chat')),
    type       text NOT NULL,
    payload    jsonb NOT NULL DEFAULT '{}',
    created_at timestamptz NOT NULL DEFAULT now()
);
```

`event` has exactly one scope column, `project_id`. Every course and chat event
is written through one method whose signature is the bug:

```go
// apps/api/internal/agent/coursestore.go:198-203  (chatstore.go:101-106 is identical)
func (s *sqlcCourseStore) InsertUserEvent(ctx context.Context, userID uuid.UUID, surface, typ string, payload []byte) error {
	_, err := s.q.AppendEvent(ctx, sqlc.AppendEventParams{
		ProjectID: pgtype.UUID{Valid: false}, UserID: userID,
		Surface: surface, Type: typ, Payload: payload,
	})
	return err
}
```

It takes no scope and hard-codes `project_id = NULL`. The **only** SELECT ever
written against `event`, anywhere in the codebase, is:

```sql
-- apps/api/internal/store/queries/event.sql
-- name: ListEventsByProject :many
SELECT * FROM event WHERE project_id = $1 ORDER BY created_at, id;
```

`WHERE project_id = $1` can never match a NULL row. So course evidence —
`course_message` with `{"unprompted":true}`, `card_surfaced`, `phase_advanced`,
`course_finished` — is written, stored, and permanently invisible.

Meanwhile `apps/web/src/shell/courses/CourseReport.tsx` already renders a
学习报告, but it has no assessment in it at all, and one of its four stat tiles
is false (§2.5).

---

## 2. Decisions

### DEC-A1.1 — `event` and `evaluations` gain a session scope; `event`'s CHECK is `NOT VALID`

Migration 0024 adds nullable `session_id` to `event` and `evaluations`,
following the established pattern exactly (0021 evaluations → 0022 thread →
0023 session): `<table>_scope_ck` naming, `num_nonnulls(...) >= 1`,
`<table>_<scope>_created_idx` indexes.

`event` is the one place the pattern must bend. `event.user_id` is `NOT NULL`,
so **ownership is never at risk** — the scope columns carry *attribution*, not
ownership. And the chat/course rows already in the database have no attribution
and none can be invented: `InsertUserEvent` never recorded a thread or session,
so there is nothing to backfill *from*. A strict CHECK would reject them; a
backfill would fabricate; a delete would destroy real records.

Therefore `event`'s scope CHECK is added **`NOT VALID`**: new rows are enforced,
existing unattributable rows are grandfathered and honestly labelled as such in
the migration comment. This is the only honest option of the three.

`evaluations` takes an ordinary (validated) CHECK — every existing row has a
`task_id` or `project_id` and satisfies it unchanged.

### DEC-A1.2 — `InsertUserEvent` is replaced, not supplemented

The hole reopens the moment a future writer forgets a scope. `CourseStore`'s
`InsertUserEvent(ctx, userID, surface, typ, payload)` is **removed** and replaced
with `InsertSessionEvent(ctx, sessionID, typ, payload)`, which resolves `user_id`
from the session row and always sets `session_id`. Surface is no longer a
parameter — a `CourseStore` writes `"course"` events and nothing else.

`ChatStore.InsertUserEvent` is out of scope and stays as-is; A2 gives chat the
same treatment. A1 must not touch it.

### DEC-A1.3 — the rubric and the assessor are not touched

Sub-project B replaces the assessment model wholesale (the DualAxis reference,
`docs/EVALUATION-0717-Student-A-AI-Interaction-Report-v3-DualAxis.html`). The
dimension count is currently three-way inconsistent — the binding design says
**9** (`九个维度，跨五大分支`, `思维印记_工作区.dc.html:1565`), `ct-rubric.json` has
**10**, the DualAxis reference has **6**. A1 resolves none of this and must
bake in none of it.

`agent.Assess`, `agent.AssessmentInput`, `rubric.CT()`, `ct-rubric.json`, the
assess system prompt, `AssessmentDTO`, and the `Assessment` Zod contract are all
**unchanged**. A1 moves scope, not meaning.

### DEC-A1.4 — "auto" = generate on first open, never inside the SSE turn

The trigger is auto (no button), but the course terminal in
`agent/course_step.go:377-396` runs inside an SSE turn. Issuing a flagship
assessment call there would stall the student's last 回看 turn behind a slow
model call, and a disconnect would lose it.

Instead the terminal is **unchanged**, and the report view generates on first
open: `GET` returns `null` → the client `POST`s once. To the student it is
automatic; nothing blocks a turn. This mirrors the project's existing
GET-null-then-act shape (`assessment.go:29-54`), differing only in that the act
is automatic rather than a button.

### DEC-A1.5 — 挑战通过 means engagement, and no AI ever judges a course answer

Today the tile is false:

```tsx
// CourseReport.tsx:33,55
const challenges = course.steps.filter((s) => s.kind === "challenge");
<Stat value={`${challenges.length}`} label="挑战通过" color="#D98263" />
```

It counts the challenges the course *contains*, so every student is told they
passed every challenge — including ones they never saw — and 挑战回顾 puts an
unconditional green ✓ on each (`CourseReport.tsx:82`). There is no pass record
anywhere: `course_step` has no verdict column, and `course_progress.completed_ordinals`
records *viewed*, not passed.

**通过 is defined as engagement: a challenge counts when the student actually
reached it** (`ordinal ∈ course_progress.completed_ordinals`). No model call, no
new table, no judgment of the student. This is deliberate and it is the stronger
product position: the course teaches, and assessment happens in the report from
evidence — the coach never grades a course answer (铁律 1, and RL-5's «never a
grade or verdict»). The signal is trustworthy because Slice 12's whole-branch
fix moved view-recording server-side into the render handler, so
`completed_ordinals` is not client-assertable.

The green ✓ in 挑战回顾 becomes conditional on the same rule.

**Rejected:** a coach-judged `challenge_verdict` typed output. RL-5 reads «the
assessment is diagnostic evidence, never a grade **or verdict**»
(`2026-07-16-slice-10-assessor-growth-report-design.md:63`), and 通过/不通过 is
both. Every judgment in this product lands on the work (`solid`, `已扎实`,
`machine_clear`), never on the student — the DualAxis reference states the same
law: «P 档贴在提示词上，不贴在学生身上».

### DEC-A1.6 — two designed blocks become real; two stay out

`CourseReport.tsx` omits four blocks the binding design specifies. A1 fills the
two that real data now supports and leaves the two that have no feature behind
them. **Nothing is fabricated to fill a designed block.**

| Block | A1 | Why |
|---|---|---|
| `能力评估` (`dc.html:466-480`) | **Add** | Exactly what the assessor produces |
| `收集到的工具` (`dc.html:482-494`) | **Add** | Real since 0023 gave `card_instances` a `session_id` |
| `我的学习笔记` (`crNote`, `dc.html:496-503`) | **Omit** | No course note exists anywhere in the repo |
| `导出笔记` (`dc.html:512-524`) | **Omit** | `导出` has zero hits in the codebase; no export feature exists |

The two omissions are recorded as carry-forwards (§9), continuing the honest
omission the existing component already practises.

---

## 3. Schema — migration 0024

```sql
-- +goose Up
-- A1: give the event stream and evaluations a course-session scope, so course
-- evidence stops being unreachable. Additive, mirroring 0023's session scope
-- over material/card_instances.
ALTER TABLE event       ADD COLUMN session_id uuid REFERENCES course_session(id) ON DELETE CASCADE;
ALTER TABLE evaluations ADD COLUMN session_id uuid REFERENCES course_session(id) ON DELETE CASCADE;

CREATE INDEX event_session_created_idx       ON event (session_id, created_at);
CREATE INDEX evaluations_session_idx         ON evaluations (session_id, created_at DESC);

-- evaluations: ordinary widening — every existing row has task_id or
-- project_id and satisfies this unchanged.
ALTER TABLE evaluations DROP CONSTRAINT evaluations_scope_ck;
ALTER TABLE evaluations ADD CONSTRAINT evaluations_scope_ck
  CHECK (num_nonnulls(task_id, project_id, session_id) >= 1);

-- event: NOT VALID, deliberately. event.user_id is NOT NULL, so ownership is
-- never at risk here — these columns carry attribution, not ownership. Every
-- chat/course event already stored was written by InsertUserEvent, which
-- recorded no scope at all (project_id NULL, no thread/session column existed):
-- those rows are permanently unattributable and there is nothing to backfill
-- FROM. NOT VALID enforces every new row while grandfathering the old ones,
-- rather than deleting real records or inventing a scope they never had.
-- A2 widens this to include thread_id.
ALTER TABLE event ADD CONSTRAINT event_scope_ck
  CHECK (num_nonnulls(project_id, session_id) >= 1) NOT VALID;

-- +goose Down
ALTER TABLE event       DROP CONSTRAINT IF EXISTS event_scope_ck;
ALTER TABLE evaluations DROP CONSTRAINT IF EXISTS evaluations_scope_ck;
ALTER TABLE evaluations ADD CONSTRAINT evaluations_scope_ck
  CHECK (task_id IS NOT NULL OR project_id IS NOT NULL);
DROP INDEX IF EXISTS event_session_created_idx;
DROP INDEX IF EXISTS evaluations_session_idx;
ALTER TABLE event       DROP COLUMN IF EXISTS session_id;
ALTER TABLE evaluations DROP COLUMN IF EXISTS session_id;
```

---

## 4. Evidence and the assessor

**Writes.** `CourseStore.InsertUserEvent` → `InsertSessionEvent(ctx, sessionID, typ, payload)`.
All five course event sites carry the session: `card_surfaced`
(`course_step.go:284`), `course_message` (`:294`), `course_finished` (`:393`),
`phase_advanced` (`:419`), and `card_completed` (`api/course_session.go:331`).

**Read.** One new query:

```sql
-- name: ListEventsBySession :many
SELECT * FROM event WHERE session_id = $1 ORDER BY created_at, id;
```

**Projection.** A new `buildAssessmentInputFromSession` mirroring
`buildAssessmentInputFromProject` (`api/assessment.go:170`), feeding the
unchanged `agent.BuildAssessmentInput`:

- `Timeline` — from `ListEventsBySession` via the existing `eventText`/`EventDigest` shape.
- `CardUses` — from the session's `card_instances` (`ListCardInstancesBySession`).
- `Dispositions` — session cards' completed/skipped status.
- `GateProgress`, `SnapshotCount`, `WordCounts`, `ReviewBands`, `GraphSummary` —
  **empty**. A course session has no gates, snapshots, or graph. `Assess` already
  defaults unevidenced dimensions to `NA`; a course report will legitimately show
  `NA` on the output dimensions, and that is the honest answer, not a bug.

---

## 5. Endpoints

Two routes, following the existing course convention
(`api.go:71-81`, `/api/v1/courses/{id}/session/...`) and resolving through
`loadOwnedSession` (`course_session.go:41`), which looks the session up by
`(caller, course_id)` — **IDOR closed by construction**, no session id in the URL:

```go
mux.Handle("GET /api/v1/courses/{id}/session/assessment",  protected(a.getCourseAssessment))
mux.Handle("POST /api/v1/courses/{id}/session/assessment", protected(a.generateCourseAssessment))
```

- `GET` — returns the session's latest evaluation, or JSON `null` when none.
  **Never a model call**, never a 404 (unassessed is a normal state).
- `POST` — `HasEntitlement` gate → `buildAssessmentInputFromSession` → one
  flagship `agent.Assess` (never downgraded) → record the call's cost
  **regardless of outcome** → `InsertSessionEvaluation` → return the DTO.

New queries: `InsertSessionEvaluation`, `GetLatestSessionEvaluation` — mirroring
`InsertProjectEvaluation` / `GetLatestProjectEvaluation`.

The reused `AssessmentDTO` / `Assessment` Zod contract is unchanged (DEC-A1.3).

---

## 6. UI

`CourseReport.tsx` gains the two blocks from §2.6, built from the binding markup
in `docs/design/思维印记_工作区.dc.html` **verbatim** (colours, spacing, copy,
inline SVG — never lucide-react):

- **`能力评估`** (`dc.html:466-480`) — heading `能力评估`, subtitle
  `按 SOLO 四级 · 来自这门课里你的表现`, then one row per dimension the assessor
  returned: `d.dim` ← `DimensionScore.name` (the human name, e.g. `信源辨识`;
  `code` is not rendered — the design shows one label per row), an `L#` chip
  (`#D98263` on `#FBEEE7`), a 4-segment level bar, and `d.note` ←
  `DimensionScore.evidence` (RL-5: every level carries its behavioral
  evidence). No total, no rank, no aggregate.

  **`NA` rendering.** `DimensionScore.level` is `L1|L2|L3|L4|NA`, and a course
  report will legitimately return `NA` for the output dimensions it has no
  evidence for (§4). The design has no `NA` state, so A1 defines one: the chip
  reads `未涉及` in the muted palette (`#9AA1B0` on `#F3F4F7`, not the `#D98263`
  level chip), **zero** of the four segments are lit, and the note carries the
  assessor's evidence text explaining the absence. An `NA` row is rendered, not
  hidden — "this course produced no evidence here" is a true and useful thing to
  show, and silently dropping the row would overstate what the session covered.
- **`收集到的工具`** (`dc.html:482-494`) — one pill per completed session card,
  named from `CARD_REGISTRY`.

On mount: `GET` → if `null`, `POST` once (DEC-A1.4), showing the existing
`正在整理你的学习报告…` idiom while it runs. A failed generation shows an honest
error, never a fabricated report.

Also fixed, per DEC-A1.5: the 挑战通过 tile counts reached challenges, and the
挑战回顾 ✓ becomes conditional.

`我的学习笔记`, `导出笔记`, and the existing action buttons' behaviour are
untouched.

---

## 7. Cost and entitlement

`HasEntitlement(ctx, user)` gates the POST before any token is spent. The call is
metered via `CourseStore.RecordCourseLLMCall`, which is `userID`-keyed and
project-free — note the project-side `RecordLLMCall` **cannot** be reused: it
resolves the owning user via `GetProject(row.ProjectID)` (`agentstore.go:526-556`)
and a course session has no project.

`RecordCourseLLMCall` currently hard-codes `Purpose: "coach"`
(`coursestore.go:206-218`); it gains a `purpose` parameter so the assessment call
records `surface="course", purpose="assessment"` — matching the project side's
existing `"studio"/"assessment"` pair (`assessment.go:102`). Cost is recorded on
every post-`Collect` return, **including rejection**, per the established rule.

---

## 8. Red lines

- **RL-5** — diagnostic evidence, never a grade or verdict. No total, no rank, no
  aggregate field; every level carries its evidence. 挑战通过 counts engagement,
  and no model judges a student's course answer (DEC-A1.5).
- **铁律 2 (不操纵)** — no streaks, no leaderboards. The report is generated
  because the student finished the course, and disclosure already exists
  (`计入成长评估`).
- **Client never calls a model; keys server-side only.** The assessor runs
  behind the API, entitlement-gated and metered.
- **Assessor isolation** — the assessor is never in the coach turn-loop
  (DEC-A1.4 keeps it out of the SSE turn); its own endpoints, its own cost rows.
- **No fabrication** — a designed block with no real data is omitted, not filled
  (DEC-A1.6).

---

## 9. Non-goals and carry-forwards

**Not in A1:** chat reports and `event.thread_id` (A2, which widens both CHECKs);
the project terminal / finish button and 成长报告's history entrance (A3); the
DualAxis model and the 9-vs-10-vs-6 dimension question (B); any cross-session
aggregation or student-level model (C).

**Carry-forwards created or restated:**
- `我的学习笔记` + `导出笔记` — designed, no feature behind them anywhere.
- `event`'s pre-A1 chat/course rows stay permanently unattributable; the
  `event_scope_ck` constraint stays `NOT VALID` forever unless those rows are
  someday deleted.
- `ChatStore.InsertUserEvent` keeps the unscoped signature until A2.
- `HasEntitlement` has no injection seam, so no entitlement path anywhere in
  the product is tested. Billing/entitlement work must add one.
- Challenges still have no notion of quality — only of engagement.
- `EVENT_TYPES` / `StudioEvent` in `packages/contracts/src/event.ts` remain dead
  code that matches neither the DB row shape nor any type string actually
  written (pre-existing; A1 does not touch it).

---

## 10. Tests

**Go**
- Migration 0024 Up **and Down** (no test runs Down anywhere in this repo today —
  A1 does, for its own migration).
- `event_scope_ck` rejects a scopeless new row and accepts a session-scoped one;
  a pre-existing unattributable row still selects fine (the NOT VALID contract).
- `InsertSessionEvent` sets `session_id` and resolves `user_id` from the session.
- `ListEventsBySession` returns only that session's events, ordered.
- `buildAssessmentInputFromSession`: timeline from real course events; empty
  gates/snapshots/graph → `NA` dimensions, not a crash.
- `GET` unassessed → `null`, **zero `llm_call` rows** (asserted, per Slice 10's
  precedent).
- `POST` → one `llm_call` row with `surface="course", purpose="assessment"`;
  cost recorded on rejection too.
- Ownership: another user's course 404s and leaks nothing.

**Not tested, deliberately:** `HasEntitlement` false → no model call. The seam
does not exist — `HasEntitlement` is a package-level `return true, nil`
(`api/entitlement.go:9`) with no injection point, so the false branch is
unreachable from a test. Building one (a `Deps.Entitled` field, threaded
through every existing call site) is a cross-cutting refactor that belongs with
the billing/entitlement work, not with assessment plumbing. The gate is verified
by reading the handler instead: it precedes every model call. Recorded as a
carry-forward in §9.

**Web**
- `CourseReport` renders the assessor's dimensions with evidence; no total/rank
  in the DOM (RL-5 asserted in the UI, per Slice 10's precedent).
- An `NA` dimension renders as a `未涉及` row with zero lit segments — present,
  not hidden, not shown as `L0` or as a lit level.
- `GET` null → exactly one `POST`, not a loop.
- 挑战通过 counts only reached challenges; an unreached challenge has no ✓.
- Generation failure → honest error, no fabricated report.

**Mock fidelity (the Slice 12 lesson).** Five of Slice 12's six Criticals came
from mocks encoding shapes the backend cannot produce. Every mock here must be
checked against the real contract: SSE frames, the DTO's camelCase, and
`completed_ordinals`' server-side origin. Card/gate/projection changes run FULL
Go packages — never `-run` subsets.
