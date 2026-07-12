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
| **2** | **Runtime loop + verbs + enforcement stack + classifier & coach** | perceive→evaluate→decide-one→act→record; the verb set (C3) as typed outputs through the enforcement stack (§6); cheap **classifier** (every event) + flagship **coach** (T-A/T-B, one action, I-ladder). No planner yet. | 0,1 | ☑ |
| **3** | **Card format proven: CRAAP (over annotate)** | CRAAP as pure C2 config over the `annotate` primitive — completion, `graph_effects` (mints evidence), observe rules, consolidation, three-key disposition. Acceptance: the card touches zero interface code. **Toulmin** (over `graph`) is proven in Slice 7 when the graph primitive lands. | 2 | ☑ |
| **4** | **Skill format + gate engine + planner + intake** | C5 skill loading; the **writing-project skill** (0457/9239) as contract DAG; gate engine (machine/student/human items, DEC-3 machine-never-`solid`, I4 gates); **planner** (plan/replan/advance) + **intake** (arrive-mid-way → owe every gate). S0–S6 live here as the skill's contracts. | 3 | ☑ |
| **5** | **Studio shell + four-view frame + contract map + coach rail** | Two-tab shell (Chat ∣ Project Space→Writing Studio), first-entry recognition moment, the S0–S6 contract map with gate progress, the 结构/素材/写作/评估 frame + free view-switching, the coach rail + 装备栏 UI. Wires runtime + primitives into the real design. **Split 5a (chrome, fixture-backed) / 5b (read-path live wiring) / 5c (interactive loop) / 5d (routing cutover).** | 4 | ◐ (5a ☑, 5b ☑, 5c/5d ☐) |
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
- **Slice 2** — ☑ **complete** (branch `refactor2-slice2-runtime`, impl commits `f666532`..HEAD).
  Spec `docs/superpowers/specs/2026-07-11-slice-2-runtime-coach-design.md` · plan
  `docs/superpowers/plans/2026-07-11-slice-2-runtime-coach.md`. Delivered (evolve the `agent`
  pkg, no orphans): `runtime.go` types + `classifier.go` (`CandidateMoves`, pure predicate:
  unsupported-claim → intervention candidate) + `coach.go`/`coach_prompt.go` (own posture
  prompt, system+user turns, body from model / anchor+criterion from Candidate, full
  enforcement stack) + `loop.go` (`RunAgentStep`: perceive→classify→decide-one→coach→enforce→
  record; silence on no-candidate AND on enforcement rejection, persisting nothing) +
  `agentstore.go` (sqlc adapter, `graph_node.body.text`→`GraphNodeView.Text`, event actor via
  project owner) + `intervention` sqlc queries. Final review found + FIXED two real gaps:
  output-check now handles full-width 。！？ (was ASCII-only → no-op for the Chinese coach), and
  the coach sends a user turn (was system-only → rejected by live providers). Gate: full
  `-short` + testcontainers (loop + store) green; `prompt.go`/`turn.go` untouched. Deferred:
  `surface_card` (Slice 3), planner/intake (Slice 4), UI/SSE (Slice 5), assessor (Slice 10),
  the cheap-model classifier hook (seam only). **RETIREMENT: legacy `RunTurn`/`summon_card`
  (`turn.go`) stays live until the Studio replaces the old workspace, then DELETE in Slice 5.**
- **Slice 3** — ☑ **complete** (branch `refactor2-slice3-cards`, impl commits `7cb736c`..HEAD).
  Spec `docs/superpowers/specs/2026-07-11-slice-3-card-runtime-design.md` · plan
  `docs/superpowers/plans/2026-07-11-slice-3-card-runtime.md`. Delivered: C2 card fields on the Go
  `cards.Spec` + **CRAAP authored as C2 config over `annotate`**; `card_completion.go`
  (`EvaluateCompletion`, `ObserveCandidates`) + `card_effects.go` (`GraphEffects` mints an evidence
  node + `evaluated-as` edge, `ConsolidationPayload`); `card_lifecycle.go` (`SurfaceCard`,
  `CompleteCard`, `RecordDisposition`); `surface_card` + `observe` wired into the Slice-2 loop;
  project-scoped `card_instance`/`disposition` sqlc. **§5.7 acceptance proven**: a second inline
  `note` card surfaces+completes with zero new runtime code. Final review SHIP; fixed 2 follow-ups
  (CompleteCard idempotency guard; rune-count length gates for Chinese). Gate: full `-short` +
  testcontainers (`Refactor2Cards`, `Card|Loop|SecondCard`) green; legacy form path/`RunTurn`/
  migrations/contracts untouched. **DEFERRED debt for Slice 5** (before a live material/answer
  transport lands): an additive migration making the legacy `task_id` NULLABLE on `material`/
  `card_instances`/`evaluations` (0016 added `project_id` but left the legacy FK NOT NULL, so the
  adapter currently borrows the material's `task_id`); and project-scoping `GetCardInstance`.
- **Slice 4** — ☑ **complete** (branch `refactor2-slice4-skill-gate-planner`, commits `154a622`..HEAD).
  Spec `docs/superpowers/specs/2026-07-11-slice-4-skill-gate-planner-design.md` · plan
  `docs/superpowers/plans/2026-07-11-slice-4-skill-gate-planner.md`. Delivered: the **`skills`
  package** (C5 `Skill`/`Contract`/`Gate` types + `go:embed` loader + DAG validation: acyclic,
  requires-resolve, machine-kind-in-closed-set, `TopoOrder`) + `syncskills` tool/`make sync-skills`;
  the **writing-project S0–S6 skill as pure config** (`packages/contracts/skills/writing-project.json`
  ↔ synced mirror); the **gate engine** (`gate.go`: closed machine-predicate set `node_present`/
  `node_count_at_least`/`no_orphan_evidence`/`no_unsupported_claim`/`no_single_sourced_claim`/
  `every_source_evaluated`; `CheckGate` three-tier report, **DEC-3** structural — machine caps at
  `machine_clear`, never `solid`); the **deterministic planner** (`planner.go`: `ReconcileGates`
  owe-every-gate · `Route` advisory reachability over the DAG · `Intake` mints `imported` nodes +
  first plan · `Replan` · `Advance` **DEC-8** blocking unlock, never marks a non-machine item);
  `check_gate` wired into the Slice-2 loop as a **no-model** action (ranked lowest — never starves
  coaching); store: `gate_state`/`plan` graph-node upsert (sqlc + adapter + fake). **§5.7 proven**:
  an inline second skill reconciles/routes/advances with zero new runtime code. Gate: full `-short`
  + testcontainers (`Slice4` intake→route→advance→replan round-trip vs real PG) green; legacy
  task/card paths untouched (`deps.Skill==nil` fully skips the new path). Final whole-branch review
  (opus): Ready to merge — fixed the check_gate-starves-coaching ordering + 2 Minors before merge.
  **MIGRATION 0017** (the spec's "no new migration" was wrong): 0016's `graph_node.type` CHECK
  enumerated 5 values, but `type` is an **open skill-vocabulary field** the gate engine matches —
  0017 relaxes it to a non-empty guard (additive, reversible). **Planner deferred (documented seam):**
  the flagship model-judgment layer (prioritize among simultaneously-unlocked contracts,
  `route_to_course` on stalls) — Slice 4's route is deterministic. **Carry-forward Minors:** skill
  card-refs validated only by a test not at `Load`; Advance upsert-before-event ordering; 0017 down
  fails if skill-typed rows exist. **Slice 5 debt still owed:** legacy `task_id` NULLABLE migration +
  project-scope `GetCardInstance` (from Slice 3), plus a live loop driver + check_gate debounce.
- **Slice 5a** — ☑ **complete** (branch `refactor2-slice5a-studio-shell`, commits `d39963b`..HEAD).
  Spec `docs/superpowers/specs/2026-07-11-slice-5a-studio-shell-design.md` · plan
  `docs/superpowers/plans/2026-07-11-slice-5a-studio-shell.md`. **First UI slice.** DISCOVERY: the
  local `思维印记_工作区.dc.html` was a stale snapshot (old chat workspace); the real Studio design
  (四视图/S0–S6/装备栏) was in the Claude Design project's newer 2767-line version — refreshed in-repo
  (`d39963b`). KEY DESIGN INSIGHT: the four views (结构/素材/写作/评估) and the S0–S6 stations are the
  SAME control — the station rail IS the view switcher (S3=素材, S4=结构, S5=写作, S6=评估; S0–S2 are
  onboarding), which maps 1:1 onto the Slice-4 contract DAG (7 contracts, each with a `view` field).
  Delivered (new `apps/web/src/studio/` tree, controlled + fixture-backed, dev-harness mounted, old
  `workspace/`/`StudentApp`/`Root` untouched — Slice-1 discipline): `Bean` mascot + `StudioState`
  view-model (the 5a↔5b seam) + fixture; `StationRail` (the contract map, per-station state + gate
  strip + soft-lock); `ViewFrame` (station-rail-driven four-view switcher, 素材 reuses Slice-1
  `SourceDossier`); the four view shells (S0 任务解码 recognition + S1/S2 shells · 结构/写作/评估 shells,
  deep interactions deferred to 6–9); `CoachRail` (thread + **三键处置** `DispositionCard` ≥15-**rune**
  gate matching the backend + **装备栏** `EquipmentBar` + `MethodologyModal` + composer); `StudioShell`
  (3-column layout + focus mode). Gate: full web suite 446 green + tsc clean; controlled (no fetch/
  SSE/model), boundary held. Final whole-branch review (opus): Merge after fixes — fixed 2 Important
  (equip→meth mapping, modal clipped to coach rail → lifted to shell) + revise→rewrite drift + double-锚定.
  **CARRIED to 5b:** `CoachMessage.ai` anchor field (chip+锚定-label split — arrives with the backend
  projection). **5b will owe:** projects API + `projectToStudioState` projection (skill contracts →
  stations, gate reports → gate progress, plan → coach anchor) · live coach (post_intervention/
  check_gate) over SSE · real disposition persistence · the app-routing flip + retiring the old chat
  `workspace/` + legacy `RunTurn`/`turn.go` · the Slice-3 deferred debt (task_id-nullable migration +
  project-scope `GetCardInstance`) · a live loop driver + check_gate debounce.
- **Slice 5b** — ☑ **complete + MERGED to main (`fe0e5bc`)** (branch `refactor2-slice5b-studio-readpath`,
  20 commits `d3333b9`..`fe0e5bc`). Spec `…/specs/2026-07-11-slice-5b-studio-readpath-design.md` · plan
  `…/plans/2026-07-11-slice-5b-studio-readpath.md`. **First backend-wired UI slice — READ PATH ONLY**,
  11 tasks subagent-driven TDD. **DECISION: 5b decomposed 5→5a/5b/5c/5d; 5b = read path only** (brainstorm
  fork). Delivered: **lean `StudioProjection` Zod contract** (`packages/contracts/src/studioState.ts`: shared
  view types + wire DTO — NOT the full frontend StudioState; frontend maps it, stubbing deferred center
  panes) + Go↔Zod parity DTO (`internal/studio/dto.go`); **pure Go read-projection** `internal/studio`
  (`projection.go` stations via `TopoOrder`+`ReconcileGates`, coach anchor/thread from interventions,
  equipment from card_instances, onboarding from decode_task nodes; `load.go`); **skill contract `title`s**
  (S0–S6 Chinese names on `writing-project.json`); **`agent` pure helpers** `GraphViewFromRows`/
  `RecordedGatesFromNodes` + **`agent.ItemResult.Attempted`** (genuine-vs-vacuous gate progress, mirrors
  `evalMachineItem`'s per-predicate scans incl. article-only materials); **migration 0018** seed demo
  Task(admin)+Project(Phoebe) ≈ STUDIO_FIXTURE (S4 current, orphan-evidence); **`GET /projects` + `/projects/{id}`**
  (ownership 404-not-403); frontend **`api/projects.ts`** (list-unwraps-envelope / detail-Zod-parses) +
  **`state.ts` adopts contract types** + CoachRail `锚定` anchor render + live **`StudioContainer`** mapping
  projection→StudioState + additive **`?studio`** route (AppShell/Root/workspace UNTOUCHED). Gate: full Go
  suite (testcontainers) exit 0 + web **453** green + tsc clean. Final whole-branch review (opus) = merge
  WITH FIXES → **caught a false green "门禁通过" banner** on the S4 landing view (StructureView `allClean`
  vacuously-true on stubbed `structure:[]`) → FIXED (deferred-shell placeholder when empty) + seed
  aggregate-comment + `Project` dedup. **HARD CARRY-FORWARDS — 5c:** onboarding producer↔reader shape
  (`agent.Intake` writes `{text}` vs reader's `{restate_prompt,rows}`); Slice-3 debt (task_id NULLABLE ripples
  `Material/CardInstance.TaskID`→pgtype.UUID + project-scope `GetCardInstance`); live coach loop/disposition/
  composer. **5d:** `StudioContainer.defaultEnsureSession` silently signs in as Phoebe on any getMe failure
  under `?studio` — MUST gate before the Studio is the real student surface; routing flip + retire workspace//turn.go.
- **NEXT = Slice 5c** (interactive loop): live coach (post_intervention/check_gate) over SSE + a loop driver +
  check_gate debounce; real disposition persistence + composer send → chat; the Slice-3 deferred debt; the
  onboarding producer↔reader reconciliation. Then 5d (routing cutover), then 6–9 (deepen each center-pane view).
