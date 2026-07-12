package studio

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"mindimprint/api/internal/store/sqlc"
)

// Load assembles ProjectData from the DB via the existing project-scoped queries.
func Load(ctx context.Context, q *sqlc.Queries, projectID uuid.UUID) (ProjectData, error) {
	pg := pgtype.UUID{Bytes: projectID, Valid: true}
	project, err := q.GetProject(ctx, projectID)
	if err != nil {
		return ProjectData{}, err
	}
	nodes, err := q.ListGraphNodesByProject(ctx, projectID)
	if err != nil {
		return ProjectData{}, err
	}
	edges, err := q.ListGraphEdgesByProject(ctx, projectID)
	if err != nil {
		return ProjectData{}, err
	}
	gateStates, err := q.ListGateStateNodes(ctx, projectID)
	if err != nil {
		return ProjectData{}, err
	}
	interventions, err := q.ListInterventionsByProject(ctx, projectID)
	if err != nil {
		return ProjectData{}, err
	}
	cardRows, err := q.ListCardInstancesByProject(ctx, pg)
	if err != nil {
		return ProjectData{}, err
	}
	d := ProjectData{
		Project: project, Nodes: nodes, Edges: edges, GateStates: gateStates,
		Interventions: interventions, Cards: cardRows,
	}
	plan, err := q.GetPlanNode(ctx, projectID)
	switch {
	case err == nil:
		d.Plan = &plan
	case errors.Is(err, pgx.ErrNoRows):
		// no plan yet — leave nil.
	default:
		return ProjectData{}, err
	}
	return d, nil
}
