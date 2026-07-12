package enforcement

import (
	"strings"
	"unicode"
)

// LexicalSimilarity is a keyless, offline Similarity implementation: it scores
// two texts by rune-bigram overlap (Jaccard index) — no embedding call, no
// API key, no network. Production callers that don't yet have an embedding
// provider wired (Slice 5c's studio turn endpoint) use this heuristic as
// deps.Sim; a real embedding-backed Similarity is a separate, later task once
// an embedding provider is chosen. Bigrams (rather than whitespace-split
// tokens) are used so the heuristic works uniformly across CJK text — which
// has no word boundaries — and space-delimited scripts alike.
type LexicalSimilarity struct{}

// Cosine returns a lexical-overlap score in [0, 1]: the Jaccard index of a's
// and b's rune-bigram sets, case-folded with whitespace stripped so spacing/
// casing differences don't move the score. Either input contributing no
// bigrams (empty or a single rune) scores 0 — no shared evidence, not
// "identical". The method name mirrors the enforcement.Similarity interface;
// this is a lexical-overlap score, not a true cosine similarity.
func (LexicalSimilarity) Cosine(a, b string) float64 {
	ga := bigramSet(a)
	gb := bigramSet(b)
	if len(ga) == 0 || len(gb) == 0 {
		return 0
	}
	shared := 0
	for g := range ga {
		if gb[g] {
			shared++
		}
	}
	union := len(ga) + len(gb) - shared
	if union == 0 {
		return 0
	}
	return float64(shared) / float64(union)
}

// bigramSet returns the set of overlapping rune-bigrams in s, lowercased with
// whitespace runes dropped first.
func bigramSet(s string) map[string]bool {
	runes := []rune(strings.ToLower(s))
	compact := make([]rune, 0, len(runes))
	for _, r := range runes {
		if unicode.IsSpace(r) {
			continue
		}
		compact = append(compact, r)
	}
	if len(compact) < 2 {
		return nil
	}
	out := make(map[string]bool, len(compact)-1)
	for i := 0; i < len(compact)-1; i++ {
		out[string(compact[i:i+2])] = true
	}
	return out
}
