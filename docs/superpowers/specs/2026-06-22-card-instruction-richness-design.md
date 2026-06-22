# 工具卡分步教学 (Card Instruction Richness) · 设计文档

> Layer B of the three-layer card-architecture improvement.
> Layer A (selection signal) shipped. Layer C (escape-hatch JSX, PoC = belief-spectrum) is the next/final layer.

**日期：** 2026-06-22
**分支：** `feat/card-instruction-richness`

## 背景与问题

学生打开一张卡时看到的「指导」只有一行扁平文本，而且大部分根本不显示：

- `Step.methodology_note` 是单个字符串（`packages/contracts/src/cardSpec.ts:8`）。
- `CardRenderer`（`apps/web/src/cards/CardRenderer.tsx:35-38`）**只渲染 `step.title` + 字段，从不渲染 `methodology_note`**。
- 两个卡面壳（`ActiveSheet.tsx:18`、`CardSheetHost.tsx:18`）只把 **`steps[0].methodology_note`** 放进一个折叠的「这个工具怎么用」面板。
- 结果：像 SIFT（4 步）这种卡，只有第一步 Stop 的 note 可能被看到；Investigate / Find / Trace 三步的 note 是**已写好但永远不显示的死数据**。全库共 **101 条 step note**。

两个问题：指导**大多不显示**，且**无结构、无富文本**（不能高亮重点、不能分「为什么/怎么做」）。

## 目标

把每一步的教学内容升级为**结构化 + Markdown 富文本**，并在**每一步**就地显示（而不是只有第一步、只在全局面板里）。

**用户已定方向：**
- 全库结构化重写（33 张卡 / 101 步）。
- 内容为 Markdown（支持 `**高亮**`、列表等）。
- 四个内容维度：**why / how / when / example**（example 可选）。

## 设计

### 1. Schema (`packages/contracts/src/cardSpec.ts`)

把 `methodology_note: z.string()` 替换为结构化对象：

```ts
export const Methodology = z.object({
  why:  z.string().min(1),        // 为什么这一步重要 / 它在防什么
  how:  z.string().min(1),        // 具体怎么做
  when: z.string().min(1),        // 什么时候用这一步（适用情形）
  example: z.string().optional(), // 一个具体例子（点睛，可选）
});

export const Step = z.object({
  key: z.string().min(1),
  title: z.string().min(1),
  disclose: z.enum(["always", "on_demand"]),
  methodology: Methodology,       // was: methodology_note: z.string()
  fields: z.array(FieldPrimitive).min(1),
});
```

四个字段都是 **Markdown 字符串**。`why/how/when` 必填，`example` 可选。

### 2. 渲染 (`apps/web/src/cards/CardRenderer.tsx`)

每一步（无论 `always` 还是 `on_demand`）在 `title` 下、字段上方，渲染一个**可折叠的「方法」面板**：

- 折叠态：一个「方法」触发条（带 info 图标）。
- 展开态：四个带标签的小块——**为什么 · 怎么做 · 什么时候用 · 例子**（example 缺省则不渲染该块），每块内容用现有的 `Markdown` 组件（`apps/web/src/workspace/Markdown.tsx`）渲染，支持高亮 / 列表。
- 展开时 emit `onExpandNote(step.key)` → 走 envelopeReducer 的 `note_open`（信号保留，且从「只有第一步」泛化到**每一步**）。

`CardRenderer` 当前的 props 已有 `onExpandStep`（用于 `on_demand` 步展开 → `step_expand`）。新增一个 `onNote(step_key)` 回调用于 methodology 展开 → `note_open`。

### 3. 卡面壳 (`ActiveSheet.tsx` / `CardSheetHost.tsx`)

- 移除 header 里的全局「这个工具怎么用」按钮与 `steps[0].methodology_note` 面板（每一步就地显示已取代它）。
- 保留 header 的 `purpose` 行。
- 把新的 `onNote` 透传给 `CardRenderer`，连到 `note_open` envelope 事件。
- `firstStepKey` / `handleNoteOpen` 相关逻辑随之调整为按 step_key 派发。

### 4. 标准信封

**不改 envelope schema。** `note_open` 事件（`packages/contracts/src/envelope.ts:6`，`{ kind, step_key, at }`）已支持任意 step_key——本设计只是让它在每一步都能触发。过程树 / 评估的地基不动。

### 5. 内容迁移（101 步）

逐卡把现有 `methodology_note` 拆/扩写为 `{why, how, when, example?}`：
- **保真**：以现有 note 为底，拆出 why（原理/防什么）与 how（动作），voice 与现有一致。
- **when**：补「什么时候该用这一步」。
- **example**：仅在能点睛时加。
- Markdown：关键词 `**加粗**`，需要时用列表。
- 按 category 分批，由 subagent 并行起草、主控审校 voice 一致性。

## 测试策略 (TDD)

- **schema**（`cardSpec.test.ts`）：`Methodology` 必填 why/how/when、example 可选；旧 `methodology_note` 形态被拒。
- **renderer**（新增 `CardRenderer` 测试）：含 methodology 的步渲染「方法」面板，展开后出现 why/how/when 文本与（有则）example；展开触发 `onNote(step_key)`；Markdown 高亮（`**x**` → `<strong>`）真实渲染。
- **envelope**：展开第 N 步方法 → envelope 出现对应 `note_open` step_key（信号保留）。
- **registry**：33 张卡全部通过新 schema（迁移完成后）。
- **门禁**：`pnpm -r typecheck && pnpm -r test` 全绿。

## 分阶段实施 (每阶段保持绿)

- **P1**：schema 加 `methodology`（**可选**，与 `methodology_note` 并存）；`CardRenderer` 渲染 methodology 面板 + Markdown + `note_open` 信号；壳层透传。Additive，不破坏现有卡。
- **P2**：迁移全部 33 张卡 / 101 步加上 `methodology`；加 guard 测试断言每步都有 `methodology`。
- **P3**：`methodology` 改为**必填**，从 schema + 两个壳层 + 测试里删除 `methodology_note`；移除旧全局 note 面板。

## 风险

- **101 步 authoring 量大 / voice 漂移**：分批 + 主控审校；以现有 note 为底，保真优先。
- **壳层有两份（ActiveSheet / CardSheetHost）**：两处都要改；测试覆盖生产壳（CardSheetHost）。
- **note_open 语义变化**（从全局→每步）：可能影响依赖该事件的评估/过程树测试——P1 跑全测确认。
