package agent

import (
	"context"
	"encoding/json"
	"log/slog"

	"github.com/google/uuid"
	"github.com/riverqueue/river"

	"mindimprint/api/internal/cards"
	"mindimprint/api/internal/gateway"
	"mindimprint/api/internal/store/sqlc"
)

// EvaluateArgs are the river job arguments for an async evaluation. Kept minimal
// (just the ids) so the worker re-reads the live task state at execution time.
type EvaluateArgs struct {
	EvaluationID uuid.UUID `json:"evaluation_id"`
	TaskID       uuid.UUID `json:"task_id"`
}

// Kind uniquely identifies the job type for river. Stable across deploys.
func (EvaluateArgs) Kind() string { return "evaluate" }

// InsertOpts caps retries for ALL eval jobs at 3 attempts (vs river's default of
// 25). Each Work invocation can spend up to two FLAGSHIP calls (runEval's initial
// parse + one parse-retry), and eval is the never-downgrade, most cost-sensitive
// path — so a deterministically-poisoned job (unparseable JSON, malformed task)
// must not burn ~50 flagship calls before it's abandoned. This is a per-job-type
// default applied even when the enqueue call passes nil insertion opts.
func (EvaluateArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{MaxAttempts: 3}
}

// EvaluateWorker is the SOLE eval entrypoint: it owns the queued→running→done/
// failed lifecycle around the shared runEval compute. The flagship model is used
// and never downgraded (Resolver must be the flagship resolver).
type EvaluateWorker struct {
	river.WorkerDefaults[EvaluateArgs]
	Store    EvalLifecycleStore
	Provider gateway.Provider
	Resolver gateway.KeyResolver // FLAGSHIP — never the chaperone resolver
	SpecByID func(string) (cards.Spec, bool)
}

// Work runs one evaluation: MarkRunning → runEval → Finish (or Fail on error).
// Returning an error lets river retry per its policy; a sanitized failure reason
// is persisted (the real cause never leaks into the stored row).
func (w *EvaluateWorker) Work(ctx context.Context, job *river.Job[EvaluateArgs]) error {
	id := job.Args.EvaluationID
	if err := w.Store.MarkRunning(ctx, id); err != nil {
		return err // transient; river retries
	}
	r, err := runEval(ctx, w.Store, w.Provider, w.Resolver, w.SpecByID, job.Args.TaskID)
	if err != nil {
		// Persist the terminal 'failed' state ONLY on the final attempt. Marking
		// it terminal on an earlier attempt would let a later successful retry
		// flip failed→done, leaving a stale error. Until attempts are exhausted
		// the row stays 'running' between retries (MarkRunning is a no-op once
		// running). Always return the error so river keeps retrying.
		if job.Attempt >= job.MaxAttempts {
			// Sanitize: store a generic reason; the real cause stays in the returned error.
			_ = w.Store.Fail(ctx, id, "评估执行失败")
		}
		return err
	}
	scoresJSON, _ := json.Marshal(r.Out.Scores)
	sigJSON, _ := json.Marshal(r.Signals)
	pt, ct := int32(r.Res.Usage.InputTokens), int32(r.Res.Usage.OutputTokens)
	cost, ok := gateway.EstimateCost(r.Resolved.Provider, r.Resolved.Model, r.Res.Usage.InputTokens, r.Res.Usage.OutputTokens)
	rv := RubricVersion
	if err := w.Store.Finish(ctx, sqlc.FinishEvaluationParams{
		ID:               id,
		Scores:           scoresJSON,
		Narrative:        r.Out.Narrative,
		Signals:          sigJSON,
		RubricVersion:    &rv,
		Model:            r.Resolved.Model,
		Tier:             r.Resolved.Tier,
		PromptTokens:     &pt,
		CompletionTokens: &ct,
		CostEstimate:     gateway.CostNumeric(cost, ok),
	}); err != nil {
		return err
	}
	// Best-effort: flip the task to 'evaluated' now that a result exists. A failure
	// here must NOT re-run the never-downgrade flagship eval, so swallow + log.
	if err := w.Store.MarkTaskEvaluated(ctx, job.Args.TaskID); err != nil {
		slog.Warn("eval: mark task evaluated failed", "task_id", job.Args.TaskID.String(), "err", err.Error())
	}
	return nil
}
