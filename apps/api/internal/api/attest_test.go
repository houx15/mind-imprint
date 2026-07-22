package api

import (
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"mindimprint/api/internal/store/sqlc"
)

// If a helper of this name already exists in the api package's tests, delete
// this one and use the existing one — do not define a second.
//
// Kept for fields that really are pgtype.UUID (e.g. Material.ProjectID,
// CardInstance.ProjectID). The generated Material.ID / GraphNode.ID /
// GraphEdge.FromID+ToID / CardInstance.ID are plain uuid.UUID — verified
// against internal/store/sqlc/models.go — so the fixtures below assign those
// directly instead of routing them through pgID.
func pgID(u uuid.UUID) pgtype.UUID { return pgtype.UUID{Bytes: u, Valid: true} }

func TestAllArticlesHaveRiskNote(t *testing.T) {
	matA, matB := uuid.New(), uuid.New()
	nodeA, nodeB := uuid.New(), uuid.New()

	article := func(id uuid.UUID) sqlc.Material { return sqlc.Material{ID: id, Kind: "article"} }
	evidence := func(id uuid.UUID, body string) sqlc.GraphNode {
		return sqlc.GraphNode{ID: id, Type: "evidence", Body: []byte(body)}
	}
	evaluatedAs := func(m, n uuid.UUID) sqlc.GraphEdge {
		return sqlc.GraphEdge{Type: "evaluated-as", FromKind: "material", FromID: m, ToKind: "graph_node", ToID: n}
	}
	withNote := `{"source_quality":{"risk_note":"入口来源，不能直接引用。"}}`
	blankNote := `{"source_quality":{"risk_note":"   "}}`

	tests := []struct {
		name      string
		materials []sqlc.Material
		nodes     []sqlc.GraphNode
		edges     []sqlc.GraphEdge
		want      bool
	}{
		{
			name:      "every article has a risk note",
			materials: []sqlc.Material{article(matA), article(matB)},
			nodes:     []sqlc.GraphNode{evidence(nodeA, withNote), evidence(nodeB, withNote)},
			edges:     []sqlc.GraphEdge{evaluatedAs(matA, nodeA), evaluatedAs(matB, nodeB)},
			want:      true,
		},
		{
			name:      "one article still unevaluated",
			materials: []sqlc.Material{article(matA), article(matB)},
			nodes:     []sqlc.GraphNode{evidence(nodeA, withNote)},
			edges:     []sqlc.GraphEdge{evaluatedAs(matA, nodeA)},
			want:      false,
		},
		{
			name:      "whitespace-only risk note does not count",
			materials: []sqlc.Material{article(matA)},
			nodes:     []sqlc.GraphNode{evidence(nodeA, blankNote)},
			edges:     []sqlc.GraphEdge{evaluatedAs(matA, nodeA)},
			want:      false,
		},
		{
			// Unlike every_source_evaluated (gate.go:54), which passes
			// vacuously by design, an empty dossier has NOT done the S3 work.
			name:      "no article materials at all is not satisfied",
			materials: []sqlc.Material{{ID: matA, Kind: "draft"}},
			want:      false,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := allArticlesHaveRiskNote(tc.materials, tc.nodes, tc.edges); got != tc.want {
				t.Errorf("allArticlesHaveRiskNote = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestSlotHasText(t *testing.T) {
	nodes := []sqlc.GraphNode{
		{ID: uuid.New(), Type: "warrant", Body: []byte(`{"text":"遥感叶面积上升不等于生态质量上升，中间这一步需要说明。"}`)},
		{ID: uuid.New(), Type: "counter", Body: []byte(`{"text":"   "}`)},
	}
	if !slotHasText(nodes, "warrant") {
		t.Error("warrant slot with text should count as written")
	}
	if slotHasText(nodes, "counter") {
		t.Error("whitespace-only counter slot must not count as written")
	}
	if slotHasText(nodes, "concession") {
		t.Error("absent slot must not count as written")
	}
}

func TestSteelmanCardCounts(t *testing.T) {
	completed := []sqlc.CardInstance{{CardID: "steelman", Status: "completed"}}
	skipped := []sqlc.CardInstance{{CardID: "steelman", Status: "skipped"}}
	if !hasCompletedSteelmanCard(completed) {
		t.Error("a completed steelman card is a producer for the steelman item")
	}
	if hasCompletedSteelmanCard(skipped) {
		t.Error("a skipped steelman card must not satisfy the steelman item")
	}
}
