package api

// course_session.go — Task 7 (Slice 12): the Course surface's session HTTP
// endpoints. Course sessions are project-free and thread-free — every
// handler here is keyed by {id} = COURSE id (never a session id: the
// UNIQUE(user_id, course_id) constraint means one caller has at most one
// session per course, so the session is always resolved from (caller,
// course_id), never addressed directly). Session get/ask/advance/card
// submit/skip all require an EXISTING session (404 if the caller has none —
// including when a DIFFERENT student's session for the same course exists,
// ownership hidden as not-found, never 403); only the start endpoint is
// get-or-create. The two SSE turn handlers share runCourseTurn, mirroring
// postChatTurn's scaffolding; card submit/skip mirror submitChatCard/
// skipChatCard verbatim (thin: no CompleteCard, no graph_effects, no refeed,
// no competence write — card_competence is dormant platform-wide).
import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"mindimprint/api/internal/agent"
	"mindimprint/api/internal/gateway"
	"mindimprint/api/internal/httpx"
	"mindimprint/api/internal/skills"
	"mindimprint/api/internal/store/sqlc"
)

// courseSkillID is the one course skill this slice seeds (non-goal: multi-
// course authoring — one skill, one seeded course, per spec §9).
const courseSkillID = "info-literacy-course"

// loadOwnedSession resolves {id} (a course id) to the CALLER's course_session,
// 404 (never 403) when none exists for this user+course — including when a
// DIFFERENT student's session for the same course exists. Same ownership-
// hidden-as-not-found convention as loadOwnedThread, over course_session
// instead of chat_thread.
func (a *API) loadOwnedSession(w http.ResponseWriter, r *http.Request) (sqlc.CourseSession, bool) {
	courseID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		httpx.WriteError(w, r, httpx.ErrNotFound("资源不存在"))
		return sqlc.CourseSession{}, false
	}
	u, _ := UserFromContext(r.Context())
	sess, err := a.d.Queries.GetCourseSessionByUserCourse(r.Context(), sqlc.GetCourseSessionByUserCourseParams{
		UserID: u.ID, CourseID: courseID,
	})
	if err != nil {
		httpx.WriteError(w, r, err) // pgx.ErrNoRows → 404
		return sqlc.CourseSession{}, false
	}
	return sess, true
}

// loadOwnedSessionCard scopes {cid} to the caller's owned session (404-no-
// leak) — mirrors loadOwnedThreadCard over course_session/card_instance
// instead of chat_thread/card_instance.
func (a *API) loadOwnedSessionCard(w http.ResponseWriter, r *http.Request) (sqlc.CourseSession, uuid.UUID, bool) {
	sess, ok := a.loadOwnedSession(w, r)
	if !ok {
		return sqlc.CourseSession{}, uuid.UUID{}, false
	}
	cid, err := uuid.Parse(r.PathValue("cid"))
	if err != nil {
		httpx.WriteError(w, r, httpx.ErrNotFound("资源不存在"))
		return sqlc.CourseSession{}, uuid.UUID{}, false
	}
	cis, err := a.d.Queries.ListCardInstancesBySession(r.Context(), pgtype.UUID{Bytes: sess.ID, Valid: true})
	if err != nil {
		httpx.WriteError(w, r, err)
		return sqlc.CourseSession{}, uuid.UUID{}, false
	}
	for _, ci := range cis {
		if ci.ID == cid {
			return sess, cid, true
		}
	}
	httpx.WriteError(w, r, httpx.ErrNotFound("资源不存在"))
	return sqlc.CourseSession{}, uuid.UUID{}, false
}

// buildCourseSessionDTO assembles the session DTO: the row itself, the
// current phase's title resolved from the skill (phaseTitle comes from
// Contract.Title — spec §5), the session's full dialogue, and any card offer
// still open (Slice-12 whole-branch Critical-2: without this, a reload
// during `guided` erased the offer and the card_dispositioned floor could
// never be met again).
func (a *API) buildCourseSessionDTO(ctx context.Context, sess sqlc.CourseSession, sk skills.Skill) (CourseSessionDTO, error) {
	rows, err := a.d.Queries.ListMessagesBySession(ctx, sess.ID)
	if err != nil {
		return CourseSessionDTO{}, err
	}
	messages := make([]CourseMessageDTO, 0, len(rows))
	for _, m := range rows {
		messages = append(messages, toCourseMessageDTO(m))
	}
	offers, err := agent.NewSqlcCourseStore(a.d.Queries).OpenCardOffers(ctx, sess.ID)
	if err != nil {
		return CourseSessionDTO{}, err
	}
	openCards := make([]CourseCardOfferDTO, 0, len(offers))
	for _, o := range offers {
		openCards = append(openCards, CourseCardOfferDTO{
			CardInstanceID: o.CardInstanceID.String(),
			CardID:         o.CardID,
			MaterialID:     o.MaterialID.String(),
		})
	}
	phaseTitle := ""
	if c, ok := sk.Contracts[sess.Phase]; ok {
		phaseTitle = c.Title
	}
	return toCourseSessionDTO(sess, phaseTitle, messages, openCards), nil
}

// startCourseSession gets-or-creates the caller's session for course {id}.
// Idempotent on the UNIQUE(user_id, course_id) constraint (CreateCourseSession
// is an upsert) — re-entering a course resumes it rather than restarting it.
func (a *API) startCourseSession(w http.ResponseWriter, r *http.Request) {
	courseID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		httpx.WriteError(w, r, httpx.ErrNotFound("资源不存在"))
		return
	}
	u, _ := UserFromContext(r.Context())

	sk, ok := skills.ByID(courseSkillID)
	if !ok {
		httpx.WriteError(w, r, httpx.ErrInternal())
		return
	}
	order, err := sk.LinearOrder()
	if err != nil || len(order) == 0 {
		httpx.WriteError(w, r, httpx.ErrInternal())
		return
	}

	sess, err := a.d.Queries.CreateCourseSession(r.Context(), sqlc.CreateCourseSessionParams{
		UserID: u.ID, CourseID: courseID, SkillID: sk.ID, Phase: order[0],
	})
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	dto, err := a.buildCourseSessionDTO(r.Context(), sess, sk)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, dto)
}

// getCourseSession returns the caller's existing session (404 if none).
func (a *API) getCourseSession(w http.ResponseWriter, r *http.Request) {
	sess, ok := a.loadOwnedSession(w, r)
	if !ok {
		return
	}
	sk, ok := skills.ByID(sess.SkillID)
	if !ok {
		httpx.WriteError(w, r, httpx.ErrInternal())
		return
	}
	dto, err := a.buildCourseSessionDTO(r.Context(), sess, sk)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, dto)
}

// runCourseTurn is the shared body of ask + advance: entitlement gate BEFORE
// committing to the stream, then RunCourseStep, then the frames. Mirrors
// postChatTurn's scaffolding (studioEmitter + startHeartbeat). RunCourseStep
// itself persists the student turn (unlike postChatTurn, which persists it
// inline before calling RunChatStep) — course_step.go's runCourseAsk already
// writes the course_message row, so no separate persist step belongs here.
func (a *API) runCourseTurn(w http.ResponseWriter, r *http.Request, intent agent.CourseIntent) {
	sess, ok := a.loadOwnedSession(w, r)
	if !ok {
		return
	}
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

	// advance decodes NO body at all — floor inputs are all server-side
	// (DEC-12.2): a floor the client can assert is not a floor.
	var userInput string
	if intent == agent.CourseIntent("ask") {
		var body struct {
			UserInput string `json:"user_input"`
		}
		if err := decodeJSON(r, &body); err != nil {
			httpx.WriteError(w, r, err)
			return
		}
		if len(body.UserInput) == 0 {
			httpx.WriteError(w, r, httpx.ErrBadRequest("validation_failed", "user_input 不能为空", nil))
			return
		}
		userInput = body.UserInput
	}

	resolved, err := a.d.ChatResolver(r.Context())
	if err != nil {
		httpx.WriteError(w, r, httpx.ErrInternal())
		return
	}
	sk, ok := skills.ByID(sess.SkillID)
	if !ok {
		httpx.WriteError(w, r, httpx.ErrInternal())
		return
	}
	course, err := a.d.Queries.GetCourse(r.Context(), sess.CourseID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}

	// Commit to streaming. After this, errors are SSE error events, not JSON.
	sse, err := gateway.NewSSEWriter(w)
	if err != nil {
		httpx.WriteError(w, r, httpx.ErrInternal())
		return
	}
	em := &studioEmitter{sse: sse}

	// Heartbeat until the turn returns or the client disconnects.
	stop, hbDone := startHeartbeat(r.Context(), em)
	defer func() {
		close(stop)
		<-hbDone
	}()

	deps := agent.CourseDeps{
		Store:       agent.NewSqlcCourseStore(a.d.Queries),
		Provider:    a.d.Provider,
		Resolved:    resolved,
		Skill:       sk,
		CourseTitle: course.Title,
		UserID:      u.ID,
		SessionID:   sess.ID,
	}
	res, err := agent.RunCourseStep(r.Context(), deps, intent, userInput)
	if err != nil {
		slog.Error("course turn: RunCourseStep",
			"err", err, "request_id", httpx.RequestIDFromContext(r.Context()))
		_ = em.ErrorEnvelope("internal_error", "对话处理失败，请重试")
		_ = em.Done()
		return
	}

	// Guard Text on a non-empty reply — an empty frame renders an empty
	// bubble (a Slice-11 whole-branch finding). RunCourseStep returns an
	// empty Reply on enforcement reject (silence, by design), so this fires.
	if res.Reply != "" {
		_ = em.Text(res.Reply)
	}
	if res.Offer != nil {
		_ = em.Card(res.Offer.CardInstanceID.String(), res.Offer.CardID, "", []byte("[]"), res.Offer.MaterialID.String())
	}
	if res.Advanced != "" {
		_ = em.Phase(res.Advanced)
	}
	_ = em.Done()
}

// postCourseAsk drives one RunCourseStep with intent "ask".
func (a *API) postCourseAsk(w http.ResponseWriter, r *http.Request) {
	a.runCourseTurn(w, r, agent.CourseIntent("ask"))
}

// postCourseAdvance drives one RunCourseStep with intent "request_advance".
// No body decoded (see runCourseTurn's doc comment).
func (a *API) postCourseAdvance(w http.ResponseWriter, r *http.Request) {
	a.runCourseTurn(w, r, agent.CourseIntent("request_advance"))
}

// submitCourseCard persists a filled session-scoped card envelope and marks
// it completed. Thin by design, mirroring submitChatCard verbatim: no
// CompleteCard, no graph_effects, no refeed, no competence write.
func (a *API) submitCourseCard(w http.ResponseWriter, r *http.Request) {
	sess, cid, ok := a.loadOwnedSessionCard(w, r)
	if !ok {
		return
	}
	u, _ := UserFromContext(r.Context())

	var body struct {
		FieldValues json.RawMessage `json:"field_values"`
		EventTrace  json.RawMessage `json:"event_trace"`
		Anchors     json.RawMessage `json:"anchors"`
	}
	if err := decodeJSON(r, &body); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if err := validateFieldValues(body.FieldValues); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if err := validateEventTrace(body.EventTrace); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if err := validateAnchors(body.Anchors); err != nil {
		httpx.WriteError(w, r, err)
		return
	}

	if _, err := a.d.Queries.SubmitSessionCardInstance(r.Context(), sqlc.SubmitSessionCardInstanceParams{
		ID: cid, SessionID: pgtype.UUID{Bytes: sess.ID, Valid: true},
		FieldValues: body.FieldValues, EventTrace: body.EventTrace, Status: "completed",
	}); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if _, err := a.d.Queries.AppendEvent(r.Context(), sqlc.AppendEventParams{
		ProjectID: pgtype.UUID{Valid: false}, UserID: u.ID,
		Surface: "course", Type: "card_completed", Payload: []byte(`{}`),
	}); err != nil {
		slog.Warn("course card submit: append card_completed event failed",
			"err", err, "request_id", httpx.RequestIDFromContext(r.Context()))
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]string{"card_status": "completed"})
}

// skipCourseCard records a skipped session-scoped card — process-is-data
// (design's 「过程即数据」), same posture as skipChatCard, minus the
// event_trace body (thin by design — see submitCourseCard's doc comment).
func (a *API) skipCourseCard(w http.ResponseWriter, r *http.Request) {
	sess, cid, ok := a.loadOwnedSessionCard(w, r)
	if !ok {
		return
	}
	if _, err := a.d.Queries.SetSessionCardInstanceStatus(r.Context(), sqlc.SetSessionCardInstanceStatusParams{
		ID: cid, SessionID: pgtype.UUID{Bytes: sess.ID, Valid: true}, Status: "skipped",
	}); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]string{"card_status": "skipped"})
}
