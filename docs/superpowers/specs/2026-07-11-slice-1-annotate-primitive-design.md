# Slice 1 — The `annotate` Primitive + Source Dossier Shell

| | |
|---|---|
| **Status** | Draft — for review |
| **Slice** | 1 of the whole-product refactor (`docs/2026-07-11-whole-product-refactor-roadmap.md`) |
| **Sources** | `2026-07-11-agent-spec.md` C1/§2 · `2026-07-11-product-spec.md` §8.2 (素材 view) · the Claude Design `.dc.html` MATERIAL view (binding UI) |
| **Depends on** | Slice 0 (C1 `AnnotateState`, C4 `StudioEvent`) |
| **Delivers** | The hand-built `annotate` interaction primitive and the 信源档案 dossier shell around it — controlled, fixture-backed, no live agent, no backend persistence, no `graph`. |

## 0. Scope and boundary

Slice 1 builds the first interaction primitive — `annotate` — as the design's MATERIAL
(素材) read-view, plus the 信源档案 source-dossier shell that navigates into it. It is a
**presentation + interaction layer only**: components are **controlled** (the parent owns
`AnnotateState`), backed by **fixtures**, emitting **C4 events** through a callback seam.

**In scope:** the `annotate` component (span-indexed material rendering, clickable
AI/student spans, select-to-reveal a span's dimension/question, student annotation via text
selection); the dossier list → open-source → read-view / summary navigation shell; a dev
host; tests.
**Out of scope (later slices):** backend persistence and the source log (Slice 6, RL-2) ·
the coach rail that answers a span's question (Slice 2) · CRAAP as a card over this
primitive (Slice 3) · the `graph` primitive and STRUCTURE view (Slice 7) · any live agent.

This is a **new** component tree under `apps/web/src/primitives/annotate/` and
`apps/web/src/workspace/material/`. It does **not** mutate the existing task-centric
`MaterialPane.tsx` (old world, retired later), though it may lift `renderBlock`'s
segment-splitting approach.

## 1. The `annotate` component

Controlled, `AnnotateState`-driven (C1: `{ material_id, spans: [{ id, range?|block_ref?, tag, note, author }] }`).

**Props:**
```ts
{
  blocks: { id: string; text: string }[];   // span-indexed material (reuse Material.blocks shape)
  state: AnnotateState;                      // spans to render (author = "ai" | "student" | "imported")
  activeSpanId: string | null;
  onSelectSpan: (spanId: string | null) => void;
  onAddSpan: (span: { block_ref: string; range: {start:number;end:number}; text: string }) => void;
  onEvent?: (e: StudioEvent) => void;        // C4 seam (e.g. prompt/interaction telemetry)
}
```

**Rendering (per the MATERIAL read-view):**
- Each block renders as a `<p>` of **runs**; a run overlapping a span is a clickable
  `<mark>`, styled by `author` (AI highlight = the design's 点亮/purple treatment; student =
  a distinct tone). Segment splitting adapts `MaterialPane.renderBlock`, keyed by span `id`.
- Clicking a highlighted run calls `onSelectSpan(id)`; the active span renders a small
  panel below the article showing its `tag`/dimension and `note`/question (design:
  `activeHlDim` + `activeHlQ`). In Slice 1 this is **display only** — answering moves to the
  coach rail in Slice 2.
- Selecting plain text (a range) offers "标注" → `onAddSpan(...)` with `author:"student"`;
  the parent adds it to `state` (controlled). Keep this minimal per the design, which
  centers AI-authored highlights.

**Authorship is explicit and visible** — AI vs student spans are visually distinct and
carry `author` in state (Slice 0 enforcement + assessment depend on it).

## 2. The source-dossier shell (信源档案)

A fixture-backed navigation shell composing the primitive (design MATERIAL view, `s3IsList`
↔ `s3IsRead`):

- **List (`s3IsList`):** "信源档案 · 已收集 N 篇 · 已锁定 X/N"; a `SourceCard[]` list, each
  row: name · CRAAP label · type · tier · "作用与风险: {role}" · locked state. Clicking a row
  opens it and emits a `source_opened` C4 event via `onEvent`.
- **Read (`s3IsRead`):** a back-to-list control, then either
  - `openIsArticle`: article header (title + meta line) + the `annotate` component over the
    source's blocks + the active-span panel; a footer hint pointing to the (future) rail; or
  - `openIsSummary`: name · type · tier · 一句话摘要 · takeaway.
- Navigation state (which source is open, list-vs-read) lives in the shell (fixture/local
  state); no backend.

**Slice-1 fixture type** (local, not a contract yet — the real source log is Slice 6):
```ts
type SourceFixture = {
  id: string; name: string; type: string; tier: string; craapLabel: string;
  role: string; locked: boolean; view: "article" | "summary";
  meta?: string; blocks: { id: string; text: string }[]; takeaway?: string;
  annotate: AnnotateState;
};
```
Seed one realistic fixture from the calibration scenario (the "How China quietly greened the
Earth" blog + a couple of sources), not lorem ipsum.

## 3. Dev host

A route/panel (mirroring the existing `apps/web/src/console` dev surface) that mounts the
dossier shell with the fixtures and logs emitted `StudioEvent`s to the screen — so the
primitive is exercisable without the Studio shell (Slice 5) or a live agent.

## 4. Styling

Match the `.dc.html` MATERIAL view: Plus Jakarta Sans / Noto Sans SC, the card/list
treatments, the highlight `<mark>` styling, the active-span panel. Inline styles consistent
with the existing web components; **icons are inline SVG, not lucide-react**.

## 5. Testing (vitest + React Testing Library)

- Renders blocks with AI + student spans; the correct runs are marked and author-styled.
- Clicking a highlighted run calls `onSelectSpan` and reveals that span's dimension/question.
- A plain-text selection → `onAddSpan` with `author:"student"` and the right range.
- Opening a source emits a `source_opened` `StudioEvent` through `onEvent`.
- Dossier list ↔ read-view ↔ summary navigation; locked/unlocked counts render.
- No network calls (controlled + fixtures); the whole slice is unit-testable.

## 6. Open questions for the plan

1. Span geometry: `range` (char offsets within a block) vs whole-`block_ref` highlights. The
   design shows sentence-level runs → start with `block_ref` + optional `range`; the
   segmenter handles both.
2. Student-annotation affordance depth — the design foregrounds AI highlights; keep student
   creation minimal in Slice 1, expand in the MATERIAL-view slice if needed.
3. Where `onEvent` ultimately routes (event stream) is Slice 2/6; Slice 1 only fires the seam.

## 7. Acceptance criteria

- `annotate` renders a span-indexed material with clickable, author-styled AI/student spans;
  selecting a span reveals its dimension/question; text selection yields a student span.
- The 信源档案 dossier shell navigates list ↔ article ↔ summary over fixtures and emits
  `source_opened`.
- A dev host exercises it end-to-end with no backend and no live agent.
- All behavior covered by vitest/RTL tests; `pnpm --filter web test` + `typecheck` green.
- No backend persistence, no `graph`, no coach rail, no card wiring introduced.
