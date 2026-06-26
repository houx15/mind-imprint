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
