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

// LiteGradingArgs carries the rubric to grade with. A regrade of a row that
// already has content does not change the row's rubric at queue time
// (RequeueLiteGrading): if the regrade fails, the row returns to draft with
// its previous content, and that content must still match the row's rubric.
// The worker grades with Rubric and writes it with the new content. A job
// queued without Rubric (before this field existed) grades with the row's.
type LiteGradingArgs struct {
	GradingID uuid.UUID       `json:"grading_id"`
	Rubric    json.RawMessage `json:"rubric,omitempty"`
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
	w.API.runLiteGrading(ctx, job.Args)
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

// liteGradingReplyLogRunes caps the model reply written to the server log on
// a final failure.
const liteGradingReplyLogRunes = 2000

// runLiteGrading claims a queued row, grades its version and writes the
// result: draft with ai = content and the rubric it was graded with, or
// failed with the reasons. A row that is no longer queued (regraded, or
// already claimed) is left alone.
func (a *API) runLiteGrading(ctx context.Context, args LiteGradingArgs) {
	id := args.GradingID
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
	rubricJSON := []byte(args.Rubric)
	if len(rubricJSON) == 0 {
		rubricJSON = g.Rubric
	}
	var rubric liteassign.Rubric
	if err := json.Unmarshal(rubricJSON, &rubric); err != nil {
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
	out := gradeWithRetry(ctx, a.d.Provider, resolved, liteGradingInput(src, rubric), func(u gateway.ChatUsage) {
		a.recordLiteLLMCall(context.WithoutCancel(ctx), g.UserID, g.AtomID, liteGradingPurpose, resolved, u)
	})
	if len(out.Reasons) > 0 {
		// The reply and the parse error go to the server log only, so a
		// failure seen online can be diagnosed. Neither contains a secret:
		// the reply is model output, the error is encoding/json's message.
		parseErr := ""
		if out.ParseErr != nil {
			parseErr = out.ParseErr.Error()
		}
		slog.Warn("lite grading: failed", "grading_id", id, "attempts", out.Attempts,
			"reasons", litegrade.JoinReasons(out.Reasons), "parse_err", parseErr,
			"reply", truncateRunes(out.LastReply, liteGradingReplyLogRunes))
		a.failLiteGrading(ctx, id, litegrade.JoinReasons(out.Reasons))
		return
	}
	result, err := json.Marshal(out.Content)
	if err != nil {
		a.failLiteGrading(ctx, id, "服务器内部错误")
		return
	}
	// context.WithoutCancel: the same reasoning as failLiteGrading above — the
	// row is already running, a successful grading must not be lost to a
	// cancelled ctx.
	if _, err := a.d.Queries.SetLiteGradingDraft(context.WithoutCancel(ctx), sqlc.SetLiteGradingDraftParams{ID: id, Result: result, Rubric: rubricJSON}); err != nil {
		slog.Warn("lite grading: store draft", "err", err, "grading_id", id)
	}
}
