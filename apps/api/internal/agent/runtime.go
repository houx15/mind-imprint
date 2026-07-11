package agent

import "mindimprint/api/internal/agent/enforcement"

// GraphNodeView is the classifier/coach's read-only view of one graph node
// within a shallow neighborhood (design §9 open question 1: the changed
// node + its direct edges + the project's claims).
type GraphNodeView struct {
	ID     string
	Type   string // "claim" | "evidence" | ...
	Author string // "student" | "ai"
	// Text is the node's own content, when the neighborhood view carries it
	// (e.g. the coach's declarative-echo comparison topic). Empty in the
	// classifier's minimal predicate fixtures — the classifier never reads it.
	Text string
}

// GraphEdgeView is one edge in the shallow neighborhood, addressed by
// (kind, id) endpoints mirroring the graph_node/artifact polymorphism used
// by the store.
type GraphEdgeView struct {
	FromKind string
	FromID   string
	ToKind   string
	ToID     string
	Type     string // "supports" | ...
}

// GraphView is the shallow graph neighborhood the classifier evaluates and
// the coach reads context from. Perceive (loop.go, Task 4) loads it via
// sqlc; Slice 2's tests build it directly as a pure fixture.
type GraphView struct {
	Nodes []GraphNodeView
	Edges []GraphEdgeView
}

// Candidate is one classifier-proposed next action awaiting the coach. The
// anchor + criterion travel with the candidate from the classifier — the
// coach only fills in the body text; it is never asked for structured
// output (design §4).
type Candidate struct {
	Verb       string // "post_intervention" in Slice 2; other C3 verbs later
	AnchorKind string // e.g. "graph_node"
	AnchorID   string
	Criterion  string // CT dimension tag, e.g. "D5"
	Reason     string // internal-only: why this candidate fired
	Level      string // I-ladder rung, e.g. "I2"
}

// Trigger mirrors the agent-spec tiers that can invoke RunAgentStep: T-A
// (summon), T-B (structural), T-C (fine-grained — context-only in Slice 2).
type Trigger struct {
	Kind string
}

// Action is the loop's single emitted step for one RunAgentStep call
// (Task 4). Silence is a nil *Action, never a zero-value Action.
type Action struct {
	Output         enforcement.AgentOutput
	InterventionID string
	Verdict        string
}
