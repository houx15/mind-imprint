# Slice 5 — 评估那一刀 + 「你的思维印记」(B6) · Design

> 日期 2026-06-21 ｜ Block B6 ｜ 前置 B0、B2、B2.5、B3/B4（均已并入 main）。
> 双真相源：UI 以 `docs/design/思维印记_工作区.dc.html`（`evalLoading` / `showEval` 块）为准（逐像素）；非视觉以 PRD §11/§12 + `docs/02_思维印记_产品骨架/思维印记_评估平台介绍.docx`（rubric 9 维、SOLO 四级、锚点样本）为准。
> 用户决定（2026-06-21）：① S5 = **rubric 评级 + 过程叙述 + 你的思维印记 modal + 手动触发**（护城河 / demo 第 7 步）；② **语义树归并推迟为 follow-on**（S4 扁平实时树已满足"可读的树"，归并风险更高，先做对评估）；③ 评估结果**落 B2.5 store**（增量字段）。

---

## 1. Goal

任务告一段落时，学生点一个「生成思维印记」按钮 → 旗舰 LLM（`evalModel`，无则回退 `model`）读**完整对话 + 全部标准信封（含 event_trace）** → 产出 ① demo rubric 子集各维的 **SOLO 四级（L1–L4）** 评级 + 一句维度 note；② 一段 **过程叙述**（哪里深、哪里跳过、关键知识时刻、成长点）→ 以 **「你的思维印记」modal** 呈现（仅学生可见）。结果落 store。**前端直连，BYO-key，旗舰不降级。**

评估引擎用 PRD/评估平台 doc 自己规定的 MVP：**不训练模型，大模型 + few-shot prompting**（5 份锚点样本作 prompt 示例）。

**不含：** 语义树归并（sub_question/key_knowledge/attempt + 重挂）→ follow-on；教师端/Dashboard/通知（PRD §11「只给学生看」）；异步增量评估（本期手动整体跑一次）。

---

## 2. Rubric（demo 子集，5 维）　👀 锚点待你审阅

权威 9 维（评估平台 doc）：D1 提问清晰度 / D2 信源辨识 / D3 横向验证 / D4 多视角与让步 / D5 论证拆解 / D6 反思与元认知 / D7 论证质量 / D8 信息再生产 / D9 AI 边界与伦理。SOLO 四级：**L1 萌芽 / L2 发展中 / L3 熟练 / L4 卓越**。

demo 两张卡（SIFT×CRAAP、让步段）命中的 **5 维**（PRD §11 子集）。`packages/contracts/src/rubric.ts`（取代 S1 占位）：

```ts
export const SoloLevel = z.enum(["L1", "L2", "L3", "L4"]);
export const SOLO_LABELS = { L1: "萌芽", L2: "发展中", L3: "熟练", L4: "卓越" } as const;
export interface RubricDimension { id: string; name: string; framework: string; anchors: { L1: string; L2: string; L3: string; L4: string } }
export const DEMO_RUBRIC: RubricDimension[]; // 下 5 维
```

**草拟锚点（据评估平台 doc 的 L1萌芽→L4卓越 + 框架锚点，待你逐条改写）：**

| 维度 | framework | L1 萌芽 | L2 发展中 | L3 熟练 | L4 卓越 |
|---|---|---|---|---|---|
| **D2 信源辨识** | 媒介/信息素养 · CRAAP | 完全信任 AI / 来源，从不追问出处 | 偶尔问「真的吗？」但不深入 | 主动要求论据，能识别来源等级 | 主动交叉验证，识别信源之间的利益关系与冲突 |
| **D3 横向验证** | ATL 研究 · 横向阅读 SHEG | 只看单一来源，不另开查证 | 想到要多看，但没真去找 | 主动多源对照，找到 2+ 独立来源 | 溯到原始出处，比较各源权威性与一致性 |
| **D4 多视角与让步** | QUEST-E · 论证评估 | 只站自己一方，无视反方 | 提到反方但轻描淡写 / 稻草人 | 主动找反方并正面回应 | 构建反方最强论证(steelman)后再让步反驳 |
| **D5 论证拆解** | QUEST-U · 论证分析 | 把观点当事实，不分论点论据 | 能复述但不辨结构 | 能识别论点-论据-假设结构 | 识别隐藏前提与论证谬误 |
| **D6 反思与元认知** | ATL 反思 · TOK 认知者与知识 | 不觉察自己被 AI 影响 | 事后偶尔回顾 | 主动校准信心，觉察思维盲点 | 觉察自己作为认知者的位置，迁移方法 |

> 注：D2 行直接来自 doc 原文；其余按同一光谱草拟。`rubric_tags`/`rubric_dims`（卡上的 D 码）与此对齐（demo 卡多用 D2/D3/D4/D5/D6）。

---

## 3. Evaluation 结果契约

`packages/contracts/src/evaluation.ts`（对应 PRD §9 `evaluation` 表）：

```ts
export const DimScore = z.object({ dim_id: z.string(), level: SoloLevel, note: z.string() });
export const Evaluation = z.object({
  task_id: z.string(),
  scores: z.array(DimScore),     // 每维一条
  narrative: z.string(),         // 过程叙述
  created_at: z.string(),
});
```

---

## 4. 评估引擎（`apps/web/src/agent/`）

### 4.1 输入装配
`assembleEvalInput({ store, taskId, registry }) → string`：把 ① 对话（`store.listMessages` 映射为 `角色：内容` 转写）+ ② 全部信封（`store.listCards` 每张：卡名 + status + 填写内容(用 `serializeCardForRefeed`) + event_trace 摘要——体现"哪里跳过/关键时刻"）拼成评估输入文本。PRD §11：输入 = 完整对话 + 全部标准信封（含 event_trace）。

### 4.2 Prompt　👀 few-shot 待你审阅
`buildEvalPrompt(rubric) → string`：评估系统 prompt = ① 定位（旗舰评估官，只给学生看，不替学生定论地"诊断"）+ ② rubric 维度定义 + SOLO 四级锚点（来自 §2）+ ③ **few-shot 锚点样本**（评估平台 doc：Phoebe L1→L4 / Marcus 被动 / Ethan 红线 / Eliza 跨模块——本 slice 先放 1 份 **Phoebe** 样例，含输入片段→期望 JSON 输出，作为格式与尺度锚）+ ④ 输出格式硬约束：**只输出 JSON** `{ scores:[{dim_id,level,note}], narrative }`，dim_id 限定为 rubric 维度 id。

> 草拟 Phoebe few-shot（真实素材，待你用 doc 的 co-design 样本替换/精修）：输入 = 学生想直接引用「中国让地球变绿」公众号文 → 用 SIFT 溯到 NASA / Nature Sustainability(IF 32.1) → 撞「碳排放全球第一」反例 → 写让步段。期望输出：D2 L4 / D3 L4 / D4 L4 / D5 L3 / D6 L3 + 一段叙述（来源意识从被动转主动；正面接住反例写让步段=L4 对立观点处理；下一步可问各来源立场）。

### 4.3 Runner
`runEvaluation({ store, chat, config, registry, taskId, now? }) → Promise<Evaluation>`：
- `system = buildEvalPrompt(DEMO_RUBRIC)`；`user = assembleEvalInput(...)`。
- `chat({ ...config, model: config.evalModel ?? config.model }, { messages:[{system},{user}], maxTokens })`。旗舰不降级。
- 解析：`JSON.parse` → `Evaluation`(去掉 task_id/created_at 的部分) zod-parse；**malformed 重试一次**（再失败抛 LlmError-ish / 由控制器置 error）。补 `task_id`/`created_at`。
- `store.putEvaluation(evaluation)`，返回。

### 4.4 状态 / 控制器
小控制器 `createEvaluator({ store, chat, config, registry, taskId })`（仿 createConversation 反应式）：`getSnapshot()/subscribe()` → `{ phase: "idle"|"running"|"done"|"error", evaluation?, error? }`；`run(): Promise<void>`（置 running → runEvaluation → done/error，不抛）。`useEvaluator(ev)` hook。

### 4.5 store 增量（B2.5）
`StoreState` 加 `evaluations: z.array(Evaluation)`（增量、可选默认 `[]`，向后兼容旧 blob）；`putEvaluation(e)`（append）、`listEvaluations(task_id)`、`getLatestEvaluation(task_id)`。冻结信封 `CardInstance` 不动。

---

## 5. UI（`apps/web/src/workspace/`）— 逐像素照 HTML

- **`EvalLoading`**（`evalLoading`）：scrim + 三点动画 +「旗舰模型正在评估这一程的思维过程……」。
- **`EvalModal`**（`showEval`「你的思维印记」）：深蓝渐变头（`过程小结 · 仅你可见` / `你的思维印记` / SOLO 四级说明）+ `evalDims` 列表（每维：`dim` 名 + `levelLabel`（如 `L4 · 卓越`）+ 4 段 `segs`（按 level 点亮 1–4 段）+ `note`）+ `过程叙述` 卡 + `回到任务` 关闭。把 `Evaluation` 映射成 `evalDims` 视图（segs：level L_n → 前 n 段高亮）。
- **触发**：`WorkspaceView` 面包屑加一个「生成思维印记」按钮（任务进行中可点）→ `evaluator.run()` → running 显示 `EvalLoading` → done 显示 `EvalModal`。
- 接线：`WorkspaceView` 持有 `createEvaluator`，`useEvaluator` 驱动三态。

---

## 6. 测试（TDD）

- **契约**：`DEMO_RUBRIC` 5 维各有 L1–L4 锚点 + id/name/framework；`Evaluation`/`DimScore` accept/reject；`SoloLevel` enum。
- **assembleEvalInput**：含对话转写 + 每张卡的名/状态/填写内容 + 跳过卡的 skip 信号。
- **buildEvalPrompt**：含 5 维定义 + SOLO 锚点 + Phoebe few-shot + 「只输出 JSON」约束。
- **runEvaluation**（对 fake chat）：脚本化合法 JSON → 解析成 `Evaluation`（scores 5 维 + narrative）、`putEvaluation` 落库、用 `evalModel`；malformed→重试一次→第二次合法则成功；两次都坏→error。
- **createEvaluator**：run→running→done(evaluation)；chat throw→error，不抛。
- **store**：`evaluations` 增量；putEvaluation/getLatestEvaluation；旧 blob（无 evaluations）仍加载（向后兼容）。
- **EvalModal/EvalLoading**（RTL）：渲染各维 levelLabel + segs 点亮数 + narrative + 关闭；loading 文案。
- **触发集成**：点「生成思维印记」→ fake evaluator → modal 出现，维度/叙述可见。

---

## 7. Global constraints

- 验收门槛：`pnpm -r typecheck` + `pnpm -r test` 全绿；类型只由 `tsc --noEmit` 保证（每个 task 各自 typecheck）。
- 前端直连、BYO-key、**旗舰不降级**（`evalModel ?? model`）；key 只在 header，绝不进 raw/error/日志/渲染（沿用 S2 边界）。
- 评估 prompt = rubric 定义 + SOLO 锚点 + few-shot（doc 的 MVP 方案）；输出严格 JSON，zod-parse + 重试一次。
- 冻结契约不动（`CardInstance`/`Message`/`Task`）；`StoreState` 增量加 `evaluations`（向后兼容）。
- 铁律：评估**只给学生看**（无教师端）；叙述"诊断 + 下一步"但不替学生定论；过程即数据（跳过也进评估输入与叙述）。
- UI 逐像素照 `docs/design/思维印记_工作区.dc.html` 的 `evalLoading`/`showEval` 块；`mk-*` token；真实 Phoebe 内容（NASA / Nature Sustainability / 碳排放 / 让步段），不用 lorem。

---

## 8. Out of scope / carry-forward

- **语义树归并（follow-on）**：eval LLM 顺手产出 `sub_question`/`key_knowledge`/`attempt` 节点 + 把 `card_use` 重挂到子问题下 + 打型；复用 S4 `ProcessNode` 模型 + `TreePanel`；并把 `nodeView` 缩进从二元改为按 node 列表走父链算真实 depth（depth×18px）。可与本 slice 的 `runEvaluation` 合并为一次结构化调用，或单独一刀。
- **S6（外壳）**：记录页「成长回顾」从 `evaluations` 的 narrative 渲染；活跃日历从 message/card 时间聚合；工具卡使用计数从 card_instance 聚合。评估触发将来也可放任务完成流。
- **benchmark / 微调**（doc 的 验证/规模化 阶段）：本期不做（MVP=few-shot）。
- few-shot 扩展：本期 1 份 Phoebe；Marcus/Ethan/Eliza 锚点样本后续补全（提准与覆盖红线/被动型）。
