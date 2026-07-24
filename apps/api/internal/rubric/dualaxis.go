package rubric

import (
	_ "embed"
	"encoding/json"
)

//go:embed dualaxis.json
var dualaxisJSON []byte

// DepthDim is one of the six depth-axis dimensions (D1-D6). Anchors keys are
// "L1".."L4". D6 additionally carries ReflectionRule (student-authored-only
// reflection rule; NA rather than a low score when absent).
type DepthDim struct {
	ID             string            `json:"id"`
	Name           string            `json:"name"`
	Means          string            `json:"means"`
	Anchors        map[string]string `json:"anchors"`
	ReflectionRule string            `json:"reflectionRule,omitempty"`
}

// AutonomySignal is one of the six autonomy-axis signals (A1-A6): a
// behavior-count signal, not a quality score.
type AutonomySignal struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Means string `json:"means"`
	Event string `json:"event"`
}

// Lens is one of the six prompt-lens process indicators. Lenses read AI
// interaction traces only; they are never a third scoring axis.
type Lens struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Guide string `json:"guide"`
}

// OfficialComponentSpec is one scoring component of an OfficialStandard
// (e.g. AP Research's Academic Paper / POD / 训练用折算 / 诚信). The JSON key
// for Kou is the literal Chinese "口径" (calibration note).
type OfficialComponentSpec struct {
	Name  string `json:"name"`
	Scale string `json:"scale"`
	Kou   string `json:"口径"`
}

// OfficialStandard is an external assessment standard (e.g. AP Research)
// that the canonical dual-axis model aligns to for reference, without being
// combined into the dual-axis score.
type OfficialStandard struct {
	ID             string                  `json:"id"`
	Name           string                  `json:"name"`
	Components     []OfficialComponentSpec `json:"components"`
	AlignmentItems []string                `json:"alignmentItems"`
}

// DualAxis is the canonical config: two axes (depth, autonomy) that never
// combine into a single score, plus prompt lenses (process evidence, not a
// third axis) and official-standard alignment references.
type DualAxis struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Axiom string `json:"axiom"`

	Depth    []DepthDim       `json:"depth"`
	Autonomy []AutonomySignal `json:"autonomy"`

	AutonomyBand    string `json:"autonomyBand"`
	OpportunityRule string `json:"opportunityRule"`

	Lenses   []Lens `json:"lenses"`
	LensNote string `json:"lensNote"`

	Standards []OfficialStandard `json:"standards"`
}

var dualaxis = mustParseDual()

func mustParseDual() DualAxis {
	var m DualAxis
	if err := json.Unmarshal(dualaxisJSON, &m); err != nil {
		panic("rubric: bad embedded dualaxis.json: " + err.Error())
	}
	return m
}

// Model returns the parsed canonical DualAxis model.
func Model() DualAxis { return dualaxis }

// DepthDims returns the six depth-axis dimensions in config order.
func DepthDims() []DepthDim { return dualaxis.Depth }

// AutonomySignals returns the six autonomy-axis signals in config order.
func AutonomySignals() []AutonomySignal { return dualaxis.Autonomy }

// Lenses returns the six prompt-lens process indicators in config order.
func Lenses() []Lens { return dualaxis.Lenses }

// AutonomyBand returns the shared behavior-count band guide text for the
// autonomy axis (levels 0-5).
func AutonomyBand() string { return dualaxis.AutonomyBand }

// Standard looks up an official external standard by id (e.g. "ap-research").
func Standard(id string) (OfficialStandard, bool) {
	for _, s := range dualaxis.Standards {
		if s.ID == id {
			return s, true
		}
	}
	return OfficialStandard{}, false
}
