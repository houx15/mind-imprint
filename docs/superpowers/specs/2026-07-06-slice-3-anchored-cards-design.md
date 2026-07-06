# Slice 3 · Material-Anchored Card Engine — Design Spec (the keystone)

> 2026-07-06 · Part of `docs/2026-07-06-refactor-roadmap.md` (Slice 3 of 5). Stacked on `slice-2-material-substrate`.
> Binding design: `思维印记 工作区.dc.html` — `TASKS: WORKSPACE` CRAAP conversation-branch + `material-annotation model` sections.
> Authority: backend/DB/API per `docs/architecture/*` (Go+Postgres). UI per the `.dc.html`.
> **Autonomy note:** produced under the user's "loop until the refactor finishes" mandate — design decisions are made here and documented; no synchronous approval gate.

## Goal

Turn the agent's card proposal from a blank form into **material-anchored guided questioning**: for annotation-mode cards, the agent reads the task's materials and, per card dimension, emits a **guiding question anchored to a specific material span**; the card renders inline as a "对话分支" with those anchored questions; the student answers, can add their own anchored question, and submits it to the process tree. The card's abstract framework appears only at close. Non-annotation cards keep the existing form behavior.

## The Anchor model (the crux — flows through every sub-slice)

New `Anchor` (Zod `packages/contracts/src/anchor.ts`, mirrored in Go), stored as an `anchors` array on the card envelope:

```
Anchor = {
  id: string,
  material_id: string,
  block_id: string,          // references a Material block (Slice 2: blocks[{id,text}])
  start: number,             // char offset into that block's text
  end: number,               // exclusive end offset
  quote: string,             // the anchored substring (display-robust if offsets drift)
  dimension: string,         // the card dimension this anchors (a step title, e.g. "权威性 · Authority")
  author: 'ai' | 'student',  // AI-generated guiding question vs the student's own
  question: string,          // the guiding question (ai) or the student's question (student)
  answer: string,            // the student's response; "" until answered
}
```

- **AI anchors** (`author:'ai'`) are generated at card-summon time, one per card dimension, each pointing at a relevant span with a specific question.
- **Student anchors** (`author:'student'`) are added by the student ("在右侧文章里划一句，自己向印记提问") — the "invite-student-question" step.
- `answer` is filled as the student responds; on submit the whole `anchors` array (with answers) persists on the card instance and is refed to the model.

`anchors` lives on `CardInstance` alongside `field_values`/`event_trace` (agent- and student-authored annotation layer; not the student form namespace).

## Annotation mode

A card spec gains `mode: 'annotation' | 'form'` (default `'form'`). Slice 3 enables annotation for the acceptance-mainline cards **`sift_craap`** and **`concession`** (set `"mode":"annotation"` in their `packages/contracts/cards/*.json`); all other cards stay `form` and are unaffected. Dimensions for anchor generation = the card spec's **step titles** (no need to widen the Go loader's methodology parsing — step titles carry the dimension intent, and the model gets the material text to ground its questions).

---

## Decomposition (sub-slices, each spec→plan→TDD→review, stacked & kept)

### Slice 3a · Anchor data model + persistence (backend)
- `packages/contracts/src/anchor.ts` (`Anchor` Zod) + `anchors: z.array(Anchor).default([])` on `CardInstance` in `envelope.ts` + `mode` on `CardSpec` in `cardSpec.ts` + tests + barrel export.
- Migration `0010_card_anchors.sql`: `ALTER TABLE card_instances ADD COLUMN anchors jsonb NOT NULL DEFAULT '[]'`. Extend `queries/cards.sql` (`CreateCardInstance` gains `anchors`; `SubmitCard`/`SkipCard` RETURNING already `*`; add `SetCardAnchors` for the summon-time write). Regen sqlc.
- `cards.go`: `validateAnchors(raw)` at the HTTP boundary (array; each element an object with string `block_id`/`dimension`/`question`/`quote`, `author` ∈ {ai,student}, numeric `start`/`end`, string `answer`); `putCard`/`skipCard` accept + persist `anchors`. `cardDTO` gains `anchors json.RawMessage`.
- Go loader `Spec` gains `Mode string` (parsed from the JSON).
- Fully testable (contract tests + testcontainers round-trip + handler boundary tests). No LLM.

### Slice 3b · Agent anchor generation (backend)
- `TurnStore` gains `ListMaterials(ctx, taskID) ([]Material, error)`; `RunTurn` loads materials before building messages; a new `BuildMaterialContext(materials)` helper injects each material's blocks **with block ids** into a leading context message so the model can reference spans.
- When the agent summons an **annotation-mode** card, generate anchors: follow the **eval pattern** (`gateway.Collect` + parse-with-retry, precedent in `agent/eval.go`) — a focused prompt gives the card's dimensions (step titles) + the material blocks and asks for a JSON array of anchors (`{block_id, quote, dimension, question}`; offsets computed server-side from `quote` via `strings.Index` into the block). Persist via `SetCardAnchors` on the proposed instance.
- **Deterministic fallback:** if generation returns nothing/invalid, or the card is annotation-mode but there is no material, fall back to one unanchored anchor per dimension (`block_id:""`, `question` = a generic prompt from the step title). The card still works.
- Tier: reuse the chaperone resolver for now (documented; a flagship route is a future option).
- Testable with the gateway **stub provider** returning canned anchor JSON + material fixtures; the parse/offset/fallback/persist logic is deterministic and unit-tested. Real-model quality is an integration concern (as with eval).

### Slice 3c · Frontend anchored card-branch + material highlighting (frontend)
- Replace the current bottom-sheet card host for annotation cards with the design's inline **"对话分支"**: per-dimension anchored question cards (dim chip + question + textarea for the answer), an "在右侧文章里划一句，自己向印记提问" affordance (adds a `student` anchor), and "提交并钉到过程树".
- **Material highlighting** in `MaterialPane`: split each block's text at its anchors' offsets into runs; highlighted runs are clickable and sync with the branch (clicking a question highlights its span; clicking a span focuses its question). Reuses Slice 2's block rendering.
- Submit: the branch's anchor answers (+ any student anchors) submit through the existing `putCard` (now carrying `anchors`), creating the process-tree `card_use` node.
- **Methodology "怎么用" modal** (工具说明书) and the **close-out abstract framework** (the card's dimensions shown as a transferable takeaway after submit) — foldable into 3c or a small 3d.
- Testable with mocked anchors/materials (RTL); no LLM.

**Out of scope for Slice 3:** courses; voice; PDF; the second working portal; teacher surface; rolling annotation mode out to all cards (only `sift_craap`+`concession` this slice); real-model anchor-quality tuning.

## Invariants preserved
Client never calls the model; ownership 404-hidden; one card per turn; standard envelope is the spine; process tree read-only; AI-restraint ladder unchanged. The anchor write happens server-side in the turn; the client only reads anchors (via the card row) and submits answers.

## Testing strategy
- Contracts: `anchor.test.ts` + envelope/cardSpec updates.
- Go 3a: testcontainers round-trip for `anchors` persistence; `cards.go` boundary tests (valid/invalid anchors).
- Go 3b: stub-provider tests for anchor generation (canned JSON → parsed anchors with computed offsets), the fallback path, and material injection into the prompt (golden-ish assertion that block ids appear).
- Web 3c: branch renders anchors from a mocked card; answering + submitting calls `putCard` with anchors; material highlighting renders runs from anchor offsets; student-question add appends a `student` anchor.

## Risks / decisions recorded
- **Anchors on the envelope vs summon tool-call:** chosen the envelope + a server-side `SetCardAnchors` write (anchors are agent-authored data, not a student form; keeps typed validation).
- **LLM structured output:** chosen the eval-style `Collect`+parse+retry over a new tool-use tool (closest existing precedent; avoids tool-schema churn). Offsets derived server-side from `quote` to avoid trusting model arithmetic.
- **Mode rollout limited to 2 cards** to bound the slice; the mechanism is card-agnostic and extends by flipping `mode` in more card JSONs later.
- Non-determinism of real anchor quality is unverifiable in autonomous execution; mitigated by the deterministic fallback so the feature degrades gracefully, exactly as eval does.
