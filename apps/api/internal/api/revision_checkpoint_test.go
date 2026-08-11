package api

// revision_checkpoint_test.go — TDD for recordCheckpoint (Task 2 of the
// revision-recording plan). Lives in package `api` (not api_test) because
// recordCheckpoint and the API{d:...} internals it needs are unexported —
// mirrors the internal-test convention used by projectcoach_nextstep_test.go
// / coach_cardref_test.go, but (uniquely among internal-package tests here)
// needs a real Postgres, so it carries its own testcontainers bootstrap
// (duplicated from maintest_test.go's newAPITestPool, which lives in the
// separate api_test package and isn't importable from here — same reason
// maintest_test.go gives for duplicating it from internal/store/sqlc_test.go).

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"mindimprint/api/internal/agent"
	"mindimprint/api/internal/auth"
	"mindimprint/api/internal/store"
	"mindimprint/api/internal/store/sqlc"
)

// seedProjectID101 is the seeded demo project (migration 0018), owned by the
// seeded student — mirrors api_test's seedProjectID constant, redeclared here
// because internal-package tests can't import the external api_test package.
const seedProjectID101 = "00000000-0000-0000-0000-000000000101"

// newCheckpointTestPool spins up a throwaway Postgres, runs all migrations
// (incl. seed), and returns the pool. Container/pool torn down via t.Cleanup.
func newCheckpointTestPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	if testing.Short() {
		t.Skip("skipping testcontainers integration in -short mode")
	}
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

func TestRecordCheckpoint_ProposalWritesRowWithHash(t *testing.T) {
	pool := newCheckpointTestPool(t)
	d := sqlc.New(pool)
	projectID := uuid.MustParse(seedProjectID101)

	// seed a proposal so there is content to snapshot
	if _, err := d.UpsertProjectProposal(context.Background(), sqlc.UpsertProjectProposalParams{
		ProjectID: projectID, Objective: "bounded yes", Reason: "r", Activities: "a", Resources: "s",
	}); err != nil {
		t.Fatalf("seed proposal: %v", err)
	}

	a := New(Deps{Queries: d, Pool: pool})
	a.recordCheckpoint(context.Background(), projectID, checkpointProposal, triggerFinish, nil)

	rows, err := d.ListRevisionCheckpoints(context.Background(), projectID)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("want 1 checkpoint, got %d", len(rows))
	}
	if rows[0].ContentHash == "" {
		t.Fatal("content_hash empty")
	}
	if !strings.Contains(string(rows[0].Content), "bounded yes") {
		t.Fatalf("content missing objective: %s", rows[0].Content)
	}
}

func TestRecordCheckpoint_DraftStoresSnapshotRefNotBody(t *testing.T) {
	pool := newCheckpointTestPool(t)
	d := sqlc.New(pool)
	projectID := uuid.MustParse(seedProjectID101)

	// commit a draft (essay) snapshot with a long body
	seq, err := d.NextSnapshotSeq(context.Background(), sqlc.NextSnapshotSeqParams{
		ProjectID: projectID, DocKind: string(agent.DocEssay),
	})
	if err != nil {
		t.Fatalf("next seq: %v", err)
	}
	snap, err := d.InsertDraftSnapshot(context.Background(), sqlc.InsertDraftSnapshotParams{
		ProjectID: projectID, DocKind: string(agent.DocEssay), Seq: seq, Content: strings.Repeat("body ", 200),
		SpanIndex: []byte("[]"),
	})
	if err != nil {
		t.Fatalf("insert snapshot: %v", err)
	}

	a := New(Deps{Queries: d, Pool: pool})
	a.recordCheckpoint(context.Background(), projectID, checkpointDraft, triggerFinish, nil)

	rows, err := d.ListRevisionCheckpoints(context.Background(), projectID)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("want 1, got %d", len(rows))
	}
	if strings.Contains(string(rows[0].Content), "body body") {
		t.Fatal("draft checkpoint duplicated body text")
	}
	if !strings.Contains(string(rows[0].Content), snap.ID.String()) {
		t.Fatalf("draft checkpoint missing snapshotId ref: %s", rows[0].Content)
	}
}

// TestRecordCheckpoint_ClaimNoSubQuestionsSkipsRow — final-review fix: a
// project with no proposal track (so essaySubQuestions returns a nil slice)
// must write NO checkpoint row rather than a `content: null` one, which the
// ClaimCheckpointContent Zod contract (z.array(...)) would reject.
func TestRecordCheckpoint_ClaimNoSubQuestionsSkipsRow(t *testing.T) {
	pool := newCheckpointTestPool(t)
	d := sqlc.New(pool)
	projectID := uuid.MustParse(seedProjectID101)
	// seed project 101's studio_state carries no proposalTrack, so
	// essaySubQuestions(state) returns nil here — nothing further to seed.

	a := New(Deps{Queries: d, Pool: pool})
	a.recordCheckpoint(context.Background(), projectID, checkpointClaim, triggerFinish, nil)

	rows, err := d.ListRevisionCheckpoints(context.Background(), projectID)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(rows) != 0 {
		t.Fatalf("want 0 checkpoints when there are no sub-questions, got %d: %+v", len(rows), rows)
	}
}

// TestRecordCheckpoint_ClaimWithSubQuestionsWritesArray — the flip side: once
// the proposal track has sub-questions, the claim checkpoint writes a proper
// non-null JSON array.
func TestRecordCheckpoint_ClaimWithSubQuestionsWritesArray(t *testing.T) {
	pool := newCheckpointTestPool(t)
	d := sqlc.New(pool)
	projectID := uuid.MustParse(seedProjectID101)

	state := agent.DefaultStudioState()
	state.ProposalTrack = &agent.WritingTrack{
		SubQuestions: []agent.SubQuestion{{ID: "sq1", Text: "does X cause Y?"}},
	}
	raw, err := json.Marshal(state)
	if err != nil {
		t.Fatalf("marshal state: %v", err)
	}
	if err := d.SetStudioState(context.Background(), sqlc.SetStudioStateParams{ID: projectID, StudioState: raw}); err != nil {
		t.Fatalf("seed studio_state: %v", err)
	}

	a := New(Deps{Queries: d, Pool: pool})
	a.recordCheckpoint(context.Background(), projectID, checkpointClaim, triggerFinish, nil)

	rows, err := d.ListRevisionCheckpoints(context.Background(), projectID)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("want 1 checkpoint, got %d", len(rows))
	}
	if strings.TrimSpace(string(rows[0].Content)) == "null" {
		t.Fatal("claim checkpoint content is null")
	}
	if !strings.HasPrefix(strings.TrimSpace(string(rows[0].Content)), "[") {
		t.Fatalf("claim checkpoint content is not a JSON array: %s", rows[0].Content)
	}
	if !strings.Contains(string(rows[0].Content), "does X cause Y?") {
		t.Fatalf("claim checkpoint missing sub-question text: %s", rows[0].Content)
	}
}

// TestGetRevisionCheckpoints_ReturnsOrdered — Task 7: the read-only GET
// endpoint projects recorded checkpoints as JSON. Self-contained HTTP-level
// auth setup (mirrors maintest_test.go's signInAs) because this file lives in
// package api (unexported recordCheckpoint access), not the external
// api_test package where that helper is defined.
func TestGetRevisionCheckpoints_ReturnsOrdered(t *testing.T) {
	pool := newCheckpointTestPool(t)
	d := sqlc.New(pool)
	projectID := uuid.MustParse(seedProjectID101)

	// seed a proposal so there is content to snapshot (mirrors
	// TestRecordCheckpoint_ProposalWritesRowWithHash above)
	if _, err := d.UpsertProjectProposal(context.Background(), sqlc.UpsertProjectProposalParams{
		ProjectID: projectID, Objective: "bounded yes", Reason: "r", Activities: "a", Resources: "s",
	}); err != nil {
		t.Fatalf("seed proposal: %v", err)
	}

	a := New(Deps{Queries: d, Pool: pool})
	a.recordCheckpoint(context.Background(), projectID, checkpointProposal, triggerFinish, nil)

	raw, hash, err := auth.NewToken()
	if err != nil {
		t.Fatalf("token: %v", err)
	}
	if _, err := d.CreateSession(context.Background(), sqlc.CreateSessionParams{
		UserID: SeedUserID, TokenHash: hash, ExpiresAt: time.Now().Add(time.Hour),
	}); err != nil {
		t.Fatalf("create session: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/projects/"+projectID.String()+"/revision-checkpoints", nil)
	req.AddCookie(&http.Cookie{Name: "mk_session", Value: raw})
	rec := httptest.NewRecorder()
	a.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("GET revision-checkpoints = %d, want 200: %s", rec.Code, rec.Body)
	}
	body := rec.Body.String()
	if !strings.Contains(body, `"artifactType":"proposal"`) {
		t.Fatalf("missing checkpoint in response: %s", body)
	}
}
