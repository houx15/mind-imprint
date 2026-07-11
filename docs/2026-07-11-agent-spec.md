# Agent Spec: One Runtime, Skills, Three Policies

| | |
|---|---|
| **Status** | Draft v3.2 — technical architecture; skill model, planner, subagent decomposition; the three module policies specified symmetrically (§5.2–5.4) with traces (§5.6) |
| **Author** | Product / Engineering |
| **Last updated** | July 2026 |
| **Scope** | The AI agent's runtime architecture; the **skill** packaging model for project types and courses; how one runtime operates across the three surfaces: **Course**, **Chat**, and **Project**. |
| **Relationship to `workbench-spec.md`** | That document is the product design: surfaces, pedagogy, red lines, the journey. This document is the technical design. Where the two disagree, treat it as a flag to reconcile, not an error in either. One reconciliation is proposed explicitly (§5.2: reverse entry). |

---

## 0. The core claim

Course, Chat, and Project are not three AI features. They are **one agent runtime**, loading **authored skills**, under **three policy configurations**.

- The **runtime** is an orchestrator: it reads structured workspace state, decides one next action, and acts through a small closed set of verbs. It never renders UI, never owns interaction flow, never writes prose into the student's work.
- A **skill** is the authored, versioned unit that defines a kind of work: **how to teach it** (milestone contracts, pedagogy), **how the teaching interacts** (view compositions over the primitive library), and **which tools apply** (the card set). A project type is a skill. A course is a skill. A card is the atomic skill.
- A **policy** sets how binding the loaded skill is and how the agent behaves around it — the only thing that differs between the three surfaces.

Why this decomposition:

1. **Interactions can't be forms, and can't be AI-generated.** Thinking tools need real manipulation — highlighting spans, building an argument graph, sorting and weighing. Hand-built interactions are the only way to get quality; AI-generated UI produces unparseable state. So the interface is a **finite library of interaction primitives**, and skills compose them; the AI parameterizes, never generates.
2. **The AI must read what the student does, not what the screen shows.** Every primitive has a fixed state schema; every student action becomes machine-readable data. The state schema is the API between interface and agent.
3. **Pedagogy must be authored; routing must be personal.** Students arrive mid-way, with drafts, with fragments, with wildly different needs. So skills declare **what must become true** (contracts), and the agent **plans a route** through them per student (§5.2). Structure is authored; the path is improvised.

> The interface externalizes thinking. The agent responds to the externalized thinking. Skills define the destination and the rules of the road; the agent drives.

---

## 1. System architecture

```
┌──────────────────────────────────────────────────────────────┐
│  SURFACES (Course / Chat / Project)                           │
│  shell, navigation, chat sidebar, views                       │
│    └── INTERACTION PRIMITIVES (finite, hand-built)            │
│         annotate · graph · sort · matrix · scale · compare    │
└──────────────┬────────────────────────────▲───────────────────┘
        events │ (state diffs)              │ actions (typed verbs)
┌──────────────▼────────────────────────────┴───────────────────┐
│  AGENT RUNTIME (the orchestrator / planner)                    │
│  perceive → evaluate → decide one next action → act → record  │
│  loads: SKILL (authored package) · bounded by: POLICY          │
│  passing through: ENFORCEMENT (typed output, output check)     │
└──────────────┬────────────────────────────▲───────────────────┘
               │ reads/writes               │ reads
┌──────────────▼────────────────────────────┴───────────────────┐
│  SHARED WORKSPACE GRAPH                                        │
│  materials · claims · evidence · links · card instances ·     │
│  snapshots · plan · gate states    + EVENT STREAM (append-only)│
└──────────────────────────────────────────────────────────────┘

SKILL REGISTRY (authored, versioned):
  atomic skills = cards · composite skills = project types, courses
```

The five contracts (all versioned, all static at runtime):

| # | Contract | Between | Contents |
|---|---|---|---|
| C1 | **Primitive state schemas** | interface ⇄ everything | one schema per interaction primitive |
| C2 | **Card format** (atomic skill) | authoring ⇄ runtime | how a thinking tool is declared (§3) |
| C3 | **Agent verb set** | agent → interface/graph | the closed set of tool calls (§4.2) |
| C4 | **Event set** | interface → agent | what state changes are pushed, when (§4.3) |
| C5 | **Skill format** (composite) | authoring ⇄ runtime | project types and courses: contracts, views, tools, intake (§5) |

Once these five are stable, the interface team, the skill authors, and the agent team ship independently.

---

## 2. Interaction primitives (C1)

A primitive is a hand-built, reusable interaction component with a **fixed state schema**. Target: **5–8 primitives total**, covering every thinking tool the platform will ever ship. Adding a primitive is a platform-level design conversation; adding a skill is authoring.

| Primitive | Manipulation | State schema (essentials) |
|---|---|---|
| `annotate` | highlight spans in a material, attach a semantic tag + note | `{material_id, spans: [{range, tag, note, author}]}` |
| `graph` | create typed nodes, link them with typed edges | `{nodes: [{id, type, text, author}], edges: [{from, to, type}]}` |
| `sort` | classify/rank a set of items into slots | `{items, slots, placements, rationales}` |
| `matrix` | fill a typed grid (rows × dimensions) | `{rows, cols, cells: [{value, note}]}` |
| `scale` | place a judgment on a labelled continuum, with reason | `{axis, position, reason}` |
| `compare` | two materials side by side with paired annotations | `{left, right, pairs: [{l_span, r_span, note}]}` |

Rules:

- **Every state field a student can change is in the schema.** If the agent can't read it, it didn't happen.
- **`author` on every unit.** Student-written vs AI-suggested vs imported is tracked at the field level — assessment and the never-writes enforcement both depend on it.
- Primitives know nothing about pedagogy. `annotate` doesn't know what CRAAP is. Skills supply the semantics.
- Skills may declare **view compositions** — layouts assembling primitives for a milestone (e.g. the argument view = `graph` + a docked `annotate` on the active source). Compositions are configuration; new primitives are not.

---

## 3. Cards (C2): the atomic skill

A card is a declarative config binding a primitive to a thinking framework. It contains **zero UI code and zero flow code**.

```yaml
card: craap
primitive: annotate
target_type: material.source          # what it attaches to
params:
  tags: [currency, relevance, authority, accuracy, purpose]
  tag_prompts:                        # per-tag guiding questions, templated
    authority: "Who stands behind this claim — trace it upstream."
completion:                           # machine-checkable, over primitive state
  - every_tag_present: [authority, accuracy, purpose]
  - field_written_by: {field: risk_note, author: student}
graph_effects:                        # what completing this card writes to the graph
  - promote: {material → evidence_node, with: source_quality}
observe:                              # agent observation rules (state → possible move)
  - when: "tag=authority AND note.length < 15"
    move: {verb: post_intervention, level: I2, hint: "authority under-argued"}
consolidation: reveal_framework_after_completion
intrusiveness_cap: I3
```

Key properties:

- **A card is an input to the agent, not a form for the student.** The agent uses `tag_prompts` and `observe` rules to question the student one anchored move at a time; the primitive is where the student's answers live.
- **`graph_effects` is what makes cards composable.** A CRAAP pass mints evidence nodes carrying source-quality attributes, which the argument-map card then consumes. Cards are not silos; they read and write one graph.
- The same card runs on all three surfaces. `card_id` is the routing key; per-student per-card competence is stored once and read everywhere — this is what makes scaffolding fade coherent across surfaces, and what lets composite skills declare cards by reference instead of redefining them.

---

## 4. The agent runtime

### 4.1 The loop

```
on trigger (see 4.3):
  state   = graph.query(scope from policy) + event diff since last run
  context = conversation (if policy admits it) + student model (per-card competence)
  moves   = evaluate(loaded skill's contracts + card observe-rules, state)
  action  = select ONE next action (or silence), at ≤ intrusiveness cap
  emit    = typed output through enforcement (§6)
  record  = action + verdicts to event stream
```

Design commitments:

- **Exactly one next action per run.** The agent is not a stream of commentary; it is a coach that makes one move. Silence is a first-class output.
- **The agent reads state, never pixels.** All perception is graph queries and event diffs.
- **Model routing:** a cheap model runs trigger predicates and state classification on every event; the flagship model writes intervention text, plans, and adjudicates quality. Assessment never runs on the cheap tier.

### 4.2 The verb set (C3)

The closed set of tool calls. Anything not listed is impossible by construction, not by prompt.

| Verb | Effect |
|---|---|
| `surface_card(card_id, target, entry_stage)` | instantiate a card onto a target; entry stage derived from target's state, not chosen freely |
| `post_intervention(anchor, criterion, body, level)` | one anchored question/diagnosis, at intrusiveness `level` |
| `check_gate(gate_id)` | evaluate a gate's machine-checkable items; report what's missing |
| `plan(route)` / `replan(route, reason)` | write or revise the student's route through the skill's contract DAG (§5.2); the plan is a graph artifact, visible to the student, every revision an event |
| `advance(flow_ref)` | move an authored flow forward (course phase, milestone unlock) — only when its authored condition holds |
| `route(destination, return_anchor)` | send the student elsewhere (course practice, earlier artifact) with a guaranteed way back |
| `invite_commit(target)` | request a snapshot / commitment of a specific artifact |
| `reply(body)` | free conversational turn — only on surfaces whose policy admits it |
| `propose(structured_suggestion)` | a suggestion requiring student disposition (accept / self-revise / reject, with reason) |

All verbs emit **typed outputs** (`question | diagnostic | reference | proposal | plan`). No output type can carry insertable prose (§6).

### 4.3 Triggers (C4): three tiers

| Tier | Source | Agent behavior |
|---|---|---|
| **T-A Summon** | student asks, invokes a card, hits "review" / "I'm stuck" | always runs, always responds |
| **T-B Structural** | artifact created/linked/committed · card stage completed · gate attempted · disposition recorded | always runs; **may** speak (one action max) |
| **T-C Fine-grained** | typing, dragging, dwell | never runs the loop; appended to context silently; accumulated patterns may raise a flag consumed at the next T-B |

Hard silences override all tiers (declared in policy, enforced in runtime): while the student is composing prose; and inside student-written-only fields (warrant, steelman, risk note — these reject non-student `author` at the schema level).

### 4.4 Intrusiveness ladder

Every action carries a level; skills and cards declare the **cap**; the agent chooses at or below it.

| | Form |
|---|---|
| **I0** | silent — log only |
| **I1** | ambient — sidebar hint, non-blocking |
| **I2** | anchored — annotation pinned to artifact/span (e.g. the orphan-evidence flag) |
| **I3** | structured ask — card surfaced, or a `propose` requiring reasoned disposition |
| **I4** | blocking — a gate; only where the skill placed one. The agent can fail a student against an authored gate; it can never invent one. |

### 4.5 Internal decomposition: subagents as configuration

Toward the student there is **one agent** — one voice, one memory, one relationship. Internally, the runtime decomposes into subagents. A subagent is not a service and not a persona; it is a tuple:

```
subagent = (context recipe, prompt, verb subset, model tier, cadence)
```

Same runtime, same graph, different configuration. Four are needed:

| Subagent | Cadence | Context recipe | Prompt assembled from | Verb subset | Model tier |
|---|---|---|---|---|---|
| **Classifier** | every event | event diff only | trigger predicates | `raise_flag` (internal) | cheap |
| **Coach** | T-A / T-B | local graph neighborhood + active card instances + current plan + recent conversation | policy posture + the loaded skill's card observe-rules, golden/banned examples, vocabulary | `surface_card` `post_intervention` `propose` `invite_commit` `check_gate` `route` `reply` | flagship |
| **Planner** | intake · gate passed · replan flag | full graph + contract DAG + per-card competence | the loaded skill's contracts + routing principles | `plan` `replan` `advance` | flagship |
| **Assessor** | offline / batch | event stream projections | dimension rubrics + leap definitions | none — writes to the assessment store | flagship, never downgraded |

The tuples above are the **Project configuration**. The policy layer reconfigures them per module — which subagents are active, what their context recipes contain, and who holds which verb (§5.5). One reassignment matters: **in Course the planner is off, and `advance` belongs to the coach**, callable only when the current phase's authored completion condition holds.

Rules that make this a decomposition rather than a multi-agent system:

- **Coordination is blackboard, not conversation.** Subagents never message each other; they read and write the graph. The planner writes the plan artifact; the coach reads it to decide the next move; when the coach sees the route failing (repeated stalls on one milestone), it writes a replan flag; the planner consumes it on its next run. No inter-agent dialogue means no context lost in handoffs, no persona leakage, and every coordination step is an inspectable artifact.
- **Only the coach addresses the student.** The planner's output reaches the student as the visible plan artifact — introduced and negotiated by the coach's voice. Examiner voices and spot-the-flaw are personas *performed by* the coach, not additional agents.
- **The assessor is isolated from the coach by design.** The coach must not grade its own coaching; the assessor reads only the event stream, and the report's credibility depends on that separation.
- **Skill loading is prompt distribution.** A skill doesn't know subagents exist; the loader partitions its fields — contracts → planner, cards → coach, vocabulary → both, dimension mappings → assessor.
- **Enforcement (§6) sits below all four.** Every outbound action from any subagent passes the same typed-output and output-check stack.

---

## 5. Skills (C5) and the three policies

### 5.1 What a skill answers

Every composite skill answers three questions, and only these:

1. **How is this kind of work taught?** — a set of **milestone contracts** and the dependencies between them, plus pedagogy content (guiding questions, vocabulary, recognition content, golden/banned examples).
2. **How does the teaching interact?** — which view compositions over the primitive library each milestone uses.
3. **Which tools apply?** — the card set (by reference to the registry), and per-milestone repertoires scoping *AI-initiated* surfacing. Student-initiated invocation is never scoped: any card, any time.

```yaml
skill: writing-project           # a project type IS a skill
kind: project
contracts:                       # a DAG, not a sequence
  frame_question:
    requires: [decode_task]
    produces: [research_question, provisional_answer(author=student), preregistration]
    view: review_composition
    repertoire: [qft, operational_definition]
    gate:
      machine: [single_question, terms_defined, preregistration_present]
  build_argument:
    requires: [frame_question, evaluate_sources]
    produces: [claim_nodes(full_proposition), warrants(author=student), concession_node]
    view: argument_composition   # graph + docked annotate
    repertoire: [toulmin, steelman, concession]
    gate:
      machine: [no_orphan_evidence, no_unsupported_claim, no_single_sourced_claim]
      student_written: [warrants, steelman]
      human: [warrant_quality_spot_check]
  # ... remaining milestones
intake: ...                      # §5.2 — mapping arbitrary incoming material to graph state
vocabulary: per-board            # rubric language packs
cards: [craap, sift, toulmin, steelman, ...]   # by reference
```

A **course** is a skill of `kind: course` whose contracts are session phases with a fully binding order — the same format at maximum strictness. A **card** is the atomic case. One registry, three sizes.

### 5.2 Project policy: the agent as planner

**Loaded skill:** a project-type skill (contract DAG). **Active subagents: all four** — this is the only module where the planner runs, and the configuration the subagent tuples in §4.5 describe.

The Project surface is deliberately Cowork-shaped: the agent organizes the work, keeps a visible plan, and brings the student along — because students arrive mid-way, with drafts, with fragments, with needs that no fixed sequence anticipates.

The critical distinction that keeps this from degrading into chat-with-tools: **contracts are fixed; routes are personal.**

- The skill declares **what must become true** — every contract's gate must eventually pass, and the DAG's dependencies hold (you cannot pass `build_argument` before `evaluate_sources`). These encode the pedagogy that must not be optimized away: pre-registration before searching exists to prevent confirmation bias, and no planner may reorder it.
- The agent declares **what to do next** — it reads the student's graph state and plans a route through the DAG: which contract to work on now, what within it, what to defer. The route is re-planned as the graph evolves (`replan`, with the reason recorded).

**Intake.** Every project skill ships an intake procedure: map arbitrary incoming material onto the graph — a pasted draft decomposed into candidate claims and evidence nodes, sources into materials, all marked `author=student, provenance=imported`. Then run every contract's gate against the reconstructed state: some pass outright, some are partial, some are empty. The first `plan` is built from that diff. A student arriving with 800 words does not restart from zero, and does not skip anything either — *the gates are all still owed; the route just starts from where they actually are.*

> This proposes reopening the product spec's removal of reverse entry: that decision priced backfilling as expensive because milestones were bound to pages. With milestones as lenses over one graph, backfill is cheap, and "walk the whole journey" can soften to "owe every gate" without losing any pedagogical guarantee.

**The plan is an artifact.** It lives in the graph, the student sees it, and every revision is an event with a reason — which makes the plan's history exactly the process log the boards award marks for ("the living plan"). The agent maintains it; the student can contest it; contested revisions are dispositions like any other proposal.

**Loops are normal operation.** Milestones are lenses over one graph, not pages. A weak source discovered during drafting gets the evaluation card re-instantiated *on that source, in place* — no navigation "back to step 3." Gates govern what unlocks, never what may be revisited.

### 5.3 Course policy: the coach as script executor

**Loaded skill:** a course skill — contracts are session phases (demonstrate → guided → independent → meta) with a fully binding order. **Active subagents:** classifier, coach, assessor. **Planner: off** — the route *is* the script, so `advance` is reassigned to the coach, and each phase's authored completion condition is checked like a gate before `advance` may fire.

**Trigger model.** Dialogue is the primary rhythm here, unlike Project: every student message in a dialogue phase enters as T-A; manipulations inside a card practice emit T-B as everywhere; phase transitions are themselves T-B events. There is little T-C — course sessions are short and conversational.

**Coach context recipe (course variant):** the session script + current phase goal + this phase's dialogue history + the active card instance + the skill's golden/banned examples. *Not* the full graph — a course's graph is small and session-scoped. The one global read/write is per-card competence, which is how course practice warms up fade everywhere else.

**The coach's runtime freedom:** instantiate the session's declared cards onto the material at hand (the anchor case, or the student's own project if one exists); choose which weakness in the student's answer to press; pace the phase and call `advance` when its condition holds; run the terminal assessment challenge.

**Hard limits:** cannot reorder or skip phases, substitute cards, or leak the terminal assessment. Machine judgment may mark work `draft` or `flagged-weak`, never `solid` — final positive adjudication needs a passed challenge or a human. The posture prompt permits explaining and demonstrating (teaching is the point), but the consolidation rule holds: a card's abstract framework is revealed after use, not before.

### 5.4 Chat policy: the coach alone, with moment detection

**Loaded skill: none.** The full card registry is available. **Active subagents:** classifier, coach, assessor (at supplementary weight). **Planner: off** — there is nothing to route.

**Trigger model.** Every student message is T-A. There is no T-C tier — there is no workspace to manipulate until a card opens one. Attachments and pasted links become `material` objects in a **thread-scoped graph**, so that cards have something to attach to; this thread graph is what makes Chat's cards the same runtime as everywhere else rather than a special case.

**Moment detection is the classifier's job.** On each message, the cheap tier runs moment predicates: *card-shaped moments* (a link pasted → source evaluation; an opinion stated → steelman; a comparison drifting toward a decision → weighing matrix) and *project-shaped conversations* (the thread is circling a real assignment). The classifier raises flags; the coach decides whether to act — an offer declined is not re-raised in the same thread.

**Coach context recipe (chat variant):** the conversation history + the thread graph + flags + per-card competence. `reply` is the default verb; `surface_card` is capped at I3 and must read as an offer, not an interruption.

**Project seeding.** On a project-moment flag, the coach `propose`s installing the matching project skill. On acceptance, that skill's **intake** runs over the thread's materials and card instances, seeding the project graph — the thread's useful fragments arrive as first artifacts, and the planner takes over from there.

**Evidence discipline:** chat events enter the same event stream at supplementary weight, with the on-the-record disclosure the product spec requires. The coach must not farm the conversation for gate items.

### 5.5 Policy summary

| | **Course** | **Project** | **Chat** |
|---|---|---|---|
| Skill loaded | course skill (binding order) | project skill (contract DAG) | none |
| Active subagents | classifier · coach · assessor | all four | classifier · coach · assessor |
| `advance` held by | coach (phase condition) | planner | — |
| Coach context recipe | script + phase + dialogue + card instance | plan + local neighborhood + card instances | conversation + thread graph + flags |
| Graph scope | session-scoped (competence global) | full project graph | thread-scoped (competence global) |
| Agent discretion | inside one exercise | routing + everything between gates | the whole conversation |
| Planner | off (route = script) | on — plan/replan over the DAG | off |
| Conversational trigger | dialogue phases only | anchored talk only | every message |
| Gates | terminal assessment | contract gates | none |
| May explain/lecture | yes | no — question, challenge, diagnose | yes, scoped |
| Card source | skill-declared | repertoire (AI-initiated) + full registry (student) | full registry, moment-triggered |
| Intrusiveness cap | I4 | I4 | I3 |
| Evidence weight | full | full | supplementary, disclosed |

### 5.6 Three traces

One end-to-end trace per module, showing event → subagent → verb. These are normative examples: an implementation that cannot reproduce these shapes has diverged from this spec.

**Course** (X7 steelman, guided-practice phase):

```
student submits steelman attempt for a peer's stance        → T-B
classifier: phase-goal-relevant, no flag
coach   (ctx: phase goal + attempt + golden/banned examples)
        → post_intervention(anchor=attempt's weakest premise, I2):
          "You gave the opposing side its easiest reason. What's their best one?"
student revises; coach judges phase condition met
        → advance(independent_practice)
assessor (offline): scores the exchange as D2/D4 evidence
```

**Chat** (a link pasted mid-conversation):

```
student: pastes news link + "this basically proves my point"  → T-A
classifier: card_moment(source_evaluation) flag; link minted as
        thread material
coach   (ctx: conversation + thread graph + flag)
        → reply (engages the point)
        → surface_card(craap, material, I3) as an offer
student accepts → annotate primitive opens in-thread
card completes → competence updated globally; evidence node in thread graph
[3 sessions later] classifier: project_moment flag
coach   → propose(install writing-project skill, seed from thread)
```

**Project** (student arrives with 800 words already written):

```
skill install → intake: draft decomposed into claim/evidence
        candidates (author=student, provenance=imported)
        → student confirms/corrects the mapping  (first interaction)
planner (ctx: full graph + contract DAG):
        gates checked against reconstructed state —
        frame_question PARTIAL (no preregistration),
        evaluate_sources EMPTY for 5 of 7 sources, build_argument DRAFT
        → plan(route: preregistration → source evaluation → argument repair)
coach   introduces the plan in its own voice; work begins
[days later] repeated stalls on evaluate_sources             → T-B pattern
coach   → raise replan flag
planner → replan(insert route_to_course(x5_source_course), reason logged)
```

### 5.7 The acceptance test

**A new project type is a skill install: contracts + view compositions + card references + intake + vocabulary — zero agent code, zero new primitives.** A skill that needs a new primitive or a new verb is a platform design conversation, not skill authoring.

---

## 6. Enforcement: "the AI never writes" as architecture

The invariant lives below the skill and policy layers and cannot be configured off, on any surface:

1. **Typed outputs only.** No output type carries insertable prose; a `reference` may only quote the student's own prior artifact, verbatim, with provenance.
2. **No write path to prose.** The agent's verbs contain no operation that inserts text into the edit buffer or any student-authored field. Not forbidden — absent.
3. **The output check.** A declarative sentence semantically close to the student's topic is intercepted and rewritten as a question (exception: cross-domain examples, which can't be pasted).
4. **Schema-level authorship.** Student-written fields reject `author != student` at write time.
5. **Audit log.** Every emitted action persists with type, anchor, level, and the output check's verdict; the guardrail metric samples this log.
6. **Banned-phrasing regression suite** per card and stage, run against every model or prompt change.

Skills cannot weaken any of this. A skill declares caps and repertoires — it has no vocabulary for exemptions.

---

## 7. Build order

1. **C1 + the graph.** Two primitives first (`annotate`, `graph`) and the workspace graph with authorship and span indexing — everything else depends on them.
2. **The event stream** (C4). Instrument student-initiated vs agent-prompted from day one; the north-star metric is a projection of this stream and cannot be retrofitted.
3. **The runtime loop + verb set** (C3) with the enforcement stack (§6) — tested hardest against the draw-out stage, where the temptation to write for the student peaks.
4. **The card format** (C2), proven by shipping CRAAP and Toulmin as pure config over the two primitives. Acceptance test: the second card touches zero interface code.
5. **The skill format + gate engine + planner** (C5), the writing-project skill as the proving case — including intake, which is on the critical path for real students (they arrive mid-way).
6. **Course and Chat policies** — by construction, mostly configuration by this point.

---

## 8. Open questions

1. **Intake quality is now load-bearing.** Decomposing an 800-word draft into claim/evidence candidates is itself a hard AI task, and mistakes poison the plan. Does intake output go through student confirmation (probably yes — which is itself a good first interaction)?
2. **Planner discretion vs student agency.** The plan is the agent's proposal; how much can the student override the route (not the gates)? A student who insists on drafting before evaluating sources will hit the gate anyway — is that friction pedagogy or frustration?
3. **Graph scale and scoping.** A project's graph grows for weeks. What scope does each trigger tier query — full graph for T-B, or a neighborhood around the changed node? Latency budget will decide.
4. **Phase-advance adjudication in Course.** "The goal of guided practice is met" is softer than a machine-checkable gate. Golden examples per phase, or human-in-the-loop for the first cohorts?
5. **Concurrent card instances.** Two cards open on the same node (steelman + concession on one claim): one shared observe-loop or two, and who owns the single-next-action budget?
6. **Skill versioning mid-project.** A project runs for weeks; the skill updates underneath it. Pin the version per project, with explicit migration? (Probably yes — contracts changing mid-flight breaks gate comparability.)
7. **Contract fields for future skills.** The DAG assumes milestones with gates. A design-project skill may need parallel tracks or optional milestones — extend the contract format, don't fork the runtime.
8. **Session resumption.** A project spans weeks; what does the coach re-read when the student returns — the plan, the last N events, open flags, the last artifact touched? This is a context-recipe question (cheap to specify, easy to forget), and it is what the product's "situated memory" moment ("last time you were halfway through the counterclaim…") depends on technically.
