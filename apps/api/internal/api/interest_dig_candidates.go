package api

import (
	"sort"
	"strings"
	"unicode"

	"mindimprint/api/internal/interest"
	"mindimprint/api/internal/interests"
	"mindimprint/api/internal/library"
	"mindimprint/api/internal/store/sqlc"
)

const digLibraryKeywordCount = 3
const digLibraryCandidateCount = 3

// Dig recommendations describe the current keyword, rather than the student's
// entire tree. First select at most three canonical interests from its identity
// and real source statements. Then consider only unread articles connected to
// those interests. No zero-score shelf fallback is appropriate in this context.
func digLibraryCandidates(kw sqlc.InterestKeyword, evidence []string, profile library.Profile) []interest.LibraryCandidate {
	return selectDigLibraryCandidates(kw, evidence, profile, interests.All(), library.All())
}

func selectDigLibraryCandidates(kw sqlc.InterestKeyword, evidence []string, profile library.Profile, catalog []interests.Interest, articles []library.Article) []interest.LibraryCandidate {
	selected := digRelatedKeywords(kw, evidence, catalog)
	if len(selected) == 0 {
		return nil
	}
	// Keyword relevance is an ordering boundary, not a numeric bonus: a rare
	// contextual discipline must never outrank the current interest. Assign each
	// article to its earliest matching keyword, then rank only within that group.
	groups := make([][]library.Article, len(selected))
	profiles := make([]library.Profile, len(selected))
	for rank, it := range selected {
		profiles[rank] = library.Profile{Tier: profile.Tier, Disciplines: map[string]float64{}}
		for _, id := range it.Disciplines {
			profiles[rank].Disciplines[id] = 1
		}
	}
	for _, article := range articles {
		if profile.ReadSlugs[article.Slug] || strings.TrimSpace(article.ZhTitle+article.Title) == "" {
			continue
		}
	keyword:
		for rank := range selected {
			for _, id := range article.Disciplines {
				if profiles[rank].Disciplines[id] > 0 {
					groups[rank] = append(groups[rank], article)
					break keyword
				}
			}
		}
	}
	out := make([]interest.LibraryCandidate, 0, digLibraryCandidateCount)
	for rank, group := range groups {
		remaining := digLibraryCandidateCount - len(out)
		if remaining == 0 {
			break
		}
		for _, rec := range library.Recommend(group, profiles[rank], remaining) {
			title := strings.TrimSpace(rec.Article.ZhTitle)
			if title == "" {
				title = strings.TrimSpace(rec.Article.Title)
			}
			out = append(out, interest.LibraryCandidate{Slug: rec.Article.Slug, Title: title, Reason: rec.Article.Reason})
		}
	}
	return out
}

// Scores represent evidence sources, not mention frequency: a long conversation
// or repeated text must not outweigh the current keyword's canonical identity.
// Equal scores keep catalog order, making the result reproducible.
func digRelatedKeywords(kw sqlc.InterestKeyword, evidence []string, catalog []interests.Interest) []interests.Interest {
	type match struct {
		interest interests.Interest
		score    int
	}
	var matches []match
	names := []string{kw.TextZh, kw.TextEn, kw.Norm}
	for _, it := range catalog {
		score := 0
		if kw.InterestID != nil && *kw.InterestID == it.ID {
			score = 1000
		}
		for _, name := range names {
			if strings.TrimSpace(name) == "" {
				continue
			}
			if strings.EqualFold(strings.TrimSpace(name), it.Zh) || strings.EqualFold(strings.TrimSpace(name), it.En) || strings.EqualFold(strings.TrimSpace(name), it.ID) {
				if score < 100 {
					score = 100
				}
			} else if digMentionsInterest(name, it) && score < 50 {
				score = 50
			}
		}
		if score == 0 {
			for _, text := range evidence {
				if digMentionsInterest(text, it) {
					score = 10
					break
				}
			}
		}
		if score > 0 {
			matches = append(matches, match{it, score})
		}
	}
	sort.SliceStable(matches, func(i, j int) bool { return matches[i].score > matches[j].score })
	if len(matches) > digLibraryKeywordCount {
		matches = matches[:digLibraryKeywordCount]
	}
	out := make([]interests.Interest, 0, len(matches))
	for _, m := range matches {
		out = append(out, m.interest)
	}
	return out
}

func digMentionsInterest(text string, it interests.Interest) bool {
	if name := strings.TrimSpace(it.Zh); len([]rune(name)) >= 2 && strings.Contains(text, name) {
		return true
	}
	// Complete English words avoid matching e.g. "art" inside "earth". IDs may
	// contain hyphens; their readable form is also matched as a complete phrase.
	words := func(s string) string {
		return " " + strings.Join(strings.FieldsFunc(strings.ToLower(s), func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsNumber(r) }), " ") + " "
	}
	haystack := words(text)
	for _, name := range []string{it.En, it.ID} {
		if strings.TrimSpace(name) != "" && strings.Contains(haystack, words(name)) {
			return true
		}
	}
	return false
}
