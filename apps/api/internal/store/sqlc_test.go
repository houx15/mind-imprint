package store_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"mindimprint/api/internal/store"
	"mindimprint/api/internal/store/sqlc"
)

// seededStudentID is the fixed UUID from migration 0002_seed.sql.
var seededStudentID = uuid.MustParse("00000000-0000-0000-0000-000000000003")

func TestStoreRoundTrip(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping testcontainers integration in -short mode")
	}
	ctx := context.Background()
	pool := newStoreTestPool(t)
	q := sqlc.New(pool)

	// The seeded user resolves.
	u, err := q.GetUserByID(ctx, seededStudentID)
	if err != nil {
		t.Fatalf("GetUserByID: %v", err)
	}
	if u.DisplayName != "Phoebe" {
		t.Fatalf("display_name = %q, want Phoebe", u.DisplayName)
	}

	// Create a task for the seeded user.
	task, err := q.CreateTask(ctx, sqlc.CreateTaskParams{
		UserID: seededStudentID,
		Title:  "中国是否让地球变得更可持续？",
		Seed:   ptr("https://example.com/article"),
	})
	if err != nil {
		t.Fatalf("CreateTask: %v", err)
	}

	// Append a user message then an assistant message carrying usage.
	if _, err := q.AppendMessage(ctx, sqlc.AppendMessageParams{
		TaskID:  task.ID,
		Role:    "user",
		Content: "这篇文章可信吗？",
	}); err != nil {
		t.Fatalf("AppendMessage(user): %v", err)
	}

	cost := pgtype.Numeric{}
	if err := cost.Scan("0.001200"); err != nil {
		t.Fatalf("scan numeric: %v", err)
	}
	if _, err := q.AppendMessage(ctx, sqlc.AppendMessageParams{
		TaskID:           task.ID,
		Role:             "assistant",
		Content:          "我们一起核查一下来源。",
		Provider:         ptr("deepseek"),
		Model:            ptr("deepseek-chat"),
		Tier:             ptr("chaperone"),
		PromptTokens:     ptrInt32(120),
		CompletionTokens: ptrInt32(45),
		CostEstimate:     cost,
	}); err != nil {
		t.Fatalf("AppendMessage(assistant): %v", err)
	}

	msgs, err := q.ListMessagesByTask(ctx, task.ID)
	if err != nil {
		t.Fatalf("ListMessagesByTask: %v", err)
	}
	if len(msgs) != 2 {
		t.Fatalf("got %d messages, want 2", len(msgs))
	}
	if msgs[0].Role != "user" || msgs[1].Role != "assistant" {
		t.Fatalf("message order wrong: %q, %q", msgs[0].Role, msgs[1].Role)
	}
	if msgs[1].Model == nil || *msgs[1].Model != "deepseek-chat" {
		t.Fatalf("assistant row missing model")
	}
}

func TestDraftSnapshotAndBufferRoundTrip(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping testcontainers integration in -short mode")
	}
	ctx := context.Background()
	pool := newStoreTestPool(t)
	q := sqlc.New(pool)
	projectID := seedTestProject(t, ctx, q)

	// Buffer upsert is idempotent per project.
	if err := q.UpsertEditBuffer(ctx, sqlc.UpsertEditBufferParams{ProjectID: projectID, Content: "draft one"}); err != nil {
		t.Fatal(err)
	}
	if err := q.UpsertEditBuffer(ctx, sqlc.UpsertEditBufferParams{ProjectID: projectID, Content: "draft two"}); err != nil {
		t.Fatal(err)
	}
	buf, err := q.GetEditBuffer(ctx, projectID)
	if err != nil || buf != "draft two" {
		t.Fatalf("GetEditBuffer = %q, %v; want \"draft two\"", buf, err)
	}

	// Snapshot seq is monotonic and content immutable.
	next, err := q.NextSnapshotSeq(ctx, projectID)
	if err != nil || next != 1 {
		t.Fatalf("NextSnapshotSeq = %d, %v; want 1", next, err)
	}
	s1, err := q.InsertDraftSnapshot(ctx, sqlc.InsertDraftSnapshotParams{
		ProjectID: projectID, Seq: 1, Content: "v1 body", SpanIndex: []byte("[]"),
	})
	if err != nil {
		t.Fatal(err)
	}
	next2, _ := q.NextSnapshotSeq(ctx, projectID)
	if next2 != 2 {
		t.Fatalf("NextSnapshotSeq after one = %d, want 2", next2)
	}
	latest, err := q.GetLatestSnapshot(ctx, projectID)
	if err != nil || latest.ID != s1.ID {
		t.Fatalf("GetLatestSnapshot = %v, %v; want %v", latest.ID, err, s1.ID)
	}
	got, err := q.GetSnapshot(ctx, sqlc.GetSnapshotParams{ID: s1.ID, ProjectID: projectID})
	if err != nil || got.Content != "v1 body" {
		t.Fatalf("GetSnapshot = %q, %v; want \"v1 body\"", got.Content, err)
	}
}

// seedTestProject creates a project owned by the fixed seeded student for
// tests that need a project_id foreign key (writing.sql's edit_buffer and
// draft_snapshot both cascade off project).
func seedTestProject(t *testing.T, ctx context.Context, q *sqlc.Queries) uuid.UUID {
	t.Helper()
	p, err := q.CreateProject(ctx, sqlc.CreateProjectParams{
		UserID:        seededStudentID,
		Qualification: "IB",
		Title:         "中国是否让地球变得更可持续？",
		BoardCfgVer:   1,
	})
	if err != nil {
		t.Fatalf("CreateProject: %v", err)
	}
	return p.ID
}

func ptr(s string) *string { return &s }
func ptrInt32(n int32) *int32 { return &n }

// newStoreTestPool spins up a throwaway Postgres, runs all migrations, and
// returns a connected pool. Lives in the external test package so it cannot
// use the unexported newTestPool from package store.
func newStoreTestPool(t *testing.T) *pgxpool.Pool {
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
