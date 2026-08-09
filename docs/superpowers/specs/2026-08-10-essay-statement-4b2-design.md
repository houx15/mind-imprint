# Slice 4b-2 — Essay Statement Stage: completion gate, claim revision, deep-link, tail polish

**Date:** 2026-08-10
**Status:** design (spec → plan → implementation)
**Behavior truth:** `docs/2026-08-09-all-statuses.md` §6 (writing paper · statement complete). This spec is checked against it below.

## Goal

Close the four remaining gaps in the essay **statement** stage left by 4b-1, so the
statement sub-machine is complete end-to-end:

1. **Statement → submission completion gate.** After the last statement step
   (论证结构), the student can finish the statement stage and enter 成文 (submission).
   Today the last step's 下一步 is disabled — a dead end. The backend
   `advance-stage {stage:"submission"}` already exists; this wires the UI gate.
2. **Claim revision (rephrase vs total-change warning).** §133–134: at the 大纲 step
   the student may modify the RQ and sub-questions "according to the materials/evidence."
   A **rephrase** keeps the claim's materials/writing valid; a **total change** of a
   sub-question invalidates them. 印记 must classify the edit and warn before it lands.
3. **Per-claim deep-link 4a → 4b.** The 4a research panel's 去写这条论点 must land the
   student **on that sub-question's claim step**, not just flip the stage.
4. **Tail step polish.** The 面对反方观点 step should surface the actual 反驳/张力 evidence
   the student collected in 4a (references with `evidenceNature="challenge"`), so the
   student argues against real counter-evidence (§140).

Out of scope (later slices): the submission stage internals — 引言/结论/成文/润色 loop
and finish→review (**4c**); topic→framework onboarding merge and reading-page UI (**5**).

## Cross-check against all-statuses.md §6

| all-statuses.md (statement complete) | This slice |
|---|---|
| "firstly go to the outline parts (initialized by main RQ + 2–4 subquestions, ask students to modify according to the materials/evidence)" | Deliverable 2 — the 大纲 step gets an editable sub-question list + revision classifier. |
| "then snippet part, guide students to write the argument paragraph for each claim one by one … evidence, analysis, limitation, how it correlates" | Already shipped in 4b-1 (claim steps + essay guide bodies). Deliverable 3 lands the student on the right claim from 4a. |
| "then guide students to write comparison or 综合 … conclusion … full 论证结构" | Already shipped in 4b-1 (synthesis/conclusion/structure steps). |
| "challenge if there is 反方观点, suggest students to discuss with these challenges" | Deliverable 4 — the 面对反方观点 step surfaces the 4a challenge-typed evidence. |
| "similar to proposal → each guided card has two buttons (AI support / AI comments)" | Already shipped in 4b-1 (GuidedWritingCard 我依然有问题 / 我写好了). |
| status change: "after final submission of paper, unlock review" | Deliverable 1 completes statement → submission (NOT statement → review). Submission → review is 4c. |

Note on 铁律②: modifying a sub-question, classifying it, and completing a stage are all
**deterministic/advisory** system steps on the student's own action — not manipulation and
not gated behind a confirm-to-proceed wall. The one confirm we add (total-change) is a
**data-safety warning** (materials/writing will no longer apply), not a friction gate;
it is skippable, and we never delete the student's writing on their behalf.

## Design

### D1 — Statement → submission completion gate

- `EssayStatementView`: on the **last** step (`structure`), replace the disabled 下一步 with
  a primary **「完成正文陈述，进入成文」** button → `onFinishStatement`.
- `EssayStatementPane.onFinishStatement` calls `advanceEssayStage(projectId, "submission")`
  (existing client), then triggers a studio-state reload so `essayStage` flips to
  `submission` and the statement pane unmounts (`WritingBlock` renders it only for
  `essayStage === "statement"`). In the submission stage the student sees the existing
  大纲/片段/正文 room and its 完成写作 button (4c will replace that flow with the
  引言/结论/成文/润色 loop). A new `onStageAdvanced?: () => void` prop threads the reload up
  through `WritingBlock` → `WorkspaceContainer` (which already reloads `studioState`).

### D2 — Claim revision: rephrase vs total-change

- **Reviewer (flagship, `EvalResolver` — 评估绝不降级).** New
  `agent.ClassifyClaimRevision(ctx, prov, resolved, in)` where
  `in = {Title, OldText, NewText, SiblingSubQuestions}`. Returns
  `ClaimRevisionVerdict{Kind: "rephrase"|"total_change", Why string}`. The prompt: a
  rephrase keeps the same underlying research target (materials/evidence still apply);
  a total change asks a different question (its materials and any writing no longer apply).
  Best-effort: on any error the caller degrades to `rephrase` (the safe, non-alarming
  default) so a model hiccup never blocks the student.
- **Endpoint** `POST /projects/{id}/essay-statement/revise-claim
  { subQuestionId, newText, confirm }`:
  - loads `ProposalTrack.SubQuestions`, finds the old text.
  - if the trimmed new text equals the old (or is empty) → `400`.
  - runs the classifier → verdict (metered as `claim_revision`).
  - `confirm=false` (default): return `{ verdict }` only — **nothing persisted**.
  - `confirm=true`: persist the new text on that sub-question (the claim steps derive
    from these), then return `{ verdict, applied: true }`. We do **not** delete the
    claim's snippet or evidence — the warning is advisory; the student owns their words.
- **Contract** `packages/contracts/src/essayTrack.ts`:
  `ClaimRevisionVerdict = z.object({ kind: z.enum(["rephrase","total_change"]), why: z.string() })`.
- **UI** — the 大纲 step (`EssayStatementView`, `step.key === "outline"`) gains an editable
  sub-question list (seeded from `step.subQuestions`). Per row: an input + a 保存 button
  (enabled only when the text changed). 保存 → `reviseClaim(confirm=false)` → shows the
  verdict inline:
  - `rephrase` → green "只是措辞调整，材料仍然适用" + auto-applies (calls `confirm=true`).
  - `total_change` → amber "这看起来是一个**全新**的子问题——它原来的材料和已经写的论述可能都不再适用。确定要换吗？" with a **确定替换** button → `reviseClaim(confirm=true)`; and a 取消 that reverts the input to the old text.
- After a successful apply, the pane reloads the statement track (`track.reload()`) so the
  claim steps reflect the new text.

### D3 — Per-claim deep-link 4a → 4b

- Extend `advance-stage` to accept an optional `claimId`:
  `POST /essay-track/advance-stage { stage:"statement", claimId? }`. When `claimId` is set
  and the target is `statement`, also set `EssayTrack.Started = true` and
  `EssayTrack.StatementStep = index of "claim:<claimId>"` in `DeriveStatementSteps` (clamped;
  if not found, leave at 0). Client `advanceEssayStage(projectId, stage, claimId?)` gains the
  optional third arg.
- `ResearchPanel`: the per-sub-question **去写这条论点** calls
  `advanceEssayStage(projectId, "statement", sq.id)`; the bottom **研究做完了，开始写作** stays
  `advanceEssayStage(projectId, "statement")` (no claim → lands on the ready gate / outline).

### D4 — 面对反方观点 step surfaces real counter-evidence

- The essay guide-card generator already branches on `challenges`. Add the student's
  collected challenge evidence to that step's `GuideGenInput` as a new field
  `ChallengeEvidence []string` (reference titles where `evidenceNature="challenge"`).
  `essayGuideBody` lists them under the challenges body ("你在研究阶段标记为『反驳/张力』的材料：…")
  so the fast model prompts the student to engage them by name.
- `generateEssayGuideCard` populates `ChallengeEvidence` (only for the `challenges` step) by
  reading the project's references and filtering `evidenceNature=="challenge"`.

## Testing

- **contracts:** `ClaimRevisionVerdict` parse (valid kinds; rejects unknown kind).
- **agent (no testcontainers):** `essayGuideBody` challenges branch includes the challenge
  titles; `guideGenUserContent` for `challenges` lists them. `ClassifyClaimRevision` parse
  helper (rephrase/total_change/degrade-to-rephrase on junk).
- **api (testcontainers):** `revise-claim` — 400 on unchanged/empty; `confirm=false` does not
  persist; `confirm=true` persists new sub-question text. `advance-stage` with `claimId` sets
  `Started` + `StatementStep` to that claim's index.
- **web (vitest):** `EssayStatementView` last step shows 完成正文陈述 → `onFinishStatement`;
  outline step renders the editable sub-question list, 保存 fires `onReviseClaim`, a
  `total_change` verdict shows 确定替换. `ResearchPanel` 去写这条论点 calls advance with the sq id.
