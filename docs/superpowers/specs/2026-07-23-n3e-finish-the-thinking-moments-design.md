# N3e · Finish the Thinking Moments — Design

> Closes the last two items of the N3 family (interaction breadth), both
> deliberately deferred out of N3d because each needs product/coach design
> rather than gate-plumbing: the **search-plan card** (S1) and the **R-9
> framework reveal**. After this slice N3 is fully DONE.

Date: 2026-07-23
Branch: `slice-n3e-thinking-moments`
Supersedes nothing; extends the writing-project surface built through N3a–N3f.

---

## 1. Motivation — two orphans

Both items are things the product *already gestures at* but does not deliver:

1. **The search plan is written and never questioned.** At S1 (`FramingView`)
   the student writes her 检索方向 (「打算去哪找证据」), persisted as a
   `preregistration` graph node (`api/framing.go:106-114`, body
   `{directions, origin}`). `PerspectivesView.tsx:172` literally promises
   「印记只追问你的检索方向，不替你找来源」 — a promise nothing keeps. There is no card,
   no summon, no coach 追问 for the retrieval plan.

2. **`framework_fill` is written for every card and read by nobody.** On card
   completion, `CompleteCard` (`agent/card_lifecycle.go:141`) writes
   `ConsolidationPayload(spec)` — `{strategy, framework, dimensions,
   tag_prompts}` — into `card_instance.framework_fill`. **All six existing
   cards** set `consolidation: "reveal_framework_after_completion"` (craap,
   sift, toulmin, fact-opinion-value, certainty-spectrum, perspective-matrix),
   so this payload is written on every completion. No projection reads it; no
   view renders it. The principle it exists for is already stated in
   `agent/course_coach.go:24`: 「工具卡的『框架』在用过之后才揭示，不在用之前——先做，再命名」.

Neither is a gate producer. S1 already walks (N3d). This slice is pure
additive enrichment — no gate is touched.

---

## 2. Component A — the search-plan card

### 2.1 What it is

A schema-driven tool card, parallel to `perspective-matrix`, that takes the
student's **own search directions as its fixed rows** and has her interrogate
each one. The card's job is to be *genuinely useful for her real retrieval
problem* — its columns carry substantive guiding questions (not a hollow
「批判这条」), so working the card actually improves her plan. 铁律 1: the card
guides with questions and framing, never answers, never 替她定论.

**Where the guidance actually reaches her (verified against `Matrix.tsx`).**
The matrix host (`StudioMatrixCard`) renders only `spec.params` — for each
row it draws every column's `label` (persistent, above the cell) and `q` (the
cell textarea's placeholder). So the in-card guidance IS the three column
questions; they must each be a pointed, self-contained question because they
are the whole of what she sees. The `steps[].methodology` block below is
**metadata only** (parity with `perspective-matrix`, which also carries it) —
the matrix host does not render `steps` *during filling*. (Component B does
read `steps[0].methodology.why` — but only for the post-completion reveal in
the 工具卡 tab, §3.2, never in the card itself; 先做再命名.)

### 2.2 Card JSON

New file `packages/contracts/cards/search-plan.json`, mirrored into
`apps/api/internal/cards/specs/` by `cd apps/api && make sync-cards` (never
hand-edited). Built on the existing `matrix` primitive; the only renderer
change it needs is the row-noun parameterization in §2.3a (no new primitive,
no new host).

```jsonc
{
  "id": "search-plan",
  "category": "溯源与多视角",
  "name": "检索方向审视",
  "name_en": "Search-Plan Audit",
  "purpose": "在真正去查之前，逐条想清楚每个检索方向会给你哪一类证据、又系统性地漏掉什么",
  "trigger_condition": "学生已经写下检索计划（打算去哪找证据），但没有想过每个方向的偏差与盲区",
  "trigger_keywords": ["去哪找", "检索", "找证据", "搜", "查资料"],
  "priority": "P1",
  "disclosure_tier": "tier-1",
  "age_band": ["MYP", "DP"],
  "interaction_type": "画布导图卡",
  "rubric_dims": ["D3"],
  "related": ["sift", "craap", "perspective-matrix"],
  "body_status": "full",
  "rubric_tags": ["D3"],
  "primitive": "matrix",
  "target_type": "project",
  "params": {
    "cols": [
      { "id": "evidence_type", "label": "会给什么", "q": "这个方向最可能给你哪一类证据？" },
      { "id": "blind_spot",    "label": "看不见什么", "q": "它系统性地看不见什么？绕开了谁、绕开了哪种反面情况？" },
      { "id": "disconfirm",    "label": "能否证伪", "q": "如果你的结论其实是错的，这个方向找得到反证吗，还是只会印证你？" }
    ],
    "min_items": 1,
    "row_prompt": "你检索计划里的一条方向",
    "row_noun": "检索方向"
  },
  "completion": [{ "kind": "matrix_complete" }],
  "graph_effects": [],
  "consolidation": "reveal_framework_after_completion",
  "intrusiveness_cap": "I3",
  "steps": [
    {
      "key": "rows",
      "title": "逐条审视检索方向",
      "disclose": "always",
      "methodology": {
        "why": "多数「找错证据」不是懒，而是**只往会同意你的地方找**——检索计划里每个方向都带着它自己的偏差和盲区，先看清，再去查，比查完一堆再发现全是一边的声音要省力得多。",
        "how": "对每条方向问三件事：它会给你**哪一类**证据、它**看不见**什么、以及它**能不能证伪**你——如果一个方向永远只会印证你，它就不是在帮你查证，是在帮你说服自己。",
        "when": "在你按这份计划真正开始检索之前。"
      },
      "fields": [
        { "type": "repeatable_group", "key": "directions", "label": "每行一条检索方向", "item_fields": [
          { "type": "text",     "key": "direction",     "label": "检索方向" },
          { "type": "textarea", "key": "evidence_type", "label": "会给你哪一类证据？" },
          { "type": "textarea", "key": "blind_spot",    "label": "看不见什么？" },
          { "type": "textarea", "key": "disconfirm",    "label": "能不能证伪你？" }
        ] }
      ]
    }
  ]
}
```

> The card **id / name / column questions** are the easiest things to tune
> later; the shape is what matters. `min_items: 1` because the card is an
> offer over *her* plan — if she opened it, critiquing even one direction is a
> real thinking move; a higher floor would wall her (铁律 2). `rubric_dims`
> D3 (证据与信源意识) — the retrieval-bias move is a sourcing skill.

### 2.3 Two contained web changes: the row-noun, and the seed

Reusing `matrix` for a *second* card surfaces that the primitive is not yet as
vocabulary-free as it claims. Both changes below are small and principled; the
matrix stays the only matrix host (no new primitive).

**(a) Parameterize the hardcoded row-noun (required, ~3 strings).** `Matrix.tsx`
hardcodes 视角 in its add button (`添加一个视角`), its progress line
(`已完成 N 个视角 · 还差 M 个`), and a remove aria-label (`删除第 N 个视角`) — it
would show 视角 on a 检索方向 card. The primitive already claims 「it does not
know the words 立场主张/依据/盲区」 for *columns*; this extends that honesty to
the row noun. Add an optional row-noun to `params` (e.g. `row_noun`, default
「视角」 so `perspective-matrix` is unchanged; `search-plan.json` sets
「检索方向」) and thread it through those three strings. This is a targeted
improvement to code we are working in, not a new interaction — the exact carve
AGENTS.md allows.

**(b) Seed the rows from her plan (server-only, no `Matrix.tsx` change).** The
rows are her `preregistration` directions, **seeded, not re-typed** — this is
what makes the card meet her where she is rather than adding busywork.
Mechanism, in priority order:

**Primary — summon-time anchor seed (server-only, no renderer/contract
change).** When the card is surfaced, seed the fresh `card_instance.anchors`
with one anchor per direction:

```
{ quote: <direction text>, dimension: "evidence_type", answer: "", author: "student" }
```

This works with the existing serializer unchanged (verified against
`apps/web/src/primitives/matrix/serialize.ts`):

- `anchorsToMatrixState` builds a row for any anchor with a non-blank `quote`
  and a `dimension` that is a declared column id (`evidence_type` qualifies),
  so each seed becomes a **labelled row with empty cells** — exactly a
  pre-populated, un-filled row.
- The server completion predicate `firstIncompleteMatrixRow`
  (`agent/card_completion.go:191`) **skips anchors whose `answer` is empty**,
  so a seed never falsely counts toward `min_items` — she must actually fill
  cells for a row to complete.
- On lock, `matrixStateToAnchors` drops empty cells, so a seeded-but-unfilled
  row simply carries no anchor — no orphan rows persisted.

`Matrix.tsx`'s comment says 「the card cannot pre-fill rows」 — that is a
*design statement about perspective-matrix* (there, naming whose perspective
is missing IS the thinking, so pre-filling would do the work for her). It is
not a technical block: the seeded rows here are her *own* prior writing, and
the row label stays an editable input she can revise. Reusing the same host
for a card that legitimately pre-fills is fine; note the comment so a future
reader does not mistake it for an invariant.

**Task-1 spike (gates the rest of the slice).** Prove end-to-end that a
freshly *surfaced* (not reloaded) search-plan card delivers its seeded anchors
to `StudioMatrixCard`'s `anchors` prop and renders the rows on first open. The
host comment says `anchors` is "non-empty only on a reload"; the spike must
confirm the surface→open path carries the seeded anchors on first open too. If
it does not, fall back to 2.3-fallback.

**Fallback — a scoped `seed_rows_from` matrix param (crosses the renderer
boundary).** Only if the spike fails: add an optional
`params.seed_rows_from` to the matrix primitive that renders a fixed,
projection-supplied list of row labels she fills. This is a genuine (small,
reusable) renderer extension — "pre-populated rows from the student's own prior
work" — the kind AGENTS.md's 「除非真需要全新交互才加原语」 carves out. It touches
`Matrix.tsx`, the matrix contract, the host, and the projection (to supply the
directions). Preferred to be avoided; documented here so the plan can pivot
without re-brainstorming.

### 2.4 Summon rule

A new project-scoped branch in `agent.SurfaceCardCandidates`
(`agent/classifier.go`), structural — **no LLM call** — modelled exactly on
the `perspective-matrix` branch:

- **Fires when** a `preregistration` graph node exists (she has written a
  search plan), **no source has been evaluated yet** (`!anyEvaluated` — the
  same graph-wide fact perspective-matrix/toulmin read), **and** no
  `search-plan` card_instance has ever been seen.
- **Why the `!anyEvaluated` upper bound** (refinement surfaced in
  implementation, Task 4): the card's own `when` is 「在你按这份计划真正开始检索
  之前」 — it is a *pre-sourcing* offer. Without the bound it stays eligible
  from S1 forever and, once the per-material CRAAP offers are exhausted,
  becomes `cands[0]` at S4 and **hijacks the toulmin surface** (this is exactly
  what broke `TestWalk_S0ToS6_FreshProject`). Bounding on `!anyEvaluated`
  retires it the moment she completes her first source evaluation. A useful
  invariant falls out: **search-plan (`!anyEvaluated`) and
  perspective-matrix/toulmin (`anyEvaluated`) are now mutually exclusive** — no
  priority conflict between them is possible, so placement between them is moot.
- **Suppressed** on any-status instance — offer-never-a-wall (铁律 2): once
  she has said no, we never ask again. (Same rule as perspective-matrix /
  toulmin: a project-scoped card mints no per-material edge, so suppression
  keys on instance existence, not the per-material `evaluated` bookkeeping.)
- **Placement** — before the `perspective-matrix` branch (harmless given the
  mutual exclusion above; kept for readability, S1 upstream of S3).

### 2.5 What it does NOT do

- **No gate coupling.** S1's `frame_question` already walks via N3d; this
  card is not in any gate's item list, and the producerless-gate guard
  (`skills/producers_test.go`) is unaffected (no new gate item).
- **No new graph-node type** (`graph_effects: []`). The completed
  card_instance's critique anchors + event_trace *are* the process record
  (铁律 4 — her retrieval reasoning becomes assessable data via the existing
  card_instance the assessor already reads). YAGNI: no standalone node type
  until something needs to read one. (Note: because it mints nothing, the
  search-plan card has no station-view output surface — it lives entirely as
  the card_instance; that is fine, it is an enrichment, not a gate producer.)
- **No new metering surface.** Surfacing is structural (classifier, no
  model). Filling is client-side. The N3b post-completion 摘要回灌 refeed may
  ask one coach question afterward — already metered by the existing turn
  loop. Nothing new to meter.

---

## 3. Component B — R-9 framework reveal (in the 成长报告 工具卡 tab)

The point of R-9 is 先做再命名: after a student completes a card, name the
transferable thinking move she just performed. That naming is currently
computed and dropped on the floor.

### 3.1 Why not the studio / process tree (the home changed after grounding)

The design first assumed two studio homes — a panel on the completed card and
a 过程树 tag. **Reading the live code, neither exists:** `CompletedCard.tsx`
is wired only into a dev harness (`dev/Harness.tsx`), not the studio; when a
card completes in the workspace its sheet simply unmounts (`ActiveCard` → nil),
so there is no per-completed-card panel to attach to; and the 过程树 is drawn
in `dc.html` but was never built as a live pane (the 「钉到过程树」 button pins
to minted graph nodes that surface inside station views, not to a visible
tree). Building either would balloon this slice well past "finish the thinking
moments." **Decision (user-approved):** deliver the reveal in the one surface
that already lists completed cards as cards — the 成长报告 → 工具卡 tab.

### 3.2 The reveal is spec-derived — pure web, no server change

`ConsolidationPayload(spec)` is a **pure function of the CardSpec**:
`framework = spec.Name`, `strategy = spec.Consolidation`, plus the card's
dimensions. The client already holds the full CardSpec in `CARD_REGISTRY`,
including `consolidation` (optional string) and `steps[].methodology`
(`{why, how, when}` — richer than `framework_fill`'s bare dimensions). So the
reveal needs **no projection field, no DTO, no Zod change, no migration** —
`ToolkitCards.tsx` reads it straight from the spec of each collected card.

- **Gate:** show the reveal only for a card whose spec has `consolidation`
  set. (All six current cards + `search-plan` do; a future card without it
  simply shows no reveal.) The 工具卡 tab already lists only *completed*
  cards (`GET /growth/cards` is completion-derived), so 先做再命名 holds by
  construction — there is no pre-completion path to this surface.
- **Content:** the framework **name** (`spec.name`) and a one-line naming of
  the move — the `why` from `spec.steps[0].methodology` (the card already
  authored it). No LLM, no badge, no score, no celebration (铁律 2) — a plain
  「你用过的思路：<名>」 line on each collected card.

### 3.3 Blast radius, and `framework_fill`'s fate

This lights up **all six existing cards** plus `search-plan` — the reveal was
designed into every card (`consolidation` set) and silently dropped since the
runtime shipped. Because we render from the spec (which carries *more* than the
stored jsonb), **`framework_fill` stays written-but-unread** after this slice.
That is deliberate: it remains a frozen completion snapshot, useful only if a
future record-fidelity surface (e.g. an actual 过程树 pane) wants the framework
*as it was at completion time* rather than the current spec. If no such surface
ever materializes, a later cleanup may drop the column; this slice does not,
to avoid a migration for zero functional gain.

---

## 4. What this slice is NOT

- **No migration.** `framework_fill` exists since 0016; `preregistration`
  nodes since N3d; no new column, no new table.
- **No NEW primitive, no new matrix host.** The one required renderer change
  is parameterizing the matrix row-noun (§2.3a) — a ~3-string vocabulary fix
  the primitive should already have had, back-compatible via a default. Row
  seeding (§2.3b) is server-only. The `seed_rows_from` fallback (§2.3b) is the
  only path that would touch matrix *rendering logic*, and only if the task-1
  spike forces it.
- **No gate touched, no producerless-gate guard change.**
- **No card JSON hand-edited in `specs/`** — authored in
  `packages/contracts/cards/`, mirrored by `make sync-cards`.
- **No LLM call added** on either component.
- **Component B touches no server code** — the reveal is read from
  `CARD_REGISTRY` in `ToolkitCards.tsx`. No projection, DTO, Zod, or migration.
  `framework_fill` is intentionally left written-but-unread (§3.3).

---

## 5. Data flow

**Search-plan card:**
```
S1 FramingView → preregistration node {directions}
  → SurfaceCardCandidates sees the node, no prior instance → surface_card(search-plan)
  → SurfaceCard seeds one blank-cell anchor per direction into card_instance.anchors
  → student opens (confirms), StudioMatrixCard rehydrates seeded rows, fills critique cells
  → lock → CompleteCard: matrix_complete over filled rows → writes framework_fill
  → (Component B lights the reveal)
```

**R-9 reveal (pure web — no server change):**
```
成长报告 → 工具卡 tab → GET /growth/cards lists completed cards (already true today)
  → ToolkitCards.tsx resolves each card's spec from CARD_REGISTRY
  → if spec.consolidation set: render 「你用过的思路：<spec.name>」 + spec.steps[0].methodology.why
  (framework_fill on the server stays written-but-unread — §3.3)
```

---

## 6. Testing

**Go (full packages, `-p 1`, never `-run` subsets — card/summon change):**
- `SurfaceCardCandidates`: fires `search-plan` when a `preregistration` node
  exists and no instance seen; suppressed once an instance (any status incl.
  skipped) exists; ordered before `perspective-matrix`.
- Seed: surfacing `search-plan` writes one blank-cell anchor per direction;
  the seed anchors are not counted complete by `firstIncompleteMatrixRow`
  until cells are filled.
- Producerless-gate guard still green (proves no gate item was added).

**Web (`npm test` + `npx tsc --noEmit`):**
- Component A: seeded rows render on first open of a surfaced `search-plan`
  card; the row-noun param renders 检索方向 (not 视角) in the add button /
  progress line / remove label; `perspective-matrix` still renders 视角
  (default preserved).
- Component B: `ToolkitCards.tsx` renders the framework reveal (name +
  methodology `why`) for a collected card whose spec has `consolidation`, and
  renders **no** reveal for a card whose spec lacks it.

**Contracts (`packages/contracts`, tests in `test/`):**
- `search-plan.json` validates against the card schema; canonical↔mirror
  copies byte-identical (`make sync-cards` reproduces).
- The new `row_noun` param is optional and back-compatible (matrix cards
  without it still validate).

---

## 7. Known limits (deliberate, carried forward)

- **The reveal is a record, not an in-the-moment beat.** It appears in the
  成长报告 工具卡 tab, not in the workspace at the instant she finishes a card
  — because no live studio completed-card panel or 过程树 pane exists (§3.1).
  A true in-studio completion moment, or the 过程树 itself, is separate work
  for a later slice. Retroactive by nature: every already-completed card shows
  its reveal the next time the tab renders, free.
- **Seeded rows follow the plan as it was when the card surfaced.** If she
  edits her search plan after opening the card, the seeded rows do not
  re-sync (the card is a snapshot of the plan at summon). Acceptable — the
  card is an offer over the plan she had; re-summon does not re-fire (offer
  never a wall). Noted, not fixed.
- **`disconfirm` is a self-report, not a check.** The card asks whether a
  direction can falsify her; it cannot verify her answer. That is by design
  (铁律 1 — the card makes her think, the assessor judges quality, the gate
  never does).
- **No standalone graph node for the critique** (§2.5) — if a later slice
  wants the retrieval critique to feed a gate or a distinct assessor signal
  beyond the card_instance itself, that is a new node type and its own slice.
