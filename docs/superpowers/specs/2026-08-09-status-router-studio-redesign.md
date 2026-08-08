# 印记 Studio Redesign — Status-Router Architecture

**Status:** approved design (2026-08-09), pending implementation plan.
**Supersedes the runtime shape of:** `docs/superpowers/specs/2026-08-07-agentic-studio-orchestrator-redesign.md` (the ONE mega-orchestrator). The north-star ("印记 IS the agent, rooms are its tools") is unchanged; this changes HOW the agent decides.

## Why

A real-frontend student walk-through (2026-08-09, Phoebe demo, EE "chronic sleep deprivation & working memory") surfaced three defects, all traceable to ONE cause — a single mega-orchestrator prompt that must reason over the whole project every turn:

1. **Note-drop.** The reasoning model narrates "我把这条记进提案了" but omits the `propose_note` tool (3/3 framing turns live; the proposal board stayed 0/4; a fresh server fetch confirmed nothing was stored; re-entering could not recover it). Tactically patched with a server-side backstop (`4e86d3c`), but the class remains.
2. **Plan-desync.** After the plan auto-generates, the stage parks at `plan_generation`, whose name the model reads as "generate now" — so it offered to regenerate an existing 8-item plan. Tactically patched via the projection (`4e86d3c`).
3. **Single-buffer writing room (unpatchable).** The writing room has ONE `edit_buffer` + ONE `finish-writing`. The plan's 阶段一 task "撰写研究提案" has nowhere to live except the same buffer the essay will use — proposal and essay collide, and "完成写作" is a single terminal event, not per-artifact.

The root fix (the user's direction): **progressive disclosure**. Give each project *status* its own short prompt, small tool set, cards, and surface. Each turn's decision space shrinks enough that a **fast non-reasoning model** can drive it reliably — closing the reliability AND the speed problem (the reasoning-model latency thread we never closed) at once.

## Goals

- Decompose the mega-orchestrator into a **status registry**: `status → {goal, allowedTools, allowedCards, surface, advanceRule}`.
- Move project transitions and structural guarantees (plan-gen, note capture) into **deterministic server-side routing**, not model judgment.
- Run the per-status conversational turn on a **fast non-reasoning model**; keep flagship reasoning ONLY for evaluation (整稿体检 / reflection / assessment) — 评估绝不降级.
- Give the writing room **multiple named documents** (proposal doc ↔ essay doc), each with its own draft, snapshots, and 完成.
- Preserve all four 铁律: AI 克制 (never writes the student's text), 不操纵 (触发自动，打开由学生确认 — transitions are one-tap-confirmed), 一次只问一个, 过程即数据.

## Non-goals

- No change to the evaluation model / rubric / assessment projections.
- No change to auth/org/DB platform.
- Not building new cards; per-status card *subsets* draw from the existing registry.
- Reading-room internals (lens dive, exploration graph) are unchanged; only its status wiring (a cross-cutting detour that returns to origin) changes.

## Architecture

### 1 · Status machine (backbone)

A linear spine; reading is a cross-cutting detour that returns to the status it was opened from.

```
立题(topic) → 立项(framework) → 写提案(proposal) → 写正文(essay) → 复盘(review)
                                    ↓↑              ↓↑
                                  阅读室 (reading; opened on demand, returns to origin)
```

Canonical statuses (rename/collapse of today's `StudioStage`; a migration maps old → new):

| status | meaning | old StudioStage |
|---|---|---|
| `topic` | no research question yet — help frame one | `topic_discussion` |
| `framework` | the four 立项 points (+ optional 反例); board surface; plan auto-generates here | `proposal_forming` + `plan_generation` |
| `proposal` | write the **proposal document** (prose: RQ + lit scope + plan); own 完成 | `proposal_writing` + `proposal_review` |
| `essay` | write the **essay** (大纲/片段/正文); own 完成 | `body_writing` |
| `review` | 复盘: reflection + AI 使用声明 + 过程评估 | `retrospective` |

Note `plan_generation` and `proposal_review` disappear as *statuses* — plan generation becomes an event within `framework`; proposal review becomes an event within `proposal`. This directly kills defect 2 (no status whose name means "generate now").

### 2 · Status registry + router (replaces the mega-prompt)

A single Go registry (`internal/agent/studioflow`), the source of truth:

```go
type Status struct {
    Code         string          // "framework" | "proposal" | ...
    Goal         string          // ≤2 lines — the router's only north-star for this status
    SystemPrompt string          // SHORT status-specific posture (goal + its 2-4 tools ONLY)
    Tools        []string        // the closed tool subset available in this status
    Cards        []string        // card_ids offerable in this status (subset of the registry)
    Surface      OpenTool        // which room opens (forming/writing/reading/reflection)
    Doc          WritingDoc      // "" | "proposal" | "essay" — the active writing document
}
```

Per-status **allowed tools** (small — the whole point):

| status | tools |
|---|---|
| `topic` | `propose_question` |
| `framework` | `propose_note`, `summon_card`(framework cards) |
| `proposal` | `open_reading`, `request_review`(proposal doc), `finish_part`, `summon_card`(proposal cards) |
| `essay` | `open_reading`, outline/snippet ops, `request_review`(essay), `finish_part`, `summon_card`(essay cards) |
| `review` | (no tools — supportive reflection only) |

`generate_plan` and `set_status` **leave the tool set entirely** — both become deterministic (below). The router LLM never advances status and never generates plans; it converses and picks from its tiny tool list. This is why a fast model suffices and why defects 1–2 can't recur (propose_note is THE tool of `framework`; there is no plan tool in `essay`).

The dispatch (`postCoach`): load status → build the SHORT status prompt (goal + tools + a status-scoped projection slice) → one fast-model call → parse the small envelope → apply the status's tools → run the deterministic router.

### 3 · Deterministic router (server-side transitions + guarantees)

Replaces `reconcileStudioFunnel`, generalized to every transition. After each turn (and on the relevant REST writes), pure Go decides:

- `topic → framework`: a research question exists (proposal.objective non-empty).
- `framework`: when all 4 required dims filled and no plan exists → **auto-generate plan** (existing `regeneratePlan`); then **offer** (one-tap) `→ proposal`.
- `proposal → essay`: proposal doc `finish_part` fired → offer `→ essay`.
- `essay → review`: essay `finish_part` fired → offer `→ review`.

"Offer" = the reply carries a **`nextStep {label, toStatus, surface}`**; the student taps to advance (铁律②: 打开由学生确认). The transition never auto-fires. The **note-capture guarantee** (today's backstop) stays but is cleaner: in `framework` the router, not the model, owns whether a student turn produced a dim — `propose_note` is the only tool and the extraction backstop remains as the safety net.

### 4 · Writing surfaces (multi-document)

Data model: add `doc_kind TEXT` (`'proposal' | 'essay'`) to `edit_buffer` and `draft_snapshot`; per-doc finish via a `writing_finish` row keyed by `(project_id, doc_kind)` (replaces the single `project.writing_finished_at`, which is migrated to an `essay` finish row). REST (`/buffer`, `/snapshots`, `/finish-writing`, review) gains a `doc` param (default `essay` for back-compat). The active document is derived from status: `proposal` status → proposal doc; `essay` status → essay doc. Switching status switches the active document; the other is preserved. Left rail unchanged (提案要点 read-only + reading takeaways + 批注). The proposal doc is a plain prose surface (textarea + markdown preview); the essay doc keeps 大纲/片段/正文.

### 5 · Model strategy

Each status router → **fast model** via a new `NewFastChaperoneResolver` (deepseek-v4-flash, or v4-pro with `{"thinking":{"type":"disabled"}}` if flash under-performs — decided empirically in Phase A). Evaluation resolver (`NewEvalKeyResolver`) is untouched — flagship reasoning, never downgraded. Because the per-status prompt is 3-5 lines with 2-4 tools, the fast model's known weakness (drifting off a large JSON contract) does not bite.

## Phasing (one spec, phased build; each phase runs the real-frontend loop)

- **Phase A — router core.** Status registry + per-status prompts + deterministic router dispatch on the fast model, keeping today's surfaces. Behavior parity + reliability + speed proven via the real-frontend loop. Deletes the mega-prompt path.
- **Phase B — multi-document writing.** `doc_kind` on buffer/snapshots/finish; proposal doc ↔ essay doc; per-status 完成; REST `doc` param. Closes defect 3.
- **Phase C — polish.** Per-status card subsets enforced; cross-cutting reading-return; one-tap `nextStep` chips in the UI; remove dead code.

## Data flow (one framework turn, Phase A)

1. `POST /coach` → load `StudioState` (status) + status-scoped projection.
2. `studioflow.Status("framework")` → short prompt (goal + `propose_note`/`summon_card` + the 4-dim slice).
3. Fast-model call → `{narrate, tools:[propose_note?]}`.
4. Apply tools (note → pending confirm chip). Note backstop if the narrate claims a recording but no note fired.
5. Deterministic router: 4 dims filled + no plan → `regeneratePlan`; set `nextStep {→ proposal}`. Never auto-advances.
6. Persist state; return `{narrate, directive, note, nextStep, ...}`.

## Error handling

- Unknown status → default `framework` if started, else `topic` (never 500 the student).
- Fast-model parse failure → the existing prose-salvage + restrained fallback, unchanged.
- Deterministic router failures (plan-gen, finish) are best-effort and logged; the turn still returns.
- `doc_kind` on an unknown value → treated as `essay` (back-compat default).

## Testing

- Unit: registry integrity (every status has a surface + non-empty tool set; every listed tool is a known tool; every listed card exists); router transition table (each gate fires exactly on its condition; never advances backward; never auto-advances without the tap); `doc_kind` validation.
- Contract: `packages/contracts` gains `nextStep` on the reply and `doc` on writing DTOs; Zod ↔ Go parity tests.
- Live: the real-frontend Playwright loop is the acceptance gate for each phase — a student completes 立项 → 写提案 → 阅读 → 写正文 → 复盘 with the proposal and essay as distinct preserved documents, on the fast model, with 0 canned fallbacks.

## Acceptance (whole redesign)

A real student, driven by 印记 on the fast model, completes the full process on the real frontend: four points captured reliably (no dropped notes), plan auto-generated (no regenerate-offer), led step-to-step with one-tap confirms, proposal written as its own finished document, essay written separately, whole-draft review, reflection + AI reflection — proposal and essay both preserved and exportable. Per-turn latency materially lower than the reasoning-model baseline.
