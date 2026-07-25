# Spec E2 · 家长报告（阶段模式）— Design

> Part of the assessment + teacher-end + parent-report program
> (`docs/2026-07-24-assessment-teacher-end-program.md`, Spec E). This is **E2**,
> the deferred second half of Spec E: the **阶段模式** parent projection + the
> teacher-side 阶段报告 export. It completes the program.

**Status:** design approved 2026-07-25. Ready for implementation plan.

**Depends on:**
- **Spec B** — canonical `agent.Report` (`DualAxisReport`) producer + contract.
- **Spec D1** — teacher read-path, tenancy guards (`assertTeacherOwnsClass` +
  `authTeacherStudent`), per-student teacher-scoped report reads.
- **Spec D2** — the event-stream **usage pipeline**, the Monday-start UTC-pinned
  **week window** math (`weekWindow`), and `WeekLabel` (第 N 周（M.D–M.D）).
- **Spec C** — `internal/ability.Aggregate`, the cross-session 能力素养 merge.
- **Spec E1** — reuses `parentReportContent.ts` (static front-matter),
  `ParentReport.tsx`, the composer/metering discipline, and migration **0035**'s
  `parent_report_prose` table (**no new migration**).

**Binding design artifact:** the `isStage` half of
`docs/design/teacher end/project/家长报告.dc.html`. All static教育 front-matter is
shared with 项目模式 (already reproduced in `parentReportContent.ts`); the
阶段模式-specific layout (`这一阶段的使用与成长`) is authoritative there.

---

## 1. Goal

Wire the teacher console's inert **导出家长版·阶段报告** stub into a real,
printable **stage-mode parent projection** for one student over one **week
window**. Unlike 项目模式, the stage view renders **no** per-dim D/A readings and
**no** opportunity block — the design's `isStage` half is: 4 usage-stat cards +
a cross-session growth narrative + the shared front-matter + advice + glossary.

**Out of scope (confirmed):**
- **Student-side 导出家长版 entry** — teacher-only, same as E1. The parent report
  stays a teacher→parent artifact.
- Server-side PDF (browser `window.print()` only).
- **Computed period-over-period per-dim delta** — the growth narrative is
  qualitative (current standing + this-window usage), not a hard numeric delta.
- Any teacher-warning / watch-tag / 👍👎 / prompt-lens / official-AP leakage.

---

## 2. Architecture — "one producer, one more projection" + D2 compose-and-store

A stage report is a per-**(student, week)** parent projection. It re-uses the
same three data classes as E1, re-sourced for the stage view:

### 2a. Deterministic, computed live on every GET (no model)

- **Usage stats (4 cards)** — a new single-student window query
  `GetStudentWeekStats :one` mirroring D2's `GetClassWeekStats`口径 exactly, for
  one `user_id` over `[week_start, week_end)`:
  - `本周活跃` = active days = `COUNT(DISTINCT (created_at AT TIME ZONE 'UTC')::date)`
  - `对话轮次` = turns = events of type `prompt_sent` **or** `course_message`
    (D1/D2口径, verbatim)
  - `生成报告` = `student_evaluation` rows in the window
  - `完成课程` = `COUNT(DISTINCT (user_id, course_id, payload->>'ordinal'))` over
    `step_viewed` events (D2's `course_steps`口径)
  Rendered as `{value, label}` cards. Labels are **server strings** (single-source,
  D1 anti-drift): `本周活跃 / 对话轮次 / 生成报告 / 完成课程`. Values are the
  numbers with their unit suffix composed server-side (`6 天` / `78` / `3 份` /
  `5 节`), matching the design fixtures.

- **Cross-session ability model** — `ability.Aggregate` over the student's
  **full** report history, via a new teacher-scoped `ListStudentEvaluationsForTeacher
  :many` (returns `scores, created_at` from the `student_evaluation` view, filtered
  to `@user_id`; the handler's `authTeacherStudent` has already proven the student
  is in a class this teacher owns). This model is fed to the composer as **growth
  substance only** — it is **never rendered as rows**, and **no ability number ever
  reaches the parent surface** (RL-5).

### 2b. Static教育 front-matter (reused verbatim from E1, never on the wire)

The stage view renders the **same** shared sections as 项目模式 — 这份报告怎么读
(principles) · 我们怎么看"能力"：两条轴 (dLevels/aStates/dDims/aSignals) · 判断是
怎么得出来的 (howList) · 平台接下来会做 (supply) · 名词解释 (glossary) · footer.
**No new static content**: all of it already lives in
`apps/web/src/console/parentReportContent.ts`.

### 2c. Model-composed warm prose (the only spend), stored first-open-wins

None of this exists in the canonical object. One **flagship** call gentles the
finished ability model + window usage into the family-facing stage bundle:

- `warmLine` (cover 温暖引导句)
- `stageGrowth` (这段时间的变化 — qualitative, informed by the ability model's
  current standing: 继续保持高位 / 从"发展"迈向"熟练" / 使用刚起步、暂未见到明显提升)
- `stageHighlight` (本阶段亮点 — **optional**; empty string ⇒ section hidden)
- `stageForward` (往前看 — one line)
- `advice[3]` (在家可以怎么帮 · each `{title, text}`)

The composer's input is the ability model (current standing + evidence counts,
qualitative) **plus** this-window usage counts + the student's display name /
subject / class. It never sees a project's per-dim canonical readings — the stage
report is not scoped to a single project.

---

## 3. The composer — `internal/agent/compose_parent_stage.go`

- ONE call, **flagship, never downgraded**, via `EvalResolver`, `llm_call`
  `Purpose:"parent_report"`, `Surface:"teacher"`, **meters even on rejection**
  (matches E1 / D2). `ProjectID` is **NULL** — a stage report is not
  project-scoped (its scope is a week).
- New types `agent.ParentStageProse{WarmLine, StageGrowth, StageHighlight,
  StageForward string; Advice []ParentAdvice}` (reuses E1's `agent.ParentAdvice`)
  and a `ParentStageFacts` input struct (name, subject, class, ability model
  summary, the 4 usage numbers).
- `ComposeParentStage(ctx, prov, resolved, facts) (ParentStageProse, gateway.ChatUsage, error)`
  + `ParentStageFactsPrompt(facts)` + `validateParentStageProse`, mirroring
  `compose_parent.go`.
- **Output validation** (reject → 200 + `prose:null`, never walls):
  1. **No internal register leaks** — reuse E1's `parentBareCode`-style regex
     (case-insensitive, space-tolerant): reject if the prose contains
     `given_taken|given_not_taken|not_supplied`, `SOLO`, `P0`–`P3`, a bare
     `\b[DA]\s*[1-6]\b`, **or a bare level code `\bL\s*[1-4]\b`** (the composer is
     fed level ints, but the parent surface shows only the 台阶 words
     起步/发展/熟练/优秀, never `L3`). **No A-axis number** ever appears (RL-5).
  2. **No empty wording** in `warmLine` / `stageGrowth` / `stageForward` / any
     advice item (`stageHighlight` **may** be empty — 敢于空白).
  3. Rune-length caps on each field (reuse E1's cap constants where sizes match).
  4. `advice` has exactly 3 items, each with non-empty title + text.
- **敢于空白:** a thin window (0 reports, 0 usage, or `ability.Model` with all
  dims at `Level == -1` / 证据不足) → the composer is instructed to write the
  起步 register ("使用刚起步，暂未见到明显提升") and leave `stageHighlight` empty.
  The deterministic zero-stats still render regardless of prose.

---

## 4. Storage — reuse migration 0035 `parent_report_prose` (no new migration)

```
parent_report_prose  (from E1, migration 0035)
  student_user_id  uuid    not null
  surface          text    not null   -- E1: 'project'; E2: 'stage'
  scope_id         text    not null   -- E1: project UUID .String(); E2: week_start 'YYYY-MM-DD'
  prose            jsonb   not null    -- E2 stores the §2c ParentStageProse bundle
  created_at       timestamptz not null default now()
  primary key (student_user_id, surface, scope_id)
```

- `scope_id` is already `text`; a `week_start` date string (`YYYY-MM-DD`) fits.
  `surface='stage'` partitions stage rows from project rows under the same PK.
- **First-open-wins** per `(student, 'stage', week_start)`: `INSERT ... ON
  CONFLICT DO NOTHING`, re-read for the concurrent winner. **scope_id churns
  weekly**, so each new week generates fresh prose — the accepted-staleness note
  from E1 applies only within a week.
- The existing sqlc queries `GetParentReportProse` / `InsertParentReportProse`
  (keyed on `student_user_id, surface, scope_id`) are **reused as-is** — no new
  sqlc for storage. New sqlc: `GetStudentWeekStats`, `ListStudentEvaluationsForTeacher`.
  `make sqlc` from `apps/api`; never hand-edit `store/sqlc/*`.

---

## 5. Endpoints — parallel stage handlers (E1's project path stays byte-unchanged)

E1's `getParentReport`/`postParentReportProse` are UUID-typed end-to-end
(`uuid.Parse(scopeId)`, `loadParentReport(scopeID uuid.UUID)`, `RecordLLMCall`
ProjectID from the scope UUID). A stage scope is a **date string**, so E2 adds
**separate** handlers rather than overloading the UUID path:

- **`GET /classes/{id}/students/{userId}/parent-stage-report/{weekStart}`**
  - `weekStart` = `YYYY-MM-DD`; empty / `current` → server resolves the current
    week via D2's `weekWindow(now)`.
  - Computes §2a usage stats live, reads stored stage prose (§2c) if present.
  - Returns the stage wire object with `prose: "present" | null`.
  - **Never calls the model.** Cost-free.

- **`POST /classes/{id}/students/{userId}/parent-stage-report/{weekStart}/prose`**
  - The **only** spend. Compose-once (`ON CONFLICT DO NOTHING`), flagship,
    metered even on rejection. `ProjectID` NULL in the `llm_call`.
  - Success → `200` + the stored bundle. Composer failure / validation reject →
    `200` + `prose:null` (never walls; the deterministic stats still render).

Both under D1 tenancy: `authTeacherStudent` (teacher owns a class the student is
enrolled in); every failure existence-hidden `404`. New handler file
`internal/api/parent_stage_report.go`; routes registered next to E1's in
`api.go` behind `teacherOrAdmin`.

**Window resolution:** the stage report always targets a **whole** week window
`[week_start, week_end)`. Default = the current week. (No teacher window-picker
UI in E2 — the route accepts an explicit `weekStart` for future reuse, but the
stub opens the current week.)

---

## 6. Contract — extend `packages/contracts/src/parentReport.ts`

Add two shapes, distinct from E1's project shapes; three parallel definitions
align (Zod here · Go compose output · Go stage DTO):

```
// Composer output = the stored §2c stage bundle.
ParentStageProse = {
  warmLine, stageGrowth, stageForward: string,
  stageHighlight: string,              // may be ""
  advice: { title, text }[]            // exactly 3
}

// One usage-stat card.
ParentStageStat = { value: string, label: string }   // "6 天" / "本周活跃"

// GET response.
ParentStageReport = {
  cover: { name, subject, klass, typeLabel, dateStr, warmLine },
  //       subject = "第 N 周（M.D–M.D）", typeLabel = "阶段报告"
  stats: ParentStageStat[],            // 4
  stageGrowth: string,
  stageHighlight: string,              // "" ⇒ hidden
  stageForward: string,
  advice: { title, text }[],           // 3, or [] pre-prose
  prose: "present" | null              // null ⇒ deterministic stats only, show 生成 button
}
```

When `prose === null`, the wire object still returns the 4 `stats` (deterministic)
+ the cover (with empty `warmLine`) + empty growth/highlight/forward/advice; the
client renders the stats and the 生成 button. Static §2b content is **not** on the
wire — it lives in `parentReportContent.ts`. Export the new shapes from `index.ts`.

---

## 7. Web — extend `apps/web/src/console/ParentReport.tsx` with a stage branch

The dc-html is one component with a `mode` prop (project|stage) sharing cover +
front-matter + advice + glossary. E2 mirrors that: `ParentReport` takes a `mode`
prop and branches the middle section.

- **Stage layout** (reproducing the `isStage` half): cover(阶段报告 · 第 N 周) →
  这份报告怎么读 → 我们怎么看"能力"：两条轴 → 判断是怎么得出来的 →
  **这一阶段的使用与成长** (4 stat cards + 这段时间的变化 + 本阶段亮点[if non-empty]
  + 往前看) → 下一步 (advice + supply) → 名词解释 → footer.
- Same print discipline as E1: `@media print` hides the rail/chrome/control-bar;
  **下载 PDF** = `window.print()`.
- On open: GET (cost-free). `prose === null` → render the deterministic stats +
  a visible **生成家长版正文** primary button → POST → re-render.
- **Derive nothing on the client** beyond presentation — stats, labels, growth,
  advice are all server strings.
- Two new API methods in `src/api/` (`getParentStageReport(classId, studentId,
  weekStart?)`, `generateParentStageProse(...)`), matching the singleton-`api`
  spy shape E1's methods use.

### Stub wiring

- **StudentDetailView** `导出家长版·阶段报告` → opens `ParentReport` in
  `mode="stage"` for `(student, current week)`. **WIRE** (was inert in E1).
- E1's `导出家长版·项目报告` + `导出家长版 PDF` stay as wired in E1 — unchanged.

---

## 8. Invariants (every task inherits — identical to E1 §8)

- 客户端绝不直连模型；所有 LLM 调用走后端网关；密钥只在 `apps/api`。
- **评估走旗舰绝不降级**；评估器隔离（stage 用 `llm_call` Purpose:"parent_report",
  Surface:"teacher", ProjectID NULL；assessment 仍 `"assessment"`）。
- **两轴永不合成总分** (RL-5)；stage 面 **无任何 A 轴数字**，且**无任何能力
  等级数字**（能力模型只喂给 composer 作叙述底料，绝不上屏）。
- **机会供给先于判定**：thin/欠账 窗口不算学生短板；composer 走 起步 register。
- **敢于空白**：无证据/无使用记 起步 叙述，`stageHighlight` 留空，不硬凑亮点。
- **说人话**：stage 面不出现内部代码 / 术语（compose 输出正则校验拦截）。
- **家长端与教师端隔离**：不含预警 / 标签 / 👍👎 / 提示词透镜 / 官方 AP 投影 /
  交互证据 / 作品与过程。
- **GET 永不 spend；POST 是唯一 spend**，失败返回 200 + prose:null，绝不成墙。
- Single-source deterministic parts (stat labels, week label) server-side.
- Test discipline: card/gate/projection/creation/config/migration changes run
  **FULL** Go packages (`-p 1 ./...`), never `-run` subsets; `internal/api`
  needs ~210s so every backend test dispatch passes `timeout: 600000`.
  Web `npm test` + `npx tsc --noEmit`; contracts `npm test` + `npx tsc --noEmit`.

---

## 9. File map

**New**
- `apps/api/internal/agent/compose_parent_stage.go` — flagship stage composer +
  `ParentStageProse` / `ParentStageFacts` + validation (+ `compose_parent_stage_test.go`).
- `apps/api/internal/api/parent_stage_report.go` — the two stage routes +
  handlers + stage DTO + usage/ability assembly (+ `parent_stage_report_test.go`).
- `apps/api/internal/store/queries/teacher.sql` — **add** `GetStudentWeekStats`,
  `ListStudentEvaluationsForTeacher` (regenerate sqlc).

**Modified**
- `apps/api/internal/api/api.go` — register the two stage routes.
- `packages/contracts/src/parentReport.ts` — add `ParentStageProse`,
  `ParentStageStat`, `ParentStageReport`; export from `index.ts` (+ test).
- `apps/web/src/console/ParentReport.tsx` — add `mode` prop + stage branch.
- `apps/web/src/api/index.ts` + the teacher API module — two new methods.
- `apps/web/src/console/StudentDetailView.tsx` — wire 阶段报告 stub.

**Reused unchanged**
- `apps/api/internal/store/migrations/0035_parent_report_prose.sql` +
  `GetParentReportProse` / `InsertParentReportProse` sqlc (storage).
- `apps/web/src/console/parentReportContent.ts` (all static front-matter).
- `internal/ability`, D2's `weekWindow` / `WeekLabel`.

---

## 10. Acceptance

For a seeded student in 吴老师's class with cross-session reports + this-week
usage events:

1. Teacher opens the student's detail page → clicks 导出家长版·阶段报告 → the
   stage report renders the current-week cover (阶段报告 · 第 N 周（M.D–M.D）), the
   4 usage-stat cards with real numbers, the shared front-matter, and a
   生成家长版正文 button (prose not yet composed).
2. Click 生成 → one `parent_report` `llm_call` recorded (ProjectID NULL) → the
   page fills with 这段时间的变化 / 本阶段亮点 / 往前看 / 在家可以怎么帮. Re-open →
   same prose, **no second llm_call** (first-open-wins).
3. The rendered prose contains **no** bare D/A codes, **no** A-axis numbers, **no**
   ability level numbers, no teacher-only constructs.
4. A student with no usage and <2 contributing sessions renders zero-stat cards +
   the 起步 register growth and **no** 本阶段亮点 section (敢于空白).
5. 下载 PDF triggers the browser print dialog; the printed page shows only the
   report (rail/chrome/control-bar hidden).
6. E1's 项目模式 path is unchanged (its handlers/tests untouched).
7. All three suites green: Go full packages, contracts, web + tsc.
