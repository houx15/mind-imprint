package library

import "testing"

// groupArticles is a tiny synthetic library for the group recommender, real
// discipline ids so Why can be checked against them.
func groupArticles() []Article {
	return []Article{
		{Slug: "climate-1", Disciplines: []string{"climate-ocean"}},
		{Slug: "hist-1", Disciplines: []string{"history"}},
	}
}

func TestRecommendForGroupPrefersTheSharedInterest(t *testing.T) {
	// Two students carry climate-ocean, one carries history: the group score
	// for climate-ocean (5+5) outweighs history (5) even though each article
	// only appears once in the tiny library above.
	members := []Profile{
		{Disciplines: map[string]float64{"climate-ocean": 5}, Tier: 2},
		{Disciplines: map[string]float64{"climate-ocean": 5}, Tier: 2},
		{Disciplines: map[string]float64{"history": 5}, Tier: 2},
	}
	got := RecommendForGroup(groupArticles(), members, 0)
	if len(got) == 0 || got[0].Article.Slug != "climate-1" {
		t.Fatalf("top recommendation = %+v, want climate-1 first", got)
	}
	if len(got[0].Why) == 0 || got[0].Why[0] != "climate-ocean" {
		t.Fatalf("why = %v, want it to name climate-ocean", got[0].Why)
	}
}

func TestRecommendForGroupReadCount(t *testing.T) {
	members := []Profile{
		{Disciplines: map[string]float64{"climate-ocean": 5}, ReadSlugs: map[string]bool{"climate-1": true}, Tier: 2},
		{Disciplines: map[string]float64{"climate-ocean": 5}, Tier: 2},
		{Disciplines: map[string]float64{"climate-ocean": 5}, Tier: 2},
	}
	got := RecommendForGroup(groupArticles(), members, 0)
	var found bool
	for _, r := range got {
		if r.Article.Slug != "climate-1" {
			continue
		}
		found = true
		if r.ReadCount != 1 {
			t.Fatalf("readCount = %d, want 1 (one of three members read it)", r.ReadCount)
		}
	}
	if !found {
		// A whole-class recommendation keeps an already-opened article as a
		// candidate for the rest of the class — it must not vanish the way it
		// would from one student's own ReadSlugs-filtered pick.
		t.Fatalf("climate-1 dropped from the group recommendations though only one of three members read it")
	}
}

func TestGroupTierIsTheMedian(t *testing.T) {
	if got := GroupTier([]Profile{{Tier: 2}, {Tier: 4}, {Tier: 5}}); got != 4 {
		t.Fatalf("median of an odd count = %d, want 4", got)
	}
	if got := GroupTier([]Profile{{Tier: 2}, {Tier: 4}}); got != 2 {
		t.Fatalf("median of an even count = %d, want the lower middle value 2", got)
	}
	if want := SuggestTier(0, 0); GroupTier(nil) != want {
		t.Fatalf("empty group tier = %d, want SuggestTier(0,0) = %d", GroupTier(nil), want)
	}
}

func TestRecommendForGroupLimitAndStableOrder(t *testing.T) {
	members := []Profile{{Disciplines: map[string]float64{"climate-ocean": 5}, Tier: 2}}
	if got := RecommendForGroup(groupArticles(), members, 1); len(got) != 1 {
		t.Fatalf("limit = 1 produced %d results", len(got))
	}
	first := RecommendForGroup(groupArticles(), members, 0)
	second := RecommendForGroup(groupArticles(), members, 0)
	if len(first) != len(second) {
		t.Fatalf("two runs of the same input returned different lengths: %d vs %d", len(first), len(second))
	}
	for i := range first {
		if first[i].Article.Slug != second[i].Article.Slug {
			t.Fatalf("order changed between two runs at index %d: %s vs %s", i, first[i].Article.Slug, second[i].Article.Slug)
		}
	}
}
