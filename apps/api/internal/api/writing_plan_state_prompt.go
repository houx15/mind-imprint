package api

// Prompt assembly for writing_plan_state.go. Pure: no database or model calls.
// Static teaching text lives in internal/prompts; context selection is separate
// where a feature has a dedicated *_context.go file.

import (
	"mindimprint/api/internal/prompts"
)

// writingPlanStalledBlock 是她连着两轮等于没答时，加进 prompt 的那一段。
//
// 🚨 这一段**只在真的停滞时出现**，不做常驻。2026-09-05 的教训：
// 一轮只能做一件事，而把这类提示做成常驻，模型会一直去处理那条提示、
// 把该做的事挤掉（那次是六轮里一直在补一张卡，她的主页三处一直是空的）。
const writingPlanStalledBlock = prompts.WritingPlanStalledBlock
