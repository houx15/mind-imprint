package api

import (
	"errors"
	"mindimprint/api/internal/claritytest"
	"mindimprint/api/internal/gateway"
	"mindimprint/api/internal/store/sqlc"
	"testing"
)

// Exercise the rewritten retry instruction, not only the initial planning prompt.
func TestClarityWritingInvite(t *testing.T) {
	wr := sqlc.Writing{Title: "学校图书馆是否应延长开放时间", Lang: "zh"}
	rows := []sqlc.WritingOutline{
		planRow("a", 0, "学校图书馆可以延长开放时间", writingKindThesis),
		planRow("b", 1, "晚自习后需要安静的学习场所", writingKindPoint),
		planRow("c", 2, "上周我和三位同学在嘈杂的走廊复习", writingKindEvidence),
		planRow("d", 1, "可以先试行以了解实际需求", writingKindPoint),
		planRow("e", 2, "校报记录的邻校试行活动，每晚约三十人到馆", writingKindReference),
	}
	req := gateway.ChatRequest{Messages: []gateway.ChatMessage{
		{Role: gateway.RoleSystem, Content: writingPlanSystemFor(genreArgument, wr.Lang)},
		{Role: gateway.RoleUser, Content: buildWritingPlanPrompt(wr, rows, nil, "这些就是我的计划，我准备开始写了。")},
		{Role: gateway.RoleAssistant, Content: `{"reply":"还有什么理由吗？","add":[],"ready":false}`},
		{Role: gateway.RoleUser, Content: writingPlanInviteNudge},
	}}
	claritytest.Run(t, gateway.ClassDialogue, req, func(raw string) error {
		out, ok := parseWritingPlanReply(raw)
		if !ok {
			return errors.New("invite parse failed")
		}
		if !out.Ready || writingPlanReplyAsks(out.Reply) {
			return errors.New("ready plan still asks a question")
		}
		if len(out.Add) > 0 {
			return errors.New("retry invented new plan content")
		}
		return checkClarityTeachingLanguage(out.Reply)
	})
}
