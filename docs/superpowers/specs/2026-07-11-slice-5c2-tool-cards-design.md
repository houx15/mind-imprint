# Slice 5c-2 — CRAAP Tool-Card Round-Trip (surface → fill → mint → refeed) · Design

> Makes the Studio's signature interaction — "the human executes the tool" —
> live. Part of the whole-product refactor #2 (`docs/2026-07-11-whole-product-refactor-roadmap.md`,
> `docs/2026-07-11-agent-spec.md`). Follows Slice 5c (conversational loop,
> `…-slice-5c-interactive-loop-design.md`), which built the loop with
> card-surfacing gated OFF (`AgentDeps.SkipSurfaceCards:true`).

## Known limitation / re-scope (whole-branch-review, post-merge)

5c-2 ships the tool-card **TRANSPORT + plumbing**: surface → open → fill (schema
field renderer) → submit → persist → refeed, all correct and tested. The
**live evidence-node mint is DEFERRED to Slice 6.** Reason: CRAAP is an
*annotation* card — its completion predicates (`EvaluateCompletion`) are
evaluated over **anchors** (material-linked, per-dimension), not over
`field_values`. The schema-driven Card Runtime wired in 5c-2 only ever
produces `field_values` (a plain form fill) — it never produces `anchors`.
Anchors are produced by a **material-annotation surface** (highlighting a
passage in a material and tagging it with a CRAAP dimension), which is
Slice 6's deliverable, not 5c-2's. So today, a real student's form-only CRAAP
submit is a **non-satisfying** submit: `CompleteCard` correctly returns
`complete=false`, the card stays `active` (not `completed`), and no evidence
node is minted. The mint path itself is proven — given a satisfying anchor
set (the shape Slice 6 will produce), `CompleteCard` mints the evidence node
and the gate flips — but that anchor set does not yet exist in the running
product. See the corrected "Acceptance" section below and the "Deferred"
section for the bridge this unblocks.

## Goal

The agent surfaces the **CRAAP** source-evaluation card; the student fills it
(reusing the existing schema-driven Card Runtime); submitting persists the filled
envelope, runs `CompleteCard` to mint an **evidence** node + a
`material --evaluated-as--> evidence` edge, and refeeds **one** `RunAgentStep` so
the coach reacts and the `every_source_evaluated` gate flips live. **One card
(CRAAP)** — the only card `SurfaceCardCandidates` produces today. Debt-free (the
material-anchored card borrows the material's `task_id`).

## Scope

**In:**
1. **Flip `SkipSurfaceCards` to false** in `studioturn.go` (the conversational turn AND the refeed can now surface cards). Add a `surface_card` case → a `card` SSE event; add `Card` to `studioEmitter`.
2. **`AgentStore` seam**: expose the two sqlc queries missing from the interface — `SetCardInstanceStatus`, `SetCardInstanceAnchors` — and a project-scoped submission persist (`field_values`/`event_trace`).
3. **Project-scoped card endpoints** (ownership-checked): `POST /projects/{id}/cards/{cid}/activate` (proposed→active), `POST /projects/{id}/cards/{cid}/submit` (**SSE**: persist filled envelope → `CompleteCard` mint → one refeed `RunAgentStep` → stream the coach's reaction), `POST /projects/{id}/cards/{cid}/skip` (persist status=skipped + event_trace — 过程即数据).
4. **Frontend**: a `card` `StudioTurnEvent` variant; card state in `conversation.ts` (`openCard`/`submitCard`/`skipCard`); the live tool-card slot in `CoachRail` (replace the static `CraapPlaceholder`) **reusing** the schema-driven Card Runtime (`CardSheetHost`/`CardRenderer`/`pickCardBody`/`envelopeReducer`/card-states), project-scoped; a `projectCards` API client.

**Out (later):**
- **The CRAAP fill→mint bridge** — CRAAP is annotation-based; the schema field renderer produces `field_values`, not `anchors`; the live mint lights up with the Slice-6 material-annotation surface. See "Known limitation / re-scope" above.
- **Mid-fill `ObserveCandidates`** (coach nudging a thin dimension answer *while* the card is open) → follow-on. 5c-2 is surface→fill→submit→refeed.
- **More card types** (richer `SurfaceCardCandidates` beyond the hardcoded `craap` — concession/sift/steelman trigger logic) → follow-on.
- **Slice-3 debt / migration** (`task_id` NULLABLE + project-scoped `GetCardInstance`) — NOT needed (material-anchored borrow works) → cleanup slice.
- **Retiring legacy `RunTurn` / the task-scoped card endpoints (`/tasks/{id}/cards/{cid}`)** → 5d.

## Global Constraints (verbatim; bind every task)

- **Client never calls the model directly** — the surface/refeed turns go through the Go gateway; keys server-side only.
- **AI restraint / 一次只问一个:** the refeed is exactly ONE `RunAgentStep` (one action); the coach may stay silent. **Card triggering is automatic, but "opening" the card is the student's confirmation** (AGENTS.md red line #2 — no forced pop-ups; the proposal is a bubble the student chooses to open).
- **AI never executes the tool for the student.** The card is filled by the STUDENT via the Card Runtime; the model never writes `field_values`/anchors. `CompleteCard` mints graph nodes from the STUDENT's anchors (`Author:"student"`), never from a model reply. The enforcement stack is unchanged.
- **过程即数据:** the fill process (`event_trace`) and a **skip** are both persisted as signals — skipping is recorded, not discarded.
- **Single source of truth:** reuse `SurfaceCard`/`CompleteCard`/`GraphEffects`/`EvaluateCompletion` (agent) and the existing frontend Card Runtime — do NOT fork a parallel card renderer or re-implement the mint logic. The card JSON registry (`packages/contracts/cards/`) is the single card spec source.
- **New card = new JSON config, not renderer code** — 5c-2 wires the RUNTIME; it must not special-case CRAAP in the renderer (CRAAP already has a bespoke `pickCardBody` renderer + `craap.json`).
- **Additive; legacy untouched.** No schema migration. The legacy `postTurn`/`RunTurn`/task-scoped card endpoints stay live (retired in 5d). `SkipSurfaceCards` is set explicitly (false) at the studio turn sites; the field's zero value still preserves other callers.
- **Studio discipline:** new endpoints + frontend card host are additive; do NOT touch `AppShell`/`Root`/old `workspace/`.
- **Icons = inline SVG, never `lucide-react`. Binding Chinese design copy verbatim — fix the test, never the copy.**

## Architecture

```
turn (send OR refeed): RunAgentStep(deps{SkipSurfaceCards:FALSE}, projectID, trigger)
  surface_card action (candidate[0] when a Kind="article" material is un-evaluated)
    → studioturn: em.Card(cardInstanceID, "craap", nudgeText, anchors)   [new case]
    → studioTurn.ts yields {type:"card", ...}
    → conversation.ts holds the proposed card
    → CoachRail renders the ProposalBubble (student CHOOSES to open — restraint)

open  → POST /projects/{id}/cards/{cid}/activate   (SetCardInstanceStatus proposed→active)
        → CardSheetHost opens with pickCardBody("craap") over the reused CardRenderer

fill  → local envelope (envelopeReducer: field_change/step_expand/note_open → event_trace)

submit → POST /projects/{id}/cards/{cid}/submit  (SSE)
  loadOwnedProject + entitlement → NewSSEWriter + heartbeat (postProjectTurn shape)
  persist: SetCardInstanceAnchors(cid, anchors) + SubmitProjectCard(cid, field_values, event_trace)
  CompleteCard(spec, cid): GetCardInstance → EvaluateCompletion(anchors) →
           complete==true  → GraphEffects mint evidence node + material--evaluated-as-->node
                             → SetCardInstanceFramework → SetCardInstanceStatus(cid, "completed")
           complete==false → leave status "active" (no mint; student can refill/resubmit) —
                             THIS is the form-only path today (no anchors from the field renderer)
  refeed:  action := RunAgentStep(deps{SkipSurfaceCards:false}, projectID, Trigger{"card_refeed"})
           → stream intervention | gate (every_source_evaluated flipped, once satisfying anchors exist) | (silence) → done

skip  → POST /projects/{id}/cards/{cid}/skip   (SetCardInstanceStatus skipped + persist event_trace; 200)
```

### Why this shape

- **`RunAgentStep` emits one action, no in-step loop** — so the "refeed" after a mint is simply **another `RunAgentStep` pass** driven by the submit. Making submit an **SSE** endpoint (persist + mint + one refeed step + stream) keeps the round-trip self-contained and reuses `postProjectTurn`'s proven SSE structure, avoiding an empty-input continuation turn.
- **`CompleteCard` reads the persisted anchors**, so the submit endpoint persists anchors BEFORE calling it. `CompleteCard` is idempotent (framework-set guard), never sets status (DEC-3) — status is set by the endpoint.
- **CRAAP is material-anchored**, so `SurfaceCard`'s `CreateCardInstance` adapter borrows the material's `task_id` (`GetMaterial(materialID).TaskID`) — no `task_id`-nullable migration needed.

## 1. Backend — surface path (`studioturn.go` + emitter)

- In `postProjectTurn`, set `SkipSurfaceCards: false` (remove the 5c gate). Add to the `switch action.Kind`:
  ```
  case "surface_card":
      em.Card(action.CardInstanceID, <card_id>, <nudge_text>, <anchors>)
  ```
  `Action` carries `CardInstanceID`. **Add `CardID` to `Action`** (set on the surface_card branch from the candidate `c.CardID`) — minimal, additive — so the emitter has the card id. **`nudge_text` is a static/derived string, NOT model-generated:** `SurfaceCard` is a model-free action (no `summon_card` model call like the legacy path), so the surfaced card carries no dynamic coach nudge. Derive it from the card spec (e.g. `spec.Purpose`, or a short fixed prompt); the coach's dynamic reaction comes on the *refeed* turn after submit, not at surface. (A separate coach intervention AT surface would be a second action per turn — violates 一次只问一个.)
- Add `Card(...)` to `studioEmitter` (mutex-delegating to `sse.Card`, like the other methods).

## 2. Backend — `AgentStore` seam + submission persist

- Add to the `AgentStore` interface (`loop.go`) + `sqlcAgentStore` adapter (`agentstore.go`), exposing existing sqlc queries: `SetCardInstanceStatus(ctx, projectID, cardInstanceID uuid.UUID, status string) error`; `SetCardInstanceAnchors(ctx, projectID, cardInstanceID uuid.UUID, anchors []byte) error`.
- Add a project-scoped submission-persist query if none covers `field_values`+`event_trace`: `SubmitProjectCardInstance(id, project_id, field_values, event_trace)` (new `card_instance.sql` query + `make sqlc`), plus a seam method — so the fill process (过程即数据) is persisted. (Anchors + status use the two above.)
- `fakeAgentStore` (loop_test.go) gains the new methods.

## 3. Backend — the card endpoints (`apps/api/internal/api/projectcards.go`, new)

All under `RequireUser`, project ownership via `loadOwnedProject`; card id scoped to the project (mirror the disposition endpoint's membership check — `ListCardInstancesByProject` + membership, 404 if the card isn't in the owned project).

- **`POST /projects/{id}/cards/{cid}/activate`**: `SetCardInstanceStatus(cid, "active")` → 204. (proposed→active; also appends a `card_activated` event.)
- **`POST /projects/{id}/cards/{cid}/submit`** (**SSE**, mirrors `postProjectTurn`): decode `{field_values, event_trace, anchors}`; validate (reuse/adapt the legacy `validateAnchors`/`validateFieldValues`/`validateEventTrace` shape); persist `SetCardInstanceAnchors` + `SubmitProjectCardInstance` + `SetCardInstanceStatus("completed")`; `spec, _ := cards.ByID(cardID)`; `CompleteCard(ctx, deps, spec, cid)`; then one refeed `RunAgentStep(deps{SkipSurfaceCards:false}, projectID, Trigger{"card_refeed"})` → stream `intervention`/`gate`/silence → `done`.
- **`POST /projects/{id}/cards/{cid}/skip`**: decode `{event_trace}`; persist `SubmitProjectCardInstance` (event_trace) + `SetCardInstanceStatus("skipped")`; append a `card_skipped` event → 204.

The submit endpoint builds `AgentDeps` exactly like `postProjectTurn` (Store, Provider, `ChatResolver`-resolved key, `studioSimilarity()`, `Skill:&writing-project`, `SkipSurfaceCards:false`).

## 4. Frontend — reuse the Card Runtime, project-scoped

- **`StudioTurnEvent`** (`api/studioTurn.ts`) gains `{ type:"card"; cardInstanceId; cardId; nudgeText; anchors: Anchor[] }`; the SSE `switch` parses the `card` event.
- **`api/projectCards.ts`** (new): `activateCard(projectId, cid)`, `submitCard(projectId, cid, {field_values, event_trace, anchors})` (**SSE** async-generator returning `StudioTurnEvent`s for the refeed), `skipCard(projectId, cid, {event_trace})`.
- **`studio/conversation.ts`**: add card state to the snapshot — the current proposed/active card (`{cardInstanceId, cardId, spec, envelope}` from the `card` event via `CARD_REGISTRY`/`newEnvelope`) + `openCard`/`submitCard`/`skipCard`. `submitCard` streams the refeed via the SSE client and appends the coach's follow-up (intervention) to `messages`, mirroring `send`.
- **`studio/CoachRail.tsx`**: replace the static `CraapPlaceholder` with the live tool-card slot — a proposal bubble (open on click) → the reused **`CardSheetHost`** (`pickCardBody(spec.id)` over `CardRenderer`, `envelopeReducer` for local fill state) wired to `onSubmit`/`onSkip`/`onClose`. The envelope reuses the existing shape; its `task_id` field is a local artifact (the project submit endpoint takes `{field_values, event_trace, anchors}` and ignores it).
- **`studio/StudioContainer.tsx`**: thread the card callbacks (open/submit/skip) from the conversation controller down through `StudioShell`→`CoachRail`.

## 5. Seed / demo readiness

The 5b seed (migration 0018) already gives the project **two `kind='article'` materials with no `evaluated-as` edge** → `SurfaceCardCandidates` fires CRAAP on them. **Verify** this in the plan (materials are `kind='article'` and un-evaluated); if not, adjust the seed so the demo surfaces CRAAP. No new migration expected.

## Testing

- **Go — seam** (`internal/agent`): `SetCardInstanceStatus`/`SetCardInstanceAnchors`/`SubmitProjectCardInstance` round-trip (testcontainers); `fakeAgentStore` implements them.
- **Go — CompleteCard wiring** (`internal/agent`, may already exist): a filled CRAAP card_instance → `CompleteCard` mints an evidence node + `evaluated-as` edge (assert via the graph) and is idempotent.
- **Go — surface case** (`internal/api`, testcontainers): a `POST /projects/{id}/turn` on the seeded project emits a `card` SSE event for `craap` (materials un-evaluated) + persists the card_instance (proposed). Confirm flipping `SkipSurfaceCards:false` doesn't break the 5c conversational tests (a project with all materials evaluated still yields intervention/silence).
- **Go — submit endpoint** (`internal/api`, testcontainers): activate → submit (with filled anchors) returns SSE with `done`, persists status=completed, mints the evidence node (assert via `ListGraphNodesByProject`), and the refeed `RunAgentStep` runs (a `gate`/`intervention`/`done` in the stream); 404 for a non-owned project / a card not in the project; skip persists status=skipped.
- **Frontend (vitest)**: `studioTurn` parses a `card` event; `projectCards.submitCard` streams; `conversation.ts` holds the proposed card, `openCard`/`submitCard` transition state + append the coach follow-up; the CoachRail renders the proposal bubble → the reused `CardSheetHost` on open; `StudioContainer` wires the card callbacks. Reuse the existing card-renderer tests (unchanged).
- **Gate:** full Go suite (testcontainers, serialized `-p 1`) green; full web suite + `tsc` green; legacy `RunTurn`/task-scoped card endpoints + their tests untouched and green; boundary held.

## Acceptance (live-verify walk)

Fresh migrated DB, `/?studio` (Phoebe demo, S4). Send a message → the agent
**surfaces the CRAAP card** as a proposal bubble in the coach rail (it does NOT
auto-open). Open it → the CRAAP sheet renders (the five dimensions +
risk-note, schema-driven). Fill the dimensions + a risk note → submit → the
fill persists (`field_values` + `event_trace` land on the card_instance) and
the **coach refeed runs** (one `RunAgentStep`; an intervention, a gate update,
or silence streams into the rail, matching the loop's normal vocabulary).

**Deferred to Slice 6:** the sheet does NOT close on a "completed" state, no
evidence node is minted, and the `every_source_evaluated` gate does NOT flip
from this walk. This is the honest, re-scoped behavior — the schema field
renderer produces `field_values`, never the `anchors` `EvaluateCompletion`
requires, so every form-only submit is (correctly) non-satisfying and the
card stays `active` for refill/resubmit. The **mint path itself is verified**,
just not via this walk: given a hand-built satisfying anchor set (the shape
Slice 6's material-annotation surface will produce), `CompleteCard` mints the
evidence node, sets the edge, and flips the gate (`TestProjectCardSubmit`,
`apps/api/internal/api/projectcards_test.go`) — that is the transport this
slice ships and tests. Skipping a proposal records the skip (过程即数据) and
moves on, independent of the mint bridge. Reload preserves the persisted fill
+ (still-active) card. The old chat workspace + task-scoped cards are
untouched.

## Deferred / carry-forward → follow-ons, cleanup, 5d

- **The CRAAP fill→mint bridge:** CRAAP is annotation-based; the schema field
  renderer produces `field_values`, not `anchors`, so a real form-only submit
  never satisfies `EvaluateCompletion` and the live mint never fires. The
  live mint lights up with the **Slice-6 material-annotation surface**
  (highlight-a-passage + tag-a-dimension), which is what actually produces
  `anchors`. Until then, the transport (persist → `CompleteCard` gate →
  refeed) is correct and tested against a hand-built satisfying anchor set,
  but the running product's form path stays non-satisfying by design.
- **Follow-on cards:** richer `SurfaceCardCandidates` (concession/sift/steelman trigger logic); mid-fill `ObserveCandidates` nudges.
- **Cleanup:** Slice-3 debt (task_id NULLABLE + project-scope `GetCardInstance`); the envelope's vestigial `task_id`; live `gate` passed/total (still 0,0 from 5c) — now exercised by the refeed, so wire real counts; unique index on `chat_thread.seeded_project_id`; onboarding live producer.
- **5d:** retire `RunTurn`/`turn.go`/task-scoped card endpoints/old `workspace/`; the routing flip; gate `StudioContainer.defaultEnsureSession`.
