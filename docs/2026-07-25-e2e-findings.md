# E2E Pre-Deploy Findings Log

Live findings from running the pre-deploy E2E suite against the real stack
(web → Go API → throwaway Postgres → **real DeepSeek**). Ordered by severity.
These are what the live suite exists to surface — most were invisible to the
Go/vitest suites because those mock the model.

---

> **STATUS 2026-07-25:** A, B, D all **FIXED** (see per-finding notes). C worked
> around in E2E; a real fix is still open. Root cause of B/D was one bug: the
> gateway default `max_tokens` of 1024 truncated large outputs; A was the coach
> being used as a redundant second gate over the structural floor.
>
> **Full live E2E suite is GREEN (9/9):** smoke, auth×3, registration, J1
> (new-student), J2 (course completion), J3 (chat + assessment), **J-studio
> (project lifecycle → commit → 整稿体检 → finish → flagship 你的思维印记 → growth
> populated)**. The composer path (class weekly / parent prose) was verified live
> too (`POST weekly-report/prose` → 200 with real prose).
>
> **Student-side coverage now:** onboarding, 工作室 project lifecycle + flagship
> eval, 课程 completion, 聊天 + assessment, 成长报告 (empty + populated 学习记录).
> **Remaining student-side gaps (future, not blockers):** a pure-UI S0→S6 studio
> walk (the UI enforces station-locking; J-studio drives the back half via the
> real API + UI-verifies growth); 语音 (TTS/ASR); 设置; 成长报告 工具卡/能力素养
> populated tabs. **Teacher/admin/parent + tenancy journeys** also remain (the
> report composer is live-verified, but no UI journey yet).
> Non-blocking hardening: retry-on-empty for the coach ask path (B); a real fix
> for the chat send-vs-load race (C).

## A — Course coach never advances phases with the live model  ·  **FIXED**

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

**FIX (shipped):** made advance **floor-authoritative** in `runCourseAdvance`
(course_step.go). Once the structural floor passes and the student clicks 继续,
the runtime advances to the single computed successor (`NextPhase`) — it no
longer asks the model to re-judge "完成条件" or trusts the model's target phase.
The coach call now only writes a warm transition line (best-effort; if it fails,
the advance still happens). The advance prompt (course_coach.go) was reframed
from a gate to a transition-message request. Verified live: the seed course now
walks 演示→引导→独立→回看→练一手 to `finished`. Forward-only is now enforced
deterministically (stronger than the old model-trusting check).

---

## B — Intermittent empty/truncated coach output silently drops turns  ·  **FIXED (mitigated)**

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

**FIX (shipped):** the gateway default `max_tokens` was raised 1024 → 8000
(deepseek.go + anthropic.go), removing the truncation source shared with D. And
on the advance path, an unavailable transition line no longer blocks (advance is
floor-authoritative now — Finding A fix). A dedicated retry-on-empty for the
coach ask path is still worth adding but is no longer journey-breaking.

---

## D — Flagship assessment returns empty output → 422 (核心功能)  ·  **FIXED**

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

**CONFIRMED blast radius:** `agent.Assess` (assess_report.go — the exact function
that threw "report output not JSON") is shared by all three assessment callers:
`assessment.go` (project 你的思维印记), `course_assessment.go` (course terminal),
and `chat_assessment.go` (chat, observed 422 live). So the project evaluation and
course assessment WILL hit the same empty-output 422. **This is a launch blocker
for the entire 过程评估 feature — the product's headline promise does not render
with the live model.** The class-weekly / parent-report prose composers are
separate code but the same empty-completion class, so they are also at risk.

**FIX (shipped + verified live):** `AssessReport` set no MaxTokens → the gateway
default of **1024** truncated the large report JSON → parse failed → 422 every
time. Set `MaxTokens: 8000` on the assess call AND raised the gateway default
1024 → 8000 for both providers (deepseek.go + anthropic.go), which also covers
the weekly/parent composers and whole-draft review (same latent truncation).
Verified: J3 now generates the chat 思维印记 live (POST /assessment → 200). Since
`agent.Assess` is shared, project + course assessment are fixed too.

---

## E — 整稿体检 review dropped every item → could never finish → no project eval  ·  **FIXED**

**Surface:** 工作室 whole-draft review (`整稿体检`, `ProposeReview`) → the finish gate
→ the project 你的思维印记 evaluation.
**Symptom:** `POST /projects/{id}/snapshots/{sid}/review` returned
`review_rejected` ("agent: review produced no usable items") on real, varied,
cited drafts. Because the review sets the `whole_draft_review` gate item, a
rejected review left it unset → `finish` 422 `gate_not_met` → the project
**你的思维印记 was unreachable through the normal flow.**

**Root cause:** `ProposeReview` matched the model's `criterion_code` against the
skill's codes (`表D/表E/表F/表H`) by EXACT string. The live model emits close-but-
not-identical codes ("表D4", "表 D", or the criterion name), so every item was
skipped → 0 usable → whole review rejected.

**FIX (shipped + verified live):** tolerant code resolution in `ProposeReview`
(review.go) — exact → normalized (spaces/case) → prefix (表D4⇄表D) → by-name;
truly-unmatched codes are dropped with a WARN instead of silently sinking the
review. **Verified live end-to-end:** on the seeded S4 project, order review →
200 (items parse), then `POST /finish` → **200 with the full 你的思维印记 report**
(depthAxis + narrative present, 6.3 KB). This is the flagship 过程评估 proven on
the real project path — confirming the Finding D maxTokens fix on `agent.Assess`
works for project + course + chat.

---

## F — 家长报告 returned `advice: null` pre-prose → blank report in the UI  ·  **FIXED**

**Surface:** 教师端 家长报告 (导出家长版·项目报告 / 阶段报告), both E1 (project) and
E2 (stage).
**Symptom:** opening either parent report before its prose is generated rendered
a **blank overlay** — no cover, no stats, no generate button. The teacher could
never generate a parent report (the button that triggers the compose lives
inside the chrome that never rendered).

**Root cause:** the GET DTO left `advice` (and, for E1, `dRows`/`aRows`) as a nil
Go slice when no prose existed yet → marshalled to JSON `null`. The client
contract (`ParentReport`/`ParentStageReport`) requires `advice: z.array(...)`, so
the response failed Zod validation → `data` never set → `ParentReportChrome`
(rendered only when `data` is present) never mounted.

**FIX (shipped + verified live):** default the DTO arrays to non-nil empty slices
(`[]ParentAdviceDTO{}`, etc.) in `parentStageDTO` + `parentReportDTO` so the wire
carries `[]`, not `null`. Verified live: J-teacher now opens the stage report and
composes the parent prose (POST → 200). Found only because the teacher UI journey
was authored — the API shape "looked right" at the top level (mock-infidelity
class of bug).

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
