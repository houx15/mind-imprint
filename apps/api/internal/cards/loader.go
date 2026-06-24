// Package cards loads the embedded card-spec catalog. The specs are a generated
// mirror of the canonical packages/contracts/cards; never hand-edit specs/.
package cards

import (
	"embed"
	"encoding/json"
	"fmt"
	"sort"
)

//go:embed specs/*.json
var specFS embed.FS

// Spec is the minimal view of a card the backend needs (catalog/prompt/refeed).
// The deep card shape stays owned by the TS Zod contract.
type Spec struct {
	ID               string `json:"id"`
	Category         string `json:"category"`
	Name             string `json:"name"`
	NameEN           string `json:"name_en"`
	Purpose          string `json:"purpose"`
	TriggerCondition string `json:"trigger_condition"`
}

// Catalog reads and parses every embedded spec, sorted by id for determinism.
func Catalog() ([]Spec, error) {
	entries, err := specFS.ReadDir("specs")
	if err != nil {
		return nil, fmt.Errorf("read embedded specs: %w", err)
	}
	specs := make([]Spec, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		data, err := specFS.ReadFile("specs/" + e.Name())
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", e.Name(), err)
		}
		var s Spec
		if err := json.Unmarshal(data, &s); err != nil {
			return nil, fmt.Errorf("parse %s: %w", e.Name(), err)
		}
		specs = append(specs, s)
	}
	sort.Slice(specs, func(i, j int) bool { return specs[i].ID < specs[j].ID })
	return specs, nil
}

// ByID returns the spec with the given id, if present.
func ByID(id string) (Spec, bool) {
	specs, err := Catalog()
	if err != nil {
		return Spec{}, false
	}
	for _, s := range specs {
		if s.ID == id {
			return s, true
		}
	}
	return Spec{}, false
}
