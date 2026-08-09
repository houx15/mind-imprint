# Slice 3b — The 批注 primitive (view-only layered colored annotations) · Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: use superpowers:executing-plans (inline) — this repo's Go suite is testcontainer-backed and runs foreground (~8 min); dispatching per-task subagents stalls on testcontainers. Steps use `- [ ]`.

**Goal:** A flagship reasoning reviewer reads the student's proposal and returns layered (paper/paragraph/sentence), colored (green/blue/red) 批注, persisted with no migration and rendered **view-only in the left reference panel** (sentence = blue/red underline; paragraph = blue/red comment; paper = green/blue/red summary) — never touching the editable draft. Plus the §4 finishers: select-to-send on the proposal surface, and the finish-proposal comment-first modal.

**Architecture:** New agent reviewer (`ReviewDraftAnnotations`, `EvalResolver`) mirrors `ReviewFramework`. 批注 persist as `type='proposal_annotation'` intervention rows via the additive `anchor` jsonb (no migration), delete-then-insert per doc. The 3a `proposal-track/review` endpoint is upgraded from a chat-bubble `ReviewVerdict` to the 批注 pipeline; a new whole-draft review + list endpoint are added. Frontend rebuilds `ReferencePanel`'s `AnnotationGroup` for the proposal, adds `ProsePane` select-to-send, and a finish comment-first modal.

**Tech Stack:** Go (`apps/api`, sqlc @v1.27.0, testcontainers foreground), Zod (`packages/contracts`), React+Vite+TS+Tailwind (`apps/web`, Vitest under `apps/web/test/`).

## Global Constraints

- **铁律① — 批注 is view-only, left panel; never edits/overlays the editable draft.** Notes are directions, never rewrites. Reviewer enforcement (`enforcement.BannedPhrasing`) runs on all free-text.
- **铁律② — non-blocking.** The finish comment-first modal is skippable; a `problem` 批注 never blocks finishing.
- **Sparse:** paragraph/sentence 批注 only where warranted; NOT every paragraph. Green only at paper level (no green underlines).
- **Never downgrade evaluation:** the 批注 reviewer runs on `a.d.EvalResolver` (flagship).
- **Client never calls the model;** all model calls server-side, metered `purpose="proposal_annotation"`.
- **No migration:** 批注 persist on the `intervention.anchor` jsonb (additive).
- **Deploy is deferred** (user: after the whole doc is done). This plan ends at green suites, not deploy.

---

## Task 1: `DraftAnnotation` contract

**Files:** Create `packages/contracts/src/proposalAnnotation.ts`; modify `packages/contracts/src/index.ts`; test `packages/contracts/test/proposalAnnotation.test.ts`.

**Interfaces (produces):**
```ts
export const AnnotationLevel = z.enum(["paper", "paragraph", "sentence"]);
export const AnnotationNature = z.enum(["good", "suggest", "problem"]);
export const DraftAnnotation = z.object({
  id: z.string(), level: AnnotationLevel, nature: AnnotationNature,
  quote: z.string(), locator: z.string(), note: z.string(),
});
```

- [ ] **Step 1: Failing test** — a paper-level `{level:"paper",nature:"good",quote:"",locator:"",note:"整体清晰"}` parses; a sentence-level with a quote parses; an invalid nature rejects.
- [ ] **Step 2: Run** `pnpm --filter @mind-imprint/contracts test -- proposalAnnotation` → FAIL.
- [ ] **Step 3: Implement** the schemas; export from `index.ts` (check no name clash with `annotation.ts`'s `Annotation` — these are new names).
- [ ] **Step 4: Run** the full contracts suite → PASS.
- [ ] **Step 5: Commit** `feat(contracts): DraftAnnotation (layered colored 批注)`.

## Task 2: `agent.ReviewDraftAnnotations` (flagship reviewer)

**Files:** Create `apps/api/internal/agent/draft_annotations.go` + `draft_annotations_test.go`.

**Interfaces (produces):**
- `type DraftAnnotationOut struct{ Level, Nature, Quote, Locator, Note string }`
- `type DraftAnnotationInput struct{ Title, Draft, Focus string }`
- `func ReviewDraftAnnotations(ctx, prov gateway.Provider, resolved gateway.Resolved, in DraftAnnotationInput) ([]DraftAnnotationOut, gateway.ChatUsage, error)`

- [ ] **Step 1: Failing test** — a stub provider (mirror `framework_review_test.go`'s `stubProviderText`) returning `{"annotations":[{"level":"paper","nature":"good","quote":"","locator":"","note":"结构清晰"},{"level":"sentence","nature":"problem","quote":"中国一定会成功","locator":"第2段","note":"这是断言，缺证据"}]}` yields 2 parsed items with the right fields; a fenced ```json``` reply parses; garbage → error; items with an empty `note` are dropped; the list is clamped to ≤10.
- [ ] **Step 2: Run** `cd apps/api && go test ./internal/agent/ -run TestReviewDraftAnnotations` → FAIL.
- [ ] **Step 3: Implement** mirroring `ReviewFramework`: system prompt per spec Pillar 2 (teacher; paper read w/ ≥1 good + main enhance; paragraph comments ONLY where warranted w/ locator; sentence marks only for specific sentences, quote verbatim; note is a direction never a rewrite; ≤10; Chinese; JSON only). `gateway.Collect`, 2-attempt retry, defensive parse (`extractJSONObject(stripFences(...))`), drop items with empty note, clamp 10. Run enforcement (`enforcement.BannedPhrasing`) over each note; a violation drops that item (do not fail the whole review — a partial teacher read is still useful). `MaxTokens: 3500`.
- [ ] **Step 4: Run → PASS.**
- [ ] **Step 5: Commit** `feat(agent): ReviewDraftAnnotations — flagship layered 批注 reviewer`.

## Task 3: Persistence — `proposal_annotation` rows (no migration)

**Files:** Create `apps/api/internal/store/queries/proposal_annotation.sql`; regen sqlc (`CGO_ENABLED=0 go tool sqlc generate` from `apps/api`); add a store helper in `apps/api/internal/agent/agentstore.go`; test in the api package (Task 5's test covers the round-trip).

**Interfaces (produces):**
- sqlc `ListProposalAnnotations(ctx, project_id) ([]Intervention, error)` — `SELECT * FROM intervention WHERE project_id=$1 AND type='proposal_annotation' ORDER BY created_at`.
- sqlc `DeleteProposalAnnotations(ctx, project_id)` — `DELETE FROM intervention WHERE project_id=$1 AND type='proposal_annotation'` (whole-doc replace; proposal is one doc here).
- Store method `ReplaceProposalAnnotations(ctx, projectID, rows []ProposalAnnotationRow) error` — in one tx: delete existing, insert each with `Type:"proposal_annotation"`, `Anchor = json({docKind:"proposal",level,nature,quote,locator})`, `Body=note`, `Level=nature`, `Criterion=level`.

- [ ] **Step 1:** Write the `.sql` (ListProposalAnnotations + DeleteProposalAnnotations). Note: whole-doc replace is fine — the proposal is a single doc; the docKind lives in the jsonb for forward-compat.
- [ ] **Step 2:** `cd apps/api && CGO_ENABLED=0 go tool sqlc generate`; confirm the two queries generated.
- [ ] **Step 3:** Add `ProposalAnnotationRow{Level,Nature,Quote,Locator,Note string}` + `ReplaceProposalAnnotations` (tx: `DeleteProposalAnnotations` then `InsertIntervention` per row) to `agentstore.go`.
- [ ] **Step 4:** `go build ./...` clean.
- [ ] **Step 5: Commit** `feat(store): proposal_annotation list/replace (no migration)`.

## Task 4: Endpoints — review (upgrade + whole-draft) + list

**Files:** Create `apps/api/internal/api/proposal_annotations.go`; modify `apps/api/internal/api/proposal_track.go` (`reviewProposalPart` → 批注 pipeline), `apps/api/internal/api/api.go` (routes); test `apps/api/internal/api/proposal_annotations_test.go`.

**Interfaces (produces):**
- `type draftAnnotationDTO struct{ ID, Level, Nature, Quote, Locator, Note string }`
- `func (a *API) reviewProposalAnnotations(w,r)` — POST `/proposal-annotations/review` (whole draft).
- `func (a *API) getProposalAnnotations(w,r)` — GET `/proposal-annotations` → `{annotations:[...]}`.
- `func (a *API) runDraftAnnotationReview(ctx, projectID, focus string) ([]draftAnnotationDTO, error)` — shared: read `edit_buffer` (proposal), guard nil `EvalResolver`/`Provider` (→ empty), run `ReviewDraftAnnotations`, meter, `ReplaceProposalAnnotations`, re-list, map to DTO (mint ids from the intervention row ids).
- Upgrade `reviewProposalPart` (proposal_track.go) to call `runDraftAnnotationReview` with `focus` = the current step's title/prompt, and return `{annotations:[...]}` instead of the `ReviewVerdict`.

- [ ] **Step 1: Failing test** — with a stub Provider returning the 批注 JSON + `EvalResolver: fakeEvalResolver()`: seed a proposal buffer (PUT /buffer?doc=proposal), POST `/proposal-annotations/review` → 200 with ≥1 annotation; GET `/proposal-annotations` returns the same set; a second review REPLACES (count doesn't accrete). Also: `proposal-track/review` now returns `{annotations:[...]}` (not `{ready,...}`).
- [ ] **Step 2: Run** `go test ./internal/api/ -run 'TestProposalAnnotations|TestProposalTrackReview'` (foreground) → FAIL.
- [ ] **Step 3: Implement** handlers + `runDraftAnnotationReview` + register 3 routes; upgrade `reviewProposalPart`.
- [ ] **Step 4: Run → PASS.**
- [ ] **Step 5: Commit** `feat(api): proposal 批注 review + list; upgrade 我写好了 to 批注`.

## Task 5: Web API client + `useProposalAnnotations`

**Files:** Create `apps/web/src/api/proposalAnnotations.ts`; modify `apps/web/src/api/proposalTrack.ts` (`reviewProposalPart` now returns `DraftAnnotation[]`); test covered by Task 6/7 component tests.

**Interfaces (produces):**
- `getProposalAnnotations(projectId): Promise<DraftAnnotation[]>`
- `reviewProposalAnnotations(projectId): Promise<DraftAnnotation[]>` (whole draft)
- `proposalTrack.ts` `reviewProposalPart(projectId, stepKey?)` → `Promise<DraftAnnotation[]>` (parse `z.array(DraftAnnotation)`).

- [ ] **Step 1:** Implement the clients (Zod-parse responses, fail-loud like `writing.ts`).
- [ ] **Step 2:** `pnpm --filter web tsc` — note this breaks 3a's `ProposalGuide.onDone`/`appendPartReview` (they consumed a `ReviewVerdict`); fix in Task 6.
- [ ] **Step 3: Commit** `feat(web): proposal 批注 API clients` (may defer commit until Task 6 compiles clean).

## Task 6: `ProposalGuide` "我写好了" → 批注 (not a chat bubble)

**Files:** Modify `apps/web/src/workspace/blocks/ProposalGuide.tsx` (`onDone`, remove `appendPartReview`'s chat-bubble use); the 批注 now surface in the left panel (Task 7), so `onDone` just triggers the review + a refresh signal + `next()`.

- [ ] **Step 1: Failing/adjust test** — update `ProposalGuide.test.tsx`: 我写好了 calls the review (returns `DraftAnnotation[]`) then `next()`; it no longer appends a chat bubble. Keep `appendPartReview` removed or repurposed. Add a callback `onAnnotationsRefreshed` the pane fires so the left panel re-fetches.
- [ ] **Step 2: Run → FAIL. Step 3: Implement** — `onDone` = `await reviewProposalPart(projectId, step.key)` → notify the reference panel to reload (a shared refresh; see Task 7) → `track.next()`. Remove the `appendPartReview` chat path. `pnpm tsc` clean.
- [ ] **Step 4: Run → PASS. Step 5: Commit** `feat(web): 我写好了 produces 批注 (left panel) not a chat bubble`.

## Task 7: `AnnotationGroup` rebuild — 总体 / 段落 / 句子 (colored, view-only)

**Files:** Modify `apps/web/src/workspace/blocks/ReferencePanel.tsx` (`AnnotationGroup`); create a small `apps/web/src/workspace/blocks/ProposalAnnotations.tsx` if the group grows large; test `apps/web/test/workspace/ProposalAnnotations.test.tsx`.

**Interfaces:** the group reads `getProposalAnnotations(projectId)` (proposal stage) and renders three sections by level. A shared reload signal (a bumped counter / callback threaded from WritingBlock) re-fetches after a review.

- [ ] **Step 1: Failing test** — render the group with a fixture `DraftAnnotation[]` (one each of paper/paragraph/sentence, natures good/suggest/problem); assert: paper `good` shows green text; paragraph shows the locator + note, no underline; sentence shows the quote WITH an underline class (`underline`) colored blue/red; sparse (empty list → the calm "批注会在印记体检…" line).
- [ ] **Step 2: Run → FAIL. Step 3: Implement** — the three-section render; nature→color map (good=green/`mk-success`, suggest=blue/`mk-info` or taro, problem=red/`mk-danger`); sentence quote as `<span className="underline decoration-... ">`. Use SOLID color tokens (the `mk-*/opacity` transparent gotcha). Wire the proposal path to read `getProposalAnnotations` (essay keeps the old `getAnnotations`).
- [ ] **Step 4: Run → PASS. Step 5: Commit** `feat(web): AI批注 group — 总体/段落/句子, colored, view-only`.

## Task 8: `ProsePane` select-to-send ("问印记")

**Files:** Modify `apps/web/src/workspace/blocks/ProsePane.tsx`; thread an `onSendToCoach(text)` prop from `ProposalGuidePane` (which has `useStudioChat`); test `apps/web/test/workspace/ProsePane.test.tsx` (extend).

- [ ] **Step 1: Failing test** — select text in the textarea (set selectionStart/End + fire mouseUp) → a "问印记" chip appears; clicking calls `onSendToCoach` with the selected text.
- [ ] **Step 2: Run → FAIL. Step 3: Implement** — mirror `DraftPane.onDraftMouseUp`/`selPop` (`WritingBlock.tsx:1102`): float a chip at the selection; click → `onSendToCoach(text)`; clear on edit. `ProposalGuidePane` passes `onSendToCoach={(t)=>void sendStudioTurn(t)}` (or pins via focusPart if reachable).
- [ ] **Step 4: Run → PASS. Step 5: Commit** `feat(web): proposal surface select-to-send (问印记)`.

## Task 9: Finish-proposal comment-first modal

**Files:** Modify `apps/web/src/workspace/blocks/WritingBlock.tsx` (the proposal branch's finish flow); test extend a WritingBlock/finish test if one exists, else a focused component test.

- [ ] **Step 1: Failing test** — for the proposal doc, opening 完成提案 when no 批注 exist shows the "先让印记看一遍 / 跳过，直接完成" choice; choosing 先让印记看一遍 calls the whole-draft review; 跳过 proceeds to `doFinishWriting`.
- [ ] **Step 2: Run → FAIL. Step 3: Implement** — intercept the proposal finish: fetch `getProposalAnnotations`; if empty, the modal offers review-or-skip; if a `problem` exists, note it (skippable). On skip/proceed → existing `doFinishWriting` (unchanged). Essay finish flow untouched.
- [ ] **Step 4: Run → PASS. Step 5: Commit** `feat(web): finish-proposal comment-first modal`.

## Task 10: Full suites (no deploy)

- [ ] `pnpm --filter @mind-imprint/contracts test` green; `pnpm --filter web tsc` clean + `pnpm --filter web test` green.
- [ ] Full `go test ./...` in `apps/api` green EXCEPT the known `TestWeeklyReportForSeededClass` flake (time-based, unrelated — do not chase).
- [ ] Commit any fixups. **Do NOT deploy** (user: deploy only after the whole doc is finished).
- [ ] Update the slice-3b memory with what shipped + that deploy is pending.

**Acceptance:** a flagship reviewer produces sparse layered colored 批注 (paper green/blue/red summary; paragraph blue/red comments where warranted, no underline; sentence blue/red underlined quotes), rendered view-only in the left panel; the editable draft is never touched; 我写好了 + a whole-draft AI check both produce 批注; select-to-send works on the proposal; the finish modal suggests a review first (skippable). No §4 proposal behavior remains unbuilt.

## Self-review (spec coverage)

- Pillar 1 (data) → T1,T3. Pillar 2 (reviewer + endpoints) → T2,T4. Pillar 3 (view-only left render) → T5,T6,T7. Pillar 4 (select-to-send + finish modal) → T8,T9. Model routing (flagship) → T2/T4 (`EvalResolver`).
- **Doc check (AGENTS.md rule):** cross-checked against `all-statuses.md §4` in the spec's "Doc consistency check" — every proposal-page behavior maps to a task; nothing left. The user's refinement (view-only left; sparse; sentence-underline only; green only at paper) is encoded in T2's prompt + T7's render.
- No placeholders; types consistent (`DraftAnnotation` reused across T1/T4/T5/T7; `DraftAnnotationOut` T2→T4).
