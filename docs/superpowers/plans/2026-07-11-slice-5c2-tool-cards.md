# Slice 5c-2 — CRAAP Tool-Card Round-Trip Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** The agent surfaces the CRAAP source-evaluation card; the student fills it (reusing the existing schema-driven Card Runtime); submitting persists the filled envelope, runs `CompleteCard` to mint an evidence node + `evaluated-as` edge, and refeeds one `RunAgentStep` so the coach reacts and the `every_source_evaluated` gate flips live. One card (CRAAP); debt-free.

**Architecture:** Flip `SkipSurfaceCards` off in the studio turn sites so `RunAgentStep` surfaces the CRAAP card as a `card` SSE event. The frontend reuses the schema-driven Card Runtime (`CardSheetHost`/`CardRenderer`/`pickCardBody`/`envelopeReducer`), project-scoped. Submit is an SSE endpoint that persists the filled envelope, runs `CompleteCard` (mints the evidence node via `graph_effects`), then drives one refeed `RunAgentStep` streaming the coach's reaction — mirroring `postProjectTurn`.

**Tech Stack:** Go (`net/http`, `pgx`/sqlc, testcontainers, SSE) · TypeScript + React + vitest (`apps/web`).

**Spec:** `docs/superpowers/specs/2026-07-11-slice-5c2-tool-cards-design.md`

## Global Constraints

- **Client never calls the model directly** — surface/refeed turns go through the Go gateway; keys server-side only.
- **AI restraint / 一次只问一个:** the refeed is exactly ONE `RunAgentStep`; the coach may stay silent. **The card triggers automatically but the student confirms "opening"** (proposal bubble, no forced pop-up).
- **AI never executes the tool for the student.** The card is filled by the STUDENT via the Card Runtime; the model never writes `field_values`/anchors. `CompleteCard` mints graph nodes from the STUDENT's anchors (`Author:"student"`). Enforcement stack unchanged.
- **过程即数据:** the fill `event_trace` AND a skip are persisted as signals — skipping is recorded, not discarded.
- **Single source of truth:** reuse `SurfaceCard`/`CompleteCard`/`GraphEffects`/`EvaluateCompletion` and the existing frontend Card Runtime — do NOT fork a parallel renderer or re-implement the mint logic. The card JSON registry (`packages/contracts/cards/`) is the single card-spec source. **No renderer special-casing** — CRAAP already has its `pickCardBody` renderer + `craap.json`.
- **Additive; legacy untouched.** No schema migration. Legacy `postTurn`/`RunTurn`/task-scoped card endpoints (`/tasks/{id}/cards/{cid}`) stay live (retired in 5d). `SkipSurfaceCards` is set explicitly (false) at the studio turn sites; the field's zero value still preserves other callers.
- **Studio discipline:** new endpoints + frontend card host are additive; do NOT touch `AppShell`/`Root`/old `workspace/`. **`nudge_text` on the surfaced card is static/derived (NOT model-generated)** — `SurfaceCard` is model-free; the coach's dynamic reaction comes on the refeed turn.
- **Icons = inline SVG, never `lucide-react`. Binding Chinese design copy verbatim — fix the test, never the copy.**
- **Commands:** `make sqlc` (in `apps/api`); Go tests `go test ./...` (serialized `-p 1` for the full suite — parallel hangs on Docker contention on this machine) / `-short`; web tests `npm test` in `apps/web`. The project-boundary hook BLOCKS shell redirects to `/dev/null`, out-of-repo paths, and the bare token `eval`.

## File Structure

- **`apps/api/internal/store/queries/card_instance.sql`** (modify — add `SubmitProjectCardInstance`) + regenerated sqlc. **store round-trip test.**
- **`apps/api/internal/agent/loop.go`** (modify — `AgentStore` seam + methods) · **`agentstore.go`** (adapter) · **`loop_test.go`** (fake) · **`runtime.go`** (`Action.CardID`) · the surface_card branch sets `Action.CardID`.
- **`apps/api/internal/api/studioturn.go`** (modify — `SkipSurfaceCards:false` + `surface_card` case + `studioEmitter.Card`) + test.
- **`apps/api/internal/api/projectcards.go`** (new — activate/submit(SSE)/skip) + **`api.go`** (routes) + **`projectcards_test.go`**.
- **`apps/web/src/api/studioTurn.ts`** (modify — `card` variant) + test.
- **`apps/web/src/api/projectCards.ts`** (new) + **`api/index.ts`** (aggregate) + test.
- **`apps/web/src/studio/conversation.ts`** (modify — card state) + test.
- **`apps/web/src/studio/CoachRail.tsx`** (modify — live card slot) · **`StudioContainer.tsx`** (wire) · **`StudioShell.tsx`** (thread) · **`state.ts`** (callbacks) + tests.

---

### Task 1: `SubmitProjectCardInstance` sqlc query (field_values + event_trace)

**Files:** Modify `apps/api/internal/store/queries/card_instance.sql` · regenerate sqlc · Test `apps/api/internal/store/card_instance_sqlc_test.go` (add a case, or a new file).

**Why:** `SetCardInstanceStatus`/`SetCardInstanceAnchors`/`SetCardInstanceFramework` exist project-scoped, but there is NO project-scoped writer for `field_values`+`event_trace`. Add one.

- [ ] **Step 1: Write the failing test** — a testcontainers round-trip (reuse `newStoreTestPool`, `-short`-gated):

```go
func TestSubmitProjectCardInstance(t *testing.T) {
	if testing.Short() { t.Skip("requires postgres") }
	pool := newStoreTestPool(t)
	q := sqlc.New(pool)
	ctx := context.Background()
	// seeded project …0101 + demo task …0100 (migration 0018)
	ci, err := q.CreateProjectCardInstance(ctx, sqlc.CreateProjectCardInstanceParams{
		TaskID:    uuid.MustParse("00000000-0000-0000-0000-000000000100"),
		ProjectID: pgUUID(uuid.MustParse("00000000-0000-0000-0000-000000000101")),
		CardID:    "craap", ContractRef: strptr("evaluate_sources"), Status: "active",
	})
	if err != nil { t.Fatal(err) }
	got, err := q.SubmitProjectCardInstance(ctx, sqlc.SubmitProjectCardInstanceParams{
		ID:          ci.ID,
		ProjectID:   pgUUID(uuid.MustParse("00000000-0000-0000-0000-000000000101")),
		FieldValues: []byte(`{"authority_verdict":"存疑"}`),
		EventTrace:  []byte(`[{"kind":"submit","at":"2026-07-12T00:00:00Z"}]`),
	})
	if err != nil { t.Fatal(err) }
	if string(got.FieldValues) != `{"authority_verdict": "存疑"}` && string(got.FieldValues) == "" {
		t.Fatalf("field_values not persisted: %s", got.FieldValues)
	}
	if len(got.EventTrace) == 0 { t.Fatalf("event_trace not persisted") }
}
```

*(Implementer: reuse the store package's `pgUUID`/`strptr` helpers if present, else inline `pgtype.UUID{Bytes:…,Valid:true}` / a `*string`. jsonb reorders keys — assert non-empty / contains, not byte-exact.)*

- [ ] **Step 2: Run — verify it fails** (`cd apps/api && go test ./internal/store/ -run TestSubmitProjectCardInstance` → undefined).

- [ ] **Step 3: Add the query** to `card_instance.sql`:

```sql
-- name: SubmitProjectCardInstance :one
UPDATE card_instances SET field_values = $3, event_trace = $4
WHERE id = $1 AND project_id = $2
RETURNING *;
```

- [ ] **Step 4: Regenerate + run** (`cd apps/api && make sqlc && go test ./internal/store/ -run TestSubmitProjectCardInstance` → PASS).

- [ ] **Step 5: Commit** (`feat(store): SubmitProjectCardInstance query (project-scoped field_values/event_trace)`).

---

### Task 2: `AgentStore` seam — status/anchors/submission + `Action.CardID`

**Files:** Modify `apps/api/internal/agent/loop.go` (interface), `agentstore.go` (adapter), `loop_test.go` (fake), `runtime.go` (`Action.CardID`), `card_lifecycle.go` (`SurfaceCard` sets `Action.CardID`). Test `agentstore_cards_sqlc_test.go` + a `runtime`/`loop` test for `Action.CardID`.

**Interfaces produced:** seam methods `SetCardInstanceStatus(ctx, projectID, id uuid.UUID, status string) error`, `SetCardInstanceAnchors(ctx, projectID, id uuid.UUID, anchors []byte) error`, `SubmitProjectCardInstance(ctx, projectID, id uuid.UUID, fieldValues, eventTrace []byte) error`; `Action.CardID string`. Consumed by Tasks 3/5.

- [ ] **Step 1: Write the failing tests** — (a) seam round-trip (testcontainers): create a card_instance, `SetCardInstanceStatus(…,"active")`, `SetCardInstanceAnchors(…, json)`, `SubmitProjectCardInstance(…, fv, et)`, then `GetCardInstance` reflects them. (b) `Action.CardID` — a `RunAgentStep` (or direct `SurfaceCard`) on a graph with an un-evaluated article material returns `Action{Kind:"surface_card", CardID:"craap"}`:

```go
func TestSurfaceCard_SetsActionCardID(t *testing.T) {
	// reuse the loop-test fake + a graph with one un-evaluated article material
	g := GraphView{Materials: []MaterialView{{ID: uuid.NewString(), Kind: "article"}}}
	deps := newFakeDeps(g) // SkipSurfaceCards defaults false
	act, err := RunAgentStep(context.Background(), deps, testProjectID, Trigger{Kind: "student_turn"})
	if err != nil { t.Fatal(err) }
	if act == nil || act.Kind != "surface_card" || act.CardID != "craap" {
		t.Fatalf("want surface_card craap, got %+v", act)
	}
}
```

*(Implementer: adapt to the real loop-test fixtures/fakes — the fake `CreateCardInstance` must return a `CardInstanceRow` for `SurfaceCard` to work. Reuse the surface_card fixture from `TestRunAgentStep_SkipSurfaceCards` (Slice 5c).)*

- [ ] **Step 2: Run — verify it fails.**

- [ ] **Step 3: Add `CardID` to `Action`** (`runtime.go`): `CardID string // set when Kind == "surface_card"`.

- [ ] **Step 4: `SurfaceCard` sets it** (`card_lifecycle.go`, the return): `return &Action{Kind: "surface_card", CardInstanceID: row.ID.String(), CardID: spec.ID}, nil`.

- [ ] **Step 5: Add the three seam methods** — to the `AgentStore` interface (`loop.go`) and the `sqlcAgentStore` adapter (`agentstore.go`, mirroring `SetCardInstanceFramework`):

```go
func (s *sqlcAgentStore) SetCardInstanceStatus(ctx context.Context, projectID, id uuid.UUID, status string) error {
	_, err := s.q.SetCardInstanceStatus(ctx, sqlc.SetCardInstanceStatusParams{ID: id, ProjectID: pgtype.UUID{Bytes: projectID, Valid: true}, Status: status})
	return err
}
func (s *sqlcAgentStore) SetCardInstanceAnchors(ctx context.Context, projectID, id uuid.UUID, anchors []byte) error {
	_, err := s.q.SetCardInstanceAnchors(ctx, sqlc.SetCardInstanceAnchorsParams{ID: id, ProjectID: pgtype.UUID{Bytes: projectID, Valid: true}, Anchors: anchors})
	return err
}
func (s *sqlcAgentStore) SubmitProjectCardInstance(ctx context.Context, projectID, id uuid.UUID, fieldValues, eventTrace []byte) error {
	_, err := s.q.SubmitProjectCardInstance(ctx, sqlc.SubmitProjectCardInstanceParams{ID: id, ProjectID: pgtype.UUID{Bytes: projectID, Valid: true}, FieldValues: fieldValues, EventTrace: eventTrace})
	return err
}
```

- [ ] **Step 6: Add the three methods to `fakeAgentStore`** (`loop_test.go`) — minimal in-memory (record calls; keep existing behavior).

- [ ] **Step 7: Run** (`cd apps/api && go test ./internal/agent/`) → PASS.

- [ ] **Step 8: Commit** (`feat(agent): card seam (status/anchors/submission) + Action.CardID`).

---

### Task 3: Studio turn surfaces the card (`studioturn.go`)

**Files:** Modify `apps/api/internal/api/studioturn.go` (`SkipSurfaceCards:false` + `surface_card` case + `studioEmitter.Card`). Test `studioturn_test.go` (add a card-surfacing case).

- [ ] **Step 1: Write the failing test** — the seeded project's article materials are un-evaluated, so a turn now surfaces CRAAP:

```go
func TestProjectTurn_SurfacesCraapCard(t *testing.T) {
	pool := newAPITestPool(t)
	h := New(Deps{Queries: sqlc.New(pool), Pool: pool, Provider: fakeProvider(), ChatResolver: fakeResolver(), SpecByID: cardsByID()}).Handler()
	cookie := signInSeed(t, pool)
	rr := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/v1/projects/00000000-0000-0000-0000-000000000101/turn", strings.NewReader(`{"user_input":"这条来源可信吗"}`))
	h.ServeHTTP(withCookieResp(rr), withCookie(req, cookie))
	body := rr.Body.String()
	if !strings.Contains(body, "event: card") || !strings.Contains(body, `"card_id":"craap"`) {
		t.Fatalf("expected a craap card event:\n%s", body)
	}
	// a card_instance (proposed) was persisted for the project
	cis, _ := sqlc.New(pool).ListCardInstancesByProject(context.Background(), pgUUID(uuid.MustParse("00000000-0000-0000-0000-000000000101")))
	if len(cis) == 0 { t.Fatalf("no card_instance persisted") }
}
```

*(Implementer: `withCookieResp` is illustrative — just use the recorder. Confirm the seed's materials are `kind='article'` + have NO `evaluated-as` edge (they don't in migration 0018) so `SurfaceCardCandidates` fires; if a prior test in the file leaves the project with an intervention-only expectation, note that flipping `SkipSurfaceCards` changes it — update those expectations to match the now-card-surfacing seed, OR assert on `event: done` where a card isn't guaranteed. Verify the existing 5c `TestProjectTurn` still passes or adjust it: with cards on, the seeded first turn surfaces a card rather than an intervention.)*

- [ ] **Step 2: Run — verify it fails** (no `surface_card` case → no card event).

- [ ] **Step 3: Add `Card` to `studioEmitter`** (mutex-delegating, like the others):

```go
func (e *studioEmitter) Card(cardInstanceID, cardID, nudgeText string, anchors []byte) error {
	e.mu.Lock(); defer e.mu.Unlock(); return e.sse.Card(cardInstanceID, cardID, nudgeText, anchors)
}
```

- [ ] **Step 4: Flip the flag + add the case** — in `postProjectTurn`: `SkipSurfaceCards: false`; add to the switch:

```go
	case action.Kind == "surface_card":
		spec, _ := cards.ByID(action.CardID)
		_ = em.Card(action.CardInstanceID, action.CardID, spec.Name, []byte("[]"))
```
(`nudge_text = spec.Name` — a static, spec-derived label; `SurfaceCard` is model-free, so no dynamic nudge. anchors empty — the student fills them.)

- [ ] **Step 5: Run** (`cd apps/api && go test ./internal/api/ -run TestProjectTurn`) → PASS (new + adjusted 5c cases).

- [ ] **Step 6: Commit** (`feat(api): studio turn surfaces the craap card (SkipSurfaceCards off + card SSE case)`).

---

### Task 4: Project card `activate` + `skip` endpoints

**Files:** Create `apps/api/internal/api/projectcards.go` (activate + skip) · Modify `api.go` (routes) · Test `apps/api/internal/api/projectcards_test.go`.

**Interfaces produced:** `POST /projects/{id}/cards/{cid}/activate`, `POST /projects/{id}/cards/{cid}/skip`. A shared `loadOwnedProjectCard(w, r) (projectID, cid uuid.UUID, ok bool)` helper (mirrors `disposition.go`'s membership pattern via `ListCardInstancesByProject`). Consumed by Task 5 (submit reuses the helper).

- [ ] **Step 1: Write the failing test** — activate (204) + skip (204) + 404 for a card not in the project:

```go
func TestProjectCardActivateSkip(t *testing.T) {
	pool := newAPITestPool(t)
	h := New(Deps{Queries: sqlc.New(pool), Pool: pool}).Handler()
	cookie := signInSeed(t, pool)
	// surface a card first via the seam, or use a seeded card_instance id.
	cid := createProjectCardForTest(t, pool) // helper: CreateProjectCardInstance on project …0101
	base := "/api/v1/projects/00000000-0000-0000-0000-000000000101/cards/" + cid

	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, withCookie(httptest.NewRequest("POST", base+"/activate", nil), cookie))
	if rr.Code != 204 { t.Fatalf("activate: %d", rr.Code) }

	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, withCookie(httptest.NewRequest("POST", base+"/skip", strings.NewReader(`{"event_trace":[{"kind":"skip","at":"2026-07-12T00:00:00Z"}]}`)), cookie))
	if rr.Code != 204 { t.Fatalf("skip: %d — %s", rr.Code, rr.Body.String()) }

	// 404 for a random (non-project) card id.
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, withCookie(httptest.NewRequest("POST", "/api/v1/projects/00000000-0000-0000-0000-000000000101/cards/"+uuid.NewString()+"/activate", nil), cookie))
	if rr.Code != 404 { t.Fatalf("foreign card: %d", rr.Code) }
}
```

- [ ] **Step 2: Run — verify it fails** (routes 404).

- [ ] **Step 3: Write `projectcards.go`** (activate + skip + the shared helper):

```go
package api

import (
	"net/http"

	"github.com/google/uuid"
	"mindimprint/api/internal/agent"
	"mindimprint/api/internal/httpx"
)

// loadOwnedProjectCard scopes {cid} to the owned {id} project (404-no-leak),
// mirroring disposition.go's ownership+membership pattern.
func (a *API) loadOwnedProjectCard(w http.ResponseWriter, r *http.Request) (uuid.UUID, uuid.UUID, bool) {
	projectID, ok := a.loadOwnedProject(w, r)
	if !ok { return uuid.UUID{}, uuid.UUID{}, false }
	cid, err := uuid.Parse(r.PathValue("cid"))
	if err != nil { httpx.WriteError(w, r, httpx.ErrNotFound("资源不存在")); return uuid.UUID{}, uuid.UUID{}, false }
	cis, err := a.d.Queries.ListCardInstancesByProject(r.Context(), pgUUID(projectID))
	if err != nil { httpx.WriteError(w, r, err); return uuid.UUID{}, uuid.UUID{}, false }
	for _, ci := range cis {
		if ci.ID == cid { return projectID, cid, true }
	}
	httpx.WriteError(w, r, httpx.ErrNotFound("资源不存在"))
	return uuid.UUID{}, uuid.UUID{}, false
}

func (a *API) activateProjectCard(w http.ResponseWriter, r *http.Request) {
	projectID, cid, ok := a.loadOwnedProjectCard(w, r)
	if !ok { return }
	store := agent.NewSqlcAgentStore(a.d.Queries)
	if err := store.SetCardInstanceStatus(r.Context(), projectID, cid, "active"); err != nil {
		httpx.WriteError(w, r, err); return
	}
	_ = store.AppendEvent(r.Context(), agent.EventRow{ProjectID: projectID, Surface: "studio", Type: "card_activated", Payload: []byte(`{}`)})
	w.WriteHeader(http.StatusNoContent)
}

func (a *API) skipProjectCard(w http.ResponseWriter, r *http.Request) {
	projectID, cid, ok := a.loadOwnedProjectCard(w, r)
	if !ok { return }
	var body struct{ EventTrace json.RawMessage `json:"event_trace"` }
	if err := decodeJSON(r, &body); err != nil { httpx.WriteError(w, r, err); return }
	if err := validateEventTrace(body.EventTrace); err != nil { httpx.WriteError(w, r, err); return }
	store := agent.NewSqlcAgentStore(a.d.Queries)
	if err := store.SubmitProjectCardInstance(r.Context(), projectID, cid, []byte("{}"), body.EventTrace); err != nil {
		httpx.WriteError(w, r, err); return
	}
	if err := store.SetCardInstanceStatus(r.Context(), projectID, cid, "skipped"); err != nil {
		httpx.WriteError(w, r, err); return
	}
	_ = store.AppendEvent(r.Context(), agent.EventRow{ProjectID: projectID, Surface: "studio", Type: "card_skipped", Payload: []byte(`{}`)})
	w.WriteHeader(http.StatusNoContent)
}
```

*(Implementer: `pgUUID` is the local `pgtype.UUID` helper used elsewhere in `api` tests/handlers — confirm/define it (studioturn/disposition already wrap project ids). `validateEventTrace` is the package func in `cards.go`. `agent.EventRow` fields per Slice 5c. Add the `json`/`encoding/json` import.)*

- [ ] **Step 4: Register routes** (`api.go`, beside the other project card-less routes):

```go
	mux.Handle("POST /api/v1/projects/{id}/cards/{cid}/activate", protected(a.activateProjectCard))
	mux.Handle("POST /api/v1/projects/{id}/cards/{cid}/skip", protected(a.skipProjectCard))
```

- [ ] **Step 5: Run** → PASS. **Step 6: Commit** (`feat(api): project card activate + skip endpoints`).

---

### Task 5: Project card `submit` endpoint (SSE: persist + CompleteCard + refeed)

**Files:** Modify `apps/api/internal/api/projectcards.go` (add `submitProjectCard`) · `api.go` (route) · Test `projectcards_test.go` (add the submit case).

**Interfaces:** `POST /projects/{id}/cards/{cid}/submit` (SSE). Mirrors `postProjectTurn`'s SSE shape (heartbeat, `studioEmitter`, the result switch) + the card persistence + `CompleteCard`.

- [ ] **Step 1: Write the failing test** — activate → submit (filled CRAAP anchors) → SSE `done`, status completed, an evidence node minted:

```go
func TestProjectCardSubmit(t *testing.T) {
	pool := newAPITestPool(t)
	h := New(Deps{Queries: sqlc.New(pool), Pool: pool, Provider: fakeProvider(), ChatResolver: fakeResolver(), SpecByID: cardsByID()}).Handler()
	cookie := signInSeed(t, pool)
	projectID := uuid.MustParse("00000000-0000-0000-0000-000000000101")
	cid := createProjectCardForTest(t, pool) // craap on a seeded material, status active
	base := "/api/v1/projects/00000000-0000-0000-0000-000000000101/cards/" + cid

	// Fully-satisfying CRAAP anchors: all 5 tags answered + a student risk_note.
	anchors := craapCompleteAnchors(t, pool, cid) // helper builds anchors keyed to the card's material
	body := `{"field_values":{"final_verdict":"存疑"},"event_trace":[{"kind":"submit","at":"2026-07-12T00:00:00Z"}],"anchors":` + anchors + `}`
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, withCookie(httptest.NewRequest("POST", base+"/submit", strings.NewReader(body)), cookie))
	if rr.Code != 200 || !strings.Contains(rr.Body.String(), "event: done") {
		t.Fatalf("submit: %d — %s", rr.Code, rr.Body.String())
	}
	// status completed + an evidence node minted
	q := sqlc.New(pool)
	got, _ := q.GetCardInstance(context.Background(), uuid.MustParse(cid))
	if got.Status != "completed" { t.Fatalf("status = %q", got.Status) }
	nodes, _ := q.ListGraphNodesByProject(context.Background(), projectID)
	if !anyNodeOfType(nodes, "evidence") { t.Fatalf("no evidence node minted") }
}
```

*(Implementer: `craapCompleteAnchors` builds an `[]Anchor` JSON with `Dimension` ∈ the 5 CRAAP tags + a `risk_note`/student anchor, each with a non-empty `Answer` and `MaterialID` = the card's evaluated material (read the `evaluates` edge or the seeded material id), so `EvaluateCompletion` passes and `GraphEffects` mints. If building fully-complete anchors is fiddly, at minimum assert status=completed + the SSE `done`; the evidence-node assertion is the stronger check — include it if the anchor fixture is tractable, else defer to the CompleteCard unit test. Use the seeded material `…0110`/`…0111`.)*

- [ ] **Step 2: Run — verify it fails** (route 404).

- [ ] **Step 3: Add `submitProjectCard`** to `projectcards.go` (SSE — clone `postProjectTurn`'s emitter+heartbeat, then persist + CompleteCard + refeed):

```go
func (a *API) submitProjectCard(w http.ResponseWriter, r *http.Request) {
	projectID, cid, ok := a.loadOwnedProjectCard(w, r)
	if !ok { return }
	u, _ := UserFromContext(r.Context())
	entitled, err := HasEntitlement(r.Context(), u)
	if err != nil { httpx.WriteError(w, r, err); return }
	if !entitled { httpx.WriteError(w, r, httpx.ErrNotEntitled()); return }

	var body struct {
		FieldValues json.RawMessage `json:"field_values"`
		EventTrace  json.RawMessage `json:"event_trace"`
		Anchors     json.RawMessage `json:"anchors"`
	}
	if err := decodeJSON(r, &body); err != nil { httpx.WriteError(w, r, err); return }
	if err := validateFieldValues(body.FieldValues); err != nil { httpx.WriteError(w, r, err); return }
	if err := validateEventTrace(body.EventTrace); err != nil { httpx.WriteError(w, r, err); return }
	if err := validateAnchors(body.Anchors); err != nil { httpx.WriteError(w, r, err); return }

	resolved, err := a.d.ChatResolver(r.Context())
	if err != nil { httpx.WriteError(w, r, httpx.ErrInternal()); return }
	sse, err := gateway.NewSSEWriter(w)
	if err != nil { httpx.WriteError(w, r, httpx.ErrInternal()); return }
	em := &studioEmitter{sse: sse}
	stop := make(chan struct{}); hbDone := make(chan struct{})
	go func() { defer close(hbDone); ticker := time.NewTicker(15 * time.Second); defer ticker.Stop()
		for { select { case <-stop: return; case <-r.Context().Done(): return; case <-ticker.C: _ = em.Heartbeat() } } }()
	defer func() { close(stop); <-hbDone }()

	store := agent.NewSqlcAgentStore(a.d.Queries)
	if err := store.SetCardInstanceAnchors(r.Context(), projectID, cid, body.Anchors); err != nil { _ = em.ErrorEnvelope("internal_error", "提交失败，请重试"); _ = em.Done(); return }
	if err := store.SubmitProjectCardInstance(r.Context(), projectID, cid, body.FieldValues, body.EventTrace); err != nil { _ = em.ErrorEnvelope("internal_error", "提交失败，请重试"); _ = em.Done(); return }
	if err := store.SetCardInstanceStatus(r.Context(), projectID, cid, "completed"); err != nil { _ = em.ErrorEnvelope("internal_error", "提交失败，请重试"); _ = em.Done(); return }

	sk, _ := skills.ByID("writing-project")
	deps := agent.AgentDeps{Store: store, Provider: a.d.Provider, Resolved: resolved, Sim: studioSimilarity(), Skill: &sk, SkipSurfaceCards: false}

	row, err := store.GetCardInstance(r.Context(), cid)
	if err == nil {
		if spec, ok := cards.ByID(row.CardID); ok {
			if _, err := agent.CompleteCard(r.Context(), deps, spec, cid); err != nil {
				slog.Error("card submit: CompleteCard", "err", err, "request_id", httpx.RequestIDFromContext(r.Context()))
			}
		}
	}

	action, err := agent.RunAgentStep(r.Context(), deps, projectID, agent.Trigger{Kind: "card_refeed"})
	if err != nil { slog.Error("card submit: refeed", "err", err, "request_id", httpx.RequestIDFromContext(r.Context())); _ = em.ErrorEnvelope("internal_error", "提交失败，请重试"); _ = em.Done(); return }
	switch {
	case action == nil:
	case action.Kind == "intervention":
		_ = em.Intervention(action.InterventionID, action.Output.Body, "", action.Output.Criterion, "")
	case action.Kind == "surface_card":
		spec, _ := cards.ByID(action.CardID)
		_ = em.Card(action.CardInstanceID, action.CardID, spec.Name, []byte("[]"))
	case action.Kind == "check_gate" && action.GateReport != nil:
		gr := action.GateReport
		_ = em.Gate(gr.Contract, gr.Status, 0, 0, gr.Missing)
	}
	_ = em.Done()
}
```

**Avoid duplication — extract `streamAction`:** the result switch (`intervention`/`surface_card`/`check_gate`/silence → `em.*`) is now needed by BOTH `postProjectTurn` (Task 3, which also gained the `surface_card` case) and `submitProjectCard`. Extract it once and call it from both:

```go
// streamAction emits one RunAgentStep result over the Studio SSE emitter.
func streamAction(em *studioEmitter, action *agent.Action) {
	switch {
	case action == nil: // silence — nothing but done
	case action.Kind == "intervention":
		_ = em.Intervention(action.InterventionID, action.Output.Body, "", action.Output.Criterion, "")
	case action.Kind == "surface_card":
		spec, _ := cards.ByID(action.CardID)
		_ = em.Card(action.CardInstanceID, action.CardID, spec.Name, []byte("[]"))
	case action.Kind == "check_gate" && action.GateReport != nil:
		gr := action.GateReport
		_ = em.Gate(gr.Contract, gr.Status, 0, 0, gr.Missing)
	}
}
```
Refactor `postProjectTurn`'s switch (added in Task 3) to `streamAction(em, action); _ = em.Done()`, and use the same in `submitProjectCard`. (Do this refactor as part of Task 5; the Task-3 inline switch is the interim.)

*(Implementer notes: (1) the refeed switch handles `surface_card` too — after completing CRAAP on material A, the loop may surface CRAAP on material B; that's the loop working, not a bug. (2) `CompleteCard` returns `(complete bool, err)`; a not-yet-complete card (anchors don't satisfy) simply doesn't mint — no error; the student can resubmit. (3) The heartbeat-goroutine boilerplate is also shared with `postProjectTurn`; extracting it too is optional, but at minimum share `streamAction`.)*

- [ ] **Step 4: Register the route** (`api.go`): `mux.Handle("POST /api/v1/projects/{id}/cards/{cid}/submit", protected(a.submitProjectCard))`.

- [ ] **Step 5: Run** (`go test ./internal/api/ -run TestProjectCard`) → PASS. Then **full Go gate** (`go build ./... && go vet ./... && go test -p 1 ./...`).

- [ ] **Step 6: Commit** (`feat(api): project card submit (SSE) — persist + CompleteCard mint + refeed`).

---

### Task 6: Frontend `card` `StudioTurnEvent` variant

**Files:** Modify `apps/web/src/api/studioTurn.ts` (+ test).

- [ ] **Step 1: Failing test** — `studioTurn` yields a `card` event:

```ts
it("yields a card event", async () => {
  vi.spyOn(global, "fetch").mockResolvedValue(sseBody(
    `event: card\ndata: {"card_instance_id":"ci1","card_id":"craap","nudge_text":"CRAAP 五维核查","anchors":[]}\n\n` +
    `event: done\ndata: {}\n\n`));
  const events = [];
  for await (const e of studioTurn("p1", "hi")) events.push(e);
  expect(events[0]).toMatchObject({ type: "card", cardInstanceId: "ci1", cardId: "craap", nudgeText: "CRAAP 五维核查" });
});
```

- [ ] **Step 2: Run — fails** (no `card` case).

- [ ] **Step 3: Add the variant + parse** — extend the union:
```ts
  | { type: "card"; cardInstanceId: string; cardId: string; nudgeText: string; anchors: Anchor[] }
```
and the switch: `case "card": yield { type: "card", cardInstanceId: data.card_instance_id, cardId: data.card_id, nudgeText: data.nudge_text, anchors: data.anchors ?? [] }; break;`. Import `Anchor` from `@mind-imprint/contracts`.

- [ ] **Step 4: Run + tsc** → PASS. **Step 5: Commit** (`feat(web-api): card variant on StudioTurnEvent`).

---

### Task 7: Frontend `api/projectCards.ts` client

**Files:** Create `apps/web/src/api/projectCards.ts` (+ test) · Modify `api/index.ts`.

**Interfaces:** `activateProjectCard(projectId, cid)`, `submitProjectCard(projectId, cid, {field_values, event_trace, anchors})` (**SSE async-generator** → `StudioTurnEvent`, mirroring `studioTurn`), `skipProjectCard(projectId, cid, {event_trace})`.

- [ ] **Step 1: Failing test** — `submitProjectCard` streams the refeed events; `activate`/`skip` POST the right shapes (mock fetch). Mirror `studioTurn.test.ts` / `projects.test.ts`.

- [ ] **Step 2: Run — fails.**

- [ ] **Step 3: Write `projectCards.ts`** — `activate`/`skip` via `apiFetch` (204); `submitProjectCard` is an SSE generator cloning `studioTurn`'s fetch+`parseSSE` loop (POST `/api/v1/projects/${projectId}/cards/${cid}/submit`, body `{field_values, event_trace, anchors}`, yield the same `StudioTurnEvent` union incl. `card`). Reuse the frame→event mapping (extract a shared `mapStudioFrame(frame)` in `studioTurn.ts` if clean, else duplicate the small switch).

- [ ] **Step 4: Wire `api/index.ts`** — import the three; add to `ApiClient` + the `api` object.

- [ ] **Step 5: Run + tsc + full `npm test`** → PASS. **Step 6: Commit** (`feat(web-api): projectCards client (activate/submit-SSE/skip)`).

---

### Task 8: Frontend conversation controller — card state

**Files:** Modify `apps/web/src/studio/conversation.ts` (+ test).

**Interfaces:** the snapshot gains `card: { cardInstanceId, cardId, spec: CardSpec, status: "proposed"|"active" } | null`; the controller gains `openCard()`, `submitCard(finalEnvelope)`, `skipCard(eventTrace)`. On a `card` event during `send`/`submitCard`, set `card` from `CARD_REGISTRY[cardId]`.

- [ ] **Step 1: Failing test** — a `send` whose stream yields a `card` event sets `snapshot.card`; `openCard` flips it to active (calls `api.activateProjectCard`); `submitCard` streams the refeed + appends the coach follow-up + clears `card`:

```ts
it("holds a proposed card, opens it, submits + refeeds", async () => {
  const api = {
    async *studioTurn() { yield { type: "card", cardInstanceId: "ci1", cardId: "craap", nudgeText: "n", anchors: [] }; yield { type: "done" }; },
    activateProjectCard: vi.fn(async () => {}),
    async *submitProjectCard() { yield { type: "intervention", interventionId: "i1", body: "补得不错", anchor: "", criterion: "D5", level: "I2" }; yield { type: "done" }; },
    skipProjectCard: vi.fn(async () => {}),
    postDisposition: vi.fn(async () => {}),
  } as any;
  const conv = createStudioConversation({ projectId: "p1", api });
  await conv.send("hi");
  expect(conv.getSnapshot().card).toMatchObject({ cardInstanceId: "ci1", cardId: "craap", status: "proposed" });
  await conv.openCard();
  expect(conv.getSnapshot().card?.status).toBe("active");
  await conv.submitCard({ field_values: {}, event_trace: [], anchors: [] } as any);
  const s = conv.getSnapshot();
  expect(s.card).toBeNull();
  expect(s.messages.at(-1)).toMatchObject({ kind: "ai", body: "补得不错" });
});
```

- [ ] **Step 2: Run — fails.**

- [ ] **Step 3: Implement** — extend `Snapshot`, add the card handlers, and handle the `card` event in the `send`/`submitCard` stream loops (set `card` from `CARD_REGISTRY[data.cardId]`). `submitCard` streams `api.submitProjectCard` and, like `send`, appends `intervention` → `{kind:"ai"}` and clears `card` on `done`. `openCard`→`api.activateProjectCard` + status active. `skipCard`→`api.skipProjectCard` + clear card. Inject `api` (default the real client) as in the existing controller.

- [ ] **Step 4: Run + tsc + full `npm test`** → PASS. **Step 5: Commit** (`feat(studio-web): card state in the conversation controller`).

---

### Task 9: Frontend — Studio card slot in the CoachRail (reuse renderer + reducer)

**Files:** Create `apps/web/src/studio/StudioCardSheet.tsx` (a lean coach-rail-fit host reusing `pickCardBody`/`CardRenderer` + `envelopeReducer`) · Modify `CoachRail.tsx` (replace `CraapPlaceholder` with the live slot) · Test `CoachRail.test.tsx` / `StudioCardSheet.test.tsx`.

**Interfaces produced:** `CoachRailProps` gains `card?: {cardInstanceId, cardId, spec, status} | null` + `onOpenCard`/`onSubmitCard`/`onSkipCard`. Consumed by Task 10.

- [ ] **Step 1: Failing test** — when `card` is a proposed CRAAP, CoachRail shows a proposal (opens on click → `onOpenCard`); when active, renders the CRAAP sheet (a field from `craap.json`) with submit/skip wired:

```tsx
it("renders the craap proposal → sheet and wires submit/skip", () => {
  const onOpenCard = vi.fn(), onSubmitCard = vi.fn(), onSkipCard = vi.fn();
  const spec = CARD_REGISTRY["craap"];
  const { rerender } = render(<CoachRail {...baseProps()} card={{ cardInstanceId: "ci1", cardId: "craap", spec, status: "proposed" }} onOpenCard={onOpenCard} onSubmitCard={onSubmitCard} onSkipCard={onSkipCard} />);
  fireEvent.click(screen.getByRole("button", { name: /打开|开始/ })); // the proposal open affordance
  expect(onOpenCard).toHaveBeenCalled();
  rerender(<CoachRail {...baseProps()} card={{ cardInstanceId: "ci1", cardId: "craap", spec, status: "active" }} onOpenCard={onOpenCard} onSubmitCard={onSubmitCard} onSkipCard={onSkipCard} />);
  // the schema-driven sheet renders a craap step title (verbatim from craap.json)
  expect(screen.getByText(/权威性|来源/)).toBeInTheDocument();
});
```

*(Implementer: match the exact proposal/open button copy to the design; if none is binding, a plain "打开" is fine. Reuse `CardRenderer` via `pickCardBody(spec.id)` inside `StudioCardSheet` with local `envelopeReducer` state; `onSubmitCard(finalEnvelope)` and `onSkipCard(eventTrace)` fire from the sheet footer. Do NOT special-case craap in the renderer.)*

- [ ] **Step 2: Run — fails.**

- [ ] **Step 3: Write `StudioCardSheet.tsx`** — local `env` state (`useState(newEnvelope(cardId, ""))` — the `task_id` arg is a vestigial local artifact), `const Body = pickCardBody(spec.id)`, render `<Body card={spec} values={env.field_values} onField onExpandStep onNote/>` + a footer (submit → `envelopeReducer(env,{submit})` then `onSubmit(finalEnvelope)`; skip → `onSkip(eventTrace)`). Lean — coach-rail column layout, NOT the workspace bottom-sheet. Inline SVG only.

- [ ] **Step 4: Wire `CoachRail.tsx`** — add the `card`/`onOpenCard`/`onSubmitCard`/`onSkipCard` props; replace the `CraapPlaceholder` branch: when `card` present → a proposal bubble (status `proposed`, opens via `onOpenCard`) or the `<StudioCardSheet>` (status `active`); else the existing `DispositionCard`/placeholder logic. Keep the `活跃view === "素材"` behavior sensible (the card slot can render regardless of view, or gate to when a card exists).

- [ ] **Step 5: Run + tsc** → PASS. **Step 6: Commit** (`feat(studio-web): live card slot in the CoachRail (reuse renderer + reducer)`).

---

### Task 10: Frontend — wire the card through `StudioContainer`/`StudioShell`/`state`

**Files:** Modify `apps/web/src/studio/StudioContainer.tsx`, `StudioShell.tsx`, `state.ts` (+ `StudioContainer.test.tsx`).

- [ ] **Step 1: Failing test** — `StudioContainer` passes the conversation's `card` + card callbacks to `StudioShell`→`CoachRail`; opening/submitting routes to `conv.openCard`/`conv.submitCard`:

```tsx
it("wires the card from the conversation to the rail", async () => {
  const conv = { getSnapshot: () => ({ messages: [], sending: false, error: null, disposableInterventionId: null, card: { cardInstanceId: "ci1", cardId: "craap", spec: CARD_REGISTRY["craap"], status: "proposed" } }), subscribe: () => () => {}, send: vi.fn(), dispose: vi.fn(), openCard: vi.fn(), submitCard: vi.fn(), skipCard: vi.fn() };
  render(<StudioContainer api={fakeApi} ensureSession={async () => {}} makeConversation={() => conv as any} />);
  await waitFor(() => expect(screen.getByText("论证构建")).toBeInTheDocument());
  // the craap proposal is visible in the rail
  await waitFor(() => expect(screen.getByText(/CRAAP|来源|核查/)).toBeInTheDocument());
});
```

- [ ] **Step 2: Run — fails.**

- [ ] **Step 3: Wire** — `StudioCallbacks` (`state.ts`) gains `onOpenCard`/`onSubmitCard`/`onSkipCard`; `StudioContainer` reads `convSnapshot.card` and maps the callbacks to `conv.openCard`/`conv.submitCard`/`conv.skipCard`; `StudioShellProps`/`CoachRail` thread `card` + the three callbacks (defaulting undefined/null). Keep the existing `onDisposition`/`onComposerSend` behavior.

- [ ] **Step 4: Run + tsc + full `npm test`** → PASS. **Step 5: Commit** (`feat(studio-web): thread the live card through StudioContainer/Shell`).

---

## Self-Review Checklist (run before final review)

- **Spec coverage:** SubmitProjectCardInstance (T1) · seam+Action.CardID (T2) · surface (T3) · activate/skip (T4) · submit-SSE+CompleteCard+refeed (T5) · card SSE variant (T6) · projectCards client (T7) · conversation card state (T8) · CoachRail card slot (T9) · container wiring (T10). No mid-fill observe, no extra cards, no migration, legacy card endpoints untouched. ✓
- **Type/behavior consistency:** the `card` SSE keys (`card_instance_id/card_id/nudge_text/anchors`) match `SSEWriter.Card` ↔ `studioTurn.ts`. `Action.CardID` set by `SurfaceCard`, consumed by both studio turn sites. `SkipSurfaceCards:false` at BOTH the turn and submit-refeed. Reuse (not fork) the renderer/reducer + `SurfaceCard`/`CompleteCard`/`GraphEffects`. ✓
- **Red lines:** the STUDENT fills the card (Author:"student" anchors); the model never writes field_values; refeed is one RunAgentStep; the proposal opens on the student's confirmation; skip persisted. ✓
- **Live-verify:** send → CRAAP proposal → open → fill → submit → evidence node minted + gate flips + coach reacts. ✓
