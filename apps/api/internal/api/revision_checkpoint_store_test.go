package api_test

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"

	"mindimprint/api/internal/store/sqlc"
)

func TestRevisionCheckpoint_InsertAndList(t *testing.T) {
	pool := newAPITestPool(t)
	q := sqlc.New(pool)
	ctx := context.Background()

	got, err := q.InsertRevisionCheckpoint(ctx, sqlc.InsertRevisionCheckpointParams{
		ProjectID: mustUUID(seedProjectID), ArtifactType: "proposal", Trigger: "finish",
		Content: []byte(`{"objective":"x"}`), ContentHash: "abc", FeedbackRef: pgtype.UUID{Valid: false},
	})
	if err != nil {
		t.Fatalf("insert: %v", err)
	}
	if got.ArtifactType != "proposal" {
		t.Fatalf("artifact_type = %s", got.ArtifactType)
	}
	rows, err := q.ListRevisionCheckpoints(ctx, mustUUID(seedProjectID))
	if err != nil || len(rows) != 1 {
		t.Fatalf("list = %d rows, %v", len(rows), err)
	}
}
