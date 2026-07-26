package api

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"mindimprint/api/internal/ability"
	"mindimprint/api/internal/agent"
	"mindimprint/api/internal/gateway"
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
		// Non-nil so the wire carries `advice: []` (the contract requires an
		// array) rather than `advice: null` pre-prose — a null broke the client
		// Zod parse and the whole parent stage report rendered blank.
		Advice: []ParentAdviceDTO{},
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

// toAbilitySummary flattens the cross-session ability model into the composer's
// import-cycle-safe reference struct.
func toAbilitySummary(m ability.Model) agent.AbilitySummary {
	s := agent.AbilitySummary{
		TotalSessions:       m.TotalSessions,
		BoundarySettings:    m.Autonomy.BoundarySettings,
		AdversaryInvites:    m.Autonomy.AdversaryInvites,
		OpportunitiesTaken:  m.Autonomy.OpportunitiesTaken,
		OpportunitiesMissed: m.Autonomy.OpportunitiesMissed,
	}
	for _, d := range m.Depth {
		s.Depth = append(s.Depth, agent.AbilityDepthFact{Name: d.Name, LevelLabel: d.LevelLabel, EvidenceCount: d.EvidenceCount})
	}
	return s
}

// studentAbility fetches the student's whole cross-session report history
// (teacher-scoped) and merges it. A malformed row must not sink the aggregate.
func (a *API) studentAbility(ctx context.Context, userID uuid.UUID) (agent.AbilitySummary, error) {
	rows, err := a.d.Queries.ListStudentEvaluationsForTeacher(ctx, userID)
	if err != nil {
		return agent.AbilitySummary{}, err
	}
	samples := make([]ability.Sample, 0, len(rows))
	for _, row := range rows {
		var rep agent.Report
		if json.Unmarshal(row.Scores, &rep) != nil {
			continue
		}
		samples = append(samples, ability.Sample{Report: rep, CreatedAt: row.CreatedAt})
	}
	return toAbilitySummary(ability.Aggregate(samples)), nil
}

// postParentStageProse handles POST .../parent-stage-report/{weekStart}/prose —
// the ONLY stage endpoint that spends. Compose-once (first-open-wins); a failed
// composition never walls (the deterministic stats still render).
func (a *API) postParentStageProse(w http.ResponseWriter, r *http.Request) {
	classID, userID, ok := a.authTeacherStudent(w, r)
	if !ok {
		return
	}
	start, err := resolveWeekStart(r.PathValue("weekStart"), time.Now())
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	ctx := r.Context()
	data, err := a.loadParentStage(ctx, classID, userID, start)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	// Already composed → return it, no spend.
	if existing, gerr := a.getParentStageProse(ctx, userID, data.ScopeID); gerr == nil {
		httpx.WriteJSON(w, http.StatusOK, parentStageDTO(data, existing))
		return
	} else if !errors.Is(gerr, pgx.ErrNoRows) {
		httpx.WriteError(w, r, gerr)
		return
	}

	prose, spent := a.composeParentStageProse(ctx, r, data)
	if !spent {
		httpx.WriteJSON(w, http.StatusOK, parentStageDTO(data, nil))
		return
	}
	raw, merr := json.Marshal(prose)
	if merr != nil {
		httpx.WriteJSON(w, http.StatusOK, parentStageDTO(data, nil))
		return
	}
	if ierr := a.d.Queries.InsertParentReportProse(ctx, sqlc.InsertParentReportProseParams{
		StudentUserID: userID, Surface: "stage", ScopeID: data.ScopeID, Prose: raw,
	}); ierr != nil {
		httpx.WriteError(w, r, ierr)
		return
	}
	stored, serr := a.getParentStageProse(ctx, userID, data.ScopeID)
	if serr != nil {
		httpx.WriteJSON(w, http.StatusOK, parentStageDTO(data, &prose))
		return
	}
	httpx.WriteJSON(w, http.StatusOK, parentStageDTO(data, stored))
}

// composeParentStageProse makes the flagship call and records its cost —
// including on rejection. ProjectID is NULL (a stage report is not
// project-scoped). spent=false means "no prose this time", never a client error.
func (a *API) composeParentStageProse(ctx context.Context, r *http.Request, data parentStageData) (agent.ParentStageProse, bool) {
	resolved, rerr := a.d.EvalResolver(ctx)
	if rerr != nil {
		slog.Warn("parent stage prose: no provider", "err", rerr)
		return agent.ParentStageProse{}, false
	}
	summary, aerr := a.studentAbility(ctx, data.StudentID)
	if aerr != nil {
		slog.Warn("parent stage prose: ability", "err", aerr)
		return agent.ParentStageProse{}, false
	}
	facts := agent.ParentStageFacts{
		Name: data.Name, Subject: data.WeekLabel, Klass: data.Klass,
		ActiveDays: data.ActiveDays, Turns: data.Turns, Reports: data.Reports, CourseSteps: data.CourseSteps,
		Ability: summary,
	}
	prose, usage, cerr := agent.ComposeParentStage(ctx, a.d.Provider, resolved, facts)
	if u, ok := UserFromContext(ctx); ok && resolved.Provider != "" {
		cost, priced := gateway.EstimateCost(resolved.Provider, resolved.Model, usage.InputTokens, usage.OutputTokens)
		if !priced {
			slog.Warn("parent stage llm_call: unpriced model — cost recorded as 0", "provider", resolved.Provider, "model", resolved.Model)
		}
		if _, err := a.d.Queries.RecordLLMCall(ctx, sqlc.RecordLLMCallParams{
			UserID: u.ID, ProjectID: pgtype.UUID{Valid: false}, // stage is not project-scoped
			Surface: "teacher", Purpose: "parent_report",
			Provider: resolved.Provider, Model: resolved.Model, Tier: resolved.Tier,
			PromptTokens: int32(usage.InputTokens), CompletionTokens: int32(usage.OutputTokens),
			CostEstimate: gateway.CostNumeric(cost, true),
		}); err != nil {
			slog.Warn("parent stage prose: record llm call", "err", err)
		}
	}
	if cerr != nil {
		slog.Warn("parent stage prose: rejected", "err", cerr)
		return agent.ParentStageProse{}, false
	}
	return prose, true
}
