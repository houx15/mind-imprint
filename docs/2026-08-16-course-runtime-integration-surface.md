# Course Runtime — Existing Integration Surface (reference for Slices 7–8)

Date: 2026-08-16. Read-only map of the CURRENT course system, to plan replacing the student player (Slice 8) and adding runtime Opening/Closing generation (Slice 7). The NEW runtime consumes CourseDefinition 2.0 (`docs/2026-08-15-student-course-runtime-data-and-renderer-design.md`); the current system is a different, pre-rendered shape.

## Current frontend player (to be replaced in Slice 8)
- Mount: `apps/web/src/shell/StudentApp.tsx:5,93` → `<CoursesContainer initialCourseId onGoPortal>` on the 课程 tab.
- `apps/web/src/shell/courses/CoursesContainer.tsx:30` — view state machine `grid | player | report`; `:40` mounts `<CoursePlayer courseId onExit onFinish>`; `:55` `<CoursesView onOpenCourse>`. Report view is an inert placeholder (old report pipeline retired).
- `courseId` passed around is actually the **slug**.
- `CoursePlayer.tsx`: `api.getCourse(slug)` → `CoursePlayerPayload`; pages `payload.renderCache.steps[ordinal]`; assets from `structure.asset_library`; audio from `payload.audioKeys[pieceId]` resolved via `api.resolveUrl(key)` through a reused `HTMLAudioElement`; writes `saveCourseProgress`/`answerCourseQuiz`/`courseAsk`(SSE).
- Data shapes are Zod in `packages/contracts/src/course.ts` (`CoursePlayerPayload`, `RenderCache`, `RenderStepContent`, `CourseStructure`, `CourseSummary`, `CourseProgress`). **This is the OLD shape — the new runtime does not use it.**

## Current frontend API client — `apps/web/src/api/courses.ts`
- `listCourses` GET `/api/v1/courses` → `{courses: CourseSummary[]}`
- `getCourse` GET `/api/v1/courses/{slug}` → `{course: CoursePlayerPayload}`
- `getCourseProgress` GET `/api/v1/courses/{slug}/progress`
- `saveCourseProgress` PUT `/api/v1/courses/{slug}/progress`
- `answerCourseQuiz` POST `/api/v1/courses/{slug}/quiz-answer`
- `courseAsk` POST `/api/v1/courses/{slug}/ask` (SSE)

## Current backend (Go)
- Routes in `apps/api/internal/api/api.go:208-215`; handlers in `apps/api/internal/api/course.go`; store `apps/api/internal/agent/coursestore.go`; SQL `apps/api/internal/store/queries/course.sql`.
- Table **`course`**: `id, slug, branch, title, blurb, time_label, card_ids, step_count, structure(jsonb), render_cache(jsonb), audio_manifest(jsonb)` (migrations 0050, audio_manifest 0052).
- Table **`course_progress`**: `user_id, course_id, current_ordinal, completed_ordinals[], started_at, completed_at, active_seconds`.
- Content is **pre-rendered JSON**, uploaded via admin route `course_admin.go postAdminUploadCourse` (OSS_ADMIN_KEY gated) or `seed_courses.go`. No runtime render step today.
- Events → `event` table (FK course_id); literals `step_viewed`, `course_message`, `course_quiz_answered`.

### Slice 8 implication
The new player needs to LOAD a CourseDefinition 2.0 **package** (course.json + relative assets), a shape the `course` table does not hold. Options for Slice 8: (a) add a `course_definition jsonb` column / new table for 2.0 packages + a `GET /api/v1/courses/{slug}/definition` endpoint returning the CourseDefinition document; (b) a new `course_v2` table. Seed at least one real 2.0 course (the golden coverage course) so the player has content before the generator (out of scope) emits 2.0. CourseSession (§16) is richer than `course_progress` — Slice 8's `SessionAdapter` needs create/appendEvent/saveSliceState/saveScene/setStatus endpoints, or a mapping onto a new `course_session` table. Decide at Slice 8; surface to the user given the "replace now" instruction and the data-format dependency.

## LLM gateway seam (for Slice 7)
- Deps resolvers `api.go:40-43`: `Provider` (MuxProvider); `ChatResolver`=chaperone/mid; `FastChatResolver`=fast; **`EvalResolver`=flagship**. Tier = which resolver you pass.
- Non-streaming: `gateway.Collect(ctx, provider, resolved, ChatRequest)` (`gateway/collect.go:10`). Streaming: `gateway.NewSSEWriter`.
- Canonical "resolve flagship → call agent → persist JSON" example: `apps/api/internal/api/evaluation_report.go:153` → `runReportGeneration` (`evaluation_generate.go`) → agent `apps/api/internal/agent/reportgen.go` (`Generate*` fns: `(ctx, prov, resolved, in) (Result, gateway.ChatUsage, error)` using `collectReport`).
- Simplest course-scoped LLM example: `apps/api/internal/agent/course_coach.go:52 ProposeCourseAskReply(...)`, called from `course.go:326`, metered via `store.RecordCourseLLMCall(...,"coach",...)`.
- **Opening/Closing generation** = new agent fn modeled on `reportgen.go`; resolve `EvalResolver` (quality) or `ChatResolver` (cheap); build prompt from approved facts + permitted signals (§6.1/§6.2); return narration text; meter the call.

## Voice / TTS (for Slice 7 audio)
- `apps/api/internal/voice/tts.go`: `Client.Synthesize(ctx, text, speed) ([]byte, error)` → Volcano v3 WS TTS, returns **mp3** bytes.
- Seam: `apps/api/internal/api/voice.go:28 VoiceService = { Synthesize; Voice() }`, `Deps.Voice` (nil → 503).
- Pre-gen pattern: `apps/api/internal/agent/course_audio.go:79 GenerateCourseAudio(ctx, synth, store, slug, renderCache) → map[pieceId]objectKey`. OSS key `courses/audio/<slug>/<stepId>_<segIdx>_<hash8>.mp3`, hash8 = sha256(voice+text)[:8]. `VoiceService` satisfies synth; `*oss.Service` satisfies store (`Exists`/`PutObject`). Nil-guarded so publish never fails without narration.
- **Runtime Opening/Closing audio**: synthesize text via `Deps.Voice.Synthesize` → `Deps.OSS.PutObject` with a new key namespace (e.g. `courses/audio/<slug>/session/<sessionId>/opening_<hash8>.mp3`), store the resolved URL in `RuntimeSceneResult.audioUrl` on `CourseSession`. Reuse the hash-cache idea. Static fallback text when Voice/OSS unavailable.
