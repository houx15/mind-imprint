package agent

import (
	"context"
	"encoding/json"

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
		// Sanitize: store a generic reason; the real cause stays in the returned error.
		_ = w.Store.Fail(ctx, id, "评估执行失败")
		return err
	}
	scoresJSON, _ := json.Marshal(r.Out.Scores)
	sigJSON, _ := json.Marshal(r.Signals)
	pt, ct := int32(r.Res.Usage.InputTokens), int32(r.Res.Usage.OutputTokens)
	cost, ok := gateway.EstimateCost(r.Resolved.Provider, r.Resolved.Model, r.Res.Usage.InputTokens, r.Res.Usage.OutputTokens)
	rv := RubricVersion
	return w.Store.Finish(ctx, sqlc.FinishEvaluationParams{
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
	})
}
