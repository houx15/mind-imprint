# Slice 3c-C — `show_if` conditional disclosure Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: superpowers:subagent-driven-development. Steps use checkbox (`- [ ]`).

**Goal:** Add a `show_if` field modifier (optional `{key, equals}` on the field base) evaluated in `CardRenderer` to conditionally show a field, and un-stub the two 步骤引导 cards: aok-methods (uses `show_if` to branch by discipline) and corpus-hook (composes from existing primitives).

**Architecture:** `show_if` is a cross-cutting OPTIONAL attribute on the field `base` — NOT a new field type (no `FieldType` union change, no `fieldRegistry` change). `CardRenderer.StepFields` filters fields by `show_if` against current `values`. Zero store/envelope change.

**Tech Stack:** `packages/contracts` (Zod) + `apps/web` (React 18 + TS), Vitest + @testing-library/react. Packages: contracts `@mind-imprint/contracts`, web `web`.

## Global Constraints

- Gate per task: `pnpm -r typecheck` AND `pnpm -r test` green (full `-r` from Task 1 — no new FieldType, so no cross-package break).
- `show_if` schema: `z.object({ key: z.string().min(1), equals: z.string() }).optional()` added to the field `base` (applies to every primitive). Backward compatible — absent on existing cards; optional Zod field is omitted when undefined so existing `.toEqual` tests stay green.
- Evaluation is step-level ONLY (`CardRenderer.StepFields`): a field shows iff `!show_if || values[show_if.key] === show_if.equals`. Inside `repeatable_group` items `show_if` is NOT evaluated (documented; no card relies on it there).
- Registry stays 33; both cards become `body_status:"full"`; JSON inner quotes use 「」.

---

## Task 1: `show_if` modifier in contracts (field base)

**Files:**
- Modify: `packages/contracts/src/primitives.ts`
- Test: `packages/contracts/test/primitives.test.ts` (append a describe block)

**Interfaces:**
- Produces: every `FieldPrimitive`/`ItemField` member optionally carries `show_if?: { key: string; equals: string }`. No new exported schema name required (it lives in `base`), but EXPORT a `ShowIf` schema for reuse/testing.

- [ ] **Step 1: Write the failing test** (append)

```ts
import { ShowIf, FieldPrimitive } from "../src/primitives";

describe("show_if modifier", () => {
  it("ShowIf parses {key, equals}", () => {
    expect(ShowIf.parse({ key: "subject", equals: "数学" })).toEqual({ key: "subject", equals: "数学" });
  });
  it("a field may carry an optional show_if", () => {
    const f = { type: "textarea", key: "proof", label: "证明", show_if: { key: "subject", equals: "数学" } };
    expect(FieldPrimitive.parse(f)).toEqual(f);
  });
  it("a field without show_if still parses (backward compatible)", () => {
    const f = { type: "textarea", key: "x", label: "X" };
    expect(FieldPrimitive.parse(f)).toEqual(f);
  });
});
```

- [ ] **Step 2: Run, verify FAIL** — `pnpm --filter @mind-imprint/contracts test -- primitives` → FAIL (`ShowIf` undefined).

- [ ] **Step 3: Implement** — in `primitives.ts`, define `ShowIf` and add it to `base`:
```ts
export const ShowIf = z.object({ key: z.string().min(1), equals: z.string() });
const base = { key: z.string().min(1), label: z.string().min(1), show_if: ShowIf.optional() };
```
(Replace the existing `const base = {...}` line. Every primitive already spreads `...base`, so all gain optional `show_if`. No union changes.)

- [ ] **Step 4: Gate** — `pnpm -r typecheck && pnpm -r test` → PASS (full `-r`; no cross-package break since no new FieldType). Existing `.toEqual` field tests stay green (optional omitted when absent).

- [ ] **Step 5: Commit**
```bash
git add packages/contracts && git commit -m "feat(contracts): show_if optional field modifier on base"
```

---

## Task 2: `CardRenderer` evaluates `show_if`

**Files:**
- Modify: `apps/web/src/cards/CardRenderer.tsx`
- Test: `apps/web/src/cards/cardRenderer.showif.test.tsx` (create)

**Interfaces:**
- Consumes: `show_if` on fields (Task 1). `CardRenderer` props unchanged (`{ card, values, onField, onExpandStep }`).
- Produces: `StepFields` renders only fields whose `show_if` is absent or satisfied by `values`.

- [ ] **Step 1: Write the failing test**

```ts
import { describe, it, expect, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import { CardRenderer } from "./CardRenderer";
import type { CardSpec } from "@mind-imprint/contracts";

const card = {
  id: "t", category: "c", name: "n", purpose: "p", trigger_condition: "tc",
  rubric_tags: [], body_status: "full",
  steps: [{ key: "s", title: "S", disclose: "always", methodology_note: "",
    fields: [
      { type: "single_choice", key: "subject", label: "学科", options: ["数学", "艺术"] },
      { type: "textarea", key: "proof", label: "数学证明", show_if: { key: "subject", equals: "数学" } },
    ] }],
} as unknown as CardSpec;

describe("CardRenderer show_if", () => {
  it("hides a show_if field until its condition is met", () => {
    const { rerender } = render(<CardRenderer card={card} values={{}} onField={vi.fn()} onExpandStep={vi.fn()} />);
    expect(screen.queryByText("数学证明")).toBeNull();
    rerender(<CardRenderer card={card} values={{ subject: "数学" }} onField={vi.fn()} onExpandStep={vi.fn()} />);
    expect(screen.getByText("数学证明")).toBeTruthy();
  });
  it("keeps the field hidden when the value does not match", () => {
    render(<CardRenderer card={card} values={{ subject: "艺术" }} onField={vi.fn()} onExpandStep={vi.fn()} />);
    expect(screen.queryByText("数学证明")).toBeNull();
  });
});
```

- [ ] **Step 2: Run, verify FAIL** — the show_if field renders unconditionally → FAIL.

- [ ] **Step 3: Implement** — in `CardRenderer.tsx`, in `StepFields`, filter before mapping:
```tsx
function StepFields({ step, values, onField }: { step: Step; values: Record<string, unknown>; onField: Props["onField"] }) {
  const visible = step.fields.filter((field) => {
    const cond = (field as { show_if?: { key: string; equals: string } }).show_if;
    return !cond || values[cond.key] === cond.equals;
  });
  return (
    <div className="space-y-4">
      {visible.map((field) => {
        const Cmp = fieldRegistry[field.type];
        if (!Cmp) throw new Error(`No component registered for field type "${field.type}"`);
        return <Cmp key={field.key} field={field} value={values[field.key]} onChange={(v: unknown) => onField(field.key, v)} />;
      })}
    </div>
  );
}
```

- [ ] **Step 4: Gate** — `pnpm -r typecheck && pnpm --filter web test -- showif` → PASS. Run full `pnpm -r test` to confirm no regression (existing CardRenderer tests still green — fields without show_if always pass the filter).

- [ ] **Step 5: Commit**
```bash
git add apps/web && git commit -m "feat(cards): CardRenderer evaluates show_if field visibility"
```

---

## Task 3: Un-stub aok-methods + corpus-hook

**Files:**
- Modify: `packages/contracts/cards/aok-methods.json`, `packages/contracts/cards/corpus-hook.json`
- Test: `packages/contracts/test/steppedCards.test.ts` (create)

- [ ] **Step 1: Write the failing test**

```ts
import { describe, it, expect } from "vitest";
import { CardSpec } from "../src/cardSpec";
import aok from "../cards/aok-methods.json";
import corpus from "../cards/corpus-hook.json";

describe("stepped-guide cards un-stubbed", () => {
  it("aok-methods is full and has show_if fields gated on a subject single_choice", () => {
    const c = CardSpec.parse(aok);
    expect(c.body_status).toBe("full");
    const fields = c.steps.flatMap((s) => s.fields);
    expect(fields.some((f) => f.type === "single_choice" && f.key === "subject")).toBe(true);
    expect(fields.some((f) => (f as { show_if?: unknown }).show_if)).toBe(true);
  });
  it("corpus-hook is full and uses repeatable_group (existing primitives)", () => {
    const c = CardSpec.parse(corpus);
    expect(c.body_status).toBe("full");
    expect(c.steps.flatMap((s) => s.fields).some((f) => f.type === "repeatable_group")).toBe(true);
  });
});
```

- [ ] **Step 2: Run, verify FAIL.**

- [ ] **Step 3: Edit `aok-methods.json`** — set `"body_status":"full"`; `steps[0].fields`:
```json
[
  { "type": "single_choice", "key": "subject", "label": "你在用哪个学科的方法？", "options": ["数学", "人文社科", "艺术"] },
  { "type": "textarea", "key": "math_proof", "label": "区分「看了很多例子」（证据）和「证明了」——这条是哪种？", "show_if": { "key": "subject", "equals": "数学" } },
  { "type": "textarea", "key": "math_counter", "label": "试着找一个反例", "show_if": { "key": "subject", "equals": "数学" } },
  { "type": "textarea", "key": "math_axiom", "label": "它在什么公理 / 前提下成立？", "show_if": { "key": "subject", "equals": "数学" } },
  { "type": "textarea", "key": "human_five_q", "label": "对这个社科结论逐条过「五个问」：因果还是相关？样本？观察效应？测量？能否复制？", "show_if": { "key": "subject", "equals": "人文社科" } },
  { "type": "textarea", "key": "arts_evidence", "label": "作品里哪句 / 哪个细节支持这个解读？还是你脑补的？", "show_if": { "key": "subject", "equals": "艺术" } }
]
```
Keep all other JSON keys; if `methodology_note` has stale 「即将上线」 text, replace it with a faithful one-line note (e.g. 「选一个学科，按它『怎么算知道』的标准逐条过——数学要证明不是举例，社科有五个问，艺术要文本依据。」).

- [ ] **Step 4: Edit `corpus-hook.json`** — set `"body_status":"full"`; `steps[0].fields`:
```json
[
  { "type": "repeatable_group", "key": "hooks", "label": "锚点与钩子问题", "item_fields": [
    { "type": "text", "key": "locator", "label": "锚点位置（如 video@0:25 / 第3段）" },
    { "type": "textarea", "key": "q", "label": "这里的预埋问题" },
    { "type": "textarea", "key": "answer", "label": "你的回答" }
  ] },
  { "type": "repeatable_group", "key": "reading_cards", "label": "推荐的精问阅读卡（1-3 张）", "item_fields": [
    { "type": "text", "key": "material", "label": "材料" },
    { "type": "textarea", "key": "key_q", "label": "核心问题" },
    { "type": "textarea", "key": "evidence_point", "label": "证据点" },
    { "type": "textarea", "key": "counter", "label": "反方" },
    { "type": "textarea", "key": "extension", "label": "延伸" },
    { "type": "textarea", "key": "essay_angle", "label": "写作角度" }
  ] },
  { "type": "textarea", "key": "direction", "label": "你想往哪个方向深入？" }
]
```
Replace any stale 「即将上线」 `methodology_note` with a faithful one-line note.

- [ ] **Step 5: Gate** — `pnpm -r typecheck && pnpm -r test` → PASS (registry 33; if any test asserted these two were stub, update — search the 2 ids + `stub`).

- [ ] **Step 6: Commit**
```bash
git add packages/contracts && git commit -m "feat(cards): un-stub aok-methods (show_if) + corpus-hook (composed)"
```

---

## Self-Review

**Spec coverage:** `show_if` modifier (T1) + renderer evaluation (T2) + aok-methods & corpus-hook un-stubbed (T3). **Type consistency:** `ShowIf` `{key, equals}` ↔ JSON `show_if` ↔ renderer filter all agree. **Placeholder scan:** none. **Note:** no new FieldType, so no cross-package typecheck break — Task 1 gate is full `-r`. The renderer change is confined to `StepFields` (covers both always + on_demand steps). Refresh any stale 「即将上线」 methodology notes on the two cards.
