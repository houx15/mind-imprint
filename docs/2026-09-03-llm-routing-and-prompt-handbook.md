# LLM 路由与 Prompt 手册

首次生成 2026-09-03 · 最后更新 2026-09-14 · 给接手「持续优化路由策略」与「打磨 prompt」的同事

> 附录 A / B 的两张清单是**从代码里生成的**，说明栏取自代码自己的注释，不是我
> 转述的。改完代码重新生成一次（见文末「怎么重新生成这份文档」），别手改表格。

---

## 0 · 一分钟看懂

- 调用点**不挑模型**，只声明「这次调用需要多少智力」——写
  `a.routeE(ctx, gateway.ClassDialogue)`，由目录决定今天这意味着哪个模型。
- 目录是 `apps/api/internal/gateway/models.json`（go:embed），**单一真相源**：
  通道 × 模型 × 能力档。换模型 = 改一个环境变量或改这个 JSON，不写 Go。
- 换绑定**必须有实测撑着**：`go run ./cmd/routebench`。人来改 models.json，
  工具只出推荐。
- 全系统 **71 处**模型调用，落在 7 个档上。**48 个 prompt 常量**，绝大多数在
  `internal/agent/`。

---

## 1 · 能力档：这套分类到底在分什么

档说的是**这次调用需要多少智力**，不是它属于哪个功能。同一个页面上的两次调用
可以在不同档；两个毫不相干的功能可以共用一个档。

分界线画在**会改变模型选择**的地方。`reflex` 和 `dialogue` 都关思考，但 reflex
可以跑最便宜的 flash 模型而 dialogue 不行——中文的顺不顺，学生当场就感觉得到。
`compose` 和 `review` 都产出结构化结果，但 compose 是从学生**已经说过**的话里派
生（确定性的系统工作，明确在铁律①之外），review 是对她**没说出口**的部分下判断。
`assess` 单独拎出来只有一个理由：它绝不降级，跟 review 合并会让降级实验碰到过程评估。

| 档 | 一句话 | 现在绑的模型 | 推理 | 延迟预算 | 调用点数 |
|---|---|---|---|---|---|
| `reflex` | 一个标签、一次路由判断，无自由文本 | `dashscope/qwen3.7-flash` | off | 2s | 4 |
| `dialogue` | 学生当场看得见的一轮 | `dashscope/deepseek-v4-pro` | off | 4s | 14 |
| `compose` | 从已陈述的输入派生一个 schema 产物 | `dashscope/glm-5.3` | low | 12s | 30 |
| `review` | 判学生的成果，判错有代价 | `dashscope/glm-5.3` | default（模型默认 low） | 40s | 12 |
| `assess` | 过程评估 / 回顾 / 周报，**绝不降级** | `dashscope/deepseek-v4-pro` | max | 180s | 3 |
| `digest` | 长输入短输出，压缩不判断 | `dashscope/deepseek-v4-flash` | off | 15s | 6 |
| `draw` | 生成一张图 | `dashscope/qwen-image-3.0` | default | 90s | 1 |

预留未用：`search`、`multimodal`。

**实测（2026-09-03，n=3，assess n=1，中位数）：**

| 档 | p50 | 首字 | 入 tokens | 出 tokens | 其中推理 |
|---|---:|---:|---:|---:|---:|
| `reflex` | 0.3s | 0.3s | 192 | 3 | 0 |
| `digest` | 1.4s | 0.7s | 303 | 67 | 0 |
| `dialogue` | 2.4s | 1.0s | 1,619 | 51 | 0 |
| `compose` | 2.9s | 0.9s | 559 | 123 | 0 |
| `review` | 9.0s | 6.8s | 319 | 356 | 256 |
| `assess` | 63.8s | 60.0s | 708 | 3,342 | 3,074 |

⚠️ **成本目前只能按 token 说，不能按钱。** 目录里 22 个 DashScope 模型
`priceUsd` 全是空的（只有直连 DeepSeek 和 Anthropic 有价）。把百炼控制台的
入/出费率填进 `models.json`，同一份实测结果不用重跑就能换算成钱。
**宁可记成本为空，也不要编一个数字**——这份目录存在的意义就是比成本。

💡 **这里最大的成本杠杆是「入」不是「出」。** lite 阅读陪练一轮送进去 4,090
tokens、拿回来 105，比例 39:1；整个 dialogue 档平均 32:1。换模型基本动不了这个数，
**prompt 和 refeed 的体积才动得了**。要降账单，从这儿下手比从绑定下手有效得多。

---

## 2 · 2026-09-03 基线实测（历史）

本节保留最初分档后的基线结果，不代表当前所有绑定。`framework ready` 的 2026-09-14
校准及当前 `review` 绑定见 `docs/2026-09-14-framework-ready-routing-benchmark-findings.md`。

判官 `dashscope/qwen3.7-max`（旗舰，不在任何候选名单里），1–5 分，n=3
（assess n=1）。**分数只在同一轮内可比**——换了判官就不能跨轮比绝对值。

### 最大的一笔收益不在绑定表里

lite 的阅读陪练、PBL 对话、写作间那批调用，原来**全都落在旗舰档、满额推理**上。
不是配错了：只有三条 lane 时，「要一点判断力」只能靠买下那条绝不降级的审阅档来换。
归档之后它们落在 `dialogue`（关思考），实测每轮 **1.2–4.0 秒**。同一个模型、同样的
判断力，少掉一整轮推理。

### `dialogue` —— 均分会把最要命的那次失败藏起来

| 模型 | pro 陪练 | lite 写作 | PBL 对话 | **lite 阅读陪练** | 最差 |
|---|---|---|---|---|---|
| **deepseek-v4-pro** | 4 | 5 | 5 | **3** | **3** |
| qwen3.8-max | 3 | 5 | 5 | **2** | 2 |
| glm-5.3 | **5** | 5 | 5 | **1** | 1 |
| glm-5.2 | 4 | 5 | 5 | **1** | 1 |

glm-5.3 拿了全场唯一的 pro 陪练 5 分，却在 lite 阅读陪练上是 1 分。判官原话：

> 「直接把答案告诉了学生（『装机容量衡量的是能力，不是真正发出的电』），严重违反
> 铁律①；且以『收工』结束了对话，等于在学生根本没有回答的情况下放她过去。」

四个学生面坏掉一个就是坏了。**这一档看最差一项，不看均分。**

### `compose` —— 三个并列，速度决定

| 模型 | 质量 | p50 | 出 tokens | 其中推理 |
|---|---|---:|---:|---:|
| **glm-5.3** | 4 | **4.1s** | 189 | **0** |
| kimi-k3 | 4 | 10.3s | 222 | 47 |
| qwen3.8-max | 4 | 12s | 511 | 307 |

三个都 4 分、结构都 100%，glm-5.3 快一倍半且不花推理。**这是这一轮唯一动的绑定。**
（第一轮 qwen3.8-max 曾被**生产解析器拒收 67%** 的输出——线上表现是阅读室退回那句
通用的「具体是哪一句」，卡也不再发。结构不合格一票否决。）

### `review` —— 2026-09-03 时四个模型全部把半成品判成 ready

| 模型 | 质量 | p50 | 出/推理 tokens |
|---|---|---:|---:|
| deepseek-v4-pro | 2 | 9.8s | 491 / 348 |
| qwen3.8-max | 2 | 17.5s | 534 / 369 |
| glm-5.3 | 2 | 4s | 181 / **0** |
| glm-5.2 | 3 | 20s | 1122 / 945（有一次调用失败，出局） |

**当时四个模型都判 ready。** 这定位出了 prompt 问题，因此当轮保持
deepseek-v4-pro 不动；该问题及绑定已由 2026-09-14 校准更新。

⚠️ 注意 glm-5.3 那一行：4 秒、**推理 token 为零**，在一个存在意义就是推理的档上。
早期的判分规则「同分挑最便宜」在这里选了它——这就是「成本不能压过差距、并列也不
构成换绑依据」两条规则的由来。

### `assess` —— 并列 5 分，不动（而且这一档绝不降级）

| 模型 | 质量 | p50 | 出/推理 tokens | 结构 |
|---|---|---:|---:|---|
| **deepseek-v4-pro** | 5 | 1m22s | 4489 / 4251 | 100% |
| qwen3.8-max | 5 | 1m20s | 3080 / 2788 | 100% |
| glm-5.3 | 1 | 4m43s | **16384 / 16382** | **0%** |

glm-5.3 把整个预算烧在思考上，**输出只有一个左括号**。qwen3.8-max 与在任者打平且更省，
是有据可查的备选，但并列不换绑。

### `reflex` / `digest` —— 干净的胜利

`reflex`（时机分类，输出就是一个 id）：qwen3.7-flash **300ms / 3 tokens**；
glm-5.3 1.5s、glm-5.3-flash 3.2s（两者即使在最低档仍花 45–54 个推理 token）。
结构都 100%，同样对就挑最快的。

`digest`（长对话压进会话记忆）：deepseek-v4-flash 与 glm-5.3 **都 5 分**，
1.4s vs 2.3s，都完整保住了必须保住的三样东西（学生自己区分「叶面积增加」与「碳汇」
的改口、NASA 那篇只支持叶面积的边界、写让步段比人均的决定）。

---

## 3 · 改绑定的正确姿势

**规则：不许凭感觉换。** 流程是——

```bash
cd apps/api
# 1. 想清楚要回答什么问题，改候选名单（不是把 20 个模型全扫一遍）
vim cmd/routebench/routebench-round2.json

# 2. 快档一起跑
DASHSCOPE_API_KEY=… go run ./cmd/routebench \
  -config cmd/routebench/routebench-round2.json \
  -cases reflex,dialogue,compose,digest,review -out round.md

# 3. assess 单独跑：单次调用 80 秒以上
DASHSCOPE_API_KEY=… go run ./cmd/routebench \
  -config cmd/routebench/routebench-round2.json \
  -cases assess -samples 1 -out round-assess.md

# 4. 人来改 models.json，并更新 findings 文档。工具只出推荐，不改生产配置。
```

跑完看「当前绑定」的画像（不比较，只画像）：

```bash
DASHSCOPE_API_KEY=… go run ./cmd/routebench -config cmd/routebench/routebench-current.json \
  -cases reflex,dialogue,compose,digest,review -out current-fast.md
```

**`cmd/routebench` 是与运行系统分开的**：不连数据库、不被 `cmd/api` 引用，有一条
测试守着这条线（`internal/routebench/separation_test.go`）。它用**真实生产 prompt**
和**真实生产解析器**跑候选模型。

### 判分规则（`internal/routebench/report.go`）

按顺序：

1. 调用失败、或**生产解析器**拒收过一次输出的 → 直接出局。结构不合格不是权衡，
   是学生看见一个报错。
2. 最差一项比「最好的那个的最差一项」低超过一分的 → 出局。
3. 剩下的按 **性能 > 速度 > 成本** 排：最差一项 → 均分 → p50 延迟 → token。

**成本只能打破质量和速度的平局，不能压过差距。** 所有档统一按最差质量、平均质量、
p50 延迟、token 排序；只有四项完全相同时才保留现有绑定。若所有模型都在关键用例上
低分，应该修 prompt 或 rubric，而不是依靠“保留 incumbent”掩盖问题。

**看最差一项，不看均分。** qwen3.7-plus 曾经在 dialogue 上均分和在任者打平，
而在 lite 阅读陪练上拿 1 分——「直接认定步骤已完成并结束阅读」。四个学生面坏掉
一个不是 75% 可用，是坏了。

### 启动即失败的几条

写错 model id、给 `assess` 指了非旗舰模型、给一个要求关思考的档绑了**完全无法**停止
推理的模型 —— 服务起不来，不会悄悄跑一周。`api --print-models` 打印目录与当前绑定。

---

## 4 · 踩过的坑（省下你几轮）

**① 模型的 API 旋钮是数据，必须实测，不能推断。**
`reasoning_effort` 不是一个统一枚举：

| | `low` | `high` | `max` | `xhigh` |
|---|---|---|---|---|
| qwen3.7-max / -plus / -flash、kimi-k2.6 | ✓ | ✓ | **400** | ✓ |
| ZHIPU/GLM-5.3 | ✓ | ✓ | ✓ | **400** |
| qwen3.8-*、deepseek-v4-*、kimi-k3、glm-5.2 | ✓ | ✓ | ✓ | ✓ |

**同一个家族的 3.7 和 3.8 都不一样。** 翻译表写在 `models.json` 的
`reasoningEffortAliases`。这条坑掉过一整轮实测：判官是 qwen3.7-max，assess 档发
`max`，于是每一次判分都 400，报告还照样按 token 印了一张推荐表。

**② `thinkingOffUnsupported` 不等于「停不下来思考」。** 它只表示「这条通道不收
关思考那个字段」。GLM-5.3 用 `reasoning_effort:"low"` 时推理 token 为 0。
`minReasoningEffort` 就是干这个的：把「关思考」翻译成这条通道能接受的最低档。

**③ thinking 开关按通道走，不按模型走。** 直连 DeepSeek 认
`thinking:{"type":"disabled"}`，**同一个模型经 DashScope 时这个字段被静默忽略**，
认的是 `enable_thinking:false`。写错不报错，只会让陪练悄悄恢复满额推理
（每轮 4,000–7,000 tokens、40–66 秒）。换通道后必须跑
`LIVE_LLM=1 go test ./internal/gateway -run TestLive` 验证推理 token 真的归零。

**④ 有的通道不收「只有 system 消息」的请求。** `ZHIPU/GLM-5.3` 会回
`code 1214 messages 参数非法`，而目录里其他模型都收。`requiresUserMessage` 会给
这类通道补一条最小 user turn。

**⑤ 判官不能是候选。** 自己给自己判卷不是测量。routebench 现在直接不启动。
判官必须是旗舰，且不在任何候选名单里。

**⑥ 有判官用例却一个分都没拿到时，不出推荐。** 缺一次测量必须读成「缺」，
不能读成「并列」。

---

## 5 · Prompt 放在哪、怎么改

**48 个 prompt 常量，绝大多数是 Go 里的反引号字符串**（见附录 B 的完整清单）。
分布：

| 位置 | 装什么 |
|---|---|
| `internal/agent/*.go` | **主力**。陪练姿态、时机分类、卡片示例、审阅、检索建议、周报、复盘等 |
| `internal/api/*.go` | 少数就近定义的：阅读陪练/带读、写作各步、计划生成 |
| `internal/pbl/*.go` | PBL 项目室的陪练与复盘 |
| `internal/interest/*.go` | 兴趣树：采集、路由、深挖、觉醒协议 |
| `internal/news/select.go` | 探索星图的新闻挑选 |
| `internal/evalbench/prompts/single_prompt_evalreport_v1.md` | 评估报告的 prompt，**独立 .md 文件** |
| `packages/contracts/cards/*.json`（34 张） | 工具卡 spec。**卡的文案与 schema 在这里，不在 Go 里** |
| `packages/contracts/skills/*.json` | 技能定义（info-literacy-course、writing-project） |

### 改 prompt 前必须知道的

1. **四条铁律**（`AGENTS.md` 顶部，2026-09-02 改写过）。特别注意①的**适用边界**：
   只针对**学生要提交的作业正文**。生成计划、从已陈述的问题派生大纲、编排、状态
   流转——都不是铁律①的事，别用它去卡这些。
2. **界面文案九条**（`AGENTS.md` §界面文案怎么写）。最容易犯的错是**把界面写成
   对话**：标签是名词（「结论」不是「你的结论，一句话」），按钮写「确认选择」不写
   「就这么定」，要她做事用「请」+ 祈使句。
3. **报错写法**：动词+失败，再接**后台原话**。学生和我们看到同一句。
4. **prompt 长度就是钱**。见 §1 的 39:1——lite 阅读陪练的入 tokens 是全系统最大的
   一笔，优先看它。

### 改完怎么验

- **结构合法性**：`cmd/routebench` 把模型输出喂**真实生产解析器**。prompt 改坏了
  结构，这里直接掉到 100% 以下。
- **任务命中**：有明确正确答案的用例在结构成功后执行自己的 `GoldCheck`。例如研究框架
  审阅逐样本校验 `ready` 是否命中 gold label；任何一次未命中都会淘汰候选，但不会混同为
  生产 JSON 解析失败。
- **质量**：judge 用例写在 `internal/*/benchcases.go`，用的是**真实的、未导出的
  prompt 常量**（不是复制一份——复制的会漂）。内容锚在验收主动脉那个场景
  （Phoebe /「中国是否让地球变得更可持续？」），每个用例里都埋了一个故意的破绽。
- **加用例**就在对应包的 `benchcases.go` 里加，`Judge` 字段写评分标准。

---

## 6 · 已知待办（建议从这里开始）

1. **`frameworkReviewSystem` 校准（2026-09-14）。** 已把正式 ready rubric 压缩为生产
   prompt，并增加四个同场景 gold 用例：半成品、宽泛目标必须 `false`；最低可用、成熟
   框架必须 `true`。专用配置在
   `apps/api/cmd/routebench/routebench-framework-ready.json`，结果与最终绑定见
   `docs/2026-09-14-framework-ready-routing-benchmark-findings.md`。后续改这个 prompt 必须
   重跑该配置；结构通过、gold 100% 与
   判官质量都不可互相替代。
2. **lite 阅读陪练 prompt 已压缩（2026-09-18）。** 4,090 是本手册记录的历史路由样本，
   不再作为当前精确基线。本轮同模型、同用例的改前／改后体检中，普通场景从约
   10.2k 降至 3.2k 入 tokens，长历史场景从 14.7k 降至 7.7k；调用与最终解析均正常。
   用例只有两次采样，教学行为仍需后续单独迭代。下一次改阅读 prompt 前，先用
   `make prompt-gate-capture` 重新采集本轮改前参照。
3. **compose 档只有一个判官用例**，结论比别的档薄。建议补一两个。
4. **填价格**：22 个模型 `UNPRICED`，填完成本就能按钱排。
5. **`dialogue` 要不要给一点推理预算**：现在关思考，所以「查看思考过程」那个折叠区
   在陪练对话上是空的。要让它有内容就得花延迟，这是个产品取舍。
6. **联网搜索还没接进任何学生流程**：`enable_search` / `search_options` 已经在网关上
   （按次开关，不按档）。注意实测——这条端点**不返回结构化 search_results**，只在
   正文里引用来源；溯源体检要可点开的 URL，仍得走 `internal/websearch`。

---

## 7 · 相关文档

- `docs/2026-09-14-framework-ready-routing-benchmark-findings.md` —— framework ready
  prompt 校准的最终结果与当前 `review` 绑定。
- `docs/2026-09-03-routing-benchmark-findings.md` —— 最初分档后的历史基线；本文 §2
  是它的结论摘要。
- `docs/superpowers/specs/2026-09-02-llm-routing-taxonomy-design.md` —— 分档设计。
- `apps/api/cmd/routebench/README.md` —— 工具本身。
- `AGENTS.md` —— 铁律、硬约束、界面文案规范。

## 怎么重新生成这份文档

附录 A / B 由脚本从源码抽取，说明栏是**代码自己的 doc comment**——不是谁转述的。
改了调用点或 prompt 之后：

```bash
cd apps/api
python3 tools/llm-inventory.py > /tmp/tables.md
```

然后把本文从「## 附录 A」到结尾整段替换成它的输出。**不要手改表格**：71 行的表
手工维护，一周之内就会是错的。

推论是：**想让某个调用点在这份文档里有一句像样的说明，就去给那个函数写 doc
comment。** 没有注释的地方表里显示「—」，脚本不会替它编一句。

核对数量：

```bash
cd apps/api
grep -rn "a\.route(\|a\.routeE(\|a\.routeFn(" --include="*.go" internal/ | grep -v _test | wc -l
```

（这个数会比附录 A 的 71 多几处：`internal/api/proposal_track.go` 里那几行是路由
接缝本身的定义，不是调用点，脚本把它排除了。）

---

## 附录 A · 全部模型调用点（71 处，按档分组）

### `reflex` — 一个标签或一次路由判断，无自由文本

共 4 处。

| 调用点 | 位置 | 这次调用在做什么（取自代码自己的注释） |
|---|---|---|
| `postCoach` | `internal/api/coach.go:167` | postCoach runs one continuous, spine-aware coach turn. |
| `postCoachStart` | `internal/api/coach.go:988` | postCoachStart is the explicit "start" gate (Task 3, studio onboarding): the student confirms she's ready, so 印记 opens 提案 (forming) and begins the 开题 … |
| `postCoachAdvance` | `internal/api/coach.go:1130` | postCoachAdvance acts on a one-tap nextStep (铁律②: 打开由学生确认). |
| `postSuggestPlacement` | `internal/api/placement.go:69` | placement.go — POST suggest-placement · 印记 suggests which research question a just-added reference belongs under (or null → 未归类). |

### `dialogue` — 学生当场看得见的一轮

共 14 处。

| 调用点 | 位置 | 这次调用在做什么（取自代码自己的注释） |
|---|---|---|
| `postReflectProjectCard` | `internal/api/card_reflect.go:89` | postReflectProjectCard persists a completed card envelope and returns a coach reply that responds to the card's compiled content. |
| `postChatTurn` | `internal/api/chat.go:212` | postChatTurn drives one RunChatStep for a thread and streams the result over SSE: an assistant text delta, then (if the moment fires) a fresh card off… |
| `postCoach` | `internal/api/coach.go:97` | postCoach runs one continuous, spine-aware coach turn. |
| `postCoachOpening` | `internal/api/coach.go:854` | postCoachOpening runs 印记's real-AI opening welcome — the very first thing the student sees on entering a project's studio, before she has said anythin… |
| `postCoachStart` | `internal/api/coach.go:950` | postCoachStart is the explicit "start" gate (Task 3, studio onboarding): the student confirms she's ready, so 印记 opens 提案 (forming) and begins the 开题 … |
| `postCoachAdvance` | `internal/api/coach.go:1123` | postCoachAdvance acts on a one-tap nextStep (铁律②: 打开由学生确认). |
| `postCourseAsk` | `internal/api/course.go:455` | postCourseAsk drives one free-Q&A course-coach turn (Task 6): the student asks a free question about the CURRENT step ({slug, ordinal}); the coach ans… |
| `postPblTurn` | `internal/api/pbl_turn.go:123` | — |
| `submitProjectCard` | `internal/api/projectcards.go:203` | submitProjectCard (SSE, Task 5) persists a filled card envelope, runs CompleteCard (mints the evidence node once the spec's completion predicates hold… |
| `explainReadingBlock` | `internal/api/reading_block.go:324` | explainReadingBlock is POST /api/v1/readings/{id}/blocks/{bid}/explain. |
| `postReadingCoachTurn` | `internal/api/reading_coach.go:1695` | postReadingCoachTurn is POST /api/v1/readings/{id}/coach. |
| `postProjectTurn` | `internal/api/studioturn.go:194` | postProjectTurn drives the agent runtime one step (RunAgentStep) for a Studio project and streams the result over SSE: at most one card, intervention,… |
| `postWritingOpening` | `internal/api/writing_setup.go:308` | postWritingOpening is POST /api/v1/writings/{id}/opening — the coach's first line, called by the frontend immediately after the setup dialog closes. |
| `postLiteWritingTurn` | `internal/api/writing_turn.go:447` | postLiteWritingTurn drives one coach turn for the writing room. |

### `compose` — 从已陈述的输入派生一个 schema 产物

共 31 处。

| 调用点 | 位置 | 这次调用在做什么（取自代码自己的注释） |
|---|---|---|
| `getAIUseDraft` | `internal/api/ai_use.go:117` | getAIUseDraft returns the objective record + a draft statement. |
| `sceneForCourse` | `internal/api/course_scene.go:109` | sceneForCourse handles POST /api/v1/courses/{slug}/scene. |
| `generateEssayGuideCard` | `internal/api/essay_statement.go:95` | — |
| `postExplorationGuide` | `internal/api/exploration.go:628` | postExplorationGuide is the "深挖一层" guide — the ONLY LLM-spending endpoint in S3's exploration surface. |
| `digExploration` | `internal/api/exploration.go:834` | digExploration hands OpenAlex candidates straight to the client-side tray — no persistence (adopting is a separate, explicit student action, 铁律①). |
| `proposeQuestionEdges` | `internal/api/exploration.go:1386` | proposeQuestionEdges wires POST /exploration/edges/propose. |
| `digSeeds` | `internal/api/interest_dig.go:169` | digSeeds 发那一次调用。返回 (种子, 失败原话)。 |
| `extractLiteAssignment` | `internal/api/lite_assignment_extract.go:107` | extractLiteAssignment handles POST /api/v1/lite/teacher/assignments/extract: one compose call turning pasted assignment text into {prompt, targetWords… |
| `generatePblPalettes` | `internal/api/pbl_look.go:56` | generatePblPalettes —— 从她第一关留下的关键词派生三组配色。 |
| `generatePblPersonas` | `internal/api/pbl_personas.go:90` | generatePblPersonas —— 印记先动：从她真做过的事里推出两三个可能的读者。 |
| `composeJourney` | `internal/api/project_create.go:167` | composeJourney runs the mid-tier journey composer and, if it yields a waived set, persists it + records a journey_composed event. |
| `postQuestionCardTurn` | `internal/api/question_card.go:79` | POST /projects/{id}/cards/question-card/turn {messages:[{role,text}]} |
| `summonReadingLens` | `internal/api/reading_lens.go:222` | summonReadingLens mints the card cardID names, or declines with the plain sentence to say instead. |
| `summonReadingLens` | `internal/api/reading_lens.go:225` | summonReadingLens mints the card cardID names, or declines with the plain sentence to say instead. |
| `summonReadingLens` | `internal/api/reading_lens.go:230` | summonReadingLens mints the card cardID names, or declines with the plain sentence to say instead. |
| `planReadingTasks` | `internal/api/reading_plan.go:370` | planReadingTasks is the plan generation itself, split out of the HTTP handler so the guided coach can plan on demand: 开始 is the only button she has, a… |
| `getReadingQuestions` | `internal/api/reading_questions.go:328` | getReadingQuestions is GET /api/v1/readings/{id}/questions. |
| `getTakeawayDraft` | `internal/api/reading_takeaway.go:205` | getTakeawayDraft is the "AI drafts, student confirms" step of the reading takeaway's split-hybrid: assembles the record half deterministically (readin… |
| `postLiteReadingTurn` | `internal/api/reading_turn.go:371` | postLiteReadingTurn drives one coach turn. |
| `postReadingTurn` | `internal/api/readturn.go:290` | postReadingTurn drives one turn of the read-together router. |
| `postSearchGuidance` | `internal/api/search_guidance.go:62` | POST /projects/{id}/search-guidance → { suggestions: [{keyword, why}] } |
| `surfaceAnchors` | `internal/api/studioturn.go:356` | surfaceAnchors generates the guidance-faded anchors (spec §3, L1/L2/L3 for annotate cards; always L1 for compare) for a just-surfaced annotate or comp… |
| `summonProjectCard` | `internal/api/summoncard.go:203` | summonProjectCard lets the student summon a CHOSEN reading card onto a material herself. |
| `summonProjectCard` | `internal/api/summoncard.go:206` | summonProjectCard lets the student summon a CHOSEN reading card onto a material herself. |
| `summonProjectCard` | `internal/api/summoncard.go:211` | summonProjectCard lets the student summon a CHOSEN reading card onto a material herself. |
| `generatePlanItems` | `internal/api/workspace_plan_generate.go:202` | generatePlanItems makes the one-shot mid-tier completion, meters it (purpose= "plan_gen") BEFORE any bail, and returns validated tasks. |
| `deepenWritingBlock` | `internal/api/writing_deepen.go:308` | deepenWritingBlock is POST /api/v1/writings/{id}/outline/{oid}/deepen — one turn of the block-scoped Socratic sub-agent. |
| `guideWritingBlock` | `internal/api/writing_guide.go:672` | guideWritingBlock is POST /api/v1/writings/{id}/outline/{oid}/guide — the single-block REGENERATE. |
| `guideWritingBlocks` | `internal/api/writing_guide.go:850` | guideWritingBlocks is POST /api/v1/writings/{id}/guide — the batch route Task 4 (B1) adds: guide EVERY block in the outline in ONE model call, persist… |
| `postWritingPlanTurn` | `internal/api/writing_plan.go:741` | postWritingPlanTurn is POST /api/v1/writings/{id}/plan/turn. |
| `suggestWritingTitles` | `internal/api/writing_title.go:278` | suggestWritingTitles is POST /api/v1/writings/{id}/title-ideas. |

### `review` — 判学生的成果，判错有代价

共 12 处。

| 调用点 | 位置 | 这次调用在做什么（取自代码自己的注释） |
|---|---|---|
| `postCoach` | `internal/api/coach.go:226` | postCoach runs one continuous, spine-aware coach turn. |
| `reviewFrameworkReadiness` | `internal/api/coach.go:706` | reviewFrameworkReadiness runs the flagship reasoning reviewer over the just-completed framework. |
| `reviseEssayClaim` | `internal/api/essay_statement.go:262` | POST /projects/{id}/essay-statement/revise-claim { subQuestionId, newText, confirm } §133–134 · the student edits a sub-question at the 大纲 step. |
| `reviewSubQuestionSaturation` | `internal/api/evidence_map.go:307` | POST /projects/{id}/evidence-map/subquestions/{sqId}/review |
| `postExplorationReview` | `internal/api/exploration_review.go:41` | POST /projects/{id}/exploration/review → { review: "..." } |
| `runDraftAnnotationReview` | `internal/api/proposal_annotations.go:49` | — |
| `evaluateProjectCard` | `internal/api/readeval.go:145` | evaluateProjectCard judges the student's picked sentence against the active reading-room card's lens. |
| `liteEvaluateCardSelectionFor` | `internal/api/reading_lens.go:356` | liteEvaluateCardSelectionFor judges the sentence the student picked against the open card's lens. |
| `orderSpotCheck` | `internal/api/spotcheck.go:89` | orderSpotCheck runs a station's spot-check. |
| `orderReview` | `internal/api/writing.go:340` | orderReview runs the student-triggered whole-draft review over a committed snapshot. |
| `commentOnSnippet` | `internal/api/writing_comment.go:518` | commentOnSnippet is POST /api/v1/writings/{id}/snippets/{sid}/comment — a spend endpoint (one model call), metered as purpose="block_comment". |
| `reviewWritingDraft` | `internal/api/writing_compose.go:231` | reviewWritingDraft is POST /api/v1/writings/{id}/review — a spend endpoint (one model call): gates on HasEntitlement and on a non-empty draft (400 mis… |

### `assess` — 过程评估 / 回顾 / 周报，绝不降级

共 3 处。

| 调用点 | 位置 | 这次调用在做什么（取自代码自己的注释） |
|---|---|---|
| `runReportGeneration` | `internal/api/evaluation_generate.go:120` | runReportGeneration gathers a project's recorded data, computes the FACT half, runs the four LLM calls, and assembles + validates the Report. |
| `getPblLookback` | `internal/api/pbl_lookback.go:146` | — |
| `composeWeeklyProse` | `internal/api/teacher_weekly.go:348` | composeWeeklyProse makes the flagship call and records its cost — including when the output is rejected, since a rejected composition still spent real… |

### `digest` — 长输入短输出，压缩不判断

共 6 处。

| 调用点 | 位置 | 这次调用在做什么（取自代码自己的注释） |
|---|---|---|
| `buildStarmap` | `internal/api/explore.go:477` | buildStarmap 抓 → 过滤 → 选 → 照着正文写。返回 (星球, 失败原话)。 |
| `harvestOneAtom` | `internal/api/interest_harvest.go:93` | harvestOneAtom 采集一个 atom。 |
| `harvestQuiz` | `internal/api/interest_quiz.go:247` | harvestQuiz 从这次作答里长词，并种进树。 |
| `addPblSiteRef` | `internal/api/pbl_sites.go:102` | addPblSiteRef —— 她粘一个网址进来。 |
| `maybeCompactBackstop` | `internal/api/projectcoach.go:164` | maybeCompactBackstop is S4 lever-1's size-threshold backstop, run at the tail of a coach turn. |
| `composeReturnSummary` | `internal/api/workspace_summary.go:113` | composeReturnSummary runs the flagship composer over the spine projection and records the call's cost (surface="studio", purpose="summary") BEFORE any… |

### `draw` — 生成一张图

共 1 处。

| 调用点 | 位置 | 这次调用在做什么（取自代码自己的注释） |
|---|---|---|
| `drawAndStore` | `internal/api/pbl_draw.go:41` | drawAndStore 画一张图，存进我们自己的 OSS，返回 object key。 |


## 附录 B · Prompt 存放位置（48 个常量）

**`internal/agent/ai_use.go`**

| 常量 | 行 | 说明 |
|---|---|---|
| `aiUseSeedSystem` | 35 | — |

**`internal/agent/chat_coach.go`**

| 常量 | 行 | 说明 |
|---|---|---|
| `chatCoachPosturePrompt` | 16 | chatCoachPosturePrompt is the Chat-variant system prompt (agent-spec §5.4): coach alone, guiding-not-answering, ONE notch more permissive th… |

**`internal/agent/claim_revision.go`**

| 常量 | 行 | 说明 |
|---|---|---|
| `claimRevisionSystem` | 34 | — |

**`internal/agent/coach_prompt.go`**

| 常量 | 行 | 说明 |
|---|---|---|
| `coachPosturePrompt` | 14 | coachPosturePrompt is the Slice-2 runtime coach's own restraint-ladder posture prompt. |

**`internal/agent/dig_query.go`**

| 常量 | 行 | 说明 |
|---|---|---|
| `digQuerySystem` | 18 | — |

**`internal/agent/digest.go`**

| 常量 | 行 | 说明 |
|---|---|---|
| `digestSystemPrompt` | 25 | — |

**`internal/agent/edge_proposer.go`**

| 常量 | 行 | 说明 |
|---|---|---|
| `edgeProposerSystem` | 64 | — |

**`internal/agent/evidence_saturation.go`**

| 常量 | 行 | 说明 |
|---|---|---|
| `evidenceSaturationSystem` | 43 | — |

**`internal/agent/exploration_guide.go`**

| 常量 | 行 | 说明 |
|---|---|---|
| `explorationGuideSystem` | 53 | — |

**`internal/agent/exploration_review.go`**

| 常量 | 行 | 说明 |
|---|---|---|
| `explorationReviewSystem` | 45 | — |

**`internal/agent/framework_review.go`**

| 常量 | 行 | 说明 |
|---|---|---|
| `frameworkReviewSystem` | 39 | — |

**`internal/agent/moment.go`**

| 常量 | 行 | 说明 |
|---|---|---|
| `momentSystemPrompt` | 115 | momentSystemPrompt is the classifier's posture. |

**`internal/agent/orchestrator.go`**

| 常量 | 行 | 说明 |
|---|---|---|
| `orchestratorSystemPrompt` | 33 | orchestratorSystemPrompt is the ONE posture for 印记-as-orchestrator. |
| `noteExtractPrompt` | 488 | — |
| `orchestratorOpeningPrompt` | 570 | orchestratorOpeningPrompt is the ONE crafted posture for 印记's real-AI welcome — the student's very first turn in a project, before she has s… |

**`internal/agent/placement.go`**

| 常量 | 行 | 说明 |
|---|---|---|
| `placementSystem` | 36 | — |

**`internal/agent/project_coach.go`**

| 常量 | 行 | 说明 |
|---|---|---|
| `projectCoachPosturePrompt` | 26 | projectCoachPosturePrompt is the one agent's posture across every room. |

**`internal/agent/project_summary.go`**

| 常量 | 行 | 说明 |
|---|---|---|
| `returnSummarySystem` | 17 | — |

**`internal/agent/proposal_guide.go`**

| 常量 | 行 | 说明 |
|---|---|---|
| `guideGenSystem` | 95 | — |

**`internal/agent/question_card.go`**

| 常量 | 行 | 说明 |
|---|---|---|
| `questionCardSystem` | 36 | — |

**`internal/agent/reading_takeaway.go`**

| 常量 | 行 | 说明 |
|---|---|---|
| `readingTakeawaySystem` | 59 | — |

**`internal/agent/review.go`**

| 常量 | 行 | 说明 |
|---|---|---|
| `reviewPosturePrompt` | 39 | — |

**`internal/agent/search_guidance.go`**

| 常量 | 行 | 说明 |
|---|---|---|
| `searchGuidanceSystem` | 36 | — |

**`internal/api/lite_assignment_extract.go`**

| 常量 | 行 | 说明 |
|---|---|---|
| `assignmentExtractSystem` | 19 | assignmentExtractSystem asks for the three writing-assignment settings the form has. |

**`internal/api/reading_block.go`**

| 常量 | 行 | 说明 |
|---|---|---|
| `readingBlockSystem` | 143 | — |

**`internal/api/reading_coach.go`**

| 常量 | 行 | 说明 |
|---|---|---|
| `readingCoachSystem` | 49 | — |

**`internal/api/reading_plan.go`**

| 常量 | 行 | 说明 |
|---|---|---|
| `readingPlanSystem` | 43 | — |

**`internal/api/reading_questions.go`**

| 常量 | 行 | 说明 |
|---|---|---|
| `readingQuestionsSystem` | 64 | — |

**`internal/api/workspace_plan_generate.go`**

| 常量 | 行 | 说明 |
|---|---|---|
| `planGenSystem` | 45 | — |

**`internal/api/writing_comment.go`**

| 常量 | 行 | 说明 |
|---|---|---|
| `writingCommentSystem` | 335 | — |

**`internal/api/writing_guide.go`**

| 常量 | 行 | 说明 |
|---|---|---|
| `writingGuideSystem` | 83 | writingGuideSystem — guides ONE block (POST /outline/{oid}/guide, the single-block regenerate). |
| `writingGuideBatchSystem` | 103 | writingGuideBatchSystem — guides EVERY block in the outline in ONE call (POST /writings/{id}/guide). |

**`internal/api/writing_plan.go`**

| 常量 | 行 | 说明 |
|---|---|---|
| `writingPlanSystem` | 89 | 🔑 每一个写进这段提示词散文里的方法名（『并列论证』『对比论证』『留个悬念』 『先抛一个问题』『开门见山』『先承认，再反驳』……）都必须**逐字**存在于 packages/contracts/vocab/methods.json 的某个 name 或 formal_name 里… |

**`internal/api/writing_setup.go`**

| 常量 | 行 | 说明 |
|---|---|---|
| `writingOpeningSystem` | 135 | writingOpeningSystem is the coach's opening line. |

**`internal/api/writing_title.go`**

| 常量 | 行 | 说明 |
|---|---|---|
| `writingTitleSystem` | 157 | writingTitleSystem asks for names for a finished piece — and nothing else. |

**`internal/interest/dig.go`**

| 常量 | 行 | 说明 |
|---|---|---|
| `digSystemPrompt` | 64 | — |

**`internal/news/select.go`**

| 常量 | 行 | 说明 |
|---|---|---|
| `selectSystemPrompt` | 81 | — |

**`internal/news/write.go`**

| 常量 | 行 | 说明 |
|---|---|---|
| `writeSystemPrompt` | 107 | — |

**`internal/pbl/coach.go`**

| 常量 | 行 | 说明 |
|---|---|---|
| `coachSystem` | 245 | — |

**`internal/pbl/look.go`**

| 常量 | 行 | 说明 |
|---|---|---|
| `paletteSystem` | 45 | — |

**`internal/pbl/lookback.go`**

| 常量 | 行 | 说明 |
|---|---|---|
| `lookbackSystem` | 82 | — |

**`internal/pbl/persona.go`**

| 常量 | 行 | 说明 |
|---|---|---|
| `personaSystem` | 56 | — |

**`internal/pbl/siteref.go`**

| 常量 | 行 | 说明 |
|---|---|---|
| `siteRefSystem` | 71 | — |

**`internal/store/sqlc/pbl_review.sql.go`**

| 常量 | 行 | 说明 |
|---|---|---|
| `answerPblReviewPrompt` | 67 | — |
| `createPblReviewPrompt` | 178 | — |
| `getPblReviewPrompt` | 301 | — |

**`internal/store/sqlc/writing_atom.sql.go`**

| 常量 | 行 | 说明 |
|---|---|---|
| `setWritingAssignedPrompt` | 466 | — |
| `setWritingOutlineGuide` | 497 | — |
