// Package skills loads the embedded C5 skill catalog — a project/course type
// authored as a contract DAG (agent-spec §5.1). The specs are a generated
// mirror of packages/contracts/skills; never hand-edit specs/.
package skills

import (
	"embed"
	"encoding/json"
	"fmt"
	"sort"
)

//go:embed specs/*.json
var specFS embed.FS

// MachineKinds is the closed set of machine gate-item kinds (Slice 4). The
// names live here (config validation); the evaluation lives in the agent
// package. Adding a kind is a runtime change, not skill authoring.
var MachineKinds = map[string]bool{
	"node_present":            true,
	"node_count_at_least":     true,
	"no_orphan_evidence":      true,
	"no_unsupported_claim":    true,
	"no_single_sourced_claim": true,
	"every_source_evaluated":  true,
}

// MachineItem is one machine-checkable gate item — a closed-set predicate over
// the workspace graph. Type/N are read only by the kinds that use them
// (node_present / node_count_at_least).
type MachineItem struct {
	Kind string `json:"kind"`
	Type string `json:"type"`
	N    int    `json:"n"`
}

// Gate is a contract's three-tier gate (agent-spec §5.1). Machine items are
// computed; student_written/human items are recorded externally (DEC-3).
type Gate struct {
	Machine        []MachineItem `json:"machine"`
	StudentWritten []string      `json:"student_written"`
	Human          []string      `json:"human"`
}

// Contract is one milestone in the DAG.
type Contract struct {
	Requires   []string `json:"requires"`
	Produces   []string `json:"produces"`
	View       string   `json:"view"`
	Title      string   `json:"title"`
	Repertoire []string `json:"repertoire"`
	Gate       Gate     `json:"gate"`

	// Course-only fields (Slice 12). All optional — writing-project.json sets
	// none of them and must keep loading unchanged.
	Goal           string          `json:"goal,omitempty"`
	Steps          []int           `json:"steps,omitempty"`
	Page           *PhasePage      `json:"page,omitempty"`
	Cards          []string        `json:"cards,omitempty"`
	AnchorMaterial *AnchorMaterial `json:"anchor_material,omitempty"`
	AskChips       []string        `json:"ask_chips,omitempty"`
	Floor          []FloorItem     `json:"floor,omitempty"`
	SoftCondition  string          `json:"soft_condition,omitempty"`
}

// CourseFloorKinds is the closed set of course phase-floor kinds (Slice 12,
// DEC-12.2). The names live here (config validation); the evaluation lives in
// the agent package — the same split MachineKinds uses. The floor is the
// structural half of phase advance: it can only ever REFUSE. The positive
// pedagogical call is the coach's (DEC-3 discipline).
var CourseFloorKinds = map[string]bool{
	"steps_viewed":           true,
	"card_dispositioned":     true,
	"student_turns_at_least": true,
}

// FloorItem is one machine-checkable phase floor. Steps/CardID/N are read only
// by the kind that uses them, mirroring MachineItem.
type FloorItem struct {
	Kind   string `json:"kind"`
	Steps  []int  `json:"steps"`
	CardID string `json:"card_id"`
	N      int    `json:"n"`
}

// PhasePage is a step-less phase's authored page (the guided phase's page is
// the card; the reflect phase's is the dialogue). Phases that wrap course_step
// ordinals render from those rows instead and leave this nil.
type PhasePage struct {
	Title    string   `json:"title"`
	Subtitle string   `json:"subtitle"`
	Body     []string `json:"body"`
}

// AnchorMaterial is the case a phase's card practice hangs on — minted as a
// session material the first time the card surfaces.
type AnchorMaterial struct {
	Title string `json:"title"`
	Text  string `json:"text"`
}

// WordBudget is the per-qualification legal word band for S5 (draft_polish).
// An in-band commit mints the word_budget_ok node that satisfies the S5
// machine gate. Single source of truth; the commit path and the projection
// both read it.
type WordBudget struct {
	Min int `json:"min"`
	Max int `json:"max"`
}

// ReviewCriterion is one mark-scheme table the whole-draft review assesses the
// draft against (0457's 表D/E/F/H). Points is the table's total descriptor-point
// count — the number of lamps the Slice-9 readiness gauge renders for it.
type ReviewCriterion struct {
	Code   string `json:"code"`
	Name   string `json:"name"`
	Points int    `json:"points"`
}

// Skill is a project/course type: a contract DAG plus card references and
// pedagogy metadata. Intake is the declared procedure (run by the planner);
// Slice 4 keeps it as raw config it does not interpret.
type Skill struct {
	ID             string              `json:"id"`
	Kind           string              `json:"kind"`
	Contracts      map[string]Contract `json:"contracts"`
	Intake         json.RawMessage     `json:"intake"`
	Vocabulary     string              `json:"vocabulary"`
	Cards          []string            `json:"cards"`
	WordBudget     *WordBudget         `json:"word_budget,omitempty"`
	ReviewCriteria []ReviewCriterion   `json:"review_criteria,omitempty"`
	CourseID       string              `json:"course_id,omitempty"`
}

// Load parses and validates one skill JSON blob.
func Load(data []byte) (Skill, error) {
	var s Skill
	if err := json.Unmarshal(data, &s); err != nil {
		return Skill{}, fmt.Errorf("parse skill: %w", err)
	}
	if err := s.Validate(); err != nil {
		return Skill{}, err
	}
	return s, nil
}

// Validate rejects a malformed contract DAG at load: unknown kind/machine-kind,
// dangling requires, or a cycle (via TopoOrder).
func (s Skill) Validate() error {
	if s.ID == "" {
		return fmt.Errorf("skill: empty id")
	}
	if s.Kind != "project" && s.Kind != "course" {
		return fmt.Errorf("skill %s: bad kind %q", s.ID, s.Kind)
	}
	for id, c := range s.Contracts {
		for _, req := range c.Requires {
			if _, ok := s.Contracts[req]; !ok {
				return fmt.Errorf("skill %s: contract %s requires unknown %s", s.ID, id, req)
			}
		}
		for _, m := range c.Gate.Machine {
			if !MachineKinds[m.Kind] {
				return fmt.Errorf("skill %s: contract %s unknown machine kind %q", s.ID, id, m.Kind)
			}
		}
		for _, f := range c.Floor {
			if !CourseFloorKinds[f.Kind] {
				return fmt.Errorf("skill %s: contract %s unknown floor kind %q", s.ID, id, f.Kind)
			}
			if f.CardID != "" && !contains(s.Cards, f.CardID) {
				return fmt.Errorf("skill %s: contract %s floor references card %s not in the skill's cards", s.ID, id, f.CardID)
			}
		}
		for _, cd := range c.Cards {
			if !contains(s.Cards, cd) {
				return fmt.Errorf("skill %s: contract %s declares card %s not in the skill's cards", s.ID, id, cd)
			}
		}
	}
	for _, c := range s.ReviewCriteria {
		if c.Points < 1 {
			return fmt.Errorf("skill %s: review criterion %s needs points >= 1", s.ID, c.Code)
		}
	}
	// A course's phase order is BINDING (agent-spec §5.3): it is a chain, not a
	// general DAG. Enforce that at load — a branching course skill is a config
	// error, not a runtime surprise.
	if s.Kind == "course" {
		if _, err := s.LinearOrder(); err != nil {
			return err
		}
		return nil
	}
	if _, err := s.TopoOrder(); err != nil {
		return err
	}
	return nil
}

func contains(xs []string, x string) bool {
	for _, s := range xs {
		if s == x {
			return true
		}
	}
	return false
}

// LinearOrder returns a course skill's phases in their binding order. It errors
// unless the requires-chain is strictly linear: exactly one root, every other
// contract requiring exactly one predecessor, and no contract required by two
// successors. TopoOrder does the cycle check.
func (s Skill) LinearOrder() ([]string, error) {
	order, err := s.TopoOrder()
	if err != nil {
		return nil, err
	}
	successors := map[string]int{}
	roots := 0
	for id, c := range s.Contracts {
		switch len(c.Requires) {
		case 0:
			roots++
		case 1:
			successors[c.Requires[0]]++
		default:
			return nil, fmt.Errorf("skill %s: contract %s requires %d predecessors — a course order must be linear", s.ID, id, len(c.Requires))
		}
	}
	if roots != 1 {
		return nil, fmt.Errorf("skill %s: a course order must have exactly one first phase, found %d", s.ID, roots)
	}
	for id, n := range successors {
		if n > 1 {
			return nil, fmt.Errorf("skill %s: phase %s is followed by %d phases — a course order must be linear", s.ID, id, n)
		}
	}
	return order, nil
}

// TopoOrder returns the contract ids in a deterministic topological order
// (requires-before-dependent). Ties broken by id for stability. Errors on a
// cycle.
func (s Skill) TopoOrder() ([]string, error) {
	indeg := map[string]int{}
	for id := range s.Contracts {
		indeg[id] = 0
	}
	for id, c := range s.Contracts {
		for _, req := range c.Requires {
			if _, ok := s.Contracts[req]; ok {
				indeg[id]++
			}
		}
	}
	var order []string
	for len(order) < len(s.Contracts) {
		// pick the lowest-id node with indeg 0 not yet emitted → determinism
		var ready []string
		for id, d := range indeg {
			if d == 0 {
				ready = append(ready, id)
			}
		}
		if len(ready) == 0 {
			return nil, fmt.Errorf("skill %s: contract DAG has a cycle", s.ID)
		}
		sort.Strings(ready)
		pick := ready[0]
		order = append(order, pick)
		delete(indeg, pick)
		for id, c := range s.Contracts {
			if _, done := indeg[id]; !done {
				continue
			}
			for _, req := range c.Requires {
				if req == pick {
					indeg[id]--
				}
			}
		}
	}
	return order, nil
}

// Catalog reads and parses every embedded skill, sorted by id.
func Catalog() ([]Skill, error) {
	entries, err := specFS.ReadDir("specs")
	if err != nil {
		return nil, fmt.Errorf("read embedded skills: %w", err)
	}
	var out []Skill
	for _, e := range entries {
		if e.IsDir() || len(e.Name()) < 5 || e.Name()[len(e.Name())-5:] != ".json" {
			continue
		}
		data, err := specFS.ReadFile("specs/" + e.Name())
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", e.Name(), err)
		}
		s, err := Load(data)
		if err != nil {
			return nil, fmt.Errorf("load %s: %w", e.Name(), err)
		}
		out = append(out, s)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

// ByID returns the embedded skill with the given id, if present.
func ByID(id string) (Skill, bool) {
	all, err := Catalog()
	if err != nil {
		return Skill{}, false
	}
	for _, s := range all {
		if s.ID == id {
			return s, true
		}
	}
	return Skill{}, false
}
