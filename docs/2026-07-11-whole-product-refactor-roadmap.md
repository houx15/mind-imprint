# Whole-Product Refactor Roadmap (2026-07)

| | |
|---|---|
| **Status** | Active — decomposition locked (layer-first); Slice 0 spec next |
| **Started** | 2026-07-11 |
| **Sources of truth** | `docs/2026-07-11-product-spec.md` (product: surfaces, pedagogy, red lines, journey) · `docs/2026-07-11-assessment-spec.md` (rubric / engine / dashboard) · `docs/2026-07-11-agent-spec.md` (**technical architecture** — one runtime, skills, three policies) · the Claude Design `思维印记 工作区.dc.html` (student platform, binding UI) |

Top-level decomposition for the second dramatic refactor: reconceive the product around
the three July-2026 specs and the Claude Design mockup. It is **not** a refactor of the
current app — most student-facing Studio behavior is new. We **evolve in place**: keep the
Go backend infra (net/http · pgx/sqlc/goose/river · SSE gateway · DeepSeek), auth, and
org; rebuild the Studio and add the new surfaces on top.

Each slice runs its own **superpowers loop**: brainstorm → spec → plan → subagent-driven
TDD build. We never build the whole thing as one block.

**Authority order:** the agent-spec is the **technical** source of truth (runtime, graph,
contracts, subagents, enforcement). The product spec governs surfaces, pedagogy, red
lines, and journey. Where they disagree it is a flag to reconcile (see Locked decisions),
not an error in either.

---

## Locked cross-cutting decisions

- **One runtime + skills + three policies (agent-spec §0).** Course / Chat / Project are
  the same agent runtime loading authored **skills** under three **policies**. Adopted
  wholesale as the technical foundation.
- **Interaction primitives are the interface foundation (C1).** A finite hand-built
  library (5–8: `annotate` · `graph` · `sort` · `matrix` · `scale` · `compare`), each with
  a fixed state schema and **field-level `author`**. The AI parameterizes primitives,
  never generates UI. **This supersedes the card `interaction_type` enum.**
- **Cards are C2 config over a primitive, not forms.** A card binds a primitive to a
  thinking framework (`primitive`, params, completion, `graph_effects`, `observe` rules,
  consolidation, intrusiveness cap). The existing card library survives as content;
  `subject`/`stage` survive as selection metadata; the old 8 `interaction_type`s retire.
  Same card runs on all three surfaces; `card_id` routes; per-card competence stored once.
- **The core is a workspace graph + append-only event stream, not station tables.** Nodes
  (materials · claims · evidence · card instances · snapshots · plan · gate states), typed
  edges, authorship per field. `graph_effects` compose cards (CRAAP mints evidence nodes
  the Toulmin card consumes). Instrument student-initiated vs agent-prompted from day one —
  the north-star metric is a projection of this stream and cannot be retrofitted.
- **"Stations" are the writing-project skill's contract DAG (C5), not a fixed S0→S6 line.**
  Milestones are lenses over one graph; gates govern *unlock*, never *revisit*. S0–S6 is the
  writing-project skill's particular contract set + gate parameters.
- **Reverse-entry: intake + owe every gate (agent-spec §5.2, reconciling product DEC-8).**
  Project start runs an **intake** mapping incoming material onto the graph
  (`author=student, provenance=imported`); every gate is still owed, but the route starts
  from wherever the student actually is — no forced S0→S6 restart, no gate skipped.
- **Rubric = separation of concerns.** Stations/gates/views follow the product spec/design;
  assessment dimensions follow the assessment spec → **9-dim CT rubric (CT-D1…D9) is
  canonical**, OPCVL (HS-D*) is its own rubric on the same engine, the old essay 6-dim is a
  reporting rollup (assessment §9.1 is the station→dimension bridge). Feedback-comprehension
  (accept/reject/rewrite) stays a distinct signal; standalone-vs-folded decided at report time.
- **"AI never writes" is enforcement below skills/policies (agent-spec §6), un-configurable:**
  typed outputs only · no write path to prose · the output check · schema-level authorship
  (`author != student` rejected at write) · audit log · banned-phrasing regression suite.
- **Subagent decomposition (agent-spec §4.5):** classifier (cheap, every event) · coach
  (flagship) · planner (flagship, Project-only) · assessor (flagship, isolated). Blackboard
  coordination via the graph — subagents never message each other.

## Working agreements

- **Design ⇄ data conflicts are stop-and-discuss.** The `.dc.html` is binding UI, but when a
  screen can't sit cleanly on the graph / runtime, surface it and decide together.
- **Teacher dashboard is gated on design.** Only the student platform is designed today; its
  slice is deferred until the teacher surface exists.
- **TDD + superpowers per slice.** No slice is one block; each produces working, testable
  software on its own.

---

## Slice roadmap (layer-first, agent-spec §7)

Sequenced by dependency: runtime foundation first, surfaces wrap around it. Status: ☐ not
started · ◐ in progress · ☑ done.

| # | Slice | Delivers | Depends on | Status |
|---|---|---|---|---|
| **0** | **Foundations: contracts + graph + events + enforcement primitives** | The five contracts as schemas (C1 primitive state schemas · C2 card format · C3 verb set · C4 event set · C5 skill format), the **workspace-graph data model** (nodes/edges/field-level author/span-index) + **append-only event stream** (studio/course/chat surface tag, unprompted/prompted), rubric + behavior ladders as config (CT-D1…D9, HS-D*), and the **enforcement primitives** (output-check, banned-phrasing suite, typed-output schema, schema-level authorship). Zod + sqlc/goose, all TDD, no UI, no live agent. | — | ☑ |
| **1** | **`annotate` primitive (MATERIAL read-view)** | Hand-build the `annotate` interaction per the design's MATERIAL view: render a span-indexed material with clickable AI/student spans (`AnnotateState` from C1), emit C4 events, controlled component + demo host. **`graph` is split out** to Slice 7 (built with the STRUCTURE view, where its card-driven-vs-map-viz shape resolves in context — decided 2026-07-11). | 0 | ☑ |
| **2** | **Runtime loop + verbs + enforcement stack + classifier & coach** | perceive→evaluate→decide-one→act→record; the verb set (C3) as typed outputs through the enforcement stack (§6); cheap **classifier** (every event) + flagship **coach** (T-A/T-B, one action, I-ladder). No planner yet. | 0,1 | ☐ |
| **3** | **Card format proven: CRAAP (over annotate)** | CRAAP as pure C2 config over the `annotate` primitive — completion, `graph_effects` (mints evidence), observe rules, consolidation, three-key disposition. Acceptance: the card touches zero interface code. **Toulmin** (over `graph`) is proven in Slice 7 when the graph primitive lands. | 2 | ☐ |
| **4** | **Skill format + gate engine + planner + intake** | C5 skill loading; the **writing-project skill** (0457/9239) as contract DAG; gate engine (machine/student/human items, DEC-3 machine-never-`solid`, I4 gates); **planner** (plan/replan/advance) + **intake** (arrive-mid-way → owe every gate). S0–S6 live here as the skill's contracts. | 3 | ☐ |
| **5** | **Studio shell + four-view frame + contract map + coach rail** | Two-tab shell (Chat ∣ Project Space→Writing Studio), first-entry recognition moment, the S0–S6 contract map with gate progress, the 结构/素材/写作/评估 frame + free view-switching, the coach rail + 装备栏 UI. Wires runtime + primitives into the real design. | 4 | ☐ |
| **6** | **Material + source log** (S2/S3) | 素材 view over `annotate`/`compare`, dossier + span highlights, search-plan→auto-log→citations-only-from-log (RL-2), CRAAP vertical + SIFT lateral. | 5 | ☐ |
| **7** | **Structure view** (S1/S4) + **`graph` primitive** | Build the `graph` primitive here (deferred from Slice 1): 结构 view = the Toulmin map visualization with the three pathologies always flagged, nodes created via the coach card flow (`graph_effects`), full-proposition gate, student-written warrant/steelman, concession node, map⇄outline. Also proves the **Toulmin** card (C2 over graph). | 5 | ☐ |
| **8** | **Writing surface + whole-draft review** (S5) | 写作 silent edit buffer (zero model write-path) + preview, immutable snapshots, student-triggered 整稿体检, examiner voices, word budget. | 5 | ☐ |
| **9** | **Readiness + reflect + export** (S0/S6) | 评估 view with the five progress-display renderers (ship 0457 table-by-table first), prediction loop S0↔S6, reflection pack, AI-usage declaration, export forks (RL-4). | 6,7,8 | ☐ |
| **10** | **Assessment engine (assessor) + growth report** | The isolated **assessor** wired live: few-shot MVP engine over event-stream projections, thinking leaps T1–T7, depth-vs-independence, the three reports. | 9 | ☐ |
| **11** | **Chat policy + surface** | Coach-alone + classifier moment-detection, thread-scoped graph, multimodal input, card surfacing as offer, project seeding via intake, supplementary+disclosed evidence. Mostly configuration by now. | 2 | ☐ |
| **12** | **Course policy + surface alignment** | Course skill (binding-order phases), coach-as-script-executor (`advance`), fold the existing course player onto the card contract + assessor. Mostly configuration. | 3,10 | ☐ |
| **13** | **Teacher dashboard** (3 layers) | Class heatmap + risk column, individual trajectory, single-conversation replay, one-click actions. | 10, **teacher-side design** | ☐ deferred |

Cambridge-first (0457 + GPR 9239). Foundations 0–4 = the runtime; surfaces 5–9 = the
Studio (spec Phase 1); 10 = the assessment moat; 11–12 = Chat/Courses; 13 waits on design.

---

## Per-slice log

Each slice appends its spec/plan links and outcome here as it completes.

- **Slice 0** — ☑ **complete** (branch `refactor2-slice0-foundations`, commits `2664dd3`..`30a45fb`).
  Spec `docs/superpowers/specs/2026-07-11-slice-0-foundations-design.md` · plan
  `docs/superpowers/plans/2026-07-11-slice-0-foundations.md`. Delivered: C1–C5 Zod contracts
  (interactionPrimitive · graph · event · agentOutput · cardSpec-evolved · skill) + CT rubric
  container; Go enforcement primitives (output-check · banned-phrasing · typed-output guard ·
  authorship guard); additive migration `0016` (project/graph_node/graph_edge/draft_snapshot/
  edit_buffer/source_log_entry/intervention/disposition/card_competence/chat_thread/chat_message/
  event + nullable project_id on material/card_instances/evaluations) + sqlc queries (project/
  graph/event, event append-only). Gate: contracts 178 green + typecheck; `go build`/`vet`;
  `-short` all green; testcontainers migration + sqlc + pre-existing migrate test green.
  Deferred by design: OPCVL ladders, non-foundation queries, CRAAP/Toulmin port (Slice 3).
- **Slice 1** — ☑ **complete** (branch `refactor2-slice1-annotate`, impl commits `8433275`..`cb77cbf`).
  Spec `docs/superpowers/specs/2026-07-11-slice-1-annotate-primitive-design.md` · plan
  `docs/superpowers/plans/2026-07-11-slice-1-annotate-primitive.md`. Delivered: the `annotate`
  primitive (`apps/web/src/primitives/annotate/` — `segmentBlock` runs + controlled `Annotate`
  component: clickable author-styled AI/student spans, select→dimension/question panel) and the
  信源档案 source-dossier shell (`apps/web/src/workspace/material/` — list↔article↔summary, locked
  count, `source_opened` C4 event) over real China-greening fixtures (Chen et al. 2019 *Nature
  Sustainability*), plus a dev-harness mount. Gate: web suite 405 green + typecheck; controlled,
  no backend/network, `MaterialPane.tsx` untouched. Final review: SHIP, no defects. Deferred:
  student-span text-selection creation, backend persistence + source log (Slice 6), coach rail
  (Slice 2), `graph` (Slice 7).
