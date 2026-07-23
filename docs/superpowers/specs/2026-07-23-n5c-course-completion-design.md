# N5c · Course Completion — Design

> Finishes the Course surface (the last keystone-only surface, Slice 12) with
> three self-contained pieces: a **terminal-assessment challenge** (a
> transfer self-check), **session restart**, and **course push-to-talk voice**.
> One clean slice on current main. Part of N5; the rest of N5 is deferred (see §7).

Date: 2026-07-23
Branch: `slice-n5c-course-completion`

---

## 1. Motivation

The Course surface runs end-to-end (演示→引导→独立→回看) but three gaps remain from Slice 12:

1. **No real ending.** The course finishes the instant the student clicks "next" in the last phase (回看), floor = one turn (`course_step.go` `runCourseAdvance` terminal). There is no moment that asks "can you now do this yourself?"
2. **No restart.** Re-entering a course *resumes* it (`getOrCreateSession` upserts on `UNIQUE(user_id, course_id)`); a student who wants to run it again cannot.
3. **Course voice is inert.** Narration-out works (`TeachingTemplate` calls `api/voice.ts`), but AskPanel's 「按住说话，问老师」 is explicitly deferred/inert (`AskPanel.tsx:279`). Voice-*in* doesn't work on the course surface, though the ASR endpoint (`GET /voice/asr`) and the working pattern (`CoachRail`) both exist.

Board-agnostic, report-agnostic, teacher-agnostic — fully buildable now.

---

## 2. Component 1 — Terminal-assessment challenge (transfer self-check)

### 2.1 What it is

A fifth phase, **「练一手」**, after 回看: a *new* claim the student has not seen, "now do it on your own." It tests **transfer** of the method (SIFT/CRAAP) rather than recall. It is a **self-check, always finishable** (the product decision): the student applies the method, sees a reflective reveal, and finishes **by their own choice** — the machine never adjudicates, never marks anything `solid`.

This satisfies DEC-3 through its **"explicit confirmation"** arm, not the "passed challenge" arm — so there is no adjudication engine to build, and it stays entirely **out of** the parked report/assessment redesign.

### 2.2 It reuses what already exists

`ChallengeTemplate.tsx` is **already a local self-check**: its 「提交我的判断」 button flips local state and shows a static encouraging reveal (「很好，你已经开始像个核查者一样思考了……」) — it never calls the server. So the "self-check, always finishable" behavior *is* the challenge's existing nature. Component 1 mostly wires a fifth phase to that template.

### 2.3 Structure — inline in the skill, NO migration

The course skill (`packages/contracts/skills/info-literacy-course.json`) already inlines phase content for step-less phases: `guided` and `reflect` carry an inline `page` (and `guided` an `anchor_material`) rather than a `course_step` row. The terminal challenge follows the same pattern:

- A new phase `challenge` on the skill: `requires: ["reflect"]`, `title: "练一手"`, `steps: []` (step-less), an inline **`challenge`** block (the ChallengeContent shape: `title` / `prompt` / `reason_hint` / `anchors[]`), a `soft_condition`, and an **empty `floor: []`** and empty `gate`.
- The **empty floor** is the honest encoding of "always finishable" (铁律 2): the student can finish whenever they want; engaging with the challenge is offered, never forced. They *see* it because entering the phase renders it.
- The terminal claim is a **fresh** Global-Perspectives-style claim, distinct from the 卫星图/中国变绿 claim used in `guided`/`independent`, so it genuinely tests transfer. Concretely:
  - claim: 「有人在群里转发：『研究证明，多喝咖啡能显著延长寿命。』」
  - dimensions (anchors): 来源可信吗（谁做的研究、发在哪）· 证据够吗（相关还是因果、样本多大）· 有没有被包装（『研究证明』这类措辞在暗示什么）
  - reason_hint: 「你会不会转发？为什么？用你这节课学到的方法说一句。」

### 2.4 The runtime & renderer changes

- **Contract type** (`skills.Contract`, `apps/api/internal/skills/skill.go`) gains an optional `Challenge *ChallengeContent` field (mirroring the existing optional `Page`/`AnchorMaterial`), and the Zod skill schema (`packages/contracts/src/…`) gains the matching optional field so the JSON validates.
- **The finish flow needs NO code change.** `runCourseAdvance`'s terminal fires when `NextPhase(currentPhase)` returns none. With `challenge` appended after `reflect`, `challenge` becomes the last phase; advancing from it (empty floor → always allowed) finishes the course exactly as `reflect` did before. `reflect → challenge` is an ordinary advance.
- **`CoursePlayer.tsx`**: today a step-less phase renders its `page` via `TeachingTemplate`. Add: a step-less phase whose skill contract carries a `challenge` block renders `ChallengeTemplate` from that inline block instead. One branch in the step-less render path; `phaseSteps(challenge) == []` already routes there.
- **AskPanel / stepper**: the new phase carries its own `ask_chips`; no special-casing needed.

### 2.5 What it does NOT do (MVP boundary)

- **No server capture of the transfer answer.** The self-check is local (as `ChallengeTemplate` already is), matching the "self-check, not an assessment" decision. The process record still captures that the student *reached and completed* the challenge phase (the phase's course events + `course_finished`). Persisting the answer for the (repositioned) report is a deliberate deferral — it lands with that report design, keeping this slice out of the parked area.
- **No new endpoint, no migration, no LLM call** for Component 1.

---

## 3. Component 2 — Session restart

### 3.1 What it is

A student-initiated **「重新开始」** that runs the course again from scratch, instead of only resuming it.

### 3.2 Mechanism — delete the session, cascade wipes the run

`course_session` has `UNIQUE(user_id, course_id)`, and the session-scoped tables (`material`, `card_instances`, `event`, `evaluations`) carry `session_id … ON DELETE CASCADE` (migrations 0023/0024). So a clean restart is: **delete the caller's `course_session` row** → the cascade removes the prior run's materials/cards/events/evaluations → the next `getOrCreateSession` mints a fresh session at the first phase.

- **Store:** a `DeleteCourseSessionByUserCourse(user_id, course_id)` query (sqlc). Deleting by the unique pair is idempotent (0 or 1 row).
- **Endpoint:** `POST /api/v1/courses/{id}/session/restart` — resolves the caller's session (the existing owner-scoped `loadOwnedSession` pattern, hidden-as-not-found), deletes it, and returns the fresh session (re-create, so the client gets a ready state in one call).
- **Web:** a 「重新开始」 control on the finished screen (`CourseReport`/the player's finished state). 铁律 2: student-triggered, never pushed; a plain control, no celebration.

### 3.3 What it does NOT do

No migration (columns/cascades already exist). No cross-user reach (owner-scoped like every other course endpoint). Restart is a full reset by design — a "resume vs restart" chooser is out of scope (resume is already the default on re-entry).

---

## 4. Component 3 — Course push-to-talk voice

### 4.1 What it is

Wire AskPanel's inert 「按住说话，问老师」 to voice-in, so a student can *speak* their question to the course coach — the same spoken-in affordance `CoachRail` already has in the Studio.

### 4.2 Mechanism — reuse the existing voice client + pattern

- The ASR endpoint (`GET /api/v1/voice/asr`, a WebSocket) and the web client (`apps/web/src/api/voice.ts`) already exist and are used by `CoachRail`. AskPanel reuses them verbatim.
- Push-to-talk: hold the button → open the ASR stream → transcribed text fills the ask input → release → the student reviews and sends via the existing `handleAsk` (so a mis-transcription is never auto-sent — the student confirms, honoring 铁律 2/克制).
- Replace the deferred-note block (`AskPanel.tsx:279–287`) with the wired control, mirroring `CoachRail`'s handler shape exactly (a new matrix/host is never introduced — the voice client is shared).

### 4.3 What it does NOT do

- **No new server work** — the TTS/ASR proxy + endpoints are already on main.
- The old live-probe (a valid seed-tts-2.0 speaker id / mp3-vs-pcm) governs **audio playback**, not ASR text-in, so it does **not** block this. If real audio capture needs a device permission or a probe, that surfaces at test time; the wiring itself is client-reuse.
- Multimodal input (image/file) stays **deferred** (non-MVP, §7).

---

## 5. Data flow

```
Terminal challenge:
  student advances through 演示→引导→独立→回看 (unchanged)
  → reflect advance → phase = challenge (练一手)
  → CoursePlayer renders ChallengeTemplate from skill.contracts.challenge.challenge (inline)
  → student does the self-check (local), sees the reveal
  → student presses next → runCourseAdvance: NextPhase(challenge)=none → session finished + course_finished event

Restart:
  student presses 重新开始 → POST /courses/{id}/session/restart
  → delete course_session (cascade wipes session-scoped material/cards/events/evaluations)
  → re-create fresh session (phase = first) → client shows a clean course

Course voice:
  hold 按住说话 → GET /voice/asr (WebSocket, existing) → transcript → ask input
  → release → student sends via handleAsk (existing courseAsk turn)
```

---

## 6. Testing

**Go (full packages, `-p 1`, never `-run` subsets — course runtime + skill change):**
- Skill loads with the new `challenge` phase; `NextPhase(reflect) == challenge`, `NextPhase(challenge)` = none.
- A course walk (funnel-style, the `course_session_test.go` harness) advances 演示→…→回看→练一手 and the final advance finishes the session (`status=finished`, `course_finished` event) — the terminal moved one phase later with no finish-code change.
- Restart: `POST …/session/restart` deletes the session; a session-scoped material/card/event created before restart is gone after; re-entry is a fresh session at the first phase.
- Restart is owner-scoped (another user's session is hidden-as-not-found).

**Web (`npm test` + `npx tsc --noEmit`):**
- `CoursePlayer` renders `ChallengeTemplate` (not `TeachingTemplate`) for a step-less phase carrying a `challenge` block; the terminal claim text renders.
- The 重新开始 control calls `api` restart and resets to a fresh session.
- AskPanel's 按住说话 invokes the voice client (mocked) and fills the input; sending goes through `handleAsk`; a mis-transcription is not auto-sent.

**Contracts (`packages/contracts`, tests in `test/`):**
- `info-literacy-course.json` validates with the new `challenge` phase + inline `challenge` block; canonical↔mirror byte-identical (`make sync-skills`).
- The skill Zod schema accepts the optional `challenge` field.

---

## 7. Deferred (recorded, not in this slice)

- **Multi-course authoring** — the course *format* isn't decided; building the catalog/selection mechanism would be blind. Revisit when a second course is ready to author.
- **Chat→project / course→project seeding** — the "when does a student carry into a project" product question is unresolved; not MVP.
- **Chat multimodal input** — non-MVP.
- **Capturing the terminal-challenge answer for the report** — lands with the repositioned (AI-usage + critical-thinking) report design, which parks with the teacher end.

---

## 8. What this slice is NOT

- **No migration** (session cascades + course content-in-skill already exist).
- **No new LLM call** (the challenge is local; restart and voice add none).
- **No adjudication / no `solid` from a machine** (DEC-3 honored via explicit confirmation).
- **No skill/gate DAG change beyond appending one phase** (the finish code is untouched).
- **No new voice/ASR server work** (endpoints already on main).
