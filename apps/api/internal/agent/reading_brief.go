package agent

// ReadingBrief is the sub-agent's view of the §6 brief-in contract (S2 spec
// §3b): why the student is reading THIS source, injected into every read-turn
// so the router's questions are purposeful instead of generic. Populated from
// the persisted reference.reading_reason/reading_focus/phase_tag plus a
// one-line proposal snapshot — see readingBriefFor (api/reading_brief.go,
// wired for real in Task 3). A zero value degrades the loop to today's
// behavior (no brief = no purposeful nudge, never a hard failure).
type ReadingBrief struct {
	Reason       string // reference.reading_reason
	Focus        string // reference.reading_focus
	PhaseTag     string // reference.phase_tag (label)
	ProposalSnap string // relevant proposal dims, one line — from GetProjectProposal
}
