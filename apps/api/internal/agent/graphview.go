package agent

import (
	"encoding/json"

	"mindimprint/api/internal/store/sqlc"
)

// GraphViewFromRows builds the shallow GraphView from already-loaded rows. Pure;
// shared by the DB-bound LoadGraph and the read-projection (internal/studio).
func GraphViewFromRows(nodes []sqlc.GraphNode, edges []sqlc.GraphEdge, materials []sqlc.Material, cards []sqlc.CardInstance) GraphView {
	g := GraphView{
		Nodes:         make([]GraphNodeView, 0, len(nodes)),
		Edges:         make([]GraphEdgeView, 0, len(edges)),
		Materials:     make([]MaterialView, 0, len(materials)),
		CardInstances: make([]CardInstanceView, 0, len(cards)),
	}
	for _, n := range nodes {
		g.Nodes = append(g.Nodes, GraphNodeView{ID: n.ID.String(), Type: n.Type, Author: n.Author, Text: graphNodeBodyText(n.Body)})
	}
	for _, e := range edges {
		g.Edges = append(g.Edges, GraphEdgeView{FromKind: e.FromKind, FromID: e.FromID.String(), ToKind: e.ToKind, ToID: e.ToID.String(), Type: e.Type})
	}
	for _, m := range materials {
		g.Materials = append(g.Materials, MaterialView{ID: m.ID.String(), Kind: m.Kind})
	}
	for _, ci := range cards {
		var anchors []Anchor
		if len(ci.Anchors) > 0 {
			_ = json.Unmarshal(ci.Anchors, &anchors)
		}
		g.CardInstances = append(g.CardInstances, CardInstanceView{ID: ci.ID.String(), CardID: ci.CardID, Status: ci.Status, Anchors: anchors})
	}
	return g
}

// RecordedGatesFromNodes reads gate_state graph_nodes into the recorded-gate map,
// keyed by contract id. A malformed body is treated as no recorded state.
func RecordedGatesFromNodes(gateStateNodes []sqlc.GraphNode) map[string]RecordedGate {
	out := make(map[string]RecordedGate, len(gateStateNodes))
	for _, r := range gateStateNodes {
		var b gateStateBody
		if err := json.Unmarshal(r.Body, &b); err != nil {
			continue
		}
		out[b.Contract] = RecordedGate{Confirmed: b.ConfirmedSolid, Items: b.Items}
	}
	return out
}
