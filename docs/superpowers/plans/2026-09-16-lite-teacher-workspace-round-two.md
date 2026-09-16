# 教师工作台第二轮 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 老师只看见她认识的词；阅读库能为班级推荐、用整张卡片来选；修补 D1 的四个缺口；落实产品负责人对五个问题的答复；建成主页（D2）与家长报告（D3）两个工作台。

**Architecture:** 全部建在 D1 已上线的壳上（`POST /api/v1/lite/teacher/workspace/turn` + `WorkspacePanel`）。服务端的工具循环按 `surface` 分派到各自的工具表、系统提示词与画布状态渲染；前端把对话状态提成 `useWorkspaceThread`，三个面共用。没有迁移；只有一条查询改动（退回置未读）。

**Tech Stack:** Go（`net/http`、`pgx`/`sqlc`、`internal/gateway` 工具循环）、React + TypeScript + Vite。

**Spec:** `docs/superpowers/specs/2026-09-16-lite-teacher-workspace-design.md` —— 以 **§12** 为准，§3 的全局约束仍然有效。

**执行辅助（不入库）：** `.superpowers/tmp/followup-map.md` 是本计划动笔前对代码的逐题调查，给出了每一处的文件与行号。实现者先读与本任务相关的那一节。行号可能已经漂移，以周围代码为准。

## Global Constraints

- 每次 LLM 调用声明能力档并计量：对话轮 `gateway.ClassDialogue`，主页摘要 `gateway.ClassDigest`；**不用 `ClassAssess`，不用旗舰档**。计量用 `a.recordLiteLLMCall(ctx, u.ID, uuid.Nil, purpose, resolved, usage)`，每次调用都记，包括没产出的那次。
- 上下文按页面与登录隔离；一轮只带该页面的画布对象与最近 `liteworkspace.TurnsWindow`（8）轮。
- **§6 的两条校验对每一个新的模型产出都成立：** 姓名按花名册（`liteworkspace.UngroundedNames`），人数按计数形状（`liteworkspace.StatedCounts` / `UngroundedCounts`，逐字段）。不要加通用数字检测器。
- 教师工具绝不读取学生与印记的对话正文（`atom_message` 的内容列）。
- 错误显形：「动词+失败：{后台原话}」，不返回兜底话。服务端错误已经带「对话失败：」前缀，前端不要再加一层。
- **模型出错时改工具契约，不改系统提示词去追着它跑。**
- 界面文案守 AGENTS.md §界面文案怎么写；代码注释与 commit message 同样不写文学腔。
- 只写逻辑测试，不写前端渲染测试；UI 在最后由控制者用真浏览器看。
- lite 不得破坏 pro：**不改 `apps/web`**（`ClassesView` 在那里，只能通过 lite 自己的 `renderClassPreview` 注入）。新建文件前先确认同名文件不存在。
- `mk-*` token 不用 Tailwind alpha 语法；不加左侧色条。
- 🚨 本机 `PATH` 上的 `go` 是 x86_64，在这台 arm64 Mac 上跑不起来。用 `/Users/houyuxin/08Coding/mind-imprint/.deploy-local/toolchain/go/bin/go`。
- Go 测试：`CGO_ENABLED=0 go test ./... -count=1 -timeout 1800s`（在 `apps/api` 下）。**不要把测试输出接到 grep/tail 上判断成败**——退出码会变成管道的。若 `internal/api` 大面积 `connection refused`，是 testcontainer 中途退出，重跑一次再下结论。
- sqlc 固定 `v1.27.0`，按 `apps/api/Makefile` 用 `CGO_ENABLED=0 go tool sqlc generate`。**失败时可能退出码仍为 0 却什么都没生成——生成后一定 `git status` 看 sqlc 目录。**
- 前端：`cd apps/lite-web && npx tsc --noEmit && npx vitest run`，全绿。

---

## File Structure

| 文件 | 部分 | 责任 |
|---|---|---|
| `apps/api/internal/liteworkspace/labels.go` (+test) | F1 | 类型/来源/难度的中文标签；slug→标题替换 |
| `apps/api/internal/library/group.go` (+test) | F2 | `RecommendForGroup` 班级聚合推荐 |
| `apps/api/internal/api/lite_teacher_library.go` (+test) | F2 | `GET …/classes/{id}/library/recommended` |
| `apps/api/internal/liteworkspace/tools.go` | F2 F7 D2 D3 | 工具表按 surface 分开 |
| `apps/api/internal/api/lite_teacher_workspace*.go` | F1 F2 F4 F7 D2 D3 | 分派、画布状态、工具执行 |
| `apps/lite-web/src/readings/ArticleCardBody.tsx` | F3 | 从 `LibraryCard` 抽出的纯展示部分 |
| `apps/lite-web/src/teacher/LibraryPicker.tsx` | F3 | 推荐行 + 卡片网格 |
| `apps/lite-web/src/api/teacherLibrary.ts` | F3 | 班级推荐客户端 |
| `apps/lite-web/src/teacher/workspace/useWorkspaceThread.ts` (+logic test) | F5 | 对话状态，三个面共用 |
| `apps/lite-web/src/teacher/workspace/ChoiceArticleCard.tsx` | F4 | 对话里的文章小卡片 |
| `apps/lite-web/src/teacher/StudentViewPreview.tsx` | F6 | 「学生看到的样子」 |
| `apps/api/internal/store/queries/lite_assignment.sql` + 生成物 | G1 | 退回置未读 |
| `apps/api/internal/api/lite_teacher_assignments.go`, `assignmentLogic.ts`, `PersonalizedPicker.tsx`, `AssignmentDetailPage.tsx` | G2 | 按学生加锁 |
| `apps/lite-web/src/writings/WritingRoomHost.tsx` 等 | G3 | 写作室里的老师批改 |
| `apps/api/internal/api/lite_class_summary.go` (+test) | D2 | 班级摘要（digest）与缓存 |
| `apps/lite-web/src/teacher/ClassChatPage.tsx`, `ClassPreview.tsx`, `teacherRouting.ts`, `LiteTeacherShell.tsx` | D2 | 主页摘要与对话页 |
| `apps/api/internal/agent/compose_lite_parent.go` | D3 | 抽出段落校验 |
| `apps/lite-web/src/teacher/ParentReportEditor.tsx` | D3 | 报告编辑器接入对话 |

---

## Part F · AI 模式修补与阅读库

### Task 1: 老师只看见中文标签（F1）

**Files:**
- Create: `apps/api/internal/liteworkspace/labels.go`, `labels_test.go`
- Modify: `apps/api/internal/api/lite_teacher_workspace.go`（`liteWorkspaceCardState` ≈506，回复与选项发出之前）

**Interfaces:**
- Produces: `liteworkspace.KindLabel(string) string`、`SourceLabel(string) string`、`TierLabel(*int) string`、`ReplaceSlugs(text string, title func(slug string) (string, bool)) string`。D2/D3 的画布渲染也用这几个。

- [ ] **Step 1: 写失败的测试** `labels_test.go`：

```go
func TestLabels(t *testing.T) {
	for in, want := range map[string]string{"reading": "阅读", "writing": "写作", "project": "项目"} {
		if got := KindLabel(in); got != want { t.Fatalf("KindLabel(%q)=%q want %q", in, got, want) }
	}
	for in, want := range map[string]string{"library": "分级阅读库", "url": "链接", "text": "正文", "file": "上传文件", "personalized": "个性化"} {
		if got := SourceLabel(in); got != want { t.Fatalf("SourceLabel(%q)=%q want %q", in, got, want) }
	}
	three := 3
	if TierLabel(&three) != "进阶" || TierLabel(nil) != "按学生水平" { t.Fatal("tier labels") }
}

// 前端的三张表是老师真正看到的字；两边一个字都不能差。
func TestLabelsMatchTheWebTables(t *testing.T) {
	// 读 apps/lite-web/src/teacher/AssignmentForm.tsx 的 KIND_OPTIONS / SOURCE_OPTIONS
	// 与 assignmentLogic.ts 的 TIER_NAMES 源码文本，断言每一对 value/label 都出现。
	// 路径相对本包用 ../../../lite-web/... 拼出。
}

func TestReplaceSlugs(t *testing.T) {
	titles := map[string]string{"biden-creates-climate-corps": "美国气候队"}
	lookup := func(s string) (string, bool) { v, ok := titles[s]; return v, ok }
	got := ReplaceSlugs("文章：biden-creates-climate-corps", lookup)
	if got != "文章：《美国气候队》" { t.Fatalf("got %q", got) }
	// 不是已知 slug 的连字符词不动
	if ReplaceSlugs("well-known 方法", lookup) != "well-known 方法" { t.Fatal("touched a non-slug") }
}
```

- [ ] **Step 2:** 跑测试，确认失败。
- [ ] **Step 3: 实现。** `ReplaceSlugs` 按 `[a-z0-9]+(-[a-z0-9]+)+` 找候选词，只替换 `title` 查得到的；已经在《》里的不再套一层。表的值与前端逐字一致。
- [ ] **Step 4: 改 `liteWorkspaceCardState`**：`类型：{KindLabel}`、`材料来源：{SourceLabel}`、`文章：《{ZhTitle}》`（查不到就写「文章：未找到」，不写 slug）、`难度：{TierLabel}`。在 `lite_teacher_workspace_test.go` 加一条：给一个带 slug 与 tier 的画布，断言渲染结果里不含 `reading`、`library`、`personalized`、slug 原文，且含《中文标题》。
- [ ] **Step 5: 发出前替换 slug**：回复文字与每个选项 label 过一遍 `ReplaceSlugs(…, 用 library.BySlug 取 ZhTitle)`。**替换发生在 §6 校验之后**，校验看的是模型的原话。加一条端点测试：stub 模型回复里写 slug，响应里变成《标题》。
- [ ] **Step 6:** `liteworkspace` 与 `api` 包测试、gofmt、vet。
- [ ] **Step 7: Commit** `feat(api): the teacher reads article titles and Chinese labels, never wire values`

### Task 2: 班级推荐（F2）

**Files:**
- Create: `apps/api/internal/library/group.go`, `group_test.go`
- Create: `apps/api/internal/api/lite_teacher_library.go`, `lite_teacher_library_test.go`
- Modify: `apps/api/internal/api/lite_teacher_routes.go`
- Modify: `apps/api/internal/liteworkspace/tools.go`、`lite_teacher_workspace.go`（新工具 `recommend_articles`）

**Interfaces:**
- Produces: `library.RecommendForGroup(articles []Article, members []Profile, limit int) []GroupRecommendation`，`GroupRecommendation{ Article; Why []string; ReadCount int; Tier int }`。
- Produces: `GET /api/v1/lite/teacher/classes/{id}/library/recommended?limit=8` → `{ "articles": [ <与 GET /library 的文章同形> + "why": [..], "readCount": n ], "tier": n }`。

- [ ] **Step 1: 写失败的测试** `group_test.go`：
  - 两个学生都偏 `climate-ocean`、一个偏 `history` → 排第一的是 climate 文章，`Why` 含该学科。
  - `ReadCount` = 画像 `ReadSlugs` 里含这篇的学生数。
  - 组的 `Tier` = 成员 `Tier` 的中位数（偶数个取较低的那个）；成员为空时为 `library.SuggestTier(0,0)` 的结果。
  - `limit` 生效；同样输入两次输出顺序一致。
- [ ] **Step 2: 实现** —— 把成员的 `Disciplines` 强度相加成一个 `Profile`，`ReadSlugs` 置空（全班读过的文章仍可推荐，只标出读过的人数），交给 `Recommend`；再附 `ReadCount` 与组 `Tier`。
- [ ] **Step 3: 端点。** `liteTeacher` 包装 + `authTeacherClass`；成员画像逐个用 `libraryProfileIn`（照 `previewLitePersonalizedReading` 的循环写，不新增查询）。文章 DTO **复用** `getLibraryShelf` 的文章序列化（找到那个函数，不要另写一份，封面地址才一致）。测试：非本班老师 404、学生 403、返回条数 ≤ limit、带 `why`/`readCount`/`tier`。
- [ ] **Step 4: 工具 `recommend_articles`**（参数可选 `disciplines []string`，`limit` 固定 6）：返回 slug、zhTitle、理由、已读人数、组 tier。系统提示词里只加一句「老师没指定文章时，先用 recommend_articles 给出推荐」——这是新增能力的说明，不是追着某个错误改提示词。端点测试：stub 模型调用它，工具结果回灌。
- [ ] **Step 5:** 测试、gofmt、vet。**Commit** `feat(api): recommend reading for a class, as a route and as a tool`

### Task 3: 整张卡片的阅读库选择器（F3）

**Files:**
- Create: `apps/lite-web/src/readings/ArticleCardBody.tsx`
- Modify: `apps/lite-web/src/readings/LibraryCard.tsx`（改为使用 `ArticleCardBody`，**外观与行为不变**）
- Create: `apps/lite-web/src/api/teacherLibrary.ts`（`getClassRecommendations(classId)` + normalizer）
- Modify: `apps/lite-web/src/teacher/LibraryPicker.tsx`、其调用处（`SettingsFields` 与个性化 `SwapDialog`）传入 `classId`

**Interfaces:**
- Consumes: Task 2 的端点。
- Produces: `LibraryPicker` 新增必填 `classId: string`（空串时不显示推荐行）。

- [ ] **Step 1:** 读 `LibraryCard.tsx`，把封面、中文标题、理由、学科标签抽到 `ArticleCardBody({ article, why?, footer? })`；`LibraryCard` 用它渲染，学生端的「开始读/继续读/已完成」与难度选择仍留在 `LibraryCard`。学生端现有测试必须仍然全绿。
- [ ] **Step 2: 客户端 + normalizer**，逻辑测试覆盖：缺 `why` 时为空数组、`readCount` 缺省 0、非法 tier 归一为 null（沿用已导出的 `tierOrNull`）。
- [ ] **Step 3: 重做 `LibraryPicker`：**
  - 顶部「为这个班推荐」：一行卡片（宽屏 3–4 张，窄屏横向滚动），卡片上显示「已读 N 人」（N>0 时）与理由。
  - 其下「全部文章」：卡片网格，保留按标题搜索（沿用 `filterArticles`）与学科筛选（沿用 `disciplineOptions`）。
  - 选中态：边框 + 「已选」标记；点已选的卡片不取消。
  - 难度选择放在网格下方，逻辑不变（`nullLabel` 仍可传入）。
  - 推荐加载失败只在推荐行显示「推荐加载失败：{原话}」+ 重试，不影响全部文章。
  - 把选择/过滤逻辑里值得测的部分（比如「推荐里的文章在全部文章里也能被选中且状态一致」）写成纯函数加测试；不写渲染测试。
- [ ] **Step 4:** 所有调用处传 `classId`。`tsc` + `vitest` 全绿。
- [ ] **Step 5: Commit** `feat(lite-web): pick reading from full cards, with a row recommended for the class`

### Task 4: 对话里的文章小卡片（F4）

**Files:**
- Modify: `apps/api/internal/liteworkspace/workspace.go`（`Choice` 增加可选 `Article *ChoiceArticle`）
- Modify: `apps/api/internal/api/lite_teacher_workspace.go`（带 slug 的选项在发出前补全）
- Create: `apps/lite-web/src/teacher/workspace/ChoiceArticleCard.tsx`
- Modify: `apps/lite-web/src/api/teacherWorkspace.ts`（normalizer）、`WorkspacePanel.tsx`

**Interfaces:**
- Produces: 响应里的选项可带 `article: { slug, zhTitle, coverUrl, reason }`；前端 `Choice.article?`。

- [ ] **Step 1:** 服务端：选项有 slug 时用 `library.BySlug` 补 `zhTitle`、封面地址（**与 Task 2/`getLibraryShelf` 用同一个封面地址函数**）、`reason`；slug 查不到的选项按现有规则整组拒绝。测试：补全后的字段正确；label 仍过 §6 校验。
- [ ] **Step 2:** normalizer 逻辑测试：缺 `article` 时为 undefined；`coverUrl` 为空串时不渲染图片。
- [ ] **Step 3:** `WorkspacePanel` 渲染：有 `article` 的选项用 `ChoiceArticleCard`（封面缩略图 + 中文标题 + 一行理由，整张可点），没有的仍是按钮。点击行为与按钮完全一样（`onChoose`）。对话栏窄，卡片纵向排列。`WorkspacePanel` 仍然不含任何作业专属文案。
- [ ] **Step 4: Commit** `feat(workspace): offer articles as cards in the conversation`

### Task 5: 对话状态提升、失败的话放回输入框（F5）

**Files:**
- Create: `apps/lite-web/src/teacher/workspace/useWorkspaceThread.ts` 与其纯逻辑的测试
- Modify: `AssignmentForm.tsx`、`AssignmentAIMode.tsx`、`WorkspacePanel.tsx`

**Interfaces:**
- Produces: `useWorkspaceThread()` 返回 `{ turns, busy, error, choices, cards, kept, run(input), reset() }`，内部保留 D1 的全部规则：`trimTurns`、`clampChoices`、快照 + `applyPatch`（调用方传入 `getCurrent/applyNext`）、代次计数器 `isCurrentTurn`、失败时 `rollbackTurn`。**D2、D3 用同一个钩子。**
- Produces: `WorkspacePanel` 新增可选 `restoreText?: string`（失败后要放回输入框的话）。

- [ ] **Step 1:** 把 `AssignmentAIMode` 里的状态与 `runTurn` 迁进钩子；能拆成纯函数的（状态迁移）拆出来写逻辑测试：成功一轮、失败回滚、代次不符时丢弃、`reset()` 让在飞的一轮作废。
- [ ] **Step 2:** `AssignmentForm` 持有钩子实例并传给 `AssignmentAIMode`，**切换到传统模式再切回来，对话、选项、卡片、保留提示都还在**。换班级仍调用 `reset()`（D1 的规则不变）。
- [ ] **Step 3: 失败的话放回输入框。** 一轮失败（且仍是当前代次）时，钩子暴露这句话；`WorkspacePanel` 在输入框为空时把它填回去，已有内容时不覆盖。「重试」照旧。纯逻辑部分写测试（何时填回、何时不填）。
- [ ] **Step 4:** `tsc` + `vitest`。**Commit** `fix(lite-web): the conversation survives a mode switch and a failed sentence returns to the input`

### Task 6: 「学生看到的样子」（F6）

**Files:**
- Create: `apps/lite-web/src/teacher/StudentViewPreview.tsx`
- Modify: `AssignmentAIMode.tsx`

- [ ] **Step 1:** 读 `apps/lite-web/src/inbox/AssignmentStrip.tsx`，找到作业条的展示部分。能直接复用就复用；需要抽出纯展示组件时，**学生端外观不变**。
- [ ] **Step 2:** 预览只用草稿已有的字段：类型、标题、说明、截止时间（`dueInput` 转成学生端的截止显示）、班级名；状态固定「未开始」（用已有的 `STATUS_LABEL`/`statusChipStyle`）。没有真实 id、未读、退回信息——不显示这些位置。把「草稿 → 预览数据」写成纯函数并测试（空标题时显示「未填写标题」这类占位由现有文案规则决定，不新造）。
- [ ] **Step 3:** 放在 AI 模式作业卡底部、发布按钮之上，默认折叠，标签「学生看到的样子」。
- [ ] **Step 4: Commit** `feat(lite-web): preview the homework as a student will see it`

### Task 7: AI 模式可用贴进来的正文（F7）

**Files:**
- Modify: `apps/api/internal/liteworkspace/tools.go`（`set_material` 的 `source` 增加 `text`，新增 `text` 参数）
- Modify: `apps/api/internal/api/lite_teacher_workspace.go`（`setMaterial`，历史轮次截断）
- Modify: `apps/lite-web/src/teacher/workspace/workspaceLogic.ts`（发送时截断历史）

- [ ] **Step 1: 写失败的测试：**
  - 老师这一轮贴了一段正文，模型 `set_material{source:"text", text:<其中一段>}` → patch 里 `readingSource:"text"`、`text` 为该段。
  - 模型给的 `text` **不是**老师这一轮输入的子串（去首尾空白后比较）→ 工具错误「正文必须来自老师贴进来的内容」，patch 不变。
  - 超过 50000 字 → 工具错误（与 `validateSettings` 的上限一致）。
  - 卡片是写作类型时仍按 D1 规则拒绝（材料只能在阅读作业上）。
- [ ] **Step 2: 实现。** 子串判断只看**本轮**老师输入的原文，不看历史、不看模型自己的话。
- [ ] **Step 3: 历史截断。** 发送历史时每条 teacher/ai 轮最多 1000 字（超出截断并以「…」结尾），当前这一轮完整发送。前端纯函数 + 测试；服务端对收到的历史再做一次同样的截断（不信任客户端），并测试。
- [ ] **Step 4:** 工具说明写清：`text` 只能照抄老师这一轮贴来的正文。**Commit** `feat(workspace): use pasted article text, only as the teacher pasted it`

---

## Part G · 产品负责人的五个答复

### Task 8: 退回置未读；钉住「可以往前设截止时间」（G1）

**Files:**
- Modify: `apps/api/internal/store/queries/lite_assignment.sql`（`SetLiteAssignmentReturned`）+ 生成的 `sqlc` 文件
- Test: `apps/api/internal/api/lite_assignment_return_test.go`、`apps/lite-web/src/teacher/assignmentLogic.test.ts`

- [ ] **Step 1: 写失败的测试：** 学生已打开过作业（`seen_at` 有值），老师退回 → 学生收件箱这一项 `unread: true`，未读数 +1。
- [ ] **Step 2:** 查询里加 `seen_at = NULL`。按全局约束跑 sqlc，**跑完 `git status` 确认生成文件真的变了**。生成不了时可以手改生成文件，但要在报告里说明。
- [ ] **Step 3: 钉住已有行为：** 服务端测试——新截止时间早于原截止、晚于现在 → 200；前端 `buildReturnInput` 测试同样的输入 → ok。确认 `ReturnDialog` 的时间输入没有 `min` 之类把它挡住（读代码，有就去掉并说明）。
- [ ] **Step 4:** 检查学生端收件箱与作业条在「已退回」时的未读展示没有别的地方另外判定而忽略了 `seen_at`。
- [ ] **Step 5: Commit** `feat(api): a returned homework is unread again for the student`

### Task 9: 个性化文章按学生加锁（G2）

**Files:**
- Modify: `apps/api/internal/api/lite_teacher_assignments.go`（`patchLiteAssignment` 的 `settingsChanged` 分支）
- Modify: `apps/lite-web/src/teacher/assignmentLogic.ts`（新增按行判断）、`PersonalizedPicker.tsx`、`AssignmentDetailPage.tsx`

**Interfaces:**
- Produces: `recipientStarted(r: RecipientDTO): boolean`；`canSwapPick(recipients, userId): boolean`。

- [ ] **Step 1: 服务端失败的测试：**
  - 个性化作业，A 已开始、B 未开始；PATCH 只改 B 的 pick → 200，B 的 pick 已更新，A 的不变。
  - 同一作业 PATCH 改 A 的 pick → 409（沿用 `errAssignmentStarted`，原话写清是哪位学生已开始）。
  - 改学科筛选、全班难度、来源或类型 → 仍 409。
  - 无人开始时行为不变。
- [ ] **Step 2: 实现。** 设置有变且已有人开始时，只放行这一种情况：类型与来源不变、`disciplines` 与全班 `tier` 不变、payload 里只有 `picks` 不同、且所有变化的 pick 都属于 `started_at IS NULL` 的学生。读每个收件人的 `started_at`（已有查询里有，不新增查询；没有就说明）。
- [ ] **Step 3: 前端。** `canEditSettings` 保持原义（其他设置仍整体锁）；个性化表格每一行按 `canSwapPick` 决定「更换」是否可用，已开始的行显示「已开始」且按钮禁用。逻辑测试覆盖两个新函数。详情页在「有人已开始」时仍然允许打开个性化表格做单行更换，其他设置保持只读。
- [ ] **Step 4: Commit** `feat: a personalised pick locks only once that student starts`

### Task 10: 写作室里的老师批改（G3）

**Files:**
- Modify: `apps/lite-web/src/writings/WritingRoomHost.tsx`（`ready` 阶段）及其布局组件
- Modify（如需）: `apps/lite-web/src/writings/TeacherGradingPanel.tsx`（让引文点击可选）

- [ ] **Step 1:** 读 `FinishedWritingPage.tsx` 如何取 `listWritingGradings` 与版本正文。
- [ ] **Step 2:** 在写作室（修改已完成的作文时，即这篇作文有已发送的批改）放一个只读、默认展开、可折叠的「老师批改」面板，复用 `TeacherGradingPanel`。没有已发送批改时不显示。取数失败显示「加载失败：{原话}」。
- [ ] **Step 3:** 引文：在当前草稿里逐字找得到才可点（点了在草稿里定位/高亮，沿用写作室已有的定位方式；没有现成方式就只做滚动到位置），找不到就只显示文字。把「引文在草稿里的位置」写成纯函数并测试（沿用 `internal/quotematch` 同一套归一化规则的前端版本，已有就复用）。
- [ ] **Step 4:** 批改面板**不能**盖住印记的对话面板，也不能挤掉正文输入区；窄屏时放在正文下方。
- [ ] **Step 5: Commit** `feat(lite-web): the writing room shows the teacher's feedback while she revises`

---

## Part D2 · 主页

### Task 11: 按 surface 分派工具循环（D2 前置）

**Files:**
- Modify: `apps/api/internal/api/lite_teacher_workspace.go`（可拆出 `lite_teacher_workspace_assignment.go`）
- Modify: `apps/api/internal/liteworkspace/tools.go`

**Interfaces:**
- Produces: 一个内部接口，例如 `type liteWorkspaceSurface interface { tools() []gateway.ChatTool; system() string; cardState() string; execute(tc) (string, bool /*terminal*/); patch() map[string]any; cards() []…; navigate() *…; checkedParts() []string; grounded() (names []string, counts []int) }`。作业面是第一个实现。
- 请求体新增 `reportId`（D3 用），`surface` 允许 `assignment`、`home`、`parentReport`；未实现的面仍 400。

- [ ] **Step 1:** 纯重构，**行为不变**：D1 的全部测试（含 live 测试的编译）照旧通过。循环、计量、§6 校验、slug 替换、选项截断留在公共部分，各面只提供上面那些方法。
- [ ] **Step 2:** 新增测试：未知 surface 仍 400；`home` 与 `parentReport` 在各自任务完成前 400。
- [ ] **Step 3: Commit** `refactor(api): dispatch the workspace loop by surface`

### Task 12: 班级摘要（D2）

**Files:**
- Create: `apps/api/internal/api/lite_class_summary.go`、`lite_class_summary_test.go`
- Modify: `apps/api/internal/api/lite_teacher_routes.go`

**Interfaces:**
- Produces: `POST /api/v1/lite/teacher/classes/{id}/summary` → `{ "summary": "…", "generatedAt": "…", "cached": bool }`。

- [ ] **Step 1: 写失败的测试：** 鉴权（401/403/非本班 404）；stub 模型返回一段话 → 200；同一天同一数据第二次请求不调模型（`cached: true`，stub 调用次数不变）；花名册变化（指纹变）后重算；模型提到花名册里没有依据的姓名或人数 → 502「摘要生成失败：…」，不缓存失败结果；每次真实调用记一行 `llm_call`，档位 `digest`。
- [ ] **Step 2: 实现。** 输入：花名册聚合与本周统计（`loadLiteClassWeek` / `liteClassWeekStats` / 卡片数据，按 `getLiteClassWeekly` 的写法取，不新增查询）。提示词：一到三句，说这个班这周值得注意的事，不写具体人数与姓名以外没有给出的内容；按 §6 校验（姓名：给出的学生名单即依据；人数：给出的统计即依据）。缓存：进程内 map，键为 `classId + 北京日期 + 花名册指纹`（指纹用花名册各行的活动字段哈希），设上限（如 512 条，满了清掉最旧的）。
- [ ] **Step 3: Commit** `feat(api): a short weekly summary for each class, cached per day`

### Task 13: 主页对话面（D2）

**Files:**
- Create: `apps/api/internal/api/lite_teacher_workspace_home.go`（+test）
- Modify: `apps/api/internal/liteworkspace/tools.go`（`HomeTools()`、`HomeSystem(...)`）

**Interfaces:**
- 工具：`class_snapshot`（本周统计 + 花名册聚合）、`list_students`（D1 闭集）、`list_assignments`（本班作业：标题、类型中文、截止、各状态人数）、`open_page{target, userId?, assignmentId?}`、`ask_choice`。
- `navigate` 响应：`{ view, classId, userId?, assignmentId?, label }`，`label` 为中文页面名（本周报告 / {学生名}的学习页 / 布置作业 / {作业标题} / 家长报告）。

- [ ] **Step 1: 写失败的测试：**
  - `open_page` 目标不在闭集 → 工具错误；`student` 的 `userId` 不在花名册 → 工具错误；`assignment` 的 id 不属于本班 → 工具错误。
  - 合法 `open_page` → 响应带 `navigate`，**服务端不做任何跳转副作用**。
  - `list_assignments` 的卡片数据来自真实查询；回复里的人数按 §6 校验，作业里给出的各状态人数算依据。
  - 画布状态、回复里没有线值（沿用 Task 1）。
- [ ] **Step 2: 实现。** 各状态人数复用作业列表已有的统计（`listLiteAssignments` 的状态计数），不新增查询；没有现成的就说明并用已有查询在 Go 里算。
- [ ] **Step 3: Commit** `feat(api): the home workspace answers about the class and offers pages to open`

### Task 14: 主页界面（D2）

**Files:**
- Modify: `apps/lite-web/src/teacher/teacherRouting.ts`（新 view `classChat`，路径 `/classes/{id}/chat`）+ 测试
- Modify: `apps/lite-web/src/teacher/LiteTeacherShell.tsx`、`ClassPreview.tsx`
- Create: `apps/lite-web/src/teacher/ClassChatPage.tsx`、`apps/lite-web/src/api/classSummary.ts`

- [ ] **Step 1:** 路由：`parseTeacherRoute` / `teacherRoutePath` 往返测试。
- [ ] **Step 2: 摘要。** `ClassPreview` 在进入视口后（沿用现有的 IntersectionObserver）请求摘要，显示在卡片顶部；加载中「摘要生成中」，失败「摘要生成失败：{原话}」+ 重试；整段可点，点击进入 `classChat`，**点击不能同时触发卡片本身的进入班级**（阻止冒泡）。
- [ ] **Step 3: 对话页。** 同一个 `WorkspacePanel` + `useWorkspaceThread`（surface `home`）。画布：顶部班级概况（复用 `LearningSnapshot`），下面是工具结果卡片（学生列表、作业列表），`navigate` 渲染成按钮「前往：{label}」，**点了才调用 `go(...)`**。页面有「返回班级」。
- [ ] **Step 4:** 纯逻辑（`navigate` → `TeacherRoute` 的映射、非法值丢弃）写测试。
- [ ] **Step 5: Commit** `feat(lite-web): a summary on each class card that opens a class conversation`

---

## Part D3 · 家长报告

### Task 15: 家长报告对话面（D3 服务端）

**Files:**
- Modify: `apps/api/internal/agent/compose_lite_parent.go`（抽出 `CheckLiteParentSections(p map[string]string, sections []string, f liteparent.Facts, otherNames []string) error`，生成流程改为调用它，**行为不变**）
- Create: `apps/api/internal/api/lite_teacher_workspace_report.go`（+test）
- Modify: `apps/api/internal/liteworkspace/tools.go`（`ReportTools()`、`ReportSystem(...)`）

**Interfaces:**
- 请求 `surface: "parentReport"`, `reportId`；归属照 `loadTeacherParentReport`（查报告 → `assertTeacherOwnsClass(rep.ClassID)`）。
- 工具 `revise_section{section, text}`、`ask_choice`。patch 形如 `{ "body": { "<section>": "<text>" } }`。

- [ ] **Step 1: 写失败的测试：**
  - 非本班老师的 reportId → 404；学生 → 403。
  - `section` 不在 `liteparent.SectionsWithFacts(VisibleFacts(…))` 里 → 工具错误；超过 2000 字 → 工具错误。
  - 改写文字里出现别的学生的名字、事实里没有的数字、事实里没有的引文 → 工具错误（**与生成草稿的检查是同一个函数**），patch 不变；模型下一次调用改对了 → patch 生效。
  - 学生已离开班级 → 400/409，与重新生成的禁用理由一致。
  - **工具不写库**：调用后报告的 body 在数据库里不变。
  - 画布状态：各段标题用 `liteparent.SectionLabels` 的中文，给出当前文字与可见事实。
- [ ] **Step 2:** 抽函数时保留 `compose_lite_parent.go` 现有测试全绿。
- [ ] **Step 3: Commit** `feat(api): revise a parent report section by conversation, under the draft's own checks`

### Task 16: 家长报告界面（D3 前端）

**Files:**
- Modify: `apps/lite-web/src/teacher/ParentReportEditor.tsx`
- Modify（如需）: `apps/lite-web/src/teacher/parentReportLogic.ts`（纯逻辑 + 测试）

- [ ] **Step 1:** 编辑器外面套 `WorkspacePanel` + `useWorkspaceThread`（surface `parentReport`，`artifact` 为当前各段文字）。
- [ ] **Step 2: 写入只有一条路。** 回来的 patch 逐段过 `applyPatch`（老师这一轮手改过的段保留，并显示「{段名} 已保留你的修改」），剩下的段**通过编辑器已有的那一个 `SerialQueue`** 调用现有的保存路径（与 `storeSection` 同一路），返回的报告照常 `applyReport`。不要新建第二条队列或直接调接口绕过它。纯逻辑部分（哪些段写、哪些保留）写测试。
- [ ] **Step 3:** 学生已离开时对话输入与选项禁用，显示与重新生成相同的原因。导出不变。
- [ ] **Step 4: 宽度。** 三栏放不下（< 1400px）时，编辑器的「草稿 / 预览」两栏改为切换（`Segmented`）。
- [ ] **Step 5: Commit** `feat(lite-web): revise a parent report by conversation beside the editor`

---

## Task 17: 真模型验证

- [ ] 扩展 `apps/api/internal/api/lite_teacher_workspace_live_test.go`（`LIVE_LLM=1`，密钥只从 `/Users/houyuxin/08Coding/mind-imprint/.deploy-local/env.prod` 读取，绝不打印），**每个场景跑三次，分别报告**：
  1. 作业面：回复与选项里没有 `reading`/`library`/`personalized`/任何 slug；老师没指定文章时模型用了 `recommend_articles`；带文章的选项有 `article` 字段。
  2. 作业面：老师贴一段正文让模型设为材料 → `readingSource: text` 且正文是她贴的原文。
  3. 主页面：「这周谁还没动」→ 用了 `list_students`，回复里没有无依据的姓名与人数；「带我去看周报」→ 有 `navigate`。
  4. 家长报告面：「把阅读那段写得具体一点」→ 有 `body.reading` 的 patch，且通过段落检查。
  5. 班级摘要：三次都通过 §6 校验。
- [ ] 模型不听话时**只诊断、不改提示词**，把原始输出写进报告。
- [ ] 检测器本身有漏看的风险：凡是走查依赖的检测器，用逃出去的那句原话写一条不需要 `LIVE_LLM` 的测试钉住。

---

## Self-Review

**Spec §12 coverage：** 12.1 → Task 1；12.2 → Task 2、3、4；12.3 → Task 5（1、2）、6（3）、7（4）；12.4 → Task 8（前两行）、Task 8 Step 3 与浏览器核对（第三行）、Task 10（第四行）、Task 9（第五行）；12.5 → Task 11–14；12.6 → Task 15–16；§8 真模型 → Task 17。

**Placeholder scan：** 前端任务给的是约束与测试要点而非完整代码——这些组件改动依赖实现者当场读现有组件，照抄一份臆测的 JSX 反而会与现有写法冲突；逻辑部分都要求纯函数加测试。Task 1 的跨语言钉子测试写了做法，没写完整代码，是因为路径拼接要看实际目录。

**Type consistency：** `Choice.article` 在 Task 4 定义，前后端同名；`useWorkspaceThread` 在 Task 5 定义，Task 14、16 使用；surface 接口在 Task 11 定义，Task 13、15 实现；`KindLabel`/`SourceLabel`/`TierLabel` 在 Task 1 定义，Task 13、15 的画布渲染复用。

**顺序依赖：** Task 5 在 Task 14、16 之前；Task 11 在 Task 13、15 之前；Task 2 在 Task 3、4 之前；Task 1 在 Task 13、15 之前。
