package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"mindimprint/api/internal/agent"
	"mindimprint/api/internal/httpx"
	"mindimprint/api/internal/store/sqlc"
	"mindimprint/api/internal/teacher"
)

// ParentStageStatDTO is one of the four usage cards (value已含单位: "6 天").
type ParentStageStatDTO struct {
	Value string `json:"value"`
	Label string `json:"label"`
}

// ParentStageReportDTO is the whole stage page. Stats/cover(week label) are
// deterministic; growth/highlight/forward/warmLine/advice come from stored
// prose (nil until composed).
type ParentStageReportDTO struct {
	Cover          ParentCoverDTO       `json:"cover"`
	Stats          []ParentStageStatDTO `json:"stats"`
	StageGrowth    string               `json:"stageGrowth"`
	StageHighlight string               `json:"stageHighlight"`
	StageForward   string               `json:"stageForward"`
	Advice         []ParentAdviceDTO    `json:"advice"`
	Prose          *string              `json:"prose"`
}

// parentStageData is everything both handlers need: the live stats + identity +
// the resolved week. GET renders it; POST composes prose over it.
type parentStageData struct {
	Name        string
	Klass       string
	WeekStart   time.Time
	WeekLabel   string
	ScopeID     string // week_start "2006-01-02" — the storage/scope key
	StudentID   uuid.UUID
	ActiveDays  int
	Turns       int
	Reports     int
	CourseSteps int
}

// resolveWeekStart maps the {weekStart} path value to a Monday-00:00-UTC start.
// "" or "current" → the week enclosing now; else strict YYYY-MM-DD (UTC).
func resolveWeekStart(weekStart string, now time.Time) (time.Time, error) {
	if weekStart == "" || weekStart == "current" {
		start, _ := teacher.WeekWindow(now)
		return start, nil
	}
	d, err := time.ParseInLocation("2006-01-02", weekStart, time.UTC)
	if err != nil {
		return time.Time{}, httpx.ErrNotFound("资源不存在")
	}
	return d, nil
}

// loadParentStage runs the deterministic stage layer for one (student, week).
// No model call.
func (a *API) loadParentStage(ctx context.Context, classID, userID uuid.UUID, start time.Time) (parentStageData, error) {
	end := start.AddDate(0, 0, 7)
	stats, err := a.d.Queries.GetStudentWeekStats(ctx, sqlc.GetStudentWeekStatsParams{
		UserID: userID, WeekStart: start, WeekEnd: end,
	})
	if err != nil {
		return parentStageData{}, err
	}
	user, err := a.d.Queries.GetUserByID(ctx, userID)
	if err != nil {
		return parentStageData{}, err
	}
	cls, err := a.d.Queries.GetClassByID(ctx, classID)
	if err != nil {
		return parentStageData{}, err
	}
	return parentStageData{
		Name: user.DisplayName, Klass: cls.Name,
		WeekStart: start, WeekLabel: teacher.WeekLabel(start), ScopeID: start.Format("2006-01-02"),
		StudentID:  userID,
		ActiveDays: int(stats.ActiveDays), Turns: int(stats.Turns),
		Reports: int(stats.Reports), CourseSteps: int(stats.CourseSteps),
	}, nil
}

// buildStageStats renders the four cards' value strings server-side (D1 anti-drift).
func buildStageStats(d parentStageData) []ParentStageStatDTO {
	return []ParentStageStatDTO{
		{Value: strconv.Itoa(d.ActiveDays) + " 天", Label: "本周活跃"},
		{Value: strconv.Itoa(d.Turns), Label: "对话轮次"},
		{Value: strconv.Itoa(d.Reports) + " 份", Label: "生成报告"},
		{Value: strconv.Itoa(d.CourseSteps) + " 节", Label: "完成课程"},
	}
}

// parentStageDTO merges the deterministic stage layer with whatever prose exists.
func parentStageDTO(d parentStageData, prose *agent.ParentStageProse) ParentStageReportDTO {
	warm := ""
	if prose != nil {
		warm = prose.WarmLine
	}
	dto := ParentStageReportDTO{
		Cover: ParentCoverDTO{
			Name: d.Name, Subject: d.WeekLabel, Klass: d.Klass,
			TypeLabel: "阶段报告", DateStr: parentDateStr(time.Now()), WarmLine: warm,
		},
		Stats: buildStageStats(d),
	}
	if prose != nil {
		dto.StageGrowth, dto.StageHighlight, dto.StageForward = prose.StageGrowth, prose.StageHighlight, prose.StageForward
		for _, ad := range prose.Advice {
			dto.Advice = append(dto.Advice, ParentAdviceDTO{Title: ad.Title, Text: ad.Text})
		}
		present := "present"
		dto.Prose = &present
	}
	return dto
}

// getParentStageProse reads and decodes the stored stage bundle, or pgx.ErrNoRows.
func (a *API) getParentStageProse(ctx context.Context, userID uuid.UUID, scopeID string) (*agent.ParentStageProse, error) {
	raw, err := a.d.Queries.GetParentReportProse(ctx, sqlc.GetParentReportProseParams{
		StudentUserID: userID, Surface: "stage", ScopeID: scopeID,
	})
	if err != nil {
		return nil, err
	}
	var p agent.ParentStageProse
	if err := json.Unmarshal(raw, &p); err != nil {
		return nil, err
	}
	return &p, nil
}

// getParentStageReport handles GET .../parent-stage-report/{weekStart}. Computes
// usage stats live, merges stored prose when present. NEVER calls a model.
func (a *API) getParentStageReport(w http.ResponseWriter, r *http.Request) {
	classID, userID, ok := a.authTeacherStudent(w, r)
	if !ok {
		return
	}
	start, err := resolveWeekStart(r.PathValue("weekStart"), time.Now())
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	data, err := a.loadParentStage(r.Context(), classID, userID, start)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	prose, perr := a.getParentStageProse(r.Context(), userID, data.ScopeID)
	switch {
	case perr == nil:
		httpx.WriteJSON(w, http.StatusOK, parentStageDTO(data, prose))
	case errors.Is(perr, pgx.ErrNoRows):
		httpx.WriteJSON(w, http.StatusOK, parentStageDTO(data, nil))
	default:
		httpx.WriteError(w, r, perr)
	}
}

// (POST handler added in Task 4 — same file.)
var _ = pgtype.UUID{} // POST (T4) uses pgtype; keep the import stable across tasks.
