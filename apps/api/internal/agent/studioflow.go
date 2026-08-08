package agent

// studioflow.go — the status-router registry (spec:
// docs/superpowers/specs/2026-08-09-status-router-studio-redesign.md). It
// COLLAPSES the seven persisted StudioStage values into five canonical
// FlowStatuses, each owning a short prompt + a small tool set + a surface. No
// data migration: StudioState.Stage keeps storing the existing values;
// StatusForStage projects them onto the router's coarse-grained view, and
// StageFloor maps a status back to the canonical stage the deterministic router
// persists.

// FlowStatus is the coarse-grained project position the router dispatches on.
type FlowStatus string

const (
	FlowTopic     FlowStatus = "topic"     // no research question yet
	FlowFramework FlowStatus = "framework" // the four 立项 points (+ plan auto-gen)
	FlowProposal  FlowStatus = "proposal"  // write the proposal document
	FlowEssay     FlowStatus = "essay"     // write the essay (outline/snippets/body)
	FlowReview    FlowStatus = "review"    // 复盘 / reflection
)

// StatusForStage collapses a persisted StudioStage onto its FlowStatus. Unknown
// or empty stages default to FlowFramework (the safe working default for a
// started project — never a dead end).
func StatusForStage(s StudioStage) FlowStatus {
	switch s {
	case StageTopicDiscussion:
		return FlowTopic
	case StageProposalForming, StagePlanGeneration:
		return FlowFramework
	case StageProposalWriting, StageProposalReview:
		return FlowProposal
	case StageBodyWriting:
		return FlowEssay
	case StageRetrospective:
		return FlowReview
	}
	return FlowFramework
}

// StageFloor is the canonical persisted stage a FlowStatus maps back to — the
// value the deterministic router writes into StudioState.Stage when it lands a
// project in that status. (The collapse is many→one; the floor is the earliest
// stage of each status so the router never jumps a project past sub-steps the
// model may still drive within a status.)
func (f FlowStatus) StageFloor() StudioStage {
	switch f {
	case FlowTopic:
		return StageTopicDiscussion
	case FlowFramework:
		return StageProposalForming
	case FlowProposal:
		return StageProposalWriting
	case FlowEssay:
		return StageBodyWriting
	case FlowReview:
		return StageRetrospective
	}
	return StageProposalForming
}
