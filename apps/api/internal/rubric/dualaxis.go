package rubric

import (
	_ "embed"
	"encoding/json"
)

//go:embed dualaxis.json
var dualaxisJSON []byte

type Axis struct {
	Name    string `json:"name"`
	Scoring string `json:"scoring"` // score | observation | descriptive
	Max     int    `json:"max"`
}

type AxisDim struct {
	ID               string            `json:"id"`
	Axis             string            `json:"axis"` // depth | autonomy | cross
	Name             string            `json:"name"`
	Anchors          map[string]string `json:"anchors"` // depth only: keys "0".."3"
	ObservationGuide string            `json:"observationGuide"`
	Guide            string            `json:"guide"`
}

type PromptTier struct {
	Tier  string `json:"tier"`
	Label string `json:"label"`
}

type SoloLevel struct {
	Level string `json:"level"`
	Name  string `json:"name"`
}

type DualAxis struct {
	ID          string          `json:"id"`
	Name        string          `json:"name"`
	Axiom       string          `json:"axiom"`
	Axes        map[string]Axis `json:"axes"`
	Dimensions  []AxisDim       `json:"dimensions"`
	PromptTiers []PromptTier    `json:"promptTiers"`
	SoloLevels  []SoloLevel     `json:"soloLevels"`
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

func dimsByAxis(axis string) []AxisDim {
	out := make([]AxisDim, 0, 4)
	for _, d := range dualaxis.Dimensions {
		if d.Axis == axis {
			out = append(out, d)
		}
	}
	return out
}

// DepthDims returns the four scored depth-axis dimensions in config order.
func DepthDims() []AxisDim { return dimsByAxis("depth") }

// AutonomyDim returns the single autonomy-axis dimension.
func AutonomyDim() AxisDim {
	d := dimsByAxis("autonomy")
	if len(d) == 0 {
		return AxisDim{}
	}
	return d[0]
}

// CrossDim returns the single cross-axis dimension.
func CrossDim() AxisDim {
	d := dimsByAxis("cross")
	if len(d) == 0 {
		return AxisDim{}
	}
	return d[0]
}
