# N1 · Close the Loop — Design Spec

> The first finishing slice (N1) of the student-platform remainder
> (`docs/2026-07-20-student-platform-remaining-work.md`). Builds the
> **project-creation funnel that does not exist today**: a student with zero
> projects can paste an assignment, create a project, land in a live S0 任务解码
> station, do their onboarding work, and reach the existing write→finish loop.

**Date:** 2026-07-20
**Status:** approved (design shape), ready for spec review → plan
**Binding design:** `docs/design/思维印记_工作区.dc.html` — directory `:669-708` (新建论文), S0 任务解码 `:836-869`.

---

## 1. Goal & the gap it closes

Today there is **no way to create a project**. Confirmed in code:
- No `POST /api/v1/projects` route or handler exists (`apps/api/internal/api/api.go` registers only `GET`). `CreateProject` (sqlc) is called only from tests/migrations.
- `agent.Intake` (`planner.go:56-71`) is never called from any handler, and it writes the wrong shape (`claim`/`evidence` nodes with `{text}`), while the reader `projectOnboarding` (`projection.go:281-298`) wants `rubric_translation {restate_prompt, rows}` + `milestone_plan {steps}` — only seed migration 0018 produces that.
- The web empty state (`StudioContainer.tsx:277-286`) is a static "创建入口马上就来" placeholder with no action.

N1 builds the funnel end-to-end. It **reuses** the reader and the OnboardingView that already exist; it does **not** touch the downstream write/finish loop (that already works).

## 2. Settled decisions (with the user, 2026-07-20)

1. **Board-static fixture, no LLM at creation.** The S0 content (restate prompt, rubric rows, plan steps) comes from an authored **0457 onboarding fixture**, not a model call. The student's *work* — writing their own restate + picking the 2 weakest rows — is what's captured (过程即数据). The fixture is the exact single-source seam **N4 (multi-board)** later extends.
2. **Focused funnel only.** N1 = create→intake→land + directory + onboarding submit. Every loose write-loop correctness seam (live gate counts, event-model normalization, `GetCardInstance` scoping, anchor labels, preview-snapshot drift, terminal idempotency, `finishProject` lock, report re-POST-on-422, Enter-to-send, dc.html copy) is **deferred to N6**.
3. **No migration.** Migration 0017 made `graph_node.type` an open type (`CHECK type <> ''`); `author` allows `student`/`ai`/`imported`. All four node types N1 uses ride existing tables. `CreateProject`, `InsertGraphNode`, `AppendEvent` all already exist. **N1 ships zero migrations.**

## 3. Architecture & data flow

```
新建论文 (directory) → paste {title, prompt}
  → POST /api/v1/projects   (atomic tx: project row + 3 graph nodes)
        · project(user_id, qualification:"0457", title)
        · graph_node type=assignment_brief  author=student body={text: prompt}
        · graph_node type=rubric_translation author=ai      body=<0457 fixture rows + restate_prompt>
        · graph_node type=milestone_plan     author=ai      body=<0457 fixture steps>
     → { id }
  → web navigates into the project → StudioProjection → S0 任务解码 (OnboardingView, LIVE data)
  → student writes restate (≥15 runes) + taps ≤2 weakest rows
  → POST /api/v1/projects/{id}/onboarding
        · graph_node type=task_restatement author=student body={restate, weak_picks:[i,j]}
        · AppendEvent studio/onboarding_restated {text: restate}   (过程即数据 → assessor)
  → projection reflects assignmentText + studentRestate + studentWeakPicks (survives reload)
  → existing write loop → 就绪度 → 完成任务·归档 (unchanged)
```

## 4. The 0457 onboarding fixture

New authored data file `apps/api/internal/onboarding/fixtures/0457.json`, Go-embedded
(`//go:embed`, the same single-source pattern as the rubric packs). Shape mirrors seed
0018's proven node bodies:

```json
{
  "restate_prompt": "用自己的话，说清这份任务在考什么。",
  "rows": [
    { "official": "<0457 criterion A wording>", "plain": "<plain-language>", "weak": false },
    ...
  ],
  "steps": [ "<plan step 1>", ... ]
}
```

Loader `onboarding.Fixture(qualification string) (Fixture, bool)` — returns the 0457
fixture; `false` for any other qualification (N4 adds more). The `rows`/`steps` content
is authored from the real 0457 rubric (reuse seed 0018's existing rows as the starting
content so the live funnel matches the demo). `weak` here is a *suggested* watch-flag,
distinct from the student's own picks (§6).

## 5. Backend

### 5.1 `POST /api/v1/projects` — `createProject`
New handler `apps/api/internal/api/project_create.go`; route in `api.go` beside the GETs.
- Auth: `UserFromContext`. **Entitlement:** `HasEntitlement(ctx, u)` gate (consistent with every write path, even though this burns no tokens) → 402 if not entitled.
- Body: `{ "title": string (optional), "prompt": string (required, non-empty) }`. Empty title → default (`"未命名论文"` or a trimmed first line of prompt — pinned in plan). Reject empty prompt (400).
- **Atomic** (`pgx` tx via a store helper, mirroring signup's class+enrollment tx): `CreateProject(user_id, qualification:"0457", title, deadline:nil, board_cfg_ver:1)`; then three `InsertGraphNode`s (assignment_brief, rubric_translation from fixture, milestone_plan from fixture). If the fixture is missing, 500 (should never happen for 0457). Rollback on any error.
- Returns `201 {"id": "<uuid>"}`.

### 5.2 `POST /api/v1/projects/{id}/onboarding` — `submitOnboarding`
New handler `apps/api/internal/api/onboarding_submit.go`; route under the project sub-routes.
- Auth + **owner check** via the existing `loadOwnedProject` (404 hides other users' projects). Entitlement gate.
- Body: `{ "restate": string, "weakPicks": []int }`. Validate `restate` ≥ **15 runes** (mirrors dc.html `s0RestateOk`); `weakPicks` length ≤ 2, each a valid row index (else 400).
- Writes: `InsertGraphNode(project_id, type:"task_restatement", author:"student", body:{restate, weak_picks})` + `AppendEvent(EventRow{ProjectID, Surface:"studio", Type:"onboarding_restated", Payload:{text: restate}})`. Telemetry-failure on the event is a `slog.Warn`, not a request failure (matches studioturn).
- Re-submit allowed; the projection reads the **latest** `task_restatement` node (§6). Returns `200`.

### 5.3 Projection (`studio/projection.go` + `dto.go`)
Extend `projectOnboarding` and `OnboardingDTO` (additive):
- Read `assignment_brief` node → `OnboardingDTO.AssignmentText string` (`json:"assignmentText"`).
- Read the **latest** `task_restatement` node → `OnboardingDTO.StudentRestate string` (`json:"studentRestate"`) + `OnboardingDTO.StudentWeakPicks []int` (`json:"studentWeakPicks"`, default `[]`).
- The existing `rubric_translation`/`milestone_plan` reads are unchanged.
- Update `dto_parity_test.go` for the three new fields. The `Weak bool` on `RubricRowDTO` stays the fixture flag; the student's picks are the separate `StudentWeakPicks`.

## 6. Contracts

- `packages/contracts/src/studioState.ts` (or wherever `OnboardingDTO`'s Zod lives): add `assignmentText: z.string()`, `studentRestate: z.string()`, `studentWeakPicks: z.array(z.number().int())` to the onboarding schema. Field-exact with the Go DTO.
- New `CreateProjectBody = { title: z.string().optional(), prompt: z.string().min(1) }` and result `{ id: z.string() }`.
- New `OnboardingSubmitBody = { restate: z.string(), weakPicks: z.array(z.number().int()) }`.

## 7. Web

### 7.1 API client (`apps/web/src/api/projects.ts` + facade)
- `createProject(body: { title?: string; prompt: string }): Promise<{ id: string }>` (POST, parse result).
- `submitOnboarding(projectId: string, body: { restate: string; weakPicks: number[] }): Promise<void>` (POST).
- Both wired into the `ApiClient` facade additively.

### 7.2 Directory (binding dc.html `:669-708`)
A directory landing (`apps/web/src/studio/Directory.tsx`, rendered by `StudioContainer` instead of auto-opening `list[0]`):
- Tabs 写作工作室 / 项目工作室 (the latter disabled "即将上线").
- Section 你的论文: subtitle "从贴题目开始，AI 陪你一站站把论证走扎实。" + a **新建论文** button.
- A list of the student's projects (from `listProjects`): title, qualification, station, **status badge** (进行中 / 已归档 — this is the minimal "finish discoverability" for N1; the finish *action* stays in `ReviewView`). Clicking a row opens that project's studio.
- Empty list → the 新建论文 button is the sole affordance (replaces the static placeholder).

### 7.3 Create flow
新建论文 → a modal/inline form: optional 标题 input + a 贴上任务要求 textarea → `createProject` → on success, open the new project's studio (lands at S0). Disable submit while the prompt is empty or the request is in flight (no double-submit).

### 7.4 OnboardingView submit + hydrate (`studio/views/OnboardingView.tsx`)
- Hydrate initial state from the projection: show `assignmentText` (the pasted task, so the student can restate it), prefill the restate textarea from `studentRestate`, and mark `studentWeakPicks` on the rubric rows.
- On submit (a "记下我的理解" / confirm action, enabled when restate ≥ 15 runes), call `submitOnboarding` and refresh the projection. The picks + restate now persist (today they are local-only `useState`).
- No change to S1/S2 stubs.

## 8. Testing

- **Go (api, testcontainers):** `createProject` — creates a project + exactly the 3 nodes with correct types/authors/bodies (fixture-sourced), returns 201+id, `qualification="0457"`; owner-scoped; entitlement gate; empty prompt → 400; atomic (a forced node-insert failure leaves no orphan project — assert via a tx-rollback path if feasible, else document). `submitOnboarding` — persists the `task_restatement` node + `onboarding_restated` event, restate <15 runes → 400, other user's project → 404.
- **Go (studio):** `projectOnboarding` reads `assignment_brief` + latest `task_restatement` into the 3 new DTO fields (two `task_restatement` nodes → latest wins); fixture-fed projection (not the seed row).
- **Go (onboarding):** fixture loader returns 0457, `false` for an unknown qualification; the embedded JSON parses to the expected shape.
- **Contracts:** the extended onboarding schema + `CreateProjectBody` + `OnboardingSubmitBody` parse a representative payload and reject a missing/short field; dto-parity.
- **Web:** `createProject`/`submitOnboarding` client tests (mock fetch, assert body + parse); Directory (renders list + 新建论文, status badges, empty-list affordance); create-flow (submit calls `createProject` then navigates; disabled while empty/in-flight); OnboardingView (hydrates from projection, submit gated at 15 runes, calls `submitOnboarding`).

## 9. Invariants

- **Client never calls a model directly;** N1 makes **no** model call at all (fixture-driven) and records zero `llm_call` rows.
- **Owner isolation:** create is owner-from-session; onboarding submit + all reads go through `loadOwnedProject`.
- **Atomic creation:** project + its 3 nodes are one transaction — never a project with missing onboarding.
- **过程即数据:** the student's restate is persisted as both a node and an event.
- **Single source:** the 0457 onboarding content lives in ONE fixture (Go-embed); the frontend receives it via the projection, never re-encodes it.
- **No migration; no schema change.** Rides `graph_node`'s open type + existing queries.
- **HasEntitlement** gates both new write endpoints (seam stays consistent for N6/billing).

## 10. Out of scope (this slice)

- The live-LLM "decode" intake (chose board-static fixture; a later enhancement).
- Multi-board fixtures / qualification picker (N4 — 0457 only, hardcoded).
- Every write-loop correctness seam (N6): live gate counts, event-model normalization, `GetCardInstance` scoping, anchor labels, preview-snapshot drift, terminal idempotency, `finishProject` double-submit lock, report re-POST-on-422, Enter-to-send, dc.html 「批判思维」copy fix.
- The finish *action*'s richer discoverability (N1 only adds status badges in the directory; the button stays in `ReviewView`).
- S1/S2 onboarding depth (still stubbed).
- Feeding `assignment_brief` into the coach prompt (stored + shown in S0; coach-integration deferred).
