package agent

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/uuid"

	"mindimprint/api/internal/cards"
	"mindimprint/api/internal/skills"
)

// TestSecondSkill_ReconcileRouteAdvanceWithZeroNewRuntimeCode is the §5.7
// acceptance test: a second, wholly different skill — authored inline here,
// never embedded in the registry — must run through the exact same
// ReconcileGates / Route / Advance functions the writing-project skill uses,
// with zero new runtime code.
func TestSecondSkill_ReconcileRouteAdvanceWithZeroNewRuntimeCode(t *testing.T) {
	// A bare "note-to-self" project skill: one contract, one machine item, no
	// cards — authored inline, never embedded — runs through the exact same
	// ReconcileGates / Route / Advance the writing-project skill uses.
	sk := skills.Skill{ID: "note-to-self", Kind: "project", Contracts: map[string]skills.Contract{
		"jot": {Gate: skills.Gate{Machine: []skills.MachineItem{{Kind: "node_present", Type: "note"}}}},
	}}
	if err := sk.Validate(); err != nil {
		t.Fatalf("inline skill invalid: %v", err)
	}
	f := &fakeAgentStore{}
	deps := AgentDeps{Store: f}
	pid := uuid.New()

	route, err := Intake(context.Background(), deps, pid, sk, []IntakeCandidate{{Type: "note", Text: "记一笔"}})
	if err != nil {
		t.Fatalf("Intake: %v", err)
	}
	if len(route) != 1 || route[0] != "jot" {
		t.Fatalf("route = %v, want [jot]", route)
	}
	ok, err := Advance(context.Background(), deps, pid, sk, "jot")
	if err != nil || !ok {
		t.Fatalf("Advance = %v,%v; want true (machine-only gate, node present)", ok, err)
	}
}

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

// TestAdvanceUsesConfirmGate is N6 C3: a passing Advance must confirm the
// gate state and record the passed gate_attempt event ATOMICALLY, through
// the single ConfirmGate seam — not via a separate UpsertGateState call
// followed by a separate AppendEvent call. Same fixture as
// TestAdvance_PassesWhenAllItemsSatisfied.
func TestAdvanceUsesConfirmGate(t *testing.T) {
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
	if f.confirmGateCalls != 1 {
		t.Fatalf("want exactly 1 ConfirmGate call, got %d", f.confirmGateCalls)
	}
	if !f.gateStates["evaluate_perspectives"].Confirmed {
		t.Fatal("gate state must be recorded Confirmed via ConfirmGate")
	}
	if f.appendEventCalls != 1 {
		t.Fatalf("want exactly 1 event appended via ConfirmGate, got %d", f.appendEventCalls)
	}
	if f.lastEvent.Type != "gate_attempt" {
		t.Fatalf("want a gate_attempt event, got %q", f.lastEvent.Type)
	}
	var payload map[string]any
	if err := json.Unmarshal(f.lastEvent.Payload, &payload); err != nil {
		t.Fatalf("event payload not JSON: %v", err)
	}
	if payload["result"] != "passed" {
		t.Fatalf("want result=passed, got %v", payload["result"])
	}
}

// TestAdvanceAll_HoldsBehindUnsolidPredecessor is the rule Advance alone
// cannot enforce (it only ever inspects one contract's own gate items):
// evaluate_perspectives's own gate is made GENUINELY fully satisfied — its
// machine item (2 "perspective" nodes) passes on the graph AND both its
// student_written items ("recon_logged", "sources_per_perspective") are
// recorded solid — while frame_question (its predecessor, and in turn
// decode_task) is left with nothing on the graph to satisfy its own machine
// items. If AdvanceAll only checked each contract's own gate (as Advance
// does), evaluate_perspectives would confirm here — the assertion below would
// pass vacuously. It must not confirm, because its predecessor chain never
// went solid.
func TestAdvanceAll_HoldsBehindUnsolidPredecessor(t *testing.T) {
	sk, _ := skills.ByID("writing-project")
	f := &fakeAgentStore{
		graph: GraphView{Nodes: []GraphNodeView{{ID: "p1", Type: "perspective"}, {ID: "p2", Type: "perspective"}}},
		gateStates: map[string]RecordedGate{
			"evaluate_perspectives": {Items: map[string]string{"recon_logged": "solid", "sources_per_perspective": "solid"}},
		},
	}
	deps := AgentDeps{Store: f}
	pid := uuid.New()

	// Sanity check the fixture is not accidentally vacuous: evaluate_
	// perspectives's OWN gate must report nothing missing before we ever call
	// AdvanceAll, and frame_question's own gate must be unmet.
	g, _ := f.LoadGraph(context.Background(), pid)
	if rep := CheckGate(sk, "evaluate_perspectives", g, f.gateStates["evaluate_perspectives"]); len(rep.Missing) != 0 {
		t.Fatalf("fixture invalid: evaluate_perspectives must be fully satisfied on its own gate, got Missing=%v", rep.Missing)
	}
	if rep := CheckGate(sk, "frame_question", g, RecordedGate{}); len(rep.Missing) == 0 {
		t.Fatal("fixture invalid: frame_question must be left unmet")
	}

	advanced, err := AdvanceAll(context.Background(), deps, pid, sk)
	if err != nil {
		t.Fatalf("AdvanceAll: %v", err)
	}
	for _, id := range advanced {
		if id == "evaluate_perspectives" {
			t.Fatalf("evaluate_perspectives must not advance behind an unsolid predecessor, got advanced=%v", advanced)
		}
	}
	if len(advanced) != 0 {
		t.Fatalf("nothing in this DAG should advance (every contract sits behind an unsolid predecessor), got %v", advanced)
	}
	states, _ := f.ListGateStates(context.Background(), pid)
	if states["evaluate_perspectives"].Confirmed {
		t.Fatal("evaluate_perspectives must not be recorded confirmed")
	}
}

// TestAdvanceAll_CascadesInTopoOrder proves the single-pass walk: one call can
// legitimately close several stations, because a contract confirmed earlier in
// the SAME walk counts as solid for its successors' `requires` check.
// decode_task and frame_question are both built to be genuinely, fully
// satisfiable on the graph — but frame_question requires decode_task, which
// starts unconfirmed. Only a single-pass walk that lets decode_task's
// just-earned solidity feed frame_question's readiness check can close both
// in one call.
func TestAdvanceAll_CascadesInTopoOrder(t *testing.T) {
	sk, _ := skills.ByID("writing-project")
	f := &fakeAgentStore{
		graph: GraphView{Nodes: []GraphNodeView{
			{ID: "rt", Type: "rubric_translation"},
			{ID: "wp1", Type: "weakness_prediction"},
			{ID: "wp2", Type: "weakness_prediction"},
			{ID: "rq", Type: "research_question"},
			{ID: "pa", Type: "provisional_answer"},
			{ID: "pr", Type: "preregistration"},
		}},
		gateStates: map[string]RecordedGate{
			"decode_task":    {Items: map[string]string{"milestone_plan": "solid"}},
			"frame_question": {Items: map[string]string{"terms_defined": "solid"}},
		},
	}
	deps := AgentDeps{Store: f}
	pid := uuid.New()

	advanced, err := AdvanceAll(context.Background(), deps, pid, sk)
	if err != nil {
		t.Fatalf("AdvanceAll: %v", err)
	}
	want := []string{"decode_task", "frame_question"}
	if len(advanced) != len(want) {
		t.Fatalf("advanced = %v, want %v", advanced, want)
	}
	for i, id := range want {
		if advanced[i] != id {
			t.Fatalf("advanced = %v, want %v (topo order matters)", advanced, want)
		}
	}
	states, _ := f.ListGateStates(context.Background(), pid)
	if !states["decode_task"].Confirmed {
		t.Fatal("decode_task must be recorded confirmed")
	}
	if !states["frame_question"].Confirmed {
		t.Fatal("frame_question must be recorded confirmed — it was only reachable because decode_task advanced earlier in this same walk")
	}
	// evaluate_perspectives has no perspective nodes and no recorded items on
	// this fixture, so the cascade correctly stops there rather than running
	// away through the rest of the DAG.
	for _, id := range advanced {
		if id == "evaluate_perspectives" {
			t.Fatalf("evaluate_perspectives has an unmet gate on this fixture and must not have advanced, got %v", advanced)
		}
	}
	if f.upsertPlanCalls != 1 {
		t.Fatalf("want Replan called exactly once when something advanced, got %d plan upserts", f.upsertPlanCalls)
	}
}

// TestAdvanceAll_NoopWhenNothingChanged covers both halves of the noop
// contract: an already-solid contract (decode_task, seeded Confirmed:true) is
// skipped outright — no CheckGate/Advance call, hence no duplicate
// gate_attempt event — and every other contract in this DAG has an
// (unavoidably) unmet own-gate on an empty graph, so nothing else advances
// either. An empty result means Replan must not be called.
func TestAdvanceAll_NoopWhenNothingChanged(t *testing.T) {
	sk, _ := skills.ByID("writing-project")
	f := &fakeAgentStore{
		graph: GraphView{}, // nothing on the graph for any other contract's machine items
		gateStates: map[string]RecordedGate{
			"decode_task": {Confirmed: true, Items: map[string]string{"milestone_plan": "solid"}},
		},
	}
	deps := AgentDeps{Store: f}
	pid := uuid.New()

	advanced, err := AdvanceAll(context.Background(), deps, pid, sk)
	if err != nil {
		t.Fatalf("AdvanceAll: %v", err)
	}
	if len(advanced) != 0 {
		t.Fatalf("want no contract to advance, got %v", advanced)
	}
	if f.appendEventCalls != 0 {
		t.Fatalf("want zero gate_attempt events (decode_task already solid must be skipped without a re-check, and every other contract's own gate is unmet on an empty graph so Advance is never called), got %d", f.appendEventCalls)
	}
	if f.upsertPlanCalls != 0 {
		t.Fatal("want Replan not called when the result is empty")
	}
}
