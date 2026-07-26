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
> **Coverage now — 13 e2e specs green:** smoke, auth×3, registration, **tenancy**
> (cross-class 404), J1 (new-student), J2 (course completion), J3 (chat +
> assessment), J-studio (project lifecycle → flagship 你的思维印记 → growth),
> **J-teacher** (班级 → 班级周报 live compose → drill-down → 家长报告 live compose),
> **J-admin** (概览 stats → mint invite → CSV import), **J-cohort** (many students
> → per-student attribution + aggregation). Student + teacher + admin + parent +
> tenancy designs all covered.
> **Environment-gated:** 语音 (TTS/ASR) needs Volcano creds — now supplied and
> verified (see round 2 below).
>
> ---
>
> **STATUS 2026-07-26 (round 2 — hardening):** the five remaining follow-ups were
> taken on:
> - **C (chat send-vs-load race): REAL FIX shipped** (pendingLocalThreadRef) +
>   unit regression + live J3 (send into empty surface) + new **J-resume**
>   (mid-interruption: 3 rounds → reload → rehydrate → resume → reload persists).
>   Resume works across a full page reload; the fix does not suppress the reload.
> - **语音 (voice): wired + verified live.** Volcano creds mapped into
>   apps/api/.env.local (APP_ID→VOICE_APP_ID, ACCESS_TOKEN→VOICE_ACCESS_KEY). The
>   app is provisioned for the `seed-tts-2.0` resource; the working speaker is
>   `VOICE_TTS_VOICE=zh_female_vv_uranus_bigtts` (a 音色 activated on the app —
>   the other resources 403 at handshake, and other speakers return upstream
>   `55000000`). New **J-voice** signs in → POST /voice/tts → real MP3
>   (ID3/MPEG, 24kHz); config-gated (skips on 503 so CI without creds passes).
>   ASR (WS + live mic PCM) still needs a fake-audio harness — deferred (see G).
> - **成长报告 populated tabs: covered.** J-studio step 4 now asserts 学习记录 row +
>   工具卡 seeded cards (知识工具) + 能力素养 leaving its empty state but showing
>   「证据不足 · 需更多任务」 (a single finish gives totalSessions=1; each depth dim
>   needs ≥2 sessions for a level — encoded as a precondition, NOT a bug).
> - **设置: covered.** New **J-settings** — surface renders (个人/AI 形象/toggles/
>   退出登录) + avatar picker responds.
> - **J-teacher flake: fixed.** Replaced the coupled network-wait + short assert
>   with a single end-state wait (150s) for the flagship weekly prose.
> - **Full pure-UI S0→S6 studio walk: DONE** (J-walk, passes clean live ~4.4m).
>   Authoring it against the live model surfaced THREE real backend bugs (H, I, J
>   below) — all journey-breaking or core-promise reliability issues invisible to
>   the mocked suite. The walk is the highest-yield spec of the whole effort.
>
> All round-2 specs verified live: J3, J-resume, J-settings, J-studio, J-teacher,
> J-voice, J-walk all PASS against the running stack.

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

## C — Chat first-message send-vs-load race drops the reply  ·  **FIXED (real fix)**

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

**FIX (shipped + verified live, 2026-07-26):** a `pendingLocalThreadRef` in
ChatContainer marks any thread created locally (via 新对话 OR the first send into
an empty surface). The `[activeThreadId]` load-effect skips server hydration for
that thread on its first activation — entries already reflect the truth (empty,
or the optimistic first turn) — and **consumes** the flag, so returning to the
thread later still reloads the persisted transcript (resume preserved). Verified:
J3 now sends directly into the empty surface (workaround dropped) and passes live;
a new J-resume spec does 3 rounds → full reload → transcript rehydrated → 4th
round → reload persists. Unit regression: send into empty surface keeps both
bubbles and never calls getMessages. The `[activeThreadId]` mount-time race the
existing ChatSurface tests documented is unchanged (still handled by `flush`).

---

## G — Voice ASR (speech-to-text) not E2E-covered  ·  **deferred (documented)**

**Surface:** 工作室 dictation mic (CoachRail) + 课程 按住说话 (AskPanel). Both drive
`AsrStream` (WebSocket to `GET /voice/asr`) fed by `MicCapture` (getUserMedia →
AudioWorklet → 16 kHz Int16 PCM frames).

**Why deferred:** driving REAL microphone audio through getUserMedia + AudioWorklet
in headless Chromium requires a fake-audio harness (Chromium
`--use-file-for-fake-audio-capture=<wav>` plus fake-device flags) and a WAV
fixture. The TTS path (J-voice) exercises the same Volcano auth/credential/wiring
stack and the same server config gate, so the voice INFRASTRUCTURE is proven; ASR
adds only the audio-capture + WS-streaming client path on top. Recommended when
picked up: launch Chromium with fake-audio flags, feed a short Mandarin WAV, and
assert a non-empty `final` transcript arrives over the WS. The backend ASR handler
+ protocol already have Go unit coverage (voice package + api handler tests).

---

## H — 信源体检 / 论证体检 rejected 100% with the live model → S3/S4 un-advanceable  ·  **FIXED**

**Surface:** 工作室 S3 信源体检 + S4 论证体检 (`agent.ProposeSpotCheck`).
**Symptom:** every spot-check returned `spot_check_rejected` ("no usable items")
with deepseek-v4-pro, so a real student could never advance past S3 or S4 through
the UI. 100% repro on both a fresh walk and the seeded project.

**Root cause:** `spotCheckPostureSources`/`spotCheckPostureArgument` said "输出
JSON 数组" but never named the per-item fields; the model emitted `{"id",
"comment"}` while `spotCheckItemWire` needs `{target_id, evidence, missing,
fix}`. Empty `TargetID` → every item dropped as an unknown target → 0 usable →
whole order rejected. Same class as Finding E (review.go), which had been
hardened; spotcheck.go had not. A direct call with a field-naming prompt returned
correct output, proving the prompt was the defect.

**FIX (shipped + verified live):** append a shared `spotCheckFieldSpec` naming the
four wire fields (target_id echoed verbatim from the bracketed id) to both
postures. Regression test asserts every posture names the fields. J-walk now
clears S3 + S4. **Present on main until this branch merges** — the existing suite
never caught it because J-studio drives the back-half via API and skips both
spot-checks.

---

## I — review / spot-check item field as a JSON array → whole order rejected ~40%  ·  **FIXED**

**Surface:** 整稿体检 (`ProposeReview`) + the two spot-checks (`ProposeSpotCheck`).
**Symptom:** ~40% of live 整稿体检 calls failed with `review output not JSON:
cannot unmarshal array into Go struct field ...missing of type string` — the
whole review rejected, `whole_draft_review` (the finish gate) unset. With
retries:1 the walk still occasionally lost both attempts.

**Root cause:** `reviewItemWire.Evidence/Missing/Fix` (and the spot-check wire's
same fields) were typed `string`, but deepseek-v4-pro sometimes returns them as a
JSON array of strings. A `[]wire` unmarshal fails entirely on the first such
field. Valid JSON, wrong shape.

**FIX (shipped + verified live):** a `flexString` type (agent/wireflex.go) that
decodes either a string or an array (joined with 「；」) or any scalar; applied to
both wires. Regression tests cover string/array/scalar and both wires tolerating
an array field.

---

## J — flagship report intermittently malformed JSON → finish 422s  ·  **FIXED (hardened)**

**Surface:** the flagship `AssessReport` — shared by project finish (你的思维印记),
course terminal assessment, and chat assessment.
**Symptom:** finish occasionally 422'd with `report output not JSON: invalid
character ':' after array element` — the flagship model emitted syntactically
malformed JSON. Low-rate but real: a student finishes and gets an error instead
of their 思维印记. Affects all three assessment surfaces.

**Root cause:** genuinely malformed model output (not a coercible shape) — an
inherent, occasional flagship generation error with the live model.

**FIX (shipped + verified live):** a bounded retry (`maxAssessAttempts = 2`) in
AssessReport re-calls the model on a PARSE failure only, accumulating usage so
cost tracking stays honest; banned-phrasing rejections and transport errors do
NOT retry. Consistent with the Finding D maxTokens hardening on this same
never-downgraded path. J-walk finish now passes clean.

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
