package store_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"mindimprint/api/internal/store/sqlc"
)

// TestChatMessagesRoundTrip exercises the chat_thread/chat_message queries
// introduced for Slice 5c's conversational loop. chat_message has no
// project_id column of its own — it joins through chat_thread.seeded_project_id,
// which is nullable, so its sqlc param type is pgtype.UUID.
func TestChatMessagesRoundTrip(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping testcontainers integration in -short mode")
	}
	pool := newStoreTestPool(t)
	q := sqlc.New(pool)
	ctx := context.Background()

	// The seed migration 0018 creates project …0101 owned by Phoebe …0003.
	projectID := uuid.MustParse("00000000-0000-0000-0000-000000000101")
	pgProject := pgtype.UUID{Bytes: projectID, Valid: true}

	// No thread yet.
	if _, err := q.GetThreadByProject(ctx, pgProject); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("expected no thread, got %v", err)
	}
	th, err := q.CreateThread(ctx, sqlc.CreateThreadParams{UserID: uuid.MustParse("00000000-0000-0000-0000-000000000003"), SeededProjectID: pgProject})
	if err != nil {
		t.Fatal(err)
	}

	for i, body := range []string{"它想证明中国在认真转型", "那要连到哪条主张？"} {
		if _, err := q.CreateChatMessage(ctx, sqlc.CreateChatMessageParams{ThreadID: th.ID, Role: "user", Content: body, Modality: "text"}); err != nil {
			t.Fatalf("msg %d: %v", i, err)
		}
	}
	msgs, err := q.ListChatMessagesByProject(ctx, pgProject)
	if err != nil {
		t.Fatal(err)
	}
	if len(msgs) != 2 || msgs[0].Content != "它想证明中国在认真转型" {
		t.Fatalf("messages = %+v", msgs)
	}
	// GetThreadByProject now finds it.
	if got, err := q.GetThreadByProject(ctx, pgProject); err != nil || got.ID != th.ID {
		t.Fatalf("GetThreadByProject = %v, %v", got, err)
	}
}
