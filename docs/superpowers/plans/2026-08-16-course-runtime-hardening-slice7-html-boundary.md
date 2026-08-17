# Slice 7 — HTML Boundary: Typed Payloads + Audio Lifecycle

> **For agentic workers:** REQUIRED SUB-SKILL: superpowers:subagent-driven-development. `- [ ]` steps.

**Goal:** Interactive-HTML completion is typed, validated, idempotent, and stored; HTML audio is an explicit host-granted capability with a host→frame lifecycle, a single-audible-source arbiter, an autoplay-blocked fallback, true `enabled=false` disabling, and pause-on-hide. Fixes review **P1-08** and the renderer half of **P1-09**.

**Requirements source:** `docs/2026-08-16-student-course-runtime-code-review.md` findings P1-08 (HTML completion untyped/unvalidated/unstored) + P1-09 (HTML audio/lifecycle). **Product decision D3.**

**Scope boundary (explicit):** **P1-10** (runtime CSP + publish-time static self-containment) is DEFERRED to Slice 9 — a real network CSP for a `src`-loaded iframe needs a server-served `Content-Security-Policy` header on the HTML asset (server/serving work), and the static "reject fetch/external src" check is a publish-time asset-validation concern; both belong with Slice 9's asset gate. The existing `sandbox="allow-scripts"` (no `allow-same-origin`) stays. Also: full audio DUCKING is future — this slice ships a single-audible-source arbiter (pause lower-priority when a higher one plays); tune ducking later.

**Depends on:** Slice 1 (the `interaction.completed` reducer + `interactionResult` already exist), Slice 2 (audio fail-safe), Slice 3 (video-interaction result pattern).

## Global Constraints
- Determinism preserved (no Date.now/Math.random in pure packages).
- Don't break the green suites (renderer 176 / web 1132 / runtime 38 / contract 56).
- HTML message payloads are validated in `course-contract` (Zod) — one shared source; the renderer imports + validates before acting.

---

### Task 1 — Typed, validated, stored HTML completion (contract + renderer + runtime)

**Files:** `packages/course-contract/src/blocks.ts` (`interactiveHtml.capabilities.audio?`) + a new payload-schemas module (e.g. `src/htmlMessages.ts`), `packages/course-renderer/src/blocks/html/protocol.ts` + `HtmlInteractionRenderer.tsx`, `packages/course-runtime/src/sessionState.ts` (verify the `interaction.completed` reducer path); tests.

- [ ] **Versioned per-message payload schemas (P1-08):** define Zod schemas for the accepted HTML→host messages `ready` / `progress` / `completed` / `error`. `protocol.ts` currently types the payload as `unknown` and validates only the envelope — add payload validation. The `completed` payload needs a stable result/interaction identity + the authored interaction's learning evidence (correct/score/answers as applicable); host-owned fields (event id, occurrence time) are stamped by the host, not trusted from the frame.
- [ ] **Validate before completing:** `HtmlInteractionRenderer` currently forwards the unvalidated payload and emits `block.completed` immediately. Validate the `completed` payload first; on invalid → reject (do NOT complete the block; surface a recoverable error). On valid → emit a typed **`interaction.completed {interactionId, result}`** (the Slice-1 reducer stores it into `interactionResult`), THEN gate `block.completed`. Make completion **idempotent** by a stable completion/result id (a duplicate `completed` message does not double-complete).
- [ ] **Contract `capabilities.audio?`:** add an optional `capabilities: { audio?: boolean }` to the `interactiveHtml` block schema (Task 2 uses it to gate `allow="autoplay"`).
- [ ] Tests: an invalid/missing-evidence `completed` payload is rejected and does NOT complete the block; a valid one produces a typed `interaction.completed` + populates `interactionResult` (persisted); a duplicate `completed` is idempotent; `progress`/`ready`/`error` payloads validate. `pnpm --filter @mind-imprint/course-contract --filter @mind-imprint/course-runtime --filter @mind-imprint/course-renderer test` + `typecheck`.
- [ ] Commit.

---

### Task 2 — HTML audio capability + lifecycle + arbiter + true disable (renderer)

**Files:** `packages/course-renderer/src/blocks/html/HtmlInteractionRenderer.tsx` + `protocol.ts` (host→frame messages), a new audio-arbiter (e.g. `packages/course-renderer/src/media/audioArbiter.ts` or extend `narration/audioEngine.ts`), `packages/course-renderer/src/slice/SlicePlayer.tsx` (wire lifecycle on hide/disable); tests.

- [ ] **Host→frame lifecycle protocol (P1-09, D3):** define host→frame messages `activate` / `deactivate` / `enable` / `disable` / `pauseMedia` / `resumeMedia` / `stopMedia` (versioned, same protocol version). The renderer posts them to the iframe on the corresponding runtime transitions (block shown/hidden, enabled/disabled, slice left).
- [ ] **Autoplay capability gating:** grant the iframe `allow="autoplay"` ONLY when `block.capabilities?.audio` is true. When the browser blocks autoplay, show a visible one-click "开始音频" fallback that (on the learner gesture) tells the frame to start its media; never deadlock a workflow waiting on audio.
- [ ] **Single-audible-source arbiter (P1-09):** a small arbiter so narration, video audio, and HTML music don't overlap — priority narration > video > HTML music; when a higher-priority source plays, pause the lower ones (basic pause, not full ducking). Wire the existing narration (`audioEngine`) + video + HTML-music (via the lifecycle `pauseMedia`/`resumeMedia`) through it.
- [ ] **True `enabled=false` (P1-09):** `enabled=false` currently only adds `aria-disabled`; pointer/keyboard interaction inside the iframe stays live. Actually block interaction (e.g. an overlay capturing pointer events / `pointer-events:none` on the frame + a `disable` message to the frame), not merely announce it. Re-enable on `enabled=true`.
- [ ] **Pause on hide/disable/leave:** a hidden/disabled/left iframe gets `pauseMedia`/`stopMedia` so its music doesn't continue.
- [ ] Tests: `allow="autoplay"` present only when `capabilities.audio`; autoplay-blocked shows the fallback and a gesture starts media (no hang); starting narration pauses HTML music (arbiter); `enabled=false` truly blocks interaction (overlay/pointer-events) + posts `disable`; hiding the block posts `pauseMedia`/`stopMedia`. `pnpm --filter @mind-imprint/course-renderer test` + `typecheck`.
- [ ] Commit.

---

## Final verification
- [ ] `pnpm --filter @mind-imprint/course-contract --filter @mind-imprint/course-runtime --filter @mind-imprint/course-renderer --filter web -r test` + `-r typecheck` green (known pre-existing contracts typecheck error excepted).
- [ ] Report notes: P1-10 (CSP + static self-containment) is Slice 9; full audio ducking is future; sandbox `allow-scripts` unchanged.
