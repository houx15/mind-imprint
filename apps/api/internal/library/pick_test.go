package library

import (
	"testing"

	"mindimprint/api/internal/disciplines"
)

// Synthetic library, in library order. Ids are real discipline ids so Reason
// can look up their Chinese names.
func pickArticles() []Article {
	return []Article{
		{Slug: "hist-1", Disciplines: []string{"history"}},
		{Slug: "astro-1", Disciplines: []string{"astronomy"}},
		{Slug: "astro-2", Disciplines: []string{"astronomy", "history"}},
		{Slug: "prob-1", Disciplines: []string{"probability"}},
	}
}

func zh(t *testing.T, id string) string {
	t.Helper()
	d, ok := disciplines.ByID(id)
	if !ok {
		t.Fatalf("discipline %q missing from the table", id)
	}
	return d.Zh
}

func TestPickForStudentInterest(t *testing.T) {
	p := Profile{Disciplines: map[string]float64{"astronomy": 4}, Tier: 3}
	got, ok := PickForStudent(pickArticles(), p, nil)
	if !ok || got.Article.Slug != "astro-1" || got.Tier != 3 {
		t.Fatalf("pick = %+v, want astro-1 at tier 3", got)
	}
	if want := "兴趣相关：" + zh(t, "astronomy"); got.Reason() != want {
		t.Fatalf("reason = %q, want %q", got.Reason(), want)
	}
}

func TestPickForStudentExcludesOpened(t *testing.T) {
	p := Profile{Disciplines: map[string]float64{"astronomy": 4}, ReadSlugs: map[string]bool{"astro-1": true}, Tier: 2}
	got, _ := PickForStudent(pickArticles(), p, nil)
	if got.Article.Slug != "astro-2" {
		t.Fatalf("pick = %s, want astro-2 (astro-1 already opened)", got.Article.Slug)
	}
}

func TestPickForStudentFilter(t *testing.T) {
	// Interest in astronomy, filter history: astro-2 carries both and ranks first.
	p := Profile{Disciplines: map[string]float64{"astronomy": 4}, Tier: 2}
	got, _ := PickForStudent(pickArticles(), p, []string{"history"})
	if got.Article.Slug != "astro-2" || got.OutsideFilter {
		t.Fatalf("pick = %+v, want astro-2 inside the filter", got)
	}
	// Interest in probability, filter history: nothing in range matches her
	// interest, so the ranked history article is taken with the plain reason.
	p = Profile{Disciplines: map[string]float64{"probability": 4}, Tier: 2}
	got, _ = PickForStudent(pickArticles(), p, []string{"history"})
	if got.Article.Slug != "hist-1" || got.Reason() != "暂无兴趣相关文章，按难度推荐" {
		t.Fatalf("pick = %s reason %q", got.Article.Slug, got.Reason())
	}
}

func TestPickForStudentFilterExhausted(t *testing.T) {
	// Every history article is opened: first unopened article overall, in library order.
	p := Profile{ReadSlugs: map[string]bool{"hist-1": true, "astro-2": true}, Tier: 2}
	got, _ := PickForStudent(pickArticles(), p, []string{"history"})
	if got.Article.Slug != "astro-1" || !got.OutsideFilter || len(got.Why) != 0 {
		t.Fatalf("pick = %+v, want astro-1 outside the filter", got)
	}
	if got.Reason() != "筛选范围内暂无合适文章" {
		t.Fatalf("reason = %q", got.Reason())
	}
}

func TestPickForStudentNoInterestData(t *testing.T) {
	got, _ := PickForStudent(pickArticles(), Profile{Tier: 2}, nil)
	if got.Article.Slug != "hist-1" || got.Reason() != "暂无兴趣数据，按难度推荐" {
		t.Fatalf("pick = %s reason %q", got.Article.Slug, got.Reason())
	}
	// Zero strengths are not interest data.
	got, _ = PickForStudent(pickArticles(), Profile{Disciplines: map[string]float64{"history": 0}, Tier: 2}, nil)
	if got.Reason() != "暂无兴趣数据，按难度推荐" {
		t.Fatalf("zero-strength reason = %q", got.Reason())
	}
}

func TestPickForStudentAllRead(t *testing.T) {
	read := map[string]bool{"hist-1": true, "astro-1": true, "astro-2": true, "prob-1": true}
	got, ok := PickForStudent(pickArticles(), Profile{ReadSlugs: read, Tier: 2}, []string{"probability"})
	if !ok || !got.AllRead || got.Article.Slug != "prob-1" || got.Reason() != "暂无未读文章" {
		t.Fatalf("filtered all-read pick = %+v reason %q", got, got.Reason())
	}
	got, _ = PickForStudent(pickArticles(), Profile{ReadSlugs: read, Tier: 2}, nil)
	if got.Article.Slug != "hist-1" {
		t.Fatalf("unfiltered all-read pick = %s, want the first article", got.Article.Slug)
	}
}

func TestPickForStudentTier(t *testing.T) {
	for _, c := range []struct{ in, want int }{{0, 2}, {6, 2}, {1, 1}, {5, 5}} {
		got, _ := PickForStudent(pickArticles(), Profile{Tier: c.in}, nil)
		if got.Tier != c.want {
			t.Errorf("Profile.Tier %d → pick tier %d, want %d", c.in, got.Tier, c.want)
		}
	}
	if _, ok := PickForStudent(nil, Profile{Tier: 2}, nil); ok {
		t.Error("an empty library must return ok=false")
	}
}

func TestPickForStudentReasonListsUpToThree(t *testing.T) {
	s := StudentPick{Why: []string{"astronomy", "history"}, HasInterest: true}
	if want := "兴趣相关：" + zh(t, "astronomy") + "、" + zh(t, "history"); s.Reason() != want {
		t.Fatalf("reason = %q, want %q", s.Reason(), want)
	}
}
