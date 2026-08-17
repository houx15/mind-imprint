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

func (a *API) listCourses(w http.ResponseWriter, r *http.Request) {
	store := agent.NewSqlcAgentStore(a.d.Queries, a.d.Pool)
	rows, err := store.ListCourses(r.Context(), isAdmin(r.Context()))
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	out := make([]courseSummaryDTO, 0, len(rows))
	for _, c := range rows {
		out = append(out, a.toCourseSummaryDTO(c))
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
	store := agent.NewSqlcAgentStore(a.d.Queries, a.d.Pool)

	// A CourseDefinition-2.0 course has an empty legacy `structure`/`render_cache`
	// and tracks progress in course_session, not course_progress — so compute its
	// report from the definition + session. found=false means "legacy course",
	// which falls through to the legacy path below unchanged.
	if rep20, ok, err := store.CourseReport20(r.Context(), user.ID, slug); err != nil {
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

	resolved, err := a.d.ChatResolver(r.Context()) // chaperone (mid-tier) — 降级 allowed, per agent-spec §5.3
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
