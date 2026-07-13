# Slice 6b — Material View, Ingestion, Source Log · Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make the 素材 center pane real — the source dossier renders the project's actual materials with state derived from what the student did, the student can add sources themselves (the AI cannot), every source is logged, and the live CRAAP card's anchors highlight in the article body.

**Architecture:** The backend's `StudioProjection` gains a `materials[]` array whose `locked` / `role` / `anchors` fields are *derived* from the existing CRAAP mint (an `evaluated-as` edge → an evidence node carrying `source_quality.risk_note`) and from the persisted `card_instances.anchors`. Two new endpoints — ingest a source (`internal/materialize` wired at last) and log an open — write `material` + `source_log_entry` rows. The frontend replaces its material fixture with the wire type, adds a 添加信源 form and a 检索日志 panel, and threads the live card's anchors from `StudioShell` down to the already-waiting `SourceDossier`.

**Tech Stack:** Go (`net/http`, pgx/sqlc/goose, testcontainers) · TypeScript + React + vitest · Zod contracts.

**Spec:** `docs/superpowers/specs/2026-07-11-slice-6b-material-view-design.md` — read §3 (derivation rules), §4 (ingestion), §5 (log), §6 (anchor merge) before starting your task.

## Global Constraints

- **The client never calls a model directly.** All LLM/network egress is server-side. The ingestion fetch runs in `apps/api` behind `HasEntitlement`.
- **The AI gets no ingestion capability.** Do not add a `summon`-able tool, agent function, or coach path that creates a material. Only the student's explicit `POST /materials` may create one. This is RL-2 enforced structurally.
- **Nothing is invented.** Every field the dossier renders must have a real producer (spec §3). Do not render, default, or fabricate a source verdict (`可信` / `存疑` / `偏弱`).
- **Binding Chinese copy is verbatim.** From the design (`docs/design/思维印记_工作区.dc.html`) and the spec:
  - source chips: `待评估` (unevaluated) · `✓ 已锁定` (locked)
  - empty role: `尚未写「作用与风险」`
  - dossier header: `信源档案 · 已收集 {n} 篇` · `已锁定 {k}/{n}`
  - log header: `检索日志 · 已记录 {n} 条`
  - add-source affordance: `添加信源`
  - fetch failure: `取不到这个链接的正文，可以直接把正文粘进来。` · empty body: `正文是空的。`
  If a test disagrees with this copy, **fix the test, never the copy.**
- **Icons are inline SVG.** Never import `lucide-react` or any icon package.
- **Go tests run serialized:** `CGO_ENABLED=0 go test -p 1 ./...` from `apps/api`. Parallel runs hang on Docker contention.
- **sqlc:** regenerate with `make sqlc` (= `CGO_ENABLED=0 go tool sqlc generate`) from `apps/api`. sqlc does **not** delete stale generated files — check `git status` after.
- **No orphans.** When a file's last consumer dies, the file dies with it. `apps/web/src/studio/material/fixtures.ts` + `fixtures.test.ts` must not exist at the end of this slice.
- **Errors use the existing `httpx` envelope.** `httpx.ErrBadRequest(code, msg, nil)` → 400 `{"error":{...}}`. There is no 422 helper; do not add one.

---

### Task 1: The `MaterialSource` contract

**Files:**
- Modify: `packages/contracts/src/studioState.ts`
- Test: `packages/contracts/src/studioState.test.ts`

**Interfaces:**
- Consumes: the existing `Anchor` schema from `packages/contracts/src/annotate.ts` (already exported through `src/index.ts`).
- Produces: `MaterialSource` (schema + type) and `StudioProjection.materials: MaterialSource[]`. Task 3 mirrors this in Go; Tasks 6–9 consume it in React.

- [ ] **Step 1: Write the failing test**

Append to `packages/contracts/src/studioState.test.ts`:

```ts
import { MaterialSource, StudioProjection } from "./studioState";

describe("MaterialSource", () => {
  const valid = {
    id: "00000000-0000-0000-0000-000000000110",
    title: "《卫星图看中国变绿》",
    sourceUrl: "",
    kind: "article",
    origin: "fetched",
    blocks: [{ id: "b1", text: "过去二十年里……" }],
    locked: false,
    role: "",
    tier: "二手 · 需追源",
    takeaway: "把 NASA 的图转述成「中国让地球更可持续」。",
    anchors: [],
  };

  it("accepts a projected source", () => {
    expect(MaterialSource.parse(valid)).toEqual(valid);
  });

  it("requires every derived field — no optionals to hide a missing producer", () => {
    const { locked, ...withoutLocked } = valid;
    expect(() => MaterialSource.parse(withoutLocked)).toThrow();
  });

  it("carries materials on the projection", () => {
    const proj = StudioProjection.parse({
      project: { title: "t", qualLabel: "q" },
      stations: [],
      activeStation: "S3",
      coach: { anchor: "", messages: [], equipment: [] },
      onboarding: { restatePrompt: "", rubricRows: [], planSteps: [] },
      materials: [valid],
    });
    expect(proj.materials[0]!.title).toBe("《卫星图看中国变绿》");
  });
});
```

- [ ] **Step 2: Run it to verify it fails**

Run: `npm test -w @mind-imprint/contracts`
Expected: FAIL — `MaterialSource` is not exported.

- [ ] **Step 3: Implement**

In `packages/contracts/src/studioState.ts`, import `Anchor` from `./annotate` (match the file's existing import style) and add above `StudioProjection`:

```ts
export const MaterialBlock = z.object({ id: z.string(), text: z.string() });
export type MaterialBlock = z.infer<typeof MaterialBlock>;

// One source in the 素材 dossier. Every field has a real producer (spec §3):
// locked/role come from the CRAAP mint (evaluated-as edge → evidence node's
// source_quality.risk_note), tier/takeaway from the student's source-log entry,
// anchors from the persisted card_instances.anchors targeting this material.
// There is deliberately NO verdict field — a 可信/存疑 judgment has no honest
// producer and would have to be fabricated.
export const MaterialSource = z.object({
  id: z.string(),
  title: z.string(),
  sourceUrl: z.string(),
  kind: z.string(),
  origin: z.string(),
  blocks: z.array(MaterialBlock),
  locked: z.boolean(),
  role: z.string(),
  tier: z.string(),
  takeaway: z.string(),
  anchors: z.array(Anchor),
});
export type MaterialSource = z.infer<typeof MaterialSource>;
```

Then add `materials: z.array(MaterialSource)` to the `StudioProjection` object (required, not optional — a projection with no materials sends `[]`).

Export `MaterialSource` / `MaterialBlock` from `packages/contracts/src/index.ts` alongside the other `studioState` exports.

- [ ] **Step 4: Run tests to verify they pass**

Run: `npm test -w @mind-imprint/contracts`
Expected: PASS, all suites green.

- [ ] **Step 5: Commit**

```bash
git add packages/contracts
git commit -m "feat(refactor2): MaterialSource contract on the studio projection"
```

---

### Task 2: Migration 0020 + source-log queries

**Files:**
- Create: `apps/api/internal/store/migrations/0020_material_source_log.sql`
- Create: `apps/api/internal/store/queries/source_log.sql`
- Modify: `apps/api/internal/store/queries/material.sql`
- Test: `apps/api/internal/store/migrate_test.go` (existing testcontainers migration test — extend)

**Interfaces:**
- Produces (sqlc-generated, used by Tasks 3/4/5): `CreateSourceLogEntry(ctx, CreateSourceLogEntryParams{ProjectID, MaterialID, Url, Title, Takeaway, Tier}) (SourceLogEntry, error)` · `ListSourceLogByProject(ctx, projectID) ([]SourceLogEntry, error)` · `AddSourceTimeSpent(ctx, AddSourceTimeSpentParams{MaterialID, TimeSpentS}) error` · `GetSourceLogByMaterial(ctx, materialID) (SourceLogEntry, error)`.
- Also: `material.task_id` becomes nullable, so `CreateProjectMaterial`'s first param accepts an invalid (NULL) `pgtype.UUID`.

- [ ] **Step 1: Write the failing test**

Append to `apps/api/internal/store/migrate_test.go` (follow the file's existing testcontainers harness — reuse whatever helper it already uses to bring up a migrated DB; do not write a new one):

```go
func TestMigration0020MaterialSourceLog(t *testing.T) {
	ctx := context.Background()
	pool := newMigratedPool(t, ctx) // existing helper in this package

	// material.task_id is nullable — a project material needs no task.
	var isNullable string
	if err := pool.QueryRow(ctx,
		`SELECT is_nullable FROM information_schema.columns
		 WHERE table_name='material' AND column_name='task_id'`).Scan(&isNullable); err != nil {
		t.Fatal(err)
	}
	if isNullable != "YES" {
		t.Fatalf("material.task_id is_nullable = %q, want YES", isNullable)
	}

	// source_log_entry.material_id exists and points at material.
	var count int
	if err := pool.QueryRow(ctx,
		`SELECT count(*) FROM information_schema.columns
		 WHERE table_name='source_log_entry' AND column_name='material_id'`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatal("source_log_entry.material_id missing")
	}

	// The seeded demo materials carry real blocks — not '[]'.
	var blocks []byte
	if err := pool.QueryRow(ctx,
		`SELECT blocks FROM material WHERE id = '00000000-0000-0000-0000-000000000110'`).Scan(&blocks); err != nil {
		t.Fatal(err)
	}
	var parsed []map[string]any
	if err := json.Unmarshal(blocks, &parsed); err != nil {
		t.Fatal(err)
	}
	if len(parsed) < 3 {
		t.Fatalf("seeded blog material has %d blocks, want >= 3 (the real article text)", len(parsed))
	}

	// And each seeded material has a source-log entry.
	if err := pool.QueryRow(ctx,
		`SELECT count(*) FROM source_log_entry WHERE project_id = '00000000-0000-0000-0000-000000000101'`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 2 {
		t.Fatalf("seeded source_log_entry count = %d, want 2", count)
	}
}
```

- [ ] **Step 2: Run it to verify it fails**

Run (from `apps/api`): `CGO_ENABLED=0 go test -p 1 ./internal/store/ -run TestMigration0020 -v`
Expected: FAIL — `material.task_id is_nullable = "NO"`.

- [ ] **Step 3: Write the migration**

`apps/api/internal/store/migrations/0020_material_source_log.sql`:

```sql
-- +goose Up
-- Slice 6b. Three things the live 素材 layer needs:
--
-- 1. material.task_id NOT NULL was Slice-3 debt: a legacy FK to the task
--    surface that 5d deleted. A project-scoped ingestion handler has no task
--    to point at, and every project created after project-creation lands will
--    have none either. Drop the constraint; keep the column (old rows still
--    reference their task).
ALTER TABLE material ALTER COLUMN task_id DROP NOT NULL;

-- 2. source_log_entry gains its material link. The table has never been
--    written to (no queries existed until this slice), so there is no data to
--    backfill.
ALTER TABLE source_log_entry
  ADD COLUMN material_id uuid REFERENCES material(id) ON DELETE CASCADE;
CREATE INDEX source_log_entry_material_idx ON source_log_entry (material_id);

-- 3. The demo project's two materials have carried blocks='[]' since 0018 —
--    the real article text lived only in a frontend fixture, which means the
--    CRAAP anchor generator has been running against empty text. Move the
--    content server-side. Real sources (AGENTS.md: no lorem ipsum): a
--    self-media blog riffing on Chen et al. 2019, Nature Sustainability
--    (DOI 10.1038/s41893-019-0220-7), and the finding itself.
UPDATE material SET
  source_url = 'https://mp.weixin.qq.com/s/demo-china-greening',
  blocks = '[
    {"id":"b1","text":"过去二十年里发生了一件几乎没人注意到的事：根据 NASA 卫星数据，地球比 2000 年整整绿了一圈，而这背后最大的推手，是中国。"},
    {"id":"b2","text":"变化大到能从太空里看见。2000 到 2017 年间，NASA 的 MODIS 卫星记录到全球绿叶面积增加了 5%，相当于新增了一整片亚马逊雨林那么大的绿色；仅占全球陆地面积 9% 的中国和印度，就贡献了这其中三分之一以上的增量。"},
    {"id":"b3","text":"很难不把这读成一个信号：那个曾经和雾霾、燃煤电厂划等号的国家，如今悄悄成了地球变绿背后最大的力量——中国的环保政策，正在起效。"}
  ]'::jsonb
WHERE id = '00000000-0000-0000-0000-000000000110';

UPDATE material SET
  title = 'Chen et al. (2019), Nature Sustainability',
  source_url = 'https://doi.org/10.1038/s41893-019-0220-7',
  blocks = '[
    {"id":"b1","text":"基于 NASA MODIS 卫星 2000–2017 年数据：全球绿叶面积净增 5%，中国、印度合计贡献全球净增量的三分之一以上；增量主要来自农业集约化耕作与大规模植树工程，而非森林自然恢复。"}
  ]'::jsonb
WHERE id = '00000000-0000-0000-0000-000000000111';

INSERT INTO source_log_entry (id, project_id, material_id, url, title, takeaway, tier, time_spent_s) VALUES
  ('00000000-0000-0000-0000-000000000170', '00000000-0000-0000-0000-000000000101', '00000000-0000-0000-0000-000000000110',
   'https://mp.weixin.qq.com/s/demo-china-greening', '《卫星图看中国变绿》',
   '把 NASA 的卫星图转述成「中国让地球更可持续」，结论被放大了，需要横向核实。', '二手 · 需追源', 240),
  ('00000000-0000-0000-0000-000000000171', '00000000-0000-0000-0000-000000000101', '00000000-0000-0000-0000-000000000111',
   'https://doi.org/10.1038/s41893-019-0220-7', 'Chen et al. (2019), Nature Sustainability',
   '卫星确认地球在变绿、中国贡献最大，但机制是农业集约化与人工造林——论文本身没说这等于「更可持续」。', '一手论文', 610)
ON CONFLICT (id) DO NOTHING;

-- +goose Down
DELETE FROM source_log_entry WHERE id IN (
  '00000000-0000-0000-0000-000000000170', '00000000-0000-0000-0000-000000000171');
DROP INDEX IF EXISTS source_log_entry_material_idx;
ALTER TABLE source_log_entry DROP COLUMN material_id;
UPDATE material SET blocks = '[]'::jsonb, source_url = NULL
  WHERE id IN ('00000000-0000-0000-0000-000000000110', '00000000-0000-0000-0000-000000000111');
ALTER TABLE material ALTER COLUMN task_id SET NOT NULL;
```

- [ ] **Step 4: Write the queries**

`apps/api/internal/store/queries/source_log.sql`:

```sql
-- The source log (spec §5, product-spec §9/S2). One row per source the student
-- brought in; time_spent_s accumulates across opens. RL-2's ledger.

-- name: CreateSourceLogEntry :one
INSERT INTO source_log_entry (project_id, material_id, url, title, takeaway, tier)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: ListSourceLogByProject :many
SELECT * FROM source_log_entry
WHERE project_id = $1
ORDER BY opened_at;

-- name: GetSourceLogByMaterial :one
SELECT * FROM source_log_entry WHERE material_id = $1;

-- name: AddSourceTimeSpent :exec
UPDATE source_log_entry
SET time_spent_s = time_spent_s + $2
WHERE material_id = $1;
```

- [ ] **Step 5: Regenerate sqlc and run the test**

```bash
cd apps/api && make sqlc && git status --short internal/store/sqlc
CGO_ENABLED=0 go test -p 1 ./internal/store/ -run TestMigration0020 -v
```
Expected: PASS. Then run the whole store package: `CGO_ENABLED=0 go test -p 1 ./internal/store/...` — expected PASS (the pre-existing migrate/sqlc tests must stay green).

- [ ] **Step 6: Commit**

```bash
git add apps/api/internal/store
git commit -m "feat(refactor2): migration 0020 — nullable material.task_id, source_log.material_id, real seeded blocks"
```

---

### Task 3: Project the materials (derivation)

**Files:**
- Modify: `apps/api/internal/studio/load.go`, `apps/api/internal/studio/dto.go`, `apps/api/internal/studio/projection.go`
- Test: `apps/api/internal/studio/projection_test.go`, `apps/api/internal/studio/dto_parity_test.go`

**Interfaces:**
- Consumes: Task 1's Zod shape (the Go DTO's JSON tags must match it exactly — `dto_parity_test.go` enforces this); Task 2's `ListSourceLogByProject`.
- Produces: `studio.MaterialDTO` + `StudioProjection.Materials []MaterialDTO`, consumed by `GET /projects/{id}` (already calls `studio.Project`) and by Task 6's frontend mapping.

**This is the slice's central claim — read spec §3 before writing code.** Derivation, not decoration:

- `locked` ⇔ a `graph_edge` exists with `type='evaluated-as'`, `from_kind='material'`, `from_id=<material.id>`. (`GraphEffects` in `internal/agent/card_effects.go` mints exactly this when a CRAAP card completes.)
- `role` = that edge's target evidence node's `body.source_quality.risk_note`, or `""`.
- `tier` / `takeaway` = the material's `source_log_entry` row, or `""`.
- `anchors` = every anchor across the project's `card_instances.anchors` whose own `material_id` equals this material's id. (`card_instances` has **no** `material_id` column; the anchor object carries it.)

- [ ] **Step 1: Write the failing test**

Add to `apps/api/internal/studio/projection_test.go` (this package's tests are pure — they build `ProjectData` in memory; follow the existing fixtures in the file for row construction):

```go
func TestProjectDerivesMaterialState(t *testing.T) {
	matA := uuid.MustParse("00000000-0000-0000-0000-0000000000aa") // evaluated
	matB := uuid.MustParse("00000000-0000-0000-0000-0000000000bb") // untouched
	evid := uuid.MustParse("00000000-0000-0000-0000-0000000000cc")

	d := baseProjectData(t) // existing helper in this file
	d.Materials = []sqlc.Material{
		{ID: pgUUID(matA), Title: "《卫星图看中国变绿》", Kind: "article", Source: "fetched",
			Blocks: []byte(`[{"id":"b1","text":"过去二十年……"}]`)},
		{ID: pgUUID(matB), Title: "IEA Renewable Investment", Kind: "article", Source: "pasted",
			Blocks: []byte(`[{"id":"b1","text":"China ranks first."}]`)},
	}
	d.SourceLog = []sqlc.SourceLogEntry{
		{MaterialID: pgUUID(matA), Tier: pgText("二手 · 需追源"), Takeaway: "结论被放大了。"},
	}
	// The CRAAP mint: evidence node + evaluated-as edge from the material.
	d.Nodes = append(d.Nodes, sqlc.GraphNode{
		ID: evid, Type: "evidence", Author: "student",
		Body: []byte(`{"source_quality":{"authority":"只是一个博主","risk_note":"入口来源，不能直接引用。"}}`),
	})
	d.Edges = append(d.Edges, sqlc.GraphEdge{
		Type: "evaluated-as", FromKind: "material", FromID: matA.String(),
		ToKind: "graph_node", ToID: evid.String(),
	})
	// A persisted CRAAP card whose anchors target matA.
	d.Cards = append(d.Cards, sqlc.CardInstance{
		CardID: "craap", Status: "completed",
		Anchors: []byte(`[{"id":"a1","material_id":"` + matA.String() + `","block_id":"b1","start":0,"end":4,"quote":"过去二十年","dimension":"authority","author":"ai","question":"原始出处是谁？","answer":"只是一个博主"}]`),
	})

	proj, err := Project(testSkill(t), testSpecByID, d)
	if err != nil {
		t.Fatal(err)
	}
	if len(proj.Materials) != 2 {
		t.Fatalf("materials = %d, want 2", len(proj.Materials))
	}

	a := proj.Materials[0]
	if !a.Locked {
		t.Error("evaluated material: Locked = false, want true (an evaluated-as edge exists)")
	}
	if a.Role != "入口来源，不能直接引用。" {
		t.Errorf("Role = %q, want the minted evidence node's source_quality.risk_note", a.Role)
	}
	if a.Tier != "二手 · 需追源" || a.Takeaway != "结论被放大了。" {
		t.Errorf("Tier/Takeaway = %q/%q, want the source-log values", a.Tier, a.Takeaway)
	}
	if len(a.Anchors) != 1 {
		t.Errorf("Anchors = %d, want 1 (the card anchor targeting this material)", len(a.Anchors))
	}

	b := proj.Materials[1]
	if b.Locked || b.Role != "" || b.Tier != "" || b.Takeaway != "" || len(b.Anchors) != 0 {
		t.Errorf("untouched material carries state it never earned: %+v", b)
	}
}
```

If `baseProjectData` / `pgUUID` / `pgText` / `testSkill` / `testSpecByID` helpers don't exist under those exact names, use whatever the file already has — **do not invent a second harness.**

- [ ] **Step 2: Run it to verify it fails**

Run: `CGO_ENABLED=0 go test -p 1 ./internal/studio/ -run TestProjectDerivesMaterialState -v`
Expected: FAIL to compile — `ProjectData` has no `Materials` field.

- [ ] **Step 3: Implement**

`load.go` — load both:

```go
materials, err := q.ListMaterialsByProject(ctx, pg)
if err != nil {
    return ProjectData{}, err
}
sourceLog, err := q.ListSourceLogByProject(ctx, projectID)
if err != nil {
    return ProjectData{}, err
}
```
and add `Materials: materials, SourceLog: sourceLog` to the `ProjectData` literal + the struct in `projection.go` (or wherever `ProjectData` is declared).

`dto.go` — add, with JSON tags matching Task 1's Zod keys exactly:

```go
type MaterialBlockDTO struct {
	ID   string `json:"id"`
	Text string `json:"text"`
}

// MaterialDTO is one source in the 素材 dossier. locked/role are DERIVED from
// the CRAAP mint (an evaluated-as edge → an evidence node's
// source_quality.risk_note), tier/takeaway from the student's source-log entry,
// anchors from the persisted card_instances.anchors targeting this material.
// There is no verdict field — see spec §3.
type MaterialDTO struct {
	ID        string             `json:"id"`
	Title     string             `json:"title"`
	SourceURL string             `json:"sourceUrl"`
	Kind      string             `json:"kind"`
	Origin    string             `json:"origin"`
	Blocks    []MaterialBlockDTO `json:"blocks"`
	Locked    bool               `json:"locked"`
	Role      string             `json:"role"`
	Tier      string             `json:"tier"`
	Takeaway  string             `json:"takeaway"`
	Anchors   []json.RawMessage  `json:"anchors"`
}
```
and `Materials []MaterialDTO \`json:"materials"\`` on `StudioProjection`. Initialize it to `[]MaterialDTO{}` (never nil) so the wire carries `[]`, not `null` — Task 1's schema requires an array.

`projection.go` — a `projectMaterials(d ProjectData) []MaterialDTO` helper called from `Project`:

```go
// projectMaterials derives each source's dossier state. Nothing here invents a
// judgment: locked/role exist only because the student completed a CRAAP card
// and the mint wrote them (agent.GraphEffects).
func projectMaterials(d ProjectData) []MaterialDTO {
	nodesByID := map[string]sqlc.GraphNode{}
	for _, n := range d.Nodes {
		nodesByID[n.ID.String()] = n
	}
	// material id → the evidence node its evaluated-as edge points at.
	evidence := map[string]sqlc.GraphNode{}
	for _, e := range d.Edges {
		if e.Type != "evaluated-as" || e.FromKind != "material" {
			continue
		}
		if n, ok := nodesByID[e.ToID]; ok {
			evidence[e.FromID] = n
		}
	}
	log := map[string]sqlc.SourceLogEntry{}
	for _, s := range d.SourceLog {
		if s.MaterialID.Valid {
			log[uuid.UUID(s.MaterialID.Bytes).String()] = s
		}
	}
	// Anchors carry their own material_id — card_instances has no such column.
	anchorsByMaterial := map[string][]json.RawMessage{}
	for _, c := range d.Cards {
		var raw []json.RawMessage
		if err := json.Unmarshal(c.Anchors, &raw); err != nil {
			continue
		}
		for _, a := range raw {
			var probe struct {
				MaterialID string `json:"material_id"`
			}
			if err := json.Unmarshal(a, &probe); err != nil || probe.MaterialID == "" {
				continue
			}
			anchorsByMaterial[probe.MaterialID] = append(anchorsByMaterial[probe.MaterialID], a)
		}
	}

	out := make([]MaterialDTO, 0, len(d.Materials))
	for _, m := range d.Materials {
		id := uuid.UUID(m.ID.Bytes).String()
		dto := MaterialDTO{
			ID: id, Title: m.Title, Kind: m.Kind, Origin: m.Source,
			Blocks: []MaterialBlockDTO{}, Anchors: []json.RawMessage{},
		}
		if m.SourceUrl.Valid {
			dto.SourceURL = m.SourceUrl.String
		}
		_ = json.Unmarshal(m.Blocks, &dto.Blocks)
		if n, ok := evidence[id]; ok {
			dto.Locked = true
			dto.Role = riskNote(n.Body)
		}
		if s, ok := log[id]; ok {
			dto.Takeaway = s.Takeaway
			if s.Tier.Valid {
				dto.Tier = s.Tier.String
			}
		}
		if as, ok := anchorsByMaterial[id]; ok {
			dto.Anchors = as
		}
		out = append(out, dto)
	}
	return out
}

// riskNote reads body.source_quality.risk_note off a minted evidence node.
func riskNote(body []byte) string {
	var b struct {
		SourceQuality map[string]string `json:"source_quality"`
	}
	if err := json.Unmarshal(body, &b); err != nil {
		return ""
	}
	return b.SourceQuality["risk_note"]
}
```

Adjust the sqlc field names (`m.SourceUrl`, `s.Tier`, `n.Body`, edge `FromID`/`ToID` types) to whatever `internal/store/sqlc` actually generated — read the generated structs, don't guess. If `GraphEdge.FromID` is a `string` there, the code above is right as written; if it is `pgtype.UUID`, convert.

- [ ] **Step 4: Verify the derivation is load-bearing, then run tests**

Temporarily hard-code `dto.Locked = true; dto.Role = "入口来源，不能直接引用。"` for every material and re-run the test — it **must fail** on the untouched material (`material carries state it never earned`). Revert the hard-code. Record this proof in your report.

Run: `CGO_ENABLED=0 go test -p 1 ./internal/studio/...`
Expected: PASS, including the existing `dto_parity_test.go` (extend its field list with the new DTO if the test enumerates fields).

- [ ] **Step 5: Commit**

```bash
git add apps/api/internal/studio
git commit -m "feat(refactor2): project materials with derived locked/role/tier/anchors"
```

---

### Task 4: Ingestion endpoint

**Files:**
- Create: `apps/api/internal/api/materials.go`
- Create: `apps/api/internal/api/materials_test.go`
- Modify: `apps/api/internal/api/api.go` (route), `apps/api/internal/api/deps.go` (or wherever `Deps` lives — add the fetcher)

**Interfaces:**
- Consumes: `materialize.NewFetcher()` → `FetchReadable(ctx, url) (title, text string, err error)`; `materialize.Segment(text) []materialize.Block`; Task 2's `CreateSourceLogEntry`; Task 3's `MaterialDTO` (the 201 body reuses it — do not define a second shape).
- Produces: `POST /api/v1/projects/{id}/materials`, called by Task 8's form.

- [ ] **Step 1: Write the failing test**

`apps/api/internal/api/materials_test.go` — use this package's existing testcontainers harness (`maintest_test.go`; copy the setup style of `projects_test.go`):

```go
func TestIngestMaterialFromURL(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(`<html><head><title>Global Greening</title></head><body>
			<p>Leaf area rose about five percent between 2000 and 2017.</p>
			<p>China and India account for a third of the net increase.</p></body></html>`))
	}))
	defer srv.Close()

	env := newTestEnv(t) // existing helper
	// The fetcher must be unguarded in tests (httptest binds loopback, which the
	// SSRF guard blocks by design). Inject a test fetcher through Deps — see Step 3.

	body := fmt.Sprintf(`{"url":%q,"takeaway":"变绿是真的，但不等于可持续。","tier":"一手数据"}`, srv.URL)
	rec := env.post(t, "/api/v1/projects/"+env.ProjectID+"/materials", body)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201: %s", rec.Code, rec.Body.String())
	}

	var out struct {
		ID     string `json:"id"`
		Title  string `json:"title"`
		Blocks []struct{ Text string } `json:"blocks"`
	}
	mustJSON(t, rec.Body.Bytes(), &out)
	if len(out.Blocks) != 2 {
		t.Fatalf("blocks = %d, want 2 (one per paragraph)", len(out.Blocks))
	}

	// The source-log entry landed in the same transaction.
	var takeaway, tier string
	if err := env.Pool.QueryRow(env.Ctx,
		`SELECT takeaway, tier FROM source_log_entry WHERE material_id = $1`, out.ID).Scan(&takeaway, &tier); err != nil {
		t.Fatalf("no source_log_entry for the ingested material: %v", err)
	}
	if takeaway != "变绿是真的，但不等于可持续。" || tier != "一手数据" {
		t.Errorf("log entry = %q/%q, want the student's takeaway/tier", takeaway, tier)
	}
}

func TestIngestMaterialFetchFailureWritesNothing(t *testing.T) {
	env := newTestEnv(t)
	materialsBefore := env.countRows(t, "material")
	logBefore := env.countRows(t, "source_log_entry")

	rec := env.post(t, "/api/v1/projects/"+env.ProjectID+"/materials",
		`{"url":"https://example.invalid/nope","takeaway":"x","tier":"y"}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "取不到这个链接的正文") {
		t.Errorf("body = %s, want the honest fetch-failure copy", rec.Body.String())
	}
	if got := env.countRows(t, "material"); got != materialsBefore {
		t.Errorf("material rows = %d, want %d — a failed fetch must write nothing", got, materialsBefore)
	}
	if got := env.countRows(t, "source_log_entry"); got != logBefore {
		t.Errorf("source_log_entry rows = %d, want %d — neither row may land without the other", got, logBefore)
	}
}

func TestIngestMaterialFromPastedText(t *testing.T) {
	env := newTestEnv(t)
	rec := env.post(t, "/api/v1/projects/"+env.ProjectID+"/materials",
		`{"title":"我抄下来的一段","text":"第一段。\n\n第二段。","takeaway":"t","tier":"二手"}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201: %s", rec.Code, rec.Body.String())
	}
	var out struct {
		Origin string `json:"origin"`
		Blocks []any  `json:"blocks"`
	}
	mustJSON(t, rec.Body.Bytes(), &out)
	if out.Origin != "pasted" || len(out.Blocks) != 2 {
		t.Errorf("origin=%q blocks=%d, want pasted/2", out.Origin, len(out.Blocks))
	}
}

func TestIngestMaterialRejectsOtherUsersProject(t *testing.T) {
	env := newTestEnv(t)
	rec := env.postAs(t, otherUser, "/api/v1/projects/"+env.ProjectID+"/materials",
		`{"title":"t","text":"x","takeaway":"t","tier":"二手"}`)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404 (ownership hidden as not-found)", rec.Code)
	}
}
```

Adapt the helper names (`newTestEnv`, `env.post`, `mustJSON`, `countRows`, `postAs`) to whatever `maintest_test.go` / `projects_test.go` already provide. **Reuse the existing harness; do not build a parallel one.**

- [ ] **Step 2: Run to verify it fails**

Run: `CGO_ENABLED=0 go test -p 1 ./internal/api/ -run TestIngestMaterial -v`
Expected: FAIL — 404, the route does not exist.

- [ ] **Step 3: Implement**

Add to `Deps` (next to `Provider`, `Queries`):

```go
// Fetcher turns a student-supplied URL into readable text. Injected so tests
// can bypass the SSRF guard (httptest binds loopback, which the production
// guard blocks by design). Never used by the agent — only the student's
// explicit POST /materials reaches it (RL-2, spec §4).
Fetcher interface {
    FetchReadable(ctx context.Context, rawURL string) (string, string, error)
}
```
Wire `materialize.NewFetcher()` in `cmd/api/main.go`.

`apps/api/internal/api/materials.go`:

```go
package api

// ingestMaterialReq is one of two shapes: a URL to fetch, or pasted text.
// takeaway/tier are the student's own words — the AI writes neither, and has no
// path to this handler at all (spec §4).
type ingestMaterialReq struct {
	URL      string `json:"url"`
	Title    string `json:"title"`
	Text     string `json:"text"`
	Takeaway string `json:"takeaway"`
	Tier     string `json:"tier"`
}

func (a *API) ingestMaterial(w http.ResponseWriter, r *http.Request) {
	projectID, ok := a.loadOwnedProject(w, r)
	if !ok {
		return
	}
	u, _ := UserFromContext(r.Context())
	entitled, err := HasEntitlement(r.Context(), u)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if !entitled {
		httpx.WriteError(w, r, httpx.ErrNotEntitled())
		return
	}
	var req ingestMaterialReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteError(w, r, httpx.ErrBadRequest("bad_json", "请求格式不对", nil))
		return
	}

	title, text, origin := req.Title, req.Text, "pasted"
	if req.URL != "" {
		t, body, ferr := a.d.Fetcher.FetchReadable(r.Context(), req.URL)
		if ferr != nil {
			httpx.WriteError(w, r, httpx.ErrBadRequest("fetch_failed",
				"取不到这个链接的正文，可以直接把正文粘进来。", nil))
			return
		}
		title, text, origin = t, body, "fetched"
	}
	blocks := materialize.Segment(text)
	if len(blocks) == 0 {
		httpx.WriteError(w, r, httpx.ErrBadRequest("empty_body", "正文是空的。", nil))
		return
	}
	if title == "" {
		title = req.URL // never a blank card in the dossier
	}
	rawBlocks, err := json.Marshal(blocks)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}

	// One transaction: a material with no log entry would be a source that was
	// never "opened" — precisely the state RL-2 forbids.
	// (Use this package's existing tx pattern — see how other multi-write
	// handlers acquire a pgx.Tx from the pool and build sqlc.New(tx).WithTx.)
	...
	mat, err := qtx.CreateProjectMaterial(ctx, sqlc.CreateProjectMaterialParams{
		TaskID:    pgtype.UUID{},            // NULL — the task surface is gone (migration 0020)
		ProjectID: pgtype.UUID{Bytes: projectID, Valid: true},
		Kind:      "article",
		Source:    origin,
		Title:     title,
		SourceUrl: pgtype.Text{String: req.URL, Valid: req.URL != ""},
		Blocks:    rawBlocks,
	})
	...
	_, err = qtx.CreateSourceLogEntry(ctx, sqlc.CreateSourceLogEntryParams{
		ProjectID: projectID, MaterialID: mat.ID,
		Url: req.URL, Title: title, Takeaway: req.Takeaway, Tier: pgtype.Text{String: req.Tier, Valid: req.Tier != ""},
	})
	...
	// commit, then respond with the same MaterialDTO shape the projection emits.
	httpx.WriteJSON(w, http.StatusCreated, dtoFor(mat, req.Takeaway, req.Tier))
}
```

Build the 201 body with `studio.MaterialDTO` (a fresh material is never locked, has no role, and no anchors — `Locked:false, Role:"", Anchors:[]`). Register the route in `api.go` beside the other project routes:

```go
mux.Handle("POST /api/v1/projects/{id}/materials", protected(a.ingestMaterial))
```

- [ ] **Step 4: Run tests**

Run: `CGO_ENABLED=0 go test -p 1 ./internal/api/ -run TestIngestMaterial -v`
Expected: PASS (4/4). Then `CGO_ENABLED=0 go test -p 1 ./internal/api/...` — expected PASS.

- [ ] **Step 5: Commit**

```bash
git add apps/api
git commit -m "feat(refactor2): student-only material ingestion (url fetch | pasted text) + source-log write"
```

---

### Task 5: Source-open logging endpoint

**Files:**
- Modify: `apps/api/internal/api/materials.go`, `apps/api/internal/api/api.go`
- Test: `apps/api/internal/api/materials_test.go`

**Interfaces:**
- Consumes: Task 2's `AddSourceTimeSpent`; the existing `AppendEvent` query (`internal/store/queries/event.sql`) — read its params before writing.
- Produces: `POST /api/v1/projects/{id}/materials/{mid}/open` with `{"time_spent_s": N}` → 204, called by Task 7's dossier timer.

- [ ] **Step 1: Write the failing test**

```go
func TestLogSourceOpenAccumulatesTime(t *testing.T) {
	env := newTestEnv(t)
	mid := env.seededMaterialID(t) // 00000000-0000-0000-0000-000000000110

	for i := 0; i < 2; i++ {
		rec := env.post(t, "/api/v1/projects/"+env.ProjectID+"/materials/"+mid+"/open", `{"time_spent_s":30}`)
		if rec.Code != http.StatusNoContent {
			t.Fatalf("status = %d, want 204: %s", rec.Code, rec.Body.String())
		}
	}

	var spent int
	if err := env.Pool.QueryRow(env.Ctx,
		`SELECT time_spent_s FROM source_log_entry WHERE material_id = $1`, mid).Scan(&spent); err != nil {
		t.Fatal(err)
	}
	// The seed starts this entry at 240s (migration 0020).
	if spent != 300 {
		t.Errorf("time_spent_s = %d, want 300 (240 seeded + 30 + 30 — it accumulates, never overwrites)", spent)
	}

	var events int
	if err := env.Pool.QueryRow(env.Ctx,
		`SELECT count(*) FROM event WHERE project_id = $1 AND type = 'source_opened'`, env.ProjectID).Scan(&events); err != nil {
		t.Fatal(err)
	}
	if events != 2 {
		t.Errorf("source_opened events = %d, want 2 — this is the ledger Slice 10's assessor reads", events)
	}
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `CGO_ENABLED=0 go test -p 1 ./internal/api/ -run TestLogSourceOpen -v`
Expected: FAIL — 404, no route.

- [ ] **Step 3: Implement**

```go
type logOpenReq struct {
	TimeSpentS int32 `json:"time_spent_s"`
}

// logSourceOpen accumulates reading time on the source-log entry and appends a
// source_opened event. Best-effort by policy: losing a timing sample must never
// break a student's reading, so a write failure warns and still returns 204.
func (a *API) logSourceOpen(w http.ResponseWriter, r *http.Request) {
	projectID, ok := a.loadOwnedProject(w, r)
	if !ok {
		return
	}
	mid, err := uuid.Parse(r.PathValue("mid"))
	if err != nil {
		httpx.WriteError(w, r, httpx.ErrNotFound("资源不存在"))
		return
	}
	var req logOpenReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.TimeSpentS < 0 {
		httpx.WriteError(w, r, httpx.ErrBadRequest("bad_json", "请求格式不对", nil))
		return
	}
	// ... AddSourceTimeSpent(mid, req.TimeSpentS); on error slog.Warn and continue.
	// ... AppendEvent(project_id, type="source_opened", payload={"surface":"studio","url":<log entry url>,"time_spent_s":N})
	//     — read event.sql's actual params first; on error slog.Warn.
	w.WriteHeader(http.StatusNoContent)
}
```

Route: `mux.Handle("POST /api/v1/projects/{id}/materials/{mid}/open", protected(a.logSourceOpen))`.

The event payload must satisfy the `source_opened` variant of `packages/contracts/src/event.ts:53` — `{type, surface, url, time_spent_s}`. Take `url` from the material's log entry (`GetSourceLogByMaterial`); a pasted source logs `url: ""`.

- [ ] **Step 4: Run tests**

Run: `CGO_ENABLED=0 go test -p 1 ./internal/api/...`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add apps/api
git commit -m "feat(refactor2): log source opens — accumulate time spent + append source_opened event"
```

---

### Task 6: Web API client + `views.material` un-stubbed

**Files:**
- Create: `apps/web/src/api/materials.ts`, `apps/web/src/api/materials.test.ts`
- Modify: `apps/web/src/api/index.ts`, `apps/web/src/studio/StudioContainer.tsx`, `apps/web/src/studio/state.ts`
- Modify: `apps/web/src/studio/StudioContainer.test.tsx`
- **Delete:** `apps/web/src/studio/material/fixtures.ts`, `apps/web/src/studio/material/fixtures.test.ts`

**Interfaces:**
- Consumes: Task 1's `MaterialSource`; Tasks 4/5's endpoints.
- Produces: `api.addMaterial(projectId, body): Promise<MaterialSource>` · `api.logSourceOpen(projectId, materialId, timeSpentS): Promise<void>` · `StudioState.views.material: MaterialSource[]` (replacing `SourceFixture[]`), consumed by Tasks 7–9.

- [ ] **Step 1: Write the failing test**

In `apps/web/src/studio/StudioContainer.test.tsx` (follow the file's existing fake-api pattern):

```tsx
it("projects the server's materials into the 素材 view — no fixture", async () => {
  const api = {
    listProjects: async () => [{ id: "p1", title: "t", qualLabel: "q", activeStation: "S3" }],
    getProject: async () => ({
      ...baseProjection,          // existing helper in this file
      activeStation: "S3",
      materials: [{
        id: "m1", title: "《卫星图看中国变绿》", sourceUrl: "https://x.test/a", kind: "article",
        origin: "fetched", blocks: [{ id: "b1", text: "过去二十年……" }],
        locked: false, role: "", tier: "二手 · 需追源", takeaway: "结论被放大了。", anchors: [],
      }],
    }),
  };
  render(<StudioContainer api={api as never} makeConversation={fakeConversation} />);
  expect(await screen.findByText("《卫星图看中国变绿》")).toBeInTheDocument();
  expect(screen.getByText(/信源档案 · 已收集 1 篇/)).toBeInTheDocument();
});
```

And in `apps/web/src/api/materials.test.ts`, assert `addMaterial` POSTs to `/api/v1/projects/p1/materials` with the body and parses the response through `MaterialSource` (mirror `projectCards.test.ts`'s fetch-mock style).

- [ ] **Step 2: Run to verify it fails**

Run: `npm test -w @mind-imprint/web -- StudioContainer`
Expected: FAIL — the dossier renders 0 sources (`views.material` is `[]`).

- [ ] **Step 3: Implement**

`apps/web/src/api/materials.ts` — two functions in the established client style (`client.ts`'s `request` helper, Zod-parsed responses):

```ts
export type AddMaterialBody =
  | { url: string; takeaway: string; tier: string }
  | { title: string; text: string; takeaway: string; tier: string };

export async function addMaterial(projectId: string, body: AddMaterialBody): Promise<MaterialSource> { ... }
export async function logSourceOpen(projectId: string, materialId: string, timeSpentS: number): Promise<void> { ... }
```
Export both through `api/index.ts`'s `api` object.

`state.ts`: `views.material: MaterialSource[]` (import the contract type; delete the `SourceFixture` import).

`StudioContainer.tsx`: `material: p.materials` in `toStudioState` — and update the stale comment (`material → Slice 6`) to name what's still stubbed (structure/writing/review → 7/8/9).

Then **delete** `material/fixtures.ts` + `fixtures.test.ts` and fix every import (`SourceDossier.tsx`, `SourceDossier.test.tsx`, and any other) to use `MaterialSource`. `grep -rn "SourceFixture\|fixtures" apps/web/src/studio/material` must come back empty.

- [ ] **Step 4: Run tests**

Run: `npm test -w @mind-imprint/web && npx tsc --noEmit -p apps/web`
Expected: PASS + clean typecheck. (Tests referencing the deleted fixture must be *updated to the wire type*, not deleted.)

- [ ] **Step 5: Commit**

```bash
git add apps/web packages
git commit -m "feat(refactor2): dossier reads real project materials; delete the material fixture"
```

---

### Task 7: Dossier chips, role fallback, and the reading timer

**Files:**
- Modify: `apps/web/src/studio/material/SourceDossier.tsx`
- Test: `apps/web/src/studio/material/SourceDossier.test.tsx`

**Interfaces:**
- Consumes: Task 6's `MaterialSource[]` + `api.logSourceOpen`.
- Produces: `SourceDossierProps.onOpenLogged?: (materialId: string, timeSpentS: number) => void` — Task 9 wires it to the API.

The dossier currently renders a fixture's invented `可信/存疑` chip and a `view: "article" | "summary"` switch. Replace with the binding design's state-derived chips.

- [ ] **Step 1: Write the failing test**

```tsx
it("renders the design's state-derived chips, never an invented verdict", () => {
  render(<SourceDossier sources={[unevaluated, locked]} />);
  expect(screen.getByText("待评估")).toBeInTheDocument();
  expect(screen.getByText("✓ 已锁定")).toBeInTheDocument();
  expect(screen.queryByText("可信")).not.toBeInTheDocument();
  expect(screen.queryByText("存疑")).not.toBeInTheDocument();
});

it("shows the design's fallback when 作用与风险 is unwritten", () => {
  render(<SourceDossier sources={[{ ...unevaluated, role: "" }]} />);
  expect(screen.getByText(/尚未写「作用与风险」/)).toBeInTheDocument();
});

it("reports the time spent when the student leaves a source", async () => {
  vi.useFakeTimers();
  const onOpenLogged = vi.fn();
  render(<SourceDossier sources={[unevaluated]} onOpenLogged={onOpenLogged} />);
  await userEvent.click(screen.getByText(unevaluated.title));
  vi.advanceTimersByTime(45_000);
  await userEvent.click(screen.getByText("返回信源列表"));
  expect(onOpenLogged).toHaveBeenCalledWith(unevaluated.id, 45);
  vi.useRealTimers();
});
```
(`userEvent` with fake timers needs `userEvent.setup({ advanceTimers: vi.advanceTimersByTime })` — set it up the way the repo's other timer tests do, if any; otherwise use `fireEvent`.)

- [ ] **Step 2: Run to verify it fails**

Run: `npm test -w @mind-imprint/web -- SourceDossier`
Expected: FAIL — chips read 存疑/可信; no timer.

- [ ] **Step 3: Implement**

- Chip: `locked ? "✓ 已锁定" : "待评估"`. Tone: locked → `{background:"#E7F3EE", color:"#4C9A82"}` (design `:2226`); else the neutral `{background:"#F1F2F6", color:"#5A6178"}`. Delete `CRAAP_TONE` and the `craapLabel` field entirely.
- Role line: `作用与风险：{source.role || "尚未写「作用与风险」"}`.
- Meta chips: `{origin === "fetched" ? "网页" : "粘贴"}` and `{tier}` (omit the tier chip when `tier === ""`).
- Read view: drop the `view: "article" | "summary"` branch — every source now has `blocks`, so every source renders through `Annotate`. Keep the 一句话摘要 panel below the article, fed by `takeaway` (render it only when non-empty).
- Timer: record `Date.now()` on open; on `backToList` (and on unmount while open) call `onOpenLogged(id, Math.round((Date.now() - openedAt) / 1000))` — skip if the elapsed time rounds to 0.
- Keep the existing `onEvent` prop and its `source_opened` emission; the server appends the canonical event, and this stays the client-side seam.

- [ ] **Step 4: Run tests**

Run: `npm test -w @mind-imprint/web -- SourceDossier`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add apps/web
git commit -m "feat(refactor2): dossier chips derive from state; report reading time on leave"
```

---

### Task 8: 添加信源 form + 检索日志 panel

**Files:**
- Create: `apps/web/src/studio/material/AddSourceForm.tsx`, `apps/web/src/studio/material/AddSourceForm.test.tsx`
- Create: `apps/web/src/studio/material/SourceLog.tsx`, `apps/web/src/studio/material/SourceLog.test.tsx`
- Modify: `apps/web/src/studio/material/SourceDossier.tsx` (mount both)

**Interfaces:**
- Consumes: Task 6's `api.addMaterial`; Task 1's `MaterialSource`.
- Produces: `AddSourceForm` (props: `onSubmit(body): Promise<void>`, `error?: string`) and `SourceLog` (props: `sources: MaterialSource[]`). Task 9 supplies `onSubmit` from the container.

The design has no add-affordance (it shows sources already collected), so this is a small extension **in the design's idiom** — reuse the dossier's card vocabulary: `#fff` card, `1px solid #E4E6EE`, `borderRadius: 12`, inputs `1px solid #E1E4ED` / `borderRadius: 10` / `fontSize: 13.5`, primary text `#1C2333`, muted `#8A93A6`.

- [ ] **Step 1: Write the failing test**

```tsx
// AddSourceForm.test.tsx
it("submits a URL with the student's takeaway and tier", async () => {
  const onSubmit = vi.fn().mockResolvedValue(undefined);
  render(<AddSourceForm onSubmit={onSubmit} />);
  await userEvent.click(screen.getByText("添加信源"));
  await userEvent.type(screen.getByPlaceholderText(/粘贴链接/), "https://x.test/a");
  await userEvent.type(screen.getByPlaceholderText(/一句话/), "变绿是真的，但不等于可持续。");
  await userEvent.click(screen.getByLabelText("一手数据"));
  await userEvent.click(screen.getByRole("button", { name: "加入信源档案" }));
  expect(onSubmit).toHaveBeenCalledWith({
    url: "https://x.test/a", takeaway: "变绿是真的，但不等于可持续。", tier: "一手数据",
  });
});

it("requires the takeaway — the student must say what they took away", async () => {
  const onSubmit = vi.fn();
  render(<AddSourceForm onSubmit={onSubmit} />);
  await userEvent.click(screen.getByText("添加信源"));
  await userEvent.type(screen.getByPlaceholderText(/粘贴链接/), "https://x.test/a");
  expect(screen.getByRole("button", { name: "加入信源档案" })).toBeDisabled();
  expect(onSubmit).not.toHaveBeenCalled();
});

it("shows the server's honest failure, not a blank source", () => {
  render(<AddSourceForm onSubmit={vi.fn()} error="取不到这个链接的正文，可以直接把正文粘进来。" />);
  expect(screen.getByRole("alert")).toHaveTextContent("取不到这个链接的正文");
});

// SourceLog.test.tsx
it("renders the ledger", () => {
  render(<SourceLog sources={[loggedSource]} />);
  expect(screen.getByText("检索日志 · 已记录 1 条")).toBeInTheDocument();
  expect(screen.getByText(loggedSource.takeaway)).toBeInTheDocument();
});

it("omits sources with no log entry rather than inventing one", () => {
  render(<SourceLog sources={[{ ...loggedSource, takeaway: "", tier: "" }]} />);
  expect(screen.getByText("检索日志 · 已记录 0 条")).toBeInTheDocument();
});
```

- [ ] **Step 2: Run to verify it fails**

Run: `npm test -w @mind-imprint/web -- AddSourceForm SourceLog`
Expected: FAIL — modules do not exist.

- [ ] **Step 3: Implement**

`AddSourceForm.tsx` — collapsed by default to a `添加信源` text button (inline `+` SVG, `#2A3B7A`, matching the design's 添加一条视角 affordance at `:948-952`). Expanded: a two-tab switch (`链接` / `粘贴正文`), the matching field(s), a required `一句话摘要` textarea (placeholder `一句话说说你从这条里读到了什么……`), a `层级` single-choice row (`一手数据` · `一手论文` · `机构报告` · `二手 · 需追源` · `评论 / 观点`) rendered as radio inputs with visible labels, and the submit button `加入信源档案` (disabled until the required fields are filled). While submitting, disable the button and show `正在取正文…`. On `error`, render it in a `role="alert"` banner (`#FBEEE7` / `#C96F4F`, the design's warn tone).

`SourceLog.tsx` — header `检索日志 · 已记录 {n} 条` where `n` counts sources that actually carry a log entry (`takeaway !== "" || tier !== ""`). One row each: title (linked to `sourceUrl` when present, `target="_blank" rel="noopener noreferrer"`), the tier chip, and the takeaway. Copy under the header: `每一条你打开过的来源都在这里——引用只能从这里来。` Mount it below the dossier list (only in list mode, not in the read view).

Mount both in `SourceDossier`: the form under the header, the log after the source list.

- [ ] **Step 4: Run tests**

Run: `npm test -w @mind-imprint/web`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add apps/web
git commit -m "feat(refactor2): 添加信源 form + 检索日志 ledger in the material view"
```

---

### Task 9: Thread the anchors + wire the container

**Files:**
- Modify: `apps/web/src/studio/StudioShell.tsx`, `apps/web/src/studio/ViewFrame.tsx`, `apps/web/src/studio/StudioContainer.tsx`, `apps/web/src/studio/state.ts`
- Test: `apps/web/src/studio/ViewFrame.test.tsx`, `apps/web/src/studio/StudioContainer.test.tsx`

**Interfaces:**
- Consumes: Task 6's `views.material` + api client; Task 7's `onOpenLogged`; Task 8's `AddSourceForm.onSubmit`; the existing live `card: LiveCard | null` already flowing into `StudioShell`.
- Produces: nothing downstream — this is the last wiring task.

The merge rule (spec §6): **the live card's anchors win** for the material they target (they carry the student's in-progress answers); the projection's persisted anchors render for every other material. `SourceDossier`'s `anchorToSpan` already filters by `material_id` — dedupe by anchor `id`, live-first.

- [ ] **Step 1: Write the failing test**

```tsx
// ViewFrame.test.tsx
it("highlights the live card's anchors only in the material they target", () => {
  const state = studioStateWith({          // existing helper style in this file
    activeStation: "S3",
    views: { ...emptyViews, material: [blogSource, nasaSource] },
  });
  const card = {
    cardInstanceId: "ci1", cardId: "craap", spec: craapSpec, status: "active" as const,
    anchors: [{
      id: "a1", material_id: blogSource.id, block_id: "b1", start: 0, end: 5,
      quote: "过去二十年", dimension: "authority", author: "ai",
      question: "原始出处是谁？", answer: "",
    }],
  };
  render(<ViewFrame state={state} card={card} />);
  fireEvent.click(screen.getByText(blogSource.title));
  expect(screen.getByText("过去二十年")).toHaveAttribute("data-span-id", "a1");
});

it("does not leak one material's anchors into another", () => {
  // …open nasaSource with the same card mounted → no highlighted span.
});
```
(Match `Annotate`'s real span markup — read `primitives/annotate` for the attribute it actually renders, and assert on that.)

```tsx
// StudioContainer.test.tsx
it("posts a new source and shows it in the dossier", async () => { /* addMaterial spy → list grows */ });
it("posts the reading time when the student leaves a source", async () => { /* logSourceOpen spy */ });
```

- [ ] **Step 2: Run to verify it fails**

Run: `npm test -w @mind-imprint/web -- ViewFrame StudioContainer`
Expected: FAIL — `ViewFrame` takes no `card` prop.

- [ ] **Step 3: Implement**

- `ViewFrameProps` gains `card?: LiveCard | null` and `material?: { onAdd, onOpenLogged, addError }`. `StudioShell` passes its existing `card` straight through (it already has it) plus the material callbacks it receives.
- `ViewFrame` computes the merged anchor list once: `[...(card?.anchors ?? []), ...state.views.material.flatMap(m => m.anchors)]` deduped by `id`, live entries first, and passes it to `SourceDossier`'s existing `anchors` prop. `SourceDossier` already filters per open source by `material_id` — **do not add a second filter.**
- `StudioContainer`: add `onAddSource` (calls `api.addMaterial`, then refetches the project and re-derives `state`; on `APIError` puts the server's `message` into an `addError` state) and `onOpenLogged` (fire-and-forget `api.logSourceOpen`, `.catch(() => {})` — a lost timing sample must never surface as an error to the student). Thread both through `StudioCallbacks` in `state.ts` → `StudioShell` → `ViewFrame`.

- [ ] **Step 4: Run tests + full gate**

```bash
npm test -w @mind-imprint/web && npm test -w @mind-imprint/contracts && npx tsc --noEmit -p apps/web
cd apps/api && CGO_ENABLED=0 go test -p 1 ./...
```
Expected: all PASS.

- [ ] **Step 5: Commit**

```bash
git add apps/web
git commit -m "feat(refactor2): thread live card anchors into the source dossier; wire ingestion + open logging"
```

---

### Task 10: Sweep, gate, roadmap

**Files:**
- Modify: `docs/2026-07-11-whole-product-refactor-roadmap.md`
- Modify: `AGENTS.md` (only if a documented invariant changed — e.g. the `material.task_id` NOT NULL note)

- [ ] **Step 1: Orphan sweep**

```bash
grep -rn "SourceFixture\|SOURCE_FIXTURES" apps/web/src   # must be empty
grep -rln "views.material: \[\]" apps/web/src            # must be empty
cd apps/api && make sqlc && git status --short internal/store/sqlc   # must be clean (no stale generated files)
```
Any hit is an orphan — remove it.

- [ ] **Step 2: Full gate**

```bash
cd apps/api && CGO_ENABLED=0 go build ./... && CGO_ENABLED=0 go vet ./... && CGO_ENABLED=0 go test -p 1 ./...
cd ../.. && npm test -w @mind-imprint/web && npm test -w @mind-imprint/contracts && npx tsc --noEmit -p apps/web
```
Expected: every package `ok`, all suites green, typecheck clean. Record the counts in your report.

- [ ] **Step 3: Roadmap entry**

Append a `- **Slice 6b** — ☑ **complete**` entry to the per-slice log: spec/plan links, the branch, the commit range, what each task delivered, the accepted gaps (spec §11 carry-forwards), and the final gate numbers. Update the Slice 6 row's status marker in the table (6b done → only 6c remains).

- [ ] **Step 4: Commit**

```bash
git add docs AGENTS.md
git commit -m "docs(refactor2): Slice 6b complete — material view, ingestion, source log"
```
