// Package benchcase is the shared vocabulary between the production packages
// that OWN a prompt and the routing workbench that measures it.
//
// It exists so a bench case can be built from the real system prompt — the
// unexported const in agent or pbl — instead of from a copy. A copied prompt
// drifts, and a routing decision made on a drifted prompt is worse than no
// measurement at all: it looks like evidence.
//
// Nothing on a request path imports this package. See cmd/routebench.
package benchcase

import "mindimprint/api/internal/gateway"

// Case is one representative call: the exact request shape a production call
// site sends, plus how to tell whether a model's answer to it is usable.
type Case struct {
	// Suite groups related single-turn and multi-turn evaluations. It is empty
	// for the older routing cases; a suite is opt-in, never a new serving path.
	Suite string
	// Version changes only when the fixture or its expected teaching behaviour
	// changes. Prompt edits deliberately do not change it: that is what a gate
	// compares.
	Version int
	// ID is stable and human-readable ("dialogue/reading-coach-turn"); it keys
	// results across runs, so renaming one loses its history.
	ID string
	// Class is the capability class this call site routes to.
	Class string
	// Site names the production function this mirrors, so a reader can go check
	// that the case still resembles it.
	Site string
	// Request is what goes on the wire, minus the model — the workbench binds
	// the model per candidate.
	Request gateway.ChatRequest

	// Parse is the production parser boundary. A failure here triggers one retry
	// only for cases that set it, matching endpoints that retry unreadable wire
	// output. It must not contain fixture expectations such as a wanted advance.
	Parse func(text string) error

	// Validate checks the final student-visible result against this fixture's
	// expectations. A failure is evidence, never a reason to call the model
	// again: production does not know a benchmark's expected answer.
	Validate func(text string) error

	// GoldCheck verifies an objective, case-specific semantic expectation. It is
	// deliberately separate from Validate: a correct JSON reply may still make
	// the wrong closed-set decision, and that is not a production-parser error.
	// The workbench applies it to every structurally valid sample. Nil means the
	// case has no objective gold answer and is assessed only by its Judge rubric.
	GoldCheck func(text string) error

	// Judge is the rubric handed to the judging model for the cases whose
	// quality cannot be read off the structure — a chaperone turn is either
	// well-pitched or it is not, and no parser can tell. Empty skips judging.
	Judge string
}
