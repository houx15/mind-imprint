package store

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

// newTestPool spins up a throwaway Postgres, runs all migrations, and returns a
// connected pool. The container is terminated via t.Cleanup.
func newTestPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	ctx := context.Background()

	pg, err := tcpostgres.Run(ctx,
		"postgres:16-alpine",
		tcpostgres.WithDatabase("mindimprint"),
		tcpostgres.WithUsername("test"),
		tcpostgres.WithPassword("test"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(60*time.Second),
		),
	)
	if err != nil {
		t.Fatalf("start postgres container: %v", err)
	}
	t.Cleanup(func() { _ = pg.Terminate(context.Background()) })

	dsn, err := pg.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatalf("connection string: %v", err)
	}

	pool, err := NewPool(ctx, dsn)
	if err != nil {
		t.Fatalf("new pool: %v", err)
	}
	t.Cleanup(pool.Close)

	if err := RunMigrations(ctx, pool); err != nil {
		t.Fatalf("run migrations: %v", err)
	}
	return pool
}

func TestMigrationsCreateTablesAndSeed(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping testcontainers integration in -short mode")
	}
	ctx := context.Background()
	pool := newTestPool(t)

	wantTables := []string{
		"schools", "classes", "users", "enrollments",
		"tasks", "messages", "card_instances", "evaluations",
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
			t.Fatalf("table %s missing after migration", name)
		}
	}

	// llm_usage view exists.
	var viewExists bool
	if err := pool.QueryRow(ctx,
		`SELECT EXISTS (SELECT 1 FROM information_schema.views
		 WHERE table_schema='public' AND table_name='llm_usage')`).Scan(&viewExists); err != nil {
		t.Fatalf("check view: %v", err)
	}
	if !viewExists {
		t.Fatal("llm_usage view missing")
	}

	// Seeded student present and org-bound; school matches the class's school.
	var (
		studentSchool string
		classSchool   string
		role          string
	)
	err := pool.QueryRow(ctx, `
		SELECT u.school_id::text, c.school_id::text, e.role_in_class
		  FROM users u
		  JOIN enrollments e ON e.user_id = u.id
		  JOIN classes c ON c.id = e.class_id
		 WHERE u.email = 'phoebe@demo.mindimprint.local'`).
		Scan(&studentSchool, &classSchool, &role)
	if err != nil {
		t.Fatalf("seeded student/enrollment not found: %v", err)
	}
	if studentSchool != classSchool {
		t.Fatalf("invariant broken: user.school_id %s != class.school_id %s", studentSchool, classSchool)
	}
	if role != "student" {
		t.Fatalf("role_in_class = %q, want student", role)
	}
}
