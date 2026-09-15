package api

// lite_grading_jobs.go — the river job behind 一键AI批改. One job per grading
// row. MaxAttempts is 1 so river never runs (and bills) a job twice; the one
// retry happens inside gradeWithRetry.

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/riverqueue/river"

	"mindimprint/api/internal/gateway"
	"mindimprint/api/internal/httpx"
	"mindimprint/api/internal/liteassign"
	"mindimprint/api/internal/litegrade"
	"mindimprint/api/internal/store/sqlc"
)

const (
	liteGradingQueue = "lite_grading"
	// Two calls of up to 150s each, plus the database work. River's default
	// job timeout (1 minute) would cancel the first call.
	liteGradingJobTimeout = 6 * time.Minute
)

type LiteGradingArgs struct {
	GradingID uuid.UUID `json:"grading_id"`
}

func (LiteGradingArgs) Kind() string { return "lite_grading" }

func (LiteGradingArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{MaxAttempts: 1, Queue: liteGradingQueue}
}

type LiteGradingWorker struct {
	river.WorkerDefaults[LiteGradingArgs]
	API *API
}

func (w *LiteGradingWorker) Timeout(*river.Job[LiteGradingArgs]) time.Duration {
	return liteGradingJobTimeout
}

// Work always reports success: the outcome, including a failure, is written
// on the grading row, and river must not retry.
func (w *LiteGradingWorker) Work(ctx context.Context, job *river.Job[LiteGradingArgs]) error {
	w.API.runLiteGrading(ctx, job.Args.GradingID)
	return nil
}

func RegisterLiteGradingWorker(w *river.Workers, a *API) error {
	return river.AddWorkerSafely(w, &LiteGradingWorker{API: a})
}

// failLiteGrading writes the outcome of a failed attempt: a first grading
// with no prior content ends up failed, a failed regrade returns to draft
// with its previous ai/content kept (SetLiteGradingFailed's CASE, Task 3) —
// either way the teacher reads msg after 「批改失败：」.
//
// Uses context.WithoutCancel: this is the terminal write for a row already
// marked running. If the caller's ctx is cancelled or timed out right here
// (a job timeout, a server shutdown), the row must still land on failed/draft
// instead of being stuck on running forever.
func (a *API) failLiteGrading(ctx context.Context, id uuid.UUID, msg string) {
	if _, err := a.d.Queries.SetLiteGradingFailed(context.WithoutCancel(ctx), sqlc.SetLiteGradingFailedParams{ID: id, Error: &msg}); err != nil {
		slog.Warn("lite grading: mark failed", "err", err, "grading_id", id)
	}
}

// runLiteGrading claims a queued row, grades its version and writes the
// result: draft with ai = content, or failed with the reasons. A row that is
// no longer queued (regraded, or already claimed) is left alone.
func (a *API) runLiteGrading(ctx context.Context, id uuid.UUID) {
	g, err := a.d.Queries.ClaimLiteGrading(ctx, id)
	if err != nil {
		if !errors.Is(err, pgx.ErrNoRows) {
			slog.Warn("lite grading: claim", "err", err, "grading_id", id)
		}
		return
	}
	entitled, err := a.studentEntitled(ctx, g.UserID)
	if err != nil {
		slog.Warn("lite grading: entitlement", "err", err, "grading_id", id)
		a.failLiteGrading(ctx, id, "服务器内部错误")
		return
	}
	if !entitled {
		a.failLiteGrading(ctx, id, httpx.ErrNotEntitled().Message)
		return
	}
	src, err := a.d.Queries.GetLiteGradingSource(ctx, g.VersionID)
	if err != nil {
		slog.Warn("lite grading: load version", "err", err, "grading_id", id)
		a.failLiteGrading(ctx, id, "服务器内部错误")
		return
	}
	var rubric liteassign.Rubric
	if err := json.Unmarshal(g.Rubric, &rubric); err != nil {
		slog.Warn("lite grading: rubric", "err", err, "grading_id", id)
		a.failLiteGrading(ctx, id, "服务器内部错误")
		return
	}
	resolved, err := a.routeE(ctx, gateway.ClassReview)
	if err != nil {
		a.failLiteGrading(ctx, id, "模型不可用："+err.Error())
		return
	}
	// context.WithoutCancel here too: a billed call must be metered even if
	// ctx is cancelled the instant the model replies — same reasoning as the
	// final write below, and llm_call is itself a terminal record of money
	// already spent, not something a cancelled ctx should get to drop.
	content, reasons, attempts := gradeWithRetry(ctx, a.d.Provider, resolved, liteGradingInput(src, rubric), func(u gateway.ChatUsage) {
		a.recordLiteLLMCall(context.WithoutCancel(ctx), g.UserID, g.AtomID, liteGradingPurpose, resolved, u)
	})
	if len(reasons) > 0 {
		slog.Info("lite grading: failed", "grading_id", id, "attempts", attempts, "reasons", litegrade.JoinReasons(reasons))
		a.failLiteGrading(ctx, id, litegrade.JoinReasons(reasons))
		return
	}
	result, err := json.Marshal(content)
	if err != nil {
		a.failLiteGrading(ctx, id, "服务器内部错误")
		return
	}
	// context.WithoutCancel: the same reasoning as failLiteGrading above — the
	// row is already running, a successful grading must not be lost to a
	// cancelled ctx.
	if _, err := a.d.Queries.SetLiteGradingDraft(context.WithoutCancel(ctx), sqlc.SetLiteGradingDraftParams{ID: id, Result: result}); err != nil {
		slog.Warn("lite grading: store draft", "err", err, "grading_id", id)
	}
}
