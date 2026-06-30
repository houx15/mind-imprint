# 遗留项追踪 · Carry-forward Tracker（保证端到端不丢）

> 目的：把每个 slice 里**明确推迟**的事都登记到一个未来 slice，确保最终能跑出一个端到端可用的平台。
> 维护：每个 slice 完成时更新本表 + `slice-progress` 记忆。版本 2026-06-21（S6 后）。
> **状态：S4+S5+S6 全部完成 → 平台端到端可跑通（merged `9d3f406`）。S3c 全部完成（10 张 stub 卡全部 un-stub，merged `4403b14`，0 stub）。剩余仅评估增强 + review minor + 视觉增强（拖拽画布/对话），均不在关键路径。**

## 路线（剩余 slice）

| Slice | 块 | 一句话 | 端到端价值 |
|---|---|---|---|
| **S4** ✅ | B5 | 过程树（确定性实时派生，merged `f427506`） | PRD §2 #3：过程长成可读的树 |
| **S5** ✅ | B6 | 评估那一刀 + 「你的思维印记」（merged `6732ab1`） | 护城河（PRD §11） |
| **S6** ✅ | B7 | 应用外壳（导航/主目录/记录页/设置页/mock 认证/key-gate；评估扩到 9 维 FULL_RUBRIC；merged `9d3f406`）| 把全部串成可用 app — **端到端达成** |
| **3c** ✅ | B1 扩展 | 富交互卡（10 张 stub 全部 un-stub，merged `4403b14`；新增 3 原语 spectrum/criteria_check/show_if，其余组合现有原语）| 0 stub，全库可交互 |

> ✅ **端到端已跑通**：**登录 → key-gate 填 key 测连接 → 新建任务粘链接 → 陪练对话 → AI 调卡 → 填卡 → 过程树实时长 → 生成思维印记（9 维评估）→ 记录页**。3c 是卡的广度扩展，不在端到端关键路径上（stub 卡当前可提议、可打开占位、可提交，优雅降级）。

## 遗留项 → 目标 slice（逐条登记）

| 遗留项 | 来自 | → 目标 slice | 端到端必需？ |
|---|---|---|---|
| **设置页**：真实 LLM 配置 UI（替代 dev `SettingsPanel`）| S2/S3b dev 入口 | **S6** | 是（用户得能填 key） |
| **非关闭式 key-gate 弹窗**（缺 `mk.llmConfig` 时强制先填+测连接）| S2.5 头脑风暴 | **S6** | 是（首跑体验） |
| **config 加固**：`mk.llmConfig` 改 zod 解析（去掉 `as`）；store mutators 返回 post-`parse` 值 | S2.5 | **S6**（设置页拥有 config）| 否（健壮性） |
| **主目录/首页**：任务列表 + 新建任务 + 粘文章链接 | 设计 HTML `showDirectory` | **S6** | 是（任务入口） |
| **目录↔工作区路由**（面包屑「返回所有任务」）| S3b | **S6** | 是 |
| **记录页**：`recCards`（卡使用计数，聚合 card_instance）/ `recLearning`（成长叙述，来自评估）/ `recAbility`（能力图——本期出轻量静态版，全技能树不做）| 设计 HTML `isRecords` | **S6** | 是（PRD §2 展示面）|
| **mock 认证**：login/register/bind 屏（点击即进，无后端）| 设计 HTML `authScreen` | **S6** | 否（demo 可跳过/mock）|
| **导航 a11y**（`role="tab"`/`aria-selected`）| S2.5/S6 | **S6** | 否 |
| **评估触发 + showEval/evalLoading「你的思维印记」UI** | PRD §11/§14 步骤7 | **S5** | 是（护城河）|
| **语义树归并**：`sub_question` 分枝 / `key_knowledge` / `attempt` 节点 + 归并打型（eval LLM 顺手做）；并把 `nodeView` 缩进从二元（`parent_id!==null`→1）改为按 node 列表**走父链算真实 depth**（depth×18px；HTML demo 卡节点在 depth 2=36px）| S4 → S5 | **follow-on（评估增强，可与 S5 `runEvaluation` 合并为一次结构化调用，或单独一刀）** | 否（S4 扁平实时树已满足"可读的树"）|
| **few-shot 锚点样本扩展**：Marcus（被动 L1-2）/ Ethan（代写红线）/ Eliza（跨模块）——本期只放 1 份 Phoebe | S5 | **follow-on（评估精度/覆盖）** | 否（demo 用 Phoebe few-shot 够）|
| **评估 benchmark / 微调**：人工标注 benchmark + 累积数据微调小模型（doc 的 验证/规模化 阶段）| S5 | **后续（非 demo）** | 否 |
| **评估触发接进任务完成流**（当前手动按钮；可在任务"告一段落"时提示/异步增量跑）| S5 | **S6**（任务流/外壳） | 否 |
| **RUBRIC_TAGS 对齐**：`rubric.ts` 旧标签（D1_来源意识/D2_交叉验证/D5_论证结构/D7_对立观点处理）与评估 D2–D6 体系不一致；2 张 demo 卡 JSON 仍用旧串——删/改/文档化 | S5 review | **S6** | 否 |
| **评估 error UX**：`phase==="error"` 当前对用户无任何提示；**re-run 覆盖**无确认 | S5 review | **S6** | 否 |
| **setState-swap 模式核查**：`createEvaluator` 已修（state 仅在 changed 时换 ref）；核查 `createConversation`/`createStore` 是否同样无条件换 ref | S5 review | **S6 / cleanup** | 否 |
| **评估增强树是否落库** vs 重派生 | S4 | **S5** 定 | 否 |
| ~~**10 张 stub 卡的真实交互**~~ ✅ 完成（S3c）：量表光谱→`spectrum`，分类标注→`criteria_check`+组合，步骤引导→`show_if`，画布导图/角色模拟→组合现有原语 | S3a | **3c ✅** | — |
| ~~**新原语**：`show_if` / spectrum~~ ✅ 完成（S3c 加了 spectrum / criteria_check / show_if 三个原语）| S3a/3c | **3c ✅** | — |
| **视觉增强（S3c 显式推迟）**：画布导图卡的自由拖拽节点画布、角色模拟卡的多轮实时对话、`spectrum` 指针拖拽、`node_map`/`role_play` 原语（如要做这些视觉层）| S3c | **后续（非阻塞）** | 否（组合版已可用）|
| **卡内容打磨**：emotional-alignment 运行期安全元数据（human_in_loop/privacy）、rabbit-hole 锚点 UX、learning-report 轨迹预填、checkpoint spot-error 字段 | S3a/3b review | **3c** | 否 |
| **一轮多卡**：re-feed 后续 `summon_card` 当前被丢弃（MVP 一轮一卡）| S3b review | **3c** | 否（MVP 足够）|
| **关卡=跳过不可逆**：scrim 误点即记 skipped；加「仅关闭不决定」语义 | S3b review | **3c** | 否 |
| **ChatLog index keys**；S1 raw-hex token 合并 / ITEM_COMPONENTS 去重 / ActiveSheet steps[0] 假设 | S3b/S1 review | **3c / cleanup** | 否（纯整洁）|

## S6 已完成（原表中目标=S6 的行，全部勾掉）

- ✅ 设置页真实 LLM 配置 UI（`SettingsView` 的 模型/API 段，复用 `LlmConfigForm`）。
- ✅ 非关闭式 key-gate 弹窗（`KeyGateModal`，测连接通过前阻断 app）。
- ✅ config 加固（`config.ts` zod `StoredConfig.safeParse` 回退 env + `verified` 标志 + `isVerified`/`markVerified`）。
- ✅ 主目录/首页（`DirectoryView`：问候 + 新建任务 URL→seed + 任务网格）。
- ✅ 目录↔工作区路由（`AppShell` taskView + `WorkspaceContainer` 面包屑返回）。
- ✅ 记录页（`RecordsView` 三页签：活跃日历 / 工具卡用量 / 9 维能力雷达）。
- ✅ mock 认证（`AuthScreen` login/register/bind 点击即进）。
- ✅ 导航 a11y（`LeftRail` role="tab"/aria-selected——tablist 包裹仍可补，见下）。
- ✅ 评估 error UX（`评估失败`+`重试`）+ re-run 覆盖确认（`重新评估`+`window.confirm`）。
- ✅ `createConversation` setState-swap 修复（assignment 移入 changed-guard）。
- ✅ **RUBRIC_TAGS 对齐——已彻底解决**：删除旧 `RUBRIC_TAGS`，评估统一为 9 维 `FULL_RUBRIC`（D1–D9）；few-shot 打满 9 维。

## S6 review 留下的 minor（非阻塞，择期 polish）

- `parseStyle` CSS 串→React style 解析在 DirectoryView/RecordsView 各写一份——抽一个共享 util。
- `radarGeometry.RadarLabel.anchor` 类型应收窄为 `"start"|"middle"|"end"`，去掉 RecordsView 里的 `as` cast。
- `KeyGateModal` 未做键盘 focus-trap / Escape 拦截（真正拦截靠 AppShell 不渲染 app；a11y polish）。
- `SettingsView` 模型/API 外层卡与 `LlmConfigForm` 自带卡双层边框——去其一。
- `LlmConfigForm` 默认 chat 走组件内 `import("../../llm")` 动态导入（每次 render 重建）——提到模块级缓存。
- `LeftRail` role="tab" 缺 role="tablist" 父容器；`AuthScreen` 输入框缺 id/htmlFor/aria-label。
- `WorkspaceView` 每次 render 调两次 `getLatestEvaluation`——DRY 成一个局部。

## 明确不做（非遗漏，是范围决定）

- 真后端 / 服务端 key / 计量（前端直连 BYO-key，§0.1 覆盖 PRD §16）。
- 真实登录鉴权（只 mock）。
- 全技能树（技能树本期不做；记录页 recAbility 至多轻量静态）。
- LLM 流式输出（非流式，陪练回复短）。
- 教师端 / 文档编辑器 / RAG 路由（PRD §4「明确不做」）。

## 结论

**S4 + S5 + S6 全部完成，平台已端到端可用**（merged `9d3f406`）；**S3c 也已完成**（10 张 stub 卡全部 un-stub，merged `4403b14`，`library.test` 守卫 0 stub）。全部路线 slice（S1–S6 + 3a/3b/3c）皆已完成并入 main。剩余仅非阻塞打磨：**视觉增强**（拖拽画布 / 多轮对话 / spectrum 拖拽）、**评估增强**（语义树归并 + Marcus/Ethan/Eliza few-shot 扩展 + nodeView 深度走父链）、**S6 review minor**、**一轮多卡 / 关卡可逆 / ChatLog keys** 等整洁项。无孤儿项，无阻塞。

---

## 卡片修复 + 富教学体验批（merged `4b54ae0`，2026-06-24，大重构前）

设计 `docs/superpowers/specs/2026-06-24-card-bugfixes-and-rich-teaching-design.md`，计划 `docs/superpowers/plans/2026-06-24-card-bugfixes-and-rich-teaching.md`。16 个 TDD 任务 + 终审（4 大不变量逐条核过，Ready to merge=Yes）全绿；375 web + 117 contracts 测试通过。

- **#1** `openCard` 现设 `pendingCardId`（切会话后卡仍可重开）。
- **#2** 关闭 ≠ 跳过：scrim/X/取消 → `conversation.closeCard`（不记跳过），仅显式「跳过这张卡」按钮 → `skipCard`。
- **#4** 系统 prompt + 工具描述改为「贴合就递」，保留克制护栏（一次一张 / 打开由学生确认）。
- **#5** 一回合可同时给解释文 + 卡：`createConversation` 存 `content=result.text`；`viewModel` 出 ai_text 气泡 + 提议（去重守卫防旧数据双显）；`messageMapping` 未解决分支回落 nudge_text。
- **#3** SIFT×CRAAP 与 内在小人(emotional-alignment) 两张富卡：共享内联 SVG 资产（`cards/teaching/assets/`）→ `TeachingModal`（A1 居中，`teachingRegistry`/`pickTeaching`）+ 两个教学 module → `CardSheetHost` 的「给我讲讲这个」入口（开教学、记一次 `note_open`、隐藏方法面板）→ 两个自定义填写渲染器（`SiftCraapRenderer`/`InnerPartsRenderer`，经 `pickCardBody`，护栏测试钉死「只写本卡 schema key / 选项串逐字」）。

### 本批 ship-as-is 遗留（终审判为非阻塞，可后续清理）

- `prompt.ts` 首段 bullet 仍留「（见下，按需，不是默认动作）」，与新「贴合就递」措辞略有张力。
- `viewModel` 去重(content==nudge)/纯空白 content 边界未单测（逻辑在）。
- `TeachingModal`「首章禁用上一步」无显式测试（`disabled={isFirst}` 在）。
- 内在小人角色卡用 `<div role="button">`（a11y 近似，已带 tabIndex/onKeyDown/aria）；教学 energy 章默认 level 2（首屏显示低电量条，纯教学不入信封）。
- `SiftCraapRenderer` ~L260 有一处过时注释（称测试不模拟展开，实际已模拟）。
- 两个填写渲染器各有 `as any` 取 field 定义；`InnerPartsRenderer` 内 `OnDemandSection` 重复了 `CardRenderer.OnDemandStep`（可抽共享）；energy 区有一个装饰性 `Battery` 与可点控件并存。

**结论：本批全部并入 main 并推送 origin（`99a307d..4b54ae0`），分支已删。无 Critical/Important 遗留。**

---

## P2 · 认证 + 最小组织（后端平台重构，merged 后更新，2026-06-26）

设计 `docs/superpowers/specs/2026-06-25-p2-auth-minimal-org-design.md`，计划 `docs/superpowers/plans/2026-06-25-p2-auth-minimal-org.md`。12 个 TDD 任务（subagent-driven，逐任务双审 + opus 终审「Ready to merge=Yes」）。后端真·Cookie 会话认证取代 P1 `ActAsSeed`：注册（班级邀请码原子建号+入班，组织不变式）/登录/登出/`GET /auth/me` + 建好但**休眠**的邮箱验证流（P2 注册即自动验证）。前端 `AuthScreen` 接真 API + `AppShell` getMe 启动门。

- 新表 `sessions` / `email_verification_tokens`（仅存 token 的 SHA-256 哈希）；`internal/auth`（argon2id `m=65536,t=1,p=4` + 32B `crypto/rand` 不透明 token）；种子 Phoebe 真密码（`phoebe@demo.mindimprint.local` / `phoebe-dev-pass`）。
- 路由公开/受保护切分：公开 = signup/signin/signout/verify-email；受保护（`RequireUser`→401）= `/auth/me` + 全部 `/tasks`；`SessionAuth` 包裹整 mux 且**自身从不拒绝**。Cookie `mk_session`：HttpOnly+SameSite=Lax+Secure(配置门控，dev 关)。
- 全门绿：Go `go vet` + `go test -p 1 ./...`（全包）；web typecheck + 295 测试 + build。

### P2 ship-as-is 遗留（终审判为非阻塞 Minor / 明确推迟）

| 遗留项 | → 目标 | 端到端必需？ |
|---|---|---|
| **Mailer + 真发邮件**：验证流已建好但休眠（注册即自动验证）；接 provider（China-first：阿里云/腾讯，或 Resend）时注册改「建 token + 发 raw token，`email_verified_at` 留 NULL」，端点零改 | 硬化轮 / 接邮件时 | 否（demo 自动验证） |
| **CSRF 双提交 token** + **限流**（signup/signin/verify 按 IP+邮箱）| 安全硬化轮 | 否（Lax cookie 已挡跨站表单 CSRF） |
| **argon2 `version` 解析后未断言**（`password.go` VerifyPassword 读 `v=%d` 但不校验；单版本部署，失配 fail-closed）| 安全硬化轮（与 CSRF/限流同批）| 否 |
| **CORS `AllowedMethods` 缺 `PATCH`**（middleware.go `{GET,POST,PUT,OPTIONS}`，但 `/tasks/{id}/cards/{cid}` 是 PATCH）——**P1 既有**，P2 首次让真浏览器跨域走到它；本地 dev 同源（vite 代理）故 demo 不受影响；openCard 的 PATCH-active 本就 best-effort fire-and-forget | 硬化/浏览器联调轮 | 否（demo 同源） |
| **SettingsView 头像块**仍硬编码 `"P"`/`#E8A33D`（`user.avatar_color` 已入 DTO 但未接到色块）| 打磨 | 否 |
| **AuthScreen 绑定步删了静态班级预览卡**（假数据；真预览需 class-lookup 端点，不在 P2）；错码经 signup `invalid_join_code` 兜底 | P3（组织端）或加 class-lookup 端点 | 否 |
| **签注 tx.Rollback 用 `r.Context()`**（处理器返回后已取消；pgx/PG 靠连接释放回滚，仅装饰性）| cleanup | 否 |
| **teacher/admin 自助 + 名单管理 + 学校维度聚合** | **P3** | — |
| **异步评估（river）** | **P4** | — |

**结论：P2 全 12 任务 + 终审完成，无 Critical/Important，可并入 main。**

---

## P3.1 · Org backend + RBAC（后端组织端，2026-06-26）

设计 `docs/superpowers/specs/`（P3.1 专项设计），计划 `docs/superpowers/plans/`。12 个 TDD 任务（subagent-driven，逐任务双审）。实现了教师邀请流（admin mint invite → teacher 用 invite code 注册 → teacher 建班 + 邀请码 → student 用 join code 注册入班）、班级名单（roster）读取、CSV 教师名单导入端点、管理员概览聚合（`GET /admin/overview`）、完整的 RBAC 分层（`assertAdminOfSchool` / `assertTeacherOwnsClass` / `RequireRole`）。端到端 org provisioning 测试（`TestE2EOrgProvisioning`）全链路通过。

### P3.1 ship-as-is 遗留（终审判为非阻塞 Minor / 明确推迟）

| 遗留项 | → 目标 | 端到端必需？ |
|---|---|---|
| **班级软归档**（class soft-archive / deactivate）：当前无停用班级端点；归档后学生不可新增入班、教师不可再用 join code | P3.2 或独立清理轮 | 否（demo 不需停用）|
| **每生工作详情可见性**（per-student work-detail）：过程树 / 对话记录 / 评估叙述对教师的可见性——等待 eval 数据模型头脑风暴完成后再建接口 | 评估增强轮 / P4 后 | 否（teacher 当前只看 roster 聚合行）|
| **CSV 解析在前端**（P3.2）：`POST /api/v1/admin/schools/{id}/import-teachers` 当前接已解析的 JSON rows；浏览器端 CSV→JSON 解析、错误预览 UI、逐行导入结果展示 = P3.2 前端任务 | P3.2 前端 | 否（端点已就绪）|
| **教师邀请 email 投递为建议性**（advisory）：invite code 当前写库但不发邮件（Mailer 桩同 P2）；admin 需手动取 code 转告教师 | 接邮件轮（同 P2 Mailer）| 否（demo 手动传递）|
| **教师邀请跨请求去重未实现**：重复导入同一教师邮箱会 mint 新 invite（invites 表不唯一约束）；建班操作是幂等的，invites 不是 | 硬化轮 | 否（demo 单次导入）|
| **限流 + CSRF 双提交 token**：从 P2 延续，signup/signin/invite 端点仍无限流；CSRF = Lax cookie 现有防护 | 安全硬化轮 | 否 |
| **教师端前端控制台**（建班 + 名单管理 + 学生工作详情入口）| **P3.2** | — |
| **管理员端前端控制台**（学校概览 + 批量导入 + 邀请管理）| **P3.3** | — |
| **异步评估（river）** | **P4** | — |

**结论：P3.1 全 12 任务 + 端到端 org provisioning 测试完成，go vet + go test -p 1 ./... 全包通过，无 Critical/Important 遗留。**

---

## P3.2 · 教师/管理员控制台前端（AppShell 角色路由 + StudentApp 提取，2026-06-26）

`AppShell` 拆出 `StudentApp`（行为保全提取）并按角色路由：`teacher`/`admin` → `ConsoleShell`，其余 → `StudentApp`。327 个 web 测试全绿，typecheck 干净。

### P3.2 ship-as-is 遗留（终审判为非阻塞 Minor / 明确推迟）

| 遗留项 | → 目标 | 端到端必需？ |
|---|---|---|
| **管理员建班 + 教师分配**：前端建班表单向 `POST /classes` 携带 `teacher_user_id`，需后端先提供 teacher-picker 端点（按 school 列教师列表）；admin 界面无"新建班级"按钮（`isTeacher=false` 已隐藏）| P3.3 | 否（admin 可通过教师控制台间接建班）|
| **班级列表卡片学生人数**：`ClassSummary` 当前无学生数字段；需后端 `GET /classes` 在 `COUNT(enrollments)` 聚合后返回 `student_count`；前端列表卡片显示 N 名学生 | P3.3（后端 COUNT 配套）| 否（邀请码可见、班级名可见，demo 够用）|
| **管理员专属屏**（教师邀请管理 / CSV 名单导入 / 学校概览）：`ConsoleShell` 当前 admin 与 teacher 共用同一视图，admin 无额外管理入口 | P3.3 | 否（P3.1 后端端点已就绪，等前端接入）|
| **每生工作详情可见性**：过程树 / 对话记录 / 评估叙述对教师的可见性——等待 eval 数据模型头脑风暴完成后再建接口（P3.1 同款遗留，从 P3.1 延续）| 评估增强轮 / P4 后 | 否（teacher 当前只看 roster 聚合行）|

**结论：P3.2 AppShell 角色路由 + StudentApp 提取完成，全 327 测试 + typecheck 通过，无 Critical/Important 遗留。**

---

## P3.3 · 管理员控制台前端（教师管理 + ClassDetailView 教师区块，2026-06-27）

`ConsoleShell` admin 专属屏（教师邀请管理 / CSV 名单导入 / 学校概览 / 建班选教师）+ `ClassDetailView` admin 教师区块（列教师 + 分配 + 移除）。12 个 TDD 任务（subagent-driven，逐任务双审）。班级列表补 `student_count` 字段（后端 COUNT 聚合 + 前端列表卡显示 N 名学生）。

### P3.3 ship-as-is 遗留（终审判为非阻塞 Minor / 明确推迟）

| 遗留项 | → 目标 | 端到端必需？ |
|---|---|---|
| **教师自助协教师管理**（co-teacher self-service）：教师可申请添加/移除同班协教师；当前分配/移除教师仅 admin 操作，教师角色无此入口 | 后续（权限扩展轮）| 否（admin 操作够 demo）|
| **每生工作详情可见性**（per-student work-detail）：过程树 / 对话记录 / 评估叙述对教师/admin 的可见性——等待 eval 数据模型头脑风暴完成后再建接口；当前只看 roster 聚合行 | 评估增强轮 / P4 后 | 否 |
| **真实邮件投递**（invite + 注册验证）：邮件验证流已建好但休眠（P2 起延续）；invite code 写库但不发邮件；接 provider（阿里云/腾讯/Resend）时可激活 | 接邮件轮 | 否（demo 手动传递）|
| **限流 + CSRF 双提交 token**：signup/signin/invite 端点仍无限流；CSRF = Lax cookie 现有防护（P2 起延续）| 安全硬化轮 | 否 |
| **CSV 转义引号（`""`）未支持**：前端 CSV 解析器当前不处理 RFC 4180 转义引号（`""` → `"`），含引号字段的名单导入可能截断；简单 split 实现已足够 demo | 硬化轮 | 否（demo 名单无含引号字段）|
| **班级软归档**（class soft-archive / deactivate）：无停用班级端点；归档后学生不可新增入班、join code 失效（P3.1 起延续）| 后续清理轮 | 否 |
| **单教师强制校验**（single-teacher enforcement）：当前可对同一班级多次分配同一教师 ID（后端 `class_assignments` 无 unique 约束在教师维度）；幂等性由调用方自保 | 硬化轮 | 否（demo 场景单次分配）|
| **控制台 a11y**（console a11y）：`ClassDetailView` 教师区块选择器缺 `<label>`；`ConsoleShell` 导航卡片用 `<div>` 点击、缺 `tablist`/键盘导航——与 `LeftRail` a11y 遗留共享优先级 | a11y polish 轮 | 否 |

**结论：P3.3 全 12 任务完成，web typecheck + 全套测试通过，无 Critical/Important 遗留。**

---

## 全栈 E2E 实烟测试套件（test/fullstack-e2e-live-smoke，2026-06-27）

4 个任务（Task 1–4）构建了完整的浏览器驱动烟测套件，覆盖真实栈（web → Go API → 一次性 Postgres → 真实 DeepSeek）。

- **位置**：`apps/web/e2e/`（harness: `run-stack.sh`，helpers: `helpers.ts`，4 个 spec，`RUNBOOK.md`）
- **运行方式**：本地开发者手动运行，**不是 CI 门禁**（需真实 `DEEPSEEK_API_KEY`）。
- **确定性 spec 已在 keyless 环境验证通过**：`auth.spec.ts`（3 个测试）、`registration.spec.ts`（1 个测试）、`smoke.spec.ts`（1 个测试）——5/5 全绿。
- **golden-path spec**（`golden-path.spec.ts`）：跨角色完整生命周期（admin invite → teacher 注册 → 建班 → student 入班 → Phoebe 任务 → 卡片召唤 → SIFT 信封 → 过程树 → refeed → 评估「你的思维印记」→ 教师名单信号 → 管理员概览）。**需真实 API key 才能运行 live-model 步骤**，由开发者按 `RUNBOOK.md` 执行。

### 遗留 / 后续

| 遗留项 | → 目标 | 必需？ |
|---|---|---|
| **确定性 CI 门禁**：如需将 E2E 接入 CI，需实现 `STUB_LLM` 脚本化 provider（stub 召卡 + stub 评估），使 golden-path 无真实 key 可运行 | 后续（CI 接入轮）| 否（当前本地手动） |
| **选择器稳定性**：auth 输入框 / 卡片表单字段缺 `data-testid`；当前用位置/类型定位（`input:not([type="password"]) nth(0)` 等），改版 UI 后可能脆性失效 | 后续（testid 补充轮）| 否（当前定位稳定） |
| **golden-path 卡片填写**：`summonCardWithRetry` 对 `tool_choice=auto` 有一次重试；`retries:1` playwright 配置已吸收一次随机失败；极偶发"模型拒绝召卡"属 live-model 方差，不是管道故障，重跑即可 | 无（RUNBOOK 已记录）| — |

**结论：全栈 E2E live-smoke 套件已建立并验证（4 spec 文件，5 确定性测试全绿，golden-path type-clean 并已被 Playwright 发现），由开发者按 RUNBOOK 持有真实 key 后执行 live 跑。**

---

## P4 异步评估 + 里程碑自动触发（2026-06-29）

P4 后端 backbone（async eval / river / 规则信号 / 10 维 v2 量规）已并入 main `0fb57c2`；其 follow-up **里程碑自动触发**已并入 main `1e2842e`（6 任务，subagent-driven，每任务复核 + opus 全分支终审「Ready WITH FIXES」+ 1 修复；全门禁绿）。Spec/plan 见 `docs/superpowers/{specs,plans}/2026-06-29-milestone-auto-trigger-*.md`。**纯后端**：服务端在里程碑（首张完成卡 **或** 6 个实质学生回合）自动入队评估，去抖（在途/10 分钟内已完成则跳过）+ 上限 3 次自动评估/任务；分区唯一索引 `evaluations_one_inflight_per_task` 保证「每任务至多 1 个在途评估」（迁移 0008 自愈，建索引前降级历史重复在途行）；worker 成功后回填 `task.status='evaluated'`（补回 P4 丢掉的标记）；手动 `POST /evaluate` 改为幂等（在途时返回现有行 202）。

### 遗留 / 后续（里程碑，均非 bug）

| 遗留项 | → 目标 | 必需？ |
|---|---|---|
| **turn 路径触发用 `r.Context()`**：最后一个 SSE 事件后客户端断连会取消 context → 该次里程碑触发被静默丢弃（best-effort 容忍）；`context.WithoutCancel` 可加固 | 加固轮 | 否 |
| **失败的里程碑入队行计入上限 3**：infra 失败的行仍计入 cap（防失控，合理）；值得加注释说明「cap 计的是尝试不是成功」 | polish 轮 | 否 |
| **`TestCountSignals` 未钉边界**：有 2 字符（排除）+ 25 字符（计入），无 19/20 字符行 → `>=20`→`>20` 回归测不出 | polish 轮 | 否 |
| **worker 在 `Finish` 后崩溃可重复消费旗舰**：唯一索引不覆盖 `done` 行；重试会重跑 `runEval`（P4 既有路径，非本轮引入） | P4 加固轮 | 否 |
| **P4 backbone fast-follows**（river-hop 集成测试、事务化入队 + stale-row reaper、死代码清理 `Deps.EvalResolver`/`EvalStore`/`CreateEvaluation`、test/content polish）| P4 加固轮 | 否 |

**结论：里程碑自动触发全 6 任务完成 + 1 终审修复（迁移自愈），全门禁绿（build/vet + `go test -p 1 ./...` 全 9 包），无 Critical/Important 遗留。**

---

## P4 follow-up：前端「你的思维印记」reveal UI（2026-06-30）

已并入 main `5f910d0`（6 任务 → 8 commits，subagent-driven，每任务复核 + opus 全分支终审「Ready WITH FIXES」+ 1 修复；全门禁绿 `pnpm -r typecheck && pnpm -r test` = 469 测试）。Spec/plan 见 `docs/superpowers/{specs,plans}/2026-06-29-mind-imprint-reveal-ui-*.md`。**纯前端（+契约同步）**：把异步 v2 评估呈现为**层级钻取**——2 张脸（🚀 生成式驾驭 / 🛡️ 批判式防护）→ 4 类（意图与编排 / 推理与论证 / 信息素养 / AI元认知与边界）→ 10 维（D1-D10）；**铁律#2**：脸/类只显「事实覆盖 chip」（N/M 维已评，无聚合等级/分数），等级词（L1-L4）只在维度叶子出现；N/A 中性（「本次未涉及」灰显无条，计为未涉及而非失败）；全 N/A → 「进行中」。视觉遵 `.dc.html`，结构遵 eval-model spec §7。`createEvaluator` 改为轮询（POST→queued→done|failed|timeout，phase 名仍 `"running"`）；新 `MindImprintIndicator`（offer-don't-push，无 badge/计数/自动弹）；`WorkspaceContainer` 加 30s 后台轮询（仅在更新时 putEvaluation）。终审修复 `5f910d0`：弹窗 guard 收紧为 `phase==="idle"||phase==="done"`（防 re-eval 以 error 结束时闪回旧评估）。

### 遗留 / 后续（reveal，均非 bug）

| 遗留项 | → 目标 | 必需？ |
|---|---|---|
| **后台轮询不在 reveal 打开时暂停**（spec §5 要求暂停）：`showEvalModal` 是 WorkspaceView 局部态，上提到 container = 结构改动；影响小（新评估可能在阅读时换掉弹窗内容） | polish 轮 | 否 |
| **`putEvaluation` 无条件 APPEND**：手动 evaluator 路径缺后台轮询那道 `created_at>known` 门 → 幂等 re-eval 可能在 `listEvaluations` 重复一行；修法 = 按 id upsert，但现有 fixture 断言空-id 评估 `toHaveLength(2)` → 需专门 fixture 轮 | 独立 fast-follow | 否 |
| **`evaluate.ts` CAST 而非 Zod-parse 响应**：`status.default("done")` 只在 `StoreState.parse` 生效；后端若漏 `status`，轮询静默超时（当前 DTO 无此问题，仅意识层面） | 意识 | 否 |
| **`@keyframes mkPop` 未定义**：indicator + modal 引用但 index.css 无定义 → 装饰性 no-op（chip/modal 无弹入动画即出现，EvalModal 既有） | polish 轮 | 否 |
| **rubric v1 锚点 D1-D9 与 v2 对账**：v2 量规取代 v1，旧锚点未逐条 reconcile | 内容轮 | 否 |

**结论：reveal UI 全 6 任务完成 + 2 复核/终审修复，全门禁绿 469 测试，无 Critical/Important 遗留。异步 v2 评估现已端到端打通（里程碑自动触发 → worker 10 维评分 → SPA 轮询 + 层级揭示 + 安静邀请）。下一步 = billing/entitlement 头脑风暴（填 `HasEntitlement`）。**
