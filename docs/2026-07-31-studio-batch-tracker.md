# Writing-studio batch — 2026-07-31

Branch: `fix/studio-batch-2026-07-31`. Source: a 23-item studio feedback batch + 2 MVP hides from the product owner. Design forks resolved (see below). #23 deferred to its own brainstorm.

## Design decisions (locked)
- **#1 project creation** → capture the **assignment prompt** (multi-line) + a **project-type selector**. The refined research question emerges in forming.
- **#4 writing language** → set **at creation** (中文 / English / 双语); coach adapts, export follows.
- **#13 forming** → add a **finish signal** (coach recognizes the four parts are covered; right panel fills as each solidifies; offers 收尾 → 生成计划).
- **#23 writing room** (3 tabs + materials sidebar) → **deferred** to its own design pass after this batch.

## MVP hides (done)
- 聊天 rail tab hidden (`LeftRail.tsx`); chat surface + backend intact, default tab = studio.
- 能力素养 tab hidden in 成长报告 (`GrowthReport.tsx`); `AbilityModel` + `/growth/ability` intact.

## Slices
- [x] **A · Forming composer bugs** — #3a taller input · #3b preserve newlines (pre-wrap) · #5 lang-toggle keeps history · #12 IME-safe Enter (all 3 composers). Tests: shell 16 green; web tsc clean.
- [x] **B · Project creation** — #1 assignment-prompt (multiline) + type selector · #4 writing-language selector · type now persists as `qualification` (was discarded), writing language → `writing_language` graph node. Coach *awareness* of the language lands in F. Tests: go create suite green, contracts 306, web tsc clean.
- [x] **C · Plan board** — #15 regenerate now REPLACES the board (new `DeletePlanItemsByProject` in the generate tx) behind a confirm modal (`RegenConfirm`) that only fires when a plan already exists · #14 plan timeline anchored to `project.createdAt` (added to the lean workspace projection): gantt header shows calendar dates + weekday, a today column/line, kanban cards show their date window. Tests: new replace test + studio/api/contracts/web all green.
- [x] **D · Reading room bugs** — ROOT CAUSE: #10/#11/#16/#21 all shared ONE origin — an empty reading record serialized `findings/keyQuotes: null`, which the client `z.array` contract rejects, crashing the finalize response parse AND every later whole-library parse. Fixed: non-nil slices at finalize construction + heal legacy null rows in `toReferenceDTO` + per-row-resilient `getLibrary` (one bad row can't blank the list). #11 modal auto-closes on success. #19 the reading floating coach now surfaces the link-add chip (was dropping `linkOffer`). #20 single-block (abstract-only) articles no longer freeze in select-mode (block-guard only applies when another block exists). Tests: new nil-slice regression pin; api + web reading (34) green.
- [ ] **E · Reading room features** — #8 self-annotate · #9 sentence select · #22 carry motivation · #7/#18 cards appearing.
- [ ] **F · Coach tuning** — #2 open questions · #13 verbosity/repetition + finish signal · #17 proactive keyword/database help.

## Key code anchors (from the code-map)
- Project creation: `apps/web/src/workspace/Directory.tsx:71,94-107`; `apps/api/internal/api/project_create.go:23,42-52`.
- Forming coach: `PlanBlock.tsx` (composer :388, renderRich :541, onToggleLang :255, WorkingPhase :568, Gantt :801, Kanban :739); Go `agent/project_coach.go:26`, `api/projectcoach.go:411` (formingCoverageNudge), `api/workspace_plan_generate.go:42,87` (APPENDS).
- Reading: `studio/reading/ReadingRoom.tsx` (finalize :156-189, STARTERS :88, LensLibrary :470), `primitives/annotate/Annotate.tsx:66` (block-granular select), `api/reading_takeaway.go:222` (finalize), `api/reading_brief.go`, `ReadingBlock.tsx` (library :124, floating coach ignores linkOffer :1133).
