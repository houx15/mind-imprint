package api

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/google/uuid"

	"mindimprint/api/internal/agent"
	"mindimprint/api/internal/httpx"
	"mindimprint/api/internal/store/sqlc"
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

// weekWindow returns the half-open [Mon 00:00, next Mon 00:00) window enclosing
// now, in now's location. D1 uses the current week only (no deltas — that is D2).
func weekWindow(now time.Time) (time.Time, time.Time) {
	y, m, d := now.Date()
	midnight := time.Date(y, m, d, 0, 0, 0, 0, now.Location())
	offset := (int(now.Weekday()) + 6) % 7 // Monday=0
	start := midnight.AddDate(0, 0, -offset)
	return start, start.AddDate(0, 0, 7)
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
	start, end := weekWindow(time.Now())
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
