package studio

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"mindimprint/api/internal/cards"
	"mindimprint/api/internal/skills"
	"mindimprint/api/internal/store/sqlc"
)

func pgUUID(u uuid.UUID) pgtype.UUID { return pgtype.UUID{Bytes: u, Valid: true} }

func strPtr(s string) *string { return &s }

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

func TestProjectCoach_AnchorAndThread(t *testing.T) {
	crit := "D5"
	d := ProjectData{
		Interventions: []sqlc.Intervention{
			// flag carries a criterion too, but its headline is the anchor label.
			{Type: "flag", Body: "图上有一处孤儿证据", Criterion: &crit, Anchor: []byte(`{"label":"孤儿证据"}`)},
			{Type: "diagnostic", Body: "连到治理决心", Criterion: &crit, Anchor: []byte(`{"label":"论证图 · 治理决心主张"}`)},
		},
	}
	coach := projectCoach(d, "论证构建")
	if coach.Anchor != "论证图 · 治理决心主张" { // latest intervention's anchor
		t.Fatalf("anchor = %q", coach.Anchor)
	}
	// flag label = anchor.label (not the criterion), even when a criterion is set.
	if len(coach.Messages) != 2 || coach.Messages[0].Kind != "flag" || coach.Messages[0].Label != "孤儿证据" {
		t.Fatalf("messages = %+v", coach.Messages)
	}
	if coach.Messages[1].Kind != "ai" || coach.Messages[1].Tag != "D5" || coach.Messages[1].Anchor != "论证图 · 治理决心主张" {
		t.Fatalf("ai msg = %+v", coach.Messages[1])
	}
}

func TestProjectCoach_AnchorFallback(t *testing.T) {
	coach := projectCoach(ProjectData{}, "论证构建") // no interventions
	if coach.Anchor != "论证构建" {
		t.Fatalf("fallback anchor = %q, want 论证构建", coach.Anchor)
	}
}

func TestProjectEquipment_SpontAndMeth(t *testing.T) {
	steel := uuid.New()
	d := ProjectData{
		Cards: []sqlc.CardInstance{
			{ID: steel, CardID: "steelman"},
			{ID: uuid.New(), CardID: "concession"},
		},
		Interventions: []sqlc.Intervention{
			{CardInstanceID: pgUUID(steel)}, // steelman was nudged
		},
	}
	spec := func(id string) (cards.Spec, bool) {
		return map[string]cards.Spec{"steelman": {ID: "steelman", Name: "钢人卡"}, "concession": {ID: "concession", Name: "让步段卡"}}[id], true
	}
	eq := projectEquipment(d, spec)
	if len(eq) != 2 {
		t.Fatalf("want 2 equip, got %d", len(eq))
	}
	if eq[0].Name != "钢人卡" || eq[0].Spont != "提示后" || eq[0].Meth != "concession" {
		t.Fatalf("steelman = %+v", eq[0])
	}
	if eq[1].Spont != "自发" {
		t.Fatalf("concession spont = %q, want 自发", eq[1].Spont)
	}
}

func TestProjectCoach_MergesStudentMessages(t *testing.T) {
	t1 := time.Now()
	d := ProjectData{
		Interventions: []sqlc.Intervention{
			{Body: "图上有一处孤儿证据", Anchor: []byte(`{"label":"孤儿证据"}`), Type: "flag", CreatedAt: t1},
		},
		ChatMessages: []sqlc.ChatMessage{
			{Role: "user", Content: "它想证明中国在认真转型", CreatedAt: t1.Add(time.Second)},
		},
	}
	coach := projectCoach(d, "论证构建")
	if len(coach.Messages) != 2 {
		t.Fatalf("want 2 messages, got %d", len(coach.Messages))
	}
	// time order: flag first, then the student bubble.
	if coach.Messages[0].Kind != "flag" || coach.Messages[1].Kind != "student" || coach.Messages[1].Body != "它想证明中国在认真转型" {
		t.Fatalf("merged = %+v", coach.Messages)
	}
}

// TestProjectDerivesMaterialState is the slice's central claim: a material's
// dossier state (locked/role/tier/takeaway/anchors) is DERIVED from what the
// student actually did, never decorated. matA has earned an evaluated-as
// edge (CRAAP mint) + a source-log entry + a targeting anchor; matB is
// untouched and must carry none of that state.
func TestProjectDerivesMaterialState(t *testing.T) {
	matA := uuid.MustParse("00000000-0000-0000-0000-0000000000aa") // evaluated
	matB := uuid.MustParse("00000000-0000-0000-0000-0000000000bb") // untouched
	evid := uuid.MustParse("00000000-0000-0000-0000-0000000000cc")

	sk := writingSkill(t)
	d := ProjectData{
		Plan: planNode(`["decode_task"]`),
		Materials: []sqlc.Material{
			{ID: matA, Title: "《卫星图看中国变绿》", Kind: "article", Source: "fetched",
				Blocks: []byte(`[{"id":"b1","text":"过去二十年……"}]`)},
			{ID: matB, Title: "IEA Renewable Investment", Kind: "article", Source: "pasted",
				Blocks: []byte(`[{"id":"b1","text":"China ranks first."}]`)},
		},
		SourceLog: []sqlc.SourceLogEntry{
			{MaterialID: pgUUID(matA), Tier: strPtr("二手 · 需追源"), Takeaway: "结论被放大了。", TimeSpentS: 240},
		},
		// The CRAAP mint: evidence node + evaluated-as edge from the material.
		Nodes: []sqlc.GraphNode{
			{ID: evid, Type: "evidence", Author: "student",
				Body: []byte(`{"source_quality":{"authority":"只是一个博主","risk_note":"入口来源，不能直接引用。"}}`)},
		},
		Edges: []sqlc.GraphEdge{
			{Type: "evaluated-as", FromKind: "material", FromID: matA, ToKind: "graph_node", ToID: evid},
		},
		// A persisted CRAAP card whose anchors target matA.
		Cards: []sqlc.CardInstance{
			{CardID: "craap", Status: "completed",
				Anchors: []byte(`[{"id":"a1","material_id":"` + matA.String() + `","block_id":"b1","start":0,"end":4,"quote":"过去二十年","dimension":"authority","author":"ai","question":"原始出处是谁？","answer":"只是一个博主"}]`)},
		},
	}

	proj, err := Project(sk, cards.ByID, d)
	if err != nil {
		t.Fatal(err)
	}
	if len(proj.Materials) != 2 {
		t.Fatalf("materials = %d, want 2", len(proj.Materials))
	}

	a := proj.Materials[0]
	if !a.Locked {
		t.Error("evaluated material: Locked = false, want true (an evaluated-as edge exists)")
	}
	if a.Role != "入口来源，不能直接引用。" {
		t.Errorf("Role = %q, want the minted evidence node's source_quality.risk_note", a.Role)
	}
	if a.Tier != "二手 · 需追源" || a.Takeaway != "结论被放大了。" {
		t.Errorf("Tier/Takeaway = %q/%q, want the source-log values", a.Tier, a.Takeaway)
	}
	if a.TimeSpentS != 240 {
		t.Errorf("TimeSpentS = %d, want 240 (the source-log entry's accumulated reading time)", a.TimeSpentS)
	}
	if len(a.Anchors) != 1 {
		t.Errorf("Anchors = %d, want 1 (the card anchor targeting this material)", len(a.Anchors))
	}

	b := proj.Materials[1]
	if b.Locked || b.Role != "" || b.Tier != "" || b.Takeaway != "" || len(b.Anchors) != 0 || b.TimeSpentS != 0 {
		t.Errorf("untouched material carries state it never earned: %+v", b)
	}
}
