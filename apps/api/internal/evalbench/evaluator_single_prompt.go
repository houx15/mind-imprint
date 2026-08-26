package evalbench

import (
	"bytes"
	"context"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"mindimprint/api/internal/evalreport"
	"mindimprint/api/internal/gateway"
	"mindimprint/api/internal/rubric"
)

const (
	singlePromptEvaluatorID      = "single-prompt-evalreport-v1"
	singlePromptPurpose          = "single_prompt_evalreport_v1"
	singlePromptImplementationV1 = "single-prompt-generator-v1-24k-json-schema"
	singlePromptTemplateVersion  = "single-prompt-evalreport-v1"
	singlePromptMaxTokens        = 24000
)

//go:embed prompts/single_prompt_evalreport_v1.md
var singlePromptTemplate string

//go:embed prompts/schemas/evaluation_report_model_output_v1.schema.json
var singlePromptSchema string

//go:embed prompts/schemas/evaluation_report_model_output_v1.example.json
var singlePromptExample string

var singlePromptOutputSchema = mustCompileSchema("evaluation-report-model-output-v1", []byte(singlePromptSchema))

// singlePromptReply is deliberately restricted to the model-authored half of
// EvaluationReport. The evaluator always copies FACT fields from Input.
type singlePromptReply struct {
	Abstract   evalreport.Abstract            `json:"abstract"`
	Depth      []evalreport.DepthDimResult    `json:"depth"`
	Autonomy   []evalreport.AutonomyDimResult `json:"autonomy"`
	PromptLens evalreport.PromptLens          `json:"promptLens"`
	Risks      []evalreport.RiskEntry         `json:"risks"`
}

type singlePromptProfile struct {
	id                    string
	purpose               string
	implementationVersion string
	maxTokens             int
	responseFormat        gateway.ResponseFormat
	promptVersion         string
	promptTemplate        string
	contractSchema        string
	contractExample       string
}

var singlePromptV1Profile = singlePromptProfile{
	id: singlePromptEvaluatorID, purpose: singlePromptPurpose, implementationVersion: singlePromptImplementationV1, maxTokens: singlePromptMaxTokens, responseFormat: gateway.ResponseFormatJSONObject, promptVersion: singlePromptTemplateVersion, promptTemplate: singlePromptTemplate, contractSchema: singlePromptSchema, contractExample: singlePromptExample,
}

type singlePromptEvalReportV1 struct{}

func (singlePromptEvalReportV1) ID() string { return singlePromptEvaluatorID }

func (singlePromptEvalReportV1) Descriptor() EvaluatorDescriptor {
	return singlePromptV1Profile.descriptor()
}

func (p singlePromptProfile) descriptor() EvaluatorDescriptor {
	return EvaluatorDescriptor{
		ID:                    p.id,
		ImplementationVersion: p.implementationVersion,
		PromptVersion:         p.promptVersion,
		PromptSHA256:          p.fingerprint(),
	}
}

func (singlePromptEvalReportV1) ValidateParams(raw json.RawMessage) error {
	return singlePromptV1Profile.validateParams(raw)
}

func (p singlePromptProfile) validateParams(raw json.RawMessage) error {
	if len(raw) == 0 {
		return nil
	}
	var params map[string]any
	if err := json.Unmarshal(raw, &params); err != nil {
		return fmt.Errorf("%s params: %w", p.id, err)
	}
	if len(params) != 0 {
		return fmt.Errorf("%s accepts no params", p.id)
	}
	return nil
}

func (singlePromptEvalReportV1) Run(ctx context.Context, deps RunDeps, in Input, _ json.RawMessage) (EvaluationResult, error) {
	return singlePromptV1Profile.run(ctx, deps, in)
}

func (p singlePromptProfile) run(ctx context.Context, deps RunDeps, in Input) (EvaluationResult, error) {
	if deps.ProviderForPurpose == nil {
		return EvaluationResult{}, fmt.Errorf("%s: ProviderForPurpose is required", p.id)
	}
	if in.GeneratedAt == "" {
		in.GeneratedAt = time.Now().UTC().Format(time.RFC3339)
	}
	req := gateway.ChatRequest{
		MaxTokens:      p.maxTokens,
		ResponseFormat: p.responseFormat,
		Messages: []gateway.ChatMessage{
			{Role: gateway.RoleSystem, Content: renderSinglePromptSystem(p.promptTemplate, p.contractSchema, p.contractExample)},
			{Role: gateway.RoleUser, Content: renderSinglePromptInput(in)},
		},
	}
	// Exactly one model request: malformed content is an experimental failure,
	// not a reason to hide single-prompt fragility behind an internal retry.
	res, err := collectEvalbench(ctx, deps.ProviderForPurpose(p.purpose), deps.Resolved, req)
	if err != nil {
		return EvaluationResult{}, err
	}
	reply, err := decodeSinglePromptReply(p.id, res.Text)
	if err != nil {
		return EvaluationResult{}, err
	}
	depth, err := canonicalDepth(p.id, reply.Depth)
	if err != nil {
		return EvaluationResult{}, err
	}
	autonomy, err := canonicalAutonomy(p.id, reply.Autonomy)
	if err != nil {
		return EvaluationResult{}, err
	}
	report := evalreport.Report{
		Version:     1,
		ReportID:    in.ReportID,
		ProjectID:   in.ProjectID,
		Basics:      in.Basics,
		Abstract:    reply.Abstract,
		Events:      in.Events,
		Materials:   in.Materials,
		Depth:       depth,
		Autonomy:    autonomy,
		PromptLens:  reply.PromptLens,
		ToolUsage:   in.ToolUsage,
		Risks:       reply.Risks,
		GeneratedAt: in.GeneratedAt,
	}
	report.Student.ID, report.Student.Name = in.StudentID, in.StudentName
	normalizeReportArrays(&report)
	evalreport.ValidateRefs(&report, evalreport.NewEvidenceIndex(in.Candidates))
	if err := validateCompleteReport(report); err != nil {
		return EvaluationResult{}, err
	}
	return EvaluationResult{Report: report, Artifacts: map[string]json.RawMessage{}, Complete: true}, nil
}

func (p singlePromptProfile) fingerprint() string {
	rubricJSON, _ := json.Marshal(rubric.Model())
	contents := p.promptTemplate + "\n--schema--\n" + p.contractSchema + "\n--example--\n" + p.contractExample + "\n--rubric--\n" + string(rubricJSON)
	sum := sha256.Sum256([]byte(contents))
	return hex.EncodeToString(sum[:])
}

func renderSinglePromptSystem(template, contractSchema, contractExample string) string {
	m := rubric.Model()
	var b strings.Builder
	b.WriteString(strings.TrimSpace(template))
	if contractSchema != "" {
		b.WriteString("\n\n## 完整 JSON Schema（模型输出的唯一结构契约）\n```json\n")
		b.WriteString(strings.TrimSpace(contractSchema))
		b.WriteString("\n```\n\n## 最小完整 JSON 实例（仅示例结构，不能复制尖括号占位值）\n```json\n")
		b.WriteString(strings.TrimSpace(contractExample))
		b.WriteString("\n```\n")
	}
	b.WriteString("\n\n## 当前 D/A rubric（唯一评分口径）\n")
	b.WriteString("公理：" + m.Axiom + "\n")
	b.WriteString("机会规则：" + m.OpportunityRule + "\n\n")
	b.WriteString("### D1–D6\n")
	for _, d := range m.Depth {
		fmt.Fprintf(&b, "%s %s — %s\nL1：%s\nL2：%s\nL3：%s\nL4：%s\n", d.ID, d.Name, d.Means, d.Anchors["L1"], d.Anchors["L2"], d.Anchors["L3"], d.Anchors["L4"])
		if d.ReflectionRule != "" {
			fmt.Fprintf(&b, "特别规则：%s\n", d.ReflectionRule)
		}
	}
	b.WriteString("\n### A1–A6\n")
	for _, a := range m.Autonomy {
		fmt.Fprintf(&b, "%s %s — %s（可计入事件：%s）\n", a.ID, a.Name, a.Means, a.Event)
	}
	fmt.Fprintf(&b, "\nA 轴 0–5 band 规则：%s\n", m.AutonomyBand)
	return b.String()
}

func renderSinglePromptInput(in Input) string {
	return fmt.Sprintf("题目：%s\n\n〔客观计数〕%s\n\n【可引用证据】\n%s\n\n【全过程记录】\n%s\n\n【学生对 AI 的提问】\n%s\n\n【风险信号】\n%s\n", in.Context.Title, in.Context.Counters, in.Context.Candidates, in.Context.Trajectory, in.Context.Prompts, in.Context.RiskSignals)
}

func decodeSinglePromptReply(evaluatorID, text string) (singlePromptReply, error) {
	var reply singlePromptReply
	raw := []byte(extractJSON(text))
	if err := validateSchemaJSON(singlePromptOutputSchema, raw); err != nil {
		return singlePromptReply{}, fmt.Errorf("%s: invalid model-output contract: %w", evaluatorID, err)
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&reply); err != nil {
		return singlePromptReply{}, fmt.Errorf("%s: invalid JSON: %w", evaluatorID, err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		if err == nil {
			return singlePromptReply{}, fmt.Errorf("%s: JSON has trailing value", evaluatorID)
		}
		return singlePromptReply{}, fmt.Errorf("%s: invalid JSON suffix: %w", evaluatorID, err)
	}
	return reply, nil
}

func canonicalDepth(evaluatorID string, got []evalreport.DepthDimResult) ([]evalreport.DepthDimResult, error) {
	return canonicalAxis(evaluatorID, got, []string{"D1", "D2", "D3", "D4", "D5", "D6"}, func(v evalreport.DepthDimResult) string { return v.ID })
}

func canonicalAutonomy(evaluatorID string, got []evalreport.AutonomyDimResult) ([]evalreport.AutonomyDimResult, error) {
	return canonicalAxis(evaluatorID, got, []string{"A1", "A2", "A3", "A4", "A5", "A6"}, func(v evalreport.AutonomyDimResult) string { return v.ID })
}

func canonicalAxis[T any](evaluatorID string, got []T, ids []string, idOf func(T) string) ([]T, error) {
	if len(got) != len(ids) {
		return nil, fmt.Errorf("%s: axis has %d entries, want %d", evaluatorID, len(got), len(ids))
	}
	allowed := make(map[string]bool, len(ids))
	byID := make(map[string]T, len(got))
	for _, id := range ids {
		allowed[id] = true
	}
	for _, item := range got {
		id := idOf(item)
		if !allowed[id] {
			return nil, fmt.Errorf("%s: axis has unknown id %q", evaluatorID, id)
		}
		if _, duplicate := byID[id]; duplicate {
			return nil, fmt.Errorf("%s: axis has duplicate id %q", evaluatorID, id)
		}
		byID[id] = item
	}
	out := make([]T, 0, len(ids))
	for _, id := range ids {
		item, ok := byID[id]
		if !ok {
			return nil, fmt.Errorf("%s: axis missing id %q", evaluatorID, id)
		}
		out = append(out, item)
	}
	return out, nil
}
