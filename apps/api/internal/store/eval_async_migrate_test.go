package store_test

import (
	"context"
	"testing"
)

func TestMigration0007AddsEvalAsyncColumns(t *testing.T) {
	pool := newStoreTestPool(t)
	ctx := context.Background()

	var n int
	err := pool.QueryRow(ctx, `
		SELECT count(*) FROM information_schema.columns
		WHERE table_name = 'evaluations' AND column_name IN ('signals','rubric_version')
	`).Scan(&n)
	if err != nil {
		t.Fatalf("query columns: %v", err)
	}
	if n != 2 {
		t.Fatalf("expected 2 new columns, got %d", n)
	}
}
