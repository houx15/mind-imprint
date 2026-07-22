# N3c · Guidance fade + student span-creation — implementation plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make the L1→L2→L3 guidance ladder live — the AI stops handing the
student the sentence, then stops handing her the question — driven by a real
per-student fade, with student text-selection span creation.

**Architecture:** The level is **derived from the anchors as surfaced**, not
stored: `author:"ai"` → L1; `author:"student"` + question → L2; `author:"student"`
+ blank question → L3. The fade is computed at surface time from her completed
uses of that card. No migration, no new API field, no new column.

**Tech Stack:** Go (`net/http`, `pgx`, `sqlc`), React + Vite + TypeScript, Zod
contracts shared front/back.

**Spec:** `docs/superpowers/specs/2026-07-22-n3c-guidance-fade-and-span-creation-design.md`
— read the section named in each task before starting it.

## Global Constraints

- **The client never calls a model.** All LLM calls go through `apps/api`; keys
  live only in server env and must never enter git, logs, thrown or rendered
  errors, stored data, or eval payloads.
- **Every LLM call is metered** (档位 + token + 成本) — including calls whose
  output is rejected. At L3 **no call is made**, so there is nothing to meter;
  do not add a metering call for a call that did not happen.
- **铁律 1 (AI 克制):** the AI never authors the student's content or draws her
  conclusion.
- **铁律 2 (不操纵):** no streaks, badges, levels-as-rewards, leaderboards,
  celebration, or comparison to others. **The fade is silent** — instructional
  copy only, never congratulatory. **An offer is never a wall:** locating a span
  is never required by any server-side completion predicate.
- **铁律 3 (一次只问一个).**
- **铁律 4 (过程即数据):** a student who cannot find a sentence is recorded, not
  blocked.
- **No migration in this slice.** If you believe you need one, stop and report
  BLOCKED.
- **Card JSON is authored only in `packages/contracts/cards/`** and mirrored by
  `cd apps/api && make sync-cards`. **Never hand-edit
  `apps/api/internal/cards/specs/`.** (This slice should not need any card JSON
  change at all.)
- `make sqlc` from `apps/api` after editing `internal/store/queries/*.sql`.
  **Never hand-edit `apps/api/internal/store/sqlc/*`.**
- **Icons are inline SVG.** Never import `lucide-react`.
- **`Anchor.start`/`Anchor.end` are rune (Unicode code point) indices** after
  Task 1. Never `String.prototype.slice` them in TS; never `strings.Index` them
  as bytes in Go.
- Go tests: `DOCKER_HOST=unix:///var/run/docker.sock CGO_ENABLED=0 go test -p 1 ./...`
  from `apps/api`. **Run FULL packages, never `-run` subsets** — a card, gate, or
  projection change that passes a filtered subset has told you nothing.
- Web tests: `npm test` and `npx tsc --noEmit` from `apps/web`. Contracts tests
  live in `packages/contracts/test/`, not `src/`.
- **Never `git add` a whole directory.** The working tree has pre-existing
  unrelated changes (`M package.json`, untracked files under `docs/` and the
  repo root) that are NOT part of this slice. Stage named files only.
- **Check any new test fake against the REAL interface it doubles.** The
  documented root cause of five Criticals in an earlier slice was mock
  infidelity — fakes encoding shapes the backend cannot produce, so a green
  suite confirmed a belief instead of testing the code.

---

### Task 1: Rune offsets in Go

**Read first:** spec §7.1.

**Files:**
- Modify: `apps/api/internal/agent/anchors.go` (`computeOffsets`, ~line 91)
- Modify: `apps/api/internal/agent/anchors.go` (`Anchor` struct doc, ~line 13)
- Test: `apps/api/internal/agent/anchors_test.go`

**Interfaces:**
- Consumes: nothing from other tasks.
- Produces: `computeOffsets(text, quote string) (int, int)` returning **rune**
  indices. Signature unchanged; semantics changed.

- [ ] **Step 1: Write the failing test**

Add to `anchors_test.go` a test named `TestComputeOffsets_RuneIndicesOnCJK`.
It must assert, on Chinese text where byte and rune indices differ:

```go
text := "过去二十年里发生了一件事：地球比 2000 年绿了一圈。"
quote := "地球比 2000 年绿了一圈"
start, end := computeOffsets(text, quote)
```

Assert `start` and `end` are **rune** indices — derive the expectation with
`utf8.RuneCountInString`, not a hardcoded number, and additionally assert the
round trip: `string([]rune(text)[start:end]) == quote`. That round-trip
assertion is the real contract; a test that only checks a magic number would
pass against a wrong convention.

Also assert the not-found case still returns `(0, 0)`.

- [ ] **Step 2: Run it and watch it fail**

```
cd apps/api && DOCKER_HOST=unix:///var/run/docker.sock CGO_ENABLED=0 go test ./internal/agent/
```
Expected: FAIL — the round trip produces the wrong substring (byte offsets).

- [ ] **Step 3: Implement**

`computeOffsets` still finds the quote with `strings.Index` (byte index `i`),
then converts before returning:

```go
// Returns RUNE (Unicode code point) indices, not byte offsets: the web
// consumes these to slice the same block text, and one CJK character is
// 3 bytes but 1 rune. Returning byte offsets highlighted the wrong
// sentence on every Chinese material (spec §7.1). (0,0) when not found —
// quote stays authoritative for the UI.
func computeOffsets(text, quote string) (int, int) {
	i := strings.Index(text, quote)
	if i < 0 {
		return 0, 0
	}
	start := utf8.RuneCountInString(text[:i])
	return start, start + utf8.RuneCountInString(quote)
}
```

Add `"unicode/utf8"` to the imports. Add to the `Anchor` struct doc comment:
`Start`/`End` are rune indices into the block's text.

- [ ] **Step 4: Run the full package**

```
cd apps/api && DOCKER_HOST=unix:///var/run/docker.sock CGO_ENABLED=0 go test -p 1 ./internal/agent/ ./internal/api/
```
Expected: PASS. If an existing test hardcoded byte offsets, fix the test's
expectation (it encoded the bug) — do not revert the change.

- [ ] **Step 5: Commit** (`fix(n3c): anchor offsets are rune indices, not bytes`)

---

### Task 2: Rune-safe span slicing on the web

**Read first:** spec §7.1.

**Files:**
- Modify: `apps/web/src/primitives/annotate/segment.ts`
- Modify: `packages/contracts/src/anchor.ts` (doc comment only)
- Test: `apps/web/src/primitives/annotate/segment.test.ts`

**Interfaces:**
- Consumes: Task 1's rune-index convention.
- Produces: `segmentBlock` unchanged in signature; slices by code point.

- [ ] **Step 1: Write the failing test**

Add `segmentBlock` cases named for what they prove — a CJK block whose span
covers a mid-string phrase, asserting the marked run's `text` is exactly that
phrase. Use Chinese text long enough that byte-vs-code-point divergence would
produce a visibly different substring. Include one non-BMP (emoji) case so the
`Array.from` behavior is pinned rather than incidental.

- [ ] **Step 2: Run it and watch it fail**

```
cd apps/web && npm test -- segment
```
Expected: FAIL — the run text is a mis-sliced substring.

- [ ] **Step 3: Implement**

In `segmentBlock`, convert once and slice the array:

```ts
// Offsets are RUNE (code point) indices — Go writes them with
// utf8.RuneCountInString (spec §7.1). String.prototype.slice counts UTF-16
// code units, which diverges from runes for every non-BMP character, so
// slice the code-point array instead.
const chars = Array.from(text);
const len = chars.length;
const sub = (from: number, to: number) => chars.slice(from, to).join("");
```

Replace every `text.slice(...)` with `sub(...)` and every `text.length` with
`len`. The whole-block default (a span with no `range`) still spans `0..len`.

Add a one-line comment to the Zod `Anchor` in `packages/contracts/src/anchor.ts`
stating that `start`/`end` are rune indices into the block's text.

- [ ] **Step 4: Run the full web + contracts suites**

```
cd apps/web && npm test && npx tsc --noEmit
cd packages/contracts && npm test
```
Expected: PASS (contracts has one known pre-existing failure,
`test/interactionPrimitive.test.ts(49,12) TS2532` — leave it).

- [ ] **Step 5: Commit** (`fix(n3c): segmentBlock slices spans by code point`)

---

### Task 3: The fade — count her completed uses of a card

**Read first:** spec §3.

**Files:**
- Modify: `apps/api/internal/store/queries/card_instance.sql`
- Regenerate: `apps/api/internal/store/sqlc/*` (via `make sqlc` — never by hand)
- Modify: `apps/api/internal/agent/agentstore.go` (store method)
- Modify: `apps/api/internal/agent/loop.go` (`AgentStore` interface, ~line 120)
- Create: `apps/api/internal/agent/guidance.go`
- Test: `apps/api/internal/agent/guidance_test.go`, plus the store test file that
  already exercises project-scoped card queries

**Interfaces:**
- Produces, consumed by Tasks 4 and 5:
  ```go
  type GuidanceLevel int
  const (
      GuidanceL1 GuidanceLevel = 1
      GuidanceL2 GuidanceLevel = 2
      GuidanceL3 GuidanceLevel = 3
  )
  func GuidanceFor(completedUses int) GuidanceLevel
  ```
  and on `AgentStore`:
  ```go
  CountCompletedCardUsesByUser(ctx context.Context, userID uuid.UUID, cardID string) (int, error)
  ```

- [ ] **Step 1: Write the failing tests**

`guidance_test.go` — `TestGuidanceFor` as a table: 0→L1, 1→L2, 2→L3, 3→L3,
7→L3, and a negative input (defensive: −1→L1, since a count can never be
negative but a clamp that silently returns 0 would be a worse bug).

For the store method, add a real-DB (testcontainers) test in the file that
already holds project-scoped card_instance store tests, named
`TestCountCompletedCardUsesByUser`. It must prove all four:
1. a `completed` project card for this user counts;
2. an `active` or `skipped` card for this user does **not** count;
3. a `completed` card with a **different** `card_id` does not count;
4. a `completed` card belonging to a **different user** does not count.

- [ ] **Step 2: Run and watch fail**

```
cd apps/api && DOCKER_HOST=unix:///var/run/docker.sock CGO_ENABLED=0 go test ./internal/agent/
```

- [ ] **Step 3: Add the query**

Append to `internal/store/queries/card_instance.sql`:

```sql
-- name: CountCompletedCardUsesByUser :one
-- How many times this student has COMPLETED this specific card, across all
-- three scopes. Same definition as ListCollectedCardsByUser above
-- (status='completed', owner-filtered through each scope's own parent join)
-- narrowed to one card_id — so the guidance fade (agent/guidance.go) counts
-- exactly what the 工具卡 tab already shows her, rather than inventing a
-- private number. A skip is a decline and does not count.
SELECT count(*)::int FROM (
  (SELECT ci.id FROM card_instances ci JOIN project p ON p.id = ci.project_id
   WHERE ci.project_id IS NOT NULL AND ci.status = 'completed'
     AND p.user_id = @user_id AND ci.card_id = @card_id)
  UNION ALL
  (SELECT ci.id FROM card_instances ci JOIN course_session cs ON cs.id = ci.session_id
   WHERE ci.session_id IS NOT NULL AND ci.status = 'completed'
     AND cs.user_id = @user_id AND ci.card_id = @card_id)
  UNION ALL
  (SELECT ci.id FROM card_instances ci JOIN chat_thread t ON t.id = ci.thread_id
   WHERE ci.thread_id IS NOT NULL AND ci.status = 'completed'
     AND t.user_id = @user_id AND ci.card_id = @card_id)
) rows;
```

Then `cd apps/api && make sqlc`.

- [ ] **Step 4: Implement `guidance.go`**

```go
package agent

// GuidanceLevel is the scaffold level a student gets on an annotate card,
// derived from how many times she has already completed that card
// (spec §3). It is never stored: it decides only what the AI fills in at
// surface time, and the anchors themselves then carry the level (§2).
type GuidanceLevel int

const (
	GuidanceL1 GuidanceLevel = 1 // AI elicits the question AND locates the span
	GuidanceL2 GuidanceLevel = 2 // AI elicits; she locates
	GuidanceL3 GuidanceLevel = 3 // she elicits AND locates
)

// GuidanceFor maps completed uses of a card onto the ladder: 0 → L1, 1 → L2,
// 2+ → L3. The fade is SILENT — no badge, no level-up, nothing celebratory
// anywhere in the UI (铁律 2). It only changes what the card asks for.
func GuidanceFor(completedUses int) GuidanceLevel {
	switch {
	case completedUses <= 0:
		return GuidanceL1
	case completedUses == 1:
		return GuidanceL2
	default:
		return GuidanceL3
	}
}
```

Add the store method to `agentstore.go` following the shape of its neighbors
(sqlc call, wrap errors the same way), and add its signature to the
`AgentStore` interface in `loop.go`. **Every existing fake implementing
`AgentStore` must gain the method** — find them all (`grep -rn "AgentStore" --include=*_test.go`)
and give each a fake that returns a settable count, not a hardcoded 0. A fake
hardcoded to 0 pins L1 forever and would make Task 5's tests vacuous.

- [ ] **Step 5: Run full packages, then commit**

```
cd apps/api && DOCKER_HOST=unix:///var/run/docker.sock CGO_ENABLED=0 go test -p 1 ./...
```
Commit (`feat(n3c): guidance ladder + per-student completed-use count`).

---

### Task 4: Level-aware anchor generation

**Read first:** spec §4.

**Files:**
- Modify: `apps/api/internal/agent/anchors.go` (`AnchorGenerator`, `Generate`,
  `buildAnchorPrompt`, `parseAnchorGen`, `fallbackAnchors`)
- Test: `apps/api/internal/agent/anchors_test.go`

**Interfaces:**
- Consumes: `GuidanceLevel` (Task 3).
- Produces, consumed by Task 5:
  ```go
  Generate(ctx context.Context, spec cards.Spec, materials []Material, level GuidanceLevel) (GenerateResult, error)
  ```

- [ ] **Step 1: Write the failing tests**

In `anchors_test.go`, against a fake provider (check it against the real
`gateway.Provider` interface before writing it):

1. `TestGenerate_L1_Unchanged` — L1 produces `author:"ai"` anchors with a
   non-empty `quote`, a resolved `block_id`, and rune offsets; assert the
   provider WAS called. This is the regression fence: L1 must not change.
2. `TestGenerate_L2_QuestionOnlyNoSpan` — every returned anchor has
   `author:"student"`, a non-empty `question`, and a blank span
   (`block_id == "" && start == 0 && end == 0` and `quote == ""`); assert the
   provider WAS called and `Resolved`/`Usage` are populated (it is still metered).
3. `TestGenerate_L3_NoModelCall` — assert the provider was **NOT** called at
   all, that one anchor exists per `spec.Params.Tags` entry, each with
   `author:"student"`, `question == ""`, blank span, and that
   `Resolved.Provider == ""` (nothing to meter).
4. `TestGenerate_L2_ParseFailureFallsBackAtL2` — a provider returning garbage
   at L2 yields fallback anchors that are still **L2-shaped**
   (`author:"student"`, question present, blank span), not L1-shaped. A
   degraded surface must degrade to the right level.

- [ ] **Step 2: Run and watch fail**

- [ ] **Step 3: Implement**

- `Generate` takes `level`. At `GuidanceL3` it returns
  `GenerateResult{Anchors: fallbackAnchors(spec, materials, level)}` **before**
  resolving a key or calling the provider — no call, no usage, nothing to
  record. Comment that this is deliberate and is not the "bail before metering"
  defect class (spec §4).
- `buildAnchorPrompt(spec, level)`: at L2 the instruction asks only for
  `{"dimension":…,"question":…}` and states the question must stand on its own
  without quoting or pointing at a specific sentence, because the student will
  find the sentence herself. Keep the closed dimension vocabulary sentence.
- `parseAnchorGen(text, spec, materials, level)`: at L2, skip `blockLookup` /
  `computeOffsets` entirely, set `Author: "student"`, `BlockID: ""`, `Quote: ""`,
  `Start/End: 0`, `MaterialID` = the single material's id (materials[0].ID; the
  card is single-material at L2 — compare cards never reach here, spec §3).
  **Keep the tag-vocabulary validation at every level.**
- `fallbackAnchors(spec, materials, level)`: at L1 unchanged; at L2 the same
  tag-prompt questions but `author:"student"` and blank span; at L3 dimension
  only with `question: ""` and `author:"student"`.

- [ ] **Step 4: Run full packages** (the call site in `studioturn.go` will not
  compile until Task 5 — update it minimally to pass `GuidanceL1` so the tree
  builds, and leave the real wiring to Task 5.)

- [ ] **Step 5: Commit** (`feat(n3c): level-aware anchor generation (L2 question-only, L3 no model call)`)

---

### Task 5: Wire the fade into the surface seam

**Read first:** spec §3, §4.

**Files:**
- Modify: `apps/api/internal/api/studioturn.go` (`surfaceAnchors`, ~line 298)
- Test: `apps/api/internal/api/studioturn_test.go`

**Interfaces:**
- Consumes: `GuidanceFor`, `CountCompletedCardUsesByUser` (Task 3); `Generate`'s
  new `level` parameter (Task 4).

- [ ] **Step 1: Write the failing test**

Add `TestSurfaceAnchors_FadesWithCompletedUses` to the existing real-DB studio
turn test file. Drive the SAME user surfacing the SAME card with 0, then 1, then
2 prior `completed` instances seeded in the DB, and assert the persisted
`card_instance.anchors` are L1-shaped, then L2-shaped, then L3-shaped
(per the §2 table). This is the test that proves the producer is live rather
than a fixture.

- [ ] **Step 2: Run and watch fail**

- [ ] **Step 3: Implement**

In `surfaceAnchors`, after the existing material load:

```go
// The guidance fade (spec §3): the scaffold recedes as she repeats a card.
// annotate only — compare/SIFT stays L1 (its lateral read is already her
// own work, and its generation is additionally constrained below).
level := agent.GuidanceL1
if spec.Primitive == "annotate" {
	if u, ok := UserFromContext(ctx); ok {
		uses, err := store.CountCompletedCardUsesByUser(ctx, u.ID, spec.ID)
		if err != nil {
			// Degrade to L1 rather than failing the surface: a card that
			// asks too much is a wall, a card that asks too little is
			// merely a slower fade.
			slog.Warn("surface anchors: guidance count failed", "err", err, "request_id", httpx.RequestIDFromContext(ctx))
		} else {
			level = agent.GuidanceFor(uses)
		}
	}
}
```

Pass `level` to `gen.Generate(...)`. Everything after — the lateral-dimension
drop, the metering block, the marshal, the persist — is unchanged.

Update the guidance-level comment at the `streamAction` call site
(`studioturn.go:250`) which currently says "Guidance level L1: the AI authors
these anchors" — it is now the fade's entry point, not a fixed level.

- [ ] **Step 4: Run full packages**
- [ ] **Step 5: Commit** (`feat(n3c): the guidance fade goes live at the surface seam`)

---

### Task 6: Two additive `TraceEvent` kinds

**Read first:** spec §6.

**Files:**
- Modify: `packages/contracts/src/envelope.ts`
- Modify: `apps/api/internal/api/cards.go` (`traceKinds`)
- Test: `packages/contracts/test/` (the envelope test file), and the Go test
  covering `validateEventTrace`

**Interfaces:**
- Produces, consumed by Tasks 8/9: `span_located` and `span_not_found` trace
  events.

- [ ] **Step 1: Write the failing tests**

Contracts: a test asserting both new kinds parse, and that an existing
`field_change`/`skip`/`submit` trace still parses unchanged (the additive
guarantee).

Go: a test asserting `validateEventTrace` **accepts** both new kinds and still
**rejects** an unknown kind. Both sides matter — this union is enforced twice,
and a one-sided change 400s every L2/L3 submit.

- [ ] **Step 2: Run and watch fail (both suites)**

- [ ] **Step 3: Implement**

Zod, appended to the union:

```ts
  z.object({ kind: z.literal("span_located"), dimension: z.string(), block_id: z.string(), at: z.string() }),
  z.object({ kind: z.literal("span_not_found"), dimension: z.string(), at: z.string() }),
```

Go: add `"span_located"` and `"span_not_found"` to `traceKinds`, with a comment
noting it mirrors the Zod union in `packages/contracts/src/envelope.ts` and the
two must move together.

- [ ] **Step 4: Run contracts + Go full packages**
- [ ] **Step 5: Commit** (`feat(n3c): span_located / span_not_found trace events`)

---

### Task 7: Text selection → span

**Read first:** spec §8, §7.1.

**Files:**
- Create: `apps/web/src/primitives/annotate/selection.ts`
- Create: `apps/web/src/primitives/annotate/selection.test.ts`
- Modify: `apps/web/src/primitives/annotate/Annotate.tsx`
- Modify: `apps/web/src/primitives/annotate/index.ts`
- Test: `apps/web/src/primitives/annotate/Annotate.test.tsx`

**Interfaces:**
- Produces, consumed by Task 9:
  ```ts
  export type CreatedSpan = { blockId: string; start: number; end: number; text: string };
  export function rangeToSpan(range: Range): CreatedSpan | null;
  export function selectionToSpan(): CreatedSpan | null;
  ```
  and on `Annotate`, two new OPTIONAL props:
  ```ts
  selectMode?: { dimension: string; onCancel: () => void } | null;
  onCreateSpan?: (span: CreatedSpan) => void;
  ```

- [ ] **Step 1: Write the failing tests**

`selection.test.ts` builds real DOM and real `Range`s (jsdom) and asserts, by
name: a selection inside one block yields the right `blockId` and **rune**
offsets and `text`; a selection whose text is Chinese yields offsets that round
trip (`Array.from(blockText).slice(start,end).join("") === text`); a collapsed
selection returns `null`; a selection spanning two blocks returns `null`; a
selection outside any `[data-block-id]` returns `null`; a selection that starts
inside a `<mark>` run and continues into a plain run still yields correct
offsets (offsets accumulate across sibling nodes, not within one).

`Annotate.test.tsx`: with `selectMode`/`onCreateSpan` absent, the rendered
output is unchanged from today (keep/extend the existing assertions). With
`selectMode` present, the hint bar naming the dimension renders and a cancel
control is present.

- [ ] **Step 2: Run and watch fail**

- [ ] **Step 3: Implement**

`selection.ts`:
- `runeLen(s)` = `Array.from(s).length`.
- `rangeToSpan(range)`: return `null` if `range.collapsed`. Find the nearest
  ancestor `[data-block-id]` of `range.startContainer` and of
  `range.endContainer`; return `null` if either is missing or they differ.
  Walk that block's descendant **text nodes in document order** (a
  `TreeWalker` with `NodeFilter.SHOW_TEXT`), accumulating `runeLen(node.data)`.
  When the walker reaches `range.startContainer`, `start = acc +
  runeLen(node.data.slice(0, range.startOffset))` — note `range.startOffset` is
  a UTF-16 offset within the text node, so it must be converted, not added.
  Same for the end. Return `null` if `end <= start`. `text` is the code-point
  slice of the block's full text, so it is always consistent with the offsets
  rather than being `range.toString()` (which can differ when the selection
  crosses element boundaries).
- `selectionToSpan()` reads `window.getSelection()`, returns `null` when there
  is no range, else delegates to `rangeToSpan`. Keeping `rangeToSpan` pure over
  a `Range` is what makes this testable.

`Annotate.tsx`:
- add `data-block-id={block.id}` to each `<p>`;
- when `selectMode` is set, render a hint bar above the article — instructional
  copy naming the dimension (e.g. 「在文章里选出你要用来回答「{dimension}」的那句
  话」) plus a 取消 button calling `selectMode.onCancel`. **No level name, no
  badge, no praise** (铁律 2);
- when `selectMode` is set, attach `onMouseUp` on the article container that
  calls `selectionToSpan()` and, on a non-null result, calls `onCreateSpan`.
  When `selectMode` is null, attach nothing.

- [ ] **Step 4: Run web suite + typecheck**
- [ ] **Step 5: Commit** (`feat(n3c): text-selection → rune-offset span creation`)

---

### Task 8: `StudioAnnotateCard` gains L2/L3 modes

**Read first:** spec §2, §5, §6.

**Files:**
- Modify: `apps/web/src/studio/StudioAnnotateCard.tsx`
- Test: `apps/web/src/studio/StudioAnnotateCard.test.tsx`

**Interfaces:**
- Consumes: `CreatedSpan` (Task 7), the new trace kinds (Task 6).
- Produces, consumed by Task 9 — three new OPTIONAL props:
  ```ts
  locatedSpans?: Record<string, { block_id: string; start: number; end: number; quote: string }>;
  onRequestLocate?: (anchorId: string, dimension: string) => void;
  onSpanNotFound?: (anchorId: string, dimension: string) => void;
  ```
  and an exported pure helper:
  ```ts
  export type AnchorMode = "answer" | "locate" | "elicit";
  export function anchorMode(a: Anchor): AnchorMode;
  ```

- [ ] **Step 1: Write the failing tests**

Named for the behavior they pin:
1. `anchorMode` table: `author:"ai"` → `"answer"`; `author:"student"` with a
   question → `"locate"`; `author:"student"` with a blank/whitespace question →
   `"elicit"`.
2. L1 anchors render exactly as today — **no** locate button, **no** question
   input. (Regression fence; extend the existing L1 tests rather than replacing
   them.)
3. L2 anchors render the AI's question read-only, plus a locate button and a
   「找不到合适的句子」 escape.
4. L3 anchors render a question **input** (empty) plus the locate button and
   escape, and no AI question text.
5. Lock gating at L2: with every answer + risk note filled but a dimension
   neither located nor marked not-found, the lock button is disabled; after the
   escape is taken, it enables. **The escape must always be able to unblock the
   lock** — an offer is never a wall.
6. Lock gating at L3 additionally requires a non-empty question per anchor
   (there is no "找不到" for writing your own question — eliciting cannot fail
   the way searching can), and no other new requirement.
7. Submitted envelope at L2/L3 carries the located span merged onto the anchor
   (`block_id`/`start`/`end`/`quote`), the student's typed question at L3, and
   the `span_not_found` trace events for the dimensions she skipped.

- [ ] **Step 2: Run and watch fail**

- [ ] **Step 3: Implement**

- Add `anchorMode` as an exported pure function with a comment pointing at spec
  §2 (the level is carried by the data).
- Local state gains `questions: Record<string, string>` for L3.
- `canLock`: keep today's rules (at least one anchor, every anchor answered,
  risk note non-empty) and add, for anchors whose mode is not `"answer"`:
  a located span **or** a taken escape; and for `"elicit"` anchors, a non-empty
  question. Track taken escapes in local state.
- `handleLock` merges `answers`, `questions`, and `locatedSpans` onto each
  anchor before appending the `risk_note` anchor exactly as today.
- The locate button calls `onRequestLocate(a.id, a.dimension)`; the escape calls
  `onSpanNotFound(a.id, a.dimension)` and records the local escape.
- When `onRequestLocate` is absent (e.g. an older host), render neither control
  and do not gate on locating — a control that cannot work must not be shown,
  and a gate with no way to satisfy it is a wall. Follow the existing
  `!hasAnchors` precedent: if a gate is dead, say why.
- Copy is instructional only (铁律 2). No level names in any user-visible string.

- [ ] **Step 4: Run web suite + typecheck**
- [ ] **Step 5: Commit** (`feat(n3c): annotate card asks her to locate (L2) and to elicit (L3)`)

---

### Task 9: Cross-pane wiring

**Read first:** spec §8, §7.2.

**Files:**
- Modify: `apps/web/src/studio/StudioContainer.tsx`
- Modify: `apps/web/src/studio/ViewFrame.tsx`
- Modify: `apps/web/src/studio/material/SourceDossier.tsx`
- Modify: `apps/web/src/studio/CoachRail.tsx` and `StudioShell.tsx` as needed to
  thread the three new `StudioAnnotateCard` props
- Test: `apps/web/src/studio/StudioContainer.test.tsx`,
  `apps/web/src/studio/material/SourceDossier.test.tsx`

**Interfaces:**
- Consumes: Task 7's `Annotate` props, Task 8's card props.

- [ ] **Step 1: Write the failing tests**

1. `SourceDossier` with the new optional `openSourceId` prop opens that source
   directly; with the prop absent, its own list→article navigation is unchanged.
2. `StudioContainer`: clicking a card's locate control switches the station to
   素材, opens the card's material, and puts the article in select mode naming
   that dimension.
3. Selecting text writes the span onto that anchor and clears select mode; the
   card then shows the located sentence.
4. The located spans and the pending trace reset when the active card instance
   changes (a new card must not inherit the previous card's located spans) —
   mirror the existing `activeCardInstanceId` effect precedent.

- [ ] **Step 2: Run and watch fail**

- [ ] **Step 3: Implement**

- `SourceDossier` gains optional `openSourceId?: string | null`; when non-null
  it wins over local `openId`. Keep the open/close timing instrumentation
  (`reportOpenElapsed`) correct — a forced open is still an open, and must log
  its elapsed time on close like any other.
- `StudioContainer` gains:
  ```ts
  const [locating, setLocating] = useState<{ anchorId: string; dimension: string; materialId: string } | null>(null);
  const [locatedSpans, setLocatedSpans] = useState<Record<string, {block_id:string;start:number;end:number;quote:string}>>({});
  const [spanTrace, setSpanTrace] = useState<TraceEvent[]>([]);
  ```
  Both reset in the existing `activeCardInstanceId` effect. `onRequestLocate`
  sets `locating`, switches the station to the material view, and forces the
  source open. `onCreateSpan` writes `locatedSpans[anchorId]`, appends a
  `span_located` trace, and clears `locating`. `onSpanNotFound` appends a
  `span_not_found` trace. The card's submit envelope must carry `spanTrace` —
  thread it into the card's event trace at the same point the card builds its
  envelope (extend `StudioAnnotateCard`'s props from Task 8 with the pending
  trace rather than mutating the envelope in the container, so the card stays
  the single builder of its own envelope).
- `ViewFrame` gains a `locating` prop group mirroring the existing `material` /
  `writing` / `review` grouping convention, threaded to `SourceDossier` →
  `Annotate`.

- [ ] **Step 4: Run web suite + typecheck**
- [ ] **Step 5: Commit** (`feat(n3c): cross-pane locate — card asks, article answers`)

---

### Task 10: End-to-end fade + full gate

**Files:**
- Modify: the real-DB Studio E2E test file that already drives surface→fill→
  submit→mint (find it; do not create a parallel one)

- [ ] **Step 1: Write the test**

`TestE2E_GuidanceFadeAcrossThreeCompletions` (or extend the existing vertical):
the same seeded student completes the CRAAP card, and on the NEXT surface the
anchors are L2-shaped; after a second completion, L3-shaped. Drive it through
the real HTTP handlers and a real Postgres — not by calling `GuidanceFor`
directly, which would prove nothing about the wiring.

Assert also that an L2 submit **completes** (mints) with a span-less anchor:
locating is never required server-side (spec §5). This is the regression fence
for the "never a wall" rule.

- [ ] **Step 2: Run the FULL gate**

```
cd apps/api && DOCKER_HOST=unix:///var/run/docker.sock CGO_ENABLED=0 go test -p 1 -count=1 ./...
cd apps/web && npm test && npx tsc --noEmit
cd packages/contracts && npm test
```
All three must be green (contracts' one known pre-existing TS2532 aside).

- [ ] **Step 3: Verify the diff touched nothing unrelated**

```
git diff --stat $(git merge-base main HEAD)..HEAD
```
Expect only `apps/api/`, `apps/web/`, `packages/contracts/`, and `docs/`.
**No migration file. No hand-edited `internal/store/sqlc/` or
`internal/cards/specs/`.**

- [ ] **Step 4: Commit** (`test(n3c): end-to-end guidance fade across three completions`)

---

## Note on test bodies

Tasks 3–10 give test **names, intent, and the exact assertions that must hold**
rather than literal bodies. This is deliberate: these tests run against real
testcontainers Postgres fixtures and existing React test harnesses whose current
helper shapes the plan author cannot reproduce verbatim without drift, and a
plan that pastes a stale helper signature produces a test that compiles against
an imagined API. Implementers must read the neighboring tests in the same file
and follow their established setup, while satisfying every assertion named here.
