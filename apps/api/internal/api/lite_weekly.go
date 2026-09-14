package api

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"mindimprint/api/internal/agent"
	"mindimprint/api/internal/gateway"
	"mindimprint/api/internal/httpx"
	"mindimprint/api/internal/liteassign"
	"mindimprint/api/internal/liteweek"
	"mindimprint/api/internal/liteweekly"
	"mindimprint/api/internal/store/sqlc"
)

// loadLiteClassWeek gathers the facts of one completed Beijing week for every
// student currently enrolled in the class. Read-only: it never generates a
// report and never calls a model.
func (a *API) loadLiteClassWeek(ctx context.Context, classID uuid.UUID, weekStart time.Time) ([]liteweekly.StudentWeek, error) {
	members, err := a.d.Queries.ListLiteWeekClassStudents(ctx, classID)
	if err != nil {
		return nil, err
	}
	return a.loadLiteWeeks(ctx, classID, members, weekStart)
}

// loadLiteStudentWeek is loadLiteClassWeek for one student. A user who is not
// a current student member of the class gets pgx.ErrNoRows.
func (a *API) loadLiteStudentWeek(ctx context.Context, classID, userID uuid.UUID, weekStart time.Time) (liteweekly.StudentWeek, error) {
	s, _, err := a.loadLiteStudentWeekWithRoster(ctx, classID, userID, weekStart)
	return s, err
}

// loadLiteStudentWeekWithRoster is loadLiteStudentWeek that also returns the
// class's student members, so the prose handler can name the other students
// without a second roster query.
func (a *API) loadLiteStudentWeekWithRoster(ctx context.Context, classID, userID uuid.UUID, weekStart time.Time) (liteweekly.StudentWeek, []sqlc.ListLiteWeekClassStudentsRow, error) {
	members, err := a.d.Queries.ListLiteWeekClassStudents(ctx, classID)
	if err != nil {
		return liteweekly.StudentWeek{}, nil, err
	}
	for _, m := range members {
		if m.ID != userID {
			continue
		}
		weeks, err := a.loadLiteWeeks(ctx, classID, []sqlc.ListLiteWeekClassStudentsRow{m}, weekStart)
		if err != nil {
			return liteweekly.StudentWeek{}, nil, err
		}
		return weeks[0], members, nil
	}
	return liteweekly.StudentWeek{}, nil, pgx.ErrNoRows
}

// loadLiteWeeks runs each fact query once for all members and groups the rows
// by user id. The result keeps the members' order.
func (a *API) loadLiteWeeks(ctx context.Context, classID uuid.UUID, members []sqlc.ListLiteWeekClassStudentsRow, weekStart time.Time) ([]liteweekly.StudentWeek, error) {
	ws := weekStart.In(liteweek.Beijing)
	we := ws.AddDate(0, 0, 7)
	prev := ws.AddDate(0, 0, -7)

	ids := make([]uuid.UUID, 0, len(members))
	byID := make(map[uuid.UUID]*liteweekly.StudentWeek, len(members))
	out := make([]liteweekly.StudentWeek, len(members))
	for i, m := range members {
		ids = append(ids, m.ID)
		out[i] = liteweekly.StudentWeek{
			UserID: m.ID.String(), Name: m.DisplayName, Minutes: -1,
			Finished: []liteweekly.Item{}, Stalled: []liteweekly.Item{},
			NewKeywords: []string{}, Moments: []liteweekly.Moment{},
		}
		byID[m.ID] = &out[i]
	}
	if len(ids) == 0 {
		return out, nil
	}
	q := a.d.Queries

	activity, err := q.ListLiteWeekActivity(ctx, sqlc.ListLiteWeekActivityParams{
		UserIds: ids, WeekStart: ws, WeekEnd: we, PrevStart: prev,
		WeekStartDay: pgDate(ws), WeekEndDay: pgDate(we), PrevStartDay: pgDate(prev),
	})
	if err != nil {
		return nil, err
	}
	for _, r := range activity {
		s := byID[r.UserID]
		if s == nil {
			continue
		}
		s.ActiveDays = int(r.ActiveDays)
		s.PrevActiveDays = int(r.PrevActiveDays)
		s.Turns = int(r.Turns)
		if r.HasAnyBucketByWeekEnd {
			s.Minutes = int(r.Seconds) / 60
		}
	}

	finished, err := q.ListLiteWeekFinished(ctx, sqlc.ListLiteWeekFinishedParams{UserIds: ids, WeekStart: ws, WeekEnd: we})
	if err != nil {
		return nil, err
	}
	for _, r := range finished {
		if s := byID[r.UserID]; s != nil {
			s.Finished = append(s.Finished, liteweekly.Item{Kind: r.Kind, Title: r.Title})
		}
	}

	states, err := q.ListLiteWeekAssignmentStates(ctx, sqlc.ListLiteWeekAssignmentStatesParams{
		ClassID: classID, UserIds: ids, WeekStart: ws, WeekEnd: we,
	})
	if err != nil {
		return nil, err
	}
	for _, r := range states {
		s := byID[r.UserID]
		if s == nil {
			continue
		}
		// Work finished after the week ended does not count as finished for
		// this week: at week end it was still overdue.
		fin := tsPtr(r.FinishedAt)
		if fin != nil && !fin.Before(we) {
			fin = nil
		}
		switch liteassign.Status(r.StartedAt.Valid, fin, r.DueAt, we) {
		case "done":
			s.AssignmentsDone++
		case "done_late":
			s.AssignmentsLate++
		case "overdue":
			s.AssignmentsOverdue++
		}
	}

	stalled, err := q.ListLiteWeekStalled(ctx, sqlc.ListLiteWeekStalledParams{UserIds: ids, WeekEnd: we, WeekEndDay: pgDate(we)})
	if err != nil {
		return nil, err
	}
	for _, r := range stalled {
		if s := byID[r.UserID]; s != nil {
			s.Stalled = append(s.Stalled, liteweekly.Item{Kind: r.Kind, Title: r.Title})
		}
	}

	keywords, err := q.ListLiteWeekNewKeywords(ctx, sqlc.ListLiteWeekNewKeywordsParams{UserIds: ids, WeekStart: ws, WeekEnd: we})
	if err != nil {
		return nil, err
	}
	for _, r := range keywords {
		if s := byID[r.UserID]; s != nil {
			s.NewKeywords = append(s.NewKeywords, r.TextZh)
		}
	}

	moments, err := q.ListLiteWeekMoments(ctx, sqlc.ListLiteWeekMomentsParams{UserIds: ids, WeekStart: ws, WeekEnd: we})
	if err != nil {
		return nil, err
	}
	for _, r := range moments {
		s := byID[r.UserID]
		if s == nil || r.ProsePending {
			continue
		}
		ms, err := weekMoments(r.Moments, r.AssignedPrompt)
		if err != nil {
			// One unreadable stored report must not hide the whole class's week.
			slog.Warn("lite weekly: unreadable report moments, skipped", "err", err, "atom_id", r.AtomID)
			continue
		}
		for _, quote := range ms {
			s.Moments = append(s.Moments, liteweekly.Moment{Quote: quote, ItemTitle: r.Title})
		}
	}
	return out, nil
}

// minPromptOverlapRunes is the shortest quote that is dropped for being part of
// the teacher's assigned prompt. Shorter quotes (「雨」) are too likely to be her
// own words that happen to share a word with the prompt.
const minPromptOverlapRunes = 8

// weekMoments reads a report's stored moments. The quotes were checked against
// her own words when the report was built, so they are taken as-is. An empty
// quote is skipped. A quote is treated as the teacher's words and skipped when
// it equals the assigned prompt, or when it is at least minPromptOverlapRunes
// long and part of the prompt.
func weekMoments(raw []byte, assignedPrompt *string) ([]string, error) {
	var ms []reportMoment
	if err := json.Unmarshal(raw, &ms); err != nil {
		return nil, err
	}
	prompt := ""
	if assignedPrompt != nil {
		prompt = strings.TrimSpace(*assignedPrompt)
	}
	out := make([]string, 0, len(ms))
	for _, m := range ms {
		q := strings.TrimSpace(m.Quote)
		if q == "" {
			continue
		}
		if prompt != "" && (q == prompt || (utf8.RuneCountInString(q) >= minPromptOverlapRunes && strings.Contains(prompt, q))) {
			continue
		}
		out = append(out, m.Quote)
	}
	return out, nil
}

// --- handlers --------------------------------------------------------------
//
// Four endpoints: a GET and a POST …/prose for one student's week, and the
// same pair for the class. A GET reads facts and any stored prose and never
// calls a model. A POST returns the stored prose when there is one (no
// entitlement check, no model call); otherwise it checks entitlement, calls
// the assess class once, records every attempt, and stores the prose only
// when it passed validation. Prose that failed validation is returned as
// proseError with prose null, and nothing is stored.

const (
	liteStudentWeeklyPurpose = "lite_student_weekly"
	liteClassWeeklyPurpose   = "lite_class_weekly"
)

// errInvalidWeek is the 400 for a ?weekStart that is not the Beijing Monday
// of a week that has already ended.
func errInvalidWeek() *httpx.APIError {
	return &httpx.APIError{Status: http.StatusBadRequest, Code: "invalid_week", Message: "请选择已经结束的一周"}
}

// The 400 messages for a week that ended at or before the reporting starts:
// her enrollment in the class (student routes) or the class's creation
// (class routes).
const (
	liteWeekBeforeEnrollment = "该周早于学生加入班级的时间"
	liteWeekBeforeClass      = "该周早于班级创建的时间"
)

func errWeekBeforeStart(message string) *httpx.APIError {
	return &httpx.APIError{Status: http.StatusBadRequest, Code: "week_before_start", Message: message}
}

// liteWeekBound checks the week starting at ws against start. A week whose
// end is at or before start is out of range (inRange false). hasPrev says
// whether the week before ws is in range, i.e. whether its end (ws) is after
// start.
func liteWeekBound(ws, start time.Time) (hasPrev, inRange bool) {
	if !ws.AddDate(0, 0, 7).After(start) {
		return false, false
	}
	return ws.After(start), true
}

// parseLiteStudentWeek is parseLiteWeek plus the lower bound of a student
// route: her enrollment in this class. ok=false means the error is already
// written.
func (a *API) parseLiteStudentWeek(w http.ResponseWriter, r *http.Request, classID, userID uuid.UUID) (ws time.Time, isLatest, hasPrev, ok bool) {
	ws, isLatest, ok = parseLiteWeek(w, r)
	if !ok {
		return time.Time{}, false, false, false
	}
	enr, err := a.d.Queries.GetEnrollment(r.Context(), sqlc.GetEnrollmentParams{UserID: userID, ClassID: classID})
	if err != nil {
		httpx.WriteError(w, r, err)
		return time.Time{}, false, false, false
	}
	hasPrev, inRange := liteWeekBound(ws, enr.CreatedAt)
	if !inRange {
		httpx.WriteError(w, r, errWeekBeforeStart(liteWeekBeforeEnrollment))
		return time.Time{}, false, false, false
	}
	return ws, isLatest, hasPrev, true
}

// parseLiteClassWeek is parseLiteWeek plus the lower bound of a class route:
// the class's creation. ok=false means the error is already written.
func parseLiteClassWeek(w http.ResponseWriter, r *http.Request, cls sqlc.Class) (ws time.Time, isLatest, hasPrev, ok bool) {
	ws, isLatest, ok = parseLiteWeek(w, r)
	if !ok {
		return time.Time{}, false, false, false
	}
	hasPrev, inRange := liteWeekBound(ws, cls.CreatedAt)
	if !inRange {
		httpx.WriteError(w, r, errWeekBeforeStart(liteWeekBeforeClass))
		return time.Time{}, false, false, false
	}
	return ws, isLatest, hasPrev, true
}

type liteWeekItemDTO struct {
	Kind  string `json:"kind"`
	Title string `json:"title"`
}

type liteWeekMomentDTO struct {
	Quote     string `json:"quote"`
	ItemTitle string `json:"itemTitle"`
}

type liteStudentWeekFactsDTO struct {
	ActiveDays         int                 `json:"activeDays"`
	Minutes            int                 `json:"minutes"` // -1 = no time records in or before this week
	Turns              int                 `json:"turns"`
	PrevActiveDays     int                 `json:"prevActiveDays"`
	Finished           []liteWeekItemDTO   `json:"finished"`
	AssignmentsDone    int                 `json:"assignmentsDone"`
	AssignmentsLate    int                 `json:"assignmentsLate"`
	AssignmentsOverdue int                 `json:"assignmentsOverdue"`
	Stalled            []liteWeekItemDTO   `json:"stalled"`
	NewKeywords        []string            `json:"newKeywords"`
	Moments            []liteWeekMomentDTO `json:"moments"`
}

type liteWeekCardDTO struct {
	Kind     string `json:"kind"`
	Code     string `json:"code"`
	Label    string `json:"label"`
	Evidence string `json:"evidence"`
}

type liteStudentWeeklyDTO struct {
	WeekStart  string                        `json:"weekStart"`
	WeekLabel  string                        `json:"weekLabel"`
	Title      string                        `json:"title"`
	IsLatest   bool                          `json:"isLatest"`
	HasPrev    bool                          `json:"hasPrev"` // the week before ended after her enrollment
	Empty      bool                          `json:"empty"`   // liteweekly.IsEmptyWeek: no prose is generated
	Facts      liteStudentWeekFactsDTO       `json:"facts"`
	Cards      []liteWeekCardDTO             `json:"cards"`
	Prose      *agent.LiteStudentWeeklyProse `json:"prose"`
	ProseReady bool                          `json:"proseReady"`
}

type liteStudentWeeklyProseDTO struct {
	liteStudentWeeklyDTO
	ProseError *string `json:"proseError"`
}

type liteClassWeekStatsDTO struct {
	ClassSize      int `json:"classSize"`
	ActiveStudents int `json:"activeStudents"`
	Minutes        int `json:"minutes"`
	Turns          int `json:"turns"`
	Finished       int `json:"finished"`
	AssignmentRate int `json:"assignmentRate"` // integer percent; -1 when nothing was due
}

type liteClassWeekCardDTO struct {
	liteWeekCardDTO
	UserID string `json:"userId"`
	Name   string `json:"name"`
}

type liteClassWeeklyDTO struct {
	WeekStart  string                      `json:"weekStart"`
	WeekLabel  string                      `json:"weekLabel"`
	Title      string                      `json:"title"`
	IsLatest   bool                        `json:"isLatest"`
	HasPrev    bool                        `json:"hasPrev"` // the week before ended after the class was created
	Empty      bool                        `json:"empty"`   // every student's week is empty, or no students: no prose is generated
	Stats      liteClassWeekStatsDTO       `json:"stats"`
	Praise     []liteClassWeekCardDTO      `json:"praise"`
	Watch      []liteClassWeekCardDTO      `json:"watch"`
	Prose      *agent.LiteClassWeeklyProse `json:"prose"`
	ProseReady bool                        `json:"proseReady"`
}

type liteClassWeeklyProseDTO struct {
	liteClassWeeklyDTO
	ProseError *string `json:"proseError"`
}

// parseLiteWeek reads ?weekStart. ok=false means the error is already written.
func parseLiteWeek(w http.ResponseWriter, r *http.Request) (weekStart time.Time, isLatest bool, ok bool) {
	now := time.Now()
	ws, err := liteweek.ParseWeekStart(r.URL.Query().Get("weekStart"), now)
	if err != nil {
		if errors.Is(err, liteweek.ErrBadWeek) {
			httpx.WriteError(w, r, errInvalidWeek())
		} else {
			httpx.WriteError(w, r, err)
		}
		return time.Time{}, false, false
	}
	return ws, ws.Equal(liteweek.LatestCompleted(now)), true
}

func liteWeekStartString(ws time.Time) string {
	return ws.In(liteweek.Beijing).Format("2006-01-02")
}

// liteWeekCards is liteweekly.Cards as a slice: watch first, then praise.
func liteWeekCards(s liteweekly.StudentWeek) []liteweekly.Card {
	watch, praise := liteweekly.Cards(s)
	out := make([]liteweekly.Card, 0, 2)
	if watch != nil {
		out = append(out, *watch)
	}
	if praise != nil {
		out = append(out, *praise)
	}
	return out
}

func liteWeekCardDTOOf(c liteweekly.Card) liteWeekCardDTO {
	return liteWeekCardDTO{Kind: c.Kind, Code: c.Code, Label: c.Label, Evidence: c.Evidence}
}

func liteWeekItemDTOs(items []liteweekly.Item) []liteWeekItemDTO {
	out := make([]liteWeekItemDTO, 0, len(items))
	for _, it := range items {
		out = append(out, liteWeekItemDTO{Kind: it.Kind, Title: it.Title})
	}
	return out
}

func liteStudentWeeklyView(s liteweekly.StudentWeek, ws time.Time, isLatest, hasPrev bool, prose *agent.LiteStudentWeeklyProse) liteStudentWeeklyDTO {
	label := liteweek.Label(ws)
	title := "表现总结 · " + label
	if isLatest {
		title = "上周表现总结 · " + label
	}
	cards := liteWeekCards(s)
	cardDTOs := make([]liteWeekCardDTO, 0, len(cards))
	for _, c := range cards {
		cardDTOs = append(cardDTOs, liteWeekCardDTOOf(c))
	}
	moments := make([]liteWeekMomentDTO, 0, len(s.Moments))
	for _, m := range s.Moments {
		moments = append(moments, liteWeekMomentDTO{Quote: m.Quote, ItemTitle: m.ItemTitle})
	}
	keywords := s.NewKeywords
	if keywords == nil {
		keywords = []string{}
	}
	return liteStudentWeeklyDTO{
		WeekStart: liteWeekStartString(ws), WeekLabel: label, Title: title, IsLatest: isLatest,
		HasPrev: hasPrev, Empty: liteweekly.IsEmptyWeek(s),
		Facts: liteStudentWeekFactsDTO{
			ActiveDays: s.ActiveDays, Minutes: s.Minutes, Turns: s.Turns, PrevActiveDays: s.PrevActiveDays,
			Finished:        liteWeekItemDTOs(s.Finished),
			AssignmentsDone: s.AssignmentsDone, AssignmentsLate: s.AssignmentsLate, AssignmentsOverdue: s.AssignmentsOverdue,
			Stalled:     liteWeekItemDTOs(s.Stalled),
			NewKeywords: keywords,
			Moments:     moments,
		},
		Cards:      cardDTOs,
		Prose:      prose,
		ProseReady: prose != nil,
	}
}

// weeklyProseFromRow decodes a stored prose body. A missing row is (nil, nil).
func weeklyProseFromRow[T any](body []byte, err error) (*T, error) {
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var p T
	if err := json.Unmarshal(body, &p); err != nil {
		return nil, err
	}
	return &p, nil
}

func (a *API) storedLiteStudentProse(ctx context.Context, userID uuid.UUID, ws time.Time) (*agent.LiteStudentWeeklyProse, error) {
	row, err := a.d.Queries.GetLiteStudentWeeklyProse(ctx, sqlc.GetLiteStudentWeeklyProseParams{UserID: userID, WeekStart: pgDate(ws)})
	return weeklyProseFromRow[agent.LiteStudentWeeklyProse](row.Body, err)
}

func (a *API) storedLiteClassProse(ctx context.Context, classID uuid.UUID, ws time.Time) (*agent.LiteClassWeeklyProse, error) {
	row, err := a.d.Queries.GetLiteClassWeeklyProse(ctx, sqlc.GetLiteClassWeeklyProseParams{ClassID: classID, WeekStart: pgDate(ws)})
	return weeklyProseFromRow[agent.LiteClassWeeklyProse](row.Body, err)
}

// requireTeacherEntitled runs the HasEntitlement seam for the request user
// before a model call. ok=false means the error is already written.
func requireTeacherEntitled(w http.ResponseWriter, r *http.Request) (User, bool) {
	u, _ := UserFromContext(r.Context())
	entitled, err := HasEntitlement(r.Context(), u)
	if err != nil {
		httpx.WriteError(w, r, err)
		return User{}, false
	}
	if !entitled {
		httpx.WriteError(w, r, httpx.ErrNotEntitled())
		return User{}, false
	}
	return u, true
}

// getLiteStudentWeekly handles
// GET /api/v1/lite/teacher/classes/{id}/students/{userId}/weekly?weekStart=.
// Never calls a model.
func (a *API) getLiteStudentWeekly(w http.ResponseWriter, r *http.Request) {
	classID, userID, ok := a.authTeacherStudent(w, r)
	if !ok {
		return
	}
	ws, isLatest, hasPrev, ok := a.parseLiteStudentWeek(w, r, classID, userID)
	if !ok {
		return
	}
	ctx := r.Context()
	s, err := a.loadLiteStudentWeek(ctx, classID, userID, ws)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	prose, err := a.storedLiteStudentProse(ctx, userID, ws)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, liteStudentWeeklyView(s, ws, isLatest, hasPrev, prose))
}

// postLiteStudentWeeklyProse handles
// POST /api/v1/lite/teacher/classes/{id}/students/{userId}/weekly/prose?weekStart=.
//
// Prose is keyed by (user_id, week_start), not by class: a student in two lite
// classes shares one row. Two concurrent POSTs may both call the model; the
// insert is ON CONFLICT DO NOTHING and both responses return the stored row.
func (a *API) postLiteStudentWeeklyProse(w http.ResponseWriter, r *http.Request) {
	classID, userID, ok := a.authTeacherStudent(w, r)
	if !ok {
		return
	}
	ws, isLatest, hasPrev, ok := a.parseLiteStudentWeek(w, r, classID, userID)
	if !ok {
		return
	}
	ctx := r.Context()
	stored, err := a.storedLiteStudentProse(ctx, userID, ws)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	s, members, err := a.loadLiteStudentWeekWithRoster(ctx, classID, userID, ws)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if stored != nil {
		httpx.WriteJSON(w, http.StatusOK, liteStudentWeeklyProseDTO{liteStudentWeeklyView(s, ws, isLatest, hasPrev, stored), nil})
		return
	}
	// An empty week has nothing to summarise: no entitlement check, no model
	// call, nothing stored. prose and proseError are both null.
	if liteweekly.IsEmptyWeek(s) {
		httpx.WriteJSON(w, http.StatusOK, liteStudentWeeklyProseDTO{liteStudentWeeklyView(s, ws, isLatest, hasPrev, nil), nil})
		return
	}
	u, ok := requireTeacherEntitled(w, r)
	if !ok {
		return
	}

	// Every other student's name, so the prose cannot talk about them. A
	// classmate whose name is part of hers (her exact name, or 王丽 for 王丽华)
	// is left out: naming her would otherwise fail every attempt. A name
	// inside a verified 《title》 or 「quote」 needs no skip here; CheckProse
	// removes those spans before the name check.
	others := make([]string, 0, len(members))
	for _, m := range members {
		if m.ID != userID && strings.TrimSpace(m.DisplayName) != "" && !strings.Contains(s.Name, m.DisplayName) {
			others = append(others, m.DisplayName)
		}
	}

	mctx, cancel := detachedModelCtx(r)
	defer cancel()
	resolved, err := a.routeE(mctx, gateway.ClassAssess)
	if err != nil {
		httpx.WriteError(w, r, httpx.ErrAIDialogueFailed("lite_student_weekly route: "+err.Error()))
		return
	}
	prose, attempts, cerr := agent.ComposeLiteStudentWeekly(mctx, a.d.Provider, resolved, s, liteweek.Label(ws), liteWeekCards(s), others)
	for _, at := range attempts {
		a.recordLiteLLMCall(mctx, u.ID, uuid.Nil, liteStudentWeeklyPurpose, resolved, at.Usage)
	}
	if cerr != nil {
		slog.Warn("lite student weekly prose: rejected", "err", cerr, "user_id", userID, "request_id", httpx.RequestIDFromContext(ctx))
		msg := cerr.Error()
		httpx.WriteJSON(w, http.StatusOK, liteStudentWeeklyProseDTO{liteStudentWeeklyView(s, ws, isLatest, hasPrev, nil), &msg})
		return
	}
	body, err := json.Marshal(prose)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if err := a.d.Queries.InsertLiteStudentWeeklyProse(mctx, sqlc.InsertLiteStudentWeeklyProseParams{
		UserID: userID, WeekStart: pgDate(ws), Body: body,
	}); err != nil {
		// The model was paid for and its prose is lost; log enough to find the
		// week again. The prose text itself is not logged.
		slog.Error("lite student weekly prose: insert failed after compose", "err", err, "user_id", userID,
			"week_start", liteWeekStartString(ws), "attempts", len(attempts), "request_id", httpx.RequestIDFromContext(ctx))
		httpx.WriteError(w, r, err)
		return
	}
	// Re-read: a concurrent POST may have won the insert, and both teachers
	// must see the winner's prose.
	saved, err := a.storedLiteStudentProse(mctx, userID, ws)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if saved == nil {
		saved = &prose
	}
	httpx.WriteJSON(w, http.StatusOK, liteStudentWeeklyProseDTO{liteStudentWeeklyView(s, ws, isLatest, hasPrev, saved), nil})
}

// liteClassWeekStats totals the loaded weeks. AssignmentRate counts
// (recipient, assignment) pairs due in the week: done and done_late over all
// of them, -1 when none were due.
func liteClassWeekStats(students []liteweekly.StudentWeek) liteweekly.ClassWeekStats {
	st := liteweekly.ClassWeekStats{ClassSize: len(students)}
	done, due := 0, 0
	for _, s := range students {
		if s.ActiveDays > 0 {
			st.ActiveStudents++
		}
		if s.Minutes >= 0 {
			st.Minutes += s.Minutes
		}
		st.Turns += s.Turns
		st.Finished += len(s.Finished)
		done += s.AssignmentsDone + s.AssignmentsLate
		due += s.AssignmentsDone + s.AssignmentsLate + s.AssignmentsOverdue
	}
	st.AssignmentRate = -1
	if due > 0 {
		st.AssignmentRate = done * 100 / due
	}
	return st
}

// liteClassWeekCards derives every student's cards. The map holds only
// students with at least one card (the class composer rejects any other key).
func liteClassWeekCards(students []liteweekly.StudentWeek) (cards map[string][]liteweekly.Card, praise, watch []liteClassWeekCardDTO) {
	cards = map[string][]liteweekly.Card{}
	praise = []liteClassWeekCardDTO{}
	watch = []liteClassWeekCardDTO{}
	for _, s := range students {
		cs := liteWeekCards(s)
		if len(cs) == 0 {
			continue
		}
		cards[s.UserID] = cs
		for _, c := range cs {
			d := liteClassWeekCardDTO{liteWeekCardDTOOf(c), s.UserID, s.Name}
			if c.Kind == "praise" {
				praise = append(praise, d)
			} else {
				watch = append(watch, d)
			}
		}
	}
	return cards, praise, watch
}

// liteClassWeekEmpty reports whether the class's week has nothing in it:
// every loaded student's week is empty, or the class has no students.
func liteClassWeekEmpty(students []liteweekly.StudentWeek) bool {
	for _, s := range students {
		if !liteweekly.IsEmptyWeek(s) {
			return false
		}
	}
	return true
}

func liteClassWeeklyView(ws time.Time, isLatest, hasPrev, empty bool, stats liteweekly.ClassWeekStats, praise, watch []liteClassWeekCardDTO, prose *agent.LiteClassWeeklyProse) liteClassWeeklyDTO {
	label := liteweek.Label(ws)
	title := "班级周报 · " + label
	if isLatest {
		title = "上周班级周报 · " + label
	}
	return liteClassWeeklyDTO{
		WeekStart: liteWeekStartString(ws), WeekLabel: label, Title: title, IsLatest: isLatest,
		HasPrev: hasPrev, Empty: empty,
		Stats: liteClassWeekStatsDTO{
			ClassSize: stats.ClassSize, ActiveStudents: stats.ActiveStudents, Minutes: stats.Minutes,
			Turns: stats.Turns, Finished: stats.Finished, AssignmentRate: stats.AssignmentRate,
		},
		Praise: praise, Watch: watch,
		Prose: prose, ProseReady: prose != nil,
	}
}

// authTeacherClass parses {id} and checks the caller teaches the class.
func (a *API) authTeacherClass(w http.ResponseWriter, r *http.Request) (sqlc.Class, bool) {
	classID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		httpx.WriteError(w, r, httpx.ErrNotFound("资源不存在"))
		return sqlc.Class{}, false
	}
	cls, err := a.assertTeacherOwnsClass(r.Context(), classID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return sqlc.Class{}, false
	}
	return cls, true
}

// getLiteClassWeekly handles GET /api/v1/lite/teacher/classes/{id}/weekly?weekStart=.
// Never calls a model.
func (a *API) getLiteClassWeekly(w http.ResponseWriter, r *http.Request) {
	cls, ok := a.authTeacherClass(w, r)
	if !ok {
		return
	}
	ws, isLatest, hasPrev, ok := parseLiteClassWeek(w, r, cls)
	if !ok {
		return
	}
	ctx := r.Context()
	students, err := a.loadLiteClassWeek(ctx, cls.ID, ws)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	prose, err := a.storedLiteClassProse(ctx, cls.ID, ws)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	_, praise, watch := liteClassWeekCards(students)
	httpx.WriteJSON(w, http.StatusOK, liteClassWeeklyView(ws, isLatest, hasPrev, liteClassWeekEmpty(students), liteClassWeekStats(students), praise, watch, prose))
}

// postLiteClassWeeklyProse handles
// POST /api/v1/lite/teacher/classes/{id}/weekly/prose?weekStart=.
// Stored once per (class, week); same lifecycle as the student prose.
func (a *API) postLiteClassWeeklyProse(w http.ResponseWriter, r *http.Request) {
	cls, ok := a.authTeacherClass(w, r)
	if !ok {
		return
	}
	ws, isLatest, hasPrev, ok := parseLiteClassWeek(w, r, cls)
	if !ok {
		return
	}
	ctx := r.Context()
	stored, err := a.storedLiteClassProse(ctx, cls.ID, ws)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	students, err := a.loadLiteClassWeek(ctx, cls.ID, ws)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	// One stats value for the prompt, the digit check and the response.
	stats := liteClassWeekStats(students)
	cards, praise, watch := liteClassWeekCards(students)
	empty := liteClassWeekEmpty(students)
	if stored != nil {
		httpx.WriteJSON(w, http.StatusOK, liteClassWeeklyProseDTO{liteClassWeeklyView(ws, isLatest, hasPrev, empty, stats, praise, watch, stored), nil})
		return
	}
	// Nothing happened in the class that week: no entitlement check, no model
	// call, nothing stored. prose and proseError are both null.
	if empty {
		httpx.WriteJSON(w, http.StatusOK, liteClassWeeklyProseDTO{liteClassWeeklyView(ws, isLatest, hasPrev, empty, stats, praise, watch, nil), nil})
		return
	}
	u, ok := requireTeacherEntitled(w, r)
	if !ok {
		return
	}

	names := make([]string, 0, len(students))
	for _, s := range students {
		names = append(names, s.Name)
	}

	mctx, cancel := detachedModelCtx(r)
	defer cancel()
	resolved, err := a.routeE(mctx, gateway.ClassAssess)
	if err != nil {
		httpx.WriteError(w, r, httpx.ErrAIDialogueFailed("lite_class_weekly route: "+err.Error()))
		return
	}
	prose, attempts, cerr := agent.ComposeLiteClassWeekly(mctx, a.d.Provider, resolved, cls.Name, liteweek.Label(ws), stats, students, cards, names)
	for _, at := range attempts {
		a.recordLiteLLMCall(mctx, u.ID, uuid.Nil, liteClassWeeklyPurpose, resolved, at.Usage)
	}
	if cerr != nil {
		slog.Warn("lite class weekly prose: rejected", "err", cerr, "class_id", cls.ID, "request_id", httpx.RequestIDFromContext(ctx))
		msg := cerr.Error()
		httpx.WriteJSON(w, http.StatusOK, liteClassWeeklyProseDTO{liteClassWeeklyView(ws, isLatest, hasPrev, empty, stats, praise, watch, nil), &msg})
		return
	}
	body, err := json.Marshal(prose)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if err := a.d.Queries.InsertLiteClassWeeklyProse(mctx, sqlc.InsertLiteClassWeeklyProseParams{
		ClassID: cls.ID, WeekStart: pgDate(ws), Body: body,
	}); err != nil {
		// The model was paid for and its prose is lost; log enough to find the
		// week again. The prose text itself is not logged.
		slog.Error("lite class weekly prose: insert failed after compose", "err", err, "class_id", cls.ID,
			"week_start", liteWeekStartString(ws), "attempts", len(attempts), "request_id", httpx.RequestIDFromContext(ctx))
		httpx.WriteError(w, r, err)
		return
	}
	saved, err := a.storedLiteClassProse(mctx, cls.ID, ws)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if saved == nil {
		saved = &prose
	}
	httpx.WriteJSON(w, http.StatusOK, liteClassWeeklyProseDTO{liteClassWeeklyView(ws, isLatest, hasPrev, empty, stats, praise, watch, saved), nil})
}
