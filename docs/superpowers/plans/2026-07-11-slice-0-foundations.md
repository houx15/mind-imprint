# Slice 0 — Foundations Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Land the runtime foundation — the five contracts (C1–C5), the workspace-graph + event-stream data model, the rubric container, and the "AI never writes" enforcement primitives — as pure schema + pure functions with full tests, no UI and no live agent.

**Architecture:** Zod contracts in `packages/contracts/src` (tested with vitest, exported via `index.ts`). Additive goose migrations + sqlc queries in `apps/api/internal/store` (tested with testcontainers via `newTestPool`, `-short` skips). Enforcement primitives live server-side in Go (`apps/api/internal/agent/enforcement`), TDD with a stubbed similarity provider. Design: `docs/superpowers/specs/2026-07-11-slice-0-foundations-design.md`.

**Tech Stack:** TypeScript + Zod + vitest · Go (pgx/sqlc/goose) + testcontainers.

## Global Constraints

- **Additive migrations only.** New tables are created; `material`/`card_instances`/`evaluations` gain nullable columns; **nothing is dropped**. The existing suite must stay green (old `tasks`/`messages` code untouched). New migration file: `0016_refactor2_foundations.sql`.
- **Contracts:** Zod in `packages/contracts/src/<name>.ts`, tests in `packages/contracts/test/<name>.test.ts`, every new module re-exported from `packages/contracts/src/index.ts`. Run `pnpm --filter @mind-imprint/contracts test` and `... typecheck`.
- **Name the C1 file `interactionPrimitive.ts`** — `primitives.ts` already exists (form `FieldPrimitive`); do not collide.
- **Enforcement is server-side Go only.** No secrets in code/logs/errors. The similarity provider is an injected interface with a test stub; no real embedding call in Slice 0.
- **No UI, no runtime loop, no verb execution, no card porting** in this slice.
- **Language:** English code + comments; Chinese only for literal rubric/UI copy already authored.
- **sqlc:** after editing `.sql`, regenerate with `sqlc generate` (from `apps/api`, `CGO_ENABLED=0`) and commit the generated `internal/store/sqlc/*` too.
- **Go tests:** integration tests guard with `if testing.Short() { t.Skip(...) }` and use `newTestPool(t)`.

---

## Task 1: C1 — Interaction primitive state schemas

**Files:**
- Create: `packages/contracts/src/interactionPrimitive.ts`
- Test: `packages/contracts/test/interactionPrimitive.test.ts`
- Modify: `packages/contracts/src/index.ts` (add `export * from "./interactionPrimitive";`)

**Interfaces:**
- Produces: `Author` (`z.enum(["student","ai","imported"])`), `AnnotateState`, `GraphState`, `PRIMITIVE_KINDS`.

- [ ] **Step 1: Write the failing test**

```ts
// packages/contracts/test/interactionPrimitive.test.ts
import { describe, it, expect } from "vitest";
import { Author, AnnotateState, GraphState, PRIMITIVE_KINDS } from "../src/interactionPrimitive";

describe("interaction primitives (C1)", () => {
  it("author is student|ai|imported", () => {
    expect(Author.safeParse("student").success).toBe(true);
    expect(Author.safeParse("teacher").success).toBe(false);
  });
  it("annotate state requires author on every span", () => {
    const ok = AnnotateState.safeParse({
      material_id: "m1",
      spans: [{ id: "s1", block_ref: "b1", tag: "authority", note: "who?", author: "student" }],
    });
    expect(ok.success).toBe(true);
    const bad = AnnotateState.safeParse({ material_id: "m1", spans: [{ id: "s1", tag: "x", note: "" }] });
    expect(bad.success).toBe(false); // missing author
  });
  it("graph state carries typed nodes/edges with author", () => {
    const ok = GraphState.safeParse({
      nodes: [{ id: "n1", type: "claim", text: "China's build-out is additive", author: "student" }],
      edges: [{ id: "e1", from: "n1", to: "n2", type: "supports" }],
    });
    expect(ok.success).toBe(true);
  });
  it("registry lists the finite primitive kinds", () => {
    expect(PRIMITIVE_KINDS).toContain("annotate");
    expect(PRIMITIVE_KINDS).toContain("graph");
  });
});
```

- [ ] **Step 2: Run it and confirm it fails** — `pnpm --filter @mind-imprint/contracts test interactionPrimitive` → module-not-found / assertion fail.

- [ ] **Step 3: Implement**

```ts
// packages/contracts/src/interactionPrimitive.ts
import { z } from "zod";

// author is tracked on every mutable unit — enforcement + assessment depend on it.
export const Author = z.enum(["student", "ai", "imported"]);
export type Author = z.infer<typeof Author>;

// The finite hand-built primitive library (agent-spec C1). annotate + graph are
// defined now (built first in Slice 1); the rest are added when their slice needs them.
export const PRIMITIVE_KINDS = ["annotate", "graph", "sort", "matrix", "scale", "compare"] as const;
export const PrimitiveKind = z.enum(PRIMITIVE_KINDS);
export type PrimitiveKind = z.infer<typeof PrimitiveKind>;

export const AnnotateSpan = z.object({
  id: z.string().min(1),
  // span anchors either to a character range or a block id in the material
  range: z.object({ start: z.number().int(), end: z.number().int() }).optional(),
  block_ref: z.string().optional(),
  tag: z.string().min(1),
  note: z.string(),
  author: Author,
});
export const AnnotateState = z.object({
  material_id: z.string().min(1),
  spans: z.array(AnnotateSpan),
});
export type AnnotateState = z.infer<typeof AnnotateState>;

export const GraphNodeUnit = z.object({
  id: z.string().min(1),
  type: z.string().min(1),
  text: z.string(),
  author: Author,
});
export const GraphEdgeUnit = z.object({
  id: z.string().min(1),
  from: z.string().min(1),
  to: z.string().min(1),
  type: z.string().min(1),
});
export const GraphState = z.object({
  nodes: z.array(GraphNodeUnit),
  edges: z.array(GraphEdgeUnit),
});
export type GraphState = z.infer<typeof GraphState>;
```

- [ ] **Step 4: Run test → PASS**, then `... typecheck`.
- [ ] **Step 5: Commit** — `feat(contracts): C1 interaction primitive state schemas (annotate, graph, author)`.

---

## Task 2: Workspace graph contracts

**Files:**
- Create: `packages/contracts/src/graph.ts`
- Test: `packages/contracts/test/graph.test.ts`
- Modify: `packages/contracts/src/index.ts`

**Interfaces:**
- Consumes: `Author` from Task 1.
- Produces: `GraphNodeType`, `GraphNode`, `GraphEdge`, `NodeRefKind`.

- [ ] **Step 1: Failing test**

```ts
// packages/contracts/test/graph.test.ts
import { describe, it, expect } from "vitest";
import { GraphNode, GraphEdge, GraphNodeType, NodeRefKind } from "../src/graph";

describe("workspace graph (hybrid)", () => {
  it("light node types are the argument-graph participants", () => {
    for (const t of ["claim", "evidence", "plan", "gate_state", "note"]) {
      expect(GraphNodeType.safeParse(t).success).toBe(true);
    }
  });
  it("a graph node carries project, type, body, author", () => {
    expect(GraphNode.safeParse({
      id: "n1", project_id: "p1", type: "claim",
      body: { text: "..." }, author: "student", created_at: "2026-07-11T00:00:00Z",
    }).success).toBe(true);
  });
  it("edges are polymorphic across heavy + light nodes", () => {
    expect(NodeRefKind.safeParse("card_instance").success).toBe(true);
    expect(GraphEdge.safeParse({
      id: "e1", project_id: "p1", type: "supports",
      from_kind: "card_instance", from_id: "c1", to_kind: "graph_node", to_id: "n1",
      created_at: "2026-07-11T00:00:00Z",
    }).success).toBe(true);
  });
});
```

- [ ] **Step 2: Confirm fail.**
- [ ] **Step 3: Implement**

```ts
// packages/contracts/src/graph.ts
import { z } from "zod";
import { Author } from "./interactionPrimitive";

// Light argument-graph node types. Heavy participants (material, draft_snapshot,
// card_instance) keep their own tables and are referenced by NodeRefKind.
export const GraphNodeType = z.enum(["claim", "evidence", "plan", "gate_state", "note"]);
export type GraphNodeType = z.infer<typeof GraphNodeType>;

export const GraphNode = z.object({
  id: z.string(),
  project_id: z.string(),
  type: GraphNodeType,
  body: z.record(z.unknown()),
  author: Author,
  span_ref: z.record(z.unknown()).nullable().optional(),
  created_at: z.string(),
});
export type GraphNode = z.infer<typeof GraphNode>;

// Polymorphic edge endpoints: an edge may link a light graph_node to a heavy node.
export const NodeRefKind = z.enum(["graph_node", "material", "draft_snapshot", "card_instance"]);
export type NodeRefKind = z.infer<typeof NodeRefKind>;

export const GraphEdge = z.object({
  id: z.string(),
  project_id: z.string(),
  type: z.string().min(1),
  from_kind: NodeRefKind,
  from_id: z.string(),
  to_kind: NodeRefKind,
  to_id: z.string(),
  created_at: z.string(),
});
export type GraphEdge = z.infer<typeof GraphEdge>;
```

- [ ] **Step 4: Test → PASS + typecheck.**
- [ ] **Step 5: Commit** — `feat(contracts): workspace graph node/edge contracts (polymorphic edges)`.

---

## Task 3: C4 — Event set + stream

**Files:**
- Create: `packages/contracts/src/event.ts`
- Test: `packages/contracts/test/event.test.ts`
- Modify: `packages/contracts/src/index.ts`

**Interfaces:**
- Produces: `Surface`, `StudioEvent` (discriminated union), `EVENT_TYPES`.

- [ ] **Step 1: Failing test**

```ts
// packages/contracts/test/event.test.ts
import { describe, it, expect } from "vitest";
import { StudioEvent, Surface, EVENT_TYPES } from "../src/event";

describe("event stream (C4)", () => {
  it("surface is studio|course|chat", () => {
    expect(Surface.safeParse("studio").success).toBe(true);
    expect(Surface.safeParse("nope").success).toBe(false);
  });
  it("card_clicked carries the unprompted flag", () => {
    const ev = StudioEvent.safeParse({
      type: "card_clicked", surface: "studio", card_id: "craap", unprompted: true,
    });
    expect(ev.success).toBe(true);
  });
  it("suggestion_disposition carries action + reason", () => {
    expect(StudioEvent.safeParse({
      type: "suggestion_disposition", surface: "studio", action: "reject", reason: "source is a blog",
    }).success).toBe(true);
  });
  it("enumerates all event types", () => {
    expect(EVENT_TYPES).toEqual(expect.arrayContaining([
      "prompt_sent","card_clicked","gate_attempt","suggestion_disposition","verbalization_submitted",
      "source_opened","citation_added","version_saved","rescue_triggered","stance_change_logged","chat_message",
    ]));
  });
});
```

- [ ] **Step 2: Confirm fail.**
- [ ] **Step 3: Implement** — `Surface = z.enum(["studio","course","chat"])`; a `z.discriminatedUnion("type", [...])` with one variant per event in `EVENT_TYPES`, each carrying its §14.2 fields (e.g. `card_clicked`: `{ card_id, unprompted: boolean }`; `suggestion_disposition`: `{ action: z.enum(["accept","reject","rewrite"]), reason: z.string() }`; `source_opened`: `{ url, time_spent_s, tier?, lateral_read? }`; etc.). Every variant includes `surface: Surface`. Export `EVENT_TYPES` as a `const` string array.
- [ ] **Step 4: Test → PASS + typecheck.**
- [ ] **Step 5: Commit** — `feat(contracts): C4 event set + surface (unprompted-vs-prompted instrumented)`.

---

## Task 4: C3 — Agent output + verb set

**Files:**
- Create: `packages/contracts/src/agentOutput.ts`
- Test: `packages/contracts/test/agentOutput.test.ts`
- Modify: `packages/contracts/src/index.ts`

**Interfaces:**
- Produces: `Verb`, `AgentOutput` (discriminated union), `Anchor` re-use from `./anchor` if compatible else local `OutputAnchor`.

- [ ] **Step 1: Failing test**

```ts
// packages/contracts/test/agentOutput.test.ts
import { describe, it, expect } from "vitest";
import { Verb, AgentOutput } from "../src/agentOutput";

describe("agent output (C3)", () => {
  it("verb set is the closed list", () => {
    for (const v of ["surface_card","post_intervention","check_gate","plan","replan",
                     "advance","route","invite_commit","reply","propose"]) {
      expect(Verb.safeParse(v).success).toBe(true);
    }
    expect(Verb.safeParse("write_essay").success).toBe(false);
  });
  it("question/diagnostic carry anchor + criterion + body", () => {
    expect(AgentOutput.safeParse({ type: "question", anchor: { kind: "graph_node", id: "n1" },
      criterion: "D4", body: "Who holds the opposing view?" }).success).toBe(true);
  });
  it("reference must quote a student artifact with provenance", () => {
    expect(AgentOutput.safeParse({ type: "reference", anchor: { kind: "artifact", id: "a1" },
      quote: "my earlier claim", provenance: "artifact:a1" }).success).toBe(true);
    expect(AgentOutput.safeParse({ type: "reference", anchor: { kind: "artifact", id: "a1" },
      quote: "x" }).success).toBe(false); // provenance required
  });
});
```

- [ ] **Step 2: Confirm fail.**
- [ ] **Step 3: Implement** — `Verb` enum (the 10 verbs). `OutputAnchor = z.object({ kind: z.enum(["graph_node","material","draft_snapshot","card_instance","artifact","gate_item"]), id: z.string(), span: z.record(z.unknown()).optional() })`. `AgentOutput = z.discriminatedUnion("type", [question, diagnostic, reference, proposal, plan])` where `question`/`diagnostic`/`proposal` = `{ anchor, criterion, body }`, `reference` = `{ anchor, quote, provenance: z.string().min(1) }`, `plan` = `{ route: z.array(z.string()) }`.
- [ ] **Step 4: Test → PASS + typecheck.**
- [ ] **Step 5: Commit** — `feat(contracts): C3 verb set + typed agent output union`.

---

## Task 5: C2 — Card format evolution (additive, keeps existing green)

**Files:**
- Modify: `packages/contracts/src/cardSpec.ts`
- Test: `packages/contracts/test/cardSpec.test.ts` (add cases; keep existing)

**Interfaces:**
- Consumes: `PrimitiveKind` (Task 1).
- Produces: extended `CardSpec` with optional `primitive`, `subject`, `stage`, `params`, `completion`, `graph_effects`, `observe`, `consolidation`, `intrusiveness_cap`.

- [ ] **Step 1: Add failing tests** — a card JSON with `primitive: "annotate"`, `subject: ["global-perspectives"]`, `stage: ["S3"]`, `completion`, `graph_effects`, `observe` parses; **and** an existing card JSON with none of these still parses (back-compat). Assert `interaction_type` is now optional/deprecated but still accepted.
- [ ] **Step 2: Confirm the new-fields case fails** (unknown keys / missing schema) while existing pass.
- [ ] **Step 3: Implement** — extend `CardSpec` with, all `.optional()`:

```ts
primitive: PrimitiveKind.optional(),
subject: z.array(z.string()).optional(),
stage: z.array(z.string()).optional(),
target_type: z.string().optional(),
params: z.record(z.unknown()).optional(),
completion: z.array(z.record(z.unknown())).optional(),
graph_effects: z.array(z.record(z.unknown())).optional(),
observe: z.array(z.object({ when: z.string(), move: z.record(z.unknown()) })).optional(),
consolidation: z.string().optional(),
intrusiveness_cap: z.enum(["I0","I1","I2","I3","I4"]).optional(),
```

Keep `steps`, `mode`, `interaction_type` (mark `interaction_type` deprecated in a comment; do not remove — retirement happens when cards are ported in Slice 3).

- [ ] **Step 4: Run the full contracts suite** — every existing card/registry test must stay green. `... test` + `... typecheck`.
- [ ] **Step 5: Commit** — `feat(contracts): C2 card format — primitive binding + subject/stage (additive)`.

---

## Task 6: C5 — Skill format

**Files:**
- Create: `packages/contracts/src/skill.ts`
- Test: `packages/contracts/test/skill.test.ts`
- Modify: `packages/contracts/src/index.ts`

**Interfaces:**
- Produces: `SkillKind`, `Contract`, `Gate`, `Skill`.

- [ ] **Step 1: Failing test** — a `writing-project` fixture with `kind: "project"`, a `contracts` map where each contract has `requires`, `produces`, `view`, `repertoire`, `gate: { machine, student_written, human }`, plus `intake`, `vocabulary`, `cards` (refs) parses; a contract missing `gate` fails.
- [ ] **Step 2: Confirm fail.**
- [ ] **Step 3: Implement**

```ts
// packages/contracts/src/skill.ts
import { z } from "zod";
export const SkillKind = z.enum(["project", "course"]);
export const Gate = z.object({
  machine: z.array(z.string()).default([]),
  student_written: z.array(z.string()).default([]),
  human: z.array(z.string()).default([]),
});
export const Contract = z.object({
  requires: z.array(z.string()).default([]),
  produces: z.array(z.string()).default([]),
  view: z.string().optional(),
  repertoire: z.array(z.string()).default([]),
  gate: Gate,
});
export const Skill = z.object({
  id: z.string().min(1),
  kind: SkillKind,
  contracts: z.record(Contract),
  intake: z.record(z.unknown()).optional(),
  vocabulary: z.string().optional(),
  cards: z.array(z.string()).default([]),
});
export type Skill = z.infer<typeof Skill>;
```

- [ ] **Step 4: Test → PASS + typecheck.**
- [ ] **Step 5: Commit** — `feat(contracts): C5 skill format (contract DAG + gate item types)`.

---

## Task 7: Rubric container (wrap existing CT dims; OPCVL deferred)

**Files:**
- Modify: `packages/contracts/src/rubric.ts`
- Test: `packages/contracts/test/rubric.test.ts` (add cases)

**Interfaces:**
- Consumes: existing `FULL_RUBRIC` (D1–D10).
- Produces: `Rubric` container + `CT_RUBRIC` (id `"ct"`), a completeness validator.

- [ ] **Step 1: Add failing tests** — `CT_RUBRIC.id === "ct"`; it has 10 dimensions; **every** dimension has non-empty L1–L4 anchors (structural completeness); `assertRubricComplete(CT_RUBRIC)` does not throw; a rubric with a blank anchor throws.
- [ ] **Step 2: Confirm fail.**
- [ ] **Step 3: Implement** — add:

```ts
export interface Rubric { id: string; name: string; dimensions: RubricDimension[]; }
export const CT_RUBRIC: Rubric = { id: "ct", name: "AI 批判思维（9+1 维）", dimensions: FULL_RUBRIC };
export function assertRubricComplete(r: Rubric): void {
  for (const d of r.dimensions) {
    for (const lvl of ["L1","L2","L3","L4"] as const) {
      if (!d.anchors[lvl] || d.anchors[lvl].trim() === "")
        throw new Error(`rubric ${r.id} dim ${d.id} missing ${lvl} anchor`);
    }
  }
}
```

Add a comment: OPCVL (HS-D1…D12) is a separate rubric, deferred to its module (assessment §10 Q3).

- [ ] **Step 4: Test → PASS + typecheck.**
- [ ] **Step 5: Commit** — `feat(contracts): CT rubric container + completeness validator (OPCVL deferred)`.

---

## Task 8: Enforcement primitives (Go)

**Files:**
- Create: `apps/api/internal/agent/enforcement/enforcement.go`
- Create: `apps/api/internal/agent/enforcement/banned_phrasing.go` (corpus + matcher)
- Test: `apps/api/internal/agent/enforcement/enforcement_test.go`

**Interfaces:**
- Produces: `AgentOutput` Go struct + `ValidateOutput`; `Author` type + `GuardStudentField`; `BannedPhrasing(text) *Rule`; `Similarity` interface + `OutputCheck(text, ctx, sim) Verdict`.

- [ ] **Step 1: Failing tests**

```go
// enforcement_test.go (essentials)
func TestValidateOutput_ReferenceNeedsProvenance(t *testing.T) {
  err := ValidateOutput(AgentOutput{Type: "reference", Quote: "x"}) // no provenance
  if err == nil { t.Fatal("reference without provenance must be rejected") }
}
func TestGuardStudentField_RejectsNonStudent(t *testing.T) {
  if err := GuardStudentField("warrant", "ai"); err == nil { t.Fatal("ai author must be rejected on a student field") }
  if err := GuardStudentField("warrant", "student"); err != nil { t.Fatalf("student must pass: %v", err) }
}
func TestBannedPhrasing_FlagsUnanchoredQuestion(t *testing.T) {
  if BannedPhrasing("Have you considered other angles?") == nil { t.Fatal("banned phrase must be flagged") }
}
func TestOutputCheck_InterceptsDeclarativeEcho(t *testing.T) {
  sim := stubSim(0.95)
  v := OutputCheck("China's transition makes the planet more sustainable.",
     Context{Topic: "is China making the planet more sustainable"}, sim)
  if v.Verdict != "intercept" { t.Fatalf("expected intercept, got %s", v.Verdict) }
}
func TestOutputCheck_CrossDomainExamplePasses(t *testing.T) {
  v := OutputCheck("In chemistry, a catalyst speeds a reaction without being consumed.",
     Context{Topic: "China sustainability", CrossDomainExample: true}, stubSim(0.99))
  if v.Verdict != "pass" { t.Fatal("cross-domain example is exempt") }
}
```

- [ ] **Step 2: Confirm fail** — `go test ./internal/agent/enforcement/...` (no `-short` needed; pure functions).
- [ ] **Step 3: Implement** — `AgentOutput` struct mirroring the Zod union (`Type`, `Anchor`, `Criterion`, `Body`, `Quote`, `Provenance`); `ValidateOutput` rejects unknown types and a `reference` without `Provenance`. `GuardStudentField(field, author)` returns error when `author != "student"`. `banned_phrasing.go` holds a versioned slice of `Rule{Name, Pattern}` (start with unanchored-question, suggested-counterclaim, candidate-example patterns) + `BannedPhrasing(text)` substring/regex match. `Similarity interface { Cosine(a, b string) float64 }`; `OutputCheck(text, ctx, sim)` = if `ctx.CrossDomainExample` → pass; else if `isDeclarative(text) && sim.Cosine(text, ctx.Topic) > threshold (0.8)` → `{Verdict:"intercept", Rewrite: asQuestion(text)}`; else pass. `stubSim(x)` test helper returns constant.
- [ ] **Step 4: Tests → PASS.** `go vet ./internal/agent/enforcement/...`, `gofmt -w`.
- [ ] **Step 5: Commit** — `feat(api): AI-never-writes enforcement primitives (output-check, banned-phrasing, guards)`.

---

## Task 9: Additive migration — the graph + event + project world

**Files:**
- Create: `apps/api/internal/store/migrations/0016_refactor2_foundations.sql`
- Test: `apps/api/internal/store/refactor2_migrate_test.go`

- [ ] **Step 1: Failing test** — mirror `migrate_test.go`: after `newTestPool`, assert the new tables exist: `project`, `graph_node`, `graph_edge`, `draft_snapshot`, `edit_buffer`, `source_log_entry`, `intervention`, `disposition`, `card_competence`, `chat_thread`, `chat_message`, `event`; and that `material`, `card_instances`, `evaluations` now have a `project_id` column (query `information_schema.columns`). Guard with `-short` skip.
- [ ] **Step 2: Confirm fail** (tables missing).
- [ ] **Step 3: Implement the migration** — `-- +goose Up`: `CREATE TABLE` for each new table per the design §2 (uuid PKs `DEFAULT gen_random_uuid()`, `project` FKs `users(id)`, child tables FK `project(id) ON DELETE CASCADE`, `event` append-only with an index on `(project_id, created_at)`, `graph_edge` polymorphic `from_kind/from_id/to_kind/to_id`); `ALTER TABLE material ADD COLUMN project_id uuid REFERENCES project(id) ON DELETE CASCADE`; same nullable add on `card_instances` (+ `contract_ref text`, `framework_fill jsonb NOT NULL DEFAULT '{}'`) and `evaluations` (+ `rubric text`, `leaps jsonb NOT NULL DEFAULT '[]'`, `project_id`). `-- +goose Down`: drop the new columns then the new tables in FK-safe order. **Do not touch `tasks`/`messages`.**
- [ ] **Step 4: Test → PASS** (`go test ./internal/store/ -run Refactor2`). Also run the existing `TestMigrationsCreateTablesAndSeed` to confirm nothing broke.
- [ ] **Step 5: Commit** — `feat(api): migration 0016 — project/graph/event foundations (additive)`.

---

## Task 10: sqlc queries for the new foundation tables

**Files:**
- Create: `apps/api/internal/store/queries/project.sql`, `graph.sql`, `event.sql`
- Regenerate: `apps/api/internal/store/sqlc/*` (via `sqlc generate`)
- Test: `apps/api/internal/store/refactor2_sqlc_test.go`

**Interfaces:**
- Produces: `CreateProject`/`GetProject`/`ListProjectsByUser`; `InsertGraphNode`/`ListGraphNodesByProject`; `InsertGraphEdge`/`ListGraphEdgesByProject`; `AppendEvent`/`ListEventsByProject` (event has **insert + select only** — no update/delete).

- [ ] **Step 1: Failing test** — `newTestPool`; create a user+project; insert a graph_node + edge; append two events; assert list queries return them in order. (Scope queries to project/graph/event only; other tables' queries land in the slices that use them.)
- [ ] **Step 2: Confirm fail** (query funcs don't exist yet).
- [ ] **Step 3: Implement** — write the `-- name: X :one|:many|:exec` annotated queries; run `sqlc generate` (from `apps/api`, `CGO_ENABLED=0 sqlc generate`); commit generated files. Deliberately author **no** UpdateEvent/DeleteEvent (append-only).
- [ ] **Step 4: Test → PASS** (`go test ./internal/store/ -run Refactor2Sqlc`).
- [ ] **Step 5: Commit** — `feat(api): sqlc queries for project/graph/event (event append-only)`.

---

## Task 11: Slice verification + roadmap log

**Files:**
- Modify: `docs/2026-07-11-whole-product-refactor-roadmap.md` (per-slice log)

- [ ] **Step 1:** Run the full gate: `pnpm --filter @mind-imprint/contracts test` + `typecheck`; `go build ./...`; `go vet ./...`; `go test ./... -short`; and the two testcontainers tests (`-run Refactor2` + existing migrate test) without `-short`.
- [ ] **Step 2:** Confirm acceptance criteria from the design §12: five contracts present with tests; additive migrations up/down green; existing suite green; CT rubric validated; four enforcement primitives green; no UI/loop introduced.
- [ ] **Step 3:** Update the roadmap per-slice log: mark Slice 0 ☑, link this plan, note commit range.
- [ ] **Step 4: Commit** — `docs(refactor2): Slice 0 foundations complete — log + status`.

---

## Self-review notes

- **Spec coverage:** C1 (T1) · C2 (T5) · C3 (T4) · C4 (T3) · C5 (T6) · graph model (T2, T9) · event stream (T3, T9, T10) · rubric config (T7) · enforcement primitives (T8) · migrations additive (T9) · sqlc (T10). All design §2–§10 items map to a task.
- **Deferred by design (not gaps):** OPCVL ladder authoring (T7 note), Go mirrors of primitive/event payload structs (added when Slice 1/2 exercise them), queries for non-foundation tables (added per slice), CRAAP/Toulmin port (Slice 3).
- **Type consistency:** `Author` defined once (T1), reused by T2 and T8. `AgentOutput` union shape matches between Zod (T4) and Go (T8). Event types list identical in T3 and referenced by T10.
