package pbl

import (
	"errors"
	"fmt"
	"strings"
)

// session.go — the rules a session has to obey, as plain functions.
//
// They live here rather than in a handler because they are the spec's
// invariants, and an invariant that can only be tested through HTTP is an
// invariant nobody tests.

// SessionKinds is the closed set, mirroring pbl_session's CHECK (0109).
//
// `free` is an untyped dig — she tapped a hook question, or opened one herself.
// The other four are the Thinking Sessions from
// docs/03-agent-loop-and-thinking-session-protocols.md §十三, which differ only
// in their trigger, the act she performs, and what they write back. The
// conversation carrying them is identical, which is why this is one table with
// a kind rather than five tables.
// 0111 又加了两种：审阅时点开一句话开的那条线，和上线之后每一轮维护。
var SessionKinds = []string{"free", "observation", "reframe", "brainstorm",
	"plan_check", "review", "keeping"}

func IsSessionKind(s string) bool {
	for _, k := range SessionKinds {
		if k == s {
			return true
		}
	}
	return false
}

// MaxSessionDepth is how many levels of nesting a student gets: a session on
// the main thread (depth 0), one inside it (1), one inside that (2).
//
// Product-owner decision 2026-09-01: sessions nest. The cap is ours, and it is
// three because the fourth level is where digging stops being depth and starts
// being lost — and because context assembly walks the parent chain on every
// turn, so an uncapped chain is a cost that stays invisible until it isn't.
const MaxSessionDepth = 3

var (
	ErrNoParent    = errors.New("pbl: parent session does not exist")
	ErrTooDeep     = fmt.Errorf("pbl: sessions nest at most %d deep", MaxSessionDepth)
	ErrBadKind     = errors.New("pbl: unknown session kind")
	ErrNoWriteBack = errors.New("pbl: a session cannot close without its write-back")
)

// ChildDepth validates a proposed nesting and returns the child's depth.
//
// parentDepth is ignored when parentExists is false — that is a top-level
// session, depth 0. Rejecting a missing parent is also the whole
// cycle-prevention story: a tree built one node at a time, where every node
// names an existing ancestor, cannot contain a cycle.
func ChildDepth(parentExists bool, parentDepth int) (int, error) {
	if !parentExists {
		return 0, nil
	}
	if parentDepth < 0 {
		return 0, ErrNoParent
	}
	child := parentDepth + 1
	if child > MaxSessionDepth-1 {
		return 0, ErrTooDeep
	}
	return child, nil
}

// WriteBack is what a session produces when it closes.
//
// Takeaway is the one-line conclusion an untyped dig leaves behind. Fields is
// the structured result a typed session's contract demands — the keys are that
// kind's own vocabulary (a frame, a Next Bet, a plan diff).
type WriteBack struct {
	Takeaway string
	Fields   map[string]string
}

// requiredFields is each kind's write-back contract: what must be non-empty
// for the session to be allowed to close.
//
// 🚨 This is the rule that prevents 方法论表演 (doc 03 §十四.3) — completing the
// ritual while the project's question, evidence and plan stay exactly as they
// were. A session that changes nothing is legitimate; a session that RECORDS
// nothing is not, which is why every contract accepts 「kept, and here is why」
// as a filled field rather than an empty one.
var requiredFields = map[string][]string{
	"free":        {},                       // the takeaway alone is the contract
	"observation": {"observation"},          // at least one thing she actually saw
	"reframe":     {"frame"},                // a new frame, parallel frames, or the old one kept with a reason
	"brainstorm":  {"next_bet"},             // the point is a bet, never a "final solution"
	"plan_check":  {"resolution", "reason"}, // what she decided, and why
	// 审阅里点开的那条线：她可以问完就走，收起时的一句话本身就是记录。
	"review": {},
	// 上线之后的一轮：数据说明了什么。没有这一句，这一轮就只是又看了一次后台。
	"keeping": {"reading"},
}

// RequiredWriteBack reports which fields `kind` must produce to close.
func RequiredWriteBack(kind string) []string {
	return requiredFields[kind]
}

// ValidateClose refuses a session that would close having recorded nothing.
func ValidateClose(kind string, wb WriteBack) error {
	if !IsSessionKind(kind) {
		return ErrBadKind
	}
	// Every kind needs SOMETHING. For an untyped dig that something is the
	// takeaway; without it the digging was just reading, and nothing returns to
	// the thread that spawned it.
	if kind == "free" {
		if strings.TrimSpace(wb.Takeaway) == "" {
			return ErrNoWriteBack
		}
		return nil
	}
	for _, f := range requiredFields[kind] {
		if strings.TrimSpace(wb.Fields[f]) == "" {
			return fmt.Errorf("%w: %s needs %q", ErrNoWriteBack, kind, f)
		}
	}
	return nil
}
