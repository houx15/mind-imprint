# Student Course Runtime 2.0 — Code Review and Gap Report

**Review date:** 2026-08-16  
**Reviewed repository:** `/Users/houyuxin/08Coding/mind-imprint`  
**Reviewed snapshot:** branch `course-authoring-lifecycle`, commit `7e4db373e198cfd4558f44c4b781d4deb8176a07`  
**Primary design references:**

- `docs/2026-08-15-student-course-runtime-data-and-renderer-design.md`
- `docs/2026-08-16-course-renderer-and-contract-guide.md`
- `apps/api/internal/store/seed/courses/coverage-course.json`

The repository had unrelated in-progress authoring-lifecycle changes during this review. This report does not review or modify those changes. It reviews the student Course Runtime 2.0 contract, runtime, renderer, production web host, and their current API boundaries.

## Executive verdict

The project has a sound architectural skeleton: a strict Zod contract, a deterministic workflow interpreter, a block registry, isolated media engines, a sandboxed HTML boundary, and useful unit tests. Those pieces should be retained.

The student runtime is **not ready to serve as the source of truth for teacher preview or published CourseDefinition 2.0 courses**. Several valid documents can pass `validateCourseDefinition` and still fail or deadlock in the production student player. This currently contradicts the integration guide's promise that a document passing the validator will play.

The release-blocking gaps are:

1. Production never loads video interaction documents, so required video cues do not appear and gated videos cannot complete.
2. The production host leaves the course as soon as session status becomes `completed`, so the generated Closing scene is normally unmounted before students see it.
3. Runtime events, workflow position, elapsed time, media results, and HTML learning data are not correctly persisted; existing sessions do not actually resume.
4. `NavigationDefinition` is not implemented. There are no previous/manual-next controls, revisit restoration, or a producer for `student.continue`.
5. The four layout presets exist only as bare CSS Grid tracks. There is no course CSS, no one-Slice/one-screen constraint, and the host explicitly permits page-level scrolling.
6. The default PDF engine always reports one page and has no zoom, error state, or usable viewport sizing.
7. The HTML boundary validates only the message envelope. It does not validate or persist learning-result payloads, cannot truly disable the iframe, does not stop media when hidden, and has no supported music-autoplay lifecycle.
8. There is no real-browser Course Runtime 2.0 journey covering layout, PDF, video cues, iframe messaging, media autoplay, resume, and Closing.

## Severity scale

- **P1 — release blocker:** a valid or intended course can fail, deadlock, lose learning data, skip a required experience, or violate a security boundary.
- **P2 — major gap:** behavior is materially incomplete, unreliable, inaccessible, or inconsistent with the approved design.
- **P3 — hardening:** lower-risk correctness, authoring quality, or maintainability issue.

## What is already implemented well

- Structural validation uses strict Zod schemas and rejects unknown fields.
- Referential validation checks major course, objective, slice, block, narration, layout-slot, and assessment references.
- Workflow validation covers step IDs, transition targets, reachability, terminal reachability, a first cycle check, and some action-target compatibility.
- `WorkflowRuntime` is deterministic and side-effect free.
- The event bus stamps course/session/part/slice provenance and prevents a different Slice from consuming events.
- All seven declared block types are registered.
- Standalone fill-blank and single-choice grading, attempt handling, feedback, and completion logic are implemented.
- The HTML iframe uses `sandbox="allow-scripts"` without `allow-same-origin`, and validates the source window, protocol, version, session token, and message type.
- The video cue controller correctly pauses, renders an assessment, resumes, and gates completion **when a parsed interaction document is injected**.
- CDN URL collection, signing-map resolution, refresh scheduling, and refresh retry exist.
- The code separates browser APIs behind audio, video, PDF, session, asset, and scene adapter seams.

## Detailed findings

### P1-01 — Production video interactions are not wired

**Evidence**

- `packages/course-renderer/src/blocks/media/VideoInteractionController.tsx:19-24` defines a synchronous `InteractionLoader` and context provider.
- `packages/course-renderer/src/blocks/media/VideoInteractionController.tsx:58-66` returns `null` when no loader is injected.
- `packages/course-renderer/src/blocks/media/VideoRenderer.tsx:108-116` always mounts the controller for an interaction reference, but does not supply `loadDocument`.
- `packages/course-renderer/src/index.ts:1-27` does not export `InteractionLoaderProvider` or an interaction loader API.
- `apps/web/src/shell/courses/RuntimeCoursePlayer.tsx:60-75` creates only asset, session, opening, and closing adapters. It never fetches or parses the interaction JSON.
- The renderer test injects the missing provider manually at `packages/course-renderer/test/media/videoInteraction.test.tsx:57-70`.
- The golden course uses `video-ended-and-interactions-completed` with an external interaction document at `apps/api/internal/store/seed/courses/coverage-course.json:48-62`.

**Impact**

In production, cues never appear, auto-pause never occurs, required cue completion never occurs, and a gated video can never emit `block.completed`. A valid golden course can therefore deadlock.

**Required change**

Create a production interaction-document adapter that fetches the signed JSON asset, parses it with `VideoInteractionDocument`, runs `validateVideoInteraction`, caches it by source, and exposes loading and typed error states. The current synchronous loader shape is not suitable for network loading and should become an async resource boundary or preloaded document map.

**Acceptance criteria**

- The real `RuntimeCoursePlayer`, without test-only providers, loads a referenced interaction JSON.
- A required cue auto-pauses at its authored timestamp, is visible to the learner, records its answer/result, resumes according to policy, and gates video completion.
- Missing, malformed, mismatched, or expired interaction assets produce a visible recoverable error instead of silent `null` or a deadlock.

### P1-02 — Video interactions are inline, not modal, and lose learning evidence

**Evidence**

- `packages/course-renderer/src/blocks/media/VideoInteractionController.tsx:130-135` renders a normal `div` with `role="group"`; there is no dialog, backdrop, portal, focus trap, or modal dismissal policy.
- `packages/course-renderer/src/blocks/media/VideoInteractionController.tsx:124-128` discards all inner answer events and their payloads. It reacts only to `block.completed`.
- `packages/course-renderer/src/blocks/media/VideoInteractionController.tsx:113-119` emits only `{ interactionId }` on completion.
- `packages/course-contract/src/videoInteraction.ts:49-58` has pause/required/activity fields but no presentation or dismissal policy.

**Impact**

The current UI does not meet a modal-interaction expectation, and the CourseSession cannot tell what the learner answered, whether it was correct, how many attempts were used, or what result should inform Closing and analytics.

**Required change**

Decide and encode the supported presentation policy. If cue activities are modal, implement an accessible dialog with focus management and explicit required/optional dismissal rules. Forward a typed cue-result payload rather than suppressing assessment evidence.

**Acceptance criteria**

- Modal cues use appropriate dialog semantics, keyboard focus, focus restoration, and required-cue dismissal rules.
- `video.interaction.completed` contains a validated, versioned result with the interaction ID and assessment evidence.
- Required and optional cues have distinct, tested skip/resume behavior.

### P1-03 — Closing is normally skipped in the production host

**Evidence**

- `packages/course-renderer/src/course/CoursePlayer.tsx:139-161` saves the Closing, then calls `setStatus(..., "completed")`, and only afterward calls `setClosing` and enters the `closing` phase.
- `apps/web/src/shell/courses/RuntimeCoursePlayer.tsx:60-68` converts `setStatus("completed")` directly into `onFinish()`.
- `apps/web/src/shell/courses/CoursesContainer.tsx:76-90` handles `onFinish` by replacing the player with the report placeholder.
- The package-level CoursePlayer test uses an in-memory adapter without the production `onFinish` wrapper, so it does not reproduce this unmount sequence (`packages/course-renderer/test/coursePlayer.test.tsx:43-95`).

**Impact**

The personalized/prepared Closing scene is generated and stored but is usually unmounted before the learner can see or hear it.

**Required change**

Make Closing an explicit lifecycle phase. Set session status to `closing`, render/play the scene, then mark the session `completed` only after the Closing completion policy is met. Expose a dedicated `CoursePlayer.onComplete` callback instead of inferring UI completion by wrapping the persistence adapter.

**Acceptance criteria**

- The learner sees the Closing before the parent route changes.
- The session transitions `in-progress -> closing -> completed` in that order.
- A production-host integration test uses the real `CoursePlayer` and proves the Closing is visible before `onFinish` fires.

### P1-04 — Events and progress are not persisted, and resume is not implemented

**Evidence**

- The event bus creates complete event envelopes at `packages/course-runtime/src/eventBus.ts:53-68`.
- `packages/course-renderer/src/slice/SlicePlayer.tsx:198-224` subscribes to events and folds state, but never calls `sessionAdapter.appendEvent`.
- A repository-wide search finds `appendEvent` implementations but no renderer call site.
- `SlicePlayer` accepts `restoreStepId` and `restoreState` at `packages/course-renderer/src/slice/SlicePlayer.tsx:40-56`, but `CoursePlayer` never passes them (`packages/course-renderer/src/course/CoursePlayer.tsx:205-217`).
- `CoursePlayer` receives an existing session but immediately sets it to opening, regenerates Opening, and resets `currentIndex` to zero (`packages/course-renderer/src/course/CoursePlayer.tsx:84-137`).
- The session contract contains `current`, workflow step, timestamps, elapsed time, media position, and interaction result (`packages/course-contract/src/session.ts:15-66`), but the renderer does not maintain most of them.

**Impact**

Event replay, auditability, Closing evidence, analytics, true resume, current-position recovery, and authoring diagnostics do not work. Returning learners restart from the beginning even though the API calls the response a resumed session.

**Required change**

Persist every accepted runtime event before or atomically with its derived state; persist active course/slice/workflow position, status timestamps, elapsed time, media position, answers, and interaction results. Restore the existing opening/closing, current Slice, workflow step, and completed state from a validated CourseSession.

**Acceptance criteria**

- Reloading mid-Slice restores the same Slice, workflow step, block state, completed cue state, assessment attempt state, and safe media position.
- Event history contains narration, video, PDF, HTML, assessment, timer, completion, and navigation events with correct provenance.
- Closing generation receives actual allowed session evidence instead of an empty object.

### P1-05 — `NavigationDefinition` is declared but not implemented

**Evidence**

- The contract declares previous, manual-next, auto-next, and revisit behavior at `packages/course-contract/src/navigation.ts:3-10`.
- `CoursePlayer` advances only when a workflow `navigate` effect calls `onNavigateNext` (`packages/course-renderer/src/slice/SlicePlayer.tsx:142-195`, `packages/course-renderer/src/course/CoursePlayer.tsx:163-171`).
- No renderer reads `slice.navigation`.
- `student.continue` is part of the event vocabulary (`packages/course-contract/src/workflow.ts:28-45`) but there is no Course Runtime 2.0 control that emits it.

**Impact**

Previous navigation, manual next, completion gating, auto-next policy, revisit, and replay semantics do not exist. A valid workflow that waits for `student.continue`, or completes without an explicit `navigate`, can become stuck.

**Required change**

Add course navigation controls owned by `CoursePlayer`/`SlicePlayer`, implement all four contract fields, and make completion state the authority for gating. Separate `completeSlice` from the subsequent navigation decision.

**Acceptance criteria**

- Previous navigation works for reached Slices.
- `manualNext: after-completion` remains disabled until valid completion; `allowed` follows its documented rule.
- `autoNext` is honored rather than requiring every workflow to embed navigation.
- Revisit restores completed state and offers an explicit replay path.
- `student.continue` has a visible, accessible producer.

### P1-06 — One-Slice/one-screen layout is not implemented

**Evidence**

- `packages/course-renderer/src/layout/LayoutRenderer.tsx:39-64` sets only `display: grid` and row/column tracks.
- A repository-wide search finds no CSS definitions for the course layout, Slice, block, PDF, video, iframe, narration, opening/closing, or focus classes.
- `apps/web/src/shell/courses/RuntimeCoursePlayer.tsx:148` explicitly sets the player viewport to `overflowY: "auto"`.
- The design requires one Slice per desktop screen, no uncontrolled page-level overflow, minimum readable sizes, and measurable bounds (`docs/2026-08-15-student-course-runtime-data-and-renderer-design.md:1651-1674` and `1927-1937`).
- Blocks use the HTML `hidden` attribute, which removes their own box from layout. The remaining wrapper has no reserved sizing, so the claim that hidden blocks retain their box is not guaranteed (`packages/course-renderer/src/layout/LayoutRenderer.tsx:35-37`, `packages/course-renderer/src/slice/SlicePlayer.tsx:241-251`).

**Impact**

The presets establish track ratios but not an actual course composition. Slot gaps, padding, height, overflow, aspect containment, typography, focus treatment, control sizes, and hidden-block stability are browser defaults. A Slice can become a scrolling page instead of one designed screen.

**Required change**

Ship a renderer-owned stylesheet or equivalent style layer with an explicit desktop viewport contract. Define Slice height, header/narration allocation, slot gaps, `minmax(0, ...)`, per-slot overflow, media containment, block stacking, hidden placeholders, and measurable overflow diagnostics.

**Acceptance criteria**

- Full, horizontal split, vertical split, and 2/3/4-cell grid are visually verified at the supported desktop viewport matrix.
- The course shell does not page-scroll during a normal Slice; overflow is constrained to an intentional slot viewer when necessary.
- Essential content is not clipped or shrunk below accessibility limits.
- Reveal/hide transitions do not cause unintended global reflow.

### P1-07 — The default PDF renderer is a scaffold, not the specified viewer

**Evidence**

- `packages/course-renderer/src/media/pdfEngine.ts:20-44` uses a native `<embed>` and always returns `totalPages() === 1`.
- `packages/course-renderer/src/blocks/media/PdfRenderer.tsx:30-38` clamps navigation to `totalPages`, so the default Next button is always disabled.
- `packages/course-renderer/src/blocks/media/PdfRenderer.tsx:59` renders an empty viewport container with no built-in height.
- There is no zoom, loading state, render failure state, browser fallback, or page-count discovery.
- PDF tests inject `FakePdfEngine(5)` (`packages/course-renderer/test/media/pdf.test.tsx:27-35`), masking the production engine's one-page behavior.

**Impact**

Multi-page navigation and zoom do not work in production, the viewport may have zero useful height, and a browser that cannot embed the PDF receives no recovery path.

**Required change**

Use a maintained browser PDF engine behind the existing seam, or explicitly reduce the product contract to native-browser viewing. The approved design currently requires all pages, page navigation, zoom, download, loading/error states, and slot-contained rendering.

**Acceptance criteria**

- A real multi-page PDF reports its page count and navigates to all pages.
- Initial page, zoom, keyboard controls, loading, malformed-file error, unsupported-browser fallback, and download are browser-tested.
- The PDF remains inside its assigned Slot and survives signed-URL renewal without showing an expired document.

### P1-08 — HTML completion data is untyped, unvalidated, and not stored

**Evidence**

- `packages/course-renderer/src/blocks/html/protocol.ts:35-37` declares accepted payload as `unknown`.
- `packages/course-renderer/src/blocks/html/protocol.ts:53-60` validates only envelope fields, not payloads for `ready`, `progress`, `completed`, or `error`.
- `packages/course-renderer/src/blocks/html/HtmlInteractionRenderer.tsx:44-65` immediately forwards the unvalidated payload and emits `block.completed`.
- `packages/course-runtime/src/sessionState.ts:114-141` has no `interaction.completed` reducer case, despite `BlockSessionState.interactionResult` existing.
- Runtime events are not appended to the CourseSession (P1-04).

**Impact**

Any correctly tokened iframe can claim completion with missing, malformed, or meaningless learning data. The platform then loses the result even though the block and session contracts imply that interaction evidence is stored.

**Required change**

Define versioned, JSON-compatible payload schemas per HTML message type. At minimum, the completed payload needs a stable interaction/result identity and the authored interaction's learning evidence; host-owned fields such as event ID and occurrence time should still be stamped by the host. Validate before emitting completion and save the validated result to both the event log and block session state.

**Acceptance criteria**

- Missing or invalid completion evidence is rejected and cannot complete the block.
- Valid completion produces a typed event and populates `interactionResult`.
- Duplicate completion is idempotent by a stable completion/result ID.
- Closing can consume allowed HTML interaction results from the saved session.

### P1-09 — HTML music autoplay and media lifecycle are not implemented

**Evidence**

- `packages/course-contract/src/blocks.ts:88-97` has no HTML media capability or playback policy.
- `packages/course-renderer/src/blocks/html/HtmlInteractionRenderer.tsx:143-154` does not grant an iframe `allow="autoplay"` permission or expose a host-to-frame media lifecycle.
- `enabled=false` only adds `aria-disabled` (`HtmlInteractionRenderer.tsx:132-150`); pointer/keyboard interaction inside the iframe remains live.
- Hidden/disabled iframes stay mounted. No message tells authored HTML to pause music when hidden, disabled, or navigated away.
- There is no audio arbiter coordinating Slice narration, video audio, and HTML music.

**Impact**

HTML music is not guaranteed to start. Browser autoplay rejection is invisible to the host, competing audio can overlap, and music may continue after the interaction is hidden or disabled. A disabled interaction can still receive learner input.

**Required change**

Treat HTML audio as an explicit cross-frame capability rather than assuming the iframe can autoplay. Add a host-to-frame lifecycle protocol such as activate/deactivate, enable/disable, pause/resume/stop media; define audio arbitration with narration/video; grant only the required Permissions Policy; and provide a visible start-audio fallback when the browser blocks playback.

**Acceptance criteria**

- After the learner's course-start gesture, visible authored HTML can start its declared music under the supported browser policy.
- If autoplay is blocked, the learner sees a clear one-click fallback and the workflow does not deadlock.
- Hiding, disabling, leaving, or unmounting the block pauses/stops its media.
- Narration, video audio, and HTML music follow one tested priority/ducking policy.
- `enabled=false` actually prevents interaction, not merely announces it to assistive technology.

### P1-10 — The HTML sandbox does not block external network access

**Evidence**

- `packages/course-renderer/src/blocks/html/HtmlInteractionRenderer.tsx:147-151` uses `sandbox="allow-scripts"` and comments that there are no network affordances.
- HTML sandbox flags remove origin privileges and navigation/form capabilities, but do not by themselves prevent scripts, images, audio, or fetches from contacting external origins.
- The design says interactive HTML is self-contained and is not an escape hatch for external network access (`docs/2026-08-15-student-course-runtime-data-and-renderer-design.md:1764-1780`).

**Impact**

Authored HTML can still make third-party requests, track learners, or exfiltrate interaction data. Relative secondary assets are also unreliable because only the top-level HTML URL is signed; relative requests do not automatically inherit that signed query.

**Required change**

Enforce self-containment at publish validation and at runtime. Validate that HTML contains no prohibited external/relative network dependencies, and serve or inject a restrictive Content Security Policy appropriate for the allowed inline/data/blob assets.

**Acceptance criteria**

- A publish-time fixture with `fetch("https://...")`, external scripts, remote images/audio, or unsigned relative dependencies is rejected.
- A runtime CSP blocks unexpected network destinations even if static validation misses one.
- A valid self-contained HTML interaction, including its music asset, runs without secondary network requests.

### P1-11 — Signed-URL refresh can break active media and does not refresh PDF correctly

**Evidence**

- `apps/web/src/shell/courses/RuntimeCoursePlayer.tsx:90-103` replaces the complete signed URL map and forces a re-render.
- Video, iframe, and image renderers resolve their `src` during render, so a refreshed query string changes the live DOM source.
- A changed video source can reload media and lose current time/cue state; a changed iframe source reloads the interaction and loses internal state.
- `PdfRenderer` captures its URL in a mount-only effect (`packages/course-renderer/src/blocks/media/PdfRenderer.tsx:18-28`), so its existing embed keeps the old, eventually expired URL instead of the new one.

**Impact**

Long sessions can reset a video or HTML interaction during active work, while the PDF follows the opposite failure mode and retains an expired URL.

**Required change**

Define per-block URL-renewal behavior. Do not blindly replace active media sources. Refresh lazily on reload/error, or preserve and restore media/interaction state around an intentional source replacement. Make PDF renewal explicit.

**Acceptance criteria**

- A forced URL refresh during active video, PDF, iframe, and image use does not lose learner state or produce a 403.
- Video retains playback/cue state, HTML retains or safely restores interaction state, and PDF receives a usable renewed URL.

### P1-12 — Asset format validation required for publication does not exist in the contract/runtime path

**Evidence**

- `packages/course-contract/src/blocks.ts:60-97` validates relative paths and basic fields, but not PDF structure, HTML structure/protocol implementation, video container/codec/faststart, audio codec, or WebVTT content.
- The design explicitly requires valid PDF structure, MP4/H.264/AAC/faststart, WebVTT, audio readability/duration, and self-contained protocol-compliant HTML (`docs/2026-08-15-student-course-runtime-data-and-renderer-design.md:1917-1925`).

**Impact**

A course can pass the single JSON validator and upload assets that the student browser cannot render. Teacher tooling cannot treat JSON validity as publish readiness.

**Required change**

Keep binary/media inspection outside the pure JSON contract, but make it a mandatory pre-publish validation stage. Return structured issues keyed by asset path and consuming block.

**Acceptance criteria**

- Video validation checks container, H.264 video, optional AAC audio, duration tolerance, faststart, and captions format.
- PDF validation opens the file and confirms a valid page tree/page count.
- HTML validation checks self-containment, required protocol handshake/completion calls, typed payload examples, and prohibited APIs/URLs.
- Publish/ship refuses a course with any release-blocking asset issue.

### P2-01 — Workflow validation does not enforce all approved invariants

**Evidence**

- `packages/course-contract/src/validate/workflow.ts:26-33` creates reference sets, but does not validate `initialState.visibleBlockIds`, `enabledBlockIds`, or `focusedTarget`.
- Transitions are checked for target-step existence only (`workflow.ts:92-109`); event source IDs are not checked against compatible producers, and interaction/timer reference semantics are incomplete.
- Ambiguity detection compares only event type and `sourceId`, ignoring `interactionId` and `timerId` (`workflow.ts:97-109`). This can reject valid disjoint cue transitions and does not model the runtime's full matcher intersection.
- A terminal is defined as any step containing `completeSlice` **or** `navigate` (`workflow.ts:133-160`). The validator does not prove that a required completion cannot be bypassed before navigation.
- The cycle check treats several event labels as evidence of boundedness (`workflow.ts:4-5`, `163-178`) but does not prove a finite attempt limit. A `submit-correct` remediation cycle can remain infinite.

**Impact**

Some impossible workflows pass validation, while some valid cue/timer-specific workflows can be rejected. The validator cannot currently support the guide's guarantee that every accepted course will play to a valid completion.

**Required change**

Add initial-state reference checks, producer-aware event validation, full matcher-overlap analysis, timer/interaction reference checks, terminal ordering rules, required-completion dominance/reachability checks, and genuinely bounded-cycle analysis.

**Acceptance criteria**

- Every rule listed in design section 12.6 has a positive and negative validator fixture.
- The golden course and cue-specific branching fixtures pass.
- Workflows with unknown event sources, bypassed required completions, unbounded remediation, or navigation-before-completion fail with precise paths.

### P2-02 — Assessment, video, and HTML state reducers do not match emitted payloads

**Evidence**

- Assessment renderers emit `answer.submitted` as `{ value }` (`SingleChoiceRenderer.tsx:29-48`, `FillBlankRenderer.tsx:39-57`).
- The session reducer stores `payload.answer` (`packages/course-runtime/src/sessionState.ts:114-125`), so real submitted values become `undefined`.
- The reducer expects `positionSeconds` for `video.paused`/`video.ended` (`sessionState.ts:132-137`), but `VideoRenderer` emits those events without a position (`VideoRenderer.tsx:44-52`, `66-72`).
- There is no reducer case for `interaction.completed` or `video.interaction.completed`.

**Impact**

Even if snapshots are saved, answers, media position, and interaction evidence are missing or wrong.

**Required change**

Define typed event payloads in one shared module and make producers, reducers, validators, tests, and analytics consume those types. Avoid separate handwritten payload assumptions.

**Acceptance criteria**

- Integration tests use the actual renderer emitters and assert the resulting CourseSession state.
- Answer values, attempt numbers, correctness/exhaustion, video position, cue results, and HTML results survive save/reload.

### P2-03 — Video control, reset, disabled-state, and optional-cue behavior are incomplete

**Evidence**

- `VideoRenderer` ignores the `enabled` prop (`packages/course-renderer/src/blocks/media/VideoRenderer.tsx:20-119`).
- Native `<video controls>` play/pause events are not observed; only the extra custom buttons and workflow handles emit started/paused events.
- `reset` clears renderer completion flags but does not reset the cue controller's fired/completed sets or required gate (`VideoRenderer.tsx:54-58`, `VideoInteractionController.tsx:71-84`).
- Once an optional cue with `pauseVideo: true` appears, there is no skip/dismiss path.
- Play-promise rejection is ignored (`packages/course-renderer/src/media/videoEngine.ts:29-35`).

**Impact**

Disabled video remains interactive, native learner actions are missing from evidence, replay may retain stale cue state, optional cues can behave as required, and autoplay failures can leave the workflow waiting silently.

**Required change**

Unify native and workflow media events, implement a true reset contract across video and cues, define optional cue dismissal, enforce disabled state, and surface play errors with learner recovery.

### P2-04 — Audio playback and scene audio are incomplete

**Evidence**

- `HtmlAudioEngine` ignores `audio.play()` rejection (`packages/course-renderer/src/narration/audioEngine.ts:37-43`). A workflow waiting on `narration.ended` can then wait forever.
- Opening and Closing `RuntimeSceneResult.audioUrl` values are never played; the scene components render text only (`OpeningScene.tsx:21-37`, `ClosingScene.tsx:16-36`).
- `pauseNarration` and `stopNarration` carry a narration ID in the contract, but `SlicePlayer` pauses/stops whichever track is active without verifying that ID (`SlicePlayer.tsx:155-165`).

**Impact**

Prepared narration and generated scene speech are not reliable under browser autoplay rules, and opening/closing audio is unused.

**Required change**

Expose playback status/error, add a learner-start fallback, render Opening/Closing audio, and enforce the target narration semantics. Coordinate this with the single audible-source policy from P1-09.

### P2-05 — Opening/Closing personalization inputs are placeholders

**Evidence**

- Opening passes `signalValues: {}` and Closing passes `sessionEvidence: {}` (`packages/course-renderer/src/course/CoursePlayer.tsx:105-117`, `142-153`).
- `OpeningScene` declares `objectives` but does not render them (`packages/course-renderer/src/scenes/OpeningScene.tsx:6-12`, `21-37`).

**Impact**

The real-time opening cannot reflect learning history, and the real-time Closing cannot reflect what happened in the course. One of the product's defining behaviors is therefore absent.

**Required change**

Resolve allowed history signals at the authenticated host/API boundary and derive Closing evidence from the validated CourseSession. Render the approved opening facts consistently, including objectives when the product requires them.

### P2-06 — Focus is visual metadata only, with no actual focus or guaranteed styling

**Evidence**

- `FocusTarget` adds `data-focused` and a class (`packages/course-renderer/src/focus/FocusManager.tsx:45-60`) but does not call DOM focus, scroll the target into view, or announce a change.
- No `.course-focus-ring` CSS definition exists in the renderer or web app.

**Impact**

Workflow `focus` actions may have no visible effect and do not move keyboard/screen-reader focus. This does not meet the design's accessible focus requirement.

**Required change**

Define whether workflow focus means visual emphasis, DOM focus, scrolling, or a combination; implement it accessibly and style it in the renderer-owned course stylesheet.

### P2-07 — Session snapshot persistence can silently lose progress

**Evidence**

- `apps/web/src/course/apiSessionAdapter.ts:47-55` clears `pending` before awaiting the save. A rejection does not restore dirty state.
- The debounce callback uses `void flush()` without error handling (`apiSessionAdapter.ts:57-64`).
- `RuntimeCoursePlayer` spreads the adapter into the narrower `SessionAdapter`, loses the `flush` surface, and does not flush on unmount/pagehide/visibility change (`RuntimeCoursePlayer.tsx:60-75`).
- Whole-session last-write-wins snapshots have no revision/ETag or conflict handling.
- The API accepts raw session JSON and extracts status best-effort rather than validating the CourseSession contract (`apps/api/internal/api/course_session.go:116-146`).

**Impact**

A transient save failure, quick exit, multiple tab, malformed client snapshot, or concurrent update can lose or overwrite progress without informing the learner.

**Required change**

Retain dirty state until successful persistence, retry with bounded backoff, flush on lifecycle exits, expose save status, validate snapshots server-side, and add optimistic concurrency or an append/event revision strategy.

### P2-08 — Course definition updates have no session compatibility/version policy

**Evidence**

- `CourseSession` records only `courseSchemaVersion: "2.0"`, not a definition revision or content hash (`packages/course-contract/src/session.ts:52-66`).
- The authoring lifecycle is designed to update the same course identity in place.
- Existing saved states can therefore reference removed Slice, step, block, or cue IDs after a definition update.

**Impact**

Updating a preview or published course can make existing sessions impossible to restore or can apply stale progress to semantically different content.

**Required change**

Add an immutable definition revision/hash to the stored definition and CourseSession. Define whether an update migrates, forks, or resets affected sessions; never infer compatibility from schema version alone.

### P2-09 — Contract-level authoring quality checks are incomplete

**Evidence**

- Language is only `min(2)`, not BCP-47 validated (`packages/course-contract/src/course.ts:51-61`).
- Estimated course minutes are not checked against Slice totals.
- Objective evidence IDs only need to exist; they can point to static text/image/PDF blocks that produce no learning evidence (`validate/referential.ts:86-92`).
- Image item IDs and choice option IDs are not checked for uniqueness.
- `presentation: "single"` can contain many images, while the renderer silently displays only the first (`packages/course-renderer/src/blocks/ImagesRenderer.tsx:76-84`).
- Text content may be empty (`packages/course-contract/src/blocks.ts:46`).
- The contract does not enforce the server's 256-asset limit (`apps/api/internal/api/course_asset_urls.go:19-22`, `75-78`).

**Impact**

Courses can validate yet contain hidden content, ambiguous answer IDs, weak evidence mapping, inconsistent duration metadata, or too many assets for the serving API.

**Required change**

Add deterministic checks where they are true contract invariants; keep pedagogical density recommendations in the authoring Skill. Avoid silently discarding authored content.

### P2-10 — Error handling can silently choose the wrong player or leave blank/stuck media

**Evidence**

- `apps/web/src/shell/courses/CoursesContainer.tsx:32-49` falls back to the legacy player for every definition-fetch error, not only a 404.
- PDF, iframe, video, narration, and workflow action failures do not have a shared error boundary or typed learner-recovery surface.
- Several imperative actions use optional chaining and silently do nothing when a media handle is unavailable (`SlicePlayer.tsx:182-187`).

**Impact**

Authentication, network, server, or malformed-course errors may mount an unrelated legacy player. Media/action failures can leave workflows waiting forever with no diagnostic information.

**Required change**

Fall back to legacy only for the explicit no-definition 404 case. Add typed runtime errors, per-block error UI, retry/skip policies where pedagogically valid, and a top-level error boundary with course/slice/block/step diagnostics.

### P2-11 — Tests pass but do not exercise the production Course Runtime 2.0 path

**Evidence**

- `apps/web/test/shell/courses/runtimeCoursePlayer.test.tsx:7-25` mocks the entire `CoursePlayer`.
- Video interaction tests inject the loader that production lacks.
- PDF tests inject a fake multi-page engine.
- `apps/web/e2e/journey-course-loop.spec.ts:5-29` exercises the legacy phase/card course, not CourseDefinition 2.0 blocks and workflows.
- There is no browser test covering the golden Course Runtime 2.0 course end to end.

**Impact**

All current tests can pass while production video cues, Closing, PDF navigation, HTML music, session resume, and one-screen layout remain broken.

**Required change**

Add a real-browser golden-course test using the production host and representative assets. Keep unit tests, but add integration tests across contract -> adapters -> real renderer -> session snapshot -> resume.

## Test baseline from this review

Commands executed:

```text
pnpm --filter @mind-imprint/course-contract \
     --filter @mind-imprint/course-runtime \
     --filter @mind-imprint/course-renderer \
     --filter web -r test

pnpm --filter @mind-imprint/course-contract \
     --filter @mind-imprint/course-runtime \
     --filter @mind-imprint/course-renderer \
     --filter web -r typecheck
```

Results:

| Package | Test result |
| --- | ---: |
| `course-contract` | 13 files, 56 tests passed |
| `course-runtime` | 5 files, 28 tests passed |
| `course-renderer` | 19 files, 97 tests passed |
| `web` | 180 files, 1,113 tests passed |
| **Total** | **217 files, 1,294 tests passed** |

All four TypeScript typechecks passed. The run used Node `20.20.2`; pnpm reported that `apps/peraspera` expects Node `22.x`. Renderer PDF tests also printed jsdom's expected “navigation not implemented” stderr when clicking download links. These warnings did not fail the suites.

No Course Runtime 2.0 Playwright journey was run because the repository does not currently contain one.

## Required release acceptance gate

The student runtime should not be declared preview/publish ready until one production-host browser journey proves all of the following with real fixture assets:

1. A valid CourseDefinition loads through the real API and matches the supported desktop one-screen layout.
2. All four layout presets render without uncontrolled page scrolling or clipped essential content.
3. Prepared narration plays or presents a recoverable user-start fallback.
4. A multi-page PDF loads, zooms, navigates, downloads, and survives URL renewal.
5. A video reaches a required cue, auto-pauses, displays the chosen interaction presentation, saves the answer/result, resumes, and completes.
6. Interactive HTML completes only with a valid typed learning-result payload; invalid payloads are rejected.
7. HTML music follows the declared autoplay/fallback and audio-arbitration policy, and stops on hide/disable/navigation.
8. The HTML frame cannot make prohibited external network requests.
9. Previous/manual/automatic navigation and `student.continue` follow `NavigationDefinition`.
10. Reloading mid-course restores the current Slice, workflow step, attempts, cue state, interaction result, and media position.
11. The Closing uses real saved session evidence, is visible/audible, and only then completes the session and routes onward.
12. A transient session-save or signed-URL-refresh failure is visible/retried and does not erase learner progress.

## Recommended engineering boundary

Keep the existing three-package split:

- `course-contract`: JSON shape plus deterministic structural/referential/workflow invariants.
- `course-runtime`: typed event payloads, deterministic workflow/session reduction, replay and restore semantics.
- `course-renderer`: the single visual/media implementation, including course CSS, accessible block states, and browser-facing error recovery.
- student web host/API: authenticated history signals, async interaction loading, asset URL lifecycle, durable session persistence, and definition revision policy.
- authoring/publish validation: binary media inspection, HTML static/runtime conformance checks, visual viewport review, and the final publish gate.

The teacher preview should consume the corrected renderer and runtime rather than reproduce them. The authoring Skill can then safely transform materials, validate the JSON and assets, launch the same renderer for annotation, revise the same course identity, upload only changed content-addressed assets, and finally call the publish API.
