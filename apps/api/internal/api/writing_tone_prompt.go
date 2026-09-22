package api

// Prompt assembly for writing_tone.go. Pure: no database or model calls.
// Static teaching text lives in internal/prompts; context selection is separate
// where a feature has a dedicated *_context.go file.

import (
	"mindimprint/api/internal/prompts"
)

// writingHostileToneNudge 重问那一轮的话。
//
// 照这个文件一贯的做法：指出犯的是**哪一处**，并且给一个改写的样子 ——
// 只说「请友善一点」，模型会把话说软，而不是把话说成描述加下一步。
const writingHostileToneNudge = prompts.WritingHostileToneNudge
