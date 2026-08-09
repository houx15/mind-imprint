package agent

// StudioStage is the AI-managed lifecycle position of a project. Tunable — the
// orchestrator may move a project forward or back; there is no rigid ordering
// constraint here (the plan-generation gate lives in the plan/generate handler,
// not in this enum).
type StudioStage string

const (
	StageTopicDiscussion StudioStage = "topic_discussion"
	StageProposalForming StudioStage = "proposal_forming"
	StagePlanGeneration  StudioStage = "plan_generation"
	StageProposalWriting StudioStage = "proposal_writing"
	StageProposalReview  StudioStage = "proposal_review"
	StageBodyWriting     StudioStage = "body_writing"
	StageRetrospective   StudioStage = "retrospective"
)

func (s StudioStage) IsValid() bool {
	switch s {
	case StageTopicDiscussion, StageProposalForming, StagePlanGeneration,
		StageProposalWriting, StageProposalReview, StageBodyWriting, StageRetrospective:
		return true
	}
	return false
}

// OpenTool is which room the shell shows. `chat` = no room (chat-only landing).
type OpenTool string

const (
	ToolChat       OpenTool = "chat"
	ToolForming    OpenTool = "forming"
	ToolPlan       OpenTool = "plan"
	ToolReading    OpenTool = "reading"
	ToolWriting    OpenTool = "writing"
	ToolReflection OpenTool = "reflection"
)

func (t OpenTool) IsValid() bool {
	switch t {
	case ToolChat, ToolForming, ToolPlan, ToolReading, ToolWriting, ToolReflection:
		return true
	}
	return false
}

// WidthTier is stored in P1 and rendered in P2. Derived from OpenTool, never a
// tool argument.
type WidthTier string

const (
	WidthChat WidthTier = "chat"
	WidthHalf WidthTier = "half"
	WidthWide WidthTier = "wide"
)

func WidthForTool(t OpenTool) WidthTier {
	switch t {
	case ToolChat:
		return WidthChat
	case ToolForming:
		return WidthHalf
	default: // plan, reading, writing, reflection
		return WidthWide
	}
}

// ReferenceRef is one item 印记 curated into the left reference panel. Lean by
// design in P1 — P3 renders it richly. kind ∈ material|note|annotation.
type ReferenceRef struct {
	Kind  string `json:"kind"`
	ID    string `json:"id"`
	Label string `json:"label"`
}

// StudioState is the AI-managed workspace state persisted on the project.
type StudioState struct {
	Stage         StudioStage    `json:"stage"`
	OpenTool      OpenTool       `json:"openTool"`
	WidthTier     WidthTier      `json:"widthTier"`
	Reference     []ReferenceRef `json:"reference"`
	UpdatedAtTurn int            `json:"updatedAtTurn"`
	Started       bool           `json:"started"`
	// PendingFrameworkVerdict (slice 2) holds the framework-readiness reviewer's
	// verdict from the moment the plan was auto-generated, until the next coach
	// turn surfaces it (reply.reviewVerdict) and clears it. Server-only + jsonb
	// (no migration); the frontend's Zod StudioState ignores this unknown key.
	// Decouples "when the review runs" (note-confirm OR a coach turn) from "when
	// it's shown" (the next coach turn).
	PendingFrameworkVerdict *FrameworkVerdict `json:"pendingFrameworkVerdict,omitempty"`
	// ProposalTrack (slice 3a) is the proposal document's guide-step track
	// (free/guided mode + dynamic per-sub-question steps). Pointer so absence is
	// distinguishable from a zero track; the essay track is slice 4.
	ProposalTrack *WritingTrack `json:"proposalTrack,omitempty"`
	// CounterpointsWaived (slice 3a, Finding 2) — the student chose to generate
	// the plan without articulating a 反例. Lets frameworkReadyForPlan proceed
	// while keeping the 反例 prompt non-blocking (铁律②). jsonb, no migration.
	CounterpointsWaived bool `json:"counterpointsWaived,omitempty"`
	// EssayTrack (slice 4) — the essay's three-stage position (research →
	// statement → submission, §6). 4a lands "research"; nil until 完成提案 enters
	// the essay. jsonb, no migration.
	EssayTrack *EssayTrack `json:"essayTrack,omitempty"`
}

// EssayStage is the essay's position in §6's three stages.
type EssayStage string

const (
	EssayResearch   EssayStage = "research"
	EssayStatement  EssayStage = "statement"
	EssaySubmission EssayStage = "submission"
)

// EssayTrack carries the essay's stage + (statement stage) the guide-step walk.
// The research stage's evidence map lives in the warren graph; the statement
// walk (outline → per-claim → synthesis → challenges → conclusion → structure)
// is driven by StatementStep over DeriveStatementSteps.
type EssayTrack struct {
	Stage EssayStage `json:"stage"`
	// slice 4b · the statement stage's walk. Started = the student passed the
	// outline-intro ready gate. StepGuides caches generated guide cards by key.
	Started       bool              `json:"statementStarted,omitempty"`
	StatementStep int               `json:"statementStep,omitempty"`
	StepGuides    map[string]string `json:"essayStepGuides,omitempty"`
}

func DefaultStudioState() StudioState {
	return StudioState{
		Stage:         StageTopicDiscussion,
		OpenTool:      ToolChat,
		WidthTier:     WidthChat,
		Reference:     []ReferenceRef{},
		UpdatedAtTurn: 0,
		Started:       false,
	}
}
