# C · Student-Level Ability Model — Design Spec

> Aggregates the per-session DualAxis reports (produced by A1/A2/A3/B) into a
> student-level 能力素养 model in 成长报告, and restores the binding tabbed layout.
> Deterministic projection, no LLM. Current-standing view, axis-asymmetric.

**Status:** approved (brainstorm 2026-07-19). Follows A1/A2/A3/B (all merged; B at `a84fefd`).
The per-session model is DualAxis (`agent.Report` / contracts `DualAxisReport`); C reads
those stored reports and merges them. No rubric change.

**Authoritative references:** the binding `docs/design/思维印记_工作区.dc.html` 成长报告
tabs (`学习记录`/`工具卡`/`能力素养`, lines ~1471–1603) and the DualAxis axiom from B's
spec. Where the binding radar (formerly 9 symmetric dims) conflicts with the DualAxis
structure, **DualAxis wins** (B's user decision) — C redesigns the radar accordingly.

---

## 0. Why

B settled the per-session assessment model (DualAxis) but left the student-level view
unbuilt: A3 rebuilt 成长报告 as a history-only hub, dropping the binding design's tabs.
C fills the 能力素养 tab — a cross-session merge of the DualAxis reports into a
current-standing ability picture — and restores the tabbed 成长报告 (`学习记录` history +
`能力素养`). This is the cross-session **aggregation** that A3 (which delivered the history
*list*) explicitly deferred.

**The central tension, resolved.** B's axiom: 「两轴永不合成总分；单次会话为事件级证据，
不构成人级档位判定」. C honors both clauses:
- **Never a cross-axis total** — the three report parts aggregate into three *separate*
  blocks; nothing sums across axes.
- **Not a person-level 档位 from one session** — C merges *many* sessions; a person-level
  depth level is shown only when backed by ≥2 sessions of evidence (a **low-N guard**),
  and always as diagnostic accumulation with its evidence count, never a rank/测验分数
  (the binding caption already states this: 「等级来自每次任务评估的归并，不是测验分数」).

---

## 1. The aggregation model (axis-asymmetric)

**Input:** all of the student's stored per-session `agent.Report`s (project + course +
chat), owner-filtered, ordered oldest→newest by `created_at`. The three report parts
aggregate **differently** — the asymmetry is the model.

### 1a. 认知深度 (D1/D3/D4/D5) → merged current level, per dim
- A session **contributes evidence** for a dim only when that dim's score is **≥1**.
  A `0` (证据不足 in that session) is treated as *absent* — absence of evidence, not
  evidence of absence — and excluded from the merge and the evidence count.
- **Current level** = recency-weighted mean of the contributing scores, rounded to a
  0–3 integer, displayed with that score's anchor label. **Recency weight:** for the
  contributing sessions ordered oldest→newest as `s_0 … s_{k-1}` (k = evidence count),
  the newest gets the largest weight via exponential decay: `w_i = DECAY^(k-1-i)` with
  **`DECAY = 0.6`**; level = `round( Σ w_i·score_i / Σ w_i )`. (Newest session weight 1;
  each older session 0.6×the next.)
- **Evidence count** = number of contributing sessions (score ≥1).
- **Low-N guard:** a dim with **< 2** contributing sessions has level `-1` (sentinel for
  「证据不足 · 需更多任务」) and no bar — the axiom made literal.
- These four dims — and only these — populate the **radar** (4 spokes, 0–3 scale).

### 1b. 智识自主 (D2, unscored) → observation summary, never a level
- Summed across all sessions: `boundarySettings` (Σ `promptLens.boundarySettings`),
  `adversaryInvites` (Σ `autonomyAxis.adversaryInvites`), `anchoredSignals`
  (Σ `len(autonomyAxis.anchoredSignals)`), `promptedSignals` (Σ
  `len(autonomyAxis.promptedSignals)`), plus `sessions` (count of sessions with an
  autonomy block). Rendered as an observation panel with an 「观察」 badge. No number
  becomes a level.

### 1c. 跨轴 元认知 (D6 / SOLO) → distribution panel, never a single level
- Over every session's `solo[]` rows (per-round SOLO judgments):
  `distribution` = count of rows at each of L1/L2/L3/L4; `highestSolo` = the highest
  level present (or `""` if none); `spontaneous` = count of rows with `initiative == "自发"`;
  `prompted` = count of the rest. Rendered as a distribution panel (highest reached +
  the L1–L4 counts + 自发/引导后 split). A distribution, not a verdict.

### 1d. Axiom / RL-5 compliance (structural)
- Three separate blocks (radar / autonomy / metacognition); **no field sums across axes**.
- Depth level requires ≥2 sessions (evidence-backed accumulation, never one session).
- Autonomy carries no level field at all; metacognition carries no single level.
- Every block shows its evidence/session count; the whole view reads as diagnostic
  accumulation, never a rank, grade, or 测验分数.

---

## 2. Data flow

### 2.1 Deterministic projection (pure, no LLM)
New package `apps/api/internal/ability`:

```go
type Sample struct { Report agent.Report; CreatedAt time.Time }

type DepthAbility struct { Code, Name string; Level int; LevelLabel string; EvidenceCount int } // Level -1 = insufficient
type AutonomyAbility struct { Sessions, BoundarySettings, AdversaryInvites, AnchoredSignals, PromptedSignals int }
type Metacognition struct { HighestSolo string; Distribution map[string]int; Spontaneous, Prompted int }
type Model struct {
    TotalSessions int
    Depth         []DepthAbility   // D1,D3,D4,D5 in model order, always length 4
    Autonomy      AutonomyAbility
    Metacognition Metacognition
}

func Aggregate(samples []Sample) Model  // pure; no store, no model call
```
`Aggregate` derives dim names/order from `rubric.DepthDims()`; the score→label map from
each depth dim's anchors. Empty `samples` → `Model{}` with zeroed blocks and length-4
depth all at level `-1`.

### 2.2 Query
`ListGrowthHistory` returns only *latest-per-scope*, so it cannot feed aggregation. Add a
sqlc query **`ListEvaluationsByUser`** returning **all** of the student's evaluation rows
(same 3-scope owner UNION as `ListGrowthHistory`, but no `DISTINCT ON`), columns
`scores`, `created_at`, ordered `created_at ASC`. `make sqlc` after editing
`queries/evaluation.sql`; never hand-edit `internal/store/sqlc/*`.

### 2.3 Endpoint + DTO
- `studio.AbilityDTO` (+ sub-DTOs) mirrors `ability.Model` byte-for-byte (camelCase);
  `studio.ToAbilityDTO(ability.Model) AbilityDTO`.
- `GET /api/v1/growth/ability`: read owner from session → `ListEvaluationsByUser` →
  unmarshal each `scores` into `agent.Report` (skip malformed rows with a log, don't 500)
  → `ability.Aggregate` → `ToAbilityDTO` → 200. Empty → an empty model (200, never 404).
  Read-only; **no `llm_call` row**, no cost (no model call). No entitlement gate needed
  (read-only, no tokens) — but mirror the existing growth-history handler's ownership.

### 2.4 Contract
`packages/contracts/src/ability.ts` — `AbilityModel` Zod mirroring `AbilityDTO`
(depth[]/autonomy/metacognition/totalSessions); barrel export. Web client
`apps/web/src/api/ability.ts` `getAbilityModel(): Promise<AbilityModel>` (GET, `.parse`),
wired into the api facade.

**No migration** — reads the existing `evaluations` table (all rows DualAxis post-0027).

---

## 3. UI — tabbed 成长报告 + 能力素养

### 3.1 Tabbed restructure
`apps/web/src/shell/growth/GrowthReport.tsx` gains a binding-style tab bar with **two**
tabs (工具卡 deferred): **学习记录** (the current history list, extracted verbatim into its
own component — behavior unchanged) and **能力素养** (new). Tab state is local; 学习记录 is
the default tab.

### 3.2 `<AbilityModel>` component
Fetches `api.getAbilityModel()` on mount. Renders top-to-bottom:
- Header + the binding caption verbatim: 「AI 批判性思维 · 能力素养模型」 / 「…等级来自每次
  任务评估的归并，不是测验分数」, plus a `totalSessions` context line.
- **4-spoke depth radar** (inline SVG, adapted from the binding radar SVG to 4 axes on a
  0–3 scale; a dim at level `-1` plots at the center/omitted) + a per-dim list: dim name,
  level label + a segment bar, and evidence count; a `-1` dim renders 「证据不足 · 需更多
  任务」 instead of a bar.
- **智识自主 observation panel** — an 「观察」 badge + the counts (边界设定 / 对手邀请 / 自发
  / 引导后 across `sessions` sessions). No level.
- **跨轴 元认知 panel** — highest SOLO, the L1–L4 distribution, the 自发/引导后 split.
- **Empty state** (totalSessions 0) — 「还没有足够的数据 · 完成更多任务后，你的能力画像会在
  这里浮现」.

Inline SVG only (never lucide); binding design tokens; the three blocks stay visually
separate (no combined score anywhere).

---

## 4. Testing + invariants

**Go — `ability` projection units:** recency-weighting yields the expected merged level
(e.g. scores [1,3] newest-last, DECAY 0.6 → weighted mean 3·1+1·0.6 / 1.6 = 2.25 →
round 2); score-0 sessions excluded from evidence + count; **<2 contributing → level -1**;
autonomy sums correct; SOLO highest + distribution + spontaneous/prompted split; empty
input → zeroed model with length-4 depth all -1.
**Go — query:** `ListEvaluationsByUser` returns **all** owner rows (not latest-per-scope),
in `created_at` order, excludes other users.
**Go — endpoint:** owner isolation (other user's data absent), empty-state 200 with empty
model, correct shape, and **zero `llm_call` rows** written (proves no model call).
**Contracts:** `AbilityModel` round-trip + Go↔TS parity.
**Web:** `<AbilityModel>` renders radar + all three panels + the `证据不足` state + the
empty state; caption verbatim; tab switching between 学习记录 and 能力素养.

**Invariants (→ plan Global Constraints, verbatim values):**
- **RL-5:** no combined total across axes; no rank/grade/测验分数; every block shows its
  evidence/session count.
- **Axiom:** depth level requires ≥2 contributing sessions (else `-1`); 智识自主 never a
  level; 元认知 never a single level; the three blocks never combine.
- **Deterministic:** no LLM call, no cost — asserted by zero `llm_call` rows on the
  endpoint.
- **Owner-isolation:** only the session user's evaluations aggregate.
- **Single-source reuse:** consumes `agent.Report` / contracts `DualAxisReport` +
  `rubric.DepthDims()`; no rubric or per-session-model change.
- `make sqlc` from `apps/api`; never hand-edit `sqlc/*`. Full Go packages
  (`CGO_ENABLED=0 go test -p 1 ./...`, `DOCKER_HOST=unix:///var/run/docker.sock`), never
  `-run` subsets, for query/endpoint/projection changes.
- Web/contracts tests from their own dirs. Inline SVG, never lucide. Direct-merge to
  `main` + push. Never `git add` a whole directory (pre-existing `M package.json` +
  untracked user files under `docs/`/repo root are not ours).

---

## 5. Out of scope (deferred)
- **工具卡** collected-cards tab (a distinct card-usage aggregation, not the ability model).
- Any **trajectory / time-series** view — this is current-standing only.
- Cross-student / class / teacher aggregation.
- Recomputing or caching the model (projected on each request; cheap, no LLM).
