# 过程评估报告 · PDF 导出 — Design

**Date:** 2026-08-14
**Status:** Approved (design), pending implementation plan
**Related:** [[retire-old-evaluation-pipeline-2026-08-14]] (introduced the locked `EvaluationReport` + the disabled 导出 PDF seam), [[evaluation-report-v1-structure-2026-08-13]] (the report contract).

## Goal

Turn the disabled `导出 PDF` button (on both the student `EvaluationReportPage`
and the teacher `TeacherReportView`) into a real export that produces a
**richer, methodologically-transparent document** than the on-screen report.
The screen report is "at a glance"; the PDF wraps the *same locked
`EvaluationReport` data* in an expert skeleton — a shared axiom up front, a
per-part methodology introduction, and, for every cognitive-depth dimension, the
full rubric anchor ladder with the student's landed rung highlighted. Everything
is deterministic and static (locked report JSON + the in-repo `dualaxis.json`
rubric); **no LLM call is involved**.

## Approach (decided)

| Decision | Choice |
|---|---|
| Generation | **Client-side print** — a dedicated A4 print component in React → `window.print()` → the browser's "Save as PDF". Reuses the existing report data + design tokens, gives full CJK & rich tables via the browser engine, needs zero new server/infra, and matches the existing client-side `.docx`/`.xlsx` exports (`apps/web/src/workspace/export/index.ts`). |
| Language | **Chinese** — matches the 过程评估报告 content and the 思维印记 audience (student / teacher / parent, China-first). All section content + rubric anchors are already Chinese. |
| Surfaces | **Both** student (`EvaluationReportPage`) and teacher (`TeacherReportView`) — same document, same code path. |
| 对标框架 appendix | **Deferred** — not in v1 (the `standards` mapping stays available in `DUALAXIS_MODEL` for a later version). |

## Why client-side print (and no new route)

- The app is **state-driven, not routed** — there is no `react-router`. `AssessmentView`
  holds `openProjectId` and conditionally renders `EvaluationReportPage`; the teacher
  console renders `TeacherReportView`. So the print view is **not a route**. It is a
  print-only component mounted on demand, plus a `@media print` visibility swap.
- The 导出 PDF button is only meaningful once the report is loaded. Both surfaces already
  hold the `EvaluationReport` object in state when `status === "ready"`. **Printing needs
  no re-fetch** — it prints the in-hand `report`.
- The rubric methodology is **already on the frontend**: `packages/contracts/src/rubric.ts`
  imports `dualaxis.json` and exports `DUALAXIS_MODEL` (+ helpers `DepthDims()`,
  `AutonomySignals()`, `Lenses()`, `Model()`), re-exported from the package index. The print
  document imports it directly — same single source of truth Go embeds. No new data plumbing.

## Data sources (all static / no-LLM)

1. **`EvaluationReport`** (the locked v1 contract) — the per-student facts already rendered
   on screen: `basics`, `abstract`, `events`, `materials`, `depth[]` (D1–D6, `level` 1–4),
   `autonomy[]` (A1–A6, `band` 0–5), `promptLens`, `toolUsage`, `risks`.
2. **`DUALAXIS_MODEL`** from `@mind-imprint/contracts` — the methodology:
   - `.axiom` — 两轴永不合成总分；单次会话为事件级证据，不构成人级档位判定。
   - `.depth[]` — each `{ id, name, means, anchors:{L1,L2,L3,L4}, reflectionRule? }`.
   - `.autonomy[]` — each `{ id, name, means, event }` (no L-ladder; see D-vs-A below).
   - `.autonomyBand` — prose rule describing the 0–5 band as a behavior count.
   - `.opportunityRule` — 机会供给先于判定 (`not_supplied` = platform owes, not a student deficit).
   - `.lenses[]` + `.lensNote` — 提问透镜 methodology (reads AI-interaction traces only; not a third axis).
3. **Color ramps** `DEPTH_RAMP` / `AUTONOMY_RAMP` from `apps/web/src/shell/report/EvaluationReport/tokens.ts`
   — reused so the print doc's dimension color chips + ladder tints match the screen report's
   color language.

**铁律 note:** the on-screen report shows level/band **by color only, never a bare digit**
(`两轴永不合成总分`). The PDF respects this and goes further for v1: it never prints the exact
`level`/`band` and never marks a single "you-are-here" rung — the student's position is a
**color only** (ramp chip), and the L1–L4 ladder / 0–5 band strip appear as neutral, color-coded
**reference** for the reader. This deliberately hedges on exact placement while the real
generator is still unvalidated. No total, no composite number, no asserted rung.

## Document structure

The print document (`EvaluationReportPrint`) is a single A4-flow document. **Methodology is
distributed** — a thin shared axiom up front, then each part carries its own methodology
introduction immediately above its data (read exactly where it's needed, not as one forgotten
wall up front).

### 0 · 封面 (cover)
Identity block: 学生姓名 · 项目标题 · 项目类型 · 起止日期 · 生成日期. A one-line 班级/学校 strip
where available (student name + project title come from `report.student` / `report.basics`).

### 1 · 关于这份报告 (the one shared, global methodology)
The **only** consolidated methodology — because it governs the whole document and a parent
must read it before *anything*:
- `DUALAXIS_MODEL.axiom` restated for a lay reader (this is event-level evidence, never a
  person-level verdict; the two axes never combine into one score).
- One line each on what 认知深度 D and 智识自主 A are (headline only — the full reading guide
  lives in §5/§6, in context).

### 2 · 综述 / 学生画像
`report.abstract` (`overview` + the material/writing/AI sentences + suggestion paragraph) plus
`report.basics.counters` as a **small table** (AI 轮次 / 材料阅读 / 字数 / AI 批注 / 编辑次数)
and `basics.milestones` as a compact date row. Intro line: how the画像/counters are assembled
(deterministic tallies from the process record).

### 3 · 过程时间线
Intro: derived from the **event trace**. `report.events[]` as a **table** — 时间 · 阶段(kind) ·
摘要 · AI 轮次.

### 4 · 材料清单  *(full intro — requested)*
Intro (authored, static): what a source record captures and how it is judged — CRAAP-style
sourcing (来源等级 / 可信度 / 最终判定 / 能支持什么 / 不能支持什么). `report.materials[]` as a
**table** — 来源 · 加入时间 · 最终判定(finalStatus) · 用在哪(usedIn) · 备注(comment) ·
局限(cannotSupport).

### 5 · 认知深度 D (D1–D6) — the heart of the expert layer

> **v1 stance — no exact rung asserted.** The real evaluation generator is not yet
> validated, so this version does **not** claim "the student landed at L3." Instead of a
> printed level number or a single highlighted rung, the student's position is conveyed by a
> **color schema only** (the ramp), consistent with the on-screen report's 铁律
> (level by color, never a digit). The full L1→L4 ladder is still shown — as neutral,
> color-coded **reference** (what each rung means), not as a per-student scorecard.

Axis intro: what 深度 means + **how to read the L1→L4 ladder** (the four rungs are qualitative
anchors along a 浅→深 progression, not a 1–4 score; the two axes never combine into a total).
Then, for **each** `report.depth[i]`, one block:
- Heading: `{id} · {DUALAXIS_MODEL.depth.name}` + the `means` line + a **color chip**
  (`DEPTH_RAMP[level-1]`) as the *only* position signal — a soft 浅→深 hue, no digit, no label.
- **Anchor-ladder reference** — all four rungs `anchors.L1…L4` from the rubric, each row **tinted
  along the ramp** (`DEPTH_RAMP[0..3]`, light→dark) so the color schema itself teaches the
  progression. **No rung is marked "you are here"** and the exact `level` is never printed —
  the dimension's color chip carries the soft signal; the ladder stays neutral reference.
- D6 only: if `reflectionRule` applies (no self-written reflection ⇒ NA, never a low score),
  render that note.
- Then the student's `summary`, `evidence[]` (verbatim `quote` → `observation` → `boundary`),
  and `suggestion`. Evidence quotes are included **verbatim** (approved — "we love a longer report").

### 6 · 智识自主 A (A1–A6)

> **v1 stance — same as D.** No printed band number, no highlighted band cell — the student's
> position is a **color** (`AUTONOMY_RAMP[band]`) only. The 0–5 band strip is shown as
> color-coded reference (what the scale is), not as a per-student marker.

Axis intro: **band 0–5 is a behavior count, not a quality score** (`autonomyBand`) + 机会供给先于
判定 (`opportunityRule`). A does **not** have an L1–L4 ladder.
Then, for **each** `report.autonomy[i]`, one block:
- Heading: `{id} · {DUALAXIS_MODEL.autonomy.name}` + the `means` line + a **color chip**
  (`AUTONOMY_RAMP[band]`) as the only position signal (no digit).
- **Band-scale reference** — the shared 0–5 band strip rendered as 6 cells tinted along
  `AUTONOMY_RAMP[0..5]` (color schema = the scale), with the per-signal `event` line
  ("什么算一次可计事件") explaining what one countable action is. **No cell is marked as the
  student's**, and the exact `band` is never printed — the color chip carries the soft signal.
- Then `summary`, `evidence[]` (verbatim), `suggestion`.

### 7 · 提问透镜  *(full intro — requested)*
Intro from `DUALAXIS_MODEL.lensNote` + `lenses[]` (五个决定完整度 etc.): this lens reads **only
the AI-interaction trace**, and is explicitly **not a third scoring axis**. Then
`report.promptLens.summary` + `prompts[]` as a **table** — 阶段 · 提问(quote) · 观察 ·
关联维度(relatedDomains) · 需要注意(attention).

### 8 · 工具卡与子代理  *(full intro — requested)*
Intro (authored, static): what a 思维工具卡 is, that **triggering is automatic but opening is
the student's choice** (铁律②: 不操纵), and what "used" records. Then `report.toolUsage[]` as a
**table** — 工具(name) · 阶段 · 用途(purpose) · 摘要(summary).

### 9 · 风险提示 (conditional)
Intro: what a risk flag means (a process signal to discuss, not a verdict). Then:
- If `report.risks[]` is non-empty → a **table**: 类型(type) · 行为(behaviour) · 建议(suggestion).
- If `report.risks[]` is **empty** → the section is still rendered, with a positive statement
  **"本次评估未发现需要提示的风险行为。"** (a named-but-clean section shows the check was run;
  a missing section reads like it was skipped).

## Print mechanism (concrete)

- **`EvaluationReportPrint({ report })`** — a new component tree under
  `apps/web/src/shell/report/print/`, rendering the whole document above with the app's `mk-*`
  tokens and the same visual language as `EvaluationReportView`. It is print-first: single
  column, A4 measure, no sticky ruler / no app chrome.
- **Trigger + isolation** — a small helper (`printReport(report)` or a `<ReportPrintPortal>`):
  on click, render `EvaluationReportPrint` into a portal at `document.body` inside a
  `.report-print-root` wrapper, then call `window.print()`; unmount on `afterprint`.
  A global print stylesheet does the visibility swap:
  - `@media screen { .report-print-root { display: none } }` — never visible on screen.
  - `@media print { body > *:not(.report-print-root) { display: none !important } .report-print-root { display: block } }`
    — only the document prints.
- **Page-break control (print CSS):** `break-inside: avoid` on each dimension block, table
  row group, and evidence item; `break-before: page` on §5/§6 (and optionally each major
  section); `print-color-adjust: exact` so the highlighted rung / ramp colors survive printing.
- **Button wiring (both surfaces):** the existing
  `<Button variant="secondary" size="sm" disabled title="即将上线">导出 PDF</Button>` becomes
  enabled only when the report is ready (`state.status === "ready"` in `EvaluationReportPage`;
  the equivalent ready state in `TeacherReportView`), with `onClick={() => printReport(report)}`.
  Disabled/loading/generating states keep it inert.

## File structure

- **New:** `apps/web/src/shell/report/print/EvaluationReportPrint.tsx` — the document tree.
- **New (as needed):** small sub-components in the same folder for the cover, the D anchor-ladder
  table, the A band panel, and the generic section table — each focused, testable in isolation.
- **New:** `apps/web/src/shell/report/print/printReport.ts` (or a `ReportPrintPortal` component)
  — the mount → `window.print()` → cleanup helper.
- **New:** a print stylesheet (a `print.css` imported once, or `@media print` rules colocated) —
  the screen/print visibility swap + page-break rules.
- **Modify:** `apps/web/src/shell/report/EvaluationReportPage.tsx` — enable + wire the button.
- **Modify:** `apps/web/src/console/TeacherReportView.tsx` — enable + wire the button.
- **No backend changes.** No new contract fields (both `EvaluationReport` and `DUALAXIS_MODEL`
  already exist and are imported).

## Testing

- **Unit (vitest + jsdom):** `EvaluationReportPrint` renders, given a fixture `EvaluationReport`:
  the cover identity block; the shared axiom; every §; for a depth dim at `level=3`, **all four
  L1–L4 rungs render as reference, none marked "you-are-here", and the exact number "3"/"L3" is
  never printed** — the dim's color chip uses `DEPTH_RAMP[2]`; for an autonomy dim at `band=2`,
  **no band cell is marked and "2" is never printed** — the color chip uses `AUTONOMY_RAMP[2]`
  and the `event` line shows; the 材料/提问透镜/工具卡 intros are present; `risks=[]` → the positive
  "未发现风险" statement; `risks=[…]` → the risk table.
- **Trigger:** `printReport` calls `window.print` once and cleans up on `afterprint` — assert via
  a stubbed `window.print` (jsdom has no print engine; we test the wiring, not the rendered PDF).
- **Button gating:** the button is disabled until ready, enabled and calls `printReport(report)`
  when ready (both surfaces).
- Existing report/contract suites stay green (`npm run typecheck` — *not* `npm run build`, which
  skips typecheck — plus web + contracts vitest).

## Non-goals

- ❌ Server-side / Go PDF generation (no headless Chrome on the ECS host, no Go PDF lib).
- ❌ Any LLM call — the document is 100% deterministic from locked data + the static rubric.
- ❌ New `EvaluationReport` contract fields or new backend endpoints.
- ❌ Batch / whole-class export (single report per print in v1).
- ❌ 对标框架 (AP Research / standards) appendix — deferred to a later version.
- ❌ A new visual language — reuse `mk-*` tokens + the established report look.
- ❌ Editing / re-styling the on-screen `EvaluationReportView` (the print doc is a separate tree).

## Phasing (for the plan)

1. **Print scaffold + trigger.** `EvaluationReportPrint` shell (cover + 关于这份报告 + section
   frames), the `printReport` portal helper, the print stylesheet (visibility swap + page breaks),
   and button wiring on both surfaces. Renders the report end-to-end (sections may be thin).
2. **Section content + methodology intros.** Fill each section: the counters/timeline/材料/透镜/
   工具卡 tables + their intros; the D anchor-ladder reference blocks (ramp-tinted rungs + dim
   color chip, no asserted rung); the A band-scale reference (ramp-tinted 0–5 strip + event line +
   dim color chip, no asserted band); the conditional 风险提示. Unit tests per section.
