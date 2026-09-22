package api

// Prompt assembly for writing_refusal.go. Pure: no database or model calls.
// Static teaching text lives in internal/prompts; context selection is separate
// where a feature has a dedicated *_context.go file.

import (
	"mindimprint/api/internal/prompts"
)

// writingRefusalBlock 是命中那一轮加进 prompt 的一段。
//
// 🚨 **一次性，不做常驻。** 2026-09-05 的教训：这类提示做成常驻，模型会一直
// 去处理那条提示、把该做的事挤掉（那次是六轮里一直在补一张卡，她的主页三处
// 一直是空的）。所以调用点只在 `writingAsksUsToDoIt(studentText)` 为真的
// 那一轮把它写进去。
const writingRefusalBlock = prompts.WritingRefusalBlock

// writingRefusalNudge 是校验没过时，重试那一轮补上去的话。
//
// 只说犯的那一处 + 这一轮该做什么，不把整段规矩重念一遍 —— 它上一轮读过了。
const writingRefusalNudge = prompts.WritingRefusalNudge
