package api

import (
	"mindimprint/api/internal/promptassembly"
	"mindimprint/api/internal/store/sqlc"
	"strings"
)

// readingCoachContext contains selected facts and explicit state signals.
// Selection never writes teaching instructions, queries storage, or calls an LLM.
// Source slices are read-only for the duration of this synchronous assembly.
type readingCoachContext struct {
	Title             string
	Blocks            []Block
	Outline           readingOutline
	Tasks             []sqlc.ReadingTask
	Picks             []readingPick
	StudentText       string
	LensDone          *readingLensDone
	OpenLens          string
	History           []sqlc.AtomMessage
	Scope             map[string]bool
	Current           *sqlc.ReadingTask
	OpenCard          *coachCard
	DroppedCardReason string
	StepStuck         bool
	Article           []readingContextBlock
	Selections        []promptassembly.Selection
}

type readingContextBlock struct {
	Tag, Text string
	Omitted   string // empty, scope, or budget; never interpreted as absent source text
}

func selectReadingCoachContext(title string, blocks []Block, outline readingOutline, tasks []sqlc.ReadingTask, msgs []sqlc.AtomMessage, picks []readingPick, studentText string, lensDone *readingLensDone, openLens string) readingCoachContext {
	tail := msgs
	if len(tail) > readingCoachTurnsWindow {
		tail = tail[len(tail)-readingCoachTurnsWindow:]
	}
	c := readingCoachContext{
		Title: title, Blocks: blocks, Outline: outline, Tasks: tasks, Picks: picks,
		StudentText: studentText, LensDone: lensDone, OpenLens: openLens,
		History: tail, Current: currentReadingTask(tasks), OpenCard: lastOpenCard(tail),
		DroppedCardReason: lastDroppedCard(tail), StepStuck: coachStepStuck(tasks, tail),
		Scope: readingDisclosureScope(blocks, outline, tasks, picks, msgs),
	}
	used, included, scopedOut, budgetOut := 0, 0, 0, 0
	for i, blk := range blocks {
		text := strings.TrimSpace(blk.Text)
		if text == "" {
			continue
		}
		tag := readingBlockTag(i, blk.ID)
		if label := loadLabels[outline.Load[blk.ID]]; label != "" {
			tag += "·" + label
		}
		part := readingContextBlock{Tag: tag}
		switch {
		case c.Scope != nil && !c.Scope[blk.ID]:
			part.Omitted = "scope"
			scopedOut++
		case used+len([]rune(text)) > readingPlanArticleRuneBudget:
			part.Omitted = "budget"
			budgetOut++
		default:
			part.Text = text
			used += len([]rune(text))
			included++
		}
		c.Article = append(c.Article, part)
	}
	c.Selections = []promptassembly.Selection{
		{ID: "history", Total: len(msgs), Included: len(tail), Unit: "messages", Reason: "last 14 messages, before role/empty filtering"},
		{ID: "article", Total: len(c.Article), Included: included, Unit: "nonempty paragraphs", Reason: "current-step scope followed by existing rune budget"},
		{ID: "article-scope", Total: len(c.Article), Included: len(c.Article) - scopedOut, Unit: "nonempty paragraphs", Reason: "readingDisclosureScope; excluded paragraphs retain a marker"},
		{ID: "article-budget", Total: len(c.Article) - scopedOut, Included: len(c.Article) - scopedOut - budgetOut, Unit: "nonempty paragraphs", Reason: "whole-paragraph budget; excluded paragraphs retain a marker"},
	}
	return c
}
