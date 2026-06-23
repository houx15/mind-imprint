# Card Custom Renderers (Layer C) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans. Tasks are tightly coupled (one feature in apps/web). Steps use checkbox (`- [ ]`) syntax.

**Goal:** Add a per-card escape-hatch custom renderer that overrides the schema `CardRenderer` and falls back to it, then prove it with a `BeliefSpectrumRenderer` (shared draggable axis), without changing the standard envelope.

**Architecture:** A `customRenderers` registry + `pickCardBody(cardId)` selector in `apps/web/src/cards`. Both sheets render `pickCardBody(spec.id)` with the SAME props as `CardRenderer`. Custom renderers write only `field_values` via `onField`, so `CardSheetHost` builds the envelope identically. `MethodologyPanel` is exported so custom renderers keep Layer B's per-step 方法.

**Tech Stack:** React 18, TS, Vitest + Testing Library. apps/web only — no contracts/eval changes.

## Global Constraints

- Standard envelope unchanged. Custom renderers touch ONLY `field_values` via `onField`, keyed by the spec's field keys. No envelope/setEnv access.
- `spectrum` value stays the integer stop index (0..stops−1) — snap-to-stops, no data-model change.
- Production sheet = `CardSheetHost`; `ActiveSheet` is dev-Harness only (keep consistent).
- New custom renderer = add to the registry; no schema/contracts edit.
- Gate: `pnpm -r typecheck && pnpm -r test` green after every task.

---

### Task 1: Renderer registry + selector + sheet wiring (fallback)

**Files:**
- Modify: `apps/web/src/cards/CardRenderer.tsx` (export `MethodologyPanel`, export `CardBodyProps` type alias)
- Create: `apps/web/src/cards/customRenderers.ts`
- Create: `apps/web/src/cards/customRenderers.test.ts`
- Modify: `apps/web/src/workspace/CardSheetHost.tsx`, `apps/web/src/cards/states/ActiveSheet.tsx`

**Interfaces:**
- Produces: `CardBodyProps` (= CardRenderer's props); `pickCardBody(cardId: string): ComponentType<CardBodyProps>`; `customRenderers` registry (empty in T1).

- [ ] **Step 1: Write the failing test** (`customRenderers.test.ts`):

```ts
import { describe, it, expect } from "vitest";
import { pickCardBody } from "./customRenderers";
import { CardRenderer } from "./CardRenderer";

describe("pickCardBody", () => {
  it("falls back to the schema CardRenderer for an unregistered card", () => {
    expect(pickCardBody("sift_craap")).toBe(CardRenderer);
    expect(pickCardBody("anything")).toBe(CardRenderer);
  });
});
```

- [ ] **Step 2: Run to verify it fails** — `pnpm --filter web test -- customRenderers` → FAIL (module missing).

- [ ] **Step 3: Implement**

In `CardRenderer.tsx`: export the `Props` type as `CardBodyProps` (e.g. `export type CardBodyProps = Props;`) and add `export` to `MethodologyPanel`.

Create `customRenderers.ts`:
```ts
import type { ComponentType } from "react";
import { CardRenderer, type CardBodyProps } from "./CardRenderer";

export const customRenderers: Record<string, ComponentType<CardBodyProps>> = {};

export function pickCardBody(cardId: string): ComponentType<CardBodyProps> {
  return customRenderers[cardId] ?? CardRenderer;
}
```

In `CardSheetHost.tsx`: replace `<CardRenderer ... />` with:
```tsx
const Body = pickCardBody(spec.id);
// ...
<Body card={spec} values={env.field_values} onField={handleField} onExpandStep={handleExpandStep} onNote={handleNote} />
```
Same swap in `ActiveSheet.tsx` (`const Body = pickCardBody(card.id)`; pass `onNote={onNoteOpen}`).

- [ ] **Step 4: Run to verify it passes** — `pnpm --filter web test -- customRenderers` → PASS. Existing sheet tests still pass (fallback is CardRenderer).

- [ ] **Step 5: Full gate** — `pnpm -r typecheck && pnpm -r test` green.

- [ ] **Step 6: Commit** — `feat(cards): custom renderer registry with schema fallback`

---

### Task 2: BeliefSpectrumRenderer (PoC) + registration

**Files:**
- Create: `apps/web/src/cards/renderers/BeliefSpectrumRenderer.tsx`
- Create: `apps/web/src/cards/renderers/BeliefSpectrumRenderer.test.tsx`
- Modify: `apps/web/src/cards/customRenderers.ts` (register it)

**Interfaces:**
- Consumes: `CardBodyProps`, `MethodologyPanel`, `TextField`/`TextAreaField` from `../fields/*`.
- Produces: `BeliefSpectrumRenderer`; `customRenderers["belief-spectrum"]`.

belief-spectrum fields: `issue`(text), `stances`(repeatable_group: who/position/believes/evidence/interest), `self`(spectrum, stops len 5), `self_reason`(textarea). `position`/`self` values are stop indices 0..4.

- [ ] **Step 1: Write the failing test** (`BeliefSpectrumRenderer.test.tsx`):

```tsx
import { describe, it, expect, vi } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import { loadRegistry, type CardSpec } from "@mind-imprint/contracts";
import { BeliefSpectrumRenderer } from "./BeliefSpectrumRenderer";

const card = loadRegistry()["belief-spectrum"] as CardSpec;

describe("BeliefSpectrumRenderer", () => {
  it("writes the self position as a stop index via onField", () => {
    const onField = vi.fn();
    render(<BeliefSpectrumRenderer card={card} values={{}} onField={onField} onExpandStep={vi.fn()} onNote={vi.fn()} />);
    // self axis: click the 3rd stop label of the 我 row → index 2
    fireEvent.click(screen.getAllByRole("radio", { name: card.steps[0]!.fields.find(f => f.key === "self")!.stops[2] })[0]!);
    expect(onField).toHaveBeenCalledWith("self", 2);
  });
  it("renders the issue text field and the methodology panel trigger", () => {
    render(<BeliefSpectrumRenderer card={card} values={{}} onField={vi.fn()} onExpandStep={vi.fn()} onNote={vi.fn()} />);
    expect(screen.getByText("争议议题是什么？")).toBeTruthy();
    expect(screen.getByText("方法")).toBeTruthy();
  });
  it("fires onNote when the methodology panel is expanded", () => {
    const onNote = vi.fn();
    render(<BeliefSpectrumRenderer card={card} values={{}} onField={vi.fn()} onExpandStep={vi.fn()} onNote={onNote} />);
    fireEvent.click(screen.getByText("方法"));
    expect(onNote).toHaveBeenCalledWith("main");
  });
});
```

- [ ] **Step 2: Run to verify it fails** — module missing.

- [ ] **Step 3: Implement `BeliefSpectrumRenderer.tsx`**

A component over `card.steps[0]`:
- Render `<MethodologyPanel step={step} onNote={onNote} />`.
- `issue` → `<TextField field={issueField} value={values.issue} onChange={(v) => onField("issue", v)} />`.
- A **shared spectrum axis** sub-component: given `stops` (from the `self` field) it lays out 5 labeled positions; each "thumb" (one per stance + one for 我) is a `role="radio"` group bound to that thumb; clicking a stop sets that thumb's index. Support click + arrow keys (mirror `SpectrumField`). Snap = integer index.
  - 我 thumb writes `onField("self", index)`.
  - stance thumb writes the stance array back: `onField("stances", stances.map((s, i) => i === idx ? { ...s, position: index } : s))`.
- `stances` rows: each has `who` (TextField) + its axis thumb + believes/evidence/interest (TextAreaField), with add/remove (write whole array via `onField("stances", next)`).
- `self_reason` → `<TextAreaField ... onChange={(v) => onField("self_reason", v)} />`.

Reuse `TextField`/`TextAreaField` for all text; hand-roll only the shared axis.

- [ ] **Step 4: Register** in `customRenderers.ts`:
```ts
import { BeliefSpectrumRenderer } from "./renderers/BeliefSpectrumRenderer";
export const customRenderers: Record<string, ComponentType<CardBodyProps>> = {
  "belief-spectrum": BeliefSpectrumRenderer,
};
```
Add a test to `customRenderers.test.ts`: `expect(pickCardBody("belief-spectrum")).toBe(BeliefSpectrumRenderer)`.

- [ ] **Step 5: Run to verify it passes** → PASS.

- [ ] **Step 6: Commit** — `feat(cards): BeliefSpectrumRenderer custom card (shared draggable axis)`

---

### Task 3: Guardrail — standard envelope unchanged

**Files:**
- Create/extend: `apps/web/src/workspace/cardSheetHost.customRenderer.test.tsx`

- [ ] **Step 1: Write the guardrail test** — render `CardSheetHost` with the real belief-spectrum spec + an active instance; interact (click a self stop), submit; assert `onSubmit`'s `finalInstance`:
  - `event_trace` contains a `field_change` for path `self` and a final `submit`;
  - `field_values.self === <index>`;
  - keys written are a subset of the spec's field keys (no foreign keys).

```tsx
// click 我's 3rd stop → field_change path "self"; submit → status completed; field_values.self === 2
```

- [ ] **Step 2: Run** — confirms the custom renderer flows through the standard envelope (should pass once T2 is in; if it fails, the guardrail is violated — fix the renderer, not the envelope).

- [ ] **Step 3: Full gate** — `pnpm -r typecheck && pnpm -r test` green.

- [ ] **Step 4: Commit** — `test(cards): guardrail — custom renderer emits standard envelope`

---

## Verification

- [ ] `pnpm -r typecheck && pnpm -r test` green.
- [ ] Manual/dev (Harness or app): open belief-spectrum — stances + 我 sit on one shared axis; dragging/clicking moves thumbs; submit pins a node to the process tree like any other card.
