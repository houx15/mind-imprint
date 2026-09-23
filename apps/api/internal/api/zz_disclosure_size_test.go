package api

import (
	"fmt"
	"strings"
	"testing"

	"mindimprint/api/internal/store/sqlc"
)

// Throwaway: how much does progressive disclosure remove at PRODUCTION scale?
//
// 🚨 benchReadingArticle is 286 runes — a toy. Production readings average
// 11,035 input tokens per coach call, i.e. thousands of runes of article. Any
// size conclusion drawn from the bench fixture is meaningless; this builds an
// article of a realistic shape instead.
func TestDisclosurePromptSize(t *testing.T) {
	enableDisclosure(t)
	// 9 paragraphs x ~700 runes — a real graded-reader article.
	var sb strings.Builder
	for i := 1; i <= 9; i++ {
		sb.WriteString(fmt.Sprintf("第%d段。", i))
		sb.WriteString(strings.Repeat("这是文章正文的一句话，它有足够的长度。", 35))
		sb.WriteString("\n\n")
	}
	article := sb.String()
	blocks := SplitBlocks(article)
	if len(blocks) < 9 {
		t.Fatalf("expected 9 blocks, got %d", len(blocks))
	}

	parts := []readingPart{
		{From: blocks[0].ID, To: blocks[2].ID, Title: "第一部分", Does: "提出问题"},
		{From: blocks[3].ID, To: blocks[5].ID, Title: "第二部分", Does: "给出证据"},
		{From: blocks[6].ID, To: blocks[8].ID, Title: "第三部分", Does: "回应反例"},
	}
	tasks := []sqlc.ReadingTask{
		{ID: fixtureTaskID(1), Position: 1, Kind: string(taskRead), BlockID: blocks[0].ID, Status: "done"},
		{ID: fixtureTaskID(2), Position: 2, Kind: string(taskRead), BlockID: blocks[3].ID, Status: "pending"},
	}
	student := "我觉得这几段是在给证据。"

	full := buildReadingCoachPrompt("测试文章", blocks, readingOutline{Parts: parts},
		[]sqlc.ReadingTask{{ID: fixtureTaskID(1), Position: 1,
			Kind: string(taskReflect), Status: "pending"}}, nil, nil, student, nil, "")
	narrowed := buildReadingCoachPrompt("测试文章", blocks, readingOutline{Parts: parts},
		tasks, nil, nil, student, nil, "")

	sys := len([]rune(buildReadingCoachSystem(readingLangOf(article), "")))
	f, n := len([]rune(full)), len([]rune(narrowed))
	fmt.Printf("\n  article            %6d runes (9 paragraphs)\n", len([]rune(article)))
	fmt.Printf("  system             %6d runes\n", sys)
	fmt.Printf("  whole article sent %6d user runes -> prompt %d\n", f, sys+f)
	fmt.Printf("  this part only     %6d user runes -> prompt %d\n", n, sys+n)
	fmt.Printf("  user cut %.0f%%   whole prompt cut %.0f%%\n",
		100*float64(f-n)/float64(f), 100*float64(f-n)/float64(sys+f))
	if n >= f {
		t.Fatalf("disclosure did not shrink the prompt: %d -> %d", f, n)
	}
}
