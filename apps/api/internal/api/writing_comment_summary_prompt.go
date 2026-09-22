package api

// Prompt assembly for writing_comment_summary.go. Pure: no database or model calls.
// Static teaching text lives in internal/prompts; context selection is separate
// where a feature has a dedicated *_context.go file.

import (
	"mindimprint/api/internal/prompts"
)

// writingSummaryAbsenceNudge 是重试那一轮加进去的纠正话。
//
// 照着 writingGuideBracketNudge 的做法：把犯的那一处指出来，而不是把整条规矩
// 再念一遍 —— 念规矩它上一轮已经读过了。
const writingSummaryAbsenceNudge = prompts.WritingSummaryAbsenceNudge

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
const writingNoPointNudge = prompts.WritingNoPointNudge

// writingCommentUnparseableNudge —— 上一份读不出来的时候，重问那一轮说的话。
//
// 照这个文件一贯的做法：指出犯的是哪一处，不把整条规矩再念一遍
// （念规矩它上一轮已经读过了）。
const writingCommentUnparseableNudge = prompts.WritingCommentUnparseableNudge
