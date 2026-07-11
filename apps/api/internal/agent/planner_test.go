package agent

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"mindimprint/api/internal/cards"
	"mindimprint/api/internal/skills"
)

func TestWritingProject_CardsResolveInRegistry(t *testing.T) {
	sk, _ := skills.ByID("writing-project")
	for _, id := range sk.Cards {
		if _, ok := cards.ByID(id); !ok {
			t.Fatalf("writing-project references unknown card %q", id)
		}
	}
	for cid, c := range sk.Contracts {
		for _, id := range c.Repertoire {
			if _, ok := cards.ByID(id); !ok {
				t.Fatalf("contract %s repertoire references unknown card %q", cid, id)
			}
		}
	}
}

func TestRoute_RespectsRequiresAndStartsFromFrontier(t *testing.T) {
	sk, _ := skills.ByID("writing-project")
	// Nothing done: only decode_task (no requires) is routable.
	reports := map[string]GateReport{}
	for id := range sk.Contracts {
		reports[id] = GateReport{Contract: id, Status: "empty"}
	}
	route := Route(sk, reports)
	if len(route) == 0 || route[0] != "decode_task" {
		t.Fatalf("route should start at decode_task, got %v", route)
	}
	for _, id := range route {
		if id == "build_argument" {
			t.Fatal("build_argument must not be routable before evaluate_sources clears")
		}
	}
}

func TestRoute_UnlocksNextWhenPredecessorMachineClear(t *testing.T) {
	sk, _ := skills.ByID("writing-project")
	reports := map[string]GateReport{}
	for id := range sk.Contracts {
		reports[id] = GateReport{Contract: id, Status: "empty"}
	}
	reports["decode_task"] = GateReport{Contract: "decode_task", Status: "machine_clear", Solid: true}
	route := Route(sk, reports)
	// decode_task is Solid → excluded; frame_question now routable.
	for _, id := range route {
		if id == "decode_task" {
			t.Fatal("solid contract must be excluded from the route")
		}
	}
	if route[0] != "frame_question" {
		t.Fatalf("frame_question should be the new frontier, got %v", route)
	}
}

func TestIntake_MintsImportedNodesAndWritesFirstPlan(t *testing.T) {
	sk, _ := skills.ByID("writing-project")
	f := &fakeAgentStore{}
	deps := AgentDeps{Store: f}
	pid := uuid.New()

	// A mid-way arrival: a research question + provisional answer already written.
	route, err := Intake(context.Background(), deps, pid, sk, []IntakeCandidate{
		{Type: "research_question", Text: "中国的经济转型是否让地球更可持续？"},
		{Type: "provisional_answer", Text: "部分是。"},
	})
	if err != nil {
		t.Fatalf("Intake: %v", err)
	}
	if f.insertGraphNodeCalls != 2 {
		t.Fatalf("want 2 imported nodes, got %d", f.insertGraphNodeCalls)
	}
	if f.lastImportedAuthor != "imported" {
		t.Fatalf("imported nodes must carry author=imported, got %q", f.lastImportedAuthor)
	}
	if f.upsertPlanCalls != 1 {
		t.Fatal("Intake must write the first plan")
	}
	// route starts at decode_task (nothing there yet); frame_question is only
	// partial (no preregistration) so it is not Solid — imported content does
	// not skip a gate.
	if len(route) == 0 || route[0] != "decode_task" {
		t.Fatalf("route should start at decode_task, got %v", route)
	}
}

func TestIntake_ImportedDoesNotSatisfyStudentWritten(t *testing.T) {
	sk, _ := skills.ByID("writing-project")
	f := &fakeAgentStore{}
	deps := AgentDeps{Store: f}
	pid := uuid.New()
	_, err := Intake(context.Background(), deps, pid, sk, []IntakeCandidate{
		{Type: "milestone_plan", Text: "我的计划"}, // imported, not student-authored-in-tool
	})
	if err != nil {
		t.Fatalf("Intake: %v", err)
	}
	// decode_task still owes its machine items AND milestone_plan is not solid.
	g, _ := f.LoadGraph(context.Background(), pid)
	rep := CheckGate(sk, "decode_task", g, RecordedGate{})
	if rep.Solid {
		t.Fatal("imported milestone_plan must not make decode_task solid")
	}
}
