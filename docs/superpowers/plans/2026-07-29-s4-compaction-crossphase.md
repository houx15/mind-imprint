# S4 · Compaction backstop + digest + cross-phase card proposing — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax.

**Goal:** Give the one continuous coach a size-threshold compaction backstop (fold oldest raw turns into a durable `conversation_digest`), let it *propose* student cards across phases via the 克制 ladder (opening on student confirm), and persist the rabbit-hole card's envelope into the process tree.

**Architecture:** Additive migration 0041 (`conversation_digest`). Backstop = a best-effort routine at the tail of `postCoach` that composes overflow turns into the digest (mid-tier) then folds them by age. Proposing = `ProposeProjectCoachReply` returns an optional `*CardProposal` (reusing the existing moment classifier); `/coach` JSON gains `proposal?`; rooms render a dismissable confirm chip that opens the card via the existing summon path. rabbit-hole card = a project-scoped create+submit endpoint that mints a top-level node via the existing `CompleteCard`/`CommitCardMint`.

**Tech Stack:** Go (`net/http`, pgx, sqlc, goose) · React+Vite+TS+Tailwind · Zod contracts. Spec: `docs/2026-07-29-s4-compaction-crossphase-spec.md`.

## Global Constraints

- **Client NEVER calls a model.** All LLM via the backend gateway; every completed call metered with tier + tokens + `Purpose`. API keys server-side only.
- **Meter ONLY a completed call:** guard `resolved.Provider != "" && cerr == nil` (S3's phantom-row fix). Skip the whole resolve/compose/meter block when there is nothing to do (empty overflow / no card candidate) → ZERO provider calls, ZERO llm_call rows.
- **AI 克制 — never concludes for the student.** Proposing a card is an offer; **opening is the student's tap** (铁律: triggering automatic, opening confirmed). Dismissable, never modal. No slot-machine mechanics.
- **过程即数据.** A proposal offered and a proposal skipped are both recorded (`coach_proposed` event). The rabbit-hole card's submit copy must be honest — it now persists.
- **Digest write precedes fold.** Never mark a turn `folded_at` before its content is durable in `conversation_digest`.
- **camelCase on the wire** everywhere (Go DTO json tags ↔ Zod). Model-parse structs may stay snake internally.
- **New card = new JSON, not renderer code.** rabbit-hole.json already exists; wire it, don't add a renderer primitive.
- **Envelope outer shape validated at the boundary** (status enum, id, `field_values` object, `event_trace` array); inner truth stays in `packages/contracts` Zod.
- **A failed compose/propose never 500s and never breaks the coach turn** — warn + return the reply.

---

### Task 1: Migration 0041 — `conversation_digest`

**Files:**
- Create: `apps/api/internal/store/migrations/0041_conversation_digest.sql`

**Interfaces:**
- Produces: table `conversation_digest(project_id uuid PK, prose text, turns_folded int, model text, tier text, created_at, updated_at)`.

- [ ] **Step 1: Write the migration**

```sql
-- +goose Up
CREATE TABLE conversation_digest (
  project_id   uuid PRIMARY KEY REFERENCES project(id) ON DELETE CASCADE,
  prose        text NOT NULL,
  turns_folded int  NOT NULL DEFAULT 0,
  model        text,
  tier         text,
  created_at   timestamptz NOT NULL DEFAULT now(),
  updated_at   timestamptz NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE conversation_digest;
```

- [ ] **Step 2: Verify it applies**

Run (needs Docker for the api package, but the migration itself can be checked by goose against a scratch DB, or deferred to Task 2's `make sqlc`): confirm the file is the highest-numbered migration and follows the 0038–0040 style (goose Up/Down markers, additive).
Expected: file present; `ls apps/api/internal/store/migrations/` shows `0041_conversation_digest.sql` as newest.

- [ ] **Step 3: Commit**

```bash
git add apps/api/internal/store/migrations/0041_conversation_digest.sql
git commit -m "feat(s4): migration 0041 conversation_digest"
```

---

### Task 2: sqlc queries — oldest-first fold + digest upsert

**Files:**
- Modify: `apps/api/internal/store/queries/chat.sql` (add queries)
- Modify: `apps/api/internal/store/queries/workspace.sql` (digest queries — or chat.sql; keep with chat)
- Generated: `apps/api/internal/store/sqlc/*.sql.go` (via `make sqlc`)

**Interfaces:**
- Produces (sqlc-generated Go):
  - `SelectOldestActiveChatMessages(ctx, arg{ProjectID uuid, KeepLast int32}) ([]ChatMessage, error)` — non-folded turns of the project's thread, oldest-first, EXCLUDING the newest `KeepLast`.
  - `FoldChatMessagesByID(ctx, ids []uuid.UUID) error`
  - `GetConversationDigest(ctx, projectID) (ConversationDigest, error)`
  - `UpsertConversationDigest(ctx, arg{ProjectID, Prose, TurnsFolded, Model, Tier}) error`

- [ ] **Step 1: Add the queries**

In `apps/api/internal/store/queries/chat.sql` (mirror the existing `ListActiveChatMessagesByProject` join on `chat_thread.seeded_project_id`):

```sql
-- name: SelectOldestActiveChatMessages :many
SELECT cm.* FROM chat_message cm
JOIN chat_thread ct ON ct.id = cm.thread_id
WHERE ct.seeded_project_id = $1 AND cm.folded_at IS NULL
ORDER BY cm.created_at ASC
LIMIT GREATEST(
  (SELECT count(*) FROM chat_message cm2
   JOIN chat_thread ct2 ON ct2.id = cm2.thread_id
   WHERE ct2.seeded_project_id = $1 AND cm2.folded_at IS NULL) - $2, 0);

-- name: FoldChatMessagesByID :exec
UPDATE chat_message SET folded_at = now() WHERE id = ANY($1::uuid[]);
```

In `apps/api/internal/store/queries/workspace.sql` (or chat.sql):

```sql
-- name: GetConversationDigest :one
SELECT * FROM conversation_digest WHERE project_id = $1;

-- name: UpsertConversationDigest :exec
INSERT INTO conversation_digest (project_id, prose, turns_folded, model, tier, updated_at)
VALUES ($1, $2, $3, $4, $5, now())
ON CONFLICT (project_id) DO UPDATE
  SET prose = EXCLUDED.prose, turns_folded = EXCLUDED.turns_folded,
      model = EXCLUDED.model, tier = EXCLUDED.tier, updated_at = now();
```

- [ ] **Step 2: Regenerate**

Run: `cd apps/api && CGO_ENABLED=0 make sqlc` (CGO_ENABLED=0 required on macOS per house notes).
Expected: PASS; new methods appear in `apps/api/internal/store/sqlc/`; `go build ./...` clean.

- [ ] **Step 3: Build**

Run: `cd apps/api && go build ./...`
Expected: no errors.

- [ ] **Step 4: Commit**

```bash
git add apps/api/internal/store/queries apps/api/internal/store/sqlc
git commit -m "feat(s4): sqlc — oldest-first fold + conversation_digest upsert"
```

---

### Task 3: Contracts — `CardProposal`, `ConversationDigest`, coach response `proposal?`

**Files:**
- Create: `packages/contracts/src/proposal.ts`
- Modify: `packages/contracts/src/index.ts` (barrel export)
- Test: `packages/contracts/test/proposal.test.ts`

**Interfaces:**
- Produces: `CardProposal {cardId, reason, nudgeText}`; `ConversationDigest {prose, turnsFolded}`; `CoachReply {reply, proposal?: CardProposal | null}`.

- [ ] **Step 1: Write the failing test**

```ts
import { describe, it, expect } from "vitest";
import { CardProposal, CoachReply } from "../src/proposal";

describe("CardProposal / CoachReply", () => {
  it("parses a summon proposal", () => {
    const p = CardProposal.parse({ cardId: "sift", reason: "横向核查这条来源", nudgeText: "要不要打开 SIFT？" });
    expect(p.cardId).toBe("sift");
  });
  it("coach reply allows null / absent proposal", () => {
    expect(CoachReply.parse({ reply: "你怎么看？" }).proposal ?? null).toBeNull();
    expect(CoachReply.parse({ reply: "x", proposal: null }).proposal).toBeNull();
  });
  it("rejects a proposal missing cardId", () => {
    expect(() => CardProposal.parse({ reason: "x", nudgeText: "y" })).toThrow();
  });
});
```

- [ ] **Step 2: Run — verify it fails**

Run: `cd packages/contracts && npx vitest run test/proposal.test.ts`
Expected: FAIL (module `../src/proposal` not found).

- [ ] **Step 3: Implement**

`packages/contracts/src/proposal.ts`:
```ts
import { z } from "zod";

export const CardProposal = z.object({
  cardId: z.string().min(1),
  reason: z.string(),
  nudgeText: z.string(),
});
export type CardProposal = z.infer<typeof CardProposal>;

export const ConversationDigest = z.object({
  prose: z.string(),
  turnsFolded: z.number().int().nonnegative(),
});
export type ConversationDigest = z.infer<typeof ConversationDigest>;

export const CoachReply = z.object({
  reply: z.string(),
  proposal: CardProposal.nullish(),
});
export type CoachReply = z.infer<typeof CoachReply>;
```

Add to `packages/contracts/src/index.ts`:
```ts
export * from "./proposal";
```

- [ ] **Step 4: Run — verify it passes**

Run: `cd packages/contracts && npx vitest run test/proposal.test.ts`
Expected: PASS (3 tests).

- [ ] **Step 5: Commit**

```bash
git add packages/contracts/src/proposal.ts packages/contracts/src/index.ts packages/contracts/test/proposal.test.ts
git commit -m "feat(s4): contracts — CardProposal, ConversationDigest, CoachReply.proposal?"
```

---

### Task 4: `agent.ComposeDigestMerge` — isolated mid-tier compose

**Files:**
- Create: `apps/api/internal/agent/digest.go`
- Test: `apps/api/internal/agent/digest_test.go`

**Interfaces:**
- Consumes: the gateway/provider pattern from `ComposeExplorationGuide` (`apps/api/internal/agent/exploration_guide.go`) — read it first; mirror its provider call, `stripFences`, `ChatUsage` return, and empty-input early-return-with-zero-calls discipline.
- Produces: `type DigestTurn struct { Role, Content string }`; `func ComposeDigestMerge(ctx context.Context, prov gateway.Provider, r gateway.Resolved, priorDigest string, turns []DigestTurn) (string, gateway.ChatUsage, error)`. Empty `turns` → `("", ChatUsage{}, nil)` with NO provider call.

- [ ] **Step 1: Write the failing test**

```go
package agent

import (
	"context"
	"testing"
)

func TestComposeDigestMerge_EmptyTurnsNoCall(t *testing.T) {
	cp := &countingProvider{} // reuse the counting provider from exploration_guide_test.go
	prose, usage, err := ComposeDigestMerge(context.Background(), cp, testResolved(), "prior", nil)
	if err != nil { t.Fatalf("err: %v", err) }
	if cp.calls != 0 { t.Fatalf("expected 0 provider calls, got %d", cp.calls) }
	if prose != "" { t.Fatalf("expected empty prose, got %q", prose) }
	_ = usage
}

func TestComposeDigestMerge_MergesTurns(t *testing.T) {
	cp := &countingProvider{reply: "学生确认 NASA 数据是 China+India combined，不是 China alone。"}
	prose, _, err := ComposeDigestMerge(context.Background(), cp, testResolved(),
		"", []DigestTurn{{Role: "user", Content: "NASA 说 greening"}, {Role: "assistant", Content: "先追上游来源"}})
	if err != nil { t.Fatalf("err: %v", err) }
	if cp.calls != 1 { t.Fatalf("expected 1 call, got %d", cp.calls) }
	if prose == "" { t.Fatalf("expected non-empty digest prose") }
}
```
(If `countingProvider`/`testResolved` are named differently in `exploration_guide_test.go`, reuse whatever those tests use — read that file first and match the existing helpers.)

- [ ] **Step 2: Run — verify it fails**

Run: `cd apps/api && go test ./internal/agent/ -run TestComposeDigestMerge`
Expected: FAIL (undefined `ComposeDigestMerge`).

- [ ] **Step 3: Implement**

`apps/api/internal/agent/digest.go` — mirror `ComposeExplorationGuide`'s structure:
```go
package agent

import (
	"context"
	"fmt"
	"strings"

	"<module>/internal/gateway" // match the import path used in exploration_guide.go
)

type DigestTurn struct {
	Role    string
	Content string
}

const digestSystemPrompt = `你在为一个学生的研究项目维护一份「会话记忆」。把下面这些较早的对话轮次，压进已有的记忆里：
- 保留可复用的事实、学生自己的推理与决定、来源功能与边界；
- 丢掉寒暄与重复；不要臆造任何内容；
- 输出一段简洁的中文记忆，不要分点堆砌。`

func ComposeDigestMerge(ctx context.Context, prov gateway.Provider, r gateway.Resolved, priorDigest string, turns []DigestTurn) (string, gateway.ChatUsage, error) {
	if len(turns) == 0 {
		return "", gateway.ChatUsage{}, nil
	}
	var b strings.Builder
	if strings.TrimSpace(priorDigest) != "" {
		fmt.Fprintf(&b, "已有记忆：\n%s\n\n", priorDigest)
	}
	b.WriteString("较早的对话轮次：\n")
	for _, t := range turns {
		fmt.Fprintf(&b, "- [%s] %s\n", t.Role, t.Content)
	}
	// Build the same request shape ComposeExplorationGuide uses (system + user), call gateway.Collect via prov/r,
	// return stripFences(result.Content), result.Usage, err.
	// COPY the exact provider-call lines from exploration_guide.go — do not invent a new call shape.
	...
}
```
Read `exploration_guide.go` and replicate its exact `gateway.Collect`/provider invocation and usage extraction. Cap nothing (single paragraph out); apply `stripFences`.

- [ ] **Step 4: Run — verify it passes**

Run: `cd apps/api && go test ./internal/agent/ -run TestComposeDigestMerge`
Expected: PASS (2 tests).

- [ ] **Step 5: Commit**

```bash
git add apps/api/internal/agent/digest.go apps/api/internal/agent/digest_test.go
git commit -m "feat(s4): agent.ComposeDigestMerge — isolated mid-tier digest compose"
```

---

### Task 5: Compaction backstop routine + wire into `postCoach`

**Files:**
- Modify: `apps/api/internal/api/projectcoach.go` (add `maybeCompactBackstop`; constants)
- Modify: `apps/api/internal/api/coach.go` (call at tail of `postCoach`)
- Modify: `apps/api/internal/agent/agentstore.go` (adapter methods if `postCoach` reaches the store via the agent adapter — else call sqlc directly through `a.q`)
- Test: `apps/api/internal/api/coach_test.go` (or a new `compaction_test.go`)

**Interfaces:**
- Consumes: Task 2 sqlc (`SelectOldestActiveChatMessages`, `FoldChatMessagesByID`, `GetConversationDigest`, `UpsertConversationDigest`); Task 4 `ComposeDigestMerge`; `a.d.ChatResolver`, `RecordLLMCall` (see `exploration.go:378-387` for the exact metering call).
- Produces: `func (a *API) maybeCompactBackstop(ctx context.Context, projectID uuid.UUID)` — best-effort, no return; constants `digestRuneBudget`, `digestKeepLastN`.

- [ ] **Step 1: Write the failing test** (testcontainers)

```go
func TestMaybeCompactBackstop_FoldsOldestIntoDigest(t *testing.T) {
	// Arrange: seed a project + many coach turns whose total runes exceed digestRuneBudget.
	// Use the api test harness (same helpers as coach_test.go). Post enough /coach turns
	// (with a stub provider that returns short replies) to exceed the budget.
	// Assert:
	//  - conversation_digest row exists with turns_folded > 0
	//  - the oldest turns now have folded_at != NULL
	//  - the newest digestKeepLastN turns remain non-folded
	//  - exactly one llm_call row with Purpose="coach_compact" per compaction that ran
}

func TestMaybeCompactBackstop_UnderBudgetNoSpend(t *testing.T) {
	// A couple of short turns: assert NO conversation_digest row, NO folded_at set,
	// NO llm_call with Purpose="coach_compact".
}

func TestMaybeCompactBackstop_DigestBeforeFold(t *testing.T) {
	// With a provider stub that ERRORS on the compact call: assert NO turns folded and NO 500 —
	// the coach reply still returns 200. (Digest write precedes fold; failure folds nothing.)
}
```
Match the exact seed/post helpers already in `coach_test.go`.

- [ ] **Step 2: Run — verify it fails**

Run: `cd apps/api && go test ./internal/api/ -run TestMaybeCompactBackstop -timeout 600s`
Expected: FAIL (undefined `maybeCompactBackstop` / assertions unmet). (Docker required; ~340s.)

- [ ] **Step 3: Implement**

In `projectcoach.go`, beside `coachHistoryWindow`:
```go
const (
	digestRuneBudget = 6000 // fold when the active window exceeds this many runes
	digestKeepLastN  = 8    // always keep the newest N turns live
)
```
`maybeCompactBackstop`:
```go
func (a *API) maybeCompactBackstop(ctx context.Context, projectID uuid.UUID) {
	active, err := a.q.ListActiveChatMessagesByProject(ctx, projectID) // or the existing loader
	if err != nil { return }
	total := 0
	for _, m := range active { total += len([]rune(m.Content)) }
	if total <= digestRuneBudget { return } // under budget → no spend

	overflow, err := a.q.SelectOldestActiveChatMessages(ctx, sqlc.SelectOldestActiveChatMessagesParams{ProjectID: projectID, KeepLast: digestKeepLastN})
	if err != nil || len(overflow) == 0 { return }

	prior, _ := a.q.GetConversationDigest(ctx, projectID) // zero value if none
	turns := make([]agent.DigestTurn, 0, len(overflow))
	ids := make([]uuid.UUID, 0, len(overflow))
	for _, m := range overflow {
		turns = append(turns, agent.DigestTurn{Role: m.Role, Content: m.Content})
		ids = append(ids, m.ID)
	}
	resolved := a.d.ChatResolver(ctx)
	prose, usage, cerr := agent.ComposeDigestMerge(ctx, a.d.Provider, resolved, prior.Prose, turns)
	// meter ONLY a completed call
	if resolved.Provider != "" && cerr == nil {
		_ = a.store.RecordLLMCall(ctx, agent.LLMCallRow{ProjectID: projectID, Surface: "studio", Purpose: "coach_compact",
			Resolved: resolved, PromptTokens: usage.PromptTokens, CompletionTokens: usage.CompletionTokens})
	}
	if cerr != nil || strings.TrimSpace(prose) == "" { return } // compose failed → fold nothing
	// digest write PRECEDES fold
	if err := a.q.UpsertConversationDigest(ctx, sqlc.UpsertConversationDigestParams{
		ProjectID: projectID, Prose: prose, TurnsFolded: prior.TurnsFolded + int32(len(overflow)),
		Model: pgText(resolved.Model), Tier: pgText(resolved.Tier)}); err != nil { return }
	_ = a.q.FoldChatMessagesByID(ctx, ids)
}
```
(Adjust field/receiver names to the actual `API` struct — `a.q`, `a.d`, `a.store` per how `exploration.go`/`coach.go` access them; read those files. `pgText` = the existing helper for `pgtype.Text`.)

In `coach.go` `postCoach`, after the assistant turn is persisted and before writing the response (or in a deferred goroutine-free tail — keep it synchronous so tests are deterministic):
```go
a.maybeCompactBackstop(ctx, projectID)
```

- [ ] **Step 4: Run — verify it passes**

Run: `cd apps/api && go test ./internal/api/ -run TestMaybeCompactBackstop -timeout 600s`
Expected: PASS (3 tests).

- [ ] **Step 5: Commit**

```bash
git add apps/api/internal/api/projectcoach.go apps/api/internal/api/coach.go apps/api/internal/agent/agentstore.go
git commit -m "feat(s4): compaction backstop — fold oldest turns into conversation_digest (metered, digest-before-fold)"
```

---

### Task 6: Projection carries the digest

**Files:**
- Modify: `apps/api/internal/api/projectcoach.go` (`buildSpineProjection`)
- Test: `apps/api/internal/api/projectcoach_test.go` (via `BuildSpineProjectionForTest`, `export_test.go:72-81`)

**Interfaces:**
- Consumes: `GetConversationDigest`.
- Produces: `buildSpineProjection` output includes a leading `会话记忆：<prose>` block when a digest exists; omitted otherwise.

- [ ] **Step 1: Write the failing test**

```go
func TestSpineProjection_IncludesDigest(t *testing.T) {
	// Seed a project + upsert a conversation_digest with prose "早前：学生已溯源到 NASA Ames。"
	proj := // BuildSpineProjectionForTest(ctx, api, projectID)
	if !strings.Contains(proj, "会话记忆") || !strings.Contains(proj, "NASA Ames") {
		t.Fatalf("projection missing digest block:\n%s", proj)
	}
}

func TestSpineProjection_NoDigestNoBlock(t *testing.T) {
	// Project with no digest row: projection must NOT contain "会话记忆".
}
```

- [ ] **Step 2: Run — verify it fails**

Run: `cd apps/api && go test ./internal/api/ -run TestSpineProjection_IncludesDigest -timeout 600s`
Expected: FAIL.

- [ ] **Step 3: Implement**

Near the top of `buildSpineProjection` (before the 主题 block), best-effort:
```go
if d, err := a.q.GetConversationDigest(ctx, projectID); err == nil && strings.TrimSpace(d.Prose) != "" {
	fmt.Fprintf(&b, "会话记忆：%s\n", truncateRunes(d.Prose, 600))
}
```
(Use the file's existing builder variable and `truncateRunes` helper `projectcoach.go:50-57`.)

- [ ] **Step 4: Run — verify it passes**

Run: `cd apps/api && go test ./internal/api/ -run TestSpineProjection -timeout 600s`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add apps/api/internal/api/projectcoach.go apps/api/internal/api/projectcoach_test.go
git commit -m "feat(s4): spine projection carries conversation digest (会话记忆)"
```

---

### Task 7: Coach returns an optional `*CardProposal`

**Files:**
- Modify: `apps/api/internal/agent/project_coach.go` (`ProposeProjectCoachReply` signature + proposal decision)
- Create/modify: `apps/api/internal/agent/proposal.go` (the `CardProposal` type + decision fn) — or inline in project_coach.go
- Test: `apps/api/internal/agent/project_coach_test.go`

**Interfaces:**
- Consumes: the moment classifier primitives (`moment.go` `ClassifyMoment`/`EligibleMomentsScoped`, `chat_step.go:184-201` `momentCard`) — read them; reuse to map `{projection, activeSurface, studentText}` → an optional card id. `cards.ByID` for the card name/placement.
- Produces: `type CardProposal struct { CardID, Reason, NudgeText string }`; `ProposeProjectCoachReply(...) (reply string, proposal *CardProposal, usage gateway.ChatUsage, err error)`. Proposal is nil unless the ladder reaches `summon`. Respect a per-project propose cap and the active-surface placement (many-to-many; suppress reading-room-only cards outside the reading surface).

- [ ] **Step 1: Write the failing test**

```go
func TestProposeProjectCoachReply_SummonAttachesProposal(t *testing.T) {
	// Provider stub returns a reply; feed a student text + surface that the moment classifier maps to a card.
	reply, proposal, _, err := ProposeProjectCoachReply(ctx, stubProv, testResolved(), history, projection, "writing", studentTextThatTriggers)
	if err != nil { t.Fatal(err) }
	if reply == "" { t.Fatal("reply should always be present") }
	if proposal == nil || proposal.CardID == "" { t.Fatal("expected a card proposal at the summon rung") }
}

func TestProposeProjectCoachReply_RespondNoProposal(t *testing.T) {
	// Neutral student text (no moment): proposal must be nil, reply present.
	_, proposal, _, _ := ProposeProjectCoachReply(ctx, stubProv, testResolved(), history, projection, "writing", "谢谢")
	if proposal != nil { t.Fatalf("expected nil proposal, got %+v", proposal) }
}
```
(Match the existing `ProposeProjectCoachReply` test setup shape; if none exists, mirror `chat_coach`/`project_coach` call sites for building `history`/`projection`.)

- [ ] **Step 2: Run — verify it fails**

Run: `cd apps/api && go test ./internal/agent/ -run TestProposeProjectCoachReply`
Expected: FAIL (signature mismatch / undefined).

- [ ] **Step 3: Implement**

- Add `CardProposal` (agent pkg).
- Change `ProposeProjectCoachReply` to compute the reply as today, then compute the proposal:
```go
func proposeCardForCoach(projection, surface, studentText string) *CardProposal {
	m := ClassifyMoment(studentText, projection) // reuse existing classifier; read moment.go for the real signature
	if m == "" { return nil }
	cardID := momentCard[m] // reuse chat_step.go mapping
	if cardID == "" { return nil }
	if !cardFitsSurface(cardID, surface) { return nil } // placement many-to-many; suppress reading-only outside reading
	spec, ok := cards.ByID(cardID)
	if !ok { return nil }
	return &CardProposal{CardID: cardID, Reason: momentReason(m), NudgeText: "要不要打开「" + spec.Name + "」？"}
}
```
Wire `proposeCardForCoach` into `ProposeProjectCoachReply` (return its result). Keep proposal computation cheap/deterministic where the existing classifier is deterministic; if it spends, meter under `Purpose="coach_propose"` at the CALL SITE (Task 8) — this fn itself should not double-meter. Decide with the reference: if `ClassifyMoment` is a pure/deterministic function (no provider), NO extra spend and no metering needed. Read `moment.go` to confirm; the plan assumes it is deterministic (it underpins the existing silence-default loop's cheap path). If it requires a provider call, thread `prov, resolved` in and meter at the call site in Task 8.

- [ ] **Step 4: Run — verify it passes**

Run: `cd apps/api && go test ./internal/agent/ -run TestProposeProjectCoachReply`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add apps/api/internal/agent/project_coach.go apps/api/internal/agent/proposal.go apps/api/internal/agent/project_coach_test.go
git commit -m "feat(s4): coach returns optional CardProposal (克制 summon rung, reuses moment classifier)"
```

---

### Task 8: `postCoach` surfaces the proposal + records `coach_proposed`

**Files:**
- Modify: `apps/api/internal/api/coach.go` (`postCoach` response + event)
- Test: `apps/api/internal/api/coach_test.go`

**Interfaces:**
- Consumes: Task 7 `ProposeProjectCoachReply` (now returns proposal); `AppendEvent` (see `coach.go:132-139` `coach_turn`).
- Produces: `/coach` JSON response gains `proposal` (nullable, camelCase `{cardId, reason, nudgeText}`); a `coach_proposed` event is appended when proposal != nil (with the card id in the payload).

- [ ] **Step 1: Write the failing test**

```go
func TestPostCoach_ReturnsProposalAndRecordsEvent(t *testing.T) {
	// Post a /coach turn whose student text triggers a proposal.
	// Assert response JSON has proposal.cardId != "" and an event of type "coach_proposed" was appended.
}
func TestPostCoach_NoProposalNoEvent(t *testing.T) {
	// Neutral turn: response proposal is null/absent; no coach_proposed event.
}
```

- [ ] **Step 2: Run — verify it fails**

Run: `cd apps/api && go test ./internal/api/ -run TestPostCoach_ReturnsProposal -timeout 600s`
Expected: FAIL.

- [ ] **Step 3: Implement**

Update `postCoach`'s response struct:
```go
type coachProposalDTO struct {
	CardID    string `json:"cardId"`
	Reason    string `json:"reason"`
	NudgeText string `json:"nudgeText"`
}
type coachReplyDTO struct {
	Reply    string            `json:"reply"`
	Proposal *coachProposalDTO `json:"proposal,omitempty"`
}
```
Capture the new return value; when proposal != nil, set `resp.Proposal` and `AppendEvent(ctx, projectID, "coach_proposed", map[string]any{"cardId": proposal.CardID, "surface": scope})`. Meter `coach_propose` here IF (and only if) Task 7 determined the classifier spends (else no metering). Keep the existing `coach` metering + `coach_turn` event untouched.

- [ ] **Step 4: Run — verify it passes**

Run: `cd apps/api && go test ./internal/api/ -run TestPostCoach -timeout 600s`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add apps/api/internal/api/coach.go apps/api/internal/api/coach_test.go
git commit -m "feat(s4): /coach surfaces card proposal + records coach_proposed event"
```

---

### Task 9: rabbit-hole card → process-tree persistence

**Files:**
- Create: `apps/api/internal/api/exploration_card.go` (`postExplorationRabbitHole`)
- Modify: `apps/api/internal/api/api.go` (register `POST /projects/{id}/exploration/rabbit-hole`, protected)
- Test: `apps/api/internal/api/exploration_card_test.go`

**Interfaces:**
- Consumes: the project card submit machinery — read `submitProjectCard` (`projectcards.go:161`), `CreateCardInstance`, `SubmitProjectCardInstance`, `CompleteCard`→`CommitCardMint` (`loop.go:144-152`). `cards.ByID("rabbit-hole")`.
- Produces: `POST /projects/{id}/exploration/rabbit-hole` — body = a completed standard envelope (`{status, field_values, event_trace, ...}`); validates outer shape; creates the instance with `parent_node_id = NULL`, submits+completes, mints a top-level node. Entitlement-gated only if it spends (it should NOT spend — pure persistence; no gate needed beyond project ownership). Returns the created instance id + minted node id.

- [ ] **Step 1: Write the failing test**

```go
func TestPostExplorationRabbitHole_PersistsAndMintsNode(t *testing.T) {
	// Owned project. POST a completed rabbit-hole envelope (status=completed, field_values populated, event_trace=[]).
	// Assert 200; a card_instance row (card_id="rabbit-hole", parent_node_id NULL, status completed) exists;
	// studio.Project(...) projection now contains the minted node.
}
func TestPostExplorationRabbitHole_RejectsBadEnvelope(t *testing.T) {
	// Missing status / field_values not an object → 400, nothing persisted.
}
func TestPostExplorationRabbitHole_IDOR(t *testing.T) {
	// Another user's project → 404/403, nothing persisted.
}
```

- [ ] **Step 2: Run — verify it fails**

Run: `cd apps/api && go test ./internal/api/ -run TestPostExplorationRabbitHole -timeout 600s`
Expected: FAIL.

- [ ] **Step 3: Implement**

Mirror `submitProjectCard`'s validate→create→submit→complete flow, but without a material anchor (`parent_node_id = NULL`), for `card_id="rabbit-hole"`. Validate the envelope outer shape exactly as `submitProjectCard` does (status enum, `field_values` object, `event_trace` array). Call `CompleteCard`/`CommitCardMint` so the node is minted and appears in projections. If `CompleteCard`/`CommitCardMint` proves to REQUIRE a material/parent node (material-coupled) — STOP and report this as a plan risk (per spec D-E): the controller decides whether to (i) mint under the project structure root, or (ii) descope to card_instance-only persistence for this slice.

- [ ] **Step 4: Run — verify it passes**

Run: `cd apps/api && go test ./internal/api/ -run TestPostExplorationRabbitHole -timeout 600s`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add apps/api/internal/api/exploration_card.go apps/api/internal/api/api.go apps/api/internal/api/exploration_card_test.go
git commit -m "feat(s4): rabbit-hole card persistence — envelope mints a top-level process node"
```

---

### Task 10: Web — proposal chip + rabbit-hole card entry

**Files:**
- Modify: `apps/web/src/workspace/api/workspace.ts` (`coach()` return type + `postRabbitHoleCard`)
- Modify: room chat components that render coach replies (`ReadingBlock.tsx`, `PlanBlock.tsx`, `WritingBlock.tsx`, `ReviewBlock.tsx`) — add the proposal chip (extract a shared `<CoachProposal>` component to avoid 4× duplication)
- Create: `apps/web/src/workspace/blocks/CoachProposal.tsx`
- Modify: `apps/web/src/workspace/blocks/exploration/ExplorationView.tsx` (replace `TODO(S4)` at :214 with the rabbit-hole card entry)
- Test: `apps/web/test/workspace/blocks/CoachProposal.test.tsx`, `apps/web/test/workspace/blocks/exploration/ExplorationView.test.tsx` (extend)

**Interfaces:**
- Consumes: Task 3 contracts (`CardProposal`), Task 8 `/coach` `proposal?`, Task 9 `/exploration/rabbit-hole`.
- Produces: dismissable proposal chip (打开 opens the card via the existing summon/`StudioCardSheet` path; 跳过 dismisses); ExplorationView rabbit-hole entry → `StudioCardSheet` → `postRabbitHoleCard`.

- [ ] **Step 1: Write the failing tests** (vitest — under `apps/web/test/**`, NOT colocated)

```tsx
// CoachProposal.test.tsx
import { render, screen, fireEvent } from "@testing-library/react";
import { CoachProposal } from "@/workspace/blocks/CoachProposal";
it("renders the nudge and opens on 打开", () => {
  const onOpen = vi.fn(), onDismiss = vi.fn();
  render(<CoachProposal proposal={{ cardId: "sift", reason: "横向核查", nudgeText: "要不要打开 SIFT？" }} onOpen={onOpen} onDismiss={onDismiss} />);
  expect(screen.getByText(/SIFT/)).toBeInTheDocument();
  fireEvent.click(screen.getByRole("button", { name: /打开/ }));
  expect(onOpen).toHaveBeenCalledWith("sift");
});
it("dismisses on 跳过 and never auto-opens", () => {
  const onOpen = vi.fn(), onDismiss = vi.fn();
  render(<CoachProposal proposal={{ cardId: "sift", reason: "x", nudgeText: "y" }} onOpen={onOpen} onDismiss={onDismiss} />);
  expect(onOpen).not.toHaveBeenCalled(); // 克制: no auto-open on render
  fireEvent.click(screen.getByRole("button", { name: /跳过/ }));
  expect(onDismiss).toHaveBeenCalled();
});
```
Extend `ExplorationView.test.tsx`: rabbit-hole entry present; clicking opens the card sheet; the standalone card is NOT auto-summoned on render (克制).

- [ ] **Step 2: Run — verify they fail**

Run: `cd apps/web && npx vitest run test/workspace/blocks/CoachProposal.test.tsx`
Expected: FAIL (component missing).

- [ ] **Step 3: Implement**

- `coach()` return type → `{ reply: string; proposal?: { cardId: string; reason: string; nudgeText: string } | null }`.
- `CoachProposal.tsx`: a small card under the coach reply — `nudgeText` + `reason`, `打开`/`跳过` buttons; `onOpen(cardId)` / `onDismiss()`. NO effect that auto-opens.
- In each room, after a coach reply that carries a proposal, render `<CoachProposal>`; `onOpen` routes to the existing summon path (or opens `StudioCardSheet` inline like `ChatSurface.tsx:298-299`); `onDismiss` drops it locally.
- `postRabbitHoleCard(projectId, envelope)` client → `POST /projects/{id}/exploration/rabbit-hole`.
- `ExplorationView.tsx:214`: replace the TODO with a 兔子洞 entry button → opens `StudioCardSheet` with the `rabbit-hole` spec → on submit calls `postRabbitHoleCard` (honest copy — it persists to the process tree).

- [ ] **Step 4: Run — verify they pass**

Run: `cd apps/web && npx vitest run test/workspace/blocks/CoachProposal.test.tsx test/workspace/blocks/exploration/ExplorationView.test.tsx && npx tsc --noEmit`
Expected: PASS + typecheck clean.

- [ ] **Step 5: Commit**

```bash
git add apps/web/src/workspace apps/web/test/workspace
git commit -m "feat(s4): web — coach proposal chip (student-confirmed open) + rabbit-hole card entry"
```

---

## Final verification (whole-branch review gate)

After Task 10, run the FULL suites (not `-run` subsets):
- `cd packages/contracts && npx vitest run`
- `cd apps/api && go test ./... -timeout 600s` (Docker up)
- `cd apps/web && npx vitest run && npx tsc --noEmit`

Then dispatch the whole-branch reviewer (`scripts/review-package $(git merge-base main HEAD) HEAD`). S2/S3 both had a happy-path cross-file bug that only the whole-branch review caught — expect one here too (likely candidates: the oldest-first fold query's `KEEP_LAST` boundary math, or digest-before-fold ordering under concurrent turns, or a proposal firing on a surface where the card is suppressed).

## Self-review notes (author)
- **Spec coverage:** (a) backstop = T5; (b) digest = T1/T2/T4/T5; projection = T6; (c) proposing = T3/T7/T8/T10; (d) rabbit-hole = T9/T10. All spec sections mapped.
- **Type consistency:** `CardProposal{cardId,reason,nudgeText}` identical across contracts (T3), Go DTO (T8), web (T10). `conversation_digest` columns identical across T1/T2/T5.
- **Known deferrals surfaced, not hidden:** T7 flags the classifier-spend question (deterministic → no meter; provider → meter at T8 call site) as a read-the-reference decision; T9 flags the material-coupling risk of `CompleteCard` as a controller-escalation, not a silent drop.
