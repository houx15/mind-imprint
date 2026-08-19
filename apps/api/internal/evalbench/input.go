package evalbench

import (
	"mindimprint/api/internal/agent"
	"mindimprint/api/internal/evalreport"
)

// Input is Evalbench's frozen, database-free projection of an evaluation
// report request. It intentionally belongs to the offline tool rather than to
// the production evaluation path.
type Input struct {
	ReportID    string
	ProjectID   string
	StudentID   string
	StudentName string
	GeneratedAt string

	Basics    evalreport.Basics
	Events    []evalreport.EventEntry
	Materials []evalreport.MaterialEntry
	ToolUsage []evalreport.ToolUsageEntry

	Context    agent.ReportGenContext
	Candidates []evalreport.Candidate
}
