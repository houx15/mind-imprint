# Lite teacher end: owner follow-ups

> **For agentic workers:** REQUIRED SUB-SKILL: use superpowers:subagent-driven-development. Steps use checkbox (`- [ ]`) syntax.

**Goal:** apply the owner's 2026-09-15 answers to the shipped lite teacher end.
- There is no parent end: remove publish and the public link, and export from the teacher editor instead.
- A teacher can hide a 金句 or keyword.
- Rename 老师布置 to 作业.
- Fix the 进行中 chip and the tile gap.
- Lock the target on an assigned writing.
- Bring the teacher pages into main's refreshed lite look.

**Architecture:** four sequential tasks, a final whole-branch review, then push. There is one new migration, 0152, which drops the publish columns and adds `hidden`.

**Spec:** `docs/superpowers/specs/2026-09-14-lite-teacher-end-design.md`. Where it conflicts, the owner answers below win: §7.3–7.5 publish, share and student inbox are superseded.

**Scope survey with file:line pointers:** `.superpowers/tmp/followup-scope.md`, which is git-ignored and read by the implementers.

## Owner answers (binding, 2026-09-15)

1. A teacher can hide a 金句 or keyword from a parent report.
2. There is no parent end. No publish button and no link: the teacher exports the report and sends it to parents herself.
3. A student should not be able to change the target of an assigned writing.
4. Change 老师布置 to 作业. Fix the 进行中 chip. Fix the odd tile gap.
5. Follow main's changed UI.

## Rulings

- **R1 · Remove publish end to end.** Drop the teacher publish and revoke routes, the public GET, student GET and seen, the report items in the student inbox, the `/r/` and `/parent-reports/:id` student routes and pages, and the nginx `/r/` block.
  - New migration `0152` drops `status`, `share_token`, `published_at` and `student_seen_at` from `lite_parent_report` (use `IF EXISTS`). The Down re-adds them with the 0151 definitions.
  - Assignment inbox JSON must stay byte-identical to plan 2.
- **R2 · Keep these.**
  - `newShareToken` (shared by reading and writing reports), `useNoIndex` (used by `/p/`), and `publishedMonthDay` (lists use it for `createdAt`).
  - The `student_left` 409 on generate and redraft.
  - Editing and export stay allowed after the student leaves.
  - Every report is now simply a draft. Remove the `status='draft'` SQL guards and the `already_published` / `report_empty` codes.
- **R3 · Export in the teacher editor.**
  - An icon button `导出图片` in the editor header runs `saveAll`, then `exportPoster(posterRef.current, "学习报告-{studentName}.png")`.
  - `ParentReportPoster` is rendered in an offscreen WRAPPER, never with the offset on the node, and keeps its light hex palette.
  - The poster uses the current edited body and the visible facts (R4).
  - Failure shows `导出失败：{message}`.
- **R4 · Hidden items, addressed by text.**
  - 0152 adds `hidden jsonb NOT NULL DEFAULT '{}'`, shaped `{"moments": [quote…], "keywords": [text…]}`.
  - PATCH accepts an optional `hidden` object alongside `body`. Every entry must match an existing moment quote or keyword text in the frozen facts; otherwise 400 `invalid_hidden`.
  - The DTO carries `hidden`.
  - A pure helper `liteparent.VisibleFacts(f, hidden)` returns facts without the hidden items. The view, the poster, `SectionsWithFacts` (the interests section disappears when every keyword is hidden, while its stored body text is kept) and the redraft composer (FactsText, Corpus, Titles) all use visible facts.
  - Generate starts with nothing hidden.
  - The editor shows a toggle beside each 金句 and keyword (`隐藏` / `显示`). A hidden item is dimmed in the editor list and absent from the preview and the export.
- **R5 · Copy.**
  - The byline is `由 {teacherName} 撰写 · {M月D日}`, using the report's `createdAt`, in both view and poster.
  - The generate dialog lead replaces `发布前可以修改` with `导出前可以修改`.
  - Remove every `发布`/`链接` wording that referred to the parent report. `发布作业` in the assignment form means publishing an assignment; keep it.
- **R6 · Rename 老师布置 → 作业, UI only.**
  - Strip title and aria-label: `作业`.
  - Room line: `作业 · 截止 {date}`.
  - Inbox lead: `作业会出现在这里。`
  - ItemPage: `驱动问题（作业）`.
  - Teacher rail item `布置` becomes `作业`, matching the page title.
  - Setup modal placeholder: `比如：想写的角度、需要注意的地方……`.
  - Go prompt strings stay as they are, because the 老师布置 wording tells the model the text is the teacher's.
- **R7 · Chip.**
  - `statusChipStyle` 进行中 uses text `var(--mk-accent-700)` on a `color-mix` of accent-500 at 14% over surface. `accent-700` is the token lite dark mode brightens.
  - Check the other four statuses in student dark mode and teacher light mode, and adjust any below 4.5:1 the same way.
  - A logic test pins the style per status.
- **R8 · Tile gap.** Parent report only: the 作业 tile takes the full row (`grid-column: 1 / -1`), so the grid never leaves an empty cell. Student reading and writing reports are unchanged.
- **R9 · Lock the assigned target.**
  - Server:
    - `PUT /writings/{id}/setup` on an assigned writing (`assigned_prompt` set) keeps the stored lang and target_words, whatever the body says, but still stamps `setup_at` and saves the note.
    - `PUT /writings/{id}/target-words` on an assigned writing returns 409 `assigned_target_locked`, message `作业的字数要求由老师设定`.
  - Frontend:
    - For an assigned writing the setup modal shows `语言`/`目标字数` read-only with the note `作业要求由老师设定`, and hides `都可以之后再改`.
    - `LengthMeter` is read-only.
  - Tests cover both handlers and a pure `isAssignedWriting` helper.
- **R10 · Teacher look follows main's refreshed lite UI.**
  - The teacher shell mounts inside the lite theme scope, so teachers get the lite tokens and dark mode. Reuse `.lite-student`/`body.lite-student-theme`, or add a `.lite-teacher` selector next to them in `lite.css`, whichever changes less. Teachers use `LITE_ACCENT_PRESETS`, and a stored pro accent id coerces to the lite default.
  - The teacher rail uses the `learning-nav` markup and classes: paper rail, accent-tinted active item.
  - Teacher pages use one shared page wrapper, consistent with the landing measure widths, and the refreshed card treatment. No redesign of layouts.
  - The pro console views mounted in the teacher shell (`@/console/*`, SettingsView) must render acceptably under the lite scope. Check them in the browser; do not edit `apps/web`.
  - If the owner pushes further UI changes to main before this task starts, merge origin/main first and follow them.

## Global constraints

- **Pro safety.** Lite must never break pro: `apps/web` is unchanged, and shared Go packages and queries change only additively, except the lite-only parent report files.
- **Copy.** Follow AGENTS.md § 界面文案怎么写 (noun labels, `{动词}失败：{message}`, no literary style).
- **Frontend rules.**
  - `noUncheckedIndexedAccess`.
  - No Tailwind alpha modifiers on `mk-*` (use `color-mix`) and no left colour bars.
  - Every page works at 400px.
  - Async loads use a cancelled flag.
- **Tests.** Logic tests only. UI is verified with real browser screenshots at 1440×900 and 400×800, light and dark.
- **Privacy (owner).** Teachers never see the chat. The report never carries teacher text as her words.
- **Model calls.** Model errors surface and are never faked. Every model call is recorded.

## Tasks

### Task 1: Backend: remove publish, add hidden items (R1, R2, R4 server side, R5 server side)
Files: migration `0152_lite_parent_report_export_only.sql`; `store/queries/lite_parent_report.sql` + sqlc; `api/lite_parent_report.go`; delete `api/lite_parent_report_read.go`; `api/lite_teacher_routes.go`; `api/api.go`; `api/lite_student_assignments.go`; `liteparent/facts.go` (`VisibleFacts`); tests.
- [ ] Migration 0152. Queries lose the publish, student and share queries and the status guards. The list queries drop status and token columns. Regenerate sqlc.
- [ ] Handlers:
  - Remove publish, revoke, the read file, and the student routes.
  - PATCH accepts `hidden`, validated per R4.
  - DTOs drop status, shareToken, publishedAt, studentSeenAt and shared, and add `hidden`.
  - Redraft composes from `VisibleFacts`.
  - `SectionsWithFacts` uses visible facts.
- [ ] Inbox: only assignment items remain, and assignment JSON is byte-identical. Keep the key-set test.
- [ ] Tests:
  - Remove the publish, public and student tests.
  - Add `VisibleFacts` table tests.
  - Add PATCH hidden: valid, invalid entry → 400, all keywords hidden removes the interests section.
  - Add: redraft with a hidden quote gives a prompt without that quote, and a draft quoting it fails the check.
  - The migration applies up and down.
  - Run the full `go test ./...`.
- [ ] Commit `refactor(lite-teacher): 家长报告只导出不发布，老师可隐藏金句与关键词（后端）`.

### Task 2: Frontend: remove publish UI, export and hide in the editor (R1, R3, R4, R5)
Files: delete `parentReport/PublicParentReportPage.tsx` and `StudentParentReportPage.tsx`; `routing.ts`+test; `rootElementFor.tsx`; `LiteApp.tsx`; `api/parentReports.ts`+test; `api/assignments.ts`+test; `inbox/*`; `nginx.conf`; `teacher/ParentReportEditor.tsx`; `teacher/parentReportLogic.ts`+test; `teacher/ParentReportsPage.tsx`; `teacher/StudentPage.tsx`; `parentReport/ParentReportView.tsx`, `ParentReportPoster.tsx`, `view.ts`; `teacher/GenerateParentReportDialog.tsx`.
- [ ] Remove the routes and pages, the inbox report rows and union branch, the publish, revoke and link UI, the status and link columns, and the nginx `/r/` block.
- [ ] Editor:
  - `导出图片` with an offscreen poster wrapper.
  - Toggles on each 金句 and keyword, which PATCH `hidden`.
  - Preview and poster use visible facts.
  - Byline and dialog copy per R5.
- [ ] Logic tests: hidden filtering helper, normalizers, routing, inbox.
- [ ] Browser walk:
  - Generate is seeded in SQL.
  - Hide a 金句 and a keyword: the preview updates.
  - Export: open the PNG and confirm it is not blank and the hidden items are absent.
  - Visit `/r/x` and `/parent-reports/x`: they fall back to explore or 404 per the routing.
  - Screenshots at 1440 and 400, light and dark.
- [ ] Commit `refactor(lite-teacher): 家长报告编辑页导出图片与隐藏金句，去掉发布与公开链接`.

### Task 3: Small fixes (R6, R7, R8, R9)
Files: `inbox/AssignmentStrip.tsx`, `AssignmentLine.tsx`, `InboxPanel.tsx`; `teacher/ItemPage.tsx`, `LiteTeacherShell.tsx` (rail label); `writings/WritingSetupModal.tsx`, `WritingRoomHost.tsx`; `teacher/assignmentLogic.ts`+test; `parentReport/ParentReportView.tsx` (tile); `api/writing_setup.go`, `api/writing_stage.go` + tests.
- [ ] Rename per R6.
- [ ] Chip per R7, with the contrast check noted in the report.
- [ ] Tile per R8.
- [ ] Target lock per R9, server and client.
- [ ] Tests; browser screenshots of the strip, room line, inbox, chip in dark mode, parent report tiles, and the assigned writing setup modal.
- [ ] Commits: focused `fix(lite…)`.

### Task 4: Teacher look (R10)
Files: `LiteApp.tsx`, `teacher/LiteTeacherShell.tsx`, maybe `apps/web/src/ui/themes/lite.css` (selector only, additive), the teacher pages' wrappers.
- [ ] Merge origin/main first if it has moved.
- [ ] Scope and presets, rail markup, page wrapper.
- [ ] Browser walk of every teacher page at 1440 and 400, light and dark, including the pro console views in the shell. Compare against the student pages' look.
- [ ] `apps/web` diff stays empty. `apps/web/src/ui/themes/lite.css` is under `apps/web`; if a selector change there is unavoidable, it must be additive and pro tests must still pass. Record it as a deviation.
- [ ] Commit `style(lite-teacher): 教师端沿用 lite 新版界面`.

### Final
- [ ] Whole-branch review, one fix wave, and pre-push checks (fetch; main moved?; migrations free; no deletions except the intended files; pro suites).
- [ ] Push to main.
- [ ] Update `docs/2026-09-15-lite-teacher-end-job-report.md` with the owner answers and these changes.
