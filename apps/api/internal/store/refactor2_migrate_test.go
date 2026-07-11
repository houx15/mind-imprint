package store

import (
	"context"
	"testing"
)

// TestRefactor2MigrationCreatesFoundationTables asserts migration 0016 lands the
// project/graph/event foundation additively: the new tables exist, and the
// existing task-scoped tables gain a nullable project_id column.
func TestRefactor2MigrationCreatesFoundationTables(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping testcontainers integration in -short mode")
	}
	ctx := context.Background()
	pool := newTestPool(t)

	wantTables := []string{
		"project", "graph_node", "graph_edge", "draft_snapshot", "edit_buffer",
		"source_log_entry", "intervention", "disposition", "card_competence",
		"chat_thread", "chat_message", "event",
	}
	for _, name := range wantTables {
		var exists bool
		err := pool.QueryRow(ctx,
			`SELECT EXISTS (SELECT 1 FROM information_schema.tables
			 WHERE table_schema='public' AND table_name=$1)`, name).Scan(&exists)
		if err != nil {
			t.Fatalf("check table %s: %v", name, err)
		}
		if !exists {
			t.Fatalf("table %s missing after migration 0016", name)
		}
	}

	wantProjectIDColumn := []string{"material", "card_instances", "evaluations"}
	for _, table := range wantProjectIDColumn {
		var exists bool
		err := pool.QueryRow(ctx,
			`SELECT EXISTS (SELECT 1 FROM information_schema.columns
			 WHERE table_schema='public' AND table_name=$1 AND column_name='project_id')`, table).Scan(&exists)
		if err != nil {
			t.Fatalf("check %s.project_id: %v", table, err)
		}
		if !exists {
			t.Fatalf("%s.project_id missing after migration 0016", table)
		}
	}
}
