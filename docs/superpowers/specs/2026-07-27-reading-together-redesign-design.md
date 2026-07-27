# 印记陪读 · Read-Together Redesign — Design Spec

> **Status:** Approved design (2026-07-27). Authoritative source for the implementation plan.
> **Scope:** Redesign the writing-studio 信源 (source-reading) experience into a dedicated, focused "read-together" page that absorbs the interaction model of the colleague's `docs/reference/mind-imprint-card-agent-demo/`, built on our existing infrastructure.

---

## 1. Goal

When a student opens a source, take them out of the crowded 3-pane studio and into a **calm, focused read-together page** where the AI reads one paper *with* them: it draws one worked example on a sentence, then the student **underlines their own evidence sentence** and the AI evaluates that pick. The page supports **both** source-checking (CRAAP/SIFT — "is this trustworthy?") **and** deep reading of the content (argument, framing, data, perspective — "what is this really saying, and how solid is it?").

**One sentence:** keep the demo's span-anchor + hanging-card + you-find-the-evidence + program-owned-verdict skeleton; power it with our own 36-card library, gateway, and anchor contract; deliver it on a dedicated focused page.

## 2. Why (the problem today)

The current 信源 experience splits **one task across two columns**: the article renders in the studio's **center pane** (`SourceDossier` article mode) while the CRAAP/SIFT card mounts in the **right rail** (`CoachRail`) — the rail even shows a `CraapPlaceholder` that literally tells the student "互动卡片见「素材」环节" (go look at the other pane). Cards arrive via an async refetch (`prepareSourceAnnotation` → full project refetch), so they "pop in from elsewhere," and on LLM slowness/failure the student gets an article with no highlights and no card (best-effort, silent). Anchors whose model `quote` isn't a verbatim substring get `(0,0)` offsets and **light up nothing**. Three overlapping identities (`craap` + `sift` split cards vs the `sift_craap` combined card + its teaching copy) mean the methodology shown doesn't match the interaction driven. Reading state is ephemeral.

The fix is structural: **one focused reading surface where everything happens inside the article**, the card physically hangs on the sentence and moves from the AI's example to the student's pick, and reading is the only thing on screen.

## 3. Design laws this must honor (from AGENTS.md)

1. **AI 克制** — the AI never concludes for the student. It draws *one* example and poses *one* question; the student does the finding. Restraint ladder = normal reply / light hint / formal summon.
2. **不操纵** — no streaks/leaderboards/nags. Cards are auto-*summoned* but the student confirms the *open*. Pacing gates prevent treadmill nagging.
3. **一次只问一个** — exactly one active card, one primary action, one next-step at a time.
4. **过程即数据** — skips, re-picks, follow-ups, and reading time are all recorded as signal.

## 4. What we reuse (infrastructure already in place)

| Capability | Where | Role in redesign |
|---|---|---|
| Span-anchor contract (`block_id` + **rune** `start/end` + verbatim `quote` + `dimension`/`author`/`question`/`answer`) | `packages/contracts/src/anchor.ts`, `apps/api/internal/agent/anchors.go:17-28` | The `span_id` model. Unchanged. |
| Block-segmented article (`{id,text}[]`, server-segmented at ingest via `materialize.Segment`) | `project_materials.blocks`, `apps/api/internal/api/materials.go:70` | Article addressability. Unchanged. |
| Block/span highlight renderer + **select-mode** (`selectionToSpan` → `onCreateSpan`) | `apps/web/src/primitives/annotate/Annotate.tsx` | Highlighting + click-to-select-your-own-sentence. |
| Guidance fade L1/L2/L3 (AI writes quote+Q → AI asks, student locates → dimension-only) keyed off `CountCompletedCardUsesByUser` | `apps/api/internal/agent/anchors.go:261-299`, `studioturn.go:335-346` | = the demo's scaffold fade by completion count. |
| Card lifecycle state machine (`proposed → active → evaluating → feedback → completed`, + `skipped`/`revise`) | client card state + `card_instances` | The read-together loop's backbone. |
| Card registry as single source of truth (36 specs, backend `go:embed` + contract JSON) | `apps/api/internal/cards/specs/*.json`, `packages/contracts/cards/*.json`, `registry.ts` | The routable deck is a *subset* of this — no new card JSON. |
| LLM gateway (flagship, never downgraded for evaluation) | `gateway.Collect` | Powers the router and the evaluator. |
| Anchors persist on `card_instances.anchors`; mint/process-tree/evidence | `projection.go`, graph effects | Reading outcomes flow to the existing process tree/assessment. |

## 5. What we build

1. A dedicated focused route + 2-column layout.
2. The **inline hanging-card** that anchors to a span and **moves** from the AI example → the student's pick.
3. The **LLM router** over a ~15-card catalog + the program-side restraint/pacing gate.
4. A uniform **3-check evaluation** with **program-owned verdict** and **server-side evidence-ID filtering**.

## 6. What we retire / reconcile

- `SourceDossier` **article mode** (`apps/web/src/studio/material/SourceDossier.tsx:316-424`) — reading moves to the new page. The **list/archive mode stays** in the studio as the way in.
- The right-rail card mount + `CraapPlaceholder` (`apps/web/src/studio/CoachRail.tsx:124-140,398-458`) on the reading path.
- The `sift_craap` **triple identity**: the combined `sift_craap.json` card + `SiftCraapRenderer` + `SiftCraapTeaching` currently double as the *methodology copy* for the split `craap`/`sift` cards (`projection.go:295-304` `cardMeth`). Reconcile so the methodology a student sees matches the card they actually run. (Detailed reconciliation is a plan task; the constraint: no student sees a framework that doesn't match their interaction.)
- The silent `(0,0)` fallback anchor path (`anchors.go:119-125`) — replaced by strict validation (§9).

---

## 7. The page & navigation

- **Route:** `/studio/:projectId/read/:materialId` (new). A persistent `← 返回工作区` control returns to the exact prior studio state.
- **Entry:** clicking a source row in the studio source list **navigates** to this route (replacing the old in-place `activateSource` → center-pane article flow).
- **Exit:** back control → studio, source list reflects any new locked/评估 state.
- **What comes along:** nothing but the reading room. No `StationRail`, no full `CoachRail`, no `ViewFrame` tabs.
- **Resumability:** if a `card_instance` for this material is open (`proposed`/`active`/`evaluating`/`feedback`) it is **restored** on entry; completed reading outcomes for the material are always shown. (We already persist card instances; this is a modest gain over today's "restart".) In-flight, pre-submit selection is client-held.

## 8. Layout — engineered for focus

Hard focus mandate (the student must feel they are reading *one paper*, not operating a console):

- **Two columns, ~39 : 61** — coach left, article right. (Demo: `grid-template-columns: minmax(360px,.78fr) minmax(600px,1.22fr)`.)
- **The active card lives *inside* the article column, hanging on a sentence** — never in a side rail. A connector line ties it to its anchor paragraph.
- **Article typography:** serif reading font, `--content-width: 736px`, 17px / 1.95 line-height, 20px paragraph spacing. Serif is used **only** for article title/deck/body/quotes; **all UI/cards use the sans UI font.**
- **Left coach column is thin:** the running dialogue plus a single compact "透镜已就绪 · 去文章" pointer — not a busy chat. One "正在看：<句子>" header when a lens is active.
- **One primary button** on the main path at any moment. Exit / re-pick / follow-up / method-notes stay secondary and **collapsed** (`<details>`) until needed.
- **Only the current step is visible:** exemplar → pick a sentence → feedback. Method explanation, follow-up, and save-preview are progressive-disclosure.
- **The ~15-card deck is invisible to the student.** It is the router's menu only. The student ever meets exactly the **one** card the AI summoned; queued follow-up lenses (≤2) are offered one at a time *after* the current one closes.
- Responsive: collapse to single column below ~980px (article first, coach dialogue below), keyboard-selectable sentences, `prefers-reduced-motion` respected.

## 9. The read-together loop

State machine (reusing existing statuses), all rendered **inside the article**:

```
student question / starter-prompt click / source open
  → router decides: respond | hint | summon   (§10)
  → [summon] AI draws ONE example sentence + explains why (≤2 sentences, ≤~70 chars)   status: proposed
      · card hangs UNDER the example sentence
  → student clicks "看懂示范，开始选句"                                                 status: active
      · article enters select-mode; every non-example sentence is selectable
  → student clicks THEIR OWN evidence sentence (single-select; example click rejected)
      · card RE-ANCHORS under the student's chosen sentence
  → student submits                                                                     status: evaluating
  → AI evaluates the pick → 3 checks → program derives verdict (§11)                    status: feedback
  → student may ask a follow-up (grounded on their sentence) or re-pick (→ active)
  → student confirms understanding
  → one merged reading outcome is saved (§12); control returns to coach                 status: completed
  → if a queued lens exists, offer to continue (student decides)
```

**The anchor-move (signature mechanic).** The inline card is rendered **once**, placed after the paragraph containing the current anchor. The anchor is computed from status:

```
anchorSpanId =
  (status ∈ {active, evaluating, feedback} && student has a selection)
    ? student's selected span
    : AI's example span
```

So during `proposed` the card hangs under the AI's example; once the student picks, it moves under the student's own sentence. (Demo: `public/app.js:422-424`.)

**Scaffold fade** (reuse `anchors.go` guidance levels; level from completion count of *that card* by *that user*):
- **L1** (0 completions): AI writes example quote + question + offset (`author:"ai"`); exemplar shown open.
- **L2** (1): AI writes the question only; student locates the span (`author:"student"`, blank span).
- **L3** (≥2): dimension-only; exemplar collapsed; button reads "直接开始选句".

## 10. The router (LLM over a constrained catalog)

Picking the right card among ~15 is content-aware; the existing graph classifier only knows the structural CRAAP→SIFT order and cannot disambiguate content cards. **Decision: LLM router**, with the graph classifier retained only as a source-check ordering guard.

**Trigger points:** student turn (`POST /projects/{id}/turn`), starter-prompt click, and source open (the "read-with" entry, replacing today's `prepareSourceAnnotation` best-effort refetch with a first-class summon on the reading page).

**Router request → model:**
```jsonc
{
  "focused_spans": [ { "block_id", "quote" } ],   // what the student is looking at (if any)
  "student_text": "…",                             // message or starter prompt (may be empty on open)
  "recent_turns": [ … ],                           // short window
  "catalog": [ { "card_id", "one_line_trigger" } ],// the ~15 reading cards only
  "scaffold_levels": { "<card_id>": 0|1|2 },        // per-card completion count → level
  "pacing": { "open_card": bool, "turns_since_last_proposal", "recently_skipped": [ … ] }
}
```
**Router response ← model:**
```jsonc
{
  "decision": "respond" | "hint" | "summon",
  "card_id": "argument-map" | null,
  "reason": "…",                       // why this card, this moment (student-facing nudge text)
  "example_span_id": "p3-s2" | null,   // the sentence to draw the worked example on (summon only)
  "example_why": "≤2 sentences",
  "followup_plan": [ "fact-opinion-value" ]  // ≤2 secondary lenses, queued not shown
}
```

**Program-side gate (deterministic, authoritative — can only DOWNGRADE, never upgrade the model's decision):**
- **One-active mutex** — if any card is `proposed`/`active`/`evaluating`/`feedback`, suppress new summons.
- **Breathing room** — no new proposal within N turns of the last (reuse the demo's `PROPOSAL_BREATHING_TURNS = 2`).
- **Skip cooldown** — a skipped card can't re-fire for N turns (reuse `SKIP_COOLDOWN_TURNS = 3`).
- **New-span reuse** — a completed card won't re-open on the same material unless the student focuses a new span.
- **Source-check ordering guard** — keep `classifier.SurfaceCardCandidates` as a prior: don't summon SIFT before the material has a CRAAP evaluation; don't offer CRAAP on a material already evaluated. The router may not override this ordering.
- **Example-span validation** — `example_span_id` **must** resolve to a real segment and the example `quote` **must** be a verbatim substring of that segment. If not: retry the router **once**; on second failure, degrade to `respond` (no card). **Never** render a `(0,0)` / "lights up nothing" anchor.

**Fallbacks:** router LLM failure → deterministic fallback (respond; or, on the source-open trigger for an un-evaluated article, the classifier's structural CRAAP suggestion). The page never crashes or hangs; a summon-in-progress shows "印记正在标注……" in the article, not a frozen screen.

## 11. Evaluation & integrity (uniform across the deck)

**One shared evaluation shape for every card** — the demo's 3-check evidence rubric. The active card supplies the *framing* (which dimension/lens is in play); the eval *shape* is identical so the mechanic is uniform and the build stays small.

Three checks, each `{ status: pass | partial | miss, evidence, explanation }`:
1. **target — 找对对象**: did the student pick a sentence whose object matches the lens task?
2. **evidence — 看得到线索**: is there a directly citable clue in the sentence? (`evidence` snippet must be **verbatim** from the student's sentence, else dropped.)
3. **centrality — 线索足够关键**: is that clue central enough to answer the task?

**Program-owned verdict** (recomputed server-side from the checks, not trusted from the model):
- `target == miss` → **rethink (暂不匹配)**
- all three `pass` → **strong (高度匹配)**
- otherwise → **partial (部分匹配)**

**Server-side evidence-ID filtering (integrity guarantee):** the model's finding/argument `spanIds` are filtered to the set of spans the **student actually selected**; if either is empty after filtering, the result is **rejected** and re-requested. A finding can never be attributed to a sentence the student didn't pick. (Demo: `agent-core.mjs:807-809`; our current gap: `computeOffsets` silently zeroing.)

**Evaluator fallback:** on evaluator LLM failure, a deterministic signal-word evaluator (per-card `selectionSignals` + method keywords, cf. demo `assessmentForSelection`) produces the same 3-check shape so the loop stays alive and offline demos work.

## 12. Reading outcome

Each closed lens saves **one merged outcome** anchored to the student's span:
```jsonc
{
  cardId, methodId, spanIds:[…], evidenceQuote,
  finding,                         // student's reading finding (provenance: student_selection)
  judgment, support, caveat,       // the argument (provenance: student_selection_and_ai_review)
  review: { verdict, verdictLabel, verdictReason,
            criterionChecks:[{key,label,status,evidence,explanation}], nextStep },
  provenance: "student_selection_and_ai_review"
}
```
- Persists on the existing `card_instances` + flows to the process tree / assessment (unchanged mint path, tightened).
- **Graph effects** come from each card's spec (existing): `craap` promotes material→evidence; `sift` mints `cross_check`. Deep-reading cards record a **reading-insight** outcome node attached to the material; where a card defines no reading-appropriate structural effect, record the outcome node **without** a structural graph mutation (YAGNI — do not invent new graph edges in this spec).
- The reading outcome view merges 发现 / 论证 / 复核 into one card per lens, with a "回到原文继续思考" control that re-focuses the span.

## 13. Data model (no schema changes required)

- `project_materials.blocks` — segmented article, unchanged.
- `card_instances.anchors` — anchors persist here (each carries its own `material_id`); no dedicated anchors table. Unchanged.
- `source_log_entry` — takeaway / tier / `time_spent_s` / `lateral_read`. Reading-time accumulation continues on the new page.
- `MaterialSource` contract (`packages/contracts/src/studioState.ts`) — unchanged; **deliberately no verdict field** on a source (可信/存疑 has no honest single producer).
- No new tables/columns. The router state (pacing) is derived per-turn from existing card-instance/graph state, not stored.

## 14. Error handling (summary)

| Failure | Behavior |
|---|---|
| Router LLM error | Deterministic fallback (respond / classifier structural suggestion). Page stays usable. |
| `example_span_id` invalid / quote not verbatim | Retry router once → else `respond`. Never render a bad anchor. |
| Evaluator LLM error | Deterministic signal-word evaluator produces the same 3-check shape. |
| Evidence-ID filter empties finding/argument | Reject + re-request; never fabricate a citation. |
| URL source fetch failure | Existing ingest path (paste fallback); reading page shows the paste route. |
| Slow summon/eval | In-article "印记正在标注……" indicator; no frozen page, no full-project refetch to reveal the card. |

## 15. Testing

**Backend (Go):**
- Router: decision tiers (respond/hint/summon), the pacing gate (mutex / breathing-room / skip-cooldown / new-span), the source-check ordering guard, downgrade-never-upgrade, and example-span validation (retry-once-then-respond).
- Evaluator: 3-check normalization, program-owned verdict derivation (the three verdict rules), evidence-ID filtering (reject-on-empty), deterministic fallback parity.
- Full `api` package (testcontainers) — not `-run` subsets.

**Frontend (web):**
- Loop state machine (`proposed→active→evaluating→feedback→completed`, `skip`, `revise`).
- Anchor-move rendering (card re-anchors example→student pick).
- Select-your-own-sentence (single-select, example-click rejected, keyboard select).
- Focus/progressive-disclosure contract (one primary button; method/follow-up/save collapsed; deck invisible; one active card).
- Full web suite + typecheck.

## 16. Seams (natural cut points for the plan)

- **Backend:** `prepareSourceAnnotation` (`materials.go:276-345`) — replaced by the reading-page summon; `surfaceAnchors` (`studioturn.go:289-385`) — anchor scope/level policy; `streamAction` (`studioturn.go:243-287`) — summon/intervention frames; `classifier.SurfaceCardCandidates` (`classifier.go:115-184`) — kept as ordering guard; `anchors.go` — guidance levels + strict validation.
- **Frontend:** new `/read` route + page; `Annotate.tsx` (reuse, add inline hanging-card slot + connector); the `CoachRail` card-mount switch (`398-456`) retired on the reading path; `anchorToSpan` merge (`SourceDossier.tsx:52-174`) migrates to the reading page; `StudioContainer` wiring (`620-709`) — navigation replaces `onPrepareAnnotation` refetch.

## 17. Out of scope (explicit)

- Disciplinary reasoning **course packs** beyond the ~15-card reading deck (psychology/politics/ecology lenses) — future, added as catalog data.
- New graph-edge types for deep-reading outcomes — reuse existing effects; record outcome nodes otherwise.
- Multi-article / cross-source reading in one room — the room reads **one** paper (SIFT's lateral source is still surfaced via the existing cross-check flow, not a second article in the room).
- Document editing / submission — we occupy the "thinking" layer; the reading page is read + underline + discuss, never an editor.
- New DB schema.

## 18. The read-together deck (this spec)

Source-check: `craap`, `sift`. Deep reading: `fact-opinion-value`, `argument-map`, `toulmin`, `steelman`, `concession`, `data-literacy`, `opcvl`, `framing`, `spin-detector`, `cda`, `perspective-matrix`, `certainty-spectrum`, `science-knowing`. All are existing specs (backend `go:embed` + contract JSON) and are sentence-based, so all fit the hang-on-sentence + you-find-evidence mechanic. Growing the deck later = adding a catalog entry with a one-line trigger; no page or mechanic change.
