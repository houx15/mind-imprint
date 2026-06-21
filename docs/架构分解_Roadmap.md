# 思维印记 · 架构分解与开发路线图 (Block Map & Roadmap)

> 版本 v0.2 ｜ 2026-06-20（v0.2：改为**前端直连 LLM / 无后端 / BYO-key**，详见 §0.1）
> 配套权威来源：`docs/思维印记_Demo_PRD.md`（产品/数据规格）+ `docs/design/思维印记_工作区.dc.html`（**已确认的最终 UI 设计**）。
> 本文件是「把整个系统拆成块、定依赖、定建造顺序」的地图。每个 slice 各自走 spec → plan → TDD 实现。

---

## 0. 双真相源规则（一旦冲突，按此裁决）

| 维度 | 真相源 | 说明 |
|---|---|---|
| **一切视觉**（布局、三态、字段渲染、配色、底部抽屉、过程树、导航、空/激活/完成态） | **`docs/design/思维印记_工作区.dc.html`** | 已确认的最终设计。逐像素照搬，**不**用 PRD 或任何 brief 的 UI 描述去覆盖它。 |
| **非视觉**（卡契约数据结构、标准信封、四条设计铁律、克制阶梯、rubric 维度） | **`docs/思维印记_Demo_PRD.md`** | 但**架构部分**已被 §0.1 覆盖。 |

> 凡 HTML 与 PRD 文字在 UI 上不一致（如导航为 `任务/记录/设置`、设计含登录/绑定班级），**以 HTML 为准**，不做"合并折中"。

## 0.1 架构决策：前端直连 LLM（覆盖 PRD §5 / §16 的后端架构）

**Demo 决定（2026-06-20，用户拍板）：不做后端。** LLM 调用在**前端直连**，密钥由用户自带（BYO-key）。理由：人人用自己的 key，密钥泄露/算力成本/网络攻击都不是 demo 的顾虑，且省去后端与流式代理的大量工作。

由此**明确覆盖** PRD §5/§16 的两条："客户端绝不直连模型" 与 "API key 只在服务端 + 记录档位/token/成本"。取而代之：

- LLM 调用走前端 `apps/web/src/llm/` 客户端模块（**provider 无关**：openai 格式 / anthropic 格式，凭 `baseUrl + model + apiKey` 配置）。
- 密钥存在浏览器（localStorage / 内存），由用户在设置页输入；也可用 git-ignored 的 `apps/web/.env.local` 预填（开发便利）。
- **非流式**（陪练回复短，零流式代码）。
- **不做计量**（成本非顾虑）；如 provider 返回 usage 可顺手展示，但不建计量系统。
- 持久化从 SQLite 改为**浏览器存储**（IndexedDB/localStorage）。S1 产出的 `packages/contracts/sqlite-schema.sql` 因此**作废为逻辑模型参考**，不执行；其"evaluation 表缺主键"的遗留项随之**作废**。
- 标准信封（`CardInstance`）契约**不变**——它与存储无关，只是落到浏览器存储而非 SQLite。

> 注：若未来要做真实/托管版本，LLM 层与存储层需重建为后端网关 + 服务端 key（PRD 原架构）。本决策仅服务 demo。

---

## 1. 块 (B0–B7 + B2.5)

| # | 块 | 负责什么 | 依赖 | 验证 PRD §2 成功判据 |
|---|---|---|---|---|
| **B0** | **Contracts & Spine** | 卡 schema、7 个字段原语、**标准信封**、数据模型定义（存储无关）、registry 加载器+派生目录、rubric 配置 | — (根) | 地基 |
| **B1** | **Card Runtime** | schema 驱动 `<CardRenderer>`、三视觉态、字段原语组件、方法论面板、事件采集 → 信封 | B0 | #2 卡可交互/可展示/可标准化存储 |
| **B2** | **LLM Client（前端、provider 无关）** | `llm/` 模块：openai/anthropic 适配器、统一 `chat()`、`LlmConfig`（含可选 `evalModel`）、配置存取、设置面板（测连接）。非流式、BYO-key | —（前端独立；不依赖 B0） | #4（真实 LLM 调用）地基 |
| **B2.5** | **Browser Store（持久化）** | IndexedDB/localStorage 仓储：`task`/`message`/`card_instance`(标准信封)，用 B0 类型。替代 SQLite | B0 | #3/#4 的存储地基 |
| **B3** | **Decision + `summon_card` Loop** | 从 registry 派生目录、克制策略、陪练编排、**前端 tool-use** 调卡 → 渲染 → 摘要回灌 | B0, B1, B2, B2.5 | #1 AI 正确调卡 · #4 |
| **B4** | **Workspace Container** | 对话主轴 + composer + 面包屑 + 底部抽屉宿主 + 右侧面板骨架 | B1, B3 | 承载 #1/#2/#4 的壳 |
| **B5** | **Process Tree** | 从浏览器存储的 `card_instance`+`message` 事件派生过程树，右侧只读渲染（7 类节点） | B0, B2.5 | #3 过程长成可读的树 |
| **B6** | **Evaluation** | 前端调 LLM 客户端（用 `evalModel`，无则回退 `model`）、SOLO L1–L4 rubric + 过程叙述、`showEval`「你的思维印记」 | B0, B2, B2.5 | 产品护城河（PRD §11） |
| **B7** | **App Shell** | 左导航(任务/记录/设置) + 认证(mock) + 主目录网格 + 记录页(recAbility/recCards/recLearning) + **设置页(LLM 配置)** | B2, B2.5, B4, B6 | 外壳 |

---

## 2. 依赖图

```
                ┌──────────────────────────┐        ┌────────────────────────┐
                │   B0  Contracts & Spine   │        │  B2 LLM Client (前端,   │
                └───────────┬──────────────┘        │  provider 无关, 独立)   │
              ┌─────────────┼───────────────┐       └───────────┬────────────┘
              ▼             ▼               ▼                    │
          ┌───────┐   ┌──────────────┐  (rubric)                │
          │  B1   │   │ B2.5 Browser │                          │
          │Runtime│   │    Store     │                          │
          └───┬───┘   └──┬────────┬──┘                          │
              │          │        │                             │
              └────┬─────┘        ├──────────────┐              │
                   ▼              ▼              ▼              ▼
              ┌─────────┐     ┌──────┐       ┌────────────────────┐
              │ B3 Loop │◄────┤      │       │      B6 Eval       │
              └────┬────┘ (B2)└ B5   ┘       └─────────┬──────────┘
                   ▼          Tree                     │
              ┌─────────┐                              │
              │ B4 Cont.│ ──────────► ┌────────────┐◄──┘
              └─────────┘             │  B7 Shell  │ (壳，串起全部；设置页装 LLM 配置)
                                      └────────────┘
```

建造顺序：**B0+B1 → B2 → B2.5 → B3+B4 → B5 → B6 → B7**。（B2 与 B2.5 互不依赖，可调序；当前先 B2 让模型先说话。）

---

## 3. Slice 划分（每个 slice = 一次 spec→plan→TDD 循环）

| Slice | 含块 | 一句话目标 | 关键产出 | TDD 适配 |
|---|---|---|---|---|
| **S1** ✅ | B0+B1 | 契约骨架 + 卡渲染器（纯前端、假数据） | 渲染器照 HTML 渲染两卡，三态可走，提交产出标准信封 | **强**（已完成，56 测试 + typecheck 绿，已并入 main） |
| **S2** | B2 | provider 无关 LLM 客户端 + 配置（前端直连、BYO-key） | openai/anthropic 适配器、`chat()`、配置存取、设置面板（测连接）；真实模型能回话 | **强**：适配器对 mock `fetch` 单测（请求形/解析/错误）；配置存取单测；1 个 live 冒烟（有 key 才跑） |
| **S2.5** ✅ | B2.5 | 浏览器持久化 | IndexedDB/localStorage 仓储：task/message/card_instance 落盘、可读回、刷新不丢 | **强**：仓储对内存/jsdom 存储单测（已并入 main） |
| **S3a** ✅ | B0+B1 扩展 | 工具卡全库导入（用户改：本期用**全部 31 张库卡**，非 2 张样例） | `CardSpec` 加 frontmatter 路由元数据（priority/tier/trigger_keywords/interaction_type…）；31 库卡 JSON 导入 + registry 注册（共 33 张，含 2 张 demo 卡）；能用 7 原语表达的给真实 body，需全新交互的先 stub（→3c）；Harness 选卡器 | **强**：逐卡 `CardSpec` 校验 + registry fail-loud + 完整性门（33 张）|
| **S3b** | B3+B4 | 决策层 + `summon_card` 前端回灌循环 + 工作区容器 | 真实对话中 AI 调对卡（在全 33 张上按 tier/priority/trigger 路由）→ 渲染 → 摘要回灌；专注全屏布局；LLM 客户端在此实现 tool-calling | **中**：tool-use 循环/信封→回灌逻辑用 fake LLM 单测；调卡"质量"属 eval 集 |
| **S3c**（later） | B1 扩展 | 富交互卡 | 为 stub 卡建真实交互（量表光谱/角色模拟/画布导图/分类标注/媒体回放/条件分支），含分步脚本与就地小讲解；把对应卡 `body_status` 由 stub 升 full；可能加新原语（如 `show_if`/spectrum）| **中**：新原语/组件单测 |
| **S4** | B5 | 过程树：事件流 → 树 | 右侧只读树实时生长，7 类节点 | **强**：树派生是纯函数 |
| **S5** | B6 | 评估那一刀 + 「你的思维印记」 | rubric L1–L4 + 过程叙述（用 `evalModel`） | **中**：prompt 组装/产物解析单测；评级质量属 anchor 样本 eval |
| **S6** | B7 | 外壳：认证(mock) + 主目录 + 记录页 + 设置页(LLM 配置) | 全 app 串起来，导航 任务/记录/设置 | **中**：组件 RTL + 路由测 |

> 先把 **S1–S3** 跑通即证明 PRD §2 成功判据 1、2、4；S4 证明 #3；S5 是护城河。

---

## 4. UI 表面清单（取自 HTML，映射到块）

- **认证** `authScreen` → `loginScreen`/`registerScreen`/`bindScreen` → **B7**（无后端，只能 mock）
- **左导航** 任务/记录/设置 + 头像 → **B7**
- **主目录** `showDirectory` → **B7**（数据来自 B2.5 store）
- **工作区** `showWorkspace`：面包屑/`footerComposer`/`treeOpen`·`treeClosed` → **B4**（树体 → B5）
- **底部抽屉** `hasActiveCard`：`isSiftActive`/`isConcessionActive`/`craapOpen`/`showMethodology` → **B1**
- **提议气泡** 三态 → **B1**（由 **B3** 驱动出现）
- **评估** `showEval`/`evalLoading` → **B6**
- **记录页** `isRecords`：`recAbility`/`recCards`/`recLearning` → **B7**（数据来自 B2.5，叙述来自 B6）
- **设置页** `isSettings` → **B7**，装 **LLM 配置**（format/baseUrl/model/apiKey + 可选 evalModel），存浏览器

---

## 5. 待裁决 / 已裁决

- ✅ **LLM 调用方式**：前端直连、BYO-key、非流式（§0.1）。
- ✅ **密钥存哪**：浏览器（localStorage）+ 可选 `.env.local` 预填。原"客户端绝不存 key"作废。
- ✅ **模型路由**：`LlmConfig.model` 通用 + 可选 `evalModel`（B6 用，无则回退）。
- ✅ **计量**：不做（成本非顾虑）。
- ⬜ **认证是否真做**：无后端 → 只能 mock UI（点击即进）。开 B7 时定。
- ⬜ **`记录` 页 `recAbility` 能力图**：技能树本期不做；HTML 有入口。倾向 S6 出轻量静态版。开 B7 时定。
- ⬜ **持久化技术**：IndexedDB vs localStorage（仓储接口隔离，开 S2.5 时定）。

---

## 6. 不可违反（贯穿所有 slice）

- **LLM 调用走前端 `llm/` 客户端**（provider 无关，BYO-key）；**不做后端、不做服务端 key、不做计量**（§0.1 已覆盖 PRD §16 的反向旧规）。
- 卡 spec 单一真相源在 registry；决策层目录从它派生（不手写第二份）。
- 新增卡 = 新增一份 JSON，**不改渲染器代码**（schema 驱动的验收标准）。
- 标准信封结构一旦定下不随意改（树/记录/评估的共同地基；现落浏览器存储）。
- 四条设计铁律：AI 克制不替学生定论 · 不操纵（卡自动触发但由学生确认打开） · 一次只问一个 · 过程即数据（跳过也记录）。
- 不做 PRD §4 / AGENTS.md「明确不做」清单里的任何一项（上瘾式游戏化等）。
- 密钥**绝不入 git**（`.env.local` git-ignored），**绝不打日志**。
- **验收门槛**：`pnpm -r typecheck` + `pnpm -r test` 必须全绿。注意 `vite build`/`vitest` 用 esbuild 转译、**不做类型检查**——类型安全只由 `tsc --noEmit`（根 `pnpm -r typecheck`）保证。
