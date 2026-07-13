package agent_test

// agentstore_chat_sqlc_test.go — Task 2's real-DB pass for the chat-history
// seam: sqlcAgentStore.CreateChatMessage/LoadChatHistory over a testcontainers
// Postgres, exercising the merge of student chat_messages with prior
// interventions (design: LoadChatHistory MERGES role "user" chat_message rows
// + role "assistant" intervention.Body rows, time-ordered, capped to limit).

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"mindimprint/api/internal/agent"
	"mindimprint/api/internal/store/sqlc"
)

// pgUUID is not defined anywhere in package agent_test (the sibling helper in
// internal/studio/projection_test.go lives in a different package), so this
// file provides its own local copy per the brief's documented fallback.
func pgUUID(id uuid.UUID) pgtype.UUID { return pgtype.UUID{Bytes: id, Valid: true} }

func TestSqlcAgentStore_ChatHistoryMerge(t *testing.T) {
	if testing.Short() {
		t.Skip("requires postgres")
	}
	pool := newTurnTestPool(t) // reuse the existing agent sqlc-test pool helper
	q := sqlc.New(pool)
	store := agent.NewSqlcAgentStore(q, pool)
	ctx := context.Background()
	projectID := uuid.MustParse("00000000-0000-0000-0000-000000000101") // seeded

	if err := store.CreateChatMessage(ctx, projectID, "user", "它想证明中国在认真转型"); err != nil {
		t.Fatal(err)
	}
	// The seed already has 2 interventions (…0120 flag, …0121 diagnostic) for this project.
	hist, err := store.LoadChatHistory(ctx, projectID, 12)
	if err != nil {
		t.Fatal(err)
	}
	// Merged: 2 seeded interventions (assistant) + 1 new user turn, time-ordered.
	if len(hist) < 3 {
		t.Fatalf("history len = %d, want >=3", len(hist))
	}
	last := hist[len(hist)-1]
	if last.Role != "user" || last.Content != "它想证明中国在认真转型" {
		t.Fatalf("last turn = %+v", last)
	}
	// A second CreateChatMessage must reuse the same thread (idempotent thread).
	if err := store.CreateChatMessage(ctx, projectID, "user", "第二句"); err != nil {
		t.Fatal(err)
	}
	th, err := q.GetThreadByProject(ctx, pgUUID(projectID))
	if err != nil {
		t.Fatalf("thread not reused: %v", err)
	}
	msgs, _ := q.ListChatMessagesByProject(ctx, pgUUID(projectID))
	if len(msgs) != 2 {
		t.Fatalf("want 2 user msgs on one thread, got %d (thread %s)", len(msgs), th.ID)
	}
}
