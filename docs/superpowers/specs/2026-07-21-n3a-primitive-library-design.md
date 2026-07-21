# N3a · Complete the Interaction-Primitive Library — Design

Date: 2026-07-21
Slice: **N3a** (first sub-slice of **N3 · Interaction breadth**, per
`docs/2026-07-20-student-platform-remaining-work.md`)

---

## Goal

Take the C1 interaction-primitive library from **3/6 to 6/6** by building
`sort`, `scale`, and `matrix` as first-class primitives, each bound to one real
thinking-framework card.

Today only `annotate` (craap), `graph` (toulmin) and `compare` (sift) exist —
`PRIMITIVE_KINDS` names all six but the other three have no state schema, no
completion predicate, and no renderer. The roadmap calls the primitive library
"the interface foundation (C1)"; everything else queued in N3 depends on it
(the semantic card-moment "comparison→matrix" cannot exist before `matrix`
does, and the S2 perspective map needs a graph-node producer).

## Non-goals (explicitly deferred)

- **No LLM anchor seeding** for the three new primitives. Like `graph`, they
  seed deterministically from `params`; `studioturn.go`'s
  `spec.Primitive == "annotate" || "compare"` branch is **untouched**.
- **No classifier / semantic summon** (N3b). These cards surface through the
  existing structural `SurfaceCardCandidates` path and the equipment bar.
- **No S2 perspective *view*.** This slice mints `perspective` graph nodes;
  rendering them as a view is N3d.
- **No R-9 summing-up reveal, no student span-creation L2/L3** (rest of N3).
- **No migration, no new `Anchor` field, no `sqlc` change.**

---

## The seam: `Anchor` is already the universal carrier

Every C2 primitive persists into `card_instance.anchors` (jsonb) as
`Anchor[]`, and each primitive owns a `serialize.ts` pair converting between
its working state and `Anchor[]` (`primitives/graph/serialize.ts` is the
reference). All three new primitives fit the **existing** `Anchor` shape with
no contract change:

| primitive | `quote` | `dimension` | `answer` | `author` |
|---|---|---|---|---|
| `sort`   | the statement being classified | bucket id | 学生的分类理由 | `student` |
| `scale`  | the item being placed | ordered stop id | 学生的定位理由 | `student` |
| `matrix` | row label (视角名，即行的身份) | column id (维度) | 该格的内容 | `student` |

`MatrixRow.id` is a client-only React key; the **label** is the row's persisted
identity (it is what refeed reads, what `matrix_complete` groups on, and what
the `perspectives` effect names the node). Two rows sharing a label collapse
into one — writing the same perspective twice is a student error, not a case
to model.

Consequences that fall out for free:

- **Refeed works with a one-line change.** `agent.anchorSteps` already folds
  answered anchors into the 摘要回灌 payload, grouping by `dimension` and
  labelling each answer with `a.Question`. These primitives have no
  AI-authored question, so the label must fall back to `quote` (the sentence /
  the row) before `dimension`. One line, and it improves every primitive.
- **The assessor sees them.** Card completion already emits events at full
  weight; nothing primitive-specific is needed.
- **Rehydration works.** `Studio*Card` hosts seed from persisted anchors on
  mount, exactly as `StudioToulminCard` does.

---

## Cards

### 1. `fact-opinion-value` → `primitive: "sort"` (upgrade)

事实 / 观点 / 价值判断三桶归类。The card's legacy `steps` stay in the JSON
(the schema-driven fallback path is untouched); the C2 block is additive.

```json
"primitive": "sort",
"target_type": "project",
"params": {
  "buckets": [
    { "id": "事实",     "label": "可查证的事实", "hint": "能被独立核查的陈述" },
    { "id": "观点",     "label": "需论证的观点", "hint": "成立需要给出理由" },
    { "id": "价值判断", "label": "藏价值的判断", "hint": "带「应该/好坏」，藏着价值排序" }
  ],
  "min_items": 3
},
"completion": [
  { "kind": "items_bucketed", "tags": ["事实", "观点", "价值判断"], "min": 3 }
],
"consolidation": "reveal_framework_after_completion",
"intrusiveness_cap": "I2"
```

No `graph_effects` — classification is a reading move, it mints no claim.

### 2. `certainty-spectrum` → `primitive: "scale"` (upgrade)

The stops come verbatim from the card's own existing `spectrum` field:
个人猜测 → 有据推断 → 强证据 → 科学共识 → 逻辑必然. The 0457 mainline crux
(「变绿」→「更可持续」的过度断言) is exactly this card.

```json
"primitive": "scale",
"target_type": "project",
"params": {
  "buckets": [
    { "id": "个人猜测", "label": "个人猜测", "hint": "只有直觉，没有证据" },
    { "id": "有据推断", "label": "有据推断", "hint": "有证据，但推理仍可争议" },
    { "id": "强证据",   "label": "强证据",   "hint": "多个独立来源支撑" },
    { "id": "科学共识", "label": "科学共识", "hint": "领域内已达成共识" },
    { "id": "逻辑必然", "label": "逻辑必然", "hint": "由定义或推理必然为真" }
  ],
  "min_items": 1,
  "rewrite_prompt": "用与该位置相称的语气词重写结论句（可能 / 大概 / 很可能 / 几乎确定 / 必然）"
},
"completion": [
  { "kind": "items_bucketed", "tags": ["个人猜测","有据推断","强证据","科学共识","逻辑必然"], "min": 1 },
  { "kind": "field_written_by", "field": "rewrite", "author": "student" }
]
```

The 重写 half is **not** a scale placement — it is a separate student-written
field, so it rides the **existing** `field_written_by` predicate on an anchor
with `dimension: "rewrite"`. This is why `items_bucketed` must *ignore*
out-of-vocabulary anchors rather than reject them (see below).

### 3. NEW `perspective-matrix` · 视角对照矩阵 → `primitive: "matrix"`

Rows are **student-authored perspectives** — identifying whose view is missing
IS the thinking, so the card cannot pre-fill them. Columns are fixed.

```json
"id": "perspective-matrix",
"category": "溯源与多视角",
"name": "视角对照矩阵",
"primitive": "matrix",
"target_type": "project",
"params": {
  "cols": [
    { "id": "position",    "label": "立场主张", "q": "这个视角主张什么？用一句话说清。" },
    { "id": "grounds",     "label": "依据",     "q": "它凭什么这么主张？它最强的依据是什么？" },
    { "id": "blind_spot",  "label": "盲区",     "q": "这个视角看不见什么？它绕开了哪个事实？" }
  ],
  "min_items": 2
},
"completion": [ { "kind": "matrix_complete" } ],
"graph_effects": [ { "kind": "perspectives" } ]
```

**Why this card earns its place:** it is the direct producer for **表E
分析（多视角）** — the review-criteria dimension N2's gauge grades and that
N1's onboarding lets the student flag as her weak spot. When the gauge says
表E is thin, this is the tool that fixes it. Its `perspectives` graph effect
also lands the backend half of the deferred "S2 perspective map,
graph-node-backed" item.

`graph_node.type` is an open type (migration 0017 `CHECK type <> ''`), so
minting `perspective` nodes needs **no migration**. `projectStructure` reads
only toulmin slot types, so the new nodes are inert to every existing view.

---

## Backend changes (all additive)

### `cards.Params` (`apps/api/internal/cards/loader.go`)

```go
// Buckets is the sort/scale primitive's target vocabulary. For sort the
// order is presentational; for scale it is the axis order and is meaningful.
Buckets []Bucket `json:"buckets"`
// Cols is the matrix primitive's fixed column axis. Rows are student-authored.
Cols []Axis `json:"cols"`
// MinItems is the minimum number of sorted items / placed items / matrix
// rows the card requires. 0 means "no minimum".
MinItems int `json:"min_items"`
```

with `Bucket{ID, Label, Hint}` and `Axis{ID, Label, Q}`. Legacy cards parse
unchanged (all fields optional).

### `EvaluateCompletion` (`apps/api/internal/agent/card_completion.go`)

Two new closed-set predicates:

- **`items_bucketed`** — serves BOTH `sort` and `scale`. Counts anchors whose
  `dimension` is in `pred.Tags` **and** whose `answer` is non-empty; complete
  when the count ≥ `pred.Min`. Anchors with an out-of-vocabulary dimension are
  **ignored, not rejected** (that is how `certainty-spectrum`'s `rewrite`
  anchor coexists). On failure `missing` carries a single stable token
  (`"items"`), because no individual tag is "the" missing one.
- **`matrix_complete`** — groups anchors by `quote` (the row). A row counts
  only if every column in `spec.Params.Cols` has a non-empty answer for it.
  Complete when ≥ `spec.Params.MinItems` rows count. `missing` carries the
  first incomplete row's `quote`, or `"rows"` when there are too few rows.

`CompletionPredicate` gains `Min int \`json:"min"\``.

### `GraphEffects` (`apps/api/internal/agent/card_effects.go`)

New `perspectives` kind: one `MintNode` per **complete** row (same row rule as
`matrix_complete`), `Type: "perspective"`, `Author: "student"`, `Body:
{"text": <row label>, "cells": {<col id>: <answer>}}`. No edges — perspectives
are free-standing project nodes.

### `anchorSteps` (`apps/api/internal/agent/refeed.go`)

Label fallback becomes `question → quote → dimension` (today: `question →
dimension`). Without this, every sort/scale/matrix refeed answer would be
labelled with its bucket name and the sentence/row would be lost.

---

## Frontend changes

### Contracts (`packages/contracts/src/interactionPrimitive.ts`)

Three state schemas mirroring `GraphState`'s shape and style:

```ts
export const SortItem = z.object({
  id: z.string().min(1), text: z.string(), bucket: z.string(),
  reason: z.string(), author: Author,
});
export const SortState = z.object({ items: z.array(SortItem) });

export const ScaleItem = z.object({
  id: z.string().min(1), text: z.string(), stop: z.string(),
  reason: z.string(), author: Author,
});
export const ScaleState = z.object({ items: z.array(ScaleItem), rewrite: z.string() });

export const MatrixRow = z.object({
  id: z.string().min(1), label: z.string(),
  cells: z.record(z.string()), author: Author,
});
export const MatrixState = z.object({ rows: z.array(MatrixRow) });
```

`CardSpec.params` is already `z.record(z.unknown())` — the new params need no
contract change.

### Primitive modules (`apps/web/src/primitives/{sort,scale,matrix}/`)

Same module shape as `primitives/graph/`: `serialize.ts` (`anchorsTo*State` /
`*StateToAnchors`, pure, unit-tested both directions) + a presentational
component + `index.ts`.

- **`Sort`** — an add-row list: each row is `[句子原文] [桶选择 chips] [理由]`.
- **`Scale`** — a horizontal axis of the ordered stops; each item is a chip
  placed on a stop, with a reason; plus the `rewrite` textarea when
  `params.rewrite_prompt` is present.
- **`Matrix`** — a table: student-added rows × fixed `params.cols`, each cell a
  textarea, column headers carrying `col.q` as the guiding question.

### Studio hosts + fork

`StudioSortCard.tsx` / `StudioScaleCard.tsx` / `StudioMatrixCard.tsx`, each
mirroring `StudioToulminCard`: seed working state lazily from persisted
`anchors`, re-seed on the `anchors` prop, serialize on lock into
`{ ...newEnvelope(spec.id, ""), anchors }`. `CoachRail`'s primitive fork gains
three branches beside the existing three.

---

## Design rules held

- **RL-4 / 铁律 1 (AI 克制)** — every anchor these primitives write is
  `author: "student"`. The AI parameterizes the primitive (buckets, columns,
  guiding questions come from card config); it never fills a cell, never picks
  a bucket, never authors a perspective. No LLM call is added by this slice.
- **过程即数据** — a skipped or half-filled card still persists its anchors and
  its event; incompleteness is a signal, not an error state.
- **铁律 2** — no scores, no streaks, no completion celebration. Progress is
  reported as "还差 N 行" plainly.
- **AGENTS.md「新增卡 = 新增一份 JSON 配置，不改渲染器代码」** — this slice adds
  *primitives*, which is the sanctioned reason to touch renderers. Once
  `matrix` exists, the next matrix card is JSON only.

---

## Testing

- **Go:** loader parses new params (and legacy cards still parse);
  `items_bucketed` (below/at/above min, out-of-vocabulary ignored, empty answer
  not counted); `matrix_complete` (missing cell, too few rows, exact min);
  `perspectives` effect (node per complete row, incomplete row skipped, body
  shape); `anchorSteps` quote fallback; card-catalog test picks up the new card.
- **Contracts:** the three new Zod states round-trip; existing suites green.
- **Web:** each `serialize.ts` round-trips state→anchors→state; each primitive
  component renders and edits; each `Studio*Card` seeds from anchors and
  submits the expected envelope; `CoachRail` forks on each new primitive.

## Risks

- **Shared-schema fixture wave.** Adding fields to `Params`/`CompletionPredicate`
  is additive and safe, but any card-catalog snapshot test will see a 34th card.
  Run FULL Go packages and both JS suites, never `-run` subsets.
- **Canonical/mirror drift.** Card JSON is authored in `packages/contracts/cards/`
  and mirrored into `apps/api/internal/cards/specs/` by `make sync-cards`.
  Never hand-edit `specs/`.
