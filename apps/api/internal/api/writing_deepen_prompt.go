package api

// Prompt assembly for writing_deepen.go. Pure: no database or model calls.
// Static teaching text lives in internal/prompts; context selection is separate
// where a feature has a dedicated *_context.go file.

import (
	"mindimprint/api/internal/prompts"

	"strings"

	"mindimprint/api/internal/store/sqlc"
	"mindimprint/api/internal/vocab"
)

// deepenSystem — the same four-part 怎么说话 doctrine every other 印记 voice
// in this room speaks (writingGuideTeachingRules, copied verbatim rather than
// re-derived so an engineer reading these files out of order never finds two
// versions of how 印记 talks), plus the Socratic charter specific to this
// sub-agent: it asks, it never writes her sentence, and its examples come
// from the borrowed material in the vocab library rather than her own topic.
const deepenSystem = prompts.DeepenSystem

// buildDeepenBrief assembles the WHOLE context this sub-agent gets, and
// nothing else: the paper's title and language, the whole outline map (so it
// can see where this block sits relative to every other block), this block's
// role/heading/text and what she has already drafted in it, and the guide
// questions already shown to her (so it does not re-ask them). It does NOT
// take the planning transcript (atom_message, block_id IS NULL) as a
// parameter — that omission from the signature IS the guarantee, not just an
// unused argument that could be added later. TestBuildDeepenBrief pins it.
func buildDeepenBrief(wr sqlc.Writing, outline []sqlc.WritingOutline, block sqlc.WritingOutline, snippetText string, guideQuestions []string, studentText, piece string) string {
	var b strings.Builder
	b.WriteString(writingTopicLine(wr, "题目："))
	b.WriteString(writingLangLine(wr))
	b.WriteString(writingLengthLine(wr, "目标篇幅"))

	// 🚨 整篇上下文（writing_piece_context.go）取代了这里原来手写的那一段。
	// 原来那段只列每一块的**标题**，不带她在那一块写下的字 —— 于是这个
	// 子 agent 和「AI审阅这一段」犯的是同一个错：它不知道后面的段里
	// 已经有那件具体的事了（同事 2026-09-20 的意见 6 和 9）。
	//
	// piece 为空（调用方读不到片段）就退回只列标题 —— 少一份上下文可以，
	// 整篇的结构**不能**没有：没有它，这个子 agent 连自己在第几段都不知道。
	if strings.TrimSpace(piece) != "" {
		b.WriteString(piece)
	} else {
		b.WriteString("\n【整篇的结构】\n")
		for _, s := range outline {
			indent := strings.Repeat("  ", int(s.Depth))
			role := writingKindLabel(writingKindOf(s), s.Source)
			if role == "" {
				role = "（未命名的块）"
			}
			line := indent + "- " + role
			if t := strings.TrimSpace(s.Text); t != "" {
				line += "：" + t
			} else {
				line += "：（还没写）"
			}
			if s.ID == block.ID {
				line += "   ← **她现在停在这一块**"
			}
			b.WriteString(line + "\n")
		}
	}

	b.WriteString("\n【她现在停住的这一块】\n")
	b.WriteString("这一块的作用：" + writingKindLabel(writingKindOf(block), block.Source) + "\n")
	if t := strings.TrimSpace(block.Text); t != "" {
		b.WriteString("她给这一块定的要点：" + t + "\n")
	} else {
		b.WriteString("她还没给这一块定要点。\n")
	}
	if t := strings.TrimSpace(snippetText); t != "" {
		b.WriteString("她已经写下的段落内容：\n" + t + "\n")
	} else {
		b.WriteString("这一段还是空的。\n")
	}

	// 她请我们替她搜索或替她写 → 这一轮先说明再往下走。一次性。
	// 见 writing_refusal.go（同事 2026-09-20 的意见 8）。
	if writingAsksUsToDoIt(studentText) {
		b.WriteString(writingRefusalBlock)
	}

	if len(guideQuestions) > 0 {
		b.WriteString("\n【已经给她看过的引导问题——不要在这个对话里重复问这些】\n")
		for _, q := range guideQuestions {
			b.WriteString("- " + q + "\n")
		}
	}

	// Position AND language (vocab.For): this sub-agent is the one that actually
	// shows examples, so a wrong-language entry here would be read out loud.
	b.WriteString("\n【可用的方法】（举例子只能用这里的，别自己编，例子讲的是别的题目，不是她的）\n")
	for _, m := range vocab.For(writingKindAppliesTo(writingKindOf(block)), wr.Lang, writingGenreOf(wr, outline)) {
		b.WriteString("- id=" + m.ID + " · " + m.Label() + "：" + m.Definition + "\n")
	}
	return b.String()
}
