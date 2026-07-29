# Writing-loop hardening — WA / WB / WC

> Closes the loop's weak link (the Write room) after a functional audit. Grounded in `2026-07-29-ideal-journey-gap-analysis.md` §G3 + the writing-studio PRD's own open item (Write-room thinking-card surfacing, PRD line 202) + the audit finding that the whole "check the writing" stack is built backend-side but has **zero UI consumers**.
> Three slices, each spec→build→whole-branch review→merge ("harden slice by slice").

## 0. 铁律 reconciliation (evolve the stale line)

AGENTS.md still says **"❌ 文档编辑器 / 作业撰写排版 / 提交功能 … 我们占「思考」这一层，不拥有学生的文档."** The shipped writing-studio redesign already contradicts this: the Write room *is* a plain student-writing panel (textarea + markdown preview + autosave) with .docx export, reconciled as "student writes, AI thinks alongside." This spec **evolves the line to match reality**, preserving the spirit:

> ✅ 学生在一个朴素的写作面上**自己写**正文；AI **只陪想、只查论证与结构，绝不代写正文**。项目在回顾处**完成→评估**，成品可**导出**带走——但我们**不代学生向学校提交**，也不做花哨的排版编辑器。右侧过程树仍是**只读**记录。

The three prohibitions that stay: **AI never writes the body · no fancy formatting/layout editor · we never submit to the school.** What changes: acknowledging the write panel + a finish→assess+export flow as legitimate (they already ship).

## WA · Surface "check my draft" (整稿体检) in the Write room — mostly frontend

**Finding:** `commitSnapshot` (save a version) + `orderReview` (whole-draft 整稿体检 with voices) + `orderSpotCheck` are fully implemented (Go `writing.go` + client `writing.ts`) but **no four-room component calls them.** The `orderReview` SSE stream already emits a final `review` frame carrying the persisted `ReviewItem[]` (`{criterion_code, criterion_name, band, evidence, missing, fix, points}`), so the advice can render straight from the stream — no new endpoint.

**Build:**
- `apps/web/src/api/writing.ts`: add `ReviewItem` zod schema + `runDraftReview(projectId, content, voice) → {items: ReviewItem[], wordCount, inBand}` that `commitSnapshot`s then consumes `orderReview`, collecting the `review` event's items (throws on the `error` event).
- `WritingBlock.tsx` Write tab (DraftPane): a **「让印记体检整稿」** control + a voice picker (评审团 / 质疑者 / 门外汉 / 审判者 = board/sceptic/layperson/executioner). On click → `runDraftReview` → render each `ReviewItem` as a read-only advice card: criterion · band · what it evidences · what's missing · a suggested *direction* (fix) — framed "印记体检的是你的论证与结构，不替你改字". Show the saved-version word count + in-band. Loading + error states. **Never mutates the draft.**
- Copy makes clear: this checks *thinking/structure*, produces advice to act on yourself, not edits.
- Tests (web): mock `runDraftReview` → renders items + voice switch re-runs; error state; the draft textarea is never written by the review.

**克制 / 铁律:** feedback is advice on thinking, read-only; the student revises. Metered server-side already (order_review purpose). No new spend path.

## WB · Finish & export flow

**Finding:** finish (project→async assessment) is reachable only inside the 回顾 tab's 完成回顾; there's no clear "I'm done" affordance in the writing flow and no obvious export of the deliverable from the loop's end.

**Build:**
- Surface a **完成项目** action reachable from the workspace (not only buried in Review) — e.g. in the Write room's header/goal-strip and/or a persistent nav affordance — that routes to the Review room's finish (keeps the single `finishProject` gate: reflection-done → 202 evaluating → assessment). No second finish path; one canonical finish.
- **Export the deliverable**: the Write room's draft + the proposal report + annotated bib already export (.docx/.xlsx per slice-6). Add a clear **导出成品 (.docx)** action at/near finish so "完成→带走" is one obvious motion. Reuse existing export endpoints; do not build new formatting.
- Honest copy: "完成会生成过程评估并记入成长报告；成品可导出带走。我们不替你向学校提交。"
- Tests (web): the finish affordance routes to the finish flow; export triggers the existing export.

## WC · Part-by-part writing with AI + card-hang (the PRD's open item)

**Finding:** the Write room's coach is one general rail; the PRD specced per-part thinking-card surfacing (like the Reading Room hangs a card on a sentence) but shipped a no-behavior stub. No structure binds a draft part to a focused conversation/card.

**Build (design):**
- In the Write tab, let the student **select a paragraph** (or an outline node) and **「和印记想想这一段」** → a focused coach exchange scoped to that part (the selected text is the turn's context), on the existing `scope="writing"` coach — AI checks *this part's* warrant / function / over-reach, asks one question, **never rewrites it**.
- **Hang a writing card on the part**: reuse the S4 cross-phase proposal + `StudioCardSheet` path so 印记 can summon a writing card (Toulmin / PEE / concession / steelman) onto the selected part; the completed envelope persists via the existing `/cards/persist` (S4) and lands in the process tree. Mirrors the Reading Room's hang-a-card, adapted to a draft span rather than an article sentence.
- Keep it a *thinking* aid: the part-conversation and card produce advice/structure the student applies in their own words; the draft textarea remains the student's.
- Tests: selecting a part opens the focused thread with the part as context; a summoned card mounts on the part and persists on submit; the draft is never machine-edited.

(WC is the largest; its exact selection UX + card-hang anchoring is finalized in its own slice spec at build time, reusing the Reading Room's card-hang patterns.)

## Sequence
WA → WB → WC. WA/WB are mostly UI over already-built backend (fast, high-leverage: fixes "can't check" + "no finish"); WC is the new build (fixes "can't write part-by-part"). Each merges independently after its own whole-branch review.
