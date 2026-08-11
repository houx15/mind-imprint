# Revision-Checkpoint Recording — Design Spec

**Date:** 2026-08-11
**Status:** approved for planning
**Author:** houyuxin (with Claude)

## 1. Motivation

The evaluation model (双轴 Depth × Autonomy + process lenses) needs to read a
student's **revision trajectory**: did they modify their work, how much, did they
do a big structural refactor, and — the highest-value signal — **what did they
change after the AI gave feedback.**

Today that signal is largely unrecoverable. The structural artifacts **overwrite**
and emit no modification record:

- `putSnippets` → `DELETE FROM snippet … INSERT` (every guided-writing revision lost)
- `PUT /outline` → replaces the whole outline (no refactor history)
- `SetReferenceTriage` / `UpdateReference` → overwrite the row (a red→yellow change
  or a deletion leaves no trace)
- only `draft_snapshot` versions anything (the body), and only at explicit commit

So we cannot answer "modified or not / how / after feedback?" — dimensions the rubric
explicitly weights (modification process, self-optimization, decisions on deletions).

**This slice must land before generating the persona-walk evaluation data**, so those
walks actually produce revision data rather than latest-state only.

## 2. Approach (decided)

Not per-mutation events (noisy, hard to attribute — every autosave fires one). Not
full version-control (too much data). Instead: **snapshot the artifact only at
semantically meaningful moments, and let the evaluator diff consecutive checkpoints.**

The two meaningful moments:

1. **ask_feedback** — the student asks the AI to check their work.
2. **finish / advance** — the student finishes a doc or advances a stage.

The diff between two consecutive checkpoints of the same artifact **is** the
modification signal. Because an `ask_feedback` checkpoint is the "before" and the next
checkpoint is the "after", **"what changed after the AI's feedback"** falls out as a
single diff, with the feedback itself linked.

This is bounded version-control (a handful of checkpoints per project, not every
save), generalizing the pattern the body draft already uses (`draft_snapshot`).

## 3. Non-goals (YAGNI)

- **No** per-keystroke / per-autosave capture.
- **No** diff computation in this slice — checkpoints store artifact *state*; the
  evaluator computes diffs and metrics later.
- **No** evaluation logic here — this slice only **records** and **exposes** the data.
- **No** separate "draft touched" frequency tick. Polish frequency is already
  derivable from the count of `ask_feedback` checkpoints + existing `draft_snapshot`
  commit count. Can be added in one line later if per-save granularity is needed.

## 4. Triggers

An artifact checkpoint is written at these existing endpoints (best-effort, never
blocking the primary action):

**ask_feedback**
| Endpoint | Artifact(s) snapshotted |
|---|---|
| `POST /snapshots/{sid}/review` (整稿体检) | draft (ref the snapshot `sid`) |
| `POST /essay-statement/review` (暂定论点检查) | claim, outline |
| `POST /exploration/review` (探索检查) | sources |
| `POST /evidence-map/subquestions/{sqId}/review` (饱和度) | sources |
| `POST /coach` with `scope="proposal_review"` | proposal |

**finish / advance**
| Endpoint | Artifact(s) snapshotted |
|---|---|
| `POST /finish-writing?doc=proposal` | proposal |
| `POST /finish-writing?doc=essay` | draft, outline, snippets |
| `POST /finish` | draft, outline, snippets, sources, proposal, claim (all) |
| `POST /coach/advance` + stage advances | current-stage artifact(s) |

`feedback_ref` is set only for `ask_feedback` triggers, pointing at the review/coach
event that produced the AI feedback, so a checkpoint pair can be read as
(what they submitted → the feedback → what they changed).

## 5. Storage

One new table (`goose` migration):

```
revision_checkpoint (
  id            uuid primary key default gen_random_uuid(),
  project_id    uuid not null references project(id) on delete cascade,
  artifact_type text not null,   -- draft | outline | snippets | sources | proposal | claim
  trigger       text not null,   -- ask_feedback | finish | advance
  content       jsonb not null,  -- artifact state snapshot; for draft = {"snapshotId": "..."} ref
  feedback_ref  uuid null,       -- event/review id that gave the feedback (ask_feedback only)
  created_at    timestamptz not null default now()
)
-- index on (project_id, artifact_type, created_at)
```

`content` holds the full artifact state as JSON (small: outline nodes / snippet
sections / source rows / proposal fields / claim text). For `artifact_type = draft`,
`content` is `{"snapshotId": <draft_snapshot id>}` — no body duplication, reuse the
existing snapshot.

A marker also lands in the **existing event stream** so checkpoints sit on the same
timeline the evaluator already reads:

```
event { type: "revision_checkpoint",
        payload: { artifactType, trigger, checkpointId, feedbackRef } }
```

## 6. Contracts (`packages/contracts`)

Add a Zod schema for the checkpoint `content` shapes and the event-marker inner
payload. Go validates only the outer envelope (`artifact_type` in enum, `trigger` in
enum, `content` is an object, `feedback_ref` a uuid-or-null); the inner deep structure
truth stays in `packages/contracts`, per the architecture rule.

## 7. Read path

`GET /projects/{id}/revision-checkpoints` → checkpoints for the project ordered by
`created_at`, grouped by `artifact_type`. Read-only; no model call. The evaluator (a
later slice) diffs consecutive rows per `artifact_type` to derive: changed?, change
size, structural-refactor flag, and post-feedback change (diff across a `feedback_ref`
pair).

## 8. Error handling

Checkpoint writes are **best-effort side effects**: a failure to write a checkpoint
must never fail or block the primary action (review/finish/advance). Log and continue,
same as the existing `AppendEvent` best-effort pattern.

## 9. Behavior-spec consistency

Before implementation, cross-check the trigger list against
`docs/2026-08-09-all-statuses.md` (the writing-flow single source of truth) to confirm
each named endpoint is a real state action in the current machine and that snapshotting
at it does not alter any status transition. Checkpoints are passive records — they must
not change any existing flow behavior.

## 10. Testing

- Migration up/down.
- Each trigger writes exactly one checkpoint per relevant artifact with correct
  `trigger`, `artifact_type`, and (for ask_feedback) a non-null `feedback_ref`.
- `draft` checkpoints store a `snapshotId` ref, not duplicated body text.
- A checkpoint-write failure does not fail the primary action (inject a store error;
  assert the review/finish still returns 2xx).
- `GET /revision-checkpoints` returns rows ordered and grouped as specified.
- Contract round-trip: content shapes validate against the Zod schemas.

## 11. Out of scope / future

- Evaluator diff + metric computation (next slice).
- Per-save polish-frequency tick (add only if needed).
- UI surfacing of revision history (records first; any student-facing view later).
