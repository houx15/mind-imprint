# Slice 1 — Card Re-catalog Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development or superpowers:executing-plans to implement task-by-task. Steps use checkbox (`- [ ]`) syntax.

**Goal:** Re-classify and re-bind the 34-card thinking-card library to the finalized model in the architecture spec — a lean per-status deck, an AI-offered reading toolkit, a cross-cutting on-demand pool — and tag every card with a first-class `interaction` type. **No card content changes and no deletions** (the toulmin+argument-map merge is dropped per user decision — see "Card merge decision"). **Re-classification + re-binding only; sub-agent/function renderers are deferred.**

**Architecture:** Card JSON in `packages/contracts/cards/*.json` is the single source of truth (Go mirrors it in `apps/api/internal/cards/specs/*.json`). Runtime binding lives in Go: `StatusRegistry().Cards` per `FlowStatus` (writing-flow decks), `ReadingDeckIDs` (reading room), and two NEW lists — `ReadingToolkitIDs` (relocated source-analysis tools) + `CrossCuttingCardIDs` (on-demand pool). `summon_card` becomes valid for `status-deck ∪ cross-cutting`.

**Tech Stack:** TypeScript + Zod (`packages/contracts`), Go (`apps/api`, sqlc unaffected), Vitest, Go test (testcontainers, foreground).

## Global Constraints

- **Card spec single source of truth = `packages/contracts`.** The Go `internal/cards/specs/*.json` is a hand-kept lockstep mirror — every JSON change here is made in BOTH copies, identical bytes for shared fields.
- **Registry count stays 34** (no deletions this slice). `packages/contracts/test/library.test.ts`'s count assertion is unchanged.

## Card merge decision (dropped)

The finalized model originally consolidated `toulmin` + `argument-map` → one card (34→33). Investigation showed BOTH are deeply wired — graph-effects (`card_effects.go`), the reading→writing classifier that auto-summons + mints a claim node (`classifier.go`), a seeded course (`seed_courses.go`), the writing shelf (`WritingBlock.tsx`), the summon allowlist (`card_persist.go`), and ~15 tests. **User decision: keep both, no merge.** essay·claim offers both. The physical merge, if ever done, is its own carefully-tested slice. **Task 1 below is dropped.**
- **No renderer work this slice.** All cards still render as forms (`CardRenderer`). We only add the `interaction` *tag*; `question-card`'s sub-agent modal and `learning-report`'s function generator are later slices.
- **铁律②:** cards are offered/summoned, opened by the student — this slice changes *which* cards are offered where, never auto-opens them.
- Go api suite runs foreground (testcontainers). `CGO_ENABLED=0` not needed here (no sqlc regen).
- Deploy = commit to main + push, then `.deploy-local/deploy.sh <web|api|full>` (resets server to origin/main).

## Finalized card model (target state — 34 cards, no deletions)

**Per-status decks** (`StatusRegistry().Cards`) — doc-faithful (doc §2/§4/§6/§7 Cards fields):
- `framework`: `question-card` (sub-agent) — **only**, per doc §2
- `topic`: folded into framework per spec decision E (Task 5)
- `proposal`: **none** (doc §4)
- `essay`: `pee`, `toulmin`, `argument-map` (both argument cards kept — user decision; together they fill doc §6's 论证解剖 role)
- `review`: **none summonable** (`learning-report` is a function-type producer, not a summon deck entry)

**Reading room** (`ReadingDeckIDs`, unchanged core): `craap`, `sift`, `lens-logic`, `lens-methods`, `lens-society`, `lens-law`, `lens-economics`, `lens-ethics`, `lens-history`, `lens-communication`, `lens-systems` (11).

**Reading toolkit** (NEW `ReadingToolkitIDs`, AI-offered when a source is open — wiring deferred, tagging only): `cda`, `money-trail`, `multimodal-decode`, `spin-detector`, `data-literacy`, `fact-opinion-value`, `opcvl`, `belief-spectrum` (8). Plus `search-plan` → AI-side (reading), removed from student decks.

**Cross-cutting on-demand** (NEW `CrossCuttingCardIDs`, summonable regardless of status): `ai-boundary`, `knower-perspective`, `metacognition`, `emotional-alignment`, `rabbit-hole`, `ethics-lenses`, `ai-decision-tree`, `perspective-matrix`, `concession` (9). *(perspective-matrix + concession moved here from status decks to stay doc-faithful to §2/§6.)*

**Merge:** dropped — `toulmin` and `argument-map` both kept unchanged.

**Interaction tags:** `question-card` = `sub-agent`; `learning-report` = `function`; all others = `form`.

**Placement tag** on each card JSON: `"status"` | `"reading"` | `"reading-toolkit"` | `"cross-cutting"` — documentation + gallery + a drift test against the Go lists.

Count check: status(question-card, pee, toulmin, argument-map, learning-report = 5) + reading(11) + reading-toolkit(9 incl. search-plan) + cross-cutting(9) = 34. ✓

---

## Task 1: ~~Consolidate toulmin + argument-map~~ — DROPPED

Per the "Card merge decision" above (user chose keep-both), there is no merge. Both cards stay unchanged. Slice 1 begins at Task 2.

## Task 2: First-class `interaction` field on the card schema

**Files:**
- Modify: `packages/contracts/src/cardSpec.ts`
- Modify: card JSON `question-card` (→ `sub-agent`) + `learning-report` (→ `function`), both contracts + Go mirror copies
- Modify: `apps/api/internal/cards/loader.go` only if it strictly validates unknown fields (verify; likely tolerant)
- Test: `packages/contracts/test/library.test.ts`

**Interfaces:**
- Produces: `interaction?: "form" | "sub-agent" | "function"` on `CardSpec` (default treated as `form` when absent). The deprecated `interaction_type` (Chinese enum) stays untouched for now (removed in a later cleanup slice).

- [ ] **Step 1: Failing test** — assert `CARD_REGISTRY["question-card"].interaction === "sub-agent"`, `CARD_REGISTRY["learning-report"].interaction === "function"`, and a form card (`pee`) is `"form"` or undefined.
- [ ] **Step 2:** run vitest — FAIL (field not in schema / not tagged).
- [ ] **Step 3:** add `interaction` to `cardSpec.ts` Zod schema; tag `question-card` + `learning-report` in both JSON copies; verify Go loader accepts the new field (add to its struct if it uses a strict typed unmarshal — check `loader.go`).
- [ ] **Step 4:** run vitest + `cd apps/api && go build ./...` — PASS.
- [ ] **Step 5:** Commit `feat(cards): first-class interaction type (form/sub-agent/function)`.

## Task 3: Placement tag + binding lists (reading-toolkit, cross-cutting) + summon validation

**Files:**
- Modify: all 34 card JSON (add `placement`), both copies — scripted, see Step 3
- Modify: `apps/api/internal/agent/studioflow.go` (rewire `StatusRegistry().Cards`; add `CrossCuttingCardIDs`; extend summon validity)
- Modify: `apps/api/internal/agent/reading_deck.go` (add `ReadingToolkitIDs`)
- Modify: `packages/contracts/src/cardSpec.ts` (add `placement` enum)
- Test: `apps/api/internal/agent/studioflow_test.go`, `packages/contracts/test/library.test.ts`

**Interfaces:**
- Produces: `placement?: "status" | "reading" | "reading-toolkit" | "cross-cutting"` on `CardSpec`; Go `CrossCuttingCardIDs []string`, `ReadingToolkitIDs []string`; a helper `IsSummonable(cardID string, status FlowStatus) bool` = `cardID ∈ StatusRegistry()[status].Cards ∪ CrossCuttingCardIDs`.
- Consumes: existing `StatusRegistry()` / `ReadingDeckIDs`.

- [ ] **Step 1: Failing test** (Go) — table asserting the finalized decks: `framework` = {question-card}; `proposal` = {} ; `essay` = {pee, toulmin, argument-map}; `search-plan` NOT in any status deck; `CrossCuttingCardIDs` contains `ai-boundary`, `perspective-matrix`, `concession`; `IsSummonable("perspective-matrix", FlowProposal)` is true; `IsSummonable("cda", FlowEssay)` is false.
- [ ] **Step 2:** run `cd apps/api && go test ./internal/agent/ -run TestStatusRegistry -run TestCross...` — FAIL.
- [ ] **Step 3:** rewire `StatusRegistry().Cards` to the finalized decks (framework = {question-card}; proposal `Cards: nil`; essay = {pee, toulmin, argument-map}; review `Cards: nil`); drop `search-plan`/`craap`/`sift`/`perspective-matrix`/`cda` from the writing decks; add `CrossCuttingCardIDs` (incl. perspective-matrix + concession) + `ReadingToolkitIDs` + `IsSummonable`; add `placement` to `cardSpec.ts`; tag every card JSON (both copies) via a script (status/reading/reading-toolkit/cross-cutting per the model).
- [ ] **Step 4:** add a contracts drift test: every card's `placement` tag agrees with the Go lists (encode the expected placement map in the test, or read a shared JSON) — and a Go test that `placement`-tagged ids partition cleanly (no id in two buckets). Run vitest + `go test ./internal/agent/`.
- [ ] **Step 5:** Commit `feat(cards): placement taxonomy + reading-toolkit + cross-cutting pool + summon validity`.

## Task 4: Wire summon validity into the coach turn

**Files:**
- Modify: `apps/api/internal/api/coach.go` (where `summon_card` tool effects are applied) or `orchestrator.go` (`FilterToolsForStatus` / tool application)
- Test: `apps/api/internal/api/coach_orchestrator_test.go`

**Interfaces:**
- Consumes: `agent.IsSummonable`. A `summon_card` for a card not summonable in the current status is dropped (logged), mirroring `FilterToolsForStatus`'s drop-unknown posture.

- [ ] **Step 1: Failing test** — a framework-status coach decision that summons `cda` (a reading-toolkit card) is dropped; one that summons `ai-boundary` (cross-cutting) or `question-card` (framework deck) is kept.
- [ ] **Step 2:** run `cd apps/api && go test ./internal/api/ -run TestPostCoach` — FAIL.
- [ ] **Step 3:** gate applied `summon_card` cards through `agent.IsSummonable(cardID, status)`; drop + log otherwise.
- [ ] **Step 4:** run the full `go test ./internal/api/ ./internal/agent/` (foreground) — PASS (except the known `TestWeeklyReportForSeededClass` flake).
- [ ] **Step 5:** Commit `feat(coach): gate summon_card by status deck ∪ cross-cutting`.

## Task 5: Fold `topic` into `framework` (spec decision E)

**Files:**
- Modify: `apps/api/internal/agent/studioflow.go` (`StatusForStage`: `topic_discussion` → `FlowFramework`; keep `FlowTopic` const only if still referenced)
- Modify: `apps/api/internal/api/coach.go` / start handler (the 开始 gate lands stage at framework's floor, not topic)
- Test: `apps/api/internal/agent/studioflow_test.go`, coach start test

**Interfaces:**
- Produces: `StatusForStage("topic_discussion") == FlowFramework`; the start gate opens directly on framework (目标).

- [ ] **Step 1: Failing test** — `StatusForStage("topic_discussion")` returns `FlowFramework`; a freshly-started project's derived status is `framework`.
- [ ] **Step 2:** run `go test ./internal/agent/ -run TestStatusForStage` — FAIL (currently maps to FlowTopic).
- [ ] **Step 3:** collapse `topic_discussion` → `FlowFramework` in `StatusForStage`; point the start gate at framework's `StageFloor`; remove the now-dead `FlowTopic` deck entry (keep the const if other code references it, else delete). Verify `POST /coach/start` opens forming/framework.
- [ ] **Step 4:** run `go test ./internal/agent/ ./internal/api/ -run 'TestStatusForStage|TestPostCoachStart'` — PASS.
- [ ] **Step 5:** Commit `refactor(studioflow): fold topic into framework (start → 目标)`.

## Task 6: Card gallery / catalog reflects placement

**Files:**
- Modify: `packages/contracts/src/registry.ts` (`deriveCatalog` — include `placement` + `interaction`)
- Modify: web card gallery (图鉴) grouping if it groups by the old `category` — verify `apps/web/src/**` gallery reads (search for the catalog/gallery component)
- Test: `packages/contracts/test/*` + web gallery test if one exists

**Interfaces:**
- Produces: `Catalog` rows carry `placement` + `interaction`; the 图鉴 still renders all 34 (no card vanishes from the gallery — placement is a grouping, not a filter).

- [ ] **Step 1: Failing test** — `deriveCatalog()` rows include `placement` and `interaction`; catalog length 34.
- [ ] **Step 2:** run vitest — FAIL.
- [ ] **Step 3:** add the two fields to `deriveCatalog`; confirm the web gallery still lists all cards (adjust grouping label only if it referenced a removed card).
- [ ] **Step 4:** run vitest + `cd apps/web && npx tsc -p tsconfig.json --noEmit` — PASS.
- [ ] **Step 5:** Commit `feat(cards): catalog carries placement + interaction`.

## Task 7: Full-suite green + deploy + live smoke

- [ ] Run `cd packages/contracts && npx vitest run`; `cd apps/web && npx tsc --noEmit && npx vitest run`; `cd apps/api && go test ./internal/agent/ ./internal/api/ ./internal/cards/` (foreground). All green except the known `TestWeeklyReportForSeededClass` flake.
- [ ] Commit to main + push; `.deploy-local/deploy.sh full`.
- [ ] Live smoke (real frontend, `?trial=1`): a fresh project reaches `framework` on 开始 (no topic limbo); the framework coach offers only question-card; the essay room offers pee + toulmin + argument-map; a cross-cutting card (e.g. ai-boundary or concession) is summonable mid-writing; the 图鉴 still shows all 34 cards. 0 console errors.

**Acceptance:** the registry stays 34 cards; every card carries `interaction` + `placement`; per-status decks match the finalized model (essay = pee + toulmin + argument-map); search-plan is AI-side; cross-cutting cards summon regardless of status; source-analysis tools no longer appear in writing-flow decks; the gallery still shows every card.

---

## Self-Review

- **Spec coverage:** re-catalog buckets (Task 3), merge dropped (see Card merge decision), interaction types (Task 2), placement + reading-toolkit + cross-cutting (Task 3), summon gating (Task 4), topic-merge E (Task 5), gallery (Task 6), deploy/smoke (Task 7). ✓
- **Types:** `interaction`, `placement`, `CrossCuttingCardIDs`, `ReadingToolkitIDs`, `IsSummonable` used consistently across tasks.
- **Placeholders:** none — each task names exact files, the finalized card lists are in "Finalized card model," and every code step has a concrete assertion.
- **Mirror discipline:** every JSON change is called out as "both copies" (contracts + Go), with the count test as the backstop.
