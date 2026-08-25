# Projects "Extremity" E2E — Interruption & Out-of-Order Robustness

> 2026-08-25. A deliberately adversarial pass over the **projects writing flow**:
> students who refresh / close the tab / navigate away mid-action, double-click,
> or jump between stages out of the ideal order — does the platform crash, lose
> data, or strand them? Playwright on live prod (`mind-web.uni-robot.cn`) as
> `@walk.mindimprint.cn` personas, backed by a full code-path map of the SSE /
> JSON turn, autosave, card, plan-gen, finish, and routing layers.

Companion to the happy-path walk `docs/2026-08-25-projects-e2e-bug-findings.md`.

## TL;DR

Two **deterministic, silent data-loss** bugs found and fixed; one benign
self-healing UX gap documented; everything else verified robust.

| # | Area | Interrupt | Result | Severity | Status |
|---|------|-----------|--------|----------|--------|
| EDGE-01 | Coach chat turn | refresh mid-turn (during the LLM call) | user message saved, AI reply lost → **orphan message, no reply** | LOW (self-healing) | documented, left as-is |
| EDGE-02 | Draft autosave (essay `DraftPane` + proposal `ProsePane`) | type then hard-refresh / close within the 1.2s debounce | last ≤1.2s of typing **silently lost** | **MED/HIGH (data-loss)** | **FIXED + verified** |
| EDGE-03 | 工具卡 partial fill (`StudioCardSheet`) | fill a card then refresh without submitting | **all typed fields wiped**, card reopens blank | **HIGH (silent data-loss)** | **FIXED + verified** |

Shipped **`126f25cc` (web)**, deployed to prod, both fixes re-verified live.

---

## Architecture note that reframed everything

The main **coach chat turn is a plain JSON POST** (`POST /coach`,
`apps/api/internal/api/coach.go`), NOT SSE. It persists the student turn first,
then runs the LLM + tools, then persists the reply LAST — all inside one
request context that Go cancels on client disconnect. The SSE endpoints
(`/turn`, `/cards/{id}/submit`, reading turns, `/snapshots/{id}/review`) are a
separate family and are **persist-before-stream**, so a mid-stream disconnect
there only loses frames, not data. This split explains EDGE-01 exactly.

---

## EDGE-01 · Coach mid-turn refresh → orphan message (LOW, self-healing)

**Repro (live):** send a coach message, reload before the reply renders. The
student turn is persisted and shown on reload, but the assistant reply is gone
permanently (the request was aborted before `coach.go` reached the reply-persist
step). Sending ANY next message recovers cleanly — the coach re-reads the orphan
user message and responds to it (verified: "你还在吗？…" got a full, on-topic
reply that referenced the orphaned research question).

**Why not fixed now:** it self-heals, loses no student input (the user message
is saved; only the never-shown generated reply is lost), and never strands or
crashes. A real fix means decoupling the LLM turn from the request context
(run to completion on `context.Background()` so a disconnect still persists the
reply) or a client-side "this turn didn't finish — resend?" affordance — both
are behavior changes worth doing deliberately, not as a drive-by. Documented for
a future pass.

## EDGE-02 · Draft autosave window → data-loss on abrupt exit (FIXED)

**Root cause:** both draft surfaces debounce the autosave PUT by 1200ms and
only *flush* a pending save on React unmount (`useEffect` cleanup) — which does
**not** run on a hard browser refresh, tab-close, or OS backgrounding. There was
no `beforeunload`/`pagehide` handler. So the last ≤1.2s of typing was silently
dropped.

**Proven live (before fix):** a marker typed and then reloaded *synchronously in
the same JS tick* (so the debounce timer could never fire) was gone after reload,
while a marker given >1.2s persisted.

**Fix (`126f25cc`):**
- New `flushBufferKeepalive()` in `apps/web/src/api/writing.ts` — a `PUT
  /buffer` with `keepalive: true` so the request outlives page teardown (cookies
  still ride along via `apiFetch`'s `credentials:"include"`).
- `DraftPane` (essay, `WritingBlock.tsx`) and `ProsePane` (proposal) each add a
  `pagehide` + `visibilitychange:hidden` listener that flushes the pending
  buffer when dirty.

**Verified live (after fix):** a marker typed then `pagehide`-dispatched with NO
debounce wait **survived** the reload.

## EDGE-03 · 工具卡 partial fill wiped on refresh (FIXED)

**Root cause:** `StudioCardSheet` held all field values in local `useState`
only — no autosave, no persistence. A refresh mid-fill lost everything; the
card_instance is `active` server-side but carries no field values, so on reopen
the sheet rebuilds a blank envelope.

**Proven live (before fix):** filled two PEE-card fields, reloaded, reopened —
both blank, markers gone from the DOM.

**Fix (`126f25cc`):** `StudioCardSheet` takes an optional `persistKey`. When set,
the in-progress envelope is mirrored to `localStorage` on every change, restored
on mount (guarded to the same `card_id` so a stale draft from another card can't
bleed in), and cleared on submit/skip. Wired into all three card hosts
(`CoachCardPanel`, `WorkspaceContainer`, `WritingBlock`) as
`mi:carddraft:{projectId}:{cardId}`. Per-viewer convenience (localStorage is the
right tool — 过程即数据 for an *unfinished* card); the server stays the source of
truth for *submitted* cards.

**Verified live (after fix):** filled three fields (Point / Evidence / Stance),
reloaded, reopened — all three restored. Skip cleared the stored draft.

---

## Verified robust (no bug)

- **Plan generation** is an atomic transaction (`regeneratePlan`: delete-all +
  recreate-all in one tx with `defer Rollback`). A refresh during generation
  commits the whole plan or none — no half-written plan. Stage is derived FROM
  plan existence, so a lagging stage self-repairs next turn.
- **Double-click 完成写作 / 定稿** — finish is idempotent (already-finished →
  200 no-op); finalize claims `evaluating` synchronously then spawns a detached
  `context.Background()` worker, and a concurrent finish 409s — no double report.
- **Out-of-order stage jumps** — jumping straight to 写作 on a project with no
  plan degrades gracefully ("还没有写下你的论点——先去开题里想清楚", empty
  outline, empty references), no crash. Off-deck card summons are dropped by
  `IsSummonable`. Status advance is monotonic (can't rewind writing→framework).
- **Empty-draft finish** shows the honest doc-aware message "正文还是空的，先写点
  东西再完成" (last round's fix, confirmed live) — no dead-end.
- **Back/Forward + deep-link** routing uses refs + a `pendingUrlSync` latch; no
  strand on the core writing flow. (Tour-only deep-link ref guards don't
  reset-on-null, but that's guided-tour scope, not real-student Back/Forward.)
- **enter-reading 422 → paste fallback** is the intended load-bearing signal for
  scrape-blocked sources — NOT a bug, left untouched.

## Notes

- There is **no "pause" control** in the projects flow; the equivalent is
  leaving / refreshing mid-action, which the above covers.
- The finish-vs-autosave stale-buffer race (finalize reads the server buffer;
  a sub-1.2s edit before clicking through the two-step finish modal could miss
  the last keystrokes) is now much narrower given the periodic 18s flush + the
  pagehide flush + the modal's own click latency. Left as a documented, very
  narrow window rather than threading a cross-component flush ref.
