# Slice 4b-1 — Essay statement stage: doc-keyed 批注 + GuidedWritingCard + per-claim writing · Plan

> REQUIRED SUB-SKILL: superpowers:executing-plans (inline). Go suite is testcontainer-backed, foreground (~8 min). Steps use `- [ ]`.

**Goal:** Land the core of the essay statement stage: doc-key the 批注 so the essay has its own; a shared configurable `GuidedWritingCard` (guidance + auto-grow textarea + 我依然有问题/我写好了); the essay statement track (steps derived from the sub-questions) + an outline seeded from the RQ + sub-questions; the ready-gate flow (outline intro → 片段 focused writing); and per-claim writing where each claim is a card whose text is a snippet and 我写好了 produces essay 批注. (4b-2 adds 写作卡, synthesis/challenges/conclusion/结构, and the claim-edit warning; the 3a proposal-parts retrofit onto `GuidedWritingCard` is the final task here.)

**Architecture:** The 批注 reviewer/storage/endpoints become doc-aware (`?doc=`, `type='essay_annotation'`). `DeriveStatementSteps` mirrors `DeriveProposalSteps` (its own function — proposal ≠ paper). Claim cards store text as snippets (`section` = claim key) via the existing `useSnippets`. `GuidedWritingCard` is one React component both surfaces render. The essay statement is guided from the coach + a ready gate on the 大纲 page.

**Tech:** Go (`apps/api`, sqlc @v1.27.0, testcontainers foreground), Zod (`packages/contracts`), React+TS (`apps/web`).

## Global Constraints

- **铁律①/②:** AI never writes the body; 批注 diagnoses; 我写好了/advancing never blocked. Reviewer enforcement on free-text.
- **批注 flagship** (`EvalResolver`); guide-card gen + 我依然有问题 fast (`FastChatResolver`).
- **GuidedWritingCard is shared + configurable** — 3a and 4b use the SAME component (task 10 retrofits 3a).
- **A step yields 1..N cards.**
- **Deploy deferred** (whole doc first). Plan ends at green suites.

---

## Task 1: Doc-key the 批注 storage + reviewer

**Files:** `apps/api/internal/agent/draft_annotations.go` (doc-aware prompt), `apps/api/internal/agent/agentstore.go` (`ReplaceProposalAnnotations`→`ReplaceAnnotations(docKind)` + `ListAnnotations(docKind)` helper), `apps/api/internal/store/queries/intervention.sql` (essay list/delete), regen sqlc; test.

- [ ] **Step 1: Failing test** (agent) — `ReviewDraftAnnotations` with an essay `What`/`docKind` phrases the prompt for 论文正文 (assert via the captured user message that it doesn't say 研究提案 when essay). Store: `ReplaceAnnotations(projectID, "essay", rows)` writes `type='essay_annotation'`; `ListAnnotations(projectID, "essay")` reads them; proposal + essay don't collide.
- [ ] **Step 2: Run** `go test ./internal/agent/ -run 'TestReviewDraftAnnotations|TestReplaceAnnotations'` → FAIL.
- [ ] **Step 3: Implement:** add `DraftAnnotationInput.DocKind` (or `What string`) → the system prompt says 研究提案 (proposal) / 论文正文 (essay); `agentstore` gains `ReplaceAnnotations(projectID, docKind, rows)` + `ListAnnotations(projectID, docKind)` writing/reading `type=docKind+"_annotation"`; keep `ReplaceProposalAnnotations` as a thin wrapper (docKind="proposal") so existing callers/tests pass. Add `List/DeleteEssayAnnotations` sqlc (or a parameterized type filter).
- [ ] **Step 4: Run → PASS.**
- [ ] **Step 5: Commit** `feat(agent): doc-key 批注 (proposal/essay)`.

## Task 2: 批注 endpoints take `?doc=`

**Files:** `apps/api/internal/api/proposal_annotations.go` (`runDraftAnnotationReview` + GET/review take doc), `apps/api/internal/api/api.go` (essay routes or the same routes reading `?doc=`); test.

- [ ] **Step 1: Failing test** — `POST /proposal-annotations/review?doc=essay` reviews the ESSAY buffer + persists essay annotations; `GET /proposal-annotations?doc=essay` lists them; the proposal path (no `?doc=` / `?doc=proposal`) is unchanged.
- [ ] **Step 2: Run `go test ./internal/api/ -run TestProposalAnnotations` → FAIL.**
- [ ] **Step 3: Implement:** `runDraftAnnotationReview(ctx, projectID, doc, focus)` reads `GetEditBuffer(docKind=doc)`, calls `ReviewDraftAnnotations` with the doc, `ReplaceAnnotations(doc)`; handlers read `docKindParam(r)` (default proposal for back-compat). Reuse the existing routes with `?doc=` (no new routes needed).
- [ ] **Step 4: Run → PASS. Step 5: Commit** `feat(api): 批注 endpoints doc-scoped via ?doc=`.

## Task 3: `DeriveStatementSteps` + `EssayTrack.StatementStep` (pure Go)

**Files:** `apps/api/internal/agent/essay_statement.go` + test; extend `EssayTrack` (studiostate.go: `StatementStep int`, `StepGuides map[string]string`).

- [ ] **Step 1: Failing test** — `DeriveStatementSteps([sq a, sq b])` = `[outline, claim:a, claim:b, synthesis, challenges, conclusion, structure]` (7 steps); no sub-questions → the 6 fixed steps (outline + synthesis + challenges + conclusion + structure = 5? confirm: outline, synthesis, challenges, conclusion, structure = 5). Titles non-empty; `challenges` present.
- [ ] **Step 2: Run → FAIL. Step 3: Implement** `Step{Key,Title,Kind}` reuse from proposal_track.go; `DeriveStatementSteps` = outline + one `claim:<id>` per sub-question + synthesis + challenges + conclusion + structure. Add `EssayTrack.StatementStep`/`StepGuides`. **Step 4: PASS. Step 5: Commit** `feat(agent): DeriveStatementSteps + EssayTrack statement fields`.

## Task 4: `SeedEssayOutlineFromProposal`

**Files:** `apps/api/internal/agent/agentstore.go` (seed helper using outline store) + test.

- [ ] **Step 1: Failing test** — with a proposal objective + 2 sub-questions and an EMPTY outline, `SeedEssayOutlineFromProposal` writes outline rows: the RQ (depth 0) + each sub-question (depth 1). Idempotent (non-empty outline → no-op).
- [ ] **Step 2: Run → FAIL. Step 3: Implement** using `PutOutline`/the outline queries (GetOutline empty-check → insert RQ + sub-question rows). **Step 4: PASS. Step 5: Commit** `feat(store): seed essay outline from RQ + sub-questions`.

## Task 5: Essay guide-card generation (doc-aware)

**Files:** `apps/api/internal/agent/proposal_guide.go` (add an essay context to `GuideGenInput`, or a thin `GenerateEssayGuideStep`) + test.

- [ ] **Step 1: Failing test** — generating a `claim:*` guide card yields a card whose prompt (via captured user msg) walks 证据/分析/局限/衔接 for the claim; fenced/parse behavior as the proposal generator.
- [ ] **Step 2: Run → FAIL. Step 3: Implement** — extend `guideGenUserContent` with an essay branch (claim = the sub-question + "写这条论点的论述段落：证据/分析/局限/如何衔接下一条"), driven by a `Doc`/`Kind` on `GuideGenInput`. **Step 4: PASS. Step 5: Commit** `feat(agent): essay claim guide cards`.

## Task 6: Essay statement-track endpoints

**Files:** `apps/api/internal/api/essay_statement.go` (GET essay-statement, start, advance, review) + routes; test.

- [ ] **Step 1: Failing test** — enter statement (4a advance-stage sets EssayTrack.Stage=statement); `GET /essay-statement` returns the derived steps + current step + (when started) the current step's guide card (generated once, cached); `POST /start` (after the outline intro), `POST /advance {dir}`; `POST /essay-statement/review {stepKey}` produces essay 批注 (reuses the doc=essay review). Guide card generated once (call-count via stub).
- [ ] **Step 2: Run `go test ./internal/api/ -run TestEssayStatement` (foreground) → FAIL.**
- [ ] **Step 3: Implement** mirroring `proposal_track.go`'s handlers (loadTrackState reused; the essay track lives on `state.EssayTrack`). The claim steps' review = the doc=essay 批注 pipeline (Task 2). **Step 4: PASS. Step 5: Commit** `feat(api): essay statement-track endpoints`.

## Task 7: `GuidedWritingCard` shared component

**Files:** create `apps/web/src/workspace/blocks/GuidedWritingCard.tsx` + test.

**Interface:** `GuidedWritingCard({ guidance, example, colorTheme?, value, onChange, onStillStuck, onDone, doneLabel?, reviewing?, cardOffer?, onOfferCard? })` — a colored card: guidance + optional English example + an **auto-growing** textarea (grows with content) + 我依然有问题/我写好了 (+ optional 写作卡 chips).

- [ ] **Step 1: Failing test** — renders guidance + example; typing calls `onChange`; the textarea auto-grows (its `rows`/height increases with content — assert the auto-grow effect sets style height, or that no fixed `rows` cap); 我依然有问题/我写好了 fire; `cardOffer` renders offer chips calling `onOfferCard`.
- [ ] **Step 2: Run → FAIL. Step 3: Implement** — auto-grow via a ref + `scrollHeight` effect (or `field-sizing` where supported, with the JS fallback). Solid mk tokens. **Step 4: PASS. Step 5: Commit** `feat(web): GuidedWritingCard — shared configurable guided writing card`.

## Task 8: `useEssayStatement` hook + client

**Files:** `apps/web/src/api/essayStatement.ts`, `apps/web/src/workspace/blocks/useEssayStatement.ts` + test.

- [ ] **Step 1:** client (`getEssayStatement`/`startEssay`/`advanceEssay`/`reviewEssayPart`) + hook (mirrors `useProposalTrack`). Test the hook with injected deps (chooseStart/advance transitions). **Step 2: tsc + test PASS. Step 3: Commit** `feat(web): useEssayStatement hook + client`.

## Task 9: Wire the essay statement into the writing room

**Files:** `apps/web/src/workspace/blocks/WritingBlock.tsx` (essay branch) — the ready gate on 大纲 + the claim cards on 片段 using `GuidedWritingCard`; reuse the `annotationsVersion` reload for essay 批注 (ReferencePanel reads `?doc=essay`).

- [ ] **Step 1: Failing test** — for the essay in the statement stage: the 大纲 tab shows the plan-intro + a 准备好了吗？/开始写作 gate; after start, the 片段 tab shows a `GuidedWritingCard` for the current claim; 我写好了 calls the essay review + bumps the annotations reload.
- [ ] **Step 2: Run → FAIL. Step 3: Implement** — a lightweight `EssayGuide` in the essay branch reading `useEssayStatement`; the ready gate; the claim card writing to a snippet (`useSnippets`, section = claim key). ReferencePanel: read essay 批注 when the writing stage is the essay (`getProposalAnnotations` → doc-param, or the new list). **Step 4: PASS. Step 5: Commit** `feat(web): essay statement — ready gate + per-claim GuidedWritingCard + essay 批注`.

## Task 10: Retrofit 3a proposal parts onto `GuidedWritingCard`

**Files:** `apps/web/src/workspace/blocks/ProposalGuide.tsx` — the guide card becomes a `GuidedWritingCard` with its own auto-growing textarea per part; storage = a per-part snippet (`section` = part key), assembled into the proposal on finish/export.

- [ ] **Step 1:** decide + implement storage: each proposal part's text is a snippet (`section` = the part key); the proposal buffer/export assembles the ordered parts. Free mode keeps the single ProsePane buffer.
- [ ] **Step 2: Failing test** — the proposal guided part renders a `GuidedWritingCard` with an auto-grow textarea; editing saves the part; 我写好了 reviews that part (proposal 批注). **Step 3: Implement. Step 4: tsc + test PASS.**
- [ ] **Step 5: Commit** `feat(web): retrofit proposal parts onto GuidedWritingCard (§4 per-part frame)`.

> **Risk note:** Task 10 re-architects proposal-part storage (shared buffer → per-part snippets + assembly). It is sequenced LAST so the essay path proves the primitive first. If the assembly/export change balloons, split it into its own follow-up slice rather than destabilizing 4b-1.

## Task 11: Full suites (no deploy)

- [ ] contracts + web (`tsc` + test) green; full `go test ./...` green except the known `TestWeeklyReportForSeededClass` flake.
- [ ] Update memory (4b-1 done; 4b-2 next). **No deploy.**

**Acceptance:** the essay has doc-keyed 批注; entering the statement stage shows the outline plan-intro + ready gate → 片段 per-claim `GuidedWritingCard` writing with 我写好了→essay 批注; the outline is seeded from the RQ + sub-questions; the proposal parts are retrofitted onto the shared card (or that retrofit is cleanly split out if it balloons). §6-stage-2 outline/claim-writing behavior is in place; 写作卡 + synthesis/challenges/conclusion/结构 + claim-edit warning are 4b-2.

## Doc check (all-statuses.md §6 stage-2)
Cross-checked: outline seed ✅ (T4), per-claim writing one-by-one with guidance ✅ (T6/T9), evidence/analysis/limitation/linkage ✅ (T5 prompt), 我依然有问题/我写好了 ✅ (T7/T9), 批注 for the essay ✅ (T1/T2). 写作卡 + synthesis/challenges/conclusion/结构 = 4b-2 (flagged). GuidedWritingCard shared + step→cards + 3a retrofit per the user's steer (T7/T10).
