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

var validLevels = map[string]bool{"L1": true, "L2": true, "L3": true, "L4": true}

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

// EvalStore is the persistence seam RunEvaluation drives.
type EvalStore interface {
	EvalMessages(ctx context.Context, taskID uuid.UUID) ([]StoredMessage, error)
	EvalCards(ctx context.Context, taskID uuid.UUID) ([]CardInstance, error)
	CreateEvaluation(ctx context.Context, p sqlc.CreateEvaluationParams) (sqlc.Evaluation, error)
}

// EvalDeps are the inputs to RunEvaluation.
type EvalDeps struct {
	Store    EvalStore
	Provider gateway.Provider
	Resolver gateway.KeyResolver // FLAGSHIP resolver; never the chaperone one
	SpecByID func(id string) (cards.Spec, bool)
	TaskID   uuid.UUID
}

// RunEvaluation assembles the eval input, calls the flagship model, parses with
// one retry, and persists the evaluation row. Ports the TS runEvaluation.
func RunEvaluation(ctx context.Context, deps EvalDeps) (sqlc.Evaluation, error) {
	msgs, err := deps.Store.EvalMessages(ctx, deps.TaskID)
	if err != nil {
		return sqlc.Evaluation{}, err
	}
	cardInsts, err := deps.Store.EvalCards(ctx, deps.TaskID)
	if err != nil {
		return sqlc.Evaluation{}, err
	}
	system := BuildEvalPrompt(FullRubric)
	user := AssembleEvalInput(msgs, cardInsts, deps.SpecByID)

	resolved, err := deps.Resolver(ctx)
	if err != nil {
		return sqlc.Evaluation{}, err
	}
	req := gateway.ChatRequest{
		Messages: []gateway.ChatMessage{
			{Role: gateway.RoleSystem, Content: system},
			{Role: gateway.RoleUser, Content: user},
		},
		// Headroom for the 9-dim JSON + narrative; reasoning models spend
		// completion tokens on hidden reasoning before the JSON (matches TS 8000).
		MaxTokens: 8000,
	}

	res, err := gateway.Collect(ctx, deps.Provider, resolved, req)
	if err != nil {
		return sqlc.Evaluation{}, err
	}
	out, perr := parseEvalOutput(res.Text)
	if perr != nil {
		res, err = gateway.Collect(ctx, deps.Provider, resolved, req) // retry once
		if err != nil {
			return sqlc.Evaluation{}, err
		}
		out, perr = parseEvalOutput(res.Text)
		if perr != nil {
			return sqlc.Evaluation{}, errors.New("评估输出解析失败")
		}
	}

	scoresJSON, err := json.Marshal(out.Scores)
	if err != nil {
		return sqlc.Evaluation{}, err
	}
	pt, ct := int32(res.Usage.InputTokens), int32(res.Usage.OutputTokens)
	cost, ok := gateway.EstimateCost(resolved.Provider, resolved.Model, res.Usage.InputTokens, res.Usage.OutputTokens)
	return deps.Store.CreateEvaluation(ctx, sqlc.CreateEvaluationParams{
		TaskID:           deps.TaskID,
		Scores:           scoresJSON,
		Narrative:        out.Narrative,
		Model:            resolved.Model,
		Tier:             resolved.Tier,
		PromptTokens:     &pt,
		CompletionTokens: &ct,
		CostEstimate:     gateway.CostNumeric(cost, ok),
	})
}

// --- sqlc adapter ----------------------------------------------------------

type sqlcEvalStore struct{ q *sqlc.Queries }

// NewSqlcEvalStore adapts sqlc queries to EvalStore.
func NewSqlcEvalStore(q *sqlc.Queries) EvalStore { return &sqlcEvalStore{q: q} }

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
