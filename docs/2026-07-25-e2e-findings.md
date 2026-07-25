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
