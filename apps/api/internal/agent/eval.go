package agent

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"github.com/google/uuid"

	"mindimprint/api/internal/cards"
	"mindimprint/api/internal/gateway"
	"mindimprint/api/internal/store/sqlc"
)

// DimScore mirrors the TS DimScore.
type DimScore struct {
	DimID string `json:"dim_id"`
	Level string `json:"level"`
	Note  string `json:"note"`
}

// EvalLlmOutput mirrors the TS EvalLlmOutput (the model's JSON contract).
type EvalLlmOutput struct {
	Scores    []DimScore `json:"scores"`
	Narrative string     `json:"narrative"`
}

var validLevels = map[string]bool{"L1": true, "L2": true, "L3": true, "L4": true, "NA": true}

// parseEvalOutput strips optional ```json fences then JSON-parses + validates,
// mirroring the TS parseEvalOutput.
func parseEvalOutput(text string) (EvalLlmOutput, error) {
	cleaned := strings.TrimSpace(text)
	switch {
	case strings.HasPrefix(cleaned, "```json"):
		cleaned = strings.TrimLeft(strings.TrimPrefix(cleaned, "```json"), " \t\r\n")
	case strings.HasPrefix(cleaned, "```"):
		cleaned = strings.TrimLeft(cleaned[3:], " \t\r\n")
	}
	if strings.HasSuffix(cleaned, "```") {
		cleaned = strings.TrimRight(cleaned[:len(cleaned)-3], " \t\r\n")
	}
	var out EvalLlmOutput
	if err := json.Unmarshal([]byte(cleaned), &out); err != nil {
		return EvalLlmOutput{}, err
	}
	if len(out.Scores) == 0 {
		return EvalLlmOutput{}, errors.New("empty scores")
	}
	for _, s := range out.Scores {
		if !validLevels[s.Level] {
			return EvalLlmOutput{}, errors.New("invalid SOLO level: " + s.Level)
		}
	}
	return out, nil
}

// EvalStore is the read seam shared by the eval compute and the worker.
type EvalStore interface {
	EvalMessages(ctx context.Context, taskID uuid.UUID) ([]StoredMessage, error)
	EvalCards(ctx context.Context, taskID uuid.UUID) ([]CardInstance, error)
	CreateEvaluation(ctx context.Context, p sqlc.CreateEvaluationParams) (sqlc.Evaluation, error)
}

// EvalLifecycleStore is the persistence seam the river EvaluateWorker drives:
// reads (messages/cards) plus the queued→running→done/failed lifecycle.
type EvalLifecycleStore interface {
	EvalMessages(ctx context.Context, taskID uuid.UUID) ([]StoredMessage, error)
	EvalCards(ctx context.Context, taskID uuid.UUID) ([]CardInstance, error)
	MarkRunning(ctx context.Context, evalID uuid.UUID) error
	Finish(ctx context.Context, p sqlc.FinishEvaluationParams) error
	Fail(ctx context.Context, evalID uuid.UUID, msg string) error
	MarkTaskEvaluated(ctx context.Context, taskID uuid.UUID) error
}

// runEvalResult bundles the shared eval compute outputs so the worker can
// persist them (and so failures short-circuit before any write).
type runEvalResult struct {
	Out      EvalLlmOutput
	Signals  EvalSignals
	Res      gateway.ChatResult
	Resolved gateway.Resolved
}

// runEval is the shared eval compute: signals → prompt → flagship Collect →
// parse-with-one-retry. It does not touch the lifecycle (MarkRunning/Finish/
// Fail) — the worker wraps that around this. Ports the TS runEvaluation compute.
func runEval(ctx context.Context, store EvalLifecycleStore, provider gateway.Provider,
	resolver gateway.KeyResolver, specByID func(string) (cards.Spec, bool), taskID uuid.UUID) (runEvalResult, error) {

	msgs, err := store.EvalMessages(ctx, taskID)
	if err != nil {
		return runEvalResult{}, err
	}
	cardInsts, err := store.EvalCards(ctx, taskID)
	if err != nil {
		return runEvalResult{}, err
	}

	sig := ComputeSignals(msgs, cardInsts)
	system := BuildEvalPrompt(FullRubric)
	user := BuildEvalUserInput(msgs, cardInsts, sig, specByID)

	resolved, err := resolver(ctx)
	if err != nil {
		return runEvalResult{}, err
	}
	req := gateway.ChatRequest{
		Messages: []gateway.ChatMessage{
			{Role: gateway.RoleSystem, Content: system},
			{Role: gateway.RoleUser, Content: user},
		},
		// Headroom for the 10-dim JSON + narrative; reasoning models spend
		// completion tokens on hidden reasoning before the JSON (matches TS 8000).
		MaxTokens: 8000,
	}

	res, err := gateway.Collect(ctx, provider, resolved, req)
	if err != nil {
		return runEvalResult{}, err
	}
	out, perr := parseEvalOutput(res.Text)
	if perr != nil {
		res, err = gateway.Collect(ctx, provider, resolved, req) // retry once
		if err != nil {
			return runEvalResult{}, err
		}
		if out, perr = parseEvalOutput(res.Text); perr != nil {
			return runEvalResult{}, errors.New("评估输出解析失败")
		}
	}
	return runEvalResult{Out: out, Signals: sig, Res: res, Resolved: resolved}, nil
}

// --- sqlc adapter ----------------------------------------------------------

type sqlcEvalStore struct{ q *sqlc.Queries }

// NewSqlcEvalStore adapts sqlc queries to the eval seams. The returned
// *sqlcEvalStore satisfies both EvalStore (reads) and EvalLifecycleStore
// (reads + queued→running→done/failed lifecycle).
func NewSqlcEvalStore(q *sqlc.Queries) *sqlcEvalStore { return &sqlcEvalStore{q: q} }

func (s *sqlcEvalStore) MarkRunning(ctx context.Context, evalID uuid.UUID) error {
	return s.q.MarkEvaluationRunning(ctx, evalID)
}

func (s *sqlcEvalStore) Finish(ctx context.Context, p sqlc.FinishEvaluationParams) error {
	return s.q.FinishEvaluation(ctx, p)
}

func (s *sqlcEvalStore) Fail(ctx context.Context, evalID uuid.UUID, msg string) error {
	return s.q.FailEvaluation(ctx, sqlc.FailEvaluationParams{ID: evalID, Error: &msg})
}

func (s *sqlcEvalStore) MarkTaskEvaluated(ctx context.Context, taskID uuid.UUID) error {
	return s.q.MarkTaskEvaluated(ctx, taskID)
}

func (s *sqlcEvalStore) EvalMessages(ctx context.Context, taskID uuid.UUID) ([]StoredMessage, error) {
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

func (s *sqlcEvalStore) EvalCards(ctx context.Context, taskID uuid.UUID) ([]CardInstance, error) {
	rows, err := s.q.ListCardsByTask(ctx, taskID)
	if err != nil {
		return nil, err
	}
	out := make([]CardInstance, 0, len(rows))
	for _, r := range rows {
		ci := CardInstance{ID: r.ID.String(), CardID: r.CardID, TaskID: r.TaskID.String(), Status: r.Status}
		if len(r.FieldValues) > 0 {
			_ = json.Unmarshal(r.FieldValues, &ci.FieldValues)
		}
		if len(r.Anchors) > 0 {
			_ = json.Unmarshal(r.Anchors, &ci.Anchors)
		}
		// event_trace length only (TS card.event_trace.length).
		if len(r.EventTrace) > 0 {
			var arr []json.RawMessage
			if json.Unmarshal(r.EventTrace, &arr) == nil {
				ci.EventTraceLen = len(arr)
			}
		}
		out = append(out, ci)
	}
	return out, nil
}

func (s *sqlcEvalStore) CreateEvaluation(ctx context.Context, p sqlc.CreateEvaluationParams) (sqlc.Evaluation, error) {
	return s.q.CreateEvaluation(ctx, p)
}
