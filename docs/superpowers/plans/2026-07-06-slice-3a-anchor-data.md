# Slice 3a · Anchor Data Model + Persistence — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development. Steps use checkbox (`- [ ]`) syntax.

**Goal:** Add the `Anchor` type and persist an `anchors` array on the card envelope end-to-end (Zod contract + Go/Postgres + API boundary), and add `mode` to the card spec — the data substrate the agent (3b) and the UI (3c) build on. No LLM, no UI.

**Architecture:** `Anchor` Zod + `anchors` on `CardInstance` + `mode` on `CardSpec`; a `0010_card_anchors.sql` migration adds an `anchors jsonb` column; a `SetCardAnchors` query writes it (used at summon time in 3b and at submit time here); `cards.go` validates anchors at the HTTP boundary and `putCard` persists the submitted anchors via a second `SetCardAnchors` write (so `SubmitCard`'s signature is untouched and every commit compiles).

**Tech Stack:** Zod/vitest (contracts), Go 1.26 + sqlc + goose + testcontainers (api). Contract tests: `cd packages/contracts && npx vitest run test/<f>`. Backend: from `apps/api`, `make sqlc`, `go test ./...`.

## Global Constraints

- **Backend + contracts only** (no UI). Ownership hidden as 404 via `loadOwnedTask`.
- **Boundary validation is structural** (like `event_trace`): the API validates anchor *shape*, not that `material_id`/`block_id` exist. Deep truth stays in the Zod contract.
- Anchor shape (identical across Zod + Go validation + tests): `{ id:string, material_id:string, block_id:string, start:number(int≥0), end:number(int≥0), quote:string, dimension:string, author:'ai'|'student', question:string, answer:string }`.
- `mode` ∈ `{annotation, form}`, default `form`; existing card JSONs (no `mode`) parse as `form`.
- Contract↔DTO parity is manual — `cardDTO.anchors` JSON tag must match the Zod `CardInstance.anchors`.
- Every task ends green + committed; `make sqlc`-generated names are the source of truth.

---

### Task 1: `Anchor` contract + envelope/spec fields

**Files:**
- Create: `packages/contracts/src/anchor.ts`
- Modify: `packages/contracts/src/envelope.ts`, `packages/contracts/src/cardSpec.ts`, `packages/contracts/src/index.ts`
- Test: `packages/contracts/test/anchor.test.ts`

**Interfaces:**
- Produces: `Anchor`, `AnchorAuthor` Zod + types; `CardInstance.anchors: Anchor[]` (defaults `[]`); `CardSpec.mode: 'annotation'|'form'` (defaults `form`).

- [ ] **Step 1: Write the failing test**

Create `packages/contracts/test/anchor.test.ts`:

```ts
import { describe, it, expect } from "vitest";
import { Anchor } from "../src/anchor";
import { CardInstance } from "../src/envelope";
import { CardSpec } from "../src/cardSpec";

const anchor = {
  id: "a0", material_id: "m1", block_id: "b0", start: 3, end: 9,
  quote: "美航局发现", dimension: "权威性 · Authority", author: "ai",
  question: "这处「美航局发现」——转载者是权威吗？", answer: "",
};

describe("Anchor + envelope/spec extensions", () => {
  it("parses a valid anchor", () => {
    expect(Anchor.parse(anchor)).toMatchObject({ id: "a0", author: "ai" });
  });
  it("rejects an unknown author", () => {
    expect(Anchor.safeParse({ ...anchor, author: "teacher" }).success).toBe(false);
  });
  it("CardInstance.anchors defaults to [] when omitted", () => {
    const ci = CardInstance.parse({
      id: "c1", card_id: "sift_craap", task_id: "t1", parent_node_id: null,
      status: "proposed", field_values: {}, event_trace: [], rubric_tags: [],
      created_at: "1", completed_at: null,
    });
    expect(ci.anchors).toEqual([]);
  });
  it("CardInstance carries anchors when present", () => {
    const ci = CardInstance.parse({
      id: "c1", card_id: "sift_craap", task_id: "t1", parent_node_id: null,
      status: "completed", field_values: {}, event_trace: [], rubric_tags: [],
      anchors: [anchor], created_at: "1", completed_at: "2",
    });
    expect(ci.anchors).toHaveLength(1);
  });
  it("CardSpec.mode defaults to form", () => {
    const spec = CardSpec.parse({
      id: "x", category: "c", name: "n", purpose: "p", trigger_condition: "t",
      steps: [{ key: "s", title: "T", disclose: "always", methodology: { why: "w", how: "h", when: "n" }, fields: [{ type: "text", key: "f", label: "L" }] }],
      rubric_tags: [],
    });
    expect(spec.mode).toBe("form");
  });
});
```

- [ ] **Step 2: Run — expect FAIL**

Run: `cd packages/contracts && npx vitest run test/anchor.test.ts`
Expected: FAIL — `Failed to resolve import "../src/anchor"`.

- [ ] **Step 3: Create `anchor.ts`**

Create `packages/contracts/src/anchor.ts`:

```ts
import { z } from "zod";

export const AnchorAuthor = z.enum(["ai", "student"]);

export const Anchor = z.object({
  id: z.string(),
  material_id: z.string(),
  block_id: z.string(),
  start: z.number().int().nonnegative(),
  end: z.number().int().nonnegative(),
  quote: z.string(),
  dimension: z.string(),
  author: AnchorAuthor,
  question: z.string(),
  answer: z.string(),
});

export type AnchorAuthor = z.infer<typeof AnchorAuthor>;
export type Anchor = z.infer<typeof Anchor>;
```

- [ ] **Step 4: Extend envelope + spec + barrel**

4a. In `packages/contracts/src/envelope.ts`: add `import { Anchor } from "./anchor";` at the top, and add this line to the `CardInstance` object after `event_trace: z.array(TraceEvent),`:

```ts
  anchors: z.array(Anchor).default([]),
```

4b. In `packages/contracts/src/cardSpec.ts`: add this line to the `CardSpec` object after `rubric_tags: z.array(z.string()),`:

```ts
  mode: z.enum(["annotation", "form"]).default("form"),
```

4c. In `packages/contracts/src/index.ts`: add after `export * from "./envelope";`:

```ts
export * from "./anchor";
```

- [ ] **Step 5: Run — expect PASS**

Run: `cd packages/contracts && npx vitest run test/anchor.test.ts && npx tsc --noEmit`
Expected: PASS (5 tests); tsc clean.

- [ ] **Step 6: Commit**

```bash
git add packages/contracts/src/anchor.ts packages/contracts/src/envelope.ts packages/contracts/src/cardSpec.ts packages/contracts/src/index.ts packages/contracts/test/anchor.test.ts
git commit -m "feat(contracts): Anchor type + CardInstance.anchors + CardSpec.mode"
```

---

### Task 2: DB — anchors column, SetCardAnchors, sqlc, DTO, loader mode

**Files:**
- Create: `apps/api/internal/store/migrations/0010_card_anchors.sql`
- Modify: `apps/api/internal/store/queries/cards.sql` (+ `SetCardAnchors`)
- Generated (via `make sqlc`): `apps/api/internal/store/sqlc/cards.sql.go`, `models.go`
- Modify: `apps/api/internal/api/dto.go` (`cardDTO.anchors`), `apps/api/internal/cards/loader.go` (`Spec.Mode`)
- Test: `apps/api/internal/api/anchors_store_test.go`

**Interfaces:**
- Produces (sqlc): `sqlc.CardInstance.Anchors []byte`; `SetCardAnchors(ctx, SetCardAnchorsParams{ID,TaskID,Anchors}) (CardInstance, error)`.
- Produces: `cardDTO.Anchors json.RawMessage` (json tag `anchors`, empty→`[]`); `cards.Spec.Mode string`.

- [ ] **Step 1: Write the migration**

Create `apps/api/internal/store/migrations/0010_card_anchors.sql`:

```sql
-- +goose Up
ALTER TABLE card_instances ADD COLUMN anchors jsonb NOT NULL DEFAULT '[]';

-- +goose Down
ALTER TABLE card_instances DROP COLUMN anchors;
```

- [ ] **Step 2: Add the query**

In `apps/api/internal/store/queries/cards.sql`, append:

```sql
-- name: SetCardAnchors :one
UPDATE card_instances SET anchors = $3
WHERE id = $1 AND task_id = $2
RETURNING *;
```

- [ ] **Step 3: Generate sqlc**

Run: `cd apps/api && make sqlc`
Expected: no errors; `SetCardAnchors`/`SetCardAnchorsParams` generated; `sqlc.CardInstance` now has an `Anchors []byte` field. (All the existing `RETURNING *` card queries now also return `anchors`.)

- [ ] **Step 4: Add the DTO field**

In `apps/api/internal/api/dto.go`, in `cardDTO` add after `EventTrace`:

```go
	Anchors json.RawMessage `json:"anchors"`
```

and in `toCardDTO`, set it (mirroring the empty-jsonb coercion pattern) right before `return d`:

```go
	d.Anchors = json.RawMessage(c.Anchors)
	if len(d.Anchors) == 0 {
		d.Anchors = json.RawMessage("[]")
	}
```

(Add `Anchors` to the initial `cardDTO{...}` literal is unnecessary — set it after, like `ParentNodeID`/`CompletedAt`. Ensure the `d :=` literal still compiles.)

- [ ] **Step 5: Add `Mode` to the loader Spec**

In `apps/api/internal/cards/loader.go`, add to the `Spec` struct (after `InteractionType`):

```go
	Mode             string `json:"mode"`
```

- [ ] **Step 6: Write the round-trip test**

Create `apps/api/internal/api/anchors_store_test.go`:

```go
package api_test

import (
	"bytes"
	"context"
	"testing"

	"mindimprint/api/internal/store/sqlc"
)

func TestSetCardAnchorsRoundTrip(t *testing.T) {
	pool := newAPITestPool(t)
	q := sqlc.New(pool)
	ctx := context.Background()

	task, err := q.CreateTask(ctx, sqlc.CreateTaskParams{UserID: mustUUID(SeedUserID.String()), Title: "anchors rt"})
	if err != nil {
		t.Fatal(err)
	}
	card, err := q.CreateCardInstance(ctx, sqlc.CreateCardInstanceParams{CardID: "sift_craap", TaskID: task.ID})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(card.Anchors, []byte("[]")) {
		t.Fatalf("default anchors want []; got %s", card.Anchors)
	}

	anchors := []byte(`[{"id":"a0","material_id":"m1","block_id":"b0","start":0,"end":5,"quote":"美航局发现","dimension":"权威性","author":"ai","question":"可信吗？","answer":""}]`)
	upd, err := q.SetCardAnchors(ctx, sqlc.SetCardAnchorsParams{ID: card.ID, TaskID: task.ID, Anchors: anchors})
	if err != nil {
		t.Fatalf("set anchors: %v", err)
	}
	if !bytes.Contains(upd.Anchors, []byte("美航局发现")) {
		t.Fatalf("anchors not persisted: %s", upd.Anchors)
	}
}
```

- [ ] **Step 7: Run — expect PASS + build**

Run: `cd apps/api && go build ./... && go test ./internal/api/ -run TestSetCardAnchorsRoundTrip`
Expected: build clean; test PASS (testcontainers).

- [ ] **Step 8: Commit**

```bash
git add apps/api/internal/store/migrations/0010_card_anchors.sql apps/api/internal/store/queries/cards.sql apps/api/internal/store/sqlc/ apps/api/internal/api/dto.go apps/api/internal/cards/loader.go apps/api/internal/api/anchors_store_test.go
git commit -m "feat(api): card_instances.anchors column + SetCardAnchors + DTO + loader mode"
```

---

### Task 3: HTTP boundary — validate + persist submitted anchors

**Files:**
- Modify: `apps/api/internal/api/cards.go` (`validateAnchors`, `putCard` persists anchors)
- Test: `apps/api/internal/api/anchors_handler_test.go`

**Interfaces:**
- Consumes: `SetCardAnchors` (Task 2), `toCardDTO` w/ anchors (Task 2).
- Produces: `putCard` accepts an `anchors` array in the body, validates it structurally, and persists it (via `SetCardAnchors` after `SubmitCard`); the response `card` DTO carries the persisted anchors.

- [ ] **Step 1: Write the handler test**

Create `apps/api/internal/api/anchors_handler_test.go`:

```go
package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	. "mindimprint/api/internal/api"
	"mindimprint/api/internal/store/sqlc"
)

func submitBodyWithAnchors(anchors string) []byte {
	return []byte(`{"status":"completed","field_values":{},"event_trace":[{"kind":"submit","at":"1"}],"anchors":` + anchors + `}`)
}

func TestPutCardPersistsAnchors(t *testing.T) {
	pool := newAPITestPool(t)
	q := sqlc.New(pool)
	task, _ := q.CreateTask(context.Background(), sqlc.CreateTaskParams{UserID: mustUUID(SeedUserID.String()), Title: "t"})
	card, _ := q.CreateCardInstance(context.Background(), sqlc.CreateCardInstanceParams{CardID: "sift_craap", TaskID: task.ID})
	h := New(Deps{Queries: q, Pool: pool}).Handler()
	cookie := signInSeed(t, pool)
	url := "/api/v1/tasks/" + task.ID.String() + "/cards/" + card.ID.String()

	anchors := `[{"id":"a0","material_id":"m1","block_id":"b0","start":0,"end":5,"quote":"美航局发现","dimension":"权威性","author":"student","question":"我不信这句","answer":"因为没出处"}]`
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("PUT", url, bytes.NewReader(submitBodyWithAnchors(anchors))), cookie))
	if rec.Code != http.StatusOK {
		t.Fatalf("submit: %d %s", rec.Code, rec.Body)
	}
	var resp struct {
		Card struct {
			Status  string          `json:"status"`
			Anchors json.RawMessage `json:"anchors"`
		} `json:"card"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &resp)
	if resp.Card.Status != "completed" || !bytes.Contains(resp.Card.Anchors, []byte("我不信这句")) {
		t.Fatalf("anchors not persisted in response: %s", rec.Body)
	}
}

func TestPutCardRejectsBadAnchors(t *testing.T) {
	pool := newAPITestPool(t)
	q := sqlc.New(pool)
	task, _ := q.CreateTask(context.Background(), sqlc.CreateTaskParams{UserID: mustUUID(SeedUserID.String()), Title: "t"})
	card, _ := q.CreateCardInstance(context.Background(), sqlc.CreateCardInstanceParams{CardID: "sift_craap", TaskID: task.ID})
	h := New(Deps{Queries: q, Pool: pool}).Handler()
	cookie := signInSeed(t, pool)
	url := "/api/v1/tasks/" + task.ID.String() + "/cards/" + card.ID.String()

	// author not in {ai,student}
	bad := `[{"id":"a0","material_id":"m1","block_id":"b0","start":0,"end":5,"quote":"x","dimension":"d","author":"teacher","question":"q","answer":""}]`
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("PUT", url, bytes.NewReader(submitBodyWithAnchors(bad))), cookie))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("want 400 for bad anchor author, got %d %s", rec.Code, rec.Body)
	}
}
```

- [ ] **Step 2: Run — expect FAIL**

Run: `cd apps/api && go test ./internal/api/ -run TestPutCard`
Expected: FAIL — `putCard` ignores `anchors` (persists nothing / doesn't validate) so both assertions fail.

- [ ] **Step 3: Add `validateAnchors` + wire `putCard`**

In `apps/api/internal/api/cards.go`:

3a. Add the validator (next to `validateEventTrace`):

```go
// validateAnchors requires a JSON array whose every element is a well-formed
// anchor (structural check only — existence of material/block is not verified).
func validateAnchors(raw json.RawMessage) error {
	if len(raw) == 0 {
		return nil // absent anchors is allowed (defaults to [])
	}
	var arr []map[string]any
	if json.Unmarshal(raw, &arr) != nil {
		return httpx.ErrBadRequest("validation_failed", "anchors 必须是数组", nil)
	}
	for _, a := range arr {
		author, _ := a["author"].(string)
		if author != "ai" && author != "student" {
			return httpx.ErrBadRequest("validation_failed", "anchors.author 非法", nil)
		}
		for _, k := range []string{"block_id", "dimension", "question", "quote", "answer", "material_id", "id"} {
			if _, ok := a[k].(string); !ok {
				return httpx.ErrBadRequest("validation_failed", "anchors 字段缺失或类型错误", nil)
			}
		}
		if _, ok := a["start"].(float64); !ok {
			return httpx.ErrBadRequest("validation_failed", "anchors.start 必须是数字", nil)
		}
		if _, ok := a["end"].(float64); !ok {
			return httpx.ErrBadRequest("validation_failed", "anchors.end 必须是数字", nil)
		}
	}
	return nil
}
```

3b. In `putCard`, add `Anchors json.RawMessage \`json:"anchors"\`` to the request `body` struct; after the existing `validateEventTrace` check, add:

```go
	if err := validateAnchors(body.Anchors); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
```

3c. In `putCard`, replace the tail (from the `SubmitCard` call through `writeCardOrNotFound`) with a version that persists anchors after a successful submit:

```go
	c, err := a.d.Queries.SubmitCard(r.Context(), sqlc.SubmitCardParams{
		ID: cardID, TaskID: taskID,
		FieldValues: []byte(body.FieldValues), EventTrace: []byte(body.EventTrace),
	})
	if err != nil {
		writeCardOrNotFound(w, r, c, err)
		return
	}
	anchors := body.Anchors
	if len(anchors) == 0 {
		anchors = json.RawMessage("[]")
	}
	c, err = a.d.Queries.SetCardAnchors(r.Context(), sqlc.SetCardAnchorsParams{
		ID: cardID, TaskID: taskID, Anchors: []byte(anchors),
	})
	if err == nil {
		a.maybeTriggerMilestoneEval(r.Context(), taskID)
	}
	writeCardOrNotFound(w, r, c, err)
```

(This keeps `SubmitCard` unchanged and adds one `SetCardAnchors` write; the final response row carries `status:'completed'` + the persisted anchors.)

- [ ] **Step 4: Run — expect PASS**

Run: `cd apps/api && go test ./internal/api/ -run TestPutCard`
Expected: PASS (both cases).

- [ ] **Step 5: Full backend build + vet + short**

Run: `cd apps/api && go build ./... && go vet ./... && go test -short ./...`
Expected: clean; short tests pass.

- [ ] **Step 6: Commit**

```bash
git add apps/api/internal/api/cards.go apps/api/internal/api/anchors_handler_test.go
git commit -m "feat(api): validate + persist submitted card anchors in putCard"
```

---

## Self-Review

**Spec coverage:** `Anchor` type + `anchors` on envelope + `mode` on spec (T1) ✅; `anchors` column + `SetCardAnchors` + DTO + loader `Mode` (T2) ✅; boundary validation + submit-persist (T3) ✅. No LLM/UI (correct for 3a).

**Placeholder scan:** No TBD; complete code / exact edits everywhere.

**Type consistency:** Anchor shape identical in Zod (T1), the Go `validateAnchors` field checks (T3), and the round-trip/handler test fixtures (T2/T3). `SetCardAnchorsParams{ID,TaskID,Anchors}` defined T2, consumed T3. `cardDTO.anchors` (T2) asserted in the handler test (T3). `SubmitCard` signature unchanged → no cross-task compile break.

**Ordering:** T1 contracts → T2 DB (independent) → T3 handler (consumes T2). `putCard` compiles at each step (T2 doesn't touch it; T3 adds anchors handling).

## Note for executor

- If `make sqlc` names the generated field `Anchors` differently, use the generated name. `putCard` currently ends with `SubmitCard(...)` + `maybeTriggerMilestoneEval` + `writeCardOrNotFound`; T3 step 3c replaces exactly that tail — preserve the surrounding handler (ownership, status/field/trace validation) unchanged.
