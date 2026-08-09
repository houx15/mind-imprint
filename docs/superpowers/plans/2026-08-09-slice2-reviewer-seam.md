# Slice 2 — Reasoning-reviewer seam + framework-readiness reviewer

> REQUIRED SUB-SKILL: superpowers:executing-plans. Steps use `- [ ]`.

**Goal:** Introduce a reusable **reasoning-model reviewer** that returns a typed `ReviewVerdict {ready, why, suggestions[]}`, and use it at the **framework-readiness gate**: when the framework's 4 dims fill, the flagship reasoning model reads the whole framework and returns concrete suggestions, surfaced to the student. **The plan still auto-generates as today** — the reviewer *reads + suggests* (all-statuses.md §2), it does not gate plan-gen (per the locked 铁律 scope: plan-gen is a deterministic system step, not a manipulation concern).

**Scope decisions (user-confirmed):** guide-step track DEFERRED to slice 3 (built with its UI consumer). Framework plan-gen stays automatic; reviewer is the "read + give suggestions" step.

**Architecture:** new `agent.ReviewFramework` (pure model call, mirrors `ProposeReview`/`generatePlanItems`) run via `EvalResolver` (flagship/reasoning, tier "flagship") from `reconcileStudioFunnel` at the plan-gen transition; verdict threaded through `advanceStudioFlow` → the 3 coach handlers → `OrchestratorReply.reviewVerdict`; frontend renders it as a system note. No migration (no new state).

**Tech:** Go (`apps/api`), Zod (`packages/contracts`), React (`apps/web`). Go api suite foreground (testcontainers).

## Global Constraints
- Client never calls the model; reviewer runs server-side via `EvalResolver`, metered (`purpose="framework_review"`). 评估/评审走旗舰绝不降级.
- 铁律 scope: the "AI doesn't do it for the student" law is WRITING-only — do NOT gate plan-gen behind confirmation.
- Reviewer is best-effort: a nil `EvalResolver`, a model error, or unparseable JSON → verdict nil, plan-gen + turn proceed unchanged (never break the turn).
- Deploy = commit main + push, then `.deploy-local/deploy.sh full`.

## Task 1: `ReviewVerdict` contract
- Files: `packages/contracts/src/orchestrator.ts`; test `packages/contracts/test/orchestrator.test.ts` (or library).
- [ ] Add `ReviewVerdict = z.object({ ready: z.boolean(), why: z.string(), suggestions: z.array(z.string()) })`; add `reviewVerdict: ReviewVerdict.nullable().optional()` to `OrchestratorReply`.
- [ ] Parity test: an OrchestratorReply with a reviewVerdict parses; without it parses (optional).
- [ ] Commit `feat(contracts): ReviewVerdict on OrchestratorReply`.

## Task 2: `agent.ReviewFramework`
- Files: create `apps/api/internal/agent/framework_review.go` + `framework_review_test.go`.
- Produces: `type FrameworkVerdict struct{ Ready bool; Why string; Suggestions []string }`; `type FrameworkReviewInput struct{ Title, Objective, Reason, Activities, Resources, Counterpoints string }`; `func ReviewFramework(ctx, prov gateway.Provider, resolved gateway.Resolved, in FrameworkReviewInput) (FrameworkVerdict, gateway.ChatUsage, error)`.
- [ ] Failing test: a stub provider returning `{"ready":true,"why":"...","suggestions":["a","b"]}` yields the parsed verdict; a fenced ```json ``` reply still parses; garbage → error (caller degrades).
- [ ] Implement: system prompt (严谨而鼓励的 IB 导师；读五维；判断"是否足以据此生成计划"；返回 JSON {ready,why,suggestions[≤4]}；建议具体指向最弱处；只回 JSON), `gateway.Collect`, 2-attempt retry on parse failure, clamp suggestions.
- [ ] Run `cd apps/api && go test ./internal/agent/ -run TestReviewFramework`; pass.
- [ ] Commit `feat(agent): ReviewFramework reasoning reviewer`.

## Task 3: Wire the reviewer into the framework gate
- Files: `apps/api/internal/api/coach.go` (`reconcileStudioFunnel`, `advanceStudioFlow`, the 3 handlers, reply DTO); test `coach_capabilities_test.go` (funnel test).
- Produces: `type reviewVerdictDTO struct{ Ready bool json:"ready"; Why string json:"why"; Suggestions []string json:"suggestions" }`; `reconcileStudioFunnel` returns `(state, planGenerated bool, verdict *reviewVerdictDTO)`; `advanceStudioFlow` returns `(state, *nextStepDTO, planGenerated bool, verdict *reviewVerdictDTO)`; `orchestratorReplyDTO` gains `ReviewVerdict *reviewVerdictDTO json:"reviewVerdict"`.
- [ ] When `planGenerated` flips true, run `agent.ReviewFramework` via `a.d.EvalResolver` (skip if nil) over the just-completed proposal; meter `purpose="framework_review"`; build the verdict DTO (nil on any error). Thread it through `advanceStudioFlow` → set `reply.ReviewVerdict` in postCoach / postCoachStart / postCoachAdvance.
- [ ] Failing test: extend the funnel test — a coach turn that fills the 4th dim returns `planGenerated` AND `reviewVerdict != nil` with non-empty suggestions (stub `EvalResolver` returns a fixed verdict). A later turn (plan already exists) returns `reviewVerdict == nil` (runs once).
- [ ] Run `go test ./internal/api/ -run 'TestFunnel|TestPostCoach'` (foreground); pass.
- [ ] Commit `feat(coach): framework-readiness reviewer at plan-gen (EvalResolver)`.

## Task 4: Frontend surfaces the verdict
- Files: `packages/contracts` type re-export (auto); `apps/web/src/workspace/WorkspaceContainer.tsx` (sendStudioTurn / startJourney apply); `apps/web/src/studio/ai/StudioChatContext.tsx` (StudioChatMsg already has `hint`); reuse the SubagentHint/system-note render.
- Produces: when `reply.reviewVerdict` is present, append a system note under 印记's narrate — "印记读了你的框架：<why>" + the suggestions as bullets (reuses the existing `hint` message path).
- [ ] Failing test: a StudioCoachChat render with a message carrying the verdict-note renders the suggestions text.
- [ ] Implement: in `sendStudioTurn`/`startJourney`, after appending narrate, if `reply.reviewVerdict` append a `hint` message composed from why+suggestions. tsc + vitest.
- [ ] Commit `feat(web): surface framework reviewer suggestions`.

## Task 5: Full suite + deploy + smoke
- [x] contracts vitest 323 (fixed a slice-1 stale key-set test); web tsc0 + vitest 1032; `go test ./internal/agent/` green; full `go test ./internal/api/` green EXCEPT the known `TestWeeklyReportForSeededClass` flake.
- [x] Committed + pushed; `.deploy-local/deploy.sh full` → **`full @ b4a7e01`**.
- [x] Live smoke (prod, authenticated fetch as Phoebe): a fresh project filled through the 4 framework dims → plan auto-generated (7 items) + the **flagship reasoning reviewer produced a real, high-quality verdict** (`ready:true` + 4 concrete suggestions: pin dataset variables, operationalize 传播速度, add a 反例, budget the 4 weeks). `reviewVerdict` surfaced on the next coach turn and was **null on the turn after** (surfaces once). nextStep → 写研究提案 offered.

**SLICE 2 COMPLETE (shipped `full @ b4a7e01`, 2026-08-09).** The reasoning-reviewer seam + framework-readiness reviewer are live; plan-gen stays automatic; verdict surfaces once then clears; degrades silently. **Deferred to slice 3:** the guide-step track (built with its UI consumer).

**Acceptance:** filling the framework's 4 dims runs a flagship reasoning review whose {ready, why, suggestions} is surfaced to the student; the plan still auto-generates; the review runs once (not on later turns); nil-resolver/model-error degrades silently. ✅

## Doc check (all-statuses.md §2)
§2 AI-role: "after all five are finished, read the whole framework, give some suggestions, and propose that it's time to generate a plan." → This slice: reasoning reviewer reads the framework + gives suggestions; plan auto-generates + coach offers 写研究提案. Faithful (suggestions delivered; plan proposed/generated). Deviation from §status-change "click generate plan" = auto-gen, user-ruled acceptable (not a 铁律 concern).
