# 「你的思维印记」Reveal UI — Design

> **Status:** design (awaiting review) · **Date:** 2026-06-29
> **Builds on:** the eval-model spec `2026-06-29-evaluation-model-design.md` (§6 trigger+async, §7 presentation),
> the shipped P4 async-eval backbone, and the shipped milestone auto-trigger (`2026-06-29-milestone-auto-trigger-design.md`).
> **Anchors:** 铁律 #2 不操纵（offer don't push; no addictive score）· #4 过程即数据.

## 1. Goal & scope

Surface the now-asynchronous, v2 (10-dimension) evaluation to the **student** as「你的思维印记」. Four
interdependent parts:
- **(a)** sync the stale TS contracts to the shipped backend (NA level, status, D10, the rollup tree);
- **(b)** rewrite the evaluator to poll the async lifecycle instead of awaiting the POST;
- **(c)** rebuild the reveal as a hierarchical **2 faces → 4 categories → 10 dims** drill-down;
- **(d)** add the quiet "offer, don't push" indicator that surfaces milestone-auto-triggered evals.

**In scope:** the student-facing reveal + indicator + the contract/evaluator plumbing it needs.
**Out of scope (separate future work):** teacher-facing levels/narrative views, class aggregates, the longitudinal
**trend** projection. (The reveal consumes a single per-task evaluation snapshot.)

### Design-source reconciliation (binding)

The binding `docs/design/思维印记_工作区.dc.html` (lines ~586–621) shows a **flat** 10-dim list and predates the
v2 cognitive model (its copy says "九个维度，跨五大分支"). The newer **eval-model spec §7** specifies a
hierarchical 2-faces → 4-categories → 10-dims presentation. Reconciliation:
- **Visual styling follows the `.dc.html` verbatim:** the gradient header, `过程小结 · 仅你可见`, the SOLO explainer
  line, the `#D98263` level pill, the 4-segment level bar, the `过程叙述` narrative card, the `回到任务` button.
- **Structure follows the eval-model spec:** the flat list becomes the deepest level of a faces→categories→dims
  drill-down.

This is the established "follow the HTML for look, the spec for structure" split, applied because the HTML predates
the model. No change to the `.dc.html` file.

## 2. Contract changes (`packages/contracts`) — prerequisite

The shipped Go DTO emits fields the TS contracts don't model yet. Sync them:

- **`src/evaluation.ts`:**
  - `SoloLevel` enum gains `"NA"` → `"L1" | "L2" | "L3" | "L4" | "NA"`.
  - `Evaluation` gains: `id: string`, `status: "queued" | "running" | "done" | "failed"`, `completed_at: string | null`.
    (Existing `task_id`, `scores`, `narrative`, `created_at` unchanged.) `DimScore` unchanged in shape
    (`{dim_id, level, note}`) but `level` now admits `NA`.
  - The internal-only `signals` / `trigger` / `trigger_milestone` columns are **not** in the DTO and stay out of the
    contract (confirmed against `toEvaluationDTO`).
- **`src/rubric.ts`:** add the missing **`D10` 协作编排** to `FULL_RUBRIC` (D1–D9 names already match v2). The
  reveal renders the dim **name** + the score's `note`; it does **not** render `anchors`, so the stale v1 anchor text
  on D1–D9 is not rewritten here (noted as carry-forward, not a display bug).
- **New `src/cognitive-model.ts`** — the single source for the display rollup tree (this mapping exists nowhere
  today). Shape:
  ```
  COGNITIVE_MODEL: Face[]
    Face   = { id, label, icon, categories: Category[] }
    Category = { id, label, dimIds: DimId[] }
  ```
  with the locked mapping from eval-model spec §2:
  ```
  🚀 生成式驾驭 (driving)   ├ 意图与编排 → [D1, D10]   └ 推理与论证 → [D4, D5, D7]
  🛡️ 批判式防护 (guarding)  ├ 信息素养   → [D2, D3]     └ AI 元认知与边界 → [D6, D8, D9]
  ```
  Plus a pure helper `assembleImprint(evaluation): AssembledImprint` that joins `evaluation.scores` (by `dim_id`)
  onto the tree, producing, per face/category, its dims with `{dimId, name, level|null, note}` and a coverage
  count `{scored, na}` (scored = level ∈ L1–L4; na = level NA **or** dim absent from scores). A dim present in the
  tree but absent from `scores` is treated as `NA` (defensive — a partial/old eval).

## 3. The reveal component (`EvalModal` → hierarchical)

Same modal chrome and styling. The body becomes a 2-level accordion built from `assembleImprint`:

- **Headline = the 2 faces, expanded by default**, each showing its **2 categories** and a **factual coverage chip**
  (`N 项已评 · M 未涉及`). **No computed level** on faces or categories — only a factual count (铁律 #2: no
  manufactured aggregate score; the cross-cutting qualitative read lives in the narrative).
- **Category → click to expand its dims.** Each dim row is the `.dc.html` row verbatim: dim name, `[L3 · 熟练]`
  pill (`#D98263`), 4-segment bar filled to the level, and the `note`.
- **N/A dims render neutral/greyed:** label + `本次未涉及`, **no bar fill**, counted in the face/category `未涉及`
  tally — **never** a low/zero bar (spec §7).
- **All-N/A eval** (a short task): reads as `进行中 · 本次未涉及` framing + the narrative — *in progress*, not broken
  or punished (spec §6 rationale).
- **Narrative card** (`过程叙述`) and `回到任务` unchanged.
- Level-word map: L1 萌芽 · L2 发展中 · L3 熟练 · L4 卓越 (the SOLO explainer line in the header keeps the
  `.dc.html` wording).

Decompose into focused units: `MindImprintReveal` (the modal shell + narrative + close), `FaceSection`,
`CategorySection`, `DimRow`, `CoverageChip` — each independently testable, assembled from the `AssembledImprint`
prop (no data fetching inside).

## 4. Async evaluator (`apps/web/src/agent/createEvaluator.ts`) — mandatory rewrite

Today it `await`s the POST and treats the response as final — **broken** against the 202 + async backend (the POST
returns a `queued` row, not a finished eval). New state machine:

- Phases: `idle → pending → done | error`.
- **Manual trigger** `run()`: `POST /evaluate` → returns a `queued` (or idempotent in-flight) row → enter `pending`
  → **poll** `GET /evaluation` every **2.5 s** until `status === "done"` (→ `done`, carry the evaluation) or
  `status === "failed"` (→ `error`); cap at **~40 attempts (~100 s)** then `error` (timeout). The flagship reasoner
  is slow, hence the generous ceiling.
- `EvalLoading` (existing) renders during `pending`; the existing error card + `重试` re-invokes `run()`.
- On `done`, the reveal opens and `lastSeenEvaluationAt` (§5) is set to the eval's `created_at`.

## 5. The quiet indicator (`MindImprintIndicator`, new) — offer, don't push

Makes the milestone auto-trigger visible without violating 铁律 #2:

- **Store marker** `lastSeenEvaluationAt` per task — the `created_at` of the eval the student last *opened*
  (set on any reveal open: manual or via the chip). Lives in the in-memory server-hydrated store (not localStorage,
  consistent with the existing store).
- **Background poll** `GET /evaluation` every **~30 s** while the workspace is mounted **and** the reveal is closed.
  If the latest `done` eval's `created_at` is newer than `lastSeenEvaluationAt` → show a calm bottom-corner chip
  `✨ 你的思维印记有新内容`.
- **Click the chip** → open the reveal, set `lastSeenEvaluationAt`, clear the chip.
- **Constraints (铁律 #2):** no badge/count, no auto-open, dismissable, no streak/score. `failed` evals never raise
  the chip. The poll pauses while the reveal is open and stops on unmount.
- Manual-trigger and the indicator share the one `lastSeenEvaluationAt` marker, so opening either path clears the
  other's "new" state.

## 6. Components & files

| Unit | File | Change |
|---|---|---|
| Eval contract | `packages/contracts/src/evaluation.ts` | NA level; `id`/`status`/`completed_at` on `Evaluation` |
| Rubric | `packages/contracts/src/rubric.ts` | add D10 协作编排 |
| Rollup tree | `packages/contracts/src/cognitive-model.ts` (new) | faces/categories/dimIds + `assembleImprint` |
| Evaluator | `apps/web/src/agent/createEvaluator.ts` | async poll state machine |
| API client | `apps/web/src/api/evaluate.ts` | surface `status`; (poll uses existing `getEvaluation`) |
| Reveal | `apps/web/src/workspace/EvalModal.tsx` (+ new sub-components) | hierarchical drill-down |
| Indicator | `apps/web/src/workspace/MindImprintIndicator.tsx` (new) | quiet chip |
| Wiring | `apps/web/src/workspace/WorkspaceView.tsx`, store | `lastSeenEvaluationAt`, background poll, mount indicator |

## 7. Testing

- **contracts:** `Evaluation` parses with `NA` + `status`; `assembleImprint` joins scores onto the tree correctly;
  **invariant test** — every D1–D10 maps to exactly one category and one face (no orphan, no duplicate, all 10
  present).
- **evaluator:** polling state machine via a fake api — `queued→running→done` reaches `done` with the evaluation;
  `failed` → `error`; never-`done` → timeout `error`. No real timers in tests (inject the poll/clock or drive
  iterations).
- **reveal:** renders the 2 faces + 4 categories with correct coverage chips; a category expands to its dims with
  pill+bar+note; an N/A dim renders neutral (`本次未涉及`, no bar) and counts toward `未涉及`; an all-N/A eval shows
  the `进行中` framing; the narrative renders.
- **indicator:** shows only when a `done` eval is newer than `lastSeenEvaluationAt` and the reveal is closed;
  hidden when not newer, when `failed`, or when the reveal is open; clicking opens the reveal and clears the chip.

## 8. Non-goals & carry-forward

- ❌ No teacher views / class aggregates / longitudinal trend (separate work; the trend projection is the eval-model
  spec's own fast-follow).
- ❌ No live-updating score, badges, streaks, counts, or auto-popup (铁律 #2).
- ❌ No rewrite of the stale v1 `anchors` text on D1–D9 in `rubric.ts` (not rendered by the reveal) — **carry-forward**:
  reconcile `rubric.ts` anchors + each card's `rubric_tags` against the v2 rubric at a later pass.
- **Carry-forward:** background-poll cadence (30 s) and evaluator poll ceiling (~100 s) are first cuts — revisit with
  real latency data, shared with the eval-model spec's cost/latency calibration note.
