# Slice 4b — Essay statement stage: outline → per-claim arguments → synthesis · Design

> **Position:** the second sub-slice of slice 4 (写正文 sub-machine). Behavioral source of truth = `docs/2026-08-09-all-statuses.md §6 stage-2 (statement-complete)`. This is the essay-writing analog of slices 3a (guided scaffold) + 3b (批注), applied to the essay: an outline seeded from the research question + its sub-questions, then a guided walk writing the argument paragraph for **each claim (sub-question) one at a time** with PEE/toulmin/argument-map 写作卡 and 批注, then comparison/synthesis → conclusion → the 论证结构 paragraph. The per-claim "去写这条论点" entry from slice 4a lands here.

## One-sentence goal

After the research stage, the student writes the essay's body as a guided walk: modify the outline (seeded from their RQ + sub-questions), then — one claim at a time — write each sub-question's argument paragraph (evidence / analysis / limitation / how it links to the others) with 写作卡 offered when they're stuck and 批注 when they say 我写好了, then write the comparison/synthesis, the conclusion, and a paragraph describing the whole 论证结构.

## Design principle (§6 stage-2)

A body of an essay is not a pile of paragraphs — it is a **推理链 (chain of reasoning)**: each claim earns its place with evidence + analysis + a stated limitation, and the claims connect. So the statement stage walks the claims in order, and the guidance for each is about building an argument (not prose polish): what's your evidence, what does it show, where does it not hold, how does it advance the next claim. The 写作卡 (PEE / 论证解剖) exist ONLY here — to help build arguments — and are offered when needed, not forced (铁律②).

## 铁律 (carried)

1. **AI never writes the body.** Guide cards ask; 写作卡 scaffold the student's own writing; 批注 diagnoses. 印记 never writes a claim's paragraph.
2. **No manipulation.** 我写好了 / advancing / finishing are never blocked; the walk is a suggestion — the student can free-write in the tabs and skip the scaffold.
3. **One thing at a time** for coach guidance; a review returns 批注 for the part.
4. **Process is data.** Which claims got 写作卡, which 批注 were acted on — recorded.
5. **Never downgrade evaluation** — 批注 on flagship (`EvalResolver`); guide-card generation + 我依然有问题 on fast.

---

## Pillar 0 · Cross-cutting prerequisite — doc-key the 批注 (from 3b)

Slice 3b's 批注 storage is proposal-only: `intervention type='proposal_annotation'`, and `runDraftAnnotationReview` hardcodes `DocKind=DocProposal` (`proposal_annotations.go:46`). Before the essay can have 批注, generalize:
- The reviewer `ReviewDraftAnnotations` gains a doc-agnostic prompt variant (or a `docKind`/`what` field so the system prompt says "研究提案" vs "论文正文").
- Storage keyed by doc: either a `docKind` in the `anchor` jsonb + a filter, or a distinct `type='essay_annotation'`. **Proposal:** keep `type='proposal_annotation'` for the proposal and add `type='essay_annotation'` for the essay (cleanest, no migration — the intervention type column already exists); `ReplaceProposalAnnotations` becomes `ReplaceAnnotations(projectID, docKind, rows)` writing the right type; the GET/review endpoints take a `?doc=` (like the writing buffer).
- The left-panel `ProposalAnnotationGroup` render is reused for the essay (it's doc-agnostic UI); `ReferencePanel` reads the essay's 批注 when the writing stage is the essay.

*(Small, mechanical; the first task group of 4b.)*

---

## Pillar 1 · The essay statement track (derived steps)

`EssayTrack` (4a: `{Stage}`) gains the statement walk:

```go
type EssayTrack struct {
    Stage        EssayStage        `json:"stage"`
    StatementStep int              `json:"statementStep,omitempty"` // index into the derived statement steps
    StepGuides   map[string]string `json:"essayStepGuides,omitempty"` // guide-card cache by step key (like proposal)
}
```

`DeriveStatementSteps(subQuestions []SubQuestion) []Step` (mirrors `DeriveProposalSteps`): 

| # | key | Kind | Title | surface | guidance |
|---|---|---|---|---|---|
| 0 | `outline` | fixed | 搭大纲 | 大纲 | the seeded outline (RQ + sub-questions); 印记 intros the whole plan + a 准备好了吗？ ready gate (Pillar 2) |
| 1..N | `claim:<sqId>` | subq | 论点 N | 片段 | write this claim's argument in a `GuidedWritingCard`: evidence / analysis / limitation / how it links to the next; 写作卡 offered |
| N+1 | `synthesis` | fixed | 比较 / 综合 | 片段 | compare/synthesize the claims |
| N+2 | `challenges` | fixed | 面对反方观点 | 片段 | a DEDICATED step (user): address the strongest counterarguments/rebuttals — not skippable-by-default |
| N+3 | `conclusion` | fixed | 结论 | 片段 | the conclusion the claims support |
| N+4 | `structure` | fixed | 论证结构 | 片段 | a paragraph describing the whole argument's structure |

- The **outline is seeded** from the main RQ + sub-questions on entering statement (if the outline is empty): `SeedEssayOutlineFromProposal` writes outline rows (RQ as a top node, each sub-question as a child) via the existing outline store. §6: "outline initialized by the main research question and 2-4 subquestions."

### Editing the claims (sub-questions) in the statement stage (§6, user-refined)

The claims **are editable here** — that's necessary. But 印记 helps the student tell two very different edits apart:
- **Rephrasing** (the usual case in the writing stage): the claim's meaning is intact, just worded better → the gathered materials + any writing for it **still apply**. Fine, no warning.
- **A total change** (a different question): the claim is no longer the same → the previously gathered materials + the writing done for it **become inapplicable**. 印记 must **warn the student** ("换成这个问题的话，之前为它找的材料和写的内容就用不上了") so it's a conscious choice, not a silent loss.
- **A question that can't become a claim** (no materials could be gathered, or it doesn't stand up): 印记 helps the student notice this and reconsider — it's a signal to drop/reshape the claim, not to keep forcing it.
- A **total change of the research question** belongs back in **reading/exploration**, not the writing stage — 印记 points there rather than doing a full re-decomposition mid-writing.

Implementation: a light "edit claim" affordance on the outline/claim; when an edit looks like a total change (not a near-rephrase), surface 印记's warning about the now-inapplicable materials/writing before committing. (A reasoning check for "rephrase vs total change" is a small reviewer call; the heuristic + a coach nudge is the 4b MVP.)

---

## Pillar 2 · Chat-guided flow + per-claim guided writing cards

The walk is **guided in the chat**, not by a heavy overlay. The concrete flow (user):

1. Entering the statement stage, the student is on the **大纲 (outline) page**. 印记 introduces the whole plan in the chat — e.g. *"接下来我们要开始进行正文的写作了。我们可以考虑首先将核心论证的部分完成。右边的大纲是根据前面我们讨论的你的核心问题以及具体子问题形成的初步大纲，接下来我们尝试先依次写各个子问题的论述段落，然后进行对比和讨论，最后构建整体问题与部分之间的关系。准备好了吗？"* — with a **准备好了吗？/开始写作** ready gate (mirrors the proposal's outline-intro gate).
2. After the student taps ready → the surface switches to the **片段 (snippet) page** for focused per-claim writing.

印记 keeps narrating the walk from there (finish the claims in order → 比较/讨论 → 面对反方观点 → 结论 → 论证结构). The heavy lifting per claim is a **`GuidedWritingCard` in 片段**.

### The `GuidedWritingCard` (shared primitive — also retrofits 3a)

Per `all-statuses.md §4/§6`, each part/claim is written in a **colorful card that carries its guidance AND its own writing surface** — not a guidance card floating above a shared draft. So 4b builds a shared `GuidedWritingCard`:
- **Clear guidance** at top: the AI-generated guiding question for this claim (evidence / analysis / limitation / how it links to the next), applied to THIS claim, + an English example.
- **A multi-line textarea that auto-grows** as the student writes (height increases with content; no fixed 6-row cage) — the student writes this claim's argument paragraph right here.
- **Two buttons (§6):** **我依然有问题** → 印记 guides in the chat (fast); **我写好了** → 批注 on this claim's text (doc-keyed essay 批注, Pillar 0), surfaced in the left panel.
- **写作卡 offer (§6):** a "需要帮你把论证搭起来吗？" affordance offering **PEE / toulmin / argument-map** (already summonable in the essay deck) — only on claim cards (§6: 写作卡 only needed here).

Each claim card = one snippet (the essay 片段 model): the card's text saves as that claim's snippet (`section` = the claim). This fits the existing `SnippetsPane`/`useSnippets` — a claim card is a snippet with guidance + an auto-growing textarea.

> **3a reconciliation (flagged):** my 3a proposal guide put the guidance card *above the shared ProsePane* — the student writes the whole proposal in one textarea, diverging from §4's per-part "snippet writing frame". The `GuidedWritingCard` here is the correct model; **I'll refactor 3a's proposal parts to use it too** (each proposal part → a guided card with its own auto-growing textarea) so both match §4 — unless you'd rather leave 3a and only apply this to 4b.

### Track + endpoints

- The essay statement track (`EssayTrack.StatementStep` + `DeriveStatementSteps`) drives which claim/step is active; the flexible entry from 4a ("去写这条论点") deep-links to that claim.
- Endpoints reuse the proposal-track **shape** generalized by `?doc=` (guide-card gen, per-part 批注 review). But the **step derivation stays doc-specific** (user: the proposal's 9 parts ≠ the paper's outline→claims→synthesis→challenges→conclusion→structure) — `DeriveStatementSteps` is its own function, not a param on `DeriveProposalSteps`.

---

## Pillar 3 · Advancing to submission

When the statement steps are done, a "正文写完了，去收尾" tap advances `EssayTrack.Stage = submission` (reuse `advance-stage` from 4a) — the submission stage (引言/conclusion/compose/polish) is slice 4c. 4b just performs the flip; 4c builds submission.

---

## Model routing

| Role | Model | What |
|---|---|---|
| 批注 (我写好了) | flagship (`EvalResolver`) | per-claim / whole-draft 批注 |
| guide-card gen + 我依然有问题 | fast (`FastChatResolver`) | claim guidance |

---

## Mapping to existing code (reuse vs new)

**Reuse (pattern-heavy):**
- `proposal_track.go` / `proposal_guide.go` / `useProposalTrack` / `ProposalGuide` — the essay track/guide mirror these.
- `DraftAnnotations` reviewer + `ProposalAnnotationGroup` + the 批注 endpoints — generalized doc-keyed (Pillar 0).
- The essay 大纲/片段/正文 surface (`OutlinePane`/`SnippetsPane`/`DraftPane`) — unchanged; the scaffold overlays.
- The essay deck cards (pee/toulmin/argument-map) + `summon_card` / `openCard` — the 写作卡 offer.
- `advance-stage` (4a) — the statement→submission flip.
- `getOutline`/`putOutline` — the outline seed.

**New:**
- `EssayTrack.StatementStep` + `DeriveStatementSteps` + `SeedEssayOutlineFromProposal`.
- Essay guide-card generator (or a doc param on the proposal one) + essay 批注 doc-keying (`ReplaceAnnotations(docKind)`, `?doc=` endpoints).
- `EssayGuide` overlay + `useEssayTrack` hook + essay-track endpoints (or generalized proposal-track).
- The 写作卡 offer affordance on claim steps.

---

## Doc consistency check (`all-statuses.md §6 stage-2`)

| §6 point | This slice | Verdict |
|---|---|---|
| Outline initialized by the main RQ + 2–4 sub-questions, student modifies per evidence | Pillar 1 outline seed | ✅ |
| Write the argument paragraph for each claim one by one, with guidance | Pillar 1/2 per-claim steps + guide cards | ✅ |
| Each claim: evidence / analysis / limitation / how it correlates | Pillar 2 claim-step prompt | ✅ |
| 写作卡 (PEE / 论证解剖) offered when needed, only here | Pillar 2 写作卡 offer on claim steps | ✅ |
| Then comparison / 综合 | Pillar 1 `synthesis` step | ✅ |
| Then a conclusion paragraph | Pillar 1 `conclusion` step | ✅ |
| Then a paragraph describing the full 论证结构 | Pillar 1 `structure` step | ✅ |
| Each guided card has AI support / AI comment (two buttons) | Pillar 2 我依然有问题 / 我写好了 | ✅ |
| Challenge 反方观点 when a part needs it | a DEDICATED `challenges` step in the walk (Pillar 1) | ✅ (separate step so it's not skipped) |
| Claims can be modified (rephrase vs total change; warn when materials/writing no longer apply) | Pillar 1 "Editing the claims" | ✅ (rephrase keeps materials; total change warns; un-claimable flagged; RQ change → reading) |

## Resolved (from your steer)

- **Flow:** 大纲 page with 印记's full plan-intro + 准备好了吗？ ready gate → 片段 page for focused per-claim writing (Pillar 2). ✅
- **Per-claim card:** `GuidedWritingCard` — clear guidance + **auto-growing** textarea + 我依然有问题/我写好了 + 写作卡. ✅
- **Editing claims:** editable, with 印记 distinguishing rephrase (keep materials) from total change (warn: materials/writing inapplicable) + flagging un-claimable questions; total RQ change → reading. ✅
- **反方观点:** a **dedicated `challenges` step** in the walk (not just cross-cutting cards — else it's skipped). ✅
- **Endpoints:** reuse the proposal-track shape via `?doc=`, but doc-specific step derivation (proposal ≠ paper). ✅
- **Split:** yes — **4b-1** (doc-key 批注 + essay track + outline seed + `GuidedWritingCard` + per-claim writing + the ready gate) then **4b-2** (写作卡 offer + synthesis/challenges/conclusion/结构 steps + claim-edit warning + statement→submission advance). ✅

## Still open for you

1. **3a reconciliation:** apply the `GuidedWritingCard` (guidance + per-part auto-growing textarea) back to **3a's proposal parts** too — so both match §4's per-part "snippet writing frame" — or leave 3a as-is and only build the correct model for the essay in 4b? *(I lean: reconcile 3a; it's the same primitive and 3a currently diverges from §4. If yes, it becomes a task in 4b-1.)*

## Out of scope / follow-ups

- 4c: submission (引言/conclusion/compose/polish loop + finish→review).
- Slice 5: topic→framework merge + reading-page UI.
- Deep sub-question re-decomposition in the statement stage (light affordance in 4b).
- No fancy layout editor, no submitting for the student, no gamification.
