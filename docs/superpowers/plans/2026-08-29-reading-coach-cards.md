# 带读的卡片 + lite 阅读房间分家 · Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 让 印记 在带读回复里递出**它现场撰写、材料取自文章**的互动卡片，学生用点/选/指回答；并在此之前把 lite 的阅读房间从 pro 的共享组件里分出来。

**Architecture:** 先做一次纯搬迁的 fork（lite 拥有自己的 `ReadingRoom.tsx`，继续 import 共享叶子组件），再在 lite 自己的房间里加卡片。卡片随 `atom_message.payload jsonb` 落库，随 `ChatMessage.node` 渲染，靠字面子串校验器保证选项真的来自文章。

**Tech Stack:** Go 1.22+（`net/http` + pgx + sqlc + goose）、PostgreSQL、React 18 + Vite + TypeScript + Tailwind、vitest、Playwright。

**Spec:** `docs/superpowers/specs/2026-08-29-reading-coach-cards-design.md`

## Global Constraints

每个任务的要求都隐含包含这一节。

1. 🚨 **`apps/web/` 下面不允许有任何文件被修改。** 这是本期唯一可机器判定的 pro 安全保证。任务结束前跑：
   ```bash
   git diff --name-only <task-base>..HEAD | grep '^apps/web/' && echo "VIOLATION" || echo "ok"
   ```
   lite 通过 `@/` **import** 共享组件（vite alias + tsconfig paths 已配好），新样式写进 lite 自己的样式表。`packages/contracts` 同理：契约没真的变就别动。
2. 🚨 **卡片选项进入 transcript 时，引用文章的每一行都要带 `> ` 前缀。** 不是风格问题：`hasHuntPickEvidence`（`reading_coach.go:202-236`）和 `report_facts.go` 的 `stripQuotedLines` 都靠这个前缀工作。这条地雷 2026-08-29 已经炸过一次（多行引用只有第一行带前缀，文章原文漏进「她自己的话」语料）。
3. 🚨 **不要在 coach system prompt 里新增字面量 `%s`。** `buildReadingCoachSystem`（`reading_coach.go:383-387`）用的是 `strings.Replace(..., "%s", ..., 1)`，不是 `Sprintf`；新增的 `%s` 会被第一次替换悄悄吃掉。段落工具菜单用 `%s`、透镜菜单用 `%LENS%`，正是为此才分成两个 token。
4. 🚨 **不要把聊天卡片写进 `atom_card`。** `atom_card_one_open_idx`（`0096`）是 `(atom_id) WHERE status IN ('proposed','active')` 的唯一偏索引，会和透镜召唤互相堵死；而且 `card_id` 必须在 Go 的 `cards.ByID` 和 TS 的 `CARD_REGISTRY` 两边都解析得到。
5. **永远不渲染对/错**：没有 ✓/✗、没有分数、没有正确答案、没有连胜（铁律②）。卡片本身**不带 answer key**。
6. **Go 测试**：白盒测试（要看未导出符号）必须放在 `*_internal_test.go` 且 `package api`；`internal/api` 下已有的 reading/writing 测试文件都是 `package api_test`，看不到未导出符号。
7. **测试命令**：Go 用 `cd apps/api && CGO_ENABLED=0 go test ./internal/api/... -timeout 1800s`；sqlc 重新生成用 `cd apps/api && make sqlc`（内含 `CGO_ENABLED=0`）。前端 `cd apps/lite-web && pnpm test` / `pnpm typecheck`。
8. **Tailwind `mk-*` 是裸 CSS 变量**：所有透明度语法（`bg-mk-x/40`、`text-mk-ink/70`）都不出 CSS。要半透明必须用 `color-mix(in srgb, var(--mk-accent-500) 40%, transparent)`。
9. **每个动画都要带 `prefers-reduced-motion` 关闭开关**（或 `motion-reduce:` 工具类）。
10. **不要用 `git stash`**：stash 栈是跨 worktree 共享的，可能弹掉别人的改动。要暂存就用临时 WIP commit。

---

## File Structure

**第 0 步（分家）**
- Create: `apps/lite-web/src/readings/ReadingRoom.tsx` —— lite 自己的阅读房间，从 pro 版复制后删减；组合 `@/primitives/annotate` 的 `Annotate`、`@/studio/reading/HangingCard`、`LensLibrary`、`ReadingOutcomes`、`FinalizeReadingPanel`、`useReadingLoop`。
- Modify: `apps/lite-web/src/readings/ReadingRoomHost.tsx` —— 改成 import 本地房间；拆掉 `renderCoach`/`renderBlockAside`/`onBlockPick` 的转接；不再传 `projectId`/`referenceId`。

**第 1 步（后端卡片）**
- Create: `apps/api/internal/store/migrations/0106_coach_card_payload.sql`
- Modify: `apps/api/internal/store/queries/atom.sql`（`payload` 进出）
- Modify: `apps/api/internal/api/reading_coach.go`（prompt 输出格式段、`readingCoachReply`、`parseReadingCoachReply`、合成学生消息、响应 map）
- Create: `apps/api/internal/api/reading_coach_card.go` —— 卡片类型、校验器、`> ` 前缀合成
- Create: `apps/api/internal/api/reading_coach_card_internal_test.go`

**第 2 步（lite 前端）**
- Create: `apps/lite-web/src/readings/CoachCard.tsx`
- Create: `apps/lite-web/src/readings/StepIndicator.tsx`
- Modify: `apps/lite-web/src/readings/ReadingCoachPanel.tsx`、`apps/lite-web/src/api/readingRoom.ts`、`apps/lite-web/src/index.css`
- Modify: `apps/api/internal/api/reading_routines.go`（改掉被推翻的注释）
- Create: `apps/lite-web/e2e/card-walk.spec.ts`

---

## Task 1: lite 拥有自己的阅读房间（纯搬迁）

**Files:**
- Create: `apps/lite-web/src/readings/ReadingRoom.tsx`
- Modify: `apps/lite-web/src/readings/ReadingRoomHost.tsx`
- Read first (不要修改): `apps/web/src/studio/reading/ReadingRoom.tsx`

**Interfaces:**
- Produces: `LiteReadingRoom`（默认导出名 `ReadingRoom`），props 见下；后续任务在这个文件里加当前步指示器。
- Consumes: 共享叶子组件，全部通过 `@/` 别名 import。

**这一步是纯搬迁。** 学生看到的东西必须一模一样。任何「顺手改进」都留到后面的任务。

- [ ] **Step 1: 通读 pro 的房间，列出 lite 实际用到的部分**

  读 `apps/web/src/studio/reading/ReadingRoom.tsx`（1096 行）和 `apps/lite-web/src/readings/ReadingRoomHost.tsx`。把 props 分成三类写进任务报告：lite 传了并且真的有用的、lite 传了但只是敷衍 pro 接口的（`projectId`/`referenceId`/`phaseTag`）、lite 从不传的。

- [ ] **Step 2: 建 lite 自己的房间文件**

  复制 pro 的 `ReadingRoom.tsx` 到 `apps/lite-web/src/readings/ReadingRoom.tsx`，然后**删掉 lite 用不到的东西**：

  - brief bar（pro 的 `:558-596`，「你读这篇是为了」）
  - 阶段下拉框（pro 的 `:597-616`）
  - 证据笔记区块（`ReferenceEvidence` / `EvidenceNote` 相关）
  - `TraceSourcePanel` 与「追来源」按钮
  - pro 自己的 chat log、composer、starter row、quote chips —— lite 的对话来自 `ReadingCoachPanel`

  **`caps` / `RoomCapabilities` 整套删掉——但必须穷尽。** `LITE_READING_CAPABILITIES`
  （`apps/web/src/rooms/capabilities.ts:47-58`）有**九个**字段，不是两个。删掉 caps
  等于把每一个都写死成 lite 的取值。**逐条对照这张表做，别凭印象**：

  | flag | lite 取值 | 在 lite 房间里的做法 |
  |---|---|---|
  | `mode` | `"lite"` | 「返回」文案写死（pro 是「返回工作区」） |
  | `plan` | `false` | 删掉相关分支 |
  | `evidenceMap` | `false` | 删掉证据笔记 |
  | `explorationLeads` | `false` | 删掉相关分支 |
  | `proposalImpact` | `false` | 删掉相关分支 |
  | `essayTrack` | `false` | 删掉相关分支 |
  | `comprehensionCheck` | `true` | **今天没有任何代码消费它**——不要为它加东西 |
  | `exemplars` | `false` | 删掉相关分支 |
  | `credibility` | `false` | 删掉相关分支（追来源 / 可信度） |

  做法：在 pro 文件里 `grep -n "caps\."` 把每一处都找出来，逐处按上表判定「保留哪一边」。
  ⚠️ 漏掉一处 `false` 的分支 = lite 学生突然看见一个 pro 专属界面；漏掉一处 `true` =
  功能消失。**任务报告里要列出你处理过的每一处 `caps.`**，这是这一步唯一的完整性证据。

  **把三个 slot 内联掉**：`renderCoach` 的位置直接渲染 `ReadingCoachPanel`，`renderBlockAside` 的位置直接渲染 `BlockToolsPanel`，`onBlockPick` 直接调本地回调。这三个 slot 存在的唯一理由就是当时不想 fork。

  **继续 import 的共享叶子**（一个都不要复制过来）：
  ```tsx
  import { Annotate } from "@/primitives/annotate";
  import { HangingCard, type HangingCardStatus, anchorBlockId } from "@/studio/reading/HangingCard";
  import { ConfirmedFindingCard } from "@/studio/reading/ConfirmedFindingCard";
  import { LensLibrary } from "@/studio/reading/LensLibrary";
  import { ReadingOutcomes } from "@/studio/reading/ReadingOutcomes";
  import { FinalizeReadingPanel } from "@/studio/reading/FinalizeReadingPanel";
  import { useReadingLoop } from "@/studio/reading/readingLoop";
  import "@/studio/reading/ReadingRoom.css";
  ```

  props 收敛成 lite 真正需要的形状，**不再有 `projectId` / `referenceId`**：
  ```tsx
  export function ReadingRoom({
    readingId,
    source,
    anchors,
    api,
    onBack,
    onFinalized,
  }: LiteReadingRoomProps) { … }
  ```
  `useReadingLoop` 目前仍是 pro 形状（吃 projectId/referenceId），**这一步不要改它**：在房间内部把 `readingId` 传两次即可，并留一行注释说明这是下一步要收拾的债。

- [ ] **Step 3: 让 Host 用本地房间**

  `ReadingRoomHost.tsx`：把 `import { ReadingRoom } from "@/studio/reading/ReadingRoom"` 换成 `./ReadingRoom`，删掉三个 slot 的转接代码和编造的 props，直接把 `ReadingCoachPanel` / `BlockToolsPanel` 需要的状态往下传。

- [ ] **Step 4: 类型检查 + 单元测试**

  ```bash
  cd apps/lite-web && pnpm typecheck && pnpm test
  ```
  Expected: 全绿。`apps/lite-web/test/readingRoomHost.test.tsx` 若因 import 路径变化而失败，**只改测试里的路径/mock，不要改变断言**——断言是「行为没变」的证据。

- [ ] **Step 5: 证明 pro 一个字没改**

  ```bash
  cd apps/web && pnpm test -- ReadingRoom
  git diff --name-only HEAD~1..HEAD | grep '^apps/web/' && echo "VIOLATION" || echo "ok — apps/web untouched"
  ```
  Expected: pro 测试全绿，且第二条打印 `ok`。

- [ ] **Step 6: Commit**

  ```bash
  git add apps/lite-web/src/readings/ReadingRoom.tsx apps/lite-web/src/readings/ReadingRoomHost.tsx
  git commit -m "refactor(lite): the reading room moves into lite, pro's is untouched"
  ```

---

## Task 2: 分家的行为守恒验证（e2e）

**Files:**
- Run only: `apps/lite-web/e2e/reading-walk.spec.ts`、`coach-walk.spec.ts`、`report-walk.spec.ts`

**Interfaces:**
- Consumes: Task 1 的 lite 房间。
- Produces: 「分家没有改变任何学生可见行为」这一结论。

- [ ] **Step 1: 起本地栈并跑三个走查**

  ```bash
  cd apps/lite-web && ./e2e/run-stack.sh
  ```
  （`run-stack.sh` 负责 Postgres + API + lite dev server；配置见 `e2e/playwright.config.ts`。）

- [ ] **Step 2: 逐条核对失败**

  任何失败都必须归到两类之一，并写进任务报告：
  - **搬迁漏了东西** → 回 Task 1 补上。
  - **测试断言的是 pro 特有的东西**（例如断言 brief bar 存在）→ 改测试，并在 commit message 里说明为什么这条断言在分家后不再成立。

  ⚠️ 不要用「行为本来就该变」来解释失败——这一步的全部意义就是行为**没有**变。

- [ ] **Step 3: Commit（若有测试改动）**

  ```bash
  git commit -am "test(lite): the walks follow the reading room into lite"
  ```

---

## Task 3: `atom_message.payload jsonb`

**Files:**
- Create: `apps/api/internal/store/migrations/0106_coach_card_payload.sql`
- Modify: `apps/api/internal/store/queries/atom.sql`

**Interfaces:**
- Produces: `atom_message.payload`（jsonb，可空）；`ListAtomMessages` / 插入消息的查询都带上它。
- Consumes: 无。

- [ ] **Step 1: 写迁移**

  ```sql
  -- +goose Up
  -- 聊天卡片挂在提出它的那条消息上（AI 侧是卡片本身，学生侧是她的回答）。
  --
  -- 为什么不用 atom_card：card_id 必须在 Go 的 cards.ByID 与 TS 的 CARD_REGISTRY
  -- 两边都解析得到，而 印记 现场写的卡在两边都没有家；更要命的是
  -- atom_card_one_open_idx（0096）是 (atom_id) WHERE status IN ('proposed','active')
  -- 的唯一偏索引，聊天卡片一旦以 open 状态落进去，就会把这篇文章的透镜召唤全堵死。
  ALTER TABLE atom_message ADD COLUMN payload jsonb;

  -- +goose Down
  ALTER TABLE atom_message DROP COLUMN payload;
  ```

- [ ] **Step 2: 更新 queries**

  `apps/api/internal/store/queries/atom.sql`：`ListAtomMessages` 的 SELECT 列表加上 `payload`；插入消息的查询加一个 `payload` 参数（AI 侧写卡片，学生侧写她的回答，其余传 NULL）。
  ⚠️ `ListAtomMessages` 现有的 `block_id IS NULL` 过滤**保持不变**（`queries/atom.sql:33-39` 的注释明确禁止加一个省略它的变体）。

- [ ] **Step 3: 重新生成 sqlc 并编译**

  ```bash
  cd apps/api && make sqlc && go build ./...
  ```
  Expected: 成功。注意 sqlc 配置了 `emit_pointers_for_null_types`，可空列生成为 Go 指针。

- [ ] **Step 4: 写一个真的会执行 SQL 的测试**

  🚨 `go build` 只检查 Go，不检查 SQL。上一期就有一条 `CASE WHEN $1 IS NULL` 通过了编译却在运行时 SQLSTATE 42P08。新增/改动的每个查询都要有一个**真的执行它**的测试（testcontainers）：写一条带 payload 的消息、读回来、断言 round-trip 一致，并断言 payload 为 NULL 的旧行读出来是 nil。

- [ ] **Step 5: 跑测试**

  ```bash
  cd apps/api && CGO_ENABLED=0 go test ./internal/api/... -timeout 1800s
  ```

- [ ] **Step 6: Commit**

  ```bash
  git commit -am "feat(api): a chat message can carry a payload"
  ```

---

## Task 4: 卡片类型与校验器

**Files:**
- Create: `apps/api/internal/api/reading_coach_card.go`
- Create: `apps/api/internal/api/reading_coach_card_internal_test.go`

**Interfaces:**
- Produces:
  ```go
  type coachCardOption struct {
      BlockID string `json:"blockId"`
      Quote   string `json:"quote"`
  }
  type coachCard struct {
      Type    string            `json:"type"`    // choose_span | pick_in_article | short_text
      Prompt  string            `json:"prompt"`
      Options []coachCardOption `json:"options"` // choose_span 专用
  }
  // 不合格返回 nil —— 静默丢弃，不报错、不渲染残卡。
  func validateCoachCard(c *coachCard, blocks []Block) *coachCard
  ```
- Consumes: `Block`（`reading_coach.go` 已在用）。

- [ ] **Step 1: 先写失败的测试**

  照抄 `validateReadingPicks`（`reading_coach.go:156-174`）与 `validateReadingQuestions`（`reading_questions.go:167-184`）的既有写法。文件必须是 `package api`（白盒）。

  ```go
  func TestValidateCoachCard(t *testing.T) {
      blocks := []Block{
          {ID: "b1", Text: "城市地表以沥青和混凝土为主，白天吸热、夜里放热。"},
          {ID: "b2", Text: "建筑密集阻碍了夜间散热。空调外机把热量排到室外。"},
      }

      t.Run("编造的句子整张卡片被丢掉", func(t *testing.T) {
          got := validateCoachCard(&coachCard{
              Type:   "choose_span",
              Prompt: "哪一句你读着最不服气？",
              Options: []coachCardOption{
                  {BlockID: "b1", Quote: "城市地表以沥青和混凝土为主"},
                  {BlockID: "b2", Quote: "这句话文章里根本没有"},
              },
          }, blocks)
          // 只剩 1 个合格选项 < 2 → 整张丢掉
          if got != nil {
              t.Fatalf("expected nil, got %+v", got)
          }
      })

      t.Run("选项必须是它自己那个 block 的子串", func(t *testing.T) {
          got := validateCoachCard(&coachCard{
              Type:   "choose_span",
              Prompt: "挑一句",
              Options: []coachCardOption{
                  // b1 的句子挂在 b2 上 —— 不算
                  {BlockID: "b2", Quote: "城市地表以沥青和混凝土为主"},
                  {BlockID: "b2", Quote: "建筑密集阻碍了夜间散热"},
              },
          }, blocks)
          if got != nil {
              t.Fatalf("expected nil, got %+v", got)
          }
      })

      t.Run("合格的卡片原样通过", func(t *testing.T) {
          got := validateCoachCard(&coachCard{
              Type:   "choose_span",
              Prompt: "哪一句你读着最不服气？",
              Options: []coachCardOption{
                  {BlockID: "b1", Quote: "白天吸热、夜里放热"},
                  {BlockID: "b2", Quote: "空调外机把热量排到室外"},
              },
          }, blocks)
          if got == nil || len(got.Options) != 2 {
              t.Fatalf("expected 2 options, got %+v", got)
          }
      })

      t.Run("超过 4 个选项截断到 4", func(t *testing.T) { /* … */ })
      t.Run("重复选项去重", func(t *testing.T) { /* … */ })
      t.Run("未知类型丢掉", func(t *testing.T) { /* … */ })
      t.Run("prompt 超长丢掉", func(t *testing.T) { /* … */ })
      t.Run("pick_in_article 不需要 options", func(t *testing.T) { /* … */ })
      t.Run("short_text 不需要 options", func(t *testing.T) { /* … */ })
  }
  ```

- [ ] **Step 2: 跑测试确认失败**

  ```bash
  cd apps/api && CGO_ENABLED=0 go test ./internal/api/ -run TestValidateCoachCard -timeout 300s
  ```
  Expected: 编译失败（`validateCoachCard` 未定义）。

- [ ] **Step 3: 实现**

  规则：类型必须在 {`choose_span`,`pick_in_article`,`short_text`} 内；`Prompt` 非空且 ≤ 60 字（runes，不是 bytes）；`choose_span` 的每个 option 的 `Quote` 必须 `strings.Contains(blockText[BlockID], Quote)`，去重、去空、截断到 4 个，**存活 < 2 → 返回 nil**；`pick_in_article` / `short_text` 忽略 options。

- [ ] **Step 4: 跑测试确认通过**

- [ ] **Step 5: Commit**

  ```bash
  git commit -am "feat(api): a coach card whose options aren't really in the article is dropped"
  ```

---

## Task 4b: 选项必须是「一句话」，不是一段字符

**来源：** Task 4 评审的 Important 缺口。完整 brief 见
`.superpowers/sdd/2026-08-29-reading-coach-cards/briefs/task-4b-brief.md`。

Task 4 的 ≥4 runes 下限管的是**长度**，不是**「是不是一句话」**。
`表以沥青和混凝土`（从词中间切）和同一句话的多个重叠片段
（`白天吸热、夜里放热` / `吸热、夜里` / `夜里放热`，去重按可见文本所以合并不掉）
今天全部能通过。片段可以靠扫几个字凑出来，一句话必须整体读过——这正是校验器
存在的理由。

**加一条从句边界规则**：quote 的开头必须在 block 开头或紧跟一个边界字符，
结尾必须在 block 结尾、紧接一个边界字符、或自己以边界字符收尾。
边界字符：`，。！？；：、\n` 及其半角形式。失败就丢，朝安全方向失败。

🚨 **Task 4 原有的 15 个子测试一个都不许改。** 新规则弄红了哪条，就说明规则收得
太紧——回去改规则。收太紧会让卡片静悄悄消失，看起来像「模型今天不想出卡片」，
比放太松更难发现。必须有正例测试：正常完整句子（带/不带尾部句号）、block 首句、
block 末句、以 `，` 分隔的从句，全都要能通过。

---

## Task 5: 把卡片接进 coach turn

**Files:**
- Modify: `apps/api/internal/api/reading_coach.go`
- Modify: `apps/api/internal/api/reading_coach_card_internal_test.go`

**Interfaces:**
- Consumes: Task 4 的 `validateCoachCard`。
- Produces: `readingCoachReply.Card *coachCard`；HTTP 响应 map 里的 `card` 键。

- [ ] **Step 1: 扩 reply 结构**

  `readingCoachReply`（`reading_coach.go:401-412`）加：
  ```go
  Card *coachCard `json:"card"`
  ```

- [ ] **Step 2: 改 prompt 的输出格式段**

  `readingCoachSystem`（`reading_coach.go:77-88`）。🚨 **不要写字面量 `%s`**（Global Constraint 3）。要写清楚：

  - `card` 是可选的，不需要就整个省略。
  - `choose_span` 的每个选项**必须逐字抄自文章**，不许改写、不许缩写。
  - **问题要问她的判断，不能有唯一正解**：「哪一句你读着最不服气」而不是「哪一句是作者的结论」。这是产品裁定，不是文风偏好。
  - 卡片承担步骤指令，`reply` 因此只说对她刚做的事的真实回应，**不要复述 label/detail**。

- [ ] **Step 3: 接进 parser**

  `parseReadingCoachReply`（`:414-468`）在 `return got, true` 之前调 `validateCoachCard`，不合格就 `got.Card = nil`。**turn 本身照常成功**——卡片不合格不是错误。

- [ ] **Step 4: 写进响应 map**

  `postReadingCoachTurn`（`:701-713`）的 map 加 `card`（nil 就不加这个键，和现有 `card`/`nudge` 的写法一致——⚠️ 注意那里已经有一个表示透镜卡的 `card` 键，**换一个不冲突的键名**，例如 `coachCard`，并在 TS 侧同名）。

- [ ] **Step 5: 落库**

  AI 消息行写入时，把卡片 JSON 放进 Task 3 的 `payload`。

- [ ] **Step 6: 测试**

  加一个 parser 测试：模型输出带一张编造句子的卡片 → `Reply` 保留、`Card` 为 nil、turn 成功。再加一个：合格卡片能从模型 JSON 一路走到响应 map。

- [ ] **Step 7: 跑全套 + Commit**

  ```bash
  cd apps/api && CGO_ENABLED=0 go test ./internal/api/... -timeout 1800s
  git commit -am "feat(api): 印记 can hand her a card inside a coach turn"
  ```

---

## Task 6: 她点完之后，服务端合成一条学生消息

**Files:**
- Modify: `apps/api/internal/api/reading_coach.go`
- Modify: `apps/api/internal/api/reading_coach_card_internal_test.go`

**Interfaces:**
- Consumes: 请求体新增 `cardAnswer`。
- Produces: 一条 `role='student'` 的 `atom_message`，内容里**每一行引用都带 `> `**，`payload` 存结构化答案。

**这是整个计划里最危险的一个任务**（Global Constraint 2）。

- [ ] **Step 1: 扩请求体**

  `reading_coach.go:491-494` 的 `struct { Text string; Picks []readingPick }` 加：
  ```go
  CardAnswer *struct {
      Type   string `json:"type"`
      Prompt string `json:"prompt"`
      Choice string `json:"choice"`   // 她选中的原句，或她写的一句话
      BlockID string `json:"blockId"`
  } `json:"cardAnswer"`
  ```

- [ ] **Step 2: 先写那条会炸的测试**

  ```go
  func TestCardAnswerQuotesEveryLine(t *testing.T) {
      // 一段硬换行的原文 —— SplitBlocks 只按空行切，所以这是「一个 block 里带换行」
      quote := "城市地表以沥青和混凝土为主，\n白天吸热、夜里放热。"
      got := composeCardAnswerMessage("哪一句你读着最不服气？", quote, "")
      for _, line := range strings.Split(quote, "\n") {
          if !strings.Contains(got, "> "+line) {
              t.Fatalf("line not quoted: %q\nfull message:\n%s", line, got)
          }
      }
  }
  ```
  ⚠️ 这条测试存在的理由：2026-08-29 报告那次，`ReadingCoachPanel` 只给引用的**第一行**加了 `> `，多行引用的后几行进了「她自己的话」语料，可以印在她署名的图片上。卡片选项就是文章原句，是同一颗地雷的第二次机会。

- [ ] **Step 3: 实现 `composeCardAnswerMessage`**

  逐行加前缀（`strings.Split(quote, "\n")`），再拼上她自己的话（如果有）。

- [ ] **Step 4: 接进 turn**

  收到 `cardAnswer` 时合成学生消息并落库（`payload` 存结构化答案），这样：
  - 下一轮的 `buildReadingCoachPrompt`（`:238-347`）看得见她答了什么；
  - `hasHuntPickEvidence`（`:202-236`）能正确识别她「指」了；
  - 刷新之后 transcript 是完整的。

  🚨 现在 `studentText == ""` 时**根本不写学生消息行**（`:620-628`）——纯点击的回答必须走新路径，不能沿用这个分支。

- [ ] **Step 5: 测试 + Commit**

  ```bash
  cd apps/api && CGO_ENABLED=0 go test ./internal/api/... -timeout 1800s
  git commit -am "feat(api): a tapped answer becomes a real turn, quoted line by line"
  ```

---

## Task 7: `CoachCard.tsx`

**Files:**
- Create: `apps/lite-web/src/readings/CoachCard.tsx`
- Create: `apps/lite-web/test/coachCard.test.tsx`
- Modify: `apps/lite-web/src/index.css`

**Interfaces:**
- Produces: `<CoachCard card={…} onAnswer={(a) => …} answered={…} />`
- Consumes: Task 5 的线上类型。

- [ ] **Step 1: 先写测试**

  三种类型各渲染一次；断言 `choose_span` 渲染出全部选项且点击回调带回原句；断言**页面上没有 ✓/✗/「正确」/「答对」字样**；断言已作答的卡片显示她的选择但不再可点。

- [ ] **Step 2: 实现**

  不要用 `@/cards/CardRenderer`（`CardSpec` 的 Zod 要求 `id/category/name/purpose/trigger_condition/steps/rubric_tags` 全齐，为了三个选项去编这些字段不值得）。写 lite 自己的窄组件。

  样式仿 `.mk-blockbar`（`apps/lite-web/src/index.css:225-249`）：130ms、`cubic-bezier(0.2, 0, 0, 1)`、`translateY(4px) scale(0.97)` → none，带 `prefers-reduced-motion`。半透明一律 `color-mix`（Global Constraint 8）。

- [ ] **Step 3: 跑测试 + Commit**

---

## Task 8: 把卡片接进对话

**Files:**
- Modify: `apps/lite-web/src/readings/ReadingCoachPanel.tsx`
- Modify: `apps/lite-web/src/api/readingRoom.ts`
- Modify: `apps/lite-web/test/readingRoomHost.test.tsx`（如受影响）

**Interfaces:**
- Consumes: `CoachCard`、`postReadingCoachTurn`。

- [ ] **Step 1: 线上类型**

  `postReadingCoachTurn`（`readingRoom.ts:372-413`）加 `coachCard` 的解析和 `cardAnswer` 的发送。

- [ ] **Step 2: 渲染进 ChatLog**

  走 `ChatMessage.node`（`@/studio/ai/ChatLog.tsx:22-29`）——`node` 是 `ReactNode`，pro 的四个房间已经用它塞 `CardTurnChip`，**共享代码一行不用改**。本地消息类型放宽成 `LiteMessage | CoachCardMessage`。

- [ ] **Step 3: 作答即发一轮**

  点选项 → `postReadingCoachTurn(id, "", [], cardAnswer)`。
  ⚠️ `send()` 现有的「什么都没有就不发」守卫（`:115-142`）不认识卡片，必须一起改，否则纯点击的回答会被自己的守卫拦下。

- [ ] **Step 4: 滚动**

  ⚠️ `ChatLog` 只在 `messages.length` 变化时自动滚（`ChatLog.tsx:55-60`），卡片是**原地长高**的（选中、反馈展开），条数不变 → 不会滚。显式处理。

- [ ] **Step 5: 刷新后仍在**

  `initialMessages` 要把带 `payload` 的消息还原成卡片。加一条测试。

- [ ] **Step 6: 测试 + Commit**

---

## Task 9: 当前步指示器

**Files:**
- Create: `apps/lite-web/src/readings/StepIndicator.tsx`
- Modify: `apps/lite-web/src/readings/ReadingRoom.tsx`（Task 1 建的那个）
- Create: `apps/lite-web/test/stepIndicator.test.tsx`

- [ ] **Step 1: 测试**

  当前步 = `tasks.find(t => t.status === "pending")`（和服务端 `currentReadingTask`，`reading_coach.go:392-399`，同一条规则）。断言渲染出「第 N 步 / 共 M 步」和当前步的 label；断言全部完成时不再显示待办步骤。

- [ ] **Step 2: 实现 + 动效**

  放在 Task 1 里 brief bar 原来的位置（那段 JSX 压根没被抄过来，位置是空的）。动效仿 `.mk-blockbar`，必带 `prefers-reduced-motion`。
  ⚠️ 不要引入进度百分比之外的任何计量（铁律②：不要分数、不要连胜）。

- [ ] **Step 3: 窄屏**

  ⚠️ `ReadingPlanRail` 挂在 `lg:` 才显示的侧栏里（`ReadingRoomHost.tsx:283`），所以这是**小屏上的第一个步骤界面**，不是搬个位置。确认小屏下它可见。

- [ ] **Step 4: 测试 + Commit**

---

## Task 10: 改掉被推翻的注释

**Files:**
- Modify: `apps/api/internal/api/reading_routines.go:14-26`

- [ ] **Step 1: 重写那段注释**

  照 spec 的「裁定二」改，**并且严格保留它的边界**：

  | 旧理由 | 结局 |
  |---|---|
  | 1. 必须通用 | 对「有哪些步骤」仍然成立（routine 库继续写死）；对「步骤里 印记 递给她什么」不再成立——那是脚手架。 |
  | 2. 必须稳定 | 完全活着，由**固定卡片类型**承担。 |
  | 3. 必须便宜且诚实 | 完全活着：routine 挑选仍是确定性的。 |

  引用产品负责人的原话（*"AI provides scaffolding. you are banning scaffolding."*）和日期，这样下一个读到的人知道这是一次有意的推翻，不是漂移。

- [ ] **Step 2: Commit**

  ```bash
  git commit -am "docs(api): scaffolding is the teacher's job, not a violation"
  ```

---

## Task 11: 印记 说话可以有结构（markdown + 强调色）

**Files:**
- Create: `apps/lite-web/src/readings/LiteChatMarkdown.tsx`
- Modify: `apps/lite-web/src/readings/ReadingCoachPanel.tsx`
- Modify: `apps/api/internal/api/reading_coach.go`（system prompt）
- Create: `apps/lite-web/test/liteChatMarkdown.test.tsx`

**Interfaces:**
- Produces: lite 自己的聊天 markdown 渲染器。
- Consumes: 无。

**先搞清楚现状**：markdown **已经在渲染了**——`ReadingCoachPanel.tsx:144-152` 把每条 AI
消息交给 `@/studio/ai/ChatMarkdown`，它支持加粗、无序/有序列表、标题、引用、行内代码、
链接。产品负责人看到的之所以是一段平铺的散文，是因为**模型没用 markdown**，不是渲染不了。

所以这个任务只有两件事。

- [ ] **Step 1: lite 自己的渲染器**

  🚨 **不要改 `apps/web/src/studio/ai/ChatMarkdown.tsx`**（Global Constraint 1）——它是
  pro 四个房间共用的，改了强调色等于改掉 pro 每一个聊天气泡。

  新建 `apps/lite-web/src/readings/LiteChatMarkdown.tsx`，照抄那份 `components` 映射，
  **只改 `strong`**：

  ```tsx
  strong: ({ children }) => (
    <strong className="font-bold" style={{ color: "var(--mk-accent-600)" }}>{children}</strong>
  ),
  ```
  ⚠️ 用 inline style 带 CSS 变量，别用 Tailwind 的透明度语法（Global Constraint 8）。
  具体用哪个 accent token 以 `apps/lite-web/src/index.css` 里实际定义的为准——**先去
  确认它存在**，不要凭猜写一个变量名。

  `ReadingCoachPanel` 改用它。学生那一侧**继续是纯文本**（`ChatMarkdown` 的文件注释
  写得很清楚：学生打的 `*` 或 `#` 绝不能被当成标记吃掉）。

- [ ] **Step 2: 测试**

  断言 `**粗体**` 渲染成 `<strong>` 且带的是强调色而不是 ink；断言 `- a\n- b` 渲染成
  `<ul><li>`；断言学生消息里的 `**x**` **原样显示**，没有被解析。

- [ ] **Step 3: prompt 里允许结构**

  `readingCoachSystem` 加一条，措辞要克制：

  - 可以用 `**加粗**` 点出**一个**关键词，用短列表并列两三个选项。
  - **不要为了好看而加结构**：一句话说得清就一句话。
  - 标题基本用不上——这是对话，不是文档。
  - 🚨 **和「一次只问一个」（铁律③）不冲突**：列表可以并列几个**选项**，但问题仍然
    只能有一个。不要用列表塞三个问题进去。

  ⚠️ 卡片承担步骤指令之后回复本来就该更短（Task 5 Step 2），这条不要把它又撑回去。

- [ ] **Step 4: 测试 + Commit**

  ```bash
  cd apps/lite-web && pnpm typecheck && pnpm test
  cd apps/api && CGO_ENABLED=0 go test ./internal/api/... -timeout 1800s
  git commit -am "feat(lite): 印记 can put a word in bold and two options in a list"
  ```

---

## Task 12: e2e 走查

**Files:**
- Create: `apps/lite-web/e2e/card-walk.spec.ts`

- [ ] **Step 1: 写走查**

  贴一篇四段中文文章 → 开始带读 → 断言第一条 印记 回复带卡片 → 断言每个选项都能在文章里逐字找到 → **只用点击**作答 → 断言 印记 下一句回应的是她选的那一项 → 刷新 → 断言卡片和她的回答都还在 → 断言全程没有 ✓/✗/分数 → 断言当前步指示器在、「你读这篇是为了」不在。

  最后一条：**在有聊天卡片的情况下召唤一次透镜**，断言成功（证明没踩 `atom_card_one_open_idx`）。

- [ ] **Step 2: 跑 + Commit**

---

## Task 13: 收尾验证

- [ ] **Step 1: 三个全套**

  ```bash
  cd apps/api && CGO_ENABLED=0 go test ./internal/api/... -timeout 1800s
  cd apps/lite-web && pnpm typecheck && pnpm test
  cd apps/web && pnpm test
  ```

- [ ] **Step 2: 机器判定 pro 安全**

  ```bash
  git diff --name-only <plan-base>..HEAD | grep '^apps/web/' && echo "VIOLATION" || echo "ok — apps/web untouched"
  ```
  Expected: `ok`。**这一条不通过就不算完成**，不管其他测试多绿。

- [ ] **Step 3: 真的看一眼**

  ⚠️ 上一期的教训：14 个任务评审 + 全branch评审全过之后，两个只有截图能发现的缺陷还在（四个 0 的统计磁贴、用第三人称称呼她）。**验收标准是视觉的时候，绿色的测试不是证据。** 渲染一次真实的带读对话，看那张卡片，确认它不像考试题。
