# Writing-studio batch — 2026-07-31

Branch: `fix/studio-batch-2026-07-31`. Source: a 23-item studio feedback batch + 2 MVP hides from the product owner. Design forks resolved (see below). #23 deferred to its own brainstorm.

**STATUS: all 6 slices committed; full suites green (contracts 306 · web full · Go internal/api 440s + studio/agent/gateway); whole-branch review CLEAN (no Critical/High/Medium — only 4 Low/no-action notes). Awaiting merge/deploy decision.** Deferred features (own passes): #23 writing-room redesign, #8 self-annotate, #9 sentence-select, #18 cards in forming/library, #13 right-bar-auto-fill.

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
- [~] **E · Reading room features** — DONE: #22 the read-turn agent now carries a proposal-derived motivation into reading (no-write fallback in `readingBriefFor` when `reading_reason` is unset + a linked ref; 克制); #7 the "这条来源可信吗？" starter now deterministically summons the CRAAP card (the router was 克制-biased and rarely proposed one). DEFERRED (features needing their own pass, documented below): #18 cards in forming/library (S4-sized — new summon path + HangingCard renderer per surface); #8 self-annotate (needs a clean `reading_note` reference column + sqlc regen — hand-editing generated scan code across every `reference` SELECT * is too risky); #9 sentence-level select (touches shared Annotate primitive: caret-offset→sentence resolution + zh/en boundary detection; also lets #20's guard become sentence-level). Tests: #22 Go test; web reading 34 green.

### Deferred follow-ups (E — captured plans)
- **#9 sentence select**: add `segmentSentences(text)` (split on `。！？；!?`, keep terminator); in Annotate select-mode, resolve the clicked rune offset via `caretPositionFromPoint`/`caretRangeFromPoint` (reuse the rune-walker in `rangeToSpan`) → containing sentence range → `onCreateSpan`; keep drag as fine override + whole-block fallback for single-sentence blocks. Then switch `readingLoop.pickSentence`'s example-guard from block-id equality to sentence-range overlap (retires the #20 single-block special-case). Fork: ASCII `.` over-splits ("U.S.", "3.5%") — decide terminator set.
- **#8 self-annotate**: add a "我的笔记" textarea in ReadingRoom saving via a dedicated `reading_note` reference column (migration + `ReferencePatch`/`referenceDTO` field + `PATCH /references/{rid}`). Avoid overloading `evaluation`.
- **#18 cards in forming/library**: wire cross-phase card proposing (S4 pattern) to the forming coach + library FloatingCoach — a summon path + HangingCard renderer per surface. Larger feature.
- [x] **F · Coach tuning** — #2 posture now favors open questions; #13 anti-repetition (acknowledge already-answered dims, don't re-ask) + a real FINISH signal (formingCoverageNudge tells the coach when the four dims are covered → "开题成形，随时可生成计划"); #17 find_sources surface steer (proactive keywords + 中文/英文期刊 + 知网/Scholar, still 克制); writing-language awareness (new `GetWritingLanguageNode` → projection line, closing B's deferral). Tests: 2 new projection tests + coach/agent green. NOTE: #13's "right bar fills from chat via confirm-chips" is a 铁律-adjacent frontend feature, deferred (prompt-level finish signal shipped).

## Key code anchors (from the code-map)
- Project creation: `apps/web/src/workspace/Directory.tsx:71,94-107`; `apps/api/internal/api/project_create.go:23,42-52`.
- Forming coach: `PlanBlock.tsx` (composer :388, renderRich :541, onToggleLang :255, WorkingPhase :568, Gantt :801, Kanban :739); Go `agent/project_coach.go:26`, `api/projectcoach.go:411` (formingCoverageNudge), `api/workspace_plan_generate.go:42,87` (APPENDS).
- Reading: `studio/reading/ReadingRoom.tsx` (finalize :156-189, STARTERS :88, LensLibrary :470), `primitives/annotate/Annotate.tsx:66` (block-granular select), `api/reading_takeaway.go:222` (finalize), `api/reading_brief.go`, `ReadingBlock.tsx` (library :124, floating coach ignores linkOffer :1133).
