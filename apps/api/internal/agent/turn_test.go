package agent_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"mindimprint/api/internal/agent"
	"mindimprint/api/internal/cards"
	"mindimprint/api/internal/gateway"
	"mindimprint/api/internal/store"
	"mindimprint/api/internal/store/sqlc"
)

// fakeTurnStore is a lightweight in-memory TurnStore for unit tests that do
// not need a real database. It records the number of AppendUserMessage calls
// so tests can assert continuation turns skip that step.
type fakeTurnStore struct {
	userAppends    int
	history        []agent.StoredMessage
	materials      []agent.Material
	anchorsWritten []byte
}

func (f *fakeTurnStore) AppendUserMessage(_ context.Context, _ uuid.UUID, _ string) (uuid.UUID, error) {
	f.userAppends++
	return uuid.New(), nil
}

func (f *fakeTurnStore) ListMessages(_ context.Context, _ uuid.UUID) ([]agent.StoredMessage, error) {
	return f.history, nil
}

func (f *fakeTurnStore) CardByID(_ context.Context, _ string) (agent.CardInstance, bool, error) {
	return agent.CardInstance{}, false, nil
}

func (f *fakeTurnStore) CreateProposedCard(_ context.Context, _ uuid.UUID, _ string) (uuid.UUID, error) {
	return uuid.New(), nil
}

func (f *fakeTurnStore) AppendAssistantMessage(_ context.Context, _ agent.AssistantMessage) (uuid.UUID, error) {
	return uuid.New(), nil
}

func (f *fakeTurnStore) ListMaterials(_ context.Context, _ uuid.UUID) ([]agent.Material, error) {
	return f.materials, nil
}
func (f *fakeTurnStore) SetCardAnchors(_ context.Context, _, _ uuid.UUID, anchors []byte) error {
	f.anchorsWritten = anchors
	return nil
}

func TestRunTurnContinuationAppendsNoUserMessage(t *testing.T) {
	store := &fakeTurnStore{
		history: []agent.StoredMessage{
			{Role: "user", Content: "hello"},
		},
	}
	prov := gateway.NewStubProvider([]gateway.StreamEvent{
		{Kind: gateway.EventTextDelta, TextDelta: "基于你刚填的卡，我们继续。"},
		{Kind: gateway.EventUsage, Usage: &gateway.ChatUsage{InputTokens: 5, OutputTokens: 7}},
		{Kind: gateway.EventDone, StopReason: gateway.StopStop},
	})
	sse := &fakeSSE{}
	err := agent.RunTurn(context.Background(), agent.TurnDeps{
		Store:    store,
		Provider: prov,
		KeyResolver: func(context.Context) (gateway.Resolved, error) {
			return gateway.Resolved{Provider: "deepseek", Model: "deepseek-chat", Tier: "chaperone"}, nil
		},
		Catalog:  nil,
		SpecByID: func(string) (cards.Spec, bool) { return cards.Spec{}, false },
		SSE:      sse,
	}, uuid.New(), "") // empty userInput = continuation turn
	if err != nil {
		t.Fatal(err)
	}
	if store.userAppends != 0 {
		t.Fatalf("continuation turn must not append a user message, got %d", store.userAppends)
	}
	// assistant reply + done still emitted
	if len(sse.texts) == 0 {
		t.Fatal("expected assistant text delta, got none")
	}
	if sse.done == "" {
		t.Fatal("expected done event, got none")
	}
}

var seededStudentID = uuid.MustParse("00000000-0000-0000-0000-000000000003")

// fakeSSE records emitted events for assertions.
type fakeSSE struct {
	texts []string
	card  *struct{ ci, cardID, nudge string }
	done  string
}

func (f *fakeSSE) Text(delta string) error { f.texts = append(f.texts, delta); return nil }
func (f *fakeSSE) Card(ci, cardID, nudge string) error {
	f.card = &struct{ ci, cardID, nudge string }{ci, cardID, nudge}
	return nil
}
func (f *fakeSSE) Done(messageID string) error { f.done = messageID; return nil }

func TestRunTurnProposesCardAndPersists(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping testcontainers integration in -short mode")
	}
	ctx := context.Background()
	pool := newTurnTestPool(t)
	q := sqlc.New(pool)

	task, err := q.CreateTask(ctx, sqlc.CreateTaskParams{
		UserID: seededStudentID,
		Title:  "中国是否让地球变得更可持续？",
	})
	if err != nil {
		t.Fatalf("CreateTask: %v", err)
	}

	catalog, err := cards.Catalog()
	if err != nil {
		t.Fatalf("Catalog: %v", err)
	}

	stub := gateway.NewStubProvider([]gateway.StreamEvent{
		{Kind: gateway.EventTextDelta, TextDelta: "我们先核查一下来源。"},
		{Kind: gateway.EventToolUse, ToolUse: &gateway.StreamToolUse{
			ID:       "toolu_1",
			Name:     "summon_card",
			ArgsJSON: `{"card_id":"sift_craap","reason":"未核查出处","nudge_text":"要不要一起核查一下来源？"}`,
		}},
		{Kind: gateway.EventUsage, Usage: &gateway.ChatUsage{InputTokens: 120, OutputTokens: 45}},
		{Kind: gateway.EventDone, StopReason: gateway.StopToolCall},
	})

	sse := &fakeSSE{}
	deps := agent.TurnDeps{
		Store:       agent.NewSqlcTurnStore(q),
		Provider:    stub,
		KeyResolver: func(context.Context) (gateway.Resolved, error) { return gateway.Resolved{Provider: "deepseek", Model: "deepseek-chat", Tier: "chaperone"}, nil },
		Catalog:     catalog,
		SpecByID:    cards.ByID,
		SSE:         sse,
	}

	if err := agent.RunTurn(ctx, deps, task.ID, "这篇文章可信吗？"); err != nil {
		t.Fatalf("RunTurn: %v", err)
	}

	// SSE: text delta, then card, then done.
	if len(sse.texts) != 1 || sse.texts[0] != "我们先核查一下来源。" {
		t.Fatalf("texts = %v", sse.texts)
	}
	if sse.card == nil || sse.card.cardID != "sift_craap" || sse.card.nudge != "要不要一起核查一下来源？" {
		t.Fatalf("card event wrong: %+v", sse.card)
	}
	// Assert card_instance_id is non-empty and a valid UUID.
	if sse.card.ci == "" {
		t.Fatalf("card_instance_id is empty")
	}
	if _, err := uuid.Parse(sse.card.ci); err != nil {
		t.Fatalf("card_instance_id is not a valid UUID: %v", err)
	}
	if sse.done == "" {
		t.Fatalf("done not emitted")
	}

	// Persistence: user + assistant messages; assistant carries usage + tool_call.
	msgs, err := q.ListMessagesByTask(ctx, task.ID)
	if err != nil {
		t.Fatalf("ListMessagesByTask: %v", err)
	}
	if len(msgs) != 2 || msgs[0].Role != "user" || msgs[1].Role != "assistant" {
		t.Fatalf("messages = %d (%v)", len(msgs), rolesOf(msgs))
	}
	asst := msgs[1]
	if asst.Model == nil || *asst.Model != "deepseek-chat" {
		t.Fatalf("assistant model not persisted")
	}
	if asst.PromptTokens == nil || *asst.PromptTokens != 120 || asst.CompletionTokens == nil || *asst.CompletionTokens != 45 {
		t.Fatalf("assistant usage not persisted")
	}
	if len(asst.ToolCall) == 0 {
		t.Fatalf("assistant tool_call not persisted")
	}

	// Persistence: a proposed card_instance row linked to the task.
	cardsRows, err := q.ListCardsByTask(ctx, task.ID)
	if err != nil {
		t.Fatalf("ListCardsByTask: %v", err)
	}
	if len(cardsRows) != 1 || cardsRows[0].CardID != "sift_craap" || cardsRows[0].Status != "proposed" {
		t.Fatalf("card row wrong: %+v", cardsRows)
	}

	// Cost: deepseek/deepseek-chat has a price entry so CostEstimate must be valid
	// (non-NULL) in the persisted assistant message.
	if !asst.CostEstimate.Valid {
		t.Fatalf("assistant CostEstimate not persisted (want Valid=true, got %+v)", asst.CostEstimate)
	}
}

type fakeAnchorGen struct{ anchors []agent.Anchor }

func (f fakeAnchorGen) Generate(_ context.Context, _ cards.Spec, _ []agent.Material) ([]agent.Anchor, error) {
	return f.anchors, nil
}

func TestRunTurnWritesAnchorsForAnnotationCard(t *testing.T) {
	store := &fakeTurnStore{
		materials: []agent.Material{{ID: "m1", Title: "T", Blocks: []agent.MaterialBlock{{ID: "b0", Text: "原句。"}}}},
	}
	spec := cards.Spec{ID: "sift_craap", Name: "CRAAP", Mode: "annotation", Steps: []cards.Step{{Key: "a", Title: "权威性"}}}
	stub := gateway.NewStubProvider([]gateway.StreamEvent{
		{Kind: gateway.EventTextDelta, TextDelta: "先核查来源。"},
		{Kind: gateway.EventToolUse, ToolUse: &gateway.StreamToolUse{ID: "t1", Name: "summon_card", ArgsJSON: `{"card_id":"sift_craap","reason":"r","nudge_text":"n"}`}},
		{Kind: gateway.EventDone, StopReason: gateway.StopToolCall},
	})
	deps := agent.TurnDeps{
		Store:       store,
		Provider:    stub,
		KeyResolver: func(context.Context) (gateway.Resolved, error) { return gateway.Resolved{Provider: "stub"}, nil },
		Catalog:     []cards.Spec{spec},
		SpecByID: func(id string) (cards.Spec, bool) {
			if id == "sift_craap" {
				return spec, true
			}
			return cards.Spec{}, false
		},
		SSE:       &fakeSSE{},
		AnchorGen: fakeAnchorGen{anchors: []agent.Anchor{{ID: "a0", BlockID: "b0", Dimension: "权威性", Author: "ai", Question: "可信吗？"}}},
	}
	if err := agent.RunTurn(context.Background(), deps, uuid.New(), "看看这个"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(store.anchorsWritten), "可信吗？") {
		t.Fatalf("anchors not persisted: %s", store.anchorsWritten)
	}
}

func rolesOf(msgs []sqlc.Message) []string {
	out := make([]string, len(msgs))
	for i, m := range msgs {
		out[i] = m.Role
	}
	return out
}

func newTurnTestPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	ctx := context.Background()
	pg, err := tcpostgres.Run(ctx, "postgres:16-alpine",
		tcpostgres.WithDatabase("mindimprint"),
		tcpostgres.WithUsername("test"),
		tcpostgres.WithPassword("test"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).WithStartupTimeout(60*time.Second)),
	)
	if err != nil {
		t.Fatalf("start postgres: %v", err)
	}
	t.Cleanup(func() { _ = pg.Terminate(context.Background()) })
	dsn, err := pg.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatalf("dsn: %v", err)
	}
	pool, err := store.NewPool(ctx, dsn)
	if err != nil {
		t.Fatalf("pool: %v", err)
	}
	t.Cleanup(pool.Close)
	if err := store.RunMigrations(ctx, pool); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return pool
}
