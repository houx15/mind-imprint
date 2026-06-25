package agent_test

import (
	"context"
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
