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
	Repertoire []string `json:"repertoire"`
	Gate       Gate     `json:"gate"`
}

// Skill is a project/course type: a contract DAG plus card references and
// pedagogy metadata. Intake is the declared procedure (run by the planner);
// Slice 4 keeps it as raw config it does not interpret.
type Skill struct {
	ID         string              `json:"id"`
	Kind       string              `json:"kind"`
	Contracts  map[string]Contract `json:"contracts"`
	Intake     json.RawMessage     `json:"intake"`
	Vocabulary string              `json:"vocabulary"`
	Cards      []string            `json:"cards"`
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
	}
	if _, err := s.TopoOrder(); err != nil {
		return err
	}
	return nil
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
