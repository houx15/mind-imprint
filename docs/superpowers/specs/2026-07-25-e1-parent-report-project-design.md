# Spec E1 · 家长报告（项目模式）— Design

> Part of the assessment + teacher-end + parent-report program
> (`docs/2026-07-24-assessment-teacher-end-program.md`, Spec E). This is **E1**;
> stage mode + student-side entry are **E2**, deferred.

**Status:** design approved 2026-07-25. Ready for implementation plan.

**Depends on:** Spec B (canonical `DualAxisReport` producer + contract), Spec D1
(teacher read-path, tenancy guards, per-student report fetch). No dependency on
D2's usage pipeline or Spec C's cross-session 能力素养 merge — those are E2.

**Binding design artifact:** `docs/design/teacher end/project/家长报告.dc.html`
(+ `_parent_da.js`). All static教育 front-matter and the two-mode layout are
authoritative there; E1 reproduces the **项目模式** half in React.

---

## 1. Goal

Wire the teacher console's inert **导出家长版·项目报告** stub into a real,
printable **parent projection** of one student's canonical project
`DualAxisReport`. The parent surface is a *gentled* view: D 轴四台阶, A 轴无数字
三态, 机会供给先于判定, 敢于空白, 名词解释. It never re-computes assessment and
never leaks teacher-only constructs.

**Out of scope (E2 / not this program):**
- 阶段模式 (weekly usage-window stats + cross-session growth narrative — pulls in
  D2's event pipeline + Spec C's `internal/ability` merge).
- Student-side 导出家长版 entry (teacher-only for E1).
- Server-side PDF rendering (browser `window.print()` only).
- Any teacher-warning / watch-tag / 👍👎 / prompt-lens / official-AP leakage.

---

## 2. Architecture — "one producer, one more projection"

The canonical assessment object is produced **once** by Spec B. Every surface is
a projection (RL: teacher = full + internal codes + AP; student = 说人话; parent
= gentled numberless). E1 adds the parent projection. It does **not** re-run the
assessor. The single new model call is a **projection-gentling** call (rules
select, model words), the same discipline as D2's class-weekly composer — NOT a
second assessment.

Three data classes make up the parent report:

### 2a. Deterministic, computed live on every read (no model)

Single-sourced server-side to avoid Go/JS drift (D1's lesson — the A-mean
double-derivation bug):

- **D-axis badge** — `internal/parent.DBadge(level)`:
  `L1→起步 · L2→发展 · L3→熟练 · L4→优秀 · NA→暂无`. The **label** crosses the
  wire; colors are client presentation constants.
- **A-axis numberless three-state** — `internal/parent.AState(level)`:
  `0–1→暂未观察到 · 2–3→偶有·多在引导后 · 4–5→观察到主动信号`.
  **No A-axis number ever crosses to the parent surface** (RL-5 + design §2
  「A 轴不出现任何数字」). `not_supplied` naturally lands on 暂未观察到 (no
  opportunity ⇒ level 0); the 「机会已给·没接住」 vs 「平台还没提供」 nuance is
  carried by the composed 机会与真实性 paragraph, not by the state chip.

### 2b. Static教育 front-matter (fixed, never model-touched, never on the wire)

Reproduced **verbatim** from `家长报告.dc.html`'s `SHARED` block as React module
constants:

- `principles` (4 · 这份报告怎么读)
- `dLevels` (4 台阶定义) · `dDims` (6 D 维含义)
- `aStates` (3 态定义) · `aSignals` (6 A 信号含义)
- `howList` (4 · 判断怎么来) · `supply` (2 · 平台接下来会做)
- `glossary` (10 · 名词解释) · `footer`

### 2c. Model-composed warm prose (the only spend), stored first-open-wins

None of this exists in the canonical object — the object only carries
analytic-register `evidence` strings. One **flagship** call gentles the finished
object into the family-facing bundle:

- `glance` (一眼判断)
- `dOverview` · `aOverview` (each axis's one-line overview)
- `dReadings[D1..D6]` (6 gentled per-dim readings)
- `aReadings[A1..A6]` (6 gentled per-signal readings)
- `opportunity` (机会与真实性 paragraph — folds in each signal's
  `opportunity` enum: 欠账 vs 没接住 vs 全程守住)
- `warmLine` (cover 温暖引导句)
- `advice[3]` (在家可以怎么帮 · each `{title, text}`)

---

## 3. The composer — `internal/agent/compose_parent.go`

- ONE call, **flagship, never downgraded**, via `EvalResolver`, `llm_call`
  `Purpose:"parent_report"`, **meters even on rejection** (matches D2).
- Input = the canonical `DualAxisReport` for `(userId, surface, scopeId)` +
  the student's display name / subject / class (for `warmLine`).
- **Output validation** (`ValidateOutput`, rejects → 200 + `prose:null`, never
  walls):
  1. every D code `D1..D6` present exactly once; every A code `A1..A6` present
     exactly once (unknown / repeated / missing → reject).
  2. no empty wording in any reading / overview / glance / opportunity /
     warmLine / advice item (D2's own Critical-fix gap).
  3. **no internal register leaks** — a bare-code / 术语 regex (reusing D2's
     evadable-code lesson, case-insensitive, space-tolerant): reject if the
     prose contains `given_taken|given_not_taken|not_supplied`, `SOLO`,
     `P0`–`P3`, or a bare `\b[DA]\s*[1-6]\b` **outside** the fixed dim/signal
     name prefix. (Dim names like "D1 任务理解与问题表述" are supplied by the
     deterministic layer, not the composer, so the composer's free prose must
     be code-free.)
- **敢于空白:** any dim whose canonical `level == NA` (or empty evidence) is
  **not** sent to the composer for a reading; its reading is the deterministic
  literal 「暂无可计入的证据」 and its badge is 暂无. The composer is instructed
  never to invent a highlight where evidence is absent. (D6 反思 is the common
  case — only self-written reflection counts.)

---

## 4. Storage — migration 0035

```
parent_report_prose
  student_user_id  uuid    not null
  surface          text    not null   -- 'project' (E1); 'course'/'chat' pluggable later
  scope_id         text    not null
  prose            jsonb   not null    -- the §2c composer bundle (single bundle, not D2's card array)
  created_at       timestamptz not null default now()
  primary key (student_user_id, surface, scope_id)
```

- PK = the **first-open-wins lock**. `INSERT ... ON CONFLICT DO NOTHING`.
- Never refreshed. **Accepted staleness:** if the underlying project report is
  re-assessed, the stored parent prose does not regenerate — same stance and
  carry-forward as D2's `class_weekly_prose`. Project reports are generated
  once at project-end and rarely re-run, so this is low-probability.
- sqlc queries in `store/queries/parent.sql`
  (`GetParentReportProse`, `InsertParentReportProse` with `ON CONFLICT DO NOTHING`).
  `make sqlc` from `apps/api`; never hand-edit `store/sqlc/*`.

---

## 5. Endpoints (two-endpoint split, cost-free read / explicit spend)

Both under D1's teacher tenancy: `assertTeacherOwnsClass` +
`authTeacherStudent` (student must be in a class this teacher owns); every
failure existence-hidden `404`.

- **`GET /students/{userId}/parent-report/{surface}/{scopeId}`**
  - Fetches the canonical report (reusing D1's read path), computes §2a
    deterministic parts live, reads stored prose (§2c) if present.
  - Returns the full parent wire object with `prose: <bundle> | null`.
  - **Never calls the model.** 404 if no canonical report exists for the scope.

- **`POST /students/{userId}/parent-report/{surface}/{scopeId}/prose`**
  - The **only** spend. Compose-once (`ON CONFLICT DO NOTHING`), flagship,
    metered even on rejection.
  - Success → `200` + the stored bundle. Composer failure / validation reject
    → `200` + `prose:null` (never walls; the deterministic report still renders).

---

## 6. Contract — `packages/contracts/src/parentReport.ts`

Single source of truth for the wire shape; three parallel definitions align
(Zod here · Go compose output · Go `ReportDTO`), same rule as B.

Two distinct shapes — the **composer output** (what the LLM returns + Zod/Go
`ValidateOutput` check + what's stored in `parent_report_prose.prose`) and the
**wire object** (what GET returns, readings already merged into rows):

```
// Composer output = the stored §2c bundle. dReadings/aReadings are keyed maps
// so validation can assert every D1..D6 / A1..A6 present exactly once.
ParentReportProse = {
  glance, dOverview, aOverview, opportunity, warmLine,
  dReadings: { D1: string, ..., D6: string },   // only for dims with evidence
  aReadings: { A1: string, ..., A6: string },
  advice: { title, text }[]
}

// Wire row shapes — badge/state (deterministic §2a) + reading merged server-side.
ParentReportDRow = { code, name, badge: string, reading: string }
ParentReportARow = { code, name, state: string, reading: string }

// GET response.
ParentReport = {
  cover: { name, subject, klass, typeLabel, dateStr, warmLine },
  glance, dOverview, aOverview,
  dRows: ParentReportDRow[],   // 6; NA dim ⇒ badge 暂无 + reading 「暂无可计入的证据」
  aRows: ParentReportARow[],   // 6
  opportunity: string,
  advice: { title, text }[],
  prose: "present" | null      // null ⇒ deterministic parts only, show 生成 button
}
```

When `prose === null`, the wire object still returns all 6 `dRows`/`aRows` with
deterministic badges/states and empty `reading`s (+ empty glance/overview/
opportunity/advice); the client renders those and the 生成 button.

Static §2b content (principles/glossary/…) is **not** on the wire — it lives in
the React view as constants. Colors are client constants keyed off the server
`badge`/`state` label.

---

## 7. Web — `apps/web/src/console/ParentReport.tsx`

- Print-friendly React view reproducing the `家长报告.dc.html` **项目模式**
  layout in inline styles: cover → 这份报告怎么读 → 我们怎么看"能力"：两条轴 →
  判断是怎么得出来的 → {{name}} 这次的表现 (glance · D 逐维 · A 逐维 · 机会与真实性)
  → 下一步 (advice + supply) → 名词解释 → footer.
- `@media print` hides the app rail/chrome and control bar; page margins ~0.85in.
  **下载 PDF** button = `window.print()` (no server PDF library).
- On open: GET (cost-free). If `prose === null`, render the deterministic report
  plus a visible **生成家长版正文** primary button → POST → re-render. Keeps
  navigation cost-free and spend explicit.
- **Derive nothing on the client** beyond presentation: all badges/states/
  readings are server strings; the view only maps a badge/state label to a color
  constant.

### Stub wiring

- **StudentDetailView** `导出家长版·项目报告` → opens `ParentReport` for
  `(student, surface:'project', primaryReport.scopeId)`. **WIRE.**
- **TeacherReportView** `导出家长版 PDF` → same project scope currently open in
  the deep report. **WIRE.**
- **StudentDetailView** `导出家长版·阶段报告` → **stays inert** (E2).

---

## 8. Invariants (every task inherits)

- 客户端绝不直连模型；所有 LLM 调用走后端网关；密钥只在 `apps/api`。
- **评估走旗舰绝不降级**；评估器隔离（`llm_call` Purpose 分离；parent 用
  `"parent_report"`, assessment 仍 `"assessment"`）。
- **两轴永不合成总分** (RL-5)；parent 面 **A 轴不出现任何数字**。
- **机会供给先于判定**：平台没给机会 = 平台欠账，不算学生短板；只有「机会已给、
  没接住」才算学生信号。（carried in the 机会与真实性 paragraph.）
- **敢于空白**：无证据记「暂无可计入的证据」，不硬凑亮点。
- **说人话**：parent 面不出现内部代码 / 术语（compose 输出正则校验拦截）。
- **家长端与教师端隔离**：不含预警 / 标签 / 👍👎 / 提示词透镜 / 官方 AP 投影 /
  交互证据 / 作品与过程。
- **GET 永不 spend；POST 是唯一 spend**，失败返回 200 + prose:null，绝不成墙。
- Single-source badges/states server-side (D1 anti-drift).
- Test discipline: card/gate/projection/creation/config/migration changes run
  **FULL** Go packages (`-p 1 ./...`), never `-run` subsets; `internal/api`
  needs ~210s so every backend test dispatch passes `timeout: 600000`.
  Web `npm test` + `npx tsc --noEmit`; contracts `npm test` + `npx tsc --noEmit`.

---

## 9. File map

**New**
- `apps/api/internal/parent/parent.go` — `DBadge`, `AState`, deterministic
  projection assembly (+ `parent_test.go`).
- `apps/api/internal/agent/compose_parent.go` — the flagship composer +
  `ValidateOutput` (+ `compose_parent_test.go`).
- `apps/api/internal/store/queries/parent.sql` — sqlc queries.
- `apps/api/internal/store/migrations/0035_parent_report_prose.sql`.
- `packages/contracts/src/parentReport.ts` (+ test).
- `apps/web/src/console/ParentReport.tsx` (+ test) + static-content constants
  module (e.g. `parentReportContent.ts`).

**Modified**
- `apps/api/internal/api/` — the two new routes + handler
  (`parent_report.go`), wired into the router with existing tenancy middleware.
- `apps/web/src/console/StudentDetailView.tsx` — wire 项目报告 stub, open view.
- `apps/web/src/console/TeacherReportView.tsx` — wire 导出家长版 PDF stub.
- `packages/contracts/src/index.ts` — export the new contract.

---

## 10. Acceptance

For a seeded student with a completed project report (D1 seeded 吴老师 + 9
students with full CASE evals):

1. Teacher opens the student's detail page → clicks 导出家长版·项目报告 → the
   parent report renders with correct D badges (起步/发展/熟练/优秀), numberless
   A three-state chips, and a 生成家长版正文 button (prose not yet composed).
2. Click 生成 → one `parent_report` `llm_call` recorded → the page fills with
   gentled readings, glance, 机会与真实性, advice. Re-open → same prose, **no
   second llm_call** (first-open-wins).
3. The rendered prose contains **no** bare D/A codes, no A-axis numbers, no
   teacher-only constructs.
4. A dim at NA renders 暂无 + 「暂无可计入的证据」, no invented highlight.
5. 下载 PDF triggers the browser print dialog; the printed page shows only the
   report (rail/chrome/control-bar hidden).
6. All three suites green: Go full packages, contracts, web + tsc.
