# Slice 3b — The 批注 primitive (layered, colored, anchored teacher annotations) · Design

> **Position:** the second half (3b) of slice 3 in the [status-machine architecture spec](2026-08-09-status-machine-writing-and-cards-design.md). Behavioral source of truth = `docs/2026-08-09-all-statuses.md §4 (4.Writing proposal)`, especially: *"批注 is very important: I hope it is like a teacher, giving comments to the whole structure; to a paragraph's 论证; to a particular sentence or phrase. use green/blue/red texts and underlines."* This slice builds that primitive and finishes the §4 proposal page (the reference tabs, select-to-send, the finish-proposal comment-first modal) — the parts slice 3a explicitly deferred here.

## One-sentence goal

Give 印记 a teacher's red/blue/green pen: a reasoning-model reviewer that reads the student's proposal and returns **layered** (whole-structure / paragraph / sentence), **colored** (green = solid, blue = suggestion, red = problem), **anchored** annotations, rendered as underlined colored spans on the draft and as a clickable AI批注 reference group — never rewriting the student's text (铁律①).

## 铁律 (carried, hard constraints)

1. **AI never writes body text.** 批注 diagnoses — it comments on structure / a paragraph's argument / a sentence; it never edits or rewrites the student's prose. The note is a direction, not a replacement sentence.
2. **No manipulation.** A 批注 is advice; the student may ignore any of it and still finish (the finish-proposal review suggestion is a suggestion, skippable).
3. **One thing at a time** applies to the *coach chat*, not to a review — a whole-draft review legitimately returns several 批注 at once (a teacher marks the whole page). Chat guidance stays one question at a time.
4. **Process is data.** Which 批注 the student acts on vs ignores is left in the record (the 批注 rows persist; acting on them is the student's edits).
5. **Never downgrade evaluation.** 批注 is teacher judgment → runs on the **flagship reasoning** model (`EvalResolver`), never the fast coach.

---

## Pillar 1 · The 批注 data model

A 批注 is one teacher comment. Three orthogonal facets (all from §4):

```
level:  "structure" | "paragraph" | "sentence"   // what it comments on
nature: "solid" | "suggest" | "problem"          // green / blue / red
anchor: { blockId, start, end } | null           // rune range into the draft; null for structure-level (whole-doc)
note:   string                                    // the teacher's comment (a direction, never a rewrite)
quote:  string                                    // the exact text the anchor covers (authoritative if offsets miss)
```

### Contracts (`packages/contracts`)

- New `packages/contracts/src/proposalAnnotation.ts`:
  ```ts
  export const AnnotationLevel = z.enum(["structure", "paragraph", "sentence"]);
  export const AnnotationNature = z.enum(["solid", "suggest", "problem"]); // green / blue / red
  export const DraftAnnotation = z.object({
    id: z.string(),
    level: AnnotationLevel,
    nature: AnnotationNature,
    blockId: z.string(),          // "" for structure-level
    start: z.number().int().nonnegative(),
    end: z.number().int().nonnegative(),
    quote: z.string(),
    note: z.string(),
  });
  ```
- Extend the generic annotate primitive with ONE optional nature color (backward-compatible): `packages/contracts/src/interactionPrimitive.ts` `AnnotateSpan` gains `nature: AnnotationNature.optional()`. When absent, `Annotate.tsx` keeps its current author-based coloring (reading is untouched); when present, the nature color (green/blue/red) wins. This is how the existing `Annotate` renderer draws the colored underlined spans without a parallel renderer.

### Persistence (no migration)

Reuse the `intervention` table's additive `anchor` jsonb (it already carries `voice`, `station`, `fingerprint` with no schema change — `0016_refactor2_foundations.sql:82-95`). 批注 rows are `type = "proposal_annotation"` with:
- `anchor` jsonb = `{"kind":"draft_snapshot","id":<sid>,"docKind":"proposal","level":…,"nature":…,"blockId":…,"start":…,"end":…,"quote":…}`
- `body` = the note text; `level` column = the nature; `criterion` column = the level (reusing the existing columns; the jsonb carries the structured truth).

A new sqlc query `ListProposalAnnotations(project_id)` filters `type='proposal_annotation'`. New annotations for a fresh review of the same doc **replace** the prior set for that doc (delete-then-insert in one tx, keyed on `project_id + docKind`), so the AI批注 tab always reflects the latest review — a re-review doesn't pile stale 批注.

---

## Pillar 2 · Producing 批注 (the reasoning reviewer)

### The reviewer (`apps/api/internal/agent`)

New `agent.ReviewDraftAnnotations(ctx, prov, resolved, in DraftAnnotationInput) ([]DraftAnnotationOut, gateway.ChatUsage, error)`, mirroring `ReviewFramework`/`ReviewProposalPart` (Collect + 2-attempt retry + defensive JSON parse + enforcement on free-text). Input: `{title, blocks:[{id,text}], focusBlockId?}` (focusBlockId set for a per-step "我写好了" review scoped to one part; empty for a whole-draft "AI check"). Output per item: `{level, nature, quote, note, blockId?}` — the model returns the **quote**, not offsets; the server computes offsets.

System prompt: a teacher marking a proposal. Return JSON `{annotations:[{level, nature, quote, note, blockId}]}`. Rules: comment on the whole structure, on a paragraph's 论证, and on specific sentences/phrases; `nature` = solid/suggest/problem; `note` is a direction, **never a rewrite** (铁律①); quote must be copied verbatim from the student's text so it can be located; ≤ ~12 annotations; only-JSON. Runs on `EvalResolver` (flagship). Playful loading line client-side ("印记正在逐句批注…").

### Offset computation (server)

For each returned annotation with a non-empty quote, the handler locates it in the draft using the existing `computeOffsets` (`apps/api/internal/agent/anchors.go:127`, rune indices, punctuation-tolerant). It picks the block:
- `structure` → no anchor (`blockId=""`, `start=end=0`); rendered as a top-level comment.
- `paragraph` / `sentence` → search the paragraph blocks (from `paragraphSpanIndex`, `writing.go:159`) for the quote; the matched block's id + the rune range within it. A quote that can't be located degrades to `start=end=0` with the quote kept (rendered as an unanchored note under its block, or top-level) — never dropped, never crashes (mirrors the reading anchor miss policy).

### Endpoints (`apps/api/internal/api`)

- Upgrade `POST /projects/{id}/proposal-track/review {stepKey}` (the "我写好了" seam from 3a): instead of returning a `ReviewVerdict` for a chat bubble, it now (a) commits the current proposal buffer as a draft snapshot (reuse the snapshot path), (b) runs `ReviewDraftAnnotations` scoped to the current step's block(s), (c) computes offsets + persists the 批注 rows (replacing prior for this doc's scope), (d) returns the `DraftAnnotation[]`. The frontend renders them (no longer a chat bubble).
- New `POST /projects/{id}/proposal-annotations/review` — the whole-draft "AI check": same pipeline, no `stepKey` (reviews all blocks).
- New `GET /projects/{id}/proposal-annotations` → `{annotations: DraftAnnotation[]}` for the doc (read path, no spend).

Metered `purpose="proposal_annotation"`. Best-effort: nil resolver / model error / all-unparseable → empty list, never breaks the surface.

---

## Pillar 3 · Rendering 批注

### On the draft — a 批注 view mode

`ProsePane` gains a third view alongside 编辑 / 预览: **批注**. A textarea can't show inline colored spans, so the 批注 view is a read-only render of the draft via the existing `Annotate` primitive:
- blocks = the draft's paragraphs (same paragraph split the server anchored against).
- spans = the doc's `DraftAnnotation[]` mapped to `AnnotateSpan{ id, block_ref, range:{start,end}, note, nature, author:"ai" }`.
- `Annotate` draws each as an underlined `<mark>` colored by `nature` (green/blue/red) — the teacher's pen. Structure-level 批注 render above the text as a list (no span).
- Clicking a span selects it (`activeSpanId`) and shows its note; clicking a 批注 in the AI批注 reference group (below) activates the same span here.

When a review returns 批注, the pane auto-switches to the 批注 view once (so the student sees the marks), then is free to switch back to 编辑.

### In the reference panel — the AI批注 group

`ReferencePanel`'s existing flat `AnnotationGroup` (`ReferencePanel.tsx:218`) is upgraded to the layered colored 批注: each row shows a nature dot (green/blue/red) + level label (结构/段落/句子) + the quoted span (for paragraph/sentence) + the note; clicking a row activates its span in the ProsePane 批注 view. It reads the new `GET /proposal-annotations` (the flat `getAnnotations` review-item path stays for the essay's 整稿体检; the proposal uses the new 批注 path). `curate_reference kind:"annotation"` continues to work (the coach can still pull a 批注 to the top).

§4's tab set (提案要点 / 阅读笔记 / **AI批注**) is honored by the panel's conditional groups (it's group-based, not literally tabbed — the AI批注 group shows once 批注 exist; 检索文献 stays out, per the F decision).

---

## Pillar 4 · Select-to-send + finish-proposal comment-first modal

### Select-to-send on the proposal surface

Mirror the essay `DraftPane`'s `selPop` "问印记" chip (`WritingBlock.tsx:1102`, `:1525`) onto `ProsePane`: on mouse-up over a selection in the 编辑 textarea, float a "问印记" chip; clicking pins the selected text into the coach composer (the shared `focusPart` mechanism). No rune conversion needed (it's sent as text to the coach, not anchored).

### Finish-proposal comment-first modal (§4)

Intercept the 完成提案 flow (`WritingBlock.tsx:238` / `doFinishWriting:179`):
- If 印记 has **never** produced 批注 for this proposal → the confirm modal first offers **"先让印记看一遍"** (runs the whole-draft 批注 review) or **"跳过，直接完成"**.
- If 批注 exist but the latest whole-draft check still flags a `problem`-nature 批注 → the modal still gently suggests addressing it (non-blocking — the student can proceed; 铁律②).
- On skip / proceed → the existing congrats + **导出 docx** + 继续下一步 flow (unchanged).

---

## Model routing

| Role | Model | What |
|---|---|---|
| 批注 reviewer (per-part "我写好了", whole-draft "AI check", finish check) | **flagship reasoning** (`EvalResolver`) | reads the draft, returns layered colored anchored 批注 |
| coach (select-to-send guidance, "我依然有问题") | fast (`FastChatResolver`) | unchanged from 3a |

Note: the essay's existing whole-draft `orderReview` runs on the chaperone tier (`writing.go:337`) — a pre-existing inconsistency left as-is this slice (the proposal 批注 is the new, flagship path). Reconciling the essay review tier is out of scope here.

---

## Mapping to existing code (reuse vs new)

**Reuse:**
- `Annotate` primitive + `segment.ts`/`selection.ts`/`sentences.ts` (`apps/web/src/primitives/annotate/`) — the colored-span renderer + rune-safe slicing + selection→offset. Extended by ONE optional `nature` color.
- `computeOffsets` + `paragraphSpanIndex` (`apps/api/internal/agent/anchors.go`, `writing.go`) — server-side quote→rune-range location.
- The `intervention.anchor` jsonb seam + `InsertIntervention` — additive, no migration.
- The snapshot commit path (`commitSnapshot`) — 批注 reviews a committed snapshot.
- `ReferencePanel` `AnnotationGroup` + `curate_reference` annotation flow — upgraded, not replaced.
- `ProsePane` (3a) — gains the 批注 view mode + select-to-send.
- The 3a `reviewProposalPart` endpoint + `ProposalGuide` "我写好了" — upgraded to the 批注 pipeline.

**New:**
- `agent.ReviewDraftAnnotations` + `DraftAnnotationInput/Out` (flagship reviewer).
- Contracts `proposalAnnotation.ts` (`DraftAnnotation`/`AnnotationLevel`/`AnnotationNature`); `AnnotateSpan.nature`.
- sqlc `ListProposalAnnotations` + a delete-then-insert persist for a doc's 批注 set.
- Endpoints: upgraded `proposal-track/review`; new `proposal-annotations` (GET + review POST).
- Web: `proposalAnnotations.ts` client; `ProsePane` 批注 view + select-to-send; `AnnotationGroup` upgrade; finish-proposal comment-first modal.

---

## Doc consistency check (against `all-statuses.md §4`)

| §4 point | This slice | Verdict |
|---|---|---|
| 批注 like a teacher: whole structure / paragraph's 论证 / a sentence or phrase | Pillar 1 `level` enum + Pillar 2 reviewer prompt | ✅ |
| green/blue/red texts and underlines | Pillar 1 `nature` + Pillar 3 `Annotate` colored underlined `<mark>` | ✅ (nature→color: solid/suggest/problem = green/blue/red) |
| Left reference: 提案要点 / 阅读笔记 / **AI批注** (检索文献 stays out) | Pillar 3 AnnotationGroup upgrade; 检索 excluded (F decision) | ✅ |
| select text → "send to AI" | Pillar 4 select-to-send on ProsePane | ✅ |
| "AI check" button | Pillar 2 whole-draft `proposal-annotations/review` | ✅ |
| finish-proposal: if AI never commented → modal suggests comment; can skip; if commented but evident flaw → still suggest | Pillar 4 finish comment-first modal | ✅ |
| skip / proceed → congrats + export docx + continue | Pillar 4 (existing flow, unchanged) | ✅ |
| "我写好了" triggers AI comment/check | Pillar 2 upgrades the 3a interim to 批注 | ✅ (replaces the 3a chat-bubble interim) |

No §4 proposal-page behavior remains unbuilt after 3b.

## Out of scope / follow-ups

- The essay (写正文) 批注 — slice 4 reuses this primitive for per-claim writing; not built here.
- Reconciling the essay's existing `orderReview` chaperone tier — left as-is.
- Student-authored 批注 on their own draft (only 印记's 批注 here).
- Slice 5: reading-page needs-resources box + search-guidance box + broader reference-panel tabbing.
- No fancy layout editor, no submitting for the student, no gamification (铁律 carried).
