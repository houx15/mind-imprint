# Interactive Course Runtime & Agent Architecture (pro edition)

> **Scope.** The pro edition's 课程 (Courses) surface: how an interactive course is
> authored, validated, played, recorded, and narrated. Covers the three workspace
> packages that make up the runtime, the four AI seams that touch a course at
> runtime and at publish time, and the authoring/publish lifecycle behind them.
>
> **Audience.** Engineers, and anyone who needs to explain *why* this is a real
> platform capability rather than a set of hard-coded lesson pages.
>
> **Source of truth.** The Zod contract in `packages/course-contract/src/*`. When
> this document and the schema disagree, the schema wins. Companion documents:
> `docs/2026-08-16-course-renderer-and-contract-guide.md` (integration guide for
> the course generator) and
> `docs/2026-08-15-student-course-runtime-data-and-renderer-design.md` (deep spec).

---

## 0. The one-sentence claim

**A course is a document, not code.** Everything a student experiences — the
layout of the screen, when a block appears, what has to be answered before the
next thing unlocks, when the narration plays, what counts as evidence of
learning — is declared in one validated JSON document (`CourseDefinition 2.0`).
A single generic runtime plays any document that passes the validator. Shipping
a new course involves zero renderer changes.

This is the same architectural bet the tool cards make on the writing side
(`新增卡 = 新增一份 JSON 配置，不改渲染器代码`), applied to a whole course.

---

## 1. Where the code lives

| Layer | Package / path | What it is |
|---|---|---|
| Contract | `packages/course-contract/src/` | Pure Zod schemas + four-layer validator. No React, no I/O. |
| Engine | `packages/course-runtime/src/` | Pure interpreters: Workflow state machine, event bus, session-state reducers, host adapter seams. No React, no wall clock, no ids. |
| Renderer | `packages/course-renderer/src/` | React block renderers, layout, focus manager, narration + media engines, `SlicePlayer` / `CoursePlayer`. |
| Backend | `apps/api/internal/api/course_*.go` | Definition storage, session get-or-create + snapshot save, asset URL signing, scene generation, admin authoring/publish. |
| Backend agents | `apps/api/internal/agent/course_*.go` | Scene generator, in-course coach, publish-time TTS pre-generation. |
| Frontend host | `apps/web/src/shell/courses/`, `apps/web/src/course/` | Catalog, player host, API-backed adapters, asset resolver. |

The three packages are source-only pnpm workspaces (`"exports": { ".": "./src/index.ts" }`) —
consumers import TypeScript directly, so there is no build step between the
contract and the runtime that could drift.

---

## 2. The document model

```
CourseDefinition
├── id, title, language (BCP-47), estimatedMinutes
├── objectives[]        — each objective names the blocks that EVIDENCE it
├── opening             — learningPreview + personalization allowlist + static fallback
├── parts[]
│   └── slices[]
│       ├── blocks[]      — the content
│       ├── layout        — where the blocks sit on the screen
│       ├── narrations[]  — voiced narration, text + pre-generated audio path
│       ├── workflow      — the state machine that drives the slice
│       └── navigation    — what the student may do (next / previous / skip)
└── closing             — preparedSummary + takeaways + transferApplications + fallback
```

Every id type (`courseId`, `partId`, `sliceId`, `blockId`, `narrationId`,
`workflowStepId`) has its own schema, and every object is `.strict()` — an
unknown key is a validation error, never a silently ignored field.

### 2.1 Objectives carry their own evidence

`CourseObjective.evidenceBlockIds` is required and non-empty. An objective must
name the blocks whose student interaction demonstrates it. The quality validator
then WARNs when an objective points at a read-only block (`text` / `images` /
`pdf`) that produces no observable evidence. This is what lets the closing
narration honestly say "you did X" instead of "you finished the course".

### 2.2 The block library (8 types)

| Block | Purpose | Notes |
|---|---|---|
| `text` | Plain authored copy | |
| `richText` | A scrollable card of authored static HTML+CSS | Rendered in a **scriptless, sandboxed `srcdoc` iframe**; the quality validator rejects off-page `src`/`href`/`url(http…)` references, so a card cannot phone home |
| `images` | One / side-by-side / gallery | Lightbox |
| `pdf` | A source document | Emits `pdf.opened`, `pdf.pageChanged` |
| `video` | Media with an optional cue timeline | See §2.5 |
| `interactiveHtml` | A sandboxed interactive frame | Versioned postMessage protocol, §2.6 |
| `fillBlank` | Assessment — `graded` (accepted answers, case sensitivity) or `reflection` (rubric) | |
| `singleChoice` | Assessment — `graded` (correct option + per-outcome feedback) or `survey` | |

Two orthogonal knobs apply to **every** block:

- **`openAs: "inline" | "modal"`.** A slice is one desktop screen. The moment
  three resources share it, a `grid` gives each a quarter of the screen and a PDF
  page at quarter size is unreadable. `modal` renders a compact launcher button in
  the slot and opens the block full-size on demand. It is a presentation choice
  only — events, completion rules and recorded payloads are identical, so a
  Workflow gating on the block does not change. Assessment blocks behave slightly
  differently by design: a modal assessment opens itself the first time the
  Workflow enables it, closes on completion, and returns read-only (a remounted
  assessment would have lost its `locked` flag and could emit a second
  `block.completed`).
- **Completion rules** on assessments: `submit-any`, `submit-correct`, or
  `submit-correct-or-exhausted` with `maxAttempts`. Video adds `video-ended` and
  `video-ended-and-interactions-completed`.

### 2.3 Layout

Four presets — `full`, `split-horizontal`, `split-vertical`, `grid` — with slot
names enforced by the schema (`main`; `left`/`right`; `top`/`bottom`;
`cell-1..cell-N`, 2–4, in order). Splits carry a bounded ratio set
(`1:1 3:2 2:3 2:1 1:2 3:1 1:3`) so no track is ever thinner than a quarter of the
screen. Layout is authored per slice, so a course can breathe: a full-bleed
reading slice, a split with the source on the left and the question on the right,
a four-cell comparison grid.

### 2.4 The Slice Workflow — a declarative state machine

This is the heart of "interactive". Each slice carries a `SliceWorkflow`:

- **`initialState`** — which blocks start visible / enabled, what starts focused.
- **`steps[]`** — each step has `enterActions[]` and `transitions[]`.
- **16 actions:** `show`, `hide`, `focus`, `clearFocus`, `enable`, `disable`,
  `playNarration`, `pauseNarration`, `stopNarration`, `playBlock`, `pauseBlock`,
  `resetBlock`, `startTimer`, `cancelTimer`, `completeSlice`, `navigate`.
- **16 event types:** `narration.ended`, `video.started|paused|ended`,
  `video.interaction.shown|completed`, `pdf.opened`, `pdf.pageChanged`,
  `interaction.completed`, `answer.submitted|correct|incorrect|attemptsExhausted`,
  `block.completed`, `student.continue`, `timer.elapsed`.

An author writes pedagogy directly: *play the narration; when it ends, reveal the
source; when the student opens page 3, enable the question; when the answer is
correct, complete the slice.* No renderer code is written for any of that.

**The interpreter is pure.** `WorkflowRuntime.send(event)` returns an ordered
list of effects; it never applies them. It has no timers, no async, no wall
clock. The same ordered event stream always yields the same effect log and the
same final step — which means a session is replayable and a slice is unit-testable
without a DOM.

### 2.5 The Video Interaction Contract

A declarative cue timeline hung on one owning video block — deliberately **not** a
second general workflow engine. Cue activities reuse the *same* assessment
sub-schemas and completion rules as standalone assessment blocks, so cue grading
is literally the same engine. Cross-document rules Zod cannot express on one
object (cue-id uniqueness, strictly increasing times, in-duration bounds,
blockId/source agreement with the owning video) live in the referential validator.

### 2.6 The `interactiveHtml` sandbox protocol

An authored interaction runs in a sandboxed frame and talks to the host over a
**versioned envelope** (`protocol` / `version` / `sessionToken` / `type`) validated
by the renderer, plus a **per-message-type payload schema** validated by the
contract. Four message types: `ready`, `progress`, `error`, `completed`.

The load-bearing rule is on `completed`: it must carry at least one learning-evidence
field (`correct` and/or `value`). A well-formed envelope carrying nothing is
rejected before it can complete a block or land in `interactionResult`. The frame
may supply a `resultId` for resend detection (network jitter, a frame
re-dispatching on focus), but the host **never** trusts it as the block's identity —
that is always the block id, stamped host-side. Host-owned fields (event id,
occurrence time) are stamped by the host bus and have no place in a frame-authored
payload.

---

## 3. Four-layer validation

`validateCourseDefinition(input)` is the single entry point, and it runs four
layers in order:

1. **Structural (Zod, `.strict()`).** Wrong shape stops here — later layers assume
   a well-formed document.
2. **Referential.** Every id an action, transition, layout slot, objective or cue
   points at must exist and be of the right kind. Cross-field rules Zod cannot
   express on one object live here (a graded `singleChoice`'s `correctOptionId`
   must reference a real option; graded fill-blank is incompatible with
   `submit-any`; `video-ended-and-interactions-completed` requires an
   `interaction` reference).
3. **Workflow graph.** Per slice. Includes an **event-producer table** mapping each
   event type to the kind of thing that can actually emit it (`video.ended` → a
   video block; `narration.ended` → a narration id, not a block id;
   `timer.elapsed` → a timer the enclosing step started). A transition can only
   wait on an event its own slice can really produce — which structurally removes
   the single most common authoring bug, a slice that hangs forever waiting for an
   event nobody will send.
4. **Quality.** Deterministic authoring invariants, split into HARD (blocks
   publication) and WARN (surfaces without blocking). Examples: the 256-asset cap
   is HARD because it mirrors the server's per-course signing cap, so a course over
   it can never be fully signed; an objective evidenced only by read-only blocks is
   WARN; `estimatedMinutes` diverging more than 35% from the summed per-slice
   seconds is WARN; an external reference inside a `richText` card is caught here.

The server does **border validation only** (`schemaVersion == "2.0"`, `course.id ==
{slug}`, every `cardId` resolves in the registry, category in the closed
7-slug vocabulary). Deep structure is the generator's job against the Zod
contract, and is never re-implemented in Go — one validator, one truth.

---

## 4. The runtime engine

### 4.1 Determinism seams

The engine never reads the wall clock and never mints ids. The host injects an
`IdFactory` and a `Clock`, plus an `AssetResolver` that turns an authored relative
path into a resolvable URL (local preview, draft, production CDN). Tests and
replays therefore produce byte-identical output.

### 4.2 The event bus

`RuntimeEventBus` normalizes every runtime event into a `CourseRuntimeEvent`
envelope, stamping course/session/part/slice/source ids, an id and an
`occurredAt` from the injected factories. It has one hard behavior worth naming:
**it refuses to deliver an event belonging to any slice other than the active
one** (dropped events are reported to an `onDropped` hook). One slice can never
consume another slice's events — a real hazard when a slice unmounts while its
video is still settling.

### 4.3 Session state and evidence

`CourseSession` is the recorded artifact:

- `status`: `created → opening → in-progress → closing → completed`
- `current`: `{ partId, sliceId, workflowStepId }` — resume lands exactly where the
  student left, including mid-slice workflow position
- `sliceStates[sliceId]`: status, current workflow step, started/completed
  timestamps, `elapsedSeconds`, and per-block state
- `blockStates[blockId]`: `visible`, `enabled`, `completed`, `attempts`, `answer`,
  `mediaPositionSeconds`, `interactionResult`
- `events[]`: the full append-only runtime event stream
- `opening` / `closing`: the generated `RuntimeSceneResult`
- `courseDefinitionHash`: see below

That is the evidence base the closing narration and the course report read from,
and it is why 过程即数据 holds on this surface too: attempts, exhausted attempts,
media positions and dropped-through interactions are all recorded, not just final
answers.

### 4.4 The staleness policy

`courseDefinitionHash` is the sha256 of the stored definition bytes, computed once
server-side. `isCourseSessionStale` compares it to the current hash. A session
with **no** recorded hash is never stale (back-compat resume-as-normal); only a
*present* hash that actively disagrees means the session's slice/step/block state
may reference ids the definition no longer has. On a stale reset the player both
stamps the new hash and calls `resetProgress` so the discarded progress cannot be
resurrected by the next `load()`.

---

## 5. The four AI seams

All four are server-side. The client holds no key and never calls a model.

### 5.1 Opening / Closing scene narration — `GenerateCourseScene`

The most interesting restraint in the course surface.

**Input** is teacher-approved facts (title, estimated minutes, objectives, learning
preview, prepared summary, takeaways, transfer applications) **plus** a closed
allowlist of student-history signals the author enabled for that course:

- opening: `recent-course-topics`, `prior-objective-performance`
- closing: `answers`, `attempts`, `time-on-slice`, `interaction-results`

Signal minimization is structural: the generator receives only what the caller was
permitted to pass, and `usedSignalTypes` is derived **deterministically** — an
allowed signal counts as used exactly when it carries non-empty evidence, which is
also the only condition under which it reaches the prompt. The report of what
influenced the narration is therefore honest regardless of the model's phrasing.

**Closing rules** encoded in the system prompt: stay consistent with the prepared
summary; mention only evidence that actually exists; **distinguish completing the
course from mastering the ability**; make no unsupported improvement claims
("you've improved" is forbidden without evidence); name the takeaways and the
transfer applications. **Opening rules**: greet, say what the student will do, give
the time estimate, connect to prior learning *only* when a signal was actually
supplied (empty signals → do not mention the past and do not hint at it), close
with an invitation.

**Degradation is total.** LLM failure → the author's static fallback text; TTS or
OSS failure → text-only. The endpoint returns 200 in every case and is stateless
and safe to call repeatedly (audio is content-hash idempotent). The player never
stalls on a 500.

### 5.2 The in-course coach — `ProposeCourseAskReply`

A free-question bar, scoped to the **current step**. Single-shot: no history, no
scripted turn, no `{"type":"reply"|"advance"}` envelope. The prompt names the
course, the step and the course goal, carries the step's authored text as grounding
context (explicitly: *use it to answer in context, do not recite it back*), and
states four hard rules: never hand over this step's quiz answer (only help the
student judge it); never conclude or write for the student; one question at a
time; keep it short.

The reply then passes the same **enforcement subset** the writing-side coach does —
`ValidateOutput` (typed output shape) and the versioned `BannedPhrasing` corpus.
Teaching *is* the point on this surface, so the coach may explain and demonstrate;
what it may not do is produce the student's assessed deliverable.

### 5.3 Publish-time narration TTS

`GenerateDefinitionAudio` walks a stored 2.0 definition's narrations plus the
opening/closing fallbacks, synthesizes one mp3 per authored audio path and uploads
it to OSS **at exactly the path the document references**. Because a 2.0 definition
names its own audio paths, this is a straight synth-and-put — no hashed keys, no
manifest. (The legacy render-cache courses use `GenerateCourseAudio`, which derives
hashed object keys and returns a manifest, idempotently skipping objects that
already exist.)

At play time the renderer runs **one shared `HTMLAudioElement` per course** behind
an audio arbiter, so narration and video can never talk over each other, and a stop
is a real stop.

### 5.4 The course generator

Outside the runtime. It is a separate authoring tool whose entire contract is:
*emit a `CourseDefinition 2.0` document that passes the validator.* Everything in
§2–§4 is what it has to satisfy; nothing about the renderer is its concern.
`apps/api/internal/store/seed/courses/coverage-course.json` is the golden coverage
course that exercises every block type, layout, narration, workflow, and
opening/closing.

---

## 6. Authoring & publish lifecycle

| Step | Endpoint | Behavior |
|---|---|---|
| Upsert | `PUT /admin/courses/{slug}/definition` | Admin-key bearer. Border validation only. A new course lands `status='preview'`; re-PUT preserves existing status. ⚠️ The body **overwrites** `blurb` / `cardIds` / `category` / `introduction` — GET first, merge, then PUT. |
| Ship | `POST /admin/courses/{slug}/ship` | Generate narration TTS from the stored definition, bind the cover, flip `preview → published` in **one** `SetCourseStatusAndCover` call so a course is never published-without-a-cover mid-request. |
| Unpublish | `POST /admin/courses/{slug}/unpublish` | The reverse transition. |
| Play | `GET /courses/{slug}/definition` → `{ definition, hash }` | 404 for a legacy course; the frontend branches on that to pick the legacy player. |
| Session | `POST /courses/{slug}/session` (get-or-create), `PUT` (snapshot save) | The server mints identity/authorship from the authed user; the client never dictates them. |
| Assets | `POST /courses/{slug}/asset-urls` | Signs a batch of relative paths into cacheable CDN read URLs. The server maps each to `courses/<slug>/<path>` — a caller can only obtain URLs inside its own course's namespace, and the server never parses block internals. Same endpoint is the refresh endpoint when a session outlasts the signing window. Cap: 256 paths, mirroring the contract's HARD quality check. |
| Scene | `POST /courses/{slug}/scene` | §5.1. Stateless, always 200. |

**Catalog metadata** is schema-driven too: a closed 7-slug category vocabulary
(`stance-value` / `source-check` / `media-literacy` / `self-knowledge` /
`data-literacy` / `research-process` / `argument-writing`) stored as slug+label
pairs so renaming a label never migrates data, plus a `CourseIntroduction`
(hook / whatYouDo / takeaways / curriculum alignment across IB, other
international and domestic tracks / keywords) rendered deterministically on the
detail page. Per-student catalog progress is resolved server-side so the catalog
is one round trip.

**Reporting.** `GET /courses/{slug}/report` returns completed step titles, the tool
cards the course teaches, seconds spent and quiz totals — computed from the
`course_session` + definition for 2.0 courses. `GET /report/answers?attempt=<id>`
lazily returns the per-attempt answer detail read back from the stored session
blob, keeping the main report fetch lean. Attempts are historied, so a restart
does not erase the prior walk.

---

## 7. Why this is a technical shining point

1. **The interaction layer is authored, not programmed.** A slice's behavior is a
   16-action / 16-event state machine in JSON. Pedagogical sequencing — reveal,
   focus, gate, narrate, time-box, complete — is expressible by a course author
   with no renderer change and no deploy of new component code.
2. **The interpreter is pure and replayable.** No wall clock, no id minting, no
   async inside the engine; effects are returned as data for the host to perform.
   A course session can be replayed deterministically from its event stream.
3. **Four validation layers, and one of them knows physics.** The event-producer
   table means the validator can prove a transition waits on an event the slice can
   actually emit. That converts the classic "the course is stuck" support ticket
   into an authoring-time error.
4. **Untrusted content is structurally contained.** Authored HTML runs in a
   scriptless sandboxed `srcdoc` iframe with external references rejected at
   validation; interactive frames talk over a versioned envelope with per-type
   payload schemas, and the host stamps identity rather than trusting the frame.
5. **AI on this surface is bounded by construction, not by tone.** The scene
   generator sees only teacher-approved facts plus an explicit signal allowlist,
   reports what it used deterministically, and is required to distinguish
   completion from mastery. The coach is single-shot, step-scoped, and passes the
   same output enforcement as the writing coach. Both degrade to authored content
   rather than to plausible invention.
6. **Evidence, not completion ticks.** Objectives declare the blocks that evidence
   them; block state records attempts, answers, media positions and interaction
   results. That is what the closing narration, the course report and the 图鉴
   proficiency model read.
7. **The same document drives the whole chain.** Definition → validation → TTS
   pre-generation → asset signing → playback → session evidence → report. One
   artifact, no second copy that can drift.
