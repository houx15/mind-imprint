// Package liteworkspace holds the teacher workspace's pure logic: the closed
// sets a tool call may name, and the checks that keep a reply grounded.
// It touches no database and no model, so every rule here is unit-testable.
package liteworkspace

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"mindimprint/api/internal/library"
)

// TurnsWindow bounds the transcript one turn carries. lite has no compaction
// layer, so this window is the only thing bounding prompt growth.
const TurnsWindow = 8

// ToolLoopMax bounds tool round-trips inside one turn. Hitting it fails the
// turn with a visible error rather than truncating silently.
const ToolLoopMax = 4

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

// SearchLibrary filters the embedded catalogue. query matches either title
// case-insensitively; tier 0 means "any tier".
func SearchLibrary(arts []library.Article, query string, disciplines []string, tier, limit int) []library.Article {
	q := strings.ToLower(strings.TrimSpace(query))
	want := make(map[string]bool, len(disciplines))
	for _, d := range disciplines {
		want[d] = true
	}
	out := make([]library.Article, 0, limit)
	for _, a := range arts {
		if q != "" && !strings.Contains(strings.ToLower(a.Title), q) && !strings.Contains(strings.ToLower(a.ZhTitle), q) {
			continue
		}
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
		out = append(out, a)
		if len(out) == limit {
			break
		}
	}
	return out
}
