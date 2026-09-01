package api_test

import (
	"context"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"mindimprint/api/internal/store/sqlc"
)

// appendOnce is the exact transaction shape every append path uses: lock the
// atom, read the next seq, insert. Extracted here so the race can be driven
// concurrently without a model call in the way.
func appendOnce(ctx context.Context, pool *pgxpool.Pool, atomID uuid.UUID, body string, lock bool) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	q := sqlc.New(pool).WithTx(tx)

	if lock {
		if _, err := q.LockAtom(ctx, atomID); err != nil {
			return err
		}
	}
	next, err := q.NextAtomMessageSeq(ctx, atomID)
	if err != nil {
		return err
	}
	if _, err := q.AppendPblSessionMessage(ctx, sqlc.AppendPblSessionMessageParams{
		AtomID: atomID, Seq: next, Role: "student", Content: body, SessionID: pgtype.UUID{},
	}); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func runConcurrentAppends(t *testing.T, pool *pgxpool.Pool, atomID uuid.UUID, n int, lock bool) int {
	t.Helper()
	var wg sync.WaitGroup
	errs := make([]error, n)
	start := make(chan struct{})
	for i := range n {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start // release them together, so they really do collide
			errs[i] = appendOnce(context.Background(), pool, atomID, "concurrent", lock)
		}(i)
	}
	close(start)
	wg.Wait()

	failed := 0
	for _, err := range errs {
		if err != nil {
			failed++
		}
	}
	return failed
}

// 🚨 The race this slice exists to close.
//
// NextAtomMessageSeq is a read followed by a separate insert. Under READ
// COMMITTED, concurrent transactions read the same MAX and both insert it; the
// (atom_id, seq) unique index rejects one, and in production that turn dies
// AFTER its model call has been paid for — the student's message is the thing
// that disappears.
//
// Sessions make two live conversations on one atom the designed behaviour, so
// this stopped being unreachable.
//
// VERIFIED RED, 2026-09-01: with `lock` flipped to false this test reports
// **8 of 12 appends rejected**. The race is not a corner case under real
// concurrency — it is the common outcome. Flip the flag if you ever want to see
// it again; that parameter is not dead configuration, it is the evidence that
// the lock is load-bearing.
func TestAtomSeq_ConcurrentAppendsDoNotCollide(t *testing.T) {
	_, _, q, pool := liteHandler(t)
	atomID := newPblAtom(t, q)

	const n = 12
	if failed := runConcurrentAppends(t, pool, atomID, n, true); failed != 0 {
		t.Fatalf("%d/%d concurrent appends failed even while holding LockAtom", failed, n)
	}

	rows, err := q.ListPblMainThread(t.Context(), atomID)
	if err != nil {
		t.Fatalf("ListPblMainThread: %v", err)
	}
	if len(rows) != n {
		t.Fatalf("landed %d of %d messages", len(rows), n)
	}
	seen := map[int32]bool{}
	for _, r := range rows {
		if seen[r.Seq] {
			t.Fatalf("duplicate seq %d — the unique index should have made this impossible", r.Seq)
		}
		seen[r.Seq] = true
	}
}
