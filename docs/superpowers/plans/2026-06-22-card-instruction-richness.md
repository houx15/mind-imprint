# Card Instruction Richness (Layer B) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans for the code phases (P1/P3) and subagent-driven authoring for the content phase (P2). Steps use checkbox (`- [ ]`) syntax.

**Goal:** Replace each step's flat `methodology_note: string` with a structured, Markdown `{why, how, when, example?}` rendered inline on every step, fixing the steps[1+] invisibility bug while preserving the `note_open` envelope signal.

**Architecture:** Phased so each phase ships green. P1 adds `methodology` as optional + renders it + wires the per-step `note_open` signal (additive, nothing breaks). P2 authors all 33 cards / 101 steps. P3 makes `methodology` required and deletes `methodology_note`.

**Tech Stack:** TypeScript, Zod (contracts), React 18, Vitest. Reuses `apps/web/src/workspace/Markdown.tsx`.

## Global Constraints

- Schema single source of truth = `packages/contracts`. New card = JSON only, no renderer change.
- Standard envelope unchanged. `note_open` event (`envelope.ts:6`) reused per-step; do NOT alter the envelope schema.
- Production sheet = `CardSheetHost` (`WorkspaceView.tsx:250`); `ActiveSheet` is dev-Harness only — keep consistent but secondary.
- Markdown content rendered via the existing `Markdown` component; no new markdown lib.
- `why/how/when` required; `example` optional.
- Gate: `pnpm -r typecheck && pnpm -r test` green after every task.

---

## Phase P1 — Schema (optional) + renderer + signal

### Task 1: Add optional `Methodology` to the Step schema

**Files:**
- Modify: `packages/contracts/src/cardSpec.ts` (Step `:4-10`)
- Test: `packages/contracts/test/cardSpec.test.ts`

**Interfaces:**
- Produces: `Methodology` type `{ why: string; how: string; when: string; example?: string }`; `Step.methodology?: Methodology` (optional in P1).

- [ ] **Step 1: Write the failing test** — add to `cardSpec.test.ts`:

```ts
describe("Methodology (Layer B)", () => {
  const base = { id: "c", category: "信息素养", name: "n", purpose: "", trigger_condition: "", rubric_tags: [],
    steps: [{ key: "s", title: "T", disclose: "always", methodology_note: "",
      methodology: { why: "因为重要", how: "这样做", when: "卡住时" },
      fields: [{ type: "textarea", key: "x", label: "L" }] }] };
  it("accepts a step with structured methodology (why/how/when)", () => {
    expect(CardSpec.safeParse(base).success).toBe(true);
  });
  it("accepts an optional example", () => {
    const withEx = { ...base, steps: [{ ...base.steps[0], methodology: { ...base.steps[0].methodology, example: "比如…" } }] };
    expect(CardSpec.safeParse(withEx).success).toBe(true);
  });
  it("rejects methodology missing a required beat (how)", () => {
    const bad = { ...base, steps: [{ ...base.steps[0], methodology: { why: "x", when: "y" } }] };
    expect(CardSpec.safeParse(bad).success).toBe(false);
  });
});
```

- [ ] **Step 2: Run to verify it fails**
Run: `pnpm --filter @mind-imprint/contracts test -- cardSpec`
Expected: FAIL (methodology not in schema → strict parse strips it, the reject-case won't fail as expected).

- [ ] **Step 3: Add the schema**

In `cardSpec.ts`, before `Step`:

```ts
export const Methodology = z.object({
  why: z.string().min(1),
  how: z.string().min(1),
  when: z.string().min(1),
  example: z.string().optional(),
});
```

Add to `Step` (keep `methodology_note` for now):

```ts
export const Step = z.object({
  key: z.string().min(1),
  title: z.string().min(1),
  disclose: z.enum(["always", "on_demand"]),
  methodology_note: z.string(),
  methodology: Methodology.optional(),
  fields: z.array(FieldPrimitive).min(1),
});
```

Add `export type Methodology = z.infer<typeof Methodology>;`.

- [ ] **Step 4: Run to verify it passes**
Run: `pnpm --filter @mind-imprint/contracts test -- cardSpec` → PASS.

- [ ] **Step 5: Commit** — `feat(contracts): add optional structured methodology to Step`

---

### Task 2: Render the per-step methodology panel in `CardRenderer`

**Files:**
- Modify: `apps/web/src/cards/CardRenderer.tsx`
- Test: Create `apps/web/src/cards/cardRenderer.methodology.test.tsx`

**Interfaces:**
- Consumes: `Step.methodology` (Task 1), `Markdown` from `../workspace/Markdown`.
- Produces: `CardRenderer` prop `onNote: (stepKey: string) => void`; a per-step collapsible methodology panel.

- [ ] **Step 1: Write the failing test** (`cardRenderer.methodology.test.tsx`):

```tsx
import { describe, it, expect, vi } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import type { CardSpec } from "@mind-imprint/contracts";
import { CardRenderer } from "./CardRenderer";

const card = {
  id: "t", category: "信息素养", name: "n", purpose: "", trigger_condition: "", rubric_tags: [],
  steps: [{
    key: "s1", title: "第一步", disclose: "always", methodology_note: "",
    methodology: { why: "**因为**重要", how: "这样做", when: "卡住时", example: "比如这样" },
    fields: [{ type: "textarea", key: "x", label: "L" }],
  }],
} as unknown as CardSpec;

describe("CardRenderer methodology panel", () => {
  it("reveals why/how/when/example (markdown) and fires onNote on expand", () => {
    const onNote = vi.fn();
    render(<CardRenderer card={card} values={{}} onField={vi.fn()} onExpandStep={vi.fn()} onNote={onNote} />);
    fireEvent.click(screen.getByText("方法"));
    expect(onNote).toHaveBeenCalledWith("s1");
    expect(screen.getByText("这样做")).toBeTruthy();
    expect(screen.getByText("卡住时")).toBeTruthy();
    expect(screen.getByText("比如这样")).toBeTruthy();
    // markdown highlight renders as <strong>, not literal **
    expect(screen.getByText("因为").tagName.toLowerCase()).toBe("strong");
  });
});
```

- [ ] **Step 2: Run to verify it fails**
Run: `pnpm --filter web test -- cardRenderer.methodology`
Expected: FAIL — `onNote` prop doesn't exist, no "方法" trigger.

- [ ] **Step 3: Implement**

Add `onNote: (stepKey: string) => void` to `CardRenderer`'s `Props`. Create a `MethodologyPanel` sub-component (collapsible; on first expand calls `onNote(stepKey)`), rendering four labeled blocks via `Markdown`. Render it inside both the `always` branch (after `<h3>`, before `<StepFields>`) and the `OnDemandStep` body. Thread `onNote` down.

```tsx
import { Markdown } from "../workspace/Markdown";
// ...
function MethodologyPanel({ step, onNote }: { step: Step; onNote: (k: string) => void }) {
  const m = step.methodology;
  const [open, setOpen] = useState(false);
  if (!m) return null;
  return (
    <div className="mb-4">
      <button type="button" aria-expanded={open}
        onClick={() => { if (!open) onNote(step.key); setOpen((v) => !v); }}
        className="flex items-center gap-2 text-[12.5px] font-semibold text-mk-primary">
        方法
      </button>
      {open && (
        <div className="mt-2 space-y-3 rounded-mk border border-mk-primary-tint bg-mk-primary-tint/30 p-4 text-[13px] leading-relaxed text-[#3A4256]">
          <div><div className="mb-1 font-bold text-mk-ink">为什么</div><Markdown text={m.why} /></div>
          <div><div className="mb-1 font-bold text-mk-ink">怎么做</div><Markdown text={m.how} /></div>
          <div><div className="mb-1 font-bold text-mk-ink">什么时候用</div><Markdown text={m.when} /></div>
          {m.example && <div><div className="mb-1 font-bold text-mk-ink">例子</div><Markdown text={m.example} /></div>}
        </div>
      )}
    </div>
  );
}
```

Place `<MethodologyPanel step={step} onNote={onNote} />` in the `always` section (after the title) and in `OnDemandStep`'s expanded body (before `StepFields`). Add `onNote` to `OnDemandStep`'s props.

- [ ] **Step 4: Run to verify it passes** → PASS.

- [ ] **Step 5: Commit** — `feat(cards): render per-step methodology panel (markdown)`

---

### Task 3: Wire `onNote` → `note_open` in both sheets; drop the global note panel

**Files:**
- Modify: `apps/web/src/workspace/CardSheetHost.tsx` (production), `apps/web/src/cards/states/ActiveSheet.tsx` (dev)
- Test: Create/extend `apps/web/src/workspace/cardSheetHost.note.test.tsx`

**Interfaces:**
- Consumes: `CardRenderer`'s `onNote`.
- Produces: per-step `note_open` envelope events.

- [ ] **Step 1: Write the failing test** — render `CardSheetHost`, expand a step's "方法", assert the envelope gains a `note_open` with that step_key:

```tsx
// expand methodology on step "s1" → onSubmit envelope events include { kind: "note_open", step_key: "s1" }
```
(Drive via clicking "方法", then "提交并钉到过程树", and inspect the `finalInstance.events` passed to `onSubmit`.)

- [ ] **Step 2: Run to verify it fails** — `onNote` not wired; no per-step note_open.

- [ ] **Step 3: Implement**
- `CardSheetHost`: remove the header "这个工具怎么用" button + the `noteOpen && note` panel + `firstStepKey`/`note` locals. Replace `handleNoteOpen` with `handleNote(step_key)` that dispatches `note_open` for that key. Pass `onNote={handleNote}` to `CardRenderer`.
- `ActiveSheet`: same shape (dev Harness) — pass `onNote` through; remove global note button/panel. Harness supplies an `onNote` (can be a no-op or wired to its event log).
- Update `Harness.tsx` call site to pass `onNote`.

- [ ] **Step 4: Run to verify it passes** → PASS.

- [ ] **Step 5: Full gate** — `pnpm -r typecheck && pnpm -r test`. Fix any test that referenced the removed global note button.

- [ ] **Step 6: Commit** — `feat(cards): per-step note_open signal; remove global note panel`

---

## Phase P2 — Author all 33 cards / 101 steps

### Task 4: Migrate card content to structured methodology

**Files:** all 33 JSONs in `packages/contracts/cards/`.
**Test:** add a guard in `packages/contracts/test/registry.test.ts`.

**Authoring format (per step):**
```json
"methodology": {
  "why":  "为什么这一步重要 / 它在防什么（Markdown，可 **高亮**）",
  "how":  "具体怎么做（Markdown）",
  "when": "什么时候用这一步（Markdown）",
  "example": "可选：一个具体例子（Markdown）"
}
```
Rules: source = the step's existing `methodology_note` (split why vs how, preserve voice); add `when`; add `example` only where it sharpens; use `**bold**` for key terms. Keep `methodology_note` in place (removed in P3).

- [ ] **Step 1: Write the guard test** (red until all migrated):

```ts
it("every step of every card has structured methodology", () => {
  const reg = loadRegistry();
  for (const card of Object.values(reg))
    for (const step of card.steps) {
      expect(step.methodology, `${card.id}/${step.key}`).toBeDefined();
      expect(step.methodology!.why.length).toBeGreaterThan(0);
      expect(step.methodology!.how.length).toBeGreaterThan(0);
      expect(step.methodology!.when.length).toBeGreaterThan(0);
    }
});
```

- [ ] **Step 2: Run → FAIL** (no card migrated yet).

- [ ] **Step 3: Author content in category batches.** Dispatch one subagent per batch (group by `category`), each given: the batch's card JSONs, the format + rules above, and the instruction to ADD `methodology` to every step preserving the existing note's meaning/voice. Controller reviews each batch for voice consistency and Markdown validity before accepting.

- [ ] **Step 4: Run → PASS** once all 101 steps carry `methodology`.

- [ ] **Step 5: Commit** (may be several commits, one per batch) — `content(cards): structured methodology for <category>`

---

## Phase P3 — Make required, remove `methodology_note`

### Task 5: Tighten schema and delete the legacy field

**Files:** `cardSpec.ts`, all 33 JSONs (remove `methodology_note`), `cardSpec.test.ts`, `cardRenderer.showif.test.tsx`, `viewModel.test.ts`, any remaining references.

- [ ] **Step 1: Update tests** — change `cardSpec.test.ts` fixtures to use `methodology` (required) and drop `methodology_note`; add a test that a step WITHOUT `methodology` is rejected; remove `methodology_note` from test fixtures in `cardRenderer.showif.test.tsx` and `viewModel.test.ts`.
- [ ] **Step 2: Run → FAIL** (schema still optional / fixtures still have old field).
- [ ] **Step 3: Implement** — in `cardSpec.ts` change `methodology: Methodology.optional()` → `methodology: Methodology` and remove `methodology_note: z.string()`. Remove `methodology_note` from all 33 JSONs. Grep to confirm zero references remain in `apps/web/src` + `packages/contracts`.
- [ ] **Step 4: Full gate** — `pnpm -r typecheck && pnpm -r test` green.
- [ ] **Step 5: Commit** — `refactor(cards): methodology required; remove legacy methodology_note`

---

## Verification

- [ ] `pnpm -r typecheck && pnpm -r test` green.
- [ ] Manual/dev: open a multi-step card (e.g. SIFT) — every step shows a "方法" panel; expanding reveals why/how/when/(example) with Markdown highlights; expansion is recorded as `note_open` per step.
