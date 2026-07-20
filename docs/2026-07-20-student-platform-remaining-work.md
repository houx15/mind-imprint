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
| **N2 · 评估 view (S0/S6)** | Reflection pack, prediction loop S0↔S6, self-score, AI-usage declaration, export forks (RL-4) | L | The biggest unbuilt student surface; one coherent view. |
| **N3 · Interaction breadth** | `sort`/`matrix`/`scale` primitives + student span-creation L2/L3 + semantic non-link card-moments (needs the classifier hook) | L | All "richer thinking moments"; share primitive + classifier infra. |
| **N4 · Multi-board breadth** | OPCVL rubric + ladders, EE/AP board packs, other gauge skins, T1–T7 leaps | L | Config/rubric breadth behind existing interfaces. |
| **N5 · Chat/Course completion** | Voice merge, multimodal, chat→project + course→project seeding, terminal-assessment challenge, multi-course authoring, competence wiring | L | Finishes the two keystone-only surfaces. |
| **N6 · Infra hardening** | Flagship planner judgment, async assessment (river worker), `HasEntitlement`/billing seam, `CostNumeric` bug, migration-Down tests, misc tech-debt | M | Pure platform/infra; low product-visibility; can run anytime. |

---

## Full remaining inventory (by slice of origin)

Each item tagged with its target finishing slice `[N#]`. Line numbers are into the
roadmap file.

### Cross-cutting / Slice 0
- **[N3]** Interaction primitives incomplete: `sort`, `matrix`, `scale` not built (only `annotate`/`graph`/`compare`). *(31–32)*
- **[N4]** OPCVL (HS-D\*) rubric + behavior ladders never built. *(115, 777)*

### Slice 1 — annotate
- **[N3]** Student free span-creation (text-selection) — guidance L2/L3; only L1 ships. *(124–126, 302–304)*

### Slice 2 — runtime loop / classifier
- **[N3]** Cheap-model classifier hook is a seam only (needed for semantic card-moments). *(140–141)*

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
- **[N3]** R-9 summing-up framework reveal. *(303–304)*
- **[N4]** `RISK_NOTE_QUESTION` drops "／局限" vs binding copy. *(304)*

### Slice 6b / 6c — material / SIFT
- **[N3]** Search-plan card (S2) — needs its own coach design. *(452, 468, 498)*
- **[N2]** RL-2 citation half — no citation surface yet (tied to 写作/Slice 8). *(452–453, 499)*
- **[N3]** S2 perspective map (视角与素材) — graph-node-backed. *(454, 499)*
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
- **[N3]** Semantic non-link card-moments (opinion→steelman, comparison→matrix). *(828)*
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
