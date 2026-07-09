# Two AI Agents — Design Summary & Research Directions (2026-07-08)

> A grounded map of the two AI agents that make up 思维印记: the **platform chaperone** (the
> critical-thinking workspace turn engine) and the **course agent** (the lesson renderer). Every
> claim below is traced to the server code in `apps/api/internal/agent/` and `apps/api/internal/api/`.
> The last section lists concrete directions if we want to research/optimize either system.

Both agents share three hard invariants (AGENTS.md 四条设计铁律): **AI 克制 (never concludes for the
student)**, **一次只问一个 (one question at a time)**, and **过程即数据 (every envelope feeds the
process tree + evaluation)**. The client never calls a model — all LLM traffic goes through the Go
gateway, which alone holds keys and records tier/token/cost.

---

## Part 1 · The Platform Chaperone Agent (workspace turn engine)

The student brings a **real task** (an essay, a claim to check, an article to cite) and thinks *with*
the AI. The chaperone's job is not to answer but to hand thinking back at the right moment.

### 1.1 The turn loop — `RunTurn` (`agent/turn.go`)

One student message = one model turn. `RunTurn` does, in order:

1. **Append the user message** (with a `source` tag — `"voice"` or empty/typed) unless this is a
   *continuation turn* (empty input, used to reply after a card is submitted/skipped).
2. **Rebuild the full context each turn** — there is no server-side conversational memory beyond the
   DB. It reloads the message history, composes the system prompt, and appends the materials block.
3. **Replay history into LLM messages** (`BuildLlmMessages`, `agent/messages.go`): each *resolved*
   card proposal becomes a `tool_use` + paired `tool_result` (the serialized refeed); each
   *unresolved* proposal degrades to plain coaching text.
4. **Stream the provider** with exactly one tool available: `summon_card`.
5. **Branch on the response:**
   - If the model emitted a `summon_card` tool call with a valid `card_id` → create a *proposed*
     `card_instance`, (for annotation cards) generate anchors, persist the assistant message with
     per-turn usage, emit a `card` SSE event, end the turn. **One card per turn — the first valid
     tool call wins.**
   - Otherwise → a plain text reply.
6. **After the turn**, `postTurn` (`api/turn.go`) best-effort triggers a milestone evaluation.

Design consequence: the agent is **stateless between turns** and **idempotent to replay** — the DB
transcript is the single source of truth. This is what makes rehydration (ST-1) and async eval
possible.

### 1.2 How it decides *how* to help — the 克制阶梯 system prompt (`agent/prompt.go`)

The system prompt (`promptTemplate`, byte-for-byte pinned by a golden test) is the whole behavioral
contract. It gives the model a **ladder of coaching moves** rather than a single reflex:

- give a **hint** that nudges one small step forward;
- ask **one guiding question** so the student finds the gap themselves;
- **point out an angle / counter-example** they missed;
- **affirm** what they already got right;
- when the situation *fits*, **propose one tool card**.

Explicit anti-reflex instruction: *"你有一整套教练手段，按情况挑用，而不是每次都反问"* — i.e. do **not**
always ask a question. Restraint means "don't conclude for them," **not** "hide the tools." The prompt
also mandates: **focus one step, short & spoken, use Markdown for emphasis.**

### 1.3 How it gets article content — materials, not link-fetching (`agent/prompt.go`)

A deliberate honesty constraint sits in the prompt: **the AI cannot open links or read web pages/PDFs
— it only sees text the student pastes.** When a student drops a bare link, the prompt tells it to:
say plainly it can't see the content, ask for the **key passage / data / original words**, and treat
that as the starting point to *source-check together*.

Mechanically, pasted/seeded article text becomes **materials**: each material is segmented into
**block-addressable paragraphs** (`MaterialBlock{ID, Text}`). `BuildMaterialContext` injects them into
the system prompt as `[b0] …` lines the model can quote by `block_id`. This block addressing is what
lets annotation cards pin a question to a specific sentence.

### 1.4 How it applies tool cards — the decision layer (`agent/prompt.go`, `agent/turn.go`)

Tool cards are modeled as **a tool in a tool-use loop whose "executor" is the human.** The plumbing:

- **`summon_card(card_id, reason, nudge_text)`** is the *only* tool (`SummonCardTool`). Its `card_id`
  enum is derived from the card registry — the decision-layer catalog is never hand-written (avoids
  `trigger_condition` drift).
- The catalog is rendered into the prompt grouped by category, each card carrying **「何时用」
  (trigger_condition)** and **「能帮他」(purpose)** (`BuildCatalogText`). The model first classifies
  which *category* of problem the student is stuck on, then picks the single best-fitting card.
- `reason` is written for the system (why it fits now); `nudge_text` is written for the student (a
  natural, invitational, non-commanding line).
- **One card per turn max.** On a decline/skip, the prompt forbids re-pushing the same card.
- **Confirm-to-open:** the model only *proposes*; the student presses 打开卡 (a `PATCH` that records
  the `active` transition) or 暂不. Skipping is itself recorded (过程即数据).

### 1.5 Material-anchored (keystone) cards — `AnchorGenerator` (`agent/anchors.go`)

For cards with `mode: "annotation"` (the flagship "AI asks a question pinned to a sentence you're
reading"), when the card is summoned `RunTurn` calls `AnchorGen.Generate`:

- The generator prompts a model to pick, **for each card dimension**, one original sentence from the
  materials + a specific guiding question tied to it, returning `{block_id, quote, dimension,
  question}`. Server-side it computes byte offsets and stamps `author:"ai"`, empty `answer`.
- **Deterministic fallback** (`fallbackAnchors`): if the model fails or there are no materials, it
  emits one *unanchored* per-dimension question (empty `block_id`). This is why the flagship only
  "lands" when material is present *before* the summon.
- Anchors ride the `card` SSE event at summon time (ST-2), so the client never depends on a refetch.

### 1.6 When it *asks* vs when it *invites the student to ask*

Two distinct question modes, both encoded in prompts:

- **The chaperone asks** (default coaching): the 克制阶梯 ladder — a single guiding question when a
  gap is worth surfacing. Governed by "一次只问一个".
- **It invites the student to propose the question** (the keystone move): annotation cards flip the
  authorship — the affordance is *"在右侧文章里划一句，自己向印记提问"*. The AI seeds a model question
  anchored to a sentence, then the student writes their own question/answer against the material. This
  is the product's highest-value interaction: authorship of the question moves to the learner.

### 1.7 摘要回灌 (refeed) — closing the loop (`agent/refeed.go`)

When a card is completed/skipped, `SerializeCardForRefeed` turns the standard envelope into a compact
`{card_id, card_name, status, steps?}` payload that re-enters the transcript as the tool's
`tool_result`. The next turn, the model "catches" the student's own written answers and pushes forward
on **one** of them. (Form cards store answers in `field_values`; annotation cards store them in
`anchors` — as of 2026-07-08 both are folded into the refeed.)

### 1.8 過程即數據 — the evaluation trigger (`api/eval_trigger.go`, `agent/eval.go`)

- A **milestone index** = `completedCards + substantiveTurns/6` collapses progress into one monotonic
  integer. Crossing a new milestone best-effort enqueues an evaluation (capped at 3 auto-evals,
  debounced, one-in-flight). It **never blocks or fails the student's request.**
- The evaluator (`runEval`) assembles the full conversation + every card envelope
  (`AssembleEvalInput`) and grades against the 10-dimension SOLO rubric — **always the flagship model,
  never downgraded.** Output = per-dimension SOLO level + a process narrative, surfaced as
  「你的思维印记」.

---

## Part 2 · The Course Agent (lesson renderer)

The course agent is a **different, simpler shape**: not a conversational loop but a **per-step
just-in-time renderer** that turns an *authored lesson skeleton* into live, on-topic teaching content,
with a guaranteed fallback. Entry point: `RenderCourseStep` (`agent/course.go`), driven by the handler
`renderCourseStep` (`api/course_render.go`).

### 2.1 What resources are brought in

A course step (`CourseStepInput`) carries:

- **`kind`** — `teaching` or `challenge`.
- **`purpose`** — the pedagogical goal of this step (authored).
- **`assets`** (`CourseAsset{id, kind, title, value}`) — the raw teaching material: text passages,
  and (by design, extensible) other kinds. These are the grounding the model composes from.
- **`authored_content`** — a curated fallback rendering, always present.
- For challenges, a **`challenge_type`** (e.g. `verify_claim`).

### 2.2 How the AI composes them & what it presents

**Teaching step** (`renderTeaching`):

1. System prompt = *"你是一名循循善诱的老师，这一步的教学目的是：{purpose}"* + the assets as the user
   message.
2. The model must return strict JSON: `{title, subtitle, body[], foreground_asset_id}` — a page title,
   a one-line spoken intro, body paragraphs, and which asset to foreground.
3. On any failure (resolver error, malformed JSON, missing title/subtitle/empty body) → return the
   **authored fallback**. Never a broken page.
4. What the student sees: a lesson page in the fixed `teaching` template, rendered from that JSON.

**Challenge step** (`renderChallenge`): reuses the *platform's* anchor machinery. It segments the
challenge text into blocks, builds a synthetic card spec whose "dimensions" come from the challenge
type (e.g. `verify_claim` → Authority / Accuracy / Purpose), and runs `AnchorGenerator` to pin a
question to a real sentence. **If anchoring fails (all fallback, empty `block_id`), it prefers the
curated authored challenge over generic placeholders** — a quality gate, not just a crash guard.

### 2.3 How the full process is generated & persisted

- **Render-on-demand, cache-once** (`api/course_render.go`): on first view of a step, generate; on
  success (`source == "generated"`) upsert into `course_step_render` keyed by step id; on later views,
  serve the cache. The **authored fallback is never cached** — so a transient model failure doesn't
  pin a degraded page forever; the next viewer retries generation.
- Entitlement is checked before any token spend (`HasEntitlement`, currently a stub returning true).
- Net: a course is an **authored skeleton (skeleton = purpose + assets + fallback) that the model
  "inflates" into live prose per step, deterministically falling back, and caching the good result.**

### 2.4 Where the course loop is weaker than the platform loop (from the live audits)

- Courses currently emit **no evaluation/activity signal** → invisible in 我的评估.
- In-course **challenge answers are ephemeral** — not persisted or evaluated like workspace envelopes.
- There is **no in-course chat** — teaching is one-directional render, not a dialogue.

---

## Part 3 · Research / Optimization Directions

### 3.1 Course agent — "how do we make the generated lesson genuinely good?"

1. **Grounding & anti-hallucination (highest leverage).** Today the model composes teaching prose from
   whatever assets a step carries, with only a JSON-shape check. Directions: (a) **retrieval-grounded
   teaching** — attach a small vetted knowledge pack per topic and require the model to cite/quote
   only from it; (b) **claim-level verification** — a second pass that checks each generated body
   paragraph is supported by an asset, rejecting/regenerating unsupported claims (mirror the platform's
   anchor "prefer authored over generic" gate).
2. **Quality evaluation harness.** There is currently no automated judge of teaching quality. Build an
   **offline eval set** (topic + assets → rendered page) scored by an LLM-judge on {on-topic,
   faithful-to-assets, right altitude for IB, no lorem-ipsum tells}. This turns "does the course read
   well" from vibes into a regression metric — prerequisite for any prompt/model iteration.
3. **Adaptivity & personalization.** The renderer ignores the learner. Directions: condition `purpose`
   expansion on prior evaluation signal (a student weak on 信源辨识 gets a source-check-heavy framing),
   and branch difficulty on in-course challenge performance — which first requires **persisting course
   activity** (close the 2.4 gap).
4. **Make courses feed the process tree.** Persist + evaluate challenge answers with the same standard
   envelope the workspace uses, so a course becomes a first-class 过程即数据 surface, not a dead end.
5. **Multi-modal assets.** `CourseAsset.kind` is already a discriminator but only `text` is exploited.
   Image/data assets + a vision-capable compose step would let "卫星图" actually be shown and reasoned
   about.

### 3.2 Platform chaperone — "how do we make the coaching sharper?"

1. **Card-summon precision (the decision layer).** The model picks a card from `何时用/能帮他` text
   alone. Directions: build a **labeled bench of (conversation snippet → correct card / no-card)** and
   measure summon precision/recall; then test cheaper interventions before fine-tuning — e.g.
   richer/structured trigger descriptions, a two-stage classify-then-pick, or few-shot exemplars per
   category. This is the single biggest driver of whether the flagship experience triggers at all.
2. **Anchor quality & *timing* (flagship-critical).** The live audits show the keystone only lands when
   material is pasted **before** the summon; otherwise anchors fall back to generic per-dimension
   questions (empty `block_id`). Directions: **gate the annotation summon on material-present**, or
   **re-anchor when material arrives later**; and add an **anchor-quality judge** (is the picked
   sentence actually the most relevant? is the question specific to it?) to regression-test the
   generator.
3. **The "ask vs invite" policy.** We have two question modes (§1.6) but no measurement of when each is
   right. Research: does inviting the student to author the question (annotation) produce measurably
   deeper SOLO outcomes than the AI asking? An A/B on card-mode coverage, scored by the existing SOLO
   evaluator, would tell us **which of the 33 cards should be annotation-mode** — currently a guess.
4. **Refeed richness.** The refeed is a compact label→value dump. Directions: test whether a short
   *model-summarized* refeed ("the student concluded X but left Y unexamined") drives a better next
   turn than the raw envelope — without violating restraint (summarize the student's words, don't
   conclude for them). Watch the round-trip cost (the audit's "extra re-ask turn").
5. **Evaluation calibration.** The SOLO evaluator is the product's payload but has no ground truth.
   Directions: assemble **expert-graded transcripts**, measure evaluator agreement (per-dimension
   confusion vs. human SOLO levels), and calibrate the rubric anchors. Also worth testing:
   self-consistency (sample N, take majority) on the flagship eval for stability.
6. **Cost/latency without quality loss.** Every turn rebuilds full context + streams a chaperone model,
   and annotation summons add an anchor-gen call. Directions: prompt-cache the static system prompt +
   catalog; measure whether the mid-tier chaperone can be downgraded further on non-summon turns while
   holding summon quality (the tier seam already exists in the gateway).

### 3.3 Cross-cutting

- **A shared offline eval harness** is the meta-recommendation: both agents currently ship on manual
  E2E audits. A small, versioned bench (course-render quality, card-summon precision, anchor quality,
  SOLO-evaluator agreement) converts every future prompt/model change from a vibe into a diffable
  number — and is the prerequisite for safely swapping models (DeepSeek ↔ Anthropic) or fine-tuning.

---

### Appendix · Key source references

| Concern | File · symbol |
|---|---|
| Turn loop | `agent/turn.go` · `RunTurn` |
| System prompt (克制阶梯) | `agent/prompt.go` · `promptTemplate`, `BuildSystemPrompt` |
| Materials / block addressing | `agent/prompt.go` · `BuildMaterialContext` |
| Card decision tool | `agent/prompt.go` · `SummonCardTool`, `BuildCatalogText` |
| History → LLM messages | `agent/messages.go` · `BuildLlmMessages` |
| Anchors (keystone) | `agent/anchors.go` · `AnchorGenerator`, `fallbackAnchors` |
| Refeed (摘要回灌) | `agent/refeed.go` · `SerializeCardForRefeed` |
| Eval trigger | `api/eval_trigger.go` · `MilestoneIndex`, `maybeTriggerMilestoneEval` |
| Eval compute | `agent/eval.go` · `runEval`; `agent/evalinput.go` · `AssembleEvalInput` |
| Course render | `agent/course.go` · `RenderCourseStep`, `renderTeaching`, `renderChallenge` |
| Course cache/handler | `api/course_render.go` · `renderCourseStep` |
