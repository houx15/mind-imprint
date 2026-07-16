package studio

import "mindimprint/api/internal/agent"

// AssessmentDimensionDTO is one CT dimension's growth-report row: level +
// the behavioral evidence backing it (RL-5: diagnostic, never a grade).
type AssessmentDimensionDTO struct {
	Code     string `json:"code"`
	Name     string `json:"name"`
	Level    string `json:"level"`
	Evidence string `json:"evidence"`
}

// AssessmentDTO is the growth report's own wire surface — NOT part of
// StudioProjection (the assessor is isolated from the coach loop, spec
// Slice 10). GeneratedAt is RFC3339, set by the handler at persist time.
type AssessmentDTO struct {
	Dimensions  []AssessmentDimensionDTO `json:"dimensions"`
	Narrative   string                   `json:"narrative"`
	GeneratedAt string                   `json:"generatedAt"`
}

// ToAssessmentDTO maps the pure agent.Assessment engine output onto the wire
// DTO. Pure — no I/O.
func ToAssessmentDTO(a agent.Assessment, generatedAt string) AssessmentDTO {
	dims := make([]AssessmentDimensionDTO, 0, len(a.Dimensions))
	for _, d := range a.Dimensions {
		dims = append(dims, AssessmentDimensionDTO{Code: d.Code, Name: d.Name, Level: d.Level, Evidence: d.Evidence})
	}
	return AssessmentDTO{Dimensions: dims, Narrative: a.Narrative, GeneratedAt: generatedAt}
}
