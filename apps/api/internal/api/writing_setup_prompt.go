package api

// Prompt assembly for writing_setup.go. Pure: no database or model calls.
// Static teaching text lives in internal/prompts; context selection is separate
// where a feature has a dedicated *_context.go file.

import (
	"strings"

	"mindimprint/api/internal/prompts"
	"mindimprint/api/internal/store/sqlc"
)

// writingOpeningSystem is the coach's opening line. It is the single most
// load-bearing prompt in the room: it sets whether the student feels
// accompanied or abandoned in her first three seconds.
//
// 铁律① is stated as a hard prohibition rather than left implicit, because
// "help me start this essay" is precisely the moment a model is most tempted
// to hand over a thesis. 铁律③ (one question at a time) is stated as a count,
// because "be concise" is not something a model reliably converts into "ask
// exactly one thing".
const writingOpeningSystem = prompts.WritingOpeningSystem

// writingOpeningSystemFor is writingOpeningSystem for this writing: unchanged
// for a writing she opened herself, with the two sentences above swapped for
// an assigned one.
func writingOpeningSystemFor(wr sqlc.Writing) string {
	if wr.Origin == "brought" {
		return writingBroughtOpeningSystem
	}
	if !writingIsAssigned(wr) {
		return writingOpeningSystem
	}
	return strings.NewReplacer(
		openingTopicOwn, openingTopicAssigned,
		openingRestateOwn, openingRestateAssigned,
	).Replace(writingOpeningSystem)
}

// writingBroughtOpeningSystem is the opening for a piece she wrote elsewhere
// and brought in for feedback.
//
// 🚨 2026-09-18 写作入口走查：带进来的一篇，印记的第一句是规划开场——
// 「你最想让读者最后相信的一件事是什么？」。她的文章已经写完、就摆在左边，
// 这句话等于没看见它。所以带进来的那一篇有自己的开场：先看见这篇，
// 再告诉她这一页怎么用。
const writingBroughtOpeningSystem = prompts.WritingBroughtOpeningSystem

// buildBroughtOpeningPrompt is the opening's input for a brought piece: the
// title, the language line, and her text.
func buildBroughtOpeningPrompt(wr sqlc.Writing, body string) string {
	var b strings.Builder
	b.WriteString("题目：" + wr.Title + "\n")
	b.WriteString(writingLangLine(wr))
	if wr.TargetWords != nil {
		b.WriteString(writingLengthLine(wr, "她定的目标篇幅"))
	}
	b.WriteString("\n【她带来的全文】\n")
	body = strings.TrimSpace(body)
	if body == "" {
		b.WriteString("（正文是空的。）\n")
	} else {
		b.WriteString(cutRunes(body, broughtOpeningDraftRunes) + "\n")
	}
	return b.String()
}

// buildWritingOpeningPrompt assembles what the coach sees: her title, the
// settings she just chose, and everything she has said. AI turns are excluded
// — on the opening path there are none by construction (the handler refuses
// to run once one exists), and including the role would only invite the model
// to continue a conversation rather than start one.
func buildWritingOpeningPrompt(wr sqlc.Writing, msgs []sqlc.AtomMessage) string {
	var b strings.Builder
	b.WriteString(writingTopicLine(wr, "题目/想法："))
	b.WriteString(writingLangLine(wr))
	if wr.TargetWords != nil {
		b.WriteString(writingLengthLine(wr, "她定的目标篇幅"))
	} else {
		b.WriteString("她还没定篇幅（这完全没问题，别追问）。\n")
	}
	b.WriteString("\n【她自己说过的话】\n")
	any := false
	for _, m := range msgs {
		if m.Role != "student" {
			continue
		}
		if s := strings.TrimSpace(m.Content); s != "" {
			b.WriteString("- " + s + "\n")
			any = true
		}
	}
	if !any {
		b.WriteString("（她还没说什么，只有上面那个题目。）\n")
	}
	return b.String()
}
