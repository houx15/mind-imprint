package agent

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"strings"

	"mindimprint/api/internal/agent/enforcement"
	"mindimprint/api/internal/gateway"
	"mindimprint/api/internal/rubric"
)

//go:embed anchors.json
var anchorsJSON []byte

// DimensionScore is one CT dimension's level + its behavioral evidence (RL-5:
// diagnostic, never a grade).
type DimensionScore struct {
	Code     string `json:"code"`
	Name     string `json:"name"`
	Level    string `json:"level"` // L1..L4 | NA
	Evidence string `json:"evidence"`
}

// Assessment is the whole growth report: one score per rubric dimension + a
// growth narrative. No overall score, no rank (RL-5).
type Assessment struct {
	Dimensions []DimensionScore `json:"dimensions"`
	Narrative  string           `json:"narrative"`
}

// AnchorSample is one few-shot exemplar (backend-only prompt priming).
type AnchorSample struct {
	Name       string           `json:"name"`
	Digest     string           `json:"digest"`
	Dimensions []DimensionScore `json:"dimensions"`
	Narrative  string           `json:"narrative"`
}

// EmbeddedAnchors returns the backend-only few-shot fixture (prompt priming).
func EmbeddedAnchors() []AnchorSample {
	var a []AnchorSample
	_ = json.Unmarshal(anchorsJSON, &a) // fixture is authored + tested; ignore err in prod path
	return a
}

var validLevel = map[string]bool{"L1": true, "L2": true, "L3": true, "L4": true, "NA": true}

type assessWire struct {
	Dimensions []struct {
		Code     string `json:"code"`
		Level    string `json:"level"`
		Evidence string `json:"evidence"`
	} `json:"dimensions"`
	Narrative string `json:"narrative"`
}

// Assess makes ONE isolated flagship call scoring the CT rubric over the process
// digest, runs the full enforcement stack, and returns per-dimension scores + a
// growth narrative. Never in the coach loop; flagship, never downgraded. Pure
// engine over (provider, rubric, input, anchors) — no store, no graph, no loop.
func Assess(ctx context.Context, prov gateway.Provider, r gateway.Resolved, rb rubric.Rubric, in AssessmentInput, anchors []AnchorSample) (Assessment, gateway.ChatUsage, error) {
	res, err := gateway.Collect(ctx, prov, r, gateway.ChatRequest{
		Messages: []gateway.ChatMessage{
			{Role: gateway.RoleSystem, Content: assessSystemPrompt(rb, anchors)},
			{Role: gateway.RoleUser, Content: assessUserInput(in)},
		},
	})
	if err != nil {
		return Assessment{}, gateway.ChatUsage{}, err
	}
	usage := res.Usage

	var wire assessWire
	if err := json.Unmarshal([]byte(strings.TrimSpace(res.Text)), &wire); err != nil {
		return Assessment{}, usage, fmt.Errorf("agent: assessment output not JSON: %w", err)
	}

	// Enforcement: narrative + every evidence field. Any match rejects the whole report.
	fields := []string{wire.Narrative}
	for _, d := range wire.Dimensions {
		fields = append(fields, d.Evidence)
	}
	for _, f := range fields {
		if f == "" {
			continue
		}
		if rule := enforcement.BannedPhrasing(f); rule != nil {
			return Assessment{}, usage, fmt.Errorf("agent: assessment rejected by banned-phrasing rule %q", rule.Name)
		}
	}

	// Index the model's scores by code; emit every rubric dimension in order,
	// defaulting to NA (missing dim, or unknown level).
	got := map[string]struct{ level, evidence string }{}
	for _, d := range wire.Dimensions {
		lvl := d.Level
		if !validLevel[lvl] {
			lvl = "NA"
		}
		got[d.Code] = struct{ level, evidence string }{lvl, d.Evidence}
	}
	out := make([]DimensionScore, 0, len(rb.Dimensions))
	for _, dim := range rb.Dimensions {
		g, ok := got[dim.ID]
		if !ok {
			g = struct{ level, evidence string }{"NA", ""}
		}
		out = append(out, DimensionScore{Code: dim.ID, Name: dim.Name, Level: g.level, Evidence: g.evidence})
	}
	return Assessment{Dimensions: out, Narrative: wire.Narrative}, usage, nil
}
