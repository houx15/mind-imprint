package api_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// TestCreateWriting_RollsBackEntirelyOnFailure — the atomicity claim, executed
// rather than reasoned about.
//
// createWriting writes three rows (atom, writing, and atom_message seq=1)
// inside one transaction, and every writing task after this one reuses that
// shape. The happy path is well covered — the sibling tests assert all three
// rows land. But "all three or none" is a claim about the FAILURE path, and a
// rollback nobody has watched happen is not a guarantee, it is a comment.
//
// So force a failure at the LAST of the three writes: a BEFORE INSERT trigger
// on atom_message raises, which fails AppendAtomMessage after CreateAtom and
// CreateWriting have already succeeded inside the transaction. That is exactly
// the window where a missing rollback stranded rows would matter — the student
// would meet a titled writing whose conversation had lost the very sentence she
// opened it with.
//
// A trigger rather than a stubbed queries layer, deliberately: this drives the
// REAL handler against a REAL Postgres, so it also proves the deferred Rollback
// reaches the database. An injected error could never show that.
//
// WHAT FAILURE LOOKS LIKE (verified by deleting the handler's `defer
// tx.Rollback` and re-running). You get the diagnostic first:
//
//	writings_rollback_test.go: failed create left 1 transaction(s) open —
//	the deferred Rollback did not run, so the connection leaked
//
// and then the run HANGS until the -timeout fires, reporting a timeout panic
// on top. The hang is the BUG's doing, not this test's: a connection that is
// never released keeps pgxpool.Pool.Close waiting forever at teardown, so any
// test touching this path would hang once the rollback is gone. Read the
// assertion message above the panic and ignore the panic itself — and note
// that a leak severe enough to wedge teardown is exactly why this matters in
// production, where it exhausts the pool and takes the room down for everyone.
func TestCreateWriting_RollsBackEntirelyOnFailure(t *testing.T) {
	h, cookie, _, pool := liteHandler(t)
	ctx := context.Background()

	countRows := func(what, query string) int {
		t.Helper()
		var n int
		if err := pool.QueryRow(ctx, query).Scan(&n); err != nil {
			t.Fatalf("count %s: %v", what, err)
		}
		return n
	}
	const atomsQ = `SELECT count(*) FROM atom WHERE kind = 'writing'`
	const writingsQ = `SELECT count(*) FROM writing`

	// A successful create first, so the counts below are proving that a FAILED
	// create adds nothing — not merely that an empty table stayed empty.
	_ = createWritingAtomHTTP(t, h, cookie, "这一篇会成功")
	atomsBefore := countRows("atoms", atomsQ)
	writingsBefore := countRows("writings", writingsQ)

	if _, err := pool.Exec(ctx, `
		CREATE FUNCTION mi_test_block_atom_message() RETURNS trigger AS $$
		BEGIN RAISE EXCEPTION 'mi_test: atom_message blocked'; END;
		$$ LANGUAGE plpgsql;
		CREATE TRIGGER mi_test_block_atom_message
			BEFORE INSERT ON atom_message
			FOR EACH ROW EXECUTE FUNCTION mi_test_block_atom_message();
	`); err != nil {
		t.Fatalf("install blocking trigger: %v", err)
	}
	defer func() {
		// A DEADLINE on the cleanup, and a logged rather than fatal error, on
		// purpose — this is load-bearing, not defensive habit.
		//
		// DROP TRIGGER needs ACCESS EXCLUSIVE on atom_message, and a
		// transaction left open by the very regression this test guards still
		// holds its locks. t.Fatalf runs deferred funcs on its way out, so an
		// unbounded cleanup blocks FOREVER at the exact moment the test has
		// just caught a real bug — the assertion fires, then the process hangs
		// until the suite timeout and reports itself as a timeout panic
		// instead of the clear message above. That is how a good test gets
		// written off as flaky and skipped.
		//
		// (A `SET LOCAL lock_timeout` in the same Exec does NOT work here — it
		// did not survive to the DROP. A context deadline is enforced by pgx
		// itself and does not depend on statement scoping.)
		//
		// The pool is a per-test container discarded moments later, so failing
		// to drop the trigger costs nothing.
		cleanupCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		if _, err := pool.Exec(cleanupCtx, `
			DROP TRIGGER IF EXISTS mi_test_block_atom_message ON atom_message;
			DROP FUNCTION IF EXISTS mi_test_block_atom_message();
		`); err != nil {
			t.Logf("cleanup: could not drop the blocking trigger (harmless, container is discarded): %v", err)
		}
	}()

	rec := httptest.NewRecorder()
	body := strings.NewReader(`{"idea":"这一篇会失败","lang":"zh"}`)
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("POST", "/api/v1/writings", body), cookie))

	if rec.Code == http.StatusCreated {
		t.Fatalf("create returned 201 even though the atom_message insert failed; body=%s", rec.Body)
	}

	// THE load-bearing assertion — and the one this test originally lacked.
	//
	// Row counts alone CANNOT prove a rollback happened. Postgres MVCC hides
	// uncommitted rows from every other connection, so "the rows are not
	// there" reads exactly the same whether the transaction rolled back or is
	// still sitting open holding them. The first version of this test asserted
	// only the counts, and when the rollback was deleted to check its teeth it
	// still "passed" its assertions — it failed the run merely by hanging in
	// cleanup. A test that catches a bug by timing out is a test that will be
	// misread as flaky and skipped.
	//
	// So assert the transaction is actually CLOSED. With the deferred Rollback
	// there is no such connection; without it, the handler's connection sits in
	// 'idle in transaction (aborted)' — a leak that, repeated per failed
	// request, eventually exhausts the pool and takes the room down for
	// everyone, which is a worse outcome than the orphaned rows below.
	var open int
	if err := pool.QueryRow(ctx, `
		SELECT count(*) FROM pg_stat_activity
		WHERE datname = current_database()
		  AND state IN ('idle in transaction', 'idle in transaction (aborted)')
	`).Scan(&open); err != nil {
		t.Fatalf("count open transactions: %v", err)
	}
	if open != 0 {
		t.Fatalf("failed create left %d transaction(s) open — the deferred Rollback did not run, so the connection leaked", open)
	}

	// And the data invariant itself: the two rows written BEFORE the failing
	// one must be gone, never a titled writing with no opening line.
	if got := countRows("writings", writingsQ); got != writingsBefore {
		t.Fatalf("failed create left a writing row behind: want %d, got %d — the transaction did not roll back",
			writingsBefore, got)
	}
	if got := countRows("atoms", atomsQ); got != atomsBefore {
		t.Fatalf("failed create left an orphan atom row: want %d, got %d — the transaction did not roll back",
			atomsBefore, got)
	}
}
