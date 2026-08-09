# Slice 3b — The 批注 primitive (layered, colored, view-only teacher annotations) · Design

> **Position:** the second half (3b) of slice 3 in the [status-machine architecture spec](2026-08-09-status-machine-writing-and-cards-design.md). Behavioral source of truth = `docs/2026-08-09-all-statuses.md §4 (4.Writing proposal)`: *"批注 is very important: I hope it is like a teacher, giving comments to the whole structure; to a paragraph's 论证; to a particular sentence or phrase. use green/blue/red texts and underlines."* This slice builds that primitive and finishes the §4 proposal page (the reference tabs, select-to-send, the finish-proposal comment-first modal) — the parts slice 3a explicitly deferred here.

## One-sentence goal

Give 印记 a teacher's pen: a reasoning-model reviewer that reads the student's proposal and returns **layered** (paper / paragraph / sentence), **colored** annotations, rendered **view-only in the left reference panel** — never overlaid on, and never touching, the student's editable writing (铁律①).

## The 批注 shape (from the user's refinement)

批注 is deliberately **sparse** — a teacher marks only what's worth marking, not every line. Three levels, each with its own rendering rule:

| level | when | color | rendering |
|---|---|---|---|
| **paper** (whole article) | always (a short overall read) | **green** = what's done well; **blue/red** = what needs enhancing | a "总体" summary block at the top of the AI批注 group |
| **paragraph** | only where there's a suggestion — NOT every paragraph | **blue** (suggest) / **red** (problem) | a comment card with detailed cases; a paragraph locator (e.g. 第 2 段); **no underline** |
| **sentence** | only for a specific sentence/phrase worth flagging | **blue** (suggest) / **red** (problem) | the quoted sentence shown **with a blue/red underline** + the comment |

- **Only sentence-level underlines** (blue/red). Paragraph-level comments and the paper-level summary are not underlines — underlining green "solid" sentences would underline the whole article, which is noise.
- Green appears **only at the paper level** ("what you did well"), as text, never as a per-sentence underline.

## 铁律 (carried, hard constraints)

1. **AI never writes body text, and never touches the writing surface.** 批注 is **view-only, in the left panel**. It comments; it does not edit, rewrite, or overlay the student's editable draft. A note is a direction, never a replacement sentence.
2. **No manipulation.** 批注 is advice; the student may ignore any of it and still finish (the finish-proposal review suggestion is skippable).
3. **One thing at a time** is a *coach-chat* rule; a review legitimately returns several 批注 at once (a teacher marks the page). Chat guidance stays one question at a time.
4. **Process is data.** The 批注 rows persist; whether the student acts on them shows in their subsequent edits.
5. **Never downgrade evaluation.** 批注 is teacher judgment → **flagship reasoning** (`EvalResolver`), never the fast coach.

---

## Pillar 1 · The 批注 data model

```
level:  "paper" | "paragraph" | "sentence"
nature: "good" | "suggest" | "problem"     // green / blue / red
quote:  string                             // the sentence text (sentence level); "" for paper; optional lead for paragraph
locator: string                            // a human paragraph reference (e.g. "第2段" or the paragraph's opening words); "" for paper
note:   string                             // the teacher's comment (a direction, never a rewrite)
```

No rune offsets, no character-range anchoring — because 批注 is view-only in the left panel (it shows the quote, it does not highlight positions inside the center draft). This is a deliberate simplification of the earlier draft of this spec.

### Contracts (`packages/contracts`)

New `packages/contracts/src/proposalAnnotation.ts`:
```ts
export const AnnotationLevel = z.enum(["paper", "paragraph", "sentence"]);
export const AnnotationNature = z.enum(["good", "suggest", "problem"]); // green / blue / red
export const DraftAnnotation = z.object({
  id: z.string(),
  level: AnnotationLevel,
  nature: AnnotationNature,
  quote: z.string(),    // sentence text; "" for paper
  locator: z.string(),  // paragraph reference; "" for paper
  note: z.string(),
});
```
No change to the shared `Annotate` primitive (it stays reading-only; 批注 does not reuse it, since 批注 is not an in-place text overlay).

### Persistence (no migration)

Reuse the `intervention` table's additive `anchor` jsonb (`0016_refactor2_foundations.sql:82-95` — already carries `voice`/`station` with no schema change). 批注 rows are `type = "proposal_annotation"`:
- `anchor` jsonb = `{"docKind":"proposal","level":…,"nature":…,"quote":…,"locator":…}`
- `body` = the note; `level` column = the nature; `criterion` column = the level.

New sqlc `ListProposalAnnotations(project_id)` filters `type='proposal_annotation'`. A fresh whole-draft review **replaces** the doc's prior set (delete-then-insert in one tx, keyed on `project_id + docKind`) so the AI批注 panel always reflects the latest read and doesn't accrete stale marks.

---

## Pillar 2 · Producing 批注 (the reasoning reviewer)

### The reviewer (`apps/api/internal/agent`)

New `agent.ReviewDraftAnnotations(ctx, prov, resolved, in DraftAnnotationInput) ([]DraftAnnotationOut, gateway.ChatUsage, error)`, mirroring `ReviewFramework`/`ReviewProposalPart` (Collect + 2-attempt retry + defensive JSON parse + enforcement on free-text). Input: `{title, draft string, focus string}` (`focus` = the current guided step's title/prompt for a per-step "我写好了"; empty for a whole-draft "AI check"). Output per item: `{level, nature, quote, locator, note}`.

System prompt — a teacher marking a proposal, returning JSON `{annotations:[…]}`. Rules baked in:
- Give a short **paper-level** read: at least one `good` (green) point and the main thing to enhance.
- Add **paragraph-level** comments **only where there is a real suggestion** (blue) or problem (red) — do NOT comment every paragraph; each carries a `locator` and detailed reasoning/cases.
- Add **sentence-level** marks **only** for specific sentences/phrases worth flagging (blue/red); copy the sentence verbatim into `quote`.
- `note` is a direction, **never a rewrite** (铁律①). ≤ ~10 annotations total. Chinese. JSON only.

Runs on `EvalResolver` (flagship). Client shows a playful loading line ("印记正在逐句批注…").

### Endpoints (`apps/api/internal/api`)

- **Upgrade** `POST /projects/{id}/proposal-track/review {stepKey}` (the 3a "我写好了" seam): read the proposal buffer, run `ReviewDraftAnnotations` with `focus` = the step, persist the resulting 批注 (replace this doc's set), return `DraftAnnotation[]`. It no longer returns a chat-bubble `ReviewVerdict`.
- **New** `POST /projects/{id}/proposal-annotations/review` — the whole-draft "AI check": same pipeline, `focus` empty (reads the whole draft).
- **New** `GET /projects/{id}/proposal-annotations` → `{annotations: DraftAnnotation[]}` (read path, no spend).

Metered `purpose="proposal_annotation"`. Best-effort: nil resolver / model error / unparseable → empty list, never breaks the surface. Reads the `edit_buffer` directly (like 3a's `reviewProposalPart`) — no snapshot commit needed.

---

## Pillar 3 · Rendering 批注 — view-only, left panel

批注 lives **entirely in the left reference panel**; the center `ProsePane` stays the plain editable textarea, untouched (铁律①). `ReferencePanel`'s existing flat `AnnotationGroup` (`ReferencePanel.tsx:218`) is rebuilt for the proposal 批注:

- **总体 (paper)** — a summary block at the top: green lines for `good`, blue/red lines for what to enhance.
- **段落 (paragraph)** — comment cards, each a blue/red dot + the `locator` (第 N 段) + the `note` (with its detailed reasoning). No underline.
- **句子 (sentence)** — cards showing the `quote` rendered **with a blue/red underline** (a simple styled span — `underline decoration-{color}`) + the `note`.

It reads the new `GET /proposal-annotations` for the proposal doc. (The essay's flat `getAnnotations` review-item path is unchanged — the proposal uses this new 批注 path.) `curate_reference kind:"annotation"` still works for the coach to pull one to the top.

§4's tab set (提案要点 / 阅读笔记 / **AI批注**) is honored by the panel's conditional groups (group-based, not literally tabbed); 检索文献 stays out (F decision). No ProsePane view-mode change, no `Annotate` extension — this slice adds nothing to the editable writing area.

---

## Pillar 4 · Select-to-send + finish-proposal comment-first modal

### Select-to-send on the proposal surface

Mirror the essay `DraftPane`'s `selPop` "问印记" chip (`WritingBlock.tsx:1102`, `:1525`) onto `ProsePane`: on mouse-up over a selection, float a "问印记" chip; clicking pins the selected text into the coach composer (the shared `focusPart` mechanism). Text only — no anchoring.

### Finish-proposal comment-first modal (§4)

Intercept the 完成提案 flow (`WritingBlock.tsx:238` / `doFinishWriting:179`):
- If 印记 has **never** produced 批注 for this proposal → the modal first offers **"先让印记看一遍"** (runs the whole-draft 批注 review) or **"跳过，直接完成"**.
- If 批注 exist and the latest set still contains a `problem`-nature 批注 → the modal gently notes it, still skippable (铁律②).
- On skip / proceed → the existing congrats + **导出 docx** + 继续下一步 flow (unchanged).

---

## Model routing

| Role | Model | What |
|---|---|---|
| 批注 reviewer ("我写好了", whole-draft "AI check", finish check) | **flagship reasoning** (`EvalResolver`) | reads the draft, returns layered colored 批注 |
| coach (select-to-send guidance, "我依然有问题") | fast (`FastChatResolver`) | unchanged from 3a |

The essay's existing `orderReview` runs on the chaperone tier (`writing.go:337`, a pre-existing inconsistency) — left as-is; the proposal 批注 is the new flagship path.

---

## Mapping to existing code (reuse vs new)

**Reuse:**
- The `intervention.anchor` jsonb seam + `InsertIntervention` — additive, no migration.
- `ReviewFramework`/`ReviewProposalPart` pattern (Collect + retry + parse + enforcement) — the new reviewer mirrors it.
- `GetEditBuffer` — the reviewer reads the proposal buffer directly.
- `ReferencePanel` `AnnotationGroup` + `curate_reference` annotation flow — rebuilt for layered 批注.
- `ProsePane` (3a) — gains select-to-send only (no view mode).
- The 3a `reviewProposalPart` endpoint + `ProposalGuide` "我写好了" — upgraded to the 批注 pipeline (renders in the left panel instead of a chat bubble).

**New:**
- `agent.ReviewDraftAnnotations` + `DraftAnnotationInput/Out` (flagship reviewer).
- Contracts `proposalAnnotation.ts` (`DraftAnnotation`/`AnnotationLevel`/`AnnotationNature`).
- sqlc `ListProposalAnnotations` + a delete-then-insert persist for a doc's 批注 set.
- Endpoints: upgraded `proposal-track/review`; new `proposal-annotations` (GET + review POST).
- Web: `proposalAnnotations.ts` client; the `AnnotationGroup` rebuild (总体/段落/句子, colored, sentence-underline); `ProsePane` select-to-send; finish-proposal comment-first modal.

**Explicitly NOT touched:** the shared `Annotate` primitive, `computeOffsets`, `paragraphSpanIndex`, and the editable writing area — 批注 is view-only on the left.

---

## Doc consistency check (against `all-statuses.md §4`)

| §4 point | This slice | Verdict |
|---|---|---|
| 批注 like a teacher: whole structure / paragraph's 论证 / a sentence or phrase | Pillar 1 `level` (paper/paragraph/sentence) + reviewer | ✅ |
| green/blue/red + underlines (but not the whole article) | Sentence underline blue/red; paragraph blue/red comment (no underline); paper green/blue/red summary | ✅ (per the user's refinement) |
| view-only, don't manipulate the student's writing | Pillar 3 — left panel only, editable draft untouched | ✅ |
| Left reference: 提案要点 / 阅读笔记 / **AI批注** (检索 stays out) | Pillar 3 AnnotationGroup rebuild; 检索 excluded (F) | ✅ |
| select text → "send to AI" | Pillar 4 select-to-send | ✅ |
| "AI check" button | Pillar 2 whole-draft `proposal-annotations/review` | ✅ |
| finish-proposal: AI-never-commented → suggest; can skip; flaw → still suggest | Pillar 4 finish comment-first modal | ✅ |
| skip / proceed → congrats + export docx + continue | Pillar 4 (existing flow) | ✅ |
| "我写好了" triggers AI comment/check | Pillar 2 upgrades the 3a interim to 批注 | ✅ |

No §4 proposal-page behavior remains unbuilt after 3b.

## Out of scope / follow-ups

- The essay (写正文) 批注 — slice 4 reuses this reviewer/renderer for per-claim writing; not built here.
- Reconciling the essay's `orderReview` chaperone tier — left as-is.
- Student-authored 批注; in-place center highlighting — deliberately not done (批注 is view-only, left).
- Slice 5: reading-page needs-resources + search-guidance boxes + broader reference tabbing.
- No fancy layout editor, no submitting for the student, no gamification (铁律 carried).
