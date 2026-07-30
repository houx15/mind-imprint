package api

import (
	"context"
	"regexp"
	"strings"

	"github.com/google/uuid"
)

// coach_link.go — the link-in-coach → resource bridge. A URL dropped into a
// coach turn is otherwise inert chat text: it is stored verbatim in
// chat_message but never becomes a reference and never points the student at
// the Reading Room. We detect the FIRST new URL server-side (free — a string
// scan + one library read, no model spend) and attach a linkOffer to the coach
// reply. The student's tap on the chip is what promotes the link to a reference
// or opens the reading room — 铁律: triggering is automatic, opening is
// confirmed. No prompt change: detection is post-hoc, the coach model is
// untouched.

// urlPattern matches an http(s) URL. Liberal on the body; the trailing-cut
// below strips sentence punctuation a student appends around a pasted link
// ("看看 https://x.org/a。" or "(https://x.org/a)").
var urlPattern = regexp.MustCompile(`https?://[^\s<>"'（）【】]+`)

// urlSentenceTrailing is punctuation (ASCII + full-width) that commonly trails a
// pasted URL inside a sentence but is never part of the address. ASCII brackets
// )]} are deliberately NOT here — they are handled by trimBalancedTrailing so a
// URL whose path legitimately ends in a paren (Wikipedia disambiguation links)
// keeps it, while a URL wrapped in prose parens loses only the wrapper.
const urlSentenceTrailing = ".,;:!?。，、；：！？”’）】》\"'"

// closerToOpener maps an ASCII closing bracket to its opener, for the balanced
// trim below.
var closerToOpener = map[byte]byte{')': '(', ']': '[', '}': '{'}

// trimURLTrailing removes trailing sentence punctuation, then removes an
// UNBALANCED trailing ASCII bracket (a wrapper paren from prose) while KEEPING a
// balanced one (part of the path). Iterates because a link can end "…)。".
func trimURLTrailing(s string) string {
	for {
		next := strings.TrimRight(s, urlSentenceTrailing)
		if len(next) == 0 {
			return next
		}
		last := next[len(next)-1]
		open, isCloser := closerToOpener[last]
		if !isCloser {
			return next
		}
		// A balanced closer (opener present at least as often) belongs to the URL.
		if strings.Count(next, string(open)) >= strings.Count(next, string(last)) {
			return next
		}
		next = next[:len(next)-1] // drop the unbalanced wrapper closer, then re-loop
		if next == s {
			return next
		}
		s = next
	}
}

// extractFirstURL returns the first http(s) URL in text with trailing sentence
// punctuation / wrapper brackets trimmed, or "" if none. Pure — unit-testable.
func extractFirstURL(text string) string {
	m := urlPattern.FindString(text)
	if m == "" {
		return ""
	}
	return trimURLTrailing(m)
}

// normalizeURL trims whitespace + trailing punctuation so a stored reference URL
// and a freshly parsed one compare equal even if one was captured mid-sentence.
func normalizeURL(u string) string {
	return trimURLTrailing(strings.TrimSpace(u))
}

// linkOfferDTO is the wire shape attached to a coach reply when the student's
// turn contains a new URL not already in the project's library.
type linkOfferDTO struct {
	URL string `json:"url"`
}

// detectLinkOffer returns a linkOffer for the first URL in userInput that is
// NOT already a reference in the project's library, or nil. Dedup is by
// normalized-URL match against ListReferences — an already-added link never
// re-offers. Any query error degrades to nil: a missing offer never fails the
// turn.
func (a *API) detectLinkOffer(ctx context.Context, projectID uuid.UUID, userInput string) *linkOfferDTO {
	url := extractFirstURL(userInput)
	if url == "" {
		return nil
	}
	refs, err := a.d.Queries.ListReferences(ctx, projectID)
	if err != nil {
		return nil
	}
	for _, r := range refs {
		if normalizeURL(r.Url) == url {
			return nil
		}
	}
	return &linkOfferDTO{URL: url}
}
