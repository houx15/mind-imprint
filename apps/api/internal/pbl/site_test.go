package pbl

import (
	"strings"
	"testing"
	"time"
)

func day(s string) time.Time {
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		panic(err)
	}
	return t
}

// 这个包最重要的一条测试：合成层不生产内容。
//
// 原型的失败就是这一条不成立——什么都不填，也能得到一个满满当当的页面，写的是
// 另一个人的生平。所以：空输入必须得到空页面。
func TestBuildSiteInventsNothing(t *testing.T) {
	got := BuildSite(SiteInput{DisplayName: "侯知遥"})

	if got.Name != "侯知遥" {
		t.Fatalf("name = %q, want 侯知遥", got.Name)
	}
	for _, c := range []struct {
		what string
		s    string
	}{
		{"headline", got.Headline},
		{"lead", got.Lead},
		{"role", got.Role},
		{"now", got.Now},
		{"email", got.Email},
	} {
		if c.s != "" {
			t.Errorf("%s = %q, 空输入不该长出字来", c.what, c.s)
		}
	}
	if len(got.About) != 0 || len(got.Projects) != 0 || len(got.Posts) != 0 || len(got.Reads) != 0 {
		t.Errorf("空输入产生了列表：about=%d projects=%d posts=%d reads=%d",
			len(got.About), len(got.Projects), len(got.Posts), len(got.Reads))
	}
	// 一个都没有的时候，站点信息里不该出现「写了 0 篇」。
	for _, s := range got.Stats {
		if strings.Contains(s.Value, "0 ") {
			t.Errorf("站点信息出现了零计数：%s %s", s.Label, s.Value)
		}
	}
}

// 空页面发不出去，而且报得出缺什么。
func TestSiteMissingBlocksAnEmptyPage(t *testing.T) {
	missing := SiteMissing(BuildSite(SiteInput{DisplayName: "侯知遥"}))
	if len(missing) == 0 {
		t.Fatal("一个字都没有的页面居然可以发布")
	}
	want := []string{"首屏那句话", "名字底下那行你是谁", "关于你自己的那段话"}
	for _, w := range want {
		found := false
		for _, m := range missing {
			if m == w {
				found = true
			}
		}
		if !found {
			t.Errorf("missing 里没有 %q，得到 %v", w, missing)
		}
	}
}

// 她把三样都写了，就能发——即使一件作品、一篇文章都还没有。刚开始的人本来就
// 没有作品，那不该是拦住她发布的理由。
func TestSiteMissingAllowsAPageWithNoWorkYet(t *testing.T) {
	c := BuildSite(SiteInput{
		DisplayName: "侯知遥",
		Draft: SiteDraft{
			Headline: "一件还能修的东西，是谁决定它该被扔的？",
			Role:     "读 IB 的高二学生",
			About:    []string{"我在拆家里所有还能拆的东西。"},
		},
	})
	if m := SiteMissing(c); len(m) != 0 {
		t.Fatalf("三样都写了还是发不出去：%v", m)
	}
}

// 她写的介绍语按稳定 id 认领，不按下标。
func TestBlurbsFollowTheItemNotItsPosition(t *testing.T) {
	older := SiteItem{AtomID: "aaa", Title: "旧的那篇", When: day("2026-01-02"), HasWhen: true}
	newer := SiteItem{AtomID: "bbb", Title: "新的那篇", When: day("2026-06-02"), HasWhen: true}
	draft := SiteDraft{Blurbs: map[string]string{"aaa": "属于旧的那篇的一句话"}}

	// 先只有一篇，再多出一篇更新的排到它前面——她写的那句话必须还跟着原来那篇。
	one := BuildSite(SiteInput{DisplayName: "侯", Draft: draft, Posts: []SiteItem{older}})
	two := BuildSite(SiteInput{DisplayName: "侯", Draft: draft, Posts: []SiteItem{older, newer}})

	if one.Posts[0].Blurb != "属于旧的那篇的一句话" {
		t.Fatalf("一篇时就丢了：%q", one.Posts[0].Blurb)
	}
	if two.Posts[0].Title != "新的那篇" {
		t.Fatalf("没有按日期倒序：%q 排在最前", two.Posts[0].Title)
	}
	if two.Posts[1].Blurb != "属于旧的那篇的一句话" {
		t.Errorf("多出一篇之后介绍语跑到别人底下了：%q", two.Posts[1].Blurb)
	}
	if two.Posts[0].Blurb != "" {
		t.Errorf("没写介绍语的那篇被填了字：%q", two.Posts[0].Blurb)
	}
}

// 稳定 id 不能是 atom id 本身，而且必须稳定、必须随人不同。
func TestSiteItemIDHidesTheAtomAndStaysStable(t *testing.T) {
	const atom = "0f1e2d3c-4b5a-6978-8796-a5b4c3d2e1f0"
	a := SiteItemID("user-1", atom)
	if a == atom || strings.Contains(a, atom) {
		t.Fatalf("公开 id 里带着 atom id：%q", a)
	}
	if a != SiteItemID("user-1", atom) {
		t.Error("同一个学生同一条，两次算出来不一样")
	}
	if a == SiteItemID("user-2", atom) {
		t.Error("换个学生算出了同一个 id")
	}
}

// 没有完成时间的条目，日期栏是空的，不是 0001-01-01。
func TestUndatedItemsShowNoDate(t *testing.T) {
	c := BuildSite(SiteInput{
		DisplayName: "侯",
		Posts:       []SiteItem{{AtomID: "x", Title: "还没标完成时间"}},
	})
	if d := c.Posts[0].Date; d != "" {
		t.Fatalf("date = %q，想要空串", d)
	}
}

// 头图的种子随人不同，同一个人不变。
func TestBannerSeedIsPersonalAndStable(t *testing.T) {
	a := BuildSite(SiteInput{DisplayName: "侯知遥"}).Seed
	if a != BuildSite(SiteInput{DisplayName: "侯知遥"}).Seed {
		t.Error("同一个人两次画出不同的头图")
	}
	if a == BuildSite(SiteInput{DisplayName: "林知遥"}).Seed {
		t.Error("两个人共用一张头图")
	}
}

// 同一件作品的色版不随「她又做了一个项目」而整排换掉。
func TestPlatesStayWithTheirProject(t *testing.T) {
	p1 := SiteItem{AtomID: "aaa", Title: "药盒"}
	p2 := SiteItem{AtomID: "bbb", Title: "地图"}
	one := BuildSite(SiteInput{DisplayName: "侯", Projects: []SiteItem{p1}})
	two := BuildSite(SiteInput{DisplayName: "侯", Projects: []SiteItem{p2, p1}})
	if one.Projects[0].Plate != two.Projects[1].Plate {
		t.Errorf("插进一个新项目之后，旧项目换了颜色：%v → %v",
			one.Projects[0].Plate, two.Projects[1].Plate)
	}
}

func TestIsSiteLayout(t *testing.T) {
	for _, ok := range SiteLayouts {
		if !IsSiteLayout(ok) {
			t.Errorf("%q 应该是合法版式", ok)
		}
	}
	for _, bad := range []string{"", "Essay", "blog", "essay "} {
		if IsSiteLayout(bad) {
			t.Errorf("%q 不该通过", bad)
		}
	}
}
