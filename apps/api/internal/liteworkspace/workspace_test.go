package liteworkspace

import (
	"testing"
	"time"

	"mindimprint/api/internal/library"
)

func TestFilterStudentsClosedSet(t *testing.T) {
	rows := []Student{
		{ID: "a", Name: "林知遥", ActiveDaysThisWeek: 0, OverdueAssignments: 1, WritingsDone: 0},
		{ID: "b", Name: "陈屿", ActiveDaysThisWeek: 3, OverdueAssignments: 0, WritingsDone: 2},
		{ID: "c", Name: "周未", ActiveDaysThisWeek: 0, OverdueAssignments: 0, WritingsDone: 1},
	}
	for _, tc := range []struct {
		filter StudentFilter
		want   []string
	}{
		{FilterAll, []string{"a", "b", "c"}},
		{FilterInactiveThisWeek, []string{"a", "c"}},
		{FilterHasOverdue, []string{"a"}},
		{FilterNoWritingYet, []string{"a"}},
	} {
		got := FilterStudents(rows, tc.filter)
		if len(got) != len(tc.want) {
			t.Fatalf("%s: got %d rows, want %d", tc.filter, len(got), len(tc.want))
		}
		for i, id := range tc.want {
			if got[i].ID != id {
				t.Fatalf("%s: row %d is %q, want %q", tc.filter, i, got[i].ID, id)
			}
		}
	}
}

func TestFilterStudentsRejectsUnknownFilter(t *testing.T) {
	if _, ok := ParseStudentFilter("below_tier"); ok {
		t.Fatal("below_tier has no query behind it and must not parse")
	}
	if f, ok := ParseStudentFilter("has_overdue"); !ok || f != FilterHasOverdue {
		t.Fatalf("has_overdue did not parse, got %q ok=%v", f, ok)
	}
}

// 姓名校验：花名册里的名字出现在回复里，就必须是这一轮工具给过的，
// 或者老师自己打过的。模型自己上一轮说过的话不算数据来源。
func TestUngroundedNames(t *testing.T) {
	roster := []string{"林知遥", "陈屿", "周未"}
	got := UngroundedNames("林知遥和陈屿这周都没有写作。", roster, []string{"林知遥"})
	if len(got) != 1 || got[0] != "陈屿" {
		t.Fatalf("got %v, want [陈屿]", got)
	}
	if n := UngroundedNames("这三位学生本周还没有写作。", roster, nil); len(n) != 0 {
		t.Fatalf("a reply that names nobody is grounded, got %v", n)
	}
}

// TestUngroundedNamesOverFlagsOverlappingNames pins the direction the check
// errs when one classmate's name is a prefix of another's. The comparison is
// exact-match on the grounded side and substring on the roster side, so an
// overlap can only ADD a flag, never remove one: a fabricated name can never
// be laundered by a grounded classmate whose name contains it.
//
// The cost is a false positive — naming 林知遥, who was returned, also flags
// 林知, who was not. That fails the turn with a visible error, which is the
// side to be wrong on: the other direction renders a fabricated name to a
// teacher as fact.
func TestUngroundedNamesOverFlagsOverlappingNames(t *testing.T) {
	roster := []string{"林知", "林知遥"}

	got := UngroundedNames("林知遥这周还没有写作。", roster, []string{"林知遥"})
	if len(got) != 1 || got[0] != "林知" {
		t.Fatalf("got %v, want [林知] — the shorter name is flagged, not silently allowed", got)
	}

	// The load-bearing direction: 林知遥 was never returned and never typed,
	// and the grounded 林知 must not cover for it.
	got = UngroundedNames("林知遥这周还没有写作。", roster, []string{"林知"})
	for _, n := range got {
		if n == "林知遥" {
			return
		}
	}
	t.Fatalf("got %v, want 林知遥 flagged — a fabricated name must not pass because a classmate's name is a prefix of it", got)
}

func TestBeijingWallToUTC(t *testing.T) {
	got, err := BeijingWallToUTC("2026-09-20T18:00")
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	want := time.Date(2026, 9, 20, 10, 0, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Fatalf("got %s, want %s", got, want)
	}
	if _, err := BeijingWallToUTC("周五下午"); err == nil {
		t.Fatal("a relative phrase must fail, not silently become now")
	}
}

func TestClampChoices(t *testing.T) {
	in := []Choice{{ID: "1", Label: "论证结构"}, {ID: "2", Label: ""}, {ID: "3", Label: "证据使用"},
		{ID: "4", Label: "语言表达"}, {ID: "5", Label: "篇幅"}, {ID: "6", Label: "体裁"}}
	got := ClampChoices(in)
	if len(got) != MaxChoices {
		t.Fatalf("got %d choices, want %d", len(got), MaxChoices)
	}
	for _, c := range got {
		if c.Label == "" {
			t.Fatal("an empty label renders as a blank button; it must be dropped")
		}
	}
}

func TestTrimTurnsKeepsTheMostRecent(t *testing.T) {
	var in []Turn
	for i := 0; i < 20; i++ {
		in = append(in, Turn{Role: "teacher", Text: string(rune('a' + i))})
	}
	got := TrimTurns(in)
	if len(got) != TurnsWindow {
		t.Fatalf("got %d turns, want %d", len(got), TurnsWindow)
	}
	if got[len(got)-1].Text != in[len(in)-1].Text {
		t.Fatal("trimming must keep the LAST turns, not the first")
	}
}

// TestSearchLibrary covers the branches a tier filter can get silently wrong:
// a tier that never matches must drop the article, not fall through as "any
// tier". Fixtures are built inline so the test does not depend on the real
// catalogue's content or size.
func TestSearchLibrary(t *testing.T) {
	arts := []library.Article{
		{
			Slug:        "ocean",
			Title:       "Ocean Currents Shift",
			ZhTitle:     "洋流与气候",
			Disciplines: []string{"biology", "geography"},
			Levels:      []library.Level{{Tier: 1}, {Tier: 2}},
		},
		{
			Slug:        "volcano",
			Title:       "Volcanoes Wake Up",
			ZhTitle:     "火山苏醒",
			Disciplines: []string{"geology"},
			Levels:      []library.Level{{Tier: 1}},
		},
		{
			Slug:        "coral",
			Title:       "Coral Reefs Bleach",
			ZhTitle:     "珊瑚礁白化",
			Disciplines: []string{"biology"},
			Levels:      []library.Level{{Tier: 3}},
		},
	}

	t.Run("query matches Title case-insensitively", func(t *testing.T) {
		got := SearchLibrary(arts, "OCEAN", nil, 0, 10)
		if len(got) != 1 || got[0].Slug != "ocean" {
			t.Fatalf("got %v, want [ocean]", slugsOf(got))
		}
	})

	t.Run("query matches ZhTitle", func(t *testing.T) {
		got := SearchLibrary(arts, "火山", nil, 0, 10)
		if len(got) != 1 || got[0].Slug != "volcano" {
			t.Fatalf("got %v, want [volcano]", slugsOf(got))
		}
	})

	t.Run("discipline filter keeps an article when any one discipline matches", func(t *testing.T) {
		got := SearchLibrary(arts, "", []string{"geography"}, 0, 10)
		if len(got) != 1 || got[0].Slug != "ocean" {
			t.Fatalf("got %v, want [ocean]; geography is one of ocean's two disciplines and must be enough", slugsOf(got))
		}
	})

	t.Run("tier filter drops an article that has no level at that tier", func(t *testing.T) {
		got := SearchLibrary(arts, "", nil, 3, 10)
		if len(got) != 1 || got[0].Slug != "coral" {
			t.Fatalf("got %v, want [coral]; a tier filter that matches nothing must return an empty shelf, not everything", slugsOf(got))
		}
	})

	t.Run("limit caps the result", func(t *testing.T) {
		got := SearchLibrary(arts, "", nil, 0, 2)
		if len(got) != 2 {
			t.Fatalf("got %d articles, want 2", len(got))
		}
	})
}

func slugsOf(arts []library.Article) []string {
	out := make([]string, len(arts))
	for i, a := range arts {
		out[i] = a.Slug
	}
	return out
}

// searchShelf reproduces three real catalogue rows, fields and all, because
// what the search has to get right depends on what is written in them: the
// climate article's headline says 气候队 and never 气候变化, its reason says
// 气候目标, and 气候变化 reaches it only through the climate-ocean alias table.
// A made-up fixture would let every branch below pass on a shelf that does not
// exist.
func searchShelf() []library.Article {
	return []library.Article{
		{
			Slug:        "biden-creates-climate-corps",
			Title:       "Biden uses executive power to create a New Deal-style American Climate Corps",
			ZhTitle:     "美国气候队",
			Reason:      "一项就业计划如何同时服务气候目标，以及它绕开国会的方式。",
			Disciplines: []string{"political-economy", "climate-ocean", "career-futures"},
			Levels:      []library.Level{{Tier: 1}, {Tier: 3}},
		},
		{
			Slug:        "aid-groups-israel-hamas-war",
			Title:       "Aid groups scramble to help as Israel-Hamas war intensifies",
			ZhTitle:     "战争中的援助机构",
			Reason:      "物资、通道与安全限制，人道组织在战区实际能做的事。",
			Disciplines: []string{"public-health", "political-economy", "ethics"},
			Levels:      []library.Level{{Tier: 2}},
		},
		{
			Slug:        "ai-can-be-a-helpful-math-tutor-if-used-right-and-harmful-if-not",
			Title:       "AI can be a helpful math tutor if used right — and harmful if not",
			ZhTitle:     "AI 当数学家教的正确用法",
			Reason:      "1300 名高中生的调查结果：哪几种用法帮得上忙，哪几种反而拖后腿。",
			Disciplines: []string{"education-learning", "ai-ml", "cognitive-psychology"},
			Levels:      []library.Level{{Tier: 2}},
		},
	}
}

// TestSearchLibraryFindsTheTopicNotJustTheHeadline — the two cases the live run
// produced, which the old title-substring search got wrong in both directions.
//
// 「气候变化」 is what a teacher types and what the model relays. The catalogue's
// only climate article is headlined 美国气候队, so a title substring misses it
// while the single character shorter 「气候」 hits — a cliff no one can see from
// the outside. 「AI」 went the other way: a bare substring matched aid, painful
// and Hawaii.
func TestSearchLibraryFindsTheTopicNotJustTheHeadline(t *testing.T) {
	arts := searchShelf()

	t.Run("a full-phrase topic reaches the article through its discipline", func(t *testing.T) {
		got := SearchLibrary(arts, "气候变化", nil, 0, 10)
		if len(got) != 1 || got[0].Slug != "biden-creates-climate-corps" {
			t.Fatalf("got %v, want [biden-creates-climate-corps]", slugsOf(got))
		}
	})

	t.Run("a short ASCII query needs a word boundary", func(t *testing.T) {
		got := SearchLibrary(arts, "AI", nil, 0, 10)
		if len(got) != 1 || got[0].Slug != "ai-can-be-a-helpful-math-tutor-if-used-right-and-harmful-if-not" {
			t.Fatalf("got %v — AI must not match Aid or painful", slugsOf(got))
		}
	})

	t.Run("a CJK phrase nobody wrote falls back to its 2-character windows", func(t *testing.T) {
		// 全球变暖 is a climate-ocean alias, so it matches whole. 气候目标 is
		// not an alias and appears in no title; only the 气候 window reaches it.
		for _, q := range []string{"全球变暖", "气候目标"} {
			got := SearchLibrary(arts, q, nil, 0, 10)
			if len(got) == 0 || got[0].Slug != "biden-creates-climate-corps" {
				t.Fatalf("query %q got %v, want the climate article first", q, slugsOf(got))
			}
		}
	})

	t.Run("a multi-word ASCII phrase falls back to its words", func(t *testing.T) {
		// No haystack carries "climate change" whole — the discipline's own
		// name is 「Climate & ocean」 — so the phrase the model reaches for when
		// its Chinese query missed would miss too.
		got := SearchLibrary(arts, "climate change", nil, 0, 10)
		if len(got) == 0 || got[0].Slug != "biden-creates-climate-corps" {
			t.Fatalf("got %v, want the climate article first", slugsOf(got))
		}
	})

	t.Run("the fallback ranks by how much of the phrase matched", func(t *testing.T) {
		// 援助气候 matches the aid article on 援助 and the climate one on 气候,
		// one window each, so catalogue order breaks the tie — and the same
		// query must return the same order twice.
		first := slugsOf(SearchLibrary(arts, "援助气候", nil, 0, 10))
		second := slugsOf(SearchLibrary(arts, "援助气候", nil, 0, 10))
		if len(first) != 2 {
			t.Fatalf("got %v, want both articles", first)
		}
		if first[0] != second[0] || first[1] != second[1] {
			t.Fatalf("same query, different order: %v then %v", first, second)
		}
	})

	t.Run("a topic nothing carries still returns nothing", func(t *testing.T) {
		if got := SearchLibrary(arts, "量子隧穿", nil, 0, 10); len(got) != 0 {
			t.Fatalf("got %v, want an empty shelf — a widened search that always answers is worse than one that says no", slugsOf(got))
		}
	})

	t.Run("the discipline filter still narrows rather than widens", func(t *testing.T) {
		// AND, not OR. The argument reads 「限定学科」, and OR-ing would make
		// disciplines:[climate-ocean] + query:AI answer with climate articles
		// the teacher did not ask for.
		got := SearchLibrary(arts, "AI", []string{"climate-ocean"}, 0, 10)
		if len(got) != 0 {
			t.Fatalf("got %v, want none: the AI article is not tagged climate-ocean", slugsOf(got))
		}
	})
}

// TestSearchLibraryAnswersTheRealShelf pins the two acceptance cases against
// the catalogue that actually ships, not a fixture. The fixture above pins the
// algorithm; this pins that the shelf in this repo answers the query a teacher
// wrote in the live run.
func TestSearchLibraryAnswersTheRealShelf(t *testing.T) {
	arts := library.All()

	got := SearchLibrary(arts, "气候变化", nil, 0, 8)
	if len(got) == 0 {
		t.Fatal("气候变化 finds nothing on the real shelf")
	}
	found := false
	for _, a := range got {
		if a.Slug == "biden-creates-climate-corps" {
			found = true
		}
	}
	if !found {
		t.Fatalf("气候变化 returned %v without the catalogue's climate article", slugsOf(got))
	}

	for _, a := range SearchLibrary(arts, "AI", nil, 0, 8) {
		switch a.Slug {
		case "aid-groups-israel-hamas-war", "scientist-studies-painful-stingers-20231008":
			t.Fatalf("AI still matches %q as a bare substring", a.Slug)
		}
	}
}
