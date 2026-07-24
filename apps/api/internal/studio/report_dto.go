package studio

import (
	"time"

	"mindimprint/api/internal/agent"
)

// ReportDTO is the canonical dual-axis growth report over the wire. Mirrors
// packages/contracts/src/dualAxisReport.ts (camelCase) byte-for-byte: every
// agent.Report field, plus generatedAt (added at read time, never persisted).
// RL-5: no cross-axis aggregate field anywhere except
// OfficialProjection.Readiness.Score.
type ReportDTO struct {
	DepthAxis           []agent.DepthDim       `json:"depthAxis"`
	AutonomyAxis        []agent.AutonomySignal `json:"autonomyAxis"`
	PromptLens          agent.PromptLens       `json:"promptLens"`
	InteractionEvidence []agent.InteractionRow `json:"interactionEvidence"`
	Narrative           string                 `json:"narrative"`
	Guidance            agent.Guidance         `json:"guidance"`
	Axiom               string                 `json:"axiom"`

	OfficialProjection *agent.OfficialProjection `json:"officialProjection,omitempty"`
	WorkAndProcess     *agent.WorkAndProcess     `json:"workAndProcess,omitempty"`

	GeneratedAt string `json:"generatedAt"`
}

// ToReportDTO adds the generation timestamp to a Report. Pure.
func ToReportDTO(r agent.Report, createdAt time.Time) ReportDTO {
	r.AnchoredNilGuards() // ensure no null arrays over the wire
	return ReportDTO{
		DepthAxis:           r.DepthAxis,
		AutonomyAxis:        r.AutonomyAxis,
		PromptLens:          r.PromptLens,
		InteractionEvidence: r.InteractionEvidence,
		Narrative:           r.Narrative,
		Guidance:            r.Guidance,
		Axiom:               r.Axiom,
		OfficialProjection:  r.OfficialProjection,
		WorkAndProcess:      r.WorkAndProcess,
		GeneratedAt:         createdAt.Format(time.RFC3339),
	}
}
