# N6-E · Template-driven personalized journeys — design

Date: 2026-07-23
Slice: N6-E (the last-deferred item of the N6 bucket — was "flagship planner
judgment"; reframed with the user into per-student journey composition).
Tracker row: `docs/2026-07-20-student-platform-remaining-work.md` (N6 row / N6
sweep section names N6-E as its own brainstorm).

## Problem

Every project today binds to the **same** static skill — `skills.ByID("writing-project")`
is hardcoded at ~10 `internal/api` call sites. All 7 stations (S0 任务解码 →
S6 反思归档), same contract DAG, for every student, regardless of where she
starts. A student who already has a research question and sources still walks
S0–S2; a student revising a draft still starts at S0. The planner is fully
deterministic (`Route`/`AdvanceAll` compute the path from gate-state + the DAG;
no LLM anywhere in journey composition).

We want the writing-project stations to become a **template** from which an LLM
**composes a per-student journey** — including *fewer* stations when her
starting status warrants — instead of forcing everyone through all 7. The same
mechanism should generalize to more templates later.

## Decisions (from brainstorming, all user-confirmed)

1. **Composition scope = selection only.** The LLM picks a SUBSET of the 7
   existing, validated contracts. Gates / producers / views / DAG edges are
   **unchanged** — every station is still the exact contract we ship and
   validate today. The journey is only *"which of the known stations."*
   **Reorder is CUT** (a writing pipeline's order is intrinsic; YAGNI). The LLM
   authoring NEW stations/gates was rejected (it would break the
   deterministic-gate guarantee — a generated gate item could have no producer,
   the exact N3d/N3f defect class, but now runtime-generated and unauthorable).

2. **AI composes and imposes at creation; stable after.** No confirm screen.
   The journey is composed once, at `POST /projects`, and does not re-shape
   under her as she works (a moving rail is a moving wall — 铁律 2). The one
   escape from "imposed" is re-open (decision 5), which is *her* action.

3. **Starting signal = the pasted material, inferred.** No new funnel
   questions. The LLM reads whatever she pasted (the assignment prompt, and
   anything she already has — notes, a draft-in-progress) and infers her
   starting point. Honest limitation: **from a bare title the LLM genuinely
   cannot tell** whether she has sources or a draft, so it will (and must)
   default to the full journey in that case. The feature earns its keep when
   she pastes *real* existing work; the paste box copy invites that.

4. **Dropped = waived + visible + re-openable.** A dropped station is recorded
   as *waived* for this project. It is honest (`waived ≠ done`), does not block
   finish, renders in the rail in a distinct `已跳过 · 可恢复` state, and the
   active route runs over the kept stations only. Auto-marking it `solid`
   (a green "done" for work she never did) was rejected on 铁律 4 grounds;
   removing it from existence (a hard wall + requires-edge rewrites) was
   rejected on 铁律 2 grounds.

5. **No mandatory spine.** The LLM may waive any station, including S4 论证构建
   and S6 反思归档. This is safe *only because* of decision 4: even a degenerate
   "S5-only" journey is one tap from re-adding S4/S6, and a bare-prompt paste
   defaults to the full journey (decision 3). A future template MAY declare a
   spine, but writing-project ships none and the composer hardcodes none.

## Core model — a journey is a waived-set over the fixed template

The template (`writing-project`'s 7 contracts) is **unchanged**. A project
gains **one** new piece of per-project state: the set of contract ids that are
**waived** for this student. "Journey" = *the template minus the waived
stations, in the template's own topological order.*

A `waived` contract:
- is **skipped by `Route`** (not in the active path);
- **counts as satisfied** for successors' reachability, `AdvanceAll` readiness,
  and finish — so a kept station behind a waived one is reachable and the walk
  can complete;
- renders in the rail as a distinct **`waived`** state (`已跳过 · 可恢复`) —
  never `done`;
- **never blocks finish**;
- is **re-openable** by the student.

### Verified correctness property (the design's load-bearing invariant)

**Every station's machine gate only checks nodes that station produces
itself** — no contract's gate references a node type produced by an upstream
contract. (Confirmed by reading `writing-project.json`: `decode_task` checks
`rubric_translation`/`weakness_prediction` which it produces; `frame_question`
checks `research_question`/`provisional_answer`/`preregistration` which it
produces; and so on through all 7.) Therefore **waiving an upstream station can
never make a downstream station's gate unsatisfiable.** This is why selection
is safe without any gate rewrite.

The *work* of a later station may implicitly want an earlier station's output
(e.g. S4 argument-building reads better with a research question from S1). That
is a UX/pedagogy concern, not a gate-machine correctness one, and re-open
(decision 5) is its remedy. Noted as a known limit, not fixed by gate plumbing.

## Composition — the first LLM call at creation

At `POST /projects`, **after** the existing project + onboarding-node mint,
a new mid-tier subagent call:

- **Purpose tag:** `compose_journey`. Metered like every LLM call (档位 + token
  + 成本) **including empty/rejected output**. **Not flagship** — this is cheap
  planning judgment, not assessment (assessment is the only never-downgrade
  flagship path).
- **Input:** the pasted material (prompt + anything she already has) + the
  skill's contract list (each contract's title + what it produces), rendered
  generically from the `Skill` struct so the prompt is **template-agnostic**.
- **Output (structured, validated):** for each contract, `{id, keep|waive,
  reason}`. **Rejected** unless every id is a real template contract id and the
  set of ids is exactly the skill's contracts.
- **Fail-safe:** malformed / empty / errored / rejected output → **default to
  the full journey** (no station waived = today's exact behavior). A failed
  compose can never strand her.
- **Persistence:** the waived-set is written into the project's existing plan
  graph-node jsonb (see below), and a `journey_composed` event is appended
  (waived ids + reasons) so the process record is honest and legible to the
  assessor (铁律 4).

## Storage — migration-free, via the existing plan graph-node

`agentstore.go` `UpsertPlan` already writes the project's single **plan
graph_node** with jsonb `{route, reason}`. The waived-set rides in that same
jsonb: `{route, reason, waived: [contract_id, ...]}`. Graph is loaded
(`LoadGraph`) on every planner and projection path already, so there is:
- **no new column**, **no new table**, **no migration**;
- one new reader helper (parse `waived` out of the plan node body), used by the
  planner, the projection, and the finish gate.

`writePlan` (shared by `Intake`/`Replan`) must **preserve** an existing
`waived` set when it rewrites `{route, reason}` — recomputing the route must not
silently un-waive stations. This is the one subtle spot: `writePlan` reads the
current plan's `waived` before overwriting, or the waived-set is threaded
through. (Plan-phase decides read-modify-write vs. explicit param; both are
migration-free.)

## Machine changes (all additive, no contract/DTO shape breaks)

- **`Route`** (`planner.go`): a contract whose id is in the waived-set is
  skipped (as if `Solid`), AND waived counts toward a successor's reachability
  test (currently `r.Solid || r.Status == "machine_clear"` → add `|| waived`).
- **`AdvanceAll`** (`planner.go`): waived counts as satisfied in the `solid[req]`
  readiness check, and a waived contract is itself skipped (never "advanced" —
  it was never worked). `AdvanceAll` must **not** confirm a waived contract
  (it stays `waived`, not `done`).
- **`studio` projection** (`projection.go`): read the waived-set from the plan
  node; emit a `waived` rail state distinct from `done`/`current`/`locked`;
  teach `canFinish` (currently `recordedGates["draft_polish"].Items[
  "whole_draft_review"] == "solid"`) to mean **"every non-waived contract is
  solid"** — so a waived S6 (or waived S5) does not wall finish, and finish
  keys on the terminal *kept* station. The `waived` state is an additive DTO
  enum value; existing consumers that switch on `done`/`current`/`locked` must
  get a `waived` arm.
- **Re-open:** `POST /projects/{id}/journey/reopen/{contractId}` removes one id
  from the waived-set (student action), then `Replan(..., "reopened")` so the
  station re-enters the route in the same request. Valid only for a currently-
  waived contract id belonging to the skill.

## Web (apps/web)

- The `StudioContainer` rail renders the new `waived` state: a muted
  `已跳过 · 可恢复` entry, tappable to re-open (calls the re-open endpoint, then
  refetches the projection — the same fetch-after-mutation pattern the studio
  already uses; guard against the known mount-effect race class).
- The create funnel's paste box copy invites pasting existing work (so there is
  signal to infer from). No new form fields.
- Icons inline SVG (never `lucide-react`).

## What's explicitly cut / deferred (YAGNI)

- **Reorder** — selection only (decision 1).
- **Multiple templates** — the composer + storage are template-agnostic by
  construction, but we ship & test **writing-project only**. A second template
  is future work, unblocked by this slice.
- **Rich starting-status form** — MVP infers from the paste (decision 3).
- **Mandatory spine config** — no template declares one yet (decision 5).
- **Re-composition mid-journey** — composed once at creation (decision 2).

## Known limits (carried forward, honest per the spec's own discipline)

- **Bare-prompt paste ⇒ full journey.** The feature is inert unless she pastes
  real existing work. Acceptable MVP: the mechanism is built; richer signal
  (the "what do you have" form) is a cheap future add.
- **A waived upstream station leaves its output un-produced.** A later kept
  station whose *work* wants that output (not its gate) is thinner; re-open is
  the remedy. Not a gate-machine defect.
- **Re-open re-enters the route but does not re-run composition.** The journey
  never re-shapes under her (decision 2); re-open is purely additive and hers.
- **`journey_composed` reasons are the LLM's, unverified.** They are recorded
  as a claim (what the AI said when it composed), not a measurement — the same
  discipline as N3f's AI-usage declaration.

## Testing strategy

- **Planner unit tests** (`internal/agent/planner_test.go`): waived skipped by
  `Route`; waived satisfies a successor's reachability; `AdvanceAll` completes a
  journey with a waived middle station and never confirms a waived one; a
  full-journey (empty waived-set) is byte-identical to today's route.
- **Projection tests** (`internal/studio/projection_test.go`): a waived station
  renders `waived` not `done`; `canFinish` true when every non-waived contract
  solid even with S6 waived; re-open flips it back to route. **Run the FULL
  `internal/studio` package** — the gate/rail enumeration tax (a rail-state
  change ripples to projection tests) has bitten every prior gate-touching
  slice.
- **Compose unit tests** (new): valid output → correct waived-set; malformed /
  empty / unknown-id output → full journey (fail-safe); every call metered
  including the rejected path.
- **Acceptance test** (`internal/api`): a project whose composed journey waives
  S0–S2 walks S3→S6 to completion and finishes, on client-reachable endpoints
  only (the N3d/N3f discipline); re-open S1 puts it back in the route.
- **Go:** `cd apps/api && DOCKER_HOST=unix:///var/run/docker.sock CGO_ENABLED=0
  go test -p 1 ./...` (FULL packages, never `-run` subsets for
  planner/gate/projection changes). **Web:** `cd apps/web && npm test` +
  `npx tsc --noEmit`. **Contracts:** `cd packages/contracts && npm test` +
  `npx tsc --noEmit` if the DTO enum touches the Zod side.

## Non-goals / invariants preserved

- The client never calls a model directly; the compose call is server-side in
  `apps/api`, key in server env only.
- No migration; no `packages/contracts` card-JSON change; the skill JSON is
  unchanged (selection needs no template edit).
- DEC-3 preserved: nothing here records a `student_written`/`human` gate item;
  `AdvanceAll` still never marks a non-machine item, and a waived station is
  explicitly **not** confirmed solid.
- Card JSON authored only in `packages/contracts/cards/`; skill JSON only in
  `packages/contracts/skills/` (unchanged here regardless).
