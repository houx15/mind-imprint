# Slice 8 (keystone) — Writing surface + whole-draft review (S5) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make the S5 写作 station live — a silent client-owned edit buffer, immutable draft snapshots, a student-triggered whole-draft review (整稿体检) that produces a typed work-order through the enforcement stack, three-key disposition, and the word-budget machine gate — with no AI write path to prose (RL-1).

**Architecture:** Evolve in place. `draft_snapshot`/`edit_buffer` tables already exist (Slice 0), so no migration. New `writing.sql` queries + endpoints persist the buffer and mint immutable snapshots (an in-band commit mints a `word_budget_ok` graph node — the S5 machine gate). A new `agent/review.go` `ProposeReview` makes one flagship model call and runs the full enforcement stack, returning a work-order persisted as `intervention` rows anchored to the snapshot. A new `writing` field on `StudioProjection` (Go DTO ↔ Zod contract, parity-guarded) carries buffer + latest snapshot + word budget + citations flag + the review work-order ⋈ dispositions. The frontend `WritingView` goes live over that projection.

**Tech Stack:** Go (`net/http`, `pgx`, `sqlc`, `goose`, testcontainers), TypeScript + React + vitest, Zod contracts. SSE via `gateway.SSEWriter`.

## Global Constraints

- **RL-1 — the AI has no write path to prose.** No endpoint, verb, or runtime action writes `edit_buffer` or a prose graph node with model output. The buffer write endpoint takes student text only. The review writes **only `intervention` rows** (typed advice), never a prose/claim node. The `fix` field is advice, never a rewritten sentence.
- **Client never calls a model directly.** All LLM calls go through the backend gateway; every real call records 档位 + token + 成本 via `RecordLLMCall` (`LLMCallRow`). Keys live only in `apps/api`.
- **Word-budget band = skill config, seeded 0457 = `{min:1500, max:2000}`** (`writing-project` skill `word_budget`). Single source of truth; the commit mint and the projection both read it.
- **Review criteria = skill config `review_criteria`** (0457 tables, each `{code, name}`); the review model assesses against exactly this set. Not the Slice-9 band engine.
- **`word_budget_ok` node:** `type:"word_budget_ok"`, `author:"ai"` (system-node convention, same as gate_state/plan), body `{word_count,min,max}`. Minted on an in-band commit; removed when out of band. Not prose — authorship guard N/A.
- **One snapshot, one review.** If `review_item` interventions exist for a snapshot, the review endpoint returns them idempotently — no second model call.
- **Disposition mapping (no migration):** the review's three keys map onto the existing enum — 我来改→`rewrite`, 保持原样→`accept`, 说明为什么不改→`reject`. Reason ≥15 runes via the existing `RecordDisposition`.
- **Ownership 404-not-403** (`loadOwnedProject`); entitlement gate **before** any model-call stream (`HasEntitlement`).
- **Binding design copy is verbatim** (`docs/design/思维印记_工作区.dc.html`, 写作 view). Icons inline SVG, never lucide-react.
- **Go tests:** `CGO_ENABLED=0 go test -p 1 ./...` on a quiet Docker daemon (testcontainers), FULL packages, never `-run` subsets for the final gate. **sqlc:** `make sqlc` (CGO_ENABLED=0). **skills:** `make sync-skills`. **Web:** `npm test` in `apps/web`. **Contracts:** `npm test` in `packages/contracts`.
- **Never `git add` a whole dir** with untracked user files; add named paths only. Pre-existing uncommitted `M package.json` + untracked user files are NOT ours — leave them.

---

## File Structure

**Backend (`apps/api`)**
- `internal/store/queries/writing.sql` (create) — buffer upsert/get, snapshot insert/get-latest/get.
- `internal/store/queries/disposition.sql` (modify) — add `ListDispositionsByProject`.
- `internal/skills/skill.go` (modify) — `WordBudget` + `ReviewCriteria` fields on `Skill`.
- `internal/agent/wordcount.go` (create) — `CountWords`.
- `internal/agent/review.go` (create) — `ProposeReview`, `ReviewItem`, the `order_review` output type.
- `internal/api/writing.go` (create) — `putEditBuffer`, `commitSnapshot`, `orderReview`, `attestGate` handlers + `snapshotEmitter`.
- `internal/api/api.go` (modify) — 4 new routes.
- `internal/studio/dto.go` (modify) — `WritingDTO` + `Writing` field on `StudioProjection`.
- `internal/studio/projection.go` (modify) — `projectWriting` + wire into `Project`; `ProjectData.EditBuffer`/`LatestSnapshot`/`Dispositions`.
- `internal/studio/load.go` (modify) — load buffer, latest snapshot, dispositions.

**Contracts / skill config**
- `packages/contracts/skills/writing-project.json` (modify, canonical) + `apps/api/internal/skills/specs/writing-project.json` (synced mirror) — `word_budget`, `review_criteria`.
- `packages/contracts/src/studioState.ts` (modify) — `WritingSnapshot`/`WritingReviewItem`/`WritingProjection` + required `writing`.

**Frontend (`apps/web`)**
- `src/api/writing.ts` (create) — `putBuffer`, `commitSnapshot`, `orderReview` (SSE), `attestGate`.
- `src/api/studioTurn.ts` (modify) — `review` frame mapping (shared SSE plumbing).
- `src/studio/views/WritingView.tsx` (modify) — live edit/commit/preview + work-order + disposition + citations attest.
- `src/studio/state.ts` (modify) — adopt `WritingProjection` contract type.
- `src/studio/StudioContainer.tsx` (modify) — un-stub `writing: p.writing`; wire buffer/commit/review/disposition/attest callbacks.
- `src/studio/ViewFrame.tsx` (modify) — pass the new props through.

---

## Task 1: Skill config — `word_budget` + `review_criteria`

**Files:**
- Modify: `packages/contracts/skills/writing-project.json` (canonical)
- Modify: `apps/api/internal/skills/specs/writing-project.json` (synced mirror — via `make sync-skills`)
- Modify: `apps/api/internal/skills/skill.go`
- Test: `apps/api/internal/skills/skill_test.go`

**Interfaces:**
- Produces: `skills.Skill.WordBudget *WordBudget` (`{Min, Max int}`), `skills.Skill.ReviewCriteria []ReviewCriterion` (`{Code, Name string}`). Consumed by Tasks 3, 6, 8.

- [ ] **Step 1: Write the failing test** — append to `apps/api/internal/skills/skill_test.go`:

```go
func TestWritingProjectWordBudgetAndReviewCriteria(t *testing.T) {
	sk, ok := skills.ByID("writing-project")
	if !ok {
		t.Fatal("writing-project skill not loaded")
	}
	if sk.WordBudget == nil {
		t.Fatal("WordBudget is nil")
	}
	if sk.WordBudget.Min != 1500 || sk.WordBudget.Max != 2000 {
		t.Fatalf("WordBudget = %+v, want {1500 2000}", *sk.WordBudget)
	}
	codes := map[string]bool{}
	for _, c := range sk.ReviewCriteria {
		if c.Code == "" || c.Name == "" {
			t.Fatalf("review criterion has empty field: %+v", c)
		}
		codes[c.Code] = true
	}
	for _, want := range []string{"表D", "表E", "表F", "表H"} {
		if !codes[want] {
			t.Errorf("review_criteria missing %q", want)
		}
	}
}
```

- [ ] **Step 2: Run — verify it fails**

Run: `cd apps/api && CGO_ENABLED=0 go test ./internal/skills/`
Expected: FAIL — `sk.WordBudget undefined` (compile error).

- [ ] **Step 3: Add the Go fields** — in `apps/api/internal/skills/skill.go`, add the two types and two fields on `Skill` (after `Cards []string`):

```go
// WordBudget is the per-qualification legal word band for S5 (draft_polish).
// An in-band commit mints the word_budget_ok node that satisfies the S5
// machine gate. Single source of truth; the commit path and the projection
// both read it.
type WordBudget struct {
	Min int `json:"min"`
	Max int `json:"max"`
}

// ReviewCriterion is one mark-scheme table the whole-draft review assesses the
// draft against (0457's 表D/E/F/H). Minimal labels — NOT the Slice-9 readiness
// band engine.
type ReviewCriterion struct {
	Code string `json:"code"`
	Name string `json:"name"`
}
```

Add to the `Skill` struct:

```go
	WordBudget     *WordBudget       `json:"word_budget,omitempty"`
	ReviewCriteria []ReviewCriterion `json:"review_criteria,omitempty"`
```

- [ ] **Step 4: Add the config to the canonical skill JSON** — in `packages/contracts/skills/writing-project.json`, add two top-level keys (siblings of `contracts`, `cards`):

```json
  "word_budget": { "min": 1500, "max": 2000 },
  "review_criteria": [
    { "code": "表D", "name": "来源与证据" },
    { "code": "表E", "name": "分析" },
    { "code": "表F", "name": "评估" },
    { "code": "表H", "name": "表达与组织" }
  ],
```

- [ ] **Step 5: Sync the mirror**

Run: `cd apps/api && make -C .. sync-skills` (or the repo's `make sync-skills`)
Expected: `apps/api/internal/skills/specs/writing-project.json` updated; `git status --short` shows both JSONs changed and identical in the two `word_budget`/`review_criteria` keys.

- [ ] **Step 6: Run the test — verify it passes**

Run: `cd apps/api && CGO_ENABLED=0 go test ./internal/skills/`
Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add packages/contracts/skills/writing-project.json apps/api/internal/skills/specs/writing-project.json apps/api/internal/skills/skill.go apps/api/internal/skills/skill_test.go
git commit -m "feat(refactor2): S5 skill config — word_budget + review_criteria"
```

---

## Task 2: `writing.sql` queries + `CountWords`

**Files:**
- Create: `apps/api/internal/store/queries/writing.sql`
- Create: `apps/api/internal/agent/wordcount.go`
- Test: `apps/api/internal/agent/wordcount_test.go`
- Test: `apps/api/internal/store/sqlc_test.go` (append a round-trip)

**Interfaces:**
- Produces (sqlc-generated): `UpsertEditBuffer(project_id, content)`, `GetEditBuffer(project_id) → content`, `InsertDraftSnapshot(project_id, seq, content, span_index) → DraftSnapshot`, `GetLatestSnapshot(project_id) → DraftSnapshot`, `GetSnapshot(id, project_id) → DraftSnapshot`.
- Produces: `agent.CountWords(s string) int`. Consumed by Task 3.

- [ ] **Step 1: Write the query file** — `apps/api/internal/store/queries/writing.sql`:

```sql
-- Slice 8 (S5): the silent edit buffer (one row per project) + immutable
-- draft snapshots. The buffer is student-owned scratch; a snapshot is an
-- immutable commit. The AI has no write path to either (RL-1) — only the
-- owning student's PUT /buffer and POST /snapshots reach these.

-- name: UpsertEditBuffer :exec
INSERT INTO edit_buffer (project_id, content)
VALUES ($1, $2)
ON CONFLICT (project_id)
DO UPDATE SET content = EXCLUDED.content, updated_at = now();

-- name: GetEditBuffer :one
SELECT content FROM edit_buffer WHERE project_id = $1;

-- name: InsertDraftSnapshot :one
INSERT INTO draft_snapshot (project_id, seq, content, span_index)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: GetLatestSnapshot :one
SELECT * FROM draft_snapshot
WHERE project_id = $1
ORDER BY seq DESC
LIMIT 1;

-- name: GetSnapshot :one
SELECT * FROM draft_snapshot WHERE id = $1 AND project_id = $2;

-- name: NextSnapshotSeq :one
SELECT COALESCE(MAX(seq), 0) + 1 AS next FROM draft_snapshot WHERE project_id = $1;
```

- [ ] **Step 2: Generate sqlc**

Run: `cd apps/api && make -C .. sqlc` (or repo `make sqlc`; CGO_ENABLED=0)
Expected: `internal/store/sqlc/writing.sql.go` generated; `git status --short internal/store/sqlc` shows the new file; no unrelated diff.

- [ ] **Step 3: Write the `CountWords` failing test** — `apps/api/internal/agent/wordcount_test.go`:

```go
package agent

import "testing"

func TestCountWords(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want int
	}{
		{"empty", "", 0},
		{"whitespace only", "   \n\t ", 0},
		{"latin words", "the quick brown fox", 4},
		{"cjk chars each count", "中国是否让地球更可持续", 11},
		{"mixed cjk and latin", "中国的 GDP 增长", 5}, // 中 国 的 + GDP + 增 长 = 6? see rule
		{"latin with punctuation", "hello, world!", 2},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := CountWords(c.in); got != c.want {
				t.Errorf("CountWords(%q) = %d, want %d", c.in, got, c.want)
			}
		})
	}
}
```

Note: the mixed case decides the rule. **Rule:** each CJK ideograph counts as one word; maximal runs of non-CJK, non-space characters count as one word each. For "中国的 GDP 增长": 中,国,的 (3) + GDP (1) + 增,长 (2) = 6. Fix the `want` to `6` in the test above before running.

- [ ] **Step 4: Run — verify it fails**

Run: `cd apps/api && CGO_ENABLED=0 go test ./internal/agent/ -run TestCountWords`
Expected: FAIL — `CountWords` undefined.

- [ ] **Step 5: Implement `CountWords`** — `apps/api/internal/agent/wordcount.go`:

```go
package agent

import "unicode"

// CountWords counts words for the S5 word-budget gate, CJK-aware: each CJK
// ideograph counts as one word, and each maximal run of non-CJK, non-space
// characters (a latin word, a number, an acronym) counts as one word.
// Punctuation attached to a run does not add a word; a standalone punctuation
// run does (rare, acceptable for a budget estimate). The rule is documented
// here because it is the single source of truth for every caller.
func CountWords(s string) int {
	count := 0
	inRun := false
	for _, r := range s {
		switch {
		case isCJK(r):
			count++
			inRun = false
		case unicode.IsSpace(r):
			inRun = false
		default:
			if !inRun {
				count++
				inRun = true
			}
		}
	}
	return count
}

func isCJK(r rune) bool {
	return unicode.Is(unicode.Han, r) ||
		(r >= 0x3040 && r <= 0x30FF) || // hiragana + katakana
		(r >= 0xFF00 && r <= 0xFFEF)    // full-width forms
}
```

- [ ] **Step 6: Run `CountWords` test — verify it passes**

Run: `cd apps/api && CGO_ENABLED=0 go test ./internal/agent/ -run TestCountWords`
Expected: PASS (all 6 cases).

- [ ] **Step 7: Add a snapshot round-trip test** — append to `apps/api/internal/store/sqlc_test.go` (follow the file's existing testcontainers harness + seed helpers; use the pattern already present for project-scoped inserts):

```go
func TestDraftSnapshotAndBufferRoundTrip(t *testing.T) {
	ctx := context.Background()
	q, cleanup := newTestQueries(t) // use the file's existing helper name
	defer cleanup()
	projectID := seedProject(t, ctx, q) // use the file's existing helper

	// Buffer upsert is idempotent per project.
	if err := q.UpsertEditBuffer(ctx, sqlc.UpsertEditBufferParams{ProjectID: projectID, Content: "draft one"}); err != nil {
		t.Fatal(err)
	}
	if err := q.UpsertEditBuffer(ctx, sqlc.UpsertEditBufferParams{ProjectID: projectID, Content: "draft two"}); err != nil {
		t.Fatal(err)
	}
	buf, err := q.GetEditBuffer(ctx, projectID)
	if err != nil || buf != "draft two" {
		t.Fatalf("GetEditBuffer = %q, %v; want \"draft two\"", buf, err)
	}

	// Snapshot seq is monotonic and content immutable.
	next, err := q.NextSnapshotSeq(ctx, projectID)
	if err != nil || next != 1 {
		t.Fatalf("NextSnapshotSeq = %d, %v; want 1", next, err)
	}
	s1, err := q.InsertDraftSnapshot(ctx, sqlc.InsertDraftSnapshotParams{
		ProjectID: projectID, Seq: 1, Content: "v1 body", SpanIndex: []byte("[]"),
	})
	if err != nil {
		t.Fatal(err)
	}
	next2, _ := q.NextSnapshotSeq(ctx, projectID)
	if next2 != 2 {
		t.Fatalf("NextSnapshotSeq after one = %d, want 2", next2)
	}
	latest, err := q.GetLatestSnapshot(ctx, projectID)
	if err != nil || latest.ID != s1.ID {
		t.Fatalf("GetLatestSnapshot = %v, %v; want %v", latest.ID, err, s1.ID)
	}
}
```

Adjust helper names (`newTestQueries`, `seedProject`) to whatever `sqlc_test.go` already defines.

- [ ] **Step 8: Run the round-trip test — verify it passes**

Run: `cd apps/api && CGO_ENABLED=0 go test ./internal/store/ -run TestDraftSnapshotAndBufferRoundTrip`
Expected: PASS (needs Docker up).

- [ ] **Step 9: Commit**

```bash
git add apps/api/internal/store/queries/writing.sql apps/api/internal/store/sqlc apps/api/internal/agent/wordcount.go apps/api/internal/agent/wordcount_test.go apps/api/internal/store/sqlc_test.go
git commit -m "feat(refactor2): S5 writing.sql queries + CJK-aware CountWords"
```

---

## Task 3: Buffer + snapshot endpoints + `word_budget_ok` mint

**Files:**
- Create: `apps/api/internal/api/writing.go`
- Modify: `apps/api/internal/api/api.go` (routes)
- Test: `apps/api/internal/api/writing_test.go`

**Interfaces:**
- Consumes: Task 1 `sk.WordBudget`; Task 2 queries + `agent.CountWords`; `a.loadOwnedProject`, `decodeJSON`, `httpx`, `agent.NewSqlcAgentStore(...).AppendEvent`, `store.InsertGraphNode`/graph queries.
- Produces: `PUT /projects/{id}/buffer`, `POST /projects/{id}/snapshots` (201 → snapshot JSON). Consumed by Tasks 8 (projection reads the same rows) and 9 (frontend).

- [ ] **Step 1: Write the failing tests** — `apps/api/internal/api/writing_test.go` (follow `materials_test.go` for the testcontainers `API` harness + owned-project seed helpers):

```go
func TestPutEditBuffer_Upserts(t *testing.T) {
	h, projectID, cookie := newStudioAPI(t) // reuse materials_test.go's helper
	body := `{"content":"我在这里安静地写"}`
	rr := doReq(t, h, "PUT", "/api/v1/projects/"+projectID+"/buffer", body, cookie)
	if rr.Code != http.StatusNoContent {
		t.Fatalf("PUT buffer = %d, want 204; body=%s", rr.Code, rr.Body)
	}
	// Second PUT overwrites.
	rr2 := doReq(t, h, "PUT", "/api/v1/projects/"+projectID+"/buffer", `{"content":"改了"}`, cookie)
	if rr2.Code != http.StatusNoContent {
		t.Fatalf("second PUT = %d, want 204", rr2.Code)
	}
}

func TestCommitSnapshot_MintsWordBudgetOkInBand(t *testing.T) {
	h, projectID, cookie := newStudioAPI(t)
	// An in-band draft (1500..2000 words). Build 1600 CJK chars.
	inBand := strings.Repeat("字", 1600)
	rr := doReq(t, h, "POST", "/api/v1/projects/"+projectID+"/snapshots",
		`{"content":`+strconv.Quote(inBand)+`}`, cookie)
	if rr.Code != http.StatusCreated {
		t.Fatalf("commit = %d, want 201; body=%s", rr.Code, rr.Body)
	}
	var snap struct {
		ID        string `json:"id"`
		Seq       int    `json:"seq"`
		WordCount int    `json:"word_count"`
		InBand    bool   `json:"in_band"`
	}
	json.Unmarshal(rr.Body.Bytes(), &snap)
	if snap.Seq != 1 || snap.WordCount != 1600 || !snap.InBand {
		t.Fatalf("snapshot = %+v; want seq1 wc1600 inBand", snap)
	}
	// A word_budget_ok node now exists.
	if !hasNodeOfType(t, h, projectID, "word_budget_ok") {
		t.Fatal("word_budget_ok node not minted for in-band commit")
	}
}

func TestCommitSnapshot_NoMintOutOfBand(t *testing.T) {
	h, projectID, cookie := newStudioAPI(t)
	short := strings.Repeat("字", 50) // below min
	rr := doReq(t, h, "POST", "/api/v1/projects/"+projectID+"/snapshots",
		`{"content":`+strconv.Quote(short)+`}`, cookie)
	if rr.Code != http.StatusCreated {
		t.Fatalf("commit = %d, want 201", rr.Code)
	}
	if hasNodeOfType(t, h, projectID, "word_budget_ok") {
		t.Fatal("word_budget_ok minted for out-of-band commit")
	}
}
```

Add a small `hasNodeOfType` helper in the test file that lists graph nodes for the project (via a direct `Queries.ListGraphNodesByProject`) and checks for the type. Reuse `newStudioAPI`/`doReq` from `materials_test.go`; if their names differ, match the existing ones.

- [ ] **Step 2: Run — verify it fails**

Run: `cd apps/api && CGO_ENABLED=0 go test ./internal/api/ -run 'TestPutEditBuffer|TestCommitSnapshot'`
Expected: FAIL — 404 (routes not registered).

- [ ] **Step 3: Implement the handlers** — `apps/api/internal/api/writing.go`:

```go
package api

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/google/uuid"

	"mindimprint/api/internal/agent"
	"mindimprint/api/internal/httpx"
	"mindimprint/api/internal/skills"
	"mindimprint/api/internal/store/sqlc"
)

// putEditBuffer upserts the student's silent edit buffer. Student text ONLY —
// there is no path for AI output to reach this handler (RL-1). No entitlement
// gate: no model call, no network.
func (a *API) putEditBuffer(w http.ResponseWriter, r *http.Request) {
	projectID, ok := a.loadOwnedProject(w, r)
	if !ok {
		return
	}
	var body struct {
		Content string `json:"content"`
	}
	if err := decodeJSON(r, &body); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if err := a.d.Queries.UpsertEditBuffer(r.Context(), sqlc.UpsertEditBufferParams{
		ProjectID: projectID, Content: body.Content,
	}); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// snapshotResp is the committed-snapshot wire shape.
type snapshotResp struct {
	ID        string `json:"id"`
	Seq       int32  `json:"seq"`
	WordCount int    `json:"word_count"`
	InBand    bool   `json:"in_band"`
}

// commitSnapshot mints an immutable draft snapshot from the posted content
// (from the buffer or a paste — identical object, spec §10). In one
// transaction: insert the snapshot, mint OR remove the word_budget_ok node
// per the word count vs the skill band, and append a version_saved event.
func (a *API) commitSnapshot(w http.ResponseWriter, r *http.Request) {
	projectID, ok := a.loadOwnedProject(w, r)
	if !ok {
		return
	}
	var body struct {
		Content string `json:"content"`
	}
	if err := decodeJSON(r, &body); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if len(body.Content) == 0 {
		httpx.WriteError(w, r, httpx.ErrBadRequest("validation_failed", "草稿是空的，先写点东西再提交。", nil))
		return
	}

	sk, _ := skills.ByID("writing-project")
	wc := agent.CountWords(body.Content)
	inBand := sk.WordBudget != nil && wc >= sk.WordBudget.Min && wc <= sk.WordBudget.Max

	tx, err := a.d.Pool.Begin(r.Context())
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	defer func() { _ = tx.Rollback(r.Context()) }()
	qtx := a.d.Queries.WithTx(tx)

	seq, err := qtx.NextSnapshotSeq(r.Context(), projectID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	spanIndex := paragraphSpanIndex(body.Content) // []byte JSON
	snap, err := qtx.InsertDraftSnapshot(r.Context(), sqlc.InsertDraftSnapshotParams{
		ProjectID: projectID, Seq: seq, Content: body.Content, SpanIndex: spanIndex,
	})
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}

	// Mint or remove the word_budget_ok node so the S5 machine gate reflects
	// the LATEST snapshot honestly.
	if err := reconcileWordBudgetNode(r.Context(), qtx, projectID, inBand, wc, sk.WordBudget); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if err := tx.Commit(r.Context()); err != nil {
		httpx.WriteError(w, r, err)
		return
	}

	// Best-effort event (never fails the commit).
	store := agent.NewSqlcAgentStore(a.d.Queries, a.d.Pool)
	if err := store.AppendEvent(r.Context(), agent.EventRow{
		ProjectID: projectID, Surface: "studio", Type: "version_saved",
		Payload: mustJSON(map[string]any{"snapshot_id": snap.ID.String(), "seq": seq, "word_count": wc}),
	}); err != nil {
		slog.Warn("commit snapshot: append version_saved event failed", "err", err)
	}

	httpx.WriteJSON(w, http.StatusCreated, snapshotResp{
		ID: snap.ID.String(), Seq: seq, WordCount: wc, InBand: inBand,
	})
}
```

Add the helpers in the same file:

```go
// paragraphSpanIndex records each paragraph's [start,end) rune offsets so the
// snapshot is a span-indexed material (product spec §... "Material | Any text
// with span indices: a source, or a draft snapshot"). Paragraphs split on
// blank lines; single newlines stay within a paragraph.
func paragraphSpanIndex(s string) []byte {
	type span struct {
		Start int `json:"start"`
		End   int `json:"end"`
	}
	spans := []span{}
	runes := []rune(s)
	start := 0
	for i := 0; i <= len(runes); i++ {
		atBreak := i == len(runes) || (i+1 < len(runes) && runes[i] == '\n' && runes[i+1] == '\n')
		if atBreak {
			if i > start {
				spans = append(spans, span{Start: start, End: i})
			}
			start = i + 2
			i++
		}
	}
	b, _ := json.Marshal(spans)
	return b
}

// reconcileWordBudgetNode makes the word_budget_ok node present iff inBand.
// author:"ai" matches the gate_state/plan system-node convention (agentstore.go);
// it is a typed marker, not prose.
func reconcileWordBudgetNode(ctx context.Context, q *sqlc.Queries, projectID uuid.UUID, inBand bool, wc int, band *skills.WordBudget) error {
	existing, err := q.ListGraphNodesByProject(ctx, projectID)
	if err != nil {
		return err
	}
	var okID *uuid.UUID
	for i := range existing {
		if existing[i].Type == "word_budget_ok" {
			id := existing[i].ID
			okID = &id
			break
		}
	}
	if inBand {
		if okID != nil {
			return nil // already present
		}
		bodyMap := map[string]any{"word_count": wc}
		if band != nil {
			bodyMap["min"], bodyMap["max"] = band.Min, band.Max
		}
		_, err := q.InsertGraphNode(ctx, sqlc.InsertGraphNodeParams{
			ProjectID: projectID, Type: "word_budget_ok",
			Body: mustJSON(bodyMap), Author: "ai",
		})
		return err
	}
	if okID != nil {
		return q.DeleteGraphNode(ctx, sqlc.DeleteGraphNodeParams{ID: *okID, ProjectID: projectID})
	}
	return nil
}

func mustJSON(v any) []byte { b, _ := json.Marshal(v); return b }
```

If `DeleteGraphNode` does not exist in `graph.sql`, add it in this task:

```sql
-- name: DeleteGraphNode :exec
DELETE FROM graph_node WHERE id = $1 AND project_id = $2;
```

then re-run `make sqlc`. Add the `context` import to `writing.go`.

- [ ] **Step 4: Register the routes** — in `apps/api/internal/api/api.go`, after the `materials/{mid}/open` line:

```go
	mux.Handle("PUT /api/v1/projects/{id}/buffer", protected(a.putEditBuffer))
	mux.Handle("POST /api/v1/projects/{id}/snapshots", protected(a.commitSnapshot))
```

- [ ] **Step 5: Run the tests — verify they pass**

Run: `cd apps/api && CGO_ENABLED=0 go test ./internal/api/ -run 'TestPutEditBuffer|TestCommitSnapshot'`
Expected: PASS (Docker up).

- [ ] **Step 6: Commit**

```bash
git add apps/api/internal/api/writing.go apps/api/internal/api/api.go apps/api/internal/api/writing_test.go apps/api/internal/store/queries/graph.sql apps/api/internal/store/sqlc
git commit -m "feat(refactor2): S5 buffer + snapshot endpoints; in-band commit mints word_budget_ok"
```

---

## Task 4: `citations_matched` attestation endpoint

**Files:**
- Modify: `apps/api/internal/api/writing.go` (add `attestGate`)
- Modify: `apps/api/internal/api/api.go` (route)
- Test: `apps/api/internal/api/writing_test.go`

**Interfaces:**
- Consumes: `agent.NewSqlcAgentStore(...).ListGateStates` + `.UpsertGateState`; `skills.ByID`; `a.loadOwnedProject`.
- Produces: `POST /projects/{id}/gate/{contractId}/attest` body `{item, confirmed}`. Consumed by Tasks 8 (projection reads the gate item) and 10 (frontend control).

- [ ] **Step 1: Write the failing test** — append to `writing_test.go`:

```go
func TestAttestGate_RecordsStudentWrittenItem(t *testing.T) {
	h, projectID, cookie := newStudioAPI(t)
	// draft_polish's student_written item is "citations_matched".
	rr := doReq(t, h, "POST", "/api/v1/projects/"+projectID+"/gate/draft_polish/attest",
		`{"item":"citations_matched","confirmed":true}`, cookie)
	if rr.Code != http.StatusNoContent {
		t.Fatalf("attest = %d, want 204; body=%s", rr.Code, rr.Body)
	}
	// The gate_state records it as solid.
	store := agent.NewSqlcAgentStore(h.queries(t), h.pool(t)) // match test harness accessors
	rec, _ := store.ListGateStates(context.Background(), mustUUID(projectID))
	if rec["draft_polish"].Items["citations_matched"] != "solid" {
		t.Fatalf("citations_matched = %q, want solid", rec["draft_polish"].Items["citations_matched"])
	}
}

func TestAttestGate_RejectsUnknownItem(t *testing.T) {
	h, projectID, cookie := newStudioAPI(t)
	rr := doReq(t, h, "POST", "/api/v1/projects/"+projectID+"/gate/draft_polish/attest",
		`{"item":"word_budget_ok","confirmed":true}`, cookie) // machine item, not student_written
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("attest machine item = %d, want 400", rr.Code)
	}
}
```

Use whatever accessors the harness exposes for `Queries`/`Pool`; if none, load the gate state via a direct `sqlc.Queries` the test already has. Provide a `mustUUID` helper.

- [ ] **Step 2: Run — verify it fails**

Run: `cd apps/api && CGO_ENABLED=0 go test ./internal/api/ -run TestAttestGate`
Expected: FAIL — 404.

- [ ] **Step 3: Implement `attestGate`** — append to `writing.go`:

```go
// attestGate records a student_written gate item as solid (or clears it). The
// gate_state STORAGE already exists (UpsertGateState); nothing else records a
// student_written item as solid — the planner's Advance deliberately never
// marks non-machine items. Restricted to the contract's own student_written
// item names so a caller cannot forge a machine/human item. No model call.
func (a *API) attestGate(w http.ResponseWriter, r *http.Request) {
	projectID, ok := a.loadOwnedProject(w, r)
	if !ok {
		return
	}
	contractID := r.PathValue("contractId")
	var body struct {
		Item      string `json:"item"`
		Confirmed bool   `json:"confirmed"`
	}
	if err := decodeJSON(r, &body); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	sk, ok2 := skills.ByID("writing-project")
	if !ok2 {
		httpx.WriteError(w, r, httpx.ErrInternal())
		return
	}
	c, ok3 := sk.Contracts[contractID]
	if !ok3 {
		httpx.WriteError(w, r, httpx.ErrNotFound("资源不存在"))
		return
	}
	allowed := false
	for _, name := range c.Gate.StudentWritten {
		if name == body.Item {
			allowed = true
			break
		}
	}
	if !allowed {
		httpx.WriteError(w, r, httpx.ErrBadRequest("validation_failed", "该条目不是学生自评项", nil))
		return
	}

	store := agent.NewSqlcAgentStore(a.d.Queries, a.d.Pool)
	recorded, err := store.ListGateStates(r.Context(), projectID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	rec := recorded[contractID]
	if rec.Items == nil {
		rec.Items = map[string]string{}
	}
	if body.Confirmed {
		rec.Items[body.Item] = "solid"
	} else {
		delete(rec.Items, body.Item)
	}
	if err := store.UpsertGateState(r.Context(), projectID, contractID, rec); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
```

Confirm `agent.RecordedGate` exposes `Items map[string]string` + `Confirmed bool` (agentstore.go `gateStateBody`/`RecordedGate`); if the exported field names differ, match them.

- [ ] **Step 4: Register the route** — in `api.go`:

```go
	mux.Handle("POST /api/v1/projects/{id}/gate/{contractId}/attest", protected(a.attestGate))
```

- [ ] **Step 5: Run — verify it passes**

Run: `cd apps/api && CGO_ENABLED=0 go test ./internal/api/ -run TestAttestGate`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add apps/api/internal/api/writing.go apps/api/internal/api/api.go apps/api/internal/api/writing_test.go
git commit -m "feat(refactor2): S5 citations_matched student-written gate attestation endpoint"
```

---

## Task 5: `ProposeReview` model action + `order_review` verb

**Files:**
- Modify: `packages/contracts/src/agentOutput.ts` (add `order_review` to `Verb`)
- Create: `apps/api/internal/agent/review.go`
- Test: `apps/api/internal/agent/review_test.go`
- Test: `packages/contracts/test/agentOutput.test.ts` (verb accepted)

**Interfaces:**
- Consumes: `gateway.Provider`, `gateway.Resolved`, `gateway.Collect`, `enforcement.BannedPhrasing`/`OutputCheck`/`ValidateOutput`, `skills.ReviewCriterion`.
- Produces: `agent.ReviewItem` (`{CriterionCode, CriterionName, Band, Evidence, Missing, Fix string}`), `agent.ProposeReview(ctx, prov, r, criteria []skills.ReviewCriterion, paragraphs []string, graphSummary string) ([]ReviewItem, gateway.ChatUsage, error)`. Consumed by Task 6.

- [ ] **Step 1: Add the verb to the contract** — in `packages/contracts/src/agentOutput.ts`, add `"order_review",` to the `Verb` enum. Add a case to `packages/contracts/test/agentOutput.test.ts` asserting `Verb.parse("order_review")` succeeds.

- [ ] **Step 2: Run the contracts test — verify it fails then passes** (add verb, then):

Run: `cd packages/contracts && npm test -- agentOutput`
Expected: PASS after the enum edit.

- [ ] **Step 3: Write the failing Go test** — `apps/api/internal/agent/review_test.go`:

```go
package agent

import (
	"context"
	"strings"
	"testing"

	"mindimprint/api/internal/gateway"
	"mindimprint/api/internal/skills"
)

// fakeProvider returns a canned model reply. Match the gateway.Provider
// interface already used by coach_test.go / anchors_test.go fakes.
func reviewProvider(reply string) gateway.Provider { /* reuse existing fake pattern */ }

func TestProposeReview_ParsesWorkOrder(t *testing.T) {
	reply := `[
      {"criterion_code":"表E","band":"5–6 段","evidence":"第2段接住反方","missing":"变绿→可持续的跳步没补","fix":"补上可持续的定义"},
      {"criterion_code":"表H","band":"7–8 段","evidence":"结构清楚","missing":"","fix":""}
    ]`
	criteria := []skills.ReviewCriterion{{Code: "表E", Name: "分析"}, {Code: "表H", Name: "表达与组织"}}
	items, usage, err := ProposeReview(context.Background(), reviewProvider(reply), gateway.Resolved{Provider: "deepseek", Model: "x"}, criteria, []string{"p1", "p2"}, "claims:1 evidence:2")
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 {
		t.Fatalf("got %d items, want 2", len(items))
	}
	if items[0].CriterionCode != "表E" || items[0].CriterionName != "分析" || items[0].Fix == "" {
		t.Fatalf("item0 = %+v", items[0])
	}
	_ = usage
}

func TestProposeReview_RejectsBannedPhrase(t *testing.T) {
	// A reply whose fix rewrites the student's sentence for her — must be
	// rejected by the enforcement stack (banned-phrasing), not returned.
	reply := `[{"criterion_code":"表E","band":"5–6 段","evidence":"e","missing":"m","fix":"你应该这样写：中国的转型是叠加式的。"}]`
	criteria := []skills.ReviewCriterion{{Code: "表E", Name: "分析"}}
	_, _, err := ProposeReview(context.Background(), reviewProvider(reply), gateway.Resolved{Provider: "deepseek", Model: "x"}, criteria, []string{"p1"}, "")
	if err == nil {
		t.Fatal("expected banned-phrasing rejection, got nil")
	}
	_ = strings.TrimSpace
}
```

Implement `reviewProvider` by copying the fake-provider construction that `coach_test.go` or `anchors_test.go` already uses (a provider returning a fixed `gateway.ChatResponse`). If the banned-phrasing suite does not already flag "你应该这样写", add that phrase pattern to the banned list in this task (`enforcement`), since a review must never author prose.

- [ ] **Step 4: Run — verify it fails**

Run: `cd apps/api && CGO_ENABLED=0 go test ./internal/agent/ -run TestProposeReview`
Expected: FAIL — `ProposeReview` undefined.

- [ ] **Step 5: Implement `ProposeReview`** — `apps/api/internal/agent/review.go`:

```go
package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"mindimprint/api/internal/agent/enforcement"
	"mindimprint/api/internal/gateway"
	"mindimprint/api/internal/skills"
)

// ReviewItem is one work-order row: which criterion, the band the draft sits
// in, the evidence it submits, what is missing, and an OPTIONAL fix — advice
// only, never a rewritten sentence (RL-1).
type ReviewItem struct {
	CriterionCode string `json:"criterion_code"`
	CriterionName string `json:"criterion_name"`
	Band          string `json:"band"`
	Evidence      string `json:"evidence"`
	Missing       string `json:"missing"`
	Fix           string `json:"fix"`
}

// reviewItemWire is the model's per-item JSON contract (name is resolved
// server-side from the criterion code — the model never invents labels).
type reviewItemWire struct {
	CriterionCode string `json:"criterion_code"`
	Band          string `json:"band"`
	Evidence      string `json:"evidence"`
	Missing       string `json:"missing"`
	Fix           string `json:"fix"`
}

const reviewPosturePrompt = `你是 IB/国际课程写作的「整稿体检」考官。学生已提交一版草稿快照。
只做一件事：对照给定的评分表，指出每一张表现在收到了哪些证据、还缺什么。
铁律：绝不替学生改写句子、绝不给示范句、绝不续写。你的「建议」只能是"要补什么/要接什么"的方向，
不能是可直接粘贴的成品句子。一次只输出 JSON 数组，每个评分表一个对象。`

// ProposeReview asks the flagship model for a whole-draft work-order over the
// snapshot's paragraphs, then runs the full enforcement stack on every field
// before returning. A single banned-phrasing / output-check violation rejects
// the WHOLE review (nothing is returned or persisted) — the same all-or-nothing
// discipline as the coach. Usage is populated whenever Collect succeeded (even
// on a later rejection) so the caller can still record 档位+token+成本.
func ProposeReview(ctx context.Context, prov gateway.Provider, r gateway.Resolved, criteria []skills.ReviewCriterion, paragraphs []string, graphSummary string) ([]ReviewItem, gateway.ChatUsage, error) {
	name := map[string]string{}
	codes := make([]string, 0, len(criteria))
	for _, c := range criteria {
		name[c.Code] = c.Name
		codes = append(codes, c.Code+"（"+c.Name+"）")
	}
	user := fmt.Sprintf("评分表：%s\n\n论证摘要：%s\n\n草稿（分段）：\n%s",
		strings.Join(codes, "、"), graphSummary, strings.Join(paragraphs, "\n\n"))

	res, err := gateway.Collect(ctx, prov, r, gateway.ChatRequest{
		Messages: []gateway.ChatMessage{
			{Role: gateway.RoleSystem, Content: reviewPosturePrompt},
			{Role: gateway.RoleUser, Content: user},
		},
	})
	if err != nil {
		return nil, gateway.ChatUsage{}, err
	}
	usage := res.Usage

	var wires []reviewItemWire
	if err := json.Unmarshal([]byte(strings.TrimSpace(res.Text)), &wires); err != nil {
		return nil, usage, fmt.Errorf("agent: review output not a JSON array: %w", err)
	}
	items := make([]ReviewItem, 0, len(wires))
	for _, wv := range wires {
		nm, known := name[wv.CriterionCode]
		if !known {
			continue // ignore criteria the skill didn't ask for
		}
		// Enforcement on every free-text field the model produced.
		for _, field := range []string{wv.Evidence, wv.Missing, wv.Fix} {
			if field == "" {
				continue
			}
			if rule := enforcement.BannedPhrasing(field); rule != nil {
				return nil, usage, fmt.Errorf("agent: review output rejected by banned-phrasing rule %q", rule.Name)
			}
		}
		items = append(items, ReviewItem{
			CriterionCode: wv.CriterionCode, CriterionName: nm,
			Band: wv.Band, Evidence: wv.Evidence, Missing: wv.Missing, Fix: wv.Fix,
		})
	}
	if len(items) == 0 {
		return nil, usage, fmt.Errorf("agent: review produced no usable items")
	}
	return items, usage, nil
}
```

If `gateway.ChatResponse` uses a field other than `.Text` for the body, match the name `coach.go`/`anchors.go` read (they call `res.Text` or similar). Verify against `coach.go` before running.

- [ ] **Step 6: Run — verify it passes**

Run: `cd apps/api && CGO_ENABLED=0 go test ./internal/agent/ -run TestProposeReview`
Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add packages/contracts/src/agentOutput.ts packages/contracts/test/agentOutput.test.ts apps/api/internal/agent/review.go apps/api/internal/agent/review_test.go apps/api/internal/agent/enforcement
git commit -m "feat(refactor2): order_review verb + ProposeReview (flagship work-order through enforcement stack)"
```

---

## Task 6: Review endpoint (SSE) + persistence + one-snapshot-one-review

**Files:**
- Modify: `apps/api/internal/api/writing.go` (add `orderReview` + `snapshotEmitter` or reuse `studioEmitter`)
- Modify: `apps/api/internal/gateway/sse.go` (add `Review` SSE method)
- Modify: `apps/api/internal/api/studioturn.go` (add `Review` to `studioEmitter`)
- Modify: `apps/api/internal/api/api.go` (route)
- Modify: `apps/api/internal/store/queries/intervention.sql` (add `ListInterventionsBySnapshot` if not derivable)
- Test: `apps/api/internal/api/writing_test.go`

**Interfaces:**
- Consumes: Task 2 `GetSnapshot`; Task 5 `ProposeReview`; Task 1 `sk.ReviewCriteria`; `store.RecordLLMCall`, `store.AppendEvent`, intervention insert query, `ListInterventionsByProject`.
- Produces: `POST /projects/{id}/snapshots/{sid}/review` (SSE, one `review` event carrying the work-order). Consumed by Tasks 8, 10.

- [ ] **Step 1: Add the SSE `Review` method** — in `apps/api/internal/gateway/sse.go`:

```go
// Review streams the whole-draft work-order as one event (the review is a
// batch, not incremental). items is the JSON-encoded []ReviewItemWire.
func (s *SSEWriter) Review(items []byte) error {
	return s.writeEvent("review", json.RawMessage(items))
}
```

Add `Review([]byte) error` to `studioEmitter` in `studioturn.go` mirroring the other mutex-guarded methods.

- [ ] **Step 2: Write the failing test** — append to `writing_test.go`:

```go
func TestOrderReview_PersistsWorkOrderAndIsIdempotent(t *testing.T) {
	h, projectID, cookie := newStudioAPIWithReviewModel(t, `[
	  {"criterion_code":"表E","band":"5–6 段","evidence":"第2段接住反方","missing":"跳步没补","fix":"补上定义"}
	]`) // harness injects a canned review provider
	// commit a snapshot first
	rr := doReq(t, h, "POST", "/api/v1/projects/"+projectID+"/snapshots",
		`{"content":`+strconv.Quote(strings.Repeat("字", 1600))+`}`, cookie)
	var snap struct{ ID string `json:"id"` }
	json.Unmarshal(rr.Body.Bytes(), &snap)

	// order review — SSE stream carries one `review` event with 1 item
	body := doReqRaw(t, h, "POST", "/api/v1/projects/"+projectID+"/snapshots/"+snap.ID+"/review", ``, cookie)
	if !strings.Contains(body, `"criterion_code":"表E"`) {
		t.Fatalf("review stream missing work order: %s", body)
	}
	// persisted as a review_item intervention
	ivs := listInterventions(t, h, projectID)
	n := 0
	for _, iv := range ivs {
		if iv.Type == "review_item" {
			n++
		}
	}
	if n != 1 {
		t.Fatalf("review_item interventions = %d, want 1", n)
	}
	// second review on the SAME snapshot returns the SAME rows, no new model call
	doReqRaw(t, h, "POST", "/api/v1/projects/"+projectID+"/snapshots/"+snap.ID+"/review", ``, cookie)
	ivs2 := listInterventions(t, h, projectID)
	n2 := 0
	for _, iv := range ivs2 {
		if iv.Type == "review_item" {
			n2++
		}
	}
	if n2 != 1 {
		t.Fatalf("after 2nd review, review_item interventions = %d, want 1 (idempotent)", n2)
	}
	// Ordering the review recorded the S5 human gate item.
	store := agent.NewSqlcAgentStore(h.queries(t), h.pool(t)) // match harness accessors
	rec, _ := store.ListGateStates(context.Background(), mustUUID(projectID))
	if rec["draft_polish"].Items["whole_draft_review"] != "solid" {
		t.Fatalf("whole_draft_review = %q, want solid", rec["draft_polish"].Items["whole_draft_review"])
	}
}
```

Provide `newStudioAPIWithReviewModel` (a variant of `newStudioAPI` that injects a canned review provider into `Deps`), `doReqRaw` (returns the raw SSE body string), and `listInterventions` helpers, mirroring the studioturn/projectcards test harnesses.

- [ ] **Step 3: Run — verify it fails**

Run: `cd apps/api && CGO_ENABLED=0 go test ./internal/api/ -run TestOrderReview`
Expected: FAIL — 404.

- [ ] **Step 4: Implement `orderReview`** — append to `writing.go`:

```go
// orderReview runs the student-triggered whole-draft review over a committed
// snapshot. One snapshot, one review (spec §12): if review_item interventions
// already anchor this snapshot, stream them back — NO second model call. The
// review writes ONLY intervention rows (typed advice), never prose (RL-1).
func (a *API) orderReview(w http.ResponseWriter, r *http.Request) {
	projectID, ok := a.loadOwnedProject(w, r)
	if !ok {
		return
	}
	u, _ := UserFromContext(r.Context())
	sid, err := uuid.Parse(r.PathValue("sid"))
	if err != nil {
		httpx.WriteError(w, r, httpx.ErrNotFound("资源不存在"))
		return
	}
	snap, err := a.d.Queries.GetSnapshot(r.Context(), sqlc.GetSnapshotParams{ID: sid, ProjectID: projectID})
	if err != nil {
		httpx.WriteError(w, r, httpx.ErrNotFound("资源不存在"))
		return
	}

	store := agent.NewSqlcAgentStore(a.d.Queries, a.d.Pool)
	existing := reviewItemsForSnapshot(r.Context(), a.d.Queries, projectID, sid)

	// Entitlement gate BEFORE the stream — only when a model call will happen.
	if len(existing) == 0 {
		entitled, eerr := HasEntitlement(r.Context(), u)
		if eerr != nil {
			httpx.WriteError(w, r, eerr)
			return
		}
		if !entitled {
			httpx.WriteError(w, r, httpx.ErrNotEntitled())
			return
		}
	}

	sse, err := gateway.NewSSEWriter(w)
	if err != nil {
		httpx.WriteError(w, r, httpx.ErrInternal())
		return
	}
	em := &studioEmitter{sse: sse}
	stop, hbDone := startHeartbeat(r.Context(), em) // extract the studioturn heartbeat goroutine into a helper
	defer func() { close(stop); <-hbDone }()

	if len(existing) > 0 { // idempotent replay
		_ = em.Review(mustJSON(existing))
		_ = em.Done()
		return
	}

	sk, _ := skills.ByID("writing-project")
	resolved, rerr := a.d.ChatResolver(r.Context())
	if rerr != nil {
		_ = em.ErrorEnvelope("internal_error", "体检失败，请重试")
		_ = em.Done()
		return
	}
	paras := snapshotParagraphs(snap.Content)
	items, usage, perr := agent.ProposeReview(r.Context(), a.d.Provider, resolved, sk.ReviewCriteria, paras, graphSummary(r.Context(), a.d.Queries, projectID))
	// Record the call cost even if enforcement then rejected the output.
	if resolved.Provider != "" {
		if err := store.RecordLLMCall(r.Context(), agent.LLMCallRow{
			ProjectID: projectID, Surface: "studio", Purpose: "order_review",
			Resolved: resolved, PromptTokens: int32(usage.PromptTokens), CompletionTokens: int32(usage.CompletionTokens),
		}); err != nil {
			slog.Warn("order_review: record llm call", "err", err)
		}
	}
	if perr != nil {
		// Enforcement rejection or parse failure: persist NOTHING, stream nothing.
		slog.Warn("order_review: proposal rejected", "err", perr)
		_ = em.ErrorEnvelope("review_rejected", "这次体检没通过内部校验，请再试一次")
		_ = em.Done()
		return
	}

	// Persist each item as a review_item intervention anchored to the snapshot.
	// The FULL ReviewItem is marshalled into intervention.body (lossless
	// reconstruction by Task 8); criterion/level stay duplicated in their flat
	// columns for any SQL that filters on them.
	anchor := mustJSON(map[string]string{"kind": "draft_snapshot", "id": sid.String()})
	persisted := make([]agent.ReviewItem, 0, len(items))
	for _, it := range items {
		if err := store.InsertReviewIntervention(r.Context(), agent.ReviewInterventionRow{
			ProjectID: projectID, Anchor: anchor,
			Criterion: it.CriterionCode + " " + it.CriterionName,
			Body:      string(mustJSON(it)), Level: it.Band,
		}); err != nil {
			slog.Warn("order_review: persist item", "err", err)
			continue
		}
		persisted = append(persisted, it)
	}
	// Ordering a review satisfies the S5 HUMAN gate item whole_draft_review.
	// Like citations_matched, nothing else records a human item as solid, so
	// do it here (best-effort — never fails the stream). draft_polish is the
	// only contract with this item; guard on its presence in the skill.
	if c, ok := sk.Contracts["draft_polish"]; ok {
		for _, h := range c.Gate.Human {
			if h == "whole_draft_review" {
				recorded, _ := store.ListGateStates(r.Context(), projectID)
				rec := recorded["draft_polish"]
				if rec.Items == nil {
					rec.Items = map[string]string{}
				}
				rec.Items["whole_draft_review"] = "solid"
				if err := store.UpsertGateState(r.Context(), projectID, "draft_polish", rec); err != nil {
					slog.Warn("order_review: record whole_draft_review", "err", err)
				}
				break
			}
		}
	}
	_ = store.AppendEvent(r.Context(), agent.EventRow{
		ProjectID: projectID, Surface: "studio", Type: "review_ordered",
		Payload: mustJSON(map[string]any{"snapshot_id": sid.String(), "items": len(persisted)}),
	})
	if err := a.d.Queries.TouchProject(r.Context(), projectID); err != nil {
		slog.Warn("order_review: touch project", "err", err)
	}
	_ = em.Review(mustJSON(persisted))
	_ = em.Done()
}
```

Add the small helpers in `writing.go`:

```go
func snapshotParagraphs(content string) []string {
	out := []string{}
	for _, p := range strings.Split(content, "\n\n") {
		if s := strings.TrimSpace(p); s != "" {
			out = append(out, s)
		}
	}
	return out
}

// reviewItemsForSnapshot returns the persisted review items anchored to sid,
// reconstructed from their intervention rows (empty if none — the not-yet-
// reviewed state).
func reviewItemsForSnapshot(ctx context.Context, q *sqlc.Queries, projectID, sid uuid.UUID) []agent.ReviewItem {
	ivs, err := q.ListInterventionsByProject(ctx, projectID)
	if err != nil {
		return nil
	}
	out := []agent.ReviewItem{}
	for _, iv := range ivs {
		if iv.Type != "review_item" {
			continue
		}
		var a struct{ Kind, ID string }
		_ = json.Unmarshal(iv.Anchor, &a)
		if a.Kind != "draft_snapshot" || a.ID != sid.String() {
			continue
		}
		out = append(out, reviewItemFromIntervention(iv))
	}
	return out
}
```

`reviewItemFromIntervention` and `graphSummary` are small pure helpers: `reviewItemFromIntervention` does `var it ReviewItem; json.Unmarshal(iv.Body, &it); it.CriterionName = it.CriterionName` — i.e. the body IS the marshalled `ReviewItem`, so reconstruction is one `json.Unmarshal` (lossless, no string-splitting). `graphSummary` counts claim/evidence nodes into a short string like `"主张 1 · 证据 2"`. This is why Task 6 stores the whole item as JSON in `intervention.body` (no migration, no extra column); `criterion`/`level` stay duplicated in their flat columns only for SQL filters.

Add the store seam in `agentstore.go`:

```go
// ReviewInterventionRow is one persisted whole-draft-review work-order item.
// Body is the JSON of the full ReviewItem (Task 8 reconstructs from it);
// Criterion/Level are duplicated flat for SQL filters.
type ReviewInterventionRow struct {
	ProjectID uuid.UUID
	Anchor    []byte
	Criterion string
	Body      string // JSON of the ReviewItem (lossless reconstruction)
	Level     string
}

func (s *sqlcAgentStore) InsertReviewIntervention(ctx context.Context, row ReviewInterventionRow) error {
	_, err := s.q.InsertIntervention(ctx, sqlc.InsertInterventionParams{
		ProjectID: row.ProjectID, Type: "review_item", Anchor: row.Anchor,
		Criterion: pgText(row.Criterion), Body: row.Body, Level: pgText(row.Level),
	})
	return err
}
```

Match `InsertIntervention`'s actual generated param names/types (check `intervention.sql`; if there is no generic insert, add one). Add `InsertReviewIntervention` to the `AgentStore` interface.

- [ ] **Step 5: Register the route** — in `api.go`:

```go
	mux.Handle("POST /api/v1/projects/{id}/snapshots/{sid}/review", protected(a.orderReview))
```

- [ ] **Step 6: Run — verify it passes**

Run: `cd apps/api && CGO_ENABLED=0 go test ./internal/api/ -run TestOrderReview`
Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add apps/api/internal/api/writing.go apps/api/internal/api/api.go apps/api/internal/api/studioturn.go apps/api/internal/gateway/sse.go apps/api/internal/agent/agentstore.go apps/api/internal/store/queries/intervention.sql apps/api/internal/store/sqlc apps/api/internal/api/writing_test.go
git commit -m "feat(refactor2): S5 whole-draft review endpoint — SSE work-order, persisted, one-snapshot-one-review"
```

---

## Task 7: Contracts — `WritingProjection`

**Files:**
- Modify: `packages/contracts/src/studioState.ts`
- Test: `packages/contracts/test/studioState.test.ts`

**Interfaces:**
- Produces: `WritingProjection`, `WritingSnapshot`, `WritingReviewItem` Zod schemas + a required `writing` field on `StudioProjection`. Consumed by Tasks 8 (Go parity target) and 9/10 (frontend types).

- [ ] **Step 1: Write the failing tests** — append to `studioState.test.ts`:

```ts
import { WritingProjection, StudioProjection } from "../src/studioState";

test("WritingProjection parses a full writing view", () => {
  const ok = {
    buffer: "我在写",
    latestSnapshot: { id: "s1", seq: 3, committedAt: "2026-07-15T00:00:00Z", wordCount: 1723, inBand: true },
    wordBudget: { min: 1500, max: 2000 },
    citationsMatched: false,
    review: { ordered: true, items: [
      { interventionId: "i1", criterion: "表E 分析", band: "5–6 段", evidence: "e", missing: "m", fix: "f",
        disposition: { action: "rewrite", reason: "我打算把跳步补成一段推理，至少十五个字。" } },
    ] },
  };
  expect(() => WritingProjection.parse(ok)).not.toThrow();
});

test("WritingReviewItem rejects a bad disposition action", () => {
  const bad = { buffer: "", latestSnapshot: null, wordBudget: { min: 1, max: 2 }, citationsMatched: false,
    review: { ordered: true, items: [
      { interventionId: "i", criterion: "c", band: "b", evidence: "", missing: "", fix: "",
        disposition: { action: "keep", reason: "x" } }] } };
  expect(() => WritingProjection.parse(bad)).toThrow();
});

test("StudioProjection requires writing", () => {
  const p: any = { /* copy an existing valid StudioProjection fixture from this file */ };
  delete p.writing;
  expect(() => StudioProjection.parse(p)).toThrow();
});
```

For the third test, build `p` from whatever full-`StudioProjection` fixture the file already uses (with `writing` present), then delete it.

- [ ] **Step 2: Run — verify it fails**

Run: `cd packages/contracts && npm test -- studioState`
Expected: FAIL — `WritingProjection` not exported.

- [ ] **Step 3: Implement the schemas** — in `packages/contracts/src/studioState.ts`:

```ts
export const WritingSnapshot = z.object({
  id: z.string(),
  seq: z.number().int(),
  committedAt: z.string(),
  wordCount: z.number().int(),
  inBand: z.boolean(),
});
export type WritingSnapshot = z.infer<typeof WritingSnapshot>;

export const WritingReviewItem = z.object({
  interventionId: z.string(),
  criterion: z.string(),
  band: z.string(),
  evidence: z.string(),
  missing: z.string(),
  fix: z.string(),
  disposition: z
    .object({ action: z.enum(["accept", "reject", "rewrite"]), reason: z.string() })
    .nullable(),
});
export type WritingReviewItem = z.infer<typeof WritingReviewItem>;

export const WritingProjection = z.object({
  buffer: z.string(),
  latestSnapshot: WritingSnapshot.nullable(),
  wordBudget: z.object({ min: z.number().int(), max: z.number().int() }),
  citationsMatched: z.boolean(),
  review: z.object({ ordered: z.boolean(), items: z.array(WritingReviewItem) }),
});
export type WritingProjection = z.infer<typeof WritingProjection>;
```

Add `writing: WritingProjection` (required) to the `StudioProjection` object.

- [ ] **Step 4: Backfill existing test payloads** — any pre-existing `StudioProjection.parse(...)` fixture in `studioState.test.ts` (and elsewhere in `packages/contracts/test`) now needs a `writing` key. Add:

```ts
  writing: { buffer: "", latestSnapshot: null, wordBudget: { min: 1500, max: 2000 }, citationsMatched: false, review: { ordered: false, items: [] } },
```

Grep `packages/contracts/test` for `StudioProjection.parse` and fix each.

- [ ] **Step 5: Run — verify it passes**

Run: `cd packages/contracts && npm test`
Expected: PASS (full contracts suite).

- [ ] **Step 6: Commit**

```bash
git add packages/contracts/src/studioState.ts packages/contracts/test/studioState.test.ts
git commit -m "feat(refactor2): WritingProjection contract + required writing on StudioProjection"
```

---

## Task 8: `projectWriting` + DTO parity + `ListDispositionsByProject`

**Files:**
- Modify: `apps/api/internal/store/queries/disposition.sql` (+ `make sqlc`)
- Modify: `apps/api/internal/studio/dto.go` (`WritingDTO` etc. + `Writing` field)
- Modify: `apps/api/internal/studio/load.go` (load buffer, latest snapshot, dispositions)
- Modify: `apps/api/internal/studio/projection.go` (`projectWriting` + wire into `Project`; `ProjectData` fields)
- Test: `apps/api/internal/studio/projection_test.go`
- Test: `apps/api/internal/studio/dto_parity_test.go`

**Interfaces:**
- Consumes: Task 1 `sk.WordBudget`; Task 2 `GetEditBuffer`/`GetLatestSnapshot`; Task 6 `review_item` interventions + `agent.CountWords`; a new `ListDispositionsByProject`.
- Produces: `StudioProjection.Writing WritingDTO` matching the Task 7 Zod shape exactly.

- [ ] **Step 1: Add the disposition query** — in `disposition.sql`:

```sql
-- name: ListDispositionsByProject :many
SELECT d.* FROM disposition d
JOIN intervention i ON i.id = d.intervention_id
WHERE i.project_id = $1
ORDER BY d.created_at;
```

Run `make sqlc`.

- [ ] **Step 2: Write the failing parity + projection tests** — in `dto_parity_test.go`, add `"writing"` to the top-level `want` key set and assert the nested key sets:

```go
assertKeys(t, top["writing"], []string{"buffer", "citationsMatched", "latestSnapshot", "review", "wordBudget"})
```

(and a `WritingReviewItem` key-set assertion `{"band","criterion","disposition","evidence","fix","interventionId","missing"}`, and `WritingSnapshot` `{"committedAt","id","inBand","seq","wordCount"}`). In `projection_test.go`, add:

```go
func TestProjectWriting(t *testing.T) {
	sk, _ := skills.ByID("writing-project")
	// no buffer, no snapshot → empty buffer, nil snapshot, review not ordered
	d := ProjectData{ /* minimal: Project set, empty slices */ }
	pw := projectWriting(sk, d)
	if pw.Buffer != "" || pw.LatestSnapshot != nil || pw.Review.Ordered {
		t.Fatalf("empty project writing = %+v", pw)
	}
	if pw.WordBudget.Min != 1500 || pw.WordBudget.Max != 2000 {
		t.Fatalf("word budget = %+v", pw.WordBudget)
	}
}
```

(A fuller real-Postgres assertion is exercised by the Task 6 e2e + a projection round-trip; keep this unit test on the pure derivation.)

- [ ] **Step 3: Run — verify it fails**

Run: `cd apps/api && CGO_ENABLED=0 go test ./internal/studio/ -run 'TestProjectWriting|Parity'`
Expected: FAIL — `projectWriting`/`Writing` undefined.

- [ ] **Step 4: Add the DTO** — in `dto.go`:

```go
type WritingSnapshotDTO struct {
	ID          string `json:"id"`
	Seq         int    `json:"seq"`
	CommittedAt string `json:"committedAt"`
	WordCount   int    `json:"wordCount"`
	InBand      bool   `json:"inBand"`
}

type WritingReviewItemDTO struct {
	InterventionID string           `json:"interventionId"`
	Criterion      string           `json:"criterion"`
	Band           string           `json:"band"`
	Evidence       string           `json:"evidence"`
	Missing        string           `json:"missing"`
	Fix            string           `json:"fix"`
	Disposition    *DispositionDTO  `json:"disposition"`
}

type DispositionDTO struct {
	Action string `json:"action"`
	Reason string `json:"reason"`
}

type WritingDTO struct {
	Buffer          string                 `json:"buffer"`
	LatestSnapshot  *WritingSnapshotDTO    `json:"latestSnapshot"`
	WordBudget      WordBudgetDTO          `json:"wordBudget"`
	CitationsMatched bool                  `json:"citationsMatched"`
	Review          WritingReviewDTO       `json:"review"`
}

type WordBudgetDTO struct {
	Min int `json:"min"`
	Max int `json:"max"`
}

type WritingReviewDTO struct {
	Ordered bool                   `json:"ordered"`
	Items   []WritingReviewItemDTO `json:"items"`
}
```

Add `Writing WritingDTO json:"writing"` to `StudioProjection`.

- [ ] **Step 5: Load the new data** — in `ProjectData` (projection.go) add:

```go
	EditBuffer     string
	LatestSnapshot *sqlc.DraftSnapshot
	Dispositions   []sqlc.Disposition
```

In `load.go`, after `sourceLog`:

```go
	buffer, err := q.GetEditBuffer(ctx, projectID)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return ProjectData{}, err
	}
	d.EditBuffer = buffer // "" when ErrNoRows

	latest, err := q.GetLatestSnapshot(ctx, projectID)
	switch {
	case err == nil:
		d.LatestSnapshot = &latest
	case errors.Is(err, pgx.ErrNoRows):
	default:
		return ProjectData{}, err
	}

	disps, err := q.ListDispositionsByProject(ctx, projectID)
	if err != nil {
		return ProjectData{}, err
	}
	d.Dispositions = disps
```

(Set `d.EditBuffer`/`d.LatestSnapshot`/`d.Dispositions` on the already-constructed `d`; adjust to the file's construction order.)

- [ ] **Step 6: Implement `projectWriting`** — in `projection.go`:

```go
// projectWriting projects the S5 写作 view: the silent buffer, the latest
// immutable snapshot, the word budget, the citations attestation, and the
// whole-draft review work-order (review_item interventions ⋈ dispositions).
// Derive-never-decorate: every field is read back from persisted rows.
func projectWriting(sk skills.Skill, d ProjectData) WritingDTO {
	out := WritingDTO{Buffer: d.EditBuffer, Review: WritingReviewDTO{Items: []WritingReviewItemDTO{}}}
	if sk.WordBudget != nil {
		out.WordBudget = WordBudgetDTO{Min: sk.WordBudget.Min, Max: sk.WordBudget.Max}
	}
	if d.LatestSnapshot != nil {
		wc := agent.CountWords(d.LatestSnapshot.Content)
		inBand := sk.WordBudget != nil && wc >= sk.WordBudget.Min && wc <= sk.WordBudget.Max
		out.LatestSnapshot = &WritingSnapshotDTO{
			ID:          d.LatestSnapshot.ID.String(),
			Seq:         int(d.LatestSnapshot.Seq),
			CommittedAt: d.LatestSnapshot.CreatedAt.Time.Format(time.RFC3339),
			WordCount:   wc, InBand: inBand,
		}
	}
	// citations_matched from the draft_polish gate state.
	recorded := agent.RecordedGatesFromNodes(d.GateStates)
	if rec, ok := recorded["draft_polish"]; ok && rec.Items["citations_matched"] == "solid" {
		out.CitationsMatched = true
	}
	// review_item interventions anchored to the latest snapshot, joined with dispositions.
	dispByIv := map[string]sqlc.Disposition{}
	for _, dp := range d.Dispositions {
		dispByIv[dp.InterventionID.String()] = dp // last write wins (latest)
	}
	for _, iv := range d.Interventions {
		if iv.Type != "review_item" {
			continue
		}
		if d.LatestSnapshot != nil {
			var a struct{ Kind, ID string }
			_ = json.Unmarshal(iv.Anchor, &a)
			if a.ID != d.LatestSnapshot.ID.String() {
				continue // only the current snapshot's review
			}
		}
		item := reviewItemDTOFromIntervention(iv) // unmarshal iv.Body JSON → fields
		if dp, ok := dispByIv[iv.ID.String()]; ok {
			item.Disposition = &DispositionDTO{Action: dp.Action, Reason: dp.Reason}
		}
		out.Review.Ordered = true
		out.Review.Items = append(out.Review.Items, item)
	}
	return out
}
```

`reviewItemDTOFromIntervention` unmarshals the `ReviewItem` JSON that Task 6 stored in `iv.Body` and sets `InterventionID: iv.ID.String()`, `Criterion: iv.Criterion` (or from the JSON), `Band: iv.Level`, etc. Keep the reconstruction identical to Task 6's storage shape (share a helper if both packages can import it, else mirror it exactly). Wire into `Project`:

```go
	Writing: projectWriting(sk, d),
```

- [ ] **Step 7: Run — verify it passes**

Run: `cd apps/api && CGO_ENABLED=0 go test ./internal/studio/`
Expected: PASS (full studio package incl. real-Postgres projection round-trip).

- [ ] **Step 8: Commit**

```bash
git add apps/api/internal/store/queries/disposition.sql apps/api/internal/store/sqlc apps/api/internal/studio/dto.go apps/api/internal/studio/load.go apps/api/internal/studio/projection.go apps/api/internal/studio/projection_test.go apps/api/internal/studio/dto_parity_test.go
git commit -m "feat(refactor2): projectWriting — buffer/snapshot/wordBudget/citations/review⋈dispositions"
```

---

## Task 9: Frontend — live edit / commit / preview

**Files:**
- Create: `apps/web/src/api/writing.ts`
- Modify: `apps/web/src/studio/state.ts` (adopt `WritingProjection`)
- Modify: `apps/web/src/studio/views/WritingView.tsx`
- Modify: `apps/web/src/studio/StudioContainer.tsx` (un-stub `writing: p.writing`, wire buffer/commit)
- Modify: `apps/web/src/studio/ViewFrame.tsx` (pass props)
- Test: `apps/web/src/studio/views/WritingView.test.tsx`
- Test: `apps/web/src/api/writing.test.ts`

**Interfaces:**
- Consumes: Task 3 `PUT /buffer` + `POST /snapshots`; Task 7 `WritingProjection` type.
- Produces: `WritingView` props = `WritingProjection` + callbacks `onBufferChange(text)`, `onCommit(text)`. Consumed by Task 10 (adds review/disposition/attest).

- [ ] **Step 1: Write `api/writing.ts`** with a Zod-parsing snapshot response + a debounced buffer put; write `api/writing.test.ts` first asserting `commitSnapshot` parses the server shape and `putBuffer` PUTs the content. (Model on `api/materials.ts` + its test.)

```ts
import { z } from "zod";

const SnapshotResp = z.object({
  id: z.string(), seq: z.number(), word_count: z.number(), in_band: z.boolean(),
});

export async function putBuffer(projectId: string, content: string): Promise<void> {
  const res = await fetch(`/api/v1/projects/${projectId}/buffer`, {
    method: "PUT", headers: { "content-type": "application/json" },
    body: JSON.stringify({ content }), credentials: "same-origin",
  });
  if (!res.ok) throw new Error(`buffer save failed: ${res.status}`);
}

export async function commitSnapshot(projectId: string, content: string) {
  const res = await fetch(`/api/v1/projects/${projectId}/snapshots`, {
    method: "POST", headers: { "content-type": "application/json" },
    body: JSON.stringify({ content }), credentials: "same-origin",
  });
  if (!res.ok) throw new Error(`commit failed: ${res.status}`);
  return SnapshotResp.parse(await res.json());
}
```

- [ ] **Step 2: Run the api test — fail then pass**

Run: `cd apps/web && npm test -- writing`
Expected: PASS after implementing.

- [ ] **Step 3: Write the failing WritingView test** — `WritingView.test.tsx` (replace the old `{draft, mode}` tests): editing the textarea calls `onBufferChange`; a commit calls `onCommit`; preview renders the snapshot's paragraphs + `snapshotMeta` from `latestSnapshot`; the 整稿体检 button is disabled when `latestSnapshot` is null and enabled otherwise.

- [ ] **Step 4: Rewrite `WritingView`** to take the `WritingProjection` (via state.ts type) + callbacks. Bind the textarea `value={buffer}` `onChange`, make it **editable** (remove `readOnly`), keep the design's silent-edit hint copy verbatim, render the real snapshot in preview, and compute `snapshotMeta` = `第 ${latestSnapshot.seq} 版快照 · 提交 · 只读` (or the not-yet-committed empty state). Enable 整稿体检 only when `latestSnapshot` exists (design's `onOrderReview`). Keep the work-order block behind `review.ordered` (Task 10 fills it; for now render the design's "还没做体检" placeholder when `!review.ordered`).

- [ ] **Step 5: Adopt the contract type in `state.ts`** — replace `writing: { draft: string; mode: "edit" | "preview" }` with `writing: WritingProjection` (import from contracts). Add the mode as local component state in `WritingView` (edit/preview is a client toggle, not server state).

- [ ] **Step 6: Wire `StudioContainer`** — `toStudioState`: `writing: p.writing`. Add `onBufferChange` (debounced `putBuffer`) + `onCommit` (`commitSnapshot` then refetch the projection so `latestSnapshot`/gate update). Update the stale deferred-views comment.

- [ ] **Step 7: Run web suite — verify pass**

Run: `cd apps/web && npm test && npx tsc --noEmit -p apps/web`
Expected: PASS + clean tsc.

- [ ] **Step 8: Commit**

```bash
git add apps/web/src/api/writing.ts apps/web/src/api/writing.test.ts apps/web/src/studio/state.ts apps/web/src/studio/views/WritingView.tsx apps/web/src/studio/views/WritingView.test.tsx apps/web/src/studio/StudioContainer.tsx apps/web/src/studio/ViewFrame.tsx
git commit -m "feat(refactor2): S5 live writing view — silent buffer autosave + snapshot commit + preview"
```

---

## Task 10: Frontend — 整稿体检 work-order + disposition + citations attest

**Files:**
- Modify: `apps/web/src/api/writing.ts` (`orderReview` SSE + `attestGate`)
- Modify: `apps/web/src/api/studioTurn.ts` (`review` frame mapping)
- Modify: `apps/web/src/studio/views/WritingView.tsx` (work-order block + three keys + citations control)
- Modify: `apps/web/src/studio/StudioContainer.tsx` (wire `onOrderReview`, `onReviewDisposition`, `onAttestCitations`)
- Modify: `apps/web/src/studio/ViewFrame.tsx` (pass callbacks)
- Test: `apps/web/src/studio/views/WritingView.test.tsx`

**Interfaces:**
- Consumes: Task 6 review SSE (`review` event) + `POST …/interventions/{iid}/disposition` (existing) + Task 4 attest endpoint.
- Produces: the completed S5 view.

- [ ] **Step 1: Add `review` frame mapping** — in `studioTurn.ts` `mapStudioFrame`, add `case "review": return { type: "review", items: data };` and extend `StudioTurnEvent`. Add `orderReview(projectId, snapshotId)` to `writing.ts` as an async generator over the SSE stream (reuse the studioTurn fetch/parse plumbing), yielding the `review` event's items. Add `attestGate(projectId, contractId, item, confirmed)` (POST, 204).

- [ ] **Step 2: Write the failing test** — `WritingView.test.tsx`: given a `review.ordered` projection with one item, the work-order block renders the criterion, band chip, evidence/missing, and the three keys 保持原样/我来改/说明为什么不改; clicking 我来改 + entering a ≥15-rune reason calls `onReviewDisposition("i1","rewrite", reason)`; the citations control calls `onAttestCitations(true)`.

- [ ] **Step 3: Implement the work-order block** — in `WritingView`, when `review.ordered`, render the design's work-order markup verbatim (§ dc.html 1120–1160): per item `criterion` + band chip (`w.bandStyle` tones) + evidence/missing + `fix` gated three keys. Map the keys to disposition actions: 保持原样→`accept`, 我来改→`rewrite`, 说明为什么不改→`reject`; each requires a ≥15-rune reason before calling `onReviewDisposition(interventionId, action, reason)`. Show the persisted `item.disposition` as the selected key when present. Add the `citations_matched` attestation control (a labelled checkbox/toggle, binding copy) calling `onAttestCitations`.

- [ ] **Step 4: Wire `StudioContainer`** — `onOrderReview(snapshotId)` consumes the `orderReview` generator and refetches the projection when done; `onReviewDisposition` calls the existing disposition endpoint then refetches; `onAttestCitations` calls `attestGate(projectId,"draft_polish","citations_matched",confirmed)` then refetches. Thread all three through `ViewFrame` to `WritingView`.

- [ ] **Step 5: Run web suite + tsc — verify pass**

Run: `cd apps/web && npm test && npx tsc --noEmit -p apps/web`
Expected: PASS + clean.

- [ ] **Step 6: Commit**

```bash
git add apps/web/src/api/writing.ts apps/web/src/api/studioTurn.ts apps/web/src/studio/views/WritingView.tsx apps/web/src/studio/views/WritingView.test.tsx apps/web/src/studio/StudioContainer.tsx apps/web/src/studio/ViewFrame.tsx
git commit -m "feat(refactor2): S5 whole-draft review UI — work-order + three-key disposition + citations attest"
```

---

## Task 11: Suite gate + roadmap + orphan sweep

**Files:**
- Modify: `docs/2026-07-11-whole-product-refactor-roadmap.md`
- Verify only: full test suites.

- [ ] **Step 1: Full Go suite**

Run: `cd apps/api && CGO_ENABLED=0 go build ./... && CGO_ENABLED=0 go vet ./... && CGO_ENABLED=0 go test -p 1 ./...`
Expected: all packages `ok` / `[no test files]`; exit 0. (Quiet Docker daemon.)

- [ ] **Step 2: Full web + contracts + tsc**

Run: `cd apps/web && npm test && npx tsc --noEmit -p apps/web && cd ../../packages/contracts && npm test`
Expected: all green.

- [ ] **Step 3: Orphan / sync sweep**

Run: `cd apps/api && make -C .. sqlc && make -C .. sync-skills && git status --short apps/api/internal/store/sqlc apps/api/internal/skills/specs packages/contracts/skills`
Expected: no diff (generated code + synced mirror already committed).

- [ ] **Step 4: Roadmap entry** — mark Slice 8 keystone status and append a `### Slice 8 (keystone)` per-slice log section summarizing: the S5 loop live (buffer→snapshot→review→disposition→gate), `word_budget_ok` machine gate + `citations_matched` attestation + `whole_draft_review` (ordering the review), RL-1 structural, the disposition mapping + word-budget seed decisions, and the 8b/9/10 carry-forwards.

- [ ] **Step 5: Commit**

```bash
git add docs/2026-07-11-whole-product-refactor-roadmap.md
git commit -m "docs(refactor2): Slice 8 keystone complete — S5 writing surface + whole-draft review live"
```

---

## Notes for the executor

- **Test-harness helper names** (`newStudioAPI`, `doReq`, `seedProject`, `newTestQueries`) are placeholders — bind them to whatever `materials_test.go` / `studioturn_test.go` / `sqlc_test.go` already define. Do NOT invent a parallel harness.
- **Verify field/method names against the real code before running** each step: `gateway.ChatResponse` body field, `sqlc.InsertInterventionParams`, `RecordedGate` exported fields, `DraftSnapshot.CreatedAt` type (`pgtype.Timestamptz`). The plan names them as they appear in the explored code; a generated-name mismatch is a mechanical fix, not a design change.
- **The heartbeat goroutine** in Task 6 should be factored out of `studioturn.go`'s `postProjectTurn` into a shared `startHeartbeat(ctx, em) (stop chan, done chan)` helper and both call sites updated — do it as part of Task 6, not a separate refactor.
- **Do NOT** add SSE token streaming, examiner-voice switching, board-specific passes, or automated citation matching — all are out of scope (8b / 9 / 10).
