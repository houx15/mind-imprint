package agent

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"mindimprint/api/internal/cards"
	"mindimprint/api/internal/gateway"
	"mindimprint/api/internal/store/sqlc"
)

// SSEEmitter is the subset of *gateway.SSEWriter the turn engine needs. RunTurn
// relays text/card/done through it; a test can inject a recorder.
type SSEEmitter interface {
	Text(delta string) error
	Card(cardInstanceID, cardID, nudgeText string) error
	Done(messageID string) error
}

// AssistantMessage is the persisted assistant row (content + optional tool_call +
// per-turn usage).
type AssistantMessage struct {
	TaskID           uuid.UUID
	Content          string
	ToolCall         *SummonCardCall
	Provider         string
	Model            string
	Tier             string
	PromptTokens     *int32
	CompletionTokens *int32
}

// TurnStore is the persistence seam RunTurn drives. NewSqlcTurnStore adapts the
// generated sqlc queries to it.
type TurnStore interface {
	AppendUserMessage(ctx context.Context, taskID uuid.UUID, content string) (uuid.UUID, error)
	ListMessages(ctx context.Context, taskID uuid.UUID) ([]StoredMessage, error)
	CardByID(ctx context.Context, id string) (CardInstance, bool, error)
	CreateProposedCard(ctx context.Context, taskID uuid.UUID, cardID string) (uuid.UUID, error)
	AppendAssistantMessage(ctx context.Context, in AssistantMessage) (uuid.UUID, error)
}

// TurnDeps are the inputs to RunTurn.
type TurnDeps struct {
	Store       TurnStore
	Provider    gateway.Provider
	KeyResolver gateway.KeyResolver
	Catalog     []cards.Spec
	SpecByID    func(id string) (cards.Spec, bool)
	SSE         SSEEmitter
}

// RunTurn runs one model turn: append the user message, build history + system
// prompt + tool, stream the provider, relay text deltas, and on a summon_card
// tool-use validate + persist a proposed card_instance and the assistant message
// (with usage), emitting the card event and ending the turn. One card per turn.
// Always emits done.
func RunTurn(ctx context.Context, deps TurnDeps, taskID uuid.UUID, userInput string) error {
	if _, err := deps.Store.AppendUserMessage(ctx, taskID, userInput); err != nil {
		return err
	}

	history, err := deps.Store.ListMessages(ctx, taskID)
	if err != nil {
		return err
	}

	systemPrompt := BuildSystemPrompt(deps.Catalog)
	llmMessages := BuildLlmMessages(BuildLlmMessagesOptions{
		SystemPrompt: systemPrompt,
		Messages:     history,
		CardByID: func(id string) (CardInstance, bool) {
			ci, ok, cerr := deps.Store.CardByID(ctx, id)
			if cerr != nil {
				return CardInstance{}, false
			}
			return ci, ok
		},
		SpecByID: deps.SpecByID,
	})

	resolved, err := deps.KeyResolver(ctx)
	if err != nil {
		return err
	}

	// Bind the provider stream to a cancelable child context so that any early
	// return from RunTurn (e.g. an SSE write error) cancels the stream goroutine
	// inside the adapter, preventing a goroutine + connection leak.
	streamCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	req := gateway.ChatRequest{
		Messages: llmMessages,
		Tools:    []gateway.ChatTool{SummonCardTool(deps.Catalog)},
	}
	stream, err := deps.Provider.Stream(streamCtx, resolved, req)
	if err != nil {
		return err
	}

	var textBuf string
	var usage gateway.ChatUsage
	var firstTool *gateway.StreamToolUse

	for ev := range stream {
		switch ev.Kind {
		case gateway.EventTextDelta:
			textBuf += ev.TextDelta
			if err := deps.SSE.Text(ev.TextDelta); err != nil {
				return err
			}
		case gateway.EventToolUse:
			if firstTool == nil && ev.ToolUse != nil && ev.ToolUse.Name == "summon_card" {
				firstTool = ev.ToolUse // one card per turn — take the first
			}
		case gateway.EventUsage:
			if ev.Usage != nil {
				usage = *ev.Usage
			}
		case gateway.EventDone:
			// terminal; loop ends when channel closes
		}
	}

	promptTokens := int32(usage.InputTokens)
	completionTokens := int32(usage.OutputTokens)

	// Did the model propose a valid card?
	if firstTool != nil {
		var args SummonCardArgs
		if err := json.Unmarshal([]byte(firstTool.ArgsJSON), &args); err == nil && args.CardID != "" {
			if _, ok := deps.SpecByID(args.CardID); ok {
				cardInstanceID, err := deps.Store.CreateProposedCard(ctx, taskID, args.CardID)
				if err != nil {
					return err
				}
				call := &SummonCardCall{
					ID:             firstTool.ID,
					Name:           "summon_card",
					Args:           args,
					CardInstanceID: cardInstanceID.String(),
				}
				msgID, err := deps.Store.AppendAssistantMessage(ctx, AssistantMessage{
					TaskID:           taskID,
					Content:          textBuf,
					ToolCall:         call,
					Provider:         resolved.Provider,
					Model:            resolved.Model,
					Tier:             resolved.Tier,
					PromptTokens:     &promptTokens,
					CompletionTokens: &completionTokens,
				})
				if err != nil {
					return err
				}
				if err := deps.SSE.Card(cardInstanceID.String(), args.CardID, args.NudgeText); err != nil {
					return err
				}
				return deps.SSE.Done(msgID.String())
			}
		}
		// Invalid args or unknown card_id → fall through to plain text.
	}

	// Plain-text reply.
	msgID, err := deps.Store.AppendAssistantMessage(ctx, AssistantMessage{
		TaskID:           taskID,
		Content:          textBuf,
		Provider:         resolved.Provider,
		Model:            resolved.Model,
		Tier:             resolved.Tier,
		PromptTokens:     &promptTokens,
		CompletionTokens: &completionTokens,
	})
	if err != nil {
		return err
	}
	return deps.SSE.Done(msgID.String())
}

// --- sqlc adapter -----------------------------------------------------------

// sqlcTurnStore adapts the generated sqlc.Queries to TurnStore.
type sqlcTurnStore struct {
	q *sqlc.Queries
}

// NewSqlcTurnStore wraps sqlc queries as a TurnStore.
func NewSqlcTurnStore(q *sqlc.Queries) TurnStore { return &sqlcTurnStore{q: q} }

func (s *sqlcTurnStore) AppendUserMessage(ctx context.Context, taskID uuid.UUID, content string) (uuid.UUID, error) {
	m, err := s.q.AppendMessage(ctx, sqlc.AppendMessageParams{TaskID: taskID, Role: "user", Content: content})
	return m.ID, err
}

func (s *sqlcTurnStore) ListMessages(ctx context.Context, taskID uuid.UUID) ([]StoredMessage, error) {
	rows, err := s.q.ListMessagesByTask(ctx, taskID)
	if err != nil {
		return nil, err
	}
	out := make([]StoredMessage, 0, len(rows))
	for _, r := range rows {
		sm := StoredMessage{Role: r.Role, Content: r.Content}
		if len(r.ToolCall) > 0 {
			var call SummonCardCall
			if err := json.Unmarshal(r.ToolCall, &call); err == nil && call.Name == "summon_card" {
				sm.ToolCall = &call
			}
		}
		out = append(out, sm)
	}
	return out, nil
}

func (s *sqlcTurnStore) CardByID(ctx context.Context, id string) (CardInstance, bool, error) {
	cid, err := uuid.Parse(id)
	if err != nil {
		return CardInstance{}, false, nil
	}
	row, err := s.q.GetCard(ctx, cid)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return CardInstance{}, false, nil
		}
		return CardInstance{}, false, err
	}
	ci := CardInstance{
		ID:     row.ID.String(),
		CardID: row.CardID,
		TaskID: row.TaskID.String(),
		Status: row.Status,
	}
	if len(row.FieldValues) > 0 {
		_ = json.Unmarshal(row.FieldValues, &ci.FieldValues)
	}
	return ci, true, nil
}

func (s *sqlcTurnStore) CreateProposedCard(ctx context.Context, taskID uuid.UUID, cardID string) (uuid.UUID, error) {
	row, err := s.q.CreateCardInstance(ctx, sqlc.CreateCardInstanceParams{CardID: cardID, TaskID: taskID})
	return row.ID, err
}

func (s *sqlcTurnStore) AppendAssistantMessage(ctx context.Context, in AssistantMessage) (uuid.UUID, error) {
	params := sqlc.AppendMessageParams{
		TaskID:           in.TaskID,
		Role:             "assistant",
		Content:          in.Content,
		PromptTokens:     in.PromptTokens,
		CompletionTokens: in.CompletionTokens,
	}
	if in.Provider != "" {
		params.Provider = &in.Provider
	}
	if in.Model != "" {
		params.Model = &in.Model
	}
	if in.Tier != "" {
		params.Tier = &in.Tier
	}
	if in.ToolCall != nil {
		b, _ := json.Marshal(in.ToolCall)
		params.ToolCall = b
	}
	// cost_estimate left zero/null for P1.2; pricing rollup is P1.3.
	params.CostEstimate = pgtype.Numeric{}
	m, err := s.q.AppendMessage(ctx, params)
	return m.ID, err
}
