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
