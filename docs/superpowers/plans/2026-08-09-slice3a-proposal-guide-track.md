# Slice 3a — Proposal-writing sub-machine + 提问卡 sub-agent + 反例 prompt · Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax.

**Goal:** Give the proposal document a dynamic, content-derived guide-step track (define 2–4 sub-questions → one guided card per sub-question, one at a time) with free/guided modes and AI-generated cached guide cards; clean up the writing page; and fold in two framework-stage fixes — the 提问卡 as a real adaptive sub-agent, and a 反例 prompt before plan generation.

**Architecture:** New agent-package logic (pure `DeriveProposalSteps`; fast-model `GenerateProposalGuideStep` + `QuestionCardTurn`; flagship `ReviewProposalPart`) mirrors the existing `ReviewFramework` pattern (`gateway.Collect` + 2-attempt retry + defensive JSON parse). State lives on `StudioState` (jsonb, no migration). REST adds `proposal-track/*`, `cards/question-card/*`, and `framework/waive-counterpoints`. Frontend layers a `ProposalGuide` component + `QuestionCardModal` onto the existing `ProsePane`.

**Tech Stack:** Go (`apps/api`, sqlc @v1.27.0, testcontainers — Go api suite runs foreground ~7min), Zod (`packages/contracts`, source-imported), React + Vite + TS + Tailwind (`apps/web`, Vitest under `apps/web/test/` mirroring `src/`).

## Global Constraints

- **铁律① — AI never writes body text.** Guide cards give a guiding question + an English example (for a *different* prompt); the 提问卡 sub-agent guides until the student's own wording emerges (`suggestedObjective` is the student's articulated question echoed for confirmation, never an AI invention). `ReviewProposalPart` critiques, never rewrites.
- **铁律② — non-blocking.** Every advance / finish / plan-gen is a suggestion; a student who insists proceeds (recorded). The 反例 prompt has a skip; the sub-question define step never hard-blocks advance.
- **铁律③ — one question at a time.** One guide card / one sub-question card / one coach question per turn.
- **铁律 scope is writing-only** (see the repo's AGENTS.md): guide-card generation, step derivation, sub-question re-derivation, and advancing are deterministic system steps — no confirmation gate on them.
- **Never downgrade evaluation:** `我写好了` review + framework reviewer run on `a.d.EvalResolver` (flagship); guide-card generation + 提问卡 sub-agent + `我依然有问题` run on `a.d.FastChatResolver` (falls back to `ChatResolver` when nil).
- **Client never calls the model.** All model calls are server-side, metered (`RecordLLMCall`, `purpose` per call).
- **Card JSON single source stays in lockstep:** any card JSON edit is applied to BOTH `packages/contracts/cards/*.json` and `apps/api/internal/cards/specs/*.json`.
- **No migration:** new state is jsonb on `studio_state` (`GetStudioState`/`SetStudioState`).
- **Deploy** = commit to main + push, then `.deploy-local/deploy.sh full` from repo root.

---

## Task 1: `DeriveProposalSteps` + template (pure Go, tested first)

**Files:**
- Create: `apps/api/internal/agent/proposal_track.go`
- Test: `apps/api/internal/agent/proposal_track_test.go`

**Interfaces:**
- Produces: `WriteMode` (`ModeUnset`/`ModeFree`/`ModeGuided`); `SubQuestion{ID,Text string}`; `WritingTrack{Mode WriteMode; Started bool; StepIndex int; SubQuestions []SubQuestion; StepGuides map[string]string}`; `StepKind` (`KindFixed`/`KindSubqDefine`/`KindSubq`); `Step{Key,Title string; Kind StepKind; SubQuestionID string}`; `func ProposalFixedParts() []Step`; `func DeriveProposalSteps(t WritingTrack) []Step`.

- [ ] **Step 1: Write the failing test**

```go
package agent

import "testing"

func TestDeriveProposalSteps_NoSubQuestions(t *testing.T) {
	steps := DeriveProposalSteps(WritingTrack{})
	// 3 intro + research-plan(define) + 5 tail = 9 steps, no subq cards yet.
	if len(steps) != 9 {
		t.Fatalf("want 9 steps, got %d", len(steps))
	}
	if steps[0].Key != "understanding" || steps[2].Key != "thesis" {
		t.Fatalf("intro order wrong: %+v", steps[:3])
	}
	if steps[3].Key != "research-plan" || steps[3].Kind != KindSubqDefine {
		t.Fatalf("step 3 must be the subq-define step, got %+v", steps[3])
	}
	if steps[4].Key != "resources" || steps[8].Key != "expected" {
		t.Fatalf("tail order wrong: %+v", steps[4:])
	}
}

func TestDeriveProposalSteps_ExpandsPerSubQuestion(t *testing.T) {
	t2 := WritingTrack{SubQuestions: []SubQuestion{{ID: "a", Text: "q1"}, {ID: "b", Text: "q2"}, {ID: "c", Text: "q3"}}}
	steps := DeriveProposalSteps(t2)
	if len(steps) != 12 { // 9 + 3 subq cards
		t.Fatalf("want 12 steps for 3 sub-questions, got %d", len(steps))
	}
	// subq cards sit between research-plan (index 3) and resources.
	if steps[4].Kind != KindSubq || steps[4].SubQuestionID != "a" {
		t.Fatalf("first subq card wrong: %+v", steps[4])
	}
	if steps[6].SubQuestionID != "c" {
		t.Fatalf("third subq card wrong: %+v", steps[6])
	}
	if steps[7].Key != "resources" {
		t.Fatalf("resources must follow the subq cards, got %+v", steps[7])
	}
}
```

- [ ] **Step 2: Run to verify it fails** — `cd apps/api && go test ./internal/agent/ -run TestDeriveProposalSteps` → FAIL (undefined).
- [ ] **Step 3: Implement.** `ProposalFixedParts()` returns the 8 fixed steps + the `research-plan` (KindSubqDefine) step in golden order: `understanding`,`question-scope`,`thesis`,`research-plan`(define),`resources`,`challenges`,`method`,`feasibility`,`expected` — each with the Chinese title from the spec's table. `DeriveProposalSteps(t)` builds: intro three + `research-plan` + one `KindSubq` step per `t.SubQuestions` (Key `"subq:"+id`, Title `子问题 N`, `SubQuestionID=id`) + the five tail fixed steps.
- [ ] **Step 4: Run to verify pass.**
- [ ] **Step 5: Commit** `feat(agent): DeriveProposalSteps — dynamic proposal guide-step template`.

## Task 2: `WritingTrack` + `CounterpointsWaived` on `StudioState`

**Files:**
- Modify: `apps/api/internal/agent/studiostate.go`
- Test: `apps/api/internal/agent/studiostate_test.go` (create if absent)

**Interfaces:**
- Produces: `StudioState.ProposalTrack *WritingTrack json:"proposalTrack,omitempty"`; `StudioState.CounterpointsWaived bool json:"counterpointsWaived,omitempty"`.

- [ ] **Step 1: Failing test** — marshal a `StudioState` with a `ProposalTrack{Mode:ModeGuided, StepIndex:2}` and `CounterpointsWaived:true`, unmarshal, assert round-trip; assert `DefaultStudioState().ProposalTrack == nil` and `CounterpointsWaived == false`.
- [ ] **Step 2: Run → FAIL.**
- [ ] **Step 3: Implement** — add the two fields (pointer track so absence is distinguishable; the `omitempty` keeps old rows clean). Leave `DefaultStudioState` unchanged (nil track).
- [ ] **Step 4: Run → PASS.**
- [ ] **Step 5: Commit** `feat(agent): StudioState carries proposalTrack + counterpointsWaived`.

## Task 3: Contracts — `proposalGuide.ts`, `questionCard.ts`, `studioState.ts`

**Files:**
- Create: `packages/contracts/src/proposalGuide.ts`, `packages/contracts/src/questionCard.ts`
- Modify: `packages/contracts/src/studioState.ts`, `packages/contracts/src/index.ts`
- Test: `packages/contracts/test/proposalGuide.test.ts`

**Interfaces (produces):**
```ts
// proposalGuide.ts
export const SubQuestion = z.object({ id: z.string(), text: z.string() });
export const GuideCard = z.object({ prompt: z.string(), example: z.string(), refHint: z.string().optional() });
export const StepKind = z.enum(["fixed", "subq-define", "subq"]);
export const ProposalGuideStep = z.object({
  key: z.string(), title: z.string(), kind: StepKind,
  index: z.number().int(), total: z.number().int(),
  mode: z.enum(["", "free", "guided"]), started: z.boolean(),
  subQuestions: z.array(SubQuestion),
  card: GuideCard.nullable(),            // null until guided+started (or a define step)
});
// questionCard.ts
export const QuestionCardTurnReply = z.object({
  narrate: z.string(), suggestedObjective: z.string().nullable(), done: z.boolean(),
});
```
`studioState.ts` gains optional `proposalTrack` (a lean read schema: `{ mode, stepIndex, started }`) + `counterpointsWaived: z.boolean().optional()` on the StudioState read shape.

- [ ] **Step 1: Failing test** — a `ProposalGuideStep.parse` of a guided `subq` step with a card parses; one with `card: null` parses; a `QuestionCardTurnReply` with `suggestedObjective: null, done:false` parses.
- [ ] **Step 2: Run** `pnpm --filter @mind-imprint/contracts test -- proposalGuide` → FAIL.
- [ ] **Step 3: Implement** the schemas; export from `index.ts`.
- [ ] **Step 4: Run** the contracts suite (`pnpm --filter @mind-imprint/contracts test`) → PASS (also confirms no existing test broke).
- [ ] **Step 5: Commit** `feat(contracts): proposal guide-step + question-card + studioState track`.

## Task 4: `GenerateProposalGuideStep` (fast model)

**Files:**
- Create: `apps/api/internal/agent/proposal_guide.go`, `apps/api/internal/agent/proposal_guide_test.go`

**Interfaces:**
- Produces: `type GuideCardOut struct{ Prompt string; Example string; RefHint string }`; `type GuideGenInput struct{ Title string; Objective, Reason, Activities, Resources string; Step Step; SiblingSubQuestions []SubQuestion; ThisSubQuestion string }`; `func GenerateProposalGuideStep(ctx, prov gateway.Provider, resolved gateway.Resolved, in GuideGenInput) (GuideCardOut, gateway.ChatUsage, error)`.

- [ ] **Step 1: Failing test** — a stub provider (mirror `framework_review_test.go`'s stub) returning `{"prompt":"...","example":"English demo","refHint":"framework 目标"}` yields the parsed card; a fenced ```json``` reply still parses; an empty `prompt` → error (drives caller degrade). Assert the *system prompt varies by step kind*: build the request for a `KindSubq` step and assert (via the stub capturing the user message) that `ThisSubQuestion` + siblings appear in the prompt.
- [ ] **Step 2: Run → FAIL.**
- [ ] **Step 3: Implement.** System prompt: a fast IB coach; produce a guiding question (Chinese, applied to THIS prompt) + one English example for a DIFFERENT prompt (never the answer here — 铁律①). Branch the user-message context on `in.Step.Kind`: fixed → framework dims + step skeleton; `KindSubqDefine` → teach decomposing into 2–4 researchable/correlated/constitutive sub-questions (English example decomposition); `KindSubq` → the key RQ + `ThisSubQuestion` + siblings, guiding the several parts (resolves what / relation to central / current view + support / advances next). `gateway.Collect` + 2-attempt retry + defensive parse (`extractJSONObject(stripFences(...))`, require non-empty `prompt`). `MaxTokens: 1200`.
- [ ] **Step 4: Run** `go test ./internal/agent/ -run TestGenerateProposalGuideStep` → PASS.
- [ ] **Step 5: Commit** `feat(agent): GenerateProposalGuideStep — step-kind-aware guide cards`.

## Task 5: `ReviewProposalPart` (flagship reasoning)

**Files:**
- Create: `apps/api/internal/agent/proposal_part_review.go`, `apps/api/internal/agent/proposal_part_review_test.go`

**Interfaces:**
- Produces: reuse `FrameworkVerdict` (`{Ready,Why,Suggestions}`). `type ProposalPartReviewInput struct{ Title, StepTitle, StepPrompt, StudentText string }`; `func ReviewProposalPart(ctx, prov gateway.Provider, resolved gateway.Resolved, in ProposalPartReviewInput) (FrameworkVerdict, gateway.ChatUsage, error)`.

- [ ] **Step 1: Failing test** — stub returns `{"ready":true,"why":"扎实","suggestions":["更具体一点"]}` → parsed; garbage → error. (Mirror `framework_review_test.go`.)
- [ ] **Step 2: Run → FAIL.**
- [ ] **Step 3: Implement** by mirroring `ReviewFramework`: system prompt = a rigorous-yet-encouraging IB tutor reviewing ONE proposal part against its guiding question; return `{ready,why,suggestions[≤4]}`; **never rewrite the student's text** (铁律①). Reuse `parseFrameworkVerdict`. `MaxTokens: 3000`.
- [ ] **Step 4: Run → PASS.**
- [ ] **Step 5: Commit** `feat(agent): ReviewProposalPart — flagship per-part reviewer`.

## Task 6: `proposal-track` REST (GET / mode / start / subquestions / advance)

**Files:**
- Create: `apps/api/internal/api/proposal_track.go`
- Modify: the route table (where `getStudioState` etc. are registered — search `proposal-track`-adjacent `mux.HandleFunc`/`r.Handle`)
- Test: `apps/api/internal/api/proposal_track_test.go`

**Interfaces:**
- `GET /api/v1/projects/{id}/proposal-track` → `ProposalGuideStep` JSON. Loads `StudioState`, initializes `ProposalTrack` if nil, derives steps, clamps `StepIndex`. When `guided && started`: for the current step, read `StepGuides[key]`; if absent, call `GenerateProposalGuideStep` (fast resolver, fallback ChatResolver), cache into `StepGuides[key]`, persist state, meter `purpose="proposal_guide"`. For a `KindSubqDefine` step, still generate the decomposition-guidance card. Returns `card:null` when `mode!=guided || !started`.
- `POST …/proposal-track/mode {mode}` → set `Mode`; if guided, `Started=false`; persist; return the step.
- `POST …/proposal-track/start` → `Started=true`, `StepIndex=0`; persist.
- `POST …/proposal-track/subquestions {subQuestions:[{id?,text}]}` → normalize (mint ids for new via `uuid`, keep existing ids, drop removed → delete their `StepGuides["subq:"+id]`), clamp to 2–4 kept (soft: store what's sent, but ignore empties), persist, return the (re-derived) step.
- `POST …/proposal-track/advance {dir}` → move `StepIndex` over the derived list, clamp `0..len-1`; persist; return the step.

- [ ] **Step 1: Failing test** (foreground testcontainer; seed a project via the suite's helper). Assert: fresh GET returns `mode:""`; POST mode=guided then start → `started:true, index:0`; POST subquestions with 3 items → GET `total` jumps by 3 and `subQuestions` length 3; advance past the define step lands on a `subq` step; the guide card is generated once (a second GET on the same step does not re-spend — assert via a call-counting stub `Provider`). Use a stub `FastChatResolver`/`Provider` returning a fixed guide card.
- [ ] **Step 2: Run** `go test ./internal/api/ -run TestProposalTrack` (foreground) → FAIL.
- [ ] **Step 3: Implement** the handlers + register routes. Reuse `loadOwnedProject`, `GetStudioState`/`SetStudioState`, the `docKindParam`-style helpers. Guard model calls behind resolver-nil (card stays null on failure — never break the read).
- [ ] **Step 4: Run → PASS.**
- [ ] **Step 5: Commit** `feat(api): proposal-track endpoints (guide-step track)`.

## Task 7: `QuestionCardTurn` sub-agent (fast model, §2 script)

**Files:**
- Create: `apps/api/internal/agent/question_card.go`, `apps/api/internal/agent/question_card_test.go`

**Interfaces:**
- Produces: `type QuestionCardOut struct{ Narrate string; SuggestedObjective string; Done bool }`; `type QuestionCardInput struct{ Title, Objective string; History []ChatTurn }`; `func QuestionCardTurn(ctx, prov gateway.Provider, resolved gateway.Resolved, in QuestionCardInput) (QuestionCardOut, gateway.ChatUsage, error)`.

- [ ] **Step 1: Failing test** — stub returns `{"narrate":"用你自己的话说说你对题目的理解？","suggestedObjective":"","done":false}` mid-conversation → parsed, `Done:false`; a final `{"narrate":"很好，这就是你的研究问题","suggestedObjective":"在 X 条件下 Y 是否 Z","done":true}` → `Done:true` + objective set. Fenced JSON parses; empty narrate → error.
- [ ] **Step 2: Run → FAIL.**
- [ ] **Step 3: Implement.** System prompt = the §2 script verbatim-intent (see spec Pillar 4): if target unrelated/too general, explain why to narrow + what a good objective is; decompose the prompt ("用你自己的话说说你对题目的理解"); ask for a concrete connected experience; ask their understanding of it; guide to a detailed RQ; one question per turn (铁律③); 铁律① — never write the RQ for them, only echo their wording as `suggestedObjective` when `done`. Include `History` as prior messages. `gateway.Collect` + retry + parse (require non-empty `narrate`). `MaxTokens: 1500`.
- [ ] **Step 4: Run → PASS.**
- [ ] **Step 5: Commit** `feat(agent): QuestionCardTurn — adaptive 提问卡 sub-agent`.

## Task 8: question-card turn + commit endpoints + trigger-authority gate

**Files:**
- Create: `apps/api/internal/api/question_card.go`
- Modify: `apps/api/internal/agent/studioflow.go` (`IsSummonable` gate), `apps/api/internal/api/coach.go` (summon gating already routes through `IsSummonable`/`cardEligibleForSummon` — thread objective-empty), the route table
- Test: `apps/api/internal/api/question_card_test.go`; `apps/api/internal/agent/studioflow_test.go` (gate)

**Interfaces:**
- `POST /api/v1/projects/{id}/cards/question-card/turn {messages}` → `QuestionCardTurnReply` JSON. Loads title + `proposal.objective`; runs `QuestionCardTurn` (fast resolver); meters `purpose="question_card"`; persists the turn to a `question_card` coach surface (reuse `AppendProjectCoachMessage(scope:"question_card")`, mirroring `postCoachSubagentTurn`). Best-effort fallback narrate on error.
- `POST /api/v1/projects/{id}/cards/question-card/commit {objective}` → writes `proposal.objective` (reuse the note-confirm/objective write path), records card usage (a `card_instance`/envelope so it lands in the process tree — reuse the existing card-instance persistence), returns 200. Rejects empty objective (422).
- **Gate:** `agent.QuestionCardAISummonable(objectiveEmpty bool) bool` → `false` when objective empty (student-manual only), `true` otherwise. Thread it: in the coach summon path, when `card_id=="question-card"` and `proposal.objective` is empty, drop the summon (the coach can't propose it). Keep the manual-open path (Task 17) always available when objective empty.

- [ ] **Step 1: Failing tests** — (a) turn endpoint returns a parsed reply from a stub; (b) commit with a non-empty objective writes it (GET project shows the new objective) + records a card usage row; commit empty → 422; (c) gate: with objective empty, a coach summon of `question-card` is dropped (assert no `coach_proposed` event); with objective set, it is offered.
- [ ] **Step 2: Run** `go test ./internal/api/ -run 'TestQuestionCard'` + `go test ./internal/agent/ -run TestQuestionCardAISummonable` → FAIL.
- [ ] **Step 3: Implement** endpoints + gate + routes.
- [ ] **Step 4: Run → PASS.**
- [ ] **Step 5: Commit** `feat(api): question-card sub-agent turn/commit + trigger-authority gate`.

## Task 9: Finding 2 — 反例 prompt before plan-gen

**Files:**
- Modify: `apps/api/internal/api/coach.go` (`reconcileStudioFunnel`), `apps/api/internal/api/projects.go` (add `frameworkReadyForPlan`), `apps/api/internal/agent/studioflow.go` (`FlowFramework` prompt), create `apps/api/internal/api/framework_waive.go`
- Test: `apps/api/internal/api/coach_capabilities_test.go` (extend the funnel test)

**Interfaces:**
- Produces: `func frameworkReadyForPlan(p sqlc.ProjectProposal, waived bool) bool` = `allRequiredDims(p) && (strings.TrimSpace(p.Counterpoints) != "" || waived)`. `POST /api/v1/projects/{id}/framework/waive-counterpoints` → sets `StudioState.CounterpointsWaived=true`, persists, 200.

- [ ] **Step 1: Failing test** — extend the funnel test: fill the 4 required dims with counterpoints EMPTY and `CounterpointsWaived=false` → plan is NOT generated (assert `ListPlanItems` empty). Then POST waive-counterpoints → next funnel run (a coach turn) generates the plan. Separately: fill 4 dims + a non-empty counterpoints → plan generates without waiving.
- [ ] **Step 2: Run** `go test ./internal/api/ -run 'TestFunnel|TestFramework'` (foreground) → FAIL.
- [ ] **Step 3: Implement** — swap `allRequiredDims(prop)` → `frameworkReadyForPlan(prop, state.CounterpointsWaived)` in `reconcileStudioFunnel`; update the `FlowFramework` `SystemPrompt` to guide all five (add 可能的反例·张力: "这条也要问到，别跳过") and to say the plan generates once the four core dims are set and counterexamples have been discussed; add the waive endpoint + route.
- [ ] **Step 4: Run → PASS.**
- [ ] **Step 5: Commit** `feat(coach): prompt for 反例 before plan-gen (non-blocking waive)`.

## Task 10: Web API clients

**Files:**
- Create: `apps/web/src/api/proposalTrack.ts`, `apps/web/src/api/questionCard.ts`
- Modify: `apps/web/src/api/projects.ts` (add `waiveCounterpoints`)
- Test: none new (thin wrappers; covered by hook/component tests)

- [ ] **Step 1:** Implement `getProposalTrack`, `setProposalMode`, `startProposalGuide`, `setSubQuestions`, `advanceProposalStep` (parse with `ProposalGuideStep`), `questionCardTurn`, `commitQuestionCard`, `waiveCounterpoints` — all via `apiFetch`, Zod-parsing responses (fail-loud like `writing.ts`).
- [ ] **Step 2:** `pnpm --filter @mind-imprint/web tsc` (typecheck) clean.
- [ ] **Step 3: Commit** `feat(web): proposal-track + question-card API clients`.

## Task 11: `useProposalTrack` hook

**Files:**
- Create: `apps/web/src/workspace/blocks/useProposalTrack.ts`
- Test: `apps/web/test/workspace/useProposalTrack.test.ts`

**Interfaces:**
- Produces: `useProposalTrack(projectId)` → `{ step, loading, chooseMode(m), start(), saveSubQuestions(list), next(), prev(), reload() }` where `step: ProposalGuideStep | null`.

- [ ] **Step 1: Failing test** — mock the api client; assert `chooseMode("guided")` then `start()` transitions `step.mode`/`started`; `next()` calls advance and reloads.
- [ ] **Step 2: Run → FAIL. Step 3: Implement. Step 4: Run → PASS.**
- [ ] **Step 5: Commit** `feat(web): useProposalTrack hook`.

## Task 12: `ProsePane` toolbar cleanup (word-count + save lower-right)

**Files:**
- Modify: `apps/web/src/workspace/blocks/ProsePane.tsx`
- Test: `apps/web/test/workspace/ProsePane.test.tsx` (create if absent)

- [ ] **Step 1: Failing test** — render `ProsePane`; assert the old top toolbar bar (`border-b … px-8 py-2`) is gone and a lower-right overlay shows `字` count + save label; preview toggle still reachable.
- [ ] **Step 2: Run → FAIL. Step 3: Implement** — remove the top `<div>` toolbar; add an absolutely-positioned lower-right overlay (`absolute bottom-3 right-4`) with word count + save status + an unobtrusive preview toggle. Keep autosave logic intact.
- [ ] **Step 4: Run → PASS. Step 5: Commit** `feat(web): proposal surface — word-count/save moved lower-right (§4)`.

## Task 13: `ProposalGuide` — choice gate + guided outline intro

**Files:**
- Create: `apps/web/src/workspace/blocks/ProposalGuide.tsx`
- Test: `apps/web/test/workspace/ProposalGuide.test.tsx`

- [ ] **Step 1: Failing test** — with `step.mode===""` render a choice card (two buttons 自己写 / 一步步带我); clicking 一步步带我 calls `chooseMode("guided")`. With `mode==="guided" && !started` render the 9-part outline intro + a 开始写作 button that calls `start()`. Pass `bufferNonEmpty` → the choice card highlights/ suggests 自己写.
- [ ] **Step 2: Run → FAIL. Step 3: Implement** the two states. **Step 4: Run → PASS. Step 5: Commit** `feat(web): ProposalGuide — mode choice + outline intro`.

## Task 14: `ProposalGuide` — guide-card scaffold + interim review + wire into WritingBlock

**Files:**
- Modify: `apps/web/src/workspace/blocks/ProposalGuide.tsx`, `apps/web/src/workspace/blocks/WritingBlock.tsx` (render `ProposalGuide` above `ProsePane` when `doc==="proposal"`), `apps/web/src/workspace/WorkspaceContainer.tsx` (reuse `appendFrameworkVerdict`-style bubble for the part review)
- Test: `apps/web/test/workspace/ProposalGuide.test.tsx` (extend)

- [ ] **Step 1: Failing test** — with `mode==="guided" && started` and a `card`, render a colored guide card (title + prompt + English example) with 我依然有问题 / 我写好了 + prev/next. 我依然有问题 pushes the prompt into the coach thread (assert the injected chat message). 我写好了 triggers the part review then `next()` (assert the review call + advance).
- [ ] **Step 2: Run → FAIL. Step 3: Implement** — the guide-card scaffold; 我依然有问题 → `useStudioChat` inject; 我写好了 → call a `reviewProposalPart` client (add to `proposalTrack.ts`) surfaced via the existing verdict-bubble path, then `next()`. Render `ProposalGuide` from `WritingBlock` (proposal doc only), replacing the bare `ProsePane` with `ProposalGuide` (which itself renders `ProsePane` as the writing surface underneath the card).
- [ ] **Step 4: Run → PASS. Step 5: Commit** `feat(web): guided proposal scaffold — guide card + interim review`.

*(Note: the `我写好了` interim review needs a server endpoint. Fold it into Task 6's handler set as `POST …/proposal-track/review {stepKey}` running `ReviewProposalPart` over the step's slice of the buffer via `EvalResolver`, returning a `ReviewVerdict`. Add its failing test to Task 6 and its client to Task 10.)*

## Task 15: Sub-question editor (the `research-plan` define step)

**Files:**
- Modify: `apps/web/src/workspace/blocks/ProposalGuide.tsx`
- Test: `apps/web/test/workspace/ProposalGuide.test.tsx` (extend)

- [ ] **Step 1: Failing test** — with a `kind==="subq-define"` step, render the decomposition guidance card + an editable list of sub-question rows (add/edit/remove, soft 2–4). A "确认子问题" button calls `saveSubQuestions(list)`. A "先去做文献探索" button calls the `open_reading` path. Fewer than 2 rows disables 确认 (soft, with a hint) but never hard-blocks a manual next.
- [ ] **Step 2: Run → FAIL. Step 3: Implement. Step 4: Run → PASS. Step 5: Commit** `feat(web): sub-question decomposition editor (research-plan step)`.

## Task 16: needs-resources box (writing page)

**Files:**
- Modify: `apps/web/src/workspace/blocks/ProposalGuide.tsx` (or a small `NeedsResourcesBox.tsx`)
- Test: `apps/web/test/workspace/NeedsResourcesBox.test.tsx`

- [ ] **Step 1: Failing test** — render the box; typing a note + tapping 去查资料 calls the `open_reading` jump with the note text.
- [ ] **Step 2: Run → FAIL. Step 3: Implement** — a small lower-area box; the jump reuses the existing reading-room open path (the `open_reading` tool / room switch). **Step 4: Run → PASS. Step 5: Commit** `feat(web): needs-resources box on the proposal writing page`.

## Task 17: `QuestionCardModal` + manual-open affordance + card-used chip

**Files:**
- Create: `apps/web/src/studio/QuestionCardModal.tsx`
- Modify: the forming room (`apps/web/src/workspace/blocks/` forming block — search for `ToolForming`/`forming` render) to add the manual-open affordance; `WorkspaceContainer.tsx`/`StudioCardSheet` routing so a summoned `question-card` opens THIS modal (not the form sheet)
- Test: `apps/web/test/studio/QuestionCardModal.test.tsx`

- [ ] **Step 1: Failing test** — render the modal; it runs a turn loop (mock `questionCardTurn`): student message → AI narrate appended; when a reply has `done:true` + `suggestedObjective`, show a confirm-objective control; confirming calls `commitQuestionCard(objective)` and closes; a card-used chip is dropped into the coach thread. Separately: the forming room shows a manual "用提问卡帮我想" affordance ONLY when `objective` is empty.
- [ ] **Step 2: Run → FAIL. Step 3: Implement** — chat modal reusing `ChatLog`/`Composer`; route summoned/manual `question-card` to it; commit → fill 目标 + close + chip. Manual affordance gated on empty objective (mirrors the server gate).
- [ ] **Step 4: Run → PASS. Step 5: Commit** `feat(web): 提问卡 sub-agent modal + manual-open + card-used chip`.

## Task 18: forming-room 反例 skip link

**Files:**
- Modify: the forming room block; `apps/web/src/api/projects.ts` already has `waiveCounterpoints` (Task 10)
- Test: forming-room test (extend or create)

- [ ] **Step 1: Failing test** — when the four dims are filled and counterpoints is empty, an unobtrusive "暂时想不到反例，先生成计划" link is shown; tapping it calls `waiveCounterpoints` then refreshes.
- [ ] **Step 2: Run → FAIL. Step 3: Implement. Step 4: Run → PASS. Step 5: Commit** `feat(web): 反例 skip link in the forming room`.

## Task 19: Full suite + deploy + smoke

- [ ] `pnpm --filter @mind-imprint/contracts test` green; `pnpm --filter @mind-imprint/web tsc` clean + `pnpm --filter @mind-imprint/web test` green.
- [ ] Full `go test ./...` in `apps/api` green (foreground; `TestWeeklyReportForSeededClass` is a known pre-existing flake — note if it appears, don't chase it).
- [ ] Commit any test fixups; push to main.
- [ ] `.deploy-local/deploy.sh full` from repo root → record the deployed SHA.
- [ ] **Live smoke (prod, authenticated as Phoebe, `?trial=1`):** (1) framework — fill 4 dims with counterpoints empty → plan does NOT gen; the coach prompts for 反例; write a 反例 → plan gens. (2) With objective empty, the 提问卡 manual affordance appears; open it, run the sub-agent to a confirmed 目标; confirm it fills the objective. (3) Proposal — choose guided → outline intro → 开始写作 → guide cards; at 研究计划, define 3 sub-questions → 3 subq cards appear, one at a time; 我写好了 surfaces a review bubble; 我依然有问题 pushes into chat. (4) Writing page — word-count/save lower-right; needs-resources box jumps to reading. Record token/cost for one full pass.

**Acceptance:** the proposal guided track derives dynamically per sub-question (one card at a time); free/guided works; guide cards are AI-generated + cached; the 提问卡 runs as an adaptive sub-agent that fills the 目标 and is gated by objective-emptiness; the framework prompts for 反例 before plan-gen with a non-blocking skip; the writing page matches §4 (icons out, word-count/save lower-right, needs-resources box). No behavioral deviation from `all-statuses.md §2/§4` remains (AI批注 tab / select-to-send / comment-first modal explicitly handed to 3b).

## Self-review (spec coverage)

- Pillar 1 (track + dynamic template) → Tasks 1,2,6,11,13,14,15. Pillar 2 (guide-card gen + cache + endpoints) → Tasks 3,4,6,10. Pillar 3 (UI cleanup) → Tasks 12,16 + 13–15. Pillar 4 (提问卡 sub-agent) → Tasks 3,7,8,17. Pillar 5 (反例 prompt) → Tasks 9,18. Interim `我写好了` review → Task 5 + Task 14 note (endpoint folded into Task 6). No placeholders; types consistent (`ProposalGuideStep`/`FrameworkVerdict`/`QuestionCardOut` reused across tasks).
- **Doc check (AGENTS.md rule):** cross-checked against `all-statuses.md §2 + §4` in the spec's "Doc consistency check" — the only §4 items not built (AI批注 tab, select-to-send, comment-first modal, colored anchored 批注) all depend on the 批注 primitive and are explicitly handed to 3b; no behavioral conflict remains.
