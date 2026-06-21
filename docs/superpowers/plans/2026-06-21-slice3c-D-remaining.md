# Slice 3c-D — Un-stub remaining cards (composed) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: superpowers:subagent-driven-development. Steps use checkbox (`- [ ]`).

**Goal:** Un-stub the final four stub cards — argument-map, money-trail, source-map (画布导图) and ethics-roleplay (角色模拟) — using EXISTING primitives (rich structured forms). No new primitive. This completes S3c (all 10 stub cards → `full`).

**Architecture:** JSON-only un-stubs. Each card flips `body_status:"stub"→"full"` and replaces its placeholder textarea with structured fields built from `text`/`textarea`/`single_choice`/`repeatable_group`. Zero code change.

**Tech Stack:** `packages/contracts` (Zod cards + Vitest). Package `@mind-imprint/contracts`.

## Global Constraints

- Gate per task: `pnpm -r typecheck` AND `pnpm -r test` green.
- Existing primitives only (no new field type, no renderer/registry change). Registry stays 33.
- Each card: set `body_status:"full"`; keep all other JSON keys (id/category/name/rubric_dims/trigger_*/related/step key/title/disclose). Replace any stale 「即将上线」/「即将」 `methodology_note` with a faithful one-line note.
- JSON: never ASCII double-quotes inside a string value — use 「」. Keep valid JSON.
- Restraint: cards set up structure + ask the student to fill/judge; never pre-fill a verdict or resolve the dilemma.

---

## Task 1: Un-stub the three 画布导图 cards (argument-map, money-trail, source-map)

**Files:**
- Modify: `packages/contracts/cards/argument-map.json`, `packages/contracts/cards/money-trail.json`, `packages/contracts/cards/source-map.json`
- Test: `packages/contracts/test/canvasCards.test.ts` (create)

- [ ] **Step 1: Write the failing test**

```ts
import { describe, it, expect } from "vitest";
import { CardSpec } from "../src/cardSpec";
import argument from "../cards/argument-map.json";
import money from "../cards/money-trail.json";
import source from "../cards/source-map.json";

describe("画布导图 cards un-stubbed (composed)", () => {
  for (const [name, json] of [["argument-map", argument], ["money-trail", money], ["source-map", source]] as const) {
    it(`${name} is full and uses a repeatable_group`, () => {
      const c = CardSpec.parse(json);
      expect(c.body_status).toBe("full");
      expect(c.steps.flatMap((s) => s.fields).some((f) => f.type === "repeatable_group")).toBe(true);
    });
  }
});
```

- [ ] **Step 2: Run, verify FAIL.**

- [ ] **Step 3: Edit `argument-map.json`** — `body_status:"full"`; `steps[0].fields`:
```json
[
  { "type": "text", "key": "claim", "label": "你要分析的论点是什么？" },
  { "type": "repeatable_group", "key": "reasons", "label": "论据与隐藏假设", "item_fields": [
    { "type": "textarea", "key": "reason", "label": "一条论据" },
    { "type": "textarea", "key": "assumption", "label": "它默认了什么没说出来的假设？" },
    { "type": "single_choice", "key": "fallacy", "label": "可疑谬误（没有就选「无」）", "options": ["无", "稻草人", "滑坡", "诉诸权威", "以偏概全", "循环论证", "虚假两难"] },
    { "type": "textarea", "key": "fallacy_why", "label": "它符合这类谬误的哪一点？" }
  ] },
  { "type": "text", "key": "weakest", "label": "整条论证里最弱的环节是哪条？" },
  { "type": "textarea", "key": "strengthen", "label": "如果这是你自己的论证，你会怎么补强它？" }
]
```
Refresh `methodology_note` to: 「把论证拆成论点-论据-隐藏假设，给可疑环节贴谬误标签，再标出最弱的一环——若是你自己的论证，去补强它。」

- [ ] **Step 4: Edit `money-trail.json`** — `body_status:"full"`; `steps[0].fields`:
```json
[
  { "type": "text", "key": "claim", "label": "要溯源的说法是什么？" },
  { "type": "repeatable_group", "key": "chain", "label": "资金 / 利益链（逐节点添加）", "item_fields": [
    { "type": "text", "key": "who", "label": "节点（谁）" },
    { "type": "single_choice", "key": "role", "label": "这是哪一环", "options": ["发布者", "资助者", "母机构", "利益相关方"] },
    { "type": "textarea", "key": "source", "label": "它的钱 / 利益从哪来？" }
  ] },
  { "type": "single_choice", "key": "alignment", "label": "出资方的利益和这条结论的方向一致吗？", "options": ["一致", "不一致", "看不出"] },
  { "type": "textarea", "key": "meaning", "label": "这说明什么？（提醒：被资助 ≠ 必假，但利益一致时要找独立来源交叉验证）" }
]
```
Refresh `methodology_note` to: 「逐节点把说法溯到发布者→资助者→利益方，判断出资方利益和结论方向是否一致——不是『被资助即假』，而是标注利益、提高警惕、交叉验证。」

- [ ] **Step 5: Edit `source-map.json`** — `body_status:"full"`; `steps[0].fields`:
```json
[
  { "type": "repeatable_group", "key": "nodes", "label": "把材料 / 观点抽成节点", "item_fields": [
    { "type": "text", "key": "material", "label": "材料 / 观点" },
    { "type": "single_choice", "key": "node_type", "label": "节点类型", "options": ["观点", "来源", "报告", "出资方", "机构", "证据"] },
    { "type": "single_choice", "key": "bias", "label": "偏见标注", "options": ["未知 / 无", "政治", "商业", "地域", "叙事"] },
    { "type": "textarea", "key": "relation", "label": "它支持 / 反对 / 出自哪个节点？" }
  ] },
  { "type": "textarea", "key": "same_source", "label": "打开「偏见图层」看全局：哪些观点其实同源？谁在背后？" },
  { "type": "textarea", "key": "shift", "label": "看完这张图，你的判断有变化吗？" }
]
```
Refresh `methodology_note` to: 「把材料和观点抽成节点、标关系和偏见，看清哪些其实同源、谁在背后——所有材料都有立场，看见它才能更负责任地判断。」

- [ ] **Step 6: Gate** — `pnpm -r typecheck && pnpm -r test` → PASS (registry 33; update any test asserting these were stub).

- [ ] **Step 7: Commit**
```bash
git add packages/contracts && git commit -m "feat(cards): un-stub argument-map + money-trail + source-map (composed)"
```

---

## Task 2: Un-stub ethics-roleplay (composed)

**Files:**
- Modify: `packages/contracts/cards/ethics-roleplay.json`
- Test: `packages/contracts/test/roleplayCard.test.ts` (create)

- [ ] **Step 1: Write the failing test**

```ts
import { describe, it, expect } from "vitest";
import { CardSpec } from "../src/cardSpec";
import roleplay from "../cards/ethics-roleplay.json";

describe("ethics-roleplay un-stubbed (composed)", () => {
  it("is full and has a repeatable_group of roles + a closing field", () => {
    const c = CardSpec.parse(roleplay);
    expect(c.body_status).toBe("full");
    const fields = c.steps.flatMap((s) => s.fields);
    expect(fields.some((f) => f.type === "repeatable_group" && f.key === "roles")).toBe(true);
    expect(fields.some((f) => f.key === "closing")).toBe(true);
  });
});
```

- [ ] **Step 2: Run, verify FAIL.**

- [ ] **Step 3: Edit `ethics-roleplay.json`** — `body_status:"full"`; `steps[0].fields`:
```json
[
  { "type": "textarea", "key": "scenario", "label": "情景与核心问题（AI 给定背景，你写下要决断的核心问题）" },
  { "type": "repeatable_group", "key": "roles", "label": "各方角色立场板", "item_fields": [
    { "type": "text", "key": "name", "label": "角色" },
    { "type": "textarea", "key": "interest", "label": "它的利益" },
    { "type": "textarea", "key": "duty", "label": "它的责任" },
    { "type": "textarea", "key": "blindspot", "label": "它的盲区" }
  ] },
  { "type": "textarea", "key": "closing", "label": "综合各方：你认为 AI 该介入到哪一步？谁该负责？" }
]
```
Refresh `methodology_note` to: 「抽一个真实伦理情景，逐角色填它的利益/责任/盲区——站到对方的利益里，才看得见自己的盲区；最后综合各方做出你的判断。」

- [ ] **Step 4: Gate** — `pnpm -r typecheck && pnpm -r test` → PASS (registry 33).

- [ ] **Step 5: Commit**
```bash
git add packages/contracts && git commit -m "feat(cards): un-stub ethics-roleplay (composed) — completes S3c"
```

---

## Self-Review

**Spec coverage:** all four remaining stub cards un-stubbed (T1 the three 画布导图; T2 ethics-roleplay) → S3c complete (0 stub cards left). **Type consistency:** all fields use existing primitives; `single_choice` options are non-empty; `repeatable_group` item_fields valid. **Placeholder scan:** none; stale 「即将上线」 notes refreshed. **Note:** no code change, no new primitive — pure JSON; the free-form canvas/dialogue visualizations are logged as future enhancements in the spec.
