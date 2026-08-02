# Course Part v2 — Reference Show-Logic + Admin Upload · Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the phase-gated course runtime with a linear, self-paced player driven by an externally-authored course structure JSON + published render cache; ship `a-mid` + `b-mid`; add an admin-key course-upload endpoint; give each course a 0..many attached tool-cards list; a simple no-LLM report; a free-Q&A AI bar.

**Architecture:** Course content is authored elsewhere (class-agent repo) and stored verbatim as `structure` + `render_cache` jsonb on the `course` row (border-validated). The player reads display content from the render cache and resolves image assets from the structure's `asset_library` via the existing `/oss/resolve-url`. The AI bar is a stateless-ish free-Q&A coach. Progress + quiz attempts + ask turns are recorded as course-scoped events (铁律④).

**Tech Stack:** React + Vite + TS + Tailwind (web); Go `net/http` + `pgx` + hand-edited sqlc + goose (api); Zod contracts (shared); PostgreSQL.

**Design spec:** `docs/superpowers/specs/2026-08-02-course-v2-reference-showlogic-design.md`
**Reference (read-only):** `docs/reference/class-agent/` — `data/courses/{a,b}-mid.json`, `data/published/{a-mid-v4,b-mid-v3}-render-cache.json`, `src/main.jsx`.

## Global Constraints

- **No back-compat** with the old course model. Retire the phase runtime (session/advance/render/assessment/card-offer) entirely — code, routes, tables, tests, client.
- **sqlc is NOT run** — hand-edit both the `.sql` file and the generated `.sql.go`. New columns go LAST in the table; a `SELECT`'s column order MUST equal the `rows.Scan(...)` order. (If regen is ever unavoidable, pin `sqlc@v1.27.0` first.)
- **Border validation only** (信封 principle): Go validates the outer envelope (structure: `id`,`title`,`steps[]`; render cache: `version`,`steps[]` each with `stepId`+`content`). Inner content stored verbatim; contracts own the inner shape.
- **Keep `course.id uuid` PK**; add `slug text UNIQUE`. `event.course_id` and `course_progress.course_id` stay `uuid`. The API path uses the slug; handlers resolve slug → uuid internally.
- **Secrets server-only.** `OSS_ADMIN_KEY` gates the upload; never log/echo it. Reuse `ossBearer(r)` + `a.d.OSSAdminKey` from `apps/api/internal/api/oss.go`.
- **铁律:** ① AI never gives quiz answers or writes conclusions; ② quizzes never gate advancement, paging is always free; ③ one question at a time; ④ every quiz attempt (incl. wrong/skip) + every ask turn logged as an event.
- Card ids MUST be a subset of the 34-card registry (`packages/contracts/cards/*.json`). Attached sets: `a-mid → ["craap"]`; `b-mid → ["sift","craap","argument-map","concession","metacognition"]`.
- **Never `git add -A`** — stage explicitly (`git add -u` + named new files). Use `$CLAUDE_JOB_DIR/tmp` for scratch. The project-boundary hook BLOCKS `/dev/null` redirects — avoid them.
- Interaction types the player MUST support: `single_choice`, `multiple_choice`, `ordering`. Asset types: `image`, `link`, `text`.

## File Structure

**Create:**
- `apps/api/internal/store/migrations/0050_course_v2.sql`
- `apps/api/internal/store/seed/courses/{a-mid,b-mid}.json`, `{a-mid,b-mid}-render-cache.json`
- `apps/api/internal/store/seed/courses/embed.go`
- `apps/api/internal/api/course_admin.go` (+ `_test.go`)
- `apps/web/src/shell/courses/SegmentTimeline.tsx`, `AssetView.tsx`

**Rewrite:**
- `packages/contracts/src/course.ts`
- `apps/api/internal/store/queries/course.sql` + `apps/api/internal/store/sqlc/course.sql.go`
- `apps/api/internal/agent/coursestore.go` (course content + progress store; drop session store)
- `apps/api/internal/api/course.go`, `course_dto.go`, `course_render.go`→ folded/removed
- `apps/api/internal/agent/course_coach.go` (simplified free-Q&A)
- `apps/api/internal/api/api.go` (route block lines 140–153)
- `apps/web/src/api/courses.ts`, `apps/web/src/api/index.ts`
- `apps/web/src/shell/courses/CoursePlayer.tsx`, `CourseReport.tsx`, `AskPanel.tsx`, `CoursesView.tsx`

**Delete:**
- `apps/api/internal/api/course_session.go`, `course_session_dto.go`, `course_assessment.go`, `course_assessment_input.go`, `course_step.go` + their `_test.go`
- `apps/api/internal/agent/course_step.go`, `course_coach_test.go` (rewrite), `course_step_test.go`
- `apps/api/internal/store/queries/coursesession.sql`, `apps/api/internal/store/sqlc/coursesession.sql.go`
- `apps/web/src/api/courseSession.ts`, `courseAssessment.ts`
- `apps/web/src/shell/courses/TeachingTemplate.tsx`, `ChallengeTemplate.tsx`
- old seed content in `0012_seed_course.sql` is superseded (leave the file; 0050 deletes its rows)

---

## Task 1: Contracts rewrite (`course.ts`)

**Files:**
- Rewrite: `packages/contracts/src/course.ts`
- Test: `packages/contracts/src/course.test.ts` (create if absent; else add cases)

**Interfaces — Produces:** `CourseSummary`, `CoursePlayerPayload`, `CourseStructure`, `RenderCache`, `RenderStepContent`, `RenderSegment`, `Interaction`, `CourseAsset`, `CourseProgress`, `CourseReport`. Removes `CourseSession`, `CourseMessage`, `CourseCardOffer`, `CourseCollectedCard`, `CourseStep`, `RenderedStep`, `CourseAssetKind`, `CourseStepKind`.

- [ ] **Step 1: Write failing test** — `packages/contracts/src/course.test.ts`

```ts
import { describe, it, expect } from "vitest";
import { RenderCache, Interaction, CoursePlayerPayload, CourseSummary, CourseReport } from "./course";

describe("course v2 contracts", () => {
  it("parses an ordering interaction", () => {
    const i = Interaction.parse({
      id: "q1", type: "ordering", prompt: "排序",
      options: [{ id: "A", text: "一" }, { id: "B", text: "二" }],
      correct_answer: ["A", "B"], explanation: "因为", remediation_questions: [],
    });
    expect(i.type).toBe("ordering");
  });
  it("parses a render cache with teaching + structure segments", () => {
    const rc = RenderCache.parse({
      version: "v4", courseId: "a-mid", courseTitle: "T",
      steps: [{ stepId: "step_01", content: {
        title: "t", subtitle: "s",
        segments: [{ kind: "teaching", flow_block_id: "b1", text: "hi", asset_ids: ["m1"], items: [] }],
        interactions: [], board: [],
      } }],
    });
    expect(rc.steps[0].content.segments[0].kind).toBe("teaching");
  });
  it("rejects an unknown interaction type", () => {
    expect(() => Interaction.parse({ id: "x", type: "essay", prompt: "", options: [], correct_answer: [], explanation: "", remediation_questions: [] })).toThrow();
  });
  it("summary + payload + report shapes", () => {
    CourseSummary.parse({ slug: "a-mid", branch: "A", title: "t", blurb: "b", time_label: "20 分钟", card_ids: ["craap"], step_count: 4 });
    CourseReport.parse({ title: "t", goal: "g", teaching_thread: "th", completedStepTitles: ["s1"], cardIds: ["craap"], secondsSpent: 600, quiz: { total: 4, correct: 3 } });
  });
});
```

- [ ] **Step 2: Run — expect FAIL** (`Interaction`/`RenderCache` not exported)

Run: `pnpm --filter @mind-imprint/contracts test -- course.test`
Expected: FAIL (import errors / undefined exports).

- [ ] **Step 3: Rewrite `packages/contracts/src/course.ts`**

```ts
import { z } from "zod";

export const CourseAssetType = z.enum(["image", "link", "text"]);
export const CourseAsset = z.object({
  id: z.string(),
  type: CourseAssetType,
  title: z.string().default(""),
  src: z.string().default(""),
  ossKey: z.string().optional().default(""),
  note: z.string().optional().default(""),
}).passthrough();

export const InteractionType = z.enum(["single_choice", "multiple_choice", "ordering"]);
export const InteractionOption = z.object({ id: z.string(), text: z.string() });
export const Interaction: z.ZodType<any> = z.lazy(() => z.object({
  id: z.string(),
  type: InteractionType,
  prompt: z.string(),
  options: z.array(InteractionOption).default([]),
  correct_answer: z.array(z.string()).default([]),
  explanation: z.string().default(""),
  remediation_questions: z.array(Interaction).default([]),
}).passthrough());

export const StructureItem = z.object({ label: z.string(), text: z.string() });
export const RenderSegment = z.object({
  kind: z.enum(["teaching", "structure"]),
  flow_block_id: z.string().default(""),
  text: z.string().default(""),
  asset_ids: z.array(z.string()).default([]),
  items: z.array(StructureItem).default([]),
}).passthrough();

export const RenderStepContent = z.object({
  title: z.string().default(""),
  subtitle: z.string().default(""),
  segments: z.array(RenderSegment).default([]),
  interactions: z.array(Interaction).default([]),
  board: z.array(z.unknown()).default([]),
}).passthrough();

export const RenderCacheStep = z.object({ stepId: z.string(), content: RenderStepContent }).passthrough();
export const RenderCache = z.object({
  version: z.string().default(""),
  courseId: z.string(),
  courseTitle: z.string().default(""),
  steps: z.array(RenderCacheStep),
}).passthrough();

// Structure: the player only reads id/title/steps[].{id,title,materials} and
// asset_library for asset resolution; everything else is authoring metadata.
export const CourseStructureStep = z.object({
  id: z.string(),
  title: z.string().default(""),
  materials: z.array(CourseAsset).default([]),
}).passthrough();
export const CourseStructure = z.object({
  id: z.string(),
  title: z.string(),
  course_goal: z.string().default(""),
  teaching_thread: z.string().default(""),
  steps: z.array(CourseStructureStep),
  asset_library: z.array(CourseAsset).default([]),
}).passthrough();

export const CourseSummary = z.object({
  slug: z.string(),
  branch: z.string(),
  title: z.string(),
  blurb: z.string(),
  time_label: z.string(),
  card_ids: z.array(z.string()),
  step_count: z.number().int(),
});

export const CoursePlayerPayload = z.object({
  slug: z.string(),
  title: z.string(),
  branch: z.string(),
  cardIds: z.array(z.string()),
  structure: CourseStructure,
  renderCache: RenderCache,
});

export const CourseProgress = z.object({
  course_slug: z.string(),
  current_ordinal: z.number().int(),
  completed_ordinals: z.array(z.number().int()),
  started_at: z.string().nullable(),
  completed_at: z.string().nullable(),
  updated_at: z.string(),
});

export const CourseReport = z.object({
  title: z.string(),
  goal: z.string(),
  teaching_thread: z.string(),
  completedStepTitles: z.array(z.string()),
  cardIds: z.array(z.string()),
  secondsSpent: z.number().int(),
  quiz: z.object({ total: z.number().int(), correct: z.number().int() }),
});

export type CourseAsset = z.infer<typeof CourseAsset>;
export type Interaction = z.infer<typeof Interaction>;
export type RenderSegment = z.infer<typeof RenderSegment>;
export type RenderStepContent = z.infer<typeof RenderStepContent>;
export type RenderCache = z.infer<typeof RenderCache>;
export type CourseStructure = z.infer<typeof CourseStructure>;
export type CourseSummary = z.infer<typeof CourseSummary>;
export type CoursePlayerPayload = z.infer<typeof CoursePlayerPayload>;
export type CourseProgress = z.infer<typeof CourseProgress>;
export type CourseReport = z.infer<typeof CourseReport>;
```

- [ ] **Step 4: Run — expect PASS**

Run: `pnpm --filter @mind-imprint/contracts test -- course.test`
Expected: PASS.

- [ ] **Step 5: Fix downstream type errors from removed exports** — `pnpm --filter @mind-imprint/contracts build`. The full-repo typecheck happens in later tasks (web) as those files are rewritten/deleted; here only ensure the contracts package builds.

- [ ] **Step 6: Commit**

```bash
git add packages/contracts/src/course.ts packages/contracts/src/course.test.ts
git commit -m "feat(contracts): course v2 types (render-cache driven, no session)"
```

---

## Task 2: DB migration 0050 + seed asset embed

**Files:**
- Create: `apps/api/internal/store/migrations/0050_course_v2.sql`
- Create: `apps/api/internal/store/seed/courses/{a-mid,b-mid}.json`, `{a-mid,b-mid}-render-cache.json`, `embed.go`
- Test: `apps/api/internal/store/migrate_test.go` (existing up/down test harness — confirm it exercises 0050)

**Interfaces — Produces:** `course` columns `slug/card_ids/structure/render_cache/step_count/updated_at`; `course_progress` with `started_at/completed_at`; embedded seed FS `seed/courses`.

- [ ] **Step 1: Copy the four reference JSONs into the seed dir**

```bash
mkdir -p apps/api/internal/store/seed/courses
cp docs/reference/class-agent/data/courses/a-mid.json apps/api/internal/store/seed/courses/a-mid.json
cp docs/reference/class-agent/data/courses/b-mid.json apps/api/internal/store/seed/courses/b-mid.json
cp docs/reference/class-agent/data/published/a-mid-v4-render-cache.json apps/api/internal/store/seed/courses/a-mid-render-cache.json
cp docs/reference/class-agent/data/published/b-mid-v3-render-cache.json apps/api/internal/store/seed/courses/b-mid-render-cache.json
```

- [ ] **Step 2: Create `embed.go`**

```go
package courses

import "embed"

//go:embed *.json
var FS embed.FS
```

- [ ] **Step 3: Write migration `0050_course_v2.sql`**

The seed rows are inserted by a Go seed routine (Task 4/5 wiring) rather than raw SQL — the JSON is large and needs `step_count`/`card_ids` computed. The migration handles schema + clearing old data.

```sql
-- +goose Up
-- Course v2: retire the phase-gated runtime. Drop the authored-step tables and
-- the session/message tables; reshape `course` to hold the externally-authored
-- structure + published render cache verbatim (+ attached tool cards); add
-- time-spent bookkeeping to course_progress. course.id stays uuid so the
-- event.course_id FK (0031) survives; a new slug is the external identifier.

DROP TABLE IF EXISTS course_message;
DROP TABLE IF EXISTS course_session;
DROP TABLE IF EXISTS course_step_render;
DROP TABLE IF EXISTS course_step;

-- Remove old seeded rows (0012). event.course_id ON DELETE CASCADE cleans up
-- any course-scoped events; course_progress rows cascade too.
DELETE FROM course;

ALTER TABLE course DROP COLUMN IF EXISTS tasks_count;
ALTER TABLE course DROP COLUMN IF EXISTS tools_count;
ALTER TABLE course ADD COLUMN slug         text NOT NULL DEFAULT '';
ALTER TABLE course ADD COLUMN card_ids     text[] NOT NULL DEFAULT '{}';
ALTER TABLE course ADD COLUMN structure    jsonb NOT NULL DEFAULT '{}';
ALTER TABLE course ADD COLUMN render_cache jsonb NOT NULL DEFAULT '{}';
ALTER TABLE course ADD COLUMN step_count   int  NOT NULL DEFAULT 0;
ALTER TABLE course ADD COLUMN updated_at   timestamptz NOT NULL DEFAULT now();
CREATE UNIQUE INDEX course_slug_uidx ON course (slug);

ALTER TABLE course_progress ADD COLUMN started_at   timestamptz;
ALTER TABLE course_progress ADD COLUMN completed_at timestamptz;

-- +goose Down
ALTER TABLE course_progress DROP COLUMN IF EXISTS completed_at;
ALTER TABLE course_progress DROP COLUMN IF EXISTS started_at;
DROP INDEX IF EXISTS course_slug_uidx;
ALTER TABLE course DROP COLUMN IF EXISTS updated_at;
ALTER TABLE course DROP COLUMN IF EXISTS step_count;
ALTER TABLE course DROP COLUMN IF EXISTS render_cache;
ALTER TABLE course DROP COLUMN IF EXISTS structure;
ALTER TABLE course DROP COLUMN IF EXISTS card_ids;
ALTER TABLE course DROP COLUMN IF EXISTS slug;
ALTER TABLE course ADD COLUMN tools_count int NOT NULL DEFAULT 0;
ALTER TABLE course ADD COLUMN tasks_count int NOT NULL DEFAULT 0;
-- NB: the dropped step/session tables are not recreated on Down (no back-compat).
```

- [ ] **Step 4: Run the migration up/down test** (testcontainers)

Run: `cd apps/api && go test ./internal/store/ -run Migrate -count=1`
Expected: PASS (up to 0050 then full down).

- [ ] **Step 5: Commit**

```bash
git add apps/api/internal/store/migrations/0050_course_v2.sql apps/api/internal/store/seed/courses/
git commit -m "feat(course): 0050 v2 schema (structure+render_cache+cards) & embed seed JSON"
```

---

## Task 3: sqlc queries (hand-edited) + delete session queries

**Files:**
- Rewrite: `apps/api/internal/store/queries/course.sql`, `apps/api/internal/store/sqlc/course.sql.go`
- Delete: `apps/api/internal/store/queries/coursesession.sql`, `apps/api/internal/store/sqlc/coursesession.sql.go`

**Interfaces — Produces (sqlc methods):** `ListCourseRows`, `GetCourseBySlug`, `UpsertCourse`, `GetCourseProgressBySlug`, `UpsertCourseProgress` (with started/completed), `AppendEvent` (existing, reused).

- [ ] **Step 1: Write `course.sql`** (replace the whole file)

```sql
-- name: ListCourseRows :many
SELECT slug, branch, title, blurb, time_label, card_ids, step_count
FROM course ORDER BY branch, title;

-- name: GetCourseBySlug :one
SELECT id, slug, branch, title, blurb, time_label, card_ids, step_count, structure, render_cache
FROM course WHERE slug = $1;

-- name: UpsertCourse :one
INSERT INTO course (slug, branch, title, blurb, time_label, card_ids, step_count, structure, render_cache, updated_at)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9, now())
ON CONFLICT (slug) DO UPDATE SET
  branch = EXCLUDED.branch, title = EXCLUDED.title, blurb = EXCLUDED.blurb,
  time_label = EXCLUDED.time_label, card_ids = EXCLUDED.card_ids,
  step_count = EXCLUDED.step_count, structure = EXCLUDED.structure,
  render_cache = EXCLUDED.render_cache, updated_at = now()
RETURNING id, slug;

-- name: GetCourseProgressBySlug :one
SELECT p.course_id, p.current_ordinal, p.completed_ordinals, p.started_at, p.completed_at, p.updated_at
FROM course_progress p JOIN course c ON c.id = p.course_id
WHERE p.user_id = $1 AND c.slug = $2;

-- name: UpsertCourseProgress :one
INSERT INTO course_progress (user_id, course_id, current_ordinal, completed_ordinals, started_at, completed_at, updated_at)
VALUES ($1,$2,$3,$4, COALESCE($5, now()), $6, now())
ON CONFLICT (user_id, course_id) DO UPDATE SET
  current_ordinal = EXCLUDED.current_ordinal,
  completed_ordinals = EXCLUDED.completed_ordinals,
  started_at = COALESCE(course_progress.started_at, EXCLUDED.started_at),
  completed_at = COALESCE(EXCLUDED.completed_at, course_progress.completed_at),
  updated_at = now()
RETURNING course_id, current_ordinal, completed_ordinals, started_at, completed_at, updated_at;
```

- [ ] **Step 2: Hand-edit `course.sql.go`** to match. Delete the old query methods (GetCourse/ListCourses/course_step/render). For EACH new query: params struct fields in `$` order; `rows.Scan(...)` in the SELECT's column order EXACTLY. `card_ids` scans into `[]string` (pgx supports `text[]`→`[]string`); `structure`/`render_cache` scan into `[]byte` (raw jsonb); `started_at`/`completed_at` into `pgtype.Timestamptz` (nullable). Model the Scan/param code on the sibling `snippet.sql.go` (also hand-edited, per project memory) for the `[]byte` jsonb + nullable-timestamp patterns.

- [ ] **Step 3: Delete session queries**

```bash
git rm apps/api/internal/store/queries/coursesession.sql apps/api/internal/store/sqlc/coursesession.sql.go
```

- [ ] **Step 4: Build** — `cd apps/api && go build ./...`
Expected: fails only in `internal/api`/`internal/agent` (handlers not yet rewritten — Tasks 4–7). The `internal/store/sqlc` package MUST build clean. Verify with `go build ./internal/store/...` → PASS.

- [ ] **Step 5: Commit**

```bash
git add apps/api/internal/store/queries/course.sql apps/api/internal/store/sqlc/course.sql.go
git commit -m "feat(course): v2 sqlc queries (slug lookup, upsert, progress+times); drop session queries"
```

---

## Task 4: Course content store + report/quiz logic (agent layer)

**Files:**
- Rewrite: `apps/api/internal/agent/coursestore.go` (content + progress + report + event helpers; drop all session/skill/card methods)
- Delete: `apps/api/internal/agent/course_step.go` and stale tests `course_step_test.go`, `course_coach_test.go`, `coursestore_test.go` (rewrite the last as Step 1)
- Test: `apps/api/internal/agent/coursestore_test.go`

**Interfaces — Consumes:** Task 3 sqlc methods. **Produces:**
- `type CourseSummaryRow struct { Slug, Branch, Title, Blurb, TimeLabel string; CardIDs []string; StepCount int }`
- `type CoursePlayerPayload struct { Slug, Title, Branch string; CardIDs []string; Structure, RenderCache json.RawMessage }`
- `(s) ListCourses(ctx) ([]CourseSummaryRow, error)`
- `(s) GetCoursePayload(ctx, slug) (CoursePlayerPayload, uuid.UUID, error)` (also returns course uuid)
- `(s) GetProgress(ctx, userID, slug) (CourseProgressRow, error)`
- `(s) SaveProgress(ctx, userID, courseUUID, currentOrdinal int, markCompletedAt bool) (CourseProgressRow, error)` — sets `completed_ordinals = existing ∪ {currentOrdinal}` server-side; sets `completed_at` when `markCompletedAt`.
- `(s) UpsertCourse(ctx, in UpsertCourseInput) error`
- `(s) LogCourseEvent(ctx, courseUUID uuid.UUID, typ string, payload []byte) error` — appends an `event` row with `Surface:"course"` + `course_id`. **NB:** `EventRow` (loop.go) has no `CourseID` field; add a course-scoped append. Model on `coursestore.go`'s existing course-scoped `AppendEvent` call (it already sets `Surface:"course"` and a course id param — reuse that sqlc `AppendEventParams` shape, which carries `CourseID`).
- `(s) CourseReport(ctx, userID, slug) (CourseReportData, error)` — computes: `completedStepTitles` from structure.steps indexed by `completed_ordinals`; `cardIds` from the row; `secondsSpent` = `completed_at|now − started_at`; `quiz{total,correct}` by counting `course_quiz_answered` events for this user+course (total attempts, correct=true count). Total quiz *questions* = sum over render cache steps of `len(content.interactions)`; report `quiz.total` = distinct-questions-answered or authored-total — use **authored total** (sum of interactions) and `correct` = count of `course_quiz_answered` events with `correct=true` deduped by `interactionId` (last attempt wins).

- [ ] **Step 1: Write failing store test** (testcontainers) covering: upsert a course from the embedded `a-mid` JSON → `ListCourses` returns it with `step_count=4`, `card_ids=["craap"]`; `GetCoursePayload` returns structure+renderCache; `SaveProgress` unions completed ordinals + sets started_at once; `CourseReport` returns 4 step titles + correct time + quiz tally after logging two `course_quiz_answered` events (one correct).

```go
// apps/api/internal/agent/coursestore_test.go — key assertions
func TestCourseStoreV2(t *testing.T) {
  // ... spin pool, run migrations, seed user ...
  raw, _ := courses.FS.ReadFile("a-mid.json")
  rc, _ := courses.FS.ReadFile("a-mid-render-cache.json")
  st := NewSqlcAgentStore(q, pool)
  must(st.UpsertCourse(ctx, UpsertCourseInput{Slug: "a-mid", Branch: "A", Title: "…", CardIDs: []string{"craap"}, Structure: raw, RenderCache: rc, StepCount: 4}))
  sums, _ := st.ListCourses(ctx)
  require.Len(t, sums, 1); require.Equal(t, 4, sums[0].StepCount); require.Equal(t, []string{"craap"}, sums[0].CardIDs)
  cu := /* course uuid from GetCoursePayload */
  st.SaveProgress(ctx, userID, cu, 0, false)
  st.SaveProgress(ctx, userID, cu, 3, true)
  st.LogCourseEvent(ctx, cu, "course_quiz_answered", []byte(`{"stepId":"step_01","interactionId":"intro_q1","selected":["B"],"correct":true}`))
  st.LogCourseEvent(ctx, cu, "course_quiz_answered", []byte(`{"stepId":"step_01","interactionId":"intro_q2","selected":["A"],"correct":false}`))
  rep, _ := st.CourseReport(ctx, userID, "a-mid")
  require.Equal(t, 1, rep.Quiz.Correct); require.Equal(t, 8, rep.Quiz.Total) // a-mid authored interactions total
  require.Contains(t, rep.CompletedStepTitles, "为什么需要信息过滤神器？")
}
```
(Confirm `a-mid` authored interaction total during implementation: sum `len(content.interactions)` over the render cache — adjust the literal `8` to the real count.)

- [ ] **Step 2: Run — expect FAIL** (`UpsertCourse` undefined). `cd apps/api && go test ./internal/agent/ -run TestCourseStoreV2 -count=1`

- [ ] **Step 3: Implement `coursestore.go`** per the Produces interface. Delete session/skill/card store methods. Use `encoding/json` only to compute `step_count` (len structure.steps) and report tallies; store structure/render_cache as raw bytes.

- [ ] **Step 4: Run — expect PASS.** `go test ./internal/agent/ -run TestCourseStoreV2 -count=1`

- [ ] **Step 5: Commit**

```bash
git add apps/api/internal/agent/coursestore.go apps/api/internal/agent/coursestore_test.go
git rm apps/api/internal/agent/course_step.go apps/api/internal/agent/course_step_test.go apps/api/internal/agent/course_coach_test.go
git commit -m "feat(course): v2 content store, progress union, simple report + quiz tally"
```

---

## Task 5: Course HTTP handlers + DTOs + route table

**Files:**
- Rewrite: `apps/api/internal/api/course.go`, `apps/api/internal/api/course_dto.go`
- Delete: `apps/api/internal/api/course_session.go`, `course_session_dto.go`, `course_assessment.go`, `course_assessment_input.go`, `course_render.go`, `course_step.go` + their `_test.go` (`course_test.go` rewritten in Step 1; keep the file, replace contents)
- Modify: `apps/api/internal/api/api.go:140-153` (route block)
- Test: `apps/api/internal/api/course_test.go`

**Interfaces — Consumes:** Task 4 store. **Produces handlers:** `listCourses`, `getCourse`, `getCourseProgress`, `putCourseProgress`, `postCourseQuizAnswer`, `getCourseReport` (and `postCourseAsk` from Task 6, `postAdminUploadCourse` from Task 7).

DTO JSON shapes MUST match the contracts (Task 1): summary uses `snake_case` per the existing course DTO convention (`step_count`, `card_ids`, `time_label`); payload uses `{ slug, title, branch, cardIds, structure, renderCache }` (structure/renderCache emitted as raw JSON via `json.RawMessage`); report matches `CourseReport`.

- [ ] **Step 1: Rewrite `course_test.go`** — httptest against a seeded course: `GET /courses` → 1+ summaries; `GET /courses/a-mid` → payload with `renderCache.steps` non-empty; `PUT …/progress {current_ordinal:1}` → 200 and a later `GET …/progress` shows `1 ∈ completed_ordinals` + non-null `started_at`; `POST …/quiz-answer {stepId,interactionId,selected,correct:false}` → 200 (never gates); `GET …/report` → shape with `quiz.total>0`. Unknown slug → 404.

- [ ] **Step 2: Run — expect FAIL.** `cd apps/api && go test ./internal/api/ -run TestCourseV2 -count=1`

- [ ] **Step 3: Implement handlers** in `course.go`. `getCourse` reads `{slug}` path param → `GetCoursePayload`; 404 on `pgx.ErrNoRows`. `putCourseProgress` accepts `{ current_ordinal int }` ONLY (ignore any client `completed_ordinals`, per the existing steps-viewed-floor rule); computes `markCompletedAt = current_ordinal == step_count-1`; on success also `LogCourseEvent("course_step_viewed", {ordinal})`. `postCourseQuizAnswer` validates body `{ stepId, interactionId, selected []string, correct bool }`, logs `course_quiz_answered`, returns `{ ok: true }`. All handlers ownership-gated via the session user (`protected`).

- [ ] **Step 4: Update `api.go` route block** — replace lines 140–153 with:

```go
	mux.Handle("GET /api/v1/courses", protected(a.listCourses))
	mux.Handle("GET /api/v1/courses/{slug}", protected(a.getCourse))
	mux.Handle("GET /api/v1/courses/{slug}/progress", protected(a.getCourseProgress))
	mux.Handle("PUT /api/v1/courses/{slug}/progress", protected(a.putCourseProgress))
	mux.Handle("POST /api/v1/courses/{slug}/quiz-answer", protected(a.postCourseQuizAnswer))
	mux.Handle("POST /api/v1/courses/{slug}/ask", protected(a.postCourseAsk))       // Task 6
	mux.Handle("GET /api/v1/courses/{slug}/report", protected(a.getCourseReport))
	mux.Handle("POST /api/v1/admin/courses", http.HandlerFunc(a.postAdminUploadCourse)) // Task 7 (admin-key gate inside)
```

- [ ] **Step 5: Delete retired handler files**

```bash
git rm apps/api/internal/api/course_session.go apps/api/internal/api/course_session_dto.go \
       apps/api/internal/api/course_assessment.go apps/api/internal/api/course_assessment_input.go \
       apps/api/internal/api/course_render.go apps/api/internal/api/course_step.go \
       apps/api/internal/api/course_session_test.go apps/api/internal/api/course_assessment_test.go \
       apps/api/internal/api/course_assessment_input_test.go apps/api/internal/api/course_render_test.go \
       apps/api/internal/api/course_render_store_test.go apps/api/internal/api/course_store_test.go
```
(Adjust the delete list to whatever `_test.go` files reference removed handlers — `go vet` will name them.)

- [ ] **Step 6: Run — expect PASS** for the new course tests; `go build ./...` clean except Task 6/7 handlers not yet added (add stubs returning 501 if needed to keep the tree building between tasks, then fill in). Prefer to land Tasks 5→6→7 before the full `go test ./internal/api/`.

- [ ] **Step 7: Commit**

```bash
git add apps/api/internal/api/course.go apps/api/internal/api/course_dto.go apps/api/internal/api/course_test.go apps/api/internal/api/api.go
git commit -m "feat(course): v2 handlers (payload/progress/quiz/report), slug routes; drop session/assessment/render endpoints"
```

---

## Task 6: Free-Q&A AI bar (coach + ask SSE)

**Files:**
- Rewrite: `apps/api/internal/agent/course_coach.go` (simplified)
- Modify: `apps/api/internal/api/course.go` (add `postCourseAsk`)
- Test: `apps/api/internal/agent/course_coach_test.go`

**Interfaces — Produces:** `BuildCourseAskPrompt(courseTitle, stepTitle, courseGoal, stepText string) (system string)`; `postCourseAsk` streams SSE `reply` frames (mirror `ChatContainer`/existing `postCourseAsk` SSE writer) and ends with `done`.

- [ ] **Step 1: Write failing test** for the prompt builder — asserts the system prompt names the step + course goal and contains the 铁律 guardrails ("不要直接给出测验答案", "不替学生下结论", "一次只问一个问题").

- [ ] **Step 2: Run — expect FAIL.** `cd apps/api && go test ./internal/agent/ -run TestCourseAskPrompt -count=1`

- [ ] **Step 3: Implement.** `BuildCourseAskPrompt` returns a concise system prompt:
> 你是「印记」，正在陪一名学生上《<courseTitle>》这门课的「<stepTitle>」这一步。本课目标：<courseGoal>。学生会自由提问，请简明地帮他把这一步想清楚。硬规则：① 不要直接给出本步测验题的正确答案，只引导他自己判断；② 不替他下结论、不替他写作；③ 一次只问一个问题；④ 回答简短。

`postCourseAsk`: load course by slug (for title/goal + current step text from render cache at the student's `current_ordinal`), run the coach turn on the mid-tier model (降级 allowed), meter the `llm_call` (`RecordCourseLLMCall(userID,"coach",…)` — reuse the existing course LLM-call recorder retained from the old code), `LogCourseEvent("course_asked", {q})`, stream `reply`. Body: `{ input string, ordinal int }`.

- [ ] **Step 4: Run — expect PASS.** `go test ./internal/agent/ -run TestCourseAskPrompt -count=1`

- [ ] **Step 5: Commit**

```bash
git add apps/api/internal/agent/course_coach.go apps/api/internal/agent/course_coach_test.go apps/api/internal/api/course.go
git commit -m "feat(course): free-Q&A AI bar (simplified coach + ask SSE, no phase logic)"
```

---

## Task 7: Admin-key course upload endpoint

**Files:**
- Create: `apps/api/internal/api/course_admin.go`, `apps/api/internal/api/course_admin_test.go`

**Interfaces — Consumes:** `ossBearer(r)` + `a.d.OSSAdminKey` (oss.go); Task 4 `UpsertCourse`; the card registry. **Produces:** `postAdminUploadCourse`.

- [ ] **Step 1: Write failing test.** With `OSSAdminKey:"admin-secret-123"`: valid bearer + body `{ course, renderCache, cardIds:["craap"] }` → 200 and the course is then listable; missing/wrong bearer → 401; `cardIds:["not-a-card"]` → 400 `validation_failed`; missing `course.steps` → 400; `render_cache.courseId != course.id` → 400.

- [ ] **Step 2: Run — expect FAIL.** `cd apps/api && go test ./internal/api/ -run TestAdminUploadCourse -count=1`

- [ ] **Step 3: Implement `course_admin.go`.**

```go
package api

// postAdminUploadCourse lets a developer push a pre-built course (structure JSON
// + published render cache + attached tool cards) with the OSS_ADMIN_KEY bearer.
// Border-validates the envelope; the class-agent repo owns inner shape + assets.
func (a *API) postAdminUploadCourse(w http.ResponseWriter, r *http.Request) {
	if a.d.OSSAdminKey == "" || ossBearer(r) != a.d.OSSAdminKey {
		httpx.WriteError(w, r, httpx.ErrUnauthorized()) // 401, no key echoed
		return
	}
	var body struct {
		Course      json.RawMessage `json:"course"`
		RenderCache json.RawMessage `json:"renderCache"`
		CardIDs     []string        `json:"cardIds"`
		Branch      string          `json:"branch"`
		Blurb       string          `json:"blurb"`
		TimeLabel   string          `json:"time_label"`
	}
	if err := decodeJSON(r, &body); err != nil { httpx.WriteError(w, r, err); return }

	// Outer-envelope validation.
	var cs struct{ ID, Title string; Steps []json.RawMessage `json:"steps"` }
	if err := json.Unmarshal(body.Course, &cs); err != nil || cs.ID == "" || cs.Title == "" || len(cs.Steps) == 0 {
		httpx.WriteError(w, r, httpx.ErrBadRequest("validation_failed", "course 结构缺少 id/title/steps", nil)); return
	}
	var rc struct{ Version, CourseID string; Steps []struct{ StepID string `json:"stepId"`; Content json.RawMessage `json:"content"` } `json:"steps"` }
	if err := json.Unmarshal(body.RenderCache, &rc); err != nil || len(rc.Steps) == 0 {
		httpx.WriteError(w, r, httpx.ErrBadRequest("validation_failed", "renderCache 缺少 steps", nil)); return
	}
	if rc.CourseID != cs.ID {
		httpx.WriteError(w, r, httpx.ErrBadRequest("validation_failed", "renderCache.courseId 与 course.id 不一致", nil)); return
	}
	for _, id := range body.CardIDs {
		if !cards.Exists(id) { // registry membership check — see Step 4
			httpx.WriteError(w, r, httpx.ErrBadRequest("validation_failed", "未知的工具卡: "+id, nil)); return
		}
	}
	store := agent.NewSqlcAgentStore(a.d.Queries, a.d.Pool)
	if err := store.UpsertCourse(r.Context(), agent.UpsertCourseInput{
		Slug: cs.ID, Branch: body.Branch, Title: cs.Title, Blurb: body.Blurb, TimeLabel: body.TimeLabel,
		CardIDs: body.CardIDs, Structure: body.Course, RenderCache: body.RenderCache, StepCount: len(cs.Steps),
	}); err != nil { httpx.WriteError(w, r, httpx.ErrInternal()); return }
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"slug": cs.ID, "step_count": len(cs.Steps)})
}
```

- [ ] **Step 4: Add a registry membership check** in the Go cards package. Confirm the exact helper name/path in `apps/api/internal/cards` (a `Registry`/`Specs` map already backs `summon_card`); add `func Exists(id string) bool` if absent, over the same embedded specs. Reference it as `cards.Exists`.

- [ ] **Step 5: Run — expect PASS.** `go test ./internal/api/ -run TestAdminUploadCourse -count=1`

- [ ] **Step 6: Commit**

```bash
git add apps/api/internal/api/course_admin.go apps/api/internal/api/course_admin_test.go
git commit -m "feat(course): admin-key upload endpoint (border-validated structure+render_cache+cards)"
```

---

## Task 8: Web API client rewrite

**Files:**
- Rewrite: `apps/web/src/api/courses.ts`
- Delete: `apps/web/src/api/courseSession.ts`, `apps/web/src/api/courseAssessment.ts`
- Modify: `apps/web/src/api/index.ts` (imports, `Api` interface, registrations)

**Interfaces — Produces (client):**

```ts
// apps/web/src/api/courses.ts
import type { CourseSummary, CoursePlayerPayload, CourseProgress, CourseReport } from "@mind-imprint/contracts";
import { apiFetch, apiStream } from "./client"; // apiStream: existing SSE helper used by chat/studio

export async function listCourses(): Promise<CourseSummary[]> {
  return (await apiFetch<{ courses: CourseSummary[] }>("/api/v1/courses")).courses;
}
export async function getCourse(slug: string): Promise<CoursePlayerPayload> {
  return (await apiFetch<{ course: CoursePlayerPayload }>(`/api/v1/courses/${slug}`)).course;
}
export async function getCourseProgress(slug: string): Promise<CourseProgress> {
  return (await apiFetch<{ progress: CourseProgress }>(`/api/v1/courses/${slug}/progress`)).progress;
}
export async function saveCourseProgress(slug: string, input: { current_ordinal: number }): Promise<CourseProgress> {
  return (await apiFetch<{ progress: CourseProgress }>(`/api/v1/courses/${slug}/progress`, { method: "PUT", body: JSON.stringify(input) })).progress;
}
export async function answerCourseQuiz(slug: string, body: { stepId: string; interactionId: string; selected: string[]; correct: boolean }): Promise<void> {
  await apiFetch(`/api/v1/courses/${slug}/quiz-answer`, { method: "POST", body: JSON.stringify(body) });
}
export async function getCourseReport(slug: string): Promise<CourseReport> {
  return (await apiFetch<{ report: CourseReport }>(`/api/v1/courses/${slug}/report`)).report;
}
export async function* courseAsk(slug: string, input: string, ordinal: number): AsyncGenerator<{ type: "reply"; body: string } | { type: "error"; message: string }> {
  // mirror the existing SSE reader in courseSession.ts (courseAsk) — reply/error frames only
  yield* apiStream(`/api/v1/courses/${slug}/ask`, { input, ordinal });
}
```

- [ ] **Step 1: Rewrite `courses.ts`** as above (align `apiStream` usage to the real helper signature used by `chat.ts`/`studio.ts`).
- [ ] **Step 2: Delete session/assessment clients** — `git rm apps/web/src/api/courseSession.ts apps/web/src/api/courseAssessment.ts`
- [ ] **Step 3: Update `index.ts`** — drop the deleted imports (lines 11/23/24) and the removed `Api` methods (`renderCourseStep`, `startCourseSession`, `getCourseSession`, `restartCourseSession`, `submitCourseCard`, `skipCourseCard`, `courseAdvance`, `getCourseAssessment`, `generateCourseAssessment`); add `getCourseReport`, `answerCourseQuiz`, and the new `courseAsk` signature; fix the type-only import list (drop `Course`, `RenderedStep`, `CourseSession`; add `CoursePlayerPayload`, `CourseReport`).
- [ ] **Step 4: Typecheck** — `pnpm --filter @mind-imprint/web typecheck`. Expected: fails only in `CoursePlayer.tsx`/`CourseReport.tsx` (Tasks 9–11). The `api/` layer must be clean.
- [ ] **Step 5: Commit**

```bash
git add apps/web/src/api/courses.ts apps/web/src/api/index.ts
git commit -m "feat(web): course v2 API client (payload/progress/quiz/report/ask); drop session+assessment clients"
```

---

## Task 9: `SegmentTimeline` + `AssetView` (ported from `main.jsx`)

**Files:**
- Create: `apps/web/src/shell/courses/SegmentTimeline.tsx`, `apps/web/src/shell/courses/AssetView.tsx`

**Interfaces — Consumes:** `RenderStepContent`, `Interaction`, `CourseAsset` (contracts); `answerCourseQuiz`; `api.resolveUrl`. **Produces:**
- `AssetView({ asset }: { asset: CourseAsset })`
- `SegmentTimeline({ content, assetsById, onQuizAnswer }: { content: RenderStepContent; assetsById: Record<string, CourseAsset>; onQuizAnswer: (i: { stepId: string; interactionId: string; selected: string[]; correct: boolean }) => void })`

Port the student-facing render logic from `docs/reference/class-agent/src/main.jsx` — specifically the timeline build + segment/interaction rendering (`buildLessonTimeline`, `renderTimelineItems`, the teaching/structure/interaction branches, `splitTeachingTextAroundAssets`, `selectedAnswerIds`, `isInteractionCorrect`, the blackboard/结构 summary). **Adaptations (the real deltas — do not copy the reference's data-fetching or teacher code):**
1. Types come from `@mind-imprint/contracts`, not the reference's ad-hoc normalizers. Skip the reference's `cleanStudentFacingText`/`normalize*` sanitizers unless a rendered string is visibly corrupted; the render cache is already clean authored content.
2. Interleave interactions by `flow_block_id`: match each `content.interactions[i]` to the segment order via its `interaction_id`/`id` (same matching the reference's `resolveInteractionForBlock` uses); fall back to appending unmatched interactions after the segments (mirror `missingInteractions`).
3. Reveal-on-click: keep the `revealedSegmentCount` + "点击页面继续" behavior.
4. `single_choice`/`multiple_choice`: option buttons → on submit compute `correct = set(selected) == set(interaction.correct_answer)`, show ✓/✗ + `explanation`, call `onQuizAnswer`. `ordering`: a reorderable list (up/down buttons — no dnd dependency) → check the order equals `correct_answer`.
5. `structure` segment → 板书 card from `items[]` (port the `.blackboard` layout, inline-styled to match `CoursePlayer`'s palette `#2A3B7A`/`#4C9A82`).
6. `AssetView`: `type==="image"` → `useEffect(() => resolveUrl(objectKeyFrom(asset)))` then `<img>` with an `onError` placeholder; `objectKeyFrom` parses `objectKey=` from `asset.src` (URL-decode) and falls back to `asset.ossKey`. `type==="link"` → titled `<a target="_blank" rel="noreferrer noopener">`. `type==="text"` → titled block.

- [ ] **Step 1: Write a component smoke test** (Vitest + Testing Library, following any existing web component test): render `SegmentTimeline` with a two-segment + one `single_choice` fixture; asserting the teaching text shows, selecting the correct option then submitting shows the explanation and calls `onQuizAnswer` with `correct:true`; an `ordering` fixture reorders and validates. (If the web package has no component-test setup, instead add a pure-function test for the answer-check helpers `isInteractionCorrect`/`orderMatches` exported from the file, and verify rendering in the Task 12 live check.)

- [ ] **Step 2: Run — expect FAIL.**  `pnpm --filter @mind-imprint/web test -- SegmentTimeline`

- [ ] **Step 3: Implement `AssetView.tsx` then `SegmentTimeline.tsx`** per the adaptations.

- [ ] **Step 4: Run — expect PASS.**

- [ ] **Step 5: Commit**

```bash
git add apps/web/src/shell/courses/SegmentTimeline.tsx apps/web/src/shell/courses/AssetView.tsx apps/web/src/shell/courses/SegmentTimeline.test.tsx
git commit -m "feat(web): SegmentTimeline + AssetView ported from reference (quizzes, 板书, OSS assets)"
```

---

## Task 10: `CoursePlayer` rewrite + `AskPanel` simplification

**Files:**
- Rewrite: `apps/web/src/shell/courses/CoursePlayer.tsx`
- Modify: `apps/web/src/shell/courses/AskPanel.tsx` (remove offer props/rendering)

**Interfaces — Consumes:** Task 8 client, Task 9 components. `CoursePlayer` props unchanged: `{ courseId: string; onExit; onFinish }` (courseId is now the slug).

- [ ] **Step 1: Simplify `AskPanel`** — remove `onAcceptOffer`/`onDismissOffer`/`onCardSubmit`/`onCardSkip` from `AskPanelProps` and the offer-rendering branch in `AskMessage`; keep `expanded/onToggle/branchColor/context/chips/messages/pending/onSend`. Keep `AskMessage = { id; role; text }`.

- [ ] **Step 2: Rewrite `CoursePlayer`.** Keep the exact header + top segmented progress bar markup (lines 267–289) but drive it from `payload.renderCache.steps.length` and local `ordinal`. Body:
  - Load `getCourse(slug)` → payload; `getCourseProgress` → resume `ordinal`; build `assetsById` from `payload.structure.asset_library` ∪ each step's `materials`.
  - Render `<SegmentTimeline content={payload.renderCache.steps[ordinal].content} assetsById={assetsById} onQuizAnswer={(i) => void answerCourseQuiz(slug, i)} />`.
  - Prev arrow (ordinal>0) and Next arrow (always enabled). On advancing, `saveCourseProgress(slug, { current_ordinal: next })`. On the last step's Next → route to report via `onFinish()` (the parent shows `CourseReport`).
  - AI bar: `<AskPanel … messages onSend={handleAsk} />`; `handleAsk` appends the student msg then consumes `courseAsk(slug, text, ordinal)` reply/error frames into one assistant message (mirror the old `consumeCourseTurn`, minus card/phase frames).
  - Delete ALL phase/session/card state + effects (`session`, `phaseSteps`, `INFO_LITERACY_COURSE_SKILL`, `openCards`, `handleAcceptOffer`, etc.).

- [ ] **Step 3: Typecheck** — `pnpm --filter @mind-imprint/web typecheck`. Expected: clean except `CourseReport.tsx` (Task 11) if still referencing old types.

- [ ] **Step 4: Commit**

```bash
git add apps/web/src/shell/courses/CoursePlayer.tsx apps/web/src/shell/courses/AskPanel.tsx
git commit -m "feat(web): linear CoursePlayer (render-cache timeline + free AI bar), drop phase runtime"
```

---

## Task 11: `CourseReport` (simple stats) + `CoursesView` tweak + template deletion

**Files:**
- Rewrite: `apps/web/src/shell/courses/CourseReport.tsx`
- Modify: `apps/web/src/shell/courses/CoursesView.tsx` (tools count from `card_ids`), `CoursesContainer.tsx` (wire report fetch if it currently fetches the old assessment)
- Delete: `apps/web/src/shell/courses/TeachingTemplate.tsx`, `ChallengeTemplate.tsx`

**Interfaces — Consumes:** `getCourseReport`, `CourseReport`, `CardCatalogEntry` (for card chip labels — reuse the card gallery's catalog fetch/labels).

- [ ] **Step 1: Rewrite `CourseReport`** to render `getCourseReport(slug)`:
  - 学到了什么: `goal` + `teaching_thread` + a list of `completedStepTitles`.
  - 学到的工具卡: `cardIds` → card chips (title from the catalog; click → jump to the card gallery, reusing the existing gallery navigation).
  - 用时: `secondsSpent` formatted `mm:ss` / `x 分钟`.
  - 小测表现: `quiz.correct / quiz.total`.
  - No LLM, no `DualAxisReport`.
- [ ] **Step 2: `CoursesView`** — the card already reads `course.tasks_count`/`tools_count`; change to `course.step_count` (个任务) and `course.card_ids.length` (个工具). Update the `CourseSummary` field references + `CoursesContainer` progress % (uses `step_count`).
- [ ] **Step 3: Delete templates** — `git rm apps/web/src/shell/courses/TeachingTemplate.tsx apps/web/src/shell/courses/ChallengeTemplate.tsx`
- [ ] **Step 4: Typecheck + web test** — `pnpm --filter @mind-imprint/web typecheck && pnpm --filter @mind-imprint/web test`. Expected: PASS, no dangling references to deleted modules.
- [ ] **Step 5: Commit**

```bash
git add apps/web/src/shell/courses/CourseReport.tsx apps/web/src/shell/courses/CoursesView.tsx apps/web/src/shell/courses/CoursesContainer.tsx
git commit -m "feat(web): simple course report (learned/cards/time/quiz) + tools count from card_ids"
```

---

## Task 12: Seed wiring, full builds, cleanup, live OSS verify

**Files:**
- Modify: whatever bootstraps DB seed on startup/migrate (confirm how seeds run — a Go seed step or a migration-time routine that reads `courses.FS`)
- Verify: full suites across all three packages

**Interfaces — Consumes:** everything above.

- [ ] **Step 1: Wire the seed.** Ensure `a-mid` + `b-mid` are upserted from `courses.FS` on migrate/boot (the same place old seed data was applied), computing `step_count`, setting `card_ids` (`a-mid → ["craap"]`, `b-mid → ["sift","craap","argument-map","concession","metacognition"]`), `branch`/`blurb`/`time_label` (author short blurbs; `time_label` e.g. "约 20 分钟"). Confirm each `cardId` exists in `packages/contracts/cards/` before committing the set.

- [ ] **Step 2: Grep for dangling references** to removed symbols across the repo (fail the task if any remain in built code): `INFO_LITERACY_COURSE_SKILL` (course usage), `CourseSession`, `renderCourseStep`, `TeachingTemplate`, `ChallengeTemplate`, `course_step`, `courseAdvance`, `generateCourseAssessment`. Remove leftover imports/exports (`packages/contracts/src/index.ts` still re-exports `courseSkill` — keep the skill file if other surfaces use it, but ensure the course path no longer imports it).

- [ ] **Step 3: Full builds & suites.**
  - Contracts: `pnpm --filter @mind-imprint/contracts test`
  - Web: `pnpm --filter @mind-imprint/web typecheck && pnpm --filter @mind-imprint/web test && pnpm --filter @mind-imprint/web build`
  - Go: `cd apps/api && go build ./... && go vet ./... && go test ./internal/agent/ ./internal/api/ ./internal/store/ -count=1` (the `internal/api` testcontainers suite is ~7–8 min — run in the background and wait).

- [ ] **Step 4: Live OSS-key verification (the flagged risk).** Against the deployed OSS resolver, confirm a sample of the seeded asset object keys (e.g. `courses/8c617e2d-a222-4299-ae91-ff1508a88631.webp` from `a-mid`) resolve via `POST /api/v1/oss/resolve-url`. If they resolve → images render. If they 404 → the `AssetView` placeholder covers it gracefully; record in the progress ledger + memory that assets need re-upload via the class-agent pipeline (out of scope), and do NOT block the merge on it.

- [ ] **Step 5: Commit**

```bash
git add -u
git add apps/api/internal/store/seed/  # any new seed-wiring file
git commit -m "feat(course): seed a-mid+b-mid, wire embed seed, retire phase runtime end-to-end"
```

---

## Self-Review notes (author)

- **Spec coverage:** runtime→Tasks 5,6,10; data model→Tasks 2,3,4; contracts→1; interactions incl. ordering→9; tool cards→2,7,11; admin upload→7; simple report→4,11; seed+retire old→2,12; OSS resolution→9,12. All covered.
- **Type consistency:** `UpsertCourseInput`, `CourseSummaryRow`, `CoursePlayerPayload` (Go) named once in Task 4 and consumed by 5/7; `courseAsk(slug,input,ordinal)` signature identical in Tasks 6/8/10; `answerCourseQuiz` body shape identical in 8/9/10; `CourseReport` shape identical in 1/4/11.
- **Known confirm-at-implementation points (not placeholders):** exact `a-mid` interaction total for the report test (count from the cache); the Go cards registry helper name for `Exists`; the `apiStream` SSE helper signature; the exact `_test.go` delete list (let `go vet` name them); how seeds are applied at migrate/boot. Each names the file to check.
