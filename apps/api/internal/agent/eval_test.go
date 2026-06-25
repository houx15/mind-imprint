package agent

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"mindimprint/api/internal/cards"
	"mindimprint/api/internal/gateway"
	"mindimprint/api/internal/store/sqlc"
)

type fakeEvalStore struct {
	msgs    []StoredMessage
	cards   []CardInstance
	created sqlc.CreateEvaluationParams
}

func (f *fakeEvalStore) EvalMessages(context.Context, uuid.UUID) ([]StoredMessage, error) {
	return f.msgs, nil
}
func (f *fakeEvalStore) EvalCards(context.Context, uuid.UUID) ([]CardInstance, error) {
	return f.cards, nil
}
func (f *fakeEvalStore) CreateEvaluation(_ context.Context, p sqlc.CreateEvaluationParams) (sqlc.Evaluation, error) {
	f.created = p
	return sqlc.Evaluation{ID: uuid.New(), TaskID: p.TaskID, Narrative: p.Narrative, Model: p.Model, Tier: p.Tier, Status: "done"}, nil
}

const goodEval = `{"scores":[{"dim_id":"D1","level":"L3","note":"n"}],"narrative":"你的思维印记"}`

func resolverFor(model string) gateway.KeyResolver {
	return func(context.Context) (gateway.Resolved, error) {
		return gateway.Resolved{Provider: "deepseek", Model: model, Tier: "flagship"}, nil
	}
}

func TestRunEvaluationPersists(t *testing.T) {
	store := &fakeEvalStore{msgs: []StoredMessage{{Role: "user", Content: "hi"}}}
	prov := gateway.NewStubProvider([]gateway.StreamEvent{
		{Kind: gateway.EventTextDelta, TextDelta: goodEval},
		{Kind: gateway.EventUsage, Usage: &gateway.ChatUsage{InputTokens: 100, OutputTokens: 200}},
		{Kind: gateway.EventDone},
	})
	ev, err := RunEvaluation(context.Background(), EvalDeps{
		Store: store, Provider: prov, Resolver: resolverFor("deepseek-reasoner"),
		SpecByID: func(string) (cards.Spec, bool) { return cards.Spec{}, false }, TaskID: uuid.New(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if ev.Status != "done" || ev.Narrative != "你的思维印记" {
		t.Fatalf("row %+v", ev)
	}
	if store.created.Tier != "flagship" || store.created.Model != "deepseek-reasoner" {
		t.Fatalf("flagship not persisted: %+v", store.created)
	}
	if store.created.PromptTokens == nil || *store.created.PromptTokens != 100 {
		t.Fatal("usage not persisted")
	}
}

func TestRunEvaluationRetriesThenFails(t *testing.T) {
	store := &fakeEvalStore{}
	prov := gateway.NewStubProvider([]gateway.StreamEvent{
		{Kind: gateway.EventTextDelta, TextDelta: "not json"}, {Kind: gateway.EventDone},
	}) // StubProvider replays the same script each Stream call → both attempts fail
	_, err := RunEvaluation(context.Background(), EvalDeps{
		Store: store, Provider: prov, Resolver: resolverFor("deepseek-reasoner"),
		SpecByID: func(string) (cards.Spec, bool) { return cards.Spec{}, false }, TaskID: uuid.New(),
	})
	if err == nil {
		t.Fatal("want parse-failure error after retry")
	}
}
