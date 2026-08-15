# Course Runtime — Slice 6: InteractiveHtml Block (sandboxed iframe + protocol)

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add `HtmlInteractionRenderer` to `packages/course-renderer` — the sandboxed-iframe boundary for `interactiveHtml` blocks — with a versioned `postMessage` protocol, per-mount session binding, strict message validation, and the standardized interaction Events.

**Architecture:** The HTML block is the extension point for complex custom interactions and the ONLY place course-authored code runs — so it is the tightest trust boundary (§20). The renderer loads one self-contained HTML file into a **sandboxed** iframe, mints a per-mount session token, and accepts a completion (or progress/error) ONLY after validating the message's `source` Window, `type`, `protocolVersion`, session token, and payload shape. Everything else is rejected silently. Valid messages become typed runtime Events; a valid completion emits `block.completed`. The renderer never grants the iframe access to app state or credentials, and the iframe has no network capability (§9.5, §17.11, §20).

**Tech Stack:** React 18, contracts + runtime packages, Vitest + jsdom + RTL. Message validation is pure and unit-testable independent of the iframe.

**Authoritative spec:** §9.5 (InteractiveHtmlBlock), §17.11 (HtmlInteractionRenderer), §20 (security boundaries).

## Global Constraints

- Consume Slice 3 `BlockRendererProps`; register `interactiveHtml`, replacing its `NotImplementedRenderer` entry.
- Iframe attributes: `sandbox="allow-scripts"` (scripts only — NO `allow-same-origin`, so the frame is opaque-origin and cannot reach app cookies/storage), no `allow-forms`/`allow-popups`/`allow-top-navigation`. Enforce the declared `aspectRatio` (`1:1`/`4:3`). Load the resolved `source` via `assetResolver`.
- **Protocol** `protocolVersion: "1.0"`: messages from the frame are `{ protocol: "mind-course-interaction"; version: "1.0"; sessionToken: string; type: "ready"|"progress"|"completed"|"error"; payload?: unknown }`. The renderer→frame handshake posts `{ protocol, version, sessionToken }` on load so the frame echoes the token back. A message is accepted only if: `event.source === iframe.contentWindow`, `protocol` matches, `version === block.protocolVersion`, `sessionToken` equals the minted token, and `type` is known. Anything else → ignored (optionally surfaced via an injectable `onRejected` for diagnostics), never an Event.
- Events: `interaction.ready`, `interaction.progress`, `interaction.completed`, `interaction.error`; `block.completed` after a valid `completed` message when `completion.rule === "interaction-complete"`.
- Deterministic tests: the session token comes from an injected `tokenFactory` (default random, tests supply fixed). No `Date.now()`/`Math.random()` in logic.
- No `/dev/null` redirects; never `git add -A`; test via `pnpm --filter @mind-imprint/course-renderer test`.

---

### Task 1: Message protocol validator (pure)

**Files:** Create `src/blocks/html/protocol.ts`. Test `test/html/protocol.test.ts`.

**Interfaces:**
- Produces: `PROTOCOL_NAME`, `PROTOCOL_VERSION`, `parseFrameMessage(raw, ctx): { ok: true; type; payload } | { ok: false; reason }` where `ctx = { sessionToken; expectedVersion }`. Validates protocol name, version, token, and known `type`; returns a typed reason on rejection.

- [ ] **Step 1: Failing test** — a well-formed `completed` message with the right token/version → `ok`; wrong token → `ok:false reason:"token"`; wrong version → `reason:"version"`; unknown type → `reason:"type"`; non-object / missing protocol → `reason:"shape"`.
- [ ] **Step 2: Run → FAIL.** — [ ] **Step 3: Implement.** — [ ] **Step 4: Run → PASS.** — [ ] **Step 5: Commit** — `feat(course-renderer): interactive-html message protocol`.

---

### Task 2: HtmlInteractionRenderer

**Files:** Create `src/blocks/html/HtmlInteractionRenderer.tsx`. Test `test/html/htmlInteraction.test.tsx`. Register `interactiveHtml`.

**Spec:** §9.5, §17.11, §20.

Behavior:
- Renders a wrapper enforcing `aspectRatio` and a sandboxed iframe (`sandbox="allow-scripts"`) with `src = assetResolver.resolve(block.source)`.
- On mount: mint a `sessionToken` via `tokenFactory`. On iframe `load`, post the handshake (`{ protocol, version, sessionToken }`) to `iframe.contentWindow`.
- Add a `window` `message` listener that: checks `event.source === iframe.contentWindow`, then `parseFrameMessage(event.data, { sessionToken, expectedVersion: block.protocolVersion })`. On `ok`: map `type` → emit `interaction.<type>`; on `completed` with `completion.rule === "interaction-complete"` also `emit(block.id, "block.completed")` and mark done. On `!ok`: call the injectable `onRejected(reason)` (default no-op) and drop it — NEVER emit.
- Clean up the listener on unmount. A second mount mints a new token (old tokens are dead).
- When `enabled=false`, the iframe still renders but the block is visually marked non-interactive (the sandbox already isolates it; interactivity gating is advisory here since the frame is opaque — document this).

- [ ] **Step 1: Failing tests** (RTL; simulate frame messages by dispatching `MessageEvent`s. To satisfy `event.source === iframe.contentWindow`, use a test seam: the renderer reads the "expected source" from a ref that the test can point at a stub window, OR the renderer exposes an internal `handleMessage(data, source)` the test calls directly — prefer testing `handleMessage(data, source)` unit-style to avoid jsdom `MessageEvent.source` being unsettable):
  - iframe renders with `sandbox="allow-scripts"` (assert NO `allow-same-origin`) and the aspect-ratio wrapper.
  - a valid `ready` message from the frame source → emits `interaction.ready`.
  - a valid `completed` message → emits `interaction.completed` then `block.completed`.
  - a message with the wrong token → NO emit, `onRejected("token")` called.
  - a message whose `source` is not the iframe window → NO emit.
- [ ] **Step 2: Run → FAIL.** — [ ] **Step 3: Implement.** Factor the message handling into an internal `handleMessage(data, source)` so it is unit-testable and the `window` listener is a thin adapter. — [ ] **Step 4: Run → PASS.** Run FULL renderer suite + typecheck. — [ ] **Step 5: Commit** — `feat(course-renderer): sandboxed interactive-html renderer`.

---

## Self-Review Notes

- **Trust boundary is the point:** every acceptance predicate (source Window, protocol, version, session token, type, payload) must pass or the message is dropped — this is §17.11 verbatim. Testing `handleMessage(data, source)` directly is the reliable way to prove each rejection path under jsdom, where `MessageEvent.source` is not freely settable.
- **No escape hatch:** `sandbox="allow-scripts"` without `allow-same-origin` gives the frame an opaque origin — it cannot read app cookies/storage, and with no `allow-forms`/`allow-popups`/network affordances it is inert beyond posting messages (§20). The renderer grants zero access to application state.
- **Registry now total with real renderers:** after this slice every block `type` maps to a real renderer; the `NotImplementedRenderer` placeholder is only used if a future unknown type appears. Update the Slice 3 registry test if it asserted `singleChoice`/`interactiveHtml` hit the placeholder.
