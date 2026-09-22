package api

// Writing prompt assembly. These functions render selected context without database or model calls.
// Preserve context selection and the order of stable and changing prompt sections.

import (
	"strings"

	"github.com/google/uuid"

	"mindimprint/api/internal/prompts"
	"mindimprint/api/internal/store/sqlc"
	"mindimprint/api/internal/vocab"
)

const writingGuideTeachingRules = prompts.WritingGuideTeachingRules
const writingGuideQuestionRules = prompts.WritingGuideQuestionRules
const writingGuideSystem = prompts.WritingGuideSystem
const writingGuideBatchSystem = prompts.WritingGuideBatchSystem

func buildWritingGuidePrompt(wr sqlc.Writing, block sqlc.WritingOutline, siblings []sqlc.WritingOutline, existing string, msgs []sqlc.AtomMessage, piece string) string {
	var b strings.Builder
	b.WriteString(writingTopicLine(wr, "题目/想法："))
	b.WriteString(writingLangLine(wr))
	b.WriteString(writingLengthLine(wr, "目标篇幅"))
	if strings.TrimSpace(piece) != "" {
		b.WriteString(piece)
	} else {
		b.WriteString("\n【整篇的结构，以及每一块学生自己写下的要点】\n")
		for _, s := range siblings {
			name := writingKindLabel(writingKindOf(s), s.Source)
			line := "- " + name
			if name == "" {
				line = "- （未命名的块）"
			}
			if t := strings.TrimSpace(s.Text); t != "" {
				line += "：" + t
			} else {
				line += "：（还没写）"
			}
			if s.ID == block.ID {
				line += "   ← **学生现在停在这一块**"
			}
			b.WriteString(line + "\n")
		}
	}

	b.WriteString("\n【学生现在停住的这一块】\n")
	b.WriteString("这一块的作用：" + writingKindLabel(writingKindOf(block), block.Source) + "\n")
	if t := strings.TrimSpace(block.Text); t != "" {
		b.WriteString("学生给这一块定的要点：" + t + "\n")
	} else {
		b.WriteString("学生还没给这一块定要点。\n")
	}
	if e := strings.TrimSpace(existing); e != "" {
		b.WriteString("学生已经写下的段落内容：\n" + e + "\n")
	} else {
		b.WriteString("这一段还是空的。\n")
	}

	b.WriteString("\n【学生在对话里说过的话】\n")
	said := recentStudentSaid(msgs)
	for _, s := range said {
		b.WriteString("- " + s + "\n")
	}
	if len(said) == 0 {
		b.WriteString("（学生还没在对话里说过什么。）\n")
	}
	b.WriteString("\n【可用的方法】（id 须取自此表）\n")
	for _, m := range vocab.For(writingKindAppliesTo(writingKindOf(block)), wr.Lang, writingGenreOf(wr, siblings)) {
		b.WriteString("- id=" + m.ID + " · " + m.Label() + "：" + m.Definition + "\n")
	}
	b.WriteString(writingMethodFamiliesLine(wr))
	return b.String()
}
func buildWritingGuideBatchPrompt(wr sqlc.Writing, blocks []sqlc.WritingOutline, textByBlock map[uuid.UUID]string, msgs []sqlc.AtomMessage) string {
	var b strings.Builder
	b.WriteString(writingTopicLine(wr, "题目/想法："))
	b.WriteString(writingLangLine(wr))
	b.WriteString(writingLengthLine(wr, "目标篇幅"))

	b.WriteString("\n【整篇的结构，以及每一块的 id、学生自己写下的要点、和已经写的段落】\n")
	for _, s := range blocks {
		role := s.Role
		if role == "" {
			role = "（未命名的块）"
		}
		line := "- id=" + s.ID.String() + " · " + role
		if t := strings.TrimSpace(s.Text); t != "" {
			line += "：" + t
		} else {
			line += "：（还没写要点）"
		}
		b.WriteString(line + "\n")
		if e := strings.TrimSpace(textByBlock[s.ID]); e != "" {
			b.WriteString("  已经写的段落：" + e + "\n")
		} else {
			b.WriteString("  这一段还是空的。\n")
		}
		if _, has := storedWritingGuide(s); has {
			b.WriteString("  这一块已经有引导了，不用再给。\n")
		}
	}

	b.WriteString("\n【学生在对话里说过的话】\n")
	said := recentStudentSaid(msgs)
	for _, s := range said {
		b.WriteString("- " + s + "\n")
	}
	if len(said) == 0 {
		b.WriteString("（学生还没在对话里说过什么。）\n")
	}

	b.WriteString("\n【可用的方法】（id 须取自此表）\n")
	for _, m := range vocab.ForLang(wr.Lang, writingGenreOf(wr, blocks)) {
		b.WriteString("- id=" + m.ID + " · " + m.Label() + "（" + m.AppliesTo + "）：" + m.Definition + "\n")
	}
	b.WriteString(writingMethodFamiliesLine(wr))
	return b.String()
}
func writingGuideAnotherAngle(prior *writingGuideDTO) string {
	if prior == nil || len(prior.Questions) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("\n【此前已提供的问题，本次请求换一组】\n")
	for _, q := range prior.Questions {
		b.WriteString("- " + q + "\n")
	}
	b.WriteString("本轮针对同一段落选择不同的构思方向，例如另一份已有材料、另一处需要说明的关系、" +
		"或者换一个方法。不要把上面那几个问题换个说法再问一遍。\n")
	return b.String()
}
