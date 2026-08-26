# 轻量版 P3（写作）Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 让一个 lite 学校的学生把一个念头打进一个框，和 AI 聊清楚，走过 构思 → 大纲 → 段落 → 成稿 四步，**自己写完每一个字**，拿到反馈与简版报告。

**Architecture:** 写作是 `atom` 的第二种形态。**共享底座一张表都不用新建**——`atom` / `atom_message` / `atom_card` / `atom_annotation` / `atom_report` 全部复用，写作只加自己的四张专属表。AI 层继续原样复用（P1 已证明 `internal/agent` 零改动可行）。前端复用 `apps/lite-web` 的壳与 `apps/web` 的组件与设计 token。

**Tech Stack:** Go（`net/http`、`pgx`、`sqlc`、`goose`）、PostgreSQL、React + Vite + TypeScript + Tailwind、Playwright。

**Spec:** `docs/superpowers/specs/2026-08-26-lite-edition-writings-readings-design.md` §6.2

## Global Constraints

- **迁移编号接着 P1 往后排**（P1 用到 `0095`）。先 `ls apps/api/internal/store/migrations | sort | tail -3` 确认当前最高号再写。
- **sqlc 重新生成：`cd apps/api && make sqlc`**；绝不手改 `internal/store/sqlc/`。
- 🚨 **实现者只跑定向测试**（`-run TestX -timeout 1800s`）。**绝不跑整包 Go 测试**——每个集成测试都会起一个 Postgres 容器，整包超过十分钟、越过前台命令上限。整包验证由 controller 统一在后台跑。
- **绝不 `git add -A`**；只 stage 本任务列出的文件。
- **归属失败一律 404**（`httpx.ErrNotFound("资源不存在")`），绝不用 403。
- `{id}` **一律是 atom id**；每个 handler 先过 `loadOwnedWritingAtom`。
- `apps/api/internal/api` 是与 pro 共享的**同一个 Go package**：`putOutline` / `getDraft` / `listSnippets` 等名字**pro 已占用**。lite 一律加 `lite` 前缀或 `Lite` 后缀，动手前 grep。
- **铁律① 是本期的红线**：AI 绝不写学生的正文。提纲派生与「合成全文」是**确定性系统步骤**（AGENTS.md 明确允许）；合成只拼接学生已写的片段，**不新造一个字**；英文示范段落明确标注、与草稿分离、界面无插入入口、**不写入任何草稿表**。
- **铁律②**：阶段是地图不是关卡；不做连胜、排行榜、推送。
- **铁律④**：跳过阶段允许，但**必须留痕**并进入报告的确定性事实区。
- `mk-*` 是裸 CSS 变量：`bg-mk-x/NN` 一律不产出 CSS，用 `linear-gradient` / `color-mix` / `box-shadow`。

---

### Task W1: 写作专属四张表

**Files:**
- Create: `apps/api/internal/store/migrations/00XX_writing_tables.sql`（编号见 Global Constraints）
- Create: `apps/api/internal/store/queries/writing.sql`
- Regenerate: `apps/api/internal/store/sqlc/`
- Test: `apps/api/internal/store/writing_store_test.go`

**Interfaces:**
- Consumes: P1 的 `atom` 表
- Produces: 表 `writing` / `writing_outline` / `writing_snippet` / `writing_draft`，以及 sqlc 方法 `CreateWriting` / `GetWriting` / `ListWritingsByUser` / `RenameWriting` / `SetWritingStage` / `SetWritingTargetWords` / `SetWritingFinished` / `ReplaceWritingOutline` / `ListWritingOutline` / `UpsertWritingSnippet` / `ListWritingSnippets` / `UpsertWritingDraft` / `GetWritingDraft`

- [ ] **Step 1: 写下会失败的测试**

照 `apps/api/internal/store/atom_store_test.go`（P1 Task 1 的产物）的形状写，用本包既有的 pool helper（`newStoreTestPool`）。必须覆盖：

1. `CreateWriting` 后 `stage` 默认 `'ideate'`、`target_words` 为 NULL、`status` 默认 `'active'`。
2. `ListWritingsByUser` 只返回本人的、按 `atom.created_at DESC`。
3. **级联**：`DELETE FROM atom` 后 outline / snippet / draft 三张表都为空——与阅读同一条不变式。
4. `stage` 的 CHECK 拒绝非法值（例如 `'drafting'`）。

- [ ] **Step 2: 跑测试确认失败**

```bash
cd apps/api && go test ./internal/store/ -run TestWritingStore -timeout 1800s
```

- [ ] **Step 3: 写迁移**

表结构逐字取自 spec §6.2.4。四张表全部 `atom_id … REFERENCES atom(id) ON DELETE CASCADE`。Down 按依赖倒序 DROP。

`stage` 的 CHECK 必须是 `('ideate','outline','snippets','draft','finished')`——**五个值，不是四个**：`finished` 是终态，与 `status` 的 `finished` 并存（`status` 说这篇结束了，`stage` 说她走到哪一步）。

- [ ] **Step 4: 写查询、regen、跑通过、提交**

```bash
cd apps/api && make sqlc && go test ./internal/store/ -run TestWritingStore -timeout 1800s
git add apps/api/internal/store/migrations/ apps/api/internal/store/queries/writing.sql apps/api/internal/store/sqlc/ apps/api/internal/store/writing_store_test.go
git commit -m "feat(lite): writing atom tables — outline, snippets, draft"
```

---

### Task W2: 写作原子 CRUD（一个框进去）

**Files:**
- Create: `apps/api/internal/api/writings.go`
- Modify: `apps/api/internal/api/api.go`
- Test: `apps/api/internal/api/writings_test.go`

**Interfaces:**
- Consumes: W1；P1 的 `liteOnly`、`liteHandler` 测试 helper
- Produces:
  - `POST /api/v1/writings` — body `{idea, lang}` → `201 {id}`
  - `GET /api/v1/writings` → `{writings:[writingDTO]}`
  - `GET|PATCH /api/v1/writings/{id}`
  - `writingDTO` = `{id,title,lang,stage,targetWords,status,createdAt,updatedAt,finishedAt}`
  - `func (a *API) loadOwnedWritingAtom(w, r) (sqlc.Atom, bool)` —— W3-W7 全部复用

- [ ] **Step 1: 写下会失败的测试**

关键行为，逐条断言：

- `POST /writings` 用 **`idea`** 建原子：`idea` 是学生打进框里的那句话，**它同时成为初始 `title`**（截断到 200 runes），并**作为第一条 `atom_message`（role='student'）写进对话**——因为「先聊」的第一句就是她说的这句，不该消失。
- 空 `idea` → 400 `missing_idea`。这与阅读不同：阅读可以先建后贴，写作没有想法就没有可聊的东西。
- 新建的 `stage` 是 `'ideate'`，`targetWords` 是 `null`。
- 列表只含本人、新的在前。
- 未知 id → 404；`writing` 形态的 atom 走 `/readings/{id}` → 404，反向亦然（**跨形态隔离**，与 P1 Task 3 的 wrong-kind 断言同一条不变式）。

- [ ] **Step 2-5: 跑失败 → 实现 → 跑通过 → 提交**

实现要点：建 `atom(kind='writing')` + `writing` + 第一条 `atom_message`，**同一个事务**。

```bash
cd apps/api && go test ./internal/api/ -run 'TestCreateWriting|TestListWritings|TestGetWriting' -timeout 1800s
```

---

### Task W3: 阶段与篇幅

**Files:**
- Create: `apps/api/internal/api/writing_stage.go`
- Modify: `apps/api/internal/api/api.go`
- Test: `apps/api/internal/api/writing_stage_test.go`

**Interfaces:**
- Produces:
  - `POST /api/v1/writings/{id}/stage` — body `{stage}` → `200 writingDTO`
  - `PUT /api/v1/writings/{id}/target-words` — body `{targetWords}` → `200 writingDTO`

- [ ] **Step 1: 写下会失败的测试**

这是本期最容易做错的地方，测试要把设计意图钉死：

- **阶段是地图，不是关卡**：可以从 `ideate` 直接跳到 `snippets`（跳过 `outline`）→ **200**，不是 400。铁律②：不设门槛。
- **但跳过要留痕**（铁律④）：每次阶段变化写一条 `atom_message`（role='system'），内容记录 from→to。断言这条记录存在，且**跳过时也存在**。
- **可以回退**：`snippets` → `outline` → 200。学生想回去补提纲是正常的。
- 非法阶段值 → 400。
- `targetWords` 必须是正整数且有上限（例如 1..100000），越界 400。
- `targetWords` 可以在任何阶段设置——虽然 spec 说构思阶段敲定，但**不强制**，同样是地图不是关卡。

- [ ] **Step 2-5: 跑失败 → 实现 → 跑通过 → 提交**

---

### Task W4: 写作陪练一轮（复用 AI 大脑）

**Files:**
- Create: `apps/api/internal/api/writing_turn.go`
- Modify: `apps/api/internal/api/api.go`
- Test: `apps/api/internal/api/writing_turn_test.go`

**Interfaces:**
- Produces: `POST /api/v1/writings/{id}/turn` → `{reply, decision, card|null, nudge, hintCardId}`；`GET /api/v1/writings/{id}/messages`

- [ ] **Step 1: 先读 AI 层，决定复用哪一个入口**

P1 的阅读用 `agent.RouteReading`，它的输入是「文章 + 学生的话」。写作没有文章，有的是**想法、提纲、已写的片段**。

```bash
grep -rn "^func " apps/api/internal/agent/*.go | grep -iv test | grep -iE "coach|route|guide|propose" | head -20
sed -n '1,60p' apps/api/internal/api/reading_turn.go
```

**判断并在报告里写明**：是复用一个既有的写作/陪练入口（若存在），还是用与 `RouteReading` 同形的方式装配一个写作输入。**不论选哪条，都不许修改 `internal/agent`**——若发现必须改，报告 BLOCKED，那是真正的架构发现。

- [ ] **Step 2: 写下会失败的测试**

必须覆盖，与 P1 Task 7 同一套硬要求：

1. 正常一轮：学生一条、AI 一条进 `atom_message`，**seq 连续**，同一事务。
2. **模型失败一律 502 `ai_dialogue_failed`，绝不返回罐头回复**（本仓库的既定规则）。
3. **`RecentTurns` 必须显式开窗**（命名常量），lite 没有 compaction。复用 P1 的 `recentTurnsWindow` 或按写作另定并说明理由。
4. 计量：`surface="lite"`、`purpose="writing_turn"`、`atom_id` 落 `llm_call`。
5. 整轮跑在 `context.WithoutCancel(r.Context())` + 超时上（P1 Task 7 的教训：断连会让「钱花了、什么都没记下」）。

- [ ] **Step 3-6: 实现 → 跑通过 → 提交**

---

### Task W5: 大纲

**Files:**
- Create: `apps/api/internal/api/writing_outline.go`
- Test: `apps/api/internal/api/writing_outline_test.go`

**Interfaces:**
- Produces: `GET|PUT /api/v1/writings/{id}/outline`；`POST /api/v1/writings/{id}/outline/generate`

- [ ] **Step 1: 写下会失败的测试**

- `PUT` 是**全量替换**（与 pro 的 outline 语义一致）：删光再按数组顺序重插，`position` = 数组下标，`depth` 夹到 0..2。测试要能区分「全量替换」与「合并」——先 PUT 三条，再 PUT 两条，断言只剩两条。
- `POST /outline/generate` 从**学生已经说过的话**（`atom_message` 中 role='student' 的内容 + `title`）派生提纲。**这是确定性系统步骤 + 一次模型调用，不是代写正文**——它产出的是结构，不是句子。生成后**不自动覆盖**已有提纲：返回候选，由学生 `PUT` 确认。断言：generate 不改数据库。
- 若 `targetWords` 已设置，把它作为粒度信号传给模型；未设置也能生成（不设门槛）。
- 模型失败 → 502 `ai_dialogue_failed`。

- [ ] **Step 2-5: 实现 → 跑通过 → 提交**

---

### Task W6: 片段与英文示范

**Files:**
- Create: `apps/api/internal/api/writing_snippets.go`
- Test: `apps/api/internal/api/writing_snippets_test.go`

**Interfaces:**
- Produces: `GET|PUT /api/v1/writings/{id}/snippets`；`POST /api/v1/writings/{id}/snippets/{sid}/exemplar`

- [ ] **Step 1: 写下会失败的测试 —— 铁律① 在这里被守住或被打破**

- 片段的 `text` **只来自学生**。断言：任何端点都不会把模型输出写进 `writing_snippet.text`。
- `POST .../exemplar` 生成**英文示范段落**：
  - **只在 `lang='en'` 时可用**；`lang='zh'` → 400 `exemplar_not_available`。
  - 返回体里示范文本在**独立字段**（例如 `{exemplar: "..."}`），**绝不**混进 snippet 的响应。
  - **断言数据库**：调用后 `writing_snippet.text` 一字未变，且**没有任何表**存了这段示范。这是本期最重要的一条测试——它是铁律① 的机械证据。
- 引导问题以 block 形式返回（`{prompts: [...]}`），同样不进草稿表。

- [ ] **Step 2-5: 实现 → 跑通过 → 提交**

---

### Task W7: 合成、反馈、完成

**Files:**
- Create: `apps/api/internal/api/writing_compose.go`
- Test: `apps/api/internal/api/writing_compose_test.go`

**Interfaces:**
- Produces: `POST /api/v1/writings/{id}/compose`；`GET|PUT /api/v1/writings/{id}/draft`；`POST /api/v1/writings/{id}/review`；`POST /api/v1/writings/{id}/finish`

- [ ] **Step 1: 写下会失败的测试**

- **`compose` 不调模型**。断言：用一个会 panic 的 provider 注入 `Deps`，`compose` 仍然成功——若它调了模型，测试就炸。这是「只拼接、不新造一个字」的机械证据。
- `compose` 的输出 = 学生片段按 `position` 顺序拼接（段落之间空行）。断言拼出来的 `body` 里每一段都能在某个 snippet 里逐字找到。
- `compose` 后可 `PUT /draft` 继续自己改。
- `review` 给整篇反馈：**返回评语，不改 `writing_draft.body`**。断言调用后 body 一字未变。模型失败 → 502。
- `finish` 以**非空 draft** 为门槛（400 `missing_draft`），幂等，置 `status='finished'` 且 `stage='finished'`。

- [ ] **Step 2-5: 实现 → 跑通过 → 提交**

---

### Task W8: 写作前端（落地页 + 写作页）

**Files:**
- Create: `apps/lite-web/src/writings/*`
- Modify: `apps/lite-web/src/LiteApp.tsx`（写作 tab 换掉「即将上线」）
- Test: `apps/lite-web/test/writings*.test.tsx`

**要做成什么样**（与阅读落地页同一套骨架，见 P1 Task 16 的产物，直接复用它的组件）：

- 居中招呼 + **一个框**：「今天想写点什么」——直接打进去就开始，不是表单。
- **推荐题目**（不知道写什么时）、**我的写作**面板（未完成在上→继续；已完成→报告）、**提示条**（未完成数量 / 教师任务位）。
- 进去之后：**先聊**（对话），旁边显示**四阶段地图**（构思 / 大纲 / 段落 / 成稿），当前阶段高亮，**可点击跳转**（地图不是关卡）。
- 大纲阶段：生成候选 + 学生编辑 + 确认。
- 段落阶段：按提纲逐段写，引导问题以 block 出现；**英文示范在明显分离的容器里，带「示范」标识，没有任何插入按钮**。
- 成稿阶段：合成 → 学生可继续改 → 要反馈。
- 复用 `apps/web` 的组件与 `mk-*` token；**不新造色板**。

- [ ] Steps: 实现 → `pnpm --filter @mind-imprint/lite-web test && typecheck && build` → **真实浏览器走一遍并截图** → 提交

---

### Task W9: 写作端到端走查

**Files:** `apps/lite-web/e2e/writing-walk.spec.ts`

一条完整走查：打开写作 → 把一个念头打进框 → AI 回应 → 设定篇幅 → 生成并确认提纲 → 写两段片段 → 合成 → 要反馈 → 完成。

断言中必须包含**两条铁律的机械证据**：

1. 合成出来的正文里，**每一段都能在学生输入过的文字里找到**——没有凭空出现的句子。
2. 英文写作时示范段落**存在于页面上**，但**不在草稿框里**，且**没有任何按钮能把它插进去**。

```bash
pnpm --filter @mind-imprint/lite-web exec playwright test -c e2e/playwright.config.ts
```

---

## 自检（写完计划后对照 spec）

| Spec §6.2 条目 | 落在哪个任务 |
|---|---|
| 一个框进去，直接写想法 | W2（`idea` 建原子并成为第一条消息） |
| 进去先聊 | W4 |
| 四阶段 构思/大纲/段落/成稿 | W3（状态与留痕）、W8（地图 UI） |
| 构思敲定篇幅 | W3（`target-words`） |
| 大纲派生 + 学生可改 | W5 |
| 引导式片段写作 | W6 |
| 英文示范（永不进草稿） | W6（服务端证据）、W9（页面证据） |
| 合成全文（只拼接） | W7（panic-provider 证据） |
| AI 反馈 | W7 |
| 推荐题目 / 历史 / 未完成 / 教师任务位 | W8 |
| 简版报告 | **本期不做** —— 与阅读报告同属 P2 的 `atom_report`，写作接入随其落地 |
| 阶段是地图不是关卡 + 跳过留痕 | W3 |

**必须在实现时亲手核实的外部名字**：当前最高迁移号（W1）；`internal/agent` 里可复用的写作/陪练入口的真实签名（W4 Step 1）；pro 已占用的 handler 名（各任务动手前 grep）；P1 Task 16 产出的落地页组件名（W8）。
