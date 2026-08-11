# Revision Recording Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Record each student's revision trajectory — checkpoint snapshots of the writing text at feedback/finish moments, and one event per deliberate exploration/source action — so the evaluator can later read *what* they changed and *how they explored/pruned*.

**Architecture:** Two mechanisms. (1) A new `revision_checkpoint` table stores full artifact content at ask-feedback/finish/advance triggers, written by a best-effort `recordCheckpoint` helper. (2) The existing `event` stream gains new typed events emitted at exploration/source mutation handlers. Nothing computes diffs or metrics — this slice records and exposes only.

**Tech Stack:** Go (`net/http`, `pgx/v5`, `sqlc`, `goose`), PostgreSQL, Zod contracts (`packages/contracts`), Vitest.

## Global Constraints

- **Best-effort recording:** a checkpoint-write or event-emit failure MUST never fail or block the primary action (review/finish/advance/mutation). Log via `slog.Warn` and continue — copy the pattern at `apps/api/internal/api/question_card.go:129-135`.
- **Records only:** no diff computation, no metrics, no evaluation logic, no UI. (Spec §3.)
- **Envelope validation split:** Go validates only outer fields (`artifact_type`/`trigger` in enum, `content` an object, `feedback_ref` uuid-or-null; event `type` in enum, `payload` an object). Inner deep-structure truth lives in `packages/contracts` Zod. (Spec §7; AGENTS.md.)
- **Mechanism 2 reuses the existing `event` table** — no new table for events. Only `revision_checkpoint` (Mechanism 1) is new.
- **draft checkpoints store a reference** `{"snapshotId": "<uuid>"}`, never duplicate body text. (Spec §6.)
- **feedback_ref is nullable + best-effort.** Populate only where a feedback id is cheaply available; for coach-scope checkpoints leave it null (the evaluator pairs by project + trigger=ask_feedback + timestamp).
- **Migration numbering:** next file is `0063_revision_checkpoint.sql`; goose `-- +goose Up` / `-- +goose Down`. Run `sqlc generate` from `apps/api/`.
- **Behavior-spec:** triggers map to `docs/2026-08-09-all-statuses.md` statuses 4 (proposal finish, l.97-99), 5 (exploration review, l.125), 6 (statement + order-review + essay finish, l.126-145), 7 (finish→eval, l.157). Recording is passive: it MUST NOT change any status transition.
- **Go tests need Docker/testcontainers.** Copy the harness from an existing api test (e.g. `apps/api/internal/api/question_card_test.go`) — seeded project + auth cookie.

---

### Task 1: `revision_checkpoint` table + sqlc queries

**Files:**
- Create: `apps/api/internal/store/migrations/0063_revision_checkpoint.sql`
- Create: `apps/api/internal/store/queries/revision_checkpoint.sql`
- Generated (via `sqlc generate`): `apps/api/internal/store/sqlc/revision_checkpoint.sql.go`
- Test: `apps/api/internal/api/revision_checkpoint_store_test.go`

**Interfaces:**
- Produces: sqlc `InsertRevisionCheckpoint(ctx, InsertRevisionCheckpointParams{ProjectID uuid.UUID, ArtifactType string, Trigger string, Content []byte, ContentHash string, FeedbackRef pgtype.UUID}) (RevisionCheckpoint, error)` and `ListRevisionCheckpoints(ctx, projectID uuid.UUID) ([]RevisionCheckpoint, error)`.

- [ ] **Step 1: Write the migration**

`apps/api/internal/store/migrations/0063_revision_checkpoint.sql`:
```sql
-- Revision recording (Mechanism 1): full-content snapshots of the writing text
-- artifacts at ask-feedback / finish / advance moments. Diffed by the evaluator
-- later to derive "what changed" and "what changed after AI feedback".
-- +goose Up
CREATE TABLE revision_checkpoint (
  id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  project_id    uuid NOT NULL REFERENCES project(id) ON DELETE CASCADE,
  artifact_type text NOT NULL,   -- draft | outline | snippets | proposal | claim
  trigger       text NOT NULL,   -- ask_feedback | finish | advance
  content       jsonb NOT NULL,
  content_hash  text NOT NULL,
  feedback_ref  uuid,
  created_at    timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX revision_checkpoint_project_artifact_idx
  ON revision_checkpoint (project_id, artifact_type, created_at);

-- +goose Down
DROP TABLE revision_checkpoint;
```

- [ ] **Step 2: Write the sqlc queries**

`apps/api/internal/store/queries/revision_checkpoint.sql`:
```sql
-- name: InsertRevisionCheckpoint :one
INSERT INTO revision_checkpoint (project_id, artifact_type, trigger, content, content_hash, feedback_ref)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: ListRevisionCheckpoints :many
SELECT * FROM revision_checkpoint
WHERE project_id = $1
ORDER BY created_at, artifact_type;
```

- [ ] **Step 3: Generate sqlc**

Run from `apps/api/`: `sqlc generate`
Expected: new `internal/store/sqlc/revision_checkpoint.sql.go` with `RevisionCheckpoint`, `InsertRevisionCheckpointParams`, `InsertRevisionCheckpoint`, `ListRevisionCheckpoints`.

- [ ] **Step 4: Write the round-trip test**

`apps/api/internal/api/revision_checkpoint_store_test.go` — copy DB/harness setup from `question_card_test.go`. Insert one checkpoint, list it back:
```go
func TestRevisionCheckpoint_InsertAndList(t *testing.T) {
    h, d, cookie, projectID := setupSeededProject(t) // harness from question_card_test.go
    _ = h; _ = cookie
    ctx := context.Background()
    got, err := d.Queries.InsertRevisionCheckpoint(ctx, sqlc.InsertRevisionCheckpointParams{
        ProjectID: projectID, ArtifactType: "proposal", Trigger: "finish",
        Content: []byte(`{"objective":"x"}`), ContentHash: "abc", FeedbackRef: pgtype.UUID{Valid: false},
    })
    if err != nil { t.Fatalf("insert: %v", err) }
    if got.ArtifactType != "proposal" { t.Fatalf("artifact_type = %s", got.ArtifactType) }
    rows, err := d.Queries.ListRevisionCheckpoints(ctx, projectID)
    if err != nil || len(rows) != 1 { t.Fatalf("list = %d rows, %v", len(rows), err) }
}
```

- [ ] **Step 5: Run the test** — `cd apps/api && go test ./internal/api/ -run TestRevisionCheckpoint_InsertAndList` — Expected: PASS.

- [ ] **Step 6: Commit** — `git add apps/api/internal/store/migrations/0063_revision_checkpoint.sql apps/api/internal/store/queries/revision_checkpoint.sql apps/api/internal/store/sqlc/revision_checkpoint.sql.go apps/api/internal/api/revision_checkpoint_store_test.go && git commit -m "feat(revision): revision_checkpoint table + sqlc"`

---

### Task 2: `recordCheckpoint` writer + artifact readers + content hash

**Files:**
- Create: `apps/api/internal/api/revision_checkpoint.go`
- Test: `apps/api/internal/api/revision_checkpoint_test.go`

**Interfaces:**
- Consumes: `sqlc.InsertRevisionCheckpoint` (Task 1); readers `a.d.Queries.ListSnippets`, `a.d.Queries.ListOutlineNodes`, `a.d.Queries.GetProjectProposal`, `a.d.Queries.GetLatestSnapshot`; `a.loadEssayState`/`a.essaySubQuestions` (`essay_statement.go:24-45`).
- Produces:
  - `func (a *API) recordCheckpoint(ctx context.Context, projectID uuid.UUID, artifactType, trigger string, feedbackRef *uuid.UUID)` — best-effort; reads the artifact, marshals canonical JSON, hashes, inserts, emits a `revision_checkpoint` event.
  - `func (a *API) recordCheckpoints(ctx context.Context, projectID uuid.UUID, trigger string, feedbackRef *uuid.UUID, artifactTypes ...string)` — loops `recordCheckpoint`.
  - artifact-type constants `checkpointDraft="draft"`, `checkpointOutline="outline"`, `checkpointSnippets="snippets"`, `checkpointProposal="proposal"`, `checkpointClaim="claim"`; trigger constants `triggerAskFeedback="ask_feedback"`, `triggerFinish="finish"`, `triggerAdvance="advance"`.

- [ ] **Step 1: Write the failing test**

`apps/api/internal/api/revision_checkpoint_test.go`:
```go
func TestRecordCheckpoint_ProposalWritesRowWithHash(t *testing.T) {
    h, d, cookie, projectID := setupSeededProject(t)
    _ = h; _ = cookie
    // seed a proposal so there is content to snapshot
    _, _ = d.Queries.UpsertProjectProposal(context.Background(), sqlc.UpsertProjectProposalParams{
        ProjectID: projectID, Objective: "bounded yes", Reason: "r", Activities: "a", Resources: "s"})
    a := &API{d: d}
    a.recordCheckpoint(context.Background(), projectID, checkpointProposal, triggerFinish, nil)
    rows, _ := d.Queries.ListRevisionCheckpoints(context.Background(), projectID)
    if len(rows) != 1 { t.Fatalf("want 1 checkpoint, got %d", len(rows)) }
    if rows[0].ContentHash == "" { t.Fatal("content_hash empty") }
    if !strings.Contains(string(rows[0].Content), "bounded yes") { t.Fatalf("content missing objective: %s", rows[0].Content) }
}

func TestRecordCheckpoint_DraftStoresSnapshotRefNotBody(t *testing.T) {
    h, d, cookie, projectID := setupSeededProject(t)
    _ = h; _ = cookie
    // commit a draft snapshot with a long body
    seq, _ := d.Queries.NextSnapshotSeq(context.Background(), projectID)
    snap, _ := d.Queries.InsertDraftSnapshot(context.Background(), sqlc.InsertDraftSnapshotParams{
        ProjectID: projectID, Seq: seq, Content: strings.Repeat("body ", 200)})
    a := &API{d: d}
    a.recordCheckpoint(context.Background(), projectID, checkpointDraft, triggerFinish, nil)
    rows, _ := d.Queries.ListRevisionCheckpoints(context.Background(), projectID)
    if len(rows) != 1 { t.Fatalf("want 1, got %d", len(rows)) }
    if strings.Contains(string(rows[0].Content), "body body") { t.Fatal("draft checkpoint duplicated body text") }
    if !strings.Contains(string(rows[0].Content), snap.ID.String()) { t.Fatalf("draft checkpoint missing snapshotId ref: %s", rows[0].Content) }
}
```

- [ ] **Step 2: Run to verify it fails** — `cd apps/api && go test ./internal/api/ -run TestRecordCheckpoint` — Expected: FAIL (`recordCheckpoint` undefined).

- [ ] **Step 3: Implement `revision_checkpoint.go`**

```go
package api

import (
    "context"
    "crypto/sha256"
    "encoding/hex"
    "encoding/json"
    "log/slog"

    "github.com/google/uuid"
    "github.com/jackc/pgx/v5/pgtype"

    "github.com/<org>/mind-imprint/apps/api/internal/agent"
    "github.com/<org>/mind-imprint/apps/api/internal/store/sqlc"
)

const (
    checkpointDraft    = "draft"
    checkpointOutline  = "outline"
    checkpointSnippets = "snippets"
    checkpointProposal = "proposal"
    checkpointClaim    = "claim"

    triggerAskFeedback = "ask_feedback"
    triggerFinish      = "finish"
    triggerAdvance     = "advance"
)

// checkpointContent reads the current state of one artifact as canonical JSON.
// Returns nil, nil when there is nothing to snapshot (skip silently).
func (a *API) checkpointContent(ctx context.Context, projectID uuid.UUID, artifactType string) ([]byte, error) {
    switch artifactType {
    case checkpointProposal:
        p, err := a.d.Queries.GetProjectProposal(ctx, projectID)
        if err != nil { return nil, err }
        return json.Marshal(map[string]any{"objective": p.Objective, "reason": p.Reason,
            "activities": p.Activities, "resources": p.Resources, "counterpoints": p.Counterpoints})
    case checkpointOutline:
        nodes, err := a.d.Queries.ListOutlineNodes(ctx, projectID)
        if err != nil { return nil, err }
        return json.Marshal(nodes)
    case checkpointSnippets:
        rows, err := a.d.Queries.ListSnippets(ctx, projectID)
        if err != nil { return nil, err }
        return json.Marshal(rows)
    case checkpointClaim:
        state, err := a.loadEssayState(ctx, projectID)
        if err != nil { return nil, err }
        return json.Marshal(a.essaySubQuestions(state))
    case checkpointDraft:
        snap, err := a.d.Queries.GetLatestSnapshot(ctx, projectID)
        if err != nil { return nil, err } // no snapshot yet -> caller logs & skips
        return json.Marshal(map[string]any{"snapshotId": snap.ID.String()})
    default:
        return nil, nil
    }
}

func (a *API) recordCheckpoint(ctx context.Context, projectID uuid.UUID, artifactType, trigger string, feedbackRef *uuid.UUID) {
    content, err := a.checkpointContent(ctx, projectID, artifactType)
    if err != nil || content == nil {
        if err != nil { slog.Warn("revision: read artifact failed", "artifact", artifactType, "err", err) }
        return
    }
    sum := sha256.Sum256(content)
    fr := pgtype.UUID{Valid: false}
    if feedbackRef != nil { fr = pgtype.UUID{Bytes: *feedbackRef, Valid: true} }
    if _, err := a.d.Queries.InsertRevisionCheckpoint(ctx, sqlc.InsertRevisionCheckpointParams{
        ProjectID: projectID, ArtifactType: artifactType, Trigger: trigger,
        Content: content, ContentHash: hex.EncodeToString(sum[:]), FeedbackRef: fr,
    }); err != nil {
        slog.Warn("revision: insert checkpoint failed", "artifact", artifactType, "err", err)
        return
    }
    store := agent.NewSqlcAgentStore(a.d.Queries, a.d.Pool)
    _ = store.AppendEvent(ctx, agent.EventRow{ProjectID: projectID, Surface: "studio",
        Type: "revision_checkpoint", Payload: mustJSON(map[string]any{"artifactType": artifactType, "trigger": trigger})})
}

func (a *API) recordCheckpoints(ctx context.Context, projectID uuid.UUID, trigger string, feedbackRef *uuid.UUID, artifactTypes ...string) {
    for _, at := range artifactTypes { a.recordCheckpoint(ctx, projectID, at, trigger, feedbackRef) }
}
```
(Use the module path already used by sibling files — copy the import prefix from the top of `question_card.go`.)

- [ ] **Step 4: Run the test** — `cd apps/api && go test ./internal/api/ -run TestRecordCheckpoint` — Expected: PASS.

- [ ] **Step 5: Commit** — `git add apps/api/internal/api/revision_checkpoint.go apps/api/internal/api/revision_checkpoint_test.go && git commit -m "feat(revision): recordCheckpoint writer + artifact readers"`

---

### Task 3: Wire Mechanism-1 ask_feedback triggers

**Files:**
- Modify: `apps/api/internal/api/writing.go:287-444` (`orderReview`)
- Modify: `apps/api/internal/api/essay_statement.go:300-333` (`reviewEssayStatement`)
- Modify: `apps/api/internal/api/card_reflect.go` (`postReflectProjectCard`, ~line 95)
- Modify: `apps/api/internal/api/coach.go:52-262` (`postCoach`)
- Test: `apps/api/internal/api/revision_trigger_test.go`

**Interfaces:** Consumes `recordCheckpoint`/`recordCheckpoints` (Task 2).

- [ ] **Step 1: Write the failing test** (drive the real endpoints; assert checkpoints appear)

```go
func TestOrderReview_RecordsDraftCheckpoint(t *testing.T) {
    h, d, cookie, projectID := setupSeededProject(t)
    sid := commitSnapshotViaAPI(t, h, cookie, projectID, "my draft body") // helper: POST /snapshots
    postJSON(t, h, cookie, "/api/v1/projects/"+projectID.String()+"/snapshots/"+sid+"/review", "")
    rows, _ := d.Queries.ListRevisionCheckpoints(context.Background(), projectID)
    if !hasCheckpoint(rows, "draft", "ask_feedback") { t.Fatal("order-review did not record a draft ask_feedback checkpoint") }
}

func TestWritingCoachTurn_RecordsSnippetsCheckpoint(t *testing.T) {
    h, d, cookie, projectID := setupSeededProject(t)
    postJSON(t, h, cookie, "/api/v1/projects/"+projectID.String()+"/coach",
        `{"scope":"writing","user_input":"帮我看这段论点够不够有力，别改写"}`)
    rows, _ := d.Queries.ListRevisionCheckpoints(context.Background(), projectID)
    if !hasCheckpoint(rows, "snippets", "ask_feedback") { t.Fatal("writing coach turn did not record snippets checkpoint") }
}
```
(`hasCheckpoint(rows, artifact, trigger)` is a 3-line helper in the test file.)

- [ ] **Step 2: Run to verify it fails** — `go test ./internal/api/ -run 'TestOrderReview_RecordsDraftCheckpoint|TestWritingCoachTurn'` — Expected: FAIL.

- [ ] **Step 3: Add the calls at each trigger** (best-effort, after the primary work succeeds, before writing the response)

In `orderReview` (`writing.go`, right after the review is persisted, near l.418):
```go
a.recordCheckpoint(r.Context(), projectID, checkpointDraft, triggerAskFeedback, nil)
```
In `reviewEssayStatement` (`essay_statement.go`, after `runDraftAnnotationReview` returns):
```go
a.recordCheckpoints(r.Context(), projectID, triggerAskFeedback, nil, checkpointClaim, checkpointOutline)
```
In `postReflectProjectCard` (`card_reflect.go`, after `persistProjectCardEnvelope` at ~l.95):
```go
a.recordCheckpoints(r.Context(), projectID, triggerAskFeedback, nil, checkpointSnippets, checkpointClaim)
```
In `postCoach` (`coach.go`, in the main studio path after the reply is persisted, ~l.249), branch on the client `scope`:
```go
switch strings.TrimSpace(body.Scope) {
case "proposal_review":
    a.recordCheckpoint(r.Context(), projectID, checkpointProposal, triggerAskFeedback, nil)
case "writing":
    a.recordCheckpoints(r.Context(), projectID, triggerAskFeedback, nil, checkpointSnippets, checkpointDraft, checkpointOutline)
}
```
(Place this AFTER the existing `AppendEvent(... "coach_turn" ...)`; `feedback_ref` stays nil — evaluator pairs by trigger+timestamp.)

- [ ] **Step 4: Run the test** — same command — Expected: PASS. Also run the whole file's neighbours to prove no regression: `go test ./internal/api/ -run 'TestOrderReview|TestCoach'`.

- [ ] **Step 5: Commit** — `git add apps/api/internal/api/writing.go apps/api/internal/api/essay_statement.go apps/api/internal/api/card_reflect.go apps/api/internal/api/coach.go apps/api/internal/api/revision_trigger_test.go && git commit -m "feat(revision): record checkpoints at ask-feedback triggers"`

---

### Task 4: Wire Mechanism-1 finish/advance triggers

**Files:**
- Modify: `apps/api/internal/api/project_writing_finish.go:27-76` (`finishWriting`)
- Modify: `apps/api/internal/api/project_finish.go:26-103` (`finishProject`)
- Modify: `apps/api/internal/api/coach.go:979+` (`postCoachAdvance`)
- Test: append to `apps/api/internal/api/revision_trigger_test.go`

- [ ] **Step 1: Write the failing test**
```go
func TestFinishEssay_RecordsWritingCheckpoints(t *testing.T) {
    h, d, cookie, projectID := setupSeededProject(t)
    postJSON(t, h, cookie, "/api/v1/projects/"+projectID.String()+"/finish-writing?doc=essay", "")
    rows, _ := d.Queries.ListRevisionCheckpoints(context.Background(), projectID)
    for _, art := range []string{"draft","outline","snippets"} {
        if !hasCheckpoint(rows, art, "finish") { t.Fatalf("finish-essay missing %s checkpoint", art) }
    }
}
```

- [ ] **Step 2: Run to verify it fails** — `go test ./internal/api/ -run TestFinishEssay_RecordsWritingCheckpoints` — Expected: FAIL.

- [ ] **Step 3: Add the calls**

In `finishWriting` (`project_writing_finish.go`, after `appendAutoLog` at l.72), branch on the `doc` query param:
```go
if doc == "proposal" {
    a.recordCheckpoint(r.Context(), projectID, checkpointProposal, triggerFinish, nil)
} else { // essay (default)
    a.recordCheckpoints(r.Context(), projectID, triggerFinish, nil, checkpointDraft, checkpointOutline, checkpointSnippets)
}
```
In `finishProject` (`project_finish.go`, after status set to `evaluating`, before `go a.runProjectReport`):
```go
a.recordCheckpoints(r.Context(), projectID, triggerFinish, nil,
    checkpointDraft, checkpointOutline, checkpointSnippets, checkpointProposal, checkpointClaim)
```
In `postCoachAdvance` (`coach.go`, after the advance is applied):
```go
a.recordCheckpoints(r.Context(), projectID, triggerAdvance, nil, checkpointDraft, checkpointOutline, checkpointSnippets)
```
(Advance snapshots the writing artifacts; missing artifacts are skipped silently by `checkpointContent` returning an error → logged, no row.)

- [ ] **Step 4: Run the test** — Expected: PASS. Regression: `go test ./internal/api/ -run 'TestFinish|TestCoachAdvance'`.

- [ ] **Step 5: Commit** — `git add apps/api/internal/api/project_writing_finish.go apps/api/internal/api/project_finish.go apps/api/internal/api/coach.go apps/api/internal/api/revision_trigger_test.go && git commit -m "feat(revision): record checkpoints at finish/advance triggers"`

---

### Task 5: Mechanism-2 exploration mutation events

**Files:**
- Modify: `apps/api/internal/api/exploration.go` — `createExplorationLead` (207-286), `deleteExplorationLead` (365-397), `createQuestionEdge` (942-1003), `deleteQuestionEdge` (1077-1118), `adoptExploration` (708-835), `attachExploration` (835-942)
- Test: `apps/api/internal/api/revision_exploration_events_test.go`

**Interfaces:** Consumes `agent.NewSqlcAgentStore(...).AppendEvent` + `mustJSON`. Produces event types: `lead_added`, `lead_removed`, `edge_added`, `edge_removed`, `lead_adopted`, `source_attached`.

- [ ] **Step 1: Add a small emit helper to `revision_checkpoint.go`**
```go
func (a *API) emitMutation(ctx context.Context, projectID uuid.UUID, eventType string, payload map[string]any) {
    store := agent.NewSqlcAgentStore(a.d.Queries, a.d.Pool)
    if err := store.AppendEvent(ctx, agent.EventRow{ProjectID: projectID, Surface: "studio",
        Type: eventType, Payload: mustJSON(payload)}); err != nil {
        slog.Warn("revision: emit mutation event failed", "type", eventType, "err", err)
    }
}
```

- [ ] **Step 2: Write the failing test** (count events by type via the event read path)
```go
func TestExploration_EmitsLeadAndEdgeEvents(t *testing.T) {
    h, d, cookie, projectID := setupSeededProject(t)
    lid := createLeadViaAPI(t, h, cookie, projectID, "sub-question A")
    deleteLeadViaAPI(t, h, cookie, projectID, lid)
    if eventCount(t, d, projectID, "lead_added") != 1 { t.Fatal("missing lead_added") }
    if eventCount(t, d, projectID, "lead_removed") != 1 { t.Fatal("missing lead_removed") }
}
```
(`eventCount(t,d,projectID,type)` selects `count(*) from event where project_id=$1 and type=$2`.)

- [ ] **Step 3: Run to verify it fails** — `go test ./internal/api/ -run TestExploration_EmitsLeadAndEdgeEvents` — Expected: FAIL.

- [ ] **Step 4: Emit at each handler** (after the sqlc mutation succeeds, before writing the response). Examples:

`createExplorationLead` (after the lead row is created, `lead` in scope):
```go
a.emitMutation(r.Context(), projectID, "lead_added", map[string]any{"leadId": lead.ID.String(), "text": lead.Text, "origin": lead.Origin})
```
`deleteExplorationLead` (after delete; you have `lid` and the pre-delete text):
```go
a.emitMutation(r.Context(), projectID, "lead_removed", map[string]any{"leadId": lid.String(), "text": prevText})
```
`createQuestionEdge`: `"edge_added"` with `{"from": from.String(), "to": to.String(), "origin": "manual"}`.
`deleteQuestionEdge`: `"edge_removed"` with `{"from","to"}` (read the edge before deleting for from/to).
`adoptExploration`: `"lead_adopted"` with `{"leadId","parentLeadId","source": title}`.
`attachExploration`: `"source_attached"` with `{"referenceId","parentLeadId"}`.

- [ ] **Step 5: Run the test** — Expected: PASS. Regression: `go test ./internal/api/ -run TestExploration`.

- [ ] **Step 6: Commit** — `git add apps/api/internal/api/exploration.go apps/api/internal/api/revision_checkpoint.go apps/api/internal/api/revision_exploration_events_test.go && git commit -m "feat(revision): exploration mutation events"`

---

### Task 6: Mechanism-2 source mutation events

**Files:**
- Modify: `apps/api/internal/api/workspace_library.go` — `createReference` (366-464), `deleteReference` (602-628), `patchReference` (464-602, decision/classification change)
- Modify: `apps/api/internal/api/evidence_map.go` — `archiveReference` (217-239), `patchReferenceTriage` (192-217), `patchReferenceEvidence` (161-192)
- Test: `apps/api/internal/api/revision_source_events_test.go`

**Interfaces:** Consumes `a.emitMutation` (Task 5). Event types: `source_added`, `source_dropped`, `source_reclassified`.

- [ ] **Step 1: Write the failing test**
```go
func TestSource_EmitsAddDropReclassify(t *testing.T) {
    h, d, cookie, projectID := setupSeededProject(t)
    rid := createRefViaAPI(t, h, cookie, projectID, "NASA greening", "期刊论文")
    patchJSON(t, h, cookie, "/api/v1/projects/"+projectID.String()+"/references/"+rid+"/triage", `{"triage":"yellow"}`)
    deleteJSON(t, h, cookie, "/api/v1/projects/"+projectID.String()+"/references/"+rid)
    if eventCount(t, d, projectID, "source_added") != 1 { t.Fatal("missing source_added") }
    if eventCount(t, d, projectID, "source_reclassified") != 1 { t.Fatal("missing source_reclassified") }
    if eventCount(t, d, projectID, "source_dropped") != 1 { t.Fatal("missing source_dropped") }
}
```

- [ ] **Step 2: Run to verify it fails** — `go test ./internal/api/ -run TestSource_EmitsAddDropReclassify` — Expected: FAIL.

- [ ] **Step 3: Emit at each handler**
`createReference` (after insert): `"source_added"` `{"referenceId","title","classification"}`.
`deleteReference` + `archiveReference` (after, with pre-read title): `"source_dropped"` `{"referenceId","title"}`.
`patchReferenceTriage` (after, with old value read before `SetReferenceTriage`): `"source_reclassified"` `{"referenceId","field":"triage","before":prev,"after":body.Triage}`.
`patchReferenceEvidence`: `"source_reclassified"` `{"referenceId","field":"evidence", ...}`.
`patchReference` (only when decision/classification actually changes): `"source_reclassified"` `{"referenceId","field":"decision"|"classification","before","after"}`.

- [ ] **Step 4: Run the test** — Expected: PASS. Regression: `go test ./internal/api/ -run 'TestReference|TestSource|TestEvidence'`.

- [ ] **Step 5: Commit** — `git add apps/api/internal/api/workspace_library.go apps/api/internal/api/evidence_map.go apps/api/internal/api/revision_source_events_test.go && git commit -m "feat(revision): source mutation events"`

---

### Task 7: `GET /revision-checkpoints` read endpoint

**Files:**
- Modify: `apps/api/internal/api/revision_checkpoint.go` (add handler)
- Modify: `apps/api/internal/api/api.go:~110` (register route in the exploration/library block)
- Test: append to `apps/api/internal/api/revision_checkpoint_test.go`

**Interfaces:** Produces `GET /api/v1/projects/{id}/revision-checkpoints` → `{"checkpoints":[{id,artifactType,trigger,contentHash,feedbackRef,createdAt},...]}` ordered by createdAt. Read-only, no model call.

- [ ] **Step 1: Write the failing test**
```go
func TestGetRevisionCheckpoints_ReturnsOrdered(t *testing.T) {
    h, d, cookie, projectID := setupSeededProject(t)
    a := &API{d: d}
    a.recordCheckpoint(context.Background(), projectID, checkpointProposal, triggerFinish, nil)
    body := getJSON(t, h, cookie, "/api/v1/projects/"+projectID.String()+"/revision-checkpoints")
    if !strings.Contains(body, "\"artifactType\":\"proposal\"") { t.Fatalf("missing checkpoint in response: %s", body) }
}
```

- [ ] **Step 2: Run to verify it fails** — Expected: FAIL (404 / no route).

- [ ] **Step 3: Implement handler + register route**
```go
func (a *API) listRevisionCheckpoints(w http.ResponseWriter, r *http.Request) {
    projectID, ok := a.loadOwnedProject(w, r)
    if !ok { return }
    rows, err := a.d.Queries.ListRevisionCheckpoints(r.Context(), projectID)
    if err != nil { httpx.WriteError(w, r, err); return }
    out := make([]map[string]any, 0, len(rows))
    for _, c := range rows {
        var fr any
        if c.FeedbackRef.Valid { fr = uuid.UUID(c.FeedbackRef.Bytes).String() }
        out = append(out, map[string]any{"id": c.ID.String(), "artifactType": c.ArtifactType,
            "trigger": c.Trigger, "contentHash": c.ContentHash, "feedbackRef": fr, "createdAt": c.CreatedAt.Time})
    }
    httpx.WriteJSON(w, http.StatusOK, map[string]any{"checkpoints": out})
}
```
Register at `api.go` near l.110: `mux.Handle("GET /api/v1/projects/{id}/revision-checkpoints", protected(a.listRevisionCheckpoints))`

- [ ] **Step 4: Run the test** — Expected: PASS.

- [ ] **Step 5: Commit** — `git add apps/api/internal/api/revision_checkpoint.go apps/api/internal/api/api.go apps/api/internal/api/revision_checkpoint_test.go && git commit -m "feat(revision): GET /revision-checkpoints read endpoint"`

---

### Task 8: Contracts — Zod schemas + event types

**Files:**
- Create: `packages/contracts/src/revisionCheckpoint.ts`
- Modify: `packages/contracts/src/event.ts` (add the new event `type` strings to `EVENT_TYPES` / the union)
- Modify: `packages/contracts/src/index.ts` (barrel export)
- Test: `packages/contracts/src/revisionCheckpoint.test.ts`

**Interfaces:** Produces `RevisionCheckpointContent` union + `REVISION_EVENT_TYPES` const. Go still validates only the outer envelope; these schemas are the inner-structure source of truth.

- [ ] **Step 1: Write the failing test**
```ts
import { describe, it, expect } from "vitest";
import { RevisionCheckpointArtifactType, ProposalCheckpointContent } from "./revisionCheckpoint";
describe("revision checkpoint contracts", () => {
  it("accepts a proposal content shape", () => {
    expect(() => ProposalCheckpointContent.parse({ objective: "x", reason: "y", activities: "a", resources: "s", counterpoints: "" })).not.toThrow();
  });
  it("rejects an unknown artifact type", () => {
    expect(RevisionCheckpointArtifactType.safeParse("banana").success).toBe(false);
  });
});
```

- [ ] **Step 2: Run to verify it fails** — `cd packages/contracts && pnpm vitest run src/revisionCheckpoint.test.ts` — Expected: FAIL (module not found).

- [ ] **Step 3: Implement `revisionCheckpoint.ts`**
```ts
import { z } from "zod";

export const RevisionCheckpointArtifactType = z.enum(["draft","outline","snippets","proposal","claim"]);
export const RevisionCheckpointTrigger = z.enum(["ask_feedback","finish","advance"]);

export const DraftCheckpointContent = z.object({ snapshotId: z.string().uuid() });
export const ProposalCheckpointContent = z.object({
  objective: z.string(), reason: z.string(), activities: z.string(), resources: z.string(), counterpoints: z.string().optional().default(""),
});
export const ClaimCheckpointContent = z.array(z.object({ text: z.string() }));
// outline / snippets mirror their existing row schemas (outlineNode.ts / snippet.ts).

export const REVISION_EVENT_TYPES = [
  "revision_checkpoint",
  "lead_added","lead_removed","edge_added","edge_removed","lead_adopted","source_attached",
  "source_added","source_dropped","source_reclassified",
] as const;
```

- [ ] **Step 4: Extend `event.ts` + barrel** — add `REVISION_EVENT_TYPES` members to the exported `EVENT_TYPES` array; `export * from "./revisionCheckpoint";` in `index.ts`.

- [ ] **Step 5: Run the test** — `cd packages/contracts && pnpm vitest run src/revisionCheckpoint.test.ts` — Expected: PASS. Then full contracts suite: `pnpm vitest run`.

- [ ] **Step 6: Commit** — `git add packages/contracts/src/revisionCheckpoint.ts packages/contracts/src/event.ts packages/contracts/src/index.ts packages/contracts/src/revisionCheckpoint.test.ts && git commit -m "feat(revision): Zod contracts for checkpoints + mutation events"`

---

### Task 9: Full-suite guard + behavior-spec note

**Files:** none created; verification only.

- [ ] **Step 1: Run the whole api package** — `cd apps/api && go test ./internal/api/...` — Expected: PASS (no regression from the added best-effort calls).
- [ ] **Step 2: Run the whole contracts suite** — `cd packages/contracts && pnpm vitest run` — Expected: PASS.
- [ ] **Step 3: Confirm no status transition changed** — grep the modified handlers for any change to `set_status` / `advanceStatusTo` logic; there must be none (recording is additive). Re-read the trigger list against `docs/2026-08-09-all-statuses.md` statuses 4/5/6/7. If any recording call sits before the primary action's success check, move it after.
- [ ] **Step 4: Commit any fixups** — `git commit -am "test(revision): full-suite guard + behavior-spec check"` (only if fixups were needed).

---

## Self-Review

**Spec coverage:** §2 Mechanism 1 → Tasks 1-4,7; §2 Mechanism 2 → Tasks 5-6; §5 event list → Tasks 5-6; §6 schema (content_hash, draft ref) → Tasks 1-2; §7 contracts → Task 8; §8 read path → Task 7; §9 best-effort → Global Constraints + every trigger step; §10 behavior-spec → Task 9. No gaps.

**Placeholder scan:** all steps carry real SQL/Go/TS and exact file:line targets; no TBD/"similar to". The one deferred detail — `feedback_ref` population — is explicitly resolved as "nullable, left null for coach-scope" in Global Constraints and Task 3 Step 3.

**Type consistency:** `recordCheckpoint`/`recordCheckpoints`/`emitMutation` signatures and the `checkpoint*`/`trigger*` constants are defined in Task 2/5 and used unchanged in Tasks 3-7. Event-type strings in Tasks 5-6 match `REVISION_EVENT_TYPES` in Task 8.
