package skills

import "testing"

func linearSkill() Skill {
	return Skill{
		ID:   "t",
		Kind: "project",
		Contracts: map[string]Contract{
			"a": {Gate: Gate{Machine: []MachineItem{{Kind: "node_present", Type: "x"}}}},
			"b": {Requires: []string{"a"}},
			"c": {Requires: []string{"b"}},
		},
	}
}

func TestValidate_AcceptsAcyclicResolvedDAG(t *testing.T) {
	if err := linearSkill().Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}
}

func TestValidate_RejectsDanglingRequires(t *testing.T) {
	s := linearSkill()
	s.Contracts["b"] = Contract{Requires: []string{"nope"}}
	if err := s.Validate(); err == nil {
		t.Fatal("want error for dangling requires")
	}
}

func TestValidate_RejectsCycle(t *testing.T) {
	s := Skill{ID: "t", Kind: "project", Contracts: map[string]Contract{
		"a": {Requires: []string{"b"}},
		"b": {Requires: []string{"a"}},
	}}
	if err := s.Validate(); err == nil {
		t.Fatal("want error for cyclic requires")
	}
}

func TestValidate_RejectsUnknownMachineKind(t *testing.T) {
	s := linearSkill()
	s.Contracts["a"] = Contract{Gate: Gate{Machine: []MachineItem{{Kind: "bogus"}}}}
	if err := s.Validate(); err == nil {
		t.Fatal("want error for unknown machine kind")
	}
}

func TestTopoOrder_IsDeterministicAndRespectsRequires(t *testing.T) {
	order, err := linearSkill().TopoOrder()
	if err != nil {
		t.Fatalf("TopoOrder: %v", err)
	}
	pos := map[string]int{}
	for i, id := range order {
		pos[id] = i
	}
	if !(pos["a"] < pos["b"] && pos["b"] < pos["c"]) {
		t.Fatalf("order violates requires: %v", order)
	}
	// determinism: same input, same output
	order2, _ := linearSkill().TopoOrder()
	for i := range order {
		if order[i] != order2[i] {
			t.Fatalf("non-deterministic order: %v vs %v", order, order2)
		}
	}
}

func TestWritingProject_LoadsValidatesAndReferencesRealCards(t *testing.T) {
	s, ok := ByID("writing-project")
	if !ok {
		t.Fatal("writing-project skill not embedded")
	}
	if s.Kind != "project" {
		t.Fatalf("kind = %q", s.Kind)
	}
	for _, want := range []string{"decode_task", "frame_question", "evaluate_perspectives",
		"evaluate_sources", "build_argument", "draft_polish", "reflect_archive"} {
		if _, ok := s.Contracts[want]; !ok {
			t.Fatalf("missing contract %s", want)
		}
	}
	// requires chain holds (build_argument after evaluate_sources)
	order, err := s.TopoOrder()
	if err != nil {
		t.Fatalf("TopoOrder: %v", err)
	}
	pos := map[string]int{}
	for i, id := range order {
		pos[id] = i
	}
	// the full S0–S6 chain is linear
	chain := []string{"decode_task", "frame_question", "evaluate_perspectives",
		"evaluate_sources", "build_argument", "draft_polish", "reflect_archive"}
	for i := 1; i < len(chain); i++ {
		if pos[chain[i-1]] >= pos[chain[i]] {
			t.Fatalf("%s must come before %s", chain[i-1], chain[i])
		}
	}
}

func TestWritingProjectContractTitles(t *testing.T) {
	sk, ok := ByID("writing-project")
	if !ok {
		t.Fatal("writing-project skill not found")
	}
	want := map[string]string{
		"decode_task":           "任务解码",
		"frame_question":        "立题",
		"evaluate_perspectives": "视角与素材",
		"evaluate_sources":      "信源评估",
		"build_argument":        "论证构建",
		"draft_polish":          "成稿打磨",
		"reflect_archive":       "反思归档",
	}
	for id, title := range want {
		if got := sk.Contracts[id].Title; got != title {
			t.Errorf("contract %s: Title = %q, want %q", id, got, title)
		}
	}
}

func TestWritingProjectWordBudgetAndReviewCriteria(t *testing.T) {
	sk, ok := ByID("writing-project")
	if !ok {
		t.Fatal("writing-project skill not loaded")
	}
	if sk.WordBudget == nil {
		t.Fatal("WordBudget is nil")
	}
	if sk.WordBudget.Min != 1500 || sk.WordBudget.Max != 2000 {
		t.Fatalf("WordBudget = %+v, want {1500 2000}", *sk.WordBudget)
	}
	codes := map[string]bool{}
	for _, c := range sk.ReviewCriteria {
		if c.Code == "" || c.Name == "" {
			t.Fatalf("review criterion has empty field: %+v", c)
		}
		codes[c.Code] = true
	}
	for _, want := range []string{"表D", "表E", "表F", "表H"} {
		if !codes[want] {
			t.Errorf("review_criteria missing %q", want)
		}
	}
}

func TestReviewCriterionPointsParsed(t *testing.T) {
	s, err := Load([]byte(`{"id":"x","kind":"project","contracts":{},
		"review_criteria":[{"code":"表D","name":"来源与证据","points":4}]}`))
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(s.ReviewCriteria) != 1 || s.ReviewCriteria[0].Points != 4 {
		t.Fatalf("want points 4, got %+v", s.ReviewCriteria)
	}
}

func TestReviewCriterionPointsMustBePositive(t *testing.T) {
	_, err := Load([]byte(`{"id":"x","kind":"project","contracts":{},
		"review_criteria":[{"code":"表D","name":"来源与证据","points":0}]}`))
	if err == nil {
		t.Fatal("want error for points < 1, got nil")
	}
}

func TestSeededWritingSkillHasCriterionPoints(t *testing.T) {
	sk, ok := ByID("writing-project")
	if !ok {
		t.Fatal("writing-project skill missing")
	}
	want := map[string]int{"表D": 4, "表E": 4, "表F": 3, "表H": 3}
	if len(sk.ReviewCriteria) != len(want) {
		t.Fatalf("want %d criteria, got %d", len(want), len(sk.ReviewCriteria))
	}
	for _, c := range sk.ReviewCriteria {
		if want[c.Code] != c.Points {
			t.Fatalf("%s: want points %d, got %d", c.Code, want[c.Code], c.Points)
		}
	}
}

func TestLoadCourseSkill(t *testing.T) {
	s, ok := ByID("info-literacy-course")
	if !ok {
		t.Fatal("course skill must load")
	}
	if s.Kind != "course" {
		t.Fatalf("kind = %q, want course", s.Kind)
	}
	order, err := s.LinearOrder()
	if err != nil {
		t.Fatalf("course chain must be linear: %v", err)
	}
	want := []string{"demonstrate", "guided", "independent", "reflect"}
	if len(order) != len(want) {
		t.Fatalf("order = %v, want %v", order, want)
	}
	for i := range want {
		if order[i] != want[i] {
			t.Fatalf("order = %v, want %v", order, want)
		}
	}
	g := s.Contracts["guided"]
	if len(g.Floor) != 1 || g.Floor[0].Kind != "card_dispositioned" || g.Floor[0].CardID != "craap" {
		t.Fatalf("guided floor = %+v, want one card_dispositioned craap", g.Floor)
	}
	if g.AnchorMaterial == nil || g.AnchorMaterial.Text == "" {
		t.Fatal("guided must declare an anchor material")
	}
	if s.Contracts["demonstrate"].SoftCondition == "" {
		t.Fatal("every phase needs a soft condition")
	}
}

func TestCourseValidationRejectsBadConfig(t *testing.T) {
	cases := map[string]string{
		"unknown floor kind": `{"id":"x","kind":"course","cards":["craap"],"contracts":{
			"a":{"floor":[{"kind":"vibes_ok"}],"gate":{}}}}`,
		"floor card not in skill cards": `{"id":"x","kind":"course","cards":[],"contracts":{
			"a":{"floor":[{"kind":"card_dispositioned","card_id":"craap"}],"gate":{}}}}`,
		"branching chain": `{"id":"x","kind":"course","contracts":{
			"a":{"gate":{}},
			"b":{"requires":["a"],"gate":{}},
			"c":{"requires":["a"],"gate":{}}}}`,
		"two roots": `{"id":"x","kind":"course","contracts":{
			"a":{"gate":{}},
			"b":{"gate":{}}}}`,
	}
	for name, blob := range cases {
		if _, err := Load([]byte(blob)); err == nil {
			t.Fatalf("%s: must be rejected at load", name)
		}
	}
}

func TestProjectSkillStillLoads(t *testing.T) {
	s, ok := ByID("writing-project")
	if !ok {
		t.Fatal("writing-project must keep loading unchanged")
	}
	if _, err := s.TopoOrder(); err != nil {
		t.Fatalf("writing-project DAG must stay valid: %v", err)
	}
}
