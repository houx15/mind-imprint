# Course Runtime — Slice 7: Opening/Closing Runtime Scene Generation

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Give the student platform a real `RuntimeSceneGenerator`: a Go endpoint that generates the Opening/Closing narration text from teacher-approved facts + permitted signals, synthesizes narration audio (Volcano TTS → OSS), and returns a `RuntimeSceneResult`; plus a frontend `ApiSceneGenerator` implementing the Slice 2 interface. Falls back to the authored static text on any generation/synthesis failure.

**Architecture:** A single new endpoint `POST /api/v1/courses/{slug}/scene` accepts `{ which: "opening"|"closing", facts, allowedSignals, signalEvidence }` and returns `{ result: RuntimeSceneResult }`. The handler resolves a model tier (`ChatResolver` — chaperone quality is fine for narration; not the flagship evaluator), calls a new `agent.GenerateCourseScene` modeled on `reportgen.go`, meters the call, then (if `Voice`+`OSS` present) synthesizes mp3 via the `course_audio.go` pattern and stores it under a scene key namespace, returning `audioUrl`. On LLM or TTS failure it returns the authored fallback text with `fallbackUsed:true` (never an error to the player). The generator receives ONLY permitted signals (§20). Generation results are owned by `CourseSession` (persisted by Slice 8), not by `CourseDefinition`.

**Tech Stack:** Go (`net/http`, existing `gateway`/`voice`/`oss` seams), frontend TS adapter. No new DB table in this slice — the endpoint is stateless; persistence is Slice 8.

**Authoritative spec:** design doc §6.1/§6.2/§6.3; integration map `docs/2026-08-16-course-runtime-integration-surface.md` (§4 LLM seam, §5 TTS).

## Global Constraints

- Reuse existing seams: LLM via `gateway.Collect(ctx, a.d.Provider, resolved, ChatRequest)` with `resolved` from `a.d.ChatResolver` (see `agent/course_coach.go`); TTS via `a.d.Voice.Synthesize(ctx, text, speed)`; storage via `a.d.OSS.PutObject`/`Exists` + a signed-URL resolve. Nil-guard `Voice`/`OSS` (like `course_audio.go`) so absence degrades to text-only, never a 500.
- Reasoning-model budget: size `MaxTokens` generously (≥4000) so a reasoning tier isn't starved (see memory `llm-reasoning-model-budgets`; narration is short but the model may spend hidden reasoning). Chaperone tier disables thinking, but keep the cap safe.
- Never leak keys/secrets into responses, logs, or the payload. The generator prompt contains only approved facts + permitted signal values.
- `RuntimeSceneResult` shape is fixed by `@mind-imprint/course-contract` (`{ text, audioUrl?, generatedAt, usedSignalTypes, fallbackUsed }`). The Go DTO must serialize to exactly that JSON.
- Go tests use testcontainers — run `go test ./...` from `apps/api` with `-timeout 1800s`, FOREGROUND (memory: testcontainers stall when backgrounded). sqlc unaffected (no new query here). `CGO_ENABLED=0` for sqlc if regenerating (not needed this slice).
- No `/dev/null` redirects; never `git add -A`.

---

### Task 1: `agent.GenerateCourseScene`

**Files:** Create `apps/api/internal/agent/course_scene.go`. Test `apps/api/internal/agent/course_scene_test.go`.

**Interfaces:**
- Produces: `GenerateCourseScene(ctx, prov gateway.Provider, resolved gateway.Resolved, in SceneGenInput) (SceneGenResult, gateway.ChatUsage, error)`.
  - `SceneGenInput`: `{ Which string; CourseTitle string; EstimatedMinutes int; Objectives []string; LearningPreview []string; PreparedSummary string; Takeaways []string; TransferApplications []string; AllowedSignals []string; SignalEvidence map[string]string }`.
  - `SceneGenResult`: `{ Text string; UsedSignalTypes []string }`.
  - Builds a system prompt enforcing §6.1/§6.2 rules: opening = greeting + what they'll do + estimated time + optional evidence-supported prior-learning connection + invitation to begin; closing = consistent with preparedSummary, mention only evidence that exists, distinguish completion from mastery, no unsupported improvement claims, name takeaways + transfer. Restrained, concise, Chinese for the initial experience.
  - Uses `gateway.Collect`; returns text + which signal types were actually used.

- [ ] **Step 1: Failing test** — a fake `gateway.Provider` (see existing agent tests for the stub pattern) returns a canned completion; assert `GenerateCourseScene` returns its text and threads `UsedSignalTypes`. Assert the prompt includes the course title and, for closing, the prepared summary (inspect the request the fake provider received).
- [ ] **Step 2: Run → FAIL** (`cd apps/api && go test ./internal/agent/ -run GenerateCourseScene -timeout 1800s`).
- [ ] **Step 3: Implement.** — [ ] **Step 4: Run → PASS.** — [ ] **Step 5: Commit** — `feat(api): course scene (opening/closing) generation agent`.

---

### Task 2: Scene endpoint `POST /courses/{slug}/scene`

**Files:** Create `apps/api/internal/api/course_scene.go`. Modify `apps/api/internal/api/api.go` (register route). Test `apps/api/internal/api/course_scene_test.go`.

**Spec:** stateless generation + TTS + fallback.

**Interfaces:**
- Produces: handler `sceneForCourse` behind `protected`. Request `{ which, facts:{title,estimatedMinutes,objectives,learningPreview,preparedSummary,takeaways,transferApplications}, allowedSignals:[], signalEvidence:{} }`. Response `{ result: { text, audioUrl?, generatedAt, usedSignalTypes, fallbackUsed } }`.
  - Flow: validate `which`; resolve `a.d.ChatResolver`; call `GenerateCourseScene`; meter via the course LLM-call recorder (`store.RecordCourseLLMCall(..., "scene", ...)` — mirror `course.go`'s coach metering). If LLM errors → build result from the request's fallback text (caller passes it) with `fallbackUsed:true`, HTTP 200. On success and if `Voice`+`OSS` non-nil: synthesize mp3 (`Synthesize(text, 1.0)`), `PutObject` under `courses/audio/<slug>/scene/<which>_<hash8>.mp3` (hash8 = sha256(voice+text)[:8], idempotent via `Exists`), resolve signed URL → `audioUrl`. TTS failure → text-only result (still 200, `fallbackUsed:false` since text generated).
  - `generatedAt` = server time (RFC3339). This is the ONE place a timestamp is stamped — the frontend passes it into the (pure) session, per Slice 2's determinism boundary.

- [ ] **Step 1: Failing test** — with a fake provider + nil Voice/OSS: POST a valid opening request → 200, `result.text` from the fake, `fallbackUsed:false`, no `audioUrl`. With a provider that errors → 200 with the fallback text and `fallbackUsed:true`. Unknown `which` → 400. Unauthenticated → 401.
- [ ] **Step 2: Run → FAIL** (`cd apps/api && go test ./internal/api/ -run Scene -timeout 1800s`).
- [ ] **Step 3: Implement** handler + route `mux.Handle("POST /api/v1/courses/{slug}/scene", protected(a.sceneForCourse))`.
- [ ] **Step 4: Run → PASS.** — [ ] **Step 5: Commit** — `feat(api): opening/closing scene generation endpoint`.

---

### Task 3: Frontend `ApiSceneGenerator` adapter

**Files:** Create `apps/web/src/course/apiSceneGenerator.ts`. Create `apps/web/src/api/courseScene.ts` (client fn). Test `apps/web/test/course/apiSceneGenerator.test.ts`.

**Interfaces:**
- Produces: `apiScene(slug, body): Promise<{ result: RuntimeSceneResult }>` (client, `POST /courses/{slug}/scene`), and `makeApiSceneGenerator(slug): RuntimeSceneGenerator` (implements the Slice 2 interface: `generate(input) → Promise<RuntimeSceneResult>`), translating the CourseDefinition Opening/Closing inputs + permitted session signals into the request body and returning the server result. On network failure → return the authored fallback text with `fallbackUsed:true` (never throw into the player).

- [ ] **Step 1: Failing test** — mock the client; `makeApiSceneGenerator("s").generate(openingInput)` returns the server `RuntimeSceneResult`; on the client rejecting, it returns the fallback result with `fallbackUsed:true`.
- [ ] **Step 2: Run → FAIL** (`pnpm --filter web test apiSceneGenerator`).
- [ ] **Step 3: Implement.** — [ ] **Step 4: Run → PASS** + `pnpm --filter web typecheck`. — [ ] **Step 5: Commit** — `feat(web): api-backed runtime scene generator`.

---

## Self-Review Notes

- **Stateless by design:** this slice only generates; persisting `opening`/`closing` results into `CourseSession` is Slice 8 (needs the session store). The endpoint is safe to call repeatedly (audio is content-hash idempotent).
- **Fallback is load-bearing:** every failure path (LLM down, TTS down, OSS down, Voice unconfigured) still returns 200 with usable text so the player never stalls (§6.1/§6.2 "use the static fallback when generation or speech synthesis fails").
- **Signal minimization:** only `allowedSignals` values reach the prompt; `usedSignalTypes` reports what was actually used, feeding `RuntimeSceneResult`.
- **Tier choice:** chaperone (`ChatResolver`), not the flagship `EvalResolver` — narration is a lighter task than rubric evaluation, and evaluation must never be downgraded, so keep them on separate resolvers.
