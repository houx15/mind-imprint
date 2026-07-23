package agent

import (
	"context"
	"log/slog"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"mindimprint/api/internal/gateway"
	"mindimprint/api/internal/store/sqlc"
)

// sqlcChatStore adapts sqlc queries to the ChatStore seam (chat_step.go).
// Isolated from sqlcAgentStore: chat threads are project-free, so every
// method here is keyed by threadID/userID, never projectID.
type sqlcChatStore struct{ q *sqlc.Queries }

// NewSqlcChatStore adapts sqlc queries to the ChatStore seam.
func NewSqlcChatStore(q *sqlc.Queries) ChatStore { return &sqlcChatStore{q: q} }

// pgUUID wraps a uuid.UUID for the nullable pgtype.UUID columns
// (material.thread_id, card_instance.thread_id, llm_call.project_id,
// event.project_id) that chat rows pass through as their FK.
func pgUUID(id uuid.UUID) pgtype.UUID { return pgtype.UUID{Bytes: id, Valid: true} }

func (s *sqlcChatStore) LoadThreadHistory(ctx context.Context, threadID uuid.UUID, limit int) ([]ChatTurn, error) {
	msgs, err := s.q.ListMessagesByThread(ctx, threadID)
	if err != nil {
		return nil, err
	}
	if limit > 0 && len(msgs) > limit {
		msgs = msgs[len(msgs)-limit:]
	}
	out := make([]ChatTurn, 0, len(msgs))
	for _, m := range msgs {
		out = append(out, ChatTurn{Role: m.Role, Content: m.Content})
	}
	return out, nil
}

func (s *sqlcChatStore) CreateThreadMessage(ctx context.Context, threadID uuid.UUID, role, content, modality string) (uuid.UUID, error) {
	m, err := s.q.CreateChatMessage(ctx, sqlc.CreateChatMessageParams{ThreadID: threadID, Role: role, Content: content, Modality: modality})
	if err != nil {
		return uuid.Nil, err
	}
	return m.ID, nil
}

func (s *sqlcChatStore) ListThreadMaterials(ctx context.Context, threadID uuid.UUID) ([]ScopedMaterial, error) {
	rows, err := s.q.ListMaterialsByThread(ctx, pgUUID(threadID))
	if err != nil {
		return nil, err
	}
	out := make([]ScopedMaterial, 0, len(rows))
	for _, r := range rows {
		sourceURL := ""
		if r.SourceUrl != nil {
			sourceURL = *r.SourceUrl
		}
		out = append(out, ScopedMaterial{ID: r.ID, Kind: r.Kind, SourceURL: sourceURL})
	}
	return out, nil
}

func (s *sqlcChatStore) CreateThreadMaterial(ctx context.Context, threadID uuid.UUID, kind, source, title, sourceURL string) (uuid.UUID, error) {
	var sourceURLPtr *string
	if sourceURL != "" {
		sourceURLPtr = &sourceURL
	}
	m, err := s.q.CreateThreadMaterial(ctx, sqlc.CreateThreadMaterialParams{
		ThreadID: pgUUID(threadID), Kind: kind, Source: source, Title: title,
		SourceUrl: sourceURLPtr, Blocks: []byte("[]"),
	})
	if err != nil {
		return uuid.Nil, err
	}
	return m.ID, nil
}

func (s *sqlcChatStore) ListThreadCards(ctx context.Context, threadID uuid.UUID) ([]ScopedCard, error) {
	rows, err := s.q.ListCardInstancesByThread(ctx, pgUUID(threadID))
	if err != nil {
		return nil, err
	}
	out := make([]ScopedCard, 0, len(rows))
	for _, r := range rows {
		out = append(out, ScopedCard{ID: r.ID, CardID: r.CardID, Status: r.Status, FieldValues: r.FieldValues, Anchors: r.Anchors})
	}
	return out, nil
}

func (s *sqlcChatStore) CreateThreadCardInstance(ctx context.Context, threadID uuid.UUID, cardID string) (uuid.UUID, error) {
	ci, err := s.q.CreateThreadCardInstance(ctx, sqlc.CreateThreadCardInstanceParams{ThreadID: pgUUID(threadID), CardID: cardID, Status: "proposed"})
	if err != nil {
		return uuid.Nil, err
	}
	return ci.ID, nil
}

func (s *sqlcChatStore) InsertThreadEvent(ctx context.Context, threadID uuid.UUID, typ string, payload []byte) error {
	th, err := s.q.GetThread(ctx, threadID)
	if err != nil {
		return err
	}
	_, err = s.q.AppendEvent(ctx, sqlc.AppendEventParams{
		ProjectID: pgtype.UUID{Valid: false},
		UserID:    th.UserID,
		SessionID: pgtype.UUID{Valid: false},
		ThreadID:  pgUUID(threadID),
		Surface:   "chat",
		Type:      typ,
		Payload:   payload,
	})
	return err
}

func (s *sqlcChatStore) RecordChatLLMCall(ctx context.Context, userID uuid.UUID, purpose string, resolved gateway.Resolved, prompt, completion int32) error {
	cost, priced := gateway.EstimateCost(resolved.Provider, resolved.Model, int(prompt), int(completion))
	if !priced {
		slog.Warn("chat llm_call: unpriced model — cost recorded as 0", "provider", resolved.Provider, "model", resolved.Model)
	}
	// llm_call.cost_estimate is NOT NULL, so an unpriced model records an
	// explicit $0.00 via CostNumeric(cost, true) (with the warn above) — NOT
	// the nullable-evaluation NULL. See gateway/pricing.go and agentstore.go.
	_, err := s.q.RecordLLMCall(ctx, sqlc.RecordLLMCallParams{
		UserID: userID, ProjectID: pgtype.UUID{Valid: false},
		Surface: "chat", Purpose: purpose,
		Provider: resolved.Provider, Model: resolved.Model, Tier: resolved.Tier,
		PromptTokens: prompt, CompletionTokens: completion, CostEstimate: gateway.CostNumeric(cost, true),
	})
	return err
}
