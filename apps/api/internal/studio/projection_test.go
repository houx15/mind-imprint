package studio

import (
	"testing"

	"github.com/google/uuid"
	"mindimprint/api/internal/skills"
	"mindimprint/api/internal/store/sqlc"
)

func writingSkill(t *testing.T) skills.Skill {
	t.Helper()
	sk, ok := skills.ByID("writing-project")
	if !ok {
		t.Fatal("writing-project not found")
	}
	return sk
}

// gateStateNode builds a gate_state graph_node marking a contract solid.
func gateStateNode(contract string) sqlc.GraphNode {
	return sqlc.GraphNode{ID: uuid.New(), Type: "gate_state",
		Body: []byte(`{"contract":"` + contract + `","confirmed_solid":true,"items":{}}`)}
}

func planNode(route string) *sqlc.GraphNode {
	n := sqlc.GraphNode{ID: uuid.New(), Type: "plan", Body: []byte(`{"route":` + route + `,"reason":"intake"}`)}
	return &n
}

func TestProjectStations_S4Current(t *testing.T) {
	sk := writingSkill(t)
	d := ProjectData{
		Nodes:      nil,
		GateStates: []sqlc.GraphNode{gateStateNode("decode_task"), gateStateNode("frame_question"), gateStateNode("evaluate_perspectives"), gateStateNode("evaluate_sources")},
		Plan:       planNode(`["build_argument","draft_polish","reflect_archive"]`),
	}
	stations, current, err := projectStations(sk, d)
	if err != nil {
		t.Fatal(err)
	}
	if len(stations) != 7 {
		t.Fatalf("want 7 stations, got %d", len(stations))
	}
	// Order + names + views from TopoOrder.
	if stations[0].Code != "S0" || stations[0].Name != "任务解码" || stations[0].View != "评估" {
		t.Fatalf("S0 = %+v", stations[0])
	}
	if stations[4].Code != "S4" || stations[4].Name != "论证构建" || stations[4].View != "结构" {
		t.Fatalf("S4 = %+v", stations[4])
	}
	// S0-S3 done (gate_state confirmed), S4 current (plan head), S5-S6 locked.
	for i, want := range []string{"done", "done", "done", "done", "current", "locked", "locked"} {
		if stations[i].State != want {
			t.Errorf("S%d state = %q, want %q", i, stations[i].State, want)
		}
	}
	if current != "S4" {
		t.Fatalf("current = %q, want S4", current)
	}
}

func TestProjectStations_GateProgress(t *testing.T) {
	sk := writingSkill(t)
	// build_argument gate: 4 machine + 2 student_written + 1 human = 7 items.
	d := ProjectData{Plan: planNode(`["decode_task"]`)}
	stations, _, err := projectStations(sk, d)
	if err != nil {
		t.Fatal(err)
	}
	s4 := stations[4]
	if s4.Gate == nil || s4.Gate.Total != 7 {
		t.Fatalf("S4 gate = %+v, want total 7", s4.Gate)
	}
	if s4.Gate.Passed != 0 {
		t.Fatalf("S4 passed = %d, want 0 (empty graph)", s4.Gate.Passed)
	}
}

// TestProjectStations_GateProgress_NoVacuousPassOnNonEmptyGraph is the
// realistic case the empty-graph test above misses: by the time a student
// reaches build_argument (S4), S0-S3 are solid and the graph already has
// unrelated nodes from earlier contracts (rubric_translation,
// research_question) — but no claim/evidence node yet. build_argument's
// negation-style machine predicates (no_orphan_evidence,
// no_unsupported_claim, no_single_sourced_claim) pass vacuously over that
// non-empty graph because there's still nothing of the scanned type
// (claim/evidence) to check. None of those vacuous passes may count as
// progress — Passed must stay 0, not ~3/7.
func TestProjectStations_GateProgress_NoVacuousPassOnNonEmptyGraph(t *testing.T) {
	sk := writingSkill(t)
	d := ProjectData{
		Nodes: []sqlc.GraphNode{
			{ID: uuid.New(), Type: "rubric_translation", Body: []byte(`{}`)},
			{ID: uuid.New(), Type: "research_question", Body: []byte(`{}`)},
		},
		GateStates: []sqlc.GraphNode{gateStateNode("decode_task"), gateStateNode("frame_question"), gateStateNode("evaluate_perspectives"), gateStateNode("evaluate_sources")},
		Plan:       planNode(`["build_argument","draft_polish","reflect_archive"]`),
	}
	stations, current, err := projectStations(sk, d)
	if err != nil {
		t.Fatal(err)
	}
	if current != "S4" {
		t.Fatalf("current = %q, want S4", current)
	}
	s4 := stations[4]
	if s4.Gate == nil || s4.Gate.Total != 7 {
		t.Fatalf("S4 gate = %+v, want total 7", s4.Gate)
	}
	if s4.Gate.Passed != 0 {
		t.Fatalf("S4 passed = %d, want 0 (no claim/evidence nodes yet — vacuous negation passes must not count on a non-empty graph)", s4.Gate.Passed)
	}
}
