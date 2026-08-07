# P2b · 印记's new agent capabilities Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Give 印记 the agent capabilities that let it *do* what two now-redundant buttons did — trigger plan generation itself (`generate_plan`) and propose exploration questions itself (`propose_question`) — plus an explicit `forming` openTool so it opens 提案 vs 管理 distinctly; then strip the `生成项目计划` button + the reading-room question boxes those tools replace.

**Architecture:** Extends the P1 orchestrator loop with three new tool vocabulary items. `forming` joins the `openTool` enum (contracts + Go) so 印记 configures 提案 vs 管理 explicitly. `generate_plan` reuses the existing plan-generation logic — extracted into a shared `regeneratePlan` — and is applied server-side in the coach turn. `propose_question` mirrors the ephemeral `propose_note` pattern: the model proposes, the reply carries it, the student confirms client-side via the existing `createLead` endpoint (no DB write until confirmed, no migration). The frontend then removes the `生成项目计划` button/regen-modal and the manual question-entry boxes, and surfaces a question confirm-chip.

**Tech Stack:** Go (`net/http`, sqlc, pgx) `apps/api` · Zod contracts `packages/contracts` · React+TS+Tailwind `apps/web`. Built on branch `studio-p2-focused-but-free` (on top of P2a @`d4ae931`).

## Global Constraints

- **Spec of record:** `docs/superpowers/specs/2026-08-07-agentic-studio-orchestrator-redesign.md` §0.5 (印记 IS the agent), §7.5 (the Tier-2 kills these tools unblock), §8 (the agentic tool set). No old-data compatibility — free breaking changes.
- **The card/tool JSON single source of truth is the contract.** Any new field in the coach reply MUST exist in `packages/contracts/src/orchestrator.ts` or the client's strict Zod `.parse()` throws and swallows the whole turn.
- **Client never emits orchestrator tools** — the MODEL emits them, parsed server-side (drop-unknown, validate-args). New tools must be added to the allowlist (`knownOrchestratorTools`), the system prompt, `validToolArgs`, and a typed accessor — miss one and the tool is silently dropped or mis-applied.
- **Secrets only in `apps/api`.** Every LLM call meters (档位+token+cost); 决策绝不降级 (orchestrator uses the flagship tier already wired).
- **铁律:** 印记 proposes, the student confirms (propose_question is a chip, never an auto-write); 印记 never decides for the student; generation is not destructive to student prose (it's the plan board).
- **Go on macOS:** `CGO_ENABLED=0`; sqlc regen pinned `@v1.27.0`: `CGO_ENABLED=0 go run github.com/sqlc-dev/sqlc/cmd/sqlc@v1.27.0 generate`. Tests need Docker (testcontainers, slow ~6min) — **run Go tests in the FOREGROUND only** (backgrounding the testcontainers suite stalls subagents). Prefer `go test ./internal/agent/...` (no docker) where possible; the docker-bound `internal/api` suite runs once at the end.
- **Web:** pnpm; tests `apps/web/test/**` (glob `test/**`); `pnpm exec tsc --noEmit` exit 0 + `pnpm exec vitest run`. Never `bg-mk-<token>/<opacity>` (transparent). Never `git add -A`.
- **Pre-existing-on-main Go failures** (NOT P2b): `TestProjectsEndpoints`, `TestWeeklyReportForSeededClass` — ignore, do not try to fix here.

---

## File Structure

- `packages/contracts/src/orchestrator.ts` — `OpenTool` +`forming`; `OrchestratorTool` union +`generate_plan`,+`propose_question`; `QuestionProposal`; `OrchestratorReply` +`question` (T1).
- `apps/api/internal/agent/studiostate.go` — `OpenTool` +`ToolForming`; `WidthForTool` (forming→half, plan→wide); `IsValid` (T2).
- `apps/api/internal/agent/orchestrator.go` — allowlist, prompt, `validToolArgs`, `GeneratePlanArgs`/`ProposeQuestionArgs` accessors (T3).
- `apps/api/internal/api/workspace_plan_generate.go` — extract `regeneratePlan(ctx, projectID) ([]planItemDTO, error)` from `postPlanGenerate`, both call it (T4).
- `apps/api/internal/api/coach.go` — apply `generate_plan` + `propose_question`; `orchestratorReplyDTO` +`Question`; `questionProposalDTO` (T4).
- `apps/web/src/workspace/studioResume.ts` — `openToolToRoom`/`roomForResume` handle `forming` (T5).
- `apps/web/src/api/exploration.ts` (or its client) + contracts client — `OrchestratorReply.question` consumed (T5).
- `apps/web/src/studio/ai/StudioChatContext.tsx` + `StudioCoachChat.tsx` — `pendingQuestion` + confirm chip → `createLead` (T7).
- `apps/web/src/workspace/WorkspaceContainer.tsx` — `confirmQuestion` + exploration-refresh nonce; refresh plan after a turn (T6/T7).
- `apps/web/src/workspace/blocks/PlanBlock.tsx` — remove `生成项目计划` button + `RegenConfirm` (T6).
- `apps/web/src/workspace/blocks/exploration/ExplorationView.tsx` — remove `记下问题`/`从笔记新建问题`/seed; consume the refresh nonce (T8).

---

## Task 1: contracts — forming openTool + generate_plan/propose_question + reply.question

**Files:** Modify `packages/contracts/src/orchestrator.ts`; Test `packages/contracts/test/orchestrator.test.ts` (add cases).

**Interfaces — Produces:** `OpenTool` gains `"forming"`. `QuestionProposal = { text: string }`. `OrchestratorTool` union gains `{name:"generate_plan", args:{}}` and `{name:"propose_question", args:{ text: string }}`. `OrchestratorReply` gains `question: QuestionProposal | null`.

- [ ] **Step 1: Write failing test** in `packages/contracts/test/orchestrator.test.ts`:

```ts
import { OpenTool, OrchestratorTool, OrchestratorReply } from "../src/orchestrator";
it("openTool includes forming", () => { expect(OpenTool.safeParse("forming").success).toBe(true); });
it("parses generate_plan and propose_question tools", () => {
  expect(OrchestratorTool.safeParse({ name: "generate_plan", args: {} }).success).toBe(true);
  expect(OrchestratorTool.safeParse({ name: "propose_question", args: { text: "中国的人均碳排放算高吗？" } }).success).toBe(true);
});
it("reply carries a nullable question proposal", () => {
  const r = OrchestratorReply.safeParse({ narrate: "x", directive: { stage: "topic_discussion", openTool: "forming", widthTier: "half", reference: [], updatedAtTurn: 0 }, note: null, card: null, question: { text: "q" }, reviewRequested: false });
  expect(r.success).toBe(true);
});
```

Run: `cd packages/contracts && pnpm exec vitest run test/orchestrator.test.ts` — FAIL.

- [ ] **Step 2: Implement.** In `orchestrator.ts`:
  - `OpenTool`: `z.enum(["chat", "forming", "plan", "reading", "writing", "reflection"])`.
  - Add `export const QuestionProposal = z.object({ text: z.string() }); export type QuestionProposal = z.infer<typeof QuestionProposal>;`.
  - `OrchestratorTool` union: add `z.object({ name: z.literal("generate_plan"), args: z.object({}) })` and `z.object({ name: z.literal("propose_question"), args: z.object({ text: z.string() }) })`.
  - `OrchestratorReply`: add `question: QuestionProposal.nullable(),`.
- [ ] **Step 3: Re-export** any new type from `src/index.ts` if the barrel enumerates them (grep `QuestionProposal`/`NoteProposal` — mirror how `NoteProposal` is exported).
- [ ] **Step 4:** Run the contracts suite green. `pnpm exec tsc --noEmit` in `packages/contracts`.
- [ ] **Step 5: Commit** `feat(contracts): forming openTool + generate_plan/propose_question tools + reply.question (P2b)`.

---

## Task 2: Go studiostate — forming openTool + width

**Files:** Modify `apps/api/internal/agent/studiostate.go`; Test `apps/api/internal/agent/studiostate_test.go` (create or extend).

**Interfaces — Produces:** `ToolForming OpenTool = "forming"`; `WidthForTool(ToolForming) == WidthHalf`; `WidthForTool(ToolPlan) == WidthWide`; `ToolForming.IsValid() == true`.

- [ ] **Step 1: Failing test** (no docker — `go test ./internal/agent/`):

```go
func TestWidthForTool_FormingAndPlan(t *testing.T) {
	if WidthForTool(ToolForming) != WidthHalf { t.Errorf("forming → %s, want half", WidthForTool(ToolForming)) }
	if WidthForTool(ToolPlan) != WidthWide { t.Errorf("plan → %s, want wide", WidthForTool(ToolPlan)) }
	if !ToolForming.IsValid() { t.Error("forming must be valid") }
}
```

- [ ] **Step 2: Implement.** Add `ToolForming OpenTool = "forming"` to the const block; add `ToolForming` to `IsValid`'s switch; update `WidthForTool` so ONLY 提案(forming) is half — the plan board, reading, writing, reflection are all wide (spec §3: 项目计划/阅读/写作 交互区变宽; 提案要点 半宽):

```go
func WidthForTool(t OpenTool) WidthTier {
	switch t {
	case ToolChat:
		return WidthChat
	case ToolForming:
		return WidthHalf
	default: // plan, reading, writing, reflection
		return WidthWide
	}
}
```

(NOTE: this also corrects P1's `plan → half` to `plan → wide`, matching the spec's "项目计划 变宽". The progression reads well: 提案要点 half → 管理 board wide once 印记 generates it.)

- [ ] **Step 3:** `go test ./internal/agent/` green (foreground). **Step 4: Commit** `feat(agent): forming openTool + width mapping (P2b)`.

---

## Task 3: Go orchestrator — register generate_plan + propose_question

**Files:** Modify `apps/api/internal/agent/orchestrator.go`; Test `apps/api/internal/agent/orchestrator_test.go` (extend).

**Interfaces — Produces:** `GeneratePlanArgs(tc)` (no args, presence only); `ProposeQuestionArgs(tc) (ProposeQuestionArgsT{Text string}, error)`. Both tool names pass the allowlist + `validToolArgs`.

- [ ] **Step 1: Failing test** — parser keeps the two new tools + validates args:

```go
func TestParseOrchestratorOutput_NewTools(t *testing.T) {
	raw := `{"narrate":"我来生成计划","tools":[{"name":"generate_plan","args":{}},{"name":"propose_question","args":{"text":"人均碳排放呢？"}}]}`
	dec, err := ParseOrchestratorOutput(raw)
	if err != nil { t.Fatal(err) }
	if len(dec.Tools) != 2 { t.Fatalf("want 2 tools, got %d", len(dec.Tools)) }
	q, err := ProposeQuestionArgs(dec.Tools[1])
	if err != nil || q.Text == "" { t.Fatalf("propose_question args: %v %q", err, q.Text) }
}
```

- [ ] **Step 2: Implement.**
  - `knownOrchestratorTools`: add `"generate_plan": true, "propose_question": true,`.
  - `validToolArgs`: add `case "generate_plan": return true` (no args); `case "propose_question":` unmarshal + require non-empty `Text`.
  - Typed accessor + struct: `type ProposeQuestionArgsT struct { Text string \`json:"text"\` }` + `func ProposeQuestionArgs(tc OrchestratorToolCall) (ProposeQuestionArgsT, error)` mirroring `ProposeNoteArgs`. (`generate_plan` needs no accessor — presence is the signal.)
  - **System prompt** (`orchestratorSystemPrompt`): document both tools in the tool list, e.g.:
    - `- generate_plan: {} —— 四项必填提案要点都齐了、该把计划落出来时，由你生成项目计划（不再有按钮）。要重排已有计划前，先在 narrate 里征得学生同意。`
    - `- propose_question: {"text": 问题} —— 在阅读/探索时，向学生提议一个值得追的研究问题（学生确认后才加入探索图谱；一次一个）。`
    - Update the closing principle line to mention 印记 opens `forming`(提案) during proposal_forming and `plan`(管理) after generate.
- [ ] **Step 3:** `go test ./internal/agent/` green (foreground). **Step 4: Commit** `feat(agent): register generate_plan + propose_question orchestrator tools (P2b)`.

---

## Task 4: Go coach apply — generate_plan (extract regeneratePlan) + propose_question

**Files:** Modify `apps/api/internal/api/workspace_plan_generate.go` (extract), `apps/api/internal/api/coach.go` (apply + reply DTO); Test `apps/api/internal/api/*_test.go` (docker suite — run once, foreground).

**Interfaces — Produces:** `func (a *API) regeneratePlan(ctx context.Context, projectID uuid.UUID) ([]planItemDTO, error)` — proposal-empty check → sentinel `errProposalEmpty`, generate, delete+recreate in tx, auto-log, fold; called by BOTH `postPlanGenerate` and the coach `generate_plan` case. `orchestratorReplyDTO.Question *questionProposalDTO`.

- [ ] **Step 1: Extract `regeneratePlan`.** Move the body of `postPlanGenerate` (proposal fetch + empty-check + `generatePlanItems` + the tx delete/recreate + `appendAutoLog` + `FoldCoachSurfaces`) into `regeneratePlan(ctx, projectID) ([]planItemDTO, error)`. Return `errProposalEmpty` (a package sentinel) instead of writing the 422 directly. `postPlanGenerate` becomes: load project → entitlement → `items, err := a.regeneratePlan(...)`; on `errProposalEmpty` write the existing 422 `proposal_empty`; else write `{items}`. Run the existing plan-generate test to confirm no behavior change (foreground).
- [ ] **Step 2: Apply `generate_plan` in coach.go.** In the tool `switch` (after `request_review`), add:

```go
case "generate_plan":
	// 印记 triggers plan generation itself (the 生成计划 button is gone). Best-
	// effort: a proposal-empty project just doesn't generate — the narration
	// still lands, 印记 will have nudged for the dims. Open 管理 on success.
	if _, gerr := a.regeneratePlan(r.Context(), projectID); gerr == nil {
		state.OpenTool = agent.ToolPlan
		state.WidthTier = agent.WidthForTool(agent.ToolPlan)
	}
```

- [ ] **Step 3: Apply `propose_question` in coach.go.** Add:

```go
case "propose_question":
	if args, aerr := agent.ProposeQuestionArgs(tc); aerr == nil && strings.TrimSpace(args.Text) != "" {
		reply.Question = &questionProposalDTO{Text: args.Text}
	}
```

Add `questionProposalDTO struct { Text string \`json:"text"\` }` and `Question *questionProposalDTO \`json:"question"\`` on `orchestratorReplyDTO` (mirror `noteProposalDTO`/`Note`). **Also add `Question` to the OTHER `orchestratorReplyDTO{...}` construction** (the sub-agent path around `coach.go:356`) so every reply serializes the field (client Zod requires it present — nullable, so `nil`→`null` is fine; verify the struct's `json` tag emits `null` not omitempty).

- [ ] **Step 4: Test (docker, foreground).** Add an api-level test: a coach turn whose stubbed model returns a `generate_plan` tool results in plan rows existing after + directive `openTool=="plan"`; a `propose_question` tool populates `reply.question.text` and writes NO lead. Use the existing coach-test harness (`gateway.SequenceStubProvider`). Run `CGO_ENABLED=0 go test ./internal/api/ -run TestCoach` foreground.
- [ ] **Step 5: Commit** `feat(api): generate_plan + propose_question coach tools; extract regeneratePlan (P2b)`.

---

## Task 5: web — forming resume mapping + reply.question type flows through

**Files:** Modify `apps/web/src/workspace/studioResume.ts`; Test `apps/web/test/workspace/studioResume.test.ts`.

**Interfaces — Consumes:** contracts `OpenTool` now includes `forming`; `OrchestratorReply.question`. **Produces:** `openToolToRoom("forming") === "forming"`; `roomForResume` returns `"forming"` when `openTool==="forming"` (explicit wins over the stage heuristic).

- [ ] **Step 1: Failing test:**

```ts
it("openTool forming maps to the forming room", () => {
  expect(openToolToRoom("forming")).toBe("forming");
  expect(roomForResume({ stage: "plan_generation", openTool: "forming", widthTier: "half", reference: [], updatedAtTurn: 0 } as any)).toBe("forming");
});
```

- [ ] **Step 2: Implement.** `openToolToRoom`: add `case "forming": return "forming";`. `roomForResume`: since `forming` is now a real openTool, the existing `if (state.openTool !== "plan" && state.openTool !== "chat") return openToolToRoom(state.openTool);` already routes `forming`→forming — verify, and keep the stage-heuristic only for `plan`/`chat`. Confirm the `WidthTier` client type already includes what's needed (no change).
- [ ] **Step 3:** vitest + tsc green. The client `coach()` return type (`OrchestratorReply`) now carries `question` — no client change needed beyond consuming it (T7). **Step 4: Commit** `feat(web): forming openTool resume mapping (P2b)`.

---

## Task 6: web — remove 生成计划 button + regen modal; refresh plan after a turn

**Files:** Modify `apps/web/src/workspace/blocks/PlanBlock.tsx`, `apps/web/src/workspace/WorkspaceContainer.tsx`; Tests as needed.

**Rationale:** 印记 now triggers generation via `generate_plan`. The button + `RegenConfirm` modal are Tier-2 chrome removed here. After a coach turn the container refreshes plan items so a just-generated plan appears.

- [ ] **Step 1: Remove the `生成项目计划` button + its hint/covered copy + the `RegenConfirm` modal** from PlanBlock (grep `生成项目计划`, `RegenConfirm`, `确定重排`, `onGenerate`, `doGenerate`, `confirmRegen`, `generating`, `genError`, `seedBoard`). Remove the now-dead generation state/handlers (`onGenerate`/`doGenerate`/`generatePlan` import/`confirmRegen`/`generating`/`genError`) and the `onPlanGenerated` prop path IF it's now only used by the button (grep — the P2a `onPlanGenerated` flips room to 管理 after the button; with the button gone, 印记's directive opens 管理 instead, so remove `onPlanGenerated` and its WorkspaceContainer wiring). Keep `导出开题报告`, `让印记看看我的开题`, DimFields.
- [ ] **Step 2: Refresh plan after a coach turn.** In `WorkspaceContainer.sendStudioTurn`, after `applyStudioState(reply.directive)`, refresh the plan spine so a `generate_plan` turn shows: call `getPlan(pid).then(setPlanItems)` (guarded by `isActive()`), best-effort. (Cheap; also keeps the spine live.)
- [ ] **Step 3: Tests.** Update PlanBlock tests: assert `生成项目计划` is GONE; assert DimFields + 导出开题报告 + 让印记看看我的开题 remain. Add/adjust a WorkspaceContainer test: a coach turn refreshes `getPlan`. Full web suite green + tsc.
- [ ] **Step 4: Commit** `strip(studio): remove 生成计划 button/regen-modal — 印记 triggers via generate_plan (P2b)`.

---

## Task 7: web — propose_question confirm chip → createLead

**Files:** Modify `apps/web/src/studio/ai/StudioChatContext.tsx`, `apps/web/src/studio/ai/StudioCoachChat.tsx`, `apps/web/src/workspace/WorkspaceContainer.tsx`; add client `createLead` import; Tests.

**Interfaces — Produces:** `pendingQuestion: QuestionProposal | null`, `confirmQuestion()`, `dismissQuestion()` on `StudioChatValue`; a question confirm-chip in `StudioCoachChat` (mirror the note chip); confirming calls `createLead(projectId, text)` then bumps an exploration-refresh nonce.

- [ ] **Step 1: Extend `StudioChatContext`** with `pendingQuestion`/`confirmQuestion`/`dismissQuestion` (mirror `pendingNote`/`confirmNote`/`dismissNote`).
- [ ] **Step 2: WorkspaceContainer** — add `pendingQuestion` state (cleared at turn start + on project switch, like `pendingNote`); in `sendStudioTurn` set `setPendingQuestion(reply.question)`. Add `explorationRefreshNonce` state (number). `confirmQuestion`: read `activeProjectIdRef`, `createLead(pid, pendingQuestion.text)`, then `setExplorationRefreshNonce(n => n+1)` and clear the chip; on failure restore the chip (`setPendingQuestion(cur => cur ?? q)`), same resilience as `confirmNote`. Pass `pendingQuestion`/`confirmQuestion`/`dismissQuestion` into `chatValue`, and thread `explorationRefreshNonce` to the reading/exploration surface (T8).
- [ ] **Step 3: StudioCoachChat** — render a question confirm-chip when `pendingQuestion` (mirror the NoteConfirmChip; button e.g. `加入探索图谱`). Use a SOLID border token (design gotcha).
- [ ] **Step 4: Tests.** A reply with `question` renders the chip; confirming calls `createLead` with the text and clears the chip. Full suite + tsc green.
- [ ] **Step 5: Commit** `feat(studio): 印记 propose_question confirm chip → createLead (P2b)`.

---

## Task 8: web — strip exploration question boxes; consume the refresh nonce

**Files:** Modify `apps/web/src/workspace/blocks/exploration/ExplorationView.tsx` (and its parent `ReadingBlock.tsx` to thread the nonce); Tests.

- [ ] **Step 1: Remove** the root-question input + `记下问题` button (`createQuestion`), `从笔记新建问题` + note-picker (`createFromNote`, `notePickerOpen`, `notesWithText`), and the `用我的研究问题开始` seed button (`drivingQuestion`). Remove the now-dead state (`newQuestion`, `creatingQuestion`, `notePickerOpen`) and the `createLead` import IF unused after (grep — it may now only be used by the container's `confirmQuestion`, not here). Leave `references`, `refresh`, `roots`, `WarrenMap`, dig/adopt/edge logic intact. Replace the empty-state that used the seed with a NEUTRAL empty state (印记 proposes questions in the chat now).
- [ ] **Step 2: Consume the refresh nonce.** Thread `explorationRefreshNonce` (container → ReadingBlock → ExplorationView) as a prop; in ExplorationView, re-run its `refresh()` in a `useEffect` keyed on the nonce so a just-confirmed 印记 question appears on the graph without a manual reload.
- [ ] **Step 3: Tests.** Assert the three manual question affordances are gone; assert a nonce bump triggers a refresh (mock the leads fetch, bump the prop, expect a re-fetch). Full suite + tsc green.
- [ ] **Step 4: Commit** `strip(studio): remove manual exploration question boxes — 印记 proposes questions (P2b)`.

---

## Self-Review Checklist (after all tasks)

- **Spec §0.5/§8 coverage:** forming openTool (T1/T2/T5) · generate_plan (T1/T3/T4/T6) · propose_question (T1/T3/T4/T7/T8). 印记 now DOES what the two killed buttons did.
- **Contract parity:** every new reply field (`question`) present in BOTH `orchestratorReplyDTO` construction sites + the Zod contract; nullable emits `null`.
- **No silent tool drop:** `generate_plan`/`propose_question` in allowlist + prompt + validToolArgs + accessor.
- **Tier-2 now truly replaced:** `生成计划` button gone AND 印记 can generate; question boxes gone AND 印记 can propose. No dead path (a project can still get a plan + questions).
- **Metering:** `generate_plan` reuses `regeneratePlan` → `generatePlanItems` already meters `plan_gen`; no new unmetered LLM call. `propose_question` makes NO LLM call (it's the orchestrator's own output).
- **Green:** `go build/vet` + `go test ./internal/agent/` + the docker `internal/api` suite (foreground, 2 pre-existing failures only) + web tsc/vitest.
- **Live-verify (with P2a):** a fresh project — 印记 proposes notes/questions, generates the plan when the dims are ready (no button), opens 管理; reading shows 印记-proposed question chips that land on the graph. Then merge + deploy (**migration? NO** — P2b adds no schema; frontend + Go only) → done.
