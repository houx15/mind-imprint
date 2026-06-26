package api_test

// maintest_test.go — shared test helpers for Tasks 6-9.
// Duplicates the testcontainers bootstrap from internal/store/sqlc_test.go
// because that helper lives in package store_test and is not importable here.

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
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

// mustNewQueries returns a *sqlc.Queries wired to pool (test helper).
func mustNewQueries(pool *pgxpool.Pool) *sqlc.Queries {
	return sqlc.New(pool)
}

// createClassViaAPI POSTs /api/v1/classes with the given name and returns the new class id.
func createClassViaAPI(t *testing.T, h http.Handler, cookie *http.Cookie, name string) string {
	t.Helper()
	body, _ := json.Marshal(map[string]any{"name": name})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("POST", "/api/v1/classes", bytes.NewReader(body)), cookie))
	if rec.Code != http.StatusCreated {
		t.Fatalf("createClassViaAPI got %d body=%s", rec.Code, rec.Body)
	}
	var resp struct {
		Class struct {
			ID string `json:"id"`
		} `json:"class"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("createClassViaAPI decode: %v", err)
	}
	return resp.Class.ID
}

// enrollStudent directly inserts a student enrollment into the DB.
func enrollStudent(t *testing.T, pool *pgxpool.Pool, userID uuid.UUID, classID string) {
	t.Helper()
	q := sqlc.New(pool)
	if _, err := q.CreateEnrollment(context.Background(), sqlc.CreateEnrollmentParams{
		UserID:      userID,
		ClassID:     uuid.MustParse(classID),
		RoleInClass: "student",
	}); err != nil {
		t.Fatalf("enrollStudent: %v", err)
	}
}

// seedTaskFor inserts a dummy task for the given user.
func seedTaskFor(t *testing.T, pool *pgxpool.Pool, userID uuid.UUID) {
	t.Helper()
	q := sqlc.New(pool)
	if _, err := q.CreateTask(context.Background(), sqlc.CreateTaskParams{
		UserID: userID, Title: "seed task",
	}); err != nil {
		t.Fatalf("seedTaskFor: %v", err)
	}
}

// seedSecondSchool inserts a second school and returns its id.
func seedSecondSchool(t *testing.T, pool *pgxpool.Pool) uuid.UUID {
	t.Helper()
	id := uuid.New()
	if _, err := pool.Exec(context.Background(),
		`INSERT INTO schools (id, name) VALUES ($1, $2)`, id, "Other School"); err != nil {
		t.Fatalf("seedSecondSchool: %v", err)
	}
	return id
}

// signInViaAPI signs in via the real POST /api/v1/auth/signin endpoint and
// returns the mk_session cookie from the Set-Cookie response header.
func signInViaAPI(t *testing.T, h http.Handler, email, password string) *http.Cookie {
	t.Helper()
	body, _ := json.Marshal(map[string]any{"email": email, "password": password})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("POST", "/api/v1/auth/signin", bytes.NewReader(body)))
	if rec.Code != http.StatusOK {
		t.Fatalf("signInViaAPI: want 200, got %d — body: %s", rec.Code, rec.Body)
	}
	for _, c := range rec.Result().Cookies() {
		if c.Name == "mk_session" {
			return c
		}
	}
	t.Fatalf("signInViaAPI: mk_session cookie not found in response")
	return nil
}
