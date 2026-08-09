# Slice 4a — Essay research stage: 证据地图 (warren) + saturation gate · Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: superpowers:executing-plans (inline) — the Go suite is testcontainer-backed and runs foreground (~8 min); subagents stall on testcontainers. Steps use `- [ ]`.

**Goal:** Grow the warren graph into the essay's 证据地图: on 完成提案, seed the graph from the proposal (main RQ + sub-questions) and redirect the student to the reading room (research stage), let them triage/tag papers 支持/反驳 under each sub-question with a per-paper note in the sidebar, and have a flagship reviewer judge each sub-question's saturation — advisory — with a flexible advance into essay writing (per-claim or whole).

**Architecture:** The warren (`exploration_lead` + `question_edge`) IS the map; 4a seeds it from the proposal and extends `reference` with per-paper evidence fields (migration). A new `EssayTrack{Stage}` on StudioState (jsonb, no migration) carries the research→statement→submission position. The proposal→essay advance (`postCoachAdvance` / `nextStepFor`) is re-pointed to the reading room + seeds the map. A new flagship `ReviewEvidenceSaturation` mirrors `ReviewFramework`. Frontend extends `PaperMeta` (sidebar) with the note/nature/triage/archive, highlights the active sub-question, and redirects 完成提案 → reading.

**Tech Stack:** Go (`apps/api`, sqlc @v1.27.0, testcontainers foreground), Zod (`packages/contracts`), React+Vite+TS+Tailwind (`apps/web`).

## Global Constraints

- **铁律①/② — AI never fabricates a source/tag; saturation is advisory** (never blocks writing; the student taps). Reviewer enforcement on free-text.
- **The warren IS the 证据地图** — reuse `exploration_lead`/`question_edge`/`WarrenMap`/`ExplorationSidebar`; do NOT build a parallel evidence panel.
- **Per-paper note lives in the sidebar, never on the graph.**
- **Active sub-question = the one 印记 proposed OR the one the student is in;** search directions apply only to the active one.
- **Never downgrade evaluation** — saturation reviewer on `EvalResolver` (flagship); search directions on `FastChatResolver`.
- **Deploy is deferred** (whole doc first). Plan ends at green suites.

---

## Task 1: Migration — `reference` evidence fields + sqlc

**Files:** Create `apps/api/internal/store/migrations/00NN_reference_evidence.sql`; add queries to `apps/api/internal/store/queries/*.sql` (where reference queries live); regen sqlc.

- [ ] **Step 1:** Write the migration:
  ```sql
  ALTER TABLE reference
    ADD COLUMN triage             text NOT NULL DEFAULT '',
    ADD COLUMN evidence_nature    text NOT NULL DEFAULT '',
    ADD COLUMN evidence_argument  text NOT NULL DEFAULT '',
    ADD COLUMN evidence_finding   text NOT NULL DEFAULT '',
    ADD COLUMN evidence_placement text NOT NULL DEFAULT '',
    ADD COLUMN archived           boolean NOT NULL DEFAULT false;
  ```
  (Next migration number after 0061.)
- [ ] **Step 2:** Add queries: `SetReferenceEvidence(id, nature, argument, finding, placement)`, `SetReferenceTriage(id, triage)`, `SetReferenceArchived(id, archived)`. Confirm the existing reference SELECTs (`ListReferences`, `GetReference`) return `*` so the new columns flow through.
- [ ] **Step 3:** `cd apps/api && CGO_ENABLED=0 go tool sqlc generate`; `go build ./...` clean.
- [ ] **Step 4: Commit** `feat(store): reference evidence fields (triage/nature/note/archive)`.

## Task 2: Contracts — Reference evidence + EssayTrack + SubQuestionVerdict

**Files:** Modify the `Reference` schema (`packages/contracts/src/reading.ts` or `reference.ts` — locate it); create `packages/contracts/src/essayTrack.ts`; test.

**Interfaces (produces):**
```ts
// Reference gains (optional, back-compat): triage?: "" | "red" | "yellow"; evidenceNature?: "" | "support" | "challenge";
//   evidenceArgument?: string; evidenceFinding?: string; evidencePlacement?: string; archived?: boolean;
// essayTrack.ts:
export const EssayStage = z.enum(["research", "statement", "submission"]);
export const SubQuestionVerdict = z.object({ subQuestionId: z.string(), saturated: z.boolean(), why: z.string(), gaps: z.array(z.string()) });
```

- [ ] **Step 1: Failing test** — a Reference with the new fields parses; without them parses (optional); a `SubQuestionVerdict` parses.
- [ ] **Step 2: Run** contracts test → FAIL. **Step 3: Implement** (export from index). **Step 4: Run** full contracts suite → PASS.
- [ ] **Step 5: Commit** `feat(contracts): reference evidence + EssayStage + SubQuestionVerdict`.

## Task 3: `agent.EssayTrack{Stage}` on `StudioState`

**Files:** Modify `apps/api/internal/agent/studiostate.go`; test `studiostate_test.go`.

- [ ] **Step 1: Failing test** — round-trip a `StudioState` with `EssayTrack{Stage: EssayResearch}`; `DefaultStudioState().EssayTrack == nil`.
- [ ] **Step 2: FAIL. Step 3: Implement** — `EssayStage` consts + `EssayTrack struct{ Stage EssayStage }` + `StudioState.EssayTrack *EssayTrack json:"essayTrack,omitempty"`. **Step 4: PASS.**
- [ ] **Step 5: Commit** `feat(agent): StudioState.EssayTrack (research/statement/submission)`.

## Task 4: `SeedEvidenceMapFromProposal` + the 完成提案 → reading redirect

**Files:** Add a store helper in `agentstore.go`; modify `apps/api/internal/api/coach.go` (`postCoachAdvance` + `nextStepFor`), `apps/web/src/workspace/blocks/WritingBlock.tsx` (doFinishWriting essay redirect); test `coach_*_test.go`.

**Interfaces (produces):**
- `store.SeedEvidenceMapFromProposal(ctx, projectID, mainRQ string, subQuestions []agent.SubQuestion) error` — idempotent: create a root `exploration_lead` for the main RQ (origin `guide`, a stable marker) + one per sub-question, each with a **confirmed** `子问题` `question_edge` to the main RQ. Skip if already seeded (match by a seed marker / existing text).

- [ ] **Step 1: Failing test** — advancing proposal→essay (`postCoachAdvance {to_status:"essay"}` with proposal.objective + ProposalTrack.SubQuestions set) (a) sets `EssayTrack.Stage="research"`, (b) seeds the leads + `子问题` edges (assert via ListExplorationLeads / ListQuestionEdges), (c) sets `state.OpenTool = ToolReading`, and the returned nextStep/directive opens reading not writing. A second advance does NOT duplicate the seed.
- [ ] **Step 2: Run** `go test ./internal/api/ -run 'TestSeed|TestProposalToResearch'` (foreground) → FAIL.
- [ ] **Step 3: Implement** — in `postCoachAdvance` (and/or `advanceStudioFlow`), when entering `FlowEssay` from the proposal side with no EssayTrack yet: set `EssayTrack.Stage=EssayResearch`, run the seed (reading `proposal.objective` + `ProposalTrack.SubQuestions`), set `OpenTool=ToolReading`. Change `nextStepFor(FlowProposal, …, finishPart)` → `{Label:"去做研究", ToStatus:FlowEssay, Surface:ToolReading}`. In `WritingBlock.doFinishWriting`, the essay-doc advance already calls `advanceStatusTo("essay")`; ensure the surface it lands on is reading (driven by the returned directive `OpenTool`).
- [ ] **Step 4: Run → PASS.**
- [ ] **Step 5: Commit** `feat(coach): seed 证据地图 from proposal + redirect 完成提案→研究`.

## Task 5: `agent.ReviewEvidenceSaturation` (flagship)

**Files:** Create `apps/api/internal/agent/evidence_saturation.go` + test.

**Interfaces (produces):**
- `type SubQuestionVerdictOut struct{ Saturated bool; Why string; Gaps []string }`
- `type EvidenceReviewInput struct{ SubQuestion string; Papers []EvidencePaper }`; `type EvidencePaper struct{ Title, Nature, Argument, Finding string }`
- `func ReviewEvidenceSaturation(ctx, prov gateway.Provider, resolved gateway.Resolved, in EvidenceReviewInput) (SubQuestionVerdictOut, gateway.ChatUsage, error)`

- [ ] **Step 1: Failing test** — a stub returning `{"saturated":false,"why":"只有支持材料","gaps":["缺一个反例或替代解释","再多几篇强证据"]}` parses; fenced parses; garbage → error.
- [ ] **Step 2: FAIL. Step 3: Implement** mirroring `ReviewFramework`: system prompt judging the three criteria (spec Pillar 5: strong reliable support; at least one limitation/challenge/substitute/perspective; diminishing returns — new reading repeats what's gathered); return `{saturated, why, gaps[≤4]}`; Collect + 2-attempt retry + defensive parse; `MaxTokens: 3000`. **Step 4: PASS.**
- [ ] **Step 5: Commit** `feat(agent): ReviewEvidenceSaturation — per-sub-question flagship reviewer`.

## Task 6: Endpoints — per-paper evidence/triage/archive + saturation review

**Files:** Create `apps/api/internal/api/evidence_map.go`; modify the route table + (if a reference PATCH exists) extend it; test `evidence_map_test.go`.

**Interfaces (produces):**
- `PATCH /projects/{id}/references/{rid}/evidence {nature, argument, finding, placement}` → sets the fields (or extend the existing reference PATCH).
- `PATCH /projects/{id}/references/{rid}/triage {triage}`; `POST /projects/{id}/references/{rid}/archive`.
- `POST /projects/{id}/evidence-map/subquestions/{sqId}/review` → gathers the sub-question lead's papers (leads with `connectedReferenceId` under the sub-question) + their evidence fields, runs `ReviewEvidenceSaturation` via `EvalResolver`, meters `purpose="evidence_saturation"`, returns `SubQuestionVerdict`. Best-effort nil-resolver → `{saturated:true, why:"", gaps:[]}`.
- `GET /projects/{id}/evidence-map` → `{ mainQuestion, subQuestions:[{ id, text, papers:[{ id, title, nature, triage, hasNote }] }] }` projected from the seeded warren (OR document that the frontend projects this from the existing `getExploration` + references — pick one; the projection endpoint is cleaner for the saturation gather).

- [ ] **Step 1: Failing test** — seed a project (advance proposal→essay), attach a reference to a sub-question lead, PATCH its evidence + triage, GET evidence-map shows it; POST the sub-question review (stub EvalResolver) returns a verdict.
- [ ] **Step 2: Run** `go test ./internal/api/ -run TestEvidenceMap` (foreground) → FAIL.
- [ ] **Step 3: Implement** handlers + routes.
- [ ] **Step 4: Run → PASS.**
- [ ] **Step 5: Commit** `feat(api): evidence-map — per-paper evidence/triage/archive + saturation review`.

## Task 7: Web API client + Reference evidence fields

**Files:** Create `apps/web/src/api/evidenceMap.ts`; extend the reference client (`apps/web/src/api/reading.ts` or exploration.ts); ensure the `Reference` type carries the new fields.

- [ ] **Step 1:** Implement `getEvidenceMap`, `reviewSubQuestion`, `setReferenceEvidence`, `setReferenceTriage`, `archiveReference` (Zod-parse). `pnpm --filter web tsc` clean.
- [ ] **Step 2: Commit** `feat(web): evidence-map + reference evidence clients`.

## Task 8: `PaperMeta` sidebar — note + nature + triage + archive

**Files:** Modify `apps/web/src/workspace/blocks/exploration/ExplorationSidebar.tsx` (`PaperMeta`); wire the setters from `ExplorationView.tsx`; test `apps/web/test/workspace/exploration/ExplorationSidebar.test.tsx` (or create).

- [ ] **Step 1: Failing test** — render `PaperMeta` (or the sidebar) with a paper reference; assert a **证据笔记** section with editable key-argument / key-evidence / placement, a **支持/反驳** toggle, a **red/yellow triage** control, and an **archive** action; editing calls the injected setters.
- [ ] **Step 2: FAIL. Step 3: Implement** — the note section in `PaperMeta` (labeled, editable, debounced save via `setReferenceEvidence`); a nature toggle (支持/反驳); a triage picker (必读红/待定黄); an 归档 button (`archiveReference`). Solid color tokens (mk-success/danger/warning). Thread the setters through `ExplorationSidebarProps` + `ExplorationView`. **Step 4: PASS.**
- [ ] **Step 5: Commit** `feat(web): paper sidebar — 证据笔记 + 支持/反驳 + 红黄 triage + 归档`.

## Task 9: Active sub-question + current-task line + search directions

**Files:** Modify `ExplorationView.tsx` / the reading-room shell; reuse `ComposeExplorationGuide` via a scoped call; test the pure task-selection logic.

- [ ] **Step 1: Failing test** — a pure `activeSubQuestion(selectedNode, subQuestions)` helper: returns the selected sub-question when one is selected, else the first not-yet-saturated sub-question (the "start from sub-question 1" default). Unit-test it.
- [ ] **Step 2: FAIL. Step 3: Implement** — a "current research task" line in the reading room ("先从「<sub-question>」开始：找能支持或挑战它的材料"), a highlight on the active sub-question node, and search directions (2–3 keywords + why) scoped to the active sub-question (reuse the existing guide call). **Step 4: PASS.**
- [ ] **Step 5: Commit** `feat(web): one-task-at-a-time — active sub-question + current task + scoped search`.

## Task 10: Saturation review UI + flexible advance

**Files:** Modify the reading-room shell + the coach thread surfacing; `WorkspaceContainer`/`WritingBlock` for the statement advance; test.

- [ ] **Step 1: Failing test** — a "这条问题的证据够了吗？" action on a sub-question calls the review and surfaces the verdict (saturated + gaps); a "去写这条论点" / "研究做完了，开始写作" tap advances to the statement stage (sets EssayTrack.Stage=statement, opens the writing surface).
- [ ] **Step 2: FAIL. Step 3: Implement** — the per-sub-question review trigger + verdict display (loading line "印记正在核对这条线的证据…"); the flexible advance taps (per-claim → statement focused on that sub-question, or whole → statement). The statement stage itself is 4b — 4a just records research-ready + performs the stage flip + opens writing. **Step 4: PASS.**
- [ ] **Step 5: Commit** `feat(web): saturation review + flexible research→statement advance`.

## Task 11: Full suites (no deploy)

- [ ] Contracts `pnpm --filter @mind-imprint/contracts test` green; web `tsc` clean + `test` green.
- [ ] Full `go test ./...` in `apps/api` green EXCEPT the known `TestWeeklyReportForSeededClass` flake.
- [ ] Commit fixups. **Do NOT deploy.** Update the memory (4a done, deploy pending; 4b/4c next).

**Acceptance:** 完成提案 seeds the warren from the proposal + lands the student in the reading room (research stage); papers under each sub-question carry 支持/反驳 nature + a sidebar note (argument/finding/placement) + red/yellow triage + archive; 印记 works one sub-question at a time (the one the student is in) with scoped search directions; a flagship saturation review (support + challenge + diminishing returns) advises per sub-question; the student advances to writing per-claim or whole. No §5/§6-stage-1 behavior remains unbuilt (needs-resources reading-page box explicitly deferred to slice 5).

## Self-review (spec coverage)

- Pillar 1 (track + redirect) → T3,T4. Pillar 2 (seed) → T4. Pillar 3 (nature/note/triage/archive) → T1,T2,T6,T8. Pillar 4 (one-task + search) → T9. Pillar 5 (saturation + flexible advance) → T5,T6,T10.
- **Doc check (AGENTS.md rule):** cross-checked against `all-statuses.md §5 + §6 stage-1` in the spec's consistency table; every point maps to a task; needs-resources reading-page box explicitly deferred to slice 5. The user's clarifications (warren IS the map; sidebar note; triage red/yellow; archive; one-task = the active one; saturation 3 criteria; flexible writing) are each encoded (T4/T8/T9/T5/T10).
- Types consistent (`SubQuestionVerdict` T2→T5→T6; Reference evidence fields T1→T2→T7→T8).
