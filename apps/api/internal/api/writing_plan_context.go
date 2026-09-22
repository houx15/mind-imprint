package api

import (
	"mindimprint/api/internal/promptassembly"
	"mindimprint/api/internal/store/sqlc"
	"mindimprint/api/internal/vocab"
)

// writingPlanContext separates state/selection from authored instructions.
// Counts are advisory facts; this refactor does not change readiness or stages.
type writingPlanContext struct {
	Writing              sqlc.Writing
	Rows                 []sqlc.WritingOutline
	History              []sqlc.AtomMessage
	StudentText          string
	Shape                writingPlanShape
	Need                 writingPlanNeed
	Stalled, AskedToDoIt bool
	Methods              []vocab.Method
	Selection            promptassembly.Selection
}

func selectWritingPlanContext(wr sqlc.Writing, rows []sqlc.WritingOutline, msgs []sqlc.AtomMessage, studentText string) writingPlanContext {
	tail := msgs
	if len(tail) > writingPlanTurnsWindow {
		tail = tail[len(tail)-writingPlanTurnsWindow:]
	}
	return writingPlanContext{
		Writing: wr, Rows: rows, History: tail, StudentText: studentText,
		Shape: writingPlanShapeOf(rows), Need: writingPlanNeedOf(wr),
		Stalled: writingPlanStalled(msgs, studentText), AskedToDoIt: writingAsksUsToDoIt(studentText),
		Methods:   vocab.ForLang(wr.Lang, writingGenreOf(wr, rows)),
		Selection: promptassembly.Selection{ID: "history", Total: len(msgs), Included: len(tail), Unit: "messages", Reason: "last 16 messages, before role/empty filtering"},
	}
}
