# 思维印记 (Mind Imprint) · Project Instructions

> A platform for international track (IB) students using AI to learn better.
> 思维印记是一个 **AI 工具**（形态类似 Cowork / Codex 这类 agent 工具），封装了两层独有能力——交互式「思维工具卡」与「过程评估」。学生带自己的真实任务进来，在与 AI 协作中被引导做结构化思考，全过程被记录、评估。

**权威来源：** `docs/思维印记_Demo_PRD.md`（技术规格 / 不变骨架）。本文件是摘要与硬约束；细节以 PRD 为准，冲突时以 PRD 为准。工具卡全库见 `docs/工具包库/`（31 张）。设计已定稿，不在本文件维护设计规范。

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

| 层 | 选型 |
|---|---|
| 前端 | React + Vite + TypeScript + Tailwind |
| 后端（很薄） | Node + Express (TypeScript) — LLM 网关、registry/数据服务、异步评估 |
| 存储 | SQLite — `task` / `message` / `card_instance` / `process_node` / `evaluation` |
| 模型 | Anthropic API（需用户提供 key）。陪练走中档模型可降级；评估走旗舰模型绝不降级 |

**三层解耦（核心心智模型：工具卡 = tool-use 循环里「由人来执行的工具」）：**
决策层（用不用 / 用哪张）→ `summon_card(card_id, reason, nudge_text)` 单函数接线 → Card Runtime（schema 驱动渲染 + 三视觉态 + 事件采集 + 标准信封落库）→ 回灌陪练 → 所有标准信封长成过程树 + rubric 评估。

---

## 给开发者的硬约束（不可违反）

- **客户端绝不直连模型。** 所有 LLM 调用走后端网关，记录档位 + token + 成本；API key 只在服务端。
- **卡 spec 单一真相源在 registry。** 决策层目录从它派生，不手写第二份（避免 `trigger_condition` 漂移）。
- **新增卡 = 新增一份 JSON 配置，不改渲染器代码。** 这是 schema 驱动是否成立的验证标准；用现有字段原语（`text` / `textarea` / `single_choice` / `multi_choice` / `rating` / `repeatable_group` / `link_check`）拼，除非真需要全新交互才加原语。
- **标准信封结构一旦定下不要随意改**——它是过程树、日历、使用计数、评估的共同地基。
- **不做「明确不做」清单里的任何一项。**

## 明确不做（本期）

- ❌ 文档编辑器 / 作业撰写排版 / 提交功能（右侧过程树是**只读**记录，不是编辑器）。
- ❌ 教师端：账户、师生关系、批改、评分、通知。评估**数据**照跑，但只给学生看。
- ❌ 工具卡全库：本期只做 2 张样例卡（**SIFT×CRAAP**、**让步段**）；全库另开任务并行灌进同一渲染器。
- ❌ 检索 / RAG 路由：卡少，决策层直接用「目录 + 分类」。
- ❌ 真实登录鉴权：demo 用单一 mock 学生。
- ❌ 上瘾式游戏化：连胜、排行榜、徽章、推送一律不做。

---

## 验收主动脉（端到端演示脚本）

场景锚定 **Phoebe / 「中国是否让地球变得更可持续？」**：新建任务粘文章链接 → AI 调 `summon_card("sift_craap")` → 学生填 SIFT 四步溯源到 NASA / Nature Sustainability → 摘要回灌、一次只问一个 → 写论证撞反例（中国碳排放全球第一）→ AI 调 `summon_card("concession")` 完成让步段 → 过程树实时生长 → 评估那一刀跑出 rubric 评级 + 过程叙述，以「你的思维印记」呈现。mockup 用真实内容，别用 lorem ipsum。

---

## Shared Memory

**Always write new instructions, rules, and memory to `AGENTS.md` only.**

Never modify `CLAUDE.md` or `GEMINI.md` directly - they only import `AGENTS.md`.
This ensures Claude Code, Codex CLI, and Gemini CLI share the same context consistently.

## Project Structure

- `docs/` - 权威产品规格：PRD、工具包库（31 张卡）
- `.claude/agents/` - Custom subagents for specialized tasks
- `.claude/skills/` - Claude Code skills (slash commands)
- `.claude/rules/` - Modular rules auto-loaded into context
- `.codex/skills/` - Codex CLI skills
- `.codex/prompts/` - Codex CLI custom slash commands
- `.gemini/skills/` - Gemini CLI skills
- `.gemini/commands/` - Gemini CLI custom slash commands (TOML)
- `.mcp.json` - MCP server configuration
