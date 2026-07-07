# E2E Audit + Polish Sub-tasks — 2026-07-07

> Live full-stack smoke of the completed 5-slice refactor. Stack: web (`:5174`) → Go API (`:8080`) → throwaway Postgres (migrated to `0013`, seeded) → **real DeepSeek** (chaperone + flagship reasoner). Driven through the browser as the seeded student **Phoebe**. Scenario: "中国是否让地球变得更可持续？ / 卫星图显示中国让地球变绿".

This is both a **verification report** (what works) and a **polish backlog** (what to build next), with each item scoped as an independently developable sub-task.

> **Update 2026-07-07 — ST-1 and ST-2 are FIXED** on branch `polish-e2e-fixes` (commits `705b799`, `c7648b7`). ST-1: `task_id` added to the message/card DTOs → reload keeps history. ST-2 (delivery): anchors now ride the `card` SSE event → the keystone renders at summon time. The ST-2 **coverage** question (how many/which cards are annotation-mode) is left open as a product decision. Remaining: ST-3..ST-9.

---

## 1. Verdict

The platform is **end-to-end functional and the AI quality is genuinely high**. Every page in the binding design is implemented and visually consistent. The course lifecycle is solid. The working-portal lifecycle works live — but the audit surfaced **one P0 correctness bug** (task history does not reload) and confirmed that **the flagship keystone — material-anchored questioning — is real at the data layer but under-delivered in the UI and under-deployed across cards**. These are exactly the "challenging parts to polish" the product now needs to dive into.

---

## 2. What was verified (works)

### (a) Product walk — design consistency ✅
- 4-tab shell (课程 · 批判思维 · 我的评估 · 设置), directory greeting + 开新项目 + project list, Courses grid, course player (teaching / challenge / report), workspace (chat + material pane + process-tree tabs + card runtime), 我的评估 (3 tabs), 设置 (个人 / AI 形象 / 其他) — **all present, all matching the design**. Content is real, not lorem. Explainable deviations only (real data shapes).
- `?trial=1` auto-login as seeded Phoebe works. Console "errors" are all benign (favicon 404; pre-login `/auth/me` 401; `/evaluation` 404 = "no eval yet").

### (b) Course lifecycle ✅ (with gaps in §4)
- **Live AI teaching content is excellent** — step 1 "当心朋友圈的「绿色」陷阱", step 2 "横向溯源：跳出单一来源" — on-topic, pedagogically sequenced, in-template.
- **Challenge step is the keystone in-course** — the AI generated 3 material-anchored questions across `verify_claim` dimensions (权威性 / 准确性 / 目的性), each grounded in the article's real claims (2019 卫星数据, 四分之一, 科技博主, 最大功劳). Submit → static feedback.
- **Report** renders (hero, stats 约40分钟 / 3-of-3 / 1 挑战 / 4 工具, learnings, challenge review, actions).
- **Persistence confirmed in Postgres**: `course_progress` (`current_ordinal=2`, `completed_ordinals={0,1,2}`), `course_step_render` (3 rows, all `source=generated`, holding the real AI content).
- **Second run** (回顾): resumes at the saved step (no step-0 flash) and replays from cache **byte-identical** with no new LLM call. Grid shows 100% · 已学完 · 回顾.

### (c) Work lifecycle ✅ (with the P0 bug in §3)
- **AI restraint + material handling is on-model.** Opening turn: validates the student, asks for the passage by paste ("我这边打不开链接"), invites the student's prior judgment. Second turn: structures the article into 一个数据 / 一个结论 / 一个暗示, names the logical flaw, proposes a card.
- **Three different cards summoned appropriately** across the session, each with a contextual `nudge_text` quoting the student's own words: `fact-opinion-value` (form) → `craap` (form) → `concession` (annotation). Confirm-to-open (打开卡 / 暂不) respects "触发自动，打开由学生确认".
- **Card lifecycle works**: summon → nudge → open → schema-driven fill (textarea / single_choice / repeatable_group) → 提交并钉到过程树 → "已用 1 张工具卡" + "已完成 · 已钉到过程树".
- **Friction-as-signal works**: skipping CRAAP recorded "已跳过（已记录为信号）· 仍可打开".
- **Evaluation is real and high-quality**: SOLO-graded 思维印记 ("基于完整对话 + 标准信封"), dimension families (🚀 生成式驾驭 / 🛡️ 批判式防护), per-dimension coverage, and an honest, specific **process narrative** citing actual behavior with dimension+level tags (D6 L3, D10 L2) and an actionable next step. Generated inline from the full backend conversation.

### Data flow ✅ (persistence) — see §3 for the reload gap
- All writes land correctly in Postgres: `messages` (14), `card_instances` (3, incl. generated anchors), `course_progress`, `course_step_render`, `evaluations`.
- Client never calls the model directly (verified). Course render served from cache on replay. Entitlement gate + flagship resolver on the AI-critical paths.

---

## 3. Bugs found (severity-ranked)

### P0 — Task history does not rehydrate on reload / re-entry  ⛔ (blocks the core "leave and come back" promise)
**Symptom:** after a page reload, re-opening an existing task shows an **empty conversation, 0 cards, empty process tree** ("已用 0 张工具卡") — even though the DB holds all 14 messages, 3 cards, and the evaluation.
**Root cause (confirmed):** `apps/api/internal/api/dto.go` — `messageDTO` (lines 99-105) and `cardDTO` (lines 125-136) **omit `task_id`**. `WorkspaceView.tsx` (lines 64, 68) filters `storeState.messages.filter(m => m.task_id === taskId)` and the same for cards. Rehydrated rows arrive with `task_id === undefined` → the filter drops all of them. Live sessions work only because the store stamps `task_id` locally on send/summon.
**Why tests missed it:** `WorkspaceContainer.test` mocks `messages: []`; `WorkspaceView.test` passes messages with `task_id` already set — neither exercises the real DTO shape.
**Fix (small):** add `task_id` to both DTOs (they're already task-scoped) — or stamp `task_id` inside `store.hydrateTask`. Add a rehydration integration test that fetches `getTask` and asserts the messages/cards survive the `WorkspaceView` filter.

### P1 — Keystone anchors generated but not delivered live ("0 / 0 维")
**Symptom:** opening the `concession` annotation card rendered the AnnotationBranch shell ("在真实材料上核查", "点亮的句子是我圈的") but showed **"0 / 0 维已回应"** with no anchored question and no material highlight.
**Evidence it's a delivery bug, not a generation bug:** the DB `card_instances` row for `concession` holds **1 real anchor**, correctly grounded — `block_id=b1`, quote "过去二十年地球新增的绿化面积中，有整整四分之一来自中国一个国家", a material-specific question, `author=ai`. The engine worked; the client never received/refetched it.
**Likely cause:** anchors are generated server-side during the turn (best-effort) and saved after the summon SSE returns; the client's card object keeps its 0-anchor snapshot and never refetches. Related to but distinct from the P0 `task_id` bug.
**Fix:** push anchors on the stream after generation (or have the client refetch the card when the turn completes); assert non-zero anchors render in an integration test.

### P2 — Card-count discrepancy
Directory card badge and workspace header showed "0 张卡" / "已用 0 张工具卡" in states where the DB had ≥1 completed card (also a symptom of the same rehydration/counting path). Reconcile the count source with the DB.

### P3 — Minor
- Eval-poll logs a console `404` every cycle before an eval exists (noisy; treat 404 as "none", don't log).
- Report "你学到了什么" surfaces the raw internal step `purpose` text ("让学生意识到…", "教横向溯源…") — author-facing phrasing shown to the student.
- Settings profile line "IB DP1 · A 班 · TOK" is hardcoded, not from org/enrollment data.

---

## 4. Polish sub-tasks (independently developable → merge)

Each is a self-contained demo-able workstream with current status.

| ID | Sub-task | Priority | Current status | Core gap to close |
|----|----------|----------|----------------|-------------------|
| **ST-1** | **Task history rehydration** | **P0** | Broken (P0 above); backend + client both have the plumbing | Add `task_id` to message/card DTOs (or stamp in `hydrateTask`) + rehydration test. Small, do first. |
| **ST-2** | **Keystone delivery + coverage** (material-anchored questioning) | **P0 / flagship** | Engine proven at data layer; **delivery broken live** (P1 above); **under-deployed** — only 2 of ~33 cards are `annotation` mode, so the decision layer rarely triggers it | (a) deliver generated anchors to the client (push/refetch); (b) render + material highlight + self-question path end-to-end; (c) decide how many/which cards should be annotation-mode so the flagship experience is reliably reachable. This is the product's differentiator and needs the most investment. |
| **ST-3** | **Course ⇄ evaluation/records integration** | P1 | Not started (deferred as "course-level SOLO eval") | Courses currently emit no evaluation/activity → course work is invisible in 我的评估 (0 天有思考). Challenge answers are ephemeral (local state, never persisted/evaluated) — violates "过程即数据" for courses. Persist course-challenge envelopes + feed the ability model / activity calendar / growth reviews. |
| **ST-4** | **In-course AI chat (问印记 ask-panel)** | P1 | Not started (deferred "4c-ask") | The "chat with AI during a course" user-story has no surface today. Add the course-side ask-panel calling a course-ask endpoint. |
| **ST-5** | **我的评估 data loading** | P2 | Works only incidentally | `RecordsView` reads `storeState.evaluations`, which only fills when a workspace hydrates a task — the records surface never independently fetches the student's evaluations/activity, so it shows empty out of context. Give it its own fetch. |
| **ST-6** | **Card-summon UX + counts** | P2 | Works, rough edges | Model tends to pre-ask in prose *then* call `summon_card` (an extra round-trip per card). Reconcile card counts (P2 above). Decide whether prose-preamble is desired or the nudge should carry it. |
| **ST-7** | **Report content polish** | P2 | Works, author-voiced | Phrase report learnings from the student's POV instead of surfacing internal `purpose` strings; real challenge pass/fail once ST-3 lands. |
| **ST-8** | **Voice (TTS/STT)** | Later | Not started (deferred "4-voice") | The one genuinely new tech; isolatable. |
| **ST-9** | **Hardening** | Later | Noted | SSRF guard CGNAT/TEST-NET ranges; `RightPanel` drag-listener unmount cleanup; eval-poll 404 log noise. |

**Recommended order:** ST-1 (unblocks everything, tiny) → ST-2 (the flagship, largest) → ST-3 → ST-4 → the rest.

---

## 5. How to reproduce this run
```
docker run -d --name mi-e2e-goal -e POSTGRES_PASSWORD=postgres -e POSTGRES_DB=mindimprint -p 55432:5432 postgres:16
cd apps/api && DATABASE_URL="postgres://postgres:postgres@localhost:55432/mindimprint?sslmode=disable" go run ./cmd/api -migrate-up
DATABASE_URL=... COOKIE_SECURE=false CORS_ORIGINS=http://localhost:5174 go run ./cmd/api      # :8080, needs apps/api/.env.local DEEPSEEK_API_KEY
pnpm --filter web dev --port 5174 --strictPort                                                # :5174
# open http://localhost:5174/?trial=1  (auto-login as seeded Phoebe)
```
(A `apps/web/e2e/run-stack.sh` harness does the same for scripted Playwright specs.)
