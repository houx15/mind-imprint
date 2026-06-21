# Slice 3a — Card Library Import Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Extend `CardSpec` with the library's frontmatter routing metadata and register all 31 library cards (+ keep the 2 existing demo cards = **33 in the registry**), so Slice 3b's decision layer can route over the full library. No renderer changes — bespoke-interaction cards get placeholder bodies.

**Architecture:** Additive optional fields on the Zod `CardSpec` (B0). 31 new card JSONs authored from `docs/工具包库/*.md` (frontmatter → metadata; 「渲染要点」/「分步脚本」 → `steps`/`fields` with the existing 7 primitives). Form-expressible cards get real bodies (`body_status:"full"`); bespoke ones get a one-textarea placeholder (`body_status:"stub"`). `loadRegistry`/`deriveCatalog` grow to 33; the card Harness gains a picker over the whole registry.

**Tech Stack:** TypeScript 5 (ES2022), Zod 3, React 18, Vitest + @testing-library/react (jsdom).

## Global Constraints

- **Acceptance gate:** `pnpm -r typecheck` AND `pnpm -r test` both green. `vite build`/`vitest` use esbuild and do **not** typecheck — run `pnpm --filter @mind-imprint/contracts typecheck` and/or `pnpm --filter web typecheck` in every task that touches that package.
- **Registry is the single source of truth**; `deriveCatalog` is a projection, never hand-duplicated. `loadRegistry` stays fail-loud (throws on an invalid card or `id !== key`).
- **New card = new JSON, NO renderer/primitive code changes.** This slice adds **zero** new field primitives and **zero** renderer logic. Any card that cannot be faithfully expressed with the existing 7 primitives (`text`/`textarea`/`single_choice`/`multi_choice`/`rating`/`repeatable_group`/`link_check`) is authored as a **stub** (see Authoring Protocol), not by adding a primitive.
- **`CardSpec` extension is additive + optional** — the existing 2 cards and the frozen `CardInstance` envelope are unaffected.
- Card content is faithful to `docs/工具包库/` (frontmatter + 渲染要点 + 分步脚本), Chinese, real material — no lorem.
- Contracts tests live in `packages/contracts/test/*.test.ts`; web tests beside source in `apps/web/src/**`.
- Card JSON files: `packages/contracts/cards/<id>.json`, where `<id>` is the frontmatter `id` (kebab-case). Registered in `packages/contracts/src/registry.ts` `DEFAULT_RAW` keyed by that same `id`.

---

## Authoring Protocol (binds Tasks 5–12)

Every card-authoring task converts a list of `docs/工具包库/NN-*.md` files into JSON specs. For **each** card file:

**1. Metadata (from the `.md` YAML frontmatter):**
| JSON field | Source | Notes |
|---|---|---|
| `id` | frontmatter `id` | kebab-case; also the filename and the `DEFAULT_RAW` key |
| `name` | frontmatter `name_zh` | |
| `name_en` | frontmatter `name_en` | |
| `category` | frontmatter `category` | one of the 8 categories |
| `purpose` | the card's 「一句话定位」 | one sentence |
| `trigger_condition` | frontmatter `trigger_context` | the semantic trigger (NOT a second field) |
| `trigger_keywords` | frontmatter `trigger_keywords` | array |
| `priority` | frontmatter `priority` | `"P0"`/`"P1"`/`"P2"` |
| `disclosure_tier` | frontmatter `disclosure_tier` | `"tier-0"`/`"tier-1"`/`"tier-2"` |
| `age_band` | frontmatter `age_band` | array, e.g. `["MYP","DP"]` |
| `interaction_type` | frontmatter `interaction_type` | one of the 8 enum values |
| `rubric_dims` | frontmatter `rubric_dims` | short D-codes, e.g. `["D2","D3"]` |
| `related` | frontmatter `related` | array of card ids |
| `rubric_tags` | = `rubric_dims` | `CardSpec.rubric_tags` is required; set it to the same D-codes unless the design names descriptive tags |

**2. Body — full vs stub triage (the red line):**
- **Can the card's 「渲染要点（卡片字段）」 + 「分步脚本」 be faithfully expressed with the 7 primitives?** → **FULL** (`body_status:"full"` or omit). Author real `steps[]` (each with `key`, `title`, `disclose:"always"|"on_demand"`, `methodology_note` from the step's 就地小讲解, and `fields[]`). Use `repeatable_group` for "list N items", `single_choice`/`multi_choice` for option picks, `rating` for scales, `link_check` for "paste a source link", `textarea`/`text` for free input.
- **Does it need a bespoke interaction** (a draggable spectrum/slider, a role-play dialogue, a node/canvas map, drag-tagging — typically `interaction_type ∈ {量表光谱卡, 角色模拟卡, 画布导图卡, 分类标注卡}`)? → **STUB** (`body_status:"stub"`). One step, one `textarea` placeholder (see the stub example below). Do NOT invent a primitive to force fidelity — when in doubt, stub it.

**3. Register:** add an `import` for the JSON and a `DEFAULT_RAW` entry keyed by the card `id` in `packages/contracts/src/registry.ts`.

**4. Validate:** `loadRegistry()` validates every card on construction (fail-loud) — a malformed card makes `CARD_REGISTRY` throw and the registry test fail. Plus the task's own batch test (below).

### FULL example — the canonical format

`packages/contracts/cards/sift_craap.json` after its Task-2 metadata backfill (real, existing card body + new metadata) is the reference for a full card. Re-read it before authoring.

### STUB example — copy this shape exactly

`packages/contracts/cards/belief-spectrum.json` (card 26, a 量表光谱卡):

```json
{
  "id": "belief-spectrum",
  "category": "溯源与多视角",
  "name": "立场光谱卡（Belief Spectrum）",
  "name_en": "Belief Spectrum",
  "purpose": "把对错两派摊成一条连续光谱，定位各方与自己",
  "trigger_condition": "学生把争议议题简化成「对错两派」，看不到中间的连续立场",
  "trigger_keywords": ["两派", "谁对谁错", "专家吵架", "争议", "中立", "我站哪边"],
  "priority": "P1",
  "disclosure_tier": "tier-1",
  "age_band": ["MYP", "DP"],
  "interaction_type": "量表光谱卡",
  "rubric_dims": ["D4"],
  "related": ["steelman", "ethics-roleplay", "source-map"],
  "body_status": "stub",
  "rubric_tags": ["D4"],
  "steps": [
    {
      "key": "main",
      "title": "立场光谱",
      "disclose": "always",
      "methodology_note": "把「对/错两派」摊成一条连续光谱：不同立场各自相信什么、引什么证据、背后有什么利益——再看看自己站哪。",
      "fields": [
        { "type": "textarea", "key": "notes", "label": "先用文字记录你的思考；这张卡的完整交互（可拖放的立场光谱）即将上线" }
      ]
    }
  ]
}
```

### Batch test pattern (each authoring task writes one)

`packages/contracts/test/cards.<category-slug>.test.ts` — import the batch's JSONs and assert each parses + carries required metadata:

```ts
import { describe, it, expect } from "vitest";
import { CardSpec } from "../src/cardSpec";
import cardA from "../cards/<idA>.json";
import cardB from "../cards/<idB>.json";

const batch = { "<idA>": cardA, "<idB>": cardB };

describe("<category> cards", () => {
  for (const [key, raw] of Object.entries(batch)) {
    it(`${key} is a valid CardSpec with routing metadata`, () => {
      const parsed = CardSpec.safeParse(raw);
      expect(parsed.success).toBe(true);
      if (!parsed.success) return;
      expect(parsed.data.id).toBe(key);
      expect(parsed.data.priority).toBeDefined();
      expect(parsed.data.disclosure_tier).toBeDefined();
      expect(parsed.data.interaction_type).toBeDefined();
      expect(parsed.data.trigger_keywords?.length).toBeGreaterThan(0);
    });
  }
});
```

---

### Task 1: Extend `CardSpec` with frontmatter metadata

**Files:**
- Modify: `packages/contracts/src/cardSpec.ts`
- Test: `packages/contracts/test/cardSpec.test.ts` (extend)

**Interfaces:**
- Produces: `Priority`, `DisclosureTier`, `InteractionType`, `BodyStatus` (Zod enums + types) and the extended `CardSpec` with optional `name_en`, `priority`, `disclosure_tier`, `age_band`, `trigger_keywords`, `interaction_type`, `rubric_dims`, `related`, `body_status`. Existing required fields unchanged.

- [ ] **Step 1: Write the failing test** — add to `packages/contracts/test/cardSpec.test.ts`:

```ts
import { CardSpec } from "../src/cardSpec";

const baseStep = { key: "s", title: "T", disclose: "always", methodology_note: "", fields: [{ type: "textarea", key: "x", label: "L" }] };
const minimal = { id: "c", category: "信息素养", name: "n", purpose: "", trigger_condition: "", steps: [baseStep], rubric_tags: [] };

describe("CardSpec metadata extension", () => {
  it("still accepts a card with no metadata (back-compat)", () => {
    expect(CardSpec.safeParse(minimal).success).toBe(true);
  });
  it("accepts full routing metadata", () => {
    const withMeta = { ...minimal, name_en: "N", priority: "P0", disclosure_tier: "tier-0",
      age_band: ["MYP","DP"], trigger_keywords: ["a"], interaction_type: "步骤引导卡",
      rubric_dims: ["D1"], related: ["other"], body_status: "stub" };
    expect(CardSpec.safeParse(withMeta).success).toBe(true);
  });
  it("rejects an unknown priority", () => {
    expect(CardSpec.safeParse({ ...minimal, priority: "P9" }).success).toBe(false);
  });
  it("rejects an unknown interaction_type", () => {
    expect(CardSpec.safeParse({ ...minimal, interaction_type: "全息卡" }).success).toBe(false);
  });
  it("rejects an unknown body_status", () => {
    expect(CardSpec.safeParse({ ...minimal, body_status: "draft" }).success).toBe(false);
  });
});
```

- [ ] **Step 2: Run to verify it fails** — `pnpm --filter @mind-imprint/contracts exec vitest run test/cardSpec.test.ts` → FAIL (unknown values currently accepted / enums undefined).

- [ ] **Step 3: Implement** — `packages/contracts/src/cardSpec.ts`:

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

export const Priority = z.enum(["P0", "P1", "P2"]);
export const DisclosureTier = z.enum(["tier-0", "tier-1", "tier-2"]);
export const InteractionType = z.enum([
  "步骤引导卡", "选择追问卡", "分类标注卡", "量表光谱卡",
  "角色模拟卡", "画布导图卡", "回放验证卡", "报告生成卡",
]);
export const BodyStatus = z.enum(["full", "stub"]);

export const CardSpec = z.object({
  id: z.string().min(1),
  category: z.string().min(1),
  name: z.string().min(1),
  purpose: z.string(),
  trigger_condition: z.string(),
  steps: z.array(Step).min(1),
  rubric_tags: z.array(z.string()),
  // routing metadata (library frontmatter) — additive, optional
  name_en: z.string().optional(),
  priority: Priority.optional(),
  disclosure_tier: DisclosureTier.optional(),
  age_band: z.array(z.string()).optional(),
  trigger_keywords: z.array(z.string()).optional(),
  interaction_type: InteractionType.optional(),
  rubric_dims: z.array(z.string()).optional(),
  related: z.array(z.string()).optional(),
  body_status: BodyStatus.optional(),
});

export type Step = z.infer<typeof Step>;
export type Priority = z.infer<typeof Priority>;
export type DisclosureTier = z.infer<typeof DisclosureTier>;
export type InteractionType = z.infer<typeof InteractionType>;
export type BodyStatus = z.infer<typeof BodyStatus>;
export type CardSpec = z.infer<typeof CardSpec>;
```

- [ ] **Step 4: Run test + typecheck** — `pnpm --filter @mind-imprint/contracts exec vitest run test/cardSpec.test.ts` → PASS; `pnpm --filter @mind-imprint/contracts typecheck` → no errors.

- [ ] **Step 5: Commit** — `git commit -am "feat(contracts): extend CardSpec with library routing metadata"`

---

### Task 2: Backfill the 2 demo cards with metadata

**Files:**
- Modify: `packages/contracts/cards/sift_craap.json`, `packages/contracts/cards/concession.json`
- Test: `packages/contracts/test/registry.test.ts` (extend) or new `test/demoCards.test.ts`

**Interfaces:**
- Consumes: extended `CardSpec` from Task 1.
- Produces: both demo cards carrying the new metadata + explicit `body_status:"full"`. This becomes the canonical FULL example for Tasks 5–12.

- [ ] **Step 1: Write the failing test** — `packages/contracts/test/demoCards.test.ts`:

```ts
import { describe, it, expect } from "vitest";
import { CardSpec } from "../src/cardSpec";
import siftCraap from "../cards/sift_craap.json";
import concession from "../cards/concession.json";

describe("demo cards carry routing metadata", () => {
  for (const [key, raw] of Object.entries({ sift_craap: siftCraap, concession })) {
    it(`${key} is valid with metadata + body_status full`, () => {
      const p = CardSpec.safeParse(raw);
      expect(p.success).toBe(true);
      if (!p.success) return;
      expect(p.data.priority).toBeDefined();
      expect(p.data.disclosure_tier).toBeDefined();
      expect(p.data.interaction_type).toBeDefined();
      expect(p.data.trigger_keywords?.length).toBeGreaterThan(0);
      expect(p.data.body_status).toBe("full");
    });
  }
});
```

- [ ] **Step 2: Run to verify it fails** — `pnpm --filter @mind-imprint/contracts exec vitest run test/demoCards.test.ts` → FAIL (metadata absent).

- [ ] **Step 3: Implement** — add these keys to each JSON (alongside existing fields; do NOT alter existing `steps`/`rubric_tags`):

`sift_craap.json` (combined SIFT×CRAAP demo card):
```json
"name_en": "SIFT × CRAAP Source Check",
"priority": "P0",
"disclosure_tier": "tier-0",
"age_band": ["MYP", "DP"],
"trigger_keywords": ["来源", "可信吗", "这是真的吗", "谁说的", "出处"],
"interaction_type": "步骤引导卡",
"rubric_dims": ["D1", "D2"],
"related": ["sift", "craap"],
"body_status": "full"
```

`concession.json` (让步段 demo card):
```json
"name_en": "Concession / Steelman",
"priority": "P0",
"disclosure_tier": "tier-1",
"age_band": ["MYP", "DP"],
"trigger_keywords": ["反例", "但是", "反方", "对方会说", "让步", "驳论"],
"interaction_type": "步骤引导卡",
"rubric_dims": ["D4"],
"related": ["steelman"],
"body_status": "full"
```

- [ ] **Step 4: Run test + typecheck** — `pnpm --filter @mind-imprint/contracts exec vitest run test/demoCards.test.ts` → PASS; existing `test/registry.test.ts` still green; `pnpm --filter @mind-imprint/contracts typecheck` → no errors.

- [ ] **Step 5: Commit** — `git add -A && git commit -m "feat(contracts): backfill demo cards with routing metadata"`

---

### Task 3: Extend `deriveCatalog` to project routing metadata

**Files:**
- Modify: `packages/contracts/src/registry.ts`
- Test: `packages/contracts/test/registry.test.ts` (extend)

**Interfaces:**
- Consumes: extended `CardSpec`; the 2 backfilled demo cards.
- Produces: `CatalogEntry` extended to `{ id, category, name, trigger_condition, trigger_keywords?, disclosure_tier?, priority?, interaction_type? }`; `deriveCatalog` projects these.

- [ ] **Step 1: Write the failing test** — add to `test/registry.test.ts`:

```ts
it("deriveCatalog projects routing metadata", () => {
  const cat = deriveCatalog(loadRegistry());
  const sift = cat.find((c) => c.id === "sift_craap");
  expect(sift).toBeDefined();
  expect(sift!.priority).toBe("P0");
  expect(sift!.disclosure_tier).toBe("tier-0");
  expect(sift!.trigger_keywords?.length).toBeGreaterThan(0);
  expect(sift!.interaction_type).toBe("步骤引导卡");
});
```

- [ ] **Step 2: Run to verify it fails** — `pnpm --filter @mind-imprint/contracts exec vitest run test/registry.test.ts` → FAIL (fields not projected).

- [ ] **Step 3: Implement** — in `registry.ts`, extend the type and projection:

```ts
export type CatalogEntry = {
  id: string; category: string; name: string; trigger_condition: string;
  trigger_keywords?: string[]; disclosure_tier?: string; priority?: string; interaction_type?: string;
};
export type Catalog = CatalogEntry[];

export function deriveCatalog(registry: Record<string, CardSpec>): Catalog {
  return Object.values(registry).map((c) => ({
    id: c.id, category: c.category, name: c.name, trigger_condition: c.trigger_condition,
    trigger_keywords: c.trigger_keywords, disclosure_tier: c.disclosure_tier,
    priority: c.priority, interaction_type: c.interaction_type,
  }));
}
```

- [ ] **Step 4: Run test + typecheck** — `pnpm --filter @mind-imprint/contracts exec vitest run test/registry.test.ts` → PASS; `pnpm --filter @mind-imprint/contracts typecheck` → no errors.

- [ ] **Step 5: Commit** — `git commit -am "feat(contracts): project routing metadata in deriveCatalog"`

---

### Task 4: Harness card picker (over the whole registry)

**Files:**
- Modify: `apps/web/src/cards/dev/Harness.tsx`
- Test: `apps/web/src/cards/dev/Harness.test.tsx` (extend)

**Interfaces:**
- Consumes: `CARD_REGISTRY` from `@mind-imprint/contracts` (currently 2 cards; grows to 33 in later tasks — the picker is data-driven so it auto-lists whatever is loaded).
- Produces: a card `<select>` (grouped/labeled by category) that renders the chosen card via the existing `<CardRenderer>`; stub cards show a small "占位 · {interaction_type}" marker.

- [ ] **Step 1: Write the failing test** — add to `Harness.test.tsx`:

```ts
it("lists registry cards and switches the rendered card", async () => {
  render(<Harness />);
  const picker = screen.getByLabelText("选择工具卡");
  // both current cards present as options
  expect(within(picker).getByRole("option", { name: /SIFT/ })).toBeInTheDocument();
  expect(within(picker).getByRole("option", { name: /让步段/ })).toBeInTheDocument();
});
```

(Use `within`/`screen` per existing Harness test imports; if the current Harness test lacks `within`, import it from `@testing-library/react`.)

- [ ] **Step 2: Run to verify it fails** — `pnpm --filter web exec vitest run src/cards/dev/Harness.test.tsx` → FAIL (no picker).

- [ ] **Step 3: Implement** — add a `<select aria-label="选择工具卡">` populated from `Object.values(CARD_REGISTRY)` (label each option `name`; optionally group by `category` with `<optgroup>`), holding the selected card id in state, and pass the selected `CardSpec` to the existing `<CardRenderer>`. When the selected card's `body_status === "stub"`, render a small marker line `占位 · 富交互待上线 · {interaction_type}` above the renderer. Do not change `CardRenderer` itself. Keep the existing harness seed/behavior for the default card.

- [ ] **Step 4: Run test + typecheck** — `pnpm --filter web exec vitest run src/cards/dev/Harness.test.tsx` → PASS; `pnpm --filter web typecheck` → no errors.

- [ ] **Step 5: Commit** — `git commit -am "feat(dev): Harness card picker over the registry"`

---

### Tasks 5–12: Author library cards by category

Each task follows the **Authoring Protocol** above: read the listed `.md` files, author one `<id>.json` per card (full or stub per the red line), register each in `registry.ts` `DEFAULT_RAW`, and add the batch test (`test/cards.<slug>.test.ts`) from the protocol's Batch test pattern. Acceptance for every task: the batch test passes, `loadRegistry()` does not throw (it validates all registered cards), and `pnpm --filter @mind-imprint/contracts typecheck` is clean. Commit at the end of each task.

> The full-vs-stub call is per-card. The notes below flag the *likely* stubs to expect, but the implementer decides by the red line (faithfully expressible with the 7 primitives?).

- [ ] **Task 5 — 探究启动 (Launch):** `01-探究启动_提问卡`, `02-探究启动_兔子洞兴趣雷达卡`, `03-探究启动_情感对齐卡`. Commit `feat(contracts): author 探究启动 cards`.
- [ ] **Task 6 — 信息素养 (Information Literacy):** `04-信息素养_CRAAP信源辨识卡` (id `craap`), `05-信息素养_SIFT横向核查卡` (id `sift`), `06-信息素养_资金链溯源卡`, `07-信息素养_营销与否认套路卡`, `08-信息素养_多模态解构卡`, `09-信息素养_CDA话语分析卡`. Commit `feat(contracts): author 信息素养 cards`.
- [ ] **Task 7 — 知识工具 (Knowledge Tools):** `10-知识工具_事实观点价值判断卡`, `11-知识工具_论证地图卡` (likely stub: canvas), `12-知识工具_让步段反方最强卡` (id `steelman`), `13-知识工具_PEE写作卡`, `14-知识工具_数据与统计素养卡`, `15-知识工具_语言框定卡`, `16-知识工具_确定度光谱卡` (likely stub: spectrum). Commit `feat(contracts): author 知识工具 cards`.
- [ ] **Task 8 — AOK (Areas of Knowledge):** `17-AOK_OPCVL史料评估卡`, `18-AOK_科学怎么算知道卡`, `19-AOK_其他学科方法卡集`. Commit `feat(contracts): author AOK cards`.
- [ ] **Task 9 — AI伦理 (AI & Ethics):** `20-AI伦理_与AI协作保持判断卡`, `21-AI伦理_AI边界与幻觉核查卡`, `22-AI伦理_伦理判断三镜头卡`, `23-AI伦理_伦理情景角色博弈卡` (likely stub: role-play), `24-AI伦理_负责任使用AI决策树卡`. Commit `feat(contracts): author AI伦理 cards`.
- [ ] **Task 10 — 溯源 (Tracing):** `25-溯源_3D溯源导图卡` (likely stub: canvas), `26-溯源_立场光谱卡` (stub: spectrum — use the protocol's belief-spectrum example), `27-溯源_语料钩子卡`. Commit `feat(contracts): author 溯源 cards`.
- [ ] **Task 11 — 反身性 (Reflexivity):** `28-反身性_认知者视角自审卡`, `29-反身性_元认知收口卡`, `30-反身性_Checkpoint无AI回放卡`. Commit `feat(contracts): author 反身性 cards`.
- [ ] **Task 12 — 成长 (Growth):** `31-成长_学习报告AI使用声明卡`. Commit `feat(contracts): author 成长 cards`.

Each task's steps: (1) read the `.md` files; (2) author the JSONs; (3) register in `DEFAULT_RAW`; (4) write the batch test; (5) run batch test + registry test + typecheck (all green); (6) commit.

---

### Task 13: Completeness gate + roadmap/memory note

**Files:**
- Test: `packages/contracts/test/library.test.ts` (new)
- Modify: `docs/架构分解_Roadmap.md` (note S3 → 3a/3b/3c split)

**Interfaces:**
- Consumes: the full `CARD_REGISTRY` (33 cards).

- [ ] **Step 1: Write the failing test** — `packages/contracts/test/library.test.ts`:

```ts
import { describe, it, expect } from "vitest";
import { CARD_REGISTRY, deriveCatalog } from "../src/index";

describe("card library is complete", () => {
  it("registry holds all 33 cards", () => {
    expect(Object.keys(CARD_REGISTRY).length).toBe(33);
  });
  it("every card carries core routing metadata and a valid body_status", () => {
    for (const card of Object.values(CARD_REGISTRY)) {
      expect(card.priority, card.id).toBeDefined();
      expect(card.disclosure_tier, card.id).toBeDefined();
      expect(card.interaction_type, card.id).toBeDefined();
      expect((card.trigger_keywords ?? []).length, card.id).toBeGreaterThan(0);
      if (card.body_status !== undefined) {
        expect(["full", "stub"]).toContain(card.body_status);
      }
    }
  });
  it("catalog projects all 33 cards", () => {
    expect(deriveCatalog(CARD_REGISTRY).length).toBe(33);
  });
});
```

- [ ] **Step 2: Run to verify it fails (until all cards authored)** — `pnpm --filter @mind-imprint/contracts exec vitest run test/library.test.ts`. (If run before Tasks 5–12 complete, the count assertion fails — that is expected; this task runs last.)

- [ ] **Step 3: Implement** — no new source code; this task is the completeness assertion. If any card is missing metadata or the count is off, fix the offending card JSON (authored in Tasks 5–12) until green. Then update `docs/架构分解_Roadmap.md` §3 to record that S3 split into **3a (this slice) → 3b (decision + summon_card loop + workspace) → 3c (rich card interactions)**, and that the registry now carries the full library (33 cards) with routing metadata.

- [ ] **Step 4: Run full gate** — `pnpm -r typecheck` and `pnpm -r test` → both green; `library.test.ts` green (33 cards).

- [ ] **Step 5: Commit** — `git add -A && git commit -m "test(contracts): library completeness gate (33 cards) + roadmap note"`

---

## Self-Review

**Spec coverage:** CardSpec extension (Task 1) ✓; backfill demo cards (Task 2) ✓; deriveCatalog projection (Task 3) ✓; Harness picker (Task 4) ✓; all 31 library cards authored, full-or-stub, registered (Tasks 5–12) ✓; 33-card completeness + metadata + catalog + stub-renders (Task 13 + batch tests) ✓; renderer untouched / zero new primitives (Global Constraints + stub red line) ✓; keep-both 33-card decision (Task 2 keeps demo cards; Tasks 5–12 import all 31 incl. sift/craap/steelman) ✓.

**Placeholder scan:** The per-card JSON bodies are authored by the implementer from each `.md` (a translation task) under the Authoring Protocol with a complete worked stub example and a pointer to the worked full example — this is intended, not a TODO. No "TBD"/"implement later" in any code step.

**Type consistency:** `CatalogEntry` fields (Task 3) match `CardSpec` field names (Task 1). The batch test pattern, `body_status` values (`full`/`stub`), and the 33 count are consistent across Tasks 2, 5–12, and 13. `rubric_tags` (required) is set = `rubric_dims` per the protocol so every authored card satisfies the schema.
