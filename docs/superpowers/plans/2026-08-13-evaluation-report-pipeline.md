# EvaluationReport v1 Pipeline — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the full `EvaluationReport` v1 pipeline end-to-end — shared Zod/Go contract, a new `evaluation_report` table with a read-no-call GET, a **placeholder generator that writes fake abundant data to the DB** (so the pipeline is demoable now and a colleague swaps in the real algorithm behind the same seam later), the new full-width report page (ported from the approved mockup), the `图鉴 → 评估` nav restructure, and teacher-end reuse.

**Architecture:** Clone the existing `assessment` + `mirror` pipeline. One canonical `EvaluationReport` object is the single payload (retiring the `DualAxisReport`/`mirror`/`summary` split for the student+teacher *process-evaluation* surface). The Zod contract in `@mind-imprint/contracts` is the source of truth; Go mirrors it with a hand-written struct + boundary validation (envelope pattern). The finish flow and a POST endpoint both call one `generateAndStoreEvaluationReport` core; today it calls the placeholder builder, later the real generator (same signature).

**Tech Stack:** TS + Zod (`packages/contracts`, raw source, no build step); Go (`net/http` + pgx + sqlc + goose, `apps/api`); React + Vite + TS + Tailwind (`apps/web`); PostgreSQL.

## Global Constraints

- **Read-no-call / write-only-spend.** `GET …/evaluation-report` and `GET …/evaluation-reports` read stored rows and NEVER call a model. Generation happens only on the finish goroutine or a `POST …:generate` (first-open-wins). (The placeholder spends nothing; the seam is preserved for the future flagship generator, which must resolve via `EvalResolver` and record an `llm_call`.)
- **Client never hits a model.** All generation is server-side.
- **Contract single-source.** The Zod schema in `packages/contracts/src/evaluationReport.ts` is authoritative; the Go struct mirrors it byte-for-byte (camelCase JSON tags) and Go only **boundary-validates** the envelope (`version`, ids, that array sections are arrays) — deep shape truth stays in Zod.
- **Structure = locked §4 + the Option-A additions only.** Per `docs/superpowers/specs/2026-08-13-evaluation-report-structure-design.md`. Option-A additions (the ONLY additions): evidence items gain `quote` / `observation` / `boundary` (+ `stage`); materials gain `finalStatus` / `cannotSupport`. No other new fields.
- **Content depth.** Every field filled abundantly to the depth of `docs/reference/2026-08-13-report-example-{detail,simple}.md`. Signature = the recurring **边界/cannot-support** framing. Prompts are **observation, never graded** (`不分档`).
- **Finish semantics unchanged.** The finish trigger (`finishProject` guards → `SetProjectEvaluating` → detached goroutine → `SetProjectFinished`, rollback to `SetProjectActive` on failure, `202 {"status":"evaluating"}`) stays mechanically identical — only the artifact generated inside the goroutine changes. **Verify against `docs/2026-08-09-all-statuses.md` (retrospective/finish section) before implementing Task C2.**
- **Design system.** Type scale = `tailwind.config.ts` `mk-*` ONLY (display 32 / h1 24 / h2 18 / h3 16 / body-lg 16 / body 14 / small 12 / label 11-uppercase). No off-scale sizes. No student-facing text < 12px. Object cards = 10px radius + `shadow-mk-md`, no border; list rows = hairline (8px + border + `shadow-mk-xs`). Warm-paper ground, accent 随人, 7 macarons, Lucide icons. **Never** `bg-mk-<token>/<opacity>` (renders transparent) — use solid tokens.
- **Visual reference (concrete, not a placeholder):** `docs/reference/2026-08-13-eval-report-mockup.html` — the approved report page. React components port its markup/CSS.
- **PDF export is OUT OF SCOPE** for this plan (deferred, separate follow-up). The `导出 PDF` button renders disabled with a `title="即将上线"`.
- **Teacher end renders the same `EvaluationReport` object/component** as the student.
- **Go tests need `-timeout 1800s`.** Never `git add -A` (use `git add -u` + explicit paths). sqlc regen: pin `sqlc @v1.27.0`, `CGO_ENABLED=0` on macOS.

---

## File structure

**Contract (shared):**
- Create `packages/contracts/src/evaluationReport.ts` — Zod schemas + inferred types.
- Modify `packages/contracts/src/index.ts` — add `export * from "./evaluationReport";`.
- Create `packages/contracts/test/evaluationReport.test.ts` — parse test.

**Backend (`apps/api`):**
- Create `internal/store/migrations/0064_evaluation_report.sql` — table.
- Create `internal/store/queries/evaluation_report.sql` — Insert / GetLatest / List; regenerate into `internal/store/sqlc/evaluation_report.sql.go`.
- Create `internal/evalreport/report.go` — Go mirror struct.
- Create `internal/evalreport/validate.go` — envelope boundary validation.
- Create `internal/evalreport/placeholder.go` — the placeholder builder (fake abundant data).
- Create `internal/evalreport/placeholder_test.go`, `validate_test.go`.
- Create `internal/api/evaluation_report.go` — GET read/list handlers, POST generate, the `generateAndStoreEvaluationReport` core, teacher read.
- Create `internal/api/evaluation_report_test.go`.
- Modify `internal/api/project_finish.go` — `runProjectReport` calls the new core.
- Modify `internal/api/api.go` — register routes.

**Frontend (`apps/web/src`):**
- Create `api/evaluationReport.ts` — client fns.
- Create `shell/report/EvaluationReport/` — `index.tsx`, `Ruler.tsx`, `Header.tsx`, `Abstract.tsx`, `Timeline.tsx`, `Materials.tsx`, `AxisPanel.tsx`, `PromptLens.tsx`, `ToolUsage.tsx`, `Risks.tsx`, `tokens.ts` (ramps/kind colors), `__fixtures__/mock.ts`.
- Create `shell/report/EvaluationReportPage.tsx` — fetch container.
- Create `shell/assessment/AssessmentView.tsx` — 评估 tab: Segmented [成长报告 | 图鉴] + timeline.
- Modify `shell/Nav.tsx` (label/icon), `shell/StudentApp.tsx` (gallery→评估 restructure).
- Modify `console/TeacherReportView.tsx`, `api/teacher.ts` — teacher reuse.
- Tests colocated `*.test.tsx`.

---

## Task 1: Zod contract `evaluationReport.ts`

**Files:**
- Create: `packages/contracts/src/evaluationReport.ts`
- Modify: `packages/contracts/src/index.ts`
- Test: `packages/contracts/test/evaluationReport.test.ts`

**Interfaces — Produces:** the Zod schema object `EvaluationReport` (has `.parse`) + inferred type `EvaluationReport`, importable as `import { EvaluationReport } from "@mind-imprint/contracts"`. Sub-types: `Ref`, `EvidenceItem`, `EventEntry`, `MaterialEntry`, `DepthDimResult`, `AutonomyDimResult`, `PromptItem`, `ToolUsageEntry`, `RiskEntry`.

- [ ] **Step 1: Write the failing test**

```ts
// packages/contracts/test/evaluationReport.test.ts
import { describe, it, expect } from "vitest";
import { EvaluationReport } from "../src/evaluationReport";

const MIN = {
  version: 1, reportId: "rep-1", projectId: "p-1",
  student: { id: "u-1", name: "Phoebe" },
  basics: {
    title: "T", type: "Extended Essay", startDate: "2026-08-01T00:00:00Z", endDate: null,
    milestones: { started: "2026-08-01T00:00:00Z", frameworkFinished: null, proposalFinished: null, writingFinished: null, projectFinished: null },
    counters: { aiTurns: 0, materialsRead: 0, wordsWritten: 0, aiCommentCount: 0, editCount: 0 },
  },
  abstract: {
    overview: "o", materialSentence: "m", writingSentence: "w", aiSentence: "a",
    suggestionParagraph: "s", suggestionSentences: ["s1"], recommendedCourses: [{ courseId: "c-1", reason: "r" }],
  },
  events: [{ ts: "2026-08-01T00:00:00Z", kind: "chat", summary: "s", aiTurns: 2 }],
  materials: [{ materialId: "m-1", addedAt: "2026-08-01T00:00:00Z", source: "NASA", url: null, usedIn: null, finalStatus: "bridge source", comment: "c", cannotSupport: "x" }],
  depth: [{ id: "D1", level: 3, summary: "s", evidence: [{ id: "msg-1", ts: "2026-08-01T00:00:00Z", stage: "立题", quote: "q", observation: "obs", boundary: "b" }], suggestion: "sg" }],
  autonomy: [{ id: "A1", band: 4, summary: "s", evidence: [], suggestion: "sg" }],
  promptLens: { summary: "s", prompts: [{ stage: "写作", quote: "q", ref: { id: "msg-2" }, observation: "o", relatedDomains: ["A3", "D5"], attention: false }] },
  toolUsage: [{ toolId: "card:craap", name: "CRAAP", stage: "阅读", purpose: "p", summary: "s" }],
  risks: [{ type: "data-scope", behaviour: "b", suggestion: "sg" }],
  generatedAt: "2026-08-06T00:00:00Z",
};

describe("EvaluationReport", () => {
  it("parses a minimal valid report", () => {
    expect(() => EvaluationReport.parse(MIN)).not.toThrow();
  });
  it("rejects a bad depth level", () => {
    const bad = structuredClone(MIN); bad.depth[0].level = 5;
    expect(() => EvaluationReport.parse(bad)).toThrow();
  });
  it("rejects an unknown risk type", () => {
    const bad = structuredClone(MIN); (bad.risks[0] as any).type = "nope";
    expect(() => EvaluationReport.parse(bad)).toThrow();
  });
});
```

- [ ] **Step 2: Run it to confirm it fails**

Run: `cd packages/contracts && npx vitest run test/evaluationReport.test.ts`
Expected: FAIL — cannot find `../src/evaluationReport`.

- [ ] **Step 3: Write the contract**

```ts
// packages/contracts/src/evaluationReport.ts
import { z } from "zod";

export const Ref = z.object({
  id: z.string(),
  label: z.string().optional(),
  ts: z.string().optional(),
}).strict();
export type Ref = z.infer<typeof Ref>;

// Option-A rich evidence item: quote is lifted verbatim from a chat_message;
// observation/boundary are short model prose. id = message/event id (G5).
export const EvidenceItem = z.object({
  id: z.string(),
  ts: z.string().optional(),
  stage: z.string().optional(),
  quote: z.string(),
  observation: z.string(),
  boundary: z.string().optional(),
}).strict();
export type EvidenceItem = z.infer<typeof EvidenceItem>;

const Milestones = z.object({
  started: z.string().nullable(),
  frameworkFinished: z.string().nullable(),
  proposalFinished: z.string().nullable(),
  writingFinished: z.string().nullable(),
  projectFinished: z.string().nullable(),
}).strict();

const Counters = z.object({
  aiTurns: z.number().int().nonnegative(),
  materialsRead: z.number().int().nonnegative(),
  wordsWritten: z.number().int().nonnegative(),
  aiCommentCount: z.number().int().nonnegative(),
  editCount: z.number().int().nonnegative(),
}).strict();

const Basics = z.object({
  title: z.string(),
  type: z.string(),
  startDate: z.string(),
  endDate: z.string().nullable(),
  milestones: Milestones,
  counters: Counters,
}).strict();

const RecommendedCourse = z.object({ courseId: z.string(), reason: z.string() }).strict();

const Abstract = z.object({
  overview: z.string(),
  materialSentence: z.string(),
  writingSentence: z.string(),
  aiSentence: z.string(),
  suggestionParagraph: z.string(),
  suggestionSentences: z.array(z.string()),
  recommendedCourses: z.array(RecommendedCourse),
}).strict();

export const EventKind = z.enum(["chat", "reading", "graph", "writing", "review", "milestone"]);
export const EventEntry = z.object({
  ts: z.string(),
  kind: EventKind,
  summary: z.string(),
  aiTurns: z.number().int().nonnegative(),
  ref: Ref.optional(),
}).strict();
export type EventEntry = z.infer<typeof EventEntry>;

// Option-A: material gains finalStatus + cannotSupport.
export const MaterialEntry = z.object({
  materialId: z.string(),
  addedAt: z.string(),
  source: z.string(),
  url: z.string().nullable(),
  usedIn: Ref.nullable(),
  finalStatus: z.string(),
  comment: z.string(),
  cannotSupport: z.string(),
}).strict();
export type MaterialEntry = z.infer<typeof MaterialEntry>;

export const DepthDimResult = z.object({
  id: z.enum(["D1", "D2", "D3", "D4", "D5", "D6"]),
  level: z.number().int().min(1).max(4),
  summary: z.string(),
  evidence: z.array(EvidenceItem),
  suggestion: z.string(),
}).strict();
export type DepthDimResult = z.infer<typeof DepthDimResult>;

export const AutonomyDimResult = z.object({
  id: z.enum(["A1", "A2", "A3", "A4", "A5", "A6"]),
  band: z.number().int().min(0).max(5),
  summary: z.string(),
  evidence: z.array(EvidenceItem),
  suggestion: z.string(),
}).strict();
export type AutonomyDimResult = z.infer<typeof AutonomyDimResult>;

export const PromptItem = z.object({
  stage: z.string(),
  quote: z.string(),
  ref: Ref,
  observation: z.string(),
  relatedDomains: z.array(z.string()),
  attention: z.boolean(),
}).strict();
export type PromptItem = z.infer<typeof PromptItem>;

const PromptLens = z.object({
  summary: z.string(),
  prompts: z.array(PromptItem),
}).strict();

export const ToolUsageEntry = z.object({
  toolId: z.string(),
  name: z.string(),
  stage: z.string(),
  purpose: z.string(),
  summary: z.string(),
}).strict();
export type ToolUsageEntry = z.infer<typeof ToolUsageEntry>;

export const RiskType = z.enum([
  "ai-ghostwrite", "missing-source", "argument-logic", "data-scope", "rabbit-hole-offtopic",
]);
export const RiskEntry = z.object({
  type: RiskType,
  behaviour: z.string(),
  ref: Ref.optional(),
  suggestion: z.string(),
}).strict();
export type RiskEntry = z.infer<typeof RiskEntry>;

export const EvaluationReport = z.object({
  version: z.literal(1),
  reportId: z.string(),
  projectId: z.string(),
  student: z.object({ id: z.string(), name: z.string() }).strict(),
  basics: Basics,
  abstract: Abstract,
  events: z.array(EventEntry),
  materials: z.array(MaterialEntry),
  depth: z.array(DepthDimResult),
  autonomy: z.array(AutonomyDimResult),
  promptLens: PromptLens,
  toolUsage: z.array(ToolUsageEntry),
  risks: z.array(RiskEntry),
  generatedAt: z.string(),
}).strict();
export type EvaluationReport = z.infer<typeof EvaluationReport>;
```

Then append to `packages/contracts/src/index.ts`:

```ts
export * from "./evaluationReport";
```

- [ ] **Step 4: Run the test to confirm it passes**

Run: `cd packages/contracts && npx vitest run test/evaluationReport.test.ts`
Expected: PASS (3 tests).

- [ ] **Step 5: Typecheck**

Run: `cd packages/contracts && npx tsc --noEmit`
Expected: no errors.

- [ ] **Step 6: Commit**

```bash
git add packages/contracts/src/evaluationReport.ts packages/contracts/src/index.ts packages/contracts/test/evaluationReport.test.ts
git commit -m "feat(contracts): add EvaluationReport v1 Zod contract"
```

---

## Task 2: Go mirror struct + envelope validation `internal/evalreport`

**Files:**
- Create: `apps/api/internal/evalreport/report.go`
- Create: `apps/api/internal/evalreport/validate.go`
- Test: `apps/api/internal/evalreport/validate_test.go`

**Interfaces — Consumes:** the field shapes from Task 1. **Produces:** `evalreport.Report` (JSON-tagged struct, camelCase matching the Zod contract byte-for-byte) and `func Validate(raw []byte) (Report, error)` — unmarshals + boundary-checks the envelope. Used by Tasks 4/5/6.

- [ ] **Step 1: Write the failing test**

```go
// apps/api/internal/evalreport/validate_test.go
package evalreport

import "testing"

func TestValidate_RejectsWrongVersion(t *testing.T) {
	_, err := Validate([]byte(`{"version":2,"reportId":"r","projectId":"p"}`))
	if err == nil {
		t.Fatal("expected error for version != 1")
	}
}

func TestValidate_AcceptsMinimalEnvelope(t *testing.T) {
	raw := []byte(`{"version":1,"reportId":"r","projectId":"p","student":{"id":"u","name":"P"},` +
		`"basics":{"title":"t","type":"EE","startDate":"x","endDate":null,` +
		`"milestones":{"started":null,"frameworkFinished":null,"proposalFinished":null,"writingFinished":null,"projectFinished":null},` +
		`"counters":{"aiTurns":0,"materialsRead":0,"wordsWritten":0,"aiCommentCount":0,"editCount":0}},` +
		`"abstract":{"overview":"","materialSentence":"","writingSentence":"","aiSentence":"","suggestionParagraph":"","suggestionSentences":[],"recommendedCourses":[]},` +
		`"events":[],"materials":[],"depth":[],"autonomy":[],"promptLens":{"summary":"","prompts":[]},"toolUsage":[],"risks":[],"generatedAt":"x"}`)
	r, err := Validate(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.ReportID != "r" || r.ProjectID != "p" {
		t.Fatalf("ids not parsed: %+v", r)
	}
}
```

- [ ] **Step 2: Run to confirm it fails**

Run: `cd apps/api && go test ./internal/evalreport/ -run TestValidate -timeout 1800s`
Expected: FAIL — package/struct not defined.

- [ ] **Step 3: Write the struct + validator**

Write `report.go` mirroring the Zod contract field-for-field with camelCase json tags. Follow the pattern in `apps/api/internal/studio/report_dto.go` (hand-written, "Mirrors packages/contracts/... byte-for-byte"). Structure:

```go
// apps/api/internal/evalreport/report.go
package evalreport

type Ref struct {
	ID    string `json:"id"`
	Label string `json:"label,omitempty"`
	TS    string `json:"ts,omitempty"`
}

type EvidenceItem struct {
	ID          string `json:"id"`
	TS          string `json:"ts,omitempty"`
	Stage       string `json:"stage,omitempty"`
	Quote       string `json:"quote"`
	Observation string `json:"observation"`
	Boundary    string `json:"boundary,omitempty"`
}

type Milestones struct {
	Started           *string `json:"started"`
	FrameworkFinished *string `json:"frameworkFinished"`
	ProposalFinished  *string `json:"proposalFinished"`
	WritingFinished   *string `json:"writingFinished"`
	ProjectFinished   *string `json:"projectFinished"`
}

type Counters struct {
	AITurns        int `json:"aiTurns"`
	MaterialsRead  int `json:"materialsRead"`
	WordsWritten   int `json:"wordsWritten"`
	AICommentCount int `json:"aiCommentCount"`
	EditCount      int `json:"editCount"`
}

type Basics struct {
	Title      string     `json:"title"`
	Type       string     `json:"type"`
	StartDate  string     `json:"startDate"`
	EndDate    *string    `json:"endDate"`
	Milestones Milestones `json:"milestones"`
	Counters   Counters   `json:"counters"`
}

type RecommendedCourse struct {
	CourseID string `json:"courseId"`
	Reason   string `json:"reason"`
}

type Abstract struct {
	Overview            string              `json:"overview"`
	MaterialSentence    string              `json:"materialSentence"`
	WritingSentence     string              `json:"writingSentence"`
	AISentence          string              `json:"aiSentence"`
	SuggestionParagraph string              `json:"suggestionParagraph"`
	SuggestionSentences []string            `json:"suggestionSentences"`
	RecommendedCourses  []RecommendedCourse `json:"recommendedCourses"`
}

type EventEntry struct {
	TS      string `json:"ts"`
	Kind    string `json:"kind"`
	Summary string `json:"summary"`
	AITurns int    `json:"aiTurns"`
	Ref     *Ref   `json:"ref,omitempty"`
}

type MaterialEntry struct {
	MaterialID    string `json:"materialId"`
	AddedAt       string `json:"addedAt"`
	Source        string `json:"source"`
	URL           *string `json:"url"`
	UsedIn        *Ref   `json:"usedIn"`
	FinalStatus   string `json:"finalStatus"`
	Comment       string `json:"comment"`
	CannotSupport string `json:"cannotSupport"`
}

type DepthDimResult struct {
	ID         string         `json:"id"`
	Level      int            `json:"level"`
	Summary    string         `json:"summary"`
	Evidence   []EvidenceItem `json:"evidence"`
	Suggestion string         `json:"suggestion"`
}

type AutonomyDimResult struct {
	ID         string         `json:"id"`
	Band       int            `json:"band"`
	Summary    string         `json:"summary"`
	Evidence   []EvidenceItem `json:"evidence"`
	Suggestion string         `json:"suggestion"`
}

type PromptItem struct {
	Stage          string   `json:"stage"`
	Quote          string   `json:"quote"`
	Ref            Ref      `json:"ref"`
	Observation    string   `json:"observation"`
	RelatedDomains []string `json:"relatedDomains"`
	Attention      bool     `json:"attention"`
}

type PromptLens struct {
	Summary string       `json:"summary"`
	Prompts []PromptItem `json:"prompts"`
}

type ToolUsageEntry struct {
	ToolID  string `json:"toolId"`
	Name    string `json:"name"`
	Stage   string `json:"stage"`
	Purpose string `json:"purpose"`
	Summary string `json:"summary"`
}

type RiskEntry struct {
	Type      string `json:"type"`
	Behaviour string `json:"behaviour"`
	Ref       *Ref   `json:"ref,omitempty"`
	Suggestion string `json:"suggestion"`
}

type Report struct {
	Version   int    `json:"version"`
	ReportID  string `json:"reportId"`
	ProjectID string `json:"projectId"`
	Student   struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	} `json:"student"`
	Basics     Basics              `json:"basics"`
	Abstract   Abstract            `json:"abstract"`
	Events     []EventEntry        `json:"events"`
	Materials  []MaterialEntry     `json:"materials"`
	Depth      []DepthDimResult    `json:"depth"`
	Autonomy   []AutonomyDimResult `json:"autonomy"`
	PromptLens PromptLens          `json:"promptLens"`
	ToolUsage  []ToolUsageEntry    `json:"toolUsage"`
	Risks      []RiskEntry         `json:"risks"`
	GeneratedAt string             `json:"generatedAt"`
}
```

```go
// apps/api/internal/evalreport/validate.go
package evalreport

import (
	"encoding/json"
	"fmt"
)

// Validate unmarshals and boundary-checks the envelope. Deep-shape truth lives
// in packages/contracts (Zod); Go only guards the outer envelope.
func Validate(raw []byte) (Report, error) {
	var r Report
	if err := json.Unmarshal(raw, &r); err != nil {
		return Report{}, fmt.Errorf("evalreport: unmarshal: %w", err)
	}
	if r.Version != 1 {
		return Report{}, fmt.Errorf("evalreport: version must be 1, got %d", r.Version)
	}
	if r.ReportID == "" || r.ProjectID == "" {
		return Report{}, fmt.Errorf("evalreport: missing reportId/projectId")
	}
	if r.Student.ID == "" {
		return Report{}, fmt.Errorf("evalreport: missing student.id")
	}
	return r, nil
}
```

- [ ] **Step 4: Run to confirm it passes**

Run: `cd apps/api && go test ./internal/evalreport/ -run TestValidate -timeout 1800s`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add apps/api/internal/evalreport/report.go apps/api/internal/evalreport/validate.go apps/api/internal/evalreport/validate_test.go
git commit -m "feat(api): EvaluationReport Go mirror struct + envelope validation"
```

---

## Task 3: Migration + sqlc queries for `evaluation_report`

**Files:**
- Create: `apps/api/internal/store/migrations/0064_evaluation_report.sql`
- Create: `apps/api/internal/store/queries/evaluation_report.sql`
- Regenerate: `apps/api/internal/store/sqlc/evaluation_report.sql.go` (via sqlc)

**Interfaces — Produces:** sqlc methods `InsertEvaluationReport(ctx, InsertEvaluationReportParams) (EvaluationReport, error)`, `GetLatestEvaluationReport(ctx, pgtype.UUID) (EvaluationReport, error)`, `ListEvaluationReports(ctx, uuid.UUID) ([]ListEvaluationReportsRow, error)`. The generated row model `EvaluationReport` has `Report []byte` (jsonb).

> ⚠️ Note the name collision: the sqlc-generated **row model** will be `sqlc.EvaluationReport` (from the table) and the contract type is `evalreport.Report`. Keep them distinct in imports.

- [ ] **Step 1: Write the migration**

```sql
-- apps/api/internal/store/migrations/0064_evaluation_report.sql
-- +goose Up
CREATE TABLE evaluation_report (
  id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  project_id  uuid NOT NULL REFERENCES project(id) ON DELETE CASCADE,
  version     int  NOT NULL DEFAULT 1,
  report      jsonb NOT NULL,
  created_at  timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX evaluation_report_project_created_idx
  ON evaluation_report (project_id, created_at DESC);

-- +goose Down
DROP TABLE evaluation_report;
```

- [ ] **Step 2: Write the queries**

```sql
-- apps/api/internal/store/queries/evaluation_report.sql

-- name: InsertEvaluationReport :one
INSERT INTO evaluation_report (project_id, version, report)
VALUES (@project_id, @version, @report)
RETURNING *;

-- name: GetLatestEvaluationReport :one
SELECT * FROM evaluation_report
WHERE project_id = @project_id
ORDER BY created_at DESC
LIMIT 1;

-- name: ListEvaluationReports :many
-- Timeline for one student: newest report per finished project they own.
SELECT DISTINCT ON (er.project_id)
  er.project_id, er.created_at,
  p.title, p.qualification
FROM evaluation_report er
JOIN project p ON p.id = er.project_id
WHERE p.user_id = @user_id
ORDER BY er.project_id, er.created_at DESC;
```

- [ ] **Step 3: Regenerate sqlc**

Run (from repo root; follow the repo's existing sqlc invocation — check `apps/api/Makefile` / `sqlc.yaml`):
```bash
cd apps/api && go run github.com/sqlc-dev/sqlc/cmd/sqlc@v1.27.0 generate
```
Expected: `internal/store/sqlc/evaluation_report.sql.go` created with the three methods; `models.go` gains `EvaluationReport` row struct with `Report []byte`.

- [ ] **Step 4: Verify it compiles**

Run: `cd apps/api && go build ./...`
Expected: builds clean.

- [ ] **Step 5: Commit**

```bash
git add apps/api/internal/store/migrations/0064_evaluation_report.sql apps/api/internal/store/queries/evaluation_report.sql apps/api/internal/store/sqlc/
git commit -m "feat(api): evaluation_report table + sqlc queries (insert/get/list)"
```

---

## Task 4: Placeholder generator `internal/evalreport/placeholder.go`

**Files:**
- Create: `apps/api/internal/evalreport/placeholder.go`
- Test: `apps/api/internal/evalreport/placeholder_test.go`

**Interfaces — Consumes:** `evalreport.Report` (Task 2). **Produces:** `func Placeholder(projectID, reportID, studentID, studentName, title, ptype, generatedAt string) Report` — returns a **full, abundant, deterministic fake** report (the Phoebe / 中国 greening content). Later swapped for a real generator with the same return type. Deterministic (no `time.Now`, no randomness) so it's testable and resume-safe.

> **Content source:** encode the SAME abundant content as the frontend fixture `apps/web/src/shell/report/EvaluationReport/__fixtures__/mock.ts` (Task 9) and the approved mockup `docs/reference/2026-08-13-eval-report-mockup.html`. All 9 sections filled to depth: abstract (overview + 3 sentences + suggestion¶ + 3–4 sentences + 2 courses), ~7 events, 8 materials (each with `finalStatus` + `comment` + `cannotSupport`), D1–D6 (each 2–3 `EvidenceItem` with `quote`/`observation`/`boundary` + suggestion), A1–A6 (same), promptLens (summary + 4 prompts, `attention` on the video one), 6 toolUsage, 6 risks.

- [ ] **Step 1: Write the failing test**

```go
// apps/api/internal/evalreport/placeholder_test.go
package evalreport

import (
	"encoding/json"
	"testing"
)

func TestPlaceholder_IsAbundantAndValid(t *testing.T) {
	r := Placeholder("p-1", "rep-1", "u-1", "Phoebe", "中国是否让地球变得更可持续？", "Extended Essay", "2026-08-06T00:00:00Z")

	if len(r.Depth) != 6 || len(r.Autonomy) != 6 {
		t.Fatalf("want 6 D + 6 A dims, got %d/%d", len(r.Depth), len(r.Autonomy))
	}
	if len(r.Materials) < 6 {
		t.Fatalf("want >=6 materials, got %d", len(r.Materials))
	}
	if len(r.Events) < 5 {
		t.Fatalf("want >=5 events, got %d", len(r.Events))
	}
	for _, d := range r.Depth {
		if len(d.Evidence) == 0 || d.Evidence[0].Quote == "" || d.Evidence[0].Boundary == "" {
			t.Fatalf("dim %s lacks rich evidence", d.ID)
		}
	}
	for _, m := range r.Materials {
		if m.CannotSupport == "" || m.FinalStatus == "" {
			t.Fatalf("material %s lacks cannotSupport/finalStatus", m.MaterialID)
		}
	}
	// round-trips through Validate
	raw, _ := json.Marshal(r)
	if _, err := Validate(raw); err != nil {
		t.Fatalf("placeholder output failed Validate: %v", err)
	}
}

func TestPlaceholder_Deterministic(t *testing.T) {
	a := Placeholder("p-1", "rep-1", "u-1", "Phoebe", "T", "EE", "2026-08-06T00:00:00Z")
	b := Placeholder("p-1", "rep-1", "u-1", "Phoebe", "T", "EE", "2026-08-06T00:00:00Z")
	ra, _ := json.Marshal(a)
	rb, _ := json.Marshal(b)
	if string(ra) != string(rb) {
		t.Fatal("placeholder is not deterministic")
	}
}
```

- [ ] **Step 2: Run to confirm it fails**

Run: `cd apps/api && go test ./internal/evalreport/ -run TestPlaceholder -timeout 1800s`
Expected: FAIL — `Placeholder` undefined.

- [ ] **Step 3: Implement `Placeholder`**

Build and return a `Report` literal. `Version:1`; `ReportID/ProjectID/Student/GeneratedAt` from args; `Basics.Title/Type` from args; the rest = the abundant fixed content (port verbatim from the mock fixture / mockup). Skeleton (fill ALL sections to the depth listed above — this abbreviated skeleton shows the shape; the implementer completes every list):

```go
// apps/api/internal/evalreport/placeholder.go
package evalreport

func strptr(s string) *string { return &s }

// Placeholder returns a full, abundant, deterministic fake EvaluationReport.
// Replace this with the real generation algorithm behind the same signature.
func Placeholder(projectID, reportID, studentID, studentName, title, ptype, generatedAt string) Report {
	r := Report{
		Version: 1, ReportID: reportID, ProjectID: projectID, GeneratedAt: generatedAt,
	}
	r.Student.ID = studentID
	r.Student.Name = studentName
	r.Basics = Basics{
		Title: title, Type: ptype,
		StartDate: "2026-08-01T09:00:00Z", EndDate: strptr("2026-08-06T16:30:00Z"),
		Milestones: Milestones{
			Started: strptr("2026-08-01T09:00:00Z"), FrameworkFinished: strptr("2026-08-02T11:00:00Z"),
			ProposalFinished: strptr("2026-08-03T14:00:00Z"), WritingFinished: strptr("2026-08-05T15:00:00Z"),
			ProjectFinished: strptr("2026-08-06T16:30:00Z"),
		},
		Counters: Counters{AITurns: 612, MaterialsRead: 9, WordsWritten: 812, AICommentCount: 23, EditCount: 18},
	}
	r.Abstract = Abstract{
		Overview: "Phoebe 呈现出一条相当完整的 AI 协作写作证据链……", // full paragraph from mockup
		MaterialSentence: "共 9 条来源，为每条记录了打开理由、来源层级与数据口径……",
		WritingSentence:  "18 次打磨中不仅改措辞，更把 prove / largest polluter……",
		AISentence:       "让 AI 递工具卡、做追问、检查越界、模拟答辩……",
		SuggestionParagraph: "把 source log 做得更标准化——每条来源都固定填入……",
		SuggestionSentences: []string{
			"如果继续扩展：可单独开一段讨论 per-capita / historical emissions……",
			"若重新加入 MEE：必须补齐真实 URL、打开理由、摘录与来源功能……",
			"若更严谨地使用视频：保留观看时间点、人物、场景……",
			"若要把反方写得更强：先承认 CO2/WUE 窄机制……",
		},
		RecommendedCourses: []RecommendedCourse{
			{CourseID: "course:source-triage", Reason: "你已能分配来源功能，这门课把……变成可复用的标准。"},
			{CourseID: "course:data-scope", Reason: "巩固 annual share / 历史累计 / 人均 的区分……"},
		},
	}
	r.Events = []EventEntry{
		{TS: "2026-08-01T09:05:00Z", Kind: "chat", Summary: "从微信公众号标题进入，提取可追源关键词，自建 source 字段……", AITurns: 24, Ref: &Ref{ID: "message:t1", Label: "研究问题成形", TS: "2026-08-01T09:20:00Z"}},
		// … 追源 / 补证 / 反方与兔子洞 / 写作 / 复盘 Review / 答辩与提交 (kind ∈ chat|reading|graph|writing|review|milestone)
	}
	r.Materials = []MaterialEntry{
		{MaterialID: "material:wechat", AddedAt: "2026-08-01T09:05:00Z", Source: "微信公众号", URL: nil, UsedIn: nil,
			FinalStatus: "origin only", Comment: "真实看到的第一条材料……她识别出「源头」一词会制造强因果感，把它降级为追源入口。",
			CannotSupport: "不能证明 NASA 原意、中国单独贡献或整体可持续。"},
		// … NASA Ames / Nature / ESSD+OWID / Economist+WorldBank / co2science / Exxon(rabbit-hole log) / MEE(candidate removed)
	}
	r.Depth = []DepthDimResult{
		{ID: "D1", Level: 3, Summary: "问题从「这真的假的、中国是不是主角」逐步收窄为 to what extent 的三维框架。",
			Evidence: []EvidenceItem{
				{ID: "message:t003", TS: "2026-08-01T09:20:00Z", Stage: "真实起点", Quote: "公众号说的地球变绿是真的吗？中国到底是不是主角？", Observation: "用人话说出最初问题，不先套研究框架。"},
				{ID: "message:t179", Stage: "研究问题形成", Quote: "To what extent has China contributed to planetary sustainability…", Observation: "在多轮追源之后才形成正式 RQ。", Boundary: "不是 AI 一开始给出三维框架。"},
			},
			Suggestion: "下次可显式写出被排除的子问题及原因，让问题边界更透明。"},
		// … D2..D6 (levels reflect the mockup color bars: D2=4, D3=3, D4=3, D5=3, D6=4)
	}
	r.Autonomy = []AutonomyDimResult{
		{ID: "A1", Band: 3, Summary: "关键方向多次由她提出：自己搜 NASA、自己补排放、要求不假造反方。",
			Evidence: []EvidenceItem{
				{ID: "message:t022", Stage: "真实起点", Quote: "我不是让 AI 给我 NASA 链接，而是自己用公众号里的线索搜。", Observation: "检索路线归属清楚。", Boundary: "AI 可帮术语，不替代打开路径。"},
			}, Suggestion: "把「为什么走这条线」也记一句，让方向选择可追溯。"},
		// … A2..A6 (bands: A2=4, A3=4, A4=3, A5=4, A6=3)
	}
	r.PromptLens = PromptLens{
		Summary: "提示词本身不分档。真正能成立为过程证据的，是提示词之后是否产生了来源表、草稿修改、Review 记录或答辩回应……",
		Prompts: []PromptItem{
			{Stage: "真实起点", Quote: "我刚刷到一篇微信公众号文章…这个真的假的？", Ref: Ref{ID: "message:t002"}, Observation: "暴露真实材料起点与初始问题，不是让 AI 直接写文章。", RelatedDomains: []string{"D1", "A1"}, Attention: false},
			{Stage: "写作过程", Quote: "我先不开全文，请你只检查写作计划，不替我写。", Ref: Ref{ID: "message:t285"}, Observation: "把 AI 限定为计划检查者。", RelatedDomains: []string{"A3", "D5"}, Attention: false},
			{Stage: "Review", Quote: "先请你只检查 claim 有没有越界，不润色。", Ref: Ref{ID: "message:t389"}, Observation: "把 AI 放在审阅者位置。", RelatedDomains: []string{"D3", "A5"}, Attention: false},
			{Stage: "视频处理", Quote: "你能不能帮我总结视频。", Ref: Ref{ID: "message:t148"}, Observation: "自然的求助点；AI 无法真正观看视频，她随后自己看完并记录——风险被她自己化解。", RelatedDomains: []string{"A3"}, Attention: true},
		},
	}
	r.ToolUsage = []ToolUsageEntry{
		{ToolID: "card:sift", Name: "SIFT 溯源", Stage: "真实起点 · 反方", Purpose: "停止判断、提取关键词、追源、横向阅读", Summary: "帮助她从公众号标题进入 NASA、Nature 与 co2science 身份检查。"},
		// … CRAAP / 数据三问 / 论证解剖warrant / 审阅子代理 / 阅读室子代理
	}
	r.Risks = []RiskEntry{
		{Type: "missing-source", Behaviour: "MEE 因缺少具体 URL 和打开理由被移出正文。", Ref: &Ref{ID: "message:t461"}, Suggestion: "报告中写成「候选 / 未采用」，下次补齐 URL、打开理由与摘录再纳入。"},
		{Type: "data-scope", Behaviour: "早期把 annual share、net leaf-area increase 口径混用了一次。", Ref: &Ref{ID: "message:t098"}, Suggestion: "陈述排放时固定口径，先声明是总量还是人均。"},
		// … argument-logic / rabbit-hole-offtopic / ai-ghostwrite (cover all 5 enum values across ≥6 entries)
	}
	return r
}
```

- [ ] **Step 4: Complete every list to the full mockup depth, then run the tests**

Run: `cd apps/api && go test ./internal/evalreport/ -timeout 1800s`
Expected: PASS (all 4 tests including round-trip Validate + determinism).

- [ ] **Step 5: Commit**

```bash
git add apps/api/internal/evalreport/placeholder.go apps/api/internal/evalreport/placeholder_test.go
git commit -m "feat(api): placeholder EvaluationReport generator (fake abundant data)"
```

---

## Task 5: Read + list handlers + generate core + routes

**Files:**
- Create: `apps/api/internal/api/evaluation_report.go`
- Modify: `apps/api/internal/api/api.go`
- Test: `apps/api/internal/api/evaluation_report_test.go`

**Interfaces — Consumes:** `evalreport.Placeholder`/`Validate` (Tasks 2/4), sqlc `InsertEvaluationReport`/`GetLatestEvaluationReport`/`ListEvaluationReports` (Task 3), `a.loadOwnedProject` (`projects.go:132`), `a.d.Queries` (`*sqlc.Queries`). **Produces:** `generateAndStoreEvaluationReport(ctx, projectID uuid.UUID) error` (the write core, called by finish + POST), and handlers `getEvaluationReport`, `listEvaluationReports`, `postGenerateEvaluationReport`.

- [ ] **Step 1: Write the handler + core**

```go
// apps/api/internal/api/evaluation_report.go
package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"mindimprint/api/internal/evalreport"
	"mindimprint/api/internal/httpx"
	"mindimprint/api/internal/store/sqlc"
)

// getEvaluationReport: read-no-call. Returns the latest stored report or JSON null.
func (a *API) getEvaluationReport(w http.ResponseWriter, r *http.Request) {
	projectID, ok := a.loadOwnedProject(w, r)
	if !ok {
		return
	}
	row, err := a.d.Queries.GetLatestEvaluationReport(r.Context(), pgtype.UUID{Bytes: projectID, Valid: true})
	if errors.Is(err, pgx.ErrNoRows) {
		httpx.WriteJSON(w, http.StatusOK, nil)
		return
	}
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	rep, verr := evalreport.Validate(row.Report)
	if verr != nil {
		httpx.WriteError(w, r, verr)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, rep)
}

// listEvaluationReports: the student's report timeline (newest per project).
func (a *API) listEvaluationReports(w http.ResponseWriter, r *http.Request) {
	u, ok := UserFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, r, httpx.ErrUnauthorized("未登录"))
		return
	}
	rows, err := a.d.Queries.ListEvaluationReports(r.Context(), u.ID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	type entry struct {
		ProjectID string `json:"projectId"`
		Title     string `json:"title"`
		Type      string `json:"type"`
		CreatedAt string `json:"createdAt"`
	}
	out := make([]entry, 0, len(rows))
	for _, row := range rows {
		out = append(out, entry{
			ProjectID: uuid.UUID(row.ProjectID.Bytes).String(),
			Title:     row.Title,
			Type:      row.Qualification,
			CreatedAt: row.CreatedAt.Time.Format("2006-01-02T15:04:05Z07:00"),
		})
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"entries": out})
}

// postGenerateEvaluationReport: first-open-wins generate (placeholder era = no spend).
func (a *API) postGenerateEvaluationReport(w http.ResponseWriter, r *http.Request) {
	projectID, ok := a.loadOwnedProject(w, r)
	if !ok {
		return
	}
	if _, err := a.d.Queries.GetLatestEvaluationReport(r.Context(), pgtype.UUID{Bytes: projectID, Valid: true}); err == nil {
		a.getEvaluationReport(w, r) // already exists — return it, no spend
		return
	}
	if err := a.generateAndStoreEvaluationReport(r.Context(), projectID); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	a.getEvaluationReport(w, r)
}

// generateAndStoreEvaluationReport is the write core (finish goroutine + POST).
// TODAY: the placeholder. LATER: resolve a.d.EvalResolver + call the real
// agent generator + RecordLLMCall, keeping this signature.
func (a *API) generateAndStoreEvaluationReport(ctx context.Context, projectID uuid.UUID) error {
	p, err := a.d.Queries.GetProject(ctx, projectID)
	if err != nil {
		return err
	}
	rep := evalreport.Placeholder(
		projectID.String(),
		uuid.NewString(),
		uuid.UUID(p.UserID.Bytes).String(), // adjust to actual UserID type on the project row
		"", // student name — fill from users table if desired; empty is acceptable for placeholder
		p.Title,
		p.Qualification,
		nowRFC3339(), // helper: time.Now().UTC().Format(time.RFC3339) — placeholder may pass "" if determinism required
	)
	raw, err := json.Marshal(rep)
	if err != nil {
		return err
	}
	_, err = a.d.Queries.InsertEvaluationReport(ctx, sqlc.InsertEvaluationReportParams{
		ProjectID: pgtype.UUID{Bytes: projectID, Valid: true},
		Version:   1,
		Report:    raw,
	})
	return err
}
```

> Adjust `p.UserID`/`p.Title`/`p.Qualification` field access to the actual `sqlc.Project` model. Provide `nowRFC3339()` in this file or reuse an existing time helper; if the project already has a `nowRFC3339`-style helper, reuse it (grep). For a fully deterministic placeholder in tests, the finish path may pass a fixed timestamp — acceptable.

- [ ] **Step 2: Register routes in `api.go`**

Near the existing `GET …/assessment` registration (`api.go:193`), add:

```go
mux.Handle("GET /api/v1/projects/{id}/evaluation-report", protected(a.getEvaluationReport))
mux.Handle("POST /api/v1/projects/{id}/evaluation-report/generate", protected(a.postGenerateEvaluationReport))
mux.Handle("GET /api/v1/evaluation-reports", protected(a.listEvaluationReports))
```

- [ ] **Step 3: Write a handler test (testcontainers, following the existing api test harness)**

```go
// apps/api/internal/api/evaluation_report_test.go
package api

import (
	"net/http"
	"testing"
)

// Uses the package's existing test harness (seeded project + authed client).
// Follow the pattern in assessment_test.go / the api test helpers.
func TestEvaluationReport_GenerateThenRead(t *testing.T) {
	env := newTestEnv(t)          // <- reuse the existing harness constructor
	defer env.Close()
	pid := env.seedFinishedProject(t) // <- reuse existing seed helper (or seed a project)

	// initially null
	if got := env.getJSON(t, "/api/v1/projects/"+pid+"/evaluation-report"); got != "null" {
		t.Fatalf("want null before generate, got %s", got)
	}
	// generate
	env.post(t, "/api/v1/projects/"+pid+"/evaluation-report/generate", nil, http.StatusOK)
	// now present + abundant
	rep := env.getReport(t, "/api/v1/projects/"+pid+"/evaluation-report")
	if len(rep.Depth) != 6 || len(rep.Materials) < 6 {
		t.Fatalf("report not abundant: %+v", rep)
	}
	// list includes it
	if !env.listContains(t, "/api/v1/evaluation-reports", pid) {
		t.Fatal("timeline missing the generated report")
	}
}
```

> Adapt the harness calls (`newTestEnv`, `seedFinishedProject`, `getJSON`, `post`, `getReport`, `listContains`) to whatever the existing `internal/api` test suite actually provides — read `assessment_test.go` first and mirror its setup exactly. Do NOT invent a new harness.

- [ ] **Step 4: Run the tests**

Run: `cd apps/api && go test ./internal/api/ -run TestEvaluationReport -timeout 1800s`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add apps/api/internal/api/evaluation_report.go apps/api/internal/api/api.go apps/api/internal/api/evaluation_report_test.go
git commit -m "feat(api): evaluation-report read/list/generate endpoints + routes"
```

---

## Task 6: Wire the finish flow to the new generator

**Files:**
- Modify: `apps/api/internal/api/project_finish.go` (`runProjectReport`, `:115`)

**Interfaces — Consumes:** `generateAndStoreEvaluationReport` (Task 5). **Preserves:** the status-flip + rollback semantics exactly (Global Constraints).

> **Before implementing:** verify the retrospective/finish section of `docs/2026-08-09-all-statuses.md` — the trigger, guards, and status transitions must stay identical; only the artifact generated inside the goroutine changes.

- [ ] **Step 1: Replace the report-generation body of `runProjectReport`**

Change the goroutine so it generates the new EvaluationReport instead of the old dual-axis report + mirror. Keep the `WithUser(context.Background(), u)` context, the rollback-to-active on failure, the finished flip, and the `project_finished` event. New body:

```go
func (a *API) runProjectReport(u User, projectID uuid.UUID) {
	ctx := WithUser(context.Background(), u)
	store := agent.NewSqlcAgentStore(a.d.Queries, a.d.Pool)

	if err := a.generateAndStoreEvaluationReport(ctx, projectID); err != nil {
		// rollback: leave the task retryable
		if aerr := a.d.Queries.SetProjectActive(ctx, projectID); aerr != nil {
			// log; nothing else to do
		}
		return
	}
	if err := a.d.Queries.SetProjectFinished(ctx, projectID); err != nil {
		return
	}
	store.AppendEvent(ctx, agent.EventRow{
		ProjectID: projectID, Surface: "studio", Type: "project_finished",
		Payload: mustJSON(map[string]any{"generatedAt": nowRFC3339()}),
	})
}
```

> Keep `errAssessmentRejected` handling only if you keep the old generator around; for the placeholder there is no reject path, so a plain error → rollback is correct. Do NOT delete the old `generateProjectReport`/`composeAndStoreProjectMirror` functions in this task (other surfaces — parent report — may still reference the old pipeline); they simply stop being called from finish. A later cleanup task can remove them.

- [ ] **Step 2: Build + run the finish-related tests**

Run: `cd apps/api && go test ./internal/api/ -run 'Finish|EvaluationReport' -timeout 1800s`
Expected: PASS. If an existing finish test asserts a mirror/assessment row is written, update it to assert an `evaluation_report` row instead (the finish artifact changed by design).

- [ ] **Step 3: Commit**

```bash
git add apps/api/internal/api/project_finish.go
git commit -m "feat(api): finish flow generates EvaluationReport (placeholder) instead of dual-axis+mirror"
```

---

## Task 7: Frontend API client `evaluationReport.ts`

**Files:**
- Create: `apps/web/src/api/evaluationReport.ts`
- Test: `apps/web/src/api/evaluationReport.test.ts`

**Interfaces — Consumes:** `EvaluationReport` from `@mind-imprint/contracts`, the app's `apiFetch` helper (grep `apps/web/src/api/*.ts` for the shared fetch, e.g. used in `assessment.ts`). **Produces:**
- `getEvaluationReport(projectId: string): Promise<EvaluationReport | null>`
- `generateEvaluationReport(projectId: string): Promise<EvaluationReport | null>`
- `listEvaluationReports(): Promise<EvalReportListEntry[]>` where `EvalReportListEntry = { projectId: string; title: string; type: string; createdAt: string }`.

- [ ] **Step 1: Write the client** (mirror `api/assessment.ts:1-15` for the fetch+parse idiom):

```ts
// apps/web/src/api/evaluationReport.ts
import { EvaluationReport } from "@mind-imprint/contracts";
import { apiFetch } from "./client"; // adjust to the real shared fetch module

export interface EvalReportListEntry {
  projectId: string;
  title: string;
  type: string;
  createdAt: string;
}

export async function getEvaluationReport(projectId: string): Promise<EvaluationReport | null> {
  const raw = await apiFetch(`/api/v1/projects/${projectId}/evaluation-report`);
  return raw == null ? null : EvaluationReport.parse(raw);
}

export async function generateEvaluationReport(projectId: string): Promise<EvaluationReport | null> {
  const raw = await apiFetch(`/api/v1/projects/${projectId}/evaluation-report/generate`, { method: "POST" });
  return raw == null ? null : EvaluationReport.parse(raw);
}

export async function listEvaluationReports(): Promise<EvalReportListEntry[]> {
  const raw = (await apiFetch("/api/v1/evaluation-reports")) as { entries: EvalReportListEntry[] };
  return raw.entries ?? [];
}
```

- [ ] **Step 2: Test the parse path** (mock `apiFetch`, assert null-passthrough + parse). Run: `cd apps/web && npx vitest run src/api/evaluationReport.test.ts`. Expected: PASS.

- [ ] **Step 3: Commit**

```bash
git add apps/web/src/api/evaluationReport.ts apps/web/src/api/evaluationReport.test.ts
git commit -m "feat(web): evaluation-report api client"
```

---

## Task 8: Report component — shared tokens + fixture + Ruler shell

**Files:**
- Create: `apps/web/src/shell/report/EvaluationReport/tokens.ts`
- Create: `apps/web/src/shell/report/EvaluationReport/__fixtures__/mock.ts`
- Create: `apps/web/src/shell/report/EvaluationReport/Ruler.tsx`
- Create: `apps/web/src/shell/report/EvaluationReport/index.tsx`
- Test: `apps/web/src/shell/report/EvaluationReport/index.test.tsx`

**Interfaces — Produces:** `<EvaluationReportView report={report} />` (prop: `report: EvaluationReport`) composing the section components (Tasks 8–11) with the ruler 目录. `tokens.ts` exports the D/A color ramps + event-kind colors + risk-type labels. `mock.ts` exports `MOCK_EVALUATION_REPORT: EvaluationReport` (the full abundant Phoebe fixture — same content as the Go placeholder Task 4).

> **Port markup/CSS from `docs/reference/2026-08-13-eval-report-mockup.html`.** Use `mk-*` type-scale utilities and the Card/hairline surfaces. The ruler = the `.ruler` block; scroll-sync via `IntersectionObserver` exactly as the mockup's `<script>`.

- [ ] **Step 1** — `tokens.ts`: export `DEPTH_RAMP = ["#CDE7E1","#8FCEC1","#4FB0A0","#177368"]` (level 1–4), `AUTONOMY_RAMP = ["#EFEAF6","#DACFEC","#C3B0E1","#A98FD3","#8A6AC0","#5B4A80"]` (band 0–5), `EVENT_KIND` map (`chat→mist`, `reading→lake`, `graph→taro`, `writing→matcha`, `review→berry`, `milestone→accent`) with `{dot, fg, bg, label}`, and `RISK_LABEL` map (the 5 enum → 中文标签). Values from `apps/web/src/ui/tokens.ts` MACARONS/SEMANTIC.
- [ ] **Step 2** — `mock.ts`: the full fixture (all 9 sections, abundant). This is the canonical TS fixture the Go placeholder mirrors.
- [ ] **Step 3** — `Ruler.tsx`: sticky ruler with the 9 items; `IntersectionObserver` sets the active tick; click scrolls to section id. Props: `{ sections: { id: string; idx: string; label: string }[] }`.
- [ ] **Step 4** — `index.tsx`: `EvaluationReportView({ report }: { report: EvaluationReport })` — the `.report` grid (ruler + `.body`), rendering `<Header/> <Abstract/> <Timeline/> <Materials/> <AxisPanel axis="depth"/> <AxisPanel axis="autonomy"/> <PromptLens/> <ToolUsage/> <Risks/>` in order, each wrapped in a `<section id="sN">`. (Import the section components from Tasks 9–11; until those exist, stub them to render their `data-testid` container so this task's test is green, then fill in.)
- [ ] **Step 5** — test `index.test.tsx`: render `<EvaluationReportView report={MOCK_EVALUATION_REPORT} />`, assert the report title, all 9 ruler labels, and one deep value per section (e.g. a D1 quote, a material `cannotSupport`) appear. Run: `cd apps/web && npx vitest run src/shell/report/EvaluationReport/index.test.tsx`. Expected: PASS.
- [ ] **Step 6: Commit**

```bash
git add apps/web/src/shell/report/EvaluationReport/
git commit -m "feat(web): EvaluationReport view shell — tokens, fixture, ruler, composition"
```

---

## Task 9: Report sections — Header, Abstract, Timeline

**Files:** Create `Header.tsx`, `Abstract.tsx`, `Timeline.tsx` under `apps/web/src/shell/report/EvaluationReport/`; test `sections-a.test.tsx`.

**Interfaces — Consumes:** `report.basics` / `report.abstract` / `report.events`, `tokens.ts` (Task 8). **Produces:** `<Header basics={report.basics} title={report.basics.title} />`, `<Abstract abstract={report.abstract} />`, `<Timeline events={report.events} />`.

- [ ] **Step 1** — `Header.tsx`: report title (`mk-h1`), type chip + date range, the **milestone stepper** (5 steps from `basics.milestones`; a step is "done" when its timestamp is non-null; render date `MM-DD`), and the **5 counter tiles** (`basics.counters`, macaron-tinted Lucide icons). Port `.rpt-head`/`.steps`/`.counters` from the mockup.
- [ ] **Step 2** — `Abstract.tsx`: overview (`mk-body-lg`, render `**…**` emphasis via a small `renderEmphasis(text)` helper that splits on `**` and wraps odd segments in `<em class="hl">`), the 3 macaron one-sentence cards, the accent callout (`suggestionParagraph`), the check-list (`suggestionSentences`), and the recommended-course chips (`recommendedCourses`).
- [ ] **Step 3** — `Timeline.tsx`: vertical timeline; per event a color-coded kind dot (`EVENT_KIND[kind]`), the `summary`, the `ts` (format), and an `AI ×{aiTurns}` badge.
- [ ] **Step 4** — test: render each with a fixture slice; assert the milestone dates, a counter value, the emphasis `<em>` renders, and an event kind label. Run vitest. Expected: PASS.
- [ ] **Step 5: Commit** (`git add` the 4 files; `feat(web): EvaluationReport sections — header, abstract, timeline`).

---

## Task 10: Report sections — Materials, AxisPanel (D & A)

**Files:** Create `Materials.tsx`, `AxisPanel.tsx`; test `sections-b.test.tsx`.

**Interfaces — Consumes:** `report.materials`, `report.depth`, `report.autonomy`, `tokens.ts`. **Produces:** `<Materials materials={report.materials} />` and `<AxisPanel axis="depth" | "autonomy" dims={...} />`.

- [ ] **Step 1** — `Materials.tsx`: one hairline card per material with a **credibility color-bar** (map `finalStatus` → bar color: core/bridge→success, 限制/第二媒介→warning, counterclaim/removed→danger, rabbit-hole→faint), the `source`, a `finalStatus` chip, a `usedIn` chip (if non-null), the `comment` prose, and a red **`cannotSupport`** line. Port `.mat` from the mockup.
- [ ] **Step 2** — `AxisPanel.tsx`: props `{ axis: "depth" | "autonomy"; dims: DepthDimResult[] | AutonomyDimResult[] }`. Renders the legend (ramp from `DEPTH_RAMP`/`AUTONOMY_RAMP`) + a **2-column grid of dimension cards, default-expanded**. Each card: color left-bar = `ramp[level-1]` (depth) / `ramp[band]` (autonomy) — **no number shown**; code chip + name + `summary`; then the evidence list (each `EvidenceItem`: `quote` as a left-border block, a `stage · id` meta line linking `#`, and a red `boundary` line when present) + the `suggestion`. Dimension display names come from a local `D_NAMES`/`A_NAMES` map (the 中文 names from the spec §4.5/4.6). **Level/band never rendered as a digit.**
- [ ] **Step 3** — test: render `<AxisPanel axis="depth" dims={mock.depth} />`; assert 6 cards, a D1 quote + boundary present, and that the digit of the level does NOT appear as a standalone badge (assert the color bar style instead). Render `<Materials/>`; assert a `cannotSupport` string shows. Run vitest. Expected: PASS.
- [ ] **Step 4: Commit** (`feat(web): EvaluationReport sections — materials, D/A axis panels`).

---

## Task 11: Report sections — PromptLens, ToolUsage, Risks

**Files:** Create `PromptLens.tsx`, `ToolUsage.tsx`, `Risks.tsx`; test `sections-c.test.tsx`.

**Interfaces — Consumes:** `report.promptLens`, `report.toolUsage`, `report.risks`, `tokens.ts`. **Produces:** the three section components.

- [ ] **Step 1** — `PromptLens.tsx`: intro card (`promptLens.summary`), then per `PromptItem`: a stage chip, the `relatedDomains` chips (e.g. `A3 · D5`), an **attention** chip only when `attention === true` (`需要留意`, warning-toned) — **no good/bad grade**, the `quote` as a blockquote, and the `observation`.
- [ ] **Step 2** — `ToolUsage.tsx`: 2-column grid; per `ToolUsageEntry` a gradient-cover card with `name`, `stage` chip, `purpose`, `summary`.
- [ ] **Step 3** — `Risks.tsx`: per `RiskEntry` a warm-warning card: type chip (`RISK_LABEL[type]`; `data-scope`→danger tint), `behaviour` (+ optional `ref` link), and the `suggestion` (labelled 处理方式). Framed as observation, not deduction.
- [ ] **Step 4** — test: assert a prompt shows its domains and NO good/needs-improvement text; the attention prompt shows `需要留意`; a risk shows its 中文 label + suggestion. Run vitest. Expected: PASS.
- [ ] **Step 5: Commit** (`feat(web): EvaluationReport sections — prompt lens, tool usage, risks`).

---

## Task 12: Report page container + 评估 nav restructure

**Files:**
- Create: `apps/web/src/shell/report/EvaluationReportPage.tsx`
- Create: `apps/web/src/shell/assessment/AssessmentView.tsx`
- Modify: `apps/web/src/shell/Nav.tsx` (`:59`), `apps/web/src/shell/StudentApp.tsx` (`:124`, `:41`)
- Test: `apps/web/src/shell/assessment/AssessmentView.test.tsx`

**Interfaces — Consumes:** `getEvaluationReport`/`generateEvaluationReport`/`listEvaluationReports` (Task 7), `<EvaluationReportView/>` (Task 8), `Segmented` (`ui/feedback.tsx`), `ToolkitCards` (existing 图鉴). **Produces:** the 评估 top-level tab.

- [ ] **Step 1** — `EvaluationReportPage.tsx`: props `{ projectId: string; onBack: () => void }`. On mount `getEvaluationReport(projectId)`; if `null`, `generateEvaluationReport(projectId)` (first-open-wins), showing a loader meanwhile; then render `<EvaluationReportView report={report} />` inside the tabbar chrome (back button + `导出 PDF` disabled button `title="即将上线"`). Empty/error states.
- [ ] **Step 2** — `AssessmentView.tsx`: a `Segmented` [`成长报告` | `图鉴`]. `图鉴` → `<ToolkitCards onOpenCourse={...} />`. `成长报告` → the **timeline**: `你的思维印记` header + a vertical list from `listEvaluationReports()` (date + title per row); clicking a row sets `openProjectId` and renders `<EvaluationReportPage projectId=... onBack={() => setOpenProjectId(null)} />`. Accept an optional `initialProjectId` (deep-link from finish) that opens its page directly.
- [ ] **Step 3** — `Nav.tsx:59`: change `label: "图鉴"` → `label: "评估"`; keep key `gallery` (or rename to `assessment` — if renamed, update `NavTab` type at `:14` and all `tab === "gallery"` sites). Recommended: keep the key `gallery` to minimize churn, change only the label + icon (`Sparkles` → `ClipboardList` from lucide-react).
- [ ] **Step 4** — `StudentApp.tsx`: replace the `tab === "gallery"` branch (`:124-125`) body with `<AssessmentView initialProjectId={growthFocus} onOpenCourse={openCourse} />`. Remove 成长报告 from the `me` Segmented (`:130-144`) — `me` keeps only `设置` (or keeps the Segmented with a single 设置; simpler: render `<SettingsView/>` directly for `me`). Route the finish deep-link (`onFinished`, `:107-112`) to `tab="gallery"` (评估) + `growthFocus=projectId` instead of `tab="me"`.
- [ ] **Step 5** — test `AssessmentView.test.tsx`: mock the api; assert the Segmented shows 成长报告/图鉴, the timeline lists a seeded entry, and clicking it renders the report (title visible). Run vitest. Expected: PASS. Also update any existing `StudentApp`/nav tests that assert the `图鉴`/`成长报告`-under-我 layout.
- [ ] **Step 6: Commit** (`feat(web): 评估 tab (成长报告 timeline + 图鉴) + report page; retire 我/成长报告`).

---

## Task 13: Teacher end reuses the report

**Files:**
- Modify: `apps/api/internal/api/evaluation_report.go` (add teacher read) + `apps/api/internal/api/api.go` (route)
- Modify: `apps/web/src/api/teacher.ts`, `apps/web/src/console/TeacherReportView.tsx`
- Test: extend `evaluation_report_test.go`

**Interfaces — Consumes:** `assertTeacherOwnsClass` (`authz.go`), `getEvaluationReport` core logic. **Produces:** teacher endpoint `GET /api/v1/classes/{id}/students/{userId}/evaluation-report/{projectId}` and a teacher client `getStudentEvaluationReport(classId, userId, projectId)`.

- [ ] **Step 1** — backend `getStudentEvaluationReport` handler: guard via `assertTeacherOwnsClass` + verify the student owns the project, then load + `evalreport.Validate` the latest row (read-no-call), return it. Register the route among the teacher-or-admin routes in `api.go`.
- [ ] **Step 2** — `api/teacher.ts`: add `getStudentEvaluationReport(classId, userId, projectId): Promise<EvaluationReport | null>` (parse via the contract).
- [ ] **Step 3** — `TeacherReportView.tsx`: replace its hand-rolled dual-axis body with `<EvaluationReportView report={report} />` (fetched via the new client), keeping the teacher chrome/back nav. (Leave `ParentReport`/`EvidenceMap` blocks as-is if they still compile against their own data; if they import the retired dual-axis type and break, gate them behind a follow-up — do not expand scope here.)
- [ ] **Step 4** — extend the Go test: a teacher in the owning class can read a student's generated report; a teacher of another class gets 404. Run `go test ./internal/api/ -run TestEvaluationReport -timeout 1800s`. Expected: PASS.
- [ ] **Step 5: Commit** (`feat: teacher end renders the shared EvaluationReport`).

---

## Task 14: Full-suite green + deploy readiness

**Files:** none (verification).

- [ ] **Step 1** — Backend: `cd apps/api && go build ./... && go test ./... -timeout 1800s`. Fix any test that asserted the old finish artifact.
- [ ] **Step 2** — Contracts: `cd packages/contracts && npx vitest run && npx tsc --noEmit`.
- [ ] **Step 3** — Web: `cd apps/web && npx tsc --noEmit && npx vitest run`.
- [ ] **Step 4** — Manual smoke (local, real DB): finish a project → confirm an `evaluation_report` row is written and the 评估 → 成长报告 timeline shows it → open it → all 9 sections render → teacher console shows the same report.
- [ ] **Step 5: Commit** any test fixups (`test: update suites for EvaluationReport pipeline`).

---

## Self-review notes (author)

- **Spec coverage:** every `EvaluationReport` §4 + Option-A field is defined in Task 1 (TS) and Task 2 (Go); read-no-call in Task 5; placeholder pipeline in Tasks 4/5/6; nav + page + teacher in Tasks 12/13. Data-gap G1–G6 are the future real-generator's concern (placeholder sidesteps them) — no task here depends on them.
- **Type consistency:** contract type names match across TS (`EvaluationReport`, `EvidenceItem`, `MaterialEntry`, …) and Go (`evalreport.Report`, `EvidenceItem`, `MaterialEntry`, …); sqlc row model is `sqlc.EvaluationReport` (distinct from `evalreport.Report`) — flagged in Task 3.
- **Deferred / out of scope (intentional):** PDF export; deletion of the old `DualAxisReport`/`mirror`/`summary`/`assessment` code (left dormant to avoid breaking parent-report/evidence-map); the real generation algorithm (colleague; swaps `generateAndStoreEvaluationReport`'s body behind the same signature).
- **Verify before build:** Task 6 must be checked against `docs/2026-08-09-all-statuses.md` (finish semantics unchanged).
