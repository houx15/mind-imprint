package agent

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"sort"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"mindimprint/api/internal/gateway"
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
	return GraphViewFromRows(nodes, edges, materials, cardInstances), nil
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

// getOrCreateThread resolves the project's chat_thread, creating it
// (get-then-create) the first time a chat message is persisted for that
// project. Mirrors AppendEvent's project->user resolution: the thread's
// owning user comes from the project row, not a separate "acting user"
// concept (Slice 2/5c's single-user-per-project model).
func (s *sqlcAgentStore) getOrCreateThread(ctx context.Context, projectID uuid.UUID) (uuid.UUID, error) {
	pg := pgtype.UUID{Bytes: projectID, Valid: true}
	th, err := s.q.GetThreadByProject(ctx, pg)
	if err == nil {
		return th.ID, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return uuid.UUID{}, err
	}
	project, err := s.q.GetProject(ctx, projectID)
	if err != nil {
		return uuid.UUID{}, err
	}
	created, err := s.q.CreateThread(ctx, sqlc.CreateThreadParams{UserID: project.UserID, SeededProjectID: pg})
	if err != nil {
		return uuid.UUID{}, err
	}
	return created.ID, nil
}

// CreateChatMessage persists one student chat turn to the project's thread
// (creating the thread on first use).
func (s *sqlcAgentStore) CreateChatMessage(ctx context.Context, projectID uuid.UUID, role, content string) error {
	threadID, err := s.getOrCreateThread(ctx, projectID)
	if err != nil {
		return err
	}
	_, err = s.q.CreateChatMessage(ctx, sqlc.CreateChatMessageParams{ThreadID: threadID, Role: role, Content: content, Modality: "text"})
	return err
}

// LoadChatHistory merges student chat_messages + prior interventions into a
// single time-ordered conversation, capped to the most-recent `limit`.
func (s *sqlcAgentStore) LoadChatHistory(ctx context.Context, projectID uuid.UUID, limit int) ([]ChatTurn, error) {
	pg := pgtype.UUID{Bytes: projectID, Valid: true}
	msgs, err := s.q.ListChatMessagesByProject(ctx, pg)
	if err != nil {
		return nil, err
	}
	ivs, err := s.q.ListInterventionsByProject(ctx, projectID)
	if err != nil {
		return nil, err
	}
	type stamped struct {
		at   time.Time
		turn ChatTurn
	}
	var all []stamped
	for _, m := range msgs {
		all = append(all, stamped{at: m.CreatedAt, turn: ChatTurn{Role: m.Role, Content: m.Content}})
	}
	for _, iv := range ivs {
		all = append(all, stamped{at: iv.CreatedAt, turn: ChatTurn{Role: "assistant", Content: iv.Body}})
	}
	sort.Slice(all, func(i, j int) bool { return all[i].at.Before(all[j].at) })
	if limit > 0 && len(all) > limit {
		all = all[len(all)-limit:]
	}
	out := make([]ChatTurn, len(all))
	for i, st := range all {
		out[i] = st.turn
	}
	return out, nil
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
		// mat.TaskID is now pgtype.UUID (material.task_id went nullable in
		// 0020); card_instances.task_id is still NOT NULL (untouched by this
		// migration), so this narrows back to uuid.UUID. Every material this
		// path runs against today still carries a valid task_id (the legacy
		// FK anchor); project-scoped materials with no task_id are Task 3/4/5
		// territory (source-log ingestion), not this adapter.
		TaskID:      uuid.UUID(mat.TaskID.Bytes),
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

// SetCardInstanceStatus writes the card_instance's lifecycle status (e.g.
// "proposed" -> "active" on open).
func (s *sqlcAgentStore) SetCardInstanceStatus(ctx context.Context, projectID, id uuid.UUID, status string) error {
	_, err := s.q.SetCardInstanceStatus(ctx, sqlc.SetCardInstanceStatusParams{
		ID:        id,
		ProjectID: pgtype.UUID{Bytes: projectID, Valid: true},
		Status:    status,
	})
	return err
}

// SetCardInstanceAnchors writes the card_instance's live anchors jsonb —
// the card runtime's per-field/observe-event state.
func (s *sqlcAgentStore) SetCardInstanceAnchors(ctx context.Context, projectID, id uuid.UUID, anchors []byte) error {
	_, err := s.q.SetCardInstanceAnchors(ctx, sqlc.SetCardInstanceAnchorsParams{
		ID:        id,
		ProjectID: pgtype.UUID{Bytes: projectID, Valid: true},
		Anchors:   anchors,
	})
	return err
}

// SubmitProjectCardInstance writes the student's final field_values +
// event_trace on submission.
func (s *sqlcAgentStore) SubmitProjectCardInstance(ctx context.Context, projectID, id uuid.UUID, fieldValues, eventTrace []byte) error {
	_, err := s.q.SubmitProjectCardInstance(ctx, sqlc.SubmitProjectCardInstanceParams{
		ID:          id,
		ProjectID:   pgtype.UUID{Bytes: projectID, Valid: true},
		FieldValues: fieldValues,
		EventTrace:  eventTrace,
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

// RecordLLMCall persists one live LLM call's usage to `llm_call` (migration
// 0019). The owning user is resolved from the project row, mirroring
// AppendEvent/getOrCreateThread's project->user resolution (Slice 2/5c's
// single-user-per-project model) — the AgentStore seam carries no separate
// "acting user" concept. Cost is computed here, once, via the shared pricing
// table (gateway.EstimateCost) rather than trusting a caller-supplied
// number. Unlike messages/evaluations' nullable cost_estimate (NULL meant
// "unpriced model" there), llm_call.cost_estimate is NOT NULL DEFAULT 0, so
// an unpriced model (EstimateCost's ok=false, cost=0) is recorded as an
// explicit $0.00 — CostNumeric(cost, true), never the ok-derived NULL
// Numeric CostNumeric(cost, ok) would otherwise produce.
func (s *sqlcAgentStore) RecordLLMCall(ctx context.Context, row LLMCallRow) error {
	project, err := s.q.GetProject(ctx, row.ProjectID)
	if err != nil {
		return err
	}
	cost, priced := gateway.EstimateCost(row.Resolved.Provider, row.Resolved.Model, int(row.PromptTokens), int(row.CompletionTokens))
	if !priced {
		// A model routed but absent from the price table bills as $0.00 and would
		// otherwise look like free usage in the org aggregate. Say so out loud.
		slog.Warn("llm_call: unpriced model — cost recorded as 0",
			"provider", row.Resolved.Provider, "model", row.Resolved.Model)
	}
	_, err = s.q.RecordLLMCall(ctx, sqlc.RecordLLMCallParams{
		UserID:           project.UserID,
		ProjectID:        pgtype.UUID{Bytes: row.ProjectID, Valid: true},
		Surface:          row.Surface,
		Purpose:          row.Purpose,
		Provider:         row.Resolved.Provider,
		Model:            row.Resolved.Model,
		Tier:             row.Resolved.Tier,
		PromptTokens:     row.PromptTokens,
		CompletionTokens: row.CompletionTokens,
		CostEstimate:     gateway.CostNumeric(cost, true),
	})
	return err
}

// gateStateBody is the gate_state graph_node body shape.
type gateStateBody struct {
	Contract       string            `json:"contract"`
	Status         string            `json:"status"`
	ConfirmedSolid bool              `json:"confirmed_solid"`
	Items          map[string]string `json:"items"`
}

// ListGateStates reads every gate_state graph_node for the project, keyed by
// its recorded contract id (Slice 4).
func (s *sqlcAgentStore) ListGateStates(ctx context.Context, projectID uuid.UUID) (map[string]RecordedGate, error) {
	rows, err := s.q.ListGateStateNodes(ctx, projectID)
	if err != nil {
		return nil, err
	}
	return RecordedGatesFromNodes(rows), nil
}

// UpsertGateState writes rec to the project's gate_state graph_node for
// contract — updating the existing row if one exists (one gate_state per
// project+contract), inserting otherwise.
func (s *sqlcAgentStore) UpsertGateState(ctx context.Context, projectID uuid.UUID, contract string, rec RecordedGate) error {
	body, err := json.Marshal(gateStateBody{
		Contract: contract, ConfirmedSolid: rec.Confirmed, Items: rec.Items,
	})
	if err != nil {
		return err
	}
	existing, err := s.q.GetGateStateNode(ctx, sqlc.GetGateStateNodeParams{ProjectID: projectID, Column2: contract})
	if err == nil {
		_, err = s.q.UpdateGraphNodeBody(ctx, sqlc.UpdateGraphNodeBodyParams{ID: existing.ID, Body: body})
		return err
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return err
	}
	_, err = s.q.InsertGraphNode(ctx, sqlc.InsertGraphNodeParams{
		ProjectID: projectID, Type: "gate_state", Body: body, Author: "ai",
	})
	return err
}

// UpsertPlan writes body to the project's single plan graph_node —
// updating the existing row if one exists, inserting otherwise.
func (s *sqlcAgentStore) UpsertPlan(ctx context.Context, projectID uuid.UUID, body []byte) error {
	existing, err := s.q.GetPlanNode(ctx, projectID)
	if err == nil {
		_, err = s.q.UpdateGraphNodeBody(ctx, sqlc.UpdateGraphNodeBodyParams{ID: existing.ID, Body: body})
		return err
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return err
	}
	_, err = s.q.InsertGraphNode(ctx, sqlc.InsertGraphNodeParams{
		ProjectID: projectID, Type: "plan", Body: body, Author: "ai",
	})
	return err
}
