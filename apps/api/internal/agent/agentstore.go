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
// the whole project graph, which is small at this scale). Task 5 adds
// Materials (the surface_card predicate's targets) and CardInstances (the
// observe predicate's targets) — both project-scoped, both small at this
// scale for the same reason.
func (s *sqlcAgentStore) LoadGraph(ctx context.Context, projectID uuid.UUID) (GraphView, error) {
	nodes, err := s.q.ListGraphNodesByProject(ctx, projectID)
	if err != nil {
		return GraphView{}, err
	}
	edges, err := s.q.ListGraphEdgesByProject(ctx, projectID)
	if err != nil {
		return GraphView{}, err
	}
	pgProjectID := pgtype.UUID{Bytes: projectID, Valid: true}
	materials, err := s.q.ListMaterialsByProject(ctx, pgProjectID)
	if err != nil {
		return GraphView{}, err
	}
	cardInstances, err := s.q.ListCardInstancesByProject(ctx, pgProjectID)
	if err != nil {
		return GraphView{}, err
	}

	g := GraphView{
		Nodes:         make([]GraphNodeView, 0, len(nodes)),
		Edges:         make([]GraphEdgeView, 0, len(edges)),
		Materials:     make([]MaterialView, 0, len(materials)),
		CardInstances: make([]CardInstanceView, 0, len(cardInstances)),
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
	for _, m := range materials {
		g.Materials = append(g.Materials, MaterialView{ID: m.ID.String(), Kind: m.Kind})
	}
	for _, ci := range cardInstances {
		var anchors []Anchor
		if len(ci.Anchors) > 0 {
			// A malformed anchors blob must never crash perceive — treat it
			// as no anchors (the observe rules simply see nothing to flag).
			_ = json.Unmarshal(ci.Anchors, &anchors)
		}
		g.CardInstances = append(g.CardInstances, CardInstanceView{
			ID:      ci.ID.String(),
			CardID:  ci.CardID,
			Status:  ci.Status,
			Anchors: anchors,
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

// CreateCardInstance instantiates a proposed card_instance for cardID on
// materialID. card_instances.task_id stays NOT NULL (a legacy FK not yet
// dropped, migration 0016's note); this adapter resolves it from
// materialID's own material row so the pure agent code (card_lifecycle.go)
// never has to know about tasks.
func (s *sqlcAgentStore) CreateCardInstance(ctx context.Context, projectID, materialID uuid.UUID, cardID, contractRef string) (CardInstanceRow, error) {
	mat, err := s.q.GetMaterial(ctx, materialID)
	if err != nil {
		return CardInstanceRow{}, err
	}
	row, err := s.q.CreateProjectCardInstance(ctx, sqlc.CreateProjectCardInstanceParams{
		TaskID:      mat.TaskID,
		ProjectID:   pgtype.UUID{Bytes: projectID, Valid: true},
		CardID:      cardID,
		ContractRef: &contractRef,
		Status:      "proposed",
	})
	if err != nil {
		return CardInstanceRow{}, err
	}
	return toCardInstanceRow(row), nil
}

// GetCardInstance reads one card_instance row.
func (s *sqlcAgentStore) GetCardInstance(ctx context.Context, id uuid.UUID) (CardInstanceRow, error) {
	row, err := s.q.GetCardInstance(ctx, id)
	if err != nil {
		return CardInstanceRow{}, err
	}
	return toCardInstanceRow(row), nil
}

// SetCardInstanceFramework writes the consolidation payload (R-9: revealed
// only after completion) to framework_fill.
func (s *sqlcAgentStore) SetCardInstanceFramework(ctx context.Context, projectID, id uuid.UUID, framework []byte) error {
	_, err := s.q.SetCardInstanceFramework(ctx, sqlc.SetCardInstanceFrameworkParams{
		ID:            id,
		ProjectID:     pgtype.UUID{Bytes: projectID, Valid: true},
		FrameworkFill: framework,
	})
	return err
}

// toCardInstanceRow maps the sqlc row to the AgentStore seam's shape.
func toCardInstanceRow(row sqlc.CardInstance) CardInstanceRow {
	var projectID uuid.UUID
	if row.ProjectID.Valid {
		projectID = row.ProjectID.Bytes
	}
	return CardInstanceRow{
		ID:            row.ID,
		ProjectID:     projectID,
		CardID:        row.CardID,
		Status:        row.Status,
		Anchors:       row.Anchors,
		FrameworkFill: row.FrameworkFill,
	}
}

// InsertGraphNode mints one graph_node from a card's graph_effects
// (card_effects.go's MintNode). node.Body is marshaled to jsonb; an empty
// map still marshals cleanly.
func (s *sqlcAgentStore) InsertGraphNode(ctx context.Context, projectID uuid.UUID, node MintNode) (uuid.UUID, error) {
	body, err := json.Marshal(node.Body)
	if err != nil {
		return uuid.UUID{}, err
	}
	row, err := s.q.InsertGraphNode(ctx, sqlc.InsertGraphNodeParams{
		ProjectID: projectID,
		Type:      node.Type,
		Body:      body,
		Author:    node.Author,
	})
	if err != nil {
		return uuid.UUID{}, err
	}
	return row.ID, nil
}

// InsertGraphEdge mints one graph_edge — either a card's graph_effects edge
// (card_effects.go's MintEdge, ids already resolved by the caller) or
// SurfaceCard's card_instance->material link.
func (s *sqlcAgentStore) InsertGraphEdge(ctx context.Context, projectID uuid.UUID, edge MintEdge) error {
	fromID, err := uuid.Parse(edge.FromID)
	if err != nil {
		return err
	}
	toID, err := uuid.Parse(edge.ToID)
	if err != nil {
		return err
	}
	_, err = s.q.InsertGraphEdge(ctx, sqlc.InsertGraphEdgeParams{
		ProjectID: projectID,
		Type:      edge.Type,
		FromKind:  edge.FromKind,
		FromID:    fromID,
		ToKind:    edge.ToKind,
		ToID:      toID,
	})
	return err
}

// InsertDisposition persists one three-key disposition (product spec
// §7.1 rule 6). Reason-length enforcement lives in the caller
// (agent.RecordDisposition, card_lifecycle.go).
func (s *sqlcAgentStore) InsertDisposition(ctx context.Context, interventionID uuid.UUID, action, reason string) (uuid.UUID, error) {
	row, err := s.q.InsertDisposition(ctx, sqlc.InsertDispositionParams{
		InterventionID: interventionID,
		Action:         action,
		Reason:         reason,
	})
	if err != nil {
		return uuid.UUID{}, err
	}
	return row.ID, nil
}
