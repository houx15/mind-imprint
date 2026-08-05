# 探索「兔子洞」地图重构 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Rebuild the 阅读 room's cluttered 探索图谱 into a two-level "rabbit-hole" map — top = a labeled graph of question nodes, inside each hole = a tidy sparse tree of evidence you search with real papers — collapsing ~12 buttons to 3 actions + a suggestions tray, powered by OpenAlex.

**Architecture:** Reuse the existing `exploration_lead` (nodes) + `reference` (papers) model. Add: a `question_edge` table for top-level labeled edges (AI-proposed, student-confirmed); a `reference.reading_status` column (待读/在读/读完) so the 图书馆 is one shelf with status; an OpenAlex client in `internal/materialize` (copying the Crossref `resolveDOI` template) that feeds a `dig` endpoint returning candidate papers to a client-side **tray**; an `adopt` endpoint that persists a chosen candidate as a reference + a lead under the dug node. Frontend guts `ExplorationView.tsx` chrome to 3 actions + tray, moves the switcher left and renames 列表→图书馆 / 探索图谱→探索 with icons, and adds a top-level question-graph view with zoom-into-hole. UI copy stays plain — no metaphor jargon in buttons.

**Tech Stack:** Go (`net/http` + `pgx`/sqlc + goose), Postgres 16, React + Vite + TS + Tailwind (`mk-*` palette), Zod contracts (`@mind-imprint/contracts`), OpenAlex REST (free, no key), testcontainers (Go), vitest (contracts) + Playwright (web e2e).

## Global Constraints

- **铁律① AI 克制:** 印记只推荐/提议，学生确认才落到地图上。Dig results go to a tray; adopting is an explicit student action. AI edge proposals are `status:"proposed"` until the student confirms.
- **铁律② 不操纵，触发自动、打开由学生确认:** No auto-inserted nodes, no auto-drawn edges.
- **铁律④ 过程即数据:** Discards/relabels are allowed actions; don't hard-block, record state.
- **占「思考」这一层，不做花哨文献管理器:** No dense citation-network import; the inside stays a sparse tree (provenance backbone + sparse cross-links), NOT a full paper graph.
- **避免行话，尤其按钮文案:** "兔子洞/洞口/warren" NEVER appear in UI button/control text. Plain words only: 问题 / 论文 / 笔记 / 探索 / 图书馆 / 深挖 / 相关论文.
- **固定小词表 for question edges:** `子问题 · 支持 · 反驳/张力 · 细化 · 依赖/前提` (closed set, no freeform).
- **不考虑旧数据兼容:** demo only, no real student data. Migrations may DROP/recreate freely; still write a coherent `-- +goose Down` (round-trip tests exercise it).
- **客户端绝不直连模型/外部 API:** OpenAlex is called server-side only. Every real LLM call records an `llm_call` row (OpenAlex calls do NOT — they're not LLM).
- **密钥只在服务端。** No secrets in git/logs/errors.
- **Contracts are the single source of truth:** new wire types get a Zod schema in `packages/contracts/src/` exported from `index.ts`; web parses every response with `Schema.parse`.
- **No new cards needed.** Do not hand-edit `apps/api/internal/cards/specs/` (generated mirror).

## Backend mechanics (apply to every backend task)

- Migrations: `apps/api/internal/store/migrations/NNNN_name.sql`, next ordinal **0053**. goose `-- +goose Up` / `-- +goose Down`. Nullable uuid FK → `pgtype.UUID`; nullable scalar → pointer (config `emit_pointers_for_null_types`).
- Queries: add `-- name: X :one|:many|:exec` blocks to `apps/api/internal/store/queries/workspace.sql` (exploration lives there). Regen: `cd apps/api && make sqlc` (= `CGO_ENABLED=0 go tool sqlc generate`, sqlc v1.27.0 pinned via `go tool`). **CGO_ENABLED=0 is mandatory on macOS.**
- Handlers: methods on `*API` in `apps/api/internal/api/exploration.go` (or a new file). Path params `r.PathValue("id")`; ownership via `a.loadOwnedProject(w,r)`; body via `decodeJSON(r,&body)`; responses `httpx.WriteJSON(w, status, v)` / `httpx.WriteError(w, r, err)`; camelCase DTO via a `toXxxDTO` mapper; list mappers pre-size `make([]T, 0, n)` (never return `null`).
- Routes: register in `(*API).Handler()` in `apps/api/internal/api/api.go`, wrapped `protected(a.handler)`.
- LLM agents: pure composer in `apps/api/internal/agent/`, `gateway.Collect(ctx, prov, resolved, req)`, always `stripFences(res.Text)` before `json.Unmarshal`, system prompt as a package `const`. Meter with the resolve→compose→meter block: resolve `a.d.ChatResolver(r.Context())`, then `agent.NewSqlcAgentStore(a.d.Queries, a.d.Pool).RecordLLMCall(ctx, agent.LLMCallRow{ProjectID, Surface:"studio", Purpose:"...", Resolved, PromptTokens:int32(usage.InputTokens), CompletionTokens:int32(usage.OutputTokens)})`. Meter ONLY when `resolved.Provider != "" && cerr == nil`. Gate `a.d.HasEntitlement`. Leave `MaxTokens` unset (inherits 16000) for large JSON output.
- External fetch: copy the `resolveDOI` template in `apps/api/internal/materialize/doi.go` — `http.NewRequestWithContext`, a **package-var base URL** so tests inject an `httptest` server, `User-Agent` with contact, `Accept: application/json`, `io.LimitReader(resp.Body, 1<<20)`, an anonymous struct of only-needed JSON fields, **best-effort: every failure returns nil/zero** so the caller degrades gracefully. Guard seam: `newUnguardedFetcher()` for loopback tests.
- Tests: black-box `package api_test`; `newAPITestPool(t)` (testcontainers postgres:16-alpine + real migrations); auth via `signInSeed(t, pool)` → cookie; handler via `New(Deps{...}).Handler()` or the `libraryTestHandler`/`doJSON` helper in `workspace_library_test.go`. **Run the FULL package** `go test ./internal/api/` (Docker must be up); never a `-run` subset for gate/creation-path changes. `-short` skips containers.

## Frontend mechanics (apply to every frontend task)

- API client: add functions to `apps/web/src/api/exploration.ts`, pattern `const raw = await apiFetch<unknown>(path, {method, body: JSON.stringify(...)}); return Schema.parse(raw);`. `apiFetch` (in `apps/web/src/api/client.ts`) sets `credentials:"include"` + JSON headers + `VITE_API_BASE_URL` base; throws `ApiError` on non-2xx; returns `undefined` on 204.
- Styling: Tailwind `mk-*` utility classes (palette in `apps/web/tailwind.config.ts`: `primary #2A3B7A`, `accent #D98263`, `green #4C9A82`, `ink #1C2333`, `muted-2 #9AA1B0`, `bg #F3F4F8`, `surface #FFFFFF`, `border #EAECF2`; radii `rounded-mk 14px`). Icons: add a `case` to the switch in `apps/web/src/workspace/Icon.tsx` (`Icon({name, size})`), render `<Icon name="..." size={14}/>`.
- The `"list" | "graph"` view-mode union in `ReadingBlock.tsx` is load-bearing across ~8 sites — **rename display labels only, keep the keys**.
- Typecheck after every task: `pnpm --filter web typecheck` (= `tsc --noEmit`) MUST pass clean. Contracts tests: `pnpm --filter @mind-imprint/contracts test`.
- The graph is drawn with CSS indented rails (`border-l-2`), no graph library — keep it that way (top-level graph uses a small hand-laid SVG/flex, not a physics lib).

---

# PHASE P-A — Inside a hole: real search + tray + declutter (ships + deploys first)

### Task A1: `reference.reading_status` column (one shelf, three states)

**Files:**
- Create: `apps/api/internal/store/migrations/0053_reference_reading_status.sql`
- Modify: `apps/api/internal/store/queries/workspace.sql` (reference queries — add status to insert/update + a status-patch query)
- Regen: `apps/api/internal/store/sqlc/*.sql.go` (via `make sqlc`)
- Modify: `apps/api/internal/api/workspace_library.go` (reference DTO mapper — expose `readingStatus`)
- Modify: `packages/contracts/src/reference.ts` (add `readingStatus` field)
- Test: `apps/api/internal/api/workspace_library_test.go` (new test)

**Interfaces:**
- Produces: `reference.reading_status text NOT NULL DEFAULT 'to_read' CHECK (reading_status IN ('to_read','reading','done'))`; DTO field `readingStatus: "to_read"|"reading"|"done"`; a `PATCH /api/v1/projects/{id}/references/{rid}` already exists (patchReference) — extend it to accept `readingStatus`.

- [ ] **Step 1: Write the migration**

```sql
-- apps/api/internal/store/migrations/0053_reference_reading_status.sql
-- +goose Up
ALTER TABLE reference ADD COLUMN reading_status text NOT NULL DEFAULT 'to_read'
    CHECK (reading_status IN ('to_read','reading','done'));

-- +goose Down
ALTER TABLE reference DROP COLUMN reading_status;
```

- [ ] **Step 2: Add/extend sqlc queries in `workspace.sql`**

Find the reference INSERT + UPDATE queries and add `reading_status` to the column list + params (default `'to_read'` on insert). Add a targeted patch query if reference updates use a merge handler; otherwise add `reading_status` to the existing update params. Confirm the generated `Reference` row struct will carry `ReadingStatus string`.

- [ ] **Step 3: Regen sqlc** — `cd apps/api && make sqlc`. Expected: `internal/store/sqlc/workspace.sql.go` now has `ReadingStatus` on the `Reference` struct + params.

- [ ] **Step 4: Expose in the DTO + accept in patch**

In `workspace_library.go`, add `ReadingStatus string \`json:"readingStatus"\`` to the reference DTO struct and set it in the mapper. In the reference PATCH handler, accept an optional `readingStatus *string`, validate against the 3 values (400 on bad), pass through to the update.

- [ ] **Step 5: Add the contract field**

```ts
// packages/contracts/src/reference.ts — inside the Reference object
readingStatus: z.enum(["to_read", "reading", "done"]).default("to_read"),
```

- [ ] **Step 6: Write the failing test**

```go
// workspace_library_test.go
func TestReference_ReadingStatusDefaultAndPatch(t *testing.T) {
    pool := newAPITestPool(t)
    h := libraryTestHandler(pool)
    cookie := signInSeed(t, pool)
    // create a project + reference via existing endpoints, capture ids...
    // GET the reference; assert readingStatus == "to_read"
    // PATCH { "readingStatus": "done" }; assert 200 and readingStatus == "done"
    // PATCH { "readingStatus": "bogus" }; assert 400
}
```

- [ ] **Step 7: Run full package** — `cd apps/api && go test ./internal/api/`. Expected: PASS (Docker up).

- [ ] **Step 8: Verify web typecheck** — `pnpm --filter web typecheck` clean; `pnpm --filter @mind-imprint/contracts test` green.

- [ ] **Step 9: Commit** — `git add apps/api/internal/store apps/api/internal/api packages/contracts/src/reference.ts && git commit -m "feat(exploration): reference.reading_status — one shelf, three states"`

---

### Task A2: OpenAlex client (search + related works)

**Files:**
- Create: `apps/api/internal/materialize/openalex.go`
- Create: `apps/api/internal/materialize/openalex_test.go`

**Interfaces:**
- Produces: on `*HTTPFetcher` two methods —
  `SearchWorks(ctx context.Context, query string, limit int) []WorkMeta` and
  `RelatedWorks(ctx context.Context, doi string, limit int) []WorkMeta`.
  `type WorkMeta struct { DOI, Title, Authors, Year, Journal, Abstract, URL string }` (`Authors` = "A, B, C" joined; `Year` string to match `Reference.year`). Best-effort: returns `nil` / partial on any failure.

- [ ] **Step 1: Write the failing test (httptest, unguarded fetcher)**

```go
// openalex_test.go — package materialize
func TestSearchWorks_ParsesResults(t *testing.T) {
    srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        if !strings.Contains(r.URL.RawQuery, "search=") { t.Fatalf("missing search param: %s", r.URL.RawQuery) }
        _, _ = io.WriteString(w, `{"results":[{"doi":"https://doi.org/10.1000/x","title":"Carbon in China",
          "publication_year":2023,"primary_location":{"source":{"display_name":"Nature Sustainability"},"landing_page_url":"https://n.example/x"},
          "authorships":[{"author":{"display_name":"Li Wei"}},{"author":{"display_name":"Zhang San"}}],
          "abstract_inverted_index":{"Carbon":[0],"rising":[1]}}]}`)
    }))
    defer srv.Close()
    old := openAlexBase; openAlexBase = srv.URL + "/works"; defer func(){ openAlexBase = old }()
    f := newUnguardedFetcher()
    got := f.SearchWorks(context.Background(), "china carbon", 5)
    if len(got) != 1 { t.Fatalf("want 1, got %d", len(got)) }
    if got[0].Title != "Carbon in China" || got[0].Year != "2023" || got[0].Journal != "Nature Sustainability" { t.Fatalf("bad parse: %+v", got[0]) }
    if got[0].Authors != "Li Wei, Zhang San" { t.Fatalf("authors: %q", got[0].Authors) }
    if !strings.HasPrefix(got[0].Abstract, "Carbon rising") { t.Fatalf("abstract reconstruct: %q", got[0].Abstract) }
    if got[0].DOI != "10.1000/x" { t.Fatalf("doi strip: %q", got[0].DOI) }
}
```

- [ ] **Step 2: Run to verify it fails** — `go test ./internal/materialize/ -run TestSearchWorks_ParsesResults` → FAIL (undefined `SearchWorks`/`openAlexBase`).

- [ ] **Step 3: Implement `openalex.go`**

Copy the `resolveDOI` idioms. Key details: `var openAlexBase = "https://api.openalex.org/works"`; build URL `?search=<q>&per-page=<limit>&mailto=hi@mind-imprint.uni-robot.cn`; `User-Agent: MindImprint/1.0 (+https://mind-imprint.uni-robot.cn)`; `io.LimitReader(resp.Body, 1<<20)`; decode `struct{ Results []work }` where `work` has `Doi string`, `Title string`, `PublicationYear int`, `PrimaryLocation struct{ Source struct{ DisplayName string } ; LandingPageURL string }`, `Authorships []struct{ Author struct{ DisplayName string } }`, `AbstractInvertedIndex map[string][]int`. Map to `WorkMeta`: strip `https://doi.org/` prefix from DOI; `Year = strconv.Itoa(PublicationYear)` (omit if 0); join author display names with ", "; reconstruct abstract from the inverted index (position→word, sorted by index). `RelatedWorks`: GET `openAlexBase/doi:<doi>` to get the work's `id`, then `?filter=related_to:<id>&per-page=<limit>` (best-effort; return nil on any failure). Return `nil` on non-200/decode error.

- [ ] **Step 4: Run to verify it passes** — `go test ./internal/materialize/ -run TestSearchWorks_ParsesResults` → PASS. Also add + pass `TestSearchWorks_BadStatusReturnsNil` (server returns 500 → nil).

- [ ] **Step 5: Run full package** — `go test ./internal/materialize/` → PASS.

- [ ] **Step 6: Commit** — `git add apps/api/internal/materialize && git commit -m "feat(exploration): OpenAlex search+related client (best-effort, testable base URL)"`

---

### Task A3: `POST /exploration/dig` — candidates to the tray (no persist)

**Files:**
- Modify: `apps/api/internal/api/api.go` (route)
- Modify: `apps/api/internal/api/exploration.go` (handler + DTO)
- Modify: `packages/contracts/src/exploration.ts` (`DigCandidate`, `DigResult` schemas)
- Test: `apps/api/internal/api/exploration_test.go`

**Interfaces:**
- Consumes: `Fetcher.SearchWorks`/`RelatedWorks` (A2); needs `Fetcher` widened. The `api.Fetcher` interface (`api.go:18-20`) must gain `SearchWorks`/`RelatedWorks`; `fakeFetcher` in tests implements them.
- Produces: `POST /api/v1/projects/{id}/exploration/dig` body `{ leadId?: string, keyword?: string }` → `{ candidates: DigCandidate[] }`. Dig from a lead uses the lead's `text` (a question) OR, if the lead is a paper node (has `connectedReferenceId` with a DOI), uses `RelatedWorks`. `keyword` digs by free text. NOT persisted. Not metered (no LLM).
- `DigCandidate = { doi, title, authors, year, journal, abstract, url }`.

- [ ] **Step 1: Widen the `Fetcher` interface + fake**

In `api.go`, add `SearchWorks(ctx, query string, limit int) []materialize.WorkMeta` and `RelatedWorks(ctx, doi string, limit int) []materialize.WorkMeta` to the `Fetcher` interface. In `exploration_test.go` (or the shared fake), extend `fakeFetcher` to return a canned slice.

- [ ] **Step 2: Write the failing test**

```go
func TestExplorationDig_KeywordReturnsCandidates(t *testing.T) {
    pool := newAPITestPool(t); cookie := signInSeed(t, pool)
    h := New(Deps{Queries: sqlc.New(pool), Pool: pool, SpecByID: cards.ByID,
        Fetcher: fakeFetcher{works: []materialize.WorkMeta{{DOI:"10.1/x", Title:"T", Year:"2023"}}}}).Handler()
    projectID := createProject(t, h, cookie)
    rec := doJSON(t, h, cookie, "POST", "/api/v1/projects/"+projectID+"/exploration/dig", `{"keyword":"china carbon"}`)
    if rec.Code != 200 { t.Fatalf("code %d: %s", rec.Code, rec.Body) }
    // decode { candidates: [...] }; assert len 1, title "T"
}
```

- [ ] **Step 3: Run to verify it fails** — `go test ./internal/api/ -run TestExplorationDig_KeywordReturnsCandidates` → FAIL (404 route).

- [ ] **Step 4: Implement the handler + route + DTO + contract**

Handler `digExploration`: `loadOwnedProject`; decode `{leadId *string, keyword *string}`; gate `HasEntitlement`; resolve query: if `keyword` non-empty use it; else if `leadId` set, load the lead, use its `text` (and if it has a connected reference with a DOI, prefer `RelatedWorks`); call `SearchWorks(ctx, query, 8)`; map to `[]DigCandidate` (`make([]T,0,n)`); `WriteJSON(200, {candidates})`. Register `mux.Handle("POST /api/v1/projects/{id}/exploration/dig", protected(a.digExploration))`. Add `DigCandidate`/`DigResult` Zod schemas to `exploration.ts` + export.

- [ ] **Step 5: Run to verify it passes** — full package `go test ./internal/api/` → PASS.

- [ ] **Step 6: Web typecheck** — `pnpm --filter @mind-imprint/contracts test` + `pnpm --filter web typecheck` clean.

- [ ] **Step 7: Commit** — `git add apps/api packages/contracts && git commit -m "feat(exploration): POST /dig — OpenAlex candidates to the tray (no persist, no meter)"`

---

### Task A4: `POST /exploration/adopt` — persist a tray pick as reference + lead

**Files:**
- Modify: `apps/api/internal/api/api.go` (route)
- Modify: `apps/api/internal/api/exploration.go` (handler)
- Modify: `apps/api/internal/store/queries/workspace.sql` if a combined create is cleaner (else reuse existing reference+lead creators)
- Test: `apps/api/internal/api/exploration_test.go`

**Interfaces:**
- Consumes: A1 `reading_status`; existing reference-create + `CreateExplorationLead`.
- Produces: `POST /api/v1/projects/{id}/exploration/adopt` body `{ parentLeadId?: string, candidate: DigCandidate }` → creates a `reference` (from candidate meta; `reading_status='reading'`, `tags=[]`, `url`=candidate.url or doi.org) + a `lead` (`origin='guide'`, `parentLeadId`, `connectedReferenceId`=new ref id, `text`=candidate.title). Returns `{ lead, reference }`. Adopting = the only way a candidate becomes a node (铁律①).

- [ ] **Step 1: Write the failing test**

```go
func TestExplorationAdopt_CreatesReferenceAndLead(t *testing.T) {
    pool := newAPITestPool(t); cookie := signInSeed(t, pool)
    h := New(Deps{Queries: sqlc.New(pool), Pool: pool, SpecByID: cards.ByID, Fetcher: fakeFetcher{}}).Handler()
    projectID := createProject(t, h, cookie)
    body := `{"candidate":{"doi":"10.1/x","title":"Nature Sust 2023","authors":"Li","year":"2023","journal":"Nature Sustainability","abstract":"...","url":"https://n/x"}}`
    rec := doJSON(t, h, cookie, "POST", "/api/v1/projects/"+projectID+"/exploration/adopt", body)
    if rec.Code != 201 { t.Fatalf("code %d: %s", rec.Code, rec.Body) }
    // decode { lead, reference }; assert reference.readingStatus=="reading",
    // lead.connectedReferenceId == reference.id, lead.origin=="guide"
    // GET /exploration; assert the lead is present
}
```

- [ ] **Step 2: Run to verify it fails** → FAIL (404).

- [ ] **Step 3: Implement** — handler `adoptExploration`: `loadOwnedProject`; decode; validate `candidate.title` non-empty (400); optional `parentLeadId` IDOR check; in one transaction (`a.d.Pool.Begin`) insert the reference (fill title/author/year/journal/abstract/url, `reading_status='reading'`) then the lead (`connected_reference_id`=ref id, `parent_lead_id`, `origin='guide'`, `status='connected'`, `position`=count); commit; `WriteJSON(201, {lead: toLeadDTO, reference: toRefDTO})`. Register route.

- [ ] **Step 4: Run full package** → PASS.

- [ ] **Step 5: Commit** — `git commit -m "feat(exploration): POST /adopt — tray pick → reference + connected lead (auto-shelved)"`

---

### Task A5: Web API client — dig + adopt

**Files:**
- Modify: `apps/web/src/api/exploration.ts`
- Test: none (thin client; covered by A6 behavior + typecheck)

**Interfaces:**
- Produces: `digExploration(projectId, opts:{leadId?:string; keyword?:string}): Promise<DigResult>` and `adoptCandidate(projectId, candidate: DigCandidate, opts?:{parentLeadId?:string}): Promise<{lead: ExplorationLead; reference: Reference}>`.

- [ ] **Step 1: Add the client functions**

```ts
import { DigResult, DigCandidate } from "@mind-imprint/contracts";
export async function digExploration(projectId: string, opts: { leadId?: string; keyword?: string }): Promise<DigResult> {
  const raw = await apiFetch<unknown>(`/api/v1/projects/${projectId}/exploration/dig`, { method: "POST", body: JSON.stringify(opts) });
  return DigResult.parse(raw);
}
export async function adoptCandidate(projectId: string, candidate: DigCandidate, opts?: { parentLeadId?: string }) {
  const raw = await apiFetch<unknown>(`/api/v1/projects/${projectId}/exploration/adopt`, { method: "POST", body: JSON.stringify({ candidate, parentLeadId: opts?.parentLeadId ?? null }) });
  return raw as { lead: ExplorationLead; reference: Reference };
}
```

- [ ] **Step 2: Typecheck** — `pnpm --filter web typecheck` clean.
- [ ] **Step 3: Commit** — `git commit -m "feat(exploration): web client for /dig + /adopt"`

---

### Task A6: Declutter ExplorationView to 3 actions + the tray (inside-a-hole view)

**Files:**
- Modify: `apps/web/src/workspace/blocks/exploration/ExplorationView.tsx`
- (Optional) Create: `apps/web/src/workspace/blocks/exploration/DigTray.tsx`

**Interfaces:**
- Consumes: A5 `digExploration`/`adoptCandidate`; existing `createLead`/`patchLead`/`getExploration`.

> **Detail-on-dispatch (frontend rework):** this is a chrome rework over stable handlers (`ExplorationView` body lines ~200–314). The controller will pair this task's dispatch with the concrete before→after button map from the spec. Scope below is binding.

**The 3 actions (everything else is removed or demoted into a hover/kebab menu):**
1. **看地图** — the tree itself (left-to-right, `border-l-2` rails kept). Sections collapse from three (有分支的来源/悬空来源/待追的线索) to a single per-question subtree; the dangling/loose concepts disappear from chrome (data still exists).
2. **深挖** — ONE primary control per node: "深挖" on a question node → `digExploration({leadId})`; on a paper node → `digExploration({leadId})` (backend picks related-works); plus a single free-text "输入关键词深挖" input at the view root → `digExploration({keyword})`. This replaces `＋添加来源`, `＋兔子洞`, `＋分支`, `＋来源`, `连接来源`, `深挖这条`, manual-lead input, and the rabbit-hole `<select>`.
3. **印记推荐 → 托盘 → 采纳** — dig results render in a `DigTray` (a panel, plain copy "刚挖到这些论文"): each candidate shows title · authors · year · journal, an "采纳" button (`adoptCandidate` → appears as a child node under the dug lead) and a "丢弃" (removes from tray only). No auto-adopt.

- [ ] **Step 1: Add tray state + dig wiring** — `const [tray, setTray] = useState<DigCandidate[]>([])`, `const [digTarget, setDigTarget] = useState<{leadId?:string} | null>(null)`. A `runDig(opts)` calls `digExploration`, sets `tray`, opens the panel. Keyword input at root calls `runDig({keyword})`.
- [ ] **Step 2: Add adopt/discard** — "采纳" calls `adoptCandidate(projectId, c, {parentLeadId: digTarget?.leadId})`, then refetch exploration + remove from tray; "丢弃" removes from tray.
- [ ] **Step 3: Remove/​demote old chrome** — delete the header `＋添加来源`/`＋兔子洞`, the rabbit-hole card `<select>` block, the manual-lead input, and per-node `＋分支`/`＋来源`/`连接来源`; keep `剪枝` + `进入阅读室` + `ⓘ` in a compact hover/kebab. Keep the three sections' DATA merged into one question-rooted tree render.
- [ ] **Step 4: Plain-copy audit** — grep the file for 兔子洞/洞/warren in any rendered string; replace with plain words ("深挖" / "相关论文" / "问题"). The rabbit metaphor must not appear in UI text.
- [ ] **Step 5: Typecheck** — `pnpm --filter web typecheck` clean.
- [ ] **Step 6: Live-verify** — run the app (`bash apps/web/e2e/run-stack.sh` or the dev server), open a project's 探索, dig a question, adopt a candidate, confirm it appears as a child node and lands in the 图书馆. Screenshot.
- [ ] **Step 7: Commit** — `git commit -m "feat(exploration): inside-a-hole view — 3 actions + dig tray, gut ~12 buttons"`

---

### Task A7: Switcher → left, rename 图书馆/探索, add icons, library status badges

**Files:**
- Modify: `apps/web/src/workspace/blocks/ReadingBlock.tsx` (ViewModeToggle labels + header alignment)
- Modify: `apps/web/src/workspace/Icon.tsx` (two new icon cases: `library`, `explore`)
- Modify: the library table/preview components to show a `readingStatus` badge + a status control (find the RefTable/Preview under `ReadingBlock`)

**Interfaces:** display-only; the `"list"|"graph"` keys stay.

- [ ] **Step 1: Rename labels + add icons** — in `ViewModeToggle`, render `图书馆` for `list` and `探索` for `graph` (keep keys), each preceded by `<Icon name={m === "list" ? "library" : "explore"} size={14}/>`. Add `library` (book/shelf path) and `explore` (nodes/graph path) cases to `Icon.tsx`.
- [ ] **Step 2: Move switcher left** — header row (`ReadingBlock.tsx:~395`) `justify-end` → `justify-start`.
- [ ] **Step 3: Library status** — in the reference table row + preview, render a small badge (待读/在读/读完 mapped from `to_read`/`reading`/`done`) using `mk-*` tints, and a control to change it (calls the extended `patchReference({readingStatus})`).
- [ ] **Step 4: Typecheck** — clean.
- [ ] **Step 5: Live-verify + screenshot** — switcher on the left, renamed with icons; a paper adopted in 探索 shows 在读 in 图书馆.
- [ ] **Step 6: Commit** — `git commit -m "feat(reading): switcher→left, 图书馆/探索 + icons, reading-status badges"`

### ▶ Deploy checkpoint 1 (after P-A)

- [ ] Run backend full suites: `cd apps/api && go test ./...` (Docker up). Web: `pnpm --filter web typecheck` + `pnpm --filter @mind-imprint/contracts test`.
- [ ] Commit + push `origin/main`. Deploy: `./.deploy-local/deploy.sh full` (P-A has a migration → `full`, which backs up DB, migrates+seeds, rebuilds api, then web). Verify `DEPLOY OK`. Live-check dig→tray→adopt on prod as Phoebe.

---

# PHASE P-B — Top labeled question graph (AI-proposed, student-confirmed)

### Task B1: `question_edge` table + queries + inclusion in GET /exploration

**Files:**
- Create: `apps/api/internal/store/migrations/0054_question_edge.sql`
- Modify: `apps/api/internal/store/queries/workspace.sql`
- Regen sqlc
- Modify: `apps/api/internal/api/exploration.go` (`getExploration` includes `edges`)
- Modify: `packages/contracts/src/exploration.ts` (`QuestionEdge`, extend `ExplorationView`)
- Test: `apps/api/internal/api/exploration_test.go`

**Interfaces:**
- Produces: `question_edge(id, project_id, from_lead_id, to_lead_id, label CHECK IN ('子问题','支持','反驳/张力','细化','依赖/前提'), status CHECK IN ('proposed','confirmed'), created_at)`. Both `from/to` FK `exploration_lead ON DELETE CASCADE`. DTO `QuestionEdge = {id, fromLeadId, toLeadId, label, status}`. `ExplorationView` gains `edges: QuestionEdge[]`.

- [ ] **Step 1: Migration** (Up creates table + `project_id` index + unique `(from_lead_id,to_lead_id)`; Down drops).
- [ ] **Step 2: Queries** — `CreateQuestionEdge :one`, `ListQuestionEdgesByProject :many`, `UpdateQuestionEdge :one` (label+status), `DeleteQuestionEdge :exec` (all `project_id`-scoped). Regen.
- [ ] **Step 3: Failing test** — `TestExploration_IncludesEdges`: seed two leads, insert an edge via a new POST (B2) or directly; GET /exploration asserts `edges` array present with the label.
- [ ] **Step 4: Implement** — extend `getExploration` to list edges + add to the view payload (mapper pre-sized). Add `QuestionEdge` Zod + `edges` on `ExplorationView`.
- [ ] **Step 5: Full package** → PASS. Web typecheck + contracts test clean.
- [ ] **Step 6: Commit** — `git commit -m "feat(exploration): question_edge table + edges in GET /exploration"`

### Task B2: Edge lifecycle endpoints (create / relabel / confirm / dismiss)

**Files:** `api.go` (routes), `exploration.go` (handlers), `exploration_test.go`.

**Interfaces:**
- `POST /exploration/edges` `{fromLeadId, toLeadId, label}` → 201 (status `confirmed` when student-created). `PATCH /exploration/edges/{eid}` `{label?, status?}` (relabel / `proposed`→`confirmed`). `DELETE /exploration/edges/{eid}` (dismiss). Web client fns `createEdge/patchEdge/deleteEdge` in `exploration.ts`.

- [ ] Step 1: Failing test `TestQuestionEdge_CRUD` (create→GET shows confirmed; patch relabels; delete removes; IDOR on foreign lead → 400/404).
- [ ] Step 2: Implement handlers (validate label ∈ closed set, both leads owned + roots; 400 otherwise) + routes + web client.
- [ ] Step 3: Full package + typecheck. Commit `feat(exploration): question-edge create/relabel/confirm/dismiss`.

### Task B3: AI-proposes-edges agent + endpoint (metered)

**Files:** `apps/api/internal/agent/edge_proposer.go` (+ `_test.go`), `exploration.go` (handler), `api.go` (route).

**Interfaces:**
- `ProposeQuestionEdges(ctx, prov, resolved, in EdgeProposerInput) ([]EdgeProposal, ChatUsage, error)` where input = the root questions (id+text) and existing edges; output `EdgeProposal{fromLeadId,toLeadId,label,why}` constrained to the closed label set. Endpoint `POST /exploration/edges/propose` → gate entitlement → resolve→compose→meter (Purpose `"edge_propose"`) → persist each proposal as a `question_edge` with `status='proposed'` → return them. Student later confirms via B2 PATCH.

- [ ] Step 1: Agent unit test with a stubbed provider returning canned JSON (follow existing agent test patterns; assert `stripFences` handling + label whitelist filtering — drop any label not in the closed set).
- [ ] Step 2: Implement composer (system `const`, JSON `{proposals:[...]}`, `MaxTokens` unset), handler with the meter block, route.
- [ ] Step 3: Handler test `TestProposeEdges_PersistsProposed` (fake/stub provider; assert edges appear with `status:"proposed"`; assert an `llm_call` row exists). Full package → PASS.
- [ ] Step 4: Commit `feat(exploration): 印记 proposes labeled question edges (metered, student-confirmed)`.

### Task B4: Top-level question-graph view + zoom-into-hole

**Files:** `apps/web/src/workspace/blocks/exploration/ExplorationView.tsx` (add a top "map" mode), (optional) `WarrenGraph.tsx` for the node-graph render, `exploration.ts` client (edge fns from B2 + propose).

> **Detail-on-dispatch (frontend).** Binding scope below.

- Two zoom states inside 探索: **地图 (overview)** = root question nodes laid out with their labeled edges (small hand-laid layout — flex/absolute + SVG lines, NO physics lib; edges show the label; `proposed` edges render dashed with a 确认/忽略 affordance per 铁律②); **钻进 (inside)** = the Task-A6 single-question tree. Clicking a question node zooms in; a "← 返回地图" (plain copy) zooms out.
- 印记 proposes edges: a subtle "印记发现的关系" affordance runs `proposeEdges`; proposals appear as dashed edges the student 确认s (PATCH→confirmed) or 忽略s (DELETE).
- Cross-cluster edges from shared papers: when the same reference id is connected under two different roots, draw a faint auto "同源" hint line (derived client-side; not a `question_edge`).

- [ ] Step 1: Add zoom state + overview render (nodes + confirmed/proposed edges). Step 2: wire propose/confirm/dismiss. Step 3: shared-paper hint lines (derived). Step 4: plain-copy audit (no jargon). Step 5: typecheck + live-verify + screenshot. Step 6: commit `feat(exploration): top question-graph map + zoom-into-hole + edge confirm`.

### ▶ Deploy checkpoint 2 (after P-B)

- [ ] Full backend suite + web typecheck/contracts. Push. `./.deploy-local/deploy.sh full` (P-B has migrations). Verify + live-check on prod.

---

# PHASE P-C — Notes → question + seeding

### Task C1: Extend lead creation (sourceReferenceId + `note` origin)

**Files:** `packages/contracts/src/exploration.ts` (add `"note"` to `LeadOrigin`), `apps/api/internal/api/exploration.go` (`createExplorationLead` accepts `sourceReferenceId` + `origin`), `apps/web/src/api/exploration.ts` (`createLead` opts), `exploration_test.go`.

**Interfaces:** `createLead(projectId, text, opts?:{parentLeadId?, sourceReferenceId?})` posts `{text, parentLeadId, sourceReferenceId}`; server sets `origin='note'` when `sourceReferenceId` present (else `'manual'`). `LeadOrigin` enum gains `"note"`.

- [ ] Step 1: Add `"note"` to `LeadOrigin` (contract) + the Go CHECK constraint via migration `0055_lead_origin_note.sql` (drop+recreate the CHECK to include `'note'`; Down reverses). Step 2: extend handler body + validation (IDOR-check `sourceReferenceId`). Step 3: failing test `TestCreateLead_FromNoteSetsOrigin`. Step 4: regen/typecheck/full package. Step 5: commit `feat(exploration): create lead from a note (sourceReferenceId, origin=note)`.

### Task C2: Reading note → promote to a question in 探索

**Files:** `apps/web/src/workspace/blocks/exploration/ExplorationView.tsx` (a "从笔记新建问题" picker), a client fn to list references with non-empty `readingNote` (reuse the loaded references — they already carry `readingNote`).

> **Detail-on-dispatch (frontend).** Binding scope:
- In 探索 (overview), a plain-copy affordance "从笔记新建问题" opens a list of the project's reading notes (references whose `readingNote` is non-empty; the note text + source title). Selecting one calls `createLead(projectId, noteText, {sourceReferenceId})` → a new root question appears. No auto-promotion.

- [ ] Step 1: Build the note picker (reads references already in memory; shows note + source). Step 2: promote → `createLead` with `sourceReferenceId`, refetch. Step 3: typecheck + live-verify (make a note in 阅读, see it selectable in 探索, promote it). Step 4: commit `feat(exploration): promote a reading note into a question node`.

### Task C3: Seed the driving question; cold-paste asks "what question?"

**Files:** `apps/web/src/workspace/blocks/exploration/ExplorationView.tsx` (empty-state seed + paste flow); backend already supports lead+reference creation.

> **Detail-on-dispatch (frontend).** Binding scope:
- **Empty state:** if a project has no root question, offer the project's driving question (from the project's topic/proposal already in memory) as a one-click "把它作为第一个问题" → `createLead(projectId, drivingQuestion)`. Student-authored text, not AI-invented.
- **Cold-paste a paper (a URL) with no question yet:** a plain-copy input "贴一篇论文的链接"; on paste, ask (plain prompt) "这篇想回答什么问题？" — the student's answer becomes a root question (`createLead`), and the pasted paper is adopted under it (reuse `/adopt` with the resolved metadata, or the existing add-source path). Papers never become roots.

- [ ] Step 1: Empty-state driving-question seed. Step 2: cold-paste → ask-question → root + adopt. Step 3: typecheck + live-verify + screenshot. Step 4: commit `feat(exploration): seed driving question; cold-paste asks what question it serves`.

### ▶ Deploy checkpoint 3 (final, after P-C)

- [ ] Full backend suite + web typecheck/contracts. Push. `./.deploy-local/deploy.sh full` (C1 migration). Verify `DEPLOY OK`. Run the Phoebe acceptance mainline (spec §11) on prod: driving question → dig via OpenAlex → tray → adopt → 反例 opens a new question → 印记 proposes a `反驳/张力` edge → confirm → note→question → export unaffected.

---

## Self-Review notes (controller)

- **Spec coverage:** §1 switcher/rename/icons → A7; §2.1 labeled question graph → B1–B4; §2.2 inside sparse tree → A6; §3 three actions + tray → A6; §4 library one-shelf+status → A1/A7; §5 note→question → C1/C2; §6 OpenAlex+Crossref → A2/A3; §7 seeding+cold-paste → C3; §8 data model deltas → A1/B1/C1; §9 不做 → Global Constraints + plain-copy audits; §10 phasing → P-A/P-B/P-C; §11 acceptance → Deploy checkpoint 3.
- **No-compat:** migrations 0053–0055 add columns/tables only (no data to migrate); Downs written for round-trip tests.
- **Metering:** only /dig is un-metered (OpenAlex, not LLM); /adopt un-metered (no LLM); /edges/propose metered (LLM). All other exploration endpoints unchanged.
- **Frontend "detail-on-dispatch":** A6/B4/C2/C3 carry binding scope; the controller supplies the exact before→after chrome map at dispatch time (a React rework is specified by behavior + targets, not verbatim transcription). Backend tasks are code-complete.
