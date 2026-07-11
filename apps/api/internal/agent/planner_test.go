package agent

import (
	"testing"

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
