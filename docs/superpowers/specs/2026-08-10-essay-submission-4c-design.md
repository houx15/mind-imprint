# Slice 4c — Essay Submission Stage (成文) + finish→review

**Date:** 2026-08-10
**Status:** design (spec → plan → implementation)
**Behavior truth:** `docs/2026-08-09-all-statuses.md` §6 (writing paper · submission complete) + §7 (review). Checked below.

## Goal

The third and last essay stage. After the statement stage (per-claim arguments +
综合/反方/结论/结构), the student assembles and finishes the whole paper:

1. **引言 (introduction)** — §143: background; the exact research question; the
   keywords/scope; why the question matters; the shape of the 论证结构.
2. **结论 (conclusion)** — §144.
3. **成文 (compose)** — §145: assemble the whole article from the statement parts +
   引言 + 结论 into one continuous draft.
4. **润色 (polish) loop** — §145: polish grammar; 印记 checks (whole-draft 批注), the
   student revises; loop until the student clicks 完成.
5. **finish → unlock review** — §151: finishing the paper advances the studio status
   to review (the terminal reflection + evaluation).

## Cross-check against all-statuses.md

| all-statuses.md | This slice |
|---|---|
| §143 "guide students to write the 引言 part → background; exact RQ; keywords/scope; importance; structure" | 引言 step = a `GuidedWritingCard` whose guidance names those five elements. |
| §144 "guide students to write the conclusion part" | 结论 step = a `GuidedWritingCard`. |
| §145 "compose the whole article and polish the grammar; then AI check - student revise → loop until finish" | 成文 step seeds the compose buffer from all parts; 润色 step = the DraftPane whole-draft review loop (整稿体检 → 批注 → revise) with a 完成 button. |
| §147 cards "PEE写作卡/论证解剖 … only needed [in claims]" | Submission steps offer NO 写作卡 (claims already covered them in 4b). |
| §151 "after final submission of paper, unlock review" | 完成 finishes the essay doc AND advances status to review. |
| §7 review "left = activity log / research framework / proposal / finished paper (four tabs); right = review form" | Confirm ReviewBlock exists with those tabs and gates on the ESSAY finish (not the legacy scalar). Fix if it gates on the legacy scalar. |

铁律 scope: composing/assembling the paper and running grammar review are deterministic
system steps on the student's own writing — **AI never writes the 正文**; the polish loop
only comments (批注) and the student revises. Finishing is the student's tap (铁律②), and
the compose step assembles what the student already wrote — it does not author prose.

## Design

### Submission step track (mirrors the statement track)

- `EssayTrack` gains submission fields: `SubmissionStarted bool`, `SubmissionStep int`,
  and reuses `StepGuides` (keyed by submission step keys, which are namespaced so they
  never collide with statement step keys).
- `DeriveSubmissionSteps()` → `[intro, conclusion, compose, polish]` (fixed; no per-claim
  fan-out — the claims are done). Step keys: `sub:intro`, `sub:conclusion`, `sub:compose`,
  `sub:polish`.
- Guide-card bodies (essay-doc generator, fast model): 引言 names the five §143 elements;
  结论 asks for the conclusion grounded in the arguments; 成文/润色 are handled by the UI
  (compose = assemble + review; polish = the review loop) so their cards are light or absent.

### Compose (成文)

- Entering the 成文 step assembles the essay BUFFER from the ordered parts the student
  wrote: 引言 + each claim snippet (in outline order) + 综合 + 反方 + 结论. Assembly reuses
  the `assembleGuidedDoc`-style join (section headers optional). Idempotent — re-entering
  re-assembles only if the buffer is still empty (never clobbers a hand-edited draft).
  The buffer is the single source (export/finish unchanged), matching the 3a/4b-1 model.
- The 成文 step surface = the plain 正文 (DraftPane) so the student sees and can hand-edit
  the whole assembled draft.

### Polish (润色) loop + finish

- The 润色 step surface = the 正文 (DraftPane) with the existing whole-draft review
  (整稿体检 → essay 批注 in the left panel), plus a **完成整篇论文** button.
- 完成整篇论文 finishes the essay doc (existing finish endpoint, doc=essay) AND advances
  the studio status to review. Reversible per 铁律② until the review is archived (existing
  reopen affordance). If 印记 has never reviewed the whole draft, a comment-first modal
  offers a review before locking (mirrors the proposal finish, §98).

### UI surface selection in the writing room

- `WritingBlock` renders, for `essayStage === "submission"`, a submission pane above the
  room (like the statement pane): the intro/conclusion `GuidedWritingCard`s for those
  steps, and for 成文/润色 it defers to the 正文 tab (auto-selects 正文) with the review
  loop + finish. When submission is active, the statement pane is not shown.

### Review gate fix

- Ensure `ReviewBlock`/回顾 unlocks on the **essay** finish (`writingFinish.essay`) and the
  status being `review`, not the legacy scalar `workspace.writingFinished`. If the current
  gate uses the scalar, switch it to the doc-aware finish + status.

## Testing

- **agent:** `DeriveSubmissionSteps` shape; essay guide bodies for `sub:intro`/`sub:conclusion`.
- **api (testcontainers):** submission endpoints (get/start/advance); 成文 assembles the
  buffer from parts idempotently; 完成 finishes essay + status→review.
- **web (vitest):** submission view renders 引言/结论 cards; 成文 shows the 正文 with the
  assembled draft; 润色 shows the review loop + 完成整篇论文 → finish+advance.
