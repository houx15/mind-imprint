package liteassign

import "time"

// LockReasonPastDue is the only reason a submitted writing homework is locked.
const LockReasonPastDue = "past_due"

// LockFacts are the inputs of the lock rule for one writing.
type LockFacts struct {
	// Homework: the writing is linked to a recipient row of a live (not archived) assignment.
	Homework bool
	// HasVersion: at least one version was submitted.
	HasVersion bool
	// DueAt is the assignment deadline. Unused when Homework is false.
	DueAt time.Time
	// ReturnedAt and ReturnDueAt are set when the teacher returned the writing.
	ReturnedAt  *time.Time
	ReturnDueAt *time.Time
}

// EffectiveDue is the return deadline once the teacher has returned the
// writing, otherwise the assignment deadline.
func EffectiveDue(f LockFacts) time.Time {
	if f.ReturnedAt != nil && f.ReturnDueAt != nil {
		return *f.ReturnDueAt
	}
	return f.DueAt
}

// Locked reports whether a writing can no longer be edited. Only a submitted
// homework past its effective deadline is locked: a homework with no version
// can still be submitted late, and a writing that is not homework never locks.
func Locked(f LockFacts, now time.Time) bool {
	return f.Homework && f.HasVersion && now.After(EffectiveDue(f))
}

// LockReason is LockReasonPastDue when the writing is locked, "" otherwise.
func LockReason(f LockFacts, now time.Time) string {
	if Locked(f, now) {
		return LockReasonPastDue
	}
	return ""
}
