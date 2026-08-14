package api

// evaluation_report_lifecycle_test.go — Task 4 (retire-old-evaluation-pipeline,
// 2026-08-14): the generation lifecycle (status + atomic claim) that makes
// generateAndStoreEvaluationReport single-flight, plus the three-state read
// envelope (writeEvalReportEnvelope). Deliberately package `api` (not
// api_test) so it can reach these unexported symbols directly — same
// convention as projectcoach_nextstep_test.go / teacher_read_internal_test.go.
// The testcontainers bootstrap below duplicates api_test's newAPITestPool
// (that helper lives in the external test package and isn't importable here;
// maintest_test.go documents the same tradeoff for its own duplication).

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"mindimprint/api/internal/store"
	"mindimprint/api/internal/store/sqlc"
)

// newLifecycleTestPool spins up a throwaway Postgres, runs all migrations
// (incl. seed), and returns the pool. Container/pool torn down via t.Cleanup.
func newLifecycleTestPool(t *testing.T) *pgxpool.Pool {
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

// lifecycleTestProjectID is the seeded demo project (owned by SeedUserID,
// migration 0002_seed.sql) — same fixture api_test's materialsTestProjectID
// points at.
const lifecycleTestProjectID = "00000000-0000-0000-0000-000000000101"

// countEvaluationReportRows counts evaluation_report rows for a project,
// optionally filtered by status ("" = any).
func countEvaluationReportRows(t *testing.T, pool *pgxpool.Pool, projectID, status string) int {
	t.Helper()
	var n int
	var err error
	if status == "" {
		err = pool.QueryRow(context.Background(),
			`SELECT count(*) FROM evaluation_report WHERE project_id = $1`, projectID).Scan(&n)
	} else {
		err = pool.QueryRow(context.Background(),
			`SELECT count(*) FROM evaluation_report WHERE project_id = $1 AND status = $2`, projectID, status).Scan(&n)
	}
	if err != nil {
		t.Fatalf("count evaluation_report rows: %v", err)
	}
	return n
}

// TestGenerateAndStoreEvaluationReport_ConcurrentSingleFlight — two goroutines
// race generateAndStoreEvaluationReport for the SAME project. The atomic
// ON CONFLICT claim must let exactly one of them own generation; the other
// gets pgx.ErrNoRows from the claim and returns nil without writing anything.
// End state: exactly one evaluation_report row, status='ready'.
func TestGenerateAndStoreEvaluationReport_ConcurrentSingleFlight(t *testing.T) {
	pool := newLifecycleTestPool(t)
	a := New(Deps{Queries: sqlc.New(pool), Pool: pool})
	projectID := uuid.MustParse(lifecycleTestProjectID)
	ctx := WithUser(context.Background(), User{ID: SeedUserID})

	var wg sync.WaitGroup
	errs := make([]error, 2)
	for i := range 2 {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			errs[i] = a.generateAndStoreEvaluationReport(ctx, projectID)
		}(i)
	}
	wg.Wait()

	for i, err := range errs {
		if err != nil {
			t.Fatalf("generateAndStoreEvaluationReport goroutine %d returned error: %v (want nil either way — winner completes, loser no-ops)", i, err)
		}
	}

	if n := countEvaluationReportRows(t, pool, lifecycleTestProjectID, ""); n != 1 {
		t.Fatalf("evaluation_report rows for project = %d, want exactly 1 (no double-generation)", n)
	}
	if n := countEvaluationReportRows(t, pool, lifecycleTestProjectID, "ready"); n != 1 {
		t.Fatalf("ready evaluation_report rows = %d, want 1", n)
	}
}

// TestClaimEvaluationReportGeneration_FailedRowIsReclaimable — a 'failed' row
// (a crashed prior generation) must be re-claimable, so generation is never
// permanently locked out by one failure.
func TestClaimEvaluationReportGeneration_FailedRowIsReclaimable(t *testing.T) {
	pool := newLifecycleTestPool(t)
	q := sqlc.New(pool)
	projectID := uuid.MustParse(lifecycleTestProjectID)
	ctx := context.Background()

	if _, err := q.ClaimEvaluationReportGeneration(ctx, projectID); err != nil {
		t.Fatalf("initial claim: %v", err)
	}
	if err := q.FailEvaluationReport(ctx, projectID); err != nil {
		t.Fatalf("fail: %v", err)
	}
	if n := countEvaluationReportRows(t, pool, lifecycleTestProjectID, "failed"); n != 1 {
		t.Fatalf("failed rows = %d, want 1 (setup)", n)
	}

	id, err := q.ClaimEvaluationReportGeneration(ctx, projectID)
	if err != nil {
		t.Fatalf("re-claim of a failed row must succeed, got: %v", err)
	}
	if id == uuid.Nil {
		t.Fatalf("re-claim returned a nil id")
	}
	if n := countEvaluationReportRows(t, pool, lifecycleTestProjectID, "generating"); n != 1 {
		t.Fatalf("generating rows after re-claim = %d, want 1", n)
	}
}

// TestClaimEvaluationReportGeneration_FreshGeneratingNotReclaimable — a
// 'generating' row younger than 30 minutes belongs to whoever is already
// running it; a second claim attempt must NOT re-claim it (pgx.ErrNoRows),
// which is exactly what prevents the double-generation race this task fixes.
func TestClaimEvaluationReportGeneration_FreshGeneratingNotReclaimable(t *testing.T) {
	pool := newLifecycleTestPool(t)
	q := sqlc.New(pool)
	projectID := uuid.MustParse(lifecycleTestProjectID)
	ctx := context.Background()

	if _, err := q.ClaimEvaluationReportGeneration(ctx, projectID); err != nil {
		t.Fatalf("initial claim: %v", err)
	}
	if n := countEvaluationReportRows(t, pool, lifecycleTestProjectID, "generating"); n != 1 {
		t.Fatalf("generating rows after initial claim = %d, want 1", n)
	}

	_, err := q.ClaimEvaluationReportGeneration(ctx, projectID)
	if !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("re-claim of a fresh generating row = %v, want pgx.ErrNoRows", err)
	}
	// Still exactly one row, still generating — the second caller did nothing.
	if n := countEvaluationReportRows(t, pool, lifecycleTestProjectID, ""); n != 1 {
		t.Fatalf("evaluation_report rows = %d, want 1 (second claim must not create a row)", n)
	}
}

// TestEvaluationReportEnvelope_ThreeStates drives GET /evaluation-report (via
// getEvaluationReport directly) through all three states: no row → JSON null,
// generating → {"status":"generating"}, ready → {"status":"ready","report":{…}}.
func TestEvaluationReportEnvelope_ThreeStates(t *testing.T) {
	pool := newLifecycleTestPool(t)
	q := sqlc.New(pool)
	a := New(Deps{Queries: q, Pool: pool})
	projectID := uuid.MustParse(lifecycleTestProjectID)
	ctx := WithUser(context.Background(), User{ID: SeedUserID})

	doGet := func() *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/projects/"+lifecycleTestProjectID+"/evaluation-report", nil).WithContext(ctx)
		req.SetPathValue("id", lifecycleTestProjectID)
		rec := httptest.NewRecorder()
		a.getEvaluationReport(rec, req)
		return rec
	}

	// 1. no row yet -> JSON null.
	rec := doGet()
	if rec.Code != http.StatusOK {
		t.Fatalf("GET (no row) = %d, want 200; body=%s", rec.Code, rec.Body)
	}
	if strings.TrimSpace(rec.Body.String()) != "null" {
		t.Fatalf("GET (no row) body = %s, want literal null", rec.Body)
	}

	// 2. claim -> status='generating' -> envelope {"status":"generating"}.
	if _, err := q.ClaimEvaluationReportGeneration(context.Background(), projectID); err != nil {
		t.Fatalf("claim: %v", err)
	}
	rec = doGet()
	if rec.Code != http.StatusOK {
		t.Fatalf("GET (generating) = %d, want 200; body=%s", rec.Code, rec.Body)
	}
	var genBody struct {
		Status string `json:"status"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &genBody); err != nil {
		t.Fatalf("decode generating envelope: %v — body=%s", err, rec.Body)
	}
	if genBody.Status != "generating" {
		t.Fatalf("GET (generating) status = %q, want generating; body=%s", genBody.Status, rec.Body)
	}

	// 3. complete -> status='ready' -> envelope {"status":"ready","report":{…}}.
	rawReport := []byte(`{"version":1,"reportId":"r1","projectId":"` + lifecycleTestProjectID + `","student":{"id":"s1"},"generatedAt":"2026-08-14T00:00:00Z"}`)
	if err := q.CompleteEvaluationReport(context.Background(), sqlc.CompleteEvaluationReportParams{
		ProjectID: projectID, Report: rawReport,
	}); err != nil {
		t.Fatalf("complete: %v", err)
	}
	rec = doGet()
	if rec.Code != http.StatusOK {
		t.Fatalf("GET (ready) = %d, want 200; body=%s", rec.Code, rec.Body)
	}
	var readyBody struct {
		Status string `json:"status"`
		Report struct {
			ProjectID string `json:"projectId"`
		} `json:"report"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &readyBody); err != nil {
		t.Fatalf("decode ready envelope: %v — body=%s", err, rec.Body)
	}
	if readyBody.Status != "ready" {
		t.Fatalf("GET (ready) status = %q, want ready; body=%s", readyBody.Status, rec.Body)
	}
	if readyBody.Report.ProjectID != lifecycleTestProjectID {
		t.Fatalf("GET (ready) report.projectId = %q, want %q; body=%s", readyBody.Report.ProjectID, lifecycleTestProjectID, rec.Body)
	}
}
