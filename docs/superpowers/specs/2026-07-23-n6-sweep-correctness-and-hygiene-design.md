# N6 Sweep · Correctness & Hygiene — Design

> N6 is a **bucket**, not a slice: ~25 heterogeneous `[N6]` items scattered
> across every slice of the refactor. This slice ships the **small,
> low-risk, low-product-visibility** subset — a handful of correctness fixes
> that keep metering and the process record honest, plus test-infra hygiene.
> The one big, exciting N6 item — **template-driven personalized journeys**
> (was "flagship planner judgment") — is deferred to its own brainstorm.

Date: 2026-07-23
Branch: `slice-n6-sweep-correctness-hygiene`

---

## 1. Motivation & scope

The `[N6]` tag was a catch-all for "pure platform/infra, low product-visibility,
can run anytime." Read closely, its items fall into five groups of very
different character and risk:

- **Correctness** — bugs where the platform silently does the wrong thing
  (this slice).
- **Hygiene** — test-infra cleanliness and dead code (this slice).
- **Entitlement seam** (`HasEntitlement`) — **dropped from scope.** It is a
  deliberate stub with ~18 call sites already threaded; there is no billing or
  quota model to plug in yet, so making it "real" now would be premature
  invention. It stays `return true, nil` until a membership model is designed.
- **Async assessment (river worker)** — **deferred.** Inline assessment works
  at today's single-mock-student scale; async only matters under load that does
  not exist yet, and it should not be built until the evaluation model (metrics
  + how they are calculated) is finalized.
- **Template-driven personalized journeys** — **deferred to its own slice.**
  The current writing-project stations (S0–S6) become a *template*; an LLM reads
  where a student actually is and composes a personalized journey rather than
  forcing everyone through all seven stations, with more templates over time.
  This is a genuine product/AI design project and gets its own
  brainstorm → spec → plan. It is explicitly **not** in this sweep.

Everything below is self-contained, needs **no migration** (except H5, which
only adds a *test*), **no new LLM subagent**, and **no schema change**.

### Two stale-tracker corrections found while scoping

Reading the code before writing this spec corrected two tracker claims (both
in `docs/2026-07-20-student-platform-remaining-work.md`, written 2026-07-20):

1. **The "CostNumeric latent bug" is not a value-bug.** The three agent-side
   recorders (`agentstore`/`chatstore`/`coursestore`) all write to the
   `llm_call` table, whose `cost_estimate` is **NOT NULL** — so NULL cannot be
   stored and `CostNumeric(cost, true)` (explicit `$0.00` + a `slog.Warn`) is
   the only legal choice. The `api/*` recorders write to the **nullable**
   `evaluation` column, so `CostNumeric(cost, priced)` (NULL when unpriced) is
   correct *there*. Each call site is already right for its own column type.
   C5 is reshaped accordingly (pin the semantics, do not "fix" a non-bug).
2. **"No migration-Down test anywhere repo-wide" is stale.** `migrate_0024`–
   `migrate_0027_test.go` already round-trip goose Down/Up. H5 extends that
   existing pattern to an older migration, it does not establish it.

---

## 2. Correctness items

### C1 — `no_unsupported_claim`: warn, do not block

**Today:** `no_unsupported_claim` is a machine gate item on `build_argument`. It
is a *negation scan* over `claim` nodes, so with **zero** claims it is trivially
true — a student who skips Toulmin entirely never mints a claim, yet
`build_argument` still reaches `machine_clear`. The item is meant to catch an
*unsupported* claim; instead it silently passes an *argument-free* station.

**The decision (铁律 2 — an offer is never a wall):** an argument with a hole in
it should be *surfaced*, not *blocked*. The gate item stops being a hard machine
gate; the coach instead raises a non-blocking prompt when the argument has no
claim yet (e.g. 「你的论证里还没有立论 — 确定要继续吗？」) and the student may proceed.

**Shape:** the exact mechanism (drop the item vs. keep it as an advisory that
never contributes to `Missing`) is a plan-time detail; the invariant is that
`build_argument` no longer *silently* clears with no argument, and the student
is never *blocked* by it. No new gate-predicate kind. The warning is coach
prose, not a new machine verdict.

### C2 — card activate/skip: one transaction

**Today:** `skipProjectCard` (`api/projectcards.go`) does three separate DB
writes back-to-back, each its own implicit transaction: persist the envelope
(`SubmitProjectCardInstance`) → set status `skipped` (`SetCardInstanceStatus`)
→ append the `card_skipped` event. A crash between them leaves a card with a
saved answer but status still `active`, or a skip with no event in the process
log (过程即数据 corrupted). `activateProjectCard` has the smaller version (status
then event; the event append is already best-effort/logged).

**Fix:** wrap the multi-write state change in a single DB transaction so it is
all-or-nothing. The `sqlcAgentStore` already carries a `*pgxpool.Pool`
(`NewSqlcAgentStore(q, pool)`), so a `pgx.Tx`-scoped path is available without
new plumbing. Event-append that is *intentionally* best-effort (logged, never
fatal) stays best-effort — the transaction covers the state that must not
diverge (envelope + status), not the advisory event.

### C3 — `Advance`: gate-state + event in one transaction

**Today:** `Advance` (`agent/planner.go`) sets `rec.Confirmed = true` and calls
`UpsertGateState`, then *separately* appends the `gate_attempt result:passed`
event. If the event append fails it returns an error — but the gate is already
committed solid, leaving the process tree with no record the gate ever passed.
Same class as C2.

**Fix:** wrap `UpsertGateState` + the passed-event append in one transaction.
The blocked path (`result:blocked`, no upsert) is a single write and needs no
change. `AdvanceAll` calls `Advance` per contract; each stays atomic
individually (a partial walk is already tolerated by design — it re-runs on the
next gate-affecting write).

### C4 — bound classifier spend

**Today:** `semanticCardCandidate` (`agent/loop.go`) runs the moment-classifier
(`ClassifyMoment`, a 5th subagent) on every student turn ≥ `MinClassifyRunes`
**as long as any moment is still eligible**. A moment retires only when its card
is *used*; if a student's writing never trips that moment, the card is never
surfaced, the moment stays eligible for the life of the project, and the
classifier fires **every turn forever**, answering "none" (which retires
nothing). This roughly doubles per-turn LLM calls (coach + classifier) in a
mature project. Cheap-tier, but unbounded.

**Fix:** cap classifier spend per project. The natural counter already exists —
every classifier call is recorded as an `llm_call` with `Purpose = "classify"`.
Add a query that counts `classify` calls for a project and short-circuit
`semanticCardCandidate` (return no-candidate, no call) once the count reaches a
cap. The cap value is a plan-time constant with a documented rationale (a
generous ceiling — the moments that will trip naturally do so early). This is a
spend backstop, not a behavior change for normal projects; it never blocks a
card that would have surfaced within the cap.

**What it does NOT do:** it does not add per-moment backoff state or a new
table. Counting the existing `classify` ledger rows is the whole mechanism.

### C5 — metering accuracy

Two small, related touches, no schema change:

- **Close the empty-result metering leak.** `surfaceAnchors`
  (`api/studioturn.go`) records the `llm_call` *after* an early `return` on
  `len(result.Anchors) == 0` — so a real anchor-generation call that happens to
  return zero usable anchors is never metered. `renderChallenge`
  (`agent/course.go`) has the analogous shape. Move/duplicate the
  `RecordLLMCall` so a call with a populated `Resolved` is metered **before**
  the empty-result bail — mirroring the already-unconditional record for the
  non-empty path.
- **Pin the CostNumeric semantics with a test** (not a "fix" — see §1). Add a
  test asserting the intended split: `llm_call` records an explicit `$0.00`
  (Valid, zero) for an unpriced model, while the nullable `evaluation` path
  records NULL (`!Valid`). Add the missing one-line explanatory comment to
  `chatstore`/`coursestore` (the comment `agentstore` already carries), so the
  `CostNumeric(cost, true)` at those sites is not misread as the `api/*` bug it
  is not.

---

## 3. Hygiene items

### H1 — move `apps/web` tests into a `test/` tree

**Today:** `apps/web/src` interleaves **113** `*.test.tsx`/`*.test.ts` files
with source (`StudioToulminCard.test.tsx` sits beside `StudioToulminCard.tsx`),
which clutters every source folder. `packages/contracts` already does the clean
thing — all tests under `test/`. This item brings `apps/web` to the same
convention.

**Change:** relocate the co-located web test files into a mirrored
`apps/web/test/` tree (e.g. `src/studio/Foo.test.tsx` →
`test/studio/Foo.test.tsx`), update the Vitest `include` globs, and rewrite each
moved file's **relative imports** to reach back into `src/`. The proof of
correctness is the unchanged behavior: the full `npm test` and `npx tsc
--noEmit` pass after the move with the same test count.

**Risk & scope note:** this is the single largest, most mechanical item in the
slice (113 files + import rewrites + config). It is **independent** of every
correctness item and is planned as its own task group at the end, so it can be
split into its own follow-up if it balloons without affecting C1–C5/H2–H5.
Co-location is a common React convention; this move is a deliberate choice for
repo-wide consistency with `packages/contracts`, per the maintainer's request.

### H2 — delete dead `gen-go-fixtures.ts`

`apps/web/scripts/gen-go-fixtures.ts` is referenced by no `package.json` script
and nothing else in the repo (verified). Delete it.

### H3 — fix the pre-existing contracts `tsc` failure

`npx tsc --noEmit` in `packages/contracts` fails with one error:
`test/interactionPrimitive.test.ts(49,12): error TS2532: Object is possibly
'undefined'`. Fix the single unsafe access (a guard or non-null assertion
consistent with the surrounding test style) so `tsc` is clean and can be a
reliable gate.

### H4 — de-flake `ChatSurface` / cross-pane tests

`apps/web/src/shell/chat/ChatSurface.test.tsx` has a load-sensitive 5s-timeout
flake, and the N3c cross-pane tests were ~50% flaky under load (they awaited a
project-projection label, then synchronously queried a conversation-snapshot
button). Make these deterministic (await the actual condition each assertion
depends on, raise/remove brittle fixed timeouts) so `npm test` is a trustworthy
gate. Scope is limited to the known-flaky files; no unrelated test churn.

### H5 — extend migration-`Down` coverage to a known-risky migration

The `migrate_0024`–`0027_test.go` pattern (goose `DownTo`/`Up` round-trip
against a real pool) already exists. Extend it to the migration the tracker
flagged as having a fragile down — `0017_graph_node_open_type` (and any sibling
whose down is non-trivial) — following the identical pattern. This is additive
test coverage only: no migration SQL changes, no new schema. If 0017's down is
found genuinely broken (not just untested), that is a real correctness finding
surfaced by the test and fixed in the same task; if it round-trips cleanly, the
test simply pins it.

---

## 4. Testing

- **Go (`apps/api`, full packages, `-p 1`, never `-run` subsets — these touch
  agent/gate/planner/store):**
  - C1: `build_argument` no longer machine-clears with zero claims via the
    silent path; the coach raises the advisory; the student is not blocked.
  - C2/C3: a forced mid-sequence failure leaves no divergent state (envelope
    without status, or gate solid without a passed event); the happy path is
    unchanged.
  - C4: past the cap, `semanticCardCandidate` makes no classifier call
    (assert via the `classify` `llm_call` count not increasing); under the cap,
    behavior is unchanged.
  - C5: the empty-anchor path records an `llm_call`; the CostNumeric split test
    (llm_call `$0.00` vs. evaluation NULL for an unpriced model).
  - H5: the 0017 down/up round-trip test passes (or fails loudly, then fixed).
- **Web (`apps/web`, `npm test` + `npx tsc --noEmit`):**
  - H1: full suite + tsc pass after the move, same test count.
  - H4: the de-flaked files pass deterministically (re-run stability).
- **Contracts (`packages/contracts`, tests in `test/`):**
  - H3: `npx tsc --noEmit` is clean.

Registry-enumeration note: this slice adds **no card and no skill phase**, so it
should not trip the registry-count tests — but any task that touches
`packages/contracts` still runs the **full** contracts suite, not a `-run`
subset (the standing rule).

---

## 5. What this slice is NOT

- **Not** the template-driven personalized journey (N6-E) — its own slice.
- **Not** async assessment / river worker (deferred to post-evaluation-model).
- **Not** a real `HasEntitlement` (dropped; stays a stub until a billing model
  exists).
- **No** new LLM subagent, **no** schema/migration change (H5 adds a test only),
  **no** card/skill JSON change, **no** machine-set `solid`.
- Correctness items are **surgical** — each is a bounded transaction wrap,
  spend cap, advisory-not-block, or metering-record move. None reshapes a
  station chain, a card, or a projection.

---

## 6. Deferred / carried forward (not this slice)

From the same `[N6]` bucket, left for later and *not* silently dropped:
- **N6-E** template-driven personalized journeys — own brainstorm.
- **N6-C** async assessment (river) — after the evaluation model is finalized.
- **N6-B** `HasEntitlement` real seam — after a billing/quota model exists.
- The **L1 wrong-material `blockLookup`** bug (N3c carry-forward) — it changes
  what every *first-time* student sees, so it needs its own design call, not a
  surgical sweep fix.
- `renderCompletedCard`/`BuildCoachContext` refeed-loop duplication;
  `CandidateMoves` `D5`-vs-`D3` mislabel; event-model normalization; the
  `偏弱` verdict chip's honest producer; `output_check_verdict` enforcement
  breadth; the assorted cosmetic items (voice-state reset, `ChatCardOfferDTO`
  parity, stale comments) — each rides the next slice that touches its code,
  or a future dedicated hygiene pass.
