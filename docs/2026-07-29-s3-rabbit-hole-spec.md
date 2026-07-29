# S3 · Rabbit-hole exploration surface — spec

> Slice S3 of the multi-agent architecture (`docs/2026-07-29-multi-agent-architecture-design.md` §11).
> Depends on S2 (reading takeaways, incl. `new_leads`). Design authority:
> `docs/2026-07-29-card-placement-map.md` §5 (the 兔子洞 arrangement) + the architecture doc
> §9.4 ("reading-list … branch links" as a spine field addition).

## 1. What S3 is

The 文献库 (research library, P3) today is a flat list of sources. S3 turns it into a
**living exploration graph** — the floating 印记 as the 兔子洞 exploration tree:

- **Sources branch into leads.** When a source's reading takeaway surfaces `new_leads`
  (S2 already stores these), each lead becomes a **first-class open branch** hanging off that
  source — a direction still to pursue.
- **Read-but-unused = a dangling branch.** A source that's been engaged but isn't feeding the
  argument (decision `drop`/未决 and no lead connects it) is surfaced so the student **connects
  or prunes** it.
- **"Dig into this branch" = the guide points.** A metered guide call reads the whole
  source+lead graph and the proposal, and **proposes the next necessary direction** — it never
  fetches a source and never decides for the student (铁律: AI 克制; points, doesn't conclude).
- The student **connects** a lead to the source that answers it, **prunes** a lead she's
  abandoning, or **adds** her own. The graph is the durable record of how her research branched.

**Boundary with the Reading Room (P4):** the exploration surface is about the *forest* — which
sources, and how they connect. Opening one source still crosses into the Reading Room, where the
per-sentence cards + lenses live. S3 does not touch the reading loop.

## 2. Design decisions (resolved by committed design + endorsed principles)

Principles (from the architecture doc, endorsed across S1/S2): student-experience first ·
performance > cost · educational · non-manipulative · 四条铁律.

- **D-S3-1 · Leads are first-class rows, not a projection.** The architecture doc names
  "branch links" as a durable spine field, and the arrangement requires a per-lead lifecycle
  (open → connected / pruned) with provenance. A projection can't carry status or the
  connect/prune the student performs. → new `exploration_lead` table (migration 0040).
- **D-S3-2 · Leads materialize from takeaways, idempotently.** On `postFinalizeReading`, each
  `new_leads` entry becomes an `open` lead attributed to that source. Re-finalize (S2 supersedes)
  must **not** duplicate leads and must **not** resurrect a lead the student already pruned.
  Dedupe key = (project_id, source_reference_id, text).
- **D-S3-3 · The guide is a plain metered handler**, mirroring S2's `getTakeawayDraft` +
  `ComposeReadingTakeawaySuggestions`. The `internal/skills` runtime is the C5 project/course
  **type catalog**, not an LLM-skill registry — the reading coach/takeaway compose are plain
  `agent` functions + handlers, and the guide follows that same shape. Mid-tier `ChatResolver`,
  purpose `exploration_guide`, gated on `HasEntitlement`, and **gated on graph content** (≥1
  source or ≥1 open lead) so an empty library spends nothing.
- **D-S3-4 · Surface = a view mode inside 文献库, not a new route.** The design calls it the
  *floating 印记* within the library. The web library (`ReadingBlock.tsx`) gains a
  列表 ⇄ 探索图谱 toggle; the exploration view is the same surface, same data.
- **D-S3-5 · Layout is a structured branch view, not a physics graph.** No graph library exists
  in the web app and we won't add one. Sources render as nodes; their leads hang as child
  chips; dangling sources and open leads get their own trays. Flexbox + light connector styling —
  legible, buildable, no D3.
- **D-S3-6 · P3 cards on the surface = the existing `rabbit-hole` card, reachable as a
  standalone reflection tool; no new card JSON, no summon-loop-on-surface.** The `rabbit-hole`
  card (id `rabbit-hole`, 兔子洞·兴趣雷达) already exists in `CARD_REGISTRY`. S3 exposes an entry
  point to open it standalone **iff** a non-anchored card-fill path already exists in the runtime;
  otherwise this is deferred to S4 and the tree/leads/guide ship without it. AI *proposing* cards
  cross-phase is explicitly S4 ("cross-phase card proposing"). This keeps S3 tight.

## 3. Data model — migration `0040_exploration_lead.sql`

Additive; no changes to existing tables.

```sql
CREATE TABLE exploration_lead (
    id                      uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id              uuid NOT NULL REFERENCES project(id) ON DELETE CASCADE,
    text                    text NOT NULL,
    status                  text NOT NULL DEFAULT 'open'
                                CHECK (status IN ('open','connected','pruned')),
    origin                  text NOT NULL DEFAULT 'takeaway'
                                CHECK (origin IN ('takeaway','manual','guide')),
    source_reference_id     uuid REFERENCES reference(id) ON DELETE SET NULL, -- who surfaced it
    connected_reference_id  uuid REFERENCES reference(id) ON DELETE SET NULL, -- who answers it
    position                int NOT NULL DEFAULT 0,
    created_at              timestamptz NOT NULL DEFAULT now(),
    updated_at              timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX exploration_lead_project_idx ON exploration_lead (project_id);
```

- `status`/`origin` CHECKs are DB-cheap invariants; the canonical enums also live in contracts.
- `source_reference_id` nullable: a `manual`/`guide` lead has no originating source; also
  `ON DELETE SET NULL` so deleting a source doesn't cascade-destroy the branch record.

### Queries (`internal/store/queries/workspace.sql`, sqlc)

- `ListExplorationLeads :many` — by project, ORDER position, created_at.
- `CreateExplorationLead :one` — (project_id, text, status, origin, source_reference_id,
  connected_reference_id, position).
- `GetExplorationLeadForProject :one` — (id, project_id) IDOR guard, mirrors
  `GetReferenceForProject`.
- `UpdateExplorationLead :one` — sets text, status, connected_reference_id, position,
  updated_at = now() (handler merges partial, mirroring `UpdateReference`).
- `DeleteExplorationLead :exec` — by (id, project_id).
- `CountExplorationLeadForSource :one` — COUNT(*) WHERE project_id AND source_reference_id AND
  text = $ (the idempotent-materialize dedupe check).

## 4. Backend — Go

### 4a. Lead lifecycle (no spend) — `internal/api/exploration.go`

Routes registered in `api.go` alongside the library routes, all `protected`:

```
GET    /api/v1/projects/{id}/exploration              a.getExploration       (projection)
POST   /api/v1/projects/{id}/exploration/leads        a.createExplorationLead (manual add)
PATCH  /api/v1/projects/{id}/exploration/leads/{lid}  a.patchExplorationLead  (connect/prune/reopen/edit)
DELETE /api/v1/projects/{id}/exploration/leads/{lid}  a.deleteExplorationLead
POST   /api/v1/projects/{id}/exploration/guide        a.postExplorationGuide  (SPENDS)
```

- `getExploration` → `{ leads: ExplorationLeadDTO[], danglingSourceIds: string[] }`.
  - `danglingSourceIds` = references with `material_id` set (engaged) AND (`decision` null or
    `drop`) AND not the `connected_reference_id` of any non-pruned lead. Computed server-side
    from `ListReferences` + `ListExplorationLeads`; no spend.
- `createExplorationLead` — body `{ text }`; origin `manual`, status `open`, appended.
- `patchExplorationLead` — partial-merge body `{ text?, status?, connectedReferenceId? }`.
  - "connect" = client sends `status:"connected"` + `connectedReferenceId`.
  - "prune" = `status:"pruned"`. "reopen" = `status:"open"` (+ null the connection).
  - Validate `status` against the enum; validate `connectedReferenceId` (when non-null) is a
    reference in this project (IDOR).
- `deleteExplorationLead` — hard delete (for manual/mistaken leads). IDOR-guarded.
- DTO `explorationLeadDTO` (camelCase, mirroring `referenceDTO` conventions): `id, text, status,
  origin, sourceReferenceId (string|null), connectedReferenceId (string|null), position`.

### 4b. Materialize-on-finalize — extend `postFinalizeReading`

After `FinalizeReadingTakeaway`, for each `new_leads` text: if
`CountExplorationLeadForSource(project, thisRef, text) == 0`, insert an `open`/`takeaway` lead
with `source_reference_id = thisRef`, `position = append`. Idempotent (re-finalize with same
leads inserts nothing) and non-resurrecting (a pruned lead still counts as present → no
re-insert). Best-effort: a lead-insert failure must not fail the finalize (log-and-continue,
same posture as the existing `appendAutoLog`).

### 4c. The guide (SPENDS) — `internal/agent/exploration_guide.go` + handler

`agent.ComposeExplorationGuide(ctx, prov gateway.Provider, r gateway.Resolved, in
ExplorationGuideInput) (directions []GuideDirection, usage gateway.ChatUsage, error)` — the
isolated compose, cloned from `ComposeReadingTakeawaySuggestions`:

```go
type ExplorationGraphSource struct {
    Title       string   // truncated
    Decision    string   // use|maybe|drop|"" (未决)
    Credibility string   // strong|mixed|weak|""
    PhaseTag    string   // "" if unset
    State       string   // 未读|在读|已归纳
}
type ExplorationGuideInput struct {
    ProposalObjective string                   // from spine; may be ""
    Sources           []ExplorationGraphSource
    OpenLeads         []string                 // open lead texts
    PrunedCount       int
    ConnectedCount    int
}
type GuideDirection struct {
    Direction string `json:"direction"` // the next necessary direction, as a prompt
    Why       string `json:"why"`       // one line: the gap it fills
}
```

- System prompt (`explorationGuideSystem`): 克制 — "你在帮学生看清研究的森林，而不是替他做研究。指出
  下一个**必要**的方向或没接上的缺口，用一句话说明它填补什么。绝不替学生检索、绝不替他下结论、绝不替他
  判定某个来源的价值。最多给 3 条，每条是一个可追问的方向，不是一个答案。" (Never fetch, never decide,
  ≤3 directions, each a question not an answer.)
- Errors early if the graph is empty (`len(Sources)==0 && len(OpenLeads)==0`) — mirrors
  `HasRecordContent` gating. Parse `{directions:[{direction,why}]}` via the real `stripFences`.

Handler `postExplorationGuide` — clone of `getTakeawayDraft`'s discipline:
gate `HasEntitlement` → build `ExplorationGuideInput` (sources from `ListReferences` + their
read-state via `readingOutcomesFromCards`/material presence, leads from `ListExplorationLeads`,
`ProposalObjective` from `GetProjectProposal`) → **skip network + meter if graph empty** →
resolve `ChatResolver` → `ComposeExplorationGuide` → **meter only when `resolved.Provider != ""`**
(`RecordLLMCall`, surface `studio`, purpose `exploration_guide`) → 200 with
`{ directions: GuideDirection[] }`; on compose failure return 200 + `directions: []` (never 500).
The guide **does not persist** anything — the student decides whether to add a direction as a lead
(via `createExplorationLead`).

### 4d. Spine projection — extend `buildSpineProjection`'s 文献库 block

Append one exploration line after the per-source lines, following the state-aware, `truncateRunes`,
best-effort conventions already there:

```
探索：待追 N 条线索 · M 个悬空来源
```

where N = open-lead count, M = dangling-source count (same computation as `getExploration`, reusing
the already-fetched refs/cards where possible to avoid an extra scan). Omit the line when both are
zero. This is how the continuous coach stays aware of the branch state (D2 always-on projection).

## 5. Contracts — `packages/contracts/src/exploration.ts`

```ts
export const LeadStatus = z.enum(["open","connected","pruned"]);
export const LeadOrigin = z.enum(["takeaway","manual","guide"]);
export const ExplorationLead = z.object({
  id: z.string(), text: z.string(),
  status: LeadStatus, origin: LeadOrigin,
  sourceReferenceId: z.string().nullable(),
  connectedReferenceId: z.string().nullable(),
  position: z.number(),
});
export const ExplorationView = z.object({
  leads: z.array(ExplorationLead),
  danglingSourceIds: z.array(z.string()),
});
export const GuideDirection = z.object({ direction: z.string(), why: z.string() });
export const ExplorationGuide = z.object({ directions: z.array(GuideDirection) });
```

Barrel: add `export * from "./exploration"` to `src/index.ts`. The Go `explorationLeadDTO` and
`GuideDirection` JSON tags match these camelCase shapes exactly (the S2 persist+wire pattern).

## 6. Frontend — React

### 6a. Client — `apps/web/src/api/exploration.ts`

Mirror `api/reading.ts` (uses `apiFetch`, Zod-parses):

```ts
getExploration(projectId): Promise<ExplorationView>
createLead(projectId, text): Promise<ExplorationLead>
patchLead(projectId, lid, patch: {text?; status?; connectedReferenceId?|null}): Promise<ExplorationLead>
deleteLead(projectId, lid): Promise<void>
digDeeper(projectId): Promise<ExplorationGuide>   // POST /exploration/guide
```

### 6b. Exploration view — `apps/web/src/workspace/blocks/exploration/ExplorationView.tsx`

Rendered inside `ReadingBlock` behind a 列表 ⇄ 探索图谱 view-mode toggle (the library block owns
the toggle + fetches `getExploration`). Structured branch layout (D-S3-5):

- **Source column** — each read source as a node card (title + phaseTag chip + state badge, reusing
  ReadingBlock's existing chip/badge styling), with its `takeaway`-spawned leads hanging beneath as
  child chips. Each lead chip: text + [连接来源] / [剪枝] (open), or a connected/pruned marker.
- **悬空来源 tray** — the `danglingSourceIds` sources, with a nudge to 连接 or 剪枝.
- **待追线索 tray** — open leads without a parent source (manual/guide origin), each add-able as a
  child, prunable; a "+ 手动添加线索" input.
- **深挖一层 (dig deeper) panel** — the floating 印记 guide: a button → `digDeeper()` → renders the
  proposed `directions` as cards, each with its `why` and a [记为线索] button that calls
  `createLead`. Non-manipulative: nothing auto-added; the student chooses.
- **Connect flow** — [连接来源] opens a small picker of this project's references; choosing one
  calls `patchLead(status:"connected", connectedReferenceId)`.

Copy is real (Phoebe / 中国可持续 domain in mockups per house rule), never lorem ipsum.

### 6c. `rabbit-hole` card entry (D-S3-6, conditional)

If a standalone (non-anchored) card-fill path exists in the runtime, add a 「兔子洞·兴趣雷达」entry
in the exploration view that opens the existing `rabbit-hole` card and logs its envelope. If no such
path exists without building a summon loop, omit it in S3 and note the deferral to S4. The plan's
first frontend task confirms which.

## 7. Non-goals (explicit)

- ❌ AI *proposing* cards/lenses cross-phase in chat — that's S4.
- ❌ New card JSON (信息金字塔 T03 etc.) — later slice.
- ❌ A physics/force graph or any graph library.
- ❌ Auto-fetching or auto-deciding on any source or lead (铁律).
- ❌ Touching the Reading-Room read-together loop.

## 8. Acceptance (end-to-end)

Phoebe, 「中国是否让地球更可持续？」: she finalizes a NASA source in the Reading Room; its
`new_leads`（例如「查 Nature Sustainability 对中国可再生能源的评估」「找中国碳排放绝对量的一手数据」）
appear as **open branches** under that source in 探索图谱. A source she skimmed but marked `drop`
shows in **悬空来源**; she prunes it. She hits **深挖一层** → the guide points: 「你有支持论点的来源，
但还没有反例检验的一手数据——下一步是否找中国碳排放绝对量的官方统计？」she taps [记为线索]. The
continuous coach's spine now reads `探索：待追 2 条线索 · 0 个悬空来源`. No client model call, every
guide call metered with tier+tokens+purpose, nothing concluded for her.
