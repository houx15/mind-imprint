# Spec D1 — 教师端读路径 + 规范投影（Teacher read-path + canonical projections）· 设计文档

日期：2026-07-24
状态：设计草案（待用户 review → 写实现计划）
性质：教师端程序（`docs/2026-07-24-assessment-teacher-end-program.md` §5 Spec D）的**第一半**。Spec D 在 brainstorm 时按**数据源接缝**拆成 D1/D2：

- **D1（本文）= 教师读路径 + 规范投影**：教师作用域读查询 + 全部学生 roster（D/A 徽章）+ 学生个人页 + 深度能力报告 + 证据地图。只依赖 Spec B（已合 main），**无新 LLM、无分析管线**。
- **D2（后续）= 班级周报分析**：使用量管线（周环比 delta、滚动窗口 watch/highlight 特征标签）、批式 Bean 班级点评、班级周报仪表盘。

**权威构件：**
- 绑定设计（唯一 UI 真相源）：`docs/design/teacher end/project/思维印记 教师端.dc.html`（四屏 + 内嵌 low/mid/high 三 CASES）。
- 终稿模型：`docs/superpowers/specs/2026-07-24-a-finalized-assessment-model-design.md`（Spec A）。
- 规范生产者：`docs/superpowers/specs/2026-07-24-b-canonical-assessment-engine-design.md`（Spec B）+ `apps/api/internal/agent/assess_report.go`（`agent.Report`）+ `apps/api/internal/studio/report_dto.go`（`studio.ReportDTO`）+ `packages/contracts/src/dualAxisReport.ts`。

---

## 1. 目标（一句话）

老师能对班里任一学生走一条 **全部学生 → 学生个人页 → 深度能力报告 → 证据地图** 的下钻路径，每个判断可点开见证据，全程**单向只读**，且全部是 Spec B 已产出的规范对象 `agent.Report` 的**教师投影**。

## 2. 架构原则（继承程序 §2）

**评估只算一次，产出一个规范对象；每个面都是它的投影。** D1 不重算、不新增评估内容——它把**已存在**的 `evaluations.scores`（= 序列化的 `agent.Report`）读出来、按教师取景渲染。D1 唯一新增的**数据加工**是一条极薄的 `event` 计数聚合（本周活跃天数 + 对话轮次），以及两个**确定性纯函数**（D/A 徽章推导）。除此之外，D1 = 新的**教师作用域读查询** + 三个 console 视图 + 一份种子迁移。

## 3. 范围

### 3.1 做（D1 交付物）

1. **教师作用域读查询**（带租户校验，跨班 → 404）：roster 投影、学生详情、单份报告读取。
2. **全部学生 roster**（升级现有 `ClassDetailView`）：每生 D 徽章 + A 徽章 + 本周活跃天 + 对话轮次 + 报告状态；行**可点入**学生个人页。
3. **学生个人页**（新页）：概况头（D/A 徽章）+ 四张使用卡 + 平台使用记录（tabs 全部/课程/对话/项目）+ 导出家长版入口（**惰性占位**，指向 E）。
4. **深度能力报告**（新页，教师视图）：`agent.Report` 的教师投影，八段（总览/官方投影/双轴读数/证据地图/交互证据/提示词透镜/作品与过程/下一步）+ 目录导航 + 面包屑。
5. **证据地图**（新组件）：ReportDTO 的**纯客户端派生**，固定 7 节点拓扑，点击看证据。
6. **种子迁移**：1 名教师 + 9 名学生的班级，其中 3 人携带由 low/mid/high 三 CASES 转写的完整项目评估。

### 3.2 不做（明确挡在 D1 外）

- ❌ 周环比 delta、滚动窗口 watch/highlight 特征标签、批式 Bean 班级点评、班级周报仪表盘 → **D2**。
- ❌ 家长柔化投影、导出/打印 → **Spec E**（D1 只放惰性入口按钮）。
- ❌ 👍/👎 校准反馈 → **绑定设计中不存在**，D1 不做（旧 PRD §6.3 有，但设计是 UI 唯一真相源）。
- ❌ 在线批改/评论/发消息/催交/出课件；自由聊天全文（仅课程内/项目内对话可生成报告）。
- ❌ 教师端写路径——任何教师动作都**不到达学生端**（三铁律①）。

## 4. 复用 vs 新建（基于后端实测）

| 能力 | 现状 | D1 动作 |
|---|---|---|
| 教师/管理员 RBAC | `teacherOrAdmin`（`api.go:121`）、`RequireRole`（`authz.go:18`）、`assertTeacherOwnsClass`（`authz.go:47`，admin-of-school 或 `role_in_class='teacher'`，否则 404） | **复用** |
| 班级 roster 读 | `getClass`（`classes.go:91`）→ `GetClassRoster`（`org.sql:41`）返回 `id,display_name,email,last_active_at,project_count,evaluation_count,card_count` | **扩展**：新增带 D/A 徽章 + 本周活跃天/轮次的 roster-report 查询 |
| 规范评估对象 | `agent.Report`（`assess_report.go:152`）↔ `studio.ReportDTO`（`report_dto.go:14`），存 `evaluations.scores`（jsonb），三作用域 project/session/thread | **复用**（读，不改形状） |
| 成长历史读 | `ListGrowthHistory`（`evaluation.sql:57`）**按 `@user_id` 自作用域** | **新建**教师作用域等价查询（按 class member） |
| console UI | `apps/web/src/console/`（`ConsoleShell`/`ClassDetailView` 等，行**不可点**，`ClassDetailView.tsx:268` 明注「暂无学生作品详情页」） | **扩展**：行可点 + 两个新视图 + 报告视图 |
| llm_call 计量 | `RecordLLMCall`（`llm_usage.sql:6`），旗舰 seam `EvalResolver`+`AssessReport` | **不用**（D1 无 LLM 调用） |
| 事件流 | `event` 表（`models.go:195`：`project_id,user_id,surface,type,payload,created_at,session_id,thread_id`），`ListEventsBy{Project,Session,Thread}` | **新建**极薄的按 user + 时间窗聚合（distinct 日 + 计数） |
| 种子 | 1 学生（Phoebe）、1 admin、0 教师、1 班、1 项目（`0002/0004/0006/0018_seed*`） | **新建** `0029_seed_teacher_class` |

## 5. 后端设计

### 5.1 教师作用域读查询（sqlc，`store/queries/`）

所有查询在 handler 层先过 `assertTeacherOwnsClass(ctx, classID)`（跨班/非教师 → 404），再执行；查询本身也带 `class_id` + 学生须 `enrollments.role_in_class='student'` AND `class_id=@class_id` 的连接，做**双重**租户约束（防越权读非本班学生）。

1. **`ListClassRosterReport(class_id)`** — 每名学生一行：`user_id, display_name, avatar_color, active_days_week, turns_week, latest_project_eval_scores (jsonb, nullable), report_status`。
   - `active_days_week` = `COUNT(DISTINCT date(event.created_at))`，窗口 = 本周（见 §5.4 时间窗口口径）。
   - `turns_week` = 本周该生「学生发出的消息」事件数（三面合计，见 §5.4）。
   - `latest_project_eval_scores` = 该生最新一份 **project 作用域** `evaluations.scores`（用于派生 D/A 徽章）；无则 NULL（→ 徽章显示 `—`，unrated）。
   - `report_status` = 派生：有 project 评估 → `generated`；有「生成中」信号 → `generating`；否则 `none`。（D1 简化：`generating` 状态若无对应事件，可先只区分 generated/none；`generating` 归 D2。**决定：D1 只 generated/none**。）

2. **`GetStudentDetailForTeacher(class_id, user_id)`** — 学生个人页数据：概况（`active_days_week, turns_week, report_count, course_count`）+ 记录列表来源。
   - `report_count` = 该生 project+chat+course 已生成评估份数。
   - `course_count` = 该生完成课程节数（沿用现有口径，若无则从 `course_progress`/评估计数近似；**决定：D1 用「已生成 course 评估份数」，真实节数留 D2**）。
   - 记录列表 = 该生的项目/课程/对话，每条 `surface, scope_id, title, occurred_at, status, has_report`。project 从 `project` 表；course/chat 从各自评估行 + session/thread 元数据。

3. **`GetStudentEvaluationForTeacher(class_id, user_id, surface, scope_id)`** — 读单份评估：返回 `evaluations.scores`（jsonb）+ `created_at`。handler 校验该 `scope_id` 确属本班该生（surface→表连接），否则 404。

> 复用而非重写：这三条是 `ListGrowthHistory`/`GetLatest*Evaluation` 的**教师作用域变体**，SQL 结构照抄、把 `WHERE user_id=@user_id` 换成 `JOIN enrollments ... WHERE class_id=@class_id AND user_id=@user_id AND role_in_class='student'`。

### 5.2 确定性徽章推导（`internal/studio` 或新 `internal/teacher`，纯函数）

从一份 `agent.Report` 派生 roster/个人页头用的两个徽章。**纯函数、无 LLM、有单测**：

- **A 徽章**（如 `4.2`）= 6 个 `autonomyAxis[].Level`（0–5）的算术平均，保留一位小数。全 NA/缺失 → `—`（unrated）。
- **D 徽章**（如 `L3–L4`）= 6 个 `depthAxis[].Level`（L1–L4，忽略 NA）的**最小档–最大档**区间；若最小=最大 → 单档 `L3`；全 NA → `—`。
- 颜色映射沿用设计 `levelColor`：L1/0–1级=红 `#C4574D`，L2/2级=橙 `#C68A3A`，L3/3级=蓝 `#3E7CA8`，L4/4–5级=绿 `#3E8A6E`，unrated=灰 `#8A92A3`（见 dc.html:1513）。

> 徽章是**摘要显示**，不是新评分轴，不落库，不进 `agent.Report`——每次读时从 scores 现算。RL-5 不变：两轴永不合成总分。

### 5.3 报告读取 + 项目上下文连接

深度报告「总览」段需 **RQ / 研究概况 / officialAnchor**，但这些**不在** `agent.Report` 里（`OfficialProjection` = `{Standard, Components, Alignment, Readiness}`，无 RQ 字段）。因此报告读端点在返回 `ReportDTO` 外，附带一小块**项目上下文**：

- `researchQuestion` — 来自 project 的 RQ：优先 `graph_node`（`type='plan'` 或 RQ 节点，`graph.sql:GetPlanNode`）的 body，回退 `project.title`。
- `profile` / `anchor` — D1 阶段：`profile` 回退用 `report.Narrative`；`anchor` 为可选静态说明（种子里带；真实项目留空则不渲染该块）。**敢于空白**：缺失即不渲染，不硬凑。

报告读端点返回 `TeacherReportDTO = { report: ReportDTO, context: { researchQuestion, projectTitle } }`。chat/course 作用域无项目上下文 → `context` 为空，报告只渲染通用内核（见 §5.5）。

### 5.4 使用量口径（D1 最小集）

绑定设计只出现 **本周活跃天数** 与 **对话轮次**，**不出现「活跃分钟数」**——因此 D1 **不做** session-time 心跳/间隔推算（程序 §8 开放问题在 D1 层面消解）：

- **活跃天数** = 本周内有任意 `event` 行的 distinct 日历日数。
- **对话轮次** = 本周内「学生发出消息」类事件计数。**口径**：以 `event.type` 中代表学生轮次的类型为准（实现期确认具体 type 枚举，如 `prompt_sent`/`chat_message`；若事件流未稳定记录，回退用 `messages` 表按 `role='user'` 计数）。三面（chat/course/project）合计。
- **时间窗口**：本周 = 传入的周锚点（默认「本周」）。D1 **不做**周环比 delta（→ D2）。窗口边界口径（周一起始）在实现期定，写进查询注释。

> 这是 D1 唯一的使用量聚合，且**极薄**（两个 `GROUP BY`）。真正重的东西——delta、滚动窗口特征标签、Bean 点评——全在 D2。

### 5.5 报告深度按模型切分（继承 Spec A/B）

- **project 作用域**报告 → 渲染**全部八段**（含官方投影 + 作品与过程 + 证据地图全节点）。`OfficialProjection`/`WorkAndProcess` 非空。
- **chat/course 作用域**报告 → 只渲染**通用内核**（总览-简/双轴读数/交互证据/提示词透镜/下一步）；无官方投影、无作品与过程；**证据地图照常渲染，但去掉 `official` 节点及其连边**（实现落定，取代本节草案期的「core 报告不渲染证据地图」默认值）。理由：其余 6 个节点对聊天/课程同样成立，地图是纯客户端派生、成本为零；缺 RQ 时该节点显示「暂无研究问题」而非消失，符合敢于空白。

### 5.6 新端点（全部 `teacherOrAdmin` + class 作用域）

| 方法 · 路径 | handler | 返回 |
|---|---|---|
| `GET /api/v1/classes/{id}/roster-report` | `getClassRosterReport` | `{ roster: RosterReportEntry[] }` |
| `GET /api/v1/classes/{id}/students/{userId}` | `getStudentDetail` | `{ student, usage, records }` |
| `GET /api/v1/classes/{id}/students/{userId}/reports/{surface}/{scopeId}` | `getStudentReport` | `TeacherReportDTO` |

租户：三者均先 `assertTeacherOwnsClass`；后两者再校验 `userId` 是本班 `role_in_class='student'` 成员、`scopeId` 属该生，否则 404（存在性隐藏）。

## 6. Web 设计（`apps/web/src/console/`）

### 6.1 视图与状态

沿用 `ConsoleShell` 无 URL router 的**顶层状态机**（`ConsoleShell.tsx:30`）。在班级详情态内扩出两级下钻态（mirror dc.html 的 `screen: roster|student|report`）：

- **RosterReportView**（升级 `ClassDetailView`）：表列 = 学生 · 本周活跃天 · 对话轮次 · **D 徽章** · **A 徽章** · 能力报告状态；行**可点** → StudentDetailView。
- **StudentDetailView**（新）：头（D/A 徽章）+ 四张使用卡 + 记录 tabs（全部/课程/对话/项目）+ 三个惰性「导出家长版」按钮（占位，指向 E）+ 记录列表（`has_report` 者可点 → TeacherReportView）。
- **TeacherReportView**（新）：面包屑（全部学生 / 学生名）+ 报告头（姓名·标题 + chips + 惰性导出 PDF）+ 右侧目录导航 + 八段内容。

### 6.2 组件复用

TeacherReportView 是一个**独立的平行渲染器**，`apps/web/src/shell/report/DualAxisReport.tsx`（学生端）**完全不动**——实现落定，取代本节草案期的「复用学生端段组件 + 最小抽取」写法。理由：两者是同一规范对象的**不同投影**（段序不同、教师版为宽版多栏 + 目录导航/面包屑/导出入口/证据地图，学生版为窄单栏），强行共享会同时扭曲两个布局；共享的是**数据**（`DualAxisReport` 契约类型）与 `badgeColor` 助手，而非 JSX。

> **已知代价（须随报告形状变更一并处理）**：D/A 卡、官方投影、提示词透镜、作品与过程四类渲染现存在**两份**（学生端 `DualAxisReport.tsx` + 教师端 `TeacherReportView.tsx`），会漂移。任何改动规范对象形状的 spec，必须同时更新两处并跑两边的渲染测试。

### 6.3 证据地图组件（EvidenceMap）

**纯客户端派生**，输入 = `TeacherReportDTO`，无任何网络请求（继承 dc.html `reportVals()` 的 map 构造，行 1614-1628）：

- 固定 7 节点：`center`（研究概况/profile）、`rq`（researchQuestion）、`official`（Readiness 摘要）、`dAxis`（D 徽章摘要）、`aAxis`（A 徽章摘要）、`prompt`（提示词透镜摘要）、`ai`（交互证据轮数）。
- 固定连边拓扑（dc.html:1626）。点击节点 → 下方详情卡显示该节点 detail。
- core-only（chat/course）报告：无 `official` 节点（或整图不渲染，见 §5.5 默认）。

### 6.4 API 客户端

`apps/web/src/api/` 新增 `teacher.ts`（或扩 `classes.ts`）：`getClassRosterReport`、`getStudentDetail`、`getStudentReport`，及对应 TS 类型（与 Go DTO 对齐）。

## 7. 契约

D1 的报告体沿用 `packages/contracts/src/dualAxisReport.ts`（`ReportDTO`，不改）。新增的**教师面 DTO**（`RosterReportEntry`、`StudentDetail`、`TeacherReportDTO` 的外层 `context`）——若跨前后端共享校验则加进 contracts；否则作为 web 侧类型 + Go DTO 双写并在测试里对齐。**决定**：D1 新壳 DTO 走 web-类型 + Go-DTO 双写（不进 Zod contracts），因为它们是投影外层、不参与评估内层深校验；报告内层仍由既有契约锁。

## 8. 种子（migration `0029_seed_teacher_class.sql`）

在 Demo School（`...001`）内：

- **1 名教师**：role `'teacher'`，`email_verified_at` + `password_hash`，enrollment `role_in_class='teacher'` 进目标班。
- **1 个班**（或复用现有 `...0002`）：`IBDP 一年级 · 研究组`（对齐 dc.html CLASS）。
- **9 名学生**：role `'student'`，各带 `school_id=...001`、`avatar_color`、`email_verified_at`、`password_hash`，enrollment `role_in_class='student'`。姓名对齐 dc.html ROSTER（林知远/沈亦然/周子墨/陈屿/吴桐/许清/何知/苏晚/罗一）。
- **3 份完整项目评估**：林知远=high、沈亦然=mid、周子墨=low。每份 = 一个 `project` 行 + 一个 `evaluations` 行（`project_id` 设、`scores` = **由对应 CASE 转写的完整 `agent.Report` JSON**）。
  - **转写映射**（CASE → `agent.Report`）：`dAxis[{dimension,level,evidence}]`→`depthAxis[{Code,Name,Level,Evidence}]`；`aAxis[{dimension,score,evidence}]`→`autonomyAxis[{Code,Name,Level,Opportunity,Evidence}]`（`score`「N级」→`Level` 整数，`Opportunity` 按证据判 given_taken/given_not_taken/not_supplied）；`prompt[]`→`promptLens.Lenses[]`；`official[]`+`officialAlignment[]`→`officialProjection{Standard:"ap-research",Components,Alignment,Readiness{Score=scoreValue,Note}}`；`workSample[]`+`process[]`→`workAndProcess`；`interaction[]`→`interactionEvidence[]`；`nextSteps[]`→`guidance.NextSteps[]`；`scoreSummary`→`narrative`。种子 JSON 须**满覆盖**（6/6/6），因种子绕过 Go normalizer。
- **其余 6 名学生**：轻量或无评估，制造空状态样本——罗一（`unrated`，0 活动）、陈屿（低活跃）等，对齐 dc.html ROSTER 的 usage/watch 语义（但 D1 不渲染 watch 标签，只体现 unrated/数据不足）。

> 校验：种子跑完后 `getClassRosterReport` 对该班返回 9 行，3 行有 D/A 徽章、6 行 unrated 或部分；`getStudentReport` 对三主角返回满覆盖报告。

## 9. 空状态（继承旧 PRD §14.2 状态矩阵中 D1 相关项）

- **未评估学生**（罗一/新转入）：roster D/A 徽章 `—`；个人页头徽章 `—`，使用卡显示事实（0/低），报告区留白「暂无可计入的证据」，无「查看报告」入口。
- **core-only 报告**（chat/course）：不渲染官方投影/作品与过程/证据地图；总览退化为 narrative。
- **缺 RQ/anchor**：总览对应块不渲染（敢于空白）。

## 10. 贯穿不变式（每条都受程序 §4 约束）

- **单向只读**：D1 无任何写学生端的路径；教师所有端点是 GET。
- **说人话**：教师视图文案不出现内部术语（门禁/钢人/自发/L1-L4 裸标签须伴行为描述）；以旧 PRD 附录 A 对照表为 lint。**注**：终稿模型下教师端**保留** D 轴/A 轴与 L1-L4/0-5 级（绑定设计如此——设计是真相源），但每个等级须伴证据原话。
- **每个判断带证据**：roster/个人页/报告的每个等级点开可见 `evidence`；无证据不上屏。
- **租户隔离**：跨班读 → 404；学生打教师端点 → 403。
- **RL-5**：两轴永不合成总分；唯一数字聚合 = 项目面 `Readiness.Score`/100（就绪度折算，绑定设计明示「不与 D/A 合成」）。
- **评估隔离/旗舰**：D1 不触发评估（纯读），不涉及降级问题。
- 客户端绝不直连模型；D1 无 LLM 调用。

## 11. 测试基线

- **Go（全包，`-p 1`，绝不 `-run` 子集）**：
  - 租户矩阵：本班教师读成员 ✅；他班教师读 → 404；学生打教师端点 → 403；admin-of-school 可读。
  - 徽章推导纯函数单测（含全 NA、单档、跨档、越界钳制）。
  - roster-report / student-detail / report-read 查询含数据的读测（testcontainers）。
  - CASE→Report 种子 JSON 满覆盖校验（读回经既有报告读路径不 panic）。
- **Web（`npm test` + `tsc --noEmit`）**：RosterReportView/StudentDetailView/TeacherReportView 渲染；EvidenceMap 派生（含 core-only 无 official 节点）；学生端 `DualAxisReport` 回归（抽取共享段后渲染不变）。
- **Contracts**（若新增 Zod）：`npm test` + `tsc`。

## 12. 留待 D2 / E 的开放项

- 周环比 delta、watch/highlight 特征标签滚动窗口阈值、批式 Bean 班级点评、班级周报仪表盘 → **D2**。
- 家长柔化投影、导出/打印 → **E**。
- 真实课程「完成节数」口径、`generating` 报告状态、活跃分钟数（若未来需要）→ D2。
- 👍/👎 校准（绑定设计外）——若未来产品决定加回，另立小 spec。
