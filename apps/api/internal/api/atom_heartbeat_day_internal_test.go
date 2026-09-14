package api

// atom_heartbeat_day_internal_test.go — Task 1's own test: recordHeartbeat
// must land the same clamped amount in BOTH atom.active_seconds (the running
// total) and today's row of atom_active_day (the per-Beijing-day bucket
// later tasks sum for "minutes this week"), in one transaction, split
// correctly across the Beijing midnight boundary.
//
// package api (not api_test): recordHeartbeat is unexported.

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"mindimprint/api/internal/liteweek"
	"mindimprint/api/internal/store/sqlc"
)

// newHeartbeatTestAtom builds an *API against a fresh migrated database and a
// bare reading atom owned by SeedUserID, mirroring createReadingAtomRow
// (atom_loader_test.go) but staying in package api since recordHeartbeat is
// unexported.
func newHeartbeatTestAtom(t *testing.T) (*API, *pgxpool.Pool, uuid.UUID) {
	t.Helper()
	pool := NewTestDB(t)
	q := sqlc.New(pool)
	ctx := context.Background()

	at, err := q.CreateAtom(ctx, sqlc.CreateAtomParams{Kind: "reading", UserID: SeedUserID})
	if err != nil {
		t.Fatalf("create reading atom: %v", err)
	}
	if _, err := q.CreateReading(ctx, sqlc.CreateReadingParams{
		AtomID: at.ID, Title: "一篇文章", Lang: "zh",
	}); err != nil {
		t.Fatalf("create reading row: %v", err)
	}

	a := New(Deps{Queries: q, Pool: pool})
	return a, pool, at.ID
}

func TestRecordHeartbeatSplitsAcrossBeijingMidnight(t *testing.T) {
	a, pool, atomID := newHeartbeatTestAtom(t)
	ctx := context.Background()

	if err := a.recordHeartbeat(ctx, atomID, 60, time.Date(2026, 9, 14, 23, 59, 30, 0, liteweek.Beijing)); err != nil {
		t.Fatal(err)
	}
	if err := a.recordHeartbeat(ctx, atomID, 500, time.Date(2026, 9, 15, 0, 0, 30, 0, liteweek.Beijing)); err != nil {
		t.Fatal(err)
	}

	rows, err := pool.Query(ctx, `SELECT day::text, seconds FROM atom_active_day WHERE atom_id=$1 ORDER BY day`, atomID)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	got := map[string]int32{}
	for rows.Next() {
		var d string
		var s int32
		if err := rows.Scan(&d, &s); err != nil {
			t.Fatal(err)
		}
		got[d] = s
	}
	if got["2026-09-14"] != 60 || got["2026-09-15"] != 120 {
		t.Fatalf("buckets = %v, want 09-14:60 09-15:120 (second call clamped)", got)
	}
	var total int32
	if err := pool.QueryRow(ctx, `SELECT active_seconds FROM atom WHERE id=$1`, atomID).Scan(&total); err != nil {
		t.Fatal(err)
	}
	if total != 180 {
		t.Fatalf("atom.active_seconds = %d, want 180", total)
	}
}
