# S2 · Reading Sub-Agent Contract — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Formalize the (already context-isolated) reading room as the reusable brief-in / takeaways-out sub-agent: a purposeful brief at source entry, a per-source lifecycle, and a compact 5-field takeaway that lands on the reading-list entry and folds one line into the spine.

**Architecture:** Additive migration on `reference` (brief + takeaway + finalize timestamp + phase_tag). The brief is injected into the existing `RouteReading` loop so its questions become purposeful. On finalize, record fields (`findings/credibility/key_quotes`) are *assembled server-side* from the student's already-confirmed reading outcomes (reusing the `notesByMaterial` projection path, extended to read `framework_fill`); the two synthesis fields (`new_leads/proposal_impact`) are *student-authored*, AI-seeded via one mid-tier compose call. Everything mirrors the assessment sub-agent template (pure input digest → one isolated call → compact struct → meter+persist) and the S1 summary template (`workspace_summary.go`).

**Tech Stack:** Go (`net/http`, `pgx`, `sqlc`, `goose`), Postgres, React + Vite + TS, Zod contracts.

## Global Constraints

- **Client never calls a model.** All LLM calls go through the backend gateway; cost recorded (surface/purpose/tokens). — copied from AGENTS.md.
- **AI 克制 — never concludes for the student.** The takeaway records the student's own confirmed work verbatim; the compose call may only *rephrase/organize* the two synthesis suggestions, never add a conclusion she did not reach.
- **Card JSON single source of truth in registry; new card = new JSON, not renderer code.** (No new cards in S2 — do not touch the registry.)
- **Standard envelope structure is the shared floor.** Go validates the envelope; the Zod contract in `packages/contracts` owns deep structure (here: the `phase_tag` enum and the `ReadingTakeaway` shape).
- **Migrations:** goose, numbered, in `apps/api/internal/store/migrations/`; the next number is **0039** (0038 was S1). `make sqlc` requires `CGO_ENABLED=0` on macOS. Prod migrates ONLY via the one-shot `run --rm api -migrate-up` (bare `/api` does NOT auto-migrate).
- **Keys/DSN only server-side.** Never in git, logs, errors, or payloads.
- **phase_tag enum (contracts-owned, no DB CHECK):** `立题探索 · 背景理解 · 支持论点 · 反例检验 · 方法参考`.
- **Deferred to S3 (do NOT build):** rabbit-hole exploration tree, `branch` links, `new_leads` as a graph. S2 stores `new_leads` as data only.

---

## File Structure

**Backend (`apps/api`):**
- `internal/store/migrations/0039_reading_takeaway.sql` — **create**: additive columns on `reference`.
- `internal/store/queries/workspace.sql` — **modify**: `UpdateReadingBrief`, `FinalizeReadingTakeaway`, `GetReferenceForProject` queries.
- `internal/store/sqlc/*` — **regenerated** by `make sqlc`.
- `internal/agent/reading_takeaway.go` — **create**: `ReadingTakeaway`/`TakeawayRecord` structs, `ComposeReadingTakeawaySuggestions` (the isolated mid-tier compose core), input struct.
- `internal/agent/reading_router.go` — **modify**: add `Brief` to `ReadingRouteInput`; use it in the prompt.
- `internal/api/reading_brief.go` — **create**: `putReadingBrief` handler + `suggestReadingReason` templating; `readingBriefFor` loader.
- `internal/api/reading_takeaway.go` — **create**: `readingOutcomesByMaterial` (assembly), `getTakeawayDraft`, `postFinalizeReading` handlers.
- `internal/api/workspace_library.go` — **modify**: `enterReading` returns `suggestedReason`; `toReferenceDTO` carries `phaseTag`/`takeaway`.
- `internal/api/readturn.go` — **modify**: fill `ReadingRouteInput.Brief` from the reference + proposal.
- `internal/api/projectcoach.go` — **modify**: the 文献库 block projects 在读/已归纳 lines.
- `internal/api/api.go` — **modify**: register 3 routes.
- Tests: `internal/api/reading_takeaway_test.go`, `internal/agent/reading_takeaway_test.go`, `internal/api/reading_brief_test.go`.

**Contracts (`packages/contracts`):**
- `src/reference.ts` — **modify**: `PhaseTag`, `ReadingTakeaway`, extend `Reference`.
- `src/reading.ts` (or extend existing) — **create/modify**: `ReadingBrief`, `TakeawayDraft`.

**Frontend (`apps/web`):**
- `src/workspace/api/workspace.ts` — **modify**: `putReadingBrief`, `getTakeawayDraft`, `postFinalizeReading` + types.
- `src/studio/reading/ReadingRoom.tsx` — **modify**: brief banner + 完成这篇 finalize panel.
- `src/studio/reading/readingLoop.ts` — **modify**: extend `ReadingLoopApi` with the 3 new calls; expose brief/finalize state.
- `src/workspace/blocks/ReadingBlock.tsx` — **modify**: phase_tag chip + 已归纳 state + structured takeaway in Preview.

---

## Task 1: Migration 0039 + reference queries

**Files:**
- Create: `apps/api/internal/store/migrations/0039_reading_takeaway.sql`
- Modify: `apps/api/internal/store/queries/workspace.sql`
- Test: `apps/api/internal/api/reading_takeaway_test.go` (round-trip via queries)

**Interfaces:**
- Produces (sqlc-generated after `make sqlc`): `sqlc.Reference` gains `ReadingReason *string`, `ReadingFocus *string`, `PhaseTag *string`, `Takeaway []byte` (jsonb), `TakeawayFinalizedAt pgtype.Timestamptz`. New query methods `Queries.UpdateReadingBrief(ctx, UpdateReadingBriefParams) (sqlc.Reference, error)`, `Queries.FinalizeReadingTakeaway(ctx, FinalizeReadingTakeawayParams) (sqlc.Reference, error)`, `Queries.GetReferenceForProject(ctx, GetReferenceForProjectParams) (sqlc.Reference, error)`.

- [ ] **Step 1: Write the migration**

Create `apps/api/internal/store/migrations/0039_reading_takeaway.sql`:

```sql
-- +goose Up
-- S2 · reading sub-agent contract. The reading room already runs isolated on a
-- material's blocks; these columns give it the two missing halves of the §6
-- contract — a durable brief-in (why read THIS source) and a compact
-- takeaways-out object — plus a phase_tag (which project moment the source
-- serves, feeds S3's rabbit-hole). All additive; NULL = legacy/not-yet.
ALTER TABLE reference ADD COLUMN reading_reason        text;
ALTER TABLE reference ADD COLUMN reading_focus         text;
ALTER TABLE reference ADD COLUMN phase_tag             text;
ALTER TABLE reference ADD COLUMN takeaway              jsonb;
ALTER TABLE reference ADD COLUMN takeaway_finalized_at timestamptz;

-- +goose Down
ALTER TABLE reference DROP COLUMN reading_reason;
ALTER TABLE reference DROP COLUMN reading_focus;
ALTER TABLE reference DROP COLUMN phase_tag;
ALTER TABLE reference DROP COLUMN takeaway;
ALTER TABLE reference DROP COLUMN takeaway_finalized_at;
```

- [ ] **Step 2: Add the queries**

Append to `apps/api/internal/store/queries/workspace.sql` (near the other `reference` queries). `ListReferences`/`CreateReference` use `SELECT *`/`RETURNING *`, so they pick up the new columns automatically — only these three are new:

```sql
-- name: GetReferenceForProject :one
-- Scope a reference id to its project (the IDOR guard for the reading-brief /
-- takeaway endpoints, mirroring patchReference's ownership scoping).
SELECT * FROM reference WHERE id = $1 AND project_id = $2;

-- name: UpdateReadingBrief :one
-- Brief-in: persist why-read-this + optional focus + phase_tag on the source.
-- Editable any time; does not touch the takeaway.
UPDATE reference
SET reading_reason = $3, reading_focus = $4, phase_tag = $5, updated_at = now()
WHERE id = $1 AND project_id = $2
RETURNING *;

-- name: FinalizeReadingTakeaway :one
-- Takeaways-out: land the compact 5-field object and stamp finalized_at.
-- UPDATE (not insert) — re-finalize on a re-read SUPERSEDES (§1, deliberately
-- unlike S1's first-open-wins proposal prose; reading is iterative).
UPDATE reference
SET takeaway = $3, takeaway_finalized_at = now(), updated_at = now()
WHERE id = $1 AND project_id = $2
RETURNING *;
```

- [ ] **Step 3: Regenerate sqlc**

Run: `cd apps/api && CGO_ENABLED=0 make sqlc`
Expected: no error; `git status` shows `internal/store/sqlc/*` regenerated with the new `Reference` fields and three new methods.

- [ ] **Step 4: Write the round-trip test**

Create `apps/api/internal/api/reading_takeaway_test.go` (uses the existing `newAPITestPool`/`signInSeed`/`seedProjectID` harness — see `coach_continuity_test.go`):

```go
package api

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"mindimprint/api/internal/store/sqlc"
)

func TestReadingBriefAndTakeawayRoundTrip(t *testing.T) {
	pool := newAPITestPool(t)
	q := sqlc.New(pool)
	ctx := context.Background()
	projectID := mustUUID(t, seedProjectID)

	// Seed a reference in the project.
	ref, err := q.CreateReference(ctx, sqlc.CreateReferenceParams{
		ProjectID: projectID, Title: "NASA 报告",
	})
	if err != nil {
		t.Fatalf("CreateReference: %v", err)
	}

	reason, focus, phase := "验证碳排放是否构成反例", "看它引用了谁", "反例检验"
	updated, err := q.UpdateReadingBrief(ctx, sqlc.UpdateReadingBriefParams{
		ID: ref.ID, ProjectID: projectID,
		ReadingReason: &reason, ReadingFocus: &focus, PhaseTag: &phase,
	})
	if err != nil {
		t.Fatalf("UpdateReadingBrief: %v", err)
	}
	if updated.ReadingReason == nil || *updated.ReadingReason != reason {
		t.Fatalf("reading_reason not persisted: %+v", updated.ReadingReason)
	}
	if updated.TakeawayFinalizedAt.Valid {
		t.Fatalf("finalized_at should be null before finalize")
	}

	obj := map[string]any{"proposal_impact": "把它作为让步段的证据"}
	raw, _ := json.Marshal(obj)
	fin, err := q.FinalizeReadingTakeaway(ctx, sqlc.FinalizeReadingTakeawayParams{
		ID: ref.ID, ProjectID: projectID, Takeaway: raw,
	})
	if err != nil {
		t.Fatalf("FinalizeReadingTakeaway: %v", err)
	}
	if !fin.TakeawayFinalizedAt.Valid {
		t.Fatalf("finalized_at should be set after finalize")
	}
	var got map[string]any
	if err := json.Unmarshal(fin.Takeaway, &got); err != nil || got["proposal_impact"] != obj["proposal_impact"] {
		t.Fatalf("takeaway round-trip mismatch: %v / %v", err, got)
	}

	// Scope guard: wrong project id must not find it.
	if _, err := q.GetReferenceForProject(ctx, sqlc.GetReferenceForProjectParams{
		ID: ref.ID, ProjectID: uuid.New(),
	}); err == nil {
		t.Fatalf("GetReferenceForProject should 404 across projects")
	}
}
```

> Note: check `CreateReferenceParams`' exact field set from the regenerated sqlc — if `CreateReference` requires more non-null params, fill them; the harness pattern is in `coach_continuity_test.go`.

- [ ] **Step 5: Run the test (Docker required for testcontainers)**

Run: `cd apps/api && go test ./internal/api/ -run TestReadingBriefAndTakeawayRoundTrip -v`
Expected: PASS. (If it fails to compile because a sqlc field name differs, align the test to the generated names — do not rename generated code.)

- [ ] **Step 6: Commit**

```bash
git add apps/api/internal/store/migrations/0039_reading_takeaway.sql apps/api/internal/store/queries/workspace.sql apps/api/internal/store/sqlc apps/api/internal/api/reading_takeaway_test.go
git commit -m "feat(s2): migration 0039 + reference brief/takeaway queries"
```

---

## Task 2: Brief-in capture — `PUT reading-brief` + suggested reason

**Files:**
- Create: `apps/api/internal/api/reading_brief.go`
- Modify: `apps/api/internal/api/workspace_library.go` (`enterReading` returns `suggestedReason`), `apps/api/internal/api/api.go` (route)
- Test: `apps/api/internal/api/reading_brief_test.go`

**Interfaces:**
- Consumes: `Queries.UpdateReadingBrief`, `Queries.GetReferenceForProject`, `Queries.GetProjectProposal` (existing), `a.loadOwnedProject` (existing).
- Produces: `func (a *API) putReadingBrief(w, r)`; `func suggestReadingReason(proposalObjective, refTitle string) string`; `func (a *API) readingBriefFor(ctx, projectID, materialID uuid.UUID) agent.ReadingBrief` (used by Task 3). Route `PUT /projects/{id}/references/{rid}/reading-brief`.

- [ ] **Step 1: Write the failing handler test**

Create `apps/api/internal/api/reading_brief_test.go`:

```go
package api

import (
	"net/http"
	"strings"
	"testing"
)

func TestPutReadingBrief_Persists(t *testing.T) {
	h, pool := planTestHandler(t)          // same helper S1's coach tests use
	token := signInSeed(t, h)
	q := newQueriesForPool(t, pool)        // thin helper; or sqlc.New(pool)
	ref := seedReference(t, q, "NASA 报告") // helper: CreateReference in seedProjectID

	body := `{"reading_reason":"验证碳排放反例","reading_focus":"看引用来源","phase_tag":"反例检验"}`
	rr := doJSON(t, h, http.MethodPut,
		"/api/v1/projects/"+seedProjectID+"/references/"+ref.ID.String()+"/reading-brief",
		token, body)
	if rr.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "反例检验") {
		t.Fatalf("response should echo the brief: %s", rr.Body.String())
	}
}

func TestSuggestReadingReason_Templated(t *testing.T) {
	got := suggestReadingReason("研究中国是否让地球更可持续", "NASA 全球碳排放报告")
	if got == "" || !strings.Contains(got, "NASA") {
		t.Fatalf("suggested reason should reference the source title: %q", got)
	}
}
```

> Use the request helpers already in the api test package (`doJSON`/`signInSeed`/`planTestHandler`). If `doJSON`/`seedReference`/`newQueriesForPool` don't exist verbatim, reuse the exact helpers `coach_continuity_test.go` uses and add a 3-line `seedReference` helper there.

- [ ] **Step 2: Run to verify it fails**

Run: `cd apps/api && go test ./internal/api/ -run 'TestPutReadingBrief_Persists|TestSuggestReadingReason' -v`
Expected: FAIL (undefined `putReadingBrief` / `suggestReadingReason`).

- [ ] **Step 3: Implement the handler + templating**

Create `apps/api/internal/api/reading_brief.go`:

```go
package api

import (
	"net/http"
	"strings"

	"github.com/google/uuid"

	"mindimprint/api/internal/agent"
	"mindimprint/api/internal/httpx"
)

// reading_brief.go — S2 · brief-in. Why-read-THIS-source is captured at entry
// (pre-filled by suggestReadingReason, student edits), persisted on the
// reference, and injected into every read-turn so the coach's questions are
// purposeful. 克制: the AI only seeds the default; the student authors intent.

type readingBriefReq struct {
	ReadingReason string `json:"reading_reason"`
	ReadingFocus  string `json:"reading_focus"`
	PhaseTag      string `json:"phase_tag"`
}

// suggestReadingReason templates a first-draft intention deterministically (no
// model call) from the proposal objective + the source title.
func suggestReadingReason(proposalObjective, refTitle string) string {
	title := strings.TrimSpace(refTitle)
	if title == "" {
		title = "这篇材料"
	}
	obj := strings.TrimSpace(proposalObjective)
	if obj == "" {
		return "读《" + title + "》想弄清什么？"
	}
	return "带着「" + obj + "」的问题读《" + title + "》，我想验证："
}

func (a *API) putReadingBrief(w http.ResponseWriter, r *http.Request) {
	projectID, ok := a.loadOwnedProject(w, r)
	if !ok {
		return
	}
	rid, err := uuid.Parse(httpx.URLParam(r, "rid"))
	if err != nil {
		httpx.WriteError(w, r, httpx.ErrNotFound("资源不存在"))
		return
	}
	var body readingBriefReq
	if err := decodeJSON(r, &body); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	reason, focus, phase := body.ReadingReason, body.ReadingFocus, body.PhaseTag
	row, err := a.d.Queries.UpdateReadingBrief(r.Context(), sqlc.UpdateReadingBriefParams{
		ID: rid, ProjectID: projectID,
		ReadingReason: &reason, ReadingFocus: &focus, PhaseTag: &phase,
	})
	if err != nil {
		httpx.WriteError(w, r, httpx.ErrNotFound("资源不存在"))
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"reference": toReferenceDTO(row, nil)})
}

// readingBriefFor loads the persisted brief for a material's reference (for
// read-turn injection, Task 3). Best-effort: a material with no reference or an
// unset brief returns a zero ReadingBrief and the loop degrades to today.
func (a *API) readingBriefFor(ctx context.Context, projectID, materialID uuid.UUID) agent.ReadingBrief {
	// (implemented in Task 3, where ReadingBrief is consumed)
	return agent.ReadingBrief{}
}
```

> `httpx.URLParam` — confirm the exact router param accessor used elsewhere in `workspace_library.go` (`patchReference` reads `rid` the same way); match it. Add the `sqlc` import.

Modify `enterReading` in `workspace_library.go` to include the suggestion in its JSON response. Find where it writes the response and add:

```go
// suggestedReason: a deterministic first-draft intention (student edits). No spend.
prop, _ := a.d.Queries.GetProjectProposal(r.Context(), projectID) // pgx.ErrNoRows tolerated
suggested := suggestReadingReason(prop.Objective, ref.Title)      // ref = the reference row already loaded here
// add "suggestedReason": suggested to the existing response map
```

- [ ] **Step 4: Register the route**

In `apps/api/internal/api/api.go`, next to the other `references/{rid}` routes:

```go
r.Put("/projects/{id}/references/{rid}/reading-brief", a.putReadingBrief)
```

(Match the actual router mounting style — chi `r.Route`/`r.Put` as used by the neighboring `enter-reading` route.)

- [ ] **Step 5: Run tests to verify pass**

Run: `cd apps/api && go test ./internal/api/ -run 'TestPutReadingBrief_Persists|TestSuggestReadingReason' -v`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add apps/api/internal/api/reading_brief.go apps/api/internal/api/workspace_library.go apps/api/internal/api/api.go apps/api/internal/api/reading_brief_test.go
git commit -m "feat(s2): reading brief-in endpoint + suggested reason"
```

---

## Task 3: Inject the brief into the read-turn

**Files:**
- Modify: `apps/api/internal/agent/reading_router.go` (add `Brief` to `ReadingRouteInput`; use in prompt), `apps/api/internal/api/readturn.go` (fill it), `apps/api/internal/api/reading_brief.go` (finish `readingBriefFor`)
- Test: `apps/api/internal/agent/reading_router_brief_test.go`

**Interfaces:**
- Produces: `agent.ReadingBrief` struct; `ReadingRouteInput.Brief agent.ReadingBrief`. Consumed by `readturn.go` and `readingBriefFor`.

- [ ] **Step 1: Define the struct + failing prompt test**

Create `apps/api/internal/agent/reading_router_brief_test.go`:

```go
package agent

import (
	"strings"
	"testing"
)

func TestReadingBrief_ReachesPrompt(t *testing.T) {
	in := ReadingRouteInput{
		StudentText: "这段说碳排放很高",
		Article:     "……全球碳排放……",
		Brief: ReadingBrief{
			Reason:       "验证碳排放是否构成反例",
			PhaseTag:     "反例检验",
			ProposalSnap: "论点：中国推动可持续",
		},
	}
	prompt := buildReadingRouteUserPrompt(in) // the pure prompt assembler
	if !strings.Contains(prompt, "验证碳排放是否构成反例") || !strings.Contains(prompt, "反例检验") {
		t.Fatalf("brief must appear in the router prompt:\n%s", prompt)
	}
}
```

> If the router assembles its user message inline inside `RouteReading` rather than via a named helper, extract the assembly into `buildReadingRouteUserPrompt(ReadingRouteInput) string` first (pure, no I/O) and call it from `RouteReading` — that refactor is part of this step and makes the behavior testable.

- [ ] **Step 2: Run to verify it fails**

Run: `cd apps/api && go test ./internal/agent/ -run TestReadingBrief_ReachesPrompt -v`
Expected: FAIL (`ReadingBrief` undefined / brief text absent).

- [ ] **Step 3: Implement**

In `reading_router.go`, add the struct and field, and weave it into the prompt:

```go
// ReadingBrief is the sub-agent's brief-in (§6): why the student is reading
// THIS source, so the coach's questions serve a purpose instead of reading
// in the abstract. Empty fields degrade the loop to its prior generic behavior.
type ReadingBrief struct {
	Reason       string // reference.reading_reason — why THIS source, now
	Focus        string // reference.reading_focus — optional student points
	PhaseTag     string // reference.phase_tag — which project moment it serves
	ProposalSnap string // relevant proposal dims, one line
}

type ReadingRouteInput struct {
	StudentText    string
	Article        string
	FocusedSpans   []FocusSpan
	RecentTurns    []string
	Catalog        []ReadingCard
	ScaffoldLevels map[string]int
	Pacing         PacingState
	Brief          ReadingBrief // S2 · brief-in
}
```

In `buildReadingRouteUserPrompt` (extracted), prepend a compact brief block when present:

```go
if b := in.Brief; strings.TrimSpace(b.Reason+b.Focus+b.PhaseTag+b.ProposalSnap) != "" {
	fmt.Fprintf(&sb, "【阅读目的】\n")
	if b.Reason != "" {
		fmt.Fprintf(&sb, "- 为什么读这篇：%s\n", b.Reason)
	}
	if b.PhaseTag != "" {
		fmt.Fprintf(&sb, "- 服务于：%s\n", b.PhaseTag)
	}
	if b.ProposalSnap != "" {
		fmt.Fprintf(&sb, "- 当前论点：%s\n", b.ProposalSnap)
	}
	if b.Focus != "" {
		fmt.Fprintf(&sb, "- 学生关注：%s\n", b.Focus)
	}
	sb.WriteString("让你的追问围绕这个目的，而不是泛泛而读。\n\n")
}
```

Also extend the router **system** prompt with one sentence: `如果给了「阅读目的」，你的每一次追问都要服务这个目的（是在找反驳、印证，还是背景）。`

Finish `readingBriefFor` in `reading_brief.go`:

```go
func (a *API) readingBriefFor(ctx context.Context, projectID, materialID uuid.UUID) agent.ReadingBrief {
	var br agent.ReadingBrief
	// reference carries the brief; find it by material_id within the project.
	if refs, err := a.d.Queries.ListReferences(ctx, projectID); err == nil {
		for _, ref := range refs {
			if ref.MaterialID.Valid && uuid.UUID(ref.MaterialID.Bytes) == materialID {
				if ref.ReadingReason != nil {
					br.Reason = *ref.ReadingReason
				}
				if ref.ReadingFocus != nil {
					br.Focus = *ref.ReadingFocus
				}
				if ref.PhaseTag != nil {
					br.PhaseTag = *ref.PhaseTag
				}
				break
			}
		}
	}
	if prop, err := a.d.Queries.GetProjectProposal(ctx, projectID); err == nil {
		br.ProposalSnap = firstNonEmpty(prop.Objective, prop.Reason)
	}
	return br
}
```

(Add a tiny `firstNonEmpty(...string) string` helper if none exists.) In `readturn.go` where `ReadingRouteInput` is built (~line 237), set `Brief: a.readingBriefFor(r.Context(), projectID, materialID)`.

- [ ] **Step 4: Run tests**

Run: `cd apps/api && go test ./internal/agent/ -run TestReadingBrief_ReachesPrompt -v && go test ./internal/api/ -run TestReadingTurn -v`
Expected: PASS (and the existing read-turn tests still green).

- [ ] **Step 5: Commit**

```bash
git add apps/api/internal/agent/reading_router.go apps/api/internal/agent/reading_router_brief_test.go apps/api/internal/api/readturn.go apps/api/internal/api/reading_brief.go
git commit -m "feat(s2): inject reading brief into the read-turn router"
```

---

## Task 4: Takeaway assembly + isolated compose core

**Files:**
- Create: `apps/api/internal/agent/reading_takeaway.go`
- Modify: `apps/api/internal/api/reading_takeaway.go` (add `readingOutcomesByMaterial`)
- Test: `apps/api/internal/agent/reading_takeaway_test.go`

**Interfaces:**
- Produces:
  - `agent.TakeawayRecord{ Findings []string; Credibility agent.Credibility; KeyQuotes []agent.KeyQuote }` where `Credibility{Verdict, Why string}`, `KeyQuote{Quote, Why string}`.
  - `agent.ReadingTakeaway{ Findings []string; Credibility agent.Credibility; KeyQuotes []agent.KeyQuote; NewLeads []string; ProposalImpact string }`.
  - `agent.ComposeReadingTakeawaySuggestions(ctx, prov, r, ReadingTakeawayInput) (newLeads []string, proposalImpact string, usage gateway.ChatUsage, err error)` — the isolated mid-tier compose that SEEDS only the two synthesis fields.
  - `agent.ReadingTakeawayInput{ Brief ReadingBrief; Record TakeawayRecord }`.
  - `func (a *API) readingOutcomesByMaterial(r, projectID, materialID) TakeawayRecord` — assembles record fields from confirmed reading cards (reuses the `notesByMaterial` idea, extended to read `framework_fill`).

- [ ] **Step 1: Failing test for compose 克制 + input shape**

Create `apps/api/internal/agent/reading_takeaway_test.go`:

```go
package agent

import (
	"context"
	"strings"
	"testing"

	"mindimprint/api/internal/gateway"
)

func TestComposeReadingTakeaway_SeedsOnlySynthesis(t *testing.T) {
	prov := &fakeProvider{reply: `{"new_leads":["中国人均排放 vs 总量的口径"],"proposal_impact":"作为让步段的反例证据"}`}
	in := ReadingTakeawayInput{
		Brief: ReadingBrief{Reason: "验证反例", PhaseTag: "反例检验"},
		Record: TakeawayRecord{
			Findings:    []string{"中国碳排放总量全球第一"},
			Credibility: Credibility{Verdict: "strong", Why: "NASA 一手数据"},
			KeyQuotes:   []KeyQuote{{Quote: "China emits the most", Why: "直接反例"}},
		},
	}
	leads, impact, _, err := ComposeReadingTakeawaySuggestions(context.Background(), prov, gateway.Resolved{Provider: "fake", Model: "m"}, in)
	if err != nil {
		t.Fatalf("compose: %v", err)
	}
	if len(leads) == 0 || impact == "" {
		t.Fatalf("expected seeded synthesis, got leads=%v impact=%q", leads, impact)
	}
}

func TestComposeReadingTakeaway_EmptyRecordErrors(t *testing.T) {
	prov := &fakeProvider{reply: `{}`}
	_, _, _, err := ComposeReadingTakeawaySuggestions(context.Background(), prov,
		gateway.Resolved{Provider: "fake", Model: "m"}, ReadingTakeawayInput{})
	if err == nil {
		t.Fatalf("empty record (nothing to organize) should error, not fabricate")
	}
}
```

> `fakeProvider` is the existing agent-test double (used by `project_coach_test.go`/`project_summary` tests). Match its constructor.

- [ ] **Step 2: Run to verify it fails**

Run: `cd apps/api && go test ./internal/agent/ -run TestComposeReadingTakeaway -v`
Expected: FAIL (undefined types/func).

- [ ] **Step 3: Implement the agent core**

Create `apps/api/internal/agent/reading_takeaway.go`:

```go
package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"mindimprint/api/internal/gateway"
)

// reading_takeaway.go — S2 · the reading sub-agent's "return". Record fields
// (findings/credibility/key_quotes) are the student's ALREADY-confirmed work,
// assembled by the caller — never re-guessed here. This file only SEEDS the two
// synthesis fields (new_leads/proposal_impact) with ONE isolated mid-tier call,
// constrained to organize the student's own material, never to conclude for her
// (铁律 · 克制). Mirrors the assessment template: pure input → one call → struct.

type Credibility struct {
	Verdict string `json:"verdict"`
	Why     string `json:"why"`
}

type KeyQuote struct {
	Quote string `json:"quote"`
	Why   string `json:"why"`
}

// TakeawayRecord is the assembled, deterministic half — the student's confirmed work.
type TakeawayRecord struct {
	Findings    []string   `json:"findings"`
	Credibility Credibility `json:"credibility"`
	KeyQuotes   []KeyQuote `json:"key_quotes"`
}

// ReadingTakeaway is the full 5-field object written to reference.takeaway.
type ReadingTakeaway struct {
	Findings       []string   `json:"findings"`
	Credibility    Credibility `json:"credibility"`
	KeyQuotes      []KeyQuote `json:"key_quotes"`
	NewLeads       []string   `json:"new_leads"`
	ProposalImpact string     `json:"proposal_impact"`
}

type ReadingTakeawayInput struct {
	Brief  ReadingBrief
	Record TakeawayRecord
}

const readingTakeawaySystem = `你是「思维印记」的陪读助手。学生刚读完一篇材料，下面是她自己已经确认的发现、可信度判断和关键引用。请只做两件事，且只能基于她已有的材料，绝不替她下新结论：
1) new_leads：这篇material还遗留、或新引出的、值得继续追的问题（0-3条，每条一句）。
2) proposal_impact：这篇如何影响她的论点/论证（一句话，用她发现里已有的东西，不新增立场）。
只输出 JSON：{"new_leads":[...],"proposal_impact":"..."}。这些是给学生的草稿建议，她会改写。`

func hasRecordContent(rec TakeawayRecord) bool {
	return len(rec.Findings) > 0 || len(rec.KeyQuotes) > 0 || strings.TrimSpace(rec.Credibility.Verdict) != ""
}

func ComposeReadingTakeawaySuggestions(ctx context.Context, prov gateway.Provider, r gateway.Resolved, in ReadingTakeawayInput) ([]string, string, gateway.ChatUsage, error) {
	if !hasRecordContent(in.Record) {
		return nil, "", gateway.ChatUsage{}, fmt.Errorf("agent: empty reading record — nothing to organize")
	}
	recJSON, _ := json.Marshal(in.Record)
	user := "阅读目的：" + in.Brief.Reason + "（" + in.Brief.PhaseTag + "）\n她已确认的内容：\n" + string(recJSON)
	res, err := gateway.Collect(ctx, prov, r, gateway.ChatRequest{
		Messages: []gateway.ChatMessage{
			{Role: gateway.RoleSystem, Content: readingTakeawaySystem},
			{Role: gateway.RoleUser, Content: user},
		},
	})
	if err != nil {
		return nil, "", gateway.ChatUsage{}, err
	}
	var out struct {
		NewLeads       []string `json:"new_leads"`
		ProposalImpact string   `json:"proposal_impact"`
	}
	if err := json.Unmarshal([]byte(extractJSON(res.Text)), &out); err != nil {
		return nil, "", res.Usage, fmt.Errorf("agent: takeaway suggestions parse: %w", err)
	}
	return out.NewLeads, strings.TrimSpace(out.ProposalImpact), res.Usage, nil
}
```

> `extractJSON` — reuse the existing tolerant JSON-extraction helper the other agent cores use (`reading_eval.go`/`assess_report.go` parse model JSON the same way); if it's named differently, call that. `gateway.Provider` is the interface type `Collect` takes (see `ComposeReturnSummary`'s signature which takes `prov gateway.Provider`).

- [ ] **Step 4: Implement the assembly helper**

Add to `apps/api/internal/api/reading_takeaway.go` (created in Task 6; if doing Task 4 first, create the file now with just this function + package):

```go
// readingOutcomesByMaterial assembles the record half of the takeaway from the
// student's CONFIRMED reading cards for this material — the same source
// notesByMaterial projects, but read through framework_fill (the persisted
// selectionEvalDTO) so we get the verdict + judgment, not just quote+finding.
// Credibility is taken from the CRAAP/SIFT card's verdict when present.
func (a *API) readingOutcomesByMaterial(r *http.Request, projectID, materialID uuid.UUID) agent.TakeawayRecord {
	var rec agent.TakeawayRecord
	cards, err := a.d.Queries.ListCardInstancesByProject(r.Context(), projectID)
	if err != nil {
		return rec
	}
	for _, ci := range cards {
		if ci.Status != "completed" {
			continue
		}
		// material attribution lives in the anchors (card_instances has no material_id).
		var anchors []agent.Anchor
		if json.Unmarshal(ci.Anchors, &anchors) != nil || len(anchors) == 0 {
			continue
		}
		if anchors[0].MaterialID != materialID.String() {
			continue
		}
		var ev selectionEvalDTO
		if len(ci.FrameworkFill) > 0 {
			_ = json.Unmarshal(ci.FrameworkFill, &ev)
		}
		if ev.Finding != "" {
			rec.Findings = append(rec.Findings, ev.Finding)
		}
		for _, an := range anchors {
			if an.Author == "student" && an.Quote != "" {
				rec.KeyQuotes = append(rec.KeyQuotes, agent.KeyQuote{Quote: an.Quote, Why: ev.Judgment})
			}
		}
		// First CRAAP/SIFT verdict seen sets credibility.
		if rec.Credibility.Verdict == "" && (ci.CardID == "craap" || ci.CardID == "sift") && ev.VerdictLabel != "" {
			rec.Credibility = agent.Credibility{Verdict: ev.VerdictLabel, Why: ev.VerdictReason}
		}
	}
	return rec
}
```

> Confirm the sqlc field name for `framework_fill` on the card instance row (likely `FrameworkFill []byte`) and `Anchors []byte`; align. `agent.Anchor` fields (`MaterialID`, `Quote`, `Author`) are used exactly as `readeval.go` uses them.

- [ ] **Step 5: Run tests**

Run: `cd apps/api && go test ./internal/agent/ -run TestComposeReadingTakeaway -v`
Expected: PASS. (Assembly helper is covered by Task 5's handler test.)

- [ ] **Step 6: Commit**

```bash
git add apps/api/internal/agent/reading_takeaway.go apps/api/internal/agent/reading_takeaway_test.go apps/api/internal/api/reading_takeaway.go
git commit -m "feat(s2): reading takeaway assembly + isolated compose core"
```

---

## Task 5: `GET takeaway-draft` — assemble + seed

**Files:**
- Modify: `apps/api/internal/api/reading_takeaway.go` (add `getTakeawayDraft`), `apps/api/internal/api/api.go` (route)
- Test: `apps/api/internal/api/reading_takeaway_test.go` (add handler test)

**Interfaces:**
- Consumes: `readingOutcomesByMaterial`, `agent.ComposeReadingTakeawaySuggestions`, `a.d.ChatResolver`, `agent.NewSqlcAgentStore(...).RecordLLMCall`.
- Produces: `func (a *API) getTakeawayDraft(w, r)`; route `GET /projects/{id}/references/{rid}/takeaway-draft`. Response: `{ record: {findings, credibility, keyQuotes}, suggestedNewLeads: [...], suggestedProposalImpact: "..." }`.

- [ ] **Step 1: Failing handler test**

Add to `reading_takeaway_test.go`:

```go
func TestGetTakeawayDraft_AssemblesAndSeeds(t *testing.T) {
	h, pool := planTestHandler(t)
	token := signInSeed(t, h)
	// Seed a material + a completed reading card with a student anchor + framework_fill.
	ref, mid := seedReadingOutcome(t, pool, "China emits the most", "中国碳排放全球第一") // helper below

	rr := doGet(t, h,
		"/api/v1/projects/"+seedProjectID+"/references/"+ref.String()+"/takeaway-draft", token)
	if rr.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "中国碳排放全球第一") {
		t.Fatalf("draft should assemble the confirmed finding: %s", rr.Body.String())
	}
	// The fake provider seeds synthesis; assert the draft spent exactly one call.
	if n := countLLMCallsByPurpose(t, pool, "reading_takeaway_draft"); n != 1 {
		t.Fatalf("want 1 draft llm_call, got %d", n)
	}
	_ = mid
}
```

> `seedReadingOutcome` helper: insert a `material` (with `source_url`), a `reference` linked to it, and a `card_instances` row with `status='completed'`, `card_id='craap'`, `anchors=[{material_id, quote, author:"student"}]`, `framework_fill={finding, verdictLabel, verdictReason, judgment}`. Model it on `reading_endpoints_test.go`'s existing seeding. `countLLMCallsByPurpose` already exists (S1 tests).

- [ ] **Step 2: Run to verify it fails**

Run: `cd apps/api && go test ./internal/api/ -run TestGetTakeawayDraft -v`
Expected: FAIL (undefined `getTakeawayDraft`).

- [ ] **Step 3: Implement**

Add to `reading_takeaway.go`. The handler must resolve the reference's material, assemble the record, run ONE mid-tier compose, meter, and return — no persist:

```go
func (a *API) getTakeawayDraft(w http.ResponseWriter, r *http.Request) {
	projectID, ok := a.loadOwnedProject(w, r)
	if !ok {
		return
	}
	u, _ := UserFromContext(r.Context())
	if entitled, err := HasEntitlement(r.Context(), u); err != nil {
		httpx.WriteError(w, r, err)
		return
	} else if !entitled {
		httpx.WriteError(w, r, httpx.ErrNotEntitled())
		return
	}
	rid, err := uuid.Parse(httpx.URLParam(r, "rid"))
	if err != nil {
		httpx.WriteError(w, r, httpx.ErrNotFound("资源不存在"))
		return
	}
	ref, err := a.d.Queries.GetReferenceForProject(r.Context(), sqlc.GetReferenceForProjectParams{ID: rid, ProjectID: projectID})
	if err != nil || !ref.MaterialID.Valid {
		httpx.WriteError(w, r, httpx.ErrNotFound("尚未进入阅读室"))
		return
	}
	materialID := uuid.UUID(ref.MaterialID.Bytes)
	record := a.readingOutcomesByMaterial(r, projectID, materialID)

	in := agent.ReadingTakeawayInput{Brief: a.readingBriefFor(r.Context(), projectID, materialID), Record: record}
	var leads []string
	var impact string
	if resolved, rerr := a.d.ChatResolver(r.Context()); rerr == nil {
		l, imp, usage, cerr := agent.ComposeReadingTakeawaySuggestions(r.Context(), a.d.Provider, resolved, in)
		if resolved.Provider != "" {
			store := agent.NewSqlcAgentStore(a.d.Queries, a.d.Pool)
			if e := store.RecordLLMCall(r.Context(), agent.LLMCallRow{
				ProjectID: projectID, Surface: "studio", Purpose: "reading_takeaway_draft",
				Resolved: resolved, PromptTokens: int32(usage.InputTokens), CompletionTokens: int32(usage.OutputTokens),
			}); e != nil {
				slog.Warn("takeaway draft: record llm", "err", e)
			}
		}
		if cerr == nil {
			leads, impact = l, imp
		}
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{
		"record":                  toTakeawayRecordDTO(record),
		"suggestedNewLeads":       nonNilStrings(leads),
		"suggestedProposalImpact": impact,
	})
}
```

Add small DTO helpers (`toTakeawayRecordDTO` → camelCase `{findings, credibility:{verdict,why}, keyQuotes:[{quote,why}]}`; `nonNilStrings`). Register route in `api.go`: `r.Get("/projects/{id}/references/{rid}/takeaway-draft", a.getTakeawayDraft)`.

- [ ] **Step 4: Run tests**

Run: `cd apps/api && go test ./internal/api/ -run 'TestGetTakeawayDraft|TestReadingBriefAndTakeawayRoundTrip' -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add apps/api/internal/api/reading_takeaway.go apps/api/internal/api/api.go apps/api/internal/api/reading_takeaway_test.go
git commit -m "feat(s2): takeaway-draft endpoint (assemble + seed synthesis)"
```

---

## Task 6: `POST finalize-reading` — persist + supersede, no spend

**Files:**
- Modify: `apps/api/internal/api/reading_takeaway.go` (add `postFinalizeReading`), `apps/api/internal/api/api.go` (route)
- Test: `apps/api/internal/api/reading_takeaway_test.go`

**Interfaces:**
- Consumes: `readingOutcomesByMaterial` (re-assemble server-side), `Queries.FinalizeReadingTakeaway`, `a.logActivity`-equivalent (match the existing activity-log write used by `postPlanGenerate`).
- Produces: `func (a *API) postFinalizeReading(w, r)`; route `POST /projects/{id}/references/{rid}/finalize-reading`. Body `{ new_leads: [...], proposal_impact: "..." }`. Response: `{ reference: <dto with takeaway> }`.

- [ ] **Step 1: Failing test — persist + supersede + no spend**

Add to `reading_takeaway_test.go`:

```go
func TestFinalizeReading_PersistsAndSupersedesNoSpend(t *testing.T) {
	h, pool := planTestHandler(t)
	token := signInSeed(t, h)
	ref, _ := seedReadingOutcome(t, pool, "China emits the most", "中国碳排放全球第一")
	url := "/api/v1/projects/" + seedProjectID + "/references/" + ref.String() + "/finalize-reading"

	rr := doJSON(t, h, http.MethodPost, url, token, `{"new_leads":["人均口径"],"proposal_impact":"让步段反例"}`)
	if rr.Code != http.StatusOK || !strings.Contains(rr.Body.String(), "让步段反例") {
		t.Fatalf("finalize 1 failed: %d %s", rr.Code, rr.Body.String())
	}
	// Re-finalize supersedes.
	rr2 := doJSON(t, h, http.MethodPost, url, token, `{"new_leads":[],"proposal_impact":"改写后的影响"}`)
	if rr2.Code != http.StatusOK || !strings.Contains(rr2.Body.String(), "改写后的影响") {
		t.Fatalf("re-finalize should supersede: %d %s", rr2.Code, rr2.Body.String())
	}
	// Finalize spends NOTHING.
	if n := countLLMCallsByPurpose(t, pool, "reading_takeaway_draft") + countLLMCallsByPurpose(t, pool, "reading_takeaway"); n != 0 {
		t.Fatalf("finalize must not spend, got %d calls", n)
	}
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `cd apps/api && go test ./internal/api/ -run TestFinalizeReading -v`
Expected: FAIL (undefined `postFinalizeReading`).

- [ ] **Step 3: Implement**

```go
type finalizeReadingReq struct {
	NewLeads       []string `json:"new_leads"`
	ProposalImpact string   `json:"proposal_impact"`
}

func (a *API) postFinalizeReading(w http.ResponseWriter, r *http.Request) {
	projectID, ok := a.loadOwnedProject(w, r)
	if !ok {
		return
	}
	rid, err := uuid.Parse(httpx.URLParam(r, "rid"))
	if err != nil {
		httpx.WriteError(w, r, httpx.ErrNotFound("资源不存在"))
		return
	}
	var body finalizeReadingReq
	if err := decodeJSON(r, &body); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	ref, err := a.d.Queries.GetReferenceForProject(r.Context(), sqlc.GetReferenceForProjectParams{ID: rid, ProjectID: projectID})
	if err != nil || !ref.MaterialID.Valid {
		httpx.WriteError(w, r, httpx.ErrNotFound("尚未进入阅读室"))
		return
	}
	materialID := uuid.UUID(ref.MaterialID.Bytes)
	// Record fields are the source of truth = the confirmed outcomes, re-assembled
	// server-side (never trusted from the client). Student authored only synthesis.
	record := a.readingOutcomesByMaterial(r, projectID, materialID)
	takeaway := agent.ReadingTakeaway{
		Findings: record.Findings, Credibility: record.Credibility, KeyQuotes: record.KeyQuotes,
		NewLeads: nonNilStrings(body.NewLeads), ProposalImpact: strings.TrimSpace(body.ProposalImpact),
	}
	raw, _ := json.Marshal(takeaway)
	row, err := a.d.Queries.FinalizeReadingTakeaway(r.Context(), sqlc.FinalizeReadingTakeawayParams{
		ID: rid, ProjectID: projectID, Takeaway: raw,
	})
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	// Activity log (best-effort) — mirror postPlanGenerate's log write.
	a.appendActivity(r.Context(), projectID, "归纳了《"+truncateRunes(ref.Title, 30)+"》")
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"reference": toReferenceDTO(row, nil)})
}
```

> `a.appendActivity` — use the exact activity-log write used elsewhere (find how `postPlanGenerate` logs; call the same). Register `r.Post("/projects/{id}/references/{rid}/finalize-reading", a.postFinalizeReading)`.

Extend `toReferenceDTO` (Task 8's contract change) so the response carries `phaseTag` + structured `takeaway`.

- [ ] **Step 4: Run tests**

Run: `cd apps/api && go test ./internal/api/ -run 'TestFinalizeReading|TestGetTakeawayDraft' -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add apps/api/internal/api/reading_takeaway.go apps/api/internal/api/api.go apps/api/internal/api/reading_takeaway_test.go
git commit -m "feat(s2): finalize-reading — persist takeaway, supersede, no spend"
```

---

## Task 7: Spine projection — 在读 / 已归纳 lines

**Files:**
- Modify: `apps/api/internal/api/projectcoach.go` (the 文献库 block)
- Test: `apps/api/internal/api/projectcoach_projection_test.go` (or extend `coach_continuity_test.go`)

**Interfaces:**
- Consumes: `sqlc.Reference` new fields (`PhaseTag`, `Takeaway`, `TakeawayFinalizedAt`), `readingOutcomesByMaterial` (for the 在读 count — or count confirmed cards directly).
- Produces: extended `buildSpineProjection` output.

- [ ] **Step 1: Failing projection test**

```go
func TestProjection_ReadingStates(t *testing.T) {
	h, pool := planTestHandler(t)
	_ = signInSeed(t, h)
	// Seed one 已归纳 reference (takeaway set) and one 在读 (material set, no takeaway).
	seedFinalizedReference(t, pool, "已归纳源", "反例检验", "作为让步段证据")
	seedInProgressReference(t, pool, "在读源", "支持论点")

	proj, err := apiFromHandler(h).buildSpineProjection(context.Background(), mustUUID(t, seedProjectID))
	if err != nil {
		t.Fatalf("projection: %v", err)
	}
	if !strings.Contains(proj, "印记：作为让步段证据") {
		t.Fatalf("已归纳 source must carry proposal_impact:\n%s", proj)
	}
	if !strings.Contains(proj, "在读") {
		t.Fatalf("在读 source must show in-progress state:\n%s", proj)
	}
}
```

> `apiFromHandler`/`seedFinalizedReference`/`seedInProgressReference` — thin test helpers; `buildSpineProjection` is a method on `*API`, so expose the `*API` the handler wraps (the S1 projection tests already reach it — reuse that accessor).

- [ ] **Step 2: Run to verify it fails**

Run: `cd apps/api && go test ./internal/api/ -run TestProjection_ReadingStates -v`
Expected: FAIL (projection still shows the old `title｜decision` line).

- [ ] **Step 3: Implement**

Replace the 文献库 loop body in `buildSpineProjection` (`projectcoach.go`) with state-aware rendering:

```go
if refs, err := a.d.Queries.ListReferences(ctx, projectID); err == nil && len(refs) > 0 {
	b.WriteString("文献库：\n")
	for i, ref := range refs {
		if i >= 6 {
			fmt.Fprintf(&b, "- …另有 %d 条\n", len(refs)-6)
			break
		}
		phase := ""
		if ref.PhaseTag != nil && *ref.PhaseTag != "" {
			phase = "｜" + *ref.PhaseTag
		}
		switch {
		case ref.TakeawayFinalizedAt.Valid && len(ref.Takeaway) > 0:
			var tk agent.ReadingTakeaway
			_ = json.Unmarshal(ref.Takeaway, &tk)
			fmt.Fprintf(&b, "- %s%s｜印记：%s\n", truncateRunes(ref.Title, 32), phase, truncateRunes(tk.ProposalImpact, 40))
		case ref.MaterialID.Valid:
			n := len(a.readingOutcomesByMaterialCtx(ctx, projectID, uuid.UUID(ref.MaterialID.Bytes)).Findings)
			fmt.Fprintf(&b, "- %s%s｜在读·已确认 %d 条发现\n", truncateRunes(ref.Title, 32), phase, n)
		default:
			meta := ""
			if ref.Decision != nil && *ref.Decision != "" {
				meta = "｜" + *ref.Decision
			}
			fmt.Fprintf(&b, "- %s%s\n", truncateRunes(ref.Title, 40), meta)
		}
	}
}
```

> `readingOutcomesByMaterial` takes `*http.Request`; add a ctx-based sibling `readingOutcomesByMaterialCtx(ctx, projectID, materialID)` that both call (the `*http.Request` one just forwards `r.Context()`), so the projection (no request) can reuse it. Add the `agent` + `encoding/json` imports if missing.

- [ ] **Step 4: Run tests**

Run: `cd apps/api && go test ./internal/api/ -run 'TestProjection_ReadingStates|TestCoach_' -v`
Expected: PASS (S1 coach continuity still green).

- [ ] **Step 5: Commit**

```bash
git add apps/api/internal/api/projectcoach.go apps/api/internal/api/reading_takeaway.go apps/api/internal/api/projectcoach_projection_test.go
git commit -m "feat(s2): spine projection carries 在读/已归纳 reading state"
```

---

## Task 8: Contracts — phase_tag, takeaway, brief

**Files:**
- Modify: `packages/contracts/src/reference.ts`
- Create/Modify: `packages/contracts/src/reading.ts` (brief + draft response)
- Test: `packages/contracts/src/reference.test.ts` (or the package's test file)

**Interfaces:**
- Produces: `PhaseTag` (enum), `ReadingTakeaway`, extended `Reference` with `phaseTag`/`takeaway`; `ReadingBrief`, `TakeawayDraft`.

- [ ] **Step 1: Failing contract test**

Add to the contracts test file:

```ts
import { PhaseTag, ReadingTakeaway, Reference } from "./reference";

test("PhaseTag accepts the enum, rejects others", () => {
  expect(PhaseTag.parse("反例检验")).toBe("反例检验");
  expect(() => PhaseTag.parse("random")).toThrow();
});

test("Reference carries an optional structured takeaway", () => {
  const r = Reference.parse({
    id: "x", title: "t", classification: "", author: "", credentials: "",
    year: "", url: "", tags: [], collectionId: null, credibility: null,
    evaluation: "", decision: null, searchHints: [], materialId: null, notes: [],
    phaseTag: "反例检验",
    takeaway: { findings: ["a"], credibility: { verdict: "strong", why: "w" }, keyQuotes: [], newLeads: [], proposalImpact: "impact" },
  });
  expect(r.takeaway?.proposalImpact).toBe("impact");
});
```

> Match the exact current `Reference` field set (see `reference.ts`) — the test object must satisfy it. `phaseTag`/`takeaway` are `.optional().nullable()`.

- [ ] **Step 2: Run to verify it fails**

Run: `cd packages/contracts && pnpm test -- reference`
Expected: FAIL (no `PhaseTag`/`takeaway`).

- [ ] **Step 3: Implement**

In `reference.ts`:

```ts
export const PhaseTag = z.enum(["立题探索", "背景理解", "支持论点", "反例检验", "方法参考"]);
export type PhaseTag = z.infer<typeof PhaseTag>;

export const Credibility5 = z.object({ verdict: z.string(), why: z.string() });
export const KeyQuote = z.object({ quote: z.string(), why: z.string() });
export const ReadingTakeaway = z.object({
  findings: z.array(z.string()),
  credibility: Credibility5,
  keyQuotes: z.array(KeyQuote),
  newLeads: z.array(z.string()),
  proposalImpact: z.string(),
});
export type ReadingTakeaway = z.infer<typeof ReadingTakeaway>;
```

Extend the `Reference` object with:

```ts
  phaseTag: PhaseTag.nullable().optional(),
  takeaway: ReadingTakeaway.nullable().optional(),
```

Create `reading.ts`:

```ts
import { z } from "zod";
import { Credibility5, KeyQuote, PhaseTag } from "./reference";

export const ReadingBrief = z.object({
  readingReason: z.string(),
  readingFocus: z.string(),
  phaseTag: PhaseTag.or(z.literal("")),
});
export type ReadingBrief = z.infer<typeof ReadingBrief>;

export const TakeawayDraft = z.object({
  record: z.object({ findings: z.array(z.string()), credibility: Credibility5, keyQuotes: z.array(KeyQuote) }),
  suggestedNewLeads: z.array(z.string()),
  suggestedProposalImpact: z.string(),
});
export type TakeawayDraft = z.infer<typeof TakeawayDraft>;
```

Export both from the package index (`src/index.ts`).

- [ ] **Step 4: Run tests**

Run: `cd packages/contracts && pnpm test`
Expected: PASS (full contracts suite green).

- [ ] **Step 5: Commit**

```bash
git add packages/contracts/src
git commit -m "feat(s2): contracts — phase_tag, reading takeaway, brief, draft"
```

---

## Task 9: Frontend — brief banner, finalize panel, library chip

**Files:**
- Modify: `apps/web/src/workspace/api/workspace.ts`, `apps/web/src/studio/reading/readingLoop.ts`, `apps/web/src/studio/reading/ReadingRoom.tsx`, `apps/web/src/workspace/blocks/ReadingBlock.tsx`
- Test: `apps/web/src/studio/reading/ReadingRoom.test.tsx` (or the room's existing test)

**Interfaces:**
- Consumes: contracts `ReadingBrief`, `TakeawayDraft`, `ReadingTakeaway`, endpoints from Tasks 2/5/6.
- Produces: `putReadingBrief(projectId, rid, brief)`, `getTakeawayDraft(projectId, rid): Promise<TakeawayDraft>`, `postFinalizeReading(projectId, rid, {newLeads, proposalImpact}): Promise<Reference>` in `workspace.ts`.

- [ ] **Step 1: Add the api client calls**

In `apps/web/src/workspace/api/workspace.ts` (mirror the S1 `getCoachHistory`/`postProjectSummary` style — zod-parse the response):

```ts
import { ReadingBrief, TakeawayDraft } from "@mindimprint/contracts";

export async function putReadingBrief(projectId: string, rid: string, brief: ReadingBrief) {
  return apiFetch(`/projects/${projectId}/references/${rid}/reading-brief`, {
    method: "PUT", body: JSON.stringify({
      reading_reason: brief.readingReason, reading_focus: brief.readingFocus, phase_tag: brief.phaseTag,
    }),
  });
}

export async function getTakeawayDraft(projectId: string, rid: string): Promise<TakeawayDraft> {
  const res = await apiFetch(`/projects/${projectId}/references/${rid}/takeaway-draft`);
  return TakeawayDraft.parse(res);
}

export async function postFinalizeReading(projectId: string, rid: string, s: { newLeads: string[]; proposalImpact: string }) {
  return apiFetch(`/projects/${projectId}/references/${rid}/finalize-reading`, {
    method: "POST", body: JSON.stringify({ new_leads: s.newLeads, proposal_impact: s.proposalImpact }),
  });
}
```

(Use the actual `apiFetch`/base-URL helper the file already uses.)

- [ ] **Step 2: Failing component test**

In `ReadingRoom.test.tsx`, render with a stubbed api; assert (a) the brief banner shows the pre-filled reason, (b) clicking 完成这篇 fetches the draft and shows the two editable synthesis fields, (c) confirming calls `postFinalizeReading` with the edited values. Write it against the real component contract; run it to see it fail first.

- [ ] **Step 3: Implement the room UI**

- Add a **brief banner** at the top of the coach column in `ReadingRoom.tsx`: shows `你读这篇是为了：<reason>` with an edit affordance; on mount, if the source's `readingReason` is empty, pre-fill from the entry `suggestedReason` (passed through the `MaterialSource`/entry response) and `putReadingBrief` on blur/confirm. Add a `phaseTag` single-select (the 5 values).
- Add a **完成这篇** button near the 阅读成果 tab. On click: `getTakeawayDraft` → open a finalize panel showing the assembled `record` (read-only: findings, credibility, key quotes) + two editable textareas seeded with `suggestedNewLeads`/`suggestedProposalImpact`. On 确认归纳: `postFinalizeReading` → mark the source 已归纳 (call `onOpenLogged`/refresh so the container re-reads the reference) → close the room or show a 已归纳 state.
- Extend `ReadingLoopApi` in `readingLoop.ts` with the three calls if the room drives them through the loop; otherwise call `workspace.ts` directly from the room. Keep it minimal.

- [ ] **Step 4: Library chip + preview (ReadingBlock.tsx)**

- Render a `phaseTag` chip on reference rows when set; show a 已归纳/在读 badge (derived: `takeaway != null` → 已归纳).
- In `Preview`, when `takeaway` is present, render the structured takeaway (findings list, credibility, key quotes, new leads, proposal impact) instead of only `notes`.

- [ ] **Step 5: Run tests**

Run: `cd apps/web && pnpm test -- ReadingRoom`
Expected: PASS. Then the full web suite: `pnpm test`.

- [ ] **Step 6: Commit**

```bash
git add apps/web/src
git commit -m "feat(s2): reading brief banner + finalize panel + library takeaway"
```

---

## Task 10: Full suites + deploy

**Files:** none (verification + deploy runbook).

- [ ] **Step 1: Regenerate + full backend suite**

Run:
```bash
cd apps/api && CGO_ENABLED=0 make sqlc && go build ./... && go test ./...
```
Expected: all packages PASS (agent + api, incl. the new reading tests; run FULL packages, not `-run` subsets — the whole-branch-review lesson).

- [ ] **Step 2: Contracts + web suites**

Run:
```bash
cd packages/contracts && pnpm test
cd ../../apps/web && pnpm test && pnpm build
```
Expected: contracts green, web green, web build clean.

- [ ] **Step 3: Merge to main**

```bash
git checkout main && git merge --ff-only feat/s2-reading-subagent && git push origin main
```

- [ ] **Step 4: Deploy (ECS 47.93.151.131, `/home/deploy/mind-imprint`)**

On the server (see `s1-continuous-session-build` memory for exact flow): `git pull` → build images (`docker compose -f deploy/docker-compose.prod.yml --env-file deploy/.env.prod build api web`) → **migrate** (`run --rm api -migrate-up` → expect `OK 0039_reading_takeaway.sql`) → `up -d api web`. Split build/up (the auto-mode guard blocks `up --build` on prod).

- [ ] **Step 5: Smoke**

- `healthz` 200; web 200; signin 200 (student).
- Enter a source → `PUT reading-brief` persists; a read-turn reply reflects the purpose.
- `GET takeaway-draft` returns assembled findings + seeded synthesis (one `reading_takeaway_draft` llm_call).
- `POST finalize-reading` → `GET /coach` context / projection shows the `印记：…` line for that source; no extra spend on finalize.

- [ ] **Step 6: Record memory**

Update `s2-...` memory + `multi-agent-architecture-direction` (S2 DONE & SHIPPED), MEMORY.md pointer.

---

## Self-Review

**Spec coverage:** §0 isolation reframe → Tasks 3–7 (brief injection + isolated compose + assembly). §1 lifecycle → Task 1 (finalized_at) + Task 6 (supersede) + Task 7 (在读/已归纳 projection). §2 migration → Task 1. §3 brief-in → Tasks 2–3. §4 takeaways-out (split-hybrid) → Tasks 4–6. §5 phase_tag → Tasks 1, 8, 9. §6 projection → Task 7. §7 frontend → Task 9. §8 contracts → Task 8. §9 tests+deploy → every task's tests + Task 10. §10 deferred (rabbit-hole/branch) → not built; `new_leads` stored as data only (Task 6). ✅ All covered.

**Placeholder scan:** No "TBD/handle errors/similar to". Every code step shows real code. The few `>`-noted verifications ("confirm the exact sqlc field name", "match the router param accessor") are *alignment checks against existing code*, not deferred work — the implementer confirms a name, they don't invent behavior.

**Type consistency:** `ReadingTakeaway` fields (`findings/credibility/keyQuotes/newLeads/proposalImpact` on the wire; `Findings/Credibility/KeyQuotes/NewLeads/ProposalImpact` in Go) consistent across Tasks 4, 6, 7, 8. `TakeawayRecord`/`Credibility`/`KeyQuote` defined in Task 4, reused in Tasks 5–7. `ComposeReadingTakeawaySuggestions` signature identical in Tasks 4 and 5. `readingOutcomesByMaterial` (+ `...Ctx` sibling) defined Task 4/5, reused Task 7. Routes registered once each (Tasks 2, 5, 6). Purpose strings `reading_takeaway_draft` (draft spends) vs finalize (no spend) consistent in Tasks 5, 6, 10. ✅
