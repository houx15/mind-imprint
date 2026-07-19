package studio

import "mindimprint/api/internal/agent"

// ReportDTO is the DualAxis growth report over the wire. Mirrors
// packages/contracts/src/dualAxisReport.ts (camelCase) byte-for-byte, adding
// generatedAt. The only number is DepthAxis.Subtotal (RL-5 + axiom).
type ReportDTO struct {
	DepthAxis    agent.DepthAxis     `json:"depthAxis"`
	AutonomyAxis agent.AutonomyAxis  `json:"autonomyAxis"`
	CrossAxis    agent.CrossAxis     `json:"crossAxis"`
	Solo         []agent.SoloRow     `json:"solo"`
	PromptLens   agent.PromptLens    `json:"promptLens"`
	Timeline     []agent.TimelineRow `json:"timeline"`
	KeyEvidence  []agent.KeyEvidence `json:"keyEvidence"`
	Guidance     agent.Guidance      `json:"guidance"`
	Narrative    string              `json:"narrative"`
	Axiom        string              `json:"axiom"`
	GeneratedAt  string              `json:"generatedAt"`
}

// ToReportDTO adds the generation timestamp to a Report. Pure.
func ToReportDTO(r agent.Report, generatedAt string) ReportDTO {
	r.AnchoredNilGuards() // ensure no null arrays over the wire
	return ReportDTO{
		DepthAxis: r.DepthAxis, AutonomyAxis: r.AutonomyAxis, CrossAxis: r.CrossAxis,
		Solo: r.Solo, PromptLens: r.PromptLens, Timeline: r.Timeline, KeyEvidence: r.KeyEvidence,
		Guidance: r.Guidance, Narrative: r.Narrative, Axiom: r.Axiom, GeneratedAt: generatedAt,
	}
}
