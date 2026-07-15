# Slice 8b — Examiner-Voice Switching + Budget-Deletion Coaching Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add switchable examiner voices (board default + 3 generic postures) to the S5 整稿体检 whole-draft review, and deterministic over-budget deletion coaching, on top of the Slice 8 keystone writing surface.

**Architecture:** Voices are a fixed Go enum selecting the review's system-prompt posture; the review's idempotency key extends from `snapshot` to `(snapshot, voice)` via a `voice` field inside the existing intervention `anchor` jsonb — **no migration**. A pure `agent.BudgetVerdict(wordCount, band)` helper yields `{state, delta}`, projected onto the snapshot DTO so the client can render a count-vs-band verdict; when a snapshot is over budget, the review runs with an added deletion-lens instruction. The review projection becomes a flat `items[]` array where each item carries its `voice`, so the frontend derives per-voice work-orders and the cached-voice set by filtering.

**Tech Stack:** Go (`net/http`, `sqlc`, testcontainers), TypeScript + React + vitest, Zod contracts.

Spec: `docs/superpowers/specs/2026-07-15-slice-8b-examiner-voices-and-budget-coaching-design.md`

## Global Constraints

- **RL-1 (structural):** the AI never writes prose. Voices change tone/lens only, never output shape. Every voice writes only `review_item` intervention rows (typed advice). The over-budget deletion lens asks diagnostic questions ("这段在向哪张表交证据？") — never "删掉…" and never a rewritten/substitute sentence. Banned-phrasing enforcement runs per voice, all-or-nothing, and rejects the whole review on any violation.
- **克制:** budget coaching is a question about which criterion a paragraph serves, not an instruction; the deterministic verdict is a neutral fact.
- **Cost recorded:** every real model call (each voice's first run) records 档位+token+成本 via `RecordLLMCall`. A replay (already-run voice on the same snapshot) makes NO model call.
- **No migration, no skill-config change.** Voice lives inside the intervention `anchor` jsonb; the word band is existing skill config.
- **Voice enum (exact):** `board | sceptic | layperson | executioner`. `board` = the existing `reviewPosturePrompt` verbatim. Unknown/empty `?voice=` → `board`.
- **Back-compat:** keystone review rows have anchors without a `voice` key; the replay filter and the projection both read a missing anchor voice as `board`. Existing `…/review` calls without `?voice=` resolve to `board`.
- **Budget verdict:** `state ∈ {in, over, under}`; `delta` is a **non-negative magnitude** (words past `max` when over, words short of `min` when under, `0` when in). UI copy: `snapshotMeta` clause `· 超出 {delta} 字` / `· 还差 {delta} 字` / `· 在预算内`; over-budget work-order note (verbatim) `超预算 {delta} 字 · 删减决策按「这段在向哪张表交证据」来做`.
- **Voice pill labels (exact):** `考官` (board) · `怀疑` (sceptic) · `外行` (layperson) · `字数` (executioner). Reuse the 编辑/预览 segmented styling; selected pill uses the active-tab treatment; a cached voice shows a small marker.
- **Go tests:** `CGO_ENABLED=0 go test -p 1 ./...` on a quiet Docker daemon; FULL packages for the gate, not `-run` subsets. sqlc → `make sqlc` from `apps/api`. Web → `npm test` (vitest). Contracts → `npm test`.
- **Binding design copy verbatim** (`docs/design/思维印记_工作区.dc.html` 写作 view). Icons inline SVG, never lucide-react.
- **Never `git add` a whole dir with untracked user files;** add named paths only. The pre-existing `M package.json` + untracked user files are NOT ours — leave them.

---

## File Structure

- `apps/api/internal/agent/review.go` — MODIFY: `Voice` type, `ParseVoice`, `reviewSystemPrompt(voice, overBudget)`, `ProposeReview(…, voice, overBudget)`.
- `apps/api/internal/agent/review_test.go` — MODIFY: update the two existing calls; add voice/over-budget tests.
- `apps/api/internal/agent/budget.go` — CREATE: pure `BudgetVerdict`.
- `apps/api/internal/agent/budget_test.go` — CREATE.
- `apps/api/internal/api/writing.go` — MODIFY: `orderReview` parses `?voice=`, voice-scoped anchor + replay filter, over-budget wiring.
- `apps/api/internal/api/writing_test.go` — MODIFY: add per-voice + keystone-row-as-board tests.
- `apps/api/internal/studio/dto.go` — MODIFY: `WritingBudgetDTO`, `budget` on `WritingSnapshotDTO`, `voice` on `WritingReviewItemDTO`, drop `Ordered` from `WritingReviewDTO`.
- `apps/api/internal/studio/projection.go` — MODIFY: fill budget; tag review items with voice; carry all voices; drop `Ordered`.
- `apps/api/internal/studio/*_test.go` — MODIFY: projection + parity assertions.
- `packages/contracts/src/studioState.ts` — MODIFY: `WritingBudget`, `voice` on `WritingReviewItem`, `review` shape.
- `packages/contracts/src/*.test.ts` — MODIFY/ADD: schema tests.
- `apps/web/src/api/writing.ts` — MODIFY: `orderReview(projectId, snapshotId, voice)`.
- `apps/web/src/studio/views/WritingView.tsx` — MODIFY: voice pills, budget-enriched `snapshotMeta`, budget note, per-voice item selection.
- `apps/web/src/studio/StudioContainer.tsx` — MODIFY: `selectedVoice` state, `onOrderReview(snapshotId, voice)`.
- web test files + `fixtures.ts` — MODIFY: backfill the new `voice`/`budget` fields, drop `ordered`.

---

### Task 1: Examiner-voice postures in `agent/review.go`

**Files:**
- Modify: `apps/api/internal/agent/review.go`
- Test: `apps/api/internal/agent/review_test.go`

**Interfaces:**
- Produces:
  - `type Voice string`; consts `VoiceBoard="board"`, `VoiceSceptic="sceptic"`, `VoiceLayperson="layperson"`, `VoiceExecutioner="executioner"`.
  - `func ParseVoice(s string) Voice` — maps the four exact strings; anything else → `VoiceBoard`.
  - `func reviewSystemPrompt(voice Voice, overBudget bool) string` — pure posture builder.
  - `func ProposeReview(ctx, prov, r, criteria, paragraphs, graphSummary string, voice Voice, overBudget bool) ([]ReviewItem, gateway.ChatUsage, error)` (two new trailing params).
- Consumes: existing `reviewItemWire`, `enforcement.BannedPhrasing`, `gateway.Collect`.

- [ ] **Step 1: Write the failing tests**

Append to `apps/api/internal/agent/review_test.go`:

```go
func TestParseVoice(t *testing.T) {
	cases := map[string]Voice{
		"board": VoiceBoard, "sceptic": VoiceSceptic, "layperson": VoiceLayperson,
		"executioner": VoiceExecutioner, "": VoiceBoard, "nonsense": VoiceBoard, "BOARD": VoiceBoard,
	}
	for in, want := range cases {
		if got := ParseVoice(in); got != want {
			t.Errorf("ParseVoice(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestReviewSystemPrompt_DistinctPerVoice(t *testing.T) {
	board := reviewSystemPrompt(VoiceBoard, false)
	if board != reviewPosturePrompt {
		t.Fatal("board voice must be the existing reviewPosturePrompt verbatim")
	}
	seen := map[string]bool{}
	for _, v := range []Voice{VoiceBoard, VoiceSceptic, VoiceLayperson, VoiceExecutioner} {
		p := reviewSystemPrompt(v, false)
		if seen[p] {
			t.Fatalf("voice %q produced a duplicate posture", v)
		}
		seen[p] = true
		// Every voice keeps the RL-1 iron rule (never rewrite / never a model sentence).
		if !strings.Contains(p, "绝不") {
			t.Fatalf("voice %q dropped the iron rule", v)
		}
	}
}

func TestReviewSystemPrompt_OverBudgetAppendsDeletionLens(t *testing.T) {
	base := reviewSystemPrompt(VoiceExecutioner, false)
	over := reviewSystemPrompt(VoiceExecutioner, true)
	if base == over {
		t.Fatal("overBudget must append a deletion-lens instruction")
	}
	if !strings.Contains(over, "删减") || !strings.Contains(over, "哪张表") {
		t.Fatalf("over-budget posture missing the deletion-lens frame: %s", over)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd apps/api && CGO_ENABLED=0 go test ./internal/agent/ -run 'TestParseVoice|TestReviewSystemPrompt'`
Expected: FAIL — `undefined: ParseVoice`, `undefined: reviewSystemPrompt`.

- [ ] **Step 3: Add the voice type, postures, and prompt builder**

In `apps/api/internal/agent/review.go`, after the `reviewPosturePrompt` const (line ~39), add:

```go
// Voice selects the examiner posture the whole-draft review performs. board is
// the default (the existing reviewPosturePrompt); the three generic voices are
// board-agnostic postures. Voices change tone and lens only — never the JSON
// output shape and never the RL-1 iron rule.
type Voice string

const (
	VoiceBoard       Voice = "board"
	VoiceSceptic     Voice = "sceptic"
	VoiceLayperson   Voice = "layperson"
	VoiceExecutioner Voice = "executioner"
)

// ParseVoice maps an untrusted ?voice= value to a Voice; anything unrecognized
// (including "") falls back to the default board voice, so every existing call
// stays valid.
func ParseVoice(s string) Voice {
	switch Voice(s) {
	case VoiceSceptic, VoiceLayperson, VoiceExecutioner:
		return Voice(s)
	default:
		return VoiceBoard
	}
}

// The three generic postures keep the SAME iron rule and JSON-array output as
// the board voice; only the stance differs.
const reviewPostureSceptic = `你是一位「整稿体检」考官，天生不信任每一个论断。学生已提交一版草稿快照。
对照给定的评分表，逐表指出：哪些说法只是断言、还没把证据摆出来，哪里的结论跑在了支撑前面。
铁律：绝不替学生改写句子、绝不给示范句、绝不续写。你的「建议」只能是"要拿出什么证据/要补什么支撑"的方向，
不能是可直接粘贴的成品句子。一次只输出 JSON 数组，每个评分表一个对象。`

const reviewPostureLayperson = `你是一位友善但完全外行的读者，不懂这个领域。学生已提交一版草稿快照。
对照给定的评分表，逐表指出：哪里有没解释的术语、没定义的概念、跳过了的推理步骤——凡是你这个外行读不懂的地方。
铁律：绝不替学生改写句子、绝不给示范句、绝不续写。你的「建议」只能是"要解释什么/要补哪一步"的方向，
不能是可直接粘贴的成品句子。一次只输出 JSON 数组，每个评分表一个对象。`

const reviewPostureExecutioner = `你是一位盯字数的「整稿体检」考官。学生已提交一版草稿快照。
对照给定的评分表，逐表追问：每一段文字有没有挣到它占的字数——哪些段落不向任何一张表交证据、纯属背景或冗余。
铁律：绝不替学生改写句子、绝不给示范句、绝不续写。你的「建议」只能是"哪一段可以砍/它本该向哪张表交证据"的方向，
不能是可直接粘贴的成品句子。一次只输出 JSON 数组，每个评分表一个对象。`

// The deletion-lens clause appended when the reviewed snapshot is over its word
// band — it reuses the review's paragraph⇄评分表 mapping to frame cuts as the
// student's decision. Diagnostic questions only (RL-1): never "删掉这段".
const reviewOverBudgetLens = `另外：这一稿已经超出字数预算。对交证据最少的那些段落，指出它们各自在向哪张表交证据；
如果一张表都不向，就把「这 N 字在向哪张表交证据」这个删减决策摆到学生面前，让她自己决定砍哪一段——你不替她删。`

// reviewSystemPrompt builds the system content for a review: the posture for
// the chosen voice, plus the deletion lens when overBudget. Pure — no I/O — so
// posture selection is unit-testable without a model.
func reviewSystemPrompt(voice Voice, overBudget bool) string {
	var base string
	switch voice {
	case VoiceSceptic:
		base = reviewPostureSceptic
	case VoiceLayperson:
		base = reviewPostureLayperson
	case VoiceExecutioner:
		base = reviewPostureExecutioner
	default:
		base = reviewPosturePrompt
	}
	if overBudget {
		return base + "\n" + reviewOverBudgetLens
	}
	return base
}
```

- [ ] **Step 4: Thread voice + overBudget through `ProposeReview`**

Change the `ProposeReview` signature and the system message. Replace the signature line and the `Content: reviewPosturePrompt` line:

```go
func ProposeReview(ctx context.Context, prov gateway.Provider, r gateway.Resolved, criteria []skills.ReviewCriterion, paragraphs []string, graphSummary string, voice Voice, overBudget bool) ([]ReviewItem, gateway.ChatUsage, error) {
```

and inside the `gateway.Collect` call, change the system message content:

```go
			{Role: gateway.RoleSystem, Content: reviewSystemPrompt(voice, overBudget)},
```

Everything else in `ProposeReview` is unchanged.

- [ ] **Step 5: Update the two existing tests to the new signature**

In `apps/api/internal/agent/review_test.go`, both existing `ProposeReview(...)` calls gain two trailing args. `TestProposeReview_ParsesWorkOrder`:

```go
	items, usage, err := ProposeReview(context.Background(), reviewProvider(reply), gateway.Resolved{Provider: "deepseek", Model: "x"}, criteria, []string{"p1", "p2"}, "claims:1 evidence:2", VoiceBoard, false)
```

`TestProposeReview_RejectsBannedPhrase` (rename intent: banned-phrasing rejects under a generic voice too — pass `VoiceSceptic`):

```go
	_, _, err := ProposeReview(context.Background(), reviewProvider(reply), gateway.Resolved{Provider: "deepseek", Model: "x"}, criteria, []string{"p1"}, "", VoiceSceptic, false)
```

- [ ] **Step 6: Run the agent tests to verify they pass**

Run: `cd apps/api && CGO_ENABLED=0 go test ./internal/agent/`
Expected: PASS (all, including the two updated + three new).

- [ ] **Step 7: Commit**

```bash
cd apps/api && git add internal/agent/review.go internal/agent/review_test.go
git commit -m "feat(refactor2): examiner-voice postures + over-budget deletion lens in ProposeReview (Slice 8b T1)"
```

---

### Task 2: Pure `agent.BudgetVerdict`

**Files:**
- Create: `apps/api/internal/agent/budget.go`
- Test: `apps/api/internal/agent/budget_test.go`

**Interfaces:**
- Consumes: `skills.WordBudget` (`{Min, Max int}`).
- Produces: `func BudgetVerdict(wc int, band *skills.WordBudget) (state string, delta int)` — `state` ∈ `"in"|"over"|"under"`; `delta` is a non-negative magnitude; a nil band → `("in", 0)` (no band configured means nothing to violate).

- [ ] **Step 1: Write the failing test**

Create `apps/api/internal/agent/budget_test.go`:

```go
package agent

import (
	"testing"

	"mindimprint/api/internal/skills"
)

func TestBudgetVerdict(t *testing.T) {
	band := &skills.WordBudget{Min: 1500, Max: 2000}
	cases := []struct {
		wc        int
		wantState string
		wantDelta int
	}{
		{1780, "in", 0},
		{1500, "in", 0},
		{2000, "in", 0},
		{2340, "over", 340},
		{2001, "over", 1},
		{1290, "under", 210},
		{1499, "under", 1},
	}
	for _, c := range cases {
		state, delta := BudgetVerdict(c.wc, band)
		if state != c.wantState || delta != c.wantDelta {
			t.Errorf("BudgetVerdict(%d) = (%q,%d), want (%q,%d)", c.wc, state, delta, c.wantState, c.wantDelta)
		}
	}
	if state, delta := BudgetVerdict(999, nil); state != "in" || delta != 0 {
		t.Errorf("BudgetVerdict(_, nil) = (%q,%d), want (in,0)", state, delta)
	}
}
```

- [ ] **Step 2: Run it to verify it fails**

Run: `cd apps/api && CGO_ENABLED=0 go test ./internal/agent/ -run TestBudgetVerdict`
Expected: FAIL — `undefined: BudgetVerdict`.

- [ ] **Step 3: Implement**

Create `apps/api/internal/agent/budget.go`:

```go
package agent

import "mindimprint/api/internal/skills"

// BudgetVerdict classifies a word count against a skill's word band. state is
// "in" when Min ≤ wc ≤ Max (inclusive both ends), "over" above Max, "under"
// below Min. delta is a NON-NEGATIVE magnitude — words past Max when over,
// words short of Min when under, 0 when in — so the UI reads direction from
// state and renders delta directly (超出 {delta} / 还差 {delta}). A nil band
// means no budget is configured: nothing to violate, so ("in", 0).
func BudgetVerdict(wc int, band *skills.WordBudget) (state string, delta int) {
	if band == nil {
		return "in", 0
	}
	switch {
	case wc > band.Max:
		return "over", wc - band.Max
	case wc < band.Min:
		return "under", band.Min - wc
	default:
		return "in", 0
	}
}
```

- [ ] **Step 4: Run it to verify it passes**

Run: `cd apps/api && CGO_ENABLED=0 go test ./internal/agent/ -run TestBudgetVerdict`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
cd apps/api && git add internal/agent/budget.go internal/agent/budget_test.go
git commit -m "feat(refactor2): pure agent.BudgetVerdict word-band classifier (Slice 8b T2)"
```

---

### Task 3: Voice-scoped review in `api/writing.go`

**Files:**
- Modify: `apps/api/internal/api/writing.go`
- Test: `apps/api/internal/api/writing_test.go`

**Interfaces:**
- Consumes: `agent.ParseVoice`, `agent.Voice`, `agent.BudgetVerdict`, `agent.CountWords`, the modified `agent.ProposeReview` (Task 1).
- Produces: `orderReview` handling `?voice=`; `reviewItemsForSnapshot(ctx, q, projectID, sid, voice)` (new trailing `voice agent.Voice` param). The anchor written for review items becomes `{kind, id, voice}`.

- [ ] **Step 1: Write the failing tests**

Append to `apps/api/internal/api/writing_test.go`. (Reuses existing helpers: `newAPITestPool`, `signInSeed`, `materialsTestProjectID`, `withCookie`, `reviewStubProvider`, `fakeResolver`, `countReviewItems`, `countLLMCalls`, `mustUUID`, and the review-anchor query. To count rows for a specific voice, add the small helper shown in Step 4.)

```go
// Two voices on the SAME snapshot are independent caches: each does one model
// call and persists its own disjoint row set; re-running a voice replays with
// no new call. A keystone row (anchor with no voice) is served as board.
func TestOrderReview_PerVoiceIndependentCaches(t *testing.T) {
	pool := newAPITestPool(t)
	reply := `[{"criterion_code":"表E","band":"5–6 段","evidence":"e","missing":"m","fix":"补定义"}]`
	h := New(Deps{
		Queries: sqlc.New(pool), Pool: pool,
		Provider: reviewStubProvider(reply), ChatResolver: fakeResolver(),
	}).Handler()
	cookie := signInSeed(t, pool)
	projectID := materialsTestProjectID

	content := strings.Repeat("字", 1600)
	rec := httptest.NewRecorder()
	req := withCookie(httptest.NewRequest("POST", "/api/v1/projects/"+projectID+"/snapshots",
		strings.NewReader(`{"content":`+strconv.Quote(content)+`}`)), cookie)
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("commit = %d; body=%s", rec.Code, rec.Body)
	}
	var snap struct{ ID string `json:"id"` }
	_ = json.Unmarshal(rec.Body.Bytes(), &snap)

	order := func(voice string) string {
		r := httptest.NewRecorder()
		url := "/api/v1/projects/" + projectID + "/snapshots/" + snap.ID + "/review"
		if voice != "" {
			url += "?voice=" + voice
		}
		req := withCookie(httptest.NewRequest("POST", url, strings.NewReader("")), cookie)
		h.ServeHTTP(r, req)
		if r.Code != http.StatusOK {
			t.Fatalf("order review voice=%q = %d; body=%s", voice, r.Code, r.Body)
		}
		return r.Body.String()
	}

	order("board")
	callsAfterBoard := countLLMCalls(t, pool, projectID)
	if callsAfterBoard != 1 {
		t.Fatalf("after board review, llm_calls = %d, want 1", callsAfterBoard)
	}
	if n := countReviewItems(t, pool, projectID); n != 1 {
		t.Fatalf("after board review, review_items = %d, want 1", n)
	}

	order("sceptic") // distinct voice → a second, independent model call + row
	if n := countLLMCalls(t, pool, projectID); n != 2 {
		t.Fatalf("after sceptic review, llm_calls = %d, want 2 (independent cache)", n)
	}
	if n := countReviewItems(t, pool, projectID); n != 2 {
		t.Fatalf("after sceptic review, review_items = %d, want 2 (disjoint set)", n)
	}

	order("sceptic") // replay — no new call, no new row
	if n := countLLMCalls(t, pool, projectID); n != 2 {
		t.Fatalf("after sceptic replay, llm_calls = %d, want 2 (replay, no new call)", n)
	}
	if n := countReviewItems(t, pool, projectID); n != 2 {
		t.Fatalf("after sceptic replay, review_items = %d, want 2 (replay)", n)
	}

	// Per-voice row counts: exactly one board row, one sceptic row.
	if n := countReviewItemsForVoice(t, pool, projectID, "board"); n != 1 {
		t.Fatalf("board rows = %d, want 1", n)
	}
	if n := countReviewItemsForVoice(t, pool, projectID, "sceptic"); n != 1 {
		t.Fatalf("sceptic rows = %d, want 1", n)
	}
}
```

- [ ] **Step 2: Run it to verify it fails**

Run: `cd apps/api && CGO_ENABLED=0 go test ./internal/api/ -run TestOrderReview_PerVoiceIndependentCaches`
Expected: FAIL — `undefined: countReviewItemsForVoice`, and (once that compiles) the second voice reuses the first's rows because the anchor/filter ignores voice.

- [ ] **Step 3: Add voice to the anchor + replay filter in `orderReview`**

In `apps/api/internal/api/writing.go`:

Parse the voice near the top of `orderReview`, after `sid` is parsed (line ~270):

```go
	voice := agent.ParseVoice(r.URL.Query().Get("voice"))
```

Change the `existing := reviewItemsForSnapshot(...)` call (line ~278) to pass the voice:

```go
	existing := reviewItemsForSnapshot(r.Context(), a.d.Queries, projectID, sid, voice)
```

Compute over-budget before the `ProposeReview` call. Replace the `paras := snapshotParagraphs(...)` / `items, usage, perr := agent.ProposeReview(...)` block (lines ~315-316) with:

```go
	paras := snapshotParagraphs(snap.Content)
	sbState, _ := agent.BudgetVerdict(agent.CountWords(snap.Content), sk.WordBudget)
	items, usage, perr := agent.ProposeReview(r.Context(), a.d.Provider, resolved, sk.ReviewCriteria, paras, graphSummary(r.Context(), a.d.Queries, projectID), voice, sbState == "over")
```

Change the anchor written for persisted rows (line ~340) to carry the voice:

```go
	anchor := mustJSON(map[string]string{"kind": "draft_snapshot", "id": sid.String(), "voice": string(voice)})
```

- [ ] **Step 4: Make `reviewItemsForSnapshot` voice-aware**

Change its signature and anchor decode (lines ~419-435):

```go
func reviewItemsForSnapshot(ctx context.Context, q *sqlc.Queries, projectID, sid uuid.UUID, voice agent.Voice) []agent.ReviewItem {
	ivs, err := q.ListInterventionsByProject(ctx, projectID)
	if err != nil {
		return nil
	}
	out := []agent.ReviewItem{}
	for _, iv := range ivs {
		if iv.Type != "review_item" {
			continue
		}
		var anchor struct{ Kind, ID, Voice string }
		if err := json.Unmarshal(iv.Anchor, &anchor); err != nil {
			continue
		}
		// A keystone row has no anchor voice — read it as board.
		av := anchor.Voice
		if av == "" {
			av = string(agent.VoiceBoard)
		}
		if anchor.Kind != "draft_snapshot" || anchor.ID != sid.String() || av != string(voice) {
			continue
		}
		if it, ok := reviewItemFromIntervention(iv); ok {
			out = append(out, it)
		}
	}
	return out
}
```

- [ ] **Step 5: Add the per-voice row-count test helper**

Add near the other `countReviewItems` helper in `writing_test.go` (find it with `grep -n "func countReviewItems" apps/api/internal/api/*_test.go`). It counts `review_item` interventions whose anchor voice matches (missing → board):

```go
func countReviewItemsForVoice(t *testing.T, pool *pgxpool.Pool, projectID, voice string) int {
	t.Helper()
	rows, err := sqlc.New(pool).ListInterventionsByProject(context.Background(), mustUUID(projectID))
	if err != nil {
		t.Fatalf("ListInterventionsByProject: %v", err)
	}
	n := 0
	for _, iv := range rows {
		if iv.Type != "review_item" {
			continue
		}
		var a struct{ Voice string }
		_ = json.Unmarshal(iv.Anchor, &a)
		if a.Voice == "" {
			a.Voice = "board"
		}
		if a.Voice == voice {
			n++
		}
	}
	return n
}
```

(If `pgxpool` / `context` / `json` / `sqlc` imports are not already present in the test file, add them — check the existing import block first.)

- [ ] **Step 6: Run the api writing tests**

Run: `cd apps/api && CGO_ENABLED=0 go test ./internal/api/ -run TestOrderReview`
Expected: PASS — the new per-voice test plus the existing `TestOrderReview_PersistsWorkOrderAndIsIdempotent` (board path, `?voice=` absent → board) and `TestOrderReview_RejectedProposalPersistsNothing`.

- [ ] **Step 7: Commit**

```bash
cd apps/api && git add internal/api/writing.go internal/api/writing_test.go
git commit -m "feat(refactor2): voice-scoped orderReview anchor + replay + over-budget lens (Slice 8b T3)"
```

---

### Task 4: Budget + voice in the studio projection

**Files:**
- Modify: `apps/api/internal/studio/dto.go`, `apps/api/internal/studio/projection.go`
- Test: `apps/api/internal/studio/dto_parity_test.go` (or the projection test file — find the existing writing projection test with `grep -rn "projectWriting\|Writing" apps/api/internal/studio/*_test.go`)

**Interfaces:**
- Consumes: `agent.BudgetVerdict` (Task 2); the anchor now carries `voice` (Task 3).
- Produces: `WritingBudgetDTO{State string, Delta int}`; `WritingSnapshotDTO.Budget`; `WritingReviewItemDTO.Voice`; `WritingReviewDTO` without `Ordered`.

- [ ] **Step 1: Write the failing test**

Add to the studio test file that already exercises `projectWriting` (per the grep above). This test asserts the projection tags items with voice (missing → board) and fills the snapshot budget verdict:

```go
func TestProjectWriting_VoiceAndBudget(t *testing.T) {
	sk, _ := skills.ByID("writing-project")
	snapID := uuid.New()
	// One review_item with an explicit sceptic anchor, one keystone-style row
	// with no anchor voice (→ board).
	mk := func(voice string) sqlc.Intervention {
		anchor := map[string]string{"kind": "draft_snapshot", "id": snapID.String()}
		if voice != "" {
			anchor["voice"] = voice
		}
		b, _ := json.Marshal(anchor)
		body, _ := json.Marshal(agent.ReviewItem{CriterionCode: "表E", CriterionName: "分析", Band: "5–6 段", Evidence: "e"})
		return sqlc.Intervention{ID: uuid.New(), Type: "review_item", Anchor: b, Body: string(body)}
	}
	d := ProjectData{
		LatestSnapshot: &sqlc.DraftSnapshot{ID: snapID, Seq: 3, Content: strings.Repeat("字", 2340), CreatedAt: time.Now()},
		Interventions:  []sqlc.Intervention{mk("sceptic"), mk("")},
	}
	out := projectWriting(sk, d)
	if out.LatestSnapshot == nil || out.LatestSnapshot.Budget.State != "over" || out.LatestSnapshot.Budget.Delta != 340 {
		t.Fatalf("budget = %+v, want state=over delta=340", out.LatestSnapshot)
	}
	voices := map[string]int{}
	for _, it := range out.Review.Items {
		voices[it.Voice]++
	}
	if voices["sceptic"] != 1 || voices["board"] != 1 {
		t.Fatalf("voice tags = %v, want one sceptic + one board", voices)
	}
}
```

(Adapt the `ProjectData` literal field names to the real struct — confirm with `grep -n "type ProjectData" apps/api/internal/studio/projection.go`. `Interventions`, `LatestSnapshot`, `Dispositions`, `GateStates` are the fields used by `projectWriting`.)

- [ ] **Step 2: Run it to verify it fails**

Run: `cd apps/api && CGO_ENABLED=0 go test ./internal/studio/ -run TestProjectWriting_VoiceAndBudget`
Expected: FAIL — `out.LatestSnapshot.Budget` undefined, `it.Voice` undefined.

- [ ] **Step 3: Extend the DTOs**

In `apps/api/internal/studio/dto.go`:

Add the budget DTO and put it on the snapshot (replace the `WritingSnapshotDTO` struct, lines ~201-207):

```go
// WritingBudgetDTO is the deterministic count-vs-band verdict (agent.BudgetVerdict).
// state ∈ {in, over, under}; delta is a non-negative magnitude (words past max
// when over, short of min when under, 0 when in) — the client reads direction
// from state and renders delta directly.
type WritingBudgetDTO struct {
	State string `json:"state"`
	Delta int    `json:"delta"`
}

type WritingSnapshotDTO struct {
	ID          string           `json:"id"`
	Seq         int              `json:"seq"`
	CommittedAt string           `json:"committedAt"`
	WordCount   int              `json:"wordCount"`
	InBand      bool             `json:"inBand"`
	Budget      WritingBudgetDTO `json:"budget"`
}
```

Add `Voice` to `WritingReviewItemDTO` (after `Fix`, line ~226):

```go
	Voice          string          `json:"voice"`
```

Drop `Ordered` from `WritingReviewDTO` (replace lines ~233-236):

```go
// WritingReviewDTO carries every review work-order row anchored to the CURRENT
// latest snapshot, across all voices the student has run. Each item is
// self-describing via its Voice; the client derives the current-voice
// work-order and the cached-voice set by filtering.
type WritingReviewDTO struct {
	Items []WritingReviewItemDTO `json:"items"`
}
```

- [ ] **Step 4: Fill budget + voice in `projectWriting`**

In `apps/api/internal/studio/projection.go`:

Fill the snapshot budget (in the `if d.LatestSnapshot != nil` block, ~322-331). Replace the `out.LatestSnapshot = &WritingSnapshotDTO{...}` assignment with:

```go
		wc := agent.CountWords(d.LatestSnapshot.Content)
		inBand := sk.WordBudget != nil && wc >= sk.WordBudget.Min && wc <= sk.WordBudget.Max
		bState, bDelta := agent.BudgetVerdict(wc, sk.WordBudget)
		out.LatestSnapshot = &WritingSnapshotDTO{
			ID:          d.LatestSnapshot.ID.String(),
			Seq:         int(d.LatestSnapshot.Seq),
			CommittedAt: d.LatestSnapshot.CreatedAt.Format(time.RFC3339),
			WordCount:   wc, InBand: inBand,
			Budget: WritingBudgetDTO{State: bState, Delta: bDelta},
		}
```

Change the review-item loop (~342-363): decode the anchor voice, tag the item, drop `Ordered`. Replace the anchor decode + append block:

```go
		var a struct{ Kind, ID, Voice string }
		_ = json.Unmarshal(iv.Anchor, &a)
		if a.Kind != "draft_snapshot" || a.ID != d.LatestSnapshot.ID.String() {
			continue
		}
		item, ok := reviewItemDTOFromIntervention(iv)
		if !ok {
			continue
		}
		item.Voice = a.Voice
		if item.Voice == "" {
			item.Voice = "board"
		}
		if dp, ok := dispByIv[iv.ID.String()]; ok {
			item.Disposition = &DispositionDTO{Action: dp.Action, Reason: dp.Reason}
		}
		out.Review.Items = append(out.Review.Items, item)
```

And change the initializer on line ~318 (drop the `Ordered` reference — the zero value of the new struct is `{Items: nil}`; keep the non-nil slice for stable JSON):

```go
	out := WritingDTO{Buffer: d.EditBuffer, Review: WritingReviewDTO{Items: []WritingReviewItemDTO{}}}
```

(`reviewItemDTOFromIntervention` does not set `Voice`; the loop sets it from the anchor — leave that helper as is.)

- [ ] **Step 5: Run studio tests**

Run: `cd apps/api && CGO_ENABLED=0 go test ./internal/studio/`
Expected: PASS — the new test plus existing projection/parity tests. If a pre-existing test referenced `Review.Ordered`, update it to derive from `len(out.Review.Items) > 0` (it is no longer a field).

- [ ] **Step 6: Commit**

```bash
cd apps/api && git add internal/studio/dto.go internal/studio/projection.go internal/studio/
git commit -m "feat(refactor2): project snapshot budget verdict + per-item voice, drop Review.Ordered (Slice 8b T4)"
```

---

### Task 5: Contracts — `WritingBudget` + voiced review item

**Files:**
- Modify: `packages/contracts/src/studioState.ts`
- Test: the contracts test that validates `WritingProjection` (find with `grep -rln "WritingProjection\|WritingReviewItem" packages/contracts/src/*.test.ts`); if none targets it, add assertions to the nearest studioState test.

**Interfaces:**
- Produces: `WritingBudget` schema; `WritingReviewItem.voice`; `WritingProjection.review = { items: WritingReviewItem[] }` (no `ordered`).

- [ ] **Step 1: Write the failing test**

Add to the contracts studioState test file:

```ts
import { WritingProjection, WritingReviewItem } from "./studioState";

test("WritingReviewItem carries a voice enum", () => {
  const ok = WritingReviewItem.safeParse({
    interventionId: "i1", criterion: "表E 分析", band: "5–6 段",
    evidence: "e", missing: "m", fix: "", voice: "sceptic", disposition: null,
  });
  expect(ok.success).toBe(true);
  const bad = WritingReviewItem.safeParse({
    interventionId: "i1", criterion: "表E", band: "5–6 段",
    evidence: "e", missing: "m", fix: "", voice: "nope", disposition: null,
  });
  expect(bad.success).toBe(false);
});

test("WritingProjection review is a flat items array with a snapshot budget", () => {
  const parsed = WritingProjection.safeParse({
    buffer: "",
    latestSnapshot: { id: "s1", seq: 3, committedAt: "2026-07-15T00:00:00Z", wordCount: 2340, inBand: false, budget: { state: "over", delta: 340 } },
    wordBudget: { min: 1500, max: 2000 },
    citationsMatched: false,
    review: { items: [] },
  });
  expect(parsed.success).toBe(true);
});
```

- [ ] **Step 2: Run it to verify it fails**

Run: `cd packages/contracts && npm test -- studioState`
Expected: FAIL — `voice` unknown key rejected by the current strict schema / `budget` missing on snapshot / `review.items` shape mismatch.

- [ ] **Step 3: Implement the schema changes**

In `packages/contracts/src/studioState.ts`:

Add budget to `WritingSnapshot` (replace lines ~152-158):

```ts
export const WritingBudget = z.object({
  state: z.enum(["in", "over", "under"]),
  delta: z.number().int(),
});
export type WritingBudget = z.infer<typeof WritingBudget>;

export const WritingSnapshot = z.object({
  id: z.string(),
  seq: z.number().int(),
  committedAt: z.string(),
  wordCount: z.number().int(),
  inBand: z.boolean(),
  budget: WritingBudget,
});
export type WritingSnapshot = z.infer<typeof WritingSnapshot>;
```

Add `voice` to `WritingReviewItem` (in the object, after `fix`):

```ts
  voice: z.enum(["board", "sceptic", "layperson", "executioner"]),
```

Change the `review` field of `WritingProjection` (replace the `review:` line):

```ts
  review: z.object({ items: z.array(WritingReviewItem) }),
```

- [ ] **Step 4: Run it to verify it passes**

Run: `cd packages/contracts && npm test`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add packages/contracts/src/studioState.ts packages/contracts/src/*.test.ts
git commit -m "feat(refactor2): WritingBudget + review-item voice, flat review items[] (Slice 8b T5)"
```

---

### Task 6: Web API client — `orderReview(voice)`

**Files:**
- Modify: `apps/web/src/api/writing.ts`
- Test: `apps/web/src/api/writing.test.ts` (find/confirm with `grep -rln "orderReview" apps/web/src/api/*.test.ts`); if absent, add a focused test file `apps/web/src/api/writing.test.ts`.

**Interfaces:**
- Produces: `orderReview(projectId, snapshotId, voice)` — voice appended as `?voice=` (omitted when `board`, keeping the URL identical to today's for the default).

- [ ] **Step 1: Write the failing test**

Assert the request URL carries the voice. Use the project's existing fetch-mock pattern (check a sibling test, e.g. `apps/web/src/api/materials.test.ts`, for how `fetch`/`apiFetch` is mocked). Minimal shape:

```ts
import { describe, it, expect, vi, afterEach } from "vitest";
import { orderReview } from "./writing";

afterEach(() => vi.restoreAllMocks());

describe("orderReview voice", () => {
  it("appends ?voice= for a non-board voice", async () => {
    const seen: string[] = [];
    vi.stubGlobal("fetch", vi.fn(async (url: string) => {
      seen.push(url);
      return { ok: true, body: new ReadableStream({ start(c) { c.close(); } }) } as unknown as Response;
    }));
    for await (const _ of orderReview("p1", "s1", "sceptic")) { /* drain */ }
    expect(seen[0]).toContain("/snapshots/s1/review?voice=sceptic");
  });

  it("omits the query for board (URL identical to the keystone call)", async () => {
    const seen: string[] = [];
    vi.stubGlobal("fetch", vi.fn(async (url: string) => {
      seen.push(url);
      return { ok: true, body: new ReadableStream({ start(c) { c.close(); } }) } as unknown as Response;
    }));
    for await (const _ of orderReview("p1", "s1", "board")) { /* drain */ }
    expect(seen[0]).toContain("/snapshots/s1/review");
    expect(seen[0]).not.toContain("voice=");
  });
});
```

- [ ] **Step 2: Run it to verify it fails**

Run: `cd apps/web && npm test -- writing`
Expected: FAIL — `orderReview` currently takes only `(projectId, snapshotId)`; the voice arg is ignored / TS error.

- [ ] **Step 3: Implement**

In `apps/web/src/api/writing.ts`, change the `orderReview` signature and URL:

```ts
export type ReviewVoice = "board" | "sceptic" | "layperson" | "executioner";

export async function* orderReview(projectId: string, snapshotId: string, voice: ReviewVoice = "board"): AsyncGenerator<StudioTurnEvent> {
  const q = voice === "board" ? "" : `?voice=${voice}`;
  const res = await fetch(`${API_BASE}/api/v1/projects/${projectId}/snapshots/${snapshotId}/review${q}`, {
    method: "POST",
    credentials: "include",
    headers: { Accept: "text/event-stream" },
  });
  // ...rest unchanged...
```

Keep the remainder of the function body identical.

- [ ] **Step 4: Run it to verify it passes**

Run: `cd apps/web && npm test -- writing`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add apps/web/src/api/writing.ts apps/web/src/api/writing.test.ts
git commit -m "feat(refactor2): orderReview accepts a voice, ?voice= query (Slice 8b T6)"
```

---

### Task 7: WritingView — voice pills, budget meta, budget note, per-voice items

**Files:**
- Modify: `apps/web/src/studio/views/WritingView.tsx`
- Test: `apps/web/src/studio/views/WritingView.test.tsx` (find with `grep -rln "WritingView" apps/web/src/studio/**/*.test.tsx`; if none, add one)

**Interfaces:**
- Consumes: `WritingProjection` (with `review.items[].voice`, `latestSnapshot.budget`).
- Produces: `onOrderReview?(snapshotId: string, voice: ReviewVoice)` — the callback gains a `voice` arg (Task 8 supplies it). Voice pills reuse the tab styling; `snapshotMeta` gains the budget clause; a budget note renders in the work-order when `latestSnapshot.budget.state === "over"`.

- [ ] **Step 1: Write the failing tests**

In `apps/web/src/studio/views/WritingView.test.tsx` (use `@testing-library/react` — match how the repo's other view tests render; check a sibling `*.test.tsx`):

```tsx
import { render, screen, fireEvent } from "@testing-library/react";
import { WritingView } from "./WritingView";

const snap = { id: "s1", seq: 3, committedAt: "2026-07-15T00:00:00Z", wordCount: 2340, inBand: false, budget: { state: "over" as const, delta: 340 } };
const base = {
  buffer: "第一段。\n\n第二段。", latestSnapshot: snap,
  wordBudget: { min: 1500, max: 2000 }, citationsMatched: false,
  review: { items: [] as any[] },
};

test("snapshotMeta shows the over-budget clause", () => {
  render(<WritingView {...(base as any)} />);
  expect(screen.getByText(/超出 340 字/)).toBeInTheDocument();
});

test("picking a voice then 整稿体检 passes that voice", () => {
  const onOrderReview = vi.fn();
  render(<WritingView {...(base as any)} onOrderReview={onOrderReview} />);
  fireEvent.click(screen.getByText("怀疑"));
  fireEvent.click(screen.getByText("整稿体检"));
  expect(onOrderReview).toHaveBeenCalledWith("s1", "sceptic");
});

test("the work-order shows only the selected voice's items", () => {
  const items = [
    { interventionId: "b1", criterion: "表E 分析", band: "5–6 段", evidence: "board-ev", missing: "", fix: "", voice: "board", disposition: null },
    { interventionId: "s1", criterion: "表E 分析", band: "5–6 段", evidence: "sceptic-ev", missing: "", fix: "", voice: "sceptic", disposition: null },
  ];
  render(<WritingView {...(base as any)} review={{ items }} />);
  // default voice = board → board item visible in preview
  fireEvent.click(screen.getByText("预览 · 批注"));
  expect(screen.getByText("board-ev")).toBeInTheDocument();
  expect(screen.queryByText("sceptic-ev")).not.toBeInTheDocument();
});

test("over-budget preview shows the deletion note", () => {
  const items = [{ interventionId: "b1", criterion: "表E 分析", band: "5–6 段", evidence: "e", missing: "", fix: "", voice: "board", disposition: null }];
  render(<WritingView {...(base as any)} review={{ items }} />);
  fireEvent.click(screen.getByText("预览 · 批注"));
  expect(screen.getByText(/删减决策按「这段在向哪张表交证据」来做/)).toBeInTheDocument();
});
```

- [ ] **Step 2: Run to verify they fail**

Run: `cd apps/web && npm test -- WritingView`
Expected: FAIL — no voice pills, `snapshotMeta` has no budget clause, `onOrderReview` called with one arg.

- [ ] **Step 3: Add the voice model + pills**

In `apps/web/src/studio/views/WritingView.tsx`:

Add the voice list near the top (after `TAB_BASE`/`tabStyle`):

```tsx
export type ReviewVoice = "board" | "sceptic" | "layperson" | "executioner";
const VOICES: Array<{ voice: ReviewVoice; label: string }> = [
  { voice: "board", label: "考官" },
  { voice: "sceptic", label: "怀疑" },
  { voice: "layperson", label: "外行" },
  { voice: "executioner", label: "字数" },
];
```

Update the props type: change `onOrderReview` to `(snapshotId: string, voice: ReviewVoice) => void`.

In the `WritingView` component body, add state and derived data (after `const [mode, setMode] = ...`):

```tsx
  const [voice, setVoice] = useState<ReviewVoice>("board");
  const items = review?.items ?? [];
  const cachedVoices = new Set(items.map((i) => i.voice));
  const itemsForVoice = items.filter((i) => i.voice === voice);
  const reviewOrdered = itemsForVoice.length > 0;
  const budget = latestSnapshot?.budget;
```

Change `snapshotMeta` to append the budget clause:

```tsx
  const budgetClause = budget
    ? budget.state === "over"
      ? ` · 超出 ${budget.delta} 字`
      : budget.state === "under"
        ? ` · 还差 ${budget.delta} 字`
        : " · 在预算内"
    : "";
  const snapshotMeta = latestSnapshot
    ? `第 ${latestSnapshot.seq} 版快照 · ${monthDayOf(latestSnapshot.committedAt)} 提交 · 只读${budgetClause}`
    : "还没有提交过快照";
```

Insert the voice pills into the header row, immediately before the `整稿体检` button's wrapping `<div style={{ marginLeft: "auto" }}>`. Reuse the segmented container styling from the 编辑/预览 tabs:

```tsx
        <div style={{ marginLeft: "auto", display: "flex", alignItems: "center", gap: 10 }}>
          <div style={{ display: "flex", gap: 3, background: "#EBEDF2", borderRadius: 9, padding: 3 }}>
            {VOICES.map((v) => {
              const active = voice === v.voice;
              const cached = cachedVoices.has(v.voice);
              return (
                <button key={v.voice} type="button" onClick={() => setVoice(v.voice)} style={tabStyle(active)}>
                  {v.label}{cached ? " ·" : ""}
                </button>
              );
            })}
          </div>
          <button
            type="button"
            disabled={!latestSnapshot}
            onClick={() => latestSnapshot && onOrderReview?.(latestSnapshot.id, voice)}
            style={{
              display: "inline-flex",
              alignItems: "center",
              gap: 7,
              background: latestSnapshot ? "#2A3B7A" : "#B9C0D6",
              border: "none",
              color: "#fff",
              fontSize: 13,
              fontWeight: 700,
              padding: "9px 15px",
              borderRadius: 10,
              cursor: latestSnapshot ? "pointer" : "not-allowed",
              fontFamily: "inherit",
            }}
          >
            <svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="#fff" strokeWidth={2} strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
              <path d="M9 11l3 3L22 4M21 12v7a2 2 0 01-2 2H5a2 2 0 01-2-2V5a2 2 0 012-2h11" />
            </svg>
            整稿体检
          </button>
        </div>
```

(This replaces the old `<div style={{ marginLeft: "auto" }}>` wrapper that held only the review button — the button now lives inside the new flex container alongside the voice pills. The button's style object and inner SVG are the current file's, verbatim; only its `onClick` gains the `voice` arg.)

Pass the selected voice's ordered/items into `PreviewPane`, and hand it the over-budget flag:

```tsx
            <PreviewPane
              buffer={buffer}
              reviewOrdered={reviewOrdered}
              reviewItems={itemsForVoice}
              overBudgetDelta={budget?.state === "over" ? budget.delta : null}
              citationsMatched={citationsMatched}
              onReviewDisposition={onReviewDisposition}
              onAttestCitations={onAttestCitations}
            />
```

- [ ] **Step 4: Render the budget note in `PreviewPane`**

Add `overBudgetDelta: number | null` to `PreviewPane`'s props, and render the note as the first child inside the work-order card (right after the header `<div>` that holds the `整稿体检 · 段落 ⇄ 评分表` chip), only when non-null:

```tsx
          {overBudgetDelta !== null && (
            <div style={{ fontSize: 12, fontWeight: 700, color: "#C96F4F", background: "#FBEEE7", borderRadius: 10, padding: "8px 12px", margin: "0 0 14px" }}>
              超预算 {overBudgetDelta} 字 · 删减决策按「这段在向哪张表交证据」来做
            </div>
          )}
```

- [ ] **Step 5: Run to verify they pass**

Run: `cd apps/web && npm test -- WritingView`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add apps/web/src/studio/views/WritingView.tsx apps/web/src/studio/views/WritingView.test.tsx
git commit -m "feat(refactor2): voice pills + budget meta/note + per-voice work-order in WritingView (Slice 8b T7)"
```

---

### Task 8: StudioContainer — voice wiring + fixture backfill

**Files:**
- Modify: `apps/web/src/studio/StudioContainer.tsx`, `apps/web/src/studio/fixtures.ts`
- Test: `apps/web/src/studio/StudioContainer.test.tsx`

**Interfaces:**
- Consumes: `orderReview(projectId, snapshotId, voice)` (Task 6); `WritingView`'s `onOrderReview(snapshotId, voice)` (Task 7).
- Produces: `onOrderReview` passes the voice straight through to `api.orderReview`; fixtures carry `budget` on the snapshot and `voice` on review items, and drop `ordered`.

- [ ] **Step 1: Write the failing test**

Add to `apps/web/src/studio/StudioContainer.test.tsx` (mirror the existing Task-10 orderReview test — find it with `grep -n "orderReview" apps/web/src/studio/StudioContainer.test.tsx`):

```tsx
it("passes the chosen voice through to api.orderReview", async () => {
  const orderReview = vi.fn(async function* () { /* empty stream */ });
  // ...render StudioContainer with the api double { ...baseApi, orderReview, getProject }
  // switch to the 写作 view, click a voice pill (怀疑), click 整稿体检...
  // then:
  expect(orderReview).toHaveBeenCalledWith(expect.any(String), expect.any(String), "sceptic");
});
```

(Follow the exact render/harness the existing writing tests in this file use — `grep -n "writingProjection\|onOrderReview\|api.orderReview" apps/web/src/studio/StudioContainer.test.tsx` shows the established pattern; reuse its `writingProjection(...)` helper, adding `budget`/`voice` per Step 3.)

- [ ] **Step 2: Run to verify it fails**

Run: `cd apps/web && npm test -- StudioContainer`
Expected: FAIL — `onOrderReview` currently ignores voice / the `writingProjection` helper lacks the new required fields (TS/parse error).

- [ ] **Step 3: Wire the voice + backfill fixtures**

In `apps/web/src/studio/StudioContainer.tsx`, change the `onOrderReview` handler signature and forward the voice:

```tsx
    onOrderReview: (snapshotId, voice) => {
      if (!projectId) return;
      (async () => {
        for await (const ev of api.orderReview(projectId, snapshotId, voice)) {
          if (ev.type === "error") setSyncError(ev.message || "体检失败，请重试。");
        }
        await refetchProject();
      })().catch(() => {
        setSyncError("画面可能未同步到最新状态，请刷新页面重试。");
      });
    },
```

Confirm the `api` type union / interface for `orderReview` matches `(projectId, snapshotId, voice) => AsyncGenerator<...>` (line ~14 lists `"orderReview"`; the actual signature is imported from `api/writing.ts`, so Task 6 already updated it — just make sure the call compiles).

In `apps/web/src/studio/fixtures.ts` (the `writing:` block, ~line 185) and in the `writingProjection(...)` helper in `StudioContainer.test.tsx`, add `budget` to the snapshot and `voice` to every review item, and remove any `ordered` key:

```ts
    writing: {
      buffer: "...",
      latestSnapshot: { id: "...", seq: /* n */, committedAt: "...", wordCount: /* n */, inBand: /* b */, budget: { state: "in", delta: 0 } },
      wordBudget: { min: 1500, max: 2000 },
      citationsMatched: /* b */,
      review: { items: [ /* each item + voice: "board" */ ] },
    },
```

(Search every fixture/test literal that builds a `writing` projection or a review item and add the two new fields — `grep -rn "committedAt\|review: {" apps/web/src/studio` finds them. The Zod parse in `getProject`/`toStudioState` will fail loudly on any you miss, so run the full web suite in Step 4.)

- [ ] **Step 4: Run to verify it passes**

Run: `cd apps/web && npm test -- StudioContainer`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add apps/web/src/studio/StudioContainer.tsx apps/web/src/studio/StudioContainer.test.tsx apps/web/src/studio/fixtures.ts
git commit -m "feat(refactor2): StudioContainer forwards review voice + fixture backfill (Slice 8b T8)"
```

---

### Task 9: Full gate + roadmap/memory

**Files:**
- Modify: `docs/2026-07-11-whole-product-refactor-roadmap.md`, memory files.

- [ ] **Step 1: Sync generated code and run every gate**

```bash
cd apps/api && make sqlc && make sync-skills
```
Expected: no diff (this slice adds no SQL query and no skill-JSON change; if either produces a diff, something drifted — investigate before proceeding).

```bash
cd apps/api && CGO_ENABLED=0 go test -p 1 ./...
```
Expected: all packages `ok`.

```bash
cd apps/web && npm test && npx tsc --noEmit
cd packages/contracts && npm test
```
Expected: web + contracts green, tsc clean.

- [ ] **Step 2: Update the roadmap**

In `docs/2026-07-11-whole-product-refactor-roadmap.md`: mark the Slice 8 row's `→ 8b` note as done and append a `### Slice 8b` per-slice log entry summarizing: 4 voices (board default + sceptic/layperson/executioner) as a fixed Go enum selecting the review posture; per-`(snapshot,voice)` idempotency via the anchor `voice` field (no migration); `agent.BudgetVerdict` + projection `budget:{state,delta}`; over-budget deletion lens folded into 整稿体检; flat voiced `review.items[]` (dropped `ordered`); EE/AP board-specific passes still deferred to their board packs.

- [ ] **Step 3: Update memory**

Update `/Users/houyuxin/.claude/projects/-Users-houyuxin-08Coding-mind-imprint/memory/whole-product-refactor-2026-07.md` (frontmatter one-liner + a `## Slice 8b` section) and the `MEMORY.md` index pointer (NEXT = Slice 9). Note whether the whole-branch review was clean.

- [ ] **Step 4: Commit**

```bash
git add docs/2026-07-11-whole-product-refactor-roadmap.md
git commit -m "docs(refactor2): Slice 8b complete — examiner voices + budget-deletion coaching (S5)"
```

(Memory files live outside the repo — write them with the Write/Edit tools, not `git add`.)

---

## Notes for the executor

- **Whole-branch review (Opus) is MANDATORY at the end.** It has caught cross-layer defects per-task reviews missed on this project. Never skip. Run `scripts/review-package $(git merge-base main HEAD) HEAD` and hand the reviewer that file plus the Global Constraints above.
- **Card/gate/projection changes MUST run FULL Go packages**, not `-run` subsets, for the final gate (`-p 1 ./...`).
- The pre-existing `M package.json` and untracked user files in the working tree are NOT part of this slice — never `git add` them.
