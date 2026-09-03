package api

// writing_lang.go — the two prompt lines every writing-room model call owes
// the student: how long she said the piece should be, and what language it is
// in.
//
// ## Why this file exists (2026-09-03, from a colleague trial)
//
// A student set up an ENGLISH piece with a target of 500 — 500 **words**,
// which is what the setup dialog offered her ("words" when lang is en, "字"
// when it is zh, `WritingSetupModal.tsx`). Every prompt builder then wrote
// that number into the prompt as:
//
//	目标篇幅：约 500 字
//
// 500 Chinese characters is a genuinely tiny piece — about two paragraphs —
// so 印记 reasoned correctly from a false premise and told her:
//
//	> 500字很短，两个都展开容易平——这就是「并排说几条」的风险。更想让哪一个
//	> 瞬间当主角？另一个可以一笔带过或不写。
//
// The product owner's reaction was 「离谱，500词的作文不要两个例子？？？」 —
// and she is right: 500 words is a normal essay that wants two developed
// examples. The advice was not a prompt-tuning problem or a model failure.
// **The unit was wrong.** `writing.target_words` is a count of whatever the
// student was shown, and the only thing that says which that is is
// `writing.lang`.
//
// So the unit is derived, in ONE place, from the same field the dialog
// branched on. Any writing-room prompt that mentions length goes through
// `writingLengthLine`; a hand-written 「约 N 字」 anywhere in this package is
// a bug waiting for the next English piece.
//
// ## The language line is a HARD rule, not a fact
//
// The second half of the same trial: an English piece grew a **Chinese**
// mind map. `wr.Lang` was reaching only 3 of the 9 writing prompt builders,
// and even where it arrived it was stated as a bare fact (「写作语言：en」)
// with nothing saying what follows from it. A model handed a Chinese system
// prompt and the neutral observation "the language is en" keeps answering in
// Chinese — including in the fields that BECOME her essay.
//
// `writingLangLine` therefore separates the two audiences a writing-room
// reply has, because they do not get the same language:
//
//   - **What 印记 says to her** — coaching, questions, explanations. Stays in
//     Chinese. She is a Chinese-speaking student learning to write in
//     English; explaining 让步 to her in English would be teaching two things
//     at once. (An `en` piece does not mean an English-speaking student.)
//   - **Anything that ends up IN the piece** — outline/mind-map node labels,
//     candidate titles, quoted phrasings, snippet scaffolding. Must be in the
//     piece's own language, because those words are the essay's own words.
//
// That distinction is why this is not simply "reply in English": doing that
// would have fixed the mind map and broken the teaching.
//
// 🚨 These two helpers return prompt text, and prompt text is only as good as
// what comes back. Per [[prompt-output-must-be-verifiable-2026-09-03]] a
// stub test proves only that the parser understands JSON we wrote ourselves —
// so `writingLengthLine`'s pure unit choice is pinned here in
// `writing_lang_internal_test.go`, and the language rule's real effect is
// checked against a live model before shipping.

import (
	"strconv"
	"strings"

	"mindimprint/api/internal/store/sqlc"
)

// langEnglish is the one non-Chinese value `writing.lang` takes today. The
// setup handler rejects anything that is not "zh" or "en"
// (writing_setup.go), so a switch on this constant is exhaustive; it is
// named rather than inlined so a third language grows one obvious call site
// per helper instead of a scatter of `== "en"`.
const langEnglish = "en"

// lengthUnit is what `writing.target_words` COUNTS, in the student's own
// terms — the exact word the setup dialog put next to the number she typed.
//
// The whole point of this function is that the number is meaningless without
// it: 500 of one is a short paragraph, 500 of the other is a full essay.
func lengthUnit(lang string) string {
	if lang == langEnglish {
		return "词"
	}
	return "字"
}

// writingLengthLine is the one line every writing-room prompt uses to state
// how long the piece is meant to be, or "" when she never said.
//
// `label` lets a call site keep the wording it already had (「目标篇幅」 vs
// 「她定的目标篇幅」 vs 「目标字数」) without any of them owning the unit.
//
// The parenthetical is not decoration. A model reading 「约 500 词」 inside an
// otherwise Chinese prompt can still read 词 as 字 by habit; naming the unit
// twice, once in the piece's own metric, is what makes the premise
// un-mistakable — and it is exactly the premise that was wrong when 印记 told
// a 500-word essay it had room for only one example.
func writingLengthLine(wr sqlc.Writing, label string) string {
	if wr.TargetWords == nil {
		return ""
	}
	n := int(*wr.TargetWords)
	unit := lengthUnit(wr.Lang)
	var b strings.Builder
	b.WriteString(label)
	b.WriteString("：约 ")
	b.WriteString(strconv.Itoa(n))
	b.WriteString(" ")
	b.WriteString(unit)
	if wr.Lang == langEnglish {
		// Said twice on purpose — see above. 「约 500 词（English words，不是
		// 汉字）」 leaves no reading in which this is a two-paragraph piece.
		b.WriteString("（English words，不是汉字；500 词是一篇完整的文章，不是一小段）")
	}
	b.WriteString("\n")
	return b.String()
}

// writingLangLine states the piece's language AND what follows from it — the
// split described in this file's comment: 印记 keeps coaching in Chinese, but
// every string that becomes part of her piece is in the piece's language.
//
// Emitted by every writing-room prompt builder, including the ones with no
// visible language-dependent field: 「写作语言」 alone was already reaching
// three of them and an English piece still grew a Chinese mind map, because
// a fact with no rule attached changes no output.
func writingLangLine(wr sqlc.Writing) string {
	if wr.Lang == langEnglish {
		return "写作语言：英文。\n" +
			"🚨 语言规则（两个不同的东西，不要混）：\n" +
			"- 你**跟她说话**——提问、讲方法、解释为什么——仍然用中文。她是中文母语的学生，正在学用英文写作；用英文给她讲「让步」等于一次教两件事。\n" +
			"- 但**任何会进入这篇文章的字**必须是英文：提纲/思维导图的节点、候选标题、你替她拟的措辞、示例句式。那些字就是她文章自己的字。\n"
	}
	return "写作语言：中文。跟她说话和文章内容都用中文。\n"
}
