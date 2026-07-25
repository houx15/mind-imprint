package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"mindimprint/api/internal/agent"
	"mindimprint/api/internal/httpx"
	"mindimprint/api/internal/parent"
	"mindimprint/api/internal/store/sqlc"
)

type ParentCoverDTO struct {
	Name      string `json:"name"`
	Subject   string `json:"subject"`
	Klass     string `json:"klass"`
	TypeLabel string `json:"typeLabel"`
	DateStr   string `json:"dateStr"`
	WarmLine  string `json:"warmLine"`
}

type ParentDRowDTO struct {
	Code    string `json:"code"`
	Name    string `json:"name"`
	Badge   string `json:"badge"`
	Reading string `json:"reading"`
}

type ParentARowDTO struct {
	Code    string `json:"code"`
	Name    string `json:"name"`
	State   string `json:"state"`
	Reading string `json:"reading"`
}

type ParentAdviceDTO struct {
	Title string `json:"title"`
	Text  string `json:"text"`
}

// ParentReportDTO is the whole parent page. Badges/states/rows are computed on
// this read; glance/overviews/opportunity/readings/warmLine/advice come from
// stored prose (nil until composed). ProseReady mirrors prose presence.
type ParentReportDTO struct {
	Cover       ParentCoverDTO    `json:"cover"`
	Glance      string            `json:"glance"`
	DOverview   string            `json:"dOverview"`
	AOverview   string            `json:"aOverview"`
	DRows       []ParentDRowDTO   `json:"dRows"`
	ARows       []ParentARowDTO   `json:"aRows"`
	Opportunity string            `json:"opportunity"`
	Advice      []ParentAdviceDTO `json:"advice"`
	ProseReady  bool              `json:"proseReady"`
	// prose sentinel mirrors the contract's `prose: "present" | null`.
	Prose *string `json:"prose"`
}

// parentReportData is everything both handlers need: the canonical report plus
// cover identity. GET renders it; POST composes prose over it.
type parentReportData struct {
	Report    agent.Report
	Name      string
	Subject   string
	Klass     string
	ScopeID   uuid.UUID
	Surface   string
	StudentID uuid.UUID
}

// loadParentReport runs the deterministic layer for one (student, project
// scope): fetch the canonical report + cover identity. No model call. E1
// serves surface "project" only.
func (a *API) loadParentReport(ctx context.Context, classID, userID uuid.UUID, surface string, scopeID uuid.UUID) (parentReportData, error) {
	if surface != "project" {
		return parentReportData{}, httpx.ErrNotFound("资源不存在")
	}
	row, err := a.d.Queries.GetStudentProjectEvaluationForTeacher(ctx, sqlc.GetStudentProjectEvaluationForTeacherParams{
		ScopeID: pgtype.UUID{Bytes: scopeID, Valid: true}, UserID: userID,
	})
	if err != nil {
		return parentReportData{}, err
	}
	var rep agent.Report
	if err := json.Unmarshal(row.Scores, &rep); err != nil {
		return parentReportData{}, err
	}
	user, err := a.d.Queries.GetUserByID(ctx, userID)
	if err != nil {
		return parentReportData{}, err
	}
	cls, err := a.d.Queries.GetClassByID(ctx, classID)
	if err != nil {
		return parentReportData{}, err
	}
	return parentReportData{
		Report: rep, Name: user.DisplayName, Subject: row.ProjectTitle, Klass: cls.Name,
		ScopeID: scopeID, Surface: surface, StudentID: userID,
	}, nil
}

// parentReportDTO merges the deterministic projection with whatever prose
// exists. NA depth dims render 暂无 + 「暂无可计入的证据」 regardless of prose
// (敢于空白). No autonomy number ever appears.
func parentReportDTO(d parentReportData, prose *agent.ParentProse) ParentReportDTO {
	warm := ""
	if prose != nil {
		warm = prose.WarmLine
	}
	dto := ParentReportDTO{
		Cover: ParentCoverDTO{
			Name: d.Name, Subject: "研究项目 · " + d.Subject, Klass: d.Klass,
			TypeLabel: "项目报告", DateStr: parentDateStr(time.Now()), WarmLine: warm,
		},
	}
	for _, dim := range d.Report.DepthAxis {
		badge := parent.DBadge(dim.Level)
		reading := ""
		if badge == "暂无" {
			reading = "暂无可计入的证据"
		} else if prose != nil {
			reading = prose.DReadings[dim.Code]
		}
		dto.DRows = append(dto.DRows, ParentDRowDTO{Code: dim.Code, Name: dim.Name, Badge: badge, Reading: reading})
	}
	for _, sig := range d.Report.AutonomyAxis {
		reading := ""
		if prose != nil {
			reading = prose.AReadings[sig.Code]
		}
		dto.ARows = append(dto.ARows, ParentARowDTO{Code: sig.Code, Name: sig.Name, State: parent.AState(sig.Level), Reading: reading})
	}
	if prose != nil {
		dto.Glance, dto.DOverview, dto.AOverview, dto.Opportunity = prose.Glance, prose.DOverview, prose.AOverview, prose.Opportunity
		for _, ad := range prose.Advice {
			dto.Advice = append(dto.Advice, ParentAdviceDTO{Title: ad.Title, Text: ad.Text})
		}
		present := "present"
		dto.Prose = &present
		dto.ProseReady = true
	}
	return dto
}

// parentDateStr renders the cover date as 2026年7月25日.
func parentDateStr(t time.Time) string {
	t = t.UTC()
	return fmt.Sprintf("%d年%d月%d日", t.Year(), int(t.Month()), t.Day())
}

// getParentReport handles GET .../parent-report/{surface}/{scopeId}. Computes
// the deterministic projection live and merges stored prose when present. This
// handler NEVER calls a model.
func (a *API) getParentReport(w http.ResponseWriter, r *http.Request) {
	classID, userID, ok := a.authTeacherStudent(w, r)
	if !ok {
		return
	}
	surface := r.PathValue("surface")
	scopeID, err := uuid.Parse(r.PathValue("scopeId"))
	if err != nil {
		httpx.WriteError(w, r, httpx.ErrNotFound("资源不存在"))
		return
	}
	data, err := a.loadParentReport(r.Context(), classID, userID, surface, scopeID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	prose, perr := a.getParentProse(r.Context(), userID, surface, scopeID)
	switch {
	case perr == nil:
		httpx.WriteJSON(w, http.StatusOK, parentReportDTO(data, prose))
	case errors.Is(perr, pgx.ErrNoRows):
		httpx.WriteJSON(w, http.StatusOK, parentReportDTO(data, nil))
	default:
		httpx.WriteError(w, r, perr)
	}
}

// getParentProse reads and decodes the stored bundle, or returns pgx.ErrNoRows.
func (a *API) getParentProse(ctx context.Context, userID uuid.UUID, surface string, scopeID uuid.UUID) (*agent.ParentProse, error) {
	raw, err := a.d.Queries.GetParentReportProse(ctx, sqlc.GetParentReportProseParams{
		StudentUserID: userID, Surface: surface, ScopeID: scopeID.String(),
	})
	if err != nil {
		return nil, err
	}
	var p agent.ParentProse
	if err := json.Unmarshal(raw, &p); err != nil {
		return nil, err
	}
	return &p, nil
}

// postParentReportProse is a TEMPORARY stub; Task 5 replaces it with the real
// compose-once-and-store handler.
func (a *API) postParentReportProse(w http.ResponseWriter, r *http.Request) {
	httpx.WriteError(w, r, httpx.ErrNotFound("资源不存在"))
}
