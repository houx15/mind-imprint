# Job-end report — lite teacher end (draft)

Branch: `worktree-lite-teacher-end`. Four plans, built in order: student data, assignments, weekly summary, parent report. Plan 4 is still in its final whole-branch review; this draft reflects the ledger as of now and may need updates once that review lands.

---

## 1. What was built

**Plan 1 — student data views.** A teacher now sees a class roster (per-student active time, AI turn count, reading/writing/project counts, overdue-assignment count, last-active date) and can open a student page showing her readings, writings and projects with status and time/turns, plus her interest tree in a read-only mode. Opening one item shows only her outputs and moments — highlights, notes, takeaway, lens-card fields, draft text, AI comments, report 金句 — never the raw chat transcript with 印记. Project time is now tracked (a heartbeat endpoint was added; before this, PBL active time stayed at zero). A teacher shell was added inside the lite app, reusing pro's class/teacher/admin console views.

**Plan 2 — practices with deadlines.** A teacher can assign a reading, writing or project to a class or to picked students, with a due date, from a form under 布置. A student sees it as a strip above 阅读/写作/项目 and in an inbox with an unread red dot; opening it starts the work. Settings lock once a student has started (a concurrent teacher edit cannot silently change what she's working on). Deleting (archiving) an assignment is idempotent. When a student starts an assigned item, the teacher's prompt or driving question is stored and labelled as the teacher's text (assigned_prompt / assigned_brief / assigned flag) — it is never recorded as something the student said, so later features (report 金句, coaching prompts) cannot put teacher words in her mouth.

**Plan 3 — weekly summary.** Each student gets a 上周表现总结 card (and each class a 周报) on the teacher's side: computed facts (active time, AI turns, finished counts, assignment completion rate) plus a short AI-written summary, generated once per (student, week) or (class, week) and cached. The AI text is checked before it's shown: every quoted phrase must be a real substring of her words (not the teacher's assigned prompt, not an invented quote), every digit must trace back to a real number, and item titles are checked separately from her own quotes so a teacher-set assignment title can never pass as something she said. Praise and watch cards flag students who need attention.

**Plan 4 — parent report.** A teacher picks a student and a date range (default last 4 weeks), and the AI drafts a report from the same fact/quote/digit-checked pipeline as plan 3. The teacher edits the draft, can redraft, and then publishes a revocable public link (no login required) that a parent can open; the same report also appears, read-only, in the student's inbox. The report can be exported as a PNG poster. Publishing, revoking and re-publishing manage a share token; a student who has since left the class can still be read, revoked or PATCHed by her original teacher, but a new report cannot be generated for her.

---

## 2. Where it lives

- Branch: `worktree-lite-teacher-end`.
- **Plan 1**: pushed. `PLAN 1 PUSHED: origin/main fast-forwarded to 47cf09ae` (fetch showed no new commits on main; only migration diff was plan 1's own 0148).
- **Plan 2**: pushed. `PLAN 2 PUSHED to origin/main (HEAD after the ClassPage fix); backup branch updated.` (This push followed a merge of main's lite visual refresh into the branch, plus one ClassPage width fix.)
- **Plan 3**: pushed. `PLAN 3 PUSHED to origin/main (fast-forward), and the backup branch was updated.`
- **Plan 4**: pushed. All 7 tasks, the final whole-branch review (With fixes), one fix wave (Ruling 18) and a scoped re-review (Ready to push: Yes) are done. The pre-push check was clean: main unchanged, migration 0151 free, no deletions, `apps/web` untouched. Fast-forwarded to `origin/main` at `d90931f1`.
- All four plans are now on `origin/main`. The backup branch `worktree-lite-teacher-end` on origin points to the same commit.

---

## 3. Every Ruling, per plan

### Plan 1 — student data (Rulings 1–11)

| Ruling id | Decision | Why | Cost if wrong |
|---|---|---|---|
| P1 Ruling 1 | Task 8's stub `StudentPage`/`ItemPage` must already export the final props Task 9 uses. | So Task 9 never has to edit the shell. | One extra shell edit in Task 9. |
| P1 Ruling 2 | *(superseded by Ruling 3)* PBL route `{id}` assumed to be `pbl_project.id`, returned alongside atom id from the test helper. | Kept the plan 2 heartbeat test moving before schema was fully checked. | Test 404s and the implementer swaps the two ids. |
| P1 Ruling 3 | `pbl_project` has no `id` column — its PK is `atom_id`, so project id == atom id (supersedes Ruling 2's wording; behaviour unchanged). | Verified against the schema by the implementer. | None — verified. |
| P1 Ruling 4 | Teacher report slice passes `prosePending bool`; `reportError` is set only when `ensureAtomReport` itself errors. The teacher ItemPage re-fetches once on `prosePending` and shows 报告文字生成中, then 报告文字暂未生成 with no error prefix if still pending. | Mirrors the student ReportPanel protocol; keeps the teacher request fast (no inline ~150s model call). | Teacher sees moments one refresh later. |
| P1 Ruling 5 | Fix the frontend lens type from `{cardId, fields}` to `{title, fields}` to match the Go handler, and render the title. | The backend is the contract other tasks already verified against. | None — type-only change plus one label. |
| P1 Ruling 6 | Under `readOnly`, `KeywordDrawer` source rows render as non-interactive text instead of navigating. | The drawer has no classId/userId to build a teacher item route; a dead-end click is worse than plain text. | Teachers can't jump from a keyword source straight to the item; they reach it via the item list instead. |
| P1 Ruling 7 | Task 10's implementer runs the browser walk and pro-safety suites and commits fixes, but does not rebase or push; the controller pushes after the final review. | The final review must see the branch before it lands on the shared branch. | Plan 1 reaches main one review later. |
| P1 Ruling 8 | 金句 in reports quoting the student's own chat turns verbatim is acceptable under DEC-1 (moments are her words by construction), but must be surfaced to the owner for confirmation — extended to also cover tree `keyword_source.evidence`. **Closed by owner decision 2026-09-14:** keep both; teachers are banned only from directly viewing the chat with 印记, not from her reading notes, writing results, 金句 or tree evidence. | Owner choice, asked in chat. | Owner might have wanted moments restricted to non-chat sources — resolved, no longer open. |
| P1 Ruling 9 | Final fix design: (F1) link renders only for http/https, backend rejects other schemes; (F2) teacher item GET never enriches prose by default, `?prose=1` opts in; (F3) `?prose=1` checks `HasEntitlement` first; (F4) readOnly copy fixed to teacher-neutral wording; plus skip the zero-second bucket upsert, hide a duplicate takeaway line, add roster tests across the week boundary. | All small, security- or correctness-motivated fixes found by the final review. | Small rework. |
| P1 Ruling 10 | Accept that a scheme-less URL (e.g. `example.org/a`) is rejected with 400 `请输入以 http 或 https 开头的链接` rather than silently prefixed with `https://`. | Rare input, actionable message; guessing a scheme risks storing a wrong address. | A student retypes the link with `https://`. |
| P1 Ruling 11 | Keep plan 1's SDD ledger until the whole job ends instead of deleting it once plan 1 is done. | The final job-end report must list every Ruling across plans 1–4. | A gitignored directory lingers a few hours. |

### Plan 2 — assignments (Rulings P2-1–P2-18)

| Ruling id | Decision | Why | Cost if wrong |
|---|---|---|---|
| P2-1 | PATCH on a started assignment returns 409 only when kind or payload *actually changes* (value comparison), not merely present. | Spec intent is to stop changing what a student already started; forms resend unchanged settings. | A client expecting 409 on an identical payload gets 200 instead. |
| P2-2 | `/start` reads the assignment inside the same transaction that locks the recipient row (`SELECT … FOR UPDATE`, then read via `qtx`). | So a concurrent teacher PATCH either commits before the student starts, or is refused by the started check. | A student could start with pre-edit settings in a narrow window. |
| P2-3 | A past `dueAt` is accepted on create/patch. | Teachers correct dates; status derivation already handles a past due date. | An assignment can be created already overdue. |
| P2-4 | A repeat DELETE (archive) on an already-archived assignment returns 404 (shared loader treats archived as not-found); the SQL itself is idempotent. Accepted; teacher UI treats 404-after-archive as success. | The loader's not-found semantics apply uniformly. | A client retrying archive sees 404 instead of 204. |
| P2-5 | Change the start-path failed-link-fetch message from "抓不到正文，请直接粘贴" to 开始失败：链接无法读取正文，请告知老师更换阅读材料. | The instruction must be doable where it appears — the student has no paste box on an assigned link reading. | Copy tweak only. |
| P2-6 | Concurrent starts of two different library assignments sharing slug+tier can 500 on the second request (no orphan created; retry succeeds). Accepted for now. | Needs two assignments at the identical article/level plus a same-millisecond double start — rare. | A rare 500 that a retry clears. |
| P2-7 | Fix for a locking gap: `/start` reads the assignment `FOR SHARE` inside its own locked transaction; PATCH reads the assignment `FOR UPDATE` before counting started recipients. | So a PATCH can no longer commit a new payload while a start is mid-flight, bypassing the 409 edit lock. | Extra lock contention on one row per assignment. |
| P2-8 | Consistent lock order (assignment → recipient) in both start and PATCH; PATCH decides everything from its locked row read; two deterministic concurrency tests added. | Closes a deadlock and a stale-read bug found in P2-7's own review. | None beyond slightly longer PATCH lock hold. |
| P2-9 | No contract mismatch on the for-atom endpoint: it returns 200 `{assignment:null}` on no-rows and 404 only for a malformed uuid or non-lite school; an implementer's "404 on every room open" report was an inference, not an observed bug. | Controller verified by reading the route registration and handler directly. | Noisy 404s in devtools with no visible effect — cosmetic only. |
| P2-10 | *(preliminary; superseded by P2-12)* Assignment start must never record teacher text as student words: writing prompt goes in as a system message, project driving question as teacher context, no auto-post of `idea` as her first turn. | 过程即数据 and the owner's visibility rule both depend on "her words" meaning her words. | Extra start-path branching and a small ProjectRoom change. |
| P2-11 | Accept a walk-report formatting deviation (1440px and 400px observations combined per row instead of one row per screen × width) without a fix round. | Both widths' screenshots exist and were spot-checked; restructuring changes no verification and no code. | A future reader has to scan width-specific notes inline. |
| P2-12 | *(supersedes P2-10)* Origin is recorded on the item itself: `writing.assigned_prompt`, `pbl_project.assigned` + `assigned_brief`. Writing start: title = assignment title, no student/system message with the prompt; prompt builders render 老师布置的题目. Project start: name = assignment title, `idea` keeps the driving question, `assigned=true`; coach/lookback render 老师布置的驱动问题 + 老师补充说明; harvest skips `idea`; ProjectRoom doesn't auto-post when assigned; teacher ItemPage labels it 驱动问题（老师布置）. No `IsAssignedAtom` query needed. | The marker survives archive, class deletion and a failed lookup, and teacher text stays visible to her and 印记 without being put in her mouth. | Two columns to move later if the design changes. |
| P2-13 | One connection per start: read assignment/recipient unlocked first (return early if already started); do network/model work before `Begin`; then lock via `lockAssignmentStart`, re-check payload unchanged (409 `assignment_changed` if it changed); create and commit in one tx. | Removes pool starvation, an 8s lock hold during a URL fetch, and orphan-item paths found by the final review (finding C1). | A rare 409 on a teacher edit racing a start. |
| P2-14 | `instructions` shows as one muted, 2-line-clamped line in the inbox and strip, capped at 2000 runes server-side (400 on create/patch). Project description reaches the coach through `assigned_brief`. Writing/reading instructions are UI-only. | The teacher's instructions field has to go somewhere visible (finding I3). | Copy placement only. |
| P2-15 | A reading URL start passes a blank title so the fetched title/lang are used; a text-source start keeps the assignment title. Start and inbox require current enrollment in the assignment's class. Deferred: the P2-6 delete-race 500, and the assigned-writing setup-modal target override (listed as a product question). | Small correctness fixes bundled from the final review's minors. | Copy/placement only. |
| P2-16 | Controller fixed two findings directly instead of a second fix wave: `writingOpeningSystemFor` swaps "her topic" sentences for teacher-labelled ones when `AssignedPrompt` is set; the lookback fallback branches on `Assigned`. M-2 and M-3 parked. | The residual was two strings plus asserts — cheaper than a new dispatch. | The model reads the assigned opening slightly differently; not validated live (no model key locally). |
| P2-17 | Merge `origin/main` into the branch instead of rebasing (main had moved 18 commits via a lite visual refresh + copy pass while plan 2 was in flight). Main's design/wording win; the branch's features are re-applied on top. | 24 commits over 13 conflicting files would re-conflict commit-by-commit under a rebase; main's history already has merge commits. | One extra merge commit in main's history. |
| P2-18 | (1) Fixed directly: `ClassPage` container widened to `max-w-[1120px]` (roster 操作 column was clipped at 1440px). (2) Accepted: the 老师布置 strip sits above the header on reading/writing but below it on projects, because main owns the landing layout — listed for the owner. (3) Accepted: inbox button sits in main's `learning-nav-bottom`. (4) Deferred minor: 进行中 chip is dim in dark mode. | Post-merge visual audit findings. | Cosmetic only. |

### Plan 3 — weekly summary (Rulings 1–18)

| Ruling id | Decision | Why | Cost if wrong |
|---|---|---|---|
| P3 Ruling 1 | Add a pure `liteweekly.ClassWeekStats` + `ClassFactsText(...)` shared by both the prompt and the digit check. | The digit check needs one canonical class facts text, not a duplicate. | A small move of the function later. |
| P3 Ruling 2 | In the loader, a recipient's `finishedAt` counts toward `liteassign.Status` only when it's before `weekEnd`; otherwise nil, so work finished after the week still counts as overdue for that week. | Spec §6.2 defines "overdue" as "已逾期 at week end." | Historical overdue/late counts shift for that one week only. |
| P3 Ruling 3 | Fills in open brief details for quote/digit/facts-text rules and error wording in `CheckProse`. | The brief left these edge cases open; each open case is either a false rejection or a leak in the allowed digit set. | Some prose rejected or accepted at the edges — absorbed by retries. |
| P3 Ruling 4 | `ProseCheck` splits into two corpora: `Corpus(s)` for her words (checks 「」“”), new `TitleCorpus(s)` for item titles (checks 《》). Prompts say 作品标题用《》，只有学生原话用「」. | After P2-12, an item title can be teacher text; a single shared corpus would let a teacher's title pass as her words. | One extra field to thread through composers. |
| P3 Ruling 5 (T5) | Model = `ClassAssess` via `detachedModelCtx`; every attempt recorded under `lite_student_weekly`/`lite_class_weekly`; `OtherNames` = roster minus her; concurrent double-POST may double-spend then dedupe on insert (`DO NOTHING` + re-read, no locks); prose keyed per user, not per class; a stored row returns before the entitlement check. | Matches pro's existing weekly pattern and plan 2's accepted patterns; locking a rare double-spend isn't worth the complexity. | An occasional duplicate model charge; a two-class student's prose can mention the wrong class's assignment counts. |
| P3 Ruling 6 (T6) | Implementer does not push; the gated `LIVE_LLM` test must compile and skip (recorded 未运行 without a key, listed pending for the owner); the walk checks tokens against main's refreshed token system and screenshots the failure state without a key. | Plan text predated the controller-push rule and the missing model key. | Prompt quality stays unverified until the owner runs it live. |
| P3 Ruling 7 | Any unmatched opening/closing 「」“”《》 fails `CheckProse` with a named error; an empty 「」 is allowed; nested marks of another family inside a span stay literal. | Truncated model output is a known failure mode (2026-09-08/09-10 lessons); the check exists to catch invented quotes. | An extra retry on a model reply with a typo'd bracket. |
| P3 Ruling 8 | Accept keeping both the table-driven nil-codes test case and the older standalone one (duplicate) rather than removing either. | The fix message explicitly allowed keeping the separate function. | One duplicate test. |
| P3 Ruling 9 | Fix round: (a) `COALESCE` fixes a `NULL` bug that silently dropped every in-progress project from "stalled"; (b) stalled activity uses a fixed 7-day pre-week-end window, not the live `last_activity_at`; (c) empty titles fall back through the assignment title, then a kind label; (d) a quote is dropped only if it equals or is a ≥8-rune substring of the assigned prompt; (e) one malformed report is logged and skipped, not fatal to the class load; (f) new boundary/two-student/null-moments tests; (g) stalled-evidence card copy changes to 《title》超过 7 天没有进展。. | (a) is a correctness bug; (b) keeps facts recomputed-on-view consistent with once-stored prose; (g) avoids presenting a teacher title as her words (would loop `CheckProse` retries). | (b) is slightly more SQL than using the activity column; (g) changes the wording of one teacher-facing card. |
| P3 Ruling 10 | Per-field `CheckProse` and a separate class names list are accepted (stricter, keeps id digits out of facts); no retry on a transport error is accepted (surfaces, UI has 重试); the card-key-with-no-student edge case is accepted as unreachable. | T5 builds cards only from already-loaded students. | One extra teacher click after a transient network failure. |
| P3 Ruling 11 | Fix round: (a) `ProseCheck.FactsText` = `FactsText` + every card's evidence text, so faithful prose restating a card's own digits (e.g. "超过 7 天") isn't rejected; (b) drop the "不写学生名单以外的学生" prompt clause (`OtherNames` was empty, contradicting it); (c) an unmatched card key is now an error, not a silent nameless card; (d) added a parse test for text around the JSON; (e) one minor skipped. | (a) is a correctness bug that fails real weeks whose label happens to lack a "7". | Evidence digits are allowed in the summary, but they were already true facts. |
| P3 Ruling 12 | A routing failure returns 502; a transport error surfaces as a raw-English `proseError` with 200 (nothing stored, attempt recorded, UI has 重试). | A missing model binding is a server fault; a failed attempt is data worth keeping, not hiding. | A teacher sees English error detail. |
| P3 Ruling 13 | `proseError` stays raw on the server; T6 prefixes it as 总结生成失败：{proseError} (copy rule 8 — 后台原话); `assignmentRate` rounds down. Other minors deferred to the final review. | Consistent with plan 1's server-side prefixing pattern. | Copy placement differs from plan 1's item report; the rate can read 1 point low. |
| P3 Ruling 14 | (1) A student with both a praise and a watch card shows the lead/action once, under 需要建议 only; (2) tile label becomes 活跃学生 (drops 本周, since the week is already in the title); (4) the date part of the class title gets `whitespace-nowrap` so it never breaks at 400px; (3)/(5) just noted for the record. | Copy rule 1 (labels are nouns) plus a real 400px layout bug. | Layout choices only. |
| P3 Ruling 15 | Accept that an empty student week sends no POST at all (no facts to summarise, no spend). | Nothing to summarize; avoids a wasted model call. | An empty week shows 该周没有学习记录 with no prose — expected. |
| P3 Ruling 16 | Final fix wave: (A) name check strips verified spans first, removes her own name before checking, skips classmate names that are substrings of hers, adds her name to the digit facts; (B) server 400s `week_before_start` for weeks before enrollment/class creation, adds an `hasPrev` flag, and skips POST/spend/store on an empty week (server-decided via `IsEmptyWeek`); (C) `archived_at IS NULL OR archived_at >= week_end` so archiving doesn't rewrite past weeks; (D) card copy: label 未使用, evidence uses 该周 not 本周; (E) prompt lines forbidding non-arabic numerals/arithmetic, bracketed keywords, live test uses the production model config; (F) small fixes (error logging, JSX spacing, boundary tests). | (A)/(B) were causing deterministic repeated flagship spend; (C)/(D) were visible contradictions; (E) reduces first-attempt failures with a real model. | The (D) copy differs from spec §6.2's literal wording (本周未使用 → 未使用) — recorded as a spec deviation. |
| P3 Ruling 17 | All fix-wave deviations accepted: span removal alone (no separate title-name skip) handles classmate names in titles; prompt lines appended as new rules 9/10; an empty week still shows tiles/cards (only the summary block changes); 上一周 disables after a failed load; the entitlement check runs after the week is loaded (DB read only); one extra enrollment read added on student routes. | None of the deviations spends tokens or misattributes text. | An occasional failed summary for a bare title that happens to contain a classmate's name — fails safe with a retry. |
| P3 Ruling 18 | Controller fixed two re-review Importants directly instead of a second dispatch: a newline is left where a span is removed (was joining adjacent text); names containing `SelfName` are checked before her name is replaced (was hiding a classmate whose name contains hers, e.g. 王丽 inside 王丽华); plus the `week_before_start` 400 for a brand-new class/student now renders as a muted line with no dead-end 重试, not 加载失败. | A second fix dispatch would cost more than these small, fully-specified changes. | A new class's teacher sees the raw server line 该周早于… instead of prose on the very first week. |

### Plan 4 — parent report (Rulings 1–17)

| Ruling id | Decision | Why | Cost if wrong |
|---|---|---|---|
| P4 Ruling 1 | Extend plan 2's `InboxItemDTO` with `publishedAt *string` for report items; assignment-only fields get `omitempty` where blank on a report row. One discriminated union by `type`. | One inbox list, one unread count, reused across both item types. | A small DTO reshape. |
| P4 Ruling 2 | *(superseded)* Generating a parent report must **not** trigger a student report's inline (phase-2) prose call; facts loading uses the phase-1-only `ensureAtomReport` path; moments for still-pending reports are simply absent from the snapshot; generate/redraft call `HasEntitlement` first; any URL uses `safeHttpUrl`. | Plan 1's own final review found an implicit model call inside a read that could block a teacher up to ~150s and spend without an entitlement check. | Parent reports may omit 金句 for items the student never finished reviewing. |
| P4 Ruling 3 | Carried from plan 3/plan 2: `liteweekly.ProseCheck` keeps separate `Corpus` (her words, 「」“”) and `Titles`/`TitleCorpus` (item titles, 《》); parent facts and the snapshot must never include `writing.assigned_prompt` or an assigned project's `idea`, titles only; frontend uses only tokens that still exist post-merge. | After P2-12 an item title or idea can be teacher text; one shared corpus would let it pass on a page parents actually read. | One extra field in the composer. |
| P4 Ruling 4 | Parent-report inbox items and the student report read route are keyed by `user_id` + published status, **not** current class enrollment. | The report is about her and already sent to her family; it should survive her leaving the class. | A student who left still sees an old report in her inbox — accepted as correct. |
| P4 Ruling 5 | `loadTeacherParentReport` no longer requires `IsEnrolledStudent`: a teacher who owns the report's class can still GET, revoke and PATCH a report after the student leaves; generate/redraft/publish are refused for a student no longer enrolled. | The brief's original rule would leave a public parent link nobody could ever revoke once the student left — revocation has to stay possible for privacy. | A teacher can still see an old report for a student who left their class. |
| P4 Ruling 6 | Generate rejects a range ending before the student joined (400 `range_before_start`, reusing plan 3's lower-bound helper); a range that only partly overlaps enrollment is allowed; an explicit generate for an empty range still runs; `CheckProse` gains `SelfName` and checks names outside verified spans (carried from plan 3 Ruling 16); prompts add 数字一律用阿拉伯数字 / 不做加减和单位换算. | Plan 3 showed facts for periods before enrollment are false; here a teacher clicking generate is a deliberate action, unlike plan 3's automatic POST. | One extra model call when a teacher generates a report for a quiet period. |
| P4 Ruling 7 | Extra queries (`GetLiteParentReportForUpdate`, draft-only guards) accepted; `FactsText` renders dates in both ISO and M月D日 form plus student/class names and a keyword count, never the teacher's name; `Minutes -1` means no buckets in range. | The extra queries serve T3's row locking; dual date forms keep faithful date prose from failing the digit check; names are needed for the `SelfName` digit rule. | A query error surfaces in T2 and gets fixed there. |
| P4 Ruling 8 | Fix round: drop the keyword-count line and per-item M月D日 dates from `FactsText` (both widened the allowed digit set enough that invented numbers passed); `FinishedAt` stays in `Facts` for the UI only; remove the duplicate `ListClassStudentNames` query; tighten the brief wording so `assigned_prompt` may be selected only to filter quotes out, never as a title or quote. | `FactsText` is the only verifiable guard against invented numbers reaching parents. | Prose can no longer cite an item's finish date directly. |
| P4 Ruling 9 | List-numbering handled in two layers: a prompt sentence (列举多条时不编号，每条单独一行) plus a pure `stripListMarkers` applied only to the text passed to `CheckProse` (a 1–2 digit count at a line start followed by 、 is exempt). | A prompt rule alone is soft (lessons of 2026-09-03/09-12); a check that fails on legitimate list formatting wastes flagship calls. | A 1–2 digit number at a line start is exempt from the digit check. |
| P4 Ruling 10 | `loadLiteParentFacts` runs only inside the generate POST, never a GET. | Prevents the N-round-trip loader from firing on every page view. | None currently — enforced by dispatch instructions to T3. |
| P4 Ruling 11 | Accept all 7 deviations in Task 3: shared `liteEnrollmentStart`/`liteEndsAfterStart` helpers; entitlement via a test-only package-var seam; new error codes (`invalid_range`, `section_too_long`, `invalid_body`, `report_empty`); publishing an empty report returns 409 `report_empty`; PATCH merges sections into the stored body; route is resolved before the row is created; generate re-checks enrollment on write after the model call. | `report_empty` stops a blank public page; PATCH-merge protects section autosave; resolving the route first avoids an orphan row on a config fault; entitlement seam is test-only, production unchanged. | A teacher can see raw English detail after 草稿生成失败. |
| P4 Ruling 12 | Fix round: (a) redraft with `replaceBody:false` also writes the draft into the body when the locked body has no non-blank section, fixing a stuck failed-generate → blank-PATCH → redraft path; (b) generate returns 201 with the row + `draftError` when compose fails after the row already exists (was losing the id); (c) an enrollment-lookup `ErrNoRows` maps explicitly to 404; (d) added a test for another school's admin → 404. | (a) breaks the failed-generate recovery path the brief centres on; (b)/(c) are cheap correctness fixes. | A body the teacher deliberately left blank could get filled by a later redraft — she can clear it again, and redraft is an explicit action. |
| P4 Ruling 13 | T6 shows `draftError` as-is when it already contains "失败：", otherwise prefixes 草稿生成失败：(same rule as plan 2's `startErrorText`). Full suite runs in T7's safety step and before the push. | Avoids a doubled "失败：失败：" message. | None if applied consistently. |
| P4 Ruling 14 | Accept all Task 4 deviations: a custom `MarshalJSON` on `InboxItemDTO` keeps plan 2's exact contract for assignment items, `omitempty` applies only to report items; `facts.teacherName` duplicates the visible 由 {teacherName} 发布 line; names come from the frozen report snapshot; POST seen returns 404 for another student's report or a draft. | Preserves plan 2's inbox contract exactly while adding report items. | After a teacher rename, an already-generated report shows the old name — accepted, it's a snapshot. |
| P4 Ruling 15 | Accept all Task 5 concerns: the poster keeps fixed hex colours (not main's theme colours) so parents get a consistent image regardless of viewer theme; `Hero`/`SectionTitle`/`macaron`/`rise` exported from `ReportView` for reuse rather than copied; a new `requestInboxReload()` window event because each `useInbox` holds its own state; odd tile counts leave grid gaps (cosmetic). | Fixed poster colours give a parent-facing artifact a stable look; reuse over copy. | Minor visual unevenness. |
| P4 Ruling 16 | Accept concerns 1 (502-before-row-exists on a missing key, per Ruling 11 deviation 6) and 5 (extra confirm/error copy, per the copy rules) as correct; concerns 2 (in-memory `draftError` lost on reload) and 3 (preview breakpoints follow viewport, not column) are deferred minors; concern 4 (edits to a published report go live on blur with no confirm) follows the brief as written and goes to the owner as a product question. | Matches prior rulings and the copy-style rules; concern 4 is a deliberate brief choice, not a bug. | An edit to a published report reaches parents immediately, with no undo confirmation. |
| P4 Ruling 17 | Task 7's own task-review is folded into plan 4's final whole-branch review rather than run separately. | The Task 7 diff is one gated live-test file; its walk evidence (screenshots, report) is directly available to the final reviewer. | A defect in the live test itself would be caught one step later. |
| P4 Ruling 18 | Final fix wave. **A.** Robots meta on the `/r/` page, an nginx `location /r/` X-Robots-Tag, and `Cache-Control: no-store` on the public API. **B.** Drop any 金句 containing a classmate's name before facts are frozen. **C.** Digit check: quotes, titles and the class name no longer count as allowed digits; reject Chinese-numeral counts (两–九, 十+ + counter word); normalise section keys. **D.** Lists and inbox show snapshot names; tile 未完成 becomes 逾期未完成; editor hint 暂无草稿，请重新生成草稿. | Spec §7.4 requires noindex for a minor's page; B removes the concrete classmate-name leak; C closes silent passes in the only guard against invented numbers; D fixes visible inconsistencies. | A legitimate quote mentioning a classmate is dropped; a legitimate 三篇 costs a retry. |
| P4 Ruling 20 | Accept and defer the re-review's minors: a class name that is only a number hides a count; the numeral regex rejects natural wording (这两周, 第三篇, 三天两头…) and misses 遍/段/节/句/题; the classmate filter only knows current members; redraft does not re-filter moments frozen at generate. The numeral false positives go to the top of the live-run checklist. | Tuning the regex without real model output would trade one guess for another; the live run measures it. | Some generates fail with 草稿生成失败 until the regex is tuned. |
| P4 Ruling 19 | Accept the fix wave's deviations: nginx `/r/` serves `index.html` directly, because plain `try_files` lost the header (verified in a real nginx container); the class name is removed from the prose for the digit check; the numeral check also skips “”; section keys that collide after normalising are an error; the editor hint shows only on draft reports. | Each keeps the rule while avoiding a false rejection or a lost header. | A retry on idioms like 三天两头. |

---

## 4. Deferred minors, per plan

### Plan 1 (38)

- Task 1: `newHeartbeatTestAtom` duplicates `createReadingAtomRow` seeding pattern (atom_loader_test.go).
- Task 1: no integration test for negative-seconds heartbeat writing a 0-second bucket row.
- Task 2: no frontend unit test for `sendHeartbeat("project")` URL (reportsApi.test.ts covers reading only).
- Task 2: `pblFinishedStatuses` is a third literal of `{review,keeping,archived}`.
- Task 2: `postPblHeartbeat` re-parses `{id}` and re-fetches `GetPblProject` although `loadOwnedPblProject` already returned atomID.
- Task 3: roster fields `ActiveDaysThisWeek`, `LastActiveAt`, `ProjectsDone/Total`, `WritingsDone` not asserted by any test.
- Task 3: 13 correlated subqueries per roster row — fine at class scale, revisit if rosters grow.
- Task 4: `TestLiteStudentPageListsItems` does not assert title fallback (name→idea), level, finishedAt/createdAt/lastActiveAt null round-trip.
- Task 5: `steps` nil slice serialises as null when there's no live plan (other lists are `[]`).
- Task 5: `GetAtom` non-NoRows errors collapsed to 404; `GetAtom` redundant with `GetLiteStudentItem` scoping.
- Task 5: `stepsTotal` counts cancelled/revised steps — confirm the denominator.
- Task 5: `reading.source` never null (url is `""` when no source row).
- Task 6: `lite_teacher_tree_test.go:37` doc comment names the wrong test function.
- Task 7: `getStudentPage` items and `getItem` trust the wire shape (no normalizer), unlike `normalizeRosterRow` — an asymmetry that matches the brief's reference code.
- Task 8: `ClassPage.tsx:130-134` null-sort comment contradicts the comparator.
- Task 8: `ClassPage` async loads not cancelled on classId change; `loadRoster` does not reset roster (unreachable today).
- Task 8: copy-join-code discards the clipboard promise — no 复制失败 feedback.
- Task 8: 移出失败 message renders in the header, may be off-screen; not cleared on 取消.
- Task 8: empty rename 保存 silently no-ops.
- Task 8: empty-state text shows 邀请码 when `getClass` failed.
- Task 8: 重试 is a `<span>`; roster rows not keyboard-operable.
- Task 8: rail onClick casts `({view:key} as TeacherRoute)` — should narrow `RailItem.key`.
- Task 8: `localeCompare` without `zh-Hans-CN`; 操作 cell is text-right under a text-left header.
- Task 9: failed `getClass` for the tree header class name is silent (the request itself is justified — no class name in roster/student responses).
- Task 9: writing 要求 shows raw lang/structureKey with no heading and always renders; lens field names show raw ids; unknown stat key renders the raw key.
- Task 9: `stats.filter(value !== 0)` hides legitimate zero stats (not requested).
- Task 9: header finished date is unlabelled (should add 完成于).
- Task 9: `TreeView` header copy 「我的兴趣树」/「你的树刚开始长…」 is student-voiced in the teacher view (pre-existing).
- Task 9: `ItemPage.test.ts` tests a trivial ternary; the normalizer test is the valuable one.
- Task 9: other item slice fields are `""`/`0` rather than null (source always an object, level non-null) — renderer truthiness copes.
- Task 10: pro 教师/班级 views are cramped at 400px (pro components, not edited by this build).
- Task 10: teacher-side tree header, intro line and drawer 「你自己写的」 still student-voiced (same as the Task 9 minor).
- Task 10: earlier-day-in-week bucket not exercised in the walk (the walk ran on a Monday).
- Task 10: 完成于 is conditional inline JSX rather than a named pure helper like `prosePendingLabel`.
- Task 10: `langLabel` accepts null/undefined though `writing.lang` is always a string.
- Final: `lite_teacher_roster_test.go:436` computes `time.Now()` separately from the server — could flake if a run straddles the Beijing Sunday→Monday midnight.
- Final: `ItemPage`'s `showReadingTakeaway` relies on `teacher.ts` casting `keep` without a shape check (server always sends string text today).
- Final: F3's not-entitled/error branches are untested while `HasEntitlement` is stubbed true.

### Plan 2 (47)

- Task 1: payload edge cases (`tier:0`, `https://` with no host, exactly 50000 runes, JSON null, wrong-typed field) correct by trace but untested.
- Task 1: `lite_assignment.created_by` FK has no `ON DELETE` — deleting a teacher with assignments fails (no deletion flow exists yet).
- Task 2: a stamp failure after a successful status write leaves status persisted with an error response (no transaction) — same shape as an existing handler.
- Task 3: no handler-level test for `POST /library/{slug}/levels/{tier}` (201 / 200 resumed / 404 / 400 order).
- Task 3: `TestSuggestLibraryTierForEmptyShelf` covers only the empty case.
- Task 3: dedupe test never asserts a finished reading is not resumed.
- Task 3: `errFetchFailed` wraps with `%v`; must not put `err.Error()` (fetcher internals) into a student response.
- Task 3: `errEmptyIdea` + a second trim are unreachable from current handlers.
- Task 3: `TestCreatePblProjectForSkipsNoGate` — name reads as a double negative.
- Task 4: `instructions` has no length cap.
- Task 4: PATCH `removeUserIds` can remove every recipient (no "at least one" rule).
- Task 4: rejection tests assert status only, not error codes; no test for a class teacher sent as a recipient.
- Task 4: no tests for a non-owning teacher / other-school admin on list, create, PATCH; no test for the Collect-error extract path.
- Task 4: `"targetWords":"600"` (string) fails Unmarshal → 502 instead of a lenient parse.
- Task 4: the same id in `removeUserIds` and `addUserIds` deletes and reinserts, clearing `seen_at`.
- Task 4: enrollment validation runs outside the transaction (negligible window).
- Task 4: fractional `targetWords` in (0,1) truncate to 0 → nil; `int(*v.TargetWords)` computed twice.
- Task 5: start holds one pool connection for the lock while helpers use another (fine at current pool size).
- Task 5: for-atom with a malformed atom id returns 404 (consistent with other routes).
- Task 5: 这篇文章不在阅读库里 / 这篇文章没有这一档 / 请输入以 http 或 https 开头的链接 all lack the 开始失败： prefix.
- Task 5: the concurrency test has no barrier forcing overlap (mutation check shows it catches the race in practice).
- Task 5: teachers calling student routes get 404/empty inbox rather than an explicit role rejection.
- Task 5: the lock test's `t.Fatalf` leaves the PATCH goroutine to finish after rollback (no leak).
- Task 5: the no-deadlock test replays start's lock order in SQL rather than pausing the real handler — should wrap both locks in one helper the test also calls.
- Task 6: no integration test for started-but-unfinished past-due → `overdueAssignments` 1.
- Task 6: no test for a student enrolled in two classes.
- Task 7: `beijingInputToISO` range-checks day 1–31 only (2026-02-30 passes); input with seconds is rejected.
- Task 7: s/arr/obj normalizer primitives duplicated between `api/teacher.ts` and `api/assignments.ts`.
- Task 7: `StartedAssignment` duplicates `StartAssignmentResult`; `StudentAssignmentRow` kind/status typed as string instead of the enum types.
- Task 7: `roomPathForStart`'s `projectId ?? atomId` fallback is untested.
- Task 8: `ClassPage` error display checks the pro `ApiError` class, so lite errors may render as `"ApiError: …"`; 重试 is a `<span>` (plan 1 carry-over).
- Task 8: forms lack `noValidate`, so browser tooltips pre-empt the Chinese validation messages.
- Task 8: form field order is 班级→标题→说明→截止时间→类型; spec order puts 类型 right after 班级.
- Task 8: blocked localStorage makes the form fall back to the first class (no in-memory fallback).
- Task 8: `StatusChip` lives in `AssignmentDetailPage.tsx`; detail page imports several things from `AssignmentForm.tsx` — should move to a shared file.
- Task 8: `<tr role="link">` drops row semantics; `LibraryPicker`'s `role=listbox/option` has no arrow-key support.
- Task 8: `settingsSummary` returns 项目 for projects (redundant).
- Task 8: extraction `null→""` mapping is inline in `AssignmentForm`, untested.
- Task 9: landing makes two inbox GETs (rail + strip); rail refetches on every popstate; both refetch on focus.
- Task 9: `InboxPanel`'s unread dot uses a `#fff` ring (invisible on a light surface, haloes in dark mode).
- Task 9: rail badge not refreshed after a failed start from a strip.
- Task 9: no test for `openAssignment` sequencing (markSeen rejection still starts + navigates; failure → prefixed text).
- Task 9: the Task 9 implementer report itself wrongly claimed a 404 on non-assignment room opens (see P2-9).
- Task 10: 提取失败 path not exercised in the walk (model key worked).
- Task 10: the reading in the walk was finished via its API, not by clicking 完成这篇.
- Task 10: commit `1f381f40` bundles the errorText swap with a 重试 span→button change.
- Task 10: stale `s2-*` screenshots are still referenced next to the corrected `s3-*` ones.

### Plan 3 (11)

- Task 1: no committed test for accepting a `+08:00` RFC3339 value; `t.In(Beijing)` is a no-op on the date branch.
- Task 3 fix round 1: leftover `last_activity_at` seeding left in one test.
- Task 5 review: T5 test gaps — another school's admin, a non-member userId, the 502 routing case, recording a transport-error attempt; and logging when the insert fails after compose. (Ruling 13: deferred to the final review.)
- Task 6: card evidence still said 「本周」 under a report titled for a different week — flagged as a deferred minor, later fixed in the final fix wave (Ruling 16 D).
- Final review (Ruling 16, marked "can wait"): `u.role` filter differs between the roster query and the weekly query.
- Final review (Ruling 16, "can wait"): `allNames` variable unused.
- Final review (Ruling 16, "can wait"): the JSON parser does not tolerate unescaped newlines.
- Scoped re-review of the fix wave (Ruling 18, deferred): `config.Load` reads `.env.local` from the current working directory.
- Scoped re-review (Ruling 18, deferred): `currentWeek` variable unused.
- Scoped re-review (Ruling 18, deferred): no tests for an empty class or mid-week enrollment.
- Scoped re-review (Ruling 18, deferred): a race condition gives a 500.

### Plan 4 (11)

- Task 2 review: T2 start-1 bucket test; blank-name table test; `(nn)` marker doc comment.
- Task 3 review (Ruling 12, minors 2/5/6 + remaining test gaps deferred): the enrollment read under the lock is not itself locked (tiny window); a logged validation error may quote section text; no body size cap on the report body; missing tests for a same-school admin, a PATCH racing a redraft, and generate hitting the second lock for a student who just left.
- Task 4 review: no comment/reflection test tying `InboxItemDTO` to `inboxAssignmentJSON`; no `X-Robots-Tag` assertion on the 404 path.
- Task 5: parent report tile grid gaps with odd tile counts (also listed as a product question below).
- Task 5 review: `seen` POSTed twice under React StrictMode (idempotent, harmless).
- Task 6 review: the in-memory `draftError` is lost on reload, leaving empty textareas with no hint.
- Task 6 review: preview breakpoints should be checked at viewport widths of 900–1200px (inherited from Task 5's viewport media queries).
- Task 7: at 400px, the open inbox panel covers the rail's own inbox button.

---

## 5. Open product questions for the owner

- **Assigned writing setup modal lets the student change the teacher's target.** The teacher still sees the actual target on her side, but the student-facing modal placeholder reads 「比如：这是老师布置的作业……」 and the field is editable. (Plan 2, Ruling P2-15.)
- **An assigned writing gets no title suggestions.** Because starting an assigned writing no longer posts the teacher's prompt as a first student message, the suggestion feature that used that message has nothing to work from. (Plan 2, final fix wave concern.)
- **The 老师布置 strip sits above the header on reading and writing, but below it on projects.** This follows how main's post-merge landing layout slots each room; not changed by this build. (Plan 2, Ruling P2-18 item 2.)
- **The 进行中 chip has low contrast in dark mode.** Found in the post-merge visual audit, not fixed. (Plan 2, Ruling P2-18 item 4.)
- **Edits to a published parent report go live immediately on blur, with no confirm step.** This matches the brief as written (editing stays available after publish), but was flagged for an owner decision. (Plan 4, Ruling 16 concern 4.)
- **Should a teacher be able to hide a single 金句 or keyword from the parent report?** Today every verified quote and keyword in the frozen facts appears on the public page and the poster. The teacher can only edit the section text. The final fix wave drops quotes that name a classmate, but other personal lines from her chat turns still show. The only way to keep one back is not to publish. A per-report hide control is not built. (Plan 4 final review, Important 2; Ruling 18 B.)
- **The parent report tile grid leaves gaps when the tile count is odd.** Cosmetic, not fixed. (Plan 4, Ruling 15 / Task 5 minor.)

---

## 6. Pending verification before deploy

**Live LLM runs never happened — no model key was available locally.**

- `TestLiveLiteWeekly*` (plan 3): compiles and skips (未运行). When run with a key, it should record whether real model output for the weekly student/class summaries passes `liteweekly.CheckProse` against a live prompt — specifically: quoted spans really are substrings of her words (not the teacher's assigned prompt or an assigned project's idea); item titles pass only through the separate `Titles`/《》 channel; all digits trace back to real facts (including the flagged real-model risks of Chinese numerals slipping past the digit check, and computed deltas/unit conversions failing it); the class prompt's new rules 9/10 (arabic numerals only, no arithmetic) actually reduce first-attempt failures; and the live test genuinely binds the production model config rather than a legacy alias (this was checked and confirmed correct during plan 3's final fix wave, but only by reading code, not by running it).
- `TestLiveLiteParent*` (plan 4): compiles and skips (未运行) with or without `LIVE_LLM=1`. When run with a key, it should record whether the parent report composer's prose (generate and redraft) passes the same `CheckProse`/`Titles` split, whether `stripListMarkers` correctly keeps a real model's "1 到 3 条" instruction from failing the digit check on invented list numbers, and whether the 草稿生成失败 error path (which could only be walked through a Playwright mock, not a real 502, during Task 6) behaves as designed against a real transport failure.

**The parent-report live run must specifically check the new numeral guard (P4 Ruling 20).**
- The Chinese-numeral check rejects natural wording such as 这两周, 从两个角度, 第三篇, 第二天, 三个月, 一两天 and 三天两头.
- 这两周 is the risk that matters most: it is the obvious way to describe a 14-day range. If the model rewrites it as `2 周` and 2 is not an allowed digit, both attempts fail and the teacher gets 草稿生成失败.
- Run `LIVE_LLM=1 go test ./internal/agent -run TestLiveLiteParent` at least 3 times with the production env, and judge by the worst run.
- If drafts fail on this wording, narrow the regex: exclude a numeral preceded by 这/那 or by 第.

**Before real parents see a report:**
- Time a real generate end to end. It is synchronous: facts, then up to 2 flagship attempts, within nginx's 300s limit.
- DashScope prices are empty in `models.json`, so these calls record cost 0.

**Other items the ledgers flag as pending:**
- Plan 2's known delete-race 500 (P2-6 class: concurrent starts of two library assignments at the identical article/level, same millisecond) is unfixed; a retry clears it, but it has not been re-verified live.
- Plan 3's Task 6 walk used a local stack script rather than the standard `run-stack.sh`, because the standard script needs `apps/api/.env.local` and runs Playwright itself — noted by the implementer, not re-verified against the standard script.
- Plan 4 Task 7's live journey walk (52 screenshots, light/dark × 1440/400) covered generate/edit/publish/copy-link/parent-view/revoke/unread-clear without a model key by exercising the 502 failure path; the success path with real generated prose has not been walked.
