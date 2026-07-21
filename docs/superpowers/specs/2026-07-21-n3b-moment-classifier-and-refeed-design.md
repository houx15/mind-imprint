# N3b · The semantic moment classifier + the live refeed seam — design

> Slice N3b of the student-platform finishing decomposition (tracker:
> `docs/2026-07-20-student-platform-remaining-work.md`). Follows **N3a**
> (`27492fd`), which took the C1 interaction-primitive library to 6/6 but left
> two of its three cards deliberately unwired.

**Goal.** Close the two seams N3a left open: the **moment that summons a card**
(a real classifier subagent, not a hardcoded card-id list) and the **seam that
feeds a finished card back to the coach** (摘要回灌, live for the first time).

---

## 1 · Why these two ship together

N3a built `fact-opinion-value` (sort) and `certainty-spectrum` (scale) through
every layer — Zod state, Go params/predicates, primitive module, Studio host,
CoachRail fork — and then wired neither. That was deliberate: their honest
trigger is a **semantic judgment about what the student just wrote**, and
`SurfaceCardCandidates` is a pure structural predicate over the graph. A
structural proxy would fire them at the wrong moment.

The N3a whole-branch review then found the second half of the problem. Its
finding **I1**: the spec's claim that "refeed works for free" was false.
`agent.anchorSteps` — whose label fallback N3a fixed — is reachable only from
`agent.BuildLlmMessages`, and `BuildLlmMessages` **has no production caller**.
The live path is `RunAgentStep(Trigger{Kind:"card_refeed"})`, which ignores the
trigger entirely and re-runs the same graph classifier. So a completed card
reaches the coach **only through its `graph_effects`**.

Those two facts compose into one dependency:

- A summon without a refeed is a **write-only sink** — the student does real
  structured thinking and the coach never sees it.
- A refeed with nothing summoning the card **never fires**.

`steelman` makes the dependency concrete: it is a legacy form card with no
`primitive` and no `graph_effects`. It stops being a write-only sink **only
because** Seam B lands. Hence: one slice, both seams.

---

## 2 · Seam A — `ClassifyMoment`, the classifier subagent made real

New file: `apps/api/internal/agent/moment.go`.

### 2.1 The closed set

The classifier picks from a **fixed enum**, never a free-form card id. That
closed set is the structural guarantee that the model cannot invent UI — the
same principle that makes C1 primitives a hand-built library rather than
generated markup.

| moment | meaning | card | primitive | criterion |
|---|---|---|---|---|
| `fact_opinion` | 学生把一个需要论证的观点当成可查证的事实陈述 | `fact-opinion-value` | sort | `D3` |
| `overclaim` | 学生表达的确定度高于他给出的证据能支撑的程度 | `certainty-spectrum` | scale | `D3` |
| `one_sided` | 学生只从一侧论证，未处理最强的反面 | `steelman` | (form) | `D4` |

One mapping table (`momentCard`) is the single source for card id, the
classifier's own description of the moment, the coach-facing flag sentence, and
the candidate's `Reason`/`Criterion`. Nothing in this slice may hardcode a card
id a second time.

Criteria come from the live DualAxis vocabulary (`internal/rubric/dualaxis.json`):
`D3` 证据与信源意识, `D4` 论证结构意识. (Note: `CandidateMoves` tags its
unsupported-claim candidate `D5` 反馈理解与修改理由, which reads like a
pre-existing mislabel — out of scope here, recorded as a minor.)

### 2.2 The signature

```go
func ClassifyMoment(
    ctx context.Context,
    prov gateway.Provider,
    r gateway.Resolved,
    text string,
    eligible []Moment,
) (Moment, gateway.ChatUsage, error)
```

One model call. The system prompt names **only the eligible moments** and
instructs the model to answer with exactly one id or `none`. The reply is
matched by **exact string equality against the eligible set** — anything else,
including a chatty reply, collapses to `MomentNone`. The classifier never sees
the card catalog and never writes a word the student reads; its entire output
is one identifier.

### 2.3 It runs only when it could act

A four-part **structural pre-gate**, evaluated before the call:

1. no `card_instance` is `proposed` or `active` (the existing in-flight guard);
2. at least one moment is still eligible — a moment is ineligible once its
   target card has a `card_instance` in **any** status;
3. the student's text is at least 12 runes (never classify 「嗯」);
4. **no structural `surface_card` candidate already won this turn.**

(4) is both the cost rule and the product statement. If CRAAP / SIFT /
perspective-matrix / toulmin already produced a candidate, decide-one would act
on that anyway, so a classifier answer would be discarded — we do not pay for
it. And the resulting behaviour is the right one: **structure first, semantics
as the fallback that notices what structure cannot see.**

When the classifier does name a moment, its candidate is inserted **after
structural `surface_card` candidates and ahead of every `post_intervention`** —
the same ranking rationale the loop already documents: a card offer is a
one-time actionable offer, a coaching nudge can wait one more step.

`deps.SkipSurfaceCards` suppresses the semantic candidate exactly as it
suppresses structural ones. A caller that has said "no cards this pass" gets no
cards.

### 2.4 Suppression: an offer is never a wall

Rule (2) above is the whole suppression policy, and it matches
`perspective-matrix`'s: **any** `card_instance` for the target card, in **any**
status including `skipped`, makes that moment permanently ineligible for that
project/thread. Once she has said no, we do not ask again (铁律 2 · 不操纵).

### 2.5 Metering and failure

Metered as `Purpose: "classify"` (`Surface: "studio"` / `"chat"`) — including
when the answer is `none` and when the reply fails to parse. It cost real money
either way; AGENTS.md's 记录档位 + token + 成本 is unconditional. A
`RecordLLMCall` failure never fails the turn (existing policy).

Any classifier error — transport, timeout, unparseable reply — is **silence**.
It never propagates to the student's turn.

### 2.6 Honest note on the "cheap tier"

The tracker calls this "the cheap-model classifier", and agent-spec §144 says a
cheap model runs trigger predicates. **There is no cheap tier in the code
today.** `keyresolver.go` resolves chaperone *and* flagship to
`deepseek-v4-pro`; they differ only in the `Tier` string. The classifier rides
the **chaperone (downgradeable) resolver**, so it becomes cheaper for free the
day a real cheap model is wired — and nothing in this slice claims a cheap tier
exists. (N3a shipped a false "works for free" claim in its spec; this note is
that lesson applied up front.)

### 2.7 Surface wiring

**Studio** — inside `RunAgentStep`, on `Trigger{Kind:"student_turn"}` only, after
the structural candidates are collected and only when the pre-gate holds.

The student's text arrives on the trigger (`Trigger.StudentText`), not from a
history load. `postProjectTurn` already holds the message it just persisted, and
`RunAgentStep` loads chat history only *after* the surface-card branch — reading
the text from history would mean either reordering that load onto every turn or
duplicating it. The trigger already exists to carry "what invoked this step";
carrying the text with it is the smaller change and keeps the pre-gate a pure
function of its inputs.

**Chat** — inside `RunChatStep`. The existing structural `ChatCardCandidate`
(link → CRAAP) stays and wins when it fires; the classifier runs only when it
does not. `BuildChatContext` already takes a `flag string` — the classifier's
flag sentence goes there, replacing nothing. On a named moment the thread mints
a `card_instance` for that card via the existing `CreateThreadCardInstance` and
emits the existing `card_surfaced` event.

A semantic chat offer has **no material**: `CardOffer.MaterialID` is
`uuid.Nil`. This is safe and verified — the chat card path is thin (no
`CompleteCard`, no material-addressed graph effects), and neither
`StudioCardSheet` nor `ChatSurface` reads `materialId`.

---

## 3 · Seam B — the live refeed, shaped per surface

### 3.1 Studio: a dedicated coach turn on submit

`Trigger` gains two fields in total across both seams:

```go
type Trigger struct {
    Kind           string
    StudentText    string // Seam A: set for Kind == "student_turn"
    CardInstanceID string // Seam B: set for Kind == "card_refeed"
}
```

Both are additive and optional — every existing caller and test that constructs
`Trigger{Kind: ...}` compiles and behaves unchanged.

`projectcards.go`'s submit handler already holds both `cid` and the resolved
`cardStatus` at the refeed call site — no new plumbing.

`RunAgentStep`, when `trigger.Kind == "card_refeed"` and `CardInstanceID` names
an instance whose status is `completed`, emits a **first-priority** candidate:

```go
Candidate{
    Verb:       "post_intervention",
    AnchorKind: "card_instance",
    AnchorID:   cid,
    Criterion:  "D6",   // 元认知与反思
    Level:      "I2",
    Reason:     "学生刚完成了一张工具卡",
}
```

`BuildCoachContext` gains one branch: when `c.AnchorKind == "card_instance"`, it
renders `SerializeCardForRefeed(spec, inst)`'s payload under a
「# 学生刚完成的工具卡」heading **in place of** the graph-node block (there is no
node to render). Everything else — history, reason, criterion, the posture
system prompt, the full enforcement stack — is unchanged. The coach asks **one**
question about what she just wrote. That is 摘要回灌 from AGENTS.md's acceptance
mainline, live for the first time.

`anchorTopic` returns `""` for a card_instance anchor (no matching node), so
`OutputCheck` runs with an empty topic — the same behaviour material-anchored
candidates already have. `enforcement.ValidateOutput` requires a non-empty
`Anchor.ID`, `Criterion`, and `Body`, all of which this candidate supplies;
anchor **kind** is a free-form string and needs no enforcement change.

**Loading the instance costs no schema work.** `GetCardInstance` is already
`SELECT * FROM card_instances WHERE id = $1` and is already on the `AgentStore`
interface. `CardInstanceRow` gains one field, `FieldValues []byte`, populated
from the row that adapter already has. **No sqlc regeneration, no new query, no
new interface method.**

`GraphView.CardInstanceView` is deliberately **not** widened. Every pure
classifier test constructs that shape as a fixture, and N1/N2 each cost a
fixture-cascade wave from widening a shared type. The refeed path loads what it
needs directly.

### 3.2 Skipped → silence

The refeed candidate fires on `completed` **only**. A skipped card is recorded
as data (铁律 4 · 过程即数据) and produces no coach turn. Answering a decline
with a question is the nagging posture 铁律 2 forbids.

### 3.3 Chat: the summary joins the next turn, not a new one

Chat's card submit is thin by policy — no `CompleteCard`, no graph effects, and
**no post-submit turn at all**. Manufacturing one so the coach can speak
unprompted into a free chat is the wrong posture for that surface.

So chat refeeds passively: `BuildChatContext` gains a block for the thread's
completed cards, serialized by the **same** `SerializeCardForRefeed`, folded in
on the student's **next** message. Zero additional LLM calls; one shared
serializer; each surface keeps its own policy.

`ScopedCard` gains `FieldValues []byte` and `Anchors []byte` — additive, zero
values are valid, and the Course adapter fills them harmlessly.

---

## 4 · Two carry-forwards, same slice

**Toulmin skip-suppression.** Today `toulmin` has no suppression at all: skip
it → no `claim` node is ever minted → `anyEvaluated && !hasClaim` stays true →
it re-offers forever. `perspective-matrix` deliberately did not copy that shape;
this applies the same any-status rule to `toulmin`. An offer is never a wall.

**`needsMaterial` completeness guard.** `card_lifecycle.go`'s
`materialConsumingEffects` closed set (`promote`, `cross_check`) has no
compile-time link to `GraphEffects`' switch — a new material-addressed effect
kind must be added to both, and nothing catches a miss. A test that enumerates
every effect kind the effects switch handles and asserts each is classified
one way or the other turns a silent runtime failure into a red test.

---

## 5 · What does not change

No migration · no sqlc regeneration · no new `Anchor` field · no card JSON
change · no `packages/contracts` change · no new model tier · no web change.

The web layer needs nothing: N3a's `StudioSortCard`/`StudioScaleCard` already
render their cards through the exhaustive `CoachRail` fork, `steelman` rides the
legacy `StudioCardSheet` fallback, and `ChatSurface` already renders card
offers.

---

## 6 · Risks accepted

**Classifier precision is a prompt-quality question only live use can settle.**
Bounded by construction: the closed set means a wrong answer can only ever name
one of three real cards, and any-status suppression caps the damage at **one
badly-timed offer per card per project**, declined once and never seen again.

**Cost.** The pre-gate means the classifier call happens only on a student turn
where nothing structural fired and an un-offered card is still eligible — a
strictly bounded subset of turns, and monotonically decreasing over a project's
life as the three cards get offered.

---

## 7 · Delivered vs carried forward

Delivered: both seams live end-to-end on both surfaces, plus the two
carry-forwards above.

Carried forward (unchanged by this slice): **N3c** (student free span-creation
L2/L3) and **N3d** (R-9 reveal, search-plan card, S2 perspective view).
`BuildLlmMessages` remains without a production caller — this slice routes
around it rather than reviving it; retiring it is an N6 tech-debt item.
