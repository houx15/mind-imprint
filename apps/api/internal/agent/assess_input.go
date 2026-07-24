package agent

import "fmt"

// EventDigest is one already-projected event line the handler feeds the digest.
// Order is the event's position in the append-only stream (unprompted-first is
// legible from the ordering).
type EventDigest struct {
	Type  string
	Order int
	Text  string
}

// CardUse is one tool-card invocation with its 自发/提示后 initiative signal
// (reused from the studio equipment projection) and the CT dimension it exercises.
type CardUse struct {
	CardID    string
	Dimension string
	Spont     string // 自发 | 提示后
}

// DispositionUse is one accept/rewrite/reject decision on a coach intervention —
// the feedback-comprehension signal.
type DispositionUse struct {
	Kind   string // accept | rewrite | reject
	Reason string
}

// Round is one student turn in order: the student's prompt plus the AI ask/act
// that framed it. Drives SOLO per-round judging and the 提示词透镜.
type Round struct {
	N             int
	StudentPrompt string
	AiContext     string
}

// AssessmentInput is the compact, temporal, model-ready digest of one project's
// process record. Pure data; built by BuildAssessmentInput, consumed by Assess.
type AssessmentInput struct {
	CardUses      []CardUse
	Dispositions  []DispositionUse
	GateProgress  []string
	SnapshotCount int
	WordCounts    []int
	ReviewBands   []string
	GraphSummary  string
	Timeline      []string
	Rounds        []Round

	// ProjectProjection marks whether this assessment should emit the
	// project-only superset (officialProjection + workAndProcess). true for
	// the project surface (the single writing-project template); false for
	// chat/course, which always get nil for both fields regardless of what
	// the model happens to emit.
	ProjectProjection bool
	// WorkSamples are project work-product excerpts (e.g. draft snapshot
	// text) fed to the assessor for the official projection + work-and-
	// process section. Empty on chat/course.
	WorkSamples []string
}

// BuildAssessmentInput digests the process record. Pure — no I/O; the handler
// extracts the primitive slices from studio.ProjectData at the call site.
// `rounds` is the ordered per-round student-turn stream (depth/autonomy/
// prompt-lens evidence); pass nil when a surface cannot supply it (dims fall
// to NA / empty honestly). `projectProjection` and `workSamples` are appended
// last to minimize call-site breakage.
func BuildAssessmentInput(
	events []EventDigest,
	cards []CardUse,
	dispositions []DispositionUse,
	gates []string,
	wordCounts []int,
	reviewBands []string,
	graphSummary string,
	rounds []Round,
	projectProjection bool,
	workSamples []string,
) AssessmentInput {
	timeline := make([]string, 0, len(events))
	for _, e := range events {
		timeline = append(timeline, fmt.Sprintf("%d. %s：%s", e.Order, e.Type, e.Text))
	}
	return AssessmentInput{
		CardUses:          cards,
		Dispositions:      dispositions,
		GateProgress:      gates,
		SnapshotCount:     len(wordCounts),
		WordCounts:        wordCounts,
		ReviewBands:       reviewBands,
		GraphSummary:      graphSummary,
		Timeline:          timeline,
		Rounds:            rounds,
		ProjectProjection: projectProjection,
		WorkSamples:       workSamples,
	}
}
