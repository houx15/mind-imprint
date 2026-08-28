// Package vocab is the single source of truth for the method names 印记 is
// allowed to use, their definitions, and the examples it may show.
//
// Two properties this package exists to guarantee:
//
//   - TERMINOLOGY IS OURS. A teacher is rigorous about terms; a model improvising
//     「递进式论证法」 on one turn and 「层进」 on the next teaches nothing. 印记
//     selects from this file and tunes; it never invents a term. An id the model
//     returns that is not in here is dropped by the caller.
//   - EXAMPLES ARE BORROWED MATERIAL. Every example here is pre-authored about a
//     topic no student is writing on. An "example 留悬念 opening" generated about
//     her own thesis IS her opening, authored by AI — 铁律① through the back door.
//     Shipping the examples as data closes that door structurally.
//
// The same JSON is imported at build time by apps/lite-web, so the two ends can
// never disagree about what a method is called.
//
// methods.json is duplicated on purpose. The editable source of truth lives at
// packages/contracts/vocab/methods.json; go:embed patterns cannot escape this
// package's own directory, so apps/api/internal/vocab/methods.json is a
// byte-identical copy embedded locally. TestEmbeddedCopyMatchesSourceOfTruth in
// vocab_test.go enforces the two never drift apart. Do not "simplify" this by
// deleting one of the two files — that reintroduces the escape-the-module
// problem this duplication exists to solve.
package vocab

import (
	_ "embed"
	"encoding/json"
	"fmt"
)

//go:embed methods.json
var methodsJSON []byte

type Example struct {
	Topic string `json:"topic"`
	Text  string `json:"text"`
}

type Pattern struct {
	Label string `json:"label"`
	Frame string `json:"frame"`
}

type Method struct {
	ID         string    `json:"id"`
	Name       string    `json:"name"`
	AppliesTo  string    `json:"applies_to"`
	Definition string    `json:"definition"`
	Examples   []Example `json:"examples"`
	Patterns   []Pattern `json:"patterns"`
}

var loaded []Method
var byID map[string]Method

func init() {
	var doc struct {
		Methods []Method `json:"methods"`
	}
	if err := json.Unmarshal(methodsJSON, &doc); err != nil {
		panic(fmt.Sprintf("vocab: methods.json is not valid: %v", err))
	}
	loaded = doc.Methods
	byID = make(map[string]Method, len(loaded))
	for _, m := range loaded {
		byID[m.ID] = m
	}
}

func All() []Method { return loaded }

func ByID(id string) (Method, bool) {
	m, ok := byID[id]
	return m, ok
}

// For returns the methods usable at a position in the piece. "any" is always
// included, because a method that fits everywhere fits here too.
func For(appliesTo string) []Method {
	out := make([]Method, 0, len(loaded))
	for _, m := range loaded {
		if m.AppliesTo == appliesTo || m.AppliesTo == "any" {
			out = append(out, m)
		}
	}
	return out
}
