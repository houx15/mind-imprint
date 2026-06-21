# Slice 3a — 工具卡全库导入 (Card Library Import) · Design

> 日期 2026-06-21 ｜ 扩展 B0（契约）+ B1（渲染器，仅 Harness 选卡器，渲染器本体不动）｜ 前置 B0/B1（已并入 main）。
> 背景：用户决定本期**使用全部 31 张卡**（覆盖 AGENTS.md「本期只做 2 张样例卡」），把 roadmap 的 S3 拆为 **3a（本 slice，卡数据/导入）→ 3b（决策层 + summon_card 循环 + 工作区）→ 3c（富交互卡：光谱/角色博弈/画布/拖拽标注的真实交互，later）**。
> 卡设计真相源：`docs/工具包库/`（31 张 `.md` + `README.md`，含 frontmatter 规范与每卡「渲染要点」）。

---

## 1. Goal

把 `docs/工具包库/` 的 **31 张库卡**全部变成可被 registry 加载的 JSON 卡 spec，并给 `CardSpec` 补上库 frontmatter 的**路由元数据**，让 3b 的决策层能在全库上按「分类 / tier / priority / 触发词」选卡。**不动渲染器**：能用现有 7 原语表达的卡给真实 body；需要全新交互的卡先**注册 + 占位 body**（真实交互留给 3c）。

**卡集决定（2026-06-21，用户拍板）：保留现有 2 张 demo 卡 + 导入全部 31 张库卡 = registry 共 33 张。** 现有 `sift_craap`（SIFT×CRAAP 合卡，对应 PRD §14 单卡流程）与 `concession`（让步段）是 demo 定制卡，**保留原 id**（S1 fixtures/测试、3b demo 主动脉都按这俩 id 引用）；库里的 `sift`(05)/`craap`(04)/`steelman`(12) 仍各自独立导入。由此产生 2 对近义卡（SIFT 系 / 让步段系）——其在 3b 决策层的取舍由 tier/trigger 元数据偏置（见 §8 carry-forward），**非本 slice 处理**。

**不含：** 决策层 / summon_card 循环 / 工作区（→ 3b）；任何新字段原语或富交互组件（→ 3c）；评估（→ S5）；外壳（→ S6）。

---

## 2. `CardSpec` 扩展（B0，增量 + 可选 → 既有 2 卡仍校验通过，随后回填）

`packages/contracts/src/cardSpec.ts` 在现有字段（`id/category/name/purpose/trigger_condition/steps/rubric_tags`）基础上**追加可选字段**，全部来自库 frontmatter（`README.md` §frontmatter 规范）：

```ts
export const Priority = z.enum(["P0", "P1", "P2"]);
export const DisclosureTier = z.enum(["tier-0", "tier-1", "tier-2"]);
export const InteractionType = z.enum([
  "步骤引导卡", "选择追问卡", "分类标注卡", "量表光谱卡",
  "角色模拟卡", "画布导图卡", "回放验证卡", "报告生成卡",
]);
export const BodyStatus = z.enum(["full", "stub"]);

export const CardSpec = z.object({
  // —— 既有（不变）——
  id: z.string().min(1),
  category: z.string().min(1),
  name: z.string().min(1),
  purpose: z.string(),
  trigger_condition: z.string(),     // 语义触发情境 ←库 frontmatter 的 trigger_context
  steps: z.array(Step).min(1),
  rubric_tags: z.array(z.string()),
  // —— 新增（可选，库 frontmatter）——
  name_en: z.string().optional(),
  priority: Priority.optional(),
  disclosure_tier: DisclosureTier.optional(),
  age_band: z.array(z.string()).optional(),        // 例 ["MYP","DP"]
  trigger_keywords: z.array(z.string()).optional(),// 词法命中信号
  interaction_type: InteractionType.optional(),
  rubric_dims: z.array(z.string()).optional(),     // 短码 D2/D3…（评估/路由用）
  related: z.array(z.string()).optional(),         // 关联卡 id
  body_status: BodyStatus.optional(),              // 缺省视为 "full"；"stub"=富交互待 3c
});
```

**字段对账（避免漂移）：**
- 库 frontmatter 的 `trigger_context`（语义情境）→ 落到既有 `trigger_condition`（不新建第二个语义字段）；`trigger_keywords`（词法）单列。
- 既有 `rubric_tags`（如 `"D1_来源意识"`，信封/渲染用）**保留**；新增 `rubric_dims`（短码 `D2`，来自 frontmatter，评估/路由用）。两者并存、各司其职。
- 既有 2 卡（`sift_craap`/`concession`）**回填**这些新元数据（priority/tier/keywords/…），并显式 `body_status:"full"`。

> 这些字段是**可选**的，所以扩展后既有卡先不报错；但导入完成时全部 31 卡都应带齐核心路由元数据（priority/disclosure_tier/trigger_keywords/interaction_type），由测试与 review 把关。

---

## 3. 31 张库卡 JSON（+ 保留 2 张 demo 卡 = 33）

每张**库卡**一份 JSON，放 `packages/contracts/cards/`（与现有两卡同目录），`id` = kebab-case（取自 frontmatter `id`，如 `belief-spectrum`/`sift`/`craap`/`steelman`），文件名 `<id>.json`。来源 = 该卡 `.md` 的 **frontmatter** + 正文「**渲染要点（卡片字段）**」节 + 「如何交互/分步脚本」节。现有 `sift_craap.json`/`concession.json` **保留不动**（仅在 §2 回填元数据），故 registry 最终 = 31 库卡 + 2 demo 卡 = **33**。

按**交互可表达性**分两类（**按卡判定，不机械按 interaction_type**）：

**A. Form-expressible（`body_status:"full"`）——能用现有 7 原语（`text`/`textarea`/`single_choice`/`multi_choice`/`rating`/`repeatable_group`/`link_check`）表达：**
- 多数 `步骤引导卡`/`选择追问卡`/`回放验证卡`/`报告生成卡`。
- 真实 `steps`（含 `disclose: always|on_demand` 渐进披露）+ `fields`，照「渲染要点」拼。
- 例：`sift_craap`（已存在，保留）、`论证地图卡`（结构填空 + 谬误 single_choice）、`事实/观点/价值判断卡`（分类 single_choice + 理由 textarea）、`PEE 写作卡`（三段 textarea）、`OPCVL 史料评估卡`（六维 textarea/rating）、`AI 边界与幻觉核查卡`（核查清单 multi_choice + 溯源 link_check）等。

**B. Bespoke（`body_status:"stub"`）——需全新交互，本 slice 仅占位：**
- 典型 `interaction_type ∈ {量表光谱卡, 角色模拟卡, 画布导图卡, 分类标注卡}`：`立场光谱卡`、`确定度光谱卡`、`伦理情景·角色博弈卡`、`3D 溯源导图卡`、（视判定）`论证地图卡`若需画布等。
- 占位 body = **一个 step、一个 `textarea`**（`disclose:"always"`，label 形如「先用文字记录你的思考；这张卡的完整交互即将上线」），`methodology_note` 放该卡一句话定位。这样**渲染器不变**也能打开/提交、产出合法信封；`interaction_type` 标明 3c 要补的富交互类型。
- frontmatter 元数据（trigger/tier/priority/rubric_dims/related）**照常写全** —— 决策层照样能在全库里提议它。

> 判定红线：**绝不为了"表达"而临时加原语**（那是 3c 的事）。拿不准能否用 7 原语忠实表达 → 归 B（stub）。

---

## 4. registry + catalog 投影

- `loadRegistry`：现有逻辑（按 key 校验、`id` 必须等于 key、fail-loud）不变，`DEFAULT_RAW` 从 2 条扩到 **31 条**（31 个 JSON import）。
- `deriveCatalog`：扩展投影出路由元数据，供 3b 决策层：
  ```ts
  CatalogEntry = { id, category, name, trigger_condition,
                   trigger_keywords?, disclosure_tier?, priority?, interaction_type? }
  ```
  （仍是 registry 的纯投影，不手写第二份。）
- **不改渲染器代码**（schema 驱动验收：新增 31 卡 = 新增 31 份 JSON）。

---

## 5. Harness 选卡器

扩展现有 dev 卡 Harness（`apps/web/src/cards/dev/`）：加一个**卡选择器**（下拉/列表，覆盖全部 31 卡，按 category 分组），选中即用现有 `<CardRenderer>` 渲染：
- `full` 卡 → 完整渲染三态、可填可提交。
- `stub` 卡 → 渲染占位 textarea body，并显示一个小标记（如 `占位 · 富交互待上线 · {interaction_type}`）。
- 用真实卡内容，便于逐卡肉眼校验导入质量。

---

## 6. 测试（TDD）

- **逐卡校验**：33 个 JSON 全部 `CardSpec.safeParse` 通过；`card.id === key`（沿用 registry 既有不变式，参数化跑全部卡）。
- **catalog**：`deriveCatalog` 产出 33 条，且每条带核心路由元数据（id/category/name/trigger_condition 必有；提议用的 trigger_keywords/disclosure_tier/priority 对全 33 卡应存在）。
- **元数据完整性**：全 33 卡都带 `priority`/`disclosure_tier`/`interaction_type`/`trigger_keywords`；`body_status` 仅 `full|stub`。
- **stub 卡**：标 `body_status:"stub"` 的卡，其 body 是单 step 单 `textarea`，经渲染器能产出合法 `CardInstance`（复用 S1 envelope 测试套路）。
- **既有卡不回归**：`sift_craap`/`concession` 仍校验通过、Harness 仍渲染；回填的元数据不破坏既有字段。
- **Harness**：选卡器列出 31 卡；切换能渲染 full 与 stub 两类（RTL）。

---

## 7. Global constraints（贯穿每个 task）

- 验收门槛：`pnpm -r typecheck` + `pnpm -r test` 全绿；类型安全只由 `tsc --noEmit` 保证（每个改 contracts/web 的 task 各自跑 typecheck）。
- **卡 spec 单一真相源在 registry**；catalog 从它派生，不手写第二份。
- **新增卡 = 新增 JSON，不改渲染器/原语代码**（schema 驱动的验收标准）。本 slice **零新原语、零渲染器逻辑改动**；需要新交互的卡一律 stub，留 3c。
- `CardSpec` 扩展**增量且可选**，不破坏既有 `CardInstance`（冻结脊椎）与既有 2 卡。
- 卡内容忠实于 `docs/工具包库/` 的设计（frontmatter + 渲染要点 + 分步脚本），中文，真实素材，不用 lorem。
- 四条铁律照旧（AI 克制由 3b 的 prompt 承载；本 slice 只产数据/契约）。

---

## 8. Out of scope / carry-forward

- **3b（下一个 slice）**：决策层用本 slice 的 catalog 元数据做「先分类 → 按 tier/priority/trigger 选卡」路由（README §渐进式披露 A：tier-0 常驻、tier-1/2 浮现、priority 取一主推其余折叠）；summon_card 循环 + 工作区。开 3b 前先据此**修订已存在的 3b 设计稿**（`2026-06-21-slice3-decision-summon-workspace-design.md`）的决策层/目录章节。
- **3b 近义卡偏置**：因保留了 demo 卡（sift_craap/concession）与库卡（sift/craap/steelman）并存，3b 决策层须保证 Phoebe 主动脉里 demo 卡胜出——手段：给库里 `sift`/`craap`/`steelman` 设较低优先级或 tier-2，或在 demo 任务上下文里偏置 trigger 匹配。本 slice 已把所需元数据落到每卡，具体偏置策略在 3b 定。
- **3c（later）**：为 stub 卡建真实富交互（量表光谱/角色模拟/画布导图/分类标注），含其分步脚本、就地小讲解、必要的新原语或专用组件；届时把对应卡 `body_status` 由 `stub` 升为 `full`。
- **评估（S5）**：可消费 `rubric_dims`；本 slice 已把该元数据落到每卡。
- **roadmap / 记忆更新**：S3 现拆为 3a/3b/3c，需同步 `docs/架构分解_Roadmap.md` 与 slice-progress 记忆（本 slice 完成时一并更新）。
