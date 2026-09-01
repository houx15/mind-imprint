package pbl

import "testing"

func TestRoute_Ordering(t *testing.T) {
	cases := []struct {
		name string
		in   TurnState
		want Mode
	}{
		{
			// Everything else is set; safety still wins.
			"safety outranks everything",
			TurnState{
				SafetyFlag: true, HasConfirmedWork: true, MissingDecision: "谁是目标读者",
				SessionSignal: "矛盾", PendingStructuralChange: true,
			},
			ModeBoundary,
		},
		{
			// 🚨 The anti-连续追问 rung. Real work outranks another question.
			"confirmed work outranks asking and proposing",
			TurnState{
				HasConfirmedWork: true, MissingDecision: "谁是目标读者",
				SessionSignal: "矛盾", SuggestedSessionKind: "reframe",
			},
			ModeDoWork,
		},
		{
			"a blocking decision outranks a session proposal",
			TurnState{MissingDecision: "谁是目标读者", SessionSignal: "矛盾", SuggestedSessionKind: "reframe"},
			ModeAskOne,
		},
		{
			"a live signal proposes a session",
			TurnState{SessionSignal: "四个新生都没看到地图", SuggestedSessionKind: "reframe"},
			ModeProposeSession,
		},
		{
			"a staged structural change asks for a Plan Check",
			TurnState{PendingStructuralChange: true},
			ModePlanCheck,
		},
		{
			"otherwise, carry on",
			TurnState{},
			ModeContinue,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := Route(c.in); got.Mode != c.want {
				t.Fatalf("Route = %q, want %q (reason %q)", got.Mode, c.want, got.Reason)
			}
		})
	}
}

// A proposal she declined does not come back without a new reason. Re-asking is
// how a helpful product becomes a nagging one (doc 03 §十一.2).
func TestRoute_DeclinedSignalDoesNotReturn(t *testing.T) {
	declined := map[string]bool{"四个新生都没看到地图": true}

	same := Route(TurnState{
		SessionSignal: "四个新生都没看到地图", SuggestedSessionKind: "reframe",
		DeclinedSignals: declined,
	})
	if same.Mode == ModeProposeSession {
		t.Fatal("a declined signal was proposed again")
	}

	// A DIFFERENT signal is a new reason, and may be proposed.
	fresh := Route(TurnState{
		SessionSignal: "访谈结果和原来的判断不一样", SuggestedSessionKind: "reframe",
		DeclinedSignals: declined,
	})
	if fresh.Mode != ModeProposeSession {
		t.Fatalf("a new signal was suppressed: %q", fresh.Mode)
	}
}

// Never two high-load sessions back to back: after thinking hard she needs to
// act, not to think about thinking (doc 03 §十四.5).
func TestRoute_NoTwoHeavySessionsInARow(t *testing.T) {
	heavy := Route(TurnState{
		SessionSignal: "又一个矛盾", SuggestedSessionKind: "brainstorm",
		LastTurnWasHeavySession: true,
	})
	if heavy.Mode == ModeProposeSession {
		t.Fatal("a second heavy session was proposed straight after the first")
	}

	// A light one is still fine — capturing an observation is not the same load.
	light := Route(TurnState{
		SessionSignal: "该回现场看看", SuggestedSessionKind: "observation",
		LastTurnWasHeavySession: true,
	})
	if light.Mode != ModeProposeSession {
		t.Fatalf("a light session was suppressed after a heavy one: %q", light.Mode)
	}
}

// The reason travels with the decision: 印记 must be able to name the specific
// trigger, not just the method (doc 03 §十一.3).
func TestRoute_CarriesItsReason(t *testing.T) {
	d := Route(TurnState{SessionSignal: "四个新生都没看到地图", SuggestedSessionKind: "reframe"})
	if d.Reason != "四个新生都没看到地图" {
		t.Fatalf("reason = %q, want the specific trigger", d.Reason)
	}
	if d.SessionKind != "reframe" {
		t.Fatalf("sessionKind = %q", d.SessionKind)
	}
	// Every mode says something; a decision with no reason cannot be explained
	// to her.
	for _, s := range []TurnState{
		{SafetyFlag: true}, {HasConfirmedWork: true}, {MissingDecision: "x"},
		{PendingStructuralChange: true}, {},
	} {
		if Route(s).Reason == "" {
			t.Fatalf("Route(%+v) produced no reason", s)
		}
	}
}

// Every kind the router may suggest must be a real session kind.
func TestRoute_HeavyKindsAreRealKinds(t *testing.T) {
	for k := range heavyKinds {
		if !IsSessionKind(k) {
			t.Fatalf("heavyKinds names %q, which is not a session kind", k)
		}
	}
}
