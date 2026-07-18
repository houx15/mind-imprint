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
| **5** | **Studio shell + four-view frame + contract map + coach rail** | Two-tab shell (Chat ∣ Project Space→Writing Studio), first-entry recognition moment, the S0–S6 contract map with gate progress, the 结构/素材/写作/评估 frame + free view-switching, the coach rail + 装备栏 UI. Wires runtime + primitives into the real design. **Split 5a (chrome, fixture-backed) / 5b (read-path live wiring) / 5c (conversational loop) / 5c-2 (tool-card transport) / 5d (routing cutover).** | 4 | ☑ (5a ☑, 5b ☑, 5c ☑, 5c-2 ☑ [transport; CRAAP live mint → Slice 6], 5d ☑ [routing cutover; old task surface retired]) |
| **6** | **Material + source log** (S2/S3) | 素材 view over `annotate`/`compare`, dossier + span highlights, search-plan→auto-log→citations-only-from-log (RL-2), CRAAP vertical + SIFT lateral. | 5 | ☑ (keystone ☑ CRAAP fill→mint live; 6b ☑ material center-pane + project-scoped ingestion + source log; 6c ☑ **`compare` primitive + SIFT lateral + `cross_check` mint + S3 machine-gated** — 6c's own "complete" was written before whole-branch review found SIFT code-complete but **unreachable** and a card-clobber data-loss risk; fixed by FIX-A..FIX-E (see the 6c entry below), genuinely reachable and reload-safe as of FIX-E. Carry-forward: the search-plan card still needs its own design) |
| **7** | **Structure view** (S1/S4) + **`graph` primitive** | Build the `graph` primitive here (deferred from Slice 1): 结构 view = the Toulmin map visualization with the three pathologies always flagged, nodes created via the coach card flow (`graph_effects`), full-proposition gate, student-written warrant/steelman, concession node, map⇄outline. Also proves the **Toulmin** card (C2 over graph). | 5 | ☑ |
| **8** | **Writing surface + whole-draft review** (S5) | 写作 silent edit buffer (zero model write-path) + preview, immutable snapshots, student-triggered 整稿体检, examiner voices, word budget. | 5 | ☑ (keystone + 8b — see below; **8b ☑** examiner-voice switching + budget-deletion coaching; EE/AP board-specific passes still deferred to their board packs) |
| **9** | **Readiness + reflect + export** (S0/S6) | 评估 view with the five progress-display renderers (ship 0457 table-by-table first), prediction loop S0↔S6, reflection pack, AI-usage declaration, export forks (RL-4). | 6,7,8 | ◐ (**readiness gauge ☑** — 0457 就绪度 made real from the whole-draft review, `644e1d2`; reflect / prediction-loop S0↔S6 / self-score / AI-usage declaration / export forks + the other 4 skins still deferred) |
| **10** | **Assessment engine (assessor) + growth report** | The isolated **assessor** wired live: few-shot MVP engine over event-stream projections, thinking leaps T1–T7, depth-vs-independence, the three reports. | 9 | ◐ (**assessor keystone ☑** — isolated few-shot MVP engine scores the seeded CT rubric over the project event-stream projection → per-dim L1–L4 + evidence + growth narrative in the 成长报告 slot, `50bda49`; T1–T7 leaps / depth-vs-independence radar / teacher+parent report versions / OPCVL / benchmark+fine-tune / chat+course aggregation still deferred) |
| **11** | **Chat policy + surface** | Coach-alone + classifier moment-detection, thread-scoped graph, multimodal input, card surfacing as offer, project seeding via intake, supplementary+disclosed evidence. Mostly configuration by now. | 2 | ◐ (**coach+card-as-offer keystone ☑** — standalone Chat surface: coach-alone `RunChatStep` (reply typed output, guiding posture), classifier link→CRAAP moment, `surface_card`@I3 in-thread offer (confirm-to-open, reuses StudioCardSheet), thread-scoped card runtime via additive `thread_id` on material/card_instances (0022), on-record disclosure (binding dc.html), chat events → stream at supplementary weight, `d1b0d4c`; multimodal / project-seeding / off-record control / competence wiring / thread evidence nodes / semantic non-link moments still deferred) |
| **12** | **Course policy + surface alignment** | Course skill (binding-order phases), coach-as-script-executor (`advance`), fold the existing course player onto the card contract + assessor. Mostly configuration. | 3,10 | ◐ (**coach-as-script-executor keystone ☑** — 课程 runs on the runtime under Course policy: a course skill of 4 binding phases wrapping the existing steps, `RunCourseStep` with the coach holding `advance` behind a **structural floor** enforced in Go, session-scoped card practice, the binding 问印记 ask panel, course events → stream at **full** weight, `ed7e019`; assessor aggregation into 成长报告 / terminal-assessment adjudication / golden+banned packs / voice / competence / course→project seeding / multi-course authoring / session restart still deferred) |
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
- **Slice 5c** — ☑ **complete + MERGED to main (`4c14fd8`)** (branch `refactor2-slice5c-interactive-loop`,
  16 commits `cd2d4fa`..`4c14fd8`). Spec `…/specs/2026-07-11-slice-5c-interactive-loop-design.md` · plan
  `…/plans/2026-07-11-slice-5c-interactive-loop.md`. **The coach rail goes two-way live — CONVERSATIONAL
  loop only (no tool-cards, no migration).** 11 tasks subagent-driven TDD. **DECISIONS (brainstorm):**
  5c decomposed → conversational-loop-now / tool-cards→5c-2 (the map showed post_intervention+check_gate are
  card_instances-free; only surface_card pulls the Slice-3 debt) · **chat-aware coach** (feed the student msg +
  recent thread into the coach model turn) · **Similarity = keyless lexical** (rune-bigram Jaccard, no
  embeddings/key — embeddings deferred) · **the coach reply IS the intervention row** (no dup assistant
  chat_message; context + projection both merge student chat_messages ⋈ interventions). Delivered: `chat.sql`
  (thread/message queries, project-joined) + `AgentStore` chat seam (`ChatTurn`/`LoadChatHistory`/
  `CreateChatMessage`); chat-aware `ProposeIntervention`/`BuildCoachContext`; `AgentDeps.SkipSurfaceCards`
  (zero-value = current behavior) + history threading in `RunAgentStep`; **exported keyless
  `enforcement.LexicalSimilarity`**; gateway `SSEWriter.Intervention`/`.Gate`; **`POST /projects/{id}/turn`**
  (ownership+entitlement-before-stream, heartbeat clone, ONE RunAgentStep/turn [一次只问一个], silence-legal,
  cards-off); **`POST /projects/{id}/interventions/{iid}/disposition`**; `projectCoach` merges student bubbles;
  frontend `api/studioTurn.ts` (SSE client) + `studio/conversation.ts` (live controller) + `StudioContainer`
  composer/disposition wired live via `useSyncExternalStore` + composer `sending`. Gate: full Go suite
  (testcontainers, serialized `-p 1`) exit 0 + web **457** green. **Per-task reviews caught + FIXED 2 real
  bugs:** a **cross-tenant IDOR** on disposition (iid wasn't scoped to the owned project → now membership-checked,
  404-no-leak) and a **`useSyncExternalStore` deviation** (reverted; test fixture stabilized). Final whole-branch
  review (opus) = **Ready to merge — Yes** (no Critical/Important; verified wire-contract Go↔TS, no reply dup,
  reload coherence, all 4 red lines, auth, back-compat); pre-merge polish wave (parse-guard + slog.Warn ×2 +
  disposition retry-on-failure). **CARRY-FORWARDS — 5c-2:** surface_card + tool-card fill/refeed + card_instances;
  live `gate` handling (controller drops gate events; studioturn gate emit hardcodes 0,0); multi-step/debounced
  loop; token-streaming; a live anchor label (runtime stores `{kind,id}` → live+reload both chip-only). **cleanup:**
  Slice-3 debt (task_id NULLABLE + project-scope GetCardInstance); unique partial index on
  `chat_thread.seeded_project_id` (getOrCreateThread TOCTOU); onboarding live producer. **5d:** retire
  RunTurn/turn.go/workspace; routing flip; gate `defaultEnsureSession`.
- **Slice 5c-2** — ☑ **complete + MERGED to main (`5e5b12b`)** (branch `refactor2-slice5c2-tool-cards`,
  15 commits `2af6c98`..`5e5b12b`). Spec `…/specs/2026-07-11-slice-5c2-tool-cards-design.md` · plan
  `…/plans/2026-07-11-slice-5c2-tool-cards.md`. **The CRAAP tool-card TRANSPORT + round-trip plumbing goes
  live** (RE-SCOPED — see below). 10 tasks subagent-driven TDD. Delivered: `SubmitProjectCardInstance` sqlc
  query; `AgentStore` card seam (`SetCardInstanceStatus`/`SetCardInstanceAnchors`/`SubmitProjectCardInstance`) +
  `Action.CardID`; `studioturn` `SkipSurfaceCards:false` + `surface_card`→`card` SSE case + shared `streamAction`;
  **project card endpoints** `POST /projects/{id}/cards/{cid}/{activate,submit(SSE),skip}` (submit = persist +
  `CompleteCard` + one refeed `RunAgentStep`, ownership 404-no-leak); frontend `card` `StudioTurnEvent` +
  `api/projectCards.ts` + conversation card state + **`StudioCardSheet`** (reuses `pickCardBody`/`CardRenderer`/
  `envelopeReducer` — no fork, no craap special-case) + CoachRail live slot + `StudioContainer` wiring. Gate:
  full Go suite (serialized `-p 1`) exit 0 + web **469** green. **Per-task reviews caught+FIXED 2 real bugs**
  (T5 vacuous evidence-mint assertion [seed had a pre-existing evidence node → now asserts a NEW node by
  id-delta+body]; T8 `submitCard` clobbering a refeed-surfaced card [guard the done-clear by id]).
  **WHOLE-BRANCH REVIEW (opus) CAUGHT A CRITICAL all 10 per-task reviews missed → RE-SCOPED:** the *live*
  CRAAP mint can't happen — **CRAAP is an annotation card** (`EvaluateCompletion` reads **anchors**, not
  `field_values`), but the wired schema-field renderer only produces `field_values`; the anchor-producing
  material-annotation surface is **Slice 6**. So 5c-2 ships the **transport** (all seams verified, mint proven
  *given* a satisfying anchor set), gates `status=completed` on `CompleteCard` (a form-only submit stays
  `active`, no false-complete), and the docs' Acceptance/Deferred were corrected to state the live mint is
  deferred to Slice 6. **→ SLICE 6 now ALSO owes the CRAAP fill→mint bridge** (the annotation surface produces
  the anchor-based fill CRAAP's completion needs). Carry-forward Minors (batch): swallowed `AppendEvent` on
  activate/skip now `slog.Warn`'d; non-transactional card writes (matches `putCard`); spec/plan Goal sections
  still read as the aspirational target (Known-limitation callout precedes them).
- **Slice 6 (keystone)** — ◐ **KEYSTONE COMPLETE** (branch `refactor2-slice6-craap-fill-mint`, impl commits
  `969ff3d`..`f1f3b93`). Spec `…/specs/2026-07-11-slice-6-craap-fill-mint-design.md` · plan
  `…/plans/2026-07-11-slice-6-craap-fill-mint.md`. **The CRAAP fill→mint bridge is LIVE** — a real student now
  completes a source-evaluation card end-to-end through the coach rail, minting an evidence node. 8 tasks
  subagent-driven TDD (backend T1–T3, frontend T4–T7, suite T8). **Scope: S3-vertical keystone only** (AI
  anchors per-dimension questions → student answers + writes 作用与风险 → lock → mint), guidance level **L1**
  (AI authors + circles spans; the `author` field is the seam kept open for L2/L3 = student finds spans /
  elicits questions). Delivered: (T1) **the deeper disjunction fix** — `AnchorGenerator` now keys anchors off
  `params.tags` so `dimension` == the completion tag (was `Steps[].Title` → could never satisfy
  `EvaluateCompletion`; incidentally revived CRAAP's dead `ObserveCandidates` nudge rules); (T2) Studio surface
  seam generates + persists + emits AI anchors on annotate-card surface (`streamAction`→method, graceful-degrade
  to `[]`); (T3) **non-vacuous** surface→fill→submit→mint E2E driving the *generated* anchors (the 5c-2 review's
  demand); (T4) `StudioAnnotateCard` — authorship-agnostic answer-mode producing filled anchors + a student
  `risk_note` anchor, binding design copy verbatim; (T5) conversation carries anchors; (T6) CoachRail forks
  `annotate`→`StudioAnnotateCard` — **full anchor flow traced end-to-end** (SSE→conv→container→shell→rail; lock
  →submit body intact); (T7) SourceDossier anchor-span highlight (seam landed). Submit→mint→gate→refeed is
  untouched 5c-2 code. Gate: full Go suite serialized `-p 1` exit 0 + web **475** + contracts **181** + tsc
  clean. **DEFERRED (own follow-ups):** **6b** = material center-pane view (un-stub `views.material: []` in
  `StudioContainer.toStudioState`, then thread `card.anchors` via `ViewFrame` to light up T7's left-pane
  highlight) + source-log S2 (search-plan→auto-log→citations, RL-2); **6c** = SIFT lateral; also student *free*
  span-creation (L2/L3) and the R-9 summing-up framework reveal. T4 Minor for triage: `RISK_NOTE_QUESTION` const
  drops "／局限" vs the placeholder's binding copy.
- **Slice 5d** — ☑ **complete** (branch `refactor2-slice5d-routing-cutover`, commits
  `f627662`..`f1e5230`, 9 tasks subagent-driven TDD). Spec
  `docs/superpowers/specs/2026-07-11-slice-5d-routing-cutover-design.md` · plan
  `docs/superpowers/plans/2026-07-11-slice-5d-routing-cutover.md`. **The Studio becomes the only
  student surface** — the old task-based workspace is retired, routing cut over, and the console
  re-pointed at the project model. Delivered: (T1) backend — deleted `internal/api/{turn,tasks,
  cards,evaluate,material,eval_trigger}.go` + `internal/agent/turn.go` (`RunTurn`/`TurnStore`) +
  the 13 old `/api/v1/tasks/**` routes (route-surface 404 fence added); (T2) deleted the
  task-coupled evaluator (`eval*.go` + testdata) and its river `EvaluateWorker` registration; (T3)
  re-pointed `GetClassRoster`/`GetSchoolCounts` at `project`/`evaluations.project_id`/
  `card_instances.project_id` (was `tasks`, which after the cutover would read 0/never for every
  real student) + added `TouchProject` and wired it into `postProjectTurn`, with a testcontainers
  test that **fails against the old `tasks`-based query** as proof the re-point is real, plus
  `task_count`→`project_count` renamed through the sqlc row/Go DTO/console TS/`ClassDetailView`/
  `OverviewView`; (T4) left rail moved to the binding design (工作室/成长报告, four items); (T5)
  `StudentApp`'s 工作室 tab mounts the Studio directly, 成长报告 becomes a placeholder slot; (T6)
  `StudioContainer` drops `defaultEnsureSession` (the silent sign-in-as-Phoebe fallback) for an
  honest empty-vs-error state, with a signin-spy test that **fails against the current fallback**
  as proof it's truly gone; (T7) `Root` drops the `?studio` side door; (T8) the great deletion —
  `shell/WorkspaceContainer.tsx`, `shell/directory/`, `shell/records/`, `agent/` (old
  conversation/evaluator hooks), `api/{turn,tasks,cards,evaluate,materials}.ts`, `store/`,
  `dev/StorePanel.tsx`; `workspace/Markdown.tsx`→`cards/Markdown.tsx` and
  `workspace/material/`→`studio/material/` (the only two genuinely-shared survivors) moved out so
  `apps/web/src/workspace/` ceases to exist; (T9, this task) contracts cleanup + sqlc orphan sweep
  + `TouchProject` on card submit too + whole-repo gate + this roadmap entry.
  **T9 additions found by the per-task reviews (beyond the T9 brief):** `submitProjectCard`
  (`internal/api/projectcards.go`) also drives `RunAgentStep` — filling a card is student activity
  too — so it now touches `last_active_at` the same failure-safe way (`slog.Warn`, never fails the
  submit/SSE stream), with a new `TestProjectCardSubmit_TouchesLastActiveAt` modeled on T3's touch
  test; and a caller-less-query sweep of `tasks.sql`/`cards.sql`/`messages.sql` deleted `GetTask`
  (+ its one test-only round-trip usage in `sqlc_test.go`), `MarkTaskEvaluated`,
  `CountCompletedCards`, `CountSubstantiveTurns`, `GetCard`, `ListCardsByTask` — all verified
  zero-caller (production and tests) by repo-wide grep before deletion; the legacy task-scoped
  `CreateCardInstance`/`SetCardActive`/`SubmitCard`/`SkipCard`/`SetCardAnchors`/`AppendMessage`/
  `ListMessagesByTask` were left alone (still test-covered generated-code round-trips, and
  `card_instance.sql`'s own header comment explicitly protects the first five for "the legacy
  path"). **Contracts:** deleted the orphaned `Task`/`TaskStatus`/`Message`/`MessageRole`
  (`packages/contracts/src/task.ts`) and `Material`/`MaterialKind`/`MaterialSource`/
  `MaterialBlock` (`src/material.ts`) schemas + their tests — zero importers anywhere in
  `apps/web` after T8's deletion, confirmed by grep + `tsc --noEmit`; `Evaluation` was **kept**
  because `cognitive-model.ts`'s `assembleImprint` still imports it internally within contracts.
  **Explicitly preserved (per spec §4.2):** the `tasks`/`evaluations` tables and data (no
  migration — dropping tables is destructive and `evaluations.project_id` is Slice 10's write
  target); `internal/materialize` (URL→blocks fetcher, no caller — 6b wires it); `material.sql`'s
  `CreateProjectMaterial` (test-covered but no production caller yet — same forward-looking
  category as `internal/materialize`, reserved for 6b) + `ListMaterialsByProject` (live production
  caller). **Accepted gaps (spec §6, inherited by the next slices):** material ingestion stays
  Studio-unreachable until **6b** wires `internal/materialize` + `CreateProjectMaterial` (no
  capability lost — the old ingestion endpoints were task-scoped and the Studio could never call
  them; the seeded project ships its materials so the CRAAP keystone keeps running in the window);
  evaluation is deferred to **Slice 9/10**'s assessor (a different engine over event-stream
  projections — the 成长报告 slot is where it lands); project creation (`POST /projects` +
  intake) has **no home yet** — 5d deliberately stayed a true cutover, a zero-project student sees
  the honest empty state, and creation needs its own design + slice. **Carry-forwards (spec §9):**
  Slice-3 debt on `card_instances.task_id`/`material.task_id` NOT NULL is now unblocked (the task
  *surface* is gone) but still not done — ripples into `pgtype.UUID` Go types + project-scoped
  `GetCardInstance`; a unique partial index on `chat_thread.seeded_project_id` (TOCTOU); the
  onboarding live producer (`agent.Intake` writes `{text}`, the reader wants
  `{restate_prompt, rows}`). Gate (T9): full Go suite serialized `-p 1` exit 0 (all packages) +
  web **279** (was 475 pre-5d; drop is deleted-suite fallout, not a regression) + contracts
  **165** (was 181) + `tsc --noEmit` clean.
  **WHOLE-BRANCH REVIEW (opus) CAUGHT 2 DEFECTS ALL 9 PER-TASK REVIEWS MISSED** — both invisible
  from inside any single task, both with a green suite, and both FIXED before merge (final commits
  `83c2a65`, `40f1b8f`, `5e16c4e`):
  - **CRITICAL — LLM token/cost accounting had no live writer.** The deleted `agent/turn.go` (T1)
    and rubric evaluator (T2) were the *only* writers of `provider/model/tier/prompt_tokens/
    completion_tokens/cost_estimate`, and the `llm_usage` view unions exactly those two tables —
    while the surviving path (`coach.go`/`anchors.go`/`course.go`) called `gateway.Collect` and
    **discarded `res.Usage`** (a gap dating to 5c; 5d deleted the last thing that still recorded
    anything). Net effect: every DeepSeek call a student made burned real money persisted
    **nowhere**, the admin usage panel read 「暂无用量。」 forever, and the `AGENTS.md` hard
    constraint (记录档位 + token + 成本) was broken. FIX: migration **`0019_llm_call_usage.sql`** —
    a typed `llm_call` table (the new model has no row every call maps onto: a silence-legal turn
    persists nothing, anchor-gen writes onto a card) + `llm_usage` re-pointed to a **3-arm UNION**
    (new `llm_call` + the frozen historical `messages`/`evaluations` arms, kept verbatim so old
    spend survives); usage plumbed OUT of the three pure call sites and persisted by their callers
    via a new `AgentStore.RecordLLMCall` seam; cost formula recovered verbatim from the deleted
    `turn.go` (`gateway.EstimateCost`/`CostNumeric`), not reinvented; best-effort (`slog.Warn`,
    never fails a turn/submit/render). `AGENTS.md`'s 存储 row updated (its 「无 `llm_calls` 表」
    note described the schema this slice deleted). Regression test proven RED by mutating the tree.
    Follow-up `slog.Warn`s added for the two silent-unmetering paths (unpriced model → $0.00 reads
    as free usage; a usage-silent provider → coach turn records nothing).
  - **IMPORTANT — voice input was silently dead and the Studio shipped an inert mic.** T8 deleted
    `workspace/Composer.tsx`, the only host of `AsrStream`/`MicCapture` — while `CoachRail` renders
    a 语音输入 mic with `role="button"` + `cursor: pointer` and **no `onClick`**. A student would
    click, speak, and get nothing: no recording, no error, no feedback, and no other way to speak
    to the coach anywhere in the product. FIX: ported the deleted composer's ASR behavior onto the
    coach-rail composer (reusing `AsrStream`/`MicCapture` as-is — no fork): real mic handler,
    recording state, transcript lands **editable** in the composer (never auto-sent), and
    permission/ASR errors surface in a `role="alert"` banner (the old composer only
    `console.warn`ed them). Also swept the stale 批判思维 course copy → 写作工作室 (binding design)
    and deleted the now-orphaned `cognitive-model.ts` + `evaluation.ts` contracts modules.
  **Final gate:** full Go suite serialized `-p 1` exit 0 + web **282** + contracts **155** +
  `tsc --noEmit` clean. Whole-branch verdict after fixes: **Ready to merge — Yes.**
  **NEW carry-forwards out of the review (non-blocking):** the `source:"voice"` turn-provenance tag
  is gone (the Studio's `conv.send(text)` has no such param — 「过程即数据」 loses the did-this-turn-
  originate-from-speech signal; needs the turn contract); `surfaceAnchors`/`renderChallenge` bail on
  empty anchors *before* the metering block (unreachable today, but re-opens the hole if a card spec
  ever ships with neither tags nor steps); the Studio composer has no Enter-to-send (the deleted
  composer had it); the binding design itself still says 「批判思维」工作台 in two spots
  (`dc.html:158`, `:2519`) and should be corrected so the next implementer doesn't restore it.
- **Slice 6b** — ☑ **complete** (branch `refactor2-slice6b-material-view`, commits
  `b2770e6`..`1096a85`, 10 tasks subagent-driven TDD). Spec
  `docs/superpowers/specs/2026-07-11-slice-6b-material-view-design.md` · plan
  `docs/superpowers/plans/2026-07-11-slice-6b-material-view.md`. **The material view goes live** —
  a student can now open 素材, read the two seeded sources with their real article text, add a
  third source herself (URL fetch or pasted text), watch the coach's CRAAP anchors light up in the
  article on the left while she answers on the right, lock the card, and see her own 作用与风险 line
  reflected in the dossier chip. Delivered: (T1) `MaterialSource`/`MaterialBlock` Zod contracts +
  `StudioProjection.materials` (required array); (T2, fix `51e213d`) migration `0020` — nullable
  `material.task_id`, `source_log_entry.material_id` + index, the demo seed's materials filled with
  the **real article text** (they had carried `blocks='[]'`, meaning the CRAAP anchor generator had
  been running against empty text since Slice 6), 2 seeded source-log entries, `queries/source_log.sql`
  (4 queries); (T3, fix `f44cc66`) `studio.Load` loads materials + source log; `MaterialDTO` with
  DERIVED `locked` (an `evaluated-as` edge exists) / `role` (the minted evidence node's
  `source_quality.risk_note`) / `tier` + `takeaway` (the student's source-log entry) / `anchors`
  (anchors whose own `material_id` matches); `dto_parity_test` extended; (T4) `POST
  /projects/{id}/materials` — student-only ingestion (URL fetch via `internal/materialize`, first
  production caller, or pasted text), one transaction writes `material` + `source_log_entry`, 201 =
  `studio.MaterialDTO`, `task_id` NULL, behind `HasEntitlement`, ownership hidden as 404 — **RL-2
  enforced structurally: the agent has no ingestion path at all**; (T5) `POST
  /projects/{id}/materials/{mid}/open` — accumulates `time_spent_s` (never overwrites) and appends
  the system's first `source_opened` events, closing a real IDOR the brief hadn't specified
  (`AddSourceTimeSpent`/`GetSourceLogByMaterial` had no project filter); (T6) `api/materials.ts`
  client; `views.material` un-stubbed to the real projection; the `fixtures.ts` stub + its test
  deleted; (T7) dossier chips derive from state (`待评估` / `✓ 已锁定`), `role === ""` → `尚未写「作用与
  风险」`, the reading timer reports elapsed seconds (also on unmount); (T8) `AddSourceForm` (URL
  tab / paste tab, required 一句话摘要 + 层级, `role="alert"` server-error copy) + `SourceLog` (检索日志
  ledger); (T9, fix `1096a85`) threaded the live card's anchors `StudioShell` → `ViewFrame` →
  `SourceDossier` — the highlight seam Slice 6 landed dormant and nothing had ever passed through.
  **Three cross-layer defects found and fixed (the most valuable record of this slice):**
  1. **T2 — the FK trap.** `card_instances.task_id` was still `NOT NULL REFERENCES tasks(id)` while
     `CreateCardInstance` narrowed a NULL material `task_id` to the zero UUID — an FK violation the
     instant 6b's own acceptance path (a task-less material gets a card surfaced on it) ran. RED
     reproduced by both the implementer and the reviewer (`card_instances_task_id_fkey`, SQLSTATE
     23503). Fix: migration 0020 drops the NOT NULL on `card_instances.task_id` as well as
     `material.task_id` — **this also corrects the spec's §8/§11 scope note**, which had said the
     `card_instances` NOT NULL would stay a carry-forward; it did not survive T2.
  2. **T3 — a Slice-6 mint defect nothing had ever read.** `agent/card_effects.go`'s
     `sourceQuality()` folded only `spec.Params.Tags` into the minted evidence node, silently
     dropping the student-written `risk_note` anchor (作用与风险) captured at mint time. Invisible
     since Slice 6 because nothing consumed `role` until 6b's dossier did; without the fix `role`
     would render empty forever.
  3. **T9 — anchor precedence was backward.** `SourceDossier` built its highlighted spans from
     PERSISTED anchors first, then appended live ones — and `segmentBlock`/`Annotate` take the
     *first* span at a given id, so a stale anchor beat the student's in-progress answer. Fixed by
     making `SourceDossier` the single merge point (live-first, dedupe by id).
  **Accepted gaps (spec §11):** the search-plan card (AI questions the plan — needs its own coach
  design); RL-2's citation half (no citation surface until Slice 8/写作 — the log 6b built is the
  data that enforcement will read); the 偏弱 verdict chip (no honest producer); the S2 perspective map
  (视角与素材, graph-node-backed — belongs with Slice 7); `source_log_entry.lateral_read` (written by
  SIFT, 6c). **New, found during 6b:** stored `event` rows keep `type`/`surface` as DB columns while
  the Zod `StudioEvent` variants are flat objects — a reader must merge columns + payload before Zod-
  validating (pre-existing, systemic across all 8 event types; Slice 10's assessor must handle it).
  **Orphan sweep (Task 10):** `SourceFixture`/`SOURCE_FIXTURES` and `views.material: []` greps both
  empty (already swept in T6); `make sqlc` produced no diff (`git status --short
  internal/store/sqlc` clean). **Final gate:** `go build`/`go vet` clean; full Go suite serialized
  `-p 1` exit 0 (14 packages `ok`, 5 `[no test files]`) + web **306** (up from 282 at 5d, +24) +
  contracts **158** (up from 155, +3) + `tsc --noEmit -p apps/web` clean.
- **NEXT = Slice 6c** (SIFT lateral read): `source_log_entry.lateral_read`, the SIFT card's lateral-
  reading step over the material view 6b just wired live. Then 7–9 (deepen 结构/写作/评估).
  **Cleanup (any time):** project-scope `GetCardInstance` (still takes only `cid`, no project
  filter — the `task_id` NOT NULL half of this carry-forward is now done, this half is not); unique
  index on `chat_thread.seeded_project_id`; onboarding live producer; live `gate` passed/total
  counts.

- **Slice 6c** — ☑ **complete** (branch `refactor2-slice6c-sift`, commits `777f674`..HEAD).
  Spec `docs/superpowers/specs/2026-07-13-slice-6c-sift-lateral-design.md` · plan
  `docs/superpowers/plans/2026-07-13-slice-6c-sift-lateral.md`. **Slice 6 is now closed.**
  Delivered: the **`compare` primitive** (the second C1 primitive, and the first since Slice 1 —
  its renderer *composes* `Annotate`, one instance per pane, rather than forking span logic);
  **SIFT as pure C2 config over it** (`params.lateral_dimension`, `lateral_source_present`
  completion, four steps Stop/Investigate/Find/Trace); the **`cross_check` mint** (a student-
  authored node + two edges, checked --cross-checked-by--> node --cites--> lateral) which
  **never promotes the lateral source to `evidence`** — a source that arrived seconds ago has been
  evaluated by nobody, and promoting it would hollow out `every_source_evaluated`; the source-log
  flip (`lateral_read` on the CHECKED source — the lateral one is the *instrument*, not the
  subject) plus the **before/after re-tier** (`tier_before` read pre-mint, `tier_after` her new
  pyramid choice, and her written 修正后的判断 — the stance change Slice 10's assessor exists to
  find); and **S3's gate promoted from self-attestation to a machine check**.
  **Acceptance proven:** SIFT touched **zero card-renderer code** (`git diff --stat` on
  `apps/web/src/cards/` across the whole branch is empty), and the gate change cost **zero lines of
  `gate.go`** — `node_present` already took a type, so it is one line of skill config.
  **Two pre-existing defects fixed on the way** (neither was in scope; both would have shipped):
  (1) `anchoredMaterialID` resolved a card's material as *the first anchor in the array*, an
  assumption its own comment stated — SIFT is the first card whose anchors span two materials, so
  the mint would have attached the cross-check to a coin flip between the source under review and
  the source used to check it. Now resolved by **declaration** (`params.lateral_dimension`).
  (2) `CompleteCard` minted node → edge → `framework_fill` as three untransacted writes, and
  `framework_fill` **is** the idempotency guard, written last — so a mid-sequence failure left a
  minted node with no guard and the retry minted a **duplicate**. CRAAP had this today. Now one
  transaction (`CommitCardMint`), all-or-nothing.
  Gate: Go build/vet + all 14 packages green (uncached); web 344/344 + `tsc` clean; contracts
  170/170. **Carry-forwards:** the search-plan card (S2) still needs its own design; RL-2's
  citation half needs a citation surface (Slice 8); the perspective map is graph-backed (Slice 7);
  stored `event` rows keep `type`/`surface` as DB columns while the Zod variants are flat, so
  Slice 10's assessor must merge columns + payload before validating.

  **Correction (post-hoc):** the "☑ complete" above was written before a whole-branch review — run
  three separate times against this same branch — found the feature described above was code-
  complete but **not actually reachable**, plus a data-loss defect the description above doesn't
  mention. Fixed across five follow-up commits, FIX-A through FIX-E:
  - **FIX-A** (`c9c1692`) — SIFT was never server-side eligible: the classifier's lateral-check
    candidate never actually fired.
  - **FIX-B** (`df28fd2`) — SIFT was never client-side reachable either, on top of FIX-A.
  - **FIX-C** (`02c784f`) — deleted a dead compare-pairs UI path and lifted the lateral-pick state
    so the center pane could actually show it live (whole-branch review finding [3]).
  - **FIX-D** (`c3c5446`) — the turn loop could surface a NEW card on top of an already-open one;
    the client applied it unconditionally, silently destroying the open card's in-progress answers
    and leaving its row a permanently "active" zombie. Fixed server-side: `SurfaceCardCandidates`
    now suppresses every `surface_card` candidate project-wide while any `card_instance` is
    `proposed`/`active`. Also closed a cross-project anchor hole (`submitProjectCard` now verifies
    every submitted anchor's material belongs to the project) and made `CommitCardMint` fail loudly
    instead of silently leaving `lateral_read` stuck false when the checked material has no
    source-log entry.
  - **FIX-E** (this entry) — FIX-D's suppression turned a pre-existing gap into a permanent-brick
    risk: `StudioProjection` never carried a live card, so a page reload while a card was open lost
    the client's only reference to it while the row stayed open server-side — and FIX-D's
    suppression then blocked every future card from surfacing, forever. Now `StudioProjection`
    projects `activeCard` (status, anchors, materialId) and the client rehydrates it on mount.
    Also hardened the client itself (`conversation.ts` now refuses to let an ordinary chat turn
    overwrite an `active` card with a different one — defense in depth alongside FIX-D's server
    suppression) and gave `relation`/`revised_judgment` — written onto every `cross_check` mint
    since 6c but read by nothing, anywhere, until now — a reader: `SourceDossier` renders them back
    to her, on the checked source, exactly as she wrote them (no invented verdict).
  With all five landed, SIFT is genuinely reachable end to end, a reload can no longer brick the
  workspace, and the `cross_check` mint's own words are no longer write-only. The line above should
  be read as "code-complete, made reachable and safe by FIX-A..FIX-E" rather than "complete."

### Slice 7 — 结构 view + `graph` primitive / Toulmin card (merged `608ed8c`)

The third hand-built interaction primitive (`graph`, a typed-slot argument builder) + the Toulmin
card as pure C2 config over it + a live S4 结构 center pane. Shipped via the full loop (spec → plan →
8 subagent-TDD tasks → per-task reviews → whole-branch review → fix wave).

- **`graph` primitive resolved the Slice-1 "card-driven vs map-viz" deferral in favor of the binding
  design: a card-driven five-slot flow** (claim / warrant / evidence / counter·steelman / concession),
  NOT a spatial map. `map⇄outline` is dropped (not in the binding design — the row text above is the
  original ambition; the design governs). Its *state* is still a node/edge `GraphState`; "graph" names
  the state it produces, not a canvas.
- **Slots ride in the envelope's anchors** (one text anchor + one source anchor per cited material, keyed
  `dimension=slotId`), so completion (`graph_slots_complete`) and the `toulmin` mint reuse the anchor
  spine with ZERO signature change — the additive-switch-case pattern from 6c.
- **The Toulmin card auto-surfaces project-scoped** (`AnchorKind:"project"`, no material): the summon
  dispatch, `SurfaceCard`, and `CreateCardInstance` now tolerate a material-less card (empty `material_id`,
  never a zero-uuid; no `evaluates` edge; NULL task_id). No migration (`card_instances` has no material
  column). This is the FIRST project-targeted card.
- **The summon-hop seam that shipped broken in 6c is now closed AND proven** by a real-Postgres end-to-end
  test crossing summon→submit→mint→gate — the exact test class whose absence let 6c ship unsummonable.
  The whole-branch review (Opus) confirmed the loop works in the real product code, not just tests — the
  first slice in six with no CRITICAL cross-layer finding.
- **S4 gate re-scoped to "card completion"** (product decision, twice): dropped `no_single_sourced_claim`
  (S3 owns source quality) AND `no_orphan_evidence` (CRAAP mints orphan `evidence` nodes that made it
  unsatisfiable in the real flow). S4 machine gate = `{no_unsupported_claim, node_present concession}`,
  both satisfiable by finishing the card. Whole-branch review then caught that S3 STILL referenced
  `no_single_sourced_claim` (the spec mis-stated this) and toulmin's single-evidence claim regressed S3 —
  fixed by dropping it from S3 too (it was vacuous there pre-Slice-7).
- **CARRY-FORWARD (7b): DONE — see the Slice 7b section below** (merged `2c882a3`). Remaining minor
  carry-forwards untouched: no skills canonical/mirror guard test; `gen-go-fixtures.ts` is dead (its TS
  prompt sources were deleted 2026-06-25 — the Go golden now regenerates via `UPDATE_GOLDEN` in-test);
  pre-existing `packages/contracts` tsc failure (not ours).

### Slice 7b — project the completed Toulmin argument into the 结构 view (merged `2c882a3`)

Revived the dead completed-state path from Slice 7: after the student locks the Toulmin card, the five
role cards (with her own sentences) + the green gate banner now render in the 结构 pane. Shipped via the
full loop (spec `7907e79` → plan `de44ac2` → 4 subagent-TDD tasks → per-task reviews → whole-branch
review → fix wave). One new backend derivation, no new interaction, no re-edit of a locked argument.

- **`projectStructure` (studio/projection.go)** reads the `toulmin` card spec's five slots (order + role
  labels — single source of truth, no second hardcoded list) and the minted graph nodes, emitting five
  `StructureCardDTO{id,role,status,preview}` on a new required `StudioProjection.structure` field.
  `done` + the student's `body.text` verbatim where a slot node exists, else `empty`; **empty slice until
  the argument is minted, so the pane keeps its pre-mint placeholder** (product decision — no 5-card
  skeleton up front).
- **The CRAAP-evidence trap is closed by `type == slot.id && body.text != ""`.** CRAAP's `promote` mints
  an `evidence`-typed node with `body.source_quality` (no `text`); only Toulmin mints slot nodes with
  `body.text`. The whole-branch review (Opus) swept every card spec with `graph_effects` and proved the
  discriminator + the `minted` gate cannot let an orphan CRAAP evidence node falsely fill the evidence
  slot or replace the placeholder with a skeleton pre-mint.
- **Second consecutive clean whole-branch review** (no Critical/Important) — all four cross-layer hops
  (mint → project → wire DTO → `toStudioState` → render) traced end-to-end against real code, and the
  e2e projects with the production `cards.ByID` loader, not a hand-built double.
- **Frontend cleanup:** deleted the now-unreachable `active` inline-edit branch of `RoleCard` (that live
  state is the full-pane `StudioToulminCard`); `StructureCardFx` is now the wire `StructureCard` verbatim.
- **Product note (flagged, not fixed):** the in-pane green `GateBanner` ("可以进成稿打磨") goes green when
  all five cards are `done`, but the S4 station gate only reaches `machine_clear` after an external
  Advance — inherited Slice 7 copy; may over-promise if read as the gate verdict.

### Slice 8 (keystone) — writing surface + whole-draft review (S5) (merged `4224231`)

The S5 写作 station goes live end-to-end: a silent client-owned edit buffer → commit an immutable
draft snapshot → student-triggered 整稿体检 (whole-draft review) → three-key disposition → the S5 gate
reconciles. Shipped via the full loop (spec `76c7417` → plan `e845023` → 11 subagent-TDD tasks →
per-task reviews → whole-branch review → fix wave). Scope split (locked in brainstorm): this keystone
ships **one default board examiner voice**; examiner-voice *switching* + the 3 generic voices +
board-specific passes (EE evaluation-density, AP org×connection) + richer budget-deletion prompts are
**Slice 8b**.

- **No migration** — `draft_snapshot`/`edit_buffer` existed since Slice 0. New `writing.sql` queries +
  endpoints: `PUT /projects/{id}/buffer` (student text only), `POST /projects/{id}/snapshots` (commit =
  paste = the same immutable object), `POST /projects/{id}/snapshots/{sid}/review` (SSE), `POST
  /projects/{id}/gate/{contractId}/attest`.
- **The word-budget machine gate is real.** An in-band commit (word count ∈ the skill's `word_budget`,
  0457 = `{1500,2000}`) mints a `word_budget_ok` graph node (`author:"ai"`, a typed marker not prose)
  in the same transaction as the snapshot — satisfying `draft_polish`'s machine gate. CJK-aware
  `CountWords` (each ideograph = 1 word). Out of band → the node is removed, so the gate reflects the
  latest snapshot honestly.
- **`order_review` is a new C3 verb.** `ProposeReview` makes ONE flagship call over the snapshot's
  paragraphs + the skill's `review_criteria` (0457 表D/E/F/H labels — NOT the Slice-9 band engine),
  returns a typed work-order, and runs the enforcement stack: a banned-phrasing match on any
  evidence/missing/fix field rejects the WHOLE review (all-or-nothing). Added banned rule
  `rewritten-sentence-zh` (你应该这样写…) so a review can never author prose. Each item persists as a
  `review_item` intervention anchored to the snapshot (the whole `ReviewItem` marshalled into
  `intervention.body`, reconstructed by one Unmarshal).
- **One snapshot, one review — idempotent.** An existing review replays its persisted rows with no
  second model call (proven by an `llm_call`-count assertion). Cross-tenant safe: `sid` is
  project-scoped via `GetSnapshot`, entitlement gated before the stream only when a model call happens.
  Cost is recorded even on an enforcement rejection.
- **RL-1 holds structurally** (whole-branch verdict): every new write path swept — the buffer takes
  student text only; the review writes only `intervention` rows; even a model-emitted sentence in `fix`
  is *displayed as advice, never written into the draft*. There is no path where model output becomes
  prose the student didn't write.
- **`citations_matched` needed a new write path.** The gate_state storage existed but nothing recorded
  a student_written item solid (the planner deliberately never does). New `attestGate` endpoint,
  restricted to the contract's own student_written names (a forged machine/human item → 400).
- **Three-key disposition** (保持原样→accept / 我来改→rewrite / 说明为什么不改→reject) reuses the
  Slice-5c disposition endpoint; reason ≥15 runes; persisted choice renders selected on reload.
  Ordering a review records the S5 human gate item `whole_draft_review` solid — now guarded on
  `len(persisted) > 0` (fix wave: never mark the gate solid on an all-insert-fail empty persist).
- **Third consecutive clean whole-branch review** (7, 7b, 8) — no Critical/Important; all four
  cross-layer hops (SQL → store → handler → projection DTO → Zod → toStudioState → WritingView) traced
  end-to-end, wire parity guarded by `dto_parity_test`.
- **Carry-forwards:** **8b** (voice switching + 3 generic voices + board-specific passes + richer
  budget prompts); **Slice 9** (the five-skin readiness gauge — the review's criteria labels are the
  minimal seam); **Slice 10** (automated citation front-to-back matching — keystone = the
  `citations_matched` attestation only; snapshot diffs feeding the process record). Minor follow-ups
  (non-blocking, from the whole-branch review): the 只读 preview renders the live buffer, not the
  committed snapshot content (the projection omits snapshot content — a post-commit edit/reload can show
  drift); the spec §5 "output-check + authorship" enforcement is narrowed to banned-phrasing only for
  the multi-field work-order (authorship N/A — the review writes no student field; OutputCheck needs a
  single topic the work-order lacks) — `output_check_verdict` left unset.

### Slice 8b — examiner-voice switching + budget-deletion coaching (S5) (merged `9a7356e`)

The two deferred pieces of the S5 写作 station land: switchable examiner voices on 整稿体检, and
over-budget deletion coaching. Shipped via the full loop (spec `fd3d563` → plan `a44c06d` → 8
subagent-TDD tasks → per-task reviews → whole-branch review → 2-test fast-follow). Scope locked in
brainstorm: **EE/AP board-specific passes stay deferred** to their Phase-2 board packs (only the 0457
skill is seeded — no board to drive an EE evaluation-density or AP org×connection pass).

- **No migration, no skill-config change.** Voice is a fixed Go enum (`board | sceptic | layperson |
  executioner`), not skill config; `board` = the keystone's existing `reviewPosturePrompt` verbatim, the
  three generic postures are board-agnostic. Voice rides inside the existing intervention `anchor` jsonb
  (`{kind,id,voice}`), so the review's idempotency key extends from `snapshot` to `(snapshot, voice)`
  with zero schema change.
- **Same snapshot, many examiners.** `?voice=` on the review endpoint (absent/unknown → board) selects
  the `ProposeReview` posture; each voice is one flagship call cached independently; re-running a voice
  replays its persisted rows with NO second model call (proven by an `llm_call`-count assertion). A
  keystone review row (anchor with no `voice` key) reads back as `board` at **both** read sites — the
  replay filter and the studio projection — so Slice-8 data stays valid with no backfill.
- **RL-1 holds structurally** (whole-branch verdict): voices swap only the system prompt; the
  banned-phrasing enforcement and the intervention-only write path are voice-invariant; the over-budget
  deletion lens asks diagnostic questions (「这段在向哪张表交证据」) and hands the cut to the student
  (「你不替她删」) — never a "删掉…" imperative or a rewritten sentence. No path turns model output into
  prose the student didn't write.
- **Budget coaching, no model for the verdict.** A pure `agent.BudgetVerdict(wc, band) → (state, delta)`
  (state ∈ in/over/under; delta a non-negative magnitude) is projected onto the snapshot as
  `budget:{state,delta}`; the 写作 `snapshotMeta` gains `· 超出 N 字` / `· 还差 N 字` / `· 在预算内`.
  When a snapshot is over `max`, the review runs with an appended deletion-lens instruction and the
  work-order shows the verbatim note `超预算 N 字 · 删减决策按「这段在向哪张表交证据」来做` — reusing the
  review's existing 段落⇄评分表 mapping, one model surface.
- **Voice-keyed read path.** The projection carries **every** run voice's work-order for the current
  snapshot as a flat `review.items[]` where each item is self-describing via its `voice` (the
  parity-friendly realization of "keyed by voice" — no `map` with enum keys). `WritingView` derives the
  current-voice work-order and the cached-voice pill markers by filtering; the `ordered` boolean is
  dropped on both Go and Zod sides. Segmented voice pills (考官/怀疑/外行/字数) reuse the 编辑/预览 styling.
- **Fourth consecutive clean whole-branch review** (7, 7b, 8, 8b) — no Critical/Important; Go↔Zod wire
  parity byte-clean (SQL → store → projection DTO → Zod → toStudioState → WritingView), the voice enum
  uncorruptable because the sole anchor writer serializes only `ParseVoice` output. Two logged Minor
  coverage gaps (under-budget clause, cached-pill marker) closed by fast-follow `1cc0194`.
- **Carry-forwards:** EE/AP board-specific passes → their board-pack slices; **Slice 9** (readiness
  gauge — the review's criteria labels are the minimal seam) is next. Non-blocking (from the
  whole-branch review, not fixed): the selected voice state isn't reset when the snapshot/project
  changes (cosmetic — cached markers + `itemsForVoice` re-derive from the projection, so nothing renders
  wrong); plus the keystone's still-open preview-renders-live-buffer follow-up, untouched by this slice.

### Slice 9 (keystone) — 0457 readiness gauge, made real (评估) (merged `644e1d2`)

The 评估 view's 就绪度 display stops being a static 表A–表H fixture and becomes real, projected from
the whole-draft review's per-table judgment. Shipped via the full loop (spec `6cd61e4` → plan `8c5b8e4`
→ 5 subagent-TDD tasks → per-task reviews → whole-branch review → 3-Minor fix wave `644e1d2`). Scope
locked in brainstorm (all four the recommended path): **gauge only** (self-score / retro / RL-4
reflection editor / AI-usage declaration blocks stay deferred), **0457 only but pluggable** (one
interface, one seeded renderer — no other skins built), **4 review tables** (表D/E/F/H, the tables a
written draft evidences), **extend the review output** (one new `points` int; no 2nd model call).

- **The seam is one integer, `points`.** The skill config's `ReviewCriterion` gains `points` = each
  0457 table's total lamp count (表D 4 · 表E 4 · 表F 3 · 表H 3), and the whole-draft review — already
  judging each table's band — also emits `points` = how many descriptor points the draft evidences. It
  rides the **existing** `review_item` intervention body (`mustJSON`), so **no migration, no schema
  change, no second model call**. A pre-Slice-9 review body with no `points` degrades cleanly to
  lit 0 / empty (proven by `TestProjectReadiness_MissingPointsIsEmpty`).
- **The projection is the single clamp site.** `ProposeReview` copies the model's `points` verbatim;
  `projectReadiness` clamps to `[0, total]`, derives level (`full` iff lit==total>0 / `empty` iff
  lit==0 / else `partial`), and blanks the note when full (a full table has nothing missing — the
  fix that turned the field's `""`-when-full contract into behavior). Note = the review's own `missing`.
- **Board-voice-only lights readiness** (the assessment of record). The points instruction is
  voice-invariant (all four voices emit it — it's assessment data, not the coaching lens), but
  `projectReadiness` reads only the board-voice review items, using the **same** `{kind,id,voice}`
  anchor convention Slice 8b's work-order uses — read identically at both sites, so a Slice-8 keystone
  row (anchor with no `voice` key) reads back as board with no backfill.
- **RL-3 held.** Lamps show which descriptor cell, never a predicted grade; the tooltip copy is
  unchanged (「落在评分表的哪一格……不是预估分数」); the summary 「已点亮 N/M 格」 is derived in the view
  from the lamp sums, not carried on the wire.
- **Design fidelity fix folded in.** `GaugeFx` graduated from a client-invented type to the
  contract-backed `Gauge` (Go `GaugeDTO` ↔ Zod ↔ TS, guarded by `dto_parity_test`), and `ReviewView`
  gained the design's summary line + separate `code`/`name` per table (the old `.tsx` crammed them into
  one field and omitted the summary).
- **Fifth consecutive clean whole-branch review** (7, 7b, 8, 8b, 9) — no Critical/Important; the
  `points` seam traced byte-consistent at every hop (config → ProposeReview → intervention body →
  projectReadiness → GaugeDTO → Zod → toStudioState → ReviewView). Three Minors (loose prompt assert,
  missing-points coverage, a `?? []` map guard) all closed by fix wave `644e1d2`.
- **Carry-forwards:** the other four skins (9239 grid, AP switches, AP band-portraits, TOK needle) —
  each later config + one renderer behind the interface this slice established; the deferred 评估 blocks
  (self-score / retro / declaration); 表A/B/C/G cross-station readiness. **Slice 10** (assessment engine
  / growth report) is next.

### Slice 10 (keystone) — the isolated assessor + growth report (成长报告) (merged `50bda49`)

The fourth subagent — the **assessor** — is wired live for the first time: the 成长报告 slot stops
being a static placeholder and renders a real assessment projected from the project's event stream.
Shipped via the full loop (spec `ae536d5` → plan `4b16ec0` → 7 subagent-TDD tasks → per-task reviews →
whole-branch review → 2-Minor fix wave `50bda49`). Scope locked in brainstorm (all three the
recommended path): **rubric + narrative only** (depth-vs-independence radar + T1–T7 leaps deferred),
**event stream projected** (reads the process record, not just the current-state snapshot),
**minimal anchor few-shot now** (1–2 compact samples; full golden set + benchmark = deferred).

- **The rubric goes single-source.** The seeded CT rubric (D1–D10 + L1–L4 SOLO ladders) was TS-only;
  the Go assessor can't score without it. Per the AGENTS.md single-source law, `FULL_RUBRIC` was
  extracted to canonical `packages/contracts/src/ct-rubric.json` (TS imports + `assertRubricComplete`s
  it — so the hand-maintained array can no longer drift), and Go `go:embed`s a synced mirror
  (`internal/rubric/`, `make sync-rubric`). `rubric.test.ts` stayed green unchanged (byte-faithful),
  and the whole-branch `diff` of canonical↔mirror is byte-identical.
- **The seam that lights the gauge here is the engine, not a column.** `agent.Assess` is one **isolated
  flagship call** (never downgraded, never in the coach loop) that reads a pure `BuildAssessmentInput`
  digest of the event-stream projection (card uses w/ 自发/提示后, dispositions, gate progress, snapshot
  word counts, review bands, graph shape, event timeline) and returns per-dimension `{level, evidence}`
  + a growth narrative. Every rubric dimension is emitted in declared order; a missing dim or an unknown
  level coerces to `NA` (never a crash, never an invented level).
- **This lands the event stream's first reader.** Slice 0's append-only `event` table was written but
  never consumed; `studio.ProjectData` now loads `Events`, and the assessor is its first consumer.
- **RL-5 held structurally** (the named invariant): the assessment is **diagnostic evidence framed for
  growth, never a grade/rank/overall score**. No aggregate can exist because no field holds one — the
  engine's `Assessment`, the `AssessmentDTO`, and the Zod `Assessment` all carry only per-dimension
  level+evidence + narrative; `GrowthReport` renders diagnostic chips (SOLO labels / muted NA) with the
  hero copy 「不是分数，是证据」; the posture prompt forbids grade/rank. The teacher makes the final call.
- **Isolation + cost, both correct.** The POST path writes exactly one `evaluations` row (reusing the
  dormant table via `0021` — additive: `task_id` nullable + scope CHECK, `project_id` already existed
  from 0016) + one `llm_call`. Because the `llm_usage` view's evaluations arm joins through `tasks`
  (dropping project-scoped rows), the `llm_call` (`Purpose:"assessment"`) is the sole path assessment
  cost reaches org aggregation — verified wired, recorded even on enforcement rejection. Enforcement is
  all-or-nothing (banned-phrasing over narrative + every evidence field); RL-1 structurally holds (writes
  `evaluations`, never the student's draft). GET is a pure read (no model call, proven by an
  `llm_call`-count assertion).
- **Sixth consecutive clean whole-branch review** (7, 7b, 8, 8b, 9, 10) — no Critical/Important; all 8
  cross-layer invariants traced (rubric byte-consistency, RL-5 no-aggregate, isolation, single-counted
  cost, all-or-nothing enforcement, level integrity, migration back-compat, Go↔Zod parity). 4 Minors
  triaged: 2 closed by fix wave `50bda49` (D1-presence test assert, `TouchProject` on generate) + an
  order-dependence comment; 2 genuine carry-forwards.
- **Carry-forwards:** the depth-vs-independence radar + T1–T7 thinking leaps (derived views composing on
  this rubric data + the 自发/提示后 signal); the teacher & parent report versions (→ teacher-dashboard
  Slice 13); OPCVL (HS-D*) rubric; the benchmark + fine-tune stages; chat/course surface aggregation;
  async assessment (this keystone is inline). Non-blocking (from the whole-branch review): card
  `Dimension` in the digest is the card's method tag not a CT D-code (no `ct_dimension` on `cards.Spec`
  today); `WordCounts` only 0–1 entries (ProjectData carries only `LatestSnapshot`, no history query).
  **Slice 12** (Course policy + surface alignment, depends on 10) is next.

### Slice 11 (keystone) — Chat surface + coach-alone + card-as-offer (聊天) (merged `d1b0d4c`)

The standalone **Chat (聊天)** surface goes live: a free conversation where the coach runs **alone**
(planner off, no project graph), replies in a guiding-not-answering posture, and — when the student
pastes a link — offers a source-evaluation (CRAAP) card **in-thread** at I3, reproducing the agent-spec
§5.6 trace end-to-end. Shipped via the full loop (spec `ca72deb` → plan `6fa09a7` → 7 subagent-TDD tasks
→ per-task reviews → whole-branch review → 2-Minor fix wave `d1b0d4c`). Scope locked in brainstorm:
**coach + card-as-offer keystone** (the "same runtime" differentiator) over an **additive `thread_id`
scope**; multimodal / project-seeding / off-record control deferred.

- **`reply` becomes a typed output (the C3 seam of this slice).** `reply` was already in the `Verb`
  enum but absent from the `AgentOutput` shape union. Added `{type:"reply", body}` (anchor-free) to the
  Zod union + the Go `enforcement.ValidateOutput` mirror. A chat reply is typed and banned-phrasing-
  enforced, but runs **no** OutputCheck echo pass (chat has no draft to echo); the five existing output
  types are unchanged.
- **The thread graph is an additive scope, not a new world.** Migration `0022` adds nullable `thread_id`
  to `material` + `card_instances` with a `num_nonnulls(task_id, project_id, thread_id) >= 1` CHECK —
  mirroring `0021`'s pattern. Chat reuses the SAME card runtime + enforcement, just thread-scoped;
  thread and project stay clean siblings (so the deferred project-seeding remains a real copy operation).
  `intervention` is untouched (the coach reply is a chat_message, not an anchored intervention).
- **`RunChatStep` is the Chat-policy turn, isolated from the project loop.** It ties a pure URL→CRAAP
  classifier (reusing the `craap` card + the `SurfaceCardCandidates` shape, one offer per thread,
  suppressed once proposed/active/completed/skipped) to `ProposeChatReply`, persisting through a
  project-free `ChatStore` seam. Metering is single-counted through `llm_call` (`surface="chat"`,
  `project_id` NULL, `user_id` set) and recorded **even on enforcement reject**; a rejected reply stays
  silent (no message, no error). The dormant `AppendEvent`/`RecordLLMCall` sqlc queries already took
  `user_id` + nullable `project_id`, so no new event/llm queries were needed.
- **The in-thread card is confirm-to-open and reuses the live runtime.** The offer renders as the binding
  dc.html card preview; an explicit 接受 mounts `StudioCardSheet` (schema-driven, `{spec, onSubmit,
  onSkip}` — cleanly decoupled) in place, and submit flows through a **thin** thread-scoped completion
  path (persist field_values + flip to `completed`) — NO project graph_effects, NO refeed, NO competence
  write (competence is dormant platform-wide; deferred). CRAAP is self-contained (step-1 `link_check`
  captures the URL), so a pasted-but-unfetched link needs no article body.
- **The on-record disclosure is binding and verbatim** (product §2.2): the subtitle 「自由对话 · AI 只提问，
  不替你下结论」, the green 「计入成长评估」 pill + tooltip, the composer placeholder, and the footer 「AI 会陪你
  把想法想深，但不替你得出结论 · 你的对话只属于你」 all render exactly as the dc.html CHAT block (527–666)
  specifies; the rail order 课程,聊天,工作室,成长报告,设置 matches the binding design.
- **Seventh consecutive clean whole-branch review** (7, 7b, 8, 8b, 9, 10, 11) — no Critical/Important; all
  8 cross-layer invariants traced (reply seam, metering-on-reject single-count, thread/project isolation,
  ownership/IDOR on all 6 routes, RL-1/RL-4, binding disclosure + rail order, SSE coherence + confirm-to-
  open, keystone honesty). 3 new Minors — 2 closed by fix wave `d1b0d4c` (skip empty text frame on silent
  reject; corrected a "flagship"→chaperone doc comment) + 1 carry-forward (`ChatCardOfferDTO` parity is
  decorative — the offer ships via snake_case `sse.Card`, not that DTO).
- **Carry-forwards:** multimodal input (composer icons render but inert); chat→project **intake seeding**
  (`chat_thread.seeded_project_id` + fragment copy); off-record thread control (open product Q §606);
  semantic non-link card-moments (opinion→steelman, comparison→matrix — need a model classifier);
  thread evidence nodes / graph_effects + chat→assessment aggregation; competence wiring (shared, dormant
  everywhere). **Slice 12** (Course policy + surface alignment, depends on 10) is next.

- **Slice 12 (keystone)** — ☑ **complete** (branch `refactor2-slice12-course-policy`, commits
  `2d1912d`..`ed7e019`, merged `ed7e019`). Spec `docs/superpowers/specs/2026-07-17-slice-12-course-policy-design.md`
  · plan `docs/superpowers/plans/2026-07-17-slice-12-course-policy.md`. **课程 was the last pre-runtime
  surface** (a dumb linear step player: LLM-rendered pages, ordinal progress, no runtime/cards/dialogue/
  events). It now runs on the one runtime under **Course policy** (agent-spec §5.3).
  Delivered: **DEC-12.1 phases wrap steps** — `course`/`course_step`/`course_progress` + the teaching/
  challenge renderer survive untouched as authored **content**; the course skill (`info-literacy-course.json`,
  `kind:"course"`) adds the **runtime** layer above them (4 phases 演示→引导→独立→回看 in a binding order —
  a strictly linear `requires` chain, validated at load by `LinearOrder`; a branching course skill is a
  config error). **Pages are content, phases are runtime.** · **DEC-12.2 floor + judgment**: a closed set
  of course floor kinds (`steps_viewed` · `card_dispositioned` · `student_turns_at_least`) named in
  `skills`, evaluated in `agent` (the Slice-4 split); the floor is **enforcement below the policy** — an
  unmet floor short-circuits BEFORE the model (zero LLM calls) and the coach physically cannot pass it;
  the soft authored condition is the coach's judgment (DEC-3 discipline: machine only ever refuses).
  `card_dispositioned` is satisfied by **completed OR skipped** — a card is an offer, never a wall ·
  **DEC-12.3 `advance` typed output** (Zod + Go `ValidateOutput`), the C3 seam this slice closes — it was
  in the Verb enum but never had an output type, exactly as `reply` was Slice 11's seam. Forward-only,
  one phase at a time, enforced by the runtime not the prompt · **DEC-12.4 session scope** (migration
  `0023`, additive): `course_session` + `course_message` + a 4th owner column on material/card_instances
  (`num_nonnulls(task_id, project_id, thread_id, session_id) >= 1`) — Course/Chat/Project stay clean
  siblings, each with its own store seam · **DEC-12.5** the next-arrow pages freely inside a phase (no
  network) and *asks* at a boundary; a refusal is a sentence in an auto-expanded panel — never a modal,
  lock, or streak (铁律 2) · **DEC-12.6** renamed Chat's `ThreadMaterial`/`ThreadCard`/`ChatCardOffer` →
  `ScopedMaterial`/`ScopedCard`/`CardOffer` (surface-neutral; Course needs the same shapes) · the binding
  **问印记 ask panel** (dc.html 349–410, copy verbatim, voice button inert) · six endpoints; course events
  → the stream at **full** weight (`surface="course"`), `llm_call` surface=course/purpose=coach/project
  NULL, metered even on reject. Gate: full Go suite all packages ok; web 90 files/471; contracts 203;
  tsc 0; zero sqlc/skills drift.
  **The whole-branch review again earned its keep — and this time so did the per-task ones.** T8 was
  **Not approved** on 3 Criticals from ONE misread frame: the server ends *every* turn with `done`, but
  the player treated it as "course complete", so asking a single question ejected the student to the
  report screen; the same line made the auto-expand-on-refusal path dead code; and nothing anywhere wrote
  `status='finished'` (**a hole in the spec** — it never said how a course finishes), so the only thing
  "finishing" a course was the bug. The whole-branch review then found 3 MORE Criticals the per-task
  reviews structurally could not see: **the course was unusable from its very first phase boundary**
  (`go()` recorded the ordinal being *left*, but leaving a phase's last ordinal routes through
  `courseAdvance` — so `demonstrate`'s `steps_viewed [0,1]` floor could never be met, and every advance
  was refused forever); **guided dead-ended after a page reload** (the offer lived only in React state
  while `CourseCardCandidate` won't re-mint a `proposed` card → the floor became unsatisfiable — an offer
  turned into a wall); and **the floor was client-asserted** (`PUT /progress` let the client write
  `completed_ordinals` wholesale, falsifying DEC-12.2's central claim). Fixes: the terminal is minted
  server-side at `NextPhase !ok` with the floor met (`reflect`'s floor still guards 回看, so it can't be
  skipped); view-recording moved **server-side** into the per-ordinal render handler (fixing the first
  and third together — and it records before the cache-hit branch, or the bug returns); open card offers
  rehydrate via `CourseSessionDTO.openCards`.
  **Root cause of five of the six Criticals: test-mock infidelity** — mocks encoded stream/state shapes
  the backend cannot produce, so the tests confirmed a belief instead of testing the code. Every mock now
  routes through one helper that appends the real `done` frame; the API test drives the real render path
  instead of seeding `completed_ordinals`; a fake-testing-the-fake test was deleted.
  **Carry-forwards:** assessor aggregation of course evidence into 成长报告 (the `course_message` event with
  `{"unprompted":true}` now lands in the stream — nothing reads it yet); terminal-assessment challenge +
  its machine-never-`solid` adjudication; golden/banned example packs per phase; voice (按住说话 renders,
  inert); competence wiring (dormant platform-wide, zero update sites); course→project seeding;
  multi-course authoring (one skill, one seeded course); session restart; non-transactional positional
  material↔card pairing in `mintPhaseCard` (unreachable at one card/session); the terminal is not
  idempotent at the API layer; an unawaited card submit/skip can leave an in-page (reload-recoverable)
  wall; no test runs migrations **Down** anywhere (pre-existing, repo-wide).

### A1 (sub-project) — Course session report: session-scoped assessment (merged `8509940`)

**Not a roadmap slice.** A1 is the first of three sub-projects closing the largest carry-forward left
after Slice 12: chat + course evidence lands in the event stream and **nothing reads it**. Sequence:
**A1 course → A2 chat → A3 project terminal + 成长报告 history entrance**, then **B** (the DualAxis
assessment model replaces the CT rubric) and **C** (the student-level ability model — the binding
design's own 能力素养 radar: 「九个维度…等级来自每次任务评估的归并，不是测验分数」).

**The blocker was structural, not a missing query.** `event` had exactly one scope column
(`project_id`); every course/chat write hard-coded it NULL; the only SELECT ever authored was
`WHERE project_id = $1`, which can never match a NULL. Course evidence was written, stored, and
unreachable **by construction**.

**Shipped:** migration 0024 (`session_id` on `event` + `evaluations`) · `InsertSessionEvent` REPLACES
the scopeless `InsertUserEvent` on `CourseStore` (its signature *was* the bug) · `ListEventsBySession`
· `buildAssessmentInputFromSession` (pure) · GET/POST `/api/v1/courses/{id}/session/assessment` ·
`collectedCards` on the session DTO · `CourseReport` gains 能力评估 + 收集到的工具 from real data.
Auto-generation is GET→null→POST-once on first report open, never inside the SSE turn (a flagship
call there would stall the student's last 回看 turn).

**Decisions.** `event`'s CHECK is **NOT VALID**: pre-A1 chat/course rows are permanently
unattributable — there is nothing to backfill FROM, and neither deleting real records nor inventing a
scope is honest. **The rubric and assessor are untouched** — B replaces them; the dimension count is
knowingly three-way inconsistent today (binding design **9**, `ct-rubric.json` **10**, DualAxis
reference **6**) and A1 bakes in none of it.

**Honesty fixes (both pre-existing, both shipped-to-students):** `挑战通过` counted the challenges the
course *contains* — every student was told they passed every challenge, including ones never opened,
each with a green ✓. There is no pass record in the schema. **通过 is now defined as engagement**
(`completed_ordinals`, server-recorded, not client-assertable); a coach-judged verdict was **rejected**
because RL-5 reads «never a grade **or verdict**», and every judgment in this product lands on the work
(`solid`, `已扎实`), never on the student. `工具收集` rendered the authored catalogue count (seeded `4`)
while the 收集到的工具 block below showed nothing collected — the page contradicted itself.

**Fixed en route, pre-existing since Slice 10:** both assessors called `ChatResolver` (**chaperone**)
while their doc-comments claimed "ONE flagship call (never downgraded)" — violating
「评估走旗舰模型绝不降级」 and recording every assessment as chaperone spend. `EvalResolver` existed, was
wired, and was called by nothing but course *rendering*. Now correct in both, with a tier guard test.

**What the reviews caught that per-task review structurally could not** — the whole-branch review
returned **DO NOT MERGE** on two Importants:
1. **0024's Down aborted the moment any course report existed** (`ADD CONSTRAINT` validates existing
   rows; a session-scoped evaluation has neither task_id nor project_id). **The repo's first migration
   Down test passed only because it seeded nothing** — the Slice-12 mock-infidelity pattern in
   migration form: it confirmed the belief "Down reverses" while never testing the case that breaks it.
   The only environment you'd ever roll back is one where the feature ran.
2. The `工具收集` fabrication above.

**Also caught mid-flight:** enforcing `event_scope_ck` a slice before chat has a scope column would
have **silently killed all chat evidence** (chat's 3 event writes swallow errors into `slog.Warn`, so
no test fails) → an explicit, schema-level `surface = 'chat'` exemption arm; **A2 MUST delete it**.
And the spec's own rejection-test fixture matched **no real banned-phrasing rule** — the test would
have passed for the wrong reason.

**LESSON (new, repo-wide):** "verify call sites by compiling" is **false in Go** — keyed struct
literals permit omitted fields, so a missed `AppendEventParams{…}` site compiles and silently writes an
unscoped row. Use grep. This found 2 sites a brief's list had missed.

**Carry-forwards:** chat keeps its unscoped `InsertUserEvent` + the exemption arm until A2 ·
`我的学习笔记` / `导出笔记` designed but no feature exists anywhere (`导出` = zero hits) · pre-A1 event
rows stay permanently unattributable (the CHECK stays NOT VALID) · **no entitlement path anywhere is
tested** — `HasEntitlement` is a package-level `return true, nil` with no injection seam; billing work
must add one · challenges still have no notion of quality, only engagement · the report re-POSTs a
flagship call on every mount after a 422, and StrictMode double-fires it in dev ·
`EVENT_TYPES`/`StudioEvent` remain dead code matching neither the DB row nor any type string written.

**DONE `a03682e` (2026-07-18, ahead of the 2026/07/24 15:59 UTC deadline):** migrated both tiers off
the deprecated `deepseek-chat` + `deepseek-reasoner` → `deepseek-v4-pro`. `gateway/keyresolver.go` both
Model literals (tiers unchanged); `gateway/pricing.go` old two rows collapsed to one
`deepseek/deepseek-v4-pro {0.435, 0.87}` (cache-miss) **in the same commit** so cost tracking stays real;
gateway tests + `apps/web/e2e/RUNBOOK.md` updated. Verified `deepseek.go` doesn't branch on the literal
model name, so the rename carries no hidden reasoning-mode coupling; flagship genuinely upgrades
(`deepseek-reasoner` had mapped to v4-**flash** thinking-mode). Full Go suite green; no codegen touched.

### A2 (sub-project) — Chat session report: thread-scoped assessment (merged `44167a9`, 2026-07-18)

**Not a roadmap slice.** Second of the A-series (A1 course → **A2 chat** → A3 project terminal), closing
the exemption A1 parked and making chat evidence reachable per-thread. Spec
`docs/superpowers/specs/2026-07-18-chat-session-report-design.md`, plan
`docs/superpowers/plans/2026-07-18-a2-chat-session-report.md`. Executed subagent-driven, 5 tasks.

**Shipped:** migration 0025 (`thread_id` on `event` + `evaluations`) — **re-adds `event_scope_ck`
WITHOUT the `surface='chat'` arm A1 parked**, still `NOT VALID` (grandfather pre-A2 rows, enforce new
ones): from 0025 a chat event without `thread_id` is rejected · `AppendEvent` gains `thread_id`,
`ListEventsByThread` added · `InsertThreadEvent` **REPLACES** the scopeless `InsertUserEvent` on
`ChatStore` (resolves user from `chat_thread`, hard-codes `surface='chat'`) · all three chat writes
(`prompt_sent`, `card_completed`, `card_surfaced`) now scoped · `RecordChatLLMCall` gains a `purpose`
param (coach vs assessment cost attribution) · A1's builder generalized `…FromSession` →
`buildAssessmentInputFromEvidence` (course + chat share ONE builder — no duplicate) ·
`InsertThreadEvaluation`/`GetLatestThreadEvaluation` · GET/POST `/api/v1/chat/threads/{id}/assessment`
(flagship `EvalResolver`, cost recorded even on 422, thread-scoped persist) · web `ChatReport` panel +
opt-in button modeled on `CourseReport`.

**Decisions.** Chat is **student-opt-in** (铁律 2 — chat never ends, so the student chooses when a thread
is worth a print): `ChatReport` GETs on mount and only POSTs on an explicit click, never auto-generates.
Rubric/assessor **untouched** (B's job), same as A1. `收集到的工具` tile **trimmed** from the chat report
(would need a thread-cards endpoint chat doesn't expose; redundant with the assessment's own
card-disposition evidence) — deferred with the 成长报告 aggregate (C). Skips stay status-only (no new
`card_skipped` event — visible via `card_instances.status`, YAGNI).

**The one cross-task defect the per-task reviews structurally could not see** — adding 0025 as the new
head migration broke **both** of A1's 0024 tests: `TestMigration0024SessionScope`'s step 2b asserted an
unscoped chat event is *accepted* (the exemption 0025 closes), and `TestMigration0024Down`'s single
`goose.DownContext` now reversed 0025 instead of 0024. Fixed: removed the superseded assertion (0025's
own test asserts the rejection) and switched the Down test to `DownToContext(…, 23)` so 0024's own Down
block still runs with its load-bearing row. **The whole-branch review streak (Slices 7–11 + A1 clean,
Slice 12 broke it) — A2 broke it too, and the surfacing was a full-package test run, not the review.**

**Process lesson (new):** one implementer subagent's report was **fabricated** — it claimed a commit SHA
that never existed (HEAD unmoved), described thread-scope work as "session"-scope, and claimed `./...`
all-green while two 0024 tests were failing. The controller caught it by verifying HEAD moved and
re-running the full packages. **Distrust an implementer's "all green"; verify the commit landed and
re-run full packages when a report smells off. A focused `-run` subset in a brief is what let the 0024
regression slip from Task 1 to Task 2.** Whole-branch review (opus) then came back clean, explicitly
checking mocks against real contracts (resolver tiers distinct, DTO shape matches Zod, migration test
inverts rather than restates) — no green-suite-over-infidelic-mocks.

**Carry-forwards:** `proposed`-status chat cards flow into the assessor as a `DispositionUse{Kind:
"proposed"}` (course-parity via the shared builder; honest "offered, not acted on" — revisit with B's
chat rubric) · the `CostNumeric(cost, true)` latent bug persists in BOTH `RecordChatLLMCall` and
`RecordCourseLLMCall` (hard-codes valid even when unpriced; dormant post-V4; a dedicated cross-recorder
fix) · chat report UI is a **new surface not in the binding design** (follows `CourseReport` precedent) ·
成长报告 aggregate (`对话数/被追问后返工/触发思考工具`) + per-thread history entrance are **C**'s work; A2
produces the rows C will aggregate · the untested `HasEntitlement` seam (A1 carry-forward) still unfixed.

**Next:** A3 (project terminal — finish button after 整稿体检 gated on `whole_draft_review == "solid"` +
成长报告 history entrance), then B (DualAxis replaces CT rubric, settles the 9-vs-10-vs-6 dimension count).
