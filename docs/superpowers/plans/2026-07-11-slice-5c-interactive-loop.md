# Slice 5c — Interactive Conversational Loop (chat-aware coach) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** A student types in the Studio composer → the message persists to the project's chat thread → the chat-aware coach runs one `RunAgentStep` and streams back one anchored intervention (or a gate check, or silence) over SSE, live into the coach rail → the student can 三键处置 a coach proposal. Conversational loop only — no tool-cards, no migration.

**Architecture:** A thin SSE driver (`POST /projects/{id}/turn`) mirrors the legacy `postTurn` but drives the new `RunAgentStep` (card-surfacing gated off via an additive `SkipSurfaceCards` flag). The coach becomes chat-aware — `ProposeIntervention`/`BuildCoachContext` take the recent thread. The coach's reply IS the intervention row (no duplicate assistant message); the coach's context and the projection thread both merge student `chat_message`s ⋈ interventions by time. Frontend: a `studioTurn` SSE client + a lean conversation controller wire `StudioContainer`'s composer/disposition live.

**Tech Stack:** Go (`net/http`, `pgx`/sqlc/goose, testcontainers, SSE) · TypeScript + React + vitest (`apps/web`).

**Spec:** `docs/superpowers/specs/2026-07-11-slice-5c-interactive-loop-design.md`

## Global Constraints

- **Client never calls the model directly** — the SSE turn goes through the Go gateway; keys server-side only.
- **AI restraint / 一次只问一个:** exactly ONE coach intervention per student turn (one `RunAgentStep`); the coach may stay SILENT (`RunAgentStep` returns `(nil,nil)` → no `intervention` event). No slot-machine mechanics.
- **AI never states conclusions:** the chat-aware coach body still passes the full enforcement stack UNCHANGED (`ValidateOutput` typed-output · `BannedPhrasing` reject · `OutputCheck` declarative-echo→question · authorship). Anchor + criterion come from the classifier candidate, never parsed from the model reply.
- **Single source of truth:** reuse `RunAgentStep`/`ProposeIntervention`/enforcement — do not fork a parallel coach. The SSE endpoint is a thin driver.
- **Additive; legacy untouched:** no schema migration, no `card_instances`/`task_id` change; legacy `postTurn`/`RunTurn`/`turn.go` stays live (retired in 5d). `AgentDeps.SkipSurfaceCards` zero value (`false`) MUST preserve current `RunAgentStep` behavior for every existing caller/test.
- **The coach reply is the intervention row** — 5c does NOT persist an assistant `chat_message` for it (would duplicate). Student turn → `chat_message(role="user")`; coach turn → `intervention` row. Context + projection = student chat_messages ⋈ interventions, time-ordered.
- **Studio discipline:** the new endpoint + frontend controller are additive; do NOT touch `AppShell`/`Root`/old `workspace/`.
- **Icons = inline SVG, never `lucide-react`. Binding Chinese design copy verbatim — fix the test, never the copy.**
- **Commands:** `make sqlc` (`CGO_ENABLED=0 go tool sqlc generate` in `apps/api`); Go tests `go test ./...` / `-short`; web tests `npm test` in `apps/web`; contracts `npm test` in `packages/contracts`. The project-boundary hook BLOCKS shell redirects to `/dev/null`, out-of-repo paths, and the bare token `eval`.

## File Structure

- **`apps/api/internal/store/queries/chat.sql`** (new) — `GetOrCreateThread`, `CreateChatMessage`, `ListChatMessagesByThread`; regenerate sqlc. **`…/store/*_test.go`** round-trip.
- **`apps/api/internal/agent/loop.go`** (modify) — `AgentStore` seam gains `LoadChatHistory`+`CreateChatMessage`; `AgentDeps.SkipSurfaceCards`; `RunAgentStep` loads history + gates card-surfacing. **`agentstore.go`** (modify) — adapter methods. **`ChatTurn`** type.
- **`apps/api/internal/agent/coach.go`** (modify) — `ProposeIntervention` + `BuildCoachContext` take `[]ChatTurn`. Callers updated to `nil`.
- **`apps/api/internal/gateway/sse.go`** (modify) — Studio event methods (`Intervention`/`Gate` + reuse `Done`/`ErrorEnvelope`/`Heartbeat`), or a `StudioSSEEmitter`.
- **`apps/api/internal/api/studioturn.go`** (new) — `postProjectTurn` + a `syncEmitter`-style Studio emitter. **`api.go`** (modify) — 2 routes. **`studioturn_test.go`** (new).
- **`apps/api/internal/api/disposition.go`** (new, or fold into studioturn.go) — `postInterventionDisposition`. **`disposition_test.go`**.
- **`apps/api/internal/studio/projection.go`** (modify — `projectCoach` merge) + **`load.go`** (load chat_messages) + `ProjectData`. **`projection_test.go`** case.
- **`apps/web/src/api/studioTurn.ts`** (new) + **`studioTurn.test.ts`** — SSE client + `StudioTurnEvent` union + `postDisposition`.
- **`apps/web/src/studio/conversation.ts`** (new) + **`conversation.test.ts`** — lean controller (mirrors `agent/createConversation.ts`).
- **`apps/web/src/studio/StudioContainer.tsx`** (modify) + **`StudioContainer.test.tsx`** — wire `onComposerSend`/`onDisposition` live + merge live thread.

---

### Task 1: Chat persistence queries (`chat.sql`) + sqlc + round-trip

**Files:**
- Create: `apps/api/internal/store/queries/chat.sql`
- Regenerate: sqlc (`make sqlc`)
- Test: `apps/api/internal/store/chat_sqlc_test.go`

**Interfaces produced:** `GetThreadByProject`, `CreateThread`, `CreateChatMessage`, `ListChatMessagesByProject` sqlc funcs. Consumed by Tasks 2 (agent adapter) + 8 (projection Load).

**Note:** `chat_message` has NO `project_id` — it joins through `chat_thread.seeded_project_id`. `seeded_project_id` is nullable → its sqlc param is `pgtype.UUID`.

- [ ] **Step 1: Write the failing test** — `chat_sqlc_test.go` (package `store_test` or the existing store test package; testcontainers, `-short`-gated like the other `*_sqlc_test.go`):

```go
func TestChatMessagesRoundTrip(t *testing.T) {
	if testing.Short() { t.Skip("requires postgres") }
	pool := newStoreTestPool(t) // reuse the existing migrated-pool helper
	q := sqlc.New(pool)
	ctx := context.Background()
	// The seed migration 0018 creates project …0101 owned by Phoebe …0003.
	projectID := uuid.MustParse("00000000-0000-0000-0000-000000000101")
	pgProject := pgtype.UUID{Bytes: projectID, Valid: true}

	// No thread yet.
	if _, err := q.GetThreadByProject(ctx, pgProject); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("expected no thread, got %v", err)
	}
	th, err := q.CreateThread(ctx, sqlc.CreateThreadParams{UserID: uuid.MustParse("00000000-0000-0000-0000-000000000003"), SeededProjectID: pgProject})
	if err != nil { t.Fatal(err) }

	for i, body := range []string{"它想证明中国在认真转型", "那要连到哪条主张？"} {
		if _, err := q.CreateChatMessage(ctx, sqlc.CreateChatMessageParams{ThreadID: th.ID, Role: "user", Content: body, Modality: "text"}); err != nil {
			t.Fatalf("msg %d: %v", i, err)
		}
	}
	msgs, err := q.ListChatMessagesByProject(ctx, pgProject)
	if err != nil { t.Fatal(err) }
	if len(msgs) != 2 || msgs[0].Content != "它想证明中国在认真转型" {
		t.Fatalf("messages = %+v", msgs)
	}
	// GetThreadByProject now finds it.
	if got, err := q.GetThreadByProject(ctx, pgProject); err != nil || got.ID != th.ID {
		t.Fatalf("GetThreadByProject = %v, %v", got, err)
	}
}
```

*(Implementer: confirm the real migrated-pool helper name used by the other `internal/store/*_sqlc_test.go` — e.g. `newStoreTestPool` — and reuse it; do not invent a new one.)*

- [ ] **Step 2: Run — verify it fails**

Run: `cd apps/api && go test ./internal/store/ -run TestChatMessagesRoundTrip`
Expected: FAIL — `GetThreadByProject`/`CreateThread`/… undefined.

- [ ] **Step 3: Write `apps/api/internal/store/queries/chat.sql`**

```sql
-- name: GetThreadByProject :one
SELECT * FROM chat_thread WHERE seeded_project_id = $1 LIMIT 1;

-- name: CreateThread :one
INSERT INTO chat_thread (user_id, seeded_project_id) VALUES ($1, $2)
RETURNING *;

-- name: CreateChatMessage :one
INSERT INTO chat_message (thread_id, role, content, modality)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: ListChatMessagesByProject :many
SELECT cm.* FROM chat_message cm
JOIN chat_thread ct ON cm.thread_id = ct.id
WHERE ct.seeded_project_id = $1
ORDER BY cm.created_at, cm.id;
```

- [ ] **Step 4: Regenerate + run**

Run: `cd apps/api && make sqlc && go test ./internal/store/ -run TestChatMessagesRoundTrip`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add apps/api/internal/store/queries/chat.sql apps/api/internal/store/sqlc apps/api/internal/store/chat_sqlc_test.go
git commit -m "feat(store): chat_thread/chat_message queries (project-joined)"
```

---

### Task 2: `AgentStore` seam — `ChatTurn` + `LoadChatHistory` + `CreateChatMessage`

**Files:**
- Modify: `apps/api/internal/agent/loop.go` (the `AgentStore` interface + a `ChatTurn` type)
- Modify: `apps/api/internal/agent/agentstore.go` (adapter methods)
- Modify: `apps/api/internal/agent/loop_test.go` (the `fakeAgentStore` gains the two methods)
- Test: `apps/api/internal/agent/agentstore_chat_sqlc_test.go`

**Interfaces produced:** `ChatTurn{Role, Content string}`; seam methods `CreateChatMessage(ctx, projectID uuid.UUID, role, content string) error` and `LoadChatHistory(ctx, projectID uuid.UUID, limit int) ([]ChatTurn, error)`. Consumed by Tasks 3/4 (coach + loop) and Task 6 (endpoint persists the student msg).

**Design:** `LoadChatHistory` MERGES student `chat_message`s (role "user") + interventions (role "assistant", `Body` as content), ordered by `created_at`, capped to the last `limit`. `CreateChatMessage` resolves-or-creates the project's thread (get-then-create).

- [ ] **Step 1: Write the failing test** — `agentstore_chat_sqlc_test.go` (package `agent`, testcontainers, `-short`-gated):

```go
func TestSqlcAgentStore_ChatHistoryMerge(t *testing.T) {
	if testing.Short() { t.Skip("requires postgres") }
	pool := newAgentTestPool(t) // reuse the existing agent sqlc-test pool helper
	q := sqlc.New(pool)
	store := NewSqlcAgentStore(q)
	ctx := context.Background()
	projectID := uuid.MustParse("00000000-0000-0000-0000-000000000101") // seeded

	if err := store.CreateChatMessage(ctx, projectID, "user", "它想证明中国在认真转型"); err != nil {
		t.Fatal(err)
	}
	// The seed already has 2 interventions (…0120 flag, …0121 diagnostic) for this project.
	hist, err := store.LoadChatHistory(ctx, projectID, 12)
	if err != nil { t.Fatal(err) }
	// Merged: 2 seeded interventions (assistant) + 1 new user turn, time-ordered.
	if len(hist) < 3 { t.Fatalf("history len = %d, want >=3", len(hist)) }
	last := hist[len(hist)-1]
	if last.Role != "user" || last.Content != "它想证明中国在认真转型" {
		t.Fatalf("last turn = %+v", last)
	}
	// A second CreateChatMessage must reuse the same thread (idempotent thread).
	if err := store.CreateChatMessage(ctx, projectID, "user", "第二句"); err != nil { t.Fatal(err) }
	th, err := q.GetThreadByProject(ctx, pgUUID(projectID))
	if err != nil { t.Fatalf("thread not reused: %v", err) }
	msgs, _ := q.ListChatMessagesByProject(ctx, pgUUID(projectID))
	if len(msgs) != 2 { t.Fatalf("want 2 user msgs on one thread, got %d (thread %s)", len(msgs), th.ID) }
}
```

*(Reuse the existing agent sqlc-test pool helper + a `pgUUID` helper if present; else `pgtype.UUID{Bytes: id, Valid: true}`.)*

- [ ] **Step 2: Run — verify it fails**

Run: `cd apps/api && go test ./internal/agent/ -run TestSqlcAgentStore_ChatHistoryMerge`
Expected: FAIL — methods undefined.

- [ ] **Step 3: Add `ChatTurn` + extend the `AgentStore` interface** (`loop.go`)

```go
// ChatTurn is one turn of the coach's conversational context: the student's
// message (role "user") or a prior coach intervention (role "assistant").
type ChatTurn struct {
	Role    string // "user" | "assistant"
	Content string
}
```
Add to the `AgentStore` interface:
```go
	CreateChatMessage(ctx context.Context, projectID uuid.UUID, role, content string) error
	LoadChatHistory(ctx context.Context, projectID uuid.UUID, limit int) ([]ChatTurn, error)
```

- [ ] **Step 4: Implement the adapter methods** (`agentstore.go`, mirroring `AppendEvent`'s project→user resolution)

```go
func (s *sqlcAgentStore) getOrCreateThread(ctx context.Context, projectID uuid.UUID) (uuid.UUID, error) {
	pg := pgtype.UUID{Bytes: projectID, Valid: true}
	th, err := s.q.GetThreadByProject(ctx, pg)
	if err == nil {
		return th.ID, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return uuid.UUID{}, err
	}
	project, err := s.q.GetProject(ctx, projectID)
	if err != nil {
		return uuid.UUID{}, err
	}
	created, err := s.q.CreateThread(ctx, sqlc.CreateThreadParams{UserID: project.UserID, SeededProjectID: pg})
	if err != nil {
		return uuid.UUID{}, err
	}
	return created.ID, nil
}

func (s *sqlcAgentStore) CreateChatMessage(ctx context.Context, projectID uuid.UUID, role, content string) error {
	threadID, err := s.getOrCreateThread(ctx, projectID)
	if err != nil {
		return err
	}
	_, err = s.q.CreateChatMessage(ctx, sqlc.CreateChatMessageParams{ThreadID: threadID, Role: role, Content: content, Modality: "text"})
	return err
}

// LoadChatHistory merges student chat_messages + prior interventions into a
// single time-ordered conversation, capped to the most-recent `limit`.
func (s *sqlcAgentStore) LoadChatHistory(ctx context.Context, projectID uuid.UUID, limit int) ([]ChatTurn, error) {
	pg := pgtype.UUID{Bytes: projectID, Valid: true}
	msgs, err := s.q.ListChatMessagesByProject(ctx, pg)
	if err != nil {
		return nil, err
	}
	ivs, err := s.q.ListInterventionsByProject(ctx, projectID)
	if err != nil {
		return nil, err
	}
	type stamped struct {
		at   time.Time
		turn ChatTurn
	}
	var all []stamped
	for _, m := range msgs {
		all = append(all, stamped{at: m.CreatedAt, turn: ChatTurn{Role: m.Role, Content: m.Content}})
	}
	for _, iv := range ivs {
		all = append(all, stamped{at: iv.CreatedAt, turn: ChatTurn{Role: "assistant", Content: iv.Body}})
	}
	sort.Slice(all, func(i, j int) bool { return all[i].at.Before(all[j].at) })
	if limit > 0 && len(all) > limit {
		all = all[len(all)-limit:]
	}
	out := make([]ChatTurn, len(all))
	for i, s := range all {
		out[i] = s.turn
	}
	return out, nil
}
```

*(Implementer: verify `sqlc.ChatMessage.CreatedAt`/`sqlc.Intervention.CreatedAt` are `time.Time`; verify `ListInterventionsByProject` param type. Add imports `errors`, `sort`, `time`, `github.com/jackc/pgx/v5`.)*

- [ ] **Step 5: Add the two methods to `fakeAgentStore` in `loop_test.go`** (so the package compiles) — a minimal in-memory impl (append to a slice; `LoadChatHistory` returns the stored user turns; interventions merge optional for the fake). Keep existing fake behavior intact.

- [ ] **Step 6: Run — verify pass**

Run: `cd apps/api && go test ./internal/agent/ -run TestSqlcAgentStore_ChatHistoryMerge` then `go test ./internal/agent/`
Expected: PASS (new test + all existing agent tests, since the interface grew and the fake now implements it).

- [ ] **Step 7: Commit**

```bash
git add apps/api/internal/agent/loop.go apps/api/internal/agent/agentstore.go apps/api/internal/agent/loop_test.go apps/api/internal/agent/agentstore_chat_sqlc_test.go
git commit -m "feat(agent): chat-history seam (ChatTurn, LoadChatHistory, CreateChatMessage)"
```

---

### Task 3: Chat-aware coach (`ProposeIntervention` + `BuildCoachContext`)

**Files:**
- Modify: `apps/api/internal/agent/coach.go` (`ProposeIntervention` signature)
- Modify: `apps/api/internal/agent/coach_prompt.go` (`BuildCoachContext` signature + body)
- Modify: callers of `ProposeIntervention`/`BuildCoachContext` (in `loop.go` — done in Task 4; and any tests) to pass history
- Test: `apps/api/internal/agent/coach_test.go` (add cases)

**Interfaces produced:** `ProposeIntervention(ctx, prov, r, g, c, history []ChatTurn, sim)`; `BuildCoachContext(g, c, history []ChatTurn) string`. Consumed by Task 4 (`RunAgentStep`).

- [ ] **Step 1: Write the failing test** — add to `coach_test.go`:

```go
func TestBuildCoachContext_IncludesChatHistory(t *testing.T) {
	g := GraphView{Nodes: []GraphNodeView{{ID: "n1", Type: "claim", Author: "student", Text: "中国有治理决心"}}}
	c := Candidate{Verb: "post_intervention", AnchorKind: "graph_node", AnchorID: "n1", Criterion: "D5", Reason: "裸主张", Level: "I2"}
	history := []ChatTurn{{Role: "user", Content: "它想证明中国在认真转型"}, {Role: "assistant", Content: "那要连到哪条主张？"}}
	ctxStr := BuildCoachContext(g, c, history)
	if !strings.Contains(ctxStr, "它想证明中国在认真转型") {
		t.Fatalf("coach context missing the student turn:\n%s", ctxStr)
	}
	// Empty history is transparent (no history section / no crash).
	_ = BuildCoachContext(g, c, nil)
}
```

- [ ] **Step 2: Run — verify it fails**

Run: `cd apps/api && go test ./internal/agent/ -run TestBuildCoachContext_IncludesChatHistory`
Expected: FAIL — `BuildCoachContext` takes 2 args (compile error).

- [ ] **Step 3: Extend `BuildCoachContext`** (`coach_prompt.go`) — add `history []ChatTurn` param; when non-empty, prepend a `# 对话记录` section BEFORE the existing anchor/edges sections:

```go
func BuildCoachContext(g GraphView, c Candidate, history []ChatTurn) string {
	var b strings.Builder
	if len(history) > 0 {
		b.WriteString("# 对话记录（最近在前为旧、在后为新）\n")
		for _, t := range history {
			who := "学生"
			if t.Role == "assistant" {
				who = "教练"
			}
			fmt.Fprintf(&b, "%s：%s\n", who, t.Content)
		}
		b.WriteString("\n")
	}
	// ... existing sections (锚点节点 / 相关边 / 为什么此刻需要介入 / CT 维度) UNCHANGED ...
}
```
*(Implementer: keep the existing section-building code verbatim after the new block; only prepend the history section.)*

- [ ] **Step 4: Extend `ProposeIntervention`** (`coach.go`) — add `history []ChatTurn` param, pass it to `BuildCoachContext`:

```go
func ProposeIntervention(ctx context.Context, prov gateway.Provider, r gateway.Resolved, g GraphView, c Candidate, history []ChatTurn, sim enforcement.Similarity) (enforcement.AgentOutput, string, error) {
	req := gateway.ChatRequest{
		Messages: []gateway.ChatMessage{
			{Role: gateway.RoleSystem, Content: coachPosturePrompt},
			{Role: gateway.RoleUser, Content: BuildCoachContext(g, c, history)},
		},
	}
	// ... rest UNCHANGED ...
```

- [ ] **Step 5: Update the existing `ProposeIntervention` call in `loop.go`** to pass `nil` for now (Task 4 replaces `nil` with real history) — this keeps the package compiling. Also update any other test callers of `BuildCoachContext`/`ProposeIntervention` to pass `nil`.

- [ ] **Step 6: Run — verify pass**

Run: `cd apps/api && go test ./internal/agent/`
Expected: PASS (new test + all existing; the enforcement/coach tests still pass since empty history is transparent).

- [ ] **Step 7: Commit**

```bash
git add apps/api/internal/agent/coach.go apps/api/internal/agent/coach_prompt.go apps/api/internal/agent/loop.go apps/api/internal/agent/coach_test.go
git commit -m "feat(agent): chat-aware coach — BuildCoachContext/ProposeIntervention take thread history"
```

---

### Task 4: `RunAgentStep` — `SkipSurfaceCards` + load + thread chat history

**Files:**
- Modify: `apps/api/internal/agent/loop.go` (`AgentDeps` field; `RunAgentStep` body)
- Test: `apps/api/internal/agent/loop_test.go` (add cases)

**Interfaces produced:** `AgentDeps.SkipSurfaceCards bool`. Consumed by Task 6 (endpoint sets `true`).

- [ ] **Step 1: Write the failing test** — add to `loop_test.go` (using the existing `fakeAgentStore`/fake provider):

```go
func TestRunAgentStep_SkipSurfaceCards(t *testing.T) {
	// A graph state that WOULD surface a card.
	g := graphThatSurfacesACard() // reuse whatever existing test sets up a surface_card candidate
	deps := newFakeDeps(g)        // fake store + provider + stub sim

	// Zero value (SkipSurfaceCards=false) → existing behavior: a surface_card action.
	act, err := RunAgentStep(context.Background(), deps, testProjectID, Trigger{Kind: "T-A"})
	if err != nil { t.Fatal(err) }
	if act == nil || act.Kind != "surface_card" {
		t.Fatalf("default: want surface_card, got %+v", act)
	}

	// SkipSurfaceCards=true → the card path is skipped; yields post_intervention or silence.
	deps.SkipSurfaceCards = true
	act, err = RunAgentStep(context.Background(), deps, testProjectID, Trigger{Kind: "student_turn"})
	if err != nil { t.Fatal(err) }
	if act != nil && act.Kind == "surface_card" {
		t.Fatalf("SkipSurfaceCards: got a surface_card action %+v", act)
	}
}
```

*(Implementer: adapt to the existing loop-test fixtures — reuse the fixture/helpers a current surface_card test uses; if none isolates surface_card, construct a minimal graph with a card trigger. The key assertions: default yields surface_card, flag set yields non-card.)*

- [ ] **Step 2: Run — verify it fails**

Run: `cd apps/api && go test ./internal/agent/ -run TestRunAgentStep_SkipSurfaceCards`
Expected: FAIL — `SkipSurfaceCards` undefined.

- [ ] **Step 3: Add the field + gate the candidate + thread history** (`loop.go`)

Add to `AgentDeps`: `SkipSurfaceCards bool // when true, RunAgentStep does not produce surface_card candidates (5c conversational loop)`.

In `RunAgentStep`, change the candidate seed (line ~124) and thread history into the coach:
```go
	var cands []Candidate
	if !deps.SkipSurfaceCards {
		cands = SurfaceCardCandidates(g)
	}
	// ... existing gate/checkGate + CandidateMoves + observe assembly UNCHANGED ...

	// (in the post_intervention dispatch, load history and pass it)
	history, err := deps.Store.LoadChatHistory(ctx, projectID, 12)
	if err != nil {
		history = nil // history is best-effort; never fail the turn on it
	}
	out, verdict, err := ProposeIntervention(ctx, deps.Provider, deps.Resolved, g, c, history, deps.Sim)
```
*(Implementer: place the `LoadChatHistory` call just before the `ProposeIntervention` call inside the post_intervention branch — not on the surface_card/check_gate branches. Keep everything else verbatim.)*

- [ ] **Step 4: Run — verify pass**

Run: `cd apps/api && go test ./internal/agent/`
Expected: PASS (new test + all existing — zero-value `SkipSurfaceCards` preserves behavior; `LoadChatHistory` on the fake returns its stored turns or empty).

- [ ] **Step 5: Commit**

```bash
git add apps/api/internal/agent/loop.go apps/api/internal/agent/loop_test.go
git commit -m "feat(agent): RunAgentStep gates surface_card (SkipSurfaceCards) + threads chat history to coach"
```

---

### Task 5: Studio SSE event methods (gateway)

**Files:**
- Modify: `apps/api/internal/gateway/sse.go` (add `Intervention` + `Gate` methods)
- Test: `apps/api/internal/gateway/sse_test.go` (add cases; create if absent)

**Interfaces produced:** `(*SSEWriter).Intervention(interventionID, body, anchor, criterion, level string) error` and `.Gate(contract, status string, passed, total int, missing []string) error`. Consumed by Task 6. (`Done`/`ErrorEnvelope`/`Heartbeat` reused as-is.)

- [ ] **Step 1: Write the failing test** — assert the wire format via an `httptest.ResponseRecorder`:

```go
func TestSSEWriter_StudioEvents(t *testing.T) {
	rec := httptest.NewRecorder()
	w, err := NewSSEWriter(rec)
	if err != nil { t.Fatal(err) }
	if err := w.Intervention("iid-1", "把它连到治理决心", "论证图 · 治理决心主张", "D5", "I2"); err != nil { t.Fatal(err) }
	if err := w.Gate("build_argument", "partial", 2, 7, []string{"concession 待完成"}); err != nil { t.Fatal(err) }
	body := rec.Body.String()
	for _, want := range []string{
		"event: intervention", `"intervention_id":"iid-1"`, `"criterion":"D5"`, `"anchor":"论证图 · 治理决心主张"`,
		"event: gate", `"contract":"build_argument"`, `"passed":2`, `"total":7`,
	} {
		if !strings.Contains(body, want) { t.Fatalf("missing %q in:\n%s", want, body) }
	}
}
```

- [ ] **Step 2: Run — verify it fails**

Run: `cd apps/api && go test ./internal/gateway/ -run TestSSEWriter_StudioEvents`
Expected: FAIL — methods undefined.

- [ ] **Step 3: Add the methods** (`sse.go`, mirroring `Text`/`Card`):

```go
// Intervention emits one coach intervention (Slice 5c Studio turn).
func (s *SSEWriter) Intervention(interventionID, body, anchor, criterion, level string) error {
	return s.writeEvent("intervention", map[string]any{
		"intervention_id": interventionID,
		"body":            body,
		"anchor":          anchor,
		"criterion":       criterion,
		"level":           level,
	})
}

// Gate emits a gate-check result.
func (s *SSEWriter) Gate(contract, status string, passed, total int, missing []string) error {
	if missing == nil {
		missing = []string{}
	}
	return s.writeEvent("gate", map[string]any{
		"contract": contract, "status": status, "passed": passed, "total": total, "missing": missing,
	})
}
```

- [ ] **Step 4: Run — verify pass**

Run: `cd apps/api && go test ./internal/gateway/`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add apps/api/internal/gateway/sse.go apps/api/internal/gateway/sse_test.go
git commit -m "feat(gateway): SSE intervention/gate events for the Studio turn"
```

---

### Task 6: Studio turn endpoint + loop driver (`POST /projects/{id}/turn`)

**Files:**
- Create: `apps/api/internal/api/studioturn.go` (`postProjectTurn` + a Studio `syncEmitter`)
- Modify: `apps/api/internal/api/api.go` (register the route)
- Test: `apps/api/internal/api/studioturn_test.go`

**Interfaces:** consumes `loadOwnedProject`, `a.d.{Queries,Provider,ChatResolver,SpecByID}`, `agent.RunAgentStep`/`NewSqlcAgentStore`/`ChatTurn`, `gateway.NewSSEWriter`, `skills.ByID`. Produces `POST /api/v1/projects/{id}/turn` (SSE).

- [ ] **Step 1: Write the failing test** — `studioturn_test.go` (`api_test`, testcontainers):

```go
func TestProjectTurn(t *testing.T) {
	pool := newAPITestPool(t)
	h := New(Deps{Queries: sqlc.New(pool), Pool: pool, Provider: fakeProvider(), ChatResolver: fakeResolver(), SpecByID: cardsByID()}).Handler()
	cookie := signInSeed(t, pool) // Phoebe
	projectID := "00000000-0000-0000-0000-000000000101"

	// 401 unauth.
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest("POST", "/api/v1/projects/"+projectID+"/turn", strings.NewReader(`{"user_input":"hi there enough"}`)))
	if rr.Code != 401 { t.Fatalf("unauth: want 401, got %d", rr.Code) }

	// happy path — SSE with a done event, and a persisted chat_message.
	rr = httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/v1/projects/"+projectID+"/turn", strings.NewReader(`{"user_input":"它想证明中国在认真转型呢"}`))
	h.ServeHTTP(rr, withCookie(req, cookie))
	if rr.Code != 200 { t.Fatalf("turn: %d — %s", rr.Code, rr.Body.String()) }
	if !strings.Contains(rr.Body.String(), "event: done") {
		t.Fatalf("stream missing done:\n%s", rr.Body.String())
	}
	msgs, _ := sqlc.New(pool).ListChatMessagesByProject(context.Background(), pgUUID(uuid.MustParse(projectID)))
	if len(msgs) < 1 { t.Fatalf("student message not persisted") }

	// 404 non-owned.
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, withCookie(httptest.NewRequest("POST", "/api/v1/projects/00000000-0000-0000-0000-0000000009ff/turn", strings.NewReader(`{"user_input":"whatever long enough"}`)), cookie))
	if rr.Code != 404 { t.Fatalf("foreign: want 404, got %d", rr.Code) }
}
```

*(Implementer: build a `fakeProvider` (a `gateway.Provider` whose `Stream` emits a short text then closes) + `fakeResolver` (a `gateway.KeyResolver` func returning a dummy `Resolved`) + a stub `enforcement.Similarity` — reuse whatever the existing `agent` sqlc integration tests use for these; if they're unexported test helpers, add small local ones in `api_test`. The coach may legitimately stay silent for the seeded graph — assert on `event: done` (always emitted) + the persisted message, not necessarily on `event: intervention`.)*

- [ ] **Step 2: Run — verify it fails**

Run: `cd apps/api && go test ./internal/api/ -run TestProjectTurn`
Expected: FAIL — route 404 / handler undefined.

- [ ] **Step 3: Write `studioturn.go`** — a Studio `syncEmitter` (mutex over `*gateway.SSEWriter` exposing `Intervention`/`Gate`/`Done`/`ErrorEnvelope`/`Heartbeat`) + the handler:

```go
package api

import (
	"context"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/google/uuid"

	"mindimprint/api/internal/agent"
	"mindimprint/api/internal/agent/enforcement"
	"mindimprint/api/internal/gateway"
	"mindimprint/api/internal/httpx"
	"mindimprint/api/internal/skills"
)

type studioEmitter struct {
	mu  sync.Mutex
	sse *gateway.SSEWriter
}

func (e *studioEmitter) Intervention(id, body, anchor, criterion, level string) error {
	e.mu.Lock(); defer e.mu.Unlock(); return e.sse.Intervention(id, body, anchor, criterion, level)
}
func (e *studioEmitter) Gate(contract, status string, passed, total int, missing []string) error {
	e.mu.Lock(); defer e.mu.Unlock(); return e.sse.Gate(contract, status, passed, total, missing)
}
func (e *studioEmitter) Done() error { e.mu.Lock(); defer e.mu.Unlock(); return e.sse.Done("") }
func (e *studioEmitter) ErrorEnvelope(code, msg string) error {
	e.mu.Lock(); defer e.mu.Unlock(); return e.sse.ErrorEnvelope(code, msg)
}
func (e *studioEmitter) Heartbeat() error { e.mu.Lock(); defer e.mu.Unlock(); return e.sse.Heartbeat() }

func (a *API) postProjectTurn(w http.ResponseWriter, r *http.Request) {
	projectID, ok := a.loadOwnedProject(w, r)
	if !ok { return }
	u, _ := UserFromContext(r.Context())

	entitled, err := HasEntitlement(r.Context(), u)
	if err != nil { httpx.WriteError(w, r, err); return }
	if !entitled { httpx.WriteError(w, r, httpx.ErrNotEntitled()); return }

	var body struct{ UserInput string `json:"user_input"` }
	if err := decodeJSON(r, &body); err != nil { httpx.WriteError(w, r, err); return }
	if len(body.UserInput) == 0 { httpx.WriteError(w, r, httpx.ErrValidation("user_input 不能为空")); return }

	resolved, err := a.d.ChatResolver(r.Context())
	if err != nil { httpx.WriteError(w, r, httpx.ErrInternal()); return }

	sse, err := gateway.NewSSEWriter(w)
	if err != nil { httpx.WriteError(w, r, httpx.ErrInternal()); return }
	em := &studioEmitter{sse: sse}

	// heartbeat goroutine — clone turn.go's shape
	stop := make(chan struct{}); hbDone := make(chan struct{})
	go func() {
		defer close(hbDone)
		ticker := time.NewTicker(15 * time.Second); defer ticker.Stop()
		for {
			select {
			case <-stop: return
			case <-r.Context().Done(): return
			case <-ticker.C: _ = em.Heartbeat()
			}
		}
	}()
	defer func() { close(stop); <-hbDone }()

	store := agent.NewSqlcAgentStore(a.d.Queries)
	if err := store.CreateChatMessage(r.Context(), projectID, "user", body.UserInput); err != nil {
		slog.Error("studio turn: persist student message", "err", err, "request_id", httpx.RequestIDFromContext(r.Context()))
		_ = em.ErrorEnvelope("internal_error", "对话处理失败，请重试"); _ = em.Done(); return
	}
	_ = store.AppendEvent(r.Context(), agent.EventRow{ProjectID: projectID, Surface: "studio", Type: "prompt_sent", Payload: []byte(`{}`)})

	sk, _ := skills.ByID("writing-project")
	deps := agent.AgentDeps{
		Store: store, Provider: a.d.Provider, Resolved: resolved,
		Sim: studioSimilarity(), Skill: &sk, SkipSurfaceCards: true,
	}
	action, err := agent.RunAgentStep(r.Context(), deps, projectID, agent.Trigger{Kind: "student_turn"})
	if err != nil {
		slog.Error("studio turn: RunAgentStep", "err", err, "request_id", httpx.RequestIDFromContext(r.Context()))
		_ = em.ErrorEnvelope("internal_error", "对话处理失败，请重试"); _ = em.Done(); return
	}
	switch {
	case action == nil: // silence
	case action.Kind == "intervention":
		// anchor is sent EMPTY: live interventions store a {kind,id} anchor (not
		// the seed's {label}), so there is no clean display label — the client
		// shows the criterion chip only. (Deriving/storing a live anchor label is
		// a deferred polish; see notes.) This matches what projectCoach produces
		// for these interventions on reload (anchorLabel("{kind,id}") == "").
		_ = em.Intervention(action.InterventionID, action.Output.Body, "", action.Output.Criterion, "")
	case action.Kind == "check_gate" && action.GateReport != nil:
		gr := action.GateReport
		_ = em.Gate(gr.Contract, gr.Status, 0, 0, gr.Missing) // passed/total from the report if available
	}
	_ = em.Done()
}
```

*(Implementer notes: (1) **anchor "" is deliberate** — do NOT send `action.Output.Anchor.ID` (it's a node UUID; the rail would render "锚定 <uuid>"). A meaningful live anchor label (resolving the node text, or storing a label at intervention-mint time) is a deferred polish; for 5c the criterion chip carries the signal and the empty anchor is consistent with the reload projection. (2) **Similarity — DECIDED (keyless lexical, embeddings deferred to a later task):** the endpoint's `deps.Sim` is a real, exported, **keyless lexical-overlap** `enforcement.Similarity` (no key, no provider, no network — computes a score from shared-token/character overlap). First check the `enforcement` package for an existing heuristic impl the tests use; if it's an unexported test stub, add a small **exported, unit-tested** heuristic (e.g. `enforcement.LexicalSimilarity`) in the enforcement package and use it here. Do NOT call any embedding API. (Embedding-based similarity is a separate later task once an embedding provider is chosen.) (3) `httpx.ErrValidation`/`ErrInternal`/`ErrNotEntitled`/`ErrNotFound` — use the ACTUAL constructors in `httpx/errors.go` (the map confirms `ErrInternal`/`ErrNotEntitled`/`ErrNotFound`; for the empty-`user_input` 400, use the same validation error `decodeJSON` produces — check its error path — rather than an assumed `ErrValidation` name). (4) `level` isn't on `Action`; pass `""` (client tolerates empty).)*

- [ ] **Step 4: Register the route** (`api.go`, beside the other project routes):

```go
	mux.Handle("POST /api/v1/projects/{id}/turn", protected(a.postProjectTurn))
```

- [ ] **Step 5: Run — verify pass**

Run: `cd apps/api && go test ./internal/api/ -run TestProjectTurn`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add apps/api/internal/api/studioturn.go apps/api/internal/api/api.go apps/api/internal/api/studioturn_test.go
git commit -m "feat(api): POST /projects/{id}/turn — SSE loop driver (one RunAgentStep, cards off)"
```

---

### Task 7: Disposition endpoint (`POST /projects/{id}/interventions/{iid}/disposition`)

**Files:**
- Create: `apps/api/internal/api/disposition.go`
- Modify: `apps/api/internal/api/api.go` (route)
- Test: `apps/api/internal/api/disposition_test.go`

**Interfaces:** consumes `loadOwnedProject`, `agent.RecordDisposition`/`NewSqlcAgentStore`. Produces `POST /api/v1/projects/{id}/interventions/{iid}/disposition`.

- [ ] **Step 1: Write the failing test** — `disposition_test.go` (`api_test`, testcontainers). Use the seeded intervention `…0121`:

```go
func TestInterventionDisposition(t *testing.T) {
	pool := newAPITestPool(t)
	h := New(Deps{Queries: sqlc.New(pool), Pool: pool}).Handler()
	cookie := signInSeed(t, pool)
	base := "/api/v1/projects/00000000-0000-0000-0000-000000000101/interventions/00000000-0000-0000-0000-000000000121/disposition"

	// happy path (≥15 runes) → 204.
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, withCookie(httptest.NewRequest("POST", base, strings.NewReader(`{"action":"accept","reason":"这条我接受，因为它把证据连回了主张"}`)), cookie))
	if rr.Code != 204 { t.Fatalf("happy: want 204, got %d — %s", rr.Code, rr.Body.String()) }

	// short reason → 400.
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, withCookie(httptest.NewRequest("POST", base, strings.NewReader(`{"action":"reject","reason":"太短"}`)), cookie))
	if rr.Code != 400 { t.Fatalf("short: want 400, got %d", rr.Code) }

	// non-owned project → 404.
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, withCookie(httptest.NewRequest("POST", "/api/v1/projects/00000000-0000-0000-0000-0000000009ff/interventions/00000000-0000-0000-0000-000000000121/disposition", strings.NewReader(`{"action":"accept","reason":"这条我接受因为理由足够长了"}`)), cookie))
	if rr.Code != 404 { t.Fatalf("foreign: want 404, got %d", rr.Code) }
}
```

- [ ] **Step 2: Run — verify it fails**

Run: `cd apps/api && go test ./internal/api/ -run TestInterventionDisposition`
Expected: FAIL — route 404.

- [ ] **Step 3: Write `disposition.go`**

```go
package api

import (
	"net/http"

	"github.com/google/uuid"

	"mindimprint/api/internal/agent"
	"mindimprint/api/internal/httpx"
)

func (a *API) postInterventionDisposition(w http.ResponseWriter, r *http.Request) {
	if _, ok := a.loadOwnedProject(w, r); !ok { return }
	iid, err := uuid.Parse(r.PathValue("iid"))
	if err != nil { httpx.WriteError(w, r, httpx.ErrNotFound("资源不存在")); return }

	var body struct {
		Action string `json:"action"`
		Reason string `json:"reason"`
	}
	if err := decodeJSON(r, &body); err != nil { httpx.WriteError(w, r, err); return }

	deps := agent.AgentDeps{Store: agent.NewSqlcAgentStore(a.d.Queries)}
	if err := agent.RecordDisposition(r.Context(), deps, iid, body.Action, body.Reason); err != nil {
		httpx.WriteError(w, r, httpx.ErrValidation("处置理由至少 15 个字")) // rune-count guard
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
```
*(Implementer: `r.PathValue("iid")` requires the route param be named `{iid}`. Confirm `httpx.ErrValidation` (400) constructor name; if RecordDisposition's error is only the rune guard here, mapping it to 400 is correct — but distinguish a DB error (500) if easy. Optionally validate `action ∈ {accept,reject,rewrite}` before calling.)*

- [ ] **Step 4: Register the route** (`api.go`):

```go
	mux.Handle("POST /api/v1/projects/{id}/interventions/{iid}/disposition", protected(a.postInterventionDisposition))
```

- [ ] **Step 5: Run — verify pass + full Go suite**

Run: `cd apps/api && go test ./internal/api/ -run TestInterventionDisposition && go build ./... && go vet ./... && go test -short ./...`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add apps/api/internal/api/disposition.go apps/api/internal/api/api.go apps/api/internal/api/disposition_test.go
git commit -m "feat(api): POST /projects/{id}/interventions/{iid}/disposition -> RecordDisposition"
```

---

### Task 8: Projection extension — merge student `chat_message`s into the coach thread

**Files:**
- Modify: `apps/api/internal/studio/projection.go` (`ProjectData` + `projectCoach`)
- Modify: `apps/api/internal/studio/load.go` (load chat messages)
- Test: `apps/api/internal/studio/projection_test.go` (add a merge case)

**Interfaces:** `ProjectData.ChatMessages []sqlc.ChatMessage`; `projectCoach` now interleaves student bubbles. Consumed by the `getProject` endpoint (reload shows both sides).

- [ ] **Step 1: Write the failing test** — add to `projection_test.go`:

```go
func TestProjectCoach_MergesStudentMessages(t *testing.T) {
	t1 := time.Now()
	d := ProjectData{
		Interventions: []sqlc.Intervention{
			{Body: "图上有一处孤儿证据", Anchor: []byte(`{"label":"孤儿证据"}`), Type: "flag", CreatedAt: t1},
		},
		ChatMessages: []sqlc.ChatMessage{
			{Role: "user", Content: "它想证明中国在认真转型", CreatedAt: t1.Add(time.Second)},
		},
	}
	coach := projectCoach(d, "论证构建")
	if len(coach.Messages) != 2 {
		t.Fatalf("want 2 messages, got %d", len(coach.Messages))
	}
	// time order: flag first, then the student bubble.
	if coach.Messages[0].Kind != "flag" || coach.Messages[1].Kind != "student" || coach.Messages[1].Body != "它想证明中国在认真转型" {
		t.Fatalf("merged = %+v", coach.Messages)
	}
}
```

- [ ] **Step 2: Run — verify it fails**

Run: `cd apps/api && go test ./internal/studio/ -run TestProjectCoach_MergesStudentMessages`
Expected: FAIL — `ProjectData` has no `ChatMessages` / `projectCoach` ignores them.

- [ ] **Step 3: Implement** — add `ChatMessages []sqlc.ChatMessage` to `ProjectData`; rewrite `projectCoach` to build a time-ordered merge of interventions (existing flag/ai mapping) + chat_messages (`role=="user"` → `{kind:"student", body: content}`), sorted by `created_at`. The anchor stays the latest intervention's label (unchanged). In `load.go`, add `chats, err := q.ListChatMessagesByProject(ctx, pg)` and set `d.ChatMessages = chats`.

*(Implementer: keep the existing intervention→flag/ai mapping verbatim; only add the chat_message interleaving + the sort. Use a stamped-slice sort like the agent `LoadChatHistory`.)*

- [ ] **Step 4: Run — verify pass**

Run: `cd apps/api && go test ./internal/studio/`
Expected: PASS (new merge test + the existing projection/round-trip tests; the seeded round-trip now also carries any chat messages — none seeded, so unaffected).

- [ ] **Step 5: Commit**

```bash
git add apps/api/internal/studio/projection.go apps/api/internal/studio/load.go apps/api/internal/studio/projection_test.go
git commit -m "feat(studio): projectCoach merges student chat messages into the thread"
```

---

### Task 9: Frontend — `api/studioTurn.ts` (SSE client) + `postDisposition`

**Files:**
- Create: `apps/web/src/api/studioTurn.ts`
- Test: `apps/web/src/api/studioTurn.test.ts`

**Interfaces produced:** `studioTurn(projectId, userInput): AsyncGenerator<StudioTurnEvent>`, `StudioTurnEvent` union, `postDisposition(projectId, interventionId, action, reason)`. Consumed by Task 10/11.

- [ ] **Step 1: Write the failing test** — mock a streaming `fetch` body:

```ts
import { describe, it, expect, vi, afterEach } from "vitest";
import { studioTurn } from "./studioTurn";

afterEach(() => { vi.restoreAllMocks(); });

function sseBody(frames: string): Response {
  const stream = new ReadableStream<Uint8Array>({
    start(c) { c.enqueue(new TextEncoder().encode(frames)); c.close(); },
  });
  return new Response(stream, { status: 200, headers: { "Content-Type": "text/event-stream" } });
}

describe("studioTurn", () => {
  it("yields intervention then done", async () => {
    vi.spyOn(global, "fetch").mockResolvedValue(sseBody(
      `event: intervention\ndata: {"intervention_id":"iid","body":"连到治理决心","anchor":"论证图 · 治理决心主张","criterion":"D5","level":"I2"}\n\n` +
      `event: done\ndata: {}\n\n`,
    ));
    const events = [];
    for await (const e of studioTurn("p1", "它想证明中国在认真转型")) events.push(e);
    expect(events[0]).toMatchObject({ type: "intervention", interventionId: "iid", criterion: "D5" });
    expect(events.at(-1)).toEqual({ type: "done" });
  });
});
```

- [ ] **Step 2: Run — verify it fails**

Run: `cd apps/web && npm test -- api/studioTurn`
Expected: FAIL — module missing.

- [ ] **Step 3: Write `studioTurn.ts`** (mirror `api/turn.ts`, reuse `parseSSE`, `apiFetch` for disposition):

```ts
import { API_BASE } from "./client";
import { apiFetch } from "./client";
import { parseSSE } from "./sse";

export type StudioTurnEvent =
  | { type: "intervention"; interventionId: string; body: string; anchor: string; criterion: string; level: string }
  | { type: "gate"; contract: string; status: string; passed: number; total: number; missing: string[] }
  | { type: "done" }
  | { type: "error"; code: string; message: string };

export async function* studioTurn(projectId: string, userInput: string): AsyncGenerator<StudioTurnEvent> {
  const res = await fetch(`${API_BASE}/api/v1/projects/${projectId}/turn`, {
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
  for await (const frame of parseSSE(res.body)) {
    const data = frame.data ? JSON.parse(frame.data) : {};
    switch (frame.event) {
      case "intervention": yield { type: "intervention", interventionId: data.intervention_id, body: data.body, anchor: data.anchor, criterion: data.criterion, level: data.level }; break;
      case "gate": yield { type: "gate", contract: data.contract, status: data.status, passed: data.passed, total: data.total, missing: data.missing ?? [] }; break;
      case "done": yield { type: "done" }; break;
      case "error": yield { type: "error", code: data.error?.code ?? "internal_error", message: data.error?.message ?? "" }; break;
    }
  }
}

export async function postDisposition(projectId: string, interventionId: string, action: "accept" | "rewrite" | "reject", reason: string): Promise<void> {
  await apiFetch<void>(`/api/v1/projects/${projectId}/interventions/${interventionId}/disposition`, {
    method: "POST",
    body: JSON.stringify({ action, reason }),
  });
}
```

- [ ] **Step 4: Run — verify pass + typecheck**

Run: `cd apps/web && npm test -- api/studioTurn && npx tsc --noEmit`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add apps/web/src/api/studioTurn.ts apps/web/src/api/studioTurn.test.ts
git commit -m "feat(web-api): studioTurn SSE client + postDisposition"
```

---

### Task 10: Frontend — studio conversation controller

**Files:**
- Create: `apps/web/src/studio/conversation.ts`
- Test: `apps/web/src/studio/conversation.test.ts`

**Interfaces produced:** `createStudioConversation({ projectId, api })` → `{ getSnapshot, subscribe, send, dispose }` where the snapshot is `{ messages: CoachMessage[], sending: boolean, error: string | null, disposableInterventionId: string | null }`. Consumed by Task 11.

- [ ] **Step 1: Write the failing test** — `conversation.test.ts`:

```ts
import { describe, it, expect } from "vitest";
import { createStudioConversation } from "./conversation";

const fakeApi = {
  async *studioTurn() {
    yield { type: "intervention", interventionId: "iid", body: "连到治理决心", anchor: "论证图 · 治理决心主张", criterion: "D5", level: "I2" };
    yield { type: "done" };
  },
  postDisposition: async () => {},
} as any;

describe("createStudioConversation", () => {
  it("appends the student bubble then the coach reply, tracks the disposable id", async () => {
    const conv = createStudioConversation({ projectId: "p1", api: fakeApi });
    await conv.send("它想证明中国在认真转型");
    const s = conv.getSnapshot();
    expect(s.messages[0]).toMatchObject({ kind: "student", body: "它想证明中国在认真转型" });
    expect(s.messages[1]).toMatchObject({ kind: "ai", body: "连到治理决心", tag: "D5", anchor: "论证图 · 治理决心主张" });
    expect(s.sending).toBe(false);
    expect(s.disposableInterventionId).toBe("iid");
  });
});
```

- [ ] **Step 2: Run — verify it fails**

Run: `cd apps/web && npm test -- studio/conversation`
Expected: FAIL — module missing.

- [ ] **Step 3: Write `conversation.ts`** (lean, mirroring `agent/createConversation.ts`'s snapshot/subscribe/setState pattern):

```ts
import type { CoachMessage } from "./state";
import { studioTurn as defaultStudioTurn, postDisposition as defaultPostDisposition, type StudioTurnEvent } from "../api/studioTurn";

type Snapshot = { messages: CoachMessage[]; sending: boolean; error: string | null; disposableInterventionId: string | null };
type Deps = { projectId: string; api?: { studioTurn: typeof defaultStudioTurn; postDisposition: typeof defaultPostDisposition } };

export function createStudioConversation({ projectId, api }: Deps) {
  const turn = api?.studioTurn ?? defaultStudioTurn;
  const dispose = api?.postDisposition ?? defaultPostDisposition;
  let state: Snapshot = { messages: [], sending: false, error: null, disposableInterventionId: null };
  const listeners = new Set<() => void>();
  const emit = () => listeners.forEach((l) => l());
  const set = (p: Partial<Snapshot>) => { state = { ...state, ...p }; emit(); };

  async function send(text: string) {
    set({ messages: [...state.messages, { kind: "student", body: text }], sending: true, error: null });
    try {
      for await (const e of turn(projectId, text) as AsyncGenerator<StudioTurnEvent>) {
        if (e.type === "intervention") {
          set({
            messages: [...state.messages, { kind: "ai", body: e.body, tag: e.criterion || undefined, anchor: e.anchor || undefined }],
            disposableInterventionId: e.interventionId,
          });
        } else if (e.type === "error") {
          set({ error: e.message });
        }
      }
    } catch {
      set({ error: "对话失败，请重试" });
    } finally {
      set({ sending: false });
    }
  }

  async function disposeIntervention(action: "accept" | "rewrite" | "reject", reason: string) {
    if (!state.disposableInterventionId) return;
    await dispose(projectId, state.disposableInterventionId, action, reason);
  }

  return {
    getSnapshot: () => state,
    subscribe: (l: () => void) => { listeners.add(l); return () => listeners.delete(l); },
    send,
    dispose: disposeIntervention,
  };
}
```

- [ ] **Step 4: Run — verify pass + typecheck**

Run: `cd apps/web && npm test -- studio/conversation && npx tsc --noEmit`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add apps/web/src/studio/conversation.ts apps/web/src/studio/conversation.test.ts
git commit -m "feat(studio-web): live conversation controller (send + disposition)"
```

---

### Task 11: Frontend — wire `StudioContainer` live + composer `sending`

**Files:**
- Modify: `apps/web/src/studio/StudioContainer.tsx`
- Modify: `apps/web/src/studio/CoachRail.tsx` (+ pass a `sending` disable to the composer)
- Test: `apps/web/src/studio/StudioContainer.test.tsx` (add a live-send case)

**Interfaces:** consumes `createStudioConversation`, the projection's `coach.messages` (seed), the controller snapshot (live). Produces the live composer + disposition wiring.

- [ ] **Step 1: Write the failing test** — add to `StudioContainer.test.tsx`:

```tsx
it("sends a composer message and renders the live coach reply", async () => {
  const conv = {
    getSnapshot: () => ({ messages: [{ kind: "ai", body: "连到治理决心", tag: "D5", anchor: "论证图 · 治理决心主张" }], sending: false, error: null, disposableInterventionId: "iid" }),
    subscribe: () => () => {},
    send: vi.fn(async () => {}),
    dispose: vi.fn(async () => {}),
  };
  render(<StudioContainer api={fakeApi} ensureSession={async () => {}} makeConversation={() => conv as any} />);
  await waitFor(() => expect(screen.getByText("论证构建")).toBeInTheDocument());
  // the live coach reply from the controller is rendered
  await waitFor(() => expect(screen.getByText("连到治理决心")).toBeInTheDocument());
});
```

*(Implementer: inject the conversation via an optional `makeConversation` prop (default `createStudioConversation`) for testability, mirroring the existing `api`/`ensureSession` injection. Reuse the Task-11 test's existing `fakeApi`/projection fixture from the 5b StudioContainer test.)*

- [ ] **Step 2: Run — verify it fails**

Run: `cd apps/web && npm test -- studio/StudioContainer`
Expected: FAIL — no live coach reply / `makeConversation` unsupported.

- [ ] **Step 3: Wire `StudioContainer`** — after the project loads, create the conversation (`makeConversation({ projectId, api })`), subscribe via `useSyncExternalStore`, and:
  - `onComposerSend: (text) => conv.send(text)`,
  - `onDisposition: (choice, reason) => conv.dispose(choice, reason)`,
  - merge the controller's live `messages` AFTER the projection's `coach.messages` (projection = history on load; controller = this session's turns): `coach: { ...state.coach, messages: [...state.coach.messages, ...convSnapshot.messages] }`.
  - Pass `convSnapshot.sending` down so the composer disables. (Thread a `sending` prop through `StudioShell`→`CoachRail`, defaulting `false`; disable the textarea + send button while true.)

*(Implementer: keep `onSelectStation`/`onToggleFocus`/`onOpenMethodology` exactly as 5b. The conversation is created once the projectId is known — guard against re-creating on every render, e.g. store it in a ref/state keyed by projectId.)*

- [ ] **Step 4: Add the `sending` disable to `CoachRail.tsx`** — add an optional `sending?: boolean` prop; when true, disable the composer `<textarea>` + send `<button>` (`disabled={sending || !composerText.trim()}`). No copy change; inline SVG only.

- [ ] **Step 5: Run — verify pass + full web suite**

Run: `cd apps/web && npm test -- studio/StudioContainer && npm test && npx tsc --noEmit`
Expected: PASS (new case + whole suite green).

- [ ] **Step 6: Commit**

```bash
git add apps/web/src/studio/StudioContainer.tsx apps/web/src/studio/CoachRail.tsx apps/web/src/studio/StudioContainer.test.tsx
git commit -m "feat(studio-web): live composer + disposition wiring in StudioContainer"
```

---

## Self-Review Checklist (run before final review)

- **Spec coverage:** chat persistence (T1) · seam LoadChatHistory/CreateChatMessage (T2) · chat-aware coach (T3) · SkipSurfaceCards + history threading (T4) · Studio SSE events (T5) · turn endpoint + loop driver (T6) · disposition endpoint (T7) · projection merge (T8) · studioTurn client (T9) · conversation controller (T10) · StudioContainer live wiring + composer sending (T11). No surface_card (5c-2), no migration (cleanup), no onboarding producer, legacy `postTurn`/`RunTurn` untouched. ✓
- **Type/behavior consistency:** `SkipSurfaceCards` zero value preserves `RunAgentStep`; `ProposeIntervention`/`BuildCoachContext` new `history` param threaded from `RunAgentStep` (real) and all other callers (`nil`); the coach reply is an intervention row only (no assistant chat_message) — merged consistently in both `agent.LoadChatHistory` (context) and `studio.projectCoach` (display); `StudioTurnEvent` fields match the Go SSE emitter keys (`intervention_id`/`body`/`anchor`/`criterion`/`level`; `contract`/`status`/`passed`/`total`/`missing`). ✓
- **Restraint:** one `RunAgentStep` per turn; silence emits no intervention; enforcement stack unchanged. ✓
- **Live-verify:** `/?studio` → type in the composer → student bubble + one anchored coach reply (or silence) → reload merges both → dispose with ≥15 chars. ✓
