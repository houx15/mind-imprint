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

// Method is one entry in the library. It carries TWO names on purpose
// (2026-08-28 product ruling): even 论证 is jargon to a 中学生, but the
// curriculum term must stay reachable rather than be hidden.
//
//   - Name is plain language — what the student reads. It names the ACTION she
//     already performs（「举个例子」「比一比」）.
//   - FormalName is the 语文 curriculum term（「举例论证」「对比论证」）, revealed
//     by a card she can click. Offered, never imposed. It is empty where no
//     curriculum term exists (closing_scope) and on the English entries, whose
//     Name is already what an English writer reads.
type Method struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	FormalName string `json:"formal_name"`
	AppliesTo  string `json:"applies_to"`
	// Lang is the writing language this method may be offered for: "zh", "en"
	// or "any". It is a STRUCTURAL guard, not a hint to the model — handing an
	// English sentence frame ("While it is true that ___") to a student writing
	// a Chinese essay is a bug, and a prompt line asking the model to please
	// avoid that is not a fix. See For.
	//
	// 🔑 What is language-bound is the WORDING, not the rhetoric. The en_*
	// entries carry literal English sentence frames, so they are "en": useless
	// inside a Chinese essay. Every structural method — hooks, parallel
	// reasons, concession, cause-and-effect — is "any", because an English
	// essay has all of those too, and 印记 discusses an English piece IN
	// CHINESE anyway (buildWritingPlanPrompt: 「这篇用英文写（但你和她用中文
	// 讨论）」), so a Chinese method name in front of an English writer is
	// exactly what the rest of the room already does. Tagging the structural
	// entries "zh" would fix the reported bug and create its mirror image: an
	// English writer left with one opening method and no closings at all.
	Lang       string    `json:"lang"`
	Definition string    `json:"definition"`
	Examples   []Example `json:"examples"`
	Patterns   []Pattern `json:"patterns"`
}

// Label is how a method is named INSIDE A PROMPT: the plain name, plus the
// curriculum term in parentheses when there is a distinct one. 印记 has to be
// able to say the plain name to the student and still know what the method is
// really called when she asks.
func (m Method) Label() string {
	if m.FormalName == "" || m.FormalName == m.Name {
		return m.Name
	}
	return m.Name + "（正式名称：" + m.FormalName + "）"
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

// For returns the methods usable at a position in the piece, IN THE LANGUAGE
// the piece is written in. "any" is always included on both axes, because a
// method that fits everywhere fits here too.
//
// The lang axis is not a refinement, it is a bug fix (2026-08-28 ruling):
// filtering on position alone handed en_concession's "While it is true that
// ___" to a student writing a Chinese essay. Language belongs in the selector,
// not in a prompt sentence asking the model to be careful.
//
// It filters EXPRESSIONS, not rhetoric — see Method.Lang. In practice: a
// Chinese piece never sees an English sentence frame, and an English piece
// keeps the whole method vocabulary plus the frames.
func For(appliesTo string, lang string) []Method {
	out := make([]Method, 0, len(loaded))
	for _, m := range loaded {
		if (m.AppliesTo == appliesTo || m.AppliesTo == "any") && speaks(m, lang) {
			out = append(out, m)
		}
	}
	return out
}

// ForLang returns every method usable in a language, at ANY position. It
// serves the two prompts that must cover the whole piece at once (the planning
// turn, and the batch guide that guides every block in one call): they annotate
// each entry with its applies_to instead of filtering by it, but they must
// still never offer the wrong language.
func ForLang(lang string) []Method {
	out := make([]Method, 0, len(loaded))
	for _, m := range loaded {
		if speaks(m, lang) {
			out = append(out, m)
		}
	}
	return out
}

func speaks(m Method, lang string) bool { return m.Lang == lang || m.Lang == "any" }
