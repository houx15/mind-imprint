# S5 · Review finalization — AI-interaction retrospective + defense-readiness conversation

> Build spec for **S5** of the multi-agent architecture (`2026-07-29-multi-agent-architecture-design.md` §3 "回顾 splits — the reflective conversation incl. the new 复盘我与 AI 的互动 component", §11). Final slice. Anchored to the approved north-star; own slice spec in the S2/S4 style.
> Grounded in the educational-scientist ideal journey (`docs/2026-07-29-interaction-mock.csv`, phases 5–6): review is **defense-readiness, not polish**; a structured self-audit (claim → evidence → citation → **AI use** → package); one question at a time; "你先自己答，不要让我替你判"; culminating in an **AI-use statement** (used-for / not-used-for) and a synthesized 印记 sentence.

## 0. What exists (verified via code map) — and the decisions

- **The assessor already reads AI-interaction traces** — `PromptLens` (6 prompt lenses, prompt judged 0–5) + `InteractionEvidence` (per-round) + `Rounds` (assess_report.go, assess_input.go). But all **derived from behavior by the assessor**, never from a **student self-report**. There is **no AI-use-statement** anywhere (neither the 5-dim `project_reflection` doc nor the S6 free-text reflection asks "how did you use AI").
- **The objective AI-interaction record already exists as data, uncomposed:** `ListLLMCallsByProject` (llm_usage.sql — Purpose per call) + the event stream (`coach_turn`, `coach_proposed`, `coach_proposal_skipped`, `card_activated/skipped/completed`, `intervention_posted`, `source_opened`, `review_ordered`, `reflection_written`, `project_finished`). Nothing composes it into a retrospective.
- **Assessment generation:** `generateProjectReport` (assessment.go:64), **flagship** `EvalResolver`, `Purpose="assessment"`, one-time at finish. Input built by `buildAssessmentInputFromProject` (:147), rendered by `assessReportUserInput` (assess_report_prompt.go:93). `getAssessment` GET never spends.
- **Review surface:** `ReviewBlock.tsx` renders the 5-dim `project_reflection` doc (none about AI use) + the mirror pane (你的思维印记, first-open-wins flagship prose). **ReviewBlock does NOT call `/coach`** — no reflective conversation pane, though `/coach scope="reflection"` ("回顾") works today (S1 continuous coach).
- **Storage templates:** student-owned doc = `project_reflection` (upsert, updatable); AI prose = `project_mirror_prose` (first-open-wins, flagship `EvalResolver`). S2's takeaway split-hybrid (`GET takeaway-draft` assemble+seed+meter / `POST finalize` persist, no-spend) is the exact shape to clone.

### Decisions (resolved by principle — student-experience first · performance > cost · educational · non-manipulative; 铁律 hold; S2 precedent)

- **D-A · Split-hybrid, 克制 at the type level (clone S2's takeaway).** The AI **ASSEMBLES the objective interaction record** from the process (events + `llm_call` rows) — factual, read-only, un-forgeable. The **student AUTHORS** the two synthesis fields (`used_for`, `not_used_for`); the AI only **seeds a draft**. **The AI never writes the reflection** (mock: "reflection 必须学生自己写"; 铁律 · AI 克制). Enforced structurally: the model-parse target for the draft carries ONLY the two authored fields, never the objective record (which is computed server-side and cannot be model-influenced).
- **D-B · Objective record = DETERMINISTIC assembly, no spend.** Built server-side from `ListLLMCallsByProject` (metered call counts by Purpose) + event counts: coach turns, cards proposed/accepted/dismissed, sources opened — and the **absences that matter** (no essay-ghostwriting event, no score-prediction, red-line refusals honored). This is the honest "how you and AI actually interacted", not the student's memory. No LLM call to compute it.
- **D-C · The seed draft is ONE metered mid-tier call, gated.** `GET /ai-use-draft` assembles the objective record (no spend) and, if there is an interaction record (`HasInteractionRecord`), makes ONE **mid-tier** (`ChatResolver`) call to seed suggested `used_for`/`not_used_for` from the record — `Purpose="ai_use_retrospective"`, gated so an empty record spends nothing, metered only on a completed call (`resolved.Provider != "" && cerr == nil`), never persisted. (Seed only — the student rewrites; mid-tier is right, per S2.)
- **D-D · Storage = student-owned doc, updatable (not first-open-wins).** `project_ai_use(project_id PK, used_for text, not_used_for text, updated_at)` — upsert like `project_reflection`; re-review supersedes. `POST /ai-use` persists the student-authored statement, **no spend**, emits an `ai_use_written` event (过程即数据).
- **D-E · Feed the assessor.** `AssessmentInput` gains an `AIUseStatement` field (the student's `used_for`/`not_used_for` + the objective record digest); wired in `buildAssessmentInputFromProject`, rendered in `assessReportUserInput`. The existing `PromptLens`/responsible-AI-use reading is now grounded in the student's self-report + the objective record. The assessor still judges — the statement is EVIDENCE, not a score.
- **D-F · Defense-readiness conversation = reuse `/coach scope="reflection"`.** No new endpoint or prompt (the `projectCoachPosturePrompt` already forbids concluding/writing the deliverable — that IS defense-readiness posture: guide-don't-answer, one question at a time). S5 adds a **coach pane to `ReviewBlock`** (reusing the coach client, like WritingBlock's rail) so the conversation is reachable in the review room. The S4 proposal chip is NOT wired here (reflection isn't in `coachProposeSurfaces` — out of scope).

## 1. Data (migration 0042)

```sql
CREATE TABLE project_ai_use (
  project_id   uuid PRIMARY KEY REFERENCES project(id) ON DELETE CASCADE,
  used_for     text NOT NULL DEFAULT '',
  not_used_for text NOT NULL DEFAULT '',
  updated_at   timestamptz NOT NULL DEFAULT now()
);
```
Additive; student-owned; upsert. (No AI-assembled record stored — it's recomputed live from events/llm_calls, always current.)

## 2. Backend — the objective record + draft + finalize

### 2a. Objective interaction record (deterministic, `internal/api` + a pure builder)
- `agent.AIUseRecord` (or an api-layer struct) fields: `CoachTurns int`, `CardsProposed int`, `CardsAccepted int`, `CardsDismissed int`, `SourcesOpened int`, `LLMCallsByPurpose map[string]int`, `GhostwroteEssay bool` (always false — we have no such event; asserted, not guessed), `PredictedScore bool` (false). Built by `buildAIUseRecord(events, llmCalls)` — pure, unit-testable. `HasInteractionRecord(rec) bool` = any coach turns / cards / llm calls.
- Data sources: `ListLLMCallsByProject` (llm_usage.sql) + the project's events (reuse the assessment's `eventDigestsFromProject` or a direct event scan).

### 2b. Seed compose (`internal/agent/ai_use.go`)
- `agent.ComposeAIUseSeed(ctx, prov, r, rec AIUseRecordView) (used, notUsed string, usage, error)` — mid-tier; system prompt seeds a *first-person draft* of used-for / not-used-for FROM the objective record, framed as the student's own honest self-audit (never inflating AI's role, never claiming AI did the thinking). Empty record → early return, ZERO provider calls (clone `HasGraphContent`). Model-parse target carries ONLY `{used_for, not_used_for}` — structurally cannot echo/forge the objective record (克制 at type level).

### 2c. `GET /projects/{id}/ai-use-draft`
Ownership-gated; assembles the objective record (no spend); if `HasInteractionRecord`, one metered `ChatResolver` call seeds `used_for`/`not_used_for` (meter `Purpose="ai_use_retrospective"` only on a completed call; empty record → no call, no meter). Returns `{ record: AIUseRecord, draft: {usedFor, notUsedFor} }`. **No persist.** Also merges any already-saved statement (so re-opening shows the student's own text, not a fresh seed — like S2's brief seeding all fields from the persisted row).

### 2d. `POST /projects/{id}/ai-use`
Persists `{usedFor, notUsedFor}` (student-authored) via upsert; **no spend**; emits `ai_use_written` event. `GET /projects/{id}/ai-use` returns the stored statement (or the zero doc), no spend.

### 2e. Feed the assessor
- `AssessmentInput` (assess_input.go) += `AIUse AIUseForAssessment` (`{UsedFor, NotUsedFor string, Record string}` — the student statement + a one-line objective-record digest).
- `buildAssessmentInputFromProject` (assessment.go:147) loads `project_ai_use` + builds the objective record, sets the field.
- `assessReportUserInput` (assess_report_prompt.go:93) renders an "学生的 AI 使用自述 + 客观交互记录" block, so the responsible-AI-use lens is grounded in both. Assessor still judges.

## 3. Contracts
`AIUseStatement {usedFor, notUsedFor}` · `AIUseRecord {coachTurns, cardsProposed, cardsAccepted, cardsDismissed, sourcesOpened, llmCallsByPurpose, ghostwroteEssay, predictedScore}` · `AIUseDraft {record, draft: AIUseStatement}`. camelCase.

## 4. Frontend (`ReviewBlock.tsx`)
- New api client: `getAIUseDraft` / `postAIUse` / `getAIUse`.
- **AI-use retrospective panel** alongside the 5 reflection prompts: renders the **objective record read-only** ("印记陪你走的这一程：N 轮对话 · 提议 M 张卡（你用了 K、跳过 L）· 你打开 P 个来源 · 没有替你写正文、没有替你预测分数") + two editable fields seeded from the draft (`used_for` / `not_used_for`) → 保存 (`postAIUse`). Honest copy: this is the student's own statement; the AI only seeded it.
- **Review coach pane**: a chat rail in ReviewBlock calling `coach(projectId, "reflection", text)` (defense-readiness conversation) — mirrors WritingBlock's rail (history via `getCoachHistory("reflection")`).

## 5. Tests + deploy
- **Go (api, testcontainers):** `buildAIUseRecord` (counts + absences); `ComposeAIUseSeed` empty→zero-calls; `GET /ai-use-draft` (assembles record; seeds only when record non-empty; no-spend on empty; merges saved statement); `POST /ai-use` persists + `ai_use_written` event + no spend; assessment input carries the AI-use statement (assert the rendered prompt / input struct includes it).
- **contracts:** the three types + a guard.
- **web:** retrospective panel renders the objective record + seeded fields + 保存; review coach pane sends a reflection turn.
- Full suites, then deploy (0042 → db v42). **Prod deploy deferred** per S2/S3/S4 precedent unless the user asks.

## 6. Explicitly deferred (not S5)
- Turning the AI-use statement into its own scored axis (it's evidence for the existing lens, not a new axis).
- A standalone AI-use-review *checklist wizard* (the mock's 5-step review order) — the conversation + statement cover it; a rigid wizard would fight the 克制 conversational model.
- Prod deploy of S2+S3+S4+S5 (batch when the user asks).
