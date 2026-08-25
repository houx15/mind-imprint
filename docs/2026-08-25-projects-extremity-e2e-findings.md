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
| EDGE-01 | Coach chat turn | refresh mid-turn (during the LLM call) | user message saved, AI reply lost → **orphan message, no reply** | LOW (self-healing) | **FIXED + verified** (`d5b86fff`) |
| EDGE-02 | Draft autosave (essay `DraftPane` + proposal `ProsePane`) | type then hard-refresh / close within the 1.2s debounce | last ≤1.2s of typing **silently lost** | **MED/HIGH (data-loss)** | **FIXED + verified** (`126f25cc`) |
| EDGE-03 | 工具卡 partial fill (`StudioCardSheet`) | fill a card then refresh without submitting | **all typed fields wiped**, card reopens blank | **HIGH (silent data-loss)** | **FIXED + verified** (`126f25cc`) |
| EDGE-04 | Coach turn (model failure) | model call errors / returns unparseable output | canned "先自己说说看…" 200 makes 印记 look broken | correctness/trust | **FIXED + verified** (`d5b86fff`) |

Shipped **`126f25cc` (web)** + **`d5b86fff` (api)**, deployed to prod, all fixes re-verified live.

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

## EDGE-01 · Coach mid-turn refresh → orphan message (FIXED, `d5b86fff`)

**Root cause:** the coach turn is a synchronous JSON POST that runs entirely on
`r.Context()`, which `net/http` cancels the instant the browser disconnects. A
refresh during the multi-second model call aborted every step after the LLM call
— tool effects, `studio_state`, and the assistant-reply persist — leaving the
already-saved student message with no answer.

**Fix:** the turn now runs on `context.WithoutCancel(r.Context())` with a 150s
cap (`coach.go`), so it completes and persists **regardless of whether the
student is still connected**; the JSON response write is best-effort (fails
harmlessly if they left). Applied to the main `postCoach`; `postCoachStart` /
`postCoachAdvance` re-fire idempotently on reload, so they didn't need it.

**Verified live (after fix):** sent a coach message, reloaded ~immediately
(mid-reply), waited, reloaded again — the AI's full on-topic reply is now
present (before the fix this exact sequence left an orphan message).

## EDGE-04 · Canned reply masks a real model failure → FIXED (`d5b86fff`)

**The worry (user, 2026-08-25):** when the model call genuinely fails, the coach
returned a 200 with a canned "先自己说说看——现在你最想弄清楚的是哪一点？", which
reads to the student as 印记 being evasive/broken.

**Fix:** a genuine model failure (`cerr != nil` from `ProposeStatusTurn` —
transport error or unparseable output) now returns a real `502
ai_dialogue_failed`, never a fabricated reply — same posture as the 提问卡 path,
per the firm rule "AI-dialogue errors must be SURFACED, never masked." The FE
already catches this and shows an honest "（…我没接住——再试一次？）" retry note
(`WorkspaceContainer.tsx`); the student turn is persisted so a resend self-heals.
A **successful-but-empty** turn (e.g. tool-only, no prose) still uses the
restrained nudge — only real failures error. Applied to `postCoach` /
`postCoachStart` / `postCoachAdvance`. The two sub-agent coaches (find_sources /
reflection) and card-reflect keep their fallback pending separate FE
error-state verification.

**Verified:** `TestPostCoach_ModelFailureSurfacesError` (malformed model output →
502 `ai_dialogue_failed`, no canned text); full coach api suite green.

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
