package agent

import (
	"context"
	"errors"
	"strings"
	"testing"

	"mindimprint/api/internal/gateway"
)

// fakeMomentProvider is a local test double for gateway.Provider. Neither
// existing fake in this package satisfies both things ClassifyMoment's tests
// need at once: gateway.StubProvider (used in coach_test.go/review_test.go)
// records LastRequest but has no way to be made to fail a call; the
// gateway.NewMuxProvider error path (used in gateway/mux_test.go) only errors
// via an unresolved provider name, not a settable per-call error, and does not
// expose the prompt text directly. So this fake — settable reply text OR
// error, plus a call counter and the last prompt string it received — is
// defined locally, matching the brief's fallback instruction.
type fakeMomentProvider struct {
	text       string
	err        error
	calls      int
	lastPrompt string
}

func (f *fakeMomentProvider) Stream(ctx context.Context, r gateway.Resolved, req gateway.ChatRequest) (<-chan gateway.StreamEvent, error) {
	f.calls++
	for _, m := range req.Messages {
		if m.Role == gateway.RoleUser {
			f.lastPrompt = m.Content
		}
	}
	if f.err != nil {
		return nil, f.err
	}
	out := make(chan gateway.StreamEvent, 3)
	out <- gateway.StreamEvent{Kind: gateway.EventTextDelta, TextDelta: f.text}
	out <- gateway.StreamEvent{Kind: gateway.EventUsage, Usage: &gateway.ChatUsage{InputTokens: 11, OutputTokens: 3}}
	out <- gateway.StreamEvent{Kind: gateway.EventDone, StopReason: gateway.StopStop}
	close(out)
	return out, nil
}

func TestMomentCardTableIsComplete(t *testing.T) {
	for _, m := range AllMoments {
		e, ok := momentCard[m]
		if !ok {
			t.Fatalf("moment %q has no momentCard entry", m)
		}
		if e.CardID == "" || e.Desc == "" || e.Flag == "" || e.Reason == "" || e.Criterion == "" {
			t.Fatalf("moment %q has an incomplete entry: %+v", m, e)
		}
	}
	if len(momentCard) != len(AllMoments) {
		t.Fatalf("momentCard has %d entries, AllMoments has %d", len(momentCard), len(AllMoments))
	}
}

func TestEligibleMomentsDropsAnyStatus(t *testing.T) {
	// An offer is never a wall: a card_instance in ANY status — including
	// skipped — makes its moment permanently ineligible.
	for _, status := range []string{"proposed", "active", "completed", "skipped"} {
		got := EligibleMoments([]CardInstanceView{{CardID: "concession", Status: status}})
		for _, m := range got {
			if m == MomentOneSided {
				t.Fatalf("status %q: one_sided still eligible after an offer", status)
			}
		}
		if len(got) != len(AllMoments)-1 {
			t.Fatalf("status %q: got %d eligible, want %d", status, len(got), len(AllMoments)-1)
		}
	}
}

func TestEligibleMomentsAllWhenNoCards(t *testing.T) {
	if got := EligibleMoments(nil); len(got) != len(AllMoments) {
		t.Fatalf("got %d eligible moments, want %d", len(got), len(AllMoments))
	}
}

func TestClassifyMomentParsesExactMatchOnly(t *testing.T) {
	cases := []struct {
		name  string
		reply string
		want  Moment
	}{
		{"exact", "one_sided", MomentOneSided},
		{"trimmed", "  fact_opinion\n", MomentFactOpinion},
		{"none", "none", MomentNone},
		{"chatty", "我认为这是 one_sided 的时机。", MomentNone},
		{"unknown id", "steelman", MomentNone},
		{"empty", "", MomentNone},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			prov := &fakeMomentProvider{text: tc.reply}
			got, usage, err := ClassifyMoment(t.Context(), prov, gateway.Resolved{}, "一段足够长的学生发言内容在这里。", AllMoments)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Fatalf("reply %q: got %q, want %q", tc.reply, got, tc.want)
			}
			// Usage is reported whatever the answer — a `none` still cost money.
			if usage.InputTokens == 0 && usage.OutputTokens == 0 {
				t.Fatalf("reply %q: usage not reported", tc.reply)
			}
		})
	}
}

func TestClassifyMomentRejectsIneligible(t *testing.T) {
	// The model named a real moment that is NOT in the eligible set — it must
	// not be honoured, or a suppressed card would be re-offered.
	prov := &fakeMomentProvider{text: "one_sided"}
	got, _, err := ClassifyMoment(t.Context(), prov, gateway.Resolved{}, "一段足够长的学生发言内容在这里。", []Moment{MomentFactOpinion})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != MomentNone {
		t.Fatalf("got %q, want none", got)
	}
}

func TestClassifyMomentNoCallWhenNothingEligible(t *testing.T) {
	prov := &fakeMomentProvider{text: "one_sided"}
	got, _, err := ClassifyMoment(t.Context(), prov, gateway.Resolved{}, "一段足够长的学生发言内容在这里。", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != MomentNone {
		t.Fatalf("got %q, want none", got)
	}
	if prov.calls != 0 {
		t.Fatalf("provider called %d times with no eligible moments; want 0", prov.calls)
	}
}

func TestClassifyMomentPromptListsOnlyEligible(t *testing.T) {
	prov := &fakeMomentProvider{text: "none"}
	if _, _, err := ClassifyMoment(t.Context(), prov, gateway.Resolved{}, "一段足够长的学生发言内容在这里。", []Moment{MomentFactOpinion}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	joined := prov.lastPrompt
	if !strings.Contains(joined, string(MomentFactOpinion)) {
		t.Fatalf("prompt does not offer the eligible moment: %s", joined)
	}
	if strings.Contains(joined, string(MomentOneSided)) {
		t.Fatalf("prompt leaked an INELIGIBLE moment: %s", joined)
	}
}

func TestClassifyMomentErrorsSurfaceForMetering(t *testing.T) {
	prov := &fakeMomentProvider{err: errors.New("boom")}
	got, _, err := ClassifyMoment(t.Context(), prov, gateway.Resolved{}, "一段足够长的学生发言内容在这里。", AllMoments)
	if err == nil {
		t.Fatal("want an error from a failed provider call")
	}
	if got != MomentNone {
		t.Fatalf("got %q, want none on error", got)
	}
}
