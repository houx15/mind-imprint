# P1 · Agentic Orchestrator Framework + Status Spine — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the studio's five scattered per-room coach decision producers with one tool-calling agent loop; give each project an AI-managed studio **stage**; resume a project at its stage and let 印记 drive room switches via a workspace **directive**.

**Architecture:** One agent pass per student turn. The handler loads the existing spine projection + a new `studio_state`, makes **one** LLM call that returns `{ narrate, tools[] }` JSON, applies the workspace tools (mutating `studio_state`, passing note/card proposals through), persists, and returns a directive the frontend applies. New `studio_state jsonb` column on `project`. The agent auto-configures the workspace (set stage, open the room, curate reference) and narrates — always overridable by the student. Because the tool calls are embedded in the same JSON as the narration, the turn is one-shot: a plain JSON endpoint (evolving `POST /coach`), not SSE.

**Tech Stack:** Go (`net/http`, `pgx`, `sqlc` @v1.27.0, `goose`), PostgreSQL; React + Vite + TypeScript + Tailwind; Zod contracts (`packages/contracts`); Vitest + jsdom; Go `testing` + testcontainers.

## Global Constraints

- **Design system (2026-08-06):** Tailwind `mk-*` tokens; exactly ONE Tailwind class per competing CSS property (alphabetical emit order); **never** `bg-mk-<hex-token>/<opacity>` (renders transparent — use solid tokens or `bg-black/NN`). Lucide icons; AI four-state / composer / chip conventions.
- **No old-data compatibility:** the product is not in real use. Free to make breaking schema changes — no back-compat migration, no legacy fallback, no preserving old fields. Target the cleanest model.
- **四条铁律:** ① AI never writes body text (proposes, never decides — `propose_note`/`summon_card` are proposals the student confirms). ② No manipulation — auto-configuring the workspace is always overridable; no forced flow. ③ One question per turn (a turn fires many *workspace* tools but exactly one narration/question). ④ Process is data — every turn is recorded.
- **Keys server-side only.** Client never calls the model directly; every LLM call meters `llm_call` (surface + purpose + tokens + cost).
- **Card JSON is the single source of truth** — no second copy of card metadata.
- **sqlc regen command (exact):** `cd apps/api && CGO_ENABLED=0 go run github.com/sqlc-dev/sqlc/cmd/sqlc@v1.27.0 generate` (schema is read from the migrations dir; pin the version or stale structs regenerate).
- **Contracts resolve via source:** `@mind-imprint/contracts` `main` is `src/index.ts` — new files must be re-exported from `packages/contracts/src/index.ts` or the web app cannot import them.

## Canonical Vocabularies (used verbatim across tasks)

**Studio stages** (`StudioStage`, code → 中文 label; tunable, no rigid forward-only rule):

| code | label |
|---|---|
| `topic_discussion` | 立题讨论 |
| `proposal_forming` | 提案要点成形 |
| `plan_generation` | 生成计划 |
| `proposal_writing` | 写提案 |
| `proposal_review` | 提案体检 |
| `body_writing` | 写正文 |
| `retrospective` | 复盘 |

**Open-tool targets** (`OpenTool`) — what room the shell shows; reuses the existing `BlockKey` plus a `chat` sentinel (chat-only, no room board):
`chat` | `plan` | `reading` | `writing` | `reflection`

**Width tiers** (`WidthTier`) — stored in P1, *rendered* in P2: `chat` | `half` | `wide`. Derived server-side from `open_tool` (not a tool arg): `chat→chat`, `plan→half`, `reading|writing|reflection→wide`.

**Proposal sections** (`ProposalSection`) — identical to the existing `FormingDimSuggestion.Dim` values:
`objective` | `reason` | `activities` | `resources` | `counterpoints`

**The agent tool set (P1):**

| tool | args | effect |
|---|---|---|
| `set_status` | `{ stage }` | writes `studio_state.stage` |
| `open_tool` | `{ tool, reason }` | writes `studio_state.openTool` + derived `widthTier` |
| `curate_reference` | `{ items: ReferenceRef[] }` | writes `studio_state.reference` (P3 renders it) |
| `propose_note` | `{ section, value }` | passed through to the response as a note-confirm proposal (NO server write — student confirms) |
| `summon_card` | `{ card_id, reason, nudge_text }` | reuses the existing `CardProposal` path — returned in the response + records a `coach_proposed` event |
| `request_review` | `{}` | sets `open_tool=writing` + `reviewRequested=true` flag in the response |

---

# Backend

### Task 1: `studio_state` column, sqlc queries, Go type

**Files:**
- Create: `apps/api/internal/store/migrations/0057_studio_state.sql`
- Modify: `apps/api/internal/store/queries/project.sql` (append two queries)
- Create: `apps/api/internal/agent/studiostate.go`
- Test: `apps/api/internal/agent/studiostate_test.go`

**Interfaces:**
- Produces: `agent.StudioStage` (string type + const set + `IsValid()`), `agent.OpenTool`, `agent.WidthTier`, `agent.ReferenceRef struct`, `agent.StudioState struct`, `agent.DefaultStudioState() StudioState`, `agent.WidthForTool(OpenTool) WidthTier`. sqlc `GetStudioState`, `SetStudioState`.

- [ ] **Step 1: Write the migration**

Create `apps/api/internal/store/migrations/0057_studio_state.sql`. Follow the existing goose format (see `0056_proposal_counterpoints.sql` for the `-- +goose Up` / `-- +goose Down` shape). The default JSON is the topic-discussion, chat-only starting state:

```sql
-- +goose Up
ALTER TABLE project
  ADD COLUMN studio_state jsonb NOT NULL
  DEFAULT '{"stage":"topic_discussion","openTool":"chat","widthTier":"chat","reference":[],"updatedAtTurn":0}'::jsonb;

-- +goose Down
ALTER TABLE project DROP COLUMN studio_state;
```

- [ ] **Step 2: Append sqlc queries to `project.sql`**

Append to `apps/api/internal/store/queries/project.sql`:

```sql
-- name: GetStudioState :one
SELECT studio_state FROM project WHERE id = $1;

-- name: SetStudioState :exec
UPDATE project SET studio_state = $2, last_active_at = now() WHERE id = $1;
```

- [ ] **Step 3: Regenerate sqlc**

Run: `cd apps/api && CGO_ENABLED=0 go run github.com/sqlc-dev/sqlc/cmd/sqlc@v1.27.0 generate`
Expected: new `GetStudioState`/`SetStudioState` methods on the generated `Queries`; `studio_state` appears on the generated `Project` struct as `[]byte` / `json.RawMessage`. If the whole `Project` struct or unrelated structs change, that is the known "stale sqlc" symptom — commit the full regen (do not hand-edit generated files).

- [ ] **Step 4: Write the failing test for the Go type**

Create `apps/api/internal/agent/studiostate_test.go`:

```go
package agent

import (
	"encoding/json"
	"testing"
)

func TestDefaultStudioStateRoundTrips(t *testing.T) {
	def := DefaultStudioState()
	if def.Stage != StageTopicDiscussion || def.OpenTool != ToolChat || def.WidthTier != WidthChat {
		t.Fatalf("unexpected default: %+v", def)
	}
	b, err := json.Marshal(def)
	if err != nil {
		t.Fatal(err)
	}
	var got StudioState
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatal(err)
	}
	if got.Stage != def.Stage || got.OpenTool != def.OpenTool || got.Reference == nil {
		t.Fatalf("round-trip mismatch: %+v", got)
	}
}

func TestWidthForTool(t *testing.T) {
	cases := map[OpenTool]WidthTier{
		ToolChat: WidthChat, ToolPlan: WidthHalf,
		ToolReading: WidthWide, ToolWriting: WidthWide, ToolReflection: WidthWide,
	}
	for tool, want := range cases {
		if got := WidthForTool(tool); got != want {
			t.Errorf("WidthForTool(%q)=%q want %q", tool, got, want)
		}
	}
}

func TestStageIsValid(t *testing.T) {
	if !StageBodyWriting.IsValid() || StudioStage("nope").IsValid() {
		t.Fatal("IsValid wrong")
	}
}
```

- [ ] **Step 5: Run test to verify it fails**

Run: `cd apps/api && go test ./internal/agent/ -run 'TestDefaultStudioState|TestWidthForTool|TestStageIsValid'`
Expected: FAIL — undefined `StudioState`, `DefaultStudioState`, etc.

- [ ] **Step 6: Implement `studiostate.go`**

Create `apps/api/internal/agent/studiostate.go`:

```go
package agent

// StudioStage is the AI-managed lifecycle position of a project. Tunable — the
// orchestrator may move a project forward or back; there is no rigid ordering
// constraint here (the plan-generation gate lives in the plan/generate handler,
// not in this enum).
type StudioStage string

const (
	StageTopicDiscussion StudioStage = "topic_discussion"
	StageProposalForming StudioStage = "proposal_forming"
	StagePlanGeneration  StudioStage = "plan_generation"
	StageProposalWriting StudioStage = "proposal_writing"
	StageProposalReview  StudioStage = "proposal_review"
	StageBodyWriting     StudioStage = "body_writing"
	StageRetrospective   StudioStage = "retrospective"
)

func (s StudioStage) IsValid() bool {
	switch s {
	case StageTopicDiscussion, StageProposalForming, StagePlanGeneration,
		StageProposalWriting, StageProposalReview, StageBodyWriting, StageRetrospective:
		return true
	}
	return false
}

// OpenTool is which room the shell shows. `chat` = no room (chat-only landing).
type OpenTool string

const (
	ToolChat       OpenTool = "chat"
	ToolPlan       OpenTool = "plan"
	ToolReading    OpenTool = "reading"
	ToolWriting    OpenTool = "writing"
	ToolReflection OpenTool = "reflection"
)

func (t OpenTool) IsValid() bool {
	switch t {
	case ToolChat, ToolPlan, ToolReading, ToolWriting, ToolReflection:
		return true
	}
	return false
}

// WidthTier is stored in P1 and rendered in P2. Derived from OpenTool, never a
// tool argument.
type WidthTier string

const (
	WidthChat WidthTier = "chat"
	WidthHalf WidthTier = "half"
	WidthWide WidthTier = "wide"
)

func WidthForTool(t OpenTool) WidthTier {
	switch t {
	case ToolChat:
		return WidthChat
	case ToolPlan:
		return WidthHalf
	default:
		return WidthWide
	}
}

// ReferenceRef is one item 印记 curated into the left reference panel. Lean by
// design in P1 — P3 renders it richly. kind ∈ material|note|annotation.
type ReferenceRef struct {
	Kind  string `json:"kind"`
	ID    string `json:"id"`
	Label string `json:"label"`
}

// StudioState is the AI-managed workspace state persisted on the project.
type StudioState struct {
	Stage         StudioStage    `json:"stage"`
	OpenTool      OpenTool       `json:"openTool"`
	WidthTier     WidthTier      `json:"widthTier"`
	Reference     []ReferenceRef `json:"reference"`
	UpdatedAtTurn int            `json:"updatedAtTurn"`
}

func DefaultStudioState() StudioState {
	return StudioState{
		Stage:         StageTopicDiscussion,
		OpenTool:      ToolChat,
		WidthTier:     WidthChat,
		Reference:     []ReferenceRef{},
		UpdatedAtTurn: 0,
	}
}
```

- [ ] **Step 7: Run test to verify it passes**

Run: `cd apps/api && go test ./internal/agent/ -run 'TestDefaultStudioState|TestWidthForTool|TestStageIsValid'`
Expected: PASS.

- [ ] **Step 8: Commit**

```bash
git add apps/api/internal/store/migrations/0057_studio_state.sql apps/api/internal/store/queries/project.sql apps/api/internal/store/gen apps/api/internal/agent/studiostate.go apps/api/internal/agent/studiostate_test.go
git commit -m "feat(studio): studio_state column + Go StudioState type + sqlc queries"
```

---

### Task 2: Orchestrator contract (Zod, shared)

**Files:**
- Create: `packages/contracts/src/orchestrator.ts`
- Modify: `packages/contracts/src/index.ts` (add `export * from "./orchestrator"`)
- Test: `packages/contracts/src/orchestrator.test.ts`

**Interfaces:**
- Produces: `StudioStage`, `OpenTool`, `WidthTier`, `ProposalSection`, `ReferenceRef`, `StudioState`, `OrchestratorTool` (discriminated union), `NoteProposal`, `CardProposalWire`, `OrchestratorReply`. These are the single wire truth the Go handler (Task 5) mirrors field-for-field (same JSON names, camelCase).

- [ ] **Step 1: Write the failing test**

Create `packages/contracts/src/orchestrator.test.ts`:

```ts
import { describe, it, expect } from "vitest";
import { StudioState, OrchestratorReply } from "./orchestrator";

describe("orchestrator contract", () => {
  it("accepts a full directive reply", () => {
    const reply = {
      narrate: "提案四点齐了 — 写作面板给你开好了。",
      directive: {
        stage: "body_writing",
        openTool: "writing",
        widthTier: "wide",
        reference: [{ kind: "material", id: "m1", label: "NASA 报告" }],
        updatedAtTurn: 3,
      },
      note: null,
      card: null,
      reviewRequested: false,
    };
    expect(() => OrchestratorReply.parse(reply)).not.toThrow();
  });

  it("rejects an unknown stage", () => {
    const bad = { stage: "nope", openTool: "chat", widthTier: "chat", reference: [], updatedAtTurn: 0 };
    expect(() => StudioState.parse(bad)).toThrow();
  });

  it("accepts a note-proposal reply with null directive fields defaulted", () => {
    const reply = {
      narrate: "要不要把这条记进「目标」？",
      directive: { stage: "proposal_forming", openTool: "plan", widthTier: "half", reference: [], updatedAtTurn: 1 },
      note: { section: "objective", value: "探究中国可持续发展对全球的净影响" },
      card: null,
      reviewRequested: false,
    };
    expect(() => OrchestratorReply.parse(reply)).not.toThrow();
  });
});
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd packages/contracts && npx vitest run src/orchestrator.test.ts`
Expected: FAIL — cannot resolve `./orchestrator`.

- [ ] **Step 3: Implement the contract**

Create `packages/contracts/src/orchestrator.ts`:

```ts
import { z } from "zod";

export const StudioStage = z.enum([
  "topic_discussion",
  "proposal_forming",
  "plan_generation",
  "proposal_writing",
  "proposal_review",
  "body_writing",
  "retrospective",
]);
export type StudioStage = z.infer<typeof StudioStage>;

export const OpenTool = z.enum(["chat", "plan", "reading", "writing", "reflection"]);
export type OpenTool = z.infer<typeof OpenTool>;

export const WidthTier = z.enum(["chat", "half", "wide"]);
export type WidthTier = z.infer<typeof WidthTier>;

export const ProposalSection = z.enum(["objective", "reason", "activities", "resources", "counterpoints"]);
export type ProposalSection = z.infer<typeof ProposalSection>;

export const ReferenceRef = z.object({
  kind: z.enum(["material", "note", "annotation"]),
  id: z.string(),
  label: z.string(),
});
export type ReferenceRef = z.infer<typeof ReferenceRef>;

export const StudioState = z.object({
  stage: StudioStage,
  openTool: OpenTool,
  widthTier: WidthTier,
  reference: z.array(ReferenceRef),
  updatedAtTurn: z.number().int(),
});
export type StudioState = z.infer<typeof StudioState>;

// The orchestrator's tool vocabulary — mirrors summonCard.ts's {name,args} shape.
// This is what the MODEL emits (parsed server-side); the client never sends it.
export const OrchestratorTool = z.discriminatedUnion("name", [
  z.object({ name: z.literal("set_status"), args: z.object({ stage: StudioStage }) }),
  z.object({ name: z.literal("open_tool"), args: z.object({ tool: OpenTool, reason: z.string() }) }),
  z.object({ name: z.literal("curate_reference"), args: z.object({ items: z.array(ReferenceRef) }) }),
  z.object({ name: z.literal("propose_note"), args: z.object({ section: ProposalSection, value: z.string() }) }),
  z.object({ name: z.literal("summon_card"), args: z.object({ card_id: z.string(), reason: z.string(), nudge_text: z.string() }) }),
  z.object({ name: z.literal("request_review"), args: z.object({}) }),
]);
export type OrchestratorTool = z.infer<typeof OrchestratorTool>;

// A note the student confirms before it lands (铁律①). Producer: propose_note.
export const NoteProposal = z.object({ section: ProposalSection, value: z.string() });
export type NoteProposal = z.infer<typeof NoteProposal>;

// The card chip surfaced this turn. Producer: summon_card. Mirrors the existing
// coachProposalDTO (cardId/reason/nudgeText).
export const CardProposalWire = z.object({
  cardId: z.string(),
  reason: z.string(),
  nudgeText: z.string(),
});
export type CardProposalWire = z.infer<typeof CardProposalWire>;

// The turn response the frontend applies.
export const OrchestratorReply = z.object({
  narrate: z.string(),
  directive: StudioState,
  note: NoteProposal.nullable(),
  card: CardProposalWire.nullable(),
  reviewRequested: z.boolean(),
});
export type OrchestratorReply = z.infer<typeof OrchestratorReply>;
```

- [ ] **Step 4: Re-export from the barrel**

Add to `packages/contracts/src/index.ts` (alongside the other `export * from "./…"` lines):

```ts
export * from "./orchestrator";
```

- [ ] **Step 5: Run test to verify it passes**

Run: `cd packages/contracts && npx vitest run src/orchestrator.test.ts`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add packages/contracts/src/orchestrator.ts packages/contracts/src/orchestrator.test.ts packages/contracts/src/index.ts
git commit -m "feat(contracts): orchestrator tool set + StudioState + OrchestratorReply"
```

---

### Task 3: Orchestrator output parser (prompt + tool parse)

**Files:**
- Create: `apps/api/internal/agent/orchestrator.go`
- Test: `apps/api/internal/agent/orchestrator_test.go`

**Interfaces:**
- Consumes: `agent.StudioState`, `agent.StudioStage`/`OpenTool` (Task 1); `stripFences` (existing helper — grep `func stripFences` / `stripJSONFence` in `internal/agent/`; reuse the existing one, do not add a second).
- Produces: `agent.OrchestratorDecision struct { Narrate string; Tools []OrchestratorToolCall }`; `agent.OrchestratorToolCall struct { Name string; Args json.RawMessage }`; `agent.ParseOrchestratorOutput(text string) (OrchestratorDecision, error)`; typed arg accessors `SetStatusArgs`/`OpenToolArgs`/`CurateReferenceArgs`/`ProposeNoteArgs`/`SummonCardArgs`; `orchestratorSystemPrompt` const.

**Design note:** the model returns a single JSON object `{"narrate": "...", "tools": [{"name":"set_status","args":{...}}, ...]}`. Unknown tool names and args-that-fail-validation are **dropped** (logged), never fatal — a chatty/partial turn degrades to narration-only. Only a JSON that will not unmarshal at all is an error (the caller retries once, then falls back).

- [ ] **Step 1: Write the failing test**

Create `apps/api/internal/agent/orchestrator_test.go`:

```go
package agent

import "testing"

func TestParseOrchestratorOutput_FencedMultiTool(t *testing.T) {
	raw := "```json\n{\"narrate\":\"写作面板开好了。\",\"tools\":[" +
		"{\"name\":\"set_status\",\"args\":{\"stage\":\"body_writing\"}}," +
		"{\"name\":\"open_tool\",\"args\":{\"tool\":\"writing\",\"reason\":\"该写正文了\"}}," +
		"{\"name\":\"propose_note\",\"args\":{\"section\":\"objective\",\"value\":\"净影响\"}}" +
		"]}\n```"
	dec, err := ParseOrchestratorOutput(raw)
	if err != nil {
		t.Fatal(err)
	}
	if dec.Narrate != "写作面板开好了。" {
		t.Fatalf("narrate=%q", dec.Narrate)
	}
	if len(dec.Tools) != 3 {
		t.Fatalf("want 3 tools, got %d", len(dec.Tools))
	}
}

func TestParseOrchestratorOutput_DropsUnknownAndInvalid(t *testing.T) {
	raw := `{"narrate":"ok","tools":[` +
		`{"name":"teleport","args":{}},` + // unknown → dropped
		`{"name":"set_status","args":{"stage":"not_a_stage"}},` + // invalid arg → dropped
		`{"name":"set_status","args":{"stage":"proposal_forming"}}` + // valid → kept
		`]}`
	dec, err := ParseOrchestratorOutput(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(dec.Tools) != 1 || dec.Tools[0].Name != "set_status" {
		t.Fatalf("expected only the valid set_status, got %+v", dec.Tools)
	}
}

func TestParseOrchestratorOutput_MalformedIsError(t *testing.T) {
	if _, err := ParseOrchestratorOutput("not json at all"); err == nil {
		t.Fatal("expected error on unparseable output")
	}
}

func TestOpenToolArgs(t *testing.T) {
	dec, _ := ParseOrchestratorOutput(`{"narrate":"","tools":[{"name":"open_tool","args":{"tool":"reading","reason":"去读那篇"}}]}`)
	args, err := OpenToolArgs(dec.Tools[0])
	if err != nil || args.Tool != ToolReading || args.Reason != "去读那篇" {
		t.Fatalf("OpenToolArgs wrong: %+v err=%v", args, err)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd apps/api && go test ./internal/agent/ -run TestParseOrchestratorOutput`
Expected: FAIL — undefined `ParseOrchestratorOutput`.

- [ ] **Step 3: Implement `orchestrator.go`**

Create `apps/api/internal/agent/orchestrator.go`. Reuse the existing fence stripper (confirm its name via grep first; the reading router uses `stripFences`). Validate each tool's args by unmarshaling into the typed struct and checking enums; drop on any failure.

```go
package agent

import (
	"encoding/json"
	"errors"
)

// orchestratorSystemPrompt is the ONE posture for 印记-as-orchestrator. It
// replaces the five scattered decision producers. 印记 runs the project like a
// Cowork agent: it reads where the project is, decides the next step, CONFIGURES
// the workspace via tools (auto — always overridable), and narrates ONE next
// question. It never writes the student's body text; notes and cards are
// proposals the student confirms.
const orchestratorSystemPrompt = `你是「印记」，一个带着学生把研究项目做完的 agent（类似 Cowork 之于写代码）。你不替学生做：不替他定论、绝不代写正文。你在对的时刻把当下这一步的工作台配置好，然后叙述你配了什么、并只问一个下一步的问题。

你每一轮只输出一个 JSON 对象，形如：
{"narrate": "给学生看的一段话，一次只问一个问题", "tools": [ ...你这一轮要执行的工作台动作... ]}

可用工具（tools 数组里的每一项是 {"name":..., "args":{...}}）：
- set_status: {"stage": 阶段码} —— 推进/回退项目阶段。阶段码 ∈ topic_discussion(立题讨论)/proposal_forming(提案要点成形)/plan_generation(生成计划)/proposal_writing(写提案)/proposal_review(提案体检)/body_writing(写正文)/retrospective(复盘)。
- open_tool: {"tool": 房间, "reason": 理由} —— 为这一步打开对的房间。房间 ∈ chat(只聊,无面板)/plan(立项与提案要点)/reading(阅读室)/writing(写作台)/reflection(复盘)。
- curate_reference: {"items": [{"kind":"material|note|annotation","id":...,"label":...}]} —— 把学生此刻会去查的材料摆到左侧。
- propose_note: {"section": 分区, "value": 内容} —— 从学生说过的话里提炼一条提案要点候选（学生确认后才落库）。分区 ∈ objective(研究问题/目标)/reason(动机与意义)/activities(活动计划)/resources(资源与文献)/counterpoints(可能的反例/张力)。
- summon_card: {"card_id":..., "reason":..., "nudge_text":...} —— 在对的时刻把一张思维工具卡塞回给学生。
- request_review: {} —— 学生写完、该做整稿体检时。

原则：一次只问一个问题（narrate 里不要连问）；只有当四项必填提案要点(objective/reason/activities/resources)都有内容后，才 set_status 到 plan_generation 或更后；不确定就少配工具、多陪聊。只输出那个 JSON，不要多余文字。`

// OrchestratorToolCall is one raw tool call the model emitted; Args stays raw
// until a typed accessor validates it.
type OrchestratorToolCall struct {
	Name string          `json:"name"`
	Args json.RawMessage `json:"args"`
}

// OrchestratorDecision is the parsed, validated turn.
type OrchestratorDecision struct {
	Narrate string
	Tools   []OrchestratorToolCall
}

type rawOrchestratorOutput struct {
	Narrate string                 `json:"narrate"`
	Tools   []OrchestratorToolCall `json:"tools"`
}

var errOrchestratorParse = errors.New("orchestrator: output not parseable")

// knownOrchestratorTools is the closed set; anything else is dropped.
var knownOrchestratorTools = map[string]bool{
	"set_status": true, "open_tool": true, "curate_reference": true,
	"propose_note": true, "summon_card": true, "request_review": true,
}

// ParseOrchestratorOutput parses the model output, dropping unknown tools and
// tools whose args fail validation. Returns an error ONLY when the output will
// not unmarshal at all (caller retries once, then falls back).
func ParseOrchestratorOutput(text string) (OrchestratorDecision, error) {
	var out rawOrchestratorOutput
	if err := json.Unmarshal([]byte(stripFences(text)), &out); err != nil {
		return OrchestratorDecision{}, errOrchestratorParse
	}
	kept := make([]OrchestratorToolCall, 0, len(out.Tools))
	for _, tc := range out.Tools {
		if !knownOrchestratorTools[tc.Name] || !validToolArgs(tc) {
			continue
		}
		kept = append(kept, tc)
	}
	return OrchestratorDecision{Narrate: out.Narrate, Tools: kept}, nil
}

func validToolArgs(tc OrchestratorToolCall) bool {
	switch tc.Name {
	case "set_status":
		a, err := SetStatusArgs(tc)
		return err == nil && a.Stage.IsValid()
	case "open_tool":
		a, err := OpenToolArgs(tc)
		return err == nil && a.Tool.IsValid()
	case "curate_reference":
		_, err := CurateReferenceArgs(tc)
		return err == nil
	case "propose_note":
		a, err := ProposeNoteArgs(tc)
		return err == nil && validSection(a.Section)
	case "summon_card":
		a, err := SummonCardToolArgs(tc)
		return err == nil && a.CardID != ""
	case "request_review":
		return true
	}
	return false
}

func validSection(s string) bool {
	switch s {
	case "objective", "reason", "activities", "resources", "counterpoints":
		return true
	}
	return false
}

// --- typed arg accessors ---

type SetStatusArgsT struct {
	Stage StudioStage `json:"stage"`
}
type OpenToolArgsT struct {
	Tool   OpenTool `json:"tool"`
	Reason string   `json:"reason"`
}
type CurateReferenceArgsT struct {
	Items []ReferenceRef `json:"items"`
}
type ProposeNoteArgsT struct {
	Section string `json:"section"`
	Value   string `json:"value"`
}
type SummonCardToolArgsT struct {
	CardID    string `json:"card_id"`
	Reason    string `json:"reason"`
	NudgeText string `json:"nudge_text"`
}

func SetStatusArgs(tc OrchestratorToolCall) (SetStatusArgsT, error) {
	var a SetStatusArgsT
	err := json.Unmarshal(tc.Args, &a)
	return a, err
}
func OpenToolArgs(tc OrchestratorToolCall) (OpenToolArgsT, error) {
	var a OpenToolArgsT
	err := json.Unmarshal(tc.Args, &a)
	return a, err
}
func CurateReferenceArgs(tc OrchestratorToolCall) (CurateReferenceArgsT, error) {
	var a CurateReferenceArgsT
	err := json.Unmarshal(tc.Args, &a)
	return a, err
}
func ProposeNoteArgs(tc OrchestratorToolCall) (ProposeNoteArgsT, error) {
	var a ProposeNoteArgsT
	err := json.Unmarshal(tc.Args, &a)
	return a, err
}
func SummonCardToolArgs(tc OrchestratorToolCall) (SummonCardToolArgsT, error) {
	var a SummonCardToolArgsT
	err := json.Unmarshal(tc.Args, &a)
	return a, err
}
```

> **Executor note:** if the existing fence-stripper is named `stripJSONFence` (not `stripFences`), use that name — grep `internal/agent/` first. Do NOT introduce a second stripper.

- [ ] **Step 4: Run test to verify it passes**

Run: `cd apps/api && go test ./internal/agent/ -run 'TestParseOrchestratorOutput|TestOpenToolArgs'`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add apps/api/internal/agent/orchestrator.go apps/api/internal/agent/orchestrator_test.go
git commit -m "feat(agent): orchestrator system prompt + tool-call parser (drop-unknown, validate-args)"
```

---

### Task 4: Orchestrator LLM call

**Files:**
- Modify: `apps/api/internal/agent/orchestrator.go` (add the producer)
- Test: `apps/api/internal/agent/orchestrator_llm_test.go`

**Interfaces:**
- Consumes: `gateway.Provider`, `gateway.Resolved`, `gateway.Collect` (existing single-shot helper), `agent.ChatTurn` (existing history type — grep its definition in `project_coach.go`), `enforcement.BannedPhrasing` (existing).
- Produces: `agent.ProposeOrchestratorTurn(ctx context.Context, prov gateway.Provider, r gateway.Resolved, spineProjection string, state StudioState, history []ChatTurn) (OrchestratorDecision, gateway.ChatUsage, error)`. On a parse error it retries the LLM call **once**; on a second failure it returns the error (caller falls back to a plain narration).

**Design note:** message assembly mirrors `ProposeProjectCoachReply` (`project_coach.go:66`): `orchestratorSystemPrompt` as the system message; the `spineProjection` + current `state` (stage/openTool) ride in the final user message so one posture adapts to every stage. Reuse `BuildProjectCoachContext`'s history-mapping approach if convenient, but the system prompt is `orchestratorSystemPrompt`.

- [ ] **Step 1: Write the failing test (stub provider)**

Create `apps/api/internal/agent/orchestrator_llm_test.go`. Use the existing test stub provider pattern (grep `internal/agent/` and `internal/gateway/stub.go` for how other `*_test.go` files build a canned-response provider — reuse that helper, e.g. a `fakeProvider` returning fixed text). The test drives one canned tool-emitting turn:

```go
package agent

import (
	"context"
	"testing"
)

func TestProposeOrchestratorTurn_ParsesToolTurn(t *testing.T) {
	prov := newStubProvider(`{"narrate":"先聊聊你的问题。","tools":[{"name":"set_status","args":{"stage":"proposal_forming"}}]}`)
	dec, usage, err := ProposeOrchestratorTurn(context.Background(), prov, stubResolved(), "SPINE", DefaultStudioState(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if dec.Narrate == "" || len(dec.Tools) != 1 {
		t.Fatalf("bad decision: %+v", dec)
	}
	if usage.PromptTokens == 0 && usage.CompletionTokens == 0 {
		t.Log("stub returned zero usage (acceptable if stub does not populate usage)")
	}
}
```

> Match `newStubProvider` / `stubResolved` to whatever the existing agent tests actually call. If the existing stub returns malformed-then-valid on successive calls, add a second test `TestProposeOrchestratorTurn_RetriesOnParseError` asserting the retry path.

- [ ] **Step 2: Run test to verify it fails**

Run: `cd apps/api && go test ./internal/agent/ -run TestProposeOrchestratorTurn`
Expected: FAIL — undefined `ProposeOrchestratorTurn`.

- [ ] **Step 3: Implement the producer**

Append to `apps/api/internal/agent/orchestrator.go` (adapt `gateway.Collect` usage + `ChatRequest` construction to match `ProposeProjectCoachReply` exactly):

```go
// ProposeOrchestratorTurn makes ONE LLM call for a student turn and returns the
// parsed decision. Retries once on a parse failure; the caller falls back to a
// plain narration on a second failure.
func ProposeOrchestratorTurn(
	ctx context.Context,
	prov gateway.Provider,
	r gateway.Resolved,
	spineProjection string,
	state StudioState,
	history []ChatTurn,
) (OrchestratorDecision, gateway.ChatUsage, error) {
	req := buildOrchestratorRequest(spineProjection, state, history) // system=orchestratorSystemPrompt; spine+state+history in messages
	var lastUsage gateway.ChatUsage
	for attempt := 0; attempt < 2; attempt++ {
		res, err := gateway.Collect(ctx, prov, r, req)
		if err != nil {
			return OrchestratorDecision{}, lastUsage, err
		}
		lastUsage = res.Usage
		dec, perr := ParseOrchestratorOutput(res.Text)
		if perr == nil {
			return dec, lastUsage, nil
		}
	}
	return OrchestratorDecision{}, lastUsage, errOrchestratorParse
}
```

Implement `buildOrchestratorRequest` next to it, copying the exact `gateway.ChatRequest`/message-role types from `ProposeProjectCoachReply` (`project_coach.go`). Put `orchestratorSystemPrompt` in the system slot; render `spineProjection` + a short "当前阶段：<label>，当前打开：<tool>" line + the `history` turns into the user/assistant messages.

- [ ] **Step 4: Run test to verify it passes**

Run: `cd apps/api && go test ./internal/agent/ -run TestProposeOrchestratorTurn`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add apps/api/internal/agent/orchestrator.go apps/api/internal/agent/orchestrator_llm_test.go
git commit -m "feat(agent): ProposeOrchestratorTurn — one LLM call, parse, retry-once"
```

---

### Task 5: Rewire `POST /coach` to the orchestrator (handler + tool application)

**Files:**
- Modify: `apps/api/internal/api/coach.go` (`postCoach` — rewrite the decision half; keep the load/meter/persist scaffolding)
- Modify: `apps/api/internal/api/projectcoach.go` (retire `coachCardProposal`/`formingDimProposal` wiring from the response path; the card-eligibility helpers they call — `EligibleMoments`, `momentCard` — stay for `summon_card`'s effect)
- Test: `apps/api/internal/api/coach_orchestrator_test.go`

**Interfaces:**
- Consumes: `a.buildSpineProjection(ctx, projectID, surface)` (existing, `projectcoach.go:288`); `store.GetStudioState`/`SetStudioState` (Task 1); `agent.ProposeOrchestratorTurn` (Task 4); `store.RecordLLMCall`, `store.AppendProjectCoachMessage`, `store.LoadActiveCoachHistory` (existing); the existing `coach_proposed` event append.
- Produces: the new response shape = the `OrchestratorReply` contract (Task 2): `{ narrate, directive, note, card, reviewRequested }`. Request body simplifies to `{ user_input string }` (scope removed — stage comes from `studio_state`).

**Tool application rules (in `postCoach`, after `ProposeOrchestratorTurn`):**
1. Start from the loaded `studio_state`; increment `UpdatedAtTurn`.
2. For each parsed tool, in order:
   - `set_status` → `state.Stage = args.Stage`.
   - `open_tool` → `state.OpenTool = args.Tool; state.WidthTier = agent.WidthForTool(args.Tool)`.
   - `curate_reference` → `state.Reference = args.Items`.
   - `propose_note` → set `reply.Note = {section, value}` (last one wins; **no** DB write — student confirms via existing `putProposal`).
   - `summon_card` → run the existing eligibility guard (reuse `coachProposeSurfaces`/`EligibleMoments`/in-flight-card check from `projectcoach.go`); if eligible, set `reply.Card = {cardId, reason, nudgeText}` and append the `coach_proposed` event. If not eligible, drop silently.
   - `request_review` → `state.OpenTool = ToolWriting; state.WidthTier = WidthWide; reply.ReviewRequested = true`.
3. `SetStudioState(projectID, state)`.
4. `reply.Directive = state`, `reply.Narrate = dec.Narrate` (fall back to `coachFallbackReply` when `dec.Narrate == ""` or the turn errored).
5. Persist the student turn and the assistant narration to `chat_message` with `surface = "studio"` (see Task 6 for why `studio` becomes a stored surface). Meter `RecordLLMCall(Surface:"studio", Purpose:"coach")` **before any bail** (keep the existing pre-bail metering position).

- [ ] **Step 1: Write the failing handler test**

Create `apps/api/internal/api/coach_orchestrator_test.go`. Follow the existing handler-test harness in `internal/api/` (grep for a `*_test.go` that spins testcontainers + seeds a project + injects a stub `ChatResolver`). The test posts one turn whose stub model output drives `set_status` + `open_tool` + `propose_note`, then asserts:

```go
// pseudo-shape — match the real harness's helpers
func TestPostCoach_AppliesOrchestratorDirective(t *testing.T) {
	env := newAPITestEnv(t) // seeds Phoebe + a project; stub resolver
	env.stubModel(`{"narrate":"写作面板开好了。","tools":[` +
		`{"name":"set_status","args":{"stage":"body_writing"}},` +
		`{"name":"open_tool","args":{"tool":"writing","reason":"该写正文"}},` +
		`{"name":"propose_note","args":{"section":"objective","value":"净影响"}}]}`)

	resp := env.postJSON(t, "/api/v1/projects/"+env.projectID+"/coach", map[string]any{"user_input": "我准备好写了"})

	// response shape
	if resp["narrate"] == "" { t.Fatal("empty narrate") }
	dir := resp["directive"].(map[string]any)
	if dir["stage"] != "body_writing" || dir["openTool"] != "writing" || dir["widthTier"] != "wide" {
		t.Fatalf("directive not applied: %v", dir)
	}
	note := resp["note"].(map[string]any)
	if note["section"] != "objective" { t.Fatalf("note: %v", note) }

	// persisted studio_state
	st := env.getStudioState(t, env.projectID)
	if st.Stage != "body_writing" { t.Fatalf("studio_state not persisted: %+v", st) }
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd apps/api && go test ./internal/api/ -run TestPostCoach_AppliesOrchestratorDirective`
Expected: FAIL (old `postCoach` returns `reply/proposal/dimSuggestion`, not `narrate/directive`).

- [ ] **Step 3: Rewrite `postCoach`'s decision half**

In `apps/api/internal/api/coach.go`: keep steps 1–4 and 6 of the existing sequence (owned-project + entitlement gate, resolver, `LoadActiveCoachHistory`, persist student turn, pre-bail meter). Replace steps 5 + 9 (the `ProposeProjectCoachReply` call and the `formingDimProposal`/`coachCardProposal` response assembly) with: load `studio_state` → `ProposeOrchestratorTurn` → apply tools per the rules above → `SetStudioState` → build `OrchestratorReply`. Change the request struct to `{ UserInput string }` and drop `Scope`. Change the response marshaling to the `OrchestratorReply` JSON.

Keep `coachFallbackReply`, `maybeCompactBackstop`, `TouchProject`, and the `coach_turn` event append.

- [ ] **Step 4: Prune the now-dead response producers**

In `projectcoach.go`, remove the calls that assembled `resp["proposal"]`/`resp["dimSuggestion"]` from the old path (`formingDimProposal`, and `coachCardProposal`'s response wiring). Keep the eligibility helpers `summon_card` reuses (`coachProposeSurfaces`, `EligibleMoments`, the in-flight-card guard, the `coach_proposed` event). Delete `ProposeFormingDim`/`FormingDimSuggestion` only if nothing else references them (grep first; if the reading path or tests use them, leave them). Record any deletions in the commit message.

- [ ] **Step 5: Run the full package**

Run: `cd apps/api && go test ./internal/api/ ./internal/agent/`
Expected: PASS. (Run the FULL packages, not a `-run` subset — the rewrite touches shared handler wiring.)

- [ ] **Step 6: Commit**

```bash
git add apps/api/internal/api/coach.go apps/api/internal/api/projectcoach.go apps/api/internal/api/coach_orchestrator_test.go
git commit -m "feat(api): route /coach through the orchestrator loop — apply directive, retire scattered producers"
```

---

### Task 6: `studio` as the stored surface + `GET /studio-state`

**Files:**
- Modify: `apps/api/internal/api/coach.go` (`getCoachHistory` — the `surface=="studio"` branch now reads stored `surface="studio"`; `workingCoachSurfaces` union collapses)
- Modify: `apps/api/internal/api/api.go` (route table — add `GET /projects/{id}/studio-state`)
- Create: `apps/api/internal/api/studiostate_handler.go` (`getStudioStateHandler`)
- Test: `apps/api/internal/api/studiostate_handler_test.go`

**Interfaces:**
- Produces: `GET /api/v1/projects/{id}/studio-state` → `StudioState` JSON (Task 2 shape). Consumes `store.GetStudioState` (Task 1) + `loadOwnedProject` (existing gate).

**Design note (no-compat simplification):** Task 5 persists every orchestrator turn with `surface = "studio"`. So `getCoachHistory("studio")` now reads the single stored `studio` surface instead of unioning `["forming","proposal_review","writing"]`. Reading + reflection sub-agents keep their own surfaces and stay excluded. Simplify `workingCoachSurfaces` accordingly (or read `ListChatMessagesByProjectSurface(projectID, "studio")` directly).

- [ ] **Step 1: Write the failing test**

Create `apps/api/internal/api/studiostate_handler_test.go`:

```go
func TestGetStudioState_ReturnsPersistedState(t *testing.T) {
	env := newAPITestEnv(t)
	// drive one orchestrator turn that advances the stage
	env.stubModel(`{"narrate":"ok","tools":[{"name":"set_status","args":{"stage":"plan_generation"}}]}`)
	env.postJSON(t, "/api/v1/projects/"+env.projectID+"/coach", map[string]any{"user_input": "go"})

	st := env.getJSON(t, "/api/v1/projects/"+env.projectID+"/studio-state")
	if st["stage"] != "plan_generation" {
		t.Fatalf("stage=%v", st["stage"])
	}
}

func TestGetStudioState_FreshProjectIsDefault(t *testing.T) {
	env := newAPITestEnv(t)
	st := env.getJSON(t, "/api/v1/projects/"+env.projectID+"/studio-state")
	if st["stage"] != "topic_discussion" || st["openTool"] != "chat" {
		t.Fatalf("fresh default wrong: %v", st)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd apps/api && go test ./internal/api/ -run TestGetStudioState`
Expected: FAIL — route 404.

- [ ] **Step 3: Implement the handler + route + history simplification**

Create `getStudioStateHandler` (load owned project, `GetStudioState`, unmarshal to `agent.StudioState`, write JSON). Register `GET /projects/{id}/studio-state` in `api.go` next to the existing `GET /projects/{id}/coach/history`. Simplify the `getCoachHistory` `"studio"` branch to read `surface="studio"`.

- [ ] **Step 4: Run the full package**

Run: `cd apps/api && go test ./internal/api/`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add apps/api/internal/api/studiostate_handler.go apps/api/internal/api/coach.go apps/api/internal/api/api.go apps/api/internal/api/studiostate_handler_test.go
git commit -m "feat(api): GET /studio-state + studio as the single stored coach surface"
```

---

# Frontend

### Task 7: API client — `getStudioState` + rewritten `coach`

**Files:**
- Modify: `apps/web/src/workspace/api/workspace.ts` (rewrite `coach`; add `getStudioState`)
- Modify: `apps/web/src/api/index.ts` (barrel — expose the two)
- Test: `apps/web/src/workspace/api/workspace.test.ts` (add cases; create if absent)

**Interfaces:**
- Produces: `coach(id: string, userInput: string) => Promise<OrchestratorReply>` (import the type from `@mind-imprint/contracts`); `getStudioState(id: string) => Promise<StudioState>`.
- The old `coach(id, scope, userInput) => CoachResult` signature and `CoachScope` are retired for the studio thread (reading/reflection keep their own turn APIs — do not touch `api/reading.ts`).

- [ ] **Step 1: Write the failing test**

In `apps/web/src/workspace/api/workspace.test.ts`, mock `apiFetch` and assert the new `coach` POSTs `{ user_input }` (no `scope`) to `/api/v1/projects/{id}/coach` and returns the parsed reply; assert `getStudioState` GETs `/studio-state`. (Match the file's existing mocking style — grep for how other functions there are tested.)

- [ ] **Step 2: Run test to verify it fails**

Run: `cd apps/web && npx vitest run src/workspace/api/workspace.test.ts`
Expected: FAIL.

- [ ] **Step 3: Implement**

Rewrite `coach` and add `getStudioState`:

```ts
import type { OrchestratorReply, StudioState } from "@mind-imprint/contracts";

export function coach(id: string, userInput: string): Promise<OrchestratorReply> {
  return apiFetch<OrchestratorReply>(`/api/v1/projects/${id}/coach`, {
    method: "POST",
    body: JSON.stringify({ user_input: userInput }),
  });
}

export function getStudioState(id: string): Promise<StudioState> {
  return apiFetch<StudioState>(`/api/v1/projects/${id}/studio-state`);
}
```

Update `api/index.ts` to re-export both. Remove the dead `CoachScope`/`CoachResult` studio usages the compiler now flags (Task 10 finishes the cleanup).

- [ ] **Step 4: Run test to verify it passes**

Run: `cd apps/web && npx vitest run src/workspace/api/workspace.test.ts`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add apps/web/src/workspace/api/workspace.ts apps/web/src/api/index.ts apps/web/src/workspace/api/workspace.test.ts
git commit -m "feat(web): studio API client — getStudioState + orchestrator coach()"
```

---

### Task 8: Resume-at-stage + chat-first landing

**Files:**
- Modify: `apps/web/src/workspace/WorkspaceContainer.tsx` (load `studio_state` on open; drive `room` from `openTool`; chat-first when `openTool==="chat"`)
- Create: `apps/web/src/workspace/blocks/ChatFirstLanding.tsx` (the empty interactive-area state when chat is primary)
- Test: `apps/web/src/workspace/WorkspaceContainer.test.tsx` (extend existing container test — mocks already stub `getPlan`/`getCoachHistory`; add `getStudioState`)

**Interfaces:**
- Consumes: `getStudioState` (Task 7); `StudioState`, `OpenTool` (contracts).
- New container state: `studioState: StudioState | null`. Mapping `openTool → room`: `plan→"plan"`, `reading→"reading"`, `writing→"writing"`, `reflection→"reflection"`, `chat→` chat-first (no room board). Loaded once per project in the existing project-load effect.

**Design note:** today `openProject` forces `setRoom("plan")` (`WorkspaceContainer.tsx:290`) and the load effect always lands on the plan board. Replace that with: after `getStudioState`, if `state.openTool === "chat"` render `<ChatFirstLanding/>` in `<main>` (chat panel stays primary via the existing portal); else `setRoom(openToolToRoom(state.openTool))`. The chat panel + `StudioChatContext` are unchanged — only what fills `<main>` changes.

- [ ] **Step 1: Write the failing test**

Extend `WorkspaceContainer.test.tsx`: mock `getStudioState` to return `{ stage:"topic_discussion", openTool:"chat", ... }` and assert the container renders the chat-first landing (a `data-testid="chat-first"` marker) and NOT the plan board. A second case returns `openTool:"writing"` and asserts the writing room mounts. Add `getStudioState: vi.fn(...)` to the api mock (the container test already mocks `getPlan`/`getCoachHistory`/`getWorkspace` — follow that pattern so the new fetch does not crash the mock).

- [ ] **Step 2: Run test to verify it fails**

Run: `cd apps/web && npx vitest run src/workspace/WorkspaceContainer.test.tsx`
Expected: FAIL.

- [ ] **Step 3: Implement `ChatFirstLanding.tsx`**

A calm empty state for the interactive area when 印记 hasn't opened a room yet — a centered `Pebble` + one line ("印记正在陪你把项目理清楚"). Design-system compliant (solid `mk-*` tokens, one class per property). Carries `data-testid="chat-first"`.

```tsx
import { Pebble } from "@/ui";

export function ChatFirstLanding() {
  return (
    <div data-testid="chat-first" className="flex h-full flex-col items-center justify-center gap-3 text-center">
      <Pebble state="idle" size={40} />
      <p className="text-[13px] text-mk-muted">印记正在陪你把项目理清楚<br />准备好了，它会为你打开对的工作台</p>
    </div>
  );
}
```

- [ ] **Step 4: Wire the container**

Add `studioState` state + load it in the project-load effect. Add `openToolToRoom`. Replace the forced-`plan` landing with the stage-driven branch. When `openTool==="chat"`, render `<ChatFirstLanding/>` in `<main>` instead of a room. Keep the switcher (Segmented/PlanSpine/NextStepGuide) mounted — it is now the manual-takeover entry (spec §6); do not remove it.

- [ ] **Step 5: Run test to verify it passes**

Run: `cd apps/web && npx vitest run src/workspace/WorkspaceContainer.test.tsx`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add apps/web/src/workspace/WorkspaceContainer.tsx apps/web/src/workspace/blocks/ChatFirstLanding.tsx apps/web/src/workspace/WorkspaceContainer.test.tsx
git commit -m "feat(web): resume-at-stage + chat-first landing (no forced plan board)"
```

---

### Task 9: Apply the directive on every turn (send loop + note/card chips)

**Files:**
- Modify: `apps/web/src/workspace/WorkspaceContainer.tsx` (own the send loop OR pass an `onDirective` down; whichever matches the current portal wiring)
- Modify: `apps/web/src/workspace/blocks/PlanBlock.tsx` + `apps/web/src/workspace/blocks/WritingBlock.tsx` (their `send()` now consumes `OrchestratorReply`: append `narrate`, apply `directive`, render `note`/`card` chips)
- Test: extend the block tests that already cover the coach send (grep for existing `send`/coach tests in `PlanBlock`/`WritingBlock`).

**Interfaces:**
- Consumes: `coach(id, userInput) => OrchestratorReply` (Task 7); the container's `applyDirective(state: StudioState)` (switches room via `openToolToRoom`, stores `studioState`); the existing `activeProjectIdRef`/`isActiveProject()` in-flight guard (keep it — apply the directive only when `isActiveProject()`).
- On a reply: (1) push the `narrate` as an `ai` message into `StudioChatContext`; (2) `applyDirective(reply.directive)`; (3) if `reply.note`, render the existing note-confirm chip (reuse `CoachProposal`/the dimSuggestion chip UI — student confirm → `putProposal`); (4) if `reply.card`, render the existing card chip (`CardTurnChip`); (5) if `reply.reviewRequested`, the directive already opened writing — no extra work.

**Design note:** this replaces the three separate response branches (`reply`, `proposal`, `dimSuggestion`) each block had. The narration + directive are uniform; note/card are optional chips. Keep the `isActiveProject()` guard on every post-await state write (the cross-project leak fix from `474f04e`).

- [ ] **Step 1: Write the failing test**

Extend the block test: mock `coach` to return a reply with `directive.openTool="writing"` + a `note`, fire the composer send, assert (a) the narrate text appears in the chat log, (b) the container switched to the writing room (or `applyDirective` was called with the directive), (c) the note-confirm chip rendered.

- [ ] **Step 2: Run test to verify it fails**

Run: `cd apps/web && npx vitest run src/workspace/blocks/`
Expected: FAIL.

- [ ] **Step 3: Implement the send loop**

Rewrite the coach `send()` in `PlanBlock`/`WritingBlock` (and/or lift it to the container) to consume `OrchestratorReply`. Thread `applyDirective` from the container. Guard every post-await write with `isActiveProject()`.

- [ ] **Step 4: Run test to verify it passes**

Run: `cd apps/web && npx vitest run src/workspace/blocks/`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add apps/web/src/workspace/WorkspaceContainer.tsx apps/web/src/workspace/blocks/PlanBlock.tsx apps/web/src/workspace/blocks/WritingBlock.tsx
git commit -m "feat(web): apply orchestrator directive per turn — narrate + room switch + note/card chips"
```

---

### Task 10: Retire dead studio coach shapes + full green

**Files:**
- Modify: any remaining references to `CoachScope`/`CoachResult`/`dimSuggestion`/the old `coach(id,scope,input)` studio call flagged by `tsc`.
- Test: the whole web + Go suites.

**Interfaces:** none new — this is the cleanup + verification task.

- [ ] **Step 1: Typecheck the web app**

Run: `cd apps/web && npx tsc --noEmit`
Fix every error that stems from the retired studio coach shapes (unused `CoachScope`, old `CoachResult` destructures, stale `dimSuggestion` handling). Do NOT touch reading/reflection turn code — those keep their own APIs.

- [ ] **Step 2: Run the full web suite**

Run: `cd apps/web && npx vitest run`
Expected: PASS. Fix any test still asserting the old `reply`/`proposal`/`dimSuggestion` studio response shape.

- [ ] **Step 3: Run the full Go suite**

Run: `cd apps/api && go test ./...`
Expected: PASS.

- [ ] **Step 4: Commit**

```bash
git add -u
git commit -m "chore(studio): retire dead studio coach shapes; full suites green"
```

---

## Manual verification (after Task 10, before the branch review)

Bring up the local stack (recipe in memory `agentic-writing-studio-2026-08-07`): docker `postgres:16-alpine` on :5433, `go run ./cmd/api --migrate-up` (seeds Phoebe + migration 0057), api on :8080, web on :5178. Open `http://localhost:5178/?trial=1`.

- [ ] A fresh project opens into the **chat-first landing** (no plan board), stage `立题讨论`.
- [ ] Talk through a proposal; watch 印记 fire `propose_note` chips you confirm, and `set_status`/`open_tool` move you into rooms with a narrated reason.
- [ ] Close and reopen the project → it **resumes at its stage** (writing project lands in writing, not the board).
- [ ] Manually switch rooms via the switcher → allowed; `studio_state` unchanged until 印记's next turn (P1: takeover doesn't yet write status back — that is P4).
- [ ] Confirm `llm_call` rows accrue with `surface=studio, purpose=coach`.

## Non-goals (this plan)

- ❌ Morphing interactive-area **width** (chat→half→wide render) — P2. P1 stores `widthTier` but the shell does not yet animate to it.
- ❌ **Dynamic AI-curated reference panel** render (materials fold-in, 批注-on-review, per-stage sets) — P3. P1 stores `reference` via `curate_reference` but keeps the current `WritingReferencePanel`.
- ❌ **"继续印记"** takeover-return + writing status back from manual navigation — P4.
- ❌ Streamed token-by-token narration — deferred (tools are embedded in the same JSON, so P1 is one-shot).
- ❌ Removing the legacy station-studio model (`packages/contracts/src/studioState.ts` `StudioProjection`, `internal/studio/`, `postProjectTurn`/`RunAgentStep`) — a separate cleanup once the four rooms fully run on the orchestrator.

---

## Self-Review (run by author)

**Spec coverage** (against `2026-08-07-agentic-studio-orchestrator-redesign.md` §8 待实现):
- ★ Real agentic framework (agent loop + tool set) → Tasks 3–5. ✅
- Status state machine (storage + AI-driven) → Tasks 1, 5. ✅
- Resume at status → Task 8. ✅
- `curate_reference` contract laid (render deferred to P3) → Tasks 2, 5 store it; §Non-goals defers render. ✅
- Width tier stored, render deferred → Task 1/5 store; P2. ✅ (spec-aligned deferral)
- Narration tone (orchestrator posture) → Task 3 prompt. ✅
- Takeover-without-status-change → explicitly P4 (Non-goals). ✅ (spec §6 deferred)

**Type consistency:** `StudioStage`/`OpenTool`/`WidthTier`/`ProposalSection`/`ReferenceRef`/`StudioState` names identical across Go (Task 1) and Zod (Task 2); `OrchestratorReply` fields (`narrate/directive/note/card/reviewRequested`) identical in Task 2 contract, Task 5 handler, Task 7 client. Tool arg JSON names (`card_id`/`nudge_text` snake, others as listed) consistent between contract union and Go accessors.

**Placeholder scan:** the two spots that require reading existing code before writing (the fence-stripper name in Task 3; the stub-provider/test-harness helpers in Tasks 4–6) are called out explicitly with a grep instruction rather than invented — deliberate, because inventing those names would drift from the real codebase.
