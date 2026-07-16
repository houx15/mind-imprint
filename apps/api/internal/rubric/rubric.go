// Package rubric exposes the canonical CT critical-thinking rubric (the single
// source in packages/contracts/src/ct-rubric.json, mirrored here by
// `make sync-rubric` — never hand-edit ct-rubric.json).
package rubric

import (
	_ "embed"
	"encoding/json"
)

//go:embed ct-rubric.json
var ctJSON []byte

type Dimension struct {
	ID        string            `json:"id"`
	Name      string            `json:"name"`
	Framework string            `json:"framework"`
	Anchors   map[string]string `json:"anchors"` // keys L1..L4
}

type Rubric struct {
	ID         string      `json:"id"`
	Name       string      `json:"name"`
	Dimensions []Dimension `json:"dimensions"`
}

var ct = mustParse()

func mustParse() Rubric {
	var r Rubric
	if err := json.Unmarshal(ctJSON, &r); err != nil {
		panic("rubric: bad embedded ct-rubric.json: " + err.Error())
	}
	return r
}

// CT returns the parsed canonical CT rubric.
func CT() Rubric { return ct }
