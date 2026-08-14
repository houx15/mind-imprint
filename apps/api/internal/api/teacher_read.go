package api

import (
	"errors"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"mindimprint/api/internal/httpx"
	"mindimprint/api/internal/store/sqlc"
	"mindimprint/api/internal/teacher"
)

// RosterEntry is one student row in View B (实时 roster): current-state activity
// counts, all no-LLM. No axis, no badges.
type RosterEntry struct {
	ID              string `json:"id"`
	DisplayName     string `json:"displayName"`
	AvatarColor     string `json:"avatarColor"`
	ActiveProjects  int32  `json:"activeProjects"`
	ReportCount     int32  `json:"reportCount"`
	CoursesFinished int32  `json:"coursesFinished"`
}

// ClassLiveHeader is View B's light live class-level snapshot.
type ClassLiveHeader struct {
	ClassSize      int32 `json:"classSize"`
	ActiveStudents int32 `json:"activeStudents"`
	ActiveProjects int32 `json:"activeProjects"`
	Turns          int32 `json:"turns"`
	Reports        int32 `json:"reports"`
}

// getClassRosterReport handles GET /api/v1/classes/{id}/roster-report: View B's
// live roster + header. assertTeacherOwnsClass is the only guard needed since
// every query JOINs enrollments with role_in_class='student'.
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
	ctx := r.Context()
	rows, err := a.d.Queries.ListClassRosterCounts(ctx, id)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	start, end := teacher.WeekWindow(time.Now())
	hdr, err := a.d.Queries.GetClassLiveHeader(ctx, sqlc.GetClassLiveHeaderParams{
		ClassID: id, WeekStart: start, WeekEnd: end,
	})
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	out := make([]RosterEntry, 0, len(rows))
	for _, row := range rows {
		out = append(out, RosterEntry{
			ID: row.ID.String(), DisplayName: row.DisplayName, AvatarColor: row.AvatarColor,
			ActiveProjects: row.ActiveProjects, ReportCount: row.ReportCount, CoursesFinished: row.CoursesFinished,
		})
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{
		"roster": out,
		"header": ClassLiveHeader{
			ClassSize: hdr.ClassSize, ActiveStudents: hdr.ActiveStudents,
			ActiveProjects: hdr.ActiveProjects, Turns: hdr.Turns, Reports: hdr.Reports,
		},
	})
}

// StudentHeadDTO is the per-student header card on the teacher's student-detail
// page: identity only. No axis badges — the D/A-axis pipeline is retired.
type StudentHeadDTO struct {
	ID          string `json:"id"`
	DisplayName string `json:"displayName"`
	AvatarColor string `json:"avatarColor"`
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
// records list (every project, reported or not) + this-week usage. No writes.
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

	head := StudentHeadDTO{ID: userID.String(), DisplayName: user.DisplayName, AvatarColor: user.AvatarColor}

	projects, err := a.d.Queries.ListStudentProjectsForTeacher(ctx, userID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	courseCount, err := a.d.Queries.CountFinishedCoursesForStudent(ctx, userID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}

	records := make([]RecordDTO, 0, len(projects))
	reportCount := 0
	for _, p := range projects {
		status := "进行中"
		if p.HasReport {
			status = "能力报告已生成"
			reportCount++
		}
		records = append(records, RecordDTO{
			Surface: "project", ScopeID: p.ID.String(), Title: p.Title,
			Date: p.LastActiveAt.Format(time.RFC3339), Status: status, HasReport: p.HasReport,
		})
	}
	// projects already ordered by last_active_at DESC — no re-sort needed.

	httpx.WriteJSON(w, http.StatusOK, map[string]any{
		"student": head,
		"usage": UsageDTO{
			ActiveDays: usageRow.ActiveDays, Turns: usageRow.Turns,
			ReportCount: reportCount, CourseCount: int(courseCount),
		},
		"records": records,
	})
}
