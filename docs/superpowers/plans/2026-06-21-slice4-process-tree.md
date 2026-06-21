# Slice 4 — 过程树（确定性实时派生）Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Derive the live process tree purely (no LLM) from the store's `task` + `card_instance`, and render it read-only in the existing `TreePanel` body so it grows in real-time as the student uses cards.

**Architecture:** A pure `deriveProcessTree(...)` over store entities + the registry (no new storage, no contract change). A `nodeView(node)` presentation mapper. The existing `TreePanel` gains a `nodes` prop and renders them per the binding HTML's `sc-for list={{tree}}` block; `WorkspaceView` feeds it `deriveProcessTree(...)` from the `useStore` snapshot, so any store change re-derives and re-renders.

**Tech Stack:** TypeScript 5, React 18, Vitest + @testing-library/react (jsdom).

## Global Constraints

- **Acceptance gate:** `pnpm -r typecheck` AND `pnpm -r test` both green. `vite build`/`vitest` do NOT typecheck — run `pnpm --filter web typecheck` in every task.
- **Pure derivation:** `deriveProcessTree` has no side effects, no LLM, no new storage — the tree is a projection of `task` + `card_instance` (PRD §9 "process_node 可派生"). It reads only its arguments.
- **Read-only panel** (not an editor). **Frozen contracts unchanged** (`Task`/`CardInstance`/`Message` shapes). No new field primitive, no card-renderer change.
- **UI is pixel-faithful** to `docs/design/思维印记_工作区.dc.html` (the `treeOpen` → `sc-for list={{tree}}` node block); use `mk-*` tokens + real Phoebe content (task root 「中国是否让地球变得更可持续？」, SIFT / 让步段 card nodes), never lorem.
- **过程即数据:** a skipped card still emits a node. The node model supports all 7 PRD types; S4 deterministically emits only `task_root`/`card_use`/`concession`/`reflection` — `sub_question`/`key_knowledge`/`attempt` are S5.
- Web tests live beside source in `apps/web/src/**`.

---

### Task 1: `ProcessNode` model + `deriveProcessTree` (pure)

**Files:**
- Create: `apps/web/src/workspace/processTree.ts`
- Test: `apps/web/src/workspace/processTree.test.ts`

**Interfaces — Produces:**
```ts
export type ProcessNodeType = "task_root" | "sub_question" | "card_use" | "attempt" | "key_knowledge" | "concession" | "reflection";
export interface ProcessNode { id: string; type: ProcessNodeType; parent_id: string | null; title: string; sub?: string; ref_id?: string; card_id?: string; status?: CardStatus; at?: string }
export function deriveProcessTree(opts: { task: Task; cards: CardInstance[]; registry: Record<string, CardSpec> }): ProcessNode[];
```

- [ ] **Step 1: Write the failing test** — `apps/web/src/workspace/processTree.test.ts`:

```ts
import { describe, it, expect } from "vitest";
import { CARD_REGISTRY } from "@mind-imprint/contracts";
import type { Task, CardInstance } from "@mind-imprint/contracts";
import { deriveProcessTree } from "./processTree";

const task: Task = { id: "t_1", title: "中国是否让地球变得更可持续？", seed: "https://example.org/article", status: "active", created_at: "2026-06-21T09:00:00.000Z", last_active_at: "2026-06-21T09:00:00.000Z" };

function card(id: string, card_id: string, status: CardInstance["status"], at: string, field_values: Record<string, unknown> = {}): CardInstance {
  return { id, card_id, task_id: "t_1", parent_node_id: null, status, field_values, event_trace: [], rubric_tags: [], created_at: at, completed_at: status === "completed" ? at : null };
}

const reg = CARD_REGISTRY;

describe("deriveProcessTree", () => {
  it("task alone → a single task_root", () => {
    const nodes = deriveProcessTree({ task, cards: [], registry: reg });
    expect(nodes).toHaveLength(1);
    expect(nodes[0]).toMatchObject({ id: "t_1", type: "task_root", parent_id: null, title: task.title });
  });

  it("a completed SIFT card → root + a card_use node under it", () => {
    const nodes = deriveProcessTree({ task, cards: [card("ci_1", "sift_craap", "completed", "2026-06-21T09:05:00.000Z", { sift: { stop: "证明中国让地球更可持续" } })], registry: reg });
    expect(nodes).toHaveLength(2);
    expect(nodes[1]).toMatchObject({ id: "ci_1", type: "card_use", parent_id: "t_1", status: "completed", card_id: "sift_craap" });
    expect(nodes[1]!.title).toBe(reg.sift_craap!.name);
    expect(typeof nodes[1]!.sub).toBe("string");
  });

  it("a concession card → a concession node; a reflexivity card → a reflection node", () => {
    const nodes = deriveProcessTree({ task, cards: [
      card("ci_a", "concession", "completed", "2026-06-21T09:06:00.000Z"),
      card("ci_b", "checkpoint", "completed", "2026-06-21T09:07:00.000Z"),
    ], registry: reg });
    expect(nodes.find((n) => n.id === "ci_a")!.type).toBe("concession");
    expect(nodes.find((n) => n.id === "ci_b")!.type).toBe("reflection");
  });

  it("a skipped card still emits a node marked skipped (过程即数据)", () => {
    const nodes = deriveProcessTree({ task, cards: [card("ci_s", "sift_craap", "skipped", "2026-06-21T09:08:00.000Z")], registry: reg });
    const n = nodes.find((x) => x.id === "ci_s")!;
    expect(n.status).toBe("skipped");
    expect(n.sub).toContain("已跳过");
  });

  it("cards are ordered by created_at, task_root first", () => {
    const nodes = deriveProcessTree({ task, cards: [
      card("ci_2", "sift_craap", "completed", "2026-06-21T09:10:00.000Z"),
      card("ci_1", "sift_craap", "completed", "2026-06-21T09:05:00.000Z"),
    ], registry: reg });
    expect(nodes.map((n) => n.id)).toEqual(["t_1", "ci_1", "ci_2"]);
  });

  it("an unknown card_id still emits a card_use node titled by the id (no throw)", () => {
    const nodes = deriveProcessTree({ task, cards: [card("ci_x", "nonexistent-card", "completed", "2026-06-21T09:09:00.000Z")], registry: reg });
    const n = nodes.find((x) => x.id === "ci_x")!;
    expect(n.type).toBe("card_use");
    expect(n.title).toBe("nonexistent-card");
  });
});
```

- [ ] **Step 2: Run to verify it fails** — `pnpm --filter web exec vitest run src/workspace/processTree.test.ts` → FAIL (module missing).

- [ ] **Step 3: Implement** — `apps/web/src/workspace/processTree.ts`:

```ts
import type { Task, CardInstance, CardSpec, CardStatus } from "@mind-imprint/contracts";

export type ProcessNodeType =
  | "task_root" | "sub_question" | "card_use"
  | "attempt" | "key_knowledge" | "concession" | "reflection";

export interface ProcessNode {
  id: string;
  type: ProcessNodeType;
  parent_id: string | null;
  title: string;
  sub?: string;
  ref_id?: string;
  card_id?: string;
  status?: CardStatus;
  at?: string;
}

function nodeTypeForCard(spec: CardSpec | undefined, card_id: string): ProcessNodeType {
  if (card_id === "concession" || spec?.id === "concession" || spec?.id === "steelman") return "concession";
  if (spec?.category?.includes("反身性")) return "reflection";
  return "card_use";
}

function firstNonEmptyValue(field_values: Record<string, unknown>): string | undefined {
  for (const step of Object.values(field_values)) {
    if (step && typeof step === "object") {
      for (const v of Object.values(step as Record<string, unknown>)) {
        if (typeof v === "string" && v.trim()) return v.trim();
      }
    }
  }
  return undefined;
}

function cardSub(card: CardInstance): string {
  if (card.status === "skipped") return "已跳过（已记录为信号）";
  if (card.status === "completed") {
    const summary = firstNonEmptyValue(card.field_values);
    return summary ? (summary.length > 40 ? summary.slice(0, 40) + "…" : summary) : "已完成";
  }
  return "进行中";
}

export function deriveProcessTree(opts: { task: Task; cards: CardInstance[]; registry: Record<string, CardSpec> }): ProcessNode[] {
  const { task, cards, registry } = opts;
  const root: ProcessNode = {
    id: task.id, type: "task_root", parent_id: null,
    title: task.title, sub: task.seed ?? undefined, at: task.created_at,
  };
  const cardNodes = [...cards]
    .sort((a, b) => (a.created_at < b.created_at ? -1 : a.created_at > b.created_at ? 1 : 0))
    .map((card): ProcessNode => {
      const spec = registry[card.card_id];
      return {
        id: card.id,
        type: nodeTypeForCard(spec, card.card_id),
        parent_id: task.id,
        title: spec?.name ?? card.card_id,
        sub: cardSub(card),
        ref_id: card.id,
        card_id: card.card_id,
        status: card.status,
        at: card.created_at,
      };
    });
  return [root, ...cardNodes];
}
```

- [ ] **Step 4: Run test + typecheck** — test PASS; `pnpm --filter web typecheck` clean.
- [ ] **Step 5: Commit** — `git commit -am "feat(workspace): deriveProcessTree (pure, deterministic)"`

---

### Task 2: `nodeView` mapping + `TreePanel` body rendering

**Files:**
- Create: `apps/web/src/workspace/nodeView.ts`
- Modify: `apps/web/src/workspace/TreePanel.tsx` (add `nodes` prop; render the node list)
- Test: `apps/web/src/workspace/nodeView.test.ts`, `apps/web/src/workspace/TreePanel.test.tsx`

**Interfaces:**
- Consumes: `ProcessNode` from `./processTree`.
- Produces: `nodeView(node: ProcessNode): { tag: string; tagStyle: CSSProperties; markerStyle: CSSProperties; rowStyle: CSSProperties; indent: boolean }` (maps type→label + colors); `TreePanel` gains `nodes: ProcessNode[]`.

- [ ] **Step 1: Write the failing tests**
  - `nodeView.test.ts`: `task_root` → tag like 「任务」; `card_use` → 「工具卡」; `concession` → 「让步」; `reflection` → 「反身」; a child node (`parent_id !== null`) → `indent === true`; root → `indent === false`. Each returns non-empty `tag` + style objects.
  - `TreePanel.test.tsx` (read the existing TreePanel first): given `nodes=[task_root]` only → shows the empty-state hint; given `nodes=[task_root, card_use(SIFT)]` → renders the card node's title and its tag; the 「过程树」 header + 只读 badge + 「边做边长 · 随评估归并枝节」 footer still present; `open={false}` still renders the collapsed rail.

- [ ] **Step 2: Run to verify they fail** — `pnpm --filter web exec vitest run src/workspace/nodeView.test.ts src/workspace/TreePanel.test.tsx` → FAIL.

- [ ] **Step 3: Implement**
  - `nodeView.ts`: a `type → { tag, accent }` table (`task_root`→{任务, 深蓝}, `card_use`→{工具卡, accent}, `concession`→{让步, orange}, `reflection`→{反身, green}, plus `sub_question`/`key_knowledge`/`attempt` mapped for S5 forward-compat), returning the `rowStyle`/`markerStyle`/`tagStyle` (lift the exact style values from the HTML `n.rowStyle`/`n.markerStyle`/`n.tagStyle` block) and `indent = node.parent_id !== null`.
  - `TreePanel.tsx`: add `nodes: ProcessNode[]` to Props. In the open body, replace the empty-state-only content: if `nodes.length <= 1` (only root) render the existing empty-state hint; else render `nodes.map(n => <node row per the HTML sc-for block, styled via nodeView(n)>)`. Keep the header, 只读 badge, and the 「边做边长…」 footer. Keep the collapsed (`open={false}`) rail unchanged. Use a stable React `key={n.id}`.

- [ ] **Step 4: Run tests + typecheck** — PASS; `pnpm --filter web typecheck` clean.
- [ ] **Step 5: Commit** — `git commit -am "feat(workspace): TreePanel renders process-tree nodes"`

---

### Task 3: Wire into `WorkspaceView` + real-time growth

**Files:**
- Modify: `apps/web/src/workspace/WorkspaceView.tsx`
- Test: `apps/web/src/workspace/WorkspaceView.test.tsx` (extend)

**Interfaces:** consumes `deriveProcessTree` (Task 1) + the `nodes`-accepting `TreePanel` (Task 2); `useStore`/`Store` + `CARD_REGISTRY`.

- [ ] **Step 1: Write the failing test** — extend `WorkspaceView.test.tsx`: with an in-memory store seeded with a task and NO cards, the tree shows the empty-state; after `store.putCard(<a completed sift_craap instance for that task>)`, the tree panel shows that card's node (title = the card name). This proves real-time growth (store change → re-derive → re-render). Keep existing WorkspaceView tests green.

- [ ] **Step 2: Run to verify it fails** — `pnpm --filter web exec vitest run src/workspace/WorkspaceView.test.tsx` → FAIL (tree not wired).

- [ ] **Step 3: Implement** — in `WorkspaceView.tsx`, compute `const treeNodes = deriveProcessTree({ task: store.getTask(taskId)!, cards: store.listCards(taskId), registry: CARD_REGISTRY })` from the `useStore` snapshot in render, and pass `nodes={treeNodes}` to `<TreePanel ... />`. (The component already calls `useStore`; derive in render so a store write re-renders with a fresh tree.) No new state.

- [ ] **Step 4: Run test + typecheck** — `pnpm --filter web exec vitest run src/workspace/WorkspaceView.test.tsx` PASS; full web suite green (no regression); `pnpm --filter web typecheck` clean.
- [ ] **Step 5: Commit** — `git commit -am "feat(workspace): wire live process tree into WorkspaceView"`

---

### Final verification

```bash
pnpm -r typecheck
pnpm -r test
```
Both green. Optional manual check (工作区 dev tab): open/fill a card → its node appears in the right panel in real time; skip a card → a skipped node still appears.

## Self-Review

**Spec coverage:** node model + pure `deriveProcessTree` with the 4 deterministic types, skip-emits-node, time-order, unknown-card fallback (Task 1) ✓; `nodeView` type→style mapping + TreePanel body rendering verbatim from the HTML + empty-state (Task 2) ✓; WorkspaceView wiring + real-time growth integration test (Task 3) ✓. Semantic types (sub_question/key_knowledge/attempt) explicitly deferred to S5 — node model carries them for forward-compat, derivation doesn't emit them.

**Placeholder scan:** Task 1 ships complete code. Tasks 2–3 specify exact behavior + tests and instruct lifting the node-row markup verbatim from the binding HTML (the project's UI source-of-truth rule, as in S1/S3b) rather than re-printing the styled JSX here. No "TBD"/"handle later" in any logic step.

**Type consistency:** `ProcessNode`/`ProcessNodeType` (Task 1) are consumed unchanged by `nodeView` and `TreePanel` (Task 2) and by `WorkspaceView` (Task 3). `deriveProcessTree`'s signature matches its Task-3 call. `TreePanel`'s new `nodes` prop is the same `ProcessNode[]`.
