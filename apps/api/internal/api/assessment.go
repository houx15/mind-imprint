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
	dto, derr := reportDTOFromEvaluationRow(row)
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
func (a *API) generateProjectReport(ctx context.Context, projectID uuid.UUID) (studio.ReportDTO, error) {
	d, err := studio.Load(ctx, a.d.Queries, projectID)
	if err != nil {
		return studio.ReportDTO{}, err
	}
	sk, skOK := skills.ByID("writing-project")
	if !skOK {
		return studio.ReportDTO{}, httpx.ErrInternal()
	}
	proj, err := studio.Project(sk, a.d.SpecByID, d)
	if err != nil {
		return studio.ReportDTO{}, err
	}
	// Include the current edit-buffer draft (never-committed work still counts).
	draft, derr := a.d.Queries.GetEditBuffer(ctx, projectID)
	if derr != nil && !errors.Is(derr, pgx.ErrNoRows) {
		return studio.ReportDTO{}, derr
	}
	in := buildAssessmentInputFromProject(d, proj, enrichedProjectGraphSummary(ctx, a.d.Queries, projectID), draft)
	// S5 · feed the student's AI-use self-report + objective interaction record to
	// the assessor (evidence for the responsible-AI-use lens). Best-effort; only
	// when the student actually authored a statement.
	if st, serr := a.d.Queries.GetProjectAIUse(ctx, projectID); serr == nil && (st.UsedFor != "" || st.NotUsedFor != "") {
		in.AIUse = agent.AIUseForAssessment{
			UsedFor: st.UsedFor, NotUsedFor: st.NotUsedFor,
			RecordLine: aiUseRecordLine(a.buildProjectAIUseRecord(ctx, projectID)),
		}
	}

	resolved, rerr := a.d.EvalResolver(ctx)
	if rerr != nil {
		return studio.ReportDTO{}, httpx.ErrInternal()
	}
	report, usage, aerr := agent.AssessReport(ctx, a.d.Provider, resolved, rubric.Model(), in)

	store := agent.NewSqlcAgentStore(a.d.Queries, a.d.Pool)
	if usage.InputTokens > 0 || usage.OutputTokens > 0 {
		if err := store.RecordLLMCall(ctx, agent.LLMCallRow{
			ProjectID: projectID, Surface: "studio", Purpose: "assessment",
			Resolved: resolved, PromptTokens: int32(usage.InputTokens), CompletionTokens: int32(usage.OutputTokens),
		}); err != nil {
			slog.Warn("generate_project_report: record llm call", "err", err)
		}
	}
	if aerr != nil {
		slog.Warn("generate_project_report: rejected", "err", aerr)
		return studio.ReportDTO{}, errAssessmentRejected
	}

	scoresJSON, merr := json.Marshal(report)
	if merr != nil {
		return studio.ReportDTO{}, httpx.ErrInternal()
	}
	cost, priced := gateway.EstimateCost(resolved.Provider, resolved.Model, usage.InputTokens, usage.OutputTokens)
	promptTokens := int32(usage.InputTokens)
	completionTokens := int32(usage.OutputTokens)
	row, err := a.d.Queries.InsertProjectEvaluation(ctx, sqlc.InsertProjectEvaluationParams{
		ProjectID:        pgtype.UUID{Bytes: projectID, Valid: true},
		Scores:           scoresJSON,
		Narrative:        report.Narrative,
		Model:            resolved.Model,
		Tier:             resolved.Tier,
		PromptTokens:     &promptTokens,
		CompletionTokens: &completionTokens,
		CostEstimate:     gateway.CostNumeric(cost, priced),
	})
	if err != nil {
		return studio.ReportDTO{}, err
	}
	return studio.ToReportDTO(report, row.CreatedAt), nil
}

// reportDTOFromEvaluationRow reconstructs the DualAxis wire DTO from a
// persisted evaluations row: row.Scores IS the marshalled agent.Report
// (generateProjectReport's own json.Marshal above) — reconstruction is one
// json.Unmarshal, never string-splitting. Shared by project/chat/course —
// every surface now writes via AssessReport (Task 6).
func reportDTOFromEvaluationRow(row sqlc.Evaluation) (studio.ReportDTO, error) {
	var report agent.Report
	if err := json.Unmarshal(row.Scores, &report); err != nil {
		return studio.ReportDTO{}, err
	}
	return studio.ToReportDTO(report, row.CreatedAt), nil
}

// buildAssessmentInputFromProject maps the loaded ProjectData + its already-
// projected StudioProjection onto agent.BuildAssessmentInput's primitive
// slices. Reuses the SAME derivations the live studio views already trust
// (proj.Coach.Equipment for 自发/提示后 spont, proj.Readiness for review
// bands, proj.Stations for gate progress) rather than recomputing any of
// them independently — the studio.Project call above is the one place those
// facts are derived.
func buildAssessmentInputFromProject(d studio.ProjectData, proj studio.StudioProjection, graph, draft string) agent.AssessmentInput {
	return agent.BuildAssessmentInput(
		eventDigestsFromProject(d.Events),
		cardUsesFromProject(d, proj.Coach.Equipment),
		dispositionUsesFromProject(d),
		gateProgressLines(proj.Stations),
		wordCountsFromProject(d),
		reviewBandsFromProject(proj.Readiness),
		graph,
		roundsFromProject(d),
		true, // ProjectProjection — the writing-project template is the one project surface
		workSamplesFromProject(d, draft),
	)
}

// enrichedProjectGraphSummary prepends the real kick-off (proposal's four
// dimensions), the student's reflection answers, and the outline text to the
// claim/evidence graph summary, so the flagship assessor sees the project's
// intent + reflection + structure — not only the argument graph. Every part is
// best-effort and only included when it actually exists (no fabrication); a
// read failure just drops that part.
func enrichedProjectGraphSummary(ctx context.Context, q *sqlc.Queries, projectID uuid.UUID) string {
	base := graphSummary(ctx, q, projectID)
	var b strings.Builder
	if p, err := q.GetProjectProposal(ctx, projectID); err == nil {
		dims := []struct{ label, val string }{
			{"目标", p.Objective}, {"为什么做", p.Reason},
			{"打算怎么做", p.Activities}, {"需要什么", p.Resources},
		}
		wrote := false
		for _, dm := range dims {
			if strings.TrimSpace(dm.val) != "" {
				if !wrote {
					b.WriteString("【开题四问】\n")
					wrote = true
				}
				fmt.Fprintf(&b, "- %s：%s\n", dm.label, strings.TrimSpace(dm.val))
			}
		}
	}
	if refl, err := q.GetProjectReflection(ctx, projectID); err == nil {
		var answers []string
		if uerr := json.Unmarshal(refl.Answers, &answers); uerr == nil {
			wrote := false
			for i, ans := range answers {
				if strings.TrimSpace(ans) != "" {
					if !wrote {
						b.WriteString("【学生回顾】\n")
						wrote = true
					}
					fmt.Fprintf(&b, "%d. %s\n", i+1, strings.TrimSpace(ans))
				}
			}
		}
	}
	if nodes, err := q.ListOutlineNodes(ctx, projectID); err == nil && len(nodes) > 0 {
		wrote := false
		for _, n := range nodes {
			if strings.TrimSpace(n.Text) == "" {
				continue
			}
			if !wrote {
				b.WriteString("【写作提纲】\n")
				wrote = true
			}
			indent := strings.Repeat("  ", int(n.Depth))
			fmt.Fprintf(&b, "%s- %s\n", indent, strings.TrimSpace(n.Text))
		}
	}
	if b.Len() == 0 {
		return base
	}
	if strings.TrimSpace(base) != "" {
		return b.String() + "\n【论证图】" + base
	}
	return strings.TrimRight(b.String(), "\n")
}

// workSamplesFromProject reports the student's real writing: the latest
// committed snapshot's text (when one exists) plus the current edit-buffer
// draft (so a draft that was never "committed" still counts). The draft is
// omitted when empty or byte-identical to the snapshot — never padded, never
// double-counted.
func workSamplesFromProject(d studio.ProjectData, draft string) []string {
	out := []string{}
	if d.LatestSnapshot != nil {
		out = append(out, d.LatestSnapshot.Content)
	}
	draft = strings.TrimSpace(draft)
	if draft != "" && (d.LatestSnapshot == nil || strings.TrimSpace(d.LatestSnapshot.Content) != draft) {
		out = append(out, draft)
	}
	return out
}

// roundsFromProject pairs each student-message event ("prompt_sent" — the
// same event studioturn.go appends right after persisting the student's chat
// message, see agent.RunAgentStep's caller) with the immediately preceding
// event as its AI/context frame, numbering rounds 1..N in stream order.
// Reuses eventText — the exact same event-text renderer eventDigestsFromProject
// already trusts — rather than inventing a second reading of the payload. When
// there is no preceding event (a student turn opens the stream), AiContext is
// left empty; that's an honest gap, not invented context.
func roundsFromProject(d studio.ProjectData) []agent.Round {
	rounds := make([]agent.Round, 0)
	var lastContext string
	n := 0
	for _, e := range d.Events {
		if e.Type == "prompt_sent" {
			n++
			rounds = append(rounds, agent.Round{N: n, StudentPrompt: promptText(e), AiContext: lastContext})
			lastContext = ""
			continue
		}
		lastContext = eventText(e)
	}
	return rounds
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

// promptText extracts the student's real prompt text from a prompt_sent
// event payload — "" when absent (honest empty; NEVER eventText's bare
// event-type fallback, which would read as the literal string "prompt_sent"
// and get treated as real evidence by the assessor). studioturn.go and
// chat.go both enrich the prompt_sent payload with {"text": body.UserInput}
// at append time; this is the one place that reads it back.
func promptText(e studio.Event) string {
	var p struct {
		Text string `json:"text"`
	}
	if len(e.Payload) == 0 {
		return ""
	}
	if err := json.Unmarshal(e.Payload, &p); err != nil {
		return ""
	}
	return p.Text
}
