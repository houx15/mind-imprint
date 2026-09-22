package api

import (
	"fmt"
	"reflect"
	"testing"

	"mindimprint/api/internal/interests"
	"mindimprint/api/internal/library"
	"mindimprint/api/internal/store/sqlc"
)

func TestDigRelatedKeywordsPrioritizesCurrentIdentityAndCapsContext(t *testing.T) {
	catalog := []interests.Interest{{ID: "art", Zh: "艺术", En: "Art"}, {ID: "climate", Zh: "气候", En: "Climate"}, {ID: "ocean", Zh: "海洋", En: "Ocean"}, {ID: "energy", Zh: "能源", En: "Energy"}, {ID: "robotics", Zh: "机器人", En: "Robotics"}}
	id := "robotics"
	got := digRelatedKeywords(sqlc.InterestKeyword{InterestID: &id, TextZh: "能源"}, []string{"气候、海洋和艺术。艺术艺术艺术。"}, catalog)
	ids := []string{}
	for _, it := range got {
		ids = append(ids, it.ID)
	}
	if want := []string{"robotics", "energy", "art"}; !reflect.DeepEqual(ids, want) {
		t.Fatalf("ids=%v want=%v", ids, want)
	}
}

func TestDigRelatedKeywordsUsesWholeEnglishWords(t *testing.T) {
	catalog := []interests.Interest{{ID: "art", Zh: "艺术", En: "Art"}, {ID: "climate", Zh: "气候", En: "Climate"}}
	got := digRelatedKeywords(sqlc.InterestKeyword{}, []string{"The earth has a changing CLIMATE."}, catalog)
	if len(got) != 1 || got[0].ID != "climate" {
		t.Fatalf("matches=%+v", got)
	}
}

func TestDigLibraryCandidatesFiltersBeforeRankingAndNeverUsesProfileFallback(t *testing.T) {
	catalog := []interests.Interest{{ID: "ocean", Zh: "海洋", Disciplines: []string{"ocean-d"}}, {ID: "music", Zh: "音乐", Disciplines: []string{"music-d"}}}
	articles := []library.Article{
		{Slug: "music", ZhTitle: "音乐文章", Disciplines: []string{"music-d"}},
		{Slug: "read", ZhTitle: "读过的海洋", Disciplines: []string{"ocean-d"}},
		{Slug: "empty", Disciplines: []string{"ocean-d"}},
		{Slug: "a", ZhTitle: "海洋一", Disciplines: []string{"ocean-d"}},
		{Slug: "b", Title: "Ocean two", Disciplines: []string{"ocean-d"}},
		{Slug: "c", ZhTitle: "海洋三", Disciplines: []string{"ocean-d"}},
		{Slug: "d", ZhTitle: "海洋四", Disciplines: []string{"ocean-d"}},
	}
	p := library.Profile{Tier: 4, ReadSlugs: map[string]bool{"read": true}, Disciplines: map[string]float64{"music-d": 10000}}
	got := selectDigLibraryCandidates(sqlc.InterestKeyword{TextZh: "海洋"}, nil, p, catalog, articles)
	slugs := []string{}
	for _, c := range got {
		slugs = append(slugs, c.Slug)
	}
	if want := []string{"a", "b", "c"}; !reflect.DeepEqual(slugs, want) {
		t.Fatalf("slugs=%v want=%v", slugs, want)
	}
	if got[1].Title != "Ocean two" {
		t.Fatal("English title fallback lost")
	}
	if got := selectDigLibraryCandidates(sqlc.InterestKeyword{TextZh: "未知话题"}, nil, p, catalog, articles); len(got) != 0 {
		t.Fatalf("unmatched keyword got generic recommendations: %+v", got)
	}
	if got := selectDigLibraryCandidates(sqlc.InterestKeyword{TextZh: "海洋"}, nil, p, catalog, articles[:3]); len(got) != 0 {
		t.Fatalf("no unread matching articles: %+v", got)
	}
	if p.Disciplines["music-d"] != 10000 || len(p.Disciplines) != 1 {
		t.Fatal("mutated shared profile")
	}
}

func TestDigLibraryCandidatesCurrentKeywordOutranksContext(t *testing.T) {
	catalog := []interests.Interest{{ID: "ocean", Zh: "海洋", Disciplines: []string{"ocean-d"}}, {ID: "music", Zh: "音乐", Disciplines: []string{"music-d"}}}
	articles := []library.Article{{Slug: "music", Title: "Music", Disciplines: []string{"music-d"}}, {Slug: "ocean", Title: "Ocean", Disciplines: []string{"ocean-d"}}}
	id := "ocean"
	got := selectDigLibraryCandidates(sqlc.InterestKeyword{InterestID: &id}, []string{"音乐"}, library.Profile{}, catalog, articles)
	if len(got) != 2 || got[0].Slug != "ocean" {
		t.Fatalf("order=%+v", got)
	}
}

// A numeric relevance bonus was insufficient: three rare contextual articles
// could outrank every article for the current keyword before the top-three cut.
func TestDigLibraryCandidatesScarcityCannotDisplaceCurrentInterest(t *testing.T) {
	catalog := []interests.Interest{
		{ID: "current", Zh: "当前兴趣", Disciplines: []string{"common", "current-rare"}},
		{ID: "context", Zh: "上下文兴趣", Disciplines: []string{"context-rare"}},
	}
	articles := []library.Article{
		{Slug: "context-a", Title: "Context A", Disciplines: []string{"context-rare"}},
		{Slug: "context-b", Title: "Context B", Disciplines: []string{"context-rare"}},
		{Slug: "context-c", Title: "Context C", Disciplines: []string{"context-rare"}},
	}
	for i := 0; i < 20; i++ {
		articles = append(articles, library.Article{Slug: fmt.Sprintf("current-%02d", i), Title: "Current", Disciplines: []string{"common"}})
	}
	// Scarcity still matters inside the current-keyword group. This article also
	// matches the contextual group but must appear only once, in the first group.
	articles = append(articles, library.Article{Slug: "current-special", Title: "Special", Disciplines: []string{"current-rare", "context-rare"}})
	id := "current"
	got := selectDigLibraryCandidates(sqlc.InterestKeyword{InterestID: &id}, []string{"上下文兴趣"}, library.Profile{}, catalog, articles)
	var slugs []string
	for _, c := range got {
		slugs = append(slugs, c.Slug)
	}
	if want := []string{"current-special", "current-00", "current-01"}; !reflect.DeepEqual(slugs, want) {
		t.Fatalf("got=%v want=%v", slugs, want)
	}
	// With fewer current-interest articles, later groups may fill remaining slots.
	got = selectDigLibraryCandidates(sqlc.InterestKeyword{InterestID: &id}, []string{"上下文兴趣"}, library.Profile{}, catalog, append(articles[:3], articles[len(articles)-1]))
	slugs = nil
	for _, c := range got {
		slugs = append(slugs, c.Slug)
	}
	if want := []string{"current-special", "context-a", "context-b"}; !reflect.DeepEqual(slugs, want) {
		t.Fatalf("remaining slots=%v want=%v", slugs, want)
	}
}

func TestDigLibraryCandidatesGamblingWithMovieContextKeepsCurrentTopic(t *testing.T) {
	id := "gambling"
	current, ok := interests.ByID(id)
	if !ok {
		t.Fatal("missing canonical gambling interest")
	}
	got := digLibraryCandidates(sqlc.InterestKeyword{InterestID: &id, TextZh: current.Zh}, []string{"我还提到了电影。"}, library.Profile{})
	if len(got) != 3 {
		t.Fatalf("expected three current-topic catalog candidates, got %+v", got)
	}
	for _, candidate := range got {
		article, ok := library.BySlug(candidate.Slug)
		if !ok {
			t.Fatalf("unknown article %s", candidate.Slug)
		}
		matches := false
		for _, d := range article.Disciplines {
			for _, target := range current.Disciplines {
				matches = matches || d == target
			}
		}
		if !matches {
			t.Fatalf("context displaced current gambling interest: %s", candidate.Slug)
		}
	}
}
