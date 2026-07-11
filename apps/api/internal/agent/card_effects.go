package agent

import (
	"fmt"

	"mindimprint/api/internal/cards"
)

// MintNode is a graph node a graph_effect proposes to create. It has no id —
// the caller (the loop, Task 5) assigns the real id on insert; MintEdge.ToID
// carries a deterministic placeholder ("$new:<index into the returned node
// slice>") the caller resolves to that id.
type MintNode struct {
	Type   string
	Author string
	Body   map[string]any
}

// MintEdge is a graph edge a graph_effect proposes to create, addressed by
// (kind, id) endpoints mirroring GraphEdgeView.
type MintEdge struct {
	Type     string
	FromKind string
	FromID   string
	ToKind   string
	ToID     string
}

// GraphEffects interprets spec.GraphEffects (a closed set of typed effects —
// agent-spec §3) over a completed card_instance's anchors and returns the
// nodes/edges to mint. Pure, no DB — the caller inserts and resolves the
// MintEdge placeholder ids. A spec with no (or no recognized) graph_effects
// yields no nodes/edges.
func GraphEffects(spec cards.Spec, materialID string, anchors []Anchor) ([]MintNode, []MintEdge) {
	var nodes []MintNode
	var edges []MintEdge
	for _, effect := range spec.GraphEffects {
		if effect.Kind != "promote" {
			continue
		}
		node := MintNode{
			Type:   effect.To,
			Author: "student",
			Body: map[string]any{
				effect.With: sourceQuality(spec, anchors),
			},
		}
		nodes = append(nodes, node)
		edges = append(edges, MintEdge{
			Type:     "evaluated-as",
			FromKind: effect.From,
			FromID:   materialID,
			ToKind:   "graph_node",
			ToID:     fmt.Sprintf("$new:%d", len(nodes)-1),
		})
	}
	return nodes, edges
}

// sourceQuality summarizes the card's per-dimension answers (the CRAAP
// verdict for each params.tags dimension) into the evidence node's body.
func sourceQuality(spec cards.Spec, anchors []Anchor) map[string]string {
	out := make(map[string]string, len(spec.Params.Tags))
	for _, tag := range spec.Params.Tags {
		for _, a := range anchors {
			if a.Dimension == tag && a.Answer != "" {
				out[tag] = a.Answer
				break
			}
		}
	}
	return out
}

// ConsolidationPayload builds the framework (methodology) map revealed to
// the student at consolidation (R-9: only after completion — the caller
// decides when to call this, not this pure function). A spec with no
// consolidation strategy yields an empty payload.
func ConsolidationPayload(spec cards.Spec) map[string]any {
	if spec.Consolidation == "" {
		return map[string]any{}
	}
	return map[string]any{
		"strategy":    spec.Consolidation,
		"framework":   spec.Name,
		"dimensions":  spec.Params.Tags,
		"tag_prompts": spec.Params.TagPrompts,
	}
}
