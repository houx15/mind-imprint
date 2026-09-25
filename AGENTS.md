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

1. **AI 不完全替代学生思考，而是鼓励学生思考。** 在明确是要提交的作业的内容中（例如写作项目），AI 不替学生撰写，而是提出建议、意见。在最后的复盘中，诚实介绍 AI 和人的分工。
2. **趣味与交互。** 这个产品需要比普通的文本 AI 更让用户可接受，需要充分设计交互形式，使用要点、按钮、分块、思维导图、便利贴、拖拽等等工具来让产品变得更有趣。
3. **输出清晰。** 教练的对话清晰、目标明确，建议一次只问一个问题，或者有多个要点时分点列出。
4. **过程即数据。** 学生跳过工具卡、让 AI 直接答，也被记录。摩擦被转化成信号，而不是被消灭。

> **编号变更（2026-09-02）：** ①③ 改写、②「不操纵」删除并换成「趣味与交互」。
> 代码与旧文档里还留着按旧编号写的注释——旧的 `铁律②` 指的是已删除的「不操纵」，
> 旧的 `铁律③` 指的是现在的③。读到旧注释按这条对照，不必逐个改写。

**铁律的适用边界（别扩大化）：** 铁律①「AI 不替学生撰写」**只针对学生要提交的作业正文（body text）**。**它不适用于确定性的系统步骤**：生成计划、从学生已陈述的研究问题或标准框架模板派生大纲/提纲，都是系统该自动做的事，不需要为此加一道学生确认门槛。不要用铁律①去质疑这些设计。

---

## 技术栈

> 注：早期为纯前端 SPA（localStorage + 浏览器直连模型）。现已定稿向「前后端分离平台」重构，以下为目标架构；细节见架构文档。

| 层 | 选型 |
|---|---|
| 前端 `apps/web` | React + Vite + TypeScript + Tailwind —— 纯渲染 + API 客户端，不持有密钥、不直连模型 |
| 后端 `apps/api` | **Go**（`net/http` + `pgx`/`sqlc` + `goose` + `river`）—— 智能网关（系统 prompt / `summon_card` / refeed / turn loop 都在服务端）、数据服务、鉴权、异步评估。唯一持有密钥、唯一访问 DB 与模型的单元 |
| 存储 | **PostgreSQL** —— `users`（含 `school_id`）/ `sessions` / `schools` / `classes` / `enrollments`（用户↔班级）/ `task` / `message`（旧任务面留存，不再有新写入）/ `card_instance` / `evaluation`（**暂定**）。无 `process_node` 表（过程树由 `parent_node_id` 投影）；`project` 面每次真实 LLM 调用（陪练 / 锚点生成 / 课程渲染）都记一行 `llm_call`（档位 + token + 成本），`llm_usage` 视图三路 UNION（`llm_call` + 冻结的 `message`/`evaluation`）供组织成本汇总 |
| 模型 | China-first。**模型目录 `apps/api/internal/gateway/models.json` 是「哪条通道跑哪个模型」的单一真相源**（go:embed）：providers（通道 + base URL + key 的 env 变量名 + thinking 开关）× models（价格 + capabilities）× **能力档**（reflex / dialogue / compose / review / assess / digest / draw，外加预留的 search / multimodal）。默认全部走**阿里云 DashScope（百炼）聚合端点**——一把 key 直达 Qwen / DeepSeek / GLM / Kimi，便于横向比能力、速度、成本。**平台持有 key**（服务端 env），经 `keyResolver` 接缝预留未来按组织计费。`assess` 档目前只接受 `flagship: true`（启动即校验）——这是一条**技术选择**，和下面那条「过程评估绝不降级」**不是一回事**，它一样要靠实测在效果与成本之间站得住。 |

**三层解耦（核心心智模型：工具卡 = tool-use 循环里「由人来执行的工具」）：**
决策层（用不用 / 用哪张，**服务端**）→ `summon_card(card_id, reason, nudge_text)` 单函数接线 → Card Runtime（schema 驱动渲染 + 三视觉态 + 事件采集 + 标准信封落库）→ 回灌陪练 → 所有标准信封长成过程树 + rubric 评估。卡 JSON 为单一共享真相源：前端构建期 import，Go `go:embed` 同一批文件。

---

## 给开发者的硬约束（不可违反）

- **每一份技术方案的第一性原理，都是效果与成本的平衡。**（产品负责人 2026-09-21）
  没有哪一档天然豁免。「这一档很重要」不是把它排除在权衡之外的理由，
  只是把它的效果那一侧定得更高而已 —— 而「效果」要拿判据量出来，不能靠形容词。
- **🚨「过程评估绝不降级」说的是数据，不是模型。**（产品负责人 2026-09-21 逐字纠正）
  它的意思是**过程评估必须建立在学生真实的过程数据上** —— 不许拿编的、
  拿别人的、拿概括过头的东西去生成那份报告。
  **它和「assess 档该用哪个模型」无关，别把它扩大成一条路由约束。**
  `assess` 档的 `flagship: true` 闸是一条独立的技术选择，要它自己的实测撑着
  （用例：`assess/lite-reading-report`，判据是金句必须逐字来自她说过的话）。
- **客户端绝不直连模型。** 所有 LLM 调用走后端网关，记录档位 + token + 成本；API key 只在服务端。
- **卡 spec 单一真相源在 registry。** 决策层目录从它派生，不手写第二份（避免 `trigger_condition` 漂移）。
- **每次 LLM 调用都要声明自己属于哪个「能力档」，而不是挑一条 lane。** 档说的是**这次调用需要多少智力**，不是它属于哪个功能：`reflex`（一个标签）/ `dialogue`（学生当场看得见的一轮）/ `compose`（从已陈述的输入派生一个 schema 产物）/ `review`（判学生的成果）/ `assess`（过程评估）/ `digest`（长输入短输出）/ `draw`（生成一张图）。调用点写 `a.routeE(ctx, gateway.ClassDialogue)`，由目录决定这今天意味着哪个模型。全清单与归属见 `docs/superpowers/specs/2026-09-02-llm-routing-taxonomy-design.md`。
- **换模型 = 改一个环境变量，不改代码；新增模型 / 新增 OpenAI 兼容厂商 = 改 `models.json`，不写 Go。** 每档一个变量（`MODEL_DIALOGUE` / `MODEL_COMPOSE` / …）指向目录里的 model id（如 `dashscope/qwen3.8-max`），只动一条、其余不变，测出来的速度与成本才可归因。旧的 `MODEL_CHAT` / `MODEL_FAST_CHAT` / `MODEL_EVAL` 仍作为别名生效（→ dialogue / reflex / assess）。写错 id、给 `assess` 指了非旗舰模型、或给一个要求关思考的档绑了停不下来思考的模型，**启动即失败**，不会悄悄跑一周。`api --print-models` 打印目录与当前绑定。
- **重新绑定要有实测撑着，不能凭感觉。** `go run ./cmd/routebench` 是一件**与运行系统分开**的工具（不连数据库、不被 `cmd/api` 引用、有测试守着这条线），用真实 prompt 跑候选模型，分开测生产解析、gold 任务命中和判官质量；硬门槛通过后按最差质量 → 平均质量 → 延迟 → **成本（钱，不是 token）**排序，只有全部指标完全相同才保留现有绑定。改 `models.json` 的是人。见 `apps/api/cmd/routebench/README.md`。
  **2026-09-20 改成按钱排**：在此之前 DashScope 一个模型都没有价格，只能按 token 排，而那在单价差一个数量级时会给出**反的**结论（qwen3.7-flash ¥0.2/¥0.8 对 deepseek-v4-pro ¥12/¥24，差 60 倍，少吐几个 token 补不回来）。价格按 `priceCny` 记在 wire 名上；没有价格的模型仍退回 token，因为缺测量要读成缺测量、不能读成打平。
- **成本要把缓存算进去，否则高估约四倍。** 百炼的**隐式缓存关不掉**：公共前缀超过 1024 token 就可能命中，命中部分按输入单价的 **20%** 计费。阅读陪练每轮重发全文，但稳定块排在前、易变块排在后，所以从第二轮起 **91%–99% 的输入是命中的**（2026-09-20 实测）。`ChatUsage.CachedInputTokens` + `gateway.EstimateCostCached` 就是为此存在，`LIVE_LLM=1 go test ./internal/gateway -run TestLiveCachedTokens` 守着这条线。
  **🚨 因此 prompt 块的顺序是一条成本契约**：把任何易变的东西（计时、还差多少字、板的状态）挪到正文前面，就把整个前缀打碎了，那一轮全价。改 prompt 组装顺序前先想这件事。
  推论：**「费 token」和「费钱」不是一回事**，裁文章省下来的只有票面的五分之一——真正的杠杆是档位选型。见 `docs/2026-09-20-lite-ai-cost-and-routing.md`。
- **🚨 `draw` 档不走对话口，它的端点是实测出来的，不是照文档推的。** 2026-09-04
  实测：聊天走的那个 DashScope maas 聚合口**画不了图**——`/images/generations`
  在两个主机上都是 404，尽管它的 `GET /models` 里就列着 `qwen-image-3.0`；
  拿图像模型打 `/chat/completions` 是 400。图要走 DashScope 原生的
  `/api/v1/services/aigc/multimodal-generation/generation`，请求体是
  `input.messages[].content[]`（一个**数组**，给字符串会 400），回包读
  `output.choices[].message.content[].image`。所以 `models.json` 里有一个独立的
  provider（`dashscope_image`，另一个主机、另一种 wire shape），三个图像模型挂在
  它上面，`validate()` 对 `draw` 档查的是画图能力而不是聊天能力。
  **那个图片地址带签名会过期**，调用点必须马上取下来存进我们自己的 OSS
  （`internal/api/pbl_draw.go`），绝不把它存进数据库。换模型或换通道之后跑
  `LIVE_LLM=1 go test ./internal/gateway -run TestLiveDraw` —— 这条第一次跑就抓到
  了目录注释里那句写错的端点。

- **thinking 开关是「按通道」的数据，不是写死在 adapter 里的代码。** 同一个模型换条路，关思考的字段就不一样：直连 DeepSeek 认 `thinking:{"type":"disabled"}`，**同一模型经 DashScope 时这个字段被静默忽略**，认的是 `enable_thinking:false`（2026-09-02 实测）。写错不会报错，只会让陪练悄悄恢复满额推理（每轮 4,000–7,000 completion tokens、40–66s）。故 `thinkingOff` 写在 `models.json` 的 provider 上；换通道后必须跑 `LIVE_LLM=1 go test ./internal/gateway -run TestLive` 验证 reasoning token 真的归零。
- **新增卡 = 新增一份 JSON 配置，不改渲染器代码。** 这是 schema 驱动是否成立的验证标准；用现有字段原语（`text` / `textarea` / `single_choice` / `multi_choice` / `rating` / `repeatable_group` / `link_check`）拼，除非真需要全新交互才加原语。
- **标准信封结构一旦定下不要随意改**——它是过程树、使用计数、评估的共同地基。后端按「边界校验」存储：Go 只校验信封外层（`status` 枚举、id、`field_values` 为对象、`event_trace` 为数组），内层深结构真相仍归 `packages/contracts` 的 Zod 契约。
- **组织不变式：每个账号必属于某个学校；学生/教师还属于 ≥1 个班级，不存在「无组织账号」。** 注册必须携带有效班级 join code；建号（`school_id` 取自 `class.school_id`）与 `enrollments` 在同一事务内完成，否则整体失败。**结构归属（学校/班级）与「会员/付费权益」是两回事**：付费权益暂不建表，由后端 `HasEntitlement(ctx,user)` 接缝（当前恒 true）在消耗 token 的端点前判定，未来接订阅/代币模型。
- **密钥只在 `apps/api` 服务端**（LLM key / DB DSN / session secret / SMTP），绝不进 git、日志、抛出或渲染的错误、数据存储、评估 payload。
- **测试只写「逻辑测试」，不堆前端渲染测试。** 值得测的是「读代码看不出对错」的东西：纯函数与其边界（字数统计、截断、去重、label/unit 解析）、reducer / 状态机、请求响应的整形与 normalizer、Go handler、权限与归属校验。**不要**为每个组件写「标题渲染了 / 类名存在 / prop 传下去了 / 列表有 N 项 / 文案 X 出现了」这类断言——它们写起来和维护起来都很贵，每次正常改版都会碎，而且抓不到真正会坏的东西。UI 用**真浏览器看**（一次性 Playwright harness + 截图），而不是靠 jsdom 断言：jsdom 看不见布局、`sandbox`、光栅化。2026-08-30 的教训：lite-web 344 个测试全绿，而导出的报告 PNG 是**全白的**。只保留少数「人眼盯不住的不变量」类前端测试，并在旁边写清楚为什么。
- **课程摆给谁看，由 `course.audience` 决定，两个版本共用同一份课程库。** 一门课的受众是 `text[]`（今天的取值是 `lite` / `pro`，空数组 = 不限受众），词表在 `packages/contracts/src/course.ts` 与 `apps/api/internal/api/course_audience.go` 两处，**不写 DB CHECK**（同 `category` 的理由）。过滤在服务端做，目录和按 slug 读的每条路径都要过（`course.go` 与 `course_visibility.go`）——只藏目录，一条手敲的 URL 就绕过去了。前端不做受众判断。
- **项目里能把学生送去上一课，且必须闭环。** 印记 `produce("course")` 只能从她这个版本看得见的课程库里挑（编出来的 slug 服务端丢掉），落一条 `pbl_course_assignment`；她在项目里的浮层上完课，写下「这一课对我的项目有什么用」，那句话经 `gatherPblCourseWork` 回灌给印记，印记接一轮。**新增任何一件会离开对话的工具，都要照这条走完「终点信号 + 产出回灌」，否则那件工具是项目外面的一件事。**
- **涉及「写作项目」流程的实施 plan（立题/管理/阅读/写作/回顾各状态、其页面/卡片/AI 角色/状态转移），写完后必须先对照 `docs/2026-08-09-all-statuses.md` 的相关章节逐条校验一致性，再进入实现。** 该文档是写作全流程行为的单一真相源；plan 与它冲突时，以它为准（或显式回到用户确认）。同理，架构 spec 也以它为行为真相源。**不涉及写作项目流程的工作（如引导旅程、课程、图鉴、营销站等）不受此约束。**
- **研究框架的 `ready` 是非阻塞审阅语义。** 权威定义见 `docs/2026-08-09-all-statuses.md` 的 Research framework：它判断现有输入能否派生初步计划，不是正式提案验收，也不得改变计划自动生成或状态流转。实现 `frameworkReviewSystem` 或其 benchmark 时，目标/活动/资源/整体连贯性关键缺口（以及占位或无关缘由）必须 `false`；反例缺失不可单独否决。

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

---

## 验收主动脉（端到端演示脚本）

场景锚定 **Phoebe / 「中国是否让地球变得更可持续？」**：新建任务粘文章链接 → AI 调 `summon_card("craap")` → 学生用 CRAAP 对来源做溯源体检，追到 NASA / Nature Sustainability → 摘要回灌、一次只问一个 → 写论证撞反例（中国碳排放全球第一）→ AI 调 `summon_card("concession")` 完成让步段 → 过程树实时生长 → 评估那一刀跑出 rubric 评级 + 过程叙述，以「你的思维印记」呈现。mockup 用真实内容，别用 lorem ipsum。

---

## 界面文案怎么写（2026-09-02 定，来自产品负责人逐条改写 444 行文案）

来源：`docs/2026-09-02-ui-wording-table.md`。下面每一条都是从「我写的 → 她改成的」里
归纳出来的，不是凭空的偏好。写任何学生能看见的字之前，先过一遍这十条。

**0 · 最容易犯的错：把界面写成了对话。**
印记在对话里说话；界面负责给东西命名。一个标签、一个按钮、一句提示，都不是印记
在跟她讲话。
> 「你的结论，一句话」→「结论」　「为什么这么定？」→「原因」　「这一步要干嘛」→「内容」

**1 · 标签是名词，不是句子。** 能用两个字说清就不用一句话。

**2 · 按钮写「做什么」或「做完了」，用书面词。**
> 「就这么定」→「确认选择」　「先到这里」→「完成」　「结构就这样」→「没有问题」　「就这么分」→「方案无误，开始执行」

不要「就…」「先…」这种口语起手式。

**3 · 要她做事就用「请」+ 祈使句。**
> 「看一遍。改哪儿都行，你说了算。」→「请审核计划并确认，或提出修改意见」
> 「想说一句为什么吗？不想说也行。」→「请阐述原因」　「跟印记说」→「请输入」

**4 · 状态用「已/待/中」的成对词，别用大白话。**
> 定了→已确定　做完了→已完成　改过了→已修改　不做了→跳过　等结果→处理中　等你定→需要确定

**5 · 先说这件事为什么值得做，再请她做。** 一句话的理由放在前面。
> 「一段一段看过去，不同意就说出来」→「AI 可能出错，需要对它产出的内容做一次深度审核」
> 「上线之后拿回来的数据，说明了什么」→「持续观察成果落地后的反馈，进一步迭代你的成果！」

**6 · 用真正的专业词，不要土话式比喻。** 学生来这儿就是要学这套词。
> 看数据→收集数据　读出意思→数据分析　改一件事→产品迭代　别人说的→访谈/调查结果　我猜的→推论

**7 · 不要铺垫、不要撒娇、不要替她减压。**
「一句就够」「不用写得好看」「随便写点什么」这类话一概删掉——它们假设她怕，
而这个假设本身就不尊重人。

**8 · 报错：动词+失败，再接后台原话。**
> 「没加上，再试一次。」→「添加失败」　「没记下。」→「记录失败：{后台原话}」

原则是「尽可能给出详细报错信息，方便 debug」，学生和我们看到的是同一句。

**9 · 感叹号留给真正的节点。** 复盘开始、审核通过、成果上线可以有一个；
平时的标签和提示不要。

**10 · 不写文学腔（2026-09-03 加）。** 界面和印记说的话都是说明文，不是散文。
以下一律删掉：

- **比喻、拟人、把系统写成会自己动的东西**：「让印记像老师一样通篇体检」
  「计划会自己长出来」「阅读进度自然地长在图上」。
- **抒情副词**：悄悄、慢慢、渐渐、轻轻、一点一点、一步一步、终于。
- **用「——」拖一句感慨**：「剩下的是它在真实的雨里能撑多久，那个只有时间知道」。
- **把动作说成心情或情景**：「回到第一次进来的样子」「这些都逛完啦」。
- **办公室套话**：深耕、打造、赋能、沉淀、助力、闭环（当形容词用时）。
- **替读者预告感受**：「你会发现……突然有了答案」。

> 「印记陪你一点一点想清楚」→「印记与你一起把它想清楚」
> 「计划会自己长出来」→「计划会据此生成」
> 「让印记像老师一样通篇体检」→「请印记通篇审阅」
> 「把学到的东西沉淀下来」→「把学到的东西记录下来」
> 「这些都逛完啦！」→「引导到这里结束」
> 「值得慢慢看」→「值得细读」
> 「回到第一次进来的样子」→「恢复初始状态」

**一句话的检验：把修饰全部拿掉，这句还剩下信息吗？** 不剩就整句删掉；
剩下的那部分就是该写的那句话。

注意边界：**领域词不是文学腔。** 兴趣树的「词」「枝」「点亮」、图鉴的「集齐」
是这个产品真实的对象名，照常用；被禁的是拿比喻去替代本来就有的专业词。
学生自己写的话、教学内容里故意写坏的示例（比如那份有引导性的问卷）、
外部文章的引文、输入框里的「……」占位符，以及代码注释，都不在这条约束内。

**「长出来 / 长成」只留给树。** 2026-09-03 全库清查发现这是最泛滥的一处：
兴趣树、过程树、枝、词是真的树，说「长出来」对；把它挪到计划、地图、报告、
段落、网页上就是比喻转移，一律换掉——
> 计划会自己长出来 → 计划会据此生成　　长成一张证据地图 → 汇成一张证据地图
> 长成一份过程评估报告 → 形成一份过程评估报告　　长成一段主体论证 → 对应一段主体论证
> 它还没长成你的主问题 → 它还没成为你的主问题

---

## 提示词怎么写（2026-09-22 定，来自产品负责人的提示词优化那一轮）

那一轮（`b151e7ed` 起）把 127 个文件里的提示词抽进 `apps/api/internal/prompts`，
同时把每一段**重写了一遍**。重写后的那一版就是标准，**以后写任何提示词都照它**。

### 1 · 分三层，各住各的文件

- `internal/prompts/` —— 固定的教学文本，Go 常量。
- 功能自己的 `*_prompt.go` —— 条件选择与装配（语言、文体、重问的那句话）。
- 功能自己的 `*_context.go` —— 上下文的挑选与投影。
- handler —— 取数据、调网关、校验、落库，不拼字符串。

没有运行时模板引擎、没有远程提示词服务、没有第二套模型配置。
`gateway/models.json` 仍然是模型路由的唯一真相源。

### 2 · 🚨 发给模型的字里**只有指令**

理由、出处、日期、谁在什么时候提的意见、为什么这么定 —— **一律不进提示词**，
写在那个常量上面的 Go 注释里。`prompts/doc.go` 的原话：
「Comments are maintenance metadata, never model input.」

- 写在注释里：🚨 2026-09-18 产品负责人：「个人经历是信效度最低的……」
- 写进提示词：材料要具体，并说明它和观点的关系。

理由是给改这段代码的人看的；模型只需要知道该做什么。

### 3 · 陈述句，不是训斥

用「在什么情况下该怎么做」，不用「绝不许……」这种绝对句式。

- 旧：**绝不要把这张表甩给她挑。** 不许问「你想用总—分还是总—分—总」。
- 新：学生尚未说明观点时，不要求她先选结构名称；先了解学生想表达的内容，
  再据此整理结构。

规矩没变，语气变了。**判据也要跟着改**：钉住新那句话，别留着钉旧措辞的测试。

### 4 · 不打比喻，直接说那个东西的名字

界面文案规则 10 同样适用于提示词，而且提示词里要写明这一条：
「描述内容归属时使用节点名称，不说『挂在半空中』等比喻。」

### 5 · 术语只从注册表来

方法名逐字来自 `packages/contracts/vocab/methods.json`；块的种类逐字来自
`writing_kind.go` 的闭表。散文里现造一个词，就是在同一口气里教模型造词。

### 6 · 🚨 改提示词要有 parity 兜底

`cmd/promptinspect` 能离线渲染每一条装配（不连库、不要 key），
`TestMainRequestParity` 按 wire request 求 sha256 和基线逐条比。改之前先跑一遍
`go test ./cmd/promptinspect`；要对着另一份基线比，用环境变量
`PROMPT_COMPARE_BASELINE` 指向旧版导出的 json。

🚨 **基线不等于覆盖。** 2026-09-22 那份基线 28 个样例，写作立题只覆盖了中文
议论文一种 —— 英文议论文、中文记叙文、英文记叙文三条分支不在里面。占位符
（`@@KINDS@@` 之类）在没被覆盖的那条分支上漏掉，整套测试照样绿，而线上那一篇
收到的是字面写着 `@@KINDS@@` 的提示词。新开一条分支就补一条装配测试
（`TestWritingPlanSystemFor_EveryGenreAndLangAssembles` 是模板）。

### 7 · 重构时逐字保留，不顺手改意思

wire 的内容、顺序、预算、截断规则都要原样保留；确实要改措辞，就**单独一次提交**、
单独跑一次 `LIVE_LLM`，别混在搬家那一次里。混在一起时 parity 会红成一片，
而红的原因分不出是搬坏了还是改了意思。

## Shared Memory

**Always write new instructions, rules, and memory to `AGENTS.md` only.**

Never modify `CLAUDE.md` or `GEMINI.md` directly - they only import `AGENTS.md`.
This ensures Claude Code, Codex CLI, and Gemini CLI share the same context consistently.

## Project Structure

- `apps/web/` - 前端 SPA（React + Vite）
- `apps/api/` - 后端 Go 服务（重构后新增）
- `apps/site-v2/` - 当前官网（Astro 静态站，中英双语；发布入口 `deploy/deploy-site-v2-artifact.sh`）
- `apps/lite-web/` - 轻量版学生端与教师端
- `apps/site/` - 旧版官网源码，非当前官网
- `archive/peraspera/` - 已归档的独立旧站，不属于当前产品或官网。处理「官网」「前端」「品牌英文名」等任务时，不检索、修改或部署此目录；只有用户明确点名 Per Aspera 时才重新纳入范围。归档目录不参与 pnpm workspace 与产品镜像构建。
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
