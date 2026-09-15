package api_test

// library_start_test.go — the student's own POST /api/v1/library/{slug}/levels/{tier}.
// Starting an assigned library reading reuses the same helper, so the resume
// and error rules it depends on are locked here.

import (
	"net/http"
	"strconv"
	"testing"

	"mindimprint/api/internal/library"
)

func TestLibraryStartRoute(t *testing.T) {
	h, pool, _, _, studentID := liteTeacherFixture(t)
	student := signInAs(t, pool, studentID)
	art := library.All()[0]
	tier := strconv.Itoa(art.Levels[0].Tier)
	path := "/api/v1/library/" + art.Slug + "/levels/" + tier

	type startOut struct {
		ID      string `json:"id"`
		Resumed bool   `json:"resumed"`
	}
	var first, repeat, fresh startOut
	steps := []struct {
		name   string
		path   string
		before func()
		out    *startOut
		want   int
		check  func() bool
	}{
		{name: "first start", path: path, out: &first, want: http.StatusCreated,
			check: func() bool { return first.ID != "" && !first.Resumed }},
		{name: "repeat unfinished start", path: path, out: &repeat, want: http.StatusOK,
			check: func() bool { return repeat.ID == first.ID && repeat.Resumed }},
		{name: "start after finishing", path: path, out: &fresh, want: http.StatusCreated,
			before: func() {
				if code := assignJSON(t, h, student, "POST", "/api/v1/readings/"+first.ID+"/finish", nil, nil); code != http.StatusOK {
					t.Fatalf("finish reading = %d", code)
				}
			},
			check: func() bool { return fresh.ID != "" && fresh.ID != first.ID && !fresh.Resumed }},
		{name: "unknown slug", path: "/api/v1/library/no-such-article/levels/" + tier, want: http.StatusNotFound},
		{name: "non-numeric tier", path: "/api/v1/library/" + art.Slug + "/levels/two", want: http.StatusBadRequest},
		{name: "tier the article lacks", path: "/api/v1/library/" + art.Slug + "/levels/99", want: http.StatusBadRequest},
		{name: "unknown slug with non-numeric tier", path: "/api/v1/library/no-such-article/levels/two", want: http.StatusNotFound},
	}
	for _, s := range steps {
		if s.before != nil {
			s.before()
		}
		var out any
		if s.out != nil {
			out = s.out
		}
		if code := assignJSON(t, h, student, "POST", s.path, nil, out); code != s.want {
			t.Fatalf("%s = %d, want %d", s.name, code, s.want)
		}
		if s.check != nil && !s.check() {
			t.Fatalf("%s: first=%+v repeat=%+v fresh=%+v", s.name, first, repeat, fresh)
		}
	}
}
