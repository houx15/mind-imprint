// Package studio builds the read-path StudioProjection wire DTO (Slice 5b).
// The DTO's JSON shape mirrors packages/contracts/src/studioState.ts exactly.
package studio

type StudioProjection struct {
	Project       ProjectHeader `json:"project"`
	Stations      []StationDTO  `json:"stations"`
	ActiveStation string        `json:"activeStation"`
	Coach         CoachDTO      `json:"coach"`
	Onboarding    OnboardingDTO `json:"onboarding"`
}

type ProjectHeader struct {
	Title     string `json:"title"`
	QualLabel string `json:"qualLabel"`
}

type StationDTO struct {
	Code     string   `json:"code"`
	Name     string   `json:"name"`
	View     string   `json:"view"`
	State    string   `json:"state"`
	Gate     *GateDTO `json:"gate,omitempty"`
	Backflow bool     `json:"backflow,omitempty"`
}

type GateDTO struct {
	Total  int `json:"total"`
	Passed int `json:"passed"`
}

type CoachDTO struct {
	Anchor    string            `json:"anchor"`
	Messages  []CoachMessageDTO `json:"messages"`
	Equipment []EquipCardDTO    `json:"equipment"`
}

// CoachMessageDTO carries the union tag in `kind`. For kind="ai": Body + optional
// Tag/Anchor. For kind="flag": Label + Body. For kind="student": Body.
type CoachMessageDTO struct {
	Kind   string `json:"kind"`
	Body   string `json:"body,omitempty"`
	Tag    string `json:"tag,omitempty"`
	Anchor string `json:"anchor,omitempty"`
	Label  string `json:"label,omitempty"`
}

type EquipCardDTO struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Spont string `json:"spont"`
	Meth  string `json:"meth"`
}

type RubricRowDTO struct {
	Official string `json:"official"`
	Plain    string `json:"plain"`
	Weak     bool   `json:"weak"`
}

type OnboardingDTO struct {
	RestatePrompt string         `json:"restatePrompt"`
	RubricRows    []RubricRowDTO `json:"rubricRows"`
	PlanSteps     []string       `json:"planSteps"`
}
