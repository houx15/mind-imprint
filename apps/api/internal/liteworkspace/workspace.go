// Package liteworkspace holds the teacher workspace's pure logic: the closed
// sets a tool call may name, and the checks that keep a reply grounded.
// It touches no database and no model, so every rule here is unit-testable.
package liteworkspace

import (
	"fmt"
	"sort"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"mindimprint/api/internal/disciplines"
	"mindimprint/api/internal/library"
)

// TurnsWindow bounds the transcript one turn carries. lite has no compaction
// layer, so this window is the only thing bounding prompt growth.
const TurnsWindow = 8

// ToolLoopMax bounds one turn's MODEL CALLS, not its tool round-trips. The
// last call has to be the one that answers, so the tool budget is ToolLoopMax
// minus one. Read the budget that way before lowering this number.
//
// 6 covers the realistic opening turn — list the students, search the library,
// set the material, set the fields, then ask a choice — with a call to spare
// for a tool the model has to retry after a bad argument. Hitting the cap
// fails the whole turn in the teacher's face, so the budget is set where a
// normal turn does not reach it.
const ToolLoopMax = 6

// MaxChoices bounds the option buttons a reply may carry.
const MaxChoices = 4

// BeijingOffset is a fixed offset, not a named zone: the distroless runtime
// image ships no tzdata and LoadLocation there fails back to UTC in silence.
var BeijingOffset = time.FixedZone("UTC+8", 8*60*60)

type Turn struct {
	Role string `json:"role"` // "teacher" | "ai"
	Text string `json:"text"`
}

type Choice struct {
	ID    string `json:"id"`
	Label string `json:"label"`
}

// Student is the workspace's view of one roster row — only the fields a
// closed-set filter reads. The api layer maps its roster DTO into this.
type Student struct {
	ID                 string `json:"id"`
	Name               string `json:"name"`
	ActiveDaysThisWeek int    `json:"activeDaysThisWeek"`
	OverdueAssignments int    `json:"overdueAssignments"`
	WritingsDone       int    `json:"writingsDone"`
}

// StudentFilter is the closed set list_students accepts. Every member must be
// answerable from a roster row above. Do NOT add a filter whose data we do not
// already have — adding the query comes first.
type StudentFilter string

const (
	FilterAll              StudentFilter = "all"
	FilterInactiveThisWeek StudentFilter = "inactive_this_week"
	FilterHasOverdue       StudentFilter = "has_overdue"
	FilterNoWritingYet     StudentFilter = "no_writing_yet"
)

func ParseStudentFilter(s string) (StudentFilter, bool) {
	switch StudentFilter(s) {
	case FilterAll, FilterInactiveThisWeek, FilterHasOverdue, FilterNoWritingYet:
		return StudentFilter(s), true
	}
	return "", false
}

// FilterStudents keeps the roster's order so the card reads the same way twice.
func FilterStudents(rows []Student, f StudentFilter) []Student {
	out := make([]Student, 0, len(rows))
	for _, r := range rows {
		keep := false
		switch f {
		case FilterAll:
			keep = true
		case FilterInactiveThisWeek:
			keep = r.ActiveDaysThisWeek == 0
		case FilterHasOverdue:
			keep = r.OverdueAssignments > 0
		case FilterNoWritingYet:
			keep = r.WritingsDone == 0
		}
		if keep {
			out = append(out, r)
		}
	}
	return out
}

// UngroundedNames returns the roster names a reply states that this turn's
// tools never returned and the teacher never typed.
//
// The check runs over the roster — a closed, exactly-comparable set — instead
// of trying to spot "a name" in free text. `grounded` is what the tools
// returned plus what the teacher herself wrote; it deliberately excludes the
// model's own earlier turns, since those are where a fabrication comes from.
//
// Only names are checked, never digits: an article title or a year would make
// a digit check fire on correct output.
func UngroundedNames(reply string, roster, grounded []string) []string {
	ok := make(map[string]bool, len(grounded))
	for _, g := range grounded {
		ok[g] = true
	}
	var bad []string
	for _, name := range roster {
		if name == "" || ok[name] {
			continue
		}
		if strings.Contains(reply, name) {
			bad = append(bad, name)
		}
	}
	sort.Strings(bad)
	return bad
}

// BeijingWallToUTC reads a wall-clock time the model wrote as Beijing time
// ("2026-09-20T18:00") and returns the instant. A relative phrase is an error:
// the model is told today's Beijing date and resolves 周五 itself, so anything
// that is not an absolute time is a bug we surface rather than guess at.
func BeijingWallToUTC(wall string) (time.Time, error) {
	t, err := time.ParseInLocation("2006-01-02T15:04", strings.TrimSpace(wall), BeijingOffset)
	if err != nil {
		return time.Time{}, fmt.Errorf("截止时间不是绝对时刻：%q", wall)
	}
	return t.UTC(), nil
}

// ClampChoices drops blank labels and keeps at most MaxChoices.
func ClampChoices(in []Choice) []Choice {
	out := make([]Choice, 0, MaxChoices)
	for _, c := range in {
		if strings.TrimSpace(c.Label) == "" {
			continue
		}
		out = append(out, c)
		if len(out) == MaxChoices {
			break
		}
	}
	return out
}

// TrimTurns keeps the most recent TurnsWindow turns.
func TrimTurns(in []Turn) []Turn {
	if len(in) <= TurnsWindow {
		return in
	}
	return in[len(in)-TurnsWindow:]
}

// SearchLibrary filters the embedded catalogue.
//
// The query is matched against what the article is ABOUT, not only what its
// headline happens to say: title, Chinese title, the one-line reason, and every
// discipline it carries, expanded through the discipline table into its Chinese
// name, English name and aliases. That expansion is the fix for the failure the
// 2026-09-16 live run found — the catalogue's only climate article is headlined
// 美国气候队, so a title substring answered 「气候」 and answered 「气候变化」 with
// nothing, a cliff between two queries a teacher cannot tell apart. 气候变化 is
// an alias of climate-ocean, so it now reaches the article the curated way.
//
// Matching depends on the script, because the two fail differently:
//
//   - A CJK query matches as a plain substring.
//   - An ASCII query must sit on a word boundary. A bare substring made 「AI」
//     return aid-groups, painful-stingers and Hawaii — nine articles, mostly
//     noise. Only ASCII letters and digits count as boundary characters, so
//     「AI」 still matches 用AI写作.
//
// Either way, a phrase that matches NOTHING anywhere falls back to its smaller
// pieces — 2-character windows for CJK, single words for a multi-word ASCII
// query — ranked by how many pieces each article carried. The fallback runs
// only on a total miss, so a phrase that works keeps its precision.
//
// The discipline argument stays ANDed with the query. It reads 限定学科 and it
// is a narrowing constraint: OR-ing it would make disciplines:[climate-ocean]
// plus query:亚运会 answer with climate articles nobody asked for. What it could
// not do before — rescue a query that found nothing — is now done by the query
// itself reaching the discipline table.
//
// tier 0 means "any tier". The same query returns the same articles in the same
// order every time: the exact path keeps catalogue order, and the fallback sorts
// by window count with catalogue order breaking every tie.
func SearchLibrary(arts []library.Article, query string, disciplines []string, tier, limit int) []library.Article {
	q := strings.ToLower(strings.TrimSpace(query))
	want := make(map[string]bool, len(disciplines))
	for _, d := range disciplines {
		want[d] = true
	}

	shelf := make([]library.Article, 0, len(arts))
	for _, a := range arts {
		if len(want) > 0 {
			hit := false
			for _, d := range a.Disciplines {
				if want[d] {
					hit = true
					break
				}
			}
			if !hit {
				continue
			}
		}
		if tier != 0 {
			if _, ok := a.LevelAt(tier); !ok {
				continue
			}
		}
		shelf = append(shelf, a)
	}
	if q == "" {
		return capAt(shelf, limit)
	}

	hay := make([]string, len(shelf))
	for i, a := range shelf {
		hay[i] = searchText(a)
	}

	out := make([]library.Article, 0, limit)
	for i, a := range shelf {
		if phraseHit(hay[i], q) {
			out = append(out, a)
		}
	}
	if len(out) > 0 {
		return capAt(out, limit)
	}
	return capAt(termFallback(shelf, hay, q), limit)
}

func capAt(arts []library.Article, limit int) []library.Article {
	if limit > 0 && len(arts) > limit {
		return arts[:limit]
	}
	return arts
}

// searchText is everything one article can be found by, lower-cased once.
//
// A discipline contributes its id, its two names and its aliases. The alias
// table is the line this product already maintains between the words a person
// uses and the subjects it knows about; reusing it here means the search and
// the interest tree agree on what 气候变化 means instead of each having a
// private opinion.
func searchText(a library.Article) string {
	var b strings.Builder
	b.WriteString(a.Title)
	b.WriteString("\n")
	b.WriteString(a.ZhTitle)
	b.WriteString("\n")
	b.WriteString(a.Reason)
	for _, id := range a.Disciplines {
		b.WriteString("\n")
		b.WriteString(id)
		d, ok := disciplines.ByID(id)
		if !ok {
			continue
		}
		b.WriteString("\n")
		b.WriteString(d.Zh)
		b.WriteString("\n")
		b.WriteString(d.En)
		for _, alias := range d.Aliases {
			b.WriteString("\n")
			b.WriteString(alias)
		}
	}
	return strings.ToLower(b.String())
}

// phraseHit reports whether hay carries the whole query. hay and q are already
// lower-cased.
func phraseHit(hay, q string) bool {
	if hasHan(q) {
		return strings.Contains(hay, q)
	}
	return boundedHit(hay, q)
}

// boundedHit is strings.Contains with a word boundary on both sides, so a
// two-letter query stops matching the middle of a longer word.
//
// Only an ASCII letter or digit counts as a boundary character. A Han character
// is a letter to unicode.IsLetter, and counting it would make 「AI」 miss
// 用AI写作 — which is how these titles are written.
func boundedHit(hay, q string) bool {
	for at := 0; at < len(hay); {
		i := strings.Index(hay[at:], q)
		if i < 0 {
			return false
		}
		start := at + i
		end := start + len(q)
		before, _ := utf8.DecodeLastRuneInString(hay[:start])
		after, _ := utf8.DecodeRuneInString(hay[end:])
		if !asciiWordRune(before) && !asciiWordRune(after) {
			return true
		}
		at = start + 1
	}
	return false
}

func asciiWordRune(r rune) bool {
	if r == utf8.RuneError {
		return false
	}
	return r < 128 && (unicode.IsLetter(r) || unicode.IsDigit(r))
}

func hasHan(s string) bool {
	for _, r := range s {
		if unicode.Is(unicode.Han, r) {
			return true
		}
	}
	return false
}

// termFallback answers a phrase nobody wrote down, by matching its smaller
// pieces and ranking on how many of them an article carried. An article that
// matched two pieces is more likely to be about the thing than one that matched
// a single incidental pair.
//
// It runs only after the whole phrase has missed everywhere, so a phrase that
// works keeps its precision.
func termFallback(shelf []library.Article, hay []string, q string) []library.Article {
	terms := fallbackTerms(q)
	if len(terms) == 0 {
		return nil
	}
	type scored struct {
		index, score int
	}
	var hits []scored
	for i := range shelf {
		score := 0
		for _, term := range terms {
			if phraseHit(hay[i], term) {
				score++
			}
		}
		if score > 0 {
			hits = append(hits, scored{i, score})
		}
	}
	sort.SliceStable(hits, func(a, b int) bool { return hits[a].score > hits[b].score })
	out := make([]library.Article, 0, len(hits))
	for _, h := range hits {
		out = append(out, shelf[h.index])
	}
	return out
}

// fallbackTerms breaks a query into the smaller pieces worth trying once the
// whole phrase has missed: overlapping 2-character windows for CJK
// (气候目标 → 气候 / 候目 / 目标), individual words for a multi-word ASCII query
// ("climate change" → climate / change).
//
// Nil when there is nothing smaller to try — a two-character CJK query IS its
// own window, and a one-word ASCII query is its own word, so retrying either
// would just repeat the search that already missed.
func fallbackTerms(q string) []string {
	if !hasHan(q) {
		words := strings.Fields(q)
		if len(words) < 2 {
			return nil
		}
		return dedupe(words)
	}
	var runes []rune
	for _, r := range q {
		if !unicode.IsSpace(r) {
			runes = append(runes, r)
		}
	}
	if len(runes) < 3 {
		return nil
	}
	out := make([]string, 0, len(runes)-1)
	for i := 0; i+1 < len(runes); i++ {
		out = append(out, string(runes[i:i+2]))
	}
	return dedupe(out)
}

func dedupe(in []string) []string {
	seen := make(map[string]bool, len(in))
	out := make([]string, 0, len(in))
	for _, s := range in {
		if seen[s] {
			continue
		}
		seen[s] = true
		out = append(out, s)
	}
	return out
}
