// Package enforcement implements the server-side "AI never writes"
// enforcement primitives: pure, injectable functions with no DB, no
// network, and no live model access. The similarity provider used by
// OutputCheck is behind the Similarity interface so tests can stub it;
// the real embedding provider lives outside this package.
package enforcement

import (
	"fmt"
	"strings"
	"unicode"
)

// AgentOutput mirrors the Zod typed-output union (C3):
//
//	{ type: "question",   anchor, criterion, body }
//	{ type: "diagnostic", anchor, criterion, body }
//	{ type: "reference",  anchor, quote, provenance }
//	{ type: "proposal",   anchor, criterion, body }
//	{ type: "plan",       route }
//
// Go mirrors the union as a single struct with all fields optional;
// ValidateOutput enforces the per-type shape.
type AgentOutput struct {
	Type       string       `json:"type"`
	Anchor     OutputAnchor `json:"anchor,omitempty"`
	Criterion  string       `json:"criterion,omitempty"`
	Body       string       `json:"body,omitempty"`
	Quote      string       `json:"quote,omitempty"`
	Provenance string       `json:"provenance,omitempty"`
	Route      []string     `json:"route,omitempty"`
}

// OutputAnchor mirrors the Zod OutputAnchor: a typed reference to a node the
// output is anchored to. `plan` outputs carry no anchor; every other type does.
type OutputAnchor struct {
	Kind string         `json:"kind,omitempty"`
	ID   string         `json:"id,omitempty"`
	Span map[string]any `json:"span,omitempty"`
}

var validOutputTypes = map[string]bool{
	"question":   true,
	"diagnostic": true,
	"reference":  true,
	"proposal":   true,
	"plan":       true,
}

// ValidateOutput enforces the typed-output guard (design §9): reject
// unknown output types, and reject a "reference" output that has no
// provenance — a reference must resolve to the student's own prior
// artifact verbatim with provenance, or it fails.
func ValidateOutput(out AgentOutput) error {
	if !validOutputTypes[out.Type] {
		return fmt.Errorf("enforcement: unknown output type %q", out.Type)
	}
	switch out.Type {
	case "reference":
		if strings.TrimSpace(out.Anchor.ID) == "" {
			return fmt.Errorf("enforcement: reference output requires an anchor")
		}
		if strings.TrimSpace(out.Provenance) == "" {
			return fmt.Errorf("enforcement: reference output requires provenance")
		}
	case "question", "diagnostic", "proposal":
		if strings.TrimSpace(out.Anchor.ID) == "" {
			return fmt.Errorf("enforcement: %s output requires an anchor", out.Type)
		}
		if strings.TrimSpace(out.Criterion) == "" {
			return fmt.Errorf("enforcement: %s output requires a criterion", out.Type)
		}
		if strings.TrimSpace(out.Body) == "" {
			return fmt.Errorf("enforcement: %s output requires a body", out.Type)
		}
	case "plan":
		// route may be empty per the contract; no additional shape check.
	}
	return nil
}

// GuardStudentField is the authorship write-guard (design §9): rejects
// author != "student" at write time for student-written fields (warrant,
// steelman, risk note, ...).
func GuardStudentField(field, author string) error {
	if author != "student" {
		return fmt.Errorf("enforcement: field %q must be authored by student, got %q", field, author)
	}
	return nil
}

// Similarity is the injected embedding-similarity provider. Production
// code wires a real embedding-backed implementation server-side; tests
// use a constant stub. No real embedding call happens in this package.
type Similarity interface {
	Cosine(a, b string) float64
}

// Context carries the student's active topic/artifact for OutputCheck.
// Threshold overrides the interception cutoff; 0 uses DefaultEchoThreshold.
// The design (§11) keeps the threshold config, not code, pending calibration.
type Context struct {
	Topic              string
	CrossDomainExample bool
	Threshold          float64
}

// Verdict is the result of OutputCheck.
type Verdict struct {
	Verdict string
	Rewrite string
}

// DefaultEchoThreshold is the cosine-similarity cutoff above which a
// declarative echo of the student's topic is intercepted. Overridable per
// call via Context.Threshold (design §11: the threshold is config, not code).
const DefaultEchoThreshold = 0.8

// OutputCheck is the draw-out heuristic gate (design §9): a declarative
// sentence semantically close to the student's current topic is
// intercepted and rewritten as a question. Cross-domain examples (they
// can't be pasted from the student's own artifact) are exempt.
func OutputCheck(text string, ctx Context, sim Similarity) Verdict {
	if ctx.CrossDomainExample {
		return Verdict{Verdict: "pass"}
	}
	threshold := ctx.Threshold
	if threshold == 0 {
		threshold = DefaultEchoThreshold
	}
	if isDeclarative(text) && sim.Cosine(text, ctx.Topic) > threshold {
		return Verdict{Verdict: "intercept", Rewrite: asQuestion(text)}
	}
	return Verdict{Verdict: "pass"}
}

// isDeclarative treats a sentence ending in a declarative terminator, with no
// question mark, as declarative. Both ASCII (. !) and full-width Chinese (。！)
// terminators and question marks (? ？) are recognized — the coach speaks
// Chinese, so ASCII-only matching would silently miss every real echo. This is
// a fast heuristic gate, not a full parser.
func isDeclarative(text string) bool {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return false
	}
	if strings.ContainsAny(trimmed, "?？") {
		return false
	}
	return strings.HasSuffix(trimmed, ".") || strings.HasSuffix(trimmed, "!") ||
		strings.HasSuffix(trimmed, "。") || strings.HasSuffix(trimmed, "！")
}

// asQuestion rewrites a declarative sentence as a question so the student, not
// the AI, supplies the claim. It strips a trailing declarative terminator
// (ASCII or full-width) and appends a full-width "？" for Chinese text, an
// ASCII "?" otherwise.
func asQuestion(text string) string {
	trimmed := strings.TrimRight(strings.TrimSpace(text), ".!。！")
	if containsHan(trimmed) {
		return trimmed + "？"
	}
	return trimmed + "?"
}

// containsHan reports whether s contains any Han (CJK) rune.
func containsHan(s string) bool {
	for _, r := range s {
		if unicode.Is(unicode.Han, r) {
			return true
		}
	}
	return false
}
