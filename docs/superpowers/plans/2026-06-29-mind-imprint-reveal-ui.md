# 「你的思维印记」Reveal UI Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Surface the async, v2 (10-dim) evaluation to the student as a hierarchical「你的思维印记」reveal (2 faces → 4 categories → 10 dims), with N/A neutral, a polling evaluator, and a quiet "offer don't push" indicator for milestone-auto-triggered evals.

**Architecture:** Sync the stale TS contracts to the shipped backend (NA level, status, D10, a new `cognitive-model` rollup tree + `assembleImprint` helper). Rewrite the evaluator to poll the async lifecycle. Rebuild `EvalModal` as a hierarchical drill-down assembled from `assembleImprint`, keeping the `.dc.html` visual styling. Add a `lastSeenEvaluationAt` store marker, a 30 s background poll, and a quiet indicator chip.

**Tech Stack:** React + Vite + TypeScript + Tailwind (web, pkg name `web`), Zod contracts (pkg `@mind-imprint/contracts`), vitest + @testing-library/react.

## Global Constraints

- **铁律 #2 不操纵:** no live score, no badges, no streaks, no counts that aggregate into "a number", no auto-popup. Faces/categories show a **factual coverage chip only** (`N 项已评 · M 未涉及`) — never a computed level. The indicator never auto-opens, has no badge count, and is dismissable.
- **N/A renders neutral:** an N/A dim shows `本次未涉及`, greyed, **no bar fill**, counted in the `未涉及` tally — never a low/zero bar. An all-N/A eval reads as `进行中` (in progress), not broken.
- **Design-source split:** visual styling (gradient header, `仅你可见`, SOLO explainer, `#D98263` pill, 4-seg bar, `过程叙述` card, `回到任务`) follows `docs/design/思维印记_工作区.dc.html` verbatim; the hierarchical **structure** follows the eval-model spec §7. Do not edit the `.dc.html`.
- **Level words:** L1 萌芽 · L2 发展中 · L3 熟练 · L4 卓越. The header SOLO explainer keeps the `.dc.html` wording `L1 单点 · L2 多点 · L3 关联 · L4 拓展`.
- **Backend-internal fields** (`signals`, `trigger`, `trigger_milestone`) are NOT in the DTO/contract.
- **Out of scope:** teacher views, class aggregates, longitudinal trend, rewriting D1–D9 stale anchors.
- **Commands** (run from `/Users/houyuxin/08Coding/mind-imprint`): monorepo types `pnpm -r typecheck`; contracts tests `pnpm --filter @mind-imprint/contracts test`; web tests `pnpm --filter web test`; focused web test `pnpm --filter web exec vitest run <path>`; focused contracts test `pnpm --filter @mind-imprint/contracts exec vitest run <path>`. Full gate: `pnpm -r typecheck && pnpm -r test`.
- **Never stage/commit the repo-root `package.json`** (pre-existing unrelated `M`). Use explicit `git add <paths>`.
- Commit trailer exactly: `Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>`.

---

### Task 1: Contract sync — NA level, status, D10, keep monorepo green

**Files:**
- Modify: `packages/contracts/src/rubric.ts` (SoloLevel +NA, ScoredLevel, narrow SOLO_LABELS, add D10)
- Modify: `packages/contracts/src/evaluation.ts` (Evaluation +id/status/completed_at)
- Modify: `apps/web/src/workspace/evalView.ts` (NA-safe; ScoredLevel)
- Modify: `apps/web/src/shell/records/ability.ts` (NA-safe; ScoredLevel)
- Test: `packages/contracts/src/evaluation.test.ts` (new); existing `apps/web/src/workspace/evalView.test.ts` (fixtures)

**Interfaces:**
- Produces: `SoloLevel = "L1"|"L2"|"L3"|"L4"|"NA"`; `ScoredLevel = "L1"|"L2"|"L3"|"L4"`; `SOLO_LABELS: Record<ScoredLevel,string>`; `FULL_RUBRIC` includes `D10`; `Evaluation` gains `id: string`, `status: "queued"|"running"|"done"|"failed"`, `completed_at: string | null` (all with parse defaults so server/legacy data both validate, but present in the inferred type).

- [ ] **Step 1: Write the failing contract test**

Create `packages/contracts/src/evaluation.test.ts`:

```ts
import { describe, it, expect } from "vitest";
import { Evaluation } from "./evaluation";

describe("Evaluation contract (v2)", () => {
  it("accepts an NA level and a status field", () => {
    const parsed = Evaluation.parse({
      id: "ev1", task_id: "t1", status: "done",
      scores: [{ dim_id: "D2", level: "NA", note: "" }, { dim_id: "D7", level: "L3", note: "好" }],
      narrative: "n", created_at: "2026-06-29T00:00:00.000Z", completed_at: "2026-06-29T00:00:10.000Z",
    });
    expect(parsed.status).toBe("done");
    expect(parsed.scores[0]!.level).toBe("NA");
  });

  it("defaults status to 'done' and the new fields when absent (legacy/fixture data)", () => {
    const parsed = Evaluation.parse({
      task_id: "t1", scores: [], narrative: "n", created_at: "2026-06-29T00:00:00.000Z",
    });
    expect(parsed.status).toBe("done");
    expect(parsed.id).toBe("");
    expect(parsed.completed_at).toBeNull();
  });
});
```

- [ ] **Step 2: Run it to verify it fails**

Run: `pnpm --filter @mind-imprint/contracts exec vitest run src/evaluation.test.ts`
Expected: FAIL (Evaluation rejects `level:"NA"` and has no `status`).

- [ ] **Step 3: Update `rubric.ts`**

Replace the top of `packages/contracts/src/rubric.ts` (the `SoloLevel`/`SOLO_LABELS` block) with:

```ts
import { z } from "zod";

export const SoloLevel = z.enum(["L1", "L2", "L3", "L4", "NA"]);
export type SoloLevel = z.infer<typeof SoloLevel>;

// A scored level excludes N/A (N/A = insufficient evidence, rendered neutrally, not on the bar).
export type ScoredLevel = Exclude<SoloLevel, "NA">;
export const SOLO_LABELS: Record<ScoredLevel, string> = { L1: "萌芽", L2: "发展中", L3: "熟练", L4: "卓越" };
```

Then append `D10` to the `FULL_RUBRIC` array (after the `D9` entry, before the closing `]`):

```ts
  { id: "D10", name: "协作编排", framework: "意图与编排 · 跨轮驱动与贡献",
    anchors: { L1: "把 AI 当答案机器：直接要成品，不带入自己的材料，不追问不调整", L2: "被 AI 追问后才补充自己的材料，不主动规划协作步骤", L3: "未经提示就带入自己的草稿/链接/提纲，并跨轮驱动改进", L4: "跨步骤编排 AI 角色、管理上下文、沉淀可复用结构" } },
```

- [ ] **Step 4: Update `evaluation.ts`**

Replace `packages/contracts/src/evaluation.ts` with:

```ts
import { z } from "zod";
import { SoloLevel } from "./rubric";

export const DimScore = z.object({ dim_id: z.string(), level: SoloLevel, note: z.string() });
export const EvalLlmOutput = z.object({ scores: z.array(DimScore), narrative: z.string() });

export const EvalStatus = z.enum(["queued", "running", "done", "failed"]);
export type EvalStatus = z.infer<typeof EvalStatus>;

export const Evaluation = z.object({
  // id/status/completed_at default so server payloads AND older fixtures both validate;
  // the server always sends real values.
  id: z.string().default(""),
  task_id: z.string(),
  status: EvalStatus.default("done"),
  scores: z.array(DimScore),
  narrative: z.string(),
  created_at: z.string(),
  completed_at: z.string().nullable().default(null),
});

export type DimScore = z.infer<typeof DimScore>;
export type EvalLlmOutput = z.infer<typeof EvalLlmOutput>;
export type Evaluation = z.infer<typeof Evaluation>;
```

- [ ] **Step 5: Run the contract test — verify it passes**

Run: `pnpm --filter @mind-imprint/contracts exec vitest run src/evaluation.test.ts`
Expected: PASS.

- [ ] **Step 6: Make the two web consumers NA-safe (keep web typecheck green)**

In `apps/web/src/workspace/evalView.ts`, change the `LEVEL_NUMBER` map and skip NA scores:

```ts
import type { Evaluation, RubricDimension, ScoredLevel } from "@mind-imprint/contracts";
import { SOLO_LABELS } from "@mind-imprint/contracts";
```
```ts
const LEVEL_NUMBER: Record<ScoredLevel, number> = { L1: 1, L2: 2, L3: 3, L4: 4 };

export function evalView(evaluation: Evaluation, rubric: RubricDimension[]): EvalView {
  const dims: EvalDimView[] = evaluation.scores
    .filter((score) => score.level !== "NA")
    .map((score) => {
      const level = score.level as ScoredLevel;
      const rubricEntry = rubric.find((r) => r.id === score.dim_id);
      const dim = rubricEntry ? rubricEntry.name : score.dim_id;
      const levelLabel = `${level} · ${SOLO_LABELS[level]}`;
      const n = LEVEL_NUMBER[level];
      const segs = [1, 2, 3, 4].map((i) => ({ filled: i <= n }));
      return { dim, levelLabel, segs, note: score.note };
    });
  return { dims };
}
```

In `apps/web/src/shell/records/ability.ts`, narrow the map and skip NA scores when building `latest`:

```ts
import type { Evaluation, RubricDimension, ScoredLevel } from "@mind-imprint/contracts";
import { SOLO_LABELS } from "@mind-imprint/contracts";

const LEVEL_NUMBER: Record<ScoredLevel, number> = { L1: 1, L2: 2, L3: 3, L4: 4 };
```
In `deriveAbility`, change the inner loop to skip NA and type `latest` as `ScoredLevel`:
```ts
  const latest = new Map<string, { level: ScoredLevel; at: string }>();
  for (const e of evaluations) {
    for (const s of e.scores) {
      if (s.level === "NA") continue;
      const prev = latest.get(s.dim_id);
      if (!prev || e.created_at > prev.at) latest.set(s.dim_id, { level: s.level, at: e.created_at });
    }
  }
```
The `AbilityDim.level` field type changes from `SoloLevel` to `ScoredLevel`; update its import/usage (`const level = latest.get(d.id)?.level ?? "L1";` stays valid since `"L1"` is a `ScoredLevel`).

- [ ] **Step 7: Fix the surviving `evalView.test.ts` fixtures**

`apps/web/src/workspace/evalView.test.ts`'s `baseEvaluation` (and any other inline `Evaluation` fixtures in that file) omit the new fields. Add them to `baseEvaluation`:

```ts
const baseEvaluation: Evaluation = {
  id: "ev-fixture", task_id: "task-phoebe-001", status: "done", completed_at: null,
  narrative: "…",  // keep existing narrative text
  created_at: "2026-06-21T00:00:00.000Z",
  scores: [],
};
```
(Spread fixtures like `{ ...baseEvaluation, scores: [...] }` inherit the new fields automatically.)

- [ ] **Step 8: Typecheck the whole monorepo + run both test suites**

Run: `pnpm -r typecheck && pnpm --filter @mind-imprint/contracts test && pnpm --filter web test`
Expected: typecheck clean across packages; all tests pass. If another file constructs a bare `Evaluation` literal and fails typecheck, add the same three fields there (search: `grep -rln "task_id:" apps/web/src | xargs grep -l "scores:"`).

- [ ] **Step 9: Commit**

```bash
cd /Users/houyuxin/08Coding/mind-imprint
git add packages/contracts/src/rubric.ts packages/contracts/src/evaluation.ts \
        packages/contracts/src/evaluation.test.ts \
        apps/web/src/workspace/evalView.ts apps/web/src/workspace/evalView.test.ts \
        apps/web/src/shell/records/ability.ts
git commit -m "feat(contracts): v2 eval sync — NA level, status, D10 (reveal UI)

Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>"
```

---

### Task 2: Cognitive-model rollup tree + `assembleImprint`

**Files:**
- Create: `packages/contracts/src/cognitive-model.ts`
- Modify: `packages/contracts/src/index.ts` (export the new module)
- Test: `packages/contracts/src/cognitive-model.test.ts` (new)

**Interfaces:**
- Consumes: `Evaluation`, `FULL_RUBRIC`, `SoloLevel` (Task 1).
- Produces: `COGNITIVE_MODEL: Face[]`; `assembleImprint(evaluation: Evaluation): AssembledImprint`. Types:
  `AssembledDim { dimId: string; name: string; level: SoloLevel; note: string }` (absent dim → `{level:"NA", note:""}`);
  `AssembledCategory { id; label; dims: AssembledDim[]; scored: number; na: number }`;
  `AssembledFace { id; label; icon; categories: AssembledCategory[]; scored: number; na: number }`;
  `AssembledImprint { faces: AssembledFace[] }`. `scored` = count of dims with level ∈ L1–L4; `na` = count with level `NA`.

- [ ] **Step 1: Write the failing test**

Create `packages/contracts/src/cognitive-model.test.ts`:

```ts
import { describe, it, expect } from "vitest";
import { COGNITIVE_MODEL, assembleImprint } from "./cognitive-model";
import { FULL_RUBRIC } from "./rubric";
import type { Evaluation } from "./evaluation";

describe("COGNITIVE_MODEL invariant", () => {
  it("maps every D1..D10 exactly once (no orphan, no dup, all present)", () => {
    const dimIds = COGNITIVE_MODEL.flatMap((f) => f.categories.flatMap((c) => c.dimIds));
    const expected = FULL_RUBRIC.map((d) => d.id).sort();
    expect([...dimIds].sort()).toEqual(expected);
    expect(new Set(dimIds).size).toBe(dimIds.length);
    expect(dimIds.length).toBe(10);
  });
});

describe("assembleImprint", () => {
  const evaluation: Evaluation = {
    id: "ev1", task_id: "t1", status: "done", completed_at: null,
    created_at: "2026-06-29T00:00:00.000Z", narrative: "n",
    scores: [
      { dim_id: "D1", level: "L3", note: "清晰" },
      { dim_id: "D10", level: "L2", note: "补充" },
      { dim_id: "D4", level: "NA", note: "" },
      // D5, D7 and the entire 批判式防护 face absent → treated as NA
    ],
  };

  it("joins scores onto the tree and resolves dim names from FULL_RUBRIC", () => {
    const out = assembleImprint(evaluation);
    const driving = out.faces.find((f) => f.id === "driving")!;
    const intent = driving.categories.find((c) => c.id === "intent")!;
    expect(intent.dims.map((d) => d.dimId)).toEqual(["D1", "D10"]);
    expect(intent.dims[0]!.name).toBe("提问清晰度");
    expect(intent.dims[0]!.level).toBe("L3");
  });

  it("treats an absent dim as NA with empty note, and counts scored vs na", () => {
    const out = assembleImprint(evaluation);
    const driving = out.faces.find((f) => f.id === "driving")!;
    const reasoning = driving.categories.find((c) => c.id === "reasoning")!;
    const d5 = reasoning.dims.find((d) => d.dimId === "D5")!;
    expect(d5.level).toBe("NA");
    expect(d5.note).toBe("");
    // intent: D1 L3 (scored), D10 L2 (scored) → scored 2, na 0
    const intent = driving.categories.find((c) => c.id === "intent")!;
    expect(intent.scored).toBe(2);
    expect(intent.na).toBe(0);
    // reasoning: D4 NA, D5 absent→NA, D7 absent→NA → scored 0, na 3
    expect(reasoning.scored).toBe(0);
    expect(reasoning.na).toBe(3);
    // face rollup sums its categories
    expect(driving.scored).toBe(2);
    expect(driving.na).toBe(3);
  });
});
```

- [ ] **Step 2: Run it to verify it fails**

Run: `pnpm --filter @mind-imprint/contracts exec vitest run src/cognitive-model.test.ts`
Expected: FAIL (`cognitive-model` not found).

- [ ] **Step 3: Implement `cognitive-model.ts`**

Create `packages/contracts/src/cognitive-model.ts`:

```ts
import type { Evaluation } from "./evaluation";
import type { SoloLevel } from "./rubric";
import { FULL_RUBRIC } from "./rubric";

export interface Category { id: string; label: string; dimIds: string[]; }
export interface Face { id: string; label: string; icon: string; categories: Category[]; }

// Locked mapping (eval-model spec §2). Every D1..D10 appears exactly once.
export const COGNITIVE_MODEL: Face[] = [
  {
    id: "driving", label: "生成式驾驭", icon: "🚀",
    categories: [
      { id: "intent", label: "意图与编排", dimIds: ["D1", "D10"] },
      { id: "reasoning", label: "推理与论证", dimIds: ["D4", "D5", "D7"] },
    ],
  },
  {
    id: "guarding", label: "批判式防护", icon: "🛡️",
    categories: [
      { id: "literacy", label: "信息素养", dimIds: ["D2", "D3"] },
      { id: "metacognition", label: "AI 元认知与边界", dimIds: ["D6", "D8", "D9"] },
    ],
  },
];

export interface AssembledDim { dimId: string; name: string; level: SoloLevel; note: string; }
export interface AssembledCategory { id: string; label: string; dims: AssembledDim[]; scored: number; na: number; }
export interface AssembledFace { id: string; label: string; icon: string; categories: AssembledCategory[]; scored: number; na: number; }
export interface AssembledImprint { faces: AssembledFace[]; }

const NAME_BY_ID = new Map(FULL_RUBRIC.map((d) => [d.id, d.name]));

export function assembleImprint(evaluation: Evaluation): AssembledImprint {
  const scoreById = new Map(evaluation.scores.map((s) => [s.dim_id, s]));

  const faces: AssembledFace[] = COGNITIVE_MODEL.map((face) => {
    const categories: AssembledCategory[] = face.categories.map((cat) => {
      const dims: AssembledDim[] = cat.dimIds.map((dimId) => {
        const score = scoreById.get(dimId);
        return {
          dimId,
          name: NAME_BY_ID.get(dimId) ?? dimId,
          level: score ? score.level : "NA",   // absent dim → NA
          note: score ? score.note : "",
        };
      });
      const scored = dims.filter((d) => d.level !== "NA").length;
      return { id: cat.id, label: cat.label, dims, scored, na: dims.length - scored };
    });
    const scored = categories.reduce((sum, c) => sum + c.scored, 0);
    const na = categories.reduce((sum, c) => sum + c.na, 0);
    return { id: face.id, label: face.label, icon: face.icon, categories, scored, na };
  });

  return { faces };
}
```

- [ ] **Step 4: Export it**

In `packages/contracts/src/index.ts`, add after the `./evaluation` export line:

```ts
export * from "./cognitive-model";
```

- [ ] **Step 5: Run the test — verify it passes**

Run: `pnpm --filter @mind-imprint/contracts exec vitest run src/cognitive-model.test.ts`
Expected: PASS.

- [ ] **Step 6: Typecheck + commit**

Run: `pnpm -r typecheck`
Expected: clean.

```bash
cd /Users/houyuxin/08Coding/mind-imprint
git add packages/contracts/src/cognitive-model.ts packages/contracts/src/cognitive-model.test.ts packages/contracts/src/index.ts
git commit -m "feat(contracts): cognitive-model rollup tree + assembleImprint (reveal UI)

Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>"
```

---

### Task 3: Async polling evaluator

**Files:**
- Modify: `apps/web/src/agent/createEvaluator.ts`
- Test: `apps/web/src/agent/createEvaluator.test.ts` (new)

**Interfaces:**
- Consumes: `api.runEvaluation` (POST→queued/in-flight row), `api.getEvaluation` (latest, or null).
- Produces: unchanged external `Evaluator` shape (`getSnapshot`/`subscribe`/`run`) and `EvalPhase = "idle"|"running"|"done"|"error"` (kept — `running` now spans queued+running so `WorkspaceView` needs no change). `EvaluatorDeps` gains `api: Pick<ApiClient,"runEvaluation"|"getEvaluation">` and optional injectables `pollIntervalMs?: number` (default 2500), `maxAttempts?: number` (default 40), `wait?: (ms: number) => Promise<void>` (default real `setTimeout`).

- [ ] **Step 1: Write the failing test**

Create `apps/web/src/agent/createEvaluator.test.ts`:

```ts
import { describe, it, expect, vi } from "vitest";
import { createEvaluator } from "./createEvaluator";
import type { Evaluation } from "@mind-imprint/contracts";
import type { Store } from "../store/createStore";

function fakeStore(): Store {
  return { putEvaluation: vi.fn() } as unknown as Store;
}
const ev = (status: Evaluation["status"]): Evaluation => ({
  id: "ev1", task_id: "t1", status, scores: [], narrative: "n",
  created_at: "2026-06-29T00:00:00.000Z", completed_at: null,
});

describe("createEvaluator (async polling)", () => {
  const noWait = async () => {};

  it("polls queued → running → done and resolves to phase 'done'", async () => {
    const getEvaluation = vi.fn()
      .mockResolvedValueOnce(ev("running"))
      .mockResolvedValueOnce(ev("done"));
    const api = { runEvaluation: vi.fn().mockResolvedValue(ev("queued")), getEvaluation };
    const evaluator = createEvaluator({ api, store: fakeStore(), taskId: "t1", wait: noWait });
    await evaluator.run();
    expect(evaluator.getSnapshot().phase).toBe("done");
    expect(evaluator.getSnapshot().evaluation?.status).toBe("done");
  });

  it("resolves to 'error' when the eval status becomes 'failed'", async () => {
    const api = {
      runEvaluation: vi.fn().mockResolvedValue(ev("queued")),
      getEvaluation: vi.fn().mockResolvedValue(ev("failed")),
    };
    const evaluator = createEvaluator({ api, store: fakeStore(), taskId: "t1", wait: noWait });
    await evaluator.run();
    expect(evaluator.getSnapshot().phase).toBe("error");
  });

  it("times out to 'error' if it never reaches a terminal status", async () => {
    const api = {
      runEvaluation: vi.fn().mockResolvedValue(ev("queued")),
      getEvaluation: vi.fn().mockResolvedValue(ev("running")),
    };
    const evaluator = createEvaluator({ api, store: fakeStore(), taskId: "t1", wait: noWait, maxAttempts: 3 });
    await evaluator.run();
    expect(evaluator.getSnapshot().phase).toBe("error");
  });
});
```

- [ ] **Step 2: Run it to verify it fails**

Run: `pnpm --filter web exec vitest run src/agent/createEvaluator.test.ts`
Expected: FAIL (current evaluator treats the POST as final; no polling; `getEvaluation` not in deps).

- [ ] **Step 3: Rewrite `createEvaluator.ts`**

Replace `apps/web/src/agent/createEvaluator.ts` with:

```ts
import type { Evaluation } from "@mind-imprint/contracts";
import type { ApiClient } from "../api";
import type { Store } from "../store/createStore";

export type EvalPhase = "idle" | "running" | "done" | "error";
export interface EvalState { phase: EvalPhase; evaluation?: Evaluation; error?: string }

export interface EvaluatorDeps {
  api: Pick<ApiClient, "runEvaluation" | "getEvaluation">;
  store: Store;
  taskId: string;
  pollIntervalMs?: number;
  maxAttempts?: number;
  wait?: (ms: number) => Promise<void>;
}

export interface Evaluator {
  getSnapshot(): EvalState;
  subscribe(listener: () => void): () => void;
  run(): Promise<void>;
}

const defaultWait = (ms: number) => new Promise<void>((r) => setTimeout(r, ms));

export function createEvaluator(deps: EvaluatorDeps): Evaluator {
  const { api, store, taskId } = deps;
  const pollIntervalMs = deps.pollIntervalMs ?? 2500;
  const maxAttempts = deps.maxAttempts ?? 40;
  const wait = deps.wait ?? defaultWait;

  let state: EvalState = { phase: "idle" };
  const listeners = new Set<() => void>();
  function setState(next: Partial<EvalState>): void {
    const merged = { ...state, ...next };
    if ((Object.keys(merged) as (keyof EvalState)[]).some((k) => merged[k] !== state[k])) {
      state = merged;
      listeners.forEach((l) => l());
    }
  }

  return {
    getSnapshot: () => state,
    subscribe(listener) { listeners.add(listener); return () => { listeners.delete(listener); }; },
    async run() {
      setState({ phase: "running", error: undefined });
      try {
        // POST enqueues (202) and returns a queued/in-flight row; then poll until terminal.
        let evaluation = await api.runEvaluation(taskId);
        for (let attempt = 0; attempt < maxAttempts; attempt++) {
          if (evaluation.status === "done") {
            store.putEvaluation(evaluation);
            setState({ phase: "done", evaluation });
            return;
          }
          if (evaluation.status === "failed") {
            setState({ phase: "error", error: "评估失败，请重试" });
            return;
          }
          await wait(pollIntervalMs);
          const latest = await api.getEvaluation(taskId);
          if (latest) evaluation = latest;
        }
        setState({ phase: "error", error: "评估超时，请重试" });
      } catch (e) {
        setState({ phase: "error", error: e instanceof Error ? e.message : String(e) });
      }
    },
  };
}
```

- [ ] **Step 4: Run the test — verify it passes**

Run: `pnpm --filter web exec vitest run src/agent/createEvaluator.test.ts`
Expected: PASS (all 3).

- [ ] **Step 5: Typecheck + web suite + commit**

Run: `pnpm -r typecheck && pnpm --filter web test`
Expected: clean; all web tests pass (`WorkspaceContainer` still constructs `createEvaluator({api, store, taskId})` — the new params are optional).

```bash
cd /Users/houyuxin/08Coding/mind-imprint
git add apps/web/src/agent/createEvaluator.ts apps/web/src/agent/createEvaluator.test.ts
git commit -m "feat(web): async polling evaluator (queued→done|failed|timeout) (reveal UI)

Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>"
```

---

### Task 4: Store — `lastSeenEvaluationAt` marker

**Files:**
- Modify: `apps/web/src/store/schema.ts`
- Modify: `apps/web/src/store/createStore.ts`
- Test: `apps/web/src/store/createStore.test.ts` (extend; if absent, create)

**Interfaces:**
- Produces on `Store`: `getLastSeenEvaluationAt(taskId: string): string | undefined`; `setLastSeenEvaluationAt(taskId: string, iso: string): void`.

- [ ] **Step 1: Write the failing test**

Append to `apps/web/src/store/createStore.test.ts` (create the file with this content if it does not exist):

```ts
import { describe, it, expect } from "vitest";
import { createStore } from "./createStore";

describe("lastSeenEvaluationAt", () => {
  it("is undefined until set, then returns the stored iso per task", () => {
    const store = createStore({});
    expect(store.getLastSeenEvaluationAt("t1")).toBeUndefined();
    store.setLastSeenEvaluationAt("t1", "2026-06-29T00:00:05.000Z");
    expect(store.getLastSeenEvaluationAt("t1")).toBe("2026-06-29T00:00:05.000Z");
    expect(store.getLastSeenEvaluationAt("t2")).toBeUndefined();
  });
});
```

- [ ] **Step 2: Run it to verify it fails**

Run: `pnpm --filter web exec vitest run src/store/createStore.test.ts`
Expected: FAIL (methods undefined).

- [ ] **Step 3: Extend the schema**

In `apps/web/src/store/schema.ts`, add the field to `StoreState` and `EMPTY_STATE`:

```ts
export const StoreState = z.object({
  version: z.literal(1),
  tasks: z.array(Task),
  messages: z.array(Message),
  cards: z.array(CardInstance),
  evaluations: z.array(Evaluation).default([]),
  lastSeenEvaluationAt: z.record(z.string()).default({}),
});

export type StoreState = z.infer<typeof StoreState>;

export const EMPTY_STATE: StoreState = { version: 1, tasks: [], messages: [], cards: [], evaluations: [], lastSeenEvaluationAt: {} };
```

- [ ] **Step 4: Add the two methods**

In `apps/web/src/store/createStore.ts`, add to the `Store` interface (after `getLatestEvaluation`):

```ts
  getLastSeenEvaluationAt(task_id: string): string | undefined;
  setLastSeenEvaluationAt(task_id: string, iso: string): void;
```

And to the returned object (after `getLatestEvaluation`):

```ts
    getLastSeenEvaluationAt: (task_id) => state.lastSeenEvaluationAt[task_id],
    setLastSeenEvaluationAt(task_id, iso) {
      commit({ ...state, lastSeenEvaluationAt: { ...state.lastSeenEvaluationAt, [task_id]: iso } });
    },
```

- [ ] **Step 5: Run the test — verify it passes**

Run: `pnpm --filter web exec vitest run src/store/createStore.test.ts`
Expected: PASS.

- [ ] **Step 6: Typecheck + web suite + commit**

Run: `pnpm -r typecheck && pnpm --filter web test`
Expected: clean; all pass (existing `EMPTY_STATE`/hydrate paths unaffected; `.default({})` keeps older stored blobs valid).

```bash
cd /Users/houyuxin/08Coding/mind-imprint
git add apps/web/src/store/schema.ts apps/web/src/store/createStore.ts apps/web/src/store/createStore.test.ts
git commit -m "feat(web): store lastSeenEvaluationAt marker (reveal UI)

Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>"
```

---

### Task 5: Hierarchical reveal component

**Files:**
- Modify: `apps/web/src/workspace/EvalModal.tsx` (rebuild as hierarchical drill-down)
- Create: `apps/web/src/workspace/imprintReveal.tsx` (FaceSection, CategorySection, DimRow, CoverageChip)
- Delete: `apps/web/src/workspace/evalView.ts`, `apps/web/src/workspace/evalView.test.ts`
- Test: `apps/web/src/workspace/EvalModal.test.tsx` (new)

**Interfaces:**
- Consumes: `assembleImprint`, `AssembledFace/Category/Dim` (Task 2), `SOLO_LABELS`, `ScoredLevel` (Task 1).
- Produces: `EvalModal({ evaluation: Evaluation; onClose: () => void })` — unchanged signature (drop-in for `WorkspaceView`), now rendering the hierarchical tree.

- [ ] **Step 1: Write the failing component test**

Create `apps/web/src/workspace/EvalModal.test.tsx`:

```tsx
import { describe, it, expect } from "vitest";
import { render, screen, fireEvent, within } from "@testing-library/react";
import { EvalModal } from "./EvalModal";
import type { Evaluation } from "@mind-imprint/contracts";

const evaluation: Evaluation = {
  id: "ev1", task_id: "t1", status: "done", completed_at: null,
  created_at: "2026-06-29T00:00:00.000Z",
  narrative: "你这一程的思维印记叙述。",
  scores: [
    { dim_id: "D1", level: "L3", note: "开场即含背景+目标+约束" },
    { dim_id: "D4", level: "NA", note: "" },
  ],
};

describe("EvalModal (hierarchical reveal)", () => {
  it("renders the two faces with factual coverage chips, no aggregate level word", () => {
    render(<EvalModal evaluation={evaluation} onClose={() => {}} />);
    expect(screen.getByText("生成式驾驭")).toBeTruthy();
    expect(screen.getByText("批判式防护")).toBeTruthy();
    // driving face: D1 L3 scored; D4/D5/D7/D10 → among them D1 scored, D4 NA, others absent→NA
    // coverage chip is factual count text containing 已评 / 未涉及, never a SOLO word like 熟练 on the face row.
    expect(screen.getAllByText(/项已评/).length).toBeGreaterThan(0);
  });

  it("expands a category to reveal a scored dim (pill + note) and renders N/A neutrally", () => {
    render(<EvalModal evaluation={evaluation} onClose={() => {}} />);
    // Expand 意图与编排 (holds D1)
    fireEvent.click(screen.getByText("意图与编排"));
    expect(screen.getByText("提问清晰度")).toBeTruthy();
    expect(screen.getByText(/L3.*熟练/)).toBeTruthy();
    expect(screen.getByText("开场即含背景+目标+约束")).toBeTruthy();
    // Expand 推理与论证 (holds D4 NA + D5/D7 absent→NA) and assert neutral text
    fireEvent.click(screen.getByText("推理与论证"));
    expect(screen.getAllByText("本次未涉及").length).toBeGreaterThan(0);
  });

  it("renders the narrative and calls onClose from 回到任务", () => {
    let closed = false;
    render(<EvalModal evaluation={evaluation} onClose={() => { closed = true; }} />);
    expect(screen.getByText("你这一程的思维印记叙述。")).toBeTruthy();
    fireEvent.click(screen.getByText("回到任务"));
    expect(closed).toBe(true);
  });
});
```

- [ ] **Step 2: Run it to verify it fails**

Run: `pnpm --filter web exec vitest run src/workspace/EvalModal.test.tsx`
Expected: FAIL (current modal renders a flat list; no faces/categories).

- [ ] **Step 3: Implement the reveal sub-components**

Create `apps/web/src/workspace/imprintReveal.tsx`:

```tsx
import { useState } from "react";
import type { AssembledFace, AssembledCategory, AssembledDim, ScoredLevel } from "@mind-imprint/contracts";
import { SOLO_LABELS } from "@mind-imprint/contracts";

export function CoverageChip({ scored, na }: { scored: number; na: number }) {
  const text = na > 0 ? `${scored} 项已评 · ${na} 未涉及` : `${scored} 项已评`;
  return (
    <span style={{ flex: "none", fontSize: "12px", fontWeight: 600, color: "#8A92A3" }}>{text}</span>
  );
}

export function DimRow({ dim, last }: { dim: AssembledDim; last: boolean }) {
  const isNA = dim.level === "NA";
  return (
    <div style={{ padding: "11px 0", borderBottom: last ? "none" : "1px solid #F2F3F7" }}>
      <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between", gap: "12px", marginBottom: isNA ? 0 : "8px" }}>
        <span style={{ fontSize: "14px", fontWeight: 700, color: isNA ? "#A7AEBC" : "#1C2333" }}>{dim.name}</span>
        {isNA ? (
          <span style={{ fontSize: "12px", fontWeight: 600, color: "#A7AEBC", background: "#F2F3F7", padding: "2px 10px", borderRadius: "999px", flexShrink: 0 }}>本次未涉及</span>
        ) : (
          <span style={{ fontSize: "12.5px", fontWeight: 700, color: "#D98263", background: "#FBEEE7", padding: "2px 10px", borderRadius: "999px", flexShrink: 0 }}>
            {dim.level} · {SOLO_LABELS[dim.level as ScoredLevel]}
          </span>
        )}
      </div>
      {!isNA && (
        <>
          <div style={{ display: "flex", gap: "5px", marginBottom: "7px" }}>
            {[1, 2, 3, 4].map((i) => (
              <span key={i} aria-hidden="true" style={{ flex: "1", height: "6px", borderRadius: "3px", background: i <= LEVEL_NUMBER[dim.level as ScoredLevel] ? "#D98263" : "#ECEEF4" }} />
            ))}
          </div>
          <div style={{ fontSize: "12.5px", color: "#8A92A3" }}>{dim.note}</div>
        </>
      )}
    </div>
  );
}

const LEVEL_NUMBER: Record<ScoredLevel, number> = { L1: 1, L2: 2, L3: 3, L4: 4 };

export function CategorySection({ category }: { category: AssembledCategory }) {
  const [open, setOpen] = useState(false);
  return (
    <div style={{ marginTop: "10px" }}>
      <button type="button" onClick={() => setOpen((v) => !v)}
        style={{ width: "100%", display: "flex", alignItems: "center", justifyContent: "space-between", gap: "12px", background: "none", border: "none", padding: "8px 0", cursor: "pointer", fontFamily: "inherit" }}>
        <span style={{ fontSize: "13.5px", fontWeight: 700, color: "#2A3B7A" }}>{category.label}</span>
        <CoverageChip scored={category.scored} na={category.na} />
      </button>
      {open && (
        <div style={{ paddingLeft: "12px", borderLeft: "2px solid #EDEFF6" }}>
          {category.dims.map((d, i) => <DimRow key={d.dimId} dim={d} last={i === category.dims.length - 1} />)}
        </div>
      )}
    </div>
  );
}

export function FaceSection({ face }: { face: AssembledFace }) {
  // Faces start expanded to show their categories + coverage (the headline structure).
  return (
    <div style={{ padding: "14px 0", borderBottom: "1px solid #EEF0F4" }}>
      <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between", gap: "12px" }}>
        <span style={{ fontSize: "16px", fontWeight: 800, color: "#1C2333" }}>{face.icon} {face.label}</span>
        <CoverageChip scored={face.scored} na={face.na} />
      </div>
      {face.categories.map((c) => <CategorySection key={c.id} category={c} />)}
    </div>
  );
}
```

- [ ] **Step 4: Rebuild `EvalModal.tsx`**

In `apps/web/src/workspace/EvalModal.tsx`, replace the imports and the body. Keep the outer scrim/card/header/narrative/button markup verbatim (the `.dc.html` styling); replace ONLY the dim-list block (`{dims.map(...)}`) with the face sections. Concretely:

- Replace the top imports:
```tsx
import type { Evaluation } from "@mind-imprint/contracts";
import { assembleImprint } from "@mind-imprint/contracts";
import { FaceSection } from "./imprintReveal";
```
- Replace `const { dims } = evalView(evaluation, FULL_RUBRIC);` with:
```tsx
  const imprint = assembleImprint(evaluation);
  const allNA = imprint.faces.every((f) => f.scored === 0);
```
- Replace the entire `{/* Dim rows */}` block (the `{dims.map((d, i) => ( ... ))}`) with:
```tsx
          {/* All-N/A short task: in-progress framing, not punished */}
          {allNA && (
            <div style={{ fontSize: "13px", color: "#8A92A3", padding: "4px 0 10px" }}>
              进行中 · 这一程暂未产生可评估的过程证据，继续推进任务后再来看你的思维印记。
            </div>
          )}
          {/* Faces → categories → dims */}
          {imprint.faces.map((face) => <FaceSection key={face.id} face={face} />)}
```
(The header, `过程叙述` narrative card reading `evaluation.narrative`, and `回到任务` button stay exactly as they are.)

- [ ] **Step 5: Delete the superseded flat view**

```bash
cd /Users/houyuxin/08Coding/mind-imprint
git rm apps/web/src/workspace/evalView.ts apps/web/src/workspace/evalView.test.ts
```
(`EvalModal` no longer imports `evalView`; confirm nothing else does: `grep -rn "evalView" apps/web/src` should return nothing after this.)

- [ ] **Step 6: Run the component test + typecheck**

Run: `pnpm --filter web exec vitest run src/workspace/EvalModal.test.tsx && pnpm -r typecheck`
Expected: PASS; typecheck clean (the `evalView` deletion leaves no dangling import).

- [ ] **Step 7: Full web suite + commit**

Run: `pnpm --filter web test`
Expected: all pass.

```bash
cd /Users/houyuxin/08Coding/mind-imprint
git add apps/web/src/workspace/EvalModal.tsx apps/web/src/workspace/imprintReveal.tsx apps/web/src/workspace/EvalModal.test.tsx
git commit -m "feat(web): hierarchical 思维印记 reveal (faces→categories→dims) (reveal UI)

Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>"
```

---

### Task 6: Quiet indicator + background poll + wiring

**Files:**
- Create: `apps/web/src/workspace/MindImprintIndicator.tsx`
- Modify: `apps/web/src/workspace/WorkspaceView.tsx` (mount indicator; modal renders store-latest done eval; lastSeen on open)
- Modify: `apps/web/src/shell/WorkspaceContainer.tsx` (30 s background poll)
- Test: `apps/web/src/workspace/MindImprintIndicator.test.tsx` (new)

**Interfaces:**
- Consumes: `store.getLatestEvaluation`, `store.getLastSeenEvaluationAt`, `store.setLastSeenEvaluationAt` (Task 4); `EvalModal` (Task 5); `api.getEvaluation`.
- Produces: `MindImprintIndicator({ visible: boolean; onOpen: () => void })` — a pure presentational chip.

- [ ] **Step 1: Write the failing indicator test**

Create `apps/web/src/workspace/MindImprintIndicator.test.tsx`:

```tsx
import { describe, it, expect } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import { MindImprintIndicator } from "./MindImprintIndicator";

describe("MindImprintIndicator", () => {
  it("renders nothing when not visible", () => {
    const { container } = render(<MindImprintIndicator visible={false} onOpen={() => {}} />);
    expect(container.firstChild).toBeNull();
  });

  it("shows the calm chip and fires onOpen when clicked", () => {
    let opened = false;
    render(<MindImprintIndicator visible onOpen={() => { opened = true; }} />);
    const chip = screen.getByText(/你的思维印记有新内容/);
    expect(chip).toBeTruthy();
    fireEvent.click(chip);
    expect(opened).toBe(true);
  });
});
```

- [ ] **Step 2: Run it to verify it fails**

Run: `pnpm --filter web exec vitest run src/workspace/MindImprintIndicator.test.tsx`
Expected: FAIL (component missing).

- [ ] **Step 3: Implement the indicator**

Create `apps/web/src/workspace/MindImprintIndicator.tsx`:

```tsx
type Props = { visible: boolean; onOpen: () => void };

// "Offer, don't push" — a calm, dismissable-by-opening chip. No badge count, no auto-open, no streak.
export function MindImprintIndicator({ visible, onOpen }: Props) {
  if (!visible) return null;
  return (
    <button type="button" onClick={onOpen}
      style={{
        position: "absolute", bottom: "24px", right: "24px", zIndex: 40,
        display: "flex", alignItems: "center", gap: "8px",
        background: "#2A3B7A", color: "#fff", border: "none",
        padding: "11px 16px", borderRadius: "999px", cursor: "pointer",
        fontFamily: "inherit", fontSize: "13.5px", fontWeight: 700,
        boxShadow: "0 6px 20px rgba(42,59,122,.28)", animation: "mkPop .3s cubic-bezier(.22,.9,.3,1)",
      }}>
      <span aria-hidden="true">✨</span> 你的思维印记有新内容
    </button>
  );
}
```

- [ ] **Step 4: Run the indicator test — verify it passes**

Run: `pnpm --filter web exec vitest run src/workspace/MindImprintIndicator.test.tsx`
Expected: PASS.

- [ ] **Step 5: Wire into `WorkspaceView.tsx`**

In `apps/web/src/workspace/WorkspaceView.tsx`:

- Add the import:
```tsx
import { MindImprintIndicator } from "./MindImprintIndicator";
```
- Compute the latest done eval + indicator visibility (after the existing `const task = store.getTask(taskId);`):
```tsx
  const latestEval = store.getLatestEvaluation(taskId);
  const latestDone = latestEval && latestEval.status === "done" ? latestEval : undefined;
  const lastSeen = store.getLastSeenEvaluationAt(taskId);
  const hasUnseen = !!latestDone && (!lastSeen || latestDone.created_at > lastSeen);
  const indicatorVisible = hasUnseen && !showEvalModal && evalState.phase !== "running";

  function openReveal() {
    if (latestDone) store.setLastSeenEvaluationAt(taskId, latestDone.created_at);
    setShowEvalModal(true);
  }
```
- Mount the indicator inside the main content area `<div style={{ flex: 1, ... position: "relative" }}>`, e.g. just before the eval-loading overlay:
```tsx
        <MindImprintIndicator visible={indicatorVisible} onOpen={openReveal} />
```
- Change the modal render block (currently keyed on `evalState`) to render the store's latest done eval, opened by either path, suppressed during an active run (so a re-evaluate shows `EvalLoading`, not the stale prior eval), and mark it seen on close:
```tsx
        {showEvalModal && latestDone && evalState.phase !== "running" && (
          <EvalModal
            evaluation={latestDone}
            onClose={() => {
              store.setLastSeenEvaluationAt(taskId, latestDone.created_at);
              setShowEvalModal(false);
            }}
          />
        )}
```
- The manual button's `onClick` already calls `setShowEvalModal(true); void evaluator.run();` — leave it. During the run `evalState.phase === "running"` suppresses the modal and shows the existing `EvalLoading` overlay; when `run()` completes it `putEvaluation`s into the store and flips to `done`, so `latestDone` updates and the modal shows the fresh eval. (The error card is unchanged.)

- [ ] **Step 6: Add the background poll in `WorkspaceContainer.tsx`**

In `apps/web/src/shell/WorkspaceContainer.tsx`, add a second `useEffect` after the hydration effect that polls every 30 s and stores a genuinely-newer done eval (gating prevents duplicate appends):

```tsx
  useEffect(() => {
    const POLL_MS = 30_000;
    let cancelled = false;
    const id = setInterval(() => {
      void (async () => {
        try {
          const latest = await api.getEvaluation(taskId);
          if (cancelled || !latest || latest.status !== "done") return;
          const known = store.getLatestEvaluation(taskId);
          if (!known || latest.created_at > known.created_at) store.putEvaluation(latest);
        } catch {
          // best-effort: a failed poll is silently ignored
        }
      })();
    }, POLL_MS);
    return () => { cancelled = true; clearInterval(id); };
  }, [taskId, store]);
```

- [ ] **Step 7: Typecheck + full web suite**

Run: `pnpm -r typecheck && pnpm --filter web test`
Expected: clean; all pass. If a `WorkspaceView` test asserted the old `evalState`-keyed modal, update it to seed a done eval into the store (`store.putEvaluation(...)`) and open via the button — the modal now reads `store.getLatestEvaluation`.

- [ ] **Step 8: Full monorepo gate + commit**

Run: `pnpm -r typecheck && pnpm -r test`
Expected: all packages green.

```bash
cd /Users/houyuxin/08Coding/mind-imprint
git add apps/web/src/workspace/MindImprintIndicator.tsx apps/web/src/workspace/MindImprintIndicator.test.tsx \
        apps/web/src/workspace/WorkspaceView.tsx apps/web/src/shell/WorkspaceContainer.tsx
git commit -m "feat(web): quiet 思维印记 indicator + 30s background poll (reveal UI)

Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>"
```

---

## Self-Review Notes (for the executor)

- **Spec coverage:** §2 contracts → T1+T2. §3 reveal → T5. §4 evaluator → T3. §5 indicator → T4+T6. §7 testing → each task's tests. The design-source split (HTML styling + spec structure) is realized by reusing the exact `.dc.html`-derived markup in `EvalModal`/`imprintReveal` while the tree is hierarchical.
- **Green build between tasks:** T1 keeps the monorepo compiling by fixing `evalView`/`ability`/fixtures alongside the contract change. T2 is additive. T3/T4 are isolated. T5 replaces the modal internals (drop-in signature) and deletes the now-dead `evalView`. T6 rewires the modal to read store-latest + adds the indicator/poll. Each task ends green.
- **Type consistency:** `SoloLevel` (incl NA) vs `ScoredLevel` (L1–L4) used consistently; `assembleImprint` returns `level: SoloLevel` with absent→`"NA"`; `DimRow` narrows via `level === "NA"` before `SOLO_LABELS[level as ScoredLevel]`; `EvalModal({evaluation,onClose})` signature unchanged across T5.
- **铁律 #2:** faces/categories render `CoverageChip` (factual counts) only — no SOLO word on face/category rows; the indicator has no count/auto-open/streak.
- **Known minor (defer):** the 30 s `setInterval` poll isn't unit-tested in T6 (the indicator visibility logic is tested via the pure component + the store/evaluator units); an integration test that drives the interval is a fast-follow. Note it in the report rather than over-building a timer test.
