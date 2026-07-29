# S5 · Review finalization — AI-interaction retrospective — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development. Steps use checkbox (`- [ ]`).

**Goal:** Give 回顾 a split-hybrid **AI-interaction retrospective** — the AI assembles the objective interaction record (events + llm_calls, un-forgeable) and seeds a draft; the student authors an honest AI-use statement (used-for / not-used-for); it feeds the assessment sub-agent. Plus a defense-readiness coach pane in the review room.

**Architecture:** Additive migration 0042 (`project_ai_use`, student-owned). Objective record = deterministic server assembly. Draft = one metered mid-tier seed (S2's `takeaway-draft` shape). Statement persisted no-spend + `ai_use_written` event. Fed to the assessor via a new `AssessmentInput.AIUse` field. Review conversation reuses `/coach scope="reflection"`.

**Tech Stack:** Go · React+Vite+TS+Tailwind · Zod. Spec: `docs/2026-07-29-s5-review-retrospective-spec.md`. Clone the S2 takeaway split-hybrid (reading_takeaway.go / agent/reading_takeaway.go).

## Global Constraints

- **Client NEVER calls a model.** All LLM via gateway; meter every COMPLETED call (`resolved.Provider != "" && cerr == nil`) with tier+tokens+Purpose. Empty record → ZERO provider calls, ZERO llm_call rows.
- **AI 克制 — the AI NEVER writes the reflection.** It ASSEMBLES the objective record (factual, read-only) and SEEDS a draft; the student AUTHORS used_for/not_used_for. Enforced at the type level: the seed's model-parse target carries ONLY `{used_for, not_used_for}`, never the objective record (computed server-side).
- **过程即数据.** `POST /ai-use` emits an `ai_use_written` event. The objective record includes the ABSENCES (no ghostwriting, no score prediction) as asserted facts, not guesses.
- **camelCase on the wire.** Model-parse structs may stay snake internally.
- **A failed seed compose never 500s** — return the assembled record with an empty draft.
- **New card = new JSON** (n/a here — no new cards).

---

### Task 1: Migration 0042 — `project_ai_use`

**Files:** Create `apps/api/internal/store/migrations/0042_project_ai_use.sql`

- [ ] **Step 1: Write the migration**
```sql
-- +goose Up
-- S5 · the student's AI-use statement (回顾 · 复盘我与 AI 的互动). Student-owned,
-- updatable (re-review supersedes) — NOT first-open-wins. The objective
-- interaction record is recomputed live from events/llm_calls, never stored.
CREATE TABLE project_ai_use (
    project_id   uuid PRIMARY KEY REFERENCES project(id) ON DELETE CASCADE,
    used_for     text NOT NULL DEFAULT '',
    not_used_for text NOT NULL DEFAULT '',
    updated_at   timestamptz NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE project_ai_use;
```
- [ ] **Step 2: Verify** it's the highest-numbered migration (0042), goose Up/Down, additive.
- [ ] **Step 3: Commit** `git commit -m "feat(s5): migration 0042 project_ai_use"`

---

### Task 2: sqlc queries

**Files:** Modify `apps/api/internal/store/queries/workspace.sql`; generated `sqlc/*.sql.go`.

**Interfaces produced:** `GetProjectAIUse(ctx, projectID) (ProjectAiUse, error)`; `UpsertProjectAIUse(ctx, {ProjectID, UsedFor, NotUsedFor})`. Confirm `ListLLMCallsByProject` already exists (llm_usage.sql) — read its row shape (Purpose, tokens).

- [ ] **Step 1: Add queries** (mirror `GetProjectReflection`/`UpsertProjectReflection` in workspace.sql):
```sql
-- name: GetProjectAIUse :one
SELECT * FROM project_ai_use WHERE project_id = $1;

-- name: UpsertProjectAIUse :exec
INSERT INTO project_ai_use (project_id, used_for, not_used_for, updated_at)
VALUES ($1, $2, $3, now())
ON CONFLICT (project_id) DO UPDATE
  SET used_for = EXCLUDED.used_for, not_used_for = EXCLUDED.not_used_for, updated_at = now();
```
- [ ] **Step 2: Regenerate** `cd apps/api && CGO_ENABLED=0 make sqlc` → PASS; `go build ./...` clean.
- [ ] **Step 3: Commit** `git commit -m "feat(s5): sqlc — project_ai_use get/upsert"`

---

### Task 3: Contracts

**Files:** Create `packages/contracts/src/aiUse.ts`; barrel in `index.ts`; Test `packages/contracts/test/aiUse.test.ts`.

**Produces:** `AIUseStatement {usedFor, notUsedFor}`; `AIUseRecord {coachTurns, cardsProposed, cardsAccepted, cardsDismissed, sourcesOpened, llmCallsByPurpose: Record<string,number>, ghostwroteEssay, predictedScore}`; `AIUseDraft {record: AIUseRecord, draft: AIUseStatement}`.

- [ ] **Step 1: Failing test**
```ts
import { describe, it, expect } from "vitest";
import { AIUseStatement, AIUseDraft } from "../src/aiUse";
describe("aiUse", () => {
  it("parses a statement", () => {
    expect(AIUseStatement.parse({ usedFor: "溯源提问", notUsedFor: "代写正文" }).usedFor).toBe("溯源提问");
  });
  it("parses a draft with an objective record", () => {
    const d = AIUseDraft.parse({
      record: { coachTurns: 12, cardsProposed: 2, cardsAccepted: 1, cardsDismissed: 1, sourcesOpened: 5, llmCallsByPurpose: { coach: 12, classify: 2 }, ghostwroteEssay: false, predictedScore: false },
      draft: { usedFor: "x", notUsedFor: "y" },
    });
    expect(d.record.ghostwroteEssay).toBe(false);
    expect(d.record.llmCallsByPurpose.coach).toBe(12);
  });
});
```
- [ ] **Step 2: Run — fail.** `cd packages/contracts && npx vitest run test/aiUse.test.ts`
- [ ] **Step 3: Implement** `aiUse.ts`:
```ts
import { z } from "zod";
export const AIUseStatement = z.object({ usedFor: z.string(), notUsedFor: z.string() });
export type AIUseStatement = z.infer<typeof AIUseStatement>;
export const AIUseRecord = z.object({
  coachTurns: z.number().int().nonnegative(),
  cardsProposed: z.number().int().nonnegative(),
  cardsAccepted: z.number().int().nonnegative(),
  cardsDismissed: z.number().int().nonnegative(),
  sourcesOpened: z.number().int().nonnegative(),
  llmCallsByPurpose: z.record(z.string(), z.number().int().nonnegative()),
  ghostwroteEssay: z.boolean(),
  predictedScore: z.boolean(),
});
export type AIUseRecord = z.infer<typeof AIUseRecord>;
export const AIUseDraft = z.object({ record: AIUseRecord, draft: AIUseStatement });
export type AIUseDraft = z.infer<typeof AIUseDraft>;
```
Add `export * from "./aiUse";` to index.ts.
- [ ] **Step 4: Run — pass.** **Step 5: Commit** `git commit -m "feat(s5): contracts — AIUseStatement/AIUseRecord/AIUseDraft"`

---

### Task 4: Objective interaction record — deterministic assembler

**Files:** Create `apps/api/internal/api/ai_use_record.go`; Test `apps/api/internal/api/ai_use_record_test.go`.

**Consumes:** `ListLLMCallsByProject` (read its row shape first); a project event scan (reuse whatever `eventDigestsFromProject`/the assessment path uses — read `assessment.go` around :354).
**Produces:** `type aiUseRecord struct { CoachTurns, CardsProposed, CardsAccepted, CardsDismissed, SourcesOpened int; LLMCallsByPurpose map[string]int; GhostwroteEssay, PredictedScore bool }`; `buildAIUseRecord(events []someEventShape, llmCalls []someLLMShape) aiUseRecord`; `hasInteractionRecord(r aiUseRecord) bool` = `r.CoachTurns>0 || r.CardsProposed>0 || len(r.LLMCallsByPurpose)>0`.

Counting rules (by event Type): `coach_turn`→CoachTurns; `coach_proposed`→CardsProposed; `card_activated`+`card_completed`→CardsAccepted (dedup by not double-counting — count `card_activated`); `coach_proposal_skipped`+`card_skipped`→CardsDismissed; `source_opened`→SourcesOpened. `GhostwroteEssay`/`PredictedScore` = always false (no such event exists — asserted, per spec D-B). `LLMCallsByPurpose` from the llm_call rows grouped by Purpose.

- [ ] **Step 1: Failing test** — drive `buildAIUseRecord` with synthetic event + llm rows (pure function, NO DB):
```go
func TestBuildAIUseRecord_CountsAndAbsences(t *testing.T) {
  rec := buildAIUseRecord(
    []eventRow{{Type:"coach_turn"},{Type:"coach_turn"},{Type:"coach_proposed"},{Type:"card_activated"},{Type:"coach_proposal_skipped"},{Type:"source_opened"}},
    []llmRow{{Purpose:"coach"},{Purpose:"coach"},{Purpose:"classify"}},
  )
  // assert CoachTurns==2, CardsProposed==1, CardsAccepted==1, CardsDismissed==1, SourcesOpened==1,
  // LLMCallsByPurpose["coach"]==2, ghostwrote/predicted == false, hasInteractionRecord==true
}
func TestBuildAIUseRecord_EmptyHasNoRecord(t *testing.T) {
  if hasInteractionRecord(buildAIUseRecord(nil,nil)) { t.Fatal("empty must have no record") }
}
```
(Define the exact `eventRow`/`llmRow` local shapes to whatever the real sqlc rows are — read them first; the assembler should take the real sqlc row slices so no adapter is needed.)
- [ ] **Step 2–4:** run-fail → implement → run-pass (`go test ./internal/api/ -run TestBuildAIUseRecord`).
- [ ] **Step 5: Commit** `git commit -m "feat(s5): deterministic AI-use objective record (counts + absences)"`

---

### Task 5: `agent.ComposeAIUseSeed` — mid-tier draft seed

**Files:** Create `apps/api/internal/agent/ai_use.go`; Test `apps/api/internal/agent/ai_use_test.go`.

**Consumes:** the gateway pattern from `agent/digest.go` / `agent/exploration_guide.go` — read one; mirror `gateway.Collect` + `stripFences` + `gateway.ChatUsage`.
**Produces:** `type AIUseRecordView struct { CoachTurns, CardsProposed, CardsAccepted, CardsDismissed, SourcesOpened int; LLMCallsByPurpose map[string]int }`; `ComposeAIUseSeed(ctx, prov, r, rec AIUseRecordView) (usedFor, notUsedFor string, usage gateway.ChatUsage, err error)`. Empty record (`CoachTurns==0 && CardsProposed==0 && len(LLMCallsByPurpose)==0`) → `("","",zero,nil)` NO provider call. Model-parse target = ONLY `{used_for, not_used_for}`.

System prompt: seed a FIRST-PERSON honest self-audit of how the student used AI, from the objective record — used-for and not-used-for. Never inflate AI's role, never claim AI did the thinking, keep each to a sentence or two; it's a SEED the student rewrites.

- [ ] **Step 1: Failing test** (reuse `countingProvider`/`NewStubProvider` from exploration_guide_test.go):
```go
func TestComposeAIUseSeed_EmptyRecordNoCall(t *testing.T) {
  cp := &countingProvider{inner: gateway.NewStubProvider([]gateway.StreamEvent{{Kind:gateway.EventDone}})}
  u,n,_,err := ComposeAIUseSeed(context.Background(), cp, gateway.Resolved{Provider:"fake"}, AIUseRecordView{})
  if err!=nil||cp.calls!=0||u!=""||n!="" { t.Fatalf("empty→no call, got calls=%d",cp.calls) }
}
func TestComposeAIUseSeed_SeedsFromRecord(t *testing.T) {
  cp := &countingProvider{inner: gateway.NewStubProvider([]gateway.StreamEvent{
    {Kind:gateway.EventTextDelta, TextDelta:`{"used_for":"用 AI 澄清检索词、核对来源功能","not_used_for":"没有让 AI 代写正文或预测分数"}`},
    {Kind:gateway.EventDone}})}
  u,n,_,err := ComposeAIUseSeed(context.Background(), cp, gateway.Resolved{Provider:"fake"}, AIUseRecordView{CoachTurns:12, LLMCallsByPurpose:map[string]int{"coach":12}})
  if err!=nil||u==""||n==""||cp.calls!=1 { t.Fatalf("expected seeded draft, calls=%d",cp.calls) }
}
```
- [ ] **Step 2–4:** fail → implement → pass. **Step 5: Commit** `git commit -m "feat(s5): agent.ComposeAIUseSeed — mid-tier AI-use draft seed (empty=no-spend)"`

---

### Task 6: `GET /ai-use-draft` + `GET /ai-use`

**Files:** Create `apps/api/internal/api/ai_use.go` (handlers); Modify `api.go` (routes, protected); Test `apps/api/internal/api/ai_use_test.go`.

**Consumes:** Task 2 sqlc, Task 4 `buildAIUseRecord`, Task 5 `ComposeAIUseSeed`, `a.d.ChatResolver`, `RecordLLMCall` (see exploration.go:378-387).
**Produces:** `GET /projects/{id}/ai-use-draft` → `{record, draft:{usedFor,notUsedFor}}` (assemble record no-spend; if `hasInteractionRecord` and NO saved statement, seed via one mid-tier call metered `Purpose="ai_use_retrospective"` on completed-call only; if a saved statement exists, return IT as the draft — no spend); `GET /projects/{id}/ai-use` → the stored statement (zero doc if none), no spend.

- [ ] **Step 1: Failing test** (testcontainers):
```go
func TestGetAIUseDraft_SeedsFromRecordAndMeters(t *testing.T){ /* seed project has events (0018) + post a coach turn → hasInteractionRecord; assert 200, record.coachTurns>0, draft present, exactly one llm_call Purpose="ai_use_retrospective" */ }
func TestGetAIUseDraft_EmptyRecordNoSpend(t *testing.T){ /* a project with no interaction → record all-zero, draft empty, ZERO ai_use_retrospective calls */ }
func TestGetAIUseDraft_ReturnsSavedStatement(t *testing.T){ /* after POST /ai-use, GET draft returns the saved text as draft, NO new spend */ }
```
(Use the api harness — `newAPITestPool`, `signInSeed`, `seedProjectID`, `momentReplyProvider`/`fakeProvider`, `countLLMCallsByPurpose`.)
- [ ] **Step 2–4:** fail → implement → pass. **Step 5: Commit** `git commit -m "feat(s5): GET /ai-use-draft (assemble+seed, metered) + GET /ai-use"`

---

### Task 7: `POST /ai-use` — persist statement

**Files:** add to `ai_use.go` + route; Test in `ai_use_test.go`.

**Produces:** `POST /projects/{id}/ai-use` body `{usedFor, notUsedFor}` → upsert; **no spend**; emit `ai_use_written` event; return the stored statement. IDOR via `loadOwnedProject`.

- [ ] **Step 1: Failing test**
```go
func TestPostAIUse_PersistsAndLogsNoSpend(t *testing.T){ /* POST → 200; GET /ai-use returns it; ai_use_written event count==1; NO llm_call added */ }
func TestPostAIUse_NonOwnedIs404(t *testing.T){ /* foreign project → 404 */ }
```
- [ ] **Step 2–4:** fail → implement → pass. **Step 5: Commit** `git commit -m "feat(s5): POST /ai-use — persist statement + ai_use_written event (no spend)"`

---

### Task 8: Feed the assessor

**Files:** Modify `apps/api/internal/agent/assess_input.go` (+ field), `apps/api/internal/agent/assess_report_prompt.go` (render), `apps/api/internal/api/assessment.go` (`buildAssessmentInputFromProject`); Test `apps/api/internal/agent/assess_report_prompt_test.go` (or assess_input_test.go).

**Produces:** `AssessmentInput.AIUse AIUseForAssessment{UsedFor, NotUsedFor, RecordLine string}` (new type in assess_input.go); `BuildAssessmentInput` gains it (append param OR set post-build — read the signature; append is cleaner). `buildAssessmentInputFromProject` loads `project_ai_use` + `buildAIUseRecord` → sets the field. `assessReportUserInput` renders a "学生的 AI 使用自述 + 客观交互记录" block when non-empty.

- [ ] **Step 1: Failing test** — assert the rendered assessor user-input (or the `AssessmentInput`) includes the AI-use statement text:
```go
func TestAssessReportInput_IncludesAIUse(t *testing.T){ /* build an AssessmentInput with AIUse{UsedFor:"溯源提问", NotUsedFor:"代写正文", RecordLine:"12 轮对话"} → assessReportUserInput contains "溯源提问" and "代写正文" */ }
```
- [ ] **Step 2–4:** fail → implement → pass (`go test ./internal/agent/ -run TestAssessReportInput`).
- [ ] **Step 5: Commit** `git commit -m "feat(s5): feed the AI-use statement + objective record into the assessment sub-agent"`

---

### Task 9: Web — AI-use retrospective panel in ReviewBlock

**Files:** Modify `apps/web/src/workspace/api/workspace.ts` (clients); Modify `apps/web/src/workspace/blocks/ReviewBlock.tsx`; Test `apps/web/test/workspace/blocks/ReviewBlock.test.tsx` (create/extend).

**Produces:** `getAIUseDraft(id) → AIUseDraft` · `postAIUse(id, statement)` · `getAIUse(id)`. Panel: objective record read-only line ("印记陪你走的这一程：N 轮对话 · 提议 M 张卡（用了 K、跳过 L）· 打开 P 个来源 · 没有替你写正文、没有替你预测分数") + two editable fields seeded from `draft` → 保存. Honest copy (student's statement, AI only seeded).

- [ ] **Step 1: Failing test** (mock `@/workspace/api/workspace` getAIUseDraft/postAIUse):
```tsx
it("renders the objective record and seeded fields, saves on 保存", async () => {
  // mock getAIUseDraft → {record:{coachTurns:12,...,ghostwroteEssay:false}, draft:{usedFor:"溯源",notUsedFor:"代写"}}
  // render, assert "12" and "没有替你写正文" appear and the two fields carry the seed; edit + click 保存 → postAIUse called with edited text
});
```
- [ ] **Step 2–4:** fail → implement → pass + `npx tsc --noEmit`. **Step 5: Commit** `git commit -m "feat(s5): web — AI-use retrospective panel (objective record + student-authored statement)"`

---

### Task 10: Web — defense-readiness coach pane in ReviewBlock

**Files:** Modify `ReviewBlock.tsx` (add a chat rail); Test extend `ReviewBlock.test.tsx`.

**Produces:** a coach rail calling `coach(projectId, "reflection", text)` (defense-readiness conversation) with history via `getCoachHistory("reflection")` — mirrors WritingBlock's rail. NOTE: web `CoachScope` currently lacks `"reflection"` — extend the union to include it (backend already labels it 回顾).

- [ ] **Step 1: Failing test**
```tsx
it("sends a reflection coach turn", async () => {
  // mock coach() → {reply:"你先自己答——你的结论回答了原题吗？", proposal:null}
  // type + send → coach called with (projectId, "reflection", text); reply rendered
});
```
- [ ] **Step 2–4:** fail → implement → pass + tsc. **Step 5: Commit** `git commit -m "feat(s5): web — defense-readiness coach pane in the review room (scope=reflection)"`

---

## Final verification (whole-branch review gate)
Full suites (`packages/contracts` vitest, `apps/api` `go test ./... -timeout 600s`, `apps/web` vitest + tsc), then dispatch the opus whole-branch reviewer (`scripts/review-package $(git merge-base main HEAD) HEAD`). S2/S3/S4 each had a happy-path cross-file bug only the whole-branch review caught — likely candidates here: the objective-record double-count (card_activated vs card_completed), the seed-vs-saved-statement precedence in `GET /ai-use-draft`, or the `CoachScope` union / gate mismatch from Task 10.

## Self-review (author)
- **Coverage:** storage T1/T2; contracts T3; objective record T4; seed T5; draft/get T6; persist T7; assessor T8; web panel T9; web conversation T10. All spec §§1–4 mapped.
- **Type consistency:** `AIUseStatement{usedFor,notUsedFor}` identical across contracts (T3), Go DTO (T6/T7), web (T9). `AIUseRecord` fields identical T3↔T4↔T9.
- **克制 at type level:** T5's `ComposeAIUseSeed` parse target = only `{used_for,not_used_for}` — cannot forge the objective record (T4, server-computed). Flagged for the reviewer.
