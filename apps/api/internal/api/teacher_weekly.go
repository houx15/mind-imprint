package api

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"mindimprint/api/internal/agent"
	"mindimprint/api/internal/gateway"
	"mindimprint/api/internal/httpx"
	"mindimprint/api/internal/store/sqlc"
	"mindimprint/api/internal/teacher"
)

// WeeklyCardDTO is one 值得表扬 / 需要建议 card. Evidence is always present —
// it is deterministic. Lead and action are "" until the prose is composed; a
// card with no wording still renders its tag and its evidence (敢于空白).
type WeeklyCardDTO struct {
	UserID      string `json:"userId"`
	DisplayName string `json:"displayName"`
	AvatarColor string `json:"avatarColor"`
	TagCode     string `json:"tagCode"`
	TagLabel    string `json:"tagLabel"`
	Kind        string `json:"kind"`
	Evidence    string `json:"evidence"`
	Lead        string `json:"lead"`
	Action      string `json:"action"`
	// ReportOverview is the deterministic 综述 teaser from the student's own
	// report; present only on praise cards that have a report. "" otherwise.
	ReportOverview string `json:"reportOverview,omitempty"`
	HasReport      bool   `json:"hasReport"`
	ReportSurface  string `json:"reportSurface,omitempty"`
	ReportScopeID  string `json:"reportScopeId,omitempty"`
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

// WeeklyReportDTO is the whole screen. Every number in it was computed on this
// read; only comment/lead/action come from storage.
type WeeklyReportDTO struct {
	WeekLabel    string          `json:"weekLabel"`
	WeekStart    string          `json:"weekStart"`
	WeekEnd      string          `json:"weekEnd"`
	AsOf         string          `json:"asOf"`
	ClassName    string          `json:"className"`
	ClassSize    int             `json:"classSize"`
	Stats        []WeeklyStatDTO `json:"stats"`
	Praise       []WeeklyCardDTO `json:"praise"`
	Watch        []WeeklyCardDTO `json:"watch"`
	Comment      *string         `json:"comment"`
	ProseReady   bool            `json:"proseReady"`
	IsLatestWeek bool            `json:"isLatestWeek"`
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

// loadWeekly runs the whole deterministic layer for one class over a
// completed week. No model call.
func (a *API) loadWeekly(ctx context.Context, cls sqlc.Class, weekStart, now time.Time) (weeklyData, error) {
	start, end, prevStart, prevEnd := teacher.CompletedWeekWindows(weekStart)

	cur, err := a.d.Queries.GetClassWeekStats(ctx, sqlc.GetClassWeekStatsParams{ClassID: cls.ID, WeekStart: start, WeekEnd: end})
	if err != nil {
		return weeklyData{}, err
	}
	prev, err := a.d.Queries.GetClassWeekStats(ctx, sqlc.GetClassWeekStatsParams{ClassID: cls.ID, WeekStart: prevStart, WeekEnd: prevEnd})
	if err != nil {
		return weeklyData{}, err
	}
	act, err := a.d.Queries.ListClassStudentWeekActivity(ctx, sqlc.ListClassStudentWeekActivityParams{
		ClassID: cls.ID, WeekStart: start, WeekEnd: end, PrevStart: prevStart, PrevEnd: prevEnd,
	})
	if err != nil {
		return weeklyData{}, err
	}
	students := make([]teacher.StudentWeek, 0, len(act))
	for _, u := range act {
		s := teacher.StudentWeek{
			UserID: u.UserID.String(), DisplayName: u.DisplayName, AvatarColor: u.AvatarColor,
			ActiveDays: int(u.ActiveDays), Turns: int(u.Turns), PrevActiveDays: int(u.PrevActiveDays),
			ReportsThisWeek: int(u.ReportsThisWeek), PriorReports: int(u.PriorReports),
		}
		if u.LatestReportProjectID.Valid {
			s.LatestReportProjectID = uuid.UUID(u.LatestReportProjectID.Bytes).String()
		}
		// latest_report_overview is a JSON ->> extract, so sqlc types it as
		// interface{}; pgx decodes the text result to a string (nil when null).
		if ov, ok := u.LatestReportOverview.(string); ok {
			s.LatestReportOverview = ov
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
	start, end, _, _ := teacher.CompletedWeekWindows(d.WeekStart)
	dto := WeeklyReportDTO{
		WeekLabel: teacher.WeekLabel(d.WeekStart),
		WeekStart: start.Format(time.RFC3339), WeekEnd: end.Format(time.RFC3339),
		AsOf:      end.Format(time.RFC3339),
		ClassName: d.Class.Name, ClassSize: d.ClassSize,
		IsLatestWeek: teacher.IsLatestCompletedWeek(d.WeekStart, d.Now),
	}
	for _, s := range d.Stats {
		dto.Stats = append(dto.Stats, WeeklyStatDTO{
			Key: s.Key, Label: s.Label, Value: s.Value, Unit: s.Unit, Foot: s.Foot,
			Delta: s.Delta, DeltaDir: s.DeltaDir,
		})
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
		dto.ProseReady = true
	}
	conv := func(cards []teacher.Card) []WeeklyCardDTO {
		out := make([]WeeklyCardDTO, 0, len(cards))
		for _, c := range cards {
			w := wording[c.UserID]
			out = append(out, WeeklyCardDTO{
				UserID: c.UserID, DisplayName: c.DisplayName, AvatarColor: c.AvatarColor,
				TagCode: c.TagCode, TagLabel: c.TagLabel, Kind: c.Kind, Evidence: c.Evidence,
				Lead: w.Lead, Action: w.Action, ReportOverview: c.ReportOverview,
				HasReport: c.HasReport, ReportSurface: "project", ReportScopeID: c.ReportScopeID,
			})
		}
		return out
	}
	dto.Praise, dto.Watch = conv(d.Weekly.Praise), conv(d.Weekly.Watch)
	return dto
}

// resolveWeekStart reads ?weekStart=<RFC3339> and validates it names a
// completed week's UTC Monday midnight; the default (no param) is the last
// completed week.
func (a *API) resolveWeekStart(r *http.Request, now time.Time) (time.Time, error) {
	q := r.URL.Query().Get("weekStart")
	if q == "" {
		return teacher.LastCompletedWeekStart(now), nil
	}
	ws, err := time.Parse(time.RFC3339, q)
	if err != nil {
		return time.Time{}, httpx.ErrBadRequest("validation_failed", "weekStart 格式不正确", nil)
	}
	if err := teacher.ValidateCompletedWeekStart(ws, now); err != nil {
		return time.Time{}, httpx.ErrBadRequest("validation_failed", "只能查看已结束的周", nil)
	}
	return ws.UTC(), nil
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
	ws, err := a.resolveWeekStart(r, now)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	data, err := a.loadWeekly(r.Context(), cls, ws, now)
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

// postClassWeeklyProse handles POST /api/v1/classes/{id}/weekly-report/prose —
// the ONLY endpoint in D2 that spends. It generates once per (class, week);
// a second call returns the stored row without calling a model (DEC-2), and a
// card that first appeared after generation is topped up (DEC-6) without
// rewriting anything already written.
func (a *API) postClassWeeklyProse(w http.ResponseWriter, r *http.Request) {
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
	ctx := r.Context()
	now := time.Now()
	ws, err := a.resolveWeekStart(r, now)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	data, err := a.loadWeekly(ctx, cls, ws, now)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	// GetClassWeeklyProseParams.WeekStart is pgtype.Date (the column is
	// `date`, not `timestamptz`) — a bare pgtype.Date{Valid: true} with no
	// Time silently encodes 0001-01-01 and reads the wrong week, so every
	// call site below builds it from data.WeekStart explicitly.
	weekParam := pgtype.Date{Time: data.WeekStart, Valid: true}

	existing, gerr := a.d.Queries.GetClassWeeklyProse(ctx, sqlc.GetClassWeeklyProseParams{
		ClassID: id, WeekStart: weekParam,
	})
	hasRow := gerr == nil
	if gerr != nil && !errors.Is(gerr, pgx.ErrNoRows) {
		httpx.WriteError(w, r, gerr)
		return
	}

	facts := teacher.BuildWeeklyFacts(cls.Name, data.ClassSize, teacher.WeekLabel(data.WeekStart), data.Weekly)

	// Nothing to say about: no cards fired. Spending a flagship call to be told
	// so is waste.
	if !hasRow && len(facts.Cards) == 0 {
		httpx.WriteJSON(w, http.StatusOK, weeklyDTO(data, nil))
		return
	}

	if hasRow {
		missing := missingCardFacts(existing.Cards, facts)
		if len(missing) == 0 {
			httpx.WriteJSON(w, http.StatusOK, weeklyDTO(data, &existing))
			return
		}
		topUp := facts
		topUp.Cards = missing
		prose, ok := a.composeWeeklyProse(ctx, r, topUp)
		if ok && len(prose.Cards) > 0 {
			if cards, merr := json.Marshal(prose.Cards); merr == nil {
				if aerr := a.d.Queries.AppendClassWeeklyProseCards(ctx, sqlc.AppendClassWeeklyProseCardsParams{
					ClassID: id, WeekStart: weekParam, Cards: cards,
				}); aerr != nil {
					slog.Warn("weekly prose: append cards", "err", aerr)
				}
			}
		}
		refreshed, rerr := a.d.Queries.GetClassWeeklyProse(ctx, sqlc.GetClassWeeklyProseParams{
			ClassID: id, WeekStart: weekParam,
		})
		if rerr != nil {
			httpx.WriteJSON(w, http.StatusOK, weeklyDTO(data, &existing))
			return
		}
		httpx.WriteJSON(w, http.StatusOK, weeklyDTO(data, &refreshed))
		return
	}

	prose, ok := a.composeWeeklyProse(ctx, r, facts)
	if !ok {
		// 敢于空白: the numbers, the tags and the evidence still render.
		httpx.WriteJSON(w, http.StatusOK, weeklyDTO(data, nil))
		return
	}
	cards, merr := json.Marshal(prose.Cards)
	if merr != nil {
		httpx.WriteJSON(w, http.StatusOK, weeklyDTO(data, nil))
		return
	}
	if ierr := a.d.Queries.InsertClassWeeklyProse(ctx, sqlc.InsertClassWeeklyProseParams{
		ClassID: id, WeekStart: weekParam,
		Comment: prose.Comment, DepthNote: "", AutonomyNote: "",
		Cards: cards,
	}); ierr != nil {
		httpx.WriteError(w, r, ierr)
		return
	}
	// Re-read: a concurrent teacher may have won the insert, and the winner's
	// row is what both of them must see.
	stored, serr := a.d.Queries.GetClassWeeklyProse(ctx, sqlc.GetClassWeeklyProseParams{
		ClassID: id, WeekStart: weekParam,
	})
	if serr != nil {
		httpx.WriteJSON(w, http.StatusOK, weeklyDTO(data, nil))
		return
	}
	httpx.WriteJSON(w, http.StatusOK, weeklyDTO(data, &stored))
}

// composeWeeklyProse makes the flagship call and records its cost — including
// when the output is rejected, since a rejected composition still spent real
// tokens. ok=false means "no prose this time", never an error to the client
// (a failed composition must never wall the screen).
func (a *API) composeWeeklyProse(ctx context.Context, r *http.Request, facts agent.WeeklyFacts) (agent.WeeklyProse, bool) {
	resolved, rerr := a.routeE(ctx, gateway.ClassAssess)
	if rerr != nil {
		slog.Warn("weekly prose: no provider", "err", rerr)
		return agent.WeeklyProse{}, false
	}
	prose, usage, cerr := agent.ComposeWeekly(ctx, a.d.Provider, resolved, facts)
	if u, ok := UserFromContext(ctx); ok && (usage.InputTokens > 0 || usage.OutputTokens > 0) {
		cost, priced := gateway.EstimateCost(resolved.Provider, resolved.Model, usage.InputTokens, usage.OutputTokens)
		if !priced {
			slog.Warn("weekly llm_call: unpriced model — cost recorded as 0", "provider", resolved.Provider, "model", resolved.Model)
		}
		if _, err := a.d.Queries.RecordLLMCall(ctx, sqlc.RecordLLMCallParams{
			UserID: u.ID, ProjectID: pgtype.UUID{Valid: false},
			Surface: "teacher", Purpose: "class_weekly",
			Provider: resolved.Provider, Model: resolved.Model, Tier: resolved.Tier,
			PromptTokens: int32(usage.InputTokens), CompletionTokens: int32(usage.OutputTokens),
			CostEstimate: gateway.CostNumeric(cost, true),
		}); err != nil {
			slog.Warn("weekly prose: record llm call", "err", err)
		}
	}
	if cerr != nil {
		slog.Warn("weekly prose: rejected", "err", cerr)
		return agent.WeeklyProse{}, false
	}
	return prose, true
}

// missingCardFacts returns the fact-sheet cards that the stored prose has no
// wording for yet — the top-up's input. stored is the jsonb []WeeklyCardProse
// already on the row; a card whose userId is already present there must be
// excluded, because AppendClassWeeklyProseCards does not de-duplicate —
// sending it again would double the entry in the stored array.
func missingCardFacts(stored []byte, facts agent.WeeklyFacts) []agent.WeeklyFactCard {
	have := map[string]bool{}
	var cards []agent.WeeklyCardProse
	if json.Unmarshal(stored, &cards) == nil {
		for _, c := range cards {
			have[c.UserID] = true
		}
	}
	var missing []agent.WeeklyFactCard
	for _, c := range facts.Cards {
		if !have[c.UserID] {
			missing = append(missing, c)
		}
	}
	return missing
}
