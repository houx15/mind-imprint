# N3f · Finish the Walk (S3 → S6) — Design

**Status:** approved 2026-07-22. Follows N3d (`9d9519d`), which made S0 → S3
walkable. Tracker: `docs/2026-07-20-student-platform-remaining-work.md`.

---

## 1. The finding

N3d fixed S0–S2 and, in its own whole-branch review, discovered that its spec's
claim 「S3–S6 already have producers」 had never been checked and was false. This
slice is the verification of the rest of the chain, done properly: every gate
item in `packages/contracts/skills/writing-project.json` was traced to a
producer in code.

Six items across three stations are unsatisfiable for a real student:

| Station | Item | Tier | Status |
|---|---|---|---|
| S3 `evaluate_sources` | `source_risk_notes` | student_written | she writes it; **nothing attests it** |
| S3 | `source_quality_spot_check` | human | **no producer anywhere** |
| S4 `build_argument` | `warrants` | student_written | she writes it; **nothing attests it** |
| S4 | `steelman` | student_written | she writes it; **nothing attests it** |
| S4 | `warrant_quality_spot_check` | human | **no producer anywhere** |
| S6 `reflect_archive` | `declaration_signed` | human | **no producer anywhere** |

S5 `draft_polish` is the only fully-wired station of the four: `word_budget_ok`
(commitSnapshot), `citations_matched` (attestGate), `whole_draft_review`
(orderReview).

The three `human` items are the design problem; the three `student_written`
items are pure attestation, because the writing already exists.

**Same lesson as N3d, one level up.** N3d's acceptance test passed while the
product was broken, because it POSTed an endpoint no UI state could generate.
This slice's acceptance test must drive only what the web client actually
calls, on a project created through the funnel — never the seeded demo
project, whose gates migration `0018` hand-writes.

---

## 2. Scope

**In:** the six items above, plus the L1 block-id collision bug (§6) carried
from N3c at raised priority.

**Out:** teacher-facing adjudication (roadmap Slice 13, deferred on design);
N3e (search-plan card, R-9 framework reveal); N2e (export forks); the L2
`materials[0]` assumption named in §6.

**No migration.** Every seam this slice needs already exists: `intervention.type`
is free text, `intervention.anchor` is jsonb, `graph_node.type` is free text.

---

## 3. What a `human` gate item means

`docs/2026-07-11-agent-spec.md:282` says positive adjudication needs "a passed
challenge or a human." The codebase has already made one operative choice:
`whole_draft_review` is a `human` item, and its producer is 整稿体检 — the
student orders an examiner-voiced AI review and the gate flips when items
actually persist (`apps/api/internal/api/writing.go:374–394`).

**This slice adopts that reading and applies it consistently.** A `human` item
is satisfied by an adjudicating voice the student summons, not by a teacher and
not by a checkbox. When a teacher surface exists, these become the natural
places to route a real teacher — the gate item names do not change.

Rejected alternatives:

- **Teacher sign-off.** Most faithful to the spec's wording, but no
  teacher-facing surface exists, so the walk stays blocked on a much larger
  unbuilt slice.
- **Student self-attestation.** Cheapest, but it multiplies the labelled
  checkboxes N3d worked to avoid, and reduces "human adjudication" to "she
  ticked a box."
- **Deleting the `human` tier at S3/S4.** Smallest diff, but it removes
  pedagogy the agent spec names explicitly (`warrant_quality_spot_check`
  appears in the spec's own skill example).

---

## 4. The three attestations (S3/S4 `student_written`)

Attestation-by-endpoint, exactly N3d's pattern: the endpoint that persists the
writing records the gate item. No new UI, no new control, no checkbox.

| Item | Attested when | Read from |
|---|---|---|
| `source_risk_notes` | every `kind:"article"` material has a non-blank `source_quality.risk_note` | the CRAAP mint at `agent/card_effects.go:186`; loop mirrors `every_source_evaluated` (`agent/gate.go:54`) |
| `warrants` | the toulmin `warrant` slot node has non-blank text | the slot→text map in `studio/projection.go:697–703` |
| `steelman` | the toulmin `counter` slot node (反方 · 钢人) has non-blank text, **or** a `steelman` card instance is completed | same, plus `card_instances` |

The `steelman` item accepts two producers because the standalone 钢人卡 has **no
`graph_effects`** and mints no node. Without the second producer, a student who
did that card and not the Toulmin slot would write a steelman and get no credit
for it. A *skipped* card never counts.

Attestation runs in `submitProjectCard` (`api/projectcards.go`), immediately
before its existing `advanceGates` call (`projectcards.go:334`), so a submit
that completes the last missing risk_note both attests and advances in one
request. All three items are read from graph state, so one attestation pass
covering all three runs at that single site — no per-card branching.

**No length floor**, deliberately — unlike N3d's ≥15-rune term definitions. The
CRAAP card's own `field_written_by(risk_note, student)` completion predicate
(`packages/contracts/cards/craap.json:31`) is already the floor. A second,
different threshold on the same text would let a card complete while its gate
item stayed missing, with nothing on screen explaining the discrepancy.

**Un-attestation.** Each check runs on every relevant submit and both sets and
clears, like `attestGate`'s set/delete pattern. Deleting a material or blanking
a slot must be able to re-open the gate; a gate that can only ever close is a
gate that lies after a deletion.

**RL-5 holds.** These check that she *did* the work, never how well. Quality
judgment belongs to the assessor and to her own self-score.

---

## 5. The two spot-checks (S3/S4 `human`)

Two student-triggered checks, named parallel to 整稿体检: **信源体检** at S3
(素材 view) and **论证体检** at S4 (结构 view).

### 5.1 Runtime

One flagship call each, through `ProposeReview`'s enforcement stack verbatim:
banned-phrasing + output check on every field, a single violation rejects the
**whole** order (nothing returned, nothing persisted), and usage is recorded
even when the output is rejected — a rejected call still cost money.

Endpoint shape mirrors `orderReview` (`api/writing.go:267`): SSE via
`gateway.NewSSEWriter`, `studioEmitter` + heartbeat, entitlement checked
**before** the stream opens and only when a model call will actually happen.

One route, contract-parameterized — `contractId` matches the path-value name
the existing gate route already uses (`api.go:69`):

```
POST /api/v1/projects/{id}/contracts/{contractId}/spot-check
```

Valid only for `evaluate_sources` and `build_argument`; any other contract id
is a 404, so the route cannot be used to mint interventions against stations
that have no check defined.

Metered via `RecordLLMCall` with `Purpose: "spot_check"` — a distinct purpose
from `order_review` so per-station cost stays legible in `llm_usage`.

### 5.2 What each check reads

- **S3 (信源体检).** Per `kind:"article"` material: title, tier, takeaway,
  `risk_note`, `lateral_read`, and the CRAAP anchor answers.
- **S4 (论证体检).** The five Toulmin slot texts (claim / warrant / evidence /
  counter / concession) plus which locked sources each cites.

### 5.3 Item shape

```go
type SpotCheckItem struct {
    TargetID   string `json:"target_id"`   // material id (S3) | slot id (S4)
    TargetName string `json:"target_name"` // resolved server-side, never model-authored
    Evidence   string `json:"evidence"`
    Missing    string `json:"missing"`
    Fix        string `json:"fix"`         // advice only, never a rewritten sentence (RL-1)
}
```

`TargetName` is resolved server-side from the id, the same discipline
`ReviewItem` uses for `CriterionName` — the model never invents labels.

**No `points`.** That field is 0457 readiness-gauge data (Slice 9), specific to
表D/E/F/H and to the whole-draft review. It has no meaning here.

**No band, and no verdict.** `MaterialDTO` deliberately carries no credibility
field because 「a 可信/存疑 judgment has no honest producer and would have to be
fabricated」 (`studio/dto.go:200–205`). A band on a spot-check item would
reintroduce exactly that judgment through a side door. The check reports what
evidence it sees and what is missing; it does not rate her sources.

### 5.4 Idempotency — the fingerprint

整稿体检 is one-snapshot-one-review, anchored on the snapshot id the student
explicitly committed. S3/S4 have no commit action, so the fingerprint is
derived from what the check actually reads:

- **S3:** the ordered list of `(material_id, risk_note)` pairs for article
  materials.
- **S4:** the ordered list of `(slot_id, text)` pairs for the five slots.

SHA-256 over a canonical serialization, hex-encoded, stored in the
intervention's jsonb `anchor` as `{"station": "...", "fingerprint": "..."}` —
the same additive-anchor-field seam Slice 8b used for `voice`, so no migration.

Behaviour:

- Ordering with an **unchanged** fingerprint re-streams the stored items and
  makes **zero model calls** — `orderReview`'s `existing == 0` guard,
  generalized from snapshot id to fingerprint.
- Improving a risk_note or rewriting a warrant changes the fingerprint, and the
  check becomes orderable again. This satisfies the spec's 「loops are normal
  operation; gates govern what unlocks, never what may be revisited」.

### 5.5 Gate flip

The gate item is marked solid **only if at least one item actually persisted**,
then `advanceGates` runs. This is `writing.go:374–394`'s guard and it is kept
for its stated reason: a solid gate with zero visible items makes the gate and
the UI disagree, and a phantom "already ordered" record would make every later
attempt a silent no-op.

### 5.6 Where the items render

Extract `WorkOrderItem` (`apps/web/src/studio/views/WritingView.tsx:162`) and
its three-key disposition composer into one shared component, and render it at
all three call sites: 写作 (existing), 素材 (S3), 结构 (S4).

The extracted component takes a normalized row shape that both feed: the
existing `WritingReviewItem` maps `criterion`→label and keeps its `band` chip;
a `SpotCheckItem` maps `target_name`→label and renders no chip. The band chip
must be optional rather than blank-stringed, so a spot-check row cannot render
an empty chip where the review shows a band.

This derives from the binding design rather than inventing a layout — the panel
is 整稿体检's own designed work order (`docs/design/思维印记_工作区.dc.html:1120–1149`)
placed at two more stations. The three-key disposition, its ≥15-character
reason rule, and the RL-1 framing all come along unchanged.

### 5.7 Contract and projection

`packages/contracts/src/studioState.ts` gains `SpotCheckItem` and
`SpotCheckFx { items, orderable }`, added as one **flat, required top-level**
`spotChecks` member keyed by station (`evaluateSources` / `buildArgument`),
mirroring how N3d added `framing`/`perspectives`.

It cannot hang off the material or structure projections: `StudioProjection`'s
`materials` and `structure` are **arrays** (`studioState.ts:275, 277`), so
nothing can be a member of them.

`orderable` is computed server-side, but it is not simply "the current
fingerprint differs from the most recent stored one" — recency alone
mishandles an edit-then-revert sequence: order at state A (fingerprint
`fpA`) → edit and order again (fingerprint `fpB`, later) → revert the text
back to exactly state A. The current fingerprint is `fpA` again, and a batch
already exists for it, but "most recent" would still select `fpB` — showing
now-stale items and reporting `orderable = true` even though pressing the
button would just replay `fpA`'s batch for free.

The projection therefore **selects the batch whose fingerprint equals the
current fingerprint when one exists** (`orderable = false` — a real match,
nothing to (re)order), and only **falls back to the most recent batch**
when no stored batch matches the current fingerprint (`orderable = true` —
deliberately stale-but-visible, so her work order does not vanish the
moment she starts typing). A station with no targets at all is never
orderable, even though the "no targets" fingerprint trivially differs from
any stored one — there is nothing to check.

The button's enabled state is therefore never a client guess about whether
a call would cost money.

The trigger control mirrors 整稿体检's button placement (`dc.html:1103`).
Examiner-voice switching is **not** offered at S3/S4 — voices are a
whole-draft-review affordance (Slice 8b) and there is no design for them here.

---

## 6. The L1 block-id collision

`materialize.Segment` numbers blocks `b0, b1, …` **per material**
(`materialize/segment.go:28`, whose own comment says "stable within a
material"). But:

- `blockLookup` (`agent/anchors.go:84`) is a flat `map[blockID]` across all
  materials — last material wins.
- `BuildMaterialContext` (`agent/prompt.go:74`) renders every material's blocks
  with colliding `[b0]` labels, so the model cannot disambiguate either.
- `projectMaterials` (`api/studioturn.go:410`) passes **all** of a project's
  materials, so the collision is live, not theoretical.

Only guidance level L1 resolves block ids (`anchors.go:117`); L2 never does and
L3 makes no model call. So N3c's guidance-fade work fixed this shape for L2/L3
and left the beginner path as the only broken one.

**Fix:** label blocks material-qualified in the prompt (`m0:b0`) and key the
lookup by the qualified id, so the model can disambiguate and the resolver
cannot silently pick the wrong article. L2/L3 paths untouched.

**Adjacent, deliberately not fixed:** L2 attributes every anchor to
`materials[0].ID` (`anchors.go:137–140`). It is masked today because at L2 the
student locates the span herself, which corrects the material. Same underlying
assumption, different failure mode; it needs its own decision about what an
unlocated anchor's material should be, and that is not this slice.

---

## 7. S6 · AI 使用申报单

The declaration is a projection over data that already exists — **no model
call**. `agent.BuildAssessmentInput` already digests every input it needs for
the assessor.

Rendered per `dc.html:1211–1223` (「自动生成 · 待你签名」), four counters:

| Counter | Source |
|---|---|
| 提问 / 追问 | `prompt_sent` events (`api/assessment.go:169` already pairs them into rounds) |
| 三键处置（接受/改/拒） | `disposition` rows for the project |
| 工具卡调用（自发/提示后） | `projectEquipment`'s existing spont split (`studio/projection.go:260`) |
| AI 代写正文 | `0` — see the caveat below |

**Signing** goes through its own endpoint,
`POST /api/v1/projects/{id}/declaration/sign`, **not** `attestGate`.
`attestGate` is deliberately restricted to a contract's `student_written` names
so a caller cannot forge a machine or human item (`api/writing.go:228–238`);
that restriction stays. The new endpoint recomputes the counters server-side,
persists them into a `declaration` graph node, and then flips
`declaration_signed`. The signature therefore records *what was signed* rather
than pointing at a live view that keeps moving — 过程即数据.

**Honesty caveat, recorded deliberately.** The `AI 代写正文 = 0` figure is a
claim by construction, not a measurement. It holds because RL-1 makes the
review's `fix` field advice that is never written into the draft, and because
no code path places model text into the edit buffer. This spec names that
invariant so that if it is ever broken, the declaration does not go on quietly
printing 0. A future slice that adds any AI-to-draft path **must** either
measure this counter or delete it.

---

## 8. Testing

### 8.1 The fresh-project walk

An end-to-end acceptance test that creates a project through `POST /projects`
and drives S0 → S6, asserting each station turns `done` in order and that the
last one can.

Two rules, both learned from N3d's failure:

1. **Never the seeded demo project.** Migration `0018_seed_demo_project.sql`
   hand-writes `confirmed_solid` gate rows; a test against it proves nothing
   about live code paths.
2. **Only endpoints the web client actually calls.** N3d's acceptance test
   POSTed `/materials/{mid}/open` directly — a request no S2 UI state could
   generate — and so passed over a station the student could not clear. Every
   step of this test must correspond to a control that exists on screen at that
   station.

### 8.2 The producerless-gate guard

A test that enumerates every gate item name in `writing-project.json` and fails
if any lacks a registered producer.

This defect class has now shipped three times: S1/S2 (found in N3d), S3/S4 and
S6 (found here). Each time it was invisible because the seeded project made the
rail look alive. A guard test is the only thing that stops a fourth occurrence,
and it is the single highest-value artifact in this slice.

Machine items are satisfied by their predicate in `agent/gate.go` — the guard
asserts each `kind` is handled by `evalMachineItem` rather than falling through
to its `default` branch. `student_written` and `human` items are checked
against an explicit producer registry: one Go map from item name to the
function that records it, living beside the attestation code.

**What this guard is and is not.** It is a tripwire, not a proof: someone can
still satisfy it by adding a map entry pointing at a function that does not
really record the item. What it makes impossible is the actual failure that
shipped three times — adding a gate item to the skill JSON and never thinking
about a producer at all. The map entry forces the thought, and names the
producer where the next reader will look for it.

---

## 9. Iron rules and their bearing here

- **铁律 1 (AI 克制).** The spot-checks report what is missing; `fix` is
  direction, never a paste-ready sentence. Same RL-1 discipline as 整稿体检.
- **铁律 2 (不操纵).** No score, no celebration, no streak on either check. An
  offer is never a wall: neither check blocks her from working, only from
  marking the station finished.
- **铁律 4 (过程即数据).** Ordering a check, its items, her dispositions, and
  the signed declaration all land in the event stream and the process tree.
- **RL-5.** The gates check that she did the work. Every quality judgment in
  this slice is advisory and dispositionable.

---

## 10. Known limits

- `source_risk_notes` is **false when the project has no article materials**,
  unlike the `every_source_evaluated` machine predicate beside it, which passes
  vacuously over an empty list by design (`agent/gate.go:54`; the codebase
  already treats vacuous passing as a hazard — see `ItemResult.Attempted`).
  An empty dossier has not done S3's work, and marking the item solid there
  would let a student clear 信源评估 having evaluated nothing.
- The S3 check reads risk_notes and CRAAP answers, not the source texts
  themselves — it can tell her a risk_note is thin, not that she misread the
  article.
- The fingerprint covers what the check reads and nothing else. Adding a
  material without writing its risk_note does not change the S3 fingerprint,
  so the check is not re-orderable until she writes one. That is intended: the
  check has nothing new to say until she has written something new.
- `AI 代写正文` is unmeasured (§7).
- The walk this slice completes is the 0457 board only. Other boards remain
  N4's problem.
- `orderable` is false whenever a station has no targets at all, which is the
  panel's very first state at both S3 and S4 (S3 until she adds an article;
  S4 until at least one Toulmin slot has non-blank text). The panel's copy in
  that state must say what has to exist before a check is possible — it must
  never instruct her to press the button, which is disabled precisely then.
- The S3 target list names only *adding* a source as the precondition, not
  *evaluating* it — `sourceSpotCheckTargets`'s own comment records that an
  unevaluated article is still a target, because 体检 covers what she has done
  **and** has not done. The copy must not teach the wrong model (that the
  button unlocks only once a source is evaluated).
- L2 still attributes every anchor to `materials[0].ID`
  (`agent/anchors.go:164,213`) — the same assumption behind the L1 block-id
  collision this slice fixed (§6), masked at L2 because the student locates
  the span herself, which corrects the material. Explicitly deferred, same as
  §6 records.
