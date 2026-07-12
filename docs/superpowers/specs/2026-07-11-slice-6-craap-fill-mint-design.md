# Slice 6 — CRAAP fill→mint (the keystone) · Design

**Date:** 2026-07-12
**Refactor:** whole-product refactor #2 (`docs/2026-07-11-whole-product-refactor-roadmap.md`)
**Predecessor carry-forward:** Slice 5c-2 shipped the CRAAP tool-card *transport* (surface → submit → mint plumbing) but could not mint from a real fill, because the UI produced `field_values` while `EvaluateCompletion` reads **anchors**. Slice 6 closes that gap.

---

## Goal

Light up the CRAAP fill→mint bridge so a real student can complete a source-evaluation card end-to-end: **AI anchors per-dimension questions on source spans → student answers each + writes 作用与风险 → lock → the answered anchors satisfy `EvaluateCompletion` → `CompleteCard` mints the evidence node + `evaluated-as` edge → the `every_source_evaluated` gate flips.**

This is the first slice in which the platform is genuinely end-to-end *completable* by a student. Everything the mint needs already exists (Slice 3 effects + 5c-2 submit endpoint); Slice 6 is fundamentally a **port + wire**, not a from-scratch build.

## Scope

**In scope — S3 vertical only:** the CRAAP (source-evaluation) fill→mint round-trip through the Studio coach rail.

**Explicitly deferred (own later sub-slices):**
- **S2 source log** (search plan → auto-log opened sources → citations only from log, RL-2) → Slice 6b.
- **S3 lateral / SIFT** (leave the source, check the claim across others) → Slice 6c.
- **Student *free* span-creation from text selection** — in Slice 6 the AI circles the spans (guidance level L1, below). Student-authored spans arrive with higher guidance levels.
- **The consolidation framework reveal animation** (R-9 summing-up panel; `consolidation: reveal_framework_after_completion`). The functional mint + lock ships now; the summing-up framework panel is a follow-up.

## Binding sources

- **Design source of truth:** `docs/design/思维印记_工作区.dc.html` — the CRAAP coach-rail card is pinned at `s3CoachCard` (lines ~1282–1320); the material-annotation state model at lines ~2015–2062; the summing-up framework at lines ~2063–2071. Copy is binding verbatim; do not paraphrase.
- **Product spec:** `docs/2026-07-11-product-spec.md` §S3 (evaluate the sources; vertical CRAAP).
- **Agent spec:** `docs/2026-07-11-agent-spec.md` §3 (annotate primitive; completion/observe as closed typed predicates).

---

## The fill model (guidance level L1) — and the ladder it sits on

"Who writes the question / who circles the relevant span" is a **deliberate scaffolding axis** (gradual release of responsibility — I-do → we-do → you-do), *not* a fixed fact. The `author` field on each anchor is the seam that encodes it:

| Level | Writes the question | Circles the span | Student does | This slice |
|---|---|---|---|---|
| **L1 · novice** | AI | AI | answers | **← builds this** |
| **L2** | AI | student | locates + answers | future config |
| **L3** | student (from angles) | student | elicits + locates + answers | future config |

**Slice 6 builds L1**, because that is what the binding `.dc.html` pins. But the architecture MUST keep the fade as **future config, not a rewrite** (see Guardrail below).

### L1 interaction (design-pinned)

1. **AI anchors the source.** When CRAAP is surfaced (an `annotate`-primitive card), the AI generates one anchored guiding question per CRAAP dimension over the target source material — each an `Anchor{ material_id, block_id, start, end, quote, dimension, question, author:"ai", answer:"" }`, where **`dimension` is the completion tag** (`currency` / `relevance` / `authority` / `accuracy` / `purpose`) and the question is seeded from `params.tag_prompts[tag]`. This adapts the existing `AnchorGenerator` (see the **generator↔completion reconciliation** below — a required backend change, not pure reuse).

**The generator↔completion reconciliation (deeper than 5c-2's disjunction).** `AnchorGenerator` today keys generated anchors off `spec.Steps[].Title` (e.g. `dimension = "C · Currency 时效性"`), but `EvaluateCompletion`'s `every_tag_present` checks for `dimension == "currency"` (the `completion` / `params.tags` names). Answered step-title anchors could therefore *never* satisfy completion — a second, deeper data-model disjunction. Slice 6 fixes it: for a card carrying `params.tags` (the C2 annotate cards), the generator keys anchors off **`params.tags`** with `dimension = tag` and the question seeded from **`params.tag_prompts[tag]`**; the legacy step-title path is preserved only for cards *without* `params.tags`. This is the load-bearing correctness change of the slice — without it, no live CRAAP fill can mint.
2. **Student answers each + writes 作用与风险.** In the coach-rail CRAAP card, each anchored question renders as: dimension chip + question text + an answer `textarea` (+ a ✓ once answered). Below the dimensions, a **作用与风险（自己写）** textarea produces the `risk_note` anchor (`author:"student"`). The left 素材 article pane highlights the same spans (reusing the `Annotate` renderer), pairing left-highlight ↔ right-question.
3. **Lock → mint.** The lock button submits the filled anchors through 5c-2's existing `submitProjectCard`. `EvaluateCompletion` requires: an answered anchor for each of the 5 tags (`currency, relevance, authority, accuracy, purpose`) **and** a student-authored `risk_note` anchor. On complete → `CompleteCard` → mint + `evaluated-as` edge → gate flip → one refeed `RunAgentStep`.

### Guardrail: keep the guidance ladder open

- **The answer renderer is authorship-agnostic.** `StudioAnnotateCard` (below) renders whatever anchors it is handed — question + span + answer field — and never assumes the AI authored them. L2/L3 change only *where the anchors come from* (student creation upstream), never the answer renderer or the submit→mint path.
- **The only L1-specific piece is the surface-time anchor generation.** It is isolated in one place (the Studio surface path). Higher levels populate anchors from student action instead; everything downstream is unchanged.
- A short "Future: guidance levels" note goes in the code near the generation seam so no implementer bakes in "AI always anchors."

---

## Architecture & data flow

```
coach surfaces CRAAP  ──▶  RunAgentStep → Action{surface_card, CardID:"craap", CardInstanceID}
                                   │
        (studioturn.go streamAction — the ONE new backend seam)
                                   ▼
   spec.Primitive == "annotate"?  ──▶  AnchorGen.Generate(ctx, spec, materials)
                                   │        └─ persist: SetCardInstanceAnchors(projectID, cid, raw)
                                   ▼
                     em.Card(cid, cardID, spec.Name, raw)      ← was []byte("[]")
                                   │  (SSE `card` event carries anchors)
                                   ▼
   client: conversation.ts stores card{ anchors }  ──▶  CoachRail forks on primitive:
                                   │                         annotate → StudioAnnotateCard
                                   ▼
   student answers each anchor + writes risk_note  ──▶  Lock
                                   │
                     submitProjectCard(projectId, cid, { anchors })   ← 5c-2, unchanged
                                   ▼
   validateAnchors → SetCardInstanceAnchors → CompleteCard → (mint evidence + evaluated-as edge) → gate flip → refeed
```

### Backend

**`apps/api/internal/agent/anchors.go` — dimension-keyed generation (the load-bearing correctness change).**
- Responsibility: for a card carrying `params.tags`, generate one anchor per tag with `dimension = tag` and the question seeded from `params.tag_prompts[tag]`, so answered anchors satisfy `EvaluateCompletion`'s `every_tag_present`. Preserve the legacy `Steps[].Title` path only for cards without `params.tags`.
- Touches both the LLM prompt (`buildAnchorPrompt` — list the tags + tag_prompts; require the model's `dimension` ∈ tags) and `fallbackAnchors` (one anchor per tag, `dimension = tag`, `question = tag_prompts[tag]`, unanchored `quote`/offsets).
- The deterministic fallback must therefore cover all of `params.tags` — this is what lets the offline backend test reach completion via generated anchors + student answers, with no key.

**`apps/api/internal/api/studioturn.go` — `streamAction` (the surface-wiring seam).**
- Responsibility: for a `surface_card` action whose spec `Primitive == "annotate"`, generate the AI anchors, persist them, and emit them on the `card` SSE event instead of the hardcoded `[]byte("[]")` (currently line ~192).
- Consumes: the project's source materials via `ListMaterialsByProject`; an `AnchorGenerator` on the API deps (wire it as `turn.go:136` already does for the legacy path: `agent.NewAnchorGenerator(provider, chatResolver)`).
- Produces: persisted `card_instance.anchors` (via `store.SetCardInstanceAnchors(projectID, cid, raw)`) and the same `raw` on the SSE `card` frame. On generator error or empty result, degrade to `[]byte("[]")` (card still surfaces; student sees no pre-anchored questions — safe, not a crash).
- Because `streamAction` is shared by `postProjectTurn` and `submitProjectCard` (a refeed can surface a new card), anchor-generation-on-annotate-surface applies uniformly to both — correct by construction.

**Naming reconciliation.** `cards.Spec` carries **both** `Mode` (legacy, `"annotation"`) and `Primitive` (C2, `"annotate"`). The legacy `turn.go` keys off `Mode == "annotation"`; the Studio path keys off `Primitive == "annotate"` (the C2 truth). Do not add a third predicate; use `Primitive`.

**Reused untouched:** `EvaluateCompletion` / `CompleteCard` / `GraphEffects` mint (`agent/card_completion.go`, `card_effects.go`, `card_lifecycle.go`), `submitProjectCard` + `validateAnchors` + `SetCardInstanceAnchors` (5c-2), the `card` SSE frame (already carries `anchors`). (`AnchorGenerator` is *modified*, not reused — see above.)

### Frontend

**`apps/web/src/studio/StudioAnnotateCard.tsx` (new) — the annotate answer-mode host.**
- Responsibility: render the design's `s3CoachCard` for an `annotate` card: header (工具卡 · CRAAP + category + 评估这条来源), one row per anchor (dimension chip + question + answer `textarea` + ✓-when-answered), a 作用与风险（自己写）`textarea`, and the lock button.
- Consumes: `{ spec, anchors, onLock, onSkip }` where `anchors: Anchor[]` are the AI-generated questions.
- Produces (on lock): the anchor array with each dimension's `answer` filled from the student's text, **plus** a `risk_note` anchor `{ dimension:"risk_note", author:"student", answer:<role/risk text>, material_id:<source>, block_id:"", start:0, end:0, quote:"", question:<fixed prompt> }`. Handed to `submitCard`.
- Authorship-agnostic: it renders the anchors it is given; it does not know or care that the AI authored them (the ladder guardrail).
- Kept **separate** from `StudioCardSheet` (which stays the schema-field host for non-annotate cards). `CoachRail` forks on `spec.primitive`: `annotate → StudioAnnotateCard`, else `StudioCardSheet`.

**`apps/web/src/studio/conversation.ts` — `submitCard`.** Send `{ anchors }` (not a field-values envelope) for annotate cards. `projectCards.ts.submitProjectCard` already POSTs `{ anchors }` (5c-2); verify the shape matches `validateAnchors` (all anchor string fields present; `start`/`end` numeric).

**`apps/web/src/workspace/material/SourceDossier.tsx` (thin include).** When the open source has live anchors, feed them to the existing `Annotate` renderer so the left article pane highlights the AI-circled spans (left-highlight ↔ right-question pairing). The mint does **not** depend on this; it is the design's paired presentation and falls out of the existing renderer.

**Reused untouched:** the `Annotate` span renderer (`primitives/annotate/`), the `card` `StudioTurnEvent` variant + `mapStudioFrame` (5c-2, already carries `anchors: Anchor[]`).

---

## craap.json (single source of truth) — leave `steps` untouched

The binding truth for completion is `completion` (anchors) + `primitive: "annotate"` + `observe`. The elaborate `steps`/`fields` block is the **consolidation framework** revealed at summing-up (`consolidation: reveal_framework_after_completion`; design L2063), **not** the primary fill. Slice 6:
- **Does not** rework the `steps`/`fields` block (it renders later, in the deferred R-9 reveal).
- **Does not** change `completion`, `observe`, `graph_effects`, or `primitive`.
- Treats the latent field-key/dimension naming inconsistency inside `steps` as out of scope (it is inert until the summing-up reveal slice touches it).

No new card JSON is added. No renderer gains a new primitive — `annotate` already exists (Slice 1).

---

## Testing (non-vacuous by construction)

The 5c-2 whole-branch review's core lesson: a mint test that hand-builds satisfying anchors is **vacuous** because it bypasses the real fill path. Slice 6's tests MUST exercise the real path:

- **Backend (Go, testcontainers + deterministic fallback, no key):** surface an `annotate` card in the Studio path → assert the persisted `card_instance.anchors` is non-empty and that each anchor's `dimension` is exactly a **completion tag** (`currency`/`relevance`/`authority`/`accuracy`/`purpose`) — **not** a step title. (This assertion directly guards the generator↔completion disjunction; without the fix it fails.) Then submit those anchors with student answers filled + a `risk_note` anchor → assert `CompleteCard` returns complete → assert a **new** evidence node is minted (by id-delta, with a `source_quality` body) and the `evaluated-as` edge exists. A negative case: submit with a missing dimension → stays `active`, no mint (the 5c-2 gating regression).
- **Frontend (vitest):** `StudioAnnotateCard` renders one answerable row per supplied anchor; filling all rows + the risk-note enables lock; lock calls `submitCard` with anchors whose `answer` fields are populated and a `risk_note` anchor present. Prove the assertion is load-bearing (break the risk-note wiring → test fails).
- The end-to-end regression the review demanded: the mint is driven by the **UI answer path**, not hand-built anchors.

---

## Acceptance criteria

1. Surfacing CRAAP in the Studio generates + persists + emits per-dimension AI anchors whose `dimension` values are the **completion tags** (`currency`…`purpose`), via the reconciled `AnchorGenerator` (deterministic fallback offline); the `card` SSE frame carries them (no longer `[]`).
2. The coach-rail CRAAP card renders each anchor as an answerable question + a 作用与风险 field, matching the binding `s3CoachCard` design; the left 素材 pane highlights the paired spans.
3. Answering all 5 dimensions + writing the risk note + locking submits anchors that satisfy `EvaluateCompletion`; `CompleteCard` mints a new evidence node + `evaluated-as` edge and flips `every_source_evaluated`; a refeed step runs.
4. An under-filled submit stays `active` with no mint and no false-complete (5c-2 gating preserved).
5. Tests exercise the real fill path (non-vacuous); the guidance-ladder guardrail is documented at the generation seam.
6. Deferred items (S2 source log, SIFT lateral, student free span-creation, R-9 framework reveal) are recorded in the roadmap as follow-ups.

## Out of scope / non-goals

- Any S2 source-log surface, search plan, auto-logging, or citation gating.
- SIFT / lateral reading.
- Student-authored spans or questions (guidance levels L2/L3) — architecture keeps them open; UI does not build them.
- The summing-up framework reveal animation.
- Retiring the legacy `RunTurn`/`turn.go`/old `workspace/` (that is Slice 5d, the routing cutover).
