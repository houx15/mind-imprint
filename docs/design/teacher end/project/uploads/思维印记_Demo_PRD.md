# 思维印记 · Demo PRD / 技术规格

> 版本 v0.1 ｜ 2026-06-20
> 面向：交付给 Claude Code 进行开发的实现规格
> 配套：《IB-AI学习平台-产品概念.md》《IB-AI学习平台-调研报告.md》《20260618讨论材料/02_思维印记_产品骨架/*》

---

## 0. 这份文档怎么用

这是一份**可直接开发**的规格。读它的顺序：先读 §1–§4 理解要做什么、不做什么；再读 §5–§12 理解架构与核心契约（这是不变的骨架，必须一次定对）；§13 是界面；§14 是端到端演示脚本，用来验收。

一句话目标：**用一个真实的 AI 工具，验证「AI 在对的时刻调对工具卡 + 工具卡可交互、可展示、可标准化存储 + 全过程长成一棵可读的树」。** 其余一概从简。

---

## 1. 产品定位与设计铁律

**定位：** 思维印记是一个 **AI 工具**（形态类似 Cowork / Codex 这类 agent 工具），但封装了两层独有能力——交互式「思维工具卡」与「过程评估」。学生带自己的真实任务进来，在与 AI 协作的过程中被引导做结构化思考，全过程被记录、评估。

**我们占的是「思考」这一层，不是「作业被完成、被提交」的地方。** 学生的作文活在他自己的世界里（Google Doc、本子、随手粘进来），我们不拥有那个文档。

**四条设计铁律（违反即自毁）：**

1. **AI 克制，绝不替学生定论。** AI 的职责不是给答案，是在对的时刻把「思考」塞回给学生。系统 prompt 的核心就是这条克制阶梯。
2. **不操纵。** 这个产品教学生「不被俘获」。所以它自己绝不能用老虎机式机制（连胜、排行榜、推送上瘾）留人，也不能用弹窗强迫——工具卡的**触发是自动的，但「打开」由学生确认**。
3. **一次只问一个。** 陪练对话短、不啰嗦，降低 token 也保护学生的思考节奏。
4. **过程即数据。** 学生跳过工具卡、让 AI 直接答，也是被记录的一部分。摩擦被转化成信号，而不是被消灭。

---

## 2. Demo 要验证什么（成功判据）

1. **AI 正确调用工具卡**：在真实对话中，AI 能在该核查来源时调出 SIFT×CRAAP 卡、在论证缺让步时调出让步段卡，而不是张冠李戴或乱弹。
2. **工具卡可交互、可展示、可标准化存储**：卡是 schema 驱动渲染的真实交互组件（不是文本气泡），填写过程被采集，最终序列化成统一的「标准信封」。
3. **过程长成一棵可读的树**：一次任务的完整过程被组织成过程树，能看出哪里用了什么卡、做了什么尝试、撞上哪些关键知识。
4. **AI 互动是真的**：陪练与评估都是真实 LLM 调用。

---

## 3. 范围：做什么

- 学生端 Web 应用，三个顶层 tab：**任务 / 记录 / 工具卡**。
- 任务工作区：对话主轴 + 底部卡交互窗 + 右侧过程树。
- 决策层（选分类 → 选卡）+ `summon_card` 调用契约。
- Card Runtime：字段原语、schema 驱动渲染器、三视觉态、事件采集、标准信封落库。
- 两张样例卡：**SIFT×CRAAP**、**让步段**。
- 评估那一刀：跑出 rubric 评级 + 过程小结，**以「你的思维印记」呈现给学生**。
- 薄后端：LLM 网关（key 保护 + 路由）、数据服务、异步评估。SQLite 持久化。

## 4. 明确不做（本期）

- ❌ 文档编辑器 / 作业撰写与排版 / 提交功能。
- ❌ 教师端：账户体系、师生关系、批改流、评分、通知、标注沉淀。（评估**数据**照跑，但只给学生看，不建教师平台。）
- ❌ 工具卡全库（~30 张）：本期只做 2 张；全库提炼**另开任务并行**，做完直接灌进同一渲染器。
- ❌ 检索/RAG 路由：卡少，决策层直接用「目录 + 分类」即可；为未来留缝但不实做。
- ❌ 真实登录鉴权：demo 用单一 mock 学生。
- ❌ 上瘾式游戏化：连胜、排行榜、推送一律不做。

---

## 5. 技术栈与总体架构

**形态：Web 端。** 无复杂本地运行时需求；零安装、发 URL 即可演示、跨平台。

**前后端分离（后端很薄）。** 客户端绝不直连模型——API key 与计量必须在服务端（见《模型路由方案》）。

| 层 | 选型 | 职责 |
|---|---|---|
| 前端 | React + Vite + TypeScript + Tailwind | 对话、schema 驱动卡渲染器、过程树、日历、工具卡使用 |
| 后端 | Node + Express（TypeScript） | LLM 网关（陪练+评估+key 保护+路由计量）、registry/数据服务、异步评估 |
| 存储 | SQLite | 持久化 task / message / card_instance / process_node / evaluation |
| 模型 | Anthropic API（需用户提供 key） | 陪练走中档模型、评估走旗舰模型 |

**三层解耦（核心心智模型：工具卡 = tool-use 循环里「由人来执行的工具」）：**

```
学生发言
  │
  ▼
[决策层 / 未来可升格为 skill]  装：卡目录(从 registry 派生) + 克制策略 + 选分类→选卡
  │  产出意图
  ▼
summon_card(card_id, reason, nudge_text)   接线协议（function calling，单函数+enum）
  │
  ▼
[Card Runtime]  registry 按 id 取 spec → schema 驱动渲染器(类型→组件)
  │              → 三视觉态(提议/激活/完成) → 事件采集 → 标准信封
  │  把填写摘要作为 tool result 回灌
  ▼
陪练 LLM 继续（克制：一次只问一个）
  │
  └─→ 所有标准信封 → 过程树 + rubric 评估（旗舰、异步）→「你的思维印记」
```

三层各司其职：**决策层**管「用不用、用哪张」；**`summon_card`** 管接线；**Card Runtime** 管「卡怎么渲染、数据怎么标准化落下来」。

**陪练 vs 评估分离（模型路由）：** 陪练实时、中档模型、可降级；评估异步、旗舰模型、绝不降级。评估是产品护城河，算力集中砸在这一刀上。

---

## 6. 工具卡契约（Card Contract）

> 这是整个系统最该一次定对的东西。任何被 `summon_card` 召唤的卡都遵守同一份契约，全系统只有一条管线伺候所有卡。

### 6.1 字段原语（整个卡库靠这几种拼，新卡不写新代码）

| type | 用途 | 渲染 |
|---|---|---|
| `text` | 短文本 | 单行输入 |
| `textarea` | 长文本 | 多行输入 |
| `single_choice` | 单选（options） | 按钮组/下拉 |
| `multi_choice` | 多选（options） | 复选组 |
| `rating` | 评分刻度（scale，如 CRAAP 1–5） | 五格/五星 |
| `repeatable_group` | 可加行的组（item_fields） | 多源对照表 |
| `link_check` | 贴 URL + 判定 | URL 输入 + 判定标签 |

新增原语只在「需要一种全新交互」时才加；目标是用现有原语覆盖绝大多数卡。

### 6.2 卡 schema 结构

```
{
  id, category, name, purpose,
  trigger_condition,          // 自然语言，决策层用它分类
  steps: [
    {
      key, title,
      disclose: "always" | "on_demand",   // 渐进式披露
      methodology_note,                    // 点「我不懂为什么」就地展开
      fields: [ <字段原语> ]
    }
  ],
  rubric_tags: [ ... ]         // 命中的评估维度
}
```

### 6.3 样例卡 1：SIFT×CRAAP 信息核查（完整）

```json
{
  "id": "sift_craap",
  "category": "信息素养",
  "name": "SIFT×CRAAP 信息核查",
  "purpose": "先横向找更多来源(SIFT)，必要时再纵向深挖单一材料(CRAAP)",
  "trigger_condition": "学生准备直接采信或引用一个网络来源，但还没核查出处",
  "steps": [
    {
      "key": "sift", "title": "SIFT · 横向找更多来源", "disclose": "always",
      "methodology_note": "遇到一条信息先横向扩展、别一头扎进单一材料：Stop 停一下、Investigate 查来源、Find better coverage 找更权威版本、Trace 溯源。",
      "fields": [
        {"key": "stop", "type": "textarea", "label": "Stop：你打算用这条信息说明什么？"},
        {"key": "sources", "type": "repeatable_group", "label": "Investigate：找出 3 个独立来源",
          "item_fields": [
            {"key": "name", "type": "text", "label": "来源"},
            {"key": "type", "type": "single_choice", "label": "类型", "options": ["官方", "主流媒体", "学者/机构", "自媒体", "社交平台"]},
            {"key": "verdict", "type": "single_choice", "label": "可信？", "options": ["可信", "存疑", "不可信"]}
          ]},
        {"key": "better", "type": "textarea", "label": "Find better coverage：更权威的版本怎么说？"},
        {"key": "trace", "type": "link_check", "label": "Trace：溯到原始出处（贴链接）"}
      ]
    },
    {
      "key": "craap", "title": "CRAAP · 纵向深挖单一材料", "disclose": "on_demand",
      "methodology_note": "当你决定重点采信某个来源时，再纵向核它五维。",
      "fields": [
        {"key": "currency", "type": "rating", "scale": 5, "label": "Currency 时效性"},
        {"key": "relevance", "type": "rating", "scale": 5, "label": "Relevance 相关性"},
        {"key": "authority", "type": "rating", "scale": 5, "label": "Authority 权威性"},
        {"key": "accuracy", "type": "rating", "scale": 5, "label": "Accuracy 准确性"},
        {"key": "purpose", "type": "rating", "scale": 5, "label": "Purpose 目的性"}
      ]
    }
  ],
  "rubric_tags": ["D1_来源意识", "D2_交叉验证"]
}
```

### 6.4 样例卡 2：让步段（同一套原语，另一份配置）

```json
{
  "id": "concession",
  "category": "知识工具",
  "name": "让步段 · 以退为进",
  "purpose": "在论证里先承认反方最强的事实，再转折反驳，使论证更有力、结构更完整",
  "trigger_condition": "学生在写论证段，且出现了与其中心论点相悖的证据，却想直接忽略或硬压",
  "steps": [
    {
      "key": "concession", "title": "让步段四步", "disclose": "always",
      "methodology_note": "让步段是以退为进：先退一步承认你不同意的一个事实，再对它反驳。它让论证更有力、逻辑更严密。",
      "fields": [
        {"key": "thesis", "type": "text", "label": "你的中心论点是什么？"},
        {"key": "counter", "type": "textarea", "label": "反方最强的那个事实是什么？"},
        {"key": "concede", "type": "textarea", "label": "先承认它（让步）"},
        {"key": "rebut", "type": "textarea", "label": "再转折反驳（为什么它不足以推翻你的论点）"}
      ]
    }
  ],
  "rubric_tags": ["D5_论证结构", "D7_对立观点处理"]
}
```

### 6.5 三视觉态（同时对应数据日志）

- **提议态**：对话里一句轻量 in-context nudge（来自 `nudge_text`）+「打开卡」按钮。学生点了才展开。
- **激活态**：底部升起大交互窗，schema 驱动渲染整张卡；`disclose:"on_demand"` 的步骤先折叠。视觉上接管这一刻——「现在轮到你想」。
- **完成态**：提交后折叠成紧凑卡片，钉在右侧过程树，并把摘要回灌给 AI。

### 6.6 渐进式披露（三个嵌套层级）

1. **库 → 卡**：学生侧按课程库五大分支折叠；AI 一次只在上下文浮出**一张**最相关的卡，绝不一次摊 30 个。
2. **卡内**：如 SIFT×CRAAP，先出 SIFT，需要时才展开 CRAAP（`disclose:"on_demand"`）。
3. **讲解**：方法论只在学生点「我不懂为什么」时就地展开（不跳走）。

---

## 7. 决策层（选分类 → 选卡）

- **不做 RAG。** 卡少，直接给模型一份**紧凑目录**（按分类组织，每张卡一行 `trigger_condition`，不是完整 schema），让它分类。
- 目录**从 registry 派生**，不手写第二份（避免 `trigger_condition` 漂移）。registry 是源，目录是投影。
- 决策层本期用「系统 prompt（装目录）+ `summon_card` 工具」实现。卡库变大、由非工程人员维护时，再升格为正式 skill。
- 决策层 prompt 要点：① 你的职责不是给答案；② 当且仅当当前这一刻命中某张卡的 trigger_condition 时，调用 `summon_card`；③ 否则正常陪练，一次只问一个；④ 学生明确拒绝某卡时不要反复弹。

---

## 8. `summon_card` 契约与回灌循环

**只注册一个函数**（不要每张卡一个 tool）：

```
summon_card(
  card_id: enum,        // enum = 卡目录；模型把「这一刻」分类进某张卡
  reason: string,       // 为什么现在该用这张卡（内部记录）
  nudge_text: string    // 给学生看的一句提议语
)
```

**端到端 tool-use 循环：**

1. 学生发言 → 后端组陪练请求（系统 prompt + 目录 + `summon_card` 工具）→ LLM。
2. LLM 决定调用 `summon_card("sift_craap", …)` → 前端按 id 从 registry 取 spec → `<CardRenderer>` 内嵌渲染（提议态）。
3. 学生确认 → 激活态 → 填写（事件被采集）→ 提交 → 标准信封落库。
4. 把**填写摘要作为 tool result 回灌**给 LLM → LLM 看到学生**自己的**思考，继续陪练。
5. 学生若跳过/拒绝，也作为事件记录（`status: skipped`）。

---

## 9. 标准信封与数据模型

**标准信封（`card_instance` 表，整个系统的脊椎）：**

```
{ id, card_id, task_id, parent_node_id,
  status,            // proposed | active | completed | skipped
  field_values,      // JSON：学生填的内容
  event_trace,       // JSON：field_change / step_expand / note_open / skip / submit + 时间
  rubric_tags,
  created_at, completed_at }
```

**数据模型（SQLite）：**

| 表 | 关键字段 | 说明 |
|---|---|---|
| `task` | id, title, seed, status, created_at, last_active_at | 学生带进来的项目 |
| `message` | id, task_id, role, content, tool_call(JSON), created_at | 对话 |
| `card_instance` | 见上「标准信封」 | **脊椎**：喂树、日历、使用计数、评估 |
| `process_node` | id, task_id, parent_id, type, label, ref_id, meta | 过程树节点（可派生） |
| `evaluation` | task_id, rubric_scores(JSON), narrative, created_at | 每任务评估 |

**配置（静态文件，非用户数据）：** 卡 spec registry（一卡一 JSON）、rubric 维度定义、决策层目录（派生）。

日历与工具卡使用次数：直接从 `card_instance` + `message` 聚合，不单独建表。

---

## 10. 过程树

**两棵树要分清：**
- **技能树**（已有设计，本期不做）：跨项目、累积的能力地图。
- **过程树**（本期做）：单个任务内学生真实走过的路径。

**过程树建模：**
- **根** = 任务（`seed`）。
- **枝** = 子问题 / 分论点。
- **节点 type（锚定这几类，避免自动生成噪声）**：`task_root` / `sub_question` / `card_use` / `attempt`（尝试草稿）/ `key_knowledge`（撞上的关键知识）/ `concession`（让步修订）/ `reflection`（反身收口）。
- 节点元数据：用了哪张卡、参与还是跳过、填了什么、当时 rubric 信号、时间。

**派生方式：** 从 `card_instance` + `message` 事件流派生；评估那一刀的 LLM 顺手把事件归并成枝、给节点打 type 与标签。**右侧面板实时渲染，只读，不是编辑器。**

---

## 11. 评估（陪练 vs 评估那一刀）

- **触发时机：** 任务进行中可异步增量跑，或任务告一段落时整体跑。
- **输入：** 完整对话 + 全部标准信封（含 event_trace）。
- **模型：** 旗舰，不降级。
- **产物：** ① 各 rubric 维度的 **SOLO 四级（L1–L4）** 评级；② 一段过程叙述（哪里深、哪里跳过、关键知识时刻、成长点）。
- **呈现：** 本期**只给学生看**——以「你的思维印记 / 过程小结」呈现。不建教师账户/批改/通知。
- rubric 维度参考《思维印记_评估平台介绍》的 AI 批判性思维 9 维；demo 用其中与两张卡相关的子集即可（如来源意识、交叉验证、论证结构、对立观点处理、元认知）。

---

## 12. 决策层 / 评估的 Prompt 设计（交付时细化）

- 陪练系统 prompt：定位 + 四条铁律 + 目录 + `summon_card` 使用条件 + 「一次只问一个」。
- 评估系统 prompt：给定 rubric 维度定义 + SOLO 四级锚点 + 锚点样本（Phoebe L1→L4 等）作为 few-shot，对整段过程打分 + 写叙述。
- 锚点样本见《评估平台介绍》：Phoebe（L1→L4 完整光谱）、Marcus（被动 L1-2）、Ethan（代写投机红线）、Eliza（跨模块迁移 L3→L4）。

---

## 13. 学生端界面规格

**顶层三 tab：`任务 / 记录 / 工具卡`。**

### 13.1 任务（主目录，首页）
- 所有任务网格，每张卡：标题、状态、上次活动、迷你进度。
- 顶部「+ 新任务」入口 = 一句「你想搞懂什么？」（带自己的东西来）。
- 点已有任务可续，点新任务开启新工作区。
- **不常驻左侧任务列表**（专注设计）。

### 13.2 任务工作区（核心，专注全屏）
- **中央**：对话（主轴），AI 回应短、克制。
- **底部**：卡触发时升起大交互窗（bottom sheet）= 激活态；填完折叠进右侧树。
- **右侧**：过程树/步骤，边做边长，可折叠，**只读**。
- **顶部**：「← 返回所有任务」面包屑（替代左侧列表）。

### 13.3 记录（你的思维印记）
- GitHub 式活动日历（每日活跃格）。
- 周期性成长回顾（来自评估叙述）。

### 13.4 工具卡（我的工具卡）
- 已收集的卡 + 使用记录/次数（哪张用了几次）。
- 兼作技能树的轻量入口。

---

## 14. 端到端演示脚本（用于验收）

场景锚定 **Phoebe / 「中国是否让地球变得更可持续？」**（取自合伙人原创课例）。

1. 学生新建任务，seed：「我看到一篇说『中国让地球变绿』的公众号文章，想用它写中国让地球更可持续。」并粘入文章链接。
2. AI 不替他评判来源 → **调用 `summon_card("sift_craap")`** → 提议态 nudge：「这儿先别急着写，我们用 SIFT 核一下这个来源？」
3. 学生打开卡（激活态）→ 填 SIFT 四步：发现原文实为 NASA / Nature Sustainability（IF 32.1）→ 必要时展开 CRAAP 评五维 → 提交。
4. 摘要回灌 → AI 陪练，一次只问一个：「你现在有几个独立来源支持『中国让地球更可持续』？」
5. 学生写论证时引入反例（中国碳排放全球第一）→ **AI 调用 `summon_card("concession")`** → 学生完成让步段四步。
6. 右侧过程树实时长出：任务根 → 子问题「来源可信吗」(card_use: SIFT) → 关键知识 (Nature Sustainability) → 子问题「碳排放反例」→ 让步 (card_use: 让步段)。
7. 任务告一段落 → 评估那一刀跑 → 「你的思维印记」展示 rubric 评级 + 过程叙述。

**验收 = §2 的四条全部可见，且第 14 步这条主动脉能稳定走通。**

---

## 15. 里程碑建议（交给工程）

1. **骨架**：Card Contract + 字段原语 + schema 驱动渲染器 + 两张样例卡（先纯前端渲染，假数据）。
2. **接线**：薄后端 LLM 网关 + `summon_card` 工具 + 回灌循环（陪练跑通，能真实调卡）。
3. **持久化**：SQLite + 标准信封落库 + 三视觉态状态机。
4. **过程树**：从事件流派生 + 右侧实时渲染。
5. **评估**：异步评估那一刀 + 「你的思维印记」呈现。
6. **外壳**：三 tab + 主目录 + 日历 + 工具卡使用。

先把 1–2 跑通（这就证明了核心成功判据 1、2、4），再往后铺。

---

## 16. 给开发者的硬约束清单

- 客户端绝不直连模型；所有 LLM 调用走后端网关，记录档位 + token + 成本。
- 卡 spec 单一真相源在 registry；决策层目录从它派生。
- 新增卡 = 新增一份 JSON 配置，**不改渲染器代码**（验证 schema 驱动是否成立）。
- 标准信封结构一旦定下不要随意改——它是树、日历、使用计数、评估的共同地基。
- 不做 §4 清单里的任何一项。
