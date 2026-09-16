// Package liteworkspace holds the teacher workspace's pure logic: the closed
// sets a tool call may name, and the checks that keep a reply grounded.
// It touches no database and no model, so every rule here is unit-testable.
package liteworkspace

import (
	"fmt"
	"sort"
	"strconv"
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

// Choice is one option button. Slug is the article that option means, when it
// means one.
//
// It exists because the model kept putting a slug in the ID: offered a list of
// articles it minted ids like biden-climate-corps, which look like slugs and
// are never equal to one, and then on the next turn searched for its own id as
// a query and told the teacher 「库里没搜到这篇」. The id field was the only place
// it had to put the article, so it used it. This is that place.
//
// Slug is validated against the catalogue when ask_choice writes it, so a
// wrong one is a tool error on the same turn, while the model still has budget
// to fix it — not a dead end two turns later.
type Choice struct {
	ID    string `json:"id"`
	Label string `json:"label"`
	Slug  string `json:"slug,omitempty"`
	// Article is filled in by the api layer, from the catalogue, after Slug
	// is already known-good (askChoice rejects an unknown one before this
	// struct ever carries it). It is not model output, so it does not go
	// through the §6 grounding checks the way Label does, and it does not go
	// through slug replacement — Slug here is meant to stay a slug.
	Article *ChoiceArticle `json:"article,omitempty"`
}

// ChoiceArticle is the article one option means, shaped for a card in the
// conversation rather than a shelf: the same fields the shelf shows for a
// title and a reason to open it, plus the cover it already signs.
type ChoiceArticle struct {
	Slug     string `json:"slug"`
	ZhTitle  string `json:"zhTitle"`
	CoverURL string `json:"coverUrl,omitempty"`
	Reason   string `json:"reason"`
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
// This function checks names only. Counts are checked separately, by
// UngroundedCounts, and on a much narrower shape: a number immediately followed
// by a counter word for people. That is not the general digit check this
// comment used to rule out — an article title, a year, a tier and a word count
// all carry digits and none of them reaches that shape.
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

// personCounterWidth reports how many runes of a counter word start at r[at],
// when the word there makes the number before it a count of PEOPLE. 0 means it
// is not such a word.
//
// 人 / 位 / 名 after a number count people in this reply or count nothing.
//
// 个 on its own does not. 「两个选项」「3 个字段」「一个办法」 are not head counts,
// and a check that fired on them would fail turns for saying nothing wrong. It
// counts only when the thing counted is spelled out right behind it —
// 个学生 / 个同学 / 个孩子. Those three are exactly the forms the live detector
// in lite_teacher_workspace_live_test.go reads, and leaving them out of the
// guard let a live run pass while production shipped 「3 个学生」.
//
// 🚨 个人 is NOT one of them, though it names people and 「12 个人」 is a real
// head count. 人 is a morpheme before it is a word: 人物, 人称, 人工智能, 人选
// all start with it, so 个人 fires on 「文中有 3 个人物」 and 「两个人称视角对比」
// — ordinary sentences about an article, and the kind of sentence that lands in
// a 说明 the model writes into the card. A word boundary does not rescue it
// either; that variant was measured and still loses cases. Missing 「12 个人」 is
// much cheaper than failing a turn over 人物.
//
// 名 and 位 are rejected when the next rune turns them into a different word:
// 名单 and 位置. Whitespace is stripped before this runs, so
// 「截止 2026-09-18 名单如下」 puts 18 straight against 名 — and the system prompt
// itself tells the model to write 「名单上的学生」, so this false positive would
// fail turns for following instructions.
func personCounterWidth(r []rune, at int) int {
	if at >= len(r) {
		return 0
	}
	next := rune(0)
	if at+1 < len(r) {
		next = r[at+1]
	}
	switch r[at] {
	case '人':
		return 1
	case '名':
		if next == '单' {
			return 0
		}
		return 1
	case '位':
		if next == '置' {
			return 0
		}
		return 1
	case '个':
		for _, w := range personNouns {
			if hasPrefixRunes(r[at+1:], w) {
				return 1 + len(w)
			}
		}
	}
	return 0
}

// personNouns are the nouns that turn 个 into a head-count counter. 人 is
// deliberately absent — see personCounterWidth.
var personNouns = [][]rune{[]rune("学生"), []rune("同学"), []rune("孩子")}

func hasPrefixRunes(r, prefix []rune) bool {
	if len(r) < len(prefix) {
		return false
	}
	for i, c := range prefix {
		if r[i] != c {
			return false
		}
	}
	return true
}

// StatedCounts returns every head count a text states: a number, written in
// digits or Chinese numerals, immediately followed by a counter word that
// counts people — 人, 位, 名, or 个 with the noun spelled out behind it
// (个学生 / 个同学 / 个孩子 / 个人).
//
// 🚨 Narrow on purpose. This is NOT a digit detector. A year, a tier, a word
// count, a date and an ordinal all carry digits and none of them is a claim
// about how many students there are; a checker that fired on those would be
// the third recorded case in this repo of a detector costing more than it
// caught. Only the number-plus-counter-word shape.
//
// Whitespace is removed first. 「发给全班 3 人」 and 「发给全班3人」 are the same
// sentence, and the first version of this check compared contiguous strings and
// therefore saw only the second — a real reply walked straight through it.
//
// 第 before the number excludes it: 「第一位」 and 「第 3 名」 are ordinals.
func StatedCounts(text string) []int {
	r := []rune(stripSpace(text))
	var out []int
	for i := 0; i < len(r); {
		n, width, ok := readNumber(r[i:])
		if !ok {
			i++
			continue
		}
		end := i + width
		if w := personCounterWidth(r, end); w > 0 && (i == 0 || r[i-1] != '第') {
			out = append(out, n)
			i = end + w
			continue
		}
		i = end
	}
	return out
}

// UngroundedCounts returns the head counts a text states that this turn's tools
// never returned and the teacher never wrote.
//
// It mirrors UngroundedNames, and for the same reason: a number about her own
// class is governed by the prompt alone, and a prompt is not a guarantee. The
// model's own earlier turns are not evidence here either — a made-up count's
// source is the model's own words.
func UngroundedCounts(text string, grounded []int) []int {
	ok := make(map[int]bool, len(grounded))
	for _, g := range grounded {
		ok[g] = true
	}
	seen := make(map[int]bool)
	var bad []int
	for _, n := range StatedCounts(text) {
		if ok[n] || seen[n] {
			continue
		}
		seen[n] = true
		bad = append(bad, n)
	}
	sort.Ints(bad)
	return bad
}

// readNumber reads one number off the front of r, in digits or in Chinese
// numerals, and returns how many runes it consumed.
func readNumber(r []rune) (value, width int, ok bool) {
	if n, w, isDigits := readDigits(r); isDigits {
		return n, w, true
	}
	return readCJKNumber(r)
}

func readDigits(r []rune) (value, width int, ok bool) {
	i := 0
	for i < len(r) && r[i] >= '0' && r[i] <= '9' {
		i++
	}
	if i == 0 {
		return 0, 0, false
	}
	n, err := strconv.Atoi(string(r[:i]))
	if err != nil {
		// A run longer than an int; it is not a class size either way.
		return 0, i, false
	}
	return n, i, true
}

var cjkDigits = map[rune]int{
	'〇': 0, '零': 0, '一': 1, '二': 2, '两': 2, '三': 3, '四': 4,
	'五': 5, '六': 6, '七': 7, '八': 8, '九': 9,
}

// readCJKNumber reads 三 / 十 / 十三 / 三十 / 三十五 — everything a class size
// can be. Larger constructions (百, 千) are not read: a head count past 99 is
// not a head count.
func readCJKNumber(r []rune) (value, width int, ok bool) {
	i := 0
	ones := -1
	if i < len(r) {
		if v, isDigit := cjkDigits[r[i]]; isDigit {
			ones = v
			i++
		}
	}
	if i < len(r) && r[i] == '十' {
		i++
		tens := 1
		if ones >= 0 {
			tens = ones
		}
		value = tens * 10
		if i < len(r) {
			if v, isDigit := cjkDigits[r[i]]; isDigit {
				value += v
				i++
			}
		}
		return value, i, true
	}
	if ones >= 0 {
		return ones, 1, true
	}
	return 0, 0, false
}

func stripSpace(s string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsSpace(r) {
			return -1
		}
		return r
	}, s)
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

// HistoryTextCapRunes bounds a HISTORY turn's text — every turn but the
// last, which is what she just sent and must reach the model whole (a
// pasted article set_material grounds against it, substring for substring).
// Must equal threadLogic.ts's HISTORY_TEXT_CAP: the client already
// truncates before sending, but this runs again server-side because the
// server never trusts that it did.
const HistoryTextCapRunes = 1000

// TruncateHistory caps every turn but the last to HistoryTextCapRunes
// runes, ending a cut string with 「…」. The last turn — the one just sent
// — is returned unchanged. Call this AFTER TrimTurns: the window bounds how
// many turns travel, this bounds how long each older one is.
func TruncateHistory(turns []Turn) []Turn {
	if len(turns) == 0 {
		return turns
	}
	out := make([]Turn, len(turns))
	copy(out, turns)
	for i := 0; i < len(out)-1; i++ {
		out[i].Text = truncateHistoryText(out[i].Text)
	}
	return out
}

func truncateHistoryText(s string) string {
	r := []rune(s)
	if len(r) <= HistoryTextCapRunes {
		return s
	}
	return string(r[:HistoryTextCapRunes]) + "…"
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
