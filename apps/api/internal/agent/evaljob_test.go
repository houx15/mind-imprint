package agent

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/riverqueue/river"

	"mindimprint/api/internal/cards"
	"mindimprint/api/internal/gateway"
	"mindimprint/api/internal/store/sqlc"
)

// fakeLifecycleStore is an in-memory EvalLifecycleStore that records the
// lifecycle calls so worker tests can assert the queued→running→done/failed
// flow without a real database (mirrors eval_test.go's fake-store style).
type fakeLifecycleStore struct {
	msgs     []StoredMessage
	cards    []CardInstance
	ranID    uuid.UUID
	finished *sqlc.FinishEvaluationParams
	failedID uuid.UUID
	failMsg  string
}

func (f *fakeLifecycleStore) EvalMessages(context.Context, uuid.UUID) ([]StoredMessage, error) {
	return f.msgs, nil
}
func (f *fakeLifecycleStore) EvalCards(context.Context, uuid.UUID) ([]CardInstance, error) {
	return f.cards, nil
}
func (f *fakeLifecycleStore) MarkRunning(_ context.Context, id uuid.UUID) error {
	f.ranID = id
	return nil
}
func (f *fakeLifecycleStore) Finish(_ context.Context, p sqlc.FinishEvaluationParams) error {
	f.finished = &p
	return nil
}
func (f *fakeLifecycleStore) Fail(_ context.Context, id uuid.UUID, msg string) error {
	f.failedID = id
	f.failMsg = msg
	return nil
}

const goodEval = `{"scores":[{"dim_id":"D1","level":"L3","note":"n"}],"narrative":"你的思维印记"}`

func flagshipResolver(model string) gateway.KeyResolver {
	return func(context.Context) (gateway.Resolved, error) {
		return gateway.Resolved{Provider: "deepseek", Model: model, Tier: "flagship"}, nil
	}
}

func noSpec(string) (cards.Spec, bool) { return cards.Spec{}, false }

// TestEvaluateWorker_Work_FinishesDone is the happy path: MarkRunning → runEval →
// Finish, with the flagship model/tier, usage, signals and rubric_version stamped.
// (Replaces the old TestRunEvaluationPersists, now that the worker is the only
// eval entrypoint.)
func TestEvaluateWorker_Work_FinishesDone(t *testing.T) {
	store := &fakeLifecycleStore{msgs: []StoredMessage{{Role: "user", Content: "hi"}}}
	prov := gateway.NewStubProvider([]gateway.StreamEvent{
		{Kind: gateway.EventTextDelta, TextDelta: goodEval},
		{Kind: gateway.EventUsage, Usage: &gateway.ChatUsage{InputTokens: 100, OutputTokens: 200}},
		{Kind: gateway.EventDone},
	})
	w := &EvaluateWorker{
		Store:    store,
		Provider: prov,
		Resolver: flagshipResolver("deepseek-reasoner"),
		SpecByID: noSpec,
	}
	evalID, taskID := uuid.New(), uuid.New()
	job := &river.Job[EvaluateArgs]{Args: EvaluateArgs{EvaluationID: evalID, TaskID: taskID}}

	if err := w.Work(context.Background(), job); err != nil {
		t.Fatalf("work: %v", err)
	}
	if store.ranID != evalID {
		t.Fatalf("MarkRunning not called with eval id: got %v", store.ranID)
	}
	if store.finished == nil {
		t.Fatal("Finish not called")
	}
	p := store.finished
	if p.ID != evalID {
		t.Fatalf("Finish id = %v, want %v", p.ID, evalID)
	}
	if p.Narrative != "你的思维印记" {
		t.Fatalf("narrative = %q", p.Narrative)
	}
	if p.Tier != "flagship" || p.Model != "deepseek-reasoner" {
		t.Fatalf("flagship not persisted: model=%q tier=%q", p.Model, p.Tier)
	}
	if p.PromptTokens == nil || *p.PromptTokens != 100 || p.CompletionTokens == nil || *p.CompletionTokens != 200 {
		t.Fatalf("usage not persisted: %+v / %+v", p.PromptTokens, p.CompletionTokens)
	}
	if p.RubricVersion == nil || *p.RubricVersion != "cognitive-model-v2" {
		t.Fatalf("rubric_version not stamped: %+v", p.RubricVersion)
	}
	if len(p.Signals) == 0 {
		t.Error("signals not persisted")
	}
	if store.finished != nil && store.failMsg != "" {
		t.Error("Fail must not be called on the done path")
	}
}

// TestEvaluateWorker_Work_FailsOnParseError exercises the retry-then-fail path:
// the stub replays unparseable text on every Collect, so both attempts fail; the
// worker must call Fail with a sanitized reason and return a non-nil error (so
// river can retry). (Replaces the old TestRunEvaluationRetriesThenFails.)
func TestEvaluateWorker_Work_FailsOnParseError(t *testing.T) {
	store := &fakeLifecycleStore{}
	prov := gateway.NewStubProvider([]gateway.StreamEvent{
		{Kind: gateway.EventTextDelta, TextDelta: "not json"},
		{Kind: gateway.EventDone},
	}) // StubProvider replays the same script each Stream call → both attempts fail
	w := &EvaluateWorker{
		Store:    store,
		Provider: prov,
		Resolver: flagshipResolver("deepseek-reasoner"),
		SpecByID: noSpec,
	}
	evalID := uuid.New()
	job := &river.Job[EvaluateArgs]{Args: EvaluateArgs{EvaluationID: evalID, TaskID: uuid.New()}}

	err := w.Work(context.Background(), job)
	if err == nil {
		t.Fatal("want parse-failure error after retry")
	}
	if store.failedID != evalID {
		t.Fatalf("Fail not called with eval id: got %v", store.failedID)
	}
	if store.failMsg == "" {
		t.Error("Fail must persist a sanitized reason")
	}
	if store.finished != nil {
		t.Error("Finish must not be called on the failure path")
	}
}

func TestEvaluateArgsKind(t *testing.T) {
	if (EvaluateArgs{}).Kind() != "evaluate" {
		t.Fatalf("kind = %q", (EvaluateArgs{}).Kind())
	}
}
