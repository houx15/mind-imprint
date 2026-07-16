package studio

import (
	"context"
	"encoding/json"
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
	chats, err := q.ListChatMessagesByProject(ctx, pg)
	if err != nil {
		return ProjectData{}, err
	}
	materials, err := q.ListMaterialsByProject(ctx, pg)
	if err != nil {
		return ProjectData{}, err
	}
	sourceLog, err := q.ListSourceLogByProject(ctx, projectID)
	if err != nil {
		return ProjectData{}, err
	}
	d := ProjectData{
		Project: project, Nodes: nodes, Edges: edges, GateStates: gateStates,
		Interventions: interventions, Cards: cardRows, ChatMessages: chats,
		Materials: materials, SourceLog: sourceLog,
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

	// S5 写作: the silent edit buffer, the latest immutable snapshot, and the
	// dispositions recorded on any of this project's interventions (review
	// items included).
	buffer, err := q.GetEditBuffer(ctx, projectID)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return ProjectData{}, err
	}
	d.EditBuffer = buffer // "" when ErrNoRows

	latest, err := q.GetLatestSnapshot(ctx, projectID)
	switch {
	case err == nil:
		d.LatestSnapshot = &latest
	case errors.Is(err, pgx.ErrNoRows):
		// no snapshot committed yet — leave nil.
	default:
		return ProjectData{}, err
	}

	disps, err := q.ListDispositionsByProject(ctx, projectID)
	if err != nil {
		return ProjectData{}, err
	}
	d.Dispositions = disps

	// Append-only event stream (C4), for the assessment path — inert to the
	// studio views themselves (Project never reads d.Events).
	eventRows, err := q.ListEventsByProject(ctx, pg)
	if err != nil {
		return ProjectData{}, err
	}
	events := make([]Event, len(eventRows))
	for i, ev := range eventRows {
		events[i] = Event{
			Type:      ev.Type,
			Surface:   ev.Surface,
			Payload:   json.RawMessage(ev.Payload),
			CreatedAt: ev.CreatedAt,
		}
	}
	d.Events = events

	return d, nil
}
