package api

// Prompt assembly for writing_plan_invite.go. Pure: no database or model calls.
// Static teaching text lives in internal/prompts; context selection is separate
// where a feature has a dedicated *_context.go file.

import (
	"mindimprint/api/internal/prompts"
)

// writingPlanInviteNudge 是重试那一轮补上去的话。
//
// 只说犯的那一处 + 这一轮该做什么，不把整段规矩重念一遍 —— 它上一轮读过了。
const writingPlanInviteNudge = prompts.WritingPlanInviteNudge
