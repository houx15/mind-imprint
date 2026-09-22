package api

// Prompt assembly for writing_guide.go. Pure: no database or model calls.
// Static teaching text lives in internal/prompts; context selection is separate
// where a feature has a dedicated *_context.go file.

import (
	"mindimprint/api/internal/prompts"

	"strings"

	"github.com/google/uuid"

	"mindimprint/api/internal/store/sqlc"
	"mindimprint/api/internal/vocab"
)

// Shared presentation rules for single-block and batch guidance. Method names
// must exist in the registry; per-field output contracts remain below.
const writingGuideTeachingRules = prompts.WritingGuideTeachingRules

// writingGuideQuestionRules is the content discipline for `questions`,
// shared by the single-block and batch prompts for the same
// never-drift-apart reason as writingGuideTeachingRules.
const writingGuideQuestionRules = prompts.WritingGuideQuestionRules

// writingGuideSystem — guides ONE block (POST /outline/{oid}/guide, the
// single-block regenerate).
const writingGuideSystem = prompts.WritingGuideSystem

// writingGuideBatchSystem — guides EVERY block in the outline in ONE call
// (POST /writings/{id}/guide). This is the point of Task 4: batching the
// whole skeleton costs about what one 卡住了？ click cost, so guidance is
// already there the moment she opens 段落 instead of waiting for her to find
// a button.
const writingGuideBatchSystem = prompts.WritingGuideBatchSystem

// buildWritingGuidePrompt assembles what the model sees for ONE block: which
// block it is (role + her own heading text), the skeleton it sits in, what she
// has already drafted there, her material, and the methods usable at this
// position.
func buildWritingGuidePrompt(wr sqlc.Writing, block sqlc.WritingOutline, siblings []sqlc.WritingOutline, existing string, msgs []sqlc.AtomMessage, piece string) string {
	var b strings.Builder
	b.WriteString(writingTopicLine(wr, "题目/想法："))
	b.WriteString(writingLangLine(wr))
	b.WriteString(writingLengthLine(wr, "目标篇幅"))

	// There is no template name to report any more: the structure is not
	// chosen from a library, it is grown out of her own planning conversation
	// (writing_plan.go). The sibling blocks below already carry everything
	// this prompt needs to know about the shape she ended up with — and they
	// carry it in HER words rather than as a category name.

	// The sibling blocks matter: a question for 「你的回应」 is only good if it
	// knows what she put in 「反方最强的说法」. Without them the model asks the
	// same generic question in every block.
	// 🚨 整篇上下文（writing_piece_context.go）取代了这里原来手写的那一段。
	// 原来那段只列每一块的**要点**（她定的标题），不带她在那一块**写下的字**。
	// 同事 2026-09-20 的意见 9：要知道别的段写了什么，才判得出这一段缺什么。
	// piece 为空（比如批量那一路）就退回只列要点。
	if strings.TrimSpace(piece) != "" {
		b.WriteString(piece)
	} else {
		b.WriteString("\n【整篇的结构，以及每一块她自己写下的要点】\n")
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
	if e := strings.TrimSpace(existing); e != "" {
		b.WriteString("她已经写下的段落内容：\n" + e + "\n")
	} else {
		b.WriteString("这一段还是空的。\n")
	}

	b.WriteString("\n【她在对话里说过的话】\n")
	said := recentStudentSaid(msgs)
	for _, s := range said {
		b.WriteString("- " + s + "\n")
	}
	if len(said) == 0 {
		b.WriteString("（她还没在对话里说过什么。）\n")
	}

	// Filtered by position AND by the piece's language — see vocab.For: an
	// English frame offered inside a Chinese essay is a bug, not a rough edge.
	b.WriteString("\n【可用的方法】（只能用这里的 id，不要自己编）\n")
	for _, m := range vocab.For(writingKindAppliesTo(writingKindOf(block)), wr.Lang, writingGenreOf(wr, siblings)) {
		b.WriteString("- id=" + m.ID + " · " + m.Label() + "：" + m.Definition + "\n")
	}
	// See writingMethodFamiliesLine: an English piece can now be helped with
	// vocabulary / sentence craft / narrative, not only with argument frames.
	b.WriteString(writingMethodFamiliesLine(wr))
	return b.String()
}

// buildWritingGuideBatchPrompt assembles what the model sees for the WHOLE
// outline at once: every block (id, role, her heading text, what she has
// already drafted there, if anything), her material, and the method library
// for THIS PIECE'S LANGUAGE annotated with where each one applies (same "list
// every position, annotate applies_to" shape buildWritingPlanPrompt already
// uses) — a single block's narrower vocab.For(...) position filter does not fit
// here because one call has to serve blocks at every position at once. The
// language filter (vocab.ForLang) still applies: it is not a position.
func buildWritingGuideBatchPrompt(wr sqlc.Writing, blocks []sqlc.WritingOutline, textByBlock map[uuid.UUID]string, msgs []sqlc.AtomMessage) string {
	var b strings.Builder
	b.WriteString(writingTopicLine(wr, "题目/想法："))
	b.WriteString(writingLangLine(wr))
	b.WriteString(writingLengthLine(wr, "目标篇幅"))

	b.WriteString("\n【整篇的结构，以及每一块的 id、她自己写下的要点、和已经写的段落】\n")
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
		// An already-guided block stays IN the list — the model needs the whole
		// shape of the piece to guide the rest well — but is marked so it does
		// not get re-guided. The handler drops any guide it returns for one of
		// these anyway (knownIDs); this line is what keeps it from wasting the
		// tokens in the first place.
		if _, has := storedWritingGuide(s); has {
			b.WriteString("  这一块已经有引导了，不用再给。\n")
		}
	}

	b.WriteString("\n【她在对话里说过的话】\n")
	said := recentStudentSaid(msgs)
	for _, s := range said {
		b.WriteString("- " + s + "\n")
	}
	if len(said) == 0 {
		b.WriteString("（她还没在对话里说过什么。）\n")
	}

	b.WriteString("\n【可用的方法】（只能用这里的 id，不要自己编）\n")
	for _, m := range vocab.ForLang(wr.Lang, writingGenreOf(wr, blocks)) {
		b.WriteString("- id=" + m.ID + " · " + m.Label() + "（" + m.AppliesTo + "）：" + m.Definition + "\n")
	}
	b.WriteString(writingMethodFamiliesLine(wr))
	return b.String()
}

// writingGuideAnotherAngle 是重新生成那一轮加进 prompt 的一段。
//
// 🚨 她按「换一组问题」是因为上一组没问到点子上，不是因为她想看同一件事
// 再问一遍。不把上一组喂回去，模型有不小的概率原地换个说法重写一遍 ——
// 那正是「每一次刷新就会变成新的东西」里最让人白按一次的那种「新」。
func writingGuideAnotherAngle(prior *writingGuideDTO) string {
	if prior == nil || len(prior.Questions) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("\n【她已经读过这几个问题，没帮上忙】\n")
	for _, q := range prior.Questions {
		b.WriteString("- " + q + "\n")
	}
	b.WriteString("这一轮换一个**角度**看同一块：换一种材料、换一个读者会卡住的地方、" +
		"或者换一个方法。不要把上面那几个问题换个说法再问一遍。\n")
	return b.String()
}
