# Slice 1 设计规格 · 契约骨架 + 卡运行时 (B0 + B1)

> 版本 v0.1 ｜ 2026-06-20 ｜ 状态：待用户评审
> 上游：`docs/架构分解_Roadmap.md`（S1 = B0+B1）、`docs/思维印记_Demo_PRD.md`（§5–§10）、`docs/design/思维印记_工作区.dc.html`（**UI 真相源**）
> 下游：本规格通过后 → writing-plans 出实现计划 → TDD 实现

---

## 1. 目标与成功判据

**一句话：** 定下"不变的骨架"（卡契约 + 标准信封 + 数据模型），并立刻用一个 schema 驱动的卡渲染器去**使用**它，从而校验骨架是对的。纯前端、假数据、无 LLM、无 DB。

**完成即满足 PRD §2 成功判据 #2**：工具卡是 schema 驱动渲染的真实交互组件（非文本气泡），填写过程被采集，最终序列化成统一的标准信封。

**可演示验收（dev harness 内）：**
1. SIFT×CRAAP 与让步段两张卡，**经同一个渲染器**、仅靠各自 JSON 渲染出来，外观对齐 HTML。
2. 三视觉态可走通：提议态（打开卡/暂不/已完成/已跳过）→ 激活态（底部抽屉填写）→ 完成态（紧凑卡）。
3. `disclose:"on_demand"` 的 CRAAP 步骤默认折叠，点开展开五维评分。
4. "这个工具怎么用"就地展开方法论（不跳走）。
5. 提交后产出一个通过 B0 schema 校验的标准信封，事件轨迹含时间戳。

---

## 2. 范围

**做（In）：**
- B0：字段原语 / 卡 spec / 标准信封 / 事件轨迹 的 Zod schema + 推断类型；registry 加载器 + 派生目录；两张卡 JSON；SQLite 物理 schema **文档化**。
- B1：7 个字段原语组件、`<CardRenderer>`、三视觉态组件、事件采集 reducer、方法论面板、dev harness、Tailwind 设计令牌（取自 HTML）。

**不做（Out，留给后续 slice）：**
- ❌ 真实对话 / LLM 网关 / `summon_card`（S2、S3）
- ❌ SQLite 实际落库与后端（S2）—— 本 slice 只产出内存信封对象
- ❌ 过程树渲染（S4）—— 完成态"钉到树"在本 slice 只渲染紧凑卡，不进真正的树面板
- ❌ 工作区容器 / 认证 / 主目录 / 记录 / 设置（S3、S6）
- ❌ 评估（S5）

---

## 3. 仓库结构（pnpm 工作区，已确认）

```
mind-imprint/
  pnpm-workspace.yaml
  package.json
  tsconfig.base.json
  packages/
    contracts/                    ← B0
      package.json
      src/
        primitives.ts             Zod：7 字段原语的判别联合
        cardSpec.ts               Zod：卡 spec / step
        envelope.ts               Zod：标准信封 + 事件轨迹
        rubric.ts                 rubric 维度常量
        registry.ts               加载器：JSON → 校验 → 类型化 spec + 派生目录
        index.ts
      cards/
        sift_craap.json
        concession.json
      sqlite-schema.sql           物理 schema（文档化，本 slice 不执行）
      test/                       Vitest 纯单测
  apps/
    web/                          ← B1
      package.json
      index.html
      vite.config.ts
      tailwind.config.ts          令牌取自 design HTML
      src/
        cards/
          CardRenderer.tsx        schema 驱动：steps→fields→组件
          fieldRegistry.ts        type → 组件 映射
          fields/
            TextField.tsx
            TextAreaField.tsx
            SingleChoiceField.tsx
            MultiChoiceField.tsx
            RatingField.tsx
            RepeatableGroupField.tsx
            LinkCheckField.tsx
          states/
            ProposalBubble.tsx    提议/完成/跳过 三变体
            ActiveSheet.tsx       激活态底部抽屉 + 方法论面板
            CompletedCard.tsx     完成态紧凑卡
          envelopeReducer.ts      (envelope, uiEvent) → envelope（纯）
        dev/
          harness.tsx             驱动两卡走三态（Phoebe 假数据）
        main.tsx
      test/                       Vitest + RTL
```

`apps/web` 通过工作区依赖 `@mind-imprint/contracts`，import 同一份 schema/类型——零漂移。

---

## 4. B0 契约细节

### 4.1 字段原语（判别联合，`type` 为判别键）

| type | 关键属性 | 渲染（对齐 HTML） |
|---|---|---|
| `text` | `key,label` | 单行输入 |
| `textarea` | `key,label,(rows?)` | 多行输入 + label + helper |
| `single_choice` | `key,label,options[]` | 按钮/pill 组（如 verdict 可信/存疑/不可信） |
| `multi_choice` | `key,label,options[]` | 复选组 |
| `rating` | `key,label,scale` | scale 格刻度（CRAAP 1–5） |
| `repeatable_group` | `key,label,item_fields[]` | 可加行的组（多源对照，"+ 添加来源"） |
| `link_check` | `key,label` | URL 输入 + 判定标签（"已溯源"） |

> 新增原语**仅当**出现一种全新交互才加（PRD §6.1）。本 slice 用上述 7 种覆盖两卡。

### 4.2 卡 spec

```
CardSpec = {
  id, category, name, purpose,
  trigger_condition,                       // 自然语言，决策层用
  steps: Step[],
  rubric_tags: string[]
}
Step = {
  key, title,
  disclose: "always" | "on_demand",
  methodology_note: string,
  fields: FieldPrimitive[]
}
```

### 4.3 标准信封（脊椎，PRD §9 —— 一次定对）

```
CardInstance = {
  id, card_id, task_id, parent_node_id,
  status: "proposed" | "active" | "completed" | "skipped",
  field_values: Record<string, unknown>,  // JSON：学生填的内容
  event_trace: TraceEvent[],               // JSON：见 4.4
  rubric_tags: string[],
  created_at: string,                      // ISO
  completed_at: string | null
}
```

### 4.4 事件轨迹（摩擦即信号，PRD §1 铁律 4）

```
TraceEvent =
  | { kind: "field_change", path: string, at: string }    // path 形如 "sift.stop" / "sources[1].verdict"
  | { kind: "step_expand",  step_key: string, at: string }
  | { kind: "note_open",    step_key: string, at: string } // "这个工具怎么用"
  | { kind: "skip",         at: string }
  | { kind: "submit",       at: string }
```

> 注：`field_change` 记录"哪个字段在何时变化"，不在轨迹里存全量值（值在 `field_values` 终态）；保持轨迹轻量、可读。

### 4.5 registry 加载器 + 派生目录

- `loadRegistry()`：读 `cards/*.json` → 逐张用 `CardSpec` schema 校验 → 返回 `Record<id, CardSpec>`。任一张不合法 → 抛出带 card id + 字段路径的清晰错误（这就是"单一真相源、防漂移"作为代码保证）。
- `deriveCatalog(registry)`：产出紧凑目录 `{ id, category, name, trigger_condition }[]`——未来决策层/`summon_card` enum 的投影源。本 slice 仅产出与测试，不接 LLM。

### 4.6 两张卡 JSON

逐字采用 PRD §6.3（`sift_craap`）与 §6.4（`concession`）的 spec，落为 `cards/sift_craap.json`、`cards/concession.json`。

### 4.7 SQLite 物理 schema（文档化，不执行）

`sqlite-schema.sql` 写出 `task / message / card_instance / process_node / evaluation` 五表 DDL（PRD §9），令标准信封"DB-ready"。本 slice 不建库、不连接；S2 执行。

---

## 5. B1 运行时细节

### 5.1 schema 驱动渲染（B1 的心脏 / PRD 验收标准）

- `fieldRegistry: Record<FieldType, Component>`。
- `<CardRenderer card={spec} envelope dispatch>`：遍历 `spec.steps`；`disclose:"on_demand"` 步骤包进折叠手风琴；每个 field 按 `field.type` 从 `fieldRegistry` 取组件，传 `field` spec + `value`（来自 `envelope.field_values`）+ `onChange`（→ dispatch `field_change`）。
- **硬约束**：渲染器内无任何 `if (card.id === ...)` 分支。两卡仅靠 JSON 区分 —— 作为测试断言（§7）。

### 5.2 三视觉态（取自 HTML）

- **ProposalBubble**：建议工具卡 chip + category pill + name + nudge；`proposed` → `打开卡`/`暂不，先继续`；`completed` → "已完成 · 已钉到过程树"；`skipped` → "已跳过（已记录为信号）" + "仍可打开"。
- **ActiveSheet**：底部抽屉，`#D98263` 顶条、scrim、`mkSheetUp` 动效；头部 "现在轮到你想" + category + name + purpose + "这个工具怎么用" 切换 + 关闭；体内渲染 `<CardRenderer>`；底部"提交"。
- **CompletedCard**：紧凑卡（完成态）。本 slice 渲染在 harness 中，不进真正过程树。

### 5.3 方法论面板

`showMethodology` 切换：就地展开当前卡/步骤的 `methodology_note`（不跳走）。打开记一条 `note_open` 事件。

### 5.4 事件采集 reducer（纯，最佳 TDD 目标）

`envelopeReducer(envelope, action) -> envelope`。action 多数与轨迹事件 1:1，外加一个不产生轨迹事件的 `activate`（PRD §9 列举的轨迹种类不含"打开"，故仅做状态转移）：
- `activate` → `status="active"`（提议→激活；不追加轨迹事件）；
- `field_change` → 写 `field_values` 对应路径 + 追加 `field_change`；
- `step_expand` / `note_open` → 仅追加对应事件；
- `skip` → `status="skipped"` + 追加 `skip`；
- `submit` → `status="completed"`、`completed_at=now` + 追加 `submit`。
- 时间戳由注入的 `now()`（默认 `() => new Date().toISOString()`）产生，便于测试确定性。

渲染器是该 reducer 之上的薄视图层，逻辑全部可脱离 DOM 单测。

### 5.5 Dev harness

`harness.tsx`：左侧选卡（SIFT×CRAAP / 让步段）+ 选态（提议/激活/完成），右侧渲染对应组件，用 HTML 里的 Phoebe 真实假数据（来源：公众号→NASA/Nature Sustainability IF 32.1；论点"中国很大程度让地球更可持续" vs 反例"中国碳排放全球第一"）。提交后把产出的标准信封 JSON 打印到面板，便于肉眼核对。

### 5.6 设计令牌（Tailwind，取自 HTML）

primary `#2A3B7A`(hover `#22305F`)、primary-tint `#EDEFF9`、accent `#D98263`(hover `#CC7355`)、accent-tint `#FBEEE7`、green `#4C9A82`/`#E7F3EE`、amber `#E8A33D`、text `#1C2333`、muted `#8A92A3`/`#9AA1B0`、bg `#F3F4F8`、surface `#FFF`、border `#EAECF2`/`#ECEEF3`、input `#E1E4ED`/bg `#FCFCFD`；radius 卡 14–18 / 输入 10–11 / pill 999 / sheet 22；字体 Plus Jakarta Sans + Noto Sans SC；柔和阴影。精确像素实现时从 HTML 现取。

---

## 6. 数据流

```
harness 选卡 → loadRegistry() 取 spec
  → 初始 envelope(status=proposed)
  → ProposalBubble（打开卡）→ dispatch(activate) → status=active
  → ActiveSheet 内 <CardRenderer spec envelope dispatch>
      字段交互 → dispatch(field_change/...) → envelopeReducer → 新 envelope → 重渲染
      "这个工具怎么用" → dispatch(note_open)
      CRAAP 展开 → dispatch(step_expand)
  → 提交 → dispatch(submit) → envelope(status=completed, completed_at)
  → CardSpec/CardInstance schema 校验通过 → harness 打印信封 JSON
跳过路径：ProposalBubble(暂不) → dispatch(skip) → status=skipped
```

---

## 7. 测试策略（TDD：先红后绿）

**B0 纯单测（contracts/test）：**
1. 信封 schema：接受合法信封；拒绝非法 `status`、缺 `card_id`、坏 `event_trace`。
2. 字段原语联合：各 type 校验自身属性；未知 `type` 被拒。
3. `loadRegistry`：两卡加载成功；故意损坏的卡 JSON 校验失败并报出 card id + 字段路径。
4. `deriveCatalog`：目录每卡一行 `trigger_condition`，无残留/陈旧项。

**B1 reducer 单测（web/test）：**
5. `field_change` 更新 `field_values[path]` 且追加事件（含注入时间戳）。
6. `step_expand` / `note_open` 仅追加对应事件，不动值。
7. `skip` → `status=skipped` + 事件；`submit` → `status=completed` + `completed_at` + 事件。

**B1 渲染 RTL（web/test）：**
8. 给 `sift_craap`：渲染 Stop `textarea`、Investigate `repeatable_group`（含"+ 添加来源"）、verdict `single_choice` pills、Find better `textarea`、Trace `link_check`；CRAAP 步骤默认折叠，展开后出现 5 个 `rating`。
9. 给 `concession`：渲染其 4 个字段（thesis `text`、counter/concede/rebut `textarea`）。
10. **schema 驱动断言**：同一 `<CardRenderer>` 无 card-specific 分支即渲染出 8、9 两卡（搜索源码无 `card.id ===` 之类分支）。
11. 三态：`proposed` 显示 打开卡/暂不；`completed` 显示 "已完成·已钉到过程树"；`skipped` 显示 "已跳过"。
12. 提交后产出的信封通过 `CardInstance` schema 校验。

---

## 8. 验收标准（Definition of Done）

- [ ] 所有测试（§7 的 1–12）绿。
- [ ] `pnpm --filter @mind-imprint/contracts test` 与 `pnpm --filter web test` 通过。
- [ ] dev harness 跑起来，两卡三态可手动走通，提交打印出合法信封 JSON。
- [ ] 渲染器源码不含任何 card-specific 分支（schema 驱动成立）。
- [ ] 视觉对齐 `docs/design/思维印记_工作区.dc.html`（底部抽屉、字段、三态）。
- [ ] `sqlite-schema.sql` 写出五表 DDL（不执行）。

---

## 9. 风险与边界

- **渲染保真 vs schema 驱动张力**：HTML 的 SIFT 抽屉很精致；必须让这种精致**从原语组件 + spec 组合中自然涌现**，而非给卡写专属布局。原语组件承载样式，spec 只描述结构。这是 B1 的核心难点，用测试 10 守住。
- **时间戳确定性**：reducer 注入 `now()`，测试传固定时钟。
- **让步段无 on_demand 步骤**：验证渲染器对"只有 always 步骤"的卡同样工作（无手风琴）。
- **本 slice 不落库**：信封仅内存对象；S2 接 SQLite 时复用同一 `CardInstance` 类型，不改结构。
