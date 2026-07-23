# Student Platform · Remaining Work & Finishing Decomposition

> Consolidated inventory of everything still open on the **student-facing platform**
> (Slices 0–12 of the whole-product refactor), plus a decomposition of that
> remainder into six shippable finishing slices (N1–N6).
>
> Source of truth for status: `docs/2026-07-11-whole-product-refactor-roadmap.md`
> (line refs below point into it). Supersedes the first-refactor tracker
> `docs/遗留项追踪_Carryforward.md` for anything post-2026-07-11.
>
> **Already DONE (excluded here):** the whole cross-surface assessment track —
> A1 `8509940`, A2 `44167a9`, A3 `f81c02a`, B (DualAxis), C (ability model),
> and the 工具卡 tab `4c59323`; the DeepSeek-V4 migration. Slice 13 (teacher
> dashboard) is out of scope, gated on teacher-side design.

Date: 2026-07-20

---

## Finishing decomposition (N1–N6)

"Finish the remaining student-platform work" is ~5–6 shippable slices, not one.
Each ships working, testable software on its own (the roadmap's slice rule).

| Slice | Closes | Size | Rationale |
|---|---|---|---|
| **N1 · Close the loop** | Project-creation entry point + S0 任务解码 intake + directory | S–M | Makes the product **reachable end-to-end**. **✅ DONE — merged `f3c4b31` (2026-07-20).** `POST /projects` (atomic project + 3 onboarding graph nodes from a board-static 0457 fixture, no LLM, no migration) + `POST /projects/{id}/onboarding` (persist restate + weak-picks as node+event) + projection surfaces them + directory-first `StudioContainer` with 新建论文 paste-prompt flow + OnboardingView submit/hydrate. Spec `docs/superpowers/specs/2026-07-20-n1-close-the-loop-design.md`, plan `…/plans/2026-07-20-n1-close-the-loop.md`. **Correctness seams were NOT bundled (deferred to N6, see below).** N1 carry-forwards → N6: back-less error screen on failed project-open (add ← to directory); `handleBack` drops `conv` without `dispose()` (SSE leak); directory shows raw station code + `0457` (no status badge / station-name label); create-atomicity has no rollback test (no fault-injection seam). |
| **N2 · 评估 view (S0/S6)** | Reflection pack, prediction loop S0↔S6, self-score, AI-usage declaration, export forks (RL-4) | L | The biggest unbuilt student surface. **◐ PARTIAL — N2a+N2b+N2c DONE, merged `3273c01` (2026-07-20)**: self-score card + S0↔S6 prediction reveal + retro editor, all on one unified `review_criteria` taxonomy (rewrote N1's fixture rows); 2 student-write endpoints (self_score/reflection nodes, no LLM/migration); retro advances the reflect_archive S6 gate; seed 0018 migrated onto the taxonomy so the demo shows it. Spec `docs/superpowers/specs/2026-07-20-n2-review-view-completion-design.md`. **N2d · AI-usage declaration** shipped by N3f (the AI 使用申报单). **N2e · export forks PARKED with teacher-side design** (2026-07-23 decision): the process-document export is the school-facing wedge and forks by board — it belongs with the teacher/report surface (Slice 13), not the student platform. Do NOT re-surface N2e as student-platform "next." |
| **N3 · Interaction breadth** | `sort`/`matrix`/`scale` primitives + student span-creation L2/L3 + semantic non-link card-moments (needs the classifier hook) | L | All "richer thinking moments"; share primitive + classifier infra. **◐ PARTIAL — N3a DONE, merged `27492fd` (2026-07-21).** The C1 primitive library is now **6/6**: `sort`/`scale`/`matrix` built end-to-end (Zod state + Go params/predicates/effect + `primitives/*` modules + `Studio*Card` hosts + an exhaustive `CoachRail` fork), each bound to a card — `fact-opinion-value`→sort, `certainty-spectrum`→scale, NEW `perspective-matrix`→matrix. **NO migration, NO LLM call, NO `Anchor` change** — all three ride the existing anchor shape (`quote`=item/row, `dimension`=bucket/stop/column, `answer`=reason/cell). Spec `docs/superpowers/specs/2026-07-21-n3a-primitive-library-design.md`, plan `…/plans/2026-07-21-n3a-primitive-library.md`; 10 tasks subagent-driven. **Bonus fix the review surfaced: `perspective-matrix` is the only producer of `perspective` graph nodes anywhere, and `writing-project.json`'s `evaluate_perspectives` gate (`node_count_at_least{perspective,2}`, a `requires` of `evaluate_sources`) had NO producer — that station chain was silently unsatisfiable and is now walkable.** **N3b DONE too, merged `eb0c419` (2026-07-21)** — the semantic classifier + the live 摘要回灌 refeed; see the N3b section below. **N3c DONE too, merged (2026-07-22)** — the guidance fade L1→L2→L3 + student text-selection span creation; see the N3c section below. **N3d DONE too** (2026-07-22) — S0→S3 made genuinely walkable for a real student: `agent.AdvanceAll` (DAG-ordered gate advancement) got its first production call sites, S0's two missing producers + S1/S2's six missing producers were all built, and the S1/S2 placeholder (`ShellView`) was replaced with the binding-design views; see the N3d section below. **N3f DONE too** (2026-07-22) — the remaining six S3/S4/S6 gate items N3d's own review found had no producer (`source_risk_notes`/`warrants`/`steelman` attestation, `source_quality_spot_check`/`warrant_quality_spot_check` via new 信源体检/论证体检 checks, `declaration_signed` via the AI 使用申报单) all got one, plus the L1 block-id collision fix; the S0→S6 walk now passes end to end on a freshly created project — see the N3f section below. **N3 is now ✅ DONE** — N3e (search-plan card + R-9 framework reveal) merged 2026-07-23; see the N3e · DONE section below. |
| **N4 · Multi-board breadth** | OPCVL rubric + ladders, EE/AP board packs, other gauge skins, T1–T7 leaps | L | Config/rubric breadth behind existing interfaces. |
| **N5 · Chat/Course completion** | Voice merge, multimodal, chat→project + course→project seeding, terminal-assessment challenge, multi-course authoring, competence wiring | L | Finishes the two keystone-only surfaces. |
| **N6 · Infra hardening** | Flagship planner judgment, async assessment (river worker), `HasEntitlement`/billing seam, `CostNumeric` bug, migration-Down tests, misc tech-debt | M | Pure platform/infra; low product-visibility; can run anytime. |

---

## Full remaining inventory (by slice of origin)

Each item tagged with its target finishing slice `[N#]`. Line numbers are into the
roadmap file.

### N3d · DONE — merged (2026-07-22)
Spec `docs/superpowers/specs/2026-07-22-n3d-walkable-stations-design.md`,
plan `…/plans/2026-07-22-n3d-walkable-stations.md`; 13 tasks (12 build + this
acceptance-test-and-tracker task).

**The finding that started this slice.** Reading the code before touching it
turned up something bigger than the three items the tracker had listed for
N3d (R-9 reveal, search-plan card, S2 perspective view). `agent.Advance` — the
only function that sets a gate's `Confirmed` flag, which
`studio.projectStations` reads as `GateReport.Solid` to decide a rail entry's
`done`/`current`/`locked` — had **zero production call sites**; it was
exercised by tests only. So for any project a real student created, no
station could ever turn `done` and the rail's head stayed pinned at S0
forever. Nobody had noticed because
`internal/store/migrations/0018_seed_demo_project.sql` hand-writes
`confirmed_solid:true` `gate_state` rows for the seeded demo project's first
four contracts — the demo project starts at S4 because a migration says so,
not because anything computed it. Underneath that, S0/S1/S2 between them had
six gate items with no producer at all (`weakness_prediction`,
`milestone_plan`, `research_question`, `provisional_answer`,
`preregistration`, `terms_defined`). Worth remembering next time something in
this codebase "looks done" only in the seeded project: check whether a
migration is doing the work a live code path should be doing.

**What's live now:**
- **`agent.AdvanceAll`** (`internal/agent/planner.go`) walks the skill's
  contract DAG in topological order and confirms a contract only when its own
  gate is clear AND every contract it `requires` is already `Solid` —
  stricter than `Route`'s reachability rule, so a station can never flip
  `done` while its predecessor is still open. It reuses `Advance` for the
  actual confirm + `gate_attempt` event (DEC-3 still holds — nothing here
  records a `student_written`/`human` item), then calls `Replan(...,
  "advanced")` so the route/head recompute in the same request.
- **A thin `advanceGates` helper** now runs best-effort at the end of every
  write that can change gate state (`submitOnboarding`, `submitFraming`,
  `submitPerspectives`, `logSourceOpen`, `attestGate`, `submitReflection`,
  `submitProjectCard`) — deliberately NOT called from read paths or from
  `ingestMaterial` (adding a source never satisfies a gate; it can only make
  `every_source_evaluated` harder).
- **S0's two missing producers**: `weakness_prediction` ×2 (one per weak
  pick, delete-then-insert scoped to `origin:"station_view"` so re-submitting
  never inflates the `n≥2` gate) and `milestone_plan` (attested on her own
  submit — the same shape `submitReflection` already used for its own item).
- **S1 立题** is a real view now (`FramingView.tsx` + `POST
  .../projects/{id}/framing`, replacing the placeholder). `research_question`
  is minted once, at project creation, from the title she typed in N1's
  funnel. She names and defines the terms in her *own* question — the binding
  design's hard-coded three terms only ever worked for the demo's own title;
  a generic list would have needed either an LLM call or a board fixture that
  can't know her question, so the panel gained an 「添加一个关键词」 control instead
  (a deliberate, documented departure, spec §5.2). `terms_defined` attests
  once ≥3 definitions clear 15 runes and un-attests honestly the moment one is
  emptied.
- **S2 视角与素材** is a real view now (`PerspectivesView.tsx` + `POST
  .../projects/{id}/perspectives`, replacing the placeholder). Her own
  perspective rows (`national` / `global_for` / `global_against`) sit
  alongside — and the save can never delete — perspectives minted by N3a's
  `perspective-matrix` tool card; those render read-only, tagged 「来自工具卡」,
  and still count toward the `node_count_at_least{perspective,2}` gate.
  `recon_logged` rides the existing source-log ledger (opening a source *is*
  the recon — no new control); `sources_per_perspective` is the one explicit
  attestation, because no structural signal for "a source per perspective"
  exists and inventing one would be fabrication. The view also grew an
  `AddSourceForm` + a compact material-title list under the perspective list
  — the binding design drew S2 with no way to add a source at all, even
  though the station is named 视角与素材 and its own gate demands sources per
  perspective, which made the gate unreachable by construction (spec §6.3).
- **`ShellView` is gone.** The placeholder 「此环节的深入交互将在后续切片接入」 that both
  S1 and S2 rendered no longer exists anywhere in the repo; `ViewFrame` now
  routes S0/S1/S2 to three distinct views instead of one shared shell.
- **The acceptance test**
  (`apps/api/internal/api/walkable_stations_test.go`,
  `TestWalkableStations_S0ToS3`) drives a **freshly created** project —
  never the seeded one — through `POST /projects` → `/onboarding` →
  `/framing` → `/materials` + `/materials/{mid}/open` → `/perspectives` →
  `/gate/evaluate_perspectives/attest`, asserting the projection's
  `stations[].state` turns `current`/`done` at each step (the rail is what
  the student actually sees — the test does not reach into gate internals).
  This is the test that could not have passed before this slice at all: it
  needs both the missing producers and a live `Advance` caller together.

**NO migration** (`0018_seed_demo_project.sql` untouched), **NO LLM call**,
**NO new gate predicate kind**, **NO `packages/contracts` shape break**
(`FramingFx`/`PerspectivesFx` are additive). The one `internal/store/sqlc/`
change (`graph.sql.go`, the origin-scoped `DeleteStationViewNodes` query) is
genuinely generated — a fresh `make sqlc` reproduces it byte-identically.

**The whole-branch review earned its keep again — and this time it caught a
Critical that would have shipped the slice broken.** All 13 per-task reviews came
back clean; the defect lived entirely in the seam between two of them.

1. **CRITICAL — S2's own gate had an item the S2 screen could not produce.**
   `recon_logged`'s only producer is `logSourceOpen`, whose only client is
   `SourceDossier`'s close handler — and `SourceDossier` renders only under
   `effectiveView === "素材"`, i.e. **S3 and later**. The S2 view shipped an inert
   `<li>{title}</li>` list plus `AddSourceForm`, and **adding a source is not
   opening one**. A real student at S2 could write her perspectives, add a source,
   tick the confirm — and still be told one item was missing, with no control left
   to press. The one thing this slice exists to deliver did not work.
   Root cause was the **spec's own §6.3** (「the full dossier stays at S3」), now
   amended in place. Fixed by rendering the real `SourceDossier` at S2 with
   `anchors={[]}`, so 「opening a source」 is the same act at S2 as at S3.
   **The acceptance test hid it** by POSTing `/materials/{mid}/open` directly — a
   request no S2 UI state can generate. Test-mock infidelity one level up: the
   fixture reached a state the product could not.
2. **Important — the advance was invisible.** `onOpenLogged` was fire-and-forget
   with no refetch. Fine when it only sampled reading time; wrong once the same
   call attests a gate item and runs `AdvanceAll`. The gate closed server-side
   while the rail still showed S2 `current` / S3 `locked` until a page reload.
3. **Important — two gate-changing writes never advanced.** `commitSnapshot`
   (mints/deletes `word_budget_ok`) and `orderReview` (writes
   `whole_draft_review`) — `draft_polish`'s only machine and only human item —
   were missing from the `advanceGates` set. One line each.
4. **Important — a silently-dropped row looked saved.** Both new views seeded
   local state once and never resynced, while the endpoints correctly drop blank
   rows. A term row with a definition but no term name stayed on screen looking
   saved and was gone on return. Fixed by syncing local rows to the payload that
   was actually sent — deliberately NOT a `useEffect` on `data`, which would
   clobber in-progress typing on any unrelated refetch.

Plus 5 Minors fixed (Go rune-vs-byte test that could not catch a regression;
`已定义 5/3`; `addRow` defaulting to a level she never picked; blank rows
unlocking the attestation; a duplicated `studio.RubricRowDTO`).

**Found and NOT fixed — a real gap for a later slice.** The spec claimed 「S3–S6
already have producers」; that was never checked and is **false**.
`evaluate_sources`'s `source_quality_spot_check` and `build_argument`'s
`warrant_quality_spot_check` — both `human` gate items — have **no producer
anywhere in the repo**. So the walk now stops at S3 for exactly the same class of
reason N3d was written to fix, one station further along. Spec §9 amended.

**Carried out of N3d → N3e** (its own entry below, not N6 — both need
product/coach design work, not infra work):
- The search-plan card — S1's 「打算去哪找证据」 panel ships as her own writing for
  now; the card that questions her retrieval plan attaches to it later
  without reshaping anything this slice built.
- The R-9 summing-up framework reveal — `framework_fill` is written
  server-side (`agent/card_lifecycle.go`) and read by nobody. It rode along
  in the old N3d tracker entry for no structural reason; it is orthogonal to
  the station chain this slice fixed.

**Noted, not carried anywhere** (design-acknowledged trade-offs, spec §9/§11):
the gate checks that she *did* the S1/S2 work, never *how well* — a thin
three-definition S1 still clears `frame_question` by design (RL-5: quality
judgment belongs to the assessor and her own self-score, never the gate);
`AdvanceAll` costs one extra `LoadGraph`+`ListGateStates` per gate-affecting
write (the same two reads the turn loop already does per turn — acceptable at
today's scale; if it ever isn't, the fix is to scope the walk to the head
contract); S0's `milestone_plan` attestation is thin (she accepts a plan she
cannot yet edit — if a later slice lets her edit it, the attestation moves to
that edit, the item name does not change).

### N3f · DONE — merged (2026-07-22)
Spec `docs/superpowers/specs/2026-07-22-n3f-finish-the-walk-design.md`,
plan `…/plans/2026-07-22-n3f-finish-the-walk.md`; 13 tasks (12 build + this
spec-amendment-and-tracker task).

**The finding that started this slice.** N3d's own whole-branch review had
already caught its spec's unchecked claim 「S3–S6 already have producers」 was
false, and named the two `human` items with no producer at all. This slice
did the check properly, tracing every gate item in
`packages/contracts/skills/writing-project.json` to a producer, and found the
gap was bigger than those two: six items across S3/S4/S6 were unsatisfiable
for a real student —

| Station | Item | Tier | Status before this slice |
|---|---|---|---|
| S3 `evaluate_sources` | `source_risk_notes` | student_written | she writes it; nothing attests it |
| S3 | `source_quality_spot_check` | human | no producer anywhere |
| S4 `build_argument` | `warrants` | student_written | she writes it; nothing attests it |
| S4 | `steelman` | student_written | she writes it; nothing attests it |
| S4 | `warrant_quality_spot_check` | human | no producer anywhere |
| S6 `reflect_archive` | `declaration_signed` | human | no producer anywhere |

Same lesson as N3d, one level up: invisible because
`0018_seed_demo_project.sql` hand-writes gate rows for the seeded demo
project, so the rail looked alive without any of these ever having been
exercised by a live code path.

**What's live now:**
- **The three attestations** (`source_risk_notes`/`warrants`/`steelman`,
  spec §4) run from graph state in `attestS3S4`, called from
  `submitProjectCard` right before its existing `advanceGates`. `steelman`
  accepts **two** producers — the Toulmin `counter` slot text, or a
  completed `steelman` card instance — because the standalone 钢人卡 mints no
  graph node; without the second producer a student who did that card and
  not the Toulmin slot got no credit. Un-attestation is symmetric: each
  check re-runs on every relevant submit and can re-open a gate a deletion
  should re-open.
- **信源体检 / 论证体检** (spec §5) — two new student-triggered spot-checks,
  parallel to 整稿体检, through `ProposeReview`'s enforcement stack verbatim
  (banned-phrasing + output check, one violation rejects the whole order,
  usage metered even on rejection). `POST
  /projects/{id}/contracts/{contractId}/spot-check`, valid only for
  `evaluate_sources`/`build_argument`. Idempotent on a SHA-256 fingerprint
  of exactly what the check reads (S3: `(material_id, risk_note)` pairs; S4:
  `(slot_id, text)` pairs) stored in the intervention's `anchor` jsonb — the
  same additive-anchor seam Slice 8b used for `voice`, no migration.
  `orderable` and the batch of items shown are both computed server-side
  from that fingerprint (`studio.projectSpotCheckFx`): the batch whose
  fingerprint matches the CURRENT fingerprint is preferred when one exists
  (`orderable=false`), falling back to the most recent batch only when none
  matches (`orderable=true`) — recency alone would mis-render an
  edit-then-revert sequence as both stale and (wrongly) re-orderable. A
  shared `WorkOrderItem` was extracted from 整稿体检's own work-order panel
  and reused at all three sites, with the band chip made optional rather
  than blank-stringed so a spot-check row renders no chip at all.
- **AI 使用申报单** (spec §7) — a pure projection over data that already
  exists (no model call): 提问/追问 rounds, 三键处置 counts, 工具卡 spont/prompted
  split. Its own `POST /projects/{id}/declaration/sign` endpoint (not
  `attestGate`, which stays restricted to `student_written` names so a
  caller can't forge a machine/human item) recomputes the counters
  server-side, persists them into a `declaration` graph node, then flips
  `declaration_signed` — the signature records what was signed, not a live
  view that keeps moving. `AI 代写正文 = 0` is recorded as a claim by
  construction, not a measurement (holds only because RL-1 keeps review
  `fix` advisory and no code path writes model text into the edit buffer) —
  a future AI-to-draft path must measure or delete this counter.
- **The L1 block-id collision** (spec §6): `materialize.Segment` numbers
  blocks `b0, b1, …` per material, but `blockLookup` was a flat map across
  **all** of a project's materials — last material wins — so with ≥2
  materials an L1 anchor could resolve to the wrong article. Fixed by
  material-qualifying the block label in the prompt (`m0:b0`) and keying the
  lookup by the qualified id, fail-closed (no bare-id fallback). L2/L3
  untouched (L2 never resolves block ids from the prompt; L3 makes no model
  call).
- **The producerless-gate guard**
  (`apps/api/internal/skills/producers_test.go`) enumerates every gate item
  name in `writing-project.json` and fails if any `student_written`/`human`
  item has no entry in an explicit Go producer registry, or any machine item
  falls through `evalMachineItem`'s `default` branch. This defect class has
  now shipped three times (S1/S2 in N3d; S3/S4/S6 here) — this guard is the
  single highest-value artifact in the slice.
- **The fresh-project S0→S6 walk** (`apps/api/internal/api/walk_s0_s6_test.go`)
  drives `POST /projects` through every station in order, on client-reachable
  endpoints only, never the seeded demo project — N3d's two learned rules,
  extended one station further.

**NO migration** (every seam already existed: `intervention.type` is free
text, `intervention.anchor` is jsonb, `graph_node.type` is free text). **No
card JSON changed.**

**The whole-branch review caught defects again, none of them structural
surprises this time but all worth keeping:**
1. **Important — `projectCoach` had no intervention-type allowlist.**
   `review_item`/`spot_check_item` interventions carry marshalled work-order
   JSON as their `Body`, not prose, so every 整稿体检 work-order item had been
   rendering in the 陪练 conversation as a raw JSON blob since Slice 8 —
   invisible for the same reason the dead gates were: the seeded project's
   interventions are hand-written prose, never one of these two types. Now
   both types are excluded from the coach rail, pinned with a projection
   test.
2. **Important — the acceptance test's Toulmin/SIFT envelopes were shapes
   the real web client cannot produce.** The same mock-infidelity class this
   repo has been bitten by before (Slice 12): `toulminAnchors` sent one
   combined text+material_id anchor per slot, but the real serializer
   (`graphStateToAnchors`) emits them **split** — a text anchor and a
   separate source anchor per `cites` edge. `fillSiftAnchors` similarly sent
   AI-generated anchors the real client discards entirely, instead of
   `StudioCompareCard.buildAnchors`'s spec-fields-only envelope. Rebuilt both
   to match the real serializers.
3. **Minor but real — a fail-closed test whose fixture could not
   discriminate.** `TestParseAnchorGenRejectsUnqualifiedBlockIDWithMultipleMaterials`
   gave both materials block id `b0`; the bare form was ambiguous under the
   removed fallback too, so the test errored identically whether the fix
   was present or not and pinned nothing. Fixed by giving the two materials
   different block ids, the only shape that actually tells the fixed and
   broken code apart.

Also fixed in review: S4 (`build_argument`) spot-check coverage had never
run in the endpoint test suite; a rejected spot-check proposal wasn't
metered the same way a rejected 整稿体检 is; `TouchProject` wasn't called on
ordering a spot-check; the S3 panel's empty-state copy named a precondition
(evaluating a source) the gate does not actually require, instructing an
action before it was possible.

**Found and deliberately NOT fixed — carried forward:**
- L2 still attributes every anchor to `materials[0].ID` (`agent/anchors.go`)
  — the same assumption as the L1 bug this slice fixed, masked because at L2
  the student locates the span herself, which corrects the material.
  Explicitly deferred (spec §6, §10); needs its own decision about what an
  unlocated anchor's material should be.
- `orderable` is false whenever a station has no targets at all — the very
  first state at both S3 and S4 — so both panels' first-ever screen shows a
  disabled button; the copy there must say what has to exist first and must
  never instruct pressing it (spec §10).
- The S3 spot-check's target list names only *adding* a source as its
  precondition, not *evaluating* it — `sourceSpotCheckTargets`'s own comment
  records that an unevaluated article is still a target, because 体检
  covers what she has done **and** has not done (spec §10).
- `AI 代写正文` stays an unmeasured claim-by-construction (§7 above); a
  future AI-to-draft path must measure or delete it.
- Teacher-facing adjudication of the three `human` items (S3/S4's checks,
  S6's declaration) remains roadmap Slice 13, deferred on teacher-side
  design — this slice's `human` reading (an adjudicating voice the student
  summons, not a teacher, not a checkbox) is a deliberate interim choice,
  spec §3.

**Whole-branch review wave 2 (2026-07-23) — I1/I2/M1/M-coach:**
- **I1.** `StructureView`'s gate banner claimed the S4 gate had passed the
  moment the five Toulmin slots were done — false, since the gate also
  requires the `human` item `warrant_quality_spot_check`, whose producer
  (论证体检) is a separate panel below the banner. Copy no longer claims the
  gate passed or that S5 is reachable; states only what the five slots
  achieved. Deliberate deviation from the binding design (`dc.html:975`),
  which predates the S4 human gate item — noted inline.
- **I2.** L1 block-id labels used the real material id as the qualifier
  (36-char UUID on the studio path), while the L1 instruction's own example
  still showed the short `m0:b0`. Fixed per spec §6's original intent, but
  with a prompt-LOCAL index alias (`m0`, `m1`, … = the material's index in
  the same slice) rather than the material's real id — `BuildMaterialContext`
  and `blockLookup` derive the identical alias from the identical materials
  slice, so no shared table or migration is needed. A new test
  (`TestMaterialAliasMatchesL1InstructionExample`) pins that the rendered
  label and the instruction's example describe the same format even when the
  real material id is a UUID — the gap the old
  `TestBuildMaterialContextQualifiesBlockIDs` never checked.
- **M1.** `ingestMaterial` created a material without recomputing
  `source_risk_notes` — adding a 3rd article after 2/2 were evaluated left
  the gate item reading solid until an unrelated card submit incidentally
  recomputed it. Now calls `attestS3S4` then `advanceGates`, same order as
  `submitProjectCard`.
- **M-coach.** `projectCoach`'s work-order exclusion was a DENYLIST
  (`review_item`/`spot_check_item`) — any future machine-readable
  intervention type would fall through and render as a raw JSON blob in the
  陪练 conversation, the exact defect this slice had just fixed for those two
  types. Inverted to an ALLOWLIST (`question`/`diagnostic`/`flag`), with a
  test (`TestProjectCoach_AllowlistExcludesUnknownFutureType`) pinning that
  an unrecognized type is excluded without anyone adding it to a denylist
  first.
- **Spec corrections** (`2026-07-22-n3f-finish-the-walk-design.md`, amended
  in place): §5.4 now names the fingerprint's actual input
  (`SpotCheckTarget{ID,Name,Detail}` rows, `Detail` = 档位/一句话收获/作用与风险/
  横向核查 — not a bare `(material_id, risk_note)`/`(slot_id, text)` pair
  list); §10 now says adding an unevaluated material changes the S3
  fingerprint immediately (it is still a target, with a `「（未写）」`
  placeholder), not only once she writes a risk_note; §5.2 no longer claims
  the S4 check reads which sources each slot cites (it reads slot text
  only); §4 now states precisely what un-attestation does — `attestS3S4`
  clears the ITEM, but `AdvanceAll` never un-confirms a STATION already
  solid, so a station can stay `done` while its item count silently drops
  below total (a known limit, not fixed here — that's its own slice). §10
  gained three more known limits: 论证体检 is effectively one-shot (the
  classifier retires the toulmin offer on any card_instance, and both
  `projectStructure`/`argumentSpotCheckTargets` take the first node per
  type, so no path re-opens an S4 slot's projected text); S6 is skippable
  (`canFinish` keys on `whole_draft_review` only, not
  `declaration_signed`); concurrent orders can double-charge (two in-flight
  requests both see a fingerprint/snapshot miss and both persist a batch —
  the client's `pending` flag guards one tab only).

### N3e · DONE — merged (2026-07-23)
Spec `docs/superpowers/specs/2026-07-23-n3e-finish-the-thinking-moments-design.md`,
plan `…/plans/2026-07-23-n3e-finish-the-thinking-moments.md`; 6 tasks
subagent-driven. **NO migration, NO gate item, NO new LLM call.** The two
items that N3d/N3f split out — both needing product/coach design rather than
gate-plumbing — are now shipped, closing the N3 family.

**What's live now:**
- **The search-plan card (`检索方向审视`, `packages/contracts/cards/search-plan.json`).**
  A project-scoped `matrix` card that takes the student's OWN retrieval
  directions (from her S1 `preregistration` node) as its fixed rows and has
  her interrogate each: 会给哪一类证据 / 系统性看不见什么 / 能否证伪你. The rows are
  **seeded server-side** at surface time (`seedSearchPlanAnchors`,
  `api/studioturn.go`, mirroring `surfaceAnchors`' persist-and-carry contract)
  — one `{quote:direction, dimension:"evidence_type", answer:""}` anchor per
  direction, which the existing matrix serializer rehydrates as a labelled
  empty row and the completion predicate skips until filled. **No renderer
  primitive change beyond parameterizing the matrix row-noun** (`params.row_noun`,
  default `视角` so `perspective-matrix` is untouched) — the 视角 vocabulary
  was hardcoded in three UI strings, a gap that only surfaced when a SECOND
  matrix card reused the primitive.
- **The summon** (`agent.SurfaceCardCandidates`, a new project-scoped branch)
  fires when a `preregistration` node exists, **no source has been evaluated
  yet** (`!anyEvaluated`), and no instance has been seen — a PRE-sourcing
  offer per the card's own `when`. The `!anyEvaluated` bound was a refinement
  surfaced in implementation: without it the card stayed eligible from S1 and
  hijacked the S4 toulmin surface (broke the S0→S6 walk). It also makes
  search-plan and perspective-matrix/toulmin (both need `anyEvaluated`)
  **mutually exclusive** — no priority conflict possible.
- **The R-9 framework reveal** finally consumes what every card has written
  since the runtime shipped. `ConsolidationPayload` is a pure function of the
  CardSpec (`framework = spec.Name`, dims = params), so the reveal is
  **pure-web**: `shell/growth/ToolkitCards.tsx` shows, for each collected card
  whose spec sets `consolidation`, a quiet 「你练的思路」 block carrying
  `steps[0].methodology.why`. Lights up all six existing cards + search-plan.
  The studio completed-card panel and 过程树 pane the first design assumed
  **don't exist live** (`CompletedCard.tsx` is dev-harness-only; a card's sheet
  unmounts on submit; the 过程树 is drawn in dc.html but never built) — the
  工具卡 tab is the one real "collected cards" surface, chosen deliberately.

**No migration** (`framework_fill` since 0016, `preregistration` since N3d).
`framework_fill` stays **written-but-unread** by design — the reveal renders
from the spec, which carries more (the methodology) than the frozen jsonb; a
future record-fidelity surface could read the column, this slice does not.

**The final full-suite gate caught what the per-task test scopes could not —
the recurring registry-enumeration tax.** Adding one card to `CARD_REGISTRY`
broke FOUR enumeration tests across all three suites (Go system-prompt golden;
Go `loader_test` count; web `Harness` option-count; contracts `library`
count+catalog). Task 1 ran only `npm test -- registry` + the Go cards package,
so only the Go `loader_test` count was caught in-task; the other three stayed
red until Task 3 (agent package) and the controller's final gate surfaced
them. **Lesson: a task that adds a card MUST run all three FULL suites, never a
`-run` subset — a new card touches every registry-enumeration test in the
repo.**

**Known limits (carried forward):**
- The R-9 reveal is a **record, not an in-the-moment beat** — it lives in the
  成长报告, not the workspace at completion time, because no live studio
  completed-card panel or 过程树 pane exists. A true in-studio completion
  moment, or the 过程树 itself, is separate work.
- **Seeded rows are a summon-time snapshot** — if she edits her search plan
  after the card surfaces, the rows do not re-sync (re-summon does not re-fire;
  offer never a wall).
- **`disconfirm` is a self-report**, not a check (铁律 1 — the card makes her
  think; the assessor judges quality, the gate never does).
- **`framework_fill` remains written-but-unread** (above).

### N3c · DONE — merged (2026-07-22)
Spec `docs/superpowers/specs/2026-07-22-n3c-guidance-fade-and-span-creation-design.md`,
plan `…/plans/2026-07-22-n3c-guidance-fade-and-span-creation.md`; 10 tasks
subagent-driven plus two per-task fix waves and a whole-branch fix wave.
**NO migration, NO card JSON change** — verified by diff; the one
`internal/store/sqlc/` change is genuinely generated (a fresh `make sqlc`
reproduces it byte-identically).

**The guidance ladder is live.** The AI stops handing her the sentence (L2), then
stops handing her the question too (L3). **The level is never stored anywhere** —
it is carried entirely by the anchors' own shape: `author:"ai"` + question + span
= L1; `author:"student"` + question + blank span = L2; `author:"student"` + blank
question + blank span = L3. So there is no new column, no new API field, no
contracts change for the level, and the reload path re-derives it for free.

- **The fade has a real producer** (`agent.GuidanceFor` + a new project-scope
  `CountCompletedCardUsesByUser`): 0 completions → L1, 1 → L2, 2+ → L3, computed
  at surface time in `surfaceAnchors`, `annotate` primitive only (compare/SIFT
  stays L1). Silent — no badge, no level name, nothing congratulatory (铁律 2).
- **L2 gets its own prompt**, not a stripped L1: an L1 question points *at* a
  sentence and would hand her the answer to the locating task. **L3 makes ZERO
  LLM calls** — the scaffold fading also removes the spend (nothing to meter,
  which is not the "bailed before metering" defect class).
- **Locating is never a wall** (铁律 2): no completion predicate changed, so a
  span is never required server-side and a card can never strand `active`. The
  card offers 「找不到合适的句子」, and both that escape and a located span can be
  undone/re-picked. Escapes and locations are recorded (`span_not_found` /
  `span_located`, an additive `TraceEvent` extension in lockstep across the Zod
  union and Go's `traceKinds`) — friction becomes signal (铁律 4).
- **Bug fixed en route:** anchor span offsets were Go **byte** offsets consumed
  by the web as UTF-16 slice indices. On Chinese material every AI anchor that
  found its quote highlighted the *wrong text*. Offsets are now **rune indices**
  on both sides.

**The whole-branch review again caught what per-task reviews structurally could
not — and the per-task reviews were clean.** Two Criticals in-slice, both
invisible because no task's fixtures could reach the bad state:
1. **L2/L3 anchors pinned to the project's OLDEST material.** `surfaceAnchors`
   narrows the material list only for `compare`, so an annotate card gets every
   project material and `materials[0]` is the first article ever pasted — not the
   one under review. On submit `checkedMaterialID` reads the anchors, so evidence
   would be promoted from the **wrong article**. Root cause was the PLAN's own
   text asserting "the card is single-material at L2". Invisible because every
   test round used a fresh one-material project.
2. **The fade's counter was fed by two ungated producers.** `chat.go` and
   `course_session.go` write `status='completed'` **unconditionally** — no
   predicate, no anchors (thin by design) — and chat surfaces the same `craap`
   card. Two throwaway in-chat submits would have put her FIRST-EVER Studio CRAAP
   at L3. Query narrowed to project scope (still across all her projects); the
   spec's original "parity with the 工具卡 tab" justification was **replaced**,
   not appended to.

Also controller-found, not by any reviewer: the three new cross-pane tests were
**flaky (~50% under load)** — they awaited a project-projection label then
synchronously queried a conversation-snapshot button. A green suite you cannot
trust is the documented root cause of five Criticals in an earlier slice.

**Carried out of N3c → N6:**
- **[RAISED PRIORITY] At L1, anchors pin to the wrong material when a project has
  ≥2 materials.** `blockLookup` builds one flat `map[blockID]` across ALL
  materials and is last-wins, while `materialize/segment.go` numbers blocks
  `b0..bN` **per material** — so every id collides and resolves to whichever
  material sorts last. CRAAP on M2 in a 3-material project mints `evaluated-as`
  against M3 and highlights the wrong article. **Pre-existing**, and N3c fixed
  this shape for L2/L3 only — which now leaves *the beginner path as the only
  broken one*. Fixing it changes what every first-time student sees, so it needs
  its own design call.
- `apps/web/src/shell/chat/ChatSurface.test.tsx` has a pre-existing
  load-sensitive 5s timeout flake — it makes `npm test` unreliable as a gate.
- Minor: `computeOffsets`' doc comment claims "quote stays authoritative for the
  UI"; no UI path uses `quote` for positioning.
- Minor: an L2 anchor whose model question comes back empty silently renders as
  L3 (degradation only).
- Minor: a reload mid-fill silently discards in-progress located spans and the
  pending trace (persisted anchors are still blank, so nothing lies).
- Spec §8's promised *immediate* green article highlight is **not shipped** — the
  located span reaches the article only after submit + refetch. Amended in place.

### N3b · DONE — merged `eb0c419` (2026-07-21)
Spec `docs/superpowers/specs/2026-07-21-n3b-moment-classifier-and-refeed-design.md`,
plan `…/plans/2026-07-21-n3b-moment-classifier-and-refeed.md`; 8 tasks subagent-driven
plus a whole-branch fix wave. **Go-only: no migration, no sqlc regeneration, no
`packages/contracts` change, no card JSON change, no web change** — verified by diff.

- ~~**[N3b]** `sort`/`scale` built but deliberately unwired; needs the semantic classifier.~~ **DONE** — `agent.ClassifyMoment` (`internal/agent/moment.go`) is a real fifth subagent: one chaperone-tier call, a **closed set** of 3 moments (`fact_opinion`→`fact-opinion-value`, `overclaim`→`certainty-spectrum`, `one_sided`→`steelman`), reply matched by exact equality against the ELIGIBLE ids so a real-but-suppressed moment collapses to none. `momentCard` is the single source for every moment's card id/flag/reason/criterion. Wired into BOTH surfaces — Studio (`RunAgentStep`) and Chat (`RunChatStep`, behind the structural link→CRAAP moment, which still wins).
- ~~**[N3b]** No live anchor-refeed seam.~~ **DONE — 摘要回灌 is live for the first time.** Studio: `Trigger{card_refeed}` now carries the card instance, `BuildCoachContext` renders `SerializeCardForRefeed`'s payload, and the coach asks ONE question about what she just wrote; it outranks every other candidate (so a following `surface_card` is deferred one hop — verified never starved). Chat refeeds **differently on purpose**: its submit is thin and has no post-submit turn, so the summary rides the student's NEXT message — zero extra LLM calls, same serializer.
- ~~**[N3b]** Toulmin has no skip-suppression.~~ **DONE** — mirrors `perspective-matrix`'s any-status rule.
- ~~**[N3b/N6]** `needsMaterial`'s closed effect set has no compile-time link to `GraphEffects`.~~ **DONE** — the guard test now iterates the real embedded card catalog's `GraphEffects[].Kind`, so a new kind arriving via card JSON is caught.

**The whole-branch review (opus) again caught a CRITICAL that 8 clean per-task reviews structurally could not — and it was the PLAN's bug.** The Studio pre-gate's "no card in flight" check did not exist: `SurfaceCardCandidates` signals in-flight by returning `nil`, so the call site's `!hasSurfaceCard(cands)` was true *precisely when* a card was in flight (that expression orders the semantic pass behind the structural one; it never reads `ci.Status`). A student typing her next message before opening an offered card got a second `card_instance` minted over the first — and any stranded row then silenced `SurfaceCardCandidates` **project-wide forever**. Invisible because `fakeAgentStore.graph` is a static fixture no test ever set an in-flight card on, and the real-DB E2E uses a material-less project where the state cannot arise. Fixed `0657891` (+ the same gap in Chat, `1a4f3f9`).

**Carried out of N3b → N6:**
- **Classifier spend is not bounded.** A `none` answer retires nothing, so in a mature project (`SurfaceCardCandidates` silent every turn) the classifier runs on **every** student turn ≥12 runes indefinitely, roughly doubling per-turn LLM calls. Needs a per-project cap or backoff before real classrooms. (The spec's original "monotonically decreasing" claim was false and has been corrected in-place rather than left standing.)
- **`no_unsupported_claim` passes vacuously.** It is a negation scan over `claim` nodes, so with zero claims it is trivially true: a student who skips toulmin never mints one, yet `build_argument` still reaches `machine_clear` once `concession` exists. The argument station can be cleared with no argument in it. Pre-existing; surfaced by N3b's starvation trace.
- `renderCompletedCard` (`chat_step.go`) duplicates `BuildCoachContext`'s refeed render loop — scope-forced; extract a shared helper opportunistically.
- `CandidateMoves` tags its unsupported-claim candidate `D5` 反馈理解与修改理由, which reads like a pre-existing mislabel (`D3` 证据与信源意识 fits).

### Cross-cutting / Slice 0
- ~~**[N3]** Interaction primitives incomplete: `sort`, `matrix`, `scale` not built.~~ **DONE (N3a, `27492fd`) — C1 library is 6/6.** *(31–32)*
- **[N4]** OPCVL (HS-D\*) rubric + behavior ladders never built. *(115, 777)*

### Slice 1 — annotate
- ~~**[N3c]** Student free span-creation (text-selection) — guidance L2/L3; only L1 ships.~~ **DONE — merged (2026-07-22).** See the N3c section below. *(124–126, 302–304)*

### Slice 2 — runtime loop / classifier
- **[N3b]** Cheap-model classifier hook is a seam only (needed for semantic card-moments — and now also to summon N3a's `sort`/`scale` cards). *(140–141)*

### Slice 3 — card runtime
- **[N1]** `GetCardInstance` not project-scoped (takes only `cid`). *(154–157, 468)*

### Slice 4 — skill/gate/planner
- **[N6]** Flagship planner judgment layer (prioritize unlocked contracts + `route_to_course` on stalls); planner is deterministic today. *(178–180)*
- **[N6]** Skill card-refs validated only by a test, not at `Load`. *(180–181)*
- **[N6]** `Advance` upsert-before-event ordering. *(181)*
- **[N6]** Migration 0017 down fails if skill-typed rows exist. *(182)*

### Slice 5c / 5c-2 — interactive loop
- **[N5]** Token-streaming of coach replies. *(256)*
- **[N6]** Multi-step / debounced loop + `check_gate` debounce (single-step today). *(183, 207, 256)*
- **[N1]** Live gate handling — controller drops gate SSE; `studioturn` hardcodes `0,0`; no live passed/total. *(256, 468)*
- **[N1]** Live anchor label — runtime stores `{kind,id}`; live+reload render chip-only, no human label. *(256)*
- **[N1]** Onboarding live producer mismatch — `agent.Intake` writes `{text}` but the reader wants `{restate_prompt, rows}`; S0 任务解码 moment not live-produced. *(228, 258–259, 362–363, 468)*
- **[N1]** Unique partial index on `chat_thread.seeded_project_id` (getOrCreateThread TOCTOU). *(257–258, 362, 468)*
- **[N6]** Non-transactional card writes on activate/skip. *(281–282)*

### Slice 5d — routing cutover
- **[N1]** **Project creation has no home** — `POST /projects` + intake exists, no UI/entry. Zero-project student stuck at empty state. *(356–358)*
- **[N5]** `source:"voice"` turn-provenance tag gone (needs the turn contract). *(399–401)*
- **[N6]** `surfaceAnchors`/`renderChallenge` bail on empty anchors before metering (unmetering hole). *(401–403)*
- **[N1]** Studio composer has no Enter-to-send. *(403–404)*
- **[N1]** dc.html still says 「批判思维」工作台 in 2 spots (`dc.html:158`, `:2519`). *(404–405)*

### Slice 6 — CRAAP fill→mint
- **[N3e]** R-9 summing-up framework reveal. *(303–304)*
- **[N4]** `RISK_NOTE_QUESTION` drops "／局限" vs binding copy. *(304)*

### Slice 6b / 6c — material / SIFT
- **[N3e]** Search-plan card (S2) — needs its own coach design. *(452, 468, 498)*
- **[N2]** RL-2 citation half — no citation surface yet (tied to 写作/Slice 8). *(452–453, 499)*
- ~~**[N3d]** S2 perspective map (视角与素材) — graph-node-backed.~~ **DONE (N3d)** — `PerspectivesView.tsx` + `POST .../perspectives`; see the N3d section above. *(454, 499)*
- **[N6]** 偏弱 verdict chip has no honest producer. *(453–454)*
- **[N1/N6]** Event-model normalization — DB `type`/`surface` columns vs flat Zod `StudioEvent`; reader must merge before validating (systemic, all 8 event types; dead `EVENT_TYPES`). *(455–458, 500–501, 954)*

### Slice 7 / 7b — structure
- **[N6]** No skills canonical/mirror guard test. *(563–564)*
- **[N6]** `gen-go-fixtures.ts` dead code. *(564)*
- **[N6]** Pre-existing `packages/contracts` tsc failure. *(565)*
- **[N1]** In-pane `GateBanner` over-promises (green at 5 cards `done`, but station gate only `machine_clear` after external Advance). *(590–592)*

### Slice 8 / 8b — writing + whole-draft review
- **[N2]** Automated citation front-to-back matching (keystone shipped attestation only). *(640–641)*
- **[N2]** Snapshot diffs feeding the process record. *(641)*
- **[N4]** EE/AP board-specific passes (only 0457 seeded). *(601–602, 638, 686–687)*
- **[N1]** 只读 preview renders live buffer, not committed snapshot content (post-commit drift). *(642–644)*
- **[N6]** Enforcement narrowed to banned-phrasing; `output_check_verdict` unset. *(644–646)*
- **[N6]** Selected voice state not reset on snapshot/project change (cosmetic). *(688–690)*

### Slice 9 — readiness + reflect + export (◐, the big one → N2)
- **[N4]** Other four gauge skins: 9239 grid, AP switches, AP band-portraits, TOK needle. *(728–729)*
- **[N2]** Self-score block. *(90, 698, 730)*
- **[N2]** Reflection pack / retro (RL-4 reflection editor). *(90, 698, 730)*
- **[N2]** Prediction loop S0↔S6. *(90, 698)*
- **[N2]** AI-usage declaration block. *(90, 698, 730)*
- **[N2]** Export forks (RL-4). *(90, 698)*
- **[N2]** 表A/B/C/G cross-station readiness (only 表D/E/F/H wired). *(730–731)*

### Slice 10 — assessor + growth report (◐)
- **[N4]** T1–T7 thinking leaps view. *(91, 775–776)*
- **[N4]** OPCVL rubric (dup). *(777)*
- **[N4]** Benchmark + fine-tune stages. *(777)*
- **[N6]** Async assessment (river worker; inline today — the old "P4"). *(778)*
- **[N6]** Card `Dimension` is method tag, not CT D-code (no `ct_dimension` on `cards.Spec`). *(778–780)*
- **[N2]** `WordCounts` carries only 0–1 entries (no snapshot-history query). *(780)*

### Slice 11 — chat (◐)
- **[N5]** Multimodal input (icons render, inert). *(826)*
- **[N5]** Chat→project intake seeding (`seeded_project_id` + fragment copy). *(826–827)*
- **[N5]** Off-record thread control (open product Q §606). *(827–828)*
- **[N3b]** Semantic non-link card-moments (opinion→steelman, comparison→matrix). *(828)*
- **[N5]** Thread evidence nodes / `graph_effects`. *(829)*
- **[N5]** Competence wiring (dormant platform-wide). *(829–830, 887)*
- **[N6]** `ChatCardOfferDTO` parity decorative (ships via snake_case `sse.Card`). *(824–825)*

### Slice 12 — course (◐)
- **[N5]** Terminal-assessment challenge + machine-never-`solid` adjudication. *(882–883)*
- **[N4]** Golden / banned example packs per phase. *(883)*
- **[N5]** Course voice (按住说话 renders, inert). *(883–884)*
- **[N5]** Course→project seeding. *(884)*
- **[N5]** Multi-course authoring (one seeded course today). *(884–885)*
- **[N5]** Session restart. *(885)*
- **[N6]** Non-transactional positional material↔card pairing in `mintPhaseCard`. *(885–886)*
- **[N1]** Course terminal not idempotent at the API layer. *(887)*
- **[N1]** Unawaited card submit/skip can leave a reload-recoverable wall. *(887–888)*
- **[N6]** No test runs migrations **Down** anywhere (repo-wide). *(888)*

### Platform-wide (surfaced in A-series)
- **[N2]** Notes feature — 我的学习笔记 / 导出笔记 designed, no feature exists. *(949–950)*
- **[N4]** Challenge quality — only engagement, no quality notion. *(952–953)*
- **[N6]** Billing / entitlement — `HasEntitlement` is `return true, nil`, no seam, no test. *(950–952, 1013, 1078)*
- **[N6]** `CostNumeric(cost, true)` latent bug in both LLM-call recorders (dormant post-V4). *(1009–1011, 1077–1078)*
- **[N6]** `proposed`-status chat cards → assessor as `DispositionUse{Kind:"proposed"}`; revisit with B's chat rubric. *(1007–1009)*
- **[N1]** Report re-POSTs a flagship call on every mount after 422; StrictMode double-fires in dev. *(953)*
- **[N1]** `finishProject` no DB lock against concurrent double-submit (409 self-heals). *(1073–1074)*
- **[N6]** Stale `generateAssessment` comment in `course_assessment.go`. *(1075)*
- **[N6]** `GrowthReport.test.tsx` no-generate assertion is a blunt whole-doc query. *(1076–1077)*
- **[N1]** Finish button lives only in 就绪度 view (discoverability). *(1078–1079)*

---

## Out of scope here
- **Slice 13 · Teacher dashboard** — class heatmap + risk column, individual trajectory, single-conversation replay, one-click actions; teacher & parent report versions. Gated on teacher-side design. *(67–68, 94, 776–777)*
- **Voice TTS/ASR** — built, merge-ready on `feat/voice-tts-asr`, unmerged. Several "voice inert" items above (N5) resolve when it lands. Its merge is a separate decision.

---

## Next action
Planning **N1 · Close the loop** first (own spec → plan → build). Its centerpiece is the
project-creation entry point (the funnel blocker); it also folds in the cheap
correctness seams that make the existing writing loop trustworthy end-to-end.
