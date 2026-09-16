# 教师端工作台设计（对话 + 画布）

**日期：** 2026-09-16
**版本：** lite（`apps/lite-web` + `apps/api`）
**前序：** `docs/superpowers/specs/2026-09-15-lite-homework-grading-and-finished-writing-design.md`（A/B/C 三部分已上线，main `7ecd8ab5`）

---

## 1. 为什么做这件事

产品负责人在 A/B/C 上线后看了线上的教师端，给出两条意见：

> the empty state is very empty, without good illustrations (we have some, reuse them).
>
> the giving homework experience is not smooth yet. it looks like a very traditional
> thing without any AI features. and it is like entering a form, instead of a teacher
> working place.

查证后两条都成立：

- 教师端有约 20 处空状态渲染成一行灰字（`暂无班级`／`暂无作业`／`暂无学生`）。而项目里已经有一套插图：`apps/lite-web/src/learning/StudentArtwork.tsx` 里的 7 张 webp，以及配套的 `StudioEmpty`（`teacher/StudioArtwork.tsx`）。教师端只有 3 处用了它。
- 布置作业是教师端唯一没有 AI 的事。家长报告、AI 批改、个性化阅读都是「AI 起草、老师审阅」；布置作业仍是一张七格表单（班级 → 类型 → 标题 → 说明 → 截止 → 材料 → 学生 → 发布），全部由老师从零填写。

她随后给出了形态：

> left part, chat with AI. right part, generated content (homework preview, selected
> material. etc. for report part. ai generated report, can chat with ai to modify.
> main page, can ask ai questions about students and learning data.)
>
> different tab, different context. different login, different context. save token right.
> and we don't need very high level models here.
>
> in front page, the original card is great. can AI firstly gives a summary? and click
> can open a chat to chat more, and AI can even bring teacher to different pages.
>
> in homework page, ai mode and traditional mode... ai provides forms, which are mainly
> selections (like the idea of claude design), and right part the homework card appears,
> and can click to send.

## 2. 目标

教师端的三个面共用一套「对话 + 画布」：左边对话，右边是正在做的那件东西本身。对话是入口与加速器，画布是产物，两边都能改。

**不做的事**（YAGNI）：不做跨页面的长对话；不做对话历史入库；不做教师端的语音；不新增插图素材；不动学生端。

## 3. 全局约束

以下每条都必须在实现中成立，任何任务的要求都隐含包含本节。

1. **每次 LLM 调用声明能力档，并计量。** 对话轮走 `gateway.ClassDialogue`；主页摘要走 `gateway.ClassDigest`。**不使用 `ClassAssess`，不使用旗舰档。** 每次调用落一行 `llm_call`（档位 + token + 成本），与项目其余部分一致。
2. **上下文按页面与登录隔离。** 一个页面的对话只携带：当前教师身份、当前班级／报告、该页面的画布对象、该线程的最近 `TEACHER_TURNS_WINDOW = 8` 轮。任何一轮都不得携带另一个页面的线程。
3. **数字与姓名不由模型写。** 见 §6。
4. **教师可见性边界。** 教师工具可以读学生产出的一切（笔记、草稿、金句、兴趣树证据），**绝不读学生与印记的对话记录**。边界写在工具里，不写在 prompt 里。
5. **错误必须显形。** 模型调用失败、解析失败、工具失败，一律按「动词+失败：{后台原话}」显示，不得返回一句看起来合理的兜底话。
6. **界面文案守 AGENTS.md §界面文案怎么写。** 标签是名词；按钮写「做什么／做完了」；状态用已／待／处理中；报错「动词+失败：{后台原话}」；不写文学腔（含代码注释与 commit message）。
7. **lite 不得破坏 pro。** 共用的 Go 包、`queries/`、`migrations/`、sqlc 目录按 `lite-must-not-break-pro` 的规矩办；新建文件前先确认同名文件不存在。
8. **前端只写逻辑测试。** 纯函数、reducer、normalizer、Go handler、权限校验。不写「标题渲染了／类名存在」这类断言。UI 用一次性 Playwright harness 看截图，看完删掉。
9. **`mk-*` token 不用 Tailwind alpha 语法**（用 `color-mix`／`linear-gradient`）；不加左侧色条。
10. **密钥只在服务端。**

---

## 4. 共用的壳：对话 + 画布

### 4.1 布局

```
┌ <页面标题> ─────────────────────────────────────────────┐
│ ┌────────────────────┬────────────────────────────────┐ │
│ │ 对话                │ 画布                           │ │
│ │                    │                                │ │
│ │ 老师说的话          │  正在做的那件东西               │ │
│ │ AI 的回复           │  每一格都能点开直接改           │ │
│ │ [选项][选项][选项]  │                                │ │
│ │                    │  ─────────────────────────     │ │
│ │ [输入框]  [发送]    │        [ 主操作按钮 ]          │ │
│ └────────────────────┴────────────────────────────────┘ │
└─────────────────────────────────────────────────────────┘
```

宽屏两栏；窄屏（`< 900px`）改为上下两段，画布在上、对话在下——老师在手机上先要看见作业长什么样。

左栏最小宽 320px、最大 420px；画布吃掉剩下的宽度。整页用 `TeacherPage width="full"`——这一档的注释写着它就是「报告编辑器的两栏（草稿与预览）」，工作台是同一个形状，不新增测量档。

### 4.2 线程的生命周期

线程存在**页面里**，不入库。刷新页面：画布保留（作业草稿／报告本来就在服务端或表单状态里），对话清空。这是刻意的省 token 设计，也是「different tab, different context」的落地。

每一轮请求携带最近 8 轮。超出的轮次直接丢弃，不做压缩、不做摘要——lite 没有压缩层，这个窗口是唯一约束 prompt 增长的东西（与阅读带读的 `readingCoachTurnsWindow = 14` 同一思路）。

### 4.3 AI 的回复以「选择」为主

这是让它不像表单的关键。模型不问开放问题，它给一个问题加 2–4 个选项，面板把选项渲染成按钮。

> 这次偏重哪一块？
> 〔论证结构〕〔证据使用〕〔语言表达〕

点一下即发回该选项。输入框始终在，但它不是主路径。

**规矩：** 一轮最多一个问题（铁律③）。选项是选项，不是三个并排的问题。选项文字是名词或短动宾，不写成句子。

### 4.4 接口

新增一个端点，所有三个面共用：

```
POST /api/v1/lite/teacher/workspace/turn
```

请求：

```jsonc
{
  "surface": "assignment" | "home" | "parentReport",
  "classId": "uuid",        // assignment / home 必填
  "reportId": "uuid",       // parentReport 必填
  "artifact": { },          // 画布现在的样子，按 surface 定形
  "turns": [                // 最近 8 轮，客户端持有
    { "role": "teacher", "text": "…" },
    { "role": "ai", "text": "…" }
  ],
  "text": "老师这一轮说的话",       // 二选一
  "choiceId": "tier_advanced"      // 她点了某个选项时
}
```

响应：

```jsonc
{
  "reply": "markdown",
  "choices": [ { "id": "…", "label": "论证结构" } ],   // 0–4 个
  "patch": { "title": "…", "dueAt": "…" },              // 只含模型改过的字段，可为空
  "cards": [ { "kind": "students", "rows": [ … ] } ],   // 见 §6，由工具结果直接渲染
  "navigate": { "view": "classWeekly", "classId": "…" } // 仅 home，可为 null
}
```

鉴权：`liteOnly` + 教师角色 + 该班级／该报告属于这位教师（沿用 `lite_teacher_*.go` 已有的归属校验）。

### 4.5 画布的改动用 patch，不用整体替换

服务端只回它改过的字段。客户端把 patch 打到**当前**状态上，并且**丢弃老师在这一轮进行中手改过的字段**。

理由：一轮在飞的时候老师完全可能去改截止时间；整体替换会把她的修改抹掉，而这类竞态不会被任何测试抓到。

被丢弃的字段在画布上显示一行：`截止时间 已保留你的修改`。

实现：客户端在发起一轮时记下画布快照，收到 patch 时逐字段比对当前值与快照值——不等就说明老师改过，该字段的 patch 丢弃。这是一个纯函数（`applyPatch(current, snapshot, patch)`），有逻辑测试。

### 4.6 工具循环

模型不直接看数据库。它调用工具，服务端执行后把结果喂回同一轮（`internal/gateway` 已支持 `tool_calls` 与流式 tool 参数）。

工具循环上限 **4 次**往返；超过即失败并显示「对话失败：工具调用次数超出上限」。不静默截断。

---

## 5. 三个面

### 5.1 D1 · 布置作业：传统模式 / AI 模式

作业页顶部加一个两档切换：

```
模式  〔传统〕〔AI〕
```

- **传统模式 = 今天线上的表单，一个字都不改。** 它是保底路径，也是老师心里已有答案时最快的路径。
- **AI 模式 = 本文的壳。**

默认值：读 `localStorage` 里这位教师上次选的模式；没有记录时默认 **AI 模式**（新东西要被看见）。切换模式时草稿内容保留——两边操作的是同一个 `AssignmentDraft`。

画布 = 作业卡：类型 / 标题 / 说明 / 材料 / 截止时间 / 学生，底部一段折叠的「学生看到的样子」，再底下是 `发布作业`。每一格都能点开直接改（复用 `AssignmentForm` 已有的 `Field`、`SettingsFields`、`RecipientChecklist`，不重写一套）。

**工具：**

| 工具 | 作用 | 返回 |
|---|---|---|
| `set_fields` | 写 `kind` / `title` / `instructions` / `dueAt` | patch |
| `search_library` | 按关键词、学科、难度查分级阅读库 | 最多 8 篇（slug、标题、学科、可用难度） |
| `set_material` | 定材料：`library`（slug + tier）／`personalized`／`text` | patch |
| `list_students` | 按条件取名单，条件见下 | 真实行 → `cards` |
| `set_recipients` | 定收件学生 | patch |
| `ask_choice` | 给一个问题加 2–4 个选项 | 本轮终止 |

`list_students` 的条件是一个**闭集**，不是自由查询：`all`、`noWritingThisWeek`、`noWritingTwoWeeks`、`belowTier`。闭集的理由是可验证、可测试、不会让模型编出一个我们没实现的过滤条件。

**个性化材料：** `set_material` 选 `personalized` 时不在这一轮算人选——它写进 patch，由已有的 `PersonalizedPicker` 预览接口去取每个学生的文章（C 部分已建好）。AI 模式不重复实现选人逻辑。

**截止时间：** 模型只能给北京时间的绝对时刻。「周五」这类相对说法由服务端在工具执行时按东八区**固定偏移**换算（distroless 镜像没有 tzdata，`LoadLocation` 会静默退回 UTC——见 `distroless-has-no-tzdata-2026-09-11`）。

### 5.2 D2 · 主页：卡片留着，上面加一句摘要

教师登录后落在班级列表。每张班级卡现在已有 `LearningSnapshot` + 近期活跃学生，**这张卡不动**。

在卡片**上方**加一段 AI 摘要：一到三句，说这个班这周值得注意的事。走 `ClassDigest`，输入是班级花名册与本周数据（和卡片用同一批数据，不另取），不额外调模型去算数。

摘要是可点的。点开就是本节的壳：左对话、右画布。画布在这个面上放的是**工具结果卡**（学生名单、本周概况），不是一个待发布的产物。

**工具：**

| 工具 | 作用 |
|---|---|
| `class_snapshot` | 取该班本周概况（沿用 roster 与 weekly 的既有接口） |
| `list_students` | 同 D1 的闭集条件 |
| `open_page` | 带老师去某一页 |

`open_page` 的目标也是闭集：`classWeekly`（本周报告）、`student`（某个学生）、`assignment`（某份作业）、`assignmentNew`（去布置作业）。跳转前在对话里先说一句要去哪，跳转本身由老师点按钮触发——AI 不在她没点的时候把页面换掉。

离开页面即丢弃该线程（§4.2）。

### 5.3 D3 · 家长报告：生成照旧，改用对话

`GenerateParentReportDialog` 与既有的生成流程不动。报告生成后，`ParentReportEditor` 右边是报告本身（今天已经是），左边加上对话。

**工具：** `revise_section(section, text)` —— 按报告已有的分节改写，写成 patch 打到编辑器里，老师可以再手改，也可以继续说。导出仍是既有的导出图片流程，不变。

`student_left` 之后重新生成本来就被禁；对话改写同样禁用，理由一致。

---

## 6. 数字与姓名不由模型写

这是本设计最容易出事的地方，也是唯一一条结构性的防线。

**做法不是加检测器，而是不让模型写。**

- 具体的数字与学生姓名由**工具结果直接渲染成画布上的卡**，不经过模型的文字。
- 模型的话只指向卡：「这三位学生本周还没写作」——「三」和名字都在卡上，由我们的查询产生。
- 系统 prompt 明确要求：不要在回复里写具体人数和学生姓名，它们会显示在卡片上。

**唯一的校验：** 回复里出现的学生姓名必须在本轮工具返回的名单里。不在就整轮失败并显示错误，不渲染。只查姓名不查数字——姓名是精确可比的闭集，数字会在文章标题、年份、第几段上误判（见 `optimizing-a-detector-made-coaching-worse-2026-09-14`：对着检测器改 prompt 会把教学改坏）。

---

## 7. E · 空状态

与 AI 无关，先做，改动小、看得见。

把教师端约 20 处一行灰字换成 `StudioEmpty`：插图 + 一句话 + 有明确下一步时给一颗按钮。插图从已有的 7 张里挑（`reading` / `writing` / `project` / `quest` / `discovery` / `ideas` / `keepsake`），**不新增素材**。

`StudioEmpty` 今天只渲染一张图和一句话（`teacher/StudioArtwork.tsx`），要加一个可选的 `action?: { label: string; onClick: () => void }`，按钮用既有的 `Button variant="primary" size="sm"`。样式在 `teacher-studio.css` 的 `.teacher-empty` 上补一条按钮间距，不新建 class 体系。

要改的位置（实现时以 grep 为准，不以本表为准）：

| 文件 | 现状 | 换成 |
|---|---|---|
| `AssignmentsPage.tsx` | `暂无班级`、`暂无作业` | 插图 + 一句 + `布置作业` |
| `AssignmentForm.tsx` | `暂无班级`、`暂无学生` | 插图 + 一句（无班级时给邀请码说明） |
| `AssignmentDetailPage.tsx` | `暂无学生`、`暂无未布置的学生` | 插图 + 一句 |
| `ParentReportsPage.tsx` | `暂无班级`、`暂无家长报告` | 插图 + 一句 + `生成报告` |
| `ClassPage.tsx` | `暂无学生`（已有 `StudioEmpty`） | 补一颗按钮：复制邀请码 |
| `StudentPage.tsx` | `暂无作业`、`暂无家长报告` | 插图 + 一句 |
| `LibraryPicker.tsx` | `暂无文章` | 保持一行字（它在一个小面板里，插图会挤） |
| `WeekSummaryCard.tsx` / `OutputRecord.tsx` | `暂无` | 保持一行字（同上） |

**判据：** 占整页或整段的空状态给插图；嵌在小面板、表格单元、下拉里的保持一行字。插图不是越多越好，挤进小格子里会比一行字更糟。

文案守 §3.6：`暂无作业` 这种标签本身是对的（状态词），加的那一句说清下一步该做什么，不写「空空如也」这类话。

---

## 8. 测试

**Go：**
- 工作台端点的鉴权与归属（非本班教师 403、学生 403、未登录 401）。
- 工具循环上限触发时返回错误而不是截断。
- `list_students` 每个闭集条件的 SQL 结果。
- 姓名校验：模型回复里出现名单外的姓名 → 整轮失败。
- 教师可见性：工具不返回印记对话记录。
- 截止时间按东八区固定偏移换算。

**前端（只写逻辑测试）：**
- `applyPatch(current, snapshot, patch)`：老师手改过的字段被保留；未改的被 patch 覆盖；空 patch 是恒等。
- 线程窗口：超过 8 轮只发最近 8 轮。
- 模式切换保留草稿。
- 选项渲染的数据整形（0 个选项、4 个选项、超过 4 个时截断到 4）。

**真浏览器：** 每部分做完跑一次性 Playwright harness，截图人眼看过再说完成。lite 的滚动在 `<main class="overflow-y-auto">` 里，`fullPage` 截图会裁——先把视口调大（见 `lite-homework-materials-2026-09-16`）。

**上线前：** 教师对话是新的 prompt，按 `prompt-output-must-be-verifiable-2026-09-03` 跑一次 `LIVE_LLM=1` 的真模型验证，至少覆盖布置作业一条完整路径。

---

## 9. 顺序

| 部分 | 内容 | 依赖 |
|---|---|---|
| **E** | 空状态换插图 | 无 |
| **D1** | 壳 + 工具循环 + 布置作业 AI 模式 | E 可并行 |
| **D2** | 主页摘要 + 对话 + 页面跳转 | D1 的壳 |
| **D3** | 家长报告对话改写 | D1 的壳 |

D1 先做，因为它的工具最多、画布最结构化，壳在这里站住了，D2 与 D3 就是复用。

---

## 10. 数据库

不新增表。对话不入库；作业草稿与家长报告用既有的表与字段。`llm_call` 按既有方式记账。

---

## 11. 留给产品负责人的问题（不阻塞实现）

1. 对话刷新即清空是刻意的省 token 设计。如果希望刷新后还在，需要加一张表——现在按不入库做。
2. 新教师默认落在 AI 模式。如果希望默认传统模式，改一个常量。
3. 主页摘要每次进页面都会调一次 `digest`。如果按班级缓存到当天，可以省掉大部分调用——现在按每次都算做，先看真实感受。

另有 A/B/C 遗留的五个问题仍未定：退回修改能否把截止时间往前设；退回是否重置未读；退回弹窗是否预填上次的话；写作室要不要放只读的老师批改面板；已有人开始后，没开始的学生能否继续更换个性化文章。
