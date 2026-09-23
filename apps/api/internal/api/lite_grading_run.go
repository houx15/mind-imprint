package api

// lite_grading_run.go — one AI 批改 attempt pair: call the review model, parse,
// litegrade.Check, and retry once with the reasons. No database here; the
// worker in lite_grading_jobs.go owns the row.

import (
	"context"
	"encoding/json"
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
		// 老师批改那一路不挑文体：她交上来的可能是任何一种，
		// 多给几条认得出的毛病不会伤到谁。
		SymptomCatalog: writingSymptomCatalog(src.Lang, genreNarrative),
		PersonJudging:  personDirectedVerdict,
		// 认 id 也认名字，一律答回 id —— 老师那一趟会把渲染给她看的名字
		// 原样送回来（见 resolveWritingSymptom / SanitizeProvenance）。
		SymptomLookup: func(idOrName string) (string, string, bool) {
			s, ok := resolveWritingSymptom(src.Lang, idOrName)
			return s.ID, s.Name, ok
		},
	}
}

// gradeOutcome is what gradeWithRetry reports. Reasons is empty on success.
// On failure LastReply and ParseErr describe the last attempt, for the
// server log: LastReply is empty when that attempt's call itself failed, and
// ParseErr is nil unless that attempt's reply could not be parsed.
type gradeOutcome struct {
	Content   litegrade.Content
	Reasons   []litegrade.Reason
	Attempts  int
	LastReply string
	ParseErr  error
	// Tried is every failed attempt's reasons, in order (a success adds
	// nothing). A grading that passes on its second try hides why the first
	// failed; the live tests print this.
	Tried [][]litegrade.Reason
}

// gradeWithRetry makes at most two model calls. A reply that fails Check is
// sent back with the reasons (and the rejected reply, so the model can fix
// it); a failed call is retried with the same messages. record is called for
// every call, because a call costs money whatever its reply.
func gradeWithRetry(ctx context.Context, prov gateway.Provider, resolved gateway.Resolved, in litegrade.Input, record func(gateway.ChatUsage)) gradeOutcome {
	msgs := []gateway.ChatMessage{
		{Role: gateway.RoleSystem, Content: litegrade.SystemPrompt(in)},
		{Role: gateway.RoleUser, Content: litegrade.UserPrompt(in)},
	}
	out := gradeOutcome{Attempts: liteGradingAttempts}
	for attempt := 1; attempt <= liteGradingAttempts; attempt++ {
		callCtx, cancel := context.WithTimeout(ctx, liteGradingCallTimeout)
		req := gateway.ChatRequest{Messages: msgs}
		// Use the existing wire-level object constraint where supported. The
		// rubric and quote checks below still validate the object's contents.
		if resolved.Kind == gateway.KindOpenAICompatible {
			req.ResponseFormat = gateway.ResponseFormatJSONObject
		}
		res, err := gateway.Collect(callCtx, prov, resolved, req)
		cancel()
		record(res.Usage)
		out.LastReply, out.ParseErr = res.Text, nil
		if err != nil {
			out.Reasons = []litegrade.Reason{{Code: litegrade.ReasonModelCall, Detail: err.Error()}}
			continue
		}
		content, perr := litegrade.Parse(res.Text)
		if perr != nil {
			out.ParseErr = perr
			out.Reasons = []litegrade.Reason{{Code: litegrade.ReasonUnparseable, Detail: perr.Error()}}
		} else {
			content = litegrade.NormalizeAI(content, in.Rubric)
			content = litegrade.SanitizeProvenance(content, in)
			if out.Reasons = litegrade.Check(content, in); len(out.Reasons) == 0 {
				return gradeOutcome{Content: content, Attempts: attempt, LastReply: res.Text, Tried: out.Tried}
			}
			// Last attempt, and the only problem left is a quotation in the
			// model's prose that is not her words: drop those quotation marks
			// rather than fail the whole grading (see UnwrapUnfoundQuotations).
			if attempt == liteGradingAttempts && litegrade.OnlyUnfoundQuotations(out.Reasons) {
				fixed := litegrade.UnwrapUnfoundQuotations(content, in)
				if len(litegrade.Check(fixed, in)) == 0 {
					return gradeOutcome{Content: fixed, Attempts: attempt, LastReply: res.Text, Tried: append(out.Tried, out.Reasons)}
				}
			}
		}
		out.Tried = append(out.Tried, out.Reasons)
		msgs = append(msgs,
			gateway.ChatMessage{Role: gateway.RoleAssistant, Content: res.Text},
			gateway.ChatMessage{Role: gateway.RoleUser, Content: litegrade.RetryNudge(out.Reasons)},
		)
	}
	return out
}

// gradingContentForView turns the STORED grading content into the shape a
// reader sees: each point's Symptom, stored as the closed table's id, is
// resolved to that table's teacher-facing name for this writing's language.
//
// 🚨 **The id is what is stored; the name is only ever rendered.** Storing
// the name instead cost us 对应毛病 on every teacher save — SanitizeProvenance
// ran a second time on the PATCH body, could not match the name it had
// written itself, and blanked the field (2026-09-23, proven by direct
// execution). It would also strand old gradings on a stale string the day a
// symptom is renamed, and break "how often does this symptom fire in this
// class?", which needs the join key.
//
// The round trip is safe in both directions: this hands the teacher a name,
// her client sends that name back, and resolveWritingSymptom recognises a
// name as well as an id and answers with the id.
//
// Anything it cannot make sense of is passed through untouched — an
// unreadable row must not 500 a list, the same posture toCommentDTO takes.
func gradingContentForView(content []byte, lang string) json.RawMessage {
	if len(content) == 0 {
		return json.RawMessage(content)
	}
	var c litegrade.Content
	if err := json.Unmarshal(content, &c); err != nil {
		return json.RawMessage(content)
	}
	changed := false
	for i, p := range c.Points {
		if strings.TrimSpace(p.Symptom) == "" {
			continue
		}
		s, ok := resolveWritingSymptom(lang, p.Symptom)
		if !ok || s.Name == p.Symptom {
			continue
		}
		c.Points[i].Symptom = s.Name
		changed = true
	}
	if !changed {
		return json.RawMessage(content)
	}
	out, err := json.Marshal(c)
	if err != nil {
		return json.RawMessage(content)
	}
	return out
}
