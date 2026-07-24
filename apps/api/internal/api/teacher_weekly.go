package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"mindimprint/api/internal/agent"
	"mindimprint/api/internal/httpx"
	"mindimprint/api/internal/store/sqlc"
	"mindimprint/api/internal/teacher"
)

// WeeklyCardDTO is one 值得表扬 / 需要建议 card. Evidence is always present —
// it is deterministic. Lead and action are "" until the prose is composed; a
// card with no wording still renders its tag and its evidence (敢于空白).
type WeeklyCardDTO struct {
	UserID        string `json:"userId"`
	DisplayName   string `json:"displayName"`
	AvatarColor   string `json:"avatarColor"`
	TagCode       string `json:"tagCode"`
	TagLabel      string `json:"tagLabel"`
	Kind          string `json:"kind"`
	Evidence      string `json:"evidence"`
	Lead          string `json:"lead"`
	Action        string `json:"action"`
	HasReport     bool   `json:"hasReport"`
	ReportSurface string `json:"reportSurface,omitempty"`
	ReportScopeID string `json:"reportScopeId,omitempty"`
}

type WeeklyStatDTO struct {
	Key      string `json:"key"`
	Label    string `json:"label"`
	Value    int    `json:"value"`
	Unit     string `json:"unit"`
	Foot     string `json:"foot"`
	Delta    string `json:"delta"`
	DeltaDir string `json:"deltaDir"`
}

type WeeklyBucketDTO struct {
	Code  string `json:"code"`
	Label string `json:"label"`
	Count int    `json:"count"`
}

type WeeklyDepthDTO struct {
	Buckets    []WeeklyBucketDTO `json:"buckets"`
	RatedCount int               `json:"ratedCount"`
	Note       string            `json:"note"`
}

type WeeklyAutonomyDTO struct {
	Mean       string `json:"mean"`
	Delta      string `json:"delta"`
	RatedCount int    `json:"ratedCount"`
	Note       string `json:"note"`
}

// WeeklyReportDTO is the whole screen. Every number in it was computed on this
// read; only comment/notes/lead/action come from storage.
type WeeklyReportDTO struct {
	WeekLabel  string            `json:"weekLabel"`
	WeekStart  string            `json:"weekStart"`
	WeekEnd    string            `json:"weekEnd"`
	AsOf       string            `json:"asOf"`
	ClassName  string            `json:"className"`
	ClassSize  int               `json:"classSize"`
	Stats      []WeeklyStatDTO   `json:"stats"`
	Praise     []WeeklyCardDTO   `json:"praise"`
	Watch      []WeeklyCardDTO   `json:"watch"`
	Depth      WeeklyDepthDTO    `json:"depth"`
	Autonomy   WeeklyAutonomyDTO `json:"autonomy"`
	Comment    *string           `json:"comment"`
	ProseReady bool              `json:"proseReady"`
}

// weeklyData is everything both handlers need: the live computation plus the
// class identity. The GET renders it; the POST composes prose over it.
type weeklyData struct {
	Class     sqlc.Class
	WeekStart time.Time
	Now       time.Time
	Weekly    teacher.Weekly
	Stats     []teacher.Stat
	ClassSize int
}

// loadWeekly runs the whole deterministic layer for one class. No model call.
func (a *API) loadWeekly(ctx context.Context, cls sqlc.Class, now time.Time) (weeklyData, error) {
	start, end := teacher.WeekWindow(now)
	prevStart, prevEnd := teacher.PrevWindow(now)

	cur, err := a.d.Queries.GetClassWeekStats(ctx, sqlc.GetClassWeekStatsParams{
		ClassID: cls.ID, WeekStart: start, WeekEnd: end,
	})
	if err != nil {
		return weeklyData{}, err
	}
	prev, err := a.d.Queries.GetClassWeekStats(ctx, sqlc.GetClassWeekStatsParams{
		ClassID: cls.ID, WeekStart: prevStart, WeekEnd: prevEnd,
	})
	if err != nil {
		return weeklyData{}, err
	}
	usage, err := a.d.Queries.ListClassStudentWindowUsage(ctx, sqlc.ListClassStudentWindowUsageParams{
		ClassID: cls.ID, WeekStart: start, WeekEnd: end, PrevStart: prevStart, PrevEnd: prevEnd,
	})
	if err != nil {
		return weeklyData{}, err
	}
	recent, err := a.d.Queries.ListClassRecentReports(ctx, cls.ID)
	if err != nil {
		return weeklyData{}, err
	}

	type pair struct {
		latest, previous *agent.Report
		surface, scopeID string
	}
	byUser := map[string]*pair{}
	for _, row := range recent {
		var rep agent.Report
		if json.Unmarshal(row.Scores, &rep) != nil {
			continue // a corrupt payload is no evidence — skip, never guess
		}
		// NOTE: ListClassRecentReportsRow.UserID/ScopeID are plain
		// google/uuid.UUID (not pgtype.UUID) — sqlc generated them that way
		// because the source columns are non-nullable uuid columns selected
		// directly, not through a LEFT JOIN. uuidText (used elsewhere in this
		// package) takes a pgtype.UUID, so it does not apply here; .String()
		// is the direct equivalent.
		key := row.UserID.String()
		p := byUser[key]
		if p == nil {
			p = &pair{}
			byUser[key] = p
		}
		if row.Rn == 1 {
			r := rep
			p.latest, p.surface, p.scopeID = &r, row.Surface, row.ScopeID.String()
		} else {
			r := rep
			p.previous = &r
		}
	}

	students := make([]teacher.StudentWeek, 0, len(usage))
	for _, u := range usage {
		s := teacher.StudentWeek{
			UserID: u.UserID.String(), DisplayName: u.DisplayName, AvatarColor: u.AvatarColor,
			ActiveDays: int(u.ActiveDays), Turns: int(u.Turns),
			PrevActiveDays: int(u.PrevActiveDays), PrevTurns: int(u.PrevTurns),
			ReportsThisWeek: int(u.ReportsThisWeek),
		}
		if p := byUser[u.UserID.String()]; p != nil {
			s.Latest, s.Previous = p.latest, p.previous
			s.LatestSurface, s.LatestScopeID = p.surface, p.scopeID
		}
		students = append(students, s)
	}

	return weeklyData{
		Class: cls, WeekStart: start, Now: now,
		Weekly: teacher.Detect(students),
		Stats: teacher.Stats(
			teacher.ClassWeekCounts{ActiveStudents: int(cur.ActiveStudents), Reports: int(cur.Reports), Turns: int(cur.Turns), CourseSteps: int(cur.CourseSteps)},
			teacher.ClassWeekCounts{ActiveStudents: int(prev.ActiveStudents), Reports: int(prev.Reports), Turns: int(prev.Turns), CourseSteps: int(prev.CourseSteps)},
			int(cur.ClassSize),
		),
		ClassSize: int(cur.ClassSize),
	}, nil
}

// weeklyDTO renders the loaded data, merging in whatever prose exists.
func weeklyDTO(d weeklyData, prose *sqlc.GetClassWeeklyProseRow) WeeklyReportDTO {
	start, end := teacher.WeekWindow(d.Now)
	dto := WeeklyReportDTO{
		WeekLabel: teacher.WeekLabel(start),
		WeekStart: start.Format(time.RFC3339), WeekEnd: end.Format(time.RFC3339),
		AsOf:      d.Now.UTC().Format(time.RFC3339),
		ClassName: d.Class.Name, ClassSize: d.ClassSize,
	}
	for _, s := range d.Stats {
		dto.Stats = append(dto.Stats, WeeklyStatDTO{
			Key: s.Key, Label: s.Label, Value: s.Value, Unit: s.Unit, Foot: s.Foot,
			Delta: s.Delta, DeltaDir: s.DeltaDir,
		})
	}
	for _, b := range d.Weekly.Depth.Buckets {
		dto.Depth.Buckets = append(dto.Depth.Buckets, WeeklyBucketDTO{Code: b.Code, Label: b.Label, Count: b.Count})
	}
	dto.Depth.RatedCount = d.Weekly.Depth.RatedCount
	dto.Autonomy = WeeklyAutonomyDTO{
		Mean: d.Weekly.Autonomy.Mean, Delta: d.Weekly.Autonomy.Delta, RatedCount: d.Weekly.Autonomy.RatedCount,
	}

	wording := map[string]agent.WeeklyCardProse{}
	if prose != nil {
		var cards []agent.WeeklyCardProse
		if json.Unmarshal(prose.Cards, &cards) == nil {
			for _, c := range cards {
				wording[c.UserID] = c
			}
		}
		c := prose.Comment
		dto.Comment = &c
		dto.Depth.Note = prose.DepthNote
		dto.Autonomy.Note = prose.AutonomyNote
		dto.ProseReady = true
	}
	conv := func(cards []teacher.Card) []WeeklyCardDTO {
		out := make([]WeeklyCardDTO, 0, len(cards))
		for _, c := range cards {
			w := wording[c.UserID]
			out = append(out, WeeklyCardDTO{
				UserID: c.UserID, DisplayName: c.DisplayName, AvatarColor: c.AvatarColor,
				TagCode: c.TagCode, TagLabel: c.TagLabel, Kind: c.Kind, Evidence: c.Evidence,
				Lead: w.Lead, Action: w.Action,
				HasReport: c.HasReport, ReportSurface: c.ReportSurface, ReportScopeID: c.ReportScopeID,
			})
		}
		return out
	}
	dto.Praise, dto.Watch = conv(d.Weekly.Praise), conv(d.Weekly.Watch)
	return dto
}

// getClassWeeklyReport handles GET /api/v1/classes/{id}/weekly-report. Every
// number is computed on this read; the stored prose is merged in when it
// exists. This handler NEVER calls a model.
func (a *API) getClassWeeklyReport(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		httpx.WriteError(w, r, httpx.ErrNotFound("资源不存在"))
		return
	}
	cls, err := a.assertTeacherOwnsClass(r.Context(), id)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	now := time.Now()
	data, err := a.loadWeekly(r.Context(), cls, now)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	prose, perr := a.d.Queries.GetClassWeeklyProse(r.Context(), sqlc.GetClassWeeklyProseParams{
		ClassID: id, WeekStart: pgtype.Date{Time: data.WeekStart, Valid: true},
	})
	switch {
	case perr == nil:
		httpx.WriteJSON(w, http.StatusOK, weeklyDTO(data, &prose))
	case errors.Is(perr, pgx.ErrNoRows):
		httpx.WriteJSON(w, http.StatusOK, weeklyDTO(data, nil))
	default:
		httpx.WriteError(w, r, perr)
	}
}
