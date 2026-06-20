# Slice 1 — Card Contract + Runtime (B0 + B1) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the immutable card-contract spine (B0) and a schema-driven card renderer (B1) that exercises it — pure frontend, fake data, no LLM, no DB.

**Architecture:** A pnpm monorepo. `packages/contracts` holds Zod schemas (field primitives, card spec, standard envelope, event trace) as the single source of truth, plus a registry loader and the two card JSONs. `apps/web` renders any card from its JSON through one `<CardRenderer>` (a `type → component` registry, zero card-specific branches), drives a pure `envelopeReducer` for event capture, and ships a dev harness to walk both cards through the three visual states.

**Tech Stack:** pnpm workspaces, TypeScript 5, Zod 3, React 18, Vite 5, Tailwind 3, Vitest + @testing-library/react (jsdom).

## Global Constraints

- **Single source of truth:** every card/envelope shape is defined once in `packages/contracts` and imported by `apps/web`. Never redefine a type in `apps/web`. (PRD §16)
- **Schema-driven renderer:** `<CardRenderer>` and field components MUST contain zero card-specific branches (no `card.id === ...`). Adding a card = adding JSON only. (PRD §16) — enforced by a test in Task 9.
- **Standard envelope is frozen:** the `CardInstance` shape (Task 4) is the spine for tree/calendar/eval in later slices. Do not add/rename fields casually. (PRD §9, §16)
- **No persistence / no LLM in this slice.** Envelope is an in-memory object only.
- **Visual source of truth:** `docs/design/思维印记_工作区.dc.html`. All styling (colors, radii, the bottom sheet, field looks, the three states) is lifted from it. Design tokens are in `apps/web/tailwind.config.ts`.
- **Package names:** contracts package is `@mind-imprint/contracts`; web app package is `web`.
- **Fake data:** use the real Phoebe content from the design (公众号 → NASA / Nature Sustainability IF 32.1; 论点 "中国很大程度让地球更可持续" vs 反例 "中国碳排放全球第一"). Never lorem ipsum.

---

### Task 1: Monorepo + toolchain scaffold

**Files:**
- Create: `pnpm-workspace.yaml`
- Create: `package.json`
- Create: `tsconfig.base.json`
- Create: `packages/contracts/package.json`
- Create: `packages/contracts/tsconfig.json`
- Create: `packages/contracts/vitest.config.ts`
- Create: `packages/contracts/src/index.ts`
- Create: `packages/contracts/test/smoke.test.ts`
- Create: `apps/web/package.json`
- Create: `apps/web/tsconfig.json`
- Create: `apps/web/vite.config.ts`
- Create: `apps/web/vitest.config.ts`
- Create: `apps/web/tailwind.config.ts`
- Create: `apps/web/postcss.config.js`
- Create: `apps/web/index.html`
- Create: `apps/web/src/main.tsx`
- Create: `apps/web/src/index.css`
- Create: `apps/web/src/test/setup.ts`
- Create: `apps/web/src/test/smoke.test.tsx`

**Interfaces:**
- Consumes: nothing.
- Produces: a working `pnpm -r test`; the `@mind-imprint/contracts` workspace dependency resolvable from `apps/web`; Tailwind `mk-*` design tokens.

- [ ] **Step 1: Create the workspace + root manifests**

`pnpm-workspace.yaml`:
```yaml
packages:
  - "packages/*"
  - "apps/*"
```

`package.json`:
```json
{
  "name": "mind-imprint",
  "private": true,
  "version": "0.0.0",
  "scripts": {
    "test": "pnpm -r test"
  },
  "devDependencies": {
    "typescript": "^5.4.0"
  }
}
```

`tsconfig.base.json`:
```json
{
  "compilerOptions": {
    "target": "ES2021",
    "module": "ESNext",
    "moduleResolution": "Bundler",
    "lib": ["ES2021", "DOM", "DOM.Iterable"],
    "strict": true,
    "esModuleInterop": true,
    "skipLibCheck": true,
    "resolveJsonModule": true,
    "forceConsistentCasingInFileNames": true,
    "noUncheckedIndexedAccess": true,
    "jsx": "react-jsx"
  }
}
```

- [ ] **Step 2: Scaffold the contracts package**

`packages/contracts/package.json`:
```json
{
  "name": "@mind-imprint/contracts",
  "version": "0.0.0",
  "type": "module",
  "main": "src/index.ts",
  "types": "src/index.ts",
  "exports": { ".": "./src/index.ts" },
  "scripts": {
    "test": "vitest run"
  },
  "dependencies": {
    "zod": "^3.23.0"
  },
  "devDependencies": {
    "vitest": "^1.6.0"
  }
}
```

`packages/contracts/tsconfig.json`:
```json
{
  "extends": "../../tsconfig.base.json",
  "compilerOptions": { "rootDir": "." },
  "include": ["src", "test"]
}
```

`packages/contracts/vitest.config.ts`:
```ts
import { defineConfig } from "vitest/config";

export default defineConfig({
  test: { environment: "node", include: ["test/**/*.test.ts"] },
});
```

`packages/contracts/src/index.ts`:
```ts
export const CONTRACTS_VERSION = "0.0.0";
```

`packages/contracts/test/smoke.test.ts`:
```ts
import { describe, it, expect } from "vitest";
import { CONTRACTS_VERSION } from "../src/index";

describe("contracts smoke", () => {
  it("exposes a version", () => {
    expect(CONTRACTS_VERSION).toBe("0.0.0");
  });
});
```

- [ ] **Step 3: Scaffold the web app**

`apps/web/package.json`:
```json
{
  "name": "web",
  "version": "0.0.0",
  "private": true,
  "type": "module",
  "scripts": {
    "dev": "vite",
    "build": "vite build",
    "test": "vitest run"
  },
  "dependencies": {
    "@mind-imprint/contracts": "workspace:*",
    "react": "^18.3.0",
    "react-dom": "^18.3.0"
  },
  "devDependencies": {
    "@testing-library/jest-dom": "^6.4.0",
    "@testing-library/react": "^16.0.0",
    "@testing-library/user-event": "^14.5.0",
    "@vitejs/plugin-react": "^4.3.0",
    "autoprefixer": "^10.4.0",
    "jsdom": "^24.1.0",
    "postcss": "^8.4.0",
    "tailwindcss": "^3.4.0",
    "vite": "^5.3.0",
    "vitest": "^1.6.0"
  }
}
```

`apps/web/tsconfig.json`:
```json
{
  "extends": "../../tsconfig.base.json",
  "compilerOptions": { "rootDir": "." },
  "include": ["src"]
}
```

`apps/web/vite.config.ts`:
```ts
import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";

export default defineConfig({ plugins: [react()] });
```

`apps/web/vitest.config.ts`:
```ts
import { defineConfig } from "vitest/config";
import react from "@vitejs/plugin-react";

export default defineConfig({
  plugins: [react()],
  test: {
    environment: "jsdom",
    globals: true,
    setupFiles: ["./src/test/setup.ts"],
    include: ["src/**/*.test.{ts,tsx}"],
  },
});
```

`apps/web/tailwind.config.ts` (tokens lifted from the design HTML):
```ts
import type { Config } from "tailwindcss";

export default {
  content: ["./index.html", "./src/**/*.{ts,tsx}"],
  theme: {
    extend: {
      colors: {
        mk: {
          primary: "#2A3B7A",
          "primary-hover": "#22305F",
          "primary-tint": "#EDEFF9",
          accent: "#D98263",
          "accent-hover": "#CC7355",
          "accent-tint": "#FBEEE7",
          green: "#4C9A82",
          "green-tint": "#E7F3EE",
          amber: "#E8A33D",
          ink: "#1C2333",
          muted: "#8A92A3",
          "muted-2": "#9AA1B0",
          bg: "#F3F4F8",
          surface: "#FFFFFF",
          border: "#EAECF2",
          "border-2": "#ECEEF3",
          input: "#E1E4ED",
          "input-bg": "#FCFCFD",
        },
      },
      borderRadius: { mk: "14px", "mk-lg": "18px", "mk-sheet": "22px" },
      fontFamily: { sans: ["Plus Jakarta Sans", "Noto Sans SC", "system-ui", "sans-serif"] },
    },
  },
  plugins: [],
} satisfies Config;
```

`apps/web/postcss.config.js`:
```js
export default { plugins: { tailwindcss: {}, autoprefixer: {} } };
```

`apps/web/index.html`:
```html
<!doctype html>
<html lang="zh">
  <head>
    <meta charset="utf-8" />
    <meta name="viewport" content="width=device-width, initial-scale=1" />
    <title>思维印记 · Card Runtime Harness</title>
  </head>
  <body>
    <div id="root"></div>
    <script type="module" src="/src/main.tsx"></script>
  </body>
</html>
```

`apps/web/src/index.css`:
```css
@tailwind base;
@tailwind components;
@tailwind utilities;
```

`apps/web/src/main.tsx`:
```tsx
import React from "react";
import { createRoot } from "react-dom/client";
import "./index.css";

createRoot(document.getElementById("root")!).render(
  <React.StrictMode>
    <div className="p-6 font-sans text-mk-ink">思维印记 harness — pending Task 13</div>
  </React.StrictMode>,
);
```

`apps/web/src/test/setup.ts`:
```ts
import "@testing-library/jest-dom/vitest";
```

`apps/web/src/test/smoke.test.tsx`:
```tsx
import { render, screen } from "@testing-library/react";

function Hello() {
  return <span>hi</span>;
}

it("renders", () => {
  render(<Hello />);
  expect(screen.getByText("hi")).toBeInTheDocument();
});
```

- [ ] **Step 4: Install dependencies**

Run: `pnpm install`
Expected: resolves workspaces, links `@mind-imprint/contracts` into `apps/web`, no errors.

- [ ] **Step 5: Run both test suites (verify toolchain green)**

Run: `pnpm -r test`
Expected: PASS — contracts smoke (1 test) and web smoke (1 test) both pass.

- [ ] **Step 6: Commit**

```bash
git add -A
git commit -m "chore: scaffold pnpm monorepo (contracts + web, vitest, tailwind tokens)"
```

---

### Task 2: Field primitive schemas (B0)

**Files:**
- Create: `packages/contracts/src/primitives.ts`
- Test: `packages/contracts/test/primitives.test.ts`
- Modify: `packages/contracts/src/index.ts`

**Interfaces:**
- Consumes: `zod`.
- Produces: `FieldPrimitive` (Zod schema + inferred type), `FieldType` union (`"text" | "textarea" | "single_choice" | "multi_choice" | "rating" | "repeatable_group" | "link_check"`), and the per-type schemas.

- [ ] **Step 1: Write the failing test**

`packages/contracts/test/primitives.test.ts`:
```ts
import { describe, it, expect } from "vitest";
import { FieldPrimitive } from "../src/primitives";

describe("FieldPrimitive", () => {
  it("accepts a textarea field", () => {
    expect(FieldPrimitive.safeParse({ type: "textarea", key: "stop", label: "Stop" }).success).toBe(true);
  });
  it("accepts single_choice with options", () => {
    expect(FieldPrimitive.safeParse({ type: "single_choice", key: "v", label: "可信？", options: ["可信", "存疑"] }).success).toBe(true);
  });
  it("rejects single_choice with empty options", () => {
    expect(FieldPrimitive.safeParse({ type: "single_choice", key: "v", label: "x", options: [] }).success).toBe(false);
  });
  it("accepts rating with a scale", () => {
    expect(FieldPrimitive.safeParse({ type: "rating", key: "c", label: "Currency", scale: 5 }).success).toBe(true);
  });
  it("accepts a repeatable_group of simple item_fields", () => {
    const ok = FieldPrimitive.safeParse({
      type: "repeatable_group", key: "sources", label: "来源",
      item_fields: [{ type: "text", key: "name", label: "来源" }],
    });
    expect(ok.success).toBe(true);
  });
  it("rejects an unknown field type", () => {
    expect(FieldPrimitive.safeParse({ type: "slider", key: "x", label: "x" }).success).toBe(false);
  });
});
```

- [ ] **Step 2: Run test to verify it fails**

Run: `pnpm --filter @mind-imprint/contracts test`
Expected: FAIL — cannot resolve `../src/primitives`.

- [ ] **Step 3: Write minimal implementation**

`packages/contracts/src/primitives.ts`:
```ts
import { z } from "zod";

const base = { key: z.string().min(1), label: z.string().min(1) };

export const TextField = z.object({ type: z.literal("text"), ...base });
export const TextAreaField = z.object({ type: z.literal("textarea"), ...base, rows: z.number().int().positive().optional() });
export const SingleChoiceField = z.object({ type: z.literal("single_choice"), ...base, options: z.array(z.string()).min(1) });
export const MultiChoiceField = z.object({ type: z.literal("multi_choice"), ...base, options: z.array(z.string()).min(1) });
export const RatingField = z.object({ type: z.literal("rating"), ...base, scale: z.number().int().positive() });
export const LinkCheckField = z.object({ type: z.literal("link_check"), ...base });

// item_fields cannot themselves be repeatable (no nesting)
export const ItemField = z.discriminatedUnion("type", [
  TextField, TextAreaField, SingleChoiceField, MultiChoiceField, RatingField, LinkCheckField,
]);

export const RepeatableGroupField = z.object({
  type: z.literal("repeatable_group"),
  ...base,
  item_fields: z.array(ItemField).min(1),
});

export const FieldPrimitive = z.discriminatedUnion("type", [
  TextField, TextAreaField, SingleChoiceField, MultiChoiceField, RatingField, LinkCheckField, RepeatableGroupField,
]);

export type FieldPrimitive = z.infer<typeof FieldPrimitive>;
export type ItemField = z.infer<typeof ItemField>;
export type FieldType = FieldPrimitive["type"];
```

- [ ] **Step 4: Run test to verify it passes**

Run: `pnpm --filter @mind-imprint/contracts test`
Expected: PASS (6 assertions).

- [ ] **Step 5: Re-export from index + commit**

Append to `packages/contracts/src/index.ts`:
```ts
export * from "./primitives";
```

```bash
git add -A
git commit -m "feat(contracts): field primitive schemas"
```

---

### Task 3: Card spec schema (B0)

**Files:**
- Create: `packages/contracts/src/cardSpec.ts`
- Create: `packages/contracts/src/rubric.ts`
- Test: `packages/contracts/test/cardSpec.test.ts`
- Modify: `packages/contracts/src/index.ts`

**Interfaces:**
- Consumes: `FieldPrimitive` from Task 2.
- Produces: `CardSpec`, `Step` (Zod + types); `RUBRIC_TAGS` constant list.

- [ ] **Step 1: Write the failing test**

`packages/contracts/test/cardSpec.test.ts`:
```ts
import { describe, it, expect } from "vitest";
import { CardSpec } from "../src/cardSpec";

const valid = {
  id: "concession", category: "知识工具", name: "让步段", purpose: "以退为进",
  trigger_condition: "出现反例却想忽略", rubric_tags: ["D5_论证结构"],
  steps: [{
    key: "concession", title: "让步段四步", disclose: "always", methodology_note: "先退一步再反驳",
    fields: [{ type: "text", key: "thesis", label: "中心论点" }],
  }],
};

describe("CardSpec", () => {
  it("accepts a valid card", () => {
    expect(CardSpec.safeParse(valid).success).toBe(true);
  });
  it("rejects an invalid disclose value", () => {
    const bad = { ...valid, steps: [{ ...valid.steps[0], disclose: "sometimes" }] };
    expect(CardSpec.safeParse(bad).success).toBe(false);
  });
  it("rejects a step with no fields", () => {
    const bad = { ...valid, steps: [{ ...valid.steps[0], fields: [] }] };
    expect(CardSpec.safeParse(bad).success).toBe(false);
  });
});
```

- [ ] **Step 2: Run test to verify it fails**

Run: `pnpm --filter @mind-imprint/contracts test`
Expected: FAIL — cannot resolve `../src/cardSpec`.

- [ ] **Step 3: Write minimal implementation**

`packages/contracts/src/cardSpec.ts`:
```ts
import { z } from "zod";
import { FieldPrimitive } from "./primitives";

export const Step = z.object({
  key: z.string().min(1),
  title: z.string().min(1),
  disclose: z.enum(["always", "on_demand"]),
  methodology_note: z.string(),
  fields: z.array(FieldPrimitive).min(1),
});

export const CardSpec = z.object({
  id: z.string().min(1),
  category: z.string().min(1),
  name: z.string().min(1),
  purpose: z.string(),
  trigger_condition: z.string(),
  steps: z.array(Step).min(1),
  rubric_tags: z.array(z.string()),
});

export type Step = z.infer<typeof Step>;
export type CardSpec = z.infer<typeof CardSpec>;
```

`packages/contracts/src/rubric.ts` (the demo subset, PRD §11):
```ts
export const RUBRIC_TAGS = [
  "D1_来源意识",
  "D2_交叉验证",
  "D5_论证结构",
  "D7_对立观点处理",
] as const;

export type RubricTag = (typeof RUBRIC_TAGS)[number];
```

- [ ] **Step 4: Run test to verify it passes**

Run: `pnpm --filter @mind-imprint/contracts test`
Expected: PASS.

- [ ] **Step 5: Re-export + commit**

Append to `packages/contracts/src/index.ts`:
```ts
export * from "./cardSpec";
export * from "./rubric";
```

```bash
git add -A
git commit -m "feat(contracts): card spec + rubric tag constants"
```

---

### Task 4: Standard envelope + event trace (B0)

**Files:**
- Create: `packages/contracts/src/envelope.ts`
- Test: `packages/contracts/test/envelope.test.ts`
- Modify: `packages/contracts/src/index.ts`

**Interfaces:**
- Consumes: `zod`.
- Produces: `CardInstance`, `TraceEvent`, `CardStatus` (Zod + types). `CardInstance` fields: `id, card_id, task_id, parent_node_id (string|null), status, field_values (Record<string,unknown>), event_trace (TraceEvent[]), rubric_tags (string[]), created_at (string), completed_at (string|null)`.

- [ ] **Step 1: Write the failing test**

`packages/contracts/test/envelope.test.ts`:
```ts
import { describe, it, expect } from "vitest";
import { CardInstance } from "../src/envelope";

const base = {
  id: "ci_1", card_id: "sift_craap", task_id: "t_1", parent_node_id: null,
  status: "proposed", field_values: {}, event_trace: [], rubric_tags: [],
  created_at: "2026-06-20T10:00:00.000Z", completed_at: null,
};

describe("CardInstance", () => {
  it("accepts a minimal proposed envelope", () => {
    expect(CardInstance.safeParse(base).success).toBe(true);
  });
  it("rejects an unknown status", () => {
    expect(CardInstance.safeParse({ ...base, status: "open" }).success).toBe(false);
  });
  it("rejects a missing card_id", () => {
    const { card_id, ...rest } = base;
    expect(CardInstance.safeParse(rest).success).toBe(false);
  });
  it("accepts a valid field_change trace event", () => {
    const env = { ...base, event_trace: [{ kind: "field_change", path: "sift.stop", at: base.created_at }] };
    expect(CardInstance.safeParse(env).success).toBe(true);
  });
  it("rejects a trace event with an unknown kind", () => {
    const env = { ...base, event_trace: [{ kind: "wiggle", at: base.created_at }] };
    expect(CardInstance.safeParse(env).success).toBe(false);
  });
});
```

- [ ] **Step 2: Run test to verify it fails**

Run: `pnpm --filter @mind-imprint/contracts test`
Expected: FAIL — cannot resolve `../src/envelope`.

- [ ] **Step 3: Write minimal implementation**

`packages/contracts/src/envelope.ts`:
```ts
import { z } from "zod";

export const TraceEvent = z.discriminatedUnion("kind", [
  z.object({ kind: z.literal("field_change"), path: z.string(), at: z.string() }),
  z.object({ kind: z.literal("step_expand"), step_key: z.string(), at: z.string() }),
  z.object({ kind: z.literal("note_open"), step_key: z.string(), at: z.string() }),
  z.object({ kind: z.literal("skip"), at: z.string() }),
  z.object({ kind: z.literal("submit"), at: z.string() }),
]);

export const CardStatus = z.enum(["proposed", "active", "completed", "skipped"]);

export const CardInstance = z.object({
  id: z.string(),
  card_id: z.string(),
  task_id: z.string(),
  parent_node_id: z.string().nullable(),
  status: CardStatus,
  field_values: z.record(z.unknown()),
  event_trace: z.array(TraceEvent),
  rubric_tags: z.array(z.string()),
  created_at: z.string(),
  completed_at: z.string().nullable(),
});

export type TraceEvent = z.infer<typeof TraceEvent>;
export type CardStatus = z.infer<typeof CardStatus>;
export type CardInstance = z.infer<typeof CardInstance>;
```

- [ ] **Step 4: Run test to verify it passes**

Run: `pnpm --filter @mind-imprint/contracts test`
Expected: PASS.

- [ ] **Step 5: Re-export + commit**

Append to `packages/contracts/src/index.ts`:
```ts
export * from "./envelope";
```

```bash
git add -A
git commit -m "feat(contracts): standard envelope + event trace schema"
```

---

### Task 5: Card JSONs + registry loader + catalog + DDL (B0)

**Files:**
- Create: `packages/contracts/cards/sift_craap.json`
- Create: `packages/contracts/cards/concession.json`
- Create: `packages/contracts/src/registry.ts`
- Create: `packages/contracts/sqlite-schema.sql`
- Test: `packages/contracts/test/registry.test.ts`
- Modify: `packages/contracts/src/index.ts`

**Interfaces:**
- Consumes: `CardSpec` from Task 3.
- Produces: `loadRegistry(raw?) => Record<string, CardSpec>` (throws on invalid/mismatched id); `deriveCatalog(registry) => Catalog` where `Catalog = { id, category, name, trigger_condition }[]`; `CARD_REGISTRY` (the loaded default).

- [ ] **Step 1: Create the two card JSONs (verbatim from PRD §6.3–6.4)**

`packages/contracts/cards/sift_craap.json`:
```json
{
  "id": "sift_craap",
  "category": "信息素养",
  "name": "SIFT×CRAAP 信息核查",
  "purpose": "先横向找更多来源(SIFT)，必要时再纵向深挖单一材料(CRAAP)",
  "trigger_condition": "学生准备直接采信或引用一个网络来源，但还没核查出处",
  "steps": [
    {
      "key": "sift", "title": "SIFT · 横向找更多来源", "disclose": "always",
      "methodology_note": "遇到一条信息先横向扩展、别一头扎进单一材料：Stop 停一下、Investigate 查来源、Find better coverage 找更权威版本、Trace 溯源。",
      "fields": [
        { "type": "textarea", "key": "stop", "label": "Stop：你打算用这条信息说明什么？" },
        { "type": "repeatable_group", "key": "sources", "label": "Investigate：找出 3 个独立来源",
          "item_fields": [
            { "type": "text", "key": "name", "label": "来源" },
            { "type": "single_choice", "key": "type", "label": "类型", "options": ["官方", "主流媒体", "学者/机构", "自媒体", "社交平台"] },
            { "type": "single_choice", "key": "verdict", "label": "可信？", "options": ["可信", "存疑", "不可信"] }
          ] },
        { "type": "textarea", "key": "better", "label": "Find better coverage：更权威的版本怎么说？" },
        { "type": "link_check", "key": "trace", "label": "Trace：溯到原始出处（贴链接）" }
      ]
    },
    {
      "key": "craap", "title": "CRAAP · 纵向深挖单一材料", "disclose": "on_demand",
      "methodology_note": "当你决定重点采信某个来源时，再纵向核它五维。",
      "fields": [
        { "type": "rating", "key": "currency", "scale": 5, "label": "Currency 时效性" },
        { "type": "rating", "key": "relevance", "scale": 5, "label": "Relevance 相关性" },
        { "type": "rating", "key": "authority", "scale": 5, "label": "Authority 权威性" },
        { "type": "rating", "key": "accuracy", "scale": 5, "label": "Accuracy 准确性" },
        { "type": "rating", "key": "purpose", "scale": 5, "label": "Purpose 目的性" }
      ]
    }
  ],
  "rubric_tags": ["D1_来源意识", "D2_交叉验证"]
}
```

`packages/contracts/cards/concession.json`:
```json
{
  "id": "concession",
  "category": "知识工具",
  "name": "让步段 · 以退为进",
  "purpose": "在论证里先承认反方最强的事实，再转折反驳，使论证更有力、结构更完整",
  "trigger_condition": "学生在写论证段，且出现了与其中心论点相悖的证据，却想直接忽略或硬压",
  "steps": [
    {
      "key": "concession", "title": "让步段四步", "disclose": "always",
      "methodology_note": "让步段是以退为进：先退一步承认你不同意的一个事实，再对它反驳。它让论证更有力、逻辑更严密。",
      "fields": [
        { "type": "text", "key": "thesis", "label": "你的中心论点是什么？" },
        { "type": "textarea", "key": "counter", "label": "反方最强的那个事实是什么？" },
        { "type": "textarea", "key": "concede", "label": "先承认它（让步）" },
        { "type": "textarea", "key": "rebut", "label": "再转折反驳（为什么它不足以推翻你的论点）" }
      ]
    }
  ],
  "rubric_tags": ["D5_论证结构", "D7_对立观点处理"]
}
```

- [ ] **Step 2: Write the failing test**

`packages/contracts/test/registry.test.ts`:
```ts
import { describe, it, expect } from "vitest";
import { loadRegistry, deriveCatalog } from "../src/registry";

describe("loadRegistry", () => {
  it("loads both bundled cards", () => {
    const reg = loadRegistry();
    expect(Object.keys(reg).sort()).toEqual(["concession", "sift_craap"]);
  });
  it("throws with the card id when a card is invalid", () => {
    const broken = { sift_craap: { id: "sift_craap", category: "信息素养" } };
    expect(() => loadRegistry(broken)).toThrow(/sift_craap/);
  });
  it("throws when the map key does not match card.id", () => {
    const reg = loadRegistry();
    const mismatched = { wrong_key: reg.sift_craap };
    expect(() => loadRegistry(mismatched as never)).toThrow(/does not match/);
  });
});

describe("deriveCatalog", () => {
  it("projects one trigger_condition line per card and nothing stale", () => {
    const cat = deriveCatalog(loadRegistry());
    expect(cat).toHaveLength(2);
    expect(cat.every((c) => typeof c.trigger_condition === "string" && c.trigger_condition.length > 0)).toBe(true);
    expect(Object.keys(cat[0]!).sort()).toEqual(["category", "id", "name", "trigger_condition"]);
  });
});
```

- [ ] **Step 3: Run test to verify it fails**

Run: `pnpm --filter @mind-imprint/contracts test`
Expected: FAIL — cannot resolve `../src/registry`.

- [ ] **Step 4: Write minimal implementation**

`packages/contracts/src/registry.ts`:
```ts
import { CardSpec } from "./cardSpec";
import siftCraap from "../cards/sift_craap.json" assert { type: "json" };
import concession from "../cards/concession.json" assert { type: "json" };

const DEFAULT_RAW: Record<string, unknown> = { sift_craap: siftCraap, concession };

export type CatalogEntry = { id: string; category: string; name: string; trigger_condition: string };
export type Catalog = CatalogEntry[];

export function loadRegistry(raw: Record<string, unknown> = DEFAULT_RAW): Record<string, CardSpec> {
  const out: Record<string, CardSpec> = {};
  for (const [key, data] of Object.entries(raw)) {
    const parsed = CardSpec.safeParse(data);
    if (!parsed.success) {
      const detail = parsed.error.issues.map((i) => `${i.path.join(".")}: ${i.message}`).join("; ");
      throw new Error(`Invalid card "${key}": ${detail}`);
    }
    if (parsed.data.id !== key) {
      throw new Error(`Card key "${key}" does not match card.id "${parsed.data.id}"`);
    }
    out[key] = parsed.data;
  }
  return out;
}

export function deriveCatalog(registry: Record<string, CardSpec>): Catalog {
  return Object.values(registry).map((c) => ({
    id: c.id, category: c.category, name: c.name, trigger_condition: c.trigger_condition,
  }));
}

export const CARD_REGISTRY = loadRegistry();
```

- [ ] **Step 5: Run test to verify it passes**

Run: `pnpm --filter @mind-imprint/contracts test`
Expected: PASS.

- [ ] **Step 6: Write the SQLite DDL (documented, not executed)**

`packages/contracts/sqlite-schema.sql` (physical shape for S2; mirrors PRD §9):
```sql
-- Physical schema for S2. NOT executed in Slice 1. Mirrors @mind-imprint/contracts types.
CREATE TABLE task (
  id TEXT PRIMARY KEY, title TEXT NOT NULL, seed TEXT,
  status TEXT NOT NULL, created_at TEXT NOT NULL, last_active_at TEXT
);
CREATE TABLE message (
  id TEXT PRIMARY KEY, task_id TEXT NOT NULL REFERENCES task(id),
  role TEXT NOT NULL, content TEXT, tool_call TEXT, created_at TEXT NOT NULL
);
CREATE TABLE card_instance (            -- the standard envelope (CardInstance)
  id TEXT PRIMARY KEY, card_id TEXT NOT NULL, task_id TEXT NOT NULL REFERENCES task(id),
  parent_node_id TEXT, status TEXT NOT NULL,
  field_values TEXT NOT NULL,           -- JSON
  event_trace TEXT NOT NULL,            -- JSON
  rubric_tags TEXT NOT NULL,            -- JSON
  created_at TEXT NOT NULL, completed_at TEXT
);
CREATE TABLE process_node (
  id TEXT PRIMARY KEY, task_id TEXT NOT NULL REFERENCES task(id),
  parent_id TEXT, type TEXT NOT NULL, label TEXT, ref_id TEXT, meta TEXT
);
CREATE TABLE evaluation (
  task_id TEXT NOT NULL REFERENCES task(id),
  rubric_scores TEXT NOT NULL, narrative TEXT, created_at TEXT NOT NULL
);
```

- [ ] **Step 7: Re-export + commit**

Append to `packages/contracts/src/index.ts`:
```ts
export * from "./registry";
```

```bash
git add -A
git commit -m "feat(contracts): card JSONs, registry loader, catalog, sqlite DDL"
```

---

### Task 6: Envelope reducer (B1, pure)

**Files:**
- Create: `apps/web/src/cards/envelopeReducer.ts`
- Test: `apps/web/src/cards/envelopeReducer.test.ts`

**Interfaces:**
- Consumes: `CardInstance`, `TraceEvent` from `@mind-imprint/contracts`.
- Produces: `envelopeReducer(env, action, now?) => CardInstance`; `ReducerAction` union: `{type:"activate"} | {type:"field_change", path, value} | {type:"step_expand", step_key} | {type:"note_open", step_key} | {type:"skip"} | {type:"submit"}`; `newEnvelope(card_id, task_id) => CardInstance`.

- [ ] **Step 1: Write the failing test**

`apps/web/src/cards/envelopeReducer.test.ts`:
```ts
import { describe, it, expect } from "vitest";
import { CardInstance } from "@mind-imprint/contracts";
import { envelopeReducer, newEnvelope } from "./envelopeReducer";

const clock = () => "2026-06-20T10:00:00.000Z";
const start = () => newEnvelope("sift_craap", "t_1");

describe("envelopeReducer", () => {
  it("activate flips proposed -> active without a trace event", () => {
    const env = envelopeReducer(start(), { type: "activate" }, clock);
    expect(env.status).toBe("active");
    expect(env.event_trace).toHaveLength(0);
  });
  it("field_change writes the value and appends a timestamped event", () => {
    const env = envelopeReducer(start(), { type: "field_change", path: "stop", value: "证明中国让地球变绿" }, clock);
    expect(env.field_values.stop).toBe("证明中国让地球变绿");
    expect(env.event_trace).toEqual([{ kind: "field_change", path: "stop", at: clock() }]);
  });
  it("field_change supports nested repeatable paths", () => {
    let env = envelopeReducer(start(), { type: "field_change", path: "sources[0].verdict", value: "存疑" }, clock);
    expect((env.field_values.sources as any)[0].verdict).toBe("存疑");
  });
  it("step_expand and note_open only append events", () => {
    let env = envelopeReducer(start(), { type: "step_expand", step_key: "craap" }, clock);
    env = envelopeReducer(env, { type: "note_open", step_key: "sift" }, clock);
    expect(env.event_trace.map((e) => e.kind)).toEqual(["step_expand", "note_open"]);
    expect(env.field_values).toEqual({});
  });
  it("skip sets status skipped + event", () => {
    const env = envelopeReducer(start(), { type: "skip" }, clock);
    expect(env.status).toBe("skipped");
    expect(env.event_trace.at(-1)).toEqual({ kind: "skip", at: clock() });
  });
  it("submit sets status completed, completed_at, + event; output is schema-valid", () => {
    const env = envelopeReducer(start(), { type: "submit" }, clock);
    expect(env.status).toBe("completed");
    expect(env.completed_at).toBe(clock());
    expect(CardInstance.safeParse(env).success).toBe(true);
  });
});
```

- [ ] **Step 2: Run test to verify it fails**

Run: `pnpm --filter web test`
Expected: FAIL — cannot resolve `./envelopeReducer`.

- [ ] **Step 3: Write minimal implementation**

`apps/web/src/cards/envelopeReducer.ts`:
```ts
import type { CardInstance, TraceEvent } from "@mind-imprint/contracts";

export type ReducerAction =
  | { type: "activate" }
  | { type: "field_change"; path: string; value: unknown }
  | { type: "step_expand"; step_key: string }
  | { type: "note_open"; step_key: string }
  | { type: "skip" }
  | { type: "submit" };

type Now = () => string;
const defaultNow: Now = () => new Date().toISOString();

let seq = 0;
export function newEnvelope(card_id: string, task_id: string, now: Now = defaultNow): CardInstance {
  return {
    id: `ci_${++seq}`,
    card_id, task_id, parent_node_id: null,
    status: "proposed", field_values: {}, event_trace: [], rubric_tags: [],
    created_at: now(), completed_at: null,
  };
}

function setPath(obj: Record<string, unknown>, path: string, value: unknown): Record<string, unknown> {
  const tokens = path.replace(/\[(\d+)\]/g, ".$1").split(".");
  const root: any = Array.isArray(obj) ? [...obj] : { ...obj };
  let cur: any = root;
  for (let i = 0; i < tokens.length - 1; i++) {
    const t = tokens[i]!;
    const existing = cur[t];
    cur[t] = Array.isArray(existing) ? [...existing] : { ...(existing ?? {}) };
    cur = cur[t];
  }
  cur[tokens[tokens.length - 1]!] = value;
  return root;
}

function append(env: CardInstance, event: TraceEvent): CardInstance {
  return { ...env, event_trace: [...env.event_trace, event] };
}

export function envelopeReducer(env: CardInstance, action: ReducerAction, now: Now = defaultNow): CardInstance {
  switch (action.type) {
    case "activate":
      return { ...env, status: "active" };
    case "field_change":
      return append(
        { ...env, field_values: setPath(env.field_values, action.path, action.value) },
        { kind: "field_change", path: action.path, at: now() },
      );
    case "step_expand":
      return append(env, { kind: "step_expand", step_key: action.step_key, at: now() });
    case "note_open":
      return append(env, { kind: "note_open", step_key: action.step_key, at: now() });
    case "skip":
      return append({ ...env, status: "skipped" }, { kind: "skip", at: now() });
    case "submit":
      return append({ ...env, status: "completed", completed_at: now() }, { kind: "submit", at: now() });
  }
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `pnpm --filter web test`
Expected: PASS (6 cases).

- [ ] **Step 5: Commit**

```bash
git add -A
git commit -m "feat(web): pure envelope reducer with event capture"
```

---

### Task 7: Simple field components + registry (B1)

**Files:**
- Create: `apps/web/src/cards/fields/TextField.tsx`
- Create: `apps/web/src/cards/fields/TextAreaField.tsx`
- Create: `apps/web/src/cards/fields/SingleChoiceField.tsx`
- Create: `apps/web/src/cards/fields/MultiChoiceField.tsx`
- Create: `apps/web/src/cards/fields/RatingField.tsx`
- Create: `apps/web/src/cards/fields/types.ts`
- Create: `apps/web/src/cards/fieldRegistry.tsx`
- Test: `apps/web/src/cards/fields/simpleFields.test.tsx`

**Interfaces:**
- Consumes: field schema types from `@mind-imprint/contracts`.
- Produces: a shared `FieldProps<F>` (`{ field: F; value: unknown; onChange: (value: unknown) => void }`); five components; `fieldRegistry` partially populated (`text, textarea, single_choice, multi_choice, rating`).

- [ ] **Step 1: Write the failing test**

`apps/web/src/cards/fields/simpleFields.test.tsx`:
```tsx
import { describe, it, expect, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { TextAreaField } from "./TextAreaField";
import { SingleChoiceField } from "./SingleChoiceField";
import { RatingField } from "./RatingField";

describe("simple fields", () => {
  it("textarea calls onChange with typed text", async () => {
    const onChange = vi.fn();
    render(<TextAreaField field={{ type: "textarea", key: "stop", label: "Stop" }} value="" onChange={onChange} />);
    await userEvent.type(screen.getByLabelText("Stop"), "x");
    expect(onChange).toHaveBeenLastCalledWith("x");
  });
  it("single_choice renders one button per option and reports the chosen value", async () => {
    const onChange = vi.fn();
    render(<SingleChoiceField field={{ type: "single_choice", key: "v", label: "可信？", options: ["可信", "存疑"] }} value={undefined} onChange={onChange} />);
    await userEvent.click(screen.getByRole("button", { name: "存疑" }));
    expect(onChange).toHaveBeenCalledWith("存疑");
  });
  it("rating renders `scale` segments and reports the picked number", async () => {
    const onChange = vi.fn();
    render(<RatingField field={{ type: "rating", key: "c", label: "Currency", scale: 5 }} value={0} onChange={onChange} />);
    const segs = screen.getAllByRole("radio");
    expect(segs).toHaveLength(5);
    await userEvent.click(segs[3]!);
    expect(onChange).toHaveBeenCalledWith(4);
  });
});
```

- [ ] **Step 2: Run test to verify it fails**

Run: `pnpm --filter web test`
Expected: FAIL — cannot resolve `./TextAreaField`.

- [ ] **Step 3: Write minimal implementation**

`apps/web/src/cards/fields/types.ts`:
```ts
export type FieldProps<F> = {
  field: F;
  value: unknown;
  onChange: (value: unknown) => void;
};
```

`apps/web/src/cards/fields/TextField.tsx`:
```tsx
import type { FieldProps } from "./types";
import type { z } from "zod";
import type { TextField as Schema } from "@mind-imprint/contracts";

type F = z.infer<typeof Schema>;

export function TextField({ field, value, onChange }: FieldProps<F>) {
  return (
    <label className="block">
      <span className="block text-[13px] font-semibold text-[#3A4256]">{field.label}</span>
      <input
        aria-label={field.label}
        value={(value as string) ?? ""}
        onChange={(e) => onChange(e.target.value)}
        className="mt-2 w-full rounded-[10px] border border-mk-input bg-mk-input-bg px-3 py-2.5 text-sm text-mk-ink outline-none"
      />
    </label>
  );
}
```

`apps/web/src/cards/fields/TextAreaField.tsx`:
```tsx
import type { FieldProps } from "./types";
import type { z } from "zod";
import type { TextAreaField as Schema } from "@mind-imprint/contracts";

type F = z.infer<typeof Schema>;

export function TextAreaField({ field, value, onChange }: FieldProps<F>) {
  return (
    <label className="block">
      <span className="block text-[13px] font-semibold text-[#3A4256]">{field.label}</span>
      <textarea
        aria-label={field.label}
        rows={field.rows ?? 2}
        value={(value as string) ?? ""}
        onChange={(e) => onChange(e.target.value)}
        className="mt-2 w-full resize-y rounded-[10px] border border-mk-input bg-mk-input-bg px-3 py-2.5 text-sm leading-relaxed text-mk-ink outline-none"
      />
    </label>
  );
}
```

`apps/web/src/cards/fields/SingleChoiceField.tsx`:
```tsx
import type { FieldProps } from "./types";
import type { z } from "zod";
import type { SingleChoiceField as Schema } from "@mind-imprint/contracts";

type F = z.infer<typeof Schema>;

export function SingleChoiceField({ field, value, onChange }: FieldProps<F>) {
  return (
    <div>
      <span className="block text-[13px] font-semibold text-[#3A4256]">{field.label}</span>
      <div className="mt-2 flex flex-wrap gap-2">
        {field.options.map((opt) => {
          const active = value === opt;
          return (
            <button
              key={opt}
              type="button"
              onClick={() => onChange(opt)}
              className={`rounded-full px-3 py-1.5 text-[13px] font-semibold ${active ? "bg-mk-primary text-white" : "bg-[#F2F3F8] text-[#6B7384]"}`}
            >
              {opt}
            </button>
          );
        })}
      </div>
    </div>
  );
}
```

`apps/web/src/cards/fields/MultiChoiceField.tsx`:
```tsx
import type { FieldProps } from "./types";
import type { z } from "zod";
import type { MultiChoiceField as Schema } from "@mind-imprint/contracts";

type F = z.infer<typeof Schema>;

export function MultiChoiceField({ field, value, onChange }: FieldProps<F>) {
  const selected = Array.isArray(value) ? (value as string[]) : [];
  const toggle = (opt: string) =>
    onChange(selected.includes(opt) ? selected.filter((o) => o !== opt) : [...selected, opt]);
  return (
    <div>
      <span className="block text-[13px] font-semibold text-[#3A4256]">{field.label}</span>
      <div className="mt-2 flex flex-wrap gap-2">
        {field.options.map((opt) => {
          const active = selected.includes(opt);
          return (
            <button
              key={opt}
              type="button"
              aria-pressed={active}
              onClick={() => toggle(opt)}
              className={`rounded-full px-3 py-1.5 text-[13px] font-semibold ${active ? "bg-mk-primary text-white" : "bg-[#F2F3F8] text-[#6B7384]"}`}
            >
              {opt}
            </button>
          );
        })}
      </div>
    </div>
  );
}
```

`apps/web/src/cards/fields/RatingField.tsx`:
```tsx
import type { FieldProps } from "./types";
import type { z } from "zod";
import type { RatingField as Schema } from "@mind-imprint/contracts";

type F = z.infer<typeof Schema>;

export function RatingField({ field, value, onChange }: FieldProps<F>) {
  const current = typeof value === "number" ? value : 0;
  return (
    <div>
      <span className="block text-[13px] font-semibold text-[#3A4256]">{field.label}</span>
      <div className="mt-2 flex gap-1.5" role="radiogroup" aria-label={field.label}>
        {Array.from({ length: field.scale }, (_, i) => i + 1).map((n) => (
          <button
            key={n}
            type="button"
            role="radio"
            aria-checked={current === n}
            aria-label={`${field.label} ${n}`}
            onClick={() => onChange(n)}
            className={`h-8 w-8 rounded-[8px] text-sm font-semibold ${current >= n ? "bg-mk-primary text-white" : "bg-[#F2F3F8] text-[#9AA1B0]"}`}
          >
            {n}
          </button>
        ))}
      </div>
    </div>
  );
}
```

`apps/web/src/cards/fieldRegistry.tsx`:
```tsx
import type { ComponentType } from "react";
import type { FieldType } from "@mind-imprint/contracts";
import type { FieldProps } from "./fields/types";
import { TextField } from "./fields/TextField";
import { TextAreaField } from "./fields/TextAreaField";
import { SingleChoiceField } from "./fields/SingleChoiceField";
import { MultiChoiceField } from "./fields/MultiChoiceField";
import { RatingField } from "./fields/RatingField";

// Partial for now; repeatable_group + link_check added in Task 8.
export const fieldRegistry: Partial<Record<FieldType, ComponentType<FieldProps<any>>>> = {
  text: TextField,
  textarea: TextAreaField,
  single_choice: SingleChoiceField,
  multi_choice: MultiChoiceField,
  rating: RatingField,
};
```

- [ ] **Step 4: Run test to verify it passes**

Run: `pnpm --filter web test`
Expected: PASS (3 cases).

- [ ] **Step 5: Commit**

```bash
git add -A
git commit -m "feat(web): simple field components + partial field registry"
```

---

### Task 8: Complex field components (repeatable_group, link_check) (B1)

**Files:**
- Create: `apps/web/src/cards/fields/LinkCheckField.tsx`
- Create: `apps/web/src/cards/fields/RepeatableGroupField.tsx`
- Modify: `apps/web/src/cards/fieldRegistry.tsx`
- Test: `apps/web/src/cards/fields/complexFields.test.tsx`

**Interfaces:**
- Consumes: `FieldProps`, the simple components (for nested item rendering), `fieldRegistry` (extends it).
- Produces: `LinkCheckField`, `RepeatableGroupField`; `fieldRegistry` now total over all 7 `FieldType`s. `RepeatableGroupField` value is an array of row objects; it emits the whole array via `onChange`.

- [ ] **Step 1: Write the failing test**

`apps/web/src/cards/fields/complexFields.test.tsx`:
```tsx
import { describe, it, expect, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { LinkCheckField } from "./LinkCheckField";
import { RepeatableGroupField } from "./RepeatableGroupField";

const sourcesField = {
  type: "repeatable_group" as const, key: "sources", label: "找出独立来源",
  item_fields: [
    { type: "text" as const, key: "name", label: "来源" },
    { type: "single_choice" as const, key: "verdict", label: "可信？", options: ["可信", "存疑"] },
  ],
};

describe("complex fields", () => {
  it("link_check shows the url and a verdict tag once a url is entered", async () => {
    const onChange = vi.fn();
    render(<LinkCheckField field={{ type: "link_check", key: "trace", label: "Trace" }} value="https://nature.com" onChange={onChange} />);
    expect(screen.getByText("https://nature.com")).toBeInTheDocument();
    expect(screen.getByText("已溯源")).toBeInTheDocument();
  });
  it("repeatable_group adds a row and emits the full array on edit", async () => {
    const onChange = vi.fn();
    render(<RepeatableGroupField field={sourcesField} value={[{}]} onChange={onChange} />);
    await userEvent.click(screen.getByRole("button", { name: "+ 添加来源" }));
    expect(onChange).toHaveBeenLastCalledWith([{}, {}]);
  });
  it("repeatable_group renders item_fields per row via their components", () => {
    render(<RepeatableGroupField field={sourcesField} value={[{ name: "公众号" }]} onChange={vi.fn()} />);
    expect(screen.getByDisplayValue("公众号")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "存疑" })).toBeInTheDocument();
  });
});
```

- [ ] **Step 2: Run test to verify it fails**

Run: `pnpm --filter web test`
Expected: FAIL — cannot resolve `./LinkCheckField`.

- [ ] **Step 3: Write minimal implementation**

`apps/web/src/cards/fields/LinkCheckField.tsx`:
```tsx
import type { FieldProps } from "./types";
import type { z } from "zod";
import type { LinkCheckField as Schema } from "@mind-imprint/contracts";

type F = z.infer<typeof Schema>;

export function LinkCheckField({ field, value, onChange }: FieldProps<F>) {
  const url = (value as string) ?? "";
  return (
    <div>
      <span className="block text-[13px] font-semibold text-[#3A4256]">{field.label}</span>
      <div className="mt-2 flex flex-wrap items-center gap-2.5">
        <input
          aria-label={field.label}
          value={url}
          onChange={(e) => onChange(e.target.value)}
          placeholder="贴链接"
          className="min-w-[200px] flex-1 rounded-[10px] border border-mk-input bg-mk-input-bg px-3 py-2.5 text-[13.5px] text-mk-ink outline-none"
        />
        {url.trim() !== "" && (
          <span className="rounded-[9px] bg-mk-green-tint px-3 py-2 text-[12.5px] font-semibold text-mk-green">已溯源</span>
        )}
      </div>
    </div>
  );
}
```

`apps/web/src/cards/fields/RepeatableGroupField.tsx`:
```tsx
import type { FieldProps } from "./types";
import type { z } from "zod";
import type { RepeatableGroupField as Schema, ItemField } from "@mind-imprint/contracts";
import { TextField } from "./TextField";
import { TextAreaField } from "./TextAreaField";
import { SingleChoiceField } from "./SingleChoiceField";
import { MultiChoiceField } from "./MultiChoiceField";
import { RatingField } from "./RatingField";
import { LinkCheckField } from "./LinkCheckField";
import type { ComponentType } from "react";
import type { FieldProps as FP } from "./types";

type F = z.infer<typeof Schema>;
type Row = Record<string, unknown>;

const ITEM_COMPONENTS: Record<ItemField["type"], ComponentType<FP<any>>> = {
  text: TextField, textarea: TextAreaField, single_choice: SingleChoiceField,
  multi_choice: MultiChoiceField, rating: RatingField, link_check: LinkCheckField,
};

export function RepeatableGroupField({ field, value, onChange }: FieldProps<F>) {
  const rows: Row[] = Array.isArray(value) ? (value as Row[]) : [];
  const update = (next: Row[]) => onChange(next);
  const setCell = (i: number, key: string, v: unknown) =>
    update(rows.map((r, idx) => (idx === i ? { ...r, [key]: v } : r)));

  return (
    <div>
      <span className="block text-[13px] font-semibold text-[#3A4256]">{field.label}</span>
      <div className="mt-2 space-y-2.5">
        {rows.map((row, i) => (
          <div key={i} className="space-y-3 rounded-[11px] border border-mk-border-2 bg-white p-3">
            {field.item_fields.map((f) => {
              const Cmp = ITEM_COMPONENTS[f.type];
              return <Cmp key={f.key} field={f} value={row[f.key]} onChange={(v) => setCell(i, f.key, v)} />;
            })}
          </div>
        ))}
      </div>
      <button
        type="button"
        onClick={() => update([...rows, {}])}
        className="mt-2.5 w-full rounded-[10px] border border-dashed border-[#CFD4E0] py-2.5 text-[13px] font-semibold text-[#6B7384]"
      >
        + 添加来源
      </button>
    </div>
  );
}
```

`apps/web/src/cards/fieldRegistry.tsx` — replace the partial object with the total one:
```tsx
import type { ComponentType } from "react";
import type { FieldType } from "@mind-imprint/contracts";
import type { FieldProps } from "./fields/types";
import { TextField } from "./fields/TextField";
import { TextAreaField } from "./fields/TextAreaField";
import { SingleChoiceField } from "./fields/SingleChoiceField";
import { MultiChoiceField } from "./fields/MultiChoiceField";
import { RatingField } from "./fields/RatingField";
import { LinkCheckField } from "./fields/LinkCheckField";
import { RepeatableGroupField } from "./fields/RepeatableGroupField";

export const fieldRegistry: Record<FieldType, ComponentType<FieldProps<any>>> = {
  text: TextField,
  textarea: TextAreaField,
  single_choice: SingleChoiceField,
  multi_choice: MultiChoiceField,
  rating: RatingField,
  link_check: LinkCheckField,
  repeatable_group: RepeatableGroupField,
};
```

- [ ] **Step 4: Run test to verify it passes**

Run: `pnpm --filter web test`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add -A
git commit -m "feat(web): repeatable_group + link_check fields; field registry now total"
```

---

### Task 9: CardRenderer — schema-driven, on-demand accordion (B1)

**Files:**
- Create: `apps/web/src/cards/CardRenderer.tsx`
- Test: `apps/web/src/cards/CardRenderer.test.tsx`
- Test: `apps/web/src/cards/noCardBranches.test.ts`

**Interfaces:**
- Consumes: `CardSpec` from contracts, `fieldRegistry`, `FieldProps`.
- Produces: `<CardRenderer card={CardSpec} values={Record<string,unknown>} onField={(path, value)=>void} onExpandStep={(key)=>void} />`. Walks `card.steps`; `disclose:"on_demand"` steps render collapsed behind a toggle; each field looked up by `field.type` from `fieldRegistry`.

- [ ] **Step 1: Write the failing tests**

`apps/web/src/cards/CardRenderer.test.tsx`:
```tsx
import { describe, it, expect, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { loadRegistry } from "@mind-imprint/contracts";
import { CardRenderer } from "./CardRenderer";

const reg = loadRegistry();

describe("CardRenderer (schema-driven)", () => {
  it("renders the SIFT card's always-step fields from JSON", () => {
    render(<CardRenderer card={reg.sift_craap!} values={{}} onField={vi.fn()} onExpandStep={vi.fn()} />);
    expect(screen.getByLabelText(/Stop/)).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "+ 添加来源" })).toBeInTheDocument();
    expect(screen.getByLabelText(/Find better coverage/)).toBeInTheDocument();
  });
  it("keeps the on_demand CRAAP step collapsed until expanded, then shows 5 ratings", async () => {
    const onExpandStep = vi.fn();
    render(<CardRenderer card={reg.sift_craap!} values={{}} onField={vi.fn()} onExpandStep={onExpandStep} />);
    expect(screen.queryByText(/Currency/)).not.toBeInTheDocument();
    await userEvent.click(screen.getByRole("button", { name: /CRAAP/ }));
    expect(onExpandStep).toHaveBeenCalledWith("craap");
    expect(screen.getByText(/Currency/)).toBeInTheDocument();
    expect(screen.getAllByRole("radiogroup")).toHaveLength(5);
  });
  it("renders the concession card (no on_demand steps) from the same component", () => {
    render(<CardRenderer card={reg.concession!} values={{}} onField={vi.fn()} onExpandStep={vi.fn()} />);
    expect(screen.getByLabelText("你的中心论点是什么？")).toBeInTheDocument();
    expect(screen.getByLabelText("先承认它（让步）")).toBeInTheDocument();
  });
});
```

`apps/web/src/cards/noCardBranches.test.ts` (enforces the schema-driven constraint):
```ts
import { describe, it, expect } from "vitest";
import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";

describe("schema-driven guarantee", () => {
  it("CardRenderer contains no card-id-specific branches", () => {
    const src = readFileSync(fileURLToPath(new URL("./CardRenderer.tsx", import.meta.url)), "utf8");
    expect(src).not.toMatch(/sift_craap|concession|card\.id\s*===|card_id\s*===/);
  });
});
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `pnpm --filter web test`
Expected: FAIL — cannot resolve `./CardRenderer`.

- [ ] **Step 3: Write minimal implementation**

`apps/web/src/cards/CardRenderer.tsx`:
```tsx
import { useState } from "react";
import type { CardSpec, Step } from "@mind-imprint/contracts";
import { fieldRegistry } from "./fieldRegistry";

type Props = {
  card: CardSpec;
  values: Record<string, unknown>;
  onField: (path: string, value: unknown) => void;
  onExpandStep: (stepKey: string) => void;
};

function StepFields({ step, values, onField }: { step: Step; values: Record<string, unknown>; onField: Props["onField"] }) {
  return (
    <div className="space-y-4">
      {step.fields.map((field) => {
        const Cmp = fieldRegistry[field.type];
        return (
          <Cmp
            key={field.key}
            field={field}
            value={values[field.key]}
            onChange={(v: unknown) => onField(field.key, v)}
          />
        );
      })}
    </div>
  );
}

export function CardRenderer({ card, values, onField, onExpandStep }: Props) {
  return (
    <div className="space-y-4">
      {card.steps.map((step) =>
        step.disclose === "on_demand" ? (
          <OnDemandStep key={step.key} step={step} values={values} onField={onField} onExpandStep={onExpandStep} />
        ) : (
          <section key={step.key} className="rounded-mk border border-mk-border-2 bg-white p-5">
            <h3 className="mb-4 text-[15px] font-bold text-mk-ink">{step.title}</h3>
            <StepFields step={step} values={values} onField={onField} />
          </section>
        ),
      )}
    </div>
  );
}

function OnDemandStep({ step, values, onField, onExpandStep }: { step: Step } & Omit<Props, "card">) {
  const [open, setOpen] = useState(false);
  return (
    <section className="overflow-hidden rounded-mk border border-mk-border-2 bg-white">
      <button
        type="button"
        onClick={() => {
          if (!open) onExpandStep(step.key);
          setOpen((v) => !v);
        }}
        className="flex w-full items-center justify-between px-5 py-4 text-left"
      >
        <span className="text-[15px] font-bold text-mk-ink">{step.title}</span>
        <span className="text-[11.5px] font-medium text-mk-muted-2">按需展开</span>
      </button>
      {open && (
        <div className="px-5 pb-5">
          <StepFields step={step} values={values} onField={onField} />
        </div>
      )}
    </section>
  );
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `pnpm --filter web test`
Expected: PASS — both `CardRenderer.test.tsx` and `noCardBranches.test.ts`.

- [ ] **Step 5: Commit**

```bash
git add -A
git commit -m "feat(web): schema-driven CardRenderer with on-demand accordion"
```

---

### Task 10: ProposalBubble — the three chat states (B1)

**Files:**
- Create: `apps/web/src/cards/states/ProposalBubble.tsx`
- Test: `apps/web/src/cards/states/ProposalBubble.test.tsx`

**Interfaces:**
- Consumes: nothing from contracts directly (takes plain props).
- Produces: `<ProposalBubble status={"proposed"|"completed"|"skipped"} category name nudge onOpen onSkip />`.

- [ ] **Step 1: Write the failing test**

`apps/web/src/cards/states/ProposalBubble.test.tsx`:
```tsx
import { describe, it, expect, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { ProposalBubble } from "./ProposalBubble";

const base = { category: "信息素养", name: "SIFT×CRAAP 信息核查", nudge: "这儿先别急着写，我们用 SIFT 核一下这个来源？" };

describe("ProposalBubble", () => {
  it("proposed: shows 打开卡 and 暂不，先继续 and wires callbacks", async () => {
    const onOpen = vi.fn(); const onSkip = vi.fn();
    render(<ProposalBubble status="proposed" {...base} onOpen={onOpen} onSkip={onSkip} />);
    await userEvent.click(screen.getByRole("button", { name: "打开卡" }));
    await userEvent.click(screen.getByRole("button", { name: "暂不，先继续" }));
    expect(onOpen).toHaveBeenCalledOnce();
    expect(onSkip).toHaveBeenCalledOnce();
  });
  it("completed: shows the pinned-to-tree confirmation, no 打开卡", () => {
    render(<ProposalBubble status="completed" {...base} onOpen={vi.fn()} onSkip={vi.fn()} />);
    expect(screen.getByText(/已钉到过程树/)).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "打开卡" })).not.toBeInTheDocument();
  });
  it("skipped: shows the recorded-as-signal note and 仍可打开", () => {
    render(<ProposalBubble status="skipped" {...base} onOpen={vi.fn()} onSkip={vi.fn()} />);
    expect(screen.getByText(/已记录为信号/)).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "仍可打开" })).toBeInTheDocument();
  });
});
```

- [ ] **Step 2: Run test to verify it fails**

Run: `pnpm --filter web test`
Expected: FAIL — cannot resolve `./ProposalBubble`.

- [ ] **Step 3: Write minimal implementation**

`apps/web/src/cards/states/ProposalBubble.tsx`:
```tsx
type Props = {
  status: "proposed" | "completed" | "skipped";
  category: string;
  name: string;
  nudge: string;
  onOpen: () => void;
  onSkip: () => void;
};

export function ProposalBubble({ status, category, name, nudge, onOpen, onSkip }: Props) {
  return (
    <div className="w-full max-w-[520px] rounded-[5px_16px_16px_16px] border border-mk-border bg-white p-4 shadow-sm">
      <div className="mb-3 flex items-center gap-2.5">
        <span className="rounded-full bg-mk-accent-tint px-2.5 py-1 text-[11px] font-bold text-mk-accent">建议工具卡</span>
        <span className="rounded-full bg-mk-primary-tint px-2.5 py-1 text-[11px] font-semibold text-mk-primary">{category}</span>
      </div>
      <div className="mb-1.5 text-[15px] font-bold text-mk-ink">{name}</div>
      <div className="text-sm leading-relaxed text-[#5B6373]">{nudge}</div>

      {status === "proposed" && (
        <div className="mt-3.5 flex items-center gap-3.5">
          <button type="button" onClick={onOpen} className="rounded-[11px] bg-mk-accent px-5 py-2.5 text-sm font-bold text-white">打开卡</button>
          <button type="button" onClick={onSkip} className="text-[13px] font-semibold text-mk-muted-2">暂不，先继续</button>
        </div>
      )}
      {status === "completed" && (
        <div className="mt-3 inline-flex items-center gap-1.5 rounded-[9px] bg-mk-green-tint px-3 py-1.5 text-[13px] font-semibold text-mk-green">
          已完成 · 已钉到过程树
        </div>
      )}
      {status === "skipped" && (
        <div className="mt-3 flex items-center gap-3">
          <span className="rounded-[9px] bg-[#F1F2F5] px-3 py-1.5 text-[13px] font-semibold text-mk-muted-2">已跳过（已记录为信号）</span>
          <button type="button" onClick={onOpen} className="text-[13px] font-semibold text-mk-primary underline underline-offset-2">仍可打开</button>
        </div>
      )}
    </div>
  );
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `pnpm --filter web test`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add -A
git commit -m "feat(web): ProposalBubble (proposed/completed/skipped states)"
```

---

### Task 11: ActiveSheet — bottom sheet + methodology panel (B1)

**Files:**
- Create: `apps/web/src/cards/states/ActiveSheet.tsx`
- Test: `apps/web/src/cards/states/ActiveSheet.test.tsx`

**Interfaces:**
- Consumes: `CardSpec`, `<CardRenderer>`.
- Produces: `<ActiveSheet card values onField onExpandStep onNoteOpen onSubmit onClose />`. Renders the sheet header ("现在轮到你想" + category + name + purpose), a "这个工具怎么用" toggle that reveals `card.steps[0].methodology_note` and fires `onNoteOpen(stepKey)`, the `<CardRenderer>`, a 提交 button, and a close control.

- [ ] **Step 1: Write the failing test**

`apps/web/src/cards/states/ActiveSheet.test.tsx`:
```tsx
import { describe, it, expect, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { loadRegistry } from "@mind-imprint/contracts";
import { ActiveSheet } from "./ActiveSheet";

const reg = loadRegistry();
const props = () => ({
  card: reg.sift_craap!, values: {},
  onField: vi.fn(), onExpandStep: vi.fn(), onNoteOpen: vi.fn(), onSubmit: vi.fn(), onClose: vi.fn(),
});

describe("ActiveSheet", () => {
  it("shows the takeover header and the card name", () => {
    render(<ActiveSheet {...props()} />);
    expect(screen.getByText("现在轮到你想")).toBeInTheDocument();
    expect(screen.getByText("SIFT×CRAAP 信息核查")).toBeInTheDocument();
  });
  it("methodology toggle reveals the note and fires onNoteOpen", async () => {
    const p = props();
    render(<ActiveSheet {...p} />);
    expect(screen.queryByText(/先横向扩展/)).not.toBeInTheDocument();
    await userEvent.click(screen.getByRole("button", { name: "这个工具怎么用" }));
    expect(p.onNoteOpen).toHaveBeenCalledWith("sift");
    expect(screen.getByText(/先横向扩展/)).toBeInTheDocument();
  });
  it("提交 fires onSubmit", async () => {
    const p = props();
    render(<ActiveSheet {...p} />);
    await userEvent.click(screen.getByRole("button", { name: "提交" }));
    expect(p.onSubmit).toHaveBeenCalledOnce();
  });
});
```

- [ ] **Step 2: Run test to verify it fails**

Run: `pnpm --filter web test`
Expected: FAIL — cannot resolve `./ActiveSheet`.

- [ ] **Step 3: Write minimal implementation**

`apps/web/src/cards/states/ActiveSheet.tsx`:
```tsx
import { useState } from "react";
import type { CardSpec } from "@mind-imprint/contracts";
import { CardRenderer } from "../CardRenderer";

type Props = {
  card: CardSpec;
  values: Record<string, unknown>;
  onField: (path: string, value: unknown) => void;
  onExpandStep: (stepKey: string) => void;
  onNoteOpen: (stepKey: string) => void;
  onSubmit: () => void;
  onClose: () => void;
};

export function ActiveSheet({ card, values, onField, onExpandStep, onNoteOpen, onSubmit, onClose }: Props) {
  const [noteOpen, setNoteOpen] = useState(false);
  const firstStepKey = card.steps[0]!.key;
  const note = card.steps[0]!.methodology_note;

  return (
    <div className="absolute inset-0 z-40 flex flex-col justify-end">
      <div onClick={onClose} className="absolute inset-0 bg-[rgba(22,28,46,0.40)]" aria-hidden />
      <div className="relative mx-auto flex h-[80%] w-full max-w-[880px] flex-col overflow-hidden rounded-[22px_22px_0_0] bg-white shadow-2xl">
        <div className="h-1 flex-none bg-mk-accent" />
        <div className="flex flex-none items-start gap-3.5 border-b border-[#F0F1F5] px-6 py-4">
          <div className="min-w-0 flex-1">
            <div className="mb-1.5 flex items-center gap-2.5">
              <span className="text-[11px] font-bold tracking-wide text-mk-accent">现在轮到你想</span>
              <span className="rounded-full bg-mk-primary-tint px-2.5 py-0.5 text-[11px] font-semibold text-mk-primary">{card.category}</span>
            </div>
            <div className="text-lg font-bold text-mk-ink">{card.name}</div>
            <div className="mt-1 text-[13px] text-mk-muted-2">{card.purpose}</div>
          </div>
          <div className="flex items-center gap-2">
            <button
              type="button"
              onClick={() => { if (!noteOpen) onNoteOpen(firstStepKey); setNoteOpen((v) => !v); }}
              className="rounded-[9px] bg-[#F2F3F8] px-3 py-2 text-[12.5px] font-semibold text-[#5B6373]"
            >
              这个工具怎么用
            </button>
            <button type="button" aria-label="关闭" onClick={onClose} className="h-8 w-8 rounded-[9px] text-mk-muted-2">✕</button>
          </div>
        </div>

        <div className="flex-1 overflow-y-auto bg-[#FAFBFC] px-6 py-5">
          {noteOpen && (
            <div className="mb-4 rounded-mk border border-mk-primary-tint bg-mk-primary-tint/40 p-4 text-[13px] leading-relaxed text-[#3A4256]">{note}</div>
          )}
          <CardRenderer card={card} values={values} onField={onField} onExpandStep={onExpandStep} />
        </div>

        <div className="flex-none border-t border-[#F0F1F5] px-6 py-3.5">
          <button type="button" onClick={onSubmit} className="rounded-[12px] bg-mk-primary px-6 py-3 text-sm font-bold text-white">提交</button>
        </div>
      </div>
    </div>
  );
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `pnpm --filter web test`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add -A
git commit -m "feat(web): ActiveSheet bottom sheet + methodology panel"
```

---

### Task 12: CompletedCard — compact done state (B1)

**Files:**
- Create: `apps/web/src/cards/states/CompletedCard.tsx`
- Test: `apps/web/src/cards/states/CompletedCard.test.tsx`

**Interfaces:**
- Consumes: `CardSpec`.
- Produces: `<CompletedCard card filledCount />` — a compact summary (name + category + "已完成" + a filled-field count).

- [ ] **Step 1: Write the failing test**

`apps/web/src/cards/states/CompletedCard.test.tsx`:
```tsx
import { describe, it, expect } from "vitest";
import { render, screen } from "@testing-library/react";
import { loadRegistry } from "@mind-imprint/contracts";
import { CompletedCard } from "./CompletedCard";

const reg = loadRegistry();

describe("CompletedCard", () => {
  it("shows the card name, a completed marker, and the filled count", () => {
    render(<CompletedCard card={reg.concession!} filledCount={3} />);
    expect(screen.getByText("让步段 · 以退为进")).toBeInTheDocument();
    expect(screen.getByText(/已完成/)).toBeInTheDocument();
    expect(screen.getByText(/3/)).toBeInTheDocument();
  });
});
```

- [ ] **Step 2: Run test to verify it fails**

Run: `pnpm --filter web test`
Expected: FAIL — cannot resolve `./CompletedCard`.

- [ ] **Step 3: Write minimal implementation**

`apps/web/src/cards/states/CompletedCard.tsx`:
```tsx
import type { CardSpec } from "@mind-imprint/contracts";

export function CompletedCard({ card, filledCount }: { card: CardSpec; filledCount: number }) {
  return (
    <div className="rounded-mk border border-mk-border bg-white p-4">
      <div className="flex items-center gap-2.5">
        <span className="rounded-full bg-mk-primary-tint px-2.5 py-0.5 text-[11px] font-semibold text-mk-primary">{card.category}</span>
        <span className="inline-flex items-center gap-1 rounded-[9px] bg-mk-green-tint px-2.5 py-1 text-[12px] font-semibold text-mk-green">✓ 已完成</span>
      </div>
      <div className="mt-2 text-sm font-bold text-mk-ink">{card.name}</div>
      <div className="mt-1 text-[12px] text-mk-muted-2">已填 {filledCount} 项</div>
    </div>
  );
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `pnpm --filter web test`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add -A
git commit -m "feat(web): CompletedCard compact done state"
```

---

### Task 13: Dev harness — drive both cards through all three states (B1, capstone)

**Files:**
- Create: `apps/web/src/dev/fixtures.ts`
- Create: `apps/web/src/dev/Harness.tsx`
- Modify: `apps/web/src/main.tsx`
- Test: `apps/web/src/dev/Harness.test.tsx`

**Interfaces:**
- Consumes: everything above (`loadRegistry`, `envelopeReducer`, `newEnvelope`, `ProposalBubble`, `ActiveSheet`, `CompletedCard`, `CardInstance`).
- Produces: a `<Harness />` that selects a card, walks proposed → active (fill) → completed/skipped, and on submit validates the envelope against `CardInstance` and renders the JSON. `fixtures.ts` exports `PHOEBE_VALUES` (the real design content) per card id.

- [ ] **Step 1: Write the failing test**

`apps/web/src/dev/Harness.test.tsx`:
```tsx
import { describe, it, expect } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { CardInstance } from "@mind-imprint/contracts";
import { Harness } from "./Harness";

describe("Harness end-to-end (fake data)", () => {
  it("proposed -> open -> submit yields a schema-valid completed envelope shown as JSON", async () => {
    render(<Harness />);
    await userEvent.click(screen.getByRole("button", { name: "打开卡" }));
    await userEvent.click(screen.getByRole("button", { name: "提交" }));

    const json = screen.getByTestId("envelope-json").textContent ?? "{}";
    const parsed = JSON.parse(json);
    expect(CardInstance.safeParse(parsed).success).toBe(true);
    expect(parsed.status).toBe("completed");
    expect(parsed.completed_at).not.toBeNull();
  });

  it("skip from the proposal yields a skipped envelope", async () => {
    render(<Harness />);
    await userEvent.click(screen.getByRole("button", { name: "暂不，先继续" }));
    const parsed = JSON.parse(screen.getByTestId("envelope-json").textContent ?? "{}");
    expect(parsed.status).toBe("skipped");
  });
});
```

- [ ] **Step 2: Run test to verify it fails**

Run: `pnpm --filter web test`
Expected: FAIL — cannot resolve `./Harness`.

- [ ] **Step 3: Write the fixtures (real Phoebe content from the design)**

`apps/web/src/dev/fixtures.ts`:
```ts
export const PHOEBE_VALUES: Record<string, Record<string, unknown>> = {
  sift_craap: {
    stop: "我想用这条信息支持「中国让地球更可持续」。",
    sources: [
      { name: "公众号《环球科技》", type: "自媒体", verdict: "存疑" },
      { name: "NASA Earth Observatory", type: "官方", verdict: "可信" },
      { name: "Nature Sustainability (IF 32.1)", type: "学者/机构", verdict: "可信" },
    ],
    better: "NASA 与 Nature Sustainability 指出变绿主要来自农业集约化与植树，并非整体生态改善。",
    trace: "https://www.nature.com/articles/s41893-019-0220-7",
  },
  concession: {
    thesis: "中国在很大程度上让地球更可持续。",
    counter: "中国是全球碳排放总量第一。",
    concede: "确实，中国的碳排放总量目前居全球首位。",
    rebut: "但其人均排放低于多数发达国家，且在可再生能源装机与植被恢复上贡献全球领先——总量第一不足以推翻其在可持续上的净贡献。",
  },
};
```

- [ ] **Step 4: Write the harness implementation**

`apps/web/src/dev/Harness.tsx`:
```tsx
import { useMemo, useState } from "react";
import { loadRegistry, type CardInstance } from "@mind-imprint/contracts";
import { envelopeReducer, newEnvelope, type ReducerAction } from "../cards/envelopeReducer";
import { ProposalBubble } from "../cards/states/ProposalBubble";
import { ActiveSheet } from "../cards/states/ActiveSheet";
import { CompletedCard } from "../cards/states/CompletedCard";

const reg = loadRegistry();
const cardIds = Object.keys(reg);

export function Harness() {
  const [cardId, setCardId] = useState(cardIds[0]!);
  const card = reg[cardId]!;
  const [env, setEnv] = useState<CardInstance>(() => newEnvelope(cardId, "t_demo"));

  const dispatch = (a: ReducerAction) => setEnv((e) => envelopeReducer(e, a));
  const reset = (id: string) => { setCardId(id); setEnv(newEnvelope(id, "t_demo")); };
  const filledCount = useMemo(() => Object.keys(env.field_values).length, [env]);

  return (
    <div className="relative min-h-screen bg-mk-bg p-8 font-sans text-mk-ink">
      <div className="mx-auto max-w-[760px] space-y-5">
        <div className="flex gap-2">
          {cardIds.map((id) => (
            <button
              key={id}
              onClick={() => reset(id)}
              className={`rounded-full px-3 py-1.5 text-[13px] font-semibold ${id === cardId ? "bg-mk-primary text-white" : "bg-white text-mk-muted-2"}`}
            >
              {reg[id]!.name}
            </button>
          ))}
        </div>

        {env.status === "proposed" && (
          <ProposalBubble
            status="proposed"
            category={card.category}
            name={card.name}
            nudge="这儿先别急着写，我们用这张卡先想一步？"
            onOpen={() => dispatch({ type: "activate" })}
            onSkip={() => dispatch({ type: "skip" })}
          />
        )}
        {env.status === "skipped" && (
          <ProposalBubble status="skipped" category={card.category} name={card.name} nudge=""
            onOpen={() => dispatch({ type: "activate" })} onSkip={() => {}} />
        )}
        {env.status === "completed" && <CompletedCard card={card} filledCount={filledCount} />}

        <pre data-testid="envelope-json" className="overflow-auto rounded-mk border border-mk-border bg-white p-4 text-[12px] text-mk-ink">
          {JSON.stringify(env, null, 2)}
        </pre>
      </div>

      {env.status === "active" && (
        <ActiveSheet
          card={card}
          values={env.field_values}
          onField={(path, value) => dispatch({ type: "field_change", path, value })}
          onExpandStep={(step_key) => dispatch({ type: "step_expand", step_key })}
          onNoteOpen={(step_key) => dispatch({ type: "note_open", step_key })}
          onSubmit={() => dispatch({ type: "submit" })}
          onClose={() => dispatch({ type: "submit" })}
        />
      )}
    </div>
  );
}
```

`apps/web/src/main.tsx` — replace the placeholder body:
```tsx
import React from "react";
import { createRoot } from "react-dom/client";
import { Harness } from "./dev/Harness";
import "./index.css";

createRoot(document.getElementById("root")!).render(
  <React.StrictMode>
    <Harness />
  </React.StrictMode>,
);
```

- [ ] **Step 5: Run test to verify it passes**

Run: `pnpm --filter web test`
Expected: PASS — both harness cases (completed + skipped envelopes are schema-valid).

- [ ] **Step 6: Full slice verification**

Run: `pnpm -r test`
Expected: PASS — all contracts + web suites.

Run: `pnpm --filter web build`
Expected: build succeeds (type-checks the whole app).

Manual: `pnpm --filter web dev`, open the URL — switch between SIFT×CRAAP and 让步段, click 打开卡, fill fields, expand CRAAP (5 ratings), open "这个工具怎么用", submit; confirm the printed envelope JSON shows `status:"completed"`, a populated `event_trace`, and `completed_at`.

- [ ] **Step 7: Commit**

```bash
git add -A
git commit -m "feat(web): dev harness driving both cards through all three states"
```

---

## Self-Review

**Spec coverage** (against `docs/superpowers/specs/2026-06-20-slice1-card-contract-runtime-design.md`):
- §4.1 field primitives → Task 2. §4.2 card spec → Task 3. §4.3 envelope → Task 4. §4.4 event trace → Task 4. §4.5 registry + catalog → Task 5. §4.6 card JSONs → Task 5. §4.7 SQLite DDL → Task 5 Step 6.
- §5.1 schema-driven renderer → Task 9 (+ no-branches guard). §5.2 three states → Tasks 10–12. §5.3 methodology panel → Task 11. §5.4 reducer (incl. `activate`) → Task 6. §5.5 dev harness → Task 13. §5.6 Tailwind tokens → Task 1.
- §7 tests 1–4 → Tasks 2–5; tests 5–7 (reducer) → Task 6; tests 8–11 (rendering/states) → Tasks 9–12; test 12 (submit → valid envelope) → Task 13.
- §8 DoD: all-green (Task 13 Step 6), harness demo (Step 6), no card branches (Task 9), DDL (Task 5), visual alignment (component styling across Tasks 7–12).

**Placeholder scan:** no TBD/TODO; every code step contains complete code; every test step contains real assertions.

**Type consistency:** `CardInstance`/`TraceEvent`/`CardStatus` (Task 4) used identically in Task 6 and Task 13; `FieldProps` (Task 7) reused in Tasks 7–8; `fieldRegistry` defined partial in Task 7 then made total in Task 8 and consumed in Task 9; `ReducerAction` (Task 6) consumed in Task 13; `loadRegistry`/`deriveCatalog`/`Catalog` (Task 5) names match across Tasks 9–13.

**Note on `activate → onClose`:** in the harness, closing the sheet maps to `submit` (the demo has no separate "save draft"); intentional for S1 since there is no persistence — the envelope is always finalized on leaving the sheet.
