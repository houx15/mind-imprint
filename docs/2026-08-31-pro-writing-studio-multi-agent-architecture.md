# Writing Studio — Multi-Agent Architecture (pro edition)

> **Scope.** The pro edition's 写作项目 (writing project) surface: one continuous
> 印记 orchestrator, the closed tool vocabulary it acts through, the context
> projection it reasons over, the ~30 context-isolated sub-agents behind it, the
> 34-card 工具卡 (thinking tool card) runtime, and the enforcement layer that makes
> "the AI never writes the student's text" a structural property rather than a
> prompt request.
>
> **Behavioral source of truth.** `docs/2026-08-09-all-statuses.md` — the single
> truth for every writing-project status, its page, its cards, the AI's role and
> its transitions. When this document and that one disagree, that one wins.
> Architecture north-star: `docs/superpowers/specs/2026-06-24-backend-platform-architecture-design.md`.

---

## 0. The one-sentence claim

**The tool card is a tool in a tool-use loop whose executor is a human.**

A normal agent loop is: model decides → model calls a tool → tool returns a result
→ model continues. Here the loop is: 印记 decides → it calls `summon_card` →
**the student fills the card in** → the card's standard envelope is serialized back
to the model as a tool result (`refeed`) → 印记 continues. The friction of the
student having to think is not something to remove; it is the tool call, and it is
the thing being recorded and later assessed.

Everything else in this document exists to make that loop safe, cheap, resumable
and honest.

---

## 1. Layer map

```
       ┌─────────────────────────────────────────────────────────────┐
       │ 印记 · one continuous orchestrator (server-side, per project)│
       │   input : spine projection + folded 会话记忆 + active window  │
       │   output: ONE JSON envelope {narrate, tools[]}                │
       └───────────────┬─────────────────────────────────────────────┘
                       │ closed tool vocabulary (10 verbs)
   ┌───────────────────┼───────────────────────────────────────────┐
   │                   │                                           │
   ▼                   ▼                                           ▼
Workspace          Card Runtime                          Context-isolated
configuration      (34 JSON specs,                       sub-agents (~30)
(status / room /   9 field primitives,                   each: pure input →
reference rail /   standard envelope)                    one call → struct
plan / notes)             │                                        │
                          │ student fills                          │
                          ▼                                        ▼
                    refeed as tool_result ──────────► process tree + llm_call
                                                       (graph_node/edge, event,
                                                        intervention, disposition)
```

---

## 2. The orchestrator

`apps/api/internal/agent/orchestrator.go`, driven by `api/coach.go` +
`api/studioturn.go` over SSE.

### 2.1 One envelope per turn

```json
{ "narrate": "one paragraph for the student, ONE question only",
  "tools":   [ { "name": "...", "args": { ... } } ] }
```

`narrate` is always written in Chinese, even when the student writes English and
the deliverable will be English — the chaperone stays in the student's thinking
language while the product stays in the assignment's language (`写作语言` rides the
projection so the model remembers the difference).

### 2.2 The closed tool vocabulary

| Tool | What it configures |
|---|---|
| `set_status` | Move the project forward or back across the 7 stages |
| `open_tool` | Open the right room: `chat` / `forming` / `plan` / `reading` / `writing` / `reflection` |
| `curate_reference` | Put the materials / snippets / annotations the student needs on the left rail |
| `propose_note` | Extract one 提案要点 candidate from what the student said, strictly typed to `objective` / `reason` / `activities` / `resources` / `counterpoints` |
| `summon_card` | Hand a thinking tool card back to the student at the right moment |
| `request_review` | Order a whole-draft 整稿体检 |
| `generate_plan` | Emit the project plan once the four required proposal dimensions exist |
| `propose_question` | Propose one research question worth chasing |
| `update_plan` | `complete` / `start` / `reopen` / `add` / `edit` / `remove` a plan item |
| `note_resource_need` | Log a keyword worth searching into 还需要探索的 |

Anything outside this set is dropped at parse time. Two of these deserve emphasis
as design decisions, not conveniences:

- **`propose_note` is typed, and lying is caught.** The prompt classifies content
  into the five dimensions with worked disambiguations ("first read literature,
  then a survey, then write, about three weeks" is `activities`, never
  `objective`). It also states the rule that if `narrate` claims a point was
  recorded, the same turn **must** actually emit the `propose_note` — say-without-doing
  is a named failure mode, not an accident.
- **`curate_reference` ids must be real.** The model may only cite `[id]`s that
  appeared in the projection. Two independent filters enforce it: item-level
  validation in `ParseOrchestratorOutput` (drop the bad item, keep the good ones —
  never the whole call), and `filterKnownReferences` server-side, which resolves
  every id against the project's real materials/snippets and fails closed on a
  query error.

### 2.3 Parse hardening

`ParseOrchestratorOutput` is written for the reality that a model sometimes wraps
its envelope in prose:

1. Strip code fences.
2. If that does not unmarshal, salvage the **first balanced `{...}` object** with a
   scanner that tracks string literals and escapes, so a `{` inside a `narrate`
   string never miscounts depth.
3. Drop unknown tools; validate each known tool's args by type
   (`validToolArgs`); filter `curate_reference` per item.
4. Only when no balanced object survives is it a parse failure — the caller
   retries once, then falls back.

The point of the salvage path: a discarded turn dumps a generic fallback line that
ignores what the student just said **and then poisons the next turn's history**.
Recovering a good envelope from a chatty wrapper is worth the ~30 lines.

### 2.4 The id-leak sanitizer

`SanitizeNarrate` strips any UUID (with surrounding brackets and one leading space)
from `narrate` before it reaches the student. The prompt already forbids ids in
prose — 印记 must say "the tentative claim on your left", not `[3f2a…]`. The regex
is defence-in-depth for the times the prompt loses.

---

## 3. The status router — progressive disclosure for the model

The mega-prompt in §2 is the full posture. In production the studio turn runs a
**collapsed** version: seven persisted `StudioStage` values project onto **five
canonical `FlowStatus`** values, and each status owns a `StatusDef`:

| FlowStatus | Goal | Tools | Cards | Surface | Doc |
|---|---|---|---|---|---|
| `topic` | Narrow a vague interest into one researchable question | `propose_question` | — (提问卡 is a student-opened chatbox button) | chat | — |
| `framework` | Get the five 立题 dimensions clear | `propose_note`, `summon_card`, `update_plan`, `note_resource_need` | cross-cutting only | forming | — |
| `proposal` | Student writes the proposal; 印记 checks argument + structure | `open_reading`, `curate_reference`, `request_review`, `finish_part`, `summon_card`, `update_plan`, `note_resource_need` | cross-cutting only | writing | proposal |
| `essay` | Student writes the body | same as proposal | `pee`, `toulmin`, `argument-map` + cross-cutting | writing | essay |
| `review` | Student writes the retrospective | none | — | reflection | — |

Three structural notes:

1. **`set_status`, `open_tool` and `generate_plan` are deliberately absent from the
   status tool set.** Stage transitions and plan generation are deterministic
   server-side decisions (`advanceStudioFlow` / `nextStepFor`), never the model's.
   The model is asked to converse well within a status; the system decides when a
   status has been earned. `nextStepFor` never offers a transition the student has
   not earned (framework needs a real plan; proposal/essay need an explicit
   `finish_part`).
2. **Each status prompt is ≤ 6 lines with 2–4 tools.** That is what makes the
   per-turn call affordable on a fast tier — see §9.
3. **Cross-cutting cards ignore status.** `ai-boundary`, `knower-perspective`,
   `metacognition`, `emotional-alignment`, `rabbit-hole`, `ethics-lenses`,
   `ai-decision-tree`, `perspective-matrix`, `concession` answer a *contextual*
   need (the AI may be hallucinating; the student is one-sided, stuck, or showing
   confirmation bias), not a phase, so they are summonable everywhere.

---

## 4. Context engineering: the spine projection

`buildSpineProjection` (`api/projectcoach.go`) assembles the compact, always-on
view the orchestrator sees every turn. It is deliberately generous but bounded, and
every slice is best-effort — a missing proposal on a fresh project degrades that
line to a placeholder rather than failing the turn.

What rides on every turn:

- **会话记忆** — the folded conversation digest (≤600 runes), so nothing the
  compaction backstop folded away is forgotten.
- **主题** — the *full assignment brief* (≤400 runes), not `project.Title`. The
  title is a 60-char truncation, and 印记 once read a mid-word cut as the student's
  unfinished title and asked her to complete it. Named bug, structural fix.
- **写作语言** — the deliverable's target language.
- **开题四问** — the five proposal dimensions with 未填 markers; the optional fifth
  (可能的反例/张力) only surfaces once non-empty so an empty one never reads as
  "still missing" (it does not gate plan generation).
- **计划** — item counts by column **plus** the concrete next not-done task and the
  room it lives in, **plus** an explicit instruction to stop offering to generate a
  plan once one exists. (Without that line the stage code `plan_generation` read to
  the model as "generate now", and it narrated "shall I generate a plan?" over an
  existing 8-item plan.)
- **文献库 / 片段 / 批注 index** — the real `[id]`s `curate_reference` may cite.
- **Surface-conditional nudges** — a different coaching posture per room: 立题
  coverage steering in `forming`, search-keyword help in `find_sources` (give
  directions, never search for her, never hand down a credibility verdict), and a
  long explicit instruction in `reflection` that this is **not** a viva — no
  interrogation, help her find her own words for whichever part she is stuck on.

### 4.1 Compaction — two mechanisms

1. **Fold-on-solidify.** When 立题/计划 solidifies (first proposal save, plan
   generation), the shaping turns on the `forming` / `proposal_review` surfaces are
   folded out of the active window. Idempotent — a second event just catches turns
   added since the first.
2. **Size-threshold backstop.** When the non-folded window still overflows a rune
   budget, the oldest overflow turns are merged into a rolling per-project
   conversation digest by one isolated mid-tier call (`digest.go`), then folded out.
   The digest preserves reusable facts, **the student's own reasoning and
   decisions**, and each source's function and limits. 过程即数据 — folded is not
   lost.

---

## 5. The card runtime

### 5.1 Single source of truth

34 card specs live as JSON in `packages/contracts/cards/`. The frontend imports
them at build time; Go embeds the same files via `go:embed`. There is no second
copy and therefore no `trigger_condition` drift. **Adding a card is adding a JSON
file** — the validation criterion for whether the schema-driven claim is real.

### 5.2 Nine field primitives

`text`, `textarea`, `single_choice`, `multi_choice`, `rating`, `link_check`,
`spectrum`, `criteria_check`, `repeatable_group` (one level, no nesting). A card is
assembled from these; a new interaction primitive is added only when a genuinely
new interaction is needed, never to express a new card.

A card spec carries: `id`, `category`, `name`, `purpose`, `trigger_condition`,
`steps[]` (each with `disclose: always|on_demand`, a `methodology` of why/how/when,
its fields, and optional `sentence_frames` — fill-in-the-blank scaffolds that are
skeletons only, never finished sentences about the student's own thesis),
`rubric_tags`, `mode: annotation|form`, `interaction: form|sub-agent|function`,
`placement: status|reading|reading-toolkit|cross-cutting`, gallery `teaching`
content, and — on the nine reading lenses — a `reading_lens` block
(`task_prompt` / `selection_hint` / `example_focus` / `method_ids`).

### 5.3 The standard envelope

```
card_instance := { id, card_id, task_id, parent_node_id, status,
                   field_values, event_trace[], anchors[], rubric_tags,
                   created_at, completed_at }
status        := proposed | active | completed | skipped
trace kinds   := field_change | step_expand | note_open | skip | submit
                 | span_located | span_not_found
```

This envelope is the common ground of the process tree, the usage counts and the
evaluation. Go validates only the **border** (status enum, ids, `field_values` is an
object, `event_trace` is an array); the deep truth stays in the Zod contract — the
same one-validator discipline the course runtime uses.

Note what `event_trace` records: expanding a step, opening the methodology note,
**skipping the card**, and failing to locate a span. Friction is converted into
signal instead of being deleted.

### 5.4 Lifecycle

1. **Surface** — `SurfaceCard` mints a `proposed` instance, no model call, no
   enforcement (surfacing only offers). It also mints a `card_instance --evaluates--> material`
   edge so a later classifier pass sees that material as spoken for and never
   re-proposes it. A project-scoped card (e.g. `toulmin`) has no material and gets
   no edge — and the frame carries the empty string, never the all-zero UUID, so
   the client can tell "no material" from "missing material".
2. **Re-offer guard** — `cardEligibleForSummon`: a card id with **any** existing
   instance in **any** status is ineligible. `skipped` counts: once the student has
   dismissed a card, we do not ask again (铁律② 不操纵). A query failure fails
   closed.
3. **Open** — the student confirms. Triggering is automatic; opening is hers.
4. **Fill** — every field write and observe event lands in anchors / trace.
5. **Complete** — `CommitCardMint` writes the minted graph nodes, their edges, and
   the consolidation framework + idempotency guard in **one transaction**.
6. **Refeed** — the outcome is serialized into a `RefeedPayload`
   (`{card_id, card_name, status, steps?}`, `steps` omitted for skipped, always
   present for completed) and handed back to the coach as a tool result.

### 5.5 The silent guidance fade

`GuidanceFor(completedUses)` maps how many times this student has completed *this
card, across all her projects* onto a ladder:

- **L1** (0 uses) — the AI elicits the guiding question **and** locates the span
- **L2** (1 use) — the AI elicits; she locates
- **L3** (2+ uses) — she elicits **and** locates

The level is never stored — it decides only what the AI fills in at surface time,
and the anchors then carry it. **The fade is silent**: no badge, no level-up, no
celebration anywhere in the UI (铁律②). It only changes what the card asks for.

### 5.6 Anchors and verbatim integrity

An `Anchor` is `{id, material_id, block_id, start, end, quote, dimension, author,
question, answer}` with **rune** (code-point) offsets, not bytes. The integrity
rule is enforced everywhere anchors are generated: a quote that is not verbatim, or
that resolves to `(0,0)`, is **never** turned into an anchor — the caller degrades
to a coach message instead of rendering a broken card. This is the same discipline
as the evaluation report's `ValidateRefs` and the writing room's literal-quote
validation: a fabricated locator is made structurally impossible to render, rather
than merely discouraged by a prompt.

---

## 6. The sub-agent roster

Each sub-agent is a **context-isolated, single-purpose call**: pure input → one
model call → a typed struct. None of them sees the whole conversation, and each is
metered with its own `llm_call` purpose.

### 6.1 Framing & planning

| Agent | Tier | What it does | Restraint |
|---|---|---|---|
| 提问卡 `question_card` | fast | Multi-turn sub-agent modal. Activates the student's own experience/intuition before AI extends it; turns a vague 目标 into a focused personal research question | `SuggestedObjective` is only *her* articulated wording echoed back for confirmation; available only while 目标 is empty |
| `framework_review` | flagship | Reads the whole 立题 framework at the readiness gate, returns `{ready, why, suggestions}` | Strong advisory — it **never blocks**; the plan generates regardless |
| `plan_gen` | flagship | Generates the project plan | Deterministic system step (see 铁律 scope note, §8) |
| `compose_journey` | flagship | Per-contract keep/waive decisions for a personalized journey | Its reason is recorded as a claim in the `journey_composed` event, never as a measurement |
| `claim_revision` | flagship | Classifies a sub-question edit as `rephrase` vs `total_change` — does the already-gathered material still apply? | Advisory warning only; never a block, never deletes her writing |

### 6.2 Reading & exploration

| Agent | Tier | What it does |
|---|---|---|
| `read_router` | flagship | Per read-turn: respond / hint / summon a reading card |
| `read_eval` | flagship | 选句复核 — evaluates the sentence the student picked against the lens's checks; returns verdict + finding/judgment/support/caveat/next-step |
| `read_card_example` | flagship | For a student-chosen lens: generate ONE example anchor to hang the pick-your-evidence flow on |
| `reading_takeaway_draft` | mid | Seeds only `new_leads` / `proposal_impact`; findings, credibility and key quotes are the student's already-confirmed work, carried verbatim |
| `dig_query` | fast | Turns her question (any language) into a 3–8-word English keyword query before OpenAlex is ever called; explicitly avoids cross-discipline collisions ("attention span", not bare "attention") |
| `search_guidance` | fast | Proposes 2–3 search directions, each a keyword + a one-line why; one-click to run, or ignore |
| `placement` | fast | Suggests which research question a new source hangs under (or 未归类) |
| `edge_propose` | flagship | Proposes labeled edges between her own question nodes. **Index-based wire protocol** — questions are numbered 1..N and the model answers in those indices, because models cannot reliably echo UUIDs |
| `exploration_guide` | mid | Points at the next necessary research direction from her own graph; never fetches, never decides which source is worth it |
| `exploration_review` | flagship | Reviews the whole exploration across sub-questions: most correlated / weakly related / still missing |
| `evidence_saturation` | flagship | Per sub-question: is the evidence map saturated? Three criteria — enough strong supporting material; **at least one limitation / challenge / substitute explanation** (support alone is never saturated); and the real test, new reading repeating what is already gathered |

### 6.3 Writing & review

| Agent | Tier | What it does |
|---|---|---|
| `proposal_guide` / `essay_guide` | fast | One guide card per writing step: a guiding question applied to *her* prompt, plus an English worked example **for a different prompt** (never the answer here), plus an optional pointer into the framework. Cached per step |
| 批注 `proposal_annotation` / `essay_annotation` | flagship | Layered (paper / paragraph / sentence) × colored (good=green / suggest=blue / problem=red) teacher annotations. Deliberately sparse; green only at paper level. **Never rewrites** — each note is a direction. Rendered view-only in the left panel, never overlaid on the editable draft |
| `order_review` (整稿体检) | flagship | A work order: per criterion, the band the draft sits in, the evidence it submits, what is missing, and an optional fix — advice only, never a rewritten sentence. `Points` says *which descriptor cell*, not a grade |
| `spot_check` | flagship | 信源体检 / 论证体检 — the same adjudicating move as 整稿体检, one and two stations earlier. These are what make the `source_quality_spot_check` / `warrant_quality_spot_check` **human** gate items satisfiable at all; when a teacher-facing surface exists, this is where a real teacher routes in, and the gate item names do not change |
| `classify` | fast | The moment classifier over a closed vocabulary (`fact_opinion`, `one_sided`) — it picks from a fixed enum and **can never invent a card id** |

### 6.4 Retrospective & reporting

`summary`, `ai_use_retrospective`, `class_weekly`, `coach_compact` / digest,
`opening` (the studio's first welcome turn), plus the four evaluation-report
generators covered in the evaluation document.

---

## 7. The reading room loop — where restraint is deterministic

The reading room is the clearest example of the split between "the model decides"
and "the system restrains":

1. `RouteReading` (flagship) proposes: respond, hint, or summon a card.
2. `ApplyReadingGate` (pure, no model) can only ever **refuse**:
   - **One-active mutex** — never a second live card over an open one.
   - **Breathing turns** — at least 2 turns between proposals.
   - **Skip cooldown** — 3 turns before re-approaching after a skip.
   - **Ordering guard** — never summon SIFT before the material has a CRAAP
     evaluation; never summon CRAAP on a material already evaluated. **The router
     may not override this.**

The reading deck is `craap` + `sift` + the nine disciplinary lenses (`lens-logic`,
`lens-methods` — reasoning; `lens-society`, `lens-law`, `lens-economics`,
`lens-ethics` — people and institutions; `lens-history`, `lens-communication`,
`lens-systems` — context and systems). The writing tool cards were deliberately
removed from this deck: "break it into a structure diagram" does not map onto the
pick-one-sentence mechanic, so a summon could almost never ground a single
illustrative sentence and hard-failed. A lens answers *from what angle to look*;
its `method_ids` point back at the writing cards that answer *what to actually do*.

A second deck, `ReadingToolkitIDs` (`cda`, `money-trail`, `multimodal-decode`,
`spin-detector`, `data-literacy`, `fact-opinion-value`, `opcvl`, `belief-spectrum`,
`search-plan`), holds source-critique tools offered contextually when a relevant
source is open.

---

## 8. Enforcement — where 铁律① stops being a prompt

`apps/api/internal/agent/enforcement/` is pure, injectable, DB-free and
network-free:

- **`ValidateOutput`** over the typed output union (`question` / `diagnostic` /
  `reference` / `proposal` / `plan` / `reply` / `advance`). `question`,
  `diagnostic` and `proposal` must each carry an anchor, a criterion and a body — an
  unanchored intervention is rejected by the type system, not by taste. A
  `reference` must carry an anchor **and** provenance: it has to resolve to the
  student's own prior artifact verbatim, or it fails.
- **`BannedPhrasing`** — a versioned regression corpus (currently v2, 8 rules)
  of forbidden AI moves in both languages: unanchored questions
  (`have you considered…` / `你有没有考虑过`), suggested counterclaims
  (`a good counterclaim would be`), candidate examples
  (`for example, you could say`), and rewritten sentences (`你应该这样写`). The
  comment on the corpus is the policy: *extend it as new draw-out failures are
  found, never silently narrow it.*
- **Lexical similarity** behind an interface so the real embedding provider stays
  outside the package and tests can stub it.

Every coach-shaped output on both the writing surface and the course surface passes
this subset. When enforcement rejects a reply, the tokens were already spent — so
the usage is still returned and still metered. Honest accounting is part of the
contract.

**Scope discipline.** 铁律① ("AI never writes it for the student") applies to the
student's **body text**. It does **not** apply to deterministic system steps —
generating a plan, deriving an outline from her already-stated research question or
from a standard framework template. Those are things the system should just do, and
adding a confirmation gate to them in the name of 反操纵 is a misreading.

---

## 9. Model routing

| Seam | Resolver | Model | Reasoning |
|---|---|---|---|
| Per-turn studio chat | `NewFastChaperoneResolver` | `deepseek-v4-pro` | **off** (request-level thinking disabled) |
| Mega-orchestrator turn | `NewKeyResolver` | `deepseek-v4-pro` | on |
| Reviewers & evaluation | `NewEvalKeyResolver` | `deepseek-v4-pro`, tier `flagship` | on — **never downgraded** |

The measured result behind this table is worth keeping: `deepseek-v4-flash` was
tried on the orchestrator turn hoping to cut latency and **regressed**. Without the
reasoning model's "reason silently, emit compact JSON" discipline, flash poured
volume into the visible output (4,000–7,000 completion tokens/turn vs. v4-pro's
600–1,100), pushing turns from ~20s to 40–66s and, on the worst runaway, truncating
the envelope mid-JSON so it fell back to the canned line. The real latency lever is
SSE-streaming the narrate (perceived first-token latency), not a cheaper tier.

The `keyResolver` seam exists so per-organization billing can later resolve a
school's own key without touching the rest of the gateway. Today the platform holds
the key, server-side only; the client never talks to a model, and **every** live
call writes an `llm_call` row (tier + tokens + cost) that the `llm_usage` view
aggregates for organization-level cost reporting.

---

## 10. The data spine

| Table | What it holds |
|---|---|
| `graph_node` / `graph_edge` | The process graph: claims, evidence, plan, gate states, card mints. The process **tree** is a projection over `parent_node_id`; there is no `process_node` table |
| `card_instances` | The standard envelope (§5.3) + `anchors` + `framework_fill` |
| `intervention` / `disposition` | Every coach intervention and what the student did with it |
| `event` | The append-only, surface-tagged event stream (`reading_focus`, `project_finished`, …) |
| `chat_message` | Turns with `role`, `stage`, `surface`, folded flag |
| `conversation_digest` | The rolling folded memory |
| `project_proposal`, `plan_item`, `reference`, `source_log_entry`, `citation`, `exploration_lead`, `question_edge`, `outline_node`, `snippet` | The project's own artifacts |
| `edit_buffer`, `draft_snapshot`, `revision_checkpoint`, `writing_finish` | The student's writing, its history, and its finish milestones |
| `llm_call` | Per-call provider/model/tier + tokens + cost, tagged with a `purpose` |

Gate state is **derived and fully recomputable from the graph**, which is why
`advanceGates` can log-and-continue on failure: the next gate-affecting write
recomputes it from scratch, and skipping a call loses nothing.

---

## 11. The contract DAG (skills)

`packages/contracts/skills/writing-project.json`, mirrored into
`apps/api/internal/skills/specs/` (generated, never hand-edited). A skill is a
project type authored as a DAG of contracts; each contract declares
`requires` / `produces` / `view` / `title` / `repertoire` and a **three-tier gate**:

- **machine** — a closed set of six predicates over the workspace graph:
  `node_present`, `node_count_at_least`, `no_orphan_evidence`,
  `no_unsupported_claim`, `no_single_sourced_claim`, `every_source_evaluated`.
  Adding a kind is a runtime change, not authoring; an unknown kind **fails closed
  with a diagnostic**.
- **student_written** — attested by the student's own recorded work.
- **human** — satisfied by an adjudicating voice she summons (§6.3's spot checks
  today; a real teacher later, with the gate item names unchanged).

`planner.go` computes the advisory route from gate state + the DAG (unmet and
reachable contracts, with a reason), stored in the project's single `plan` graph
node together with the contract ids the journey composer or the student has waived.

---

## 12. Why this is a technical shining point

1. **A tool-use loop whose executor is a human.** `summon_card` → the student
   thinks → the standard envelope is refed as a tool result. That is a genuinely
   different agent shape from "assistant that answers", and it is what makes the
   process assessable.
2. **A closed, validated tool vocabulary with per-item recovery.** Ten verbs,
   typed args, per-tool validation, per-item filtering, balanced-object salvage,
   and a server-side id-existence filter. A hallucinated id cannot reach the
   student's reference rail.
3. **Progressive disclosure as a cost architecture.** One mega-prompt collapsed
   into five ≤6-line status postures with 2–4 tools each — small enough for a fast
   tier, with reviewers kept on the flagship tier and evaluation never downgraded.
4. **Restraint is deterministic, not prompted.** The reading gate can only refuse.
   `set_status` / `generate_plan` are not the model's to call. `cardEligibleForSummon`
   treats a skip as a permanent answer. The guidance fade is silent by rule.
5. **"The AI never writes" is enforced by types.** A typed output union with
   required anchors and provenance, plus a versioned banned-phrasing regression
   corpus that both surfaces run through, plus verbatim-quote integrity everywhere
   a locator is produced.
6. **Schema-driven cards with one source of truth.** 34 JSON specs shared by the
   TS build and Go's `go:embed`; nine field primitives; a new card is a new file.
7. **Context engineering that is auditable.** The projection is a readable text
   block assembled from named, best-effort slices, with a two-stage compaction path
   that preserves the student's own reasoning rather than her transcript. Every
   surface-conditional nudge in it exists because a specific misbehavior was
   observed and pinned.
8. **Sub-agents are isolated by construction.** Around thirty of them, each pure input → one
   call → a typed struct, each separately metered. None can wander into the main
   thread's context, and each one's restraint is written next to its prompt.
