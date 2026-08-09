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

| # | key | Kind | Title | tab | guidance |
|---|---|---|---|---|---|
| 0 | `outline` | fixed | 搭大纲 | 大纲 | modify the outline (seeded from RQ + sub-questions) per your evidence |
| 1..N | `claim:<sqId>` | subq | 论点 N | 片段/正文 | write this claim's argument: evidence / analysis / limitation / how it links to the next; 写作卡 offered |
| N+1 | `synthesis` | fixed | 比较 / 综合 | 正文 | compare/synthesize the claims |
| N+2 | `conclusion` | fixed | 结论 | 正文 | the conclusion the claims support |
| N+3 | `structure` | fixed | 论证结构 | 正文 | a paragraph describing the whole argument's structure |

- The per-claim steps come from `ProposalTrack.SubQuestions` (as possibly modified — §6 says the statement stage lets the RQ + sub-questions be modified; 4b MVP reads the proposal's set, with a light "edit sub-questions" affordance deferred if large).
- The **outline is seeded** from the main RQ + sub-questions on entering statement (if the outline is empty): `SeedEssayOutlineFromProposal` writes outline rows (RQ as a top node, each sub-question as a child) via the existing outline store. §6: "outline initialized by the main research question and 2-4 subquestions."

---

## Pillar 2 · The guided scaffold (`EssayGuide`, overlay on the essay surface)

Mirrors `ProposalGuidePane` (3a) but over the essay's 大纲/片段/正文 surface. The scaffold is a colored guide card for the current step; it **points the student to the relevant tab** (it does not replace the tabs — the student writes in 大纲/片段/正文 as today, the scaffold guides).

- Guide card (AI-generated, essay-worded, cached): for a `claim:<sqId>` step the prompt walks evidence / analysis / limitation / linkage for THAT claim (reuse `GenerateProposalGuideStep` with an essay variant, or a new `GenerateEssayGuideStep`).
- Two buttons (§6): **我依然有问题** → coach chat (fast); **我写好了** → 批注 on the essay draft (doc-keyed, Pillar 0), surfaced in the left panel.
- **写作卡 offer (§6):** on `claim:*` steps, a "需要帮你把论证搭起来吗？" affordance offers **PEE / toulmin / argument-map** (already summonable in the essay deck) — student-tap opens the card. Only on claim steps (§6: 写作卡 only needed here).
- prev/next nav; the flexible entry from 4a ("去写这条论点") deep-links to that claim's step.
- Endpoints mirror `proposal-track`: `GET/mode?/start?/advance` on an `essay-track/statement` path (or reuse the proposal-track shape with a doc param). **Decision to confirm:** reuse the proposal-track endpoints generalized by `?doc=`, vs a parallel `essay-track/statement` set.

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
| Challenge 反方观点 when a part needs it | reuse concession/perspective cross-cutting cards + coach nudge | ⚠️ via cross-cutting cards (not a dedicated step) — flag |
| (modified question + subquestions) | 4b reads the proposal's set; a light in-statement edit affordance | ⚠️ deferred/minimal — flag |

## Open design questions for you

1. **Scaffold vs surface:** I propose the `EssayGuide` scaffold **overlays** the existing 大纲/片段/正文 tabs (guides + points to the tab), not a redesigned surface. Good, or do you want the per-claim writing to happen inside the scaffold card itself (like the proposal guide)?
2. **Sub-question editing in the statement stage:** §6 says the RQ + sub-questions can be modified here. Full edit is sizable — is a light "these are your claims; tweak the wording" affordance enough for 4b, with deeper re-decomposition deferred?
3. **反方观点:** handle via the existing concession / perspective-matrix cross-cutting cards + a coach nudge (my proposal), or a dedicated "challenges" step in the walk?
4. **Endpoints:** generalize the proposal-track endpoints with `?doc=` (less code, shared shape), or a parallel `essay-track/statement` set (clearer separation)? I lean generalize.
5. **Split:** 4b is ~3a+3b combined. Split into **4b-1** (doc-key 批注 + essay track + outline seed + per-claim scaffold) and **4b-2** (写作卡 offer + synthesis/conclusion/结构 steps + polish)? Or one slice?

## Out of scope / follow-ups

- 4c: submission (引言/conclusion/compose/polish loop + finish→review).
- Slice 5: topic→framework merge + reading-page UI.
- Deep sub-question re-decomposition in the statement stage (light affordance in 4b).
- No fancy layout editor, no submitting for the student, no gamification.
