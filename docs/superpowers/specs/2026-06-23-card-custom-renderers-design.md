# 工具卡自定义渲染器 (Card Custom Renderers / Escape-hatch JSX) · 设计文档

> Layer C of the three-layer card-architecture improvement. Layers A (selection signal) and B (instruction richness) shipped.

**日期：** 2026-06-23
**分支：** `feat/card-custom-renderers`

## 背景与目标

到目前为止所有卡共用一个 schema 驱动渲染器（`CardRenderer` + 9 个字段原语）。有些卡的交互本质上「不是填表」——例如**立场光谱**：把各方观点与「自己」放到**同一条连续光谱轴**上、彼此相对地看，单选 radio 行做不到。

用户已定方向：**为单卡开「逃生舱」——允许某张卡用自定义 JSX 组件渲染，覆盖 schema 渲染器，其余卡仍走 schema。**

**硬护栏（不可违反）：** 自定义组件可以渲染任意 JSX，但**必须仍然产出标准信封**——它只是同一份 `field_values` 的另一种编辑器。过程树、评估、refeed 全部由 `CardSheetHost` 按原样生成，不因自定义渲染而改变。**自定义外观，标准产出。**

## 设计

### 1. 渲染器注册表 (`apps/web/src/cards/customRenderers.ts`)

```ts
export type CardBodyProps = {
  card: CardSpec;
  values: Record<string, unknown>;
  onField: (path: string, value: unknown) => void;
  onExpandStep: (stepKey: string) => void;
  onNote?: (stepKey: string) => void;
};
export const customRenderers: Record<string, ComponentType<CardBodyProps>> = {
  "belief-spectrum": BeliefSpectrumRenderer,
};
```

`CardBodyProps` 与 `CardRenderer` 的 props **完全一致**——自定义渲染器是 `CardRenderer` 的 drop-in 替换。默认表里只有 PoC 一项；其余 card_id 不命中 → 走 `CardRenderer`。

### 2. 壳层选择 (`CardSheetHost` 生产 / `ActiveSheet` dev)

```tsx
const Body = customRenderers[spec.id] ?? CardRenderer;
// ...
<Body card={spec} values={env.field_values} onField={handleField} onExpandStep={handleExpandStep} onNote={handleNote} />
```

壳层其余逻辑（envelope reducer、submit、note_open、过程树）**完全不变**。这是护栏成立的机制：自定义渲染器只能通过 `onField` 写入既有 `field_values`（键名严格对应 spec 的字段键），信封由壳层统一生成。

### 3. 共享 MethodologyPanel

自定义渲染器替换整个卡体，因此 Layer B 的「方法」面板要保留，需把 `MethodologyPanel` 从 `CardRenderer.tsx` **导出**，让自定义渲染器自行渲染（per-step，触发 `onNote`）。

### 4. PoC — `BeliefSpectrumRenderer` (`apps/web/src/cards/renderers/BeliefSpectrumRenderer.tsx`)

belief-spectrum 的 `fields`：`issue`(text) · `stances`(repeatable_group: who/position/believes/evidence/interest) · `self`(spectrum) · `self_reason`(textarea)。`spectrum` 值是 **stops 索引 (0..4)**（见 `SpectrumField`）。

自定义渲染：
- 复用现有字段组件渲染文本部分（`TextField`/`TextAreaField`）——保持一致、减少重写。
- **新交互：一条共享光谱轴**（5 个 stop 标签），所有 stance 的 thumb + 「我」的 thumb 都落在**同一条轴**上，可拖动；拖动**吸附到最近的 stop**（值仍是整数索引 0..4，数据模型不变 → 评估/refeed 不受影响）。也支持点击 stop 与键盘（与 `SpectrumField` 一致，无障碍）。
- stance 增删（repeatable）：通过 `onField("stances", nextArray)` 整体写回。
- `self` → `onField("self", index)`；`issue`/`self_reason` → 各自 `onField(key, value)`。
- 顶部渲染 `MethodologyPanel`（step「main」）。

「有意思」的核心：在**同一条轴**上同时看见各方与自己的相对位置，而不是一串各自独立的 radio 行。

## 测试策略 (TDD)

- **registry**：`customRenderers["belief-spectrum"]` 存在；任意其它 card_id 不在表内（壳层 fallback 到 `CardRenderer`）。
- **BeliefSpectrumRenderer**：点击/键盘把 `self` 写成正确的 stop 索引（`onField("self", n)`）；改 stance 位置写回 `stances` 数组且 `position` 为索引；渲染 issue/self_reason 文本字段；渲染 MethodologyPanel（展开触发 `onNote("main")`）。
- **护栏（关键）**：`CardSheetHost` 用 belief-spectrum + 自定义渲染器，操作后提交，`onSubmit` 的 `finalInstance.event_trace` 仍含标准 `field_change`/`submit`，`field_values` 键名与 spec 字段一致——信封与 schema 卡无差别。
- **门禁**：`pnpm -r typecheck && pnpm -r test` 全绿。

## 非目标

- 不改 envelope / eval / refeed / contracts。自定义渲染器纯属 `apps/web` UI 层。
- 不把 `spectrum` 值改成连续值（用户选定 snap-to-stops，数据模型不变）。
- 不为其它卡写自定义渲染器（只 PoC 一张，证明机制）。

## 风险

- **自定义渲染器漂移出护栏**（写了非 spec 字段键、或绕过 onField 自己攒信封）→ 评估读不到。缓解：`CardBodyProps` 只给 `onField`（不给 setEnv/envelope 访问）；护栏测试断言信封等价。
- **拖拽在 jsdom 难测**：交互同时支持点击/键盘，测试走点击；拖拽为渐进增强。
