# Course Runtime 2.0 Hardening — Program Design & Decomposition

> **Source of requirements:** `docs/2026-08-16-student-course-runtime-code-review.md` (Codex review — 12 P1 release-blockers + 11 P2 + hardening, each with acceptance criteria). That review IS the requirement spec; this doc turns it into an **ordered, buildable set of slices**, and records the **default product decisions** I make for the items the review leaves open (so they can be overridden).
>
> **Goal:** make the student Course Runtime 2.0 actually deliver the promise "a document that passes `validateCourseDefinition` (+ the publish asset gate) will play correctly in the production student host" — the release-acceptance gate in the review (§"Required release acceptance gate", 12 criteria).
>
> **Engineering boundary (from the review, kept):** `course-contract` = JSON shape + deterministic invariants; `course-runtime` = typed event payloads + deterministic reduce/replay/restore; `course-renderer` = the single visual/media implementation incl. course CSS + accessible states + error recovery; student web host/API = auth history signals, async interaction loading, asset-URL lifecycle, durable persistence, definition-revision policy; publish validation = binary/media inspection + HTML conformance + the final gate.

## Default product decisions (open items the review asks us to decide)

These are chosen to be shippable and reversible; each is flagged where the user may want a different call.

- **D1 — Video cue presentation (P1-02):** cues render as an **accessible modal dialog** (portal + backdrop + focus trap + focus restore). Required cues block dismissal until completed; optional cues (`required:false`) get an explicit "跳过" control. Rationale: the design language and the review both lean modal; inline is the current bug.
- **D2 — PDF viewer (P1-07):** bundle **pdf.js** (`pdfjs-dist`) behind the existing `PdfEngine` seam — real page count, page nav, zoom, loading/error, download, slot-contained. It is self-contained (no external CDN — the worker is bundled), which the Artifact-style CSP and our own serving both require. Rationale: the design contract requires all-pages/zoom/nav; native `<embed>` cannot deliver it. (Alt the user could pick: reduce the product contract to native single-file viewing — cheaper, less capable.)
- **D3 — HTML audio/autoplay (P1-09):** treat HTML music as an **explicit host-granted capability**: iframe gets `allow="autoplay"` only for blocks that declare audio; a host→frame lifecycle (`activate/deactivate/enable/disable/pause/resume/stop`) over the existing postMessage protocol; a single **audio arbiter** gives priority narration > video > html-music with ducking; a visible one-click "开始音频" fallback when the browser blocks autoplay. Add a `capabilities.audio?: boolean` to the `interactiveHtml` block (contract addition).
- **D4 — HTML self-containment + CSP (P1-10):** enforce at BOTH publish validation (static: reject `fetch`/external `src`/remote url/unsigned relative dep) AND runtime (inject a strict per-frame CSP via a `srcdoc` wrapper or `Content-Security-Policy` on the served asset limiting to `'self' data: blob:`). Interactive HTML is self-contained by contract.
- **D5 — Definition revision policy (P2-08):** stamp an immutable **content hash** on the stored definition and on each `CourseSession`; on load, if the session's hash ≠ the current definition's hash, the session is **reset** (progress cleared, learner restarts) with a visible notice. Migrate/fork is out of scope for now. Rationale: simplest safe policy; in-place edits during preview are the norm and a reset is honest.
- **D6 — Layout viewport (P1-06):** target **one Slice per desktop screen** at a supported matrix (≥1280×720, ≥1440×900); renderer-owned stylesheet; the host viewport stops page-scrolling (`overflow: hidden` on the shell, per-slot `overflow:auto` only where a slot opts in). Mobile is out of scope for this program.

## Slice order (dependency-aware)

Foundation first (data correctness), then experience (things students see), then media depth, then the gate.

### Slice 1 — Typed event payloads + persistence + resume (P1-04, P2-02, P2-07 core)
The data foundation everything else records into. **Highest priority.**
- `course-runtime`: one shared **typed event-payload module** (answer.submitted `{value}`, video.paused/ended `{positionSeconds}`, video.interaction.completed `{interactionId, result}`, interaction.completed `{...}`, etc.); reducers consume the typed payloads (fix the `payload.answer` vs `{value}` and missing-position mismatches); add reducer cases for `interaction.completed` / `video.interaction.completed`; maintain current part/slice/workflow-step, status timestamps, elapsed, media position, answers, results in `CourseSession`.
- `course-renderer`: `SlicePlayer` calls `sessionAdapter.appendEvent` for every accepted event (before/atomic with derived state); `CoursePlayer` passes `restoreStepId`/`restoreState` and restores current Slice/step/opening/closing/completed from a **validated** existing `CourseSession` instead of resetting to opening/index 0.
- host: `apiSessionAdapter` retains dirty state until save succeeds, bounded-retry, flush on pagehide/visibilitychange/unmount, expose save status; server-side validates the snapshot against the `CourseSession` contract (border) + a revision/hash guard (D5).
- **Acceptance:** reload mid-Slice restores Slice/step/block-state/attempts/cue-state/safe-media-position; event history has correct provenance across all types; Closing generation receives real evidence.

### Slice 2 — Closing lifecycle + scene audio + personalization inputs (P1-03, P2-04, P2-05)
Students currently never see the Closing. **High visibility.**
- `course-renderer`: make Closing an explicit `closing` phase; add a real `CoursePlayer.onComplete` callback (stop inferring completion by wrapping the persistence adapter); status transitions `in-progress → closing → completed` only after the Closing completion policy; render Opening objectives + play Opening/Closing `audioUrl` (coordinated with the audio arbiter from Slice 5); enforce target narration id in pause/stop; audio play-rejection surfaces a learner-start fallback.
- host: `RuntimeCoursePlayer` uses `onComplete` (not the setStatus wrapper); Opening `signalValues` / Closing `sessionEvidence` derived from the authenticated host/API + the validated session (no more `{}`).
- **Acceptance:** learner sees the Closing before the route changes; the production-host test proves it; scene audio plays or offers a fallback.

### Slice 3 — Production video interaction loading + cue modal + result (P1-01, P1-02, P2-03)
Unblocks the golden course's `video-ended-and-interactions-completed` deadlock.
- `course-renderer`: replace the synchronous `InteractionLoader` with an **async resource boundary** (loading/error/typed states); export an interaction-loader API; the cue controller renders the modal (D1) with focus management + required/optional dismissal, forwards a **typed cue-result** (`video.interaction.completed` with interactionId + assessment evidence), and gates completion; unify native `<video>` play/pause events into the evidence stream; real `reset` clears cue fired/completed sets; honor `enabled`; surface play-promise rejection.
- host: a production interaction-document adapter fetches the signed JSON, parses `VideoInteractionDocument`, runs `validateVideoInteraction`, caches by source, exposes loading/typed-error.
- **Acceptance:** real host (no test providers) loads the cue JSON; required cue auto-pauses/shows/records/resumes/gates; missing/malformed/expired asset → visible recoverable error, never silent null/deadlock.

### Slice 4 — NavigationDefinition (P1-05, P2-06 focus)
- `course-renderer`: `CoursePlayer`/`SlicePlayer`-owned nav controls implementing all four `NavigationDefinition` fields (previous, manualNext `after-completion`/`allowed`, autoNext, revisit); completion state is the gating authority (separate `completeSlice` from the navigate decision); an accessible visible producer for `student.continue`; implement `focus` action as real DOM focus + scroll-into-view + a styled `.course-focus-ring` in the course stylesheet.
- **Acceptance:** previous works for reached slices; manualNext gates on completion; autoNext honored; revisit restores completed state + replay; `student.continue` has an accessible producer.

### Slice 5 — Course stylesheet / one-screen layout + audio arbiter (P1-06, P1-09 arbiter, P2-06)
- `course-renderer`: ship a **renderer-owned stylesheet** — course shell (no page scroll), Slice height/header/narration allocation, slot gaps, `minmax(0,…)`, per-slot overflow, media containment, block stacking, **hidden-block placeholders that reserve their box** (fix the `hidden`-attribute reflow), focus ring, control sizes; the 4 presets verified at the D6 viewport matrix; the audio arbiter (D3) coordinating narration/video/html-music.
- host: the shell stops page-scrolling (remove `overflowY:auto`; constrain to intentional slot viewers).
- **Acceptance:** the 4 presets render at the viewport matrix without uncontrolled scroll/clipping; reveal/hide doesn't reflow globally.

### Slice 6 — PDF viewer (P1-07) + signed-URL refresh safety (P1-11)
- `course-renderer`: real `PdfEngine` on **pdf.js** (D2) — page count, nav, zoom, keyboard, loading/error, unsupported fallback, download, slot-contained; PDF renewal explicit (re-reads the refreshed URL).
- `course-renderer`/host: per-block URL-renewal — don't blindly swap active media `src`; refresh lazily on reload/error or preserve/restore media/interaction state around an intentional source replacement; video retains playback/cue state, HTML retains/safely-restores, PDF gets a usable renewed URL (no 403, no expired doc).
- **Acceptance:** a forced refresh during active video/PDF/iframe/image loses no state and no 403.

### Slice 7 — HTML boundary hardening (P1-08, P1-09 lifecycle, P1-10)
- `course-contract`: versioned per-message HTML payload schemas (ready/progress/completed/error); `interactiveHtml.capabilities.audio?` (D3).
- `course-renderer`: validate payloads before emitting `block.completed`; forward a typed, id-stable, idempotent completion result; the host→frame lifecycle protocol (D3) incl. pause-on-hide/disable and true `enabled=false` interaction blocking; per-frame CSP + `allow="autoplay"` gating (D3/D4); autoplay-blocked fallback.
- `course-runtime`: `interaction.completed` reducer populates `interactionResult`.
- **Acceptance:** invalid completion rejected; valid → typed event + `interactionResult`; duplicate idempotent; disabled truly blocks; music starts post-gesture or shows fallback and stops on hide/disable/nav; sandbox blocks prohibited external requests.

### Slice 8 — Validation completeness (P2-01, P2-09) + definition revision (P2-08, D5)
- `course-contract`: workflow validation — initial-state ref checks, producer-aware event validation, full matcher-overlap (incl. interactionId/timerId), timer/interaction ref checks, terminal-ordering + required-completion dominance, bounded-cycle analysis; contract quality — image-item/option id uniqueness, non-empty text where a true invariant, 256-asset cap, estimatedMinutes-vs-slice-total (warn), objective-evidence-produces-evidence (warn); every design §12.6 rule gets +/- fixtures.
- host/contract: definition content-hash on stored definition + `CourseSession` (D5) + session-reset-on-mismatch.
- **Acceptance:** impossible workflows fail with precise paths; valid cue/timer workflows pass; the golden + branching fixtures pass; stale-session reset works.

### Slice 9 — Pre-publish asset validation gate (P1-12)
Depends on the just-shipped authoring/publish endpoints.
- `apps/api`: a mandatory pre-publish (ship) validation stage inspecting binaries: video container/H.264/AAC/duration/faststart/WebVTT; PDF page-tree/page-count; HTML self-containment + protocol handshake/completion + prohibited APIs/URLs. Structured issues keyed by asset path + consuming block; **ship refuses** a course with any release-blocking asset issue.
- **Acceptance:** the review's P1-12 acceptance criteria.

### Slice 10 — Real-browser golden-course journey (P1 acceptance gate, P2-11) + error handling (P2-10)
- A production-host Playwright journey over the golden course with representative fixture assets proving all 12 release-gate criteria; legacy-fallback only on the explicit no-definition 404 (not every fetch error); typed runtime errors + per-block error UI + top-level error boundary with course/slice/block/step diagnostics.
- **Acceptance:** the 12-criteria release gate passes green in a real browser.

## Notes on scale & sequencing

This is ~10 slices, each its own spec→plan→build (subagent-driven), verified before the next. Slices 1–3 are the correctness/visibility core and come first. Media-heavy slices (5,6,7) add real fixture assets. Slice 9 depends on the authoring endpoints shipped in the prior program; Slice 10 is the gate. Product-decision items are pinned in D1–D6 above and can be revised without reshaping the slices.
