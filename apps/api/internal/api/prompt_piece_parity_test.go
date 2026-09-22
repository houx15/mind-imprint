package api

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
	"mindimprint/api/internal/store/sqlc"
)

// This fixture is also run against main 03c31b99 before writing the hashes.
// It protects Unicode truncation, block ordering, assigned-task provenance and
// feedback state for all three consumers of the shared piece context.
func TestWritingPieceRequestParity(t *testing.T) {
	var rows []sqlc.WritingOutline
	var snippets []sqlc.WritingSnippet
	for i := 0; i < 14; i++ {
		row := sqlc.WritingOutline{ID: fixtureTaskID(byte(i + 1)), Kind: writingKindPoint, Depth: 1, Position: int32(i), Text: fmt.Sprintf("理由%d", i)}
		if i == 0 {
			row.Kind = writingKindThesis
			row.Depth = 0
		}
		rows = append(rows, row)
		snippets = append(snippets, sqlc.WritingSnippet{ID: fixtureTaskID(byte(i + 101)), OutlineID: pgtype.UUID{Bytes: row.ID, Valid: true}, Text: strings.Repeat("甲🦉", 200) + fmt.Sprint(i)})
	}
	comments := []sqlc.WritingComment{{Scope: "block", SnippetID: pgtype.UUID{Bytes: snippets[1].ID, Valid: true}, SourceText: "旧版正文", Points: []byte(`[{"kind":"issue","text":"解释材料与理由的关系","action":"说明这个例子能证明什么"}]`)}}
	assigned := "根据两份材料讨论图书馆开放时间。"
	got := map[string]string{}
	for _, lang := range []string{"zh", "en"} {
		wr := sqlc.Writing{Title: "图书馆", Lang: lang, AssignedPrompt: &assigned}
		for _, c := range []struct {
			id       string
			outline  []sqlc.WritingOutline
			snippets []sqlc.WritingSnippet
			comments []sqlc.WritingComment
			focus    *sqlc.WritingOutline
		}{
			{id: "empty"},
			{id: "truncated", outline: rows, snippets: snippets, comments: comments, focus: &rows[1]},
			{id: "missing-body", outline: rows[:3], focus: &rows[0]},
			{id: "free", outline: rows[:3], snippets: snippets[:3]},
		} {
			text := buildWritingPieceContext(wr, c.outline, c.snippets, c.comments, c.focus)
			got[lang+"/"+c.id] = fmt.Sprintf("%x", sha256.Sum256([]byte(text)))
		}
	}
	if path := os.Getenv("PROMPT_PIECE_BASELINE_OUT"); path != "" {
		b, err := json.MarshalIndent(got, "", "  ")
		if err != nil {
			t.Fatal(err)
		}
		if err = os.WriteFile(path, append(b, '\n'), 0644); err != nil {
			t.Fatal(err)
		}
		return
	}
	b, err := os.ReadFile("testdata/prompt_piece_hashes.json")
	if err != nil {
		t.Fatal(err)
	}
	var want map[string]string
	if err = json.Unmarshal(b, &want); err != nil {
		t.Fatal(err)
	}
	if len(want) != len(got) {
		t.Fatal("fixture set changed")
	}
	for id, hash := range got {
		if want[id] != hash {
			t.Errorf("piece context changed from main: %s", id)
		}
	}
}
