# Slice 0 — Foundations: Contracts, Graph, Events, Enforcement

| | |
|---|---|
| **Status** | Draft — for review |
| **Slice** | 0 of the whole-product refactor (`docs/2026-07-11-whole-product-refactor-roadmap.md`) |
| **Sources** | `2026-07-11-agent-spec.md` (technical, primary) · `2026-07-11-product-spec.md` · `2026-07-11-assessment-spec.md` |
| **Delivers** | The five contracts (C1–C5), the workspace-graph data model + event stream, the rubric/ladder config, and the enforcement primitives. **No UI, no live agent.** |

## 0. Purpose and boundary

Slice 0 lays the foundation the whole runtime stands on: the versioned contracts, the
storage for the workspace graph and the event stream, the rubric definitions, and the
"AI never writes" enforcement primitives — everything that must exist before a primitive
can render or an agent can act. It ships as Zod contracts (`packages/contracts`) + Go
mirrors (sqlc/`go:embed`) + goose migrations + tests. It is **pure schema + pure
functions**, fully unit-testable, with **no interface components and no runtime loop**
(those are Slices 1 and 2).

**In scope:** C1–C5 schemas · the graph + event tables · rubric & ladder config · the
enforcement primitives (output-check, banned-phrasing suite, typed-output guard,
authorship write-guard).
**Out of scope (later slices):** hand-built primitive components (1) · the runtime loop,
verbs, subagents (2) · porting real cards (3) · the writing-project skill content, gate
engine, planner, intake (4) · any view or surface (5+).

## 1. Migration strategy — additive, build stays green

The old task/message world has no live data, but the current Go code still compiles
against `tasks`/`messages`. To keep the build and test suite green throughout Slice 0,
**all migrations are additive**: new tables are created; `material`/`card_instance`/
`evaluation` gain a nullable `project_id` alongside their existing `task_id`; nothing is
dropped. Old tables and the code using them stay dormant and are retired **per surface
slice** as each is replaced. A retirement checklist lives in the roadmap's per-slice log
so the end state is tidy.

## 2. The workspace graph (hybrid model)

Aggregate root: **`project`** (replaces `tasks`). Heavy, span-bearing participants keep
their own tables; light argument-graph participants live in a `graph_node`/`graph_edge`
layer. Edges are **polymorphic** — `(from_kind, from_id) → (to_kind, to_id)` — so an edge
can link a `graph_node` claim to a `card_instance`-derived evidence node or a `material`
without forcing handle rows. *(Alternative considered: a thin handle row per heavy node so
every edge is node→node; rejected as more write-amplification for weeks-long graphs. Flag
for the plan if edge queries turn out to need it.)*

Tables (key columns; full DDL in the plan):

```
project(id, user_id, qualification, title, deadline, board_cfg_ver, status,
        created_at, last_active_at)

graph_node(id, project_id, type[claim|evidence|plan|gate_state|note],
           body jsonb, author[student|ai|imported], span_ref jsonb?, created_at)
graph_edge(id, project_id, type, from_kind, from_id, to_kind, to_id, created_at)

material(id, project_id, task_id?, kind[source], source[fetched|pasted],
         title, source_url, blocks jsonb, created_at)          -- evolve: +project_id; sources only
draft_snapshot(id, project_id, seq, content, span_index jsonb, created_at)  -- immutable; the "snapshot" material
-- interventions/anchors target either a `material` (source) or a `draft_snapshot`
edit_buffer(id, project_id, content, updated_at)               -- mutable scratch, NOT a record

source_log_entry(id, project_id, url, title, time_spent_s, takeaway, tier,
                 lateral_read, opened_at)

card_instance(id, project_id, task_id?, card_id, contract_ref, status,
              anchors jsonb, framework_fill jsonb, event_trace jsonb,
              rubric_tags text[], created_at, completed_at)     -- evolve: +project_id, +contract_ref, +framework_fill
intervention(id, project_id, card_instance_id?, type, anchor jsonb, criterion,
             body, level, output_check_verdict, created_at)
disposition(id, intervention_id, action[accept|reject|rewrite], reason, created_at)

card_competence(id, user_id, card_id, scaffold_state, unprompted_count,
                prompted_count, updated_at)                     -- shared with Course

evaluation(id, project_id, task_id?, rubric[ct9|opcvl], scores jsonb, narrative,
           leaps jsonb, status, model, tier, ...usage..., created_at, completed_at)

chat_thread(id, user_id, title, seeded_project_id?, created_at)
chat_message(id, thread_id, role, content, modality[text|voice|file|image],
             attachments jsonb, quoted_fragment, created_at)

event(id, project_id?, user_id, surface[studio|course|chat], type,
      payload jsonb, created_at)                               -- append-only
```

`process_record` and `llm_usage` remain **projections/views**, not tables. `event` is
append-only (no UPDATE/DELETE path in sqlc).

## 3. C1 — Primitive state schemas

Define the **C1 registry shape** plus **`annotate` and `graph` concretely** (the two built
first, Slice 1); `sort`/`matrix`/`scale`/`compare` are authored when their slice needs them
(each is small and additive). **`author` is mandatory on every mutable unit** — the never-
writes enforcement and assessment both read it.

```ts
Annotate = { material_id, spans: [{ id, range|block_ref, tag, note, author }] }
Graph    = { nodes: [{ id, type, text, author }], edges: [{ id, from, to, type }] }
```

Rule captured as a test: every student-mutable field appears in the schema (if the agent
can't read it, it didn't happen).

## 4. C2 — Card format (evolve `CardSpec`)

Evolve today's `CardSpec` into primitive-binding config. Keep `id`/`name`/`rubric_tags`/
`methodology`; **add** `primitive`, `target_type`, `params`, `completion`, `graph_effects`,
`observe`, `consolidation`, `intrusiveness_cap`, and the selection metadata **`subject`**
and **`stage`**. **Retire** `interaction_type` and the form `steps.fields` from the student
path (the primitive holds answers; `params`/`observe` drive the coach's questions).
`methodology` becomes the `consolidation` payload (framework revealed *after* use, R-9).

```yaml
card: craap
primitive: annotate
target_type: material.source
subject: [global-perspectives, history, ...]      # NEW — selection
stage: [S3]                                        # NEW — selection
params: { tags: [...], tag_prompts: {...} }
completion: [ {every_tag_present: [...]}, {field_written_by: {field: risk_note, author: student}} ]
graph_effects: [ {promote: {material -> evidence_node, with: source_quality}} ]
observe: [ {when: "...", move: {verb: post_intervention, level: I2}} ]
consolidation: reveal_framework_after_completion
intrusiveness_cap: I3
```

Existing 33 card JSONs stay valid by gaining fields; **porting CRAAP + Toulmin to the full
format is Slice 3**, so Slice 0 only defines the schema and validates a couple of fixtures.

## 5. C3 — Verb set and typed outputs

Define the closed verb-name enum and the **typed-output union** every verb must emit. The
union is the contract; the verbs execute in Slice 2.

```ts
Verb   = surface_card | post_intervention | check_gate | plan | replan
       | advance | route | invite_commit | reply | propose
Output = { type: "question",   anchor, criterion, body }
       | { type: "diagnostic", anchor, criterion, body }
       | { type: "reference",  anchor, quote, provenance }   // quotes student's OWN artifact only
       | { type: "proposal",   anchor, criterion, body }     // requires disposition
       | { type: "plan",       route }
```

`Output` carries no insertable prose by construction (§10 guards this).

## 6. C4 — Event set + stream

The §14.2 events as a discriminated union, each declaring the dimensions it feeds; plus the
`event` table (§2). `card_clicked` carries the **unprompted-vs-prompted** flag (AI mentioned
the card in the last three turns?) — instrumented from day one, since the north-star metric
is a projection of this and can't be retrofitted.

Events: `prompt_sent` · `card_clicked` · `gate_attempt` · `suggestion_disposition` ·
`verbalization_submitted` · `source_opened` · `citation_added` · `version_saved` ·
`rescue_triggered` · `stance_change_logged` · `chat_message`.

## 7. C5 — Skill format

The contract-DAG schema for composite skills (project types, courses). Content — the
writing-project skill's actual contracts, 0457/9239 gate parameters — is **Slice 4**; Slice
0 defines the format and validates a small fixture.

```yaml
skill: writing-project
kind: project
contracts:
  frame_question:
    requires: [decode_task]
    produces: [research_question, provisional_answer(author=student), preregistration]
    view: review_composition
    repertoire: [qft, operational_definition]
    gate: { machine: [single_question, terms_defined], student_written: [], human: [] }
intake: <declared>
vocabulary: per-board
cards: [craap, sift, toulmin, steelman]      # by reference
```

## 8. Rubric + behavior ladders (config)

CT-D1…D9 (canonical) and HS-D1…D12 (OPCVL) as versioned, Zod-validated config. Each
dimension declares `id`, `name`, `core_question`, `framework_anchor`, and the **L1–L4
behavior ladder** (observable behaviors, SOLO-aligned). The ladder text is a standing
authoring workstream; Slice 0 seeds it from the assessment spec's worked examples and the
current evaluator's dimension text, and validates completeness (every dimension has four
rungs). The 9↔6 rollup map (assessment §9.1) is config, not code.

## 9. Enforcement primitives

Pure, injectable, tested hardest against draw-out. No live agent needed — the embedding
provider is behind an interface so tests stub it; the real provider is server-side only.

- **`outputCheck(text, context) → { verdict: pass|intercept, rewritten? }`** — a declarative
  sentence semantically close to the student's current topic is intercepted and rewritten as
  a question. **Recommended mechanism (review this):** a fast heuristic gate (declarative +
  high topic-token overlap) → embedding cosine-similarity against the student's active
  topic/artifact above a tuned threshold → intercept. LLM-judge is an optional escalation,
  **not** the hot path (cost/latency). **Exception:** cross-domain examples (can't be pasted).
- **Banned-phrasing suite** — a versioned list of forbidden moves (unanchored questions,
  suggested counterclaims, candidate examples) + a matcher; a standing regression suite.
- **Typed-output guard** — validates every `Output`; a `reference` must resolve to the
  student's own prior artifact verbatim with provenance, or it fails.
- **Authorship write-guard** — `rejectNonStudentAuthor(field, unit)` for student-written
  fields (warrant, steelman, risk note); rejects `author != student` at write time.

## 10. Contracts / Go / DB layout and testing

- `packages/contracts/src/` gains: `primitive.ts` (C1), `card.ts` (C2 evolve of `cardSpec.ts`),
  `agentOutput.ts` (C3), `event.ts` (C4), `skill.ts` (C5), `rubric.ts` (evolve),
  `graph.ts` (node/edge), plus enforcement helpers in `enforcement/`.
- Go: sqlc models + queries for the new/evolved tables; `go:embed` for the shared card &
  rubric config; Go mirrors of the enforcement primitives (the ones the runtime will call).
- **Tests:** Zod round-trip + fixture validation; migration up/down under testcontainers;
  sqlc query tests; enforcement unit tests with a stubbed embedding provider and a
  golden/banned corpus. `-short` stays green without Docker for the pure-schema/function
  tests.

## 11. Open questions for the plan

1. Polymorphic edge refs vs handle rows (§2) — start polymorphic; revisit if the argument-map
   queries in Slice 7 want uniform node→node joins.
2. `outputCheck` threshold + provider choice (§9) — needs a small calibration set; the
   threshold is config, not code.
3. Behavior-ladder authoring depth for v1 — seed from existing text now, deepen with
   examiner-experienced teachers as a standing workstream (assessment §10 Q5).

## 12. Acceptance criteria

- All five contracts exist as Zod schemas with Go mirrors, each with round-trip + fixture tests.
- Additive migrations create the graph + event + project world; `up`/`down` pass under
  testcontainers; existing suite stays green (nothing dropped).
- Rubric config validates CT-D1…D9 + HS-D1…D12 with complete L1–L4 ladders.
- The four enforcement primitives pass unit tests, including the draw-out banned-phrasing
  corpus and the cross-domain-example exception.
- No interface component and no runtime loop introduced.
