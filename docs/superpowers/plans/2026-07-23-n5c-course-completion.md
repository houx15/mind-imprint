# N5c · Course Completion — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax.

**Goal:** Finish the Course surface with a terminal-assessment challenge (transfer self-check), session restart, and course push-to-talk voice.

**Architecture:** The terminal challenge is a new inline `challenge` phase appended to the one course skill (empty floor = always-finishable), rendered by the already-local `ChallengeTemplate`; the finish code is untouched (the terminal fires at the new last phase). Restart deletes the `course_session` (cascade wipes session-scoped rows) and re-creates it. Course voice reuses the existing ASR endpoint + `CoachRail`'s client pattern.

**Tech Stack:** Go (`net/http`, sqlc), React + TypeScript, Zod contracts, skill JSON single-sourced in `packages/contracts/skills/`.

**Spec:** `docs/superpowers/specs/2026-07-23-n5c-course-completion-design.md`

## Global Constraints

- **Skill JSON authored ONLY in `packages/contracts/skills/`**, mirrored to `apps/api/internal/skills/specs/` by `cd apps/api && make sync-skills` (verify the exact target name in `apps/api/Makefile`; there is a `tools/syncskills`). **NEVER hand-edit `specs/`.**
- **NO migration** — session cascades (0023/0024) and content-in-skill already exist. **NO new LLM call.** **NO adjudication / no machine-set `solid`** (DEC-3 via explicit confirmation).
- **NO gate/skill DAG change beyond appending one phase.** The `runCourseAdvance` finish code must not change.
- **sqlc:** `cd apps/api && make sqlc`; **never hand-edit `apps/api/internal/store/sqlc/*`.**
- **铁律 2** — restart is student-triggered, never pushed; the challenge is always finishable; plain controls, no celebration. **铁律 1** — the coach questions, never answers. **克制** — a voice mis-transcription is never auto-sent; the student confirms.
- **Go tests:** from `apps/api`, `DOCKER_HOST=unix:///var/run/docker.sock CGO_ENABLED=0 go test -p 1 ./...` — FULL packages, never `-run` subsets for course/skill changes.
- **Web:** from `apps/web`, `npm test` + `npx tsc --noEmit`. **Contracts:** from `packages/contracts`, `npm test`.
- **Never `git add` a whole directory** — the pre-existing `M package.json` and untracked `docs/` files are NOT ours. Add named files only.
- **Icons inline SVG**; never import `lucide-react`.

---

## File Structure

- `packages/contracts/src/skill.ts` — **modify**: add `PhaseChallenge` Zod + optional `challenge` on `Contract`.
- `packages/contracts/skills/info-literacy-course.json` — **modify**: add the `challenge` (练一手) phase.
- `apps/api/internal/skills/specs/info-literacy-course.json` — **generated** by `make sync-skills`.
- `apps/api/internal/skills/skill.go` — **modify**: add `PhaseChallenge` Go struct + `Challenge` field on `Contract`.
- `apps/web/src/shell/courses/CoursePlayer.tsx` — **modify**: render `ChallengeTemplate` for a step-less phase carrying a `challenge` block.
- `apps/api/internal/store/queries/course_session.sql` (or wherever course_session queries live — confirm) — **modify**: add `DeleteCourseSessionByUserCourse`.
- `apps/api/internal/api/course_session.go` — **modify**: add `restartCourseSession` handler.
- `apps/api/internal/api/api.go` — **modify**: route `POST /courses/{id}/session/restart`.
- `apps/web/src/api/*` (course API client) + `apps/web/src/shell/courses/CourseReport.tsx` (or the finished screen) — **modify**: restart control.
- `apps/web/src/shell/courses/AskPanel.tsx` — **modify**: wire the inert 按住说话 to `AsrStream`.
- `docs/2026-07-20-student-platform-remaining-work.md` — **modify** (Task 8): mark N5c done, record deferrals.

---

## Task 1: The `challenge` phase (contracts + Go + skill JSON)

**Files:**
- Modify: `packages/contracts/src/skill.ts`
- Modify: `packages/contracts/skills/info-literacy-course.json`
- Modify: `apps/api/internal/skills/skill.go`
- Generated: `apps/api/internal/skills/specs/info-literacy-course.json` (via `make sync-skills`)
- Test: `packages/contracts/test/` (skill validation)

**Interfaces:**
- Produces: a `challenge` phase on `info-literacy-course` with `requires:["reflect"]`, `steps:[]`, `floor:[]`, an inline `challenge` block of shape `{title, prompt, reason_hint, anchors:[{id,dimension,question,answer}]}`. Later tasks read `contracts.challenge.challenge`.

- [ ] **Step 1: Add the Zod shape.** In `packages/contracts/src/skill.ts`, after `AnchorMaterial` (line ~38), add:

```ts
export const PhaseChallengeAnchor = z.object({
  id: z.string(),
  dimension: z.string(),
  question: z.string(),
  answer: z.string(),
});
export const PhaseChallenge = z.object({
  title: z.string(),
  prompt: z.string(),
  reason_hint: z.string(),
  anchors: z.array(PhaseChallengeAnchor),
});
export type PhaseChallenge = z.infer<typeof PhaseChallenge>;
```

and in the `Contract` object (after `anchor_material: AnchorMaterial.optional(),`, line ~53) add:

```ts
  challenge: PhaseChallenge.optional(),
```

- [ ] **Step 2: Add the Go struct.** In `apps/api/internal/skills/skill.go`, near `PhasePage`/`AnchorMaterial` (line ~86-97), add:

```go
// PhaseChallengeAnchor is one method dimension of a terminal self-check — a
// local type so the skills package takes NO dependency on agent.Anchor.
type PhaseChallengeAnchor struct {
	ID       string `json:"id"`
	Dimension string `json:"dimension"`
	Question string `json:"question"`
	Answer   string `json:"answer"`
}

// PhaseChallenge is a step-less phase's inline transfer self-check (练一手,
// N5c) — the ChallengeTemplate content the client renders. Machine never
// adjudicates it (DEC-3 via explicit confirmation); it is a local self-check.
type PhaseChallenge struct {
	Title      string                 `json:"title"`
	Prompt     string                 `json:"prompt"`
	ReasonHint string                 `json:"reason_hint"`
	Anchors    []PhaseChallengeAnchor `json:"anchors"`
}
```

Then add to the `Contract` struct (after `AnchorMaterial`, line ~60):

```go
	Challenge *PhaseChallenge `json:"challenge,omitempty"`
```

- [ ] **Step 3: Add the phase to the skill JSON.** In `packages/contracts/skills/info-literacy-course.json`, add a fifth entry to `contracts` (after `reflect`):

```json
    "challenge": {
      "requires": ["reflect"], "title": "练一手", "steps": [],
      "goal": "把方法迁移到一条全新的说法上——自己走一遍",
      "challenge": {
        "title": "练一手：这条你信不信？",
        "prompt": "有人在群里转发：「研究证明，多喝咖啡能显著延长寿命。」用你这节课学到的方法，自己判断一次。",
        "reason_hint": "你会不会转发？为什么？用你学到的方法说一句。",
        "anchors": [
          { "id": "c1", "dimension": "来源", "question": "这条说法的来源可信吗？谁做的研究、发在哪、是不是能查到原文？", "answer": "" },
          { "id": "c2", "dimension": "证据", "question": "证据够吗？是相关还是因果、样本多大、有没有说清楚适用范围？", "answer": "" },
          { "id": "c3", "dimension": "包装", "question": "有没有被包装？「研究证明」这类措辞在暗示什么、想让你直接相信什么？", "answer": "" }
        ]
      },
      "ask_chips": ["我可以说「还不确定」吗？", "光看一句话，能判断到什么程度？"],
      "floor": [],
      "soft_condition": "学生把方法用在了这条新说法上，而不是凭印象下结论",
      "gate": { "machine": [], "student_written": [], "human": [] }
    }
```

> `floor: []` is deliberate — always-finishable (铁律 2). Do NOT add a floor kind.

- [ ] **Step 4: Mirror to Go + write the failing test.** Run `cd apps/api && make sync-skills` (confirm the target). Add a contracts test in `packages/contracts/test/` asserting the skill parses and the `challenge` phase carries its inline block:

```ts
import { describe, it, expect } from "vitest";
import { Skill } from "../src/skill";
import course from "../skills/info-literacy-course.json";

describe("info-literacy-course challenge phase", () => {
  it("parses with a challenge phase carrying an inline challenge block", () => {
    const parsed = Skill.parse(course);
    const ch = parsed.contracts["challenge"];
    expect(ch).toBeDefined();
    expect(ch!.requires).toEqual(["reflect"]);
    expect(ch!.floor ?? []).toEqual([]);
    expect(ch!.challenge?.anchors.length).toBe(3);
  });
});
```

- [ ] **Step 5: Run tests.**

Run: `cd packages/contracts && npm test`
Expected: PASS (the new test + all existing; the skill validates).
Run: `cd apps/api && DOCKER_HOST=unix:///var/run/docker.sock CGO_ENABLED=0 go test -p 1 ./internal/skills/...`
Expected: PASS (the skill loads with the new phase; the canonical↔mirror guard, if any, sees byte-identical copies).

- [ ] **Step 6: Commit.**

```bash
git add packages/contracts/src/skill.ts packages/contracts/skills/info-literacy-course.json apps/api/internal/skills/skill.go apps/api/internal/skills/specs/info-literacy-course.json packages/contracts/test/*.ts
git commit -m "feat(n5c): add the 练一手 terminal-challenge phase to the course skill"
```

---

## Task 2: Terminal finish moves to the challenge phase (Go runtime)

Verify the course now finishes at `challenge`, not `reflect`, with NO change to the finish code — and that an empty floor always advances.

**Files:**
- Test: `apps/api/internal/api/course_session_test.go` (extend) and/or `apps/api/internal/agent/course_step_test.go`

**Interfaces:**
- Consumes: `skills.NextPhase(skill, phase)` (course_step.go), `runCourseAdvance` terminal.

- [ ] **Step 1: Write the failing test.** In `apps/api/internal/agent/course_step_test.go` (or the course session test), assert phase ordering and empty-floor advance. First the pure ordering (no DB):

```go
func TestCourseChallengeIsTerminal(t *testing.T) {
	sk, ok := skills.ByID("info-literacy-course")
	if !ok { t.Fatal("skill missing") }
	if next, ok := skills.NextPhase(sk, "reflect"); !ok || next != "challenge" {
		t.Fatalf("NextPhase(reflect) = %q,%v; want challenge,true", next, ok)
	}
	if _, ok := skills.NextPhase(sk, "challenge"); ok {
		t.Fatal("challenge must be the terminal phase (NextPhase = none)")
	}
}
```

> Match `NextPhase`'s real signature/return (read `course_step.go`); adapt if it returns a `Contract` rather than a phase id.

- [ ] **Step 2: Run to verify it fails, then passes.**

Run: `cd apps/api && DOCKER_HOST=unix:///var/run/docker.sock CGO_ENABLED=0 go test -p 1 ./internal/agent/ -run TestCourseChallengeIsTerminal`
Expected: initially may already PASS (Task 1 added the phase). If `NextPhase` needs the phases in a specific order, confirm the skill's contract iteration order yields reflect→challenge.

- [ ] **Step 3: Verify empty-floor advance in the runtime.** Confirm `runCourseAdvance`'s floor check treats `floor: []` as "always met" (an empty `unmetFloor` → advance allowed). Read the floor-checking helper; if an empty floor is already handled (it iterates floor items, none unmet → allowed), NO code change is needed. If it special-cases and blocks an empty floor, that is a bug to fix minimally (empty floor = met). Add a course-walk test driving the DB harness (`startSession` + `courseAdvance`) from the first phase to `challenge` and asserting the final advance sets `status=finished` + a `course_finished` event — reusing `course_session_test.go`'s `courseProvider`/`startSession`/`signInSeed`. Model it on the existing `TestCourseSession_Ask` setup.

- [ ] **Step 4: Run the FULL agent + api packages.**

Run: `cd apps/api && DOCKER_HOST=unix:///var/run/docker.sock CGO_ENABLED=0 go test -p 1 ./internal/agent/ ./internal/api/`
Expected: PASS, no regression (existing course-finish tests still green — the terminal moved one phase later).

- [ ] **Step 5: Commit.**

```bash
git add apps/api/internal/agent/course_step_test.go apps/api/internal/api/course_session_test.go
git commit -m "test(n5c): course finishes at 练一手, empty floor always advances"
```

---

## Task 3: CoursePlayer renders the inline challenge (web)

**Files:**
- Modify: `apps/web/src/shell/courses/CoursePlayer.tsx` (render block ~line 242-303)
- Test: `apps/web/src/shell/courses/CoursePlayer.test.tsx`

**Interfaces:**
- Consumes: `INFO_LITERACY_COURSE_SKILL.contracts[phase].challenge` (Task 1), `ChallengeTemplate` (already imported), `phaseSteps` (already returns `[]` for the step-less challenge phase, so the `renderCourseStep` effect early-returns).

- [ ] **Step 1: Write the failing test.** In `CoursePlayer.test.tsx`, mock a session at phase `challenge` and assert the challenge prompt renders (ChallengeTemplate, not TeachingTemplate). Match the file's existing session-mocking pattern (how it stubs `api.getCourseSession`):

```tsx
it("renders the ChallengeTemplate for the step-less 练一手 phase", async () => {
  // ... mock getCourseSession to return { phase: "challenge", status:"active", messages:[], openCards:[], collectedCards:[] }
  render(<CoursePlayer courseId={COURSE_ID} onFinish={() => {}} />);
  expect(await screen.findByText(/多喝咖啡能显著延长寿命/)).toBeInTheDocument();
});
```

- [ ] **Step 2: Run to verify it fails.**

Run: `cd apps/web && npm test -- CoursePlayer`
Expected: FAIL — no challenge branch; the step-less phase renders nothing/TeachingTemplate.

- [ ] **Step 3: Add the challenge branch.** In `CoursePlayer.tsx`, near the derived render flags (line ~242-252), add:

```tsx
  const challengeBlock = currentContract?.challenge;
  const showAuthoredChallenge = stepless && !!challengeBlock;
```

and in the render JSX (line ~296), add the challenge branch FIRST (before `showAuthoredPage`), so a step-less phase with a `challenge` block renders it:

```tsx
            {showAuthoredChallenge && challengeBlock ? (
              <ChallengeTemplate content={challengeBlock} />
            ) : showAuthoredPage && pageBlock ? (
              <TeachingTemplate content={{ title: pageBlock.title, subtitle: pageBlock.subtitle, body: pageBlock.body, foreground_asset_id: null }} />
            ) : !rendered ? (
              /* existing loading */
            ) : rendered.template === "challenge" ? (
              <ChallengeTemplate content={rendered.content as ChallengeContent} />
            ) : (
              <TeachingTemplate content={rendered.content as TeachingContent} />
            )}
```

> `challengeBlock` is already the `ChallengeContent` shape (`{title,prompt,reason_hint,anchors}`); no mapping needed. Keep the existing three branches intact below the new one.

- [ ] **Step 4: Run tests.**

Run: `cd apps/web && npm test -- CoursePlayer && npx tsc --noEmit`
Expected: PASS, tsc clean.

- [ ] **Step 5: Commit.**

```bash
git add apps/web/src/shell/courses/CoursePlayer.tsx apps/web/src/shell/courses/CoursePlayer.test.tsx
git commit -m "feat(n5c): render the inline 练一手 challenge in CoursePlayer"
```

---

## Task 4: Session restart (Go query + endpoint)

**Files:**
- Modify: the course_session queries file (find it: `grep -rl "CreateCourseSession" apps/api/internal/store/queries/`) — add `DeleteCourseSessionByUserCourse`.
- Modify: `apps/api/internal/api/course_session.go` — add `restartCourseSession`.
- Modify: `apps/api/internal/api/api.go` — route it.
- Test: `apps/api/internal/api/course_session_test.go`

**Interfaces:**
- Produces: `POST /api/v1/courses/{id}/session/restart` → deletes the caller's session (cascade) and returns a fresh `CourseSessionDTO`.

- [ ] **Step 1: Add the delete query.** In the course_session `.sql` file, add:

```sql
-- name: DeleteCourseSessionByUserCourse :exec
DELETE FROM course_session WHERE user_id = $1 AND course_id = $2;
```

Run `cd apps/api && make sqlc` (regenerates; never hand-edit sqlc output).

- [ ] **Step 2: Write the failing test.** In `course_session_test.go`:

```go
func TestCourseSession_Restart(t *testing.T) {
	pool := newAPITestPool(t)
	h := /* the course-enabled handler, as other course tests build it */
	cookie := signInSeed(t, pool)
	courseID := /* the seeded course id, as startSession uses */
	sess := startSession(t, h, cookie, courseID)
	// advance once so there is session state to wipe, then:
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("POST", "/api/v1/courses/"+courseID+"/session/restart", nil), cookie))
	if rec.Code != http.StatusOK { t.Fatalf("restart = %d; %s", rec.Code, rec.Body) }
	fresh := startSession(t, h, cookie, courseID) // or parse the restart body
	if fresh.ID == sess.ID { t.Fatal("restart must mint a NEW session id") }
	if fresh.Phase != /* first phase, e.g. "demonstrate" */ "demonstrate" { t.Fatalf("restart phase = %q; want first", fresh.Phase) }
}
```

Also add an ownership test: another user's restart is 404 (mirror `TestCourseSession_OwnershipIs404`).

- [ ] **Step 3: Run to verify it fails.**

Run: `cd apps/api && DOCKER_HOST=unix:///var/run/docker.sock CGO_ENABLED=0 go test -p 1 ./internal/api/ -run TestCourseSession_Restart`
Expected: FAIL — route/handler absent (404 or 405).

- [ ] **Step 4: Add the handler + route.** In `course_session.go`:

```go
// restartCourseSession deletes the caller's session for this course — the
// ON DELETE CASCADE on session-scoped material/card_instances/event/
// evaluations wipes the prior run — then re-creates a fresh session at the
// first phase and returns it. 铁律 2: student-triggered, never pushed.
func (a *API) restartCourseSession(w http.ResponseWriter, r *http.Request) {
	u, _ := UserFromContext(r.Context())
	courseID, err := uuid.Parse(r.PathValue("id"))
	if err != nil { httpx.WriteError(w, r, err); return }
	// Owner-scoped: only delete a session this caller owns (the delete is
	// already keyed by user_id, so a non-owner deletes nothing — idempotent).
	if err := a.d.Queries.DeleteCourseSessionByUserCourse(r.Context(), sqlc.DeleteCourseSessionByUserCourseParams{
		UserID: u.ID, CourseID: pgUUID(courseID),
	}); err != nil { httpx.WriteError(w, r, err); return }
	// Re-create + return fresh state, reusing the existing get-or-create path.
	// (Call the same code startCourseSession/getOrCreateSession uses.)
	dto, err := a.getOrCreateCourseSessionDTO(r.Context(), u.ID, courseID) // reuse existing helper; if the existing handler inlines this, extract it minimally
	if err != nil { httpx.WriteError(w, r, err); return }
	httpx.WriteJSON(w, http.StatusOK, dto)
}
```

> Match the real param types (`pgUUID`, `sqlc.…Params`) and reuse whatever get-or-create helper `startCourseSession` already calls — do NOT duplicate session-creation logic; extract it if inlined. In `api.go`, add: `mux.Handle("POST /api/v1/courses/{id}/session/restart", protected(a.restartCourseSession))`.

- [ ] **Step 5: Run the FULL api package.**

Run: `cd apps/api && DOCKER_HOST=unix:///var/run/docker.sock CGO_ENABLED=0 go test -p 1 ./internal/api/`
Expected: PASS.

- [ ] **Step 6: Commit.**

```bash
git add apps/api/internal/store/queries/*.sql apps/api/internal/store/sqlc/ apps/api/internal/api/course_session.go apps/api/internal/api/api.go apps/api/internal/api/course_session_test.go
git commit -m "feat(n5c): session restart — delete+recreate, cascade wipes the run"
```

---

## Task 5: Restart control (web)

**Files:**
- Modify: the course API client (find it: `grep -rl "getCourseSession" apps/web/src`) — add `restartCourseSession`.
- Modify: `apps/web/src/shell/courses/CourseReport.tsx` (or the finished screen) — a 「重新开始」 control.
- Test: the matching `.test.tsx`.

- [ ] **Step 1: Write the failing test.** Assert the finished screen shows a 「重新开始」 control that calls the client and resets. Match the existing test's mocking:

```tsx
it("offers 重新开始 on the finished screen and restarts on click", async () => {
  const restart = vi.spyOn(api, "restartCourseSession").mockResolvedValue(/* fresh session */);
  render(/* the finished-state component */);
  await userEvent.click(screen.getByRole("button", { name: /重新开始/ }));
  expect(restart).toHaveBeenCalledWith(COURSE_ID);
});
```

- [ ] **Step 2: Run to verify it fails.**

Run: `cd apps/web && npm test -- CourseReport`
Expected: FAIL — no control.

- [ ] **Step 3: Add the client + control.** In the course API client add:

```ts
export async function restartCourseSession(courseId: string): Promise<CourseSession> {
  return postJSON(`/api/v1/courses/${courseId}/session/restart`, {});
}
```

(match the file's real request helper + return type). Add a plain 「重新开始」 button on the finished screen (no celebration — 铁律 2) that calls it and drives the player back to a fresh session (reset local state to the returned session / navigate to the course start).

- [ ] **Step 4: Run tests.**

Run: `cd apps/web && npm test -- CourseReport && npx tsc --noEmit`
Expected: PASS, tsc clean.

- [ ] **Step 5: Commit.**

```bash
git add apps/web/src/api/* apps/web/src/shell/courses/CourseReport.tsx apps/web/src/shell/courses/CourseReport.test.tsx
git commit -m "feat(n5c): 重新开始 control restarts a finished course"
```

---

## Task 6: Course push-to-talk voice (web)

Wire AskPanel's inert 按住说话 to `AsrStream`, mirroring `CoachRail`'s ASR handler exactly.

**Files:**
- Modify: `apps/web/src/shell/courses/AskPanel.tsx` (the deferred block ~line 279-287)
- Reference (read, do not change): `apps/web/src/studio/CoachRail.tsx` (`AsrStream`, `recording`, `asrRef`, `stopRecording`, the hold handlers) + `apps/web/src/api/voice.ts`.
- Test: `apps/web/src/shell/courses/AskPanel.test.tsx`

- [ ] **Step 1: Write the failing test.** Mock `AsrStream`; assert holding 按住说话 opens the stream and a transcript fills the input, and releasing does NOT auto-send (克制 — the student confirms). Match `CoachRail.test.tsx`'s voice-mock approach:

```tsx
it("holds 按住说话 to transcribe into the input, and does not auto-send", async () => {
  // ... mock AsrStream to emit a transcript "这条我不太信"
  render(<AskPanel /* props */ />);
  fireEvent.mouseDown(screen.getByText(/按住说话/));
  // ... emit transcript
  fireEvent.mouseUp(screen.getByText(/按住说话/));
  expect((screen.getByPlaceholderText(/输入你的问题/) as HTMLTextAreaElement).value).toContain("这条我不太信");
  // onAsk was NOT called on release
});
```

- [ ] **Step 2: Run to verify it fails.**

Run: `cd apps/web && npm test -- AskPanel`
Expected: FAIL — the button is inert.

- [ ] **Step 3: Wire it.** Replace the deferred block (line ~279-287) with a live push-to-talk control copying `CoachRail`'s pattern: a `recording` state, an `asrRef` holding the `AsrStream`, `onMouseDown`/`onTouchStart` → open the stream (transcript → the ask input state), `onMouseUp`/`onTouchEnd`/`onMouseLeave` → `stopRecording`. Fill the existing ask-input state; the student sends via the existing send handler. Show a `voiceError` note on failure (a plain line, not a modal — 铁律 2). Inline SVG mic icon; no `lucide-react`.

- [ ] **Step 4: Run tests.**

Run: `cd apps/web && npm test -- AskPanel && npx tsc --noEmit`
Expected: PASS, tsc clean.

- [ ] **Step 5: Commit.**

```bash
git add apps/web/src/shell/courses/AskPanel.tsx apps/web/src/shell/courses/AskPanel.test.tsx
git commit -m "feat(n5c): wire course push-to-talk voice to the ASR endpoint"
```

---

## Task 7: Full-suite gate + tracker

- [ ] **Step 1: Run EVERY suite.**

```bash
cd apps/api && DOCKER_HOST=unix:///var/run/docker.sock CGO_ENABLED=0 go test -p 1 -count=1 ./...
cd ../../apps/web && npm test && npx tsc --noEmit
cd ../packages/contracts && npm test
```
Expected: all green. Skill canonical↔mirror byte-identical (`make sync-skills` reproduces). Watch for any card/skill *enumeration* test that counts skills/courses (the N3e lesson — adding to a registry can bump a hardcoded count); fix any stale count.

- [ ] **Step 2: Update the tracker.** In `docs/2026-07-20-student-platform-remaining-work.md`, mark the N5 row / items done for this slice (terminal challenge, session restart, course voice) and record the deferrals (multi-course — course format undecided; chat/course→project seeding — the "carry into a project" product question; multimodal — non-MVP; terminal-answer capture — with the report design). Add an `### N5c · DONE` section summarizing the three components + the "self-check via explicit confirmation, no adjudication, no migration" shape.

- [ ] **Step 3: Commit.**

```bash
git add docs/2026-07-20-student-platform-remaining-work.md
git commit -m "docs(n5c): mark course-completion slice done + record deferrals"
```

---

## Notes for the executor

- **The finish code does not change.** If a task tempts you to edit `runCourseAdvance`'s terminal, stop — appending the `challenge` phase makes it terminal automatically. The only Go behavior to confirm is that an empty `floor` advances (Task 2 Step 3).
- **No migration, no LLM call, no machine-set `solid`.** The challenge is a local self-check; completion is the student's explicit next-press.
- **Reuse, don't duplicate:** restart reuses the get-or-create session helper; course voice reuses `CoachRail`'s `AsrStream` client. Extract a shared helper only if the existing one is inlined; never copy a logic block verbatim.
