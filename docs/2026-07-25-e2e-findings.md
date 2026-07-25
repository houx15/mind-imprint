# E2E Pre-Deploy Findings Log

Live findings from running the pre-deploy E2E suite against the real stack
(web → Go API → throwaway Postgres → **real DeepSeek**). Ordered by severity.
These are what the live suite exists to surface — most were invisible to the
Go/vitest suites because those mock the model.

---

## A — Course coach never advances phases with the live model  ·  **blocking (course completion)**

**Surface:** 课程 (course runtime), phase `演示` (demonstrate).
**Symptom:** the course cannot progress past the first phase. Across multiple
runs and **20+ turns** of strong, on-topic student engagement, the session
stayed `demonstrate/active` and `完成课程` was never reached.

**Evidence:**
- `course_progress.completed_ordinals = {0,1}` ⇒ the structural floor
  (`steps_viewed [0,1]`) is met.
- The `soft_condition` ("学生能说出「随手一信」的风险，或提出了一个真实的疑问")
  is met — the simulated student articulated the risk *and* asked genuine
  questions *and* explicitly said "我准备好继续下一部分了".
- Both `POST /session/advance` and `POST /session/ask` fire (10 each in one run);
  the advance endpoint IS being hit.
- `runCourseAdvance` (agent/course_step.go:386–423): after the floor passes, it
  calls the live coach and only moves the phase when the model emits
  `{"type":"advance","to":"guided"}`. The model kept returning `reply`
  (teaching turns), never the advance verb.

**Why the Go tests missed it:** `course_step_test.go` drives a fake model that
emits `advance`. Mock-infidelity — the live model behaves differently.

**Impact:** a real student is stuck in phase 1. Course completion (and anything
downstream that needs a finished course: growth-report course entry, teacher
course signals) is unreachable in production.

**Suggested investigation:** the advance prompt (agent/course_coach.go:29–31,
67–69) asks the model to judge "完成条件是否真的达成". The live model is far more
conservative than the floor/soft_condition imply. Options: lower the advance
threshold, make the floor authoritative (advance when floor met unless the model
actively objects), or give the model an explicit "the student signalled ready +
floor met ⇒ advance" rule.

---

## B — Intermittent empty/truncated coach output silently drops turns  ·  **important**

**Surface:** 课程 coach (`RunCourseStep` ask + advance).
**Symptom:** 7× in one session the coach output came back unparseable —
`course coach: parse output: unexpected end of JSON input` — and the turn was
silently dropped (WARN "output rejected — staying silent", no assistant message
saved). The student sends and gets nothing back.

**Evidence:** in one session, `course_message` had 6 student rows but only 2
assistant rows — the missing replies are the dropped turns.

**Likely cause:** empty model completion (see the known "reasoning models need a
large maxTokens or content is empty" note). The structured-output parse then
fails on empty content.

**Impact:** silent dead turns in the course; compounds Finding A (fewer usable
coach turns → even less chance to advance).

**Suggested investigation:** raise maxTokens for the course coach call and/or
treat empty content as a retry rather than a silent drop; surface a soft "让我
再想想" to the student instead of nothing.

---

## D — Flagship assessment returns empty output → 422 (核心功能)  ·  **blocking (assessment)**

**Surface:** 聊天 assessment (`生成本次对话的思维印记`); almost certainly ALL flagship
assessments (project 你的思维印记, course terminal assessment) share this path.
**Symptom:** `POST /chat/threads/{id}/assessment` spends ~18s of flagship model
time then returns **422 `assessment_rejected`**. The report never generates.

**Root cause (from log):**
`generate_chat_assessment: rejected — agent: report output not JSON: unexpected
end of JSON input`. The flagship model returned **empty/truncated content**, so
the report JSON parse failed. This is the SAME failure mode as Finding B (empty
completions), now on the flagship assessment path — consistent across retries.

**Why it matters most:** the flagship 过程评估 is the product's core promise
(「过程即数据」). If it 422s with the live model, the headline deliverable — the
思维印记 rubric + narrative — does not render in production. The Go tests use a
mock returning well-formed JSON, so this was invisible.

**Likely cause + fix:** the known "reasoning model → all budget to reasoning,
content empty unless maxTokens is large enough" issue (see the
llm-reasoning-model-budgets note). Raise maxTokens for the assessment call,
and/or retry-on-empty, and/or fall back gracefully instead of 422. **Verify
whether the project evaluation (你的思维印记) hits the same 422** — if so this is a
launch blocker.

---

## C — Chat first-message send-vs-load race drops the reply  ·  **important**

**Surface:** 聊天 (ChatContainer), the very first message of a new thread.
**Symptom:** after sending the first message in a fresh chat, the assistant reply
does not render and `生成本次对话的思维印记` stays disabled — even though the coach
reply IS saved server-side (verified in `chat_message`).

**Root cause (confirmed in code):** `handleSend` creates the thread and sets
`activeThreadId` (ChatContainer.tsx:74–77). That state change fires the
load-messages effect (39–53), which `getMessages()` → `setEntries(...)` and
**overwrites the optimistic + streaming entries** the same `handleSend` is
appending (85, 90). The empty/partial server read wins the race, so the streamed
reply is discarded client-side. Subsequent sends in an already-active thread
don't hit it (no `activeThreadId` change). This is the pre-existing
"ChatContainer mount-effect send-vs-load race" carry-forward.

**Impact:** the first turn of every new conversation looks broken (no reply);
the report can't be generated until a second message or a reload.

**Workaround in the E2E (J3):** create the thread first via `新对话`, then send —
so the send doesn't change `activeThreadId` and the effect doesn't race.

**Suggested fix:** guard the load effect so it doesn't clobber in-flight optimistic
entries (e.g. skip the fetch for a thread just created locally, or merge instead
of replace), or create the thread eagerly before the first send.

---

## Plan corrections (real-product truths the initial plan assumed wrong)

- **成长报告 is populated by evaluations only.** `getGrowthHistory`
  (`ListGrowthHistory`) returns reports from the `evaluations` table. An
  in-progress project/course/chat does **not** appear in 学习记录 until it is
  finished + assessed. (Plan J1 step 7 was wrong; corrected in the shipped J1.)
- **Studio card-summon is station + material gated.** A brand-new project fresh
  from the funnel is not at the materials station with an ingested source, so a
  chat nudge alone does **not** summon a CRAAP card. Card-summon journeys must
  drive the materials flow first. (Old golden-path assumed a chat nudge summons.)

---

## Verified working (live)

- **J1 — new-student journey:** register → empty states (工作室 + all 成长报告 tabs)
  → create paper → live coach reply (POST /turn 200) → paper listed in the
  工作室 directory. GREEN.
- **Course runtime mechanics:** list, open, session start, step render, local
  paging, `问印记` ask round-trip, and `advance` (floor gate) all fire correctly.
  Only full *completion* is blocked by Finding A.
- **LLM student-simulator harness:** produces concrete, on-topic replies that
  genuinely engage the coach (occasional DeepSeek hiccup falls back safely).
