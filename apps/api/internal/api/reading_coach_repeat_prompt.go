package api

// Prompt assembly for reading_coach_repeat.go. Pure: no database or model calls.
// Static teaching text lives in internal/prompts; context selection is separate
// where a feature has a dedicated *_context.go file.

import (
	"mindimprint/api/internal/prompts"
)

// coachStuckNudge 是卡住时加进 prompt 的那一节。
//
// 它说的是**做什么**，不是「你错了」：给三条具体的出路。只说「别重复」的话，
// 模型会把这一节也当成一条要遵守的禁令，然后继续卡在原地 —— 换一个说法接着问。
const coachStuckNudge = prompts.CoachStuckNudge
