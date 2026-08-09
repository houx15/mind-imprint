# Prod user walk + model-routing token/latency report (2026-08-10)

Deploy: `full @ 8e9d23d`, db v62, `mind-web.uni-robot.cn` / `mind-api.uni-robot.cn`.

## Full user walk (Phoebe, `?trial=1`, real prod) — PASS, 0 console errors

Verified live on the "印刷术…宗教改革（Phase B）" project (essay/review stage):

| Surface (this round) | Result |
|---|---|
| §3 recap-landing on return | ✅ opens on 管理 plan, **甘特图 default**, "欢迎回来… 继续工作" banner |
| §3 继续工作 → current status room | ✅ jumped to 回顾 (the project's status) |
| §7 review four-tab artifact panel | ✅ LEFT = 活动日志 / 研究框架 / 提案 / 成品 (all render real content), RIGHT = review form |
| §101/§115 还需要探索的 box | ✅ present in the writing panel + reading room |
| §5 reading confirm-start gate | ✅ manual 阅读 entry shows "开始一段文献探索？" before the room |
| §113/§116 search-guidance box | ✅ "让印记建议检索方向" → 3 relevant keyword+why suggestions, each with 搜索 |
| §4 reference panel | ✅ needs-box + empty-state render (multi-tab is unit-verified; this project had no curated refs) |

No bugs surfaced — 0 console errors across the whole walk.

## Model routing (current production)

| Work | Resolver | Model |
|---|---|---|
| Coach turn (the interactive thread) | FastChatResolver | **deepseek-v4-flash** |
| Thread compaction (coach_compact) | ChatResolver | deepseek-v4-pro (chaperone) |
| Guides / search-guidance / classify | FastChatResolver | deepseek-v4-flash |
| Framework review, 论证地图 saturation, mirror, assessment, course render, parent/weekly | EvalResolver | deepseek-v4-pro (**flagship — never downgrade**) |
| Plan-gen + whole-draft 整稿体检 (this round's fix) | EvalResolver | deepseek-v4-pro (flagship) |

## Token cost (prod `llm_call`, avg per call)

| Purpose | Model | avg tokens (in+out) | avg cost |
|---|---|---|---|
| coach (NEW) | v4-flash | 4192 | **$0.00090** |
| coach (OLD, pre-routing) | v4-pro | 2718 | $0.00140 |
| coach_compact | v4-pro | 680 | $0.00044 |
| search_guidance | v4-flash | 861 | $0.00019 |
| framework_review | v4-pro flagship | 847 | $0.00060 |
| plan_gen | v4-pro | 1869 | $0.00147 |
| order_review (整稿体检) | v4-pro | 2032 | $0.00154 |
| mirror | v4-pro flagship | 1931 | $0.00145 |
| assessment (terminal) | v4-pro flagship | 22471 | $0.01635 |

**Coach cost after the routing change:** moving the coach v4-pro → **v4-flash** cut per-turn
cost ~**36%** ($0.00140 → $0.00090) even though flash emits ~54% more tokens (4192 vs 2718) —
flash's per-token price is far lower. A full interactive turn ≈ coach ($0.0009) + one
compaction ($0.0004) ≈ **$0.0013/turn**. Reviewers stay on the flagship seam (unchanged).

## Response time (measured live; not DB-metered)

- **Coach turn ≈ 20–21s** (2 live turns: 20.0s, 21.2s) on a long thread. Breakdown: the
  v4-flash coach inference is the fast part; each turn on a long thread also runs a
  **v4-pro `coach_compact`** (~8–10s) that dominates the wall-clock.
- **search-guidance (v4-flash) ≈ 6s**.

**Interpretation:** the routing change made the coach model itself fast + ~36% cheaper, but
per-turn latency is still ~20s because (a) a v4-pro thread-compaction runs each turn on long
threads and (b) the narrate is not SSE-streamed to the UI. The known remaining latency win is
**streaming the coach narrate** (previously declined twice) and/or moving `coach_compact` to
the fast model — neither changed this round.
