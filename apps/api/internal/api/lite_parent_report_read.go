package api

// lite_parent_report_read.go — the read side of a published parent report:
// the parent page by share token (no session), her own copy, marking it seen,
// and its item in her inbox.
//
// Access is decided by the share token (parent page) or by user_id plus
// published status (her copy), never by current enrollment: the report is
// about her and has already been sent to her family (plan 4 Ruling 4).
//
// None of these payloads carries chat text, the model's draft or, on her
// route, the share token. The parent page carries no id at all.

import (
	"net/http"

	"github.com/google/uuid"

	"mindimprint/api/internal/httpx"
	"mindimprint/api/internal/liteparent"
	"mindimprint/api/internal/store/sqlc"
)

// PublicParentReportDTO is the parent page. The names are the ones frozen
// into the facts when the report was generated; TeacherName is the report's
// created_by.
type PublicParentReportDTO struct {
	StudentName string            `json:"studentName"`
	ClassName   string            `json:"className"`
	TeacherName string            `json:"teacherName"`
	RangeStart  string            `json:"rangeStart"`
	RangeEnd    string            `json:"rangeEnd"`
	PublishedAt string            `json:"publishedAt"`
	Facts       liteparent.Facts  `json:"facts"`
	Sections    []string          `json:"sections"`
	Body        map[string]string `json:"body"`
}

// StudentParentReportDTO is her copy: the parent page plus the report id.
type StudentParentReportDTO struct {
	ID string `json:"id"`
	PublicParentReportDTO
}

func newPublicParentReportDTO(row sqlc.LiteParentReport) (PublicParentReportDTO, error) {
	dto, err := newParentReportDTO(row)
	if err != nil {
		return PublicParentReportDTO{}, err
	}
	published := ""
	if dto.PublishedAt != nil {
		published = *dto.PublishedAt
	}
	return PublicParentReportDTO{
		StudentName: dto.Facts.StudentName, ClassName: dto.Facts.ClassName, TeacherName: dto.Facts.TeacherName,
		RangeStart: dto.RangeStart, RangeEnd: dto.RangeEnd, PublishedAt: published,
		Facts: dto.Facts, Sections: dto.Sections, Body: dto.Body,
	}, nil
}

// getPublicParentReport handles GET /api/v1/public/parent-reports/{token}.
// No session. An unknown or revoked token is 404 (a revoke sets share_token
// to NULL, so the lookup finds nothing).
func (a *API) getPublicParentReport(w http.ResponseWriter, r *http.Request) {
	// She is a minor and the link is for her family, not for search engines.
	// The headers are set before any outcome, 404 included. no-store keeps the
	// page out of shared caches, so a revoke or an edit shows on the next load.
	w.Header().Set("X-Robots-Tag", "noindex, nofollow, noarchive")
	w.Header().Set("Cache-Control", "no-store")
	token := r.PathValue("token")
	if token == "" {
		httpx.WriteError(w, r, httpx.ErrNotFound("资源不存在"))
		return
	}
	row, err := a.d.Queries.GetLiteParentReportByToken(r.Context(), token)
	if err != nil {
		writeNotFoundOr(w, r, err)
		return
	}
	dto, err := newPublicParentReportDTO(row)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"report": dto})
}

// loadStudentParentReport parses {rid} and loads her own published report.
// A malformed id, another student's report or a draft is 404. ok=false means
// the error is already written.
func (a *API) loadStudentParentReport(w http.ResponseWriter, r *http.Request) (sqlc.LiteParentReport, bool) {
	u, _ := UserFromContext(r.Context())
	rid, err := uuid.Parse(r.PathValue("rid"))
	if err != nil {
		httpx.WriteError(w, r, httpx.ErrNotFound("资源不存在"))
		return sqlc.LiteParentReport{}, false
	}
	row, err := a.d.Queries.GetStudentParentReport(r.Context(), sqlc.GetStudentParentReportParams{ID: rid, UserID: u.ID})
	if err != nil {
		writeNotFoundOr(w, r, err)
		return sqlc.LiteParentReport{}, false
	}
	return row, true
}

// getStudentParentReport handles GET /api/v1/lite/parent-reports/{rid}. It
// stays readable after the teacher revokes the link and after she leaves the
// class.
func (a *API) getStudentParentReport(w http.ResponseWriter, r *http.Request) {
	row, ok := a.loadStudentParentReport(w, r)
	if !ok {
		return
	}
	dto, err := newPublicParentReportDTO(row)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"report": StudentParentReportDTO{ID: row.ID.String(), PublicParentReportDTO: dto}})
}

// markStudentParentReportSeen handles POST /api/v1/lite/parent-reports/{rid}/seen.
// The first time is kept; a repeat is still 204.
func (a *API) markStudentParentReportSeen(w http.ResponseWriter, r *http.Request) {
	row, ok := a.loadStudentParentReport(w, r)
	if !ok {
		return
	}
	if err := a.d.Queries.MarkParentReportSeen(r.Context(), sqlc.MarkParentReportSeenParams{ID: row.ID, UserID: row.UserID}); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// parentReportInboxEntry is one published report as an inbox item.
func parentReportInboxEntry(row sqlc.ListStudentPublishedParentReportsRow) inboxEntry {
	return inboxEntry{
		item: InboxItemDTO{
			Type: inboxTypeParentReport, ID: row.ID.String(),
			Title: "家长报告（" + liteparent.MonthDay(liteParentDate(row.RangeStart)) +
				"–" + liteparent.MonthDay(liteParentDate(row.RangeEnd)) + "）",
			ClassName: row.ClassName, PublishedAt: tsStringPtr(row.PublishedAt),
			Unread: !row.StudentSeenAt.Valid,
		},
		published: row.PublishedAt.Time,
	}
}
