package evalbench

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"mindimprint/api/internal/evalreport"
	"mindimprint/api/internal/gateway"
)

type ComparisonItem struct {
	Area             string `json:"area"`
	Code             string `json:"code"`
	Aspect           string `json:"aspect"`
	Comparison       string `json:"comparison"`
	Confidence       string `json:"confidence"`
	GoldExcerpt      string `json:"goldExcerpt"`
	CandidateExcerpt string `json:"candidateExcerpt"`
	Reason           string `json:"reason"`
	ManualReview     bool   `json:"manualReview"`
}
type Comparison struct {
	Items []ComparisonItem `json:"items"`
}

const ComparatorPromptVersion = "comparator-json-v1"

// comparisonProjection is the model-authored portion of an EvaluationReport.
// FACT fields stay outside of the LLM comparator by design.
type comparisonProjection struct {
	Abstract   evalreport.Abstract            `json:"abstract"`
	Depth      []evalreport.DepthDimResult    `json:"depth"`
	Autonomy   []evalreport.AutonomyDimResult `json:"autonomy"`
	PromptLens evalreport.PromptLens          `json:"promptLens"`
	Risks      []evalreport.RiskEntry         `json:"risks"`
}

func projectForComparison(report evalreport.Report) comparisonProjection {
	return comparisonProjection{
		Abstract: report.Abstract, Depth: report.Depth, Autonomy: report.Autonomy,
		PromptLens: report.PromptLens, Risks: report.Risks,
	}
}

// ExpectedComparisonItems covers only model-produced judgements/prose. FACT
// sections are validated deterministically by the adapter and never judged by
// an LLM.
func ExpectedComparisonItems() []ComparisonItem {
	out := make([]ComparisonItem, 0, 46)
	for _, id := range []string{"D1", "D2", "D3", "D4", "D5", "D6"} {
		for _, aspect := range []string{"judgement", "evidence", "suggestion"} {
			out = append(out, ComparisonItem{Area: "depth", Code: id, Aspect: aspect})
		}
	}
	for _, id := range []string{"A1", "A2", "A3", "A4", "A5", "A6"} {
		for _, aspect := range []string{"judgement", "evidence", "suggestion"} {
			out = append(out, ComparisonItem{Area: "autonomy", Code: id, Aspect: aspect})
		}
	}
	for _, aspect := range []string{"overview", "materials", "writing", "ai-boundary", "guidance"} {
		out = append(out, ComparisonItem{Area: "abstract", Code: "abstract", Aspect: aspect})
	}
	for _, aspect := range []string{"summary", "representative-prompts", "attention-flags"} {
		out = append(out, ComparisonItem{Area: "promptLens", Code: "promptLens", Aspect: aspect})
	}
	for _, aspect := range []string{"coverage", "false-positive-boundary"} {
		out = append(out, ComparisonItem{Area: "risks", Code: "risks", Aspect: aspect})
	}
	return out
}

func Compare(ctx context.Context, rt *Runtime, use ModelUse, candidate, gold evalreport.Report, recorder *CallRecorder) (Comparison, error) {
	resolved, _, err := rt.Resolve(use.Model)
	if err != nil {
		return Comparison{}, err
	}
	// Deliberately exclude deterministic FACT sections from both judge inputs.
	candidateJSON, err := json.Marshal(projectForComparison(candidate))
	if err != nil {
		return Comparison{}, err
	}
	goldJSON, err := json.Marshal(projectForComparison(gold))
	if err != nil {
		return Comparison{}, err
	}
	// The comparator needs to emit a complete, fixed 46-cell JSON matrix. On
	// DeepSeek reasoning mode can consume the full output budget before any JSON
	// token is streamed, so keep the same flagship model but reserve its budget
	// for the judge's structured output.
	req := gateway.ChatRequest{MaxTokens: 12000, DisableThinking: true, Messages: []gateway.ChatMessage{{Role: gateway.RoleSystem, Content: comparatorPrompt()}, {Role: gateway.RoleUser, Content: "人工 Gold EvaluationReport 的模型部分 JSON：\n" + string(goldJSON) + "\n\n候选 EvaluationReport 的模型部分 JSON：\n" + string(candidateJSON)}}}
	res, err := collectEvalbench(ctx, rt.Observed("comparator", recorder), resolved, req)
	if err != nil {
		return Comparison{}, err
	}
	var c Comparison
	if err := json.Unmarshal([]byte(extractJSON(res.Text)), &c); err != nil {
		return Comparison{}, fmt.Errorf("evalbench: comparator returned invalid JSON: %w", err)
	}
	if err := ValidateComparison(c); err != nil {
		return Comparison{}, err
	}
	return c, nil
}

func comparatorPrompt() string {
	expected, _ := json.Marshal(ExpectedComparisonItems())
	return `你只比较人工 Gold EvaluationReport JSON 与候选评估的模型生成部分，不读取或推断学生过程，也不判断绝对正确性。每项只相对 Gold：aligned、overstates、understates、not_comparable。Gold 缺失、含糊、非序数状态不可定向或低置信度时，必须 not_comparable 且 manualReview=true。忽略 FACT（basics/events/materials/toolUsage）。必须只输出 JSON：{"items":[...]}; items 必须与下面完整矩阵一一对应，不可新增、遗漏或重复。每项带 goldExcerpt、candidateExcerpt、reason，且 excerpt 必须来自给定输入。完整矩阵：` + string(expected)
}
func extractJSON(s string) string {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "```json")
	s = strings.TrimPrefix(s, "```")
	s = strings.TrimSuffix(s, "```")
	return strings.TrimSpace(s)
}
func ValidateComparison(c Comparison) error {
	expected := map[string]bool{}
	for _, i := range ExpectedComparisonItems() {
		expected[comparisonKey(i)] = true
	}
	if len(c.Items) != len(expected) {
		return fmt.Errorf("evalbench: comparator matrix has %d items, want %d", len(c.Items), len(expected))
	}
	seen := map[string]bool{}
	for n := range c.Items {
		i := &c.Items[n]
		key := comparisonKey(*i)
		if !expected[key] || seen[key] {
			return fmt.Errorf("evalbench: comparator emitted unknown or duplicate item %q", key)
		}
		if i.Comparison != "aligned" && i.Comparison != "overstates" && i.Comparison != "understates" && i.Comparison != "not_comparable" {
			return fmt.Errorf("evalbench: invalid comparison %q", i.Comparison)
		}
		if i.Confidence != "high" && i.Confidence != "medium" && i.Confidence != "low" {
			return fmt.Errorf("evalbench: invalid confidence %q", i.Confidence)
		}
		if i.Confidence == "low" {
			i.Comparison, i.ManualReview = "not_comparable", true
		}
		seen[key] = true
	}
	return nil
}
func comparisonKey(i ComparisonItem) string {
	return strings.Join([]string{i.Area, i.Code, i.Aspect}, "/")
}
func SortedComparisonItems(c Comparison) []ComparisonItem {
	out := append([]ComparisonItem(nil), c.Items...)
	sort.Slice(out, func(i, j int) bool { return comparisonKey(out[i]) < comparisonKey(out[j]) })
	return out
}
