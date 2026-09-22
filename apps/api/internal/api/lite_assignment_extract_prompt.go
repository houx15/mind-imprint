package api

// Prompt assembly for lite_assignment_extract.go. Pure: no database or model calls.
// Static teaching text lives in internal/prompts; context selection is separate
// where a feature has a dedicated *_context.go file.

import (
	"mindimprint/api/internal/prompts"
)

// assignmentExtractSystem asks for the three writing-assignment settings the
// form has. The prompt field keeps the teacher's own words: this call is a
// compose step over text she already wrote, not a rewrite.
const assignmentExtractSystem = prompts.AssignmentExtractSystem
