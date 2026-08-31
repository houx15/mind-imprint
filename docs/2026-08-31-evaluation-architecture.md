# Process Evaluation (过程评估) — Architecture

> **Scope.** The evaluation pipeline end to end: the canonical dual-axis model,
> the full input surface it reads (every recorded stream), the deterministic
> pre-processing that turns that surface into bounded, groundable context, the
> four-call generation workflow, the anti-hallucination stack, the nine output
> sections, and the per-audience projections.
>
> **Companion documents.** `docs/2026-08-11-evaluation-and-teacher-code-map.md`
> (where every endpoint/query/migration lives), `docs/2026-08-11-evaluation-data-storage-guide.md`
> (where the data lands), `docs/2026-08-13-evaluation-design.md` (report structure
> spec), `docs/2026-08-15-eval-report-generator-benchmark.html` (measured results).

---

## 0. The one-sentence claim

**We assess the process, from the process's own records, and we can prove every
sentence.**

The final essay is one input among dozens. The report is built from a project's
whole recorded trajectory — chats, cards (including skipped ones), sources and how
they were tiered, exploration leads, revisions, annotations, reflections — and
every judgment it prints must cite a real record id, or the id is stripped before
the report is stored.

Two axioms are structural, not stylistic:

1. **两轴永不合成总分** — the depth axis and the autonomy axis never combine into a
   single score.
2. **单次会话为事件级证据，不构成人级档位判定** — one session is event-level evidence,
   not a verdict on a person.

---

## 1. The canonical model

`apps/api/internal/rubric/dualaxis.json` (embedded via `go:embed`), mirrored in
`packages/contracts/src/dualaxis.json`.

### 1.1 Depth axis — D1–D6, four anchored levels each (L1–L4)

| | Dimension | What it means |
|---|---|---|
| D1 | 任务理解与问题表述 | Narrowing a broad topic into a question that can actually be researched and answered |
| D2 | 证据与信源 | Finding material, judging whether it can be trusted, saying what each source can support |
| D3 | 论证结构 | Deriving a conclusion from evidence step by step, rather than a conclusion larger than its evidence |
| D4 | 视角与偏见 | Engaging opposing views; acknowledging limits and bias |
| D5 | 反馈处理与修订 | After a challenge — genuinely improving the argument, or only the wording |
| D6 | 反思与元认知 | Looking back at one's own reasoning and saying where a judgment came from |

Each level has a written anchor. D2's L4, for example, is "traced to primary
sources, identified counter-evidence, analyzed the source's stance and the range
its evidence applies to; derived the gap stably from the relations between
studies." These anchors go into the prompt verbatim — the model is scoring against
a published rubric, not an implicit sense of quality.

**D6 carries a `reflectionRule`:** it is evidenced only by student-authored
reflection. Absent reflection makes D6 **NA**, never a low score.

### 1.2 Autonomy axis — A1–A6, bands 0–5

Behavior-count signals, explicitly **not** quality scores:

| | Signal | What it means |
|---|---|---|
| A1 | 方向自主 | Deciding the research direction herself, rather than letting a teacher or an AI decide |
| A2 | 发起自主 | Going to verify and to gather more evidence on her own initiative |
| A3 | 边界主权 | Setting boundaries with the AI (no ghostwriting, no invented sources) |
| A4 | 对抗与检验 | Asking to be challenged; hunting for holes in her own argument |
| A5 | 判断署名 | Owning her conclusions — "this is my judgment" |
| A6 | 求真优先 | When the evidence does not support it, shrinking the claim rather than defending it |

### 1.3 Prompt lenses — six process indicators

`L_decisions` (how many of the five decisions a prompt carries: task / material /
boundary / acceptance criteria …), `L_maturity` (asking for an answer or a product
vs. asking for a process or a judgment), `L_boundary` (count of effective boundary
statements), `L_adversary` (count of requests to be challenged), `L_directive`
(share of turns that actively change the task or pick a route), `L_acceptance`
(does she supply acceptance criteria that trace back to the research question,
method or evidence — as opposed to "make it more academic").

Lenses read AI-interaction traces only. **They are never a third scoring axis.**

### 1.4 Official-standard alignment

The config also carries external standards (e.g. AP Research's Academic Paper /
POD / calibration note / integrity components) as **reference alignment**. They are
aligned to, never combined into the dual-axis result.

---

## 2. The input surface — what "enormous" actually means

Nothing here is a snapshot of the final artifact. Every stream below is
append-only or versioned, written during the work, and read at evaluation time.

### 2.1 Framing and planning

| Source | Contents |
|---|---|
| `project` | Title, qualification, created-at |
| `project_proposal` | The five 立题 dimensions: 目标 / 缘由 / 活动与时间 / 资源 / 可能的反例 |
| `plan_item` | Every plan step with its stage tag, column (`todo`/`doing`/`done`), and days |
| `graph_node` / `graph_edge` | The process graph: claims, evidence, the `plan` node, gate states, card mints, and the edges between them |

### 2.2 Sources, reading and exploration

| Source | Contents |
|---|---|
| `reference` | Title, URL, credibility verdict, takeaway, evidence finding, plus the reading brief (`reading_reason`, `reading_focus`, `phase_tag`) |
| `source_log_entry` | The tier she assigned at ingestion, accumulated reading seconds, and `lateral_read` — flipped only once a cross-check actually happened on **that** source |
| `citation` | Which source is cited where in the body |
| `exploration_lead` | Open leads she logged |
| `question_edge` | Labeled relations she confirmed between her own research questions |
| `reading_takeaway`, `reading_block_note`, `reading_brief` | Her reading-room work products |

### 2.3 Thinking tool cards

`card_instances` — the standard envelope, and the reason skipped cards matter:

- `status` ∈ `proposed | active | completed | skipped` — **a skip is data.**
- `field_values` — what she actually filled in, per step, per field.
- `event_trace` — `field_change`, `step_expand`, `note_open`, `skip`, `submit`,
  `span_located`, `span_not_found`. Expanding a step, opening the methodology note,
  and **failing to locate a span** are all recorded.
- `anchors` — rune-indexed spans into a material with her question and answer.
- `framework_fill` — the consolidation a completed card mints.

### 2.4 Interaction

| Source | Contents |
|---|---|
| `chat_message` | Every turn with `role`, `stage`, `surface`, folded flag |
| `conversation_digest` | The rolling compaction of folded turns — her reasoning and decisions, preserved |
| `intervention` | Every coach intervention with its anchor, criterion, I-ladder level and output-check verdict |
| `disposition` | What she did with each intervention |
| `event` | The append-only, surface-tagged stream: `reading_focus` (what she had selected while reading, with her own words), `project_finished`, milestone events, and more |

### 2.5 Writing and revision

| Source | Contents |
|---|---|
| `edit_buffer` / `draft_snapshot` | The live buffer and its versioned snapshots, keyed by `doc_kind` (`proposal` / `essay`) |
| `revision_checkpoint` | Each recorded revision |
| `writing_finish` | Per-document finish milestones |
| `outline_node`, `snippet`, `writing_comment` | Outline, fragments, and persisted comments |
| 批注 (annotations) | The layered/colored teacher-style annotations, counted for the report |

### 2.6 Reflection and self-report

| Source | Contents |
|---|---|
| `project_reflection` | Her five-dimension retrospective |
| `project_ai_use` | The AI-use statement she authored (used-for / not-used-for). The AI assembles the objective record and may seed a first-person draft; the statement itself is hers |

### 2.7 Metering

`llm_call` — provider, model, tier, prompt/completion tokens, cost and a `purpose`
string for every live call. It is both the cost ledger (aggregated by the
`llm_usage` view for organization reporting) and, filtered through
`evalreport.StudentFacingSubagents`, the inventory of which sub-agents the student
actually engaged with.

**Only three sub-agent groups are considered student-facing** and reach the
report's `promptLens` / `toolUsage`:

- `review` → `proposal_annotation`, `essay_annotation`, `order_review`
- `card` → `question_card`, `read_card_example`
- `reading` → `read_router`, `read_eval`, `reading_takeaway_draft`

Everything else — search/dig, exploration guide and review, edge proposal,
placement, plan generation, opening, classification, compaction — is internal and
deliberately excluded. Their existence is metered; they are not presented to the
student as her own interactions.

---

## 3. Deterministic pre-processing

The report has a **FACT half** and an **LLM half**. The FACT half never touches a
model.

### 3.1 Hard signals — `ComputeSignals`

Derives facts with no semantic judgment: message counts by role, student character
count, distinct-URL counts by author (student vs. AI), per-card signals (status,
op count, filled and empty fields), max verbatim overlap between AI output and the
student's text, and N/A candidates.

### 3.2 The FACT half of the report

| Field | Source |
|---|---|
| `counters` | `aiTurns` (assistant messages), `materialsRead` (references), `wordsWritten`, `aiCommentCount`, `editCount` (revision checkpoints) |

`wordsWritten` uses `CountWords`, one canonical rule for mixed Chinese/English
writing: **CJK ideographs + non-CJK whitespace-delimited tokens**, with CJK
punctuation excluded and glued runs split ("中国GDP" → 2 + 1). Whitespace
tokenizing alone undercounts CJK by ~100%; counting characters over-counts English
by ~5×. Implemented once, server-side, and reused everywhere a body length is
shown.
| `milestones` | `started` (project created), `frameworkFinished`, `proposalFinished`, `writingFinished` (per-doc `writing_finish`), `projectFinished` (latest `project_finished` event) |
| `events` | The compact reader-facing timeline mapped from the event stream |
| `materials` | Per source: added-at, origin, URL, where it was used (citation → section, else lead), final status, comment, and what it cannot support |
| `toolUsage` | Assembled from card instances |
| `basics.title` | The research question (`proposal.objective`), falling back to the project title |

The **finished body** resolution order is: latest `essay` snapshot → `essay` edit
buffer → latest `proposal` snapshot. A project that never reached the essay is
still evaluated on what exists.

### 3.3 Four context-partitioned digests

The design decision here: **do not send the big trajectory digest to a call that
only needs a slice of it.** Each digest is separately built and rune-budgeted.

| Digest | Budget | Contents | Feeds |
|---|---|---|---|
| `Trajectory` | 9,000 runes | 立题框架 (5 dims) → **学生自写反思** → plan stages → sources with credibility/takeaway/evidence-finding → cards with status → body excerpt (1,800 runes) | rubric (C), abstract (D) |
| `Prompts` | 6,000 runes | Every student chat turn with its `[id]` and stage, plus every `reading_focus` event's student text | promptLens (A) |
| `RiskSignals` | 5,000 runes | Body excerpt + per-source traceability state (有链接 / 无链接·未溯源) + citation and lead counts | risks (B) |
| `Counters` | one line | The counter summary | abstract (D) |

One detail worth calling out because it fixed a real scoring bug: **the student's
own reflection is lifted to the front of the trajectory digest.** Reflection
usually sits at the end of a draft, the draft excerpt is capped, and tail
truncation was burying D6's only valid evidence — scoring D6 at 1 when it should
have been 3. `extractReflection` finds the 反思 / Reflection section and surfaces it
up-front, capped at 900 runes.

### 3.4 The evidence index

`BuildEvidenceIndex` gathers every citable record across **five streams** — chat
messages, events, references, cards, exploration leads — into an id-indexed set,
each with a 60-rune human label so the model can pick the right id. Up to 140
candidates are rendered into the prompt as `[id] kind: label` lines.

A failure in any single stream fails the whole build: the generator wants a
complete candidate set or none, because a partial set would silently make real ids
un-citable.

---

## 4. The generation workflow

`runReportGeneration` (`api/evaluation_generate.go`) + `agent/reportgen.go`.

```
                    ┌──────────────────────────────────────────┐
 recorded data ────►│ assembleFacts (deterministic, no model)   │──┐
                    └──────────────────────────────────────────┘  │
                    ┌──────────────────────────────────────────┐  │
                    │ BuildEvidenceIndex  →  candidate list      │  │
                    └──────────────────────────────────────────┘  │
                    ┌──────────────────────────────────────────┐  │
                    │ 4 digests (context-partitioned, budgeted) │  │
                    └───────────────┬──────────────────────────┘  │
                                    │                              │
        resolveEval (FLAGSHIP, never downgraded)                   │
                                    │                              │
        ┌───────────┬───────────────┴──────────┐                   │
        ▼           ▼                          ▼                   │
   A promptLens  B risks                  C rubric                 │
   (lean)        (lean)                   (full trajectory)        │
        └───────────┴───────────────┬──────────┘  concurrent       │
                                    ▼                              │
                             D abstract (needs C's axes)           │
                                    │                              │
                                    ▼                              ▼
                          assemble Report ◄─────────────────────────
                                    │
                            ValidateRefs (drop unknown ids)
                                    │
                          store as evaluation_report
```

### 4.1 The four calls

| | Call | Context it receives | Produces |
|---|---|---|---|
| A | `GeneratePromptLens` | prompts digest only | The six-lens read + per-prompt items (stage, quote, ref, observation, related domains, attention flag) |
| B | `GenerateRisks` | risk-signal digest only | Risk entries: type, behaviour, ref, suggestion |
| C | `GenerateRubric` | full trajectory + candidates | **12 judgments** — D1–D6 with level 1–4, A1–A6 with band 0–5; each with a summary, evidence items (id, quote, observation, **boundary**) and a suggestion |
| D | `GenerateAbstract` | full digest + counters + C's axis results | Overview, material sentence, writing sentence, AI sentence, suggestion paragraph and sentences, recommended courses |

A, B and C are context-independent and run **concurrently** (goroutines +
`WaitGroup`); D waits because it synthesizes C's results.

Each call: `MaxTokens` 16,000; system prompt = rules, user message = facts;
JSON-object salvage on the reply; **one retry** on an unparseable reply; metered as
`llm_call` `purpose = eval_report_{promptlens,risks,rubric,abstract}`; timed into a
per-call stat (input/output tokens, millis, ok, error).

### 4.2 Degradation policy

**Best-effort by construction.** A failed or absent LLM call degrades *that
section* rather than failing the report:

- No resolver / no provider → a `resolver` stat is recorded, all four sections
  degrade, the FACT half still stores a usable report.
- `fillDepth` / `fillAutonomy` backfill any missing dimension to a conservative
  floor, so the report always carries a full 6 + 6.
- A parse failure on the abstract yields an empty abstract, not a dead report.
- **Usage is metered even for a failed call** — the tokens were spent.

Only a data-gather failure returns an error; that fails the row so it becomes
re-claimable rather than stuck.

### 4.3 Lifecycle

- `POST /projects/{id}/evaluation-report/generate` and the project-finish flow
  share one write core, `generateAndStoreEvaluationReport`.
- Generation is claimed **atomically** (`ClaimEvaluationReportGeneration`, an
  `ON CONFLICT` claim): concurrent callers can never double-fire. A `ready` or
  fresh `generating` row is left alone; a `failed` or stale `generating` row is
  re-claimed — that is the 重试 path.
- Project finish runs it in a detached worker (`go a.runProjectReport`) on a fresh
  context, so finishing does not block on a ~4-minute generation.
- **Read-no-call.** `GET` returns a three-state envelope — JSON `null` when no row
  exists, `{"status":"generating"|"failed"}`, or `{"status":"ready","report":{…}}`
  — and **never** 404s and **never** calls a model.
- The teacher's read of the same report
  (`GET /classes/{id}/students/{userId}/evaluation-report/{projectId}`) is the same
  stored object, guarded by class ownership + this-class membership + an explicit
  project-ownership check, so a valid `projectId` belonging to a different student
  still 404s (IDOR-safe not-found).

### 4.4 Measured cost and behavior

Benchmarked on 10 seeded personas against `deepseek-v4-pro` (env-gated
`evaluation_benchmark_test.go`, incremental per-persona writes so a kill does not
lose the run):

- **Average per report: ≈13.7k in / 24.2k out tokens, 3.9 minutes, $0.027.**
- Per call: promptLens ≈3.2k/4.2k/57s (10/10) · risks ≈3.4k/4.2k/63s (10/10) ·
  **rubric ≈5.2k/13.1k/186s (10/10 — the long pole: 54% of output, 48% of time)** ·
  abstract ≈2.0k/2.8k/46s (9/10, one parse failure degrading gracefully to empty).
- **It discriminates cleanly:** an L1 persona scored depth ≈1.7 / autonomy ≈0.2; an
  L3 persona scored depth 3.5 / autonomy 4.2.
- Ref-drop rates scale with sparsity (a thin L1 project dropped 15 of 35 ids) —
  which is the intended behavior, not a defect: a sparse project has fewer real
  records to cite.
- Overviews came back honest and non-flattering.

---

## 5. The anti-hallucination stack

A free-writing model invents ids. Three independent layers stop that:

1. **Closed candidate list in the prompt.** The generator is told, in the user
   message, that `evidence.id` may only be one of the bracketed ids given, and to
   leave it as an empty string when it cannot find one.
2. **Schema parse.** Every reply is parsed into a typed struct; unknown shape is a
   retry, then a degraded section.
3. **`ValidateRefs` at store time.** Every `Ref.id` and every bare evidence id that
   does not resolve in the index is **blanked in place** — the label is kept so the
   UI still renders text, but the dead id is removed so no broken deep-link is
   offered. It returns `{Total, Dropped}` as a **quality signal**, which is recorded
   in the benchmark stats.

This is the same discipline used across the product wherever a model produces a
locator: the writing room's comment points must appear literally in the commented
text or they are dropped; card anchors are never minted from a non-verbatim quote;
the exported report's 金句 can only be quoted from a corpus that structurally
excludes source text and AI text. **The rule is consistent: make fabrication
impossible to render, rather than merely discouraged by a prompt.**

Go validates only the report envelope's border (`version == 1`, non-empty
`reportId` / `projectId` / `student.id`); deep shape truth stays in the Zod
contract.

---

## 6. Output — the nine sections

`EvaluationReportView` renders a sticky ruler 目录 plus a scrollable body, nine
sections in fixed order, each anchored `id="sN"` for scroll-sync.

| # | Section | Kind |
|---|---|---|
| 01 | 基本信息 — title, dates, milestones, counters | **FACT** |
| 02 | 综述 — overview, material/writing/AI sentences, suggestions, recommended courses | **PROSE** (call D) |
| 03 | 过程时间线 — milestones, source adds, card completions, reviews, finishes | **FACT** |
| 04 | 材料清单 — each source, where used, final status, what it cannot support | **FACT** |
| 05 | 认知深度 D — D1–D6, level + summary + evidence + suggestion | **MODEL** (call C) + **REF** |
| 06 | 智识自主 A — A1–A6, band + summary + evidence + suggestion | **MODEL** (call C) + **REF** |
| 07 | 提问透镜 — lens summary + per-prompt items | **MODEL** (call A) + **REF** |
| 08 | 工具卡与子代理 — which cards and which student-facing sub-agents, with purpose | **FACT** |
| 09 | 风险提示 — behaviour, ref, suggestion | **MODEL** (call B) + **REF** |

### 6.1 How the axes are displayed

The raw L1–L4 / band 0–5 numbers are **hidden from the student-facing report**.
They are mapped to semantic tiers with their own words and colors:

- depth level 1–4 → 需加深 / 发展中 / 良好 / 扎实
- autonomy band 0–1 → 偏依赖, 2 → 渐自主, 3 → 较自主, 4–5 → 高自主

Position on the visual ruler is by **color only** — there is no numeric ladder to
read off, no total, and no rank. Two axes, never summed.

Every evidence item carries a `boundary` field alongside its quote and
observation — the explicit statement of what this piece of evidence does **not**
establish. That is the difference between a report that assesses and a report that
labels.

### 6.2 Export

导出 PDF is `window.print()` over a print-only React document
(`shell/report/print/`). No backend renderer, no headless browser, no PDF service.

---

## 7. Per-audience projections

One canonical evaluation object, projected per audience — never a second
assessment.

| Audience | Surface | Model calls |
|---|---|---|
| Student | The nine-section report + PDF export | none on read |
| Teacher — per student | The same stored report | none |
| Teacher — roster (实时) | Current-state activity counts | **none** |
| Teacher — 班级周报 | 值得表扬 / 需要建议 cards, every one carrying evidence | one composer call over `WeeklyFacts` |
| Parent | Stage / project prose | one composer call |

The class-weekly composer deserves a note because it is the pattern: `WeeklyFacts`
is a **deterministic fact sheet** built by `teacher.BuildWeeklyFacts`, and **every
name, tag and number in it was decided by the rule layer, never by the model that
later turns it into prose.** The model's job is wording; the facts are not
negotiable.

`strong_engagement` was removed from the teacher-end activity model on purpose
(铁律②): "this student is highly engaged" is an engagement metric, and engagement
metrics are the seed of the slot-machine dynamics this product exists to refuse.

The AI-use retrospective follows the same split: the objective interaction record
is assembled server-side and can never be echoed or forged by the model (the parse
target carries only the two authored fields); the model may seed a first-person
draft; the statement the student signs is hers.

---

## 8. Why this is a technical shining point

1. **The input is the process, not the artifact.** Dozens of append-only streams,
   including the ones that record friction — skipped cards, expanded steps,
   spans she failed to locate, revisions that changed wording but not argument.
   These are the signals a finished essay cannot carry.
2. **The rubric is a config file, not a prompt.** Six anchored depth dimensions,
   six autonomy bands, six lenses and external-standard alignment live in one
   embedded JSON shared by Go and TypeScript. Recalibration is a config change with
   a golden-test diff, not a prompt rewrite.
3. **Context is partitioned by what each judgment actually needs.** Four separate
   digests with their own rune budgets, so the lean calls stay lean and the
   expensive call carries only the trajectory. That is what makes a 12-judgment
   flagship report cost $0.027.
4. **A three-layer citation guarantee.** Closed candidate list → typed parse →
   post-validation that blanks unknown ids and reports the drop rate as a quality
   metric. Every deep-link in a stored report resolves.
5. **Degradation is designed, not accidental.** A failed call degrades one section;
   the report always stores; failed calls are still metered; the row is
   re-claimable; reads never call a model and never 404.
6. **The evaluation tier is never downgraded.** Coaching runs on a fast resolver;
   every assessment path resolves through the flagship `EvalResolver` seam.
7. **The refusals are as engineered as the features.** Two axes that never sum.
   Numeric levels hidden behind semantic tiers, positioned by color only. NA
   instead of a low score when the evidence for a dimension does not exist. A
   `boundary` field on every piece of evidence. An engagement metric deleted from
   the teacher end on principle. A weekly report whose every number comes from the
   rule layer.
8. **It was measured before it shipped.** Ten personas, real flagship calls,
   per-call token/time/success stats, ref-drop rates, and a demonstrated L1→L3
   spread — with the honest caveat recorded that templated seed drafts score lower
   than real work would.
