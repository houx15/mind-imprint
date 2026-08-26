package store_test

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"mindimprint/api/internal/store/sqlc"
)

// TestAtomSubstrate_ReadingRoundTrips — an atom carries identity, the reading
// row carries the reading's own fields, and the two are created together.
func TestAtomSubstrate_ReadingRoundTrips(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping testcontainers integration in -short mode")
	}
	pool := newStoreTestPool(t)
	q := sqlc.New(pool)
	ctx := context.Background()

	var uid uuid.UUID
	if err := pool.QueryRow(ctx, `SELECT id FROM users LIMIT 1`).Scan(&uid); err != nil {
		t.Fatalf("seed user: %v", err)
	}

	a, err := q.CreateAtom(ctx, sqlc.CreateAtomParams{Kind: "reading", UserID: uid})
	if err != nil {
		t.Fatalf("CreateAtom: %v", err)
	}
	rd, err := q.CreateReading(ctx, sqlc.CreateReadingParams{AtomID: a.ID, Title: "气候与农业", Lang: "zh"})
	if err != nil {
		t.Fatalf("CreateReading: %v", err)
	}
	if rd.Status != "active" {
		t.Fatalf("status = %q, want \"active\"", rd.Status)
	}

	list, err := q.ListReadingsByUser(ctx, uid)
	if err != nil {
		t.Fatalf("ListReadingsByUser: %v", err)
	}
	if len(list) != 1 || list[0].Title != "气候与农业" {
		t.Fatalf("list = %+v, want exactly my one reading", list)
	}
}

// TestAtomSubstrate_SharedMachineryIsPerAtom — messages, cards and annotations
// hang off the atom id, so every future kind (writing, chat, project) reuses
// them without a second implementation.
func TestAtomSubstrate_SharedMachineryIsPerAtom(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping testcontainers integration in -short mode")
	}
	pool := newStoreTestPool(t)
	q := sqlc.New(pool)
	ctx := context.Background()

	var uid uuid.UUID
	if err := pool.QueryRow(ctx, `SELECT id FROM users LIMIT 1`).Scan(&uid); err != nil {
		t.Fatalf("seed user: %v", err)
	}
	a, err := q.CreateAtom(ctx, sqlc.CreateAtomParams{Kind: "reading", UserID: uid})
	if err != nil {
		t.Fatalf("CreateAtom: %v", err)
	}

	if _, err := q.AppendAtomMessage(ctx, sqlc.AppendAtomMessageParams{
		AtomID: a.ID, Seq: 1, Role: "student", Content: "这段在讲什么？",
	}); err != nil {
		t.Fatalf("AppendAtomMessage: %v", err)
	}
	if _, err := q.AppendAtomMessage(ctx, sqlc.AppendAtomMessageParams{
		AtomID: a.ID, Seq: 2, Role: "ai", Content: "它在比较两种口径。",
	}); err != nil {
		t.Fatalf("AppendAtomMessage 2: %v", err)
	}
	msgs, err := q.ListAtomMessages(ctx, a.ID)
	if err != nil {
		t.Fatalf("ListAtomMessages: %v", err)
	}
	if len(msgs) != 2 || msgs[0].Seq != 1 || msgs[1].Role != "ai" {
		t.Fatalf("messages = %+v, want the two in seq order", msgs)
	}

	// (atom_id, seq) is unique — a duplicated seq must be rejected, so a
	// concurrent double-append can never silently reorder the transcript.
	if _, err := q.AppendAtomMessage(ctx, sqlc.AppendAtomMessageParams{
		AtomID: a.ID, Seq: 2, Role: "student", Content: "重复",
	}); err == nil {
		t.Fatal("duplicate seq accepted, want a unique-violation")
	}
}

// TestAtomSubstrate_CascadesFromAtom — deleting the atom takes its whole world
// with it; no orphaned messages, cards or annotations.
func TestAtomSubstrate_CascadesFromAtom(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping testcontainers integration in -short mode")
	}
	pool := newStoreTestPool(t)
	q := sqlc.New(pool)
	ctx := context.Background()

	var uid uuid.UUID
	if err := pool.QueryRow(ctx, `SELECT id FROM users LIMIT 1`).Scan(&uid); err != nil {
		t.Fatalf("seed user: %v", err)
	}
	a, err := q.CreateAtom(ctx, sqlc.CreateAtomParams{Kind: "reading", UserID: uid})
	if err != nil {
		t.Fatalf("CreateAtom: %v", err)
	}
	if _, err := q.CreateReading(ctx, sqlc.CreateReadingParams{AtomID: a.ID, Title: "t", Lang: "zh"}); err != nil {
		t.Fatalf("CreateReading: %v", err)
	}
	if _, err := q.AppendAtomMessage(ctx, sqlc.AppendAtomMessageParams{
		AtomID: a.ID, Seq: 1, Role: "student", Content: "x",
	}); err != nil {
		t.Fatalf("AppendAtomMessage: %v", err)
	}

	if _, err := pool.Exec(ctx, `DELETE FROM atom WHERE id = $1`, a.ID); err != nil {
		t.Fatalf("delete atom: %v", err)
	}
	var n int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM atom_message WHERE atom_id = $1`, a.ID).Scan(&n); err != nil {
		t.Fatalf("count messages: %v", err)
	}
	if n != 0 {
		t.Fatalf("orphaned %d messages after atom delete", n)
	}
}
