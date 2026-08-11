# Evaluation Data Storage Guide — Where One Student's Whole Journey Lives

> **Audience:** an engineer picking up the **process-evaluation (过程评估)** work. This document maps, table by table and column by column, where a student's **every interaction with the AI**, every **choice**, every card use, every source judgment, every draft, and every review outcome is stored — and how to pull one student's entire journey for a single task by `project_id`.
>
> Column names come from the **live production schema** (`information_schema`), not the older architecture docs. Where a table is written or read by specific code, the relevant file is named inline so you can jump straight to it. A companion doc, **`2026-08-11-evaluation-and-teacher-code-map.md`**, maps the *code* (endpoints, agents, sqlc, migrations); this doc is about the *data*.

**One-sentence mental model:** everything centers on **`project`** (one task by one student). Identity/org lives on the `users`/`classes` side; the **AI conversation** for the task lives in `chat_thread`/`chat_message`; **structured choices and artifacts** are spread across a handful of purpose-specific tables (`project_proposal`, `plan_item`, `exploration_lead`, `reference`, `snippet`, `evaluations`, …); **process signals and billing** live in `event` / `activity_log_entry` / `llm_call`. Almost every table carries a `project_id` — that is the key you use to pull all of a student's data for one task.

---

## 0. Identity & organization (who is doing the work)

| Table | Key columns | What it holds |
|---|---|---|
| `users` | `id, email, role, school_id, display_name, card_theme` | Accounts. `role` ∈ student/teacher/admin. Every account must belong to a school. |
| `schools` | `id, name` | Schools. |
| `classes` | `id, school_id, name, join_code` | Classes + the join code used at signup. |
| `enrollments` | `user_id, class_id, role_in_class` | User ↔ class membership (`role_in_class` ∈ student/teacher). |

Join here when you need to aggregate evaluation by person / class / school. **The process data for one specific task is not here** — it is in the tables below, linked back to the person via `project.user_id`.

---

## 1. The task itself: `project`

| Column | Meaning |
|---|---|
| `id` | Project primary key — **the foreign key on every process table below**. |
| `user_id` | The owning student. |
| `qualification` | Subject / project type (EE / TOK / AP …). Drives the rubric and the AI's role. |
| `title` | The research topic (in the current architecture, the fallback source for the driving question). |
| `status` | Lifecycle state (`active` / `evaluating` / `finished` / …). |
| `studio_state` | **jsonb** — the orchestrator's runtime state (current stage, proposal track, resource needs, `started`, …). **This is the live snapshot of "where the student is and what 印记 is walking them through."** |
| `deadline, created_at, last_active_at, cover` | Metadata. |

> `studio_state` is the key to reconstructing a session's *orchestration trajectory*: it records the flow status / studio stage (`topic_discussion` → `proposal_forming` → `plan_generation` → `proposal_writing`/`review` → `body_writing` → `retrospective`). Read it when you need to see how a student moved through, and dwelt in, each stage. Its shape is validated by the `studioState` / `orchestrator` Zod contracts in `packages/contracts/src`.

---

## 2. Student ↔ AI conversation (**the primary table of AI-interaction history**)

The single continuous "印记" conversation across a task (spanning the proposal / reading / writing / review rooms) is exactly the "detailed interaction" evaluation most wants.

| Table | Key columns | Notes |
|---|---|---|
| `chat_thread` | `id, user_id, seeded_project_id, title` | One 印记 thread. `seeded_project_id` links it to the project. |
| `chat_message` | `id, thread_id, role, content, surface, stage, modality, attachments, quoted_fragment, folded_at, created_at` | **One row per turn.** |

The `chat_message` columns *are* the interaction detail:

- `role` — user / assistant (who spoke).
- `content` — the message body (what the student asked, what 印记 answered).
- `surface` — **which room/UI** the turn happened in (proposal / reading / writing / review …), so you can tell "asked the AI in the writing room" from "asked in the reading room."
- `stage` — the studio stage at the time (added in migration `0038_continuous_session.sql` / `chat_message.stage`), pinning each turn to a flow stage.
- `quoted_fragment` — **which sentence of the body text** the student quoted to ask about ("ask 印记 about this line" in the writing room).
- `folded_at` — whether the turn was compacted (long-conversation compaction). The compacted summary lives in `conversation_digest.prose`.

> **"Where did the student ask the AI for help?"** = group `chat_message` by `role='user'` + `surface`/`stage`: how many times they asked in the reading room, which lines they asked about while writing, what they asked in review.

**Legacy:** `messages` (keyed by `task_id`, the old task surface) is frozen — no new writes. Old rows remain and the `llm_usage` view UNIONs them for cost roll-ups. All new conversation goes to `chat_message`.

---

## 3. Proposal stage: proposal + plan + outline

| Table | Key columns | What it holds |
|---|---|---|
| `project_proposal` | `project_id, objective, reason, activities, resources, counterpoints` | The structured four-part research proposal + counterexamples/tension (`counterpoints`). **One row per project**, updated in place. |
| `plan_item` | `project_id, title, tag, col, stage, ref_material_id, start_day, days, position` | Each item of the research **plan** (kanban-style). Derived automatically by the system from the framework / research question — a deterministic step, no student-confirmation gate. |
| `outline_node` | `project_id, text, depth, position` | The essay **outline** tree (initialized from the main question + 2–4 sub-questions; the student edits it against their materials). `depth` projects the hierarchy. |
| `snippet` (section `prop:*`) | see §6 | Body-text fragments for each proposal part (during guided writing). |

---

## 4. Reading & exploration (the 兔子洞地图 + sources)

This is the whole record of "how the student finds, reads, and judges sources."

| Table | Key columns | What it holds |
|---|---|---|
| `exploration_lead` | `project_id, text, status, origin, parent_lead_id, connected_reference_id, source_reference_id, position` | **Nodes of the rabbit-hole graph.** `connected_reference_id=NULL` → a question node (`parent_lead_id=NULL` is the main question, non-null is a sub-question); `connected_reference_id` set → a paper node hanging under a question. `origin` ∈ takeaway/manual/guide/note (how this question/lead came to exist). `status` ∈ open/connected/pruned. |
| `question_edge` | `project_id, from_lead_id, to_lead_id, label, status` | **Directed, labeled relations between questions** (sub-question / support / rebut-tension / refine / depend). `status` ∈ proposed (印记 suggested, not confirmed) / confirmed (student confirmed or drew it themselves). **The "印记 proposes → student confirms" choice trail lives here.** |
| `reference` | `project_id, title, author, year, journal, url, abstract, classification, credibility, decision, triage, evidence_*, reading_status, material_id, archived, tags` | **The full record of every source.** `decision` ∈ use/maybe/drop (the student's keep/drop judgment); `triage`/`credibility`/`evidence_nature`/`evidence_argument`/`evidence_finding`/`evidence_placement` (added in `0062`) = the student's annotation of a source's **evidentiary function**; `reading_status` ∈ to_read/reading/done; a non-null `material_id` = actually read (engaged). |
| `material` | `project_id, kind, source, title, source_url, blocks, scratch, thread_id` | The **imported body/material of one source** (chunked into `blocks`), the substrate for sentence-by-sentence co-reading in the reading room. |
| `collection` | `project_id, name, parent_id, position` | Source folders/collections (a *separate* dimension from graph placement / "未归类"). |
| `reading_note` | `project_id, …` (added in `0043`) | The student's **own notes** written in the reading room. |
| `source_log_entry` | `project_id, url, title, time_spent_s, takeaway, tier, lateral_read, opened_at, material_id` | **The source log:** which link was opened, how long they stayed, whether they lateral-read (`lateral_read`), a one-line takeaway, credibility tier (`tier`). A goldmine for evaluating search/verification behavior. |

> **Source placement (new, 2026-08-11):** a self-added source that is not hung under any question falls into the "未归类" (unfiled) node (= a `reference` row that exists but has no non-pruned `connected_reference_id` lead). Placing it = creating a connecting lead with `origin='manual'`. 印记's placement suggestion is one `llm_call` with `purpose='suggest_placement'`. See `apps/api/internal/api/placement.go` + `agent/placement.go`.

---

## 5. Card usage (思维工具卡)

| Table | Key columns | What it holds |
|---|---|---|
| `card_instances` | `project_id, card_id, status, field_values, event_trace, rubric_tags, anchors, framework_fill, thread_id, created_at, completed_at` | **Every card summon and answer.** `card_id` = which card; `status` = lifecycle (proposed/in-progress/done/skipped); `field_values` = what the student filled in (jsonb); `event_trace` = the sequence of in-card interaction events (**the step-by-step record of "how the student used this card"**); `rubric_tags` = evaluation markers. **A student who skips a card and lets the AI answer directly still leaves a row** (friction is signal). |
| `card_competence` | `user_id, card_id, scaffold_state, unprompted_count, prompted_count` | **Cross-project card proficiency:** how many times a card was used unprompted vs. prompted, and its scaffold state. The basis of the card-gallery star rating. |
| `intervention` | `project_id, card_instance_id, type, anchor, criterion, body, level, output_check_verdict` | One **AI intervention/follow-up** triggered by a card (attached to a `card_instance`), including the restraint-ladder `level` and the output-check verdict. |
| `disposition` | `intervention_id, action, reason` | The student's **response** to that intervention (adopt / ignore / …) + reason. **The finest-grained "AI proposes, how the student responds" choice trail.** |

The envelope shape (`status` enum, `id`, `field_values` object, `event_trace` array) is validated at the Go boundary; the deep interior shape is owned by the Zod contracts in `packages/contracts` (see the boundary-validation note at the end).

---

## 6. Writing

| Table | Key columns | What it holds |
|---|---|---|
| `snippet` | `project_id, text, section, position` | **Body fragments, by section / by claim.** `section` keys: `prop:<step>` (proposal parts), `claim:<id>` / `subq:<id>` (essay claims / sub-questions). Guided writing writes each part as one sectioned snippet. |
| `edit_buffer` | `project_id, content, doc_kind` | The **whole-document buffer** currently being edited (`doc_kind` ∈ proposal / essay — multi-doc since `0061`). Autosave writes here. |
| `draft_snapshot` | `project_id, seq, content, span_index, doc_kind` | The **version-snapshot sequence** of the body (`seq` increments) — the writing-evolution trail. |
| `writing_finish` | `project_id, doc_kind, finished_at` | The moment a document was "finished" (proposal done / essay done). One of the milestones that triggers process evaluation. |

> Design law: the body text lives on the student's side and the AI never ghostwrites it. These tables store **text the student wrote** + versions, not AI-generated output.

---

## 7. Review & evaluation (the rubric cut)

> **Model note:** the canonical scoring model here is the **dual-axis (双轴) model** — depth **D1–D6** (anchored L1–L4) × autonomy **A1–A6** (event-counted, band 0–5) + **6 process lenses** — defined in `apps/api/internal/rubric/dualaxis.{go,json}` and mirrored in `packages/contracts/src/rubric.ts`. The two axes are **never combined into a single total score**.

| Table | Key columns | What it holds |
|---|---|---|
| `project_reflection` | `project_id, answers, done` | The student's **retrospective answers** (structured `answers`). Written by `submitReflection` / `putReflection`. |
| `evaluations` | `project_id, scores, narrative, rubric, signals, leaps, model, tier, trigger, trigger_milestone, status, prompt_tokens, completion_tokens, cost_estimate, session_id, thread_id` | **The formal product of process evaluation.** `scores` = rubric levels; `narrative` = the process narrative; `signals`/`leaps` = extracted process signals and "thinking leaps"; `trigger`/`trigger_milestone` = which milestone fired it; `status` models queued/running/done/failed (async columns from `0007`/`0008`, with a one-in-flight-per-task partial-unique index). Evaluation runs on the **flagship tier — never downgraded** — and its token usage is recorded on this same row. |
| `project_mirror_prose` | `project_id, sections, carry_forwards` | The "你的思维印记" mirror narrative (sectioned + carry-forwards for next time). Composed by `composeAndStoreProjectMirror`. |
| `project_summary_prose` | `project_id, prose` | Project-level summary prose (summary-on-return). |
| `parent_report_prose` | `student_user_id, surface, scope_id, prose` | Projection narrative for parents / other audiences. |
| `class_weekly_prose` | (class dimension) | Class weekly-report prose (teacher-end aggregation). |

> Pattern throughout evaluation: **live numbers computed on the fly + LLM prose written once and stored (first-open-wins).** GET endpoints never spend tokens; only a POST produces an `llm_call`. This is the read-no-call / write-only-spend contract — see the code map for the exact endpoints.

---

## 8. Process signals & billing (running through the whole journey)

| Table | Key columns | What it holds |
|---|---|---|
| `event` | `project_id, user_id, surface, type, payload, session_id, thread_id, course_id` | **The general instrumentation event stream:** what `type` of thing happened on which `surface` (payload jsonb). Use it to rebuild the behavior timeline. |
| `activity_log_entry` | `project_id, entry_date, text, source` | The human-readable **activity log** ("opened the four framing questions / added a source / 印记 reviewed the full draft / finished the essay body" …). The timeline shown on the review page. |
| `conversation_digest` | `project_id, prose, turns_folded, model, tier` | The **compacted summary** of a long conversation (the condensed form of the turns folded away by `chat_message.folded_at`). |
| `llm_call` | `project_id, user_id, surface, purpose, provider, model, tier, prompt_tokens, completion_tokens, cost_estimate` | **One row per real model call** (coaching / anchor / course rendering / suggest_placement / evaluation …). `purpose` distinguishes the use; `tier` the model class. The foundation of cost aggregation. Written via `RecordLLMCall`/`RecordChatLLMCall` (`agent/agentstore.go`). |
| `project_ai_use` | `project_id, used_for, not_used_for` | The student's **AI-use declaration** (what I used AI for, what I didn't) — an academic-integrity signal. |

The `llm_usage` view is a three-way UNION (`llm_call` + the frozen `message`/`evaluation` legacy rows) for organization-level cost roll-ups.

---

## 9. Reverse lookup (an evaluation question → which table)

| To find out… | Go to |
|---|---|
| **Where & about what the student asked the AI** | `chat_message` where `role='user'` — look at `surface`/`stage`/`quoted_fragment` |
| **What the AI said, and how restrained it was** | `chat_message` where `role='assistant'`; in-card interventions in `intervention.level` |
| **The student's keep/drop & evidence judgments** | `reference.decision` / `triage` / `evidence_*` / `credibility` |
| **How the student searched & whether they verified** | `source_log_entry` (dwell, `lateral_read`, `tier`) + `exploration_lead.origin` |
| **The exploration structure** (question ↔ sub-question ↔ paper ↔ relation) | `exploration_lead` + `question_edge` |
| **How cards were used, and whether skipped** | `card_instances.status`/`field_values`/`event_trace` + `card_competence` |
| **How the student chose after an AI proposal** | `disposition` (card interventions), `question_edge.status` (relation proposals), `reference.decision` |
| **Writing evolution** | `snippet` (by section) → `draft_snapshot` (versions) → `writing_finish` (done) |
| **The formal evaluation conclusion** | `evaluations` + `project_mirror_prose` |
| **AI usage / cost / integrity** | `llm_call` + `project_ai_use` |
| **The behavior timeline** | `event` (machine) + `activity_log_entry` (human-readable) |

---

## 10. Pull one project's entire process data (query recipes)

```sql
-- 1) Locate the project
SELECT id, user_id, title, status, studio_state->>'stage' AS stage, created_at
FROM project WHERE user_id = :uid ORDER BY created_at DESC;

-- 2) The full AI conversation (in order, with room and stage)
SELECT m.created_at, m.role, m.surface, m.stage, m.quoted_fragment, m.content
FROM chat_message m
JOIN chat_thread t ON t.id = m.thread_id
WHERE t.seeded_project_id = :pid
ORDER BY m.created_at;

-- 3) Exploration structure (questions + papers + relations)
SELECT id, text, origin, status, parent_lead_id, connected_reference_id
FROM exploration_lead WHERE project_id = :pid ORDER BY position;
SELECT from_lead_id, to_lead_id, label, status FROM question_edge WHERE project_id = :pid;

-- 4) Sources & evidence judgments
SELECT title, decision, triage, credibility, reading_status,
       evidence_nature, evidence_argument, evidence_finding, evidence_placement
FROM reference WHERE project_id = :pid ORDER BY position;

-- 5) Card use + disposition
SELECT ci.card_id, ci.status, ci.field_values, ci.rubric_tags,
       i.type AS intervention, d.action AS disposition
FROM card_instances ci
LEFT JOIN intervention i ON i.card_instance_id = ci.id
LEFT JOIN disposition d ON d.intervention_id = i.id
WHERE ci.project_id = :pid ORDER BY ci.created_at;

-- 6) Writing (by section → versions → finished)
SELECT section, position, length(text) FROM snippet WHERE project_id = :pid ORDER BY position;
SELECT doc_kind, seq, length(content) FROM draft_snapshot WHERE project_id = :pid ORDER BY doc_kind, seq;
SELECT doc_kind, finished_at FROM writing_finish WHERE project_id = :pid;

-- 7) Evaluation + usage
SELECT scores, trigger_milestone, tier, status FROM evaluations WHERE project_id = :pid;
SELECT purpose, surface, tier, prompt_tokens, completion_tokens, cost_estimate
FROM llm_call WHERE project_id = :pid ORDER BY created_at;

-- 8) Behavior timeline
SELECT entry_date, source, text FROM activity_log_entry WHERE project_id = :pid ORDER BY entry_date;
SELECT created_at, surface, type FROM event WHERE project_id = :pid ORDER BY created_at;
```

> **Boundary-validation principle:** Go validates only the outer envelope of the standard structures (`status` enum, ids, `field_values` is an object, `event_trace` is an array); the deep interior shape is owned by the Zod contracts in `packages/contracts`. When reading jsonb columns for evaluation, treat those contracts as the source of truth for shape.

---

## Ready-made evaluation dataset

Ten persona full-walks were run on production to seed this dataset — 10 fully-evaluated projects (`evaluations.status = done`), one per behavioral persona, all on the same EE topic. Each project carries the uniform data described above (≈13 chat turns, 5 cards, 5 exploration leads incl. 2 paper nodes, 2–3 question edges, 3 evidence-tagged references, full draft + snapshots + `writing_finish`, reflection, ai-use, activity log, 23–27 `llm_call`, and a completed evaluation). Query any of them with the recipes above to design evaluation logic against real, persona-differentiated data. (Known gap: per-claim `snippet` rows are empty — the full essay text is in `edit_buffer`/`draft_snapshot`, but guided per-claim writing was not driven in the walk.)
