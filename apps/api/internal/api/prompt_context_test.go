package api

import (
	"strings"
	"testing"

	"mindimprint/api/internal/promptassembly"
	"mindimprint/api/internal/store/sqlc"
)

func checkPromptDocument(t *testing.T, d promptassembly.Document) {
	t.Helper()
	end := 0
	for _, s := range d.Sections {
		if s.Start != end || s.End <= s.Start || s.End > len(d.Text) || s.Source == "" {
			t.Fatalf("invalid section %#v", s)
		}
		end = s.End
	}
	if end != len(d.Text) {
		t.Fatal("document has unattributed text")
	}
}

func TestReadingContextBudgetRetainsOmissionMarkers(t *testing.T) {
	blocks := []Block{{ID: "b1", Text: strings.Repeat("字", readingPlanArticleRuneBudget+1)}, {ID: "b2", Text: "保留的短段"}, {ID: "b3", Text: "  "}}
	var history []sqlc.AtomMessage
	for i := 0; i < 20; i++ {
		history = append(history, sqlc.AtomMessage{Role: "student", Content: "观察"})
	}
	c := selectReadingCoachContext("测试", blocks, readingOutline{}, nil, history, nil, "继续", nil, "")
	if len(c.History) != readingCoachTurnsWindow || len(c.Article) != 2 {
		t.Fatal("selection changed")
	}
	if c.Article[0].Omitted != "budget" || c.Article[0].Text != "" || c.Article[1].Text != "保留的短段" {
		t.Fatal("must skip whole oversized paragraph and continue")
	}
	d := renderReadingCoachPrompt(c)
	checkPromptDocument(t, d)
	if !strings.Contains(d.Text, "这一段没放进来，但它存在") || !strings.Contains(d.Text, "保留的短段") {
		t.Fatal("omission is represented as absence")
	}
	if history[0].Content != "观察" || len(history) != 20 || blocks[0].Text != strings.Repeat("字", readingPlanArticleRuneBudget+1) {
		t.Fatal("source mutated")
	}
}

func TestWritingPlanSelectionAndConditionalInstructions(t *testing.T) {
	wr := sqlc.Writing{Title: "图书馆", Lang: "zh"}
	var history []sqlc.AtomMessage
	for i := 0; i < 20; i++ {
		history = append(history, sqlc.AtomMessage{Role: "student", Content: "我有具体理由"})
	}
	c := selectWritingPlanContext(wr, nil, history, "请替我写")
	if len(c.History) != writingPlanTurnsWindow || !c.AskedToDoIt {
		t.Fatal("selection/state signal changed")
	}
	d := renderWritingPlanPrompt(c)
	checkPromptDocument(t, d)
	found := false
	for _, s := range d.Sections {
		if s.ID == "delegation-request" {
			found = true
		}
	}
	if !found {
		t.Fatal("triggered instruction not traceable")
	}
	d = renderWritingPlanPrompt(selectWritingPlanContext(wr, nil, history, "我认为图书馆应该延长开放时间"))
	checkPromptDocument(t, d)
	for _, s := range d.Sections {
		if s.ID == "delegation-request" {
			t.Fatal("inactive rule included")
		}
	}
}
