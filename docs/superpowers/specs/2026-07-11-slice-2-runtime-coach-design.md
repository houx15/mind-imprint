# Slice 2 — Runtime Loop + Classifier + Coach (the first live agent)

| | |
|---|---|
| **Status** | Draft — for review |
| **Slice** | 2 of the whole-product refactor (`docs/2026-07-11-whole-product-refactor-roadmap.md`) |
| **Sources** | `2026-07-11-agent-spec.md` §4 (runtime, verbs, triggers, I-ladder, subagents) + §6 (enforcement) — primary · product spec §7–§8 (coach posture, red lines) |
| **Depends on** | Slice 0 (graph/event tables, `enforcement`, C3 `AgentOutput`) · the `gateway` package |
| **Delivers** | The graph-triggered agent loop, the cheap **classifier**, and the flagship **coach** emitting one anchored `post_intervention` through the enforcement stack, persisted. No cards, no planner, no UI. |

## 0. Scope and boundary

Slice 2 builds the **first live agent runtime** in Go: the loop that reads workspace-graph
state, decides one next action, emits it as a typed output through the enforcement stack, and
records it. It runs the **classifier** (cheap tier, trigger predicates) and the **coach**
(flagship tier, one anchored `post_intervention`). It proves the loop end-to-end over a
fixture graph, with no UI and no cards.

**In scope:** the loop (`perceive→evaluate→decide-one→act→record`) · the classifier subagent ·
the coach subagent emitting `post_intervention` (anchored, criterion-tagged, one per run) ·
the enforcement stack wired under every coach output · model routing (cheap/flagship via the
gateway) · persistence to `intervention` (+ the `event` append) · sqlc queries for
`intervention` and graph reads.
**Out of scope (later slices):** `surface_card` (needs the card runtime → Slice 3) · the
planner + `plan`/`replan`/`advance`/intake (Slice 4) · any UI or SSE transport (Slice 5) · the
assessor (Slice 10). Verbs other than `post_intervention` are represented in the typed-output
union but **not executed** yet.

## 1. Decision: evolve the `agent` package; no orphans

The new runtime lands as **new files in `apps/api/internal/agent`**, not a parallel package.
It **reuses** — never duplicates — the `gateway` (Provider/keyResolver/SSE), Slice 0's
`enforcement`, the **克制阶梯 restraint-ladder prompt content** (consolidated out of
`prompt.go` into a shared coach-posture prompt), `anchors`, and `eval`.

The old `RunTurn` / `summon_card` path (`turn.go`, the `summon_card` tool) is **legacy, not
orphaned**: it still powers the current task-workspace app, so it stays live and compiling,
marked with a `// LEGACY: retired in Slice 5 (Studio replaces the old workspace)` note. It is
**deleted decisively in Slice 5**, tracked in the roadmap retirement list. Shared pedagogy
(the restraint ladder) is moved to one place both paths read, so there is no duplication in
the meantime.

## 2. The runtime loop

One exported entry point, called when workspace state changes (real triggers wire in later
slices; Slice 2 tests call it directly):

```go
// RunAgentStep performs one loop iteration for a project: perceive graph + event
// diff, classify, decide ONE next action (or silence), emit through enforcement,
// record. Returns the emitted action (nil = silence).
func RunAgentStep(ctx context.Context, deps AgentDeps, projectID uuid.UUID, trigger Trigger) (*Action, error)
```

- **perceive:** load the graph neighborhood for the project (nodes/edges via new sqlc reads)
  + the recent event diff + per-card competence.
- **evaluate:** the classifier (§3) runs trigger predicates → a set of flags / candidate moves.
- **decide one:** pick at most ONE next action at or below the intrusiveness cap; **silence is
  a first-class outcome** (return `nil`).
- **act:** the coach (§4) produces the action's body; it passes the enforcement stack (§5).
- **record:** persist the `intervention` (with `output_check_verdict`) and append an `event`.

`Trigger` mirrors the agent-spec tiers (`T-A` summon / `T-B` structural / `T-C` fine-grained);
Slice 2 handles T-A and T-B (T-C only accumulates context). Hard silences (while composing;
student-written-only fields) are honored.

## 3. The classifier subagent

Cheap tier. Input: the event diff + a shallow graph view. Output: internal flags (candidate
moves), never student-facing text. Slice 2 implements a small set of **trigger predicates**
(e.g. "a claim node has no supporting evidence edge" → candidate `post_intervention` on that
node) as pure, unit-testable Go over the graph, plus a cheap-model classification hook (behind
the gateway) for signals that need the model. The classifier holds no verbs that reach the
student.

## 4. The coach subagent

Flagship tier. Input: the local graph neighborhood + the active flags + recent context +
per-card competence + the **coach-posture prompt** (the reused 克制阶梯 restraint ladder,
one-question-at-a-time, anchored, board vocabulary). Output: exactly one `post_intervention` —
`{ anchor, criterion, body }` — anchored to a specific node/span and tagged with a CT
criterion. In Slice 2 the coach only emits `post_intervention` (and silence); `surface_card`,
`propose`, `check_gate`, `reply` are defined in the verb set but return "not-yet-implemented"
if selected, so the decision logic never routes to them.

## 5. Enforcement wiring

Every coach output passes the Slice 0 stack **before** it is persisted or returned:
`ValidateOutput` (typed shape) → the banned-phrasing suite → `OutputCheck` (declarative-echo
interception; a flagged output is re-asked as a question) → the authorship guard is available
for the field-write paths that later slices add. The `output_check_verdict` is stored on the
`intervention` row (the audit log, agent-spec §6.5). Enforcement cannot be configured off.

## 6. Model routing

Via `gateway.Provider` + `keyResolver` (server-side keys only): the **classifier** runs the
cheap tier; the **coach** runs the flagship tier. Assessment (Slice 10) will never downgrade —
noted, not built here. Token/cost are recorded per call as the existing gateway does.

## 7. Persistence + new sqlc

Slice 2 adds sqlc queries (Slice 0 only did project/graph/event): `intervention`
(insert + list-by-project), graph reads (`ListGraphNodesByProject` exists; add neighborhood /
by-node-and-edges reads as needed), and `card_competence` (get-by-user-card). The `event`
append reuses Slice 0's `AppendEvent`.

## 8. Testing

- **Integration** (testcontainers): seed a project + a fixture graph (a claim node with no
  evidence), call `RunAgentStep` with a **stub provider** scripted to return an anchored
  question; assert one `intervention` row persisted with the right anchor/criterion and an
  output-check verdict, and one `event` appended. A second case asserts **silence** (no
  candidate move → no intervention row).
- **Unit:** classifier trigger predicates over fixture graphs; the coach output shape; the
  enforcement wiring (a declarative-echo stub output is intercepted/rewritten before persist;
  a banned phrase is rejected). No secrets in any test; the live-provider path is opt-in.

## 9. Open questions for the plan

1. Graph-neighborhood query shape — how deep a neighborhood the coach reads (latency vs
   context). Start with the changed node + its direct edges + the project's claims; widen if
   needed (agent-spec §8 Q3).
2. Classifier model-hook vs pure predicates — Slice 2 leans on pure predicates for the one
   demonstrated trigger; the cheap-model hook is a seam, exercised more in later slices.
3. Where the coach-posture prompt physically lives once consolidated (shared module read by
   both the new coach and, until Slice 5, legacy `RunTurn`).

## 10. Acceptance criteria

- `RunAgentStep` runs the full loop over a fixture graph and, on a real candidate move, emits
  exactly one `post_intervention` — anchored, criterion-tagged — persisted to `intervention`
  with an output-check verdict, plus an appended `event`; on no candidate, returns silence.
- The coach output always passes the enforcement stack; a declarative-echo output is
  intercepted; a banned phrase is rejected — proven by tests.
- Classifier trigger predicates are pure and unit-tested; model routing uses cheap/flagship
  via the gateway.
- The old `RunTurn` path still compiles and passes its existing tests (legacy, untouched
  except the shared-prompt consolidation); no duplicated pedagogy; no orphaned/unused code.
- No `surface_card`, planner, UI, or SSE transport introduced.
