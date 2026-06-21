# Slice 5 — 评估那一刀 + 「你的思维印记」Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** A manual "生成思维印记" trigger runs a flagship LLM over the full conversation + envelopes → SOLO L1–L4 ratings across the 5 demo rubric dimensions + a process narrative, shown in the「你的思维印记」modal and persisted to the store.

**Architecture:** Rubric + `Evaluation` contracts in `@mind-imprint/contracts`. `evaluations` added to the B2.5 store (additive, backward-compatible). An eval engine in `apps/web/src/agent/` (input assembly → few-shot prompt → `chat(evalModel ?? model)` → JSON parse + one retry → persist) behind a reactive `createEvaluator` controller. The `evalLoading`/`showEval` modals in `apps/web/src/workspace/`, triggered from `WorkspaceView`.

**Tech Stack:** TypeScript 5, Zod 3, React 18 (`useSyncExternalStore`), Vitest + @testing-library/react (jsdom). Non-streaming, BYO-key, flagship not downgraded.

## Global Constraints

- **Acceptance gate:** `pnpm -r typecheck` AND `pnpm -r test` both green. `vite build`/`vitest` do NOT typecheck — run `pnpm --filter @mind-imprint/contracts typecheck` and/or `pnpm --filter web typecheck` per task.
- **Key-safety / flagship not downgraded:** eval uses `config.evalModel ?? config.model`; the API key stays only in request headers, never in `raw`/errors/logs/render (preserve the S2 boundary).
- **Frozen contracts unchanged** (`CardInstance`/`Message`/`Task`). `StoreState` gains `evaluations` **additively** with a `.default([])` so existing localStorage blobs still parse.
- **Eval is student-only** (no teacher surface). The narrative diagnoses + suggests a next step but never concludes for the student. 过程即数据: skipped cards still enter the eval input + narrative.
- **Strict JSON output** from the model (`{scores,narrative}`), zod-parsed, with exactly one retry on malformed output.
- **UI pixel-faithful** to `docs/design/思维印记_工作区.dc.html` (`evalLoading` / `showEval` blocks); `mk-*` tokens + real Phoebe content (NASA / Nature Sustainability / 碳排放 / 让步段), never lorem.
- Contracts tests in `packages/contracts/test/*`; web tests beside source.

---

### Task 1: Rubric + Evaluation contracts

**Files:**
- Modify: `packages/contracts/src/rubric.ts` (additive — keep existing `RUBRIC_TAGS`/`RubricTag`)
- Create: `packages/contracts/src/evaluation.ts`
- Modify: `packages/contracts/src/index.ts` (barrel)
- Test: `packages/contracts/test/evaluation.test.ts`

**Interfaces — Produces:** `SoloLevel`, `SOLO_LABELS`, `RubricDimension`, `DEMO_RUBRIC`; `DimScore`, `EvalLlmOutput`, `Evaluation` (Zod + types).

- [ ] **Step 1: Write the failing test** — `packages/contracts/test/evaluation.test.ts`:

```ts
import { describe, it, expect } from "vitest";
import { DEMO_RUBRIC, SoloLevel, SOLO_LABELS } from "../src/rubric";
import { Evaluation, EvalLlmOutput, DimScore } from "../src/evaluation";

describe("rubric", () => {
  it("DEMO_RUBRIC has the 5 demo dims, each with L1–L4 anchors", () => {
    expect(DEMO_RUBRIC.map((d) => d.id)).toEqual(["D2", "D3", "D4", "D5", "D6"]);
    for (const d of DEMO_RUBRIC) {
      expect(d.name).toBeTruthy();
      expect(d.framework).toBeTruthy();
      for (const lvl of ["L1", "L2", "L3", "L4"] as const) expect(d.anchors[lvl]).toBeTruthy();
    }
  });
  it("SoloLevel + labels", () => {
    expect(SoloLevel.safeParse("L4").success).toBe(true);
    expect(SoloLevel.safeParse("L5").success).toBe(false);
    expect(SOLO_LABELS.L1).toBe("萌芽");
  });
});

describe("evaluation contracts", () => {
  const score = { dim_id: "D2", level: "L4", note: "主动溯到 NASA / Nature Sustainability" };
  it("EvalLlmOutput accepts scores + narrative", () => {
    expect(EvalLlmOutput.safeParse({ scores: [score], narrative: "..." }).success).toBe(true);
  });
  it("DimScore rejects a bad level", () => {
    expect(DimScore.safeParse({ ...score, level: "L9" }).success).toBe(false);
  });
  it("Evaluation requires task_id + created_at", () => {
    expect(Evaluation.safeParse({ task_id: "t_1", scores: [score], narrative: "x", created_at: "2026-06-21T10:00:00.000Z" }).success).toBe(true);
    expect(Evaluation.safeParse({ scores: [score], narrative: "x" }).success).toBe(false);
  });
});
```

- [ ] **Step 2: Run to verify it fails** — `pnpm --filter @mind-imprint/contracts exec vitest run test/evaluation.test.ts` → FAIL.

- [ ] **Step 3: Implement**

Append to `packages/contracts/src/rubric.ts` (keep the existing `RUBRIC_TAGS`/`RubricTag`):

```ts
import { z } from "zod";

export const SoloLevel = z.enum(["L1", "L2", "L3", "L4"]);
export type SoloLevel = z.infer<typeof SoloLevel>;
export const SOLO_LABELS: Record<SoloLevel, string> = { L1: "萌芽", L2: "发展中", L3: "熟练", L4: "卓越" };

export interface RubricDimension {
  id: string; name: string; framework: string;
  anchors: { L1: string; L2: string; L3: string; L4: string };
}

export const DEMO_RUBRIC: RubricDimension[] = [
  { id: "D2", name: "信源辨识", framework: "媒介/信息素养 · CRAAP",
    anchors: { L1: "完全信任 AI / 来源，从不追问出处", L2: "偶尔问「真的吗？」但不深入", L3: "主动要求论据，能识别来源等级", L4: "主动交叉验证，识别信源之间的利益关系与冲突" } },
  { id: "D3", name: "横向验证", framework: "ATL 研究 · 横向阅读 SHEG",
    anchors: { L1: "只看单一来源，不另开查证", L2: "想到要多看，但没真去找", L3: "主动多源对照，找到 2+ 独立来源", L4: "溯到原始出处，比较各源权威性与一致性" } },
  { id: "D4", name: "多视角与让步", framework: "QUEST-E · 论证评估",
    anchors: { L1: "只站自己一方，无视反方", L2: "提到反方但轻描淡写 / 稻草人", L3: "主动找反方并正面回应", L4: "构建反方最强论证(steelman)后再让步反驳" } },
  { id: "D5", name: "论证拆解", framework: "QUEST-U · 论证分析",
    anchors: { L1: "把观点当事实，不分论点论据", L2: "能复述但不辨结构", L3: "能识别论点-论据-假设结构", L4: "识别隐藏前提与论证谬误" } },
  { id: "D6", name: "反思与元认知", framework: "ATL 反思 · TOK 认知者与知识",
    anchors: { L1: "不觉察自己被 AI 影响", L2: "事后偶尔回顾", L3: "主动校准信心，觉察思维盲点", L4: "觉察自己作为认知者的位置，迁移方法" } },
];
```

Create `packages/contracts/src/evaluation.ts`:

```ts
import { z } from "zod";
import { SoloLevel } from "./rubric";

export const DimScore = z.object({ dim_id: z.string(), level: SoloLevel, note: z.string() });
export const EvalLlmOutput = z.object({ scores: z.array(DimScore), narrative: z.string() });
export const Evaluation = z.object({
  task_id: z.string(),
  scores: z.array(DimScore),
  narrative: z.string(),
  created_at: z.string(),
});

export type DimScore = z.infer<typeof DimScore>;
export type EvalLlmOutput = z.infer<typeof EvalLlmOutput>;
export type Evaluation = z.infer<typeof Evaluation>;
```

Add `export * from "./evaluation";` to `packages/contracts/src/index.ts` (the rubric is already exported via `./rubric`).

- [ ] **Step 4: Run test + typecheck** — test PASS; `pnpm --filter @mind-imprint/contracts typecheck` clean.
- [ ] **Step 5: Commit** — `git add -A && git commit -m "feat(contracts): rubric dimensions + Evaluation schema"`

---

### Task 2: Store `evaluations` (additive, backward-compatible)

**Files:**
- Modify: `apps/web/src/store/schema.ts`
- Modify: `apps/web/src/store/createStore.ts`
- Test: `apps/web/src/store/createStore.evaluations.test.ts`

**Interfaces:**
- Consumes: `Evaluation` from contracts.
- Produces: `StoreState` gains `evaluations: z.array(Evaluation).default([])`; `Store` gains `putEvaluation(e: Evaluation): void` (append), `listEvaluations(task_id: string): Evaluation[]`, `getLatestEvaluation(task_id: string): Evaluation | undefined`.

- [ ] **Step 1: Write the failing test** — `apps/web/src/store/createStore.evaluations.test.ts`:

```ts
import { describe, it, expect } from "vitest";
import { createStore } from "./createStore";
import { makeMemoryStorage, STORE_KEY } from "./storage";
import type { Evaluation } from "@mind-imprint/contracts";

function ev(task_id: string, at: string, narrative: string): Evaluation {
  return { task_id, scores: [{ dim_id: "D2", level: "L4", note: "n" }], narrative, created_at: at };
}

describe("store evaluations", () => {
  it("appends + lists + latest by created_at", () => {
    const store = createStore({ storage: makeMemoryStorage() });
    store.putEvaluation(ev("t_1", "2026-06-21T10:00:00.000Z", "first"));
    store.putEvaluation(ev("t_1", "2026-06-21T11:00:00.000Z", "second"));
    store.putEvaluation(ev("t_2", "2026-06-21T10:30:00.000Z", "other"));
    expect(store.listEvaluations("t_1")).toHaveLength(2);
    expect(store.getLatestEvaluation("t_1")!.narrative).toBe("second");
    expect(store.getLatestEvaluation("t_x")).toBeUndefined();
  });

  it("loads a legacy blob with no evaluations field (backward compatible)", () => {
    const storage = makeMemoryStorage();
    storage.setItem(STORE_KEY, JSON.stringify({ version: 1, tasks: [], messages: [], cards: [] }));
    const store = createStore({ storage });
    expect(store.listEvaluations("t_1")).toEqual([]);
  });
});
```

- [ ] **Step 2: Run to verify it fails** — `pnpm --filter web exec vitest run src/store/createStore.evaluations.test.ts` → FAIL.

- [ ] **Step 3: Implement**
  - `schema.ts`: import `Evaluation`; add `evaluations: z.array(Evaluation).default([])` to `StoreState`; add `evaluations: []` to `EMPTY_STATE`.
  - `createStore.ts`: import `Evaluation`; add to the `Store` interface `putEvaluation`/`listEvaluations`/`getLatestEvaluation`; implement (mirroring the card methods) — `putEvaluation` commits `{ ...state, evaluations: [...state.evaluations, e] }`; `listEvaluations` filters by `task_id`; `getLatestEvaluation` returns the max-`created_at` for the task or undefined.

- [ ] **Step 4: Run test + typecheck** — test PASS; existing store tests green; `pnpm --filter web typecheck` clean.
- [ ] **Step 5: Commit** — `git commit -am "feat(store): evaluations collection (additive, backward-compatible)"`

---

### Task 3: `assembleEvalInput` (pure)

**Files:**
- Create: `apps/web/src/agent/evalInput.ts`
- Test: `apps/web/src/agent/evalInput.test.ts`

**Interfaces:**
- Consumes: `Message`/`CardInstance`/`CardSpec`, `serializeCardForRefeed` from contracts; the store's read methods (pass values, keep it pure).
- Produces: `assembleEvalInput(opts: { messages: Message[]; cards: CardInstance[]; registry: Record<string,CardSpec> }): string` — a text blob with the conversation transcript + per-card envelope dump (name + status + filled content + a short event_trace note).

- [ ] **Step 1: Write the failing test** — assert the output contains: a user line and an assistant line from the transcript; for a completed sift_craap card, the card name + a filled value (e.g. "NASA"); for a skipped card, a skip signal ("跳过"). Keep it a behavioral string-contains test.
- [ ] **Step 2: Run to verify it fails** — `pnpm --filter web exec vitest run src/agent/evalInput.test.ts` → FAIL.
- [ ] **Step 3: Implement** — build two sections: `## 对话` (each message as `学生：…` / `陪练：…` / tool turns summarized), and `## 工具卡` (each card: `【{spec.name}】 {status}` + `JSON.stringify(serializeCardForRefeed(spec, card))` for completed/skipped + a one-line `event_trace` count note like `（共 N 个操作事件）`). Pure function over its args.
- [ ] **Step 4: Run test + typecheck** — PASS; `pnpm --filter web typecheck` clean.
- [ ] **Step 5: Commit** — `git commit -am "feat(agent): assembleEvalInput (conversation + envelopes)"`

---

### Task 4: `buildEvalPrompt` (rubric + SOLO anchors + Phoebe few-shot)

**Files:**
- Create: `apps/web/src/agent/evalPrompt.ts`
- Test: `apps/web/src/agent/evalPrompt.test.ts`

**Interfaces:**
- Consumes: `RubricDimension`/`SOLO_LABELS`/`DEMO_RUBRIC` from contracts.
- Produces: `buildEvalPrompt(rubric: RubricDimension[]): string`.

- [ ] **Step 1: Write the failing test** — assert the prompt contains: each dim's name + its L1 and L4 anchor text; the SOLO labels (萌芽/卓越); the Phoebe few-shot marker (e.g. "Phoebe" or "示例"); and the strict-JSON instruction with the keys `scores`/`narrative` and `dim_id`/`level`/`note`.
- [ ] **Step 2: Run to verify it fails** — `pnpm --filter web exec vitest run src/agent/evalPrompt.test.ts` → FAIL.
- [ ] **Step 3: Implement** — compose the eval system prompt:
  - **角色**: 旗舰评估官；只给学生看；基于完整对话 + 标准信封，按 SOLO 四级评每一维 + 写一段过程叙述（诊断 + 一个下一步），不替学生定论。
  - **rubric**: for each `RubricDimension`, render `[{id}] {name}（{framework}）` + the 4 anchors `L1 {萌芽}: …` … `L4 {卓越}: …`.
  - **few-shot (Phoebe)**: a compact worked example — input gist (想引用「中国让地球变绿」公众号文 → SIFT 溯到 NASA / Nature Sustainability(IF 32.1) → 撞碳排放反例 → 写让步段) → expected JSON output `{"scores":[{"dim_id":"D2","level":"L4","note":"…"},{"dim_id":"D3","level":"L4",…},{"dim_id":"D4","level":"L4",…},{"dim_id":"D5","level":"L3",…},{"dim_id":"D6","level":"L3",…}],"narrative":"…来源意识从被动转主动…正面接住反例写让步段=L4…下一步可问各来源立场…"}`. Real Phoebe content.
  - **输出约束**: 只输出一个 JSON，形如 `{"scores":[{"dim_id","level","note"}],"narrative":""}`；`dim_id` 必须是上面 rubric 的维度 id；`level` ∈ L1–L4；不要输出 JSON 以外的任何文字。
- [ ] **Step 4: Run test + typecheck** — PASS; `pnpm --filter web typecheck` clean.
- [ ] **Step 5: Commit** — `git commit -am "feat(agent): buildEvalPrompt (rubric + SOLO anchors + Phoebe few-shot)"`

---

### Task 5: `runEvaluation` (assemble → chat → parse + retry → persist)

**Files:**
- Create: `apps/web/src/agent/runEvaluation.ts`
- Test: `apps/web/src/agent/runEvaluation.test.ts`

**Interfaces:**
- Consumes: `assembleEvalInput` (T3), `buildEvalPrompt` (T4); `EvalLlmOutput`/`Evaluation`/`DEMO_RUBRIC` from contracts; the store + `chat`/`LlmConfig` from llm.
- Produces:
  ```ts
  runEvaluation(deps: { store: Store; chat: ChatFn; config: Partial<LlmConfig>; registry: Record<string,CardSpec>; taskId: string; now?: () => string }): Promise<Evaluation>
  ```

- [ ] **Step 1: Write the failing test** — with a memory store (seeded task + a couple messages + a completed sift_craap card) and a **fake chat** capturing the request:
  - scripted valid JSON output → `runEvaluation` returns an `Evaluation` (scores + narrative), persisted via `store.getLatestEvaluation(taskId)`, and the chat was called with `model === config.evalModel` (set `evalModel` in the test config).
  - first response malformed (`"not json"`), second valid → succeeds (one retry).
  - both malformed → throws.
  - assert the request `messages` contain the system prompt (a rubric dim name) + the user input (a card name).
- [ ] **Step 2: Run to verify it fails** — `pnpm --filter web exec vitest run src/agent/runEvaluation.test.ts` → FAIL.
- [ ] **Step 3: Implement**:
  - `system = buildEvalPrompt(DEMO_RUBRIC)`; `user = assembleEvalInput({ messages: store.listMessages(taskId), cards: store.listCards(taskId), registry })`.
  - `const model = config.evalModel ?? config.model;` call `chat({ ...config, model }, { messages: [{role:"system",content:system},{role:"user",content:user}], maxTokens: 1500 })`.
  - Parse: a helper that `JSON.parse`es the text (strip ```json fences if present) and `EvalLlmOutput.parse`es it; on failure, retry the chat once; on second failure, throw `new Error("评估输出解析失败")` (the controller turns this into phase "error").
  - Build `const evaluation: Evaluation = { task_id: taskId, scores: out.scores, narrative: out.narrative, created_at: (now ?? (() => new Date().toISOString()))() }`; `store.putEvaluation(evaluation)`; return it.
- [ ] **Step 4: Run test + typecheck** — PASS; `pnpm --filter web typecheck` clean.
- [ ] **Step 5: Commit** — `git commit -am "feat(agent): runEvaluation (flagship eval + parse/retry + persist)"`

---

### Task 6: `createEvaluator` controller + `useEvaluator`

**Files:**
- Create: `apps/web/src/agent/createEvaluator.ts`, `apps/web/src/agent/useEvaluator.ts`
- Modify: `apps/web/src/agent/index.ts` (barrel)
- Test: `apps/web/src/agent/createEvaluator.test.ts`

**Interfaces:**
- Produces:
  ```ts
  type EvalPhase = "idle" | "running" | "done" | "error";
  interface EvalState { phase: EvalPhase; evaluation?: Evaluation; error?: string }
  interface Evaluator { getSnapshot(): EvalState; subscribe(l:()=>void):()=>void; run(): Promise<void> }
  function createEvaluator(deps: { store; chat; config; registry; taskId; now? }): Evaluator
  useEvaluator(ev: Evaluator): EvalState
  ```

- [ ] **Step 1: Write the failing test** — fake chat: `run()` → phase goes `running` then `done` with `evaluation` set; a fake chat that throws → phase `error`, `error` set, `run()` resolves (does not throw). Reactive: a subscriber fires on phase change; `getSnapshot` is a stable reference between changes.
- [ ] **Step 2: Run to verify it fails** — `pnpm --filter web exec vitest run src/agent/createEvaluator.test.ts` → FAIL.
- [ ] **Step 3: Implement** — a reactive controller mirroring `createConversation` (immutable `EvalState`, `getSnapshot`/`subscribe`/notify). `run()`: set `running` → `await runEvaluation(deps)` → set `done` + evaluation; catch → set `error` + message (never throw). `useEvaluator` = `useSyncExternalStore(ev.subscribe, ev.getSnapshot)`. Add all of agent's eval exports to `index.ts`.
- [ ] **Step 4: Run test + typecheck** — PASS; full `src/agent` suite green; `pnpm --filter web typecheck` clean.
- [ ] **Step 5: Commit** — `git commit -am "feat(agent): createEvaluator controller + useEvaluator"`

---

### Task 7: `EvalModal` + `EvalLoading` (UI)

**Files:**
- Create: `apps/web/src/workspace/EvalModal.tsx`, `apps/web/src/workspace/EvalLoading.tsx`
- Create: `apps/web/src/workspace/evalView.ts` (maps `Evaluation` → view model)
- Test: `apps/web/src/workspace/EvalModal.test.tsx`, `apps/web/src/workspace/evalView.test.ts`

**Interfaces:**
- Consumes: `Evaluation`/`DEMO_RUBRIC`/`SOLO_LABELS` from contracts.
- Produces: `evalView(evaluation, rubric): { dims: Array<{ dim: string; levelLabel: string; segs: Array<{filled:boolean}>; note: string }> }` (dim name from rubric, `levelLabel` like `L4 · 卓越`, `segs` = 4 bars with the first N filled where N = level number); `EvalModal({ evaluation, onClose })`; `EvalLoading()`.

- [ ] **Step 1: Write the failing tests**
  - `evalView.test.ts`: a score `{dim_id:"D4", level:"L4", note}` → `levelLabel` contains `L4` and `卓越`, `segs` has 4 entries with 4 filled; `level:"L2"` → 2 filled; dim name resolved from `DEMO_RUBRIC` (多视角与让步).
  - `EvalModal.test.tsx` (RTL): given an `Evaluation` with 5 scores + a narrative, renders 你的思维印记 header, each dim name + its levelLabel, the narrative text, and a 回到任务 button that calls `onClose`.
- [ ] **Step 2: Run to verify they fail** — `pnpm --filter web exec vitest run src/workspace/EvalModal.test.tsx src/workspace/evalView.test.ts` → FAIL.
- [ ] **Step 3: Implement** — lift the `showEval` and `evalLoading` markup verbatim from `docs/design/思维印记_工作区.dc.html` (deep-blue gradient header + SOLO line; the `evalDims` `sc-for` with dim/levelLabel/segs/note; the 过程叙述 card; close + 回到任务). `evalView` builds the `dims`/`segs`. `EvalLoading` = the scrim + dots + 旗舰模型正在评估…… text. `mk-*` tokens; real content.
- [ ] **Step 4: Run tests + typecheck** — PASS; `pnpm --filter web typecheck` clean.
- [ ] **Step 5: Commit** — `git commit -am "feat(workspace): EvalModal + EvalLoading"`

---

### Task 8: Trigger wiring into `WorkspaceView`

**Files:**
- Modify: `apps/web/src/workspace/WorkspaceView.tsx`
- Modify: `apps/web/src/workspace/index.ts` (export the new pieces if needed)
- Test: `apps/web/src/workspace/WorkspaceView.test.tsx` (extend)

**Interfaces:** consumes `createEvaluator`/`useEvaluator` (T6), `EvalModal`/`EvalLoading` (T7).

- [ ] **Step 1: Write the failing test** — extend `WorkspaceView.test.tsx` (with an in-memory store + a **fake evaluator** prop, or a real evaluator over a fake chat): a 「生成思维印记」 button is present in the breadcrumb; clicking it shows `EvalLoading` while running and then the `EvalModal` with the dims; closing returns to the workspace. (No live LLM — drive via the fake.)
- [ ] **Step 2: Run to verify it fails** — `pnpm --filter web exec vitest run src/workspace/WorkspaceView.test.tsx` → FAIL.
- [ ] **Step 3: Implement** — `WorkspaceView` constructs/receives an `Evaluator` (accept an optional `evaluator` prop for testability, defaulting to one built from its store/chat/config/taskId), reads `useEvaluator(evaluator)`, renders a 「生成思维印记」 breadcrumb button → `evaluator.run()`, and renders `<EvalLoading/>` when `phase==="running"` and `<EvalModal evaluation={state.evaluation} onClose={...}/>` when `phase==="done"`. Keep existing WorkspaceView tests green.
- [ ] **Step 4: Run test + typecheck** — `pnpm --filter web exec vitest run src/workspace/WorkspaceView.test.tsx` PASS; full web suite green; `pnpm --filter web typecheck` clean.
- [ ] **Step 5: Commit** — `git commit -am "feat(workspace): 生成思维印记 trigger + eval modal wiring"`

---

### Final verification

```bash
pnpm -r typecheck
pnpm -r test
```
Both green. Optional manual smoke (with a real key + the 工作区 dev tab): run the Phoebe artery, click 生成思维印记 → loading → 你的思维印记 modal with SOLO ratings + narrative.

## Self-Review

**Spec coverage:** rubric (5 dims, SOLO) + Evaluation contracts (T1) ✓; store evaluations additive + backward-compatible (T2) ✓; eval input assembly (T3) ✓; few-shot prompt (T4) ✓; runEvaluation flagship + parse/retry + persist (T5) ✓; reactive evaluator + hook (T6) ✓; EvalModal/EvalLoading + evalView (T7) ✓; trigger wiring (T8) ✓. Deferred (spec §8): semantic tree enrichment, Marcus/Ethan/Eliza few-shot expansion, benchmark — recorded in `docs/遗留项追踪_Carryforward.md`.

**Placeholder scan:** T1–T6 ship complete code/logic. T7–T8 (UI) specify exact behavior + tests and instruct lifting the modal markup verbatim from the binding HTML (the project's UI-source-of-truth rule). The Phoebe few-shot content in T4 is the spec-approved draft. No "TBD"/"add later" in any logic step.

**Type consistency:** `SoloLevel`/`DimScore`/`Evaluation`/`EvalLlmOutput` (T1) flow unchanged through the store (T2), `runEvaluation` (T5), the controller (T6), and `evalView` (T7). `runEvaluation`'s deps shape matches the `createEvaluator` deps (T6) and the `WorkspaceView` construction (T8). `DEMO_RUBRIC` feeds T4 + T7.
