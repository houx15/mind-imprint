# Slice 6 — App Shell (B7) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the dev `DevApp` with the real product shell — mock auth, left-rail nav, task directory, directory↔workspace routing, a records page (active calendar + card usage + 9-dim ability radar), and a settings page owning real LLM config behind a non-dismissable first-run key-gate — and fold in the S5 carry-forwards (canonical 9-dim rubric, eval error UX, re-run guard, setState-swap audit).

**Architecture:** A top-level `AppShell` mirrors the design HTML's screen state machine (`screen / tab / taskView / recordTab / activeTaskId`) in React `useState` — no router. Two concerns persist outside the data store: `mk.session` (auth flag + AI-avatar color) and `mk.llmConfig` (gains a `verified` flag). Records views are thin renderers over pure derivation functions of the existing store. The workspace is owned by a `WorkspaceContainer` that instantiates `createConversation`/`createEvaluator` per `activeTaskId`.

**Tech Stack:** pnpm monorepo — `packages/contracts` (Zod) + `apps/web` (React 18 + Vite + TS + Tailwind). Tests: Vitest + @testing-library/react. Binding UI source: `docs/design/思维印记_工作区.dc.html`.

## Global Constraints

- Verification gate per task: `pnpm -r typecheck` (tsc `--noEmit`) **and** `pnpm -r test` must be green. vite/vitest do NOT typecheck — run `pnpm -r typecheck` explicitly.
- **Key-safety (hard):** API key lives ONLY in `mk.llmConfig` and request headers. NEVER in any log, thrown error, rendered output, the data store, or `mk.session`.
- **Reactive controllers:** `useSyncExternalStore` needs a STABLE `getSnapshot` — swap the state ref AND call listeners only when an actual change occurred (the `createEvaluator` fix). Applies to `session` and the `createConversation`/`createStore` audit.
- **Design fidelity:** lift inline styles verbatim from the binding HTML at the cited line ranges, converting `style="..."` → JSX style objects; real Phoebe content, never lorem ipsum. One documented copy deviation: card-usage groups by the **8 real registry categories** and the intro copy reads "课程库分支" (not "五大分支").
- **Restraint:** records is read-only; no streaks/leaderboards/badges/push.
- Existing store API (do not change signatures): `createStore({storage,now?,genId?})` → `createTask({title,seed})`, `getTask`, `listTasks`, `updateTask`, `appendMessage({task_id,role,content,tool_call?})`, `listMessages`, `listCards(task_id)`, `listEvaluations(task_id)`, `getLatestEvaluation(task_id)`, `getSnapshot`, `subscribe`. `makeMemoryStorage()` for tests (from `../store`).
- Controller deps: `createConversation({store,chat,config,registry,catalog,taskId})`, `createEvaluator({store,chat,config,registry,taskId})`. `catalog = demoCatalog(deriveCatalog(CARD_REGISTRY))`. `chat, loadConfig` from `../llm`. `CARD_REGISTRY, deriveCatalog` from `@mind-imprint/contracts`.

---

## File Structure

**Contracts (modify):**
- `packages/contracts/src/rubric.ts` — `FULL_RUBRIC` (9 dims), delete `RUBRIC_TAGS`.

**Agent / workspace (modify):**
- `apps/web/src/agent/runEvaluation.ts`, `agent/evalPrompt.ts` (few-shot 9 scores), `workspace/EvalModal.tsx`, `workspace/evalView` callers — use `FULL_RUBRIC`.
- `apps/web/src/agent/createConversation.ts` — setState-swap audit fix.
- `apps/web/src/workspace/EvalModal.tsx` / `WorkspaceView.tsx` — eval error UX + re-run guard.

**New shell (create) — all under `apps/web/src/shell/`:**
- `session.ts` — `mk.session` reactive holder + `useSession`.
- `auth/AuthScreen.tsx` — login/register/bind.
- `LeftRail.tsx` — nav rail.
- `directory/taskCardView.ts` — pure Task→card view mapping.
- `directory/DirectoryView.tsx` — home + new-task + grid.
- `records/activityCalendar.ts`, `records/growthReviews.ts`, `records/cardUsage.ts`, `records/ability.ts`, `records/radarGeometry.ts` — pure derivations.
- `records/RecordsView.tsx` — 3-tab records page.
- `settings/LlmConfigForm.tsx` — shared LLM config form + 测试连接.
- `settings/SettingsView.tsx` — settings page.
- `KeyGateModal.tsx` — non-dismissable first-run gate.
- `WorkspaceContainer.tsx` — per-task controller lifecycle.
- `AppShell.tsx` — top-level state machine + nav.

**Config (modify):** `apps/web/src/llm/config.ts` (zod parse + `verified`). `apps/web/src/main.tsx` (mount `AppShell`).

---

## Task 1: Canonical 9-dim rubric (FULL_RUBRIC) + 9-score few-shot

**Files:**
- Modify: `packages/contracts/src/rubric.ts`
- Modify: `apps/web/src/agent/runEvaluation.ts`, `apps/web/src/agent/evalPrompt.ts`, `apps/web/src/workspace/EvalModal.tsx`
- Modify tests: `packages/contracts/src/rubric.test.ts` (if present; else add), `apps/web/src/agent/evalPrompt.test.ts`, `apps/web/src/agent/runEvaluation.test.ts`, `apps/web/src/workspace/evalView.test.ts`, `apps/web/src/workspace/EvalModal.test.tsx`, `apps/web/src/workspace/WorkspaceView.test.tsx`

**Interfaces:**
- Produces: `FULL_RUBRIC: RubricDimension[]` (9 entries, ids `D1`–`D9`), exported from `@mind-imprint/contracts`. `RUBRIC_TAGS`/`RubricTag` removed. `SoloLevel`, `SOLO_LABELS`, `RubricDimension` unchanged.
- Consumes: existing `RubricDimension` interface `{ id; name; framework; anchors: {L1,L2,L3,L4} }`.

This is one atomic task because `DEMO_RUBRIC` is imported by 7 files; the rename + 9-dim expansion must land together to keep `pnpm -r typecheck` green.

- [ ] **Step 1: Replace the rubric in `packages/contracts/src/rubric.ts`**

Delete `RUBRIC_TAGS` and `RubricTag` (no importers — verified). Rename `DEMO_RUBRIC` → `FULL_RUBRIC` and append D1, D7, D8, D9. Keep D2–D6 anchors verbatim. Full file body after the unchanged `SoloLevel`/`SOLO_LABELS`/`RubricDimension` block:

```ts
export const FULL_RUBRIC: RubricDimension[] = [
  { id: "D1", name: "提问清晰度", framework: "ATL 思维 · QUEST-Q（输入）",
    anchors: { L1: "直接抛一句话问题，不给 AI 任何背景或目标", L2: "给一点背景，但目标/约束模糊，常需 AI 反问澄清", L3: "主动提供任务背景、目标与约束，问题具体可执行", L4: "结构化拆解需求，分步追问并根据回答迭代提问" } },
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
  { id: "D7", name: "论证质量", framework: "QUEST-S · ATL 沟通（输出）",
    anchors: { L1: "只堆观点 / 复制 AI 原话，无论点-论据结构", L2: "有结论但论据零散，结构不完整", L3: "论点-论据-解释结构完整，引用有出处", L4: "结构严谨且回应反方，论证链条经得起追问" } },
  { id: "D8", name: "信息再生产", framework: "ATL 媒介伦理 · 学术诚信（输出）",
    anchors: { L1: "整段照搬 AI 输出，不标注、不改写", L2: "偶尔改写，但分不清哪些是 AI、哪些是自己的", L3: "明确区分 AI 贡献与个人加工，主动声明 AI 使用", L4: "在 AI 基础上有独立判断与增量，诚信声明清晰可核" } },
  { id: "D9", name: "AI 边界与伦理", framework: "TOK 知识与技术 · 伦理使用（输出）",
    anchors: { L1: "把 AI 当全知，不质疑其可能出错或编造", L2: "知道 AI 会错，但不主动核查", L3: "主动核查 AI 可能幻觉处，识别其知识边界", L4: "系统性评估 AI 局限与伦理风险，按场景决定是否/如何用" } },
];
```

- [ ] **Step 2: Expand the Phoebe few-shot in `agent/evalPrompt.ts` to 9 scores**

In the `phoebeExample` JSON `scores` array, after the existing D6 entry add these four (keep the existing D2–D6 entries unchanged):

```json
,
    { "dim_id": "D1", "level": "L3", "note": "Phoebe 提供了任务背景（用公众号文写中国可持续）与明确目标，问题具体可执行；但未结构化分步追问，停在 L3" },
    { "dim_id": "D7", "level": "L3", "note": "让步段产出论点-论据-解释结构完整，引用 NASA 与 Nature Sustainability 有出处并回应反方；论证链条尚未到严丝合缝，维持 L3" },
    { "dim_id": "D8", "level": "L2", "note": "放弃公众号改引一手来源体现了一定加工，但对话中未见明确区分 AI 贡献与个人贡献的声明，停在 L2" },
    { "dim_id": "D9", "level": "L2", "note": "识别公众号不可信属信源层面；对 AI 本身局限/幻觉的主动核查在本次对话中较少，维持 L2" }
```

Update the example `narrative` is optional but keep it consistent (it already reads well). No code-logic change — `buildEvalPrompt` already renders `rubric` from its argument.

- [ ] **Step 3: Point all `DEMO_RUBRIC` importers at `FULL_RUBRIC`**

`runEvaluation.ts`: change `import { ..., DEMO_RUBRIC }` → `FULL_RUBRIC` and `buildEvalPrompt(DEMO_RUBRIC)` → `buildEvalPrompt(FULL_RUBRIC)`. `EvalModal.tsx`: change `import { DEMO_RUBRIC }` → `FULL_RUBRIC` and `evalView(evaluation, DEMO_RUBRIC)` → `evalView(evaluation, FULL_RUBRIC)`.

- [ ] **Step 4: Update tests to the 9-dim shape**

In `evalPrompt.test.ts`, `runEvaluation.test.ts`, `evalView.test.ts`, `EvalModal.test.tsx`, `WorkspaceView.test.tsx`: replace `DEMO_RUBRIC` imports/usages with `FULL_RUBRIC`. Update any count assertion (e.g. `EvalModal.test.tsx` "renders all 5 dim names" → 9; assertions that built a 5-element `scores` fixture should either pass a 9-element fixture or assert on the dims actually provided — `evalView` maps over `evaluation.scores`, so a fixture with N scores yields N dims). Keep `evalView`/`EvalModal` tolerant: a fixture may include only a subset of dims and still render.

Add a contracts test `packages/contracts/src/rubric.test.ts`:

```ts
import { describe, it, expect } from "vitest";
import { FULL_RUBRIC, SOLO_LABELS } from "./rubric";

describe("FULL_RUBRIC", () => {
  it("has 9 dimensions D1..D9 in order", () => {
    expect(FULL_RUBRIC.map((d) => d.id)).toEqual(["D1","D2","D3","D4","D5","D6","D7","D8","D9"]);
  });
  it("every dim has a name, framework, and all 4 SOLO anchors", () => {
    for (const d of FULL_RUBRIC) {
      expect(d.name.length).toBeGreaterThan(0);
      expect(d.framework.length).toBeGreaterThan(0);
      for (const lvl of ["L1","L2","L3","L4"] as const) {
        expect(d.anchors[lvl].length).toBeGreaterThan(0);
      }
    }
  });
  it("SOLO_LABELS unchanged", () => {
    expect(SOLO_LABELS).toEqual({ L1: "萌芽", L2: "发展中", L3: "熟练", L4: "卓越" });
  });
});
```

- [ ] **Step 5: Verify gate + commit**

Run: `pnpm -r typecheck && pnpm -r test`
Expected: PASS (contracts + web green).

```bash
git add -A && git commit -m "feat(contracts,eval): canonical 9-dim FULL_RUBRIC + 9-score few-shot"
```

---

## Task 2: `session.ts` — mk.session reactive holder

**Files:**
- Create: `apps/web/src/shell/session.ts`
- Test: `apps/web/src/shell/session.test.ts`

**Interfaces:**
- Consumes: `RawStorage` and `makeMemoryStorage` from `../store` (re-exported there).
- Produces: `Session = { authed: boolean; aiAvatar: string }`; `SessionStore = { getSnapshot(): Session; subscribe(l): () => void; setAuthed(v: boolean): void; setAvatar(color: string): void }`; `createSession({storage}): SessionStore`; `useSession(s: SessionStore): Session`.

- [ ] **Step 1: Write the failing test**

```ts
import { describe, it, expect } from "vitest";
import { makeMemoryStorage } from "../store";
import { createSession, SESSION_KEY } from "./session";

describe("createSession", () => {
  it("defaults to logged-out with the default avatar", () => {
    const s = createSession({ storage: makeMemoryStorage() });
    expect(s.getSnapshot()).toEqual({ authed: false, aiAvatar: "#2A3B7A" });
  });
  it("persists authed + avatar across instances", () => {
    const storage = makeMemoryStorage();
    const a = createSession({ storage });
    a.setAuthed(true);
    a.setAvatar("#D98263");
    const b = createSession({ storage });
    expect(b.getSnapshot()).toEqual({ authed: true, aiAvatar: "#D98263" });
  });
  it("returns a stable snapshot ref when nothing changes", () => {
    const s = createSession({ storage: makeMemoryStorage() });
    const before = s.getSnapshot();
    s.setAuthed(false); // no-op (already false)
    expect(s.getSnapshot()).toBe(before);
  });
  it("notifies subscribers on change only", () => {
    const s = createSession({ storage: makeMemoryStorage() });
    let n = 0;
    s.subscribe(() => { n++; });
    s.setAuthed(true);   // change
    s.setAuthed(true);   // no-op
    expect(n).toBe(1);
  });
  it("fail-soft on corrupt storage", () => {
    const storage = makeMemoryStorage();
    storage.setItem(SESSION_KEY, "{not json");
    const s = createSession({ storage });
    expect(s.getSnapshot()).toEqual({ authed: false, aiAvatar: "#2A3B7A" });
  });
});
```

- [ ] **Step 2: Run test to verify it fails**

Run: `pnpm --filter @mind-imprint/web test -- session`
Expected: FAIL (module not found).

- [ ] **Step 3: Implement `apps/web/src/shell/session.ts`**

```ts
import { z } from "zod";
import { useSyncExternalStore } from "react";
import type { RawStorage } from "../store";

export const SESSION_KEY = "mk.session";

export const Session = z.object({
  authed: z.boolean().default(false),
  aiAvatar: z.string().default("#2A3B7A"),
});
export type Session = z.infer<typeof Session>;

const DEFAULT: Session = { authed: false, aiAvatar: "#2A3B7A" };

function load(storage: RawStorage): Session {
  const raw = storage.getItem(SESSION_KEY);
  if (raw == null) return DEFAULT;
  try {
    const parsed = Session.safeParse(JSON.parse(raw));
    return parsed.success ? parsed.data : DEFAULT;
  } catch {
    return DEFAULT;
  }
}

export interface SessionStore {
  getSnapshot(): Session;
  subscribe(listener: () => void): () => void;
  setAuthed(v: boolean): void;
  setAvatar(color: string): void;
}

export function createSession(opts: { storage: RawStorage }): SessionStore {
  let state: Session = load(opts.storage);
  const listeners = new Set<() => void>();

  function commit(next: Session): void {
    const changed = next.authed !== state.authed || next.aiAvatar !== state.aiAvatar;
    if (!changed) return;
    state = next;
    opts.storage.setItem(SESSION_KEY, JSON.stringify(state));
    listeners.forEach((l) => l());
  }

  return {
    getSnapshot: () => state,
    subscribe(l) { listeners.add(l); return () => { listeners.delete(l); }; },
    setAuthed(v) { commit({ ...state, authed: v }); },
    setAvatar(color) { commit({ ...state, aiAvatar: color }); },
  };
}

export function useSession(s: SessionStore): Session {
  return useSyncExternalStore(s.subscribe, s.getSnapshot);
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `pnpm --filter @mind-imprint/web test -- session` and `pnpm -r typecheck`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add -A && git commit -m "feat(shell): mk.session reactive holder + useSession"
```

---

## Task 3: `config.ts` hardening — zod parse + `verified` flag

**Files:**
- Modify: `apps/web/src/llm/types.ts` (add `verified?: boolean` to `LlmConfig`)
- Modify: `apps/web/src/llm/config.ts`
- Test: `apps/web/src/llm/config.test.ts` (add cases; create if absent)

**Interfaces:**
- Produces: `LlmConfig` gains optional `verified?: boolean`. `loadConfig` parses stored JSON with a zod schema (fall back to env on failure). New `markVerified(cfg: LlmConfig): void` (saves with `verified: true`). `isVerified(cfg: Partial<LlmConfig>): boolean` (`isConfigured(cfg) && cfg.verified === true`).

- [ ] **Step 1: Write the failing test**

```ts
import { describe, it, expect, beforeEach } from "vitest";
import { loadConfig, saveConfig, isConfigured, isVerified, markVerified } from "./config";

describe("config hardening", () => {
  beforeEach(() => localStorage.clear());

  it("loads a valid stored config", () => {
    saveConfig({ format: "openai", baseUrl: "https://x/v1", model: "m", apiKey: "k" });
    expect(isConfigured(loadConfig({}))).toBe(true);
  });
  it("falls back to env defaults when stored JSON is structurally invalid", () => {
    localStorage.setItem("mk.llmConfig", JSON.stringify({ format: 123 }));
    const cfg = loadConfig({ VITE_LLM_MODEL: "envModel" });
    expect(cfg.model).toBe("envModel");
  });
  it("isVerified requires both configured and verified flag", () => {
    const base = { format: "openai" as const, baseUrl: "b", model: "m", apiKey: "k" };
    expect(isVerified(base)).toBe(false);
    expect(isVerified({ ...base, verified: true })).toBe(true);
  });
  it("markVerified persists verified:true without touching the key in any log", () => {
    const base = { format: "openai" as const, baseUrl: "b", model: "m", apiKey: "k" };
    saveConfig(base);
    markVerified(base);
    expect(loadConfig({}).verified).toBe(true);
  });
});
```

- [ ] **Step 2: Run test, verify FAIL** — `pnpm --filter @mind-imprint/web test -- config` → FAIL (`isVerified`/`markVerified` undefined).

- [ ] **Step 3: Implement**

In `llm/types.ts`, add to `LlmConfig`:
```ts
  verified?: boolean; // set true only after a passing 测试连接; gates the key-gate
```

Rewrite `llm/config.ts`:
```ts
import { z } from "zod";
import type { LlmConfig, LlmFormat } from "./types";

const STORAGE_KEY = "mk.llmConfig";
type EnvSource = Record<string, string | undefined>;

const StoredConfig = z.object({
  format: z.enum(["openai", "anthropic"]).optional(),
  baseUrl: z.string().optional(),
  model: z.string().optional(),
  apiKey: z.string().optional(),
  evalModel: z.string().optional(),
  verified: z.boolean().optional(),
});

export function loadConfig(env: EnvSource = import.meta.env as EnvSource): Partial<LlmConfig> {
  const stored = typeof localStorage !== "undefined" ? localStorage.getItem(STORAGE_KEY) : null;
  if (stored) {
    try {
      const parsed = StoredConfig.safeParse(JSON.parse(stored));
      if (parsed.success) return parsed.data;
    } catch {
      /* corrupt storage — fall through to env defaults */
    }
  }
  const fmt = env.VITE_LLM_FORMAT;
  return {
    format: fmt === "anthropic" || fmt === "openai" ? (fmt as LlmFormat) : undefined,
    baseUrl: env.VITE_LLM_BASE_URL,
    model: env.VITE_LLM_MODEL,
    apiKey: env.VITE_LLM_API_KEY,
    evalModel: env.VITE_LLM_EVAL_MODEL || undefined,
  };
}

export function saveConfig(cfg: LlmConfig): void {
  localStorage.setItem(STORAGE_KEY, JSON.stringify(cfg));
}

export function isConfigured(cfg: Partial<LlmConfig>): cfg is LlmConfig {
  return Boolean(cfg.format && cfg.baseUrl && cfg.model && cfg.apiKey);
}

export function isVerified(cfg: Partial<LlmConfig>): boolean {
  return isConfigured(cfg) && cfg.verified === true;
}

export function markVerified(cfg: LlmConfig): void {
  saveConfig({ ...cfg, verified: true });
}
```

- [ ] **Step 4: Run tests + typecheck** → PASS.

- [ ] **Step 5: Commit**
```bash
git add -A && git commit -m "feat(llm): config zod-parse hardening + verified flag"
```

---

## Task 4: `activityCalendar.ts` — pure activity-grid derivation

**Files:**
- Create: `apps/web/src/shell/records/activityCalendar.ts`
- Test: `apps/web/src/shell/records/activityCalendar.test.ts`

**Interfaces:**
- Produces: `deriveActivityCalendar(events: { created_at: string }[], now: Date): { cells: { style: string }[]; activeDays: number; weeks: number }`. 17 weeks × 7 days = 119 cells, column-major (design grid `grid-auto-flow:column`, 7 rows). Intensity 0–4 by per-day event count → swatch colors.

- [ ] **Step 1: Write the failing test**

```ts
import { describe, it, expect } from "vitest";
import { deriveActivityCalendar } from "./activityCalendar";

const NOW = new Date("2026-06-21T12:00:00.000Z");

describe("deriveActivityCalendar", () => {
  it("returns 119 cells over 17 weeks", () => {
    const r = deriveActivityCalendar([], NOW);
    expect(r.cells).toHaveLength(119);
    expect(r.weeks).toBe(17);
  });
  it("empty input → all lowest swatch, 0 active days", () => {
    const r = deriveActivityCalendar([], NOW);
    expect(r.activeDays).toBe(0);
    expect(r.cells.every((c) => c.style.includes("#EDEFF4"))).toBe(true);
  });
  it("counts a day with events as active and raises its intensity", () => {
    const day = "2026-06-20T09:00:00.000Z";
    const r = deriveActivityCalendar(
      [{ created_at: day }, { created_at: day }, { created_at: day }], NOW,
    );
    expect(r.activeDays).toBe(1);
    const active = r.cells.filter((c) => !c.style.includes("#EDEFF4"));
    expect(active).toHaveLength(1);
    expect(active[0]!.style).toContain("#97A3D2"); // 3 events → level 3 swatch
  });
  it("ignores events outside the 17-week window", () => {
    const old = "2024-01-01T00:00:00.000Z";
    const r = deriveActivityCalendar([{ created_at: old }], NOW);
    expect(r.activeDays).toBe(0);
  });
});
```

- [ ] **Step 2: Run, verify FAIL** — `pnpm --filter @mind-imprint/web test -- activityCalendar` → FAIL.

- [ ] **Step 3: Implement**

```ts
const SWATCHES = ["#EDEFF4", "#C9D0E8", "#97A3D2", "#5C6CB0", "#2A3B7A"];
const WEEKS = 17;
const DAYS = WEEKS * 7; // 119

function dayKey(d: Date): string {
  return d.toISOString().slice(0, 10);
}
function intensity(count: number): number {
  if (count <= 0) return 0;
  if (count === 1) return 1;
  if (count === 2) return 2;
  if (count <= 4) return 3;
  return 4;
}

export interface CalendarCell { style: string; }
export interface ActivityCalendar { cells: CalendarCell[]; activeDays: number; weeks: number; }

export function deriveActivityCalendar(
  events: { created_at: string }[],
  now: Date,
): ActivityCalendar {
  const counts = new Map<string, number>();
  for (const e of events) {
    const k = e.created_at.slice(0, 10);
    counts.set(k, (counts.get(k) ?? 0) + 1);
  }
  const cells: CalendarCell[] = [];
  let activeDays = 0;
  const today = new Date(Date.UTC(now.getUTCFullYear(), now.getUTCMonth(), now.getUTCDate()));
  // Oldest cell first so column-major fill matches the design grid.
  for (let i = DAYS - 1; i >= 0; i--) {
    const d = new Date(today);
    d.setUTCDate(today.getUTCDate() - i);
    const c = counts.get(dayKey(d)) ?? 0;
    if (c > 0) activeDays++;
    const color = SWATCHES[intensity(c)]!;
    cells.push({ style: `width:13px; height:13px; border-radius:3px; background:${color};` });
  }
  return { cells, activeDays, weeks: WEEKS };
}
```

- [ ] **Step 4: Run tests + typecheck** → PASS.
- [ ] **Step 5: Commit** — `git commit -am "feat(shell): activity calendar derivation"`

---

## Task 5: `growthReviews.ts` — eval narratives → review cards

**Files:**
- Create: `apps/web/src/shell/records/growthReviews.ts`
- Test: `apps/web/src/shell/records/growthReviews.test.ts`

**Interfaces:**
- Consumes: `Evaluation` from `@mind-imprint/contracts` (`{ task_id; scores; narrative; created_at }`).
- Produces: `deriveGrowthReviews(evaluations: Evaluation[]): { period: string; text: string }[]`, newest first, `period` = `YYYY 年 M 月 D 日` from `created_at`.

- [ ] **Step 1: Write the failing test**

```ts
import { describe, it, expect } from "vitest";
import { deriveGrowthReviews } from "./growthReviews";
import type { Evaluation } from "@mind-imprint/contracts";

function ev(created_at: string, narrative: string): Evaluation {
  return { task_id: "t1", scores: [], narrative, created_at };
}

describe("deriveGrowthReviews", () => {
  it("empty → []", () => {
    expect(deriveGrowthReviews([])).toEqual([]);
  });
  it("orders newest first and formats the period", () => {
    const out = deriveGrowthReviews([
      ev("2026-05-01T00:00:00.000Z", "早"),
      ev("2026-06-10T00:00:00.000Z", "晚"),
    ]);
    expect(out.map((r) => r.text)).toEqual(["晚", "早"]);
    expect(out[0]!.period).toBe("2026 年 6 月 10 日");
  });
});
```

- [ ] **Step 2: Run, verify FAIL.**

- [ ] **Step 3: Implement**

```ts
import type { Evaluation } from "@mind-imprint/contracts";

export interface GrowthReview { period: string; text: string; }

function fmt(iso: string): string {
  const d = new Date(iso);
  return `${d.getUTCFullYear()} 年 ${d.getUTCMonth() + 1} 月 ${d.getUTCDate()} 日`;
}

export function deriveGrowthReviews(evaluations: Evaluation[]): GrowthReview[] {
  return [...evaluations]
    .sort((a, b) => (a.created_at < b.created_at ? 1 : -1))
    .map((e) => ({ period: fmt(e.created_at), text: e.narrative }));
}
```

- [ ] **Step 4: Run + typecheck** → PASS.
- [ ] **Step 5: Commit** — `git commit -am "feat(shell): growth reviews derivation"`

---

## Task 6: `cardUsage.ts` — card_instances grouped by category

**Files:**
- Create: `apps/web/src/shell/records/cardUsage.ts`
- Test: `apps/web/src/shell/records/cardUsage.test.ts`

**Interfaces:**
- Consumes: `CardInstance[]` (`{ card_id; status; ... }`) and the card registry `Record<string, CardSpec>` (`CARD_REGISTRY` from contracts; each spec has `name`, `category`, optional `purpose`/`one_liner`). Use `spec.category` for grouping and `spec.name` for the title. For the purpose line use `spec.purpose ?? spec.one_liner ?? ""` (read `CardSpec` for the exact field name; if neither exists, pass `""`).
- Produces: `deriveCardUsage(cards: CardInstance[], registry: Record<string, CardSpec>): CardGroup[]` where
  `CardGroup = { name: string; dotStyle: string; countLabel: string; cards: CardUsageView[] }` and
  `CardUsageView = { name; purpose; usage; badge; badgeStyle; boxStyle }`.
- Group order = first appearance across the registry's categories; only categories that have ≥1 used card appear (records shows what the student has collected). `usage` = `用过 N 次`. `badge` = `常用` if N≥3 else `已用`.

- [ ] **Step 1: Write the failing test**

```ts
import { describe, it, expect } from "vitest";
import { deriveCardUsage } from "./cardUsage";
import type { CardInstance, CardSpec } from "@mind-imprint/contracts";

const registry = {
  sift_craap: { id: "sift_craap", name: "SIFT×CRAAP", category: "信息素养" },
  concession: { id: "concession", name: "让步段", category: "知识工具" },
} as unknown as Record<string, CardSpec>;

function ci(card_id: string, id: string): CardInstance {
  return { id, card_id, task_id: "t1", parent_node_id: null, status: "completed",
    field_values: {}, event_trace: [], rubric_tags: [], created_at: "2026-06-01T00:00:00Z", completed_at: null };
}

describe("deriveCardUsage", () => {
  it("empty → []", () => {
    expect(deriveCardUsage([], registry)).toEqual([]);
  });
  it("groups used cards by category with counts + badges", () => {
    const groups = deriveCardUsage(
      [ci("sift_craap", "a"), ci("sift_craap", "b"), ci("sift_craap", "c"), ci("concession", "d")],
      registry,
    );
    const info = groups.find((g) => g.name === "信息素养")!;
    expect(info.cards[0]!.name).toBe("SIFT×CRAAP");
    expect(info.cards[0]!.usage).toBe("用过 3 次");
    expect(info.cards[0]!.badge).toBe("常用");
    const know = groups.find((g) => g.name === "知识工具")!;
    expect(know.cards[0]!.badge).toBe("已用");
  });
  it("skips unknown card_ids without throwing", () => {
    expect(deriveCardUsage([ci("ghost", "x")], registry)).toEqual([]);
  });
});
```

- [ ] **Step 2: Run, verify FAIL.**

- [ ] **Step 3: Implement** (read `CardSpec` for the purpose field; the test passes `""` implicitly via `?? ""`)

```ts
import type { CardInstance, CardSpec } from "@mind-imprint/contracts";

export interface CardUsageView {
  name: string; purpose: string; usage: string;
  badge: string; badgeStyle: string; boxStyle: string;
}
export interface CardGroup {
  name: string; dotStyle: string; countLabel: string; cards: CardUsageView[];
}

const DOT = "width:9px; height:9px; border-radius:50%; background:#2A3B7A; display:inline-block;";
const BOX = "background:#fff; border:1px solid #EAECF2; border-radius:14px; padding:14px 16px;";
const BADGE_COMMON = "font-size:11px; font-weight:700; color:#2A3B7A; background:#EDEFF9; padding:2px 9px; border-radius:999px;";
const BADGE_USED = "font-size:11px; font-weight:700; color:#6B7384; background:#F1F2F5; padding:2px 9px; border-radius:999px;";

export function deriveCardUsage(
  cards: CardInstance[],
  registry: Record<string, CardSpec>,
): CardGroup[] {
  const counts = new Map<string, number>();
  for (const c of cards) counts.set(c.card_id, (counts.get(c.card_id) ?? 0) + 1);

  const groups = new Map<string, CardUsageView[]>();
  const order: string[] = [];
  for (const [cardId, n] of counts) {
    const spec = registry[cardId];
    if (!spec) continue;
    const category = spec.category;
    if (!groups.has(category)) { groups.set(category, []); order.push(category); }
    const purpose = (spec as unknown as { purpose?: string; one_liner?: string }).purpose
      ?? (spec as unknown as { one_liner?: string }).one_liner ?? "";
    groups.get(category)!.push({
      name: spec.name, purpose, usage: `用过 ${n} 次`,
      badge: n >= 3 ? "常用" : "已用",
      badgeStyle: n >= 3 ? BADGE_COMMON : BADGE_USED,
      boxStyle: BOX,
    });
  }
  return order.map((category) => {
    const cs = groups.get(category)!;
    return { name: category, dotStyle: DOT, countLabel: `${cs.length} 张`, cards: cs };
  });
}
```

- [ ] **Step 4: Run + typecheck** → PASS.
- [ ] **Step 5: Commit** — `git commit -am "feat(shell): card usage derivation by category"`

---

## Task 7: `ability.ts` + `radarGeometry.ts` — 9-dim aggregation + radar trig

**Files:**
- Create: `apps/web/src/shell/records/ability.ts`, `apps/web/src/shell/records/radarGeometry.ts`
- Test: `apps/web/src/shell/records/ability.test.ts`, `apps/web/src/shell/records/radarGeometry.test.ts`

**Interfaces:**
- Consumes: `Evaluation[]`, `FULL_RUBRIC`, `SoloLevel`, `SOLO_LABELS` from contracts.
- `deriveAbility(evaluations: Evaluation[], rubric: RubricDimension[]): AbilityDim[]` where `AbilityDim = { dimId; dim; level: SoloLevel; levelLabel; segs: {style:string}[]; dotStyle:string }`. Per dim: take the **latest** evaluation that scored it (by `created_at`); dims never scored → `L1`. `levelLabel` = `SOLO_LABELS[level]`. `segs` = 4 cells, first `n` filled (n = level number).
- `radarGeometry(levels: number[], cx?: number, cy?: number, r?: number): { rings; axes; polygonPoints; dots; labels }` for the design's 280×270 viewBox (default cx=140, cy=128, r=96). `levels.length` must equal the number of axes (9). `labels` come from a parallel `names: string[]` arg.

- [ ] **Step 1: Write the failing tests**

`ability.test.ts`:
```ts
import { describe, it, expect } from "vitest";
import { deriveAbility } from "./ability";
import { FULL_RUBRIC } from "@mind-imprint/contracts";
import type { Evaluation } from "@mind-imprint/contracts";

function ev(created_at: string, scores: { dim_id: string; level: "L1"|"L2"|"L3"|"L4" }[]): Evaluation {
  return { task_id: "t1", created_at, narrative: "",
    scores: scores.map((s) => ({ ...s, note: "" })) };
}

describe("deriveAbility", () => {
  it("returns one entry per rubric dim in rubric order", () => {
    const out = deriveAbility([], FULL_RUBRIC);
    expect(out.map((a) => a.dimId)).toEqual(FULL_RUBRIC.map((d) => d.id));
  });
  it("unscored dims default to L1", () => {
    const out = deriveAbility([], FULL_RUBRIC);
    expect(out.every((a) => a.level === "L1")).toBe(true);
  });
  it("uses the latest evaluation's score per dim", () => {
    const out = deriveAbility([
      ev("2026-05-01T00:00:00Z", [{ dim_id: "D2", level: "L2" }]),
      ev("2026-06-01T00:00:00Z", [{ dim_id: "D2", level: "L4" }]),
    ], FULL_RUBRIC);
    const d2 = out.find((a) => a.dimId === "D2")!;
    expect(d2.level).toBe("L4");
    expect(d2.levelLabel).toContain("卓越");
    expect(d2.segs.filter((s) => s.style.includes("#2A3B7A"))).toHaveLength(4);
  });
});
```

`radarGeometry.test.ts`:
```ts
import { describe, it, expect } from "vitest";
import { radarGeometry } from "./radarGeometry";

describe("radarGeometry", () => {
  it("produces one axis/dot/label per level and 4 rings", () => {
    const names = ["D1","D2","D3","D4","D5","D6","D7","D8","D9"];
    const g = radarGeometry([1,2,3,4,1,2,3,4,2], names);
    expect(g.axes).toHaveLength(9);
    expect(g.dots).toHaveLength(9);
    expect(g.labels).toHaveLength(9);
    expect(g.rings).toHaveLength(4);
    expect(g.polygonPoints.split(" ")).toHaveLength(9);
  });
  it("dot radius grows with level (L4 farther from center than L1)", () => {
    const g = radarGeometry([1,1,1,1,1,1,1,1,4], new Array(9).fill("x"));
    const center = { x: 140, y: 128 };
    const dist = (d: { cx: number; cy: number }) => Math.hypot(d.cx - center.x, d.cy - center.y);
    expect(dist(g.dots[8]!)).toBeGreaterThan(dist(g.dots[0]!));
  });
});
```

- [ ] **Step 2: Run, verify FAIL.**

- [ ] **Step 3: Implement `radarGeometry.ts`**

```ts
export interface RadarDot { cx: number; cy: number; }
export interface RadarAxis { x1: number; y1: number; x2: number; y2: number; }
export interface RadarLabel { x: number; y: number; anchor: string; text: string; }
export interface RadarGeometry {
  rings: { points: string }[];
  axes: RadarAxis[];
  polygonPoints: string;
  dots: RadarDot[];
  labels: RadarLabel[];
}

export function radarGeometry(
  levels: number[],
  names: string[],
  cx = 140, cy = 128, r = 96,
): RadarGeometry {
  const n = levels.length;
  const angle = (i: number) => -Math.PI / 2 + (i * 2 * Math.PI) / n; // start at top
  const pt = (i: number, radius: number) => ({
    x: cx + radius * Math.cos(angle(i)),
    y: cy + radius * Math.sin(angle(i)),
  });

  const rings = [0.25, 0.5, 0.75, 1].map((f) => ({
    points: Array.from({ length: n }, (_, i) => {
      const p = pt(i, r * f); return `${p.x.toFixed(1)},${p.y.toFixed(1)}`;
    }).join(" "),
  }));

  const axes: RadarAxis[] = Array.from({ length: n }, (_, i) => {
    const p = pt(i, r); return { x1: cx, y1: cy, x2: +p.x.toFixed(1), y2: +p.y.toFixed(1) };
  });

  const dots: RadarDot[] = levels.map((lv, i) => {
    const p = pt(i, (r * lv) / 4); return { cx: +p.x.toFixed(1), cy: +p.y.toFixed(1) };
  });

  const polygonPoints = dots.map((d) => `${d.cx},${d.cy}`).join(" ");

  const labels: RadarLabel[] = names.map((text, i) => {
    const p = pt(i, r + 16);
    const anchor = Math.abs(p.x - cx) < 4 ? "middle" : p.x > cx ? "start" : "end";
    return { x: +p.x.toFixed(1), y: +p.y.toFixed(1), anchor, text };
  });

  return { rings, axes, polygonPoints, dots, labels };
}
```

- [ ] **Step 4: Implement `ability.ts`**

```ts
import type { Evaluation, RubricDimension, SoloLevel } from "@mind-imprint/contracts";
import { SOLO_LABELS } from "@mind-imprint/contracts";

const LEVEL_NUMBER: Record<SoloLevel, number> = { L1: 1, L2: 2, L3: 3, L4: 4 };
const SEG_FILLED = "flex:1; height:6px; border-radius:999px; background:#2A3B7A;";
const SEG_EMPTY = "flex:1; height:6px; border-radius:999px; background:#EEF0F4;";
const DOT = "width:9px; height:9px; border-radius:50%; background:#2A3B7A; display:inline-block;";

export interface AbilityDim {
  dimId: string; dim: string; level: SoloLevel; levelLabel: string;
  segs: { style: string }[]; dotStyle: string;
}

export function deriveAbility(evaluations: Evaluation[], rubric: RubricDimension[]): AbilityDim[] {
  // latest score per dim id
  const latest = new Map<string, { level: SoloLevel; at: string }>();
  for (const e of evaluations) {
    for (const s of e.scores) {
      const prev = latest.get(s.dim_id);
      if (!prev || e.created_at > prev.at) latest.set(s.dim_id, { level: s.level, at: e.created_at });
    }
  }
  return rubric.map((d) => {
    const level = latest.get(d.id)?.level ?? "L1";
    const n = LEVEL_NUMBER[level];
    const segs = [1, 2, 3, 4].map((i) => ({ style: i <= n ? SEG_FILLED : SEG_EMPTY }));
    return { dimId: d.id, dim: d.name, level, levelLabel: `${level} · ${SOLO_LABELS[level]}`, segs, dotStyle: DOT };
  });
}
```

- [ ] **Step 5: Run tests + typecheck** → PASS.
- [ ] **Step 6: Commit** — `git commit -am "feat(shell): ability aggregation + radar geometry"`

---

## Task 8: `taskCardView.ts` — directory task-card mapping

**Files:**
- Create: `apps/web/src/shell/directory/taskCardView.ts`
- Test: `apps/web/src/shell/directory/taskCardView.test.ts`

**Interfaces:**
- Consumes: `Task` (`{ title; status; created_at; last_active_at }`) + that task's `CardInstance[]`.
- Produces: `taskCardView(task: Task, cards: CardInstance[], now: Date): TaskCardView` where
  `TaskCardView = { title; status; statusStyle; last; barStyle; cardsLabel }`.
  - `status`: `active`→`进行中`, `evaluated`→`已评估`.
  - `last`: relative time from `last_active_at` (`刚刚` / `N 分钟前` / `N 小时前` / `N 天前`).
  - completed card count = `cards.filter(c => c.status === "completed").length`; `cardsLabel` = `N 张卡`; bar width = `min(100, completed*25)%`.

- [ ] **Step 1: Write the failing test**

```ts
import { describe, it, expect } from "vitest";
import { taskCardView } from "./taskCardView";
import type { Task, CardInstance } from "@mind-imprint/contracts";

const NOW = new Date("2026-06-21T12:00:00.000Z");
function task(o: Partial<Task> = {}): Task {
  return { id: "t1", title: "中国是否让地球更可持续？", seed: null, status: "active",
    created_at: "2026-06-21T10:00:00.000Z", last_active_at: "2026-06-21T10:00:00.000Z", ...o };
}
function card(status: CardInstance["status"]): CardInstance {
  return { id: Math.random().toString(), card_id: "sift_craap", task_id: "t1", parent_node_id: null,
    status, field_values: {}, event_trace: [], rubric_tags: [], created_at: "x", completed_at: null };
}

describe("taskCardView", () => {
  it("maps active status + relative time + card count", () => {
    const v = taskCardView(task(), [card("completed"), card("completed"), card("skipped")], NOW);
    expect(v.status).toBe("进行中");
    expect(v.last).toBe("2 小时前");
    expect(v.cardsLabel).toBe("2 张卡");
    expect(v.barStyle).toContain("50%"); // 2*25
  });
  it("maps evaluated status", () => {
    expect(taskCardView(task({ status: "evaluated" }), [], NOW).status).toBe("已评估");
  });
});
```

- [ ] **Step 2: Run, verify FAIL.**

- [ ] **Step 3: Implement**

```ts
import type { Task, CardInstance } from "@mind-imprint/contracts";

export interface TaskCardView {
  title: string; status: string; statusStyle: string;
  last: string; barStyle: string; cardsLabel: string;
}

const PILL_ACTIVE = "font-size:11.5px; font-weight:600; color:#4C9A82; background:#E7F3EE; padding:3px 10px; border-radius:999px;";
const PILL_DONE = "font-size:11.5px; font-weight:600; color:#2A3B7A; background:#EDEFF9; padding:3px 10px; border-radius:999px;";

function relTime(iso: string, now: Date): string {
  const diff = now.getTime() - new Date(iso).getTime();
  const min = Math.floor(diff / 60000);
  if (min < 1) return "刚刚";
  if (min < 60) return `${min} 分钟前`;
  const hr = Math.floor(min / 60);
  if (hr < 24) return `${hr} 小时前`;
  return `${Math.floor(hr / 24)} 天前`;
}

export function taskCardView(task: Task, cards: CardInstance[], now: Date): TaskCardView {
  const completed = cards.filter((c) => c.status === "completed").length;
  const pct = Math.min(100, completed * 25);
  return {
    title: task.title,
    status: task.status === "evaluated" ? "已评估" : "进行中",
    statusStyle: task.status === "evaluated" ? PILL_DONE : PILL_ACTIVE,
    last: relTime(task.last_active_at, now),
    cardsLabel: `${completed} 张卡`,
    barStyle: `width:${pct}%; height:100%; background:#2A3B7A; border-radius:999px;`,
  };
}
```

- [ ] **Step 4: Run + typecheck** → PASS.
- [ ] **Step 5: Commit** — `git commit -am "feat(shell): directory task-card view mapping"`

---

## Task 9: `AuthScreen.tsx` — login / register / bind (click-through)

**Files:**
- Create: `apps/web/src/shell/auth/AuthScreen.tsx`
- Test: `apps/web/src/shell/auth/AuthScreen.test.tsx`

**Interfaces:**
- Consumes: nothing from store.
- Produces: `<AuthScreen onEnterApp={() => void} />`. Internal `useState<"login"|"register"|"bind">("login")`. Markup lifted verbatim from binding HTML **lines 31–98** (login 53–62, register 64–75, bind 77–93), `style="..."` → JSX style objects. Inputs are cosmetic with the design's pre-filled Phoebe values.
- Binding map: `onEnterApp` ← 登录 button (line 60), 完成进入 (91), 暂时跳过 (92). `onGoRegister` ← 注册 link (61) → step `register`. `onGoLogin` ← 登录 link (74) → step `login`. `onGoBind` ← 下一步 (73) → step `bind`.

- [ ] **Step 1: Write the failing test**

```ts
import { describe, it, expect, vi } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import { AuthScreen } from "./AuthScreen";

describe("AuthScreen", () => {
  it("starts on login and enters the app on 登录", () => {
    const onEnterApp = vi.fn();
    render(<AuthScreen onEnterApp={onEnterApp} />);
    fireEvent.click(screen.getByRole("button", { name: "登录" }));
    expect(onEnterApp).toHaveBeenCalledOnce();
  });
  it("navigates login → register → bind → enter", () => {
    const onEnterApp = vi.fn();
    render(<AuthScreen onEnterApp={onEnterApp} />);
    fireEvent.click(screen.getByText("注册"));
    fireEvent.click(screen.getByRole("button", { name: /下一步/ }));
    fireEvent.click(screen.getByRole("button", { name: /完成/ }));
    expect(onEnterApp).toHaveBeenCalledOnce();
  });
  it("暂时跳过 enters the app", () => {
    const onEnterApp = vi.fn();
    render(<AuthScreen onEnterApp={onEnterApp} />);
    fireEvent.click(screen.getByText("注册"));
    fireEvent.click(screen.getByRole("button", { name: /下一步/ }));
    fireEvent.click(screen.getByText("暂时跳过"));
    expect(onEnterApp).toHaveBeenCalledOnce();
  });
});
```

- [ ] **Step 2: Run, verify FAIL.**

- [ ] **Step 3: Implement** — `AuthScreen.tsx` with `const [step, setStep] = useState<"login"|"register"|"bind">("login")`, rendering the outer card (HTML 33–95) and the active step block. Lift the three `<sc-if>` blocks (53–62, 64–75, 77–93) verbatim as conditional JSX. Wire the buttons per the binding map. Logo SVG lifted from HTML 37–46.

- [ ] **Step 4: Run tests + typecheck** → PASS.
- [ ] **Step 5: Commit** — `git commit -am "feat(shell): AuthScreen login/register/bind click-through"`

---

## Task 10: `LeftRail.tsx` — navigation rail

**Files:**
- Create: `apps/web/src/shell/LeftRail.tsx`
- Test: `apps/web/src/shell/LeftRail.test.tsx`

**Interfaces:**
- Produces: `<LeftRail tab={"tasks"|"records"|"settings"} onTab={(t) => void} />`. Markup from HTML **104–141** (logo 106–115; 任务 117–122; 记录 124–129; 设置 131–136; avatar dot 138–140 → `onTab("settings")`). Active item gets the filled box style; inactive the muted style. Add `role="tab"` + `aria-selected={tab===...}` to each item.
- Active styles (from design `nav*.box/.icon/.label`): active box `background:#EDEFF9;`, icon stroke `#2A3B7A`, label `color:#2A3B7A; font-weight:700`; inactive box transparent, icon `#9AA1B0`, label `color:#9AA1B0`. Box base: `width:42px; height:42px; border-radius:12px; display:flex; align-items:center; justify-content:center;`.

- [ ] **Step 1: Write the failing test**

```ts
import { describe, it, expect, vi } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import { LeftRail } from "./LeftRail";

describe("LeftRail", () => {
  it("marks the active tab via aria-selected", () => {
    render(<LeftRail tab="records" onTab={() => {}} />);
    const tabs = screen.getAllByRole("tab");
    const records = tabs.find((t) => t.textContent?.includes("记录"))!;
    expect(records.getAttribute("aria-selected")).toBe("true");
  });
  it("fires onTab when a nav item is clicked", () => {
    const onTab = vi.fn();
    render(<LeftRail tab="tasks" onTab={onTab} />);
    fireEvent.click(screen.getByText("设置"));
    expect(onTab).toHaveBeenCalledWith("settings");
  });
});
```

- [ ] **Step 2: Run, verify FAIL.**
- [ ] **Step 3: Implement** — map over `[{key:"tasks",label:"任务",icon:...},{key:"records",...},{key:"settings",...}]`, each a `role="tab"` div with `aria-selected={tab===key}`, computed box/icon/label styles, `onClick={() => onTab(key)}`. Icons lifted from HTML 119/126/133. Avatar dot (138–140) also calls `onTab("settings")`.
- [ ] **Step 4: Run + typecheck** → PASS.
- [ ] **Step 5: Commit** — `git commit -am "feat(shell): LeftRail nav with a11y tabs"`

---

## Task 11: `DirectoryView.tsx` — home + new-task + task grid

**Files:**
- Create: `apps/web/src/shell/directory/DirectoryView.tsx`
- Test: `apps/web/src/shell/directory/DirectoryView.test.tsx`

**Interfaces:**
- Consumes: `Store`, `taskCardView` (Task 8). `useStore`/`useSyncExternalStore` for the task list.
- Produces: `<DirectoryView store={store} onOpenTask={(taskId: string) => void} now?: () => Date />`. Markup from HTML **146–196** (greeting 150–151; new-task entry 154–170; list header 172–175; grid 177–193).
- New-task flow on 开始 (HTML 166): trim input; if empty, do nothing. Extract the first URL (regex `/(https?:\/\/\S+)/`) as `seed` (else `null`). `const t = store.createTask({ title: trimmed, seed })`. Seed the first student message: `store.appendMessage({ task_id: t.id, role: "user", content: trimmed })`. Then `onOpenTask(t.id)`. Existing task card click (HTML 179) → `onOpenTask(t.id)`.

- [ ] **Step 1: Write the failing test**

```ts
import { describe, it, expect, vi } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import { DirectoryView } from "./DirectoryView";
import { createStore, makeMemoryStorage } from "../../store";

function freshStore() {
  let i = 0;
  return createStore({ storage: makeMemoryStorage(), genId: () => `id${i++}`, now: () => "2026-06-21T10:00:00.000Z" });
}

describe("DirectoryView", () => {
  it("creates a task + seed message and opens it", () => {
    const store = freshStore();
    const onOpenTask = vi.fn();
    render(<DirectoryView store={store} onOpenTask={onOpenTask} now={() => new Date("2026-06-21T12:00:00Z")} />);
    fireEvent.change(screen.getByPlaceholderText(/把你正在纠结的问题/), {
      target: { value: "中国是否让地球更可持续？ https://mp.weixin.qq.com/s/x" },
    });
    fireEvent.click(screen.getByRole("button", { name: /开始/ }));
    const tasks = store.listTasks();
    expect(tasks).toHaveLength(1);
    expect(tasks[0]!.seed).toBe("https://mp.weixin.qq.com/s/x");
    expect(store.listMessages(tasks[0]!.id)).toHaveLength(1);
    expect(onOpenTask).toHaveBeenCalledWith(tasks[0]!.id);
  });
  it("empty input does not create a task", () => {
    const store = freshStore();
    render(<DirectoryView store={store} onOpenTask={vi.fn()} />);
    fireEvent.click(screen.getByRole("button", { name: /开始/ }));
    expect(store.listTasks()).toHaveLength(0);
  });
  it("renders an existing task card and opens it on click", () => {
    const store = freshStore();
    const t = store.createTask({ title: "已有任务", seed: null });
    const onOpenTask = vi.fn();
    render(<DirectoryView store={store} onOpenTask={onOpenTask} />);
    fireEvent.click(screen.getByText("已有任务"));
    expect(onOpenTask).toHaveBeenCalledWith(t.id);
  });
});
```

- [ ] **Step 2: Run, verify FAIL.**
- [ ] **Step 3: Implement** — subscribe via `useSyncExternalStore(store.subscribe, store.getSnapshot)`; `const tasks = store.listTasks()`; `taskCount = tasks.length`. Local `useState("")` for the input. `now = props.now ?? (() => new Date())`. Render greeting + new-task entry (lift 154–170; input `placeholder` exactly `把你正在纠结的问题写下来——带上你自己的东西（链接、草稿、本子上的话）。`) + grid of `taskCardView(t, store.listCards(t.id), now())` mapped to the card markup (179–191). Logo SVG from 155–164.
- [ ] **Step 4: Run tests + typecheck** → PASS.
- [ ] **Step 5: Commit** — `git commit -am "feat(shell): DirectoryView home + new-task + task grid"`

---

## Task 12: `RecordsView.tsx` — learning / cards / ability tabs

**Files:**
- Create: `apps/web/src/shell/records/RecordsView.tsx`
- Test: `apps/web/src/shell/records/RecordsView.test.tsx`

**Interfaces:**
- Consumes: `Store`, `FULL_RUBRIC`, `CARD_REGISTRY` from contracts, and the four derivations (Tasks 4–7).
- Produces: `<RecordsView store={store} registry={CARD_REGISTRY} now?: () => Date />`. Internal `useState<"learning"|"cards"|"ability">("learning")` (HTML tab pills 631–635). Markup from HTML **624–747**.
- Derivation wiring: gather events = `[...messages, ...cards, ...evaluations]` across all tasks (each has `created_at`) for the calendar; `deriveGrowthReviews(all evaluations)`; `deriveCardUsage(all cards, registry)`; `deriveAbility(all evaluations, FULL_RUBRIC)` + `radarGeometry(levels, names)` where `levels = ability.map(a => LEVEL_NUMBER[a.level])` and `names = ability.map(a => a.dim)`.
- Empty-states: calendar always renders (all-low when empty); reviews list shows `还没有评估记录。完成一次任务并生成思维印记后，这里会留下你的成长回顾。` when empty; cards tab shows `你还没有用过工具卡。` when no groups; ability always renders (defaults to L1).

- [ ] **Step 1: Write the failing test**

```ts
import { describe, it, expect, fireEvent } from "vitest"; // note: fireEvent from @testing-library
import { render, screen } from "@testing-library/react";
import { fireEvent as fe } from "@testing-library/react";
import { RecordsView } from "./RecordsView";
import { createStore, makeMemoryStorage } from "../../store";
import { CARD_REGISTRY } from "@mind-imprint/contracts";

const NOW = () => new Date("2026-06-21T12:00:00Z");

describe("RecordsView", () => {
  it("shows the learning tab by default with the activity calendar header", () => {
    const store = createStore({ storage: makeMemoryStorage() });
    render(<RecordsView store={store} registry={CARD_REGISTRY} now={NOW} />);
    expect(screen.getByText("活跃日历")).toBeTruthy();
  });
  it("switches to the 工具卡 tab and shows the empty-state when no cards", () => {
    const store = createStore({ storage: makeMemoryStorage() });
    render(<RecordsView store={store} registry={CARD_REGISTRY} now={NOW} />);
    fe.click(screen.getByText("工具卡"));
    expect(screen.getByText(/还没有用过工具卡/)).toBeTruthy();
  });
  it("switches to the 能力素养 tab and renders 9 ability rows", () => {
    const store = createStore({ storage: makeMemoryStorage() });
    render(<RecordsView store={store} registry={CARD_REGISTRY} now={NOW} />);
    fe.click(screen.getByText("能力素养"));
    // 9 dim names from FULL_RUBRIC appear
    expect(screen.getByText("提问清晰度")).toBeTruthy();
    expect(screen.getByText("AI 边界与伦理")).toBeTruthy();
  });
});
```
(Use the standard `import { render, screen, fireEvent } from "@testing-library/react"` — the duplicate import above is illustrative; the implementer writes the clean version.)

- [ ] **Step 2: Run, verify FAIL.**
- [ ] **Step 3: Implement** — `RecordsView` subscribes to the store, computes the four derivations from all-task aggregates, and renders the active tab. Tab pills lift 631–635 (active pill `background:#fff; box-shadow:...`, inactive transparent). LearningTab lifts 638–671; CardsTab lifts 675–697 with the softened intro copy `你收集到的思维工具卡，按课程库分支组织。用得越多，越成为你的本能。`; AbilityTab lifts 701–742 (radar svg consumes `radarGeometry`, list consumes `deriveAbility`). Define `LEVEL_NUMBER` locally or import from a shared util.
- [ ] **Step 4: Run tests + typecheck** → PASS.
- [ ] **Step 5: Commit** — `git commit -am "feat(shell): RecordsView learning/cards/ability tabs"`

---

## Task 13: `LlmConfigForm.tsx` — shared LLM config + 测试连接

**Files:**
- Create: `apps/web/src/shell/settings/LlmConfigForm.tsx`
- Test: `apps/web/src/shell/settings/LlmConfigForm.test.tsx`

**Interfaces:**
- Consumes: `loadConfig`, `saveConfig`, `markVerified`, `isConfigured` from `../../llm`; a `chat: ChatFn` prop (injected so tests can stub) defaulting to the real `chat`.
- Produces: `<LlmConfigForm chat={chatFn} onVerified={() => void} />`. Local state mirrors `loadConfig()` fields (format/baseUrl/model/evalModel/apiKey) + a `testState: "idle"|"testing"|"ok"|"fail"` + `testError: string`.
- 测试连接 handler: build `LlmConfig` from fields; if not `isConfigured`, set fail with `请先填写完整配置`; else `saveConfig(cfg)`, set `testing`, `await chat({ config: cfg, messages: [{role:"user", content:"ping"}], maxTokens: 4 })` (read `chat`'s real signature and match it); on success `markVerified(cfg)`, `testState="ok"`, `onVerified()`; on throw `testState="fail"`, `testError = e.message` (NEVER include the key — the message comes from `LlmError`, which already excludes it). Editing any field resets `testState` to `idle`.

- [ ] **Step 1: Write the failing test**

```ts
import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { LlmConfigForm } from "./LlmConfigForm";

beforeEach(() => localStorage.clear());

function fill() {
  fireEvent.change(screen.getByLabelText("Base URL"), { target: { value: "https://api/v1" } });
  fireEvent.change(screen.getByLabelText("模型"), { target: { value: "gpt-x" } });
  fireEvent.change(screen.getByLabelText("API Key"), { target: { value: "sk-secret" } });
}

describe("LlmConfigForm", () => {
  it("calls onVerified after a passing 测试连接 and never renders the key in errors", async () => {
    const chat = vi.fn().mockResolvedValue({ text: "pong" });
    const onVerified = vi.fn();
    render(<LlmConfigForm chat={chat} onVerified={onVerified} />);
    fill();
    fireEvent.click(screen.getByRole("button", { name: /测试连接/ }));
    await waitFor(() => expect(onVerified).toHaveBeenCalledOnce());
    expect(localStorage.getItem("mk.llmConfig")).toContain("\"verified\":true");
  });
  it("shows a failure state without leaking the key", async () => {
    const chat = vi.fn().mockRejectedValue(new Error("401 unauthorized"));
    render(<LlmConfigForm chat={chat} onVerified={vi.fn()} />);
    fill();
    fireEvent.click(screen.getByRole("button", { name: /测试连接/ }));
    await waitFor(() => expect(screen.getByText(/连接失败/)).toBeTruthy());
    expect(document.body.textContent).not.toContain("sk-secret");
  });
  it("rejects an incomplete config", () => {
    render(<LlmConfigForm chat={vi.fn()} onVerified={vi.fn()} />);
    fireEvent.click(screen.getByRole("button", { name: /测试连接/ }));
    expect(screen.getByText(/请先填写完整配置/)).toBeTruthy();
  });
});
```

- [ ] **Step 2: Run, verify FAIL.**
- [ ] **Step 3: Implement** — read the real `chat` signature in `apps/web/src/llm/client.ts` and match the call/return shape exactly. Provider is a `<select>` (openai/anthropic); fields are labeled inputs (`姓名`-style labels: `Base URL`, `模型`, `评估模型（可选）`, `API Key` as `type="password"`). The 测试连接 button + ✓/✗ status row styled to match the design's card inputs (border `#E1E4ED`, radius 11px). Wrap fields in the design's card chrome.
- [ ] **Step 4: Run tests + typecheck** → PASS.
- [ ] **Step 5: Commit** — `git commit -am "feat(shell): LlmConfigForm with 测试连接"`

---

## Task 14: `SettingsView.tsx` — profile + avatar + 模型/API + logout

**Files:**
- Create: `apps/web/src/shell/settings/SettingsView.tsx`
- Test: `apps/web/src/shell/settings/SettingsView.test.tsx`

**Interfaces:**
- Consumes: `SessionStore` + `useSession` (Task 2), `LlmConfigForm` (Task 13), `chat` from `../../llm`.
- Produces: `<SettingsView session={sessionStore} chat={chatFn} onLogout={() => void} />`. Markup from HTML **749–828** (profile 755–769 cosmetic; AI 形象 771–805; 其他 toggles 807–825; logout 821–824), plus a NEW **模型 / API** card (placed between AI 形象 and 其他) wrapping `<LlmConfigForm chat={chat} onVerified={() => {}} />`.
- Avatar picker (790–804): `avatarOptions` = `["#2A3B7A","#D98263","#4C9A82","#E8A33D"]`; clicking one calls `session.setAvatar(color)`; the selected wrapper gets a highlight ring. Toggles (810–820) are cosmetic local `useState` booleans (no behavior). 退出登录 → `onLogout()`.

- [ ] **Step 1: Write the failing test**

```ts
import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import { SettingsView } from "./SettingsView";
import { createSession } from "../session";
import { makeMemoryStorage } from "../../store";

beforeEach(() => localStorage.clear());

describe("SettingsView", () => {
  it("persists a chosen avatar color to the session", () => {
    const session = createSession({ storage: makeMemoryStorage() });
    render(<SettingsView session={session} chat={vi.fn()} onLogout={vi.fn()} />);
    // click the 2nd avatar option (#D98263)
    const swatches = screen.getAllByTestId("avatar-option");
    fireEvent.click(swatches[1]!);
    expect(session.getSnapshot().aiAvatar).toBe("#D98263");
  });
  it("fires onLogout from 退出登录", () => {
    const session = createSession({ storage: makeMemoryStorage() });
    const onLogout = vi.fn();
    render(<SettingsView session={session} chat={vi.fn()} onLogout={onLogout} />);
    fireEvent.click(screen.getByText("退出登录"));
    expect(onLogout).toHaveBeenCalledOnce();
  });
  it("renders the 模型 / API section", () => {
    const session = createSession({ storage: makeMemoryStorage() });
    render(<SettingsView session={session} chat={vi.fn()} onLogout={vi.fn()} />);
    expect(screen.getByText("模型 / API")).toBeTruthy();
  });
});
```

- [ ] **Step 2: Run, verify FAIL.**
- [ ] **Step 3: Implement** — `useSession(session)` for the current avatar; render the four sections. Give each avatar option a `data-testid="avatar-option"` and selected-ring style when `o.color === aiAvatar`. Mount `LlmConfigForm` in the 模型/API card. Logout row lifts 821–824.
- [ ] **Step 4: Run tests + typecheck** → PASS.
- [ ] **Step 5: Commit** — `git commit -am "feat(shell): SettingsView profile/avatar/model/logout"`

---

## Task 15: `KeyGateModal.tsx` — non-dismissable first-run gate

**Files:**
- Create: `apps/web/src/shell/KeyGateModal.tsx`
- Test: `apps/web/src/shell/KeyGateModal.test.tsx`

**Interfaces:**
- Consumes: `LlmConfigForm` (Task 13).
- Produces: `<KeyGateModal chat={chatFn} onPass={() => void} />` — a full-screen scrim (no close affordance, scrim click does nothing) containing a card with a short headline (`先连接你的模型`), a one-line explanation (`思维印记需要你自己的模型 API Key 才能开始。它只存在你的浏览器里，不上传任何服务器。`), and `<LlmConfigForm chat={chat} onVerified={onPass} />`. The host (AppShell) decides whether to mount it.

- [ ] **Step 1: Write the failing test**

```ts
import { describe, it, expect, vi } from "vitest";
import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import { KeyGateModal } from "./KeyGateModal";

describe("KeyGateModal", () => {
  it("has no close button and does not pass on scrim click", () => {
    const onPass = vi.fn();
    const { container } = render(<KeyGateModal chat={vi.fn()} onPass={onPass} />);
    expect(screen.queryByRole("button", { name: /关闭|×|取消/ })).toBeNull();
    fireEvent.click(container.firstChild as Element); // scrim
    expect(onPass).not.toHaveBeenCalled();
  });
  it("calls onPass after a successful connection test", async () => {
    const chat = vi.fn().mockResolvedValue({ text: "pong" });
    const onPass = vi.fn();
    render(<KeyGateModal chat={chat} onPass={onPass} />);
    fireEvent.change(screen.getByLabelText("Base URL"), { target: { value: "https://api/v1" } });
    fireEvent.change(screen.getByLabelText("模型"), { target: { value: "m" } });
    fireEvent.change(screen.getByLabelText("API Key"), { target: { value: "sk-x" } });
    fireEvent.click(screen.getByRole("button", { name: /测试连接/ }));
    await waitFor(() => expect(onPass).toHaveBeenCalledOnce());
  });
});
```

- [ ] **Step 2: Run, verify FAIL.**
- [ ] **Step 3: Implement** — fixed-position scrim `position:fixed; inset:0; background:rgba(20,30,60,.45); display:flex; align-items:center; justify-content:center; z-index:50;` with `onClick` that does nothing; inner card stops propagation. No X / cancel.
- [ ] **Step 4: Run tests + typecheck** → PASS.
- [ ] **Step 5: Commit** — `git commit -am "feat(shell): non-dismissable KeyGateModal"`

---

## Task 16: `WorkspaceContainer.tsx` — per-task controller lifecycle

**Files:**
- Create: `apps/web/src/shell/WorkspaceContainer.tsx`
- Test: `apps/web/src/shell/WorkspaceContainer.test.tsx`

**Interfaces:**
- Consumes: `createConversation`, `createEvaluator` from `../agent`; `demoCatalog` from `../agent/prompt`; `WorkspaceView` from `../workspace`; `CARD_REGISTRY`, `deriveCatalog` from contracts; `chat`, `loadConfig` from `../llm`; `Store`.
- Produces: `<WorkspaceContainer store={store} taskId={string} onBack={() => void} chat?={chatFn} config?={Partial<LlmConfig>} />`. Builds `conversation` + `evaluator` for `taskId` in a `useMemo` keyed on `taskId` (recreate when it changes) and renders `<WorkspaceView store={store} conversation={conversation} evaluator={evaluator} taskId={taskId} onBack={onBack} />`.
- `chat`/`config` are injectable for tests (default real `chat` and `loadConfig()`). `catalog = demoCatalog(deriveCatalog(CARD_REGISTRY))` computed once (module const).

- [ ] **Step 1: Write the failing test**

```ts
import { describe, it, expect, vi } from "vitest";
import { render, screen, fireEvent, act } from "@testing-library/react";
import { WorkspaceContainer } from "./WorkspaceContainer";
import { createStore, makeMemoryStorage } from "../store";

function storeWithTwoTasks() {
  const store = createStore({ storage: makeMemoryStorage() });
  const a = store.createTask({ title: "任务 A", seed: null });
  const b = store.createTask({ title: "任务 B", seed: null });
  store.appendMessage({ task_id: a.id, role: "user", content: "你好来自 A" });
  store.appendMessage({ task_id: b.id, role: "user", content: "你好来自 B" });
  return { store, a, b };
}

describe("WorkspaceContainer", () => {
  it("renders the workspace for the given task and routes back", () => {
    const { store, a } = storeWithTwoTasks();
    const onBack = vi.fn();
    render(<WorkspaceContainer store={store} taskId={a.id} onBack={onBack} chat={vi.fn()} config={{}} />);
    expect(screen.getByText("你好来自 A")).toBeTruthy();
    fireEvent.click(screen.getByText("返回所有任务"));
    expect(onBack).toHaveBeenCalledOnce();
  });
  it("swaps to a fresh task's content when taskId changes", () => {
    const { store, a, b } = storeWithTwoTasks();
    const { rerender } = render(<WorkspaceContainer store={store} taskId={a.id} onBack={vi.fn()} chat={vi.fn()} config={{}} />);
    expect(screen.getByText("你好来自 A")).toBeTruthy();
    rerender(<WorkspaceContainer store={store} taskId={b.id} onBack={vi.fn()} chat={vi.fn()} config={{}} />);
    expect(screen.getByText("你好来自 B")).toBeTruthy();
    expect(screen.queryByText("你好来自 A")).toBeNull();
  });
});
```

- [ ] **Step 2: Run, verify FAIL.**
- [ ] **Step 3: Implement**

```tsx
import { useMemo } from "react";
import type { Store } from "../store/createStore";
import type { ChatFn } from "../agent/runEvaluation";
import type { Partial as _ } from "../llm/types"; // illustrative; import LlmConfig type properly
import type { LlmConfig } from "../llm/types";
import { chat as realChat, loadConfig } from "../llm";
import { createConversation, createEvaluator } from "../agent";
import { demoCatalog } from "../agent/prompt";
import { CARD_REGISTRY, deriveCatalog } from "@mind-imprint/contracts";
import { WorkspaceView } from "../workspace";

const CATALOG = demoCatalog(deriveCatalog(CARD_REGISTRY));

interface Props {
  store: Store;
  taskId: string;
  onBack: () => void;
  chat?: ChatFn;
  config?: Partial<LlmConfig>;
}

export function WorkspaceContainer({ store, taskId, onBack, chat, config }: Props) {
  const cfg = config ?? loadConfig();
  const chatFn = chat ?? realChat;
  const { conversation, evaluator } = useMemo(() => ({
    conversation: createConversation({ store, chat: chatFn, config: cfg, registry: CARD_REGISTRY, catalog: CATALOG, taskId }),
    evaluator: createEvaluator({ store, chat: chatFn, config: cfg, registry: CARD_REGISTRY, taskId }),
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }), [taskId]);

  return (
    <WorkspaceView store={store} conversation={conversation} evaluator={evaluator} taskId={taskId} onBack={onBack} />
  );
}
```
(Match `ChatFn`'s real import path and `createConversation`/`createEvaluator` dep shapes exactly; the `Partial as _` import line is illustrative — use plain `Partial<LlmConfig>`.)

- [ ] **Step 4: Run tests + typecheck** → PASS.
- [ ] **Step 5: Commit** — `git commit -am "feat(shell): WorkspaceContainer per-task controller lifecycle"`

---

## Task 17: `AppShell.tsx` — top-level state machine + entry swap

**Files:**
- Create: `apps/web/src/shell/AppShell.tsx`
- Modify: `apps/web/src/main.tsx`
- Test: `apps/web/src/shell/AppShell.test.tsx`

**Interfaces:**
- Consumes: everything above. Owns module-level singletons `store = createStore({ storage: window.localStorage })` and `session = createSession({ storage: window.localStorage })` (mirroring `WorkspaceDev`'s singleton pattern), OR accepts them as optional props for tests: `<AppShell store?={Store} session?={SessionStore} chat?={ChatFn} initialConfig?={Partial<LlmConfig>} />`.
- State: `screen` derived from `session.authed` (`authed ? "app" : "auth"`); `tab: "tasks"|"records"|"settings"` (useState); `taskView: "directory"|"workspace"` + `activeTaskId: string|null` (useState). Key-gate: mount `<KeyGateModal>` when `screen==="app"` AND `!isVerified(loadConfig())` (track a `verified` state bumped on gate pass).
- Layout: when `screen==="auth"` render `<AuthScreen onEnterApp={() => session.setAuthed(true)} />`. When `"app"`, render the flex layout (HTML 102) = `<LeftRail tab onTab>` + content by tab:
  - tasks + directory → `<DirectoryView store onOpenTask={(id) => { setActiveTaskId(id); setTaskView("workspace"); }} />`
  - tasks + workspace → `<WorkspaceContainer store taskId={activeTaskId!} onBack={() => setTaskView("directory")} chat config />`
  - records → `<RecordsView store registry={CARD_REGISTRY} />`
  - settings → `<SettingsView session chat onLogout={() => { session.setAuthed(false); }} />`
  - plus `<KeyGateModal chat onPass={() => setVerified(true)} />` overlay when gating.
- `onTab` resets to `taskView="directory"` when switching to tasks from elsewhere is optional; keep current `taskView` otherwise.

- [ ] **Step 1: Write the failing test**

```ts
import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import { AppShell } from "./AppShell";
import { createStore, makeMemoryStorage } from "../store";
import { createSession } from "./session";

beforeEach(() => localStorage.clear());

function deps() {
  const storage = makeMemoryStorage();
  return {
    store: createStore({ storage }),
    session: createSession({ storage: makeMemoryStorage() }),
    chat: vi.fn(),
  };
}

describe("AppShell", () => {
  it("shows auth when logged out, app after 登录", () => {
    const d = deps();
    // a verified config so the key-gate doesn't block
    localStorage.setItem("mk.llmConfig", JSON.stringify({ format: "openai", baseUrl: "b", model: "m", apiKey: "k", verified: true }));
    render(<AppShell store={d.store} session={d.session} chat={d.chat} />);
    fireEvent.click(screen.getByRole("button", { name: "登录" }));
    expect(screen.getByText("你想搞懂什么？")).toBeTruthy();
  });
  it("blocks the app with the key-gate when no verified config", () => {
    const d = deps();
    d.session.setAuthed(true);
    render(<AppShell store={d.store} session={d.session} chat={d.chat} />);
    expect(screen.getByText(/先连接你的模型/)).toBeTruthy();
  });
  it("navigates tasks → records via the rail", () => {
    const d = deps();
    d.session.setAuthed(true);
    localStorage.setItem("mk.llmConfig", JSON.stringify({ format: "openai", baseUrl: "b", model: "m", apiKey: "k", verified: true }));
    render(<AppShell store={d.store} session={d.session} chat={d.chat} />);
    fireEvent.click(screen.getByText("记录"));
    expect(screen.getByText("活跃日历")).toBeTruthy();
  });
});
```

- [ ] **Step 2: Run, verify FAIL.**
- [ ] **Step 3: Implement `AppShell.tsx`** per the interface; thread `chat`/`config` into `WorkspaceContainer` and `SettingsView`/`KeyGateModal`. Use `useSession(session)` to react to auth/avatar. Compute `verified` initial from `isVerified(loadConfig())`, with a `useState` bumped by the gate's `onPass`.
- [ ] **Step 4: Swap the entry in `main.tsx`** — replace the `DevApp` mount with `<AppShell />` (no props → uses the module-level `store`/`session` singletons). Leave `DevApp`/`Harness`/`StorePanel`/`WorkspaceDev`/`SettingsPanel` files untouched in-repo.
- [ ] **Step 5: Run tests + typecheck** → PASS.
- [ ] **Step 6: Commit** — `git commit -am "feat(shell): AppShell state machine + mount as entry"`

---

## Task 18: S5 carry-forwards — eval error UX, re-run guard, setState audit

**Files:**
- Modify: `apps/web/src/workspace/EvalModal.tsx` (or `EvalLoading.tsx`) + `apps/web/src/workspace/WorkspaceView.tsx`
- Modify: `apps/web/src/agent/createConversation.ts`
- Tests: `apps/web/src/workspace/WorkspaceView.test.tsx` (add cases), `apps/web/src/agent/createConversation.test.ts` (add stability case)

**Interfaces:**
- Consumes: existing `Evaluator` `EvalState` (`phase: "idle"|"running"|"done"|"error"; error?`), `store.getLatestEvaluation(taskId)`.

- [ ] **Step 1: Eval error UX test (WorkspaceView)** — render with an evaluator whose snapshot is `{ phase: "error", error: "网络错误" }`; assert a visible message (e.g. `评估失败` + the error text) and a `重试` button that calls `evaluator.run`.

```ts
it("shows an error message + 重试 when evaluation fails", () => {
  const evaluator = { getSnapshot: () => ({ phase: "error", error: "网络错误" }), subscribe: () => () => {}, run: vi.fn() };
  // render WorkspaceView with this evaluator (reuse the file's makeStore + fake conversation helpers)
  // expect 评估失败 text + 网络错误 + a 重试 button that calls evaluator.run
});
```

- [ ] **Step 2: Re-run guard test (WorkspaceView)** — when `store.getLatestEvaluation(taskId)` returns an evaluation, the trigger label reads `重新评估` and clicking it (after a `window.confirm` stub returning true) calls `evaluator.run`; if `confirm` returns false, `run` is not called. Implement with `vi.spyOn(window, "confirm")`.

- [ ] **Step 3: Implement the UX** — in `WorkspaceView`: derive `hasEval = !!store.getLatestEvaluation(taskId)`; trigger button label `hasEval ? "重新评估" : "生成思维印记"`; on click, if `hasEval` and `!window.confirm("重新评估会覆盖上一次的思维印记，确定吗？")` return; else `evaluator.run()` + open modal. In `EvalModal`/host: when `evalState.phase === "error"`, render the error card (`评估失败` + `evalState.error` + 重试 button → `evaluator.run()`); when `"running"`, keep `EvalLoading`.

- [ ] **Step 4: setState-swap audit fix (`createConversation.ts`)** — change lines ~54–58 so the state ref swaps only when changed:

```ts
function setState(next: Partial<ConvState>): void {
  const nextState = { ...state, ...next };
  const changed = (Object.keys(nextState) as (keyof ConvState)[]).some((k) => nextState[k] !== state[k]);
  if (changed) {
    state = nextState;
    listeners.forEach((l) => l());
  }
}
```

Add a `createConversation.test.ts` case: calling a method that results in no state change leaves `getSnapshot()` referentially identical (mirror the `createEvaluator` stability test). Confirm `createStore.commit` needs no change (it only runs on real mutations — note this in the commit message).

- [ ] **Step 5: Run full suite + typecheck** — `pnpm -r typecheck && pnpm -r test` → PASS.
- [ ] **Step 6: Commit** — `git commit -am "fix(workspace,agent): eval error UX + re-run guard + setState-swap audit"`

---

## Self-Review

**Spec coverage:** auth (T9) · left rail (T10) · directory + new-task + routing (T11, T16, T17) · records calendar/growth/usage/ability (T4–T7, T12) · settings + LLM config + key-gate (T13–T15, T17) · session (T2) · config hardening (T3) · 9-dim rubric + eval (T1) · S5 carry-forwards error/re-run/audit (T18) · entry swap (T17). All spec sections map to a task.

**Placeholder scan:** No TBD/TODO. Two illustrative-code notes are explicitly flagged for the implementer to resolve against real signatures (the `ChatFn`/`chat` call shape in T13/T16 — implementer reads `llm/client.ts`). The duplicate-import line in T12's test is flagged as illustrative.

**Type consistency:** `FULL_RUBRIC` used consistently (T1, T7, T12). `SessionStore`/`createSession`/`useSession` consistent (T2, T14, T17). `taskCardView` (T8) consumed by T11. Derivation names (`deriveActivityCalendar`/`deriveGrowthReviews`/`deriveCardUsage`/`deriveAbility`/`radarGeometry`) consistent T4–T7 → T12. `WorkspaceContainer` props consistent T16 → T17. `isVerified`/`markVerified` consistent T3 → T13 → T17.

**Open implementer notes (resolve against real code, not guesses):** (a) the exact `chat()` signature/return shape (`llm/client.ts`) for the 测试连接 ping and `WorkspaceContainer`; (b) `CardSpec`'s purpose-ish field name for `cardUsage` (fall back to `""`); (c) `createConversation`/`createEvaluator` dep object shapes (already cited but confirm).
