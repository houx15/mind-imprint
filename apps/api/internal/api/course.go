package api

// course.go — Task 5's course v2 HTTP surface: slug-keyed handlers over the
// Task 4 content/progress/event store (agent.NewSqlcAgentStore). Replaces the
// retired course-id + session/step-render surface (course_session.go,
// course_assessment*.go, course_render.go — deleted alongside this file, per
// migration 0050's course_session/course_step/course_step_render drop).
// postCourseAsk (Task 6) is the free-Q&A AI bar: no phase runtime left to
// drive (agent/course_coach.go's package doc), just one coach turn scoped to
// {slug, ordinal}, streamed over SSE mirroring chat.go's postChatTurn.
// postAdminUploadCourse (Task 7) is the OSS_ADMIN_KEY-gated developer publish
// path — see course_admin.go, not this file.

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"

	"mindimprint/api/internal/agent"
	"mindimprint/api/internal/gateway"
	"mindimprint/api/internal/httpx"
)

// listCourses returns the catalog WITH each card's own progress for the authed
// student (`progress`, null when untouched). The progress used to be the
// client's job — the 课程 list fired one GET /courses/{slug}/progress per card,
// an N+1 that grew with the catalog and raced its own renders. It is one extra
// indexed query here (ListProgressForUser), so the list is a single round trip.
//
// Progress is per-student, so it is attached only when there IS an authed
// student in context; the admin listing (listCoursesAdmin) reuses
// toCourseSummaryDTO without it, and an admin browsing the catalog sees their
// own progress like anyone else.
func (a *API) listCourses(w http.ResponseWriter, r *http.Request) {
	store := agent.NewSqlcAgentStore(a.d.Queries, a.d.Pool)
	rows, err := store.ListCourses(r.Context(), isAdmin(r.Context()))
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	var progress map[string]agent.CourseCatalogProgress
	if user, ok := UserFromContext(r.Context()); ok {
		progress, err = store.ListProgressForUser(r.Context(), user.ID)
		if err != nil {
			httpx.WriteError(w, r, err)
			return
		}
	}
	// 🚨 受众过滤（migration 0133）。一门标了 pro 的课对一个 lite 学生就是不
	// 存在——不是灰着、不是"升级可见"，是不在目录里。管理员看全部：他们是作者。
	//
	// 过滤放在这里而不是 SQL 里，是因为 ListCourses 还服务着 admin 目录和种子
	// 校验，把版本塞进那条查询会让它多一个所有调用点都得回答的问题。目录只有
	// 几十行，一次 Go 侧过滤便宜得多。
	edition := ""
	if !isAdmin(r.Context()) {
		edition = a.callerEdition(r.Context())
	}
	out := make([]courseSummaryDTO, 0, len(rows))
	for _, c := range rows {
		if !courseVisibleTo(c.Audience, edition) {
			continue
		}
		out = append(out, withCatalogProgress(a.toCourseSummaryDTO(c), progress))
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"courses": out})
}

// getCourse loads one course's full player payload by its external slug.
// httpx.WriteError already maps pgx.ErrNoRows (an unknown slug) to 404.
func (a *API) getCourse(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")
	if !a.requireVisibleCourse(w, r, slug) {
		return
	}
	store := agent.NewSqlcAgentStore(a.d.Queries, a.d.Pool)
	payload, _, err := store.GetCoursePayload(r.Context(), slug)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"course": toCoursePayloadDTO(payload)})
}

func (a *API) getCourseProgress(w http.ResponseWriter, r *http.Request) {
	user, _ := UserFromContext(r.Context())
	slug := r.PathValue("slug")
	store := agent.NewSqlcAgentStore(a.d.Queries, a.d.Pool)
	row, err := store.GetProgress(r.Context(), user.ID, slug)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"progress": toCourseProgressDTO(slug, row)})
}

// postCourseRestart relearns a course. For a 2.0 course this is the attempt-log
// model (0077): any FINISHED attempt is KEPT as a permanent record (its report is
// still viewable from the history), and a brand-new 'created' attempt is minted to
// start over from Opening. A partial, never-finished current attempt is discarded
// first so abandoned runs don't accrete. A legacy course keeps the old wipe (its
// single course_progress row). Course events (铁律④ evidence) are kept either way.
// Owner-scoped: only the authed user's own rows are touched.
func (a *API) postCourseRestart(w http.ResponseWriter, r *http.Request) {
	user, _ := UserFromContext(r.Context())
	slug := r.PathValue("slug")
	if !a.requireVisibleCourse(w, r, slug) {
		return
	}
	store := agent.NewSqlcAgentStore(a.d.Queries, a.d.Pool)
	_, courseID, err := store.GetCoursePayload(r.Context(), slug)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	// A 2.0 course carries a definition; a legacy one doesn't. The slug is already
	// validated (GetCoursePayload above), so a non-nil error here is a real DB fault.
	def, _, err := store.GetCourseDefinition(r.Context(), slug)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if len(def) > 0 {
		// 2.0: drop only an unfinished current attempt, then mint a fresh one. The
		// finished attempts (each with completed_at set) stay as history records.
		if err := store.DeleteIncompleteLatestCourseSession(r.Context(), user.ID, courseID); err != nil {
			httpx.WriteError(w, r, err)
			return
		}
		raw, status, berr := buildInitialCourseSession(def, user.ID)
		if berr != nil {
			httpx.WriteError(w, r, &httpx.APIError{
				Status: http.StatusUnprocessableEntity, Code: "invalid_course_definition", Message: "课程定义格式无效",
			})
			return
		}
		if _, err := store.CreateCourseSession(r.Context(), user.ID, courseID, raw, status); err != nil {
			httpx.WriteError(w, r, err)
			return
		}
		httpx.WriteJSON(w, http.StatusOK, map[string]any{"ok": true})
		return
	}
	// Legacy course: wipe the single progress row (idempotent).
	if err := store.DeleteCourseProgress(r.Context(), user.ID, courseID); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// getCourseHistory returns the courses the authed student has engaged with —
// across both the 2.0 runtime (course_session) and legacy (course_progress)
// storage — newest activity first. Title/cover are intentionally left out (the
// frontend enriches from the course list) so this endpoint stays cover-signing
// free. status is the raw runtime session status or 'completed'/'in-progress'.
func (a *API) getCourseHistory(w http.ResponseWriter, r *http.Request) {
	user, _ := UserFromContext(r.Context())
	store := agent.NewSqlcAgentStore(a.d.Queries, a.d.Pool)
	items, err := store.ListCourseHistory(r.Context(), user.ID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	out := make([]map[string]any, 0, len(items))
	for _, it := range items {
		row := map[string]any{
			"attemptId":      it.AttemptID, // "" for a legacy course
			"slug":           it.Slug,
			"status":         it.Status,
			"completedCount": it.CompletedCount,
			"updatedAt":      it.UpdatedAt.UTC().Format("2006-01-02T15:04:05Z07:00"),
		}
		// completedAt is the FROZEN completion date — present only once finished.
		if it.CompletedAt != nil {
			row["completedAt"] = it.CompletedAt.UTC().Format("2006-01-02T15:04:05Z07:00")
		}
		out = append(out, row)
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"items": out})
}

// putCourseProgress writes the resume position (current_ordinal), optionally
// marks ONE step complete (completed_ordinal — the step the student just
// finished per the per-step gate), and accumulates active-focus time
// (active_seconds_delta). A client-sent completed_ordinalS array is still
// silently ignored (decodeJSON has no DisallowUnknownFields) — completion is
// driven only by the singular completed_ordinal, and whether the WHOLE course
// is finished is computed in the store from the authored step count (parsed
// here from structure.steps), never asserted by the client.
func (a *API) putCourseProgress(w http.ResponseWriter, r *http.Request) {
	user, _ := UserFromContext(r.Context())
	slug := r.PathValue("slug")
	var body struct {
		CurrentOrdinal     int  `json:"current_ordinal"`
		CompletedOrdinal   *int `json:"completed_ordinal"`
		ActiveSecondsDelta int  `json:"active_seconds_delta"`
	}
	if err := decodeJSON(r, &body); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	store := agent.NewSqlcAgentStore(a.d.Queries, a.d.Pool)
	payload, courseID, err := store.GetCoursePayload(r.Context(), slug)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	var structure struct {
		Steps []json.RawMessage `json:"steps"`
	}
	_ = json.Unmarshal(payload.Structure, &structure)
	stepCount := len(structure.Steps)

	row, err := store.SaveProgress(r.Context(), user.ID, courseID, body.CurrentOrdinal, body.CompletedOrdinal, body.ActiveSecondsDelta, stepCount)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	// 铁律④: the step turn itself is evidence, logged even though paging never
	// gates. A log failure must not fail the request — the progress write
	// already succeeded.
	// Event type is "step_viewed" (NOT "course_step_viewed") — the vocabulary
	// GetClassWeekStats/GetStudentWeekStats already filter for (teacher_weekly.sql.go
	// / teacher.sql.go's course_steps CTE). A mismatched literal here would make
	// course engagement silently read 0 in every teacher/parent analytics view.
	eventPayload, _ := json.Marshal(map[string]any{"ordinal": body.CurrentOrdinal})
	if err := store.LogCourseEvent(r.Context(), user.ID, courseID, "step_viewed", eventPayload); err != nil {
		slog.Warn("course progress: append step_viewed event failed", "err", err, "slug", slug)
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"progress": toCourseProgressDTO(slug, row)})
}

// postCourseQuizAnswer logs one quiz attempt. 铁律②: this NEVER gates —
// selected/correct are recorded as evidence regardless of correctness; the
// 200 response is unconditional.
func (a *API) postCourseQuizAnswer(w http.ResponseWriter, r *http.Request) {
	user, _ := UserFromContext(r.Context())
	slug := r.PathValue("slug")
	var body struct {
		StepID        string   `json:"stepId"`
		InteractionID string   `json:"interactionId"`
		Selected      []string `json:"selected"`
		Correct       bool     `json:"correct"`
	}
	if err := decodeJSON(r, &body); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if body.StepID == "" || body.InteractionID == "" {
		httpx.WriteError(w, r, httpx.ErrBadRequest("validation_failed", "stepId / interactionId 不能为空", nil))
		return
	}
	store := agent.NewSqlcAgentStore(a.d.Queries, a.d.Pool)
	_, courseID, err := store.GetCoursePayload(r.Context(), slug)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	eventPayload, err := json.Marshal(body)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if err := store.LogCourseEvent(r.Context(), user.ID, courseID, "course_quiz_answered", eventPayload); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// getCourseReport assembles CourseReportData (the store's fresh-computed
// completedStepTitles/cardIds/secondsSpent/quiz tally) with the three header
// fields the store doesn't carry (title/goal/teaching_thread), read from the
// same structure JSON GetCoursePayload already returns.
func (a *API) getCourseReport(w http.ResponseWriter, r *http.Request) {
	user, _ := UserFromContext(r.Context())
	slug := r.PathValue("slug")
	// ?attempt=<course_session id> selects a specific PAST run's frozen report
	// (the learning history's "看那一次的报告"); absent = the current attempt, the
	// live behavior. Owner/course scoping + a 404 for an unknown attempt live in
	// CourseReport20.
	attemptID := r.URL.Query().Get("attempt")
	store := agent.NewSqlcAgentStore(a.d.Queries, a.d.Pool)

	// A CourseDefinition-2.0 course has an empty legacy `structure`/`render_cache`
	// and tracks progress in course_session, not course_progress — so compute its
	// report from the definition + session. found=false means "legacy course",
	// which falls through to the legacy path below unchanged.
	if rep20, ok, err := store.CourseReport20(r.Context(), user.ID, slug, attemptID); err != nil {
		httpx.WriteError(w, r, err)
		return
	} else if ok {
		header := courseStructureHeader{Title: rep20.Title, CourseGoal: rep20.Goal}
		httpx.WriteJSON(w, http.StatusOK, map[string]any{"report": toCourseReportDTO(header, rep20.Data)})
		return
	}

	rep, err := store.CourseReport(r.Context(), user.ID, slug)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	payload, _, err := store.GetCoursePayload(r.Context(), slug)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	var header courseStructureHeader
	_ = json.Unmarshal(payload.Structure, &header)
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"report": toCourseReportDTO(header, rep)})
}

// getCourseAnswerReport returns the per-attempt answer detail — the student's
// actual recorded answers + per-slice time, read back from the stored
// course_session blob + the course definition (2.0 courses only; a legacy
// course returns an empty `slices` array). Lazy-loaded when the student opens
// the 小测/我的答案 drawer, so the main report fetch stays lean.
//
// ?attempt=<course_session id> selects a specific past run (the learning
// history's "看那一次的报告"); absent = the current (latest) attempt. Owner/course
// scoping + a 404 for an unknown attempt live in CourseAnswerReport20.
func (a *API) getCourseAnswerReport(w http.ResponseWriter, r *http.Request) {
	user, _ := UserFromContext(r.Context())
	slug := r.PathValue("slug")
	attemptID := r.URL.Query().Get("attempt")
	store := agent.NewSqlcAgentStore(a.d.Queries, a.d.Pool)

	rep, ok, err := store.CourseAnswerReport20(r.Context(), user.ID, slug, attemptID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if !ok {
		// Legacy course (no 2.0 definition) — no per-block answer detail exists.
		httpx.WriteJSON(w, http.StatusOK, map[string]any{"report": courseAnswerReportDTO{Slices: []courseAnswerSliceDTO{}}})
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"report": toCourseAnswerReportDTO(rep)})
}

// courseAskStructure is the narrow slice of `course.structure` postCourseAsk
// reads: the course's title/goal (for the prompt header) and, per step, only
// the title — a fallback for stepTitle when render_cache's own content.title
// (courseAskStepDigest's preferred source) is blank.
type courseAskStructure struct {
	Title      string `json:"title"`
	CourseGoal string `json:"course_goal"`
	Steps      []struct {
		Title string `json:"title"`
	} `json:"steps"`
}

// courseAskRenderCache is the narrow slice of `course.render_cache`
// courseAskStepDigest reads: one step's authored title/subtitle and its
// teaching-segment texts (RenderSegment.kind=="teaching" — the "structure"
// kind is the scaffold the STUDENT fills in via items, not authored prose to
// hand the coach, so it is deliberately skipped here).
type courseAskRenderCache struct {
	Steps []struct {
		Content struct {
			Title    string `json:"title"`
			Subtitle string `json:"subtitle"`
			Segments []struct {
				Kind string `json:"kind"`
				Text string `json:"text"`
			} `json:"segments"`
		} `json:"content"`
	} `json:"steps"`
}

// courseAskStepDigest returns the current step's title (render_cache's own
// content.title when present, else structureStepTitle — the authored
// structure step's title, always available even before any content is
// published) and a short text digest (subtitle + every teaching segment,
// blank-line joined) — just enough grounding for the coach prompt without
// shipping the whole authored step JSON into it. Out-of-range ordinal (a
// stale client, or a course with fewer rendered steps than structure steps)
// degrades to an empty digest rather than erroring — the coach can still
// answer from the course title/goal alone.
func courseAskStepDigest(renderCache json.RawMessage, structureStepTitle string, ordinal int) (title, digest string) {
	title = structureStepTitle
	var rc courseAskRenderCache
	_ = json.Unmarshal(renderCache, &rc)
	if ordinal < 0 || ordinal >= len(rc.Steps) {
		return title, ""
	}
	content := rc.Steps[ordinal].Content
	if content.Title != "" {
		title = content.Title
	}
	parts := make([]string, 0, len(content.Segments)+1)
	if content.Subtitle != "" {
		parts = append(parts, content.Subtitle)
	}
	for _, seg := range content.Segments {
		if seg.Kind == "teaching" && strings.TrimSpace(seg.Text) != "" {
			parts = append(parts, seg.Text)
		}
	}
	return title, strings.Join(parts, "\n\n")
}

// postCourseAsk drives one free-Q&A course-coach turn (Task 6): the student
// asks a free question about the CURRENT step ({slug, ordinal}); the coach
// answers once, guarded by 铁律①③ (agent.BuildCourseAskPrompt's guardrails —
// never the quiz answer, never conclude/write for the student, one question
// at a time). Mirrors chat.go's postChatTurn for the SSE scaffolding
// (entitlement gate before committing to the stream, NewSSEWriter, heartbeat,
// ErrorEnvelope-not-500 once committed) — there is no card/refeed loop here,
// course v2 has no card runtime left (agent/course_coach.go's package doc).
func (a *API) postCourseAsk(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")
	u, _ := UserFromContext(r.Context())

	// Entitlement gate — JSON error BEFORE committing to the stream.
	entitled, err := HasEntitlement(r.Context(), u)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if !entitled {
		httpx.WriteError(w, r, httpx.ErrNotEntitled())
		return
	}

	var body struct {
		Input   string `json:"input"`
		Ordinal int    `json:"ordinal"`
	}
	if err := decodeJSON(r, &body); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if body.Input == "" {
		httpx.WriteError(w, r, httpx.ErrBadRequest("validation_failed", "input 不能为空", nil))
		return
	}

	store := agent.NewSqlcAgentStore(a.d.Queries, a.d.Pool)
	payload, courseID, err := store.GetCoursePayload(r.Context(), slug)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	var structure courseAskStructure
	_ = json.Unmarshal(payload.Structure, &structure)
	structureStepTitle := ""
	if body.Ordinal >= 0 && body.Ordinal < len(structure.Steps) {
		structureStepTitle = structure.Steps[body.Ordinal].Title
	}
	stepTitle, stepText := courseAskStepDigest(payload.RenderCache, structureStepTitle, body.Ordinal)

	resolved, err := a.routeE(r.Context(), gateway.ClassDialogue) // chaperone (mid-tier) — 降级 allowed, per agent-spec §5.3
	if err != nil {
		httpx.WriteError(w, r, httpx.ErrInternal())
		return
	}

	// Commit to streaming. After this, errors are SSE error events, not JSON.
	sse, err := gateway.NewSSEWriter(w)
	if err != nil {
		httpx.WriteError(w, r, httpx.ErrInternal())
		return
	}
	em := &studioEmitter{sse: sse}
	stop, hbDone := startHeartbeat(r.Context(), em)
	defer func() {
		close(stop)
		<-hbDone
	}()

	// 铁律④: the question itself is evidence, logged BEFORE the coach turn —
	// unconditionally, regardless of whether the model call below succeeds,
	// errors, or is enforcement-rejected (friction IS the signal in the
	// reject case, not something to omit). Mirrors chat.go's postChatTurn,
	// which logs its own prompt_sent event before calling RunChatStep. A log
	// failure must not fail the turn that hasn't happened yet.
	// Event type is "course_message" (NOT "course_asked") — the vocabulary the
	// `turns` filter already counts (GetClassWeekStats/GetStudentWeekStats:
	// type IN ('prompt_sent','course_message')), so a course question counts
	// as a course dialogue turn instead of silently reading 0.
	eventPayload, perr := json.Marshal(map[string]any{"input": body.Input, "ordinal": body.Ordinal})
	if perr != nil {
		eventPayload = []byte(`{}`)
	}
	if err := store.LogCourseEvent(r.Context(), u.ID, courseID, "course_message", eventPayload); err != nil {
		slog.Warn("course ask: append course_message event failed",
			"err", err, "request_id", httpx.RequestIDFromContext(r.Context()))
	}

	out, usage, aerr := agent.ProposeCourseAskReply(r.Context(), a.d.Provider, resolved,
		structure.Title, stepTitle, structure.CourseGoal, stepText, body.Input)
	// Meter regardless of aerr: enforcement can reject AFTER the tokens were
	// already spent (agent.ProposeCourseAskReply's own doc comment) — usage
	// is populated whenever the model call itself succeeded.
	if usage.InputTokens > 0 || usage.OutputTokens > 0 {
		if merr := store.RecordCourseLLMCall(r.Context(), u.ID, "coach", resolved, int32(usage.InputTokens), int32(usage.OutputTokens)); merr != nil {
			slog.Warn("course ask: record llm usage failed",
				"err", merr, "request_id", httpx.RequestIDFromContext(r.Context()))
		}
	}
	if aerr != nil {
		slog.Warn("course ask: reply not emitted",
			"err", aerr, "request_id", httpx.RequestIDFromContext(r.Context()))
		_ = em.ErrorEnvelope("internal_error", "对话处理失败，请重试")
		_ = em.Done()
		return
	}

	_ = em.Text(out.Body)
	_ = em.Done()
}
