# Slice 4 — Skill format + gate engine + planner + intake

| | |
|---|---|
| **Status** | Draft — for review |
| **Slice** | 4 of the whole-product refactor (`docs/2026-07-11-whole-product-refactor-roadmap.md`) |
| **Sources** | `2026-07-11-agent-spec.md` §5.1 (C5 skill format) · §5.2 (Project policy: planner + intake) · §4.2 (`check_gate`/`plan`/`replan`/`advance`) · §4.5 (planner subagent) · §5.7 (skill-install acceptance) — primary · product spec §5.2 (S0–S6 stations + gate items), §5 S4 four-part gate (line 329), DEC-3 (machine never `solid`), DEC-8 (gates unlock) |
| **Depends on** | Slice 0 (Zod `Skill`, `graph_node` `plan`/`gate_state` types, `author='imported'`) · Slice 2 (the loop, `AgentStore`, `GraphView`) · Slice 3 (CRAAP mints the `evaluated-as` edge the gate engine reads) |
| **Delivers** | The C5 skill substrate — a Go skill loader over the writing-project contract DAG, a gate engine (closed machine-predicate set, DEC-3), a **deterministic** planner (route/replan/advance over the DAG), and intake (imported → graph, owe every gate) — proven by **the writing-project S0–S6 skill authored as pure config**, tested over fixtures + testcontainers. No UI, no transport, no model call. |

## 0. Scope and boundary

Slice 4 makes the **skill** the working unit one level above the card: a project type is
declarative C5 config — a contract DAG with gates — and the runtime loads it, evaluates its
gates over the workspace graph, computes a route through the unmet contracts, and reconciles a
mid-way arrival onto the graph. The **acceptance test is agent-spec §5.7**: the writing-project
S0–S6 journey ships as config (contracts + gates + card references + intake), the runtime
touches **zero new agent code and zero new primitives**, and a hypothetical second project type
(e.g. a bare "note-to-self" skill) would need zero new runtime code.

**In scope (backend runtime + config, over fixtures):**
- the C5 skill representation in Go (mirror the Slice-0 Zod `Skill`) + a `go:embed` loader with
  DAG validation (acyclic, `requires` resolve, `cards` resolve to the registry);
- the **writing-project skill JSON** — S0–S6 as contracts with `requires`/`produces`/`view`/
  `repertoire`/`gate{machine,student_written,human}`, single-source in `packages/contracts/
  skills/` synced to `apps/api`;
- the **gate engine** — `CheckGate` over a closed set of typed machine predicates against the
  graph, merged with recorded non-machine item status, returning a report; DEC-3 structural;
- **intake** — mint a structured list of imported candidates as `graph_node`s (`author=
  'imported'`) + `ReconcileGates` (run every gate → the empty/partial/machine-clear diff);
- the **deterministic planner** — `Route` (unmet + reachable contracts in DAG order) written as
  the **plan artifact** (`graph_node type=plan`); `Replan` (revise, reason recorded as an
  event); `Advance` (flip a `gate_state` node when its gate passes, unlocking dependents);
- **wiring** — `check_gate` as a coach verb candidate; the planner runs on intake / gate-passed.

**Out of scope (later slices, documented as seams):**
- the **flagship model-judgment planner** — prioritizing among simultaneously-unlocked
  contracts, `route_to_course` on repeated stalls, deciding what to defer. Slice 4's route is
  deterministic (see §1). The model layer lands when a real messy-state student + course
  detours exist (post-Studio);
- the **model-driven intake decomposition** — turning a pasted draft's raw prose into candidate
  claim/evidence nodes. Slice 4 mints from an *already-structured* candidate list; the prose→
  candidates model step and the live **material** transport land with the Studio (Slice 5/6/11);
- the challenge/human transport that records `student_written`/`human` gate items as satisfied —
  Slice 4 reads their status from fixture `gate_state` bodies;
- all UI (Slice 5), the Studio views (5–9), the assessor (10).

## 1. Decisions (flagged for review)

- **Deterministic planner now; model layer deferred.** *(Confirmed with the user.)* Over the
  mostly-linear S0–S6 DAG the route is determined by gate state + dependencies, so `Route` is a
  pure function (unmet reachable contracts in topological DAG order). The flagship model-judgment
  (§4.5 planner) is a documented seam — staged exactly as classifier(pure)→coach(model) was.
- **One branch.** *(Confirmed.)* Skill loader + gate engine + writing-project JSON + intake +
  planner ship together; the pieces are tightly coupled. Task boundaries are drawn in the plan.
- **~~No new migration~~ → one small additive migration (`0017`).** Slice 0 provisioned the
  `plan`/`gate_state` node types and `imported` author. But its `graph_node.type` **CHECK
  enumerated a fixed 5 values** — wrong, because `graph_node.type` is architecturally an *open,
  skill-defined vocabulary* (the gate engine's `node_present{type}` matches whatever artifact
  names a skill's contracts declare: `rubric_translation`, `perspective`, `preregistration`, …).
  Slice 4's integration test surfaced this (intake minting those types hit SQLSTATE 23514).
  Migration `0017` relaxes the CHECK to a non-empty guard; the structural types still work as
  exact string values, and a typo'd type just fails its gate (stays owed — the safe DEC-3
  direction). Additive, reversible, no sqlc/Go change.
- **Provenance = the `imported` author value, not a new column.** Intake marks candidates
  `author='imported'` (§5.2's "author=student, provenance=imported" collapses onto the existing
  enum). Imported content is **present but owes the gate**: it never auto-satisfies a
  `student_written` item — faithful to "owe every gate," and it needs no schema change.
- **Machine gate items are a closed set of typed graph predicates**, mirroring Slice 3's
  completion predicates: `node_present{type}` · `node_count_at_least{type,n}` ·
  `no_orphan_evidence` · `no_unsupported_claim` · `no_single_sourced_claim` ·
  `every_source_evaluated`. Semantically-hard items ("full proposition", "single question",
  warrant/steelman *quality*) are **not** machine — they are authored under `student_written`
  (presence + `author=student`, quality still owed) or `human`. A genuinely new predicate kind is
  a runtime change (§5.7's escape hatch); authoring a skill only references existing kinds.
- **DEC-3 is structural, not a rule the engine chooses to follow.** `CheckGate` can return
  `machine_clear` at best. `solid` is unreachable through any machine path — it exists only on a
  `gate_state` body written by a passed challenge or a human (later slices).

## 2. The skill, in Go

Mirror the Slice-0 Zod `Skill` as small typed Go structs in a new `skill` package, loaded the
same way cards are (`go:embed` of the synced JSON; single-source in `packages/contracts/skills/`,
`make sync-cards`-style copy into `apps/api/internal/skills/specs/`):

```
skill              "writing-project"
kind               "project"
contracts          map[string]Contract   // the DAG, keyed by contract id
  Contract.requires   [contract ids]      // DAG edges
  Contract.produces   [artifact tags]
  Contract.view       "结构" | "素材" | "写作" | "评估"
  Contract.repertoire [card ids]          // AI-initiated scoping (not enforced here)
  Contract.gate       Gate{ machine, student_written, human }
    Gate.machine        []MachineItem     // {kind, params} — the closed predicate set
    Gate.student_written[]string          // item names (presence+author=student owed)
    Gate.human          []string          // item names (human-only)
intake             IntakeProcedure        // §5.2 — declared, run by the planner
cards              [card ids]             // by reference to the registry
vocabulary         "per-board"
```

**Loader validation** (fail loudly at load, like the card loader): the contract graph is acyclic;
every `requires` names an existing contract; every `cards` / `repertoire` id resolves in the card
registry; every `MachineItem.kind` is one of the closed set. A skill that fails validation is a
load error, never a silent partial.

## 3. The writing-project skill (S0–S6 as config)

Author the seven contracts faithfully to product spec §5.2 (a linear `requires` chain — the
S2–S4 *loop* is "gates govern unlock, never revisit", not a DAG cycle):

| Contract | requires | view | machine items | student_written / human |
|---|---|---|---|---|
| `decode_task` (S0) | — | 评估 | `node_present{rubric_translation}`, `node_count_at_least{weakness_prediction,2}` | milestone_plan (student) |
| `frame_question` (S1) | decode_task | 结构 | `node_present{research_question}`, `node_present{provisional_answer}`, `node_present{preregistration}` | terms_defined (student) |
| `evaluate_perspectives` (S2) | frame_question | 素材 | `node_count_at_least{perspective,2}` | recon_logged, sources_per_perspective (student) |
| `evaluate_sources` (S3) | evaluate_perspectives | 素材 | `every_source_evaluated`, `no_single_sourced_claim` | source_risk_notes (student) |
| `build_argument` (S4) | evaluate_sources | 结构 | `no_orphan_evidence`, `no_unsupported_claim`, `no_single_sourced_claim`, `node_present{concession}` | warrants, steelman (student) / warrant_quality_spot_check (human) |
| `draft_polish` (S5) | build_argument | 写作 | `node_present{word_budget_ok}` | citations_matched (student) / whole_draft_review (human) |
| `reflect_archive` (S6) | draft_polish | 评估 | — | reflection (student) / declaration_signed (human) |

`repertoire` and `cards` reference the existing registry (`craap` on S3, etc.); Slice 4 does not
require new cards. The exact machine params are locked in the plan against the JSON; the table is
the authoring intent, not the literal file.

## 4. The gate engine

`CheckGate(skill, contractID, graph, recorded) → GateReport`:

- evaluate each `MachineItem` over the `GraphView` (the closed predicate set, §1) → per-item
  pass/fail + a human-readable "what's missing";
- merge the `student_written` / `human` item status from `recorded` (the `gate_state` node body —
  fixture-provided in Slice 4; a later slice writes it from challenges/teacher marks);
- overall status: `empty` (nothing present) · `partial` (some items pass) · `machine_clear` (all
  machine items pass, non-machine still owed) · `solid` (**only** if `recorded` already carries
  it — never computed here).

`GateReport` carries the overall status, the per-item results, and the ordered `missing` list
(what `check_gate` reports to the student). The `no_*` graph predicates read the same
polymorphic edges Slice 3 mints — `no_orphan_evidence` = every `evidence` node has an outgoing
`supports` edge to a `claim`; `no_unsupported_claim` = every `claim` has ≥1 incoming `supports`;
`no_single_sourced_claim` = every `claim` traces to ≥2 distinct source materials;
`every_source_evaluated` = every source `material` has an `evaluated-as` edge (Slice 3's CRAAP
effect).

## 5. Intake + reconciliation

`Intake(ctx, deps, projectID, skill, candidates) → (firstPlan, []GateReport)`:

- `candidates` is a **structured** list `{kind, text, span_ref?}` (the model decomposition that
  produces it is out of scope, §0); mint each as a `graph_node` with `author='imported'` via the
  store;
- `ReconcileGates(skill, graph, recorded)` runs `CheckGate` for **every** contract → the diff;
- build the first route from that diff (§6) and write the plan artifact.

A student arriving with reconstructed state finds some gates `partial`, some `empty`, none
skipped — "the gates are all still owed; the route just starts from where they actually are."

## 6. Planner: route / replan / advance (deterministic)

- **`Route(skill, gateReports) → []contractID`** — the **advisory** route: a contract is
  **reachable** iff all its `requires` gates are `machine_clear`-or-better (its predecessors are
  structurally sound, so work may begin even while their `student_written`/`human` items are
  chased in parallel — "loops are normal operation", §5.2); the route is the reachable-and-unmet
  contracts in topological DAG order. Pure function. This is distinct from the **blocking unlock**
  (DEC-8): the true pass that permits *finishing* a contract is `solid`, granted only by `Advance`
  when every item — machine computed **and** non-machine recorded — is satisfied. The route looks
  ahead past a structurally-clear gate; it can never mark one finished.
- **`Plan(ctx, deps, projectID, skill, gateReports, reason)`** — write/replace the project's
  single `plan` `graph_node` (body = `{route, reason, revision}`) and append a `plan_written`
  event. **`Replan`** is the same write with an incremented revision + a `plan_revised` event
  carrying the reason (§5.2: "every revision is an event with a reason").
- **`Advance(ctx, deps, projectID, skill, contractID, graph, recorded) → (advanced bool)`** —
  evaluate the contract's gate; if it **passes** (all items satisfied — machine computed +
  non-machine recorded), write/update its `gate_state` node to the passing status and append a
  `gate_attempt` event; dependents become reachable on the next `Route`. `Advance` **refuses**
  (returns false, writes a `gate_attempt` with what's missing) when any machine item fails or any
  `student_written`/`human` item is unrecorded. It never invents `solid` for a non-machine item
  (DEC-3).

## 7. Wiring into the loop

- **`check_gate` candidate.** The classifier gains a predicate: for the contract the plan's route
  currently points at, if its gate is not `machine_clear`, propose a `check_gate` candidate
  (anchor = the contract, level I1). `RunAgentStep` handles a `check_gate` candidate by running
  `CheckGate` and recording the report — no model call (it is a structural report, like
  `surface_card`). Ordering: `surface_card` > `post_intervention` > `check_gate` — `check_gate` is
  the **lowest-priority fallback**. (The draft ordering put `check_gate` above `post_intervention`;
  the whole-branch review showed that is wrong: `check_gate` is a no-op read that never clears
  itself, so ranking it above the coaching nudge would let it starve the very coaching that moves
  the student toward `machine_clear`. It fires only when there is nothing to coach.)
- **The planner runs on intake and on a passed gate** (agent-spec §4.5 cadence). Slice 4 exposes
  `Intake`/`Advance` as store-backed functions the loop calls at those moments; the standing
  "planner runs every T-B" cadence is a later refinement (the deterministic route only changes
  when a gate flips, so recomputing on gate-pass is sufficient and cheaper).

## 8. Storage (no new tables)

- **plan** — one `graph_node{type:'plan', author:'ai'}` per project; body `{route:[…], reason,
  revision}`. Updated in place; history lives in `plan_written`/`plan_revised` events.
- **gate_state** — one `graph_node{type:'gate_state', author:'ai'}` per (project, contract); body
  `{contract, status, machine:{item:pass|fail}, student_written:{item:status}, human:{…}}`.
  Carries the recorded non-machine status `CheckGate` merges. Updated in place; history lives in
  `gate_attempt` events.
- **imported nodes** — `graph_node{type:'claim'|'evidence'|…, author:'imported'}` from intake.

New `AgentStore` methods (sqlc-backed adapter + fake): `UpsertGraphNodeByKey` (one plan node per
project; one gate_state per project+contract — keyed upsert, not blind insert), `GetGraphNode`
(read a gate_state/plan body). `AppendEvent` already exists. `InsertGraphNode` already exists
(intake reuses it).

## 9. Testing (fixtures + testcontainers, no UI)

- **skill loads as C5 config** — `writing-project.json` parses; the loader exposes the typed DAG;
  a cyclic / dangling-`requires` / unknown-card / unknown-machine-kind skill is a load error.
- **gate engine** — each machine predicate over crafted `GraphView`s (orphan evidence fails
  `no_orphan_evidence`; a claim on one source fails `no_single_sourced_claim`; an unevaluated
  source fails `every_source_evaluated`; all-present → `machine_clear`); **DEC-3**: no input makes
  `CheckGate` return `solid`.
- **intake** — a structured candidate list mints `imported` nodes; `ReconcileGates` returns the
  empty/partial/machine-clear diff across all seven contracts; an imported claim does **not**
  satisfy a `student_written` item.
- **route** — reachability respects `requires` (build_argument not routed before evaluate_sources
  passes); the route is unmet reachable contracts in DAG order; replan re-routes and appends an
  event with the reason.
- **advance** — a passing gate flips `gate_state` + unlocks the dependent on the next `Route`; a
  gate with a failing machine item or an unrecorded student/human item refuses and records what's
  missing; advance never writes `solid` for a non-machine item.
- **wiring** — with a route pointing at an unmet contract and nothing to coach, `RunAgentStep`
  emits a `check_gate` action recording the report; the ordering (`surface_card` >
  `post_intervention` > `check_gate`, check_gate lowest) holds — a coaching nudge always outranks
  the gate-report fallback.
- **§5.7 acceptance** — a trivial second skill authored inline (one contract, one machine item,
  no cards) loads + reconciles + routes + advances through the exact same functions, with **zero
  new runtime code**.
- **testcontainers** — `UpsertGraphNodeByKey`/`GetGraphNode`/intake-mint round-trip against real
  Postgres (one plan node per project; one gate_state per project+contract).

## 10. Open questions for the plan

1. `gate_state` uniqueness — enforce "one per (project, contract)" by an app-level upsert keyed on
   `body->>'contract'` (no new unique index, staying migration-free), or accept a partial unique
   index? Default: app-level upsert (migration-free), matching the no-DDL decision.
2. Whether `Route` should carry within-contract sub-steps (the missing gate items as ordered
   work) in the plan body, or just the contract sequence. Default: contract sequence in Slice 4;
   the coach derives the next item from `CheckGate.missing`. Sub-steps are a UI concern (Slice 5).
3. `word_budget_ok` as a `node_present` machine item is a placeholder for S5's real word-count
   check (Slice 8). Kept as a presence predicate here so the DAG is complete; refined when the
   writing surface lands.

## 11. Acceptance criteria

- The writing-project S0–S6 journey runs end-to-end as **config**: the skill loads with a
  validated DAG, its gates evaluate over fixture graph state, intake reconciles a mid-way arrival
  ("owe every gate"), and the deterministic planner routes + advances through the DAG — all proven
  by tests.
- The machine-item kinds are a typed closed set; **a second skill needs zero new runtime code**
  (proven by a test).
- `CheckGate` never returns `solid`, and `Advance` never writes `solid` for a `student_written`/
  `human` item (DEC-3, proven by tests).
- The advisory route respects `requires` (structural reachability), and the blocking unlock is
  `solid` via `Advance` (DEC-8: a gate must fully pass before the dependent is *finishable*);
  going back is a re-`Route`, never a failure.
- `check_gate` is wired into the Slice-2 loop as a no-model structural action; the planner runs on
  intake and on a passed gate.
- No new migration, no UI, no transport, no model call, no new primitive, no changes to the legacy
  task/card paths.
