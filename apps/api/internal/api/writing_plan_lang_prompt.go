package api

// Writing prompt assembly. These functions render selected context without database or model calls.
// Preserve context selection and the order of stable and changing prompt sections.

import (
	"mindimprint/api/internal/prompts"
)

const writingPlanMaterialZH = prompts.WritingPlanMaterialZH
const writingPlanMaterialEN = prompts.WritingPlanMaterialEN

const writingPlanSkeletonZH = prompts.WritingPlanSkeletonZH
const writingPlanSkeletonEN = prompts.WritingPlanSkeletonEN

const writingPlanEnglishArgumentKinds = prompts.WritingPlanEnglishArgumentKinds

const writingPlanEnglishNarrativeKinds = prompts.WritingPlanEnglishNarrativeKinds
