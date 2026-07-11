package enforcement

import "regexp"

// Rule is one entry in the banned-phrasing corpus: a human-readable
// Name and a compiled Pattern to match against agent output text.
type Rule struct {
	Name    string
	Pattern *regexp.Regexp
}

// bannedPhrasingVersion tracks the corpus revision. Bump it whenever
// Rules changes so downstream logs/evaluations can record which
// version of the corpus flagged a turn.
const bannedPhrasingVersion = 1

// Rules is the versioned corpus of forbidden AI moves (design §9): the
// AI must never hand the student an unanchored question, a suggested
// counterclaim, or a candidate example — those are the student's job.
// This is a standing regression suite; extend it as new draw-out
// failures are found, never silently narrow it.
var Rules = []Rule{
	{
		Name:    "unanchored-question",
		Pattern: regexp.MustCompile(`(?i)have you considered other angles`),
	},
	{
		Name:    "unanchored-question-generic",
		Pattern: regexp.MustCompile(`(?i)have you (?:thought about|considered)\b`),
	},
	{
		Name:    "suggested-counterclaim",
		Pattern: regexp.MustCompile(`(?i)(?:you could|maybe you should|why not) argue (?:that|instead)`),
	},
	{
		Name:    "suggested-counterclaim-explicit",
		Pattern: regexp.MustCompile(`(?i)a good counterclaim (?:would be|is)`),
	},
	{
		Name:    "candidate-example",
		Pattern: regexp.MustCompile(`(?i)for example, you could (?:say|use|cite)`),
	},
	{
		Name:    "candidate-example-here",
		Pattern: regexp.MustCompile(`(?i)here('|')?s an example you can use`),
	},
	{
		Name:    "unanchored-question-zh",
		Pattern: regexp.MustCompile(`你有没有考虑过`),
	},
}

// BannedPhrasing checks text against the versioned Rules corpus
// (case-insensitive substring/regexp match) and returns the first
// matching Rule, or nil if the text is clean.
func BannedPhrasing(text string) *Rule {
	for i := range Rules {
		if Rules[i].Pattern.MatchString(text) {
			return &Rules[i]
		}
	}
	return nil
}
