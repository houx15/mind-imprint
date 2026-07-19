package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"mindimprint/api/internal/agent"
	"mindimprint/api/internal/gateway"
	"mindimprint/api/internal/httpx"
	"mindimprint/api/internal/rubric"
	"mindimprint/api/internal/skills"
	"mindimprint/api/internal/store/sqlc"
	"mindimprint/api/internal/studio"
)

// getAssessment returns the project's most recently generated growth report,
// or an explicit JSON null when none has been generated yet (the growth
// report's own empty state — never a 404: "not yet assessed" is a normal
// state for a project, not a missing resource). No model call, ever.
func (a *API) getAssessment(w http.ResponseWriter, r *http.Request) {
	projectID, ok := a.loadOwnedProject(w, r)
	if !ok {
		return
	}
	row, err := a.d.Queries.GetLatestProjectEvaluation(r.Context(), pgtype.UUID{Bytes: projectID, Valid: true})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			httpx.WriteJSON(w, http.StatusOK, nil)
			return
		}
		httpx.WriteError(w, r, err)
		return
	}
	dto, derr := dtoFromEvaluationRow(row)
	if derr != nil {
		httpx.WriteError(w, r, httpx.ErrInternal())
		return
	}
	httpx.WriteJSON(w, http.StatusOK, dto)
}

// errAssessmentRejected marks a generated report that failed internal
// enforcement — the caller maps it to 422 assessment_rejected. Cost is already
// recorded when this is returned.
var errAssessmentRejected = errors.New("assessment rejected")

// generateProjectReport runs the isolated flagship growth-assessor over the
// project's whole process record and persists ONE project-scoped evaluation.
// It records the call's cost even when the output is then rejected. Returns the
// wire DTO on success; errAssessmentRejected on a rejected output; any other
// error on I/O failure. This is A3's single project-report generation core —
// the finish endpoint is its only caller (the standalone regenerate route is
// gone; one-time generation, DEC-A3.5).
func (a *API) generateProjectReport(ctx context.Context, projectID uuid.UUID) (studio.AssessmentDTO, error) {
	d, err := studio.Load(ctx, a.d.Queries, projectID)
	if err != nil {
		return studio.AssessmentDTO{}, err
	}
	sk, skOK := skills.ByID("writing-project")
	if !skOK {
		return studio.AssessmentDTO{}, httpx.ErrInternal()
	}
	proj, err := studio.Project(sk, a.d.SpecByID, d)
	if err != nil {
		return studio.AssessmentDTO{}, err
	}
	in := buildAssessmentInputFromProject(d, proj, graphSummary(ctx, a.d.Queries, projectID))

	resolved, rerr := a.d.EvalResolver(ctx)
	if rerr != nil {
		return studio.AssessmentDTO{}, httpx.ErrInternal()
	}
	assessment, usage, aerr := agent.Assess(ctx, a.d.Provider, resolved, rubric.CT(), in, agent.EmbeddedAnchors())

	store := agent.NewSqlcAgentStore(a.d.Queries, a.d.Pool)
	if resolved.Provider != "" {
		if err := store.RecordLLMCall(ctx, agent.LLMCallRow{
			ProjectID: projectID, Surface: "studio", Purpose: "assessment",
			Resolved: resolved, PromptTokens: int32(usage.InputTokens), CompletionTokens: int32(usage.OutputTokens),
		}); err != nil {
			slog.Warn("generate_project_report: record llm call", "err", err)
		}
	}
	if aerr != nil {
		slog.Warn("generate_project_report: rejected", "err", aerr)
		return studio.AssessmentDTO{}, errAssessmentRejected
	}

	scoresJSON, merr := json.Marshal(assessment.Dimensions)
	if merr != nil {
		return studio.AssessmentDTO{}, httpx.ErrInternal()
	}
	cost, priced := gateway.EstimateCost(resolved.Provider, resolved.Model, usage.InputTokens, usage.OutputTokens)
	promptTokens := int32(usage.InputTokens)
	completionTokens := int32(usage.OutputTokens)
	row, err := a.d.Queries.InsertProjectEvaluation(ctx, sqlc.InsertProjectEvaluationParams{
		ProjectID:        pgtype.UUID{Bytes: projectID, Valid: true},
		Scores:           scoresJSON,
		Narrative:        assessment.Narrative,
		Model:            resolved.Model,
		Tier:             resolved.Tier,
		PromptTokens:     &promptTokens,
		CompletionTokens: &completionTokens,
		CostEstimate:     gateway.CostNumeric(cost, priced),
	})
	if err != nil {
		return studio.AssessmentDTO{}, err
	}
	return dtoFromEvaluationRow(row)
}

// dtoFromEvaluationRow reconstructs the wire DTO from a persisted evaluations
// row: row.Scores IS the marshalled []agent.DimensionScore (generateProjectReport's
// own json.Marshal above) — reconstruction is one json.Unmarshal, never
// string-splitting.
func dtoFromEvaluationRow(row sqlc.Evaluation) (studio.AssessmentDTO, error) {
	var dims []agent.DimensionScore
	if err := json.Unmarshal(row.Scores, &dims); err != nil {
		return studio.AssessmentDTO{}, err
	}
	assessment := agent.Assessment{Dimensions: dims, Narrative: row.Narrative}
	return studio.ToAssessmentDTO(assessment, row.CreatedAt.Format(time.RFC3339)), nil
}

// buildAssessmentInputFromProject maps the loaded ProjectData + its already-
// projected StudioProjection onto agent.BuildAssessmentInput's primitive
// slices. Reuses the SAME derivations the live studio views already trust
// (proj.Coach.Equipment for 自发/提示后 spont, proj.Readiness for review
// bands, proj.Stations for gate progress) rather than recomputing any of
// them independently — the studio.Project call above is the one place those
// facts are derived.
func buildAssessmentInputFromProject(d studio.ProjectData, proj studio.StudioProjection, graph string) agent.AssessmentInput {
	return agent.BuildAssessmentInput(
		eventDigestsFromProject(d.Events),
		cardUsesFromProject(d, proj.Coach.Equipment),
		dispositionUsesFromProject(d),
		gateProgressLines(proj.Stations),
		wordCountsFromProject(d),
		reviewBandsFromProject(proj.Readiness),
		graph,
		nil, // Rounds — not yet wired for this surface
	)
}

// cardUsesFromProject zips d.Cards with proj.Coach.Equipment — projectEquipment
// (studio/projection.go) builds Equipment by iterating d.Cards in the exact
// same order, so index i's EquipCardDTO carries the 自发/提示后 signal for
// card_instance i. CardID stays the original card id (ci.CardID), not the
// card_instance id EquipCardDTO.ID carries.
func cardUsesFromProject(d studio.ProjectData, equipment []studio.EquipCardDTO) []agent.CardUse {
	out := make([]agent.CardUse, 0, len(d.Cards))
	for i, ci := range d.Cards {
		if i >= len(equipment) {
			break
		}
		out = append(out, agent.CardUse{CardID: ci.CardID, Dimension: equipment[i].Meth, Spont: equipment[i].Spont})
	}
	return out
}

// dispositionUsesFromProject carries each recorded disposition's action/reason
// straight through — the feedback-comprehension signal.
func dispositionUsesFromProject(d studio.ProjectData) []agent.DispositionUse {
	out := make([]agent.DispositionUse, 0, len(d.Dispositions))
	for _, dp := range d.Dispositions {
		out = append(out, agent.DispositionUse{Kind: dp.Action, Reason: dp.Reason})
	}
	return out
}

// wordCountsFromProject honestly reports only what ProjectData actually
// carries: the latest committed snapshot's word count. There is no query
// listing every historical snapshot, so this never claims more than one
// data point — derive-never-decorate.
func wordCountsFromProject(d studio.ProjectData) []int {
	if d.LatestSnapshot == nil {
		return nil
	}
	return []int{agent.CountWords(d.LatestSnapshot.Content)}
}

// gateProgressLines renders one digest line per S0..S6 station, reusing
// proj.Stations (studio.projectStations' own derivation) rather than
// recomputing gate reports independently.
func gateProgressLines(stations []studio.StationDTO) []string {
	out := make([]string, 0, len(stations))
	for _, st := range stations {
		line := fmt.Sprintf("%s %s：%s", st.Code, st.Name, stationStateLabel(st.State))
		if st.Gate != nil {
			line += fmt.Sprintf("（关卡 %d/%d）", st.Gate.Passed, st.Gate.Total)
		}
		out = append(out, line)
	}
	return out
}

func stationStateLabel(state string) string {
	switch state {
	case "done":
		return "已完成"
	case "current":
		return "进行中"
	default:
		return "未开始"
	}
}

// reviewBandsFromProject renders one digest line per 0457 readiness gauge,
// reusing proj.Readiness (studio.projectReadiness's own board-voice review
// derivation) rather than re-deriving it from raw intervention rows.
func reviewBandsFromProject(readiness []studio.GaugeDTO) []string {
	out := make([]string, 0, len(readiness))
	for _, g := range readiness {
		out = append(out, fmt.Sprintf("%s %s：%s", g.Code, g.Name, bandLabel(g.Level)))
	}
	return out
}

func bandLabel(level string) string {
	switch level {
	case "full":
		return "熟练"
	case "partial":
		return "发展中"
	default:
		return "未开始"
	}
}

// eventDigestsFromProject numbers the append-only event stream in its
// persisted (temporal) order — the ordering itself is legible signal
// (unprompted-first reads differently from prompted-first).
func eventDigestsFromProject(events []studio.Event) []agent.EventDigest {
	out := make([]agent.EventDigest, 0, len(events))
	for i, e := range events {
		out = append(out, agent.EventDigest{Type: e.Type, Order: i + 1, Text: eventText(e)})
	}
	return out
}

// eventText renders an event's opaque payload defensively: known or unknown
// shape, it never fails — just falls back to the bare type name.
func eventText(e studio.Event) string {
	if len(e.Payload) == 0 {
		return e.Type
	}
	var m map[string]any
	if err := json.Unmarshal(e.Payload, &m); err != nil || len(m) == 0 {
		return e.Type
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, fmt.Sprintf("%s=%v", k, m[k]))
	}
	return e.Type + "：" + strings.Join(parts, "，")
}
