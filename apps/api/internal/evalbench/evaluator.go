package evalbench

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"mindimprint/api/internal/agent"
	"mindimprint/api/internal/evalreport"
	"mindimprint/api/internal/gateway"
)

type RunDeps struct {
	Resolved           gateway.Resolved
	ProviderForPurpose func(string) gateway.Provider
}

type EvaluationResult struct {
	Report    evalreport.Report
	Artifacts map[string]json.RawMessage
	Complete  bool
}

// EvaluatorDescriptor records the versioned behavior behind a configured
// evaluator. The prompt hash is present when an evaluator owns one embedded
// prompt; the production-baseline evaluator identifies its local replay by
// ImplementationVersion and the experiment's git revision.
type EvaluatorDescriptor struct {
	ID                    string `json:"id"`
	ImplementationVersion string `json:"implementationVersion"`
	PromptVersion         string `json:"promptVersion,omitempty"`
	PromptSHA256          string `json:"promptSha256,omitempty"`
}

type Evaluator interface {
	ID() string
	Descriptor() EvaluatorDescriptor
	ValidateParams(json.RawMessage) error
	Run(context.Context, RunDeps, Input, json.RawMessage) (EvaluationResult, error)
}

type productionEvalReportV1 struct{}

func (productionEvalReportV1) ID() string { return "production-evalreport-v1" }
func (productionEvalReportV1) Descriptor() EvaluatorDescriptor {
	return EvaluatorDescriptor{
		ID:                    "production-evalreport-v1",
		ImplementationVersion: "evalbench-production-baseline-v1",
		PromptVersion:         "agent-reportgen-v1",
	}
}
func (productionEvalReportV1) ValidateParams(raw json.RawMessage) error {
	if len(raw) == 0 {
		return nil
	}
	var p map[string]any
	if err := json.Unmarshal(raw, &p); err != nil {
		return fmt.Errorf("production-evalreport-v1 params: %w", err)
	}
	if len(p) != 0 {
		return fmt.Errorf("production-evalreport-v1 accepts no params")
	}
	return nil
}
func (productionEvalReportV1) Run(ctx context.Context, deps RunDeps, in Input, _ json.RawMessage) (EvaluationResult, error) {
	if deps.ProviderForPurpose == nil {
		return EvaluationResult{}, fmt.Errorf("production-evalreport-v1: ProviderForPurpose is required")
	}
	if in.GeneratedAt == "" {
		in.GeneratedAt = time.Now().UTC().Format(time.RFC3339)
	}

	// This intentionally mirrors the production generator's four-call
	// orchestration without sharing production control flow: three independent
	// calls in parallel, then abstract after rubric has returned.
	var promptLens evalreport.PromptLens
	var risks []evalreport.RiskEntry
	var rubricRes agent.RubricResult
	var promptErr, risksErr, rubricErr error
	var wg sync.WaitGroup
	wg.Add(3)
	go func() {
		defer wg.Done()
		promptLens, _, promptErr = agent.GeneratePromptLens(ctx, deps.ProviderForPurpose("eval_report_promptlens"), deps.Resolved, in.Context)
	}()
	go func() {
		defer wg.Done()
		risks, _, risksErr = agent.GenerateRisks(ctx, deps.ProviderForPurpose("eval_report_risks"), deps.Resolved, in.Context)
	}()
	go func() {
		defer wg.Done()
		rubricRes, _, rubricErr = agent.GenerateRubric(ctx, deps.ProviderForPurpose("eval_report_rubric"), deps.Resolved, in.Context)
	}()
	wg.Wait()
	abstract, _, abstractErr := agent.GenerateAbstract(ctx, deps.ProviderForPurpose("eval_report_abstract"), deps.Resolved, in.Context, rubricRes)

	report := evalreport.Report{
		Version: 1, ReportID: in.ReportID, ProjectID: in.ProjectID, Basics: in.Basics,
		Abstract: abstract, Events: in.Events, Materials: in.Materials,
		Depth: fillDepth(rubricRes.Depth), Autonomy: fillAutonomy(rubricRes.Autonomy),
		PromptLens: promptLens, ToolUsage: in.ToolUsage, Risks: risks, GeneratedAt: in.GeneratedAt,
	}
	report.Student.ID, report.Student.Name = in.StudentID, in.StudentName
	normalizeReportArrays(&report)
	evalreport.ValidateRefs(&report, evalreport.NewEvidenceIndex(in.Candidates))
	if err := validateCompleteReport(report); err != nil {
		return EvaluationResult{}, err
	}
	complete := promptErr == nil && risksErr == nil && rubricErr == nil && abstractErr == nil
	return EvaluationResult{Report: report, Artifacts: map[string]json.RawMessage{}, Complete: complete}, nil
}

func EvaluatorByID(id string) (Evaluator, bool) {
	switch id {
	case "production-evalreport-v1":
		return productionEvalReportV1{}, true
	case "single-prompt-evalreport-v1":
		return singlePromptEvalReportV1{}, true
	default:
		return nil, false
	}
}

func evaluatorDescriptors(variants []VariantConfig) map[string]EvaluatorDescriptor {
	out := make(map[string]EvaluatorDescriptor, len(variants))
	for _, v := range variants {
		if evaluator, ok := EvaluatorByID(v.Evaluator); ok {
			out[v.ID] = evaluator.Descriptor()
		}
	}
	return out
}
