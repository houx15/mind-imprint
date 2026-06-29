# 讨论议题队列 · Discussion Queue

> **建立：** 2026-06-28。在开新模块前，先把这五个「需要重度讨论」的设计议题逐个 brainstorm 定稿。
> **状态约定：** 🔜 待讨论 · 🟡 讨论中 · ✅ 已定稿（产出 spec → plan → build）。
> 每个议题独立走 `superpowers:brainstorming` → spec（`docs/superpowers/specs/`）→ plan → build。
> 锚点：四条设计铁律（AI 克制 / 不操纵 / 一次只问一个 / 过程即数据）；后端 north-star
> `docs/superpowers/specs/2026-06-24-backend-platform-architecture-design.md`。
>
> **▶ 当前位置（2026-06-29）：** #5 ✅、#1+#3 ✅、#2 ✅（均出 spec，待 review）。
> **下一步 = brainstorm #4（组织/注册复核）** —— 队列最后一项。全部为「讨论先于实现」，spec 定稿后再决定是否进入 plan/build。
> #2 被 reframe：从「可扩展性复核」变为「卡模型演进」—— 卡内 AI 导师 + DB 运行时存储（teacher 授权流程本轮显式不做）。

---

## 推荐顺序（按数据依赖）

1. **#5 评估模型流（先做）** — P4（river 异步评估）的前置。评估 schema 在 `AGENTS.md` 仍标 **暂定**，
   既未定稿又卡住下一实施期；先定它，避免在不稳的 schema 上设计 P4 而返工。
2. **#1 + #3（合并讨论）** — AI 如何召唤卡 + agent 该做什么，其实是同一个设计面（服务端决策层 + turn loop）。
3. **#2 卡架构是否可扩展** — 多为「现状能否撑下一批卡 / 多模态」的复核，而非重设计。
4. **#4 组织/注册** — 大部分已建成并通过端到端 live smoke，偏「盘点缺口」而非全新 brainstorm。

> 顺序可调；以上为默认建议。

---

## 五个议题

### #5 · 评估模型流（Evaluation Model Flow）　🔜　**建议最先**
- 评估消费什么数据：过程树、标准信封、跳过/摩擦信号如何投影成评估 payload。
- 旗舰模型（`deepseek-reasoner`，绝不降级）如何被 prompt；rubric 评级 + 过程叙述如何生成。
- `evaluation` 表存什么（现 **暂定**）；与 `message`/`card_instance`/过程树的关系。
- 异步路径：P4 的 river 任务 + worker，入队 + 轮询。
- 「你的思维印记」呈现：评级 + 过程叙述的结构。
- **为何先做：** 唯一同时「未定稿」且「阻塞 P4」的议题。

### #1 · AI 如何召唤卡（Card Summoning / 决策层）　🔜
- 何时/为何触发 `summon_card`；`tool_choice` 策略（今为 `auto`，概率性召唤）。
- 正确的卡是否可靠出现；召唤可靠性 vs. 设计铁律「触发自动，但打开由学生确认」。
- 决策层目录如何从卡 registry 派生（避免 `trigger_condition` 漂移）。
- 既是 prompt 工程问题，也是架构问题。建议与 #3 合并讨论。

### #3 · AI agent 该做什么（Agent Responsibilities）　🔜
- 完整 turn loop：系统 prompt、refeed、一次只问一个、何时召唤 vs. 仅陪练。
- 模型分档：陪练中档（可降级）vs. 评估旗舰（绝不降级）。
- #1 与 #5 之间的连接组织。建议与 #1 合并为一次 brainstorm（服务端决策层 + turn loop）。

### #2 · 卡的最佳技术架构（Card Architecture）　🔜
- schema 驱动的 Card Runtime：字段原语、标准信封、卡 JSON 单一真相源（web import + Go `go:embed`）。
- 现有 7 原语（`text`/`textarea`/`single_choice`/`multi_choice`/`rating`/`repeatable_group`/`link_check`）
  + 逃生舱 JSX 能否撑下一批卡 / 多模态。
- 「新卡 = 新 JSON」与「需新原语」的真实边界在哪。
- **现状：** 33 张卡、0 stub、schema 驱动已成立 → 偏「可扩展性复核」而非重设计。

### #4 · 学校/教师/班级/学生 的创建与注册（Org & Registration）　🔜
- 组织模型：provisioning 流、join code / 教师 invite code、「无组织账号不存在」不变式、`HasEntitlement` 接缝。
- **现状：** P2 + 全部 P3 已建成并合入 main，并刚被端到端 live smoke 验证 → 偏「盘点已有 + 决定缺口」，
  非全新设计。短复核即可。

---

## 进度记录

| 议题 | 状态 | spec | plan | 备注 |
|---|---|---|---|---|
| #5 评估模型流 | ✅ 设计定稿（待 review） | `2026-06-29-evaluation-model-design.md` | — | 全 5 步定稿：认知模型(10维/3层/hybrid) + 量规 v2(过 V1–V6) + hybrid-anchor 管线 + 数据模型 + milestone 异步触发 + 呈现/隐私。记录见 `2026-06-28-evaluation-model-design-notes.md`、量规 `2026-06-28-cognitive-model-rubric.md` |
| #1 卡召唤 | ✅ 设计定稿（待 review） | `2026-06-29-card-summon-decision-layer-design.md` | — | 决策层：共享规则信号 → stage-1 `summon_when` 确定性闸门 → 策略(去重/冷却) → stage-2 全自治(auto over k 候选)。卡 spec 加 `summon_when`，单一真相源 + build 校验。 |
| #3 agent 职责 | ✅ 设计定稿（待 review） | `2026-06-29-card-summon-decision-layer-design.md` | — | 与 #1 同 spec：保持反应式单步陪练，但不再 process-blind（知道用过哪些卡 + rubric_dims 钩子留待 weak-dim 召唤）。 |
| #2 卡架构 | ✅ 设计定稿（待 review） | `2026-06-29-card-architecture-evolution-design.md` | — | reframe 为卡模型演进：卡=教学区(图文+方法限定导师)/录入区(声明式表单,无AI)/外部陪练；存储=DB数据+文件行为拆分(DB为运行时唯一源,33 JSON 转 seed)；卡内导师=单运行时按卡 grounding、pull 非 push、走网关、对话入 event_trace。teacher 授权流程+导师机制+数据模型 = carry-forward。 |
| #4 组织/注册 | 🔜 | — | — | 盘点缺口为主（队列最后一项） |
