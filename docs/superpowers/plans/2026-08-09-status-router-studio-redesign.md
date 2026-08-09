# Status-Router Studio Redesign — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development or superpowers:executing-plans to implement task-by-task. Steps use checkbox (`- [ ]`) syntax.

**Goal:** Replace the one mega-orchestrator prompt with a per-status registry + deterministic server-side transitions running on a fast non-reasoning model, then give the writing room separate proposal/essay documents.

**Architecture:** A `studioflow` registry maps each canonical status (`topic|framework|proposal|essay|review`) — a COLLAPSE of the existing persisted `StudioStage` values, so no data migration — to `{goal, systemPrompt, tools, cards, surface, doc}`. The coach turn builds a SHORT status-specific prompt (goal + that status's 2-4 tools only), calls a fast resolver, applies only the status's tools, then a deterministic router owns transitions/plan-gen and emits a one-tap `nextStep`. Phase B keys writing on `doc_kind`.

**Tech Stack:** Go (net/http, sqlc @v1.27.0, pgx, goose), DeepSeek (v4-flash / v4-pro thinking-off vs v4-pro reasoning), React+Vite+TS, Zod contracts.

## Global Constraints

- Client NEVER calls the model directly; all LLM via the backend gateway, metered (purpose per call). Keys server-only.
- Card spec single source of truth = registry; decision layers derive from it, never a hand-written second copy.
- Standard envelope shape unchanged; server validates the outer envelope only.
- 四条铁律: AI 克制 (never writes student text), 不操纵 (transitions are one-tap-confirmed; 触发自动，打开由学生确认), 一次只问一个, 过程即数据.
- 评估绝不降级: the eval resolver (`NewEvalKeyResolver`) is untouched — flagship reasoning only. Fast model applies ONLY to the per-status conversational router.
- sqlc: `CGO_ENABLED=0 go tool sqlc generate`, pin `@v1.27.0`. Go api suite runs foreground (testcontainers).
- Deploy = commit to main + push, then `.deploy-local/deploy.sh <api|web|full>` (resets server to origin/main).

---

## Phase A — Router core

### Task A1: Canonical status + stage→status collapse

**Files:**
- Create: `apps/api/internal/agent/studioflow.go`
- Test: `apps/api/internal/agent/studioflow_test.go`

**Interfaces:**
- Produces: `type FlowStatus string` with consts `FlowTopic|FlowFramework|FlowProposal|FlowEssay|FlowReview`; `func StatusForStage(StudioStage) FlowStatus`; `func (FlowStatus) StageFloor() StudioStage` (the canonical persisted stage a status maps back to when the router sets it).

- [ ] **Step 1: Failing test** — `StatusForStage` collapses: topic_discussion→topic; proposal_forming & plan_generation→framework; proposal_writing & proposal_review→proposal; body_writing→essay; retrospective→review. Unknown→framework.
- [ ] **Step 2:** run, verify fail.
- [ ] **Step 3:** implement `FlowStatus`, `StatusForStage`, `StageFloor` (framework→proposal_forming, proposal→proposal_writing, essay→body_writing, review→retrospective, topic→topic_discussion).
- [ ] **Step 4:** run, pass.
- [ ] **Step 5:** commit `feat(studioflow): canonical status + stage collapse`.

### Task A2: Status registry (goal/tools/cards/surface/doc)

**Files:**
- Modify: `apps/api/internal/agent/studioflow.go`
- Test: `apps/api/internal/agent/studioflow_test.go`

**Interfaces:**
- Produces: `type StatusDef struct { Goal, SystemPrompt string; Tools []string; Cards []string; Surface OpenTool; Doc WritingDoc }`; `func StatusRegistry() map[FlowStatus]StatusDef`; `WritingDoc` string type (`""|proposal|essay`).
- Tool subsets: topic→[propose_question]; framework→[propose_note,summon_card]; proposal→[open_reading,request_review,finish_part,summon_card]; essay→[open_reading,request_review,finish_part,summon_card]; review→[].
- Each `SystemPrompt` is ≤6 lines: the 印记 identity one-liner + this status's goal + its tools' one-line contracts + 铁律 reminders (narrate 中文, 一次一个, 绝不代写). Copy the propose_note section rules verbatim from the current prompt for `framework`.

- [ ] **Step 1: Failing test** — registry integrity: every status present; every Surface valid (`OpenTool.IsValid`); every tool in `Tools` ∈ the known tool set (extend `knownOrchestratorTools` with `open_tool`? NO — `open_tool`/`set_status`/`generate_plan` are REMOVED from status tool sets; add `open_reading` and `finish_part` as new tool names); every card in `Cards` resolves in the card registry; `framework.Doc==""`, `proposal.Doc=="proposal"`, `essay.Doc=="essay"`.
- [ ] **Step 2:** run, fail.
- [ ] **Step 3:** implement the registry + prompts; add `knownStatusTools` map incl. new `open_reading`,`finish_part`; keep `propose_note,summon_card,request_review,propose_question,curate_reference`.
- [ ] **Step 4:** run, pass.
- [ ] **Step 5:** commit `feat(studioflow): per-status registry`.

### Task A3: Status-scoped request builder + tool filtering

**Files:**
- Modify: `apps/api/internal/agent/orchestrator.go`
- Test: `apps/api/internal/agent/orchestrator_test.go`

**Interfaces:**
- Produces: `func BuildStatusRequest(def StatusDef, spineProjection string, state StudioState, history []ChatTurn) gateway.ChatRequest` (uses `def.SystemPrompt` as system message; same history mapping as `buildOrchestratorRequest`); `func FilterToolsForStatus(dec OrchestratorDecision, def StatusDef) OrchestratorDecision` (drop any tool not in `def.Tools`).
- Consumes: `ParseOrchestratorOutput` (unchanged).

- [ ] **Step 1: Failing test** — a decision carrying `generate_plan` + `propose_note` filtered against the `framework` def keeps only `propose_note`; against `essay` def, a `propose_note` is dropped.
- [ ] **Step 2:** run, fail.
- [ ] **Step 3:** implement both funcs.
- [ ] **Step 4:** run, pass.
- [ ] **Step 5:** commit `feat(orchestrator): status-scoped request + tool filter`.

### Task A4: Fast chaperone resolver

**Files:**
- Modify: `apps/api/internal/gateway/keyresolver.go`, `apps/api/internal/store/deps.go` (or wherever `ChatResolver` is wired)
- Test: `apps/api/internal/gateway/keyresolver_test.go`

**Interfaces:**
- Produces: `func NewFastChaperoneResolver(...) Resolver` returning `deepseek-v4-flash` (model constant + pricing row already exist per memory); a `d.FastChatResolver(ctx)` seam alongside `d.ChatResolver`.

- [ ] **Step 1: Failing test** — `NewFastChaperoneResolver` resolves to model `deepseek-v4-flash`, tier chaperone.
- [ ] **Step 2:** run, fail.
- [ ] **Step 3:** implement resolver + wire `d.FastChatResolver`.
- [ ] **Step 4:** run, pass.
- [ ] **Step 5:** commit `feat(gateway): fast chaperone resolver`.

### Task A5: Deterministic flow router (transitions + nextStep)

**Files:**
- Modify: `apps/api/internal/api/coach.go` (generalize `reconcileStudioFunnel` → `advanceStudioFlow`)
- Create: `apps/api/internal/api/studioflow_router_test.go`

**Interfaces:**
- Produces: `func (a *API) advanceStudioFlow(ctx, projectID, state, effects) (StudioState, *nextStepDTO, bool)` returning reconciled state, an optional one-tap next step, and planGenerated. Rules: not started→no-op; framework: 4 dims filled & no plan → regeneratePlan + nextStep{label:"写研究提案",toStatus:proposal,surface:writing}; proposal: `finish_part` (effects) → nextStep{→essay}; essay: `finish_part` → nextStep{→review}; never advance stage backward (reuse `stageOrder`); never auto-advance (only emit nextStep).
- `type nextStepDTO struct { Label string; ToStatus string; Surface string }`.

- [ ] **Step 1: Failing test** (table): each state → expected (stage floor, nextStep, planGenerated). Uses a fake store or the testcontainer harness matching existing coach tests.
- [ ] **Step 2:** run, fail.
- [ ] **Step 3:** implement `advanceStudioFlow`; add `finish_part` to `orchestratorToolEffects` (bool `FinishPartRequested`).
- [ ] **Step 4:** run, pass.
- [ ] **Step 5:** commit `feat(coach): deterministic flow router + nextStep`.

### Task A6: Wire the status router into postCoach / postCoachStart

**Files:**
- Modify: `apps/api/internal/api/coach.go`, `apps/api/internal/api/projectcoach.go` (contract DTO), `packages/contracts/src/orchestrator.ts`
- Test: contract parity + existing live coach tests

**Interfaces:**
- `postCoach`: derive `status := StatusForStage(state.Stage)`, `def := StatusRegistry()[status]`, build request via `BuildStatusRequest`, call `a.d.FastChatResolver`, `FilterToolsForStatus`, apply tools, `advanceStudioFlow`, set `reply.NextStep`. Keep the note backstop (framework only). Delete the mega-prompt call path (`buildOrchestratorRequest` retained only if still used by opening; else remove).
- `orchestratorReplyDTO` gains `NextStep *nextStepDTO json:"nextStep"`; Zod `OrchestratorReply` gains `nextStep: NextStep.nullable()`.

- [ ] **Step 1: Failing test** — Zod parity test for `nextStep`; a coach turn in framework with 4 dims filled returns `nextStep.toStatus=="proposal"`.
- [ ] **Step 2:** run, fail.
- [ ] **Step 3:** implement the wiring; add sub-agent scopes (find_sources/reflection) unchanged.
- [ ] **Step 4:** run full `go test ./internal/api/ ./internal/agent/` (foreground); pass.
- [ ] **Step 5:** commit `feat(coach): dispatch per-status router on fast model`.

### Task A7: Real-frontend loop (Phase A acceptance) + deploy

- [x] Deployed `full` @ `5fd0bf9`. Ran the Playwright real-frontend journey on a FRESH project (History EE, printing press / Reformation): 立项 four points — **note capture reliable on the fast model, the 记进「目标」chip appeared + confirmed + field filled (the `note:null` bug is gone)**; plan auto-generated (8 items, content-specific); coach recognized the existing plan (NO regenerate offer); the **下一步 · 写研究提案 chip rendered**; tapping it advanced status → `proposal_writing`/`writing` and 印记 greeted the new phase. All on **deepseek-v4-flash**, restrained Chinese narrate, 0 canned fallbacks. **Phase A acceptance MET.**
- [x] Committed A1–A6b; Phase A live-verified.

**PHASE A COMPLETE (shipped + live-verified 2026-08-09).** Commits: A1 canonical status → A6b one-tap advance, deployed `full @ 5fd0bf9`. NEXT: Phase B (multi-doc writing) — the writing room in `proposal_writing` still shows the essay 大纲/片段/正文 surface (proposal & essay still share one buffer); gap 4 closes in B.

---

## Phase B — Multi-document writing (outline; plan in its own pass when A lands)

- Migration: add `doc_kind TEXT DEFAULT 'essay'` to `edit_buffer`, `draft_snapshot`; new `writing_finish(project_id, doc_kind, finished_at)`; backfill `project.writing_finished_at` → an `essay` finish row.
- REST `doc` param on `/buffer`, `/snapshots`, `/finish-writing`, review; active doc derived from status (proposal→proposal doc, essay→essay doc).
- Frontend: proposal doc = prose surface (textarea+markdown); essay doc keeps 大纲/片段/正文; per-doc 完成; the other doc preserved on switch.
- Acceptance: real-frontend run writes proposal AND essay as distinct preserved documents.

## Phase C — Polish (outline)

- Enforce per-status card subsets (summon_card gated by `def.Cards`).
- Cross-cutting reading-return (open_reading remembers origin status, returns on finalize).
- One-tap `nextStep` chip in the chat UI (renders `reply.nextStep`; tap → advance status + open surface + guiding turn).
- Remove dead mega-prompt code; sweep.

---

## Self-Review

- **Spec coverage:** status machine (A1/A2) ✓; per-status router (A2/A3/A6) ✓; deterministic transitions + nextStep (A5) ✓; fast model (A4/A6) ✓; multi-doc (Phase B) ✓; model strategy incl. eval-untouched (A4) ✓; phasing ✓; testing (per-task + A7 live) ✓.
- **Types:** `FlowStatus`, `StatusDef`, `WritingDoc`, `nextStepDTO`, `advanceStudioFlow`, `BuildStatusRequest`, `FilterToolsForStatus`, `NewFastChaperoneResolver` used consistently across tasks.
- **Placeholders:** none; each task has concrete files, interfaces, and test intent.
