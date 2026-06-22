# Card Selection Signal (Layer A) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task (tasks are tightly coupled — one feature across contracts + web). Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Surface each card's `purpose` to the LLM decision layer alongside `trigger_condition`, so the model can decide whether to summon a card from both "when it fits" and "what it does" — fixing the 0/12 no-summon behavior.

**Architecture:** `purpose` already exists on every `CardSpec` (required) but `deriveCatalog` drops it. Add it to `CatalogEntry` + `deriveCatalog` (contracts), render it in `buildCatalogText` and reference it in the system prompt (web). No envelope changes, no new authored content.

**Tech Stack:** TypeScript, Zod (contracts), Vitest. pnpm monorepo. Gate: `pnpm -r typecheck && pnpm -r test`.

## Global Constraints

- 目录从 registry 派生，不手写第二份 (卡 spec 单一真相源在 registry)。
- 标准信封结构不动。
- 保留全部「克制 / 按需」铁律语句；只增补，不删减克制。
- `trigger_keywords` 保持死元数据，不为其接线。
- Gate must stay green: `pnpm -r typecheck && pnpm -r test`.

---

### Task 1: contracts — carry `purpose` through `deriveCatalog`

**Files:**
- Modify: `packages/contracts/src/registry.ts` (CatalogEntry type ~`:75`, deriveCatalog ~`:97`)
- Test: `packages/contracts/test/registry.test.ts:22-37`

**Interfaces:**
- Consumes: `CardSpec.purpose: string` (already required, `cardSpec.ts:24`).
- Produces: `CatalogEntry.purpose: string` — consumed by Task 2's `buildCatalogText`.

- [ ] **Step 1: Update the failing test**

In `packages/contracts/test/registry.test.ts`, update the exact-key assertion (line 27) to include `purpose`, and add a value assertion. Replace the `deriveCatalog` describe block's first test body and add a purpose check to the routing test:

```ts
  it("projects one trigger_condition line per card and nothing stale", () => {
    const cat = deriveCatalog(loadRegistry());
    expect(cat).toHaveLength(33);
    expect(cat.every((c) => typeof c.trigger_condition === "string" && c.trigger_condition.length > 0)).toBe(true);
    expect(Object.keys(cat[0]!).sort()).toEqual(["category", "disclosure_tier", "id", "interaction_type", "name", "priority", "purpose", "trigger_condition", "trigger_keywords"]);
  });
  it("deriveCatalog projects routing metadata", () => {
    const cat = deriveCatalog(loadRegistry());
    const sift = cat.find((c) => c.id === "sift_craap");
    expect(sift).toBeDefined();
    expect(sift!.priority).toBe("P0");
    expect(sift!.disclosure_tier).toBe("tier-0");
    expect(sift!.trigger_keywords?.length).toBeGreaterThan(0);
    expect(sift!.interaction_type).toBe("步骤引导卡");
    expect(typeof sift!.purpose).toBe("string");
    expect(sift!.purpose.length).toBeGreaterThan(0);
  });
```

- [ ] **Step 2: Run test to verify it fails**

Run: `pnpm --filter @mind-imprint/contracts test -- registry`
Expected: FAIL — exact-key assertion mismatch (`purpose` missing) and `sift!.purpose` undefined.

- [ ] **Step 3: Add `purpose` to type + projection**

In `packages/contracts/src/registry.ts`, add `purpose: string;` to `CatalogEntry`:

```ts
export type CatalogEntry = {
  id: string; category: string; name: string; purpose: string; trigger_condition: string;
  trigger_keywords?: string[]; disclosure_tier?: string; priority?: string; interaction_type?: string;
};
```

And include it in `deriveCatalog`'s map:

```ts
export function deriveCatalog(registry: Record<string, CardSpec>): Catalog {
  return Object.values(registry).map((c) => ({
    id: c.id, category: c.category, name: c.name, purpose: c.purpose, trigger_condition: c.trigger_condition,
    trigger_keywords: c.trigger_keywords, disclosure_tier: c.disclosure_tier,
    priority: c.priority, interaction_type: c.interaction_type,
  }));
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `pnpm --filter @mind-imprint/contracts test -- registry`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add packages/contracts/src/registry.ts packages/contracts/test/registry.test.ts
git commit -m "feat(contracts): carry card purpose through deriveCatalog"
```

---

### Task 2: web — render `purpose` + `trigger_condition` in `buildCatalogText`

**Files:**
- Modify: `apps/web/src/agent/prompt.ts` (`buildCatalogText` `:6-28`)
- Test: `apps/web/src/agent/prompt.test.ts:8-14`

**Interfaces:**
- Consumes: `CatalogEntry.purpose` (Task 1), `.trigger_condition`, `.interaction_type`, `.name`, `.id`.
- Produces: catalog text consumed by `buildSystemPrompt` (Task 3).

- [ ] **Step 1: Update the failing test**

In `apps/web/src/agent/prompt.test.ts`, replace the first test to assert both dimensions and the category header:

```ts
  it("catalog text groups by category and shows each card's trigger_condition and purpose", () => {
    const txt = buildCatalogText(full);
    expect(txt).toContain("【信息素养】");
    expect(txt).toContain("sift_craap");
    const sift = full.find((c) => c.id === "sift_craap")!;
    expect(txt).toContain(sift.trigger_condition);
    expect(txt).toContain(sift.purpose);
    expect(txt).toContain("何时用");
    expect(txt).toContain("能帮他");
  });
```

- [ ] **Step 2: Run test to verify it fails**

Run: `pnpm --filter web test -- prompt`
Expected: FAIL — output lacks `purpose`, `何时用`, `能帮他`.

- [ ] **Step 3: Rewrite the per-card rendering**

In `apps/web/src/agent/prompt.ts`, replace the inner loop body of `buildCatalogText` (the `for (const e of entries)` block) so each card renders a small block:

```ts
    for (const e of entries) {
      const kind = e.interaction_type ? `（${e.interaction_type}）` : "";
      lines.push(`· ${e.id}｜${e.name}${kind}`);
      lines.push(`   何时用：${e.trigger_condition}`);
      lines.push(`   能帮他：${e.purpose}`);
    }
```

(The `【${category}】` header push stays as-is. Remove the old single `· ${e.id} — ${e.name} [${tier}·${prio}]：${e.trigger_condition}` line and the now-unused `tier`/`prio` locals.)

- [ ] **Step 4: Run test to verify it passes**

Run: `pnpm --filter web test -- prompt`
Expected: PASS. (The `buildSystemPrompt` test still passes — it only checks `sift_craap` is present.)

- [ ] **Step 5: Commit**

```bash
git add apps/web/src/agent/prompt.ts apps/web/src/agent/prompt.test.ts
git commit -m "feat(agent): show card purpose + trigger in decision catalog"
```

---

### Task 3: web — system prompt references the two-dimension catalog

**Files:**
- Modify: `apps/web/src/agent/prompt.ts` (`PROMPT_TEMPLATE`, the "# 工具卡（按需，不是每次）" block `:52-58`)
- Test: `apps/web/src/agent/prompt.test.ts:15-20`

**Interfaces:**
- Consumes: nothing new.
- Produces: system prompt string; existing callers unchanged.

- [ ] **Step 1: Add a failing assertion**

In `apps/web/src/agent/prompt.test.ts`, extend the system-prompt test to assert the new guidance line and that restraint language remains:

```ts
  it("system prompt embeds the catalog and the restraint rules", () => {
    const p = buildSystemPrompt(full);
    expect(p).toContain("不替他定论");
    expect(p).toContain("summon_card");
    expect(p).toContain("sift_craap");
    // new: tells the model the catalog carries both dimensions, and that a
    // genuine match is good coaching (restraint still intact).
    expect(p).toContain("何时用");
    expect(p).toContain("能帮他");
  });
```

- [ ] **Step 2: Run test to verify it fails**

Run: `pnpm --filter web test -- prompt`
Expected: FAIL — `何时用` / `能帮他` not in the template (only in the injected catalog, but template text itself should name them).

- [ ] **Step 3: Add one guidance line to the template**

In `apps/web/src/agent/prompt.ts`, inside the "# 工具卡（按需，不是每次）" bullet list, add one bullet (keep all existing restraint bullets):

```
- 目录里每张卡都标了「何时用」(适用情形) 和「能帮他」(这张卡能给学生什么)。当学生此刻的处境**同时贴合**这两栏时，提议这张卡就是好的陪练——这不违反克制；克制是指不替他定论，不是永不递工具。
```

Place it as the first bullet under that heading, before the existing `- 只有当学生此刻的处境**正好命中**...` bullet.

- [ ] **Step 4: Run test to verify it passes**

Run: `pnpm --filter web test -- prompt`
Expected: PASS.

- [ ] **Step 5: Full gate**

Run: `pnpm -r typecheck && pnpm -r test`
Expected: all packages green.

- [ ] **Step 6: Commit**

```bash
git add apps/web/src/agent/prompt.ts apps/web/src/agent/prompt.test.ts
git commit -m "feat(agent): prompt names the two-dimension catalog as summon guidance"
```

---

### Task 4: Live verification (evidence, not an automated test)

**Not a code task — produces the proof the fix works.**

- [ ] Re-run the live DeepSeek summon probe using the *new* `buildSystemPrompt(demoCatalog(...))` + `summonCardTool(...)`, with the same opening lines that previously returned 0/12.
- [ ] Record the before→after summon rate.
- [ ] If the model now over-summons (fires on weak fits), tighten the Task 3 prompt wording — do NOT revert the signal (Task 1/2). Re-run.
- [ ] Note the result for the PR / carryforward tracker.

Key/baseURL/model come from `apps/web/.env.local` (gitignored). The key never enters git, logs, or any committed file.
