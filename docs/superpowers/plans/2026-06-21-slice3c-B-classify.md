# Slice 3c-B — `criteria_check` primitive Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: superpowers:subagent-driven-development. Steps use checkbox (`- [ ]`).

**Goal:** Add the `criteria_check` field primitive (a fixed list of named criteria, each judged on a small shared scale) and un-stub the two 分类标注 cards: science-knowing (uses `criteria_check`) and spin-detector (composes from existing primitives — no new primitive).

**Architecture:** `criteria_check` is added to the `FieldPrimitive` union (top-level only — not nested in `repeatable_group`, so no `ItemField`/`RepeatableGroupField` change) + a component + a `fieldRegistry` entry. spin-detector un-stubs with existing `repeatable_group`/`single_choice`/`textarea`. Zero `CardRenderer`/store/envelope change.

**Tech Stack:** `packages/contracts` (Zod) + `apps/web` (React 18 + TS + Tailwind `mk-*`), Vitest + @testing-library/react. Package names: contracts `@mind-imprint/contracts`, web `web`.

## Global Constraints

- Gate per task: `pnpm -r typecheck` AND `pnpm -r test` green (Task 1 is contracts-only — see its note). vitest does NOT typecheck — run `pnpm -r typecheck`.
- `criteria_check` value = `number[]` of length `criteria.length`; `value[i]` = chosen level index for `criteria[i]`; untouched = absent / `-1`. Immutable update (never mutate the input array — mirror `MultiChoiceField`).
- Added to `FieldPrimitive` ONLY (top-level). Existing primitives + envelope (`field_values: z.record(z.unknown())`) unchanged.
- Component conforms to `FieldProps<F> = { field, value, onChange }`; reuse the radio idiom (`role="radiogroup"`/`role="radio"`, `aria-checked`).
- Registry stays 33 cards; both cards become `body_status:"full"`; quotes inside JSON strings use 「」 not ASCII `"`.

---

## Task 1: `criteria_check` primitive in contracts

**Files:**
- Modify: `packages/contracts/src/primitives.ts`
- Test: `packages/contracts/test/primitives.test.ts` (add a describe block; file exists from 3c-A)

**Interfaces:**
- Produces: `CriteriaCheckField` Zod schema + type, exported; `criteria_check` accepted by `FieldPrimitive` (and `Step.fields`/`CardSpec`).

- [ ] **Step 1: Write the failing test** (append to `primitives.test.ts`)

```ts
import { CriteriaCheckField } from "../src/primitives";

describe("CriteriaCheckField", () => {
  const ok = { type: "criteria_check", key: "sci", label: "四标准", criteria: ["可证伪", "对照"], levels: ["满足", "不满足"] };
  it("parses a valid criteria_check field", () => {
    expect(CriteriaCheckField.parse(ok)).toEqual(ok);
  });
  it("requires at least 2 criteria and at least 2 levels", () => {
    expect(CriteriaCheckField.safeParse({ ...ok, criteria: ["只一个"] }).success).toBe(false);
    expect(CriteriaCheckField.safeParse({ ...ok, levels: ["只一个"] }).success).toBe(false);
  });
  it("is accepted by the FieldPrimitive union", () => {
    const { FieldPrimitive } = require("../src/primitives");
    expect(FieldPrimitive.parse(ok)).toEqual(ok);
  });
});
```
(If `require` is awkward under ESM, import `FieldPrimitive` at the top with the other imports instead.)

- [ ] **Step 2: Run, verify FAIL** — `pnpm --filter @mind-imprint/contracts test -- primitives` → FAIL (`CriteriaCheckField` undefined).

- [ ] **Step 3: Implement** — in `primitives.ts`, after `SpectrumField`:
```ts
export const CriteriaCheckField = z.object({
  type: z.literal("criteria_check"),
  ...base,
  criteria: z.array(z.string()).min(2),
  levels: z.array(z.string()).min(2),
});
```
Add `CriteriaCheckField` to the `FieldPrimitive` discriminated-union list (NOT `ItemField`).

- [ ] **Step 4: Gate (contracts-only)** — run `pnpm --filter @mind-imprint/contracts typecheck` + `pnpm --filter @mind-imprint/contracts test` → PASS. Do NOT run full `-r typecheck`: adding `"criteria_check"` to `FieldType` makes `apps/web`'s `fieldRegistry` `Record<FieldType,...>` require a `criteria_check` key, added in Task 2. That web break is EXPECTED here; note it in the report.

- [ ] **Step 5: Commit**
```bash
git add packages/contracts && git commit -m "feat(contracts): criteria_check field primitive"
```

---

## Task 2: `CriteriaCheckField` component + registry

**Files:**
- Create: `apps/web/src/cards/fields/CriteriaCheckField.tsx`
- Modify: `apps/web/src/cards/fieldRegistry.tsx`
- Test: `apps/web/src/cards/fields/criteriaCheckField.test.tsx`

**Interfaces:**
- Consumes: `FieldProps` from `./types`; `CriteriaCheckField as Schema` type from `@mind-imprint/contracts`.
- Produces: `CriteriaCheckField` component; `criteria_check` key in `fieldRegistry`.

- [ ] **Step 1: Write the failing test**

```ts
import { describe, it, expect, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { CriteriaCheckField } from "./CriteriaCheckField";
import { fieldRegistry } from "../fieldRegistry";

const field = { type: "criteria_check" as const, key: "sci", label: "四标准",
  criteria: ["可证伪", "对照", "可重复", "同行评审"], levels: ["满足", "部分", "不满足"] };

describe("CriteriaCheckField", () => {
  it("renders one row per criterion and `levels` radios each", () => {
    render(<CriteriaCheckField field={field} value={undefined} onChange={vi.fn()} />);
    expect(screen.getByText("可证伪")).toBeTruthy();
    expect(screen.getByText("同行评审")).toBeTruthy();
    expect(screen.getAllByRole("radio")).toHaveLength(4 * 3);
  });
  it("clicking a level reports an array with that criterion set, others -1, input not mutated", async () => {
    const onChange = vi.fn();
    const initial: number[] = [];
    render(<CriteriaCheckField field={field} value={initial} onChange={onChange} />);
    // click the 3rd level (index 2) of the 1st criterion (index 0)
    const group0 = screen.getAllByRole("radiogroup")[0]!;
    await userEvent.click(within(group0).getAllByRole("radio")[2]!);
    expect(onChange).toHaveBeenLastCalledWith([2, -1, -1, -1]);
    expect(initial).toEqual([]);
  });
  it("marks the stored level as checked", () => {
    render(<CriteriaCheckField field={field} value={[0, -1, -1, 2]} onChange={vi.fn()} />);
    const groups = screen.getAllByRole("radiogroup");
    expect(within(groups[0]!).getAllByRole("radio")[0]!.getAttribute("aria-checked")).toBe("true");
    expect(within(groups[3]!).getAllByRole("radio")[2]!.getAttribute("aria-checked")).toBe("true");
  });
  it("is registered under `criteria_check`", () => {
    expect(fieldRegistry.criteria_check).toBe(CriteriaCheckField);
  });
});
```
(Add `import { within } from "@testing-library/react";`.)

- [ ] **Step 2: Run, verify FAIL** — `pnpm --filter web test -- criteriaCheckField` → FAIL (module not found).

- [ ] **Step 3: Implement `CriteriaCheckField.tsx`**

```tsx
import type { FieldProps } from "./types";
import type { z } from "zod";
import type { CriteriaCheckField as Schema } from "@mind-imprint/contracts";

type F = z.infer<typeof Schema>;

export function CriteriaCheckField({ field, value, onChange }: FieldProps<F>) {
  const current: number[] = Array.isArray(value) ? (value as number[]) : [];
  const levelAt = (ci: number) => (typeof current[ci] === "number" ? current[ci]! : -1);

  function setLevel(ci: number, li: number) {
    const next = field.criteria.map((_, i) => (i === ci ? li : levelAt(i)));
    onChange(next);
  }

  return (
    <div>
      <span className="block text-[13px] font-semibold text-[#3A4256]">{field.label}</span>
      <div className="mt-3 space-y-2">
        {field.criteria.map((crit, ci) => (
          <div key={ci} className="flex items-center justify-between gap-3 rounded-[10px] border border-mk-border-2 px-3 py-2">
            <span className="text-[13px] text-mk-ink">{crit}</span>
            <div role="radiogroup" aria-label={crit} className="flex gap-1.5">
              {field.levels.map((lv, li) => (
                <button
                  key={li}
                  type="button"
                  role="radio"
                  aria-checked={levelAt(ci) === li}
                  aria-label={`${crit} ${lv}`}
                  onClick={() => setLevel(ci, li)}
                  className={`rounded-[8px] px-2.5 py-1 text-[12px] font-semibold ${levelAt(ci) === li ? "bg-mk-primary text-white" : "bg-[#F2F3F8] text-[#9AA1B0]"}`}
                >
                  {lv}
                </button>
              ))}
            </div>
          </div>
        ))}
      </div>
    </div>
  );
}
```

- [ ] **Step 4: Register** — in `fieldRegistry.tsx`, import `CriteriaCheckField` and add `criteria_check: CriteriaCheckField,`.

- [ ] **Step 5: Gate** — `pnpm -r typecheck && pnpm --filter web test -- criteriaCheckField` → PASS (full `-r typecheck` now green).

- [ ] **Step 6: Commit**
```bash
git add apps/web && git commit -m "feat(cards): CriteriaCheckField component + registry entry"
```

---

## Task 3: Un-stub science-knowing + spin-detector

**Files:**
- Modify: `packages/contracts/cards/science-knowing.json`, `packages/contracts/cards/spin-detector.json`
- Test: `packages/contracts/test/classifyCards.test.ts` (create)

**Interfaces:** JSON only + parse test. science-knowing uses `criteria_check`; spin-detector uses existing primitives.

- [ ] **Step 1: Write the failing test**

```ts
import { describe, it, expect } from "vitest";
import { CardSpec } from "../src/cardSpec";
import science from "../cards/science-knowing.json";
import spin from "../cards/spin-detector.json";

describe("classify cards un-stubbed", () => {
  it("science-knowing is full and has a criteria_check field", () => {
    const c = CardSpec.parse(science);
    expect(c.body_status).toBe("full");
    expect(c.steps.flatMap((s) => s.fields).some((f) => f.type === "criteria_check")).toBe(true);
  });
  it("spin-detector is full and uses repeatable_group + single_choice (no new primitive)", () => {
    const c = CardSpec.parse(spin);
    expect(c.body_status).toBe("full");
    const types = c.steps.flatMap((s) => s.fields).map((f) => f.type);
    expect(types).toContain("repeatable_group");
  });
});
```

- [ ] **Step 2: Run, verify FAIL.**

- [ ] **Step 3: Edit `science-knowing.json`** — set `"body_status":"full"`; `steps[0].fields`:
```json
[
  { "type": "text", "key": "claim", "label": "你要检验的主张是什么？" },
  { "type": "criteria_check", "key": "criteria", "label": "用四个标准逐条检验它", "criteria": ["可证伪", "有对照", "可重复", "经同行评审"], "levels": ["满足", "部分", "不满足"] },
  { "type": "textarea", "key": "contrast", "label": "对照一个伪科学样例：差别在哪？" },
  { "type": "textarea", "key": "tentative", "label": "满足标准 ≠ 永远正确。用一句话说明：这个结论目前有多可靠、什么会修正它？" }
]
```
Keep all other JSON keys (id/category/name/rubric_dims/trigger_*/related/step key/title/disclose/methodology_note).

- [ ] **Step 4: Edit `spin-detector.json`** — set `"body_status":"full"`; replace `steps` with two steps (both `disclose:"always"`):
```json
[
  { "key": "greenwash", "title": "漂绿核对：说的 vs 做的", "disclose": "always",
    "methodology_note": "把「它宣称什么」和「它实际做了什么/有什么证据」并排，看落差。",
    "fields": [
      { "type": "repeatable_group", "key": "pairs", "label": "说的 vs 做的", "item_fields": [
        { "type": "textarea", "key": "claim", "label": "它宣称什么" },
        { "type": "textarea", "key": "actual", "label": "它实际做了什么 / 证据" }
      ] }
    ] },
  { "key": "flicc", "title": "FLICC 套路标注", "disclose": "always",
    "methodology_note": "把可疑句子归到五类否认套路之一，并说明它符合这类的哪一点。",
    "fields": [
      { "type": "repeatable_group", "key": "tags", "label": "可疑句子 + 套路", "item_fields": [
        { "type": "textarea", "key": "snippet", "label": "可疑句子" },
        { "type": "single_choice", "key": "tactic", "label": "属于哪类套路", "options": ["假专家", "逻辑谬误", "不可能的标准", "挑拣证据", "阴谋论"] }
      ] },
      { "type": "textarea", "key": "persuasion", "label": "用一句话说明：这段在如何说服我？" }
    ] }
]
```

- [ ] **Step 5: Gate** — `pnpm -r typecheck && pnpm -r test` → PASS (registry still 33; if any test asserted these 2 were stub, update it — search the 2 ids + `stub`).

- [ ] **Step 6: Commit**
```bash
git add packages/contracts && git commit -m "feat(cards): un-stub science-knowing (criteria_check) + spin-detector (composed)"
```

---

## Self-Review

**Spec coverage:** `criteria_check` primitive (T1) + component/registry (T2) + science-knowing & spin-detector un-stubbed (T3). **Type consistency:** `CriteriaCheckField` schema/type ↔ `criteria_check` registry key ↔ JSON `"type":"criteria_check"`; value is `number[]` everywhere. **Placeholder scan:** none. **Note:** Task 1 gate is contracts-only (web typecheck red until Task 2 adds the registry key — same pattern as 3c-A). spin-detector adds NO new primitive (existing repeatable_group/single_choice/textarea).
