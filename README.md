<div align="center">

# 思维印记 · Mind Imprint

**一个把「思考过程」本身变成产品的 AI 学习平台。**

学生带着真实的任务进来，在与 AI 的协作中被引导做结构化思考——全过程被记录、被评估，最后长成一枚只属于他自己的「思维印记」。

[![Go](https://img.shields.io/badge/Go-1.26-00ADD8?logo=go&logoColor=white)](https://go.dev/)
[![PostgreSQL](https://img.shields.io/badge/PostgreSQL-16-4169E1?logo=postgresql&logoColor=white)](https://www.postgresql.org/)
[![TypeScript](https://img.shields.io/badge/TypeScript-5.4-3178C6?logo=typescript&logoColor=white)](https://www.typescriptlang.org/)
[![React](https://img.shields.io/badge/React-18-61DAFB?logo=react&logoColor=black)](https://react.dev/)
[![Vite](https://img.shields.io/badge/Vite-5-646CFF?logo=vite&logoColor=white)](https://vitejs.dev/)
[![pnpm](https://img.shields.io/badge/pnpm-workspace-F69220?logo=pnpm&logoColor=white)](https://pnpm.io/)

[设计铁律](#-四条设计铁律) · [架构](#-架构核心心智模型) · [快速开始](#-快速开始) · [工具卡系统](#-工具卡系统) · [过程评估](#-过程评估那一刀) · [路线图](#-状态与路线图)

</div>

---

## 这是什么

市面上的 AI 工具帮学生**更快地完成作业**。思维印记反过来——它帮学生**更深地完成思考**。

形态上它像 Cowork / Codex 这类 agent 工具，但封装了两层别处没有的能力：

1. **交互式「思维工具卡」** — 在对话里的对的时刻，AI 把一张结构化的思考脚手架塞回给学生（溯源、让步段、论证拆解……），由学生亲手填，而不是 AI 替他想。
2. **「过程评估」** — 完整的对话 + 每一次填卡都被序列化为标准信封，长成一棵**只读的过程树**，最后由旗舰模型按 SOLO 四级、**认知模型十维**生成一段诊断式的过程叙述。

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
                决策层（服务端）                  Card Runtime（前端）             回灌 & 沉淀
   ┌───────────────────────────┐   ┌────────────────────────────┐   ┌──────────────────────────┐
   │ 陪练 LLM 判断「此刻该不该  │   │ schema 驱动渲染            │   │ 标准信封 → 过程树（只读） │
   │  用卡 / 用哪张」           │──▶│ 三视觉态（建议/激活/完成） │──▶│ 摘要回灌陪练，一次只问一个 │
   │                           │   │ 事件采集                   │   │ 全部信封 → 旗舰 rubric 评估 │
   │  summon_card(card_id,     │   │ 提交 → 标准信封落库        │   │  → 「你的思维印记」        │
   │     reason, nudge_text)   │   │                            │   │  (river 异步 · 里程碑触发) │
   └───────────────────────────┘   └────────────────────────────┘   └──────────────────────────┘
```

**三层解耦**让每一层都能独立演化：

- **决策层** 只做一个决定，并通过**单一函数** `summon_card(card_id, reason, nudge_text)` 接线。**它、系统 prompt、refeed、整个 turn loop 都在服务端**。
- **Card Runtime** 是 **schema 驱动**的——卡的 spec 是数据，不是代码。
- **沉淀层** 把所有标准信封长成过程树 + 喂给评估，二者共享同一个地基。

### 前后端分离 · 密钥只在服务端

本仓库是一个**前后端分离的平台**：

- **前端 `apps/web`** 是**纯渲染 + API 客户端**——它不持有任何密钥、不直连模型。所有 LLM 调用都走后端网关。
- **后端 `apps/api`（Go）** 是**智能网关**：唯一持有密钥、唯一访问数据库与模型的单元。系统 prompt / `summon_card` 决策 / refeed / turn loop 全在这里，每一轮的档位 + token + 成本落在 `message` 行。
- **密钥（LLM key / DB DSN / session secret / SMTP）只活在 `apps/api` 的服务端环境变量里**——绝不进 git、日志、抛出或渲染的错误、数据存储或评估 payload。
- 模型 China-first：默认 **DeepSeek**，Anthropic 可选。平台持有 key，经 `keyResolver` 接缝预留未来按组织计费；**陪练可走中档模型、评估走旗舰模型绝不降级**。

> **组织不变式：** 每个账号必属于某个学校；学生 / 教师还属于 ≥1 个班级，不存在「无组织账号」。注册须携带有效班级 join code，建号与 `enrollments` 在同一事务内完成。结构归属（学校 / 班级）与「付费权益」是两回事——付费权益暂不建表，由服务端 `HasEntitlement(ctx, user)` 接缝（当前恒 `true`）在消耗 token 的端点前判定，未来接订阅 / 代币模型。

---

## 🗂 仓库结构

pnpm monorepo（前端 + 契约）+ 独立 Go module（后端）：

```
mind-imprint/
├── packages/contracts/          # 单一真相源：Zod schema + 卡库（前后端共享）
│   ├── src/
│   │   ├── primitives.ts         # 字段原语（discriminated union on `type`）
│   │   ├── cardSpec.ts           # 一张卡的 schema
│   │   ├── registry.ts           # 33 张卡的注册表（决策层目录从它派生）
│   │   ├── envelope.ts           # 标准信封结构
│   │   ├── rubric.ts             # 十维认知量规（FULL_RUBRIC）
│   │   ├── cognitive-model.ts    # 印记两面 → 四类 → 十维的归类映射
│   │   ├── evaluation.ts         # 评估输出契约
│   │   ├── summonCard.ts         # summon_card 工具参数契约
│   │   └── refeed.ts             # 卡摘要回灌陪练
│   └── cards/                    # 33 张卡，每张一个 JSON（新增卡 = 新增 JSON）
├── apps/api/                     # Go 后端（智能网关 · 数据服务 · 鉴权 · 异步评估）
│   ├── cmd/api/                  # 入口：migrate / 服务 / 内嵌 river worker
│   └── internal/
│       ├── gateway/              # LLM 网关：deepseek / anthropic provider、SSE、keyResolver、计价
│       ├── agent/                # prompt / refeed / turn loop / 评估任务（EvaluateWorker）
│       ├── api/                  # HTTP handler：auth / tasks / turn / evaluate / 组织端 / 权限
│       ├── auth/                 # argon2id、session
│       ├── org/                  # 学校 / 班级 / 名单
│       ├── cards/                # go:embed 卡 spec 加载
│       ├── store/                # pgx pool + sqlc + goose migrations
│       ├── config/               # 12-factor env 加载
│       └── httpx/                # server、错误信封
├── apps/web/                     # React SPA（纯渲染 + API 客户端）
│   └── src/
│       ├── api/                  # 后端 API 客户端（auth / tasks / turn SSE / evaluate / 组织端）
│       ├── store/                # 客户端视图缓存（从 API 水合）
│       ├── agent/                # 对话 / 评估轮询编排
│       ├── cards/                # schema 驱动的卡渲染器 + 字段组件 + 信封 reducer
│       ├── workspace/            # 工作区：对话 / 过程树 / 卡 sheet / 评估揭示
│       ├── console/              # 教师 / 管理员组织端
│       ├── shell/                # 应用外壳：导航 / 鉴权 / 主目录 / 记录 / 设置
│       └── dev/                  # 开发期 harness（不在产品外壳里挂载）
└── docs/                         # 权威产品规格 + 架构 north-star
```

---

## 🚀 快速开始

**前置：** [Go](https://go.dev/dl/) ≥ 1.26、[Node](https://nodejs.org/) ≥ 20、[pnpm](https://pnpm.io/installation) ≥ 8、一个可用的 **PostgreSQL** 实例，以及一个 LLM API Key（DeepSeek 或 Anthropic）。测试用到 [testcontainers](https://testcontainers.com/) 需要 Docker。

```bash
# 1. 安装前端 / 契约依赖
pnpm install

# 2. 配置后端（拷贝模板 → 填写；该文件已被 gitignore）
cp apps/api/.env.example apps/api/.env.local
#   编辑 apps/api/.env.local：DATABASE_URL、CORS_ORIGINS、DEEPSEEK_API_KEY / ANTHROPIC_API_KEY

# 3. 建表 + 种子（学校 + 班级 + 一名 mock 学生 Phoebe）
cd apps/api && make migrate-up && cd ../..

# 4. 启动后端网关（内嵌 river worker）
cd apps/api && make run          # → http://localhost:8080
#   另开一个终端：

# 5. 配置并启动前端
cp apps/web/.env.example apps/web/.env.local   # 指向 http://localhost:8080
pnpm --filter web dev            # → http://localhost:5173
```

### 质量门禁

```bash
pnpm -r typecheck                # 前端 + 契约类型检查（tsc --noEmit）
pnpm -r test                     # 前端 + 契约测试（Vitest）
cd apps/api && make test         # Go 单元 + testcontainers 集成测试
```

> Go 集成测试通过 testcontainers 拉起临时 Postgres，需要本机 Docker（`DOCKER_HOST` 指向 docker socket）。重新生成 sqlc 代码：`cd apps/api && make sqlc`（macOS 上用 `CGO_ENABLED=0` 走 WASM parser）。

### 配置 LLM Key（服务端）

`apps/api/.env.local`（git-ignored）：

```dotenv
PORT=8080
DATABASE_URL=postgres://user:pass@localhost:5432/mindimprint?sslmode=disable
CORS_ORIGINS=http://localhost:5173
COOKIE_SECURE=false                          # 本地 http 开发需设为 false

# LLM provider keys（只在服务端，绝不发往浏览器）
DEEPSEEK_API_KEY=sk-...
ANTHROPIC_API_KEY=                           # 可选
```

> **推理模型（reasoning model）** 会先消耗一段「思考」token 再吐可见内容。若 `maxTokens` 太小，`content` 会为空 / JSON 被截断。网关已为各调用路径留足余量（陪练 vs 评估各有预算）；评估走**旗舰模型绝不降级**。

> 🔐 **安全：** API Key 只活在 `apps/api` 的服务端环境里。它不会被提交进 git、不会出现在日志或报错文本里、不会进入数据存储或评估 payload。浏览器永远拿不到它。`.env.*` 已在 `.gitignore`。

### 端到端演示脚本（验收主动脉）

> 注册 / 登录（凭班级 join code）→ 新建任务（粘文章链接）→ 陪练对话 → 服务端调 `summon_card` → 打开并填卡 → **过程树实时生长** → 里程碑触达自动入队评估 → river worker 十维打分 → 前端轮询 `GET /evaluation` → **「你的思维印记」分层揭示**（两面 → 四类 → 十维）→ 记录页能力沉淀。

---

## 🃏 工具卡系统

工具卡是 **schema 驱动**的：**新增一张卡 = 新增一份 JSON 配置，不改渲染器代码。** 这是「schema 驱动是否成立」的验证标准。卡 JSON 是**单一共享真相源**——前端构建期 `import`，Go 用 `go:embed` 嵌同一批文件。

一张卡用现有**字段原语**拼出来：

`text` · `textarea` · `single_choice` · `multi_choice` · `rating` · `repeatable_group` · `link_check` · `spectrum` · `criteria_check`，外加一个 `show_if` 条件显隐修饰符。

每张卡有三种**视觉态**：`建议工具卡`（学生确认是否打开）→ `现在轮到你想`（激活、采集事件）→ `已完成 · 已钉到过程树`（序列化为标准信封）。

> **卡库现状：33 张卡全部可交互（0 stub）**，覆盖知识工具、信息素养、AI 伦理、溯源与多视角、探究启动、反身性与元认知、AOK 等类别。其中 `sift_craap`（SIFT×CRAAP 信息核查）与 `concession`（让步段）为两张样例卡。

新增交互的成本阶梯（绝大多数情况停在第 1 步）：

1. **能用现有原语拼出来** → 只写一份卡 JSON。
2. 真需要全新交互 → 加一个字段原语：在 `primitives.ts` 的 union 里加类型、写组件、在 `fieldRegistry` 注册——然后才是写卡 JSON。

---

## 🎯 过程评估（那一刀）

这是产品的护城河。任务「告一段落」（里程碑触发）时，评估**异步入队**——river worker 让**旗舰模型**读完整对话 + 全部标准信封，按 **SOLO 四级**对**认知模型十维**独立打分，并写一段诊断式过程叙述。

十个维度归入**两面 · 四类**的认知模型：

| 面 | 类 | 维度 |
|---|---|---|
| 🚀 **生成式驾驭** | 意图与编排 | D1 提问清晰度 · D10 协作编排 |
| | 推理与论证 | D4 多视角与让步 · D5 论证拆解 · D7 论证质量 |
| 🛡️ **批判式防护** | 信息素养 | D2 信源辨识 · D3 横向验证 |
| | AI 元认知与边界 | D6 反思与元认知 · D8 信息再生产 · D9 AI 边界与伦理 |

每维打 **SOLO 四级**：`L1 萌芽` · `L2 发展中` · `L3 熟练` · `L4 卓越`；证据不足则记 `N/A`，中性呈现、不计入进度。

评估结果**只给学生本人看**，语气是诊断而非判断——描述「你做了什么」，而不是「你应该怎么想」。结果以「你的思维印记」**分层揭示**（两面 → 四类 → 十维），并遵循铁律 #2 **提供而不推送**：安静的指示器、无徽章、不自动弹开，由学生自己点开。

---

## 🧱 技术栈

| 层 | 选型 |
|---|---|
| 前端 `apps/web` | React 18 + Vite + TypeScript + Tailwind CSS —— 纯渲染 + API 客户端 |
| 后端 `apps/api` | **Go 1.26**（`net/http` + `pgx/v5` + `sqlc` + `goose` + `river`）—— 智能网关 · 数据服务 · 鉴权 · 异步评估 |
| 存储 | **PostgreSQL** —— `users`(含 `school_id`) / `sessions` / `schools` / `classes` / `enrollments` / `task` / `message`（每轮 token+成本落 assistant 行）/ `card_instance` / `evaluation` |
| 契约 | Zod（单一真相源，`@mind-imprint/contracts`）；卡 JSON 前端 `import` + Go `go:embed` 共享 |
| 模型 | China-first：默认 DeepSeek，Anthropic 可选；平台持有 key（服务端 env），评估走旗舰绝不降级 |
| 鉴权 / 组织 | argon2id + session cookie；学生 / 教师 / 管理员三角色 + RBAC；班级 join-code 注册门槛 |
| 异步评估 | `river` 任务队列 + 内嵌 worker（里程碑触发入队 → 前端轮询揭示） |
| 测试 | Vitest + Testing Library（前端）· Go test + testcontainers（后端集成） |
| 工具链 | pnpm workspace · `tsc --noEmit` 类型门禁 · goose migrations · sqlc 代码生成 |

---

## ✅ 状态与路线图

平台**端到端可跑通**：真后端 + Postgres + 真实鉴权 + 组织端 + 异步评估全部落地并合入 `main`。

- ✅ **P1 · 后端地基 + 智能网关** — Go 服务、Postgres + goose、信封落库、SSE 网关、prompt / refeed / turn loop 移植、每轮用量落 message 行、`HasEntitlement` 桩。
- ✅ **P2 · 鉴权 + 最小组织** — 注册 / 邮箱验证 / 登录、argon2id、session、班级 join-code 门槛、任务归属到人。
- ✅ **P3 · 完整组织端** — 教师 / 管理员登录、建班 + 名单管理、CSV 导入、师资分配、学校维度聚合、RBAC。
- ✅ **P4 · 异步评估** — river 任务 + 内嵌 worker、里程碑自动触发入队、前端轮询、「你的思维印记」分层揭示 UI。
- ✅ 33 张卡全部可交互，0 stub。
- 🔭 **下一步：** 计费 / 权益方案设计（填 `HasEntitlement` 接缝）。
- 🔭 非阻塞增强（登记于 `docs/遗留项追踪_Carryforward.md`）：评估语义扩展、若干 river / 事务 / 并发的收尾项、a11y / 整洁项。

### 明确不做（范围决定，非遗漏）

- ❌ 文档编辑器 / 作业撰写排版 / 提交功能（过程树是**只读**记录，我们不拥有学生的文档）。
- ❌ 上瘾式游戏化：连胜、排行榜、徽章、推送一律不做。

---

## 📚 文档

权威产品规格与架构 north-star 都在 `docs/`：

- **`docs/思维印记_Demo_PRD.md`** — 产品 / 不变骨架（产品理念冲突时以 PRD 为准）。
- **`docs/superpowers/specs/2026-06-24-backend-platform-architecture-design.md`** — 后端平台架构 north-star（涉及后端 / DB / API / 鉴权 / 组织 / 异步评估以架构文档为准）。
- **`docs/architecture/`** — `database-schema` · `api-design` · `go-backend-best-practices`。
- **`docs/工具包库/`** — 工具卡方法论全库。
- **`docs/design/`** — 设计稿（`.dc.html` 对 UI 具有约束力）。
- **`docs/遗留项追踪_Carryforward.md`** — 逐项遗留追踪，保证端到端不丢。
- **`AGENTS.md`** — 给 AI 协作者（Claude Code / Codex / Gemini）的硬约束摘要。

---

## 🤝 贡献

1. 改动前先读 `AGENTS.md` 的「硬约束」与四条设计铁律。
2. 卡 spec 的单一真相源在 `packages/contracts` 的 registry——决策层目录从它派生，不手写第二份。
3. **客户端绝不直连模型**，密钥只在 `apps/api` 服务端。
4. 提交前确保 `pnpm -r typecheck`、`pnpm -r test` 与 `cd apps/api && make test` 全绿。
5. **共享记忆只写进 `AGENTS.md`**（`CLAUDE.md` / `GEMINI.md` 只 import 它，勿直接改）。

---

## 📄 许可证

本仓库为内部 demo，暂未指定开源许可证。
