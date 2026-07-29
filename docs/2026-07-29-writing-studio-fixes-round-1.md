# Writing Studio — Fixes Round 1 (post-deploy feedback)

12 issues from real use of the shipped four-room studio. Grouped into a backend workstream + per-room frontend workstreams. Contracts pinned here so parallel agents agree.

## Backend (Go + contracts) — one workstream

### BE1 · Paste article body (fixes #1: enter-reading `fetch_failed`)
Many URLs can't be fetched (site blocks / network). `ingestMaterial` already supports pasted text (`req.Text` → `materialize.Segment` → blocks, origin "pasted").
- New endpoint `POST /api/v1/projects/{id}/references/{rid}/paste-content` body `{text, title?}` → returns the full `MaterialSource`. It creates a `material` (source="pasted") from the text via the same paste path, links `reference.material_id`, `appendAutoLog("粘贴正文《title》")`. 400 if text empty.
- Keep `enter-reading` returning 422 `{error:"..."}` on fetch failure (already does) — the frontend offers the paste box on that error.

### BE2 · Coach: review scope (fixes #2 "ask AI to review")
Extend `POST /coach` scope enum with `"proposal_review"`. Prompt (restrained, one focused critique, never rewrites):
`你是「印记」。学生刚把开题四问填了个初稿（在下面）。请只做一件事：指出最值得再想清楚的一两点——哪里还含糊、哪个尺度没定、哪个反例没考虑；给一个具体的下一步。不要替他重写，不要给成段答案。简短。`
Frontend passes the four dims as `user_input`.

### BE3 · Generate plan from the kickoff (fixes #2 "generate plan")
New `POST /api/v1/projects/{id}/plan/generate` → `{items: PlanItem[]}`. Spend (mid-tier `ChatResolver`, meter `purpose="plan_gen"`).
- Read the proposal. If all four dims empty → 422 `{code:"proposal_empty", message:"先聊清楚开题，再生成计划"}`.
- One-shot LLM → STRICT JSON array of 5–9 tasks `[{"title","tag":"read|write|review","stage","start","days"}]` grounded in the objective/activities/resources (Chinese titles; stages like "阶段一 · 研究" / "阶段二 · 写作"; sensible start/days along a ~18-day timeline; tags balanced read/write/review).
- Persist each as a `plan_item` (column="todo", position by index). Return them. `appendAutoLog("印记根据开题生成了项目计划")`.
- Graceful: on LLM failure/parse-fail, seed a small deterministic default plan (3–5 items) so the button always yields a usable board (record cost if a call happened).

### BE4 · Mirror: never persist the failure fallback (fixes #8 "思维印记 is fake")
Root cause: `POST /mirror` stores `MinimalMirror()` permanently via ON CONFLICT DO NOTHING even when compose fails, so canned text sticks forever.
- Change `POST /mirror`: only `InsertProjectMirror` on a **successful** compose. On failure, return `MinimalMirror()` (or better, an honest empty) **without persisting**, so a later open retries once real data/LLM is available.
- `GET /mirror` unchanged (returns stored or null).

### BE5 · Project status lifecycle + non-blocking finish (fixes #9, feeds #11)
- Migration `0037_project_status_evaluating.sql`: widen `project.status` CHECK to `('active','evaluating','finished')` (Up + Down).
- `POST /finish`: after the reflection-done gate + not-already-finished/evaluating check, set `status='evaluating'`, return **202** `{status:"evaluating"}` immediately. Spawn a goroutine with a **detached `context.Background()`** (NOT the request context) that runs `generateProjectReport`; on success `SetProjectFinished` (status='finished'); on failure set status back to `'active'` (retryable). Best-effort also compose+store the mirror there (real post-completion data). Metering as before. Guard: refuse a second finish while `evaluating`.
- **Derived display status** on both the project-list items and the workspace projection: `finished→"done"`, `evaluating→"evaluating"`, `active` with any non-empty proposal dim OR ≥1 plan_item → `"working"`, else `"forming"`.
  - `GET /projects` list items: add `status` (one of forming|working|evaluating|done). Keep existing fields.
  - `GET /projects/{id}` (workspace projection): add `status` field.
- Contracts: add `status` to `WorkspaceProjection` (`packages/contracts/src/workspace.ts`, enum), and to the project-list item type used by the web (`apps/web/src/api/projects.ts` `ProjectListItem` — add `status`). Update the Go DTOs to match. Update any list/projection Go test.

### Contracts/type changes summary
- `WorkspaceProjection`: `+ status: "forming"|"working"|"evaluating"|"done"`.
- `ProjectListItem` (web): `+ status: same enum`.
- New client fns (added by the MAIN agent, not the room agents): `pasteContent(id,rid,text,title?)`, `generatePlan(id)`, extend `CoachScope` with `"proposal_review"`, and `getWorkspace`/`listProjects` parse the new `status`.

## Frontend — per-room workstreams (each edits ONLY its block; the main agent owns api/workspace.ts + container status/nav)

### FE-Plan (`workspace/blocks/PlanBlock.tsx`) — #2, #3, #4, #5, #11
- **#11**: initial phase = `forming` when the proposal is empty (all 4 dims blank), else `working`.
- **#2 guided coach**: the forming chat opens with a SCRIPTED guiding message: “要做好一个研究项目，先想清楚四件事：**目标**（想回答什么）、**缘由**（为什么做）、**活动与时间**（打算怎么做）、**资源**（需要什么）。想让我一部分一部分带你想，还是你已经有想法、想直接填右边？” Offer two quick chips: “带我一部分一部分想” and “我自己填”. The back-and-forth uses `coach(projectId,"forming",...)`. Add a **“让印记看看我的开题” quick button** near the dims → `coach(projectId,"proposal_review", <dims summary>)`, append the suggestion into the chat. The dims stay student-editable (AI writes nothing into them directly).
- **生成项目计划** button → `generatePlan(projectId)` (show a brief loading state), then flip to working and load the generated items. Keep it available once ≥1 dim is filled.
- **#3 gantt add task**: add a “+ 添加任务” affordance in the Gantt view (create via `createPlanItem`, default stage = the last stage, then refetch) — the Kanban already has one.
- **#4 card = view/edit detail**: clicking a plan card opens a **detail/edit popover/modal** (edit title, tag, stage, column, start, days → `patchPlanItem`; delete). A SINGLE separate small button on the card (“进入 →”) does the doorway jump to the room. So: card body click = edit; the arrow button = navigate. Apply in BOTH kanban and gantt rows.
- **#5 activity log grouped by date**: render one **box per date** containing all that date’s lines (each line keeps its 自动/我记的 tag). Keep “记一笔”. Export still produces the merged doc.

### FE-Read (`workspace/blocks/ReadingBlock.tsx`) — #1, #10
- **#1 paste fallback**: `enterReading` throws on 422/`fetch_failed`. Catch it and show an inline **paste-body box** in the preview (“取不到正文，把正文粘进来”): textarea → `pasteContent(projectId, ref.id, text)` → `setReadingSource(materialSource)`. Also add a **“粘贴正文” tab** to the Add-source modal (creates the reference, then immediately pastes content) — optional but preferred.
- **#10 empty-state coach**: when the library is empty, the floating 印记 must be **openable and guiding** — open it by default (or make it clearly clickable) with a guiding opener: “你的文献库还空着。告诉我你的题目和你想找什么证据，我给你方向和关键词（我不替你搜）。” Uses `coach(projectId,"find_sources",...)`.

### FE-Write (`workspace/blocks/WritingBlock.tsx`) — #6, #7
- **#6 outline keyboard editing**: in the bullets view, **Enter = new sibling line** (focus it), **Tab = indent**, **Shift+Tab = outdent**, Backspace on an empty line deletes it and focuses the previous. Persist via the existing debounced `putOutline`. Make the **mind-map** nodes editable inline too (they partly are — ensure add/edit is easy; at minimum inline-edit + an add-child affordance).
- **#7 markdown render**: the “写在这里” panel claims Markdown but shows raw text. Add a **preview** — a 写/预览 toggle (or split) that renders the Markdown (add a small dep like `marked` + sanitize, or `react-markdown`; dynamic-import to keep the bundle lean). Word count stays.

### FE-Review+Directory+Container (owned by MAIN agent) — #8, #9, #11, #12-hook
- **#9 finish flow**: 完成回顾 → `putReflection({done:true})` → `finishProject` (now returns 202 evaluating). Show a **modal**: “评估报告正在生成，可能需要几分钟。你可以先去别处，生成好后能在‘全部项目’里点开查看。” with a “回到全部项目” action → `onFinished?.()`/back to Directory. Do NOT block.
- **Directory status**: each project row shows a **status pill** (立题中 forming / 进行中 working / 评估中 evaluating / 已完成 done). While any project is `evaluating`, **poll `listProjects` every ~15s**. A `done` project’s row gets a “查看评估报告” action → routes to the growth report for that project (see #12).
- **#8**: MirrorPane already calls getMirror/postMirror; with BE4 the fallback no longer sticks. Additionally, only auto-`postMirror` when the proposal is non-empty (avoid composing an empty mirror on a blank project) — else show “完成一些工作后，这里会长出你的思维印记”.

### FE-Growth (`shell/growth/GrowthReport.tsx`) — #12 (option 2)
- The 学习记录 tab becomes a **list of records** across surfaces (project / course / chat) from `getGrowthHistory()`. Each entry shows label + date + surface; entries that HAVE a generated report are **clickable to open the detailed report** (the existing report view). Entries without a report show a muted “未生成评估”. Keep the other tabs (工具卡 / 能力素养) intact. A `done` project navigated from the Directory “查看评估报告” should deep-link/scroll to its entry.

## Acceptance
- `go test ./...`, contracts test, web test + build all green.
- Manual: paste a body → reading room opens; guided forming chat + review + generate-plan; gantt add; card click edits (arrow jumps); log grouped by date; outline Enter/Tab; markdown preview; real mirror (no canned text); finish → modal + list status evaluating→done → open report; new project lands in forming; 学习记录 is a clickable list.
