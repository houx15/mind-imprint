# Slice 3 — Card Runtime Proven: CRAAP over `annotate`

| | |
|---|---|
| **Status** | Draft — for review |
| **Slice** | 3 of the whole-product refactor (`docs/2026-07-11-whole-product-refactor-roadmap.md`) |
| **Sources** | `2026-07-11-agent-spec.md` §3 (C2 card format) + §5.7 (acceptance test) + §4.2 (`surface_card`) — primary · product spec §8.3–§8.4 (cards run in stages), §7.1 rule 6 (disposition) |
| **Depends on** | Slice 0 (C2 card format, `card_instance`/`disposition` tables, graph) · Slice 1 (`AnnotateState`) · Slice 2 (the loop, classifier, coach) |
| **Delivers** | The C2 card runtime — `surface_card`, completion over the primitive state, `graph_effects`, consolidation, `observe`→coach, three-key disposition — proven by **CRAAP as pure config over `annotate`**, tested over fixtures. No UI/transport. |

## 0. Scope and boundary

Slice 3 makes the card the working atom: a card is **declarative C2 config** binding the
`annotate` primitive to the CRAAP framework, and the runtime instantiates it, questions
through it, checks its completion over the student's primitive state, writes its
`graph_effects` into the graph, reveals its framework at consolidation, and records the
student's disposition. The **acceptance test is agent-spec §5.7**: CRAAP ships as config, the
runtime touches **zero interface code and zero new primitives**, and a hypothetical second
card would need zero new runtime code.

**In scope (backend runtime + config, over fixtures):** the C2 card representation in Go ·
`surface_card` verb (instantiate a `card_instance` onto a target) wired into the Slice-2
loop · completion predicates over `card_instance.anchors` · `graph_effects` (mint an evidence
node) · consolidation (framework → `framework_fill`) · `observe` rules → coach candidates ·
three-key disposition persistence · CRAAP authored as C2 config.
**Out of scope (later slices):** the UI that renders the card in the rail + the transport
that carries the student's annotate answers to the server (Slice 5) · SIFT/lateral (Slice 6) ·
Toulmin over `graph` (Slice 7) · the planner (Slice 4). Slice 3 drives the runtime over
**fixture `card_instance` state**, not a live UI.

## 1. Decisions (flagged for review)

- **Extend the Go card spec with the C2 fields — one card type, no orphans.** Mirror Slice 0's
  TS `CardSpec` evolution in the Go `cards` loader: add optional `primitive`, `target_type`,
  `params`, `completion`, `graph_effects`, `observe`, `consolidation`, `intrusiveness_cap` (the
  structured ones as small typed Go structs, not `map[string]any`, so the runtime reads them
  safely). The legacy form `steps` stay for the old renderer. **No second card type.**
- **CRAAP gains C2 fields in place** (`packages/contracts/cards/craap.json` + the synced Go
  copy): it keeps its `steps` (legacy) and adds the `primitive: annotate` binding + params +
  completion + graph_effects + observe + consolidation. One file, dual-readable.
- **Card state lives in `card_instance.anchors`** (the `annotate` spans = the student's
  per-dimension answers) + `framework_fill` (consolidation). No new table.
- **The old form-card runtime** (web `CardRenderer`/`cardSheetHost`, the form path) is
  **legacy**, retired in Slice 5 with the old workspace — tracked in the roadmap retirement
  list; not touched here.

## 2. The C2 card, in Go

Extend the Go card spec so a card declares:
```
primitive         "annotate"
target_type       "material.source"
params.tags       [currency relevance authority accuracy purpose]
params.tag_prompts { authority: "谁站在这条主张背后——原始出处是谁？", ... }
completion        [ {kind:"every_tag_present", tags:[authority accuracy purpose]},
                    {kind:"field_written_by", field:"risk_note", author:"student"} ]
graph_effects     [ {kind:"promote", from:"material", to:"evidence", with:"source_quality"} ]
observe           [ {when:"tag=authority AND note_len<15", verb:"post_intervention", level:"I2"} ]
consolidation     "reveal_framework_after_completion"
intrusiveness_cap "I3"
```
Completion predicates, graph-effects, and observe rules are a **small closed set** of typed
kinds — adding a new *card* uses these kinds as config; a genuinely new kind is a runtime
change (agent-spec §5.7's escape hatch).

## 3. The card lifecycle

`proposed → active → completed | skipped` (the Slice-0 `card_instance.status` enum).

- **`surface_card(card_id, target, entry_stage)`** — the coach verb (wired here, deferred from
  Slice 2): instantiate a `card_instance` (status `proposed`) for the card on a target
  (a `material`), with `contract_ref`/`stage` from the card. Entry stage is **derived from the
  target/gate state, never chosen** (agent-spec §4.2). A new classifier predicate proposes it:
  a source `material` with no evaluation `card_instance` and no evidence node → candidate
  `surface_card(craap)`.
- **active** — the student answers the anchored CRAAP questions; their answers land as
  `annotate` spans on `card_instance.anchors` (fixture-provided in Slice 3). `observe` rules
  read that state and emit coach candidates (e.g. a weak authority note → `post_intervention`).
- **completion** — `EvaluateCompletion(card, instance)` runs the predicates over the anchors;
  incomplete → still `active`/`flagged-weak` (machine never marks `solid`, DEC-3). On complete:
  apply `graph_effects` (mint an `evidence` `graph_node` from the material, `author=student`,
  carrying `source_quality`) and set `framework_fill` from the consolidation payload; the
  status becomes `completed` only via a passed challenge or explicit confirmation.

## 4. `graph_effects`, consolidation, disposition

- **`graph_effects`** are pure functions of (card, completed instance, graph) → new
  nodes/edges. CRAAP's `promote` mints one `evidence` node linked to the source material by a
  typed edge (`supports`? no — an `evaluated`/`derived-from` edge), carrying the CRAAP verdict
  as `source_quality`. This is what a later card (Toulmin) will consume — cards compose over
  one graph.
- **Consolidation** (R-9): the card's framework (the CRAAP methodology text) is written to
  `framework_fill` **only after** completion — revealed after use, never before.
- **Three-key disposition** (product §7.1 rule 6): a `RecordDisposition(intervention_id,
  action, reason)` persists accept/reject/rewrite + reason (≥15 chars enforced) to the
  `disposition` table; feeds D5 / thinking leaps later.

## 5. `observe` → the coach

The Slice-2 classifier gains card-aware candidates: for each `active` card_instance, its
`observe` rules are evaluated over the primitive state; a matching rule yields a `Candidate`
(verb + level + anchor) the coach turns into an anchored `post_intervention` through the same
enforcement stack. No new coach code — `observe` is another candidate source.

## 6. Testing (fixtures, no UI)

- **Card loads as C2 config** — `craap.json` parses with primitive/params/completion/etc.; the
  Go loader exposes them typed.
- **`surface_card`** — a source material with no evaluation → the classifier proposes it → the
  loop instantiates a `proposed` `card_instance` on that material; a source already evaluated →
  no proposal (silence).
- **completion** — a fixture `card_instance` with anchors covering the required tags + a
  student-written risk note → `EvaluateCompletion` passes; missing a required tag or the note →
  fails; the machine never sets `solid`.
- **`graph_effects`** — on completion, exactly one `evidence` node is minted from the material
  with `source_quality`, linked by an edge; running effects twice is idempotent.
- **consolidation** — `framework_fill` is empty while active and populated on completion.
- **disposition** — accept/reject/rewrite persists with a ≥15-char reason; a short reason is
  rejected.
- **§5.7 acceptance** — a second trivial card authored as config (e.g. a `note` card over
  `annotate`) surfaces + completes with **zero new runtime code** (a test that authors it
  inline and runs it).

## 7. Open questions for the plan

1. The edge type minted by `graph_effects` (material→evidence): name it `evaluated-as` (source
   → evidence node) so Slice 7's Toulmin can consume evidence nodes uniformly.
2. How `entry_stage` is derived in Slice 3 without gates (Slice 4) — default to `draw_out`
   for `proposed`, `interrogate` for `active`; the gate-state derivation lands with the gate
   engine (Slice 4).
3. Reason-length enforcement location — enforce ≥15 in `RecordDisposition` (server), mirrored
   in the UI later.

## 8. Acceptance criteria

- CRAAP runs end-to-end as **config over `annotate`**: surfaced by the coach, completed over
  fixture primitive state, minting an evidence node, revealing its framework, with a recorded
  disposition — all proven by tests.
- The completion/graph-effect/observe kinds are a typed closed set; **a second card needs zero
  new runtime code** (proven by a test).
- `surface_card` is wired into the Slice-2 loop; `observe` feeds the coach through the existing
  enforcement stack.
- The machine never marks a card `solid` (DEC-3); consolidation reveals the framework only
  after completion (R-9); dispositions require a ≥15-char reason.
- No UI/transport, no planner, no new primitive, no changes to the legacy form-card path.
