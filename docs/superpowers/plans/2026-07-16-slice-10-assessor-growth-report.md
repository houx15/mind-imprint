# Slice 10 — the isolated assessor + growth report (成长报告) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Wire the isolated **assessor** live — a few-shot MVP engine that reads a projection of one project's event stream and scores the seeded CT rubric (per-dimension L1–L4 + evidence + a growth narrative) into the 成长报告 slot.

**Architecture:** The seeded CT rubric goes single-source (canonical JSON, TS import + Go embed). `ProjectData` finally loads the event stream; a pure `BuildAssessmentInput` digests the process record; `agent.Assess` makes one isolated flagship call through the enforcement stack; the result persists to the reused `evaluations` table (new `project_id`) and renders in `GrowthReport` behind GET (read, no model call) / POST (generate) endpoints.

**Tech Stack:** Go (`net/http`, sqlc, goose, testcontainers, `go:embed`), TypeScript + React + vitest, Zod contracts, `gateway.StubProvider` for engine tests.

## Global Constraints

- **DEC-10.1** rubric + narrative only — depth-vs-independence radar + T1–T7 leaps deferred.
- **DEC-10.2** the assessor reads a projection of the append-only event stream; `ProjectData` gains `Events`.
- **DEC-10.3** minimal anchor few-shot now (1–2 compact Phoebe-arc samples), backend-only `go:embed`.
- **RL-5** the assessment is diagnostic evidence framed for growth — **no number-grade, no rank, no overall score**; every level carries its behavioral evidence.
- Full **10-dim** CT rubric scored (D1–D10); `NA` allowed for thin evidence; unknown level → `NA`; a missing dimension → `NA`; dimensions emitted in the rubric's declared order.
- Assessment is **student-triggered**, inline (`status='done'`), **isolated** (own endpoint, never in the coach loop), flagship model **never downgraded**.
- Enforcement: banned-phrasing on the narrative + every evidence field; **any** match rejects the WHOLE assessment (all-or-nothing). RL-1 structurally N/A (writes `evaluations`, never the draft).
- **Single-source law:** the rubric JSON is canonical in `packages/contracts/`; Go embeds a synced mirror — never hand-edit the mirror. `rubric.test.ts` must stay green (byte-faithful extraction).
- `evaluations` column names are verbatim from `0001_init.sql`: `scores`, `narrative`, `model`, `tier`, `prompt_tokens`, `completion_tokens`, `cost_estimate`, `status`.
- Go tests: `CGO_ENABLED=0 go test -p 1 ./...` on a quiet Docker; **FULL packages** for the gate, not `-run` subsets. `make sqlc` / `make sync-rubric` from `apps/api`. Web/contracts: `npm test`; `tsc --noEmit` clean.
- Whole-branch review (Opus) MANDATORY at the end. **Never `git add` untracked user files** — the pre-existing `M package.json` + untracked user docs/pngs are NOT ours; stage named paths only. Icons inline SVG, never lucide-react.

---

### Task 1: CT rubric goes single-source (contracts JSON + Go embed)

**Files:**
- Create: `packages/contracts/src/ct-rubric.json` (canonical)
- Modify: `packages/contracts/src/rubric.ts` (import + parse the JSON)
- Test: `packages/contracts/test/rubric.test.ts` (unchanged — must stay green)
- Create: `apps/api/tools/syncrubric/main.go` (mirror tool)
- Modify: `apps/api/Makefile` (add `sync-rubric`)
- Create: `apps/api/internal/rubric/ct-rubric.json` (synced mirror — generated, never hand-edit)
- Create: `apps/api/internal/rubric/rubric.go` (embed + parse)
- Test: `apps/api/internal/rubric/rubric_test.go`

**Interfaces:**
- Consumes: nothing.
- Produces: TS `CT_RUBRIC`/`FULL_RUBRIC` (unchanged public API, now JSON-backed); Go `rubric.CT() Rubric`, `rubric.Rubric{ID,Name,Dimensions}`, `rubric.Dimension{ID,Name,Framework,Anchors map[string]string}`.

- [ ] **Step 1: Extract the seeded rubric to canonical JSON.**

Create `packages/contracts/src/ct-rubric.json` by transcribing the current `FULL_RUBRIC` array from `packages/contracts/src/rubric.ts` **verbatim** (all 10 dimensions D1–D10, each `{id, name, framework, anchors:{L1,L2,L3,L4}}`), wrapped as:

```json
{
  "id": "ct",
  "name": "AI 批判思维（9+1 维）",
  "dimensions": [
    { "id": "D1", "name": "提问清晰度", "framework": "ATL 思维 · QUEST-Q（输入）",
      "anchors": { "L1": "…", "L2": "…", "L3": "…", "L4": "…" } }
  ]
}
```

Copy the exact Chinese anchor strings and framework labels from `rubric.ts` lines 22–45 — do not paraphrase. The `id`/`name` are `CT_RUBRIC`'s (`"ct"` / `"AI 批判思维（9+1 维）"`).

- [ ] **Step 2: Point `rubric.ts` at the JSON, keep the public API.**

In `packages/contracts/src/rubric.ts`, replace the hand-written `FULL_RUBRIC` array literal and the `CT_RUBRIC` literal with a parse of the JSON (keep every other export — `SoloLevel`, `ScoredLevel`, `SOLO_LABELS`, the Zod `RubricDimension`/`Rubric` schemas, `assertRubricComplete`):

```ts
import ctRubricJson from "./ct-rubric.json";

export const CT_RUBRIC: Rubric = Rubric.parse(ctRubricJson);
export const FULL_RUBRIC: RubricDimension[] = CT_RUBRIC.dimensions;
assertRubricComplete(CT_RUBRIC);
```

If `RubricDimension`/`Rubric` are TS `interface`s (not Zod), add a Zod `Rubric` object schema mirroring them for the `.parse` (validate `id`, `name`, `dimensions[].{id,name,framework,anchors.L1..L4}` all strings). Keep the existing interface exports if other code imports them as types.

- [ ] **Step 3: Run the existing rubric test — must stay green.**

Run: `cd packages/contracts && npx vitest run test/rubric.test.ts`
Expected: PASS unchanged (10 dims D1–D10 in order, all L1–L4 anchors non-empty). This proves the extraction is byte-faithful. If it fails, the JSON diverged from the original — fix the JSON, not the test.

- [ ] **Step 4: Write the mirror tool.**

Create `apps/api/tools/syncrubric/main.go`, modeled on `apps/api/tools/syncskills/main.go`, copying the single file `../../packages/contracts/src/ct-rubric.json` → `internal/rubric/ct-rubric.json`:

```go
// Command syncrubric mirrors the canonical CT rubric JSON from
// packages/contracts/src/ct-rubric.json into internal/rubric so it can be
// embedded. It is the ONLY writer of the mirror; never hand-edit it.
package main

import (
	"fmt"
	"os"
)

const canonical = "../../packages/contracts/src/ct-rubric.json"
const mirror = "internal/rubric/ct-rubric.json"

func main() {
	data, err := os.ReadFile(canonical)
	if err != nil {
		fmt.Fprintln(os.Stderr, "syncrubric:", err)
		os.Exit(1)
	}
	if err := os.MkdirAll("internal/rubric", 0o755); err != nil {
		fmt.Fprintln(os.Stderr, "syncrubric:", err)
		os.Exit(1)
	}
	if err := os.WriteFile(mirror, data, 0o644); err != nil {
		fmt.Fprintln(os.Stderr, "syncrubric:", err)
		os.Exit(1)
	}
	fmt.Println("syncrubric: mirrored ct-rubric.json")
}
```

Add to `apps/api/Makefile` (extend `.PHONY` and add the target):

```makefile
# Mirror the canonical CT rubric JSON into the embeddable directory.
sync-rubric:
	go run ./tools/syncrubric
```

Run: `cd apps/api && make sync-rubric` — creates `internal/rubric/ct-rubric.json`.

- [ ] **Step 5: Write the Go rubric package + failing test.**

Create `apps/api/internal/rubric/rubric.go`:

```go
// Package rubric exposes the canonical CT critical-thinking rubric (the single
// source in packages/contracts/src/ct-rubric.json, mirrored here by
// `make sync-rubric` — never hand-edit ct-rubric.json).
package rubric

import (
	_ "embed"
	"encoding/json"
)

//go:embed ct-rubric.json
var ctJSON []byte

type Dimension struct {
	ID        string            `json:"id"`
	Name      string            `json:"name"`
	Framework string            `json:"framework"`
	Anchors   map[string]string `json:"anchors"` // keys L1..L4
}

type Rubric struct {
	ID         string      `json:"id"`
	Name       string      `json:"name"`
	Dimensions []Dimension `json:"dimensions"`
}

var ct = mustParse()

func mustParse() Rubric {
	var r Rubric
	if err := json.Unmarshal(ctJSON, &r); err != nil {
		panic("rubric: bad embedded ct-rubric.json: " + err.Error())
	}
	return r
}

// CT returns the parsed canonical CT rubric.
func CT() Rubric { return ct }
```

Create `apps/api/internal/rubric/rubric_test.go`:

```go
package rubric

import "testing"

func TestCTHasTenDimensionsInOrder(t *testing.T) {
	r := CT()
	if r.ID != "ct" {
		t.Fatalf("id = %q, want ct", r.ID)
	}
	want := []string{"D1", "D2", "D3", "D4", "D5", "D6", "D7", "D8", "D9", "D10"}
	if len(r.Dimensions) != len(want) {
		t.Fatalf("got %d dimensions, want %d", len(r.Dimensions), len(want))
	}
	for i, d := range r.Dimensions {
		if d.ID != want[i] {
			t.Errorf("dim %d = %q, want %q", i, d.ID, want[i])
		}
	}
}

func TestCTEveryDimensionHasFourAnchors(t *testing.T) {
	for _, d := range CT().Dimensions {
		if d.Name == "" || d.Framework == "" {
			t.Errorf("%s: empty name/framework", d.ID)
		}
		for _, lvl := range []string{"L1", "L2", "L3", "L4"} {
			if d.Anchors[lvl] == "" {
				t.Errorf("%s: empty %s anchor", d.ID, lvl)
			}
		}
	}
}
```

Run: `cd apps/api && CGO_ENABLED=0 go test ./internal/rubric/`
Expected: PASS (the embed + parse works, 10 dims, all anchors present).

- [ ] **Step 6: Commit.**

```bash
git add packages/contracts/src/ct-rubric.json packages/contracts/src/rubric.ts \
  apps/api/tools/syncrubric/main.go apps/api/Makefile \
  apps/api/internal/rubric/ct-rubric.json apps/api/internal/rubric/rubric.go \
  apps/api/internal/rubric/rubric_test.go
git commit -m "feat(refactor2): Slice 10 T1 — CT rubric single-source (contracts JSON + Go embed)"
```

---

### Task 2: Migration 0017 + evaluations project queries

**Files:**
- Create: `apps/api/internal/store/migrations/0017_assessment_project.sql`
- Create: `apps/api/internal/store/queries/evaluation.sql`
- Modify: generated `apps/api/internal/store/sqlc/*` (via `make sqlc`)
- Test: `apps/api/internal/store/migrate_0017_test.go` (Docker-backed)

**Interfaces:**
- Consumes: the `project` table (exists since 0016), the dormant `evaluations` table (0001).
- Produces: sqlc `InsertProjectEvaluation(ctx, InsertProjectEvaluationParams) (Evaluation, error)`, `GetLatestProjectEvaluation(ctx, projectID) (Evaluation, error)`.

- [ ] **Step 1: Write the migration.**

Create `apps/api/internal/store/migrations/0017_assessment_project.sql`:

```sql
-- +goose Up
ALTER TABLE evaluations ADD COLUMN project_id uuid REFERENCES project(id) ON DELETE CASCADE;
ALTER TABLE evaluations ALTER COLUMN task_id DROP NOT NULL;
ALTER TABLE evaluations ADD CONSTRAINT evaluations_scope_ck
  CHECK (task_id IS NOT NULL OR project_id IS NOT NULL);
CREATE INDEX evaluations_project_idx ON evaluations (project_id, created_at DESC);

-- +goose Down
DROP INDEX IF EXISTS evaluations_project_idx;
ALTER TABLE evaluations DROP CONSTRAINT IF EXISTS evaluations_scope_ck;
ALTER TABLE evaluations ALTER COLUMN task_id SET NOT NULL;
ALTER TABLE evaluations DROP COLUMN IF EXISTS project_id;
```

(Confirm the `project` table name against `0016_refactor2_foundations.sql` — it is `project` singular. Confirm `evaluations.task_id` FK cascade wording matches 0001 before writing the Down `SET NOT NULL`.)

- [ ] **Step 2: Write the queries.**

Create `apps/api/internal/store/queries/evaluation.sql`:

```sql
-- name: InsertProjectEvaluation :one
INSERT INTO evaluations (project_id, scores, narrative, model, tier,
  prompt_tokens, completion_tokens, cost_estimate, status)
VALUES (@project_id, @scores, @narrative, @model, @tier,
  @prompt_tokens, @completion_tokens, @cost_estimate, 'done')
RETURNING *;

-- name: GetLatestProjectEvaluation :one
SELECT * FROM evaluations
WHERE project_id = @project_id
ORDER BY created_at DESC
LIMIT 1;
```

- [ ] **Step 3: Regenerate sqlc.**

Run: `cd apps/api && make sqlc`
Expected: `internal/store/sqlc/evaluation.sql.go` generated with `InsertProjectEvaluation`/`GetLatestProjectEvaluation`; `Evaluation` model gains nullable `ProjectID` + `TaskID` becomes nullable (`pgtype.UUID`). No error.

- [ ] **Step 4: Write the Docker-backed migration test.**

Create `apps/api/internal/store/migrate_0017_test.go` — follow the existing migration-test harness in this package (find the helper that spins a testcontainers Postgres + runs goose up; reuse it, do not invent a new one). Assert:
1. A project-scoped insert works: seed a `project` (+ its owning user/school/class as the existing seed helper does), `InsertProjectEvaluation` with a `project_id`, `scores` jsonb, `narrative`, `status` returns a row with `TaskID` NULL and `ProjectID` set.
2. `GetLatestProjectEvaluation` returns the most recent row for that project.
3. A legacy task-scoped insert path still satisfies the CHECK (a raw `INSERT ... (task_id, ...)` with a seeded task succeeds — proving back-compat; if seeding a legacy task is heavy, instead assert the CHECK rejects a row with **both** ids NULL via a raw insert expecting an error).

Run: `cd apps/api && CGO_ENABLED=0 go test -p 1 ./internal/store/ -run 0017`
Expected: PASS. (Then run the FULL package before marking done — see gate.)

- [ ] **Step 5: Commit.**

```bash
git add apps/api/internal/store/migrations/0017_assessment_project.sql \
  apps/api/internal/store/queries/evaluation.sql \
  apps/api/internal/store/sqlc/ apps/api/internal/store/migrate_0017_test.go
git commit -m "feat(refactor2): Slice 10 T2 — evaluations.project_id migration + project eval queries"
```

---

### Task 3: Load the event stream into ProjectData

**Files:**
- Modify: `apps/api/internal/studio/projection.go` (`ProjectData` gains `Events`; add `Event` view type)
- Modify: `apps/api/internal/studio/load.go` (load `ListEventsByProject`)
- Test: `apps/api/internal/studio/load_events_test.go` (Docker-backed) OR extend `roundtrip_test.go`

**Interfaces:**
- Consumes: sqlc `ListEventsByProject` (exists), `ProjectData`.
- Produces: `studio.Event{Type, Surface string; Payload json.RawMessage; CreatedAt time.Time}`; `ProjectData.Events []Event` (ordered by created_at, id).

- [ ] **Step 1: Add the `Event` type + `ProjectData.Events` field.**

In `apps/api/internal/studio/projection.go`, add near the other view structs:

```go
// Event is one row of the append-only event stream, projected for the assessor.
// Payload is opaque; consumers read known keys defensively.
type Event struct {
	Type      string          `json:"type"`
	Surface   string          `json:"surface"`
	Payload   json.RawMessage `json:"payload"`
	CreatedAt time.Time       `json:"createdAt"`
}
```

Add `Events []Event` to the `ProjectData` struct. Do **not** touch `Project()` — the studio views don't consume events; the field is loaded for the assessment path and is inert here.

- [ ] **Step 2: Write the failing load test.**

Create `apps/api/internal/studio/load_events_test.go` (Docker-backed; reuse the seed helper `roundtrip_test.go` uses). Append two events for a seeded project (via the existing `AppendEvent` store method or a raw insert), then `Load` the project and assert `d.Events` has 2 entries in created_at order with the right `Type`/`Surface`.

Run: `cd apps/api && CGO_ENABLED=0 go test -p 1 ./internal/studio/ -run Events`
Expected: FAIL (Events not loaded yet).

- [ ] **Step 3: Load events in `load.go`.**

In `apps/api/internal/studio/load.go`, add a `ListEventsByProject` call and map rows → `[]Event`. Follow the existing load pattern for the other slices (e.g. how interventions/dispositions are loaded). Map `pgtype.Timestamptz` → `time.Time`, the sqlc `[]byte` payload → `json.RawMessage`.

Run: `cd apps/api && CGO_ENABLED=0 go test -p 1 ./internal/studio/ -run Events`
Expected: PASS.

- [ ] **Step 4: Run the full studio package (projection unaffected).**

Run: `cd apps/api && CGO_ENABLED=0 go test -p 1 ./internal/studio/`
Expected: PASS — existing projection/parity/roundtrip tests unchanged (events are inert to `Project()`).

- [ ] **Step 5: Commit.**

```bash
git add apps/api/internal/studio/projection.go apps/api/internal/studio/load.go \
  apps/api/internal/studio/load_events_test.go
git commit -m "feat(refactor2): Slice 10 T3 — load the event stream into ProjectData"
```

---

### Task 4: BuildAssessmentInput — the pure process-record digest

**Files:**
- Create: `apps/api/internal/agent/assess_input.go`
- Test: `apps/api/internal/agent/assess_input_test.go`

**Interfaces:**
- Consumes: primitive process-record slices (defined below — the handler extracts these from `ProjectData` at the call site, so this stays free of a `studio` import).
- Produces: `agent.AssessmentInput` + `agent.BuildAssessmentInput(...)`.

- [ ] **Step 1: Define the input types + write the failing test.**

Create `apps/api/internal/agent/assess_input_test.go`:

```go
package agent

import "testing"

func TestBuildAssessmentInputDigestsProcessRecord(t *testing.T) {
	in := BuildAssessmentInput(
		[]EventDigest{
			{Type: "card_surfaced", Order: 1, Text: "CRAAP 卡触发"},
			{Type: "card_completed", Order: 2, Text: "学生自发完成 SIFT"},
		},
		[]CardUse{{CardID: "sift", Dimension: "D3", Spont: "自发"}},
		[]DispositionUse{{Kind: "reject", Reason: "我不同意这条，因为原文语境不同"}},
		[]string{"S3 evaluate: solid", "S5 draft_polish: owed"},
		[]int{1780},
		[]string{"表D 熟练", "表E 发展中"},
		"claims:1 evidence:2 concession:1（钢人由学生撰写）",
	)
	if len(in.CardUses) != 1 || in.CardUses[0].Spont != "自发" {
		t.Fatalf("card uses not carried: %+v", in.CardUses)
	}
	if len(in.Timeline) != 2 || in.Timeline[0] != "1. card_surfaced：CRAAP 卡触发" {
		t.Fatalf("timeline order/format wrong: %+v", in.Timeline)
	}
	if in.SnapshotCount != 1 || len(in.WordCounts) != 1 {
		t.Fatalf("snapshot facts wrong: %+v", in)
	}
	if len(in.Dispositions) != 1 || in.Dispositions[0].Kind != "reject" {
		t.Fatalf("dispositions not carried")
	}
}

func TestBuildAssessmentInputEmptyProjectIsMinimal(t *testing.T) {
	in := BuildAssessmentInput(nil, nil, nil, nil, nil, nil, "")
	if len(in.Timeline) != 0 || in.SnapshotCount != 0 || in.GraphSummary != "" {
		t.Fatalf("empty project should digest to a minimal input: %+v", in)
	}
}
```

- [ ] **Step 2: Run it — fails to compile.**

Run: `cd apps/api && CGO_ENABLED=0 go test ./internal/agent/ -run BuildAssessmentInput`
Expected: FAIL (undefined: `BuildAssessmentInput`, types).

- [ ] **Step 3: Implement the digest.**

Create `apps/api/internal/agent/assess_input.go`:

```go
package agent

import "fmt"

// EventDigest is one already-projected event line the handler feeds the digest.
// Order is the event's position in the append-only stream (unprompted-first is
// legible from the ordering).
type EventDigest struct {
	Type  string
	Order int
	Text  string
}

// CardUse is one tool-card invocation with its 自发/提示后 initiative signal
// (reused from the studio equipment projection) and the CT dimension it exercises.
type CardUse struct {
	CardID    string
	Dimension string
	Spont     string // 自发 | 提示后
}

// DispositionUse is one accept/rewrite/reject decision on a coach intervention —
// the feedback-comprehension signal.
type DispositionUse struct {
	Kind   string // accept | rewrite | reject
	Reason string
}

// AssessmentInput is the compact, temporal, model-ready digest of one project's
// process record. Pure data; built by BuildAssessmentInput, consumed by Assess.
type AssessmentInput struct {
	CardUses      []CardUse
	Dispositions  []DispositionUse
	GateProgress  []string
	SnapshotCount int
	WordCounts    []int
	ReviewBands   []string
	GraphSummary  string
	Timeline      []string
}

// BuildAssessmentInput digests the process record. Pure — no I/O; the handler
// extracts the primitive slices from studio.ProjectData at the call site.
func BuildAssessmentInput(
	events []EventDigest,
	cards []CardUse,
	dispositions []DispositionUse,
	gates []string,
	wordCounts []int,
	reviewBands []string,
	graphSummary string,
) AssessmentInput {
	timeline := make([]string, 0, len(events))
	for _, e := range events {
		timeline = append(timeline, fmt.Sprintf("%d. %s：%s", e.Order, e.Type, e.Text))
	}
	return AssessmentInput{
		CardUses:      cards,
		Dispositions:  dispositions,
		GateProgress:  gates,
		SnapshotCount: len(wordCounts),
		WordCounts:    wordCounts,
		ReviewBands:   reviewBands,
		GraphSummary:  graphSummary,
		Timeline:      timeline,
	}
}
```

Run: `cd apps/api && CGO_ENABLED=0 go test ./internal/agent/ -run BuildAssessmentInput`
Expected: PASS.

- [ ] **Step 4: Commit.**

```bash
git add apps/api/internal/agent/assess_input.go apps/api/internal/agent/assess_input_test.go
git commit -m "feat(refactor2): Slice 10 T4 — BuildAssessmentInput process-record digest"
```

---

### Task 5: agent.Assess — the isolated flagship engine + anchor few-shot

**Files:**
- Create: `apps/api/internal/agent/assess.go`
- Create: `apps/api/internal/agent/assess_prompt.go`
- Create: `apps/api/internal/agent/anchors.json` (`go:embed`, backend-only few-shot)
- Test: `apps/api/internal/agent/assess_test.go`

**Interfaces:**
- Consumes: `rubric.CT()` (Task 1), `AssessmentInput` (Task 4), `gateway.{Provider,Resolved,Collect,ChatRequest}`, `enforcement.BannedPhrasing`.
- Produces: `agent.DimensionScore`, `agent.Assessment`, `agent.AnchorSample`, `agent.Assess(...)`.

- [ ] **Step 1: Author the anchor few-shot fixture.**

Create `apps/api/internal/agent/anchors.json` — an array of **1–2** compact samples in the exact output shape. Author a partial-Phoebe arc (real content, no lorem). Each sample: a short process digest + per-dimension `{code,name,level,evidence}` for a handful of dimensions + a narrative. Example shape (fill with real, coherent content spanning L1→L4 signals):

```json
[
  {
    "name": "Phoebe（节选·L1→L4 弧线）",
    "digest": "自发带入 NASA 与 Nature Sustainability 两个一手源；SIFT 横向验证 3 次；撞上'中国碳排放全球第一'反例后自发触发让步卡，钢人由本人撰写。",
    "dimensions": [
      {"code":"D2","name":"信源辨识","level":"L4","evidence":"主动交叉验证 NASA 与 Nature，识别二手转述并降级"},
      {"code":"D4","name":"多视角与让步","level":"L3","evidence":"撞反例后正面回应，构建让步段而非回避"},
      {"code":"D6","name":"反思与元认知","level":"L2","evidence":"事后回顾判断变化，但未主动校准信心"}
    ],
    "narrative": "你这次最大的跃迁在 S3：从依赖 AI 复述，转向自发横向验证并溯源。"
  }
]
```

- [ ] **Step 2: Write the failing engine test.**

Create `apps/api/internal/agent/assess_test.go` (reuse the `reviewProvider`/`gateway.NewStubProvider` stub pattern from `review_test.go`):

```go
package agent

import (
	"context"
	"strings"
	"testing"

	"mindimprint/api/internal/gateway"
	"mindimprint/api/internal/rubric"
)

func assessProvider(reply string) *gateway.StubProvider {
	return gateway.NewStubProvider([]gateway.StreamEvent{
		{Kind: gateway.EventTextDelta, TextDelta: reply},
		{Kind: gateway.EventUsage, Usage: &gateway.ChatUsage{InputTokens: 50, OutputTokens: 30}},
		{Kind: gateway.EventDone, StopReason: gateway.StopStop},
	})
}

func TestAssessParsesScoresAndNarrative(t *testing.T) {
	reply := `{"dimensions":[{"code":"D2","level":"L4","evidence":"交叉验证两个一手源"}],"narrative":"你这次最大的跃迁在 S3。"}`
	in := BuildAssessmentInput(nil, []CardUse{{CardID: "sift", Dimension: "D3", Spont: "自发"}}, nil, nil, []int{1780}, nil, "claims:1")
	a, usage, err := Assess(context.Background(), assessProvider(reply), gateway.Resolved{Provider: "deepseek", Model: "x"}, rubric.CT(), in, EmbeddedAnchors())
	if err != nil {
		t.Fatal(err)
	}
	if usage.OutputTokens != 30 {
		t.Errorf("usage not returned")
	}
	// Every rubric dimension is represented, in order; D2 lit, the rest NA.
	if len(a.Dimensions) != 10 || a.Dimensions[0].Code != "D1" {
		t.Fatalf("want 10 dims in order, got %d", len(a.Dimensions))
	}
	byCode := map[string]DimensionScore{}
	for _, d := range a.Dimensions {
		byCode[d.Code] = d
	}
	if byCode["D2"].Level != "L4" || byCode["D2"].Name != "信源辨识" {
		t.Errorf("D2 not scored/named: %+v", byCode["D2"])
	}
	if byCode["D5"].Level != "NA" {
		t.Errorf("unscored dim should be NA, got %q", byCode["D5"].Level)
	}
	if a.Narrative == "" {
		t.Errorf("narrative dropped")
	}
}

func TestAssessCoercesUnknownLevelToNA(t *testing.T) {
	reply := `{"dimensions":[{"code":"D1","level":"卓越","evidence":"x"}],"narrative":"n"}`
	a, _, err := Assess(context.Background(), assessProvider(reply), gateway.Resolved{}, rubric.CT(), AssessmentInput{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range a.Dimensions {
		if d.Code == "D1" && d.Level != "NA" {
			t.Errorf("unknown level should coerce to NA, got %q", d.Level)
		}
	}
}

func TestAssessRejectsBannedPhrasingWholeReport(t *testing.T) {
	// A narrative containing a ghostwriting imperative must reject the WHOLE report.
	reply := `{"dimensions":[{"code":"D1","level":"L2","evidence":"e"}],"narrative":"你应该这样写：中国在可持续发展上……"}`
	_, _, err := Assess(context.Background(), assessProvider(reply), gateway.Resolved{}, rubric.CT(), AssessmentInput{}, nil)
	if err == nil {
		t.Fatal("want banned-phrasing rejection, got nil")
	}
}

func TestAssessPromptCarriesRubricAndAnchors(t *testing.T) {
	// Prove the prompt includes a rubric ladder anchor + an anchor-sample name.
	// (Assert via a provider that captures the request — reuse the capture
	// pattern from coach_test.go if present; else assert indirectly through a
	// helper that builds the prompt string.)
}
```

For the banned-phrasing test to fire, confirm the `rewritten-sentence-zh` rule (你应该这样写…) exists in `enforcement/banned_phrasing.go` (it was added in Slice 8); if the exact trigger differs, use the real banned string from that file.

For `TestAssessPromptCarriesRubricAndAnchors`, prefer extracting the prompt builder into `assess_prompt.go` as a pure `assessSystemPrompt(rb, anchors)` + `assessUserInput(in)` so the test asserts on the returned strings directly (mirrors how `reviewSystemPrompt` is testable). Fill the test body to assert the system prompt contains a known D-anchor substring and the user/system contains the anchor sample name.

- [ ] **Step 3: Run it — fails to compile.**

Run: `cd apps/api && CGO_ENABLED=0 go test ./internal/agent/ -run Assess`
Expected: FAIL (undefined `Assess`, `DimensionScore`, `Assessment`, `AnchorSample`, `embeddedAnchors`).

- [ ] **Step 4: Implement the prompt builder.**

Create `apps/api/internal/agent/assess_prompt.go`:

```go
package agent

import (
	"fmt"
	"strings"

	"mindimprint/api/internal/rubric"
)

const assessPosture = `你是「思维印记」的过程评估者。你的职责是依据可观察的行为证据，判断学生在与 AI 协作中「怎么思考」，而不是给分数、不排名、不下结论式判决。
每个维度给出 L1–L4（或证据不足时 NA）以及支撑该等级的具体行为证据（引用过程记录里的真实动作）。最后给一段面向成长的叙事，指出这次最大的跃迁与下一步。
严禁替学生改写或撰写作文内容；只描述与诊断其思考路径。只输出 JSON。`

// assessSystemPrompt builds the system turn: posture + the rubric ladders + the
// anchor few-shot samples.
func assessSystemPrompt(rb rubric.Rubric, anchors []AnchorSample) string {
	var b strings.Builder
	b.WriteString(assessPosture)
	b.WriteString("\n\n评分维度与等级阶梯：\n")
	for _, d := range rb.Dimensions {
		b.WriteString(fmt.Sprintf("%s %s（%s）：L1 %s ｜ L2 %s ｜ L3 %s ｜ L4 %s\n",
			d.ID, d.Name, d.Framework,
			d.Anchors["L1"], d.Anchors["L2"], d.Anchors["L3"], d.Anchors["L4"]))
	}
	if len(anchors) > 0 {
		b.WriteString("\n参考样例（few-shot）：\n")
		for _, a := range anchors {
			b.WriteString(fmt.Sprintf("【%s】过程：%s\n", a.Name, a.Digest))
			for _, d := range a.Dimensions {
				b.WriteString(fmt.Sprintf("  %s=%s（%s）\n", d.Code, d.Level, d.Evidence))
			}
			b.WriteString("  叙事：" + a.Narrative + "\n")
		}
	}
	b.WriteString(`
输出格式（严格 JSON，dimensions 覆盖上述每个维度）：
{"dimensions":[{"code":"D1","level":"L1|L2|L3|L4|NA","evidence":"…"}],"narrative":"…"}`)
	return b.String()
}

// assessUserInput serialises the process-record digest as the user turn.
func assessUserInput(in AssessmentInput) string {
	var b strings.Builder
	b.WriteString("过程记录：\n")
	if len(in.Timeline) > 0 {
		b.WriteString("时间线：\n" + strings.Join(in.Timeline, "\n") + "\n")
	}
	for _, c := range in.CardUses {
		b.WriteString(fmt.Sprintf("工具卡：%s（维度 %s，%s）\n", c.CardID, c.Dimension, c.Spont))
	}
	for _, d := range in.Dispositions {
		b.WriteString(fmt.Sprintf("对反馈的处置：%s —— %s\n", d.Kind, d.Reason))
	}
	if len(in.GateProgress) > 0 {
		b.WriteString("关卡进度：" + strings.Join(in.GateProgress, "；") + "\n")
	}
	if in.SnapshotCount > 0 {
		b.WriteString(fmt.Sprintf("草稿快照：%d 次，字数 %v\n", in.SnapshotCount, in.WordCounts))
	}
	if len(in.ReviewBands) > 0 {
		b.WriteString("整稿体检：" + strings.Join(in.ReviewBands, "、") + "\n")
	}
	if in.GraphSummary != "" {
		b.WriteString("论证结构：" + in.GraphSummary + "\n")
	}
	return b.String()
}
```

- [ ] **Step 5: Implement the engine.**

Create `apps/api/internal/agent/assess.go`:

```go
package agent

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"strings"

	"mindimprint/api/internal/agent/enforcement"
	"mindimprint/api/internal/gateway"
	"mindimprint/api/internal/rubric"
)

//go:embed anchors.json
var anchorsJSON []byte

// DimensionScore is one CT dimension's level + its behavioral evidence (RL-5:
// diagnostic, never a grade).
type DimensionScore struct {
	Code     string `json:"code"`
	Name     string `json:"name"`
	Level    string `json:"level"` // L1..L4 | NA
	Evidence string `json:"evidence"`
}

// Assessment is the whole growth report: one score per rubric dimension + a
// growth narrative. No overall score, no rank (RL-5).
type Assessment struct {
	Dimensions []DimensionScore `json:"dimensions"`
	Narrative  string           `json:"narrative"`
}

// AnchorSample is one few-shot exemplar (backend-only prompt priming).
type AnchorSample struct {
	Name       string           `json:"name"`
	Digest     string           `json:"digest"`
	Dimensions []DimensionScore `json:"dimensions"`
	Narrative  string           `json:"narrative"`
}

// EmbeddedAnchors returns the backend-only few-shot fixture (prompt priming).
func EmbeddedAnchors() []AnchorSample {
	var a []AnchorSample
	_ = json.Unmarshal(anchorsJSON, &a) // fixture is authored + tested; ignore err in prod path
	return a
}

var validLevel = map[string]bool{"L1": true, "L2": true, "L3": true, "L4": true, "NA": true}

type assessWire struct {
	Dimensions []struct {
		Code     string `json:"code"`
		Level    string `json:"level"`
		Evidence string `json:"evidence"`
	} `json:"dimensions"`
	Narrative string `json:"narrative"`
}

// Assess makes ONE isolated flagship call scoring the CT rubric over the process
// digest, runs the full enforcement stack, and returns per-dimension scores + a
// growth narrative. Never in the coach loop; flagship, never downgraded.
func Assess(ctx context.Context, prov gateway.Provider, r gateway.Resolved, rb rubric.Rubric, in AssessmentInput, anchors []AnchorSample) (Assessment, gateway.ChatUsage, error) {
	res, err := gateway.Collect(ctx, prov, r, gateway.ChatRequest{
		Messages: []gateway.ChatMessage{
			{Role: gateway.RoleSystem, Content: assessSystemPrompt(rb, anchors)},
			{Role: gateway.RoleUser, Content: assessUserInput(in)},
		},
	})
	if err != nil {
		return Assessment{}, gateway.ChatUsage{}, err
	}
	usage := res.Usage

	var wire assessWire
	if err := json.Unmarshal([]byte(strings.TrimSpace(res.Text)), &wire); err != nil {
		return Assessment{}, usage, fmt.Errorf("agent: assessment output not JSON: %w", err)
	}

	// Enforcement: narrative + every evidence field. Any match rejects the whole report.
	fields := []string{wire.Narrative}
	for _, d := range wire.Dimensions {
		fields = append(fields, d.Evidence)
	}
	for _, f := range fields {
		if f == "" {
			continue
		}
		if rule := enforcement.BannedPhrasing(f); rule != nil {
			return Assessment{}, usage, fmt.Errorf("agent: assessment rejected by banned-phrasing rule %q", rule.Name)
		}
	}

	// Index the model's scores by code; emit every rubric dimension in order,
	// defaulting to NA (missing dim, or unknown level).
	got := map[string]struct{ level, evidence string }{}
	for _, d := range wire.Dimensions {
		lvl := d.Level
		if !validLevel[lvl] {
			lvl = "NA"
		}
		got[d.Code] = struct{ level, evidence string }{lvl, d.Evidence}
	}
	out := make([]DimensionScore, 0, len(rb.Dimensions))
	for _, dim := range rb.Dimensions {
		g, ok := got[dim.ID]
		if !ok {
			g = struct{ level, evidence string }{"NA", ""}
		}
		out = append(out, DimensionScore{Code: dim.ID, Name: dim.Name, Level: g.level, Evidence: g.evidence})
	}
	return Assessment{Dimensions: out, Narrative: wire.Narrative}, usage, nil
}
```

Run: `cd apps/api && CGO_ENABLED=0 go test ./internal/agent/ -run Assess`
Expected: PASS (all Assess tests, once the prompt test body is filled).

- [ ] **Step 6: Run the full agent package.**

Run: `cd apps/api && CGO_ENABLED=0 go test -p 1 ./internal/agent/`
Expected: PASS — no regression in classifier/coach/planner/review.

- [ ] **Step 7: Commit.**

```bash
git add apps/api/internal/agent/assess.go apps/api/internal/agent/assess_prompt.go \
  apps/api/internal/agent/anchors.json apps/api/internal/agent/assess_test.go
git commit -m "feat(refactor2): Slice 10 T5 — agent.Assess isolated engine + anchor few-shot"
```

---

### Task 6: Assessment endpoints + AssessmentDTO + parity

**Files:**
- Create: `apps/api/internal/api/assessment.go` (GET + POST handlers)
- Modify: `apps/api/internal/api/api.go` (register 2 routes)
- Create: `apps/api/internal/studio/assessment_dto.go` (`AssessmentDTO` + a pure `ToAssessmentDTO`)
- Modify: `apps/api/internal/studio/dto_parity_test.go` (add AssessmentDTO keys)
- Test: `apps/api/internal/api/assessment_test.go` (Docker-backed)

**Interfaces:**
- Consumes: `agent.Assess`, `agent.BuildAssessmentInput`, `rubric.CT()`, `studio.Load` + `ProjectData`, `HasEntitlement`, sqlc eval queries (Task 2), `store.RecordLLMCall`.
- Produces: `GET/POST /api/v1/projects/{id}/assessment` → `AssessmentDTO`; `studio.AssessmentDTO{Dimensions []AssessmentDimensionDTO; Narrative, GeneratedAt string}`.

- [ ] **Step 1: Define the DTO + mapper.**

Create `apps/api/internal/studio/assessment_dto.go`:

```go
package studio

import "mindimprint/api/internal/agent"

type AssessmentDimensionDTO struct {
	Code     string `json:"code"`
	Name     string `json:"name"`
	Level    string `json:"level"`
	Evidence string `json:"evidence"`
}

// AssessmentDTO is the growth report's own surface — NOT part of StudioProjection
// (the assessor is isolated). GeneratedAt is RFC3339.
type AssessmentDTO struct {
	Dimensions  []AssessmentDimensionDTO `json:"dimensions"`
	Narrative   string                   `json:"narrative"`
	GeneratedAt string                   `json:"generatedAt"`
}

func ToAssessmentDTO(a agent.Assessment, generatedAt string) AssessmentDTO {
	dims := make([]AssessmentDimensionDTO, 0, len(a.Dimensions))
	for _, d := range a.Dimensions {
		dims = append(dims, AssessmentDimensionDTO{Code: d.Code, Name: d.Name, Level: d.Level, Evidence: d.Evidence})
	}
	return AssessmentDTO{Dimensions: dims, Narrative: a.Narrative, GeneratedAt: generatedAt}
}
```

(If `studio` importing `agent` creates a cycle — check `go list -deps` — instead take primitive args in `ToAssessmentDTO` and map in the handler. Verify before committing.)

- [ ] **Step 2: Add the parity assertion + run it (RED).**

In `apps/api/internal/studio/dto_parity_test.go`, add a test asserting the JSON key set of `AssessmentDTO` = `{dimensions, narrative, generatedAt}` and `AssessmentDimensionDTO` = `{code, name, level, evidence}` (mirror the existing key-set assertion style in that file).

Run: `cd apps/api && CGO_ENABLED=0 go test ./internal/studio/ -run Parity`
Expected: FAIL until Step 1's struct compiles / PASS once it does. (This locks the wire shape before the Zod side in Task 7.)

- [ ] **Step 3: Write the failing handler test.**

Create `apps/api/internal/api/assessment_test.go` (Docker-backed; reuse the harness `writing`/`review` handler tests use — seeded project + auth). Assert:
1. **GET before any generate** → 200 with an empty/`null` assessment (no row) and **no `llm_call`** recorded.
2. **POST** (with a stub provider wired into the test app, as the review handler test does) → 200 with an `AssessmentDTO` whose `dimensions` cover all 10 codes; a row is persisted (`GetLatestProjectEvaluation` returns it); one `llm_call`/eval cost recorded.
3. **GET after POST** → 200 with the persisted assessment and **no new model call**.
4. **Ownership:** another user's token → 403/404.

(Find how the existing review handler test injects a stub provider + resolver into the test `deps`; reuse that exact wiring.)

Run: `cd apps/api && CGO_ENABLED=0 go test -p 1 ./internal/api/ -run Assessment`
Expected: FAIL (handlers/route not defined).

- [ ] **Step 4: Implement the handlers.**

Create `apps/api/internal/api/assessment.go` with `getAssessment` + `generateAssessment`, following `orderReview` in `writing.go` for the deps (`a.d.Queries`, `a.d.Pool`, `a.d.Provider`, `a.d.ChatResolver`, `agent.NewSqlcAgentStore`, `store.RecordLLMCall`), ownership check, and `HasEntitlement` before the model call. Sketch:

```go
func (a *App) getAssessment(w http.ResponseWriter, r *http.Request) {
	u := userFrom(r.Context())
	projectID := r.PathValue("id")
	// ownership check (reuse the helper orderReview uses)
	row, err := a.d.Queries.GetLatestProjectEvaluation(r.Context(), projectID)
	if errors.Is(err, pgx.ErrNoRows) {
		writeJSON(w, http.StatusOK, nil) // empty — the slot renders its empty state
		return
	}
	if err != nil { /* 500 */ }
	writeJSON(w, http.StatusOK, dtoFromEvaluationRow(row))
}

func (a *App) generateAssessment(w http.ResponseWriter, r *http.Request) {
	u := userFrom(r.Context())
	projectID := r.PathValue("id")
	// ownership check
	entitled, _ := HasEntitlement(r.Context(), u)
	if !entitled { /* 402/403 */ }

	d, err := studio.Load(r.Context(), a.d.Queries, projectID) // the same loader the studio view uses
	// map ProjectData -> agent primitives (cards w/ 自发/提示后 from the equipment
	// projection, dispositions, gate progress, snapshot word counts, review bands,
	// graphSummary) -> BuildAssessmentInput
	in := agent.BuildAssessmentInput(events, cards, disps, gates, wordCounts, bands, graphSummary(...))
	resolved, _ := a.d.ChatResolver(r.Context())
	assessment, usage, err := agent.Assess(r.Context(), a.d.Provider, resolved, rubric.CT(), in, agent.EmbeddedAnchors())
	store := agent.NewSqlcAgentStore(a.d.Queries, a.d.Pool)
	if resolved.Provider != "" {
		_ = store.RecordLLMCall(r.Context(), agent.LLMCallRow{ /* project, resolved, tokens, cost, kind:"assessment" */ })
	}
	if err != nil { /* enforcement rejection -> 422; other -> 500 */ }
	scoresJSON, _ := json.Marshal(assessment.Dimensions)
	row, err := a.d.Queries.InsertProjectEvaluation(r.Context(), sqlc.InsertProjectEvaluationParams{
		ProjectID: projectID, Scores: scoresJSON, Narrative: assessment.Narrative,
		Model: resolved.Model, Tier: resolved.Tier,
		PromptTokens: int32(usage.InputTokens), CompletionTokens: int32(usage.OutputTokens),
		CostEstimate: /* usage cost */,
	})
	writeJSON(w, http.StatusOK, dtoFromEvaluationRow(row))
}
```

Add a small `dtoFromEvaluationRow(row) studio.AssessmentDTO` helper (unmarshal `row.Scores` → `[]agent.DimensionScore`, `row.CreatedAt` → RFC3339). `agent.EmbeddedAnchors()` (exported in T5) reaches the fixture. Confirm exact field names on `sqlc.InsertProjectEvaluationParams` + the LLM-call `kind` enum against `RecordLLMCall`'s existing values (add an `"assessment"` kind if the column is a free text; if it's a CHECK-constrained enum, extend the CHECK in migration 0017 or reuse an allowed value — check `0001`/`llm_call` definition first).

Register the routes in `apps/api/internal/api/api.go` beside the review route:

```go
mux.Handle("GET /api/v1/projects/{id}/assessment", protected(a.getAssessment))
mux.Handle("POST /api/v1/projects/{id}/assessment", protected(a.generateAssessment))
```

Run: `cd apps/api && CGO_ENABLED=0 go test -p 1 ./internal/api/ -run Assessment`
Expected: PASS.

- [ ] **Step 5: Full packages (api + studio).**

Run: `cd apps/api && CGO_ENABLED=0 go test -p 1 ./internal/api/ ./internal/studio/`
Expected: PASS.

- [ ] **Step 6: Commit.**

```bash
git add apps/api/internal/api/assessment.go apps/api/internal/api/api.go \
  apps/api/internal/api/assessment_test.go apps/api/internal/studio/assessment_dto.go \
  apps/api/internal/studio/dto_parity_test.go apps/api/internal/agent/assess.go
git commit -m "feat(refactor2): Slice 10 T6 — assessment GET/POST endpoints + AssessmentDTO + parity"
```

---

### Task 7: Contracts Assessment schema + GrowthReport client

**Files:**
- Create: `packages/contracts/src/assessment.ts` (Zod `Assessment`/`DimensionScore`)
- Modify: `packages/contracts/src/index.ts` (export)
- Test: `packages/contracts/test/assessment.test.ts`
- Create: `apps/web/src/api/assessment.ts` (client) + `apps/web/src/api/assessment.test.ts`
- Create: `apps/web/src/shell/growth/GrowthReport.tsx` + `GrowthReport.test.tsx`
- Delete: `apps/web/src/shell/growth/GrowthPlaceholder.tsx` + `.test.tsx`
- Modify: `apps/web/src/shell/StudentApp.tsx` (mount `GrowthReport`)

**Interfaces:**
- Consumes: `AssessmentDTO` wire shape (Task 6), `SoloLevel`/`SOLO_LABELS` (from `rubric.ts`), the `growth` tab mount.
- Produces: Zod `Assessment`; `getAssessment(projectId)`, `generateAssessment(projectId)`; `GrowthReport`.

- [ ] **Step 1: Write the contracts schema + failing test.**

Create `packages/contracts/src/assessment.ts`:

```ts
import { z } from "zod";
import { SoloLevel } from "./rubric";

export const DimensionScore = z.object({
  code: z.string(),
  name: z.string(),
  level: SoloLevel,
  evidence: z.string(),
});
export type DimensionScore = z.infer<typeof DimensionScore>;

export const Assessment = z.object({
  dimensions: z.array(DimensionScore),
  narrative: z.string(),
  generatedAt: z.string(),
});
export type Assessment = z.infer<typeof Assessment>;
```

Export from `packages/contracts/src/index.ts`. Create `packages/contracts/test/assessment.test.ts`: a valid payload parses; a `level:"卓越"` (non-SoloLevel) is rejected; `dimensions:[]` + `narrative:""` + `generatedAt` parses (empty state shape).

Run: `cd packages/contracts && npx vitest run test/assessment.test.ts`
Expected: PASS (after Step 2 wiring it's green; if run before the export, fix the import path).

- [ ] **Step 2: Write the API client + test.**

Create `apps/web/src/api/assessment.ts`:

```ts
import { Assessment } from "@mindimprint/contracts";
import { apiFetch } from "./client";

export async function getAssessment(projectId: string): Promise<Assessment | null> {
  const raw = await apiFetch(`/api/v1/projects/${projectId}/assessment`);
  if (raw == null) return null;
  return Assessment.parse(raw);
}

export async function generateAssessment(projectId: string): Promise<Assessment> {
  const raw = await apiFetch(`/api/v1/projects/${projectId}/assessment`, { method: "POST" });
  return Assessment.parse(raw);
}
```

(Match the real helper names in `apps/web/src/api/client.ts` — use whatever `writing.ts`/`projects.ts` use for GET/POST + JSON parsing; the above is illustrative.) Create `apps/web/src/api/assessment.test.ts` mirroring `writing.test.ts` (mock fetch; assert GET maps a DTO, GET null → null, POST parses).

Run: `cd apps/web && npx vitest run src/api/assessment.test.ts`
Expected: PASS.

- [ ] **Step 3: Write the failing GrowthReport test.**

Create `apps/web/src/shell/growth/GrowthReport.test.tsx`. Cover:
1. renders a dimension row: the CT name (信源辨识), a level chip using `SOLO_LABELS` (L4 → 卓越), and the evidence quote;
2. an `NA` dimension renders muted (no fake level);
3. the growth narrative renders;
4. **empty state** (getAssessment → null): shows the "还没有成长报告" prompt + the 生成成长报告 button, and NO dimension rows / NO fake data;
5. the button triggers `generateAssessment`.

Use a mocked `getAssessment`/`generateAssessment` and a stubbed active-project resolution (`listProjects` → one project, same as `StudioContainer`).

Run: `cd apps/web && npx vitest run src/shell/growth/GrowthReport.test.tsx`
Expected: FAIL (component missing).

- [ ] **Step 4: Implement GrowthReport.**

Create `apps/web/src/shell/growth/GrowthReport.tsx`:
- On mount: resolve the active project (`listProjects()` → `[0].id`, guarding empty), then `getAssessment(projectId)`.
- Render per-dimension rows: `name`, a **level chip** (`SOLO_LABELS[level]` for L1–L4; a muted "证据不足 · NA" for NA — never a number/grade), and the `evidence` quote beneath. Then the growth `narrative` in its own block, framed as 你的思维印记.
- **Empty state:** honest prompt copy (「还没有成长报告 —— 完成一些思考后，点下方生成」) + the button. No placeholder rows.
- A **生成成长报告 / 重新生成** button → `generateAssessment` → set state; disabled + spinner label while pending.
- **RL-5 in the UI:** no total, no rank, no aggregate score anywhere; level chips are per-dimension diagnostic labels with their evidence. Icons inline SVG.
- Follow the design language of the retired `GrowthPlaceholder` card + the sibling views for spacing/typography.

Run: `cd apps/web && npx vitest run src/shell/growth/GrowthReport.test.tsx`
Expected: PASS.

- [ ] **Step 5: Mount it + delete the placeholder.**

In `apps/web/src/shell/StudentApp.tsx`: replace the `GrowthPlaceholder` import + `{tab === "growth" && <GrowthPlaceholder />}` with `GrowthReport`. Delete `GrowthPlaceholder.tsx` + `GrowthPlaceholder.test.tsx`.

Run: `cd apps/web && npx vitest run && npx tsc --noEmit`
Expected: web suite PASS (no dangling `GrowthPlaceholder` import); tsc clean.

- [ ] **Step 6: Full contracts + web gate.**

Run: `cd packages/contracts && npx vitest run` then `cd apps/web && npx vitest run && npx tsc --noEmit`
Expected: all green.

- [ ] **Step 7: Commit.**

```bash
git add packages/contracts/src/assessment.ts packages/contracts/src/index.ts \
  packages/contracts/test/assessment.test.ts \
  apps/web/src/api/assessment.ts apps/web/src/api/assessment.test.ts \
  apps/web/src/shell/growth/GrowthReport.tsx apps/web/src/shell/growth/GrowthReport.test.tsx \
  apps/web/src/shell/StudentApp.tsx
git rm apps/web/src/shell/growth/GrowthPlaceholder.tsx apps/web/src/shell/growth/GrowthPlaceholder.test.tsx
git commit -m "feat(refactor2): Slice 10 T7 — Assessment contract + GrowthReport replaces the placeholder"
```

---

## Final gate (before whole-branch review)

- Go: `cd apps/api && CGO_ENABLED=0 go test -p 1 ./...` — ALL packages `ok` on a quiet Docker (rubric, store, studio, agent, api). NOT `-run` subsets.
- Contracts: `cd packages/contracts && npx vitest run` — all green (rubric.test.ts unchanged + assessment.test.ts).
- Web: `cd apps/web && npx vitest run && npx tsc --noEmit` — all green + tsc clean.
- Sync drift: `cd apps/api && make sync-rubric` produces no diff (mirror in step with canonical).
- Git: only the named Slice-10 paths staged across the 7 commits; the pre-existing `M package.json` + untracked user files untouched.

## Whole-branch review

Dispatch the Opus whole-branch reviewer (`scripts/review-package <merge-base> HEAD`). Attention lens = the Global Constraints above, especially: RL-5 (no grade/rank anywhere — engine, DTO, or UI); the single-source rubric law (Go mirror is generated, `rubric.test.ts` byte-faithful); the `points`/`level`-style seam traced byte-consistent (rubric JSON → Go embed → Assess → evaluations.scores → AssessmentDTO → Zod → GrowthReport); banned-phrasing all-or-nothing; the isolation invariant (assessor never in the coach loop); GET makes no model call (llm_call-count assertion). Fix Critical/Important via ONE fix subagent with the full findings list; log Minors to the ledger.
