# Slice 11 — Chat surface + coach-alone + card-as-offer Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Ship the standalone **Chat (聊天)** surface: a coach-alone conversational loop that replies in a guiding posture and, when the student pastes a link, offers a source-evaluation (CRAAP) card in-thread at I3, reproducing agent-spec §5.6 end-to-end.

**Architecture:** A dedicated, isolated Chat path parallel to the Studio's project loop. Thread-scoped card runtime via an additive `thread_id` column on `material`/`card_instances` (migration 0022, mirroring 0021). A new `RunChatStep` (not `RunAgentStep`) ties a pure URL-classifier + a chat-variant coach `reply` typed output, persisting through a small isolated `ChatStore` seam. The surface follows the binding Claude Design dc.html lines 527–666. SSE transport reuses the existing `Text`/`Card`/`Done` envelopes and `parseSSE` client.

**Tech Stack:** Go (net/http · sqlc · goose · testcontainers · `go:embed`) · TypeScript + React + vitest · Zod contracts.

## Global Constraints

- **Client never calls a model directly; keys server-side only in `apps/api`.** All LLM calls go through the gateway; record 档位+token+成本.
- **Spec is `docs/superpowers/specs/2026-07-16-slice-11-chat-surface-design.md`.** Decisions DEC-11.1…11.4 bind.
- **DEC-11.2 — thread scope on `material` + `card_instances` ONLY.** `intervention` is NOT touched. `graph_node`/`graph_edge` thread-scoping deferred (no thread evidence nodes / graph_effects).
- **Competence deferred.** `card_competence` is dormant platform-wide; do NOT write it. The in-thread card only persists answers and flips to `completed`.
- **`reply` is anchor-free but typed.** `{ type:"reply", body }` runs banned-phrasing; echo `OutputCheck` is a no-op in chat (no draft).
- **On-record disclosure is mandatory and binding** (dc.html): subtitle 「自由对话 · AI 只提问，不替你下结论」, green pill 「计入成长评估」 + tooltip 「这些对话会成为你成长评估的一部分」, footer 「AI 会陪你把想法想深，但不替你得出结论 · 你的对话只属于你」. Copy verbatim.
- **RL-1/RL-4 hold.** No write path to any student deliverable (chat has none). `RecordLLMCall` recorded even when enforcement rejects the reply.
- **Cost single-counting:** chat coach calls record one `llm_call` row (`surface="chat"`, `purpose="coach"`, `project_id` NULL, `user_id` set) — the sole org-cost path (`llm_usage` view unions `llm_call`).
- **Multimodal deferred:** render composer attach/image/voice icons per design but wire only text send (inert controls).
- **Icons inline SVG, never lucide-react.** Follow existing shell components.
- **Go tests:** `CGO_ENABLED=0 go test -p 1 ./...` on a quiet Docker daemon; FULL packages for the gate. `make sqlc` from `apps/api` after query changes. `make sync-*` unaffected.
- **Never `git add` untracked user files;** stage named paths only. Pre-existing `M package.json` + untracked user docs/pngs (`docs/03_课程库_单课设计/`, `docs/2026-07-06-spec.md`, `docs/astranova/`, the Toddle handoff zip, `pm-e2e-01-directory.png`, `walk-01-directory.png`) are NOT ours — leave untouched.
- **Project-boundary hook** blocks redirects to `/dev/null` and out-of-project paths; avoid `2>/dev/null`/`> /dev/null` in Bash.
- **Keystone material characteristic:** a pasted link mints a thread `material` with empty `blocks` (no server-side fetch). CRAAP is self-contained — its step 1 (`link_check`) captures the URL and the five dimensions are the student's judgment; span-annotation over fetched source text is a Studio-only richness and is out of scope here.

---

## File Structure

- `apps/api/internal/store/migrations/0022_chat_thread_scope.sql` — additive thread_id + scope CHECKs.
- `apps/api/internal/store/queries/{chat,material,card_instance}.sql` — thread-scoped queries (extend).
- `packages/contracts/src/agentOutput.ts` — add `reply` to `AgentOutput` union.
- `apps/api/internal/agent/enforcement/enforcement.go` — `reply` in `validOutputTypes` + `ValidateOutput`.
- `apps/api/internal/agent/chat_coach.go` — `chatCoachPosturePrompt`, `BuildChatContext`, `ProposeChatReply`.
- `apps/api/internal/agent/chat_step.go` — `DetectURL`, `ChatCardCandidate`, `ChatStore`, `ChatDeps`, `ChatStepResult`, `RunChatStep`.
- `apps/api/internal/agent/chatstore.go` — `sqlcChatStore` adapter + `NewSqlcChatStore`.
- `apps/api/internal/api/chat.go` — thread CRUD + turn (SSE) + card submit/skip handlers.
- `apps/api/internal/api/chat_dto.go` — `ChatThreadDTO`, `ChatMessageDTO`, `ChatCardOfferDTO` + `dto_parity`.
- `apps/api/internal/api/api.go` — route registration.
- `packages/contracts/src/chat.ts` + `index.ts` — Zod DTOs.
- `apps/web/src/api/chat.ts` + `index.ts` — client (list/create/messages/turn SSE/submit/skip).
- `apps/web/src/shell/chat/ChatSurface.tsx` (+ `.test.tsx`), `ChatContainer.tsx` — surface UI.
- `apps/web/src/shell/LeftRail.tsx`, `StudentApp.tsx` — add `chat` tab + mount.

---

## Task 1: Migration 0022 + thread-scoped queries + sqlc

**Files:**
- Create: `apps/api/internal/store/migrations/0022_chat_thread_scope.sql`
- Modify: `apps/api/internal/store/queries/chat.sql`, `material.sql`, `card_instance.sql`
- Regenerate: `apps/api/internal/store/sqlc/*` (via `make sqlc`)
- Test: `apps/api/internal/store/chat_thread_scope_test.go`

**Interfaces:**
- Produces (sqlc, package `sqlc`): `ListThreadsByUser`, `CreateStandaloneThread`, `GetThread`, `ListMessagesByThread`, `CreateThreadMaterial`, `ListMaterialsByThread`, `CreateThreadCardInstance`, `ListCardInstancesByThread`, `SubmitThreadCardInstance`, `SetThreadCardInstanceStatus`. New param structs carry `ThreadID pgtype.UUID`.

- [ ] **Step 1: Verify column nullability (no code — read).**

Confirm from migrations that `material.task_id`, `material.project_id`, `card_instances.task_id`, `card_instances.project_id` are all nullable (0016 added project_id nullable; 0020 dropped task_id NOT NULL). Confirm `chat_thread` exists (0016). Expected: all four nullable — a thread-only row (all-NULL except thread_id) is legal against FKs; only the new CHECK governs it.

- [ ] **Step 2: Write the migration.**

Create `apps/api/internal/store/migrations/0022_chat_thread_scope.sql`:

```sql
-- +goose Up
-- Slice 11: give Chat's card runtime a thread-scoped home. Additive, mirrors
-- 0021's evaluations scope pattern. A material / card_instance belongs to
-- exactly one owner: a project, a chat thread, or a legacy task. intervention
-- is intentionally NOT touched (DEC-11.2: the coach reply is a chat_message,
-- not an anchored intervention; the CRAAP card completes without one).
ALTER TABLE material       ADD COLUMN thread_id uuid REFERENCES chat_thread(id) ON DELETE CASCADE;
ALTER TABLE card_instances ADD COLUMN thread_id uuid REFERENCES chat_thread(id) ON DELETE CASCADE;

CREATE INDEX material_thread_created_idx       ON material (thread_id, created_at);
CREATE INDEX card_instances_thread_created_idx ON card_instances (thread_id, created_at);

-- Scope discipline: at least one owner set. Existing rows (project_id or
-- task_id set, thread_id NULL) satisfy these unchanged; a chat row sets
-- thread_id only.
ALTER TABLE material       ADD CONSTRAINT material_scope_ck
  CHECK (num_nonnulls(task_id, project_id, thread_id) >= 1);
ALTER TABLE card_instances ADD CONSTRAINT card_instances_scope_ck
  CHECK (num_nonnulls(task_id, project_id, thread_id) >= 1);

-- +goose Down
ALTER TABLE material       DROP CONSTRAINT IF EXISTS material_scope_ck;
ALTER TABLE card_instances DROP CONSTRAINT IF EXISTS card_instances_scope_ck;
DROP INDEX IF EXISTS material_thread_created_idx;
DROP INDEX IF EXISTS card_instances_thread_created_idx;
ALTER TABLE material       DROP COLUMN IF EXISTS thread_id;
ALTER TABLE card_instances DROP COLUMN IF EXISTS thread_id;
```

- [ ] **Step 3: Add thread queries to `chat.sql`.**

Append to `apps/api/internal/store/queries/chat.sql`:

```sql
-- Standalone Chat surface (Slice 11): threads owned by a user, not a project.
-- The existing CreateChatMessage above is already thread-keyed and is reused.

-- name: ListThreadsByUser :many
SELECT * FROM chat_thread WHERE user_id = $1 ORDER BY created_at DESC;

-- name: CreateStandaloneThread :one
INSERT INTO chat_thread (user_id, title) VALUES ($1, $2) RETURNING *;

-- name: GetThread :one
SELECT * FROM chat_thread WHERE id = $1;

-- name: ListMessagesByThread :many
SELECT * FROM chat_message WHERE thread_id = $1 ORDER BY created_at, id;
```

- [ ] **Step 4: Add thread material + card queries.**

Append to `apps/api/internal/store/queries/material.sql`:

```sql
-- Thread-scoped materials (Slice 11): task_id + project_id NULL, thread_id set.

-- name: CreateThreadMaterial :one
INSERT INTO material (thread_id, kind, source, title, source_url, blocks)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: ListMaterialsByThread :many
SELECT * FROM material WHERE thread_id = $1 ORDER BY created_at;
```

Append to `apps/api/internal/store/queries/card_instance.sql`:

```sql
-- Thread-scoped card_instances (Slice 11): thread_id set, task_id/project_id NULL.

-- name: CreateThreadCardInstance :one
INSERT INTO card_instances (thread_id, card_id, status)
VALUES ($1, $2, $3)
RETURNING *;

-- name: ListCardInstancesByThread :many
SELECT * FROM card_instances WHERE thread_id = $1 ORDER BY created_at, id;

-- name: SubmitThreadCardInstance :one
UPDATE card_instances SET field_values = $3, event_trace = $4, status = $5
WHERE id = $1 AND thread_id = $2
RETURNING *;

-- name: SetThreadCardInstanceStatus :one
UPDATE card_instances SET status = $3
WHERE id = $1 AND thread_id = $2
RETURNING *;
```

- [ ] **Step 5: Regenerate sqlc.**

Run: `cd apps/api && make sqlc`
Expected: `apps/api/internal/store/sqlc/*.sql.go` regenerated; new methods present; `go build ./...` clean. If `make sqlc` needs `CGO_ENABLED=0`, the Makefile already sets it.

- [ ] **Step 6: Write the failing store test.**

Create `apps/api/internal/store/chat_thread_scope_test.go` (mirror the testcontainers setup used by `chat_sqlc_test.go` — same `newTestQueries(t)`/pool helper):

```go
package store

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
)

func TestChatThreadScope(t *testing.T) {
	ctx := context.Background()
	q, cleanup := newTestQueries(t) // same helper chat_sqlc_test.go uses
	defer cleanup()

	userID := seedUser(t, q) // same seed helper the sibling chat test uses

	// A standalone thread (no project).
	th, err := q.CreateStandaloneThread(ctx, CreateStandaloneThreadParams{UserID: userID, Title: "test"})
	if err != nil { t.Fatalf("CreateStandaloneThread: %v", err) }

	// A thread-scoped material: task_id + project_id NULL, thread_id set — legal.
	tid := pgtype.UUID{Bytes: th.ID.Bytes, Valid: true}
	m, err := q.CreateThreadMaterial(ctx, CreateThreadMaterialParams{
		ThreadID: tid, Kind: "article", Source: "pasted", Title: "x", SourceUrl: pgtype.Text{String: "https://e.com", Valid: true}, Blocks: []byte("[]"),
	})
	if err != nil { t.Fatalf("CreateThreadMaterial: %v", err) }
	if m.ThreadID != tid { t.Fatalf("material thread_id not set") }

	mats, err := q.ListMaterialsByThread(ctx, tid)
	if err != nil || len(mats) != 1 { t.Fatalf("ListMaterialsByThread: %v n=%d", err, len(mats)) }

	// A thread-scoped card_instance flips to completed.
	ci, err := q.CreateThreadCardInstance(ctx, CreateThreadCardInstanceParams{ThreadID: tid, CardID: "craap", Status: "proposed"})
	if err != nil { t.Fatalf("CreateThreadCardInstance: %v", err) }
	done, err := q.SubmitThreadCardInstance(ctx, SubmitThreadCardInstanceParams{
		ID: ci.ID, ThreadID: tid, FieldValues: []byte(`{"a":1}`), EventTrace: []byte("[]"), Status: "completed",
	})
	if err != nil || done.Status != "completed" { t.Fatalf("SubmitThreadCardInstance: %v status=%s", err, done.Status) }

	cis, err := q.ListCardInstancesByThread(ctx, tid)
	if err != nil || len(cis) != 1 { t.Fatalf("ListCardInstancesByThread: %v n=%d", err, len(cis)) }
}
```

> Adapt `newTestQueries`, `seedUser`, and the exact param field names (`SourceUrl` vs `SourceURL`) to what sqlc actually generated and what `chat_sqlc_test.go` already uses — read that sibling file first.

- [ ] **Step 7: Run the migration + test to verify it passes.**

Run: `cd apps/api && CGO_ENABLED=0 go test -p 1 ./internal/store/ -run TestChatThreadScope -v`
Expected: PASS (goose applies 0022; thread-scoped rows insert; completion flips).

- [ ] **Step 8: Verify back-compat — full store package.**

Run: `cd apps/api && CGO_ENABLED=0 go test -p 1 ./internal/store/`
Expected: `ok` — existing project/task material + card_instance tests still pass against the new CHECKs.

- [ ] **Step 9: Commit.**

```bash
git add apps/api/internal/store/migrations/0022_chat_thread_scope.sql apps/api/internal/store/queries/chat.sql apps/api/internal/store/queries/material.sql apps/api/internal/store/queries/card_instance.sql apps/api/internal/store/sqlc apps/api/internal/store/chat_thread_scope_test.go
git commit -m "feat(refactor2): Slice 11 T1 — thread-scoped material/card_instance + standalone thread queries"
```

---

## Task 2: The `reply` typed output (C3 contract change)

**Files:**
- Modify: `packages/contracts/src/agentOutput.ts:28-57`
- Modify: `apps/api/internal/agent/enforcement/enforcement.go:42-80`
- Test: `packages/contracts/src/agentOutput.test.ts` (extend or create), `apps/api/internal/agent/enforcement/enforcement_test.go`

**Interfaces:**
- Produces: Zod `AgentOutput` accepts `{ type:"reply", body }`; Go `enforcement.ValidateOutput` accepts a well-formed reply and rejects an empty body.

- [ ] **Step 1: Write the failing contract test.**

Add to `packages/contracts/src/agentOutput.test.ts` (create if absent, mirror a sibling `*.test.ts`):

```ts
import { describe, it, expect } from "vitest";
import { AgentOutput } from "./agentOutput";

describe("AgentOutput reply", () => {
  it("accepts a reply with a body", () => {
    expect(AgentOutput.safeParse({ type: "reply", body: "让我们想想。" }).success).toBe(true);
  });
  it("rejects a reply with an empty body", () => {
    expect(AgentOutput.safeParse({ type: "reply", body: "" }).success).toBe(false);
  });
});
```

- [ ] **Step 2: Run it to verify it fails.**

Run: `cd packages/contracts && npx vitest run src/agentOutput.test.ts`
Expected: FAIL (reply not in the union).

- [ ] **Step 3: Add `reply` to the Zod union.**

In `packages/contracts/src/agentOutput.ts`, add as the last member of the `AgentOutput` discriminated union (after the `plan` object, before the closing `]`):

```ts
  z.object({
    type: z.literal("reply"),
    body: z.string().min(1),
  }),
```

- [ ] **Step 4: Run the contract test to verify it passes.**

Run: `cd packages/contracts && npx vitest run src/agentOutput.test.ts`
Expected: PASS.

- [ ] **Step 5: Write the failing Go enforcement test.**

Add to `apps/api/internal/agent/enforcement/enforcement_test.go`:

```go
func TestValidateOutputReply(t *testing.T) {
	if err := ValidateOutput(AgentOutput{Type: "reply", Body: "让我们想想。"}); err != nil {
		t.Fatalf("well-formed reply rejected: %v", err)
	}
	if err := ValidateOutput(AgentOutput{Type: "reply", Body: ""}); err == nil {
		t.Fatalf("empty-body reply should be rejected")
	}
}
```

- [ ] **Step 6: Run it to verify it fails.**

Run: `cd apps/api && CGO_ENABLED=0 go test ./internal/agent/enforcement/ -run TestValidateOutputReply`
Expected: FAIL (`unknown output type "reply"`).

- [ ] **Step 7: Add `reply` to Go enforcement.**

In `apps/api/internal/agent/enforcement/enforcement.go`: add `"reply": true,` to `validOutputTypes`, and add a case to `ValidateOutput`'s switch:

```go
	case "reply":
		if strings.TrimSpace(out.Body) == "" {
			return fmt.Errorf("enforcement: reply output requires a body")
		}
```

- [ ] **Step 8: Run both test suites to verify they pass.**

Run: `cd apps/api && CGO_ENABLED=0 go test ./internal/agent/enforcement/`
Expected: `ok`. (Contracts already green from Step 4.)

- [ ] **Step 9: Commit.**

```bash
git add packages/contracts/src/agentOutput.ts packages/contracts/src/agentOutput.test.ts apps/api/internal/agent/enforcement/enforcement.go apps/api/internal/agent/enforcement/enforcement_test.go
git commit -m "feat(refactor2): Slice 11 T2 — reply typed output (C3), banned-phrasing enforced"
```

---

## Task 3: Chat coach — ProposeChatReply + BuildChatContext + posture prompt

**Files:**
- Create: `apps/api/internal/agent/chat_coach.go`
- Test: `apps/api/internal/agent/chat_coach_test.go`

**Interfaces:**
- Consumes: `gateway.Provider`, `gateway.Resolved`, `gateway.Collect`, `enforcement.AgentOutput`, `enforcement.BannedPhrasing`, `ChatTurn` (loop.go).
- Produces:
  - `func BuildChatContext(history []ChatTurn, threadSummary string, flag string) string`
  - `func ProposeChatReply(ctx context.Context, prov gateway.Provider, r gateway.Resolved, history []ChatTurn, threadSummary, flag string) (enforcement.AgentOutput, gateway.ChatUsage, error)` — returns a `{Type:"reply"}` output; usage populated whenever `Collect` succeeded (including on enforcement reject).

- [ ] **Step 1: Write the failing test.**

Create `apps/api/internal/agent/chat_coach_test.go` (mirror how `coach_test.go` stubs a provider — reuse its fake provider helper):

```go
package agent

import (
	"context"
	"testing"

	"mindimprint/api/internal/gateway"
)

func TestProposeChatReplyAccepts(t *testing.T) {
	prov := newStubProvider("你为什么觉得它证明了你的观点？") // reuse coach_test.go's stub
	out, usage, err := ProposeChatReply(context.Background(), prov, gateway.Resolved{Provider: "deepseek", Model: "x"},
		[]ChatTurn{{Role: "user", Content: "这篇报道证明了我的观点"}}, "", "")
	if err != nil { t.Fatalf("unexpected err: %v", err) }
	if out.Type != "reply" || out.Body == "" { t.Fatalf("want reply with body, got %+v", out) }
	if usage.OutputTokens == 0 && usage.InputTokens == 0 { t.Fatalf("usage not populated") }
}

func TestProposeChatReplyBannedPhrasingRejects(t *testing.T) {
	prov := newStubProvider("你应该这样写：中国让地球更可持续。") // ghostwriting guard
	_, usage, err := ProposeChatReply(context.Background(), prov, gateway.Resolved{Provider: "deepseek", Model: "x"},
		[]ChatTurn{{Role: "user", Content: "帮我写"}}, "", "")
	if err == nil { t.Fatalf("banned phrasing should reject") }
	if usage.OutputTokens == 0 && usage.InputTokens == 0 { t.Fatalf("rejected reply must still be metered") }
}
```

> Read `coach_test.go` first for the exact stub-provider constructor name and usage-injection shape; match it.

- [ ] **Step 2: Run it to verify it fails.**

Run: `cd apps/api && CGO_ENABLED=0 go test ./internal/agent/ -run TestProposeChatReply`
Expected: FAIL (`undefined: ProposeChatReply`).

- [ ] **Step 3: Implement `chat_coach.go`.**

```go
package agent

import (
	"context"
	"fmt"
	"strings"

	"mindimprint/api/internal/agent/enforcement"
	"mindimprint/api/internal/gateway"
)

// chatCoachPosturePrompt is the Chat-variant system prompt (agent-spec §5.4):
// coach alone, guiding-not-answering, ONE notch more permissive than the
// Studio (may explain and inform, scoped — product §2.2) but never producing
// the student's assessed deliverable, one question at a time, restraint ladder.
const chatCoachPosturePrompt = `你是「思维印记」的自由对话陪练。你的职责不是给答案，而是在对的时刻把「思考」塞回给学生。

- 引导，不代答：你可以解释、可以科普（这是学习空间），但绝不替学生写出他要被评估的成品，绝不替他下结论。
- 一次只问一个：回复简短，不啰嗦，顺着学生的话往深里带一步。
- 克制：当学生已经在思考时，别打断；当他想让你替他想时，把问题还给他。
- 当学生贴进一个来源链接、并把它当成论据时，先顺着他的点回应一句，再（由系统）把「信源辨识」作为一个邀请递上——是邀请，不是打断。

只输出你要对学生说的那段话本身，不要任何前缀、标签或格式。`

// BuildChatContext assembles the chat-variant coach user message: the recent
// conversation, an optional one-line thread-graph summary, and an optional
// classifier flag naming a fresh card moment. Kept pure for testing.
func BuildChatContext(history []ChatTurn, threadSummary, flag string) string {
	var b strings.Builder
	b.WriteString("对话（从旧到新）：\n")
	for _, t := range history {
		who := "学生"
		if t.Role == "assistant" {
			who = "你"
		}
		b.WriteString(fmt.Sprintf("- %s：%s\n", who, t.Content))
	}
	if strings.TrimSpace(threadSummary) != "" {
		b.WriteString("\n此对话中已有的材料：" + threadSummary + "\n")
	}
	if strings.TrimSpace(flag) != "" {
		b.WriteString("\n刚刚发生的思考时机：" + flag + "\n")
	}
	b.WriteString("\n现在，用一句话回应学生最新的发言。")
	return b.String()
}

// ProposeChatReply asks the flagship model for one conversational reply, then
// runs the chat enforcement subset: ValidateOutput (reply shape) + BannedPhrasing.
// There is no OutputCheck echo pass — chat has no draft to echo. Usage is
// populated whenever Collect succeeded, INCLUDING when enforcement then rejects
// (a rejected reply still cost money; the caller must still meter it).
func ProposeChatReply(ctx context.Context, prov gateway.Provider, r gateway.Resolved, history []ChatTurn, threadSummary, flag string) (enforcement.AgentOutput, gateway.ChatUsage, error) {
	req := gateway.ChatRequest{
		Messages: []gateway.ChatMessage{
			{Role: gateway.RoleSystem, Content: chatCoachPosturePrompt},
			{Role: gateway.RoleUser, Content: BuildChatContext(history, threadSummary, flag)},
		},
	}
	res, err := gateway.Collect(ctx, prov, r, req)
	if err != nil {
		return enforcement.AgentOutput{}, gateway.ChatUsage{}, err
	}
	usage := res.Usage

	out := enforcement.AgentOutput{Type: "reply", Body: strings.TrimSpace(res.Text)}
	if err := enforcement.ValidateOutput(out); err != nil {
		return enforcement.AgentOutput{}, usage, err
	}
	if rule := enforcement.BannedPhrasing(out.Body); rule != nil {
		return enforcement.AgentOutput{}, usage, fmt.Errorf("agent: chat reply rejected by banned-phrasing rule %q", rule.Name)
	}
	return out, usage, nil
}
```

- [ ] **Step 4: Run the test to verify it passes.**

Run: `cd apps/api && CGO_ENABLED=0 go test ./internal/agent/ -run TestProposeChatReply -v`
Expected: PASS (accept returns reply + usage; banned-phrasing rejects + still meters).

- [ ] **Step 5: Commit.**

```bash
git add apps/api/internal/agent/chat_coach.go apps/api/internal/agent/chat_coach_test.go
git commit -m "feat(refactor2): Slice 11 T3 — chat coach (ProposeChatReply, guiding posture, reply output)"
```

---

## Task 4: Chat classifier + RunChatStep + ChatStore seam

**Files:**
- Create: `apps/api/internal/agent/chat_step.go`
- Create: `apps/api/internal/agent/chatstore.go`
- Test: `apps/api/internal/agent/chat_step_test.go`

**Interfaces:**
- Consumes: `ProposeChatReply`, `ChatTurn`, `gateway.Provider`/`Resolved`.
- Produces:
  - `func DetectURL(s string) (string, bool)` — first http(s) URL in the message.
  - types `ThreadMaterial{ID uuid.UUID; Kind, SourceURL string}`, `ThreadCard{ID uuid.UUID; CardID, Status string}`.
  - `ChatStore` interface (below).
  - `ChatDeps{Store ChatStore; Provider gateway.Provider; Resolved gateway.Resolved; UserID, ThreadID uuid.UUID}`.
  - `ChatStepResult{Reply string; Offer *ChatCardOffer}`, `ChatCardOffer{CardInstanceID, MaterialID uuid.UUID; CardID string}`.
  - `func ChatCardCandidate(materials []ThreadMaterial, cards []ThreadCard) (materialID uuid.UUID, cardID string, ok bool)` — pure.
  - `func RunChatStep(ctx context.Context, deps ChatDeps, studentMessage string) (ChatStepResult, error)`.
  - `func NewSqlcChatStore(q *sqlc.Queries) ChatStore` (in chatstore.go).

- [ ] **Step 1: Write the failing pure-classifier test.**

Create `apps/api/internal/agent/chat_step_test.go`:

```go
package agent

import (
	"testing"

	"github.com/google/uuid"
)

func TestDetectURL(t *testing.T) {
	if u, ok := DetectURL("看这个 https://nasa.gov/x 它证明了"); !ok || u != "https://nasa.gov/x" {
		t.Fatalf("got %q %v", u, ok)
	}
	if _, ok := DetectURL("我觉得中国让地球更可持续"); ok {
		t.Fatalf("no URL should be detected")
	}
}

func TestChatCardCandidate(t *testing.T) {
	m := uuid.New()
	mats := []ThreadMaterial{{ID: m, Kind: "article", SourceURL: "https://e.com"}}

	// fresh article, no card yet → offer craap on it
	mid, cid, ok := ChatCardCandidate(mats, nil)
	if !ok || cid != "craap" || mid != m { t.Fatalf("want craap offer, got %v %s %v", mid, cid, ok) }

	// a craap card already exists in the thread (any status) → suppressed
	for _, st := range []string{"proposed", "active", "completed", "skipped"} {
		if _, _, ok := ChatCardCandidate(mats, []ThreadCard{{ID: uuid.New(), CardID: "craap", Status: st}}); ok {
			t.Fatalf("status %s should suppress re-offer", st)
		}
	}
}
```

- [ ] **Step 2: Run it to verify it fails.**

Run: `cd apps/api && CGO_ENABLED=0 go test ./internal/agent/ -run 'TestDetectURL|TestChatCardCandidate'`
Expected: FAIL (undefined).

- [ ] **Step 3: Implement the pure classifier + types in `chat_step.go`.**

```go
package agent

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/url"
	"strings"

	"github.com/google/uuid"

	"mindimprint/api/internal/gateway"
)

// DetectURL returns the first http(s) URL in s. Keyless, no fetch — the minted
// material is source="pasted" with empty blocks; CRAAP is self-contained (its
// step 1 link_check captures the URL).
func DetectURL(s string) (string, bool) {
	for _, tok := range strings.Fields(s) {
		tok = strings.Trim(tok, "，。！？;,.!?()（）「」\"'")
		if strings.HasPrefix(tok, "http://") || strings.HasPrefix(tok, "https://") {
			if u, err := url.Parse(tok); err == nil && u.Host != "" {
				return tok, true
			}
		}
	}
	return "", false
}

// ThreadMaterial / ThreadCard are the thread graph's minimal projection the
// classifier reads.
type ThreadMaterial struct {
	ID        uuid.UUID
	Kind      string
	SourceURL string
}
type ThreadCard struct {
	ID     uuid.UUID
	CardID string
	Status string
}

// ChatCardCandidate implements the keystone card-moment predicate: offer CRAAP
// (I3) on the most recent article material in the thread, UNLESS the thread
// already has a CRAAP card_instance in ANY status (proposed/active/completed/
// skipped) — an offer, once made or declined, is not re-raised in the thread.
// One source-evaluation offer per thread is a deliberate keystone simplification
// (card_instances carry no material link without the deferred thread graph edges).
func ChatCardCandidate(materials []ThreadMaterial, cards []ThreadCard) (uuid.UUID, string, bool) {
	for _, c := range cards {
		if c.CardID == craapCardID {
			return uuid.Nil, "", false
		}
	}
	var target uuid.UUID
	found := false
	for _, m := range materials {
		if m.Kind == "article" {
			target = m.ID // last article wins (materials come in created order)
			found = true
		}
	}
	if !found {
		return uuid.Nil, "", false
	}
	return target, craapCardID, true
}

// ChatStore is RunChatStep's isolated persistence seam (project-free). The
// sqlc adapter is chatstore.go; chat_step_test.go uses an in-memory fake.
type ChatStore interface {
	LoadThreadHistory(ctx context.Context, threadID uuid.UUID, limit int) ([]ChatTurn, error)
	CreateThreadMessage(ctx context.Context, threadID uuid.UUID, role, content, modality string) (uuid.UUID, error)
	ListThreadMaterials(ctx context.Context, threadID uuid.UUID) ([]ThreadMaterial, error)
	CreateThreadMaterial(ctx context.Context, threadID uuid.UUID, kind, source, title, sourceURL string) (uuid.UUID, error)
	ListThreadCards(ctx context.Context, threadID uuid.UUID) ([]ThreadCard, error)
	CreateThreadCardInstance(ctx context.Context, threadID uuid.UUID, cardID string) (uuid.UUID, error)
	InsertUserEvent(ctx context.Context, userID uuid.UUID, surface, typ string, payload []byte) error
	RecordChatLLMCall(ctx context.Context, userID uuid.UUID, resolved gateway.Resolved, prompt, completion int32) error
}

type ChatDeps struct {
	Store    ChatStore
	Provider gateway.Provider
	Resolved gateway.Resolved
	UserID   uuid.UUID
	ThreadID uuid.UUID
}

type ChatCardOffer struct {
	CardInstanceID uuid.UUID
	MaterialID     uuid.UUID
	CardID         string
}
type ChatStepResult struct {
	Reply string
	Offer *ChatCardOffer
}

// RunChatStep is the Chat-policy turn (agent-spec §5.4): coach alone, planner
// off. It (1) mints a thread material for a newly pasted URL, (2) asks the
// coach for one reply, metering the call even on enforcement reject, (3) on an
// accepted reply persists it as an assistant chat_message, and (4) surfaces a
// fresh CRAAP offer (I3) when the thread has an un-carded article. The handler
// has already persisted the student message + the prompt_sent event.
func RunChatStep(ctx context.Context, deps ChatDeps, studentMessage string) (ChatStepResult, error) {
	// (1) URL → thread material (dedup by source_url).
	if u, ok := DetectURL(studentMessage); ok {
		mats, err := deps.Store.ListThreadMaterials(ctx, deps.ThreadID)
		if err != nil {
			return ChatStepResult{}, err
		}
		exists := false
		for _, m := range mats {
			if m.SourceURL == u {
				exists = true
				break
			}
		}
		if !exists {
			host := u
			if pu, err := url.Parse(u); err == nil && pu.Host != "" {
				host = pu.Host
			}
			if _, err := deps.Store.CreateThreadMaterial(ctx, deps.ThreadID, "article", "pasted", host, u); err != nil {
				return ChatStepResult{}, err
			}
		}
	}

	// Classifier flag + coach context.
	mats, err := deps.Store.ListThreadMaterials(ctx, deps.ThreadID)
	if err != nil {
		return ChatStepResult{}, err
	}
	cards, err := deps.Store.ListThreadCards(ctx, deps.ThreadID)
	if err != nil {
		return ChatStepResult{}, err
	}
	materialID, cardID, moment := ChatCardCandidate(mats, cards)
	flag := ""
	if moment {
		flag = "学生贴进了一个来源链接，并把它当成论据——这是做「信源辨识（CRAAP）」的时机。"
	}
	history, err := deps.Store.LoadThreadHistory(ctx, deps.ThreadID, 12)
	if err != nil {
		slog.Warn("chat: load history failed; proceeding without it", "thread_id", deps.ThreadID.String(), "err", err.Error())
		history = nil
	}
	summary := threadMaterialSummary(mats)

	// (2) Coach reply, metered even on reject.
	out, usage, err := ProposeChatReply(ctx, deps.Provider, deps.Resolved, history, summary, flag)
	if usage.InputTokens > 0 || usage.OutputTokens > 0 {
		if rerr := deps.Store.RecordChatLLMCall(ctx, deps.UserID, deps.Resolved, int32(usage.InputTokens), int32(usage.OutputTokens)); rerr != nil {
			slog.Warn("chat: record llm usage failed", "thread_id", deps.ThreadID.String(), "err", rerr.Error())
		}
	}
	if err != nil {
		// Enforcement (or the model call) rejected the reply — stay silent.
		slog.Warn("chat: reply not emitted", "thread_id", deps.ThreadID.String(), "err", err.Error())
		return ChatStepResult{}, nil
	}

	// (3) Persist the accepted reply.
	if _, err := deps.Store.CreateThreadMessage(ctx, deps.ThreadID, "assistant", out.Body, "text"); err != nil {
		return ChatStepResult{}, err
	}
	result := ChatStepResult{Reply: out.Body}

	// (4) Fresh card offer.
	if moment {
		ciID, err := deps.Store.CreateThreadCardInstance(ctx, deps.ThreadID, cardID)
		if err != nil {
			return ChatStepResult{}, err
		}
		result.Offer = &ChatCardOffer{CardInstanceID: ciID, MaterialID: materialID, CardID: cardID}
		payload, _ := json.Marshal(map[string]string{"card_id": cardID})
		if err := deps.Store.InsertUserEvent(ctx, deps.UserID, "chat", "card_surfaced", payload); err != nil {
			slog.Warn("chat: append card_surfaced event failed", "thread_id", deps.ThreadID.String(), "err", err.Error())
		}
	}
	return result, nil
}

func threadMaterialSummary(mats []ThreadMaterial) string {
	var parts []string
	for _, m := range mats {
		if m.Kind == "article" {
			parts = append(parts, m.SourceURL)
		}
	}
	return strings.Join(parts, "、")
}
```

- [ ] **Step 4: Run the pure tests to verify they pass.**

Run: `cd apps/api && CGO_ENABLED=0 go test ./internal/agent/ -run 'TestDetectURL|TestChatCardCandidate' -v`
Expected: PASS.

- [ ] **Step 5: Write the RunChatStep fake-store test.**

Add to `chat_step_test.go` an in-memory `fakeChatStore` implementing `ChatStore` (record calls; return canned history/materials/cards), then:

```go
func TestRunChatStepReplyOnly(t *testing.T) {
	fs := newFakeChatStore() // no materials, no url in message
	prov := newStubProvider("你为什么这么想？")
	res, err := RunChatStep(context.Background(), ChatDeps{Store: fs, Provider: prov, Resolved: gateway.Resolved{Provider: "deepseek", Model: "x"}, UserID: uuid.New(), ThreadID: uuid.New()}, "我觉得我对")
	if err != nil { t.Fatalf("err: %v", err) }
	if res.Reply == "" || res.Offer != nil { t.Fatalf("want reply-only, got %+v", res) }
	if fs.llmCalls != 1 { t.Fatalf("coach call must be metered once, got %d", fs.llmCalls) }
	if fs.assistantMsgs != 1 { t.Fatalf("reply must be persisted") }
}

func TestRunChatStepMintsMaterialAndOffers(t *testing.T) {
	fs := newFakeChatStore()
	prov := newStubProvider("这确实值得核实一下。")
	res, err := RunChatStep(context.Background(), ChatDeps{Store: fs, Provider: prov, Resolved: gateway.Resolved{Provider: "deepseek", Model: "x"}, UserID: uuid.New(), ThreadID: uuid.New()}, "看 https://nasa.gov/x 它证明了我的观点")
	if err != nil { t.Fatalf("err: %v", err) }
	if fs.materialsCreated != 1 { t.Fatalf("a thread material must be minted for the URL") }
	if res.Offer == nil || res.Offer.CardID != "craap" { t.Fatalf("want a craap offer, got %+v", res.Offer) }
}

func TestRunChatStepBannedReplyStaysSilentButMeters(t *testing.T) {
	fs := newFakeChatStore()
	prov := newStubProvider("你应该这样写：中国让地球更可持续。")
	res, err := RunChatStep(context.Background(), ChatDeps{Store: fs, Provider: prov, Resolved: gateway.Resolved{Provider: "deepseek", Model: "x"}, UserID: uuid.New(), ThreadID: uuid.New()}, "帮我写")
	if err != nil { t.Fatalf("silence is not an error: %v", err) }
	if res.Reply != "" { t.Fatalf("rejected reply must not be emitted") }
	if fs.llmCalls != 1 { t.Fatalf("rejected reply must still be metered") }
	if fs.assistantMsgs != 0 { t.Fatalf("rejected reply must not be persisted") }
}
```

- [ ] **Step 6: Run RunChatStep tests to verify they pass.**

Run: `cd apps/api && CGO_ENABLED=0 go test ./internal/agent/ -run TestRunChatStep -v`
Expected: PASS.

- [ ] **Step 7: Implement the sqlc `ChatStore` adapter in `chatstore.go`.**

Mirror `agentstore.go`'s `sqlcAgentStore` (constructor, `pgtype.UUID` wrapping, cost via `gateway.EstimateCost` + `gateway.CostNumeric`). Each method wraps one sqlc query:

```go
package agent

import (
	"context"
	"log/slog"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"mindimprint/api/internal/gateway"
	"mindimprint/api/internal/store/sqlc"
)

type sqlcChatStore struct{ q *sqlc.Queries }

func NewSqlcChatStore(q *sqlc.Queries) ChatStore { return &sqlcChatStore{q: q} }

func tid(id uuid.UUID) pgtype.UUID { return pgtype.UUID{Bytes: id, Valid: true} }

func (s *sqlcChatStore) LoadThreadHistory(ctx context.Context, threadID uuid.UUID, limit int) ([]ChatTurn, error) {
	msgs, err := s.q.ListMessagesByThread(ctx, tid(threadID))
	if err != nil {
		return nil, err
	}
	if limit > 0 && len(msgs) > limit {
		msgs = msgs[len(msgs)-limit:]
	}
	out := make([]ChatTurn, 0, len(msgs))
	for _, m := range msgs {
		out = append(out, ChatTurn{Role: m.Role, Content: m.Content})
	}
	return out, nil
}

func (s *sqlcChatStore) CreateThreadMessage(ctx context.Context, threadID uuid.UUID, role, content, modality string) (uuid.UUID, error) {
	m, err := s.q.CreateChatMessage(ctx, sqlc.CreateChatMessageParams{ThreadID: tid(threadID), Role: role, Content: content, Modality: modality})
	if err != nil {
		return uuid.Nil, err
	}
	return m.ID.Bytes, nil
}

func (s *sqlcChatStore) ListThreadMaterials(ctx context.Context, threadID uuid.UUID) ([]ThreadMaterial, error) {
	rows, err := s.q.ListMaterialsByThread(ctx, tid(threadID))
	if err != nil {
		return nil, err
	}
	out := make([]ThreadMaterial, 0, len(rows))
	for _, r := range rows {
		out = append(out, ThreadMaterial{ID: r.ID.Bytes, Kind: r.Kind, SourceURL: r.SourceUrl.String})
	}
	return out, nil
}

func (s *sqlcChatStore) CreateThreadMaterial(ctx context.Context, threadID uuid.UUID, kind, source, title, sourceURL string) (uuid.UUID, error) {
	m, err := s.q.CreateThreadMaterial(ctx, sqlc.CreateThreadMaterialParams{
		ThreadID: tid(threadID), Kind: kind, Source: source, Title: title,
		SourceUrl: pgtype.Text{String: sourceURL, Valid: sourceURL != ""}, Blocks: []byte("[]"),
	})
	if err != nil {
		return uuid.Nil, err
	}
	return m.ID.Bytes, nil
}

func (s *sqlcChatStore) ListThreadCards(ctx context.Context, threadID uuid.UUID) ([]ThreadCard, error) {
	rows, err := s.q.ListCardInstancesByThread(ctx, tid(threadID))
	if err != nil {
		return nil, err
	}
	out := make([]ThreadCard, 0, len(rows))
	for _, r := range rows {
		out = append(out, ThreadCard{ID: r.ID.Bytes, CardID: r.CardID, Status: r.Status})
	}
	return out, nil
}

func (s *sqlcChatStore) CreateThreadCardInstance(ctx context.Context, threadID uuid.UUID, cardID string) (uuid.UUID, error) {
	ci, err := s.q.CreateThreadCardInstance(ctx, sqlc.CreateThreadCardInstanceParams{ThreadID: tid(threadID), CardID: cardID, Status: "proposed"})
	if err != nil {
		return uuid.Nil, err
	}
	return ci.ID.Bytes, nil
}

func (s *sqlcChatStore) InsertUserEvent(ctx context.Context, userID uuid.UUID, surface, typ string, payload []byte) error {
	_, err := s.q.AppendEvent(ctx, sqlc.AppendEventParams{
		ProjectID: pgtype.UUID{Valid: false}, UserID: pgtype.UUID{Bytes: userID, Valid: true},
		Surface: surface, Type: typ, Payload: payload,
	})
	return err
}

func (s *sqlcChatStore) RecordChatLLMCall(ctx context.Context, userID uuid.UUID, resolved gateway.Resolved, prompt, completion int32) error {
	cost, priced := gateway.EstimateCost(resolved.Provider, resolved.Model, int(prompt), int(completion))
	if !priced {
		slog.Warn("chat llm_call: unpriced model — cost recorded as 0", "provider", resolved.Provider, "model", resolved.Model)
	}
	_, err := s.q.RecordLLMCall(ctx, sqlc.RecordLLMCallParams{
		UserID: pgtype.UUID{Bytes: userID, Valid: true}, ProjectID: pgtype.UUID{Valid: false},
		Surface: "chat", Purpose: "coach",
		Provider: resolved.Provider, Model: resolved.Model, Tier: resolved.Tier,
		PromptTokens: prompt, CompletionTokens: completion, CostEstimate: gateway.CostNumeric(cost, true),
	})
	return err
}
```

> Verify exact sqlc field names/types (`AppendEventParams.UserID` may be `uuid.UUID` not `pgtype.UUID`; `RecordLLMCallParams` field names; `m.ID.Bytes` vs `m.ID`) against the generated code and `agentstore.go`'s existing usage — match them exactly. Adjust the `.Bytes` unwrapping to whatever sqlc emits.

- [ ] **Step 8: Verify the adapter compiles + full agent package.**

Run: `cd apps/api && CGO_ENABLED=0 go build ./... && CGO_ENABLED=0 go test -p 1 ./internal/agent/`
Expected: build clean; `ok`.

- [ ] **Step 9: Commit.**

```bash
git add apps/api/internal/agent/chat_step.go apps/api/internal/agent/chatstore.go apps/api/internal/agent/chat_step_test.go
git commit -m "feat(refactor2): Slice 11 T4 — RunChatStep + chat classifier + isolated ChatStore"
```

---

## Task 5: Chat endpoints + DTOs + parity

**Files:**
- Create: `apps/api/internal/api/chat.go`, `apps/api/internal/api/chat_dto.go`
- Modify: `apps/api/internal/api/api.go` (routes)
- Test: `apps/api/internal/api/chat_test.go`, `apps/api/internal/api/chat_dto_parity_test.go`

**Interfaces:**
- Consumes: `agent.RunChatStep`, `agent.NewSqlcChatStore`, `HasEntitlement`, `UserFromContext`, `loadOwned*` pattern, `gateway.NewSSEWriter`, `studioEmitter`/`startHeartbeat`, `httpx.*`, `decodeJSON`, `validateFieldValues`/`validateEventTrace`/`validateAnchors`.
- Produces routes:
  - `GET /api/v1/chat/threads` · `POST /api/v1/chat/threads` · `GET /api/v1/chat/threads/{id}/messages` · `POST /api/v1/chat/threads/{id}/turn` (SSE) · `POST /api/v1/chat/threads/{id}/cards/{cid}/submit` (JSON) · `POST /api/v1/chat/threads/{id}/cards/{cid}/skip` (JSON).
  - DTOs `ChatThreadDTO{id,title,createdAt}`, `ChatMessageDTO{id,role,content,modality,createdAt}`, `ChatCardOfferDTO{cardInstanceId,cardId,materialId}`.

- [ ] **Step 1: Write the DTOs + parity test.**

Create `apps/api/internal/api/chat_dto.go`:

```go
package api

import (
	"time"

	"mindimprint/api/internal/store/sqlc"
)

type ChatThreadDTO struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	CreatedAt string `json:"createdAt"`
}

type ChatMessageDTO struct {
	ID        string `json:"id"`
	Role      string `json:"role"`
	Content   string `json:"content"`
	Modality  string `json:"modality"`
	CreatedAt string `json:"createdAt"`
}

type ChatCardOfferDTO struct {
	CardInstanceID string `json:"cardInstanceId"`
	CardID         string `json:"cardId"`
	MaterialID     string `json:"materialId"`
}

func toChatThreadDTO(t sqlc.ChatThread) ChatThreadDTO {
	return ChatThreadDTO{ID: uuidStr(t.ID), Title: t.Title, CreatedAt: t.CreatedAt.Format(time.RFC3339)}
}

func toChatMessageDTO(m sqlc.ChatMessage) ChatMessageDTO {
	return ChatMessageDTO{ID: uuidStr(m.ID), Role: m.Role, Content: m.Content, Modality: m.Modality, CreatedAt: m.CreatedAt.Format(time.RFC3339)}
}
```

> `uuidStr` and the `CreatedAt` accessor: match how sibling DTOs (e.g. `assessment_dto.go`, existing `*_dto.go`) convert `pgtype.UUID`/`timestamptz` to string — reuse the existing helper, don't invent one.

Create `apps/api/internal/api/chat_dto_parity_test.go` mirroring `studio/dto_parity_test.go`'s `TestAssessmentDTOJSONKeys` — marshal each DTO, assert the exact JSON key set (`id,title,createdAt` / `id,role,content,modality,createdAt` / `cardInstanceId,cardId,materialId`).

- [ ] **Step 2: Run the parity test to verify it fails.**

Run: `cd apps/api && CGO_ENABLED=0 go test ./internal/api/ -run TestChatDTO`
Expected: FAIL (types undefined) → after Step 1 compiles, the key-set asserts drive the shape.

- [ ] **Step 3: Implement the handlers in `chat.go`.**

Write handlers on `*API` mirroring the named patterns:
- `getChatThreads` — `UserFromContext` → `ListThreadsByUser` → `[]ChatThreadDTO` via `httpx.WriteJSON`.
- `createChatThread` — decode `{title?}` → `CreateStandaloneThread(user, title)` → `ChatThreadDTO`.
- `getChatMessages` — `loadOwnedThread` (below) → `ListMessagesByThread` → `[]ChatMessageDTO`.
- `postChatTurn` — SSE; mirror `postProjectTurn` (`studioturn.go:119-213`) with these changes: resolve+own the thread (not project); entitlement gate; decode `{user_input}`; persist the student message via `CreateChatMessage` + one `AppendEvent(project NULL, user, "chat", "prompt_sent", "{}")`; build `agent.ChatDeps{Store: agent.NewSqlcChatStore(a.d.Queries), Provider: a.d.Provider, Resolved: resolved, UserID: u.ID, ThreadID: threadID}`; `res, err := agent.RunChatStep(ctx, deps, body.UserInput)`; stream `em.Text(res.Reply)` then, if `res.Offer != nil`, `em.Card(res.Offer.CardInstanceID, res.Offer.CardID, "", []byte("[]"), res.Offer.MaterialID)`; `em.Done("")`.
- `submitChatCard` — JSON; `loadOwnedThreadCard`; validate field_values/event_trace/anchors (reuse existing validators); `SubmitThreadCardInstance(id, thread, field_values, event_trace, "completed")`; `AppendEvent(... "chat","card_completed" ...)`; return `{"card_status":"completed"}`.
- `skipChatCard` — JSON; `SetThreadCardInstanceStatus(id, thread, "skipped")`; return `{"card_status":"skipped"}`.

Add ownership helpers mirroring `loadOwnedProject`:

```go
// loadOwnedThread resolves {id} and 404s (not 403) unless it belongs to the caller.
func (a *API) loadOwnedThread(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil { httpx.WriteError(w, r, httpx.ErrNotFound()); return uuid.Nil, false }
	u, _ := UserFromContext(r.Context())
	th, err := a.d.Queries.GetThread(r.Context(), pgtype.UUID{Bytes: id, Valid: true})
	if err != nil || uuidStr(th.UserID) != u.ID.String() { httpx.WriteError(w, r, httpx.ErrNotFound()); return uuid.Nil, false }
	return id, true
}
```

> Read `studioturn.go` (`postProjectTurn`, `studioEmitter`, `startHeartbeat`) and `projectcards.go` (`loadOwnedProjectCard`, the validators) and transcribe the SSE + ownership scaffolding faithfully, changing only project→thread and RunAgentStep→RunChatStep. The chat turn has NO refeed and NO graph_effects.

- [ ] **Step 4: Register routes in `api.go`.**

Next to the existing `mux.Handle("POST /api/v1/projects/{id}/turn", protected(a.postProjectTurn))`:

```go
mux.Handle("GET /api/v1/chat/threads", protected(a.getChatThreads))
mux.Handle("POST /api/v1/chat/threads", protected(a.createChatThread))
mux.Handle("GET /api/v1/chat/threads/{id}/messages", protected(a.getChatMessages))
mux.Handle("POST /api/v1/chat/threads/{id}/turn", protected(a.postChatTurn))
mux.Handle("POST /api/v1/chat/threads/{id}/cards/{cid}/submit", protected(a.submitChatCard))
mux.Handle("POST /api/v1/chat/threads/{id}/cards/{cid}/skip", protected(a.skipChatCard))
```

- [ ] **Step 5: Write the handler tests.**

Create `apps/api/internal/api/chat_test.go` mirroring `assessment` handler tests' harness (testcontainers app, seeded user, cookie). Assert:
- `POST /chat/threads` then `GET /chat/threads` returns the created thread.
- `GET /chat/threads/{other-user-thread}/messages` → 404 (ownership).
- `POST /chat/threads/{id}/turn` with a stub provider: SSE yields a `text` frame (reply) + `done`; a URL message additionally yields a `card` frame; one `llm_call` row recorded (`surface='chat'`, `project_id` NULL). Reuse the stub-provider wiring the studio turn tests use.
- Entitlement gate: with `HasEntitlement` false, `/turn` → JSON `not_entitled` before streaming.
- `submitChatCard` flips the thread card to `completed`; `skipChatCard` → `skipped`.

- [ ] **Step 6: Run the api tests to verify they pass.**

Run: `cd apps/api && CGO_ENABLED=0 go test -p 1 ./internal/api/ -run 'TestChat'`
Expected: PASS.

- [ ] **Step 7: Full api + agent + store packages.**

Run: `cd apps/api && CGO_ENABLED=0 go test -p 1 ./internal/api/ ./internal/agent/ ./internal/store/`
Expected: `ok` for all three.

- [ ] **Step 8: Commit.**

```bash
git add apps/api/internal/api/chat.go apps/api/internal/api/chat_dto.go apps/api/internal/api/api.go apps/api/internal/api/chat_test.go apps/api/internal/api/chat_dto_parity_test.go
git commit -m "feat(refactor2): Slice 11 T5 — chat endpoints (threads/turn SSE/card submit) + DTOs"
```

---

## Task 6: Web client + Zod DTOs

**Files:**
- Create: `packages/contracts/src/chat.ts`; Modify: `packages/contracts/src/index.ts`
- Create: `apps/web/src/api/chat.ts`; Modify: `apps/web/src/api/index.ts`
- Test: `apps/web/src/api/chat.test.ts`

**Interfaces:**
- Consumes: `apiFetch` (`client.ts`), `parseSSE` (`sse.ts`), `API_BASE`.
- Produces:
  - Zod `ChatThread`, `ChatMessage`, `ChatCardOffer` (+ inferred types).
  - `listThreads(): Promise<ChatThread[]>`, `createThread(title?): Promise<ChatThread>`, `getMessages(threadId): Promise<ChatMessage[]>`, `submitChatCard(threadId, cardInstanceId, payload): Promise<void>`, `skipChatCard(threadId, cardInstanceId): Promise<void>`, and `async function* chatTurn(threadId, userInput): AsyncGenerator<ChatTurnEvent>`.
  - `ChatTurnEvent = { type:"reply"; body:string } | { type:"card"; cardInstanceId; cardId; materialId } | { type:"done" } | { type:"error"; code; message }`.

- [ ] **Step 1: Add the Zod DTOs.**

Create `packages/contracts/src/chat.ts`:

```ts
import { z } from "zod";

export const ChatThread = z.object({ id: z.string(), title: z.string(), createdAt: z.string() });
export type ChatThread = z.infer<typeof ChatThread>;

export const ChatMessage = z.object({
  id: z.string(),
  role: z.enum(["user", "assistant", "system"]),
  content: z.string(),
  modality: z.enum(["text", "voice", "file", "image"]),
  createdAt: z.string(),
});
export type ChatMessage = z.infer<typeof ChatMessage>;

export const ChatCardOffer = z.object({ cardInstanceId: z.string(), cardId: z.string(), materialId: z.string() });
export type ChatCardOffer = z.infer<typeof ChatCardOffer>;
```

Export from `packages/contracts/src/index.ts` (mirror how `assessment.ts` is re-exported).

- [ ] **Step 2: Write the failing client test.**

Create `apps/web/src/api/chat.test.ts` mirroring `studioTurn.test.ts`/`assessment.test.ts` (mock `fetch`). Assert: `listThreads` parses an array; `createThread` posts + parses; the SSE `chatTurn` maps a `text` frame → `{type:"reply"}`, a `card` frame → `{type:"card"}`, a `done` frame → `{type:"done"}`; a non-ok response yields `{type:"error"}`.

- [ ] **Step 3: Run it to verify it fails.**

Run: `cd apps/web && npx vitest run src/api/chat.test.ts`
Expected: FAIL (module not found).

- [ ] **Step 4: Implement `apps/web/src/api/chat.ts`.**

```ts
import type { ChatThread, ChatMessage } from "@mind-imprint/contracts";
import { API_BASE, apiFetch } from "./client";
import { parseSSE } from "./sse";

export type ChatTurnEvent =
  | { type: "reply"; body: string }
  | { type: "card"; cardInstanceId: string; cardId: string; materialId: string }
  | { type: "done" }
  | { type: "error"; code: string; message: string };

export function listThreads(): Promise<ChatThread[]> {
  return apiFetch<ChatThread[]>("/api/v1/chat/threads");
}
export function createThread(title = ""): Promise<ChatThread> {
  return apiFetch<ChatThread>("/api/v1/chat/threads", { method: "POST", body: JSON.stringify({ title }) });
}
export function getMessages(threadId: string): Promise<ChatMessage[]> {
  return apiFetch<ChatMessage[]>(`/api/v1/chat/threads/${threadId}/messages`);
}
export function submitChatCard(threadId: string, cardInstanceId: string, payload: { field_values: unknown; event_trace: unknown; anchors: unknown }): Promise<void> {
  return apiFetch<void>(`/api/v1/chat/threads/${threadId}/cards/${cardInstanceId}/submit`, { method: "POST", body: JSON.stringify(payload) });
}
export function skipChatCard(threadId: string, cardInstanceId: string): Promise<void> {
  return apiFetch<void>(`/api/v1/chat/threads/${threadId}/cards/${cardInstanceId}/skip`, { method: "POST" });
}

export async function* chatTurn(threadId: string, userInput: string): AsyncGenerator<ChatTurnEvent> {
  const res = await fetch(`${API_BASE}/api/v1/chat/threads/${threadId}/turn`, {
    method: "POST",
    credentials: "include",
    headers: { "Content-Type": "application/json", Accept: "text/event-stream" },
    body: JSON.stringify({ user_input: userInput }),
  });
  if (!res.ok || !res.body) {
    let code = "internal_error", message = `HTTP ${res.status}`;
    try { const b = await res.json(); if (b?.error) { code = b.error.code ?? code; message = b.error.message ?? message; } } catch { /* non-JSON */ }
    yield { type: "error", code, message };
    return;
  }
  let reply = "";
  for await (const frame of parseSSE(res.body)) {
    let data: any;
    try { data = frame.data ? JSON.parse(frame.data) : {}; } catch { continue; }
    switch (frame.event) {
      case "text": reply += data.delta ?? ""; yield { type: "reply", body: reply }; break;
      case "card": yield { type: "card", cardInstanceId: data.card_instance_id, cardId: data.card_id, materialId: data.material_id ?? "" }; break;
      case "done": yield { type: "done" }; break;
      case "error": yield { type: "error", code: data.error?.code ?? "internal_error", message: data.error?.message ?? "" }; break;
    }
  }
}
```

> Confirm the `text` SSE frame's JSON key for the delta (`gateway.SSEWriter.Text` — likely `{"delta": "..."}`). Match it. Export the new functions from `apps/web/src/api/index.ts`.

- [ ] **Step 5: Run client + contracts tests to verify they pass.**

Run: `cd apps/web && npx vitest run src/api/chat.test.ts` and `cd packages/contracts && npx vitest run`
Expected: PASS / all green.

- [ ] **Step 6: Commit.**

```bash
git add packages/contracts/src/chat.ts packages/contracts/src/index.ts apps/web/src/api/chat.ts apps/web/src/api/index.ts apps/web/src/api/chat.test.ts
git commit -m "feat(refactor2): Slice 11 T6 — chat web client + Zod DTOs"
```

---

## Task 7: Chat surface UI + rail tab + mount

**Files:**
- Create: `apps/web/src/shell/chat/ChatSurface.tsx`, `apps/web/src/shell/chat/ChatContainer.tsx`, `apps/web/src/shell/chat/ChatSurface.test.tsx`
- Modify: `apps/web/src/shell/LeftRail.tsx` (add `chat` tab), `apps/web/src/shell/StudentApp.tsx` (tab type + mount)

**Interfaces:**
- Consumes: `listThreads`, `createThread`, `getMessages`, `chatTurn`, `submitChatCard`, `skipChatCard` (chat.ts); the existing card-runtime component (reused for the in-thread offer).

- [ ] **Step 1: Add the `chat` tab to the rail.**

In `LeftRail.tsx`: extend `TabKey` to include `"chat"`; add a `NAV_ITEMS` entry FIRST (before 课程), label `聊天`, using the binding chat SVG (dc.html line 117):

```tsx
{ key: "chat", label: "聊天", icon: (stroke) => (
  <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke={stroke} strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"><path d="M21 15a2 2 0 01-2 2H7l-4 4V5a2 2 0 012-2h14a2 2 0 012 2z" /></svg>
) },
```

Update the rail comment (line 1–3) to note 聊天's surface now ships.

- [ ] **Step 2: Mount in StudentApp.**

In `StudentApp.tsx`: add `"chat"` to `type Tab`; import `ChatContainer`; render `{tab === "chat" && <ChatContainer />}`. Leave the default tab as `"studio"`.

- [ ] **Step 3: Write the failing surface test.**

Create `apps/web/src/shell/chat/ChatSurface.test.tsx` (mirror `GrowthReport.test.tsx`'s mocking of the api module). Assert:
- Renders the binding disclosure copy: the pill text `计入成长评估`, the subtitle `自由对话 · AI 只提问，不替你下结论`, and the footer `AI 会陪你把想法想深，但不替你得出结论 · 你的对话只属于你`.
- With a mocked `listThreads` returning one thread + `getMessages` returning `[user, assistant]`, both bubbles render.
- Sending a message calls `chatTurn`; a yielded `{type:"reply"}` renders an assistant bubble; a yielded `{type:"card"}` renders the in-thread card offer (assert the card tag/title text appears).
- Empty state (no threads): renders the 新对话 button and no fabricated history.

- [ ] **Step 4: Run it to verify it fails.**

Run: `cd apps/web && npx vitest run src/shell/chat/ChatSurface.test.tsx`
Expected: FAIL (component missing).

- [ ] **Step 5: Implement `ChatContainer.tsx` + `ChatSurface.tsx`.**

`ChatContainer` loads threads (`listThreads`), owns active-thread + messages state, and passes handlers to the presentational `ChatSurface`. `ChatSurface` renders the binding dc.html 527–666 layout with inline styles copied from the design:
- History sidebar (288px): 新对话 button (`createThread` → select it) + `对话历史` list from `listThreads`.
- Thread column: header (Bean avatar placeholder + active title + the binding subtitle + the green `计入成长评估` pill w/ tooltip); chat log (max-width 760px) mapping messages to AI/student rows; the in-thread **card offer** block (dc.html `hasCard`, lines 589–608) rendered when a message carries an offer — accepting mounts the existing card-runtime component (pass card spec by `cardId` + `materialId`), on submit call `submitChatCard`, on dismiss call `skipChatCard`; composer (textarea + binding placeholder + send wired via `chatTurn`; attach/image/voice icons rendered but inert) + the binding footer disclosure.
- All icons inline SVG. On send: append the student bubble optimistically, then consume `chatTurn`, updating the streaming assistant bubble and appending any card offer.

> For the Bean avatar (dc-import in the design), use a simple inline SVG circle placeholder consistent with the shell — do not import a design-tool component. Keep the offer's card runtime reuse minimal: if wiring the full interactive card in-thread is heavy, render the offer preview (tag/title/desc/steps) + an 接受 button that mounts the existing card component the studio uses; a reviewer will check the offer reads as an offer, not an interruption.

- [ ] **Step 6: Run the surface test to verify it passes.**

Run: `cd apps/web && npx vitest run src/shell/chat/ChatSurface.test.tsx`
Expected: PASS.

- [ ] **Step 7: Full web suite + tsc.**

Run: `cd apps/web && npx vitest run && npx tsc --noEmit`
Expected: all tests green; tsc clean (only the pre-existing `interactionPrimitive` error, if still present, is acceptable — confirm no NEW errors).

- [ ] **Step 8: Commit.**

```bash
git add apps/web/src/shell/chat apps/web/src/shell/LeftRail.tsx apps/web/src/shell/StudentApp.tsx
git commit -m "feat(refactor2): Slice 11 T7 — Chat surface UI (binding dc.html), rail tab, mount"
```

---

## Final gate (before whole-branch review)

- `cd apps/api && CGO_ENABLED=0 go test -p 1 ./...` → all packages `ok` (FULL packages, not `-run` subsets — card/gate/projection changes demand it).
- `cd packages/contracts && npx vitest run` → green.
- `cd apps/web && npx vitest run && npx tsc --noEmit` → green; tsc clean (no NEW errors).
- `cd apps/api && make sqlc` → no diff (generated code committed).
- `git status` → only Slice-11 named paths touched; pre-existing user files untouched.
- Whole-branch review (Opus) MANDATORY — point it at this plan's Global Constraints + the spec's DEC-11.x and RL notes.

---

## Self-review notes (author)

- **Spec coverage:** §3 migration → T1; §3 standalone queries → T1; §4 RunChatStep/classifier/coach → T3+T4; §5 reply typed output → T2; §6 endpoints/DTO/client → T5+T6; §7 surface → T7; §8 evidence/disclosure → T5 (event insert) + T7 (binding copy); §9 non-goals → not built (multimodal inert in T7, competence/graph/seeding/off-record absent by construction). §10 tests distributed across tasks.
- **Type consistency:** `ChatStore`, `ChatDeps`, `ChatStepResult`, `ChatCardOffer`, `ThreadMaterial`, `ThreadCard` defined in T4 and consumed in T5; `ChatThreadDTO`/`ChatMessageDTO`/`ChatCardOfferDTO` in T5 mirrored by Zod `ChatThread`/`ChatMessage`/`ChatCardOffer` in T6; `chatTurn` frame mapping (`text`/`card`/`done`/`error`) matches the server's `Text`/`Card`/`Done` emitters in T5.
- **Keystone honesty:** competence NOT written (dormant); no graph_effects on thread card completion; one CRAAP offer per thread (no card→material edge exists); pasted links not fetched (empty blocks; CRAAP self-contained via step-1 link_check).
