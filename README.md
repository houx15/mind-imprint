<div align="center">

# 思维印记 · Mind Imprint

**一个把「思考过程」本身变成产品的 AI 学习平台。**

学生带着真实的任务进来，在与 AI 的协作中被引导做结构化思考——全过程被记录、被评估，最后长成一枚只属于他自己的「思维印记」。

[![Live](https://img.shields.io/badge/status-LIVE-brightgreen)](https://mind-web.uni-robot.cn)
[![Go](https://img.shields.io/badge/Go-1.26-00ADD8?logo=go&logoColor=white)](https://go.dev/)
[![PostgreSQL](https://img.shields.io/badge/PostgreSQL-16-4169E1?logo=postgresql&logoColor=white)](https://www.postgresql.org/)
[![TypeScript](https://img.shields.io/badge/TypeScript-5.4-3178C6?logo=typescript&logoColor=white)](https://www.typescriptlang.org/)
[![React](https://img.shields.io/badge/React-18-61DAFB?logo=react&logoColor=black)](https://react.dev/)
[![Vite](https://img.shields.io/badge/Vite-5-646CFF?logo=vite&logoColor=white)](https://vitejs.dev/)
[![DeepSeek](https://img.shields.io/badge/LLM-DeepSeek_V4-4D6BFE)](https://www.deepseek.com/)
[![Docker](https://img.shields.io/badge/deploy-Docker_Compose-2496ED?logo=docker&logoColor=white)](https://docs.docker.com/compose/)

[线上环境](#-线上环境) · [设计铁律](#-四条设计铁律) · [架构](#-架构核心心智模型) · [快速开始](#-快速开始) · [双轴评估](#-过程评估双轴模型) · [部署](#-部署) · [路线图](#-状态与路线图)

</div>

---

## 这是什么

市面上的 AI 工具帮学生**更快地完成作业**。思维印记反过来——它帮学生**更深地完成思考**。

形态上它像 Cowork / Codex 这类 agent 工具，但封装了两层别处没有的能力：

1. **交互式「思维工具卡」** — 在对话里的对的时刻，AI 把一张结构化的思考脚手架塞回给学生（溯源、让步段、论证拆解……），由学生亲手填，而不是 AI 替他想。
2. **「过程评估」** — 完整的对话 + 每一次填卡都被序列化为标准信封，长成一棵**只读的过程树**，最后由旗舰模型按 **AI 批判性思维双轴模型**（深度轴 D1–D6 × L1–L4 · 自主轴 A1–A6 × 0–5 带）生成一段诊断式的过程叙述。

学生有三个真实的协作**面**，每一面都各自被评估：**工作室（项目式长研究）· 课程（引导式单课）· 陪练对话**。工作室本身是一个**四房间工作区**——**项目管理 · 阅读 · 写作 · 回顾**——其中「阅读」室是一张两层的**兔子洞地图**（文献探索图谱，见下文）。

> **我们占的是「思考」这一层，不是「作业被完成、被提交」的地方。** 学生的作文活在他自己的世界里（Google Doc、本子、随手粘进来），我们不拥有那个文档——我们只拥有他思考的轨迹。

锚定场景：**Phoebe 在写「中国是否让地球变得更可持续？」**，想直接引用一篇公众号文章当证据。AI 没有替她判断真假，而是召唤 `CRAAP` 溯源体检卡，陪她把来源溯到 NASA 与 *Nature Sustainability*；她在「阅读」室的**兔子洞地图**上把问题一条条挖开，让印记顺着每个问题从 OpenAlex 找相关文献、采纳进自己的证据网；当她撞上「中国碳排放全球第一」这个反例时，AI 又召唤「让步段」卡，陪她把反例正面接住。整个过程被记录、被评估。

---

## 🌐 线上环境

平台**已上线并端到端跑通**（Aliyun ECS · Docker Compose · certbot HTTPS）：

| 用途 | 地址 |
|---|---|
| Web（学生 / 教师 / 管理员 · 一份按角色路由的 SPA） | **https://mind-web.uni-robot.cn** |
| API（Go 智能网关 · 唯一持密钥单元） | **https://mind-api.uni-robot.cn** |
| 体验入口（自动登录为种子学生 Phoebe） | `https://mind-web.uni-robot.cn/?trial=1` |

前端**不是三个独立应用**——`apps/web` 是**一个按角色路由的 SPA**：登录后由后端返回的 `role` 决定渲染学生端还是教师 / 管理员控制台。因此对外只有 **两个** 服务：API + 一份 web bundle。部署细节见 [部署](#-部署) 与 `docs/deploy/`。

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
   │                           │   │ 事件采集                   │   │ 全部信封 → 旗舰双轴评估    │
   │  summon_card(card_id,     │   │ 提交 → 标准信封落库        │   │  → 「你的思维印记」        │
   │     reason, nudge_text)   │   │                            │   │  (按需触发 · 旗舰不降级)   │
   └───────────────────────────┘   └────────────────────────────┘   └──────────────────────────┘
```

**三层解耦**让每一层都能独立演化：

- **决策层** 只做一个决定，并通过**单一函数** `summon_card(card_id, reason, nudge_text)` 接线。**它、系统 prompt、refeed、整个 turn loop 都在服务端**。
- **Card Runtime** 是 **schema 驱动**的——卡的 spec 是数据，不是代码。
- **沉淀层** 把所有标准信封长成过程树 + 喂给评估，二者共享同一个地基。

### 三个协作面，一套评估模型

学生在三个不同的**面**里与 AI 协作，每一面都产出同构的标准信封，共用同一套双轴评估：

| 面 | 是什么 | 评估端点 |
|---|---|---|
| **工作室 Studio** | 项目式长研究（粘链接、多轮陪练、过程树生长） | `GET /projects/{id}/assessment` |
| **课程 Course** | 引导式单课（锚点驱动、schema 渲染的课程运行时） | `GET · POST /courses/{id}/session/assessment` |
| **陪练对话 Chat** | 轻量单线对话 | `GET · POST /chat/threads/{id}/assessment` |

> **成本纪律：** `GET` 读取已存结果**不花 token**；`POST` 才调用旗舰模型生成叙述并花费（首次生成、后续复用）。这条「读免费 / 写才花钱」贯穿所有评估与报告端点。

### 工作室的四个房间

工作室不是单一对话框，而是一个按研究流程组织的**四房间工作区**（`apps/web/src/workspace/`，一份 `blocks/` 组件覆盖）：

| 房间 | 是什么 |
|---|---|
| **项目管理** | 甘特图（默认）+ 看板 + 活动日志，把长研究拆成任务、进出各房间。回来时先落在这里做进度**回顾**，一键「继续工作」回到当前状态。 |
| **阅读** | 两个视图：**图书馆**（收藏的文献，带标签与 待读/在读/读完 状态）+ **探索**（下述**兔子洞地图**，写正文阶段它就是**证据地图**）。主区有「检索方向」引导盒——印记按你的问题给 2–3 个关键词 + 理由、一键搜。 |
| **写作** | 一条**状态机式的写作旅程**（详见下）：写**研究提案** → 建**证据地图** → 写**正文陈述**（逐条论点）→ **成文定稿**。AI 只陪想、查论证与结构（**绝不代写正文**），一次一问，像老师一样给**分层批注**（总体/段落/句子 · 绿蓝红）。回顾处**完成 → 过程评估**，成品可**导出**（.docx / .xlsx）带走。 |
| **回顾** | 左边四个只读页签（**活动日志 / 研究框架 / 提案 / 成品**）对照过程，右边学生自己写反思；再触发过程评估（「你的思维印记」）。 |

### 写作旅程（状态机）

写作不是一个大文本框，而是一条**确定性状态机**——`docs/2026-08-09-all-statuses.md` 是它的行为单一真相源。**AI 的职责收窄为三件事：教（引导）/ 关（内容够不够格进下一步）/ 显（在对的时刻递上对的思维卡）**。每个状态的界面是**预先设计好、已知的**，agent 不临场发明流程；稳定性来自三层：**确定性状态机（流程）+ 快模型陪练（引导）+ 推理模型审阅（把关）**。

| 状态 | 学生在做什么 |
|---|---|
| **立项 / 研究框架** | 一次一件把 目标 / 缘由 / 活动与时间 / 资源 / 反例 聊清楚；卡壳时印记递「提问卡」（子代理）。四件事齐了自动生成计划。 |
| **写研究提案** | 自由写 or **一步步引导**——每部分一张**引导写作卡**（清晰引导问题 + 会长高的输入框 + 我依然有问题/我写好了 两个按钮）；研究计划部分先定 2–4 个**子问题**，再逐条写。写作面有「还需要探索的」盒可跳去阅读室。 |
| **建证据地图（研究）** | 在阅读室顺着子问题找材料，给每篇标 支持/反驳、必读红/待定黄；印记按**饱和度**审阅每个子问题的证据是否够写。 |
| **正文陈述** | 大纲（可改子问题，印记区分「换措辞」还是「换了问题」并提醒）→ 逐条论点写论述段（PEE/论证卡）→ 综合 → 面对反方 → 结论 → 论证结构。 |
| **成文定稿** | 引言 → 结论 → 把各部分**拼成全文** → 通篇**润色体检**循环 → 完成 → 解锁回顾。 |

### 阅读室的兔子洞地图（文献探索）

「阅读 → 探索」是一张**两层的思维地图**，把「找文献」变成「让论证长出来」，而不是堆一摞 PDF：

- **顶层 = 问题图（兔子洞地图）** — 节点是学生**要追的问题**（根问题），用 [React Flow](https://reactflow.dev/) 渲染成可拖拽、可缩放的椭圆节点（每个根问题一种配色）。问题之间由**带标签的关系边**相连（`子问题 / 支持 / 反驳·张力 / 细化 / 依赖·前提` 固定词表）——**印记提议关系、学生确认**（铁律 #2），学生也可拖一个问题到另一个上手动连边。
- **钻进一个问题 = 它的文献 mindmap（Level-2）** — 问题居中，采纳的论文作为子节点向外生长（dagre 自动排布）；右侧统一侧栏三态切换：**印记陪聊（默认）→ 节点元信息 → 搜索结果**，每个非默认态都能返回陪聊。
- **深挖 = 服务端调 OpenAlex**（免费、无需 key）：对一个问题「找相似文献」（印记先把中文问题**提炼成英文关键词**再检索），对一篇论文可「找它引用的 / 找引用它的」（references / cited-by）。结果先进侧栏**候选托盘**，学生**采纳**才落到地图上（铁律 #1）——采纳的论文自动进图书馆。DOI 元信息经 Crossref 物化。

> 数据地基复用既有 `exploration_lead`（节点）+ `reference`（论文），新增 `question_edge`（带标签关系边）；每次真实 LLM 调用（问题提炼 / 关系提议）照常记档位 + token + 成本，OpenAlex 检索本身不是 LLM、不计费。

### 前后端分离 · 密钥只在服务端

本仓库是一个**前后端分离的平台**：

- **前端 `apps/web`** 是**纯渲染 + API 客户端**——它不持有任何密钥、不直连模型。所有 LLM 调用都走后端网关。
- **后端 `apps/api`（Go）** 是**智能网关**：唯一持有密钥、唯一访问数据库与模型的单元。系统 prompt / `summon_card` 决策 / refeed / turn loop / 语音 全在这里，每一轮的档位 + token + 成本落库。
- **密钥（LLM key / DB DSN / 语音凭据 / SMTP）只活在 `apps/api` 的服务端环境变量里**——绝不进 git、日志、抛出或渲染的错误、数据存储或评估 payload。
- 模型 China-first：Anthropic 可选。平台持有 key，经 `keyResolver` 三条接缝分档：**陪练/引导走快模型 `deepseek-v4-flash`**（`FastChatResolver`，更快更省）· 线程压缩走中档 `deepseek-v4-pro`（`ChatResolver`）· **审阅（框架把关 / 生成计划 / 整稿体检 / 证据地图饱和 / 评估）走旗舰 `deepseek-v4-pro`（`EvalResolver`）——绝不降级**。每轮记档位 + token + 成本落库。
- 语音 China-first：**火山引擎** TTS（`seed-tts-2.0`）+ ASR（`volc.bigasr`），同样只在服务端持凭据。

> **组织不变式：** 每个账号必属于某个学校；学生 / 教师还属于 ≥1 个班级，不存在「无组织账号」。注册须携带有效班级 join code（教师须凭管理员签发的邀请码），建号与 `enrollments` 在同一事务内完成。结构归属（学校 / 班级）与「付费权益」是两回事——付费权益暂不建表，由服务端 `HasEntitlement(ctx, user)` 接缝（当前恒 `true`）在消耗 token 的端点前判定，未来接订阅 / 代币模型。

---

## 🗂 仓库结构

pnpm monorepo（前端 + 契约 + 营销站）+ 独立 Go module（后端）：

```
mind-imprint/
├── packages/contracts/          # 单一真相源：Zod schema + 卡库（前后端共享）
│   ├── src/
│   │   ├── primitives.ts         # 字段原语（discriminated union on `type`）
│   │   ├── cardSpec.ts           # 一张卡的 schema
│   │   ├── registry.ts           # 34 张卡的注册表（决策层目录从它派生）
│   │   ├── envelope.ts / event.ts / graph.ts   # 标准信封 · 事件 · 过程树投影
│   │   ├── dualaxis.json         # 权威双轴评估模型（深度 D1–D6 · 自主 A1–A6 · 六透镜）
│   │   ├── dualAxisReport.ts     # 单次会话双轴报告契约
│   │   ├── ability.ts            # 能力素养模型（六辐能力雷达 · 两轴永不合成）
│   │   ├── growthHistory.ts / collectedCards.ts  # 成长报告：学习记录 · 工具卡
│   │   ├── parentReport.ts       # 家长报告（项目式 + 阶段式）
│   │   ├── course.ts / courseSkill.ts / anchor.ts # 课程运行时 · 锚点
│   │   ├── chat.ts / summonCard.ts / refeed.ts    # 陪练 · summon_card · 回灌
│   │   ├── exploration.ts / reference.ts          # 兔子洞地图：问题节点·带标签边·dig 候选 · 文献库
│   │   └── skill.ts / rubric.ts / studioState.ts  # 技能 · 量规 · 工作室状态
│   └── cards/                    # 34 张卡，每张一个 JSON（新增卡 = 新增 JSON）
├── apps/api/                     # Go 后端（智能网关 · 数据服务 · 鉴权 · 评估）
│   ├── cmd/api/                  # 入口：migrate / 服务
│   └── internal/
│       ├── gateway/              # LLM 网关：deepseek / anthropic provider、SSE、keyResolver、计价
│       ├── voice/                # 火山 TTS / ASR
│       ├── agent/                # prompt / refeed / turn loop / 评估编排
│       ├── studio/ course/ chat/ # 三个协作面的领域逻辑
│       ├── api/                  # HTTP handler：auth / 三面 / 评估 / 组织端 / 教师端 / 权限
│       ├── ability/ rubric/      # 能力素养 · 双轴量规
│       ├── teacher/ parent/      # 班级周报 · 学生报告 · 家长报告
│       ├── auth/ org/            # argon2id · session · 学校 / 班级 / 名单
│       ├── cards/ skills/ onboarding/ materialize/  # go:embed 卡 · 技能 · 引导 · 文献物化（OpenAlex 检索 + Crossref DOI）
│       ├── store/                # pgx pool + sqlc + goose migrations
│       └── config/ httpx/        # 12-factor env · server · 错误信封
├── apps/web/                     # React SPA（纯渲染 + API 客户端，一份 bundle 覆盖三角色）
│   └── src/
│       ├── api/                  # 后端 API 客户端（auth / 三面 / 评估 / 组织端 / 报告 / 探索）
│       ├── workspace/            # 工作室四房间：项目管理 / 阅读 / 写作 / 回顾
│       │   └── blocks/exploration/  # 兔子洞地图（React Flow）：地图·问题 mindmap·统一侧栏·dagre 排布
│       ├── studio/               # 工作室共享件：对话 / 过程树 / 卡 sheet / 阅读室 / 评估揭示
│       ├── cards/ primitives/    # schema 驱动的卡渲染器 + 字段组件 + 信封 reducer
│       ├── console/              # 教师 / 管理员组织端 + 班级周报 + 报告导出
│       ├── audio/                # 语音录制 / 播放
│       ├── shell/                # 应用外壳：导航 / 鉴权 / 主目录 / 成长报告 / 设置
│       └── dev/                  # 开发期 harness（不在产品外壳里挂载）
├── apps/site/                    # 思维印记营销站（Astro 静态站 · zh/en 双语）
├── apps/peraspera/              # Per Aspera 母品牌营销站（独立 Astro app · 部署 Vercel）
├── deploy/                       # 生产编排：docker-compose + nginx 站点 + 镜像助手（无密钥）
└── docs/                         # 权威产品规格 + 架构 north-star + 部署运行手册
```

---

## 🚀 快速开始

**前置（版本与 Docker 构建对齐，请勿放宽为「≥ 某版本」）：** [Go](https://go.dev/dl/) **1.26**（`apps/api/go.mod` + `golang:1.26` 构建镜像）、[Node](https://nodejs.org/) **22**（生产镜像 `node:22-bookworm-slim`）、[pnpm](https://pnpm.io/installation) **10.29.3**（由根 `package.json` 的 `packageManager` 字段精确钉住，与 `pnpm-lock.yaml` 及 web 镜像里的 `corepack prepare pnpm@10.29.3` 一致）、一个可用的 **PostgreSQL 16** 实例，以及一个 LLM API Key（DeepSeek 或 Anthropic）。测试用到 [testcontainers](https://testcontainers.com/) 需要 Docker。

```bash
# 0. 启用 corepack —— 读 packageManager 字段自动启用 pnpm 10.29.3（无需手动装 pnpm）
corepack enable

# 1. 安装前端 / 契约依赖（--frozen-lockfile 与 CI/Docker 一致）
pnpm install --frozen-lockfile

# 2. 配置后端（拷贝模板 → 填写；该文件已被 gitignore）
cp apps/api/.env.example apps/api/.env.local
#   编辑 apps/api/.env.local：DATABASE_URL、CORS_ORIGINS、DEEPSEEK_API_KEY / ANTHROPIC_API_KEY

# 3. 建表 + 种子（学校 + 班级 + 一名 mock 学生 Phoebe + wu.teacher + admin）
cd apps/api && make migrate-up && cd ../..

# 4. 启动后端网关
cd apps/api && make run          # → http://localhost:8080
#   另开一个终端：

# 5. 配置并启动前端
cp apps/web/.env.example apps/web/.env.local   # 指向 http://localhost:8080
pnpm --filter web dev            # → http://localhost:5173
```

种子登录：`phoebe@demo.mindimprint.local` / `wu.teacher@demo.mindimprint.local`（密码 `phoebe-dev-pass`）；或访问 `?trial=1` 自动登录为 Phoebe。

### 质量门禁

```bash
pnpm -r typecheck                # 前端 + 契约类型检查（tsc --noEmit）
pnpm -r test                     # 前端 + 契约测试（Vitest）
cd apps/api && make test         # Go 单元 + testcontainers 集成测试
```

> Go 集成测试通过 testcontainers 拉起临时 Postgres，需要本机 Docker（`DOCKER_HOST` 指向 docker socket）。重新生成 sqlc 代码：`cd apps/api && make sqlc`（macOS 上用 `CGO_ENABLED=0` 走 WASM parser）。
>
> 另有一套**真实 live E2E 套件**（打真后端 + 真 DeepSeek key），用于捕获 mock 套件抓不到的 LLM 输出形状漂移与语音链路问题。

### 配置 LLM / 语音 Key（服务端）

`apps/api/.env.local`（git-ignored）：

```dotenv
PORT=8080
DATABASE_URL=postgres://user:pass@localhost:5432/mindimprint?sslmode=disable
CORS_ORIGINS=http://localhost:5173
COOKIE_SECURE=false                          # 本地 http 开发需设为 false

# LLM provider keys（只在服务端，绝不发往浏览器）
DEEPSEEK_API_KEY=sk-...
ANTHROPIC_API_KEY=                           # 可选

# 语音（火山引擎，可选）
VOICE_APP_ID=
VOICE_ACCESS_KEY=
VOICE_TTS_VOICE=
```

> **推理模型（reasoning model）** 会先消耗一段「思考」token 再吐可见内容。若 `maxTokens` 太小，`content` 会为空 / JSON 被截断。网关已为各调用路径留足余量（陪练 vs 评估各有预算）；评估走**旗舰模型绝不降级**。

> 🔐 **安全：** API Key 与语音凭据只活在 `apps/api` 的服务端环境里。它们不会被提交进 git、不会出现在日志或报错文本里、不会进入数据存储或评估 payload。浏览器永远拿不到它们。`.env.*` 已在 `.gitignore`。

### 端到端演示脚本（验收主动脉）

> 注册 / 登录（凭班级 join code）→ 新建任务（粘文章链接）→ 陪练对话 → 服务端调 `summon_card` → 打开并填卡 → 进「阅读」室在**兔子洞地图**上追问题、让印记从 OpenAlex 深挖文献、采纳进证据网、印记提议问题间关系 → 进「写作」室自己写正文（AI 只查论证）→ **过程树实时生长** → 打开评估触发旗舰双轴打分 → **「你的思维印记」分层揭示**（深度轴 × 自主轴）→ 成长报告沉淀能力素养 → 教师端看班级周报 / 学生报告 / 导出家长报告。

---

## 🃏 工具卡系统

工具卡是 **schema 驱动**的：**新增一张卡 = 新增一份 JSON 配置，不改渲染器代码。** 这是「schema 驱动是否成立」的验证标准。卡 JSON 是**单一共享真相源**——前端构建期 `import`，Go 用 `go:embed` 嵌同一批文件。

一张卡用现有**字段原语**拼出来：

`text` · `textarea` · `single_choice` · `multi_choice` · `rating` · `repeatable_group` · `link_check` · `spectrum` · `criteria_check`，外加一个 `show_if` 条件显隐修饰符。

每张卡有三种**视觉态**：`建议工具卡`（学生确认是否打开）→ `现在轮到你想`（激活、采集事件）→ `已完成 · 已钉到过程树`（序列化为标准信封）。

> **卡库现状：34 张卡全部可交互（0 stub）**，覆盖知识工具、信息素养、AI 伦理、溯源与多视角、探究启动、反身性、AOK 等类别。其中 `craap`（CRAAP 溯源体检）与 `concession`（让步段）为两张样例卡。

新增交互的成本阶梯（绝大多数情况停在第 1 步）：

1. **能用现有原语拼出来** → 只写一份卡 JSON。
2. 真需要全新交互 → 加一个字段原语：在 `primitives.ts` 的 union 里加类型、写组件、在 `fieldRegistry` 注册——然后才是写卡 JSON。

---

## 🎯 过程评估（双轴模型）

这是产品的护城河。任务告一段落时，学生打开评估——**旗舰模型**读完整对话 + 全部标准信封，按 **AI 批判性思维双轴模型**给出诊断式过程叙述。权威定义在 `packages/contracts/src/dualaxis.json`。

**公理：两轴永不合成总分；单次会话是事件级证据，不构成人级档位判定。**

### 深度轴（Depth · D1–D6，每维 SOLO 式 L1–L4）

学生想得有多深、多结构化。

| 维 | 名称 | 一句话 |
|---|---|---|
| D1 | 任务理解与问题表述 | 把宽泛话题收窄成真正能研究、能回答的问题 |
| D2 | 证据与信源 | 会不会找资料、判断可信度、说清每份能支持什么 |
| D3 | 论证结构 | 用证据一步步推出结论，而非「结论大于证据」 |
| D4 | 视角与偏见 | 有没有考虑反方、承认研究的局限和偏差 |
| D5 | 反馈处理与修订 | 收到质疑后是真改进论证，还是只改措辞 |
| D6 | 反思与元认知 | 能否回头审视思路、说清判断从何而来（只认学生**亲手写**的反思，否则记 NA） |

`L1 萌芽` · `L2 发展中` · `L3 熟练` · `L4 卓越`；证据不足记 `N/A`，中性呈现、不判低分。

### 自主轴（Autonomy · A1–A6，行为计数带 0–5）

学生在多大程度上**自己掌舵**——不是质量分，而是**可计入事件的计数带**。

| 维 | 名称 | 一句话 |
|---|---|---|
| A1 | 方向自主 | 是不是自己决定研究方向 |
| A2 | 发起自主 | 会不会主动去查证、补证据 |
| A3 | 边界主权 | 用 AI 时会不会设边界（不要代写 / 不要编来源） |
| A4 | 对抗与检验 | 会不会主动请人挑刺、找自己论证的问题 |
| A5 | 判断署名 | 愿不愿意为结论负责，说清「这是我的判断」 |
| A6 | 求真优先 | 证据不支持时，愿不愿意把结论改小而不是硬撑 |

> **机会供给先于判定：** `not_supplied`（平台还没提供这个锻炼机会）记为**平台欠账**、不算学生短板、渲染「暂无 · 机会未提供」、不拉低画像；只有 `given_not_taken`（机会给了没用）才算学生信号（0/1 级）。

### 六透镜（Prompt Lenses）

只读 AI 互动痕迹，为双轴**补过程证据**——**不是第三根评分轴，不并入任何总分**：五个决定完整度 · 提示成熟度 · 边界意识 · 对手邀请 · 主动指令率 · 验收标准自给。

评估结果**只给学生本人看**，语气是诊断而非判断——描述「你做了什么」，而不是「你应该怎么想」，并遵循铁律 #2 **提供而不推送**：安静的指示器、无徽章、不自动弹开。模型还带 **AP Research** 标准对齐口径（Academic Paper 1–5 档、POD /24、真实性），只作参考、不与双轴合成。

---

## 👀 成长报告 · 教师端 · 家长报告

同一套双轴模型，投影成不同受众的视图——**核心对象只有一个，按受众投影**：

**学生 · 成长报告**（跨会话沉淀，三个 tab，全部 `GET` 免费读）：

- **学习记录** — 历次会话与过程树（`GET /growth/history`）
- **工具卡** — 收集到的工具卡图鉴（`GET /growth/cards`）
- **能力素养** — **六辐能力雷达**（对齐深度六维）+ 诚实的自主标签（`GET /growth/ability`）

**教师端**（`teacherOrAdmin` 鉴权，严格跨班隔离）：

- **学生报告** — 按面读某学生某次会话（`GET …/students/{id}/reports/{surface}/{scopeId}`）
- **班级周报** — 班级维度聚合（`GET …/weekly-report`；`POST …/weekly-report/prose` 生成叙述）
- **家长报告** — 项目式（`…/parent-report/{surface}/{scopeId}`）+ 阶段式周报（`…/parent-stage-report/{weekStart}`），可导出

所有报告端点都遵守「读免费 / 写才花钱」：数字实时算，LLM 叙述首次 `POST` 生成后复用。

---

## 🚢 部署

生产以 **Docker Compose** 部署到一台 ECS，宿主机 nginx 终止 TLS。所有编排文件在 `deploy/`，**均不含密钥**：

```
                Internet (HTTPS)
                      │
             ┌────────┴─────────┐
             │   host nginx     │   ← TLS 终止 (certbot)，与既有站点共存
             └───┬──────────┬───┘
   mind-api ─────┘          └───── mind-web
   127.0.0.1:8090            127.0.0.1:8091
        │                         │
  ┌─────┴─────┐             ┌─────┴──────┐
  │ api (Go)  │             │ web (nginx │
  │  :8080    │             │  static)   │
  └─────┬─────┘             └────────────┘
        │  compose network
  ┌─────┴─────┐
  │ db (pg16) │  ← 仅 compose 网络内可达，不发布主机端口
  └───────────┘
```

- `deploy/docker-compose.prod.yml` — `db`（postgres:16 · 命名卷 · 不发布端口）+ `api`（distroless）+ `web`（node 构建 → nginx 静态托管），全部 `restart: unless-stopped`。
- `deploy/env.prod.example` — 环境变量样例（复制为 `deploy/.env.prod` 填真值；已 gitignore）。
- `deploy/nginx/mind-{api,web}.uni-robot.cn.conf` — 宿主机反代站点（API 侧带 `$http_upgrade` map 走 SSE + 语音 WS，`proxy_read_timeout 300s` 兜住旗舰评估耗时）。
- **迁移 + 种子**：`docker compose run --rm --build api -migrate-up`（goose · 幂等）。
- **跨子域鉴权**：`mind-web` 与 `mind-api` 同属 `uni-robot.cn`（same-site），会话 Cookie `mk_session` 为 `Secure; HttpOnly; SameSite=Lax`，配合 API 的精确 `CORS_ORIGINS` 白名单跨子域携带凭证。
- **中国网络注意**：首次构建若报 `not found` / `gcr.io` 拉不动，先跑 `bash deploy/pull-base-images.sh`（经 DaoCloud 代理拉基础镜像并重打成规范名）；Go 走 `GOPROXY=goproxy.cn`，web 镜像固定 `corepack prepare pnpm@10.29.3`。

运行手册在 `docs/deploy/`：`README`（总览）· `api` · `web` · `tls`。

---

## 🧱 技术栈

| 层 | 选型 |
|---|---|
| 前端 `apps/web` | React 18 + Vite + TypeScript + Tailwind CSS —— 纯渲染 + API 客户端（一份 bundle 覆盖三角色）；兔子洞地图用 **React Flow（@xyflow/react）+ dagre** |
| 后端 `apps/api` | **Go 1.26**（`net/http` + `pgx/v5` + `sqlc` + `goose`）—— 智能网关 · 数据服务 · 鉴权 · 评估 · 语音 |
| 存储 | **PostgreSQL 16** —— `users`(含 `school_id`) / `sessions` / `schools` / `classes` / `enrollments` / 三面项目 / `message`（每轮 token+成本落行）/ `card_instance` / `evaluation` / `reference`(文献库) / `exploration_lead`(问题节点) / `question_edge`(带标签关系边) |
| 契约 | Zod（单一真相源，`@mind-imprint/contracts`）；卡 JSON 前端 `import` + Go `go:embed` 共享 |
| 模型 | China-first：默认 **DeepSeek V4-pro**（陪练 + 旗舰两档同名 model，评估绝不降级），Anthropic 可选；平台持有 key（服务端 env） |
| 语音 | **火山引擎** TTS（`seed-tts-2.0`）+ ASR（`volc.bigasr`），凭据只在服务端 |
| 文献探索 | **OpenAlex**（相似 / references / cited-by · 免费无 key）+ **Crossref**（DOI 元信息）· 服务端调用，检索本身不计费；查询提炼走陪练 LLM |
| 鉴权 / 组织 | argon2id + session cookie；学生 / 教师 / 管理员三角色 + RBAC；班级 join-code + 教师邀请码注册门槛 |
| 评估 | 旗舰双轴模型按需触发（`GET` 读免费 / `POST` 写才花钱），首生成后复用 |
| 测试 | Vitest + Testing Library（前端）· Go test + testcontainers（后端集成）· 真实 live E2E 套件 |
| 部署 | Docker Compose（db + api + web）+ 宿主机 nginx + certbot HTTPS · Aliyun ECS |
| 工具链 | **pnpm 10.29.3**（corepack 钉住）workspace · **Node 22** · `tsc --noEmit` 类型门禁 · goose migrations · sqlc 代码生成 |

---

## ✅ 状态与路线图

平台**已上线、端到端可跑通**：真后端 + Postgres + 真实鉴权 + 组织端 + 三面评估 + 教师端 + 家长报告 + 语音全部落地并合入 `main`，并已部署到生产（见 [线上环境](#-线上环境)）。

- ✅ **P1 · 后端地基 + 智能网关** — Go 服务、Postgres + goose、信封落库、SSE 网关、prompt / refeed / turn loop 移植、每轮用量落 message 行、`HasEntitlement` 桩。
- ✅ **P2 · 鉴权 + 最小组织** — 注册 / 登录、argon2id、session、班级 join-code + 教师邀请码门槛、任务归属到人。
- ✅ **P3 · 完整组织端** — 教师 / 管理员登录、建班 + 名单管理、CSV 导入、师资分配、学校维度聚合、RBAC。
- ✅ **评估模型定稿** — AI 批判性思维**双轴模型**（深度 D1–D6 × L1–L4 · 自主 A1–A6 × 0–5 · 六透镜），单次会话 = 事件级证据；三面（工作室 / 课程 / 陪练）各自评估。
- ✅ **成长报告 + 教师端 + 家长报告** — 学生三 tab（学习记录 / 工具卡 / 六辐能力雷达）、教师班级周报 + 学生报告、家长报告（项目式 + 阶段式）。
- ✅ **语音** — 火山 TTS / ASR，端到端 live 验证。
- ✅ **工作室四房间** — 项目管理 / 阅读 / 写作 / 回顾；写作室（学生自写 + AI 查论证 + 完成导出）。
- ✅ **写作旅程状态机重构**（2026-08，行为真相源 `docs/2026-08-09-all-statuses.md`）— 立项 → 写提案 → 建证据地图 → 正文陈述 → 成文定稿 → 回顾，全程按状态机驱动；**AI = 教 / 关 / 显**。落地：提案 & 正文的**引导写作卡**（一次一部分）· 老师式**分层批注**（总体/段落/句子 · 绿蓝红 · 只读）· 大纲改子问题的「换措辞 vs 换问题」提醒 · 证据地图饱和度审阅 · 阅读室「检索方向」引导盒 + 「还需要探索的」盒 + 手动进阅读室的确认门 · 回顾四页签（活动日志/研究框架/提案/成品）· 提案完成的祝贺 + 导出。**已部署上线并在真实前端端到端走查通过（0 报错）。**
- ✅ **模型分档路由** — 陪练 / 引导走快模型 `deepseek-v4-flash`（每轮陪练成本较旗舰降约 36%）；审阅（框架 / 计划 / 整稿 / 饱和 / 评估）走旗舰 `deepseek-v4-pro` 绝不降级。见 `docs/2026-08-10-user-walk-and-model-routing-report.md`。
- ✅ **阅读室兔子洞地图** — 两层文献探索图（React Flow + dagre）：问题图 + 带标签关系边（印记提议 / 学生确认）+ 钻进问题的 mindmap + 统一三态侧栏；**OpenAlex** 相似 / references / cited-by 检索（印记提炼查询）+ 候选托盘采纳 + 图书馆自动上架。端到端 live 验证。
- ✅ **34 张卡全部可交互，0 stub。**
- ✅ **生产部署上线** — Docker Compose + 宿主机 nginx + certbot HTTPS。
- 🔭 **下一步：** 计费 / 权益方案设计（填 `HasEntitlement` 接缝）；评估异步化（river worker）为规模化预留。
- 🔭 非阻塞增强（登记于 `docs/遗留项追踪_Carryforward.md`）：评估语义扩展、a11y / 整洁项。

### 明确不做（范围决定，非遗漏）

> 写作面的定位（2026-07-30 更新）：写作室是一个**朴素的写作面**——学生**自己写**正文，成品可导出带走。三条硬约束不变：

- ❌ **AI 代写正文**（写作室只服务「学生自己写、AI 陪想查论证」；过程树只读）。
- ❌ **花哨排版 / 文档编辑器**（不是 Google Docs 的替代品，不做样式排版）。
- ❌ **代学生向学校提交**（我们占「思考」这一层，不拥有、不提交成品文档）。
- ❌ 上瘾式游戏化：连胜、排行榜、徽章、推送一律不做。

---

## 📚 文档

权威产品规格与架构 north-star 都在 `docs/`：

- **`docs/2026-08-11-developer-handover.md`** — **开发者上手 / 交接文档（英文）**：技术要求 · 代码结构 · 设计铁律 · 全量文档地图。新同学从这里开始。
- **`docs/思维印记_Demo_PRD.md`** — 产品 / 不变骨架（产品理念冲突时以 PRD 为准）。
- **`docs/superpowers/specs/2026-06-24-backend-platform-architecture-design.md`** — 后端平台架构 north-star（涉及后端 / DB / API / 鉴权 / 组织 / 评估以架构文档为准）。
- **`docs/architecture/`** — `database-schema` · `api-design` · `go-backend-best-practices` · `chat-history-storage`。
- **`docs/2026-08-09-all-statuses.md`** — 写作全流程各状态行为的**单一真相源**（写 plan 前必对照）。
- **评估 / 教师端交接（英文）** — `docs/2026-08-11-evaluation-data-storage-guide.md`（学生全流程数据落库地图）· `docs/2026-08-11-evaluation-and-teacher-code-map.md`（评估与教师端代码地图）。
- **`docs/deploy/`** — 部署运行手册：`README` · `api` · `web` · `tls`。
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
