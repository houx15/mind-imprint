# Course Renderer & Contract — Integration Guide (for the Course Generator)

> **Purpose.** The student platform ships a **clean, reusable course runtime** (three
> workspace packages) that turns a `CourseDefinition 2.0` document into an interactive,
> played course. The **course generator** (a separate tool/skill) only has to **emit a
> `CourseDefinition 2.0` document that passes the contract's validator** — then this
> renderer plays it, unchanged. This doc says **where everything lives**, **what the
> generator must produce**, and **how to reuse the packages**.
>
> **Authoritative references (read alongside this):**
> - Deep data-model + rationale spec: `docs/2026-08-15-student-course-runtime-data-and-renderer-design.md` (§ references below point here).
> - The **single source of truth is the Zod contract** in `packages/course-contract/src/*` — this doc is a map, not a second copy. When prose here and the Zod schema disagree, the Zod wins.
> - A complete, valid, worked example: `apps/api/internal/store/seed/courses/coverage-course.json` (the "golden coverage course" — exercises every block type, layout, narration, workflow, and opening/closing).

---

## 1. Where it lives

Three source-only pnpm workspace packages (`"exports": { ".": "./src/index.ts" }` — **no build step**; consumers import the TypeScript source directly):

| Package | Path | Role |
|---|---|---|
| `@mind-imprint/course-contract` | `packages/course-contract/` | **The contract.** Zod schemas + types for the whole data model, the asset-path collector, and the **`validateCourseDefinition` gate**. Zero React, zero I/O. **This is the package the generator depends on.** |
| `@mind-imprint/course-runtime` | `packages/course-runtime/` | **Pure runtime core.** The workflow interpreter (deterministic state machine), session-state reducer, typed event bus, and the host **adapter interfaces**. Zero React, zero I/O — real time/ids/network are injected. |
| `@mind-imprint/course-renderer` | `packages/course-renderer/` | **React renderer.** `CoursePlayer` + block renderers + layout/narration/scenes. Consumes a document, drives the runtime, paints the DOM. |

Consumed by the student app at the **host boundary** `apps/web/src/shell/courses/RuntimeCoursePlayer.tsx` (where real `Date`/`crypto`/network enter) and the server surfaces under `apps/api/internal/api/course_definition.go` / `course_session.go` / `course_scene.go` / `course_asset_urls.go`.

### Package source layout (what each file owns)

```
packages/course-contract/src/
  primitives.ts     # id shape (lower-case-hyphenated) + relativeAssetPathSchema
  blocks.ts         # the 7 block types (discriminated union on `type`)
  layout.ts         # LayoutDefinition (full / split-* / grid + slots)
  narration.ts      # NarrationDefinition (text + audio asset)
  workflow.ts       # SliceWorkflow — the per-slice state machine (actions/events/transitions)
  navigation.ts     # NavigationDefinition (how a slice advances)
  course.ts         # CourseObjective, Opening/Closing, Slice, Part, CourseDefinition,
                    #   and CourseDefinitionDocument = { schemaVersion:"2.0", course }
  session.ts        # CourseSession (runtime/progress state — NOT authored by the generator)
  videoInteraction.ts # VideoInteractionDocument (a SEPARATE 1.1 doc for video cue timelines)
  assets.ts         # collectAssetPaths(document) -> string[]
  validate/         # referential.ts + workflow.ts + index.ts (validateCourseDefinition)

packages/course-runtime/src/
  adapters.ts       # AssetResolver, SessionAdapter, RuntimeSceneGenerator, CourseRuntimeAdapters, InMemorySessionAdapter
  eventBus.ts       # RuntimeEventBus, SliceEmitter, Clock, IdFactory
  workflowRuntime.ts# WorkflowRuntime — pure, replayable interpreter of a SliceWorkflow
  sessionState.ts   # initSliceState / applyEffect / applyEvent reducer

packages/course-renderer/src/
  course/CoursePlayer.tsx      # top-level: validate -> Opening -> Part/Slice walk -> Closing
  slice/SlicePlayer.tsx        # one slice: wires workflow runtime <-> layout <-> blocks
  layout/LayoutRenderer.tsx    # 4 layout presets -> slot boxes
  blocks/registry.ts + types.ts# BlockType -> renderer map; BlockRendererProps
  blocks/*                     # Text, Images, Pdf, Video(+interaction), HtmlInteraction, FillBlank, SingleChoice
  narration/*                  # NarrationPlayer + injectable AudioEngine
  focus/FocusManager.tsx       # focus targeting
  scenes/*                     # OpeningScene / ClosingScene
```

---

## 2. The pipeline (how the generator's JSON becomes a played course)

```
generator ──emits──▶ CourseDefinitionDocument (JSON, schemaVersion "2.0")
                         │
                         ▼
        validateCourseDefinition(doc)   ◀── the ONE gate. Structural (Zod) → referential → per-slice workflow-graph.
                         │  ok:true
                         ▼
   CoursePlayer(document, adapters, studentId, idFactory, clock)
                         │
        Opening scene → for each Part → for each Slice:
                         │      SlicePlayer runs the slice's WorkflowRuntime:
                         │        initialState → enterActions → wait for events → transitions → completeSlice
                         │      LayoutRenderer places blocks in slots; block renderers emit events back
                         ▼
                    Closing scene → session status = "completed"
```

**The generator's entire job is the first box.** If `validateCourseDefinition` returns `ok:true`, the document will play. Everything below the gate is the platform's responsibility.

---

## 3. What the generator must emit

A **`CourseDefinitionDocument`**:

```jsonc
{
  "schemaVersion": "2.0",
  "course": {
    "id": "evidence-comparability",         // lower-case-hyphenated id (primitives.ts idSchema)
    "title": "…",
    "language": "en",                         // BCP-47
    "estimatedMinutes": 12,
    "objectives": [ { "id":"o1", "text":"…", "evidenceBlockIds":["b-…"] } ],
    "opening":  { /* OpeningDefinition: learningPreview[], personalization, fallback{text,audio?} */ },
    "parts": [
      {
        "id": "p1", "title": "…", "objectiveIds": ["o1"],
        "slices": [
          {
            "id": "s1", "title": "…", "objectiveIds": ["o1"], "estimatedSeconds": 90,
            "blocks":     [ /* 1+ BlockDefinition — see §4 */ ],
            "layout":     { /* LayoutDefinition — assigns each block to exactly one slot */ },
            "narrations": [ /* NarrationDefinition[] — text + audio asset */ ],
            "workflow":   { /* SliceWorkflow — the state machine, see §5 */ },
            "navigation": { /* NavigationDefinition */ }
          }
        ]
      }
    ],
    "closing":  { /* ClosingDefinition: preparedSummary, takeaways[], transferApplications[], personalization, fallback */ }
  }
}
```

Exact shapes are the Zod schemas in `packages/course-contract/src/course.ts` (and the files it imports). **Do not hand-transcribe them into the generator — import and validate (see §7).**

### The three data layers (design §4)
- **CourseDefinition** (what the generator emits) — the authored, portable, replayable document. Relative asset paths only; no URLs, no runtime state.
- **CourseSession** (`session.ts`) — per-student progress/runtime state. **The platform owns this; the generator never emits it.**
- **RuntimeScene** (opening/closing personalized text+audio) — produced at play time by a `RuntimeSceneGenerator`. The document carries only the *fallback* text/audio.

---

## 4. Block types (design §9)

`BlockDefinition` is a **discriminated union on `type`** (`blocks.ts`). The seven members:

| `type` | Key fields | Notes |
|---|---|---|
| `text` | `content` (string) | Markdown-ish text. No asset. |
| `images` | `presentation` (`single`/`side-by-side`/`gallery`), `items[]` each `{id, source, alt, caption?}` | `source` = relative asset path. |
| `pdf` | `title`, `source`, `initialPage?` | `source` = relative pdf path. |
| `video` | `source`, `poster?`, `captions?`, `durationSeconds?`, `interaction?{source}`, `completion?` | `interaction.source` points to a **VideoInteractionDocument** (`videoInteraction.ts`, its own `schemaVersion "1.1"`). |
| `interactiveHtml` | `source`, `protocolVersion:"1.0"`, `aspectRatio`(`1:1`/`4:3`), `completion?` | Sandboxed iframe (`allow-scripts` only) talking a fixed postMessage protocol (`blocks/html/protocol.ts`). |
| `fillBlank` | `prompt`, `placeholder?`, `assessment`, `completion` | `assessment`: `graded`(acceptedAnswers) or `reflection`(rubric). |
| `singleChoice` | `prompt`, `options[]`, `assessment`, `completion` | `assessment`: `graded`(correctOptionId) or `survey`. |

**Adding a block type is out of scope for the generator** — it needs a new renderer + registry entry in `course-renderer`. Generate only the seven above.

**Every relative asset field** (`images[].source`, `pdf.source`, `video.source/poster/captions/interaction.source`, `interactiveHtml.source`, narration `audio`, opening/closing `fallback.audio`) must satisfy `relativeAssetPathSchema` (`primitives.ts`): **no leading `/`, no `..` segment, no URL scheme, no backslash.** `collectAssetPaths(document)` returns exactly this set — the platform signs each to a CDN URL at play time (see §6).

---

## 5. The slice workflow (design §12–§13) — the part to get right

Each slice carries a **deterministic state machine** (`SliceWorkflow`, `workflow.ts`):

```jsonc
"workflow": {
  "version": "1.0",
  "initialState": { "visibleBlockIds": ["…"], "enabledBlockIds": ["…"], "focusedTarget": {"blockId":"…"} },
  "initialStepId": "intro",
  "steps": [
    {
      "id": "intro",
      "enterActions": [ { "type":"show", "targetId":"b-video" }, { "type":"playNarration", "narrationId":"n1" } ],
      "transitions": [ { "on": { "type":"video.ended", "sourceId":"b-video" }, "to": "ask" } ]
    },
    { "id": "ask", "enterActions": [ … ], "transitions": [ { "on": {"type":"answer.correct"}, "to":"done" } ] },
    { "id": "done", "enterActions": [ { "type":"completeSlice" } ], "transitions": [] }
  ]
}
```

- **Actions** (`WorkflowAction`, 16 kinds): `show`/`hide`/`enable`/`disable`/`focus`/`clearFocus`, `playNarration`/`pause`/`stop`, `playBlock`/`pauseBlock`/`resetBlock`, `startTimer`/`cancelTimer`, `completeSlice`, `navigate:nextSlice`.
- **Events** (`WorkflowEventType`, what a transition fires on): narration/video/pdf/interaction lifecycle, `answer.submitted/correct/incorrect/attemptsExhausted`, `block.completed`, `student.continue`, `timer.elapsed`. A matcher may pin `sourceId`/`interactionId`/`timerId`.
- The interpreter (`course-runtime/workflowRuntime.ts`) **arms transitions before running enterActions**, is pure, and is replayable from the event log.

**Referential rules the validator enforces** (`validate/referential.ts` + `validate/workflow.ts`) — the generator MUST satisfy all:
- Every `objectives[].id`, `parts[].id`, slice `id`, block `id`, narration `id` unique in its scope; block ids globally unique.
- Every `objectiveIds`/`evidenceBlockIds` reference resolves.
- **Every block is assigned to exactly one layout slot; every slot references a real block.**
- `singleChoice` graded `correctOptionId` is one of its options; graded `fillBlank` is incompatible with completion `submit-any`; a `video` whose completion is `video-ended-and-interactions-completed` must have an `interaction`.
- Workflow: `initialStepId` and every transition `to` resolve to a real step; the graph is well-formed (see `validate/workflow.ts`).

---

## 6. Assets & serving (this session's work — design §4/§20)

The document carries **relative asset paths only**; it never contains a URL. At play time:
- A course's asset `assets/videos/case.mp4` maps to the OSS object key **`courses/<course.id>/assets/videos/case.mp4`**.
- The web host collects paths (`collectAssetPaths`), calls `POST /courses/{slug}/asset-urls`, and the server signs each into a **cacheable Aliyun CDN URL鉴权 link** (`?auth_key=…`). The renderer's `AssetResolver` is then a pure lookup over that `{ relativePath → signedUrl }` map.
- **Generator implication:** name assets with stable relative paths and **upload the bytes to `courses/<course.id>/<relativePath>` in OSS** (`course_material` scope). The document stays portable; the platform handles signing/caching/access-control.

Full rationale: `docs/2026-08-16-oss-cdn-url-auth-design.md`.

---

## 7. How to reuse the renderer/contract in the generator

The generator lives outside this repo/package set, but the **contract package is the shared truth**. Two supported ways to consume it:

**A. Depend on the workspace package (preferred if the generator is in this monorepo):**
```jsonc
// generator package.json
"dependencies": { "@mind-imprint/course-contract": "workspace:*" }
```
```ts
import { validateCourseDefinition, collectAssetPaths, type CourseDefinitionDocument } from "@mind-imprint/course-contract";

const result = validateCourseDefinition(generated);   // the ONE gate
if (!result.ok) throw new Error(result.issues.map(i => `${i.path}: ${i.message}`).join("\n"));
const assets = collectAssetPaths(generated);           // paths you must upload to courses/<id>/…
```

**B. If the generator is a separate project:** copy the `packages/course-contract` source (it's dependency-light: Zod only) or publish it. **Always validate the generated document with `validateCourseDefinition` before emitting** — it is the same gate `CoursePlayer` runs, so passing it guarantees the renderer will play the course.

**To render/preview** (build a local preview harness, or just to understand the target): mount `CoursePlayer` from `@mind-imprint/course-renderer` with:
```ts
import { CoursePlayer } from "@mind-imprint/course-renderer";
// props: { document, adapters: CourseRuntimeAdapters, studentId, idFactory, clock, sessionId?, onBusReady? }
```
`CourseRuntimeAdapters` = `{ assetResolver, sessionAdapter, openingGenerator, closingGenerator }` (`course-runtime/adapters.ts`). For a preview, `InMemorySessionAdapter`, a trivial `assetResolver` (`{ resolve: p => p }` or a base-URL join), and fallback scene generators are enough. `idFactory`/`clock` are where real `crypto.randomUUID()` / `new Date().toISOString()` enter — keep them out of generated data (the document must stay deterministic and time-free). See `apps/web/src/shell/courses/RuntimeCoursePlayer.tsx` for the production wiring.

---

## 8. Generator invariants checklist

Produce output that satisfies **all** of these (the validator enforces most; the rest are conventions):

- [ ] `schemaVersion` is exactly `"2.0"`; the top-level is `{ schemaVersion, course }`.
- [ ] All ids are **lower-case-hyphenated** (`^[a-z0-9]+(-[a-z0-9]+)*$`) and unique in scope; block ids globally unique.
- [ ] Every slice has **≥1 block**, a layout that assigns **each block to exactly one slot**, and a valid `workflow` whose steps/transitions all resolve.
- [ ] Only the **seven block types** in §4; assessment/completion cross-field rules (§5) hold.
- [ ] Every asset reference is a **relative path** passing `relativeAssetPathSchema`; the matching bytes are uploaded under `courses/<course.id>/<relativePath>`.
- [ ] The document is **deterministic and time-free** — no URLs, no timestamps, no random ids baked in; those enter only at the host boundary.
- [ ] `validateCourseDefinition(output).ok === true` **before** you emit. This is the contract.
- [ ] Cross-check against the golden example `apps/api/internal/store/seed/courses/coverage-course.json`.
```
