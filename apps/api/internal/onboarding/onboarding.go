// Package onboarding provides board-static S0 任务解码 content used to seed a
// new project's onboarding graph nodes at creation time. No model call — the
// content is authored per qualification and embedded. This is the single-source
// seam that multi-board (N4) extends by adding more fixture files + Load cases.
package onboarding

import (
	_ "embed"
	"encoding/json"
)

//go:embed fixtures/0457.json
var fixture0457 []byte

// Row is one rubric criterion translated to plain language. Weak is a suggested
// watch-flag from the fixture — distinct from the student's own picks.
type Row struct {
	Official string `json:"official"`
	Plain    string `json:"plain"`
	Weak     bool   `json:"weak"`
}

// Fixture is the S0 onboarding content for one qualification.
type Fixture struct {
	RestatePrompt string   `json:"restate_prompt"`
	Rows          []Row    `json:"rows"`
	Steps         []string `json:"steps"`
}

// Load returns the onboarding fixture for a qualification, or ok=false if none
// exists. Only 0457 is authored today (N4 adds boards).
func Load(qualification string) (Fixture, bool) {
	if qualification != "0457" {
		return Fixture{}, false
	}
	var f Fixture
	if err := json.Unmarshal(fixture0457, &f); err != nil {
		return Fixture{}, false
	}
	return f, true
}
