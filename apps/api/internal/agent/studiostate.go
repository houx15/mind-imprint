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
