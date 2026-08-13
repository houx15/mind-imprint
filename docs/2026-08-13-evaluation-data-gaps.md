# Evaluation Data Gaps — pre-work for report generation

> **Purpose:** the `EvaluationReport` v1 structure (`docs/superpowers/specs/2026-08-13-evaluation-report-structure-design.md`) is fully backed by the live schema **except** the six items below. These are **pre-work for the generation phase** — after the evaluation *design* (structure + looking) is settled, we build these so the generation algorithm can fill every field faithfully.
>
> **Reference:** field→table map in the structure spec's Appendix A; storage truth in `docs/2026-08-11-evaluation-data-storage-guide.md`; code map in `docs/2026-08-11-evaluation-and-teacher-code-map.md`.
>
> Each gap is tagged **Gn** (matching the structure spec) with: the problem, the decision (from 2026-08-13 review), and the concrete work.

---

## G1 — `frameworkFinished` milestone has no timestamp

**Problem.** `proposalFinished` / `writingFinished` / `projectFinished` all have clean timestamps (`writing_finish`, `evaluations`). "立题完成" does not — it's not a persisted moment.

**Decision.** `frameworkFinished` = **the moment the student finishes 立题 and clicks「生成计划」(`generate_plan`)**. Persist that timestamp.

**Work.**
- On the generate-plan action (server handler that produces `plan_item` rows), record a milestone timestamp. Options, pick one at build time:
  - (a) a dedicated `event` row `type='milestone:framework_finished'` (cheapest, no migration), **or**
  - (b) a `framework_finished_at timestamptz` column on `project` (queryable directly).
- Recommendation: **(a)** — milestones are read once at report time; an `event` row keeps `project` clean and matches the existing instrumentation stream.
- Backfill for the seeded persona projects: derive from `MIN(plan_item.created_at)` (or the first `proposal_writing`-stage `chat_message`) and write the milestone event, so existing eval data has the field.
- Generator reads it as `milestones.frameworkFinished`.

---

## G2 — AI writing comments (批注) + comment-open are not stored

**Problem.** `counters.aiCommentCount` and any "did the student engage with AI feedback" signal need the AI's writing comments (the「让印记通读并批注」output) and the student's click/open of each comment. Neither is durably persisted per-item today.

**Decision.** **Add storage** for (a) each AI writing comment, and (b) each time the student opens/clicks a comment.

**Work.**
- New table `writing_comment`:
  `id, project_id, doc_kind (proposal|essay), anchor (span/quoted text or offset), body, created_at, opened_at (nullable)`.
  - `anchor` mirrors how `chat_message.quoted_fragment` anchors a line, so a comment can scroll-to / highlight the span.
- Write path: the 通读批注 review pass persists one row per comment (currently the annotations may only live in transient response / `intervention`; make them durable here).
- Open signal: set `opened_at` (or append an `event` `type='writing_comment_opened'`) when the student clicks a comment.
- Feeds: `counters.aiCommentCount` = `COUNT(writing_comment)`; an engagement signal (`opened / total`) available to the evaluator for D5 (反馈处理与修订) and the risk/abstract prose.
- Verify against `docs/2026-08-09-all-statuses.md` (the 通读批注 flow) before building.

---

## G3 — `materials[].usedIn` → essay claim is weakly linked

**Problem.** Source ↔ *exploration question* is stored (`exploration_lead.connected_reference_id`), but source ↔ *essay claim/section* is not. So "this source was used to support claim X in the essay" can't be answered precisely.

**Decision (how we solve it).** Two-tier:
- **v1 (no new work):** `usedIn` = the **exploration question node** the source hangs under (`node:*`), read from `exploration_lead`. This already answers "used to support question X" faithfully.
- **Claim-level (the real fix):** capture a **citation link at material-insert time.** The writing surface already inserts materials at the caret ("materials insert-at-caret"). When a student inserts a source into a section, record the link.

**Work (claim-level).**
- New table `snippet_reference` (or `citation`): `id, project_id, reference_id, section (claim:<id> / subq:<id> / prop:<step>), created_at`.
- Populate it from the existing material-insert action in the writing room (piggyback, no new UI, respects the light-writing 铁律).
- Generator then resolves `usedIn` to `claim:*` when a link exists, else falls back to `node:*`.
- Note: current seeded walk data has empty per-claim `snippet` rows, so claim-level `usedIn` won't demo from existing data until a walk drives guided per-claim writing.

---

## G4 — subagent coverage for `promptLens` / `toolUsage`

**Problem.** "Subagent prompts/usage" is ambiguous; not every internal subagent has student-authored input worth surfacing.

**Decision.** `promptLens` and `toolUsage` cover **only three student-facing subagents**: **review agent**, **card agent**, **reading-room subagent**. Exclude internal ones (search/dig, placement, etc.).

**Work.**
- Identify the three by `llm_call.purpose` (and/or `surface`) so the generator can filter to exactly these.
- Ensure each one's **student-facing input** is retrievable for a `promptLens` entry:
  - card agent → `card_instances.field_values` / `event_trace` (what the student filled),
  - reading-room subagent → the student's reading selection / question (reading loop input),
  - review agent → the doc/part the student sent for review (review target).
- If any of the three does not persist its student-facing input today, add minimal storage (small; likely only the review-agent input needs a durable anchor).
- `toolUsage` for these three = one entry each with `name/stage/purpose` (FACT) + `summary` (PROSE).

---

## G5 — evidence-id reliability (constrained selection)

**Problem.** `depth[].evidence` / `autonomy[].evidence` / `promptLens` / event & risk refs need **real** `chat_message` / `event` / `material` ids. A free-writing model will hallucinate ids, producing dead deep-links.

**Decision.** **Yes — build constrained id selection.** The generator selects evidence ids from a provided, id-indexed candidate list; it never invents ids.

**Work.**
- Build an **evidence-candidate index** per project: a compact, id-indexed list of citable records — `chat_message` (id, role, stage, short label), key `event`/`activity_log_entry` rows, `reference`, `card_instances`, `exploration_lead` — each with a stable `id` and a short label.
- Feed that index into the generation prompt; instruct the model to cite **only** ids from it.
- **Post-validation** (server, at store time): drop or repair any `Ref.id` not present in the candidate index; keep the `label` so the UI still renders. Log the drop rate as a quality signal.
- This is generator-side (colleague's), but the candidate-index builder + the post-validation are shared infra we should provide alongside the read/write seam.

---

## G6 — `wordsWritten` counting rule

**Problem.** "Words" is ambiguous for mixed Chinese/English body text (a paragraph of Chinese has ~0 whitespace-delimited words).

**Decision.** **Yes — define one rule.**

**Work.**
- `countWords(text)` = **(number of CJK codepoints) + (number of non-CJK whitespace-delimited tokens)**.
  - CJK range covers Han ideographs (and CJK punctuation excluded); each Han char counts as 1.
  - Non-CJK runs are split on whitespace; each token counts as 1.
- Implement once in Go (server-side, since `counters` is computed there), unit-tested on a mixed zh/en sample; reuse anywhere a body-text length is shown.
- Apply to `counters.wordsWritten` over the finished body text (`draft_snapshot` latest / `edit_buffer`).

---

## Sequencing

These are **pre-work for generation**, not blockers for the structure + looking design. Order once we start:

1. **G6** (pure function) and **G1** (one event write + backfill) — trivial, do first.
2. **G5** (candidate index + post-validation) — needed before any real evidence-bearing generation.
3. **G2** (writing-comment storage) and **G4** (subagent input anchors) — small schema/store additions.
4. **G3** claim-level citation link — last / optional; v1 ships with `node:*`.
