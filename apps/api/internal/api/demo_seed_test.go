package api_test

// demo_seed_test.go — asserts migration 0082 seeds the guided-tour demo
// project (00000000-0000-0000-0000-000000000200) with substantive content for
// rooms 立题 / 管理 / 阅读. Tasks 4-5 extend this test (写作/回顾 + eval report).
// Direct pool queries against the migrated testcontainer DB.

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

const demoProjectID = "00000000-0000-0000-0000-000000000200"
const demoOwnerID = "00000000-0000-0000-0000-000000000003" // Phoebe

func TestDemoSeed(t *testing.T) {
	pool := newAPITestPool(t)
	ctx := context.Background()

	// Core: the demo project exists, is world-readable demo, owned by Phoebe.
	var owner string
	var isDemo bool
	var status, title string
	var started bool
	if err := pool.QueryRow(ctx,
		`SELECT user_id, is_demo, status, title, (studio_state->>'started')::bool
		   FROM project WHERE id = $1`, demoProjectID,
	).Scan(&owner, &isDemo, &status, &title, &started); err != nil {
		t.Fatalf("demo project row: %v", err)
	}
	if owner != demoOwnerID {
		t.Errorf("demo project owner = %s, want %s (Phoebe)", owner, demoOwnerID)
	}
	if !isDemo {
		t.Error("demo project is_demo = false, want true")
	}
	if !started {
		t.Error("demo project studio_state.started = false, want true")
	}
	if title == "" {
		t.Error("demo project title is empty")
	}

	// 立题: proposal has all four dimensions filled with non-trivial prose.
	var obj, reason, acts, res string
	if err := pool.QueryRow(ctx,
		`SELECT objective, reason, activities, resources
		   FROM project_proposal WHERE project_id = $1`, demoProjectID,
	).Scan(&obj, &reason, &acts, &res); err != nil {
		t.Fatalf("project_proposal row: %v", err)
	}
	for _, d := range []struct {
		name, val string
	}{{"objective", obj}, {"reason", reason}, {"activities", acts}, {"resources", res}} {
		if len([]rune(d.val)) < 20 {
			t.Errorf("proposal.%s too short (%q) — want a real paragraph", d.name, d.val)
		}
	}

	// 管理: ≥3 plan items.
	if n := countDemoRows(t, pool, `SELECT count(*) FROM plan_item WHERE project_id = $1`, demoProjectID); n < 3 {
		t.Errorf("plan_item count = %d, want ≥3", n)
	}

	// 管理: ≥3 activity log entries.
	if n := countDemoRows(t, pool, `SELECT count(*) FROM activity_log_entry WHERE project_id = $1`, demoProjectID); n < 3 {
		t.Errorf("activity_log_entry count = %d, want ≥3", n)
	}

	// 管理/过程: the 立题-done milestone event exists.
	if n := countDemoRows(t, pool,
		`SELECT count(*) FROM event WHERE project_id = $1 AND type = 'milestone:framework_finished'`,
		demoProjectID); n < 1 {
		t.Error("milestone:framework_finished event missing")
	}

	// 论证图: ≥2 graph nodes (a claim + evidence).
	if n := countDemoRows(t, pool, `SELECT count(*) FROM graph_node WHERE project_id = $1`, demoProjectID); n < 2 {
		t.Errorf("graph_node count = %d, want ≥2", n)
	}

	// 阅读: ≥4 references.
	if n := countDemoRows(t, pool, `SELECT count(*) FROM reference WHERE project_id = $1`, demoProjectID); n < 4 {
		t.Errorf("reference count = %d, want ≥4", n)
	}

	// 阅读: ≥1 material with a non-empty blocks array (the reading room renders
	// only when blocks are present).
	if n := countDemoRows(t, pool,
		`SELECT count(*) FROM material WHERE project_id = $1 AND jsonb_array_length(blocks) > 0`,
		demoProjectID); n < 1 {
		t.Error("no material with non-empty blocks for demo project")
	}

	// 阅读: exploration leads + at least one question edge.
	if n := countDemoRows(t, pool, `SELECT count(*) FROM exploration_lead WHERE project_id = $1`, demoProjectID); n < 3 {
		t.Errorf("exploration_lead count = %d, want ≥3", n)
	}
	if n := countDemoRows(t, pool, `SELECT count(*) FROM question_edge WHERE project_id = $1`, demoProjectID); n < 1 {
		t.Error("no question_edge for demo project")
	}

	// 阅读: ≥1 citation.
	if n := countDemoRows(t, pool, `SELECT count(*) FROM citation WHERE project_id = $1`, demoProjectID); n < 1 {
		t.Error("no citation for demo project")
	}

	// studio thread + a studio-surface message.
	if n := countDemoRows(t, pool,
		`SELECT count(*) FROM chat_message m
		   JOIN chat_thread t ON t.id = m.thread_id
		  WHERE t.seeded_project_id = $1 AND m.surface = 'studio'`,
		demoProjectID); n < 1 {
		t.Error("no studio-surface chat_message for demo project")
	}
}

func countDemoRows(t *testing.T, pool *pgxpool.Pool, sql string, args ...any) int {
	t.Helper()
	var n int
	if err := pool.QueryRow(context.Background(), sql, args...).Scan(&n); err != nil {
		t.Fatalf("count query %q: %v", sql, err)
	}
	return n
}
