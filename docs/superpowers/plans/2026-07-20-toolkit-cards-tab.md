# 工具卡 (Toolkit Cards) Tab Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a third 工具卡 tab to 成长报告 showing the tool cards a student has completed across project/course/chat, grouped by category, as a cost-free read-only projection.

**Architecture:** One new owner-scoped SQL query (`ListCollectedCardsByUser`, UNION-ALL of the 3 scopes, `GROUP BY card_id`) → one new endpoint `GET /api/v1/growth/cards` (handler-only, no model call) → one new Zod contract `CollectedCards` → one web client `getGrowthCards` → one self-fetching `<ToolkitCards>` component that joins each `cardId` to `CARD_REGISTRY` for name/purpose/category, groups, and renders → wired as a tab in `GrowthReport`. Mirrors the existing `/growth/ability` work end-to-end.

**Tech Stack:** Go (`net/http` + sqlc + pgx), PostgreSQL, React + Vite + TypeScript, Zod contracts, vitest + @testing-library/react, testcontainers-go.

## Global Constraints

- **Client never calls a model directly.** All data via the API; keys server-side only. This tab makes **no** model call and writes **zero** `llm_call` rows.
- **RL-5:** no score, level, rank, or total anywhere in this tab. Usage counts are descriptive, never a grade.
- **铁律 2 (不操纵):** no locked "collect them all" catalog, no progress-to-100%, no earnable badges, no streaks. Only cards actually completed appear.
- **Collected = `status = 'completed'` only.** Skipped/proposed/active cards never appear (matches `collectedCourseSessionCards`).
- **Owner isolation:** `card_instances` has no `user_id`; every row is reached through its scope's owner table (`project.user_id` / `course_session.user_id` / `chat_thread.user_id`).
- **Single source of truth:** card name/purpose/category come from `CARD_REGISTRY` (`@mind-imprint/contracts`), never re-encoded server-side. Unknown `cardId`s are dropped client-side, never faked.
- `make sqlc` from `apps/api` (`CGO_ENABLED=0`); **never hand-edit** `apps/api/internal/store/sqlc/*`.
- Go tests: `DOCKER_HOST=unix:///var/run/docker.sock CGO_ENABLED=0 go test -p 1 ./...`. For migration/query/endpoint changes run the **full package**, never a `-run` subset.
- Web tests from `apps/web`; contracts tests from `packages/contracts`. Icons inline SVG, never `lucide-react`.
- **NEVER `git add` a bare directory or the pre-existing untracked user files** (`M package.json` + untracked files under `docs/` and repo root are NOT ours). Add only the exact files each task touches.
- Intro copy is softened from the dc.html mockup's "五大分支" to "按类别组织" (the card taxonomy is 9 categories, not 5 branches).

---

### Task 1: `ListCollectedCardsByUser` query + query test

**Files:**
- Modify: `apps/api/internal/store/queries/card_instance.sql` (append the query at end)
- Regenerate: `apps/api/internal/store/sqlc/card_instance.sql.go` (via `make sqlc`)
- Test: `apps/api/internal/store/collected_cards_query_test.go` (create)

**Interfaces:**
- Produces: `Queries.ListCollectedCardsByUser(ctx, userID uuid.UUID) ([]sqlc.ListCollectedCardsByUserRow, error)`. The generated `ListCollectedCardsByUserRow` struct has fields `CardID string`, `Uses int32`, `Surfaces []string`, `LastUsed <T>`. `<T>` is sqlc's inference for `max(created_at)` — **expected `pgtype.Timestamptz`** (aggregate ⇒ nullable-typed). Verify the exact type after `make sqlc` and report it in the task report, because Task 2 formats it.

- [ ] **Step 1: Write the failing test**

Create `apps/api/internal/store/collected_cards_query_test.go`. Seed patterns copied from `growth_history_query_test.go` (same package; `newTestPool`, `refactor2SeededStudentID`, the course-session insert columns, and the "other user" insert are all reused verbatim).

```go
package store

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"

	"mindimprint/api/internal/store/sqlc"
)

// ListCollectedCardsByUser returns one row per card_id the caller has COMPLETED,
// across all three scopes, with uses (count), surfaces (distinct), and last_used
// (max created_at). Skipped instances and other users' rows are excluded.
func TestListCollectedCardsByUser(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping testcontainers integration in -short mode")
	}
	ctx := context.Background()
	pool := newTestPool(t)
	q := sqlc.New(pool)

	// Owned project with TWO completed instances of card "concession" (dedupe →
	// uses 2, last_used = the later one) at controlled created_at.
	var projectID pgtype.UUID
	if err := pool.QueryRow(ctx, `
		INSERT INTO project (user_id, title) VALUES ($1, '中国可持续') RETURNING id`,
		refactor2SeededStudentID).Scan(&projectID); err != nil {
		t.Fatalf("seed project: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO card_instances (project_id, card_id, status, created_at)
		VALUES ($1, 'concession', 'completed', now() - interval '3 days')`, projectID); err != nil {
		t.Fatalf("seed concession #1: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO card_instances (project_id, card_id, status, created_at)
		VALUES ($1, 'concession', 'completed', now() - interval '1 day')`, projectID); err != nil {
		t.Fatalf("seed concession #2: %v", err)
	}
	// A SKIPPED instance of a different card — must be excluded entirely.
	if _, err := pool.Exec(ctx, `
		INSERT INTO card_instances (project_id, card_id, status)
		VALUES ($1, 'toulmin', 'skipped')`, projectID); err != nil {
		t.Fatalf("seed skipped toulmin: %v", err)
	}

	// Owned course session with ONE completed instance of card "opcvl".
	var sessionID pgtype.UUID
	if err := pool.QueryRow(ctx, `
		INSERT INTO course_session (user_id, course_id, skill_id, phase)
		VALUES ($1, '00000000-0000-0000-0000-0000000000c1', 'info-literacy-course', 'reflect')
		RETURNING id`, refactor2SeededStudentID).Scan(&sessionID); err != nil {
		t.Fatalf("seed course_session: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO card_instances (session_id, card_id, status)
		VALUES ($1, 'opcvl', 'completed')`, sessionID); err != nil {
		t.Fatalf("seed opcvl: %v", err)
	}

	// A DIFFERENT user's completed card — must be excluded (owner isolation).
	var otherUser pgtype.UUID
	if err := pool.QueryRow(ctx, `
		INSERT INTO users (email, password_hash, role, school_id, display_name, avatar_color)
		SELECT 'cards-other@example.com', password_hash, 'student', school_id, 'Other', avatar_color
		FROM users WHERE id = $1
		RETURNING id`, refactor2SeededStudentID).Scan(&otherUser); err != nil {
		t.Fatalf("seed other user: %v", err)
	}
	var otherProject pgtype.UUID
	if err := pool.QueryRow(ctx, `
		INSERT INTO project (user_id, title) VALUES ($1, 'not mine') RETURNING id`,
		otherUser).Scan(&otherProject); err != nil {
		t.Fatalf("seed other project: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO card_instances (project_id, card_id, status)
		VALUES ($1, 'craap', 'completed')`, otherProject); err != nil {
		t.Fatalf("seed other completed card: %v", err)
	}

	rows, err := q.ListCollectedCardsByUser(ctx, refactor2SeededStudentID)
	if err != nil {
		t.Fatalf("ListCollectedCardsByUser: %v", err)
	}
	// Two owned cards: concession (uses 2, project) + opcvl (uses 1, course).
	// toulmin skipped → absent; craap belongs to another user → absent.
	if len(rows) != 2 {
		t.Fatalf("got %d rows, want 2 (concession + opcvl; skipped & other-user excluded)", len(rows))
	}
	byCard := map[string]sqlc.ListCollectedCardsByUserRow{}
	for _, r := range rows {
		byCard[r.CardID] = r
	}
	con, ok := byCard["concession"]
	if !ok {
		t.Fatalf("concession missing; got %v", byCard)
	}
	if con.Uses != 2 {
		t.Errorf("concession uses = %d, want 2 (deduped count)", con.Uses)
	}
	if len(con.Surfaces) != 1 || con.Surfaces[0] != "project" {
		t.Errorf("concession surfaces = %v, want [project]", con.Surfaces)
	}
	op, ok := byCard["opcvl"]
	if !ok {
		t.Fatalf("opcvl missing; got %v", byCard)
	}
	if op.Uses != 1 || len(op.Surfaces) != 1 || op.Surfaces[0] != "course" {
		t.Errorf("opcvl = uses %d surfaces %v, want 1 / [course]", op.Uses, op.Surfaces)
	}
	if _, bad := byCard["toulmin"]; bad {
		t.Error("toulmin (skipped) must not appear")
	}
	if _, bad := byCard["craap"]; bad {
		t.Error("craap (other user) must not appear")
	}
}
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `cd apps/api && DOCKER_HOST=unix:///var/run/docker.sock CGO_ENABLED=0 go test -p 1 ./internal/store/ -run TestListCollectedCardsByUser`
Expected: FAIL — `q.ListCollectedCardsByUser undefined` (query not written yet).

- [ ] **Step 3: Add the query**

Append to `apps/api/internal/store/queries/card_instance.sql`:

```sql
-- name: ListCollectedCardsByUser :many
-- Every tool card the caller has COMPLETED at least once, across all three
-- scopes, deduped to one row per card_id with a usage summary. status='completed'
-- only (a skip is a decline, matching collectedCourseSessionCards). Owner-filtered
-- through each scope's own parent join (card_instances has no user_id). No cost,
-- no model — a pure read for the 工具卡 tab. created_at (always non-null) is the
-- usage timestamp; completed_at is only set on the session-scope path, so it is
-- not used here.
SELECT card_id,
       count(*)::int AS uses,
       array_agg(DISTINCT surface)::text[] AS surfaces,
       max(created_at) AS last_used
FROM (
  (SELECT ci.card_id AS card_id, 'project'::text AS surface, ci.created_at AS created_at
   FROM card_instances ci JOIN project p ON p.id = ci.project_id
   WHERE ci.project_id IS NOT NULL AND ci.status = 'completed' AND p.user_id = @user_id)
  UNION ALL
  (SELECT ci.card_id, 'course'::text, ci.created_at
   FROM card_instances ci JOIN course_session cs ON cs.id = ci.session_id
   WHERE ci.session_id IS NOT NULL AND ci.status = 'completed' AND cs.user_id = @user_id)
  UNION ALL
  (SELECT ci.card_id, 'chat'::text, ci.created_at
   FROM card_instances ci JOIN chat_thread t ON t.id = ci.thread_id
   WHERE ci.thread_id IS NOT NULL AND ci.status = 'completed' AND t.user_id = @user_id)
) rows
GROUP BY card_id
ORDER BY uses DESC, last_used DESC;
```

- [ ] **Step 4: Regenerate sqlc**

Run: `cd apps/api && CGO_ENABLED=0 make sqlc`
Expected: `apps/api/internal/store/sqlc/card_instance.sql.go` gains `ListCollectedCardsByUser` + `ListCollectedCardsByUserRow`. Open the generated struct and note the `LastUsed` field's type (expected `pgtype.Timestamptz`) — record it in the task report for Task 2.

- [ ] **Step 5: Run the test to verify it passes**

Run: `cd apps/api && DOCKER_HOST=unix:///var/run/docker.sock CGO_ENABLED=0 go test -p 1 ./internal/store/`
Expected: PASS (full store package, not a `-run` subset — the query change must not regress sibling query tests).

- [ ] **Step 6: Commit**

```bash
git add apps/api/internal/store/queries/card_instance.sql apps/api/internal/store/sqlc/card_instance.sql.go apps/api/internal/store/collected_cards_query_test.go
git commit -m "feat(toolkit): ListCollectedCardsByUser query (completed cards, owner-filtered)"
```

---

### Task 2: `GET /api/v1/growth/cards` endpoint

**Files:**
- Create: `apps/api/internal/api/cards_growth.go`
- Modify: `apps/api/internal/api/api.go` (add one route beside `/growth/ability`)
- Test: `apps/api/internal/api/cards_growth_test.go` (create)

**Interfaces:**
- Consumes: `a.d.Queries.ListCollectedCardsByUser(ctx, u.ID) ([]sqlc.ListCollectedCardsByUserRow, error)` from Task 1. `Row.LastUsed` is `pgtype.Timestamptz` (confirm against Task 1's report; if it differs, adjust the `.Time` access accordingly).
- Produces: JSON body `{"cards":[{"cardId":string,"uses":int,"surfaces":[string],"lastUsed":RFC3339}]}`. Consumed by Task 3's Zod contract and Task 4's client.

- [ ] **Step 1: Write the failing test**

Create `apps/api/internal/api/cards_growth_test.go`. Mirrors `ability_test.go`'s harness (`newAPITestPool`, `signInSeed`, `withCookie`, `countAllLLMCalls`, the `Deps`/`New(...).Handler()` shape). Empty-state: a fresh student has no completed cards → `{"cards":[]}` 200, zero `llm_call` rows.

```go
package api_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	. "mindimprint/api/internal/api"
	"mindimprint/api/internal/cards"
	"mindimprint/api/internal/store/sqlc"
)

// TestGetGrowthCards_EmptyStateNoModelCall — a fresh student with no completed
// cards gets a 200 {"cards":[]}, and NO llm_call row is written (pure read).
func TestGetGrowthCards_EmptyStateNoModelCall(t *testing.T) {
	pool := newAPITestPool(t)
	h := New(Deps{
		Queries: sqlc.New(pool), Pool: pool,
		Provider: assessStubProvider(dualAxisReply), ChatResolver: fakeResolver(), EvalResolver: fakeEvalResolver(), SpecByID: cards.ByID,
	}).Handler()
	cookie := signInSeed(t, pool)

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("GET", "/api/v1/growth/cards", nil), cookie))
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /growth/cards = %d, want 200; body=%s", rec.Code, rec.Body)
	}
	var body struct {
		Cards []struct {
			CardID   string   `json:"cardId"`
			Uses     int      `json:"uses"`
			Surfaces []string `json:"surfaces"`
			LastUsed string   `json:"lastUsed"`
		} `json:"cards"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode cards: %v — body=%s", err, rec.Body)
	}
	if len(body.Cards) != 0 {
		t.Fatalf("fresh student cards = %d, want 0", len(body.Cards))
	}
	if n := countAllLLMCalls(t, pool); n != 0 {
		t.Fatalf("llm_call rows = %d, want 0 (cards is a pure read)", n)
	}
}
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `cd apps/api && DOCKER_HOST=unix:///var/run/docker.sock CGO_ENABLED=0 go test -p 1 ./internal/api/ -run TestGetGrowthCards_EmptyStateNoModelCall`
Expected: FAIL — route 404 (handler not registered).

- [ ] **Step 3: Write the handler**

Create `apps/api/internal/api/cards_growth.go`:

```go
package api

import (
	"net/http"
	"time"

	"mindimprint/api/internal/httpx"
)

// collectedCardDTO is one tool card the caller has completed, with a usage
// summary. Instance-derived only — name/purpose/category are resolved on the web
// from CARD_REGISTRY (single source of truth), never re-encoded here. RL-5: uses
// is descriptive, never a grade.
type collectedCardDTO struct {
	CardID   string   `json:"cardId"`
	Uses     int      `json:"uses"`
	Surfaces []string `json:"surfaces"` // subset of {"project","course","chat"}
	LastUsed string   `json:"lastUsed"` // RFC3339
}

// getGrowthCards returns every tool card the caller has completed, across
// project/course/chat, deduped with a usage summary. No model call, ever
// (mirrors getAbilityModel).
func (a *API) getGrowthCards(w http.ResponseWriter, r *http.Request) {
	u, _ := UserFromContext(r.Context())
	rows, err := a.d.Queries.ListCollectedCardsByUser(r.Context(), u.ID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	cards := make([]collectedCardDTO, 0, len(rows))
	for _, row := range rows {
		surfaces := row.Surfaces
		if surfaces == nil {
			surfaces = []string{}
		}
		cards = append(cards, collectedCardDTO{
			CardID:   row.CardID,
			Uses:     int(row.Uses),
			Surfaces: surfaces,
			LastUsed: row.LastUsed.Time.Format(time.RFC3339),
		})
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"cards": cards})
}
```

If Task 1 reported `LastUsed` as a type other than `pgtype.Timestamptz`, adjust the `.Time.Format(...)` line: for a bare `time.Time`, use `row.LastUsed.Format(time.RFC3339)`; for `interface{}`, type-assert to `time.Time` first.

- [ ] **Step 4: Register the route**

In `apps/api/internal/api/api.go`, add beneath the existing growth routes (currently lines 56–57):

```go
	mux.Handle("GET /api/v1/growth/cards", protected(a.getGrowthCards))
```

- [ ] **Step 5: Run the test to verify it passes**

Run: `cd apps/api && DOCKER_HOST=unix:///var/run/docker.sock CGO_ENABLED=0 go test -p 1 ./internal/api/ -run TestGetGrowthCards_EmptyStateNoModelCall`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add apps/api/internal/api/cards_growth.go apps/api/internal/api/api.go apps/api/internal/api/cards_growth_test.go
git commit -m "feat(toolkit): GET /growth/cards endpoint (cost-free collected-cards read)"
```

---

### Task 3: contracts `CollectedCards` schema

**Files:**
- Create: `packages/contracts/src/collectedCards.ts`
- Modify: `packages/contracts/src/index.ts` (add one barrel export line)
- Test: `packages/contracts/src/collectedCards.test.ts` (create)

**Interfaces:**
- Produces: `CollectedCard` (`{ cardId: string; uses: number; surfaces: string[]; lastUsed: string }`) and `CollectedCards` (`{ cards: CollectedCard[] }`) Zod schemas + inferred types, exported from the barrel. Consumed by Task 4 (`.parse`) and Task 5 (the type).

- [ ] **Step 1: Write the failing test**

Create `packages/contracts/src/collectedCards.test.ts`:

```ts
import { describe, it, expect } from "vitest";
import { CollectedCards } from "./collectedCards";

describe("CollectedCards", () => {
  it("parses a representative payload", () => {
    const parsed = CollectedCards.parse({
      cards: [
        { cardId: "concession", uses: 2, surfaces: ["project"], lastUsed: "2026-07-18T00:00:00Z" },
        { cardId: "opcvl", uses: 1, surfaces: ["course"], lastUsed: "2026-07-17T00:00:00Z" },
      ],
    });
    expect(parsed.cards.length).toBe(2);
    expect(parsed.cards[0]!.uses).toBe(2);
  });

  it("rejects a card missing a required field", () => {
    expect(() => CollectedCards.parse({ cards: [{ cardId: "x", uses: 1, surfaces: [] }] })).toThrow();
  });
});
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `cd packages/contracts && npx vitest run src/collectedCards.test.ts`
Expected: FAIL — cannot resolve `./collectedCards`.

- [ ] **Step 3: Write the schema**

Create `packages/contracts/src/collectedCards.ts`:

```ts
import { z } from "zod";

// CollectedCard: one tool card the student has completed, with a usage summary.
// Mirrors apps/api/internal/api.collectedCardDTO (camelCase). Instance-derived
// only — name/purpose/category are resolved from CARD_REGISTRY on the web, not
// carried here. RL-5: uses is descriptive, never a grade.
export const CollectedCard = z.object({
  cardId: z.string(),
  uses: z.number().int(),
  surfaces: z.array(z.string()), // subset of "project" | "course" | "chat"
  lastUsed: z.string(),          // RFC3339
});
export type CollectedCard = z.infer<typeof CollectedCard>;

export const CollectedCards = z.object({ cards: z.array(CollectedCard) });
export type CollectedCards = z.infer<typeof CollectedCards>;
```

- [ ] **Step 4: Export from the barrel**

In `packages/contracts/src/index.ts`, add after the `./ability` export (currently line 21):

```ts
export * from "./collectedCards";
```

- [ ] **Step 5: Run the test to verify it passes**

Run: `cd packages/contracts && npx vitest run src/collectedCards.test.ts`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add packages/contracts/src/collectedCards.ts packages/contracts/src/index.ts packages/contracts/src/collectedCards.test.ts
git commit -m "feat(toolkit): contracts CollectedCards schema"
```

---

### Task 4: web `getGrowthCards` client + facade wiring

**Files:**
- Create: `apps/web/src/api/cards.ts`
- Modify: `apps/web/src/api/index.ts` (4 additive touch points)
- Test: `apps/web/src/api/cards.test.ts` (create)

**Interfaces:**
- Consumes: `CollectedCards` from Task 3, `apiFetch` from `./client`.
- Produces: `getGrowthCards(): Promise<CollectedCard[]>`, exposed on the `ApiClient` facade as `getGrowthCards`. Consumed by Task 5's `<ToolkitCards>` via `api.getGrowthCards()`.

- [ ] **Step 1: Write the failing test**

Create `apps/web/src/api/cards.test.ts` (mirrors `ability.test.ts`):

```ts
import { describe, it, expect, vi, afterEach } from "vitest";
import { getGrowthCards } from "./cards";

afterEach(() => { vi.restoreAllMocks(); });

describe("getGrowthCards", () => {
  it("parses the collected-cards list", async () => {
    const body = { cards: [
      { cardId: "concession", uses: 2, surfaces: ["project"], lastUsed: "2026-07-18T00:00:00Z" },
      { cardId: "opcvl", uses: 1, surfaces: ["course"], lastUsed: "2026-07-17T00:00:00Z" },
    ] };
    vi.spyOn(global, "fetch").mockResolvedValue(new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" } }));
    const cards = await getGrowthCards();
    expect(cards.length).toBe(2);
    expect(cards[0]!.cardId).toBe("concession");
  });
});
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `cd apps/web && npx vitest run src/api/cards.test.ts`
Expected: FAIL — cannot resolve `./cards`.

- [ ] **Step 3: Write the client**

Create `apps/web/src/api/cards.ts`:

```ts
import { CollectedCards, type CollectedCard } from "@mind-imprint/contracts";
import { apiFetch } from "./client";

// The 工具卡 tab: tool cards the student has completed across all surfaces.
// Read-only, cost-free.
export async function getGrowthCards(): Promise<CollectedCard[]> {
  const raw = await apiFetch<unknown>(`/api/v1/growth/cards`);
  return CollectedCards.parse(raw).cards;
}
```

- [ ] **Step 4: Wire into the facade (4 additive points)**

In `apps/web/src/api/index.ts`:

1. Add `CollectedCard` to the type import on line 1 (append to the `@mind-imprint/contracts` type list): `..., AbilityModel, CollectedCard } from "@mind-imprint/contracts";`
2. After the `getAbilityModel` import (line 14), add: `import { getGrowthCards } from "./cards";`
3. In the `ApiClient` interface, after `getAbilityModel(): Promise<AbilityModel>;` (line 83), add: `getGrowthCards(): Promise<CollectedCard[]>;`
4. In the `api` object, after `getAbilityModel,` (line 101), add: `getGrowthCards,`

- [ ] **Step 5: Run the test + typecheck**

Run: `cd apps/web && npx vitest run src/api/cards.test.ts && npx tsc --noEmit`
Expected: test PASS; `tsc` reports no NEW errors (a single pre-existing `interactionPrimitive.test.ts` error is known and unrelated).

- [ ] **Step 6: Commit**

```bash
git add apps/web/src/api/cards.ts apps/web/src/api/index.ts apps/web/src/api/cards.test.ts
git commit -m "feat(toolkit): web getGrowthCards client + facade"
```

---

### Task 5: `<ToolkitCards>` component

**Files:**
- Create: `apps/web/src/shell/growth/ToolkitCards.tsx`
- Test: `apps/web/src/shell/growth/ToolkitCards.test.tsx` (create)

**Interfaces:**
- Consumes: `api.getGrowthCards()` (Task 4), `CARD_REGISTRY` + `type CollectedCard` from `@mind-imprint/contracts`.
- Produces: the `ToolkitCards` component (default-less named export `export function ToolkitCards()`). Consumed by Task 6's `GrowthReport`.

- [ ] **Step 1: Write the failing test**

Create `apps/web/src/shell/growth/ToolkitCards.test.tsx`. `concession` is category `知识工具`, `opcvl` is `AOK` (verified in the registry); `__no_such_card__` is unknown and must be dropped.

```tsx
import { describe, it, expect, vi, afterEach } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import { ToolkitCards } from "./ToolkitCards";
import { api } from "../../api";

afterEach(() => { vi.restoreAllMocks(); });

const cards = [
  { cardId: "concession", uses: 2, surfaces: ["project"], lastUsed: "2026-07-18T00:00:00Z" }, // 知识工具
  { cardId: "opcvl", uses: 1, surfaces: ["course", "chat"], lastUsed: "2026-07-17T00:00:00Z" }, // AOK
  { cardId: "__no_such_card__", uses: 9, surfaces: ["project"], lastUsed: "2026-07-16T00:00:00Z" }, // dropped
];

describe("ToolkitCards", () => {
  it("groups by category, shows usage, and drops unknown card ids", async () => {
    vi.spyOn(api, "getGrowthCards").mockResolvedValue(cards as never);
    render(<ToolkitCards />);
    await waitFor(() => expect(screen.getByText(/按类别组织/)).toBeTruthy());
    // category group headers
    expect(screen.getByText("知识工具")).toBeTruthy();
    expect(screen.getByText("AOK")).toBeTruthy();
    // the ×N badge (concession used twice) and a surface/最近 usage line
    expect(screen.getByText("×2")).toBeTruthy();
    expect(screen.getByText(/最近 07-18/)).toBeTruthy();
    expect(screen.getByText(/项目/)).toBeTruthy();
    // unknown id contributed no tile: its bogus ×9 must be absent
    expect(screen.queryByText("×9")).toBeNull();
  });

  it("renders the empty state when nothing is collected", async () => {
    vi.spyOn(api, "getGrowthCards").mockResolvedValue([] as never);
    render(<ToolkitCards />);
    await waitFor(() => expect(screen.getByText(/还没有收集到工具卡/)).toBeTruthy());
  });
});
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `cd apps/web && npx vitest run src/shell/growth/ToolkitCards.test.tsx`
Expected: FAIL — cannot resolve `./ToolkitCards`.

- [ ] **Step 3: Write the component**

Create `apps/web/src/shell/growth/ToolkitCards.tsx`:

```tsx
import { useEffect, useState } from "react";
import { CARD_REGISTRY, type CollectedCard } from "@mind-imprint/contracts";
import { api } from "../../api";

const SURFACE_LABEL: Record<string, string> = { project: "项目", course: "课程", chat: "聊天" };

// Fixed display order of the card categories (roughly the course-library arc),
// each with a dot color. Any category not listed falls into a final 其他 group.
const CATEGORY_ORDER: { key: string; dot: string }[] = [
  { key: "探究启动", dot: "#D98263" },
  { key: "信息素养", dot: "#2A3B7A" },
  { key: "溯源与多视角", dot: "#4C9A82" },
  { key: "知识工具", dot: "#7C6BB0" },
  { key: "论证结构", dot: "#C77CA6" },
  { key: "AOK", dot: "#D9A23D" },
  { key: "AI伦理", dot: "#3E8EA8" },
  { key: "反身性与元认知", dot: "#5C6CB0" },
  { key: "成长与沉淀", dot: "#8A9A5B" },
];
const OTHER = { key: "其他", dot: "#9AA1B0" };

type Enriched = CollectedCard & { name: string; purpose: string; category: string };

function enrich(cards: CollectedCard[]): Enriched[] {
  const out: Enriched[] = [];
  for (const c of cards) {
    const spec = CARD_REGISTRY[c.cardId];
    if (!spec) continue; // drop ids absent from the registry (defensive)
    out.push({ ...c, name: spec.name, purpose: spec.purpose, category: spec.category });
  }
  return out;
}

function usageLine(c: CollectedCard): string {
  const surfaces = c.surfaces.map((s) => SURFACE_LABEL[s] ?? s).join(" · ");
  const md = c.lastUsed.slice(5, 10); // MM-DD from an RFC3339 date
  return `${surfaces} · 最近 ${md}`;
}

export function ToolkitCards() {
  const [cards, setCards] = useState<CollectedCard[] | undefined>(undefined);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;
    void (async () => {
      try {
        const list = await api.getGrowthCards();
        if (!cancelled) setCards(list);
      } catch {
        if (!cancelled) setError("加载失败，请重试");
      }
    })();
    return () => { cancelled = true; };
  }, []);

  if (error) return <div style={{ padding: 24, color: "#B0432E" }}>{error}</div>;
  if (cards === undefined) return <div style={{ padding: 24, color: "#9AA1B0" }}>正在整理你的工具卡…</div>;

  const enriched = enrich(cards);
  if (enriched.length === 0) {
    return (
      <div style={{ background: "#fff", border: "1px solid #EAECF2", borderRadius: 16, padding: "34px 24px", textAlign: "center" }}>
        <div style={{ fontSize: 14.5, fontWeight: 700, color: "#3A4256" }}>还没有收集到工具卡</div>
        <div style={{ fontSize: 13.5, color: "#6B7384", marginTop: 8, lineHeight: 1.7 }}>在任务、课程或聊天里用过一张思维工具卡，它就会出现在这里。</div>
      </div>
    );
  }

  const groups = [...CATEGORY_ORDER, OTHER].map(({ key, dot }) => ({
    key, dot,
    cards: enriched.filter((c) => (key === OTHER.key
      ? !CATEGORY_ORDER.some((g) => g.key === c.category)
      : c.category === key)),
  })).filter((g) => g.cards.length > 0);

  return (
    <div>
      <div style={{ fontSize: 13.5, color: "#6B7384", marginBottom: 18, lineHeight: 1.6 }}>
        你收集到的思维工具卡，按类别组织。用得越多，越成为你的本能。
      </div>
      {groups.map((g) => (
        <div key={g.key} style={{ marginBottom: 24 }}>
          <div style={{ display: "flex", alignItems: "center", gap: 8, marginBottom: 12 }}>
            <span style={{ width: 9, height: 9, borderRadius: 999, background: g.dot, flex: "none" }} />
            <span style={{ fontSize: 14, fontWeight: 700, color: "#1C2333" }}>{g.key}</span>
            <span style={{ fontSize: 12, color: "#9AA1B0", fontWeight: 600 }}>{g.cards.length} 张</span>
          </div>
          <div style={{ display: "grid", gridTemplateColumns: "repeat(2,1fr)", gap: 12 }}>
            {g.cards.map((c) => (
              <div key={c.cardId} style={{ background: "#fff", border: "1px solid #EAECF2", borderRadius: 14, padding: "14px 16px" }}>
                <div style={{ display: "flex", alignItems: "flex-start", justifyContent: "space-between", gap: 10 }}>
                  <div style={{ fontSize: 14.5, fontWeight: 700, color: "#1C2333", lineHeight: 1.4 }}>{c.name}</div>
                  <span style={{ flex: "none", fontSize: 12, fontWeight: 700, color: "#5B6474", background: "#F1F2F6", borderRadius: 8, padding: "2px 8px" }}>×{c.uses}</span>
                </div>
                <div style={{ fontSize: 12.5, color: "#8A92A3", lineHeight: 1.55, marginTop: 6 }}>{c.purpose}</div>
                <div style={{ fontSize: 12, color: "#9AA1B0", marginTop: 10, fontWeight: 600 }}>{usageLine(c)}</div>
              </div>
            ))}
          </div>
        </div>
      ))}
    </div>
  );
}
```

- [ ] **Step 4: Run the test to verify it passes**

Run: `cd apps/web && npx vitest run src/shell/growth/ToolkitCards.test.tsx`
Expected: PASS (both cases).

- [ ] **Step 5: Commit**

```bash
git add apps/web/src/shell/growth/ToolkitCards.tsx apps/web/src/shell/growth/ToolkitCards.test.tsx
git commit -m "feat(toolkit): <ToolkitCards> component (category-grouped collected cards)"
```

---

### Task 6: wire 工具卡 tab into `GrowthReport`

**Files:**
- Modify: `apps/web/src/shell/growth/GrowthReport.tsx`
- Test: `apps/web/src/shell/growth/GrowthReport.test.tsx` (extend the existing tabs test)

**Interfaces:**
- Consumes: `<ToolkitCards>` from Task 5.
- Produces: a three-tab 成长报告 (学习记录 · 工具卡 · 能力素养), default `learning`.

- [ ] **Step 1: Update the failing test**

In `apps/web/src/shell/growth/GrowthReport.test.tsx`, the `GrowthReport tabs` describe block: add a `getGrowthCards` mock and assert the new tab exists and switches to the empty state. Replace the existing `"defaults to 学习记录 and switches to 能力素养"` test with:

```tsx
  it("shows all three tabs and switches between them", async () => {
    vi.spyOn(api, "getGrowthHistory").mockResolvedValue([]);
    vi.spyOn(api, "getAbilityModel").mockResolvedValue(emptyAbility as never);
    vi.spyOn(api, "getGrowthCards").mockResolvedValue([] as never);
    render(<GrowthReport />);
    expect(screen.getByRole("button", { name: /学习记录/ })).toBeTruthy();
    const cardsTab = screen.getByRole("button", { name: /工具卡/ });
    const abilityTab = screen.getByRole("button", { name: /能力素养/ });
    // default tab is the history hub
    await waitFor(() => expect(screen.getByText(/还没有报告/)).toBeTruthy());
    // 工具卡
    fireEvent.click(cardsTab);
    await waitFor(() => expect(screen.getByText(/还没有收集到工具卡/)).toBeTruthy());
    // 能力素养
    fireEvent.click(abilityTab);
    await waitFor(() => expect(screen.getByText(/还没有足够的数据/)).toBeTruthy());
  });
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `cd apps/web && npx vitest run src/shell/growth/GrowthReport.test.tsx`
Expected: FAIL — no `工具卡` button / `还没有收集到工具卡` not found.

- [ ] **Step 3: Wire the tab**

In `apps/web/src/shell/growth/GrowthReport.tsx`:

1. Add the import near the top (after the `AbilityModel` import): `import { ToolkitCards } from "./ToolkitCards";`
2. Change the tab-state type and add the middle tab button + render branch. Replace the `GrowthReport` function's body from `const [tab, setTab] = useState...` through the returned JSX:

```tsx
export function GrowthReport() {
  const [tab, setTab] = useState<"learning" | "cards" | "ability">("learning");
  const tabStyle = (active: boolean) => ({
    padding: "8px 16px", borderRadius: 999, border: "none", cursor: "pointer", fontFamily: "inherit",
    fontSize: 13.5, fontWeight: 700,
    background: active ? "#2A3B7A" : "transparent", color: active ? "#fff" : "#6B7384",
  });
  return (
    <div style={{ flex: 1, minHeight: 0, height: "100%", overflowY: "auto", background: "#F3F4F8" }}>
      <div style={{ maxWidth: 760, margin: "0 auto", padding: "34px 40px 56px" }}>
        <div style={{ display: "flex", gap: 8, marginBottom: 18 }}>
          <button type="button" style={tabStyle(tab === "learning")} onClick={() => setTab("learning")}>学习记录</button>
          <button type="button" style={tabStyle(tab === "cards")} onClick={() => setTab("cards")}>工具卡</button>
          <button type="button" style={tabStyle(tab === "ability")} onClick={() => setTab("ability")}>能力素养</button>
        </div>
        {tab === "learning" ? <LearningRecord /> : tab === "cards" ? <ToolkitCards /> : <AbilityModel />}
      </div>
    </div>
  );
}
```

- [ ] **Step 4: Run the test + full web suite + typecheck**

Run: `cd apps/web && npx vitest run src/shell/growth/GrowthReport.test.tsx && npx vitest run && npx tsc --noEmit`
Expected: target test PASS; full web suite green (prior count + the new tests); `tsc` no new errors.

- [ ] **Step 5: Commit**

```bash
git add apps/web/src/shell/growth/GrowthReport.tsx apps/web/src/shell/growth/GrowthReport.test.tsx
git commit -m "feat(toolkit): 工具卡 tab in 成长报告 (学习记录 · 工具卡 · 能力素养)"
```

---

## Notes for the whole-branch review

- **Grouping is frontend-only**; the backend returns instance-derived facts, and `CARD_REGISTRY` is the single source for name/purpose/category. Verify no card metadata leaked into the Go side.
- **Cost-free invariant**: the endpoint must make no model call — the empty-state test asserts `llm_call == 0`.
- **Owner isolation**: the query joins each scope to its owner table; the query test seeds another user's completed card and asserts exclusion.
- **RL-5 / 铁律 2**: confirm no score/level/total and no locked-catalog/progress-bar/badge-to-earn anywhere in `<ToolkitCards>`.
- **`LastUsed` type**: Task 1 records the sqlc-generated type; confirm Task 2's `.Time.Format(...)` matches it.
