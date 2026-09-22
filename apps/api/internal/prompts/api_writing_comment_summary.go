package prompts

// Purpose: api/writing_comment_summary.go 的固定提示词与条件指令。
// Consumer: internal/api/writing_comment_summary.go; builders choose conditions and render context.
// Comments are maintenance metadata and are never sent to a model.

// WritingSummaryAbsenceNudge retains the production text of writingSummaryAbsenceNudge.
// writingSummaryAbsenceNudge 是重试那一轮加进去的纠正话。
//
// 照着 writingGuideBracketNudge 的做法：把犯的那一处指出来，而不是把整条规矩
// 再念一遍 —— 念规矩它上一轮已经读过了。
const WritingSummaryAbsenceNudge = `刚才那份 summary 里说了她「缺」什么（出现了「%s」）。

summary 概述文章已有的内容和表达效果。需要补充或修改的内容放在 points 中，
每条附上原文引句和具体修改建议。

重新输出一次完整的 JSON，只改 summary 这一个字段，points 原样保留。`

// WritingNoPointNudge retains the production text of writingNoPointNudge.
// writingNoPointNudge —— 说了这篇有问题，却一条都没指出来的那一轮。
//
// 照着 writingSummaryAbsenceNudge 的做法：指出犯的是哪一处，不把整条规矩再念一遍。
// 🚨 这段话原来写的是「points 却是空的」—— 而 points **往往不是空的**：
// 里面常常有一条 good。模型照着这句话回头看自己那一份，发现 points 有东西，
// 于是认为这条提醒不适用，原样又回一遍。
//
// **提醒里说的那件事必须是真发生的那件事**，否则它只是一句模型对不上号的话。
// 现在说的是「没有一条 issue」，并且把 issue 活下来要满足的条件列清楚 ——
// 实测里被丢掉的那些，十有八九是漏了其中一条。
const WritingNoPointNudge = `刚才那一份里，verdict 不是 pass，但 points 里**没有一条 kind 是 issue**
（有 good 也不算——夸奖不是她能照着改的东西）。

你说了这篇还有要改的地方，却没有给出一条她能动手的意见。她看到的会是一句
挂在最上面、点不动也追不到原文的话。

重新输出一次完整的 JSON：
- 真的有要改的地方 → 至少给一条 kind 为 issue 的，并且四样都要齐，缺一样这条就会被丢掉：
  · quote：**单独写在 quote 这个字段里**，逐字照抄她原文里的一句（写在 text 里不算）；
  · symptom：只能用给定清单里的 id；
  · text：说清楚是什么问题；
  · action：她现在就能做的那一个动作。
- 其实没有 → 把 verdict 改成 pass，summary 概述文章已有的内容和表达效果。`

// WritingCommentUnparseableNudge retains the production text of writingCommentUnparseableNudge.
// writingCommentUnparseableNudge —— 上一份读不出来的时候，重问那一轮说的话。
//
// 照这个文件一贯的做法：指出犯的是哪一处，不把整条规矩再念一遍
// （念规矩它上一轮已经读过了）。
const WritingCommentUnparseableNudge = `刚才那一份我读不出来 —— 它不是一个能解析的 JSON 对象。

请**只输出那一个 JSON 对象**：不要围栏、不要在前面或后面加解释、
不要先写一份再写第二份。字符串里的引号要转义，数组和对象的括号要配对。

内容按上面的要求重新给一次。`
