package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"sort"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"mindimprint/api/internal/agent"
	"mindimprint/api/internal/httpx"
	"mindimprint/api/internal/store/sqlc"
	"mindimprint/api/internal/studio"
	"mindimprint/api/internal/teacher"
)

// RosterReportEntry is one student row in the teacher's 全部学生 view: usage
// facts + derived D/A badges. Badges are display summaries (RL-5), recomputed
// from the latest project report on every read.
type RosterReportEntry struct {
	ID          string `json:"id"`
	DisplayName string `json:"displayName"`
	AvatarColor string `json:"avatarColor"`
	DBadge      string `json:"dBadge"`
	ABadge      string `json:"aBadge"`
	ActiveDays  int32  `json:"activeDays"`
	Turns       int32  `json:"turns"`
	HasReport   bool   `json:"hasReport"`
	Unrated     bool   `json:"unrated"`
}

// getClassRosterReport handles GET /api/v1/classes/{id}/roster-report: the
// teacher's per-student usage + D/A badge view. assertTeacherOwnsClass is the
// only guard needed here — ListClassRosterReport itself JOINs enrollments with
// role_in_class='student', so it can never return non-student or foreign-class
// rows.
func (a *API) getClassRosterReport(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		httpx.WriteError(w, r, httpx.ErrNotFound("资源不存在"))
		return
	}
	if _, err := a.assertTeacherOwnsClass(r.Context(), id); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	start, end := teacher.WeekWindow(time.Now())
	rows, err := a.d.Queries.ListClassRosterReport(r.Context(), sqlc.ListClassRosterReportParams{
		ClassID: id, WeekStart: start, WeekEnd: end,
	})
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	out := make([]RosterReportEntry, 0, len(rows))
	for _, row := range rows {
		hr, _ := row.HasReport.(bool)
		e := RosterReportEntry{
			ID: row.ID.String(), DisplayName: row.DisplayName, AvatarColor: row.AvatarColor,
			ActiveDays: row.ActiveDays, Turns: row.Turns, HasReport: hr,
			DBadge: "—", ABadge: "—", Unrated: true,
		}
		if len(row.LatestProjectScores) > 0 {
			var rep agent.Report
			if json.Unmarshal(row.LatestProjectScores, &rep) == nil {
				e.DBadge, e.ABadge, e.Unrated = teacher.DBadge(rep), teacher.ABadge(rep), false
			}
		}
		out = append(out, e)
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"roster": out})
}

// StudentHeadDTO is the per-student header card on the teacher's student-detail
// page: identity + derived D/A badges (RL-5 display summary, recomputed from
// the latest project report on every read, never stored).
type StudentHeadDTO struct {
	ID          string `json:"id"`
	DisplayName string `json:"displayName"`
	AvatarColor string `json:"avatarColor"`
	DBadge      string `json:"dBadge"`
	ABadge      string `json:"aBadge"`
	Unrated     bool   `json:"unrated"`
}

// UsageDTO is this-week activity + report counts for the student-detail head.
type UsageDTO struct {
	ActiveDays  int64 `json:"activeDays"`
	Turns       int64 `json:"turns"`
	ReportCount int   `json:"reportCount"`
	CourseCount int   `json:"courseCount"`
}

// RecordDTO is one row in the student's 平台使用记录 list: either a reported
// scope (project/course/chat, hasReport:true) or an in-progress project that
// has not been reported on yet (hasReport:false).
type RecordDTO struct {
	Surface   string `json:"surface"` // project|course|chat
	ScopeID   string `json:"scopeId"`
	Title     string `json:"title"`
	Date      string `json:"date"`
	Status    string `json:"status"`
	HasReport bool   `json:"hasReport"`
}

// authTeacherStudent closes the "the caller teaches this class AND userId is a
// STUDENT member of it" gate shared by the student-detail endpoints (Tasks 4
// and 5). Any failure — malformed path values, class not owned, target not a
// member, or a member with a non-student role — is reported as 404 so cross-
// tenant/cross-class probing can't distinguish "wrong class" from "not a
// student" from "doesn't exist".
func (a *API) authTeacherStudent(w http.ResponseWriter, r *http.Request) (classID, userID uuid.UUID, ok bool) {
	classID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		httpx.WriteError(w, r, httpx.ErrNotFound("资源不存在"))
		return uuid.UUID{}, uuid.UUID{}, false
	}
	userID, err = uuid.Parse(r.PathValue("userId"))
	if err != nil {
		httpx.WriteError(w, r, httpx.ErrNotFound("资源不存在"))
		return uuid.UUID{}, uuid.UUID{}, false
	}
	if _, err := a.assertTeacherOwnsClass(r.Context(), classID); err != nil {
		httpx.WriteError(w, r, err)
		return uuid.UUID{}, uuid.UUID{}, false
	}
	enr, err := a.d.Queries.GetEnrollment(r.Context(), sqlc.GetEnrollmentParams{UserID: userID, ClassID: classID})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			httpx.WriteError(w, r, httpx.ErrNotFound("资源不存在"))
			return uuid.UUID{}, uuid.UUID{}, false
		}
		httpx.WriteError(w, r, err)
		return uuid.UUID{}, uuid.UUID{}, false
	}
	if enr.RoleInClass != "student" {
		httpx.WriteError(w, r, httpx.ErrNotFound("资源不存在"))
		return uuid.UUID{}, uuid.UUID{}, false
	}
	return classID, userID, true
}

// getStudentDetail handles GET /api/v1/classes/{id}/students/{userId}: the
// teacher's per-student page — head badges, this-week usage, and the merged
// records list (every project, reported or not, plus every course/chat
// report). No writes.
func (a *API) getStudentDetail(w http.ResponseWriter, r *http.Request) {
	_, userID, ok := a.authTeacherStudent(w, r)
	if !ok {
		return
	}
	ctx := r.Context()

	user, err := a.d.Queries.GetUserByID(ctx, userID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}

	start, end := teacher.WeekWindow(time.Now())
	usageRow, err := a.d.Queries.GetStudentUsageForTeacher(ctx, sqlc.GetStudentUsageForTeacherParams{
		UserID: userID, WeekStart: start, WeekEnd: end,
	})
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}

	reports, err := a.d.Queries.ListStudentReportsForTeacher(ctx, userID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}

	projects, err := a.d.Queries.ListStudentProjectsForTeacher(ctx, userID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}

	head := StudentHeadDTO{
		ID: userID.String(), DisplayName: user.DisplayName, AvatarColor: user.AvatarColor,
		DBadge: "—", ABadge: "—", Unrated: true,
	}
	scores, err := a.d.Queries.GetLatestProjectScoresForStudent(ctx, userID)
	switch {
	case err == nil:
		var rep agent.Report
		if json.Unmarshal(scores, &rep) == nil {
			head.DBadge, head.ABadge, head.Unrated = teacher.DBadge(rep), teacher.ABadge(rep), false
		}
	case errors.Is(err, pgx.ErrNoRows):
		// Unrated — head keeps its "—"/unrated defaults.
	default:
		httpx.WriteError(w, r, err)
		return
	}

	// Merge: a project row (reported or not) + a course/chat report row. A
	// project scope is deduped to one record — if it has a report, that
	// report's row (from `reports`) supplies the record instead of the plain
	// project row, so it never appears twice.
	reportByProjectID := make(map[string]sqlc.ListStudentReportsForTeacherRow, len(reports))
	reportCount, courseCount := 0, 0
	type dated struct {
		rec RecordDTO
		at  time.Time
	}
	var rows []dated
	for _, rep := range reports {
		reportCount++
		if rep.Surface == "course" {
			courseCount++
		}
		if rep.Surface == "project" {
			reportByProjectID[uuidText(rep.ScopeID)] = rep
			continue // folded into the project loop below
		}
		rows = append(rows, dated{
			rec: RecordDTO{
				Surface: rep.Surface, ScopeID: uuidText(rep.ScopeID), Title: rep.Label,
				Date: rep.CreatedAt.Format(time.RFC3339), Status: "能力报告已生成", HasReport: true,
			},
			at: rep.CreatedAt,
		})
	}
	for _, p := range projects {
		id := p.ID.String()
		if rep, has := reportByProjectID[id]; has {
			rows = append(rows, dated{
				rec: RecordDTO{
					Surface: "project", ScopeID: id, Title: rep.Label,
					Date: rep.CreatedAt.Format(time.RFC3339), Status: "能力报告已生成", HasReport: true,
				},
				at: rep.CreatedAt,
			})
			continue
		}
		rows = append(rows, dated{
			rec: RecordDTO{
				Surface: "project", ScopeID: id, Title: p.Title,
				Date: p.LastActiveAt.Format(time.RFC3339), Status: "进行中", HasReport: false,
			},
			at: p.LastActiveAt,
		})
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].at.After(rows[j].at) })
	records := make([]RecordDTO, len(rows))
	for i, row := range rows {
		records[i] = row.rec
	}

	httpx.WriteJSON(w, http.StatusOK, map[string]any{
		"student": head,
		"usage": UsageDTO{
			ActiveDays: usageRow.ActiveDays, Turns: usageRow.Turns,
			ReportCount: reportCount, CourseCount: courseCount,
		},
		"records": records,
	})
}

// ReportContext is the deep-report's framing: the display title, and (project
// surface only) the research question the student is answering. DBadge/ABadge
// are the SAME display-summary badges the roster/student-head views show
// (teacher.DBadge/ABadge over this exact report) — the single producer for
// every screen that shows a D/A badge, so a teacher never sees two different
// numbers for the same report on two adjacent screens.
type ReportContext struct {
	ProjectTitle     string `json:"projectTitle,omitempty"`
	ResearchQuestion string `json:"researchQuestion,omitempty"`
	Title            string `json:"title,omitempty"`
	DBadge           string `json:"dBadge,omitempty"`
	ABadge           string `json:"aBadge,omitempty"`
}

// TeacherReportDTO is the deep-report data source for the teacher's
// per-scope report view: the same canonical ReportDTO the student sees, plus
// the project framing context (empty on course/chat).
type TeacherReportDTO struct {
	Report  studio.ReportDTO `json:"report"`
	Context ReportContext    `json:"context"`
}

// rqFromNode extracts the research question from a research_question graph
// node's body ({"text": "..."}, minted at project creation — project_create.go).
// Tries "text" then "question" for forward compatibility; an empty/absent
// body falls back to the project title; if that is also empty, returns ""
// (敢于空白 — never fabricate a question).
func rqFromNode(body []byte, fallback string) string {
	if len(body) == 0 {
		return fallback
	}
	var m map[string]any
	if err := json.Unmarshal(body, &m); err != nil {
		return fallback
	}
	for _, key := range []string{"text", "question"} {
		if v, ok := m[key].(string); ok && v != "" {
			return v
		}
	}
	return fallback
}

// getStudentReport handles GET
// /api/v1/classes/{id}/students/{userId}/reports/{surface}/{scopeId}: the
// deep-report data source for one scope (project/course/chat). Guarded by
// authTeacherStudent (class ownership + this-class student membership); the
// eval queries additionally re-check that the scope is owned by that exact
// student, so a correct surface+scopeId belonging to a DIFFERENT student
// still 404s. Per-surface query errors are written directly via
// httpx.WriteError, which already maps pgx.ErrNoRows to 404 — missing
// scope/ownership is hidden as not-found, same as authTeacherStudent.
func (a *API) getStudentReport(w http.ResponseWriter, r *http.Request) {
	_, userID, ok := a.authTeacherStudent(w, r)
	if !ok {
		return
	}
	surface := r.PathValue("surface")
	scopeID, err := uuid.Parse(r.PathValue("scopeId"))
	if err != nil {
		httpx.WriteError(w, r, httpx.ErrNotFound("资源不存在"))
		return
	}
	pgScopeID := pgtype.UUID{Bytes: scopeID, Valid: true}

	var scores []byte
	var createdAt time.Time
	var ctx ReportContext

	switch surface {
	case "project":
		row, e := a.d.Queries.GetStudentProjectEvaluationForTeacher(r.Context(), sqlc.GetStudentProjectEvaluationForTeacherParams{
			ScopeID: pgScopeID, UserID: userID,
		})
		if e != nil {
			httpx.WriteError(w, r, e)
			return
		}
		scores, createdAt = row.Scores, row.CreatedAt
		ctx.ProjectTitle = row.ProjectTitle
		ctx.Title = row.ProjectTitle
		ctx.ResearchQuestion = rqFromNode(row.RqBody, row.ProjectTitle)
	case "course":
		row, e := a.d.Queries.GetStudentSessionEvaluationForTeacher(r.Context(), sqlc.GetStudentSessionEvaluationForTeacherParams{
			ScopeID: pgScopeID, UserID: userID,
		})
		if e != nil {
			httpx.WriteError(w, r, e)
			return
		}
		scores, createdAt = row.Scores, row.CreatedAt
		ctx.Title = row.CourseTitle
	case "chat":
		row, e := a.d.Queries.GetStudentThreadEvaluationForTeacher(r.Context(), sqlc.GetStudentThreadEvaluationForTeacherParams{
			ScopeID: pgScopeID, UserID: userID,
		})
		if e != nil {
			httpx.WriteError(w, r, e)
			return
		}
		scores, createdAt = row.Scores, row.CreatedAt
		ctx.Title = row.ThreadTitle
	default:
		httpx.WriteError(w, r, httpx.ErrNotFound("资源不存在"))
		return
	}

	var rep agent.Report
	if err := json.Unmarshal(scores, &rep); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	ctx.DBadge = teacher.DBadge(rep)
	ctx.ABadge = teacher.ABadge(rep)
	httpx.WriteJSON(w, http.StatusOK, TeacherReportDTO{
		Report:  studio.ToReportDTO(rep, createdAt),
		Context: ctx,
	})
}
