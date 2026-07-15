# Slice 7 — 结构 view + `graph` primitive · Design

> Whole-product refactor #2. Third hand-built interaction primitive (`graph`) +
> the **Toulmin card** as pure C2 config over it + live wiring of the
> already-scaffolded STRUCTURE view (S4 论证构建). Reuses the existing spine:
> `SurfaceCardCandidates` auto-surface, atomic `CommitCardMint`, the gate engine,
> the standard envelope. No new transport.

## 0. Context & what already exists

The roadmap line for Slice 7 reads larger than the actual work, because the S4
machinery already landed in earlier slices:

- **The S4 contract exists** (`writing-project.json`): `produces: [claim,
  concession]`, gate = `no_unsupported_claim` + `no_single_sourced_claim` +
  `node_present{concession}`, `student_written: [warrants, steelman]`.
- **All three gate predicates are implemented** in `gate.go`
  (`no_unsupported_claim`, `no_single_sourced_claim`, `no_orphan_evidence`,
  `node_present`).
- **The STRUCTURE view shell already renders** the five-role card list
  (`StructureView.tsx`), but its source-picker and textarea are dead stubs
  marked "Slice 7 接入", and `StudioContainer` feeds it `views.structure: []`.
- **`GraphState` (nodes + edges, field-level `author`) already exists** in
  `packages/contracts/src/interactionPrimitive.ts` — a Slice-1 forward
  declaration.

What is genuinely missing — the real Slice 7 — is: (1) the `graph` primitive as
a live interaction, (2) the Toulmin card that feeds it, (3) the end-to-end
summon→render→mint wiring, (4) one gate-contract reconciliation.

### Binding design

The binding design (`docs/design/思维印记_工作区.dc.html`, the STRUCTURE view,
`<!-- STRUCTURE view : Toulmin map + orphan / naked flags -->`, ~L959–1016 and
the `S4META` state ~L1947) renders S4 as **one integrated five-slot flow**, in
the **center pane** (not the coach rail):

| slot id | role (verbatim) | needSrc | gate role |
|---|---|---|---|
| `claim` | 核心主张 | no | mints `claim` node |
| `warrant` | 理据 · 推理 | yes | mints `warrant` node + `cites` edges |
| `evidence` | 支撑证据 | yes | mints `evidence` node + `supports`→claim + `cites` edges |
| `counter` | 反方 · 钢人 | no | mints `counter` node (student steelman) |
| `concession` | 让步 · 转折 | yes | mints `concession` node + `cites` edges |

Each slot: open it → (if needSrc) multi-select from the CRAAP-locked source
chips → write the step as a sentence (≥12 chars, verbatim `s4complete`). The
per-slot flow copy is verbatim from the design (① 选择相关素材（信源评估里已锁定的）/
② 基于素材，把这一步写成句子 / placeholder 用你自己的话写……).

## 1. Decisions locked in brainstorming

1. **Primitive shape = typed-slot argument builder**, not a spatial canvas. The
   renderer is the fixed five-slot list the design shows. `graph` names the
   *state it produces* (typed nodes + edges), not a spatial visualization. This
   resolves the Slice-1 "card-driven-vs-map-viz" deferral in favor of the
   design.
2. **The Toulmin card owns all five slots** (including 反方·钢人 and 让步·转折).
   The standalone `concession.json` / `steelman.json` cards stay **legacy and
   untouched** (old `interaction_type` format, chat-surface only); they are NOT
   migrated in this slice, and are NOT removed from the S4 `repertoire` list.
3. **S4 gate = card completion only.** Following the design banner literally
   (五张卡片都写成了句子、该接素材的都接上了——本环节门禁通过). Rationale:
   source *quality* is already owned by S3 (CRAAP / SIFT / cross_check); S4
   owns argument *structure*. Concretely: **drop `no_single_sourced_claim`**
   from the S4 contract; keep `no_unsupported_claim` (satisfied by the
   evidence→`supports`→claim mint) and `node_present{concession}` (satisfied by
   the concession slot). The `gate.go:78` "distinct source materials"
   refinement carry-forward becomes **moot** and is dropped — no contract uses
   `no_single_sourced_claim` after this slice (the predicate stays implemented
   as generic gate machinery, just unreferenced).
4. **Summon = the existing auto-surface mechanism**, extended, plus an
   explicit end-to-end test of the summon→render→mint hop — the concrete guard
   against the 6c defect (SIFT was unsummonable because no task's diff spanned
   the seam).

### Out of scope (deliberate, follows the binding HTML over the roadmap prose)

- **map⇄outline toggle** — the roadmap line mentions it; the design's STRUCTURE
  view has no such toggle. Dropped (YAGNI).
- Migrating `concession` / `steelman` to C2.
- A spatial node/edge canvas.
- Any change to S0–S3, S5, S6.

## 2. The `graph` primitive

**File:** `apps/web/src/primitives/graph/Graph.tsx` (+ test).

A controlled component in the annotate/compare mold: props in, `onChange` out,
no internal persistence, no model calls. It renders the design's five-slot list
and nothing else — it does not know the word "Toulmin"; the five roles, their
questions, and their `needSrc` flags arrive as **params from the card spec**
(schema-driven: a new card = new JSON, no renderer edit).

### 2.1 State

The primitive's working state is a **`GraphState`** (already in contract):

```ts
GraphState = { nodes: GraphNodeUnit[]; edges: GraphEdgeUnit[] }
GraphNodeUnit = { id, type, text, author }        // type = slot role id
GraphEdgeUnit = { id, from, to, type }            // type ∈ {supports, cites}
```

- One node per slot the student has touched: `id` = slot role id
  (`claim`/`warrant`/…), `type` = same role id, `text` = the sentence,
  `author: "student"` (always — every slot is the student's own words; there is
  no AI-authored slot text, and enforcement rejects `author != student` at
  write).
- `cites` edges: `from` = a needSrc slot node id, `to` = a **material id**
  (a plain string; the primitive/mint distinguish material-target from
  node-target by edge `type`: `cites` → material, `supports` → node). One
  `cites` edge per selected source material.
- `supports` edge: `from` = `evidence`, `to` = `claim`. Authored implicitly by
  completing the evidence slot (the structural fact that evidence backs the
  claim), not free-drawn.

**Carrier — anchors, no shared-spine signature change.** A slot cites a *list*
of materials, and each `Anchor` holds a single `material_id`, so a slot is
serialized to **multiple anchors, all keyed by `dimension: <slotId>`**:

- one **text anchor** per slot: `{dimension: slotId, answer: <sentence>,
  author: "student", material_id: ""}`;
- one **source anchor** per selected material on a needSrc slot:
  `{dimension: slotId, answer: "", author: "student", material_id: <sourceId>}`.

Text vs source anchors never collide (a text anchor has `material_id: ""`; a
source anchor has `answer: ""`). This reuses the anchor spine wholesale:
`EvaluateCompletion(spec, anchors)` and `GraphEffects(spec, materialID,
anchors)` keep their exact signatures — Slice 7 only adds new `switch` cases
(`graph_slots_complete`, `toulmin`), the same additive move 6c used for
`lateral_source_present` / `cross_check`. `GraphState` remains the **frontend
primitive's working type**; the studio card serializes it to anchors on lock
(exactly as `StudioAnnotateCard.handleLock` builds `[]Anchor` from its state
today) and rebuilds it from anchors on rehydrate.

### 2.2 Events (C4)

Emit through the existing envelope reducer / event trace:
`step_expand` (slot opened), `field_change` (source toggled / text edited),
`submit` (lock). These already exist in `envelopeReducer.ts`; the primitive
wires slot interactions to them so the process tree and assessment see
student-initiated structure work.

### 2.3 What it composes / does not fork

`annotate` = spans over one material; `compare` = two `annotate` panes. `graph`
is a genuinely new state shape (typed nodes + edges, no span offsets), so it is
a new primitive, not a fork. It does **not** reuse `Annotate` internally.

## 3. The Toulmin card (C2 config)

**Files:** `packages/contracts/cards/toulmin.json` + the synced Go mirror
`apps/api/internal/cards/specs/toulmin.json` (via `make sync-cards`;
`TestMirrorMatchesCanonical` guards the mirror — every card-JSON change resyncs
it).

Pure config, no renderer code. Shape (following the `sift.json` template):

```jsonc
{
  "id": "toulmin",
  "category": "论证结构",
  "name": "论证构建卡（图尔敏）",
  "primitive": "graph",
  "target_type": "project",          // about the whole argument, not one material
  "params": {
    "slots": [
      { "id": "claim",      "role": "核心主张",   "needSrc": false, "q": "你要论证的核心判断，用一句话说清。" },
      { "id": "warrant",    "role": "理据 · 推理", "needSrc": true,  "q": "为什么这些证据能支撑主张？把中间的推理写出来。" },
      { "id": "evidence",   "role": "支撑证据",   "needSrc": true,  "q": "挑一条证据，用自己的话概括它如何支撑主张。" },
      { "id": "counter",    "role": "反方 · 钢人", "needSrc": false, "q": "坐到反方席——对方最硬的一张牌是什么？" },
      { "id": "concession", "role": "让步 · 转折", "needSrc": true,  "q": "承认反方后如何转折？给一条能核查的证据。" }
    ]
  },
  "completion": [
    { "kind": "graph_slots_complete" }   // all slots text ≥12 chars; every needSrc slot ≥1 cited material
  ],
  "graph_effects": [ { "kind": "toulmin" } ],
  "consolidation": "reveal_framework_after_completion",
  "intrusiveness_cap": "I3"
}
```

The `slots`, `role`, `q` copy is verbatim from `S4META`. The `steps`/
`methodology` blocks follow the design's existing card idiom (why/how/when/
example) and are authored, not lorem.

### 3.1 Completion — `graph_slots_complete`

New completion predicate (a `switch` case in `EvaluateCompletion`, anchor-based):
for every slot declared in `params.slots`, some anchor with `dimension == slotId`
has trimmed `answer` length ≥ 12; and every slot with `needSrc: true` has ≥ 1
anchor with `dimension == slotId` and non-empty `material_id`. Returns the
missing slot ids (for the banner / refill). This is exactly the design's
`s4complete` predicate, server-side.

### 3.2 Mint — the `toulmin` graph effect

**File:** `apps/api/internal/agent/card_effects.go` (a `switch` case in
`GraphEffects`), reusing the existing `CompleteCard` insert/placeholder-resolve
path (`card_lifecycle.go`) and the atomic commit already used by cross_check.

On the card reaching `completed`, `GraphEffects(spec, materialID, anchors)`
returns, from the slot anchors:

- `MintNode`s (`Type` = slot id, `Author: "student"`, `Body: {"text": sentence}`)
  for `claim`, `warrant`, `evidence`, `counter`, `concession` — one per slot that
  has a text anchor.
- a `MintEdge` `supports`: `FromKind/ToKind = graph_node`, `FromID =
  "$new:<evidence idx>"`, `ToID = "$new:<claim idx>"` (both endpoints are
  placeholders — `CompleteCard` already resolves both, per the `card_effects.go`
  header comment).
- `MintEdge` `cites` per source anchor: `FromKind = graph_node`, `FromID =
  "$new:<slot idx>"`, `ToKind = material`, `ToID = <material_id>` (the material
  already exists — a real id, not a placeholder).

The `framework_fill` idempotency guard is written **last** by the same commit
path cross_check uses; re-submission is a no-op.

After the mint, the S4 gate re-reconciles: `no_unsupported_claim` passes (claim
now has a `supports` from evidence) and `node_present{concession}` passes
(concession node exists) → S4 flips to solid, S5 unlocks. This is the assertion
the mint test verifies.

## 4. Data flow — no shared-spine signature change

Because the slot state is carried in **anchors** (§2.1), both shared entry
points keep their signatures: `EvaluateCompletion(spec, anchors)` and
`GraphEffects(spec, materialID, anchors)` gain only new `switch` cases. No call
site changes shape. This is the same additive pattern 6c used, and it is why R1
(below) is small: CRAAP/SIFT code is not touched at all.

The frontend graph primitive holds `GraphState` (nodes + edges) as its working
type; the studio card serializes it to text + source anchors on lock and rebuilds
it from anchors on rehydrate — the `StudioAnnotateCard.handleLock` /
`activeCardToState` pattern, applied to a graph shape.

## 5. Summon + render wiring (the 6c-lesson task)

### 5.1 Auto-surface

`SurfaceCardCandidates` (`classifier.go`) gains a Toulmin rule, keyed purely on
graph state (a no-model action, like CRAAP/SIFT): **S4 reachable** (S3 contract
solid) **AND no `claim` node exists yet** → emit a `surface_card` candidate
naming `toulmin`. Suppressed once a `claim` node exists (the card has done its
job) and while a `toulmin` card_instance is already proposed/active (the FIX-D
suppression already generalizes to any in-flight card).

**Project-targeted, no pre-generated anchors.** CRAAP/SIFT are
`material.source`-targeted and pre-generate anchors at summon (`card_lifecycle`
`checkedMaterialID` / `surfaceAnchors`), carried on the SSE `Card` event's
`materialID`. Toulmin is `target_type: "project"`: it targets no single material
and generates **no anchors** — the student authors empty slots. The summon path
must therefore tolerate an empty/absent `materialID` and an empty anchor set
(the SSE `Card` event already sends `"[]"` for no anchors; `materialID` becomes
`""`). No anchor-generation call runs for this card. This is called out because
the material-targeting assumption is baked into the 6c card_lifecycle helpers,
and a project-targeted card is the first to not fit it.

### 5.2 Render host

Unlike CRAAP/SIFT (coach rail + material-view highlights), the Toulmin card
renders **center-pane** in `StructureView`: the current dead source-picker and
textarea stubs go live, driven by the `graph` primitive bound to the active
`toulmin` card_instance. `StudioContainer` stops stubbing `views.structure: []`
and projects the live card. The coach rail carries only the nudge
(S3→S4 handoff line from the design, ~L1768).

### 5.3 End-to-end test — non-negotiable

One test drives the real controller across the whole hop: seed a project at S4
with locked CRAAP sources and no claim → assert `toulmin` is surfaced → submit a
complete graph envelope → assert the nodes/edges minted and the S4 gate flipped.
This is the test class that did not exist in 6c and let an unsummonable card pass
12 green tasks. It is a first-class task here, not folded into another.

## 6. Files

**Create:**
- `apps/web/src/primitives/graph/Graph.tsx` (+ `Graph.test.tsx`)
- `packages/contracts/cards/toulmin.json` + mirror `apps/api/internal/cards/specs/toulmin.json`

**Modify:**
- `apps/web/src/studio/views/StructureView.tsx` — stubs → live graph primitive
- `apps/web/src/studio/StudioContainer.tsx` — project live `views.structure`
- `apps/web/src/studio/state.ts` — `StructureCardFx` gains source-pick + text-commit handlers
- `apps/api/internal/agent/card_completion.go` — `graph_slots_complete` case (no signature change)
- `apps/api/internal/agent/card_effects.go` — `toulmin` case in `GraphEffects` (no signature change)
- `apps/api/internal/agent/classifier.go` — Toulmin surface rule
- `apps/api/internal/skills/specs/writing-project.json` — S4 gate: drop `no_single_sourced_claim`; add `toulmin` to `cards`/`repertoire`
- `apps/api/internal/cards/loader.go` — add `Slots []Slot` to `Params` (+ `Slot{ID, Role string; NeedSrc bool; Q string}`) so the Go side reads `params.slots`

## 7. Testing

- **Primitive unit** (vitest): slot renders question + chips + textarea; text
  commit + source toggle update state; `author: "student"` stamped; events
  emitted; ≥12-char + ≥1-source completion mirrors the server.
- **Card completion** (Go): `graph_slots_complete` true only when all slots
  written + needSrc sourced; missing slot ids reported.
- **Mint** (Go): the five nodes + `supports` + `cites` edges land atomically;
  re-submit is a no-op (framework_fill guard); S4 gate flips solid.
- **Summon** (Go): §5.3 end-to-end.
- **Contract**: `TestMirrorMatchesCanonical`; the studio card's
  `GraphState → anchors → GraphState` round-trip is loss-free (serialize on lock,
  rebuild on rehydrate).
- **Suite hygiene**: `CGO_ENABLED=0 go test -p 1 ./...` on a **quiet** Docker
  daemon (the 6c "hang" was contention from concurrent testcontainer runs, not a
  bug); web `vitest`; `tsc`.

## 8. Risks

- **R1 — new predicate/effect cases share `EvaluateCompletion` / `GraphEffects`
  with CRAAP/SIFT.** No signature change (§4), only additive `switch` cases, so
  the existing cases are untouched. Mitigation: the existing CRAAP/SIFT
  completion + mint tests must stay green unchanged, proving no regression.
- **R2 — center-pane card rendering is a new host.** All prior cards live in the
  rail. Mitigation: the §5.3 end-to-end test exercises the real render path, not
  a fixture.
- **R2b — first project-targeted card.** The 6c card_lifecycle helpers assume a
  material target + pre-generated anchors (§5.1). Toulmin has neither. Mitigation:
  a summon test asserts the `Card` event carries `materialID: ""` and `anchors:
  []` and the card still mounts and completes — no code path assumes a material.
- **R3 — dropping `no_single_sourced_claim` loosens rigor.** Accepted and
  deliberate (decision 3): S3 owns source quality; the spec records it so a
  future reader does not "restore" it as a bug.
- **R4 — `cites` edge `to` is a bare material-id string.** A typo'd id mints a
  dangling edge (graph edges are polymorphic, no FK — the 6c lesson). Mitigation:
  the mint sources material ids only from the student's actual CRAAP-locked
  chips (a closed set the UI supplies), never free text; a mint test asserts
  every `cites` target resolves to a real material row.

## 9. Carry-forwards

- Migrate `concession` / `steelman` to C2-over-graph and reconcile the S4
  `repertoire` (deferred here; they stay legacy).
- The "at most one in-flight card" invariant is still enforced by nothing
  (DTO-comment only) — unchanged by this slice.
- S5 writing view / S6 review deepening (Slices 8–9).
- `no_single_sourced_claim` predicate remains implemented but unreferenced; a
  future slice may reintroduce a distinct-materials structural check if desired.
