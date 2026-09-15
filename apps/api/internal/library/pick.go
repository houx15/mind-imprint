package library

import (
	"strings"

	"mindimprint/api/internal/disciplines"
)

// StudentPick is the one article a personalised reading homework gives a
// student, with what the teacher's preview says about it.
type StudentPick struct {
	Article Article
	Tier    int
	// Why is the matched discipline ids, at most three (from Recommend).
	Why []string
	// HasInterest: the profile carries at least one positive discipline strength.
	HasInterest bool
	// OutsideFilter: no unopened article carries a filtered discipline, so
	// the first unopened article overall was taken.
	OutsideFilter bool
	// AllRead: she has opened every article; the first article in the
	// filter (or the first overall) was taken.
	AllRead bool
}

// PickForStudent runs Recommend over the whole library (so the rarity
// discount matches the student shelf) and takes the first result that carries
// a filtered discipline. An empty filter takes the first result. ok is false
// only when articles is empty.
func PickForStudent(articles []Article, p Profile, filter []string) (StudentPick, bool) {
	if len(articles) == 0 {
		return StudentPick{}, false
	}
	pick := StudentPick{Tier: p.Tier}
	if pick.Tier < 1 || pick.Tier > 5 {
		pick.Tier = 2
	}
	for _, v := range p.Disciplines {
		if v > 0 {
			pick.HasInterest = true
			break
		}
	}
	want := make(map[string]bool, len(filter))
	for _, id := range filter {
		want[id] = true
	}
	inFilter := func(a Article) bool {
		if len(want) == 0 {
			return true
		}
		for _, id := range a.Disciplines {
			if want[id] {
				return true
			}
		}
		return false
	}

	recs := Recommend(articles, p, 0)
	for _, rec := range recs {
		if inFilter(rec.Article) {
			pick.Article, pick.Why = rec.Article, rec.Why
			return pick, true
		}
	}
	if len(recs) > 0 {
		// Unopened articles exist, none in the filter.
		for _, a := range articles {
			if !p.ReadSlugs[a.Slug] {
				pick.Article, pick.OutsideFilter = a, true
				return pick, true
			}
		}
	}
	pick.AllRead = true
	pick.Article = articles[0]
	for _, a := range articles {
		if inFilter(a) {
			pick.Article = a
			break
		}
	}
	return pick, true
}

// Reason is the line the teacher reads next to the pick. Built in code; no model.
func (s StudentPick) Reason() string {
	if s.AllRead {
		return "暂无未读文章"
	}
	if s.OutsideFilter {
		return "筛选范围内暂无合适文章"
	}
	names := make([]string, 0, len(s.Why))
	for _, id := range s.Why {
		if d, ok := disciplines.ByID(id); ok {
			names = append(names, d.Zh)
		}
	}
	if len(names) > 0 {
		return "兴趣相关：" + strings.Join(names, "、")
	}
	if s.HasInterest {
		return "暂无兴趣相关文章，按难度推荐"
	}
	return "暂无兴趣数据，按难度推荐"
}
