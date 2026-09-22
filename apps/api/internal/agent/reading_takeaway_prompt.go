package agent

// Prompt assembly for reading_takeaway.go. Pure: no database or model calls.
// Static teaching text lives in internal/prompts; context selection is separate
// where a feature has a dedicated *_context.go file.

import (
	"mindimprint/api/internal/prompts"
)

const readingTakeawaySystem = prompts.ReadingTakeawaySystem
