package api

// Prompt assembly for writing_plan_lang.go. Pure: no database or model calls.
// Static teaching text lives in internal/prompts; context selection is separate
// where a feature has a dedicated *_context.go file.

import (
	"mindimprint/api/internal/prompts"
)

const writingPlanMaterialZH = prompts.WritingPlanMaterialZH

// 英文议论文那一份。它不排「个人经历最弱」那个次序 —— 英文写作课要的是
// reason 底下有具体的 example，而 example 是她自己的经历还是读来的材料，
// 不影响得分；真正会被扣分的是**摆完材料没有 commentary**。
const writingPlanMaterialEN = prompts.WritingPlanMaterialEN

const writingPlanSkeletonZH = prompts.WritingPlanSkeletonZH

// 英文议论文那一份骨架表。名字用英文 —— 那几个词是她要学会的东西，
// 而且她的英文老师就是这么叫它们的。
const writingPlanSkeletonEN = prompts.WritingPlanSkeletonEN

const writingPlanEnglishArgumentKinds = prompts.WritingPlanEnglishArgumentKinds

const writingPlanEnglishNarrativeKinds = prompts.WritingPlanEnglishNarrativeKinds
