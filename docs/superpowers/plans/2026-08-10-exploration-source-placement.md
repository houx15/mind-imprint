# 探索图谱 · 来源归位 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 让学生加进的任何一篇来源在加进的当下被「归位」到主问题 / 某个子问题（成为该问题洞里的文献节点），或落进探索图谱上的一个「未归类」系统节点——使兔子洞地图成为全部来源探索的忠实记录。

**Architecture:** 复用 `adoptExploration` 的「建 connected lead」逻辑，新增 `attach` 端点（挂**已有** reference 到某问题）与 `suggest-placement` 端点（印记为一篇来源建议归属问题，中档模型、reasoning-off、记 `llm_call`）。前端：加来源成功后弹「归位」步（印记预高亮建议、学生点选）；`WarrenMap` 增一个中性「未归类」节点，点开是未归位来源清单，每条可进入阅读室或挂到问题下。共享 `PlacementPicker` 组件。

**Tech Stack:** Go (`net/http` + `pgx`/`sqlc`)；React + Vite + TS + React Flow (`@xyflow/react`)；Zod 契约（`packages/contracts`，web 直接 import `src`，无需构建步）。

## Global Constraints

- **客户端绝不直连模型。** 印记的归属建议只走 `POST …/exploration/suggest-placement`，记 `llm_call`（`meterCall`，purpose `"suggest_placement"`），在 `HasEntitlement` / `resolveFast` 之后。
- **铁律②（不操纵）：** 建议是**预高亮**，挂 / 落进未归类都由学生点一下。绝不自动挂。
- **papers-never-roots：** attach 出来的文献 lead 必带 `parentLeadId`（挂在某问题下），绝不为 root。
- **不改「悬空来源」语义：** `computeDanglingSourceIds`（`apps/api/internal/api/exploration.go:170`）与 `getExploration` 响应不动。「未归类」是**前端另算**的集合。
- **命名：** 地图仍叫「兔子洞地图」（不改）；新节点工作名「未归类」。**注意冲突**：`AddSourceModal` 的「放进合集」下拉已有一个 `<option>未归类`（`ReadingBlock.tsx:1373`，指「无合集」）——那是另一个维度，勿混淆；新节点是「未挂到任何问题」。最终命名待定，本期用「未归类」。
- **行为真相源：** `docs/2026-08-09-all-statuses.md` §5 reading + §130（每篇来源标注 which sub question）。本期只做 which-sub-question，不做「where it can appear（正文哪段）」。
- **测试纪律：** Go 测试用 testcontainers（需 Docker，前台跑，别后台化）；提交前跑**整包** `go test ./internal/api/ ./internal/agent/`。前端聚焦测试 `cd apps/web && npx vitest run <path>`，提交前 `pnpm --filter web typecheck`。契约改动后 `pnpm --filter @mind-imprint/contracts typecheck`。

---

### Task 1: `attach` 端点 —— 把已有 reference 挂到某问题

把一篇**已存在**的 reference 挂到一个**问题** lead 下，建一个 `connected` 文献 lead（不新建 reference）。复用 `adoptExploration` 的 position / IDOR / CreateExplorationLead 套路。

**Files:**
- Modify: `apps/api/internal/api/exploration.go`（在 `adoptExploration` 之后，约 825 行处新增 `attachExploration`）
- Modify: `apps/api/internal/api/api.go:116`（在 adopt 路由后加一行）
- Test: `apps/api/internal/api/exploration_test.go`（新增 `TestAttachExploration_*`）

**Interfaces:**
- Consumes: `a.loadOwnedProject`、`decodeJSON`、`a.d.Queries.GetReferenceForProject(ctx, sqlc.GetReferenceForProjectParams{ID, ProjectID})`、`GetExplorationLeadForProject`、`ListExplorationLeads`、`CreateExplorationLead`、`toExplorationLeadDTO`、`toReferenceDTO`、`httpx.ErrBadRequest`、`httpx.WriteJSON`。
- Produces: `POST /api/v1/projects/{id}/exploration/attach`，body `{ "referenceId": string, "parentLeadId": string }`，`201 { "lead": ExplorationLeadDTO, "reference": ReferenceDTO }`。用于 Task 5 的 `attachReference` 客户端。

- [ ] **Step 1: Write the failing test**

在 `exploration_test.go` 末尾新增（顶部 setup 照抄 `TestExplorationLead_Branch`：`h, cookie := newHarness(t)`、`pid := createProjectForTest(t, h, cookie)`、`base := "/api/v1/projects/"+pid`）：

```go
func TestAttachExploration_AttachesReferenceUnderQuestion(t *testing.T) {
	h, cookie := newHarness(t)
	pid := createProjectForTest(t, h, cookie)
	base := "/api/v1/projects/" + pid

	// A question (root) lead + a bare reference (no lead yet).
	rec := doJSON(t, h, cookie, "POST", base+"/exploration/leads", `{"text":"主问题"}`)
	mustStatus(t, rec, 201)
	qid := gjsonString(t, rec.Body.Bytes(), "lead.id")
	rec = doJSON(t, h, cookie, "POST", base+"/references", `{"title":"Green spaces and mortality"}`)
	mustStatus(t, rec, 201)
	refID := gjsonString(t, rec.Body.Bytes(), "reference.id")

	rec = doJSON(t, h, cookie, "POST", base+"/exploration/attach",
		`{"referenceId":"`+refID+`","parentLeadId":"`+qid+`"}`)
	mustStatus(t, rec, 201)
	if got := gjsonString(t, rec.Body.Bytes(), "lead.connectedReferenceId"); got != refID {
		t.Fatalf("connectedReferenceId = %q, want %q", got, refID)
	}
	if got := gjsonString(t, rec.Body.Bytes(), "lead.parentLeadId"); got != qid {
		t.Fatalf("parentLeadId = %q, want %q", got, qid)
	}
	if got := gjsonString(t, rec.Body.Bytes(), "lead.origin"); got != "manual" {
		t.Fatalf("origin = %q, want manual", got)
	}

	// Idempotent: attaching the same pair again returns the SAME lead, no dup.
	firstLead := gjsonString(t, rec.Body.Bytes(), "lead.id")
	rec = doJSON(t, h, cookie, "POST", base+"/exploration/attach",
		`{"referenceId":"`+refID+`","parentLeadId":"`+qid+`"}`)
	mustStatus(t, rec, 201)
	if got := gjsonString(t, rec.Body.Bytes(), "lead.id"); got != firstLead {
		t.Fatalf("second attach minted a new lead %q, want %q", got, firstLead)
	}
}

func TestAttachExploration_RejectsPaperParent(t *testing.T) {
	h, cookie := newHarness(t)
	pid := createProjectForTest(t, h, cookie)
	base := "/api/v1/projects/" + pid

	rec := doJSON(t, h, cookie, "POST", base+"/exploration/leads", `{"text":"主问题"}`)
	qid := gjsonString(t, rec.Body.Bytes(), "lead.id")
	rec = doJSON(t, h, cookie, "POST", base+"/references", `{"title":"A"}`)
	refA := gjsonString(t, rec.Body.Bytes(), "reference.id")
	rec = doJSON(t, h, cookie, "POST", base+"/references", `{"title":"B"}`)
	refB := gjsonString(t, rec.Body.Bytes(), "reference.id")

	// Attach A under the question → makes a PAPER lead.
	rec = doJSON(t, h, cookie, "POST", base+"/exploration/attach",
		`{"referenceId":"`+refA+`","parentLeadId":"`+qid+`"}`)
	mustStatus(t, rec, 201)
	paperLead := gjsonString(t, rec.Body.Bytes(), "lead.id")

	// Attaching B under a PAPER lead must 400 (papers can't parent sources).
	rec = doJSON(t, h, cookie, "POST", base+"/exploration/attach",
		`{"referenceId":"`+refB+`","parentLeadId":"`+paperLead+`"}`)
	mustStatus(t, rec, 400)
}
```

> `mustStatus` / `gjsonString` helpers: reuse whatever `exploration_test.go` already uses to assert status codes and pull JSON fields (grep the file — it already parses `rec.Body`). If a JSON-path helper doesn't exist, unmarshal into a `map[string]any` inline as the neighbouring tests do.

- [ ] **Step 2: Run test to verify it fails**

Run: `cd apps/api && go test ./internal/api/ -run TestAttachExploration -v`
Expected: FAIL — route 404 / handler `attachExploration` undefined.

- [ ] **Step 3: Write minimal implementation**

In `exploration.go`, after `adoptExploration` (ends ~825):

```go
// attachExploration hangs an EXISTING reference under a question lead — the
// self-added-source twin of adoptExploration (which mints a fresh reference
// from a dig candidate). It creates ONLY a connected lead (origin "manual" —
// the student's own placement), never a reference. The parent must be a
// QUESTION lead (connected_reference_id NULL): a source can't hang under
// another source (keeps the question→paper two layers clean; papers-never-roots
// holds because parentLeadId is always set). Idempotent: re-attaching the same
// (reference, parent) pair returns the existing non-pruned lead.
func (a *API) attachExploration(w http.ResponseWriter, r *http.Request) {
	projectID, ok := a.loadOwnedProject(w, r)
	if !ok {
		return
	}
	var body struct {
		ReferenceID  string `json:"referenceId"`
		ParentLeadID string `json:"parentLeadId"`
	}
	if err := decodeJSON(r, &body); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	refUUID, perr := uuid.Parse(strings.TrimSpace(body.ReferenceID))
	if perr != nil {
		httpx.WriteError(w, r, httpx.ErrBadRequest("validation_failed", "referenceId 不是有效的 id", nil))
		return
	}
	ref, err := a.d.Queries.GetReferenceForProject(r.Context(), sqlc.GetReferenceForProjectParams{ID: refUUID, ProjectID: projectID})
	if err != nil {
		httpx.WriteError(w, r, httpx.ErrBadRequest("validation_failed", "referenceId 不是这个项目里的来源", nil))
		return
	}
	pid, perr := uuid.Parse(strings.TrimSpace(body.ParentLeadID))
	if perr != nil {
		httpx.WriteError(w, r, httpx.ErrBadRequest("validation_failed", "parentLeadId 不是有效的 id", nil))
		return
	}
	parentLead, err := a.d.Queries.GetExplorationLeadForProject(r.Context(), sqlc.GetExplorationLeadForProjectParams{ID: pid, ProjectID: projectID})
	if err != nil {
		httpx.WriteError(w, r, httpx.ErrBadRequest("validation_failed", "parentLeadId 不是这个项目里的线索", nil))
		return
	}
	if parentLead.ConnectedReferenceID.Valid {
		httpx.WriteError(w, r, httpx.ErrBadRequest("validation_failed", "只能把来源挂到「问题」下，不能挂到另一篇文献下", nil))
		return
	}
	parent := pgtype.UUID{Bytes: pid, Valid: true}

	existing, err := a.d.Queries.ListExplorationLeads(r.Context(), projectID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	// Idempotency: a non-pruned lead already connecting this (ref, parent)?
	for _, l := range existing {
		if l.Status == "pruned" {
			continue
		}
		if l.ParentLeadID == parent && l.ConnectedReferenceID.Valid && uuid.UUID(l.ConnectedReferenceID.Bytes) == refUUID {
			httpx.WriteJSON(w, http.StatusCreated, map[string]any{
				"lead":      toExplorationLeadDTO(l),
				"reference": toReferenceDTO(ref, nil),
			})
			return
		}
	}
	var position int32
	for _, l := range existing {
		if l.ParentLeadID == parent {
			position++
		}
	}

	lead, err := a.d.Queries.CreateExplorationLead(r.Context(), sqlc.CreateExplorationLeadParams{
		ProjectID:            projectID,
		Text:                 ref.Title,
		Status:               "connected",
		Origin:               "manual",
		ConnectedReferenceID: pgtype.UUID{Bytes: ref.ID, Valid: true},
		Position:             position,
		ParentLeadID:         parent,
	})
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, map[string]any{
		"lead":      toExplorationLeadDTO(lead),
		"reference": toReferenceDTO(ref, nil),
	})
}
```

In `api.go`, after line 116 (`.../exploration/adopt`):

```go
	mux.Handle("POST /api/v1/projects/{id}/exploration/attach", protected(a.attachExploration))
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd apps/api && go test ./internal/api/ -run TestAttachExploration -v`
Expected: PASS (both tests).

- [ ] **Step 5: Commit**

```bash
git add apps/api/internal/api/exploration.go apps/api/internal/api/api.go apps/api/internal/api/exploration_test.go
git commit -m "feat(exploration): attach an existing reference under a question lead"
```

---

### Task 2: `PlacementSuggestion` 契约

印记归属建议的响应形状，前后端共享。

**Files:**
- Modify: `packages/contracts/src/exploration.ts`（在 `DigResult` 之后追加）
- Test: `packages/contracts/test/exploration.test.ts`（若不存在则新建；否则追加一个 `describe`）

**Interfaces:**
- Produces: `PlacementSuggestion = z.object({ leadId: z.string().nullable(), reason: z.string() })` + `type PlacementSuggestion`。Task 4 的服务端响应、Task 5 的 `suggestPlacement` 客户端都用它。`exploration.ts` 已由 `src/index.ts` re-export，web 直接可用。

- [ ] **Step 1: Write the failing test**

```ts
import { describe, it, expect } from "vitest";
import { PlacementSuggestion } from "../src/exploration";

describe("PlacementSuggestion", () => {
  it("accepts a chosen leadId", () => {
    expect(PlacementSuggestion.parse({ leadId: "abc", reason: "贴主问题" })).toEqual({ leadId: "abc", reason: "贴主问题" });
  });
  it("accepts null (未归类)", () => {
    expect(PlacementSuggestion.parse({ leadId: null, reason: "" })).toEqual({ leadId: null, reason: "" });
  });
  it("rejects a missing reason", () => {
    expect(() => PlacementSuggestion.parse({ leadId: null })).toThrow();
  });
});
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd packages/contracts && npx vitest run test/exploration.test.ts`
Expected: FAIL — `PlacementSuggestion` is not exported.

- [ ] **Step 3: Write minimal implementation**

Append to `packages/contracts/src/exploration.ts`:

```ts
// 印记 for one reference suggests the best-fit question to hang it under, or
// null (→ 未归类). Advisory only (铁律②): the student taps to confirm the placement.
export const PlacementSuggestion = z.object({
  leadId: z.string().nullable(),
  reason: z.string(),
});
export type PlacementSuggestion = z.infer<typeof PlacementSuggestion>;
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd packages/contracts && npx vitest run test/exploration.test.ts`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add packages/contracts/src/exploration.ts packages/contracts/test/exploration.test.ts
git commit -m "feat(contracts): PlacementSuggestion (印记 source-placement suggestion)"
```

---

### Task 3: `SuggestBestQuestion` agent 生成器

给定一篇来源（标题/摘要）+ 本项目的问题清单，让模型选一个最贴合的问题 id 或 null。Best-effort：任何错误 → caller 降级为 null。镜像 `agent/search_guidance.go`。

**Files:**
- Create: `apps/api/internal/agent/placement.go`
- Test: `apps/api/internal/agent/placement_test.go`

**Interfaces:**
- Consumes: `gateway.Provider`、`gateway.Resolved`、`gateway.Collect`、`gateway.ChatRequest/ChatMessage/RoleSystem/RoleUser/ChatUsage`、`extractJSONObject`、`stripFences`（同 package，已用于 `parseSearchGuidance`）。
- Produces:
  ```go
  type PlacementQuestion struct { ID, Text string }
  type SuggestPlacementInput struct { Title, Abstract, Journal, Year string; Questions []PlacementQuestion }
  type PlacementSuggestionOut struct { LeadID string `json:"leadId"`; Reason string `json:"reason"` } // LeadID "" = 未归类
  func SuggestBestQuestion(ctx, prov gateway.Provider, resolved gateway.Resolved, in SuggestPlacementInput) (PlacementSuggestionOut, gateway.ChatUsage, error)
  ```
  Task 4 的 handler 调它。

- [ ] **Step 1: Write the failing test**

```go
package agent

import (
	"context"
	"testing"

	"mindimprint/api/internal/gateway"
)

func placementProvider(reply string) *gateway.StubProvider {
	return gateway.NewStubProvider([]gateway.StreamEvent{
		{Kind: gateway.EventTextDelta, TextDelta: reply},
		{Kind: gateway.EventUsage, Usage: &gateway.ChatUsage{InputTokens: 30, OutputTokens: 12}},
		{Kind: gateway.EventDone, StopReason: gateway.StopStop},
	})
}

func TestSuggestBestQuestion_PicksAValidLead(t *testing.T) {
	in := SuggestPlacementInput{
		Title:     "Urban tree canopy and heat",
		Questions: []PlacementQuestion{{ID: "q1", Text: "树冠能降温吗"}, {ID: "q2", Text: "政策成本"}},
	}
	out, _, err := SuggestBestQuestion(context.Background(), placementProvider(`{"leadId":"q1","reason":"直接回答降温机制"}`),
		gateway.Resolved{Provider: "deepseek", Model: "x"}, in)
	if err != nil {
		t.Fatal(err)
	}
	if out.LeadID != "q1" || out.Reason == "" {
		t.Fatalf("out = %+v, want leadId q1 + reason", out)
	}
}

func TestSuggestBestQuestion_DropsUnknownLeadToEmpty(t *testing.T) {
	in := SuggestPlacementInput{Title: "X", Questions: []PlacementQuestion{{ID: "q1", Text: "a"}}}
	// Model hallucinates an id not in the set → must fall back to "" (未归类).
	out, _, err := SuggestBestQuestion(context.Background(), placementProvider(`{"leadId":"nope","reason":"r"}`),
		gateway.Resolved{Provider: "deepseek", Model: "x"}, in)
	if err != nil {
		t.Fatal(err)
	}
	if out.LeadID != "" {
		t.Fatalf("leadId = %q, want empty (unknown id dropped)", out.LeadID)
	}
}

func TestSuggestBestQuestion_NullIsNone(t *testing.T) {
	in := SuggestPlacementInput{Title: "X", Questions: []PlacementQuestion{{ID: "q1", Text: "a"}}}
	out, _, err := SuggestBestQuestion(context.Background(), placementProvider(`{"leadId":null,"reason":"都不太贴"}`),
		gateway.Resolved{Provider: "deepseek", Model: "x"}, in)
	if err != nil {
		t.Fatal(err)
	}
	if out.LeadID != "" {
		t.Fatalf("leadId = %q, want empty", out.LeadID)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd apps/api && go test ./internal/agent/ -run TestSuggestBestQuestion -v`
Expected: FAIL — `SuggestBestQuestion` undefined.

- [ ] **Step 3: Write minimal implementation**

`apps/api/internal/agent/placement.go`:

```go
package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"mindimprint/api/internal/gateway"
)

// placement.go — 印记 for one reading source suggests the best-fit research
// question to hang it under (or none → 未归类). Fast model, reasoning-off.
// Advisory (铁律②): the student confirms the placement with a tap. Best-effort:
// any error → caller degrades to "no suggestion" (未归类).

type PlacementQuestion struct {
	ID   string
	Text string
}

type SuggestPlacementInput struct {
	Title    string
	Abstract string
	Journal  string
	Year     string
	Questions []PlacementQuestion
}

// PlacementSuggestionOut.LeadID == "" means "no good fit → 未归类".
type PlacementSuggestionOut struct {
	LeadID string `json:"leadId"`
	Reason string `json:"reason"`
}

const placementSystem = `你是一位 IB 研究导师。学生刚加进一篇文献。下面给你这篇文献的信息，和这个项目的若干【研究问题】（每个带一个 id）。请判断这篇文献最能支撑/回答哪一个问题。

只返回一个 JSON 对象：
{"leadId": "最贴合的问题 id，或 null", "reason": "一句话中文：为什么挂这个问题（或为什么都不太贴）"}

要求：
- leadId 必须是给定问题里的某个 id；如果都不太贴，返回 null。
- reason 一句话，具体，给学生看的。
- 只回 JSON，不要任何解释或代码块外的文字。`

const maxPlacementAttempts = 2

func SuggestBestQuestion(ctx context.Context, prov gateway.Provider, resolved gateway.Resolved, in SuggestPlacementInput) (PlacementSuggestionOut, gateway.ChatUsage, error) {
	var b strings.Builder
	fmt.Fprintf(&b, "文献标题：%s\n", strings.TrimSpace(in.Title))
	if s := strings.TrimSpace(in.Journal); s != "" {
		fmt.Fprintf(&b, "期刊/来源：%s\n", s)
	}
	if s := strings.TrimSpace(in.Year); s != "" {
		fmt.Fprintf(&b, "年份：%s\n", s)
	}
	if s := strings.TrimSpace(in.Abstract); s != "" {
		fmt.Fprintf(&b, "摘要：%s\n", s)
	}
	b.WriteString("\n研究问题：\n")
	for _, q := range in.Questions {
		fmt.Fprintf(&b, "  - id=%s  %s\n", q.ID, strings.TrimSpace(q.Text))
	}

	req := gateway.ChatRequest{
		Messages: []gateway.ChatMessage{
			{Role: gateway.RoleSystem, Content: placementSystem},
			{Role: gateway.RoleUser, Content: b.String()},
		},
		MaxTokens: 600,
	}

	valid := map[string]bool{}
	for _, q := range in.Questions {
		valid[q.ID] = true
	}

	var lastUsage gateway.ChatUsage
	var lastErr error
	for attempt := 0; attempt < maxPlacementAttempts; attempt++ {
		res, err := gateway.Collect(ctx, prov, resolved, req)
		lastUsage = res.Usage
		if err != nil {
			lastErr = err
			continue
		}
		out, perr := parsePlacement(res.Text, valid)
		if perr != nil {
			lastErr = perr
			continue
		}
		return out, lastUsage, nil
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("placement: no parseable suggestion")
	}
	return PlacementSuggestionOut{}, lastUsage, lastErr
}

func parsePlacement(text string, valid map[string]bool) (PlacementSuggestionOut, error) {
	raw := extractJSONObject(stripFences(text))
	if raw == "" {
		return PlacementSuggestionOut{}, fmt.Errorf("placement: no JSON object in reply")
	}
	var parsed struct {
		LeadID *string `json:"leadId"`
		Reason string  `json:"reason"`
	}
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		return PlacementSuggestionOut{}, fmt.Errorf("placement: unmarshal: %w", err)
	}
	out := PlacementSuggestionOut{Reason: strings.TrimSpace(parsed.Reason)}
	if parsed.LeadID != nil {
		id := strings.TrimSpace(*parsed.LeadID)
		if valid[id] { // drop hallucinated / null ids → "" (未归类)
			out.LeadID = id
		}
	}
	return out, nil
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd apps/api && go test ./internal/agent/ -run TestSuggestBestQuestion -v`
Expected: PASS (all three).

- [ ] **Step 5: Commit**

```bash
git add apps/api/internal/agent/placement.go apps/api/internal/agent/placement_test.go
git commit -m "feat(agent): SuggestBestQuestion — 印记 suggests a source's home question"
```

---

### Task 4: `suggest-placement` 端点

镜像 `postSearchGuidance`：加载 reference + 问题清单，无问题则直接返回 null（不调模型）；否则 fast 模型 + `meterCall`。

**Files:**
- Create: `apps/api/internal/api/placement.go`
- Modify: `apps/api/internal/api/api.go`（在 adopt/attach 路由附近加一行）
- Test: `apps/api/internal/api/placement_test.go`

**Interfaces:**
- Consumes: `a.loadOwnedProject`、`decodeJSON`、`GetReferenceForProject`、`ListExplorationLeads`、`a.d.Provider`、`a.resolveFast`、`a.meterCall`、`agent.SuggestBestQuestion`、`agent.SuggestPlacementInput`、`agent.PlacementQuestion`。
- Produces: `POST /api/v1/projects/{id}/exploration/suggest-placement`，body `{ "referenceId": string }`，`200 { "leadId": string|null, "reason": string }`（`leadId` 空串 → 序列化为 `null`）。Task 5 的 `suggestPlacement` 客户端消费。

- [ ] **Step 1: Write the failing test**

```go
func TestSuggestPlacement_NoQuestions_ReturnsNullNoSpend(t *testing.T) {
	h, cookie := newHarness(t)
	pid := createProjectForTest(t, h, cookie)
	base := "/api/v1/projects/" + pid

	rec := doJSON(t, h, cookie, "POST", base+"/references", `{"title":"孤零零的来源"}`)
	refID := gjsonString(t, rec.Body.Bytes(), "reference.id")

	rec = doJSON(t, h, cookie, "POST", base+"/exploration/suggest-placement", `{"referenceId":"`+refID+`"}`)
	mustStatus(t, rec, 200)
	// No questions yet → leadId null, empty reason, and (implicitly) no model call.
	var body struct {
		LeadID *string `json:"leadId"`
		Reason string  `json:"reason"`
	}
	mustUnmarshal(t, rec.Body.Bytes(), &body)
	if body.LeadID != nil {
		t.Fatalf("leadId = %v, want null when there are no questions", *body.LeadID)
	}
}

func TestSuggestPlacement_ForeignReference_400(t *testing.T) {
	h, cookie := newHarness(t)
	pid := createProjectForTest(t, h, cookie)
	other := createProjectForTest(t, h, cookie)
	rec := doJSON(t, h, cookie, "POST", "/api/v1/projects/"+other+"/references", `{"title":"别处"}`)
	refID := gjsonString(t, rec.Body.Bytes(), "reference.id")

	rec = doJSON(t, h, cookie, "POST", "/api/v1/projects/"+pid+"/exploration/suggest-placement", `{"referenceId":"`+refID+`"}`)
	mustStatus(t, rec, 400)
}
```

> `mustUnmarshal`: if the file has no such helper, `json.Unmarshal(rec.Body.Bytes(), &body)` inline and `t.Fatal` on error.
> The happy-path (with a stub provider) is covered by Task 3's agent test; here we only need the no-questions and IDOR branches, which don't require a provider. The harness's `a.d.Provider` is nil in tests unless wired — the no-questions branch returns before touching it.

- [ ] **Step 2: Run test to verify it fails**

Run: `cd apps/api && go test ./internal/api/ -run TestSuggestPlacement -v`
Expected: FAIL — route 404.

- [ ] **Step 3: Write minimal implementation**

`apps/api/internal/api/placement.go`:

```go
package api

import (
	"log/slog"
	"net/http"
	"strings"

	"github.com/google/uuid"

	"mindimprint/api/internal/agent"
	"mindimprint/api/internal/httpx"
	"mindimprint/api/internal/store/sqlc"
)

// placement.go — POST suggest-placement · 印记 suggests which research question
// a just-added reference belongs under (or null → 未归类). Fast model, spends
// tokens (metered). Best-effort: no provider / model error / no questions →
// { leadId: null, reason: "" }, 200. Advisory (铁律②): the student confirms via
// the attach endpoint.
func (a *API) postSuggestPlacement(w http.ResponseWriter, r *http.Request) {
	projectID, ok := a.loadOwnedProject(w, r)
	if !ok {
		return
	}
	var body struct {
		ReferenceID string `json:"referenceId"`
	}
	if err := decodeJSON(r, &body); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	refUUID, perr := uuid.Parse(strings.TrimSpace(body.ReferenceID))
	if perr != nil {
		httpx.WriteError(w, r, httpx.ErrBadRequest("validation_failed", "referenceId 不是有效的 id", nil))
		return
	}
	ref, err := a.d.Queries.GetReferenceForProject(r.Context(), sqlc.GetReferenceForProjectParams{ID: refUUID, ProjectID: projectID})
	if err != nil {
		httpx.WriteError(w, r, httpx.ErrBadRequest("validation_failed", "referenceId 不是这个项目里的来源", nil))
		return
	}

	// Question leads = non-pruned leads with no connected reference (root main
	// questions + nested sub-questions). Papers (connected_reference_id set) and
	// pruned leads are not placement targets.
	leads, err := a.d.Queries.ListExplorationLeads(r.Context(), projectID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	var questions []agent.PlacementQuestion
	for _, l := range leads {
		if l.Status == "pruned" || l.ConnectedReferenceID.Valid {
			continue
		}
		questions = append(questions, agent.PlacementQuestion{ID: l.ID.String(), Text: l.Text})
	}

	// Best-effort default (also the no-questions / no-provider answer).
	leadID := ""
	reason := ""
	if len(questions) > 0 && a.d.Provider != nil {
		if resolved, rok := a.resolveFast(r.Context()); rok {
			out, usage, gerr := agent.SuggestBestQuestion(r.Context(), a.d.Provider, resolved, agent.SuggestPlacementInput{
				Title: ref.Title, Abstract: ref.Abstract, Journal: ref.Journal, Year: ref.Year, Questions: questions,
			})
			a.meterCall(r.Context(), projectID, resolved, "suggest_placement", usage)
			if gerr != nil {
				slog.Warn("suggest placement: generation failed", "err", gerr, "request_id", httpx.RequestIDFromContext(r.Context()))
			} else {
				leadID = out.LeadID
				reason = out.Reason
			}
		}
	}

	// leadId serialises as null when empty (未归类).
	var leadOut any
	if leadID != "" {
		leadOut = leadID
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"leadId": leadOut, "reason": reason})
}
```

In `api.go`, after the attach route from Task 1:

```go
	mux.Handle("POST /api/v1/projects/{id}/exploration/suggest-placement", protected(a.postSuggestPlacement))
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd apps/api && go test ./internal/api/ -run TestSuggestPlacement -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add apps/api/internal/api/placement.go apps/api/internal/api/api.go apps/api/internal/api/placement_test.go
git commit -m "feat(exploration): suggest-placement endpoint (印记 source-home suggestion, metered)"
```

---

### Task 5: 前端客户端 —— `attachReference` + `suggestPlacement`

**Files:**
- Modify: `apps/web/src/api/exploration.ts`（追加两个导出）
- Test: `apps/web/test/api/exploration.test.ts`（若无则新建）

**Interfaces:**
- Consumes: `apiFetch`、`ExplorationLead`、`Reference`（type）、`PlacementSuggestion`（Task 2）。
- Produces:
  ```ts
  attachReference(projectId: string, referenceId: string, parentLeadId: string): Promise<{ lead: ExplorationLead; reference: Reference }>
  suggestPlacement(projectId: string, referenceId: string): Promise<PlacementSuggestion>
  ```
  Task 6/8/9 消费。

- [ ] **Step 1: Write the failing test**

```ts
import { describe, it, expect, vi, beforeEach } from "vitest";
import * as client from "../../src/api/client";
import { attachReference, suggestPlacement } from "../../src/api/exploration";

describe("source-placement client", () => {
  beforeEach(() => vi.restoreAllMocks());

  it("attachReference POSTs referenceId+parentLeadId and parses the lead", async () => {
    const spy = vi.spyOn(client, "apiFetch").mockResolvedValue({
      lead: { id: "l1", text: "T", status: "connected", origin: "manual", sourceReferenceId: null, connectedReferenceId: "r1", position: 0, parentLeadId: "q1", createdAt: "2026-08-10T00:00:00Z" },
      reference: { id: "r1", title: "T" },
    } as unknown);
    const out = await attachReference("p1", "r1", "q1");
    expect(spy).toHaveBeenCalledWith("/api/v1/projects/p1/exploration/attach", expect.objectContaining({ method: "POST" }));
    const body = JSON.parse((spy.mock.calls[0]![1] as { body: string }).body);
    expect(body).toEqual({ referenceId: "r1", parentLeadId: "q1" });
    expect(out.lead.connectedReferenceId).toBe("r1");
  });

  it("suggestPlacement parses a null leadId", async () => {
    vi.spyOn(client, "apiFetch").mockResolvedValue({ leadId: null, reason: "" } as unknown);
    expect(await suggestPlacement("p1", "r1")).toEqual({ leadId: null, reason: "" });
  });
});
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd apps/web && npx vitest run test/api/exploration.test.ts`
Expected: FAIL — exports undefined.

- [ ] **Step 3: Write minimal implementation**

Append to `apps/web/src/api/exploration.ts` (add `PlacementSuggestion` to the existing contracts import on line 3):

```ts
// Attach an EXISTING reference under a question lead → a connected paper node
// (the self-added-source twin of adoptCandidate). Server mirrors adopt but
// creates no reference. Response mirrors adopt: { lead, reference }.
export async function attachReference(
  projectId: string,
  referenceId: string,
  parentLeadId: string,
): Promise<{ lead: ExplorationLead; reference: Reference }> {
  const raw = await apiFetch<unknown>(`/api/v1/projects/${projectId}/exploration/attach`, {
    method: "POST",
    body: JSON.stringify({ referenceId, parentLeadId }),
  });
  const r = raw as { lead: unknown; reference: Reference };
  return { lead: ExplorationLead.parse(r.lead), reference: r.reference };
}

// 印记 suggests the best-fit question to hang a reference under (or null → 未归类).
// Advisory (铁律②): the student taps to confirm via attachReference.
export async function suggestPlacement(projectId: string, referenceId: string): Promise<PlacementSuggestion> {
  const raw = await apiFetch<unknown>(`/api/v1/projects/${projectId}/exploration/suggest-placement`, {
    method: "POST",
    body: JSON.stringify({ referenceId }),
  });
  return PlacementSuggestion.parse(raw);
}
```

Update the import on line 3 to include `PlacementSuggestion`:

```ts
import { ExplorationView, ExplorationLead, ExplorationGuide, DigResult, DigCandidate, QuestionEdge, PlacementSuggestion } from "@mind-imprint/contracts";
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd apps/web && npx vitest run test/api/exploration.test.ts`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add apps/web/src/api/exploration.ts apps/web/test/api/exploration.test.ts
git commit -m "feat(web): attachReference + suggestPlacement clients"
```

---

### Task 6: `PlacementPicker` 组件

纯展示组件：主问题 + 缩进子问题的可点列表 + 印记建议预高亮 + 「先放进未归类」。add-time 弹窗与未归类面板共用。

**Files:**
- Create: `apps/web/src/workspace/blocks/exploration/PlacementPicker.tsx`
- Test: `apps/web/test/workspace/blocks/exploration/PlacementPicker.test.tsx`

**Interfaces:**
- Produces:
  ```ts
  export type PlacementQuestion = { id: string; text: string; parentId: string | null };
  export function PlacementPicker(props: {
    questions: PlacementQuestion[];
    suggestedLeadId: string | null;
    reason: string;
    busy?: boolean;
    onPick: (leadId: string | null) => void; // null = 未归类
  }): JSX.Element
  ```
  Task 8/9 消费。

- [ ] **Step 1: Write the failing test**

```tsx
import { describe, it, expect, vi } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import { PlacementPicker } from "../../../../src/workspace/blocks/exploration/PlacementPicker";

const qs = [
  { id: "q1", text: "主问题一", parentId: null },
  { id: "q1a", text: "子问题 A", parentId: "q1" },
  { id: "q2", text: "主问题二", parentId: null },
];

describe("PlacementPicker", () => {
  it("marks the suggested question and shows its reason", () => {
    render(<PlacementPicker questions={qs} suggestedLeadId="q1a" reason="贴子问题A" onPick={() => {}} />);
    expect(screen.getByText("贴子问题A")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /子问题 A/ })).toHaveAttribute("data-suggested", "true");
  });

  it("picks a question id", () => {
    const onPick = vi.fn();
    render(<PlacementPicker questions={qs} suggestedLeadId={null} reason="" onPick={onPick} />);
    fireEvent.click(screen.getByRole("button", { name: /主问题二/ }));
    expect(onPick).toHaveBeenCalledWith("q2");
  });

  it("picks 未归类 (null)", () => {
    const onPick = vi.fn();
    render(<PlacementPicker questions={qs} suggestedLeadId={null} reason="" onPick={onPick} />);
    fireEvent.click(screen.getByRole("button", { name: /先放进未归类/ }));
    expect(onPick).toHaveBeenCalledWith(null);
  });
});
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd apps/web && npx vitest run test/workspace/blocks/exploration/PlacementPicker.test.tsx`
Expected: FAIL — module not found.

- [ ] **Step 3: Write minimal implementation**

`apps/web/src/workspace/blocks/exploration/PlacementPicker.tsx`:

```tsx
// PlacementPicker — 来源归位选择器 (铁律②: 印记 suggests, the student taps). Main
// questions are top-level chips; sub-questions indent under their parent. The
// 印记-suggested question is pre-highlighted with its one-line reason. A final
// 「先放进未归类」 lands the source in the 未归类 node instead. Pure/presentational;
// shared by the add-time modal (ReadingBlock) and the 未归类 panel (ExplorationView).

export type PlacementQuestion = { id: string; text: string; parentId: string | null };

export function PlacementPicker({
  questions,
  suggestedLeadId,
  reason,
  busy,
  onPick,
}: {
  questions: PlacementQuestion[];
  suggestedLeadId: string | null;
  reason: string;
  busy?: boolean;
  onPick: (leadId: string | null) => void;
}) {
  // Group: roots in order, each followed by its sub-questions.
  const roots = questions.filter((q) => q.parentId == null);
  const subsOf = (id: string) => questions.filter((q) => q.parentId === id);

  const row = (q: PlacementQuestion, indented: boolean) => {
    const suggested = q.id === suggestedLeadId;
    return (
      <button
        key={q.id}
        type="button"
        data-suggested={suggested ? "true" : "false"}
        disabled={busy}
        onClick={() => onPick(q.id)}
        className={
          "flex w-full items-center gap-2 rounded-mk border px-3 py-2 text-left text-[13.5px] transition disabled:opacity-50 " +
          (suggested ? "border-mk-accent bg-mk-accent-50 text-mk-ink" : "border-mk-border bg-mk-surface text-mk-ink hover:border-mk-accent") +
          (indented ? " ml-4" : "")
        }
      >
        <span className="min-w-0 flex-1 truncate">{q.text}</span>
        {suggested && <span className="flex-none rounded-full bg-mk-accent px-2 py-0.5 text-[11px] font-bold text-white">印记建议</span>}
      </button>
    );
  };

  return (
    <div className="flex flex-col gap-2">
      {suggestedLeadId && reason && (
        <p className="rounded-mk bg-mk-accent-50 px-3 py-2 text-[12.5px] leading-relaxed text-mk-muted">印记：{reason}</p>
      )}
      {roots.length === 0 && (
        <p className="text-[12.5px] text-mk-faint">还没有研究问题——先把它放进未归类，之后再挂。</p>
      )}
      <div className="flex flex-col gap-1.5">
        {roots.map((r) => (
          <div key={r.id} className="flex flex-col gap-1.5">
            {row(r, false)}
            {subsOf(r.id).map((s) => row(s, true))}
          </div>
        ))}
      </div>
      <button
        type="button"
        disabled={busy}
        onClick={() => onPick(null)}
        className="mt-1 rounded-mk border border-dashed border-mk-border px-3 py-2 text-[13px] font-semibold text-mk-muted hover:border-mk-accent hover:text-mk-accent disabled:opacity-50"
      >
        先放进未归类
      </button>
    </div>
  );
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd apps/web && npx vitest run test/workspace/blocks/exploration/PlacementPicker.test.tsx`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add apps/web/src/workspace/blocks/exploration/PlacementPicker.tsx apps/web/test/workspace/blocks/exploration/PlacementPicker.test.tsx
git commit -m "feat(web): PlacementPicker — source-placement chooser"
```

---

### Task 7: `WarrenMap` 的「未归类」节点

在 map 上加一个中性系统节点（`unfiledCount>0` 时才出现），点它触发 `onOpenUnfiled`——不参与拖拽持久化、删除、连边、也不走 `onZoom`。

**Files:**
- Modify: `apps/web/src/workspace/blocks/exploration/WarrenMap.tsx`
- Test: `apps/web/test/workspace/blocks/exploration/WarrenMap.test.tsx`（若无则新建；React Flow 在 jsdom 里需 mock，见下）

**Interfaces:**
- Consumes: 现有 `WarrenMapProps`。
- Produces: `WarrenMapProps` 新增 `unfiledCount: number; onOpenUnfiled: () => void;`；导出常量 `export const UNFILED_NODE_ID = "__unfiled__";`。Task 8 传入这两个 prop。

- [ ] **Step 1: Write the failing test**

WarrenMap 用 React Flow，jsdom 里渲染不了完整画布，但节点模型的构建是可断言的。最稳的测试是断言**点击路由不误伤**：`onNodeClick` 对 `__unfiled__` 调 `onOpenUnfiled`、对普通 id 调 `onZoom`。把这条逻辑抽成一个可单测的纯函数，避免依赖 React Flow 渲染：

在 `WarrenMap.tsx` 顶部导出：

```ts
export function routeNodeClick(nodeId: string, onZoom: (id: string) => void, onOpenUnfiled: () => void): void {
  if (nodeId === UNFILED_NODE_ID) onOpenUnfiled();
  else onZoom(nodeId);
}
```

测试：

```tsx
import { describe, it, expect, vi } from "vitest";
import { routeNodeClick, UNFILED_NODE_ID } from "../../../../src/workspace/blocks/exploration/WarrenMap";

describe("WarrenMap node-click routing", () => {
  it("routes the 未归类 sentinel to onOpenUnfiled, not onZoom", () => {
    const onZoom = vi.fn();
    const onOpen = vi.fn();
    routeNodeClick(UNFILED_NODE_ID, onZoom, onOpen);
    expect(onOpen).toHaveBeenCalledTimes(1);
    expect(onZoom).not.toHaveBeenCalled();
  });
  it("routes a real root to onZoom", () => {
    const onZoom = vi.fn();
    const onOpen = vi.fn();
    routeNodeClick("root-1", onZoom, onOpen);
    expect(onZoom).toHaveBeenCalledWith("root-1");
    expect(onOpen).not.toHaveBeenCalled();
  });
});
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd apps/web && npx vitest run test/workspace/blocks/exploration/WarrenMap.test.tsx`
Expected: FAIL — `routeNodeClick` / `UNFILED_NODE_ID` not exported.

- [ ] **Step 3: Write minimal implementation**

In `WarrenMap.tsx`:

1) Near the top (after imports), export the sentinel + router:

```ts
export const UNFILED_NODE_ID = "__unfiled__";

export function routeNodeClick(nodeId: string, onZoom: (id: string) => void, onOpenUnfiled: () => void): void {
  if (nodeId === UNFILED_NODE_ID) onOpenUnfiled();
  else onZoom(nodeId);
}
```

2) A neutral node view (put beside `WarrenNodeView`):

```tsx
function UnfiledNodeView({ data }: NodeProps) {
  const d = data as unknown as { count: number };
  return (
    <div
      className="flex flex-col justify-center overflow-hidden border border-dashed border-mk-border bg-mk-paper py-3 pl-5 pr-4"
      style={{ width: 208, height: 104, borderRadius: "var(--mk-radius-md)", boxShadow: "var(--mk-shadow-xs)", cursor: "pointer" }}
    >
      <span className="text-[14px] font-bold text-mk-muted">未归类</span>
      <span className="mt-1 text-[12px] font-bold text-mk-faint">还没挂到问题下 · {d.count} 篇</span>
      <span className="mt-1 text-[11px] text-mk-faint">点开，把它们挂到问题下</span>
    </div>
  );
}
```

3) Register it (extend the existing const on line 254):

```ts
const nodeTypes = { warren: WarrenNodeView, unfiled: UnfiledNodeView };
```

4) Extend `WarrenMapProps` (after `onDeleteLead`, line 271):

```ts
  // The 未归类 system node: how many references hang under no question (read or
  // unread), and what to do when the student opens it. count 0 → node hidden.
  unfiledCount: number;
  onOpenUnfiled: () => void;
```

5) Destructure them in `WarrenMapInner({ ... })` (line 291-303 param list): add `unfiledCount,` and `onOpenUnfiled,`.

6) In the reconciliation effect (lines 333-347), append the sentinel node when `unfiledCount > 0`, preserving a dragged position if any:

```tsx
  useEffect(() => {
    setRfNodes((prev) => {
      const prevById = new Map(prev.map((n) => [n.id, n]));
      const rootNodes = nodeModels.map((m) => {
        const existing = prevById.get(m.id);
        return {
          id: m.id,
          type: "warren",
          position: existing?.position ?? m.position,
          data: { text: m.text, paperCount: m.paperCount, theme: m.theme, onRequestDelete: requestDelete },
          draggable: true,
        } as RFNode;
      });
      if (unfiledCount > 0) {
        const existing = prevById.get(UNFILED_NODE_ID);
        rootNodes.push({
          id: UNFILED_NODE_ID,
          type: "unfiled",
          position: existing?.position ?? { x: 0, y: 320 },
          data: { count: unfiledCount },
          draggable: false,
        } as unknown as RFNode);
      }
      return rootNodes;
    });
  }, [nodeModels, requestDelete, unfiledCount]);
```

7) Route the click (line 440):

```tsx
          onNodeClick={(_evt, node) => routeNodeClick(node.id, onZoom, onOpenUnfiled)}
```

8) Guard `onNodeDragStop` (it's `draggable:false` so it won't fire, but be defensive — line 354-365): first line of the callback body:

```tsx
      if (node.id === UNFILED_NODE_ID) return;
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd apps/web && npx vitest run test/workspace/blocks/exploration/WarrenMap.test.tsx`
Expected: PASS. Also `cd apps/web && npx vitest run test/workspace/blocks/exploration/ExplorationView.test.tsx` to confirm no existing WarrenMap-prop test broke (it renders WarrenMap with the new required props via Task 8; if ExplorationView tests fail here it's because Task 8 isn't done yet — that's expected, they go green in Task 8).

- [ ] **Step 5: Commit**

```bash
git add apps/web/src/workspace/blocks/exploration/WarrenMap.tsx apps/web/test/workspace/blocks/exploration/WarrenMap.test.tsx
git commit -m "feat(web): 未归类 system node on the warren map (routing + view)"
```

---

### Task 8: `ExplorationView` —— 未归类集合 + 面板 + 挂载接线

算出未归类来源，喂给 WarrenMap；新增 `"unfiled"` zoom 面板，列出未归类来源，每条可进入阅读室或用 `PlacementPicker` 挂到问题下。

**Files:**
- Modify: `apps/web/src/workspace/blocks/exploration/ExplorationView.tsx`
- Test: `apps/web/test/workspace/blocks/exploration/ExplorationView.test.tsx`（追加）

**Interfaces:**
- Consumes: `attachReference`（Task 5）、`PlacementPicker`/`PlacementQuestion`（Task 6）、`WarrenMap` 新 props + `UNFILED_NODE_ID`（Task 7）、现有 `references` prop、`onEnterReading`、`onLibraryChanged`、`refresh`。
- Produces: 无对外新接口（内部行为）。

- [ ] **Step 1: Write the failing test**

追加到 `ExplorationView.test.tsx`（沿用文件顶部既有的 React Flow mock + `renderExploration` 辅助；若辅助不同就照抄现有一个 test 的 render 方式，多传 `references`）。断言未归类计算的纯逻辑最稳——把它抽成导出的纯函数 `unfiledReferences`：

```tsx
import { unfiledReferences } from "../../../../src/workspace/blocks/exploration/ExplorationView";

describe("unfiledReferences", () => {
  const refs = [
    { id: "r1", title: "A", archived: false },
    { id: "r2", title: "B", archived: false },
    { id: "r3", title: "C", archived: true }, // archived → excluded
  ] as any[];
  const leads = [
    { id: "l1", connectedReferenceId: "r1", status: "connected", parentLeadId: "q1" },
    { id: "l2", connectedReferenceId: "r2", status: "pruned", parentLeadId: "q1" }, // pruned → r2 still unfiled
  ] as any[];

  it("returns refs with no non-pruned connected lead, excluding archived", () => {
    const out = unfiledReferences(refs, leads).map((r) => r.id);
    expect(out).toEqual(["r2"]); // r1 attached, r2 only pruned-attached, r3 archived
  });
});
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd apps/web && npx vitest run test/workspace/blocks/exploration/ExplorationView.test.tsx -t unfiledReferences`
Expected: FAIL — `unfiledReferences` not exported.

- [ ] **Step 3: Write minimal implementation**

In `ExplorationView.tsx`:

1) Add imports:

```ts
import { adoptCandidate, attachReference, createEdge, /* …existing… */ } from "../../../api/exploration";
import { PlacementPicker, type PlacementQuestion } from "./PlacementPicker";
import { UNFILED_NODE_ID } from "./WarrenMap";
```

2) Export the pure helper (top level, beside `zoomMemo`):

```ts
// 未归类 = references with no NON-PRUNED connected lead (read or unread — the
// whole point is faithfulness), excluding archived ones. Computed client-side;
// the server's danglingSourceIds (read-but-unfollowed) is a different, sharper set.
export function unfiledReferences(references: Reference[], leads: ExplorationLead[]): Reference[] {
  const attached = new Set<string>();
  for (const l of leads) {
    if (l.status !== "pruned" && l.connectedReferenceId) attached.add(l.connectedReferenceId);
  }
  return references.filter((r) => !(r.archived ?? false) && !attached.has(r.id));
}
```

3) Extend `ZoomState` to a third mode and its memo (line 41-42):

```ts
type ZoomState = { mode: "map" | "hole" | "unfiled"; focusRootId: string | null };
```

4) Compute the unfiled list + question list (near `roots`, line 406):

```ts
  const unfiled = useMemo(() => unfiledReferences(references, view.leads), [references, view.leads]);
  const placementQuestions = useMemo<PlacementQuestion[]>(
    () =>
      view.leads
        .filter((l) => l.status !== "pruned" && l.connectedReferenceId == null)
        .map((l) => ({ id: l.id, text: l.text, parentId: l.parentLeadId })),
    [view.leads],
  );
```

5) Attach handler (beside `adopt`):

```ts
  const [attaching, setAttaching] = useState<string | null>(null); // referenceId in flight
  async function attach(referenceId: string, parentLeadId: string) {
    if (attaching) return;
    setAttaching(referenceId);
    setActionError(false);
    try {
      await attachReference(projectId, referenceId, parentLeadId);
      await refresh();
      onLibraryChanged?.();
    } catch {
      setActionError(true);
    } finally {
      setAttaching(null);
    }
  }
```

6) Pass the two new props wherever `<WarrenMap … />` is rendered (line ~592):

```tsx
            <WarrenMap
              projectId={projectId}
              roots={roots}
              countByRoot={countByRoot}
              edges={view.edges}
              onZoom={zoomInto}
              unfiledCount={unfiled.length}
              onOpenUnfiled={() => goZoom({ mode: "unfiled", focusRootId: null })}
              busyEdgeIds={busyEdgeIds}
              onConfirmEdge={confirmEdge}
              onDismissEdge={dismissEdge}
              onRelabelEdge={relabelEdge}
              onCreateEdge={createRelation}
              onDeleteLead={removeRoot}
            />
```

7) Render the `"unfiled"` panel. Add a branch BEFORE the `inHole` branch (or after — order doesn't matter as long as it's its own early return), mirroring the hole layout (back button + the two-page aux column):

```tsx
  if (zoom.mode === "unfiled") {
    const unfiledMain = (
      <div className="flex min-h-0 flex-1 flex-col">
        <div className="flex flex-none items-center gap-2 border-b border-mk-border bg-mk-surface px-4 py-2.5">
          <button
            type="button"
            onClick={backToMap}
            className="flex-none rounded-full border border-mk-border bg-mk-surface px-2.5 py-1 text-[12px] font-bold text-mk-accent hover:border-mk-accent hover:bg-mk-accent-50"
          >
            ← 返回兔子洞地图
          </button>
          <h2 className="min-w-0 truncate font-sans text-[14px] font-bold text-mk-ink">未归类的来源 · {unfiled.length} 篇</h2>
        </div>
        <div className="mk-scroll min-h-0 flex-1 overflow-y-auto px-6 py-4">
          {unfiled.length === 0 ? (
            <div className="flex h-full items-center justify-center">
              <EmptyState illustration="warren" title="都归好位了" body="每一篇来源都挂到了某个问题下——干净。" />
            </div>
          ) : (
            <ul className="flex flex-col gap-3">
              {unfiled.map((ref) => (
                <li key={ref.id} className="rounded-mk-md border border-mk-border bg-mk-surface p-3">
                  <p className="text-[14px] font-bold text-mk-ink">{ref.title || "未命名来源"}</p>
                  <div className="mt-2 flex flex-col gap-2">
                    {onEnterReading && (
                      <button
                        type="button"
                        onClick={() => void enterSource(ref)}
                        disabled={enteringRefId === ref.id}
                        className="self-start rounded-mk border border-mk-border px-2.5 py-1 text-[12px] font-bold text-mk-accent hover:bg-mk-accent-50 disabled:opacity-50"
                      >
                        {enteringRefId === ref.id ? "打开中…" : "进入阅读室"}
                      </button>
                    )}
                    <div className="rounded-mk border border-mk-border bg-mk-paper p-2.5">
                      <p className="mb-2 text-[12px] font-bold text-mk-faint">挂到问题下</p>
                      <PlacementPicker
                        questions={placementQuestions}
                        suggestedLeadId={null}
                        reason=""
                        busy={attaching === ref.id}
                        onPick={(leadId) => leadId && void attach(ref.id, leadId)}
                      />
                    </div>
                  </div>
                </li>
              ))}
            </ul>
          )}
        </div>
      </div>
    );
    return (
      <div className="relative flex h-full min-h-0 bg-mk-paper">
        {auxOnLeft && auxColumn}
        {unfiledMain}
        {!auxOnLeft && auxColumn}
      </div>
    );
  }
```

> The 未归类 panel uses `PlacementPicker` with `suggestedLeadId={null}` (no per-item LLM call in the bulk list — the add-time flow in Task 9 is where 印记 actively suggests). `onPick(null)` in this panel is a no-op (the source is already unfiled), so only a real `leadId` triggers `attach`.

- [ ] **Step 4: Run test to verify it passes**

Run: `cd apps/web && npx vitest run test/workspace/blocks/exploration/ExplorationView.test.tsx`
Expected: PASS (the new `unfiledReferences` test + all existing tests — they now supply the WarrenMap props transitively through ExplorationView).

- [ ] **Step 5: Commit**

```bash
git add apps/web/src/workspace/blocks/exploration/ExplorationView.tsx apps/web/test/workspace/blocks/exploration/ExplorationView.test.tsx
git commit -m "feat(web): 未归类 set + panel + attach wiring in ExplorationView"
```

---

### Task 9: 加来源即归位 —— `ReadingBlock` 的 `PlacementModal`

`addSource` 建完 reference 后，弹一个归位弹窗：拉取问题清单 + 调 `suggestPlacement` 预高亮建议 + `PlacementPicker` → `attachReference`。无问题则跳过（悄悄进未归类）。

**Files:**
- Create: `apps/web/src/workspace/blocks/exploration/PlacementModal.tsx`
- Modify: `apps/web/src/workspace/blocks/ReadingBlock.tsx`（`addSource` 后开弹窗 + 渲染弹窗）
- Test: `apps/web/test/workspace/blocks/exploration/PlacementModal.test.tsx`

**Interfaces:**
- Consumes: `getExploration`、`suggestPlacement`、`attachReference`（Task 5）、`PlacementPicker`/`PlacementQuestion`（Task 6）。
- Produces:
  ```ts
  export function PlacementModal(props: {
    projectId: string;
    referenceId: string;
    onClose: () => void;
    onAttached?: () => void; // parent reloads library/graph
  }): JSX.Element | null
  ```

- [ ] **Step 1: Write the failing test**

```tsx
import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import { PlacementModal } from "../../../../src/workspace/blocks/exploration/PlacementModal";
import * as expl from "../../../../src/api/exploration";

describe("PlacementModal", () => {
  beforeEach(() => vi.restoreAllMocks());

  it("suggests then attaches on pick", async () => {
    vi.spyOn(expl, "getExploration").mockResolvedValue({
      leads: [{ id: "q1", text: "主问题", status: "open", origin: "guide", sourceReferenceId: null, connectedReferenceId: null, position: 0, parentLeadId: null, createdAt: "2026-08-10T00:00:00Z" }],
      danglingSourceIds: [],
      edges: [],
    } as any);
    vi.spyOn(expl, "suggestPlacement").mockResolvedValue({ leadId: "q1", reason: "很贴" });
    const attach = vi.spyOn(expl, "attachReference").mockResolvedValue({ lead: {} as any, reference: {} as any });
    const onAttached = vi.fn();
    const onClose = vi.fn();

    render(<PlacementModal projectId="p1" referenceId="r1" onClose={onClose} onAttached={onAttached} />);
    await waitFor(() => expect(screen.getByText("很贴")).toBeInTheDocument());
    fireEvent.click(screen.getByRole("button", { name: /主问题/ }));
    await waitFor(() => expect(attach).toHaveBeenCalledWith("p1", "r1", "q1"));
    expect(onAttached).toHaveBeenCalled();
    expect(onClose).toHaveBeenCalled();
  });
});
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd apps/web && npx vitest run test/workspace/blocks/exploration/PlacementModal.test.tsx`
Expected: FAIL — module not found.

- [ ] **Step 3: Write minimal implementation**

`apps/web/src/workspace/blocks/exploration/PlacementModal.tsx`:

```tsx
import { useEffect, useState } from "react";
import { getExploration, suggestPlacement, attachReference } from "../../../api/exploration";
import { PlacementPicker, type PlacementQuestion } from "./PlacementPicker";

// PlacementModal — 加来源即归位. After a reference is created, ask 印记 for the
// best-fit question (pre-highlighted) and let the student place it (or 未归类).
// If the project has no questions yet, there's nothing to place under → the
// caller shouldn't even open this; but if opened, it shows the 未归类 escape.
export function PlacementModal({
  projectId,
  referenceId,
  onClose,
  onAttached,
}: {
  projectId: string;
  referenceId: string;
  onClose: () => void;
  onAttached?: () => void;
}) {
  const [questions, setQuestions] = useState<PlacementQuestion[]>([]);
  const [suggestedLeadId, setSuggestedLeadId] = useState<string | null>(null);
  const [reason, setReason] = useState("");
  const [loading, setLoading] = useState(true);
  const [busy, setBusy] = useState(false);

  useEffect(() => {
    let cancelled = false;
    (async () => {
      try {
        const view = await getExploration(projectId);
        if (cancelled) return;
        setQuestions(
          view.leads
            .filter((l) => l.status !== "pruned" && l.connectedReferenceId == null)
            .map((l) => ({ id: l.id, text: l.text, parentId: l.parentLeadId })),
        );
        // Suggestion is best-effort — a failure just means no pre-highlight.
        try {
          const s = await suggestPlacement(projectId, referenceId);
          if (!cancelled) {
            setSuggestedLeadId(s.leadId);
            setReason(s.reason);
          }
        } catch {
          /* no suggestion; manual pick still works */
        }
      } finally {
        if (!cancelled) setLoading(false);
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [projectId, referenceId]);

  async function pick(leadId: string | null) {
    if (busy) return;
    if (leadId == null) {
      onClose(); // 未归类 — leave it unattached
      return;
    }
    setBusy(true);
    try {
      await attachReference(projectId, referenceId, leadId);
      onAttached?.();
      onClose();
    } catch {
      setBusy(false); // let the student retry / choose 未归类
    }
  }

  return (
    <div className="absolute inset-0 z-30 flex items-center justify-center bg-black/40 px-6" onClick={onClose}>
      <div className="w-[440px] rounded-mk-lg border border-mk-border bg-mk-surface p-5 shadow-[0_20px_60px_rgba(28,35,51,0.25)]" onClick={(e) => e.stopPropagation()}>
        <div className="mb-3 flex items-center justify-between">
          <h3 className="font-sans text-[16px] font-bold text-mk-ink">这篇挂到哪个问题下？</h3>
          <button type="button" onClick={onClose} className="text-[18px] leading-none text-mk-faint hover:text-mk-ink">×</button>
        </div>
        {loading ? (
          <p className="py-6 text-center text-[13px] text-mk-faint">印记在想它属于哪儿…</p>
        ) : (
          <PlacementPicker questions={questions} suggestedLeadId={suggestedLeadId} reason={reason} busy={busy} onPick={pick} />
        )}
      </div>
    </div>
  );
}
```

In `ReadingBlock.tsx`:

1) Import: `import { PlacementModal } from "./exploration/PlacementModal";`
2) State (with the other `useState`s): `const [placeFor, setPlaceFor] = useState<string | null>(null);` (holds the just-created referenceId to place).
3) In `addSource` (line 299-315), after `setSelId(created.id);`, open the placement modal instead of just closing:

```ts
      setRefs((xs) => [created, ...xs]);
      setSelId(created.id);
      setPlaceFor(created.id); // 加来源即归位 — ask 印记 + let the student place it
```

(leave the `finally { setAdding(false); }` — the AddSourceModal still closes; the PlacementModal opens on top.)

4) Render the modal near the other modals in ReadingBlock's JSX (wherever `AddSourceModal` is conditionally rendered):

```tsx
      {placeFor && (
        <PlacementModal
          projectId={projectId}
          referenceId={placeFor}
          onClose={() => setPlaceFor(null)}
          onAttached={() => reload()}
        />
      )}
```

> `reload()` is ReadingBlock's existing library/graph refresh (it drives `explorationSignal` too). `createUntrackedSource` and `addPastedSource` are intentionally NOT wired to placement here: the untracked-source path already lives inside the graph (a later polish could place it), and the paste path opens the reading room immediately. Both land in 未归类 and are placeable from the panel (Task 8). Keep this task to the `addSource` (link/upload/manual) path.

- [ ] **Step 4: Run test to verify it passes**

Run: `cd apps/web && npx vitest run test/workspace/blocks/exploration/PlacementModal.test.tsx`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add apps/web/src/workspace/blocks/exploration/PlacementModal.tsx apps/web/src/workspace/blocks/ReadingBlock.tsx apps/web/test/workspace/blocks/exploration/PlacementModal.test.tsx
git commit -m "feat(web): add-time source placement modal (印记 suggests, student places)"
```

---

## Final verification (after all tasks)

- [ ] `cd apps/api && go test ./internal/api/ ./internal/agent/` — full packages green (testcontainers, foreground).
- [ ] `cd apps/web && npx vitest run` — full web suite green.
- [ ] `pnpm --filter web typecheck` and `pnpm --filter @mind-imprint/contracts typecheck` — clean.
- [ ] `pnpm --filter web build` — clean.
- [ ] Manual smoke (dev or prod after deploy): add a source via 链接/DOI → placement modal appears with 印记's pre-highlighted suggestion → pick a question → the source shows as a paper node in that question's hole; add another and pick 未归类 → the 未归类 node appears on the map with count 1 → open it → attach it to a question → node count updates, 未归类 hides when empty.

## Self-review notes (author)

- **Spec §3a attach:** Task 1 (parent-must-be-question 400, idempotency, origin manual, no new reference). ✅
- **Spec §3b suggest + PlacementSuggestion contract:** Tasks 2/3/4 (no-questions → null no-call; unknown id dropped; metered; behind resolveFast). ✅
- **Spec §4a add-time placement + 印记 suggests:** Task 9. ✅
- **Spec §4b 未归类 node + panel (read or unread; count-gated; two-page aux):** Tasks 7/8. ✅
- **Spec §4c shared PlacementPicker:** Task 6, reused by 8 + 9. ✅
- **Spec "kept separate" (danglingSourceIds untouched):** no task edits `computeDanglingSourceIds` or `getExploration`. ✅
- **Naming collision note (collection「未归类」 vs node「未归类」):** flagged in Global Constraints. ⚠️ working name retained per user.
- **Type consistency:** `PlacementSuggestion {leadId, reason}`, `PlacementQuestion {id,text,parentId}`, `attachReference(projectId, referenceId, parentLeadId)`, `UNFILED_NODE_ID`, `unfiledReferences(references, leads)` — used identically across tasks. ✅
