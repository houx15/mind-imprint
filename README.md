<div align="center">

# 思维印记 · Mind Imprint

**一个把「思考过程」本身变成产品的 AI 学习平台。**

学生带着真实的任务进来，在与 AI 的协作中被引导做结构化思考——全过程被记录、被评估，最后长成一枚只属于他自己的「思维印记」。

[![TypeScript](https://img.shields.io/badge/TypeScript-5.4-3178C6?logo=typescript&logoColor=white)](https://www.typescriptlang.org/)
[![React](https://img.shields.io/badge/React-18-61DAFB?logo=react&logoColor=black)](https://react.dev/)
[![Vite](https://img.shields.io/badge/Vite-5-646CFF?logo=vite&logoColor=white)](https://vitejs.dev/)
[![pnpm](https://img.shields.io/badge/pnpm-workspace-F69220?logo=pnpm&logoColor=white)](https://pnpm.io/)
[![Vitest](https://img.shields.io/badge/tested%20with-Vitest-6E9F18?logo=vitest&logoColor=white)](https://vitest.dev/)

[设计铁律](#-四条设计铁律) · [架构](#-架构核心心智模型) · [快速开始](#-快速开始) · [工具卡系统](#-工具卡系统) · [过程评估](#-过程评估那一刀) · [路线图](#-状态与路线图)

</div>

---

## 这是什么

市面上的 AI 工具帮学生**更快地完成作业**。思维印记反过来——它帮学生**更深地完成思考**。

形态上它像 Cowork / Codex 这类 agent 工具，但封装了两层别处没有的能力：

1. **交互式「思维工具卡」** — 在对话里的对的时刻，AI 把一张结构化的思考脚手架塞回给学生（溯源、让步段、论证拆解……），由学生亲手填，而不是 AI 替他想。
2. **「过程评估」** — 完整的对话 + 每一次填卡都被序列化为标准信封，长成一棵**只读的过程树**，最后由旗舰模型按 SOLO 四级、九个维度生成一段诊断式的过程叙述。

> **我们占的是「思考」这一层，不是「作业被完成、被提交」的地方。** 学生的作文活在他自己的世界里（Google Doc、本子、随手粘进来），我们不拥有那个文档——我们只拥有他思考的轨迹。

锚定场景：**Phoebe 在写「中国是否让地球变得更可持续？」**，想直接引用一篇公众号文章当证据。AI 没有替她判断真假，而是召唤 `SIFT×CRAAP` 卡，陪她把来源溯到 NASA 与 *Nature Sustainability*；当她撞上「中国碳排放全球第一」这个反例时，AI 又召唤「让步段」卡，陪她把反例正面接住。整个过程被记录、被评估。

---

## 🧭 四条设计铁律

这四条是产品的脊梁，**违反即自毁**：

| # | 铁律 | 含义 |
|---|---|---|
| 1 | **AI 克制，绝不替学生定论** | AI 的职责不是给答案，而是在对的时刻把「思考」塞回给学生。系统 prompt 的核心就是这条克制阶梯。 |
| 2 | **不操纵** | 这个产品教学生「不被俘获」。绝不用老虎机式机制（连胜、排行榜、推送上瘾）留人——工具卡**触发是自动的，但「打开」永远由学生确认**。 |
| 3 | **一次只问一个** | 陪练对话短、不啰嗦，既降 token，也保护学生的思考节奏。 |
| 4 | **过程即数据** | 学生跳过工具卡、让 AI 直接答，也被记录。摩擦被转化成信号，而不是被消灭。 |

---

## 🏗 架构（核心心智模型）

核心抽象只有一句话：**工具卡 = 一次 tool-use 循环里「由人来执行的工具」。**

```
                决策层                         Card Runtime                     回灌 & 沉淀
   ┌───────────────────────────┐   ┌────────────────────────────┐   ┌──────────────────────────┐
   │ 陪练 LLM 判断「此刻该不该  │   │ schema 驱动渲染            │   │ 标准信封 → 过程树（只读） │
   │  用卡 / 用哪张」           │──▶│ 三视觉态（建议/激活/完成） │──▶│ 摘要回灌陪练，一次只问一个 │
   │                           │   │ 事件采集                   │   │ 全部信封 → 旗舰 rubric 评估 │
   │  summon_card(card_id,     │   │ 提交 → 标准信封落库        │   │  → 「你的思维印记」        │
   │     reason, nudge_text)   │   │                            │   │                          │
   └───────────────────────────┘   └────────────────────────────┘   └──────────────────────────┘
```

**三层解耦**让每一层都能独立演化：

- **决策层** 只做一个决定，并通过**单一函数** `summon_card(card_id, reason, nudge_text)` 接线。
- **Card Runtime** 是 **schema 驱动**的——卡的 spec 是数据，不是代码。
- **沉淀层** 把所有标准信封长成过程树 + 喂给评估，二者共享同一个地基。

### 前端直连 · BYO-key

本仓库是一个**纯前端 SPA**（无后端、无数据库）：

- LLM 调用由浏览器**直连**模型供应商，用户自带 API Key（Bring-Your-Own-Key）。
- Key 只存在于浏览器 `localStorage`（`mk.llmConfig`）与请求头里——**绝不进入 git、日志、数据存储或评估 payload**。
- 数据（task / message / card_instance / process_node / evaluation）持久化在 `localStorage`（`mk.store`）。
- 支持 **OpenAI 兼容**与 **Anthropic** 两种 wire format，适配器可插拔。

> PRD 里描述了一层「很薄的 Node + Express 网关 + SQLite」作为未来形态；当前 demo 以前端直连实现同等能力，便于零后端启动。

---

## 🗂 仓库结构

pnpm monorepo，两个 workspace：

```
mind-imprint/
├── packages/contracts/          # 单一真相源：Zod schema + 卡库
│   ├── src/
│   │   ├── primitives.ts         # 字段原语（discriminated union on `type`）
│   │   ├── cardSpec.ts           # 一张卡的 schema
│   │   ├── registry.ts           # 33 张卡的注册表（决策层目录从它派生）
│   │   ├── envelope.ts           # 标准信封结构
│   │   ├── rubric.ts             # 9 维 SOLO 量规（FULL_RUBRIC）
│   │   ├── evaluation.ts         # 评估输出契约
│   │   ├── summonCard.ts         # summon_card 工具参数契约
│   │   └── refeed.ts             # 卡摘要回灌陪练
│   └── cards/                    # 33 张卡，每张一个 JSON（新增卡 = 新增 JSON）
├── apps/web/                     # React SPA
│   └── src/
│       ├── llm/                  # BYO-key 网关：openai / anthropic 适配器、config
│       ├── store/                # localStorage 持久化 + 响应式 store
│       ├── agent/                # 陪练对话循环 + 评估那一刀
│       ├── cards/                # schema 驱动的卡渲染器 + 字段组件 + 信封 reducer
│       ├── workspace/            # 工作区：对话 / 过程树 / 卡 sheet / 评估弹窗
│       ├── shell/                # 应用外壳：导航 / 主目录 / 记录页 / 设置 / key-gate
│       └── dev/                  # 开发期 harness（不在产品外壳里挂载）
└── docs/                         # 权威产品规格：PRD、Roadmap、工具包库、设计稿
```

---

## 🚀 快速开始

**前置：** [Node](https://nodejs.org/) ≥ 20、[pnpm](https://pnpm.io/installation) ≥ 8，以及一个 LLM API Key（OpenAI 兼容或 Anthropic）。

```bash
# 1. 安装依赖
pnpm install

# 2. 配置你的 Key（拷贝模板 → 填写；该文件已被 gitignore）
cp apps/web/.env.example apps/web/.env.local
#   编辑 apps/web/.env.local，填入 VITE_LLM_* 配置

# 3. 启动开发服务器
pnpm --filter web dev          # → http://localhost:5173

# 4. 质量门禁
pnpm -r typecheck              # 全工作区类型检查（tsc --noEmit）
pnpm -r test                   # 全工作区测试（Vitest）
```

> 也可以**不**配 `.env.local`，直接在 app 内的首跑「key-gate」弹窗或设置页填写 Key——填完点「测试连接」通过即可解锁。

### 配置 LLM Key

`apps/web/.env.local`（git-ignored）：

```dotenv
VITE_LLM_FORMAT=openai                       # openai | anthropic
VITE_LLM_BASE_URL=https://api.openai.com/v1  # 适配器会拼上 /chat/completions
VITE_LLM_MODEL=gpt-4o
VITE_LLM_API_KEY=sk-...
VITE_LLM_EVAL_MODEL=                          # 可选；评估走旗舰模型，留空则与上同
```

<details>
<summary><b>示例：DeepSeek（OpenAI 兼容）& 推理模型注意事项</b></summary>

```dotenv
VITE_LLM_FORMAT=openai
VITE_LLM_BASE_URL=https://api.deepseek.com
VITE_LLM_MODEL=deepseek-v4-pro
VITE_LLM_EVAL_MODEL=deepseek-v4-pro
VITE_LLM_API_KEY=sk-...
```

**推理模型（reasoning model）** 会先消耗一段「思考」token 再吐可见内容。若 `maxTokens` 太小，`content` 会为空 / JSON 被截断。代码里已为各调用路径留足余量（陪练默认 1024、评估 8000）；自定义调用时请确保预算覆盖「推理 + 输出」。

</details>

> 🔐 **安全：** API Key 只活在你的浏览器和发往模型的请求头里。它不会被提交进 git、不会出现在日志或报错文本里、不会进入数据存储或评估 payload。`.env.*` 已在 `.gitignore`。

### 端到端演示脚本（验收主动脉）

> 登录（mock）→ key-gate 填 Key 并测连接 → 新建任务（粘文章链接）→ 陪练对话 → AI 调 `summon_card` → 打开并填卡 → **过程树实时生长** → 点「生成思维印记」→ 九维评估 + 过程叙述 → 记录页能力雷达。

---

## 🃏 工具卡系统

工具卡是 **schema 驱动**的：**新增一张卡 = 新增一份 JSON 配置，不改渲染器代码。** 这是「schema 驱动是否成立」的验证标准。

一张卡用现有**字段原语**拼出来：

`text` · `textarea` · `single_choice` · `multi_choice` · `rating` · `repeatable_group` · `link_check` · `spectrum` · `criteria_check`，外加一个 `show_if` 条件显隐修饰符。

每张卡有三种**视觉态**：`建议工具卡`（学生确认是否打开）→ `现在轮到你想`（激活、采集事件）→ `已完成 · 已钉到过程树`（序列化为标准信封）。

> **卡库现状：33 张卡全部可交互（0 stub）**，覆盖知识工具、信息素养、AI 伦理、溯源与多视角、探究启动、反身性与元认知、AOK 等类别。其中 `sift_craap`（SIFT×CRAAP 信息核查）与 `concession`（让步段）为两张样例卡。

新增交互的成本阶梯（绝大多数情况停在第 1 步）：

1. **能用现有原语拼出来** → 只写一份卡 JSON。
2. 真需要全新交互 → 加一个字段原语：在 `primitives.ts` 的 union 里加类型、写组件、在 `fieldRegistry` 注册——然后才是写卡 JSON。

---

## 🎯 过程评估（那一刀）

这是产品的护城河。任务「告一段落」时，旗舰模型读完整对话 + 全部标准信封，按 **SOLO 四级**（L1 单点 · L2 多点 · L3 关联 · L4 拓展）对**九个维度**独立打分，并写一段诊断式过程叙述：

| | 维度 | | 维度 |
|---|---|---|---|
| D1 | 提问清晰度 | D6 | 反思与元认知 |
| D2 | 信源辨识 | D7 | 论证质量 |
| D3 | 横向验证 | D8 | 信息再生产 |
| D4 | 多视角与让步 | D9 | AI 边界与伦理 |
| D5 | 论证拆解 | | |

评估结果**只给学生本人看**，语气是诊断而非判断——描述「你做了什么」，而不是「你应该怎么想」。评估走**旗舰模型，绝不降级**（陪练对话可走中档模型）。结果以「你的思维印记」弹窗呈现，并在记录页沉淀为活跃日历、工具卡用量与九维能力雷达。

---

## 🧱 技术栈

| 层 | 选型 |
|---|---|
| 前端 | React 18 + Vite + TypeScript + Tailwind CSS |
| 契约 | Zod（单一真相源，`@mind-imprint/contracts`） |
| 状态 | 自建响应式 store（`useSyncExternalStore`），`localStorage` 持久化 |
| 模型 | Anthropic / OpenAI 兼容；浏览器直连，BYO-key |
| 测试 | Vitest + Testing Library（contracts 25 文件 / apps/web 58 文件，全绿） |
| 工具链 | pnpm workspace · `tsc --noEmit` 类型门禁 |

---

## ✅ 状态与路线图

平台**端到端可跑通**，并已用真实 LLM Key 在浏览器里走通完整验收流程（登录 → key-gate → 陪练 → 召唤卡 → 填卡 → 过程树 → 九维评估 → 记录页）。

- ✅ 全部路线 slice 完成（应用外壳、过程树、评估那一刀、富交互卡）。
- ✅ 33 张卡全部可交互，0 stub。
- 🔭 非阻塞增强（已登记于 `docs/遗留项追踪_Carryforward.md`）：评估语义树归并与 few-shot 扩展、画布导图的自由拖拽 / 角色模拟的多轮对话等视觉层、若干 a11y / 整洁项。

### 本期明确不做（范围决定，非遗漏）

- ❌ 文档编辑器 / 作业撰写排版 / 提交功能（过程树是**只读**记录）。
- ❌ 教师端：账户、师生关系、批改、评分、通知。
- ❌ 真实登录鉴权（demo 用单一 mock 学生）。
- ❌ 上瘾式游戏化：连胜、排行榜、徽章、推送一律不做。
- ❌ 检索 / RAG 路由（卡少，决策层直接用目录 + 分类）。

---

## 📚 文档

权威产品规格都在 `docs/`：

- **`docs/思维印记_Demo_PRD.md`** — 技术规格 / 不变骨架（冲突时以 PRD 为准）。
- **`docs/架构分解_Roadmap.md`** — 分块路线图。
- **`docs/工具包库/`** — 工具卡方法论全库。
- **`docs/design/`** — 设计稿（`.dc.html` 对 UI 具有约束力）。
- **`docs/遗留项追踪_Carryforward.md`** — 逐项遗留追踪，保证端到端不丢。
- **`AGENTS.md`** — 给 AI 协作者（Claude Code / Codex / Gemini）的硬约束摘要。

---

## 🤝 贡献

1. 改动前先读 `AGENTS.md` 的「硬约束」与四条设计铁律。
2. 卡 spec 的单一真相源在 `packages/contracts` 的 registry——决策层目录从它派生，不手写第二份。
3. 提交前确保 `pnpm -r typecheck` 与 `pnpm -r test` 全绿。
4. **共享记忆只写进 `AGENTS.md`**（`CLAUDE.md` / `GEMINI.md` 只 import 它，勿直接改）。

---

## 📄 许可证

本仓库为内部 demo，暂未指定开源许可证。
