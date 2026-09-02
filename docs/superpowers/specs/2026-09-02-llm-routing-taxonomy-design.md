# LLM 调用分级路由：把「三条 lane」换成「六个能力档」

2026-09-02 · design spec

**目标：** 让每一次 LLM 调用都声明「这件事需要多少智力」，然后由目录决定用哪个模型跑。
在此基础上横向实测，找到「能跑通全部已建功能」前提下，性能 / 速度 / 成本最平衡的一组绑定。

---

## 1 · 现状：三条 lane 已经不够用了

`models.json` 今天只有三条 lane —— `chat` / `fastChat` / `eval`。全仓实测的调用点分布：

| surface | chat | fastChat | eval | 合计 |
|---|---|---|---|---|
| `proOnly` | 14 | 6 | 9 | 29 |
| `liteOnly` | 3 | 0 | **13** | 16 |
| `protected`（课程等） | 3 | 0 | 0 | 3 |
| 内部 helper（无路由） | 6 | 3 | 10 | 19 |
| **合计** | 26 | 9 | 32 | **67** |

（清单由 `apps/api/.audit/surface.py` 从路由表 + resolver 使用点直接生成，不是手抄。）

三个结论，每一个都直接是钱和时间：

**① lite 几乎整个跑在旗舰档上。** 16 个 lite 调用点里 13 个走 `resolveEval`——
阅读陪练、PBL 对话、写作间的 plan / guide / deepen / title / comment / review 全部。
`eval` 是 `flagship` 档，`catalog_provider.go` 只在 `Tier == "chaperone"` 时关思考，
所以**每一轮学生对话都在满额推理**。这不是配置失误，是「只有三条 lane，而对话又不能用
`fastChat` 的质量」逼出来的：档位不够细，就只能往上取。

**② `chat` 这一条里塞了三种不同的活。** 学生看得见的对话（`postCoach`）、结构生成
（`ComposeJourney`、`planReadingTasks`）、长文压缩（`maybeCompactBackstop`）现在共用
一个模型。它们对「智力」的要求差一个数量级，对延迟的容忍度差两个数量级。
一起绑，等于按最贵的那个需求给全部三种付钱。

**③ 换模型时无法归因。** 只有三个旋钮，而旋钮后面是七类工作，测出来的
「速度变快了 / 质量变差了」没法归到具体哪类调用上。

## 2 · 六个能力档

档（class）描述的是**这次调用需要多少智力**，不是它属于哪个功能。同一个页面上的
两次调用可以分属两档；两个毫不相干的功能可以共用一档。

| class | 中文 | 判据 | 推理 | tier | 延迟预算 | 质量怎么测 |
|---|---|---|---|---|---|---|
| `reflex` | 反射 | 输入短、输出是一个标签或一次路由选择，没有自由文本 | OFF | chaperone | < 2s | 对固定用例的标签命中率 |
| `dialogue` | 陪练 | 学生当场看得见的对话轮，一次只问一个 | OFF | chaperone | < 4s | 判官打分 + 铁律违例计数 |
| `compose` | 结构生成 | 从已陈述的输入派生一个 schema 形状的产物（计划 / 提纲 / 引导卡 / 锚点） | low | mid | < 12s | schema 通过率 + 判官打分 |
| `review` | 审阅 | 读学生的成果并作判断，判错有代价 | ON | flagship | < 40s | 对 gold 的判官打分 |
| `assess` | 过程评估 | rubric 评估报告 / 回顾 / 周报 | ON max | flagship，绝不降级 | 异步 | evalbench gold 比对 |
| `digest` | 摘要 | 长输入 → 短输出，压缩而不判断 | OFF | chaperone 长上下文 | < 15s | 判官打分 |

预留（本期只登记，不绑定）：`search`（联网检索，需 `tools` 能力 + 多轮工具循环）、
`multimodal`（图文输入）。

**为什么是这六个而不是别的：** 每一档的边界都落在一个**会改变模型选择**的地方。
`reflex` 和 `dialogue` 都关思考，但前者可以用最便宜的 flash，后者不行——中文表达能力
是学生直接感知的。`compose` 和 `review` 都产出结构化结果，但前者是从**学生已经说过的话**
里派生（确定性工作，AGENTS.md 里明确写了这不受铁律②约束），后者要对**学生没说的话**
下判断。`assess` 单独成档只有一个理由：它绝不降级，把它和 `review` 合并会让降级实验
不小心波及到评估。

### 档位归属（全部 67 个调用点）

| class | 调用点 |
|---|---|
| `reflex` | `ClassifyMoment` · `ClassifyClaimRevision` · `RouteReading`（pro + lite 阅读轮路由）· `BuildStatusRequest` · `postSuggestPlacement` · 项目类型识别 |
| `dialogue` | `postCoach` · `postCoachOpening/Start/Advance` · `postChatTurn` · `postCourseAsk` · `postProjectTurn` · `postReadingCoachTurn`（lite）· `postLiteWritingTurn` · `postPblTurn` · `postWritingOpening` · `explainReadingBlock` · `postReflectProjectCard` |
| `compose` | `ComposeJourney` · `Generate`（锚点）· `ProposeCardExample` · `ProposeSearchKeywords` · `GenerateProposalGuideStep` · `QuestionCardTurn` · `ComposeExplorationGuide` · `ComposeDigQuery` · `ProposeQuestionEdges` · `GenerateCourseScene` · `ComposeAIUseSeed` · `planReadingTasks` · `postWritingPlanTurn` · `suggestWritingTitles` · `generatePlanItems` · `getReadingQuestions` · `guideWritingBlock(s)` · `deepenWritingBlock` · `ComposeReadingTakeawaySuggestions` · `generateEssayGuideCard` · `generateGuideCard` · `summonProjectCard` · `summonReadingLens` |
| `review` | `ReviewFramework` · `ReviewDraftAnnotations` · `ReviewEvidenceSaturation` · `ReviewExploration` · `ProposeReview` · `ProposeSpotCheck` · `EvaluateSelection`（pro + lite）· `reviewWritingDraft` · `commentOnSnippet` · `reviseEssayClaim` |
| `assess` | `collectReport`（评估报告）· `generateReportProse` · `getPblLookback` · `ComposeWeekly` |
| `digest` | `ComposeDigestMerge` · `ComposeReturnSummary` · `maybeCompactBackstop` |

## 3 · 结构：档是目录里的数据

`models.json` 的 `lanes` 从 3 条长到 8 条（6 个在用 + 2 个预留）。lane 的 schema 增加
三个字段，全部是**声明这一档要什么**，而不是声明用哪个模型：

```json
"dialogue": {
  "model": "dashscope/qwen3.8-max",
  "tier": "chaperone",
  "label": "陪练 — 学生当场看得见的对话轮",
  "reasoning": "off",
  "latencyBudgetMs": 4000
}
```

- `reasoning`: `"off" | "low" | "high" | "max" | "default"` —— 档位对推理的要求。
  它压过模型的 `defaultReasoningEffort`，被单次请求的 `ReasoningEffort` 压过。
- `latencyBudgetMs`: 这一档的预算。**它不是超时**，是 routebench 的判分线，
  也是启动时的一致性检查：给一个 `< 4000ms` 的档绑一个实测中位数 12s 的模型，
  bench 会红，但服务照常启动（预算是目标，不是硬闸）。
- `tier` 语义不变：`chaperone` 触发 `thinkingOff`，`flagship` 不触发。

**向后兼容：** `chat` / `fastChat` / `eval` 保留为别名，分别指向 `dialogue` / `reflex` /
`assess`。旧的 `MODEL_CHAT` / `MODEL_FAST_CHAT` / `MODEL_EVAL` 继续生效。
新的每档环境变量按 class 名派生：`MODEL_DIALOGUE`、`MODEL_COMPOSE`、`MODEL_REVIEW`……
一次只动一条，测出来的差异才可归因。

**启动校验（沿用现有的「写错就不启动」）：**
- 档绑了目录里没有的 model id → 报错并列出全部合法 id
- `assess` 档绑非 flagship → 拒绝启动
- `reasoning: "off"` 的档绑了 `thinkingOffUnsupported` 的模型（GLM-5.3、kimi-k2.7-code）
  → **拒绝启动**。这是新增的一条：今天这个组合只会在 live 测试里红，或者更糟——
  在 GLM-5.3 上每一轮硬报错。
- 档绑了非 chat 能力的模型 → 拒绝启动（不变）

## 4 · Go 侧接线

`Resolvers` 从三个具名字段改成一张表 + 一个方法：

```go
type Resolvers struct {
    byClass  map[string]KeyResolver
    Bindings map[string]Resolved
    Chat, FastChat, Eval KeyResolver // 兼容别名，指向 byClass
}
func (rs Resolvers) For(class string) KeyResolver
```

`api.Deps` 增加 `Route func(class string) gateway.KeyResolver`，三个旧字段保留为
兼容层。调用点从

```go
resolved, err := a.d.ChatResolver(ctx)
```

改成

```go
resolved, err := a.route(ctx, gateway.ClassDialogue)
```

`a.route` 是 `internal/api` 上的一个 helper，负责 nil 兜底与错误整形，
行为与现有的 `resolveEval` / `resolveFast` 一致。

**分批迁移，一档一次提交**，每批跑全量 Go 测试。顺序按风险从低到高：
`digest` → `reflex` → `compose` → `review` → `dialogue` → `assess`。

## 5 · routebench：让「哪个组合最好」变成一次可重跑的测量

新增 `internal/routebench`，与 `evalbench` 平行（`evalbench` 只测评估报告这一个任务，
测的是 gold 比对；routebench 测的是**每一档的代表性调用**）。

**用例来源不是手写 prompt。** 每一档挑 3–5 个真实调用点，把它们的
system + user 消息按真实构造逻辑录成 fixture（JSON），存在
`internal/routebench/fixtures/<class>/<case>.json`。录制脚本从种子数据跑一遍真实
handler，把 `ChatRequest` 落盘——这样 bench 测的是生产里真的会发出去的那个 prompt，
不是一个近似。

**每个用例对每个候选模型测四件事：**

| 指标 | 怎么得到 | 为什么可信 |
|---|---|---|
| 延迟 | 首 token 时间 + 总时长，取 n=3 的中位数 | 单次采样上下浮动三分之一，2026-09-02 已经因此造出过一个不存在的「档位控制」 |
| token | `usage`，含 `reasoning_tokens` | 成本的唯一客观基础 |
| 结构合法性 | 输出喂给该调用点真实的解析函数（Go 里已有） | 免费且客观：大多数调用点本来就产出 JSON 信封 |
| 质量 | 判官模型对同一批输出盲评打分 | 只对 `dialogue` / `review` / `assess` 跑——这三档的好坏读不出来 |

输出一张表：`class × model → p50 延迟 / 输出 token / 结构通过率 / 质量分`，
外加一个**推荐绑定**：在满足「结构通过率 100% 且质量分不低于基线」的模型里，
选 token 最省的那个。

**成本这一列现在是空的**，因为 DashScope 那批模型全部 `UNPRICED`——目录里宁可记空
也不编数字。routebench 因此按 **token 量**排序而不是按钱，并在报告顶端标明这一点。
把百炼控制台的费率填进 `models.json` 的 `priceUsd` 之后，同一份 bench 结果不用重跑
就能换算出钱。

## 6 · 不在本期

- 联网检索（`search` 档）与多模态（`multimodal` 档）：只在目录里登记档位与预留校验，
  不接调用点。
- 按组织计费：`keyResolver` 的接缝不动。
- 把 workspace id 从 base URL 里挪进环境变量：与本期无关，单独一次。

## 7 · 验收

1. `api --print-models` 打印 8 个档、各自的绑定模型、推理要求与延迟预算。
2. 给 `dialogue` 绑 `dashscope/glm-5.3`（`thinkingOffUnsupported`）→ 启动失败并说明原因。
3. 给 `assess` 绑非 flagship → 启动失败。
4. 全量 Go 测试绿；`LIVE_LLM=1` 的 live 测试里，`dialogue` 档 reasoning token 为 0。
5. `routebench` 跑完输出上述表格，且表里每一行的结构通过率是真的用生产解析函数算出来的。
6. lite 的阅读陪练与 PBL 对话不再落在 `assess` 档上。
