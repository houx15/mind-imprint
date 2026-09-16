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
