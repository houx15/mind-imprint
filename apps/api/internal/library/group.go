package library

// group.go — recommending reading to a whole class rather than one student.
// Everything here is a pure function over the same Profile Recommend already
// takes, so a class-wide recommendation is not a second recommender: it is
// one aggregate Profile fed through the same scoring pass.

import "sort"

// GroupRecommendation is one article recommended to a whole class: the
// article, the class's shared Tier, which disciplines earned it a slot (Why,
// straight from Recommend), and how many of the class's members have already
// opened it.
type GroupRecommendation struct {
	Article   Article
	Tier      int
	Why       []string
	ReadCount int
}

// GroupTier is the class-wide default difficulty: the median of the members'
// own Tier, the LOWER of the two middle values when the count is even (a
// class split between two levels reads the easier one together, rather than
// rounding up toward the half who would struggle). An empty class falls back
// to SuggestTier(0, 0) — the same cold-start default a single student gets.
func GroupTier(members []Profile) int {
	if len(members) == 0 {
		return SuggestTier(0, 0)
	}
	tiers := make([]int, len(members))
	for i, p := range members {
		t := p.Tier
		if t < 1 || t > 5 {
			t = 2
		}
		tiers[i] = t
	}
	sort.Ints(tiers)
	mid := len(tiers) / 2
	if len(tiers)%2 == 0 {
		return tiers[mid-1]
	}
	return tiers[mid]
}

// RecommendForGroup recommends articles for a whole class. Every member's
// Disciplines strengths are summed into one aggregate Profile and scored the
// same way Recommend scores one student — a class where two students carry
// strong climate-ocean interest and one carries history outranks history for
// the group, the same way it would for either of the two climate-interested
// students alone.
//
// The aggregate Profile's ReadSlugs is left empty on purpose: an article some
// of the class has already opened is still worth assigning to the rest, so it
// stays a candidate rather than disappearing the way it would for the one
// student who read it. ReadCount carries the fact instead of hiding it.
//
// The group's Tier (see GroupTier) is attached to every result, not each
// member's own tier — a class recommendation is read together.
func RecommendForGroup(articles []Article, members []Profile, limit int) []GroupRecommendation {
	agg := Profile{Disciplines: map[string]float64{}, Tier: GroupTier(members)}
	for _, p := range members {
		for id, strength := range p.Disciplines {
			agg.Disciplines[id] += strength
		}
	}
	recs := Recommend(articles, agg, limit)
	out := make([]GroupRecommendation, 0, len(recs))
	for _, rec := range recs {
		count := 0
		for _, p := range members {
			if p.ReadSlugs[rec.Article.Slug] {
				count++
			}
		}
		out = append(out, GroupRecommendation{
			Article: rec.Article, Tier: rec.Tier, Why: rec.Why, ReadCount: count,
		})
	}
	return out
}
