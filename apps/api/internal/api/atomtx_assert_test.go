package api_test

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// assertNoOpenTransactions fails unless every connection has closed its
// transaction. This is the assertion that actually distinguishes "rolled back"
// from "still open and holding the rows invisible" — row counts cannot, because
// MVCC hides uncommitted rows from other connections either way.
//
// It POLLS rather than sampling once, and that is not defensive padding: it
// removes a real false-failure mode. pg_stat_activity is a statistics view each
// backend updates as it changes state, so a connection that has just committed
// or rolled back can still read as 'idle in transaction' for a moment
// afterwards. A single sample therefore fails at random — and a guard that
// fails at random is one somebody eventually marks flaky and skips, which costs
// the guarantee entirely. (A reviewer did observe exactly one such
// non-reproducing failure before this was added.)
//
// Polling costs the teeth nothing. A genuinely leaked transaction never closes,
// so it is still there at the deadline; only the reporting lag resolves, and it
// resolves in milliseconds.
func assertNoOpenTransactions(t *testing.T, ctx context.Context, pool *pgxpool.Pool, what string) {
	t.Helper()
	const settle = 3 * time.Second
	deadline := time.Now().Add(settle)
	for {
		var open int
		if err := pool.QueryRow(ctx, `
			SELECT count(*) FROM pg_stat_activity
			WHERE datname = current_database()
			  AND state IN ('idle in transaction', 'idle in transaction (aborted)')
		`).Scan(&open); err != nil {
			t.Fatalf("count open transactions: %v", err)
		}
		if open == 0 {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("%s left %d transaction(s) open after %s — the deferred Rollback did not run, so the connection leaked",
				what, open, settle)
		}
		time.Sleep(50 * time.Millisecond)
	}
}
