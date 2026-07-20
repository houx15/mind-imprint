# N2 · 评估 View Completion — Design Spec

> The second finishing slice (N2) of the student-platform remainder
> (`docs/2026-07-20-student-platform-remaining-work.md`). Completes the
> student-writable part of the 评估 view: **self-score + S0↔S6 prediction reveal
> + reflection retro editor**, on top of the shipped 就绪度 gauge. Defers the
> AI-usage declaration (N2d, needs the event ledger) and export forks (N2e).

**Date:** 2026-07-20
**Status:** approved (design shape), ready for spec review → plan
**Binding design:** `docs/design/思维印记_工作区.dc.html` — 评估 view `:1158–1227` (就绪度 done · 先自己评一评 `:1186–1200` · 写一段研究回顾 `:1202–1209` · AI 使用申报单 `:1211–1224` deferred).
**Follows:** N1 (`f3c4b31`) — reuses its `task_restatement.weak_picks` S0 capture and its board-fixture + node/endpoint patterns.

---

## 1. Goal & the gap it closes

The 评估 view today is the 就绪度 gauge + finish button only (`ReviewView.tsx`, Slice 9 `644e1d2`). The binding dc.html shows it as one screen of four stacked cards. N2 adds the three student-writable ones:
- **N2a 先自己评一评** (self-score) — the student rates each criterion against the mark scheme, beside the gauge.
- **N2b S0↔S6 prediction reveal** — pairs the student's S0 predicted-weakest criteria with the gauge's actual weakness. "Today S0 and S6 don't talk to each other; this connects them." (product-spec §138) — the metacognition payload (D6).
- **N2c 写一段研究回顾** (retro) — a student-written reflection (RL-4: AI never authors), which is also the `reflection` node the S6 `reflect_archive` gate names.

## 2. Settled decisions (with the user, 2026-07-20)

1. **One slice = N2a + N2b + N2c** (the student-writable 评估 cards). Defer **N2d** (AI-usage declaration — needs the event ledger) and **N2e** (export forks — its own underspecified item, no format decided).
2. **Unify the taxonomy onto `review_criteria`.** Three near-but-not-equal 4-criteria sets exist today (gauge `review_criteria` 来源与证据/分析/评估/表达与组织; N1's S0 rubric rows 分析/证据/反思/表达; dc.html self-score dims). The prediction loop only works if the student predicts over the **tested** criteria (spec §128: "predicts their two weakest criteria, tested at S6"). So: **S0 prediction, self-score, and the gauge all use `review_criteria`** (the skill config, already there). Concretely N2 rewrites N1's 0457 onboarding fixture rows to be plain-language `review_criteria` in `review_criteria` order, so `weak_picks[i] ↔ review_criteria[i] ↔ gauge[i]`. Self-score dims come straight from `sk.ReviewCriteria` — no new dims config. (Fixture-content change only; new projects + N1's fixture test still pass — it asserts count/non-empty, not content; seed 0018's own node is untouched.)
3. **Read `weak_picks` as the S0 prediction** (N1 decision, carried). The skill gate's `weakness_prediction` node-type expectation is a separate advancement concern — a logged carry-forward, not blocking.

## 3. The unified taxonomy — `review_criteria`

Single source: `writing-project.json:8–13` (`sk.ReviewCriteria`): `表D 来源与证据 (4)`, `表E 分析 (4)`, `表F 评估 (3)`, `表H 表达与组织 (3)`, in this order. Everything in the 评估 view keys off it:
- **gauge** — `projectReadiness` already iterates it (projection.go:415).
- **S0 prediction** — the rewritten 0457 fixture rows are plain-language of these 4, in this order, so `weak_picks` (0–3 indices, ≤2) point at `review_criteria`.
- **self-score** — one row per `review_criteria` entry (code+name), × 3 universal bands.

### 3.1 Rewrite `apps/api/internal/onboarding/fixtures/0457.json` `rows`
Four rows, in `review_criteria` order, `official` = the criterion name(+code), `plain` = a friendly one-liner, `weak` = a suggested watch-flag:
```json
"rows": [
  { "official": "来源与证据（表D）", "plain": "用可信来源，并说清它可不可信", "weak": true },
  { "official": "分析（表E）",       "plain": "能从不同视角分析，不只罗列观点", "weak": true },
  { "official": "评估（表F）",       "plain": "权衡取舍、指出局限，而不是各打五十大板", "weak": false },
  { "official": "表达与组织（表H）", "plain": "结构清楚、表达清晰", "weak": false }
]
```
`restate_prompt` and `steps` unchanged.

## 4. Data model (no migration — `graph_node.type` is open, migration 0017)

Two new student-authored node types + one read-only projection. Both writes mirror N1's `onboarding_submit.go` (owner-scoped, entitlement-gated, node + best-effort event).

- **`self_score`** node — author `student`, body `{"scores":[{"code":"表D","band":2}, …]}` where `band ∈ {0,1,2}` (还需努力/基本达到/稳了). Latest node wins.
- **`reflection`** node — author `student`, body `{"text":"…"}`. Latest wins. **This is the node the S6 `reflect_archive` gate names** (`writing-project.json:104–115`, student-written `reflection`), so writing the retro advances that gate.
- Prediction reveal writes nothing — it is a pure projection over existing data.

### 4.1 Endpoints (mirror `onboarding_submit.go`)
- `POST /api/v1/projects/{id}/self-score` — body `{ "scores": [{ "code": string, "band": int }] }`. Validate each `code ∈ review_criteria` and `band ∈ 0..2` (else 400). Write `self_score` node + `self_scored` event. Owner via `loadOwnedProject` first, then `HasEntitlement`. 200.
- `POST /api/v1/projects/{id}/reflection` — body `{ "text": string }`. Validate `text ≥ 20 runes` (a reflection is substantive; else 400). Write `reflection` node + `reflection_written` event. Same owner/entitlement order. 200.

## 5. Projections + DTOs + contracts

Three projections in `studio/projection.go`, three DTOs in `studio/dto.go` added to `StudioProjection`, three Zod mirrors in `studioState.ts`. `Project()` (projection.go:326) assembles them beside `Onboarding`/`Readiness`.

- **`projectSelfScore(sk, d) SelfScoreDTO`** — `{ dims: [{code, name, band}], bands: [string] }`. `dims` = one per `sk.ReviewCriteria` (code+name); `band` = the student's pick from the latest `self_score` node, or **-1** (unpicked) — never null; `bands` = the 3 universal labels `["还需努力","基本达到","稳了"]` (a Go constant, single-source).
- **`projectPrediction(sk, d) PredictionDTO`** — `{ predicted: [{code, name}], actual: [{code, name}], overlap: int, revealed: bool }`. `predicted` = `weak_picks` → `sk.ReviewCriteria[i]`. `actual` = the gauges (`projectReadiness`) whose `level != full`, by code+name. `overlap` = count of codes in both (descriptive self-knowledge distance, **never a score** — RL-5). `revealed` = whether the whole-draft review has informed the gauge (any gauge `lit>0` or `note!=""`); before that, `actual=[]`, `revealed=false` → the view shows "跑完整稿体检后，这里对照实际".
- **`projectReflection(d) ReflectionDTO`** — `{ text: string, prompts: [string] }`. `text` = latest `reflection` node text (or ""); `prompts` = 3–4 causal-reflection prompt chips (a Go constant, single-source; e.g. "哪一步真正改变了你的判断？为什么？").

Zod (`studioState.ts`): `SelfScoreFx`/`PredictionFx`/`ReflectionFx` field-exact with the DTOs; `StudioProjection` gains `selfScore`/`prediction`/`reflection`; new `SelfScoreSubmitBody = { scores: [{code, band:int}] }` and `ReflectionSubmitBody = { text: string }`. (Adding required projection fields will break existing StudioProjection fixtures — update ALL of them, both packages, as N1 learned.)

## 6. Web

- **Clients** (`api/projects.ts` + facade, additive): `submitSelfScore(projectId, { scores })` , `submitReflection(projectId, { text })` — POST, mirror N1's `submitOnboarding`.
- **`ReviewView`** — below the existing gauge, render three cards (binding dc.html + the prediction reveal):
  1. **Prediction reveal** (right after the gauge): "开头你预测最弱的是 X、Y" + (when `revealed`) "跑完这轮，评分表上实际最弱的是 Z"; a gentle overlap line. Read-only.
  2. **先自己评一评** (dc.html:1186–1200): badge "已评 n/4", "对照上面的就绪度，给自己每一块打个档"; one row per dim with 3 band chips; picking calls the submit handler; hydrates from `selfScore.dims[].band`.
  3. **写一段研究回顾** (dc.html:1202–1209): prompt chips + a value-bound textarea (hydrated from `reflection.text`); a "记下我的反思" submit enabled at ≥20 runes calling the submit handler. RL-4 copy "这段反思由你自己写——印记只提供问题，不代笔".
- **Threading**: `selfScore`/`prediction`/`reflection` travel on `state` (projection fields, like `Readiness`); the write callbacks `onSelfScore`/`onReflection` travel on the `review` prop object (like `onFinish`) through StudioContainer → StudioShell → ViewFrame → ReviewView. StudioContainer defines the handlers (`api.submitSelfScore(projectId, …)` + `refetchProject()`), guarded by `projectId`.

## 7. Testing

- **Go (api, testcontainers):** self-score endpoint persists a `self_score` node + `self_scored` event, rejects an out-of-range band / unknown code (400), owner-isolation 404, entitlement. Reflection endpoint persists a `reflection` node + event, rejects <20-rune text (400), owner/entitlement. Assert node **body content** (not just row counts), per N1's lesson.
- **Go (studio):** `projectSelfScore` (latest node wins, unpicked → band -1, dims = review_criteria); `projectPrediction` (predicted from weak_picks; actual from empty/partial gauges; overlap count; `revealed` false pre-review → actual empty; revealed true post-review); `projectReflection` (latest text + prompts). Fixture-fed, not seed.
- **Go (onboarding):** the rewritten fixture still loads (4 rows, non-empty, generic prompt).
- **Contracts:** the 3 new Fx schemas + 2 submit bodies parse/reject; **all StudioProjection fixtures across contracts + web updated** for the 3 new required fields (run the FULL contracts + web suites, not the narrow test).
- **Web:** `submitSelfScore`/`submitReflection` clients (POST + body); ReviewView renders the 3 cards, hydrates self-score picks + reflection text, gates the reflection submit at 20 runes, shows the prediction reveal's pre-review vs revealed states.

## 8. Invariants

- **No model call; no migration.** Fixture/config-driven DB writes only; zero `llm_call`. New node types ride `graph_node`'s open type.
- **RL-3**: the gauge + the prediction reveal are never a predicted grade; overlap is a descriptive count.
- **RL-4**: self-score + retro are **student** writes; the platform never authors reflective text.
- **RL-5**: no aggregate score anywhere; self-score is the student's own per-criterion assessment.
- **Owner isolation + entitlement**: both write endpoints go through `loadOwnedProject` (before the entitlement gate) + `HasEntitlement`.
- **Single source**: one taxonomy (`review_criteria`) drives gauge + prediction + self-score; bands + prompts are single Go constants surfaced via the projection; the web never re-encodes them.
- **过程即数据**: self-score + reflection each persist as a node AND an event.

## 9. Out of scope (this slice)

- **N2d · AI-usage declaration** — the 4th dc.html card; needs event-ledger aggregation + the `declaration_signed` human gate. Separate slice.
- **N2e · Export forks (RL-4)** — no feature exists, format undecided (download vs link); its own item, needs a design decision.
- The reflection "evidence pack" (S1 pre-registration + snapshot diffs beside the retro) — editor ships now; the evidence pack is a later enrichment.
- Gate alignment for the S0 `weakness_prediction` node type (carry-forward; N2b reads `weak_picks`).
- Feeding self-score/reflection into the assessor or 成长报告 aggregation (the reflection node exists for the S6 gate + future consumption; wiring it into the report is deferred).
