# Slice 4 · Courses Pillar (hybrid AI-generated) — Design Spec

> 2026-07-07 · Part of `docs/2026-07-06-refactor-roadmap.md` (Slice 4 of 5). Stacked on `slice-3-anchored-cards`.
> Binding design: `思维印记 工作区.dc.html` — `COURSES: LIST`, `COURSE PLAYER`, `COURSE REPORT` sections.
> Authority: backend/DB/API per `docs/architecture/*` (Go + Postgres). UI per the `.dc.html`.
> **Autonomy note:** built under the user's "loop until finished" mandate; the two shaping decisions (backend course content; hybrid AI-generated-content-within-templates) were made WITH the user (they chose them) and are documented here.

## Goal

Deliver the Courses pillar: AI-interactive courses where the **backend stores each step's purpose + assets + kind**, and the **AI generates the teaching content / challenge questions at runtime**, rendered through a small set of **fixed layout templates**. Course challenges **reuse the Slice-3 material-anchored engine**. Voice (TTS/STT) is deferred to a later "4-voice" sub-slice.

## Core idea (the hybrid)

**The AI decides _what to teach and what to ask_; fixed templates decide _how it's laid out_.** The LLM never emits raw UI. Per step:
- **teaching** → the AI generates narration + which asset to foreground, from `purpose` + `assets`;
- **challenge** → the AI generates guiding questions from the step's `assets` — the **same `AnchorGenerator`/anchored mechanic built in Slice 3**.

Generation is **server-side (flagship model), `Collect`+parse (the eval/anchor pattern), cached per step, with the step's `authored_content` as a deterministic fallback** so every step always renders (graceful degradation, exactly like eval and the anchor generator).

## Data model

### Contract (`packages/contracts/src/course.ts`)
```
CourseStepKind    = 'teaching' | 'challenge'
CourseAsset       = { id, kind: 'image'|'text'|'link', title, value }   // value = url/text
CourseStep        = { id, course_id, ordinal, kind, purpose, assets: CourseAsset[], challenge_type: string|null, authored_content: unknown }
CourseSummary     = { id, branch, title, blurb, tasks_count, tools_count, time_label, step_count }
Course            = CourseSummary & { steps: CourseStep[] }   // steps carry metadata, NOT generated content
CourseProgress    = { course_id, current_ordinal, completed_ordinals: number[], updated_at }
RenderedStep      = { ordinal, kind, template: 'teaching'|'challenge', content: <template-specific JSON>, source: 'generated'|'authored' }
```
- **teaching `content`**: `{ title, subtitle (narration), body: string[], foreground_asset_id: string|null }`.
- **challenge `content`**: `{ title, prompt, anchors: Anchor[] (reusing the Slice-3 Anchor shape, author:'ai'), reason_hint }` — the anchored questions on the step's material asset.

### Go / Postgres (migration `0011_courses.sql`)
- `course(id uuid pk, branch text, title text, blurb text, tasks_count int, tools_count int, time_label text, created_at)`.
- `course_step(id uuid pk, course_id uuid fk→course on delete cascade, ordinal int, kind text check(teaching,challenge), purpose text, assets jsonb default '[]', challenge_type text, authored_content jsonb not null default '{}', unique(course_id, ordinal))`.
- `course_progress(id uuid pk, user_id uuid fk→users, course_id uuid fk→course, current_ordinal int default 0, completed_ordinals int[] default '{}', updated_at, unique(user_id, course_id))`.
- `course_step_render(course_step_id uuid pk fk→course_step, content jsonb, source text, created_at)` — the per-step generation cache (content is not per-student in this MVP).
- Seed migration: one real course ("一条网络信息，该不该信") with a few steps (teaching + ≥1 challenge), each carrying purpose + assets + a solid `authored_content` fallback.

## API (Go, `internal/api`, RequireUser)
- `GET /api/v1/courses` → `{ courses: CourseSummary[] }`.
- `GET /api/v1/courses/{id}` → `{ course: Course }` (steps metadata; ownership n/a — courses are shared catalog, but require a session).
- `GET /api/v1/courses/{id}/progress` / `PUT .../progress` → per-user `CourseProgress` (upsert current_ordinal/completed).
- `POST /api/v1/courses/{id}/steps/{ordinal}/render` → `{ rendered: RenderedStep }`. Reads cache; on miss, generates (flagship `Collect`+parse for teaching; `AnchorGenerator` for challenge — the step's material asset becomes the `Material`), writes cache, returns. On generation failure → returns the `authored_content` as `source:'authored'`. Token-spending endpoint → guarded by `HasEntitlement` like `turn`/`evaluate`.

## Generation runtime (`internal/agent/course.go` or `internal/coursegen`)
- `RenderStep(ctx, step, provider, resolver, anchorGen) RenderedStep`:
  - `teaching` → build a prompt from `purpose`+`assets` → `Collect` → parse `{subtitle, body[], foreground_asset_id}` (fence-strip + validate) → on error, `authored_content`.
  - `challenge` → turn the step's primary text asset into a `materialize`-style `Material` (segment into blocks) → `anchorGen.Generate(pseudoSpec, materials)` where the "dimensions" come from `challenge_type`/`purpose` → wrap as challenge `content` → on error/empty, `authored_content`.
- Reuses `gateway.Collect`, the fence-strip/parse idiom, and the `AnchorGenerator` from Slice 3. Flagship resolver (course teaching quality matters); documented.

## Frontend (`apps/web`)
- **Courses grid** (`shell/courses/CoursesView.tsx`, from Slice 1): read real data via `api.listCourses()`, replacing `MOCK_COURSES`; progress from `api.getCourseProgress`.
- **Course player** (`shell/courses/CoursePlayer.tsx` + templates):
  - header + stage/step **stepper**; prev/next/finish; the "问印记" **ask-panel** (text input + context chip; the voice button is rendered **disabled/hidden** until 4-voice).
  - per step: `GET`/`POST render` → dispatch on `template`: `TeachingTemplate` (subtitle + body + foreground asset) or `ChallengeTemplate` (prompt + the anchored questions using the **Slice-3 `AnnotationBranch`-style rendering** — reuse where practical — + reason + feedback). Progress advances via `PUT progress`.
- **Course report** (`shell/courses/CourseReport.tsx`): the design's completion summary (stats, learned, challenge review, tools collected, notes, actions) — data assembled from progress + the course's steps.
- Routing: `课程` tab → grid → player → report (in `StudentApp`/a courses container).

## Decomposition (sub-slices, each spec-referenced plan → TDD → review, stacked & kept)
- **4a** — backend course model (migration + queries + sqlc + contract + DTO) + list/get/progress handlers + seed one course + frontend Courses grid reads real data (replace mock). No generation yet (render returns authored_content).
- **4b** — the render generation runtime (teaching Collect+parse; challenge via AnchorGenerator; cache + authored fallback) + the render endpoint.
- **4c** — course player (templates + stepper + ask-panel + nav + progress wiring).
- **4d** — course report + finish wiring.

## Invariants preserved
Client never calls the model (render is server-side); token-spending endpoints gated by `HasEntitlement`; keys server-side; graceful fallback so a step always renders; the Slice-3 anchored engine is reused, not reimplemented. Not a chat app; no addictive mechanics.

## Testing
- Contracts: `course.test.ts`.
- Go 4a: testcontainers round-trip (course/step/progress CRUD, seed present); handler tests (list/get/progress, ownership of progress).
- Go 4b: stub-provider tests for teaching generation (canned JSON → parsed content), challenge generation (reuses AnchorGenerator, canned anchors), cache hit/miss, authored fallback on generation failure.
- Web 4c: player renders a teaching step (mocked render), a challenge step (anchored questions), stepper advance, ask-panel; 4d report renders from mocked data.

## Out of scope (deferred)
Voice (TTS/STT) → "4-voice"; per-student challenge personalization (cache is per-step); dynamic course authoring UI (teacher-side); multi-course catalog beyond the seed; the deeper "challenges === work-portal tool cards" unification (course challenges reuse the anchored *generation*, rendered in a course template).

## Risks / decisions recorded
- **AI-generated content within fixed templates** (not raw AI layout): chosen for reliability/safety; the LLM emits structured JSON parsed into templates, never UI.
- **Per-step cache (not per-student)**: bounds cost/latency; teaching content is student-agnostic; challenge anchors are cached per step for the MVP (personalization deferred).
- **Authored fallback is mandatory** on every step so generation failure never breaks a course.
- Non-determinism of generated teaching content is acceptable (like eval); the authored fallback and caching bound the variance.
