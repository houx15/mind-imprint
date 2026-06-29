package api

import (
	"context"
	"errors"
	"log/slog"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"mindimprint/api/internal/agent"
	"mindimprint/api/internal/store/sqlc"
)

const (
	substantiveTurnsPerMilestone = 6
	maxAutoEvals                 = 3
)

// MilestoneIndex collapses task progress into one monotonic integer: one per
// completed card plus one per N substantive student turns.
func MilestoneIndex(completedCards, substantiveTurns int64) int32 {
	return int32(completedCards + substantiveTurns/substantiveTurnsPerMilestone)
}

// maybeTriggerMilestoneEval best-effort enqueues an async evaluation when the
// task crosses a new milestone, subject to the cap/debounce/in-flight guards in
// TryEnqueueMilestoneEvaluation. It NEVER blocks (beyond a few cheap queries),
// aborts, or fails the caller's request — every error is logged and swallowed.
func (a *API) maybeTriggerMilestoneEval(ctx context.Context, taskID uuid.UUID) {
	if a.d.Enqueuer == nil {
		return // no queue wired (e.g. tests that omit Enqueuer); skip silently
	}
	cards, err := a.d.Queries.CountCompletedCards(ctx, taskID)
	if err != nil {
		slog.Warn("milestone: count cards failed", "task_id", taskID.String(), "err", err.Error())
		return
	}
	turns, err := a.d.Queries.CountSubstantiveTurns(ctx, taskID)
	if err != nil {
		slog.Warn("milestone: count turns failed", "task_id", taskID.String(), "err", err.Error())
		return
	}
	m := MilestoneIndex(cards, turns)
	if m < 1 {
		return // no milestone reached yet
	}
	ev, err := a.d.Queries.TryEnqueueMilestoneEvaluation(ctx, sqlc.TryEnqueueMilestoneEvaluationParams{
		TaskID:    taskID,
		Milestone: m,
		Cap:       maxAutoEvals,
	})
	if err != nil {
		// ErrNoRows = guards rejected the trigger; 23505 = a concurrent trigger
		// won the one-in-flight race. Both are normal "do nothing" outcomes.
		if errors.Is(err, pgx.ErrNoRows) || isUniqueViolation(err) {
			return
		}
		slog.Warn("milestone: try-enqueue failed", "task_id", taskID.String(), "err", err.Error())
		return
	}
	if err := a.d.Enqueuer.EnqueueEvaluate(ctx, agent.EvaluateArgs{EvaluationID: ev.ID, TaskID: taskID}); err != nil {
		// Best-effort: mark the row failed so nothing polls a job that never queued.
		_ = a.d.Queries.FailEvaluation(ctx, sqlc.FailEvaluationParams{ID: ev.ID, Error: ptrStr("入队失败")})
		slog.Error("milestone: enqueue failed", "task_id", taskID.String(), "err", err.Error())
	}
}

// isUniqueViolation is defined in signup.go (same package); used here and by
// the manual postEvaluate idempotency path (Task 6).
