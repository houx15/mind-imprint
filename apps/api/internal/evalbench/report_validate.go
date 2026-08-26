package evalbench

import (
	_ "embed"
	"encoding/json"
	"fmt"

	"mindimprint/api/internal/evalreport"
)

//go:embed prompts/schemas/evaluation_report_v1.schema.json
var evaluationReportSchemaJSON []byte

var evaluationReportSchema = mustCompileSchema("evaluation-report-v1", evaluationReportSchemaJSON)

func normalizeReportArrays(r *evalreport.Report) {
	if r.Events == nil {
		r.Events = []evalreport.EventEntry{}
	}
	if r.Materials == nil {
		r.Materials = []evalreport.MaterialEntry{}
	}
	if r.Depth == nil {
		r.Depth = []evalreport.DepthDimResult{}
	}
	if r.Autonomy == nil {
		r.Autonomy = []evalreport.AutonomyDimResult{}
	}
	if r.ToolUsage == nil {
		r.ToolUsage = []evalreport.ToolUsageEntry{}
	}
	if r.Risks == nil {
		r.Risks = []evalreport.RiskEntry{}
	}
	if r.Abstract.SuggestionSentences == nil {
		r.Abstract.SuggestionSentences = []string{}
	}
	if r.Abstract.RecommendedCourses == nil {
		r.Abstract.RecommendedCourses = []evalreport.RecommendedCourse{}
	}
	if r.PromptLens.Prompts == nil {
		r.PromptLens.Prompts = []evalreport.PromptItem{}
	}
	for i := range r.Depth {
		if r.Depth[i].Evidence == nil {
			r.Depth[i].Evidence = []evalreport.EvidenceItem{}
		}
	}
	for i := range r.Autonomy {
		if r.Autonomy[i].Evidence == nil {
			r.Autonomy[i].Evidence = []evalreport.EvidenceItem{}
		}
	}
	for i := range r.PromptLens.Prompts {
		if r.PromptLens.Prompts[i].RelatedDomains == nil {
			r.PromptLens.Prompts[i].RelatedDomains = []string{}
		}
	}
}

func validateCompleteReport(r evalreport.Report) error {
	raw, err := json.Marshal(r)
	if err != nil {
		return fmt.Errorf("evalbench: marshal report: %w", err)
	}
	if err := validateSchemaJSON(evaluationReportSchema, raw); err != nil {
		return fmt.Errorf("evalbench: invalid EvaluationReport contract: %w", err)
	}
	if r.Version != 1 || r.ReportID == "" || r.ProjectID == "" || r.Student.ID == "" {
		return fmt.Errorf("evalbench: incomplete report envelope")
	}
	if r.Basics.Title == "" || r.Basics.StartDate == "" {
		return fmt.Errorf("evalbench: missing required basics")
	}
	if err := validateDepth(r.Depth); err != nil {
		return err
	}
	if err := validateAutonomy(r.Autonomy); err != nil {
		return err
	}
	for _, e := range r.Events {
		switch e.Kind {
		case "chat", "reading", "graph", "writing", "review", "milestone":
		default:
			return fmt.Errorf("evalbench: invalid event kind %q", e.Kind)
		}
	}
	for _, risk := range r.Risks {
		switch risk.Type {
		case "ai-ghostwrite", "missing-source", "argument-logic", "data-scope", "rabbit-hole-offtopic":
		default:
			return fmt.Errorf("evalbench: invalid risk type %q", risk.Type)
		}
	}
	if r.Events == nil || r.Materials == nil || r.ToolUsage == nil || r.Risks == nil || r.Abstract.SuggestionSentences == nil || r.Abstract.RecommendedCourses == nil || r.PromptLens.Prompts == nil {
		return fmt.Errorf("evalbench: report contains nil array")
	}
	return nil
}

func validateDepth(got []evalreport.DepthDimResult) error {
	ids := []string{"D1", "D2", "D3", "D4", "D5", "D6"}
	if len(got) != len(ids) {
		return fmt.Errorf("evalbench: depth length = %d, want 6", len(got))
	}
	for i, id := range ids {
		if got[i].ID != id || got[i].Level < 1 || got[i].Level > 4 || got[i].Evidence == nil {
			return fmt.Errorf("evalbench: invalid depth entry %d", i)
		}
	}
	return nil
}

func validateAutonomy(got []evalreport.AutonomyDimResult) error {
	ids := []string{"A1", "A2", "A3", "A4", "A5", "A6"}
	if len(got) != len(ids) {
		return fmt.Errorf("evalbench: autonomy length = %d, want 6", len(got))
	}
	for i, id := range ids {
		if got[i].ID != id || got[i].Band < 0 || got[i].Band > 5 || got[i].Evidence == nil {
			return fmt.Errorf("evalbench: invalid autonomy entry %d", i)
		}
	}
	return nil
}

func fillDepth(got []evalreport.DepthDimResult) []evalreport.DepthDimResult {
	byID := make(map[string]evalreport.DepthDimResult, len(got))
	for _, d := range got {
		byID[d.ID] = d
	}
	out := make([]evalreport.DepthDimResult, 0, 6)
	for _, id := range []string{"D1", "D2", "D3", "D4", "D5", "D6"} {
		if d, ok := byID[id]; ok {
			out = append(out, d)
			continue
		}
		out = append(out, evalreport.DepthDimResult{ID: id, Level: 1, Summary: "暂无足够证据判定这一维。", Suggestion: ""})
	}
	return out
}

func fillAutonomy(got []evalreport.AutonomyDimResult) []evalreport.AutonomyDimResult {
	byID := make(map[string]evalreport.AutonomyDimResult, len(got))
	for _, a := range got {
		byID[a.ID] = a
	}
	out := make([]evalreport.AutonomyDimResult, 0, 6)
	for _, id := range []string{"A1", "A2", "A3", "A4", "A5", "A6"} {
		if a, ok := byID[id]; ok {
			out = append(out, a)
			continue
		}
		out = append(out, evalreport.AutonomyDimResult{ID: id, Band: 0, Summary: "暂无足够证据判定这一维。", Suggestion: ""})
	}
	return out
}
