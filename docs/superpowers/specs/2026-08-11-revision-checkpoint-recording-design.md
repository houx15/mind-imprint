# Revision-Recording — Design Spec

**Date:** 2026-08-11
**Status:** approved for planning
**Author:** houyuxin (with Claude)

## 1. Motivation

The evaluation model (双轴 Depth × Autonomy + process lenses) needs to read a
student's **revision trajectory**: did they modify their work, how much, did they do a
big structural refactor, what did they change **after the AI gave feedback**, and how
actively did they **explore and prune** their question graph and sources.

Today that signal is largely unrecoverable, because the artifacts **overwrite** and emit
no modification record:

- `putSnippets` → `DELETE FROM snippet … INSERT` (every guided-writing revision lost)
- `PUT /outline` → replaces the whole outline (no refactor history)
- `SetReferenceTriage` / `UpdateReference` → overwrite the row (a red→yellow change or a
  deletion leaves no trace)
- exploration `lead` / `edge` create & delete → no event emitted (add/remove of a
  question node is not recorded)
- only `draft_snapshot` versions anything (the body), and only at explicit commit

So we cannot answer "modified or not / how / after feedback?" or "did they actively add
and prune exploration nodes and sources?" — dimensions the rubric explicitly weights.

**This slice must land before generating the persona-walk evaluation data**, so those
walks produce real revision/exploration data rather than latest-state only.

## 2. Approach — two mechanisms

Different artifacts need different recording, because they change in different ways.

**Mechanism 1 — checkpoint snapshots (content).** For the **text artifacts edited in
place** (writing draft, outline, snippets, proposal, claim). Recording every keystroke
would be noisy and unattributable; full version-control every save is too much data.
Instead we snapshot the artifact's **full content** only at **meaningful moments** — when
the student **asks for feedback** and when they **finish / advance** — and let the
evaluator **diff consecutive checkpoints**. The diff *is* the modification signal; and
because an ask-feedback checkpoint is the "before" and the next is the "after,"
**"what changed after the AI's feedback"** falls out as one diff with the feedback
linked. This generalizes the pattern the body already uses (`draft_snapshot`).

**Mechanism 2 — mutation events (append-only).** For the **exploration graph and source
decisions**, where each action is **discrete, deliberate, and low-frequency** — adding a
question node, removing one, drawing an edge, adopting a dug paper, dropping or
re-triaging a source. Here the *action itself* is the signal (proactive exploration,
pruning, deletions), so we emit **one typed event per action** into the existing event
stream, carrying the minimal before→after. These reconstruct the full explore-and-prune
and sourcing trajectory. (This is not the "noisy autosave" case the checkpoint approach
avoids — these are deliberate single acts.)

## 3. Non-goals (YAGNI)

- **No** per-keystroke / per-autosave capture of writing.
- **No** diff/metric computation in this slice — checkpoints store state; the evaluator
  computes diffs and metrics later.
- **No** evaluation logic here — this slice only **records** and **exposes** the data.
- **No** separate "draft touched" frequency tick. Polish frequency is derivable from the
  count of ask_feedback checkpoints + existing `draft_snapshot` commit count.
- **No** UI surfacing of history yet (records first; any student/teacher view is later).

## 4. Mechanism 1 — checkpoint triggers

A checkpoint captures the artifact's full content at these existing endpoints
(best-effort, never blocking the primary action). Artifact types:
**`draft | outline | snippets | proposal | claim`**.

**ask_feedback**
| Trigger | Artifact(s) snapshotted |
|---|---|
| `POST /snapshots/{sid}/review` (整稿体检) | draft (ref the snapshot `sid`) |
| `POST /essay-statement/review` (暂定论点检查) | claim, outline |
| `POST /cards/reflect` (反思一张已填的卡) | snippets, claim |
| `POST /coach` `scope="proposal_review"` | proposal |
| `POST /coach` `scope="writing"` (guided-writing / ask 印记 to comment on a claim) | snippets, draft, outline |

The coach-mediated writing case is the highest-value one: the student writes a claim as
a snippet and asks 印记 to comment (without rewriting), then revises — the per-claim
"what changed after feedback" loop. The coach request carries only `{user_input, scope}`
(the artifact in context comes from `studio_state`), so we key off **scope**. Every
`writing`-scope coach exchange is a checkpoint; `feedback_ref` = that coach reply event.
Forming-scope chat is not checkpointed (nothing under revision yet) except
`proposal_review`. Bounded: a walk has ~10–15 writing turns, not thousands of autosaves.

**finish / advance**
| Trigger | Artifact(s) snapshotted |
|---|---|
| `POST /finish-writing?doc=proposal` | proposal |
| `POST /finish-writing?doc=essay` | draft, outline, snippets |
| `POST /finish` | draft, outline, snippets, proposal, claim (all) |
| `POST /coach/advance` + stage advances | current-stage text artifact(s) |

Checkpoints are **always written** (no dedup); each carries a `content_hash` so the
evaluator reads **unchanged** consecutive hashes as "asked for feedback / finished but
did **not** revise" (itself a signal — e.g., ignored the AI's comment) and diffs
`content` when the hash changed.

## 5. Mechanism 2 — exploration & source mutation events

One typed event per deliberate action, appended to the **existing** event stream (no new
table). Emitted at the respective mutation endpoints, best-effort:

| Action / endpoint | Event type | Payload (minimal) |
|---|---|---|
| `POST /exploration/leads` | `lead_added` | leadId, text, origin (manual/adopted) |
| `DELETE /exploration/leads/{lid}` | `lead_removed` | leadId, text |
| `POST /exploration/edges` / `.../edges/propose` accept | `edge_added` | from, to, origin |
| `DELETE /exploration/edges/{eid}` | `edge_removed` | from, to |
| `POST /exploration/adopt` | `lead_adopted` | leadId, parentLeadId, source |
| `POST /exploration/attach` | `source_attached` | referenceId, parentLeadId |
| `POST /references` | `source_added` | referenceId, title, classification |
| `DELETE /references/{rid}` · `.../archive` | `source_dropped` | referenceId, title |
| `PATCH /references/{rid}/triage` · `.../evidence` · decision change | `source_reclassified` | referenceId, field, before→after |

The evaluator reads these in timeline order to reconstruct: proactive node/source
additions, pruning/deletions, and every triage/decision change with its before→after.

Where any of these endpoints already emits a related event today, the plan extends the
existing emission rather than duplicating it (verified during planning).

## 6. Storage

One new table (`goose` migration) for Mechanism 1; Mechanism 2 reuses the `event` table.

```
revision_checkpoint (
  id            uuid primary key default gen_random_uuid(),
  project_id    uuid not null references project(id) on delete cascade,
  artifact_type text not null,   -- draft | outline | snippets | proposal | claim
  trigger       text not null,   -- ask_feedback | finish | advance
  content       jsonb not null,  -- full artifact state; for draft = {"snapshotId": "..."} ref
  content_hash  text not null,   -- for unchanged-vs-changed detection at read time
  feedback_ref  uuid null,       -- coach/review event id that gave the feedback (ask_feedback only)
  created_at    timestamptz not null default now()
)
-- index on (project_id, artifact_type, created_at)
```

`content` holds the full artifact state as JSON (small: outline nodes / snippet sections
/ proposal fields / claim text). For `artifact_type = draft`, `content` is
`{"snapshotId": <draft_snapshot id>}` — no body duplication, reuse the existing snapshot.

## 7. Contracts (`packages/contracts`)

Add Zod schemas for (a) the checkpoint `content` shapes per artifact_type and (b) the
Mechanism-2 event payload shapes. Go validates only the outer envelope (`artifact_type`
and `trigger` in enum, `content` an object, `feedback_ref` uuid-or-null; event `type` in
the new enum, `payload` an object). Inner deep-structure truth stays in
`packages/contracts`, per the architecture rule.

## 8. Read path (records only)

`GET /projects/{id}/revision-checkpoints` → checkpoints ordered by `created_at`, grouped
by `artifact_type`. Mechanism-2 events are already readable via the existing event/log
read path. Both read-only; no model call. The evaluator (a later slice) diffs
consecutive checkpoints per `artifact_type` and folds the mutation events into the
exploration/source trajectory.

## 9. Error handling

Both mechanisms are **best-effort side effects**: failing to write a checkpoint or emit
an event must never fail or block the primary action (review / finish / advance / graph
mutation). Log and continue — same pattern as the existing `AppendEvent`.

## 10. Behavior-spec consistency

Before implementation, cross-check every named trigger/endpoint against
`docs/2026-08-09-all-statuses.md` (the writing-flow single source of truth): confirm each
is a real action in the current state machine and that recording at it does **not** alter
any status transition. Records are passive — they must not change existing flow behavior.

## 11. Testing

- Migration up/down.
- Each Mechanism-1 trigger writes exactly one checkpoint per relevant artifact with the
  correct `trigger`, `artifact_type`, `content_hash`, and (for ask_feedback) a non-null
  `feedback_ref`.
- `draft` checkpoints store a `snapshotId` ref, not duplicated body text.
- Unchanged content across two feedback checkpoints yields equal `content_hash`.
- Each Mechanism-2 endpoint emits exactly one event of the right type with correct
  before→after payload (add / remove / adopt / attach / reclassify / drop).
- A recording failure does not fail the primary action (inject a store error; assert the
  review / finish / graph mutation still returns 2xx).
- `GET /revision-checkpoints` returns rows ordered and grouped as specified.
- Contract round-trip: content and event payload shapes validate against the Zod schemas.

## 12. Out of scope / future

- Evaluator diff + metric computation (next slice).
- Per-save polish-frequency tick.
- UI surfacing of revision / exploration history.
