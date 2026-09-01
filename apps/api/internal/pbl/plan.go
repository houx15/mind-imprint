package pbl

// plan.go — how a proposed change to the plan is graded, and what a step's
// status may be.

// StepStatuses mirrors pbl_plan_step's CHECK (0109), which mirrors
// docs/02-project-conception-and-dynamic-planning-design.md §十.2.
//
// The two waiting states carry most of the value. They let the board say WHY
// nothing is moving — waiting on evidence, waiting on her — and that is exactly
// the thing a stuck student cannot usually put into words.
var StepStatuses = []string{
	"settled", "tentative", "awaiting_evidence", "awaiting_decision",
	"done", "revised", "cancelled",
}

func IsStepStatus(s string) bool {
	for _, k := range StepStatuses {
		if k == s {
			return true
		}
	}
	return false
}

// Grade is how much ceremony a plan change has to go through.
type Grade string

const (
	// GradeProgress — applied silently. A step finished; a file exists.
	GradeProgress Grade = "progress"
	// GradeLocal — applied, announced in one line, undoable.
	GradeLocal Grade = "local"
	// GradeStructural — must go through Plan Check and get her decision.
	GradeStructural Grade = "structural"
	// GradeFork — two directions are both worth keeping; 印记 proposes a branch.
	GradeFork Grade = "fork"
)

// ChangeField names what a proposed change touches. The grading is a property
// of WHAT moved, not of how many words moved.
type ChangeField string

const (
	FieldStepStatus      ChangeField = "step_status"
	FieldStepOrder       ChangeField = "step_order"
	FieldStepSplit       ChangeField = "step_split"
	FieldStepText        ChangeField = "step_text"
	FieldQuestion        ChangeField = "question"
	FieldPeople          ChangeField = "people"
	FieldNeeds           ChangeField = "needs"
	FieldSolution        ChangeField = "solution"
	FieldSuccessCriteria ChangeField = "success_criteria"
	FieldScope           ChangeField = "scope"
)

// structuralFields are the ones doc 03 §十.1 says cannot move without her.
//
// They are the project's identity: what it is asking, who it is for, what it
// is making, and what would count as having worked. 印记 may propose any of
// them and may not apply any of them.
var structuralFields = map[ChangeField]bool{
	FieldQuestion:        true,
	FieldPeople:          true,
	FieldNeeds:           true,
	FieldSolution:        true,
	FieldSuccessCriteria: true,
	FieldScope:           true,
}

// localFields move the shape of the work without moving what the work is.
var localFields = map[ChangeField]bool{
	FieldStepOrder: true,
	FieldStepSplit: true,
	FieldStepText:  true,
}

// ProposedChange is what 印记 wants to do to the plan.
type ProposedChange struct {
	Fields []ChangeField
	// KeepsBothDirections marks the case where the honest answer is "both are
	// worth keeping" rather than a choice between them.
	KeepsBothDirections bool
}

// GradeChange decides how much ceremony a change needs.
//
// The ordering matters: a fork outranks structural, and any structural field
// outranks any number of local ones. A change that renames three steps AND
// moves the success criteria is structural — the small edits do not dilute the
// large one, which is the way this grading most plausibly gets gamed.
func GradeChange(c ProposedChange) Grade {
	if c.KeepsBothDirections {
		return GradeFork
	}
	local := false
	for _, f := range c.Fields {
		if structuralFields[f] {
			return GradeStructural
		}
		if localFields[f] {
			local = true
		}
	}
	if local {
		return GradeLocal
	}
	return GradeProgress
}

// NeedsPlanCheck reports whether a change may reach the live plan directly.
//
// 🚨 The single most important line in this package. `false` here is what lets
// a change be written to pbl_plan_step; `true` sends it to pbl_pending_change
// instead, where it waits for her. 隐形重规划 — 印记 quietly swapping the goal
// because new information arrived — is exactly what returning the wrong answer
// here would reintroduce.
func NeedsPlanCheck(g Grade) bool {
	return g == GradeStructural || g == GradeFork
}
