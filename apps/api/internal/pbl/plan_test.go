package pbl

import "testing"

func TestGradeChange(t *testing.T) {
	cases := []struct {
		name string
		in   ProposedChange
		want Grade
	}{
		{
			"a finished step is progress",
			ProposedChange{Fields: []ChangeField{FieldStepStatus}},
			GradeProgress,
		},
		{
			"reordering and splitting steps is local",
			ProposedChange{Fields: []ChangeField{FieldStepOrder, FieldStepSplit}},
			GradeLocal,
		},
		{
			"moving the success criteria is structural",
			ProposedChange{Fields: []ChangeField{FieldSuccessCriteria}},
			GradeStructural,
		},
		{
			"changing who it is for is structural",
			ProposedChange{Fields: []ChangeField{FieldPeople}},
			GradeStructural,
		},
		{
			// The way this grading most plausibly gets gamed: bury one large
			// change among several small ones and hope it grades local.
			"small edits do not dilute a structural one",
			ProposedChange{Fields: []ChangeField{FieldStepText, FieldStepOrder, FieldQuestion}},
			GradeStructural,
		},
		{
			"keeping both directions is a fork, whatever else moved",
			ProposedChange{Fields: []ChangeField{FieldStepText}, KeepsBothDirections: true},
			GradeFork,
		},
		{
			"nothing named is progress",
			ProposedChange{},
			GradeProgress,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := GradeChange(c.in); got != c.want {
				t.Fatalf("GradeChange = %q, want %q", got, c.want)
			}
		})
	}
}

// 🚨 The gate. `false` lets a change be written straight to the live plan.
func TestNeedsPlanCheck(t *testing.T) {
	if NeedsPlanCheck(GradeProgress) {
		t.Fatal("progress must not interrupt her")
	}
	if NeedsPlanCheck(GradeLocal) {
		t.Fatal("a local change applies with a line and an undo, not a checkpoint")
	}
	if !NeedsPlanCheck(GradeStructural) {
		t.Fatal("a structural change reached the live plan — 隐形重规划 is back")
	}
	if !NeedsPlanCheck(GradeFork) {
		t.Fatal("a fork must be hers to decide")
	}
}

// Every structural field must gate. This is written as a sweep rather than
// per-field so that ADDING a structural field cannot silently skip the gate.
func TestEveryStructuralFieldGates(t *testing.T) {
	for f := range structuralFields {
		g := GradeChange(ProposedChange{Fields: []ChangeField{f}})
		if !NeedsPlanCheck(g) {
			t.Fatalf("field %q graded %q and does not need a Plan Check", f, g)
		}
	}
	for f := range localFields {
		if structuralFields[f] {
			t.Fatalf("field %q is in both the structural and local sets", f)
		}
	}
}

func TestStepStatusesMatchSchema(t *testing.T) {
	want := map[string]bool{
		"settled": true, "tentative": true, "awaiting_evidence": true,
		"awaiting_decision": true, "done": true, "revised": true, "cancelled": true,
	}
	if len(StepStatuses) != len(want) {
		t.Fatalf("StepStatuses = %v", StepStatuses)
	}
	for _, s := range StepStatuses {
		if !want[s] {
			t.Fatalf("%q is in StepStatuses but not in 0109's CHECK", s)
		}
	}
	if IsStepStatus("blocked") {
		t.Fatal("IsStepStatus accepted a status the database will reject")
	}
}
