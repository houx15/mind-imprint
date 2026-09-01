package pbl

// router.go — the per-turn decision order.
//
// From docs/03-agent-loop-and-thinking-session-protocols.md §三.3. It is a pure
// function over an explicit state struct, so the ORDER can be tested without a
// model in the way — and the order is the whole point.
//
// What it exists to prevent: 印记 delaying action because it could always ask
// one more question. Doc 03 §十四.1 names that failure 连续追问 — every turn a
// "why", with the student producing all the content. The ordering answers it
// structurally: doing real work outranks asking, and asking outranks proposing
// a session.

// Mode is what this turn should be.
type Mode string

const (
	// ModeBoundary — a safety, privacy or permission problem. Nothing else
	// happens this turn.
	ModeBoundary Mode = "boundary"
	// ModeDoWork — advance something she has already decided. The default when
	// there is real work available.
	ModeDoWork Mode = "do_work"
	// ModeAskOne — one decision is missing that would change what happens next.
	// Exactly one, never a list.
	ModeAskOne Mode = "ask_one"
	// ModeProposeSession — a signal that needs structured thinking. A proposal,
	// never an interruption: she can decline.
	ModeProposeSession Mode = "propose_session"
	// ModePlanCheck — the plan's structure is about to change.
	ModePlanCheck Mode = "plan_check"
	// ModeContinue — carry on with the current step.
	ModeContinue Mode = "continue"
)

// TurnState is everything the ordering reads. Assembled by the caller from the
// project's rows; deliberately plain values, so this stays testable.
type TurnState struct {
	// SafetyFlag is set when the turn raises a safety, privacy or permission
	// question — a real name, someone else's photo, a place she should not go.
	SafetyFlag bool

	// HasConfirmedWork is true when something she already decided can be
	// advanced right now.
	HasConfirmedWork bool

	// MissingDecision names the one decision whose absence would change what
	// happens next. Empty when nothing is blocked on her.
	MissingDecision string

	// SessionSignal names the structured-thinking signal this turn raised — a
	// contradiction, a question that is too big, an answer baked into the
	// question. Empty when there is none.
	SessionSignal string
	// SuggestedSessionKind is the kind that signal calls for.
	SuggestedSessionKind string

	// DeclinedSignals are signals she has already turned down. A proposal she
	// declined does not come back without a NEW reason (doc 03 §十一.2) —
	// re-asking is how a helpful product becomes a nagging one.
	DeclinedSignals map[string]bool

	// LastTurnWasHeavySession is true when she just finished a high-load
	// session. Never two in a row: after thinking hard, what she needs is to
	// act, not to think about thinking.
	LastTurnWasHeavySession bool

	// PendingStructuralChange is true when a structural change is staged and
	// waiting for her.
	PendingStructuralChange bool
}

// Decision is what the router chose, and why. The reason is not decoration:
// 印记 has to be able to say why it is proposing something, and doc 03 §十一.3
// requires naming the specific trigger rather than the method's name.
type Decision struct {
	Mode        Mode
	SessionKind string
	Reason      string
}

// heavyKinds are the sessions that cost real cognitive effort. Two of these
// back to back is the 「Session 打断过多」 failure (doc 03 §十四.5).
var heavyKinds = map[string]bool{
	"reframe":    true,
	"brainstorm": true,
}

// Route picks this turn's mode. The ordering is the contract; see the file
// comment for why each rung sits where it does.
func Route(s TurnState) Decision {
	// 1 · Boundaries first. Nothing is worth doing on top of an unresolved
	// safety or privacy question.
	if s.SafetyFlag {
		return Decision{Mode: ModeBoundary, Reason: "safety or privacy first"}
	}

	// 2 · Do real work on a direction she has already confirmed. This rung is
	// ABOVE asking on purpose: an agent that always finds one more question is
	// an agent that never helps.
	if s.HasConfirmedWork {
		return Decision{Mode: ModeDoWork, Reason: "a confirmed direction can be advanced"}
	}

	// 3 · Ask for exactly the one decision that is blocking.
	if s.MissingDecision != "" {
		return Decision{Mode: ModeAskOne, Reason: s.MissingDecision}
	}

	// 4 · Propose a session, if the signal is live, not already declined, and
	// she is not fresh out of a heavy one.
	if s.SessionSignal != "" && !s.DeclinedSignals[s.SessionSignal] {
		if !(s.LastTurnWasHeavySession && heavyKinds[s.SuggestedSessionKind]) {
			return Decision{
				Mode:        ModeProposeSession,
				SessionKind: s.SuggestedSessionKind,
				Reason:      s.SessionSignal,
			}
		}
	}

	// 5 · A staged structural change is waiting for her judgement.
	if s.PendingStructuralChange {
		return Decision{Mode: ModePlanCheck, Reason: "a structural change is waiting for her"}
	}

	// 6 · Otherwise keep the project's rhythm.
	return Decision{Mode: ModeContinue, Reason: "carry on with the current step"}
}
