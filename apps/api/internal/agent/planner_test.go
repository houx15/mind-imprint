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
	// The imported milestone_plan node is on the graph, yet decode_task's
	// student_written `milestone_plan` item must STILL be reported as owed —
	// imported content never auto-satisfies a student_written item. (This is
	// the invariant; it holds because CheckGate satisfies student_written items
	// only from recorded status, never from graph presence.)
	g, _ := f.LoadGraph(context.Background(), pid)
	rep := CheckGate(sk, "decode_task", g, RecordedGate{})
	if rep.Solid {
		t.Fatal("imported milestone_plan must not make decode_task solid")
	}
	var item ItemResult
	found := false
	for _, it := range rep.Items {
		if it.Name == "milestone_plan" {
			item, found = it, true
		}
	}
	if !found {
		t.Fatal("decode_task report should list the milestone_plan student_written item")
	}
	if item.Kind != "student_written" {
		t.Fatalf("milestone_plan should be a student_written item, got %q", item.Kind)
	}
	if item.Pass {
		t.Fatal("imported milestone_plan must NOT satisfy the student_written milestone_plan item")
	}
	owed := false
	for _, m := range rep.Missing {
		if m == "milestone_plan 待完成" {
			owed = true
		}
	}
	if !owed {
		t.Fatalf("milestone_plan should be reported as owed in Missing, got %v", rep.Missing)
	}
}

func TestAdvance_MachineOnlyGateAdvancesOnMachineClear(t *testing.T) {
	// A trivial one-machine-item skill: advancing needs only machine_clear.
	sk := skills.Skill{ID: "mini", Kind: "project", Contracts: map[string]skills.Contract{
		"only": {Gate: skills.Gate{Machine: []skills.MachineItem{{Kind: "node_present", Type: "x"}}}},
	}}
	f := &fakeAgentStore{graph: GraphView{Nodes: []GraphNodeView{{ID: "n", Type: "x"}}}}
	deps := AgentDeps{Store: f}
	pid := uuid.New()
	ok, err := Advance(context.Background(), deps, pid, sk, "only")
	if err != nil || !ok {
		t.Fatalf("Advance = %v, %v; want true", ok, err)
	}
	states, _ := f.ListGateStates(context.Background(), pid)
	if !states["only"].Confirmed {
		t.Fatal("machine-only gate should be confirmed solid on advance")
	}
}

func TestAdvance_RefusesWhenMachineItemMissing(t *testing.T) {
	sk, _ := skills.ByID("writing-project")
	f := &fakeAgentStore{graph: GraphView{}} // no perspectives
	deps := AgentDeps{Store: f}
	ok, err := Advance(context.Background(), deps, uuid.New(), sk, "evaluate_perspectives")
	if err != nil {
		t.Fatalf("Advance: %v", err)
	}
	if ok {
		t.Fatal("must refuse: machine items missing")
	}
	if f.lastEvent.Type != "gate_attempt" {
		t.Fatalf("want a gate_attempt event recording what's missing, got %q", f.lastEvent.Type)
	}
}

func TestAdvance_RefusesWhenStudentItemUnrecorded_DEC3(t *testing.T) {
	sk, _ := skills.ByID("writing-project")
	// machine items pass, but student_written items unrecorded → refuse.
	f := &fakeAgentStore{graph: GraphView{Nodes: []GraphNodeView{
		{ID: "p1", Type: "perspective"}, {ID: "p2", Type: "perspective"},
	}}}
	deps := AgentDeps{Store: f}
	ok, _ := Advance(context.Background(), deps, uuid.New(), sk, "evaluate_perspectives")
	if ok {
		t.Fatal("DEC-3: machine may not advance a gate whose student_written items are unrecorded")
	}
}

func TestAdvance_PassesWhenAllItemsSatisfied(t *testing.T) {
	sk, _ := skills.ByID("writing-project")
	f := &fakeAgentStore{
		graph:      GraphView{Nodes: []GraphNodeView{{ID: "p1", Type: "perspective"}, {ID: "p2", Type: "perspective"}}},
		gateStates: map[string]RecordedGate{"evaluate_perspectives": {Items: map[string]string{"recon_logged": "solid", "sources_per_perspective": "solid"}}},
	}
	deps := AgentDeps{Store: f}
	ok, err := Advance(context.Background(), deps, uuid.New(), sk, "evaluate_perspectives")
	if err != nil || !ok {
		t.Fatalf("Advance = %v,%v; want true", ok, err)
	}
}
