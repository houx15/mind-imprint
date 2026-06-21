# Slice 3c-A — `spectrum` primitive Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: superpowers:subagent-driven-development. Steps use checkbox (`- [ ]`).

**Goal:** Add one new schema-driven field primitive `spectrum` (a labeled horizontal axis the student positions a marker on) and un-stub the two 量表光谱 cards (certainty-spectrum, belief-spectrum) to real interactions.

**Architecture:** Additive to the `FieldPrimitive` discriminated union (and `ItemField`, so it works inside `repeatable_group`); a new field component registered in `fieldRegistry`; the two cards' JSON flipped `stub→full`. Zero `CardRenderer`/store/envelope changes.

**Tech Stack:** `packages/contracts` (Zod) + `apps/web` (React 18 + TS + Tailwind `mk-*` tokens), Vitest + @testing-library/react.

## Global Constraints

- Gate per task: `pnpm -r typecheck` AND `pnpm -r test` green. vitest does NOT typecheck — run `pnpm -r typecheck` explicitly.
- `spectrum` value semantics: `field_values[key]` = integer index `0..stops.length-1`; untouched = not a number (`-1` sentinel in the component). `stops` array has min length 2; first/last are the poles.
- Add `SpectrumField` to BOTH `ItemField` and `FieldPrimitive` unions (belief-spectrum nests it in `repeatable_group`).
- Component conforms to `FieldProps<F> = { field, value, onChange }`; mirror the `RatingField` radiogroup idiom (`role="radiogroup"` + `role="radio"` stops, `aria-checked`, `onClick → onChange(index)`). Accessible + testable; pointer-drag is a future enhancement (out of scope).
- Registry stays 33 cards; all existing tests stay green; the two cards become `body_status:"full"`.

---

## Task 1: `spectrum` primitive in contracts

**Files:**
- Modify: `packages/contracts/src/primitives.ts`
- Test: `packages/contracts/test/primitives.test.ts` (create if absent; else add a describe block)

**Interfaces:**
- Produces: `SpectrumField` Zod schema + type, exported from `@mind-imprint/contracts`; `spectrum` accepted by `ItemField`, `FieldPrimitive`, and (transitively) `Step.fields` / `CardSpec`.

- [ ] **Step 1: Write the failing test**

```ts
import { describe, it, expect } from "vitest";
import { SpectrumField, FieldPrimitive, ItemField, RepeatableGroupField } from "../src/primitives";

describe("SpectrumField", () => {
  const ok = { type: "spectrum", key: "pos", label: "位置", stops: ["低", "中", "高"] };
  it("parses a valid spectrum field", () => {
    expect(SpectrumField.parse(ok)).toEqual(ok);
  });
  it("requires at least 2 stops", () => {
    expect(SpectrumField.safeParse({ ...ok, stops: ["只有一个"] }).success).toBe(false);
  });
  it("is accepted by the FieldPrimitive union", () => {
    expect(FieldPrimitive.parse(ok)).toEqual(ok);
  });
  it("is accepted as an ItemField (usable inside repeatable_group)", () => {
    expect(ItemField.parse(ok)).toEqual(ok);
    const group = { type: "repeatable_group", key: "g", label: "G", item_fields: [ok] };
    expect(RepeatableGroupField.parse(group)).toEqual(group);
  });
});
```

- [ ] **Step 2: Run, verify FAIL** — `pnpm --filter @mind-imprint/contracts test -- primitives` → FAIL (`SpectrumField` undefined). (If the package filter name differs, the implementer reads `packages/contracts/package.json` `name` and uses it; or run `pnpm -r test`.)

- [ ] **Step 3: Implement** — in `packages/contracts/src/primitives.ts`, after `LinkCheckField`:

```ts
export const SpectrumField = z.object({ type: z.literal("spectrum"), ...base, stops: z.array(z.string()).min(2) });
```

Add `SpectrumField` to the `ItemField` union AND the `FieldPrimitive` union (both `z.discriminatedUnion("type", [...])` lists). Final unions:

```ts
export const ItemField = z.discriminatedUnion("type", [
  TextField, TextAreaField, SingleChoiceField, MultiChoiceField, RatingField, LinkCheckField, SpectrumField,
]);

export const FieldPrimitive = z.discriminatedUnion("type", [
  TextField, TextAreaField, SingleChoiceField, MultiChoiceField, RatingField, LinkCheckField, SpectrumField, RepeatableGroupField,
]);
```

- [ ] **Step 4: Run tests + typecheck** → PASS. (`FieldType` union now includes `"spectrum"`, which will make `fieldRegistry`'s `Record<FieldType, ...>` require a `spectrum` entry — that is added in Task 2; if Task 1 is committed alone, `apps/web` typecheck would fail on the missing key, so commit Task 1 together with Task 2's registry entry OR temporarily it's fine because contracts typecheck is independent. To keep BOTH packages green at the Task 1 commit, ALSO add the registry entry now is NOT allowed — Task 1 scope is contracts only. Resolution: run only `pnpm --filter @mind-imprint/contracts typecheck && pnpm --filter @mind-imprint/contracts test` for Task 1's gate; the full `pnpm -r typecheck` will go green at the end of Task 2. Note this in the commit.)

- [ ] **Step 5: Commit**
```bash
git add packages/contracts && git commit -m "feat(contracts): spectrum field primitive (ItemField + FieldPrimitive)"
```

---

## Task 2: `SpectrumField` component + registry

**Files:**
- Create: `apps/web/src/cards/fields/SpectrumField.tsx`
- Modify: `apps/web/src/cards/fieldRegistry.tsx`
- Test: `apps/web/src/cards/fields/spectrumField.test.tsx`

**Interfaces:**
- Consumes: `FieldProps` from `./types`; `SpectrumField as Schema` type from `@mind-imprint/contracts`.
- Produces: `SpectrumField` component; `spectrum` key in `fieldRegistry`.

- [ ] **Step 1: Write the failing test**

```ts
import { describe, it, expect, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { SpectrumField } from "./SpectrumField";
import { fieldRegistry } from "../fieldRegistry";

const field = { type: "spectrum" as const, key: "pos", label: "确定度", stops: ["个人猜测", "有据推断", "强证据", "科学共识", "逻辑必然"] };

describe("SpectrumField", () => {
  it("renders one radio per stop with its label", () => {
    render(<SpectrumField field={field} value={undefined} onChange={vi.fn()} />);
    const stops = screen.getAllByRole("radio");
    expect(stops).toHaveLength(5);
    expect(screen.getByText("科学共识")).toBeTruthy();
  });
  it("clicking a stop reports its index", async () => {
    const onChange = vi.fn();
    render(<SpectrumField field={field} value={undefined} onChange={onChange} />);
    await userEvent.click(screen.getAllByRole("radio")[2]!);
    expect(onChange).toHaveBeenCalledWith(2);
  });
  it("marks the current value as checked", () => {
    render(<SpectrumField field={field} value={3} onChange={vi.fn()} />);
    expect(screen.getAllByRole("radio")[3]!.getAttribute("aria-checked")).toBe("true");
  });
  it("ArrowRight moves the marker forward and clamps at the last stop", async () => {
    const onChange = vi.fn();
    render(<SpectrumField field={field} value={4} onChange={onChange} />);
    screen.getByRole("radiogroup").focus();
    await userEvent.keyboard("{ArrowRight}");
    expect(onChange).toHaveBeenLastCalledWith(4); // clamped at max
  });
  it("ArrowLeft from index 2 reports 1", async () => {
    const onChange = vi.fn();
    render(<SpectrumField field={field} value={2} onChange={onChange} />);
    screen.getByRole("radiogroup").focus();
    await userEvent.keyboard("{ArrowLeft}");
    expect(onChange).toHaveBeenLastCalledWith(1);
  });
  it("is registered under the `spectrum` key", () => {
    expect(fieldRegistry.spectrum).toBe(SpectrumField);
  });
});
```

- [ ] **Step 2: Run, verify FAIL** — `pnpm --filter @mind-imprint/web test -- spectrumField` → FAIL (module not found). (Filter name per `apps/web/package.json` `name`; or `pnpm -r test`.)

- [ ] **Step 3: Implement `SpectrumField.tsx`**

```tsx
import type { FieldProps } from "./types";
import type { z } from "zod";
import type { SpectrumField as Schema } from "@mind-imprint/contracts";

type F = z.infer<typeof Schema>;

export function SpectrumField({ field, value, onChange }: FieldProps<F>) {
  const max = field.stops.length - 1;
  const index = typeof value === "number" ? value : -1;
  const clamp = (n: number) => Math.max(0, Math.min(max, n));

  function onKeyDown(e: React.KeyboardEvent) {
    const from = index < 0 ? 0 : index;
    if (e.key === "ArrowRight" || e.key === "ArrowUp") { e.preventDefault(); onChange(clamp(from + 1)); }
    else if (e.key === "ArrowLeft" || e.key === "ArrowDown") { e.preventDefault(); onChange(clamp(from - 1)); }
    else if (e.key === "Home") { e.preventDefault(); onChange(0); }
    else if (e.key === "End") { e.preventDefault(); onChange(max); }
  }

  return (
    <div>
      <span className="block text-[13px] font-semibold text-[#3A4256]">{field.label}</span>
      <div
        role="radiogroup"
        aria-label={field.label}
        tabIndex={0}
        onKeyDown={onKeyDown}
        className="relative mt-3 flex items-start outline-none"
      >
        {/* connecting track behind the dots */}
        <div className="pointer-events-none absolute left-[10%] right-[10%] top-[7px] h-[2px] bg-[#EEF0F4]" aria-hidden="true" />
        {field.stops.map((stop, i) => (
          <button
            key={i}
            type="button"
            role="radio"
            aria-checked={index === i}
            aria-label={stop}
            tabIndex={-1}
            onClick={() => onChange(i)}
            className="relative z-[1] flex flex-1 flex-col items-center gap-1.5 bg-transparent"
          >
            <span className={`h-3.5 w-3.5 rounded-full ${index === i ? "bg-mk-primary" : "bg-[#D9DDE7]"}`} />
            <span className={`text-center text-[11.5px] leading-tight ${index === i ? "font-semibold text-mk-ink" : "text-[#9AA1B0]"}`}>{stop}</span>
          </button>
        ))}
      </div>
    </div>
  );
}
```

- [ ] **Step 4: Register** — in `apps/web/src/cards/fieldRegistry.tsx`, import `SpectrumField` and add `spectrum: SpectrumField,` to the `fieldRegistry` object.

- [ ] **Step 5: Run tests + full typecheck** → `pnpm -r typecheck && pnpm --filter @mind-imprint/web test -- spectrumField` PASS. The full `pnpm -r typecheck` now goes green (the `Record<FieldType,...>` has its `spectrum` entry).

- [ ] **Step 6: Commit**
```bash
git add apps/web && git commit -m "feat(cards): SpectrumField component + registry entry"
```

---

## Task 3: Un-stub certainty-spectrum + belief-spectrum

**Files:**
- Modify: `packages/contracts/cards/certainty-spectrum.json`, `packages/contracts/cards/belief-spectrum.json`
- Test: `packages/contracts/test/spectrumCards.test.ts` (create)

**Interfaces:**
- Consumes: the `spectrum` primitive (Task 1). No code change — JSON only + a parse test.

- [ ] **Step 1: Write the failing test**

```ts
import { describe, it, expect } from "vitest";
import { CardSpec } from "../src/cardSpec";
import certainty from "../cards/certainty-spectrum.json";
import belief from "../cards/belief-spectrum.json";

describe("spectrum cards un-stubbed", () => {
  it("certainty-spectrum is full and parses with a spectrum field", () => {
    const c = CardSpec.parse(certainty);
    expect(c.body_status).toBe("full");
    const types = c.steps[0]!.fields.map((f) => f.type);
    expect(types).toContain("spectrum");
  });
  it("belief-spectrum is full and nests a spectrum inside a repeatable_group", () => {
    const c = CardSpec.parse(belief);
    expect(c.body_status).toBe("full");
    const group = c.steps[0]!.fields.find((f) => f.type === "repeatable_group");
    expect(group).toBeTruthy();
    // @ts-expect-error narrow at runtime
    expect(group.item_fields.some((f: { type: string }) => f.type === "spectrum")).toBe(true);
  });
});
```

- [ ] **Step 2: Run, verify FAIL** — FAIL (cards still `stub`, no spectrum field).

- [ ] **Step 3: Edit `certainty-spectrum.json`** — set `"body_status": "full"` and replace `steps[0].fields` with:
```json
[
  { "type": "text", "key": "claim", "label": "你的结论是什么？" },
  { "type": "spectrum", "key": "position", "label": "把它放到确定度光谱上", "stops": ["个人猜测", "有据推断", "强证据", "科学共识", "逻辑必然"] },
  { "type": "textarea", "key": "why", "label": "为什么是这个位置？支撑它的证据类型是什么？" },
  { "type": "textarea", "key": "rewrite", "label": "用与该位置相称的语气词重写结论句（可能 / 大概 / 很可能 / 几乎确定 / 必然）" }
]
```
Keep `methodology_note` (or refine to the card's own); keep all other fields (rubric_dims, etc.). Use 「」 for any quotes inside JSON strings — never ASCII double-quotes.

- [ ] **Step 4: Edit `belief-spectrum.json`** — set `"body_status": "full"` and replace `steps[0].fields` with:
```json
[
  { "type": "text", "key": "issue", "label": "争议议题是什么？" },
  { "type": "repeatable_group", "key": "stances", "label": "把各方立场放到光谱上", "item_fields": [
    { "type": "text", "key": "who", "label": "是谁 / 哪个立场" },
    { "type": "spectrum", "key": "position", "label": "在光谱上的位置", "stops": ["这一极", "偏这边", "中间", "偏那边", "那一极"] },
    { "type": "textarea", "key": "believes", "label": "它相信什么" },
    { "type": "textarea", "key": "evidence", "label": "它引什么证据" },
    { "type": "textarea", "key": "interest", "label": "背后有什么利益 / 背景" }
  ] },
  { "type": "spectrum", "key": "self", "label": "我现在站这里", "stops": ["这一极", "偏这边", "中间", "偏那边", "那一极"] },
  { "type": "textarea", "key": "self_reason", "label": "我站这里的理由（他们的分歧在证据、价值，还是利益？）" }
]
```

- [ ] **Step 5: Run tests + full gate** → `pnpm -r typecheck && pnpm -r test` PASS (incl. registry still 33, Harness/registry count tests green). If any existing test asserted these two were `stub`, update it to reflect `full` (search `belief-spectrum`/`certainty-spectrum` + `stub`).

- [ ] **Step 6: Commit**
```bash
git add packages/contracts && git commit -m "feat(cards): un-stub certainty-spectrum + belief-spectrum to spectrum interaction"
```

---

## Self-Review

**Spec coverage:** `spectrum` primitive (T1) + component/registry (T2) + both cards un-stubbed (T3) = the full 3c-A spec. **Type consistency:** `SpectrumField` schema/type and the `spectrum` registry key and the JSON `"type":"spectrum"` all agree; value is an integer index everywhere. **Placeholder scan:** none. **Note for implementer:** package filter names — read `packages/contracts/package.json` and `apps/web/package.json` `name` fields for the exact `pnpm --filter` targets, or just use `pnpm -r test` / `pnpm -r typecheck`. Task 1's gate is contracts-only typecheck (full `-r` typecheck goes green after Task 2 adds the registry key).
