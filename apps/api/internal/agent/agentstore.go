package agent

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"mindimprint/api/internal/store/sqlc"
)

// sqlcAgentStore adapts sqlc queries to the AgentStore seam (loop.go): graph
// reads feed LoadGraph, and InsertIntervention/AppendEvent record the
// coach's emitted action.
type sqlcAgentStore struct{ q *sqlc.Queries }

// NewSqlcAgentStore adapts sqlc queries to the AgentStore seam.
func NewSqlcAgentStore(q *sqlc.Queries) *sqlcAgentStore { return &sqlcAgentStore{q: q} }

// LoadGraph reads the project's graph_node/graph_edge rows into the shallow
// GraphView the classifier/coach read (design §9 open question 1: the
// changed node + its direct edges + the project's claims — Slice 2 loads
// the whole project graph, which is small at this scale).
func (s *sqlcAgentStore) LoadGraph(ctx context.Context, projectID uuid.UUID) (GraphView, error) {
	nodes, err := s.q.ListGraphNodesByProject(ctx, projectID)
	if err != nil {
		return GraphView{}, err
	}
	edges, err := s.q.ListGraphEdgesByProject(ctx, projectID)
	if err != nil {
		return GraphView{}, err
	}

	g := GraphView{
		Nodes: make([]GraphNodeView, 0, len(nodes)),
		Edges: make([]GraphEdgeView, 0, len(edges)),
	}
	for _, n := range nodes {
		g.Nodes = append(g.Nodes, GraphNodeView{
			ID:     n.ID.String(),
			Type:   n.Type,
			Author: n.Author,
			Text:   graphNodeBodyText(n.Body),
		})
	}
	for _, e := range edges {
		g.Edges = append(g.Edges, GraphEdgeView{
			FromKind: e.FromKind,
			FromID:   e.FromID.String(),
			ToKind:   e.ToKind,
			ToID:     e.ToID.String(),
			Type:     e.Type,
		})
	}
	return g, nil
}

// graphNodeBodyText reads the "text" field out of a graph_node.body jsonb
// blob — the only body field the runtime reads (the coach's
// declarative-echo comparison topic; the classifier's pure predicates never
// read Text). A malformed or absent field yields "", not an error.
func graphNodeBodyText(body []byte) string {
	var parsed struct {
		Text string `json:"text"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return ""
	}
	return parsed.Text
}

// InsertIntervention persists one coach output to the `intervention` table.
func (s *sqlcAgentStore) InsertIntervention(ctx context.Context, row InterventionRow) (uuid.UUID, error) {
	var cardInstanceID pgtype.UUID
	if row.CardInstanceID != nil {
		cardInstanceID = pgtype.UUID{Bytes: *row.CardInstanceID, Valid: true}
	}
	params := sqlc.InsertInterventionParams{
		ProjectID:      row.ProjectID,
		CardInstanceID: cardInstanceID,
		Type:           row.Type,
		Anchor:         row.Anchor,
		Body:           row.Body,
	}
	if row.Criterion != "" {
		params.Criterion = &row.Criterion
	}
	if row.Level != "" {
		params.Level = &row.Level
	}
	if row.OutputCheckVerdict != "" {
		params.OutputCheckVerdict = &row.OutputCheckVerdict
	}
	ivn, err := s.q.InsertIntervention(ctx, params)
	if err != nil {
		return uuid.UUID{}, err
	}
	return ivn.ID, nil
}

// AppendEvent writes the C4 event, attributed to the project's owning user —
// the AgentStore seam carries no separate "acting user" concept in Slice 2's
// single-user-per-project model (migration 0016).
func (s *sqlcAgentStore) AppendEvent(ctx context.Context, row EventRow) error {
	project, err := s.q.GetProject(ctx, row.ProjectID)
	if err != nil {
		return err
	}
	_, err = s.q.AppendEvent(ctx, sqlc.AppendEventParams{
		ProjectID: pgtype.UUID{Bytes: row.ProjectID, Valid: true},
		UserID:    project.UserID,
		Surface:   row.Surface,
		Type:      row.Type,
		Payload:   row.Payload,
	})
	return err
}
