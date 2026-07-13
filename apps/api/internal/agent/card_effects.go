package agent

import (
	"fmt"
	"strings"

	"mindimprint/api/internal/cards"
)

// MintNode is a graph node a graph_effect proposes to create. It has no id —
// the caller (the loop, Task 5) assigns the real id on insert; a MintEdge's
// FromID or ToID (either endpoint) can carry a deterministic placeholder
// ("$new:<index into the returned node slice>") the caller resolves to that
// id — CompleteCard resolves both endpoints (card_lifecycle.go), not just
// ToID, which is what cross_check relies on (its edge to the checked
// material puts the placeholder in FromID).
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
		switch effect.Kind {
		case "promote":
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

		case "cross_check":
			// SIFT's lateral read never promotes the source she went and
			// found — it has been evaluated by nobody yet. Promoting it
			// would let a source become citable without evaluation and
			// hollow out the every_source_evaluated gate (design §4.1).
			lat, ok := lateralAnchor(spec, anchors)
			if !ok {
				continue
			}
			nodes = append(nodes, MintNode{
				Type:   "cross_check",
				Author: "student",
				Body:   crossCheckBody(anchors),
			})
			ref := fmt.Sprintf("$new:%d", len(nodes)-1)
			edges = append(edges,
				// the source under review --was checked by--> this cross-check
				MintEdge{Type: "cross-checked-by", FromKind: "material", FromID: materialID, ToKind: "graph_node", ToID: ref},
				// ...which --cites--> the independent source she went and found
				MintEdge{Type: "cites", FromKind: "graph_node", FromID: ref, ToKind: "material", ToID: lat.MaterialID},
			)
		}
	}
	return nodes, edges
}

// crossCheckBody carries what the student produced by reading laterally: the
// relation SHE chose (印证/反驳/限定 — the AI never picks it), where she traced
// the claim to, her first reaction, and the pyramid tier she landed on after
// checking, plus her own revised judgment in her own words. tier_before is
// filled by the caller (Task 7) from the source log, which is where her
// ingestion-time tier lives — it is deliberately absent here.
func crossCheckBody(anchors []Anchor) map[string]any {
	body := map[string]any{}
	for _, dim := range []string{"stop", "investigate", "find", "relation", "trace_origin", "tier_after", "revised_judgment"} {
		for _, a := range anchors {
			if a.Dimension == dim && strings.TrimSpace(a.Answer) != "" {
				body[dim] = a.Answer
				break
			}
		}
	}
	return body
}

// sourceQuality summarizes the card's per-dimension answers (the CRAAP
// verdict for each params.tags dimension) into the evidence node's body,
// plus — under the "risk_note" key — the student's own written 作用与风险
// judgment. The risk_note anchor is special: unlike the tag dimensions
// (which are the AI's answers to AI-posed questions), it is required for
// completion (field_written_by, author=student) and is the student's own
// words, not a verdict on an AI question — the single most valuable thing
// on the card, and the source the dossier projection reads for its 作用与
// 风险 line. Keyed off the anchor's dimension (not the card id), so any
// future card carrying a student risk_note anchor behaves the same.
func sourceQuality(spec cards.Spec, anchors []Anchor) map[string]string {
	out := make(map[string]string, len(spec.Params.Tags)+1)
	for _, tag := range spec.Params.Tags {
		for _, a := range anchors {
			if a.Dimension == tag && a.Answer != "" {
				out[tag] = a.Answer
				break
			}
		}
	}
	for _, a := range anchors {
		if a.Dimension == "risk_note" && a.Answer != "" {
			out["risk_note"] = a.Answer
			break
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
