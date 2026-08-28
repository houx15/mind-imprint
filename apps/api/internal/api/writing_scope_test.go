package api_test

// writing_scope_test.go — Task 1: the safety property of migration 0102. A
// sub-agent's block-scoped turn must be invisible to every reader of the
// room's own thread. ListAtomMessages itself filters block_id IS NULL, so
// every one of its seven existing callers (writing_setup.go, reading_turn.go
// x2, writing_turn.go, writing_plan.go, writing_guide.go, reading_coach.go)
// becomes correct with no edit — this test is what proves that filter is
// actually there and actually works, not just documented in a comment.

import (
	"context"
	"testing"

	"mindimprint/api/internal/store/sqlc"
)

// contents renders a slice of atom_message rows down to their text, for
// compact failure messages.
func contents(msgs []sqlc.AtomMessage) []string {
	out := make([]string, len(msgs))
	for i, m := range msgs {
		out[i] = m.Content
	}
	return out
}

func TestListAtomMessages_ExcludesBlockScopedTurns(t *testing.T) {
	ctx := context.Background()
	pool := newAPITestPool(t)
	q := sqlc.New(pool)
	at := createWritingAtom(t, q)

	if _, err := q.AppendAtomMessage(ctx, sqlc.AppendAtomMessageParams{
		AtomID: at.ID, Seq: 1, Role: "student", Content: "主线里的话",
	}); err != nil {
		t.Fatal(err)
	}
	blockID := "block-1"
	if _, err := q.AppendAtomBlockMessage(ctx, sqlc.AppendAtomBlockMessageParams{
		AtomID: at.ID, Seq: 2, Role: "student", Content: "只属于这一块的话",
		BlockID: &blockID,
	}); err != nil {
		t.Fatal(err)
	}

	main, err := q.ListAtomMessages(ctx, at.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(main) != 1 || main[0].Content != "主线里的话" {
		t.Fatalf("main thread = %d rows %v, want only the main-thread turn", len(main), contents(main))
	}

	scoped, err := q.ListAtomBlockMessages(ctx, sqlc.ListAtomBlockMessagesParams{
		AtomID: at.ID, BlockID: &blockID,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(scoped) != 1 || scoped[0].Content != "只属于这一块的话" {
		t.Fatalf("block thread = %d rows %v, want only the block turn", len(scoped), contents(scoped))
	}
}
