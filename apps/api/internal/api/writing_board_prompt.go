package api

// Prompt assembly for writing_board.go. Pure: no database or model calls.
// Static teaching text lives in internal/prompts; context selection is separate
// where a feature has a dedicated *_context.go file.

import (
	"mindimprint/api/internal/prompts"
)

// writingRoleBoardNote 是标注板摆完的那一轮，加进 projection 的那段话。
//
// 三件事，顺序是有讲究的：先接住她摆的，再说那个缺口，最后给下一步。
// 「先接住」来自 master-writing 的四步反馈（先说哪个动作已经用对）；
// 「一次只说一件」来自那条贯穿整个房间的优先级。
const writingRoleBoardNote = prompts.WritingRoleBoardNote
