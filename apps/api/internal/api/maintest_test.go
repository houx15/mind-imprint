package api_test

// maintest_test.go — shared test helpers for Tasks 6-9.
// Duplicates the testcontainers bootstrap from internal/store/sqlc_test.go
// because that helper lives in package store_test and is not importable here.

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	. "mindimprint/api/internal/api"
	"mindimprint/api/internal/auth"
	"mindimprint/api/internal/store"
	"mindimprint/api/internal/store/sqlc"
)

// newAPITestPool spins up a throwaway Postgres, runs all migrations (incl. seed),
// and returns the pool. Container/pool torn down via t.Cleanup.
func newAPITestPool(t *testing.T) *pgxpool.Pool {
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

// signInSeed creates a live session for the seeded student and returns the
// cookie to attach to authed requests (replaces the implicit ActAsSeed inject).
func signInSeed(t *testing.T, pool *pgxpool.Pool) *http.Cookie {
	return signInAs(t, pool, SeedUserID)
}

// signInAs creates a live session for an arbitrary user and returns its cookie.
func signInAs(t *testing.T, pool *pgxpool.Pool, userID uuid.UUID) *http.Cookie {
	t.Helper()
	q := sqlc.New(pool)
	raw, hash, err := auth.NewToken()
	if err != nil {
		t.Fatalf("token: %v", err)
	}
	if _, err := q.CreateSession(context.Background(), sqlc.CreateSessionParams{
		UserID: userID, TokenHash: hash, ExpiresAt: time.Now().Add(time.Hour),
	}); err != nil {
		t.Fatalf("create session: %v", err)
	}
	return &http.Cookie{Name: "mk_session", Value: raw}
}

func signInAdmin(t *testing.T, pool *pgxpool.Pool) *http.Cookie {
	return signInAs(t, pool, SeedAdminID)
}

// createTeacher inserts a verified teacher in schoolID and returns its id.
func createTeacher(t *testing.T, pool *pgxpool.Pool, schoolID uuid.UUID, email string) uuid.UUID {
	t.Helper()
	q := sqlc.New(pool)
	u, err := q.CreateUser(context.Background(), sqlc.CreateUserParams{
		Email: email, PasswordHash: "x", Role: "teacher", SchoolID: schoolID,
		DisplayName: "T " + email, AvatarColor: "#888888",
		EmailVerifiedAt: pgtype.Timestamptz{Time: time.Now(), Valid: true},
	})
	if err != nil {
		t.Fatalf("create teacher: %v", err)
	}
	return u.ID
}

// withCookie attaches c to req and returns it (for inline request building).
func withCookie(req *http.Request, c *http.Cookie) *http.Request {
	req.AddCookie(c)
	return req
}

// newTestAPI builds a minimal *API wired to the given pool (no gateway/catalog).
func newTestAPI(pool *pgxpool.Pool) *API {
	return New(Deps{Queries: sqlc.New(pool), Pool: pool, CookieSecure: false})
}

// mustUUID parses a UUID string and panics on error (test-only convenience).
func mustUUID(s string) uuid.UUID {
	return uuid.MustParse(s)
}
