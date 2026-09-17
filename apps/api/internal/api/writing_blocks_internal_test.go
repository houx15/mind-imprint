package api

import (
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"mindimprint/api/internal/store/sqlc"
)

// Same cases as apps/lite-web/src/writings/slots.test.ts — the two sides must
// number blocks the same way.
func TestWritingBlockNumbersFoldsMaterial(t *testing.T) {
	id := func() uuid.UUID { return uuid.New() }
	tID, aID, a1ID, bID := id(), id(), id(), id()
	outline := []sqlc.WritingOutline{
		{ID: tID, Depth: 0, Position: 0, Text: "论点"},
		{ID: aID, Depth: 1, Position: 1, Text: "理由A"},
		{ID: a1ID, Depth: 2, Position: 2, Text: "例子a"},
		{ID: bID, Depth: 1, Position: 3, Text: "理由B"},
	}
	link := func(o uuid.UUID) pgtype.UUID { return pgtype.UUID{Bytes: o, Valid: true} }
	sT := sqlc.WritingSnippet{ID: id(), OutlineID: link(tID), Position: 0, Text: "论点段"}
	sB := sqlc.WritingSnippet{ID: id(), OutlineID: link(bID), Position: 3, Text: "B段"}
	free := sqlc.WritingSnippet{ID: id(), Position: 9, Text: "加的一段"}

	got := writingBlockNumbers(outline, []sqlc.WritingSnippet{sT, sB, free})
	if got[sT.ID] != 1 || got[sB.ID] != 3 || got[free.ID] != 4 {
		t.Fatalf("numbers = %v; want 论点=1 B=3 free=4 (例子a folded)", got)
	}

	// A material node she already wrote on keeps its block.
	sA1 := sqlc.WritingSnippet{ID: id(), OutlineID: link(a1ID), Position: 2, Text: "例子段"}
	got = writingBlockNumbers(outline, []sqlc.WritingSnippet{sT, sA1, sB})
	if got[sA1.ID] != 3 || got[sB.ID] != 4 {
		t.Fatalf("numbers = %v; want 例子=3 B=4", got)
	}
}
