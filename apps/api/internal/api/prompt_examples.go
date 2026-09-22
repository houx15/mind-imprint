package api

// Synthetic offline examples, like benchcases.go. No handler calls this file;
// it never reads student data, credentials or a database. Prompt text is built
// by the same functions production uses, not copied into fixture templates.

import (
	"github.com/jackc/pgx/v5/pgtype"
	"mindimprint/api/internal/gateway"
	"mindimprint/api/internal/promptassembly"
	"mindimprint/api/internal/store/sqlc"
	"strings"
)

type PromptExample struct {
	ID      string              `json:"id"`
	Class   string              `json:"class"`
	Request gateway.ChatRequest `json:"request"`
	// Documents maps message index to its section/selection trace.
	Documents        map[int]promptassembly.Document    `json:"documents,omitempty"`
	ContextFragments map[string]promptassembly.Document `json:"contextFragments,omitempty"`
}

func PromptAssemblyExamples() []PromptExample {
	var out []PromptExample
	for _, c := range []struct {
		id, text string
		done     *readingLensDone
		open     string
	}{
		{id: "reading/concept", text: "装机容量和发电量有什么区别？"},
		{id: "reading/open-lens", text: "这个工具怎么用？", open: openLensLine(true, "信源评估")},
		{id: "reading/lens-completed", done: &readingLensDone{CardName: "信源评估", Quote: "新增装机容量增长了。", Finding: "这里说的是装机容量。"}},
	} {
		blocks := SplitBlocks("新增装机容量增长了。\n\n实际发电量还与设备运行时间有关。")
		tasks := []sqlc.ReadingTask{{Kind: "focus_block", Label: "比较两个指标", Status: "pending", BlockID: "b1"}}
		doc := renderReadingCoachPrompt(selectReadingCoachContext("能源指标", blocks, readingOutline{}, tasks, nil, nil, c.text, c.done, c.open))
		out = append(out, PromptExample{ID: c.id, Class: gateway.ClassDialogue, Request: gateway.ChatRequest{Messages: []gateway.ChatMessage{
			{Role: gateway.RoleSystem, Content: buildReadingCoachSystem("zh")}, {Role: gateway.RoleUser, Content: doc.Text},
		}}, Documents: map[int]promptassembly.Document{1: doc}})
	}
	for _, lang := range []string{"zh", "en"} {
		for _, genre := range []string{genreArgument, genreNarrative} {
			wr := sqlc.Writing{Lang: lang, Title: "一次图书馆里的经历"}
			doc := renderWritingPlanPrompt(selectWritingPlanContext(wr, nil, nil, "我想记录上周和同学一起找资料的经历。"))
			out = append(out, PromptExample{ID: "writing/plan/" + lang + "/" + genre, Class: gateway.ClassDialogue, Request: gateway.ChatRequest{MaxTokens: 4096, Messages: []gateway.ChatMessage{
				{Role: gateway.RoleSystem, Content: writingPlanSystemFor(genre, lang)}, {Role: gateway.RoleUser, Content: doc.Text},
			}}, Documents: map[int]promptassembly.Document{1: doc}})
		}
	}
	wr := sqlc.Writing{Lang: "zh", Title: "校园观察"}
	node := sqlc.WritingOutline{ID: fixtureTaskID(1), Kind: writingKindPoint, Depth: 1, Text: "筑巢位置的选择"}
	text := strings.Repeat("我观察到鸟把树枝衔进屋檐。", 40)
	snippets := []sqlc.WritingSnippet{{ID: fixtureTaskID(2), OutlineID: pgtype.UUID{Bytes: node.ID, Valid: true}, Text: text}}
	piece := renderWritingPieceContext(selectWritingPieceContext(wr, []sqlc.WritingOutline{node}, snippets, nil, &node))
	out = append(out, PromptExample{ID: "writing/feedback/long-context", Class: gateway.ClassReview, Request: gateway.ChatRequest{Messages: []gateway.ChatMessage{
		{Role: gateway.RoleSystem, Content: buildWritingCommentSystem("zh", writingBlockCommentMaxIssues, writingKindPoint, helpAsk, genreArgument)},
		{Role: gateway.RoleUser, Content: buildWritingCommentPrompt(wr, "筑巢位置的选择", text, piece.Text, genreArgument)},
	}}, ContextFragments: map[string]promptassembly.Document{"piece": piece}})
	return out
}
