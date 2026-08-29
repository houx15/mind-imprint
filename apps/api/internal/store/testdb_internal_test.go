package store

// testdb_internal_test.go — the package's Postgres fixture.
//
// Before (2026-08-30): every helper that wanted a database started its own
// testcontainer and replayed ~106 migrations into it — ~60 containers per
// `go test ./internal/store/` run, at a couple of seconds each.
//
// Now: ONE container per package run. Migrations run ONCE into a template
// database; each call then does `CREATE DATABASE t_<n> TEMPLATE <template>`,
// which Postgres implements as a file copy (tens of milliseconds). Every test
// still gets its own pristine, isolated database, so no test semantics change
// and no call site had to be touched. Migration-targeted tests that need an
// UNmigrated database get one from newBareTestDB, cloned from template0.
//
// The helper lives in package `store` (not `store_test`) so both test packages
// compiled into this binary can share the single container: `store_test`
// imports this package and calls the exported NewTestDB.

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

// templateDBName is the migrated database every per-test database is cloned
// from. Nothing may ever hold a connection to it: CREATE DATABASE ... TEMPLATE
// refuses while another session is attached.
const templateDBName = "mi_template"

var (
	sharedPGOnce  sync.Once
	sharedPG      *tcpostgres.PostgresContainer
	sharedAdmin   *pgxpool.Pool // connected to the bootstrap db, never to the template
	sharedBaseDSN string        // DSN of the bootstrap db; per-test DSNs swap the path
	sharedPGErr   error

	cloneMu sync.Mutex    // serialises CREATE/DROP DATABASE against the template
	dbSeq   atomic.Uint64 // per-process counter → unique, always-valid identifiers
)

// TestMain keeps the shared container alive for the whole package run and
// tears it down once at the end. (testcontainers' Ryuk reaper still cleans up
// if the process dies without getting here.)
func TestMain(m *testing.M) {
	code := m.Run()
	if sharedPG != nil {
		if sharedAdmin != nil {
			sharedAdmin.Close()
		}
		_ = sharedPG.Terminate(context.Background())
	}
	os.Exit(code)
}

// startSharedPostgres boots the one container and builds the migrated
// template. Runs exactly once; errors are latched into sharedPGErr so callers
// can fail their own test with them.
func startSharedPostgres() {
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
		sharedPGErr = fmt.Errorf("start postgres: %w", err)
		return
	}
	sharedPG = pg

	dsn, err := pg.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		sharedPGErr = fmt.Errorf("dsn: %w", err)
		return
	}
	sharedBaseDSN = dsn

	admin, err := NewPool(ctx, dsn)
	if err != nil {
		sharedPGErr = fmt.Errorf("admin pool: %w", err)
		return
	}
	sharedAdmin = admin

	if _, err := admin.Exec(ctx, `CREATE DATABASE `+templateDBName); err != nil {
		sharedPGErr = fmt.Errorf("create template db: %w", err)
		return
	}

	// Migrate the template through a pool we close immediately afterwards —
	// from here on nothing connects to the template again.
	tmplDSN, err := dsnForDB(dsn, templateDBName)
	if err != nil {
		sharedPGErr = err
		return
	}
	tmplPool, err := NewPool(ctx, tmplDSN)
	if err != nil {
		sharedPGErr = fmt.Errorf("template pool: %w", err)
		return
	}
	migErr := RunMigrations(ctx, tmplPool)
	tmplPool.Close()
	if migErr != nil {
		sharedPGErr = fmt.Errorf("migrate template: %w", migErr)
	}
}

// dsnForDB rewrites base's database path to name.
func dsnForDB(base, name string) (string, error) {
	u, err := url.Parse(base)
	if err != nil {
		return "", fmt.Errorf("parse dsn: %w", err)
	}
	u.Path = "/" + name
	return u.String(), nil
}

// NewTestDB returns a pool onto a freshly cloned, fully migrated database —
// private to the calling test and dropped in t.Cleanup.
//
// Exported so the external store_test package can share the same container.
// Test-only.
func NewTestDB(t *testing.T) *pgxpool.Pool {
	t.Helper()
	return newDBFromTemplate(t, templateDBName)
}

// newBareTestDB returns a pool onto a fresh EMPTY database — no migrations —
// for the migration-targeted tests that drive goose to a specific version.
// "" means the cluster default template (template1), i.e. an empty database.
func newBareTestDB(t *testing.T) *pgxpool.Pool {
	t.Helper()
	return newDBFromTemplate(t, "")
}

func newDBFromTemplate(t *testing.T, template string) *pgxpool.Pool {
	t.Helper()
	sharedPGOnce.Do(startSharedPostgres)
	if sharedPGErr != nil {
		t.Fatalf("shared postgres: %v", sharedPGErr)
	}
	ctx := context.Background()

	name := fmt.Sprintf("t_%d", dbSeq.Add(1))
	stmt := `CREATE DATABASE ` + name
	if template != "" {
		stmt += ` TEMPLATE ` + template
	}
	cloneMu.Lock()
	_, err := sharedAdmin.Exec(ctx, stmt)
	cloneMu.Unlock()
	if err != nil {
		t.Fatalf("create database %s (template %q): %v", name, template, err)
	}

	dsn, err := dsnForDB(sharedBaseDSN, name)
	if err != nil {
		t.Fatalf("dsn: %v", err)
	}
	pool, err := NewPool(ctx, dsn)
	if err != nil {
		t.Fatalf("pool: %v", err)
	}
	t.Cleanup(func() {
		pool.Close()
		cloneMu.Lock()
		defer cloneMu.Unlock()
		// Best-effort: keeps the container's disk from growing by one full
		// database copy per test.
		_, _ = sharedAdmin.Exec(context.Background(), `DROP DATABASE IF EXISTS `+name+` WITH (FORCE)`)
	})
	return pool
}
