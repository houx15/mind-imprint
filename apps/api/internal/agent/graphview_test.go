package agent

import (
	"testing"

	"github.com/google/uuid"
	"mindimprint/api/internal/store/sqlc"
)

func TestGraphViewFromRows(t *testing.T) {
	id := uuid.New()
	nodes := []sqlc.GraphNode{{ID: id, Type: "claim", Author: "student", Body: []byte(`{"text":"中国治理决心"}`)}}
	edges := []sqlc.GraphEdge{{FromKind: "evidence", FromID: uuid.New(), ToKind: "claim", ToID: id, Type: "supports"}}
	g := GraphViewFromRows(nodes, edges, nil, nil)
	if len(g.Nodes) != 1 || g.Nodes[0].Type != "claim" || g.Nodes[0].Text != "中国治理决心" {
		t.Fatalf("node view = %+v", g.Nodes)
	}
	if len(g.Edges) != 1 || g.Edges[0].Type != "supports" {
		t.Fatalf("edge view = %+v", g.Edges)
	}
}

func TestRecordedGatesFromNodes(t *testing.T) {
	nodes := []sqlc.GraphNode{
		{Type: "gate_state", Body: []byte(`{"contract":"decode_task","confirmed_solid":true,"items":{"milestone_plan":"solid"}}`)},
		{Type: "gate_state", Body: []byte(`bad json`)},
	}
	rec := RecordedGatesFromNodes(nodes)
	got, ok := rec["decode_task"]
	if !ok || !got.Confirmed || got.Items["milestone_plan"] != "solid" {
		t.Fatalf("recorded = %+v", rec)
	}
	if len(rec) != 1 { // malformed row skipped
		t.Fatalf("want 1 recorded gate, got %d", len(rec))
	}
}
