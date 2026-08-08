package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"mindimprint/api/internal/gateway"
	"mindimprint/api/internal/store/sqlc"
)

// TxBeginner is the minimal pool capability CommitCardMint needs: starting a
// transaction. Mirrors internal/api's TxBeginner (same one-method shape,
// redeclared here rather than shared — internal/api imports internal/agent,
// so the reverse import would cycle). Any value satisfying api.TxBeginner
// (e.g. api.Deps.Pool) also satisfies this interface, and so does a bare
// *pgxpool.Pool: Go interface assignability only cares about the method set.
type TxBeginner interface {
	Begin(ctx context.Context) (pgx.Tx, error)
}

// sqlcAgentStore adapts sqlc queries to the AgentStore seam (loop.go): graph
// reads feed LoadGraph, and InsertIntervention/AppendEvent record the
// coach's emitted action. pool backs CommitCardMint's transaction — every
// other method still runs unwrapped queries through q.
type sqlcAgentStore struct {
	q    *sqlc.Queries
	pool TxBeginner
}

// NewSqlcAgentStore adapts sqlc queries to the AgentStore seam. pool is used
// only by CommitCardMint, to begin the one transaction a completed card's
// mint (nodes + edges + framework_fill guard) commits inside.
func NewSqlcAgentStore(q *sqlc.Queries, pool TxBeginner) *sqlcAgentStore {
	return &sqlcAgentStore{q: q, pool: pool}
}

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
		SessionID: pgtype.UUID{Valid: false},
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

// AppendProjectCoachMessage persists one surface-tagged coach turn (role
// "user"|"assistant") to the project's ONE thread (S1 · one continuous
// session), creating the thread on first use. surface is the room the turn
// happened on; "" stores NULL (untagged). stage is the studio_state.stage in
// effect when the turn was persisted (过程即数据 — the evaluation layer reads
// the per-turn lifecycle arc); "" stores NULL, same convention as surface.
// Unlike CreateChatMessage (the intervention-loop's student-only writer),
// this persists BOTH sides of the conversational coach exchange, since the
// four-room coach's replies are conversational (chat_message), not
// interventions.
func (s *sqlcAgentStore) AppendProjectCoachMessage(ctx context.Context, projectID uuid.UUID, role, content, surface, stage string) error {
	return s.appendProjectCoachMessage(ctx, projectID, role, content, surface, stage, nil)
}

// AppendProjectCoachCardMessage is AppendProjectCoachMessage for a CARD turn:
// the same persisted chat turn, but carrying a structured card reference in the
// attachments jsonb ({"card":{"cardId","fieldValues"}}) so a reloaded thread
// re-renders the completed card as a clickable, content-first chip (opening a
// read-only record) instead of the plain compiled text. `content` stays the
// compiled fallback (any non-card-aware reader still sees the words).
func (s *sqlcAgentStore) AppendProjectCoachCardMessage(ctx context.Context, projectID uuid.UUID, role, content, surface, stage string, attachments []byte) error {
	return s.appendProjectCoachMessage(ctx, projectID, role, content, surface, stage, attachments)
}

// appendProjectCoachMessage is the shared body. attachments defaults to the
// column's empty-array shape ('[]') when nil so the NOT NULL column never sees a
// NULL (the explicit param bypasses the SQL DEFAULT).
func (s *sqlcAgentStore) appendProjectCoachMessage(ctx context.Context, projectID uuid.UUID, role, content, surface, stage string, attachments []byte) error {
	threadID, err := s.getOrCreateThread(ctx, projectID)
	if err != nil {
		return err
	}
	var surfPtr *string
	if surface != "" {
		surfPtr = &surface
	}
	var stagePtr *string
	if stage != "" {
		stagePtr = &stage
	}
	if len(attachments) == 0 {
		attachments = []byte("[]")
	}
	_, err = s.q.CreateProjectCoachMessage(ctx, sqlc.CreateProjectCoachMessageParams{
		ThreadID: threadID, Role: role, Content: content, Surface: surfPtr, Attachments: attachments, Stage: stagePtr,
	})
	return err
}

// LoadActiveCoachHistory reads the coach's CONTEXT window: every NON-FOLDED
// turn on the project's thread, both roles, oldest→newest, capped to the most
// recent `limit`. Continuity ignores surface — the one agent sees the whole
// thread; folded turns (already distilled into the spine) are excluded so the
// window stays lean (S1 · lever 1).
func (s *sqlcAgentStore) LoadActiveCoachHistory(ctx context.Context, projectID uuid.UUID, limit int) ([]ChatTurn, error) {
	pg := pgtype.UUID{Bytes: projectID, Valid: true}
	msgs, err := s.q.ListActiveChatMessagesByProject(ctx, pg)
	if err != nil {
		return nil, err
	}
	turns := make([]ChatTurn, 0, len(msgs))
	for _, m := range msgs {
		turns = append(turns, ChatTurn{Role: m.Role, Content: m.Content})
	}
	if limit > 0 && len(turns) > limit {
		turns = turns[len(turns)-limit:]
	}
	return turns, nil
}

// FoldCoachSurfaces folds every live turn on the named surfaces into the spine
// (S1 · lever 1, compaction): set folded_at so those raw turns leave the
// coach's active window the moment their artifact solidifies (proposal
// finalized → forming/proposal_review; plan generated → plan). The turns stay
// in the thread (still shown on reload); the spine now carries the result.
// Best-effort at the call site — a fold failure must never fail the save.
func (s *sqlcAgentStore) FoldCoachSurfaces(ctx context.Context, projectID uuid.UUID, surfaces []string) error {
	return s.q.FoldChatSurface(ctx, sqlc.FoldChatSurfaceParams{
		SeededProjectID: pgtype.UUID{Bytes: projectID, Valid: true},
		Column2:         surfaces,
	})
}

// CreateCardInstance instantiates a proposed card_instance for cardID on
// materialID. card_instances.task_id went nullable alongside material.task_id
// (migration 0020) — project-scoped materials from Slice 6b's source-log
// ingestion carry no task_id, so this adapter passes materialID's own
// task_id straight through (NULL stays NULL), keeping the pure agent code
// (card_lifecycle.go) ignorant of tasks either way.
func (s *sqlcAgentStore) CreateCardInstance(ctx context.Context, projectID, materialID uuid.UUID, cardID, contractRef string) (CardInstanceRow, error) {
	// A project-scoped card (materialID == uuid.Nil, e.g. toulmin) is not about
	// any one material, so there is no material row to inherit a task_id from —
	// it simply carries a NULL task_id. Only look a material up when there is one
	// (GetMaterial(uuid.Nil) would error with no rows).
	var taskID pgtype.UUID
	if materialID != uuid.Nil {
		mat, err := s.q.GetMaterial(ctx, materialID)
		if err != nil {
			return CardInstanceRow{}, err
		}
		taskID = mat.TaskID
	}
	row, err := s.q.CreateProjectCardInstance(ctx, sqlc.CreateProjectCardInstanceParams{
		TaskID:      taskID,
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

// CountCompletedCardUsesByUser counts this user's status='completed'
// card_instances for cardID, PROJECT SCOPE ONLY (user-scoped across ALL her
// projects) — the guidance fade's producer (guidance.go, spec §3).
//
// Whole-branch review IMPORTANT 2: this deliberately DIVERGES from
// ListCollectedCardsByUser (store/queries/card_instance.sql), which stays
// three-scope (project/course/chat) for the 工具卡 tab. Only the project
// scope's "completed" is gated on the card's own completion predicate
// (projectcards.go's CompleteCard runs it before flipping status); chat.go
// and course_session.go both write status='completed' unconditionally on
// submit, with no predicate and no anchors (those cards are thin by
// design). Counting those ungated surfaces here would let a student remove
// her own AI scaffolding by submitting throwaway thin sheets she never
// actually did the card's work on — see the query's own SQL comment and the
// design doc's §3 for the full reasoning.
func (s *sqlcAgentStore) CountCompletedCardUsesByUser(ctx context.Context, userID uuid.UUID, cardID string) (int, error) {
	n, err := s.q.CountCompletedCardUsesByUser(ctx, sqlc.CountCompletedCardUsesByUserParams{
		UserID: userID, CardID: cardID,
	})
	if err != nil {
		return 0, err
	}
	return int(n), nil
}

// CountClassifierCalls backs the N6 C4 spend cap (loop.go's
// MaxClassifyCallsPerProject): how many moment-classifier calls
// (Purpose="classify") this project has already been metered for.
func (s *sqlcAgentStore) CountClassifierCalls(ctx context.Context, projectID uuid.UUID) (int64, error) {
	return s.q.CountLLMCallsByProjectPurpose(ctx, sqlc.CountLLMCallsByProjectPurposeParams{
		ProjectID: pgtype.UUID{Bytes: projectID, Valid: true},
		Purpose:   "classify",
	})
}

// GetSourceLogByMaterial reads one source_log_entry's ingestion-time tier —
// what CompleteCard reads as tier_before, BEFORE a cross_check's mint
// overwrites it with her post-check re-tier (Task 7). tier is nullable in
// storage (*string); a NULL collapses to "" here, same as an entry with no
// tier ever recorded.
func (s *sqlcAgentStore) GetSourceLogByMaterial(ctx context.Context, materialID uuid.UUID) (SourceLogRow, error) {
	row, err := s.q.GetSourceLogByMaterial(ctx, pgtype.UUID{Bytes: materialID, Valid: true})
	if err != nil {
		return SourceLogRow{}, err
	}
	tier := ""
	if row.Tier != nil {
		tier = *row.Tier
	}
	return SourceLogRow{Tier: tier}, nil
}

// SetCardInstanceFramework writes the consolidation payload (R-9: revealed
// only after completion) to framework_fill.
func (s *sqlcAgentStore) SetCardInstanceFramework(ctx context.Context, projectID, id uuid.UUID, framework []byte) error {
	return setCardInstanceFrameworkQ(ctx, s.q, projectID, id, framework)
}

// setCardInstanceFrameworkQ is SetCardInstanceFramework's body, parameterized
// over *sqlc.Queries so both the plain method (s.q) and CommitCardMint's
// transaction (qtx := s.q.WithTx(tx)) share one mapping — no duplicated SQL
// param-building to drift out of sync.
func setCardInstanceFrameworkQ(ctx context.Context, q *sqlc.Queries, projectID, id uuid.UUID, framework []byte) error {
	_, err := q.SetCardInstanceFramework(ctx, sqlc.SetCardInstanceFrameworkParams{
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

// SubmitAndSkipCardInstance persists the student's (empty, on skip)
// field_values + event_trace AND flips the status to "skipped" in ONE
// transaction, so a crash can never leave a saved envelope with a stale
// "active" status (or the reverse). 过程即数据: the two records of one act
// cannot drift. Mirrors CommitCardMint's tx shape.
func (s *sqlcAgentStore) SubmitAndSkipCardInstance(ctx context.Context, projectID, id uuid.UUID, fieldValues, eventTrace []byte) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	qtx := s.q.WithTx(tx)
	if _, err := qtx.SubmitProjectCardInstance(ctx, sqlc.SubmitProjectCardInstanceParams{
		ID: id, ProjectID: pgtype.UUID{Bytes: projectID, Valid: true},
		FieldValues: fieldValues, EventTrace: eventTrace,
	}); err != nil {
		return err
	}
	if _, err := qtx.SetCardInstanceStatus(ctx, sqlc.SetCardInstanceStatusParams{
		ID: id, ProjectID: pgtype.UUID{Bytes: projectID, Valid: true}, Status: "skipped",
	}); err != nil {
		return err
	}
	return tx.Commit(ctx)
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
		FieldValues:   row.FieldValues,
	}
}

// InsertGraphNode mints one graph_node from a card's graph_effects
// (card_effects.go's MintNode). node.Body is marshaled to jsonb; an empty
// map still marshals cleanly.
func (s *sqlcAgentStore) InsertGraphNode(ctx context.Context, projectID uuid.UUID, node MintNode) (uuid.UUID, error) {
	return insertGraphNodeQ(ctx, s.q, projectID, node)
}

// insertGraphNodeQ is InsertGraphNode's body, parameterized over
// *sqlc.Queries — see setCardInstanceFrameworkQ's comment for why.
func insertGraphNodeQ(ctx context.Context, q *sqlc.Queries, projectID uuid.UUID, node MintNode) (uuid.UUID, error) {
	body, err := json.Marshal(node.Body)
	if err != nil {
		return uuid.UUID{}, err
	}
	row, err := q.InsertGraphNode(ctx, sqlc.InsertGraphNodeParams{
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
	return insertGraphEdgeQ(ctx, s.q, projectID, edge)
}

// insertGraphEdgeQ is InsertGraphEdge's body, parameterized over
// *sqlc.Queries — see setCardInstanceFrameworkQ's comment for why.
func insertGraphEdgeQ(ctx context.Context, q *sqlc.Queries, projectID uuid.UUID, edge MintEdge) error {
	fromID, err := uuid.Parse(edge.FromID)
	if err != nil {
		return err
	}
	toID, err := uuid.Parse(edge.ToID)
	if err != nil {
		return err
	}
	_, err = q.InsertGraphEdge(ctx, sqlc.InsertGraphEdgeParams{
		ProjectID: projectID,
		Type:      edge.Type,
		FromKind:  edge.FromKind,
		FromID:    fromID,
		ToKind:    edge.ToKind,
		ToID:      toID,
	})
	return err
}

// LateralRead is the source-log side of a cross_check: the CHECKED source is
// marked laterally read and re-tiered to the pyramid level she landed on after
// checking. Written inside CommitCardMint's transaction, so the graph node
// (which the gate reads) and the log row (which the dossier chip and the
// ledger read) cannot disagree.
type LateralRead struct {
	MaterialID uuid.UUID
	TierAfter  string
}

// CardMint is everything a completed card produces (CompleteCard,
// card_lifecycle.go): the nodes/edges GraphEffects minted, plus the
// consolidation payload that also serves as the idempotency guard
// (isFrameworkSet). MintEdge endpoints may still carry "$new:<i>"
// placeholders into Nodes — CommitCardMint resolves them after inserting
// each node, exactly as CompleteCard used to. LateralRead is nil for every
// card except a cross_check (only SIFT's lateral-read card sets it).
type CardMint struct {
	Nodes       []MintNode
	Edges       []MintEdge
	Framework   []byte
	LateralRead *LateralRead
}

// CommitCardMint writes everything in m — the minted nodes, their edges
// (with $new: placeholders resolved against the just-inserted node ids), and
// the consolidation framework — in ONE transaction.
//
// Why this exists: framework_fill is written LAST and is also the
// idempotency guard CompleteCard checks first (isFrameworkSet). Before this
// method, CompleteCard called InsertGraphNode, then InsertGraphEdge, then
// SetCardInstanceFramework as three separate, unwrapped store calls. A
// failure between the node insert and the framework write left a partial
// mint on disk with the guard still unset, so a retry sailed past the guard
// and minted a SECOND node/edge for the same completion. Wrapping all three
// writes in one transaction makes the mint all-or-nothing: any failure
// anywhere in the sequence rolls back every write that came before it in
// this same call, so a retry always starts from "nothing minted yet".
func (s *sqlcAgentStore) CommitCardMint(ctx context.Context, projectID, cardInstanceID uuid.UUID, m CardMint) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	// A no-op Rollback after a successful Commit is expected pgx behavior
	// (it errors ErrTxClosed, which we discard) — the defer is just the
	// safety net for every early return above.
	defer func() { _ = tx.Rollback(ctx) }()
	qtx := s.q.WithTx(tx)

	nodeIDs := make(map[int]uuid.UUID, len(m.Nodes))
	for i, n := range m.Nodes {
		id, err := insertGraphNodeQ(ctx, qtx, projectID, n)
		if err != nil {
			return err
		}
		nodeIDs[i] = id
	}
	for _, e := range m.Edges {
		fromID, err := resolveMintRef(e.FromID, nodeIDs)
		if err != nil {
			return err
		}
		toID, err := resolveMintRef(e.ToID, nodeIDs)
		if err != nil {
			return err
		}
		e.FromID, e.ToID = fromID.String(), toID.String()
		if err := insertGraphEdgeQ(ctx, qtx, projectID, e); err != nil {
			return err
		}
	}
	if m.LateralRead != nil {
		rows, err := qtx.MarkSourceLateralRead(ctx, sqlc.MarkSourceLateralReadParams{
			ProjectID:  projectID,
			MaterialID: pgtype.UUID{Bytes: m.LateralRead.MaterialID, Valid: true},
			TierAfter:  m.LateralRead.TierAfter,
		})
		if err != nil {
			return err
		}
		// A zero-row match means the checked material has no
		// source_log_entry at all — a mint that cannot record its own log
		// flip must not half-land (spec §4.3: "two records, one act, one
		// transaction — they cannot drift"). Failing loudly here rolls back
		// the whole transaction (node/edge inserts included) instead of
		// leaving lateral_read stuck false forever with no honest way to
		// clear it (whole-branch review IMPORTANT 4).
		if rows == 0 {
			return fmt.Errorf("card: MarkSourceLateralRead matched no source_log_entry for project %s material %s", projectID, m.LateralRead.MaterialID)
		}
	}
	if err := setCardInstanceFrameworkQ(ctx, qtx, projectID, cardInstanceID, m.Framework); err != nil {
		return err
	}
	return tx.Commit(ctx)
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

// ReviewInterventionRow is one persisted whole-draft-review work-order item
// (Task 6, orderReview). Body is the JSON of the full agent.ReviewItem — a
// review reconstructs it with one json.Unmarshal, never by string-splitting
// the flat Criterion/Level columns, which stay duplicated only for SQL
// filters.
type ReviewInterventionRow struct {
	ProjectID uuid.UUID
	Anchor    []byte
	Criterion string
	Body      string
	Level     string
}

// InsertReviewIntervention writes one review_item intervention row. No
// card_instance_id — a whole-draft review is not a card submission.
func (s *sqlcAgentStore) InsertReviewIntervention(ctx context.Context, row ReviewInterventionRow) error {
	_, err := s.q.InsertIntervention(ctx, sqlc.InsertInterventionParams{
		ProjectID: row.ProjectID,
		Type:      "review_item",
		Anchor:    row.Anchor,
		Criterion: &row.Criterion,
		Body:      row.Body,
		Level:     &row.Level,
	})
	return err
}

// SpotCheckInterventionRow is one persisted station spot-check item. Anchor
// carries {"station":…,"fingerprint":…} — the additive-anchor-field seam Slice
// 8b used for `voice`, so no migration. No card_instance_id: a station
// spot-check is not a card submission.
type SpotCheckInterventionRow struct {
	ProjectID uuid.UUID
	Anchor    []byte
	Body      string
}

// InsertSpotCheckIntervention writes one spot_check_item intervention row.
func (s *sqlcAgentStore) InsertSpotCheckIntervention(ctx context.Context, row SpotCheckInterventionRow) error {
	_, err := s.q.InsertIntervention(ctx, sqlc.InsertInterventionParams{
		ProjectID: row.ProjectID,
		Type:      "spot_check_item",
		Anchor:    row.Anchor,
		Body:      row.Body,
	})
	return err
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
	return upsertGateStateQ(ctx, s.q, projectID, contract, rec)
}

// upsertGateStateQ is UpsertGateState's body, parameterized over
// *sqlc.Queries — see setCardInstanceFrameworkQ's comment for why. ConfirmGate
// (N6 C3) calls this with a qtx := s.q.WithTx(tx) so the gate-state write and
// its passed event land in the same transaction.
func upsertGateStateQ(ctx context.Context, q *sqlc.Queries, projectID uuid.UUID, contract string, rec RecordedGate) error {
	body, err := json.Marshal(gateStateBody{
		Contract: contract, ConfirmedSolid: rec.Confirmed, Items: rec.Items,
	})
	if err != nil {
		return err
	}
	existing, err := q.GetGateStateNode(ctx, sqlc.GetGateStateNodeParams{ProjectID: projectID, Column2: contract})
	if err == nil {
		_, err = q.UpdateGraphNodeBody(ctx, sqlc.UpdateGraphNodeBodyParams{ID: existing.ID, Body: body})
		return err
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return err
	}
	_, err = q.InsertGraphNode(ctx, sqlc.InsertGraphNodeParams{
		ProjectID: projectID, Type: "gate_state", Body: body, Author: "ai",
	})
	return err
}

// ConfirmGate confirms rec's gate state AND appends passedEvent (the
// gate_attempt result:passed event) in ONE transaction (N6 C3) — closing the
// gap where a separate UpsertGateState + AppendEvent pair could leave the
// gate reading solid while the process tree has no record it passed. Modelled
// on CommitCardMint's transaction shape and AppendEvent's project->user
// resolution (Slice 2's single-user-per-project model).
func (s *sqlcAgentStore) ConfirmGate(ctx context.Context, projectID uuid.UUID, contract string, rec RecordedGate, passedEvent EventRow) error {
	project, err := s.q.GetProject(ctx, projectID)
	if err != nil {
		return err
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	qtx := s.q.WithTx(tx)

	if err := upsertGateStateQ(ctx, qtx, projectID, contract, rec); err != nil {
		return err
	}
	if _, err := qtx.AppendEvent(ctx, sqlc.AppendEventParams{
		ProjectID: pgtype.UUID{Bytes: projectID, Valid: true},
		UserID:    project.UserID,
		SessionID: pgtype.UUID{Valid: false},
		Surface:   passedEvent.Surface,
		Type:      passedEvent.Type,
		Payload:   passedEvent.Payload,
	}); err != nil {
		return err
	}
	return tx.Commit(ctx)
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

// LoadWaived reads the project's waived-set from the plan graph-node body.
// A project with no plan node yet (fresh, pre-Replan) has an empty journey =
// full template. Never errors on absence.
func (s *sqlcAgentStore) LoadWaived(ctx context.Context, projectID uuid.UUID) (map[string]bool, error) {
	node, err := s.q.GetPlanNode(ctx, projectID)
	if errors.Is(err, pgx.ErrNoRows) {
		return map[string]bool{}, nil
	}
	if err != nil {
		return nil, err
	}
	var b struct {
		Waived []string `json:"waived"`
	}
	if json.Unmarshal(node.Body, &b) != nil {
		return map[string]bool{}, nil // malformed body → treat as full journey
	}
	m := make(map[string]bool, len(b.Waived))
	for _, id := range b.Waived {
		m[id] = true
	}
	return m, nil
}

// SetWaived writes the waived-set into the plan node, PRESERVING any existing
// route/reason so a compose/re-open does not clobber a computed route (and a
// later Replan does not clobber the waived-set — writePlan reloads it).
func (s *sqlcAgentStore) SetWaived(ctx context.Context, projectID uuid.UUID, waived []string) error {
	body := planBody{Reason: "journey", Waived: waived}
	if node, err := s.q.GetPlanNode(ctx, projectID); err == nil {
		var cur planBody
		if json.Unmarshal(node.Body, &cur) == nil {
			body.Route = cur.Route
			if cur.Reason != "" {
				body.Reason = cur.Reason
			}
		}
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return err
	}
	raw, err := json.Marshal(body)
	if err != nil {
		return err
	}
	return s.UpsertPlan(ctx, projectID, raw)
}
