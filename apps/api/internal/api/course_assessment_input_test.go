package api

import (
	"strings"
	"testing"

	"mindimprint/api/internal/store/sqlc"
	"mindimprint/api/internal/studio"
)

// TestBuildAssessmentInputFromSession: a course session's evidence is its
// event stream and its cards. It has no gates, snapshots, or graph — those
// stay empty, and Assess's own NA defaults report the unevidenced dimensions
// honestly (spec §4). Pure function, no I/O.
func TestBuildAssessmentInputFromSession(t *testing.T) {
	events := []studio.Event{
		{Type: "course_message", Surface: "course", Payload: []byte(`{"unprompted":true}`)},
		{Type: "phase_advanced", Surface: "course", Payload: []byte(`{"to":"guided"}`)},
	}
	cards := []sqlc.CardInstance{
		{CardID: "craap", Status: "completed"},
		{CardID: "concession", Status: "skipped"},
	}

	in := buildAssessmentInputFromSession(events, cards)

	if len(in.Timeline) != 2 {
		t.Fatalf("Timeline = %v, want one line per event", in.Timeline)
	}
	if !strings.Contains(in.Timeline[0], "course_message") || !strings.Contains(in.Timeline[0], "unprompted") {
		t.Fatalf("Timeline[0] = %q, want the event type and its payload rendered", in.Timeline[0])
	}
	if len(in.CardUses) != 2 || in.CardUses[0].CardID != "craap" {
		t.Fatalf("CardUses = %+v, want one per session card", in.CardUses)
	}
	if len(in.Dispositions) != 2 {
		t.Fatalf("Dispositions = %+v, want one per session card (completed and skipped both count)", in.Dispositions)
	}

	// A course session genuinely has none of these. Empty is the honest answer
	// — Assess defaults the unevidenced dimensions to NA.
	if len(in.GateProgress) != 0 || len(in.WordCounts) != 0 || len(in.ReviewBands) != 0 || in.GraphSummary != "" {
		t.Fatalf("course input claims studio-only evidence: gates=%v words=%v bands=%v graph=%q",
			in.GateProgress, in.WordCounts, in.ReviewBands, in.GraphSummary)
	}
	if in.SnapshotCount != 0 {
		t.Fatalf("SnapshotCount = %d, want 0 (a course session has no drafts)", in.SnapshotCount)
	}
}

// An empty session must not panic and must not invent evidence.
func TestBuildAssessmentInputFromSessionEmpty(t *testing.T) {
	in := buildAssessmentInputFromSession(nil, nil)
	if len(in.Timeline) != 0 || len(in.CardUses) != 0 || len(in.Dispositions) != 0 {
		t.Fatalf("empty session produced evidence: %+v", in)
	}
}
