# Slice 12 — Course policy + surface alignment · Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Run the 课程 surface on the agent runtime under Course policy — a course skill of binding-order phases wrapping the existing steps, a coach that executes the script and holds `advance` behind a structural floor, a session-scoped card practice, and the binding 问印记 ask panel.

**Architecture:** Additive throughout. `course`/`course_step`/`course_progress` and the teaching/challenge renderer survive untouched as authored *content*; a new course **skill** (C5 config) adds the phase layer above them. `RunCourseStep` mirrors Slice 11's `RunChatStep` (coach + classifier, planner off) behind a project-free `CourseStore` seam; migration 0023 adds `course_session`/`course_message` and a fourth scope column on material/card_instances, exactly as 0022 added the third.

**Tech Stack:** Go (net/http · sqlc · goose · testcontainers · pgx/pgtype) · TypeScript + React + vitest · Zod contracts.

**Spec:** `docs/superpowers/specs/2026-07-17-slice-12-course-policy-design.md` — read the decision it names when a task cites one.

## Global Constraints

- **Binding design** is `docs/design/思维印记_工作区.dc.html` lines 215–410. UI copy verbatim: `问印记` · `随时打断我，问任何问题` · `正在看：` · `你可能想问` · `输入你的问题……` · `按住说话，问老师`. Binding design wins over anything in this plan.
- **Icons are inline SVG. Never lucide-react.**
- **DEC-12.2 floor discipline:** the floor is checked in Go *before* any model call; an unmet floor means **no LLM call at all**. Floor inputs are server-side only — `steps_viewed` reads `course_progress.completed_ordinals`, never a client-supplied list.
- **DEC-12.2 no dead-end:** the `guided` floor kind is `card_dispositioned` — a **skipped** card satisfies it. A floor never traps a student.
- **`advance` is forward-only, one phase at a time:** an `advance` output naming anything but the current phase's single successor leaves the phase unchanged.
- **Metering on reject:** every real model call records one `llm_call` row — `surface="course"`, `purpose="coach"`, `project_id` NULL, `user_id` set — **including when enforcement rejects the output**. A rejected output is silence: no message, no advance, not an error return.
- **Thin card path:** course card submit/skip persists answers + status only. **No** graph_effects, **no** refeed, **no** competence write (`card_competence` is dormant platform-wide — zero update sites; do not add one).
- **RL-1:** Course has no draft write-path at all. Banned-phrasing still runs on every output.
- **铁律 2 (不操纵):** card offers are confirm-to-open; a refused advance is a sentence in the ask panel — never a modal, lock, streak, or leaderboard.
- **Secrets** never in logs, errors, migrations, payloads.
- **Go tests:** `CGO_ENABLED=0 go test -p 1 ./...` from `apps/api`, needs Docker (testcontainers). Run **FULL packages**, never `-run` subsets, for any card/gate/projection-touching change.
- **Codegen:** `make sqlc` and `make sync-skills` from `apps/api`. Never hand-edit `internal/store/sqlc/*` or `internal/skills/specs/*`.
- **Never `git add` a whole directory.** Name every path. The pre-existing `M package.json` and untracked user files (`docs/03_课程库_单课设计/`, `docs/2026-07-06-spec.md`, `docs/astranova/`, the Toddle handoff zip, `pm-e2e-01-directory.png`, `walk-01-directory.png`) are NOT ours — leave them untouched.

---

## File structure

| File | Responsibility |
|---|---|
| `packages/contracts/src/agentOutput.ts` (M) | + `advance` union member |
| `apps/api/internal/agent/enforcement/enforcement.go` (M) | + `To` field, `advance` validation |
| `packages/contracts/src/skill.ts` (M) | + course-only optional Contract/Skill fields |
| `apps/api/internal/skills/skill.go` (M) | + same fields in Go, `CourseFloorKinds`, linear-chain validation |
| `packages/contracts/skills/info-literacy-course.json` (C) | the course skill (single source of truth) |
| `apps/api/internal/store/migrations/0023_course_session_scope.sql` (C) | `course_session` · `course_message` · `session_id` scope |
| `apps/api/internal/store/queries/coursesession.sql` (C) | session/message/floor queries |
| `apps/api/internal/store/queries/{material,card_instance}.sql` (M) | session-scoped material/card queries |
| `apps/api/internal/agent/course_coach.go` (C) | posture prompt · `BuildCourseContext` · `ProposeCourseReply` |
| `apps/api/internal/agent/course_step.go` (C) | `NextPhase` · `CheckFloor` · `CourseCardCandidate` · `CourseStore` · `RunCourseStep` |
| `apps/api/internal/agent/coursestore.go` (C) | sqlc adapter for `CourseStore` |
| `apps/api/internal/api/course_session.go` (C) | the six endpoints |
| `apps/api/internal/api/course_session_dto.go` (C) | session/message DTOs |
| `packages/contracts/src/course.ts` (M) | + `CourseSession` · `CourseMessage` Zod |
| `apps/web/src/api/courseSession.ts` (C) | web client |
| `apps/web/src/shell/courses/AskPanel.tsx` (C) | the binding 问印记 panel |
| `apps/web/src/shell/courses/CoursePlayer.tsx` (M) | phase label · panel mount · boundary advance · card sheet |

---

### Task 1: Rename Chat's scoped-graph types to surface-neutral names (DEC-12.6)

Mechanical rename, zero behavior change. Slice 11 named these for the only surface that then existed; Course needs exactly them.

**Files:**
- Modify: `apps/api/internal/agent/chat_step.go`, `apps/api/internal/agent/chatstore.go`, `apps/api/internal/agent/chat_step_test.go`, `apps/api/internal/api/chat.go` (and any other hit the grep finds)

**Interfaces:**
- Produces: `ScopedMaterial{ID uuid.UUID; Kind, SourceURL string}`, `ScopedCard{ID uuid.UUID; CardID, Status string}`, `CardOffer{CardInstanceID, MaterialID uuid.UUID; CardID string}` — Tasks 6 and 7 consume these names.

- [ ] **Step 1: Find every occurrence**

```bash
cd apps/api && grep -rn "ThreadMaterial\|ThreadCard\b\|ChatCardOffer" --include=*.go .
```

Expect hits in `internal/agent/chat_step.go`, `internal/agent/chatstore.go`, `internal/agent/chat_step_test.go`, `internal/api/chat.go`.

Note `ThreadCard` needs the `\b` — do **not** rename `CreateThreadCardInstance`, `ListThreadCards`, `SubmitThreadCardInstanceParams`, or any other identifier that merely *contains* the substring. Only the three bare type names change.

- [ ] **Step 2: Rename the three types**

In `chat_step.go`, the declarations become:

```go
// ScopedMaterial / ScopedCard are the minimal projection of a scoped graph
// (a chat thread's, a course session's) that the classifier reads. Surface-
// neutral by design: the same shapes serve Chat and Course.
type ScopedMaterial struct {
	ID        uuid.UUID
	Kind      string
	SourceURL string
}
type ScopedCard struct {
	ID     uuid.UUID
	CardID string
	Status string
}
```

and

```go
// CardOffer is one card surfaced to a student as an offer — the card
// instance, the card it renders, and the material it hangs on.
type CardOffer struct {
	CardInstanceID, MaterialID uuid.UUID
	CardID                     string
}
```

Update every use site (signatures, struct literals, the `ChatStore` interface methods `ListThreadMaterials`/`ListThreadCards` return types, `ChatCardCandidate`'s params, `ChatStepResult.Offer`, the test's fakes). Method **names** stay as they are — only the types change.

- [ ] **Step 3: Verify nothing behavioral changed**

```bash
cd apps/api && CGO_ENABLED=0 go build ./... && CGO_ENABLED=0 go test -p 1 ./internal/agent/ ./internal/api/
```

Expected: build clean; both packages `ok`. No test file's *assertions* should have changed — only type names. If a test's expected values changed, you renamed something you shouldn't have.

- [ ] **Step 4: Commit**

```bash
git add apps/api/internal/agent/chat_step.go apps/api/internal/agent/chatstore.go apps/api/internal/agent/chat_step_test.go apps/api/internal/api/chat.go
git commit -m "refactor(refactor2): Slice 12 T1 — rename Chat's scoped-graph types surface-neutral (DEC-12.6)"
```

---

### Task 2: The `advance` typed output (C3 seam)

`advance` is in the C3 `Verb` enum but has never had an `AgentOutput` member — the same gap `reply` had before Slice 11.

**Files:**
- Modify: `packages/contracts/src/agentOutput.ts`
- Modify: `apps/api/internal/agent/enforcement/enforcement.go`
- Test: `packages/contracts/test/agentOutput.test.ts` (note: contracts tests live under `test/`, **not** `src/` — that's the vitest config's include)
- Test: `apps/api/internal/agent/enforcement/enforcement_test.go`

**Interfaces:**
- Produces: `enforcement.AgentOutput.To string` (JSON `to`); output type `"advance"`. Task 6 emits and validates it.

- [ ] **Step 1: Write the failing contracts test**

Append to `packages/contracts/test/agentOutput.test.ts`:

```ts
describe("advance output", () => {
  it("accepts an advance naming a target phase", () => {
    const r = AgentOutput.safeParse({ type: "advance", to: "guided" });
    expect(r.success).toBe(true);
  });

  it("rejects an advance with no target", () => {
    expect(AgentOutput.safeParse({ type: "advance", to: "" }).success).toBe(false);
    expect(AgentOutput.safeParse({ type: "advance" }).success).toBe(false);
  });

  it("leaves the existing output types unchanged", () => {
    expect(AgentOutput.safeParse({ type: "reply", body: "嗯？" }).success).toBe(true);
    expect(AgentOutput.safeParse({ type: "plan", route: ["a"] }).success).toBe(true);
  });
});
```

- [ ] **Step 2: Run it and watch it fail**

```bash
cd packages/contracts && npx vitest run test/agentOutput.test.ts
```

Expected: FAIL — the advance cases fail because no `advance` member exists in the discriminated union.

- [ ] **Step 3: Add the union member**

In `packages/contracts/src/agentOutput.ts`, add as the last member of the `AgentOutput` discriminated union (after `reply`):

```ts
  z.object({
    type: z.literal("advance"),
    to: z.string().min(1),
  }),
```

- [ ] **Step 4: Run it and watch it pass**

```bash
cd packages/contracts && npx vitest run test/agentOutput.test.ts
```

Expected: PASS.

- [ ] **Step 5: Write the failing Go enforcement test**

Append to `apps/api/internal/agent/enforcement/enforcement_test.go`:

```go
func TestValidateOutputAdvance(t *testing.T) {
	if err := ValidateOutput(AgentOutput{Type: "advance", To: "guided"}); err != nil {
		t.Fatalf("advance with a target must validate, got %v", err)
	}
	if err := ValidateOutput(AgentOutput{Type: "advance"}); err == nil {
		t.Fatal("advance with no target must be rejected")
	}
	if err := ValidateOutput(AgentOutput{Type: "advance", To: "   "}); err == nil {
		t.Fatal("advance with a blank target must be rejected")
	}
}
```

- [ ] **Step 6: Run it and watch it fail**

```bash
cd apps/api && CGO_ENABLED=0 go test ./internal/agent/enforcement/
```

Expected: FAIL to compile — `AgentOutput` has no field `To`.

- [ ] **Step 7: Implement**

In `apps/api/internal/agent/enforcement/enforcement.go`, add the field to `AgentOutput`:

```go
	Route      []string     `json:"route,omitempty"`
	To         string       `json:"to,omitempty"`
```

Add to `validOutputTypes`:

```go
	"advance":    true,
```

Add to `ValidateOutput`'s switch (alongside the `reply` case):

```go
	case "advance":
		if strings.TrimSpace(out.To) == "" {
			return fmt.Errorf("enforcement: advance output requires a target phase")
		}
```

- [ ] **Step 8: Run the FULL enforcement package**

```bash
cd apps/api && CGO_ENABLED=0 go test -p 1 ./internal/agent/enforcement/
```

Expected: `ok`. The banned-phrasing suite and the other six output types must be unaffected.

- [ ] **Step 9: Commit**

```bash
git add packages/contracts/src/agentOutput.ts packages/contracts/test/agentOutput.test.ts apps/api/internal/agent/enforcement/enforcement.go apps/api/internal/agent/enforcement/enforcement_test.go
git commit -m "feat(refactor2): Slice 12 T2 — advance typed output across the Zod/Go C3 seam"
```

---

### Task 3: Course skill config — C5 course fields, floor kinds, linear validation, the skill JSON

**Files:**
- Modify: `packages/contracts/src/skill.ts`
- Modify: `apps/api/internal/skills/skill.go`
- Create: `packages/contracts/skills/info-literacy-course.json`
- Test: `packages/contracts/test/skill.test.ts`
- Test: `apps/api/internal/skills/skill_test.go`

**Interfaces:**
- Consumes: nothing from Tasks 1–2.
- Produces, for Tasks 6–8:
  ```go
  type FloorItem struct {
      Kind   string `json:"kind"`
      Steps  []int  `json:"steps"`
      CardID string `json:"card_id"`
      N      int    `json:"n"`
  }
  type PhasePage struct {
      Title    string   `json:"title"`
      Subtitle string   `json:"subtitle"`
      Body     []string `json:"body"`
  }
  type AnchorMaterial struct {
      Title string `json:"title"`
      Text  string `json:"text"`
  }
  // added to Contract:
  //   Goal string; Steps []int; Page *PhasePage; Cards []string
  //   AnchorMaterial *AnchorMaterial; AskChips []string
  //   Floor []FloorItem; SoftCondition string
  // added to Skill:
  //   CourseID string `json:"course_id,omitempty"`
  var CourseFloorKinds = map[string]bool{"steps_viewed": true, "card_dispositioned": true, "student_turns_at_least": true}
  func (s Skill) LinearOrder() ([]string, error)  // course only; errors unless the chain is strictly linear
  ```
  Skill id: `"info-literacy-course"`. Phase ids in order: `demonstrate` → `guided` → `independent` → `reflect`.

- [ ] **Step 1: Write the failing Go skills tests**

Append to `apps/api/internal/skills/skill_test.go`:

```go
func TestLoadCourseSkill(t *testing.T) {
	s, err := LoadByID("info-literacy-course")
	if err != nil {
		t.Fatalf("course skill must load: %v", err)
	}
	if s.Kind != "course" {
		t.Fatalf("kind = %q, want course", s.Kind)
	}
	order, err := s.LinearOrder()
	if err != nil {
		t.Fatalf("course chain must be linear: %v", err)
	}
	want := []string{"demonstrate", "guided", "independent", "reflect"}
	if len(order) != len(want) {
		t.Fatalf("order = %v, want %v", order, want)
	}
	for i := range want {
		if order[i] != want[i] {
			t.Fatalf("order = %v, want %v", order, want)
		}
	}
	g := s.Contracts["guided"]
	if len(g.Floor) != 1 || g.Floor[0].Kind != "card_dispositioned" || g.Floor[0].CardID != "craap" {
		t.Fatalf("guided floor = %+v, want one card_dispositioned craap", g.Floor)
	}
	if g.AnchorMaterial == nil || g.AnchorMaterial.Text == "" {
		t.Fatal("guided must declare an anchor material")
	}
	if s.Contracts["demonstrate"].SoftCondition == "" {
		t.Fatal("every phase needs a soft condition")
	}
}

func TestCourseValidationRejectsBadConfig(t *testing.T) {
	cases := map[string]string{
		"unknown floor kind": `{"id":"x","kind":"course","cards":["craap"],"contracts":{
			"a":{"floor":[{"kind":"vibes_ok"}],"gate":{}}}}`,
		"floor card not in skill cards": `{"id":"x","kind":"course","cards":[],"contracts":{
			"a":{"floor":[{"kind":"card_dispositioned","card_id":"craap"}],"gate":{}}}}`,
		"branching chain": `{"id":"x","kind":"course","contracts":{
			"a":{"gate":{}},
			"b":{"requires":["a"],"gate":{}},
			"c":{"requires":["a"],"gate":{}}}}`,
		"two roots": `{"id":"x","kind":"course","contracts":{
			"a":{"gate":{}},
			"b":{"gate":{}}}}`,
	}
	for name, blob := range cases {
		if _, err := Load([]byte(blob)); err == nil {
			t.Fatalf("%s: must be rejected at load", name)
		}
	}
}

func TestProjectSkillStillLoads(t *testing.T) {
	s, err := LoadByID("writing-project")
	if err != nil {
		t.Fatalf("writing-project must keep loading unchanged: %v", err)
	}
	if _, err := s.TopoOrder(); err != nil {
		t.Fatalf("writing-project DAG must stay valid: %v", err)
	}
}
```

If `LoadByID` does not exist in `skills`, use whatever loader the package already exposes for the embedded catalog (read `skill.go` — the `go:embed specs/*.json` FS is there) and adjust these three call sites; report the actual name in your report so later tasks use it.

- [ ] **Step 2: Run and watch it fail**

```bash
cd apps/api && CGO_ENABLED=0 go test ./internal/skills/
```

Expected: FAIL to compile — no `Floor`, `AnchorMaterial`, `SoftCondition`, `LinearOrder`.

- [ ] **Step 3: Add the Go types + validation**

In `apps/api/internal/skills/skill.go`:

```go
// CourseFloorKinds is the closed set of course phase-floor kinds (Slice 12,
// DEC-12.2). The names live here (config validation); the evaluation lives in
// the agent package — the same split MachineKinds uses. The floor is the
// structural half of phase advance: it can only ever REFUSE. The positive
// pedagogical call is the coach's (DEC-3 discipline).
var CourseFloorKinds = map[string]bool{
	"steps_viewed":           true,
	"card_dispositioned":     true,
	"student_turns_at_least": true,
}

// FloorItem is one machine-checkable phase floor. Steps/CardID/N are read only
// by the kind that uses them, mirroring MachineItem.
type FloorItem struct {
	Kind   string `json:"kind"`
	Steps  []int  `json:"steps"`
	CardID string `json:"card_id"`
	N      int    `json:"n"`
}

// PhasePage is a step-less phase's authored page (the guided phase's page is
// the card; the reflect phase's is the dialogue). Phases that wrap course_step
// ordinals render from those rows instead and leave this nil.
type PhasePage struct {
	Title    string   `json:"title"`
	Subtitle string   `json:"subtitle"`
	Body     []string `json:"body"`
}

// AnchorMaterial is the case a phase's card practice hangs on — minted as a
// session material the first time the card surfaces.
type AnchorMaterial struct {
	Title string `json:"title"`
	Text  string `json:"text"`
}
```

Add to `Contract` (all optional — `writing-project.json` sets none of them and must keep loading):

```go
	Goal           string          `json:"goal,omitempty"`
	Steps          []int           `json:"steps,omitempty"`
	Page           *PhasePage      `json:"page,omitempty"`
	Cards          []string        `json:"cards,omitempty"`
	AnchorMaterial *AnchorMaterial `json:"anchor_material,omitempty"`
	AskChips       []string        `json:"ask_chips,omitempty"`
	Floor          []FloorItem     `json:"floor,omitempty"`
	SoftCondition  string          `json:"soft_condition,omitempty"`
```

Add to `Skill`:

```go
	CourseID string `json:"course_id,omitempty"`
```

In `Validate()`, inside the existing `for id, c := range s.Contracts` loop, after the machine-kind check:

```go
		for _, f := range c.Floor {
			if !CourseFloorKinds[f.Kind] {
				return fmt.Errorf("skill %s: contract %s unknown floor kind %q", s.ID, id, f.Kind)
			}
			if f.CardID != "" && !contains(s.Cards, f.CardID) {
				return fmt.Errorf("skill %s: contract %s floor references card %s not in the skill's cards", s.ID, id, f.CardID)
			}
		}
		for _, cd := range c.Cards {
			if !contains(s.Cards, cd) {
				return fmt.Errorf("skill %s: contract %s declares card %s not in the skill's cards", s.ID, id, cd)
			}
		}
```

and, replacing the tail `if _, err := s.TopoOrder(); err != nil { return err }`:

```go
	// A course's phase order is BINDING (agent-spec §5.3): it is a chain, not a
	// general DAG. Enforce that at load — a branching course skill is a config
	// error, not a runtime surprise.
	if s.Kind == "course" {
		if _, err := s.LinearOrder(); err != nil {
			return err
		}
		return nil
	}
	if _, err := s.TopoOrder(); err != nil {
		return err
	}
	return nil
}

func contains(xs []string, x string) bool {
	for _, s := range xs {
		if s == x {
			return true
		}
	}
	return false
}

// LinearOrder returns a course skill's phases in their binding order. It errors
// unless the requires-chain is strictly linear: exactly one root, every other
// contract requiring exactly one predecessor, and no contract required by two
// successors. TopoOrder does the cycle check.
func (s Skill) LinearOrder() ([]string, error) {
	order, err := s.TopoOrder()
	if err != nil {
		return nil, err
	}
	successors := map[string]int{}
	roots := 0
	for id, c := range s.Contracts {
		switch len(c.Requires) {
		case 0:
			roots++
		case 1:
			successors[c.Requires[0]]++
		default:
			return nil, fmt.Errorf("skill %s: contract %s requires %d predecessors — a course order must be linear", s.ID, id, len(c.Requires))
		}
	}
	if roots != 1 {
		return nil, fmt.Errorf("skill %s: a course order must have exactly one first phase, found %d", s.ID, roots)
	}
	for id, n := range successors {
		if n > 1 {
			return nil, fmt.Errorf("skill %s: phase %s is followed by %d phases — a course order must be linear", s.ID, id, n)
		}
	}
	return order, nil
}
```

- [ ] **Step 4: Author the skill JSON**

Create `packages/contracts/skills/info-literacy-course.json` with exactly the config in spec §3 (copy it verbatim from the spec's code block — it is the authored content, not an example).

- [ ] **Step 5: Sync the Go mirror and run the FULL skills package**

```bash
cd apps/api && make sync-skills && CGO_ENABLED=0 go test -p 1 ./internal/skills/
```

Expected: `internal/skills/specs/info-literacy-course.json` appears; package `ok`. If `make sync-skills` fails, read the `syncskills` tool — do not hand-copy the file.

- [ ] **Step 6: Mirror the fields in Zod + test**

In `packages/contracts/src/skill.ts`, extend `Contract` and `Skill` with the same optional fields:

```ts
export const FloorItem = z.object({
  kind: z.enum(["steps_viewed", "card_dispositioned", "student_turns_at_least"]),
  steps: z.array(z.number().int()).optional(),
  card_id: z.string().optional(),
  n: z.number().int().optional(),
});
export type FloorItem = z.infer<typeof FloorItem>;

export const PhasePage = z.object({
  title: z.string(),
  subtitle: z.string().default(""),
  body: z.array(z.string()).default([]),
});
export type PhasePage = z.infer<typeof PhasePage>;

export const AnchorMaterial = z.object({ title: z.string(), text: z.string() });
export type AnchorMaterial = z.infer<typeof AnchorMaterial>;
```

Add to the `Contract` object: `goal: z.string().optional()`, `steps: z.array(z.number().int()).optional()`, `page: PhasePage.optional()`, `cards: z.array(z.string()).optional()`, `anchor_material: AnchorMaterial.optional()`, `ask_chips: z.array(z.string()).optional()`, `floor: z.array(FloorItem).optional()`, `soft_condition: z.string().optional()`. Add to `Skill`: `course_id: z.string().optional()`.

Append to `packages/contracts/test/skill.test.ts`:

```ts
import courseSkill from "../skills/info-literacy-course.json";
import projectSkill from "../skills/writing-project.json";

describe("course skill", () => {
  it("parses the authored course skill", () => {
    const r = Skill.safeParse(courseSkill);
    expect(r.success).toBe(true);
  });

  it("still parses the project skill unchanged", () => {
    expect(Skill.safeParse(projectSkill).success).toBe(true);
  });

  it("rejects a floor kind outside the closed set", () => {
    expect(FloorItem.safeParse({ kind: "vibes_ok" }).success).toBe(false);
  });
});
```

- [ ] **Step 7: Run the contracts suite**

```bash
cd packages/contracts && npx vitest run && npx tsc --noEmit
```

Expected: all green, tsc exit 0. If the JSON import needs `resolveJsonModule`, check `tsconfig` — `writing-project.json` is already imported somewhere; follow that precedent.

- [ ] **Step 8: Commit**

```bash
git add packages/contracts/src/skill.ts packages/contracts/test/skill.test.ts packages/contracts/skills/info-literacy-course.json apps/api/internal/skills/skill.go apps/api/internal/skills/skill_test.go apps/api/internal/skills/specs/info-literacy-course.json
git commit -m "feat(refactor2): Slice 12 T3 — course skill config, floor kinds, binding-order validation"
```

---

### Task 4: Migration 0023 + session queries

**Files:**
- Create: `apps/api/internal/store/migrations/0023_course_session_scope.sql`
- Create: `apps/api/internal/store/queries/coursesession.sql`
- Modify: `apps/api/internal/store/queries/material.sql`, `apps/api/internal/store/queries/card_instance.sql`
- Test: `apps/api/internal/store/course_session_scope_test.go`

**Interfaces:**
- Produces, for Tasks 6–7 (sqlc-generated): `CreateCourseSession`, `GetCourseSession`, `GetCourseSessionByUserCourse`, `SetCourseSessionPhase`, `CreateCourseMessage`, `ListMessagesBySession`, `CountStudentTurnsInPhase`, `CreateSessionMaterial`, `ListMaterialsBySession`, `CreateSessionCardInstance`, `ListCardInstancesBySession`, `SubmitSessionCardInstance`, `SetSessionCardInstanceStatus`. Read the generated params structs before using them — sqlc emits `*string` for nullable text and `pgtype.UUID` for **nullable** uuid FKs, while NOT-NULL uuid columns (every `id`, `user_id`) are plain `uuid.UUID`.

- [ ] **Step 1: Write the migration**

Create `apps/api/internal/store/migrations/0023_course_session_scope.sql` with exactly the SQL in spec §2 DEC-12.4, prefixed:

```sql
-- +goose Up
-- Slice 12: give Course's dialogue + card runtime a session-scoped home.
-- Additive, mirroring 0022's thread scope. A material / card_instance belongs
-- to exactly one owner: a project, a chat thread, a course session, or a
-- legacy task. course_step/course_progress are NOT touched — they are authored
-- content + page position; the phase layer lives here (DEC-12.1).
```

…then the DDL from the spec, then:

```sql
-- +goose Down
ALTER TABLE material       DROP CONSTRAINT IF EXISTS material_scope_ck;
ALTER TABLE card_instances DROP CONSTRAINT IF EXISTS card_instances_scope_ck;
ALTER TABLE material       ADD CONSTRAINT material_scope_ck
  CHECK (num_nonnulls(task_id, project_id, thread_id) >= 1);
ALTER TABLE card_instances ADD CONSTRAINT card_instances_scope_ck
  CHECK (num_nonnulls(task_id, project_id, thread_id) >= 1);
DROP INDEX IF EXISTS material_session_created_idx;
DROP INDEX IF EXISTS card_instances_session_created_idx;
ALTER TABLE material       DROP COLUMN IF EXISTS session_id;
ALTER TABLE card_instances DROP COLUMN IF EXISTS session_id;
DROP TABLE IF EXISTS course_message;
DROP TABLE IF EXISTS course_session;
```

- [ ] **Step 2: Write the queries**

Create `apps/api/internal/store/queries/coursesession.sql`:

```sql
-- course_session / course_message queries (Slice 12, Course policy).
-- A session is the runtime unit: one per (user, course), holding the phase.
-- course_progress stays the page-position unit and is queried separately —
-- it is also the server-side truth the steps_viewed floor reads (DEC-12.2).

-- name: CreateCourseSession :one
INSERT INTO course_session (user_id, course_id, skill_id, phase)
VALUES ($1, $2, $3, $4)
ON CONFLICT (user_id, course_id) DO UPDATE SET updated_at = now()
RETURNING *;

-- name: GetCourseSession :one
SELECT * FROM course_session WHERE id = $1;

-- name: GetCourseSessionByUserCourse :one
SELECT * FROM course_session WHERE user_id = $1 AND course_id = $2;

-- name: SetCourseSessionPhase :one
UPDATE course_session SET phase = $2, updated_at = now()
WHERE id = $1
RETURNING *;

-- name: CreateCourseMessage :one
INSERT INTO course_message (session_id, phase, role, content)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: ListMessagesBySession :many
SELECT * FROM course_message WHERE session_id = $1 ORDER BY created_at, id;

-- name: CountStudentTurnsInPhase :one
SELECT count(*) FROM course_message
WHERE session_id = $1 AND phase = $2 AND role = 'student';
```

Append to `apps/api/internal/store/queries/material.sql`:

```sql
-- Course session scope (Slice 12).

-- name: CreateSessionMaterial :one
INSERT INTO material (session_id, kind, source, title, blocks)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: ListMaterialsBySession :many
SELECT * FROM material WHERE session_id = $1 ORDER BY created_at, id;
```

Append to `apps/api/internal/store/queries/card_instance.sql`:

```sql
-- Course session scope (Slice 12).

-- name: CreateSessionCardInstance :one
INSERT INTO card_instances (session_id, material_id, card_id, tool_id, status)
VALUES ($1, $2, $3, $3, $4)
RETURNING *;

-- name: ListCardInstancesBySession :many
SELECT * FROM card_instances WHERE session_id = $1 ORDER BY created_at, id;

-- name: SubmitSessionCardInstance :one
UPDATE card_instances
SET field_values = $3, event_trace = $4, status = $5, completed_at = now()
WHERE id = $1 AND session_id = $2
RETURNING *;

-- name: SetSessionCardInstanceStatus :one
UPDATE card_instances SET status = $3
WHERE id = $1 AND session_id = $2
RETURNING *;
```

Before writing these, **read the existing `CreateThreadMaterial` / `CreateThreadCardInstance` / `SubmitThreadCardInstance` queries in the same files** and match their column lists — if `material` or `card_instances` has NOT-NULL columns beyond the ones above, include them the way the thread queries do. `SubmitSessionCardInstance` sets `completed_at = now()` deliberately: the thread equivalent does not, which was logged as a Slice-11 minor; do not repeat it.

- [ ] **Step 3: Generate + verify no drift**

```bash
cd apps/api && make sqlc && git status --short internal/store/sqlc/
```

Expected: new/updated generated files, no errors. `make sqlc` needs `CGO_ENABLED=0` on macOS — the Makefile handles it.

- [ ] **Step 4: Write the store test**

Create `apps/api/internal/store/course_session_scope_test.go` (package `store_test`, using the existing `newStoreTestPool` + `sqlc.New` + `seededStudentID` helpers — read a neighbouring `*_scope_test.go` for their exact signatures):

```go
func TestCourseSessionScope(t *testing.T) {
	ctx := context.Background()
	pool := newStoreTestPool(t)
	q := sqlc.New(pool)
	uid := seededStudentID(t, ctx, q)
	courseID := uuid.MustParse("00000000-0000-0000-0000-0000000000c1")

	s, err := q.CreateCourseSession(ctx, sqlc.CreateCourseSessionParams{
		UserID: uid, CourseID: courseID, SkillID: "info-literacy-course", Phase: "demonstrate",
	})
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	// One session per (user, course): re-creating resumes, never duplicates.
	again, err := q.CreateCourseSession(ctx, sqlc.CreateCourseSessionParams{
		UserID: uid, CourseID: courseID, SkillID: "info-literacy-course", Phase: "demonstrate",
	})
	if err != nil {
		t.Fatalf("re-create session: %v", err)
	}
	if again.ID != s.ID {
		t.Fatalf("second create minted a new session %s, want the existing %s", again.ID, s.ID)
	}

	// A session-scoped material satisfies the widened scope CHECK with only
	// session_id set — no task, no project, no thread.
	m, err := q.CreateSessionMaterial(ctx, sqlc.CreateSessionMaterialParams{
		SessionID: pgtype.UUID{Bytes: s.ID, Valid: true},
		Kind:      "claim", Source: "authored", Title: "待核实的说法", Blocks: []byte("[]"),
	})
	if err != nil {
		t.Fatalf("session material must satisfy the scope CHECK: %v", err)
	}

	ci, err := q.CreateSessionCardInstance(ctx, sqlc.CreateSessionCardInstanceParams{
		SessionID: pgtype.UUID{Bytes: s.ID, Valid: true},
		MaterialID: m.ID, CardID: "craap", Status: "proposed",
	})
	if err != nil {
		t.Fatalf("session card instance: %v", err)
	}

	cards, err := q.ListCardInstancesBySession(ctx, pgtype.UUID{Bytes: s.ID, Valid: true})
	if err != nil || len(cards) != 1 || cards[0].ID != ci.ID {
		t.Fatalf("ListCardInstancesBySession = %v (err %v), want the one card", cards, err)
	}

	// Phase moves.
	moved, err := q.SetCourseSessionPhase(ctx, sqlc.SetCourseSessionPhaseParams{ID: s.ID, Phase: "guided"})
	if err != nil || moved.Phase != "guided" {
		t.Fatalf("SetCourseSessionPhase = %+v (err %v), want phase guided", moved, err)
	}

	// Messages + the student-turn count the floor reads.
	for _, role := range []string{"student", "assistant", "student"} {
		if _, err := q.CreateCourseMessage(ctx, sqlc.CreateCourseMessageParams{
			SessionID: s.ID, Phase: "guided", Role: role, Content: "x",
		}); err != nil {
			t.Fatalf("create message: %v", err)
		}
	}
	n, err := q.CountStudentTurnsInPhase(ctx, sqlc.CountStudentTurnsInPhaseParams{SessionID: s.ID, Phase: "guided"})
	if err != nil || n != 2 {
		t.Fatalf("CountStudentTurnsInPhase = %d (err %v), want 2", n, err)
	}
	if n, err := q.CountStudentTurnsInPhase(ctx, sqlc.CountStudentTurnsInPhaseParams{SessionID: s.ID, Phase: "reflect"}); err != nil || n != 0 {
		t.Fatalf("turns in an untouched phase = %d (err %v), want 0", n, err)
	}
}
```

Adjust the exact param field names to whatever sqlc generated (Step 3's output is the truth) and report any difference.

- [ ] **Step 5: Run the FULL store package**

```bash
cd apps/api && CGO_ENABLED=0 go test -p 1 ./internal/store/...
```

Expected: `ok` — including the pre-existing migration test, which runs 0023 up and down. Docker must be running.

- [ ] **Step 6: Commit**

```bash
git add apps/api/internal/store/migrations/0023_course_session_scope.sql apps/api/internal/store/queries/coursesession.sql apps/api/internal/store/queries/material.sql apps/api/internal/store/queries/card_instance.sql apps/api/internal/store/sqlc/ apps/api/internal/store/course_session_scope_test.go
git commit -m "feat(refactor2): Slice 12 T4 — course_session/course_message + session scope (0023)"
```

---

### Task 5: The course coach

**Files:**
- Create: `apps/api/internal/agent/course_coach.go`
- Test: `apps/api/internal/agent/course_coach_test.go`

**Interfaces:**
- Consumes: `enforcement.AgentOutput` (+ `To`, Task 2); `skills.Contract` (+ `Goal`/`SoftCondition`/`AskChips`, Task 3); the existing `ChatTurn{Role, Content string}`; `gateway.Provider`/`Resolved`/`ChatUsage`/`Collect`.
- Produces, for Task 6:
  ```go
  type CourseScript struct {
      CourseTitle string
      PhaseTitles []string  // in binding order
      CurrentIdx  int
  }
  func BuildCourseContext(script CourseScript, phase skills.Contract, history []ChatTurn, cardSummary, intentHint string) string
  func ProposeCourseReply(ctx context.Context, prov gateway.Provider, r gateway.Resolved, ctxStr string) (enforcement.AgentOutput, gateway.ChatUsage, error)
  ```

Read `apps/api/internal/agent/chat_coach.go` first — this file mirrors it exactly in structure (system+user turns, `gateway.Collect`, JSON output parse, full enforcement stack, usage populated even on reject, **no** OutputCheck echo pass). The differences are the posture prompt and the context recipe.

- [ ] **Step 1: Write the failing tests**

Create `apps/api/internal/agent/course_coach_test.go`:

```go
func TestBuildCourseContextIsTheCourseRecipe(t *testing.T) {
	script := CourseScript{
		CourseTitle: "一条网络信息，该不该信",
		PhaseTitles: []string{"演示", "引导", "独立", "回看"},
		CurrentIdx:  1,
	}
	phase := skills.Contract{
		Goal:          "在这条真实说法上，带学生做一次完整的信源辨识",
		SoftCondition: "学生对这条说法做完了一次真实的溯源，不是走过场",
	}
	history := []ChatTurn{{Role: "student", Content: "这条是真的吧？"}, {Role: "assistant", Content: "你怎么看？"}}

	got := BuildCourseContext(script, phase, history, "CRAAP 卡：进行中", "advance")

	for _, want := range []string{
		"一条网络信息，该不该信", // the script
		"引导",                  // the current phase
		phase.Goal,             // the phase goal
		phase.SoftCondition,    // the condition being judged
		"这条是真的吧？",        // this phase's dialogue
		"CRAAP 卡：进行中",      // the active card instance
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("course context is missing %q:\n%s", want, got)
		}
	}
}

func TestProposeCourseReplyReturnsAReply(t *testing.T) {
	prov := scriptedProvider(`{"type":"reply","body":"你先说说，这条说法里哪一句最像是被加工过的？"}`)
	out, usage, err := ProposeCourseReply(context.Background(), prov, gateway.Resolved{}, "ctx")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.Type != "reply" || out.Body == "" {
		t.Fatalf("out = %+v, want a reply with a body", out)
	}
	if usage.InputTokens == 0 && usage.OutputTokens == 0 {
		t.Fatal("usage must be populated so the caller can meter the call")
	}
}

func TestProposeCourseReplyReturnsAnAdvance(t *testing.T) {
	prov := scriptedProvider(`{"type":"advance","to":"independent"}`)
	out, _, err := ProposeCourseReply(context.Background(), prov, gateway.Resolved{}, "ctx")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.Type != "advance" || out.To != "independent" {
		t.Fatalf("out = %+v, want advance to independent", out)
	}
}

func TestProposeCourseReplyMetersARejectedOutput(t *testing.T) {
	// A ghostwriting reply must be rejected by the enforcement stack — but the
	// tokens were already spent, so usage must still come back for the caller
	// to record an llm_call row (the metering-on-reject rule).
	prov := scriptedProvider(`{"type":"reply","body":"你可以这样写：中国的绿化成就无可否认。"}`)
	_, usage, err := ProposeCourseReply(context.Background(), prov, gateway.Resolved{}, "ctx")
	if err == nil {
		t.Fatal("a ghostwriting reply must be rejected")
	}
	if usage.InputTokens == 0 && usage.OutputTokens == 0 {
		t.Fatal("usage must be populated even on reject")
	}
}
```

`scriptedProvider` is the existing stub helper in this package (it wraps `gateway.NewStubProvider`) — read `chat_coach_test.go` for its exact signature. If the banned-phrasing suite does not reject the string in the last test, pick a phrase the suite *does* ban (read `enforcement`'s banned list) and use that instead; report which you used.

- [ ] **Step 2: Run and watch it fail**

```bash
cd apps/api && CGO_ENABLED=0 go test ./internal/agent/ -run TestBuildCourseContext
```

Expected: FAIL to compile — no `CourseScript`, `BuildCourseContext`, `ProposeCourseReply`.

- [ ] **Step 3: Implement**

Create `apps/api/internal/agent/course_coach.go`:

```go
package agent

// course_coach.go — the Course-variant coach (agent-spec §5.3). Course is the
// most permissive of the three postures: teaching IS the point here, so the
// coach may explain and demonstrate — but it still never produces the
// student's assessed deliverable, and the consolidation rule holds. Runs on
// the chaperone (mid-tier) resolver like the chat coach, not the flagship.

const courseCoachPosturePrompt = `你是「思维印记」的课程陪练。你在带一节课，按脚本走。

- 可以讲解、可以演示：这是学习空间，把方法讲清楚是你的职责。但绝不替学生写出他要被评估的成品，绝不替他下结论。
- 一次只问一个：回复简短，顺着学生的话往深里带一步。
- 克制：当学生已经在思考时，别打断；当他想让你替他想时，把问题还给他。
- 工具卡的「框架」在用过之后才揭示，不在用之前——先做，再命名。
- 你不能重排或跳过阶段，不能替换这一阶段声明的卡。

输出必须是一个 JSON 对象，只能是下面两种之一：
{"type":"reply","body":"你要对学生说的话"}
{"type":"advance","to":"下一阶段的 id"}

只有当这一阶段的完成条件真的达成时，才输出 advance；否则输出 reply，把还差的那一步说给学生。`

// CourseScript is the session script the coach is executing: what course this
// is, its phases in binding order, and where we are.
type CourseScript struct {
	CourseTitle string
	PhaseTitles []string
	CurrentIdx  int
}

// BuildCourseContext is the course context recipe (agent-spec §5.3) and
// nothing more: the session script + the current phase goal + this phase's
// dialogue + the active card instance. Explicitly NOT the project graph — a
// course's graph is small and session-scoped. Pure; its own test.
func BuildCourseContext(script CourseScript, phase skills.Contract, history []ChatTurn, cardSummary, intentHint string) string {
	var b strings.Builder
	b.WriteString("课程：" + script.CourseTitle + "\n")
	b.WriteString("脚本（阶段顺序，不可重排）：" + strings.Join(script.PhaseTitles, " → ") + "\n")
	if script.CurrentIdx >= 0 && script.CurrentIdx < len(script.PhaseTitles) {
		b.WriteString("当前阶段：" + script.PhaseTitles[script.CurrentIdx] + "\n")
	}
	b.WriteString("这一阶段的目标：" + phase.Goal + "\n")
	if phase.SoftCondition != "" {
		b.WriteString("这一阶段的完成条件：" + phase.SoftCondition + "\n")
	}
	if cardSummary != "" {
		b.WriteString("当前工具卡：" + cardSummary + "\n")
	}
	b.WriteString("\n这一阶段的对话（从旧到新）：\n")
	for _, t := range history {
		who := "学生"
		if t.Role == "assistant" {
			who = "你"
		}
		b.WriteString(who + "：" + t.Content + "\n")
	}
	switch intentHint {
	case "advance":
		b.WriteString("\n学生想进入下一阶段。判断完成条件是否真的达成：达成就 advance，没达成就用 reply 告诉他还差什么。\n")
	default:
		b.WriteString("\n学生问了你一个问题。用 reply 回应他。\n")
	}
	return b.String()
}

// ProposeCourseReply runs one course-coach call through the full enforcement
// stack. Mirrors ProposeChatReply: no anchor, no criterion, no OutputCheck
// echo pass (Course has no draft to echo). Usage is returned even on reject —
// the tokens were spent, so the caller must still meter the call.
func ProposeCourseReply(ctx context.Context, prov gateway.Provider, r gateway.Resolved, ctxStr string) (enforcement.AgentOutput, gateway.ChatUsage, error) {
	req := gateway.ChatRequest{
		Messages: []gateway.ChatMessage{
			{Role: gateway.RoleSystem, Content: courseCoachPosturePrompt},
			{Role: gateway.RoleUser, Content: ctxStr},
		},
	}
	res, err := gateway.Collect(ctx, prov, r, req)
	if err != nil {
		return enforcement.AgentOutput{}, gateway.ChatUsage{}, err
	}
	// From here on, usage is non-zero and MUST travel with every return.
	usage := res.Usage
	var out enforcement.AgentOutput
	if err := json.Unmarshal([]byte(res.Content), &out); err != nil {
		return enforcement.AgentOutput{}, usage, fmt.Errorf("course coach: parse output: %w", err)
	}
	if err := enforcement.ValidateOutput(out); err != nil {
		return enforcement.AgentOutput{}, usage, err
	}
	if err := enforcement.CheckBannedPhrasing(out.Body); err != nil {
		return enforcement.AgentOutput{}, usage, err
	}
	return out, usage, nil
}
```

Match `chat_coach.go`'s exact call shapes — `gateway.Collect`'s return type, the usage field names, and the banned-phrasing function's real name (`CheckBannedPhrasing` above is this plan's guess; use whatever `chat_coach.go` calls). Report any difference.

- [ ] **Step 4: Run the FULL agent package**

```bash
cd apps/api && CGO_ENABLED=0 go test -p 1 ./internal/agent/
```

Expected: `ok` (includes the package's testcontainer integration tests).

- [ ] **Step 5: Commit**

```bash
git add apps/api/internal/agent/course_coach.go apps/api/internal/agent/course_coach_test.go
git commit -m "feat(refactor2): Slice 12 T5 — course coach (posture, context recipe, propose)"
```

---

### Task 6: The course runtime — floor, phase walk, card candidate, `RunCourseStep`

**Files:**
- Create: `apps/api/internal/agent/course_step.go`
- Create: `apps/api/internal/agent/coursestore.go`
- Test: `apps/api/internal/agent/course_step_test.go`

**Interfaces:**
- Consumes: Task 1's `ScopedMaterial`/`ScopedCard`/`CardOffer`; Task 3's `skills.Skill`/`Contract`/`FloorItem`/`CourseFloorKinds`/`LinearOrder`; Task 5's `CourseScript`/`BuildCourseContext`/`ProposeCourseReply`; Task 4's generated queries.
- Produces, for Task 7: the `CourseStore` interface, `CourseDeps`, `CourseIntent`, `CourseStepResult`, `RunCourseStep`, `NewSqlcCourseStore(q *sqlc.Queries) CourseStore`, `CourseSession{ID uuid.UUID; CourseID uuid.UUID; SkillID, Phase string}` — as spec §4 declares them, with **one addition**: `CourseDeps` carries a `CourseTitle string` (the script's course name for the context recipe). The API handler has the `course` row already; the store seam does not need a query for it.

Read `apps/api/internal/agent/chat_step.go` and `chatstore.go` first — this pair mirrors them. Nothing from `RunAgentStep` is reused (it is project-graph-coupled).

- [ ] **Step 1: Write the failing pure-helper tests**

Create `apps/api/internal/agent/course_step_test.go`:

```go
func courseTestSkill() skills.Skill {
	return skills.Skill{
		ID: "t", Kind: "course", Cards: []string{"craap"},
		Contracts: map[string]skills.Contract{
			"demonstrate": {Title: "演示", Goal: "g0", SoftCondition: "s0",
				Steps: []int{0, 1},
				Floor: []skills.FloorItem{{Kind: "steps_viewed", Steps: []int{0, 1}}}},
			"guided": {Title: "引导", Goal: "g1", SoftCondition: "s1", Requires: []string{"demonstrate"},
				Cards:          []string{"craap"},
				AnchorMaterial: &skills.AnchorMaterial{Title: "待核实的说法", Text: "卫星图显示……"},
				Floor:          []skills.FloorItem{{Kind: "card_dispositioned", CardID: "craap"}}},
			"reflect": {Title: "回看", Goal: "g2", SoftCondition: "s2", Requires: []string{"guided"},
				Floor: []skills.FloorItem{{Kind: "student_turns_at_least", N: 1}}},
		},
	}
}

func TestNextPhaseWalksTheBindingOrder(t *testing.T) {
	sk := courseTestSkill()
	for _, c := range []struct{ cur, want string }{{"demonstrate", "guided"}, {"guided", "reflect"}} {
		got, ok := NextPhase(sk, c.cur)
		if !ok || got != c.want {
			t.Fatalf("NextPhase(%s) = %q,%v want %q,true", c.cur, got, ok, c.want)
		}
	}
	if _, ok := NextPhase(sk, "reflect"); ok {
		t.Fatal("the last phase has no successor")
	}
	if _, ok := NextPhase(sk, "nope"); ok {
		t.Fatal("an unknown phase has no successor")
	}
}

func TestCheckFloor(t *testing.T) {
	viewed := FloorState{ViewedSteps: []int{0, 1}}
	if unmet := CheckFloor([]skills.FloorItem{{Kind: "steps_viewed", Steps: []int{0, 1}}}, viewed); len(unmet) != 0 {
		t.Fatalf("all steps viewed → floor met, got unmet %+v", unmet)
	}
	if unmet := CheckFloor([]skills.FloorItem{{Kind: "steps_viewed", Steps: []int{0, 1, 2}}}, viewed); len(unmet) != 1 {
		t.Fatalf("a missing step → floor unmet, got %+v", unmet)
	}

	// DEC-12.2's no-dead-end guarantee: a SKIPPED card satisfies the floor. A
	// card is an offer; a floor only a completed card can satisfy would turn
	// the offer into a wall.
	item := []skills.FloorItem{{Kind: "card_dispositioned", CardID: "craap"}}
	for _, st := range []FloorState{
		{DispositionedCards: []string{"craap"}},
	} {
		if unmet := CheckFloor(item, st); len(unmet) != 0 {
			t.Fatalf("a dispositioned card meets the floor, got unmet %+v", unmet)
		}
	}
	if unmet := CheckFloor(item, FloorState{}); len(unmet) != 1 {
		t.Fatalf("an untouched card leaves the floor unmet, got %+v", unmet)
	}

	if unmet := CheckFloor([]skills.FloorItem{{Kind: "student_turns_at_least", N: 2}}, FloorState{StudentTurns: 2}); len(unmet) != 0 {
		t.Fatalf("turns met → floor met, got %+v", unmet)
	}
	if unmet := CheckFloor([]skills.FloorItem{{Kind: "student_turns_at_least", N: 2}}, FloorState{StudentTurns: 1}); len(unmet) != 1 {
		t.Fatalf("turns short → floor unmet, got %+v", unmet)
	}

	// Fail closed: Load rejects an unknown kind, so this is defense in depth.
	if unmet := CheckFloor([]skills.FloorItem{{Kind: "vibes_ok"}}, FloorState{}); len(unmet) != 1 {
		t.Fatalf("an unknown floor kind must count as UNMET, got %+v", unmet)
	}
}

func TestCourseCardCandidateSurfacesOnce(t *testing.T) {
	sk := courseTestSkill()
	id, ok := CourseCardCandidate(sk, "guided", nil)
	if !ok || id != "craap" {
		t.Fatalf("guided must offer craap, got %q,%v", id, ok)
	}
	if _, ok := CourseCardCandidate(sk, "demonstrate", nil); ok {
		t.Fatal("a phase with no declared card offers nothing")
	}
	for _, status := range []string{"proposed", "active", "completed", "skipped"} {
		if _, ok := CourseCardCandidate(sk, "guided", []ScopedCard{{CardID: "craap", Status: status}}); ok {
			t.Fatalf("an existing craap card in status %s must not be re-offered", status)
		}
	}
}
```

- [ ] **Step 2: Run and watch it fail**

```bash
cd apps/api && CGO_ENABLED=0 go test ./internal/agent/ -run 'TestNextPhase|TestCheckFloor|TestCourseCardCandidate'
```

Expected: FAIL to compile — no `NextPhase`, `FloorState`, `CheckFloor`, `CourseCardCandidate`.

- [ ] **Step 3: Implement the pure helpers + the types**

Create `apps/api/internal/agent/course_step.go` starting with:

```go
package agent

// course_step.go — the Course runtime (agent-spec §5.3): the coach executes an
// authored script, planner OFF, `advance` held by the coach behind a
// structural floor. Mirrors chat_step.go's shape; shares nothing with
// RunAgentStep, which is project-graph-coupled.

// CourseSession is the runtime's view of one student's run through one course.
type CourseSession struct {
	ID       uuid.UUID
	CourseID uuid.UUID
	SkillID  string
	Phase    string
}

// FloorState is everything the phase floor is computed from. Every field comes
// from the STORE — never from the client (DEC-12.2): a floor the client can
// assert is not a floor.
type FloorState struct {
	ViewedSteps        []int
	DispositionedCards []string // session cards in status completed OR skipped
	StudentTurns       int
}

// NextPhase returns the current phase's single successor in the skill's binding
// order. Course advance is forward-only, one phase at a time.
func NextPhase(sk skills.Skill, cur string) (string, bool) {
	order, err := sk.LinearOrder()
	if err != nil {
		return "", false
	}
	for i, id := range order {
		if id == cur && i+1 < len(order) {
			return order[i+1], true
		}
	}
	return "", false
}

// CheckFloor evaluates the closed set of course floor kinds and returns the
// items that are NOT met. The floor is the structural half of phase advance:
// it can only refuse. An unknown kind counts as unmet — Load already rejects
// it, so this is defense in depth, and failing closed is the safe direction.
func CheckFloor(items []skills.FloorItem, st FloorState) []skills.FloorItem {
	var unmet []skills.FloorItem
	for _, f := range items {
		ok := false
		switch f.Kind {
		case "steps_viewed":
			ok = true
			for _, want := range f.Steps {
				if !containsInt(st.ViewedSteps, want) {
					ok = false
					break
				}
			}
		case "card_dispositioned":
			ok = containsStr(st.DispositionedCards, f.CardID)
		case "student_turns_at_least":
			ok = st.StudentTurns >= f.N
		}
		if !ok {
			unmet = append(unmet, f)
		}
	}
	return unmet
}

// CourseCardCandidate returns the phase's declared card, unless a session card
// instance for it already exists in ANY status — a card is surfaced once per
// session, the same offer discipline Chat uses.
func CourseCardCandidate(sk skills.Skill, phase string, cards []ScopedCard) (string, bool) {
	c, ok := sk.Contracts[phase]
	if !ok || len(c.Cards) == 0 {
		return "", false
	}
	want := c.Cards[0]
	for _, existing := range cards {
		if existing.CardID == want {
			return "", false
		}
	}
	return want, true
}

func containsInt(xs []int, x int) bool {
	for _, v := range xs {
		if v == x {
			return true
		}
	}
	return false
}

func containsStr(xs []string, x string) bool {
	for _, v := range xs {
		if v == x {
			return true
		}
	}
	return false
}
```

- [ ] **Step 4: Run and watch them pass**

```bash
cd apps/api && CGO_ENABLED=0 go test ./internal/agent/ -run 'TestNextPhase|TestCheckFloor|TestCourseCardCandidate'
```

Expected: PASS.

- [ ] **Step 5: Write the failing `RunCourseStep` tests**

Append to `course_step_test.go` a `fakeCourseStore` implementing `CourseStore` with counters (`llmCalls`, `assistantMsgs`, `studentMsgs`, `materialsCreated`, `cardsCreated`, `phaseSets`, `events`), then:

```go
func TestRunCourseStepAskGetsAReply(t *testing.T) {
	st := newFakeCourseStore("demonstrate")
	deps := CourseDeps{
		Store: st, Provider: scriptedProvider(`{"type":"reply","body":"你觉得这句话里，哪一部分是证据？"}`),
		Resolved: gateway.Resolved{}, Skill: courseTestSkill(), UserID: uuid.New(), SessionID: st.session.ID,
	}
	res, err := RunCourseStep(context.Background(), deps, "ask", "这条是真的吗？")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Reply == "" {
		t.Fatal("an ask must produce a reply")
	}
	if res.Advanced != "" {
		t.Fatalf("an ask must never advance, got %q", res.Advanced)
	}
	if st.studentMsgs != 1 || st.assistantMsgs != 1 {
		t.Fatalf("both turns must be persisted, got student=%d assistant=%d", st.studentMsgs, st.assistantMsgs)
	}
	if st.llmCalls != 1 {
		t.Fatalf("llmCalls = %d, want exactly 1", st.llmCalls)
	}
}

func TestRunCourseStepUnmetFloorNeverCallsTheModel(t *testing.T) {
	// demonstrate's floor wants steps 0 and 1 viewed; the store reports none.
	st := newFakeCourseStore("demonstrate")
	st.viewedSteps = nil
	deps := CourseDeps{
		Store: st, Provider: scriptedProvider(`{"type":"advance","to":"guided"}`),
		Resolved: gateway.Resolved{}, Skill: courseTestSkill(), UserID: uuid.New(), SessionID: st.session.ID,
	}
	res, err := RunCourseStep(context.Background(), deps, "request_advance", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Advanced != "" {
		t.Fatal("an unmet floor must NOT advance, whatever the model would have said")
	}
	if st.llmCalls != 0 {
		t.Fatalf("an unmet floor must short-circuit BEFORE the model: llmCalls = %d, want 0", st.llmCalls)
	}
	if res.Reply == "" {
		t.Fatal("a refused advance must tell the student what is still owed")
	}
	if st.phaseSets != 0 {
		t.Fatal("the phase must not move")
	}
}

func TestRunCourseStepMetFloorAdvances(t *testing.T) {
	st := newFakeCourseStore("demonstrate")
	st.viewedSteps = []int{0, 1}
	deps := CourseDeps{
		Store: st, Provider: scriptedProvider(`{"type":"advance","to":"guided"}`),
		Resolved: gateway.Resolved{}, Skill: courseTestSkill(), UserID: uuid.New(), SessionID: st.session.ID,
	}
	res, err := RunCourseStep(context.Background(), deps, "request_advance", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Advanced != "guided" {
		t.Fatalf("Advanced = %q, want guided", res.Advanced)
	}
	if st.phaseSets != 1 || st.session.Phase != "guided" {
		t.Fatalf("the phase must move once, got sets=%d phase=%s", st.phaseSets, st.session.Phase)
	}
	if !st.hasEvent("phase_advanced") {
		t.Fatal("advancing must emit phase_advanced")
	}
}

func TestRunCourseStepRejectsAdvanceToANonSuccessor(t *testing.T) {
	// The coach may not skip a phase — forward-only, one at a time, enforced
	// by the runtime rather than requested by the prompt.
	st := newFakeCourseStore("demonstrate")
	st.viewedSteps = []int{0, 1}
	deps := CourseDeps{
		Store: st, Provider: scriptedProvider(`{"type":"advance","to":"reflect"}`),
		Resolved: gateway.Resolved{}, Skill: courseTestSkill(), UserID: uuid.New(), SessionID: st.session.ID,
	}
	res, err := RunCourseStep(context.Background(), deps, "request_advance", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Advanced != "" || st.phaseSets != 0 {
		t.Fatalf("a jump past guided must be refused, got Advanced=%q sets=%d", res.Advanced, st.phaseSets)
	}
}

func TestRunCourseStepSurfacesThePhaseCardOnce(t *testing.T) {
	st := newFakeCourseStore("guided")
	deps := CourseDeps{
		Store: st, Provider: scriptedProvider(`{"type":"reply","body":"我们一起来查查这条说法。"}`),
		Resolved: gateway.Resolved{}, Skill: courseTestSkill(), UserID: uuid.New(), SessionID: st.session.ID,
	}
	res, err := RunCourseStep(context.Background(), deps, "ask", "接下来做什么？")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Offer == nil || res.Offer.CardID != "craap" {
		t.Fatalf("guided must offer craap, got %+v", res.Offer)
	}
	if st.materialsCreated != 1 || st.cardsCreated != 1 {
		t.Fatalf("one anchor material + one card, got m=%d c=%d", st.materialsCreated, st.cardsCreated)
	}

	// Second turn: the card exists now, so it must not be offered again.
	res2, err := RunCourseStep(context.Background(), deps, "ask", "还有别的吗？")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res2.Offer != nil {
		t.Fatal("the card must be surfaced once per session")
	}
	if st.materialsCreated != 1 || st.cardsCreated != 1 {
		t.Fatalf("no second mint, got m=%d c=%d", st.materialsCreated, st.cardsCreated)
	}
}

func TestRunCourseStepRejectedOutputIsSilentButMetered(t *testing.T) {
	st := newFakeCourseStore("demonstrate")
	deps := CourseDeps{
		Store: st, Provider: scriptedProvider(`{"type":"reply","body":"你可以这样写：中国的绿化成就无可否认。"}`),
		Resolved: gateway.Resolved{}, Skill: courseTestSkill(), UserID: uuid.New(), SessionID: st.session.ID,
	}
	res, err := RunCourseStep(context.Background(), deps, "ask", "帮我写一句")
	if err != nil {
		t.Fatalf("a rejected output is silence, NOT an error: %v", err)
	}
	if res.Reply != "" {
		t.Fatal("a rejected output must produce no reply")
	}
	if st.assistantMsgs != 0 {
		t.Fatal("a rejected output must persist no assistant message")
	}
	if st.llmCalls != 1 {
		t.Fatalf("the tokens were spent — llmCalls = %d, want 1", st.llmCalls)
	}
}
```

- [ ] **Step 6: Run and watch them fail**

```bash
cd apps/api && CGO_ENABLED=0 go test ./internal/agent/ -run TestRunCourseStep
```

Expected: FAIL to compile — no `CourseStore`, `CourseDeps`, `RunCourseStep`.

- [ ] **Step 7: Implement `RunCourseStep`**

Append to `course_step.go` the `CourseStore` interface, `CourseDeps`, `CourseIntent`, `CourseStepResult` exactly as spec §4 declares them, then:

```go
// RunCourseStep drives one course turn. Two intents:
//   ask            — the student asked something; the coach replies (and may
//                    surface this phase's declared card as an offer).
//   request_advance — the student wants the next phase. The STRUCTURAL FLOOR is
//                    checked first, in Go, with store-side inputs: unmet means
//                    no model call at all. Only with the floor met does the
//                    coach judge the soft condition (DEC-12.2).
func RunCourseStep(ctx context.Context, deps CourseDeps, intent CourseIntent, studentMessage string) (CourseStepResult, error) {
	sess, err := deps.Store.GetSession(ctx, deps.SessionID)
	if err != nil {
		return CourseStepResult{}, err
	}
	phase, ok := deps.Skill.Contracts[sess.Phase]
	if !ok {
		return CourseStepResult{}, fmt.Errorf("course step: session phase %q is not in skill %s", sess.Phase, deps.Skill.ID)
	}

	if intent == "request_advance" {
		return runCourseAdvance(ctx, deps, sess, phase)
	}
	return runCourseAsk(ctx, deps, sess, phase, studentMessage)
}
```

…then the two flow bodies (spec §4 steps 2–4) and their helpers:

```go
// buildScript renders the session script the coach executes.
func buildScript(sk skills.Skill, courseTitle, cur string) CourseScript {
	order, err := sk.LinearOrder()
	if err != nil {
		return CourseScript{CourseTitle: courseTitle, CurrentIdx: -1}
	}
	titles := make([]string, 0, len(order))
	idx := -1
	for i, id := range order {
		titles = append(titles, sk.Contracts[id].Title)
		if id == cur {
			idx = i
		}
	}
	return CourseScript{CourseTitle: courseTitle, PhaseTitles: titles, CurrentIdx: idx}
}

// cardSummary describes the phase's card instance for the context recipe.
func cardSummary(sk skills.Skill, phase skills.Contract, cards []ScopedCard) string {
	if len(phase.Cards) == 0 {
		return ""
	}
	for _, c := range cards {
		if c.CardID == phase.Cards[0] {
			return phase.Cards[0] + "：" + c.Status
		}
	}
	return ""
}

// floorOwedText says what an unmet floor is still waiting for — one plain
// sentence, authored, no model call. This is what a refused advance looks
// like: a sentence in the ask panel, never a lock (铁律 2).
func floorOwedText(unmet []skills.FloorItem, phase skills.Contract) string {
	if len(unmet) == 0 {
		return ""
	}
	switch unmet[0].Kind {
	case "steps_viewed":
		return "这一节还有没看完的部分——先看完，我们再往下走。"
	case "card_dispositioned":
		return "先把这张卡过一遍吧——做完，或者你觉得不需要就跳过，都行。"
	case "student_turns_at_least":
		return "先说说你的想法，哪怕只有一句。说完我们就继续。"
	default:
		return "这一阶段还差一点：" + phase.Goal
	}
}

// meter records the llm_call row whenever tokens were spent — including when
// enforcement rejected the output. The call happened; the cost is real.
func meter(ctx context.Context, deps CourseDeps, usage gateway.ChatUsage) {
	if usage.InputTokens == 0 && usage.OutputTokens == 0 {
		return
	}
	if err := deps.Store.RecordCourseLLMCall(ctx, deps.UserID, deps.Resolved, usage.InputTokens, usage.OutputTokens); err != nil {
		slog.Warn("course step: record llm_call failed", "err", err)
	}
}

func runCourseAsk(ctx context.Context, deps CourseDeps, sess CourseSession, phase skills.Contract, studentMessage string) (CourseStepResult, error) {
	if _, err := deps.Store.CreateSessionMessage(ctx, sess.ID, sess.Phase, "student", studentMessage); err != nil {
		return CourseStepResult{}, err
	}
	history, err := deps.Store.LoadPhaseHistory(ctx, sess.ID, sess.Phase, 12)
	if err != nil {
		return CourseStepResult{}, err
	}
	cards, err := deps.Store.ListSessionCards(ctx, sess.ID)
	if err != nil {
		return CourseStepResult{}, err
	}

	script := buildScript(deps.Skill, deps.CourseTitle, sess.Phase)
	out, usage, err := ProposeCourseReply(ctx, deps.Provider, deps.Resolved,
		BuildCourseContext(script, phase, history, cardSummary(deps.Skill, phase, cards), "ask"))
	meter(ctx, deps, usage)
	if err != nil {
		// Silence, not an error: a rejected output is never shown, never
		// persisted, and never surfaced to the student as a failure.
		slog.Warn("course ask: output rejected — staying silent", "err", err, "phase", sess.Phase)
		return CourseStepResult{}, nil
	}
	if out.Type != "reply" {
		// The coach does not advance on a question. An advance here is a
		// script error, not a phase transition.
		slog.Warn("course ask: non-reply output ignored", "type", out.Type, "phase", sess.Phase)
		return CourseStepResult{}, nil
	}
	if _, err := deps.Store.CreateSessionMessage(ctx, sess.ID, sess.Phase, "assistant", out.Body); err != nil {
		return CourseStepResult{}, err
	}

	res := CourseStepResult{Reply: out.Body}
	cardID, ok := CourseCardCandidate(deps.Skill, sess.Phase, cards)
	if !ok || phase.AnchorMaterial == nil {
		return res, nil
	}
	matID, err := deps.Store.CreateSessionMaterial(ctx, sess.ID, phase.AnchorMaterial.Title, phase.AnchorMaterial.Text)
	if err != nil {
		return CourseStepResult{}, err
	}
	ciID, err := deps.Store.CreateSessionCardInstance(ctx, sess.ID, cardID, matID)
	if err != nil {
		return CourseStepResult{}, err
	}
	if err := deps.Store.InsertUserEvent(ctx, deps.UserID, "course", "card_surfaced", []byte(`{}`)); err != nil {
		slog.Warn("course ask: append card_surfaced event failed", "err", err)
	}
	res.Offer = &CardOffer{CardInstanceID: ciID, MaterialID: matID, CardID: cardID}
	return res, nil
}

func runCourseAdvance(ctx context.Context, deps CourseDeps, sess CourseSession, phase skills.Contract) (CourseStepResult, error) {
	// The floor first, from store-side inputs only. An unmet floor short-
	// circuits BEFORE the model: no call, no tokens, no advance (DEC-12.2).
	viewed, err := deps.Store.ViewedSteps(ctx, deps.UserID, sess.CourseID)
	if err != nil {
		return CourseStepResult{}, err
	}
	cards, err := deps.Store.ListSessionCards(ctx, sess.ID)
	if err != nil {
		return CourseStepResult{}, err
	}
	turns, err := deps.Store.CountStudentTurns(ctx, sess.ID, sess.Phase)
	if err != nil {
		return CourseStepResult{}, err
	}
	var dispositioned []string
	for _, c := range cards {
		if c.Status == "completed" || c.Status == "skipped" {
			dispositioned = append(dispositioned, c.CardID)
		}
	}
	st := FloorState{ViewedSteps: intsOf(viewed), DispositionedCards: dispositioned, StudentTurns: turns}

	if unmet := CheckFloor(phase.Floor, st); len(unmet) > 0 {
		owed := floorOwedText(unmet, phase)
		if _, err := deps.Store.CreateSessionMessage(ctx, sess.ID, sess.Phase, "assistant", owed); err != nil {
			return CourseStepResult{}, err
		}
		return CourseStepResult{Reply: owed}, nil
	}

	next, ok := NextPhase(deps.Skill, sess.Phase)
	if !ok {
		done := "这节课到这里就走完了。"
		if _, err := deps.Store.CreateSessionMessage(ctx, sess.ID, sess.Phase, "assistant", done); err != nil {
			return CourseStepResult{}, err
		}
		return CourseStepResult{Reply: done}, nil
	}

	history, err := deps.Store.LoadPhaseHistory(ctx, sess.ID, sess.Phase, 12)
	if err != nil {
		return CourseStepResult{}, err
	}
	script := buildScript(deps.Skill, deps.CourseTitle, sess.Phase)
	out, usage, err := ProposeCourseReply(ctx, deps.Provider, deps.Resolved,
		BuildCourseContext(script, phase, history, cardSummary(deps.Skill, phase, cards), "advance"))
	meter(ctx, deps, usage)
	if err != nil {
		slog.Warn("course advance: output rejected — staying silent", "err", err, "phase", sess.Phase)
		return CourseStepResult{}, nil
	}

	// Forward-only, one phase at a time — enforced here, not requested in the
	// prompt. An advance naming anything but the single successor is refused.
	if out.Type == "advance" && out.To == next {
		if err := deps.Store.SetSessionPhase(ctx, sess.ID, next); err != nil {
			return CourseStepResult{}, err
		}
		if err := deps.Store.InsertUserEvent(ctx, deps.UserID, "course", "phase_advanced", []byte(`{}`)); err != nil {
			slog.Warn("course advance: append phase_advanced event failed", "err", err)
		}
		return CourseStepResult{Advanced: next}, nil
	}
	if out.Type == "advance" {
		slog.Warn("course advance: refused a non-successor target", "to", out.To, "want", next)
		return CourseStepResult{}, nil
	}
	if _, err := deps.Store.CreateSessionMessage(ctx, sess.ID, sess.Phase, "assistant", out.Body); err != nil {
		return CourseStepResult{}, err
	}
	return CourseStepResult{Reply: out.Body}, nil
}

func intsOf(xs []int32) []int {
	out := make([]int, 0, len(xs))
	for _, x := range xs {
		out = append(out, int(x))
	}
	return out
}
```

`usage.InputTokens`/`OutputTokens` above assume `gateway.ChatUsage`'s field names and `int32` types — verify against `chat_coach.go`/`chatstore.go`'s real usage and adjust `meter`'s signature if they differ; report any difference.

Then create `apps/api/internal/agent/coursestore.go` — the sqlc adapter, mirroring `chatstore.go`'s conventions exactly (`pgUUID` for the nullable FK params `material.session_id` / `card_instances.session_id` / `event.project_id` / `llm_call.project_id`; plain `uuid.UUID` for NOT-NULL columns and row `ID`s). `RecordCourseLLMCall` records `Surface: "course"`, `Purpose: "coach"`, `ProjectID: pgtype.UUID{Valid: false}`. `ViewedSteps` reads `course_progress.completed_ordinals` via the existing progress query — read `internal/api/course.go`'s `getCourseProgress` for its name.

- [ ] **Step 8: Run the FULL agent package**

```bash
cd apps/api && CGO_ENABLED=0 go build ./... && CGO_ENABLED=0 go test -p 1 ./internal/agent/
```

Expected: `ok`, all new tests passing, all Chat/Studio tests unaffected.

- [ ] **Step 9: Commit**

```bash
git add apps/api/internal/agent/course_step.go apps/api/internal/agent/coursestore.go apps/api/internal/agent/course_step_test.go
git commit -m "feat(refactor2): Slice 12 T6 — RunCourseStep, phase floor, session card offer, CourseStore"
```

---

### Task 7: The six endpoints + DTOs + web client

**Files:**
- Create: `apps/api/internal/api/course_session.go`, `apps/api/internal/api/course_session_dto.go`
- Modify: `apps/api/internal/api/api.go` (routes)
- Modify: `apps/api/internal/api/studioturn.go` (one new emitter method)
- Modify: `packages/contracts/src/course.ts`
- Create: `apps/web/src/api/courseSession.ts`; modify the api barrel (`apps/web/src/api/index.ts` — follow how `chat.ts` is exported)
- Test: `apps/api/internal/api/course_session_test.go`, `apps/web/src/api/courseSession.test.ts`

**Interfaces:**
- Consumes: Task 6's `agent.CourseDeps`/`RunCourseStep`/`NewSqlcCourseStore`/`CourseStepResult`; Task 3's skill loader; Task 4's queries.
- Produces, for Task 8:
  ```ts
  export type CourseTurnEvent =
    | { type: "reply"; body: string }
    | { type: "card"; cardInstanceId: string; cardId: string; materialId: string }
    | { type: "phase"; to: string }
    | { type: "done" }
    | { type: "error"; code: string; message: string };
  export function startCourseSession(courseId: string): Promise<CourseSession>;
  export function getCourseSession(courseId: string): Promise<CourseSession>;
  export function courseAsk(courseId: string, userInput: string): AsyncGenerator<CourseTurnEvent>;
  export function courseAdvance(courseId: string): AsyncGenerator<CourseTurnEvent>;
  export function submitCourseCard(courseId: string, cardInstanceId: string, body: { field_values: unknown; event_trace: unknown; anchors: unknown }): Promise<void>;
  export function skipCourseCard(courseId: string, cardInstanceId: string): Promise<void>;
  ```
  `CourseSession = { id, courseId, phase, phaseTitle, status, messages: CourseMessage[] }`; `CourseMessage = { id, phase, role, content, createdAt }`.

Read `apps/api/internal/api/chat.go` + `chat_dto.go` + `apps/web/src/api/chat.ts` first — every handler here mirrors its chat sibling (entitlement gate before the stream, persist-before-run, ownership 404-not-403, thin card submit/skip). List endpoints return **bare JSON**, not wrapped.

- [ ] **Step 1: Add the `phase` SSE frame to the emitter**

In `apps/api/internal/api/studioturn.go`, beside the existing `Card`/`Text` methods:

```go
// Phase announces a course phase transition (Slice 12). Mutex-guarded like the
// emitter's other frames.
func (e *studioEmitter) Phase(to string) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.sse.Event("phase", map[string]string{"to": to})
}
```

Match the existing methods' exact locking + `e.sse` call shape — copy `Text`'s body and change the event name/payload.

- [ ] **Step 2: Write the failing API tests**

Create `apps/api/internal/api/course_session_test.go`, following `chat_test.go`'s harness (testcontainer-backed API + a seeded student). Cover:

```go
// 1. session get-or-create is idempotent
func TestCourseSession_CreateIsIdempotent(t *testing.T)
// POST twice → same id, 200/201; GET returns phase "demonstrate".

// 2. ownership: another user's session is 404, never 403
func TestCourseSession_OwnershipIs404(t *testing.T)
// All six routes with a session belonging to a different student → 404.

// 3. ask streams a reply and records exactly one llm_call
func TestCourseSession_Ask(t *testing.T)
// POST .../session/ask with a stub provider returning a reply →
// SSE has event:text then event:done; SELECT count(*) FROM llm_call
// WHERE surface='course' AND purpose='coach' AND project_id IS NULL = 1.

// 4. advance with an unmet floor spends nothing
func TestCourseSession_AdvanceUnmetFloorSpendsNothing(t *testing.T)
// No progress rows → POST .../session/advance → SSE carries a text frame,
// NO phase frame; SELECT count(*) FROM llm_call WHERE surface='course' = 0;
// the session's phase is unchanged in the DB.

// 5. advance with a met floor emits the phase frame and moves the row
func TestCourseSession_AdvanceMovesThePhase(t *testing.T)
// Seed course_progress completed_ordinals={0,1} → stub returns
// {"type":"advance","to":"guided"} → SSE has event:phase {"to":"guided"};
// course_session.phase = 'guided' in the DB.

// 6. thin card submit writes no graph
func TestCourseSession_CardSubmitIsThin(t *testing.T)
// Submit a session card → status completed; assert zero graph_node and zero
// graph_edge rows exist for this user, and no intervention row was written.
```

Write these out fully in the style of the existing `chat_test.go` cases — same request helpers, same SSE-parsing helper, same assertion style. Use `count(*)` for the llm_call assertions, not "most recent row".

- [ ] **Step 3: Run and watch them fail**

```bash
cd apps/api && CGO_ENABLED=0 go test -p 1 ./internal/api/ -run TestCourseSession
```

Expected: FAIL — the routes 404 (they don't exist yet).

- [ ] **Step 4: Implement the DTOs**

Create `apps/api/internal/api/course_session_dto.go`:

```go
package api

// CourseSessionDTO is the Course surface's runtime state: which phase the
// student is in, and this session's dialogue. Page position stays in
// course_progress and ships through the existing progress endpoints (DEC-12.1:
// pages are content, phases are runtime).
type CourseSessionDTO struct {
	ID         string             `json:"id"`
	CourseID   string             `json:"courseId"`
	Phase      string             `json:"phase"`
	PhaseTitle string             `json:"phaseTitle"`
	Status     string             `json:"status"`
	Messages   []CourseMessageDTO `json:"messages"`
}

type CourseMessageDTO struct {
	ID        string `json:"id"`
	Phase     string `json:"phase"`
	Role      string `json:"role"`
	Content   string `json:"content"`
	CreatedAt string `json:"createdAt"`
}
```

…plus `toCourseSessionDTO` / `toCourseMessageDTO` converters mirroring `chat_dto.go`'s (same timestamp formatting).

- [ ] **Step 5: Implement the handlers**

Create `apps/api/internal/api/course_session.go` with `loadOwnedSession` (resolve `{id}` = course id → the caller's session, 404 not 403), then the six handlers per spec §5. The two SSE handlers share one body:

```go
// runCourseTurn is the shared body of ask + advance: entitlement gate BEFORE
// committing to the stream, then RunCourseStep, then the frames. Mirrors
// postChatTurn's scaffolding (studioEmitter + startHeartbeat).
func (a *API) runCourseTurn(w http.ResponseWriter, r *http.Request, intent agent.CourseIntent) {
	// ... resolve session, entitlement gate, decode (ask only), resolver,
	// SSE commit, heartbeat, deps, RunCourseStep, then:
	if res.Reply != "" {
		_ = em.Text(res.Reply)
	}
	if res.Offer != nil {
		_ = em.Card(res.Offer.CardInstanceID.String(), res.Offer.CardID, "", []byte("[]"), res.Offer.MaterialID.String())
	}
	if res.Advanced != "" {
		_ = em.Phase(res.Advanced)
	}
	_ = em.Done()
}
```

Guard `em.Text` on a non-empty reply (an empty frame renders an empty bubble — a Slice-11 whole-branch finding; do not reintroduce it). The `ask` handler rejects an empty `user_input` with `httpx.ErrBadRequest`; `advance` decodes no body at all.

Load the skill once per request via the `skills` loader with `session.SkillID`.

Register in `apps/api/internal/api/api.go`, after the existing course routes:

```go
	mux.Handle("POST /api/v1/courses/{id}/session", protected(a.startCourseSession))
	mux.Handle("GET /api/v1/courses/{id}/session", protected(a.getCourseSession))
	mux.Handle("POST /api/v1/courses/{id}/session/ask", protected(a.postCourseAsk))
	mux.Handle("POST /api/v1/courses/{id}/session/advance", protected(a.postCourseAdvance))
	mux.Handle("POST /api/v1/courses/{id}/session/cards/{cid}/submit", protected(a.submitCourseCard))
	mux.Handle("POST /api/v1/courses/{id}/session/cards/{cid}/skip", protected(a.skipCourseCard))
```

`submitCourseCard`/`skipCourseCard` mirror `submitChatCard`/`skipChatCard` verbatim, substituting the session queries and `Surface: "course"` on the event — thin: no `CompleteCard`, no graph_effects, no refeed, no competence.

- [ ] **Step 6: Run the FULL api package**

```bash
cd apps/api && CGO_ENABLED=0 go test -p 1 ./internal/api/
```

Expected: `ok`.

- [ ] **Step 7: Add the Zod + web client + test**

Append to `packages/contracts/src/course.ts`:

```ts
export const CourseMessage = z.object({
  id: z.string(),
  phase: z.string(),
  role: z.enum(["student", "assistant"]),
  content: z.string(),
  createdAt: z.string(),
});

export const CourseSession = z.object({
  id: z.string(),
  courseId: z.string(),
  phase: z.string(),
  phaseTitle: z.string(),
  status: z.enum(["active", "finished"]),
  messages: z.array(CourseMessage),
});

export type CourseMessage = z.infer<typeof CourseMessage>;
export type CourseSession = z.infer<typeof CourseSession>;
```

Create `apps/web/src/api/courseSession.ts` mirroring `apps/web/src/api/chat.ts` exactly: the async generators accumulate `text` deltas into one `reply` event, map `card` → `{type:"card", cardInstanceId, cardId, materialId}` (the wire is snake_case: `card_instance_id`/`card_id`/`material_id`), map the new `phase` frame → `{type:"phase", to}`, and pass `done`/`error` through. Export from the api barrel the way `chat.ts` is exported.

Create `apps/web/src/api/courseSession.test.ts` mirroring `chat.test.ts`: assert `courseAsk` accumulates deltas into one reply; assert `courseAdvance` surfaces a `phase` event; assert a non-ok response yields an `error` event.

- [ ] **Step 8: Run contracts + web**

```bash
cd packages/contracts && npx vitest run
cd ../../apps/web && npx vitest run && npx tsc --noEmit
```

Expected: all green, tsc exit 0. Run each from its own directory — a compound `cd` can leave you in the wrong one and silently re-run the other suite.

- [ ] **Step 9: Commit**

```bash
git add apps/api/internal/api/course_session.go apps/api/internal/api/course_session_dto.go apps/api/internal/api/course_session_test.go apps/api/internal/api/api.go apps/api/internal/api/studioturn.go packages/contracts/src/course.ts apps/web/src/api/courseSession.ts apps/web/src/api/courseSession.test.ts apps/web/src/api/index.ts
git commit -m "feat(refactor2): Slice 12 T7 — course session endpoints + phase SSE frame + web client"
```

---

### Task 8: The 问印记 ask panel + the player on the runtime

**Files:**
- Create: `apps/web/src/shell/courses/AskPanel.tsx`, `apps/web/src/shell/courses/AskPanel.test.tsx`
- Modify: `apps/web/src/shell/courses/CoursePlayer.tsx`
- Test: `apps/web/src/shell/courses/CoursePlayer.test.tsx`

**Interfaces:**
- Consumes: Task 7's `startCourseSession`/`getCourseSession`/`courseAsk`/`courseAdvance`/`submitCourseCard`/`skipCourseCard`/`CourseTurnEvent`/`CourseSession`; the existing `StudioCardSheet` and `CARD_REGISTRY`.

**Binding design:** `docs/design/思维印记_工作区.dc.html` lines 349–410 (the ask panel) and 215–232 (the header's `pgStage`). Read those lines before writing a pixel. Copy is verbatim; the design wins over this plan.

- [ ] **Step 1: Write the failing AskPanel test**

Create `apps/web/src/shell/courses/AskPanel.test.tsx`:

```tsx
const props = {
  expanded: true,
  onToggle: vi.fn(),
  branchColor: "#2A3B7A",
  context: "引导",
  chips: ["这张卡要我做什么？", "我该先查哪一步？"],
  messages: [],
  pending: false,
  onSend: vi.fn(),
};

describe("AskPanel", () => {
  it("renders the binding disclosure copy verbatim", () => {
    render(<AskPanel {...props} />);
    expect(screen.getByText("问印记")).toBeInTheDocument();
    expect(screen.getByText("随时打断我，问任何问题")).toBeInTheDocument();
    expect(screen.getByText("正在看：引导")).toBeInTheDocument();
    expect(screen.getByText("你可能想问")).toBeInTheDocument();
    expect(screen.getByPlaceholderText("输入你的问题……")).toBeInTheDocument();
    expect(screen.getByText("按住说话，问老师")).toBeInTheDocument();
  });

  it("sends a chip as a message", async () => {
    const onSend = vi.fn();
    render(<AskPanel {...props} onSend={onSend} />);
    await userEvent.click(screen.getByText("这张卡要我做什么？"));
    expect(onSend).toHaveBeenCalledWith("这张卡要我做什么？");
  });

  it("sends the composer's text", async () => {
    const onSend = vi.fn();
    render(<AskPanel {...props} onSend={onSend} />);
    await userEvent.type(screen.getByPlaceholderText("输入你的问题……"), "这算证据吗？");
    await userEvent.click(screen.getByLabelText("发送"));
    expect(onSend).toHaveBeenCalledWith("这算证据吗？");
  });

  it("leaves the voice button inert — voice is deferred", async () => {
    render(<AskPanel {...props} />);
    await userEvent.click(screen.getByText("按住说话，问老师"));
    expect(props.onSend).not.toHaveBeenCalled();
  });

  it("collapses to the vertical rail", () => {
    render(<AskPanel {...props} expanded={false} />);
    expect(screen.queryByPlaceholderText("输入你的问题……")).not.toBeInTheDocument();
    expect(screen.getByText("问印记")).toBeInTheDocument();
  });
});
```

Match the existing web tests' imports/harness (read a neighbouring `*.test.tsx`).

- [ ] **Step 2: Run and watch it fail**

```bash
cd apps/web && npx vitest run src/shell/courses/AskPanel.test.tsx
```

Expected: FAIL — `AskPanel` does not exist.

- [ ] **Step 3: Build the panel**

Create `apps/web/src/shell/courses/AskPanel.tsx` transcribing dc.html 349–410: the 330px expanded column (`borderLeft: "1px solid #EAECF2"`), the Bean + 问印记 + 随时打断我，问任何问题 header with the collapse chevron, the `正在看：{context}` pill (`#EDEFF9` bg, `#2A3B7A` text, the pulsing `#4C9A82` dot), the 你可能想问 chip list, the message list, the composer (placeholder `输入你的问题……`, the send button — give it `aria-label="发送"`), and the 按住说话，问老师 button rendered with **no onClick** (voice is deferred; it renders and does nothing). Collapsed = the 46px rail with the vertical 问印记 label. Icons inline SVG.

- [ ] **Step 4: Run and watch it pass**

```bash
cd apps/web && npx vitest run src/shell/courses/AskPanel.test.tsx
```

Expected: PASS.

- [ ] **Step 5: Write the failing player tests**

Append to `apps/web/src/shell/courses/CoursePlayer.test.tsx` (mock the `courseSession` client module):

```tsx
it("renders the phase title as the stage label", async () => {
  // session phase = demonstrate → the header and the page show 演示.
});

it("pages inside a phase without touching the network", async () => {
  // Click next from ordinal 0 to 1 (both in the demonstrate phase) →
  // courseAdvance is NOT called; the page moves.
});

it("asks the coach when next would cross a phase boundary", async () => {
  // At the phase's last step, next → courseAdvance called exactly once.
});

it("auto-expands the panel when the coach refuses to advance", async () => {
  // courseAdvance yields {type:"reply"} and no phase event → the panel is
  // expanded and shows the reply; the page has not moved.
});

it("moves the phase when the coach advances", async () => {
  // courseAdvance yields {type:"phase", to:"guided"} → the stage label
  // becomes 引导.
});

it("requires 接受 before the card sheet mounts", async () => {
  // courseAsk yields {type:"card", ...} → the offer renders; the sheet is
  // absent until 接受 is clicked.
});
```

Write each of these out fully against the real component API, following the existing tests in this file for the harness and the `api` mock shape.

- [ ] **Step 6: Wire the player**

Modify `apps/web/src/shell/courses/CoursePlayer.tsx`:

- On mount, `startCourseSession(courseId)`; hold `{phase, phaseTitle, messages}`.
- Render `pgStage` = `phaseTitle` in the header (beside `courseProgressLabel`) and above the page title, per dc.html 226 + 245.
- Mount `<AskPanel>` as the right column; `onSend` → `courseAsk`, accumulating the reply into the message list.
- The next-arrow: if the next ordinal is inside the current phase, page locally exactly as today (`go(next)`); if it would cross the boundary (the ordinal is not in this phase's `steps`, or this phase has no next step), call `courseAdvance` instead. On a `phase` event, set the phase and page to that phase's first step; on a `reply` with no `phase` event, expand the panel and show the reply — the page does not move.
- Step-less phases (`guided`, `reflect`) render their skill-authored `page` block instead of a `course_step` render. Get the phase's `page`/`ask_chips` by importing the skill JSON from `@mind-imprint/contracts` (`packages/contracts/skills/info-literacy-course.json`) — it is the single source of truth the backend also reads.
- The card offer renders in the ask panel with an explicit 接受 button; only on 接受 does `<StudioCardSheet spec={CARD_REGISTRY[cardId]} onSubmit={…submitCourseCard} onSkip={…skipCourseCard} />` mount. Confirm-to-open, never auto-open.
- Backward paging is never gated.

- [ ] **Step 7: Run the FULL web suite**

```bash
cd apps/web && npx vitest run && npx tsc --noEmit
```

Expected: all files green, tsc exit 0. Run from `apps/web` — not via a compound `cd`.

- [ ] **Step 8: Commit**

```bash
git add apps/web/src/shell/courses/AskPanel.tsx apps/web/src/shell/courses/AskPanel.test.tsx apps/web/src/shell/courses/CoursePlayer.tsx apps/web/src/shell/courses/CoursePlayer.test.tsx
git commit -m "feat(refactor2): Slice 12 T8 — 问印记 ask panel + player on the course runtime"
```

---

## Final gate (whole branch, before review)

```bash
cd apps/api && CGO_ENABLED=0 go test -p 1 ./...
cd ../../packages/contracts && npx vitest run
cd ../../apps/web && npx vitest run && npx tsc --noEmit
cd ../../apps/api && make sqlc && make sync-skills && git status --short
```

All Go packages `ok`; contracts and web suites green; tsc exit 0; codegen produces **no drift** (`git status --short` shows only the pre-existing user files listed in Global Constraints).
