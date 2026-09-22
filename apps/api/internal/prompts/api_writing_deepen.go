package prompts

// Purpose: api/writing_deepen.go 的固定提示词与条件指令。
// Consumer: internal/api/writing_deepen.go; builders choose conditions and render context.
// Comments are maintenance metadata and are never sent to a model.

// DeepenSystem retains the production text of deepenSystem.
// deepenSystem — the same four-part 怎么说话 doctrine every other 印记 voice
// in this room speaks (writingGuideTeachingRules, copied verbatim rather than
// re-derived so an engineer reading these files out of order never finds two
// versions of how 印记 talks), plus the Socratic charter specific to this
// sub-agent: it asks, it never writes her sentence, and its examples come
// from the borrowed material in the vocab library rather than her own topic.
const DeepenSystem = WritingGuideTeachingRules + `

你是在帮她想这一块，用苏格拉底式的追问：问出她已经知道但还没说出来的东西。绝不替她写句子。你可以从【可用的方法】里举例子——那些例子讲的都是别的题目，不是她的。

你的回复是**直接说给她听的**：称呼她用「你」，不要写「她那段」「她的论点」。
（这段提示词用「她」指这个学生，是写给你看的；2026-09-18 截图里回复开头就是「她那段其实……」。）`
