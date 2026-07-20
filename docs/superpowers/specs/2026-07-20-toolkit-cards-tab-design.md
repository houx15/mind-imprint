# 工具卡 (Toolkit Cards) Tab — Design Spec

> The last deferred item of the cross-surface assessment track (A1→A2→A3→B→C).
> A third tab in 成长报告 that shows the thinking tool cards a student has
> collected across all three surfaces (project / course / chat).

**Date:** 2026-07-20
**Status:** approved, ready for plan
**Related:** `docs/superpowers/specs/2026-07-19-c-student-ability-model-design.md` (the tab it sits beside), the binding design `docs/design/思维印记_工作区.dc.html` lines 1534–1558.

---

## 1. Goal

A read-only, cost-free "your thinking toolkit" record: which structured-thinking
tool cards the student has actually worked through, grouped by category, with an
honest usage summary. It complements 学习记录 (per-session reports) and 能力素养
(the ability model). It is a keepsake, **not** an achievement board.

## 2. Product decisions (settled with the user, 2026-07-20)

1. **Grouping = the existing `category` field (9 values).** Each card carries a
   `category`; across the 34 cards there are nine distinct values:
   `知识工具`(8), `信息素养`(7), `AI伦理`(5), `AOK`(3), `溯源与多视角`(3),
   `反身性与元认知`(3), `探究启动`(3), `成长与沉淀`(1), `论证结构`(1). The dc.html
   mockup groups "按课程库五大分支" (five branches), but the single-source card JSON
   carries this finer `category` taxonomy, not a per-card branch. Rather than
   introduce a new `branch` field across all 34 card JSONs, we group by the existing
   `category` and soften the intro copy from "五大分支" to "按类别组织". Fully honest
   to the data we have; zero card-data change. Only categories with ≥1 collected
   card render (engaged-only, §2.3).
2. **Collected = completed only.** A card is "collected" once the student worked
   through it at least once (`card_instances.status = 'completed'`). This matches
   `collectedCourseSessionCards` (course_session.go), which already filters to
   `completed` and treats a skip as a decline. Skipped / proposed / active cards do
   NOT appear in this tab. (过程即数据 still holds elsewhere: skips remain evidence
   for the assessor via `dispositionUsesFromEvidence`; the toolkit tab is a
   different artifact — a record of tools *practiced*, not tools *encountered*.)
3. **Engaged cards only (no locked catalog).** Show only categories/cards the
   student has actually completed. No "0/34 unlock more", no progress bar, no
   badges-to-earn. 铁律 2 (不操纵) forbids slot-machine / achievement mechanics;
   an honest keepsake of what was practiced is the safe design.

## 3. Architecture & data flow

Mirrors the existing `/growth/ability` endpoint exactly: an owner-scoped,
read-only projection that makes **no model call and records no cost**.

```
GET /api/v1/growth/cards
  → getGrowthCards handler (owner from context)
  → ListCollectedCardsByUser (UNION ALL of 3 scopes, owner-filtered, GROUP BY card_id)
  → []collectedCardDTO
  → WriteJSON

web <ToolkitCards> (self-fetching)
  → getGrowthCards() → CollectedCard[]
  → join each cardId to CARD_REGISTRY (frontend, drop unknown ids like CourseReport)
  → group by category, render per dc.html layout
```

**Where the dedupe/count happens — decided: in SQL.** The aggregation is trivial
(group by card_id, count, collect distinct surfaces, max date), so a single
`GROUP BY` query is the right tool — less code than C's pure-Go `Aggregate`
package (which was justified there by recency-weight math + a no-cost assertion,
neither of which applies here). A testcontainers query test covers it.

**Registry join stays on the frontend.** `CARD_REGISTRY` (name / purpose /
category) lives in `@mind-imprint/contracts`; the backend returns only
instance-derived facts (cardId, uses, surfaces, lastUsed). This matches how
`CourseReport` resolves collected cards today and keeps the backend free of card
metadata. Unknown `cardId`s (absent from the registry) are dropped client-side.

## 4. Backend

### 4.1 Query — `ListCollectedCardsByUser`

New query in `apps/api/internal/store/queries/card_instance.sql`. `card_instances`
has no `user_id`; ownership is reached through each scope's parent table
(`project.user_id` / `course_session.user_id` / `chat_thread.user_id`) — the same
owner-isolation join `ListEvaluationsByUser` uses. Scope columns are
`project_id` / `session_id` / `thread_id`. Status literal is `'completed'`.

```sql
-- name: ListCollectedCardsByUser :many
-- Every tool card the caller has COMPLETED at least once, across all three
-- scopes, deduped to one row per card_id with a usage summary. status='completed'
-- only (a skip is a decline, matching collectedCourseSessionCards). Owner-filtered
-- through each scope's own parent join (card_instances has no user_id). No cost,
-- no model — a pure read for the 工具卡 tab.
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

Regenerate with `make sqlc` (from `apps/api`, `CGO_ENABLED=0`). Never hand-edit
`internal/store/sqlc/*`. sqlc infers row types: `card_id string`, `uses int32`,
`surfaces []string`, `last_used` (interface{}/time — the handler formats it).

### 4.2 Handler — `getGrowthCards`

New file `apps/api/internal/api/cards_growth.go`, matching the `getAbilityModel`
shape (ability.go):

```go
type collectedCardDTO struct {
    CardID   string   `json:"cardId"`
    Uses     int      `json:"uses"`
    Surfaces []string `json:"surfaces"` // subset of {"project","course","chat"}
    LastUsed string   `json:"lastUsed"` // RFC3339
}

func (a *API) getGrowthCards(w http.ResponseWriter, r *http.Request) {
    u, _ := UserFromContext(r.Context())
    rows, err := a.d.Queries.ListCollectedCardsByUser(r.Context(), u.ID)
    if err != nil { httpx.WriteError(w, r, err); return }
    cards := make([]collectedCardDTO, 0, len(rows))
    for _, row := range rows {
        cards = append(cards, collectedCardDTO{
            CardID:   row.CardID,
            Uses:     int(row.Uses),
            Surfaces: row.Surfaces,
            LastUsed: <row.LastUsed formatted RFC3339>,
        })
    }
    httpx.WriteJSON(w, http.StatusOK, map[string]any{"cards": cards})
}
```

The exact `LastUsed` conversion depends on sqlc's inferred type for
`max(created_at)` (likely `pgtype.Timestamptz` or `interface{}`); the plan pins it
after inspecting the generated struct. Route in `api.go` beside the others:
`GET /api/v1/growth/cards → protected(a.getGrowthCards)`.

## 5. Contract + web client

### 5.1 `packages/contracts/src/collectedCards.ts`

```ts
export const CollectedCard = z.object({
  cardId: z.string(),
  uses: z.number().int(),
  surfaces: z.array(z.string()),
  lastUsed: z.string(),
});
export type CollectedCard = z.infer<typeof CollectedCard>;

export const CollectedCards = z.object({ cards: z.array(CollectedCard) });
export type CollectedCards = z.infer<typeof CollectedCards>;
```

Exported from the contracts barrel. Field parity 1:1 with `collectedCardDTO`.

### 5.2 `apps/web/src/api/cards.ts`

```ts
export async function getGrowthCards(): Promise<CollectedCard[]> {
  const raw = await apiFetch<unknown>(`/api/v1/growth/cards`);
  return CollectedCards.parse(raw).cards;
}
```

Wired into the `ApiClient` facade interface + the concrete client + any mock,
additively (the same 3 points `getAbilityModel` touches).

## 6. Component & UI

Binding layout: dc.html lines 1534–1558. `<ToolkitCards>` in
`apps/web/src/shell/growth/ToolkitCards.tsx`, self-fetching (same pattern as
`<AbilityModel>`).

- **Registry join + drop:** for each `CollectedCard`, look up
  `CARD_REGISTRY[cardId]`; skip ids not present (defensive, matches CourseReport).
  Read `name`, `purpose`, `category` from the spec.
- **Grouping:** bucket by `category`. Fixed group order (roughly the course-library
  arc), each with a dot color; only groups with ≥1 collected card render:
  1. `探究启动` `#D98263`  2. `信息素养` `#2A3B7A`  3. `溯源与多视角` `#4C9A82`
  4. `知识工具` `#7C6BB0`  5. `论证结构` `#C77CA6`  6. `AOK` `#D9A23D`
  7. `AI伦理` `#3E8EA8`  8. `反身性与元认知` `#5C6CB0`  9. `成长与沉淀` `#8A9A5B`.
  Any card whose `category` is not in this list falls into a final `其他` `#9AA1B0`
  group (defensive — a future category is surfaced, never silently dropped). Each
  group header: a colored dot + category name + `countLabel` = "N 张" (distinct
  cards in the group).
- **Tile (2-column grid):** `name` (bold) + a compact `×N` badge top-right (N =
  `uses`) + `purpose` (grey, clamped ~2 lines) + a usage line:
  surface labels joined by " · " then " · 最近 MM-DD" from `lastUsed`.
  Surface labels: `project→项目`, `course→课程`, `chat→聊天` (reuse the existing
  `SURFACE_LABEL` map). Within a group, order by `uses` desc then `lastUsed` desc
  (the query already returns this order; preserve it).
- **Intro copy:** "你收集到的思维工具卡，按类别组织。用得越多，越成为你的本能。"
- **Empty state:** "还没有收集到工具卡" + "在任务、课程或聊天里用过一张思维工具卡，它就会出现在这里。"
- **Loading state:** "正在整理你的工具卡…" (matches the sibling tabs' tone).

### 6.1 `GrowthReport.tsx`

Extend the tab state from `"learning" | "ability"` to
`"learning" | "cards" | "ability"`. Add the 工具卡 button **between** 学习记录 and
能力素养 (dc.html tab order). Render `<ToolkitCards>` for `"cards"`. Default tab
stays `"learning"`.

## 7. Testing

- **Query test** (`apps/api/internal/store`, testcontainers): seed for the owner
  a project with two `completed` instances of card A (dedupe → uses 2) + one
  `completed` of card B in a course session; a `skipped` instance of card C
  (excluded); and a different user's `completed` instance (excluded). Assert: two
  rows; card A uses=2 surfaces=[project]; card B uses=1 surfaces=[course]; card C
  absent; other user's absent. Also assert `last_used` is the later of A's two.
- **Endpoint test** (`apps/api/internal/api`, testcontainers): fresh student →
  `{"cards":[]}` 200; assert `SELECT count(*) FROM llm_call == 0` (cost-free).
- **Contract test** (`packages/contracts`): `CollectedCards.parse` round-trips a
  representative payload; rejects a missing field.
- **Component test** (`apps/web`): given a `CollectedCard[]` with two categories +
  one unknown `cardId`, asserts the unknown is dropped, groups render in fixed
  order with correct "N 张" counts, a tile shows its `×N` badge and 最近 date, and
  the empty list renders the empty state.

## 8. Invariants (must hold)

- **RL-5:** no score, level, rank, or total anywhere in this tab — usage counts
  are descriptive, never a grade.
- **铁律 2 (不操纵):** no locked catalog, no progress-to-100%, no earnable badges,
  no streaks. Only cards actually practiced appear.
- **Cost-free:** the endpoint makes no model call and writes zero `llm_call` rows.
- **Owner isolation:** every card row is reached through its scope's owner join;
  a student never sees another student's cards.
- **Single source of truth:** card name/purpose/category come from `CARD_REGISTRY`
  (contracts), not re-encoded server-side. Unknown ids are dropped, never faked.
- **Client never calls a model directly** (all data via the API; no keys client-side).

## 9. Out of scope (YAGNI)

- No drill-into-instance view (the 过程树 already shows per-session card content).
- No 自发/提示后 (spontaneous vs prompted) signal — it is a studio-only derivation,
  not honestly available cross-surface.
- No new `branch` field on card JSONs (deferred; group by existing category).
- No skipped-card display (completed-only, per §2.2).
