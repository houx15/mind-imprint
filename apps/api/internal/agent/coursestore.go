package agent

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"mindimprint/api/internal/gateway"
	"mindimprint/api/internal/materialize"
	"mindimprint/api/internal/store/sqlc"
)

// sqlcCourseStore adapts sqlc queries to the CourseStore seam (course_step.go).
// Isolated from sqlcChatStore/sqlcAgentStore: course sessions are project-free
// and thread-free, so every method here is keyed by sessionID/userID/courseID,
// mirroring chatstore.go's conventions exactly.
type sqlcCourseStore struct{ q *sqlc.Queries }

// NewSqlcCourseStore adapts sqlc queries to the CourseStore seam.
func NewSqlcCourseStore(q *sqlc.Queries) CourseStore { return &sqlcCourseStore{q: q} }

func (s *sqlcCourseStore) GetSession(ctx context.Context, sessionID uuid.UUID) (CourseSession, error) {
	row, err := s.q.GetCourseSession(ctx, sessionID)
	if err != nil {
		return CourseSession{}, err
	}
	return CourseSession{ID: row.ID, CourseID: row.CourseID, SkillID: row.SkillID, Phase: row.Phase}, nil
}

func (s *sqlcCourseStore) SetSessionPhase(ctx context.Context, sessionID uuid.UUID, phase string) error {
	_, err := s.q.SetCourseSessionPhase(ctx, sqlc.SetCourseSessionPhaseParams{ID: sessionID, Phase: phase})
	return err
}

func (s *sqlcCourseStore) LoadPhaseHistory(ctx context.Context, sessionID uuid.UUID, phase string, limit int) ([]ChatTurn, error) {
	rows, err := s.q.ListMessagesBySession(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	var inPhase []sqlc.CourseMessage
	for _, r := range rows {
		if r.Phase == phase {
			inPhase = append(inPhase, r)
		}
	}
	if limit > 0 && len(inPhase) > limit {
		inPhase = inPhase[len(inPhase)-limit:]
	}
	out := make([]ChatTurn, 0, len(inPhase))
	for _, m := range inPhase {
		out = append(out, ChatTurn{Role: m.Role, Content: m.Content})
	}
	return out, nil
}

func (s *sqlcCourseStore) CountStudentTurns(ctx context.Context, sessionID uuid.UUID, phase string) (int, error) {
	n, err := s.q.CountStudentTurnsInPhase(ctx, sqlc.CountStudentTurnsInPhaseParams{SessionID: sessionID, Phase: phase})
	if err != nil {
		return 0, err
	}
	return int(n), nil
}

func (s *sqlcCourseStore) CreateSessionMessage(ctx context.Context, sessionID uuid.UUID, phase, role, content string) (uuid.UUID, error) {
	m, err := s.q.CreateCourseMessage(ctx, sqlc.CreateCourseMessageParams{
		SessionID: sessionID, Phase: phase, Role: role, Content: content,
	})
	if err != nil {
		return uuid.Nil, err
	}
	return m.ID, nil
}

func (s *sqlcCourseStore) ListSessionCards(ctx context.Context, sessionID uuid.UUID) ([]ScopedCard, error) {
	rows, err := s.q.ListCardInstancesBySession(ctx, pgUUID(sessionID))
	if err != nil {
		return nil, err
	}
	out := make([]ScopedCard, 0, len(rows))
	for _, r := range rows {
		out = append(out, ScopedCard{ID: r.ID, CardID: r.CardID, Status: r.Status})
	}
	return out, nil
}

// CreateSessionCardInstance mints a session-scoped card_instance. card_instances
// has no material_id/tool_id column (verified across every migration through
// 0022) — the material link is carried in the Go CardOffer struct, assembled
// by the runtime after this call, exactly as RunChatStep does for
// CreateThreadCardInstance.
func (s *sqlcCourseStore) CreateSessionCardInstance(ctx context.Context, sessionID uuid.UUID, cardID string) (uuid.UUID, error) {
	ci, err := s.q.CreateSessionCardInstance(ctx, sqlc.CreateSessionCardInstanceParams{
		SessionID: pgUUID(sessionID), CardID: cardID, Status: "proposed",
	})
	if err != nil {
		return uuid.Nil, err
	}
	return ci.ID, nil
}

// CreateSessionMaterial mints the phase's anchor material. Uses kind="article",
// source="pasted" — material.kind/source CHECK constraints (migration 0009)
// only allow ('article','draft') / ('fetched','pasted'); the brief's guessed
// "claim"/"authored" values would violate them. text is segmented into blocks
// the same way a pasted project material is, so the card runtime has content
// to render.
func (s *sqlcCourseStore) CreateSessionMaterial(ctx context.Context, sessionID uuid.UUID, title, text string) (uuid.UUID, error) {
	blocks, err := json.Marshal(materialize.Segment(text))
	if err != nil {
		return uuid.Nil, err
	}
	m, err := s.q.CreateSessionMaterial(ctx, sqlc.CreateSessionMaterialParams{
		SessionID: pgUUID(sessionID), Kind: "article", Source: "pasted", Title: title, Blocks: blocks,
	})
	if err != nil {
		return uuid.Nil, err
	}
	return m.ID, nil
}

func (s *sqlcCourseStore) ListSessionMaterials(ctx context.Context, sessionID uuid.UUID) ([]ScopedMaterial, error) {
	rows, err := s.q.ListMaterialsBySession(ctx, pgUUID(sessionID))
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

// ViewedSteps reads course_progress.completed_ordinals — the existing,
// already-persisted page position. Server-side only (DEC-12.2): a floor the
// client can assert is not a floor.
func (s *sqlcCourseStore) ViewedSteps(ctx context.Context, userID, courseID uuid.UUID) ([]int32, error) {
	p, err := s.q.GetCourseProgress(ctx, sqlc.GetCourseProgressParams{UserID: userID, CourseID: courseID})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			// No progress row yet = nothing viewed, not an error — mirrors
			// getCourseProgress's own zero-progress default for a first-time
			// visitor (internal/api/course.go).
			return nil, nil
		}
		return nil, err
	}
	return p.CompletedOrdinals, nil
}

func (s *sqlcCourseStore) InsertUserEvent(ctx context.Context, userID uuid.UUID, surface, typ string, payload []byte) error {
	_, err := s.q.AppendEvent(ctx, sqlc.AppendEventParams{
		ProjectID: pgtype.UUID{Valid: false}, UserID: userID,
		Surface: surface, Type: typ, Payload: payload,
	})
	return err
}

func (s *sqlcCourseStore) RecordCourseLLMCall(ctx context.Context, userID uuid.UUID, resolved gateway.Resolved, prompt, completion int32) error {
	cost, priced := gateway.EstimateCost(resolved.Provider, resolved.Model, int(prompt), int(completion))
	if !priced {
		slog.Warn("course llm_call: unpriced model — cost recorded as 0", "provider", resolved.Provider, "model", resolved.Model)
	}
	_, err := s.q.RecordLLMCall(ctx, sqlc.RecordLLMCallParams{
		UserID: userID, ProjectID: pgtype.UUID{Valid: false},
		Surface: "course", Purpose: "coach",
		Provider: resolved.Provider, Model: resolved.Model, Tier: resolved.Tier,
		PromptTokens: prompt, CompletionTokens: completion, CostEstimate: gateway.CostNumeric(cost, true),
	})
	return err
}
