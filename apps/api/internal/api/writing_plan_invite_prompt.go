package api

// Writing prompt assembly. These functions render selected context without database or model calls.
// Preserve context selection and the order of stable and changing prompt sections.

import (
	"mindimprint/api/internal/prompts"
)

const writingPlanInviteNudge = prompts.WritingPlanInviteNudge
