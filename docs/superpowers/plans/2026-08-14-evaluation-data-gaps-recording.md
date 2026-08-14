# Evaluation Data Gaps — Recording Infrastructure Implementation Plan

> **For agentic workers:** executes the six gaps in `docs/2026-08-13-evaluation-data-gaps.md`. Steps use checkbox (`- [ ]`) syntax.

**Goal:** Build ALL deterministic data infrastructure the (future) evaluation-report generator needs — recording (G1/G2/G3), retrievable subagent inputs + the-three filter (G4), evidence-candidate index + ref post-validation (G5), and `CountWords` (G6). **Only the LLM generation algorithm (recorded data → depth levels / autonomy bands / prose) stays deferred** (`evalreport.Placeholder` unchanged).

**Architecture:** Go backend (`apps/api`, pgx/sqlc/goose) + React frontend (`apps/web`). New: one table (`citation`), two milestone/open event types on the existing free-form `event` stream, several sqlc read/count queries, three pure `evalreport` modules (index / validate / countwords), two small write endpoints, and provenance threading through the writing-room insert path.

**Tech Stack:** Go 1.x, sqlc (pin v1.27.0, `CGO_ENABLED=0`), goose, testcontainers (run FOREGROUND, `-timeout 1800s`); React + Vite + TS.

## Global Constraints

- Recording only — do NOT wire any of this into a real report-generation pass; `generateAndStoreEvaluationReport` keeps emitting `evalreport.Placeholder`.
- Envelope discipline unchanged; `packages/contracts` stays the deep-shape truth.
- Every LLM-spending path already exists; no new model calls in this plan.
- Section keys for citations use the machine convention already in use: `claim:<uuid>` / `subq:<id>` / `prop:<step>` (see memory `guided-snippet-labels`).
- `event.surface` CHECK is `studio|course|chat` → always `studio` here. `event.type` is free-form.
- Consistency vs `docs/2026-08-09-all-statuses.md`: framework-finish ties to the generate-plan click (§2→§3); 批注 exist at proposal (§4) + paper (§6); source→claim is the tagging model (§6). All three confirmed consistent.

---

### Task 1: G6 — `CountWords` pure function

**Files:**
- Create: `apps/api/internal/evalreport/countwords.go`
- Test: `apps/api/internal/evalreport/countwords_test.go`

**Interfaces:**
- Produces: `func CountWords(text string) int` — (# CJK codepoints) + (# non-CJK whitespace-delimited tokens). Han ideographs each count 1; CJK punctuation excluded; non-CJK runs split on unicode whitespace.

- [ ] Write table tests: `""`→0; `"hello world"`→2; `"中国最可持续"`→6; mixed `"中国 is green"`→ 2(中国? no: 2 CJK "中国"=2) +2 tokens = 4; punctuation-only CJK excluded.
- [ ] Implement with `unicode.Is(unicode.Han, r)` for CJK count and a whitespace-split token count over the non-CJK residue.
- [ ] Run `go test ./internal/evalreport/ -run CountWords`.
- [ ] Commit.

---

### Task 2: G5 — evidence-candidate index + ref post-validation

**Files:**
- Create: `apps/api/internal/evalreport/evidence_index.go` (builder + `ValidateRefs`)
- Test: `apps/api/internal/evalreport/evidence_index_test.go`

**Interfaces:**
- Produces:
  - `type Candidate struct { ID, Kind, Label string }`
  - `type EvidenceIndex struct { byID map[string]Candidate }` with `Has(id) bool`, `Get(id) (Candidate, bool)`, `Len() int`.
  - `func BuildEvidenceIndex(ctx, src IndexSource, projectID uuid.UUID) (EvidenceIndex, error)` where `IndexSource` is a narrow interface over the existing list queries (chat_message, event, reference, card_instances, exploration_lead).
  - `func ValidateRefs(rep *Report, idx EvidenceIndex) DropStats` — walks every `Ref.ID` and `EvidenceItem.ID` in the report; any id not in the index is blanked (keep `Label`); returns `DropStats{Total, Dropped int}`.

- [ ] Define `IndexSource` interface + a real adapter over `sqlc.Queries` (`evidence_index.go`).
- [ ] `BuildEvidenceIndex`: each record → `Candidate{ID, Kind, Label}` (Kind ∈ chat|event|reference|card|lead; Label = short human string, ≤60 runes).
- [ ] `ValidateRefs`: reflectively/explicitly visit `Events[].Ref`, `Materials[].UsedIn`, `Depth[].Evidence[].ID`, `Autonomy[].Evidence[].ID`, `PromptLens.Prompts[].Ref`, `Risks[].Ref`; blank unknown ids.
- [ ] Test `ValidateRefs` on a report with 1 valid + 1 bogus id → `Dropped==1`, label preserved, valid untouched. (Builder tested in Task 8 with testcontainers.)
- [ ] Run `go test ./internal/evalreport/ -run 'Validate|Index'`.
- [ ] Commit.

---

### Task 3: G4 — the-three subagent filter + reading-input audit

**Files:**
- Create: `apps/api/internal/evalreport/subagents.go` (+ test)
- Possibly modify: `apps/api/internal/api/readturn.go` (only if reading input is not persisted)

**Interfaces:**
- Produces: `var StudentFacingSubagents = map[string][]string{ "review": {"proposal_annotation","essay_annotation","order_review"}, "card": {"question_card","read_card_example"}, "reading": {"read_router","read_eval","reading_takeaway_draft"} }` and `func IsStudentFacingPurpose(p string) (group string, ok bool)`.

- [ ] Write `subagents.go` with the grouping + lookup; unit-test the lookup + that excluded purposes (`dig_query`,`suggest_placement`,`search_guidance`,`exploration_guide`,`edge_propose`,`exploration_review`) return `ok==false`.
- [ ] Audit reading-room input persistence in `readturn.go`: confirm the student's reading selection/question is retrievable (chat_message surface=reading, or an event). If NOT persisted, append a best-effort `event` `type="reading_input"` with the selection/question payload at turn start. Card input = `card_instances.field_values/event_trace` (already durable); review input = `edit_buffer`/`draft_snapshot` (already durable) — no work.
- [ ] Run `go test ./internal/evalreport/ -run Subagent`.
- [ ] Commit.

---

### Task 4: Migrations + sqlc queries (G1 read, G2 count, G3 citation table, G1 backfill)

**Files:**
- Create: `apps/api/internal/store/migrations/0068_citation.sql`
- Create: `apps/api/internal/store/migrations/0069_backfill_framework_finished.sql`
- Create: `apps/api/internal/store/queries/citation.sql`
- Modify: `apps/api/internal/store/queries/event.sql` (add `GetFrameworkFinishedAt`, `CountEventsByType`)
- Modify: `apps/api/internal/store/queries/intervention.sql` (add `CountAnnotationsByProject`)
- Regenerate: `apps/api/internal/store/sqlc/*` (sqlc generate)

- [ ] `0068_citation.sql`: `CREATE TABLE citation (id uuid PK default gen_random_uuid(), project_id uuid NOT NULL REFERENCES project(id) ON DELETE CASCADE, reference_id uuid NOT NULL REFERENCES reference(id) ON DELETE CASCADE, section text NOT NULL, created_at timestamptz NOT NULL default now())`; index `(project_id)`. Down: drop table.
- [ ] `0069_backfill_framework_finished.sql` (Up): `INSERT INTO event (id,project_id,user_id,surface,type,payload,created_at) SELECT gen_random_uuid(), p.id, p.user_id, 'studio', 'milestone:framework_finished', '{}'::jsonb, MIN(pi.created_at) FROM project p JOIN plan_item pi ON pi.project_id=p.id WHERE NOT EXISTS (SELECT 1 FROM event e WHERE e.project_id=p.id AND e.type='milestone:framework_finished') GROUP BY p.id, p.user_id;` Down: `DELETE FROM event WHERE type='milestone:framework_finished'` (best-effort; note it also removes going-forward rows — acceptable for a dev down-migration).
- [ ] `citation.sql`: `CreateCitation` (project_id, reference_id, section) `:one`; `ListCitationsByProject` `:many`.
- [ ] `event.sql`: `GetFrameworkFinishedAt :one` = `SELECT MIN(created_at) FROM event WHERE project_id=$1 AND type='milestone:framework_finished'`; `CountEventsByType :one` = count by (project_id, type).
- [ ] `intervention.sql`: `CountAnnotationsByProject :one` = count where `type IN ('proposal_annotation','essay_annotation')`.
- [ ] Regenerate sqlc (`cd apps/api && CGO_ENABLED=0 go run github.com/sqlc-dev/sqlc/cmd/sqlc@v1.27.0 generate` or the repo's make target).
- [ ] `go build ./...`.
- [ ] Commit.

---

### Task 5: G1 — emit framework-finished milestone on plan generation

**Files:**
- Modify: `apps/api/internal/api/workspace_plan_generate.go` (`regeneratePlan`)
- Test: `apps/api/internal/api/workspace_plan_generate_test.go` (or the existing plan-gen test)

- [ ] After the successful `tx.Commit`, emit the milestone ONCE: if `CountEventsByType(projectID,"milestone:framework_finished")==0`, fetch `project.UserID`, `store.AppendEvent(EventRow{ProjectID, Surface:"studio", Type:"milestone:framework_finished", Payload: {}})`. Best-effort (log on error, never fail the request). Idempotent across regenerations.
- [ ] Test: first `regeneratePlan` → exactly one milestone event; second call → still exactly one.
- [ ] Run `go test ./internal/api/ -run Plan -timeout 1800s`.
- [ ] Commit.

---

### Task 6: G2 — annotation-open recording endpoint

**Files:**
- Create/modify: `apps/api/internal/api/proposal_annotations.go` (add `postAnnotationOpen`)
- Modify: router registration (find where `reviewProposalAnnotations` is routed)
- Test: `apps/api/internal/api/proposal_annotations_test.go`

- [ ] `POST /projects/{id}/annotations/open` body `{annotationId, doc}` → `loadOwnedProject` → `AppendEvent(type="annotation_opened", payload={annotation_id, doc})` → 204. (Keyed by intervention id; opens accumulate on the event stream — robust to `ReplaceAnnotations`.)
- [ ] Register the route next to the other proposal-annotation routes.
- [ ] Test: post open → one `annotation_opened` event with the id in payload.
- [ ] Run `go test ./internal/api/ -run Annotation -timeout 1800s`.
- [ ] Commit.

---

### Task 7: G3 — citation write endpoint

**Files:**
- Create: `apps/api/internal/api/citation.go` (`postCitation`)
- Modify: router registration
- Test: `apps/api/internal/api/citation_test.go`

- [ ] `POST /projects/{id}/citations` body `{referenceId, section}` → `loadOwnedProject` → verify the reference belongs to the project (`GetReferenceForProject`) → `CreateCitation` → 201 `{id}`. Ignore/400 on unknown reference.
- [ ] Register route.
- [ ] Test: insert a reference, POST citation → row present via `ListCitationsByProject`; cross-project reference → 404/400.
- [ ] Run `go test ./internal/api/ -run Citation -timeout 1800s`.
- [ ] Commit.

---

### Task 8: G5 builder integration test (testcontainers)

**Files:**
- Modify: `apps/api/internal/evalreport/evidence_index_test.go` (add a DB-backed builder test) OR a new `apps/api/internal/api/evidence_index_it_test.go` if it needs the seeded project.

- [ ] Against a seeded project, `BuildEvidenceIndex` returns candidates spanning ≥2 kinds; every returned `ID` resolves via `Has`. Keep it small.
- [ ] Run `go test ./... -timeout 1800s` for the touched packages (FOREGROUND).
- [ ] Commit.

---

### Task 9: Frontend — annotation-open + citation-at-insert wiring

**Files:**
- Modify: `apps/web/src/api/*` (add `recordAnnotationOpen`, `recordCitation` clients)
- Modify: `apps/web/src/workspace/blocks/ReferencePanel.tsx` (thread `doc`+`projectId`+annotation id into `ProposalAnnotationGroup.onJump`; thread `referenceId` into `CollectedSection.onInsert`)
- Modify: `apps/web/src/workspace/WorkspaceContainer.tsx` (pass ids/doc down; extend the shared insert ref signature)
- Modify: `apps/web/src/workspace/blocks/WritingBlock.tsx` (`insertAtCaret(text, referenceId?)`; sections path carries `referenceId` to the section editor)
- Modify: the sections editor that sets `sectionInsertRef.current` (fires `recordCitation` when a `referenceId` + focused section are present)

- [ ] Add client fns: `recordAnnotationOpen(projectId, annotationId, doc)` → `POST /projects/{id}/annotations/open`; `recordCitation(projectId, referenceId, section)` → `POST /projects/{id}/citations`.
- [ ] G2: in `ProposalAnnotationGroup.onJump`, after the scroll call, fire `recordAnnotationOpen(projectId, a.id, doc)` (best-effort, no await-block). Thread `projectId`+`doc` as props.
- [ ] G3: `CollectedSection` library-fragment insert passes the owning `ref.id`; thread through `onInsert(text, referenceId?)` → shared ref `(text, referenceId?)` → `insertAtCaret` → sections editor, which fires `recordCitation(projectId, referenceId, section)` for the focused section. Snippet inserts + free-mode inserts record nothing (no source id / no section).
- [ ] `npm run typecheck` (NOT build) in `apps/web`.
- [ ] Add/adjust a light test for the two client fns if the api dir has a test pattern; otherwise rely on typecheck.
- [ ] Commit.

---

### Task 10: Full verification + deploy

- [ ] `cd apps/api && go build ./... && go vet ./...`.
- [ ] `go test ./internal/evalreport/... ./internal/api/... -timeout 1800s` (FOREGROUND).
- [ ] `cd apps/web && npm run typecheck && npx vitest run` (touched areas).
- [ ] Commit + push; deploy `full` via `./.deploy-local/deploy.sh full`.
- [ ] Prod smoke: generate a plan → milestone event present; open a 批注 → annotation_opened event; insert a source into a section → citation row.
- [ ] Update memory.
