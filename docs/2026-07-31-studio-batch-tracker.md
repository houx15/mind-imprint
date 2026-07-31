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
- [ ] **B · Project creation** — #1 assignment-prompt + type selector · #4 writing-language · fix hardcoded `0457` qualification (`project_create.go:52`).
- [ ] **C · Plan board** — #15 regenerate confirm + replace (not append) · #14 gantt/kanban calendar dates + today line.
- [ ] **D · Reading room bugs** — #10 归纳 422 · #11 modal auto-close+return · #16/#21 empty library · #19 link-add in reading coach · #20 lens-read breakage.
- [ ] **E · Reading room features** — #8 self-annotate · #9 sentence select · #22 carry motivation · #7/#18 cards appearing.
- [ ] **F · Coach tuning** — #2 open questions · #13 verbosity/repetition + finish signal · #17 proactive keyword/database help.

## Key code anchors (from the code-map)
- Project creation: `apps/web/src/workspace/Directory.tsx:71,94-107`; `apps/api/internal/api/project_create.go:23,42-52`.
- Forming coach: `PlanBlock.tsx` (composer :388, renderRich :541, onToggleLang :255, WorkingPhase :568, Gantt :801, Kanban :739); Go `agent/project_coach.go:26`, `api/projectcoach.go:411` (formingCoverageNudge), `api/workspace_plan_generate.go:42,87` (APPENDS).
- Reading: `studio/reading/ReadingRoom.tsx` (finalize :156-189, STARTERS :88, LensLibrary :470), `primitives/annotate/Annotate.tsx:66` (block-granular select), `api/reading_takeaway.go:222` (finalize), `api/reading_brief.go`, `ReadingBlock.tsx` (library :124, floating coach ignores linkOffer :1133).
