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

// MaterialView is the classifier's read-only view of one of the project's
// source materials (design §3: surface_card targets a "material.source").
// Kind mirrors the material table's check constraint ("article" | "draft");
// the surface-card predicate only ever proposes on "article" (source)
// materials, never on the student's own draft.
type MaterialView struct {
	ID   string
	Kind string
}

// CardInstanceView is the classifier/loop's read-only view of one of the
// project's card_instance rows, carrying just enough state (id, card id,
// status, live anchors) for ObserveCandidates (Task 2) to evaluate the
// card's observe rules over its primitive state.
type CardInstanceView struct {
	ID      string
	CardID  string
	Status  string
	Anchors []Anchor
}

// GraphView is the shallow graph neighborhood the classifier evaluates and
// the coach reads context from. Perceive (loop.go, Task 4) loads it via
// sqlc; Slice 2's tests build it directly as a pure fixture. Materials and
// CardInstances are Task 5 additions (Slice 3): the surface_card predicate
// reads Materials + Edges (a card_instance->material edge marks a material
// already surfaced; a material->evidence "evaluated-as" edge marks it
// already promoted); the observe predicate reads CardInstances.
type GraphView struct {
	Nodes         []GraphNodeView
	Edges         []GraphEdgeView
	Materials     []MaterialView
	CardInstances []CardInstanceView
}

// Candidate is one classifier-proposed next action awaiting the coach. The
// anchor + criterion travel with the candidate from the classifier — the
// coach only fills in the body text; it is never asked for structured
// output (design §4).
type Candidate struct {
	Verb       string // "post_intervention" | "surface_card" (Task 5); other C3 verbs later
	AnchorKind string // e.g. "graph_node" | "material"
	AnchorID   string
	Criterion  string // CT dimension tag, e.g. "D5"
	Reason     string // internal-only: why this candidate fired
	Level      string // I-ladder rung, e.g. "I2"
	CardID     string // set for "surface_card": the card id to instantiate (e.g. "craap")
}

// Trigger mirrors the agent-spec tiers that can invoke RunAgentStep: T-A
// (summon), T-B (structural), T-C (fine-grained — context-only in Slice 2).
type Trigger struct {
	Kind string

	// StudentText is the message that provoked this step, carried on the
	// trigger rather than re-read from history: RunAgentStep loads chat
	// history only AFTER the surface-card branch, and the semantic pre-gate
	// runs before it. Set for Kind == "student_turn"; empty elsewhere, which
	// disables the semantic classifier by construction (N3b).
	StudentText string

	// CardInstanceID names the card_instance whose completion provoked this
	// step. Set for Kind == "card_refeed"; empty elsewhere, which disables the
	// refeed branch by construction (N3b Seam B).
	CardInstanceID string
}

// Action is the loop's single emitted step for one RunAgentStep call
// (Task 4). Silence is a nil *Action, never a zero-value Action. Kind
// discriminates the shapes later tasks add: "intervention" (the Slice-2
// shape — Output/InterventionID/Verdict populated), "surface_card" (Task
// 5 — CardInstanceID populated, no model call, no enforcement), and
// "check_gate" (Task 10 — GateReport populated, no model call, no
// enforcement: a structural read of one contract's gate).
type Action struct {
	Kind           string // "intervention" | "surface_card" | "check_gate"
	Output         enforcement.AgentOutput
	InterventionID string
	Verdict        string
	CardInstanceID string      // set when Kind == "surface_card"
	CardID         string      // set when Kind == "surface_card": the card spec id (e.g. "craap")
	MaterialID     string      // set when Kind == "surface_card": the material this card is ABOUT (the card_instance--evaluates-->material edge target) — carried so callers (the SSE frame, the studio projection) never have to guess it from anchors or array position (whole-branch review finding [5]).
	GateReport     *GateReport // set when Kind == "check_gate"
}
