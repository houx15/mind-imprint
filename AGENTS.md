# 思维印记 (Mind Imprint) · Project Instructions

> A platform for international track (IB) students using AI to learn better.
> 思维印记是一个 **AI 工具**（形态类似 Cowork / Codex 这类 agent 工具），封装了两层独有能力——交互式「思维工具卡」与「过程评估」。学生带自己的真实任务进来，在与 AI 协作中被引导做结构化思考，全过程被记录、评估。

**权威来源：**
- **产品 / 不变骨架：** `docs/思维印记_Demo_PRD.md`（产品规格、工具卡、过程评估理念）。
- **后端平台架构（north-star）：** `docs/superpowers/specs/2026-06-24-backend-platform-architecture-design.md`，配套 `docs/architecture/database-schema.md`、`docs/architecture/api-design.md`、`docs/architecture/go-backend-best-practices.md`。**凡涉及后端、数据库、API、鉴权、组织（学校/班级）、异步评估，以架构文档为准；与 PRD 旧的「薄 Node 后端 / SQLite」描述冲突时，以架构文档为准。**

本文件是摘要与硬约束；产品理念细节以 PRD 为准。工具卡全库见 `docs/工具包库/`。设计已定稿，不在本文件维护设计规范。

---

## 我们占的是哪一层

**「思考」这一层，不是「作业被完成、被提交」的地方。** 学生的作文活在他自己的世界里（Google Doc、本子、随手粘进来），我们不拥有那个文档。

## 四条设计铁律（违反即自毁）

1. **AI 克制，绝不替学生定论。** AI 的职责不是给答案，而是在对的时刻把「思考」塞回给学生。系统 prompt 的核心就是这条克制阶梯。
2. **不操纵。** 这个产品教学生「不被俘获」。绝不用老虎机式机制（连胜、排行榜、推送上瘾）留人，也不用弹窗强迫——工具卡**触发是自动的，但「打开」由学生确认**。
3. **一次只问一个。** 陪练对话短、不啰嗦，降低 token，也保护学生的思考节奏。
4. **过程即数据。** 学生跳过工具卡、让 AI 直接答，也被记录。摩擦被转化成信号，而不是被消灭。

---

## 技术栈

> 注：早期为纯前端 SPA（localStorage + 浏览器直连模型）。现已定稿向「前后端分离平台」重构，以下为目标架构；细节见架构文档。

| 层 | 选型 |
|---|---|
| 前端 `apps/web` | React + Vite + TypeScript + Tailwind —— 纯渲染 + API 客户端，不持有密钥、不直连模型 |
| 后端 `apps/api` | **Go**（`net/http` + `pgx`/`sqlc` + `goose` + `river`）—— 智能网关（系统 prompt / `summon_card` / refeed / turn loop 都在服务端）、数据服务、鉴权、异步评估。唯一持有密钥、唯一访问 DB 与模型的单元 |
| 存储 | **PostgreSQL** —— `users`（含 `school_id`）/ `sessions` / `schools` / `classes` / `enrollments`（用户↔班级）/ `task` / `message`（旧任务面留存，不再有新写入）/ `card_instance` / `evaluation`（**暂定**）。无 `process_node` 表（过程树由 `parent_node_id` 投影）；`project` 面每次真实 LLM 调用（陪练 / 锚点生成 / 课程渲染）都记一行 `llm_call`（档位 + token + 成本），`llm_usage` 视图三路 UNION（`llm_call` + 冻结的 `message`/`evaluation`）供组织成本汇总 |
| 模型 | China-first：默认 **DeepSeek**，Anthropic 可选。**平台持有 key**（服务端 env），经 `keyResolver` 接缝预留未来按组织计费。陪练走中档模型可降级；评估走旗舰模型**绝不降级** |

**三层解耦（核心心智模型：工具卡 = tool-use 循环里「由人来执行的工具」）：**
决策层（用不用 / 用哪张，**服务端**）→ `summon_card(card_id, reason, nudge_text)` 单函数接线 → Card Runtime（schema 驱动渲染 + 三视觉态 + 事件采集 + 标准信封落库）→ 回灌陪练 → 所有标准信封长成过程树 + rubric 评估。卡 JSON 为单一共享真相源：前端构建期 import，Go `go:embed` 同一批文件。

---

## 给开发者的硬约束（不可违反）

- **客户端绝不直连模型。** 所有 LLM 调用走后端网关，记录档位 + token + 成本；API key 只在服务端。
- **卡 spec 单一真相源在 registry。** 决策层目录从它派生，不手写第二份（避免 `trigger_condition` 漂移）。
- **新增卡 = 新增一份 JSON 配置，不改渲染器代码。** 这是 schema 驱动是否成立的验证标准；用现有字段原语（`text` / `textarea` / `single_choice` / `multi_choice` / `rating` / `repeatable_group` / `link_check`）拼，除非真需要全新交互才加原语。
- **标准信封结构一旦定下不要随意改**——它是过程树、使用计数、评估的共同地基。后端按「边界校验」存储：Go 只校验信封外层（`status` 枚举、id、`field_values` 为对象、`event_trace` 为数组），内层深结构真相仍归 `packages/contracts` 的 Zod 契约。
- **组织不变式：每个账号必属于某个学校；学生/教师还属于 ≥1 个班级，不存在「无组织账号」。** 注册必须携带有效班级 join code；建号（`school_id` 取自 `class.school_id`）与 `enrollments` 在同一事务内完成，否则整体失败。**结构归属（学校/班级）与「会员/付费权益」是两回事**：付费权益暂不建表，由后端 `HasEntitlement(ctx,user)` 接缝（当前恒 true）在消耗 token 的端点前判定，未来接订阅/代币模型。
- **密钥只在 `apps/api` 服务端**（LLM key / DB DSN / session secret / SMTP），绝不进 git、日志、抛出或渲染的错误、数据存储、评估 payload。

## 重构路线（已定稿，取代旧的「明确不做（本期）」）

架构 north-star + 分期实施，每期独立 spec → plan → build：

- **P1 · 后端地基 + 智能网关**：Go 服务、Postgres+goose、信封落库、SSE 网关、prompt/refeed/loop 移植、每轮用量落 message 行、种子「学校+班级+学生」、`HasEntitlement` 桩、**内联**评估；前端改用 API。终态 = 今天的体验 + 真后端 + 单一 mock 学生。
- **P2 · 鉴权 + 最小组织**：注册/邮箱验证/登录、argon2id、session；管理员预置班级 + join-code 注册门槛；任务归属到人。
- **P3 · 完整组织端**：教师/管理员角色登录、建班 + 名单管理、学校维度聚合。
- **P4 · 异步评估**：river 任务 + worker，评估入队 + 轮询。
- P4 之后（本轮不展开）：对象存储 / OSS、多模态、更多卡、更丰富评估。

**写作面的定位（2026-07-30 更新，取代旧的「❌ 撰写/提交」一刀切）：** 写作房间是一个**朴素的写作面**——学生在其中**自己写**正文（textarea + Markdown 预览 + 自动存草稿），也可粘贴/上传。铁律不变的三条硬约束：**① AI 绝不代写正文**（只陪想、只查论证与结构、一次一问）；**② 不做花哨的排版/文档编辑器**（不是 Google Docs 的替代品，不做样式排版）；**③ 我们不代学生向学校提交**。项目在回顾处**完成→过程评估**，成品可**导出**（.docx/.xlsx）带走。我们仍占「思考」这一层：右侧过程树是**只读**记录，成品文档活在学生自己的世界里，我们不拥有、不提交它。

**仍然明确不做：**
- ❌ AI 代写正文 / 花哨排版编辑器 / 代学生向学校提交（写作面只服务「学生自己写、AI 陪想查论证」；过程树只读）。
- ❌ 上瘾式游戏化：连胜、排行榜、徽章、推送一律不做。

---

## 验收主动脉（端到端演示脚本）

场景锚定 **Phoebe / 「中国是否让地球变得更可持续？」**：新建任务粘文章链接 → AI 调 `summon_card("craap")` → 学生用 CRAAP 对来源做溯源体检，追到 NASA / Nature Sustainability → 摘要回灌、一次只问一个 → 写论证撞反例（中国碳排放全球第一）→ AI 调 `summon_card("concession")` 完成让步段 → 过程树实时生长 → 评估那一刀跑出 rubric 评级 + 过程叙述，以「你的思维印记」呈现。mockup 用真实内容，别用 lorem ipsum。

---

## Shared Memory

**Always write new instructions, rules, and memory to `AGENTS.md` only.**

Never modify `CLAUDE.md` or `GEMINI.md` directly - they only import `AGENTS.md`.
This ensures Claude Code, Codex CLI, and Gemini CLI share the same context consistently.

## Project Structure

- `apps/web/` - 前端 SPA（React + Vite）
- `apps/api/` - 后端 Go 服务（重构后新增）
- `apps/site/` - 营销站（Astro 静态站，中英双语 zh 默认 / en 于 `/en/`；纯展示 + 链接到 app，不持有密钥；风格 = Toddle 暖编辑 + Apple 叙事）
- `packages/contracts/` - Zod 契约 + 卡 JSON 单一真相源（前后端共享）
- `docs/` - 权威产品规格：PRD、工具包库
- `docs/architecture/` - 后端平台架构参考：database-schema、api-design、go-backend-best-practices
- `docs/superpowers/specs/` - 已定稿设计 spec（含后端平台架构 north-star）
- `.claude/agents/` - Custom subagents for specialized tasks
- `.claude/skills/` - Claude Code skills (slash commands)
- `.claude/rules/` - Modular rules auto-loaded into context
- `.codex/skills/` - Codex CLI skills
- `.codex/prompts/` - Codex CLI custom slash commands
- `.gemini/skills/` - Gemini CLI skills
- `.gemini/commands/` - Gemini CLI custom slash commands (TOML)
- `.mcp.json` - MCP server configuration
