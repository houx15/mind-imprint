package api

import (
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"mindimprint/api/internal/httpx"
	"mindimprint/api/internal/liteweek"
	"mindimprint/api/internal/store/sqlc"
)

// LiteRosterRowDTO is one row of the lite teacher end's class roster: identity
// + live per-student totals, all no-LLM (same shape family as pro's RosterEntry
// in teacher_read.go, but counting lite atoms instead of pro projects).
type LiteRosterRowDTO struct {
	ID                 string  `json:"id"`
	DisplayName        string  `json:"displayName"`
	AvatarColor        string  `json:"avatarColor"`
	LastActiveAt       *string `json:"lastActiveAt"`
	ActiveDaysThisWeek int32   `json:"activeDaysThisWeek"`
	MinutesTotal       int32   `json:"minutesTotal"`
	MinutesThisWeek    int32   `json:"minutesThisWeek"` // -1 = no time buckets recorded yet
	Turns              int32   `json:"turns"`
	ReadingsDone       int32   `json:"readingsDone"`
	ReadingsTotal      int32   `json:"readingsTotal"`
	WritingsDone       int32   `json:"writingsDone"`
	WritingsTotal      int32   `json:"writingsTotal"`
	ProjectsDone       int32   `json:"projectsDone"`
	ProjectsTotal      int32   `json:"projectsTotal"`
}

// currentLiteWeek is [Monday 00:00 Beijing, next Monday) around now.
func currentLiteWeek(now time.Time) (start, end time.Time) {
	start = liteweek.WeekStart(now)
	return start, start.AddDate(0, 0, 7)
}

func pgDate(t time.Time) pgtype.Date { return pgtype.Date{Time: t, Valid: true} }

func secondsToMinutes(s int32) int32 { return (s + 30) / 60 }

// getLiteClassRoster handles GET /api/v1/lite/teacher/classes/{id}/roster.
func (a *API) getLiteClassRoster(w http.ResponseWriter, r *http.Request) {
	classID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		httpx.WriteError(w, r, httpx.ErrNotFound("资源不存在"))
		return
	}
	if _, err := a.assertTeacherOwnsClass(r.Context(), classID); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	start, end := currentLiteWeek(time.Now())
	rows, err := a.d.Queries.ListLiteClassRoster(r.Context(), sqlc.ListLiteClassRosterParams{
		ClassID: classID, WeekStart: start, WeekEnd: end,
		WeekStartDay: pgDate(start), WeekEndDay: pgDate(end),
	})
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	out := make([]LiteRosterRowDTO, 0, len(rows))
	for _, row := range rows {
		out = append(out, liteRosterRow(row))
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"roster": out})
}

// liteEpoch is the SQL sentinel ('epoch'::timestamptz, 1970-01-01 UTC) the
// query substitutes for a student with zero atoms: sqlc's static nullability
// inference (no live DB connection configured) treats every aggregate
// subquery here as NOT NULL, so the generated row holds a plain time.Time /
// int32, not pointers — the query COALESCEs instead of relying on a Go nil.
// bucket_count is what actually distinguishes "never measured" from
// "measured, zero seconds" for minutesThisWeek.
var liteEpoch = time.Unix(0, 0).UTC()

func liteRosterRow(row sqlc.ListLiteClassRosterRow) LiteRosterRowDTO {
	dto := LiteRosterRowDTO{
		ID: row.ID.String(), DisplayName: row.DisplayName, AvatarColor: row.AvatarColor,
		ActiveDaysThisWeek: row.ActiveDaysThisWeek,
		MinutesTotal:       secondsToMinutes(row.SecondsTotal),
		MinutesThisWeek:    -1,
		Turns:              row.Turns,
		ReadingsDone:       row.ReadingsDone,
		ReadingsTotal:      row.ReadingsTotal,
		WritingsDone:       row.WritingsDone,
		WritingsTotal:      row.WritingsTotal,
		ProjectsDone:       row.ProjectsDone,
		ProjectsTotal:      row.ProjectsTotal,
	}
	if row.BucketCount > 0 {
		dto.MinutesThisWeek = secondsToMinutes(row.SecondsThisWeek)
	}
	if !row.LastActiveAt.Equal(liteEpoch) {
		s := row.LastActiveAt.Format(time.RFC3339)
		dto.LastActiveAt = &s
	}
	return dto
}
