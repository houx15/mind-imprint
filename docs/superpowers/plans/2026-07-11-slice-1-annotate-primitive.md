# Slice 1 — `annotate` Primitive + Dossier Shell Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the `annotate` interaction primitive (span-indexed material read-view with clickable AI/student spans) and the 信源档案 source-dossier shell around it — controlled, fixture-backed, no backend, no live agent, no `graph`.

**Architecture:** New React tree under `apps/web/src/primitives/annotate/` (the primitive) and `apps/web/src/workspace/material/` (the dossier shell + fixtures), plus a dev-harness mount. Components are controlled; state comes from props/fixtures; `AnnotateState` (C1) and `StudioEvent` (C4) come from `@mind-imprint/contracts`. Tested with vitest + React Testing Library. Design: `docs/superpowers/specs/2026-07-11-slice-1-annotate-primitive-design.md`; visual reference: the `.dc.html` MATERIAL view.

**Tech Stack:** React + TypeScript + vitest + @testing-library/react.

## Global Constraints

- **Frontend only.** No backend, no contracts changes, no network/`api` calls, no live agent. All state is props/fixtures.
- **Controlled components.** The parent (dossier shell / dev host) owns state; the primitive renders and calls back.
- **Reuse contracts:** `AnnotateState` (C1) and `StudioEvent` (C4) from `@mind-imprint/contracts`. Do not redefine them.
- **New tree only.** Create under `apps/web/src/primitives/annotate/` and `apps/web/src/workspace/material/`. **Do NOT mutate** `apps/web/src/workspace/MaterialPane.tsx` (old task world).
- **Icons = inline SVG**, never lucide-react. Inline styles consistent with existing components; visual per the `.dc.html` MATERIAL view (Plus Jakarta Sans / Noto Sans SC, the purple `<mark>` highlight, the card/list treatments).
- **Student-span *creation* (text-selection → new span) is DEFERRED** to the MATERIAL-view slice (jsdom can't drive real text selection; the design centers AI highlights). Slice 1 *renders* student-authored spans if present in state, but does not add a selection-to-create affordance. (Spec §6 Q2.)
- **Commands** (from `apps/web`): `pnpm test` (vitest run) and `pnpm typecheck` (tsc --noEmit) — both green after each task.
- **TDD:** failing test → confirm fail → implement → confirm pass → commit.

---

## Task 1: The block segmenter (pure)

**Files:**
- Create: `apps/web/src/primitives/annotate/segment.ts`
- Test: `apps/web/src/primitives/annotate/segment.test.ts`

**Interfaces:**
- Produces: `type Run = { text: string; spanId: string | null; author: Author | null }` and `segmentBlock(blockId, text, spans): Run[]`.

- [ ] **Step 1: Failing test**

```ts
// segment.test.ts
import { describe, it, expect } from "vitest";
import { segmentBlock } from "./segment";
import type { AnnotateState } from "@mind-imprint/contracts";

const spans: AnnotateState["spans"] = [
  { id: "s1", block_ref: "b1", range: { start: 3, end: 7 }, tag: "authority", note: "who?", author: "ai" },
];

describe("segmentBlock", () => {
  it("returns one plain run when no spans touch the block", () => {
    expect(segmentBlock("b1", "hello world", [])).toEqual([{ text: "hello world", spanId: null, author: null }]);
  });
  it("splits a mid-block span into plain/marked/plain runs", () => {
    const runs = segmentBlock("b1", "abcXXXXdef", spans); // range 3..7 => "XXXX"
    expect(runs).toEqual([
      { text: "abc", spanId: null, author: null },
      { text: "XXXX", spanId: "s1", author: "ai" },
      { text: "def", spanId: null, author: null },
    ]);
  });
  it("ignores spans for other blocks", () => {
    expect(segmentBlock("bOther", "abcXXXXdef", spans)).toEqual([{ text: "abcXXXXdef", spanId: null, author: null }]);
  });
  it("marks the whole block when a span has no range", () => {
    const whole: AnnotateState["spans"] = [{ id: "s2", block_ref: "b1", tag: "t", note: "", author: "student" }];
    expect(segmentBlock("b1", "abc", whole)).toEqual([{ text: "abc", spanId: "s2", author: "student" }]);
  });
});
```

- [ ] **Step 2: Confirm fail** (`pnpm test segment`).
- [ ] **Step 3: Implement** — collect `spans.filter(s => s.block_ref === blockId)`; for each, a range `[start,end)` (default `[0, text.length]` when `range` absent); sort by start; walk the text emitting plain runs between spans and marked runs for each span (first-wins on overlap — assume non-overlapping in Slice 1); return `Run[]` covering the whole text. Import `Author` type from contracts for `Run.author`.
- [ ] **Step 4: Test → PASS**, `pnpm typecheck`.
- [ ] **Step 5: Commit** — `feat(web): annotate block segmenter (spans -> runs)`.

---

## Task 2: The `annotate` component

**Files:**
- Create: `apps/web/src/primitives/annotate/Annotate.tsx`, `apps/web/src/primitives/annotate/index.ts`
- Test: `apps/web/src/primitives/annotate/Annotate.test.tsx`

**Interfaces:**
- Consumes: `segmentBlock` (Task 1), `AnnotateState` (C1).
- Produces: `Annotate` component with props:
```ts
{ blocks: { id: string; text: string }[]; state: AnnotateState;
  activeSpanId: string | null; onSelectSpan: (id: string | null) => void; }
```

- [ ] **Step 1: Failing test**

```tsx
// Annotate.test.tsx
import { describe, it, expect, vi } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import { Annotate } from "./Annotate";
import type { AnnotateState } from "@mind-imprint/contracts";

const blocks = [{ id: "b1", text: "abcXXXXdef" }];
const state: AnnotateState = { material_id: "m1",
  spans: [{ id: "s1", block_ref: "b1", range: { start: 3, end: 7 }, tag: "权威性", note: "这条往上追，原始出处是谁？", author: "ai" }] };

describe("Annotate", () => {
  it("renders a clickable AI-highlighted span and selects it", () => {
    const onSelect = vi.fn();
    render(<Annotate blocks={blocks} state={state} activeSpanId={null} onSelectSpan={onSelect} />);
    fireEvent.click(screen.getByText("XXXX"));
    expect(onSelect).toHaveBeenCalledWith("s1");
  });
  it("reveals the active span's dimension + question", () => {
    render(<Annotate blocks={blocks} state={state} activeSpanId="s1" onSelectSpan={() => {}} />);
    expect(screen.getByText("权威性")).toBeInTheDocument();
    expect(screen.getByText(/原始出处是谁/)).toBeInTheDocument();
  });
});
```

- [ ] **Step 2: Confirm fail.**
- [ ] **Step 3: Implement** — for each block render a `<p>`; `segmentBlock(block.id, block.text, state.spans)` → runs; a run with `spanId` renders a clickable `<mark>` (author-styled: `ai` = purple design treatment, `student` = a distinct tone; `activeSpanId === spanId` gets an active outline) calling `onSelectSpan(spanId)`; plain runs render `<span>`. Below the article, when `activeSpanId` matches a span, render the active-span panel showing its `tag` (dimension) and `note` (question). Style per the `.dc.html` MATERIAL read-view. `index.ts` re-exports `Annotate` and the segmenter types.
- [ ] **Step 4: Test → PASS**, typecheck.
- [ ] **Step 5: Commit** — `feat(web): annotate read-view component (clickable spans + active panel)`.

---

## Task 3: Source fixtures

**Files:**
- Create: `apps/web/src/workspace/material/fixtures.ts`
- Test: `apps/web/src/workspace/material/fixtures.test.ts`

**Interfaces:**
- Produces: `type SourceFixture` (per design §2) and `SOURCE_FIXTURES: SourceFixture[]`.

- [ ] **Step 1: Failing test** — assert `SOURCE_FIXTURES` has ≥2 entries; at least one `view: "article"` with ≥1 block and ≥1 AI span whose `tag`/`note` are non-empty; at least one `view: "summary"` with a `takeaway`; every span's `author` is a valid `Author`; every `annotate.material_id` matches the source id.
- [ ] **Step 2: Confirm fail.**
- [ ] **Step 3: Implement** — define `SourceFixture` (`{ id, name, type, tier, craapLabel, role, locked, view, meta?, blocks, takeaway?, annotate: AnnotateState }`) and seed from the calibration scenario: the "How China quietly greened the Earth — seen from space" blog (article, 2–3 real blocks, 1–2 AI spans with CRAAP dimension + question in `tag`/`note`) plus one官方 source as a `summary` with a takeaway. Real content, not lorem ipsum.
- [ ] **Step 4: Test → PASS**, typecheck.
- [ ] **Step 5: Commit** — `feat(web): source dossier fixtures (China-greening scenario)`.

---

## Task 4: The source-dossier shell

**Files:**
- Create: `apps/web/src/workspace/material/SourceDossier.tsx`
- Test: `apps/web/src/workspace/material/SourceDossier.test.tsx`

**Interfaces:**
- Consumes: `Annotate` (Task 2), `SourceFixture` (Task 3), `StudioEvent` (C4).
- Produces: `SourceDossier` with props `{ sources: SourceFixture[]; onEvent?: (e: StudioEvent) => void }`.

- [ ] **Step 1: Failing test**

```tsx
// SourceDossier.test.tsx (essentials)
import { describe, it, expect, vi } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import { SourceDossier } from "./SourceDossier";
import { SOURCE_FIXTURES } from "./fixtures";

it("lists sources with locked count and opens one, emitting source_opened", () => {
  const onEvent = vi.fn();
  render(<SourceDossier sources={SOURCE_FIXTURES} onEvent={onEvent} />);
  expect(screen.getByText(/信源档案/)).toBeInTheDocument();
  fireEvent.click(screen.getByText(SOURCE_FIXTURES[0]!.name));
  expect(onEvent).toHaveBeenCalledWith(expect.objectContaining({ type: "source_opened", surface: "studio" }));
});

it("in an article source, clicking a span reveals its question; back returns to the list", () => {
  render(<SourceDossier sources={SOURCE_FIXTURES} />);
  const article = SOURCE_FIXTURES.find((s) => s.view === "article")!;
  fireEvent.click(screen.getByText(article.name));
  // click the first AI span text, assert its note shows, then go back
  // (use the fixture's span text/note)
});
```

- [ ] **Step 2: Confirm fail.**
- [ ] **Step 3: Implement** — local state `{ openId, activeSpanId }`. **List view** (`openId == null`): header "信源档案 · 已收集 N 篇" + "已锁定 X/N" (X = `sources.filter(s=>s.locked).length`); a row per source (name · craapLabel · type · tier · "作用与风险：{role}"); row click sets `openId`, resets `activeSpanId`, and fires `onEvent({ type:"source_opened", surface:"studio", url: source.id, time_spent_s: 0 })`. **Read view** (`openId` set): a back control (→ list); if `view==="article"` render header (name + `meta`) + `<Annotate blocks state activeSpanId onSelectSpan={setActiveSpanId} />` + the footer hint pointing to the future rail; if `view==="summary"` render name/type/tier/一句话摘要/`takeaway`. Style per the `.dc.html`.
- [ ] **Step 4: Test → PASS**, typecheck.
- [ ] **Step 5: Commit** — `feat(web): 信源档案 source-dossier shell over the annotate primitive`.

---

## Task 5: Dev-harness mount

**Files:**
- Modify: `apps/web/src/dev/DevApp.tsx` (add an "Annotate / 素材" entry) — read the file first and follow its existing panel pattern.
- Test: extend `apps/web/src/dev/DevApp.test.tsx` (or a new `material dev` test) asserting the dossier renders in the harness and emitted events are logged.

- [ ] **Step 1: Failing test** — the dev harness shows the annotate/素材 panel; opening a source logs a `source_opened` event line.
- [ ] **Step 2: Confirm fail.**
- [ ] **Step 3: Implement** — add a panel/route that mounts `<SourceDossier sources={SOURCE_FIXTURES} onEvent={append-to-log} />` and renders the event log, following DevApp's existing structure. No backend.
- [ ] **Step 4: Test → PASS**, typecheck, and `pnpm test` (whole web suite green).
- [ ] **Step 5: Commit** — `feat(web): dev-harness mount for the annotate dossier + event log`.

---

## Task 6: Slice verification + roadmap log

**Files:**
- Modify: `docs/2026-07-11-whole-product-refactor-roadmap.md` (per-slice log + Slice 1 status ☑).

- [ ] **Step 1:** From `apps/web`: `pnpm test` (whole suite green) + `pnpm typecheck`. Confirm no `api`/network calls were introduced and `MaterialPane.tsx` is unchanged (`git diff --stat main...HEAD` should not list it).
- [ ] **Step 2:** Confirm acceptance criteria (design §7): annotate renders clickable author-styled spans + active panel; dossier navigates list↔article↔summary + emits `source_opened`; dev host works; all tested; no backend/graph/rail/card introduced.
- [ ] **Step 3:** Update the roadmap per-slice log: Slice 1 ☑, link spec + this plan, commit range.
- [ ] **Step 4: Commit** — `docs(refactor2): Slice 1 annotate primitive complete — log + status`.

---

## Self-review notes

- **Spec coverage:** annotate component (T1–T2) · dossier shell (T3–T4) · dev host (T5) · C4 event seam (T4) · verification (T6). All design §1–§5 items map to a task.
- **Deferred by design:** student-span creation via text selection (Global Constraints + spec §6 Q2); backend persistence + source log (Slice 6); coach rail answering (Slice 2); `graph` (Slice 7).
- **Type consistency:** `AnnotateState`/`Author`/`StudioEvent` come from contracts; `Run` defined once in `segment.ts` and consumed by `Annotate.tsx`; `SourceFixture` defined once in `fixtures.ts` and consumed by the shell + dev host.
