# Course Part v2 — Reference Show-Logic + Admin Upload · Design

> Date: 2026-08-02 · Status: approved (design), pending implementation plan.
> Supersedes the phase-gated course runtime (Slice-12 `INFO_LITERACY_COURSE_SKILL`
> session model). No backward compatibility with the old course model is required.

## Goal

Replace the course part's runtime and display with the **class-agent** reference's
show-logic: a linear, self-paced player driven by an externally-authored
**course structure JSON** + a **published render cache**. Keep our top per-step
progress bar and side AI bar. Ship `a-mid` and `b-mid` as the two example
courses. Add an admin-key upload endpoint so developers can push new course
content. Each course carries a 0..many attached tool-cards list.

Authoritative reference (read-only, not part of the student platform build):
`docs/reference/class-agent/` — `data/courses/{a,b}-mid.json` (structure),
`data/published/{a-mid-v4,b-mid-v3}-render-cache.json` (published cache),
`src/main.jsx` (student display logic).

## Principles (铁律 alignment)

- **① AI 克制：** the AI bar answers free student questions about the current
  step; it never gives away quiz answers or writes conclusions for the student.
- **② 不操纵：** quizzes give immediate feedback but **never hard-gate**
  advancement; paging is always free (forward and back). No streaks/leaderboards.
- **③ 一次只问一个：** the free-Q&A coach replies conversationally, one question
  at a time.
- **④ 过程即数据：** every quiz attempt (including wrong ones and skips) and every
  AI-bar turn is logged as a process event.

## Scope

### In scope
1. Clean rewrite of the course data model, contracts, backend endpoints, and
   frontend player to be driven by the reference structure JSON + render cache.
2. Linear player: reveal-on-click segment timeline, inline interactions
   (`single_choice` / `multiple_choice` / `ordering`), 板书 (structure) summary,
   prev/next, image/link assets.
3. Free-Q&A AI bar (no phase gating, no card offers).
4. Attached tool-cards (`card_ids`) as course metadata.
5. Simple, no-LLM course report (learned / cards / time / quiz score).
6. Admin-key upload endpoint (`OSS_ADMIN_KEY` bearer) accepting the structure
   JSON + pre-built render cache + card_ids, upsert by slug.
7. Seed `a-mid` + `b-mid`; retire the old info-literacy phase seed/runtime.

### Out of scope (lives in the class-agent repo)
- docx → course-JSON extraction; asset extraction/upload pipeline.
- Teacher-facing course editor.
- AI (re)generation of the render cache (the cache is uploaded pre-built).
- Any migration/coexistence with the old phase-gated course model.

## Data model (clean rewrite)

The reference identifies courses by **slug** (`a-mid`, `b-mid`), and the admin
upload upserts by slug, so the course identifier becomes the slug.

```sql
-- new course table (replaces course / course_step / course_step_render)
CREATE TABLE course (
    slug         text PRIMARY KEY,          -- 'a-mid'
    branch       text NOT NULL DEFAULT '',  -- catalog grouping label (e.g. 'A')
    title        text NOT NULL,
    blurb        text NOT NULL DEFAULT '',
    time_label   text NOT NULL DEFAULT '',
    card_ids     text[] NOT NULL DEFAULT '{}',   -- attached tool cards (metadata)
    step_count   int  NOT NULL DEFAULT 0,
    structure    jsonb NOT NULL,            -- course structure JSON, verbatim
    render_cache jsonb NOT NULL,            -- published render cache, verbatim
    created_at   timestamptz NOT NULL DEFAULT now(),
    updated_at   timestamptz NOT NULL DEFAULT now()
);

-- course_progress keyed by slug; + time-spent bookkeeping
CREATE TABLE course_progress (
    user_id            uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    course_slug        text NOT NULL REFERENCES course(slug) ON DELETE CASCADE,
    current_ordinal    int  NOT NULL DEFAULT 0,
    completed_ordinals int[] NOT NULL DEFAULT '{}',
    started_at         timestamptz,          -- set on first render/open
    completed_at       timestamptz,          -- set when the last step is reached
    updated_at         timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, course_slug)
);
```

- **Dropped:** `course_step`, `course_step_render`, and all coursesession /
  course-assessment tables + code.
- **Border validation only** (信封 principle): Go validates the outer envelope on
  upload — structure has `id` + `title` + `steps[]`; render cache has `version` +
  `steps[]` where each step has `stepId` + `content`. Inner content is stored
  verbatim as `jsonb` and trusted by the player (contracts own the inner shape).
- Process events (`event` table, surface `course`): `course_quiz_answered`
  `{ stepId, interactionId, selected, correct }`, `course_asked`, plus the
  existing render/open event for the steps-viewed floor.

## Contracts (`packages/contracts/src/course.ts` rewrite)

New Zod types mirroring the reference (border-validated; inner content is
`z.object({}).passthrough()` where the reference carries authoring-only fields
the player ignores):

- `CourseAsset` — `{ id, type: "image"|"link"|"text", title, src, ossKey?, note? }`.
- `Interaction` — `{ id, type: "single_choice"|"multiple_choice"|"ordering",
  prompt, options: {id,text}[], correct_answer: string[], explanation,
  remediation_questions: Interaction[] }`.
- `RenderSegment` — `{ kind: "teaching"|"structure", flow_block_id, text,
  asset_ids: string[], items: {label,text}[] }`.
- `RenderStepContent` — `{ title, subtitle, segments: RenderSegment[],
  interactions: Interaction[], board: [] }`.
- `RenderCache` — `{ version, courseId, courseTitle, steps: { stepId, content:
  RenderStepContent }[] }`.
- `CourseStructure` — `{ id, title, course_goal, teaching_thread,
  steps: {...}[], asset_library: CourseAsset[] }` (steps passthrough; the player
  only reads `id`, `title`, `materials`/`asset_library` for asset resolution).
- `CoursePlayerPayload` — `{ slug, title, branch, cardIds, structure,
  renderCache }` (what `GET /courses/{slug}` returns).
- `CourseSummary` — `{ slug, branch, title, blurb, time_label, card_ids,
  step_count }` (`tools_count` derived from `card_ids.length`).
- `CourseProgress` — `{ course_slug, current_ordinal, completed_ordinals,
  started_at, completed_at, updated_at }`.
- `CourseReport` — `{ title, goal, teaching_thread, completedStepTitles: string[],
  cardIds: string[], secondsSpent: number, quiz: { total, correct } }`.

Remove the old `CourseSession` / `CourseMessage` / `CourseCardOffer` /
`CourseCollectedCard` / `CourseStep` / `RenderedStep` types.

## Backend (`apps/api/internal/api/course*.go`, `agent/course*.go`)

Routes (all `protected` except the admin upload):

| Method + path | Handler | Notes |
|---|---|---|
| `GET /api/v1/courses` | `listCourses` | summaries; `tools_count = len(card_ids)` |
| `GET /api/v1/courses/{slug}` | `getCourse` | returns `CoursePlayerPayload` (structure + renderCache + cardIds); no per-step render call |
| `GET /api/v1/courses/{slug}/progress` | `getCourseProgress` | + started/completed |
| `PUT /api/v1/courses/{slug}/progress` | `putCourseProgress` | accepts `current_ordinal` only; sets `started_at` on first open, `completed_at` when last ordinal reached; writes the steps-viewed event/`completed_ordinals` server-side |
| `POST /api/v1/courses/{slug}/quiz-answer` | `postCourseQuizAnswer` | logs `course_quiz_answered` (铁律④); returns `{ ok: true }` — never gates |
| `POST /api/v1/courses/{slug}/ask` (SSE) | `postCourseAsk` | free-Q&A coach turn; persists the turn + logs `course_asked`; simplified prompt (step context, 铁律 restraint, one question, no answer-giving) |
| `GET /api/v1/courses/{slug}/report` | `getCourseReport` | computes `CourseReport` from progress + structure + quiz events; no LLM |
| `POST /api/v1/admin/courses` | `postAdminUploadCourse` | **`OSS_ADMIN_KEY` bearer gate** (reuse `gateAdminKey` from `oss.go`); body `{ course, renderCache, cardIds }`; validates outer envelope + `card_ids ⊆ registry`; upserts by slug; returns the summary |

- **Retire** the session/advance/render/assessment/card endpoints (lines
  `api.go:144-153`) and their handlers/store/tests.
- **AI bar coach** (`agent/course_coach.go` simplified): system prompt = the
  student is on `<step title>` of `<course title>`; goal `<course_goal>`; answer
  the student's question helpfully and briefly, one question at a time; never
  reveal a quiz's correct answer; never write the student's essay/conclusion.
  Uses the mid-tier model (降级 allowed), meters the `llm_call`.
- **Asset resolution:** the player resolves each `image` asset's OSS object key
  via the existing `POST /api/v1/oss/resolve-url`. The uploaded `src` uses the
  class-agent convention `/api/oss/read?objectKey=<urlencoded key>`; the player
  parses the `objectKey` param (falling back to the asset's `ossKey` field) and
  calls `resolveUrl`. `link`/`text` assets render without OSS.

## Frontend (`apps/web/src/shell/courses/`)

- **`CoursePlayer.tsx` rewrite:** keep the header + top segmented progress bar +
  `AskPanel` (now a free helper: `onSend` → `postCourseAsk`; no offers, no phase
  chips beyond generic ones). Body renders a new **`SegmentTimeline`**.
- **`SegmentTimeline.tsx` (new, ported from `main.jsx`):**
  - reveal-on-click: segments appear progressively; "点击页面继续" hint.
  - `teaching` segment → text with inline image/link assets (split around assets).
  - `structure` segment → 板书 card (`items[]` label/text).
  - inline interactions interleaved by `flow_block_id` order:
    - `single_choice` / `multiple_choice` → option buttons → on submit show
      correct/incorrect + `explanation`; log via `quiz-answer`.
    - `ordering` → reorderable list → check against `correct_answer` order.
  - prev/next nav; next is always enabled (no gating); finishing the last step
    sets `completed_at` and routes to the report.
- **`AssetView.tsx` (new/ported):** `image` → `resolveUrl` then `<img>` with a
  broken-image placeholder fallback; `link` → titled external link; preview modal.
- **`CoursesView.tsx`:** unchanged card layout; `工具` count from `card_ids`.
- **`CourseReport.tsx` rewrite:** simple stats — 学到了什么 (goal +
  teaching_thread + completed step titles), 学到的工具卡 (card chips linking to
  the card gallery), 用时 (`secondsSpent` formatted), 小测表现 (`correct/total`).
- Remove all phase/session/card-offer client code
  (`courseSession.ts`, `courseAssessment.ts` client, `TeachingTemplate`,
  `ChallengeTemplate`, `AskPanel` offer props).

## Seed & assets

- Copy the four reference JSON files into the repo under
  `apps/api/internal/store/seed/courses/` (`a-mid.json`, `b-mid.json`,
  `a-mid-render-cache.json`, `b-mid-render-cache.json`), `go:embed` them, and
  seed both courses in a migration (or a seed routine invoked by the migration),
  computing `step_count` and setting `card_ids`.
- **`card_ids` (validated against the 34-card registry during planning):**
  - `a-mid` (CRRAAB 信源评估) → `["craap"]`.
  - `b-mid` (SIFT + 良构论证 + 让步段 + 元认知) → candidates
    `["craap","argument-map","concession","metacognition"]` (final set confirmed
    against `packages/contracts/cards/*.json` in the plan; only registry ids ship).
- Retire the old `0012_seed_course.sql` content (superseded by the new schema +
  seed).

## Risks

- **OSS key resolution (must verify):** the seeded/uploaded asset object keys
  (`courses/…webp`) must exist in our OSS bucket for images to render. Verify
  against the live bucket during the build. If a key does not resolve, the
  player shows a placeholder (never a broken layout); document the re-upload path
  (the asset pipeline in the class-agent repo, out of scope here).
- **Render-cache/structure drift:** the player reads display content from the
  render cache but resolves assets from the structure's `asset_library`. Upload
  validates both are present and reference the same `courseId`.

## Acceptance (end-to-end)

1. `GET /courses` lists `a-mid` + `b-mid` with correct `工具` counts.
2. Opening `a-mid` renders step 1's teaching segments + the two `single_choice`
   quizzes inline; answering logs a `course_quiz_answered` event; a wrong answer
   still lets the student proceed.
3. The AI bar answers a free question about the current step and refuses to hand
   over a quiz answer.
4. `b-mid` renders an `ordering` interaction correctly.
5. Images resolve through `resolve-url` (or fall back to a placeholder).
6. Finishing a course shows the simple report with goal, card chips, time spent,
   and quiz score.
7. `POST /api/v1/admin/courses` with a valid `OSS_ADMIN_KEY` bearer upserts a
   course; without the key it is rejected; unknown `card_ids` are rejected.
8. No references remain to the old phase runtime (session/advance/render/
   assessment) in built code.
