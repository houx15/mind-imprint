package api

import (
	"encoding/json"
	"strings"
	"testing"

	"mindimprint/api/internal/store/sqlc"
	"mindimprint/api/internal/studio"
)

// TestBuildAssessmentInputFromEvidence_CourseShape: a course session's
// evidence is its event stream and its cards. It has no gates, snapshots, or
// graph — those stay empty, and Assess's own NA defaults report the
// unevidenced dimensions honestly (spec §4). Pure function, no I/O.
func TestBuildAssessmentInputFromEvidence_CourseShape(t *testing.T) {
	events := []studio.Event{
		{Type: "course_message", Surface: "course", Payload: []byte(`{"unprompted":true}`)},
		{Type: "phase_advanced", Surface: "course", Payload: []byte(`{"to":"guided"}`)},
	}
	cards := []sqlc.CardInstance{
		{CardID: "craap", Status: "completed"},
		{CardID: "concession", Status: "skipped"},
	}

	in := buildAssessmentInputFromEvidence(events, cards)

	if len(in.Timeline) != 2 {
		t.Fatalf("Timeline = %v, want one line per event", in.Timeline)
	}
	if !strings.Contains(in.Timeline[0], "course_message") || !strings.Contains(in.Timeline[0], "unprompted") {
		t.Fatalf("Timeline[0] = %q, want the event type and its payload rendered", in.Timeline[0])
	}
	if len(in.CardUses) != 2 || in.CardUses[0].CardID != "craap" {
		t.Fatalf("CardUses = %+v, want one per session card", in.CardUses)
	}
	// A course session has no Equipment projection to zip Spont against, and no
	// studio derivation for Dimension — both must stay empty rather than a
	// fabricated guess (a course session's evidence must never invent a
	// 自发/提示后 signal that does not exist for it).
	for _, cu := range in.CardUses {
		if cu.Spont != "" || cu.Dimension != "" {
			t.Fatalf("CardUses = %+v, want Spont and Dimension left empty (unknowable for a course session)", in.CardUses)
		}
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

// TestPromptText_RealTextVsHonestEmpty proves the C1 fix: a prompt_sent event
// whose payload actually carries {"text": "..."} (as studioturn.go/chat.go
// now enrich it) yields that real text, and an empty payload yields "" —
// NEVER eventText's bare event-type fallback ("prompt_sent" itself), which
// would otherwise be fed to the assessor and read as fabricatable "real"
// evidence (whole-branch review C1).
func TestPromptText_RealTextVsHonestEmpty(t *testing.T) {
	withText := studio.Event{Type: "prompt_sent", Surface: "studio", Payload: []byte(`{"text":"我想改 thesis"}`)}
	if got := promptText(withText); got != "我想改 thesis" {
		t.Fatalf("promptText(with text) = %q, want %q", got, "我想改 thesis")
	}
	empty := studio.Event{Type: "prompt_sent", Surface: "studio", Payload: []byte(`{}`)}
	if got := promptText(empty); got != "" {
		t.Fatalf("promptText(empty payload) = %q, want \"\" (not the bare event type)", got)
	}
	if got := promptText(empty); got == empty.Type {
		t.Fatalf("promptText(empty payload) fell back to the event type %q — must be honest empty", empty.Type)
	}
	noPayload := studio.Event{Type: "prompt_sent", Surface: "studio"}
	if got := promptText(noPayload); got != "" {
		t.Fatalf("promptText(nil payload) = %q, want \"\"", got)
	}
}

// TestRoundsFromProject_UsesRealPromptText proves roundsFromProject reads the
// real student text via promptText, not eventText — a prompt_sent event with
// real text yields that text on the Round, and one with an empty payload
// yields "" rather than the literal "prompt_sent" (whole-branch review C1).
func TestRoundsFromProject_UsesRealPromptText(t *testing.T) {
	d := studio.ProjectData{Events: []studio.Event{
		{Type: "card_surfaced", Surface: "studio", Payload: []byte(`{"card_id":"sift_craap"}`)},
		{Type: "prompt_sent", Surface: "studio", Payload: []byte(`{"text":"我想改 thesis"}`)},
		{Type: "prompt_sent", Surface: "studio", Payload: []byte(`{}`)},
	}}
	rounds := roundsFromProject(d)
	if len(rounds) != 2 {
		t.Fatalf("rounds = %+v, want 2", rounds)
	}
	if rounds[0].StudentPrompt != "我想改 thesis" {
		t.Fatalf("rounds[0].StudentPrompt = %q, want the real text", rounds[0].StudentPrompt)
	}
	if rounds[1].StudentPrompt != "" {
		t.Fatalf("rounds[1].StudentPrompt = %q, want honest empty (not \"prompt_sent\")", rounds[1].StudentPrompt)
	}
}

// TestRoundsFromEvidence_UsesRealPromptText mirrors the project-side test for
// the course/chat evidence path (course_assessment_input.go).
func TestRoundsFromEvidence_UsesRealPromptText(t *testing.T) {
	events := []studio.Event{
		{Type: "prompt_sent", Surface: "chat", Payload: []byte(`{"text":"这段论据够吗"}`)},
		{Type: "prompt_sent", Surface: "chat", Payload: []byte(`{}`)},
	}
	rounds := roundsFromEvidence(events)
	if len(rounds) != 2 {
		t.Fatalf("rounds = %+v, want 2", rounds)
	}
	if rounds[0].StudentPrompt != "这段论据够吗" {
		t.Fatalf("rounds[0].StudentPrompt = %q, want the real text", rounds[0].StudentPrompt)
	}
	if rounds[1].StudentPrompt != "" {
		t.Fatalf("rounds[1].StudentPrompt = %q, want honest empty (not \"prompt_sent\")", rounds[1].StudentPrompt)
	}
}

// TestRoundsFromEvidence_CourseMessageIsARound proves the course surface now
// yields per-round evidence: a course_message event carrying real text becomes a
// Round via the same {"text":…} reader chat's prompt_sent uses, so course SOLO /
// 提示词透镜 are no longer empty by construction. A course_message with no text
// still yields an honest empty prompt (never a fabricated one).
func TestRoundsFromEvidence_CourseMessageIsARound(t *testing.T) {
	events := []studio.Event{
		{Type: "card_surfaced", Surface: "course", Payload: []byte(`{"card_id":"sift_craap"}`)},
		{Type: "course_message", Surface: "course", Payload: []byte(`{"unprompted":true,"text":"这条数据可信吗？"}`)},
		{Type: "course_message", Surface: "course", Payload: []byte(`{"unprompted":true}`)},
	}
	rounds := roundsFromEvidence(events)
	if len(rounds) != 2 {
		t.Fatalf("rounds = %+v, want 2 (two course_message turns, card_surfaced is not a turn)", rounds)
	}
	if rounds[0].StudentPrompt != "这条数据可信吗？" {
		t.Fatalf("rounds[0].StudentPrompt = %q, want the real course text", rounds[0].StudentPrompt)
	}
	if rounds[1].StudentPrompt != "" {
		t.Fatalf("rounds[1].StudentPrompt = %q, want honest empty (no text in payload)", rounds[1].StudentPrompt)
	}
}

// An empty session must not panic and must not invent evidence.
func TestBuildAssessmentInputFromEvidenceEmpty(t *testing.T) {
	in := buildAssessmentInputFromEvidence(nil, nil)
	if len(in.Timeline) != 0 || len(in.CardUses) != 0 || len(in.Dispositions) != 0 {
		t.Fatalf("empty session produced evidence: %+v", in)
	}
}

// TestBuildAssessmentInputFromEvidence_ChatShape confirms the shared builder
// produces NA output dimensions for chat evidence — a chat thread has no
// gates/word-counts/review-bands/graph — those args are empty so Assess
// reports the writing dimensions NA (spec §DEC-A2.4).
func TestBuildAssessmentInputFromEvidence_ChatShape(t *testing.T) {
	events := []studio.Event{
		{Type: "prompt_sent", Surface: "chat", Payload: json.RawMessage(`{}`)},
		{Type: "card_surfaced", Surface: "chat", Payload: json.RawMessage(`{"card_id":"sift_craap"}`)},
	}
	cards := []sqlc.CardInstance{{CardID: "sift_craap", Status: "completed"}}
	in := buildAssessmentInputFromEvidence(events, cards)
	if len(in.GateProgress) != 0 || len(in.WordCounts) != 0 || in.GraphSummary != "" {
		t.Fatalf("chat evidence must carry no gate/wordcount/graph args: %+v", in)
	}
	if len(in.CardUses) != 1 || in.CardUses[0].CardID != "sift_craap" {
		t.Fatalf("card evidence not mapped: %+v", in.CardUses)
	}
}
