package api

// lite_grading_run.go — one AI 批改 attempt pair: call the review model, parse,
// litegrade.Check, and retry once with the reasons. No database here; the
// worker in lite_grading_jobs.go owns the row.

import (
	"context"
	"strings"
	"time"

	"mindimprint/api/internal/gateway"
	"mindimprint/api/internal/liteassign"
	"mindimprint/api/internal/litegrade"
	"mindimprint/api/internal/store/sqlc"
)

const (
	liteGradingPurpose     = "teacher_grading"
	liteGradingCallTimeout = 150 * time.Second
	liteGradingAttempts    = 2
)

// liteGradingInput builds litegrade's input from the version being graded.
// The person check and the symptom table are the writing room's own.
func liteGradingInput(src sqlc.GetLiteGradingSourceRow, rubric liteassign.Rubric) litegrade.Input {
	prompt := ""
	if src.AssignedPrompt != nil {
		prompt = strings.TrimSpace(*src.AssignedPrompt)
	}
	target := 0
	if src.TargetWords != nil {
		target = int(*src.TargetWords)
	}
	return litegrade.Input{
		Lang: src.Lang, Title: src.Title, Body: src.Body, AssignedPrompt: prompt,
		TargetWords: target, VersionNumber: int(src.Number), Rubric: rubric,
		SymptomCatalog: writingSymptomCatalog(src.Lang),
		PersonJudging:  personDirectedVerdict,
	}
}

// gradeWithRetry makes at most two model calls. A reply that fails Check is
// sent back with the reasons (and the rejected reply, so the model can fix
// it); a failed call is retried with the same messages. record is called for
// every call, because a call costs money whatever its reply.
func gradeWithRetry(ctx context.Context, prov gateway.Provider, resolved gateway.Resolved, in litegrade.Input, record func(gateway.ChatUsage)) (litegrade.Content, []litegrade.Reason, int) {
	msgs := []gateway.ChatMessage{
		{Role: gateway.RoleSystem, Content: litegrade.SystemPrompt(in)},
		{Role: gateway.RoleUser, Content: litegrade.UserPrompt(in)},
	}
	var reasons []litegrade.Reason
	for attempt := 1; attempt <= liteGradingAttempts; attempt++ {
		callCtx, cancel := context.WithTimeout(ctx, liteGradingCallTimeout)
		res, err := gateway.Collect(callCtx, prov, resolved, gateway.ChatRequest{Messages: msgs})
		cancel()
		record(res.Usage)
		if err != nil {
			reasons = []litegrade.Reason{{Code: litegrade.ReasonModelCall, Detail: err.Error()}}
			continue
		}
		content, perr := litegrade.Parse(res.Text)
		if perr != nil {
			reasons = []litegrade.Reason{{Code: litegrade.ReasonUnparseable}}
		} else {
			content = litegrade.NormalizeAI(content, in.Rubric)
			if reasons = litegrade.Check(content, in); len(reasons) == 0 {
				return content, nil, attempt
			}
		}
		msgs = append(msgs,
			gateway.ChatMessage{Role: gateway.RoleAssistant, Content: res.Text},
			gateway.ChatMessage{Role: gateway.RoleUser, Content: litegrade.RetryNudge(reasons)},
		)
	}
	return litegrade.Content{}, reasons, liteGradingAttempts
}
