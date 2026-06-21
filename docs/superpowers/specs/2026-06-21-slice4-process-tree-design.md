# Slice 4 — 过程树（确定性实时派生）(B5) · Design

> 日期 2026-06-21 ｜ Block B5 ｜ 前置 B0、B2.5（store）、B4（工作区 + TreePanel 骨架，已并入 main）。
> 双真相源：UI 以 `docs/design/思维印记_工作区.dc.html`（`treeOpen` 内 `sc-for list={{tree}}` 节点块）为准（逐像素）；非视觉以 PRD §10（过程树）为准。
> 用户决定（2026-06-21）：S4 = **确定性实时树**（纯函数，无 LLM、无新存储）；语义归并（sub_question 分枝 / key_knowledge / attempt / 打型）= **S5 评估那一刀**。见 `docs/遗留项追踪_Carryforward.md`。

---

## 1. Goal

把 store 的 `task` + `card_instance`(+`message`) **纯确定性**派生成过程树节点，填进 B4 已建的右侧 `TreePanel` 体内，**只读**、随学生用卡**实时生长**（"边做边长"）。无 LLM、无新存储（PRD §9：`process_node` 可派生）。

**不含：** 语义层（sub_question 分枝、key_knowledge 提取、attempt 草稿、归并打型）——这些由 **S5 评估 LLM** 顺手产出，走同一节点模型 / 同一面板渲染。

---

## 2. 节点模型（支持全 7 型，S4 只确定性发出其中 4 型）

PRD §10 的 7 型：`task_root` / `sub_question` / `card_use` / `attempt` / `key_knowledge` / `concession` / `reflection`。

```ts
// apps/web/src/workspace/processTree.ts
export type ProcessNodeType =
  | "task_root" | "sub_question" | "card_use"
  | "attempt" | "key_knowledge" | "concession" | "reflection";

export interface ProcessNode {
  id: string;                 // 稳定 key：task_root 用 task.id；卡节点用 card_instance.id
  type: ProcessNodeType;
  parent_id: string | null;   // S4：卡节点的 parent = task_root.id（扁平）；S5 可插 sub_question 重挂
  title: string;              // 主标题（任务标题 / 卡名）
  sub?: string;               // 次行（状态/摘要），对应 HTML 的 n.sub / n.hasSub
  ref_id?: string;            // 关联实体 id（card_instance.id）
  card_id?: string;           // 用了哪张卡
  status?: CardStatus;        // proposed/active/completed/skipped（卡节点）
  at?: string;                // 时间（created_at），用于排序
}
```

> S4 只发 `task_root` + 由卡派生的 `card_use` / `concession` / `reflection`。`sub_question` / `key_knowledge` / `attempt` 留给 S5（同模型追加 / 重挂）。

---

## 3. 派生（纯函数）

```ts
export function deriveProcessTree(opts: {
  task: Task;
  cards: CardInstance[];            // 该任务的全部 card_instance
  registry: Record<string, CardSpec>;
}): ProcessNode[];
```

规则（确定性、无语义）：
- 第一个节点 = **`task_root`**：`{ id: task.id, type:"task_root", parent_id:null, title: task.title, sub: task.seed ?? undefined, at: task.created_at }`。
- 对每个 `card_instance`（按 `created_at` 升序），产出一个节点，`parent_id = task.id`：
  - **类型按卡元数据判定**（查 `registry[card_id]`，无 LLM）：
    - `card_id === "concession"`（或 spec 是让步段族）→ `type:"concession"`
    - spec.`category` 含「反身性」→ `type:"reflection"`
    - 否则 → `type:"card_use"`
  - `title` = 卡名（`spec.name`，spec 缺失则回退 `card_id`）。
  - `sub` = 状态摘要：`skipped`→「已跳过（已记录为信号）」；`completed`→ 一句填写摘要（取首个非空字段值，截断）或「已完成」；`proposed`/`active`→「进行中」。
  - `status`/`card_id`/`ref_id`/`at` 照填。
- 排序：`task_root` 在首，卡节点按 `at` 升序（实时追加 = 末尾增长）。
- **跳过也出节点**（铁律：过程即数据）。
- spec 缺失（理论不该，registry 兜底）：仍出 `card_use` 节点，`title=card_id`，不抛错。

> 纯函数：不读 store、不写存储、不调 LLM。输入即 store 当前快照里的 task+cards。

---

## 4. 渲染（填 B4 的 TreePanel 体）

改 `apps/web/src/workspace/TreePanel.tsx`：体内从「仅空态」改为渲染 `ProcessNode[]`，逐像素照 HTML 的 `sc-for list={{tree}}` 块（marker 列 + 连接线 + `tagStyle`/`tag` + `title` + `hasSub`/`sub`）。
- 一个纯映射 `nodeView(node): { rowStyle, markerStyle, tagStyle, tag, title, hasSub, sub }`——`tag` 与配色**按 type**（`task_root`/`card_use`/`concession`/`reflection`…各有标签与 marker 色），缩进按是否有 `parent_id`（root 顶格，子节点缩进）。lift 自 HTML，不自创。
- 保留底部「边做边长 · 随评估归并枝节」。
- **空态**：只有 `task_root`（还没用过卡）时，显示既有空态提示；有卡节点则渲染树。
- 数据来源：`WorkspaceView` 用 `useStore` 取快照 → `deriveProcessTree(...)` → 传给 `TreePanel`。store 一变即重派生 → **实时生长**。`treeOpen`/`treeClosed` 折叠沿用 B4。

---

## 5. 接线

`WorkspaceView.tsx`：把 `deriveProcessTree({ task: store.getTask(taskId), cards: store.listCards(taskId), registry: CARD_REGISTRY })` 的结果传入 `TreePanel`。派生在 render 里算（输入是 `useStore` 快照里的稳定数组），不另起状态。无新 store 字段、无新契约。

---

## 6. 测试（TDD）

- **`deriveProcessTree` 纯单测**（核心）：
  - 仅 task → 单个 `task_root`（title/sub/at 正确）。
  - + 一张 completed `sift_craap` → root + 一个 `card_use` 节点（title=卡名、status=completed、sub 为填写摘要、parent=root、at 正确）。
  - concession 卡 → `concession` 型节点；反身性卡（如 `checkpoint`/`metacognition`）→ `reflection` 型。
  - skipped 卡 → 节点 status=skipped、sub=「已跳过…」（仍出节点）。
  - 多卡按 `at` 升序；root 恒在首。
  - spec 缺失的 card_id → 仍出 `card_use`，title=card_id，不抛。
- **TreePanel RTL**：给定 nodes 渲染 root + 卡节点（标签/标题可见）；只有 root 时显示空态；`nodeView` 对各 type 给出不同 tag。
- **实时生长（RTL/集成）**：用内存 store + WorkspaceView，`putCard` 一张完成卡后，树面板出现该卡节点（store 变 → 重派生 → 重渲染）。

---

## 7. Global constraints

- 验收门槛：`pnpm -r typecheck` + `pnpm -r test` 全绿；类型安全只由 `tsc --noEmit` 保证（每个改 web 的 task 跑 typecheck）。
- **纯派生**：`deriveProcessTree` 无副作用、无 LLM、无新存储；过程树是 `task`+`card_instance` 的投影（PRD §9「process_node 可派生」）。
- **只读**：右侧面板不可编辑（不是编辑器）。
- 冻结契约不动（`CardInstance`/`Message`/`Task` 形状不变）；不加新原语、不改卡渲染器。
- UI 逐像素照 `docs/design/思维印记_工作区.dc.html` 的树节点块；`mk-*` token；真实 Phoebe 内容（任务根「中国是否让地球变得更可持续？」、SIFT/让步段卡节点），不用 lorem。
- 铁律：过程即数据（跳过也出节点）；只读不替学生定论。

---

## 8. Out of scope / carry-forward

- **S5（评估）**：语义树——`sub_question` 分枝（把卡节点重挂到子问题下）、`key_knowledge`（撞上的关键知识，如 NASA / Nature Sustainability）、`attempt`（草稿尝试）、归并打型。eval LLM 顺手产出，复用本 slice 的 `ProcessNode` 模型 + `TreePanel` 渲染。评估增强树**是否落库**在 S5 定（当前 S4 纯重派生）。
- **S6**：把工作区接进真实外壳后，过程树面板随任务切换。
- 记录页的"卡使用计数"（PRD §13.4）也从 `card_instance` 聚合 → S6，与本派生同源。
