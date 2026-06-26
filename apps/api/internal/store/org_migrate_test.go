package store_test

import (
	"context"
	"testing"
)

// TestOrgMigrationArtifacts verifies 0005 created the table, column, and view.
func TestOrgMigrationArtifacts(t *testing.T) {
	pool := newStoreTestPool(t) // existing helper in sqlc_test.go
	ctx := context.Background()

	var n int
	if err := pool.QueryRow(ctx,
		`SELECT count(*) FROM information_schema.tables WHERE table_name='teacher_invites'`,
	).Scan(&n); err != nil || n != 1 {
		t.Fatalf("teacher_invites table missing: n=%d err=%v", n, err)
	}
	if err := pool.QueryRow(ctx,
		`SELECT count(*) FROM information_schema.columns WHERE table_name='classes' AND column_name='created_by'`,
	).Scan(&n); err != nil || n != 1 {
		t.Fatalf("classes.created_by missing: n=%d err=%v", n, err)
	}
	if err := pool.QueryRow(ctx,
		`SELECT count(*) FROM information_schema.views WHERE table_name='llm_usage'`,
	).Scan(&n); err != nil || n != 1 {
		t.Fatalf("llm_usage view missing: n=%d err=%v", n, err)
	}
	// Unique constraint on code: a duplicate insert must fail.
	_, err := pool.Exec(ctx, `INSERT INTO teacher_invites (school_id, code, created_by, expires_at)
		VALUES ('00000000-0000-0000-0000-000000000001','T-DUPE','00000000-0000-0000-0000-000000000003', now()+interval '1 day')`)
	if err != nil {
		t.Fatalf("first invite insert: %v", err)
	}
	_, err = pool.Exec(ctx, `INSERT INTO teacher_invites (school_id, code, created_by, expires_at)
		VALUES ('00000000-0000-0000-0000-000000000001','T-DUPE','00000000-0000-0000-0000-000000000003', now()+interval '1 day')`)
	if err == nil {
		t.Fatal("expected unique violation on duplicate code, got nil")
	}
}
