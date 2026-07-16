# Slice 9 — 0457 readiness gauge, made real (评估) · design

> Whole-product refactor #2. Builds on Slices 0–8b. Turns the static 就绪度
> fixture into a real readiness display projected from the whole-draft review's
> per-table judgment. One interface, one seeded renderer (0457 lamps), RL-3-clean.

## 1. Context — what exists, what this slice changes

- The 评估 view (`apps/web/src/studio/views/ReviewView.tsx`) already renders the
  就绪度 display as **0457 lamp gauges** — but the data is a **hard-coded fixture**
  (`apps/web/src/studio/fixtures.ts`, 表A–表H), wired to no real signal.
- The whole-draft review (Slice 8/8b) already produces the real per-table signal:
  each `review_item` intervention carries `agent.ReviewItem{CriterionCode,
  CriterionName, Band, Evidence, Missing, Fix}` for the **4 tables a written draft
  can evidence** — 表D 来源与证据 / 表E 分析 / 表F 评估 / 表H 表达与组织
  (`writing-project.json`'s `review_criteria`).
- `skills.ReviewCriterion` is annotated *"Minimal labels — NOT the Slice-9
  readiness band engine."* This slice is that engine, in its minimal form.
- The binding design (`docs/design/思维印记_工作区.dc.html`, the REVIEW view,
  lines ~1157–1226) stacks four blocks in the 评估 view: ① 就绪度 gauge · ② 先自己评
  一评 (self-score) · ③ 写一段研究回顾 (retro) · ④ AI 使用申报单. **Only ① is in
  scope here** (decided in brainstorm); ②③④ stay deferred.

## 2. Scope & locked decisions

Locked in brainstorm (all four the recommended path):

- **DEC-9.1 — Gauge only.** The 评估 view renders only block ① 就绪度. Blocks
  ②③④ (self-score / retro / RL-4 reflection editor / AI-declaration ledger) are a
  later slice. The `.tsx` stays gauge-only.
- **DEC-9.2 — 0457 only, pluggable.** Build the one-interface abstraction + only
  the 0457 table-by-table lamps renderer (the one board with seeded per-descriptor
  data). A second skin (9239 grid) later is config + one renderer, not a rewrite.
  Do **not** build four empty renderers (the Slice 8b anti-pattern).
- **DEC-9.3 — 4 review tables.** The gauge shows exactly 表D/E/F/H — the tables the
  whole-draft review covers. This is the literal "the review's criteria labels are
  the seam." No cross-station gauge projection for 表A/B/C/G in this slice.
- **DEC-9.4 — Extend the review output.** Lamps are lit by one new integer,
  `points`, emitted per-criterion by the review model that is *already* judging each
  table's band. No second model call, no separate assessor engine, no migration.

**RL-3 (never a predicted grade)** governs throughout: lamps show *which descriptor
cell* the draft sits in, never a score. The existing tooltip copy already states
this (「落在评分表的哪一格……不是预估分数」) and is unchanged.

## 3. Backend — the `points` seam

### 3.1 Skill config (`apps/api/internal/skills/skill.go` + `specs/writing-project.json`)

`ReviewCriterion` gains `Points` — the table's **total lamp count** (0457's point
ceiling for that table):

```go
// ReviewCriterion is one mark-scheme table the whole-draft review assesses the
// draft against (0457's 表D/E/F/H). Points is the table's total descriptor-point
// count — the total number of lamps the Slice-9 readiness gauge renders for it.
type ReviewCriterion struct {
	Code   string `json:"code"`
	Name   string `json:"name"`
	Points int    `json:"points"`
}
```

`writing-project.json` `review_criteria` gains `points` per table, using the
design's own numbers: **表D 4 · 表E 4 · 表F 3 · 表H 3**.

`Skill.Validate` requires `Points >= 1` for every declared review criterion (a
gauge with zero total lamps is meaningless). The annotation on `ReviewCriterion`
that said "NOT the Slice-9 readiness band engine" is removed.

### 3.2 Review output (`apps/api/internal/agent/review.go`)

Both the model's raw verdict and the enriched item gain `Points` (lit lamps):

```go
type ReviewItem struct {
	CriterionCode string `json:"criterion_code"`
	CriterionName string `json:"criterion_name"`
	Band          string `json:"band"`
	Evidence      string `json:"evidence"`
	Missing       string `json:"missing"`
	Fix           string `json:"fix"`
	Points        int    `json:"points"` // descriptor points evidenced, 0..criterion total
}

type WritingReviewVerdict struct {
	CriterionCode string `json:"criterion_code"`
	Band          string `json:"band"`
	Evidence      string `json:"evidence"`
	Missing       string `json:"missing"`
	Fix           string `json:"fix"`
	Points        int    `json:"points"`
}
```

The `ProposeReview` system prompt gains one instruction (all four voices — it is
voice-invariant, part of the assessment, not the coaching lens): for each
criterion, in addition to the band, emit `points` = how many of that table's
descriptor points the draft currently evidences, an integer from 0 up to the
table's total (the total is given to the model per criterion). `ProposeReview`
already maps `WritingReviewVerdict → ReviewItem`; it copies `Points` across
verbatim (no clamping here — the model's number is preserved as produced; the
projection is the single clamp site, §5). The criteria total is passed to the
prompt from the skill config the caller already holds.

## 4. Projection — review ⋈ config → gauges

### 4.1 New DTO (`apps/api/internal/studio/dto.go`)

```go
// GaugeDTO is one 0457 mark-scheme table on the 就绪度 readiness display: the
// descriptor cell the latest board-voice review placed the draft in. Lit/Total
// are lamp counts (which cell), never a predicted grade (RL-3). Note is the
// review's own `missing` — what's absent, so the student knows the next step;
// "" when the table is full (nothing missing). Level is derived from lit/total.
type GaugeDTO struct {
	Code  string `json:"code"`  // 表D..表H
	Name  string `json:"name"`  // 来源与证据 / 分析 / 评估 / 表达与组织
	Lit   int    `json:"lit"`   // clamp(review.points, 0, total)
	Total int    `json:"total"` // skill config points
	Note  string `json:"note"`  // review.missing; "" when full
	Level string `json:"level"` // "full" | "partial" | "empty"
}
```

`StudioDTO` gains a top-level `Readiness []GaugeDTO json:"readiness"` (always
non-nil `[]`), sitting beside `Writing`. It is its own view's content, mapped 1:1
to `views.review` client-side. The summary line («已点亮 N/M 格») is **not** carried
on the wire — it is pure presentation, derived in the view from the lamp sums.

### 4.2 `projectReadiness` (`apps/api/internal/studio/projection.go`)

A focused sibling of `projectWriting`, following the same per-view-helper pattern:

1. Iterate the skill's `ReviewCriteria` in declared order — the gauge card order is
   the config order (表D, 表E, 表F, 表H).
2. Load the latest snapshot's `review_item` interventions filtered to the **board
   voice** (reuse the exact Slice-8b voice tag: an intervention whose anchor has no
   `voice` key, or `voice=="board"`, is board; sceptic/layperson/executioner are
   coaching lenses and never drive readiness). Build a map `criterionCode →
   ReviewItem`.
3. For each config criterion produce a `GaugeDTO`:
   - `Total` = config `Points`.
   - `Lit` = `clamp(reviewItem.Points, 0, Total)` when a board review item exists
     for this code; **0** when none (no review yet, or a pre-Slice-9 review whose
     JSON has no `points` field → unmarshals to 0).
   - `Note` = `reviewItem.Missing` (or "" when no item / when full).
   - `Level` = `full` if `Lit == Total && Total > 0`; `empty` if `Lit == 0`; else
     `partial`.
4. Always emit all config criteria (so the display is stable pre-review: 4 unlit
   cards). Return `[]GaugeDTO`.

**Board-voice-only rule:** if the student ran only a sceptic pass and never a board
pass, there is no board review item → all gauges empty. Honest: the readiness
display is the assessment of record, and only the board voice produces that.

No re-query concern beyond the existing projection pattern — `projectReadiness`
loads the same latest-snapshot interventions `projectWriting` does; the plan may
share the load if it reads cleanly, otherwise a focused second read is consistent
with the codebase's per-view helpers.

## 5. Contracts (`packages/contracts/src/studioState.ts`)

`GaugeFx` graduates from a client-invented type to a **contract-backed** Zod
schema, matching `GaugeDTO` byte-for-byte:

```ts
export const Gauge = z.object({
  code: z.string(),
  name: z.string(),
  lit: z.number().int(),
  total: z.number().int(),
  note: z.string(),
  level: z.enum(["full", "partial", "empty"]),
});
export type Gauge = z.infer<typeof Gauge>;
```

The studio state schema gains `readiness: z.array(Gauge)` (non-nil). Go↔Zod parity
is guarded by `dto_parity_test.go` (add `readiness`/`GaugeDTO` to the Go-side key
sets). `agent.ReviewItem`'s Zod mirror, if any exists in contracts, gains `points`.

## 6. Client — `toStudioState` + `ReviewView`

- **`toStudioState`**: map `dto.readiness → views.review` (`GaugeFx[]`). Delete the
  static 表A–表H fixture from `fixtures.ts`; replace with a realistic **4-table**
  seeded readiness (表D/E/F/H, mixed levels) for stories/tests.
- **`GaugeFx`** (`state.ts`) is re-exported from the contract `Gauge` and changes
  shape from `{table,total,lit,note,level}` to `{code,name,lit,total,note,level}`.
- **`ReviewView`** renders per the binding design (`.dc.html` REVIEW view):
  - the 就绪度 heading + `?` tooltip (unchanged copy),
  - the `{{ gaugeSummary }}` line, top-right — **added** (the current `.tsx` omits
    it): `已点亮 {Σlit}/{Σtotal} 格`, computed from the tables,
  - the 2-col gauge grid: each card shows `code` + `name` (separately, per design —
    the current `.tsx` crams them into one `table` field), the `lit/total`
    countLabel, the lamp row (`total` lamps, `lit` filled in the level colour), and
    the `note`.
  - Empty state: 4 unlit cards render with blank notes; the summary reads
    `已点亮 0/14 格`. No separate "no review" banner — the honest unlit grid is the
    empty state.
- Level colours unchanged: full `#4C9A82` / partial `#D9A23D` / empty `#AEB4C2`.

## 7. Testing

- **Go — skills:** `writing-project.json` parses with `points`; `Validate` rejects a
  review criterion with `Points < 1`.
- **Go — agent:** `ProposeReview` includes the per-criterion total + points
  instruction in the prompt and parses `points` from the model JSON into
  `ReviewItem.Points` (verbatim, no clamp at this layer).
- **Go — studio:** Docker-backed seeded projection — `projectReadiness` builds 4
  gauges from a board review ⋈ config (lit = clamped points, level derivation,
  note = missing); board-voice-only filter (a sceptic-only review yields empty
  gauges); pre-review empty state (no review → 4 unlit cards); out-of-range points
  clamp to `[0,total]`; missing `points` (old JSON) → 0.
- **Go — parity:** `dto_parity_test` covers `readiness`/`GaugeDTO`.
- **Contracts:** `Gauge` schema + studio-state `readiness` parse; Go/Zod key
  parity.
- **Web:** `ReviewView` renders code+name+countLabel+lamps+note+summary from
  state; empty state (0/N); level colours; `toStudioState` maps `readiness →
  views.review`.

## 8. Non-goals / carry-forwards

- **Blocks ②③④** of the 评估 view (先自己评一评 self-score, 写一段研究回顾 retro /
  RL-4 read-only reflection editor, AI 使用申报单 declaration ledger) — a later S6
  slice.
- **The other four skins** (9239 grid, AP switches, AP band-portraits, TOK needle)
  — each is later config + one renderer behind the interface DEC-9.2 establishes.
- **表A/B/C/G** (cross-station readiness from gate/graph signal) — deferred; would
  need a cross-station gauge projection this slice deliberately avoids.
- **The Slice-10 assessment engine** (a different engine over the event stream →
  the 成长报告 reports) is unchanged and unrelated; the gauge reuses the existing
  review, not that engine.
```
